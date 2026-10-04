//go:build qemu

package image

// The hub image test (image/machines/hub.build): boots the HUB image under QEMU with a virtio GPU, a USB keyboard and a USB
// mouse and drives it over the serial console and the QEMU monitor (screendump, mouse, keys). Run together with TestImage:
//   go test -tags qemu -count=1 -timeout 150m -v ./tools/image
// HUBOS_HUB_SHOTS = a folder to keep the PNG screenshots (default: inside the temporary directory, deleted afterwards).
// Everything is built from source or from the pinned snapshot; keys are made at run time and never stored.

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

const hubEnv = "XDG_RUNTIME_DIR=/run/dw WAYLAND_DISPLAY=wayland-1 HOME=/run/hub LC_ALL=C.UTF-8 DBUS_SESSION_BUS_ADDRESS=unix:path=/run/dw/bus"
const hubSock = "--socket /run/hubos/hubd.sock"

// asHub runs a command as the desktop user on the guest.
func asHub(c string) string { return "s6-setuidgid hub env " + hubEnv + " " + c }

// monitorConn is the one connection to the QEMU monitor (the monitor serves one client at a time); it is made again after
// every start of the VM.
var monitorConn net.Conn
var monitorFor *vm

// monitor sends one command to the QEMU monitor and returns its answer.
func (r *rig) monitor(cmd string) string {
	if monitorConn == nil || monitorFor != r.vm {
		if monitorConn != nil {
			monitorConn.Close()
		}
		var err error
		for i := 0; i < 50; i++ {
			if monitorConn, err = net.Dial("unix", r.monPath); err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if err != nil {
			r.t.Fatalf("monitor: %v", err)
		}
		monitorFor = r.vm
		r.readMonitor()
	}
	monitorConn.SetDeadline(time.Now().Add(30 * time.Second))
	io.WriteString(monitorConn, cmd+"\n")
	return r.readMonitor()
}

func (r *rig) readMonitor() string {
	buf := make([]byte, 65536)
	var out []byte
	for {
		n, err := monitorConn.Read(buf)
		out = append(out, buf[:n]...)
		if bytes.HasSuffix(bytes.TrimRight(out, " \r\n"), []byte("(qemu)")) || err != nil {
			return string(out)
		}
	}
}

// shot saves a screenshot of the guest's display as a PNG and returns its path.
func (r *rig) shot(name string) string {
	path := filepath.Join(r.shots, name+".png")
	r.monitor("screendump " + path + " -f png")
	for i := 0; i < 20; i++ {
		if fi, err := os.Stat(path); err == nil && fi.Size() > 0 {
			time.Sleep(200 * time.Millisecond)
			return path
		}
		time.Sleep(200 * time.Millisecond)
	}
	r.t.Fatalf("no screenshot %s", path)
	return ""
}

func loadPNG(t *testing.T, path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return img
}

// countColor counts the pixels of the region that are near the colour.
func countColor(img image.Image, x0, y0, x1, y1 int, want color.RGBA) int {
	n := 0
	near := func(a, b uint8) bool { d := int(a) - int(b); return d > -6 && d < 6 }
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if near(uint8(r>>8), want.R) && near(uint8(g>>8), want.G) && near(uint8(b>>8), want.B) {
				n++
			}
		}
	}
	return n
}

// findCursor finds the tip of the mouse arrow (white, with a vertical left edge at least 13 px tall) in the part of the screen
// to the right of the menu and below the bar, where the dot grid's 2-pixel dots cannot be mistaken for it.
func findCursor(img image.Image) (x, y int, found bool) {
	white := func(x, y int) bool {
		r, g, b, _ := img.At(x, y).RGBA()
		return r>>8 > 245 && g>>8 > 245 && b>>8 > 245
	}
	for yy := 31; yy < 600; yy++ {
		for xx := 669; xx < 1020; xx++ {
			if white(xx, yy) && white(xx, yy+5) && white(xx, yy+9) && white(xx, yy+13) {
				return xx, yy, true
			}
		}
	}
	return 0, 0, false
}

var (
	barAlert = color.RGBA{0x8a, 0x1c, 0x1c, 255} // the red box of hubd's alert (overlay waybar.css)
	barBg    = color.RGBA{0x1d, 0x22, 0x30, 255} // the bar's background
	menuBg   = color.RGBA{0xf6, 0xf5, 0xf4, 255} // wofi's list background (default style)
)

// barDrawn: a 30 px bar across the top (background colour over most of the top row band), with the red alert box at its left.
func barDrawn(img image.Image) (bg, alert int) {
	return countColor(img, 0, 0, 1024, 30, barBg), countColor(img, 0, 0, 120, 30, barAlert)
}

func TestHubImage(t *testing.T) {
	work := os.Getenv("HUBOS_HUB_WORK") // keep the (long) build between runs, for development
	if work == "" {
		var err error
		if work, err = os.MkdirTemp("", "hubos-hub-"); err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(work)
	}
	os.MkdirAll(work, 0o755)
	r := &rig{t: t, work: work, disk: filepath.Join(work, "vm"), hangDir: filepath.Join(work, "hangs"), machine: "image/machines/hub.build",
		gui: true, monPath: filepath.Join(work, "mon.sock"), shots: os.Getenv("HUBOS_HUB_SHOTS")}
	if r.shots == "" {
		r.shots = filepath.Join(work, "shots")
	}
	os.MkdirAll(r.shots, 0o755)
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
		t.Logf("\nRESULTS OF THE HUB IMAGE (QEMU hangs seen and retried: %d; every hang: %s; QEMU crashes (died from a signal) seen and retried: %d; every crash: %s)\n%s", r.hangs, hl, r.crashes, crashList(r), strings.Join(results, "\n"))
		if r.vm != nil {
			r.vm.kill()
		}
	}()

	// ---- build ----
	r.script("fetch-tools.sh", nil)
	kdir := filepath.Join(work, "keys")
	os.RemoveAll(kdir)
	os.MkdirAll(kdir, 0o700)
	os.Setenv("LD_LIBRARY_PATH", filepath.Join(work, "tools/root/usr/lib/x86_64-linux-gnu")+":"+filepath.Join(work, "tools/root/lib/x86_64-linux-gnu"))
	r.pub, r.sec = filepath.Join(kdir, "update.pub"), filepath.Join(kdir, "update.sec")
	if out, err := exec.Command(r.tool("bin/signify-openbsd"), "-G", "-n", "-p", r.pub, "-s", r.sec, "-c", "throwaway hub key").CombinedOutput(); err != nil {
		t.Fatalf("signify -G: %v\n%s", err, out)
	}
	r.script("build-kernel.sh", []string{"UPDATE_PUB=" + r.pub})
	r.script("build-base.sh", nil)
	r.script("build-hub-parts.sh", nil)
	bdir := filepath.Join(work, "bundles")
	os.RemoveAll(bdir)
	os.MkdirAll(bdir, 0o755)
	for _, b := range []struct{ v, f string }{{"1", "good"}, {"2", "good"}, {"3", "unhealthy"}} {
		r.script("build-bundle.sh", nil, b.v, b.f, filepath.Join(bdir, "h"+b.v+"-"+b.f), r.pub, r.sec)
	}
	// the fake machine "ai-1" (a listener on 127.0.0.12:21002) is the repo's test helper, served to the guest over HTTP; it is not in the image
	if out, err := exec.Command("go", "build", "-o", filepath.Join(bdir, "fakenode"), "../fakenode").CombinedOutput(); err != nil {
		t.Fatalf("go build fakenode: %v\n%s", err, out)
	}
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	go http.Serve(ln, http.FileServer(http.Dir(bdir)))
	r.base = fmt.Sprintf("http://10.0.2.2:%d", ln.Addr().(*net.TCPAddr).Port)
	defer ln.Close()

	// ---- files ----
	t.Run("H0_the_hub_root", func(t *testing.T) {
		r.t = t
		start := time.Now()
		root := filepath.Join(work, "hub-root")
		if out, err := exec.Command(r.tool("usr/bin/unsquashfs"), "-no-xattrs", "-d", root, filepath.Join(bdir, "h1-good", "rootfs.sqsh")).CombinedOutput(); err != nil {
			t.Fatalf("unsquashfs: %v\n%s", err, out)
		}
		defer os.RemoveAll(root)
		ok := checkNoSystemd(t, root, true)
		lu, _ := os.Readlink(filepath.Join(root, "usr/lib/x86_64-linux-gnu/libudev.so.1"))
		keys, _ := filepath.Glob(filepath.Join(root, "etc/hubos/keys/*.pub"))
		need := []string{"usr/local/bin/driftwm", "usr/sbin/udevd", "usr/bin/udevadm", "usr/bin/hubd", "usr/bin/waybar", "usr/bin/wofi", "usr/sbin/seatd", "usr/bin/dbus-daemon",
			"etc/s6/sv/driftwm/run", "etc/s6/sv/waybar/run", "etc/s6/sv/hubd/run", "etc/s6/sv/udevd/run", "etc/s6/sv/seatd/run", "etc/s6/sv/dbus/run", "usr/lib/hubos/follow-driftwm"}
		var missing []string
		for _, f := range need {
			if _, err := os.Stat(filepath.Join(root, f)); err != nil {
				missing = append(missing, f)
			}
		}
		grp, _ := os.ReadFile(filepath.Join(root, "etc/group"))
		good := ok && lu == "libudev.so.1.6.3" && len(keys) == 1 && len(missing) == 0 && strings.Contains(string(grp), "seat:x:1001:hub")
		record("H0 the hub root: no systemd program, unit or banned package; eudev's libudev.so.1 (not the systemd-built one); driftwm, udevd, hubd, Waybar, wofi, seatd, dbus-daemon and the six desktop services are in it; user hub in group seat; the keyring has the key", good, time.Since(start), fmt.Sprintf("libudev.so.1 -> %s; missing: %v; keys: %d", lu, missing, len(keys)))
		if !good {
			t.Errorf("lu=%q missing=%v keys=%d", lu, missing, len(keys))
			t.Fail()
		}
	})

	r.script("build-disk.sh", nil, filepath.Join(bdir, "h1-good", "rootfs.sqsh"), filepath.Join(work, "out", "kernel-a.efi"), r.disk)
	state := func() string { _, o := r.sh(asHub("driftwm msg state")); return o }
	// waitDesktop waits until driftwm answers and Waybar has its bar (a layer surface), up to d.
	waitDesktop := func(d time.Duration) (string, bool) {
		end := time.Now().Add(d)
		var st string
		for time.Now().Before(end) {
			_, st = r.vm.sh(asHub("driftwm msg state 2>&1"), 30*time.Second)
			if strings.Contains(st, "layers 1") && strings.Contains(st, "waybar") {
				return st, true
			}
			time.Sleep(3 * time.Second)
		}
		return st, false
	}
	var firstPNG string

	t.Run("H1_first_boot_desktop_and_bar", func(t *testing.T) {
		r.t = t
		start := time.Now()
		r.vm = r.bootVM("hub first boot")
		handover := time.Since(start)
		// the fake machine ai-1 comes up (as the user hub): the bar then shows "1 of 2 up"
		r.sh(`wget -q -O /tmp/fakenode ` + r.base + `/fakenode && chmod +x /tmp/fakenode && echo fakenode-ok`)
		r.sh(asHub("sh -c '/tmp/fakenode 127.0.0.12:21002 > /tmp/fakenode.log 2>&1 &'"))
		st, up := waitDesktop(240 * time.Second)
		frameAt := time.Since(start)
		time.Sleep(12 * time.Second) // let hubd's first check round finish and the bar redraw
		firstPNG = r.shot("hub-1-first-frame")
		img := loadPNG(t, firstPNG)
		bg, alert := barDrawn(img)
		_, who := r.sh(`ps | grep -E 'udevd|seatd|dbus-daemon|driftwm|waybar|hubd' | grep -v -E 'grep|s6-supervise|follow'`)
		_, feed := r.sh(`/bin/busybox timeout 6 ` + asHub("hubd feed "+hubSock) + ` | head -n 1`)
		rcSt, hubSt := r.sh(asHub("hubd list " + hubSock + " | head -n 8"))
		statusLine := regexp.MustCompile(`\d+ of \d+ up`).FindString(feed)
		userOK := regexp.MustCompile(`(?m)^\s*\d+\s+hub\s+\S*driftwm`).MatchString(who) && regexp.MustCompile(`(?m)^\s*\d+\s+hub\s+\S*waybar`).MatchString(who) && regexp.MustCompile(`(?m)^\s*\d+\s+hub\s+\S*hubd`).MatchString(who)
		ok := up && strings.Contains(st, "camera") && statusLine != "" && bg > 20000 && alert > 500 && userOK && rcSt == 0
		record("H1 first boot of the hub image: the desktop comes up (driftwm msg state answers, Waybar's bar is a layer), the bar is drawn with hubd's status line, and driftwm, Waybar and hubd run as the user hub", ok, time.Since(start),
			fmt.Sprintf("handover line after %.0f s, bar after %.0f s; status line %q; bar pixels %d, alert box pixels %d", handover.Seconds(), frameAt.Seconds(), statusLine, bg, alert))
		t.Logf("state:\n%s\nprocesses:\n%s\nhubd list:\n%s", st, who, hubSt)
		if !ok {
			t.Errorf("up=%v status=%q bg=%d alert=%d userOK=%v", up, statusLine, bg, alert, userOK)
			t.Fail()
		}
	})

	t.Run("H2_driftwm_restarts_and_the_bar_comes_back", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		pidOf := func(n string) string {
			_, o := r.sh(`pidof ` + n)
			return regexp.MustCompile(`(?m)^\d+`).FindString(strings.TrimSpace(o))
		}
		old, owb := pidOf("driftwm"), pidOf("waybar")
		r.sh(`kill -9 $(pidof driftwm)`)
		time.Sleep(3 * time.Second)
		st, back := waitDesktop(180 * time.Second)
		time.Sleep(10 * time.Second)
		nw, nwb := pidOf("driftwm"), pidOf("waybar")
		rcL, _ := r.sh(asHub("hubd list " + hubSock + " > /dev/null"))
		img := loadPNG(t, r.shot("hub-2-after-driftwm-restart"))
		bg, alert := barDrawn(img)
		ok := back && old != "" && nw != "" && old != nw && owb != nwb && rcL == 0 && bg > 20000 && alert > 500
		record("H2 killing driftwm (kill -9) brings the desktop back: s6 restarts it, Waybar and hubd are restarted by their wait-for-driftwm scripts, hubd answers again and the bar is drawn", ok, time.Since(start),
			fmt.Sprintf("driftwm pid %s -> %s, waybar pid %s -> %s, back after %.0f s", old, nw, owb, nwb, time.Since(start).Seconds()))
		if !ok {
			t.Errorf("back=%v old=%q new=%q bg=%d alert=%d\n%s", back, old, nw, bg, alert, st)
			t.Fail()
		}
	})

	t.Run("H3_hubd_open_and_the_menu", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		rc, out := r.sh(asHub("hubd open " + hubSock + " ai-1 2>&1"))
		time.Sleep(8 * time.Second)
		st := state()
		home := regexp.MustCompile(`#\d+ hubos-ai-1 \[0, -100\] \d+x\d+\s+"AI Box"`).MatchString(st)
		r.shot("hub-3-hubd-open")
		// the menu: click the bar item (move far past the top-left corner, then step to the item); the list must show.
		// A click can be lost while the pointer has only just arrived, so it is tried up to three times (the attempts are logged).
		r.sh(`(libinput debug-events > /tmp/ev.log 2>&1 &); sleep 2; echo watching`)
		attempts := 0
		for attempts < 3 {
			attempts++
			r.monitor("mouse_move -4000 -4000")
			time.Sleep(500 * time.Millisecond)
			r.monitor("mouse_move 40 15")
			time.Sleep(time.Second)
			r.monitor("mouse_button 1")
			time.Sleep(time.Second)
			r.monitor("mouse_button 0")
			time.Sleep(6 * time.Second)
			_, w := r.vm.sh(`ps | grep -c '[w]ofi --dmenu'`, 30*time.Second)
			if nums := regexp.MustCompile(`(?m)^\d+\s*$`).FindAllString(w, -1); len(nums) > 0 && strings.TrimSpace(nums[len(nums)-1]) != "0" {
				break
			}
			t.Logf("attempt %d: no menu after the click", attempts)
		}
		_, evs := r.sh(`grep -E 'POINTER_BUTTON|POINTER_MOTION' /tmp/ev.log | awk '{print $1, $2}' | sort | uniq -c; pkill -x libinput; true`)
		t.Logf("input events the guest saw during the click:\n%s", evs)
		img := loadPNG(t, r.shot("hub-3-menu"))
		list := countColor(img, 0, 60, 660, 460, menuBg)
		count := func() string {
			_, w := r.sh(`ps | grep -c '[w]ofi --dmenu'`)
			nums := regexp.MustCompile(`(?m)^\d+\s*$`).FindAllString(w, -1)
			if len(nums) == 0 {
				return "?"
			}
			return strings.TrimSpace(nums[len(nums)-1])
		}
		wofi := count()
		// close the menu again (Escape), a row click is tested in the experiment (docs/proposals/phase-b-desktop.md)
		r.monitor("sendkey esc")
		time.Sleep(3 * time.Second)
		wofi2 := count()
		ok := rc == 0 && home && list > 40000 && wofi != "0" && wofi != "?"
		record("H3 hubd open ai-1 (fake viewer: foot with the app-id hubos-ai-1 and the title AI Box) places the window at its home (0,-100); a click on the Waybar item opens hubd's menu with the machine list drawn", ok, time.Since(start),
			fmt.Sprintf("open rc=%d; click attempts %d; list-background pixels %d (an empty list is under 20000); wofi running %s, after Escape %s", rc, attempts, list, wofi, wofi2))
		if !ok {
			t.Errorf("rc=%d home=%v list=%d\n%s\n%s", rc, home, list, out, st)
			t.Fail()
		}
	})

	t.Run("H3b_menu_row_click", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		// close the AI Box window first, so the click has to open it again (not just go to a window that exists)
		rcEnd, outEnd := r.sh(asHub("hubd end " + hubSock + " ai-1 2>&1"))
		time.Sleep(3 * time.Second)
		stEnd := state()
		closed := rcEnd == 0 && !strings.Contains(stEnd, "hubos-ai-1")
		wofiCount := func() string {
			_, w := r.sh(`ps | grep -c '[w]ofi --dmenu'`)
			nums := regexp.MustCompile(`(?m)^\d+\s*$`).FindAllString(w, -1)
			if len(nums) == 0 {
				return "?"
			}
			return strings.TrimSpace(nums[len(nums)-1])
		}
		homeRe := regexp.MustCompile(`#\d+ hubos-ai-1 \[0, -100\] \d+x\d+\s+"AI Box"`)
		// up to three tries: open the menu with the bar click, move the pointer onto the "ai-1  AI Box" row (the rows are
		// 23 px apart; ai-1 is the 7th line) and click it
		// calibration: clamp the pointer to the bottom right corner, move it by (-150,-150) and find the arrow in a screenshot
		r.monitor("mouse_move 4000 4000")
		time.Sleep(500 * time.Millisecond)
		r.monitor("mouse_move -150 -150")
		time.Sleep(time.Second)
		kx, ky := 1.0, 1.0
		if tx, ty, found := findCursor(loadPNG(t, r.shot("hub-3-pointer-calibration"))); found {
			kx, ky = float64(1023-tx)/150, float64(639-ty)/150
		}
		t.Logf("pointer scale (screen pixels per monitor unit): x %.2f, y %.2f", kx, ky)
		attempts, opened, home := 0, false, false
		var st string
		for attempts < 3 && !home {
			attempts++
			if wofiCount() == "0" {
				r.monitor("mouse_move -4000 -4000")
				time.Sleep(500 * time.Millisecond)
				r.monitor(fmt.Sprintf("mouse_move %d %d", int(math.Round(40/kx)), int(math.Round(15/ky))))
				time.Sleep(time.Second)
				r.monitor("mouse_button 1")
				time.Sleep(time.Second)
				r.monitor("mouse_button 0")
				time.Sleep(6 * time.Second)
			}
			if wofiCount() == "0" {
				t.Logf("attempt %d: the menu did not open", attempts)
				continue
			}
			opened = true
			// The pointer moves by a different amount than the monitor command says (both the USB and the PS/2 mouse of the
			// virtual machine get every move; one jump of 60,198 landed at about 125,405), so the real scale was measured
			// above (kx, ky) and the move is divided by it, in small steps.
			for step := 0; step < 12; step++ {
				r.monitor(fmt.Sprintf("mouse_move %d %d", int(math.Round(5/kx)), int(math.Round(16.5/ky))))
				time.Sleep(120 * time.Millisecond)
			}
			r.monitor("mouse_button 1")
			time.Sleep(time.Second)
			r.monitor("mouse_button 0")
			time.Sleep(8 * time.Second)
			st = state()
			home = homeRe.MatchString(st)
			if !home {
				t.Logf("attempt %d: no AI Box window at its home after the click", attempts)
			}
		}
		r.shot("hub-3-row-click-opened")
		after := wofiCount()
		ok := closed && opened && home
		record("H3b a click on a row of the wofi menu (relative QEMU mouse) makes hubd open that machine: the closed AI Box window comes back at its home (0,-100)", ok, time.Since(start),
			fmt.Sprintf("window closed before: %v (end rc=%d); menu opened: %v; click attempts %d; window at home after the click: %v; wofi running after: %s", closed, rcEnd, opened, attempts, home, after))
		if !ok {
			t.Errorf("closed=%v opened=%v home=%v\n%s\n%s\n%s", closed, opened, home, outEnd, stEnd, st)
			t.Fail()
		}
	})

	hubUpdate := func(name string) (int, string) {
		rc, out := r.vm.sh("hubos-ctl update "+r.base+"/"+name+" 2>&1 | tail -n 14", 900*time.Second)
		t.Logf("$ hubos-ctl update %s\n%s", name, out)
		return rc, out
	}
	t.Run("H4_ab_update_to_a_second_hub_release", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		rc, out := hubUpdate("h2-good")
		m := r.vm.mark()
		io.WriteString(r.vm.in, "sync; reboot -f\n")
		r.afterReset(m)
		st, up := waitDesktop(300 * time.Second)
		confirmed := r.waitConfirmed()
		slot, rel, conf, stt := r.status()
		l := r.vm.text(r.bootPos)
		iEvent := strings.Index(l, "Starting event loop")
		iConf := strings.Index(l, "confirm: boot of slot")
		time.Sleep(10 * time.Second)
		img := loadPNG(t, r.shot("hub-4-release-2"))
		bg, alert := barDrawn(img)
		ok := rc == 0 && up && confirmed && slot == "b" && rel == "2" && conf && iEvent >= 0 && iConf > iEvent && failures(stt) == "0/3" && bg > 20000
		record("H4 an A/B update to a second hub release (signed, trial boot of slot b, confirm): the desktop and bar come up in the trial boot and the confirm step only confirmed after hubd answered (which waits for driftwm)", ok, time.Since(start),
			fmt.Sprintf("update rc=%d; now slot %s release %s, confirmed %v, counter %s; driftwm's event loop started before the confirm line: %v; bar pixels %d, alert %d", rc, slot, rel, conf, failures(stt), iEvent >= 0 && iConf > iEvent, bg, alert))
		if !ok {
			t.Errorf("rc=%d up=%v confirmed=%v slot=%s rel=%s iEvent=%d iConf=%d\n%s\n%s", rc, up, confirmed, slot, rel, iEvent, iConf, out, st)
			t.Fail()
		}
	})

	t.Run("H5_a_bad_second_release_rolls_back", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		rc, _ := hubUpdate("h3-unhealthy")
		m := r.vm.mark()
		io.WriteString(r.vm.in, "sync; reboot -f\n")
		saw := r.vm.wait(`this boot FAILED|BdsDxe: starting Boot[0-9A-F]{4} "hubos-b"`, 400*time.Second, m)
		failAt := time.Since(start).Seconds()
		hh := r.vm.wait(handoverRe, 400*time.Second, max(saw, m))
		if hh >= 0 {
			r.bootPos = hh
			r.ready(r.vm)
		} else {
			r.noteHang(r.vm, "rollback after the bad hub release")
			r.vm.kill()
			r.vm = r.retryBoot("rollback after the bad hub release")
		}
		st, up := waitDesktop(300 * time.Second)
		r.waitConfirmed()
		slot, rel, conf, stt := r.status()
		time.Sleep(10 * time.Second)
		img := loadPNG(t, r.shot("hub-5-after-rollback"))
		bg, _ := barDrawn(img)
		ok := rc == 0 && saw >= 0 && up && slot == "b" && rel == "2" && conf && failures(stt) == "0/3" && bg > 20000
		record("H5 a bad second release (its hubd never gets healthy) is tried, never confirmed, and rolled back: the machine is back on release 2 with the desktop and the bar", ok, time.Since(start),
			fmt.Sprintf("update rc=%d; failure after %.0f s; afterwards slot %s release %s, confirmed %v, counter %s, bar pixels %d", rc, failAt, slot, rel, conf, failures(stt), bg))
		if !ok {
			t.Errorf("rc=%d saw=%d up=%v slot=%s rel=%s conf=%v\n%s", rc, saw, up, slot, rel, conf, st)
			t.Fail()
		}
	})
}
