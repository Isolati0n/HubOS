# TLA+ model of the focus core

Files: `FocusCore.tla` (the model), `FocusCore.cfg` (the checked bounds), `FocusCoreRun.tla` (runs one scripted event sequence through the model's own step relation; used to make the shared trace suite), `tlc-output.txt` (the TLC output of the last full check), `mutants/run_mutants.py` and `mutants-result.txt` (the mutation check).
The rules are in `docs/proposals/focus-core.md` section 2.5; this file says what was checked and what was not.

## How it was checked (TESTED, 2026-10-11)
TLC 2026.10.06 (`tla2tools.jar` v1.8.0 from the TLA+ GitHub releases, sha256 `7beec0f04818732a62fa193731711a99aa4f11279499b2360a7d156c519ea78d`; the project publishes no hash I could compare, so the hash is recorded, not verified), Java 21 as already installed on the machine (nothing was installed). Command: `java -cp tla2tools.jar tlc2.TLC -workers 4 -config FocusCore.cfg FocusCore.tla`.
**Result: no error. 49,056,156 states generated, 210,417 distinct states, search depth 24, about 4 minutes.** Checked invariants: `TypeOK`, `NoBad` (invariants 1, 2, 4, 5 and the verification rules, checked inside every step), `NoStuckKeys` (invariant 3), `Bounded` (invariant 6).

## The bounds (what the checked model contains)
3 machines (the code supports 20), 2 distinct keys, 1 mouse button, the epoch bumps at most 4 times and each worker restarts at most once (these two bounds belong to the model; the real core saturates), the verification timeout is 2 model ticks. Event stamps (generation and epoch of an input) range over every value in those bounds, so stale, current and impossible stamps all occur. Every event can happen in every order that the urgent clock allows.

## What it does NOT cover
- More than 3 machines, more than 2 keys or 1 button: no interaction that needs a fourth machine or a third key is checked. The proofs and the fuzz runs in the three implementations cover the real sizes.
- **The held-key limit** (`MAX_KEYS`, `MAX_BTNS`): overflow is not modelled (2 keys never overflow 32). The fuzz run exercises it.
- Motion and scroll events (they change no state in the core; the fuzz run includes motion).
- Real time: the clock is the tick counter `age`. **Invariant 6 holds only under the model's assumption that the clock is urgent: once a verification is overdue, only events that end or restart the wait can happen, i.e. the I/O shell delivers `VerifyTimeout` when its timer fires.** The model does not prove the shell does that. The real-time bound is UNKNOWN until measured on the real hub in December.
- The I/O shell (sockets, the compositor, the presenter), the keyboard forwarder on the gaming box, the conductor.
- Concurrency inside the core: one event at a time is assumed.

## Is the model able to fail? (the mutation check, TESTED)
`mutants/run_mutants.py` breaks one rule at a time on purpose; TLC must report a violation. **All 12 mutants are detected** (`mutants-result.txt`): no synthesized releases, no epoch bump on a focus request, reconnect without release-all, delivery without the connection check, delivery without the generation check, hub input with a stale epoch, the flip chord dropped, a stale verification accepted, a release owed but never recorded, a worker restart that keeps held keys, a timeout that does not clear the wait, a stale timeout accepted. The first run of this check found two weaknesses in the checks themselves (a check that repeated the rule it was checking, and an equivalent mutant); both were fixed before the numbers above.

## Where the shared traces come from
`tools/session/focus-core/gen_traces.py` writes event scripts (19 hand-written scenarios, each exercising one rule, and 60 seeded random ones) and runs each through `FocusCoreRun.tla` with TLC. The states and actions TLC prints are the expected results in `tools/session/focus-core/traces/all.trace`. Every implementation runs the same file and must match every step.
