//go:build qemu && hubisolation

package image

// THROWAWAY EXPERIMENT (docs/proposals/hub-isolation.md): what the hub's desktop does when one of its supervised programs
// is killed (kill -9) or frozen (SIGSTOP). It reuses the rig of tools/image/qemu_test.go and hub_test.go and changes none
// of them. To run it, copy this file next to them (it is not part of the normal test suite) and run from tools/image:
//
//	cp experiments/hub-isolation/hubisolation_test.go ./zz_hubisolation_test.go
//	HUBOS_HUB_WORK=/some/dir/outside/the/repo go test -tags 'qemu hubisolation' -count=1 -run TestHubIsolation -timeout 300m -v .
//	rm ./zz_hubisolation_test.go
//
// Output: a block per case and one "ISO SUMMARY" line per case; grep for "ISO".

import (
	"fmt"
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

// guest helper functions, defined once in the guest's console shell (one line each).
const isoFuncs = `fp() { for d in /proc/[0-9]*; do c=$(tr '\000' ' ' < $d/cmdline 2>/dev/null); case "$c" in "$1"*) echo ${d#/proc/}; return;; esac; done; }
pst() { [ -n "$1" ] && sed -n 's/^State:[[:space:]]*\(.\).*/\1/p' /proc/$1/status 2>/dev/null; }
ISOE="` + hubEnv + `"
smp() { echo "up=$(cut -d' ' -f1 /proc/uptime) seatd=$(fp 'seatd -g') udevd=$(fp 'udevd') dbus=$(fp 'dbus-daemon --session') driftwm=$(fp '/usr/local/bin/driftwm --backend') waybar=$(fp 'waybar -c') hubd=$(fp '/usr/bin/hubd serve')"; o=$(/bin/busybox timeout -s KILL 8 s6-setuidgid hub env $ISOE driftwm msg state 2>&1); echo "drift_rc=$? bar=$(echo "$o" | grep -c waybar)"; /bin/busybox timeout -s KILL 8 s6-setuidgid hub env $ISOE hubd list --socket /run/hubos/hubd.sock >/dev/null 2>&1; echo "hubd_rc=$?"; }
svs() { for s in seatd udevd dbus driftwm waybar hubd; do echo "$s: $(s6-svstat /run/service/$s 2>&1 | cut -c1-40)"; done; }
`

type isoSample struct {
	at      time.Duration
	pids    map[string]string
	driftRC string
	bar     string
	hubdRC  string
	raw     string
}

var isoNames = []string{"seatd", "udevd", "dbus", "driftwm", "waybar", "hubd"}

func parseIso(raw string, at time.Duration) isoSample {
	s := isoSample{at: at, pids: map[string]string{}, raw: raw}
	for _, n := range isoNames {
		m := regexp.MustCompile(`\b` + n + `=(\d*)`).FindStringSubmatch(raw)
		if m != nil {
			s.pids[n] = m[1]
		}
	}
	if m := regexp.MustCompile(`drift_rc=(\d+) bar=(\d+)`).FindStringSubmatch(raw); m != nil {
		s.driftRC, s.bar = m[1], m[2]
	}
	if m := regexp.MustCompile(`hubd_rc=(\d+)`).FindStringSubmatch(raw); m != nil {
		s.hubdRC = m[1]
	}
	return s
}

// healthy: all six programs exist, driftwm answers and has the bar's layer, hubd answers.
func (s isoSample) healthy() bool {
	for _, n := range isoNames {
		if s.pids[n] == "" {
			return false
		}
	}
	return s.driftRC == "0" && s.bar != "" && s.bar != "0" && s.hubdRC == "0"
}

// isoSetup builds everything (cached in HUBOS_HUB_WORK) and returns the rig, the work folder and a clean-up function.
// The throwaway update key is made once and reused by later runs, so the recovery kernel is not rebuilt.
func isoSetup(t *testing.T) (*rig, func()) {
	var cleanups []func()
	cleanup := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}
	work := os.Getenv("HUBOS_HUB_WORK")
	if work == "" {
		var err error
		if work, err = os.MkdirTemp("", "hubos-iso-"); err != nil {
			t.Fatal(err)
		}
		cleanups = append(cleanups, func() { os.RemoveAll(work) })
	}
	os.MkdirAll(work, 0o755)
	r := &rig{t: t, work: work, disk: filepath.Join(work, "vm"), hangDir: filepath.Join(work, "hangs"), machine: "image/machines/hub.build",
		gui: true, monPath: filepath.Join(work, "mon.sock"), shots: filepath.Join(work, "shots")}
	os.MkdirAll(r.shots, 0o755)
	cleanups = append(cleanups, func() {
		if r.vm != nil {
			r.vm.kill()
		}
	})
	// ---- build (same steps as TestHubImage, one good release only) ----
	r.script("fetch-tools.sh", nil)
	kdir := filepath.Join(work, "keys")
	os.MkdirAll(kdir, 0o700)
	os.Setenv("LD_LIBRARY_PATH", filepath.Join(work, "tools/root/usr/lib/x86_64-linux-gnu")+":"+filepath.Join(work, "tools/root/lib/x86_64-linux-gnu"))
	r.pub, r.sec = filepath.Join(kdir, "update.pub"), filepath.Join(kdir, "update.sec")
	if _, err := os.Stat(r.sec); err != nil {
		if out, err := exec.Command(r.tool("bin/signify-openbsd"), "-G", "-n", "-p", r.pub, "-s", r.sec, "-c", "throwaway hub key").CombinedOutput(); err != nil {
			t.Fatalf("signify -G: %v\n%s", err, out)
		}
	}
	r.script("build-kernel.sh", []string{"UPDATE_PUB=" + r.pub})
	r.script("build-base.sh", nil)
	r.script("build-hub-parts.sh", nil)
	bdir := filepath.Join(work, "bundles")
	os.RemoveAll(bdir)
	os.MkdirAll(bdir, 0o755)
	r.script("build-bundle.sh", nil, "1", "good", filepath.Join(bdir, "h1-good"), r.pub, r.sec)
	if out, err := exec.Command("go", "build", "-o", filepath.Join(bdir, "fakenode"), "../fakenode").CombinedOutput(); err != nil {
		t.Fatalf("go build fakenode: %v\n%s", err, out)
	}
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	go http.Serve(ln, http.FileServer(http.Dir(bdir)))
	r.base = fmt.Sprintf("http://10.0.2.2:%d", ln.Addr().(*net.TCPAddr).Port)
	cleanups = append(cleanups, func() { ln.Close() })
	r.script("build-disk.sh", nil, filepath.Join(bdir, "h1-good", "rootfs.sqsh"), filepath.Join(work, "out", "kernel-a.efi"), r.disk)
	return r, cleanup
}

func TestHubIsolation(t *testing.T) {
	r, cleanup := isoSetup(t)
	defer cleanup()

	// ---- boot ----
	start := time.Now()
	r.vm = r.bootVM("hub isolation first boot")
	t.Logf("ISO boot: handover line after %.0f s", time.Since(start).Seconds())
	prepare := func() {
		r.sh(`wget -q -O /tmp/fakenode ` + r.base + `/fakenode && chmod +x /tmp/fakenode && echo fakenode-ok`)
		r.sh(asHub("sh -c '/tmp/fakenode 127.0.0.12:21002 127.0.0.13:21003 127.0.0.15:21005 > /tmp/fakenode.log 2>&1 &'"))
		for _, l := range strings.Split(strings.TrimSpace(isoFuncs), "\n") {
			r.vm.sh(l, 30*time.Second)
		}
	}
	prepare()
	sample := func(t0 time.Time) isoSample {
		_, raw := r.vm.sh("smp", 90*time.Second)
		return parseIso(raw, time.Since(t0))
	}
	fmtS := func(s isoSample) string {
		return fmt.Sprintf("+%5.1fs seatd=%s udevd=%s dbus=%s driftwm=%s waybar=%s hubd=%s | driftwm_state rc=%s bar_layer=%s | hubd_list rc=%s healthy=%v",
			s.at.Seconds(), s.pids["seatd"], s.pids["udevd"], s.pids["dbus"], s.pids["driftwm"], s.pids["waybar"], s.pids["hubd"], s.driftRC, s.bar, s.hubdRC, s.healthy())
	}
	// waitHealthy samples until healthy or the limit; returns the last sample and whether it is healthy.
	waitHealthy := func(limit time.Duration) (isoSample, bool) {
		t0 := time.Now()
		var s isoSample
		for time.Since(t0) < limit {
			s = sample(t0)
			if s.healthy() {
				return s, true
			}
			time.Sleep(3 * time.Second)
		}
		return s, false
	}
	s0, ok := waitHealthy(420 * time.Second)
	t.Logf("ISO baseline: %s", fmtS(s0))
	if !ok {
		t.Fatalf("the hub desktop never became healthy; nothing to test")
	}

	type tcase struct {
		name, prog string
		sig        string // KILL or STOP
	}
	var cases []tcase
	for _, sig := range []string{"KILL", "STOP"} {
		for _, p := range []string{"seatd", "udevd", "dbus", "waybar", "hubd", "driftwm"} {
			cases = append(cases, tcase{sig + " " + p, p, sig})
		}
	}
	for _, c := range cases {
		c := c
		t.Run(strings.ReplaceAll(c.name, " ", "_"), func(t *testing.T) {
			r.t = t
			before, ok := waitHealthy(300 * time.Second)
			if !ok {
				t.Logf("ISO %s: NOT healthy before the test (%s); rebooting the machine to get a clean start", c.name, fmtS(before))
				m := r.vm.mark()
				fmt.Fprintf(r.vm.in, "sync; reboot -f\n")
				r.afterReset(m)
				prepare()
				if before, ok = waitHealthy(420 * time.Second); !ok {
					t.Fatalf("no clean start after a reboot")
				}
			}
			victim := before.pids[c.prog]
			mark := r.vm.mark()
			t0 := time.Now()
			r.vm.sh(fmt.Sprintf("kill -%s %s; echo injected", c.sig, victim), 30*time.Second)
			inj := time.Since(t0)
			limit := 240 * time.Second
			if c.sig == "STOP" {
				limit = 75 * time.Second
			}
			var firstBad, recovered time.Duration = -1, -1
			var last isoSample
			var lines []string
			for time.Since(t0) < limit {
				s := sample(t0)
				last = s
				lines = append(lines, fmtS(s))
				if !s.healthy() && firstBad < 0 {
					firstBad = s.at
				}
				if s.healthy() && recovered < 0 && (firstBad >= 0 || (c.sig == "KILL" && s.pids[c.prog] != victim)) {
					recovered = s.at
					break
				}
				time.Sleep(2 * time.Second)
			}
			var changed []string
			for _, n := range isoNames {
				if last.pids[n] != before.pids[n] {
					changed = append(changed, n)
				}
			}
			_, sv := r.vm.sh("svs", 60*time.Second)
			_, st := r.vm.sh(fmt.Sprintf("echo state=$(pst %s)", victim), 20*time.Second)
			unfrozenAt := time.Duration(-1)
			if c.sig == "STOP" {
				t1 := time.Now()
				r.vm.sh(fmt.Sprintf("kill -CONT %s 2>/dev/null; echo cont", victim), 30*time.Second)
				if s, ok := waitHealthy(240 * time.Second); ok {
					unfrozenAt = time.Since(t1)
					last = s
				}
				for _, n := range isoNames {
					if last.pids[n] != before.pids[n] && !contains(changed, n) {
						changed = append(changed, n)
					}
				}
			}
			var logLines []string
			re := regexp.MustCompile(`(?i)follow-driftwm|driftwm:|udevd:|s6-|supervise|error|panic|lost|failed|exited|killed|died|HUBOS`)
			for _, l := range strings.Split(r.vm.text(mark), "\n") {
				if re.MatchString(l) && !strings.Contains(l, "__D") && len(logLines) < 30 {
					logLines = append(logLines, "    "+strings.TrimSpace(l))
				}
			}
			shot := r.shot("iso-" + strings.ReplaceAll(c.name, " ", "_"))
			bg, alert := barDrawn(loadPNG(t, shot))
			t.Logf("ISO %s: victim pid %s (injection took %.1fs)\n  samples:\n    %s\n  s6-svstat after:\n    %s\n  victim state letter after: %s\n  console lines:\n%s",
				c.name, victim, inj.Seconds(), strings.Join(lines, "\n    "), strings.ReplaceAll(sv, "\n", "\n    "), st, strings.Join(logLines, "\n"))
			t.Logf("ISO SUMMARY %-14s first_bad=%s healthy_again=%s after_SIGCONT=%s programs_with_new_pid=%v bar_pixels=%d alert_pixels=%d",
				c.name, durS(firstBad), durS(recovered), durS(unfrozenAt), changed, bg, alert)
		})
	}
}

// TestHubIsolationDeadlines measures how long the hub's own small clients wait for a frozen hubd or a frozen driftwm
// (the "deadline on every call" rule), and records the memory of each program and what the hub kernel offers for limits.
func TestHubIsolationDeadlines(t *testing.T) {
	r, cleanup := isoSetup(t)
	defer cleanup()
	r.vm = r.bootVM("hub isolation deadlines boot")
	r.sh(`wget -q -O /tmp/fakenode ` + r.base + `/fakenode && chmod +x /tmp/fakenode && echo fakenode-ok`)
	r.sh(asHub("sh -c '/tmp/fakenode 127.0.0.12:21002 127.0.0.13:21003 127.0.0.15:21005 > /tmp/fakenode.log 2>&1 &'"))
	for _, l := range strings.Split(strings.TrimSpace(isoFuncs), "\n") {
		r.vm.sh(l, 30*time.Second)
	}
	var s isoSample
	ok := false
	for i := 0; i < 100 && !ok; i++ {
		_, raw := r.vm.sh("smp", 90*time.Second)
		s = parseIso(raw, 0)
		ok = s.healthy()
		if !ok {
			time.Sleep(3 * time.Second)
		}
	}
	if !ok {
		t.Fatalf("hub desktop never healthy")
	}
	t.Logf("ISO2 baseline pids: %v", s.pids)
	// what the hub kernel offers for limits, and the memory of each program
	_, o := r.vm.sh(`uname -r; echo "--- /proc/cgroups:"; cat /proc/cgroups 2>&1 | head -5; echo "--- /sys/fs/cgroup:"; ls /sys/fs/cgroup 2>&1 | head -5; echo "--- /proc/pressure:"; ls /proc/pressure 2>&1 | head -3; echo "--- tainted: $(cat /proc/sys/kernel/tainted)"; echo "--- ulimit -n (this shell): $(ulimit -n)"; grep -E 'MemTotal|MemAvailable' /proc/meminfo`, 30*time.Second)
	t.Logf("ISO2 kernel limits:\n%s", o)
	for _, n := range isoNames {
		p := s.pids[n]
		_, o := r.vm.sh(fmt.Sprintf(`echo %s pid=%s $(grep -E 'VmRSS|Threads|VmSwap' /proc/%s/status 2>&1 | tr -s ' \n\t' ' ') oom_score_adj=$(cat /proc/%s/oom_score_adj)`, n, p, p, p), 30*time.Second)
		t.Logf("ISO2 memory %s", o)
	}
	_, o = r.vm.sh(`ps -o pid,user,args 2>&1 | grep -E 'seatd|udevd|dbus-daemon|driftwm|waybar|hubd|s6-' | grep -v grep | cut -c1-140`, 30*time.Second)
	t.Logf("ISO2 who runs as which user:\n%s", o)

	timed := func(name, cmd string, limit int) {
		_, o := r.vm.sh(fmt.Sprintf(`a=$(date +%%s); /bin/busybox timeout -s KILL %d %s > /tmp/iso.out 2>&1; rc=$?; b=$(date +%%s); echo "rc=$rc seconds=$((b-a)) output_lines=$(wc -l < /tmp/iso.out) first=$(head -c 120 /tmp/iso.out | head -n 1)"`, limit, cmd), time.Duration(limit+60)*time.Second)
		t.Logf("ISO2 DEADLINE %-52s %s", name, o)
	}
	hubdList := asHub("hubd list " + hubSock)
	hubdFeed := asHub("hubd feed " + hubSock)
	drift := asHub("driftwm msg state")
	timed("hubd list, hubd healthy", hubdList, 30)
	timed("hubd feed (killed by the test after 12 s), healthy", hubdFeed, 12)
	// freeze hubd
	r.vm.sh(fmt.Sprintf("kill -STOP %s; echo stopped", s.pids["hubd"]), 30*time.Second)
	timed("hubd list while hubd is frozen (SIGSTOP)", hubdList, 100)
	timed("hubd feed while hubd is frozen (limit 20 s)", hubdFeed, 20)
	r.vm.sh(fmt.Sprintf("kill -CONT %s; echo cont", s.pids["hubd"]), 30*time.Second)
	time.Sleep(5 * time.Second)
	// freeze driftwm
	r.vm.sh(fmt.Sprintf("kill -STOP %s; echo stopped", s.pids["driftwm"]), 30*time.Second)
	timed("driftwm msg state while driftwm is frozen (limit 40 s)", drift, 40)
	timed("hubd list while driftwm is frozen (limit 100 s)", hubdList, 100)
	timed("hubd feed while driftwm is frozen (limit 15 s)", hubdFeed, 15)
	r.vm.sh(fmt.Sprintf("kill -CONT %s; echo cont", s.pids["driftwm"]), 30*time.Second)
}

func contains(a []string, s string) bool {
	for _, x := range a {
		if x == s {
			return true
		}
	}
	return false
}

func durS(d time.Duration) string {
	if d < 0 {
		return "never"
	}
	return fmt.Sprintf("%.0fs", d.Seconds())
}
