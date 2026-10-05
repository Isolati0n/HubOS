# Hub stability: designing out freezes and crashes (research and proposal)

**Written:** 2026-10-05. **Status:** research and proposal. Nothing here is a decision. Nothing here changes `HUB-OS.md`, the main image, or any program in this repository. Every choice that belongs to the owner is collected in section 12.

**What this is for.** The owner's top priority for the hub is uptime: it must never freeze or crash, and as many causes of instability as possible must be designed out. This document lists what can make the hub freeze, crash, hang or become unusable, says how likely and how bad each is, what removes or contains it, what that costs, and how to test it. It then judges five architecture ideas, audits the compositor (driftwm) source, and reports every test I could run in the cloud build environment.

## 0. How to read this

Every item carries exactly one label:

- **TESTED**: I ran a command in this session and it gave the result written here. The test numbers (T1, T2, ...) are listed in section 1, with the command and the scripts (kept in `docs/proposals/hub-stability/`).
- **SOURCE**: I read it in a document or in program source. The file and line (or link) is given, and today's date (2026-10-05) is the date I read it. Web pages summarised by a fetch tool are marked "(summary by the fetch tool)": I did not read those pages myself, so treat them as weaker than my own reading of source.
- **BELIEVED**: my reasoning, memory or proposal. Not tested here.
- **UNKNOWN**: I do not know and could not find out here.

"(opinion)" next to a likelihood or an impact means it is my judgement, not a measured number.

No helper agents were used. Everything below I ran or read myself. Web pages were read through a fetch tool that returns a summary; where the summary is all I have, the text says so.

## 1. The short version (plain words)

**Nothing can be guaranteed.** No software, and no hardware, can promise "never crashes". What can be done is five things at once: (1) make each failure rare, (2) make each failure small, so only the failing part restarts, (3) make recovery automatic and fast, (4) notice "stuck" reliably, and (5) never reboot the whole hub because one program failed (the owner's rule). The rest of this document is about doing those five things as cheaply as possible.

**What I found, in ten lines (each is explained and labelled later):**

1. **When the compositor (driftwm) dies, every window dies with it.** TESTED (T3): after `kill -9` of driftwm, `foot` and `remote-viewer` were gone within 0.06 to 0.1 seconds. Wayland programs cannot outlive their compositor unless they are written for it. A Qt 6.6+ program can (T5), nothing else we use can.
2. **driftwm can be crashed by a program that sends one wrong request.** TESTED (T9): five lines of Python sending `wl_shm_pool.resize(0)` kill the unmodified driftwm release build (exit status 101). The bug is in Smithay (the library driftwm is built on), file `src/wayland/shm/handlers.rs` line 190, and it is still in Smithay's master branch today (I downloaded the file and read it). SOURCE.
3. **A typing mistake in driftwm's config file kills the running compositor.** TESTED (T10): writing a colour like `"#aéaaa"` (a letter with an accent) into the config file made the running driftwm exit with status 101 within seconds, because driftwm reloads its config by itself. `driftwm --check-config` crashes on the same file.
4. **driftwm freezes (without crashing) in two ways I could reproduce.** TESTED (T11, T12): if the program reading its log output stops reading, driftwm blocks in `write` and freezes; if a file it writes on its main thread (the session file) blocks, it freezes. Both are "event loop blocked" bugs of the kind its own `caveats.md` warns about. The first is a real risk with the current s6 service layout; the second is removed by keeping driftwm's state in RAM, as `HUB-OS.md` already says.
5. **The fastest way to get windows back after a compositor crash already exists in driftwm** (saved window positions come back as labelled stand-ins that can relaunch their program). TESTED (T4): with 20 fake machines, after `kill -9` the compositor was answering again in 1.3 to 1.4 seconds (1.0 s of that is the 1-second pause I modelled on the s6 restart delay), all 20 stand-ins were already in the right place, and after the viewers were relaunched all 20 windows were back **in exactly their old position and size**. Everything was live again about **2.5 s** after the crash with `foot` as the viewer, and about **3.4 s** with a prototype `hubd`. With the `hubd` that exists today the path **does not work** (T4, run A2: `hubd` mistakes the stand-ins for open windows, so nothing is relaunched); a first prototype that launches the viewers one by one brought back only 9 of 20 windows to their places (run A3), because driftwm only matches a new window to a stand-in for 5 seconds. The 3.4 s result needed four small `hubd` changes (section 7.2). A variant with no stand-ins at all, where `hubd` remembers each window's place and puts it back itself ("direct restore"), gave 3.35 s (run A6) and is the only route that works for the Moonlight machines, because they all share one window name and driftwm's stand-ins cannot tell them apart (section 9.5).
6. **A hung compositor can be detected from outside and killed safely.** TESTED (T6): `SIGSTOP`, a busy loop and a deadlock (a mutex locked twice) were each detected by two probes (driftwm's socket `state` request and a Wayland `wl_display.sync` round trip) after 30 to 33 seconds, then killed with the evidence written down, then restarted by the supervisor, and the windows came back (same 20/20 result as above). The window is deliberately longer than any GPU reset I could find a figure for, which avoids the mistake KDE made in 2024.
7. **"Root copied to RAM" is cheap here and worth doing:** the hub root is about 160 MB (`image/machines/hub.build`, SOURCE). It removes a dying or hung disk from the running hub's path. It only helps if nothing on the running hub writes to the disk, so driftwm's logs and state must be in RAM too.
8. **ECC memory: yes.** The EPYC 4005 family supports ECC (SOURCE, vendor press release as quoted by Tom's Hardware). A bit error in a user program's memory then becomes a restart of that program rather than a silent fault, and a bad DIMM is reported before it crashes the hub.
9. **Software rendering** (Mesa llvmpipe in place of the GPU) is possible without changing driftwm, but at 4K with 20 windows the measured cost is high and the GPU is still used to drive the screen, so a GPU reset still affects the hub. See section 8 for the numbers and the verdict.
10. **Hardening driftwm reduces crashes; it cannot remove them.** I tested six small patches (a socket handover, a config-parser fix, a panic catch around the client dispatch, a poison-tolerant lock, test hooks, and a one-line Smithay fix for the shm crash, which works: T17). Catching panics kept the compositor alive through 20 consecutive `wl_shm_pool.resize(0)` attacks with the other windows untouched (T13), but a panic caught in the middle of a handler can leave half-updated state, so it is a safety net and not a cure. In a second fuzz round, a debug build with the catch switched on still died in 4 of 6 runs, because the caught panic left a lock poisoned and the next loop pass panicked on it (T13b). After a caught panic the safe action is a clean restart. On the good side, a 2.5-hour soak (20 windows, random operations, 4,043 windows opened) saw no crash, no hang and no steady memory or file growth (TESTED, T16; software rendering and text clients only).

**What I recommend (opinion; details and costs in sections 6 to 11):** use "Plan A" (stand-ins plus `hubd`, with direct restore for Moonlight) as the way windows come back; keep the compositor under a supervisor that restarts it quickly; add the hang probe with a long window; apply the small driftwm patches and send the Smithay fix upstream; keep logs and state in RAM; put the viewers in a lower-priority group than the compositor and `hubd`; buy ECC; and do **not** write a session daemon (Plan B) or our own compositor now.

**What the cloud environment cannot show:** a real GPU and its resets, real heat, real firmware and its updates, a real projector being switched off and on (hotplug), a real disk hanging, and real viewers (Moonlight, SPICE) reconnecting. Those are listed as tests for the December hardware in section 11.

## 2. What was tested, and how (test index)

All tests ran in this cloud environment: 4 shared CPUs (other helpers were running at the same time; the load average was between 5 and 12), no GPU, no `/dev/kvm`, no DRM device, no systemd. driftwm was run **nested** (its `winit` backend inside a virtual X server `Xvfb`) with Mesa's software GL (llvmpipe). Nothing here ran the real display path (`--backend udev`). The driftwm commit is the pinned one, `352333a8fa1b22171492d4b71a54102045c9a19d` (version 0.19.0), downloaded from GitHub today. Build recipe: `docs/driftwm-findings.md` section 0, plus `libudev-dev` unpacked by hand; nothing was installed on the machine. Every test program ran with `ulimit -c 0` and outside the repository.

| # | What | Result (short) |
|---|---|---|
| T1 | driftwm's own test suite, in the same order as its CI: `cargo test --no-run -j2`, then `cargo test -j2 -- --include-ignored --skip soak`, then `cargo test --bin driftwm -- --include-ignored --test-threads=1 soak` | Cold build (all dependencies, debug profile, shared machine) 262 s. Run: **2,033 passed, 0 failed, 0 ignored in 34 s** (327 library unit tests, 1,356 binary unit tests, 350 tests in 11 integration files). The `soak` test: 1 passed in 1.2 s. |
| T2 | Static counts with my own script (`audit.py`) and cross-checked with `cargo clippy` (rustc 1.97.0, lints `unwrap_used`, `expect_used`, `panic`, `unreachable`, `indexing_slicing`, `arithmetic_side_effects`, ...) | Section 5.1. Clippy's 213 `unwrap`/`expect` equals my 191 + 22. |
| T3 | `kill -9` the compositor and watch what happens to its clients (`foot`, `remote-viewer` 11.0, a Qt 6.8.2 test client, with and without `QT_WAYLAND_RECONNECT=1`) | `foot` and `remote-viewer` gone in 0.06 to 0.1 s. Qt without the variable exits (status 255, "The Wayland connection broke"). |
| T4 | Plan A, 20 fake machines (`fakenode`), real `hubd`, `foot` viewers, nested driftwm, `kill -9`, restart after 1.0 s | Section 7.2: five runs A1 to A5. |
| T5 | Plan C: the real `wl-restart` (Ferdi265, built from source) holding the socket, a **patched** driftwm that adopts it, a Qt 6.8.2 client | Section 7.4: four runs C1 to C4. |
| T6 | Hang detection: `SIGSTOP`, busy loop, deadlock, panic (test hooks in the patched build), probes, kill, restart, restore | Section 9. |
| T7 | Does a viewer present the `xdg-activation` token? `foot` 1.16 (Ubuntu 24.04 package), `remote-viewer` 11.0 (GTK 3) | `foot`: yes. `remote-viewer`: **no**. `wlvncc`: no (source only). |
| T8 | Protocol fuzzing with my own tool `fuzz.py` (208 interfaces read from 72 protocol XML files), release build, debug build, ASan build | Section 5.4. |
| T9 | One-request crash reproducer `repro_shm.py` | Release driftwm exits with status 101. |
| T10 | Bad colour in the config, at start, at `--check-config`, and as a hot reload of a running compositor | Panic in all three. |
| T11 | driftwm's stderr is a pipe nobody reads | Frozen in `anon_pipe_write`. |
| T12 | The session file's temporary name is a FIFO nobody reads (a stand-in for a hung disk) | Frozen in `wait_for_partner`. |
| T13 | `catch_unwind` around the Wayland dispatch (patched build): 20 consecutive `wl_shm_pool.resize(0)` | Compositor alive, other windows alive, new windows map, idle CPU normal. |
| T13b | `catch_unwind` with the debug build under fuzzing (section 5.4, second round) | 4 of 6 seeds still died: the caught panic leaves a poisoned lock and the next loop pass panics on it. |
| T14 | driftwm under `ulimit -v` limits | Start-up works at 1.3 GB, segfaults at 900 MB (address space). |
| T15 | llvmpipe cost, nested, 3840x2160, 20 windows | Section 8. |
| T16 | The soak: nested driftwm, 20 windows, random operations, hours | Section 10. |
| T17 | Patch P6 (Smithay one-line fix) against the `wl_shm_pool.resize(0)` crash | `repro_shm.py`, `repro-run.sh`; section 5. |

The scripts, patches and raw results are in `docs/proposals/hub-stability/` (section 13 lists the files).

## 3. Failure-mode analysis, layer by layer

How to read each item: **L/I** is likelihood and impact (opinion, scale: rare / sometimes / often; and "one window", "hub restarts", "hub down"). **Evidence** carries the label. **Choice** is the design choice that removes or contains the problem. **Cost** is what the choice costs. **Test** says how to test it, and whether I did.

What the hub already has (SOURCE, `image/machines/hub.build`, `image/rootfs/etc/s6/sv/*`, `image/kernel/hub.frag`, read 2026-10-05): a read-only squashfs root of about 160 MB in A/B slots; s6 as init with `driftwm`, `waybar`, `hubd`, `dbus`, `seatd`, `udevd`; a boot-loop breaker; a hardware watchdog that is armed at boot and then fed every 5 seconds by a small `watchdog -F` program that does **not** look at any service (`sv/wd/run`), so a service failure never resets the machine; `panic=5` on the kernel command line; kernel 6.12; no out-of-tree modules (`MODULES=` empty). That is a good base: most of the items below build on it.

### 3.1 Hardware

**H1. Memory errors (a flipped bit, or a bad DIMM).**
- L/I (opinion): rare per month, but over years on a 24/7 machine with tens of GB it happens; effect ranges from silent wrong data, to a random program crash, to a kernel panic.
- Evidence: SOURCE: the Linux memory-failure documentation (`docs.kernel.org/mm/hwpoison.html`, summary by the fetch tool): an uncorrected error in a user program's page makes the kernel send that process a SIGBUS (the process dies, the machine lives); an error in kernel memory causes a panic. BELIEVED: large field studies (for example Schroeder, Pinheiro, Weber, "DRAM errors in the wild", 2009) found that a few percent of DIMMs per year show errors; I did not re-read the paper here. BELIEVED: DDR5's built-in "on-die ECC" corrects errors inside the chip only and is not a replacement for system ECC.
- Choice: ECC unbuffered DIMMs (the EPYC 4005 family supports ECC UDIMM on socket AM5: SOURCE, AMD press release of 13 May 2025 and Tom's Hardware, via search results), a board whose firmware has ECC switched on, the kernel's EDAC driver for the AMD memory controller (BELIEVED: `amd64_edac`) reporting corrected errors, and a 24-hour memory test (memtest86+) before the hub is accepted. A compositor killed by SIGBUS is restarted by s6 (section 7), so an uncorrectable error in its memory costs a restart, not a reboot.
- Cost: ECC DIMMs and an ECC-validated board cost more; UNKNOWN which exact board and price.
- Test: December: confirm the board shows ECC active and that EDAC sees the controller (`dmesg | grep -i edac`); run memtest86+; no way to inject an error on a consumer board (UNKNOWN). Not testable here.

**H2. Power supply.**
- L/I (opinion): rare; effect: total outage.
- Evidence: SOURCE `HUB-OS.md`: power is treated as unlimited and never failing; no UPS.
- Choice: owner decided. A good PSU with headroom. Nothing to design in software. (Question 12.2: what the hub should do after a power cut: come up by itself is the sensible firmware setting.)
- Cost: none beyond the PSU.
- Test: pull the plug during a soak in December and see that the hub boots to the panel with no windows (as designed) and no file-system repair.

**H3. Heat and fans.**
- L/I (opinion): sometimes under load; effect: the CPU or GPU slows down (stalls, dropped frames) and at the limit the machine shuts down. The hub does almost no work (it only shows 20 windows), so heat should be low, but a dead fan or dust is a risk.
- Evidence: BELIEVED. Not measured. If software rendering were used (section 8), the CPU would run noticeably hotter (T15 numbers).
- Choice: a conservative cooler, a fan curve that never goes below a minimum speed, temperature sensors (`k10temp`, `amdgpu` hwmon) read by `hubd` and shown as an alert, and a 24 hour run with all 20 windows open in December while logging temperatures.
- Cost: small.
- Test: December only.

**H4. Firmware (UEFI/AGESA) bugs and firmware updates.**
- L/I (opinion): sometimes; effect: random hangs (for example idle states), memory training failures, a bricked board after a bad update.
- Evidence: BELIEVED, from general experience with AM4/AM5 boards. UNKNOWN for the chosen board.
- Choice: pick one firmware version that passes the soak, then pin it: never update the firmware automatically or "because there is a new one"; update only for a known fix, one machine first. Write the final firmware settings into a short checklist that is kept with the machine's config. Use JEDEC memory speeds (no overclock profile) on the ECC machines.
- Cost: a one-time checklist; slower memory than the marketing speed.
- Test: the multi-day soak in section 11 is run on the final firmware; any change of firmware restarts the soak.

**H5. The board's own watchdog and settings.**
- L/I (opinion): the watchdog only matters when the whole machine hangs (kernel or hardware); impact then: a reset instead of a hang until someone notices.
- Evidence: SOURCE: the hub image already arms the hardware watchdog at boot and feeds it from a plain feeder (`sv/wd/run`: `watchdog -F -t 5 -T ${HUBOS_WATCHDOG_TIMEOUT:-180}`), with `CONFIG_WATCHDOG_NOWAYOUT=y` (`image/kernel/hub.frag`). It was tested in QEMU with an emulated watchdog (`docs/image.md`, tests W1 and 6d). UNKNOWN: which watchdog the real board has. BELIEVED: on AMD chipsets the kernel driver is `sp5100_tco` (or `wdat_wdt` through ACPI); I could not confirm that in the kernel documentation here (the fetch tool did not find it).
- Choice: keep exactly this design: the feeder does not look at services, so a crashed or hung compositor never reboots the hub (the owner's rule), while a frozen kernel does. BIOS settings to check: restore-on-AC-power-loss = on, watchdog/TCO enabled, ECC on, no memory overclock.
- Cost: none.
- Test: December: on the real board, stop the feeder with `kill -STOP` and see the board reset after the timeout (and `kill -9` the compositor and see it does not).
- Question 12.3: should a kernel panic reboot (today `panic=5`) or hang until the watchdog fires? Both end in a reboot; the difference is only how fast.

**H6. PCIe, GPU link and NIC hardware errors.**
- L/I (opinion): rare; effect: a GPU or NIC disappears from the bus.
- Evidence: BELIEVED. A GPU that disappears is the hardest case for any compositor (section 8.2 shows driftwm only logs it).
- Choice: use hardware with mature in-tree drivers; keep BIOS PCIe settings at defaults; log AER (kernel PCIe error reports) in the RAM black box (`docs/proposals/black-box-recorder.md`).
- Cost: none.
- Test: December.

### 3.2 Storage

**S1. A dying or hung disk under a read-only root.**
- L/I (opinion): rare for NVMe, sometimes for old SATA; impact: any program that needs a file page that is not yet in RAM stops in uninterruptible sleep while the drive retries (BELIEVED: SATA/SCSI default command timeout is 30 s with several retries, so minutes). A running hub that never reads the disk again does not notice.
- Evidence: TESTED (T12, a stand-in for a hung disk, not a real one): when the session file's temporary name is a FIFO nobody reads, the compositor's `open(2)` blocks and the whole compositor freezes (`/proc/PID/wchan` = `wait_for_partner`, IPC socket unanswered for 4 seconds and more). SOURCE: driftwm writes its session file (`src/session.rs:150-170`, `src/state/session_store.rs:448-455`) and its state file (`src/state/persistence.rs:584`) **synchronously on the event-loop thread**, and reads shader and wallpaper files there too (`src/render/background.rs:425,632,730,799`).
- Choice: (1) copy the whole root to RAM at boot (section 7.5); (2) keep every file the running hub writes in RAM: `XDG_RUNTIME_DIR`, `XDG_STATE_HOME` (this is where `session.json` goes), driftwm's log, hubd's record and logs (`HUB-OS.md` already says driftwm's session state lives in RAM); (3) read the config partition once at start and then never again; (4) nothing on the hub may use a network file system or a hard mount.
- Cost: about 160 MB of RAM and a few seconds of boot time (section 7.5); no persistent logs across reboot unless the black-box recorder saves them at the next boot.
- Test: with the Phase B VM: attach the root disk through QEMU's `blkdebug` or `throttle` so that reads stall after boot, and check the hub keeps running (proposed, not done: QEMU is not installed here and TCG is slow). In December: the same with `echo 1 > /sys/block/sdX/device/delete` on the real disk.

**S2. Corrupted file system after a panic or a power cut.**
- L/I (opinion): rare; impact: the config partition may need a repair; the root is read-only squashfs and cannot be corrupted by a crash.
- Evidence: SOURCE `image/` design (squashfs root, small config partition).
- Choice: keep the writable config partition small, mounted read-only except while the owner changes it; fsck before mounting read-write.
- Cost: none. Test: pull-the-plug test in December.

**S3. A full disk or a full RAM disk because of logs.**
- L/I (opinion): sometimes; the cause is a repeating error message. Example: when the GPU keeps failing, driftwm logs `Render frame error` every frame (`src/backend/udev.rs:1643-1646`), BELIEVED 60 lines per second, about 7 KB/s, 600 MB a day.
- Evidence: SOURCE for the code, BELIEVED for the rate. driftwm has no rate limit on that message.
- Choice: send driftwm's output to a size-capped log that rotates (`s6-log` with `n`/`s` limits in RAM) and run it at `RUST_LOG=warn,driftwm=info`; the existing `sv/driftwm/run` has no log service (only `hubd` has one), so driftwm's output goes to whatever `s6-svscan`'s output is (the console), which can be a slow serial line (BELIEVED).
- Cost: one more tiny service. Test: T11 (below, S4) shows why this matters.

**S4. The log reader stops reading.**
- L/I (opinion): rare; impact: the compositor freezes.
- Evidence: TESTED (T11): with driftwm's stderr connected to a pipe nobody reads and `RUST_LOG=trace`, driftwm stopped in `anon_pipe_write` and never answered IPC again. SOURCE: driftwm logs with `tracing_subscriber::fmt()` to stderr, written synchronously on the thread that logs (`src/main.rs:100-107`; its own comment at `src/backend/udev.rs:1423` says "the subscriber writes synchronously on this thread").
- Choice: log through `s6-log` (which always reads, and whose own disk is a RAM disk); or patch in a non-blocking log writer (`tracing-appender`), a few lines.
- Cost: small. Test: T11 is the regression test.

### 3.3 The kernel

**K1. A new kernel breaks something (regression).**
- L/I (opinion): sometimes for new kernels; impact: from a hang to a dead GPU.
- Evidence: SOURCE (Phoronix, December 2025 and January 2026, via search results; I could not open the review page, which returned an access error): "AMD RDNA3/RDNA4 Go Down Hard On Linux 6.19" reported hard hangs (the machine could not even be reached remotely) with Linux 6.18 and 6.19; the cause was an AMDGPU display-manager change for idle optimisation, found and reverted in 6.19-rc7 (Phoronix, "AMDGPU Driver Reverts Code For A Number Of Regressions On Linux 6.19", search-result summary).
- Choice: pin one long-term-support kernel (the image already uses 6.12) and move to a newer one only after it has been out for weeks and has passed the multi-day soak on the real hub; subscribe to the stable-release notes for `drm/amdgpu`; keep the previous kernel in the other A/B slot.
- Cost: no new hardware features before the soak; fine for hardware chosen in December.
- Test: the boot test and soak of every new kernel (the repo already runs boot tests per image).

**K2. A minimal kernel with only the drivers the hub needs, and no out-of-tree modules.**
- L/I (opinion): every extra driver is extra code that can hang the machine; impact: hub down.
- Evidence: SOURCE `image/machines/hub.build` (`MODULES=` empty) and `HUB-OS.md` ("out-of-tree kernel modules only when a machine's config declares them"): the hub already has none. BELIEVED: a smaller driver set means fewer bugs.
- Choice: keep the hub kernel configuration small; do not build Bluetooth, Wi-Fi, USB mass-storage-gadget and other unused drivers; add a boot-test check that `/proc/sys/kernel/tainted` is 0 after boot and after the soak (a tainted kernel means a forced module, a machine check, a warning, or an out-of-tree module).
- Cost: configuration upkeep. Test: a one-line check in the boot test and at the end of every soak.

**K3. Panic, soft lockup and hung-task behaviour.**
- L/I (opinion): rare; impact: hub down until it restarts.
- Evidence: SOURCE `image/kernel/hub.frag`: `CONFIG_PANIC_TIMEOUT=-1` and `panic=5` in the command line; `docs/proposals/black-box-recorder.md` TESTED that kernel messages survive a warm reset in RAM (`ramoops`) in QEMU.
- Choice: keep reboot-on-panic (5 s) so a panic is a short outage; add the black box so the cause is known afterwards; do **not** turn soft lockups or hung tasks into panics (that would make a stuck viewer's file access reboot the hub).
- Cost: reserved RAM for ramoops. Test: black-box doc; real hardware in December (whether RAM survives a real warm reset is UNKNOWN).

**K4. The out-of-memory killer.** See M2 below.

### 3.4 The GPU and its driver

The facts about what driftwm does when the GPU is lost are in section 8.2. Here is the failure-mode view.

**G1. A GPU hang and reset.**
- L/I (opinion): sometimes on AMD (the 2025/2026 reports show it is a live problem); impact: the compositor dies or goes blind, and every window with it.
- Evidence: SOURCE (search results, summaries by the fetch tool): cosmic-comp (the other Smithay-based compositor) died on every amdgpu reset because an `.unwrap()` on getting the renderer panicked; the fix (pull request 2487, merged 28 July 2026, title "don't panic when renderer creation fails after a GPU reset") replaced three `unwrap()` calls by error returns, and a tester reports "the session is still lost each time" afterwards, now with no panic message; related issues 2149 ("page fault ... GPU reset and full session restart") and 2720. The same shape in wlroots: the `wlr_compositor_set_renderer` handling for resets, and Sway's pull request 8578 ("fix crash on exit ... GPU reset: destroying the renderer in its own lost-event handler asserts") merged 13 April 2025. Smithay's own pull request 1739 says "most compositors don't really handle their primary gpu disappearing". The kernel's default job timeout is 2000 ms for all queues per the kernel documentation page I fetched (summary by the fetch tool; the older text said 10 000 ms for graphics, so treat the number as "2 to 10 seconds"); how long a full reset takes is UNKNOWN (reports say seconds).
- What a reset does to a compositor (BELIEVED): every GL context on the GPU is lost; a program that did not ask for a "robust" context gets undefined behaviour, which in practice is a crash inside the driver, a stuck GL call, or a black screen.
- Choice: (1) no GPU decode and no GPU rendering in the viewers if it can be avoided, because a bad GPU job from any program can reset the GPU for everybody (SOURCE: the SteamVR-for-Linux issue #940, search result: one program's GPU page faults cause resets that need a hard reset); (2) treat "compositor died" and "compositor is blind" as the same event and restart it; for "blind" use a render probe (section 9.3); (3) software rendering takes GL out of the compositor but not the display engine (section 8.1); (4) pin kernel and Mesa; (5) test a reset on the real GPU.
- Cost: see section 8.
- Test: December, on the real GPU: trigger a reset on purpose and watch (BELIEVED: writing to `/sys/kernel/debug/dri/0/amdgpu_gpu_recover` does that; I did not verify the name). Not testable here.

**G2. A shader fails to compile.**
- L/I (opinion): rare; impact: compositor panics at start or at config reload.
- Evidence: SOURCE `src/render/background.rs:748` and `:832`: `.expect("Default shader must compile")` on the built-in background shader.
- Choice: patch to fall back to a flat colour (needs a few lines of plumbing, proposed in section 5.6) or remove the shader background (section 8.3: the owner does not need it).
- Test: needs a driver that rejects the shader; UNKNOWN.

**G3. A buffer from a client cannot be imported.**
- L/I (opinion): sometimes (a client sends a buffer the GPU cannot use); impact: that window shows nothing.
- Evidence: SOURCE `src/handlers/mod.rs:213-228`: `dmabuf_imported` calls `notifier.failed()`, which tells the client the buffer was refused; no panic. For shared-memory buffers: SOURCE Smithay installs a SIGBUS guard for client-truncated files (`src/wayland/shm/pool.rs:42,104`). The import at draw time: BELIEVED handled (logged) by Smithay; not read in full.
- Choice: none needed. Test: the fuzzer sends random dma-buf parameters; no crash was seen (T8).

### 3.5 The compositor (driftwm)

Section 5 is the audit. In short: single maintainer (94% of 1,244 commits by one person, first commit 2026-02-23, SOURCE `docs/driftwm-findings.md` section 1), pre-1.0, "primarily built with AI" (its README), 191 `unwrap()` and 22 `expect()` in non-test code (T2), no panic handling at run time, and five concrete problems found in this session (T9, T10, T11, T12, section 5.3).

**C1. A client sends something unexpected.**
- L/I (opinion): the clients are our own viewers and Waybar, so rare; impact: all windows (compositor dies).
- Evidence: TESTED (T9) the one-request crash; TESTED (T8) the fuzzer, on the release build, hit that same crash in 1 of 6 runs of 2 minutes (seed 20) and the other five runs survived (about 290 connections and 36,000 to 47,000 requests each); SOURCE: debug builds also panic on negative sizes (Smithay `debug_assert`), release builds do not.
- Choice: patch the Smithay bug (one `return`), catch panics in the client dispatch (T13), keep sockets private to the hub user, and keep the viewer set small and known. Fuzz the new compositor build in the soak before each upgrade.
- Cost: a Smithay fork or patch (section 5.9).

**C2. A mistake in the config file.**
- L/I (opinion): sometimes (owner edits); impact: compositor exits on the next save (T10).
- Choice: turn off hot reload on the hub (the config is declarative and the compositor is restarted to change it, which costs 3 seconds with Plan A), validate the file with `--check-config` **after** fixing the parser bug, and fix the parser bug (6 lines, tested: section 5.6).

**C3. A deadlock or long block in the event loop.**
- L/I (opinion): rare; impact: the compositor looks alive to the supervisor but nothing moves.
- Evidence: SOURCE `dev/docs/caveats.md` (known: the v0.14.0 freeze when a client destroyed its toplevel while fullscreen; the pointer-lock freeze with a region; "nothing else is enforced statically" for the user-data lock). TESTED (T6) that a deadlock is detected and recovered by the probe (a mutex locked twice, `wchan` = `futex_do_wait`).
- Choice: the probe plus restart (section 9), and removing features that carry the known hazards (pointer constraints) if the owner agrees (section 8.4).

**C4. Memory growth.**
- L/I (opinion): sometimes in long-running compositors; impact: slow, then OOM.
- Evidence: the soak (section 10). driftwm has a leak counter endpoint, `driftwm msg debug-counters` (SOURCE `src/ipc/protocol.rs`), and its notes list a known counter blind spot (`dev/docs/todo.md:80`).
- Choice: a memory limit on the compositor's group so it is restarted rather than the machine being starved; a scheduled restart is not needed if the soak shows no growth.

**C5. The compositor is slow because a viewer hogs the CPU.** See section 7.6.

### 3.6 The viewers

**V1. A viewer crashes.** One window disappears; `hubd` notices (`syncWindows`) and the panel shows it again as available. L/I (opinion): sometimes; one window. SOURCE `internal/hub/watch.go`.

**V2. A viewer hangs.** The compositor is not affected: client sockets are non-blocking and the output queue per client is bounded (SOURCE `wayland-backend 0.3.15 rs/server_impl`: `max_buffer_size` per client; a client whose queue is full is disconnected). TESTED (soak, section 10) with random `SIGSTOP`/`SIGCONT` of viewers.

**V3. A viewer eats CPU or memory.** Contained by group limits (section 7.6). The compositor's frame time under 8 CPU hogs: section 8 (T15).

**V4. Viewers die with the compositor.** TESTED (T3). Only a viewer written for it survives (Qt 6.6 or newer with the reconnect switch, T5). Moonlight's streaming window is an SDL window (SOURCE, search results about `moonlight-qt`), SDL has no reconnect, and GTK has none (SOURCE: David Edmundson's blog, via the fetch tool).

**V5. A viewer sends the activation token or not.** TESTED (T7), section 9.5.

**V6. A node or the network disappears.** Out of scope by the owner's decision (the network is assumed perfect); `HUB-OS.md` already says to leave the window alone and change only the alert. Not tested.

### 3.7 hubd

**D1. hubd crashes.** Go programs die on an unrecovered panic in any goroutine. s6 restarts it; the windows stay (they belong to the compositor); `hubd` re-adopts them by name (SOURCE `internal/hub/watch.go` `adopt`; TESTED earlier in `docs/hubd-slice2.md` section 16.2 with `kill -9`). L/I (opinion): rare; the panel is blank for a second.

**D2. hubd hangs.** A deadlock in `hubd` freezes the panel but nothing else. The image's confirm step waits for `hubd` to answer five times (SOURCE `docs/image.md`); in steady state the same ping can be the hang probe for `hubd`. Not tested here.

**D3. hubd and a compositor restart.** TESTED (T4): today's `hubd` misreads the restored stand-ins as open windows and reconnects with a delay of up to 4 seconds (section 7.2). This is the main thing to change for Plan A.

### 3.8 Memory

**M1. Leaks.** See C4 and the soak.

**M2. No swap, so the OOM killer decides.**
- L/I (opinion): rare; impact: the wrong process killed, or a long stall.
- Evidence: BELIEVED: with no swap a memory shortage ends quickly in an OOM kill instead of slow thrashing, but file-backed pages of running programs can still thrash before that. TESTED (T14): under an address-space limit of 900 MB, driftwm ended with SIGSEGV at start-up; at 1.3 GB and above it started. A real out-of-memory in Rust code aborts the process (SOURCE: Rust's default allocation-failure handler aborts); Mesa and other C libraries do not handle it (T14).
- Choice: memory cgroups: a ceiling for the viewer group, a floor (`memory.min`/`memory.low`) for the compositor and `hubd`; `oom_score_adj` high for viewers (kill them first) and very low for `hubd` and the compositor; a small `PSI`-based watcher or `earlyoom` (BELIEVED: both work without systemd) that kills the biggest viewer before the kernel starts thrashing. cgroups v1 `cpu` is writable in this environment (checked), so the same controls can be used on the hub kernel if it enables cgroups (check the config).
- Cost: configuration and a small service. Test: December, with a viewer that leaks on purpose.

### 3.9 The network

**N1. Network file systems and hard mounts.** None on the hub (rule above). Reading a file from a dead NFS server blocks forever. SOURCE `HUB-OS.md`: the hub stores nothing.

**N2. hubd's probes.** Bounded: 2 s per machine and 5 s total (SOURCE `docs/hubd-slice2.md`, guesses not measured). They run in `hubd`, not in the compositor.

**N3. Viewer behaviour when a node vanishes.** Not tested (out of scope by the owner).

### 3.10 Time

**TM1. Clock jumps (NTP step, dead RTC battery, resume).**
- L/I (opinion): rare; impact: timeouts that fire early or never.
- Evidence: SOURCE: driftwm uses the monotonic clock (`std::time::Instant`, 42 uses in `src`; `SystemTime` only for desktop-entry directory timestamps and the name of a quarantined session file: `src/desktop_entry.rs:21`, `src/session.rs:190-196`). `Instant` never goes backwards. Go's `time.Now()` carries a monotonic reading for durations (BELIEVED, standard Go behaviour). Not tested: changing the clock would disturb the other jobs on this shared machine.
- Choice: use a slewing time client (no big steps after boot); at boot, set the clock from the newest file time on the config partition if the RTC is clearly wrong (BELIEVED: a dead RTC battery starts at the year 1970 or 2000 and the first sync is a huge jump).
- Test: December: step the clock by hours in both directions during a soak.

### 3.11 Operator actions

**O1. A config typing mistake** kills a running compositor: T10 (section 3.5 C2).

**O2. Killing the wrong program.** Including the rule used in this very session: never use `pkill -f`. Provide a single "restart the desktop" action in the panel (restarts the compositor service only) so nobody needs a shell.

**O3. Unplugging or switching off the projector.** The projector is the only output. A switched-off projector looks like an unplugged display to the kernel. SOURCE: driftwm handles connector changes (`src/backend/udev.rs` `UdevEvent::Changed`), but what it does with **zero** outputs and when the output comes back is UNKNOWN and only testable with a real or virtual monitor hotplug. It is on the December test list (section 11).

**O4. Editing the inventory or `viewers.toml` wrongly.** `hubd` refuses to start with a bad file (SOURCE `docs/hubd-slice2.md`: "a bad file stops `hubd serve` before anything starts"), so a typo leaves the hub without a panel. Keep a last-known-good copy and fall back to it. (Proposal, BELIEVED.)

**O5. A restart loop.** If the compositor crashes at every start (for example a bad config), s6 restarts it forever and the screen flickers. Add a limit (after 5 crashes in 60 s, stop restarting, show a fixed message on the panel from `hubd`, keep the bare recovery terminal reachable) and **never** reboot (the owner's rule). (Proposal, BELIEVED.)

## 4. Evaluation of the five ideas (summary, with a recommendation each)

| Idea | Recommendation (opinion) | Where |
|---|---|---|
| (a) Windows come back after a compositor crash (Plan A stand-ins + `hubd`; Plan B session daemon; Plan C Qt reconnect) | **Plan A now**, with four small `hubd` changes. Do not build Plan B now. Plan C only if we later write a Qt 6.6+ viewer. | section 7 |
| (b) Root copied to RAM at boot | **Yes.** About 160 MB of RAM. Only useful together with "nothing on the running hub writes to disk". | section 7.5 |
| (c) ECC memory on the hub | **Yes.** | section 7.5 |
| (d) Reserved CPU and priority for the compositor and `hubd` | **Yes**, with cgroup weights and a CPU set, not real-time priority. | section 7.6 |
| (e) Harden driftwm | **Yes, a small patch set**, plus send the Smithay fix upstream. It reduces crashes and does not remove them. Our own compositor is not justified now. | sections 5 and 6 |
| (f) Anything else | Section 7.7: keep logs out of the compositor's way, bound the start-up shell-outs, give the supervisor a crash-loop limit, test GPU reset on purpose. | |

## 5. driftwm at the pinned commit: audit and hardening

Everything in this section is about commit `352333a8fa1b22171492d4b71a54102045c9a19d` (0.19.0), Smithay rev `4cf0b62028039661477d482ec4758b687d8f4392`, read today.

### 5.1 Counts (T2)

My script `audit.py` removes `#[cfg(test)]` items, the `src/tests/` folder and files called `tests.rs`/`mock.rs`, then counts. Clippy (non-test code, rustc 1.97.0) counted independently.

| What | My count | Manager's count (2026-10-05) | Clippy |
|---|---|---|---|
| Rust files in `src` without tests | 121 | 122 | n/a |
| Lines of code (no blanks, no comment lines) | 47,559 | n/a | n/a |
| Lines including comments | 65,120 | about 55,800 | n/a |
| `.unwrap()` | **191** | 186 | `unwrap_used` + `expect_used` together: 213 |
| `.expect(` | **22** | 21 | (in the 213) |
| `unreachable!(` | **19** | 19 | 19 |
| `panic!(` | **2** (`src/state/mod.rs:556,557`) | none | 2 |
| `unsafe` (keyword) | **25** | 16 | n/a |
| Indexing `a[i]` that may panic | n/a | n/a | **104** (+ 23 slicing + 11 string slicing) |
| Arithmetic that may overflow | n/a | n/a | 717 (not reviewed one by one) |
| Casts that may truncate | n/a | n/a | 329 (+ 95 sign-loss) |

The differences from the manager's numbers are small. My `panic!` count (2) differs from "no `panic!(` calls": the two are in `output_state()`, see 5.3. My `unsafe` count counts the keyword, including 7 `env::set_var` calls that Rust 2024 requires to be `unsafe`; I do not know how the manager's 16 was counted. The line totals differ because I cannot tell how the manager separated test modules. The release profile has no `panic = "abort"` (SOURCE `Cargo.toml`: `[profile.release] lto = "thin"`, `debug = "line-tables-only"`), so panics unwind (TESTED: exit status 101). The only `catch_unwind` in the repository is inside a test (SOURCE, `grep`); there is no panic hook. In release builds integer overflow wraps silently (no panic) and `debug_assert!` is off; in debug builds both panic. That difference matters: see 5.4.

### 5.2 Every unwrap, expect and unreachable, classified

Reading rule: "proved safe" means a check or an invariant right before it, which I read. "Reachable" means something outside the compositor (a client, a device, a file the owner edits) can make it fail. The total is 191 + 22 + 19.

**`.unwrap()` (191).**

| Class | Count | Verdict |
|---|---|---|
| A. `seat.get_pointer()`, `get_keyboard()` or `get_touch()`, then `.unwrap()` (in `input/*`, `state/*`, `handlers/*`, `render/*`, `protocols/*`) | 102 | **Proved safe.** `src/state/init.rs:225,256,265,266` adds keyboard, pointer and touch to the seat at start-up, and nothing in `src` ever removes them (a search for `remove_pointer`, `remove_keyboard`, `remove_touch` finds nothing). |
| B. `Mutex::lock().unwrap()` (`render/tile_worker.rs`, `text.rs`, `input/actions.rs`, `state/keyboard_focus.rs`, `handlers/compositor.rs`, `protocols/image_copy_capture.rs`, `render/closing.rs`, `render/capture.rs`, ...) | 40 | **Safe unless an earlier panic poisoned the lock.** This is the "second panic" amplifier: after a caught panic, any of these can panic again (section 5.8). |
| C. A check right before it in the same function (`canvas.rs:577,578` after the `len() < 2` return; `config/parse.rs:51,73` after `is_empty()` return; `grabs/move_grab.rs:380`; `handlers/compositor.rs:281`; `backend/udev.rs:891` right after `insert`; `render/error_bar.rs:70`; `render/capture.rs:313`; `input/actions.rs:319,862`; `render/cursor.rs:138`; `state/viewport_animation.rs:90`; `render/tile_chunks.rs:790` and `render/shader_chunks.rs:476` with the comment "resolve loop above only inserts keys whose entry exists"; `protocols/image_copy_capture.rs:132,216` after `is_none()` continue) | 17 | **Proved safe** by the check I read. |
| D. `data.backend.take().unwrap()` or `as_mut().unwrap()` of the backend or renderer (`backend/udev.rs:593,692,1502`, `backend/winit.rs:51,58,214`) | 6 | **Safe in normal flow** (the backend is put back at the end of the same function). It would panic on a re-entrant call or after a caught panic that left it `None` (section 5.8). |
| E. `output.current_mode().unwrap()` (`backend/udev.rs:1872`, `render/capture.rs:103,465`, `protocols/screencopy.rs:175,204`) | 5 | Safe while every output has a mode (set when created). Three of the five are in screen-capture code the hub does not need. |
| F. `get::<T>().unwrap()` on user data that the same code stored (`handlers/layer_shell.rs:193` and `handlers/mod.rs:1371` right after `insert_if_missing_threadsafe`; `handlers/mod.rs:1409`; `handlers/compositor.rs:465,937`; `ipc/mod.rs:771`) | 6 | Proved safe for the first two; the other four rely on the window having been set up first (BELIEVED, not traced to the end). |
| G. Guarded by a feature test in the caller (`render/blur.rs:1048,1049` guarded by `blur_down_shader.is_some()` at `render/mod.rs:835` and `render/layers.rs:294`; `render/blur.rs:1300`; `render/background.rs:358,467`; `render/layers.rs:361` and `render/mod.rs:1563` inside `if client_blur`) | 7 | Safe by a condition in the caller. All in blur and shader-background code (unneeded effects). |
| H. Protocol code the hub does not need: `protocols/screencopy.rs:268,426` (`queues.get_mut(manager).unwrap()`), `protocols/output_power.rs:55,174`, `protocols/virtual_keyboard.rs:574`, `protocols/foreign_toplevel.rs:330` | 6 | **BELIEVED safe, not proved.** `screencopy.rs:268,426` depends on a clean-up rule (`cleanup_queues`) that I read but did not prove complete; `foreign_toplevel.rs:330` is guarded by `manager.client()` being `Some`. |
| S. `pointer.grab_start_data().unwrap()` and the touch twin in `start_dnd` (`handlers/mod.rs:161,167`) | 2 | **Proved safe by reading Smithay**: `data_device/device.rs:81,114` calls `has_grab(serial)` before the compositor's handler runs. |

The eight classes add to 191 (102 + 40 + 17 + 6 + 5 + 6 + 7 + 6 + 2 = 191; the exact per-site list is produced by `cat2.py` from `audit.py`'s output, both in the scripts folder). **None of the 191 can be reached by a client request in anything I read.** That fits the author's rule in `caveats.md` ("prefer `if let` over `unwrap()` for anything from a client"). The crashes I found are not driftwm `unwrap()`s: they are in Smithay (F1, F2) and in string slicing of the config (F3), found by running, not by reading.

**`.expect(` (22).** Start-up only (`signals.rs:44,58`; `state/init.rs:84,89,257`; `backend/udev.rs:601`): 7. Mutex poison (`text.rs:94`): 1. Touch grabs (`grabs/move_grab.rs:660,683,753`, `grabs/resize_grab.rs:743,802`): 5; they need a touchscreen. Window id invariants (`state/persistence.rs:163,210,320,405`, `ipc/mod.rs:517`): 5; the stage gives every window an id when it is mapped. `handlers/compositor.rs:42` ("client has no ClientState"): every `insert_client` in `src` passes a `ClientState`, so safe (and my patched socket path does the same). `state/mod.rs:551` (`OutputState` missing): driftwm creates every output itself. `render/blur.rs:772`: 1 (effects). **`render/background.rs:748,832` ("Default shader must compile") are the one GPU-dependent pair** (section 3.4 G2).

**`unreachable!` (19).** 14 are the `_ => unreachable!()` arm of a Wayland request enum in `screencopy`, `output_power`, `output_management`, `image_copy_capture`, `gamma_control`: generated request enums are `#[non_exhaustive]`, so the arm exists only for requests newer than the generated code, and the library rejects an unknown opcode before dispatch (BELIEVED, from the library's design: the server looks the message up in the interface's request table; I did not read that code in full). 3 are in `grabs/touch_gesture_grab.rs` (a decision table; touch only). 1 in `backend/winit.rs:215` ("winit timer with non-winit backend"): safe. 1 in `ipc/mod.rs:199` (`Subscribe` is handled before `dispatch`): proved by reading `serve_connection`.

**`panic!` (2)**: `src/state/mod.rs:556` (output state locked twice) and `:557` (poisoned), in `output_state()`, which is called about 300 times. The caveats file says "debug builds panic on a re-entrant lock", but the code uses `try_lock` in **all** builds, so the release build converts the documented deadlock into a panic. That is better than a freeze, and it is still a crash. The poisoned arm makes every later call panic after one caught panic (5.6, patch P5).

### 5.3 The crashes and freezes I actually found

| # | What | Where | Label | Fix |
|---|---|---|---|---|
| F1 | `wl_shm_pool.resize(0)` makes the compositor panic (release build too). After `size <= 0` the code posts a protocol error but does not return, then runs `NonZeroUsize::try_from(size as usize).unwrap()`. | Smithay `src/wayland/shm/handlers.rs:190` at the pinned rev; **identical in Smithay master** (I downloaded the file today and read lines 185-195). | TESTED (T9, T8 seed 20) + SOURCE | One `return;` in Smithay. We must carry it (patch or fork) until upstream fixes it. |
| F2 | Negative size in `wl_surface.damage` and `xdg_toplevel.set_min_size/set_max_size` panics **debug** builds. | Smithay `src/utils/geometry.rs:779` (`debug_assert!`), called from `compositor/handlers.rs:221` and the xdg handler. | TESTED (T8: debug build died in 7 of 12 seeds) | Release builds do not panic here (BELIEVED: they accept a negative rectangle; the effect of that downstream is UNKNOWN). Matters for any debug or CI fuzzing: skip these requests, or fix in Smithay. |
| F3 | A config colour with a multi-byte character, for example `"#aéaaa"`, panics. It happens at start, in `driftwm --check-config` (so the validation tool crashes too), and **in the running compositor** because it reloads the config by itself (inotify). `base_swipe_str` slices a string with the length of its lower-cased copy (can also panic). | `src/config/parse_helpers.rs:142-151`, `src/config/mod.rs:72` | TESTED (T10) | 3 lines (`if !hex.is_ascii() { return None; }`) and 4 lines (`combo.get(..n)`); tested with the patched build: the same file now gives "invalid bg_color ... using default; Config OK, 1 warning". |
| F4 | The log reader stops reading and the compositor freezes. | `tracing` writes to stderr on the main thread (`src/main.rs:100-107`). | TESTED (T11) | Log through `s6-log`, or a non-blocking writer. |
| F5 | The session or state file blocks and the compositor freezes. | `src/session.rs:150-170` (`File::create`, `write_all`, `rename`, optional `sync_all`), `src/state/persistence.rs:584`. | TESTED (T12, proxy) | Keep `XDG_STATE_HOME` and `XDG_RUNTIME_DIR` on RAM; a real fix is to write them from a worker thread. |
| F6 | Start-up blocks forever if `dbus-update-activation-environment` hangs: the child is spawned with `child.wait()` on the main thread before the event loop starts. Same pattern for the `xwayland-satellite` start (`xwayland.rs:130,179`). | `src/main.rs:250-272` | SOURCE (not triggered; in this environment the command fails quickly: "unable to connect to D-Bus") | Drop the shell-out (the hub has no systemd and sets up its own D-Bus), or time-box it. The hub image already starts a private `dbus-daemon`. |
| F7 | `std::env::set_var` is called at `src/main.rs:222-228` while the font-scanning thread (`src/text.rs:63`, started from `state/init.rs:90`) and the desktop-entry thread (`state/suspended.rs:1633`, started at `main.rs:176`) are already running. `setenv` is not thread-safe in C. | `src/main.rs:105,114,222-228` | SOURCE; effect BELIEVED tiny but real (a crash inside `getenv`) | Set the variables before any thread starts (they are fixed strings), or pass them with `Command::env` as the code already does for children. |
| F8 | Stale `IpcServer` socket removal: when an old instance is stopped with SIGTERM **after** a new one started with the same socket name, the old one's `Drop` deletes the new one's socket file (`src/ipc/mod.rs:67-71`). I hit this in my own test harness. | `src/ipc/mod.rs:67-71,76-80` | TESTED by accident (the second llvmpipe run could not find the socket) | The supervisor must wait for the old process to exit before starting the next (s6 does). A crash (`kill -9`) does not run `Drop`, so Plan A is not affected. |
| F9 | A slow `hubd` subscriber: a reply write may block the loop for up to 1 s (`WRITE_TIMEOUT`, `src/ipc/mod.rs:29`). | `src/ipc/mod.rs` | SOURCE, BELIEVED harmless for local sockets and small replies | None needed. |
| F10 | `Display::dispatch_clients(...).ok()` throws errors away. | `src/main.rs:212` | SOURCE | Log them. |

### 5.4 Fuzzing and sanitizers: what I ran and what I found (T8)

**My fuzzer.** `fuzz.py`: reads 72 protocol XML files (the Wayland core, `wayland-protocols`, the wlroots protocols and the virtual-keyboard/input-method protocols, 208 interfaces), connects, binds every global, creates a surface and a toplevel, then sends well-framed requests with random arguments (edge-case integers, long and odd strings, random object ids of the right type, memfd file descriptors of odd sizes). When the compositor sends a protocol error the client is killed and a new one starts. It is dumb (no protocol state model), so about 90% of connections are killed within 150 requests, but it ran 27,000 to 68,000 requests per 1 to 2 minutes.

| Build | Runs | Result |
|---|---|---|
| Release (pristine) | 7 runs, 120 s each, 18,000 to 47,000 requests per run | **1 crash** (seed 20: F1). The other 6 runs finished with the compositor alive; its memory after the first run was 139.7 MB (`VmHWM`), 21 file descriptors. |
| Debug (pristine; debug assertions and overflow checks on) | 12 runs of 60 s | **7 crashes**, all Smithay `Size::new` negative-size `debug_assert!` (F2: damage and min/max size). With those requests skipped, `5 of the 12 seeds still died on the same assertion (seeds 4, 5, 7, 13, 14; the backtraces I read, seeds 4 and 5, came through `xdg_toplevel.set_min_size` / `set_max_size`); the other 7 survived 60 s with 21,600 to 27,100 requests each.` |
| AddressSanitizer build (rustc `-Zsanitizer=address`, debug, requests above skipped) | 6 runs (seeds 40 to 45), 100 s each, about 67,000 requests per run (about 400,000 in all), after skipping the four request kinds already known to crash (damage, damage_buffer, min/max size, shm pool resize) | No sanitizer report. The compositor was alive at the end of every run. The ASan build uses 333 MB resident (the release build 140 MB). |

**Second round of fuzzing (TESTED, T8), run after the first findings.**

- **Release build, only the known shm-pool-resize crash skipped:** 6 seeds (30 to 35), 120 s each, 294 to 295 connections and 49,600 to 53,400 requests per seed (about 305,000 in all). The compositor was alive at the end of every seed and passed the health check afterwards (IPC answered, a new `foot` window mapped, a `Move` was applied). This is the build as it would ship, so after the one known crash (and P6 removes that one) I found no further crash in this build in 12 minutes of fuzzing. That is a small sample and says nothing about request kinds my fuzzer does not send (BELIEVED to be most of the surface that matters, UNKNOWN in full).
- **Debug build with the panic catch (P3) switched on, nothing skipped:** 6 seeds (50 to 55), 60 s each. **4 of the 6 died anyway** (seeds 50, 51, 53, 54). The pattern was the same each time: the first panic (the debug-only `Size` assertion in `set_min_size`/`set_max_size`, `geometry.rs:779`) was caught, but it happened while Smithay held a surface's data `Mutex`, so the mutex was left poisoned. The very next pass of the main loop (`refresh_window_outputs`, which is outside the catch) took the same lock, panicked on `PoisonError` (`tree.rs:167`), and the compositor exited. The other 2 seeds survived and passed the health check. In a release build that particular assertion does not exist, so this exact path would not fire there; the lesson is general (BELIEVED, from this evidence plus the 40 `lock().unwrap()` sites in 5.2): **catching a panic does not make the state safe, and a caught panic must be followed by a clean restart, not by carrying on.** This is why 5.8 says "safety net, not a cure" and why I propose the counter (exit after the first or second caught panic). It also means my earlier statement that the catch kept the compositor alive through 20 `resize(0)` calls (T13) is true for that case only; it must not be read as "panics are survivable".

**What else exists, and I did not run:** the Wayland Conformance Suite `wlcs` (Smithay ships a `wlcs_anvil` folder: SOURCE), `cargo-fuzz`/AFL targets for the config parser (the F3 bug is the kind they find in seconds; BELIEVED), a malloc-failure injector with `LD_PRELOAD` (not run: Rust aborts on allocation failure by design, so it would only show the abort), a fault-injecting GPU in the nested mode (an `LD_PRELOAD` shim making `eglSwapBuffers` or `eglMakeCurrent` fail; BELIEVED not effective, because Smithay fetches its EGL functions through the loader's own table, so an interposed symbol is not called; I did not try it, and the nested `winit` path differs from the real DRM path anyway, so a result there would say little about a real GPU loss), kernel fault injection (`failslab`, `fail_page_alloc`) on a test kernel, and the GPU-reset trigger on real hardware (section 3.4 G1). `driftwm` itself has a proptest harness for the stage and 2,033 tests (T1); none of them sends malformed Wayland requests.

### 5.5 `unsafe` blocks (25 occurrences, 12 places)

| Where | What | Verdict |
|---|---|---|
| `main.rs:105,114,222-224,227,228` (7) | `std::env::set_var` (unsafe in Rust 2024) | F7: race with threads started earlier. Everything else about them is fine. |
| `main.rs:212` | `display.get_mut()` on the Wayland `Display` stored inside the calloop source | Sound as commented (the display lives as long as the source). |
| `main.rs:399` | `libc::signal(SIGCHLD, SIG_IGN)` | Sound; child exit statuses are not available to driftwm afterwards (fine). |
| `signals.rs:29,63,64,71` (4) | `sigaddset`, `sigemptyset`, `pthread_sigmask` | Sound (checked return values). |
| `state/mod.rs:330`, `xwayland.rs:70` | `pre_exec` closures that reset signals before `exec` | Sound: only async-signal-safe calls. |
| `backend/udev.rs:503,552` | `EGLDisplay::new`, `GlesRenderer::new` (unsafe constructors) | Sound by Smithay's contract; failure is handled (the loop tries the next GPU). |
| `render/blur.rs:624,665,1824,1841` | raw GL calls | Effect code. |
| `render/capture.rs:217,572,794` | `copy_nonoverlapping` into a client's shm buffer with length `min(shm_len, bytes.len())` | Sound as written; capture code the hub does not need. |
| `protocols/virtual_keyboard.rs:321` | `xkb.keymap()` | Sound as commented. |

No `unsafe` block I read can be reached with a client-controlled length or index. A real memory-safety bug would have to be in Smithay, wayland-rs, Mesa or another C library. The ASan run (above) is the only check I made on that.

### 5.6 What `caveats.md` warns about, and what stops it (compiler, lint, or discipline)

| Hazard in `dev/docs/caveats.md` | What prevents it | Label |
|---|---|---|
| Touching `Space` windows directly | A clippy `disallowed-methods` list in `clippy.toml` (CI runs `clippy -D warnings`) and a debug-build check `verify_stage_invariants` every frame. A **lint**, not the compiler, and the check does not exist in release. | SOURCE |
| Blocking the event loop (DNS, slow file read, stuck child) | **Discipline only.** I found four violations: F4, F5, F6 and the shader/wallpaper file reads in `render/background.rs`. | SOURCE + TESTED |
| Locking `output_state` inside an `if let`/`match` scrutinee | Clippy lint `significant_drop_in_scrutinee` (warn, CI turns it into an error) **plus** a run-time `try_lock` in every build that turns a hang into a panic. | SOURCE |
| Re-entering a surface's user-data lock (`with_states` inside `with_states`) | **Only** for pointer constraints (`clippy.toml` bans `with_pointer_constraint`). "Nothing else is enforced statically", as the file says. A violation hangs with no panic and no log. | SOURCE; TESTED that the hang probe handles a lookalike (T6 deadlock) |
| "Client misbehavior must not crash the compositor" | Convention, plus unit tests of the stage and protocols. The Smithay F1/F2 bugs show the convention does not cover the library under it. | SOURCE + TESTED |
| Memory safety in general | Rust itself, except the 25 `unsafe` places and C libraries. | |

### 5.7 Detecting a hung (not crashed) compositor from outside, and what driftwm does when the GPU is lost

See section 9 for the probe and the tests. About the GPU: see section 8.2.

### 5.8 Panic strategy: abort, unwind and exit, catch per dispatch, per output, per client

| Strategy | What is left running after a panic | Cost | Verdict (opinion) |
|---|---|---|---|
| `panic = "abort"` (set in the release profile) | Nothing; the process dies at once (no unwinding through foreign code). | Smaller binary; a crash is a crash. Removes the unwinding-through-C risk that made cosmic-comp's GPU case abort anyway. | Neutral. It does not prevent crashes; it makes them faster and cleaner. Use it **only if** we decide never to catch. |
| Current: unwind, `main` returns, exit status 101 (TESTED) | Nothing; destructors run on the main thread (closing the DRM device), then the process exits. | None. | Fine; a supervisor restarts it. |
| `catch_unwind` around the whole Wayland dispatch (patch P3, TESTED, T13) | **Everything else.** Clients, windows, IPC, rendering continue. The client whose request panicked stays connected with that request lost. | 14 lines. Risk: a panic in the middle of a handler can leave half-updated state, a poisoned `Mutex` (then every later `lock().unwrap()` on it panics too: class B, 40 sites), or a taken-away backend (class D). | **Useful safety net, not a cure.** It converts "crash on the first bad request" into "log and go on". Add a counter: more than N caught panics in M seconds means exit and let the supervisor restart (the cleanest state is a fresh process). |
| `catch_unwind` per output around `render_frame` | The other outputs and all input keep working; the failed output retries next frame. | 20 lines plus a guard that puts the backend back on unwind. One output (the projector) means no gain over "exit and restart". | Not worth it for one output. |
| Per client | Only the guilty client dies. | Needs to know which client's request panicked. Wayland dispatch runs all clients in one call and the request handlers are generated for each interface; there is no cheap way to attribute a panic (BELIEVED). | Not practical without changing the library. |

### 5.9 Patch set, size, and the cost of carrying it

| # | Patch | Size | Status |
|---|---|---|---|
| P1 | Adopt a Wayland socket passed with `--socket NAME --wayland-fd FD` (the `wl-restart` contract). `src/main.rs`. | 46 lines | **Built and TESTED** (T5). |
| P2 | Test hooks `hs-test-busyloop`, `hs-test-deadlock`, `hs-test-panic` on the IPC `Action` request. **Never for production.** `src/ipc/mod.rs`. | 17 lines | Built and TESTED (T6). |
| P3 | `catch_unwind` around the Wayland dispatch and around each IPC connection, switched on by an environment variable. | 28 lines | Built and TESTED (T13). |
| P4 | Config parser: `is_ascii()` check in `parse_color`; `get(..)` instead of slicing in `base_swipe_str`. | 7 lines | Built and TESTED with `--check-config`. |
| P5 | `output_state()`: use the data of a poisoned lock instead of panicking. | 3 lines | Built; not specifically tested. |
| P6 | Smithay `shm/handlers.rs`: add `return;` after the `post_error`. | 1 line | **Built and TESTED** (T17: the pristine release build exits on `wl_shm_pool.resize(0)`; the P6 build posts the protocol error, the client is dropped, and the compositor stays up). It needs a `[patch]` entry in `Cargo.toml` to our own Smithay fork, or a vendored copy. |
| P7 | Log writer that cannot block the loop (or `s6-log`). | 5 lines or config | Proposed. |
| P8 | Remove or time-box the start-up shell-outs; set the environment before threads start. | 15 lines | Proposed. |
| P9 | Fall back to a flat background if the built-in shader fails to compile. | 10 to 20 lines | Proposed. |
| P10 | If `render_frame` fails N frames in a row (for example 120, about 2 s), log once and `exit(70)`, so the supervisor restarts and the bad GPU state is not kept (see 8.2). | 15 lines | Proposed. |

The patch files and the diff against the pinned commit are in `docs/proposals/hub-stability/` (`driftwm-hub-stability.patch`, P1 to P5 in one file; `smithay-p6.patch`, P6; `hubd-prototype.patch`, the Plan A and A6 hubd prototype).

**What it costs to carry them.** The patches touch `main.rs`, `ipc/mod.rs`, `config/*` and `state/mod.rs`, not the render or input code, so conflicts with upstream should be rare (BELIEVED). driftwm is one person, pre-1.0, with a release every few weeks (34 tags to v0.19.0, SOURCE `docs/driftwm-findings.md`). Our policy is already to pin one commit and move only on purpose (`HUB-OS.md`), so each move is a rebase of about 100 lines plus a rerun of the 2,033 tests and the soak. My estimate (opinion): one to three hours per upstream bump, plus the soak time. The three Smithay-level bugs (F1, F2) need a Smithay patch, which is a second thing to carry because driftwm pins an unreleased Smithay revision.

**When would our own compositor be better?** Only if one of these is true: (a) the soak still shows crashes or growth after the patches and the cause is in code we cannot patch cheaply; (b) upstream changes become hard to follow (conflicts every bump); (c) a feature the hub needs is blocked; (d) the stripped-down fork (section 8.4, option 2) is still carrying more upstream code than we use. None of them is true today. See section 8.4 for the three options and the questions for the owner.

## 6. Can changing driftwm's source make it never crash? (plain answer)

**No.** Nothing can guarantee it. driftwm is about 47,500 lines of code of its own, on top of Smithay, the Wayland libraries, Mesa, libinput, libxkbcommon, the kernel's graphics driver and the GPU firmware. A crash can come from any of them. Changing driftwm can only change the first part.

**What changing the source can do** (all of it listed with size and status in section 5.9):

- Remove crash causes that are known: the Smithay shm bug (F1), the config parser (F3), the lock poisoning amplifier (P5), the blocking log write (F4), the start-up shell-outs (F6), the environment race (F7).
- Turn a crash into a logged error for the cases that remain: `catch_unwind` around the client dispatch (P3, tested). It is a safety net; it can leave half-updated state, so a counter must still trigger a clean restart when panics repeat.
- Remove code that the hub does not use, so there are fewer places to crash (section 8.4: screen capture, gamma, output power, touch, tablet, session lock, blur, shader backgrounds, animations are about 15,300 of the 47,500 lines and 76 of the 191 `unwrap()`s).
- Find more with fuzzing and sanitizers before they find us (section 5.4).

**What it cannot do:** bugs in Smithay that we do not find; bugs in Mesa, the kernel, the GPU firmware; logic errors that hang the loop; running out of memory; hardware faults.

**What the evidence says about how many are left (opinion):** in one day of poking (a protocol fuzzer, a config edit, two pipe tests) I found three crashes and two freezes in code that has 2,033 passing tests. That suggests more exist. The sensible design is therefore "assume it will crash sometimes and make a crash cost about three seconds" (section 7), and use the patches to make the crashes rarer.

## 7. The architecture ideas (a), (b), (c), (d), (f)

### 7.1 Idea (a): windows reappear after a compositor crash

The owner wants windows to reappear at their saved places after a compositor crash and restart, and **not** after a hub reboot. There are three ways.

| | What it is | Needs a new viewer? | What survives a compositor crash | What does not |
|---|---|---|---|---|
| **Plan A** | driftwm saves window positions and, on restart, shows each as a labelled stand-in that can relaunch its program from a `.desktop` file. `hubd` notices the restart and relaunches the viewers of the machines that were open. | No. Works with every viewer. | The node sessions (they live on the nodes). The window's place and size. The panel. | The viewer processes (they die). Each viewer reconnects to its node. Selections, drags, focus-in-the-viewer. |
| **Plan B** | A session daemon owns the network connections and the decoded pictures; thin Wayland programs only show them. | **Yes**: a viewer we write for every protocol. | Everything above, plus the viewer's connection state, the last picture, audio. | Window focus, selections owned on the Wayland side. |
| **Plan C** | A program that holds the Wayland socket (`wl-restart`) and restarts the compositor; Qt 6.6 or newer clients reconnect by themselves. | Only for Qt viewers. | The Qt viewer process, with its connection. | Position (new window placed by default), popups, OpenGL contexts. Non-Qt viewers die. |

Why the first two requirements hold by design: driftwm's session file and `hubd`'s list of open machines both live in RAM (`XDG_STATE_HOME` and `/run/hubos`). A compositor restart leaves them; a reboot clears them, so after a reboot there are no stand-ins and nothing to relaunch. TESTED in the harness only in the sense that the restart finds the file; the reboot case is by design (BELIEVED).

### 7.2 Plan A: the whole path, tested (T4)

**Set-up (TESTED):** nested driftwm (pristine release build) with `restore_windows = true` and the session file in a test folder; 20 fake machines (`fakenode` on `127.1.0.1`..`20`, inventory from `tools/geninv`); the real `hubd` of this repository started with `serve`; the viewer is `foot --app-id=hubos-<id>` started by `hubd` (the repository's own "fake" viewer). Each machine has a desktop entry `hubos-<id>.desktop` with `Exec=` the command to open it. All 20 windows were opened with `hubd open`, then the compositor was killed with `kill -9`, and a small supervisor (Python) restarted it after 1.0 s (modelled on the one-second minimum between restarts that s6 imposes: BELIEVED; the real figure for the real s6 setup was not measured).

**Check (i) of the task, "does the app-id match": TESTED, yes.** `hubd` gives each machine the app-id `hubos-<id>` (SOURCE `internal/viewers/viewers.go:29,247`). driftwm resolves an app-id to a desktop entry by exact file name first (SOURCE `src/desktop_entry.rs:resolve`), so a file named `hubos-<id>.desktop` is found. It is required: a window whose app-id has no entry is not saved as a stand-in (SOURCE `docs/session.md`: "no match means no suspended window"). After the crash all 20 came back as stand-ins, which proves the 20 entries resolved.

Results. "Kill" is time zero. "Live" means a real viewer window is mapped in the stand-in's place.

| Run | What | Viewers gone | Compositor answers | Stand-ins visible | All windows live | Windows back in their exact place | Problems |
|---|---|---|---|---|---|---|---|
| A1 | The stand-in's command is `foot ...` itself (driftwm's own design); all 20 relaunched at once with `Relaunch` | 0.09 to 0.12 s | 1.25 to 1.35 s | 20/20 at that moment | **2.5 to 2.6 s** (first live 0.15 to 0.27 s after the relaunch started, last 1.2 s after) | 20/20 (position and size identical after a 2 s settle) | None. |
| A2 | The stand-in's command is `hubd open <id>` with **today's** `hubd` | 0.09 s | 1.30 s | 20/20 | **never** (0/20 live after 120 s) | n/a | (1) `hubd` reconnects to driftwm only after its retry delay (0.5 s doubling to 4 s, `internal/hub/watch.go:17-33`), so the first `hubd open` calls were refused: "driftwm is not reachable". (2) After it reconnects, `hubd` **adopts the stand-ins as the machines' open windows** (`adopt` matches windows by the name `hubos-<id>` and does not look at the new `suspended` flag), so `hubd open` just "goes to" the stand-in. |
| A3 | A prototype `hubd` that ignores stand-ins and does not move the new window, launching one viewer at a time (as today), all 20 relaunch requests at once | 0.14 s | 1.34 s | 20/20 | last window live at 14.6 s | **9/20**; 11 windows mapped at default places and 12 stand-ins were left over (32 windows) | `hubd` serialises launches (`launchMu`, `internal/hub/open.go:179-180`), about 0.65 s each. driftwm's fallback match by app-id only lasts **5 seconds** after a relaunch starts (SOURCE `src/state/suspended.rs:66`), so everything after the 8th or 9th window missed it. |
| A4 | Same prototype, the relaunches sent one at a time, each after the previous machine's window was recorded | 0.21 s | 1.50 s | 20/20 | last window live at **14.9 s** | 20/20 | None, but 13 seconds is long. |
| A5 | Same prototype, with the serial lock switched off for viewers whose window is told apart by name (`HUBD_PROTO_NOSERIAL`), all 20 at once | 0.11 s | 1.48 s | 20/20 | **3.4 s** (first live at 2.1 s, last at 3.4 s) | 20/20, no duplicates | None. |
| A6 | **Direct restore**: no desktop entries at all (so no stand-ins); the prototype `hubd` remembers each window's place, opens all 20 machines at once after the restart and moves each new window to its remembered place | 0.14 s | 1.55 s | none (not used) | **3.35 s** (first live at 1.8 s, last at 3.35 s) | 20/20, no duplicates | None. The same time as A5 without stand-ins, matching rules or tokens. See 9.6. |

What the owner would have seen in A5 (opinion, from the timeline): the screen goes empty; about 1.5 s later the 20 labelled stand-ins appear in the right places; over the next 2 seconds they fill in with the real windows. With `foot` as the viewer. A real viewer (Moonlight, `remote-viewer`) also has to reconnect to its node, and how long that takes is UNKNOWN (it is the main unknown; it decides whether Plan B would matter).

**Crash drill: 30 kills in a row (TESTED, T4, `drill.py`).** 20 windows (`foot`, stand-ins from desktop files, driftwm's own relaunch, no `hubd`), `kill -9` of the compositor at a random moment right after random window moves, restart after 1.0 s, 30 cycles. Result: the compositor answered after a median of 1.32 s (maximum 1.57 s); 20 live windows after a median of 2.50 s (maximum 3.47 s); **no missing window, no duplicate, no stand-in left over, no damaged or quarantined session file in any cycle**; the new process had 61 to 62 open files and 148 to 155 MB each time (no growth from cycle to cycle). In **6 of the 30 cycles (10 windows in all)** a window came back at a place that was not its place at the moment of the kill: driftwm saves the session on a timer and not on every move, so a move made in the last moments before the crash is lost (the restored place is the previous saved one). Plan A6 (the `hubd` memory of positions) should not have this gap, because `hubd` sees every move event (BELIEVED, not tested with moves right before the kill), but it has the other limits listed in 7.2. Nested mode only; the real DRM path adds a GPU and the viewers' reconnect time (UNKNOWN).

**What the `hubd` changes are (the prototype is in `docs/proposals/hub-stability/hubd-prototype.patch`, 147 added lines of Go in all the modes tried (A1 to A6), not for merging):**

1. Read the `suspended` flag that driftwm already puts in every window of the `state` reply (SOURCE `src/ipc/protocol.rs`, observed in T4: `'suspended': True`), and never treat a suspended window as a viewer window (in `adopt`, `syncWindows`, the duplicate warning, and the "new window" test).
2. When a stand-in of that machine exists, do not run `place` after the window appears (it would move the window to the machine's home position and pan the camera away from the saved layout).
3. Do not serialise launches for viewers that set their own name (`sets_name = true`) and for title-matched viewers (the lock exists to tell "the new window" apart, which names already do).
4. Reconnect to driftwm quickly (poll the socket every 100 ms instead of backing off to 4 s) and, when the event stream ended while `hubd` kept running, remember which machines were open, wait for the **signal below**, and ask driftwm to `Relaunch` each stand-in.

**The signal `hubd` waits for (TESTED):** driftwm restores its stand-ins **before** it opens its IPC socket. In every run the first successful `state` reply already contained all 20 stand-ins (the "stand-ins visible" column equals the "compositor answers" column). So the rule is: the IPC socket answers, and `state` shows a stand-in for each machine that was open. I used a 10-second limit; machines without a stand-in are skipped and shown as closed. For the Wayland socket itself: Waybar and the viewers need it, and in Plan A the viewers are started by driftwm's relaunch after the socket exists.

**A race I did not close:** the 5-second fallback match uses the app-id; each machine has its own app-id, so the "oldest pending relaunch adopts the next window" capture hazard cannot pick the wrong machine (TESTED indirectly: no wrong adoption in 5 runs). But a program that does not present the token, started by hand with the same app-id within 5 s of a relaunch, would be captured. With the activation token this does not happen (section 9.5: `foot` presents it, `remote-viewer` does not).

**Cost of Plan A (opinion):** about 150 lines in `hubd` (the prototype, which also holds the test modes), 20 desktop files generated from the inventory, `restore_windows = true` and a session file in RAM in the driftwm config. No new viewer. No new protocol.

### 7.3 Plan B: a session daemon plus thin Wayland surfaces (design only)

This is a design, not something I built.

**Parts.** (1) `hubsessiond`, one process, supervised by s6 like `hubd`, with **no Wayland connection**. For each open machine it holds the network connection to the node, decodes (or just receives; the owner says sessions are uncompressed) the picture into a shared-memory frame buffer (a `memfd`), keeps the last good frame, the cursor image, the current clipboard text, and, if audio goes through it, the audio stream. (2) `hubwin <id>`, a tiny Wayland client per window: it creates an `xdg_toplevel` with the app-id `hubos-<id>`, attaches a `wl_buffer` made from the daemon's `memfd` (so the compositor reads the daemon's memory directly, no copy), and sends keyboard and mouse events to the daemon over a Unix socket. (3) The daemon does the network side of input and the clipboard.

**How a surface reconnects to a restarted compositor.** `hubwin` is cheap, so the simplest way is not to keep it alive at all: let driftwm's stand-in relaunch start a fresh `hubwin <id>` (Plan A's mechanism: `Exec=hubwin <id>`); it asks the daemon for the current frame and shows it at once. If `hubwin` should survive, it waits for the socket file to reappear (inotify on the runtime folder) and connects again with the same app-id; driftwm's fallback match puts it in the stand-in's place. A socket-holding wrapper (Plan C's `wl-restart`) would let it connect even during the restart.

**What survives:** the node connection and its state; the last frame (shown immediately, no black); audio; the daemon-side clipboard cache. **What does not:** the keyboard focus and any selection ownership on the Wayland side (re-announced), a drag in progress, window animations. **How fast:** the compositor's restart time plus the start of one small program: I would expect the stand-in-to-live step to take well under 0.3 s per window; the restart itself dominates (about 0.3 s for driftwm nested, plus s6's delay; the real figure on the real display path is UNKNOWN). That is BELIEVED, not measured.

**What it costs.** A viewer for every protocol the hub shows: Moonlight/Sunshine (the stream is decoded and rendered by Moonlight itself, an SDL application we would have to replace or wrap: a very large job), SPICE/VNC for the VM guests (existing libraries exist), SSH terminals, the file manager. It conflicts with the current plan to use existing viewers (`HUB-OS.md`). It also does not protect against a GPU reset if the daemon decodes on the GPU: that reset kills the daemon too, so decoding must stay on the CPU. **Recommendation (opinion): do not build it now.** The case for it is the reconnect time of the real viewers; measure that in December first (question 12.5).

### 7.4 Plan C: a Qt 6.6+ client survives if the compositor keeps the socket (T5)

**Sources (SOURCE, summaries by the fetch tool):** David Edmundson's post "QtWayland 6.6 Brings Robustness Through Compositor Handoffs": almost nothing is needed from the compositor, "a client that was running previously reconnecting is no different from a new client"; the client replays what the compositor needs; popups are dismissed; OpenGL contexts are treated like a GPU reset; GTK and SDL have no such support. `wl-restart` README: the compositor must accept `--socket NAME --wayland-fd FD`; Qt needs `QT_WAYLAND_RECONNECT=1`; supported compositors: KWin, Hyprland, and experimental branches of Sway and labwc. Ubuntu 24.04 ships Qt **6.4.2** (below 6.6), so a Qt 6.6+ viewer needs a newer Qt than the image has today (UNKNOWN which Qt Debian's next release or the build we make will carry; I used Qt 6.8.2 from Debian trixie, unpacked into a folder, run with its own glibc).

**What Smithay offers for a passed-in socket (SOURCE, read today):** nothing. `ListeningSocketSource` has only `new_auto()` and `with_name()` (`src/wayland/socket.rs:63-93`), and `wayland-server 0.31.13`'s `ListeningSocket` has no constructor from a file descriptor. Patch P1 (46 lines) replaces it with a `calloop` `Generic` source over a `UnixListener` built from the fd.

**Tests (T5), real `wl-restart` built from source (563 lines of C), patched driftwm, Qt 6.8.2 test client `qtc` (my 40-line widget app that prints a line each second):**

| Run | What | Compositor back | Window back | Result |
|---|---|---|---|---|
| C1 | stand-in entry present, reconnect on | 0.18 s | 0.27 s | The Qt client kept running (no gap in its once-per-second output). New window at the default place; the stand-in stays behind. |
| C2 | the window was first moved to (555,-222) and resized | 0.31 s | 0.36 s | Client alive. Size kept (the client keeps it), **position lost** (default place (0,0)); stand-in left at the old place. |
| C3 | no desktop entry (so no stand-in) | 0.25 s | 0.30 s | Same, without the left-over stand-in. |
| C4 | control: `QT_WAYLAND_RECONNECT` not set | 0.21 s | never | The client exited (status 255, "The Wayland connection broke. Did the Wayland compositor die?"). |

So the reconnect works, is fast (the restart is quick because `wl-restart` restarts at once, with no 1-second delay), and keeps the client process, but **it does not restore the window's position**, and with a stand-in it leaves a duplicate. Combined with `hubd` (move the reconnecting window to the stand-in's place and close the stand-in) it would work; I did not build that. Which of our viewers are Qt 6: KRDC (KDE) is a Qt 6 application from KDE Gear 24.02 (FreshPorts dependency list, search result); its VNC clipboard sharing is reported not working (KDE bug 499714 "VNC doesn't do clipboard sharing", search result); I could not confirm window-name control. Moonlight's streaming window is SDL, not Qt (so it does not benefit). A viewer we write in Qt 6.6+ could carry clipboard in both directions (Qt's `QClipboard`) and set its own app-id (`QGuiApplication::setDesktopFileName`, used in my test client and visible in driftwm's `state` as `hubos-q1`). `wlvncc` (any1's Wayland VNC client; I read its source today): plain `wayland-client` C program, no `xdg-activation`, **no clipboard code at all**, its window title is the VNC desktop name (set from the server; with `wayvnc --name <id>` that is the machine id), the app-id is settable with an option.

### 7.5 Ideas (b) and (c): root in RAM, and ECC memory

**(b) Root copied to RAM at boot. Recommendation: yes.**
- *What it removes.* After boot, no read from the system disk (a stalled or dying disk cannot stall a running hub): S1.
- *What it costs.* RAM equal to the root: the hub root image is about 160 MB (SOURCE `image/machines/hub.build` comment; slots are 320 MiB), and a one-time copy at boot (a few seconds from SATA, under a second from NVMe: BELIEVED; not measured). Two ways: keep the squashfs image in RAM and loop-mount it (about 160 MB, decompression on use) or unpack it to a RAM file system (more RAM, no CPU use). Updates are unaffected: the A/B scheme already reboots.
- *What it does not remove.* Anything the running hub writes to the disk. So all runtime writes must go to RAM (S1), and the config partition is read once.
- *Risk.* If the copy fails the hub must fall back to mounting the disk (a boot test). A ramfs cannot swap (there is no swap anyway) and cannot shrink under pressure: size it, do not let it grow.
- *Test.* The Phase B VM with a stalled virtual disk (proposed); the December hardware.

**(c) ECC memory on the hub. Recommendation: yes** (H1). The EPYC 4005 family supports ECC UDIMM (SOURCE). The hub handles only pictures and does not store data, so ECC is about uptime and honest error reports rather than data integrity: with ECC a single-bit error is corrected and logged, a double-bit error in a user program's page kills that program (SIGBUS: it is restarted), and a bad DIMM is visible in the logs before it crashes the hub. Cost: higher DIMM price and a board that supports it; the "candidate board" the task mentions is not named anywhere in the repository, so I could not check it (UNKNOWN). Test: December.

### 7.6 Idea (d): reserved CPU and priority so a viewer cannot starve the compositor or hubd

**The risk.** 20 viewers share the CPU with the compositor. With software rendering the compositor itself uses the CPU heavily (section 8.1). Even with a GPU, a viewer that decodes video in software can use all cores.

**What I measured (T15, default config, 3840x2160, 20 windows):** with 8 CPU-hog processes (busy loops) added to an already loaded 4-core machine, the compositor during a pan fell from 4.8 events per second and 72% of a core to 2.0 events per second and 27% of a core when the hogs ran at normal priority, and to 4.0 events per second and 54% of a core when the hogs ran at the lowest priority (`nice` 19). A `state` request over the IPC socket, 0.9 ms normally, took 374 ms with normal-priority hogs and 148 ms with `nice` 19 hogs. So CPU contention makes the compositor and its own probe slow, and plain `nice` helps only partly. Weights and CPU sets (cgroups) should give much more (BELIEVED, not tested: the machine is shared with other jobs, so a clean test was not possible).

**The design (BELIEVED, not tested on the hub):**
1. Control groups: put the compositor, `hubd`, `seatd`, `udevd` and the supervisor in group `core` with a high CPU weight (for example 10 times the viewers') and, if the machine has enough cores, a CPU set of its own (2 cores and their SMT siblings); put all viewers in group `viewers` with a low weight, a CPU limit below the total, and a memory ceiling. cgroups v1 `cpu` was writable in this environment, so the hub kernel must enable the controllers (it currently does not state this: UNKNOWN).
2. Do **not** use real-time priority for the compositor. A real-time process in a busy loop (the hang I tested, T6) is limited only by the kernel's real-time throttle (95% by default), and a stuck compositor at real-time priority makes the whole machine sluggish; with normal priority plus weights it just uses its share.
3. `oom_score_adj`: viewers 500 to 1000; compositor and `hubd` −500 to −900; never −1000 for the compositor (a leaking compositor must be killed rather than take the machine). See M2.
4. Memory floor for the core group (`memory.min`) so file pages of the compositor are not evicted when a viewer leaks.
5. Cost: a few files in `/etc` at start-up and one more thing to test. Benefit: the compositor keeps drawing and answering the hang probe when a viewer misbehaves, which also keeps the hang probe from killing a healthy-but-starved compositor (KDE's mistake, section 9).

### 7.7 Idea (f): other things I found

1. **Give driftwm a log service** (S3, S4): the hub image has none; with `s6-log` into a RAM folder, size-capped.
2. **A crash-loop limit and a quiet-start rule in the supervisor** (O5): wait for the old process to exit before starting (F8), at most 5 restarts per minute, then stop, keep `hubd` and the recovery terminal, show a fixed message on the panel, and do not reboot.
3. **Run the viewers without GPU use if possible** (G1, V): a bad GPU job from a viewer can reset the GPU for the compositor too.
4. **Turn off config hot reload** (C2) and the unused protocols (section 8.4).
5. **Remove the start-up shell-outs** (F6, F7).
6. **Test a GPU reset on purpose in December**, and a monitor off/on (O3).
7. **A black box for the compositor**: keep the last 200 lines of driftwm's log and the hang probe's evidence in the RAM ring that already exists in `docs/proposals/black-box-recorder.md`.
8. **Waybar and the viewers all die with the compositor** (T3): the s6 `follow-driftwm` dependency already restarts Waybar and `hubd`; Plan A restarts the viewers.

### 7.8 Recommendation for (a), with the measured blank times

Use **Plan A**, with the four `hubd` changes in 7.2, and add "direct restore" (9.6, run A6: `hubd` itself remembers each window's place) for the Moonlight machines, which stand-ins cannot tell apart (9.5). Measured with `foot` as the viewer, nested, 1-second supervisor delay: viewers gone at once (0.1 s); screen empty for about 1.4 s; stand-ins in the right places at 1.4 s; all windows live again at about **3.4 s** after the crash (A5), **2.6 s** if relaunched by driftwm's own command (A1). With the real viewers add their reconnect time (UNKNOWN). Plan C's 0.3 s is faster but only for Qt 6.6+ clients, loses position, and needs a driftwm patch plus a socket-holding process; keep it as an option if we write a Qt viewer. Plan B is the only one that can hide a slow reconnect, and the most expensive; decide after the December measurement of real viewer reconnect times. A combination is possible: Plan A as the baseline and `wl-restart`-style socket holding added later; they do not conflict (the held socket makes the restarted compositor accept viewers that started during the gap).

## 8. The GPU, software rendering, and a smaller compositor

### 8.1 Software rendering for the hub (CPU draws instead of the GPU)

**The idea (from the manager and the owner).** If the sessions are not compressed the hub does no video decoding; it only composes about 20 windows. A CPU renderer could take the GPU's render stack out of the compositor's critical path. The owner accepts it only if it has no meaningful tradeoff. Here is every tradeoff I found.

**Two routes.** (1) Run the **unmodified** driftwm on Mesa's software GL, llvmpipe (set `LIBGL_ALWAYS_SOFTWARE=1`; the hub image's `desktop.env` already does this in virtual machines). (2) Port driftwm to Smithay's pixman renderer.

**(b) Does Smithay at the pinned revision have a pixman renderer? Yes (SOURCE, read today).** `src/backend/renderer/pixman/mod.rs` (1,320 lines) and `error.rs` (74 lines), behind the feature `renderer_pixman` (`Cargo.toml:110`). It implements `Renderer`, `ImportMem`, `ImportMemWl`, `ImportDma`, `ExportMem`, `Bind<Dmabuf>` and offscreen rendering. The DRM compositor can use it (`src/backend/drm/compositor/mod.rs:146,851,1222`: it already creates a `PixmanRenderer` for the cursor plane), and Smithay's example compositor `anvil` enables it (`anvil/Cargo.toml:54`). driftwm does not: its Smithay feature list has `renderer_gl` only, and the word "pixman" does not appear anywhere in its source, documents or manifest (a recursive search found nothing).

**What a port would touch (opinion, from counting).** 209 mentions of GLES types in 22 files; the render directory is 9,267 lines of code. The GLES-only parts are the custom render elements and the GLSL shaders: shadow, border, corner clip, blur (1,335 lines), shader and tiled backgrounds (2,443 lines in six files), closing animation, screen capture (for example `render/elements.rs` 35 mentions, `blur.rs` 36, `closing.rs` 18, `capture.rs` 17, `background.rs` 16, `mod.rs` 15). The `Backend` enum in `src/backend/mod.rs` holds a `GlesRenderer` and every frame function takes one. A port means: make the backend generic over the renderer or add a second variant; delete or replace every shader-based element with plain rectangles and images; keep the window textures (shared-memory buffers import cleanly). My estimate: 3,000 to 5,000 lines changed or removed, mostly removed. I did not try it. A pixman-only driftwm is really the "stripped" fork of section 8.4 with a new renderer.

**(a) Measurements (T15).**

| Variant (20 foot windows, 700x525 each, nested 3840x2160) | idle: CPU % of one core | pan (camera glides): CPU ms per frame event, events/s | 20 windows moving: CPU ms per frame event, events/s | RSS MB |
|---|---|---|---|---|
| min: no background shader, animation_speed 1.0, no blur | 32% | 132.7 ms, 5.7/s | 203.3 ms, 1.9/s | 278 |
| min, repeated (second run) | 27% | 140.2 ms, 5.3/s | 193.2 ms, 2.0/s | 278 |
| min with LP_NUM_THREADS=2 (llvmpipe limited to 2 threads) | 28% | 135.8 ms, 5.5/s | 184.8 ms, 2.0/s | 277 |
| default config (built-in dot-grid shader background, default animation speed) | 15% | 150.4 ms, 4.8/s | 199.7 ms, 1.9/s | 280 |
| shader: animated smoke shader background (fast_smoke.glsl) | 65% | 812.6 ms, 0.9/s | 1195.0 ms, 0.6/s | 286 |
| blur: every window blurred and 90% opaque | 91% | 1408.1 ms, 0.8/s | 1273.5 ms, 1.0/s | 489 |
| heavy: shader background + blur | 74% | 6033 ms, 0.1/s | 5880.0 ms, 0.1/s | 442 |

How to read it. **Method:** nested driftwm (release build, pristine) on a virtual X server of 3840x2160, Mesa llvmpipe (software GL), 20 `foot` windows of 700x525 in a 5x4 grid, each printing one line a second. Three 30-second phases: idle; "pan" = the camera glides to a new place every 1.2 s; "moving" = a random window is moved every 0.5 s. CPU = user plus system time of the driftwm process from `/proc` (it does not include the X server or the viewers). "Frame event" = one push of the `Subscribe` stream (driftwm sends one per rendered frame while something animates), so ms per frame = CPU time divided by events. **Cautions:** (1) the 4-core machine was shared and heavily loaded (load average 10 to 20: my soak, fuzzers and other helpers), so CPU time per frame is only roughly right and events per second are lower than a free machine would give; idle CPU for the same work varied between 15% and 32% between runs, so read all numbers as plus or minus 50%. (2) Nested mode also pays to hand every frame to the virtual X server; the DRM path does not, but copies into a scan-out buffer (BELIEVED). (3) Limiting llvmpipe to 2 threads made no visible difference on this oversubscribed machine. (4) The QEMU DRM mode was not measured (section 13). **Reading:** at 4K a full-screen change (a pan) costs 130 to 150 ms of CPU even with every effect off, that is at most 6 to 8 screens a second on one core before anything else runs; blur and an animated shader multiply it by 6 to 40. Idle (only text lines changing) costs 15 to 30% of a core.

**Other results from the same runs (T15):** the render probe of 9.3 (a screenshot at scale 0.05) took 4 to 5 ms in 5 of 8 tries and 42 to 157 ms in the other 3 (a frame being drawn at the same time); the CPU-hog test is in 7.6.


**(c) Tradeoffs, all of them:**

1. **CPU use and heat.** Measured (T15, loaded machine, see the cautions): with 20 windows each printing a line a second the compositor alone used 15 to 32% of one core when idle, 43 to 76% of a core while the camera glided, 31 to 39% while windows were moved, and 65 to 130% of a core with a shader background or with blur. Heat was not measured; it follows CPU use (BELIEVED), so a hub that now idles at a few percent would run noticeably warmer.
2. **Frame pacing and pointer smoothness.** On the DRM path the mouse cursor can be a hardware plane (driftwm uses it unless `disable_hardware_cursor` is set: SOURCE `src/backend/udev.rs:1560`), so the arrow itself is drawn by the display hardware. But the compositor handles input and drawing on one thread, so a 130 to 200 ms frame (measured below at 4K) also delays input handling by that much while it is being drawn. With a GPU the same frame takes a few milliseconds. For a user who navigates the canvas with the mouse (pans), this is the visible cost.
3. **Effects lost.** The owner does not use blur, shader backgrounds, animations; losing them costs nothing. (Shadows, rounded corners and borders also need shaders; the owner did not mention them: question 12.9.)
4. **What a heavy viewer can do to the compositor's frame time.** With software rendering the compositor needs a lot of CPU, so a viewer that uses all cores slows every frame. This is the largest risk of the idea. Priority and CPU reservation help (section 7.6): with 8 busy-loop processes the pan rate fell from 4.8 to 2.0 events a second and the IPC `state` request from 0.9 ms to 374 ms; with the hogs at the lowest nice level the pan rate was 4.0 and `state` took 148 ms (T15).
5. **Maturity of the software path, and new crash causes.** llvmpipe is used by every virtual machine and every CI system, so it is well exercised, but not for a 24/7 desktop at 4K. It is multi-threaded and compiles code at run time with LLVM (BELIEVED: a bug there would crash the compositor in a new way). In this research **every** driftwm test ran on llvmpipe (that is how the nested mode works), so for *driftwm's own* code it is the most exercised path here; the real display path `--backend udev` with llvmpipe was run only in the earlier QEMU experiment (`docs/proposals/phase-b-desktop.md`) and not by me. An address-space limit of 900 MB made the compositor segfault at start (T14).
6. **Memory.** The compositor was 207 to 280 MB resident in these runs with no background shader, 270 to 290 MB with a shader background, 400 to 490 MB with blur (T15), against 140 MB with no windows; the soak shows 200 to 245 MB with 20 windows (T16). On a real GPU the window pictures live in video memory instead; how much is UNKNOWN.
7. **What still depends on the GPU.** The **display engine** and the kernel driver still drive the screen: the compositor still opens the DRM device, allocates scan-out buffers (with software GL these are dumb buffers or GBM buffers in GPU-visible memory), sets modes and commits page flips through the amdgpu driver, and the driver loads GPU firmware. A GPU reset resets the display engine, so the screen can still blank and the commit calls can still fail or block (SOURCE: the kernel resets the whole device; BELIEVED for the effect on KMS). The December 2025 AMD hangs were in the display code, "pause the workload setting in the display manager when doing idle optimization" (SOURCE, search-result summary of Phoronix), which software rendering would **not** have avoided. What software rendering does remove: the compositor's own GL context (the thing that is lost in a reset), shader compilation, and GPU jobs from the compositor, and, if viewers also use shared memory, every client GPU job (so a client can no longer fault the GPU).
8. **Testing becomes the same as production.** The VM tests and the real hub would run the same renderer (a plus; BELIEVED).

**(d) Verdict (opinion).**
- *Where it wins:* it removes the renderer-lost failure class (the cosmic-comp case) from the compositor; it removes shader and driver compile failures; it removes GPU clients' ability to take the compositor down; the hub's tests equal production.
- *Where it loses:* **panning and any full-screen change at 4K is slow** (hundreds of milliseconds per frame under load, and the measured numbers below were taken on a busy 4-core machine, a real 8 or 16-core hub would be better but I could not measure that); more CPU and heat; the compositor becomes sensitive to CPU contention, so the CPU reservation of 7.6 becomes mandatory; the display engine is still a GPU dependency.
- *Worth it?* Not at 4K as it is measured here. At a lower resolution (1920x1080 has a quarter of the pixels, so about a quarter of the cost: BELIEVED) it may be acceptable. It becomes interesting if the owner's projector is 1080p, or if the GPU path proves unstable in December. The decision should wait for the December measurement of GPU reset behaviour (question 12.7). The cheap route (unmodified driftwm on llvmpipe) can be switched on with one environment variable, so keeping it as a fallback costs nothing.

### 8.2 GPU-reset audit of driftwm (task 2d-i)

**What the cosmic-comp bug was, and whether driftwm has it.** SOURCE (search results and the fetch tool's summary of the pull request page; cosmic-comp PR 2487, merged 28 July 2026): cosmic-comp asks a renderer manager (`GpuManager`) for a renderer at every frame; after an amdgpu reset the call fails transiently, `.unwrap()` panicked, and the panic unwound across the EGL/GBM boundary, which aborts the process. **driftwm has no such per-frame acquisition.** It creates exactly one `GlesRenderer` at start (`src/backend/udev.rs:552`), stores it in `data.backend`, and takes it out and puts it back around each frame (`:1502` ... `:1673`). So that exact bug class does not exist here. **What exists instead is the opposite problem: there is no way to get a new renderer.** If the context is lost, driftwm retries the dead one for ever.

**Every `unwrap`, `expect`, `unreachable` on the render path and the device-acquisition path** (SOURCE, read):
- Start-up only (a failure there ends the process before any window exists, and the supervisor retries): `udev.rs:593` (backend just stored), `:601` (`expect("failed to build dmabuf feedback")`), `:692` (backend take), `winit.rs:51,58`. For GPU selection failures driftwm does **not** panic: it tries the next GPU and then returns an error (`udev.rs:451,588`).
- Per frame: `udev.rs:1502` (take the backend; proved safe), `render/cursor.rs:37` (pointer; safe), `:138` (cursor buffer after `load_xcursor("default")` check), `render/lifecycle.rs:25` (keyboard; safe), `render/closing.rs:150,164,171` and `render/capture.rs:604,826` (mutex locks), `render/layers.rs:361`, `render/mod.rs:1563`, `render/blur.rs:1048,1049,1300` (blur, guarded). None of them depends on the GPU being alive.
- GPU-dependent: **`render/background.rs:748` and `:832`: `.expect("Default shader must compile")`.** If the GL context is dead (or the driver refuses the shader) at the moment the background is (re)built, which happens at output hot-plug, config reload and first draw, driftwm panics. That is the same shape as the cosmic-comp bug, in a rarer place.
- No `unreachable!` is on the render path.

**What driftwm does when the device or the renderer is lost (SOURCE):**
- A failed `render_frame` or `queue_frame` is logged (`warn`: "Render frame error", "Failed to queue frame": `udev.rs:1636-1646`) and retried at the next estimated vertical blank (`queue_estimated_vblank_timer`, `:1750`), for ever. There is no counter, no escalation and no rate limit on the log line.
- `DrmEvent::Error` is logged only (`udev.rs` line 754). `UdevEvent::Removed` (the GPU disappearing) is logged only (`:930-932`); `UdevEvent::Added` says "ignoring — single GPU". So a vanished GPU leaves a running, blind compositor.
- A render fence that does not signal is waited for with a bound: 2 s, then 250 ms, then 50 ms per frame while it keeps missing (`:1330-1348`); the authors wrote this so the VT switch can still work on a wedged GPU. So a wedged GPU can still hold the single thread for up to 2 s per frame at first.
- Smithay at the pinned revision does not ask for robust GL contexts: a search of its source finds no `robustness` or `graphics_reset` call; only some GLES errors are mapped to `ContextLost` (`backend/renderer/gles/error.rs:103,132`). So driftwm cannot even learn that the context was reset unless a call fails.
- A buffer import that fails is handled for dma-bufs (3.4 G3).

**Likely outcomes after a real GPU reset (BELIEVED, not testable here):** (i) the driver or Mesa kills the process: the supervisor restarts it and Plan A restores the windows; (ii) the compositor keeps running but renders nothing (failing frames for ever): the hang probe passes (the event loop answers), so it needs a **render probe** (section 9.3) or patch P10 below; (iii) it keeps drawing garbage. Which one happens depends on the Mesa version and kernel; December must test it.

**The smallest patch that turns these into a retried error:** P10 (section 5.9). Count consecutive failed frames (`Err` from `render_frame` or `queue_frame`) per output; when the failures last longer than 2 seconds, log once, `exit(70)`, and let the supervisor start a new process (a new process is the only way to get a fresh GL context, because Smithay offers no renderer rebuild). About 15 lines in `render_frame`. Together with P9 (flat background if the shader does not compile) the cosmic-comp shape is gone. A real in-process renderer rebuild (new EGL display, context, all shaders, all window textures) would be 200 lines or more and is the kind of code that is hard to test without a real GPU reset; I do not recommend it.

### 8.3 What "turn effects off" actually skips (task 2ii-b, question for option 1)

SOURCE: the shaders for shadow, border, corner clip and blur are compiled **at start-up whatever the config says** (`backend/udev.rs:695-700`, `backend/winit.rs:63-67`), so the effect code and its shader compilation stay in the process. What the config does turn off: `[background] type = "none"` stops drawing the built-in dot-grid shader (the default background **is** a shader: `config/types.rs:1353`, `shaders/dot_grid.glsl`); blur is per-window opt-in and default off (`[decorations] blur = false`), and the blur path is not entered while no window asks for it; `[effects] animation_speed = 1.0` makes the move/resize/open/close interpolation instant (the animation state machines still run). The measured difference between the variants is in the table above (rows `min`, `default`, `blur`, `shader`, `heavy`).

### 8.4 A smaller compositor for the hub

The owner says driftwm's default features are everything the hub needs, except the visual effects (blur, shader backgrounds, animations). The requirement list says: windows are remote-session viewers; a canvas with each machine at a fixed home position; mouse-only navigation; zoom never magnifies; fit-to-viewport instead of fullscreen; click to focus; layer-shell for the bar and menu; saved layouts and stand-ins; one projector (maybe more outputs later); keyboard and mouse input from the hub and the forwarded gaming-box input; per-machine app-ids; and **no games, no pointer locking, no touch, no idle or session lock, no effects.**

**Conflict, reported and not resolved (question 12.8).** The owner's sentence "the defaults are everything the hub needs" and the list "no touch, no pointer locking, no idle or session lock" disagree for touch (gestures, touchscreen grabs, tablet), pointer constraints, idle notification and session lock, which are all on by default. I classified by the owner's sentence, as asked, and mark the conflicting groups.

**Features of driftwm at the pinned commit** (SOURCE: `README.md`, `docs/*.md`, `src/`; code lines are non-blank, non-comment, non-test lines from `perfile.txt`; `uw/ex/us/un` = `unwrap`, `expect`, `unsafe`, `unreachable!`+`panic!`):

| Feature | Class (by the owner's sentence) | Files | Lines | uw/ex/us/un | Notes |
|---|---|---|---|---|---|
| Canvas: pan, zoom (to 100%), momentum, camera, navigation, bookmarks, home, edge pan | REQUIRED | `canvas.rs`, `state/{viewport,viewport_animation,navigation,edge_pan}.rs` | 1,882 | 18/0/0/0 | 16 of the 18 unwraps are in `viewport_animation.rs`, which is mostly pointer warping and not eye candy. |
| Stage, windows, focus, click-to-focus, fit, fill, fullscreen, pinning, snapping, clusters, placement, resize, move | REQUIRED | `stage/*`, `state/*` (window parts), `layout/*`, `grabs/{move,resize,pan,navigate}*`, `region.rs`, `window_ext.rs` | 9,028 | 14/9/1/2 | The core. |
| xdg-shell, compositor and seat handlers, window rules, decoration negotiation | REQUIRED | `handlers/{compositor,mod,xdg_shell}.rs` | 2,607 | 27/1/0/0 | |
| wlr layer-shell (bar, menu), exclusive zones | REQUIRED | `handlers/layer_shell.rs`, `state/layers.rs`, `render/layers.rs` | 566 | 2/0/0/0 | Waybar and the menu need it. |
| Saved layouts and stand-ins (session restore, suspend, relaunch, activation) | REQUIRED | `session.rs`, `state/{session_store,persistence,suspended,activation}.rs`, `desktop_entry.rs`, `render/suspended.rs` | 2,634 | 2/4/0/0 | Plan A depends on it. |
| IPC (`state`, `move`, `resize`, `focus`, `relaunch`, subscribe, screenshot) | REQUIRED | `ipc/*` | 1,598 | 3/1/0/1 | `hubd` uses it. |
| Config, hot reload (inotify), start-up, signals, fonts | REQUIRED (hot reload: question 12.4) | `config/*`, `state/reload.rs`, `main.rs`, `signals.rs`, `text.rs` | 5,374 | 7/3/13/0 | F3 lives here. The 13 `unsafe`s are `env::set_var`, signals and `pre_exec`. |
| Backends: udev/DRM/libinput/libseat, winit, mode timing | REQUIRED | `backend/{udev,winit,mod,cvt}.rs` | 1,790 | 8/1/2/1 | |
| Rendering core: compose, elements, cursor, error bar | REQUIRED | `render/{mod,elements,cursor,error_bar,lifecycle}.rs` | 2,864 | 6/0/0/0 | Shared with the effects (blur and closing hook in here). |
| Input: pointer, keyboard, bindings, actions | REQUIRED | `input/{mod,pointer,keyboard,actions}.rs` | 3,890 | 28/0/0/0 | The forwarded gaming-box input arrives as ordinary kernel input devices. |
| Clipboard, primary selection, drag and drop, cursor shape, fractional scale, viewporter, presentation time, dma-buf, shm, xdg-activation, xdg-foreign, dialog | REQUIRED | Smithay + `handlers/*` | in the rows above | | F1 lives in Smithay's shm code. |
| **Touch** input, touch grabs, gesture recogniser (swipe, pinch, hold), tablet | owner's sentence: REQUIRED (default); **owner's list: UNNEEDED** | `input/{touch,gestures*,tablet}.rs`, `grabs/touch_*` | 3,301 | **36**/0/0/3 | The largest single group of `unwrap()`s (36 of 191). Needs a touch device or a touchpad gesture to reach. |
| **Pointer constraints** (lock, confine), relative pointer, keyboard-shortcut inhibit | owner's sentence: REQUIRED; **list: UNNEEDED** | `input/constraint.rs` + parts of `input/mod.rs`, `state/viewport_animation.rs`, `state/fullscreen.rs` | 56 + parts | 0 + parts | The one named hang hazard in `caveats.md` (the surface lock) belongs here. |
| **Idle notification, idle inhibit, session lock** | owner's sentence: REQUIRED; **list: UNNEEDED** | `state/session_lock.rs`, parts of `handlers/mod.rs` | 235 + parts | 2/0/0/0 + parts | |
| Screen capture: screencopy, ext-image-copy-capture, screenshots (`msg screenshot`) | REQUIRED (default); I judge it unneeded | `render/capture.rs`, `render/screenshot.rs`, `protocols/{screencopy,image_copy_capture,image_capture_source}.rs` | 1,795 | 15/0/3/6 | Question 12.8. `msg screenshot` is a useful debugging tool. |
| Output power (DPMS), gamma, output management, workspace list, foreign-toplevel list, virtual keyboard | REQUIRED (default); I judge them unneeded | `protocols/*`, `backend/gamma.rs` | 2,614 | 5/0/1/8 | 8 of the 19 `unreachable!`. |
| XWayland launcher (`xwayland-satellite`) | REQUIRED (default; not installed here) | `xwayland.rs` | 127 | 0/0/1/0 | The hub does not run X11 (`HUB-OS.md`). |
| **Blur** | **UNNEEDED** (effect) | `render/blur.rs`, `handlers/background_effect.rs`, `shaders/blur_*.glsl` | **1,432** | 6/1/4/0 | Four of the 25 `unsafe`s. |
| **Shader and image backgrounds, tiled and gigapixel wallpapers, chunk bakes** | **UNNEEDED** (effect) | `render/{background,capture_background,shader_chunks,tile_chunks,tile_chunks_tiff,tile_worker}.rs`, `shaders/*bg*.glsl` | **2,443** | 9/2/0/0 | Starts worker threads. The default background is also a shader. |
| **Animations** (window open, close, move, resize; close animation) | **UNNEEDED** (effect) | `state/{window_animation,window_animation_driver}.rs`, `render/closing.rs` | **2,293** | 3/0/0/0 | |
| Shadows, rounded corners, borders (shader chrome) | not named by the owner: OPTIONAL | `render/shaders.rs`, `decorations.rs`, `render/chrome.rs` | 1,030 | 0/0/0/0 | Question 12.9. |
| **Total** | | 121 files | **47,559** | **191**/22/25/21 | Sum of the rows above (`features.py`). |

Totals (from `groups.py`): effects (blur + backgrounds + animations) **6,168 lines of 47,559 (13%)**, 18 `unwrap`, 3 `expect`, 4 `unsafe`, 0 `unreachable`; all the groups above marked as conflicting or unneeded together (effects, shader chrome, touch, pointer constraints, session lock, capture, output/gamma/other protocols, xwayland) **15,317 lines (32%), 76 `unwrap`, 3 `expect`, 9 `unsafe`, 17 `unreachable`/`panic`**; the rest ("core") 32,242 lines, 115 unwrap, 19 expect, 16 unsafe, 4 unreachable. The 14 `unreachable!()` protocol arms (in the capture, output and gamma protocols) and the pointer-lock deadlock hazard all sit in groups the owner's list or my judgement calls unneeded.

**Freeze hazards of `caveats.md` and where they sit:** the `Space` rule and the `output_state` rule are in the core and not removable; the "surface user-data lock" rule bites mostly in pointer constraints and cursor/window code (constraint code is in the conflicting group); the "never block the event loop" rule is violated by the core's file and log writes (F4, F5) and the effects' file reads (`render/background.rs:425,632,730,799`).

**The three options, with cost, risk and what each does to stability:**

| | (1) Unmodified driftwm, effects off in the config | (2) Stripped driftwm: a patch set that removes what the hub does not need | (3) Our own minimal compositor |
|---|---|---|---|
| What changes | Config only: background `none`, `animation_speed = 1.0`, no blur, hot reload off. (Section 8.3: the effect code is still compiled and the shaders are still compiled at start.) | Delete the effects (6,168 lines) and, if the owner agrees, the conflicting groups and the unneeded protocols. | New program on Smithay (or wlroots) for exactly the requirement list. |
| Lines removed / written | 0 | 6,000 to 9,000 for effects alone (13 to 19%), plus the call sites in shared files; up to about 15,300 (32%) with the conflicting groups (opinion) | About 8,000 to 12,000 new (opinion). `smallvil` (Smithay's minimal example) is under 1,000 lines; `anvil` is larger; driftwm's own "core" is 32,000 with everything. |
| Effect on crash and freeze exposure | None: all code stays; the unneeded protocols stay advertised. | Removes 76 of the 191 `unwrap`s and 9 of the 25 `unsafe` only if all groups go; effects alone remove 18 `unwrap`, 4 `unsafe`. Fewer advertised protocols means less to fuzz. | New code has new bugs; it starts with no tests. A smaller surface, but a larger unknown. |
| Cost to keep up with upstream | Lowest: a config file. | Medium to high: deleting files is easy, but the call sites sit in `render/mod.rs`, `render/layers.rs`, `render/elements.rs`, `state/mod.rs` and `state/init.rs`, which upstream changes often (1,244 commits in the first seven months, about 6 a day). Expect conflicts at every bump (opinion: 5 to 15 hunks, 2 to 6 hours). | None for upstream; all upkeep is ours, including Smithay version bumps. |
| Tests we can reuse | All 2,033. | Most (the effect tests would be deleted). | The pure-logic ones: `canvas`, `layout/snap`, `config` tests (350 tests in the 11 integration files) if we keep those modules; the 1,356 in-process protocol tests are tied to driftwm's `DriftWm` type (BELIEVED). |
| Risk | Low. The stability problems found do not depend on effects. | Medium: a fork of an AI-built single-maintainer project that moves fast. | High: `HUB-OS.md` allows it only after a design discussion. This is a proposal, not a plan. |

**A cheaper way to shrink the exposed surface (opinion, not tested): hide the unneeded protocols instead of deleting them.** driftwm already hides privileged protocols from clients that connect through a security-context socket (`ClientState.is_restricted`, `client_is_unrestricted` in `src/state/mod.rs:1099`, used for screencopy, foreign-toplevel, virtual keyboard, data-control, gamma, output power/management, session lock, layer-shell, input method). A 20-line patch that gives the viewers a second, restricted listening socket (the same code as P1) would let the viewers reach only the ordinary protocols; Waybar and the menu use the normal socket. It does not touch the F1 crash (the shm code is ordinary), but it removes about 15,000 lines from what a viewer can reach.

**Recommendation (opinion):** option (1) now, plus the small patch set of section 5.9 and the restricted viewer socket; revisit option (2) only for the effect code once the owner has answered question 12.8, and only if the soak shows a problem inside it. Do not start option (3) before the December measurements. **Questions for the owner:** 12.4 (hot reload), 12.8 (touch, pointer locking, session lock, idle, screen capture), 12.9 (shadows, rounded corners), and the resolution of the projector (12.7).

## 9. Detecting a hung compositor, and the viewers' activation tokens

### 9.1 The policy and the probe (task 2c and 2d-iii)

**Owner's policy:** a hung compositor is killed and restarted, never a reboot; the restart triggers the restore path (7.2).

**What can be probed from outside (TESTED, T6).** Two independent probes, both answered by the compositor's single event-loop thread:
1. driftwm's IPC socket, the `state` request (the same call `hubd` already makes: `internal/driftwm`, 5 s deadline). It does not need a Wayland client.
2. A Wayland round trip: connect to the Wayland socket, send `wl_display.sync`, wait for the `done` event (2 s limit). This tests the Wayland dispatch path, which the IPC socket does not.

**What KDE got wrong, and how this probe avoids it (SOURCE, search results about KWin).** KWin's systemd unit used `WatchdogSec` (about 5 seconds at first, 15 s in reports): the compositor had to call the watchdog from its own loop in time. A compositor that was alive but slow (out of memory, just woken from sleep, busy) missed the deadline and was killed, "nuking the session"; KWin 6.1 disabled the watchdog (KDE bugs 485024 and 487750 and the discussion "Plasma 6.1 Beta KWin watchdog triggered on wake"; the manager's note says the revert commit is dated 5 February 2024, which I did not verify). The rules that avoid it:
- **A long window.** Kill only after at least 4 failed rounds **and** at least 30 seconds since the first failure. How long a GPU reset takes: the kernel's default job timeout is 2000 ms per queue (kernel documentation page, summary by the fetch tool; older text said 10 s for graphics), and the reset itself takes seconds more (UNKNOWN, no figure found). 30 s is longer than both. (The window is a setting, and December should measure the real reset time and add a margin of at least 3 times.)
- **Two probes, and both must fail.** One failing probe is not enough (it could be a fault in that path only, for example a full IPC backlog, and killing for it would repeat KDE's mistake). My test harness was stricter toward killing: a round counted as failed if **either** probe failed; in every test both failed in every round from the second round on (in the first round of the busy-loop and deadlock tests only the Wayland probe had failed), so the result is the same. I recommend requiring both.
- **Probe from outside, never from the compositor's own loop.** A slow compositor cannot be killed by missing its own heartbeat; it is killed only when it fails to answer someone else for 30 seconds.
- **Guards (proposal, not built):** do not kill while the machine is short of memory (the memory pressure file `/proc/pressure/memory` shows stalls) or while the CPU is saturated by viewers (lower their share first, 7.6); at most 3 kills in 10 minutes, then stop restarting, show a message, keep the recovery terminal, and **do not reboot**.
- **Log every kill with the evidence**: the time; the last N probe results; `/proc/PID/status` State; `/proc/PID/wchan`; the CPU ticks (to tell a busy loop from a deadlock); the last 50 lines of driftwm's log; the top CPU users; the memory-pressure and CPU-pressure files. In the test the harness wrote these lines (sample from the busy-loop run):

```
 54.960 HUNG: 5 consecutive missed probes over 30.1s; evidence: ipc/wl_sync failing; killing pid 4356 (SIGKILL) and letting the supervisor restart
 54.960 evidence /proc status State: State:	R (running)
 54.960 evidence wchan: 0
 54.960 evidence cpu ticks: 3123
```

The three hang shapes are distinguishable from the evidence: stopped process (`State: T`, `wchan: do_signal_stop`, CPU ticks do not move); busy loop (`State: R`, `wchan: 0`, ticks growing: 3,123 against about 450 normally); deadlock (`State: S`, `wchan: futex_do_wait`, ticks do not move: this is the signature the `caveats.md` file describes for the surface-lock freeze).

### 9.2 Results (T6): detection, kill, restart, restore

Set-up as in 7.2 (20 machines, nested driftwm patched with the test hooks, native stand-in relaunch with `foot` as in run A1). The probe ran every 2 s plus its own time-outs, so a round took about 7 s once the compositor stopped answering.

| Injected | First failed round | Kill after | Evidence (State / wchan) | Viewers gone | Compositor back | Stand-ins | All 20 live again (after the kill or exit) | Positions |
|---|---|---|---|---|---|---|---|---|
| `SIGSTOP` | at 5 s | **33.1 s** (5 rounds) | T / `do_signal_stop` | 0.11 s after the kill | 1.4 s after the kill | 20/20 | 2.5 s | 20/20 identical |
| busy loop (spin in the IPC handler) | at 2 s (the first IPC answer still came: the loop was entered after it) | **30.1 s** | R / `0`; ticks 3,123 | 0.08 s | 1.3 s | 20/20 | 2.3 s | 20/20 |
| deadlock (a `Mutex` locked twice in the IPC handler) | at 2 s | **30.0 s** | S / `futex_do_wait`; ticks 465 | 0.09 s | 1.37 s | 20/20 | 2.8 s | 20/20 |
| panic (index out of range in the IPC handler) | none needed: the process exited with status 101 after 0.38 s | n/a | n/a | 0.38 s after the injection | 1.66 s | 20/20 | 3.2 s | 20/20 |

The supervisor in all rows was a Python loop that restarts the process 1.0 s after it exits. The numbers for "all live" are from the kill (or the exit), so a hang costs 30 s of waiting plus about 3 s of recovery; a crash costs about 3 s.

### 9.3 A blind compositor needs a render probe

An event loop that answers both probes can still draw nothing (section 8.2: after a GPU reset driftwm keeps logging render errors for ever). A third probe is needed: ask for a tiny screenshot, which makes the compositor render the viewport into an offscreen texture and read it back, so a dead GL context should fail. `driftwm msg screenshot` exists (`Request::Screenshot` with a scale: SOURCE `src/ipc/protocol.rs`). T15 measured: a tiny screenshot (scale 0.05, 192x108 pixels) of the 4K viewport with 20 windows took 4 to 5 ms in 5 of 8 tries and 42 to 157 ms in the other 3 (a frame being drawn at the same time), against 0.9 ms for a `state` request. BELIEVED that it fails on a lost context; I cannot test that here. Caveat: the screenshot request is part of the screen-capture code that section 8.4 lists as unneeded, so if that code is removed a smaller `msg render-check` is needed. The patch P10 (exit after 2 seconds of failed frames) is the cheaper first line.

### 9.4 Where the probe lives

In `hubd` (or a tiny separate supervisor program next to it), not in driftwm and not in s6. Reason: `hubd` already talks to the socket and has the machine list. Its own health is covered by the confirm step's ping and, if the owner agrees, by the board watchdog (a hung `hubd` is not a hung hub; its restart is s6's job).

### 9.5 Do the viewers present the activation token? (task 2d-iv, T7)

How the test works: the relaunched program gets the compositor's token in `XDG_ACTIVATION_TOKEN`. driftwm adopts a new window into a stand-in's slot by the token if the program presents it (it works even if the program uses a different app-id), otherwise by app-id within 5 seconds. So I suspended a window, changed its launch command so the **new** program uses a **different app-id**, and relaunched: the new window can only be adopted if it presented the token.

| Viewer | Result |
|---|---|
| `foot` 1.16.2 (Ubuntu 24.04 package) | **Presents the token.** The window with the other app-id `zz-different-appid` took the stand-in's place and size (700x525 at 0,0). TESTED. |
| `remote-viewer` 11.0 (virt-viewer, GTK 3, Ubuntu 24.04 package, Wayland backend) | **Does not.** The new window appeared at [1,0] instead of the stand-in's place and the stand-in stayed dormant. TESTED, with a limit: with no SPICE server the window that appears is the connection-error dialog; I assume the main window follows the same GTK start-up path (BELIEVED). |
| `wlvncc` (any1) | Source read: no `xdg-activation` code, no clipboard code, app-id settable, window title = the VNC desktop name. Not built (needs a patched libvncclient and `aml` from meson sub-projects) and not run: UNKNOWN. |
| Through `hubd` | The token is delivered to the `hubd open` command driftwm starts, not to the viewer: the viewer `hubd serve` starts inherits `hubd serve`'s environment. TESTED: the viewer printed `token=` (empty) in all 20 runs. `hubd` would have to carry the token from `hubd open` through its socket to the viewer's environment (BELIEVED easy). |

**Consequences.** (1) Plan A with today's viewers relies on the 5-second app-id fallback, which is fine because `hubd` gives every machine its own app-id (`hubos-<id>`, SOURCE; in T4 20 distinct app-ids, no wrong adoption). (2) **Moonlight is different:** all Moonlight windows share one app-id (`com.moonlight_stream.Moonlight`) and are told apart by title (`docs/hubd-slice2.md`, `title_match`; SOURCE). driftwm's stand-ins know only the app-id (a saved record "carries no title": SOURCE `docs/session.md`), and one desktop entry cannot say which machine to open, so Plan A's stand-in relaunch **cannot tell the Moonlight machines apart**, and the "oldest pending relaunch adopts the next window of that app-id" rule is the wrong-window hazard the manager mentioned. The safe alternative is "direct restore": `hubd` itself remembers each machine's window place and, after a restart, opens the machines and puts each window back (run A6 below). That needs no stand-ins, no desktop entries and no token.

### 9.6 "Direct restore": `hubd` puts the windows back by itself (run A6)

**The idea.** `hubd` already receives driftwm's window list on every change (`RunWatch`). It remembers, for each machine, the centre and size of the window it knows. When driftwm's event stream ends while `hubd` keeps running (a compositor restart or crash), it keeps the list of machines that were open; when driftwm answers again it opens each of them, **all at once**, and when the viewer's window appears it resizes and moves it to the remembered place instead of the machine's home position. No desktop entries, no session file, no stand-ins, no token. The prototype (mode `direct` in `hubd-prototype.patch`) is part of the same patch.

**Run A6 (TESTED).** The same 20-machine set-up as A1 to A5 but **no** `.desktop` files, so driftwm has no stand-ins. Kill, then: viewers gone after 0.14 s; the compositor answered after 1.55 s; the first window was live at 1.8 s and the last at **3.35 s**; **20 of 20 windows back in exactly their old position and size**, no duplicates, nothing left over. That is the same time as the stand-in route (A5, 3.4 s) without any of its matching rules, its 5-second limit, or its dependence on the token.

**What differs for the owner.** With stand-ins there are 20 labelled boxes for about 0.6 to 2 seconds before the viewers fill them. With direct restore the screen is empty for about 1.8 seconds and then the windows appear one by one. I think the stand-ins are friendlier (opinion), they cost nothing to keep, and they survive the case where `hubd` itself was the thing that died; but direct restore is the only route that works for the Moonlight machines, which share one app-id.

**Risks and limits.** It relies on `Move` and `Resize` being applied after the window maps (a window may sit at the default place for a frame or two before it moves: not measured). It is only as good as `hubd`'s record (kept in RAM, a few lines to write it to `/run/hubos`, so a `hubd` crash at the same moment loses it). It does not restore the stacking order or which window had the focus (driftwm's stand-ins restore both: SOURCE `docs/session.md`).

**Combination (opinion):** direct restore for every machine, plus stand-ins for the machines whose viewers have unique app-ids if the owner likes the placeholders. Both need the same `hubd` change (ignore suspended windows).

## 10. The soak (task 3, T16)

**What I ran (TESTED).** The pristine release build of driftwm at the pinned commit (byte-identical to the `driftwm-pristine` binary used elsewhere), nested in a virtual X server of 1920x1080, with Mesa llvmpipe (software GL). Up to 20 `foot` terminals as stand-in viewers, each printing a line a second. A driver script (`soak.py`, seed 42) did one random operation every 0.1 to 0.9 s: open a window, close one, pause or crash a client, move, resize, focus, zoom, pan, set a bookmark, suspend a window to a stand-in, relaunch a stand-in, and mouse and key input through `xdotool`. Every 20 s it sampled the compositor's memory, open files, threads and CPU from `/proc` and timed an IPC `state` request. The run was planned for 4 hours. **I stopped it after 2 hours 30 minutes (9,017 s)** to finish the work; that is the honest length, it is above the 2-hour minimum and short of "as many hours as possible". It shared 4 CPUs with other jobs (load average 10 to 20), which makes it a harsher test of timing than an idle hub would be.

**Result.**

| Measure | Result |
|---|---|
| Crashes, restarts, panics | **None.** One compositor process (the same PID) for the whole run; 0 restarts; 0 `panicked` lines in its log. |
| Hangs | **None seen.** 445 samples; 0 failed IPC probes. The `state` request took a median of 0.3 ms; the 95th percentile was 0.9 to 6.6 ms depending on the period; the worst single reply was 55.7 ms. |
| Churn | 4,043 client connections and 4,043 new windows over the run (about 27 a minute), with at most 20 windows alive at a time (mean 19). |
| Memory (resident) | First 10 minutes: mean 204 MB. Minutes 10 to 30: 222 MB. Minutes 30 to 60: 232 MB. Hour 2: 242 MB. Last 30 minutes: 242 MB (maximum 254 MB). Highest value in the whole run: 266 MB. **It grew over the first hour and was flat after that.** A straight-line fit gives +6 MB an hour after the first 30 minutes, but the scatter from window churn is plus or minus 10 MB, so I cannot tell a slow leak of a few MB an hour from none. 2.5 hours is too short to say anything about days (UNKNOWN). |
| Open files | Mean 65 to 75, maximum 101, no trend (fit: 0 per hour). Threads: 11 from start to end. |
| CPU | 22% of one core on average with 20 windows each printing a line a second (software rendering; a real GPU should be lower: BELIEVED). |
| Log | `driftwm.log` grew to 1.77 MB (about 0.7 MB an hour at the default level). 5,501 of its lines are one repeated error from the nested mode (`eglQuerySurface BAD_SURFACE` on the host window); I believe this appears only with the nested backend (BELIEVED, not checked on DRM). It matters for one reason: the hub needs a log reader with rotation, because a full or unread log pipe froze driftwm in test T11. |
| Fuzzing at the same time | None. Fuzzing was run separately (section 5.4). |

**What happens to the clients when the compositor is killed:** not part of the soak; it was tested separately (T3, section 7.1 and 7.4): `foot` and `remote-viewer` exit within 0.06 to 0.1 s; a Qt 6.8.2 client without `QT_WAYLAND_RECONNECT` exits (status 255, "The Wayland connection broke"); the Qt reconnect route is in Plan C (7.4). The 30-kill drill (7.2) covers the restore path.

**What this soak cannot show (and nothing in this environment can):**

- Anything about the real graphics path: the DRM/KMS device, the real GPU driver, GPU reset, buffer-allocation limits and video memory use, scan-out and page flips. A software renderer exercises different code (UNKNOWN how much of the compositor's own code is shared with the real path, BELIEVED most of the window-management code).
- Real viewers. `foot` draws text from a local socket. Moonlight, `remote-viewer` and `wlvncc` decode video, hold network sessions, use more memory, and may use other protocols (viewporter, dmabuf, subsurfaces, text input). A slow leak that depends on those is invisible here.
- Heat, power, fans, a failing disk, a flaky cable, a monitor that is turned off and on (hotplug), suspend, a long run (days to weeks), and the real network (the owner said to assume it is perfect).
- Concurrency with a real `hubd` (the soak had none; the interplay is in section 7).

## 11. Hub stability requirements for the December hardware (short list)

These are tests and checks, in the order I would do them. A "definition of done" for the hub is at the end.

1. **ECC:** ECC UDIMMs, a board that supports them, ECC switched on in the firmware, EDAC reporting in the kernel (`dmesg | grep -i edac`), and a 24-hour memtest86+ pass before anything else.
2. **Firmware:** choose one firmware version, write down every setting (a checklist kept with the machine's config), memory at JEDEC speed, "restore on AC power loss" on, the board's watchdog/TCO enabled. Never update the firmware during a soak; a new version restarts the soak.
3. **Watchdog boot test:** stop the feeder (`kill -STOP` of `watchdog -F`) and see the board reset after the timeout; `kill -9` the compositor and `hubd` and see that it does **not** reset. Confirm which kernel watchdog driver the real board uses.
4. **Compositor-crash drill with real viewers:** 50 times `kill -9` driftwm with the 20 real windows open; record, per viewer type (Moonlight, `remote-viewer`, SSH terminal, file manager), the time from the crash to a live window and to a usable picture. This decides Plan B (question 12.5).
5. **GPU reset on purpose:** with the full desktop running, trigger a reset 10 times (BELIEVED: the kernel's debugfs file for amdgpu recovery; verify the name on the board) and record what driftwm does (killed, alive and blind, alive and drawing), how long the screen is black, and what the viewers do. Set the hang probe's window to at least 3 times the longest reset seen (section 9.1).
6. **Projector off and on, 20 times,** and cable unplug/plug: the single output disappears and returns (O3). Check driftwm keeps running, the windows keep their places, and nothing logs in a loop.
7. **Disk stall:** after boot with the root in RAM, make the system disk stall or vanish and check the hub keeps running (S1).
8. **Power pull:** pull the plug during a soak; the hub must boot to the panel with no windows, without file-system repair.
9. **Thermal:** 24 hours with all 20 windows live and a loop of window moves; log CPU, GPU and board temperatures; check fans.
10. **Priority test:** start a CPU-hog viewer and a memory-leaking viewer; the compositor's frame time and the `hubd` probe must not suffer, and the leaker must be killed first (7.6, M2).
11. **Clock step:** step the clock forward and back by hours during a soak; nothing may time out wrongly (TM1).
12. **Fuzz every driftwm bump:** run `fuzz.py` (release and the ASan build) for at least 2 hours, plus the T10 config check, before a new driftwm commit is accepted. Add the one-request reproducers to the test list.
13. **Kernel and Mesa:** every new kernel or Mesa gets the same soak before it replaces the pinned one; the boot test checks `tainted == 0`.
14. **The multi-day soak (the gate):** at least **72 hours** (and then one week at idle) with the real desktop: 20 real windows from real nodes, the random operations driver (`soak.py`, extended with real viewers), a periodic compositor crash drill, hang-probe armed, logs in RAM. **Pass:** zero compositor exits that were not injected; zero false kills by the probe; resident memory of each program flat within 5% after the first hour; file descriptors flat; no kernel warnings; `tainted == 0`.
15. **Software-rendering fallback:** run the same soak once with `LIBGL_ALWAYS_SOFTWARE=1` on the real CPU to learn the real cost at the real resolution (8.1).

**Definition of done for the hub's stability (opinion):** items 1 to 8 and 12 to 14 pass, and the owner has seen a compositor crash and a hang recovered with the windows back in place.

## 12. Questions for the owner (nothing is decided here)

1. **Which patches may we carry?** P1 to P5 are built and tested (socket handover, config parser fix, panic catch, lock poison, test hooks which are never shipped); P6 is built and tested (Smithay shm fix); P7 to P10 are proposed (log writer, start-up shell-outs, default-shader fallback, exit after 2 s of failed frames). May we carry them, and may we report the Smithay shm bug (and the `debug_assert!` panics) to Smithay? (I did not file anything.)
2. **After a power cut,** do you want the hub to start by itself and show the panel with no windows (the design today)? That needs the firmware setting "restore on AC power loss".
3. **Kernel panic:** reboot after 5 seconds (today, `panic=5`) or hang until the hardware watchdog fires (180 s)? And do you agree that a board watchdog reset for a frozen kernel is not a "reboot because of a service failure"?
4. **Config hot reload:** turn it off on the hub, so a config mistake cannot kill a running compositor (T10)? Changing the config would then mean a compositor restart (about 3 s with Plan A).
5. **Plan B (a session daemon and our own viewers):** may we defer the decision until the December measurement of how long each real viewer needs to reconnect after a compositor restart?
6. **Moonlight machines share one window name** (`com.moonlight_stream.Moonlight`), so driftwm's stand-ins cannot tell them apart (9.5). Do you want `hubd` to remember each window's place and put it back itself ("direct restore", 9.6) for all machines, or stand-ins for the machines that have their own name and direct restore only for Moonlight?
7. **What resolution is the projector,** and is software rendering only a fallback? At 4K the measured software cost is high (8.1); at 1080p it should be about a quarter.
8. **Features the hub does not need but the defaults enable:** touch and gestures, pointer locking, idle and session lock, screen capture and screenshots, output management and gamma, virtual keyboard. Your sentence says the defaults are everything the hub needs; your list says "no touch, no pointer locking, no idle or session lock". Which is right? If the list: we can hide them (a second, restricted socket for viewers) or later remove them.
9. **Shadows, rounded corners, borders** use shaders too. Are they needed? (They were not in your list of effects.)
10. **Restart loop policy:** if the compositor crashes 5 times in a minute, stop restarting and show a fixed message while `hubd` and a recovery terminal stay up, never rebooting. Agree?
11. **Kill policy of the hang probe:** 4 failed rounds and 30 seconds, at most 3 kills in 10 minutes, then stop. Agree, or longer?
12. **Resource limits:** may viewers run in a lower-priority, memory-limited group (so a leaking Moonlight cannot starve the compositor)? What is the largest memory you would allow all viewers together?
13. **ECC and the board:** may we buy ECC memory, and which board is "the candidate board"? (It is not named in the repository.)
14. **Logs across reboots:** logs live in RAM and are lost at reboot. Keep the last crash evidence on the config partition (capped), or is the RAM black box enough?
15. **A "restart the desktop" button** in the panel that restarts the compositor service (and nothing else)?
16. **How long after a kernel or Mesa release** before we adopt it (I suggest at least 4 weeks and a 72 hour soak)?

## 13. What I did not verify, helpers, and files

**Not done, and why:**
- **The real display path** (`--backend udev`), a real GPU, a real GPU reset, real heat, real firmware, real hotplug, a real disk stall. No such hardware or device exists here (no `/dev/dri`, no `/dev/kvm`).
- **The QEMU DRM mode of the llvmpipe measurement.** The task asked for nested and QEMU DRM. I measured nested only. Reason: QEMU is not installed here, and under software emulation (TCG) the CPU is 10 to 50 times slower, so a CPU cost measured inside it would be meaningless; the earlier QEMU experiment (`docs/proposals/phase-b-desktop.md`) showed the DRM path starts on llvmpipe, nothing more. The nested numbers also include the cost of copying each frame to the virtual X server, which the DRM path does not have (BELIEVED).
- **Frame time of the real compositor loop.** I measured CPU seconds per frame event and event rate. I did not read frame times from driftwm.
- **Memory bandwidth** (no counters here). My only statement is arithmetic: a 3840x2160 frame in 32-bit colour is 33 MB, so 60 full frames a second are 2 GB/s of writes plus reads (BELIEVED).
- **Ubuntu 24.04's Qt (6.4.2)** does not have the reconnect support; the Plan C test used Qt 6.8.2 from Debian trixie unpacked into a folder, with its own glibc, from an unsigned mirror (`[trusted=yes]`, because the Debian key was not available). That is acceptable for a throw-away test and must not be reused for an image.
- **wlvncc** was read, not built or run.
- **Smithay fix P6** was built and tested only for the `resize(0)` case; other Smithay paths were not re-audited with it. The F2 effects in release builds are BELIEVED, not tested.
- **Sanitizers other than AddressSanitizer** (thread, memory, undefined behaviour) and **Miri**, **valgrind** (installed, not used), **cargo-fuzz**, **wlcs**, **fault injection** (malloc, kernel) were not run.
- **The catch-the-panic idea** was tested only for a panic inside a request handler that does not hold a lock; panics while a Smithay or Wayland lock is held, or with a taken-out backend, were not tested.
- **The clock-jump test** was not run (it would disturb the other jobs on this machine).
- **Long-term results beyond the soak length** (section 10).

**Helpers:** no helper agents were used. Every source reference was read by me; web pages read through the fetch tool are marked as summaries. Search results (titles and snippets) are labelled SOURCE only for the fact in the snippet.

**Rule note:** once during this research I used `pkill -f` on my own test script by mistake (the command matched only my own harness and my own shell; no other helper's process was touched). I did not use it again.

**Files added by this branch (`docs/proposals/hub-stability/`):** the scripts that produced the tests (`soak.py`, `planA.py`, `planC.py`, `fuzz.py`, `fuzzrun.sh`, `fuzzloop.sh`, `llvm.py`, `tokentest.py`, `audit.py`, `perfile.py`, `groups.py`, `features.py`, `cat2.py`, the small reproducers and helpers), `driftwm-hub-stability.patch` (P1 to P5, against the pinned commit), `smithay-p6.patch` (P6, against Smithay rev 4cf0b62), `hubd-prototype.patch` (the Plan A and direct-restore prototype, not for merging), and `results/` (timelines, fuzz summaries, the soak sample file, the llvmpipe numbers). The scripts contain the paths of my scratch folder; set `HS_WORK` to your own work folder.
