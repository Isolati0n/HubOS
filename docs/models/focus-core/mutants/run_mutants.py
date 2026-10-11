#!/usr/bin/env python3 -I
"""Mutation check of the TLA+ model: each mutant breaks one rule on purpose; TLC must report a violation.
A model whose invariants hold on a broken rule would be worth nothing. Usage: python3 -I run_mutants.py JAR WORKDIR"""
import os, shutil, subprocess, sys
JAR, WORK = sys.argv[1], sys.argv[2]
HERE = os.path.dirname(os.path.abspath(__file__))
SRC = open(os.path.join(HERE, "..", "FocusCore.tla")).read()
MUT = {
 "no_synth_releases": ("acts  == IF send THEN", "acts  == IF FALSE THEN"),
 "no_epoch_bump_on_request": ("     /\\ epoch' = epoch + 1\n     /\\ held' = [m \\in M |-> {}]", "     /\\ epoch' = epoch\n     /\\ held' = [m \\in M |-> {}]"),
 "reconnect_without_release_all": ('Finish({ [t |-> "RelAll", dest |-> m] }, ev)', "Finish({}, ev)"),
 "deliver_without_conn_check": ("ok      == d \\in M /\\ d = focus /\\ ev.gen = gen[d] /\\ ev.ep = epoch /\\ conn[d]", "ok      == d \\in M /\\ d = focus /\\ ev.gen = gen[d] /\\ ev.ep = epoch"),
 "deliver_without_gen_check": ("ok      == d \\in M /\\ d = focus /\\ ev.gen = gen[d] /\\ ev.ep = epoch /\\ conn[d]", "ok      == d \\in M /\\ d = focus /\\ ev.ep = epoch /\\ conn[d]"),
 "hub_input_with_stale_epoch": ("hubok   == d = HUB /\\ focus = HUB /\\ ev.ep = epoch", "hubok   == d = HUB /\\ focus = HUB"),
 "chord_dropped": ('acts    == IF special THEN { [t |-> "DeliverHub", kind |-> ev.kind, code |-> ev.code] }', 'acts    == IF special THEN { [t |-> "Drop"] }'),
 "stale_verification_accepted": ('CASE ev.kind = "vok" -> waiting = ev.m /\\ ev.ep = epoch /\\ conn[ev.m]', 'CASE ev.kind = "vok" -> waiting = ev.m /\\ conn[ev.m]'),
 "owed_never_set": ("owed' = [m \\in M |-> IF oldM /\\ m = old /\\ ~conn[old] /\\ (hk # {} \\/ hb # {}) THEN TRUE ELSE owed[m]]", "owed' = owed"),
 "restart_keeps_held": ("     /\\ held' = [held EXCEPT ![m] = {}]\n     /\\ heldB' = [heldB EXCEPT ![m] = {}]\n     /\\ owed' = [owed EXCEPT ![m] = TRUE]", "     /\\ held' = held\n     /\\ heldB' = heldB\n     /\\ owed' = [owed EXCEPT ![m] = TRUE]"),
 "timeout_does_not_clear_the_wait": ("ELSE /\\ focus' = HUB /\\ waiting' = NONE /\\ age' = 0", "ELSE /\\ focus' = HUB /\\ waiting' = waiting /\\ age' = 0"),
 "stale_verification_timeout_accepted": ('[] ev.kind = "vto" -> waiting = ev.m /\\ ev.ep = epoch    \\* the timer', '[] ev.kind = "vto" -> waiting = ev.m    \\* the timer'),
}
cfg = "SPECIFICATION Spec\nCONSTANTS\n N = 3\n NKeys = 2\n NButtons = 1\n MaxEpoch = 3\n MaxGen = 1\n T = 2\n Trace = FALSE\nINVARIANTS TypeOK NoBad NoStuckKeys Bounded\n"
results = []
for name, (a, b) in MUT.items():
    if a not in SRC:
        results.append((name, "MUTATION DID NOT APPLY")); continue
    d = os.path.join(WORK, name); os.makedirs(d, exist_ok=True)
    open(os.path.join(d, "FocusCore.tla"), "w").write(SRC.replace(a, b, 1))
    open(os.path.join(d, "FocusCore.cfg"), "w").write(cfg)
    try:
        r = subprocess.run(["java", "-XX:+UseParallelGC", "-Xmx2g", "-cp", JAR, "tlc2.TLC", "-workers", "4", "-config", "FocusCore.cfg",
                            "-metadir", os.path.join(d, "meta"), "FocusCore.tla"], cwd=d, capture_output=True, text=True, timeout=900)
        out = r.stdout + r.stderr
    except subprocess.TimeoutExpired:
        results.append((name, "NOT DETECTED within 900 s")); continue
    v = [l for l in out.splitlines() if l.startswith("Error:")]
    results.append((name, "detected: " + v[0] if v else "NOT DETECTED (model checking completed without error)"))
for n, r in results:
    print("%-34s %s" % (n, r))
