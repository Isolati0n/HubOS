# Init comparison: Step 2 run log (TESTED/SOURCE/BELIEVED/UNKNOWN labels per line)

Started 2026-10-10. Plan (owner): (a) quick scorecard measurements; (b) elimination round by fault injection; (c) 30-60 min pilot on one finalist to measure memory noise and set the chunk length; (d) soak of finalists only, at least 4 h each, one at a time, chunks of 2-4 h, new random seed per chunk, PSS from smaps_rollup, leak slope per thousand restarts; (e) stop early on clear failure. No init is written here. Results are added below as they finish.

Environment: cloud container, QEMU 8.2.2 without KVM (timings are distorted). Long jobs are waited on with a Monitor loop (see CLAUDE.md in PR #75).

## What each label is (provenance, SOURCE: `tools/bench/init/candidates/*/rootfs/cand/boot.sh`, `build.sh`, git history)
None of these is a custom init, and nothing in them was written as an init. Every candidate runs the same stand-in services (`hubsim`, a small Go program that behaves like seatd, udevd, dbus, driftwm, waybar and hubd for what an init can see) under the same fault injector. The candidate folders, the stand-ins, the injector and the two brain prototypes were all committed on 2026-10-05, before Step 2 (commits b0eac48, 47b3110), except where the last column says otherwise.

| Label | What it is | PID 1 | Code | Written in Step 2? |
|---|---|---|---|---|
| s6plain | s6 alone: hand-written `run` scripts per service, no s6-rc | s6-svscan | `candidates/s6plain/` | no (2026-10-05) |
| s6rc | s6 plus the s6-rc service graph; my first setup lacked the follow rule and the `-wait` rules | s6-svscan | `candidates/s6rc/` | no |
| s6rc-par | s6rc with the same wait and follow rules as s6plain (parity fix) | s6-svscan | `candidates/s6rc-par/` | **yes** (2026-10-10) |
| runit | runit as its own PID 1 (stage 1/2/3, `runsvdir`, `runsv`) | runit | `candidates/runit/` | no |
| dinit | dinit as PID 1, hard `depends-on` | dinit | `candidates/dinit/` | no |
| dinit-par | dinit with the same wait and follow rules | dinit | `candidates/dinit-par/` | **yes** (2026-10-10) |
| dinit-sm | dinit-par plus `smooth-recovery = true` on four services | dinit | `candidates/dinit-sm/` | **yes** (2026-10-10) |
| dinit-ms | dinit-par with `depends-ms` and an unlimited restart count (follow-up, section "Follow-up") | dinit | `candidates/dinit-ms/` | **yes** (2026-10-11) |
| openrc | OpenRC with supervise-daemon | openrc-init | `candidates/openrc/` | no |
| openrc-par | openrc with the same wait and follow rules | openrc-init | `candidates/openrc-par/` | **yes** (2026-10-10) |
| openrc-doc, openrc-doc2 | openrc-par changed only with options OpenRC's documentation describes (follow-up) | openrc-init | `candidates/openrc-doc/`, `openrc-doc2/` | **yes** (2026-10-11 and 12) |
| **c-go** | **NOT a custom PID 1.** s6 + s6-rc (architecture C) plus the Go "policy brain" as one extra s6 service; the brain probes the compositor and commands s6 (`s6-svc`) | s6-svscan | `candidates/c-go/`, brain: `prototypes/go/` | no (2026-10-05) |
| **c-elixir** | **NOT a custom PID 1.** The same as c-go with the brain written in Elixir/OTP (Erlang/OTP 25 runtime, unpacked with `dpkg -x`) | s6-svscan | `candidates/c-elixir/`, brain: `prototypes/elixir/` | no |
| **b-go** | **Yes: "the Go policy brain over s6"**, but not over s6-rc: architecture B approximated. s6-svscan is the PID 1 (it only reaps and restarts the brain and the watchdog feeder), and the Go brain starts and watches the six services itself (mode `direct`, no s6-supervise per service, no s6-rc). It is not an init. | s6-svscan | `candidates/b-go/`, brain: `prototypes/go/` | no |
| pid1-stubs | tiny PID 1 programs in C, Rust, Zig, Go, measured only for size, start time and panic behaviour (not candidates in the fault suite) | n/a | `prototypes/pid1-stubs/`, results `results/pid1-stubs.txt` | no |

Harness code changed in Step 2 (all of it listed in this log): the `supOf` fix in `hubsim/chaos.go`, the soak mode (PSS every 60 s, restart counter), `run-qemu.py --skip`, `soakq.sh`, `soak_analyze.py`, `report.py` (candidate list from the environment), and in the follow-up `soak_ci.py` and a timing line in the trial hook's rollback message.

## Log
- 2026-10-10 TESTED: the benchmark tools rebuild with `fetch-and-build.sh` (unchanged script; only the last step needed the repo path) and all 8 candidate images build. Quick measurements (1 boot each, QEMU no KVM): see table below.
- Quick scorecard row (one boot each; supervisor start to desktop s / PSS of PID 1 + supervisors + policy kB / init-side idle CPU ticks in 30 s): s6plain 5.7 / 2490 / 1; s6rc 6.4 / 2682 / 0; runit 6.0 / 1904 / 1; dinit 5.0 / 2096 / 11; openrc 8.5 / 706 / 0; c-go 6.1 / 11038 / 218; c-elixir 6.2 / 34814 / 218; b-go 7.4 / 4948 / 27. One boot each is not a distribution; timings are distorted by emulation.
- s6plain elimination unit (3 boots, 2 PID-1 crashes, 1 PID-1 wedge, 3 trial boots, fault suite reps 3) in `tools/bench/init/results/step2/s6plain/`. Fault suite stopped making progress at 830 s after a supervisor-kill (waybar recovery 25.4 s): from then the run log is a loop of s6-log lock errors until the 5400 s timeout (UNKNOWN whether candidate or harness cause). A rerun of steps 35 to 43 alone completed (scanner-kill step did nothing: PID 1 is not killable by SIGKILL from inside; clocks, OOM ok). Kills of the six services recovered in 1.3 to 6.4 s; a hung driftwm (SIGSTOP or spin) is never noticed (65 s = the harness fixed it, needs a person).
- s6rc (s6 + s6-rc, no policy layer) elimination unit, `results/step2/s6rc/`: boots 3 (6.3 to 6.4 s), PID 1 killed: back in 20 s (both), PID 1 wedged: back in 51 s, trial boots: healthy confirmed 22.3 s, driftwm cannot start rolled back at 60 s with the guard feeding, driftwm wedged 2 s after working CONFIRMED (false confirmation, 21.8 s). Fault suite: 8 launches, never finished (1815 s, suite_done=False): after `kill -9` of driftwm or seatd, hang-stop of driftwm and a driftwm crash storm the run was left with only Waybar and no hubd (`STUCK ... clients missing: ok clients=waybar`, a reboot is the only way out) 8 times; kills of udevd, dbus, waybar, hubd recovered in 0.2 to 1.9 s. A hung driftwm (SIGSTOP) is not noticed. Crash storm: no give-up and no alert (35 starts of hubd in 40 s, `alert=no`).
  - **Dropped as a bare candidate (rule 2.2 of the scorecard: a test that leaves the machine in a state only a reboot clears is rated 0): s6rc alone fails R1 (service killed) and R2 (hung service). Reason: after driftwm or seatd dies its dependents hubd and Waybar are not brought back in the right order, and nothing probes health. s6-rc remains in the comparison only as the base of the policy-layer candidates c-go and c-elixir (the scorecard scores "with a layer" separately).** Label: TESTED (one run of reps 3; the stuck state repeated 8 times).

## Answers to the owner's three questions (written 2026-10-10 during the elimination round; updated at the end)

1. **s6plain stuck loop: verdict UNKNOWN (candidate or harness not decided).** Correction of my own earlier wording: the loop did NOT start at the scanner-kill step. In the full run (`s6plain-faults-1.log`) the last event is `ev=37 supervisor-killed target=waybar rec=25.40` at host time 828.4 s; the first `s6-log: fatal: unable to lock` / `guard: cannot open watchdog` lines are at 830.6 s; neither the matching `after-supervisor-killed` event nor `STEP 37 fScanKill` was ever logged, and the same lines repeat (25 978 times) until the 5400 s timeout. A rerun of steps 35 to 43 alone on a fresh boot (`s6plain-faults-rerun-from-step35.log`) passed the same waybar step (ev 3, 4) and all later steps; the scanner step produced no event at all (PID 1 is s6-svscan and a SIGKILL from inside is not delivered). So it happened once, after 36 earlier faults, and did not recur in isolation. TESTED: those two facts. UNKNOWN: the cause. A repeat of the whole s6plain fault suite is needed to say if it recurs (queued at the end of the round, see below).
   - The `--skip` change to `run-qemu.py` (SOURCE: `git diff`: one new argument with default 0, and `skip = a.skip` instead of `skip = 0`) changes nothing for any run that does not pass `--skip`. Only the two s6plain re-test runs used it; no other candidate's result used it.
2. **Two VMs at once:** once, for about 20 s (17:03:32 to 17:03:54 UTC), by my mistake: my first diagnostic rerun (`--skip 37`) was still running when I started the second (`--skip 34`). I stopped both, deleted both logs and ran the `--skip 34` test again alone (17:03:54 to 17:07:14); only that alone-run is in the repository. No elimination-unit result (s6plain 15:22 to 17:00, s6rc from 17:07:30, runit from 17:46:36) overlapped another VM (the `loadavg_at_start` values in their logs are 0.07 to 0.58). Nothing recorded needs to be marked not counted. (Only short report.py and git commands ran beside the units.)
3. **Dropped candidates and configuration** (s6rc so far; others below as they finish):
   - s6rc: BELIEVED that a configuration change fixes it, NOT TESTED. The service definitions were NOT equivalent: s6plain's Waybar and hubd are started through `follow-driftwm` (the hub image's own script: waits for the compositor, exits and is restarted when the compositor goes away or is replaced), while the s6rc definitions only declare start-order dependencies (`dependencies.d`: waybar after dbus and driftwm, hubd after driftwm) and readiness by notification fd; s6-rc starts dependents once at bring-up and has no rule that restarts them when a dependency restarts. The run lost hubd after a driftwm or seatd death, which is what the missing rule would be expected to cause. So the s6rc elimination above compares s6rc WITHOUT the restart rule against s6plain WITH it, and that is not a like-for-like comparison. Rerunning s6rc with the same `follow-driftwm` run scripts would be the fair test; I have NOT done it (not in the plan) and ask the owner.

### runit elimination unit (results/step2/runit/) and a HARNESS BUG found by it
- TESTED: 3 boots 6.1 s; PID 1 killed: back in 16 and 15 s; PID 1 wedged: back in 48 s; trial boots: healthy confirmed 19.8 s, driftwm cannot start CONFIRMED (false: Waybar not running, 19.4 s), driftwm wedged CONFIRMED (false, 19.6 s). Fault suite steps 1 to 30: kills of the six services recovered in 0.15 to 4.4 s (no person needed, 18 of 18); hang-stop of driftwm never noticed (needed the harness, 63 s); a busy-looping driftwm noticed once in 0.08 s and not in 2 of 3; crash storms ended with the services running; slow seatd 19.2 s, failed seatd 4.6 s; disk full: services stayed up.
- **Harness bug, SOURCE (panic trace at the end of `runit-faults-1.log`): at step 31 (fSupKill) the fault injector itself crashed: `panic: runtime error: slice bounds out of range [1:0]` in `supOf` (`hubsim/chaos.go:119`: the command line of a process that had just exited was empty and `Fields(...)[1:]` panics). The run then waited until the 5400 s timeout (`suite_done=False`). Steps 31 to 43 (supervisor killed, scanner killed, brain killed, clock jumps, OOM) were NOT measured for runit.** Not a runit failure; not counted as one. Fixed in `hubsim/chaos.go` (skip a process whose command line is empty); the fix is a new commit. It could affect any candidate run; s6rc and s6plain logs contain no such panic trace (checked with grep), so their stops have other causes (s6plain: unknown, see question 1; s6rc: dependents not restarted). The runit steps 31 to 43 will be rerun with `--skip 30` after the images are rebuilt with the fixed injector (rebuilding changes the images of all candidates; results already taken keep their old injector; the change only affects the one code path that panicked).

### dinit and openrc elimination units (results/step2/dinit/, results/step2/openrc/) and a FAIRNESS PROBLEM in my own setup
- dinit (TESTED): 3 boots 4.8 to 5.3 s; PID 1 killed: back in 15 and 18 s; PID 1 wedged: back in 46 s; trial boots: healthy confirmed 20.2 s, driftwm cannot start rolled back at 60 s (guard feeding), driftwm wedged CONFIRMED (false, 20.0 s). Fault suite: 8 launches, never finished (2016 s, `suite_done=False`), **8 stuck states** (`STUCK`: a reboot is the only way out): after `kill -9` of dbus, udevd or driftwm (Waybar or driftwm "not running"), crash storms of hubd and driftwm, and a failed seatd. Kills of seatd, waybar, hubd recovered in 0.1 to 5.7 s; hang-stop/hang-spin of driftwm never noticed (needed the harness, 64 s); slow seatd 32.9 s.
- openrc (TESTED): 3 boots 8.4 to 9.6 s; PID 1 killed: back in 20 and 22 s; PID 1 wedged: back in 51 s; **all three trial boots, including the healthy one, ended in `ROLLBACK reboot at 60 s`** (UNKNOWN why a healthy trial rolled back: `true_state` was empty, so the harness's trial hook may not work for this candidate); fault suite finished (`suite_done=True`, 3 launches), 2 stuck states (crash storm on driftwm; failed seatd). `kill -9` of seatd or driftwm: **not recovered within the harness's 60 s** (rec=60.5 to 63.8 s, `ok=0`) in 6 of 6; supervisor-killed and after-supervisor-killed: also 60 s (a person needed); a hubd crash storm never ended (60 s). Other kills recovered in 1.2 to 2.9 s.
- **Fairness problem (SOURCE: the run scripts in `tools/bench/init/candidates/`):** s6plain and runit start Waybar and hubd through the image's `follow-driftwm` wrapper and give driftwm `-wait /run/seatd.sock,/run/udev/ready` (they wait for the compositor's and seatd's sockets and exit and start again when the compositor is replaced). **s6rc, dinit and openrc have neither**: their run commands are `policy driftwm --backend udev -mem 100 -notify 3`, `waybar -notify 3`, `policy hubd -notify 3` with only start-order dependencies (s6-rc `dependencies.d`, dinit `depends-on`, OpenRC `need`). Those three are therefore not tested with the same restart and wait rules as the first two. Their stuck states (Waybar or hubd missing after driftwm or seatd dies) are what the missing wait and restart rule would be expected to cause (BELIEVED, not tested). **The drops written above for s6rc are therefore PROVISIONAL, and the dinit and openrc results are "failed as configured", not a fair elimination.** The plan: after the remaining candidates are run, re-run s6rc, dinit and openrc with the same wait and follow rules as s6plain (a new commit, clearly marked as a rerun after a configuration parity fix), unless the owner objects; the original results stay in the repository.

### c-go (s6 + s6-rc with the Go policy brain) elimination unit (results/step2/c-go/)
- TESTED, with the fixed injector: 3 boots 6.3 to 6.7 s (PSS of PID 1, supervisors and brain together 10 962 kB; init-side idle CPU 267 ticks in 30 s, the highest of all); PID 1 killed: back in 20 and 21 s; PID 1 wedged: back in 51 s; trial boots: healthy confirmed 23.3 s; driftwm cannot start: rolled back at 60 s (guard feeding); **driftwm wedged 2 s after it first worked: confirmed after 41.7 s with no false-confirmation note** (the brain's probe restarted it first; the other candidates confirmed falsely or rolled back). Policy-bug test (the brain made to misbehave 5 times): brain replaced within 1.4 s, services kept running (5 of 5).
- Fault suite finished (`suite_done=True`, 2 launches, 880 s): all 18 kills recovered in 0.13 to 8.9 s; a hung driftwm (SIGSTOP or spin) recovered by the probe in 0.13 to 14 s in 6 of 6 (the only candidate so far that notices a hang by itself); hubd crash storm 0.94 s; **driftwm crash storm 64 s** and **supervisor-killed driftwm 66 s** (slow); **1 stuck state** (`STUCK`, step 33, supervisor-killed driftwm: Waybar not running afterwards). Not a clean sheet: two 64 to 66 s recoveries and one stuck state need a look before it counts as a finalist.
- Fairness: c-go uses the s6-rc service definitions without the follow rule, plus the brain (the brain restarts what is missing); so the s6-rc "bare" stuck states did not recur here (BELIEVED because the brain restarts dependents; one still did).

### c-elixir (s6 + s6-rc with the Elixir/OTP policy brain) elimination unit (results/step2/c-elixir/)
- TESTED, fixed injector: 3 boots 6.2 to 6.5 s (PSS 35 574 kB for PID 1, supervisors and the BEAM; init-side idle CPU 234 ticks); PID 1 killed: back in 21 and 22 s; PID 1 wedged: back in 52 s; trial boots: healthy confirmed 28.9 s; driftwm cannot start rolled back at 60 s; **driftwm wedged: confirmed after 32.6 s, no false confirmation** (the probe restarted it). Policy-bug test: 5 of 5, services kept running, brain answering again after 0.7 s.
- Fault suite finished (`suite_done=True`, 3 launches, 1094 s): kills recovered in 0.16 to 11.1 s (a few kills of driftwm/udevd 5.9 to 11 s); hung driftwm recovered by the probe in 10 to 15 s in 6 of 6; hubd storm 1.1 s; **driftwm storm 65 s; supervisor-killed driftwm 66.5 s and 17.8 s**; **2 stuck states** (kill of driftwm at step 10; supervisor-killed driftwm at step 33: Waybar not running afterwards). Same pattern as c-go: the Elixir and the Go brain behave alike in this harness (BELIEVED: the weak point is under both, not in the language).

### b-go (architecture B approximated: s6-svscan as a tiny PID 1, the Go brain starting and watching the services itself) elimination unit (results/step2/b-go/)
- TESTED, fixed injector: 3 boots 7.2 to 7.4 s (PSS 5028 kB; init-side idle CPU 29 ticks); PID 1 killed: back in 18 and 18 s; PID 1 wedged: back in 46 s; trial boots: healthy confirmed 23.5 s; driftwm cannot start rolled back at 60 s; driftwm wedged confirmed after 37.5 s, no false-confirmation note. Policy-bug test 5 of 5 (brain replaced, services kept running).
- Fault suite finished in one launch (457 s), **no stuck state, 38 events all recovered**: kills 0.35 to 6.7 s; hung driftwm recovered in 10.6 to 15 s (SIGSTOP) and 0.13 to 0.16 s (busy loop, noticed at once); crash storms 1.2 and 2.6 s; slow seatd 18.9 s; disk full, brain killed (restarted by s6-svscan), both clock jumps and OOM recovered. No supervisor-killed step (there is no per-service supervisor). **This is a prototype approximation, not an init, and it is a single process that holds every service: its weak points are listed in the soak, not here.**

### runit steps 31 to 43 rerun with the fixed injector (results/step2/runit/runit-faults-rerun-from31.log)
- TESTED: with the `supOf` fix the supervisor-killed steps (31 to 36) ran: **`kill -9` of the per-service supervisor `runsv`: the service kept running in 36 of 36 cases, the supervisor came back in 0.1 to 5.6 s**. Step 37 (kill of the scanner `runsvdir`): **runit's PID 1 restarts the whole of stage 2, so every service and the injector itself are restarted** (9 times in this log, each time the injector starts from its first step again, 5.6 s later). That is a finding for runit (a scanner death restarts the whole desktop; the services come back, but windows would not survive) and also the reason the suite did not get past step 37: the injector is part of the tree it kills. The run hit its 1500 s limit (`suite_done=False`). Steps 38 to 43 (brain-killed, clock jumps, OOM) will be run separately with `--skip 37`.

### s6rc-par (s6 + s6-rc with the same wait and follow rules as s6plain) (results/step2/s6rc-par/) — RERUN after the configuration parity fix
- TESTED: 3 boots 7.6 to 8.0 s; PID 1 killed: back in 21 and 20 s; PID 1 wedged: back in 49 s; trial boots: healthy confirmed 23.1 s, driftwm cannot start rolled back at 60 s, **driftwm wedged CONFIRMED after 22.7 s (false: driftwm stopped)**. Fault suite finished in one launch (818 s), **no stuck state**: kills recovered in 0.2 to 5.1 s; hung driftwm: not noticed (64 to 65 s = the harness fixed it, needs a person) in 5 of 6 (the busy-loop case once 0.10 s); crash storms 1.9 and 5.2 s; supervisor-killed driftwm 18 s then 6 s.
- **Conclusion (TESTED):** the earlier `s6rc` drop (8 stuck states) was caused by my setup, not by s6-rc: with `follow-driftwm` and the same `-wait` rules the stuck states are gone and s6rc-par behaves like s6plain. The provisional drop of s6rc is therefore WITHDRAWN. The remaining weakness is the same as s6plain's: a hung (not dead) compositor is not noticed, and a wedged one is falsely confirmed in a trial boot.

### dinit-par (dinit with the same wait and follow rules as s6plain) (results/step2/dinit-par/) — RERUN after the parity fix
- TESTED: 3 boots 6.0 to 6.1 s; PID 1 killed: back in 15 and 15 s; PID 1 wedged: back in 45 s; trial boots: healthy confirmed 21.2 s, driftwm cannot start rolled back at 60 s, driftwm wedged CONFIRMED after 20.8 s (false). Fault suite: 8 launches, never finished (1913 s), **the same 8 stuck states as the first dinit run** (kill of driftwm, udevd; crash storms of hubd and driftwm; failed seatd): Waybar, driftwm or hubd not running afterwards.
- **Conclusion (TESTED):** the wait and follow rules do NOT remove dinit's stuck states, so they are not caused by the missing rules. BELIEVED cause (SOURCE: dinit-service(5) from the source tarball, read 2026-10-10): the services use `depends-on`, a hard dependency; stopping or restarting a dependency stops its dependents and dinit does not start them again by itself. The manual names `smooth-recovery = true` ("an automatic process restart is performed, without first stopping any dependent services") for this. A variant with `smooth-recovery` on seatd, udevd, dbus and driftwm (`dinit-sm`) is queued as the next test; until it runs, dinit is **failed as configured, fixable by a documented setting: UNKNOWN until tested**.

### openrc-par (OpenRC with supervise-daemon and the same wait and follow rules as s6plain) (results/step2/openrc-par/) — RERUN after the parity fix
- TESTED: 3 boots 9.2 to 9.7 s (the slowest); PID 1 killed: back in 21 and 27 s; PID 1 wedged: back in 51 s; **all three trial boots, the healthy one too, ended in ROLLBACK at 60 s** (`true_state` empty: UNKNOWN whether the harness's trial hook does not work for OpenRC or OpenRC never reports healthy; not counted as an OpenRC result until understood). Fault suite finished (`suite_done=True`, one launch, 1259 s), **no stuck state** (the first openrc run had 2 and recovered nothing after a kill of seatd or driftwm; the parity rules fixed that): kills of the six services 1.2 to 7.3 s.
- **Still needs a person (rec about 60 s = the harness fixed it), TESTED:** `kill -9` of `supervise-daemon` (12 of 12: the service keeps running but nothing restarts the supervisor and the next kill of the service is not noticed), a hung driftwm by SIGSTOP (3 of 3, never noticed), crash storms on hubd (60 s) and driftwm (63 s) (never end in a returned service), a failed seatd (63 s). A busy-looping driftwm was noticed in 0.1 s twice and 15.7 s once (supervise-daemon's health check).

### dinit-sm (dinit-par plus `smooth-recovery = true` on seatd, udevd, dbus, driftwm) (results/step2/dinit-sm/)
- TESTED: the fault suite again never finished (8 launches, 1365 s): **8 stuck states, the same steps as dinit-par for kills (steps 1, 4, 7, 10, 13, 16: seatd or udevd "not running" after a kill) and 2 for a hung driftwm (steps 19 and 21; dinit cancelled the start of driftwm after its start timeout, `[FAILED] driftwm`)**. `smooth-recovery` did NOT remove the stuck states. So the dinit drop stands for the configurations I tried (default, parity, smooth-recovery). UNKNOWN whether another dinit setting (for example `restart` with `depends-ms` instead of `depends-on`) would fix it; I did not try it. Spam lines (`SAMPLE`) stripped from the saved logs.

### runit steps 38 to 43 (results/step2/runit/runit-faults-rerun2-from38.log)
- TESTED: the brain-killed steps (38, 39) produced no event line (runit has no brain: nothing to kill; BELIEVED, not checked in the harness source), clock jumps (steps 40, 41): all recovered in 1.8 to 5.0 s; out-of-memory (steps 42, 43): the memory hog was killed, no service lost, 0.07 s. No stuck state. The runit fault suite is now complete, steps 1 to 43.

### s6plain full repeat (results/step2/s6plain-repeat/) — answer to Q1 (stuck loop)
- TESTED: the full fault suite run again alone, one launch, 695 s, `suite_done=True`, 48 events, no stuck state, **the stuck loop seen in the first s6plain run did NOT recur**. The 3 SIGSTOP hangs of driftwm (steps 19, 21, 23; the harness fixes them after about 65 s, `fixed=1`; this line first said 6, which was wrong) are the same weakness as in the first run: plain s6 does not notice a hung process. Steps 31 to 43 all ran (supervisor-killed, scanner-killed, brain-killed, clocks, OOM).
- **Verdict for Q1 stays UNKNOWN**: one occurrence, not reproduced in the complete repeat of the suite. Cause not found. BELIEVED, not tested: it is not a systematic s6 fault, since the same steps passed twice. Spam lines stripped.

## Soak round (results/step2/soak/) — written 2026-10-11 22:30 UTC
**Method (TESTED, my own code, `tools/bench/init/soakq.sh`, `soak_analyze.py`).** One candidate at a time, two chunks of 7200 s each, a new seed per chunk (6000+), QEMU 8.2.2 without KVM, 2 vCPU, 1 GB. The fault injector picks a random fault every 5 to 20 s (kills 50 %, hangs 10 %, crash storms 5 %, slow/failed dependency 7 %, full disk 4 %, supervisor kill 10 %, scanner kill 2 %, brain kill 4 %, clock jump 3 %, out of memory 5 %). Every 60 s the log gets the PSS (from `/proc/PID/smaps_rollup`) of PID 1, the scanner, every supervisor and the policy layer, and the number of service restarts so far (a service whose PID changed). Analysis: least-squares slope of PSS against restarts (kB per 1000 restarts) with its standard error, first 600 s of each boot left out; a time that goes backwards counts as a reboot. A chunk is stopped early after 2 STUCK states (a state only a reboot clears).
**Pilot (45 min, b-go, `PILOT-b-go.*`).** The Go brain's PSS is a saw-tooth between 3.5 and 7.5 MB (Go garbage collector), noise sd about 0.8 MB around the line; 285 restarts in 2713 s (about 380 per hour). The naive slope (+4.2 to +6.4 MB per 1000 restarts, std. error 1.5 to 2.1) is NOT distinguishable from start-up growth in 45 minutes. From this I set the chunk length to 2 h (about 700 to 830 restarts per chunk, std. error about 0.4 MB per 1000 restarts for the Go brain, 2 to 5 kB per 1000 for plain C supervisors). A leak smaller than about 1 MB per 1000 restarts (1 kB per restart) is invisible for the Go brain. A first pilot attempt was thrown away because the image still held the old `hubsim` binary (no memory log); a c-go pilot after it was stopped early (below).

| Candidate | chunks | stuck states | memory result (slope per 1000 restarts, std. err.) | notes |
|---|---|---|---|---|
| s6plain | 2 x 2 h (607 + 621 restarts) | 1 (chunk 2, after a hung driftwm: the harness asked for operator action, then "clients missing: waybar" stayed; rebooted) | +15.8 (2.4) and +22.2 (5.0) kB, total PSS of the init side 2.4 MB, range 16 to 28 kB | tiny drift, 16 to 22 bytes per restart (BELIEVED not a leak; UNKNOWN, a 4-hour run cannot tell a very slow leak from allocator noise) |
| s6rc-par | 2 x 2 h (655 + 702) | 0 | -1.1 (3.0) and -10.1 (1.9) kB, total 2.7 MB | flat |
| runit | 2 x 2 h | 0 stuck, but **the machine rebooted by itself 1 time (chunk 1) and 28 times (chunk 2) because I killed the scanner `runsvdir`: runit's PID 1 runs stage 3 and reboots (log: "runit: system reboot")** | chunk 1: -44 (66) kB, total 1.7 to 1.9 MB; chunk 2: no segment longer than 600 s, so no slope | each reboot is a service-failure reboot (scorecard I4 asks for none); the machine came back by itself each time in about 15 s |
| b-go | 2 x 2 h (831 + 813) | 0 | +298 (382) and -161 (411) kB for the Go brain, noise sd 1 MB, range 3.2 to 7.6 MB | no growth visible; s6-svscan constant |
| c-go | stopped after 14 min (pilot-like run, 3 launches) | 2 STUCK (supervisor of driftwm killed, "waybar not running"), same step in seeds 5001 and 5002 | not usable (reboots) | `EARLYSTOP-c-go.log` |
| c-elixir | stopped by me after 1333 s + a loop | 1 STUCK (supervisor of driftwm killed), then after the reboot **one kill made all 8 `s6-log` loggers fail to take their log lock about 6000 times each in 6400 s, and the test driver wrote nothing more** | not usable | I stopped the run at 05:50 (the queue had not noticed because the driver was silent, not stuck). Cause not investigated (UNKNOWN). The same state may exist in c-go and s6plain (their loggers are the same programs) but did not show up there |

Honest limits: 4 hours per candidate, two seeds. One run of 2 h is about 750 restarts; 1000-restart slopes are therefore extrapolations by a factor 1.3. The stand-in services are small Go programs, not the real driftwm, waybar, hubd; their own memory is not measured (only the init side). QEMU without KVM is slow, so time-based faults (storms 25 s, hang checks 60 s) are the same in wall time as on real hardware, but CPU-heavy phases take longer. Four candidates were soaked; dinit and OpenRC were not (below). A single `git push` from the queue script failed after a container restart (stale proxy port); every chunk was pushed by hand and checked (`chunk N pushed` lines in the report).

## Step 2 summary (for the owner) — 2026-10-11
**Dropped, and why (all TESTED unless stated):**
- **dinit** (default, `dinit-par` with the same wait/follow rules, `dinit-sm` with `smooth-recovery`): 8 stuck states in the fault suite, the same in all three setups. The wait/follow rules and `smooth-recovery` do not remove them. Fixable by another configuration: UNKNOWN (BELIEVED cause: hard `depends-on`; I did not try `depends-ms`). Definitions: equivalent after the parity fix.
- **s6rc alone**: dropped first, drop WITHDRAWN: the 8 stuck states came from my setup missing the follow rule; `s6rc-par` has none. Definitions were not equivalent before the fix; they are now.
- **OpenRC** (and `openrc-par`): needs a person: a kill of `supervise-daemon` is not noticed (12 of 12), a hung driftwm is never noticed, and every trial boot rolled back (even the healthy one: UNKNOWN whether the harness hook or OpenRC is the cause). Fixable by configuration: UNKNOWN. Not soaked.
- **c-go**: stuck twice within 14 minutes of soak at "supervisor of driftwm killed"; in the elimination round the same fault took 66 s (needs a person). Fixable by configuration: UNKNOWN (the brain is meant to restart what is missing; it did not).
- **c-elixir**: stuck state + logger loop (above). Same open question.
**Finalists that passed the soak:** s6rc-par (clean, flat memory), b-go (clean, memory flat within noise), s6plain (1 stuck state after a hang, a known weakness: s6 does not notice a hung process; flat memory). **runit passed with a reservation**: no stuck state and no leak, but the scanner kill reboots the whole machine.
**Q1 (s6plain stuck loop): UNKNOWN.** One occurrence in the first run, none in the full repeat (695 s, 48 events) nor in 4 hours of soak; the soak's one stuck state (hung driftwm, then waybar missing) is a different state. **Q2:** no result counted was taken while two VMs ran (one 20 s overlap of diagnostic reruns, discarded and rerun alone). **Q3:** see the dropped list above (fixable by configuration / equivalence of definitions).
**What could not be tested:** the real driftwm, waybar and hubd (stand-ins only); real hardware, KVM speed, real watchdog behaviour; leaks below about 1 kB per restart in the Go brain and below the 4-hour horizon for all; whether dinit/OpenRC/c-go/c-elixir have a configuration that passes; real power loss and disk corruption; memory use by the services themselves. I did not decide whether s6 stays the design (owner order).

---

# Follow-up (2026-10-11 to 2026-10-12): data only

Order from the owner: items 1 to 6. No init was written, no winner is named, whether s6 stays is not decided. Item 1 is the provenance table at the top of this file. Every number below comes from a log in `tools/bench/init/results/step2/` (the raw tables of the whole elimination round, regenerated with `report.py`, are in `results/step2/FINAL-TABLES.md`).

## Item 2: one more fair try for dinit and OpenRC

### dinit (`candidates/dinit-ms/`, logs `results/step2/dinit-ms/`)
**Definitions** (`candidates/dinit-ms/rootfs/etc/dinit.d/{boot,guard,seatd,udevd,dbus,driftwm,waybar,hubd}`). Compared with `dinit-par`: every `depends-on` became `depends-ms` (the change asked for), and every service got `restart-limit-count = 0` and `restart-delay = 1.0`. The second change is the "equivalent restart limits" part: SOURCE (`dinit-service(5)`, dinit 0.21.0): the default is `restart-limit-count = 3` in `restart-limit-interval = 10` seconds, "specify 0 to disable"; SOURCE (`s6-supervise` page of s6 2.15.1.0): s6 restarts at once if the service was ready for more than 1 s, otherwise after a 1-second pause, with no limit. Unchanged: the commands, the `-wait` arguments, the `follow-driftwm` wrapper, the `policy` settings (dinit uses its own `oom-score-adj`, `nice`, `run-in-cgroup` keys where s6rc-par uses the `policy` wrapper).
**Did the equal-definition rule hold?** For the graph (the same six edges, now milestone edges, which matches s6-rc, where a dependency only orders the start and never stops a dependent), the restart limit and delay, and the follow rules: yes. Not identical and not changeable by me: logging (dinit `log-type = buffer` against s6-log files), how the resource settings are applied (native keys against the wrapper), and what a failed start does (see below).
**Why the first dinit runs got stuck (TESTED, `results/step2/dinit-par/dinit-par-faults-1.log` line about 90):** `dinit: Service waybar restarting too quickly; stopping.` followed by `[STOPPD] waybar`, i.e. the default restart limit gave up on waybar after a driftwm kill had made it restart three times in 10 s. That is a setting I did not make equal; it was not `depends-on` alone.
**Result (TESTED), elimination round, 6 launches, suite finished:** boots 6.1 to 6.3 s; PID 1 killed back in 16 s (twice); wedged 46 s; trial boots: healthy confirmed 21.4 s, driftwm cannot start rolled back at 60 s, driftwm wedged CONFIRMED after 21.2 s (false: driftwm stopped). **5 stuck states** (before: 8): step 20 (hung driftwm, spin: hubd not running), 25 (crash storm on hubd), 26 (crash storm on driftwm), 27 (seatd slow), 28 (seatd fails to start). All 18 plain kills now recover (median 1.8 s, worst 3.9 s), which they did not before. Hung driftwm by SIGSTOP: not noticed, 3 of 3 needed a person.
**Why the five remain (BELIEVED, from the log lines and the pages named; not tested separately):** (a) after a crash storm the log shows `[FAILED] hubd` and then hubd stays stopped. `dinit-service(5)` says of a milestone dependency: if the dependency "fails to start or has its startup cancelled, the dependent will not start (and will return to the stopped state)", and `dinit(8)` says a service that fails to start is not run again by itself. (b) step 28: the next seatd start exits with code 1 (the fault), `[FAILED] seatd`, and driftwm, waybar and hubd stop with it; dinit's `restart` property is about a service that stops, not about a start that fails, and I found no property in `dinit-service(5)` that retries a failed start. s6 retries because its run script is simply started again. (c) `waits-for` is the documented relationship that lets a dependent start even if the dependency failed; I did **not** run it, because the instruction was one attempt with `depends-ms`. Question for the owner below.
Since it did not pass, it was not soaked.

### OpenRC (`candidates/openrc-doc/` and `openrc-doc2/`, logs `results/step2/openrc-doc/`, `openrc-doc2/`)
Only options from OpenRC 0.63.3's own pages (SOURCE: the pages in the source tree `man/supervise-daemon.8`, `man/openrc-run.8`, `man/rc-status.8`, `supervise-daemon-guide.md`, `user-guide.md`). **Definitions:** `candidates/openrc-doc/rootfs/etc/init.d/{seatd,udevd,dbus,driftwm,waybar,hubd}` and `etc/rc.conf`. Changes against `openrc-par` (the only ones): `respawn_max=0` in place of `respawn_max=10`/`respawn_period=60` ("0 means unlimited", `supervise-daemon.8`; matches s6, which has no limit and a 1-second pause, `respawn_delay=1` was already there), and `retry="SIGTERM/5/SIGKILL/5"` (`openrc-run.8`: "retry schedule to use when stopping the daemon... multiple signal/timeout pairs"). The `depend()` lines (`need ...`), the healthcheck on driftwm (`healthcheck_timer=2`, `healthcheck_delay=10`, `healthcheck()` calling the stand-in's `hubsim probe`, which was already in `openrc-par`), the wrapper commands and the follow rules are unchanged. Equal-definition rule: held for restart limit/delay, follow and wait rules; **not** identical: OpenRC's `need` is a hard dependency in its model and I did not change it; logging differs (`output_logger` runs s6-log, the same program as the s6 candidates).

| Finding in the first run | Documented option tried | Result (TESTED) |
|---|---|---|
| A killed `supervise-daemon` is not noticed (12 of 12) | **No documented option exists.** `user-guide.md` ("Dealing with Orphaned Processes") names the case, "if you are monitoring a service with supervise-daemon and supervise-daemon dies for an unknown reason", and only offers cleaning up orphans afterwards (`rc_cgroup_cleanup`, `cgroup_cleanup`); `rc-status -c` only lists crashed services. Nothing restarts the supervisor. | Unchanged: 6 of 6 follow-up kills of the service after its supervisor was killed needed a person (the other 6 steps of that fault recover). I did not try the cgroup option (it cleans up, it does not restart; and the image has cgroups off). |
| A hung driftwm (SIGSTOP) is never noticed | The `healthcheck_*` options were already set; the missing piece was the stop schedule. `supervise-daemon.8` says the default retry is `SIGTERM/5` (SIGTERM only); a stopped process does not act on SIGTERM. Option: `retry="SIGTERM/5/SIGKILL/5"`. SOURCE `supervise-daemon.c` lines 724 to 745: one failed health check stops the process and respawns it. | **Fixed:** SIGSTOP of driftwm recovered in 20.1 s median (19.4 to 20.2), 3 of 3, nobody needed. Spin: 0.1 s (the probe still answers) and 20 s. |
| Every trial boot rolled back | Not an OpenRC finding: the new line in the rollback message (`health_checks_run=15 slowest_check=4.3s consecutive_good=11 (15 needed)`) shows that my test hook `/ops/healthy` made six `rc-service ... status` calls that took 3 to 4 s each under QEMU without KVM, so only 15 checks fitted into 60 s and 15 good in a row were never reached, although all six services were started. With one `rc-status -C -i started` call (documented in `rc-status.8`) in `openrc-doc2` (only that file differs): | healthy CONFIRMED after 27.0 s; driftwm cannot start rolled back at 60 s; driftwm wedged CONFIRMED after 27.1 s with `true_state="driftwm not running"`, i.e. a false confirmation (the service was being restarted by the health check at that moment). |

**Result (TESTED), `openrc-doc` elimination round, 1 launch, suite finished, no stuck state:** boots 8.9 s; PID 1 killed back in 20 s (twice); wedged 50 s; all 18 kills recovered (median 2.8 s, worst 17.7 s); crash storms and slow or failed seatd recovered by themselves (before: 2 and 1 needed a person); 6 of 48 events needed a person, all the same fault (a service killed after its supervisor was killed). Not soaked, as instructed. Whether this counts as "passed" is for the owner: no state needed a reboot, one fault class (a killed supervisor) still needs a person.

## Item 3: leak slope with an interval (existing soak logs; `soak_ci.py`, output `results/step2/soak/SLOPES-with-intervals.txt`)
Method, one line: per stretch between reboots, the first 600 s left out, ordinary least squares of the total PSS (PID 1, scanner, all supervisors, policy layer) against the number of service restarts; the standard error widened for the correlation between neighbouring samples (effective sample count n(1-r)/(1+r), r = lag-1 autocorrelation of the residuals); stretches combined by inverse variance; 95 % interval from a t table. Restarts = services whose PID changed (counted inside the VM).

| Candidate | stretches used | restarts counted | PSS at start, end (kB; mean of first and last 5 samples) | slope, kB per 1000 restarts | 95 % interval | noise (sd of residuals) |
|---|---|---|---|---|---|---|
| s6rc-par | 2 | 1357 | 2687, 2681 | -8.6 | -19.5 to +2.2 | 4.9 kB |
| s6plain | 3 | 1228 | 2457, 2464 | +20.9 | +1.7 to +40.0 | 3.6 kB |
| runit | 2 (chunk 1 only) | 646 | 1900, 1879 | -70.4 | -168.9 to +28.1 | 23.7 kB |
| b-go (the Go brain) | 2 | 1644 | 6242, 6948 | +99.9 | -1084 to +1284 | 992.5 kB |

Reading (no recommendation): s6plain's interval lies above zero by a hair (2 to 40 bytes per restart; total change over 1228 restarts: +7 kB); s6rc-par and runit include zero; b-go's interval is wide because the Go heap moves by about a megabyte (the garbage collector), so a leak below about 1 MB per 1000 restarts (1 kB per restart) is invisible there. Runit's second chunk gave no usable stretch (the machine rebooted itself about every 4 minutes, see item 5). The logs were enough; no re-run is proposed except, if the owner wants a tighter interval for b-go, a longer chunk (the interval narrows with the 1.5th power of the length).

## Item 4: the hung compositor (SIGSTOP of driftwm) on every surviving candidate
The same fault (`hang-stop`, step 19, 21, 23 of the suite, 3 per run; for runit 6) in the elimination round of each candidate, same injector. "Recovery" is how long until the harness sees the compositor answering; "needed a person" means nothing happened for 60 s and the harness then killed/restarted it (the harness's own operator action). SIGSTOP was used for all (no other method needed). Spin (busy loop) results are in `FINAL-TABLES.md`; with some seeds the compositor kept answering during the spin (0.1 s), so only SIGSTOP is compared here.

| Candidate | Noticed by itself? | After | What it does | Stuck state if it does nothing (TESTED, STATUS lines in the logs) |
|---|---|---|---|---|
| s6plain | no (3 of 3) | - | nothing | supervisor reports `up (pid N) ...` for driftwm for as long as you look; the process is in state T; hubd and waybar stay up and keep serving a frozen picture |
| s6rc-par | no (3 of 3) | - | nothing | same: `driftwm: up (pid 1508 ...) 77 seconds, ready 75 seconds` |
| runit | no (6 of 6) | - | nothing | `run: /run/service/driftwm/: (pid 1462) 77s`; same |
| b-go | yes (3 of 3) | 10.6 to 15.1 s | the brain's probe gets no answer 3 times in a row (grace 8 s after a start), kills driftwm (SIGKILL), holds hubd and waybar until driftwm is ready again, starts them (log lines `brain: driftwm failed (health probe: no answer)`, `holding hubd`, `starting hubd`) | - |
| c-go | yes (3 of 3) | 9.3 to 13.7 s | the same brain commands s6 (`s6-svc -k`) | - |
| c-elixir | yes (3 of 3) | 14.1 to 15.4 s | the same, brain in Elixir | - |
| dinit-ms | no (3 of 3) | - | nothing | dinit lists driftwm as started; process in state T |
| openrc-doc | yes (3 of 3) | 19.4 to 20.2 s | `supervise-daemon` runs `healthcheck()` every 2 s (`hubsim probe`, 1 s timeout); a failure runs `unhealthy()`, stops the process with the `retry` schedule (TERM, then KILL after 5 s) and respawns it | - |
| openrc-par | no (3 of 3) | - | same health check, but the default stop schedule is TERM only, which a stopped process ignores | `rc-status` shows `driftwm started`; process in state T |
| dinit-par, openrc (first runs) | no | - | - | as above |

**Where the hang rule (kill after 4 failed rounds and 30 s with both probes failing, at most 3 kills in 10 minutes; HUB-OS.md, not built) COULD live, and what each placement needs that the candidate lacks.** Options and facts only.
- **s6plain, s6rc-par (s6 `run` scripts; the same facts apply):** (1) a new long-running program as its own service that probes and kills with `s6-svc -k` (documented in the s6 pages): needs the program, two probes, a round counter and a kill-rate memory (s6 offers none of these; `s6-supervise` only restarts after a death); (2) inside the driftwm `run` script as a wrapper that does not `exec` driftwm: needs a parent that stays alive next to the compositor; s6-supervise would then watch the wrapper and not the compositor; (3) inside hubd: needs hubd to see the compositor's process ID, to be allowed to signal it (same user), and a decision about what happens when hubd itself is down.
- **runit:** the same three, with `sv kill`/`sv force-restart` instead of `s6-svc`; `runsv` has no probe, no counter. Additional fact (SOURCE, `sv(8)`): a `./check` script in the service folder is run by `sv` when a command asks for it, not on a timer.
- **dinit-ms:** `dinitctl restart driftwm` or a signal; no probe, no timer, no counter in the init; the same three placements.
- **b-go, c-go, c-elixir (the brain):** the brain already has the probe (one probe, answer within 1 s, 3 failures in a row, 8 s grace), the kill, and a ladder with a window (60 s) and give-up counts (bench values: 6 failures in 60 s). What it lacks against the rule as written: a second probe, the "4 rounds and 30 s" timing, the "3 kills in 10 minutes" memory (the ladder counts all failures in 60 s, not kills in 10 minutes), and a way to make "stop retrying and show a fixed message" survive a brain restart (the brain keeps its state in memory; a killed brain starts with an empty record; TESTED: brain-killed events recovered in 0.1 to 0.2 s with services untouched).
- **OpenRC:** the probe and the kill already exist as options (`healthcheck_timer`, `healthcheck()`, `unhealthy()`, `retry`). What is missing: the supervisor acts on the first failed check (SOURCE `supervise-daemon.c` 724 to 745: no count of failed rounds and no 30-second condition; the check function could keep a counter in a file); and kills by health check count as respawns toward `respawn_max`/`respawn_period` (SOURCE, lines 765 to 785), so `respawn_max=3` with `respawn_period=600` would approximate "at most 3 in 10 minutes", but it counts crashes too, and when the limit is exceeded `supervise-daemon` exits and the service stays down (a documented give-up, `supervise-daemon.8`), which is close to "stop restarting" but shows nothing by itself.

## Item 5: why killing runit's `runsvdir` reboots the machine
**Cause, SOURCE (runit 2.1.2 upstream source, `src/runit.c` lines 175 to 199, downloaded from https://smarden.org/runit/ for reading only; `runit(8)` of the Ubuntu package 2.1.2-59ubuntu1):** `runit(8)` says stage 2 "should not return until system shutdown; if it crashes, or exits 111, it will be restarted". The code tests `if (wait_exitcode(wstat) != 0)` first and only inside that branch asks `wait_crashed(wstat)`. `wait.h` defines `wait_exitcode(w)` as `(w) >> 8` and `wait_crashed(w)` as `(w) & 127`. A process killed by a signal has `(w) >> 8 == 0`, so the whole block is skipped: the page's "crashes" case is not reached for a death by signal, runit logs `leave stage: /etc/runit/2` and goes to stage 3. (This reading of the code matches the log: `runit: leave stage: /etc/runit/2 / enter stage: /etc/runit/3`, `results/step2/soak/runit-chunk1.log` at 4617 s.) In my image stage 2 is `exec runsvdir -P /run/service` (`candidates/runit/rootfs/etc/runit/2`), so `runsvdir` *is* stage 2 and `kill -9 runsvdir` is a signal death of stage 2. **After stage 3 runit reboots or halts** depending on `/run/runit.reboot` (`runit(8)`; `runit.c` lines 296 to 332: executable file present = reboot, else power off). **That file is made by my test image** (`candidates/runit/rootfs/cand/boot.sh`: `touch /run/runit.reboot; chmod +x`, with the comment that it mimics the s6 images' `finish` script that reboots). Without it the VM would power off (also a stop of the machine). So: the leave of stage 2 is runit's own handling of a stage that dies by a signal (SOURCE), and the reboot (instead of power-off) is the test image's setup. It is not a defect of the harness's PID 1 setup beyond that file.
**Another documented way (SOURCE `useinit.html` in the package's doc folder):** run runit's stage 2 "as a service with your init", e.g. `SV:123456:respawn:/sbin/runsvdir-start` in `/etc/inittab` (sysvinit) or an upstart `respawn` stanza, so that another init respawns `runsvdir`. That needs a different PID 1 with respawn (sysvinit or upstart; neither is in the candidate list; BELIEVED that `s6-svscan` supervising `runsvdir-start` would also work, the pages do not describe it). The page's own description of stage 2 also suggests that a stage 2 *script* (not `exec`) that exits 111 when `runsvdir` dies would be restarted by runit (`runit.c` line 190: `wait_exitcode(wstat) == 111`). **Not tested:** any of these, because the instruction was not to change runit's definition. **Measured effect of the present setup (TESTED):** 29 reboots of the VM by runit itself in the two soak chunks (1 in chunk 1, 28 in chunk 2), after scanner kills, each of the VM within about 15 s, all services restarted, no stuck state.

## Item 6: tests of the earlier build tasks (run 2026-10-11, `nice -n 19`, `go test -p 1`, no VM running)
| PR | What | Tests run (TESTED) | Result | Not tested / why |
|---|---|---|---|---|
| #79 source mirror (`tools/mirror`) | the 8 tests of the proposal plus a signature test | `go vet`, `go test -count=1 -v ./tools/mirror/...`; then by hand: a local web server (127.0.0.1) and the real binary: `fetch` (ok), `path`, `verify`, a file corrupted by hand, `path` after it | 10 of 10 tests pass; by hand all as designed. **One thing to know:** `fetch` of an entry that is already stored, with a different expected hash, reports "stored file has sha256 ..., expected ...: marked corrupt" and marks the good stored file corrupt, after which `path` refuses it (exit 1) until it is repaired; a typo in a repeat `fetch` therefore poisons an entry. I did not change it (no features). | a real internet source, a real upstream signature key, the retention rule over time, the alert path to the bar (not built: a status file and a non-zero exit only), an off-site copy (decided later) |
| #80 generated inventory (`internal/machinefile`, `internal/inventory/diff.go`, `cmd/hubd/reload.go`, `tools/invgen`) | generator, migration script, validation, `hubd reload --dry-run` | `go vet ./...`; `go test` for `internal/machinefile` (7 tests incl. no guest chains), `internal/inventory` (diff and the older validation tests, all pass), `cmd/hubd` reload tests (3: added node, malformed file, refuses without `--dry-run`); the whole repository `go test -p 1 ./...` (all packages ok, 55 s); by hand: `invgen migrate` of `examples/inventory.example.toml` (5 machines, 4 notes that friendly names become comments), `invgen generate`, `hubd reload --dry-run` of the result against the original | all pass; the round trip differs from the original only in the friendly `name` field of 4 machines (reported as `changed: ... (name)`), which is the documented loss | live reload (not built; `hubd` reads the inventory once), the generated file on a real hub image, 100 and 5000 machine files, a machine file with a guest of a guest on a real hub |
| #78 eight proposals | documents only | none possible (no code) | - | the proposals are not implemented; nothing to run |

## Final scorecard table (all candidates, elimination round and soak; no ranking)
Source: `results/step2/FINAL-TABLES.md` (generated by `report.py` from the logs) and the soak files. "Needed a person" = events where nothing recovered in 60 s until the harness acted. "Stuck" = the harness had to reboot (state only a reboot clears).

| Candidate | Elimination: events (needed a person) | Stuck states | Hung compositor (SIGSTOP) | Soak (2 x 2 h) | Notes |
|---|---|---|---|---|---|
| s6plain (repeat run) | 48 (3) | 0 | not noticed | no leak seen (+21 kB per 1000 restarts, interval +1.7 to +40); 1 stuck state after a hung driftwm | |
| s6rc-par | 48 (5) | 0 | not noticed | flat (-9, interval -20 to +2) | |
| runit | first run steps 1 to 30: 73 events (9); steps 31 to 43 rerun: 0 needed | 0 | not noticed | no leak seen; scanner kill reboots the machine (29 times) | |
| b-go | 38 (0) | 0 | noticed 10.6 to 15.1 s | no stuck state; memory noisy, no trend visible | brain over s6-svscan, not an init |
| c-go | 48 (2) | 1 | noticed 9.3 to 13.7 s | stopped after 14 min: 2 stuck states (supervisor of driftwm killed) | |
| c-elixir | 47 (2) | 2 | noticed 14.1 to 15.4 s | stopped: 1 stuck state, then a logger loop | |
| dinit-par | 20 (4) | 8 | not noticed | not run | default restart limit gave up on waybar |
| dinit-sm | 13 (0) | 8 | not noticed | not run | |
| dinit-ms | 31 (3) | 5 | not noticed | not run | storms and failed starts leave dependents stopped |
| openrc-par | 48 (12) | 0 | not noticed | not run | stop schedule TERM only |
| openrc-doc | 48 (6) | 0 | noticed 19.4 to 20.2 s | not run (not soaked as instructed) | a killed supervisor still needs a person |
(The first runs of `s6rc`, `dinit` and `openrc` are in the run log above and in `FINAL-TABLES.md`; their setups were not equivalent.)

## What I could not test (this follow-up)
Real hardware and KVM timing; the real compositor and viewers (stand-ins only); `waits-for` for dinit; the documented and undocumented alternatives for runit's stage 2 (item 5); whether `supervise-daemon` in a long soak keeps its memory flat (OpenRC was not soaked); a re-run of Item 2 with another seed; the second probe the hang rule needs.

## Questions for the owner
1. dinit: run `waits-for` (the documented relationship that lets a dependent start after its dependency failed) in place of `depends-ms`? I did not, to keep to one attempt.
2. OpenRC (`openrc-doc`): no state needed a reboot, but a killed `supervise-daemon` still needs a person. Does that count as passed (then it would be soaked) or not?
3. runit: should a variant with a stage-2 script that exits 111 (or runit under another supervision) be run, given that the stock stage 2 turns a `runsvdir` death into a reboot?
4. The hang rule: which of the placements above do you want me to cost out further?
5. Mirror tool: should a repeat `fetch` with a different hash really mark the stored file corrupt, or only report a mismatch?


---

# Follow-up 2 (2026-10-12): data only

Order from the owner: items 1 to 9. No init written, no winner named, whether s6 stays is not decided. New scripts (all under `tools/bench/init/`): `event_counts.py` (item 1), `needed_person.py` (item 2), `soak_ci.py` (intervals), and `report.py` now merges the logs of one candidate and counts a step once (see item 1).

## Item 1: why the event counts differ (no rerun; computed from the logs by `event_counts.py`)
**What the script is (SOURCE `hubsim/chaos.go`, `faultSuite`).** 43 numbered steps: 1 to 18 plain kills (3 x 6 services), 19 to 24 hangs (SIGSTOP and busy loop, 3 each), 25 to 26 crash storms, 27 to 28 slow and failing seatd, 29 to 30 full disk, 31 to 36 supervisor kills (2 events each), 37 scanner kill (2 events), 38 to 39 brain kills, 40 to 41 clock jumps (2 events each), 42 to 43 out of memory. **The event list is not the same for every candidate, by design:** a step that needs something the candidate does not have gives no event. Supervisor kills (12 events) exist only for candidates with a separate supervisor process per service (s6, runit, OpenRC; not dinit, not b-go, whose brain starts the services itself). The scanner kill (2) exists only for runit (`runsvdir`). The brain kills (2) exist only where there is a brain (b-go, c-go, c-elixir). So the full script gives **36 events to dinit, 38 to b-go, 48 to s6plain, s6rc and OpenRC, 50 to c-go, c-elixir and runit.** The 48s are not "most candidates are right and the others are short": they are the ones with supervisors and no scanner or brain.
**Why a run ended early or lost events (the table):** a step that ends in a stuck state gives no event (the machine is rebooted and the run goes on at the next step), so each stuck state costs the events of its step; the driver relaunches the machine at most 8 times, so a run with 8 stuck states stops there and the later steps are never reached (dinit, dinit-par, dinit-sm, s6rc: stopped by the relaunch limit at step 21 to 28); one run (the first s6plain) stopped in the supervisor-kill step in the unexplained loop and ran into its time limit (step 34, 10 events never reached; superseded by the full repeat); runit's counts need a note (below). No driver crash and no infrastructure failure ended a run in this table, except runit's first run, where the driver itself crashed (a Go panic in `supOf`, fixed, item "runit" in the older sections).
**runit.** Its raw logs held more events than the script defines (73, then 183 when merged) because the machine rebooted itself (the scanner kill, step 37) and the driver started again from step 1 or from the same skip value, so steps repeated. `event_counts.py` and `report.py` now count a step once, from the first cycle in which it appears. Result: 50 defined, 48 ran; the 2 missing are the two events of step 37 (scanner kill) in the original runit: the machine rebooted before the harness could log them. The "9 events that needed a person" in my last message were counted over the repeated cycles; the correct number is 4. The same repair was applied to every table below.
**Are the counts comparable?** Not as totals. The 36 events every candidate has (steps 1 to 30 and 40 to 43) are comparable; the last column gives, for each candidate, how many of them ran, how many needed a person and how many ended in a stuck state. For the other 2 to 14 events (supervisor, scanner, brain) only the candidates that have the part can be compared with each other.

| candidate | events the script defines for it | events that ran (EV lines) | lost to stuck states | never reached | other difference | stuck steps (fault) | last step reached | launches | script finished | of the 36 common events: ran / needed a person / stuck |
|---|---|---|---|---|---|---|---|---|---|---|
| s6plain | 48 | 48 | 0 | 0 | 0 | - | 43 | 1 | yes | 36 / 3 / 0 |
| s6rc-par | 48 | 48 | 0 | 0 | 0 | - | 43 | 1 | yes | 36 / 5 / 0 |
| runit | 50 | 48 | 0 | 0 | 2 | - | 43 | 3 | yes | 36 / 4 / 0 |
| runit-111 | 50 | 50 | 0 | 0 | 0 | - | 43 | 1 | yes | 36 / 7 / 0 |
| b-go | 38 | 38 | 0 | 0 | 0 | - | 43 | 1 | yes | 36 / 0 / 0 |
| c-go | 50 | 48 | 2 | 0 | 0 | 33 (supervisor-killed) | 43 | 2 | yes | 36 / 1 / 0 |
| c-elixir | 50 | 47 | 3 | 0 | 0 | 10 (kill), 33 (supervisor-killed) | 43 | 3 | yes | 35 / 1 / 1 |
| dinit-par | 36 | 20 | 8 | 8 | 0 | 4 (kill), 8 (kill), 10 (kill), 14 (kill), 16 (kill), 25 (storm), 26 (storm), 28 (dep-fail) | 28 | 8 | no | 20 / 4 / 8 |
| dinit-sm | 36 | 13 | 8 | 15 | 0 | 1 (kill), 4 (kill), 7 (kill), 10 (kill), 13 (kill), 16 (kill), 19 (hang-stop), 21 (hang-stop) | 21 | 8 | no | 13 / 0 / 8 |
| dinit-ms | 36 | 31 | 5 | 0 | 0 | 20 (hang-spin), 25 (storm), 26 (storm), 27 (dep-slow), 28 (dep-fail) | 43 | 6 | yes | 31 / 3 / 5 |
| dinit-wf | 36 | 33 | 3 | 0 | 0 | 25 (storm), 26 (storm), 28 (dep-fail) | 43 | 4 | yes | 33 / 3 / 3 |
| openrc-par | 48 | 48 | 0 | 0 | 0 | - | 43 | 1 | yes | 36 / 6 / 0 |
| openrc-doc | 48 | 48 | 0 | 0 | 0 | - | 43 | 1 | yes | 36 / 0 / 0 |
| s6rc@first | 48 | 19 | 8 | 21 | 0 | 4 (kill), 7 (kill), 13 (kill), 19 (hang-stop), 21 (hang-stop), 23 (hang-stop), 26 (storm), 27 (dep-slow) | 27 | 8 | no | 19 / 3 / 8 |
| dinit@first | 36 | 20 | 8 | 8 | 0 | 3 (kill), 8 (kill), 10 (kill), 14 (kill), 16 (kill), 25 (storm), 26 (storm), 28 (dep-fail) | 28 | 8 | no | 20 / 6 / 8 |
| openrc@first | 48 | 46 | 2 | 0 | 0 | 26 (storm), 28 (dep-fail) | 43 | 3 | yes | 34 / 14 / 2 |
| s6plain@first | 48 | 37 | 0 | 10 | 1 | - | 34 | 1 | no | 30 / 6 / 0 |

(`@first` = the first run of a candidate whose setup I later changed; kept for the record. "Lost to stuck states" = events of steps that ended in a stuck state. Negative or non-zero "other difference" is explained in the paragraph on runit.)

## Item 2: what "needed a person" means (computed by `needed_person.py`)
An event is counted as "needed a person" when the machine had not recovered by itself 60 seconds after the fault and the harness then did, in the person's place, the action in the second table (the candidate's `ops/fix`, a command the owner would type), after which the harness recorded `fixed=1`. It is **not** a stuck state (a state only a reboot clears): every row below was cleared by that one command. A stuck state is a different, worse class and is listed in item 1. The tables give every event of every candidate that has such events (s6plain = the full repeat; runit = canonical cycles).

| candidate | step | fault | target | state at 60 s | alert file (1 = the policy layer gave up on purpose) | other facts from the log |
|---|---|---|---|---|---|---|
| s6plain | 19 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| s6plain | 21 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| s6plain | 23 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| s6rc-par | 19 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| s6rc-par | 21 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| s6rc-par | 22 | hang-spin | driftwm | driftwm does not answer (no answer) | 0 | fixed=1 |
| s6rc-par | 23 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| s6rc-par | 24 | hang-spin | driftwm | driftwm does not answer (no answer) | 0 | fixed=1 |
| runit | 19 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| runit | 21 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| runit | 22 | hang-spin | driftwm | driftwm does not answer (no answer) | 0 | fixed=1 |
| runit | 23 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| runit-111 | 19 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| runit-111 | 20 | hang-spin | driftwm | driftwm does not answer (no answer) | 0 | fixed=1 |
| runit-111 | 21 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| runit-111 | 22 | hang-spin | driftwm | driftwm does not answer (no answer) | 0 | fixed=1 |
| runit-111 | 23 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| runit-111 | 24 | hang-spin | driftwm | driftwm does not answer (no answer) | 0 | fixed=1 |
| runit-111 | 40 | after-clock-3600 | hubd | clients missing: ok clients=waybar | 0 | fixed=1 |
| c-go | 26 | storm | driftwm | driftwm not running | 1 | fixed=1 starts=11 last10=2 cpu=318 bar-down |
| c-go | 31 | supervisor-killed | driftwm | waybar not running | 1 | fixed=1 child_kept_running=1 |
| c-elixir | 26 | storm | driftwm | driftwm not running | 1 | fixed=1 starts=10 last10=1 cpu=338 bar-down |
| c-elixir | 31 | supervisor-killed | driftwm | waybar not running | 1 | fixed=1 child_kept_running=1 |
| dinit-par | 19 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| dinit-par | 21 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| dinit-par | 22 | hang-spin | driftwm | driftwm does not answer (no answer) | 0 | fixed=1 |
| dinit-par | 23 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| dinit-ms | 19 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| dinit-ms | 21 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| dinit-ms | 23 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| dinit-wf | 19 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| dinit-wf | 21 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| dinit-wf | 23 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| openrc-par | 19 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| openrc-par | 21 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| openrc-par | 23 | hang-stop | driftwm | driftwm stopped | 0 | fixed=1 |
| openrc-par | 25 | storm | hubd | hubd not running | 0 | fixed=1 starts=8 last10=0 cpu=1 bar-up |
| openrc-par | 26 | storm | driftwm | driftwm not running | 0 | fixed=1 starts=11 last10=0 cpu=2 bar-down |
| openrc-par | 28 | dep-fail | seatd | seatd not running | 0 | fixed=1 |
| openrc-par | 31 | after-supervisor-killed | driftwm | driftwm not running | 0 | fixed=1 supervisor_back=1 |
| openrc-par | 32 | after-supervisor-killed | waybar | waybar not running | 0 | fixed=1 supervisor_back=0 |
| openrc-par | 33 | after-supervisor-killed | driftwm | driftwm not running | 0 | fixed=1 supervisor_back=1 |
| openrc-par | 34 | after-supervisor-killed | waybar | waybar not running | 0 | fixed=1 supervisor_back=0 |
| openrc-par | 35 | after-supervisor-killed | driftwm | driftwm not running | 0 | fixed=1 supervisor_back=1 |
| openrc-par | 36 | after-supervisor-killed | waybar | waybar not running | 0 | fixed=1 supervisor_back=0 |
| openrc-doc | 31 | after-supervisor-killed | driftwm | driftwm not running | 0 | fixed=1 supervisor_back=1 |
| openrc-doc | 32 | after-supervisor-killed | waybar | waybar not running | 0 | fixed=1 supervisor_back=0 |
| openrc-doc | 33 | after-supervisor-killed | driftwm | waybar not running | 0 | fixed=1 supervisor_back=1 |
| openrc-doc | 34 | after-supervisor-killed | waybar | waybar not running | 0 | fixed=1 supervisor_back=0 |
| openrc-doc | 35 | after-supervisor-killed | driftwm | driftwm not running | 0 | fixed=1 supervisor_back=1 |
| openrc-doc | 36 | after-supervisor-killed | waybar | waybar not running | 0 | fixed=1 supervisor_back=0 |

What the harness did in the person's place (same for all rows of a candidate):

| candidate | command |
|---|---|
| s6plain | `s6-svc -u /run/service/$1 2>/dev/null; s6-svc -k /run/service/$1 2>/dev/null` |
| s6rc-par | `s6-rc -l /run/s6-rc -u change top 2>/dev/null; s6-svc -u /run/service/$1 2>/dev/null; s6-svc -r /run/service/$1 2>/dev/null` |
| runit | `sv -w 3 force-restart /run/service/$1 2>/dev/null; sv up /run/service/$1` |
| runit-111 | `sv -w 3 force-restart /run/service/$1 2>/dev/null; sv up /run/service/$1` |
| c-go | `hubsim ctl /run/hubos/brain.sock "retry $1" 2>/dev/null; s6-rc -l /run/s6-rc -u change top 2>/dev/null; s6-svc -u /run/service/$1 2>/dev/null` |
| c-elixir | `hubsim ctl /run/hubos/brain.sock "retry $1" 2>/dev/null; s6-rc -l /run/s6-rc -u change top 2>/dev/null; s6-svc -u /run/service/$1 2>/dev/null` |
| dinit-par | `/opt/dinit/bin/dinitctl start boot 2>/dev/null; /opt/dinit/bin/dinitctl restart $1 2>/dev/null` |
| dinit-ms | `/opt/dinit/bin/dinitctl start boot 2>/dev/null; /opt/dinit/bin/dinitctl restart $1 2>/dev/null` |
| dinit-wf | `/opt/dinit/bin/dinitctl start boot 2>/dev/null; /opt/dinit/bin/dinitctl restart $1 2>/dev/null` |
| openrc-par | `rc-service $1 restart 2>/dev/null; openrc default 2>/dev/null` |
| openrc-doc | `rc-service $1 restart 2>/dev/null; openrc default 2>/dev/null` |

**Classes, and which candidates share them (TESTED):**
- **H, a frozen compositor whose supervisor still says "up" (SIGSTOP, or a busy loop that stops answering):** s6plain 3 (all SIGSTOP), s6rc-par 5, runit 4, runit-111 6, dinit-par 4, dinit-ms 3, dinit-wf 3, openrc-par 3 (SIGSTOP). Same class, same machine state afterwards: the compositor process is in state T (or loops), the supervisor lists it as up, hubd and waybar keep running against a frozen picture; a person would run the restart command above. OpenRC with a stop schedule that ends in SIGKILL (openrc-doc) has none of these. The three brain candidates have none (the brain's probe restarts it).
- **S, the supervisor process itself was killed, then its service was killed:** openrc-par 6 and openrc-doc 6 (the "supervisor_back=0/1" facts). Nothing restarts `supervise-daemon`, so the service stays down until `rc-service restart`. No other candidate has this class as "needed a person": s6 and runit supervisors are restarted by their scanner (`s6-svscan`, `runsvdir`) within a second.
- **B, the policy layer gave up on purpose and raised the bar alert (alert file = 1):** c-go 2 and c-elixir 2 (a crash storm on driftwm; a supervisor-killed driftwm). This is the designed ladder end ("stop retrying, show an alert, the owner acts"); the person's command is the brain's `retry`.
- **R/D, a service that crashes at every start (storm) or fails to start (seatd) left down:** openrc-par 3 (storm on hubd, storm on driftwm, failing seatd). The same faults are **stuck states** (not "needed a person") in dinit, dinit-ms and dinit-wf; they are recovered by themselves in s6, runit, b-go and openrc-doc.
- **T, an odd one:** runit-111, after a clock jump of -3600 s, waybar missing (`clients missing: ok clients=waybar`); once; BELIEVED a consequence of the lock errors described in item 5, not examined further.
- **Unknown: none.**

## Item 3: the runit claim, checked against the source
**Version and source.** The image runs the Ubuntu 24.04 package `runit 2.1.2-59ubuntu1` (unpacked with `dpkg -x`; its boot message prints `$Id: 25da3b86f7bed4038b8a039d2f8e8c9bbcf0822b $`). The source I read is the **upstream tarball runit-2.1.2** downloaded for reading from https://smarden.org/runit/runit-2.1.2.tar.gz (file `admin/runit-2.1.2/src/runit.c`); its `runit.c` carries the **same `$Id`**, so the revision of that file is the same. I did not read Ubuntu's patch set: the changes I know of from the binary are the paths (`strings` on `/sbin/runit` shows `/run/runit.reboot` and `/run/runit.stopit`; upstream `runit.h` line 3 has `#define REBOOT "/etc/runit/reboot"`). So the claim is SOURCE for upstream and BELIEVED for the Ubuntu binary in the lines below; the log (`runit: leave stage: /etc/runit/2`, then stage 3, without "child crashed") matches it.
**The lines (`src/runit.c` 175 to 199, verbatim):**
```
        if (wait_exitcode(wstat) != 0) {
          if (wait_crashed(wstat))
            strerr_warn3(WARNING, "child crashed: ", stage[st], 0);
          else
            strerr_warn3(WARNING, "child failed: ", stage[st], 0);
          if (st == 0)
            /* this is stage 1 */
            if (wait_crashed(wstat) || (wait_exitcode(wstat) == 100)) {
              strerr_warn3(INFO, "leave stage: ", stage[st], 0);
              strerr_warn2(WARNING, "skipping stage 2...", 0);
              st++;
              break;
            }
          if (st == 1)
            /* this is stage 2 */
            if (wait_crashed(wstat) || (wait_exitcode(wstat) == 111)) {
              strerr_warn2(WARNING, "killing all processes in stage 2...", 0);
              kill(-pid, 9);
              sleep(5);
              strerr_warn2(WARNING, "restarting.", 0);
              st--;
              break;
            }
        }
        strerr_warn3(INFO, "leave stage: ", stage[st], 0);
        break;
```
and `src/wait.h` lines 11 to 13: `#define wait_crashed(w) ((w) & 127)`, `#define wait_exitcode(w) ((w) >> 8)`. For a process killed by a signal the status word is the signal number (here 9), so `(w) >> 8` is 0: the outer `if (wait_exitcode(wstat) != 0)` is false, the whole block, including the "crashed" and the "111" cases, is skipped, and the code falls to `leave stage` and then stage 3. (Reading of the code; matches the logs of the original runit.)
**What the manual says** (https://smarden.org/runit/runit.8.html, fetched 2026-10-12): "STAGE 2: runit runs /etc/runit/2, which should not return until system shutdown; if it crashes, or exits 111, it will be restarted." "STAGE 3: If runit is told to shutdown the system, or stage 2 returns, it terminates stage 2 if it is running, and runs /etc/runit/3. [...] If stage 3 returns, runit checks if the file /etc/runit/reboot exists and has the execute by owner permission set. If so, the system is rebooted, it's halted otherwise." The Ubuntu package's own page (`runit(8)`, in the image tree) says the same with `/run/runit.reboot` and `/run/runit.stopit`.
**Is the test image's `/run/runit.reboot` the documented `/etc/runit/reboot`?** The same mechanism under a different path: neither a symlink nor a different mechanism. The upstream manual names `/etc/runit/reboot`; the Ubuntu build moved the flag files to `/run` (binary strings and the package's manual page). The test image creates the file in `candidates/runit/rootfs/cand/boot.sh` (`touch /run/runit.reboot; chmod +x`). A machine reached by `kill -9 runsvdir` therefore reboots **because** stage 2 returned (runit.c, as above) **and** that flag exists; without the flag the same event powers the machine off. The manual's "crashes" wording describes the case the code does not reach for a death by signal; I read this as a discrepancy between the page and the code, not as a documented behaviour. Nothing in runit's definition was changed for this item.

## Item 4: dinit with waits-for (`candidates/dinit-wf/`, logs `results/step2/dinit-wf/`)
**Definition.** `dinit-ms` with every `depends-ms` replaced by `waits-for` (the 5 edges: driftwm after seatd and udevd; hubd after driftwm; waybar after driftwm and dbus), nothing else: the same commands, `-wait` arguments, `follow-driftwm` wrapper, `restart-limit-count = 0`, `restart-delay = 1.0`. The pages (`dinit-service(5)`, `dinit(8)`) name nothing else that is needed for the follow rule. For a restart after a failed start the pages name **no** property; the nearest statement is `ready-notification` ("If not specified, a process service is considered started as soon as it has begun execution"), i.e. a service without readiness notification cannot fail to start by dying early. That is a change of definition (the s6 candidates use the readiness file descriptor as well), so I did **not** apply it, and ask in the questions.
**Result (TESTED).** Boots 6.1 to 6.3 s; PID 1 killed back in 16 and 18 s; wedged 45 s; trial boots: healthy confirmed 21.1 s, driftwm cannot start rolled back at 60 s, driftwm wedged CONFIRMED after 20.9 s (false: driftwm stopped). Fault suite: 4 launches, finished, 33 of 36 events, **3 stuck states: steps 25 (crash storm on hubd), 26 (crash storm on driftwm), 28 (seatd fails to start)**. Against dinit-ms (5 stuck states) the slow-seatd step and the hung-driftwm busy loop are now recovered. All 18 kills recover (median 1.8 s, worst 4.0 s). SIGSTOP of driftwm: not noticed, 3 of 3 needed a person.
**Cause as far as the log shows (TESTED, BELIEVED for the general rule):** step 25 log: `[STOPPD] hubd`, `[FAILED] hubd` within 1.2 s, then `hubd (exit status: 0)` and nothing more after the 40 s storm ended. The storm makes the service exit at every start, before it reports readiness; dinit then marks the start as FAILED and does not start it again (`dinit(8)`: a service that fails to start is not run again by itself; `restart` is about a service that stops after it started). With `waits-for` the dependents are no longer dragged down, but the failed service itself stays down. Step 28 is the same for seatd. Did not pass, so it is not soaked.

## Item 5: runit with the documented exit-111 restart (`candidates/runit-111/`, logs `results/step2/runit-111/`)
**Definition.** Only `/etc/runit/2` differs from `candidates/runit`: instead of `exec runsvdir -P /run/service` it is `runsvdir -P /run/service` followed by `exit 111` (the manual: "if it crashes, or exits 111, it will be restarted"). The reboot hook (`/run/runit.reboot`) is unchanged.
**Result (TESTED).** Boots 5.9 to 6.2 s; PID 1 killed back in 17 and 16 s; wedged 47 s; trial boots like runit. Fault suite: 1 launch, finished, 50 of 50 events, **0 stuck states, 0 self-reboots** (the original runit rebooted itself at every scanner kill). Scanner kill: `runit: warning: child failed: /etc/runit/2`, `killing all processes in stage 2...`, 5 s later `restarting.` and `enter stage: /etc/runit/2`. **Events that needed a person: 7** (3 SIGSTOP and 3 busy-loop hangs of driftwm, as in runit; and 1 after a clock jump: waybar missing).
**But the restarted stage 2 does not take over cleanly (TESTED, log):** after the restart every new `runsv` prints `runsv NAME: fatal: unable to lock supervise/lock: temporary failure` and every `svlogd` prints `unable to lock directory ...`; 292 such lines until the end of the run (about one per service per second), zombies grew from 0 to 8 and the process count from 88 to 95. The services kept running. **Cause (SOURCE for the code, BELIEVED for the effect):** `runsvdir.c` line 73 `if (pgrp) setsid();` and the stage-2 script passes `-P`, so each `runsv` is in its own session; the `kill(-pid, 9)` of `runit.c` (stage 2's process group) therefore does not reach the old `runsv` and `svlogd` processes, they keep their locks, the new `runsv` cannot take them, and `runsvdir` starts them again every second. That is configuration I did not change (instruction). A variant without `-P` is a question below.
**Passes the owner's rule (0 stuck states, no self-reboot), so it goes into the soak queue.**

## Item 6: the mirror fix (PR #79, commits on branch `tools-source-mirror`)
A repeat `fetch` of a stored entry with a different expected hash now only **reports**: `mirror: demo 1.0 a.txt: hash mismatch: stored file has sha256 ..., expected ...; stored file and its state left untouched` (exit 1). It writes no `.corrupt` marker, downloads nothing, changes neither the file nor its meta file; `path` and `verify` still say ok; the right hash afterwards still says "present". A file that is **downloaded** with the wrong hash was already never stored (it is deleted, "nothing placed"); I kept that: there is no quarantine folder (only the empty lock file stays). New test `TestRepeatFetchWithWrongHashOnlyReports`. Both files are now `gofmt`-clean (they were not before; only whitespace). TESTED: `go test ./tools/mirror/` 11 of 11 pass; `go test -p 1 ./...` all packages ok (56 s); and by hand with a local server: wrong-hash repeat fetch exit 1 and the entry intact, `path` exit 0, `verify` exit 0, wrong-hash download exit 1 with nothing stored.


## Memory counter fix (found during item 7; applies to OpenRC only)
The soak driver finds the init-side processes by name (`/proc/PID/comm`). The kernel cuts a process name at 15 characters, so OpenRC's `supervise-daemon` (16 characters) was never found: every OpenRC memory figure I had given before held **PID 1 (`openrc-init`) only**. This is a measurement defect of mine; the figures were `openrc` 664 kB, `openrc-par` 586 kB and `openrc-doc` 548 kB in the boot tables. Fixed in `hubsim/chaos.go` (`supGroup` also matches the command line). Re-measured with the fixed driver (3 boots each, `results/step2/openrc-remeasure/`): **`openrc-par` 3036 kB and `openrc-doc` 3056 kB** (PID 1 about 700 kB plus six supervisors of about 400 kB each). The first openrc-doc pilot (30 minutes) was run before the fix, its memory columns are void and it is kept as `INVALID-openrc-doc-pilot1.*` (fault and restart data valid); the pilot was repeated with new seeds. Every other candidate's processes have names of 15 characters or fewer (`s6-supervise`, `runsv`, `runsvdir`, `brain`, `beam.smp`); for c-elixir the two small Erlang helper processes (`erl_child_setup`, `inet_gethost`) are not in the figure.

## Item 7: the soak queue (candidates that passed the elimination round: openrc-doc, runit-111; dinit-wf did not pass and was not soaked)
**Method.** The same as before (`soakq.sh`, `soak_analyze.py`, `soak_ci.py`): a 30-minute pilot first, then 2 chunks of 2 hours, a new seed for every run, PSS of PID 1 + scanner + all supervisors + brain every 60 s, restarts counted inside the VM, first 600 s of each boot left out, slope with a 95 % interval (standard error widened for the correlation of neighbouring samples), result files committed after every chunk (`results/step2/soak/`). The pilots measured the noise: openrc-doc total PSS noise 71 kB on 3 MB (it varies with the number of supervisor processes), runit-111 18 kB on 1.9 MB; both are small against 2-hour chunks of 800 to 900 restarts, so 2-hour chunks were kept. **Events** in the tables are the injector's EV lines of the soak (a random fault every 5 to 20 s); **needed a person** = EV lines with ok=0 (item 2 explains the term); stuck = a state only a reboot clears; self-reboot = the machine rebooted itself.

| | openrc-doc | runit-111 |
|---|---|---|
| Soak length | 2 x 2 h (+ 30 min pilot, not counted) | 2 x 2 h (+ 30 min pilot, not counted) |
| Injected events | 569 | 647 |
| Restarts counted (stretches used) | 1624 | 1573 |
| PSS of the init side, start, end | 2988 kB, 2977 kB | 1896 kB, 1841 kB |
| **Slope, kB per 1000 restarts** | **+33.5, 95 % interval [-50.2, +117.2]** | **+0.2, 95 % interval [-29.3, +29.7]** |
| Noise (sd around the line) | 126 kB | 23 kB |
| Stuck states | 0 | 0 |
| Self-reboots | 0 | 0 (the original runit rebooted itself 29 times in the same fault mix) |
| Events that needed a person | 53 of 569: all of one kind, a service killed after its `supervise-daemon` had been killed (driftwm 11, hubd 18, seatd 16, waybar 8) | 55 of 647: 38 SIGSTOP and 16 busy-loop hangs of driftwm, 1 supervisor-killed driftwm |
| Other | the hung compositor is noticed by the health check (about 20 s) | 6 scanner kills in 4 h, each `runit: warning: child failed: /etc/runit/2`, `restarting.`, no reboot; **7831 lines `unable to lock supervise/lock` / `svlogd: unable to lock directory`** (2757 in chunk 1, 5074 in chunk 2: the restarted stage 2 keeps trying to start `runsv` and `svlogd` while the old ones hold the locks, item 5) |
Per process, runit-111's `runsvdir` moved +38 kB per 1000 restarts in chunk 1 and -45 in chunk 2 (standard errors about 3), i.e. no consistent direction; `runsv` +47 (chunk 2) with standard error 10. OpenRC's `supervise-daemon` (six processes together) +47 [standard error 48]. No leak is visible in either at this length; a leak under about 50 kB per 1000 restarts (OpenRC) or 30 kB (runit-111) would not be seen. 4 hours is the horizon of the test.

**Earlier soaks, same method, for the table below:** s6plain (2 x 2 h; 1228 restarts; 2457 to 2464 kB; slope +20.9 [+1.7, +40.0]; noise 3.6 kB; 595 events, 54 needed a person (hang class), 1 stuck state after a hung driftwm, 0 self-reboots), s6rc-par (1357; 2687 to 2681; -8.6 [-19.5, +2.2]; 4.9 kB; 663 events, 50 needed a person, 0 stuck, 0 self-reboots), runit (646 restarts in the one usable chunk; 1900 to 1879; -70.4 [-168.9, +28.1]; 24 kB; 779 events, 43 needed a person, 0 stuck, 29 self-reboots), b-go (1644; the Go brain moves by 1 MB; +99.9 [-1084, +1284]; 702 events, 0 needed a person, 0 stuck). c-go and c-elixir were stopped early (item 1 of the first follow-up).

## Item 8: a sharper memory measurement for b-go (`candidates/b-go-stats/`, logs `results/step2/soak/b-go-stats-*`, script `brain_counters.py`)
**What I added (and only this).** (1) In the brain (`prototypes/go/main.go`): one new command on its status socket, `gcstats`. It calls `debug.FreeOSMemory()` (a forced garbage collection followed by returning freed memory to the operating system), then prints `runtime.MemStats` `HeapInuse`, `HeapAlloc`, `HeapSys`, `HeapReleased`, `NumGC`, `runtime.NumGoroutine()` and the number of open file descriptors (`/proc/self/fd`, which includes the one descriptor the listing itself holds). Nothing the brain decides depends on it. The brain binary of `b-go-stats` is that program; `b-go` itself is unchanged. (2) In the soak driver (`hubsim/chaos.go`): when the kernel command line has `hub.gcstats=1`, every 60 s, right before the PSS is read, it sends `gcstats` and logs one `BRAINSTAT` line next to the restart count. So **the PSS of the brain in this run is read just after a forced collection**, which changes what is measured (the heap is trimmed every minute); this is the point of the measurement, and it also means the new brain PSS values are lower and quieter than those of the earlier b-go soak. The same mix of faults and 60-second sampling as before; 2 x 2 h with new seeds after a 30-minute pilot.
**Result (TESTED; 1542 restarts counted, 220 samples, 0 stuck states, 0 self-reboots, 0 events that needed a person).**

| counter | start, end (mean of first and last 5 samples) | slope per 1000 restarts | 95 % interval | noise (sd) |
|---|---|---|---|---|
| brain PSS (smaps_rollup) | 4140 kB, 4274 kB | -37.2 kB | [-273.9, +199.4] kB | 277 kB |
| HeapInuse | 589 005 B, 619 315 B | +7 158 B | [-24 076, +38 391] B | 46 036 B |
| HeapAlloc | 118 821 B, 135 336 B | +4 648 B | [-11 662, +20 957] B | 19 604 B |
| HeapSys | 3 627 418 B, 3 545 498 B | -33 320 B | [-132 028, +65 389] B | 119 812 B |
| HeapReleased | 3 038 413 B, 2 926 182 B | -47 279 B | [-171 820, +77 263] B | 159 840 B |
| goroutines | 11, 14 | +2.0 | [-0.6, +4.6] | 3.9 |
| open file descriptors | 15, 16 | +1.7 | [-0.6, +4.1] | 3.6 |

Reading (no pick): the 95 % interval of the brain's PSS slope is now 473 kB wide, against 2368 kB before (5 times narrower; s6rc-par's is 22 kB wide, so the Go brain is still about 20 times wider). At 95 %, a leak of more than about 200 kB per 1000 restarts (200 bytes per restart) in the brain's PSS is excluded, and in the Go heap in use more than about 38 bytes per restart and in the live allocations more than about 21 bytes per restart. Goroutines and descriptors rose by two over 1500 restarts; the interval includes zero, so it is not shown to be a trend, and it is not shown to be none (UNKNOWN: a drift of about one goroutine per 500 restarts cannot be told from none in this run). The Go runtime's counters are about ten times quieter than the PSS, which still holds the noise of the C library and the kernel's accounting. The c-go brain was **not** measured (the instruction: only if time allows after everything else; I stopped here).

## Item 9: hang rule data (no pick)
The owner's decision (2026-10-12): the hang rule is a tiny separate watcher that probes the compositor and hubd and kills a frozen one; the watcher and the conductor probe each other. The open question is only whether any init's own built-in check is worth using instead. Facts, per candidate that survived the elimination round (no stuck state: s6plain, s6rc-par, runit, runit-111, b-go, openrc-doc; c-go and c-elixir are included because they only had the by-design give-up). The dinit variants did not survive and are left out. "Lacks" means: lacks in the configuration I tested.

| Candidate | (a) supervise the watcher (restart it when it dies) | (b) kill a frozen service | (c) restart it, with "after 4 failed rounds and 30 s with both probes failing, at most 3 kills in 10 minutes" | What it lacks |
|---|---|---|---|---|
| s6plain, s6rc-par | Add one service folder with a `run` script; `s6-supervise` restarts the watcher at once (1-second pause, TESTED for all services). s6rc-par also needs the folder compiled into the s6-rc database (`s6-rc-compile`). | `s6-svc -k` sends SIGKILL to the supervised process (SOURCE, s6 pages; used by the harness's fix command); the pid is in `s6-svstat`. A frozen (SIGSTOP) process dies to SIGKILL. | After the kill `s6-supervise` starts the service again after at most 1 s (TESTED, 18 kills, median 2 to 3 s to a working desktop). No limit exists in s6, so the 4 rounds, the 30 s and the 3-in-10-minutes count would all live in the watcher. | A rate limit or a count of kills; a way to tell the init "give up and show a message" (the harness's alert is its own file). |
| runit, runit-111 | One service folder; `runsv` restarts the watcher. | `sv kill` or `sv force-restart` (SOURCE, `sv(8)`; the harness's fix uses `force-restart`); the harness's fix uses `force-restart`. | `runsv` restarts after the kill (TESTED, kills median 1.9 s). No limit built in; all counting in the watcher. | The same. Additionally runit's scanner (`runsvdir`) kill is a machine reboot in the stock setup (item 3) or a degraded restart in runit-111 (item 5): the watcher's supervisor tree is not stable under that fault. |
| b-go (brain over s6-svscan, direct mode) | The brain starts and watches the services itself. The watcher would be a seventh entry in the brain's graph (a Go table in the source, `mk(...)` calls in `main.go`) or a separate s6 service next to the brain (`s6-svscan` is PID 1 and supervises the brain and the watchdog feeder). Needs a source change or a new service folder. | `Kill(name)` exists in the brain's backend (SIGKILL in direct mode, `s6-svc -k` in s6 mode); the watcher could also kill by itself. | The brain already does probe, kill and restart for driftwm (TESTED 10.6 to 15.1 s) with a ladder: restart, pause, restart dependents, give up (bench values: failures in a 60-second window, give up after 6). The rule's 10-minute window and 3 kills would be a change of the ladder's numbers; the 4 rounds and 30 s do not exist (see the table below). | A second probe, a 30-second condition, a ten-minute window; probing the brain itself (nothing probes it: a killed brain is restarted by `s6-svscan`, TESTED 0.1 s, services untouched). |
| c-go, c-elixir | Same brain; the watcher is an s6-rc service (source folder + database) or the brain's graph. | Same brain backend (`s6-svc -k`). | Same ladder. Measured: storm on driftwm and a killed supervisor ended in the by-design give-up (alert file, `retry` by a person). | Same as b-go. |
| openrc-doc | Init script with `supervisor=supervise-daemon`: restarts the watcher if it dies. If `supervise-daemon` itself is killed, nothing restarts it (class S in item 2, no documented option). | `supervise-daemon NAME --signal KILL` (SOURCE `supervise-daemon.8`, option `-s/--signal`: tell a supervisor to signal the process it supervises; not tested), or `rc-service X restart`, which stops by the `retry` schedule (here TERM, 5 s, KILL, 5 s), or the watcher kills the pid itself. | OpenRC's own health check already does probe, stop and respawn (below). `respawn_max=3`, `respawn_period=600` would approximate "3 in 10 minutes" but counts crashes too; past the limit `supervise-daemon` exits and leaves the service down. | The 4 rounds and 30 s (the first failed check acts); a kill-only count; the supervisor-of-the-supervisor. |

**OpenRC's documented health check and retry schedule against a separate watcher (cost, SOURCE `supervise-daemon.c`, `supervise-daemon-guide.md`, TESTED in openrc-doc):**
- Per service a `healthcheck()` shell function plus `healthcheck_timer`, `healthcheck_delay`, `retry=...` in the service's script: for the owner's two probes (compositor and hubd) that is two functions in two scripts; the rule's counting (4 rounds, 30 s, kill budget) would have to be kept in a state file written by the function, because `supervise-daemon` keeps no count of failed checks (one failed check = stop and respawn, lines 724 to 745).
- The check runs as a child that `supervise-daemon` waits for before it does anything else (`rc_waitpid`, lines 729 to 731): a check that hangs holds the supervisor; the stand-in's probe has a 1-second timeout, a different probe program would need its own.
- Health-check restarts count as respawns toward `respawn_max`/`respawn_period` (lines 765 to 785); after the limit the supervisor exits and the service stays down.
- Measured recovery of a SIGSTOP'ed compositor: 19.4 to 20.2 s (check every 2 s after a 10 s delay, TERM wait 5 s, KILL, respawn 1 s) against 9.3 to 15.4 s for the brains and "not noticed" for the others. These timings are bench values for both.
- It works only while `supervise-daemon` lives; the separate watcher is itself supervised by something, and in OpenRC that something is the same program.
- A separate watcher: one program for all inits, the counting in one place, mutual probing with the conductor possible; it needs (a) to (c) above from whatever init the candidate is.

**How the brain prototypes' probing differs from the owner's rule (so nobody reads their 10 to 15 s as the rule working):**

| | Owner's rule (HUB-OS.md, hub stability decisions) | Brain prototypes (`prototypes/go/probe.go`, `ladder.go`, `main.go`; Elixir: `prototypes/elixir/brain.ex`) |
|---|---|---|
| What is probed | the compositor and hubd; "both probes" failing | driftwm only: one question ("state") on `/run/dw/wayland-1`, must answer "ok..." within 1 s |
| Rounds | 4 failed rounds and 30 s | 3 failed probes in a row (`Need: 3`), one probe per 1-second tick; 8 s grace after a start |
| Earliest kill | after 30 s | after about 3 s of failures (plus the 8 s grace after a start) |
| Time to a working desktop after SIGSTOP | not measured (rule not built) | 9.3 to 15.4 s (kill, restart, wait for the socket, restart hubd and waybar) |
| Kill budget | at most 3 kills in 10 minutes | ladder over a 60-second window: pause after 2 failures, restart dependents after 4, give up after 6 (bench numbers), then "degraded", alert file, and only the owner's `retry` resumes |
| After the budget | stop restarting the compositor, fixed message, hubd and recovery terminal stay up | degraded + alert file (no fixed message, no terminal) |
| Probing the prober | the watcher and the conductor probe each other | nobody probes the brain; s6 restarts it if it dies (TESTED 0.1 to 0.2 s) |
The brains therefore detect a freeze about ten times sooner than the rule would and have a different budget; their timings measure the prototype's bench values, not the rule.


## Updated final table (all candidates; no ranking; explanation of the counts in item 1, of "needed a person" in item 2)
"Stuck" is counted in the elimination round. "Needed a person" counts EV lines with ok=0 in the elimination round. "Soak" is 2 x 2 h unless said otherwise; the leak slope is in kB per 1000 restarts with its 95 % interval (item 3 and item 7).

| Candidate | Events the script defines / ran | Stuck states | Needed a person | SIGSTOP of the compositor | Soak (4 h): events, needed a person, stuck, self-reboots; PSS start to end; slope [95 % interval] |
|---|---|---|---|---|---|
| s6plain (full repeat) | 48 / 48 | 0 | 3 | not noticed | 595, 54, 1, 0; 2457 to 2464 kB; +20.9 [+1.7, +40.0] |
| s6rc (first setup, not equal) | 48 / 19 | 8 | 3 | - | not run |
| s6rc-par | 48 / 48 | 0 | 5 | not noticed | 663, 50, 0, 0; 2687 to 2681 kB; -8.6 [-19.5, +2.2] |
| runit | 50 / 48 (2 lost to the self-reboot at the scanner kill) | 0 | 4 | not noticed | 779, 43, 0, 29; 1900 to 1879 kB (one usable chunk); -70.4 [-168.9, +28.1] |
| runit-111 | 50 / 50 | 0 | 7 | not noticed | 647, 55, 0, 0; 1896 to 1841 kB; +0.2 [-29.3, +29.7]; 7831 lock-error lines |
| b-go | 38 / 38 | 0 | 0 | noticed, 10.6 to 15.1 s | 702, 0, 0, 0; brain PSS 6242 to 6948 kB; +99.9 [-1084, +1284] |
| b-go-stats (b-go + counters, forced collection) | not run as elimination | - | - | - | 700, 0, 0, 0; brain PSS 4140 to 4274 kB; -37.2 [-274, +199] |
| c-go | 50 / 48 | 1 | 2 | noticed, 9.3 to 13.7 s | stopped after 14 minutes: 2 stuck states |
| c-elixir | 50 / 47 | 2 | 2 | noticed, 14.1 to 15.4 s | stopped: 1 stuck state, then a logger loop |
| dinit (first setup) | 36 / 20 (relaunch limit) | 8 | 6 | not noticed | not run |
| dinit-par | 36 / 20 (relaunch limit) | 8 | 4 | not noticed | not run |
| dinit-sm | 36 / 13 (relaunch limit) | 8 | 0 | not noticed | not run |
| dinit-ms | 36 / 31 | 5 | 3 | not noticed | not run |
| dinit-wf | 36 / 33 | 3 | 3 | not noticed | not run |
| openrc (first setup) | 48 / 46 | 2 | 20 | not noticed | not run |
| openrc-par | 48 / 48 | 0 | 12 | not noticed | not run |
| openrc-doc | 48 / 48 | 0 | 6 | noticed, 19.4 to 20.2 s | 569, 53, 0, 0; 2988 to 2977 kB; +33.5 [-50.2, +117.2] |

Boot (supervisor start to desktop), PID 1 killed (back in), PSS at idle, trial boots: see `results/step2/FINAL-TABLES.md`; the OpenRC PSS there is the corrected one.

## What I could not test (follow-up 2)
Real hardware and KVM timing; the real compositor, viewers and hubd (stand-ins only); `waits-for` together with a dinit service that does not report readiness (so a failed start cannot occur: not run, one attempt only); runit-111 without `-P` (would let the old `runsv` die with the stage-2 process group); the c-go brain's counters; leaks smaller than the intervals above; anything longer than 4 hours; the hang rule itself.

## Questions for the owner (follow-up 2)
1. dinit: the pages name no property that retries a failed start; the nearest is not giving the service a readiness notification (then it cannot "fail to start" by dying early). Run that as one more variant (`waits-for` without `ready-notification`)? It changes the definition (the s6 candidates use the readiness descriptor).
2. runit-111 passes your rule (0 stuck, no self-reboot) but its restarted stage 2 loops on lock errors (7831 lines in 4 h) because `runsvdir -P` puts each `runsv` in its own session. Run a variant without `-P`?
3. openrc-doc: 0 stuck, no self-reboot, but a killed `supervise-daemon` still needs a person (53 times in 4 h). Is that "passed"?
4. Should the c-go brain get the same counters (4.5 h)?
5. The hang rule data (item 9) is complete; which init, if any, do you want costed in more detail for the separate watcher?


---

# Owner decisions of 2026-10-12

Recorded from the owner's message (no change by me):
- **dinit:** no further variants. Three documented setups were tried (`depends-on`: dinit-par and dinit-sm; `depends-ms`: dinit-ms; `waits-for`: dinit-wf); the results are recorded and dinit goes no further.
- **openrc-doc:** recorded as passing the elimination rules (0 stuck states, 0 self-reboots) but **not** recovering by itself from a killed supervisor (class S events). It is not called passed or failed overall.
- **Hang rule architecture:** a tiny separate watcher probes the compositor and hubd and kills a frozen one (after 4 failed rounds and 30 s with both probes failing, at most 3 kills in 10 minutes, never a reboot); the init only supervises and restarts. The hang rule is still not built in the product; where it lives is what Part 6 informs.

---

# Follow-up 3 (2026-10-12): data only

> **Correction added in follow-up 4.** The "rule as written" rows of this section paired a compositor probe with a **hubd** probe and required both to fail. That is not the owner's rule: `hub-stability.md` section 9.1 ("Two probes, and both must fail") pairs two probes that are **both aimed at the compositor** (driftwm's socket `state` request and a Wayland `wl_display.sync` round trip), both answered by the compositor's single event-loop thread. So the rows `s6rc-par-wd` and `runit-wd` are relabelled "compositor + hubd probes (not the owner's rule)" and the `-wdc` rows "one compositor probe only (not the owner's rule)". The numbers are unchanged and kept as measured. The rows for the owner's rule are in "Follow-up 4" below. The program used here is kept unchanged in `tools/bench/init/prototypes/watcher-fu3/`.

Order from the owner: items 1 to 5. No init written, no winner named, whether s6 stays is not decided. Parked items (focus core, session layer, backlog) were not started. The only new program is the test watcher, a **TEST STAND-IN, not the product watcher; the hang rule is not built** (`tools/bench/init/prototypes/watcher/`). One virtual machine at a time throughout; no packages installed; no result was taken while another VM ran. Logs: `tools/bench/init/results/step2/<candidate>/` (spam `SAMPLE` lines stripped); tables: `results/step2/FINAL-TABLES.md`.

## Item 1: runit-111np (`candidates/runit-111np/`, logs `results/step2/runit-111np/`)
**Definition.** `candidates/runit-111` with exactly one change: stage 2 is `runsvdir /run/service` followed by `exit 111`, **without `-P`**. The exit-111 restart and the reboot hook are unchanged.
**Result (TESTED), elimination round, one launch, suite finished:**

| | runit-111np |
|---|---|
| Boots / PID 1 killed / PID 1 wedged | 6.6 s (6.5 to 6.6); back in 17 and 16 s; back in 47 s |
| Trial boots | healthy CONFIRMED 20.7 s; driftwm cannot start CONFIRMED 21.0 s (FALSE: waybar not running); driftwm wedged CONFIRMED 20.7 s (FALSE: driftwm stopped) (same as runit and runit-111) |
| Events (defined / ran) | 50 / 50 |
| Stuck states | 0 |
| Self-reboots | 0 (one launch; the scanner kill: `child failed: /etc/runit/2`, `killing all processes in stage 2`, `restarting`, services back) |
| Events that needed a person, by class | 6, all class H (3 SIGSTOP and 3 busy-loop hangs of driftwm, steps 19 to 24); none of class S, R, D, B or T (runit-111 had one extra, class T, after a clock jump) |
| Lines `unable to lock` | **53** (all `svlogd: warning: unable to lock directory: /var/log/driftwm` (44) or `/var/log/waybar` (9)); **0** `runsv ... unable to lock supervise/lock` lines (runit-111 had those after the stage-2 restart) |

Where the 53 lines come from (TESTED, per step): only steps 31 to 36 (killing the per-service supervisor `runsv`: 9, 11, 9, 10, 4, 10 lines); none after the scanner kill (step 37) or later. runit-111 had lock lines at steps 31 to 36 as well (11, 10, 4, 10, 5, 10), plus 20 at step 37 and 146 at step 40 and later. The stock `runit` rerun of steps 31 to 43 has 0 such lines in its log (UNKNOWN why; not examined). BELIEVED, not tested: after `runsv` is killed, its log process `svlogd` keeps the log directory lock and the new `runsv` starts a second `svlogd` that cannot take it, and exits; the services themselves kept running in all these events.
**Soak: not run.** The owner's rule was 0 stuck, 0 self-reboots **and** 0 lock-error lines; there are 53 lock-error lines. runit-111np removes the `runsv`/stage-2 lock errors (runit-111: 292 in its elimination round) but not these. Whether 53 lines of `svlogd` at supervisor kills is acceptable is the owner's question (below).

## Item 2: final table and the new column
See "Updated final table (follow-up 3)" below: new column "restarts a killed supervisor by itself", from the supervisor-kill steps 31 to 36 (the supervisor of driftwm or waybar killed, then the service killed) and the scanner step 37.

## Item 3: the test watcher with s6rc-par and runit-111
**What the program is (SOURCE: `prototypes/watcher/main.go`, 161 lines, static Go).** Probes copied from the brain prototype (`Probe(sock, request)` of `prototypes/go/probe.go`; no new probe): the stand-in compositor's `state` on `/run/dw/wayland-1` and the stand-in hubd's `ping` on `/run/hubos/hubd.sock`, once per second. A round fails when **both** probes fail. After 4 failed rounds **and** 30 s since the first failed round of the streak it sends SIGKILL to the process named `driftwm` and starts a new streak; at most 3 kills in 10 minutes (the 4th and later are logged as `CAP: would kill ... not killing`); it never reboots; it restarts nothing (the init does); it logs every probe round and every kill with a timestamp to stderr and the console; no ladder, alert or conductor. The rule is unit-tested (`go test ./tools/bench/init/prototypes/watcher/`: 4 tests pass: needs 4 rounds and 30 s, both probes must fail, an answer resets the streak, 3 kills in 10 minutes then allowed again after the budget returns). Its memory (the rule's counters) lives in the process: a watcher that is killed starts with empty counters (SOURCE; not tested separately).
**Setup.** `candidates/s6rc-par-wd/` (s6rc-par plus an s6-rc long-run service `watcher` that depends on `hubd`, listed in `top`) and `candidates/runit-wd/` (runit-111 plus `/etc/sv/watcher/run` and a log service). `*-wdc` uses the same images with the watcher in `-mode compositor` (see below). The fault suite is the same plus steps 44 and 45: kill the watcher with SIGKILL; the init must start it again (EV `watcher-killed`).
**What I found before the runs (TESTED in the first boots): the owner's rule as worded cannot fire for a frozen stand-in compositor.** The stand-in hubd keeps answering `ping` while the compositor is stopped (its "connected" flag only drops when the compositor's socket closes). A frozen compositor therefore gives "compositor fails, hubd ok", not "both fail". I ran the rule **as written** first (`-wd`) and, as a clearly labelled variation, the same program acting on the compositor probe alone (`-wdc`, still 4 rounds, 30 s, 3 kills in 10 minutes). The `-wdc` variation is not the owner's rule; it is there to show what the init does once a kill happens. What the product's second probe should be is a question for the owner.

| | s6rc-par-wd (compositor + hubd probes, not the owner's rule) | runit-wd (compositor + hubd probes, not the owner's rule) | s6rc-par-wdc (one compositor probe only, not the owner's rule) | runit-wdc (one compositor probe only, not the owner's rule) |
|---|---|---|---|---|
| Events defined / ran | 50 / 50 | 52 / 52 | 50 / 50 | 52 / 52 |
| Stuck states | 0 | 0 | 0 | 0 |
| Self-reboots | 0 | 0 | 0 | 0 |
| Needed a person (steps) | 5: 19, 20, 21, 23, 24 (class H; step 22 answered by itself in 0.11 s) | 5: 19, 21, 22, 23, 24 (class H; step 20 answered in 0.12 s) | 1: 23 (class H, cap reached) | 3: 22, 23, 24 (class H, cap reached) |
| Hang steps 19 to 24: watcher kills | **0 of 6 hangs covered**: no kill | **0 of 6 covered**: no kill | **3 kills** (steps 19, 20, 21), then the cap | **3 kills** (19, 20, 21), then the cap |
| Fault to desktop answering again, for the hangs that were killed | - | - | 36.5, 38.0, 36.5 s | 36.8, 35.8, 36.9 s |
| Cap at work (lines `CAP: would kill`) | none | none | step 23: 42 lines (the 4th hang: harness fixed it at 65 s, needed a person); step 26 (driftwm crash storm): 14 lines | steps 22, 23, 24: 34 lines each; step 26: 13 lines |
| Kills outside a hang | none (longest streak of both-failing rounds 30 rounds = 29.0 s, in the driftwm crash storm, step 26: one second short of the 30 s condition) | **1 kill, at step 26 (driftwm crash storm, 31 failed rounds = 30.0 s): a compositor that was crash-looping, not hung** | none (kills only at 19 to 21; in the step 26 storm the cap refused 14 times) | none (same) |
| Watcher killed (steps 44 and 45): init restarted it | yes, 2 of 2: new pid after 1.07 and 1.08 s | yes, 2 of 2: new pid after 0.21 and 0.21 s | yes, 2 of 2: 1.15 and 1.10 s | yes, 2 of 2: 0.21 and 0.21 s |
| Other differences noticed | - | the OOM step 49 lost driftwm and hubd, step 50 driftwm (restarted, `rec` 0.17 and 0.15 s); not seen in the same image in the `-wdc` run (UNKNOWN: random order of kills by the out-of-memory killer, not examined) | - | - |
| Lock lines `unable to lock` | n/a | 154 (steps 31 to 36 and 37 on, as runit-111) | n/a | 100 |

**Against the owner's rule and against b-go (TESTED).** The rule's own timing is 30 s plus the restart. Measured fault-to-desktop with a kill: **35.8 to 38.0 s** (s6rc-par and runit alike), against **10.6 to 15.1 s** for b-go (brain with its own 3-failure probe). The restart after the kill took 3.8 to 5.8 s of that (the driftwm stand-in waits for seatd and udev sockets and the dependents follow). Up to the cap the watcher was reliable in the `-wdc` variation: 6 of 6 kills at steps 19, 20, 21 recovered without a person; at the 4th the cap refused as designed (steps 22 to 24 and the storm lines) and the harness fixed the hang by its own command after 64 to 65 s.
**What the watcher did when killed.** Both inits restarted it (1.1 s s6, 0.2 s runit; new pid; in `-wd` and `-wdc`, 8 of 8 watcher kills). A killed watcher forgets its streak and its kill budget (SOURCE: counters in memory); whether that matters is not tested (no fault in the suite kills the watcher during a hang).
**Not tested:** the rule with a real second probe; the budget returning after 10 minutes in a VM run (only the unit test covers it); the watcher probing the conductor and the conductor probing it; a soak with the watcher.
**Do I need a soak?** I did not run one (instruction). A reason to consider one: the rule's false kill in the runit-wd crash storm (a compositor restarting at every start for 30 s is "both probes failing" for 30 s) shows the rule interacts with crash storms; the soak's 5 % storms would show how often.

## Item 4: what the separate watcher costs per init (data only; no pick)
"Evidence" = the commands/files of Item 3 or an earlier TESTED result. A dash = not run with the watcher.

| | (a) supervise the watcher | (b) the watcher kills a service and the init restarts it | (c) alert after N restarts in a minute |
|---|---|---|---|
| **s6rc-par** | A directory `source/watcher/` with `type` (`longrun`), `run` (3 lines: reads `hub.wmode` from `/proc/cmdline`, `exec watcher`), `dependencies.d/hubd`; one line `watcher` in `source/top/contents`; `s6-rc-compile` at build; the binary is copied into the image. 4 small files and 1 build line. Evidence: `candidates/s6rc-par-wd/`; watcher killed twice, restarted in 1.07 and 1.08 s. | None beyond the watcher itself: the watcher sends SIGKILL to the pid (pid found by name in `/proc`); `s6-supervise` starts the service again. Alternative with the init's own command: `s6-svc -k /run/service/driftwm` (used by the harness's fix command, TESTED). Evidence: steps 19 to 21 of `s6rc-par-wdc`: killed, desktop answering again 36.5 to 38.0 s after the fault. | **Missing.** s6 has no count of restarts and no alert; `s6-supervise` restarts forever with a 1-second pause (TESTED: 18 to 26 starts in a 40-s storm, `alert=no`). The count and the alert would be in the watcher (a counter on observed pid changes) or in a second program. |
| **runit (runit-111 / runit-111np)** | A directory `/etc/sv/watcher/` with `run` (3 lines) and `log/run` (3 lines), copied to `/run/service` at boot by `boot.sh`; `runsv` supervises it. 2 small files and 1 build line. Evidence: `candidates/runit-wd/`; watcher killed twice, restarted in 0.21 s each. Not stable under a scanner kill (runit-111: stage 2 restarts, 5 s, all services restarted; stock runit: reboot). | None beyond the watcher itself (SIGKILL by pid); `runsv` restarts the service. With the init's own command: `sv kill driftwm` / `sv force-restart` (the harness's fix command, TESTED). Evidence: steps 19 to 21 of `runit-wdc`: 35.8 to 36.9 s fault to desktop. | **Missing.** `runsv` restarts forever with a 1-second pause and keeps no count (TESTED: 26 to 33 starts in a 40-s storm, `alert=no`). `sv status` shows only the time since the last start. Same remedy as s6: counter in the watcher or another program. |
| **openrc-doc** | An init script `/etc/init.d/watcher` (about 12 lines, like `hubd`: `supervisor=supervise-daemon`, `command=...`, `respawn_delay=1`, `respawn_max=0`, `depend() { need hubd }`) and a symlink in `/etc/runlevels/default`. Evidence: the same pattern for the six stand-ins (TESTED: a service killed after its supervisor was killed is **not** restarted; the supervisor of the watcher itself is not supervised by anything). - (not run with the watcher) | The watcher sends SIGKILL by pid; `supervise-daemon` respawns after `respawn_delay` (TESTED for plain kills of the six services: median 2.8 s, worst 17.7 s). Alternative with OpenRC's own command: `supervise-daemon driftwm --signal KILL` (SOURCE `supervise-daemon.8`, not tested). - (not run with the watcher) | **Partly.** `respawn_max`/`respawn_period` give up after N respawns in a period (SOURCE `supervise-daemon.8`); the supervisor then exits and the service stays down (a give-up, not an alert; with `respawn_max=0` there is no limit, TESTED in `openrc-doc`). No alert is shown. |
| **b-go** | The watcher would be a seventh entry in the brain's table (`mk("watcher", "hubd")`, Go source change and rebuild) or a separate s6 service folder next to the brain (`s6-svscan` is PID 1 and restarts the brain). TESTED earlier: brain killed, restarted by `s6-svscan` in 0.1 s, services untouched. - (not run with the watcher) | The brain already restarts a killed service (TESTED: 38 events; kills median 3.6 s, worst 6.7 s); a watcher kill is a normal death to it. - (not run with the watcher) | **Already there in part:** the ladder counts failures in a 60-second window and gives up after 6 (bench value) with the alert file; the harness found `alert=no` in the 40-s storms (UNKNOWN why, not examined). The number and window are source constants. |

**OpenRC's documented health check beside the separate watcher** (SOURCE `supervise-daemon.c`, `supervise-daemon-guide.md`; TESTED in `openrc-doc`; the watcher side is TESTED in Item 3):

| | OpenRC `healthcheck()` + `retry` | Separate watcher (Item 3) |
|---|---|---|
| Probe | a shell function per service in its init script (`healthcheck_timer=2`, `healthcheck_delay=10`); here `hubsim probe` with a 1-s timeout | a program, two probes per round |
| Rounds before it acts | none: the first failed check stops and respawns (SOURCE lines 724 to 745) | 4 failed rounds and 30 s (unit-tested; run in `-wdc`) |
| Kill budget | health-check restarts count toward `respawn_max`/`respawn_period` (SOURCE lines 765 to 785); past the limit `supervise-daemon` exits and the service stays down | 3 kills in 10 minutes, logged `CAP`; the compositor is left as it is |
| Time fault to desktop | 19.4 to 20.2 s (TESTED, SIGSTOP) | 35.8 to 38.0 s (TESTED, `-wdc`); the compositor + hubd pairing (not the owner's rule) did not fire |
| Works while | `supervise-daemon` of that service lives (a killed supervisor: nothing restarts it, TESTED 6 of 6 follow-up kills needed a person) | the watcher lives (killed: restarted by the init in 0.2 to 1.1 s, 8 of 8) |
| A check that hangs | holds the supervisor (SOURCE `rc_waitpid`, lines 729 to 731) | the probe has a 1-s timeout; a hung watcher is not probed by anything here (the conductor would) |
| One place for the counting | no: one function per service plus a state file | yes, one program for every init |

## Updated final table (follow-up 3; no ranking; counts explained in follow-up 2, items 1 and 2)
New column: **restarts a killed supervisor by itself** (yes / no / not applicable), from the supervisor-kill steps 31 to 36 (12 events per candidate that has the part: the supervisor of driftwm or waybar is killed with `kill -9`, then the service is killed) and, for runit, step 37 (kill of the scanner).

| Candidate | Events defined / ran | Stuck | Needed a person | SIGSTOP of the compositor | Restarts a killed supervisor by itself | Soak (4 h): events, needed a person, stuck, self-reboots; PSS start to end; slope [95 %] |
|---|---|---|---|---|---|---|
| s6plain (full repeat) | 48 / 48 | 0 | 3 | not noticed | **yes** (steps 31 to 36: 12 of 12 events ok; `s6-svscan` restarts `s6-supervise`, the service kept running) | 595, 54, 1, 0; 2457 to 2464 kB; +20.9 [+1.7, +40.0] |
| s6rc (first setup) | 48 / 19 | 8 | 3 | - | not tested (run ended at step 27) | not run |
| s6rc-par | 48 / 48 | 0 | 5 | not noticed | **yes** (steps 31 to 36: 12 of 12 ok) | 663, 50, 0, 0; 2687 to 2681 kB; -8.6 [-19.5, +2.2] |
| runit | 50 / 48 | 0 | 4 | not noticed | **yes** (steps 31 to 36: 12 of 12 ok; `runsvdir` restarts `runsv`). The scanner `runsvdir` itself is not restarted: step 37 reboots the machine | 779, 43, 0, 29; 1900 to 1879 kB (one usable chunk); -70.4 [-168.9, +28.1] |
| runit-111 | 50 / 50 | 0 | 7 | not noticed | **yes** (12 of 12 ok); step 37: stage 2 restarted by runit, `runsvdir` and all `runsv` come back | 647, 55, 0, 0; 1896 to 1841 kB; +0.2 [-29.3, +29.7]; 7831 lock-error lines |
| runit-111np | 50 / 50 | 0 | 6 | not noticed | **yes** (12 of 12 ok); step 37 as runit-111 | not run (53 lock-error lines) |
| b-go | 38 / 38 | 0 | 0 | noticed, 10.6 to 15.1 s | not applicable (no separate supervisors; the brain starts the services) | 702, 0, 0, 0; brain PSS 6242 to 6948 kB; +99.9 [-1084, +1284] |
| b-go-stats | not run as elimination | - | - | - | not applicable | 700, 0, 0, 0; 4140 to 4274 kB; -37.2 [-274, +199] |
| c-go | 50 / 48 | 1 | 2 | noticed, 9.3 to 13.7 s | **yes, as s6** (`s6-svscan` restarts `s6-supervise`); 1 of the supervisor-kill events ended at the brain's give-up and a stuck state (step 33) | stopped after 14 minutes: 2 stuck states |
| c-elixir | 50 / 47 | 2 | 2 | noticed, 14.1 to 15.4 s | **yes, as s6**; same 1 event ended at the give-up (step 33) | stopped: 1 stuck state, then a logger loop |
| dinit (first setup) | 36 / 20 | 8 | 6 | not noticed | not applicable (no separate supervisors) | not run |
| dinit-par / dinit-sm | 36 / 20, 36 / 13 | 8 / 8 | 4 / 0 | not noticed | not applicable | not run |
| dinit-ms / dinit-wf | 36 / 31, 36 / 33 | 5 / 3 | 3 / 3 | not noticed | not applicable | not run |
| openrc (first setup) | 48 / 46 | 2 | 20 | not noticed | **no** (the 6 follow-up kills of the service after its supervisor was killed: 6 of 6 needed a person) | not run |
| openrc-par | 48 / 48 | 0 | 12 | not noticed | **no** (6 of 6, steps 31 to 36) | not run |
| openrc-doc | 48 / 48 | 0 | 6 | noticed, 19.4 to 20.2 s | **no** (6 of 6, steps 31 to 36; no documented option exists) | 569, 53, 0, 0; 2988 to 2977 kB; +33.5 [-50.2, +117.2] |
| s6rc-par-wd (compositor + hubd probes, not the owner's rule) | 50 / 50 | 0 | 5 | not noticed (watcher did not fire) | **yes** (12 of 12 ok) | not run |
| runit-wd (compositor + hubd probes, not the owner's rule) | 52 / 52 | 0 | 5 | not noticed (one false kill in a crash storm) | **yes** (14 of 14 ok, includes scanner events) | not run |
| s6rc-par-wdc (one compositor probe only, not the owner's rule) | 50 / 50 | 0 | 1 | noticed after 3 kills, 36.5 to 38.0 s; the 4th hang capped | **yes** (12 of 12 ok) | not run |
| runit-wdc (one compositor probe only, not the owner's rule) | 52 / 52 | 0 | 3 | noticed after 3 kills, 35.8 to 36.9 s; the 4th to 6th capped | **yes** (14 of 14 ok) | not run |

## What I could not test (follow-up 3)
Real hardware and KVM timing; the real compositor, viewers and hubd (stand-ins only; the hubd stand-in answers while the compositor is frozen, which the real hubd may or may not do: UNKNOWN); a real second probe for the hang rule; the watcher under OpenRC and under b-go (not run, by the instruction: s6rc-par and runit only); a soak with the watcher; the kill budget returning after 10 minutes in a VM; the watcher and the conductor probing each other; a watcher killed while a hang is in progress; the 53 `svlogd` lock lines' cause; why OOM step 49 of `runit-wd` lost driftwm and hubd.

## Questions for the owner (follow-up 3)
1. The rule says "both probes failing". With the stand-ins a frozen compositor leaves the hubd probe answering, so the rule as written never fired (0 of 6 hangs); one crash storm made both probes fail for 30 s and the watcher killed a crash-looping compositor. Which should the product probe, and should the rule also fire when only the compositor probe fails (the `-wdc` variation)?
2. runit-111np has 0 stuck, 0 self-reboots and 53 `svlogd` lock lines at supervisor kills (none at the scanner kill). Does that count as clean enough to soak, or must it be 0 lines?
3. When the cap (3 kills in 10 minutes) is reached the compositor stays frozen until a person acts (it did: 65 s in the harness). Is that the intended end state, with the fixed message and recovery terminal as in HUB-OS.md?
4. A restarted watcher forgets its streak and its kill budget. Should the budget be kept outside the process (a file)? It changes what a killed watcher can do to the cap.
5. Should I run the watcher under OpenRC and under b-go too, to complete Item 4's "- (not run)" cells?

---

# Follow-up 4 (2026-10-12): the owner's rule with two compositor probes, the kill budget in a file, hang runs again, the svlogd lock lines (data only)

Order from the owner: items 1 to 5. **Data only: no recommendation, no ranking, no init written, the product watcher is not built.** Everything about the watcher is a **TEST STAND-IN, not the product watcher** (`tools/bench/init/prototypes/watcher/`). One virtual machine at a time; no package installed (tools unpacked with `dpkg -x` into a scratch folder; the s6 suite built from the same source tarballs as before); `ulimit -c 0` in every shell; every push checked with `tools/check-push.sh` against the remote branch tip (`CHECK_PUSH_BASE=origin/init-comparison-step2`; the default base `origin/main` does not work for this branch, it shares no history with it and reports two old documentation lines (a udev property listing and a placeholder in a viewer command example) as credentials: not credentials, not mine, left alone). **No helper agents were used in this follow-up**; every claim below was read or measured by me. Logs: `tools/bench/init/results/step2/<candidate>/` (spam `SAMPLE` lines stripped). Labels: TESTED (a command was run, output in the log named), SOURCE (read in a file named), BELIEVED, UNKNOWN.

New candidate names: `s6rc-par-wd2` (= `s6rc-par` plus the follow-up 4 watcher as an s6-rc long-run service) and `runit-111-wd2` (= `runit-111` plus the same watcher as a runit service). The rebuilt tools (QEMU 8.2.2, kernel 6.8.0-146, busybox, runit 2.1.2-59ubuntu1, skalibs 2.15.1.0, execline 2.9.9.2, s6 2.15.1.0, s6-rc 0.7.0.0) are the same versions as before.

## Follow-up 4: owner decisions (recorded in plain words)

1. The second probe is the Wayland `wl_display.sync` round trip, and a round fails only when **both** compositor probes fail. No compositor-only variant is wanted for the product; the `-wdc` rows stay only as an already-measured comparison.
2. Do not soak `runit-111np` until the cause of the 53 svlogd "unable to lock" lines is known. If the cause is found and no log line is lost, run one 2-hour soak chunk of `runit-111np` and report it. If the cause is not found, or log lines are lost, stop and report.
3. When the cap (3 kills in 10 minutes) is reached, the compositor stays frozen until a person acts. This is acceptable only if a clear message is shown. The hub still never reboots. For this test the watcher writes a one-line state file (for example `/run/hubos/watcher.state` containing `capped <time>`) when the cap blocks a kill and removes it when kills are allowed again. No bar or panel is built; the file is only the hook the bar would read.
4. The kill budget is kept outside the process, in a file in the in-memory area (`/run`), so a restarted watcher does not get a fresh budget.
5. No OpenRC or b-go watcher runs now. Their cost-table cells stay empty and say "not run".

## Item 1: the probes of the test watcher

**Does the stand-in compositor answer a Wayland `wl_display.sync`? No (SOURCE `hubsim/svc.go` before the change).** It spoke only text lines (`hello NAME`, `state`) on its socket; its handler read up to a newline and a `wl_display.sync` message (12 binary bytes, none of them a newline) would have been read and never answered. I did not run that (it is clear from the source).

**What I added to the STAND-IN compositor (not to any init), flagged as a stand-in change.** In `compositor()` (`tools/bench/init/hubsim/svc.go`), in the same per-connection handler loop as `state`, before it reads a text line: it peeks at the first byte; if it is `0x01` (a Wayland message to object 1, `wl_display`), it reads the 12-byte message, applies **the same `hung` check as the `state` handler** (a hung compositor answers neither), and, if the opcode is 0 (`sync`), writes `wl_display.delete_id` and `wl_callback.done` for the callback id it was given (24 bytes). Text requests start with a letter, so they cannot be mistaken for it. Nothing else changed in how the stand-in behaves. SIGSTOP freezes the process (both answers stop); the busy loop and the new deadlock fault set the `hung` flag, so both answers stop (TESTED: all hang shapes below are noticed by the watcher). Limits, stated plainly: the stand-in is not a Wayland compositor (it understands this one request); both probes go to the same socket and the same handler, so in the VM **they cannot fail separately**: the VM runs cannot show what requiring "both" buys, only the unit tests do; in the real driftwm the two probes are two sockets answered by one event-loop thread (SOURCE `hub-stability.md` 9.1; the real compositor was not run here).

**The rule, as built (SOURCE `prototypes/watcher/main.go`).** Per round (one per second) the watcher asks, in parallel, driftwm's `state` request and a hand-written Wayland `wl_display.sync` (limit 1 s each; section 9.1 names 2 s for the sync, a setting) and also pings hubd. A round fails only when **both compositor probes** fail. It kills (SIGKILL by process name) after 4 failed rounds in a row **and** at least 30 s since the first failed round of the streak, at most 3 kills in 10 minutes, and never reboots anything; it restarts nothing (the init does). **The hubd ping stays in the log as `hubd(info only)` and takes no part in the decision.** Before every kill it logs the evidence (State, wchan, CPU ticks, age of the process; this is from the re-runs of item 3: the images of the first runs lacked it, see there), and when no process of that name exists at the moment of a kill it says so (the kill still counts against the budget).

**Unit tests** (`go test ./tools/bench/init/prototypes/watcher/`, 9 tests pass): both probes failing fails the round; one probe failing (either) does not, for 300 rounds; an answer from either probe resets the streak; a kill needs 4 rounds and 30 s (and 30 s without 4 rounds is not enough); 3 kills in 10 minutes, the 4th is capped, the budget is back 600 s after the first kill; the sync probe against a fake server speaking the wire format (answers, silence, no socket).

README: `prototypes/watcher/README.md` states in one line which probes it uses and that the rule is the owner's section 9.1 rule.

## Item 2: the kill budget in a file

**What was built (SOURCE `prototypes/watcher/budget.go`).** The times of the kills made in the last 10 minutes are in `/run/hubos/watcher.budget`. Every kill is recorded **before** the SIGKILL is sent, by writing a temporary file in the same directory, flushing it, and renaming it over the real file. **Clock: `CLOCK_BOOTTIME`** (time since the kernel started), read with `clock_gettime`. Why: it is not changed when someone sets the wall clock (the fault suite sets it back an hour and forward a day, and a restarted watcher would otherwise see kill times in the future or a day old); a new process reads the same clock (Go's own monotonic reading does not survive a restart); it keeps counting if the machine sleeps; it restarts from zero only at a reboot, which also empties `/run` where the file lives, so a file and its clock always belong to the same boot. On start the watcher reads the file; a **missing** file or a **damaged** one (wrong header, a line that is not a time, a time in the future of this boot, wrong or missing `end N` line, text after it, more than 1000 entries) gives an empty budget and a log line saying so; it never crashes on the file. Entries older than 10 minutes are dropped at load and while running. **The 30-second streak is not kept**: a restarted watcher counts the 30 s again from its own first failed round (SOURCE; a restart during the 30 s of a hang was not tested).

**State file (decision 3).** When the cap blocks a kill the watcher writes `capped <UTC time>` (atomically) to `/run/hubos/watcher.state`; it removes the file in the first round in which the budget has room again. No bar, no panel.

**Unit tests** (same test run): the budget survives a restart (a new budget object, as a new process would make, after each kill sees all kills; a 4th hang after a restart is capped; the budget is back 631 s after the first kill); a damaged file does not crash (12 kinds of damage, a directory instead of the file, an unwritable place: the write error is reported and the kill still counts in memory); entries of exactly 10 minutes and older drop out, at load and while running, and no temporary file is left behind; the state-file content and removal.

**VM test (TESTED; `results/step2/s6rc-par-wd2/s6rc-par-wd2-budget-1.log`, `results/step2/runit-111-wd2/runit-111-wd2-budget-1.log`; the harness suite `budget`).** Three hangs (SIGSTOP, busy loop, deadlock), each killed by the watcher; **after each kill the watcher itself was killed (SIGKILL)** and the init restarted it; then a 4th hang (SIGSTOP) with no help; then a 10-minute wait; then one more hang.

| | s6rc-par-wd2 | runit-111-wd2 |
|---|---|---|
| Hangs 1 to 3: fault to kill / fault to desktop back | 32.5, 32.6, 32.3 s / 38.1, 37.3, 37.8 s | 32.6, 32.5, 32.6 s / 36.4, 36.4, 36.3 s |
| Watcher killed after each kill: init restarted it in | 1.14, 1.15, 1.10 s | 0.23, 0.21, 0.22 s |
| The restarted watcher's log line | `budget file read: N kills in the last 10m kept` with N = 1, 2, 3 | same, N = 1, 2, 3 |
| Budget file after hang 3 | 3 entries | 3 entries |
| 4th hang: state file appeared after | 32.6 s: `capped 2026-10-12T19:01:33Z`; compositor State `T` (stopped) | 32.6 s: `capped ...19:28:49Z`; State `T` |
| 20 s later; watcher killed once more during the cap | still frozen, file still there; after the restart the file and the 3 entries were unchanged | same |
| How long the compositor stayed frozen with nobody helping | **485.4 s**, then the watcher killed it | **492.4 s**, then the watcher killed it |
| At that moment | state file removed (same second as the kill, log: `STATE kills are allowed again`); desktop back 5.2 s later | state file removed; desktop back 3.7 s later |
| More than 10 minutes after the last kill, a further hang (busy loop, then deadlock in the log) | killed at 33.1 s, desktop back after 37.9 s, state file absent, budget file 1 entry | killed at 32.6 s, desktop back after 37.2 s, absent, 1 entry |

**What this shows, without a judgement.** (a) A watcher killed between kills does not get a fresh budget: the 4th hang was capped in both inits (TESTED). (b) The budget returned by itself 10 minutes after the first kill (the first kill was at 51.7 s of uptime, the frozen compositor was killed at 654 s on s6, at 649 s on runit): **the compositor did not stay frozen until a person acted; it stayed frozen until the oldest kill was 10 minutes old**, because the watcher's streak of failed rounds goes on during the cap and the first round with room in the budget kills. Decision 3 as worded says "stays frozen until a person acts". The cap as built is a window, not a latch (see question 1). (c) With a person acting at once (the full fault suite below: hangs 4 to 6 fixed by the harness after 60 s), the state file stays until the budget has room, **even though the desktop is back**: it still said `capped` at the end of each of hangs 4, 5 and 6, minutes after the first of them was fixed and while the compositor answered (TESTED, `faults` logs, column `watcher state file` in the hang tables; see question 2). (d) A restart of the watcher during the cap did not change anything because the cap was already reached; a restart during the 30 s of a hang before the cap was **not tested** (the streak would start again: the kill would come up to 30 s later). (e) These two runs used the images built before the evidence log lines and the numbered heartbeat lines were added; nothing else differs.

## Item 4: the svlogd "unable to lock directory" lines of runit-111np

**Answer in one line.** The 53 lines come from **two `svlogd` processes on one log directory at the same moment** after a per-service supervisor (`runsv`) was killed: the old `svlogd` (still alive) holds the lock, the new one cannot take it. In the lock-probe runs **no log line was lost and none was doubled**. The 2-hour soak then showed a **second source** of lock lines (a log process that sits paused because `/var` was full and keeps the lock), which I did not find the cause of.

**SOURCE (runit 2.1.2, upstream `runsv.c`, `svlogd.c`, `runsvdir.c`, from the source tarball `runit_2.1.2.orig.tar.gz` at archive.ubuntu.com, unpacked into a scratch folder; the Debian patches of 2.1.2-59ubuntu1 that touch `svlogd.c` (0012, 0014, 0015, 0018) change spin-locking, declarations, indentation and leaks, none of them the lock call: I checked their changed lines for the word "lock")**:
- `svlogd.c` `logdir_open` (lines 403 to 424): opens `<dir>/lock` and takes an **exclusive non-blocking `flock`**; if that fails it prints `warning: unable to lock directory: <dir>` (the "temporary failure" text is the errno for would-block), and `logdirs_reopen` (line 564 to 574) then calls `fatalx("no functional log directories.")`: the second `svlogd` exits. The lock belongs to the open file, so it lives as long as the first `svlogd`.
- `runsv.c`: each `runsv` makes **its own pipe** (`pipe(logpipe)`, line 431) between the service and the log service and keeps both ends open itself (lines 433, 434; closed only at exit, lines 601, 602). It starts the log service first and then the service (lines 523 to 527). A log service that exits is started again after at least 1 second (lines 575 to 585).
- Killing `runsv` with SIGKILL does not kill its children (nothing in `runsv` does that on SIGKILL), so the old service and the old `svlogd` go on, with the old pipe. `runsvdir` looks at its directory every 5 seconds (`runsvdir.c` line 258) and starts a new `runsv` for the service whose `runsv` is gone; the new `runsv` makes a new pipe and starts a **second `svlogd`** (and a second service process).

**TESTED: the lock probe (`hubsim lockprobe`; logs `results/step2/runit-111np/runit-111np-lockprobe-41.log` (3 reps) and `-42.log` (12 reps)).** The probe kills `runsv` of driftwm or waybar, waits, kills the service (the same two steps as the supervisor-kill steps of the fault suite) and every 100 ms prints which `runsv`, `svlogd` and service processes exist and who holds the `flock` on the log directory's `lock` file (from `/proc/locks`). 15 reps x 2 services = 30 supervisor-kill steps (60 events, all recovered by themselves).
- **Which process holds the lock:** always the old `svlogd` (for example `svlogd=[368(S) 711(R)] lock_holder=368`: the old one, 368, holds it; 711 is the new one, running, about to fail).
- **Two `svlogd` on the same directory at that moment:** yes, seen in 10 samples of the 12-rep run alone (and in the 3-rep run). They are always one old (holder) and one new (failing).
- **Order:** t = 0 `runsv` killed (old `svlogd` and service keep running); about 2 to 5 s later (the 5-s scan of `runsvdir`) a new `runsv` appears, then its `svlogd` (fails, prints the warning and `svlogd: fatal: no functional log directories.`, is started again about once a second by the new `runsv`) and its service (the stand-in refuses to run twice: `already running, exiting`, started again about once a second); then the harness kills the old service, its pipe gets end-of-file, the old `svlogd` exits and releases the lock, and the next new `svlogd` takes it. If the 5-s scan comes **after** the old service and `svlogd` are gone, nothing overlaps and **no line is printed** (this is why only some of the steps print lines: in the 12-rep run 29 warning lines for 24 steps, in the 3-rep run 4 lines for 6 steps).
- **Lost or doubled log lines: none (TESTED).** The stand-in numbers its heartbeat lines (`pid=` and `seq=`); after the runs the log directories were printed to the console and `logcheck.py` checked them: for every service process (4 driftwm and 7 waybar processes in the 3-rep run, more in the 12-rep run) the numbers run from 1 to the last without a gap and without a repeat (`TOTAL gaps inside the numbered runs: 0; repeated lines: 0`, both runs). The lines the new instance wrote to the new pipe while no `svlogd` could read it (for example `already running, exiting`) are not lost either: they wait in the pipe (which `runsv` keeps open) and appear when the new `svlogd` starts; **they carry a later time stamp than when they were written** because `svlogd` stamps a line when it reads it (SOURCE `svlogd.c` `-tt`; seen in the logs). Limit: only the heartbeat lines are numbered; the lines at the very end of a killed process (up to 0.5 s) cannot be told from "the process stopped"; the pipe holds 64 kB, the runs never came near it.
- Why the stock `runit` rerun had 0 lock lines in follow-up 3 (earlier "UNKNOWN"): not examined further. BELIEVED from the mechanism: it depends on the timing of the 5-s scan against the harness steps, so a single run can have none.

**The 2-hour soak (`results/step2/soak/runit-111np-fu4chunk1.log`; decision 2 allowed it: cause found, no line lost in the probe).** Result, reported as it came: 290 events, **34 needed a person, 1 stuck state, 0 self-reboots**; 4161 lines `unable to lock directory`, 0 lines of `runsv ... unable to lock supervise/lock` (runit-111 had those). The 34 are all hangs of the compositor (25 SIGSTOP, 9 busy loop; no watcher in this image, so nobody notices a hang: the same as runit-111, class H). The one stuck state is at the very end (7182 s): a slow seatd start (dep-slow) after which `waybar` was "not running" and stayed so after the harness's own restart; at that moment the log directory of waybar showed the lock error and `out of disk space`. **Where the lock lines are:** 194 in the first 5800 s (the mechanism above) and **3967 in the last 1500 s**, when the stand-in's small `/var` (4 MB, there to test a full disk) was full and several `svlogd` processes printed `pausing: unable to write to current: ... out of disk space` about every 3 seconds (2747 such lines in all). A `svlogd` that is paused for lack of space holds its lock, so a new log service started in the meantime (after a kill of a service or its `runsv`) cannot get it and fails every second (BELIEVED from the source and the log; not isolated). **What I did not find (UNKNOWN):** why `/var` stayed full from about 5800 s to the end (the fault suite's own disk-full step lasts 15 s; BELIEVED: the logs of six services at up to 6 files of 100 kB each, about 3.6 MB, plus partial files, come close to the 4 MB of the stand-in's `/var`; no `du` was taken), and therefore whether log lines were lost or delayed in that period (UNKNOWN: the logs were not dumped; the lines of a service whose log service cannot start wait in the pipe, SOURCE `runsv.c`). So: **decision 2 was applied** (cause found for the 53 lines; no loss in the probe; soak run once). The soak does not show a clean runit-111np: 1 stuck state and a long lock-line period, linked to a full `/var` that I did not explain.

## Item 3: the hang runs again, with the owner's rule and the budget file

**Setup.** `candidates/s6rc-par-wd2/` and `candidates/runit-111-wd2/` (the images of follow-up 3's `-wd` candidates with the follow-up 4 watcher; the watcher is supervised like any other service; it is not given a restart rule by hand). Runs: the full fault suite (`faults`; with the same 3 repetitions of the kill steps as in follow-up 3 so that the step numbers agree), a **spaced hang run** (`hangs`: 9 hangs in three groups of three, the shapes SIGSTOP, busy loop and **deadlock** shuffled by the seed, each fault after a random pause of up to one second so that it falls at a random point of the watcher's one-second round; before each further group the harness waits until 10 minutes and 15 seconds after the first fault of the previous group so that every hang finds a full budget), a crash-storm run (`storm`: 3 storms of 40 s), an out-of-memory run (`oom`: 3 out-of-memory steps per boot), and short re-runs with the evidence log lines. Seeds: `s6rc-par-wd2` 51, 52, 53, 54, 55, 56, 57, 75; `runit-111-wd2` 61, 62, 63, 64, 65, 66, 67, 85 (new, none used before). "Deadlock" is new: the harness's step list of follow-up 3 had only SIGSTOP and the busy loop; the owner's list names the deadlock, so I added a fault that makes the stand-in stop answering while using no CPU (State `S`, `futex_wait_queue`), flagged as a harness change. The first run of the full suite used 5 repetitions of the kill steps (more kill steps; the hang steps are then steps 31 to 36), an error of mine in the queue file that I noticed afterwards; it is kept (`*-faults-51.log`, `*-faults-61.log`), and the 3-repetition runs are the ones in the table of the full suites.

**A harness fault found and fixed on the way (TESTED).** In the first 3-repetition run of the full suite on `s6rc-par-wd2` (`results/step2/s6rc-par-wd2/s6rc-par-wd2-faults-74-harness-died.log`) the injector stopped at step 36 and the stage-2 service of s6-rc started again: the injector had called `kill(pidOf("waybar"), SIGKILL)` at a moment when waybar had no process (the supervisor-kill step), `pidOf` gave 0, and `kill(0, SIGKILL)` signals the sender's whole process group: the injector and the processes grouped with it died (SOURCE `hubsim/chaos.go`, 17 such calls, and the kill(2) manual). Every earlier run of this follow-up had finished its suite. The fix is one helper that ignores a pid of 0 or less (a log line `KILLP skipped` says when it did). The affected run is **not used** for any number below; the 3-repetition runs were done again (seeds 75 and 85).

### Hang table (TESTED; `tools/bench/init/hangtable.py` prints it per log)

The two probes and the rule are the owner's (section 9.1). "Fault to kill" is from the moment the fault took effect (for the busy loop and deadlock the stand-in needs up to 0.5 s to arm; the arm delay is in the table output, 0.00 to 0.50 s) to the moment the compositor's process id changed or disappeared. "Fault to desktop back" is until the harness sees the compositor answering, with waybar and hubd registered at it again (the stand-in windows), twice in a row. "Windows back" = those two stand-in clients had re-registered at the new compositor. "Needed a person" = nothing recovered within 60 s and the harness acted.

| | s6rc-par-wd2 | runit-111-wd2 |
|---|---|---|
| Hangs killed by the watcher alone (full budget available) | **19 of 19** (SIGSTOP 7, busy loop 6, deadlock 6) | **19 of 19** (SIGSTOP 7, busy loop 6, deadlock 6) |
| Fault to kill, all 19 (median) | 31.9 to 33.4 s (32.4) | 31.9 to 32.6 s (32.3) |
| Fault to desktop back, all 19 (median) | 36.5 to 38.9 s (37.7) | 35.6 to 37.2 s (36.5) |
| by shape, fault to desktop back | SIGSTOP 37.3 to 38.1; busy loop 36.5 to 38.9; deadlock 36.9 to 38.7 | SIGSTOP 35.7 to 37.0; busy loop 35.6 to 36.6; deadlock 35.6 to 37.2 |
| A stand-in window (waybar and hubd) came back | 19 of 19 | 19 of 19 |
| Needed a person | 0 of 19 | 0 of 19 |
| Hangs 4, 5, 6 of the full suite (70 s apart, so the cap of 3 kills in 10 minutes was reached) | capped: not killed; the harness acted after 60 s (**3 of 3 needed a person**, fixed in 64.8, 65.9, 65.2 s from the fault) | same: 3 of 3 capped and needed a person (63.2, 63.3, 63.4 s) |
| The 19 are from | `hangs` run seed 52 (9), `hangs` seed 57 (3, with evidence lines), the first 3 hangs of the full suite (3), the `budget` run (4) | seeds 62 (9), 67 (3), full suite (3), `budget` run (4) |

Evidence the watcher logs before each kill (from the evidence re-runs, seeds 57 and 67; each shape has a different signature, as `hub-stability.md` 9.1 says): SIGSTOP `state=T wchan=do_signal_stop`, 109 and 148 CPU ticks, age 42 s and 44 s; deadlock `state=S wchan=futex_wait_queue`, 125 and 129 ticks (they do not move), age 41 s; busy loop 2940 and 3194 ticks (against about 130 for the others; the process shows `S` with `futex_wait_queue` on s6 and `R` with `wchan=0` on runit, because the line shows the main thread's state, not the spinning thread's), age 40 to 41 s.

**Harness artefact in follow-up 3's table (correction).** Rows of follow-up 3 that read "answered by itself in 0.11 s" (step 22 of `s6rc-par-wd`, step 20 of `runit-wd`) were not recoveries: the busy-loop fault is switched on by the stand-in's heartbeat tick (every 500 ms) and the harness looked at the desktop before the fault was in effect. The new harness waits until the fault has taken effect (`arm_delay` in the log).

**Time from fault to kill and to desktop against b-go (the owner's request; no explanation attached).**

| | fault to desktop back | where it comes from |
|---|---|---|
| b-go (the brain's own probe: 3 failures in a row, 8 s grace) | 10.6 to 15.1 s | earlier data (follow-up 2, item 9 and follow-up 3, item 3) |
| watcher, `s6rc-par-wd2` | 36.5 to 38.9 s (kill after 31.9 to 33.4 s) | this follow-up, 19 hangs |
| watcher, `runit-111-wd2` | 35.6 to 37.2 s (kill after 31.9 to 32.6 s) | this follow-up, 19 hangs |

The watcher's own timing is the 30-second window of the rule plus the restart (3.7 to 5.8 s after the kill in these runs). The 30 s is deliberate: it avoids killing a slow but alive compositor, the KDE mistake of `hub-stability.md` section 9.1.

### The crash storm (TESTED; `storm` runs, seeds 53, 56 (s6) and 63, 66 (runit); logs with evidence lines: 56 and 66)

The step: a stand-in compositor that crashes at every start (a flag file) for 40 s, then the flag is removed. The init restarts it 22 to 24 times in the 40 s (about 1.7 s per start). Both compositor probes fail all that time.
- **The watcher still killed a crash-looping compositor: 12 of 12 storms** (3 storms in each of the 4 spaced runs, budget empty at the start of each run): after 30 s of failed rounds it decided to kill, as the rule says, and used one of its 3 kills.
- What the kill hit: in 3 of those 12 there was a process named driftwm at that moment (evidence lines of seeds 56 and 66: **age 0.16 s** and **0.54 s**, State `R`, 10 and 50 CPU ticks; seed 53: one kill with a process, age not logged), in the other 9 **no process of that name existed** (the log says `KILL driftwm: no process of that name exists at this moment (the kill counts against the budget)`). The process that was hit was one the init had started less than a second before; the init was already restarting the compositor by itself.
- **Would I call it a false kill? The evidence, not a recommendation:** the compositor was not hung (it started and exited by itself 22 to 24 times in 40 s, the stand-in exits about 50 ms after it starts when the crash flag is set), the init was already restarting it, and the kill changed nothing the init was not doing (9 of 12 found nothing to kill). It was a kill by the rule as written (both probes failing for 30 s: true). It cost the kill budget: three storms in a row used all 3 kills, so a real hang within the next 10 minutes would be capped (that is what the rule says; not tested in this order). In follow-up 3 the same step under the "compositor + hubd probes" pairing killed once (runit) or not at all (s6, 29.0 s, one second short of the window); with the owner's pairing it fires in every storm.
- In the full suite the storm came after the cap was reached by the hangs, so the storm only produced `CAP` lines (226 and 115).

### Out of memory (TESTED; step `oom`)

| | s6rc-par-wd2 | runit-111-wd2 |
|---|---|---|
| Out-of-memory steps run on this image (3-step runs: 2 boots; full suites) | 10 (2 in the full suite seed 51, 3 + 3 in seeds 54 and 55, 2 in the full suite seed 75) | 10 (2 in seed 61, 3 + 3 in seeds 64 and 65, 2 in seed 85) |
| driftwm or hubd lost (process id changed) | **none in 10 of 10** (`services_lost=[]`, hog killed each time) | **none in 10 of 10** (`services_lost=[]`, hog killed each time) |

**Follow-up 3's case (runit-wd lost driftwm and hubd once, then driftwm; the same image in the compositor-only run lost nothing): not reproduced, and the cause is UNKNOWN.** What the logs show: in follow-up 3's `runit-wd` run, steps 49 and 50 ran 25 to 36 s after the scanner kill, while the restarted stage 2 was producing `unable to lock supervise/lock` lines and new `runsv` processes every second; the harness's "lost" is only "the process id differs from before", so a restart caused by that churn would count too. In this follow-up the same churn was present at the out-of-memory steps of the `runit-111-wd2` full suite (lock lines at the time) and nothing was lost. No kernel "Out of memory: Killed process" line is in any of these logs (the kernel's console level was quiet), so I cannot say which processes the kernel killed in follow-up 3 or whether any was driftwm or hubd. The processes have an out-of-memory score of -900 (driftwm, hubd) in all runs, as before.

## Corrected table: the watcher runs side by side (data only; no ranking)

"Rule" in the header is what the watcher required to fail. Rows 1 and 3 of follow-up 3 are the old runs (relabelled; numbers as measured then); the last two columns are the owner's rule (probes: `state` and `wl_display.sync`, both must fail; 4 rounds and 30 s; 3 kills in 10 minutes; budget in a file). All use the same fault suite (3 repetitions; hangs are steps 19 to 24).

| | s6rc-par-wd: compositor + hubd probes (not the owner's rule) | s6rc-par-wdc: one compositor probe only (not the owner's rule) | **s6rc-par-wd2: owner's rule** | runit-wd: compositor + hubd probes (not the owner's rule) | runit-wdc: one compositor probe only (not the owner's rule) | **runit-111-wd2: owner's rule** |
|---|---|---|---|---|---|---|
| Events defined / ran | 50 / 50 | 50 / 50 | 50 / 50 | 52 / 52 | 52 / 52 | 52 / 52 |
| Stuck states / self-reboots | 0 / 0 | 0 / 0 | 0 / 0 | 0 / 0 | 0 / 0 | 0 / 0 |
| Needed a person | 5 (class H) | 1 | **3** (steps 22, 23, 24: capped) | 5 | 3 | **3** (steps 22, 23, 24: capped) |
| Hangs 19 to 24 (6 per suite): killed by the watcher | 0 of 6 | 3 of 6 (then the cap) | **3 of 6** (then the cap) | 0 of 6 | 3 of 6 | **3 of 6** (then the cap) |
| Fault to kill / fault to desktop back, hangs that were killed | no kill | 36.5 to 38.0 s to desktop | **32.4 to 32.7 s / 37.0 to 38.4 s** | no kill | 35.8 to 36.9 s to desktop | **31.9 to 32.8 s / 35.9 to 36.5 s** |
| In the spaced hang run (budget full each time) | not run | not run | **9 of 9 killed, 0 persons; 31.9 to 33.4 s / 36.5 to 38.9 s** (all 19 killed hangs of this follow-up: 19 of 19, 0 persons) | not run | not run | **9 of 9 killed, 0 persons; 31.9 to 32.6 s / 35.6 to 37.2 s** (19 of 19) |
| Crash storm (driftwm, 40 s) | one kill in runit-wd only (31 failed rounds), none in s6rc-par-wd | none (cap refused) | **a kill decision in 12 of 12 spaced storms (both inits together: 6 per init), 9 of 12 found no process**; in the full suite capped | 1 kill | none | see the s6 column (storm runs: both inits) |
| Watcher killed (steps 44, 45): init restarted it in | 1.07, 1.08 s | 1.15, 1.10 s | 1.10, 1.09 s | 0.21, 0.21 s | 0.21, 0.21 s | 0.21, 0.21 s |
| Budget kept in a file / state file | no (memory) | no | **yes / yes** (TESTED: survives a watcher kill, returns after 10 minutes) | no | no | **yes / yes** |
| Lock lines (`unable to lock`) | n/a | n/a | 0 | 154 | 100 | 111 (18 svlogd, 93 runsv) |
| Out of memory (steps 42, 43 and extra runs): driftwm or hubd lost | not seen | not seen | none in 10 of 10 | **lost in one run (UNKNOWN why)** | not seen | none in 10 of 10 |

(The crash-storm cell of the owner's rule is one number for both inits: 12 of 12 storms in the 4 spaced runs, 6 per init; the full suite's storms came after the cap was reached.)

## Updated final table, follow-up 4 (rows that changed or are new; all other rows are unchanged, see follow-up 3's table and `FINAL-TABLES.md`)

| Candidate | Events defined / ran | Stuck | Needed a person | SIGSTOP of the compositor | Restarts a killed supervisor by itself | Soak: events, needed a person, stuck, self-reboots; memory |
|---|---|---|---|---|---|---|
| s6rc-par-wd (compositor + hubd probes, not the owner's rule) | 50 / 50 | 0 | 5 | not noticed (watcher did not fire) | yes (12 of 12 ok) | not run |
| runit-wd (compositor + hubd probes, not the owner's rule) | 52 / 52 | 0 | 5 | not noticed (one kill in a crash storm) | yes (14 of 14 ok) | not run |
| s6rc-par-wdc (one compositor probe only, not the owner's rule) | 50 / 50 | 0 | 1 | noticed after 3 kills, 36.5 to 38.0 s | yes (12 of 12 ok) | not run |
| runit-wdc (one compositor probe only, not the owner's rule) | 52 / 52 | 0 | 3 | noticed after 3 kills, 35.8 to 36.9 s | yes (14 of 14 ok) | not run |
| **s6rc-par-wd2** (owner's rule) | 50 / 50 | 0 | 3 (capped hangs 4 to 6) | noticed: killed at 32.4 to 32.7 s, desktop 37.0 to 38.4 s in the suite; 19 of 19 in all runs with a full budget | yes (12 of 12 ok) | not run |
| **runit-111-wd2** (owner's rule) | 52 / 52 | 0 | 3 (capped hangs 4 to 6) | noticed: killed at 31.9 to 32.8 s, desktop 35.9 to 36.5 s; 19 of 19 | yes (14 of 14 ok) | not run |
| runit-111np | 50 / 50 | 0 | 6 | not noticed | yes (12 of 12 ok) | **2 h, one chunk: 290 events, 34 needed a person (all hangs), 1 stuck, 0 self-reboots**; 4161 lock lines (194 before 5800 s, 3967 while `/var` was full); slope -87.8 kB per 1000 restarts (std.err 37.2; 749 restarts) |

Cost table (follow-up 3, item 4): decision 5 of the owner leaves the OpenRC and b-go cells "not run"; they stay so. For `s6rc-par` and `runit-111` the follow-up 4 watcher adds, over follow-up 3's: the budget file `/run/hubos/watcher.budget` and the state file `/run/hubos/watcher.state` (both written by the watcher itself, nothing for the init to configure; a restarted watcher is a new process of the same service), and nothing else (TESTED: `s6rc-par-wd2` and `runit-111-wd2`, watcher killed in the budget runs and in steps 44 and 45 of the suites, restarted by the init every time: 1.1 s on s6, 0.2 s on runit).

## What could not be tested (follow-up 4)

- The real driftwm, its real IPC socket and Wayland socket, and the real hubd (stand-ins only). The sync probe was answered by a stand-in that understands that one request; whether the real driftwm answers `wl_display.sync` from the same thread as `state` is SOURCE (`hub-stability.md` 9.1), not tested here.
- What requiring "both" probes buys: in the VM the two probes share one socket and one handler and cannot fail separately; only the unit tests show the "one probe failing does not fail the round" behaviour.
- A watcher killed during the 30 seconds of a hang (the streak starts again; the kill would come later), and a kill budget across a reboot (the file is in `/run`, so it is empty after a reboot: SOURCE, not tested).
- A slow but alive compositor (a stall of 10 to 25 s that ends by itself), the case the 30 s window is for; no such fault exists in the suite.
- The watcher under OpenRC and under b-go (not run, owner decision 5); a soak with the watcher; the hang rule inside the product.
- Why `runit-wd` lost driftwm and hubd at an out-of-memory step in follow-up 3 (UNKNOWN; 20 steps on the two new images lost nothing; no kernel line was captured).
- Why `/var` (the stand-in's 4 MB) stayed full from about 5800 s in the runit-111np soak, and whether log lines were lost or delayed during that period (UNKNOWN); the other long soaks did not show it (66 and 48 "out of disk space" lines in the two chunks of runit-111).
- PID 1 killed or wedged and the trial boots for `s6rc-par-wd2` and `runit-111-wd2` (not run).
- Real hardware and KVM timing; the first 3-repetition run of `s6rc-par-wd2` (invalid: the injector killed itself, see item 3).

## Questions for the owner (follow-up 4)

Each has my suggestion; a suggestion is my opinion, not a measurement, and none of them is about which init or where the watcher lives.

1. **The cap is a window, not a latch.** Decision 3 says the compositor stays frozen until a person acts; the watcher built for the test kills the frozen compositor again as soon as the oldest kill is 10 minutes old (485 s and 492 s in the two VM runs). Which is meant: a window (as built), or a latch that stays until a person clears it? *Suggestion:* the latch, as decision 3 is worded (with the message shown): with the window the compositor is replaced by itself 10 minutes later and the message disappears, so a persistent fault repeats every 10 minutes with the message coming and going.
2. **When should the state file go?** As built it is removed when the budget has room again, so it said `capped` for minutes after a person had already fixed the compositor. *Suggestion:* also remove it when both probes answer again (the message then always describes a present fault), keeping the cap itself as is.
3. **A crash storm is killed by the rule in every storm (12 of 12), though nothing is hung** (9 of 12 kills found no process; the others hit a process younger than 1 s); each uses one of the 3 kills. Is that wanted? *Suggestion:* do not count a kill that found no process in the budget (a one-line change); otherwise leave it and accept that three storms in 10 minutes use the whole budget.
4. **Probe limit.** `hub-stability.md` 9.1 names 2 s for the sync and 5 s for `state` (hubd's own deadline); the watcher uses 1 s for both. Keep 1 s or take the document's numbers? *Suggestion:* the document's numbers, since a loaded hub is slower than this VM.
5. **Should the 30-second streak survive a watcher restart?** Now only the budget does. *Suggestion:* no (counting the 30 s again is the safe side).
6. **runit-111np:** its 2-hour soak had 1 stuck state and a long lock-line period linked to a full `/var` I did not explain (UNKNOWN). Soak it again? *Suggestion:* first one run with a larger stand-in `/var` and a `du` printed at the end, to see whether the full-disk state and the stuck state come from the stand-in's size; no further soak before that.

# Follow-up 5

Order from the owner: items 1 to 3. **Data only: no recommendation, no ranking, no init written, the product watcher is not built.** Everything about the watcher is a **TEST STAND-IN, not the product watcher** (`tools/bench/init/prototypes/watcher/`). Rules kept: a separate clone of the repository, new commits only, `tools/check-push.sh` before every push with `CHECK_PUSH_BASE=origin/init-comparison-step2`, one VM at a time, no package installed (tools unpacked with `dpkg -x` into a scratch folder, the s6 suite rebuilt from the same source versions), `ulimit -c 0` in every shell, no `kill` with a process id of 0 or less (the harness helper from follow-up 4 stays). In this document, strings that look like credentials or device-property lines are described in words. Labels: TESTED (a command was run, the output is in the log named), SOURCE (read in a file named), BELIEVED, UNKNOWN.

## Follow-up 5: owner decisions

1. **Latch, not a window.** When the cap is reached and the rule wants a 4th kill, the watcher stops killing and stays stopped until a person clears it with one command. The hub still never reboots. A message is shown while the compositor is failing (the state file, as in follow-up 4). The measured follow-up 4 behaviour (killed again after about 485 to 492 s) was the window version; those numbers stay as measured and that version is labelled "window".
2. The "capped" message (the state file) also goes away by itself in the first round in which both probes answer again. The latch itself stays until the person clears it.
3. **Kill only the same process that was failing.** At the first failed round of a streak the watcher records the identity of the compositor process (process id AND start time from `/proc/PID/stat`, so a reused id is not mistaken for the same process). At the kill moment it kills only if that identical process is still there. If the process is gone or is a different one (the init already restarted it), the watcher does NOT kill, does NOT count a kill, logs "SKIP: process changed", and starts a new streak with the new process's identity (the new process gets its own 30 s). A kill that finds no process is never counted.
4. Probe limits follow `hub-stability.md` section 9.1: the driftwm "state" request has a 5 s deadline, the Wayland `wl_display.sync` round trip has a 2 s limit (the follow-up 4 watcher used 1 s for both). The rule (4 rounds and 30 s) is unchanged.
5. The 30-second streak does NOT survive a watcher restart (only the budget file and the latch do).
6. `runit-111np` is NOT soaked again now. No soak was run in this follow-up.

The hang rule as the owner stated it for this run: two probes of the compositor, both must fail; 4 failed rounds in a row and at least 30 s since the first failed round; at most 3 kills in 10 minutes; never a reboot; the init restarts what the watcher kills.

**Environment of this follow-up (TESTED).** A fresh container; the tools were fetched again: QEMU 8.2.2 (without KVM, as before), kernel 6.8.0-146 (fetched from the `linux-signed` pool of the Ubuntu archive because the package index now points to -147; the -147 image was unpacked too but not used), busybox 1.36.1, runit 2.1.2-59ubuntu1, skalibs 2.15.1.0, execline 2.9.9.2, s6 2.15.1.0, s6-rc 0.7.0.0 (built from the skarnet.org tarballs). Timings come from a different physical host than follow-up 4 (UNKNOWN how much that matters; the VM runs under software emulation either way).

New candidates (TESTED, built and run): `s6rc-par-wd3` (= `s6rc-par` plus the follow-up 5 watcher as an s6-rc long-run service) and `runit-111-wd3` (= `runit-111` plus the same watcher as a runit service). Same images as the `-wd2` ones except for the watcher binary. The `-wd2` candidates, their logs and their rows are unchanged (their images were **not** rebuilt; to rebuild them the watcher folder must be taken from commit f130829, because `build.sh` copies whichever watcher is built from the folder). Seeds (all new): s6: 101 (slow), 102 (latch), 103 (latch c and d), 104 (spaced hangs), 105 (storms), 106 (full suite, 3 repetitions); runit: 111 to 116 in the same order. Logs: `tools/bench/init/results/step2/<candidate>/` (spam `SAMPLE` lines stripped; the watcher's lines appear twice in the logs, once from the console and once from standard error). **No helper agents were used in this follow-up**; every claim below was read or measured by me.

## Item 1: the test watcher (latch version)

**What was built (SOURCE `prototypes/watcher/`: `main.go`, `watcher.go`, `latch.go`, `budget.go`; README updated).**
- **Latch** (decision 1): when the rule wants a kill and 3 kills were already made in the last 10 minutes, the watcher writes `/run/hubos/watcher.latch` (temporary file, flush, rename) and stops killing for good until a person clears it. It reads the file at start and in every round. A damaged latch file counts as latched and is logged; it never crashes the watcher.
- **Person-command, test hook only:** `watcher reset` (a subcommand of the same binary) empties the budget file, removes the latch and removes the state file (in that order, so a running watcher never sees "no latch and a full budget"); a running watcher notices within one round. **The product's command is not designed.**
- **State file** (decisions 1 and 2): `capped <UTC time>` is written when the rule wants a kill and the latch (or a full budget) blocks it, and removed in the first round in which both probes answer, and by `reset`. A round in which only one probe answers does not remove it. The latch stays.
- **Identity rule** (decision 3): at the first failed round of a streak the process named driftwm is recorded (id and start time, field 22 of `/proc/PID/stat`); at the kill moment the watcher kills only that identical process; otherwise it logs `SKIP: process changed`, kills nothing, counts nothing and starts a new streak with whatever process exists then. A kill that fails because the process is gone is taken back out of the budget. If no process exists at the first failed round, the recorded identity is "none" and the kill moment always ends in a skip (BELIEVED to be the safe side; see question 2).
- **Probe limits** (decision 4): `state` 5 s, `wl_display.sync` 2 s. The hubd ping (information only) keeps 1 s.
- **Streak** (decision 5): not kept over a restart (only the budget file and the latch are).
- The follow-up 4 rule code is kept as the "window" version (`Rule.Round`, with its tests); the program uses the new `Watcher.Step`.

**Stand-in changes (flagged).** (1) Slow but alive compositor, 17 lines in `hubsim/svc.go` (`slowDelay`): while the file `driftwm.slow` exists in the fault folder, an answer to `state` and/or to the sync request is delayed by 3 to 4 s (file content "both" or "sync"); the process keeps running. (2) New harness suites in `hubsim/chaos.go` (`latch`, `latchc`, `hangsP`, `storm3`, `slow`); the HANG log line now also shows `latch=0/1`. (3) `event_counts.py` knows the two new candidates. Everything the harness does as "the person" is marked `PERSON-COMMAND` in the logs.

**Unit tests (TESTED; `go test ./tools/bench/init/prototypes/watcher/`, 22 tests pass; 10 old ones unchanged and passing, 12 new ones; I checked by hand that removing the identity check or the latch check makes the matching tests fail).** Latch set when a 4th kill is wanted; no kill while latched across 40 minutes of failed rounds (the budget has room after 10 minutes, the latch still blocks); latch survives a restart; reset clears latch and budget and the running watcher then kills within one round; state file appears, stays through a round in which only one probe answers, goes in the first round with both answering, comes back, and is removed by reset; identity rule (same id and start time kills; same id with another start time does not; another id does not; none does not; a process among others; none of these counted; the new process gets its own 30 s); a crash storm with a new process every second kills nothing; a kill that finds no process is taken back; the streak is not kept over a restart; a damaged latch file (four kinds, a directory, a 1 MiB file) is latched and logged; slow but alive (one probe answering for 600 rounds) never kills; probe limits (a `state` answer after 4 s counts as answered, a sync answer after 3 s counts as failed; the test takes 9 s). Not unit-tested: the real `killProc` kills with `SIGKILL` only after re-reading the start time; the check and the signal are not atomic (no pidfd), so a reuse of the id between them is possible in theory (SOURCE).

## Item 2: VM tests (TESTED; logs under `results/step2/<candidate>/`)

### 2a, 2b: the latch, and the latch survives a watcher kill (`*-latch-102.log`, `*-latch-112.log`)

Three hangs (SIGSTOP, busy loop, deadlock) each killed by the watcher alone; then a 4th hang (SIGSTOP), nobody helps. The harness sampled the compositor every 2 s for 961 s (about 16 minutes, longer than the 10-minute window of the cap) and checked three things: the same process, State `T` (stopped), the latch file and the state file present. At 120 s into the freeze the watcher was killed (SIGKILL).

| | s6rc-par-wd3 | runit-111-wd3 |
|---|---|---|
| 4th hang: state file appeared after | 35.3 s: `capped ...` | 35.3 s |
| Latch file / compositor | present / State `T`, same process | present / State `T`, same process |
| **Stayed frozen the whole time?** | **yes: 475 samples over 961 s, not frozen in 0, latch missing in 0, state file missing in 0** | **yes: 476 samples over 961 s, 0 / 0 / 0** |
| (b) Watcher killed (SIGKILL) while latched and frozen: init restarted it in | 1.08 s | 0.21 s |
| After the restart: latch, state file, budget file, frozen state | all byte-for-byte unchanged (the harness compared the strings) | same |
| Person-command (`watcher reset`, **played by the harness**, marked in the log) | budget emptied, latch and state file removed | same |
| Reset to kill / reset to desktop back | 2.13 s / 6.82 s | 0.38 s / 4.61 s |
| After the reset | state file absent, budget file 1 entry (the new kill), latch absent | same |

The window version had killed the frozen compositor at 485 s (s6) and 492 s (runit); here 961 s passed without a kill. The spread of reset-to-kill (0.38 to 4.59 s over four resets) is where the reset falls in the watcher's rounds, which are about 5 s long while the compositor is frozen (see the timing note below).

### 2c: the message goes by itself, the latch stays (`*-latchc-103.log`, `*-latchc-113.log`)

Same set-up (three kills, a 4th hang, latched). After 10 s of freeze the harness resumed the compositor (SIGCONT) **without** reset.

| | s6rc-par-wd3 | runit-111-wd3 |
|---|---|---|
| State file removed after the SIGCONT | **0.05 s** | **0.05 s** |
| Latch after that / desktop | still set / ok | still set / ok |
| Same compositor hung again (SIGSTOP of the same process, 5 s later): state file came back after | 35.6 s | 35.6 s |
| 120 s after the second hang | **not killed**, State `T`, same process, state file and latch present | same |
| Person-command, then reset to kill / to desktop | 4.59 s / 9.92 s | 4.44 s / 8.19 s |

The state file was gone within 0.05 s because the round that was waiting for the frozen compositor's answer ended at once when the process continued (SOURCE `main.go`: the next round's decision follows the probes). The latch was never touched by the removal of the state file.

### 2d: the streak across a watcher restart (same logs)

After the reset of 2c a new SIGSTOP hang; the watcher was killed 15.0 s into the streak.

| | s6rc-par-wd3 | runit-111-wd3 |
|---|---|---|
| Init restarted the watcher in | 1.11 s | 0.07 s |
| From the restart to the kill | 36.19 s | 36.26 s |
| Fault to kill / fault to desktop back | 52.32 s / 56.99 s | 51.34 s / 55.17 s |

The restarted watcher counted its own 30 s (expected about 30 s; the extra 6 s is the round length, see the timing note). Without a restart the same fault takes about 36 s to the kill.

### 2e: spaced hang run (`*-hangsP-104.log`, `*-hangsP-114.log`, 9 hangs each; new seeds; the harness ran the person-command after hangs 3 and 6 so the latch never blocked)

| | s6rc-par-wd3 | runit-111-wd3 |
|---|---|---|
| Hangs killed by the watcher alone | **9 of 9** (SIGSTOP 3, busy loop 3, deadlock 3) | **9 of 9** (3, 3, 3) |
| Fault to kill | 35.3 to 36.3 s | 35.3 to 36.8 s |
| Fault to desktop back | 39.9 to 41.4 s | 38.7 to 40.9 s |
| Stand-in windows (waybar, hubd) back | 9 of 9 | 9 of 9 |
| Needed a person | 0 of 9 | 0 of 9 |
| All hangs killed alone in all runs of this follow-up (latch, latch-c, spaced, storm and full-suite hangs) | 21 of 21: kill 35.3 to 36.3 s (median 35.9), desktop 39.7 to 41.4 s (median 40.5) | 21 of 21: kill 35.3 to 36.8 s (median 35.9), desktop 38.7 to 40.9 s (median 39.7) |

**Difference to follow-up 4's 19 of 19 (reported first, as asked):** every hang was killed (no change in the count), but the kill comes **about 3.5 s later**: 35.3 to 36.8 s instead of 31.9 to 33.4 s (s6) and 31.9 to 32.6 s (runit), and the desktop is back 38.7 to 41.4 s instead of 35.6 to 38.9 s. TESTED: the watcher's round log shows rounds about 5.0 s apart while the compositor is frozen (gaps 5.03, 5.05, 5.03 s in `s6rc-par-wd3-latch-102.log`). SOURCE `main.go`: a round waits for its probes, and the `state` probe now waits up to 5 s (follow-up 4: 1 s); the kill is decided after the round's probes end, from the time the round began. BELIEVED: that is the whole difference (not isolated by a run with a 1-s `state` limit; the flag `-state-limit` exists to do that).

### 2f: crash storms with the identity rule (`*-storm3-105.log`, `*-storm3-115.log`; the budget file was empty at the start of each storm)

The step is the one of follow-up 4: a stand-in compositor that crashes at every start for 40 s (the init restarts it 26 times). Three storms, then two real hangs (killed), then a fourth storm, then a real hang.

| | s6rc-par-wd3 | runit-111-wd3 |
|---|---|---|
| Storms run (3 with an empty budget + 1 after two real hang kills) | 4 | 4 |
| Watcher decided to kill (the rule's 4 rounds and 30 s were reached; `SKIP` line) | 4 of 4 | 4 of 4 |
| What the identity check found | recorded identity "none" in 3 (no process at the first failed round), a process that was gone in 1 | "none" in 1, a different process (another id and start time) in 2, gone in 1 |
| **Killed** | **0 of 4** | **0 of 4** |
| **Kills counted** | **0** (budget file empty after storms 1 to 3; still 2 entries, the two real kills, after storm 4) | **0** (same) |
| Real hang afterwards (deadlock, third hang kill) | **killed**, fault to kill 36.03 s, desktop back 41.06 s; budget file 3 entries | **killed**, 36.28 s, 40.31 s; 3 entries |
| Latch / state file after the storms | not set / absent | not set / absent |

(The two real hangs before the fourth storm: s6 35.45 and 35.49 s, runit 35.79 and 35.55 s to the kill, no person.) In follow-up 4 the same step was killed by the rule in 12 of 12 storms (9 of the kills found no process).

### 2g: slow but alive compositor (`*-slow-101.log`, `*-slow-111.log`; new fault, stand-in change of 17 lines, flagged above)

For 60 s every answer is 3 to 4 s late (variant "both": the `state` answer within its 5-s deadline, the sync answer over its 2-s limit; variant "sync": only the sync answer is late). The compositor keeps running.

| | s6rc-par-wd3 | runit-111-wd3 |
|---|---|---|
| Variant "both": rounds in the 60 s | 17 (3.1 to 3.9 s apart): `state` ok 17, sync FAIL 17 | 17 (3.0 to 3.9 s apart): ok 17, FAIL 17 |
| Variant "sync": rounds | 29 (2.0 s apart): `state` ok 29, sync FAIL 29 | 29: ok 29, FAIL 29 |
| Failed rounds (both probes failing) | 0 | 0 |
| **Kills** | **0** (same process before and after, desktop ok 10 s after the end, no state file, no latch, budget empty) | **0** (same) |

TESTED: one probe failing for 60 s (many times the 30 s window) never fails the round and never kills, as the unit test says. Not tested: both probes late (a compositor that needs more than 5 s for `state`), a stall that ends by itself after 10 to 25 s (the case the 30 s window is for), and the real driftwm.

### The earlier cheap checks: the full fault suite, 3 repetitions (`*-faults-106.log`, `*-faults-116.log`)

| | s6rc-par-wd3 | runit-111-wd3 |
|---|---|---|
| Events defined / ran | 50 / 50 | 52 / 52 |
| Stuck states / self-reboots (launches of the VM) | **0 / 0** (1 launch) | **0 / 0** (1 launch) |
| Needed a person | 3 (steps 22, 23, 24: hangs 4 to 6, latched; fixed by the harness's ordinary 60-s operator action, not by `reset`) | 3 (same steps) |
| Hangs 19 to 24: killed by the watcher | 3 of 6, then the latch (set once) | 3 of 6, then the latch |
| Fault to kill / fault to desktop, the 3 killed | 35.4 to 36.2 s / 39.7 to 41.4 s | 35.4 to 36.2 s / 39.0 to 40.0 s |
| Watcher killed (steps 44, 45): init restarted it in | 1.08 s, 1.08 s | 0.21 s, 0.21 s |
| Watcher kills in the suite / extra kills by storms | 3 / 0 (4 skips) | 3 / 0 (4 skips) |
| Out of memory steps: driftwm or hubd lost | none in 2 of 2 | none in 2 of 2 |
| Lines with "unable to lock" | 0 | 134 (counted with grep, not split by type) |

## Corrected table: the watcher runs side by side, window version (-wd2) against latch version (-wd3) (data only; no ranking)

Rows for `-wd2` are copied from follow-up 4 (measured then); `-wd3` is this follow-up. Same fault suite (3 repetitions; hangs are steps 19 to 24).

| | s6rc-par-wd2 (window) | **s6rc-par-wd3 (latch)** | runit-111-wd2 (window) | **runit-111-wd3 (latch)** |
|---|---|---|---|---|
| Probe limits (`state` / sync) | 1 s / 1 s | 5 s / 2 s | 1 s / 1 s | 5 s / 2 s |
| Events defined / ran | 50 / 50 | 50 / 50 | 52 / 52 | 52 / 52 |
| Stuck states / self-reboots | 0 / 0 | 0 / 0 | 0 / 0 | 0 / 0 |
| Needed a person | 3 (steps 22 to 24, capped) | 3 (steps 22 to 24, latched) | 3 | 3 |
| Hangs 19 to 24 killed by the watcher | 3 of 6, then the cap | 3 of 6, then the latch | 3 of 6 | 3 of 6 |
| Fault to kill / to desktop, those hangs | 32.4 to 32.7 s / 37.0 to 38.4 s | 35.4 to 36.2 s / 39.7 to 41.4 s | 31.9 to 32.8 s / 35.9 to 36.5 s | 35.4 to 36.2 s / 39.0 to 40.0 s |
| Spaced hang run (budget room each time) | 9 of 9 (19 of 19 in all runs), 31.9 to 33.4 s / 36.5 to 38.9 s | 9 of 9 (21 of 21 in all runs), 35.3 to 36.3 s / 39.9 to 41.4 s | 9 of 9 (19 of 19), 31.9 to 32.6 s / 35.6 to 37.2 s | 9 of 9 (21 of 21), 35.3 to 36.8 s / 38.7 to 40.9 s |
| 4th hang with the cap reached | killed again after 485.4 s | **not killed in 961 s; killed 2.1 s after `reset`** | killed again after 492.4 s | **not killed in 961 s; killed 0.4 s after `reset`** |
| State file | stays until the budget has room (still said capped after the desktop was back) | goes within 0.05 s of the compositor answering; comes back at the next hang | same as s6 | same as s6 |
| Crash storm (driftwm, 40 s) | kill decided in 12 of 12 (both inits together), 9 of 12 found no process | decided 4 of 4, **killed 0, counted 0** | (see s6 column) | decided 4 of 4, **killed 0, counted 0** |
| Slow but alive (3 to 4 s late for 60 s) | not tested | no kill (2 variants) | not tested | no kill (2 variants) |
| Watcher killed during a hang's 30 s | not tested | 15 s in: kill 36.2 s after the restart | not tested | 36.3 s after the restart |
| Watcher killed (steps 44, 45): init restarted it in | 1.10, 1.09 s | 1.08, 1.08 s | 0.21, 0.21 s | 0.21, 0.21 s |
| Latch survives a watcher kill | n/a | yes (unchanged) | n/a | yes |
| Lines with "unable to lock" | 0 | 0 | 111 (18 svlogd, 93 runsv) | 134 (grep count) |
| Out of memory: driftwm or hubd lost | none in 10 of 10 | none in 2 of 2 | none in 10 of 10 | none in 2 of 2 |

## What changed in behaviour, in plain words (TESTED, from the logs above)

- **The 4th hang.** Window: the frozen compositor was replaced by itself after 485 to 492 s. Latch: it stayed frozen for the whole 961 s that were watched (and would stay so; nothing in the code ends it) and was killed 0.4 to 4.6 s after the person-command.
- **The message.** The state file now describes a present fault: it appears 35 to 36 s after the hang, stays through a watcher restart, and goes within 0.05 s of the compositor answering again. The latch itself stays after the message is gone; a new hang is then not killed, and the message comes back after about 36 s.
- **The crash storm.** The rule still wants a kill in every storm (4 of 4), but the identity rule stops it: 0 kills, 0 counted, the budget is untouched, and a real hang afterwards is killed at the normal time.
- **Time to the kill.** About 3.5 s longer than in follow-up 4 (rounds are 5 s while the compositor is frozen, because of the 5-s `state` deadline). Nothing else about the 4-rounds-and-30-s rule changed.
- **A restarted watcher** needs its own 30 s (36 s from the restart to the kill, measured).

## What could not be tested (follow-up 5)

- The real driftwm, its two real sockets, and the real hubd (stand-ins only). In the stand-in both probes share one socket and one handler, so **they cannot fail separately**; "one probe failing" was tested only by the stand-in's slow answers (state answers, sync late) and by the unit tests.
- The person-command as a product feature: the harness runs `watcher reset` as root in the VM; who may run it, from where, and how a bar shows the message are not designed or tested.
- A damaged latch file in the VM (only unit-tested), a `reset` that falls in the middle of a round (only unit-tested for the file changes), and the latch after a reboot (the file is in `/run`, so a reboot clears it: SOURCE, not tested).
- A compositor that needs more than 5 s for `state` (both probes late), and a stall of 10 to 25 s that ends by itself.
- The time-of-check to time-of-use gap between reading the start time and sending the signal (no pidfd; not tested).
- A run with the follow-up 4 limits (1 s) on the new code, to isolate the 3.5 s: not done (BELIEVED to be the whole cause).
- Soaks (none run, decision 6), the watcher under OpenRC and b-go, the hang rule inside the product, real hardware and KVM timing (timings come from software emulation on another host than follow-up 4).
- `-wd2` images were not rebuilt or re-run.

## Questions for the owner (follow-up 5)

Each has my suggestion; a suggestion is my opinion, not a measurement.

1. **A frozen compositor now takes about 3.5 s longer to be killed (35.3 to 36.8 s)** because every round waits up to 5 s for `state`. Accept, or make the rounds a fixed 1-s tick with probes that overlap, or measure the 30 s from the first failed round's end? *Suggestion:* accept; the 30 s is deliberate and 3.5 s is small against it.
2. **No process at the first failed round gives the identity "none", and a "none" always ends in a skip and a new 30 s.** If the init restarts the compositor and the new one hangs at once, the kill can come up to 30 s later than otherwise (in the storms 3 of 8 recorded "none"). Accept? *Suggestion:* accept; it is the safe side.
3. **Who runs the person-command and how is it shown?** The test used a subcommand run by the harness. The product needs a design (a hubd command, a bar button, a phone page, a console command). *Suggestion:* decide it when the hang rule's place is decided; this test shows only that one command clears latch, budget and message within one round.
4. **The latch lives in `/run` and is cleared by a reboot.** The hub never reboots by itself, so that happens only when a person reboots. Is that right? *Suggestion:* yes.
