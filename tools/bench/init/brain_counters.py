#!/usr/bin/env python3 -I
"""brain_counters.py LOG [LOG...] : slope with a 95 % interval of each counter the instrumented Go brain reports (BRAINSTAT lines:
Go runtime numbers read right after a forced garbage collection that also returns memory to the OS) and of the brain's own PSS
(smaps_rollup, read right after, MEM lines with comm=brain), against the number of service restarts. Same method as soak_ci.py:
stretches between reboots, first 600 s of each left out, ordinary least squares, standard error widened for the correlation of
neighbouring samples, stretches combined by inverse variance, 95 % interval from a t table."""
import re, sys, math
WARMUP = 600
T975 = {1:12.71,2:4.30,3:3.18,4:2.78,5:2.57,6:2.45,7:2.36,8:2.31,9:2.26,10:2.23,15:2.13,20:2.09,30:2.04,40:2.02,60:2.00,120:1.98}
def tcrit(df):
    df = max(1, int(df))
    for k in sorted(T975):
        if df <= k: return T975[k]
    return 1.96
COUNTERS = ["pss_kb", "heap_inuse", "heap_alloc", "heap_sys", "heap_released", "goroutines", "fds"]
def segments(path):
    segs = []; cur = None; last_t = None; s = None
    for l in open(path, errors="replace"):
        if l.startswith("H# launch"): cur = []; segs.append(cur); last_t = None; continue
        m = re.search(r"SOAKMEM t=(\d+) restarts=(\d+)", l)
        if m and cur is not None:
            t = int(m[1])
            if last_t is not None and t < last_t: cur = []; segs.append(cur)
            last_t = t; s = {"t": t, "r": int(m[2])}; cur.append(s); continue
        if s is None: continue
        if "BRAINSTAT gcstats" in l:
            for k, v in re.findall(r"(\w+)=(\d+)", l.split("gcstats", 1)[1]): s[k] = int(v)
        m = re.search(r" MEM pid=\d+ comm=brain rss_kb=\d+ pss_kb=(\d+)", l)
        if m: s["pss_kb"] = int(m[1])
    return [[x for x in sg if x["t"] >= WARMUP and "heap_inuse" in x and "pss_kb" in x] for sg in segs]
def fit(xs, ys):
    n = len(xs)
    if n < 8 or max(xs) == min(xs): return None
    mx, my = sum(xs)/n, sum(ys)/n
    sxx = sum((x-mx)**2 for x in xs)
    b = sum((x-mx)*(y-my) for x, y in zip(xs, ys))/sxx
    a = my - b*mx
    res = [y-(a+b*x) for x, y in zip(xs, ys)]
    s2 = sum(r*r for r in res)/(n-2)
    den = sum(r*r for r in res)
    r1 = sum(res[i]*res[i+1] for i in range(n-1))/den if den > 0 else 0.0
    neff = n if r1 <= 0 else max(3.0, n*(1-r1)/(1+r1))
    return b, math.sqrt(s2/sxx)*math.sqrt(n/neff), neff, math.sqrt(s2), r1
segs = [sg for p in sys.argv[1:] for sg in segments(p) if len(sg) >= 8]
print("| counter | samples | restarts counted | value at start, end (mean of first and last 5) | slope per 1000 restarts | 95 % interval | noise (sd of residuals) |")
print("|---|---|---|---|---|---|---|")
for c in COUNTERS:
    aw = awb = 0.0; dfs = []; st = []; en = []; nz = []; nr = 0; ns = 0
    for sg in segs:
        f = fit([x["r"] for x in sg], [x[c] for x in sg])
        if not f: continue
        b, se, neff, sd, r1 = f
        w = 1/(se*se) if se > 0 else 1e12
        aw += w; awb += w*b; dfs.append(neff-2); nz.append(sd)
        st.append(sum(x[c] for x in sg[:5])/5); en.append(sum(x[c] for x in sg[-5:])/5)
        nr += sg[-1]["r"] - sg[0]["r"]; ns += len(sg)
    if not aw: print("| %s | none | | | | | |" % c); continue
    b = awb/aw; se = math.sqrt(1/aw); t = tcrit(sum(dfs))
    unit = "kB" if c == "pss_kb" else ("bytes" if c.startswith("heap") else "")
    k = 1000
    print("| %s | %d | %d | %.0f, %.0f %s | %+.1f %s | [%+.1f, %+.1f] | %.1f %s |" % (c, ns, nr, sum(st)/len(st), sum(en)/len(en), unit, k*b, unit, k*(b-t*se), k*(b+t*se), sum(nz)/len(nz), unit))
