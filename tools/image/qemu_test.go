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
	"encoding/json"
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
	"syscall"
	"testing"
	"time"
)

const handoverRe = `HUBOS: handing over to s6-svscan`

type vm struct {
	t         *testing.T
	cmd       *exec.Cmd
	in        io.WriteCloser
	mu        sync.Mutex
	buf       strings.Builder
	log       *os.File
	done      chan struct{} // closed when QEMU's output ends
	exit      chan struct{} // closed when QEMU has been reaped (ps is then set)
	ps        *os.ProcessState
	byUs      bool // the test itself killed QEMU (not a crash)
	crashSeen bool
	rig       *rig
}

type rig struct {
	t          *testing.T
	work       string
	disk       string // directory with disk.img and vars.fd
	base       string // http://10.0.2.2:PORT
	sec        string
	pub        string
	hangs      int
	hangLog    []string // every hang: number, step, saved log
	hangDir    string
	forceHang  int      // test hook: treat the next N boots as hangs (HUBOS_TEST_FORCE_HANG=once|twice)
	crashes    int      // times QEMU died from a signal (a crash, not a hang)
	crashLog   []string // every crash: number, signal, step, saved log
	fwd        int      // host port forwarded to port 8480 of the guest (the recovery agent test); 0 = none
	forceCrash int      // test hook: kill the next N boots with SIGSEGV (HUBOS_TEST_FORCE_CRASH=once|twice)
	dead       bool     // the run was stopped by a repeated hang; later VM tests skip
	hungReboot bool     // a reboot hung and was retried: BootNext was used up, so the trial slot did NOT boot
	repeating  bool     // a step is being repeated after such a hang; a second hang now fails the run
	recSig     string   // what the first recovery boot looked like (kernel command line and release), to compare later ones
	vm         *vm
	logSeq     int
	bootPos    int    // where the output of the current boot starts in the VM buffer
	machine    string // MACHINE for the build scripts (default image/machines/qemu-test.build)
	gui        bool   // the hub test: a virtio GPU, a USB keyboard and mouse and a monitor socket (r.monPath)
	monPath    string
	shots      string // where PNG screenshots go
}

func (r *rig) script(name string, env []string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command(filepath.Join("..", "..", "tools", "image", name), args...)
	cmd.Env = append(os.Environ(), append([]string{"WORK=" + r.work}, env...)...)
	if r.machine != "" {
		cmd.Env = append(cmd.Env, "MACHINE="+r.machine)
	}
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

func (r *rig) fwdArg() string {
	if r.fwd == 0 {
		return ""
	}
	return fmt.Sprintf(",hostfwd=tcp:127.0.0.1:%d-:8480", r.fwd)
}

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
		"-netdev", "user,id=n0" + r.fwdArg(), "-device", "virtio-net-pci,netdev=n0,romfile=",
		"-device", "i6300esb", "-watchdog-action", "reset", "-serial", "stdio", "-monitor", "none",
	}
	if r.gui {
		os.Remove(r.monPath)
		args = []string{
			"-L", T + "/usr/share/qemu", "-L", T + "/usr/share/seabios",
			"-machine", "q35,smm=off", "-accel", "tcg", "-smp", "2", "-m", "2048", "-display", "none",
			"-drive", "if=pflash,format=raw,unit=0,readonly=on,file=" + T + "/usr/share/OVMF/OVMF_CODE_4M.fd",
			"-drive", "if=pflash,format=raw,unit=1,file=" + filepath.Join(r.disk, "vars.fd"),
			"-drive", "file=" + filepath.Join(r.disk, "disk.img") + ",if=none,id=d0,format=raw", "-device", "virtio-blk-pci,drive=d0",
			"-netdev", "user,id=n0", "-device", "virtio-net-pci,netdev=n0,romfile=",
			"-device", "i6300esb", "-watchdog-action", "reset",
			"-device", "virtio-vga,xres=1024,yres=640",
			"-device", "qemu-xhci,id=xhci", "-device", "usb-kbd,bus=xhci.0", "-device", "usb-mouse,bus=xhci.0",
			"-serial", "stdio", "-monitor", "unix:" + r.monPath + ",server,nowait",
		}
	}
	cmd := exec.Command(T+"/usr/bin/qemu-system-x86_64", args...)
	cmd.Env = append(os.Environ(),
		"LD_LIBRARY_PATH="+T+"/usr/lib/x86_64-linux-gnu:"+T+"/lib/x86_64-linux-gnu:"+T+"/usr/lib",
		"QEMU_MODULE_DIR="+T+"/usr/lib/x86_64-linux-gnu/qemu")
	in, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	v := &vm{t: r.t, cmd: cmd, in: in, log: lf, done: make(chan struct{}), exit: make(chan struct{}), rig: r}
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
				cmd.Wait() // reap QEMU: its exit status tells a crash (a signal) from a hang
				v.ps = cmd.ProcessState
				close(v.exit)
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
		if sig := v.crashSignal(); sig != "" && !v.crashSeen {
			v.crashSeen = true
			v.rig.noteCrash(v, sig, "a command on the guest: "+cmd)
			v.t.Fatalf("QEMU died from %s while a command was running; a crash in the middle of a step cannot be retried here (only at boot and reboot points), so the run fails", sig)
		}
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
	v.byUs = true
	v.cmd.Process.Kill()
	<-v.exit
	v.log.Close()
}

// crashSignal returns the name of the signal that killed QEMU when it died by itself (for example "segmentation fault"), or
// "" when QEMU is still running, exited normally, or was killed by the test.
func (v *vm) crashSignal() string {
	select {
	case <-v.exit:
	case <-time.After(3 * time.Second):
		return ""
	}
	if v.byUs || v.ps == nil {
		return ""
	}
	if ws, ok := v.ps.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return fmt.Sprintf("signal %d (%s)", int(ws.Signal()), ws.Signal())
	}
	return ""
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
	if r.forceCrash > 0 {
		// Test hook: really crash this boot. QEMU is started, then killed from outside with SIGSEGV, so the code that
		// follows sees a real death by signal (not a kill by the test).
		r.forceCrash--
		time.Sleep(3 * time.Second)
		v.cmd.Process.Signal(syscall.SIGSEGV)
		step += " [forced by HUBOS_TEST_FORCE_CRASH]"
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
	r.t.Fatalf("the machine hung or crashed twice in the same step (%s); the run fails (hangs so far: %s; crashes so far: %s)", step, strings.Join(r.hangLog, "; "), strings.Join(r.crashLog, "; "))
	return nil
}

func (r *rig) ready(v *vm) {
	time.Sleep(2500 * time.Millisecond)
	io.WriteString(v.in, "stty -echo\n")
	time.Sleep(500 * time.Millisecond)
}

// noteCrash records a QEMU that died from a signal (a crash, for example a segmentation fault): not a hang. The callers
// treat it like a hang: the step is retried once, a second one in the same step fails the run.
func (r *rig) noteCrash(v *vm, sig, what string) {
	r.crashes++
	os.MkdirAll(r.hangDir, 0o755)
	dst := filepath.Join(r.hangDir, fmt.Sprintf("crash-%d.log", r.crashes))
	r.crashLog = append(r.crashLog, fmt.Sprintf("crash %d (QEMU died from %s) in step %q (serial log %s)", r.crashes, sig, what, dst))
	os.WriteFile(dst, []byte(v.text(0)), 0o644)
	r.t.Logf("CRASH %d (%s): QEMU died from %s; serial log saved to %s", r.crashes, what, sig, dst)
}

func (r *rig) noteHang(v *vm, what string) {
	if sig := v.crashSignal(); sig != "" {
		r.noteCrash(v, sig, what)
		return
	}
	r.hangs++
	os.MkdirAll(r.hangDir, 0o755)
	dst := filepath.Join(r.hangDir, fmt.Sprintf("hang-%d.log", r.hangs))
	r.hangLog = append(r.hangLog, fmt.Sprintf("hang %d in step %q (serial log %s)", r.hangs, what, dst))
	os.WriteFile(dst, []byte(v.text(0)), 0o644)
	r.t.Logf("HANG %d (%s): no known log line in time; serial log saved to %s", r.hangs, what, dst)
	lines := strings.Split(strings.TrimRight(v.text(0), "\n"), "\n")
	if len(lines) > 14 {
		lines = lines[len(lines)-14:]
	}
	r.t.Logf("last lines of the hung boot:\n%s", strings.Join(lines, "\n"))
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
	if r.repeating {
		r.dead = true
		r.t.Fatalf("the reboot hung again while a step was being repeated after a hang; the run fails")
	}
	r.vm = r.retryBoot("reboot")
	r.hungReboot = true
	return 0
}

// repeatIfHung is called after a reboot that was meant to start a trial slot (BootNext). If that reboot hung, the
// retry booted the confirmed slot instead (BootNext is used up), so the step is repeated once: redo() prepares
// the trial again (for example the update) and the machine is rebooted again. A second hang fails the run.
func (r *rig) repeatIfHung(redo func()) {
	if !r.hungReboot {
		return
	}
	r.hungReboot = false
	r.t.Logf("the reboot hung and was retried (BootNext was lost); repeating the step once")
	redo()
	r.repeating = true
	m := r.vm.mark()
	io.WriteString(r.vm.in, "sync; reboot -f\n")
	r.afterReset(m)
	r.repeating = false
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
	end := r.vm.wait(`confirm: (boot of slot [ab].* confirmed|slot [ab] is already the confirmed slot)`, 90*time.Second, r.bootPos)
	if end < 0 {
		return false
	}
	// The confirm step then looks at the recovery kernel (it keeps the boot partition mounted while it does): wait for its last line, so
	// that a test that mounts the boot partition next does not meet the mount of the confirm step.
	r.vm.wait(`confirm: (recovery kernel|this release carries no recovery kernel)`, 60*time.Second, end)
	return true
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
	setNext := `efibootmgr -q -n $(efibootmgr | sed -n 's/^Boot\([0-9A-F]*\)\*\{0,1\}[[:space:]]hubos-recovery[[:space:]].*/\1/p' | head -n 1)`
	r.sh(setNext)
	m := r.vm.mark()
	io.WriteString(r.vm.in, "sync; reboot -f\n")
	if !r.waitRecovery(r.vm, m) {
		// a hang: restart once (the normal slot boots, BootNext is used up), set BootNext again and repeat once
		r.noteHang(r.vm, "recovery boot")
		r.vm.kill()
		r.retryBoot("recovery boot")
		r.waitConfirmed()
		r.sh(setNext)
		m = r.vm.mark()
		io.WriteString(r.vm.in, "sync; reboot -f\n")
		if !r.waitRecovery(r.vm, m) {
			r.noteHang(r.vm, "recovery boot (repeated)")
			r.dead = true
			r.t.Fatalf("the recovery boot hung twice; the run fails")
		}
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

var failRe = regexp.MustCompile(`boot-failures=(\d+) limit=(\d+)`)

// failures returns "N/limit" from `hubos-ctl status` text.
func failures(st string) string {
	m := failRe.FindStringSubmatch(st)
	if m == nil {
		return "?"
	}
	return m[1] + "/" + m[2]
}

// espRecovery returns the recovery version and kernel hash on the boot partition.
func (r *rig) espRecovery() (ver, hash string) {
	_, o := r.sh(`mount -t vfat $(findfs PARTLABEL=hubos-esp) /boot/efi; echo ver=$(cat /boot/efi/EFI/hubos/recovery.version); sha256sum /boot/efi/EFI/hubos/kernel-recovery.efi; umount /boot/efi`)
	v := regexp.MustCompile(`ver=(\d+)`).FindStringSubmatch(o)
	h := regexp.MustCompile(`[0-9a-f]{64}`).FindString(o)
	if v == nil {
		return "?", h
	}
	return v[1], h
}

// trialBoot reboots into the trial slot that an update just prepared, waits until the slot is confirmed and returns the
// confirm step's recovery lines. If the reboot hung, the update is repeated once.
func (r *rig) trialBoot(redo func()) (slot, rel string, confirmed bool, st, recLog string) {
	m := r.vm.mark()
	io.WriteString(r.vm.in, "sync; reboot -f\n")
	r.afterReset(m)
	r.repeatIfHung(redo)
	r.waitConfirmed()
	r.vm.wait(`confirm: (recovery kernel|this release carries no recovery)[^\n]*`, 40*time.Second, r.bootPos)
	recLog = regexp.MustCompile(`(?m)^confirm: (recovery kernel|this release carries no recovery)[^\n]*`).FindString(r.vm.text(r.bootPos))
	slot, rel, confirmed, st = r.status()
	return
}

// waitRecoveryFor is waitRecovery with its own time limit (a run of failed boots takes a while).
// waitRecoveryRetryCrash waits for the recovery banner on the running VM after position from. If QEMU died from a signal
// meanwhile, the crash is recorded and the step is retried once: QEMU is started again on the same disk (both slot roots are
// garbage when this is used, so the boot-loop breaker brings up the recovery shell) and the banner is waited for again.
func (r *rig) waitRecoveryRetryCrash(from int, step string) bool {
	if r.waitRecoveryFor(r.vm, from, 900*time.Second) {
		return true
	}
	if sig := r.vm.crashSignal(); sig != "" {
		r.noteCrash(r.vm, sig, step)
		return r.restartIntoRecovery(step + " (retry)")
	}
	return false
}

// restartIntoRecovery starts QEMU again on the same disk and waits for the recovery shell (both roots garbage: three failed boots).
func (r *rig) restartIntoRecovery(step string) bool {
	r.vm.kill()
	v := r.startVM()
	if r.waitRecoveryFor(v, 0, 900*time.Second) {
		return true
	}
	if sig := v.crashSignal(); sig != "" {
		r.noteCrash(v, sig, step)
	}
	r.dead = true
	return false
}

// settleOnConfirmedSlot starts QEMU again on the same disk (after a crash) and waits until the machine runs the confirmed slot b:
// if the firmware starts an unconfirmed trial slot first, the trial fails and rolls back, and this waits for that.
func (r *rig) settleOnConfirmedSlot() {
	r.vm.kill()
	v := r.startVM()
	r.vm = v
	pos := 0
	for i := 0; i < 3; i++ {
		e := v.wait(handoverRe, 300*time.Second, pos)
		if e < 0 {
			if sig := v.crashSignal(); sig != "" {
				r.noteCrash(v, sig, "restart after a crash")
			}
			r.dead = true
			r.t.Fatalf("the machine did not come back after the QEMU crash; the run fails")
		}
		r.bootPos = e
		r.ready(v)
		if slot, _, _, _ := r.status(); slot == "b" {
			r.waitConfirmed()
			return
		}
		pos = max(e, v.wait(`this boot FAILED`, 200*time.Second, e))
	}
	r.dead = true
	r.t.Fatalf("the machine did not settle on the confirmed slot after the QEMU crash")
}

func (r *rig) waitRecoveryFor(v *vm, from int, d time.Duration) bool {
	if v.wait(`HUBOS: RECOVERY MODE`, d, from) < 0 {
		return false
	}
	time.Sleep(2500 * time.Millisecond)
	io.WriteString(v.in, "stty -echo\n")
	time.Sleep(500 * time.Millisecond)
	return true
}

func sha256File(t *testing.T, p string) string {
	out, err := exec.Command("sha256sum", p).Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(out))[0]
}

func crashList(r *rig) string {
	if len(r.crashLog) == 0 {
		return "none"
	}
	return "\n  " + strings.Join(r.crashLog, "\n  ")
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
	if ln, err := net.Listen("tcp", "127.0.0.1:0"); err == nil { // a free host port, forwarded to port 8480 of the guest (the recovery agent test)
		r.fwd = ln.Addr().(*net.TCPAddr).Port
		ln.Close()
	}
	switch os.Getenv("HUBOS_TEST_FORCE_HANG") {
	case "once":
		r.forceHang = 1 // the first boot is treated as a hang; the retry must succeed and the run continues
	case "twice":
		r.forceHang = 2 // the first boot and its retry are treated as hangs; the run must fail
	}
	switch os.Getenv("HUBOS_TEST_FORCE_CRASH") {
	case "once":
		r.forceCrash = 1 // the first boot is crashed with SIGSEGV; the retry must succeed and the run continues
	case "twice":
		r.forceCrash = 2 // the first boot and its retry are crashed; the run must fail
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
		t.Logf("\nRESULTS (QEMU hangs seen and retried: %d; every hang: %s; QEMU crashes (died from a signal) seen and retried: %d; every crash: %s)\n%s", r.hangs, hl, r.crashes, crashList(r), strings.Join(results, "\n"))
		if r.vm != nil {
			r.vm.kill()
		}
	}()

	// ---- build ----
	r.script("fetch-tools.sh", nil)
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
	// The recovery kernel carries the PUBLIC update key (so that `hubos-ctl update` works in the recovery shell).
	pub2, sec2k := genKey("next") // the NEXT update key (key 2) for the key rotation tests
	r.script("build-kernel.sh", []string{"UPDATE_PUB=" + r.pub})
	r.script("build-base.sh", nil)
	// Three recovery kernels for the key rotation: key 1 only (the one built above, version 1), keys 1 and 2 (version 4),
	// key 2 only (version 5). (Versions 2 and 3 are used by the recovery-at-confirm tests before the rotation.) Each rebuild only relinks the recovery kernel (the slot kernels are up to date).
	outDir := filepath.Join(work, "out")
	recK1 := filepath.Join(outDir, "rec-k1.efi")
	copyFile(t, filepath.Join(outDir, "kernel-recovery.efi"), recK1)
	r.script("build-kernel.sh", []string{"UPDATE_KEYS=" + r.pub + " " + pub2, "RECOVERY_VERSION=4"})
	recK12 := filepath.Join(outDir, "rec-k12.efi")
	copyFile(t, filepath.Join(outDir, "kernel-recovery.efi"), recK12)
	r.script("build-kernel.sh", []string{"UPDATE_KEYS=" + pub2, "RECOVERY_VERSION=5"})
	recK2 := filepath.Join(outDir, "rec-k2.efi")
	copyFile(t, filepath.Join(outDir, "kernel-recovery.efi"), recK2)
	// The TEST recovery kernel with the recovery agent (T18): key 2 only (the machine runs key-2 releases by then), version 5, the
	// agent built static, and a management key the test signs its requests with. kernel-recovery.efi is not touched by this build.
	mgmtPub, mgmtSec := genKey("mgmt")
	_, otherMgmtSec := genKey("othermgmt")
	agentBin := filepath.Join(outDir, "recovery-agent")
	{
		cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", agentBin, "./experiments/recoveryagent")
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go build recovery-agent: %v\n%s", err, out)
		}
	}
	r.script("build-kernel.sh", []string{"UPDATE_KEYS=" + pub2, "RECOVERY_VERSION=5", "RECOVERY_AGENT_BIN=" + agentBin, "RECOVERY_AGENT_KEYS=" + mgmtPub})
	recAgent := filepath.Join(outDir, "kernel-recovery-agent.efi")
	r.script("build-kernel.sh", []string{"UPDATE_PUB=" + r.pub}) // back to key 1 only, version 1: the kernel the disk and the bundles use
	if sha256File(t, recK1) != sha256File(t, filepath.Join(outDir, "kernel-recovery.efi")) {
		t.Fatalf("the key-1 recovery kernel was not rebuilt identically")
	}

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
	// The recovery kernel travels inside the root of a release. Other recovery kernels for the tests: the real one with a
	// marker appended (still boots), so that a hash and a version can be told apart.
	markerKernel := func(name, marker string) string {
		b, err := os.ReadFile(filepath.Join(work, "out", "kernel-recovery.efi"))
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(work, "out", name)
		if err := os.WriteFile(p, append(b, []byte(marker)...), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	rec2, rec3 := markerKernel("recovery2.efi", "hubos-test-marker-2"), markerKernel("recovery3.efi", "hubos-test-marker-3")
	sha := func(p string) string {
		out, _ := exec.Command("sha256sum", p).Output()
		return strings.Fields(string(out))[0]
	}
	recHash0, recHash2, recHash3 := sha(filepath.Join(work, "out", "kernel-recovery.efi")), sha(rec2), sha(rec3)
	// bundles for the recovery-kernel-at-confirm rules: none in the release, a newer one, the same version again, a newer one in a
	// release that never gets healthy
	bundle("9", "good", "v9-norecovery", "NO_RECOVERY=1")
	bundle("10", "good", "v10-recovery2", "RECOVERY_VERSION_OVERRIDE=2", "RECOVERY_KERNEL_FILE="+rec2)
	bundle("11", "good", "v11-recovery2-again", "RECOVERY_VERSION_OVERRIDE=2", "RECOVERY_KERNEL_FILE="+rec2)
	bundle("12", "unhealthy", "v12-unhealthy-recovery3", "RECOVERY_VERSION_OVERRIDE=3", "RECOVERY_KERNEL_FILE="+rec3)
	// releases installed from the recovery shell, and the three kinds of bad boot for the boot-loop breaker
	bundle("13", "good", "v13-good")
	bundle("14", "unhealthy", "v14-unhealthy")
	bundle("15", "noinit", "v15-noinit")
	bundle("16", "garbage", "v16-garbage")
	bundle("17", "unhealthy", "v17-unhealthy")
	// the key rotation (docs/image.md): a bundle signed with key 2 that carries only key 2; a release signed with key 1 that
	// carries keys 1 and 2 and a recovery kernel that knows both; a release signed with key 2 that drops key 1 (recovery
	// kernel with key 2 only); and a bundle signed with key 1 after that
	bundleSigned := func(ver, name, sec string, env ...string) {
		r.script("build-bundle.sh", env, ver, "good", filepath.Join(bdir, name), r.pub, sec)
	}
	bundleSigned("20", "v20-k2only", sec2k, "KEYRING_PUBS="+pub2)
	bundleSigned("21", "v21-rotate", r.sec, "KEYRING_PUBS="+r.pub+" "+pub2, "RECOVERY_KERNEL_FILE="+recK12, "RECOVERY_VERSION_OVERRIDE=4")
	bundleSigned("22", "v22-k2-drops-k1", sec2k, "KEYRING_PUBS="+pub2, "RECOVERY_KERNEL_FILE="+recK2, "RECOVERY_VERSION_OVERRIDE=5")
	bundleSigned("23", "v23-k1-after-drop", r.sec)
	bundleSigned("24", "v24-k2", sec2k, "KEYRING_PUBS="+pub2, "RECOVERY_KERNEL_FILE="+recK2, "RECOVERY_VERSION_OVERRIDE=5")
	bundleSigned("28", "v28-agent", sec2k, "KEYRING_PUBS="+pub2, "RECOVERY_KERNEL_FILE="+recK2, "RECOVERY_VERSION_OVERRIDE=5")
	os.MkdirAll(filepath.Join(bdir, "agent-kernel"), 0o755)
	copyFile(t, recAgent, filepath.Join(bdir, "agent-kernel", "kernel-recovery.efi"))
	// the recovery install rule (T17): a changed file at the SAME version (a rotation release that forgot to bump it) installs; a
	// lower version never installs; the same file leaves things alone
	recK2m := filepath.Join(outDir, "rec-k2-marked.efi") // the key-2 recovery kernel with a marker appended: another file, still boots
	if kb, err := os.ReadFile(recK2); err != nil {
		t.Fatal(err)
	} else if err := os.WriteFile(recK2m, append(kb, []byte("hubos-test-marker-k2m")...), 0o644); err != nil {
		t.Fatal(err)
	}
	recK2mHash := sha(recK2m)
	bundleSigned("25", "v25-same-version-new-file", sec2k, "KEYRING_PUBS="+pub2, "RECOVERY_KERNEL_FILE="+recK2m, "RECOVERY_VERSION_OVERRIDE=5")
	bundleSigned("26", "v26-lower-version", sec2k, "KEYRING_PUBS="+pub2, "RECOVERY_KERNEL_FILE="+recK12, "RECOVERY_VERSION_OVERRIDE=4")
	bundleSigned("27", "v27-same-file", sec2k, "KEYRING_PUBS="+pub2, "RECOVERY_KERNEL_FILE="+recK2m, "RECOVERY_VERSION_OVERRIDE=5")
	// refused variants: made from v2-good with hard links
	variant := func(name string, change func(dir string)) {
		d := filepath.Join(bdir, name)
		os.MkdirAll(d, 0o755)
		for _, f := range []string{"manifest", "manifest.sig", "kernel-a.efi", "kernel-b.efi", "rootfs.sqsh"} {
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
	// a correctly signed manifest whose recovery hash does not match the recovery kernel inside the root
	variant("v13-recoverymismatch", func(d string) {
		os.Remove(filepath.Join(d, "manifest"))
		os.Remove(filepath.Join(d, "manifest.sig"))
		for _, f := range []string{"kernel-a.efi", "kernel-b.efi", "rootfs.sqsh"} {
			os.Remove(filepath.Join(d, f))
			if err := os.Link(filepath.Join(bdir, "v13-good", f), filepath.Join(d, f)); err != nil {
				t.Fatal(err)
			}
		}
		m, _ := os.ReadFile(filepath.Join(bdir, "v13-good", "manifest"))
		re := regexp.MustCompile(`(?m)^kernel-recovery\.efi sha256 [0-9a-f]+`)
		os.WriteFile(filepath.Join(d, "manifest"), re.ReplaceAll(m, []byte("kernel-recovery.efi sha256 "+strings.Repeat("ab", 32))), 0o644)
		resign(d)
	})
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
		ok := checkNoSystemd(t, root, false)
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
		hashOf := func(x string) string { return regexp.MustCompile(`[0-9a-f]{64}`).FindString(x) }
		if hashOf(beforeB) == "" || hashOf(beforeB) != hashOf(afterB) {
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
		r.repeatIfHung(func() { r.update("v2-good") })
		rebootSec := time.Since(bt).Seconds()
		r.waitConfirmed()
		slot, rel, confirmed, st5 := r.status()
		_, marker := r.sh(`cat /config/hubos/marker; cat /config/hubos/state/min_version`)
		_, old := r.update("v1-good")
		ok := rc == 0 && pinned && slot == "b" && rel == "2" && confirmed && strings.Contains(marker, "survive-me") && strings.Contains(marker, "2") && strings.Contains(old, "REFUSED") && failures(st5) == "0/3"
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
			var rc, saw int
			var m int
			trial := ""
			var sawAfter float64
			for attempt := 1; attempt <= 2; attempt++ {
				rc, _ = r.update(bundleName)
				m = r.vm.mark()
				io.WriteString(r.vm.in, "sync; reboot -f\n")
				trial = ""
				if strings.HasPrefix(name, "6c") {
					// the unconfirmed trial boot of slot a is up (hubd never becomes healthy): where does recovery point now?
					if hh := r.vm.wait(handoverRe, 150*time.Second, m); hh >= 0 {
						r.ready(r.vm)
						_, e := r.vm.sh(`efibootmgr -v | grep hubos-recovery | sed 's/.*File/File/'`, 30*time.Second)
						trial = regexp.MustCompile(`File\S+`).FindString(e)
						t.Logf("during the unconfirmed trial boot of slot a the recovery entry is: %s", trial)
					}
				}
				saw = r.vm.wait(expect, 200*time.Second, m)
				sawAfter = time.Since(start).Seconds()
				if saw < 0 && attempt == 1 {
					if sig := r.vm.crashSignal(); sig != "" {
						// QEMU died from a signal during the trial boot: record it, start QEMU again, wait until the machine is back on the
						// confirmed slot, and do the whole step once more (a second crash here fails the step)
						r.noteCrash(r.vm, sig, "the trial boot of "+name)
						r.settleOnConfirmedSlot()
						continue
					}
				}
				break
			}
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
			slot, rel, confirmed, stb := r.status()
			ok := rc == 0 && saw >= 0 && slot == "b" && rel == "2" && confirmed && failures(stb) == "0/3"
			note := fmt.Sprintf("failure line seen after %.1f s; rollback complete after %.1f s; failure counter %s after the rollback (recovery not triggered)", sawAfter, time.Since(start).Seconds(), failures(stb))
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
	badBoot("6c bad boot rolls back: boots but never gets healthy (confirm times out)", "v5-unhealthy", `this boot FAILED|starting Boot[0-9A-F]{4} "hubos-b"`)
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
		r.repeatIfHung(func() { r.update("v7-good") })
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
		r.repeatIfHung(func() {
			r.sh(`efibootmgr -q -n $(efibootmgr | sed -n 's/^Boot\([0-9A-F]*\)\*\{0,1\}[[:space:]]hubos-b.*/\1/p' | head -1)`)
		})
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

	// ---- the recovery kernel is installed at the confirm step, never by the update ----
	t.Run("T12_recovery_kernel_installed_at_confirm", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		v0, h0 := r.espRecovery()
		okStart := v0 == "1" && h0 == recHash0
		// 1. a release without a recovery kernel: the update and the confirm leave recovery alone
		rc1, u1 := r.update("v9-norecovery")
		va, ha := r.espRecovery()
		slot1, rel1, conf1, _, log1 := r.trialBoot(func() { r.update("v9-norecovery") })
		v1, h1 := r.espRecovery()
		ok1 := okStart && rc1 == 0 && strings.Contains(u1, "the manifest has no recovery kernel") && va == v0 && ha == h0 && slot1 == "a" && rel1 == "9" && conf1 &&
			strings.Contains(log1, "carries no recovery kernel") && v1 == v0 && h1 == h0
		// 2. a newer recovery kernel: the UPDATE does not touch the boot partition; the CONFIRM step installs it
		rc2, u2 := r.update("v10-recovery2")
		vb, hb := r.espRecovery()
		slot2, rel2, conf2, _, log2 := r.trialBoot(func() { r.update("v10-recovery2") })
		v2, h2 := r.espRecovery()
		ok2 := rc2 == 0 && strings.Contains(u2, "installed at the confirm step, not now") && vb == "1" && hb == h0 && slot2 == "b" && rel2 == "10" && conf2 &&
			strings.Contains(log2, "recovery kernel version 2 installed (was 1)") && v2 == "2" && h2 == recHash2
		// 3. the same recovery version again: left alone at confirm
		rc3, u3 := r.update("v11-recovery2-again")
		slot3, rel3, conf3, _, log3 := r.trialBoot(func() { r.update("v11-recovery2-again") })
		v3, h3 := r.espRecovery()
		ok3 := rc3 == 0 && strings.Contains(u3, "installed at the confirm step, not now") && slot3 == "a" && rel3 == "11" && conf3 &&
			strings.Contains(log3, "version 2 is the same file as the installed one; left alone") && v3 == "2" && h3 == recHash2
		t.Logf("recovery version/hash on the boot partition: start %s %s; after update 9 %s %s, after its confirm %s %s; after update 10 %s %s, after its confirm %s %s; after 11 %s %s", v0, h0[:12], va, ha[:12], v1, h1[:12], vb, hb[:12], v2, h2[:12], v3, h3[:12])
		record("C1 the recovery kernel is installed at the confirm step: a release without one leaves recovery alone; the update itself never touches the boot partition; a newer recovery-version is installed after the healthy boot; the same file is left alone", ok1 && ok2 && ok3, time.Since(start),
			fmt.Sprintf("installed version %s -> %s (update 9, confirm) -> %s (update 10: unchanged until its confirm) -> %s -> %s (update 11)", v0, v1, vb, v2, v3))
		if !ok1 || !ok2 || !ok3 {
			t.Errorf("ok1=%v ok2=%v ok3=%v\n%s\n%s\n%s\n%q %q %q", ok1, ok2, ok3, u1, u2, u3, log1, log2, log3)
			t.Fail()
		}
		// 4. a trial boot that never confirms (hubd never healthy) leaves the old recovery kernel in place
		start4 := time.Now()
		rc4, _ := r.update("v12-unhealthy-recovery3")
		vc, hc := r.espRecovery()
		m := r.vm.mark()
		io.WriteString(r.vm.in, "sync; reboot -f\n")
		saw := r.vm.wait(`this boot FAILED|BdsDxe: starting Boot[0-9A-F]{4} "hubos-a"`, 200*time.Second, m)
		hh := r.vm.wait(handoverRe, 300*time.Second, max(saw, m))
		if hh >= 0 {
			r.bootPos = hh
			r.ready(r.vm)
		} else {
			r.noteHang(r.vm, "rollback after the unconfirmed recovery-3 trial")
			r.vm.kill()
			r.vm = r.retryBoot("rollback after the unconfirmed recovery-3 trial")
		}
		r.waitConfirmed()
		slot4, rel4, conf4, _ := r.status()
		v4, h4 := r.espRecovery()
		ok4 := rc4 == 0 && saw >= 0 && vc == "2" && hc == recHash2 && slot4 == "a" && rel4 == "11" && conf4 && v4 == "2" && h4 == recHash2 && h4 != recHash3
		record("C2 a trial boot that never confirms leaves the old recovery kernel in place (release 12 carries recovery version 3 but was rolled back)", ok4, time.Since(start4),
			fmt.Sprintf("boot partition before the trial %s, after the rollback %s (still the version-2 file); back on slot %s release %s", vc, v4, slot4, rel4))
		if !ok4 {
			t.Errorf("saw=%d vc=%s v4=%s slot=%s rel=%s conf=%v hashOK=%v", saw, vc, v4, slot4, rel4, conf4, h4 == recHash2)
			t.Fail()
		}
		// 5. a correctly signed manifest whose recovery hash does not match the file inside the root is refused
		start5 := time.Now()
		rc5, u5 := r.update("v13-recoverymismatch")
		_, st5 := r.sh("hubos-ctl status")
		ok5 := rc5 == 2 && strings.Contains(u5, "REFUSED: the recovery kernel inside the new root does not match the manifest") && !strings.Contains(u5, "kernel installed") && !strings.Contains(st5, "BootNext")
		record("C3 a signed manifest whose recovery hash does not match the recovery kernel inside the new root is refused; BootNext is not set", ok5, time.Since(start5), fmt.Sprintf("rc=%d", rc5))
		if !ok5 {
			t.Errorf("rc=%d\n%s\n%s", rc5, u5, st5)
			t.Fail()
		}
	})

	// ---- the per-machine timeouts ----
	t.Run("T15_per_machine_timeouts", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		chk := filepath.Join("..", "..", "image", "rootfs", "usr", "lib", "hubos", "check-timers.sh")
		run := func(args ...string) (int, string) {
			out, err := exec.Command(chk, args...).CombinedOutput()
			rc := 0
			if ee, ok := err.(*exec.ExitError); ok {
				rc = ee.ExitCode()
			}
			return rc, string(out)
		}
		rcA, _ := run("30", "60")
		rcB, oB := run("30", "45") // exactly confirm + 15: refused
		rcC, _ := run("30", "46")
		rcD, oD := run("30", "abc")
		rcE, _ := run("120", "180")
		checkOK := rcA == 0 && rcB == 1 && strings.Contains(oB, "REFUSED") && rcC == 0 && rcD == 1 && strings.Contains(oD, "REFUSED") && rcE == 0
		// the build scripts refuse a bad pair
		// a machine file with a bad pair (the later assignment wins), selected with MACHINE
		mf, _ := os.ReadFile(filepath.Join("..", "..", "image", "machines", "qemu-test.build"))
		badMachine := filepath.Join(work, "bad-timers.build")
		os.WriteFile(badMachine, append(mf, []byte("\nCONFIRM_TIMEOUT=60\nWATCHDOG_TIMEOUT=70\n")...), 0o644)
		repoAbs, _ := filepath.Abs(filepath.Join("..", ".."))
		machineRel, _ := filepath.Rel(repoAbs, badMachine)
		bad := func(script string, args ...string) (int, string) {
			cmd := exec.Command(filepath.Join("..", "..", "tools", "image", script), args...)
			cmd.Env = append(os.Environ(), "WORK="+work, "MACHINE="+machineRel)
			out, err := cmd.CombinedOutput()
			rc := 0
			if ee, ok := err.(*exec.ExitError); ok {
				rc = ee.ExitCode()
			}
			return rc, string(out)
		}
		tmpDisk := filepath.Join(work, "baddisk")
		rcK, oK := bad("build-kernel.sh")
		rcDk, oDk := bad("build-disk.sh", filepath.Join(bdir, "v1-good", "rootfs.sqsh"), filepath.Join(work, "out", "kernel-a.efi"), tmpDisk)
		_, noDisk := os.Stat(filepath.Join(tmpDisk, "disk.img"))
		buildOK := rcK != 0 && strings.Contains(oK, "REFUSED") && rcDk != 0 && strings.Contains(oDk, "REFUSED") && noDisk != nil
		record("D1 build time: the check refuses a watchdog timeout that is not more than the confirm timeout plus 15 s (30/45 refused, 30/46 fine); build-kernel.sh and build-disk.sh refuse a bad pair (60/70) and make nothing", checkOK && buildOK, time.Since(start), fmt.Sprintf("check rc: %d %d %d %d %d; kernel script rc=%d, disk script rc=%d, no disk.img: %v", rcA, rcB, rcC, rcD, rcE, rcK, rcDk, noDisk != nil))
		if !checkOK || !buildOK {
			t.Errorf("check: %d %d %d %d %d; kernel rc=%d %q; disk rc=%d %q noDisk=%v", rcA, rcB, rcC, rcD, rcE, rcK, oK, rcDk, oDk, noDisk)
			t.Fail()
		}
		// at boot: a refused pair in the node config falls back to the defaults; a good pair is used
		start2 := time.Now()
		setConf := func(c, w string) {
			r.sh(fmt.Sprintf(`sed -i 's/^CONFIRM_TIMEOUT=.*/CONFIRM_TIMEOUT=%s/; s/^WATCHDOG_TIMEOUT=.*/WATCHDOG_TIMEOUT=%s/' /config/hubos/node.conf; sync; grep -E 'TIMEOUT' /config/hubos/node.conf`, c, w))
		}
		bootWith := func() (string, string) {
			m := r.vm.mark()
			io.WriteString(r.vm.in, "sync; reboot -f\n")
			r.afterReset(m)
			r.waitConfirmed()
			_, wd := r.sh(`i=0; while [ -z "$(pidof watchdog)" ] && [ $i -lt 20 ]; do sleep 1; i=$((i+1)); done; for p in $(pidof watchdog); do tr '\000' ' ' < /proc/$p/cmdline; echo; done`)
			wd = regexp.MustCompile(`watchdog -F -t 5 -T \d+`).FindString(wd)
			return r.vm.text(m), wd
		}
		setConf("30", "40")
		t1, wd1 := bootWith()
		refused := strings.Contains(t1, "STAGE0: REFUSED: watchdog timeout 40s must be more than the confirm timeout 30s plus 15s") && strings.Contains(t1, "using the defaults 120 s and 180 s") &&
			strings.Contains(t1, "confirm timeout 120 s, watchdog timeout 180 s") && strings.Contains(wd1, "-T 180")
		setConf("20", "50")
		t2, wd2 := bootWith()
		used := strings.Contains(t2, "re-armed with 50 s") && strings.Contains(t2, "confirm timeout 20 s, watchdog timeout 50 s") && strings.Contains(wd2, "-T 50") && !strings.Contains(t2, "REFUSED")
		setConf("30", "60")
		r.sh(`sync`)
		record("D2 boot time: a pair in the node config with watchdog <= confirm + 15 s is refused with a message and the defaults (120 s, 180 s) are used; a good pair (20/50) is used by stage 0, the confirm step and the watchdog feeder", refused && used, time.Since(start2), fmt.Sprintf("refused pair 30/40 -> watchdog feeder '%s'; good pair 20/50 -> '%s'", strings.TrimSpace(wd1), strings.TrimSpace(wd2)))
		if !refused || !used {
			t.Errorf("refused=%v used=%v\n%s\n%s\n---\n%s\n%s", refused, used, firstLines(t1, 30), wd1, firstLines(t2, 30), wd2)
			t.Fail()
		}
	})

	// ---- the recovery shell installs a signed bundle (both roots garbage) ----
	t.Run("T13_recovery_installs_a_bundle", func(t *testing.T) {
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
		if !banner {
			if sig := r.vm.crashSignal(); sig != "" {
				// QEMU died from a signal at this reboot. Retry the step once: start QEMU again on the same disk. Both roots are
				// garbage, so the machine fails its boots and the boot-loop breaker brings up the recovery shell.
				r.noteCrash(r.vm, sig, "recovery boot with both roots garbage")
				r.vm.kill()
				v2 := r.startVM()
				banner = r.waitRecoveryFor(v2, 0, 900*time.Second)
				if !banner {
					if sig2 := v2.crashSignal(); sig2 != "" {
						r.noteCrash(v2, sig2, "recovery boot with both roots garbage (retry)")
					}
					r.dead = true
					t.Fatalf("QEMU crashed or the recovery shell did not come up twice in this step; the run fails")
				}
			}
		}
		var shell string
		if banner {
			_, shell = r.vm.sh(recoveryCmds, 60*time.Second)
			t.Logf("recovery shell with both roots garbage:\n%s", shell)
		}
		sig := recoverySig(shell)
		_, tools := r.sh(`e2fsck -fn $(findfs PARTLABEL=hubos-config) 2>&1 | tail -n 2; blkid | sed 's/ UUID.*PARTLABEL/ PARTLABEL/' | head -n 8; ip -4 addr show eth0 | grep inet; wget -q -O /tmp/m ` + r.base + `/v1-good/manifest && echo wget-ok $(wc -c < /tmp/m) bytes; signify-openbsd 2>&1 | head -n 1; efibootmgr | head -n 2; ls /etc/hubos/keys`)
		okTools := regexp.MustCompile(`hubos-config: \d+/\d+ files`).MatchString(tools) && strings.Contains(tools, "wget-ok") && strings.Contains(tools, "inet ") && strings.Contains(tools, `PARTLABEL="hubos-root-a"`) && strings.Contains(tools, ".pub")
		ok := banner && sig == r.recSig && okTools
		record("C4 recovery boots with BOTH slot roots garbage, the same way as before; its shell runs hubos-ctl status, e2fsck, findfs/blkid, ip and wget, and holds the update public key", ok, time.Since(start), "sig: "+sig)
		if !ok {
			t.Errorf("banner=%v sig=%q want %q\n%s", banner, sig, r.recSig, tools)
			t.Fail()
		}
		if !banner {
			return
		}
		// the watchdog in recovery: armed with the machine's timeout (60 s here, set by test 15) and fed by a small feeder; a normal
		// recovery stays up for longer than the timeout; with the feeder killed the machine resets and goes back through stage 0
		startW := time.Now()
		_, w1 := r.sh(`for p in $(pidof watchdog); do tr '\000' ' ' < /proc/$p/cmdline; echo; done; cut -d. -f1 /proc/uptime`)
		feederOK := strings.Contains(w1, "watchdog -F -T 60 -t 5 /dev/watchdog")
		_, w2 := r.vm.sh(`sleep 80; cut -d. -f1 /proc/uptime`, 150*time.Second)
		upNums := regexp.MustCompile(`(?m)^\d+\s*$`).FindAllString(w2, -1)
		uptime := 0
		if len(upNums) > 0 {
			fmt.Sscanf(strings.TrimSpace(upNums[len(upNums)-1]), "%d", &uptime)
		}
		stayedUp := uptime > 80 && !strings.Contains(w2, "<timeout>")
		// kill the feeder; at most twice (a QEMU crash in the middle is retried once, see waitRecoveryRetryCrash)
		var resetPos int
		var resetAfter time.Duration
		var backInRecovery bool
		for attempt := 1; attempt <= 2; attempt++ {
			mW := r.vm.mark()
			r.vm.sh(`kill -9 $(pidof watchdog); echo killed`, 30*time.Second)
			killedAt := time.Now()
			resetPos = r.vm.wait(`STAGE0: start`, 240*time.Second, mW)
			resetAfter = time.Since(killedAt)
			if resetPos >= 0 {
				backInRecovery = r.waitRecoveryRetryCrash(resetPos, "recovery after the watchdog reset")
				break
			}
			if sig := r.vm.crashSignal(); sig != "" && attempt == 1 {
				r.noteCrash(r.vm, sig, "the watchdog reset of the recovery shell")
				if !r.restartIntoRecovery("the watchdog reset of the recovery shell (retry)") {
					break
				}
				continue
			}
			break
		}
		okW := feederOK && stayedUp && resetPos >= 0 && backInRecovery
		record("W1 recovery arms the hardware watchdog with the machine's timeout and feeds it: the feeder runs with -T 60, a normal recovery stays up longer than the timeout (uptime "+fmt.Sprint(uptime)+" s), and with the feeder killed the machine resets and goes through stage 0 and back to recovery", okW, time.Since(startW),
			fmt.Sprintf("feeder command line ok: %v; reset %.0f s after the kill; back in the recovery shell: %v", feederOK, resetAfter.Seconds(), backInRecovery))
		if !okW {
			t.Errorf("feederOK=%v stayedUp=%v resetPos=%d back=%v\n%s\n%s", feederOK, stayedUp, resetPos, backInRecovery, w1, w2)
			t.Fail()
			if !backInRecovery {
				return
			}
		}
		// refusals in the recovery shell: unsigned, tampered, below the floor; nothing written
		start2 := time.Now()
		rootSum := `for s in a b; do head -c 4194304 $(findfs PARTLABEL=hubos-root-$s) | sha256sum; done`
		_, before := r.sh(rootSum)
		all := true
		_, fl := r.sh(`cat /config/hubos/state/min_version`)
		for _, c := range []struct{ name, want string }{{"v2-unsigned", "no signature"}, {"v2-badmanifest", "bad signature"}, {"v2-good", "below the floor"}, {"v20-k2only", "bad signature"}} {
			rc, out := r.vm.sh("hubos-ctl update "+r.base+"/"+c.name+" a 2>&1", 120*time.Second)
			good := rc == 2 && strings.Contains(out, "REFUSED") && strings.Contains(out, c.want) && !strings.Contains(out, "wrote ") && !strings.Contains(out, "kernel installed")
			t.Logf("recovery update %s: rc=%d\n%s", c.name, rc, out)
			if !good {
				t.Errorf("%s: expected REFUSED/%q, got rc=%d:\n%s", c.name, c.want, rc, out)
				all = false
			}
		}
		_, after := r.sh(rootSum)
		_, st := r.sh("hubos-ctl status")
		same := before == after && regexp.MustCompile(`[0-9a-f]{64}`).FindString(before) != ""
		record("R3 recovery (whose kernel has key 1 only) refuses an unsigned, a tampered, a below-the-floor and a key-2-signed bundle (floor "+strings.TrimSpace(regexp.MustCompile(`\d+`).FindString(fl))+"); neither root and no BootNext was touched", all && same && !strings.Contains(st, "BootNext"), time.Since(start2), "4 bundles offered in the recovery shell")
		if !all || !same || strings.Contains(st, "BootNext") {
			t.Errorf("all=%v same=%v\n%s", all, same, st)
			t.Fail()
		}
		// the install: v13 into slot a (both roots garbage); the machine then boots it
		start3 := time.Now()
		rcI, outI := r.vm.sh("hubos-ctl update "+r.base+"/v13-good a 2>&1", 300*time.Second)
		t.Logf("recovery update v13-good a: rc=%d\n%s", rcI, firstLines(outI, 40))
		m = r.vm.mark()
		io.WriteString(r.vm.in, "sync; reboot -f\n")
		r.afterReset(m)
		r.hungReboot = false // a hang here only loses BootNext; slot a is first in BootOrder and boots anyway
		r.waitConfirmed()
		slot, rel, confirmed, stI := r.status()
		okI := rcI == 0 && strings.Contains(outI, "installed version 13 in slot a") && strings.Contains(outI, "recovery kernel inside the new root matches the manifest") && slot == "a" && rel == "13" && confirmed && failures(stI) == "0/3"
		record("R4 recovery installs a signed bundle (release 13) into slot a with both roots garbage; the machine then boots it and confirms it", okI, time.Since(start3), fmt.Sprintf("rc=%d; afterwards slot %s release %s, confirmed %v, failure counter %s", rcI, slot, rel, confirmed, failures(stI)))
		if !okI {
			t.Errorf("rc=%d slot=%s rel=%s confirmed=%v\n%s\n%s", rcI, slot, rel, confirmed, outI, stI)
			t.Fail()
		}
	})

	// ---- the boot-loop breaker ----
	t.Run("T14_boot_loop_breaker", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		// state: slot a runs release 13 (confirmed, floor 13); slot b is garbage; the failure counter is 0.
		// P2: the counter survives kill -9 of QEMU. An update to slot b whose release never gets healthy is started
		// (a trial boot); QEMU is killed right after stage 0 has counted the boot.
		_, st0 := r.sh("hubos-ctl status")
		rc, _ := r.update("v14-unhealthy")
		m := r.vm.mark()
		io.WriteString(r.vm.in, "sync; reboot -f\n")
		sw := r.vm.wait(`STAGE0: switching to slot b`, 200*time.Second, m)
		if sw < 0 {
			t.Fatalf("the trial boot of slot b did not start")
		}
		r.vm.kill()
		r.vm = r.bootVM("start after kill -9 during a counted boot")
		counted := regexp.MustCompile(`STAGE0: failed boots counted so far: (\d+) \(limit 3\)`).FindAllStringSubmatch(r.vm.text(0), -1)
		seen := "none"
		if len(counted) > 0 {
			seen = counted[len(counted)-1][1]
		}
		r.waitConfirmed()
		slot, rel, confirmed, st1 := r.status()
		okKill := rc == 0 && seen == "1" && slot == "a" && rel == "13" && confirmed && failures(st1) == "0/3" && failures(st0) == "0/3"
		record("P2 the failure counter survives kill -9 of QEMU (the variable is in the firmware's store), a normal update trial does not trip the breaker, and the healthy boot afterwards clears it", okKill, time.Since(start),
			fmt.Sprintf("counter before %s; stage 0 read %s after the kill; after the confirm %s; slot %s release %s", failures(st0), seen, failures(st1), slot, rel))
		if !okKill {
			t.Errorf("rc=%d seen=%s slot=%s rel=%s confirmed=%v st0=%s st1=%s", rc, seen, slot, rel, confirmed, failures(st0), failures(st1))
			t.Fail()
		}
		// P1: three kinds of failing boot lead to the recovery shell after N=3 failed boots, whatever kind they are.
		// The bad releases are installed into slot a FROM the recovery shell (the confirmed slot is the one that fails).
		start1 := time.Now()
		banner, _ := r.bootRecoveryEntry()
		if !banner {
			t.Fatalf("no recovery shell")
		}
		kinds := []struct{ name, short, bundle, line string }{
			{"stage 0 cannot mount the root (garbage root)", "garbage root", "v16-garbage", `STAGE0: cannot mount the root of slot a`},
			{"no /sbin/init in the root", "no init", "v15-noinit", `STAGE0: slot a has no /sbin/init`},
			{"init runs but the boot never gets healthy (confirm timeout)", "never healthy", "v17-unhealthy", `STAGE0: switching to slot a`},
		}
		allKinds := true
		var notes []string
		for _, k := range kinds {
			ks := time.Now()
			rcK, outK := r.vm.sh("hubos-ctl update "+r.base+"/"+k.bundle+" a 2>&1", 300*time.Second)
			m := r.vm.mark()
			io.WriteString(r.vm.in, "sync; reboot -f\n")
			got := r.waitRecoveryFor(r.vm, m, 420*time.Second)
			txt := r.vm.text(m)
			counts := regexp.MustCompile(`STAGE0: failed boots counted so far: (\d+) \(limit 3\)`).FindAllStringSubmatch(txt, -1)
			var seq []string
			for _, c := range counts {
				seq = append(seq, c[1])
			}
			fails := strings.Count(txt, k.line)
			sent := strings.Contains(txt, "3 failed boots in a row (limit 3): starting the recovery shell")
			var sh string
			if got {
				_, sh = r.vm.sh(recoveryCmds, 60*time.Second)
			}
			good := rcK == 0 && got && sent && strings.Join(seq, ",") == "0,1,2,3" && fails == 3 && failures(sh) == "3/3" && recoverySig(sh) == r.recSig
			t.Logf("%s: update rc=%d; counts seen by stage 0 %v; failing boots %d; recovery shell reached %v; counter in recovery %s", k.name, rcK, seq, fails, got, failures(sh))
			if !good {
				t.Errorf("%s: not as expected\n%s\n---\n%s", k.name, firstLines(outK, 6), firstLines(txt, 40))
				allKinds = false
			}
			notes = append(notes, fmt.Sprintf("%s: stage 0 counts %s, %d failing boots, recovery after %.0f s", k.short, strings.Join(seq, ","), fails, time.Since(ks).Seconds()))
			if !got {
				break
			}
		}
		record("P1 three kinds of failing boot (garbage root, no init, never healthy) each end in the recovery shell after 3 failed boots in a row; stage 0 counted 0,1,2,3; the shell shows the counter 3/3", allKinds, time.Since(start1), strings.Join(notes, "; "))
		if !allKinds {
			t.Fail()
		}
		// P3: a good boot after recovery works: install release 13 again from the recovery shell, boot it, confirm
		start3 := time.Now()
		rcG, outG := r.vm.sh("hubos-ctl update "+r.base+"/v13-good a 2>&1", 300*time.Second)
		_, stR := r.vm.sh("hubos-ctl status", 30*time.Second)
		m = r.vm.mark()
		io.WriteString(r.vm.in, "sync; reboot -f\n")
		r.afterReset(m)
		r.waitConfirmed()
		slotG, relG, confG, stG := r.status()
		okG := rcG == 0 && strings.Contains(outG, "boot-failure counter cleared (fresh install)") && failures(stR) == "0/3" && slotG == "a" && relG == "13" && confG && failures(stG) == "0/3"
		record("P3 a good boot after recovery works: the install from the recovery shell clears the counter, the machine boots the release and confirms it", okG, time.Since(start3), fmt.Sprintf("counter in recovery after the install %s; afterwards slot %s release %s, confirmed %v, counter %s", failures(stR), slotG, relG, confG, failures(stG)))
		if !okG {
			t.Errorf("rc=%d\n%s\n%s", rcG, firstLines(outG, 20), stG)
			t.Fail()
		}
	})

	// ---- the update keyring and a key rotation ----
	t.Run("T16_key_rotation", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		keyInfo := func() (n string, list string) {
			_, o := r.sh(`ls /etc/hubos/keys | wc -l; ls /etc/hubos/keys | tr '\n' ' '`)
			f := strings.Fields(o)
			if len(f) == 0 {
				return "?", o
			}
			return f[0], strings.Join(f[1:], " ")
		}
		sigRe := regexp.MustCompile(`signature OK \(key ([0-9a-f]{16}), (\d+) key\(s\) in the keyring\)`)
		// state: slot a runs release 13 signed with key 1; the keyring has key 1 only
		n0, l0 := keyInfo()
		// K0: a bundle signed with key 2 is refused while only key 1 is in the keyring
		rc0, o0 := r.update("v20-k2only")
		ok0 := n0 == "1" && rc0 == 2 && strings.Contains(o0, "REFUSED: bad signature") && strings.Contains(o0, "any of the 1 key(s)")
		// K1: a release signed with key 1 that carries key 2 (and a recovery kernel that knows both) is accepted ...
		rc1, o1 := r.update("v21-rotate")
		m1 := sigRe.FindStringSubmatch(o1)
		slot1, rel1, conf1, _, log1 := r.trialBoot(func() { r.update("v21-rotate") })
		n1, l1 := keyInfo()
		ev1, eh1 := r.espRecovery()
		ok1 := rc1 == 0 && m1 != nil && m1[2] == "1" && slot1 == "b" && rel1 == "21" && conf1 && n1 == "2" && ev1 == "4" && eh1 == sha256File(t, recK12) && strings.Contains(log1, "recovery kernel version 4 installed (was 2)")
		// K2: ... and after its confirm a bundle signed with key 2 is accepted; that release drops key 1
		rc2, o2 := r.update("v22-k2-drops-k1")
		m2 := sigRe.FindStringSubmatch(o2)
		slot2, rel2, conf2, _, log2 := r.trialBoot(func() { r.update("v22-k2-drops-k1") })
		n2, l2 := keyInfo()
		ev2, eh2 := r.espRecovery()
		ok2 := rc2 == 0 && m2 != nil && m2[2] == "2" && m1 != nil && m2[1] != m1[1] && slot2 == "a" && rel2 == "22" && conf2 && n2 == "1" && ev2 == "5" && eh2 == sha256File(t, recK2) && strings.Contains(log2, "recovery kernel version 5 installed (was 4)")
		// K3: after a release that drops key 1, a bundle signed with key 1 is refused
		rc3, o3 := r.update("v23-k1-after-drop")
		ok3 := rc3 == 2 && strings.Contains(o3, "REFUSED: bad signature") && strings.Contains(o3, "any of the 1 key(s)")
		record("K1 key rotation: key 1 is accepted; a release signed with key 1 that carries key 2 is accepted and its confirm installs a recovery kernel that knows both; after that a bundle signed with key 2 is accepted (and drops key 1 with a recovery kernel for key 2); then a bundle signed with key 1 is refused", ok0 && ok1 && ok2 && ok3, time.Since(start),
			fmt.Sprintf("keyring files %s -> %s -> %s; key-2 bundle before the rotation rc=%d; recovery kernel on the boot partition version %s -> %s; key-1 bundle after the drop rc=%d", n0, n1, n2, rc0, ev1, ev2, rc3))
		if !ok0 || !ok1 || !ok2 || !ok3 {
			t.Errorf("ok0=%v ok1=%v ok2=%v ok3=%v\n%s\n%s\n%s\n%s\nkeys: %s | %s | %s\nlog1=%q log2=%q ev1=%s ev2=%s", ok0, ok1, ok2, ok3, o0, o1, o2, o3, l0, l1, l2, log1, log2, ev1, ev2)
			t.Fail()
		}
		// K4: the recovery shell now uses the keyring of the NEW recovery kernel (key 2 only): it refuses key 1 and installs a key-2 bundle
		start4 := time.Now()
		banner, _ := r.bootRecoveryEntry()
		var rcA, rcB int
		var oA, oB string
		if banner {
			rcA, oA = r.vm.sh("hubos-ctl update "+r.base+"/v23-k1-after-drop a 2>&1", 200*time.Second)
			rcB, oB = r.vm.sh("hubos-ctl update "+r.base+"/v24-k2 b 2>&1 | tail -n 6", 400*time.Second)
			m := r.vm.mark()
			io.WriteString(r.vm.in, "sync; reboot -f\n")
			r.afterReset(m)
			r.hungReboot = false
			r.waitConfirmed()
		}
		slot4, rel4, conf4, _ := r.status()
		ok4 := banner && rcA == 2 && strings.Contains(oA, "REFUSED: bad signature") && rcB == 0 && strings.Contains(oB, "installed version 24 in slot b") && slot4 == "b" && rel4 == "24" && conf4
		record("K2 the recovery shell uses the keyring of the recovery kernel on the boot partition (key 2 only after the rotation): it refuses the key-1 bundle and installs a key-2 bundle into slot b, which boots and confirms", ok4, time.Since(start4),
			fmt.Sprintf("key-1 bundle rc=%d, key-2 bundle rc=%d; afterwards slot %s release %s confirmed %v", rcA, rcB, slot4, rel4, conf4))
		if !ok4 {
			t.Errorf("banner=%v rcA=%d rcB=%d slot=%s rel=%s\n%s\n%s", banner, rcA, rcB, slot4, rel4, oA, oB)
			t.Fail()
		}
	})

	// ---- the recovery kernel install rule at confirm: install when the file differs AND the version is not lower ----
	t.Run("T17_recovery_install_rule", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		// state after T16: release 24 (key 2 only) is confirmed; the boot partition has recovery version 5, the file recK2
		v0, h0 := r.espRecovery()
		ok0 := v0 == "5" && h0 == sha256File(t, recK2)
		// 1. the same version (5) but a different file: a key-rotation release that forgot to bump the version. It installs.
		rc1, u1 := r.update("v25-same-version-new-file")
		slot1, rel1, conf1, _, log1 := r.trialBoot(func() { r.update("v25-same-version-new-file") })
		v1, h1 := r.espRecovery()
		ok1 := rc1 == 0 && strings.Contains(u1, "installed at the confirm step, not now") && slot1 == "a" && rel1 == "25" && conf1 &&
			strings.Contains(log1, "recovery kernel version 5 installed (was 5)") && v1 == "5" && h1 == recK2mHash && h1 != h0
		// 2. a LOWER version (4) with a different file never installs
		rc2, _ := r.update("v26-lower-version")
		slot2, rel2, conf2, _, log2 := r.trialBoot(func() { r.update("v26-lower-version") })
		v2, h2 := r.espRecovery()
		ok2 := rc2 == 0 && slot2 == "b" && rel2 == "26" && conf2 &&
			strings.Contains(log2, "version 4 is lower than the installed 5; never a downgrade, left alone") && v2 == "5" && h2 == recK2mHash
		// 3. the same file and version again: left alone
		rc3, _ := r.update("v27-same-file")
		slot3, rel3, conf3, _, log3 := r.trialBoot(func() { r.update("v27-same-file") })
		v3, h3 := r.espRecovery()
		ok3 := rc3 == 0 && slot3 == "a" && rel3 == "27" && conf3 &&
			strings.Contains(log3, "version 5 is the same file as the installed one; left alone") && v3 == "5" && h3 == recK2mHash
		ok := ok0 && ok1 && ok2 && ok3
		record("C5 the recovery kernel install rule at confirm: a changed file at the same version installs (a rotation release that forgot to bump it); a lower version never installs; the same file is left alone", ok, time.Since(start),
			fmt.Sprintf("boot partition: version %s file %s -> %s %s (same version, new file) -> %s %s (lower version 4: unchanged) -> %s %s (same file: unchanged)", v0, h0[:12], v1, h1[:12], v2, h2[:12], v3, h3[:12]))
		if !ok {
			t.Errorf("ok0=%v ok1=%v ok2=%v ok3=%v\nlog1=%q\nlog2=%q\nlog3=%q", ok0, ok1, ok2, ok3, log1, log2, log3)
			t.Fail()
		}
	})

	// ---- the recovery agent inside a TEST recovery kernel ----
	t.Run("T18_recovery_agent_in_a_test_recovery_kernel", func(t *testing.T) {
		r.t = t
		if r.dead {
			t.Skip("run stopped by a repeated hang")
		}
		start := time.Now()
		// put the TEST recovery kernel (the key-2 recovery kernel plus the agent) on the boot partition, then boot recovery
		normalSize := fileSize(t, recK2)
		agentSize := fileSize(t, recAgent)
		_, put := r.sh(`mount -t vfat $(findfs PARTLABEL=hubos-esp) /boot/efi && wget -q -O /boot/efi/EFI/hubos/kernel-recovery.efi.new ` + r.base + `/agent-kernel/kernel-recovery.efi && mv /boot/efi/EFI/hubos/kernel-recovery.efi.new /boot/efi/EFI/hubos/kernel-recovery.efi && sync; umount /boot/efi; echo put-done`)
		banner, shellOut := r.bootRecoveryEntry()
		ac := newAgentClient(t, r)
		// 1. GET /v1/status from the host answers "recovery"
		var code int
		var body string
		for i := 0; i < 20; i++ { // the agent starts after DHCP; the forward needs a moment
			if code, body = ac.do("GET", "/v1/status", "", nil); code == 200 {
				break
			}
			time.Sleep(2 * time.Second)
		}
		var st struct {
			State        string
			Release      string `json:"recovery_release"`
			Machine      string
			BootFailures int `json:"boot_failures"`
			FailureLimit int `json:"failure_limit"`
		}
		json.Unmarshal([]byte(body), &st)
		okStatus := banner && strings.Contains(put, "put-done") && code == 200 && st.State == "recovery" && st.Release == "recovery-1" && st.FailureLimit == 3
		// 2. refused requests: unsigned, a signature of another key, a body changed after signing, a replayed request
		reqBody := []byte(`{"Slot":"b","BaseURL":"` + r.base + `/v28-agent"}`)
		c1, b1 := ac.do("POST", "/v1/install", "", reqBody)
		c2, b2 := ac.signed(otherMgmtSec, "POST", "/v1/install", reqBody)
		n3 := ac.nonce()
		auth3 := ac.sign(mgmtSec, "POST", "/v1/install", n3, []byte(`{"Slot":"a","BaseURL":"`+r.base+`/v28-agent"}`))
		c3, b3 := ac.do("POST", "/v1/install", auth3, reqBody) // signed for slot a, sent for slot b
		n4 := ac.nonce()
		auth4 := ac.sign(mgmtSec, "POST", "/v1/clear-failures", n4, nil)
		c4, b4 := ac.do("POST", "/v1/clear-failures", auth4, nil) // a good signed request: accepted ...
		c5, b5 := ac.do("POST", "/v1/clear-failures", auth4, nil) // ... and the same bytes again: replayed
		okRefused := c1 == 401 && c2 == 401 && c3 == 401 && c4 == 200 && c5 == 401 && strings.Contains(b5, "used or expired nonce") && strings.Contains(b2, "signature not accepted")
		// 3. a signed install request: installs a signed bundle into slot b; the machine then boots it
		ic, ib := ac.signed(mgmtSec, "POST", "/v1/install", reqBody)
		okInstall := ic == 200 && strings.Contains(ib, "installed version 28 in slot b")
		m := r.vm.mark()
		io.WriteString(r.vm.in, "sync; reboot -f\n")
		r.afterReset(m)
		r.hungReboot = false
		confirmed := r.waitConfirmed()
		slot, rel, conf, _ := r.status()
		ev, eh := r.espRecovery()
		okBoot := confirmed && slot == "b" && rel == "28" && conf && ev == "5" && eh == sha256File(t, recK2) // the confirm step put the normal recovery kernel back (same version, other file)
		ok := okStatus && okRefused && okInstall && okBoot
		record("T18 the recovery agent inside a TEST recovery kernel: GET /v1/status from the host answers recovery; unsigned, wrong-key, tampered and replayed requests are refused; a signed install request installs a signed bundle into slot b and the machine boots and confirms it", ok, time.Since(start),
			fmt.Sprintf("status %d %s; refused: unsigned %d, other key %d, changed body %d, replay %d (good clear-failures %d); install %d; booted slot %s release %s confirmed %v; recovery kernel with the agent %d bytes, without %d bytes (+%d); normal recovery kernel back on the boot partition: %v", code, strings.TrimSpace(body), c1, c2, c3, c5, c4, ic, slot, rel, conf, agentSize, normalSize, agentSize-normalSize, eh == sha256File(t, recK2)))
		if !ok {
			t.Errorf("okStatus=%v okRefused=%v okInstall=%v okBoot=%v\nput=%q\n%s\n%d %s | %d %s | %d %s | %d %s | %d %s\ninstall %d %s", okStatus, okRefused, okInstall, okBoot, put, shellOut, c1, b1, c2, b2, c3, b3, c4, b4, c5, b5, ic, ib)
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
		kr1 := sum(filepath.Join(work, "out", "kernel-recovery.efi"))
		os.Rename(filepath.Join(work, "out", "kernel-a.efi"), filepath.Join(work, "out", "kernel-a-first.efi"))
		os.Rename(filepath.Join(work, "out", "kernel-b.efi"), filepath.Join(work, "out", "kernel-b-first.efi"))
		r.script("build-kernel.sh", []string{"FORCE=1", "UPDATE_PUB=" + r.pub})
		ka2, kb2 := sum(filepath.Join(work, "out", "kernel-a.efi")), sum(filepath.Join(work, "out", "kernel-b.efi"))
		kr2 := sum(filepath.Join(work, "out", "kernel-recovery.efi"))
		t.Logf("recovery kernel: %s / %s -> identical: %v", kr1, kr2, kr1 == kr2)
		k1, k2 := ka1+kb1+kr1, ka2+kb2+kr2
		t.Logf("kernel a: %s / %s -> identical: %v; kernel b: %s / %s -> identical: %v; a differs from b: %v", ka1, ka2, ka1 == ka2, kb1, kb2, kb1 == kb2, ka1 != kb1)
		if k1 != k2 {
			out, _ := exec.Command("cmp", filepath.Join(work, "out", "kernel-a-first.efi"), filepath.Join(work, "out", "kernel-a.efi")).CombinedOutput()
			t.Logf("kernel a differs: %s", strings.TrimSpace(string(out)))
		}
		record("9 two builds of the root give identical hashes (root tar asserted; squashfs and the three kernels reported)", first == second, time.Since(start),
			fmt.Sprintf("tar %v, squashfs %v, kernels %v", first == second, sa == sb, k1 == k2))
	})
}

// checkNoSystemd looks in the unpacked root for systemd programs and unit directories and lists the
// systemd libraries that remain.
func checkNoSystemd(t *testing.T, root string, eudev bool) bool {
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
		case !info.IsDir() && (n == "systemd" || strings.HasPrefix(n, "systemd-") || n == "systemctl" || n == "journalctl" || (!eudev && (n == "udevadm" || n == "udevd"))):
			bad = append(bad, "program "+rel)
		case info.IsDir() && (rel == "/etc/systemd" || rel == "/usr/lib/systemd" || rel == "/lib/systemd" || (!eudev && (rel == "/etc/udev" || rel == "/usr/lib/udev" || rel == "/lib/udev"))):
			bad = append(bad, "directory "+rel)
		case strings.Contains(rel, "/usr/share/dbus-1/") && strings.HasSuffix(n, ".service"):
			// D-Bus activation files (for example ca.desrt.dconf.service from the dconf package): not systemd units
			named = append(named, rel)
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
