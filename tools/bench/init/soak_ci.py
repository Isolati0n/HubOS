#!/usr/bin/env python3 -I
"""soak_ci.py CAND LOG [LOG...] : memory slope of the init side with a 95 % confidence interval, from soak logs.
Input: SOAKMEM / MEM lines (PSS from smaps_rollup of PID 1, the scanner, every supervisor, the policy layer, every 60 s, and the
number of service restarts so far). Method (one line): segments between reboots (a time that goes backwards), first 600 s of each
segment left out; per segment ordinary least squares of total PSS against restarts; the standard error is widened for the
correlation between neighbouring samples (n_eff = n (1-r1)/(1+r1), r1 = lag-1 autocorrelation of the residuals, never above n);
segments are combined with weights 1/SE^2; the 95 % interval is slope +- t(0.975, n_eff-2) * SE (t from a table).
Prints per candidate: restarts counted in the analysed segments, PSS at the start and the end (mean of the first and of the last 5
samples of the analysed segments), slope in kB per 1000 restarts, its 95 % interval, the noise (sd of the residuals)."""
import re, sys, math, collections
WARMUP = 600
T975 = {1:12.71,2:4.30,3:3.18,4:2.78,5:2.57,6:2.45,7:2.36,8:2.31,9:2.26,10:2.23,15:2.13,20:2.09,30:2.04,40:2.02,60:2.00,120:1.98}
def tcrit(df):
    df = max(1, int(df))
    ks = sorted(T975)
    for k in ks:
        if df <= k: return T975[k]
    return 1.96
def segments(path):
    segs = []; cur = None; sample = None; last_t = None
    for line in open(path, errors="replace"):
        if line.startswith("H# launch"):
            cur = []; segs.append(cur); last_t = None; continue
        m = re.search(r"SOAKMEM t=(\d+) restarts=(\d+) faults=(\d+)", line)
        if m and cur is not None:
            t = int(m[1])
            if last_t is not None and t < last_t:
                cur = []; segs.append(cur)
            last_t = t
            sample = {"t": t, "r": int(m[2]), "pss": 0}; cur.append(sample); continue
        m = re.search(r" MEM pid=\d+ comm=(\S+) rss_kb=\d+ pss_kb=(\d+)", line)
        if m and sample is not None:
            sample["pss"] += int(m[2])
    return [[s for s in sg if s["t"] >= WARMUP] for sg in segs]
def fit(xs, ys):
    n = len(xs)
    if n < 8 or max(xs) == min(xs): return None
    mx, my = sum(xs)/n, sum(ys)/n
    sxx = sum((x-mx)**2 for x in xs)
    b = sum((x-mx)*(y-my) for x, y in zip(xs, ys))/sxx
    a = my - b*mx
    res = [y-(a+b*x) for x, y in zip(xs, ys)]
    s2 = sum(r*r for r in res)/(n-2)
    num = sum(res[i]*res[i+1] for i in range(n-1)); den = sum(r*r for r in res)
    r1 = num/den if den > 0 else 0.0
    neff = n if r1 <= 0 else max(3.0, n*(1-r1)/(1+r1))
    se = math.sqrt(s2/sxx) * math.sqrt(n/neff)
    return b, se, neff, math.sqrt(s2), r1
cand = sys.argv[1]
acc_w = acc_wb = 0.0; nr = 0; dfs = []; starts = []; ends = []; noise = []; nseg = 0; r1s = []
for path in sys.argv[2:]:
    for sg in segments(path):
        f = fit([s["r"] for s in sg], [s["pss"] for s in sg])
        if not f: continue
        b, se, neff, sd, r1 = f
        w = 1/(se*se) if se > 0 else 1e9
        acc_w += w; acc_wb += w*b; dfs.append(neff-2); noise.append(sd); r1s.append(r1)
        nr += sg[-1]["r"] - sg[0]["r"]; nseg += 1
        starts.append(sum(s["pss"] for s in sg[:5])/5); ends.append(sum(s["pss"] for s in sg[-5:])/5)
if not acc_w:
    print("%-9s no analysable segment (every segment shorter than the %d s warm-up or too few samples)" % (cand, WARMUP)); sys.exit()
b = acc_wb/acc_w; se = math.sqrt(1/acc_w); t = tcrit(min(dfs) if dfs else 30) if len(dfs) == 1 else tcrit(sum(dfs))
print("%-9s segments %d  restarts %5d  PSS start %6.0f end %6.0f kB  slope %+8.1f kB/1000 restarts  95%% CI [%+8.1f, %+8.1f]  noise sd %6.1f kB  lag-1 r %.2f" % (
    cand, nseg, nr, sum(starts)/len(starts), sum(ends)/len(ends), 1000*b, 1000*(b-t*se), 1000*(b+t*se), sum(noise)/len(noise), sum(r1s)/len(r1s)))
