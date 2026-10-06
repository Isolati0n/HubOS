//go:build qemu

package image

// Helpers for the hub image tests of saved layouts, direct restore after a compositor kill, and the STALE marker
// (docs/hubd-slice2.md section 18). The subtests themselves are in hub_test.go (TestHubImage) because they use the
// virtual machine started there.

import (
	"fmt"
	"image"
	"image/color"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// hubWin is one window line of `driftwm msg state`.
type hubWin struct {
	ID, X, Y, W, H int
	App            string
	Focused        bool
}

var hubWinRe = regexp.MustCompile(`^(\*?)\s*#(\d+) (\S+) \[(-?\d+), (-?\d+)\] (\d+)x(\d+)`)
var hubCamRe = regexp.MustCompile(`(?m)^camera (-?\d+(?:\.\d+)?) (-?\d+(?:\.\d+)?)\s*$`)
var hubZoomRe = regexp.MustCompile(`(?m)^zoom (\d+(?:\.\d+)?)\s*$`)

// parseHubState reads `driftwm msg state`: the windows, the camera and the zoom.
func parseHubState(st string) (ws []hubWin, cam [2]float64, zoom float64, ok bool) {
	for _, ln := range strings.Split(st, "\n") {
		ln = strings.TrimSpace(strings.TrimRight(ln, "\r"))
		if m := hubWinRe.FindStringSubmatch(ln); m != nil {
			w := hubWin{App: m[3], Focused: m[1] == "*"}
			w.ID, _ = strconv.Atoi(m[2])
			w.X, _ = strconv.Atoi(m[4])
			w.Y, _ = strconv.Atoi(m[5])
			w.W, _ = strconv.Atoi(m[6])
			w.H, _ = strconv.Atoi(m[7])
			ws = append(ws, w)
		}
	}
	if m := hubCamRe.FindStringSubmatch(st); m != nil {
		cam[0], _ = strconv.ParseFloat(m[1], 64)
		cam[1], _ = strconv.ParseFloat(m[2], 64)
		ok = true
	}
	if m := hubZoomRe.FindStringSubmatch(st); m != nil {
		zoom, _ = strconv.ParseFloat(m[1], 64)
	} else {
		ok = false
	}
	return
}

// hubWinsByApp keeps the windows whose app-id starts with the prefix, by app-id.
func hubWinsByApp(ws []hubWin, prefix string) map[string]hubWin {
	m := map[string]hubWin{}
	for _, w := range ws {
		if strings.HasPrefix(w.App, prefix) {
			m[w.App] = w
		}
	}
	return m
}

// placeDiff lists, in words, every window of want that is missing from got or stands elsewhere (same centre and size).
func placeDiff(want, got map[string]hubWin) []string {
	var out []string
	var keys []string
	for k := range want {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		w := want[k]
		g, ok := got[k]
		switch {
		case !ok:
			out = append(out, k+" is missing")
		case g.X != w.X || g.Y != w.Y || g.W != w.W || g.H != w.H:
			out = append(out, fmt.Sprintf("%s is at [%d, %d] %dx%d, saved place [%d, %d] %dx%d", k, g.X, g.Y, g.W, g.H, w.X, w.Y, w.W, w.H))
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			out = append(out, k+" is an extra window")
		}
	}
	return out
}

// alertWidth is how far to the right the red alert box of the bar reaches in the top row (no text is drawn in row 3, so the
// width is the width of the box).
func alertWidth(img image.Image) int {
	w := 0
	near := func(a, b uint8) bool { d := int(a) - int(b); return d > -6 && d < 6 }
	for x := 0; x < 700; x++ {
		r, g, b, _ := img.At(x, 3).RGBA()
		if near(uint8(r>>8), barAlertRGBA().R) && near(uint8(g>>8), barAlertRGBA().G) && near(uint8(b>>8), barAlertRGBA().B) {
			w = x
		}
	}
	return w
}

func barAlertRGBA() color.RGBA { return color.RGBA{0x8a, 0x1c, 0x1c, 255} }

// h20 is the 20-machine test inventory for the restore test: hub plus w01..w20 on 127.0.1.1..20, ports 22001..22020,
// homes on a grid of 5 columns and 4 rows, 500 by 300 apart (the fake viewer's window is far smaller than that).
func h20Inventory() string {
	var b strings.Builder
	b.WriteString("# Test inventory of the hub image test H3f: 20 fake machines. Nothing real.\nformat = 1\n\n")
	b.WriteString("[[machine]]\nid = \"hub\"\nname = \"Desk Hub\"\nrole = \"hub\"\naddress = \"127.0.0.10\"\nopen = [\"none\"]\nhome = { x = 4000, y = 4000 }\n\n")
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&b, "[[machine]]\nid = \"w%02d\"\nname = \"Window %02d\"\nrole = \"desktop\"\naddress = \"127.0.1.%d\"\nopen = [\"ssh\"]\nhome = { x = %d, y = %d }\nport = %d\nuser = \"owner\"\n\n",
			i+1, i+1, i+1, -1000+(i%5)*500, -(i/5)*300, 22001+i)
	}
	return b.String()
}

func h20Addrs() string {
	var a []string
	for i := 0; i < 20; i++ {
		a = append(a, fmt.Sprintf("127.0.1.%d:%d", i+1, 22001+i))
	}
	return strings.Join(a, " ")
}

// h20Viewers is the fake viewer with a small window (20 terminals must fit in 2 GB and a software-drawn screen).
const h20Viewers = `# Test viewers table of the hub image test H3f: the fake viewer (a terminal that sits there), small.
format = 1

[[viewer]]
id        = "fake"
programs  = ["moonlight", "spice", "vnc", "ssh", "files"]
command   = ["foot", "--app-id={app_id}", "--title={title}", "--window-size-chars=24x4", "--", "sleep", "infinity"]
sets_name = true
window_wait = "10s"
late_grace  = "120s"
`

// h6lib is sourced by the guest commands of the restore test (root shell on the guest).
func h6lib() string {
	return `HE="` + hubEnv + `"
dwm() { s6-setuidgid hub env $HE driftwm msg "$@"; }
hubc() { c=$1; shift; s6-setuidgid hub env $HE hubd $c --socket /run/hubos/hubd.sock "$@"; }
kill_dw() { for p in $(pidof driftwm); do tr '\000' ' ' < /proc/$p/cmdline 2>/dev/null | grep -q -- '--backend' && kill -9 $p; done; }
kill_hubd() { for p in $(pidof hubd); do tr '\000' ' ' < /proc/$p/cmdline 2>/dev/null | grep -q ' serve ' && kill -9 $p; done; }
`
}

// h6kill is the guest script that kills the compositor at a chosen moment: MODE ID X Y DELAY.
//
//	idle     wait DELAY seconds, then kill -9
//	move     move window ID to X Y, wait DELAY seconds, then kill -9 (the move is the last thing the owner did)
//	save     start a row of 40 `hubd layout save` commands, kill -9 after DELAY seconds (the kill lands during a layout save)
//	restore  kill -9, and kill -9 again DELAY seconds after the first restored window shows (a second crash during the restore)
const h6kill = `. /tmp/h6lib.sh
mode=$1; id=$2; x=$3; y=$4; d=$5
case $mode in
 idle) sleep $d; kill_dw;;
 move) dwm move $x $y --id $id >/dev/null 2>&1; sleep $d; kill_dw;;
 save) ( i=0; while [ $i -lt 40 ]; do hubc layout save h6-save --replace >/dev/null 2>&1; i=$((i+1)); done ) & sleep $d; kill_dw; wait;;
 restore) kill_dw; n=0; while [ $n -lt 480 ]; do c=$(dwm state 2>/dev/null | grep -c ' hubos-w'); if [ "$c" -ge 1 ] && [ "$c" -lt 20 ]; then sleep $d; kill_dw; echo second-kill-with-$c-windows; break; fi; n=$((n+1)); sleep 0.25; done;;
esac
echo killed-$mode
`

// firstLine is the first line of s, trimmed.
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "\n"); i >= 0 {
		return s[:i]
	}
	return s
}
