//go:build qemu

package image

// Throwaway experiment (not part of the normal test run; select it with -run TestHangCompare): how often does the machine stall in the
// reboot into the TRIAL BOOT of slot b? Each cycle: a fresh disk (build-disk.sh), first boot, `hubos-ctl update` of the bundle h2-good into
// slot b, `reboot -f`, then wait (once, no retry) for the handover line of the trial boot; the outcome is counted: ok, hang (no handover line
// in 150 s), crash (QEMU died from a signal). Every serial log is saved. The kernel is the only thing that differs between two runs:
//   HUBOS_HANG_KERNEL=new   the current image/kernel/hub.frag
//   HUBOS_HANG_KERNEL=old   hub.frag without the text console options (CONFIG_VT, FB, DRM_FBDEV_EMULATION, FRAMEBUFFER_CONSOLE, FONTS, FONT_8x16)
//   HUBOS_HANG_N=10         cycles (default 10)
//   HUBOS_HANG_LOGS=DIR     where the serial logs of every cycle are kept (required)
//   HUBOS_HUB_WORK=DIR      keep the build between the two runs (the root and the base are reused; only the kernel is rebuilt)
// Example: HUBOS_HANG_KERNEL=old HUBOS_HANG_LOGS=/some/dir HUBOS_HUB_WORK=/tmp/x go test -tags qemu -count=1 -timeout 150m -v -run 'TestHangCompare$' ./tools/image

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestHangCompare(t *testing.T) {
	which := os.Getenv("HUBOS_HANG_KERNEL")
	logs := os.Getenv("HUBOS_HANG_LOGS")
	if which != "old" && which != "new" || logs == "" {
		t.Skip("set HUBOS_HANG_KERNEL=old|new and HUBOS_HANG_LOGS=DIR")
	}
	n := 10
	if s := os.Getenv("HUBOS_HANG_N"); s != "" {
		n, _ = strconv.Atoi(s)
	}
	work := os.Getenv("HUBOS_HUB_WORK")
	if work == "" {
		var err error
		if work, err = os.MkdirTemp("", "hubos-hang-"); err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(work)
	}
	os.MkdirAll(work, 0o755)
	os.MkdirAll(logs, 0o755)

	// a machine file and a fragment of their own (generated, removed at the end), so the committed files stay as they are
	frag, _ := os.ReadFile("../../image/kernel/hub.frag")
	text := string(frag)
	if which == "old" {
		var keep []string
		skip := regexp.MustCompile(`^(CONFIG_VT=y|CONFIG_FB=y|CONFIG_DRM_FBDEV_EMULATION=y|CONFIG_FRAMEBUFFER_CONSOLE=y|CONFIG_FONTS=y|CONFIG_FONT_8x16=y)$`)
		for _, l := range strings.Split(text, "\n") {
			if skip.MatchString(strings.TrimSpace(l)) {
				continue
			}
			keep = append(keep, l)
		}
		text = strings.Join(keep, "\n")
	}
	fragRel := "image/kernel/hang-compare-" + which + ".frag"
	machRel := "image/machines/hang-compare-" + which + ".build"
	os.WriteFile(filepath.Join("../..", fragRel), []byte(text), 0o644)
	mb, _ := os.ReadFile("../../image/machines/hub.build")
	mach := regexp.MustCompile(`(?m)^KERNEL_FRAGMENT=.*$`).ReplaceAllString(string(mb), "KERNEL_FRAGMENT="+fragRel)
	os.WriteFile(filepath.Join("../..", machRel), []byte(mach), 0o644)
	defer os.Remove(filepath.Join("../..", fragRel))
	defer os.Remove(filepath.Join("../..", machRel))

	r := &rig{t: t, work: work, disk: filepath.Join(work, "vm"), hangDir: filepath.Join(work, "hangs"), machine: machRel,
		gui: true, monPath: filepath.Join(work, "mon.sock"), shots: filepath.Join(work, "shots")}
	os.MkdirAll(r.shots, 0o755)
	defer func() {
		if r.vm != nil {
			r.vm.kill()
		}
	}()

	r.script("fetch-tools.sh", nil)
	kdir := filepath.Join(work, "keys")
	os.RemoveAll(kdir)
	os.MkdirAll(kdir, 0o700)
	os.Setenv("LD_LIBRARY_PATH", filepath.Join(work, "tools/root/usr/lib/x86_64-linux-gnu")+":"+filepath.Join(work, "tools/root/lib/x86_64-linux-gnu"))
	r.pub, r.sec = filepath.Join(kdir, "update.pub"), filepath.Join(kdir, "update.sec")
	if out, err := exec.Command(r.tool("bin/signify-openbsd"), "-G", "-n", "-p", r.pub, "-s", r.sec, "-c", "throwaway hang-compare key").CombinedOutput(); err != nil {
		t.Fatalf("signify -G: %v\n%s", err, out)
	}
	r.script("build-kernel.sh", []string{"UPDATE_PUB=" + r.pub})
	r.script("build-base.sh", nil)
	r.script("build-hub-parts.sh", nil)
	bdir := filepath.Join(work, "bundles")
	os.RemoveAll(bdir)
	os.MkdirAll(bdir, 0o755)
	for _, b := range []struct{ v, f string }{{"1", "good"}, {"2", "good"}} {
		r.script("build-bundle.sh", nil, b.v, b.f, filepath.Join(bdir, "h"+b.v+"-"+b.f), r.pub, r.sec)
	}
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	go http.Serve(ln, http.FileServer(http.Dir(bdir)))
	r.base = fmt.Sprintf("http://10.0.2.2:%d", ln.Addr().(*net.TCPAddr).Port)
	defer ln.Close()

	// the kernel that is really in the image: size and hash, for the record
	ks := ""
	for _, f := range []string{"kernel-a.efi", "kernel-b.efi", "kernel-recovery.efi"} {
		ks += fmt.Sprintf(" %s=%d bytes", f, fileSize(t, filepath.Join(work, "out", f)))
	}
	t.Logf("kernel %s:%s", which, ks)

	var ok, hang, crash, firstBootTrouble, notConfirmed int
	var lines []string
	for i := 1; i <= n; i++ {
		os.RemoveAll(r.disk)
		r.script("build-disk.sh", nil, filepath.Join(bdir, "h1-good", "rootfs.sqsh"), filepath.Join(work, "out", "kernel-a.efi"), r.disk)
		v := r.bootOnce(fmt.Sprintf("cycle %d first boot", i))
		if v == nil {
			firstBootTrouble++
			lines = append(lines, fmt.Sprintf("cycle %d: the FIRST boot (slot a) hung or crashed; cycle not counted", i))
			continue
		}
		r.vm = v
		rc, _ := r.update("h2-good")
		if rc != 0 {
			t.Fatalf("cycle %d: update rc=%d", i, rc)
		}
		m := v.mark()
		start := time.Now()
		fmt.Fprintf(v.in, "sync; reboot -f\n")
		// the one measurement: the trial boot of slot b (no retry)
		if e := v.wait(handoverRe, 150*time.Second, m); e >= 0 {
			ok++
			r.bootPos = e
			r.ready(v)
			conf := r.waitConfirmed()
			slot, rel, _, _ := r.status()
			if !conf || slot != "b" {
				notConfirmed++
			}
			lines = append(lines, fmt.Sprintf("cycle %d: trial boot OK after %.0f s (slot %s release %s, confirmed %v)", i, time.Since(start).Seconds(), slot, rel, conf))
		} else {
			sig := v.crashSignal()
			dst := filepath.Join(logs, fmt.Sprintf("%s-cycle%02d-trial", which, i))
			if sig != "" {
				crash++
				dst += "-CRASH.log"
				lines = append(lines, fmt.Sprintf("cycle %d: QEMU CRASH (%s) in the trial boot", i, sig))
			} else {
				hang++
				dst += "-HANG.log"
				lines = append(lines, fmt.Sprintf("cycle %d: HANG in the trial boot (no handover line in 150 s); log %s", i, dst))
			}
			os.WriteFile(dst, []byte(v.text(0)), 0o644)
		}
		// keep every serial log, hang or not
		os.WriteFile(filepath.Join(logs, fmt.Sprintf("%s-cycle%02d-all.log", which, i)), []byte(v.text(0)), 0o644)
		v.kill()
		r.vm = nil
	}
	summary := fmt.Sprintf("HANG-COMPARE kernel=%s cycles=%d trial-boot ok=%d hang=%d crash=%d (first-boot trouble %d, trial boots not confirmed on slot b %d); %s\n%s",
		which, n, ok, hang, crash, firstBootTrouble, notConfirmed, strings.TrimSpace(ks), strings.Join(lines, "\n"))
	t.Log("\n" + summary)
	os.WriteFile(filepath.Join(logs, which+"-summary.txt"), []byte(summary+"\n"), 0o644)
}
