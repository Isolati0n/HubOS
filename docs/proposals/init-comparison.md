# STATUS: PAUSED, INCOMPLETE (2026-10-05, by the coordinator's order; owner: not vital yet)

## Owner decisions (2026-10-05, round 5)

These are the owner's decisions, recorded by the lead agent from the owner's message, and also recorded in `HUB-OS.md`.

- At least 4 hours of soak per candidate.
- The scorecard wording of R3 becomes "returns by itself within the retry limit, then degraded and alert".
- The owner's action on a degraded service is a bar button and a command.
- The confirm step asks the init for health.
- No machine runs systemd (the owner's correction of 2026-10-05); the init would be exclusive to the hub.


**Done:** (1) the scorecard (section 1 below, committed before any measurement). (3) the harness in `tools/bench/init/` (stand-in services, fault injector with kill, hang, crash storm, slow/failed dependency, full disk, supervisor killed, PID 1 killed or wedged, out of memory, clock jumps, update trial boot, policy-bug test; a chaos soak mode; eight candidates: s6 plain, s6+s6-rc, runit, dinit, OpenRC, and architecture C with a Go brain, C with an Elixir/OTP brain, B approximated). Prototypes of the escalation ladder and health probe in Go and Elixir (unit tests pass). Static PID 1 language stubs measured (`results/pid1-stubs.txt`). Host-side checks (`results/graph-checks.txt`, `credentials-test.txt`, `sizes.txt`).

**Not done:** the measurement campaign over all candidates (only s6plain was run: boots, PID 1 killed/wedged, three trial boots, and 36 of about 60 steps of the fault suite, in `results/partial-s6plain/`; its supervisor-killed, storm, dependency, disk-full, clock and OOM results do not exist); no chaos soak ran at all (so no soak hours to report); sections 2 and 4 to 6 of the task (comparison text, the data summary, the custom-init design and the owner questions) are not written, only drafts of some tables in `results/drafts/`. **No conclusion about s6 versus a custom init has been drawn.**

**Partial findings (TESTED in QEMU 8.2.2 without KVM, host loaded, 4 CPUs seen):** today's s6 recovers a killed service in a median 3.1 s (worst 7.4 s, 30 kills); a wedged driftwm (SIGSTOP or busy loop) is never noticed (6 of 6 needed a person); in a trial boot a wedged driftwm is CONFIRMED (false confirmation, 19.8 s) because s6 only sees that the process exists; a driftwm that cannot start leads to a rollback at 60 s with the watchdog guard still feeding; PID 1 killed: machine back in 20 to 23 s; PID 1 wedged (ptrace stop): the guard stops feeding and the machine is back in 49 s. Go brain over s6: hung driftwm recovered in about 12 to 17 s by probe, crash storm ended in "degraded + alert" with the rest running (debug run, not in the campaign).

**How to resume:** the work directory and all downloads were deleted to free disk. Rebuild with `W=<empty dir outside the repo> bash tools/bench/init/fetch-and-build.sh` (about 30 min; NOT re-run as one script after it was assembled), then use the scripts in `tools/bench/init/resume/` (`env.sh` holds the paths: edit W and REPO; `campaign1.sh` runs boot, PID 1, trial and fault suites for all candidates one VM at a time; `bugtest.sh`; `rep.sh` or `python3 tools/bench/init/report.py RUNDIR [SOAKDIR]` makes the tables). Soak: `python3 tools/bench/init/run-qemu.py CAND soak --dur 2700 --seed 1 --out RUNDIR/CAND-soak-1.log` (build the rootfs first with `bash tools/bench/init/mkrootfs.sh CAND`). The `gen_*.py` and `patch*.py` files in `resume/` are the generators and edits already applied to the candidates (kept only for the record). Known harness facts: s6-rc needs `/opt/s6` paths mirrored at the host build prefix; `select{}` in the PID-1 wedge helper is a deadlock; the Elixir brain must be started with `-eval "'Elixir.Brain':main()."`; dinit's `boot` must use `waits-for`, otherwise a failed service stops `boot`.

**Security:** no core, dump, environment or credential files are in this branch (checked by `find`); runs used `ulimit -c 0` and ran outside the repository.

---

# PROPOSAL: is a purpose-built init for the hub better than s6? (scorecard written BEFORE any measurement)

**Status: PROPOSAL / MEASUREMENTS. Nothing here is decided and nothing here is in the image.** `HUB-OS.md` wins if this file disagrees with it. Written 2026-10-05 by a helper agent (branch `init-comparison`). Unverified until the owner's lead has read the sources.

**This first version of the file contains only the scorecard.** It was committed before any measurement was taken, so the weights, the tests and the pass rules cannot be bent to fit the results. The results, the comparison and the questions are added in later commits (git history shows the order).

## 0. How to read this

Every item in this file carries one label:

- **TESTED**: I ran it; the command and the output are shown or named.
- **SOURCE**: read in a document or program source; path or link and the date read (today is 2026-10-05).
- **BELIEVED**: my reasoning, not tested.
- **UNKNOWN**: nobody has checked.
- **DESIGN**: a proposal made here, for the owner to decide.

Plain words: an "init" is the first program the kernel starts. It starts every other program, restarts the ones that die, and is the one program that must never die. The owner asked: would a custom init be *objectively better than s6* for the hub, in the things that matter most to the owner (the hub never being the thing that fails, and the init working together with updates, recovery and hubd)?

## 1. The scorecard

### 1.1 Rules of the game (fixed now)

1. **Weights (total 100 points).** Reliability 45. Integration 40. The candidate features the owner listed 10. The low-weight items (boot time, memory, idle CPU, code size) 5 in all. Number of languages or tools on the hub: **not a criterion at all** (owner decision).
2. **Each criterion is rated 0 to 3**, and a candidate earns `weight x rating / 3`.
   - **3** = it does this by itself (native), with no extra script, and the test passes every run.
   - **2** = it does this with a script or a small helper we write around it (the helper is named), and the test passes every run.
   - **1** = it only half does it: it works only some of the time, or only a person can finish it, or it costs a reboot.
   - **0** = it cannot, or the test leaves the machine in a state only a reboot clears.
3. **Every candidate is scored twice**: *alone* (its own features) and *with a layer* (the same candidate plus the one policy program we would write, described in section 1.4). The honest margin of a custom init is "custom score minus (s6 plus a layer) score", not "custom minus bare s6".
4. **Timing numbers** (recovery time, boot time) come from QEMU in software emulation without KVM. They are reported with the number of CPUs and the load, as median and spread over several runs. Because emulation distorts timing, a **timing rating uses ratios against the other candidates measured the same way**, not absolute seconds; the absolute numbers are given for information only. What only real hardware can show is listed in the results.
5. **A test that "passes" must leave the machine usable without a reboot.** Where a candidate needs the owner to act (for example to restart a service by hand), that is rated 1 and the time is recorded as "needs a person".
6. **The hub never reboots itself because of a service failure** (owner decision). Any candidate or layer that reboots on a service failure gets 0 on the criteria it affects (I3, I4). The only allowed automatic reboots are (a) a rollback when an update's trial boot never becomes healthy and (b) the hardware watchdog when the kernel or PID 1 is dead.
7. **The watchdog is fed only by a minimal guard** that checks the kernel and PID 1 are alive. It is **never** fed or starved by service health.

### 1.2 Reliability criteria: 45 points. Each is a fault-injection test.

All run against the hub's real service graph, or a stand-in with the same start order, sockets and failure behaviour (section 1.5): seatd, udevd, dbus, driftwm, Waybar, hubd.

| # | Criterion | Pts | Fault injected | Measured | Rating 3 / 2 / 1 / 0 |
|---|---|---|---|---|---|
| R1 | A service is killed | 5 | `kill -9` of each of the six in turn, 10 times each | time until the desktop and bar answer again (median and worst); service restarted; dependents ok | 3: back by itself, dependents follow, worst case within 2x the best candidate; 2: back by itself, dependents need a layer; 1: some need a person; 0: stays dead |
| R2 | A service is HUNG but alive | 6 | `SIGSTOP` on driftwm, and a busy-looping driftwm that no longer answers `state` | does anything notice; time to notice; does it restart it; what it does to the dependents | 3: notices and restarts by health probe by itself; 2: with our helper; 1: only a person can (it reports nothing); 0: not even visible |
| R3 | Crash storm | 6 | a service that exits at once, every start, for 60 s, then is fixed | restarts per minute (CPU burnt), does it back off, does it give up, is it reported, does everything else keep running, does it return after the fix | 3: backoff and give-up by itself, reported, returns by itself within the retry limit, then degraded and alert; 2: with our helper; 1: spins at full speed or cannot return by itself; 0: takes other services down |
| R4 | A failed or slow dependency | 5 | seatd slow to start (20 s) and seatd failing to start | do dependents wait without busy polling, start when it is ready, avoid starting in the wrong order; what is reported | 3: ordering and waiting by itself, no polling scripts; 2: with scripts; 1: dependents crash-loop until it is up; 0: deadlock |
| R5 | Disk full | 4 | fill the log/state filesystem to 100% (ENOSPC) while services log | do the services and the supervisor keep running, does logging stall a service, does it recover after space returns | 3: nothing stalls, recovers; 2: logs lost, nothing else; 1: a service stalls; 0: the supervisor or PID 1 stops or dies |
| R6 | The supervisor itself is killed | 5 | `kill -9` of the per-service supervisor (e.g. `s6-supervise`, `runsv`) and of the scanner/manager (`s6-svscan`, `runsvdir`) where it is not PID 1 | do the managed services keep running (not orphaned or killed); who restarts the supervisor; time to full health | 3: restarted by its parent, children keep running; 2: restarted, children restarted; 1: only a person; 0: children lost forever or PID 1 dies |
| R7 | PID 1 killed or wedged | 6 | PID 1 made to exit (the kernel panics: `Attempted to kill init`); and a wedged PID 1 (stopped) in a PID namespace | what the kernel does; time until the machine is back; the guard (watchdog feeder) detects a wedged PID 1 without looking at any service | 3: reset by the kernel/watchdog and back with no state lost that matters, guard independent of services; 2: same with a script; 1: hangs until a person; 0: not recoverable |
| R8 | Out of memory | 4 | a runaway program allocates until the cgroup limit or the machine limit | is driftwm/hubd protected (oom score, cgroup), is the culprit killed rather than a desktop service, is PID 1 safe | 3: protection configurable natively; 2: with `echo` into /proc and a helper; 1: only luck; 0: PID 1 or the compositor is the victim |
| R9 | Clock jumps | 4 | the wall clock set forward by 1 day and back by 1 hour | do restart delays, timeouts, back-off and uptime counters stay right; does anything stall for an hour | 3: all timing monotonic; 2: one cosmetic effect; 1: a timeout stalls or fires wrongly; 0: services restart wrongly or never |

### 1.3 Integration criteria: 40 points

| # | Criterion | Pts | What is checked | Rating 3 / 2 / 1 / 0 |
|---|---|---|---|---|
| I1 | Trial-boot confirm tied to service health, watchdog **not** | 5 | The update's confirm step can ask "are driftwm, Waybar and hubd healthy" from the init's own state (not a polling script) and confirms only then; while the hardware watchdog is fed by a guard that looks only at kernel and PID 1 | 3: native query/dependency; 2: a small script over the init's status; 1: only ad hoc; 0: confirm cannot see service health, or the watchdog depends on services |
| I2 | Restart wedged services by health probe | 4 | e.g. restart driftwm when `driftwm msg state` stops answering | 3: native probe; 2: our helper over the init's control interface; 1: a cron-like loop in a shell script; 0: cannot |
| I3 | Escalation ladders that **end in an alert, not a reboot** | 6 | per service: restart, back off, restart its dependents, stop retrying, mark degraded, show an alert on the bar, everything else keeps running | 3: declared per service, native; 2: with our helper; 1: fixed single policy; 0: ends in a reboot or cannot stop retrying |
| I4 | The hub never reboots itself because of a service failure | 4 | grep every reboot path; inject every service fault and check no reboot happens; the only two permitted automatic reboots exist | 3: no service-failure reboot path exists and the two allowed ones are present; 2: one must be removed from our config; 1: reboot is the default and must be switched off; 0: cannot be avoided |
| I5 | Dependents restart when a dependency restarts, without polling scripts | 4 | restart driftwm; Waybar and hubd follow | 3: declared; 2: helper using the init's events; 1: polling script (today's `follow-driftwm`); 0: not possible |
| I6 | Status socket for hubd and the node helper | 3 | per-service state, restart count, time since last failure | 3: native socket; 2: our helper reading the init's state; 1: parsing command output; 0: nothing to read |
| I7 | Last log lines and exit statuses captured in a crash record the recovery agent serves at `/v1/logs` (see `docs/proposals/black-box-recorder.md`) | 4 | kill a service; is its exit status and the tail of its output saved in a capped record | 3: native; 2: helper; 1: logs exist but no exit status or no cap; 0: lost |
| I8 | Readiness by contract | 3 | a service says "I am ready" by a defined mechanism, dependents wait for that, not for a sleep | 3: native; 2: via a helper; 1: sleeps and polling; 0: none |
| I9 | Service graph generated from the machine config and checked at build time | 3 | cycle, missing dependency, missing binary caught before an image exists | 3: native compiler/checker; 2: our checker; 1: partial; 0: first seen at boot |
| I10 | Resource policy | 4 | cgroup v2 limits, CPU priority, OOM protection for driftwm and hubd | 3: declared per service natively; 2: via scripts/helper; 1: global only; 0: none |

### 1.4 The "layer" that every candidate may add

DESIGN, for scoring only, not a decision. A layer is **one program, which is a normal supervised service, never PID 1**, that (a) probes health, (b) runs the escalation ladder by commanding the candidate's own control tool, (c) offers a status socket, (d) writes the crash record, (e) answers the confirm step. It is the "policy brain" of architecture C (section 2 of the later results). The watchdog feeder is **not** part of the layer.

### 1.5 The test setup (fixed now)

- The hub's six services are replaced by **stand-ins**: one small static program that behaves like each real one for what the init can see: start order and waiting (driftwm waits for seatd and udevd; Waybar and hubd need driftwm's socket and **exit when driftwm's socket drops**, like the real programs do), a readiness signal, a socket that answers `state`, injectable faults (crash on start, hang, slow start, leak memory, fill disk). The real driftwm cannot run usefully in a no-KVM virtual machine at speed and the owner's own `follow-driftwm` script is reproduced faithfully for the candidates that have no dependency restarts. **A stand-in is not the real thing; this is stated again in the results.**
- Everything runs in QEMU 8.2.2 without KVM, one run at a time, or, where PID 1 is not the thing under test, in a PID namespace of the host. Which is which is stated per test.
- The invariants checked all the time (chaos soak): the desktop (driftwm answers) and the bar (Waybar and hubd registered with it) always come back; the time to recover is recorded; no zombie, file-descriptor or memory growth in PID 1 and supervisors; no state it cannot leave without a reboot. Every unrecovered state is reported.

### 1.6 Candidates

s6 plain (what the image has today, with `follow-driftwm`), s6 with s6-rc, runit, dinit, OpenRC, and the custom designs: **A** custom PID 1 that supervises itself, **B** tiny stable PID 1 plus our own smart supervisor as a normal service, **C** s6 does the per-service supervision and a separate policy brain decides (Erlang/Elixir with OTP supervision trees against Go and Rust). systemd is not a candidate; its documentation may be cited for ideas.

### 1.7 Candidate features (10 points, rated 0 to 3 the same way, each equal weight)

Boot progress screen with plain-words status and errors; boot timeline history with regressions flagged; boot self-tests (disk, GPU, network, time) into the crash record; safe boot of the last good config when a new one fails validation; config validated at build time and at boot; credentials handed to services without command lines; built-in timers for housekeeping (state backup to the NAS, log capping, time sync); per-service resource limits and priorities; structured, capped log queryable by hubd; a diagnose command; a live service-graph view through the status socket; an orderly stop sequence for restarts the owner chooses. For each: value, cost, risk, and whether s6 plus a layer can do it.

### 1.8 Low-weight criteria (5 points in all)

Boot time to the last service ready (2), PID 1 and supervisor memory (1), idle CPU (1), code size (1). Measured and reported; they decide nothing on their own.
