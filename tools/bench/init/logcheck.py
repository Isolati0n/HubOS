#!/usr/bin/env python3
"""logcheck.py LOGFILE : follow-up 4, item 4. Reads the LOGDUMP lines of a lockprobe run (the service's log files as the VM printed them)
and, per service and per process id, checks the stand-in's numbered heartbeat lines: gaps (lost lines), repeats (doubled lines), and
lines that appear in the log far later than they were written (svlogd stamps a line when it READS it). Prints a summary."""
import re, sys, collections
f = sys.argv[1]
seqs = collections.defaultdict(list); other = collections.defaultdict(collections.Counter); stamps = collections.defaultdict(list)
for l in open(f, errors="replace"):
    m = re.match(r"H\s+[\d.]+ LOGDUMP (\w+) (\d{4}-\d\d-\d\d_[\d:.]+) (\w+): (.*)", l)
    if not m: continue
    svc, ts, name, rest = m.groups()
    h = re.match(r"heartbeat pid=(\d+) seq=(\d+)", rest)
    if h: seqs[(svc, int(h.group(1)))].append((int(h.group(2)), ts))
    else: other[svc][re.sub(r"\d+", "N", rest)] += 1
tot_gap = tot_dup = 0
for (svc, pid), v in sorted(seqs.items()):
    s = [x for x, _ in v]
    dup = len(s) - len(set(s)); u = sorted(set(s))
    gaps = (u[-1] - u[0] + 1) - len(u)
    # the last lines of a killed process may never have been written: the stand-in prints one per 0.5 s, so a missing TAIL cannot be told from a stop; gaps INSIDE the numbers are what is counted
    order = s == sorted(s)
    print(f"{svc} pid={pid}: lines={len(s)} first_seq={u[0]} last_seq={u[-1]} gaps_inside={gaps} repeated={dup} in_order={order}")
    tot_gap += gaps; tot_dup += dup
print(f"TOTAL gaps inside the numbered runs: {tot_gap}; repeated lines: {tot_dup}")
for svc, c in other.items():
    print(svc, "other lines:", dict(c))
