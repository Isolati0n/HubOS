#!/usr/bin/env python3
import csv, sys, statistics
p = sys.argv[1]
rows = list(csv.DictReader(open(p)))
pids = sorted(set(r['pid'] for r in rows))
print('samples', len(rows), 'duration_s', rows[-1]['t'], 'pids', pids, 'restarts', rows[-1]['restarts'])
def col(n): return [float(r[n]) for r in rows]
t = col('t'); rss = col('rss_kb'); hwm = col('hwm_kb'); fds = col('fds'); thr = col('threads'); ipc = col('ipc_ms'); win = col('windows'); sta = col('standins'); cpu = col('cpu_ticks')
def slope(x, y):
    n = len(x); mx = sum(x) / n; my = sum(y) / n
    return sum((a - mx) * (b - my) for a, b in zip(x, y)) / sum((a - mx) ** 2 for a in x)
def seg(a, b):
    idx = [i for i in range(len(t)) if a <= t[i] <= b]
    return idx
print('RSS MB: min %.0f max %.0f last %.0f; VmHWM last %.0f' % (min(rss) / 1024, max(rss) / 1024, rss[-1] / 1024, hwm[-1] / 1024))
for (a, b) in ((0, 600), (600, 1800), (1800, 3600), (3600, 7200), (7200, 14400), (14400, 99999)):
    idx = seg(a, b)
    if len(idx) > 3:
        print('  t=%5d-%5d s: RSS mean %.1f MB, max %.1f MB; fds mean %.1f max %d; threads %d-%d; ipc ms median %.1f p95 %.1f max %.1f; windows mean %.1f' % (
            a, min(b, t[-1]), statistics.mean(rss[i] for i in idx) / 1024, max(rss[i] for i in idx) / 1024, statistics.mean(fds[i] for i in idx), max(fds[i] for i in idx),
            min(thr[i] for i in idx), max(thr[i] for i in idx), statistics.median(ipc[i] for i in idx), sorted(ipc[i] for i in idx)[int(.95 * len(idx)) - 1], max(ipc[i] for i in idx), statistics.mean(win[i] for i in idx)))
# growth: linear fit of RSS after the first 30 min
idx = [i for i in range(len(t)) if t[i] > 1800]
if len(idx) > 10:
    s = slope([t[i] for i in idx], [rss[i] for i in idx]) * 3600 / 1024
    print('linear RSS trend after 30 min: %.2f MB per hour (noise from window churn is +/- 10 MB)' % s)
    s2 = slope([t[i] for i in idx], [fds[i] for i in idx]) * 3600
    print('linear fd trend after 30 min: %.2f fds per hour' % s2)
print('IPC probe failures (ipc_ms<0):', sum(1 for x in ipc if x < 0))
print('CPU of driftwm: %.1f%% of one core on average' % (100 * (cpu[-1] - cpu[0]) / 100 / (t[-1] - t[0])))
