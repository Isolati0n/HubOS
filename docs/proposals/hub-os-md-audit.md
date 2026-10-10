# Audit: where HUB-OS.md may be behind later owner decisions (read-only; HUB-OS.md is not edited by this file)

Written 2026-10-10 on `main` at 2b9173e. Method: for each topic, (a) what `HUB-OS.md` says now (line numbers are of `HUB-OS.md` at that commit), (b) whether a repo document records a NEWER decision, with its path, or "not recorded in the repo". Nothing here is a guess: where the repo is silent the entry says "not recorded". Labels: SOURCE = read in the file named. Owner decisions given only in chat are not in the repo, so they appear as "not recorded". Written by the lead agent; no helper wrote any part.

## 1. Machine roles dropped in favour of named machines (only "hub" and "guest of that host" kept)
- (a) `HUB-OS.md` lines 65-77: the "Machine roles" table (hub, gaming box, AI/GPU box, general desktop node, NAS, backup NAS, VM host); line 110: "Roles are text labels only: the inventory gives behaviour to hub, guest and vm-host and to nothing else."
- (b) **Not recorded in the repo.** `docs/inventory-format.md` line 24 still says `role` is required and is exactly one of eight values (`hub`, `gaming`, `ai`, `desktop`, `nas`, `backup-nas`, `vm-host`, `guest`). `docs/proposals/distro-workshop.md` (lines 224, 247) calls the role "a text label only (HUB-OS.md)". Neither records a move to named machines.

## 2. Designed for at most 20 machines
- (a) Lines 47, 65, 222: real-world reference at most 20 machines; hardware and always-open windows sized for 20; headroom for 100, stretch 5000.
- (b) Recorded and consistent: Change log 2026-10-04 (line 577); `docs/hubd-slice2.md` section 7 (100 and 5000 measured with fake nodes); `docs/proposals/hub-stability.md` tests with 20 windows. No newer decision found.

## 3. Display protocol; whether Moonlight/Sunshine stay; hubd's Moonlight code
- (a) Line 101: wayvnc/neatvnc, security type None, no passwords; line 503: VNC with wayvnc, PipeWire RTP for sound; line 523: "Moonlight and Sunshine are not planned for use; hubd's Moonlight support ... stays in the code and is not removed until the display protocol is final, and is then deleted." Line 70 and 332: the gaming box has no game window on the hub.
- (b) `docs/proposals/remote-display.md` banner (2026-10-06) records no passwords and no encryption. The same file's summary item 7 (line 25) still says "Sunshine and Moonlight stay for the gaming box (decided)", which is OLDER than `HUB-OS.md` line 523 and the no-game-window rule: the proposal is out of date, not newer. hubd's Moonlight code is still in the source (SOURCE: `internal/viewers/viewers.go` and its tests contain it); no document records the deletion happening or a date. A final display protocol decision: not recorded (the session layer, topic 4, is a direction).

## 4. The session layer (workers, conductor, bake-off, Raw RFB)
- (a) Lines 157-166 (owner direction 2026-10-06, nothing built): one worker per machine, a presenter, an Elixir/OTP conductor, Raw rectangles with security type None, RFB Extended Clipboard, no ContinuousUpdates, no Fence; native-core language NOT decided; bake-off of Rust with Kani, Ada with SPARK, Zig with the gates listed. Also line 521 (open question) and line 195 (stage 1 and a separate 24-hour soak).
- (b) `docs/STATE.md` section 3.2 repeats it as a parked item. The session-layer design review (the written answers about buffer ownership, memfd sealing, SCM_RIGHTS, conductor epoch fencing, chaos design) was given in chat and is **not recorded in the repo**; no dedicated session-layer proposal file exists. Background docs named by STATE.md: `hub-isolation.md`, `hub-stability.md`, `clipboard.md`, `notifications.md`, `remote-display.md`.

## 5. Security assumptions (no display passwords, no request signing, only the image signature stays)
- (a) Line 14 (security is not a concern), lines 100-104 (recovery agent plain requests; hub requests not signed or encrypted; only the image signature stays), line 101 (no password).
- (b) Recorded in: `docs/proposals/secrets.md` (rows marked "REMOVED (owner, 2026-10-06)"), `docs/proposals/remote-display.md` and `docs/proposals/file-transfer.md` (owner-decision banners), `docs/proposals/recovery-and-out-of-band.md`, `docs/proposals/notifications.md` line 11. **Stale:** `docs/proposals/node-helper-api.md` still describes signed requests (as `notifications.md` line 11 says). No newer decision found.

## 6. Clipboard history and targeted paste
- (a) Lines 274-281: history of every copy with source and time, targeted paste, rich items, quick picker, RAM history with pins on disk, about 500 text items plus about 100 MB.
- (b) Recorded: `docs/proposals/clipboard.md` section 1 (owner decisions 2026-10-06). Nothing built. No newer decision found.

## 7. File transfer tickets
- (a) Line 278: files are references that move node to node only on paste (the hub never carries the bytes). Line 519: the file-transfer questions (file-transfer.md section 8) are for the owner to answer later. **`HUB-OS.md` does not mention tickets.**
- (b) Recorded in `docs/proposals/file-transfer.md` banner and section 2.3: since the owner decision of 2026-10-06 a ticket is a plain one-time RANDOM string that the sender's helper remembers (path, receiver, expiry, used once); it makes the destination pull the right file once and is not for security (the tested prototype signed tickets). So `HUB-OS.md` is behind this one detail.

## 8. Notifications daemon
- (a) Lines 283-288: the daemon, toasts of about 5 s at the top right, bar badge and list, severity, serious ones stay until dismissed.
- (b) Recorded: `docs/proposals/notifications.md` section 1 (owner decisions). Nothing built. No newer decision found.

## 9. Hub stability rules (watcher, restart limits, memory limits, kernel cgroups and pressure information)
- (a) Lines 123-137: after 5 compositor crashes in a minute no more restarts and a fixed message; hang probe kills the compositor after 4 failed rounds and 30 s, at most 3 kills in 10 minutes (NOT built, waits for the init comparison); viewers in a lower-priority memory-limited group, all viewers at most 25% of RAM and each at most 1 GiB; line 186 and 183 (memory ceiling, restart on abort).
- (b) Restart limit and hang rule: `docs/proposals/hub-stability.md` (owner decisions 2026-10-05 round 5) and `docs/proposals/driftwm-patches.md`. A **user-space watcher** that kills the largest viewer above its cap appears only as a proposal (`docs/proposals/hub-isolation.md` line 234); that it was decided: **not recorded**. **Kernel:** `hub-isolation.md` line 167 and `docs/STATE.md` section 4 (known gaps) record that the hub kernel has no control groups and no pressure-stall information, so the memory-group decision cannot be implemented until the kernel is rebuilt; that the owner decided to add those kernel options: **not recorded** (it is step 1 of a proposal list). `HUB-OS.md` does not mention this gap.

## 10. Kernel, Mesa and firmware adoption rule
- (a) Line 137: "A new kernel or Mesa is adopted only after at least 4 weeks and a 72-hour soak." Line 112: kernel and graphics-stack versions pinned and tested; NAS and backup NAS updated a week apart.
- (b) `docs/proposals/hub-stability.md` line 22 (same wording, as owner decision 16) and line 847 (the question). The parts named in the request, **"72-hour soak on the real hub", "2 weeks probation" and any rule for firmware, are not recorded** in the repo (searched for "probation", "2 weeks", "two weeks", "firmware" rules).

## 11. From-scratch hub build
- (a) Line 32 (eventually built from scratch, kernel stays upstream Linux, current Ubuntu image is the scaffold), line 522 (open question: which components stay upstream).
- (b) `docs/proposals/from-scratch-hub.md` banner: the decision was relayed by the lead on 2026-10-05; the staged plan itself is a proposal, "nothing here is a decision". No newer decision found.

## 12. Distro workshop and machine files
- (a) Line 155: a node's whole OS is declared in one file and whether it boots is checked before deployment (a preflight); points to `docs/proposals/distro-workshop.md`. Line 520: which distro each node gets is undecided.
- (b) `docs/proposals/distro-workshop.md` banner: the owner decided that the workshop exists; stages, file format, names are recommendations; owner questions are in its section 10. Contract items: `docs/proposals/distro-contract.md`. No decision on the file format: not recorded.

## 13. The "Erlang/Elixir parked" entry
- (a) Line 530 ("Parked ideas"): "Erlang/Elixir for uptime: parked; revisit later. v1 uses Go." Line 545 (Change log 2026-10-01) records it as parked. **But line 162 (2026-10-06) puts an Elixir/OTP conductor in the session layer, and line 578 says "Go stays (Erlang parked)".** `HUB-OS.md` therefore contradicts itself.
- (b) The init comparison carries an Elixir brain as a candidate (`docs/proposals/init-comparison.md`). A document that records the owner lifting the "parked" status: **not recorded**; the only trace is line 162 itself.

## Recorded in the repo (HUB-OS.md matches, or the repo holds a newer detail)
- Scale of 20 machines (2); security assumptions (5); clipboard history (6); notifications (8); from-scratch direction (11, as a relayed decision); workshop exists (12); kernel/Mesa 4 weeks and 72-hour soak wording (10, first part).
- Newer detail in the repo that `HUB-OS.md` lacks: file-transfer tickets are plain random one-time tickets (7).
- Out-of-date documents (older than `HUB-OS.md`): `remote-display.md` item 7 (Sunshine/Moonlight "stay"), `node-helper-api.md` (signed requests), `inventory-format.md` (eight roles required).

## Not recorded: needs the owner
1. Machine roles replaced by named machines, with only "hub" and "guest of that host" kept (1).
2. Whether the Moonlight code in hubd is to be deleted now or at a date (3), and the final display protocol.
3. The session-layer design review answers (buffers, memfd, SCM_RIGHTS, epoch fencing, chaos design) as a repo document (4).
4. Whether the user-space watcher for viewer memory is decided, and the owner's decision to add control groups and pressure information to the hub kernel (9).
5. "72-hour soak on the real hub", "2 weeks probation" and a firmware adoption rule (10).
6. The format of the declarative machine file (12).
7. Whether "Erlang/Elixir parked" should be removed or reworded now that the session layer has an Elixir conductor (13).
