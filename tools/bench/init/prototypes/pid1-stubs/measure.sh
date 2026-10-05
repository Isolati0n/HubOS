#!/bin/bash
# measure.sh OUTDIR : build the four measurement stubs (C, Rust, Go, Zig) statically and measure them (BENCH CODE).
# Needs gcc, rustc, go and a zig binary in $ZIG (default: zig on PATH). The stubs run as ordinary processes here; the
# kernel treats PID 1 the same way except that it panics when PID 1 dies (tested separately in QEMU).
set -e
ulimit -c 0
O=${1:?outdir}; mkdir -p "$O"
H=$(cd "$(dirname "$0")" && pwd)
gcc -O2 -static -s -o "$O/stub-c" "$H/stub.c"
rustc -C opt-level=z -C panic=abort -C strip=symbols -C target-feature=+crt-static -C lto=fat -o "$O/stub-rust-abort" "$H/stub.rs" 2>/dev/null
rustc -C opt-level=z -C panic=unwind -C strip=symbols -C target-feature=+crt-static -o "$O/stub-rust-unwind" "$H/stub.rs" 2>/dev/null
( cd "$H/go" && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$O/stub-go" . )
${ZIG:-zig} build-exe -O ReleaseSmall -target x86_64-linux-musl -fstrip -femit-bin="$O/stub-zig" "$H/stub.zig" 2>&1 | tail -5 || true
python3 - "$O" <<'EOF'
import os, subprocess, sys, time, signal, statistics as st
O = sys.argv[1]
names = ["stub-c", "stub-rust-abort", "stub-rust-unwind", "stub-go", "stub-zig"]
def rss(pid):
    d = {}
    for l in open(f"/proc/{pid}/status"):
        k, _, v = l.partition(":")
        d[k] = v.strip()
    return d
print(f"{'stub':18s} {'size bytes':>11s} {'VmRSS kB':>9s} {'threads':>7s} {'exit-mode ms (median of 300)':>30s}  panic behaviour")
for n in names:
    p = f"{O}/{n}"
    if not os.path.exists(p):
        print(f"{n:18s} (not built)"); continue
    size = os.path.getsize(p)
    pr = subprocess.Popen([p], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    time.sleep(1.0)
    s = rss(pr.pid)
    pr.send_signal(signal.SIGKILL); pr.wait()
    ts = []
    for _ in range(300):
        t = time.perf_counter(); subprocess.run([p, "exit"]); ts.append((time.perf_counter() - t) * 1000)
    pn = subprocess.run([p, "panic"], capture_output=True, text=True)
    how = f"signal {-pn.returncode}" if pn.returncode < 0 else f"exit code {pn.returncode}"
    msg = (pn.stderr.strip().splitlines() or ["(no message)"])[0][:60]
    print(f"{n:18s} {size:11d} {s['VmRSS'].split()[0]:>9s} {s['Threads']:>7s} {st.median(ts):30.2f}  {how}; first line: {msg}")
EOF
