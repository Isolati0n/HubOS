# Where the project stands (STATE.md)

**Written:** 2026-10-06, by the lead agent, after the merges of pull requests #68, #71, #69 and #70. **Purpose:** so that work can resume cleanly after a pause. This file points to the details; it does not copy them. `HUB-OS.md` is the source of truth and wins over this file; every decision is recorded in its Change log.

**Labels** (as everywhere): **TESTED** (a command was run and its output is in the file named), **SOURCE** (read in a file, named), **BELIEVED**, **UNKNOWN**.

---

## 1. What is merged, by area

Everything below is on `main`. "PR" numbers are GitHub pull requests of this repository. Details are in the file named, not here.

**The brief and the rules.** `HUB-OS.md` (the project brief), `CLAUDE.md` (rules for every session). PRs #54, #61, #68 and the round-8 docs PR rewrote them as the owner decided. No machine runs systemd (only the owner's dev-node experiments); no display passwords, no encryption on display connections, no request signing (only the image signature stays).

**The hub program, `hubd` (Go).** `cmd/hubd`, `internal/hub`, `internal/driftwm`, `internal/inventory`, `internal/probe`, `internal/viewers`. Inventory, health checks, the list and menu, open and end, record, saved layouts, direct restore after a compositor restart, the STALE marker when hubd is down, `hubd restart-desktop`. Details: `docs/hubd-slice2.md` (sections 17 and 18), `docs/inventory-format.md`. PRs #8 to #15, #60, #63.

**The image and update plumbing.** `tools/image/`, `image/`: A/B slots, per-slot kernels with stage 0, signed bundles, the confirm step and watchdog feeder, the failure counter, the recovery kernel, the recovery agent, the hub image (driftwm, Waybar, wofi, hubd under s6). Details: `docs/image.md`, `docs/proposals/phase-b-image.md`, `phase-b-desktop.md`. PRs #17, #20, #24, #27, #30 to #35, #39, #43, #46.

**The hub compositor, driftwm.** The pinned driftwm commit plus a patch set: `image/patches/driftwm/` (22 driftwm patches, 6 Smithay patches, `PINS`, `README.md`). Config checks at build time and the last-good fallback at run time; 14 protocols hidden; a restart policy that stops after 5 crashes in a minute and shows a fixed message. Details: `docs/proposals/driftwm-patches.md`, `docs/proposals/hub-stability.md`, `docs/proposals/bulletproof-compositor.md` (with `bulletproof-compositor/` scripts, results and patches). PRs #59, #64, #66, #67, #71.

**The recovery agent (prototype).** `tools/image/experiments/recoveryagent/`: plain unsigned requests; the install request still refuses anything that is not a correctly signed image bundle. Details: `docs/proposals/recovery-and-out-of-band.md`, `docs/proposals/node-helper-api.md`. PRs #48, #69. Last QEMU result: T18 and T19 PASS on the branch of #69 before the merge (see section 3, "Not re-run").

**The black box recorder (research and prototype).** `docs/proposals/black-box-recorder.md`, `tools/image/experiments/black-box/`. Decision: ramoops on, efi-pstore off. Not wired into stage 0 or the recovery kernel. PR #51.

**Display, clipboard, notifications, files.** `docs/proposals/remote-display.md`, `remote-display-benchmarks.md`, `remote-display-lod.md` (marked not planned), `docs/proposals/clipboard.md`, `notifications.md`, `file-transfer.md`, `driftwm-layouts.md`, `docs/viewers-research.md`. All are proposals or research, with test evidence; nothing of clipboard or notifications is built beyond a fake-RFB test script. PRs #38, #42, #50, #52, #56, #70.

**Machines and distros.** `docs/proposals/distro-contract.md`, `distro-workshop.md`, `from-scratch-hub.md`, `hub-isolation.md`, `init-comparison.md` (PAUSED), `language-comparison.md`, `systemd-libraries.md`, `secrets.md`, `hub-user-split.md` (NOT PLANNED, kept as history). Proposals only. PRs #19, #41, #55, #57, #58, #61, #62.

**Games and input.** `docs/games-research.md`, `games-research-2.md`, `docs/input-sharing-research.md`. Research only (the gaming box has no game window on the hub; all play is on its own monitors). PRs #22, #23, #26.

**Tools and hygiene.** `tools/check-push.sh` and its test `tools/checkpush/` (run before every push); `tools/fakenode`, `tools/geninv`; the benchmark harness `tools/bench/` (remote display, init). `.gitignore` entries for core dumps. PRs #50, #53, #65.

## 2. Decided

All decisions are in `HUB-OS.md`. Where to look:

- Who we are, the hub, the principles and their order: "What Hub OS is" and "Core principles".
- The cluster and the machines: "Physical setup", "Machine roles".
- The system rules for the hub, the reference images and the contract every machine meets: "The Hub OS system" with its subsections (stability, the contract every machine meets, the session layer, compositor decisions, run schedule).
- What the hub shows and does: "Hub behavior" (windows, canvas, panel, screen and bar, clipboard, notifications, phone access).
- What is out: "Explicitly out". What is not yet proven: "Unverified". What is still open: "Open questions for the owner".
- History: the Change log at the bottom.

## 3. PARKED (not started; each waits for the owner's go-ahead)

### 3.1 Part 6: the init comparison
- **What:** compare s6 with s6-rc, runit, dinit, OpenRC, the Go and Elixir "policy brains" and the custom PID 1 designs against the scorecard (reliability and integration high; boot speed and code size low; the hub never reboots for a service failure).
- **Plan decided by the owner:** an ELIMINATION ROUND by fault injection first, then a soak of at least 4 hours of ONLY the finalists. One long job at a time.
- **Where the paused work is:** `docs/proposals/init-comparison.md` (scorecard, status paragraph at the top, partial results for plain s6 only, the Go brain over s6-rc results) and `tools/bench/init/` (harness `hubsim`, candidate folders, prototypes, resume scripts). Only s6 was measured; no soak ran; sections 2 and 4 to 6 are not written.
- **It also decides:** where the hang rule lives (see section 4).

### 3.2 The session-layer prototype
- **What:** the hub's own viewer: one worker process per machine, a presenter, an Elixir/OTP conductor, hubd stays in Go. Direction and gates are in `HUB-OS.md` ("The session layer").
- **Stage 1:** the native-core bake-off (Rust with Kani, Ada with SPARK if it installs without root, Zig) against the gates (ten million fuzz cases per parser with zero crashes and hangs, a no-panic proof for the parser and frame copy, worst-case handoff stalls under a 20-session load, recovery times when any layer is killed or hung); the conductor; integration under the nested compositor.
- **Stage 2:** a separate 24-hour soak, run later.
- **Where the background is:** `docs/proposals/hub-isolation.md`, `hub-stability.md` (section on a session daemon), `clipboard.md`, `notifications.md`, `remote-display.md`.

### 3.3 Open questions waiting for the owner's answers
| Document | Questions open | Where |
|---|---|---|
| `docs/proposals/clipboard.md` | 16 | section 14 |
| `docs/proposals/notifications.md` | 16 | section 12 |
| `docs/proposals/file-transfer.md` | 13 | section 8 (the layouts questions of `driftwm-layouts.md` were answered) |
| `docs/proposals/from-scratch-hub.md` | 14 | section 6 |
| `docs/proposals/hub-isolation.md` | 10 | section 11 |
| `docs/proposals/distro-workshop.md` | 11 | section 10 |
| `docs/proposals/driftwm-patches.md` | answered in 11.12a | one item remains: what else to patch |

(`distro-contract.md` section 5, `bulletproof-compositor.md` section 12, `hub-stability.md` section 12 and `driftwm-layouts.md` section 9 were answered; see their "Owner decisions" sections.) The owner said the file-transfer and from-scratch questions will be answered later (an open question in `HUB-OS.md`).

### 3.4 December hardware (all hardware is bought at once)
- The AI box GPU (NVIDIA with CUDA or AMD with ROCm, decided from the generative-media software).
- The ultra-high-quality DAC for the hub's sound (research in December).
- The exact ECC memory modules for the hub (candidate board ASRock Rack B650D4U-2L2T/BCM with an EPYC 4005).
- The management switch.
- The rack and the power strip (also the out-of-band choice per machine: none, a PiKVM-class device, a relay or a switched power strip).
- The final projector, once independent text-sharpness reviews exist.
- Plus the real-hardware tests listed in `docs/proposals/bulletproof-compositor.md` (section 10, the December test plan), `hub-stability.md` (section 11) and `black-box-recorder.md` (what needs real hardware).

## 4. The build backlog (decided, not built)

Each item is decided by the owner; none is built yet. Do them ONE AT A TIME (see section 5).

1. `panic = "abort"` for the hub compositor build (`HUB-OS.md`, Compositor decisions).
2. A compositor IPC field with the true window stacking order (today the IPC list puts the focused window first, so restore is not exact for the top two windows after a Move; `driftwm-patches.md` 11).
3. Stacking order saved in layouts as an optional field (`hubd`, `driftwm-layouts.md`).
4. The bar alert for the compositor's config fallback, through hubd's feed (today it goes to the serial console, the service log and `/run/hub/driftwm-config-fallback`).
5. The hang rule (kill after 4 failed rounds and 30 s with both probes failing; at most 3 kills in 10 minutes): NOT in the code today; it waits for Part 6 to decide where it lives.
6. BC-9 (client buffers cost about 8 times their size and nothing limits a client; the owner chose the memory limit for viewers instead of a per-client limit), BC-14 (an abort on a failed allocation under a 6 GB ceiling; no minimal reproduction), the Smithay audit of client-input paths, the nightly fuzz job and extra test machines, and the 72-hour soak: scheduled later, one at a time (`bulletproof-compositor.md`).
7. The init-neutral "service up/down" command for the node helper and the distro contract (`distro-contract.md`, `node-helper-api.md`).
8. The Go rewrite of the update tool `hubos-ctl` as one static program (`distro-contract.md`, decision Q8).

**Other known gaps found while working (not owner-listed; each is in the document named):**
- The hub kernel has no control groups and no pressure information, so the owner's resource-fence decisions (viewers in a limited group, reserved CPU) cannot be implemented until the kernel is rebuilt with those options (`hub-isolation.md`).
- The hub's stage-1 script still SOURCES `node.conf`; the owner decided it is parsed, never sourced (`distro-contract.md`, Q10). Not done.
- `hubd serve` does not set the viewer-log limits that `docs/hubd-slice2.md` section 2.5 describes (`hubd-slice2.md` 18.11, question 7).
- The boot confirm step asks the hub with five `hubd list` calls plus a record of the compositor's answer; it does not yet "ask the init for health" as the owner decided (`init-comparison.md`).
- **Not re-run after the last merges:** the QEMU tests (TestHubImage and TestImage) were last run on the branches of #71 (TestHubImage runs 3 and 4 PASS, TestImage PASS) and #69 (TestImage PASS with the unsigned T18), separately. The merged `main` was checked only with `gofmt`, `go vet` and `go test`. The next QEMU run on `main` is the first test of the combination (UNKNOWN until run).

## 5. The run rules

- **Long jobs never overlap.** One long job (a QEMU test, an image build, a soak or a fuzz run) at a time. Before starting one, check with `pgrep -a` for another (qemu, go test, image.test, mmdebstrap, build scripts, soak, fuzz) and wait if one is running. Never kill another job's processes unless the owner says so.
- **Push early.** Push a draft pull request after each finished piece and at least every 2 hours, because a stopped session loses everything that is not pushed. Run `tools/check-push.sh` (after `git fetch origin`) before EVERY push.
- **Security.** Never commit a core dump, an environment dump or anything that could contain a credential; `ulimit -c 0` in every shell; never name or fetch the leaked core-dump commit.
- **Deleting.** Before any `rm -rf` run `findmnt` and check for mounts under the path; use `--one-file-system`; never touch `/root/.cargo` or `/root/.rustup`.
- **History.** New commits only; no amend, no rebase, no force-push. A conflict in the Change log is solved by merging `origin/main` into the branch and keeping both lines.
- **Packages.** Install no packages on the machine: unpack with `dpkg -x` into a temporary directory and delete it afterwards.
- **Labels and honesty.** Label every item TESTED, SOURCE, BELIEVED or UNKNOWN; say what a helper agent wrote; ask the owner about every design question not already answered.
- **Do not start parked items** (section 3) without the owner's go-ahead.

## 6. How to resume: which prompt to send

Send the owner's prompt in this form (start with "Read CLAUDE.md and HUB-OS.md first.", then this file). Examples; the owner may change the wording.

- **Part 6 (the init comparison):** "Read CLAUDE.md, HUB-OS.md and docs/STATE.md first. Resume the paused init comparison (docs/proposals/init-comparison.md, tools/bench/init/): finish the scorecard measurements with an elimination round by fault injection first, then soak only the finalists for at least 4 hours each, one long job at a time; open a draft PR early and push at least every 2 hours. Do not write an init."
- **Session-layer prototype, stage 1:** "Read ... STATE.md first. Start stage 1 of the session layer (HUB-OS.md, The session layer): the native-core bake-off with the stated gates, the Elixir conductor, and integration under the nested compositor. No 24-hour soak yet."
- **Session-layer prototype, stage 2:** "Read ... STATE.md first. Run the 24-hour soak of the stage 1 prototype now (nothing else may run)."
- **Answering open questions:** "Read ... STATE.md first. My answers to docs/proposals/clipboard.md section 14: 1 ..., 2 ... (and so on)." Then ask for the answers to be recorded in `HUB-OS.md` and in the document's "Owner decisions" section, with a Change log line.
- **A backlog item:** "Read ... STATE.md first. Build backlog item N (the compositor IPC stacking-order field), on a new branch, with tests; run TestHubImage twice and TestImage once when the machine is free."
- **A QEMU run on `main`:** "Read ... STATE.md first. Run TestHubImage twice and TestImage once on main now and report every full RESULTS block; nothing else may run."
- **December:** "Read ... STATE.md first. Here is the hardware that arrived: ... Prepare the December test plan (bulletproof-compositor.md section 10, hub-stability.md section 11) for it."
