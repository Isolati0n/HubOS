#!/usr/bin/env python3 -I
"""Makes the shared trace suite (traces/all.trace) with the TLA+ model as the oracle.

A scenario is just a list of events. Each scenario is run by TLC through the model's own Step relation
(docs/models/focus-core/FocusCoreRun.tla); the states and actions TLC prints are written down as the expected results.
So the expected results come from the model, not from any implementation.

Usage: python3 -I gen_traces.py TLA2TOOLS_JAR WORKDIR [N_RANDOM] [SEED]
"""
import json, os, random, subprocess, sys

JAR, WORK = sys.argv[1], sys.argv[2]
NRAND = int(sys.argv[3]) if len(sys.argv) > 3 else 60
SEED = int(sys.argv[4]) if len(sys.argv) > 4 else 1
HERE = os.path.dirname(os.path.abspath(__file__))
MODEL = os.path.join(HERE, "..", "..", "..", "docs", "models", "focus-core")
N = 3

# --- events as tuples: (kind, a, code, gen, ep) ---
def kd(d, c=1, g=0, e=0): return ("kd", d, c, g, e)
def ku(d, c=1, g=0, e=0): return ("ku", d, c, g, e)
def bd(d, c=1, g=0, e=0): return ("bd", d, c, g, e)
def bu(d, c=1, g=0, e=0): return ("bu", d, c, g, e)
def chord(): return ("chord", 0, 0, 0, 0)
def menu(): return ("menu", 0, 0, 0, 0)
def req(t): return ("req", t, 0, 0, 0)
def vok(m, e): return ("vok", m, 0, 0, e)
def vto(m, e): return ("vto", m, 0, 0, e)
def rst(m): return ("rst", m, 0, 0, 0)
def dis(m): return ("dis", m, 0, 0, 0)
def rec(m): return ("rec", m, 0, 0, 0)

def tla(ev):
    k, a, c, g, e = ev
    if k in ("kd", "ku", "bd", "bu"):
        return '[kind |-> "%s", code |-> %d, dest |-> %d, gen |-> %d, ep |-> %d]' % (k, c, a, g, e)
    if k in ("chord", "menu"):
        return '[kind |-> "%s", code |-> 0, dest |-> 0, gen |-> 0, ep |-> 0]' % k
    if k == "req":
        return '[kind |-> "req", target |-> %d]' % a
    if k in ("vok", "vto"):
        return '[kind |-> "%s", m |-> %d, ep |-> %d]' % (k, a, e)
    return '[kind |-> "%s", m |-> %d]' % (k, a)

# --- hand-written scenarios: each exercises one rule ---
HAND = {
 "hub_input_at_start": [kd(0, 1, 0, 0), chord(), menu(), kd(0, 1, 0, 5)],
 "focus_change_verify_ok": [req(1), kd(1, 1, 0, 1), vok(1, 1), kd(1, 1, 0, 1), ku(1, 1, 0, 1)],
 "no_input_while_waiting": [req(2), kd(2, 1, 0, 1), kd(0, 1, 0, 1), vto(2, 1), kd(0, 1, 0, 1)],
 "stale_epoch_dropped": [req(1), vok(1, 1), kd(1, 1, 0, 1), req(2), kd(1, 1, 0, 1), kd(1, 1, 0, 2)],
 "synth_key_and_button_release": [req(1), vok(1, 1), kd(1, 1, 0, 1), kd(1, 2, 0, 1), bd(1, 1, 0, 1), req(2), vok(2, 2)],
 "stale_verification_ignored": [req(1), req(2), vok(1, 1), vok(2, 1), vok(2, 2), vto(2, 2)],
 "verification_timeout_falls_back": [req(3), vto(3, 1), kd(0, 1, 0, 1), chord()],
 "disconnect_flash_and_reconnect_release_all": [req(1), vok(1, 1), kd(1, 1, 0, 1), dis(1), ku(1, 1, 0, 1), kd(1, 2, 0, 1), rec(1), kd(1, 2, 0, 1)],
 "focus_away_from_dead_machine_owes_release": [req(1), vok(1, 1), kd(1, 2, 0, 1), bd(1, 1, 0, 1), dis(1), req(2), vok(2, 2), rec(1)],
 "worker_restart_of_focus_machine": [req(1), vok(1, 1), kd(1, 1, 0, 1), rst(1), kd(1, 1, 1, 1), rec(1), vok(1, 2), kd(1, 1, 1, 2)],
 "worker_restart_while_waiting": [req(2), rst(2), vok(2, 1), vok(2, 2), rec(2), vok(2, 2)],
 "worker_restart_of_other_machine": [req(1), vok(1, 1), rst(3), kd(1, 1, 0, 1), rec(3)],
 "gen_stamp_mismatch_dropped": [req(1), vok(1, 1), rst(1), rec(1), req(1), vok(1, 3), kd(1, 1, 0, 3), kd(1, 1, 1, 3)],
 "disconnect_while_waiting": [req(1), dis(1), vok(1, 1), vto(1, 1), rec(1), req(1), vok(1, 2)],
 "chord_reaches_hub_in_every_state": [req(1), chord(), vok(1, 1), chord(), dis(1), menu(), rst(1), chord(), req(0), menu()],
 "request_hub_cancels_wait": [req(2), req(0), vok(2, 1), vok(2, 2), kd(0, 1, 0, 2)],
 "repeat_request_same_machine": [req(1), vok(1, 1), kd(1, 1, 0, 1), req(1), vok(1, 2), kd(1, 1, 0, 2)],
 "double_connect_and_double_disconnect": [dis(2), dis(2), rec(2), rec(2), kd(0, 1, 0, 0)],
 "button_release_on_focus_change": [req(3), vok(3, 1), bd(3, 1, 0, 1), bd(3, 1, 0, 1), req(0)],
}

def random_script(rng, length):
    ev, gen, epoch = [], [0] * (N + 1), 0
    for _ in range(length):
        k = rng.randrange(100)
        cur_ep = epoch if rng.random() < 0.8 else rng.randrange(epoch + 2)
        d = rng.randrange(N + 1)
        g = (gen[d] if rng.random() < 0.8 else rng.randrange(3)) if d else 0
        if k < 40:
            kind = rng.choice(["kd", "ku", "bd", "bu"])
            c = rng.randrange(1, 3) if kind in ("kd", "ku") else 1
            ev.append((kind, d, c, g, cur_ep))
        elif k < 46:
            ev.append(chord() if rng.random() < 0.5 else menu())
        elif k < 66:
            ev.append(req(rng.randrange(N + 1))); epoch += 1
        elif k < 78:
            ev.append((rng.choice(["vok", "vto"]), rng.randrange(1, N + 1), 0, 0, cur_ep))
        elif k < 84:
            m = rng.randrange(1, N + 1); ev.append(rst(m)); gen[m] += 1; epoch += 1
        elif k < 92:
            ev.append(dis(rng.randrange(1, N + 1)))
        else:
            ev.append(rec(rng.randrange(1, N + 1)))
    return ev

def run_tlc(name, events):
    d = os.path.join(WORK, name)
    os.makedirs(d, exist_ok=True)
    script = "<<" + ", ".join(tla(e) for e in events) + ">>"
    with open(os.path.join(d, "Run.tla"), "w") as f:
        f.write("---- MODULE Run ----\nEXTENDS FocusCoreRun\nScriptDef == %s\n====\n" % script)
    with open(os.path.join(d, "Run.cfg"), "w") as f:
        f.write("SPECIFICATION RSpec\nCONSTANTS\n N = %d\n NKeys = 2\n NButtons = 1\n MaxEpoch = 1000\n MaxGen = 1000\n T = 2\n Trace = TRUE\n Script <- ScriptDef\nINVARIANT NotDone\n" % N)
    out = os.path.join(d, "trace.json")
    if os.path.exists(out):
        os.remove(out)
    r = subprocess.run(["java", "-XX:+UseSerialGC", "-cp", JAR + ":" + MODEL, "tlc2.TLC", "-config", "Run.cfg", "-dumpTrace", "json", out,
                        "-metadir", os.path.join(d, "meta"), "Run.tla"], cwd=d, capture_output=True, text=True,
                       env=dict(os.environ, TLA_PATH=MODEL))
    open(os.path.join(d, "tlc.out"), "w").write(r.stdout + r.stderr)
    return out

def to_sets(v):
    """TLC JSON writes sets as lists and functions on 1..N as lists or dicts; return python values."""
    return v

def dump(name, events, jpath):
    j = json.load(open(jpath))
    acts = j["counterexample"]["action"]
    states = [acts[0][0][1]] + [x[-1][1] for x in acts]   # TLC prints: [[n, state], action, [n+1, state]] per step
    lines = ["scenario " + name]
    for i, ev in enumerate(events):
        s = states[i + 1]
        lines.append("ev %s %d %d %d %d" % ev)
        for a in s["lastActs"]:
            t = a["t"]
            if t == "Deliver": lines.append("act D %d %s %d" % (a["dest"], a["kind"], a["code"]))
            elif t == "DeliverHub": lines.append("act H %s %d" % (a["kind"], a["code"]))
            elif t == "Drop": lines.append("act X")
            elif t == "Flash": lines.append("act F %d" % a["dest"])
            elif t == "SynthKU": lines.append("act SK %d %d" % (a["dest"], a["code"]))
            elif t == "SynthBU": lines.append("act SB %d %d" % (a["dest"], a["code"]))
            elif t == "RelAll": lines.append("act RA %d" % a["dest"])
            else: raise SystemExit("unknown action " + t)
        def fn(x):  # function on 1..N
            return [x[str(m)] if isinstance(x, dict) else x[m - 1] for m in range(1, N + 1)]
        def held(x):
            items = []
            for m, ks in zip(range(1, N + 1), fn(x)):
                for c in sorted(ks):
                    items.append("%d:%d" % (m, c))
            return ",".join(items) if items else "-"
        w = s["waiting"]
        lines.append("state focus=%d epoch=%d waiting=%s gen=%s conn=%s owed=%s keys=%s btns=%s" % (
            s["focus"], s["epoch"], "-" if w == -1 else str(w),
            ",".join(str(x) for x in fn(s["gen"])),
            ",".join("1" if x else "0" for x in fn(s["conn"])),
            ",".join("1" if x else "0" for x in fn(s["owed"])),
            held(s["held"]), held(s["heldB"])))
    lines.append("end")
    return "\n".join(lines) + "\n"

def main():
    os.makedirs(WORK, exist_ok=True)
    rng = random.Random(SEED)
    scenarios = list(HAND.items())
    for i in range(NRAND):
        scenarios.append(("random_%03d" % i, random_script(rng, rng.randrange(10, 30))))
    out = ["# Shared trace suite for the focus core. GENERATED by gen_traces.py; the expected results come from the TLA+ model\n",
           "# (docs/models/focus-core/FocusCoreRun.tla run by TLC). Do not edit by hand.\n", "n %d\n" % N]
    for name, events in scenarios:
        jpath = run_tlc(name, events)
        if not os.path.exists(jpath):
            raise SystemExit("TLC produced no trace for %s: see %s/%s/tlc.out" % (name, WORK, name))
        out.append(dump(name, events, jpath))
    with open(os.path.join(HERE, "traces", "all.trace"), "w") as f:
        f.write("".join(out))
    print("wrote %d scenarios" % len(scenarios))

main()
