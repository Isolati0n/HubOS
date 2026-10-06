# PROPOSAL: the hub as separate processes under a supervisor (isolation, failure table, interface rules, resource fences)

**Status: PROPOSAL.** Nothing here is decided, built into the image, or a change to `HUB-OS.md`, the main image or `hubd`. `HUB-OS.md` wins if this file disagrees with it. Written 2026-10-05 on branch `hub-isolation-and-workshop` (from `origin/main`). I recommend and I list questions (section 11); I do not decide design.

**Labels on every item** (as in the other proposals):

- **TESTED**: I ran it in this session; the command and the result are written next to it. Section 3.1 lists the exact experiment.
- **SOURCE**: I read it in a file of this repository (path given) or in a document, on 2026-10-05.
- **BELIEVED**: my reasoning or memory. Not tested, not read in a source.
- **UNKNOWN**: nobody has checked.
- **DESIGN**: a recommendation made here for the owner to decide.

**Helpers:** no helper agents were used. Every file reference was read by me; everything marked TESTED I ran myself.

**Input documents:** `docs/proposals/hub-stability.md`, `docs/proposals/init-comparison.md` (PAUSED and INCOMPLETE: only s6 was measured, and only part of the fault suite), `docs/proposals/hub-user-split.md`, and the repository files named below.

---

## 0. Plain-words summary

1. **The idea.** Today the hub is already several separate programs under a supervisor: `s6` starts `seatd`, `udevd`, `dbus`, `driftwm`, Waybar and `hubd` as separate processes, and restarts a program that dies (SOURCE: `image/machines/hub.build`, `image/machines/hub/rootfs/etc/s6/sv/*`). The proposal is to keep going in that direction on purpose: every job that can fail on its own is its own process with exactly one owner for each piece of state, so that when one program dies, hangs or is slow, only that job is affected and everything else keeps going. The planned pieces that do not exist yet (sound mixer, input receiver, layout store, clipboard, phone web page, node helper clients) are listed in section 2 with the process each would be.
2. **What exists and what is only planned.** These exist in the image today: PID 1, the supervisor, seatd, udevd, dbus, driftwm, Waybar plus wofi, `hubd`, the confirm step and the watchdog feeder. The viewer is a test stand-in (`foot`); the real viewer is an open question. The mixer and PipeWire, the input receiver, the layout store, the clipboard client, the phone web service and the node-helper clients **do not exist** (section 2).
3. **What happens when each piece fails (section 3).** I killed (`kill -9`) and froze (`SIGSTOP`) each of seatd, udevd, dbus, driftwm, Waybar and hubd inside the hub QEMU image and wrote down what happened, how long recovery took and what restarted with it. In short (TESTED, one run, coarse timing): a killed program is restarted by the supervisor within about one to two seconds and the desktop keeps answering; killing dbus also takes Waybar down and killing driftwm restarts Waybar and `hubd` with it, nothing else cascades. A **frozen** program is never noticed: the supervisor says "up" and nothing restarts or alerts, for all six. A frozen `hubd` leaves the bar waiting for ever; a frozen compositor leaves `hubd` answering "3 of 4 up", so the boot-confirm step would call such a release healthy.
4. **A frozen program is the weak spot.** The supervisor only knows whether a process *exists*, not whether it still *works*. A frozen compositor or a frozen `hubd` is invisible to it (this matches the earlier `init-comparison` finding on the compositor, TESTED there; I repeat it here for all six programs). Section 3 shows what the owner sees; section 5 lists the rules that make a freeze detectable and bounded (a deadline on every call, a heartbeat on every stream).
5. **Five interface rules (section 5):** a deadline on every call; no blocking without a timeout; one owner per piece of state; state files that are written safely and checked when read; versioned messages. For each, section 5 says where today's code already follows it and where it does not (SOURCE and TESTED), and section 8 says how the rule becomes a test.
6. **Resource fences (section 6).** The owner decided: viewers run in a lower-priority, memory-limited group; all viewers together at most 25% of RAM; each at most 1 GiB; reserved CPU for the compositor and `hubd`. **The hub kernel has no control-group support at all today** (TESTED in the hub image, section 3.1, and SOURCE: `image/kernel/hub.frag`), so none of this can be done until the kernel is rebuilt with it. I tested the mechanism on this build machine's kernel (not the hub's): a memory limit kills the leaking program alone, and Go can start a child directly inside a group. I also found a trap: when the sum of the viewers hits the group's limit, the kernel can kill a viewer that did not cause it (section 6.3).
7. **Arithmetic the owner should see (section 6.2):** 25% of RAM spread over 20 windows is only 1/80 of RAM per window. With 32 GiB that is about 410 MiB per window, far below the 1 GiB cap; the cap only matters for fewer windows.
8. **Stale state (section 7)** is what a restarted program wrongly believes about the old world (a socket file, a pid, a window list, a "healthy" flag, a state file written by the previous release). The section lists each case that exists today and what removes it.
9. **I could not test:** a real GPU, real input devices, PipeWire, the not-yet-built pieces, a disk that stalls, and anything about timing on real hardware (QEMU runs here without KVM and the host was heavily loaded, load average 8 to 12). Section 12 lists everything.

---

## 1. Constraints from the owner (SOURCE: `HUB-OS.md` and the owner's task text passed on to me)

- The hub never reboots itself because of a service failure; it reboots only to roll back a trial boot that never became healthy, or when the hardware watchdog fires for a dead kernel or PID 1. A failing service is restarted, then its dependents, then retrying stops and an alert shows on the bar.
- Priorities: (1) uptime and stability, (2) integration, (3) optimisation for the hub's one use, (4) building it ourselves only when it measurably wins, (5) simplicity.
- Network, security, bandwidth and latency are not design concerns.
- The hub is built from scratch eventually (glibc, upstream Linux); the init is s6 now, kept swappable (candidates s6, dinit).
- `hubd` is Go.
- Owner decisions on resources (from the task): viewers in a lower-priority, memory-limited group; all viewers together at most 25% of RAM; each viewer at most 1 GiB; the compositor and `hubd` get reserved CPU and priority (`HUB-OS.md` also records the last).

---

## 2. The components, the state each owns, and its sockets

"Exists" means it is in the hub image today (SOURCE: the files named). "Planned" means a document or `HUB-OS.md` describes it but no code or image part exists.

| # | Component | Exists? | Runs as / started by | State it owns | Sockets and files | Label |
|---|---|---|---|---|---|---|
| 1 | **PID 1**: `/sbin/init`, a shell script, which ends in `exec s6-svscan /run/service` (so PID 1 *is* `s6-svscan`) | exists | root; the kernel starts it from stage 0 | which service directories were copied to `/run/service`; `/run/hubos-slot` | `s6-svscan`'s control files under `/run/service/.s6-svscan/` (BELIEVED, s6's design; not read in its source); its `finish` and `crash` scripts run `reboot -f` | SOURCE `image/rootfs/usr/sbin/init` |
| 2 | **Supervisor**: one `s6-supervise` per service under PID 1 | exists | root | each service's up/down status files in `/run/service/NAME/supervise/` | per-service control FIFOs there (BELIEVED, s6's design) | SOURCE (`init`), BELIEVED (s6 internals) |
| 3 | **seatd** | exists | root | which clients hold the seat | `/run/seatd.sock` | SOURCE `.../sv/seatd/run`, `driftwm/run` |
| 4 | **udevd** (eudev) | exists | root | the udev database `/run/udev/data`, the marker `/run/udev/ready` | `/run/udev/control`, a kernel netlink socket | SOURCE `.../sv/udevd/run` |
| 5 | **dbus** (the session bus, `dbus-daemon` only) | exists | user `hub` | bus connections | `/run/dw/bus` | SOURCE `.../sv/dbus/run` |
| 6 | **driftwm** (compositor) | exists | user `hub`, group `seat` | window list, positions, camera, focus; its session file `session.json` in `/run/hub/state/driftwm/` (RAM) | Wayland socket `/run/dw/wayland-1`; its control socket `$XDG_RUNTIME_DIR/driftwm/ipc-<display>.sock` (mode 0600) | SOURCE `.../sv/driftwm/run`, `hub-env.sh`, `docs/proposals/driftwm-layouts.md` section 4, `hub-user-split.md` section 1 |
| 7 | **Bar and menu**: Waybar (the bar) and wofi (the list, started per click by `hubd menu`) | exists | user `hub`; Waybar waits for driftwm (`follow-driftwm`) | nothing of its own; the bar's text is whatever `hubd feed` prints | none of its own; Waybar's `custom/hub` module runs `hubd feed --socket /run/hubos/hubd.sock` as a child and `hubd menu` on click | SOURCE `waybar.json`, `.../sv/waybar/run` |
| 8 | **hubd** | exists | user `hub`; waits for driftwm (`follow-driftwm`); its log goes through `s6-log` (`n5 s200000`, `/var/log/hubd`, RAM) | the machine list and health results, the panel state, the record of the windows it opened (`/run/hubos/hubd.record.json`) | `/run/hubos/hubd.sock` (mode 0600, folder 0700); driftwm's socket as a client; machines as a client | SOURCE `cmd/hubd/slice2.go`, `internal/hub/ipc.go`, `record.go`, `.../sv/hubd/run`, `log/run` |
| 9 | **One viewer per open machine** | partly: `hubd` starts any program named in `viewers.toml` (own process group, output to a log file); in the image tests the stand-in is `foot`; the real viewer is not chosen | user `hub`; started by `hubd` | its own connection to the machine; nothing the hub owns | a Wayland client of driftwm; the node's display port | SOURCE `internal/hub/viewerlog.go` line 40, `HUB-OS.md` open questions ("Which viewer the hub uses") |
| 10 | **confirm** (once per boot) and **wd** (watchdog feeder) and **console** | exist | root | the confirm step writes `min_version` and clears the failure counter; wd holds `/dev/watchdog` open | `/run/hubos/hubd.sock` (confirm is a client) | SOURCE `image/rootfs/etc/s6/sv/confirm/run`, `wd/run` |
| 11 | **Mixer and PipeWire** | **planned**; not in the image (no PipeWire package in `image/packages/hub.list`) | would be user `hub` | per-node volume and mute, master volume (where they are kept is an open question in `HUB-OS.md`) | PipeWire's socket; a mixer control | SOURCE `HUB-OS.md` (Sound; unverified list), `image/packages/hub.list` |
| 12 | **Input receiver** (the hub side of the keyboard and mouse forwarder) | **planned**; nothing built; `/dev/uinput` does not exist in the build environment and the hub kernel has no `UINPUT` option | would need access to `/dev/uinput` | which machine owns the input | a network port from the gaming box (not chosen) | SOURCE `docs/input-sharing-research.md`, `image/kernel/hub.frag` (grep: no UINPUT) |
| 13 | **Layout store** | **planned**; a design exists (`hubd layout save|apply ...`, files `/config/hubos/layouts/NAME.toml`) but it is part of `hubd`'s plan, not a process | would write to the config partition | named layouts | files on `/config` | SOURCE `docs/proposals/driftwm-layouts.md` section 5 |
| 14 | **Clipboard client** (hub to node push, via the node helper API) | **planned**; "hubd clipboard push ... (not built)" | would read the hub clipboard with `wl-paste`/`wl-copy` | the last text pushed (to stop echo) | node helper API calls | SOURCE `HUB-OS.md` unverified list; `docs/proposals/node-helper-api.md` section 8 |
| 15 | **Phone web service** | **planned**; an owner decision in `HUB-OS.md` ("a web page served by the hub"); no design file found | would face the network | which sessions it exposes | an HTTP port (not chosen) | SOURCE `HUB-OS.md` "Phone access"; grep of `docs/` found no design |
| 16 | **Node helper and recovery agent clients** | **planned**; the node helper does not exist; a test recovery agent exists only as an experiment | in `hubd` today (calls to nodes) | per-machine "state" (running or recovery) | node port 8480 | SOURCE `docs/proposals/node-helper-api.md`, `recovery-and-out-of-band.md` |
| 17 | **Kernel and the hardware watchdog** | exist | the kernel | the watchdog timer (fed only by `wd`, started by `confirm` after a healthy boot) | `/dev/watchdog` | SOURCE `wd/run`, `confirm/run`, `image/kernel/hub.frag` (`WATCHDOG_NOWAYOUT`) |

**Observations (SOURCE and BELIEVED):**

1. **Dependencies are enforced by waiting scripts, not by the supervisor.** `driftwm/run` waits up to 30 s for `/run/seatd.sock` and up to 60 s for `/run/udev/ready`, then starts anyway; `follow-driftwm` makes Waybar and `hubd` wait for driftwm and exit when driftwm's process id changes (SOURCE the three scripts). So "restart the dependents" is already implemented for driftwm -> Waybar and `hubd` only; seatd, udevd and dbus have **no** dependents restarted when they die (TESTED in section 3).
2. **Two copies of the window state.** driftwm owns the window positions; `hubd` keeps a record of the windows it opened (checked against driftwm before it is trusted: SOURCE `record.go` comment). Plan A and "direct restore" of `hub-stability.md` would add a third place (a remembered place per machine). One owner per piece of state (rule 3, section 5) says which one is the master; see question 6.
3. **Waybar depends on `hubd` through a child process.** If `hubd` dies, `hubd feed` (Waybar's child) ends and Waybar must start it again; whether Waybar does that, and how fast, is in the failure table (section 3).
4. **`s6-svscan` is PID 1.** If PID 1 dies the kernel panics (BELIEVED, standard Linux behaviour); the image's `finish`/`crash` scripts also run `reboot -f`. This is the "PID 1 dead" case the owner allows to end in a reboot.

---

## 3. What happens when each piece dies, hangs or is slow

### 3.1 The experiment (TESTED)

- **Set-up.** The hub image of this repository (`image/machines/hub.build`: Linux 6.12, Ubuntu 24.04 snapshot 20261001, driftwm at the pinned commit built from source, eudev 3.2.14) built with the repository's own scripts and booted in QEMU 8.2.2 under software emulation (no KVM; 2 vCPUs, 2 GiB, virtio GPU, USB keyboard and mouse), exactly as `tools/image/hub_test.go` does it (`HUBOS_HUB_WORK` keeps the build between runs). The three fake machines `ai-1`, `desk-1`, `desk-2` run (`tools/fakenode`), so the bar shows "3 of 4 up". Desktop drawn in software (`LIBGL_ALWAYS_SOFTWARE=1`, as the image's `DESKTOP_ENV` says).
- **Experiment file.** `tools/image/experiments/hub-isolation/hubisolation_test.go` (build tag `hubisolation`; throwaway; copy it next to the other tests to run it; **no existing test or file was changed**). `TestHubIsolation` kills (`SIGKILL`) and then freezes (`SIGSTOP`) each of seatd, udevd, dbus, Waybar, hubd and driftwm in turn, one at a time, with the machine healthy before each. `TestHubIsolationDeadlines` measures how long the hub's own small clients wait for a frozen `hubd` or `driftwm`, and records memory and the limits the hub kernel offers.
- **How to run it** (from `tools/image`, as root, with the QEMU tools fetched by the repository script; about 55 minutes cold under this host's load, most of it the kernel and the driftwm build): `cp experiments/hub-isolation/hubisolation_test.go ./zz_hubisolation_test.go`, then `HUBOS_HUB_WORK=<empty folder outside the repository> go test -tags 'qemu hubisolation' -count=1 -run TestHubIsolation -timeout 300m -v .` (and `-run TestHubIsolationDeadlines` for the second test), then delete the copied file. The first test was run before the set-up code was moved into a helper function (`isoSetup`) so that the second test could share it; the only behavioural change was that the throw-away update key is kept between runs; the first test's results are from the earlier version of the file.
- **What "healthy" means in the experiment.** All six programs exist; `driftwm msg state` answers within 8 s and lists Waybar's layer; `hubd list` answers within 8 s. Sampled every few seconds; a sample takes several seconds in this emulation, so **times have a resolution of roughly 5 to 10 seconds**. A screenshot of the final state counts the bar's pixels.
- **What it cannot show.** Real timing (software emulation, loaded host), a real GPU, real input, sound, viewers other than the stand-in. The "owner sees" column is inferred from the bar pixels, `driftwm msg state` and the process list; I did not watch the screen as a person.

**Results (TESTED; one run of `TestHubIsolation`, 12 cases, all 12 completed, `--- PASS: TestHubIsolation (3193.41s)`; command in section 3.1; raw logs `iso1.log` and `iso2.log` are in the session scratchpad, not in the repository).** Each sample took about 15 to 30 seconds in this emulation, so a "back within" figure is an upper bound with that resolution. "New pid" = the program was started again by the supervisor.

| Injected into | What the probes and the process list showed | Programs that got a new process id | Back to healthy |
|---|---|---|---|
| `kill -9` **seatd** | seatd's number was missing in the first sample, then a new seatd (pid 153 -> 1787); `s6-svstat` reported the new one "up 30 seconds" when the sample ended 31 s after the kill, so s6 restarted it about 1 s after. `driftwm msg state` answered and the bar layer was present throughout. | seatd only | within 31 s (first sample at 16 s had no seatd) |
| `kill -9` **udevd** | restarted (170 -> 3043); its run script triggered the devices again (`udevd: devices triggered, database in /run/udev/data (629 entries)`). All probes ok at the first sample. | udevd only | within 20 s |
| `kill -9` **dbus** | restarted (150 -> 3956) **and Waybar exited and was restarted** (console: `follow-driftwm: s6-setuidgid ended`; Waybar pid 894 -> 4021); driftwm and hubd untouched. | dbus, Waybar | within 24 s |
| `kill -9` **Waybar** | restarted (4021 -> 5013); nothing else restarted. | Waybar only | within 24 s |
| `kill -9` **hubd** | restarted (889 -> 6029); Waybar kept its pid; `hubd list` answers again. | hubd only | within 23 s |
| `kill -9` **driftwm** | restarted (157 -> 6873); Waybar and hubd were stopped by `follow-driftwm` and started again (new pids); console `driftwm: starting as user hub 20:39:12`. | driftwm, Waybar, hubd | within 24 s |
| `SIGSTOP` **seatd** | `s6-svstat` kept saying "up"; no restart; all probes ok in four samples over 86 s. | none | n/a (not noticed by anyone); healthy 24 s after `SIGCONT` |
| `SIGSTOP` **udevd** | the same: "up", no restart, probes ok over 80 s. | none | not noticed; ok 27 s after `SIGCONT` |
| `SIGSTOP` **dbus** | the same: "up", no restart, probes ok over 83 s. | none | not noticed; ok 26 s after `SIGCONT` |
| `SIGSTOP` **Waybar** | the same: "up", no restart, probes ok over 78 s (the probes do not look at what the bar shows). | none | not noticed; ok 22 s after `SIGCONT` |
| `SIGSTOP` **hubd** | `hubd list` never answered inside the probe's 8 s (killed by the test's timeout, status 137) in three samples over 99 s; `s6-svstat` still "up"; driftwm and the bar layer fine. | none | not noticed; ok 26 s after `SIGCONT` |
| `SIGSTOP` **driftwm** | `driftwm msg state` never answered inside 8 s (status 137) in three samples over 95 s; `s6-svstat` "up"; **`hubd list` still answered** (status 0); Waybar and hubd not restarted. | none | not noticed; ok 24 s after `SIGCONT` |

What the table shows, in plain words:

1. **A dead program comes back by itself within about one to two seconds, and the desktop kept answering throughout (all six; the one "unhealthy" sample, for seatd, only lacked seatd's own process number).** Only two of the six have anything restarted with them: driftwm (Waybar and `hubd` follow, by the wait-for-driftwm script) and, unexpectedly, **dbus (Waybar follows it down)**, even though nothing in the service files says so; that is Waybar's own behaviour when its bus goes away (BELIEVED; the console line shows the wrapper noticed the child end).
2. **A frozen program is not noticed by anything.** For all six the supervisor reports "up" for the whole freeze (86 to 99 s) and does nothing, and nothing shows an alert. This repeats the earlier `init-comparison` finding for driftwm and extends it to the other five (TESTED here).
3. **`hubd list` answers while the compositor is frozen.** The confirm step's only test of a trial boot is `hubd list` five times in a row, so a release whose compositor hangs would still be **confirmed** (the same false confirmation `init-comparison` measured, now seen from the other side: TESTED here that `hubd list` still answers with driftwm stopped). A frozen `hubd`, on the other hand, would fail the confirm step and roll the release back.
4. **Not seen by this experiment:** whether a restarted seatd or udevd leaves the running compositor able to use its devices (the compositor already had them; the probes only show it still answers), whether the bar's text or clock freezes when Waybar is frozen (the probes cannot see the picture; the screenshot after `SIGCONT` is not informative), and whether hot-plugged keyboards still work after a udevd restart. These need a real device and a person looking, or a more careful test.

### 3.2 What earlier documents already measured (SOURCE, not re-run)

| Fact | Where | Label |
|---|---|---|
| `kill -9` of driftwm: every Wayland client (`foot`, `remote-viewer`) is gone within 0.06 to 0.1 s; only a Qt 6.6+ client with the reconnect switch survives | `hub-stability.md` T3, T5 | TESTED there (nested driftwm) |
| After a compositor crash the compositor answers again about 1.3 to 1.4 s later (1.0 s of that is the supervisor delay modelled) and, with the prototype Plan A, 20 windows are live again at about 2.5 to 3.4 s; today's `hubd` does not do this | `hub-stability.md` 7.2 (T4) | TESTED there |
| A frozen compositor is detected by two probes (driftwm's `state` request and a Wayland `wl_display.sync`) after 30 to 33 s and then killed and restarted with the windows back | `hub-stability.md` 9.2 (T6) | TESTED there (nested) |
| s6 recovers a killed service in a median 3.1 s (worst 7.4 s, 30 kills); a wedged driftwm is never noticed (6 of 6 needed a person); in a trial boot a wedged driftwm is **confirmed** (a false "healthy", 19.8 s) because s6 sees only that the process exists; a driftwm that cannot start leads to a rollback at 60 s; PID 1 killed: machine back in 20 to 23 s; PID 1 wedged (ptrace stop): back in 49 s, but only because that harness fed the watchdog through a guard that checks PID 1 (the hub image's `wd` does not) | `init-comparison.md` status paragraph (PAUSED, incomplete; QEMU without KVM) | TESTED there |
| `hubd` crash: windows stay, `hubd` re-adopts them by name | `hub-stability.md` 3.7 D1, `docs/hubd-slice2.md` 16.2 | TESTED there (earlier) |
| Compositor crash modes found: one-request crash (`wl_shm_pool.resize(0)`), a bad colour in the config, a blocked log pipe, a blocked session-file write | `hub-stability.md` 1 items 2 to 4 (T9 to T12) | TESTED there |

### 3.3 How the failure table is built

For each component: **dies**, **hangs** (alive but not answering; here `SIGSTOP`, which stands for any freeze that keeps the process), **slow** (answers late; not injected, reasoned from the code). For each: what the owner sees, how it is detected, what restarts it, what restarts with it.

Cells say: **owner sees** / **detected by** / **restarted by** / **restarts with it**. A label follows each fact. "Today" means the image as it is; "DESIGN" is what the rules of section 5 would add.

| Component | Dies (`kill -9`, crash) | Hangs (frozen, alive) | Slow |
|---|---|---|---|
| **PID 1 / supervisor** (`s6-svscan`) | Owner sees: the machine reboots (`finish` and `crash` scripts run `reboot -f`; SOURCE `init`). Detected by: the kernel (PID 1 gone) / the script. Restarted by: reboot into the same slot; a trial boot that does not become healthy rolls back. Restarts with it: everything. **Not tested here** (PID 1 killed: back in 20 to 23 s, wedged: 49 s, in `init-comparison.md`, TESTED there, QEMU). | Owner sees: nothing at first. **Today nothing notices a wedged PID 1:** `wd` is a plain feeder (`watchdog -F -t 5 -T N /dev/watchdog`, SOURCE `wd/run`) that does not look at PID 1, so it keeps feeding while `s6-svscan` is frozen (BELIEVED from the script; not run). The 49 s reset in `init-comparison.md` came from that harness's own "guard" that checks PID 1, which is a proposal, not in the image (SOURCE `init-comparison.md` 1.1 rule 7, and its status paragraph). | Owner sees: every restart slower. Not measured. |
| **seatd** | Owner sees: nothing (TESTED: driftwm, Waybar, hubd answered throughout). Detected by: `s6-supervise` at once. Restarted by: s6 (about 1 s, TESTED). Restarts with it: nothing (TESTED). **Untested:** a driftwm that is *started* while seatd is down: `driftwm/run` waits at most 30 s for the socket and then starts anyway (SOURCE), so it would start without a seat and, BELIEVED, exit and be restarted in a loop until seatd returns. | Owner sees: nothing (TESTED: probes fine for 86 s). Detected by: nobody (TESTED). Effect on a driftwm *start* or a new seat client: they would wait forever on the socket (BELIEVED). DESIGN: a probe that opens the seat socket with a deadline. | Slow seat hand-over delays only a compositor start. BELIEVED. |
| **udevd** | Owner sees: nothing. Restarted by: s6, which re-runs the device triggers (TESTED: 629 entries). Restarts with it: nothing. **Untested:** a keyboard or mouse plugged in during the gap is missed until the triggers run again (BELIEVED); this matters for the emergency spare keyboard (SOURCE `HUB-OS.md`). | Owner sees: nothing until a device is plugged in or removed, then the new device is not seen (BELIEVED, not tested). Detected by: nobody (TESTED). | Slow trigger or settle: bounded (`udevadm settle --timeout=30`, SOURCE). |
| **dbus** | Owner sees: the bar disappears and comes back (TESTED: Waybar exited and was restarted). Detected by: s6 for dbus, and for Waybar through the exit of its wrapper. Restarted by: s6. Restarts with it: **Waybar** (TESTED). | Owner sees: nothing in the probes (TESTED); BELIEVED a program asking the bus (Waybar) would wait. Detected by: nobody. | BELIEVED: bar slow to update. |
| **driftwm** (compositor) | Owner sees: **every window disappears and the screen goes empty**, the bar and `hubd` come back about 1 to 2 s later (TESTED here: all three new within the first sample; SOURCE `hub-stability.md` T3: viewers are gone within 0.1 s). Windows do **not** come back with today's `hubd` (TESTED in `hub-stability.md` 7.2 run A2); the owner wants them back (SOURCE `HUB-OS.md`; Plans A to C undecided). Detected by: s6 at once. Restarted by: s6 (about 1 s, TESTED). Restarts with it: Waybar, `hubd` (TESTED), and every viewer dies with it (SOURCE T3). | Owner sees: **the picture frozen, mouse dead, windows stuck**; the bar's clock stops (BELIEVED). Detected by: **nobody today** (TESTED here; `init-comparison` 6 of 6 needed a person). `hubd list` still answers (TESTED), so a trial boot would be **confirmed** (SOURCE `confirm/run` + TESTED). DESIGN: probe from outside (driftwm's `state` request and a Wayland `sync`, `hub-stability.md` 9.1), kill after 4 failed rounds and 30 s; restart by s6; restarts with it: as for "dies". | Owner sees: dropped frames, laggy mouse. Detected by: a frame-time or `state` latency probe (DESIGN). Cause candidates: CPU starvation by viewers (TESTED in `hub-stability.md` 7.6: `state` 0.9 ms idle, 374 ms with 8 normal-priority hogs), a blocked log pipe (T11) or session-file write (T12). |
| **Waybar** (bar) | Owner sees: the bar vanishes briefly (TESTED: back at the first sample). Detected by: s6. Restarted by: s6 (through `follow-driftwm`'s wrapper). Restarts with it: nothing (TESTED). | Owner sees: the bar stays drawn but stops updating (BELIEVED; the probes cannot see it). Detected by: nobody (TESTED). DESIGN: a bar that cannot be trusted to update must have a heartbeat the owner can see (clock) and a check that the bar's layer answers. | BELIEVED: stale text. |
| **wofi** (menu) | Owner sees: the list closes. Started per click, nothing to restart. BELIEVED. | Owner sees: a stuck list; closing it with Escape or another click. BELIEVED, `hubd menu` has no deadline on its child (not read). | - |
| **hubd** | Owner sees: the panel's text briefly empty or stale, then back (TESTED: restarted, `hubd list` answers again at the first sample; Waybar kept its pid so its module restarted `hubd feed`: BELIEVED). Windows stay (they belong to the compositor); `hubd` re-adopts them by name (SOURCE `hub-stability.md` D1, TESTED earlier in `docs/hubd-slice2.md` 16.2). Detected by: s6; the confirm step if it happens during a trial boot. Restarted by: s6. Restarts with it: nothing (TESTED). | Owner sees: **the bar shows its last text forever, and the menu does not open**; windows and everything else keep working (TESTED probes). Detected by: nobody today (TESTED); the confirm step, only at boot (a frozen `hubd` fails the five checks and rolls the release back: SOURCE). `Call` clients wait up to 60 s and `Feed` clients for ever (SOURCE; measured below). DESIGN: heartbeat on `feed`; a probe with a deadline; kill and restart on 4 failed rounds. | Owner sees: slow menu or stale machine states. The probes are bounded per machine (2 s) and in total (5 s) (SOURCE), so one slow machine cannot hold the others (BELIEVED from the code; the 20-window case was not run). |
| **A viewer** (one per open machine) | Owner sees: that one window disappears; the machine is shown as closed again; others unaffected. Detected by: `hubd` (window gone: SOURCE `internal/hub/watch.go`). Restarted by: nobody (the owner clicks again). Restarts with it: nothing. SOURCE `hub-stability.md` V1. | Owner sees: a window that does not respond; compositor unaffected (client sockets are bounded: SOURCE `hub-stability.md` V2, TESTED there in the soak with random `SIGSTOP`). Detected by: nobody. | Owner sees: that window slow; with 20 windows a CPU-hungry viewer slows the compositor (TESTED `hub-stability.md` 7.6). Fence: section 6. |
| **confirm / wd / console** | confirm exits after success by design (`s6-svc -O .`). wd killed: s6 restarts it (BELIEVED, same mechanism); a gap longer than the watchdog timeout resets the machine (TESTED in `init-comparison.md`/`docs/image.md` for the feeder). | wd frozen: the machine resets after the watchdog timeout (intended: it is the "kernel or PID 1 dead" case). | - |
| **Mixer / PipeWire** (planned) | Not built. DESIGN: sound stops; nothing else; restarted by s6; settings come back from the mixer's own state file (rule 4). | Not built. DESIGN: a probe on the mixer's socket with a deadline. | - |
| **Input receiver** (planned) | Not built. DESIGN: the forwarded keyboard and mouse stop; the spare keyboard on the hub still works. The input state "which machine owns the keyboard" must be re-announced on restart (stale state S-class). | - | - |
| **Layout store, clipboard client, phone web service, node-helper clients** (planned) | Not built. DESIGN: each its own failure row once its process boundary is decided (question 7); none may be on the path of the compositor or `hubd`'s panel. | - | - |
| **Kernel / GPU / disk** | Out of scope here: `hub-stability.md` sections 3.1 to 3.4. | | |

**Deadline measurements (rule 1; TESTED, `TestHubIsolationDeadlines`, second run, `ok hubos/tools/image 413.370s`).** Each line is one command run on the guest as the desktop user, wrapped in the BusyBox `timeout -s KILL N`; "seconds" is wall-clock on the guest:

| Command | Condition | Result |
|---|---|---|
| `hubd list --socket /run/hubos/hubd.sock` | everything healthy | answered in 1 s (12 lines) |
| `hubd feed ...` | healthy, killed by the test after 12 s | printed 3 lines, the first `{"text":"3 of 4 up","class":"alert",...}` |
| `hubd list` | **hubd frozen (`SIGSTOP`)** | gave up after **60 s** with `hubd: read unix @->/run/hubos/hubd.sock: i/o timeout` (the 60 s `Call` deadline in `internal/hub/ipc.go`) |
| `hubd feed` | **hubd frozen** | printed **nothing** and was still waiting when the test killed it at 20 s (no deadline in `Feed`) |
| `driftwm msg state` | **driftwm frozen** | still waiting when the test killed it at 40 s (this is driftwm's own client, not ours) |
| `hubd list` | **driftwm frozen** | answered in 1 s (12 lines): hubd does not wait for the compositor to answer a list |
| `hubd feed` | **driftwm frozen** | printed 4 lines in 15 s, the first again `"3 of 4 up"`; nothing in it says the compositor is frozen |

Consequences, in plain words: (1) a frozen `hubd` makes the bar's module wait without limit and show nothing new, and makes the confirm step's `hubd list` wait 60 s per attempt (its own loop ends at the confirm timeout only between attempts, SOURCE `confirm/run`), so a hung `hubd` in a trial boot can keep the confirm step waiting up to 60 s past its timeout. With the real defaults (confirm 120 s, watchdog 180 s) that worst case is 180 s, the same as the watchdog timeout, so the margin the build check enforces (watchdog more than confirm plus 15 s, SOURCE `check-timers.sh`) does not hold for this case. Either way the machine ends up rebooting into the old slot; only which of the two reboots comes first is unclear (BELIEVED arithmetic from the code; not tested); (2) the panel does not tell the owner that the compositor is frozen.

---

## 4. Findings from reading the code that matter for isolation (SOURCE, this repository, read 2026-10-05)

1. **The supervisor does not look at health.** `s6-svscan` restarts a process that exited. `wd` feeds the hardware watchdog without looking at any service (SOURCE `wd/run`), which is exactly what the owner's rule needs ("a service failure never reboots") and exactly why a hung service stays hung unless something else looks (SOURCE `hub-stability.md` 9.4 puts the hang probe in `hubd` or a small program beside it).
2. **`confirm` decides a trial boot is healthy when `hubd list` answers 5 times in a row**, nothing else (SOURCE `confirm/run`). It does not check the compositor or the bar; `init-comparison.md` measured the consequence (a false confirmation with a wedged driftwm, TESTED there).
3. **`hubd`'s own calls already have deadlines in most places**: dialing a socket 1 to 2 s; a request must arrive within 5 s of connecting (server side); the driftwm client uses a 2 s dial and a 5 s deadline; machine probes 2 s each (SOURCE `internal/hub/ipc.go`, `internal/driftwm/driftwm.go`, `internal/hub/hub.go`, `internal/probe/probe.go`). Gaps: `Call` waits **60 s** for a reply (SOURCE `ipc.go` line 237); `Feed` has **no read deadline at all** (SOURCE `ipc.go`); the server writes replies with no write deadline (SOURCE `reply`). Section 5 rule 1; the measured effect is in section 3 (deadline measurements).
4. **`hubd`'s record file is written by temporary file and rename, with no flush and no checksum** (SOURCE `internal/hub/record.go`). It lives in RAM (`/run`) so a power cut is irrelevant, and the file is only a hint that is checked against driftwm before it is trusted (SOURCE comment in the file), which is a good pattern. The layouts design for `/config` already asks for flush and size limits (SOURCE `driftwm-layouts.md` 5.4).
5. **The hub kernel has no control groups, no pressure-stall information and no `UINPUT`** (SOURCE `image/kernel/hub.frag`: it is `tinyconfig` plus listed options; grep for `CGROUP`, `PSI`, `MEMCG`, `UINPUT` finds none). Section 6.

---

## 5. Interface rules (DESIGN)

For every rule: the rule, why, how the code stands today (SOURCE or TESTED), and the test (section 8 collects them).

**Rule 1. A deadline on every call.** Every request from one hub component to another (and to a node) has a deadline chosen by the caller, and a late answer is treated as a failure that is reported, not as "wait". Streams (a connection that stays open, like `hubd feed`) must carry a **heartbeat** so the reader can tell "nothing changed" from "the sender is frozen".
- Today: see section 4 item 3. `hubd feed` sends a line only when something changes, "never more than one line per second" (SOURCE `ipc.go` comment on `feed`), and the reader has no deadline; so a frozen `hubd` leaves the bar showing its last text with no sign that anything is wrong (the measurement is in section 3, deadline results).
- Recommendation (DESIGN): a heartbeat line (for example every 5 s even if nothing changed) and a reader deadline of 3 heartbeats; `Call` deadline set by the caller, default 5 s (the confirm step needs its own: each of its checks takes one `hubd list`).

**Rule 2. No blocking without a timeout.** No hub program waits for another without an upper bound, in code (`context`, deadlines) or in scripts (bounded loops). A bound that runs out ends in a visible, logged state, not in silence.
- Today: good examples: `driftwm/run` waits at most 30 s for seatd and 60 s for udev, `udevd/run` uses `udevadm settle --timeout=30`, `hubd` probes are bounded (SOURCE). Counter-examples: `follow-driftwm` loops forever until driftwm appears (by design, but with no message and no limit; it prints nothing while waiting; SOURCE); `confirm`'s per-attempt `hubd list` can block up to the 60 s of `Call` past its own overall limit (SOURCE reading; measured in section 3).
- DESIGN: every waiting script writes one line when it starts waiting and one when the bound is hit; the bound for "driftwm never comes back" ends in the crash-loop policy of `hub-stability.md` O5 (stop restarting after 5 in 60 s, show a fixed message, never reboot).

**Rule 3. One owner per piece of state.** Each piece of state has exactly one process that writes it; everyone else asks that process or keeps a cache that is thrown away when it cannot be checked.

| State | Owner (recommended) | Today | Label |
|---|---|---|---|
| Window positions, camera, focus | driftwm | driftwm owns; `hubd` has a record of the windows *it opened* (a cache, checked against driftwm) | SOURCE `record.go` |
| Which machine each window belongs to | hubd | hubd (matches by app-id or title) | SOURCE `docs/hubd-slice2.md` |
| Machine list and health | hubd | hubd | SOURCE |
| "Where each machine's window should go" after a compositor restart | **open**: driftwm's stand-ins (Plan A) or hubd's own memory (direct restore) or the layout store | not built | SOURCE `hub-stability.md` 7.2, 9.6 (question 6 below) |
| Named layouts | layout store (hubd or a small process) | not built | SOURCE `driftwm-layouts.md` |
| Mixer settings (per-node volume, mute) | the mixer | not built; where they live is an open question | SOURCE `HUB-OS.md` |
| Credentials | none (owner, 2026-10-06: no display passwords, no request signing, no user split) | - | - |
| The hub clipboard | the compositor (Wayland owns the selection); the clipboard client only reads/sets it | not built | SOURCE `node-helper-api.md` 8 |
| Boot and update state (slot, floor, failure counter) | `hubos-ctl` and stage 0 only | as designed | SOURCE `distro-contract.md` C8, C10, C18 |

**Rule 4. Safe state files.** A state file is written to a temporary name in the same folder, flushed, renamed, and the folder flushed; it carries a format number, a length limit and a checksum; a reader treats it as untrusted: a missing, short, wrong-checksum or unknown-version file is ignored with one logged line and the program starts from empty state, never crashes and never blocks.
- Today: `record.json` (RAM): temp + rename, no flush, no checksum, but verified against driftwm before use (SOURCE). driftwm's own session file is written on the compositor's main thread, can block it, and is not flushed (SOURCE `hub-stability.md` S1, `driftwm-layouts.md` 5.4); it lives in RAM on the hub. Files on `/config` that stage 0 reads (`node.conf`) are shell-readable `KEY=VALUE` files read with `sed` and sourced by stage 1 (SOURCE `distro-contract.md` C16). **A state file that survives a rollback is read by the older release:** files on `/config` written by release N+1 are read by release N after an automatic rollback (BELIEVED consequence of A/B plus a shared config partition; nothing tests it). That is why the format number matters.
- DESIGN: RAM files get checksum + format number; `/config` files get flush as well; both get size caps.

**Rule 5. Versioned messages.** Every message between components, and every file, carries a format number; a receiver that does not know it refuses it by name and says which versions it knows.
- Today: the hubd socket protocol has no version field (SOURCE: `Request` has `Cmd`, `ID`, `Line`, `Flat`, `Filter`); the inventory has `format = 1` (SOURCE `docs/inventory-format.md`); the node helper API has a version rule in its design (SOURCE `node-helper-api.md` 3.2, "nothing implements it yet").
- Note (BELIEVED): inside the hub all programs come from one image, so they are always the same release; skew between hub programs occurs only between a restarted program and a long-lived one from the *same* release, which cannot differ. Versioning inside the hub therefore matters for **state files across a rollback** and for **hub to node** messages, much less for hub-internal sockets. Cheap to add anyway.

**Rule 6 (added). A restart is idempotent and never inherits a belief.** A program that starts must not need to know whether it is the first start; it re-reads the world (socket exists? still answers? pid still the same program?) and removes only a leftover that nobody answers on. `hubd`'s `Listen` already does exactly this for its own socket (SOURCE `ipc.go`: another hubd answering is an error; a leftover socket nobody answers on is removed; a file or folder at that path is never removed). Section 7.

**Rule 7 (added). Bounded restarts, then an alert, never a reboot.** Restart a failing program, then its dependents, then stop retrying and show an alert on the bar (`HUB-OS.md`). Today s6 restarts without limit (BELIEVED, s6's default; the crash-storm test was not run for s6 in `init-comparison.md`), and nothing shows an alert for a program stuck in a restart loop.

---

## 6. Resource fences

### 6.1 What the owner decided (SOURCE: the task, `HUB-OS.md`)

Viewers run in a lower-priority, memory-limited group. All viewers together at most 25% of RAM. Each viewer at most 1 GiB. The compositor and `hubd` get reserved CPU and priority.

### 6.2 Arithmetic (BELIEVED; arithmetic only)

- Group limit `L = 0.25 x RAM`. With the owner's cap of 1 GiB per viewer, the cap and the group limit conflict as soon as there are more than `L / 1 GiB` viewers at their cap: at 32 GiB RAM, 8 viewers; at 64 GiB, 16 viewers. The hub is designed for 20 open windows (SOURCE `HUB-OS.md`).
- Average allowed per window with 20 windows: `0.25 x RAM / 20 = RAM / 80`: 32 GiB gives 410 MiB, 64 GiB gives 819 MiB. The hub's RAM is not chosen (December); the hub also copies its root (about 160 MB) into RAM (SOURCE `HUB-OS.md`, `hub-stability.md` 7.5).
- A viewer's real memory is UNKNOWN: the viewer is not chosen, and one 3840x2160 frame at 4 bytes per pixel is 33 MB (BELIEVED arithmetic), so a few frame buffers per window are tens to hundreds of MB. Nothing was measured with a real viewer.
- Measured here (TESTED, hub image booted in QEMU with 2 GiB RAM and a software-drawn desktop, `TestHubIsolationDeadlines`; **no viewer was open**, three fake machines only): resident memory (`VmRSS`) of the hub's own programs: driftwm 121,696 kB (10 threads), Waybar 42,840 kB (8 threads), `hubd` 4,636 kB (8 threads), dbus 3,908 kB, udevd 2,316 kB, seatd 1,588 kB, about 177 MB in all. `oom_score_adj` is 0 for all of them. The shell the test used showed `ulimit -n` 1024 (the per-process open-file limit; the `HUB-OS.md` open question about setting `ulimit -n` for the hub is therefore not yet answered by the image; whether the s6 services have the same limit was not checked). Free memory after boot: 1,931,484 kB of 2,042,484 kB. With a real GPU driver driftwm's numbers will differ (UNKNOWN).

### 6.3 Mechanisms, and what the hub kernel lacks

| Need | Mechanism | In the hub kernel? | Label |
|---|---|---|---|
| A memory ceiling per viewer and for all viewers | control groups, memory controller (`memory.max`; or v1 `memory.limit_in_bytes`) | **No control groups at all.** TESTED in the booted hub image: `/proc/cgroups` ("No such file or directory"), `/sys/fs/cgroup` (same) and `/proc/pressure` (same) do not exist; the built kernel configuration has `# CONFIG_CGROUPS is not set` and `# CONFIG_PSI is not set` | TESTED (section 3.1 second run; distro-workshop 9.3), SOURCE `hub.frag` |
| Lower CPU priority for viewers, reserved CPU for compositor and `hubd` | cgroup CPU weights (and optionally a CPU set), not real-time priority | no (same) | SOURCE `hub-stability.md` 7.6 for the design; kernel options needed: BELIEVED `CGROUPS`, `CGROUP_SCHED`, `FAIR_GROUP_SCHED`, `MEMCG`, `CPUSETS` (and `PSI` for pressure files); I did not check the exact option names in the kernel's `Kconfig` |
| Someone has to mount the control-group file system and create the groups | a small oneshot in the image (no systemd does this for us) | no | BELIEVED |
| Start each viewer inside its group | `hubd`'s launcher (`exec.Command` with `Setpgid`) can place the child directly in a cgroup-v2 group when it starts | n/a | TESTED on this build machine (not the hub kernel): Go's `SysProcAttr{UseCgroupFD: true, CgroupFD: fd}` started `sleep` and `/proc/PID/cgroup` showed `0::/hubiso`; works for cgroup v2 only (BELIEVED, as the feature uses a v2-only kernel call; not read) |
| A memory limit that kills only the leaking viewer | one group per viewer with its own limit | n/a | TESTED on this build machine's kernel (6.18, cgroup v1): a group limited to 64 MiB killed a program that grew past it (exit 137) and nothing else; the group's `oom_kill` counter went to 1 |
| A limit for all viewers together | a parent group with the 25% limit | n/a | TESTED (same host, one run, scaled down): parent 96 MiB, children 64 MiB each; one child held 50 MiB, the other grew to 60 MiB; the **50 MiB child was killed** (exit 137) and the 60 MiB child survived. A single run, timing dependent: BELIEVED that the kernel picks the biggest at the moment of overflow, which was not the one that caused it. **The parent limit can kill an innocent viewer.** |
| Only the leaking viewer is killed, whatever the sum | a user-space watcher that reads each viewer's memory and kills the largest above its cap, with the group limit only as a back-stop above the sum of the caps | not designed | DESIGN / BELIEVED |
| Without cgroups: `nice`, `oom_score_adj` | per-process knobs; need no kernel option | `oom_score_adj` exists (TESTED: readable in `/proc/PID/oom_score_adj`, value 0 for all six hub programs) | TESTED that the file exists and is 0; BELIEVED that writing it works in the hub kernel (not tried) |
| Without cgroups: address-space limits (`RLIMIT_AS`) | `s6-softlimit` | n/a | `hub-stability.md` T14 measured driftwm segfaulting at start-up under a 900 MB address-space limit and starting at 1.3 GB (TESTED there); BELIEVED to be a poor tool for GPU clients (large virtual size) |
| Reserved CPU measured | | | `hub-stability.md` 7.6 (T15): 8 CPU hogs cut driftwm's event rate from 4.8 to 2.0 per second at normal priority and to 4.0 at `nice` 19; `state` took 0.9 ms idle, 374 ms and 148 ms respectively (TESTED there, shared 4-core machine, software rendering at 3840x2160) |

### 6.4 Recommendation (DESIGN)

1. Add the control-group options (and pressure information) to `image/kernel/hub.frag` and test the hub still boots; this is the prerequisite for everything in this section. (A kernel rebuild is about 3.5 minutes with 4 CPUs, SOURCE `docs/image.md` section 5; the QEMU hub test takes 22 to 26 minutes.)
2. A group tree `core` (compositor, `hubd`, seatd, udevd, dbus, supervisor) and `viewers/<machine>`: the viewers' CPU weight low, `core` high, `core` memory floor set; no real-time priority (SOURCE `hub-stability.md` 7.6).
3. The 25% group limit as a back-stop only; the 1 GiB cap per viewer as the first limit; a watcher (in `hubd` or beside it) that kills the *largest* viewer above a soft threshold before the group limit is reached; `oom_score_adj` high on viewers and low on `core`.
4. The viewers' memory limit must come out of a **measurement with the real viewer** in December: question 4.

---

## 7. Stale state

"Stale" = something a restarted or still-running program believes about the world that is no longer true. Each case, what exists today, and the remedy.

| # | Stale thing | Where it arises | Today (label) | Remedy (DESIGN) |
|---|---|---|---|---|
| S1 | A **socket file** left by a killed program | `/run/hubos/hubd.sock`, `/run/dw/wayland-1`, `/run/seatd.sock`, driftwm's IPC socket | hubd removes a leftover socket nobody answers on (SOURCE `ipc.go` `Listen`); `driftwm/run` removes `wayland-1` and its lock before starting (SOURCE); TESTED (section 3): after `kill -9` the restarted seatd and the restarted driftwm both came up and answered, so their leftover socket files did not block them (which of the programs removes its own old socket file was not checked) | the same pattern for every socket: "connect first; remove only if refused" |
| S2 | A **process id** remembered by a script | `follow-driftwm` uses driftwm's pid as the identity of the compositor and stops its child when it changes (SOURCE) | works; a recycled pid is possible in principle (BELIEVED) | compare start time as well as pid |
| S3 | **Windows** hubd believes are open after the compositor restarted | hubd's window cache and record | hubd keeps the record only if the compositor identity matches (SOURCE `record.go`: `Driftwm` field); stand-in windows are mistaken for open windows by today's hubd (TESTED in `hub-stability.md` 7.2 run A2) | read the `suspended` flag (SOURCE/TESTED there); owner of "where windows go" decided (question 6) |
| S4 | **"Healthy"** claims | `confirm` decided "healthy" 5 times at boot and then never again; the `wd` feeder never looks at services | by design | continuous health belongs to the hang probe (`hub-stability.md` 9) |
| S5 | **Machine up/down results** that are old | hubd's probe rounds | hubd has a STALE rule for results older than a round (SOURCE `HUB-OS.md` change log, `docs/hubd-slice2.md`) | keep; test it for a frozen prober |
| S6 | **A bar that shows an old line** when `hubd` froze | Waybar shows whatever `hubd feed` last printed | no heartbeat (section 5 rule 1) | heartbeat; show "hubd not answering" |
| S7 | **State files written by the other release** after a rollback | `/config` files, layouts, mixer settings, the failure counter | untested (BELIEVED) | format number and tolerant readers (rule 4, rule 5) |
| S8 | **Marker files**: `/run/udev/ready`, `/run/hubos-slot` | `udevd/run` removes `ready` at start and recreates it after settling (SOURCE); `/run` is tmpfs so everything is gone at reboot | fine at reboot; after a udevd restart `ready` disappears until settling ends (SOURCE) | dependents only wait at their own start |
| S9 | **A viewer window for a machine that went away** | node down, window open | by owner decision the window is left alone and only the alert changes (SOURCE `HUB-OS.md`) | no remedy needed |
| S10 | **Logs and counters that grow** | driftwm's output has no log service in the image (only hubd has `s6-log`) (SOURCE `hub-stability.md` S3, S4) | driftwm's stderr goes to the supervisor's output; a blocked reader freezes the compositor (TESTED there, T11) | give driftwm a capped `s6-log` |

---

## 8. How each rule becomes testable in the hub test (DESIGN)

The existing hub test (`tools/image/hub_test.go`, H0 to H5, 22 to 26 minutes) boots the image, drives it over the serial console and the QEMU monitor and reads screenshots. Two kinds of test, because most rules do not need a virtual machine:

**(a) Plain Go tests (seconds, run with `go test ./...`).** For rules about one program's behaviour with a fake peer, in the style of the existing `internal/hub` tests that use fakes for the launcher and the compositor (SOURCE `internal/hub/hub_test.go`, `hub-user-split.md` section 1 on the seams):

| Rule | Test |
|---|---|
| 1 deadline | A listener that accepts and never answers: `Call` must return an error within its stated bound; a listener that sends one line and then nothing: `Feed` must report "no heartbeat" within 3 heartbeat periods. (Today the first returns after 60 s and the second never: measured in section 3.) |
| 2 no blocking | A fake compositor whose socket accepts and never replies: `hubd`'s window code must return inside 5 s (SOURCE: the deadline exists; test it stays). |
| 4 safe files | Write the state file, truncate it, flip a byte, replace it with a newer format number, make the folder read-only: the reader returns "empty state" and one log line each time; never panics. |
| 5 versions | A request with an unknown `v` is refused by name. |
| 6 idempotent restart | Start `Listen` on a path with a stale socket, with a live socket, with a regular file, with a folder (the existing behaviour; SOURCE `ipc.go`; add the tests). |

**(b) Hub-image subtests (added later to the hub test, same style as H2).** Each uses `asHub`, `waitDesktop` and the screenshot helpers.

| Test | Steps | Pass rule |
|---|---|---|
| H6 kill each | For each of seatd, udevd, dbus, driftwm, Waybar, hubd: `kill -9`, wait | Healthy again within the bound in section 3's table; dependents as the table says; bar drawn |
| H7 freeze each | `SIGSTOP` each, probe, `SIGCONT` | The detection rule of the table fires (once a probe exists) within its window; after `SIGCONT` or after the probe's kill the desktop is healthy |
| H8 restart limit | A service whose `run` exits at once | After the limit, s6 stops restarting (or the wrapper does), the bar alert shows, the machine does **not** reboot, `hubd` still answers |
| H9 stale state | Kill driftwm with windows open; check no duplicate and no ghost window in hubd's list | list equals driftwm's state |
| H10 fences | (needs the cgroup kernel) a viewer that grows without bound is killed alone at 1 GiB; a CPU-hog viewer does not slow `driftwm msg state` beyond a bound | the compositor's probe time stays under a bound |
| H11 rollback and state files | Write a state file with a newer format on `/config`, roll back to the older release, boot | the old release ignores it and the desktop comes up |

The pass bounds must come from the December hardware, not from the QEMU numbers here (software emulation; loaded host); the QEMU test checks "recovers and does the right thing", not "recovers in N seconds" (BELIEVED to be the only honest use of this emulation).

---

## 9. Recommendations (DESIGN; none is decided)

1. **Keep the process-per-job structure** and make three additions that cost little: a **heartbeat on `hubd feed`** (so a frozen `hubd` is visible); a **`s6-log` for driftwm**; and a **continuous health probe** (outside driftwm, outside s6) following `hub-stability.md` 9.1, because the supervisor cannot see a freeze.
2. **Give the dependents of seatd, udevd and dbus the same "follow" treatment as driftwm's** if the section 3 results show they do not recover by themselves (see the table).
3. **Add the control-group kernel options now** (a kernel option change, cheap to test) so the fence work can be built and tested before December.
4. **Make the viewer memory cap and the 25% group limit a back-stop plus a watcher**, not the group limit alone (the innocent-victim result, section 6.3).
5. **Make state files follow rule 4**, starting with the ones that will live on `/config` (layouts, mixer settings), before any is written.
6. **Write the tests of section 8(a) first**: they are fast and prove the rules without a virtual machine.
7. **Do not add new processes for the planned pieces until the viewer and the "who owns window places" questions are answered**; the process boundaries depend on them.

---

## 10. What this document does not say

It does not decide how windows come back after a compositor crash (Plan A, B or C of `hub-stability.md`), which viewer is used, what the restart-loop policy is, or whether `hubd` is split into two users (`hub-user-split.md`). It gives the failure behaviour of today's pieces and the rules to hold future ones to.

---

## 11. Questions for the owner (nothing is decided here)

1. **Heartbeat on the bar.** May `hubd feed` send a line every few seconds even when nothing changed, so the bar can show "hubd not answering" when it freezes? (Section 5 rule 1.)
2. **Who looks for freezes?** The supervisor cannot. May a small separate program (or `hubd` itself) probe driftwm and `hubd` and kill-and-restart a frozen one, never rebooting, as `hub-stability.md` 9 proposes (4 failed rounds and 30 s, at most 3 kills in 10 minutes)? And who watches the watcher (`hubd` frozen)?
3. **Restart limit.** After how many restarts of one service (hub-stability suggests 5 in 60 s) should the supervisor stop and show the alert? And what should happen to the services that depend on it (stopped, or left running with a message)?
4. **Fence numbers.** With 20 windows, 25% of RAM is only RAM/80 per window (section 6.2). Is 25% the intended ceiling for all viewers even when 20 are open, or should the 1 GiB cap rule and the group limit be higher? What RAM does the hub get (it changes the answer)?
5. **Control groups in the hub kernel.** May the hub kernel get the control-group and pressure options (and the small image step that mounts them)? This is required for any fence.
6. **Who owns "where windows go"** after a compositor restart: driftwm's stand-ins, `hubd`'s own memory (direct restore), or the layout store? (Rule 3; `hub-stability.md` question 6.)
7. **Process boundaries for the planned pieces.** Should the mixer, the clipboard client, the layout store and the phone web service be separate processes from `hubd` (more isolation, more sockets) or parts of it for now (fewer parts)? Every separate process needs its own socket, state owner and failure row.
8. **A state file from a newer release read after a rollback.** Is it acceptable that an older release ignores a newer file (and loses that setting) rather than failing?
9. **Alert wording.** What should the bar say for each case: "desktop restarted", "hubd not answering", "stopped retrying X"? (Plain words; the bar currently shows "3 of 4 up".)
10. **How strict should the hub test be about time?** Only "recovers correctly" (QEMU can show this) or also "within N seconds" (needs real hardware; December)?

---

## 12. What I did not verify

- **The pieces that do not exist** (mixer and PipeWire, input receiver, layout store, clipboard client, phone service, node helper): only described in section 2 from the documents. No failure row for them is TESTED.
- **A real GPU, real input devices, real sound, a real viewer, a real network, real timing.** The experiment ran in QEMU 8.2.2 under software emulation on a shared 4-CPU machine with a load average of 8 to 12; times have a coarse resolution (section 3.1).
- **A disk that stalls under the RAM-copied root** (`hub-stability.md` S1): not run.
- **s6's internal behaviour** (control files, restart delay, whether `s6-svscan` rate-limits restarts): read from experience and from `init-comparison.md`'s partial results, not from s6's source or manual.
- **cgroup tests ran on this build machine's kernel** (6.18, cgroup v1 for memory, cgroup v2 for the Go test), not on the hub kernel, which has no cgroups. What the hub kernel will do with the same limits is UNKNOWN until it is rebuilt.
- **The exact kernel option names** for cgroups and pressure information were not checked against the kernel's `Kconfig`.
- **The innocent-victim result** is one run; it is consistent with the kernel's documented behaviour (BELIEVED) but not repeated.
- **Frozen programs other than by `SIGSTOP`**: a busy loop, a deadlock and a blocked write behave differently for a supervisor than a stopped process only in how they look to `/proc`; `hub-stability.md` T6 covers those for driftwm (nested).
- **No helper agents were used.**
