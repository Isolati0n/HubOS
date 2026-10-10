# Init comparison: Step 2 run log (TESTED/SOURCE/BELIEVED/UNKNOWN labels per line)

Started 2026-10-10. Plan (owner): (a) quick scorecard measurements; (b) elimination round by fault injection; (c) 30-60 min pilot on one finalist to measure memory noise and set the chunk length; (d) soak of finalists only, at least 4 h each, one at a time, chunks of 2-4 h, new random seed per chunk, PSS from smaps_rollup, leak slope per thousand restarts; (e) stop early on clear failure. No init is written here. Results are added below as they finish.

Environment: cloud container, QEMU 8.2.2 without KVM (timings are distorted). Long jobs are waited on with a Monitor loop (see CLAUDE.md in PR #75).

## Log
- 2026-10-10 TESTED: the benchmark tools rebuild with `fetch-and-build.sh` (unchanged script; only the last step needed the repo path) and all 8 candidate images build. Quick measurements (1 boot each, QEMU no KVM): see table below.
- Quick scorecard row (one boot each; supervisor start to desktop s / PSS of PID 1 + supervisors + policy kB / init-side idle CPU ticks in 30 s): s6plain 5.7 / 2490 / 1; s6rc 6.4 / 2682 / 0; runit 6.0 / 1904 / 1; dinit 5.0 / 2096 / 11; openrc 8.5 / 706 / 0; c-go 6.1 / 11038 / 218; c-elixir 6.2 / 34814 / 218; b-go 7.4 / 4948 / 27. One boot each is not a distribution; timings are distorted by emulation.
- s6plain elimination unit (3 boots, 2 PID-1 crashes, 1 PID-1 wedge, 3 trial boots, fault suite reps 3) in `tools/bench/init/results/step2/s6plain/`. Fault suite stopped making progress at 830 s after a supervisor-kill (waybar recovery 25.4 s): from then the run log is a loop of s6-log lock errors until the 5400 s timeout (UNKNOWN whether candidate or harness cause). A rerun of steps 35 to 43 alone completed (scanner-kill step did nothing: PID 1 is not killable by SIGKILL from inside; clocks, OOM ok). Kills of the six services recovered in 1.3 to 6.4 s; a hung driftwm (SIGSTOP or spin) is never noticed (65 s = the harness fixed it, needs a person).
