# Init comparison: Step 2 run log (TESTED/SOURCE/BELIEVED/UNKNOWN labels per line)

Started 2026-10-10. Plan (owner): (a) quick scorecard measurements; (b) elimination round by fault injection; (c) 30-60 min pilot on one finalist to measure memory noise and set the chunk length; (d) soak of finalists only, at least 4 h each, one at a time, chunks of 2-4 h, new random seed per chunk, PSS from smaps_rollup, leak slope per thousand restarts; (e) stop early on clear failure. No init is written here. Results are added below as they finish.

Environment: cloud container, QEMU 8.2.2 without KVM (timings are distorted). Long jobs are waited on with a Monitor loop (see CLAUDE.md in PR #75).

## Log
- (nothing measured yet)
