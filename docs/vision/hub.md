# Hub: one-page target list

AGI premise, not buildable today. Machine: hub. Date: 2026-10-14. Status: approved by the owner on 2026-10-14. Track: theorycraft (project file HUB-OS-AGI.md). The real build's source of truth stays HUB-OS.md on main; nothing here changes it.

Labels: TESTED = a command and its output. SOURCE = a link or file and date. BELIEVED = not checked. UNKNOWN = not established. Almost every number below is a target, not a measurement.

## Fixed boundaries used
- Hardware is fixed. All vendor firmware is a fixed boundary (board, GPU, network chip, SSD, projector, DAC). Firmware time is not counted in any target.
- The network between machines and the internet are perfect: zero latency, unlimited bandwidth, always on, free. Everything inside the hub (PCIe, memory, storage) is real.
- Electricity is free, unlimited and always on. Power never fails.
- The projector is decided in December. Until then targets are in refreshes at 60 Hz (1 refresh = 16.7 ms; HUB-OS.md: 4K at 60). Projector lag is measured, not targeted.
- "Full load" = 20 windows open, each redrawing its whole 4K picture at 60 frames per second, plus sound from every node.

## Targets
Format: what is measured; target; why; how it will be checked; label.

Never freeze or crash
1. Longest stall of picture or pointer: at most 33 ms (2 refreshes) in a 30-day full-load run with no faults injected. Why: a frozen hub blocks every machine. Check: a capture card films an on-screen frame counter; firmware pauses are logged separately. BELIEVED.
2. Hub reboots or desktop restarts in that run: 0. Why: there is no spare hub. Check: the run log, plus machine-checked proofs that no hub program can crash on anything a node sends. BELIEVED.
3. A failing node, window or sound stream costs 0 frames elsewhere. Its window keeps its last picture marked stale and goes live within 1 s of the node coming back. Why: one bad machine must not hurt the rest. Check: 1,000 injected failures. Stale picture: SOURCE (HUB-OS.md, session layer). Numbers: BELIEVED.

Recovery when a part of the hub fails
4. A hub program dies or hangs (a program restart, not a reboot): the screen is back within 1 s with the same windows in the same places and sessions alive; 0 reboots; 0 false kills. Why: uptime comes first. Baseline in the real build: about 35 s to notice a hang and about 3 s to restore (TESTED there, docs/proposals/bulletproof-compositor.md). Check: kill and freeze each program 100 times. Targets: BELIEVED. Baseline: SOURCE.
5. GPU reset, or projector switched off and on: no reboot; windows back within 2 s after the device itself is ready (the device's own reset or sync time is measured, not targeted). Why: both can happen in normal use. Check: 50 forced GPU resets and 50 projector power cycles. BELIEVED.
6. Disk dies: every window keeps running and an alert shows; ECC memory corrects bit errors and reports them. Why: a dead disk must not stop work. Check: switch the disk off in a test mode. SOURCE (HUB-OS.md: system copied into RAM, ECC).
7. Interrupted save: 0 settings lost or corrupted when the hub is reset (watchdog or kernel panic) in the middle of a save. Why: this replaces the power-cut target, because power never fails. Check: 200 forced resets at random moments. BELIEVED.
8. Part replaced: after a failed part is replaced, the hub is restored from the NAS backup to its previous settings, saved layouts, mixer settings, credentials and crash record in at most 5 minutes, at most 3 steps, with no typed commands. Windows are not reopened (they never reopen by themselves). A new board's firmware settings come on top. Why: there is no spare hub. Check: wipe the disk, restore, time it. BELIEVED.

Click to window
9. An open machine's window is fully shown within 33 ms of the click. A closed machine's live desktop is shown within 50 ms. Why: both are under the 0.1 s "feels instant" limit. Check: hub timestamps plus light-sensor spot checks; worst of 1,000 clicks. Targets: BELIEVED. 0.1 s limit: SOURCE (Nielsen, Response Times: The 3 Important Limits, 1993, updated 2014, https://www.nngroup.com/articles/response-times-3-important-limits/, read 2026-10-14).

Input delay on the hub side
10. The hub adds at most 1 ms to any key, click or pointer move, worst case in 24 h. This excludes the wait for the projector's next refresh. Why: delay is felt on every action. Check: hub timestamps at arrival, hand-off and screen flip, plus a light-sensor check end to end. BELIEVED.
11. Input flip: the hub takes input within 1 refresh of the flip reaching it; 0 stuck keys or buttons in 10,000 random flips in the middle of a press. Why: a stuck key acts on its own. Check: scripted flips. No stuck keys: SOURCE (HUB-OS.md, input sharing). Numbers: BELIEVED.

Sound
12. All nodes heard at once, mixed, with per-node volume and mute, a master volume and a mute-all, settings remembered per node; 0 dropouts in 24 h with every node playing. Why: the owner listens to several machines at once. Check: hub counters plus a recording of the DAC output. Features: SOURCE (HUB-OS.md). 0 dropouts: BELIEVED.
13. Lip sync at the owner's seat: sound at most 5 ms early and at most 15 ms late against the picture, projector lag compensated. Why: experts notice from 45 ms early or 125 ms late. Check: a flash-and-click test with a light sensor and a microphone, 1,000 samples. Limits: SOURCE as quoted on https://en.wikipedia.org/wiki/Audio-to-video_synchronization (ITU-R BT.1359-1 detectability 45 ms lead / 125 ms lag; EBU R37 per-stage +5 / -15 ms), read 2026-10-14; the standards themselves were not read.
14. Bit-perfect sound: with one node playing and its volume and the master at 100%, nothing else playing, the bits sent to the DAC equal the node's bits; 0 differing bits over 10 minutes. Why: the owner asked for it. Check: record the digital stream just before the DAC and compare bit for bit. The DAC's own conversion is fixed hardware and is not checked. Mixed sound: a target will be proposed with the design. Label: target; UNKNOWN whether the mixer path allows it until the DAC is chosen.

Picture
15. Whole and exact: at 100% zoom the hub's output carries exactly the node's pixels. Every frame is shown whole, once, at a refresh; 0 torn frames (part old, part new) in 24 h at full load. Cost accepted: an average wait of 8.3 ms, 16.7 ms at worst, at 60 Hz (TESTED, arithmetic). Check: a capture card films a test pattern that changes every frame; a torn frame shows as a split. Lossless: SOURCE (HUB-OS.md). Rest: BELIEVED. Open: this target assumes lossless pictures.

Clipboard
16. A copy anywhere lands in the history within 100 ms; a paste reaches the chosen machines within 100 ms; text arrives byte-identical in any language (the real build's Ubuntu wayvnc 0.7.2 garbled non-ASCII text). Why: it should feel instant. Check: 10,000 scripted copies with emoji and non-Latin text. BELIEVED; 0.1 s limit: SOURCE (Nielsen, as in target 9).
17. Nothing is pasted without the owner's action; no echo loops; the history holds at least 500 text items plus 100 MB of images and file references, searchable, with pins kept; files move node to node only on paste, never through the hub. Why: no surprises in any machine. Check: scripted tests. SOURCE (HUB-OS.md, Clipboard).

Updates and rollback
18. 0 updates unless the owner starts one. 100 of 100 unsigned or tampered images refused. The screen is gone at most 10 s during an update. Why: nothing changes without the owner. Check: a set of tampered images; timed updates. Rules: SOURCE (HUB-OS.md). 10 s: BELIEVED. Board firmware time: UNKNOWN.
19. Bad update: the hub goes back to the old image by itself, with the panel up within 30 s of the fault, for faults up to 10 minutes after boot (100 of 100 injected). Recovery mode works with both slots broken, using the spare keyboard. Why: a bad update fixes itself. Baseline in the real build: it waits 120 to 180 s, and its trial check once missed a fault 30 s after start (TESTED there, docs/proposals/init-comparison.md). A/B and recovery: SOURCE. Numbers: BELIEVED.

Maintenance
20. 0 routine tasks. Anything the hub cannot fix itself shows on the bar within 1 s, in plain words, with one button. No typed commands outside recovery mode. Why: the owner is not a cluster operator. Check: every injected fault above ends in a fix or an alert. No schedules, no terminals: SOURCE (HUB-OS.md). Rest: BELIEVED.
21. Crash record: every crash, hang, reset and rollback is saved with its cause and kept through reboots. Why: causes can be found without an expert. Check: one record per injected fault. SOURCE (HUB-OS.md, black box recorder).

Also matters
22. Down alert: a machine that stops answering shows as down within 1 s, and as up within 1 s of being ready; 0 false alerts in 7 days. The network is perfect, so "down" always means the machine itself failed; the real build's "unreachable" state is not used. The real build's own limits (2 s per machine, 5 s total) are guesses (HUB-OS.md, Unverified). Why: trouble is visible at a glance. Check: 100 node pulls and boots. BELIEVED.

Scale
23. 20 windows: every target is met with all 20 changing every frame. 100 windows: every target is met with 100 open and 20 of them changing every frame; RAM use at most 50%. Why: the owner's cluster plus headroom. Check: the same runs at 20 and at 100. 20 and 100: SOURCE (HUB-OS.md scale reference). 50%: BELIEVED. Physics: 100 windows all changing every frame is 199 GB/s of raw 4K pictures (3840 x 2160 x 4 bytes x 60 x 100; TESTED, arithmetic; 4 bytes per pixel is BELIEVED). The candidate board's two-channel DDR5-5200 has a theoretical peak of 83.2 GB/s (SOURCE for 2 channels and 5200: https://www.thomas-krenn.com/en/wiki/AMD_EPYC_4005_Grado, read 2026-10-14). So all 100 changing cannot be met on that board; the limit on other hardware is UNKNOWN.
24. 5,000 machines listed (stretch): no crash or freeze; the list and search answer within 100 ms. Why: the real build's stretch target. Check: a fake list of 5,000. Stretch target: SOURCE (HUB-OS.md). 100 ms: BELIEVED.

## Not decided yet
- Projector model, refresh rate and lag: December.
- Mixed-sound target (several nodes at once): with the hub design.
- Whether pictures must stay lossless if another display protocol is chosen: open (target 15).
- The project file calls the hub's GPU "discrete AMD"; HUB-OS.md on main does not say discrete. Check in December.
- No clock-serving target: the owner decided against it.
- Phone page and shared phone sessions: dropped from this track.
- No design here. The design stage comes after the owner approves these targets (already approved on 2026-10-14).
