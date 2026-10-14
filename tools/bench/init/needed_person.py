#!/usr/bin/env python3 -I
"""needed_person.py CAND=LOG[,LOG...] ... : every event of the fault suite that was counted as "needed a person" (EV line with ok=0),
with the fault, the target, the state the machine was in at the 60 s mark (the UNRECOVERED line's `why`), whether the harness's alert
file existed (alert=1: the policy layer had given up on purpose and raised the bar alert), the supervisor's own state where the
harness logged it, and the action the harness took in the person's place (the candidate's ops/fix, quoted from its file).
Same merge rule as event_counts.py (a step repeated after a self-reboot is counted once, from its first cycle)."""
import re, sys, os, collections
HERE = os.path.dirname(os.path.abspath(__file__))
CLASS = {
 "hang-stop": "H: a process that is alive but frozen (SIGSTOP); its supervisor shows it as up",
 "hang-spin": "H: a process that is alive but not answering (busy loop); its supervisor shows it as up",
 "after-supervisor-killed": "S: the per-service supervisor process itself was killed earlier, then the service was killed; nothing restarts it",
 "supervisor-killed": "S: the per-service supervisor process itself was killed (with the policy layer present: it gave up and raised the alert)",
 "storm": "R: a service that crashes at every start (40 s); the restart limit or the failed start leaves it down",
 "dep-slow": "D: a dependency (seatd) slow to start; a dependent was left stopped",
 "dep-fail": "D: a dependency (seatd) fails to start; the service was left stopped",
 "kill": "K: a plain kill of a service that was not restarted",
}
def fixtext(c):
    p = os.path.join(HERE, "candidates", c, "rootfs", "ops", "fix")
    try:
        L = [l.strip() for l in open(p) if l.strip() and not l.startswith("#")]
        return "; ".join(l for l in L if not l.startswith("#!"))
    except OSError:
        return "?"
def rows(c, paths):
    out = []; cycle = 0; first = {}; cur = None; pend = {}
    for p in paths:
        cur = None
        for l in open(p, errors="replace"):
            if " CHAOS start " in l: cycle += 1
            m = re.search(r" STEP (\d+) ", l)
            if m: cur = int(m[1])
            m = re.search(r" UNRECOVERED ev=(\d+) fault=(\S+) target=(\S+) alert=(\d) why=\"([^\"]*)\"", l)
            if m: pend[(m[1], cycle)] = (m[2], m[3], m[4], m[5])
            m0 = re.search(r" EV ev=", l)
            if m0 and cur and first.setdefault(cur, cycle) != cycle: continue     # same rule as event_counts.py: first cycle with any event of the step
            m = re.search(r" EV ev=(\d+) fault=(\S+) target=(\S+) rec=([\d.]+) ok=0 (.*)", l)
            if m and cur:
                u = pend.get((m[1], cycle), (m[2], m[3], "?", "?"))
                out.append((cur, m[2], m[3], u[3], u[2], m[5].strip()))
    return out
if __name__ == "__main__":
    print("| candidate | step | fault | target | state at 60 s | alert file | other facts from the log | what the harness did in the person's place (ops/fix) |")
    print("|---|---|---|---|---|---|---|---|")
    cnt = collections.Counter()
    for a in sys.argv[1:]:
        c, ps = a.split("=", 1)
        for st, f, t, why, al, rest in rows(c.split("@")[0], ps.split(",")):
            cnt[(c, f)] += 1
            print("| %s | %d | %s | %s | %s | %s | %s | `%s` |" % (c, st, f, t, why, al, rest.replace("|", "/"), fixtext(c.split("@")[0]).replace("|", "/")))
    print()
    print("CLASSCOUNT", dict(cnt))
