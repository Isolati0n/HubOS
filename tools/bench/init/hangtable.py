#!/usr/bin/env python3
"""hangtable.py LOG... : follow-up 4, item 3. Prints one row per HANG line (fault to kill, fault to desktop back, windows back,
needed a person), the storm lines with the watcher's kill lines in the same window, and the out-of-memory lines. Data only."""
import re, sys
for f in sys.argv[1:]:
    print(f"### {f}")
    lines = open(f, errors="replace").read().splitlines()
    print("| n | shape | arm delay s | fault to kill s | fault to desktop s | windows back | needed a person | watcher state file at the end |\n|---|---|---|---|---|---|---|---|")
    for l in lines:
        m = re.search(r"HANG n=(\d+) mode=(\w+) arm_delay=([\d.]+) .*fault_to_pidchange=(-?[\d.]+) fault_to_desktop=([\d.]+) windows_back=(\d) needed_person=(\d) watcher_state=\"([^\"]*)\"", l)
        if m:
            n, mode, arm, k, d, w, p, st = m.groups()
            print(f"| {n} | {mode} | {arm} | {k if k!='-1.00' else 'no kill'} | {d} | {'yes' if w=='1' else 'NO'} | {'YES' if p=='1' else 'no'} | {st or '-'} |")
    for l in lines:
        if " STORM " in l or "fault=oom" in l or "UNRECOVERED" in l or "STUCK" in l:
            print("    " + re.sub(r"^H\s+[\d.]+ ", "", l)[:230])
