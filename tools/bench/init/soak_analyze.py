#!/usr/bin/env python3 -I
"""soak_analyze.py LOGFILE [LOGFILE...]  : memory slope of the init side during a soak.
Reads SOAKMEM / MEM lines (PSS from /proc/PID/smaps_rollup of PID 1, scanner, every supervisor, policy layer, every 60 s).
For the total and for each process name: least-squares slope of PSS against the number of service restarts, in kB per 1000
restarts, plus the noise (standard deviation of the points around the line) and the slope's standard error.
Only samples between two reboots are joined (a reboot resets memory; a time that goes backwards counts as a reboot); the slopes of the launches are averaged weighted by
their number of restarts."""
import re, sys, math, collections
WARMUP = 600   # seconds of each launch left out: the first minutes are start-up and heap growth

def fit(xs, ys):
    n = len(xs)
    if n < 3: return None
    mx, my = sum(xs)/n, sum(ys)/n
    sxx = sum((x-mx)**2 for x in xs)
    if sxx == 0: return None
    b = sum((x-mx)*(y-my) for x, y in zip(xs, ys))/sxx
    a = my - b*mx
    res = [y-(a+b*x) for x, y in zip(xs, ys)]
    s = math.sqrt(sum(r*r for r in res)/(n-2))
    return b, s, s/math.sqrt(sxx), n

for path in sys.argv[1:]:
    launches = []; cur = None; sample = None
    for line in open(path, errors="replace"):
        if line.startswith("H# launch"):
            cur = []; launches.append(cur); continue
        m = re.search(r"SOAKMEM t=(\d+) restarts=(\d+) faults=(\d+)", line)
        if m and cur is not None:
            if cur and int(m[1]) < cur[-1]["t"]:   # time went back: the machine rebooted by itself inside one launch
                cur = []; launches.append(cur)
            sample = {"t": int(m[1]), "restarts": int(m[2]), "pss": collections.defaultdict(int)}; cur.append(sample); continue
        m = re.search(r" MEM pid=\d+ comm=(\S+) rss_kb=\d+ pss_kb=(\d+)", line)
        if m and sample is not None:
            sample["pss"][m[1]] += int(m[2])
    launches = [[x for x in l if x["t"] >= WARMUP] for l in launches]
    print("== %s: %d launch(es), first %d s of each left out" % (path, len(launches), WARMUP))
    names = sorted({k for l in launches for s in l for k in s["pss"]})
    for key in ["TOTAL"] + names:
        tot_w = 0; acc = 0.0; noise = []; se = []
        for l in launches:
            xs = [s["restarts"] for s in l]
            ys = [sum(s["pss"].values()) if key == "TOTAL" else s["pss"].get(key, 0) for s in l]
            f = fit(xs, ys)
            if not f: continue
            b, s_, e, n = f
            w = max(xs) - min(xs)
            acc += b*w; tot_w += w; noise.append(s_); se.append(e)
        if tot_w:
            last = [sum(s["pss"].values()) if key == "TOTAL" else s["pss"].get(key, 0) for l in launches for s in l]
            print("%-14s slope %+8.2f kB per 1000 restarts (std.err %6.2f)  noise sd %6.1f kB  range %d..%d kB  samples %d  restarts covered %d" % (
                key, 1000*acc/tot_w, 1000*max(se), sum(noise)/len(noise), min(last), max(last), len(last), tot_w))
