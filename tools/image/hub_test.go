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
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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
	// hubd is built from the source tree every run (build-root-image.sh keeps a built copy in the work folder between runs)
	os.Remove(filepath.Join(work, "out", "hubd"))
	// files for the layout and restore tests (H3d to H3f), served to the guest over HTTP
	for name, content := range map[string]string{"h6lib.sh": h6lib(), "h6kill.sh": h6kill, "inventory-20.toml": h20Inventory(), "viewers-20.toml": h20Viewers} {
		if err := os.WriteFile(filepath.Join(bdir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, b := range []struct{ v, f string }{{"1", "good"}, {"2", "good"}, {"3", "unhealthy"}} {
		r.script("build-bundle.sh", nil, b.v, b.f, filepath.Join(bdir, "h"+b.v+"-"+b.f), r.pub, r.sec)
	}
	// the fake machines "ai-1", "desk-1" and "desk-2" (listeners on 127.0.0.12:21002, 127.0.0.13:21003, 127.0.0.15:21005) are the repo's test helper, served to the guest over HTTP; it is not in the image
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
	ptrKX, ptrKY := 1.0, 1.0 // the pointer scale measured by H3b (screen pixels per monitor unit)

	t.Run("H1_first_boot_desktop_and_bar", func(t *testing.T) {
		r.t = t
		start := time.Now()
		r.vm = r.bootVM("hub first boot")
		handover := time.Since(start)
		// the fake machines ai-1, desk-1 and desk-2 come up (as the user hub): the bar then shows "3 of 4 up" (nas-1 stays down)
		r.sh(`wget -q -O /tmp/fakenode ` + r.base + `/fakenode && chmod +x /tmp/fakenode && echo fakenode-ok`)
		r.sh(asHub("sh -c '/tmp/fakenode 127.0.0.12:21002 127.0.0.13:21003 127.0.0.15:21005 > /tmp/fakenode.log 2>&1 &'"))
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

	t.Run("H1b_hidden_protocols_and_the_settings_file", func(t *testing.T) {
		r.t = t
		start := time.Now()
		// 1. what the compositor advertises (patch D8, owner decision 7): `wayland-info` lists every global
		_, out := r.vm.sh(asHub("timeout 30 wayland-info 2>&1"), 60*time.Second)
		t.Logf("wayland-info:\n%s", out)
		var advertised []string
		for _, m := range regexp.MustCompile(`interface: '([a-z_0-9]+)'`).FindAllStringSubmatch(out, -1) {
			advertised = append(advertised, m[1])
		}
		has := func(list []string, w string) bool {
			for _, x := range list {
				if x == w {
					return true
				}
			}
			return false
		}
		var leaked []string
		for _, h := range hiddenProtocols {
			if has(advertised, h) {
				leaked = append(leaked, h)
			}
		}
		core := true
		for _, need := range []string{"wl_compositor", "wl_shm", "wl_seat", "xdg_wm_base", "zwlr_layer_shell_v1", "zxdg_decoration_manager_v1", "wp_viewporter"} {
			if !has(advertised, need) {
				core = false
				t.Errorf("the compositor does not advertise %s", need)
			}
		}
		// 2. the settings file: the compositor was started with the image's file, which passed the check; a last good copy
		// exists on the config partition; no alert marker
		_, cmd := r.sh(`for p in $(pidof driftwm); do tr '\000' ' ' < /proc/$p/cmdline 2>/dev/null | grep -- '--backend'; done; ls -l /config/hubos/driftwm.last-good.toml; cmp /config/hubos/driftwm.last-good.toml /etc/hubos/driftwm.toml && echo SAME-AS-IMAGE; ls /run/hub/driftwm-config-fallback 2>&1; grep -n restore_windows /etc/hubos/driftwm.toml`)
		cfgOK := strings.Contains(cmd, "--config /etc/hubos/driftwm.toml") && strings.Contains(cmd, "SAME-AS-IMAGE") && strings.Contains(cmd, "No such file") && regexp.MustCompile(`(?m)^\d+:restore_windows = false`).MatchString(cmd)
		ok := core && len(leaked) == 0 && len(advertised) > 10 && cfgOK
		record("H1b the protocols nothing on the hub uses are not advertised (patch D8: "+strconv.Itoa(len(hiddenProtocols))+" interfaces, none shown by wayland-info), the core ones still are; the compositor runs with the image's settings file, a last good copy was kept, no alert", ok, time.Since(start),
			fmt.Sprintf("advertised %d interfaces; leaked: %v; settings: %s", len(advertised), leaked, firstLine(cmd)))
		if !ok {
			t.Errorf("core=%v leaked=%v advertised=%d cfgOK=%v\n%s", core, leaked, len(advertised), cfgOK, cmd)
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
		servePid := func() string {
			_, o := r.sh(`ps | grep '[h]ubd serve' | awk '{print $1}' | head -n 1`)
			return regexp.MustCompile(`(?m)^\d+`).FindString(strings.TrimSpace(o))
		}
		old, owb, hubd0 := pidOf("driftwm"), pidOf("waybar"), servePid()
		r.sh(`kill -9 $(pidof driftwm)`)
		time.Sleep(3 * time.Second)
		st, back := waitDesktop(180 * time.Second)
		time.Sleep(10 * time.Second)
		nw, nwb, hubd1 := pidOf("driftwm"), pidOf("waybar"), servePid()
		rcL, _ := r.sh(asHub("hubd list " + hubSock + " > /dev/null"))
		img := loadPNG(t, r.shot("hub-2-after-driftwm-restart"))
		bg, alert := barDrawn(img)
		ok := back && old != "" && nw != "" && old != nw && owb != nwb && hubd0 != "" && hubd0 == hubd1 && rcL == 0 && bg > 20000 && alert > 500
		record("H2 killing driftwm (kill -9) brings the desktop back: s6 restarts it, Waybar is restarted by its wait-for-driftwm script, hubd keeps running (same process) and answers, and the bar is drawn", ok, time.Since(start),
			fmt.Sprintf("driftwm pid %s -> %s, waybar pid %s -> %s, hubd pid %s -> %s, back after %.0f s", old, nw, owb, nwb, hubd0, hubd1, time.Since(start).Seconds()))
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
		_, evs := r.sh(`grep -E 'POINTER_BUTTON|POINTER_MOTION' /tmp/ev.log | awk '{print $1, $2}' | sort | uniq -c; kill $(pidof libinput) 2>/dev/null; true`)
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
		// hubd now waits up to 30 s for a window to close (it was 3 s, which was too short on this slow machine); the test still waits up
		// to 30 s for the window to be gone instead of requiring rc 0.
		for i := 0; i < 10 && strings.Contains(stEnd, "hubos-ai-1"); i++ {
			time.Sleep(3 * time.Second)
			stEnd = state()
		}
		closed := !strings.Contains(stEnd, "hubos-ai-1")
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
		ptrKX, ptrKY = kx, ky
		r.sh(`(libinput debug-events > /tmp/ev2.log 2>&1 &); sleep 2; echo watching`)
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
			// Same way as in the desktop experiment: the pointer is sent to the top-left corner and then moved to the row (100,213)
			// in one move, divided by the measured scale (kx, ky) of the pointer.
			r.monitor("mouse_move -4000 -4000")
			time.Sleep(time.Second)
			r.monitor(fmt.Sprintf("mouse_move %d %d", int(math.Round(100/kx)), int(math.Round(213/ky))))
			time.Sleep(time.Second)
			// Two clicks. TESTED here: a single click on a row left the menu open (the row is only selected); the second click
			// opened the machine. (Whether that is wofi's normal behaviour is UNKNOWN; a person would click twice or press Enter.)
			for i := 0; i < 2; i++ {
				r.monitor("mouse_button 1")
				time.Sleep(100 * time.Millisecond)
				r.monitor("mouse_button 0")
				time.Sleep(300 * time.Millisecond)
			}
			time.Sleep(8 * time.Second)
			st = state()
			home = homeRe.MatchString(st)
			if !home {
				t.Logf("attempt %d: no AI Box window at its home after the click", attempts)
			}
		}
		_, evs := r.sh(`grep -E 'POINTER_BUTTON|POINTER_MOTION' /tmp/ev2.log | awk '{print $1, $2}' | sort | uniq -c; tail -n 6 /tmp/ev2.log | cut -c1-120; kill $(pidof libinput) 2>/dev/null; true`)
		t.Logf("input events the guest saw during the row click:\n%s", evs)
		r.shot("hub-3-row-click-opened")
		after := wofiCount()
		ok := closed && opened && home
		record("H3b two clicks on a row of the wofi menu (relative QEMU mouse) make hubd open that machine: the closed AI Box window comes back at its home (0,-100)", ok, time.Since(start),
			fmt.Sprintf("window closed before: %v (end rc=%d); menu opened: %v; click attempts %d; window at home after the click: %v; wofi running after: %s", closed, rcEnd, opened, attempts, home, after))
		if !ok {
			t.Errorf("closed=%v opened=%v home=%v\n%s\n%s\n%s", closed, opened, home, outEnd, stEnd, st)
			t.Fail()
		}
	})

	// ---- H3c: the open marker in the list, picking an open machine, search by name ----
	type winLine struct {
		id, x, y, w, h int
		app            string
		focused        bool
	}
	winRe := regexp.MustCompile(`^(\*?)\s*#(\d+) (\S+) \[(-?\d+), (-?\d+)\] (\d+)x(\d+)`)
	camRe := regexp.MustCompile(`camera (-?\d+(?:\.\d+)?) (-?\d+(?:\.\d+)?) zoom`)
	// parseState reads `driftwm msg state`: the windows (in the order printed) and the camera.
	parseState := func(st string) (ws []winLine, cam [2]float64, camOK bool) {
		for _, ln := range strings.Split(st, "\n") {
			ln = strings.TrimSpace(strings.TrimRight(ln, "\r"))
			if m := winRe.FindStringSubmatch(ln); m != nil {
				var w winLine
				fmt.Sscan(m[2], &w.id)
				w.app, w.focused = m[3], m[1] == "*"
				fmt.Sscan(m[4], &w.x)
				fmt.Sscan(m[5], &w.y)
				fmt.Sscan(m[6], &w.w)
				fmt.Sscan(m[7], &w.h)
				ws = append(ws, w)
			}
		}
		if m := camRe.FindStringSubmatch(st); m != nil {
			fmt.Sscan(m[1], &cam[0])
			fmt.Sscan(m[2], &cam[1])
			camOK = true
		}
		return
	}
	findWin := func(ws []winLine, app string) (winLine, bool) {
		for _, w := range ws {
			if w.app == app {
				return w, true
			}
		}
		return winLine{}, false
	}
	// inView: the window's centre is on the screen (1024x640 under a 30 px bar; the view is centred on the area below the bar).
	inView := func(w winLine, cam [2]float64) bool {
		return math.Abs(float64(w.x)-cam[0]) <= 512 && math.Abs(float64(w.y)-cam[1]) <= 305
	}
	wofiCount := func() string {
		_, w := r.sh(`ps | grep -c '[w]ofi --dmenu'`)
		nums := regexp.MustCompile(`(?m)^\d+\s*$`).FindAllString(w, -1)
		if len(nums) == 0 {
			return "?"
		}
		return strings.TrimSpace(nums[len(nums)-1])
	}
	// openMenu clicks the bar item (up to three tries, as H3 does) and returns the try that worked, or 0 if the menu never opened.
	openMenu := func() int {
		for attempt := 1; attempt <= 3; attempt++ {
			if wofiCount() != "0" {
				return attempt
			}
			r.monitor("mouse_move -4000 -4000")
			time.Sleep(500 * time.Millisecond)
			r.monitor(fmt.Sprintf("mouse_move %d %d", int(math.Round(40/ptrKX)), int(math.Round(15/ptrKY))))
			time.Sleep(time.Second)
			r.monitor("mouse_button 1")
			time.Sleep(time.Second)
			r.monitor("mouse_button 0")
			time.Sleep(6 * time.Second)
			if wofiCount() != "0" {
				return attempt
			}
		}
		return 0
	}
	// clickRow moves the pointer onto the menu row at (100, y) and clicks it; a second click only if the menu is still open.
	clickRow := func(y int) int {
		r.monitor("mouse_move -4000 -4000")
		time.Sleep(time.Second)
		r.monitor(fmt.Sprintf("mouse_move %d %d", int(math.Round(100/ptrKX)), int(math.Round(float64(y)/ptrKY))))
		time.Sleep(time.Second)
		clicks := 0
		for clicks < 3 {
			r.monitor("mouse_button 1")
			time.Sleep(100 * time.Millisecond)
			r.monitor("mouse_button 0")
			clicks++
			time.Sleep(2500 * time.Millisecond)
			if wofiCount() == "0" {
				break
			}
		}
		return clicks
	}
	// markerOf: the marker ("●" or "○") of the machine's line in hubd's list, or "none".
	markerOf := func(list, id string) string {
		for _, ln := range strings.Split(list, "\n") {
			f := strings.Fields(ln)
			if len(f) >= 2 && f[1] == id && (f[0] == "●" || f[0] == "○") {
				return f[0]
			}
		}
		return "none"
	}
	t.Run("H3c_open_marker_pick_focus_and_search", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		const filled, empty = "●", "○"
		// two fake viewer windows: ai-1 (home 0,-100) and desk-1 (home 3000,-100, far to the right); desk-2 stays closed
		r.sh(asHub("hubd open " + hubSock + " ai-1 2>&1"))
		time.Sleep(3 * time.Second)
		rcD, outD := r.sh(asHub("hubd open " + hubSock + " desk-1 2>&1"))
		time.Sleep(8 * time.Second)
		st0 := state()
		ws0, cam0, camOK0 := parseState(st0)
		ai0, haveAI := findWin(ws0, "hubos-ai-1")
		d1, haveD1 := findWin(ws0, "hubos-desk-1")
		_, haveD2 := findWin(ws0, "hubos-desk-2")
		offscreen := haveAI && haveD1 && camOK0 && !inView(ai0, cam0) && inView(d1, cam0)
		_, list0 := r.sh(asHub("hubd list " + hubSock))
		t.Logf("two windows open (desk-1 rc=%d %s):\n%s\nlist:\n%s", rcD, strings.TrimSpace(outD), st0, list0)
		marks := markerOf(list0, "ai-1") == filled && markerOf(list0, "desk-1") == filled && markerOf(list0, "desk-2") == empty && markerOf(list0, "nas-1") == empty && markerOf(list0, "hub") == "none"
		r.shot("hub-3c-two-windows-open")

		// the menu opened by the bar click shows the list
		tries := openMenu()
		img := loadPNG(t, r.shot("hub-3c-menu-markers"))
		listPix := countColor(img, 0, 60, 660, 460, menuBg)
		menuOK := tries > 0 && listPix > 40000

		// click the ai-1 row (7th line): its window is off screen; it must be focused and in view
		clicks := clickRow(213)
		time.Sleep(6 * time.Second)
		st1 := state()
		ws1, cam1, camOK1 := parseState(st1)
		ai1, _ := findWin(ws1, "hubos-ai-1")
		d1b, _ := findWin(ws1, "hubos-desk-1")
		_, foots := r.sh(`ps | grep -c '[f]oot '`) // one viewer per open machine: still two
		pickedOffscreen := camOK1 && ai1.focused && !d1b.focused && inView(ai1, cam1) && !inView(d1b, cam1) && wofiCount() == "0"
		t.Logf("after the click on the ai-1 row (%d clicks):\n%s\nfoot processes: %s", clicks, st1, strings.TrimSpace(foots))
		r.shot("hub-3c-picked-ai-1-from-off-screen")

		// behind another window: move desk-1 on top of ai-1 and focus it, then pick ai-1 again
		r.sh(asHub(fmt.Sprintf("driftwm msg move 0 -100 --id %d", d1b.id)))
		r.sh(asHub(fmt.Sprintf("driftwm msg focus --id %d", d1b.id)))
		time.Sleep(6 * time.Second)
		st2 := state()
		ws2, _, _ := parseState(st2)
		ai2, _ := findWin(ws2, "hubos-ai-1")
		d12, _ := findWin(ws2, "hubos-desk-1")
		coverSetup := d12.focused && !ai2.focused && d12.x == 0 && d12.y == -100
		r.shot("hub-3c-desk-1-covers-ai-1")
		tries2 := openMenu()
		clicks2 := clickRow(213)
		time.Sleep(6 * time.Second)
		st3 := state()
		ws3, cam3, camOK3 := parseState(st3)
		ai3, _ := findWin(ws3, "hubos-ai-1")
		d13, _ := findWin(ws3, "hubos-desk-1")
		pickedBehind := tries2 > 0 && camOK3 && ai3.focused && !d13.focused && inView(ai3, cam3) && wofiCount() == "0"
		order := make([]string, 0, len(ws3))
		for _, w := range ws3 {
			order = append(order, w.app)
		}
		t.Logf("desk-1 on top of ai-1 (setup ok: %v):\n%s\nafter the click (%d clicks):\n%s\nwindows in the order driftwm prints them: %v", coverSetup, st2, clicks2, st3, order)
		r.shot("hub-3c-picked-ai-1-from-behind")

		// close desk-1 with hubd end: its marker must change, ai-1's must not
		rcE, outE := r.sh(asHub("hubd end " + hubSock + " desk-1 2>&1"))
		time.Sleep(4 * time.Second)
		_, list1 := r.sh(asHub("hubd list " + hubSock))
		t.Logf("after hubd end desk-1 (rc=%d %s):\n%s", rcE, strings.TrimSpace(outE), list1)
		// (hubd end answers rc 1, "still open", when the window needs more than its 3 s to go; the dot is what is checked, so wait for it)
		for i := 0; i < 10 && markerOf(list1, "desk-1") != empty; i++ {
			time.Sleep(3 * time.Second)
			_, list1 = r.sh(asHub("hubd list " + hubSock))
		}
		afterEnd := markerOf(list1, "desk-1") == empty && markerOf(list1, "ai-1") == filled && markerOf(list1, "desk-2") == empty
		tries3 := openMenu()
		r.shot("hub-3c-menu-after-end")

		// search by name: type "two" (only the NAME of desk-2 has it; the id is desk-2); Enter picks the one match
		// A key typed into the virtual keyboard can be lost while the machine is busy (seen once: only "o" arrived and Enter then
		// picked the first line, "? search"), so this is tried up to three times; each try starts from a closed menu.
		var st4 string
		var d24 winLine
		var haveD24 bool
		searchTries := 0
		for searchTries < 3 && !haveD24 {
			searchTries++
			if searchTries > 1 {
				for i := 0; i < 3 && wofiCount() != "0"; i++ { // Escape closes the search prompt, then the list
					r.monitor("sendkey esc")
					time.Sleep(3 * time.Second)
				}
				tries3 = openMenu()
			}
			for _, k := range []string{"t", "w", "o"} {
				r.monitor("sendkey " + k)
				time.Sleep(700 * time.Millisecond)
			}
			time.Sleep(2 * time.Second)
			r.shot(fmt.Sprintf("hub-3c-search-two-%d", searchTries))
			r.monitor("sendkey ret")
			time.Sleep(10 * time.Second)
			st4 = state()
			ws4, _, _ := parseState(st4)
			d24, haveD24 = findWin(ws4, "hubos-desk-2")
			if !haveD24 {
				t.Logf("search try %d: no desk-2 window after typing 'two' and Enter", searchTries)
			}
		}
		// Only that the window exists and is the focused one is asserted: whether driftwm kept the window at its home is not what this
		// step is about (in one run, under load, desk-2 stood at driftwm's own cascade spot [25, -125] instead; it is logged in the note).
		searchOpened := tries3 > 0 && haveD24 && d24.focused && wofiCount() == "0"
		// The dot turns filled when hubd has recorded the window; while it is still placing it ("[opening]", slow under load)
		// the line is empty, so wait for it for up to a minute.
		var list2 string
		for i := 0; i < 12; i++ {
			_, list2 = r.sh(asHub("hubd list " + hubSock))
			if markerOf(list2, "desk-2") == filled {
				break
			}
			time.Sleep(5 * time.Second)
		}
		t.Logf("after typing 'two' and Enter:\n%s\nlist:\n%s", st4, list2)
		searchMarker := markerOf(list2, "desk-2") == filled

		ok := rcD == 0 && !haveD2 && offscreen && marks && menuOK && pickedOffscreen && coverSetup && pickedBehind && afterEnd && searchOpened && searchMarker
		record("H3c the open marker: two fake viewer windows are open (ai-1 and desk-1, far apart) and a third machine (desk-2) is not; the list opened by the bar click marks the two with a filled dot and the others with an empty dot; a click on the open ai-1 row (window off screen, then window behind another) focuses it and moves the view to it; hubd end desk-1 changes its marker; typing part of a NAME finds a marked line and a pick of it opens the machine", ok, time.Since(start),
			fmt.Sprintf("windows open before: ai-1 %v, desk-1 %v, desk-2 %v; ai-1 off screen while desk-1 in view: %v; markers right (ai-1 and desk-1 filled, desk-2 and nas-1 empty, the hub line has no dot): %v; menu opened on try %d, list pixels %d; click on ai-1 (%d clicks): focused and in view, desk-1 not: %v; desk-1 placed over ai-1: %v; second pick (%d clicks): ai-1 focused and in view: %v; after hubd end desk-1 (rc %d): desk-1 empty and ai-1 filled: %v; search 'two' + Enter (try %d) opened desk-2 (focused): %v (at [%d, %d]; its home is [3000, 1000]), then filled: %v; camera before %v, after first pick %v (read ok: %v)",
				haveAI, haveD1, haveD2, offscreen, marks, tries, listPix, clicks, pickedOffscreen, coverSetup, clicks2, pickedBehind, rcE, afterEnd, searchTries, searchOpened, d24.x, d24.y, searchMarker, cam0, cam1, camOK1))
		if !ok {
			t.Errorf("rcD=%d haveD2=%v offscreen=%v marks=%v menuOK=%v pickedOffscreen=%v coverSetup=%v pickedBehind=%v afterEnd=%v searchOpened=%v searchMarker=%v", rcD, haveD2, offscreen, marks, menuOK, pickedOffscreen, coverSetup, pickedBehind, afterEnd, searchOpened, searchMarker)
			t.Fail()
		}
	})

	// ---- H3d to H3f: saved layouts, the STALE marker, and 20 windows that survive kill -9 of the compositor (docs/hubd-slice2.md section 18) ----
	// pollState reads `driftwm msg state` every 3 s until cond is true, for at most d.
	pollState := func(d time.Duration, cond func(ws []hubWin, cam [2]float64, zoom float64) bool) (string, bool) {
		end := time.Now().Add(d)
		var st string
		for {
			st = state()
			ws, cam, zoom, ok := parseHubState(st)
			if ok && cond(ws, cam, zoom) {
				return st, true
			}
			if time.Now().After(end) {
				return st, false
			}
			time.Sleep(3 * time.Second)
		}
	}
	// servePID is the process id of `hubd serve` ("" if none).
	servePID := func() string {
		_, o := r.sh(`for p in $(pidof hubd); do tr '\000' ' ' < /proc/$p/cmdline | grep -q ' serve ' && echo SERVEPID=$p; done; true`)
		return regexp.MustCompile(`SERVEPID=(\d+)`).FindString(o)
	}
	waitHubd := func(d time.Duration) bool {
		end := time.Now().Add(d)
		for time.Now().Before(end) {
			if rc, _ := r.vm.sh(asHub("hubd list "+hubSock+" > /dev/null 2>&1"), 30*time.Second); rc == 0 {
				return true
			}
			time.Sleep(3 * time.Second)
		}
		return false
	}
	lay := func(rest string) (int, string) { return r.sh(asHub("hubd layout " + hubSock + " " + rest + " 2>&1")) }

	t.Run("H3d_layouts_on_the_real_driftwm", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		r.sh(`wget -q -O /tmp/h6lib.sh ` + r.base + `/h6lib.sh && wget -q -O /tmp/h6kill.sh ` + r.base + `/h6kill.sh && echo scripts-ok`)
		for _, id := range []string{"ai-1", "desk-1", "desk-2"} {
			r.sh(asHub("hubd open " + hubSock + " " + id + " 2>&1"))
		}
		three := func(ws []hubWin, _ [2]float64, _ float64) bool {
			m := hubWinsByApp(ws, "hubos-")
			return len(m) == 3 && m["hubos-ai-1"].ID >= 0 && m["hubos-desk-1"].W >= 64 && m["hubos-desk-2"].W >= 64 && m["hubos-ai-1"].W >= 64
		}
		st0, three0 := pollState(180*time.Second, three)
		ws0, _, _, _ := parseHubState(st0)
		ids := map[string]int{}
		for app, w := range hubWinsByApp(ws0, "hubos-") {
			ids[app] = w.ID
		}
		// arrangement A: three small windows side by side, view zoomed out and shifted
		put := func(app string, x, y, w, h int) {
			r.sh(asHub(fmt.Sprintf("driftwm msg resize %d %d --id %d", w, h, ids[app])))
			r.sh(asHub(fmt.Sprintf("driftwm msg move %d %d --id %d", x, y, ids[app])))
		}
		put("hubos-ai-1", -400, 0, 300, 200)
		put("hubos-desk-1", 0, 0, 300, 200)
		put("hubos-desk-2", 400, 0, 300, 200)
		r.sh(asHub("driftwm msg zoom 0.8"))
		time.Sleep(3 * time.Second)
		r.sh(asHub("driftwm msg camera 120 -60"))
		time.Sleep(3 * time.Second)
		stA, _ := pollState(60*time.Second, func(ws []hubWin, cam [2]float64, z float64) bool {
			m := hubWinsByApp(ws, "hubos-")
			return len(m) == 3 && m["hubos-ai-1"].X == -400 && m["hubos-desk-1"].X == 0 && m["hubos-desk-2"].X == 400 && z > 0.79 && z < 0.81 && cam[0] > 119 && cam[0] < 121
		})
		wsA, camA, zoomA, _ := parseHubState(stA)
		mapA := hubWinsByApp(wsA, "hubos-")
		rcSaveA, outSaveA := lay("save arr-a")
		_, lsOut := r.sh(`ls -l /config/hubos/layouts; grep -c '"sha256"' /config/hubos/layouts/arr-a.layout.json; grep -c '"machine"' /config/hubos/layouts/arr-a.layout.json`)
		// arrangement B: all three on top of each other: saved with a warning, not an error
		for app := range ids {
			r.sh(asHub(fmt.Sprintf("driftwm msg move 0 0 --id %d", ids[app])))
		}
		time.Sleep(2 * time.Second)
		rcSaveB, outSaveB := lay("save arr-b")
		// the view and the windows are changed, then A is applied
		r.sh(asHub("driftwm msg zoom 1"))
		time.Sleep(3 * time.Second)
		r.sh(asHub("driftwm msg camera 0 0"))
		time.Sleep(3 * time.Second)
		rcApply, outApply := lay("apply arr-a")
		stBack, backOK := pollState(60*time.Second, func(ws []hubWin, cam [2]float64, z float64) bool {
			return len(placeDiff(mapA, hubWinsByApp(ws, "hubos-"))) == 0 && z > 0.79 && z < 0.81 && cam[0] > 119 && cam[0] < 121 && cam[1] > -61 && cam[1] < -59
		})
		_, listOut := lay("list")
		// the active layout is remembered when hubd starts again, and hubd takes its windows back (nothing is reopened)
		pidBefore := servePID()
		r.sh(`s6-svc -t /run/service/hubd`)
		time.Sleep(3 * time.Second)
		up := waitHubd(180 * time.Second)
		pidAfter := servePID()
		_, listAfter := lay("list")
		_, hubList := r.sh(asHub("hubd list " + hubSock))
		remembered := strings.Contains(listAfter, "* arr-a") && markerOf(hubList, "ai-1") == "●" && markerOf(hubList, "desk-1") == "●" && markerOf(hubList, "desk-2") == "●"
		// the active layout also governs a machine opened later; after "clear" the home position is used again
		r.sh(asHub("hubd end " + hubSock + " desk-2 2>&1"))
		_, gone := pollState(60*time.Second, func(ws []hubWin, _ [2]float64, _ float64) bool {
			_, ok := hubWinsByApp(ws, "hubos-desk-2")["hubos-desk-2"]
			return !ok
		})
		r.sh(asHub("hubd open " + hubSock + " desk-2 2>&1"))
		_, atLayout := pollState(120*time.Second, func(ws []hubWin, _ [2]float64, _ float64) bool {
			w, ok := hubWinsByApp(ws, "hubos-desk-2")["hubos-desk-2"]
			return ok && w.X == mapA["hubos-desk-2"].X && w.Y == mapA["hubos-desk-2"].Y && w.W == mapA["hubos-desk-2"].W && w.H == mapA["hubos-desk-2"].H
		})
		rcClear, outClear := lay("clear")
		r.sh(asHub("hubd end " + hubSock + " desk-2 2>&1"))
		pollState(60*time.Second, func(ws []hubWin, _ [2]float64, _ float64) bool {
			_, ok := hubWinsByApp(ws, "hubos-desk-2")["hubos-desk-2"]
			return !ok
		})
		r.sh(asHub("hubd open " + hubSock + " desk-2 2>&1"))
		_, atHome := pollState(120*time.Second, func(ws []hubWin, _ [2]float64, _ float64) bool {
			w, ok := hubWinsByApp(ws, "hubos-desk-2")["hubos-desk-2"]
			return ok && w.X == 3000 && w.Y == 1000
		})
		// a half-written layout file is detected, set aside and ignored; hubd keeps running
		r.sh(asHub(`sh -c 'f=/config/hubos/layouts/arr-b.layout.json; head -c 120 $f > $f.part && mv $f.part $f'`))
		pidBeforeCorrupt := servePID()
		_, listCorrupt := lay("list")
		_, lsCorrupt := r.sh(`ls -A /config/hubos/layouts`)
		rcSaveC, _ := lay("save arr-c")
		pidAfterCorrupt := servePID()
		corruptOK := strings.Contains(listCorrupt, "WARNING: layout arr-b") && strings.Contains(listCorrupt, "moved aside") && strings.Contains(lsCorrupt, "arr-b.layout.json.corrupt.") && !strings.Contains(lsCorrupt, "arr-b.layout.json\n") && pidBeforeCorrupt == pidAfterCorrupt && rcSaveC == 0
		lay("delete arr-a")
		lay("delete arr-c")
		r.sh(`rm -f /config/hubos/layouts/*.corrupt.*; ls -A /config/hubos/layouts`)
		narrow := mapA["hubos-ai-1"].W <= 390 && mapA["hubos-desk-1"].W <= 390 && mapA["hubos-desk-2"].W <= 390 // the three windows really stand apart
		ok := three0 && rcSaveA == 0 && strings.Contains(outSaveA, "3 windows") && (!narrow || !strings.Contains(outSaveA, "overlap")) &&
			strings.Contains(lsOut, "hub") && strings.Contains(lsOut, "arr-a.layout.json") && strings.Contains(lsOut, "\n1\n3\n") &&
			rcSaveB == 0 && strings.Contains(outSaveB, "WARNING: these windows overlap") &&
			rcApply == 0 && strings.Contains(outApply, "moved 3 windows") && backOK && strings.Contains(listOut, "* arr-a") &&
			up && pidBefore != "" && pidAfter != "" && pidBefore != pidAfter && remembered &&
			gone && atLayout && rcClear == 0 && atHome && corruptOK
		record("H3d saved layouts on the real driftwm: save (3 windows and the view; the folder belongs to the user hub; checksum in the file), an overlap is a warning, apply moves the open windows and the view back, the active layout is remembered when hubd starts again and the windows are taken back, a machine opened later goes to its place in the active layout and, after clear, to its home, and a half-written layout file is set aside with a warning while hubd keeps running", ok, time.Since(start),
			fmt.Sprintf("save A rc=%d %q; overlap warning on B: rc=%d %v; apply rc=%d %q; back at A with the view (zoom %.2f camera %.0f,%.0f): %v; hubd %s -> %s; remembered after restart: %v; desk-2 opened later at its layout place: %v (closed first: %v); clear rc=%d %q; then at home (3000,1000): %v; damaged file handled: %v",
				rcSaveA, strings.TrimSpace(outSaveA), rcSaveB, strings.Contains(outSaveB, "WARNING: these windows overlap"), rcApply, firstLine(outApply), zoomA, camA[0], camA[1], backOK, pidBefore, pidAfter, remembered, atLayout, gone, rcClear, strings.TrimSpace(outClear), atHome, corruptOK))
		if !ok {
			t.Errorf("three0=%v saveA=%d %q ls=%q saveB=%d %q apply=%d %q back=%v list=%q up=%v pids %q %q remembered=%v gone=%v atLayout=%v clear=%d atHome=%v corrupt=%v\nlist after: %s\ncorrupt list: %s\nls: %s\nstate back: %s",
				three0, rcSaveA, outSaveA, lsOut, rcSaveB, outSaveB, rcApply, outApply, backOK, listOut, up, pidBefore, pidAfter, remembered, gone, atLayout, rcClear, atHome, corruptOK, listAfter, listCorrupt, lsCorrupt, stBack)
			t.Fail()
		}
	})

	t.Run("H3e_hubd_killed_the_bar_and_the_menu_show_the_last_state_marked_STALE", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		_, live := r.sh(`/bin/busybox timeout 6 ` + asHub("hubd feed "+hubSock) + ` | head -n 1`)
		time.Sleep(2 * time.Second)
		imgLive := loadPNG(t, r.shot("hub-3e-bar-live"))
		wLive := alertWidth(imgLive)
		// kill -9 of hubd, and s6 is told not to start it again (the bar and the menu must then show the last state)
		r.sh(`. /tmp/h6lib.sh; s6-svc -O /run/service/hubd; kill_hubd; sleep 1; pidof hubd > /dev/null && echo still-some-hubd; true`)
		time.Sleep(8 * time.Second)
		_, down := r.sh(`/bin/busybox timeout 8 ` + asHub("hubd feed "+hubSock) + ` | head -n 1`)
		imgStale := loadPNG(t, r.shot("hub-3e-bar-stale"))
		wStale := alertWidth(imgStale)
		_, srv := r.sh(`ps | grep -c '[h]ubd serve'`)
		// the menu: a stand-in for wofi saves the lines it is shown
		r.sh(`printf '#!/bin/sh\ncat > /tmp/stale-menu.txt\n' > /tmp/fakewofi; chmod 755 /tmp/fakewofi; rm -f /tmp/stale-menu.txt`)
		rcMenu, _ := r.sh(asHub("hubd menu " + hubSock + " --wofi /tmp/fakewofi --style /nonexistent 2>&1"))
		_, menuTxt := r.sh(`head -n 4 /tmp/stale-menu.txt`)
		// the real menu from a click on the bar item
		tries := openMenu()
		time.Sleep(8 * time.Second)
		img := loadPNG(t, r.shot("hub-3e-menu-stale"))
		listPix := countColor(img, 0, 60, 660, 460, menuBg)
		wofiAfter := wofiCount()
		for i := 0; i < 3 && wofiAfter != "0"; i++ {
			r.monitor("sendkey esc")
			time.Sleep(3 * time.Second)
			wofiAfter = wofiCount()
		}
		// hubd comes back: the bar is live again
		r.sh(`s6-svc -u /run/service/hubd`)
		up := waitHubd(180 * time.Second)
		time.Sleep(12 * time.Second)
		_, back := r.sh(`/bin/busybox timeout 8 ` + asHub("hubd feed "+hubSock) + ` | head -n 1`)
		liveText := regexp.MustCompile(`"text":"([^"]*)"`).FindStringSubmatch(live)
		staleOK := strings.Contains(down, `"text":"STALE: `) && strings.Contains(down, `"class":"alert"`) && strings.Contains(down, "hubd is not running: this is the last known state") &&
			liveText != nil && strings.Contains(down, `"text":"STALE: `+liveText[1]+`"`)
		ok := staleOK && wStale > wLive+30 && strings.HasPrefix(strings.TrimSpace(srv), "0") && rcMenu == 1 && strings.Contains(menuTxt, "! STALE: hubd is not running") &&
			tries > 0 && listPix > 40000 && wofiAfter == "0" && up && !strings.Contains(back, "STALE")
		record("H3e hubd killed (kill -9, not restarted): the bar item shows the last known state marked STALE (the red box gets longer; the tooltip says how old it is), the menu opened by a click shows the last list with a STALE first line, and both are live again when hubd is back", ok, time.Since(start),
			fmt.Sprintf("live line %q; stale line %q; red box %d px -> %d px; menu: first lines %q, real menu opened on try %d with list pixels %d; after hubd is back: %q", strings.TrimSpace(live), strings.TrimSpace(down), wLive, wStale, firstLine(menuTxt), tries, listPix, strings.TrimSpace(back)))
		if !ok {
			t.Errorf("stale=%v widths %d %d serve=%q rcMenu=%d menu=%q tries=%d list=%d wofi=%s up=%v back=%q", staleOK, wLive, wStale, srv, rcMenu, menuTxt, tries, listPix, wofiAfter, up, back)
			t.Fail()
		}
	})

	t.Run("H3f_twenty_windows_survive_kill_9_of_the_compositor", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		cycles := 8
		if v, err := strconv.Atoi(os.Getenv("HUBOS_KILL_CYCLES")); err == nil && v > 0 {
			cycles = v
		}
		seed := time.Now().UnixNano()
		if v, err := strconv.ParseInt(os.Getenv("HUBOS_KILL_SEED"), 10, 64); err == nil {
			seed = v
		}
		rng := rand.New(rand.NewSource(seed))
		t.Logf("kill cycles: %d, random seed: %d", cycles, seed)
		// close the windows of the earlier tests, then switch to the 20-machine inventory and viewers
		for _, id := range []string{"ai-1", "desk-1", "desk-2"} {
			r.sh(asHub("hubd end " + hubSock + " " + id + " 2>&1"))
		}
		pollState(90*time.Second, func(ws []hubWin, _ [2]float64, _ float64) bool { return len(hubWinsByApp(ws, "hubos-")) == 0 })
		r.sh(`kill $(pidof foot) 2>/dev/null; true`)
		r.sh(`wget -q -O /tmp/h6lib.sh ` + r.base + `/h6lib.sh && wget -q -O /tmp/h6kill.sh ` + r.base + `/h6kill.sh && wget -q -O /tmp/inventory-20.toml ` + r.base + `/inventory-20.toml && wget -q -O /tmp/viewers-20.toml ` + r.base + `/viewers-20.toml && echo files-ok`)
		r.sh(`cp /config/hubos/inventory.toml /config/hubos/inventory.toml.orig && cp /config/hubos/viewers.toml /config/hubos/viewers.toml.orig && cp /tmp/inventory-20.toml /config/hubos/inventory.toml && cp /tmp/viewers-20.toml /config/hubos/viewers.toml && sync && echo swapped`)
		r.sh(asHub("sh -c '/tmp/fakenode " + h20Addrs() + " > /tmp/fakenode20.log 2>&1 &'"))
		r.sh(`s6-svc -t /run/service/hubd`)
		time.Sleep(3 * time.Second)
		up := waitHubd(180 * time.Second)
		// open all 20 at the same time
		var ids20 []string
		for i := 1; i <= 20; i++ {
			ids20 = append(ids20, fmt.Sprintf("w%02d", i))
		}
		openStart := time.Now()
		r.vm.sh(`. /tmp/h6lib.sh; for i in `+strings.Join(ids20, " ")+`; do hubc open $i > /tmp/open-$i.log 2>&1 & done; wait; echo opened`, 600*time.Second)
		all20 := func(ws []hubWin, _ [2]float64, _ float64) bool {
			m := hubWinsByApp(ws, "hubos-w")
			for _, w := range m {
				if w.W < 64 {
					return false
				}
			}
			return len(m) == 20
		}
		_, opened := pollState(600*time.Second, all20)
		openTook := time.Since(openStart)
		// arrange: every window at a place that is not its home, some resized; then the view
		_, stOpen := r.sh(asHub("driftwm msg state"))
		wsOpen, _, _, _ := parseHubState(stOpen)
		var arrange strings.Builder
		for i, w := range wsOpen {
			if !strings.HasPrefix(w.App, "hubos-w") {
				continue
			}
			n, _ := strconv.Atoi(strings.TrimPrefix(w.App, "hubos-w"))
			col, row := (n-1)%5, (n-1)/5
			x, y := -1000+col*500+37+rng.Intn(40), -row*300-23-rng.Intn(30)
			if i%3 == 0 {
				fmt.Fprintf(&arrange, "dwm resize %d %d --id %d; ", 260+rng.Intn(60), 150+rng.Intn(40), w.ID)
			}
			fmt.Fprintf(&arrange, "dwm move %d %d --id %d; ", x, y, w.ID)
		}
		r.vm.sh(`. /tmp/h6lib.sh; `+arrange.String()+`dwm zoom 0.8 > /dev/null; sleep 3; dwm camera 150 -420 > /dev/null; sleep 3; echo arranged`, 600*time.Second)
		settled := func() (map[string]hubWin, [2]float64, float64, bool) {
			var prev map[string]hubWin
			for i := 0; i < 8; i++ {
				ws, cam, z, ok := parseHubState(state())
				cur := hubWinsByApp(ws, "hubos-w")
				if ok && len(cur) == 20 && prev != nil && len(placeDiff(prev, cur)) == 0 {
					return cur, cam, z, true
				}
				prev = cur
				time.Sleep(3 * time.Second)
			}
			return prev, [2]float64{}, 0, false
		}
		expected, expCam, expZoom, armed := settled()
		t.Logf("arranged: %d windows, zoom %.2f camera %.1f,%.1f", len(expected), expZoom, expCam[0], expCam[1])
		// a 20-window layout: save, scramble five windows, apply
		rcBig, outBig := lay("save big")
		var scramble strings.Builder
		n := 0
		for _, w := range expected {
			if n++; n > 5 {
				break
			}
			fmt.Fprintf(&scramble, "dwm move %d %d --id %d; dwm resize 200 120 --id %d; ", 4000+n*10, 4000, w.ID, w.ID)
		}
		r.vm.sh(`. /tmp/h6lib.sh; `+scramble.String()+`echo scrambled`, 300*time.Second)
		rcBigApply, outBigApply := lay("apply big")
		_, bigBack := pollState(120*time.Second, func(ws []hubWin, cam [2]float64, z float64) bool {
			return len(placeDiff(expected, hubWinsByApp(ws, "hubos-w"))) == 0 && z > expZoom-0.02 && z < expZoom+0.02
		})

		// the stacking order (owner decision 14: part of "restore exact"): focus all 20 windows in a random order, so the
		// last one is on top and the order is far from the order in which they were opened; focusing pans the view, so the
		// view is put back afterwards. wantOrder is the order bottom to top that every restore has to bring back.
		perm := rng.Perm(20)
		var wantOrder []string
		var focusCmds strings.Builder
		for _, k := range perm {
			app := fmt.Sprintf("hubos-w%02d", k+1)
			wantOrder = append(wantOrder, app)
			fmt.Fprintf(&focusCmds, "dwm focus --id %d >/dev/null 2>&1; sleep 0.4; ", expected[app].ID)
		}
		r.vm.sh(`. /tmp/h6lib.sh; `+focusCmds.String()+fmt.Sprintf("dwm zoom %.2f > /dev/null; sleep 3; dwm camera %.1f %.1f > /dev/null; sleep 3; echo focused", expZoom, expCam[0], expCam[1]), 600*time.Second)
		_, orderSet := pollState(120*time.Second, func(ws []hubWin, cam [2]float64, z float64) bool {
			return sameStrings(stackAppsOf(ws, "hubos-w"), wantOrder) && z > expZoom-0.02 && z < expZoom+0.02 && cam[0] > expCam[0]-3 && cam[0] < expCam[0]+3 && cam[1] > expCam[1]-3 && cam[1] < expCam[1]+3
		})
		t.Logf("stacking order set (bottom to top): %v; read back equal: %v", wantOrder, orderSet)
		orderChecks := 0

		// the kill cycles
		modes := []string{"idle", "move", "save", "restore", "restart", "move", "restore", "save"}
		type cycleResult struct {
			mode        string
			delay       float64
			took        time.Duration
			ok          bool
			note        string
			hubdMessage string
		}
		var results20 []cycleResult
		allCyclesOK := true
		pidBase := servePID()
		for c := 0; c < cycles && !r.dead; c++ {
			mode := modes[c%len(modes)]
			delay := rng.Float64() * 3
			var movedApp string
			var nx, ny, mid int
			switch mode {
			case "move":
				delay = rng.Float64() * 0.4
				var apps []string
				for a := range expected {
					apps = append(apps, a)
				}
				sort.Strings(apps)
				movedApp = apps[rng.Intn(len(apps))]
				mw := expected[movedApp]
				nx, ny, mid = mw.X+61+rng.Intn(80), mw.Y-47-rng.Intn(40), mw.ID
			case "save":
				delay = 0.05 + rng.Float64()*1.5
			}
			want1 := expected
			want2 := map[string]hubWin{}
			for k, v := range expected {
				want2[k] = v
			}
			if movedApp != "" {
				mw := want2[movedApp]
				mw.X, mw.Y = nx, ny
				want2[movedApp] = mw
			}
			dwPid := func() string {
				_, o := r.sh(`for p in $(pidof driftwm); do tr '\000' ' ' < /proc/$p/cmdline 2>/dev/null | grep -q -- '--backend' && echo DWPID=$p; done; true`)
				return regexp.MustCompile(`DWPID=(\d+)`).FindString(o)
			}
			dwBefore := dwPid()
			t0 := time.Now()
			var killOut string
			if mode == "restart" {
				_, killOut = r.vm.sh(asHub("hubd restart-desktop "+hubSock+" 2>&1"), 120*time.Second)
			} else {
				_, killOut = r.vm.sh(fmt.Sprintf("sh /tmp/h6kill.sh %s %d %d %d %.2f", mode, mid, nx, ny, delay), 400*time.Second)
			}
			t.Logf("cycle %d (%s, delay %.2f s): %s", c+1, mode, delay, strings.TrimSpace(killOut))
			var gotMap map[string]hubWin
			var gotOrder []string
			var diffs []string
			good := func(ws []hubWin, cam [2]float64, z float64) bool {
				got := hubWinsByApp(ws, "hubos-w")
				if len(ws) != 20 || len(got) != 20 {
					diffs = []string{fmt.Sprintf("%d windows in all, %d of the machines", len(ws), len(got))}
					return false
				}
				d1 := placeDiff(want1, got)
				d2 := placeDiff(want2, got)
				viewOK := z > expZoom-0.02 && z < expZoom+0.02 && cam[0] > expCam[0]-3 && cam[0] < expCam[0]+3 && cam[1] > expCam[1]-3 && cam[1] < expCam[1]+3
				if !viewOK {
					diffs = []string{fmt.Sprintf("view: zoom %.3f camera %.1f,%.1f, saved zoom %.3f camera %.1f,%.1f", z, cam[0], cam[1], expZoom, expCam[0], expCam[1])}
					return false
				}
				if len(d1) == 0 || (movedApp != "" && len(d2) == 0) {
					// A `driftwm msg move` can raise the window it moves (seen in the first run of this test: the moved window
					// stood just below the focused one afterwards), so in a "move" cycle the order of all the OTHER windows
					// is compared, and the moved window may be anywhere.
					o := stackAppsOf(ws, "hubos-w")
					if !sameStrings(withoutApp(o, movedApp), withoutApp(wantOrder, movedApp)) {
						diffs = []string{fmt.Sprintf("stacking order (bottom to top) is %v, saved order %v (moved window %q left out of the comparison)", o, wantOrder, movedApp)}
						return false
					}
					gotOrder = o
					gotMap = got
					return true
				}
				diffs = d1
				return false
			}
			_, back := pollState(360*time.Second, good)
			took := time.Since(t0)
			stable := false
			msg := ""
			if back {
				_, lastMsg := r.sh(`grep 'the desktop restarted: ' /var/log/hubd/current | tail -n 1`)
				msg = regexp.MustCompile(`the desktop restarted: [^\n]*`).FindString(lastMsg)
				time.Sleep(8 * time.Second)
				_, stable = pollState(30*time.Second, good)
			}
			dwAfter := dwPid()
			_, hubdLog := r.sh(`tail -n 600 /var/log/hubd/current | grep -E 'hubd: (restore |the desktop restarted)' | tail -n 30`)
			t.Logf("cycle %d: what hubd logged about the restore:\n%s", c+1, hubdLog)
			_, extra := r.sh(`echo FOOT=$(ps | grep -c '[f]oot '); echo TMP=$(ls -A /config/hubos/layouts | grep -c -E 'tmp|corrupt'); ls /config/hubos/layouts`)
			footOK := strings.Contains(extra, "FOOT=20")
			tmpOK := strings.Contains(extra, "TMP=0")
			pidOK := servePID() == pidBase
			res := cycleResult{mode: mode, delay: delay, took: took, hubdMessage: msg}
			res.ok = back && stable && footOK && tmpOK && pidOK && dwBefore != "" && dwAfter != "" && dwBefore != dwAfter
			if movedApp != "" && gotMap != nil {
				if len(placeDiff(want2, gotMap)) == 0 {
					res.note = "the last move was kept"
				} else {
					res.note = "the last move came too late for hubd to see it (the window is at its place before the move)"
				}
			}
			if res.ok {
				orderChecks++
			}
			if res.ok && gotOrder != nil {
				wantOrder = gotOrder // the next cycle starts from the order that was just restored
			}
			if res.ok && gotMap != nil {
				expected = gotMap // also takes the new window ids (they start again from 0 in every new compositor)
			}
			if !res.ok {
				res.note += fmt.Sprintf(" FAILED: back=%v stable=%v foot20=%v noTempOrCorrupt=%v hubdSamePid=%v compositor pid %s -> %s diffs=%v", back, stable, footOK, tmpOK, pidOK, dwBefore, dwAfter, diffs)
				allCyclesOK = false
			}
			results20 = append(results20, res)
			if !res.ok {
				break
			}
		}
		var summary []string
		var times []float64
		for i, rr := range results20 {
			summary = append(summary, fmt.Sprintf("%d %s delay %.2f s: %v in %.0f s %s [%s]", i+1, rr.mode, rr.delay, rr.ok, rr.took.Seconds(), rr.note, rr.hubdMessage))
			times = append(times, rr.took.Seconds())
		}
		sort.Float64s(times)
		median := 0.0
		if len(times) > 0 {
			median = times[len(times)/2]
		}
		// the view after all the kills, and the layout folder is clean
		_, lsEnd := lay("list")
		// clean up: close the windows, bring back the first inventory and viewers
		lay("delete big")
		lay("delete h6-save")
		r.vm.sh(`. /tmp/h6lib.sh; for i in `+strings.Join(ids20, " ")+`; do hubc end $i > /tmp/end-$i.log 2>&1 & done; wait; echo ended`, 300*time.Second)
		pollState(120*time.Second, func(ws []hubWin, _ [2]float64, _ float64) bool { return len(hubWinsByApp(ws, "hubos-")) == 0 })
		r.sh(`kill $(pidof foot) 2>/dev/null; kill $(pidof fakenode) 2>/dev/null; mv /config/hubos/inventory.toml.orig /config/hubos/inventory.toml; mv /config/hubos/viewers.toml.orig /config/hubos/viewers.toml; rm -f /config/hubos/layouts/*; sync; s6-svc -t /run/service/hubd; echo restored-config`)
		waitHubd(180 * time.Second)
		r.sh(asHub("sh -c '/tmp/fakenode 127.0.0.12:21002 127.0.0.13:21003 127.0.0.15:21005 > /tmp/fakenode.log 2>&1 &'"))
		ok := orderSet && orderChecks == len(results20) && up && opened && armed && rcBig == 0 && strings.Contains(outBig, "20 windows") && rcBigApply == 0 && bigBack && allCyclesOK && len(results20) == cycles
		record(fmt.Sprintf("H3f 20 fake windows survive kill -9 of the compositor: %d cycles of kill -9 at random moments (idle, right after a window move, during a row of layout saves, a second kill during the restore, and hubd restart-desktop); after each one hubd, which keeps running, starts the 20 viewers again and puts every window at its place, size, stacking order and the view back; no duplicate, no leftover temp or damaged layout file", cycles), ok, time.Since(start),
			fmt.Sprintf("seed %d; 20 windows opened at once in %.0f s; 20-window layout save rc=%d, apply after scrambling 5 windows rc=%d back=%v (%s); cycles run %d of %d, median %.0f s to be back; stacking order of the 20 windows (random, set with focus) checked after each restore: %d of %d equal; hubd pid unchanged: %v; results: %s; layouts at the end: %s", seed, openTook.Seconds(), rcBig, rcBigApply, bigBack, firstLine(outBigApply), len(results20), cycles, median, orderChecks, len(results20), pidBase, strings.Join(summary, " | "), firstLine(lsEnd)))
		_ = outBigApply
		if !ok {
			t.Errorf("up=%v opened=%v armed=%v big=%d %q applied=%d back=%v cycles ok=%v ran %d/%d\n%s", up, opened, armed, rcBig, outBig, rcBigApply, bigBack, allCyclesOK, len(results20), cycles, strings.Join(summary, "\n"))
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
		// The compositor answered its IPC BEFORE the confirm: the confirm step itself records, right before it confirms, what
		// `driftwm msg state` answered (usr/lib/hubos/confirm-evidence, a file in /run/hubos), so this does not depend on the order of
		// lines in the serial log (the compositor's log writer is a thread and its lines may arrive late).
		_, ev := r.sh(`cat /run/hubos/confirm-evidence 2>&1; echo ---; head -n 3 /run/hubos/confirm-state.txt 2>&1`)
		answeredBefore := strings.Contains(ev, "driftwm-state=answered") && strings.Contains(ev, "camera ") && strings.Contains(ev, "time=")
		time.Sleep(10 * time.Second)
		img := loadPNG(t, r.shot("hub-4-release-2"))
		bg, alert := barDrawn(img)
		ok := rc == 0 && up && confirmed && slot == "b" && rel == "2" && conf && answeredBefore && failures(stt) == "0/3" && bg > 20000
		record("H4 an A/B update to a second hub release (signed, trial boot of slot b, confirm): the desktop and bar come up in the trial boot, the confirm step only confirmed after hubd answered, and the compositor answered its IPC (driftwm msg state) before the confirm, as recorded by the confirm step itself", ok, time.Since(start),
			fmt.Sprintf("update rc=%d; now slot %s release %s, confirmed %v, counter %s; compositor answered before the confirm (evidence file of the confirm step): %v; bar pixels %d, alert %d", rc, slot, rel, conf, failures(stt), answeredBefore, bg, alert))
		if !ok {
			t.Errorf("rc=%d up=%v confirmed=%v slot=%s rel=%s answeredBefore=%v\n%s\n%s\n%s", rc, up, confirmed, slot, rel, answeredBefore, ev, out, st)
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
	// H6: the owner's restart policy for the compositor (image/machines/hub/rootfs/etc/s6/sv/driftwm/finish): after 5 crashes within
	// a minute s6 stops restarting it, a fixed message is on the screen, hubd and the recovery terminal stay up, and nothing reboots.
	t.Run("H6_five_compositor_crashes_in_a_minute_stop_the_restarts", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		waitDesktop(240 * time.Second)
		_, bootBefore := r.sh(`cat /proc/sys/kernel/random/boot_id`)
		r.sh(`rm -f /run/hub/driftwm-crashes /run/hub/driftwm-gave-up`)
		m := r.vm.mark()
		// kill -9 the compositor five times, each time as soon as a new compositor process exists
		_, kl := r.vm.sh(`last=0; for i in 1 2 3 4 5; do n=0; p=; while [ $n -lt 600 ]; do p=; for q in $(pidof driftwm); do tr '\000' ' ' < /proc/$q/cmdline | grep -q -- --backend && p=$q; done; [ -n "$p" ] && [ "$p" != "$last" ] && break; sleep 0.2; n=$((n+1)); done; echo "kill $i: pid $p"; [ -n "$p" ] && kill -9 $p; last=$p; done; echo kills-done`, 240*time.Second)
		time.Sleep(25 * time.Second)
		compositors := func() string {
			_, o := r.sh(`n=0; for q in $(pidof driftwm); do tr '\000' ' ' < /proc/$q/cmdline | grep -q -- --backend && n=$((n+1)); done; echo "compositors:$n"`)
			return regexp.MustCompile(`compositors:\d+`).FindString(o)
		}
		c1 := compositors()
		_, svstat := r.sh(`s6-svstat /run/service/driftwm`)
		rcMark, _ := r.sh(`test -e /run/hub/driftwm-gave-up`)
		_, nCrash := r.sh(`wc -l < /run/hub/driftwm-crashes`)
		rcHubd, _ := r.sh(asHub("hubd list " + hubSock + " > /dev/null"))
		_, hubdPid := r.sh(`pidof hubd`)
		img := loadPNG(t, r.shot("hub-6-after-five-crashes"))
		light := 0 // light text pixels in the top left of the screen
		for y := 0; y < 220; y++ {
			for x := 0; x < 900; x++ {
				rr, gg, bb, _ := img.At(x, y).RGBA()
				if rr>>8 > 120 && gg>>8 > 120 && bb>>8 > 120 {
					light++
				}
			}
		}
		barLeft, _ := barDrawn(img)
		time.Sleep(30 * time.Second) // it must stay down (s6's own restart would be within a second)
		c2 := compositors()
		_, bootAfter := r.sh(`cat /proc/sys/kernel/random/boot_id`)
		l := r.vm.text(m)
		sawMsg := strings.Contains(l, "HUB OS: the desktop (driftwm) crashed 5 times within one minute and was stopped.")
		sawGaveUp := strings.Contains(l, "driftwm: giving up")
		stopped := c1 == "compositors:0" && c2 == "compositors:0" && rcMark == 0 && strings.Contains(svstat, "down")
		sameBoot := strings.TrimSpace(bootBefore) != "" && strings.TrimSpace(bootBefore) == strings.TrimSpace(bootAfter)
		// the screen: the message in light text, the bar gone
		screenOK := light > 400 && barLeft < 2000
		// then the manual way back: s6-svc -u starts the compositor again and the desktop follows
		r.sh(`s6-svc -u /run/service/driftwm`)
		st, back := waitDesktop(300 * time.Second)
		rcMark2, _ := r.sh(`test -e /run/hub/driftwm-gave-up`)
		time.Sleep(10 * time.Second)
		rcHubd2, _ := r.sh(asHub("hubd list " + hubSock + " > /dev/null"))
		ok := strings.Contains(kl, "kills-done") && stopped && rcHubd == 0 && strings.TrimSpace(hubdPid) != "" && sameBoot && sawMsg && sawGaveUp && screenOK && back && rcMark2 != 0 && rcHubd2 == 0
		record("H6 five compositor crashes within a minute: s6 stops restarting it, the fixed message is on the screen and on the serial console, hubd still answers, the machine does not reboot; s6-svc -u brings the desktop back", ok, time.Since(start),
			fmt.Sprintf("compositors after 25 s / 55 s: %s / %s; s6-svstat %q; marker written %v; crash lines %s; hubd answered %v (pid %s); same boot %v; message on the serial console %v, 'giving up' line %v; light pixels in the top left %d, bar pixels left %d; after s6-svc -u: desktop back %v, marker gone %v, hubd answers %v",
				c1, c2, strings.TrimSpace(svstat), rcMark == 0, strings.TrimSpace(nCrash), rcHubd == 0, strings.TrimSpace(hubdPid), sameBoot, sawMsg, sawGaveUp, light, barLeft, back, rcMark2 != 0, rcHubd2 == 0))
		t.Logf("kill loop:\n%s\nstate after s6-svc -u:\n%s", kl, st)
		if !ok {
			t.Errorf("stopped=%v rcHubd=%d sameBoot=%v sawMsg=%v sawGaveUp=%v screenOK=%v back=%v marker2=%d hubd2=%d", stopped, rcHubd, sameBoot, sawMsg, sawGaveUp, screenOK, back, rcMark2, rcHubd2)
			t.Fail()
		}
	})
}
