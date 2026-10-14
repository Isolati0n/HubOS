# PROPOSAL: the burn-in tool (fake nodes, a fault injector, a monitor, a leak report)

**Status: PROPOSAL. Nothing here is decided, built or tested** unless a line says TESTED or SOURCE. `HUB-OS.md` wins if this file disagrees with it. Written 2026-10-10 by the lead agent (no helpers). No code. **Ports, addresses, thresholds and deadlines are placeholders**; deadlines and thresholds are to be set from a pilot run, not guessed. Labels: TESTED, SOURCE, BELIEVED, UNKNOWN.

## 1. Purpose
Show, with data, that the hub keeps working for days with 20 machines while things fail: windows present, no unexpected restart, no leak, bounded recovery. It re-uses methods already used in the repo: the init comparison's PSS sampling and fault injector (`tools/bench/init/hubsim`, SOURCE), the compositor soak of `docs/proposals/bulletproof-compositor.md` and the stability plan in `hub-stability.md` (the 72-hour gate, SOURCE).

## 2. Parts
1. **Fake nodes:** small Go programs that serve an RFB handshake (security None) and a slowly changing frame buffer in **Raw** rectangles, emit toast traffic (the helper's notification long poll) and clipboard traffic, and follow a **scripted lifecycle** (up, slow, down, back). They bind to **loopback addresses real nodes do not use** (the repo's existing fake nodes use `127.1.x.y`: SOURCE `docs/hubd-slice2.md` 7; `tools/fakenode` is a listener only and does not speak RFB: BELIEVED, to be checked).
2. **Fault injector** (section 3).
3. **Monitor** (section 4).
4. **Leak report** (section 5).
The session layer's workers and presenter do not exist yet (SOURCE `docs/STATE.md` 3.2), so until they do, the "RFB node" targets are the external viewers the hub uses today and the monitor checks windows in driftwm's list; this is stated in every report.

## 3. Faults
| Fault | How | Where |
|---|---|---|
| kill | `kill -9` of a named process | everywhere |
| stop/continue | `SIGSTOP` then `SIGCONT` after N s | everywhere |
| delayed writes | a proxy or `LD_PRELOAD`-free delay on the fake node's socket writes (placeholder: a delaying relay) | fake nodes |
| corrupt bytes | flip bytes in a stream through the relay | fake nodes |
| message flood | far more notifications/clipboard items than allowed | fake nodes, seams |
| fd exhaustion | lower `ulimit -n` of a victim or hold fds open | victim process |
| memory exhaustion in a cgroup | a hog inside a cgroup with a limit (needs cgroups: the hub kernel has none today: SOURCE `docs/proposals/hub-isolation.md` line 167, `docs/STATE.md` 4) | test machine with cgroups |
| restart-order permutations | restart hub components in every order of a small set | everywhere |
| fake clock jump | the **fake node's view** only (not the hub's clock) | fake nodes |
| **GPU reset** | write to the amdgpu debugfs file `amdgpu_gpu_recover` (the owner gives it as SOURCE: Linux v6.12 amdgpu driver; needs `CONFIG_DEBUG_FS`; `hub-stability.md` G1 labels the same file BELIEVED, "I did not verify the name"; I have not verified it) | **real hub, debug image only** |
| kill real hub components | kill compositor, `hubd`, bar, conductor, workers | **real hub, debug image only** |
`CONFIG_DEBUG_FS` is not in the hub kernel fragment (SOURCE `image/kernel/hub.frag`: searched, not found), so the debug image needs it added.

## 4. The monitor (every second)
Checks: `hubd` alive (answers on its socket); every machine's window present (driftwm window list); no unexpected restart (process start times of hub components; an injected restart is recorded as expected); compositor frame heartbeat (the compositor's frame counter or a probe: placeholder, SOURCE `hub-stability.md` 9.1 has the two probes); seam drop counts (`seams-catalogue.md` rule 2); memory of `hubd`, `driftwm`, the conductor and the workers. A **violation** is any check failing outside an injected fault's allowed window; its **length** is recorded.

## 5. Leak measurement
- **Per process:** PSS from `/proc/PID/smaps_rollup` (the method already used by the init comparison harness: SOURCE `tools/bench/init/hubsim`), sampled at a fixed interval (placeholder 60 s), plus runtime counters: **Go** `MemStats.Sys - HeapReleased` (BELIEVED to be the right "memory the process holds" figure; check against the Go runtime docs before use), **BEAM** allocator carriers (`erlang:memory()` and `erlang:system_info(allocator)`; BELIEVED).
- **Slope:** least-squares fit of memory against the number of restarts (or injected faults), reported **per thousand restarts**, with a **confidence interval taken from the variance of a pilot run** on the same build. **Do not report "zero positive slope":** noisy data cannot show that; report the interval and compare its upper end with the threshold.
- The pilot run (30 to 60 minutes, no faults) sets the noise and the chunk length; this is the same method as the init comparison (TESTED there only in a pilot still to come on 2026-10-10).

## 6. Report
Recovery time **median and max per fault type**; leak slopes with intervals; counts of faults injected, expected restarts, unexpected restarts; every violation with its length; drop counters per seam; the build id and the seeds. Machine-readable (JSON) and human-readable (text).

## 7. Pass criteria
- No violation longer than **3 seconds**.
- Every recovery under a **deadline set from the pilot** per fault type (not guessed; until a pilot exists the deadline is UNKNOWN).
- No leak slope whose upper confidence bound exceeds its **threshold** (the threshold is also set from the pilot and the owner's tolerance; UNKNOWN).

## 8. Schedule on the real hub
The existing rule (SOURCE `hub-stability.md` 11.14 and `HUB-OS.md` "A new kernel or Mesa is adopted only after at least 4 weeks and a 72-hour soak"): a **72-hour accelerated fault-injected soak** (7 days when the change is riskier), then **2 weeks of probation with the monitor on and no injection**. *(The 2 weeks of probation is the owner's text; it is not recorded in the repo — see `hub-os-md-audit.md` in the docs PR.)* Runs are cut into **chunks of 2 to 4 hours**, each with a **new random seed**, results **committed per chunk**. In this cloud environment only an hour-scale run on fake nodes and a test machine is possible; the 72-hour and probation runs need the real hub in December.
**Promotion after probation is manual**, by the owner, from the report.

## 9. Failure table
| What happens | Result |
|---|---|
| Injector fails to inject | Recorded as `injection_failed`; not counted as a recovery |
| A victim does not recover before its deadline | Violation; the run continues; reported |
| Monitor itself dies | The run is invalid from that moment; the report says so |
| Fake node crashes by itself | Counted as an unexpected restart of a fake node, listed apart from the hub's |
| Clock of the host jumps | Samples use a monotonic clock; a jump does not create a violation |
| The machine reboots | The run is split; the part after the reboot is a new chunk |
| Disk full for the report | The report is written to RAM first and copied; a copy failure is reported |

## 10. Tests of the tool itself
1. `kill`: gives exactly one recovery, with its time.
2. `SIGSTOP`/`SIGCONT`: gives a stall and then a recovery.
3. cgroup memory exhaustion: the victim is killed by the limit and the event is recorded.
4. Fake clock jump: no false violation.
5. A window deliberately removed (closed with no injected fault): detected as a violation.
6. The report is valid JSON and the text report parses back to the same numbers.
7. A synthetic memory series with a known slope: the fitted slope and interval are right.
8. A flat synthetic series: the interval contains zero and the tool does not say "no leak" (it prints the interval).

## 11. Five assumptions I am least sure of
1. That fake nodes on loopback are close enough to real nodes for the hub's behaviour (UNKNOWN).
2. That `amdgpu_gpu_recover` exists under that name on the target driver (SOURCE per the owner; unverified by me).
3. That PSS plus Go/BEAM counters see the leaks that matter (BELIEVED).
4. That a 3-second violation limit is achievable with a restart of the compositor (UNKNOWN: compositor restart plus window restore took seconds in the repo's tests; see `hub-stability.md`).
5. That a pilot variance is stable across builds (UNKNOWN).

## 12. Decided by the owner (2026-10-10)

- **Faults are injected into real hub components in the debug image.**
- **Recovery deadlines come from the pilot run**, not from guesses.
- **Promotion after probation is manual**, based on the tool's report.
- Fake nodes use **loopback ports**, not network namespaces.

## 13. Questions for the owner (still open)

None left from the earlier list.
