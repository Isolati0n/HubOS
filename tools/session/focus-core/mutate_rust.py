#!/usr/bin/env python3 -I
"""Mutation check of the fuzz checker: break the Rust core on purpose and see that the fuzz run reports a violation.
Usage: python3 -I mutate_rust.py WORKDIR [kani]   (with "kani": the Kani proof must FAIL for every mutant; needs the Kani environment on PATH)"""
import os, shutil, subprocess, sys
WORK = sys.argv[1]
KANI = len(sys.argv) > 2 and sys.argv[2] == "kani"
HERE = os.path.dirname(os.path.abspath(__file__))
SRC = open(os.path.join(HERE, "rust", "src", "lib.rs")).read()
MUT = {
 "no_synth_key_release": ("out.push(Act { t: ActKind::SynthKu, dest: old, kind: Kind::Ku, code: self.keys[i].code, gen: 0, ep: 0 });", ""),
 "no_synth_button_release": ("out.push(Act { t: ActKind::SynthBu, dest: old, kind: Kind::Bu, code: self.btns[i].code, gen: 0, ep: 0 });", ""),
 "no_epoch_bump_on_request": ("self.epoch = self.epoch.saturating_add(1);\n        let old = self.focus;", "let old = self.focus;"),
 "deliver_without_conn_check": ("&& ev.ep == self.epoch\n            && self.conn[d as usize];", "&& ev.ep == self.epoch;"),
 "deliver_without_gen_check": ("&& ev.gen == self.gen[d as usize]\n            && ev.ep == self.epoch", "&& ev.ep == self.epoch"),
 "reconnect_without_release_all": ("out.push(Act { t: ActKind::ReleaseAll, dest: m, kind: Kind::Motion, code: 0, gen: 0, ep: 0 });", ""),
 "stale_verification_accepted": ("if is_machine(m) && self.waiting == m && ev.ep == self.epoch && self.conn[m as usize] {", "if is_machine(m) && self.waiting == m && self.conn[m as usize] {"),
 "restart_keeps_held": ("remove_dest(&mut self.keys, &mut self.nkeys, m);\n        remove_dest(&mut self.btns, &mut self.nbtns, m);\n        self.owed[mi] = true;", "self.owed[mi] = true;"),
 "owed_never_set": ("self.owed[old as usize] = true; // sent at reconnect", ""),
 "chord_dropped_while_waiting": ("// The hub path: never subject to the delivery rule, in every state.\n                out.push(Act { t: ActKind::DeliverHub, dest: HUB, kind: ev.kind, code: ev.code, gen: 0, ep: 0 });", "// mutant\n                if self.waiting == NONE { out.push(Act { t: ActKind::DeliverHub, dest: HUB, kind: ev.kind, code: ev.code, gen: 0, ep: 0 }); }"),
 "hub_input_with_stale_epoch": ("} else if d == HUB && self.focus == HUB && ev.ep == self.epoch {", "} else if d == HUB && self.focus == HUB {"),
}
for name, (a, b) in MUT.items():
    if a not in SRC:
        print("%-34s MUTATION DID NOT APPLY" % name); continue
    d = os.path.join(WORK, name)
    shutil.rmtree(d, ignore_errors=True)
    shutil.copytree(os.path.join(HERE, "rust"), d)
    open(os.path.join(d, "src", "lib.rs"), "w").write(SRC.replace(a, b, 1))
    env = dict(os.environ, CARGO_TARGET_DIR=os.path.join(d, "target"))
    if KANI:
        r = subprocess.run(["nice", "-n", "19", "cargo", "kani", "--harness", "step_keeps_all_invariants", "--output-format", "terse"], cwd=d, env=env, capture_output=True, text=True)
        out = r.stdout + r.stderr
        if "VERIFICATION:- FAILED" in out:
            print("%-34s proof FAILS (mutant detected)" % name)
        elif "VERIFICATION:- SUCCESSFUL" in out:
            print("%-34s PROOF STILL SUCCEEDS (mutant NOT detected)" % name)
        else:
            print("%-34s kani did not finish: %s" % (name, out.strip().splitlines()[-1][:80]))
        continue
    r = subprocess.run(["nice", "-n", "19", "cargo", "build", "--release", "--bin", "fuzz"], cwd=d, env=env, capture_output=True, text=True)
    if r.returncode != 0:
        print("%-34s BUILD FAILED: %s" % (name, r.stderr.strip().splitlines()[-1][:80])); continue
    f = subprocess.run([os.path.join(d, "target", "release", "fuzz"), "1", "200000"], capture_output=True, text=True)
    v = [l for l in f.stderr.splitlines() if l.startswith("VIOLATION")]
    print("%-34s %s" % (name, ("detected: " + v[0][:70]) if v else "NOT DETECTED in 200000 cases"))
