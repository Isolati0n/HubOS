//go:build qemu

package image

// The Phase B first-slice tests (docs/image.md, docs/proposals/phase-b-image.md section 8).
// They build the kernel, the root and the bundles with the scripts in this directory,
// then boot a UEFI disk under QEMU in software emulation and drive it over the serial
// console. Run:  go test -tags qemu -count=1 -timeout 120m -v ./tools/image
// Run them by hand before merging any change to image/ or tools/image/ (docs/image.md).
// Environment: HUBOS_IMAGE_WORK = a directory to keep the (long) build between runs.
// Every key is made at run time in a temporary directory; no key is ever stored.

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const handoverRe = `HUBOS: handing over to s6-svscan`

type vm struct {
	t    *testing.T
	cmd  *exec.Cmd
	in   io.WriteCloser
	mu   sync.Mutex
	buf  strings.Builder
	log  *os.File
	done chan struct{}
}

type rig struct {
	t         *testing.T
	work      string
	disk      string // directory with disk.img and vars.fd
	base      string // http://10.0.2.2:PORT
	sec       string
	pub       string
	hangs     int
	hangLog   []string // every hang: number, step, saved log
	hangDir   string
	forceHang int    // test hook: treat the next N boots as hangs (HUBOS_TEST_FORCE_HANG=once|twice)
	dead      bool   // the run was stopped by a repeated hang; later VM tests skip
	recSig    string // what the first recovery boot looked like (kernel command line and release), to compare later ones
	vm        *vm
	logSeq    int
	bootPos   int // where the output of the current boot starts in the VM buffer
}

func (r *rig) script(name string, env []string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command(filepath.Join("..", "..", "tools", "image", name), args...)
	cmd.Env = append(os.Environ(), append([]string{"WORK=" + r.work}, env...)...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	start := time.Now()
	err := cmd.Run()
	r.t.Logf("%s %s: %s (%.0f s)", name, strings.Join(args, " "), lastLine(out.String()), time.Since(start).Seconds())
	if err != nil {
		r.t.Fatalf("%s failed: %v\n%s", name, err, out.String())
	}
	return out.String()
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "\n"); i >= 0 {
		return s[i+1:]
	}
	return s
}

func (r *rig) tool(path string) string { return filepath.Join(r.work, "tools", "root", path) }

func (r *rig) startVM() *vm {
	r.logSeq++
	logPath := filepath.Join(r.disk, fmt.Sprintf("serial-%02d.log", r.logSeq))
	lf, err := os.Create(logPath)
	if err != nil {
		r.t.Fatal(err)
	}
	T := filepath.Join(r.work, "tools", "root")
	args := []string{
		"-L", T + "/usr/share/qemu", "-L", T + "/usr/share/seabios",
		"-machine", "q35,smm=off", "-accel", "tcg", "-smp", "2", "-m", "1024", "-nographic", "-display", "none",
		"-drive", "if=pflash,format=raw,unit=0,readonly=on,file=" + T + "/usr/share/OVMF/OVMF_CODE_4M.fd",
		"-drive", "if=pflash,format=raw,unit=1,file=" + filepath.Join(r.disk, "vars.fd"),
		"-drive", "file=" + filepath.Join(r.disk, "disk.img") + ",if=none,id=d0,format=raw", "-device", "virtio-blk-pci,drive=d0",
		"-netdev", "user,id=n0", "-device", "virtio-net-pci,netdev=n0,romfile=",
		"-device", "i6300esb", "-watchdog-action", "reset", "-serial", "stdio", "-monitor", "none",
	}
	cmd := exec.Command(T+"/usr/bin/qemu-system-x86_64", args...)
	cmd.Env = append(os.Environ(),
		"LD_LIBRARY_PATH="+T+"/usr/lib/x86_64-linux-gnu:"+T+"/lib/x86_64-linux-gnu:"+T+"/usr/lib",
		"QEMU_MODULE_DIR="+T+"/usr/lib/x86_64-linux-gnu/qemu")
	in, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	v := &vm{t: r.t, cmd: cmd, in: in, log: lf, done: make(chan struct{})}
	if err := cmd.Start(); err != nil {
		r.t.Fatal(err)
	}
	go func() {
		b := make([]byte, 4096)
		for {
			n, err := out.Read(b)
			if n > 0 {
				s := strings.ReplaceAll(string(b[:n]), "\r", "")
				v.mu.Lock()
				v.buf.WriteString(s)
				v.mu.Unlock()
				lf.WriteString(s)
			}
			if err != nil {
				close(v.done)
				return
			}
		}
	}()
	r.vm = v
	return v
}

func (v *vm) mark() int { v.mu.Lock(); defer v.mu.Unlock(); return v.buf.Len() }

func (v *vm) text(from int) string {
	v.mu.Lock()
	defer v.mu.Unlock()
	s := v.buf.String()
	if from > len(s) {
		from = len(s)
	}
	return s[from:]
}

// wait looks for the pattern in the output after position from; it returns the position after
// the match, or -1 on timeout.
func (v *vm) wait(pat string, d time.Duration, from int) int {
	re := regexp.MustCompile(pat)
	end := time.Now().Add(d)
	for time.Now().Before(end) {
		s := v.text(from)
		if m := re.FindStringIndex(s); m != nil {
			return from + m[1]
		}
		select {
		case <-v.done:
			s = v.text(from)
			if m := re.FindStringIndex(s); m != nil {
				return from + m[1]
			}
			return -1
		default:
		}
		time.Sleep(100 * time.Millisecond)
	}
	return -1
}

var tagSeq atomic.Int64

// sh runs a shell command on the guest console and returns its status and output.
func (v *vm) sh(cmd string, d time.Duration) (int, string) {
	tag := fmt.Sprintf("__D%d__", tagSeq.Add(1))
	m := v.mark()
	fmt.Fprintf(v.in, "%s\necho %s$?\n", cmd, tag)
	end := v.wait(regexp.QuoteMeta(tag)+`\d+`, d, m)
	if end < 0 {
		return -1, "<timeout> " + v.text(m)
	}
	seg := v.text(m)[:end-m]
	mm := regexp.MustCompile(regexp.QuoteMeta(tag) + `(\d+)`).FindStringSubmatchIndex(seg)
	var rc int
	fmt.Sscanf(seg[mm[2]:mm[3]], "%d", &rc)
	var keep []string
	for _, l := range strings.Split(seg[:mm[0]], "\n") {
		if !strings.Contains(l, tag) {
			keep = append(keep, l)
		}
	}
	return rc, strings.TrimSpace(strings.Join(keep, "\n"))
}

func (v *vm) kill() {
	if v == nil {
		return
	}
	v.cmd.Process.Kill()
	v.cmd.Wait()
	v.log.Close()
}

// bootOnce starts QEMU on the disk and waits for the handover line. If the line does not come in time (an
// unexplained hang), the hang is recorded with its step, the serial log is saved and nil is returned.
func (r *rig) bootOnce(step string) *vm {
	v := r.startVM()
	if r.forceHang > 0 {
		// Test hook: pretend this boot hung. QEMU really was started; we do not wait for the handover line.
		r.forceHang--
		time.Sleep(3 * time.Second)
		r.noteHang(v, step+" [forced by HUBOS_TEST_FORCE_HANG]")
		v.kill()
		return nil
	}
	if v.wait(handoverRe, 150*time.Second, 0) >= 0 {
		r.bootPos = 0
		r.ready(v)
		return v
	}
	r.noteHang(v, step)
	v.kill()
	return nil
}

// bootVM boots the disk. A hang is retried ONCE; a second hang in the same step fails the run.
func (r *rig) bootVM(step string) *vm {
	if v := r.bootOnce(step); v != nil {
		return v
	}
	return r.retryBoot(step)
}

// retryBoot is the one retry after a hang in a step; if it hangs too, the run fails.
func (r *rig) retryBoot(step string) *vm {
	if v := r.bootOnce(step + " (retry)"); v != nil {
		r.vm = v
		return v
	}
	r.dead = true
	r.t.Fatalf("the machine hung twice in the same step (%s); the run fails (hangs so far: %s)", step, strings.Join(r.hangLog, "; "))
	return nil
}

func (r *rig) ready(v *vm) {
	time.Sleep(2500 * time.Millisecond)
	io.WriteString(v.in, "stty -echo\n")
	time.Sleep(500 * time.Millisecond)
}

func (r *rig) noteHang(v *vm, what string) {
	r.hangs++
	os.MkdirAll(r.hangDir, 0o755)
	dst := filepath.Join(r.hangDir, fmt.Sprintf("hang-%d.log", r.hangs))
	r.hangLog = append(r.hangLog, fmt.Sprintf("hang %d in step %q (serial log %s)", r.hangs, what, dst))
	os.WriteFile(dst, []byte(v.text(0)), 0o644)
	r.t.Logf("HANG %d (%s): no known log line in time; serial log saved to %s", r.hangs, what, dst)
}

// reboot sends a reboot and waits for the next handover; on a hang it restarts QEMU on the same disk.
func (r *rig) reboot() int {
	v := r.vm
	m := v.mark()
	io.WriteString(v.in, "sync; reboot -f\n")
	return r.afterReset(m)
}

// afterReset waits for the next boot of the running VM; position m is where its output starts.
func (r *rig) afterReset(m int) int {
	v := r.vm
	if e := v.wait(handoverRe, 150*time.Second, m); e >= 0 {
		r.bootPos = e
		r.ready(v)
		return m
	}
	r.noteHang(v, "reboot")
	v.kill()
	r.vm = r.retryBoot("reboot")
	return 0
}

func (r *rig) sh(cmd string) (int, string) {
	rc, out := r.vm.sh(cmd, 120*time.Second)
	r.t.Logf("$ %s\n%s", cmd, out)
	return rc, out
}

var (
	slotRe  = regexp.MustCompile(`slot=([ab]) release=(\d+)`)
	orderRe = regexp.MustCompile(`BootOrder: ([0-9A-F]{4})`)
)

// status returns the slot and release the machine runs, and whether that slot is first in BootOrder.
func (r *rig) status() (slot string, rel string, confirmed bool, text string) {
	_, out := r.sh("hubos-ctl status")
	m := slotRe.FindStringSubmatch(out)
	if m == nil {
		return "", "", false, out
	}
	slot, rel = m[1], m[2]
	e := regexp.MustCompile(`Boot([0-9A-F]{4})\*? hubos-` + slot).FindStringSubmatch(out)
	o := orderRe.FindStringSubmatch(out)
	return slot, rel, e != nil && o != nil && e[1] == o[1], out
}

// waitConfirmed waits until the running slot has been confirmed (BootOrder first) or the boot failed.
func (r *rig) waitConfirmed() bool {
	return r.vm.wait(`confirm: (boot of slot [ab].* confirmed|slot [ab] is already the confirmed slot)`, 90*time.Second, r.bootPos) >= 0
}

func (r *rig) update(name string) (int, string) {
	rc, out := r.vm.sh("hubos-ctl update "+r.base+"/"+name+" 2>&1", 300*time.Second)
	r.t.Logf("$ hubos-ctl update %s\n%s", name, out)
	return rc, out
}

// bootEntries returns the hubos entries the firmware still had when it started the most recent boot (a line stage 1
// prints before the confirm step can recreate anything).
func (r *rig) bootEntries() string {
	ms := regexp.MustCompile(`HUBOS: entries present at boot: ([^\n]*)`).FindAllStringSubmatch(r.vm.text(0), -1)
	if len(ms) == 0 {
		return "unknown"
	}
	return strings.TrimSpace(ms[len(ms)-1][1])
}

// recoveryCmds are run in the recovery shell. The output is also the "signature" of a recovery boot.
const recoveryCmds = `cat /proc/cmdline; tr '\n' ' ' < /etc/hubos-release; echo; hubos-ctl status; efibootmgr -v | grep hubos-recovery; ps | head -n 4`

// recoverySig reduces the recovery shell output to the command line and the release line.
func recoverySig(out string) string {
	cl := regexp.MustCompile(`(?m)^console=.*$`).FindString(out)
	rel := regexp.MustCompile(`(?m)^version=\S+ .*$`).FindString(out)
	return strings.TrimSpace(cl) + " | " + strings.TrimSpace(rel)
}

// waitRecovery waits for the separate recovery kernel's banner on the running VM, then makes the shell usable.
func (r *rig) waitRecovery(v *vm, from int) bool {
	if v.wait(`HUBOS: RECOVERY MODE`, 200*time.Second, from) < 0 {
		return false
	}
	time.Sleep(2500 * time.Millisecond)
	io.WriteString(v.in, "stty -echo\n")
	time.Sleep(500 * time.Millisecond)
	return true
}

// bootRecoveryEntry points BootNext at the recovery entry and reboots. If recovery mode comes up it runs a few
// commands in the bare terminal and returns banner=true and their output. The caller then reboots out of
// recovery.
func (r *rig) bootRecoveryEntry() (banner bool, out string) {
	r.sh(`efibootmgr -q -n $(efibootmgr | sed -n 's/^Boot\([0-9A-F]*\)\*\{0,1\}[[:space:]]hubos-recovery[[:space:]].*/\1/p' | head -n 1)`)
	m := r.vm.mark()
	io.WriteString(r.vm.in, "sync; reboot -f\n")
	if !r.waitRecovery(r.vm, m) {
		return false, r.vm.text(m)
	}
	printed := r.vm.text(m)
	_, sh := r.vm.sh(recoveryCmds, 60*time.Second)
	r.t.Logf("recovery shell:\n%s", sh)
	return true, printed + "\n" + sh
}

// startRecovery starts QEMU on the disk and waits for the recovery shell (the disk has BootNext set to recovery).
func (r *rig) startRecovery() *vm {
	v := r.startVM()
	if !r.waitRecovery(v, 0) {
		r.t.Fatalf("the recovery shell did not come up")
	}
	r.vm = v
	return v
}

func copyFile(t *testing.T, src, dst string) {
	if out, err := exec.Command("cp", "--sparse=always", src, dst).CombinedOutput(); err != nil {
		t.Fatalf("cp %s %s: %v %s", src, dst, err, out)
	}
}

// ---------------------------------------------------------------------------

func TestImage(t *testing.T) {
	work := os.Getenv("HUBOS_IMAGE_WORK")
	if work == "" {
		var err error
		if work, err = os.MkdirTemp("", "hubos-image-"); err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(work)
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	r := &rig{t: t, work: work, disk: filepath.Join(work, "vm"), hangDir: filepath.Join(work, "hangs")}
	switch os.Getenv("HUBOS_TEST_FORCE_HANG") {
	case "once":
		r.forceHang = 1 // the first boot is treated as a hang; the retry must succeed and the run continues
	case "twice":
		r.forceHang = 2 // the first boot and its retry are treated as hangs; the run must fail
	}
	var results []string
	record := func(name string, ok bool, d time.Duration, note string) {
		s := "PASS"
		if !ok {
			s = "FAIL"
		}
		results = append(results, fmt.Sprintf("%-4s %7.1fs  %s  %s", s, d.Seconds(), name, note))
	}
	defer func() {
		hl := "none"
		if len(r.hangLog) > 0 {
			hl = "\n  " + strings.Join(r.hangLog, "\n  ")
		}
		t.Logf("\nRESULTS (QEMU hangs seen and retried: %d; every hang: %s)\n%s", r.hangs, hl, strings.Join(results, "\n"))
		if r.vm != nil {
			r.vm.kill()
		}
	}()

	// ---- build ----
	r.script("fetch-tools.sh", nil)
	r.script("build-kernel.sh", nil)
	r.script("build-base.sh", nil)
	kdir := filepath.Join(work, "keys")
	os.RemoveAll(kdir)
	os.MkdirAll(kdir, 0o700)
	genKey := func(name string) (pub, sec string) {
		pub, sec = filepath.Join(kdir, name+".pub"), filepath.Join(kdir, name+".sec")
		out, err := exec.Command(r.tool("bin/signify-openbsd"), "-G", "-n", "-p", pub, "-s", sec, "-c", "throwaway key "+name).CombinedOutput()
		if err != nil {
			t.Fatalf("signify -G: %v\n%s", err, out)
		}
		return
	}
	// The signify binary needs its libraries from the unpacked tools.
	os.Setenv("LD_LIBRARY_PATH", filepath.Join(work, "tools/root/usr/lib/x86_64-linux-gnu")+":"+filepath.Join(work, "tools/root/lib/x86_64-linux-gnu"))
	r.pub, r.sec = genKey("update")
	_, sec2 := genKey("other")
	t.Logf("throwaway signing keys made in %s (deleted with the temporary directory; never committed)", kdir)

	bdir := filepath.Join(work, "bundles")
	os.RemoveAll(bdir)
	os.MkdirAll(bdir, 0o755)
	bundle := func(ver, flavor, name string, env ...string) {
		r.script("build-bundle.sh", env, ver, flavor, filepath.Join(bdir, name), r.pub, r.sec)
	}
	for _, b := range []struct{ v, f string }{{"1", "good"}, {"2", "good"}, {"3", "noinit"}, {"4", "garbage"}, {"5", "unhealthy"}, {"6", "hang"}, {"7", "good"}, {"8", "good"}} {
		bundle(b.v, b.f, "v"+b.v+"-"+b.f)
	}
	bundle("2", "good", "v2-nokernelversion", "NO_KERNEL_VERSION=1")
	// bundles for the recovery-kernel update rules: none in the manifest, a newer one (changed file), the same version again
	bundle("9", "good", "v9-norecovery", "NO_RECOVERY=1")
	bundle("10", "good", "v10-recovery2", "RECOVERY_VERSION_OVERRIDE=2")
	bundle("11", "good", "v11-recovery2-again", "RECOVERY_VERSION_OVERRIDE=2")
	// refused variants: made from v2-good with hard links
	variant := func(name string, change func(dir string)) {
		d := filepath.Join(bdir, name)
		os.MkdirAll(d, 0o755)
		for _, f := range []string{"manifest", "manifest.sig", "kernel-a.efi", "kernel-b.efi", "kernel-recovery.efi", "rootfs.sqsh"} {
			if err := os.Link(filepath.Join(bdir, "v2-good", f), filepath.Join(d, f)); err != nil {
				t.Fatal(err)
			}
		}
		change(d)
	}
	rewrite := func(d, f string, fn func([]byte) []byte) {
		p := filepath.Join(d, f)
		b, _ := os.ReadFile(p)
		os.Remove(p)
		os.WriteFile(p, fn(b), 0o644)
	}
	variant("v2-unsigned", func(d string) { os.Remove(filepath.Join(d, "manifest.sig")) })
	variant("v2-badmanifest", func(d string) {
		rewrite(d, "manifest", func(b []byte) []byte { return bytes.Replace(b, []byte("version 2"), []byte("version 9"), 1) })
	})
	variant("v2-wrongkey", func(d string) {
		os.Remove(filepath.Join(d, "manifest.sig"))
		out, err := exec.Command(r.tool("bin/signify-openbsd"), "-S", "-s", sec2, "-m", filepath.Join(d, "manifest"), "-x", filepath.Join(d, "manifest.sig")).CombinedOutput()
		if err != nil {
			t.Fatalf("%v %s", err, out)
		}
	})
	resign := func(d string) {
		os.Remove(filepath.Join(d, "manifest.sig"))
		if out, err := exec.Command(r.tool("bin/signify-openbsd"), "-S", "-s", r.sec, "-m", filepath.Join(d, "manifest"), "-x", filepath.Join(d, "manifest.sig")).CombinedOutput(); err != nil {
			t.Fatalf("%v %s", err, out)
		}
	}
	dropLine := func(prefix string) func([]byte) []byte {
		return func(b []byte) []byte {
			var keep []string
			for _, l := range strings.Split(string(b), "\n") {
				if !strings.HasPrefix(l, prefix) {
					keep = append(keep, l)
				}
			}
			return []byte(strings.Join(keep, "\n"))
		}
	}
	recKernel10 := ""
	{
		d := filepath.Join(bdir, "v10-recovery2")
		kp := filepath.Join(d, "kernel-recovery.efi")
		b, err := os.ReadFile(kp)
		if err != nil {
			t.Fatal(err)
		}
		b = append(b, []byte("hubos-test-marker")...)
		os.Remove(kp)
		os.WriteFile(kp, b, 0o644)
		sumOut, _ := exec.Command("sha256sum", kp).Output()
		recKernel10 = strings.Fields(string(sumOut))[0]
		rewrite(d, "manifest", func(m []byte) []byte {
			re := regexp.MustCompile(`(?m)^kernel-recovery\.efi sha256 [0-9a-f]+ size \d+$`)
			return re.ReplaceAll(m, []byte(fmt.Sprintf("kernel-recovery.efi sha256 %s size %d", recKernel10, len(b))))
		})
		resign(d)
	}
	// correctly signed manifests that list only ONE kernel
	variant("v2-onekernel-a", func(d string) { rewrite(d, "manifest", dropLine("kernel-b.efi ")); resign(d) })
	variant("v2-onekernel-b", func(d string) { rewrite(d, "manifest", dropLine("kernel-a.efi ")); resign(d) })
	variant("v2-badroot", func(d string) {
		rewrite(d, "rootfs.sqsh", func(b []byte) []byte { b[5000000] ^= 0xff; return b })
	})
	// the unsigned-by-owner "no kernel version" bundle is a correctly signed one (built above)

	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	go http.Serve(ln, http.FileServer(http.Dir(bdir)))
	r.base = fmt.Sprintf("http://10.0.2.2:%d", ln.Addr().(*net.TCPAddr).Port)
	defer ln.Close()

	// ---- the tests that need only files ----
	t.Run("systemd_programs_absent_from_the_root", func(t *testing.T) {
		start := time.Now()
		r.t = t
		root := filepath.Join(work, "root-extracted")
		os.RemoveAll(root)
		if out, err := exec.Command(r.tool("usr/bin/unsquashfs"), "-no-xattrs", "-d", root, filepath.Join(bdir, "v2-good", "rootfs.sqsh")).CombinedOutput(); err != nil {
			t.Fatalf("unsquashfs: %v\n%s", err, out)
		}
		defer os.RemoveAll(root)
		ok := checkNoSystemd(t, root)
		record("root has no systemd program, unit directory or banned package (and the apt pin was in the build)", ok, time.Since(start), "")
		start = time.Now()
		ok2 := checkStripped(t, root)
		record("stripped root: no apt, libapt, procps, libproc2, PAM modules, login, passwd file left; the two systemd libraries kept", ok2, time.Since(start), "")
		start = time.Now()
		ok3 := checkLibs(t, root)
		record("ldd finds no unresolved library in /usr, /bin, /sbin, /lib (and the checker fails when a library is removed)", ok3, time.Since(start), "")
		if !ok || !ok2 || !ok3 {
			t.Fail()
		}
	})

	// ---- the disk with release 1, and the first boot ----
	r.script("build-disk.sh", nil, filepath.Join(bdir, "v1-good", "rootfs.sqsh"), filepath.Join(work, "out", "kernel-a.efi"), r.disk)

	t.Run("T01_read_only_root", func(t *testing.T) {
		r.t = t
		start := time.Now()
		bt := time.Now()
		r.vm = r.bootVM("first boot")
		t.Logf("first boot to the handover line (including QEMU start): %.1f s", time.Since(bt).Seconds())
		_, out := r.sh(`touch /usr/x 2>&1; echo rc=$?; touch /etc/x 2>&1; echo rc=$?; echo ok > /var/y && echo var-writable; mount | grep " on / "`)
		ok := strings.Contains(out, "Read-only file system") && strings.Contains(out, "var-writable") && strings.Contains(out, "squashfs")
		record("1 read-only root", ok, time.Since(start), "")
		if !ok {
			t.Fail()
		}
	})

	t.Run("T02_first_boot_hubd_from_config_partition", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		r.vm.wait(`ensure|confirm:`, 20*time.Second, 0)
		time.Sleep(time.Second)
		rc1, out := r.sh(`hubos-ctl status | head -3; ls -la /config/hubos; hubd check --inventory /config/hubos/inventory.toml 2>&1 | tail -3`)
		rc2, list := r.sh(`hubd list --socket /run/hubos/hubd.sock | head -4`)
		_, pid1 := r.sh(`cat /proc/1/comm; ps | grep -c "[s]ystemd"; echo; ls /run/service`)
		_, nsd := r.sh(`n=$(cat /proc/[0-9]*/comm | grep -c '^systemd'); echo systemd-processes=$n`)
		noSd := strings.Contains(nsd, "systemd-processes=0")
		record("no process named systemd runs in the booted image", noSd, 0, "counted in /proc/*/comm")
		if !noSd {
			t.Errorf("a process named systemd runs: %s", nsd)
			t.Fail()
		}
		ok := rc1 == 0 && rc2 == 0 && strings.Contains(list, "Hub") && strings.Contains(out, "node.conf") && strings.HasPrefix(pid1, "s6-svscan")
		record("2 first boot; hubd runs from the config partition; PID 1 is s6-svscan, no systemd process", ok, time.Since(start), "")
		if !ok {
			t.Fail()
		}
		_, libs := r.sh(`find / -xdev \( -name 'libsystemd*' -o -name 'libudev*' \) 2>/dev/null | sort`)
		t.Logf("systemd libraries in the running root (the two that remain, by decision, for now):\n%s", libs)
		r.sh(`echo survive-me > /config/hubos/marker; sync`)
	})

	t.Run("T02b_slot_entries_have_no_load_options", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		_, ev := r.sh(`efibootmgr -v | grep -E "hubos|BootOrder"`)
		_, ord := r.sh(`efibootmgr | grep -E "^BootOrder|^Boot[0-9A-F]{4}\*? hubos" | sed 's/\t.*//'`)
		_, cl := r.sh(`cat /proc/cmdline; echo "slot file: $(cat /run/hubos-slot)"; head -1 /etc/hubos-release`)
		ok := true
		for _, sl := range []string{"hubos-a\\s.*kernel-a", "hubos-b\\s.*kernel-b", "hubos-recovery\\s.*kernel-recovery"} {
			re := regexp.MustCompile(`(?m)` + sl + `\.efi\)\s*$`)
			if !re.MatchString(ev) {
				t.Errorf("entry %q is missing, points elsewhere or has load options:\n%s", sl, ev)
				ok = false
			}
		}
		// BootOrder: slot a, slot b, then the recovery entry
		num := func(label string) string {
			m := regexp.MustCompile(`(?m)^Boot([0-9A-F]{4})\*? ` + label + `\s*$`).FindStringSubmatch(ord)
			if m == nil {
				return "?"
			}
			return m[1]
		}
		wantOrder := num("hubos-a") + "," + num("hubos-b") + "," + num("hubos-recovery")
		inOrder := strings.Contains(ord, "BootOrder: "+wantOrder)
		ok = ok && inOrder && strings.Contains(cl, "hubos.slot=a") && strings.Contains(cl, "root=PARTLABEL=hubos-root-a") && !strings.Contains(cl, "hubos.recovery") && strings.Contains(cl, "slot file: a")
		record("A1 the a, b and recovery boot entries are all created WITHOUT load options; the recovery entry points at kernel-recovery.efi and is in BootOrder after the two slots; slot a boots with its own kernel", ok, time.Since(start), "BootOrder "+wantOrder)
		if !ok {
			t.Errorf("inOrder=%v want %s\n%s", inOrder, wantOrder, ord)
			t.Fail()
		}
	})

	t.Run("T03_init_restarts_a_killed_service", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		_, out := r.sh(`OLD=$(s6-svstat /run/service/hubd | sed 's/.*pid \([0-9]*\).*/\1/'); echo old=$OLD; kill -9 $OLD; sleep 2; s6-svstat /run/service/hubd; hubd list --socket /run/hubos/hubd.sock >/dev/null && echo hubd-answers-again`)
		m := regexp.MustCompile(`old=(\d+)`).FindStringSubmatch(out)
		ok := m != nil && strings.Contains(out, "hubd-answers-again") && !strings.Contains(out, "up (pid "+m[1]+")")
		record("3 init restarts a killed service", ok, time.Since(start), "")
		if !ok {
			t.Fail()
		}
	})

	t.Run("T04_bad_bundles_are_refused", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		all := true
		slotB := `head -c 4194304 $(findfs PARTLABEL=hubos-root-b) | sha256sum`
		_, beforeB := r.sh(slotB)
		for _, c := range []struct{ name, want string }{
			{"v2-unsigned", "no signature"}, {"v2-badmanifest", "bad signature"}, {"v2-wrongkey", "bad signature"},
			{"v2-badroot", "tampered or damaged"}, {"v1-good", "not newer than"},
			{"v2-nokernelversion", "no kernel version line"},
			{"v2-onekernel-a", "manifest does not list kernel-b.efi"}, {"v2-onekernel-b", "manifest does not list kernel-a.efi"},
		} {
			rc, out := r.update(c.name)
			good := rc == 2 && strings.Contains(out, "REFUSED") && strings.Contains(out, c.want) && !strings.Contains(out, "writing to slot") && !strings.Contains(out, "wrote ") && (!strings.HasPrefix(c.name, "v2-onekernel") || !strings.Contains(out, "fetching rootfs"))
			if !good {
				t.Errorf("%s: expected REFUSED/%q, got:\n%s", c.name, c.want, out)
				all = false
			}
		}
		_, st := r.sh("hubos-ctl status")
		if strings.Contains(st, "BootNext") {
			t.Error("BootNext was set by a refused bundle")
			all = false
		}
		_, afterB := r.sh(slotB)
		if beforeB != afterB {
			t.Errorf("the other slot's root changed while only refused bundles were offered")
			all = false
		}
		record("4 unsigned, tampered, wrong-key, replayed/not-newer, no-kernel-version and ONE-kernel manifests refused; nothing written (slot b unchanged)", all, time.Since(start), "8 bundles")
	})

	t.Run("T05_signed_update_accepted", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		ut := time.Now()
		rc, out := r.update("v2-good")
		updSec := time.Since(ut).Seconds()
		pinned := strings.Contains(out, "kernel version 6.12")
		m := r.vm.mark()
		bt := time.Now()
		io.WriteString(r.vm.in, "sync; reboot -f\n")
		r.afterReset(m)
		rebootSec := time.Since(bt).Seconds()
		r.waitConfirmed()
		slot, rel, confirmed, _ := r.status()
		_, marker := r.sh(`cat /config/hubos/marker; cat /config/hubos/state/min_version`)
		_, old := r.update("v1-good")
		ok := rc == 0 && pinned && slot == "b" && rel == "2" && confirmed && strings.Contains(marker, "survive-me") && strings.Contains(marker, "2") && strings.Contains(old, "REFUSED")
		record("5 signed update accepted; config survives; floor raised; old bundle refused; kernel version in the manifest shown", ok, time.Since(start),
			fmt.Sprintf("update %.1f s, reboot to handover %.1f s", updSec, rebootSec))
		if !ok {
			t.Fail()
		}
	})

	t.Run("T05b_slot_b_boots_with_its_own_kernel", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		_, cl := r.sh(`cat /proc/cmdline; echo "slot file: $(cat /run/hubos-slot)"; efibootmgr | grep "^BootCurrent"`)
		ok := strings.Contains(cl, "hubos.slot=b") && strings.Contains(cl, "root=PARTLABEL=hubos-root-b") && !strings.Contains(cl, "hubos.slot=a") && strings.Contains(cl, "slot file: b")
		record("A1 slot b boots with its own kernel (kernel-b.efi installed by the update; /proc/cmdline shows hubos.slot=b and its own root)", ok, time.Since(start), "")
		if !ok {
			t.Fail()
		}
	})

	t.Run("T05c_recovery_after_an_update", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		atBoot := r.bootEntries()
		_, ev := r.sh(`efibootmgr -v | grep -E "hubos-(a|b|recovery)"`)
		kept := strings.Contains(atBoot, "hubos-recovery") && regexp.MustCompile(`(?m)hubos-recovery\s.*kernel-recovery\.efi\)\s*$`).MatchString(ev)
		record("R0 the recovery entry is in BootOrder, so it is still there when the firmware (OVMF) starts the next boot; nothing has to recreate it", kept, 0,
			fmt.Sprintf("entries present at the boot of slot b: [%s]; it points at kernel-recovery.efi", atBoot))
		if !kept {
			t.Errorf("atBoot=%q\n%s", atBoot, ev)
			t.Fail()
		}
		banner, out := r.bootRecoveryEntry()
		sig := recoverySig(out)
		r.recSig = sig
		okSig := banner && strings.Contains(sig, "version=recovery-1") && !strings.Contains(sig, "hubos.slot") && !strings.Contains(sig, "root=") && strings.Contains(out, "min_version=")
		t.Logf("recovery after the update to slot b: %s", sig)
		record("R1 recovery after an update: boots the separate recovery kernel (no slot, no root in its command line) and `hubos-ctl status` runs in its shell", okSig, time.Since(start), sig)
		if !okSig {
			t.Fail()
		}
		r.reboot()
		r.waitConfirmed()
	})

	badBoot := func(name, bundleName, expect string) {
		t.Run(name, func(t *testing.T) {
			r.t = t
			if r.dead {
				t.Skip("run stopped by a repeated hang")
			}
			start := time.Now()
			rc, _ := r.update(bundleName)
			m := r.vm.mark()
			io.WriteString(r.vm.in, "sync; reboot -f\n")
			trial := ""
			if strings.HasPrefix(name, "6c") {
				// the unconfirmed trial boot of slot a is up (hubd never becomes healthy): where does recovery point now?
				if hh := r.vm.wait(handoverRe, 150*time.Second, m); hh >= 0 {
					r.ready(r.vm)
					_, e := r.vm.sh(`efibootmgr -v | grep hubos-recovery | sed 's/.*File/File/'`, 30*time.Second)
					trial = regexp.MustCompile(`File\S+`).FindString(e)
					t.Logf("during the unconfirmed trial boot of slot a the recovery entry is: %s", trial)
				}
			}
			saw := r.vm.wait(expect, 200*time.Second, m)
			sawAfter := time.Since(start).Seconds()
			// back in the confirmed slot: a new handover line after the failure
			h := r.vm.wait(handoverRe, 300*time.Second, max(saw, m))
			if h >= 0 {
				r.bootPos = h
				r.ready(r.vm)
			} else {
				r.noteHang(r.vm, "rollback after "+name)
				r.vm.kill()
				r.vm = r.retryBoot("rollback after " + name)
			}
			r.waitConfirmed()
			slot, rel, confirmed, _ := r.status()
			ok := rc == 0 && saw >= 0 && slot == "b" && rel == "2" && confirmed
			note := fmt.Sprintf("failure line seen after %.1f s; rollback complete after %.1f s", sawAfter, time.Since(start).Seconds())
			if trial != "" {
				note += "; during the unconfirmed trial boot the recovery entry pointed at " + trial
			}
			record(name, ok, time.Since(start), note)
			if !ok {
				t.Fail()
			}
		})
	}
	badBoot("6a bad boot rolls back: signed bundle with no /sbin/init (stage 0 refuses it)", "v3-noinit", `STAGE0: slot a has no /sbin/init; rebooting`)
	t.Run("T06a2_recovery_after_a_rollback", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		// slot a now holds the bad release 3 (no /sbin/init) that was just rolled back from
		banner, out := r.bootRecoveryEntry()
		sig := recoverySig(out)
		if banner {
			r.reboot() // leave the recovery shell; the firmware then boots the confirmed slot
			r.waitConfirmed()
		}
		slot, rel, confirmed, _ := r.status()
		same := sig == r.recSig
		record("R2 recovery after a rollback boots the SAME recovery kernel as after the update, whatever slot a holds", banner && same && slot == "b" && rel == "2" && confirmed, time.Since(start),
			fmt.Sprintf("same as after the update: %v; afterwards slot %s release %s confirmed", same, slot, rel))
		if !banner || !same || slot != "b" || rel != "2" || !confirmed {
			t.Errorf("banner=%v sig=%q want %q", banner, sig, r.recSig)
			t.Fail()
		}
	})
	badBoot("6b bad boot rolls back: signed bundle whose root is garbage (stage 0 cannot mount it)", "v4-garbage", `STAGE0: cannot mount the root of slot a; rebooting`)
	badBoot("6c bad boot rolls back: boots but never gets healthy (confirm times out)", "v5-unhealthy", `this boot FAILED`)
	badBoot("6d bad boot rolls back: init hangs (the watchdog resets the machine)", "v6-hang", `STAGE0: switching to slot a`)

	t.Run("T07_recovery_mode", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		_, ev := r.sh(`efibootmgr -v | grep hubos-recovery`)
		noOpts := regexp.MustCompile(`(?m)File\(\\EFI\\hubos\\kernel-recovery\.efi\)\s*$`).MatchString(ev)
		banner, shell := r.bootRecoveryEntry()
		sig := recoverySig(shell)
		ok := banner && noOpts && sig == r.recSig && strings.Contains(shell, "RECOVERY MODE") && strings.Contains(shell, "min_version=") && strings.Contains(shell, "PID")
		record("7 recovery mode: the recovery entry has NO load options and boots the separate recovery kernel (banner, bare terminal, hubos-ctl status)", ok, time.Since(start), "after rollbacks and failed trials: still the same recovery boot")
		if !ok {
			t.Errorf("noOpts=%v banner=%v\n%s\n%s", noOpts, banner, ev, shell)
			t.Fail()
		}
		r.reboot()
		r.waitConfirmed()
	})

	// ---- the interrupted update, at known log lines ----
	t.Run("T08_interrupted_update", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		points := []struct{ name, line string }{
			{"during the download", `update: fetching rootfs.sqsh`},
			{"during the write to the other slot", `update: wrote 8 of \d+ MiB to slot a`},
			{"after the write, before the kernel is copied", `update: root written and verified`},
			{"after the kernel copy, before BootNext", `update: kernel installed`},
		}
		all := true
		for _, p := range points {
			ptStart := time.Now()
			m := r.vm.mark()
			fmt.Fprintf(r.vm.in, "hubos-ctl update %s/v7-good\n", r.base) // no redirect: the lines must appear as they happen
			e := r.vm.wait(p.line, 200*time.Second, m)
			if e < 0 {
				t.Errorf("%s: the line %q never appeared", p.name, p.line)
				all = false
				continue
			}
			t.Logf("interrupt %s: killing QEMU at the line %q (%.1f s into the update)", p.name, p.line, time.Since(ptStart).Seconds())
			r.vm.kill()
			r.vm = r.bootVM("start after the interruption " + p.name)
			r.waitConfirmed()
			slot, rel, confirmed, st := r.status()
			good := slot == "b" && rel == "2" && confirmed && !strings.Contains(st, "BootNext")
			t.Logf("after the interruption %s: slot=%s release=%s confirmed=%v", p.name, slot, rel, confirmed)
			if !good {
				t.Errorf("%s: the confirmed slot did not boot cleanly: %s", p.name, st)
				all = false
			}
		}
		// the update still works afterwards, and the machine ends up on release 7 in slot a
		rc, out := r.update("v7-good")
		m := r.vm.mark()
		io.WriteString(r.vm.in, "sync; reboot -f\n")
		r.afterReset(m)
		r.waitConfirmed()
		slot, rel, confirmed, _ := r.status()
		if rc != 0 || slot != "a" || rel != "7" || !confirmed {
			t.Errorf("a clean update after the interruptions failed: rc=%d slot=%s rel=%s confirmed=%v\n%s", rc, slot, rel, confirmed, out)
			all = false
		}
		record("8 an update interrupted at 4 known log lines never harms the confirmed slot; a clean update still works", all, time.Since(start), "kill -9 of QEMU at each line")
		if !all {
			t.Fail()
		}
	})

	// ---- not enough free space on the boot partition ----
	t.Run("T10_no_free_space_on_the_boot_partition", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		sz := fileSize(t, filepath.Join(bdir, "v2-good", "rootfs.sqsh"))
		esp := `mount -t vfat $(findfs PARTLABEL=hubos-esp) /boot/efi`
		rootSum := fmt.Sprintf(`head -c %d $(findfs PARTLABEL=hubos-root-b) | sha256sum`, sz)
		_, before := r.sh(esp + `; sha256sum /boot/efi/EFI/hubos/kernel-b.efi; ` + rootSum + `; dd if=/dev/zero of=/boot/efi/FILL bs=1M 2>&1 | tail -n 1; sync; df -k /boot/efi | tail -n 1; umount /boot/efi`)
		rc, out := r.update("v8-good")
		_, after := r.sh(esp + `; sha256sum /boot/efi/EFI/hubos/kernel-b.efi; ls /boot/efi/EFI/hubos; ` + rootSum + `; rm -f /boot/efi/FILL; sync; umount /boot/efi; hubos-ctl status | grep -c BootNext`)
		hashes := func(s string) []string { return regexp.MustCompile(`[0-9a-f]{64}`).FindAllString(s, -1) }
		hb, ha := hashes(before), hashes(after)
		clean := rc == 2 && strings.Contains(out, "not enough free space") && strings.Contains(out, "nothing was written") && !strings.Contains(out, "wrote ") && !strings.Contains(out, "kernel installed")
		same := len(hb) == 2 && len(ha) == 2 && hb[0] == ha[0] && hb[1] == ha[1]
		noTemp := !strings.Contains(after, ".new")
		noNext := regexp.MustCompile(`(?m)^0\s*$`).MatchString(after) // the last command printed the number of BootNext lines
		t.Logf("kernel-b.efi and slot b's root unchanged: %v; no .new file: %v; no BootNext: %v", same, noTemp, noNext)
		// the old kernel still boots: BootNext to slot b (the slot whose kernel the failed update meant to replace)
		r.sh(`efibootmgr -q -n $(efibootmgr | sed -n 's/^Boot\([0-9A-F]*\)\*\{0,1\}[[:space:]]hubos-b.*/\1/p' | head -1)`)
		m := r.vm.mark()
		io.WriteString(r.vm.in, "sync; reboot -f\n")
		r.afterReset(m)
		// the older slot (release 2) is below the floor 7: its own confirm step must REFUSE
		refusedAuto := r.vm.wait(`confirm: REFUSED`, 90*time.Second, r.bootPos) >= 0
		slot, rel, confirmed, st := r.status()
		_, cl := r.sh(`cat /proc/cmdline`)
		boots := slot == "b" && rel == "2" && strings.Contains(cl, "hubos.slot=b")
		rcC, outC := r.sh(`hubos-ctl confirm`)
		refuses := refusedAuto && rcC == 2 && strings.Contains(outC, "below the floor 7") && !confirmed && !strings.Contains(outC, "confirmed;")
		record("B1 confirm refuses a slot whose release (2) is below the floor (7): message, exit 2, BootOrder and floor unchanged", refuses && boots, time.Since(start), fmt.Sprintf("automatic confirm step refused: %v; manual confirm rc=%d; slot b first in BootOrder: %v", refusedAuto, rcC, confirmed))
		if !refuses {
			t.Errorf("refusedAuto=%v rc=%d confirmed=%v\n%s\n%s", refusedAuto, rcC, confirmed, outC, st)
		}
		_, fl := r.sh(`cat /config/hubos/state/min_version`)
		floorKept := regexp.MustCompile(`(?m)^7\s*$`).MatchString(fl)
		rc5, out5 := r.update("v5-unhealthy")
		belowFloor := rc5 == 2 && strings.Contains(out5, "below the floor 7")
		record("A the floor only goes up: the older slot (release 2) booted by hand after the rollbacks; the floor stays 7 and release 5 is refused", floorKept && belowFloor && boots, time.Since(start), "floor "+regexp.MustCompile(`\d+`).FindString(fl))
		if !floorKept || !belowFloor {
			t.Errorf("floor=%q belowFloor=%v\n%s", fl, belowFloor, out5)
		}
		ok := clean && same && noTemp && noNext && boots
		record("B update with a full boot partition fails cleanly; kernel-b.efi and slot b's root unchanged; the old kernel still boots", ok, time.Since(start), fmt.Sprintf("rc=%d; old kernel booted slot %s release %s", rc, slot, rel))
		if !ok {
			t.Errorf("clean=%v same=%v noTemp=%v noNext=%v boots=%v\nout:\n%s", clean, same, noTemp, noNext, boots, out)
		}
	})

	t.Run("T11_rollback_command", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		// state: slot b (release 2) runs, below the floor 7, slot a is first in BootOrder
		rc0, out0 := r.sh(`hubos-ctl rollback`)
		usage := rc0 == 2 && strings.Contains(out0, "usage")
		rc, out := r.sh(`hubos-ctl rollback "test: go back to release 2 on purpose"`)
		slot, rel, confirmed, st := r.status()
		_, lg := r.sh(`cat /config/hubos/state/rollback.log`)
		floor2 := regexp.MustCompile(`(?m)^min_version=2$`).MatchString(st)
		logged := strings.Contains(lg, "floor 7 -> 2") && strings.Contains(lg, "test: go back to release 2 on purpose") && strings.Contains(lg, "slot b release 2")
		ok := usage && rc == 0 && strings.Contains(out, "floor 7 -> 2") && confirmed && slot == "b" && rel == "2" && floor2 && logged
		record("B2 hubos-ctl rollback REASON: refuses without a reason; logs old and new floor and the reason; floor 7 -> 2; the running slot is first in BootOrder", ok, time.Since(start), fmt.Sprintf("rc=%d; log: %s", rc, regexp.MustCompile(`(?m)^.*rollback: slot.*$`).FindString(lg)))
		if !ok {
			t.Errorf("usage=%v rc=%d confirmed=%v floor2=%v logged=%v\n%s\n%s\n%s", usage, rc, confirmed, floor2, logged, out, st, lg)
			t.Fail()
		}
	})

	t.Run("T12_recovery_kernel_updates", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		espInfo := `mount -t vfat $(findfs PARTLABEL=hubos-esp) /boot/efi; echo ver=$(cat /boot/efi/EFI/hubos/recovery.version); sha256sum /boot/efi/EFI/hubos/kernel-recovery.efi; umount /boot/efi`
		parse := func(o string) (string, string) {
			v := regexp.MustCompile(`ver=(\d+)`).FindStringSubmatch(o)
			h := regexp.MustCompile(`[0-9a-f]{64}`).FindString(o)
			if v == nil {
				return "?", h
			}
			return v[1], h
		}
		_, o0 := r.sh(espInfo)
		v0, h0 := parse(o0)
		// 1. a manifest without a recovery kernel: the update works and recovery is left alone
		rc1, u1 := r.update("v9-norecovery")
		_, o1 := r.sh(espInfo)
		v1, h1 := parse(o1)
		ok1 := rc1 == 0 && strings.Contains(u1, "the manifest has no recovery kernel") && v1 == v0 && h1 == h0
		// 2. a newer recovery version: installed
		rc2, u2 := r.update("v10-recovery2")
		_, o2 := r.sh(espInfo)
		v2, h2 := parse(o2)
		ok2 := rc2 == 0 && strings.Contains(u2, "recovery kernel version 2 is newer than the installed 1") && strings.Contains(u2, "recovery kernel version 2 installed") && v2 == "2" && h2 == recKernel10 && h2 != h0
		// 3. the same recovery version again: left alone
		rc3, u3 := r.update("v11-recovery2-again")
		_, o3 := r.sh(espInfo)
		v3, h3 := parse(o3)
		ok3 := rc3 == 0 && strings.Contains(u3, "not newer than the installed 2; left alone") && v3 == "2" && h3 == h2
		t.Logf("recovery version/hash: start %s %s; after no-recovery bundle %s %s; after version 2 %s %s; after version 2 again %s %s", v0, h0[:12], v1, h1[:12], v2, h2[:12], v3, h3[:12])
		record("C1 recovery kernel updates: a manifest without one works and leaves recovery alone; a newer recovery-version installs the new file; the same version is left alone", ok1 && ok2 && ok3, time.Since(start), fmt.Sprintf("installed version %s -> %s -> %s -> %s", v0, v1, v2, v3))
		if !ok1 || !ok2 || !ok3 {
			t.Errorf("ok1=%v ok2=%v ok3=%v\n%s\n%s\n%s", ok1, ok2, ok3, u1, u2, u3)
			t.Fail()
		}
		// boot the last update (release 11 in slot a), then recovery with the NEW recovery kernel
		m := r.vm.mark()
		io.WriteString(r.vm.in, "sync; reboot -f\n")
		r.afterReset(m)
		r.waitConfirmed()
		slot, rel, confirmed, _ := r.status()
		banner, out := r.bootRecoveryEntry()
		same := recoverySig(out) == r.recSig
		record("C2 after the recovery kernel update the machine boots release 11 in slot a, and recovery (the new file) boots the same way", slot == "a" && rel == "11" && confirmed && banner && same, time.Since(start), fmt.Sprintf("slot %s release %s; recovery banner %v; same as before %v", slot, rel, banner, same))
		if slot != "a" || rel != "11" || !confirmed || !banner || !same {
			t.Fail()
		}
		r.reboot()
		r.waitConfirmed()
	})

	t.Run("T13_recovery_with_both_roots_garbage", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		// destroy both slot roots (the first 4 MiB of each) and reboot into recovery without touching the disk again:
		// the reboot is a sysrq reboot, which needs no file from the (now broken) root.
		r.sh(`efibootmgr -q -n $(efibootmgr | sed -n 's/^Boot\([0-9A-F]*\)\*\{0,1\}[[:space:]]hubos-recovery[[:space:]].*/\1/p' | head -n 1)`)
		r.sh(`dd if=/dev/zero of=$(findfs PARTLABEL=hubos-root-a) bs=1M count=4 conv=fsync 2>&1 | tail -n 1; dd if=/dev/zero of=$(findfs PARTLABEL=hubos-root-b) bs=1M count=4 conv=fsync 2>&1 | tail -n 1; sync`)
		m := r.vm.mark()
		io.WriteString(r.vm.in, "echo b > /proc/sysrq-trigger\n")
		banner := r.waitRecovery(r.vm, m)
		var shell string
		if banner {
			_, shell = r.vm.sh(recoveryCmds, 60*time.Second)
			t.Logf("recovery shell with both roots garbage:\n%s", shell)
		}
		sig := recoverySig(shell)
		_, tools := r.sh(`e2fsck -fn $(findfs PARTLABEL=hubos-config) 2>&1 | tail -n 2; blkid | sed 's/ UUID.*PARTLABEL/ PARTLABEL/' | head -n 8; ip -4 addr show eth0 | grep inet; wget -q -O /tmp/m ` + r.base + `/v1-good/manifest && echo wget-ok $(wc -c < /tmp/m) bytes; signify-openbsd 2>&1 | head -n 1; efibootmgr | head -n 2`)
		okTools := strings.Contains(tools, "clean") && strings.Contains(tools, "wget-ok") && strings.Contains(tools, "inet ")
		ok := banner && sig == r.recSig && okTools
		record("C3 recovery boots with BOTH slot roots garbage, the same way as before; its shell runs hubos-ctl status, e2fsck, findfs/blkid, ip and wget (it fetched a file from the host)", ok, time.Since(start), "sig: "+sig)
		if !ok {
			t.Errorf("banner=%v sig=%q want %q\n%s", banner, sig, r.recSig, tools)
			t.Fail()
		}
	})

	t.Run("T14_firmware_fallthrough_experiment", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		// We are in the recovery shell; both roots are garbage. Experiment: the first BootOrder entry is a kernel
		// that loads and then reboots at once (stage 0 cannot mount its root). Does the firmware ever reach the
		// second or third entry? Then: the first entry's FILE is missing (the loader cannot load it).
		starts := func(txt string) map[string]int {
			c := map[string]int{}
			for _, m := range regexp.MustCompile(`BdsDxe: starting Boot[0-9A-F]{4} "([^"]+)"`).FindAllStringSubmatch(txt, -1) {
				c[m[1]]++
			}
			return c
		}
		fails := func(txt string) map[string]int {
			c := map[string]int{}
			for _, m := range regexp.MustCompile(`BdsDxe: failed to load Boot[0-9A-F]{4} "([^"]+)"`).FindAllStringSubmatch(txt, -1) {
				c[m[1]]++
			}
			return c
		}
		setNext := `efibootmgr -q -n $(efibootmgr | sed -n 's/^Boot\([0-9A-F]*\)\*\{0,1\}[[:space:]]hubos-recovery[[:space:]].*/\1/p' | head -n 1)`
		r.sh(setNext + `; sync`)
		// snapshot of the disk and the variable store, with BootNext = recovery, to come back to a shell later
		r.vm.kill()
		snapD, snapV := filepath.Join(r.disk, "disk.snap"), filepath.Join(r.disk, "vars.snap")
		copyFile(t, filepath.Join(r.disk, "disk.img"), snapD)
		copyFile(t, filepath.Join(r.disk, "vars.fd"), snapV)
		observe := func(v *vm, d time.Duration) string {
			m := v.mark()
			io.WriteString(v.in, "sync; reboot -f\n")
			time.Sleep(d)
			return v.text(m)
		}
		// experiment 1: all files present, boot order a, b, recovery; a loads and fails
		v := r.startRecovery()
		txt1 := observe(v, 100*time.Second)
		st1, fl1 := starts(txt1), fails(txt1)
		v.kill()
		t.Logf("experiment 1 (first entry loads, then reboots at once): starts %v, load failures %v", st1, fl1)
		// experiment 2: restore, remove the first entry's FILE, same order
		copyFile(t, snapD, filepath.Join(r.disk, "disk.img"))
		copyFile(t, snapV, filepath.Join(r.disk, "vars.fd"))
		v = r.startRecovery()
		v.sh(`mount -t vfat $(findfs PARTLABEL=hubos-esp) /boot/efi; rm /boot/efi/EFI/hubos/kernel-a.efi; umount /boot/efi; `+setNext+`; sync`, 30*time.Second)
		m := v.mark()
		io.WriteString(v.in, "reboot -f\n")
		if !r.waitRecovery(v, m) { // BootNext boot (consumed here)
			t.Fatalf("no recovery shell after BootNext")
		}
		txt2 := observe(v, 100*time.Second)
		st2, fl2 := starts(txt2), fails(txt2)
		t.Logf("experiment 2 (first entry's file missing): starts %v, load failures %v", st2, fl2)
		t1 := st1["hubos-a"] >= 2 && st1["hubos-b"] == 0 && st1["hubos-recovery"] == 0
		t2 := fl2["hubos-a"] >= 1 && st2["hubos-b"] >= 1 && st2["hubos-recovery"] == 0
		record("C4 OVMF: a first entry that LOADS and then reboots at once is retried forever; the second and third entries are never reached", t1, time.Since(start), fmt.Sprintf("in 100 s: starts %v, load failures %v", st1, fl1))
		record("C5 OVMF: a first entry whose FILE is missing falls through to the second; after the second loads and fails, the third (recovery) is still never reached", t2, time.Since(start), fmt.Sprintf("in 100 s: starts %v, load failures %v", st2, fl2))
		if !t1 || !t2 {
			t.Errorf("t1=%v t2=%v\n%s\n---\n%s", t1, t2, firstLines(txt1, 6), firstLines(txt2, 6))
			t.Fail()
		}
	})

	// ---- reproducibility (files only) ----
	t.Run("T09_reproducible_builds", func(t *testing.T) {
		r.t = t
		start := time.Now()
		sum := func(p string) string {
			out, _ := exec.Command("sha256sum", p).Output()
			return strings.Fields(string(out))[0]
		}
		// the root tar: two builds into two files
		first := sum(filepath.Join(work, "out", "base.tar"))
		r.script("build-base.sh", nil, filepath.Join(work, "out", "base2.tar"))
		second := sum(filepath.Join(work, "out", "base2.tar"))
		t.Logf("root tar: build 1 %s, build 2 %s -> identical: %v", first, second, first == second)
		if first != second {
			t.Errorf("the root tar is not reproducible")
		}
		// the squashfs: the same inputs twice
		a, b := filepath.Join(work, "out", "root-a.sqsh"), filepath.Join(work, "out", "root-b.sqsh")
		r.script("build-root-image.sh", nil, "2", "good", a, r.pub)
		r.script("build-root-image.sh", nil, "2", "good", b, r.pub)
		sa, sb := sum(a), sum(b)
		t.Logf("squashfs root: %s / %s -> identical: %v", sa, sb, sa == sb)
		if sa != sb {
			out, _ := exec.Command("cmp", a, b).CombinedOutput()
			t.Logf("squashfs differs: %s", strings.TrimSpace(string(out)))
		}
		// the kernels (one per slot): a second full build with the same identity
		ka1, kb1 := sum(filepath.Join(work, "out", "kernel-a.efi")), sum(filepath.Join(work, "out", "kernel-b.efi"))
		os.Rename(filepath.Join(work, "out", "kernel-a.efi"), filepath.Join(work, "out", "kernel-a-first.efi"))
		os.Rename(filepath.Join(work, "out", "kernel-b.efi"), filepath.Join(work, "out", "kernel-b-first.efi"))
		r.script("build-kernel.sh", []string{"FORCE=1"})
		ka2, kb2 := sum(filepath.Join(work, "out", "kernel-a.efi")), sum(filepath.Join(work, "out", "kernel-b.efi"))
		k1, k2 := ka1+kb1, ka2+kb2
		t.Logf("kernel a: %s / %s -> identical: %v; kernel b: %s / %s -> identical: %v; a differs from b: %v", ka1, ka2, ka1 == ka2, kb1, kb2, kb1 == kb2, ka1 != kb1)
		if k1 != k2 {
			out, _ := exec.Command("cmp", filepath.Join(work, "out", "kernel-a-first.efi"), filepath.Join(work, "out", "kernel-a.efi")).CombinedOutput()
			t.Logf("kernel a differs: %s", strings.TrimSpace(string(out)))
		}
		record("9 two builds of the root give identical hashes (root tar asserted; squashfs and both kernels reported)", first == second, time.Since(start),
			fmt.Sprintf("tar %v, squashfs %v, kernels %v", first == second, sa == sb, k1 == k2))
	})
}

// checkNoSystemd looks in the unpacked root for systemd programs and unit directories and lists the
// systemd libraries that remain.
func checkNoSystemd(t *testing.T, root string) bool {
	ok := true
	var libs, bad, named []string
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel := strings.TrimPrefix(p, root)
		n := info.Name()
		switch {
		case (strings.HasPrefix(n, "libsystemd") || strings.HasPrefix(n, "libudev")) && strings.Contains(n, ".so"):
			libs = append(libs, rel)
		case !info.IsDir() && (n == "systemd" || strings.HasPrefix(n, "systemd-") || n == "systemctl" || n == "journalctl" || n == "udevadm" || n == "udevd"):
			bad = append(bad, "program "+rel)
		case info.IsDir() && (rel == "/etc/systemd" || rel == "/usr/lib/systemd" || rel == "/lib/systemd" || rel == "/etc/udev" || rel == "/usr/lib/udev" || rel == "/lib/udev"):
			bad = append(bad, "directory "+rel)
		case strings.HasSuffix(n, ".service") || strings.HasSuffix(n, ".socket") || strings.HasSuffix(n, ".target") || strings.HasSuffix(n, ".timer"):
			bad = append(bad, "unit "+rel)
		case !info.IsDir() && strings.Contains(n, "systemd") && !strings.Contains(rel, "/usr/share/doc") && !strings.Contains(rel, "/var/lib/dpkg/info"):
			named = append(named, rel)
		}
		return nil
	})
	// packages that must never be installed (the apt pin of image/apt/no-systemd.pref) and the pin itself
	banned := []string{"systemd", "systemd-sysv", "libpam-systemd", "dbus-user-session", "udev", "systemd-timesyncd", "systemd-resolved"}
	if st, err := os.ReadFile(filepath.Join(root, "var/lib/dpkg/status")); err == nil {
		for _, stanza := range strings.Split(string(st), "\n\n") {
			var pkg string
			installed := false
			for _, l := range strings.Split(stanza, "\n") {
				if v, ok := strings.CutPrefix(l, "Package: "); ok {
					pkg = v
				}
				if l == "Status: install ok installed" {
					installed = true
				}
			}
			for _, b := range banned {
				if installed && pkg == b {
					bad = append(bad, "installed package "+pkg)
				}
			}
		}
	} else {
		bad = append(bad, "no dpkg status file to check")
	}
	pin, err1 := os.ReadFile(filepath.Join("..", "..", "image", "apt", "no-systemd.pref"))
	inRoot, err2 := os.ReadFile(filepath.Join(root, "etc/apt/preferences.d/hubos-no-systemd"))
	if err1 != nil || err2 != nil || !bytes.Equal(pin, inRoot) || !strings.Contains(string(pin), "Pin-Priority: -1") {
		bad = append(bad, "the apt pin file was not in the build (image/apt/no-systemd.pref vs /etc/apt/preferences.d/hubos-no-systemd)")
	}
	for _, b := range banned {
		if !strings.Contains(string(pin), b) {
			bad = append(bad, "the pin does not list "+b)
		}
	}
	for _, b := range bad {
		t.Errorf("systemd found in the root: %s", b)
		ok = false
	}
	t.Logf("systemd libraries that remain in the root (%d files): %s", len(libs), strings.Join(libs, " "))
	t.Logf("other files with systemd in their name (not systemd programs; allowed, listed for the record): %s", strings.Join(named, " "))
	return ok
}

func fileSize(t *testing.T, p string) int64 {
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return fi.Size()
}

// removedPackages are deleted from every finished image after the build (tools/image/strip-root.sh).
var removedPackages = []string{"apt", "libapt-pkg6.0t64", "procps", "libproc2-0", "libpam-runtime", "libpam-modules", "libpam-modules-bin", "login", "passwd"}

// checkStripped checks that no file of the removed packages (from dpkg's own lists) is left, and that the two
// systemd libraries (kept by the owner's decision) are.
func checkStripped(t *testing.T, root string) bool {
	ok := true
	total := 0
	for _, pkg := range removedPackages {
		l, err := os.ReadFile(filepath.Join(root, "var/lib/dpkg/info", pkg+".list"))
		if err != nil { // multi-arch packages: PACKAGE:amd64.list
			l, err = os.ReadFile(filepath.Join(root, "var/lib/dpkg/info", pkg+":amd64.list"))
		}
		if err != nil {
			// not installed at all (mmdebstrap's minbase root has no apt): fine, but then it must not be in the status file
			if st, _ := os.ReadFile(filepath.Join(root, "var/lib/dpkg/status")); strings.Contains(string(st), "\nPackage: "+pkg+"\n") || strings.HasPrefix(string(st), "Package: "+pkg+"\n") {
				t.Errorf("%s is in the dpkg status but has no file list: %v", pkg, err)
				ok = false
			} else {
				t.Logf("%s is not installed in this root (nothing to remove)", pkg)
			}
			continue
		}
		for _, f := range strings.Split(strings.TrimSpace(string(l)), "\n") {
			fi, err := os.Lstat(filepath.Join(root, f))
			if err != nil {
				continue
			}
			if !fi.IsDir() {
				t.Errorf("a file of the removed package %s is still in the root: %s", pkg, f)
				ok = false
			}
			total++
		}
	}
	t.Logf("removed packages: %s (their listed files are gone; %d listed directories remain)", strings.Join(removedPackages, ", "), total)
	for _, lib := range []string{"usr/lib/x86_64-linux-gnu/libsystemd.so.0", "usr/lib/x86_64-linux-gnu/libudev.so.1"} {
		if _, err := os.Lstat(filepath.Join(root, lib)); err != nil {
			t.Errorf("the library %s should still be in the root (kept by decision): %v", lib, err)
			ok = false
		}
	}
	return ok
}

// checkLibs runs tools/image/check-libs.sh (ldd inside the root) and then shows that it can fail: it removes a
// library from the unpacked copy and expects the check to report it.
func checkLibs(t *testing.T, root string) bool {
	script := filepath.Join("..", "..", "tools", "image", "check-libs.sh")
	out, err := exec.Command(script, root).CombinedOutput()
	t.Logf("check-libs.sh on the built root: %s", strings.TrimSpace(string(out)))
	if err != nil {
		t.Errorf("unresolved libraries in the built root: %v\n%s", err, out)
		return false
	}
	libs, _ := filepath.Glob(filepath.Join(root, "usr/lib/x86_64-linux-gnu/libmount.so.1*"))
	if len(libs) == 0 {
		t.Errorf("the negative check needs libmount, which is not in the root")
		return false
	}
	for _, l := range libs {
		os.Remove(l)
	}
	out, err = exec.Command(script, root).CombinedOutput()
	t.Logf("check-libs.sh after removing libmount (must fail): rc!=0 is %v; first lines: %s", err != nil, firstLines(string(out), 3))
	if err == nil || !strings.Contains(string(out), "libmount.so.1 => not found") {
		t.Errorf("the library check did not notice a removed library")
		return false
	}
	return true
}

func firstLines(s string, n int) string {
	l := strings.Split(strings.TrimSpace(s), "\n")
	if len(l) > n {
		l = l[:n]
	}
	return strings.Join(l, " | ")
}
