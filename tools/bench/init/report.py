#!/usr/bin/env python3
"""report.py RUNDIR [soak RUNDIR2] : aggregate the run logs of every candidate into Markdown tables (BENCH CODE).

RUNDIR holds <candidate>-boot-N.log, -pid1crash-N.log, -pid1wedge-1.log, -trial-<scenario>.log, -faults-1.log written by
run-qemu.py. Prints tables for the document; every number comes from a log line, nothing is typed in by hand."""
import glob, os, re, statistics as st, sys, collections

CANDS = ["s6plain", "s6rc", "runit", "dinit", "openrc", "c-go", "c-elixir", "b-go"]

def kv(line):
    return {m.group(1): m.group(2).strip('"') for m in re.finditer(r'(\w+)=("[^"]*"|\S+)', line)}

def lines(path):
    try:
        return open(path, errors="replace").read().splitlines()
    except FileNotFoundError:
        return []

def hosttime(l):
    m = re.match(r"H\s*([\d.]+) ", l)
    return float(m.group(1)) if m else None

def med(v):
    return st.median(v) if v else float("nan")

def q(v, p):
    v = sorted(v)
    return v[min(len(v) - 1, int(round(p * (len(v) - 1))))] if v else float("nan")

def fmt(v, d=1):
    return "n/a" if v != v else f"{v:.{d}f}"

def table(head, rows):
    out = ["| " + " | ".join(head) + " |", "|" + "|".join("---" for _ in head) + "|"]
    for r in rows:
        out.append("| " + " | ".join(str(x) for x in r) + " |")
    return "\n".join(out)

def boot(d):
    rows = []
    for c in CANDS:
        t, pss, ticks, busy, idle = [], [], [], [], []
        for p in sorted(glob.glob(f"{d}/{c}-boot-*.log")):
            sup = None; mem = 0; got = False
            for l in lines(p):
                if "SUP-START" in l:
                    sup = float(kv(l)["up"])
                m = re.search(r"BOOT-OK up=([\d.]+) ok=1", l)
                if m and sup is not None and not got:
                    t.append(float(m.group(1)) - sup); got = True
                if " MEM pid=" in l:
                    mem += int(kv(l)["pss_kb"])
                if " IDLE-CPU " in l:
                    k = kv(l)
                    ticks.append(int(k["init_side_ticks"])); busy.append(int(k["whole_vm_busy_ticks"])); idle.append(int(k["whole_vm_idle_ticks"]))
            if mem:
                pss.append(mem)
        rows.append([c, len(t), f"{fmt(med(t))} ({fmt(min(t) if t else float('nan'))}-{fmt(max(t) if t else float('nan'))})", fmt(med(pss), 0), fmt(med(ticks), 0), f"{fmt(med(busy), 0)} / {fmt(med(idle), 0)}"])
    return table(["candidate", "boots", "supervisor start to desktop, s: median (min-max)", "PSS of PID 1 + supervisors + policy layer, kB", "init-side CPU ticks in 30 s idle", "whole VM busy / idle ticks in 30 s (2 vCPU)"], rows)

def pid1(d):
    rows = []
    for c in CANDS:
        cr, wd = [], []
        for p in sorted(glob.glob(f"{d}/{c}-pid1crash-*.log")):
            L = lines(p); inj = pan = ok2 = None; n = 0
            for l in L:
                if "INJECT pid1-segv" in l: inj = hosttime(l)
                if "Attempted to kill init" in l and pan is None: pan = hosttime(l)
                if "BOOT-OK" in l:
                    n += 1
                    if n == 2: ok2 = hosttime(l)
            if pan and ok2: cr.append((ok2 - pan, pan - inj if inj else float("nan")))
        for p in sorted(glob.glob(f"{d}/{c}-pid1wedge-*.log")):
            L = lines(p); inj = ok2 = None; n = 0; first_fail = None
            for l in L:
                if "INJECT pid1-wedge" in l: inj = hosttime(l)
                if "guard: PID 1 check failed (1)" in l and first_fail is None: first_fail = hosttime(l)
                if "BOOT-OK" in l:
                    n += 1
                    if n == 2: ok2 = hosttime(l)
            if inj and ok2: wd.append((ok2 - inj, (first_fail - inj) if first_fail else float("nan")))
        rows.append([c, ", ".join(f"{a:.0f}" for a, b in cr) or "n/a", ", ".join(f"{a:.0f} (guard noticed after {b:.0f})" for a, b in wd) or "n/a"])
    return table(["candidate", "PID 1 killed: panic to desktop again, s", "PID 1 wedged: injection to desktop again, s (watchdog 30 s)"], rows)

def trial(d):
    rows = []
    for c in CANDS:
        r = [c]
        for s in ("healthy", "crash", "hang"):
            res = "n/a"
            for l in lines(f"{d}/{c}-trial-{s}.log"):
                if " TRIAL CONFIRMED" in l:
                    k = kv(l); res = f"CONFIRMED after {k['after'].rstrip('s')} s" + (" (FALSE: " + k["true_state"] + ")" if k.get("true_state") else "")
                elif " TRIAL ROLLBACK" in l:
                    k = kv(l); res = "ROLLBACK reboot at 60 s" + (", guard kept feeding" if k.get("guard_still_feeding") == "true" else ", GUARD GONE")
            r.append(res)
        rows.append(r)
    return table(["candidate", "trial: healthy", "trial: driftwm cannot start", "trial: driftwm wedged 2 s after it first worked"], rows)

def faults(d):
    rows = []
    allk = collections.OrderedDict()
    data = {}
    for c in CANDS:
        ev = collections.defaultdict(list); unrec = collections.Counter(); extra = collections.defaultdict(list)
        for l in lines(f"{d}/{c}-faults-1.log"):
            if " EV ev=" in l:
                k = kv(l.split(" EV ", 1)[1]); f = k["fault"]
                key = "kill (any of 6)" if f == "kill" else f
                ev[key].append((float(k["rec"]), k["ok"] == "1", k))
            elif "UNRECOVERED" in l:
                k = kv(l.split("UNRECOVERED", 1)[1]); unrec[(k["fault"], k.get("alert", "0"))] += 1
            elif " STORM " in l:
                extra["storm"].append(kv(l))
            elif " DEPSTARTS " in l:
                extra["dep"].append(kv(l))
            elif " OOMSCORE " in l:
                extra["oom"].append(l)
        data[c] = (ev, unrec, extra)
    keys = ["kill (any of 6)", "hang-stop", "hang-spin", "dep-slow", "dep-fail", "disk-full", "supervisor-killed", "scanner-killed", "brain-killed", "clock-3600", "clock+86400", "oom"]
    for k in keys:
        r = [k]
        for c in CANDS:
            ev = data[c][0].get(k, [])
            if not ev:
                r.append("n/a"); continue
            ok = [x[0] for x in ev if x[1]]
            bad = len(ev) - len(ok)
            s = f"{fmt(med(ok))} (max {fmt(max(ok) if ok else float('nan'))})" if ok else "-"
            if bad: s += f"; {bad}/{len(ev)} needed a person"
            r.append(s)
        rows.append(r)
    t1 = table(["fault: median recovery s (worst)"] + CANDS, rows)
    rows = []
    for tgt in ("hubd", "driftwm"):
        r = [f"crash storm on {tgt}, 40 s"]
        for c in CANDS:
            s = [x for x in data[c][2]["storm"] if x.get("target") == tgt]
            if s:
                x = s[0]; r.append(f"{x['starts']} starts; {x['starts_last10s']} in last 10 s; alert={'yes' if any('ALERT' in y for y in lines(f'{d}/{c}-faults-1.log')) else 'no'}")
            else:
                r.append("n/a")
        rows.append(r)
    for mode in ("slow", "fail"):
        r = [f"seatd {mode}: starts of driftwm / waybar / hubd while waiting"]
        for c in CANDS:
            s = [x for x in data[c][2]["dep"] if x.get("mode") == mode]
            r.append(f"{s[0]['driftwm']} / {s[0]['waybar']} / {s[0]['hubd']}" if s else "n/a")
        rows.append(r)
    t2 = table(["measure"] + CANDS, rows)
    rows = []
    for k in sorted({k for c in CANDS for k in data[c][1]}):
        r = [f"{k[0]} (alert={k[1]})"]
        for c in CANDS:
            r.append(data[c][1].get(k, 0))
        rows.append(r)
    t3 = table(["unrecovered by itself, count"] + CANDS, rows)
    return t1 + "\n\n" + t2 + "\n\n" + t3

def soak(d):
    rows = []
    det = []
    for c in CANDS:
        p = f"{d}/{c}-soak-1.log"
        L = lines(p)
        if not L:
            continue
        ev = collections.defaultdict(list); alln = []; unrec = []; stuck = 0; stage1 = 0; samples = []; wall = None
        for l in L:
            if "CH STAGE1" in l: stage1 += 1
            if " EV ev=" in l:
                k = kv(l.split(" EV ", 1)[1])
                if k["ok"] == "1":
                    ev[k["fault"]].append(float(k["rec"])); alln.append(float(k["rec"]))
            if "UNRECOVERED" in l:
                k = kv(l.split("UNRECOVERED", 1)[1]); unrec.append((k["fault"], k.get("alert", "0"), k.get("why", "")))
            if " STUCK " in l: stuck += 1
            if " SAMPLE " in l:
                k = kv(l); samples.append({a: int(b) for a, b in k.items() if a in ("zombies", "nproc", "init_side_fds", "init_side_rss_kb", "memavail_kb", "init_side_cpu_ticks")})
            m = re.search(r"H# end wall=([\d.]+)s", l)
            if m: wall = float(m.group(1))
        n_ev = sum(len(v) for v in ev.values())
        deg = sum(1 for u in unrec if u[1] == "1")
        s0, s1 = (samples[0], samples[-1]) if samples else ({}, {})
        rows.append([c, f"{(wall or 0)/3600:.2f}", n_ev + len(unrec), fmt(med(alln), 2), fmt(q(alln, .9), 2), fmt(q(alln, .99), 2), fmt(max(alln) if alln else float('nan'), 1), len(unrec), deg, stuck, max(0, stage1 - 1),
                     f"{max(s['zombies'] for s in samples) if samples else 'n/a'}", f"{s0.get('init_side_fds','?')}->{s1.get('init_side_fds','?')}", f"{s0.get('init_side_rss_kb','?')}->{s1.get('init_side_rss_kb','?')}", f"{s0.get('memavail_kb','?')}->{s1.get('memavail_kb','?')}"])
        c2 = collections.Counter((u[0], u[1], u[2][:45]) for u in unrec)
        det.append((c, c2))
    t = table(["candidate", "hours", "faults", "median s", "p90", "p99", "max (recovered alone)", "needed a person", "of which degraded+alert by design", "stuck (reboot)", "unplanned VM boots", "max zombies", "init-side fds first->last", "init-side RSS kB first->last", "MemAvailable kB first->last"], rows)
    out = [t, ""]
    for c, c2 in det:
        if c2:
            out.append(f"**{c}** unrecovered states: " + "; ".join(f"{n} x {f} (alert={a}) {w}" for (f, a, w), n in c2.most_common()))
    return "\n".join(out)

if __name__ == "__main__":
    d = sys.argv[1]
    print("### Boot, memory, idle CPU\n"); print(boot(d))
    print("\n### PID 1 killed or wedged\n"); print(pid1(d))
    print("\n### Update trial boot\n"); print(trial(d))
    print("\n### Fault injection\n"); print(faults(d))
    if len(sys.argv) > 2:
        print("\n### Chaos soak\n"); print(soak(sys.argv[2]))
