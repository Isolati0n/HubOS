#!/usr/bin/env python3 -I
"""event_counts.py CAND=LOG[,LOG...] ... : how many events the fault script defines for a candidate, how many ran, and why not.
The script (hubsim/chaos.go faultSuite) has 43 numbered steps: 1-18 kills (3 x 6 services), 19-24 hangs (stop, spin x3), 25-26 crash
storms, 27-28 slow/failed seatd, 29-30 full disk, 31-36 supervisor kills (2 events each), 37 scanner kill (2), 38-39 brain kills,
40-41 clock jumps (2 each), 42-43 out of memory. A step that needs something the candidate does not have (a per-service supervisor,
a scanner, a brain) is counted as a step but gives no event. A step that ends in STUCK gives no EV line (the machine is rebooted
and the script goes on at the next step). A run also ends when the driver has relaunched 8 times (run-qemu.py), or at its time limit.
Usage example: event_counts.py runit=a.log,b.log,c.log   (the logs of one candidate are merged by step number)"""
import re, sys, collections
FEAT = {  # which features each candidate has (from hub.sup / hub.scan / hub.brain in its cmdline file)
 "sup":  {"s6plain","s6rc","s6rc-par","s6rc-par-wd","s6rc-par-wdc","runit","runit-111","runit-111np","runit-wd","runit-wdc","c-go","c-elixir","openrc","openrc-par","openrc-doc"},
 "scan": {"runit","runit-111","runit-111np","runit-wd","runit-wdc"},
 "watcher": {"s6rc-par-wd","s6rc-par-wdc","runit-wd","runit-wdc"},  # steps 44-45 (follow-up 3): kill the test watcher; the init must restart it
 "brain":{"b-go","c-go","c-elixir"},
}
def expect(step, c):
    if step in (44, 45): return 1 if c in FEAT["watcher"] else 0
    if step <= 30: return 1
    if 31 <= step <= 36: return 2 if c in FEAT["sup"] else 0
    if step == 37: return 2 if c in FEAT["scan"] else 0
    if step in (38, 39): return 1 if c in FEAT["brain"] else 0
    if step in (40, 41): return 2
    return 1
def analyse(c, paths):
    ev = collections.Counter(); np = collections.Counter(); stuck = {}; reached = set(); launches = 0; done = False; first_cycle = {}; cycle = 0
    # a "cycle" starts at every "CHAOS start" line; when the machine reboots itself (runit after a scanner kill) the driver starts again
    # at step 1 or at the same skip value, so the same step can appear in several cycles: its events are counted once, from the first
    # cycle in which it produced events.
    for p in paths:
        cur = None
        for l in open(p, errors="replace"):
            if l.startswith("H# launch"): launches += 1
            if " CHAOS start " in l: cycle += 1
            m = re.search(r" STEP (\d+) ", l)
            if m: cur = int(m[1]); reached.add(cur)
            if " EV ev=" in l and cur:
                if first_cycle.setdefault(cur, cycle) == cycle:
                    ev[cur] += 1
                    if " ok=0" in l: np[cur] += 1
            m = re.search(r" STUCK ev=\d+ step=(\d+) fault=(\S+)", l)
            if m: stuck[int(m[1])] = m[2]; reached.add(int(m[1]))
            if "SUITE-DONE" in l: done = True
    defined = sum(expect(s, c) for s in range(1, 46))
    ran = sum(ev.values())
    lost_stuck = sum(max(0, expect(s, c) - ev[s]) for s in stuck)
    notreached = sum(expect(s, c) for s in range(1, 46) if s not in reached)
    other = defined - ran - lost_stuck - notreached
    common = [x for x in range(1, 44) if x <= 30 or x >= 40]      # the 36 events every candidate has (not the supervisor, scanner, brain ones)
    # steps 40 and 41 give two events each, 42 and 43 one: 30 + 4 + 2 = 36
    cran = sum(ev[x] for x in common); cnp = sum(np[x] for x in common); cstuck = sum(1 for x in stuck if x in common)
    return defined, ran, lost_stuck, notreached, other, sorted(stuck.items()), max(reached) if reached else 0, launches, done, cran, cnp, cstuck
if __name__ == "__main__":
    print("| candidate | events the script defines for it | events that ran (EV lines) | lost to stuck states | never reached | other difference | stuck steps (fault) | last step reached | launches | script finished | of the 36 common events: ran / needed a person / stuck |")
    print("|---|---|---|---|---|---|---|---|---|---|---|")
    for a in sys.argv[1:]:
        c, ps = a.split("=", 1)
        d, r, ls, nr, o, st, last, la, dn, cr, cn, cs = analyse(c.split("@")[0], ps.split(","))
        print("| %s | %d | %d | %d | %d | %d | %s | %d | %d | %s | %d / %d / %d |" % (c, d, r, ls, nr, o, ", ".join("%d (%s)" % x for x in st) or "-", last, la, "yes" if dn else "no", cr, cn, cs))
