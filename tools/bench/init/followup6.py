#!/usr/bin/env python3
"""followup6.py : Markdown tables of follow-up 6 (Parts B, C, D), every number parsed from the logs in results/step2/ (BENCH CODE)."""
import glob, os, re, statistics as st

R = os.path.join(os.path.dirname(os.path.abspath(__file__)), "results", "step2")

def L(path):
    return open(path, errors="replace").read().splitlines()

def first(lines, pat):
    for l in lines:
        m = re.search(pat, l)
        if m:
            return m
    return None

def table(head, rows):
    out = ["| " + " | ".join(head) + " |", "|" + "|".join("---" for _ in head) + "|"]
    out += ["| " + " | ".join(str(x) for x in r) + " |" for r in rows]
    return "\n".join(out)

print("## Part B: the latch clears itself (s6rc-par-wd4)\n")
rows = []
for f in sorted(glob.glob(f"{R}/s6rc-par-wd4/s6rc-par-wd4-clear-*.log")):
    ls = L(f)
    name = os.path.basename(f)
    case = re.search(r"clear-(\w+?)-(\d+)\.log", name)
    set_ = "3600 s (default)" if "full" in name else ("120 s" if "-iv-" not in name else "120 s")
    clr = first(ls, r"LATCH CLEARED ([\d.]+)s after SIGCONT")
    fb = None
    if not clr:  # the harness line was lost from the console: host time of the watcher's own clear line minus host time of the resume
        a = first(ls, r"^H\s*([\d.]+) .*SIGCONT sent"); b = first(ls, r"^H\s*([\d.]+) watcher: LATCH CLEARED")
        if a and b:
            fb = float(b.group(1)) - float(a.group(1))
    sg = first(ls, r"state file gone after ([\d.]+)s")
    note = ""
    if "-ii-" in name:
        a = first(ls, r"slow-but-alive started at ([\d.]+)s"); b = first(ls, r"slow-but-alive ended at ([\d.]+)s")
        note = f"sync late from {a.group(1)} s to {b.group(1)} s after the resume" if a and b else ""
    if "-iii-" in name:
        a = first(ls, r"restart at ([\d.]+)s"); note = f"watcher killed at 60 s, restarted by the init at {a.group(1)} s" if a else ""
    if "-i-" in name:
        a = first(ls, r"HANG n=5 .*?fault_to_pidchange=([\d.]+)"); note = f"later hang (budget had room) killed {a.group(1)} s after the fault" if a else ""
    if "-iv-" in name:
        a = first(ls, r"after 300 s frozen: latch present=(\w+)"); note = f"after 300 s frozen: latch present={a.group(1)}" if a else ""
    rows.append((name.replace("s6rc-par-wd4-", ""), set_, sg.group(1) + " s" if sg else "-", (clr.group(1) + " s") if clr else (f"{fb:.1f} s (host clock; the harness line is missing from the console log)" if fb else "not cleared"), note))
print(table(["log", "setting", "state file gone after resume", "latch cleared after resume", "note"], rows))

print("\n## Part C: restart-limit alert (s6rc-par-wd4a)\n")
rows = []
for f in sorted(glob.glob(f"{R}/s6rc-par-wd4a/*alert-storm-*.log")):
    ls = L(f)
    s = re.search(r"storm-(\w+)-(\d+)", f)
    al = first(ls, r"STORM-C (\w+) ALERT after ([\d.]+)s: starts since the kill=(\d+); text=\"([^\"]*)\"")
    kept = first(ls, r"others kept their process: (\S+?);")
    hr = first(ls, r"hub-retry output=.*?desktop ok=(\w+) after ([\d.]+)s")
    two = first(ls, r"after the second round of hub-retry: desktop ok=(\w+) after ([\d.]+)s")
    cpu = first(ls, r"watcher cpu ticks used (\d+) .*? in ([\d.]+) s; state file=\"([^\"]*)\" latch=(\w+)")
    rows.append((os.path.basename(f).replace("s6rc-par-wd4a-", ""), al.group(3) if al else "-", al.group(2) + " s" if al else "-", al.group(4) if al else "-",
                 kept.group(1) if kept else "-", (f"{hr.group(1)} after {hr.group(2)} s" if hr else "-") + ("; 2nd retry: " + two.group(1) + f" after {two.group(2)} s" if two else ""),
                 f"{cpu.group(1)} ticks in {cpu.group(2)} s; state file={cpu.group(3)!r}; latch={cpu.group(4)}" if cpu else "-"))
print(table(["log", "starts after the kill, before the stop", "time to the stop", "alert text", "other services kept their process (true/false)", "hub-retry", "watcher while stopped"], rows))
print()
for f in sorted(glob.glob(f"{R}/s6rc-par-wd4a/*alert-mustnot*.log")):
    ls = L(f)
    print("**" + os.path.basename(f) + "**: " + "; ".join(sorted({re.sub(r'^H *[\d.]+ CH ', '', l)[:230] for l in ls if re.search(r"MUSTNOT (part . result|spread \d|driftwm 4-deaths)", l)})) + "\n")

print("\n## Part D: strict trial-boot confirm (s6rc-par-confirm)\n")
rows = []
for f in sorted(glob.glob(f"{R}/s6rc-par-confirm/*trial-*.log"), key=lambda p: (re.search(r"trial-(\w+)-", p).group(1), p)):
    ls = L(f)
    scn = re.search(r"trial-(\w+)-(\d+)", f)
    c = first(ls, r"TRIAL CONFIRMED scenario=(\w+) after=([\d.]+)s plain_condition_met_at=([\d.]+)s strict_added=([\d.]+)s")
    r = first(ls, r"TRIAL ROLLBACK scenario=(\w+) after=60s .*?true_state=\"([^\"]*)\".*?probe_rounds=(\d+)")
    inj = first(ls, r"TRIAL-INJECT .*? ([\d.]+)s after the desktop first worked")
    aft = first(ls, r"TRIAL-AFTER-CONFIRM .*true state is \"([^\"]*)\"")
    if c:
        res = f"CONFIRMED at {c.group(2)} s (plain condition at {c.group(3)} s, strict added {c.group(4)} s)"
    elif r:
        res = f"ROLLED BACK (60 s); true state {r.group(2)!r}; probe rounds run {r.group(3)}"
    else:
        res = "no verdict line"
    rows.append((scn.group(1), scn.group(2), res, f"wedge injected {inj.group(1)} s after first working" if inj else "-", (f"true state 3 s after a late wedge: {aft.group(1)!r}" if aft else "")))
print(table(["scenario", "seed", "verdict", "injection", "after the verdict"], rows))
