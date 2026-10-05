#!/usr/bin/env python3
"""analyze.py LOG... : summarise run logs written by run-qemu.py (BENCH CODE).

Prints, per log: the boots, the recovery time per fault kind (n, ok, median, p90, max), every UNRECOVERED state and
whether it was a deliberate 'degraded + alert' stop, the drift of zombies / init-side file descriptors / free memory,
and the number of virtual-machine reboots that nobody asked for."""
import re, sys, statistics as st, collections

def q(v, p):
    v = sorted(v)
    return v[min(len(v) - 1, int(round(p * (len(v) - 1))))] if v else float("nan")

def kv(line):
    return {m.group(1): m.group(2).strip('"') for m in re.finditer(r'(\w+)=("[^"]*"|\S+)', line)}

def analyze(path):
    ev = collections.defaultdict(list)
    unrec, stuck, samples, boots = [], [], [], []
    sup_start = None
    extra = collections.defaultdict(list)
    alerts = 0
    idle = []
    mem = collections.Counter()
    reboots = 0
    for line in open(path, errors="replace"):
        line = line.rstrip("\n")
        if "CH STAGE1" in line:
            reboots += 1
        if "SUP-START" in line:
            sup_start = float(kv(line)["up"])
        m = re.search(r"BOOT-OK up=([\d.]+) ok=(\d)", line)
        if m:
            boots.append((float(m.group(1)), sup_start, int(m.group(2))))
        if " EV ev=" in line:
            d = kv(line.split(" EV ", 1)[1])
            ev[d["fault"] + ":" + d["target"] if d["fault"] in ("kill", "storm") else d["fault"]].append((float(d["rec"]), d["ok"] == "1", d["fixed"] == "1", d))
        elif "UNRECOVERED" in line:
            d = kv(line.split("UNRECOVERED", 1)[1])
            unrec.append(d)
        elif " STUCK " in line:
            stuck.append(kv(line.split("STUCK", 1)[1]))
        elif " SAMPLE " in line:
            d = kv(line)
            samples.append({k: int(v) for k, v in d.items() if k in ("zombies", "nproc", "init_side_fds", "init_side_rss_kb", "memavail_kb", "init_side_cpu_ticks")})
        elif " IDLE-CPU " in line:
            idle.append(line.split("CH ", 1)[-1])
        elif " MEM pid=" in line:
            d = kv(line)
            mem[d["comm"]] += int(d["pss_kb"])
        elif any(k in line for k in (" STORM ", " DEPSTARTS ", " OOMSCORE ", "ALERT-FILE")):
            extra[line.split()[2] if len(line.split()) > 2 else "?"].append(line.split("CH ", 1)[-1])
    print(f"== {path}")
    for b in boots[:1]:
        print(f"  first boot: BOOT-OK at uptime {b[0]:.1f}s, supervisor started at {b[1]}s -> {b[0]-(b[1] or 0):.1f}s from supervisor start to desktop")
    for l in idle[:2]:
        print("  " + l[:220])
    if mem:
        print("  PSS kB by program (PID 1, supervisors, policy layer): " + ", ".join(f"{k}={v}" for k, v in mem.most_common()) + f"  total={sum(mem.values())}")
    print(f"  VM boots in log: {reboots}  stuck states needing a reboot: {len(stuck)}")
    print("  fault                          n  recovered  median    p90    max   (seconds, includes the 60 s wait when not recovered alone)")
    for k in sorted(ev):
        v = ev[k]
        rec = [r for r, ok, fx, d in v if ok]
        n_ok = len(rec)
        print(f"  {k:28s} {len(v):3d}  {n_ok:3d}/{len(v):<3d}   {st.median(rec) if rec else float('nan'):6.2f} {q(rec, .9):6.2f} {max(rec) if rec else float('nan'):6.2f}")
    if unrec:
        c = collections.Counter((u["fault"], u["target"], u.get("alert", "0"), u["why"][:40]) for u in unrec)
        print("  UNRECOVERED states (needed a person):")
        for (f, t, a, w), n in sorted(c.items()):
            print(f"    {n:3d} x {f} {t} alert_raised={a} why={w}")
    if samples:
        a, b = samples[0], samples[-1]
        print(f"  drift over {len(samples)} samples: zombies max {max(s['zombies'] for s in samples)}; nproc {a['nproc']}->{b['nproc']} (max {max(s['nproc'] for s in samples)}); init-side fds {a['init_side_fds']}->{b['init_side_fds']} (max {max(s['init_side_fds'] for s in samples)}); init-side RSS kB {a['init_side_rss_kb']}->{b['init_side_rss_kb']}; MemAvailable kB {a['memavail_kb']}->{b['memavail_kb']} (min {min(s['memavail_kb'] for s in samples)})")
    for k in ("STORM", "DEPSTARTS", "OOMSCORE"):
        for l in extra.get(k, [])[:6]:
            print("  " + l[:200])
    return ev

if __name__ == "__main__":
    for p in sys.argv[1:]:
        analyze(p)
