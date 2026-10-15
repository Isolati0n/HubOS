# Focus core: results of the three candidates (stage 1, part A)

Date: 2026-10-11. Labels: TESTED = I ran it and saw the result; SOURCE = read in a document or source; UNKNOWN = not established. **No winner is picked.** Code: `tools/session/focus-core/{rust,zig,ada}/`. Model: `docs/models/focus-core/`. Rules: `docs/proposals/focus-core.md`. No helper agents were used.

## 1. What each candidate shows for each invariant

Invariant numbers are those of `focus-core.md` section 3. "Fuzz" = the independent checker (`after_step` in each language, built from the pre-state, the event, the actions and a ghost of what every node believes is held) run after every step.

| Invariant | Rust | Zig | Ada |
|---|---|---|---|
| 1 no stale delivery | proven by Kani (small bounds) + fuzzed (full bounds) | fuzzed only | fuzzed only |
| 2 at most one destination | proven (small bounds) + fuzzed | fuzzed only | fuzzed only |
| 3 no stuck keys | proven (small bounds) + fuzzed | fuzzed only | fuzzed only |
| 4 epoch never decreases | proven (small bounds) + fuzzed | fuzzed only | fuzzed; also the `Step` postcondition `Epoch >= Epoch'Old` is proven by GNATprove at full bounds |
| 5 hub path always reachable | proven (small bounds) + fuzzed | fuzzed only | fuzzed; the postcondition "chord/menu gives exactly one `Deliver_Hub` first" is proven at full bounds |
| 6 bounded transition | model only (TLC, with the urgent-clock assumption); the core has no clock; `stale_verification_accepted` mutant shows the check sees it | same | same |
| No crash (index, overflow, range) | proven by Kani within the small bounds; plus overflow-checked arithmetic | not proven; Zig `ReleaseSafe` traps, 0 traps in the 10 fuzz chunks (2000 M steps) | **proven by GNATprove at the real bounds (20 machines, 32 keys, 8 buttons)**: 96 of 96 run-time checks |
| State stays valid (limits, "waiting means hub has focus", only the focus holds anything, hub holds nothing) | proven (small bounds) | fuzzed only | **proven at full bounds** (`Step` postcondition `Valid`) |

So: only Rust has a machine proof of invariants 1–5 (and only at the small bounds, 3 machines / 3 keys / 2 buttons: the model checker does not finish at 20/32/8). Ada has a stronger proof of a smaller thing (no run-time error and a valid state, at the real bounds) and tests invariants 1–5. Zig has tests only (UNKNOWN whether any Zig prover exists; I did not find one). Invariants 1–5 at the real bounds are TESTED only in all three candidates. SPARK could express invariants 1–5 with ghost code, as Kani's checker does; I did not write it (see 5).

## 2. Checks that were run (TESTED)

- Shared trace suite: 79 scenarios, 1340 steps (`traces/all.trace`, generated from the TLC runs of the model): Rust, Zig, Ada all 0 mismatches (state and actions compared at every step). The Ada trace was re-run with run-time contracts on (`-gnata`) after the SPARK edits: still 0 mismatches.
- Fuzz: 10 chunks x 10 000 000 cases (about 200 M steps per chunk), new seed per chunk (1001 to 1010), per candidate: **0 violations, 0 crashes, 0 hangs**; and the run hash (every action and the final state) is **identical in all three languages** for each seed (compared for all ten seeds; Ada rerun after the SPARK edits for seed 1001). Files: `results/{rust,zig,ada}-fuzz.txt`.
- Mutation check of the fuzz checker: Rust 11/11 mutants detected, Zig 11/11, Ada 10/10 (the 11th, "chord dropped while waiting", was not written for Ada; one Ada mutant is caught by a run-time contract rather than by the checker). Kani: 11/11 mutants make the proof fail. Files: `results/*-mutants.txt`.
- Model: TLC, 49 056 156 states generated, 210 417 distinct, 0 errors (N=3 machines, 2 keys, 1 button); mutation run of the model in `docs/models/focus-core/mutants-result.txt`.

## 3. Comparison table (TESTED unless marked)

| | Rust (+ Kani) | Zig | Ada (+ SPARK) |
|---|---|---|---|
| Core lines (types, `step`, helpers; no checker) | about 385 (`lib.rs` lines 1-386) | about 222 (`core.zig` lines 1-222) | 413 (`.ads` 96 + `.adb` 317; includes about 130 lines of contracts and loop invariants) |
| Checker lines | about 155 | about 90 | 167 |
| Proof effort | `proofs` module about 115 lines; the first attempt's mutation run found two gaps in the checker which I fixed; proof took 61.7 s | none (no prover) | four GNATprove runs (not timed in hours) to get from "1 postcondition unproved" to all proved: helper postconditions and loop invariants (about 130 lines), one added `Valid` clause (hub holds nothing), two postconditions on the count functions; final run 22 s |
| Proof result | `step_keeps_all_invariants`: 1959 checks, 0 failed, 9 of 9 cover properties reached; `initial_state_is_valid`: 226 checks, 0 failed (Kani 0.68.0 / CBMC 6.11.0, small bounds) | n/a | GNATprove 16.1.0: 168 checks, all proved, 0 unproved (96 run-time, 56 assertions, 9 functional contracts, 7 termination) at full bounds |
| Clean build time | not measured reliably (cargo reported 0.5 s from a warm directory; UNKNOWN) | 38 s | 2.4 s (GNAT 13) |
| Fuzz throughput (steps/s, one thread) | about 6.0 M | about 7.9 M | about 5.1 M (200 M steps in 38 s) |
| Binary size (fuzz binary) | 505 KB (395 KB stripped) | 4.2 MB (static, ReleaseSafe) | 64 KB (dynamic, needs libgnat) |
| Tool problems | Kani needs a private cargo/rustup and a 1+ GB bundle; fine without root | Zig 0.17.0 std API changed from my memory (`main(init)`, no `**` for arrays); no prover | GNAT 13 from Ubuntu unpacked with `dpkg -x` (no root); `gnat1` and `libgnat` paths needed manual flags; SPARK is not in Ubuntu: GNATprove 16.1.0 (1.3 GB) came from Alire into a private folder, worked first time; provers are the bundled CVC5, Z3, Alt-Ergo |

Line counts are from `wc -l` on the files; the split between "core" and "checker" is by the module boundaries in each file.

## 4. What this does NOT show

- The real-time bound of invariant 6 (UNKNOWN until hardware exists, December). The core never measures time.
- That the six invariants are the right ones or complete for the real hub: they were written before the code (owner decision), and checked against `HUB-OS.md`.
- Anything about the I/O shell (how events arrive, the stamping of epoch and generation, the verification timer). The core is pure.
- Zig and Ada invariants at the real bounds are TESTED by fuzz only, not proven.
- Proofs use Kani's small bounds (3/3/2): a bug that needs 4 or more machines or keys would not be seen by the proof (fuzz has 3 to 20 machines).

## 5. Open questions for the owner (nothing was blocked on them)

1. Limits: are `MAX_KEYS = 32`, `MAX_BUTTONS = 8`, `MAX_ACTS = 64` placeholders OK, and is "drop a press that does not fit" right?
2. What does "verified" mean for a machine?
3. How does the compositor's window focus become a `FocusRequest`?
4. How does the presenter learn the current epoch and generation to stamp inputs?
5. Which physical key is the "one menu key"?
6. Are scroll events button events for `held`?
7. Is a window flash acceptable as the cue for input dropped on a down machine?
8. Should I also write invariants 1-5 as SPARK ghost code (more proof work; size UNKNOWN) so that Ada reaches the same level as Rust, or is "no run-time error + valid state" enough for the comparison?
9. The Ada binary needs `libgnat` (dynamic) unless built with static runtime; the Zig binary is static. Does static linking matter for the hub?
