# STATUS: MEASUREMENTS DONE AS FAR AS THEY CAN BE DONE HERE; THE OWNER DECIDES NEXT (updated 2026-10-13)

**What is done (TESTED unless marked):** the scorecard (section 1, committed before any measurement); the harness (`tools/bench/init/`); an elimination round by fault injection over about 19 candidate setups (s6 plain, s6+s6-rc, runit in three setups, dinit in five, OpenRC in four, the Go and Elixir brains over s6-rc, the Go brain over s6-svscan alone); a 4-hour soak for the candidates that had no stuck state (s6plain, s6rc-par, runit, runit-111, b-go, openrc-doc); hang-rule data and a test watcher (a TEST STAND-IN, not the product watcher) run under s6rc-par and runit-111, first with a wrong probe pairing (follow-up 3), then with the owner's rule of `hub-stability.md` section 9.1 (follow-up 4: two compositor probes, the driftwm `state` request and a Wayland `wl_display.sync`, both must fail; 4 rounds and 30 s; at most 3 kills in 10 minutes; the kill budget kept in a file in `/run`; a state file `capped <time>`), with 19 spaced hangs per init, the budget tested across watcher restarts and across 10 minutes, a crash-storm and out-of-memory re-run, the cause of the svlogd lock lines of runit-111np (two `svlogd` on one log directory after a supervisor kill), and one 2-hour soak chunk of runit-111np (decision of the owner). Sections 2, 3, 4, 5 and 6 below are now written from the run log (`init-comparison-run.md`), as data only. **No init was written. No candidate is recommended or ranked. Whether s6 stays the hub's init is not decided here.**

**What waits for the owner:** the open questions in section 6 (among them: does "needs a person after a killed supervisor" count as passed for OpenRC; which place the hang rule lives in; whether the cap is a window or a latch; whether more soak or another seed is wanted). The scorecard points (section 1) were not computed: several ratings need an owner decision first (for example what "2 = with a script" means for a test watcher), and the plan says the owner reads the data before any total is made.

**What was not done / cannot be done here:** real hardware and KVM timing, the real driftwm, waybar and hubd, real power loss, runs longer than 4 hours, the hang rule inside the product (it is not built), the real driftwm's answer to a Wayland sync (the test stand-in answers one request only), a watcher killed during the 30 s of a hang, a slow but alive compositor, a soak with the watcher, the watcher under OpenRC and b-go (not run, owner decision), the cause of a full `/var` in the runit-111np soak (UNKNOWN), c-go and c-elixir soaks (both were stopped early for a stuck state), a soak of dinit (no setup passed the elimination round).

**How to resume / rebuild:** `W=<empty dir outside the repo> bash tools/bench/init/fetch-and-build.sh`, then `mkrootfs.sh CAND` and `run-qemu.py CAND faults|soak ...` (see `tools/bench/init/` and `init-comparison-run.md`; the scripts used for the numbers are `report.py`, `event_counts.py`, `needed_person.py`, `soak_ci.py`). Results: `tools/bench/init/results/step2/`.

**Security:** no core, dump, environment or credential files are in this branch; every push was checked with `tools/check-push.sh`; runs used `ulimit -c 0` and ran outside the repository.

## Owner decisions (2026-10-05, round 5)

These are the owner's decisions, recorded by the lead agent from the owner's message, and also recorded in `HUB-OS.md`.

- At least 4 hours of soak per candidate.
- The scorecard wording of R3 becomes "returns by itself within the retry limit, then degraded and alert".
- The owner's action on a degraded service is a bar button and a command.
- The confirm step asks the init for health.
- No machine runs systemd (the owner's correction of 2026-10-05); the init would be exclusive to the hub.

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

Boot progress screen with plain-words status and errors; boot timeline history with regressions flagged; boot self-tests (disk, GPU, network, time) into the crash record; safe boot of the last good config when a new one fails validation; config validated at build time and at boot; built-in timers for housekeeping (state backup to the NAS, log capping, time sync); per-service resource limits and priorities; structured, capped log queryable by hubd; a diagnose command; a live service-graph view through the status socket; an orderly stop sequence for restarts the owner chooses. For each: value, cost, risk, and whether s6 plus a layer can do it.

### 1.8 Low-weight criteria (5 points in all)

Boot time to the last service ready (2), PID 1 and supervisor memory (1), idle CPU (1), code size (1). Measured and reported; they decide nothing on their own.

---

# RESULTS (written 2026-10-12 from `init-comparison-run.md` and `tools/bench/init/results/step2/`; data only)

Labels as in section 0. Every number below is in the run log; where I give a number without a label it is TESTED in the harness named in section 2. The raw tables of the elimination round are in `tools/bench/init/results/step2/FINAL-TABLES.md`. There is no ranking and no recommendation in this part, by the owner's order.

## 2. What was tested, and how

- **The machine.** QEMU 8.2.2 without KVM, 2 vCPU, 1 GB, Ubuntu 24.04 kernel 6.8, one virtual machine at a time. Timings are therefore slower and noisier than on real hardware; ratios between candidates measured the same way are the useful part.
- **The services.** Six stand-ins (seatd, udevd, dbus, driftwm, waybar, hubd) written for the test: one small Go program that behaves like each real one for what an init can see (start order, readiness, a socket that answers `state`, exit when the compositor's socket goes away, injectable faults). The real compositor, bar and hub daemon were not run. Waybar and hubd are started through the image's `follow-driftwm` wrapper (the "parity rules") in every candidate that was given them.
- **The candidates.** s6 plain (what the image has), s6 with s6-rc (`s6rc`, then `s6rc-par` with the same wait and follow rules), runit (stock; `runit-111` with the documented exit-111 restart; `runit-111np` without `-P`), dinit (default, `-par`, `-sm` smooth-recovery, `-ms` milestone dependencies, `-wf` waits-for), OpenRC (`openrc`, `-par`, `-doc` with documented options only, `-doc2` with a different confirm hook), architecture C (s6-rc plus a policy brain in Go, `c-go`, or in Elixir, `c-elixir`) and architecture B approximated (`b-go`: s6-svscan as PID 1 and the Go brain starting and watching the services itself). The first setups of `s6rc`, `dinit` and `openrc` were not equal to s6 plain in their wait and follow rules; they were re-run and the first results are kept and marked.
- **The elimination round (per candidate).** 3 boots; PID 1 killed twice; PID 1 wedged once (a hardware-watchdog stand-in fed by a minimal guard); 3 trial (update) boots: healthy, driftwm that cannot start, driftwm wedged 2 s after it first worked; the fault suite. The fault suite has 43 steps: 18 kills (3 per service), 6 hangs of driftwm (3 SIGSTOP, 3 busy loop), 2 crash storms (40 s), a slow and a failing seatd, 2 full-disk, 12 supervisor-kill events, 2 scanner-kill events, 2 brain kills, 4 clock jumps (+1 day, -1 hour), 2 out-of-memory. Which steps give events depends on what the candidate has (a supervisor, a scanner, a brain): 36 events are common to all, the others only to those who have the part (run log, follow-up 2, item 1).
- **Two counts that are easy to misread.** "Stuck" = a state only a reboot clears. "Needed a person" = nothing recovered within 60 s and the harness then ran the candidate's own restart command (what the owner would type); it is not a stuck state.
- **The soak.** Random faults every 5 to 20 s for 2 x 2 hours with a new seed per chunk (after a 30-minute pilot), memory of PID 1, scanner, supervisors and brain read every 60 s, restarts counted in the VM, slope of memory against restarts with a 95 % interval. Only candidates without a stuck state in the elimination round were soaked (the owner's rule), plus two that were stopped early.
- **Extra checks.** The hung compositor on every surviving candidate; the runit source reading; the OpenRC and dinit documented-options tries; a sharper memory measurement for the Go brain; the hang-rule data; the test watcher (run log, follow-ups 3 and 4). **The test watcher (follow-up 4)** is one small Go program, supervised like any service, a TEST STAND-IN, not the product watcher: it asks the stand-in compositor two things per second (the `state` request and a Wayland `wl_display.sync`, which the stand-in compositor was given the ability to answer, a flagged stand-in change), a round fails only when both fail, it kills after 4 failed rounds and 30 s, at most 3 kills in 10 minutes, it never reboots anything, and it writes its kill times to `/run/hubos/watcher.budget` and `capped <time>` to `/run/hubos/watcher.state`. Hang runs used SIGSTOP, a busy loop and a deadlock; the hubd ping is only logged.

## 3. What could not be tested

(The list for the watcher is in the run log, follow-up 4, "What could not be tested".) Real hardware, real watchdog behaviour, KVM-speed timing; the real driftwm, waybar, hubd and viewers; real power loss and disk damage; the services' own memory (only the init side was measured); runs longer than 4 hours; leaks smaller than each interval (about 20 bytes per restart for s6rc-par, 200 bytes for the Go brain); the c-go brain's counters; a soak of c-go and c-elixir (stopped early with a stuck state), of dinit (no setup passed) and of the first setups; whether dinit has a setting that retries a failed start (three setups tried and recorded: `depends-on`, `depends-ms`, `waits-for`; the owner stopped further variants); the hang rule in the product (not built); anything about the custom designs A and B beyond the one approximation of B.

## 4. Results per candidate (elimination round; soak where it ran)

| Candidate | Elimination: events ran (needed a person), stuck | Soak (4 h): events, needed a person, stuck, self-reboots; slope per 1000 restarts [95 %] | What stands out (TESTED) |
|---|---|---|---|
| s6plain | 48 (3), 0 | 595, 54, 1, 0; +20.9 [+1.7, +40.0] | hung compositor never noticed; the first run once looped on logger lock errors (not reproduced in the full repeat) |
| s6rc (first setup) | 19 of 48 (3), 8 | not run | stuck because the setup lacked the follow rule; withdrawn by `s6rc-par` |
| s6rc-par | 48 (5), 0 | 663, 50, 0, 0; -8.6 [-19.5, +2.2] | hung compositor never noticed (SIGSTOP 3 of 3, busy loop 2 of 3) |
| runit (stock) | 48 of 50 (4), 0 | 779, 43, 0, 29; -70.4 [-168.9, +28.1] (one usable chunk) | a kill of `runsvdir` makes runit leave stage 2 and reboot the machine (29 times in the soak) |
| runit-111 | 50 (7), 0 | 647, 55, 0, 0; +0.2 [-29.3, +29.7] | no reboot at a scanner kill; the restarted stage 2 produces lock-error lines (7831 in 4 h) |
| runit-111np | 50 (6), 0 | 2 h, one chunk (owner's decision after the cause was found): 290, 34, 1, 0; -87.8 kB per 1000 restarts (std.err 37.2) | 53 lock-error lines in the elimination round: two `svlogd` on one log directory after a supervisor kill, no log line lost or doubled in 15 lock-probe reps; in the soak 4161 lock lines (194 before 5800 s, 3967 while the stand-in's 4 MB `/var` was full, cause of the full disk UNKNOWN), the 34 persons were hangs (no watcher), 1 stuck state at the very end |
| s6rc-par-wd, runit-wd (compositor + hubd probes, not the owner's rule) | 50 (5), 0 / 52 (5), 0 | not run | the stand-in hubd keeps answering while the compositor is frozen, so the watcher did not fire (0 of 6 hangs); one kill in a runit crash storm |
| s6rc-par-wdc, runit-wdc (one compositor probe only, not the owner's rule) | 50 (1), 0 / 52 (3), 0 | not run | 3 kills, then the cap; fault to desktop 35.8 to 38.0 s |
| s6rc-par-wd2, runit-111-wd2 (owner's rule) | 50 (3), 0 / 52 (3), 0 | not run | 19 of 19 hangs killed by the watcher alone when the budget was full (31.9 to 33.4 s; desktop back 35.6 to 38.9 s); hangs 4 to 6 of the suite capped (3 of 3 needed a person); the budget survived 3 watcher kills and returned after 10 minutes |
| b-go (B approximated) | 38 (0), 0 | 702, 0, 0, 0; +99.9 [-1084, +1284]; with forced collection and counters (b-go-stats) -37.2 [-274, +199] | hung compositor noticed 10.6 to 15.1 s; one process holds every service |
| c-go (C with Go brain) | 48 of 50 (2), 1 | stopped after 14 min: 2 stuck (supervisor of driftwm killed) | hung compositor noticed 9.3 to 13.7 s; the supervisor-killed driftwm step ends at the brain's give-up (alert) and a stuck state |
| c-elixir | 47 of 50 (2), 2 | stopped: 1 stuck, then a logger-lock loop | hung compositor noticed 14.1 to 15.4 s; same weak step as c-go |
| dinit (default, -par, -sm, -ms, -wf) | 20 / 20 / 13 / 31 / 33 of 36, stuck 8 / 8 / 8 / 5 / 3 | not run | a service that fails to start (crash storm, failing seatd) stays down; none of the three documented relationships (`depends-on`, `depends-ms`, `waits-for`) fixed all of it |
| openrc (first setup) | 46 of 48 (20), 2 | not run | not equal to s6 plain in its rules |
| openrc-par | 48 (12), 0 | not run | killed `supervise-daemon` never restarted; SIGSTOP never noticed (stop schedule TERM only) |
| openrc-doc | 48 (6), 0 | 569, 53, 0, 0; +33.5 [-50.2, +117.2] | hung compositor noticed 19.4 to 20.2 s; killed supervisor not restarted (all 6 and 53 "needed a person" events are that) |

## 5. What each part of the scorecard shows (no points, no total)

The scorecard weights reliability (45) and integration (40) most; boot speed, memory, idle CPU and code size least (5 together). Only the parts the harness measured are shown. A dash means the harness has no data for that candidate.

| Scorecard part | What the data show, per candidate |
|---|---|
| **R1 service killed (5)** | All 18 plain kills recovered by themselves in s6plain (median 2.8 s, worst 6.5), s6rc-par (1.9, 5.0), runit (1.9, 4.2), runit-111 (1.8, 4.2), b-go (3.6, 6.7), c-go (3.1, 8.9), c-elixir (2.8, 11.1; one stuck at a kill of driftwm), openrc-doc (2.8, 17.7), dinit-wf (1.8, 4.0). dinit-par and dinit-sm left services down after kills (5 and 6 times). |
| **R2 hung but alive (6)** | Noticed by itself: b-go (10.6 to 15.1 s), c-go (9.3 to 13.7), c-elixir (14.1 to 15.4), openrc-doc (19.4 to 20.2). Not noticed (SIGSTOP 3 of 3, supervisor shows "up"): s6plain, s6rc-par, runit, runit-111, every dinit setup, openrc-par. |
| **R3 crash storm (6)** | Services running again after the 40-s storm without a person: s6, runit, b-go, openrc-doc (none of these raise an alert; they restart without limit, 18 to 39 starts in 40 s). c-go and c-elixir give up on purpose and raise the alert (the owner's wording: "degraded and alert"); a person's `retry` ends it. Dinit (all) and openrc-par leave the failed service down (stuck, or a person). |
| **R4 slow or failed dependency (5)** | Slow seatd: dependents recovered in 18.8 to 22.6 s in the s6, runit, b-go, c-go, c-elixir, openrc-doc, dinit-wf candidates (dinit-par 33.9, dinit 32.9); the s6, runit and OpenRC candidates start dependents 2 to 12 times while waiting (the polling of `follow-driftwm`), b-go 1 time each, the brains 1 to 19. Failing seatd: recovered in s6, runit, b-go, c-go, c-elixir, openrc-doc; stuck or a person in dinit (all), openrc-par. |
| **R5 disk full (4)** | No candidate stalled or lost a service; recovery 0.1 to 0.2 s (openrc-par 3.4 s). |
| **R6 supervisor killed (5)** | s6plain, s6rc-par, runit, runit-111: the scanner restarts the supervisor, the service keeps running (0.1 to 18 s). b-go, dinit: no separate supervisors (not applicable). c-go, c-elixir: 1 of 5 each ended stuck (supervisor-killed driftwm). OpenRC: nothing restarts `supervise-daemon`; the service keeps running, but the next kill of the service needs a person (6 of 12 supervisor events in openrc-par and in openrc-doc). Scanner killed: stock runit reboots the machine; runit-111 and runit-111np restart stage 2 (5 s) with lock-error lines. |
| **R7 PID 1 killed or wedged (6)** | PID 1 killed: machine back in 15 to 27 s in every candidate. Wedged: guard noticed within 0 to 2 s (where logged), machine back in 45 to 52 s in every candidate. The harness's guard and the reset path are the same for all, so this part shows little difference. |
| **R8 out of memory (4)** | The memory hog was killed and no service was lost in every candidate (0.1 to 0.2 s). The stand-in's protection came from the `policy` wrapper or native keys; not compared separately. |
| **R9 clock jumps (4)** | All candidates recovered from +1 day and -1 hour in 0.1 to 7.3 s, except: openrc (first setup) needed a person for both, runit-111 had one event with waybar missing after -1 hour. |
| **I1 trial-boot confirm tied to health (5)** | A driftwm wedged 2 s after it worked was confirmed falsely in s6plain, s6rc-par, runit, runit-111, dinit (all), openrc-doc2; the brain candidates did not confirm falsely (32.6 to 41.7 s, b-go 37.5). A driftwm that cannot start: rolled back at 60 s everywhere except stock runit and runit-111 (confirmed falsely with a service missing). OpenRC's own hook rolled back every trial including the healthy one; the cause was my test hook (six slow `rc-service` calls), fixed in `openrc-doc2`. |
| **I3 ladder that ends in an alert (6)** | Only c-go and c-elixir (alert file after the limit; storm on driftwm and a supervisor kill reached it). b-go has a ladder in its source (give up after 6 failures in 60 s) but in the 40-s storms the harness found no alert (`alert=no`; the reason was not examined: UNKNOWN). The others have no limit and no alert. |
| **I4 no service-failure reboot (4)** | Stock runit reboots at a scanner kill (29 times in the soak). No other candidate rebooted itself because of a service fault. |
| **I5 dependents follow (4)** | All candidates with the follow rule: yes (polling script `follow-driftwm`, the owner's own); the brains did it by their graph (start counts of 1 to 2 for hubd while the compositor restarted). |
| **Low weight: boot, memory, idle CPU** | Supervisor start to desktop (median s): dinit 5.0 to 6.2, s6plain 5.8, runit 6.1, s6rc 6.4, c-go 6.4, c-elixir 6.3, b-go 7.3, s6rc-par 7.9, openrc 8.4 to 10.2. PSS of PID 1 plus supervisors plus layer (kB): runit 1880, dinit 2096, s6plain 2466, s6rc-par 2674, openrc-doc 3056, b-go 5028, c-go 10962, c-elixir 35574. Init-side idle CPU in 30 s (ticks): 0 to 13 for the plain inits, 29 (b-go), 234 to 267 (c-go, c-elixir). Code size was not measured here. |
| **Memory over 4 h** | No slope was distinguishable from zero except s6plain (+21 kB per 1000 restarts, interval +1.7 to +40.0, i.e. 2 to 40 bytes per restart); b-go's interval is wide (±1.2 MB per 1000 restarts; ±240 kB with forced collections). |
| **The hang rule (owner's decision)** | Run log, follow-ups 3 and 4. With the owner's pairing (both compositor probes) and the budget in a file, under s6rc-par and runit-111 alike: 19 of 19 hangs (SIGSTOP 7, busy loop 6, deadlock 6 per init) killed after 31.9 to 33.4 s, desktop back after 35.6 to 38.9 s, the stand-in windows (waybar, hubd) back every time, no person; b-go's own probe: 10.6 to 15.1 s. Hangs 4 to 6 in 10 minutes: capped, state file `capped`, 3 of 3 needed a person. The cap as built is a window: the frozen compositor was killed 485 s and 492 s after the cap was reached, when the oldest kill was 10 minutes old. A watcher killed between kills was restarted by the init (1.1 s s6, 0.2 s runit) and read the budget from the file. A crash storm was killed by the rule in 12 of 12 storms (9 found no process); not a hang. Cost per init: see follow-up 3, item 4 (OpenRC and b-go: not run). |

Not in this table because the harness did not measure them: I2 (native probe: the brains and OpenRC have one; the others none), I6 to I10, and the ten candidate features of 1.7. Their ratings, if the owner wants them, come from reading the init's documentation (SOURCE), not from these runs.

## 6. Open questions for the owner

1. OpenRC (`openrc-doc`): 0 stuck, 0 self-reboots, but a killed `supervise-daemon` is not restarted (no documented option). The owner's decision of 2026-10-12 records it as passing the elimination rules but not recovering by itself; is that enough, or is a restart of the supervisor required?
2. runit: the 53 lock-error lines of runit-111np come from two `svlogd` on one log directory after a supervisor kill (no line lost); its 2-hour soak had 1 stuck state and a long lock-line period while the stand-in's `/var` was full (cause UNKNOWN). Should it be soaked again with a larger stand-in `/var` (run log, follow-up 4, question 6), and is a lock-error line that does not lose log lines acceptable?
3. The hang rule (answered by the owner in follow-up 4: the second probe is the Wayland `wl_display.sync` round trip, both compositor probes must fail). New questions from the data: is the cap a window or a latch, when should the state file go, should a crash storm use up the kill budget, probe limits and streak after a watcher restart (run log, follow-up 4, questions 1 to 5).
4. The brains (c-go, c-elixir) and b-go notice a hang by themselves; the owner's decision was a separate watcher. Is a brain still wanted for anything else (crash-storm alert, trial-boot confirm)?
5. Should the c-go brain get the same counters as b-go (4.5 hours), and should c-go or c-elixir be soaked again after the supervisor-killed step is understood?
6. Which scorecard ratings should be computed (points), now that the data exist, and who decides what "2 = a helper we write" means for the watcher?
7. Real hardware: is a run on a real machine (KVM speed, real watchdog) wanted before any decision?
