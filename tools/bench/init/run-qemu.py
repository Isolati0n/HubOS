#!/usr/bin/env python3
"""run-qemu.py CAND SUITE [--dur S] [--seed N] [--reps N] [--timeout S] [--out FILE]

Boots tools/bench/init candidate CAND (an initramfs built by mkrootfs.sh) in QEMU 8.2.2 without KVM and records the
serial console with the HOST's arrival time on every line (so a reboot caused by a panic or the watchdog can be timed).
If the fault injector reports STUCK (a state only a reboot clears) the machine is powered off by the injector and this
script starts it again (soak: with the remaining time; faults: skipping the steps already done).
BENCH CODE. Environment: W (work dir), T (unpacked tools root), REPO. One VM at a time.
"""
import argparse, os, re, select, subprocess, sys, time, signal

ap = argparse.ArgumentParser()
ap.add_argument("cand"); ap.add_argument("suite")
ap.add_argument("--dur", type=int, default=600); ap.add_argument("--seed", type=int, default=1)
ap.add_argument("--reps", type=int, default=5); ap.add_argument("--timeout", type=int, default=3600)
ap.add_argument("--out", default=None); ap.add_argument("--mem", type=int, default=1024)
ap.add_argument("--smp", type=int, default=2)
ap.add_argument("--trial", default="healthy"); ap.add_argument("--preflag", default="")
a = ap.parse_args()
W, T, REPO = os.environ["W"], os.environ["T"], os.environ.get("REPO", ".")
extra = open(f"{REPO}/tools/bench/init/candidates/{a.cand}/cmdline").read().strip()
out = a.out or f"{W}/runs/{a.cand}-{a.suite}-{a.seed}.log"
os.makedirs(os.path.dirname(out), exist_ok=True)
T0 = time.monotonic()
skip = 0
dur_left = a.dur
done = False
launches = 0
with open(out, "wb") as f:
    f.write(f"H# start {time.strftime('%F %T')} cand={a.cand} suite={a.suite} cpus_seen={os.cpu_count()} loadavg_at_start={open('/proc/loadavg').read().split()[:3]}\n".encode())
    while not done and launches < 8 and time.monotonic() - T0 < a.timeout:
        launches += 1
        cmd = ["qemu-system-x86_64", "-L", f"{T}/usr/share/qemu", "-L", f"{T}/usr/share/seabios",
               "-machine", "q35,smm=off", "-accel", "tcg,thread=multi", "-smp", str(a.smp), "-m", str(a.mem),
               "-nographic", "-nodefaults", "-no-user-config", "-display", "none", "-monitor", "none",
               "-serial", "stdio", "-device", "i6300esb", "-watchdog-action", "reset", "-nic", "none",
               "-kernel", f"{T}/boot/vmlinuz-6.8.0-146-generic", "-initrd", f"{W}/rf/{a.cand}.cpio.gz",
               "-append", f"console=ttyS0 panic=5 loglevel=4 quiet rdinit=/init hub.suite={a.suite} hub.dur={dur_left} hub.seed={a.seed + launches - 1} hub.reps={a.reps} hub.skip={skip} hub.trial={a.trial} hub.preflag={a.preflag} {extra}"]
        f.write(f"H# launch {launches} skip={skip} dur={dur_left} cmd: {' '.join(cmd)}\n".encode())
        p = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, stdin=subprocess.DEVNULL, preexec_fn=os.setsid)
        t0 = time.monotonic()
        stuck_step = None
        boot_ok = 0
        trial_seen = None
        while True:
            if time.monotonic() - T0 > a.timeout:
                f.write(b"H# TIMEOUT, killing qemu\n"); break
            r, _, _ = select.select([p.stdout], [], [], 5)
            if a.suite == "trial" and trial_seen and time.monotonic() - trial_seen > 20:
                f.write(b"H# trial finished\n"); done = True; break
            if not r:
                continue
            line = p.stdout.readline()
            if not line:
                break
            f.write(f"H{time.monotonic()-T0:9.2f} ".encode() + line.rstrip(b"\r\n") + b"\n"); f.flush()
            if b"SUITE-DONE" in line:
                done = True
            if a.suite == "trial" and b" TRIAL " in line and (b"CONFIRMED" in line or b"ROLLBACK" in line):
                trial_seen = time.monotonic()
            if a.suite == "trial" and trial_seen and (b"STAGE1" in line or time.monotonic() - trial_seen > 20):
                f.write(b"H# trial finished\n"); done = True; break
            if b"BOOT-OK" in line:
                boot_ok += 1
                if a.suite in ("pid1crash", "pid1wedge") and boot_ok >= 2:
                    # the machine came back by itself after the injected PID 1 fault: that is the measurement
                    f.write(b"H# second BOOT-OK seen: the machine recovered by itself\n")
                    done = True
                    break
            m = re.search(rb"STUCK ev=\d+ step=(\d+)", line)
            if m:
                stuck_step = int(m.group(1))
        try:
            os.killpg(p.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        p.wait()
        f.write(f"H# vm exit code {p.returncode} after {time.monotonic()-t0:.0f}s, loadavg {open('/proc/loadavg').read().split()[:3]}\n".encode())
        if stuck_step is not None:
            skip = stuck_step if a.suite == 'faults' else 0
            dur_left = max(60, a.dur - int(time.monotonic() - T0))
        elif not done:
            break
    f.write(f"H# end wall={time.monotonic()-T0:.1f}s launches={launches} suite_done={done}\n".encode())
print(out, "done" if done else "NOT-DONE", f"{time.monotonic()-T0:.0f}s")
