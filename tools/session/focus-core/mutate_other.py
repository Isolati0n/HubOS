#!/usr/bin/env python3 -I
"""Mutation check of the fuzz checker for the Zig and Ada cores (same mutants as mutate_rust.py).
Usage: python3 -I mutate_other.py zig|ada WORKDIR     (Zig: env ZIG=path; Ada: env GNAT_ROOT=path)
A mutant counts as detected when the fuzz run (or, for Ada, a run-time contract check) reports a problem within 200000 cases."""
import os, shutil, subprocess, sys
LANG, WORK = sys.argv[1], sys.argv[2]
HERE = os.path.dirname(os.path.abspath(__file__))
if LANG == "zig":
    FILE = "src/core.zig"
    MUT = {
     "no_synth_key_release": ("if (self.keys[i].dest == old) out.push", "if (false) out.push"),
     "no_synth_button_release": ("if (self.btns[i].dest == old) out.push", "if (false) out.push"),
     "no_epoch_bump_on_request": ("self.epoch +|= 1; // the epoch is bumped FIRST (saturating)\n", ""),
     "deliver_without_conn_check": ("ev.ep == self.epoch and self.conn[d];", "ev.ep == self.epoch;"),
     "deliver_without_gen_check": ("ev.gen == self.gen[d] and ev.ep == self.epoch", "ev.ep == self.epoch"),
     "reconnect_without_release_all": ("out.push(.{ .t = .release_all,", "if (false) out.push(.{ .t = .release_all,"),
     "stale_verification_accepted": ("self.waiting == m and ev.ep == self.epoch and self.conn[m]", "self.waiting == m and self.conn[m]"),
     "restart_keeps_held": ("self.conn[m] = false;\n        removeDest(&self.keys, &self.nkeys, m);\n        removeDest(&self.btns, &self.nbtns, m);\n", "self.conn[m] = false;\n"),
     "owed_never_set": ("self.owed[old] = true;\n", "\n"),
     "chord_dropped_while_waiting": (".chord, .menu => out.push(", ".chord, .menu => if (self.waiting == NONE) out.push("),
     "hub_input_with_stale_epoch": ("d == HUB and self.focus == HUB and ev.ep == self.epoch", "d == HUB and self.focus == HUB"),
    }
else:
    FILE = "src/focus_core.adb"
    MUT = {
     "no_synth_key_release": ("if S.Keys (I).Dest = Old then\n                  Push (Output, (T => Synth_Ku", "if False then\n                  Push (Output, (T => Synth_Ku"),
     "no_synth_button_release": ("if S.Btns (I).Dest = Old then\n                  Push (Output, (T => Synth_Bu", "if False then\n                  Push (Output, (T => Synth_Bu"),
     "no_epoch_bump_on_request": ("      S.Epoch := Sat_Inc (S.Epoch);\n      if Is_Machine (Old) then", "      if Is_Machine (Old) then"),
     "deliver_without_conn_check": ("and then Ev.Ep = S.Epoch and then S.Conn (Integer (D));", "and then Ev.Ep = S.Epoch;"),
     "deliver_without_gen_check": ("and then Ev.Gen = S.Gen (Integer (D))\n        and then Ev.Ep", "and then Ev.Ep"),
     "reconnect_without_release_all": ("Push (Output, (T => Release_All,", "if False then Push (Output, (T => Release_All,"),
     "stale_verification_accepted": ("S.Waiting = Ev.A and then Ev.Ep = S.Epoch and then S.Conn (Integer (Ev.A))", "S.Waiting = Ev.A and then S.Conn (Integer (Ev.A))"),
     "restart_keeps_held": ("         Remove_Dest_Keys (S, M);\n         Remove_Dest_Btns (S, M);\n", ""),
     "owed_never_set": ("S.Owed (Integer (Old)) := True;", "null;"),
     "hub_input_with_stale_epoch": ("elsif D = Hub and then S.Focus = Hub and then Ev.Ep = S.Epoch then", "elsif D = Hub and then S.Focus = Hub then"),
    }
SRC = open(os.path.join(HERE, LANG, FILE)).read()
if LANG == "ada":   # the two statements that need a matching "end if;"
    MUT["reconnect_without_release_all"] = ("Push (Output, (T => Release_All, Dest => Ev.A, K => Motion, Code => 0, Gen => 0, Ep => 0));", "null;")
for name, (a, b) in MUT.items():
    if a not in SRC:
        print("%-34s MUTATION DID NOT APPLY" % name); continue
    d = os.path.join(WORK, name)
    shutil.rmtree(d, ignore_errors=True)
    shutil.copytree(os.path.join(HERE, "traces"), os.path.join(d, "traces"))
    d = os.path.join(d, LANG)
    shutil.copytree(os.path.join(HERE, LANG), d, ignore=shutil.ignore_patterns("out"))
    open(os.path.join(d, FILE), "w").write(SRC.replace(a, b, 1))
    if LANG == "zig":
        r = subprocess.run(["nice", "-n", "19", "sh", os.path.join(d, "run.sh")], capture_output=True, text=True)
        exe = os.path.join(d, "out", "fuzz")
    else:
        r = subprocess.run(["nice", "-n", "19", "sh", os.path.join(d, "build.sh")], capture_output=True, text=True)
        exe = os.path.join(d, "out", "fuzz_run")
    if r.returncode != 0 or not os.path.exists(exe):
        print("%-34s BUILD FAILED: %s" % (name, (r.stderr.strip().splitlines() or ["?"])[-1][:90])); continue
    f = subprocess.run([exe, "1", "200000"], capture_output=True, text=True)
    txt = (f.stdout + f.stderr).strip().splitlines()
    bad = [l for l in txt if l.startswith("VIOLATION") or "violations=" in l and "violations=0" not in l or "raised" in l or "failed" in l.lower()]
    print("%-34s %s" % (name, ("detected: " + bad[0][:80]) if bad or f.returncode != 0 else "NOT DETECTED in 200000 cases"))
