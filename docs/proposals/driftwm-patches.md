# The driftwm patch set in the hub image (what was built, how it is applied, how it was tested)

**Written:** 2026-10-05. **Status:** built and tested in virtual machines and on a nested compositor; nothing here was run on real hardware. This document says what was *done*; the reasons for the patches are in `docs/proposals/hub-stability.md` (section 5, "Patch set"). Every open choice is in section 10.

Labels, as everywhere: **TESTED** (I ran it, command and output given), **SOURCE** (I read it in a file; path given; read 2026-10-05), **BELIEVED** (my reasoning, not tested), **UNKNOWN**. No helper agents were used. Everything below I ran or read myself.

## 1. In plain words

The hub's compositor (driftwm, the program that draws the windows) is now built from the pinned source **plus a set of small patches** that remove the crashes and freezes found in the stability research. The image build applies the patches itself and **stops with an error if any patch does not fit** the pinned source. The compositor also gets a small settings file (no shadows, no rounded corners, no borders), and its service now has a **restart rule**: after 5 crashes within one minute it stays down, the screen shows a fixed message, the hub's control program (hubd) and the recovery terminal keep running, and the machine does not reboot.

## 2. The two pins

| What | Value | Label |
|---|---|---|
| driftwm | commit `352333a8fa1b22171492d4b71a54102045c9a19d` (the pin of `HUB-OS.md`) | SOURCE `HUB-OS.md`, `tools/image/build-hub-parts.sh` (before this change) |
| Smithay (the library under driftwm) | revision `4cf0b62028039661477d482ec4758b687d8f4392` | SOURCE: `Cargo.lock` of driftwm at the pinned commit, line 2109: `source = "git+https://github.com/Smithay/smithay.git?rev=4cf0b62028039661477d482ec4758b687d8f4392#4cf0b620..."`, read 2026-10-05 |

Both are written in `image/patches/driftwm/PINS`. The build checks that driftwm's own `Cargo.lock` still names that Smithay revision and **fails** if not. TESTED (section 6.1).

## 3. The patches

Files: `image/patches/driftwm/0*.patch` (driftwm, applied in name order), `image/patches/driftwm/smithay/0006-p6-shm-pool-resize-zero.patch` (Smithay), `image/patches/driftwm/not-applied/0002-p2-test-hooks.patch` (never put in an image). P1 to P6 come from the research branch (`docs/proposals/hub-stability/driftwm-hub-stability.patch` and `smithay-p6.patch`); I split the combined file into one file per P-number (the contents are the same, except P1 which also updates driftwm's `docs/cli.md`, see below). P7 to P10 I wrote in this round, from the one-line definitions in `docs/proposals/hub-stability.md` section 5.9.

| # | What it does (plain words) | Size (lines added / removed) | In the image? | Test (section 6) |
|---|---|---|---|---|
| P1 | driftwm can adopt a Wayland socket opened by a parent process (`--socket NAME --wayland-fd FD`). Without those options it behaves as before. The hub's service does **not** use the options. | +75 / -16 (including `docs/cli.md`) | yes | not re-tested here (TESTED earlier: hub-stability T5); every test below runs the patched binary without the options |
| P2 | Test hooks that hang or panic the compositor on purpose (`hs-test-busyloop`, `hs-test-deadlock`, `hs-test-panic`). | +17 / -0 | **no** (`not-applied/`) | used only to build a test binary for the "panic while the log pipe is full" case |
| P3 | Catches a panic in the Wayland dispatch and in each IPC connection, **only if** the environment variable `HS_CATCH_PANICS` is set. | +29 / -3 | yes, but **switched off**: the hub service does not set the variable (question 2) | not tested here |
| P4 | A colour with a non-ASCII letter in the config file no longer panics (at start, in `--check-config`, or when the running compositor reloads its config). | +8 / -1 | yes | TESTED (6.2, `config`) |
| P5 | `output_state()` uses the data of a poisoned lock instead of panicking. | +3 / -1 | yes | not specifically tested (as in the research) |
| P6 | **Smithay:** `wl_shm_pool.resize(0)` posts the protocol error and returns (one line: `return;`) instead of panicking. | +1 / -0 | yes (Smithay copy, see 4) | TESTED (6.2, `shm`) |
| P7 | Log lines go into a queue and a separate thread writes them, so a log reader that stops reading cannot freeze the compositor. | +157 / -0 | yes | TESTED (6.2, `pipe`, `pipepanic`; unit test) |
| P8 | The start-up helper programs are waited for at most 3 seconds, then killed; the environment variables whose values are known at start are set before the first thread starts. | +79 / -16 | yes | TESTED (6.2, `startup`; unit tests) |
| P9 | If the built-in default background shader does not compile, the background is flat (driftwm's existing "no background" mode) instead of a panic. | +25 / -5 | yes | TESTED (6.2, `badshader`, with a deliberately broken shader) |
| P10 | If every frame of an output fails for 2 seconds in a row, log once and exit with status 70, so the service restarts the compositor. | +103 / -0 | yes | unit tests only (6.3). The real path (a lost GPU) is **UNKNOWN** |

**What I implemented for P7 to P10, and where the document is not exact** (all in question list, section 10):

- **P7.** The document says "a log writer that cannot block the loop (or `s6-log`), 5 lines or config". I wrote a writer of my own with the standard library only (no new crate): the compositor thread does `try_send` into a queue of 4096 lines and never waits; one thread writes to a private copy of file descriptor 1. When the queue is full, the line is dropped and counted, and the next line that does get written says how many were lost. I did not use `tracing-appender` (a new dependency) or `s6-log` (the compositor is started by a service without a log service, and `s6-log` reading its pipe would not remove the problem if it stopped). Two facts the document does not state: (a) **driftwm's log lines go to standard output, not standard error** (TESTED: stdout 13,522 bytes and stderr 192 bytes in 4 seconds of start-up, for both the unpatched and the patched binary; `tracing_subscriber::fmt()` writes to stdout by default); the hub's service joins the two with `exec 2>&1` so there is no difference there. (b) The default panic message is written to standard error by the thread that panics, and would block on a full pipe, turning a crash into a freeze; so P7 also replaces the panic hook by one that logs the panic through the same queue and waits at most 0.5 s for it to be written. This second part is my addition, and the research did not ask for it.
- **P8.** The document says "remove or time-box the start-up shell-outs; set the environment before threads start". I **time-boxed** (3 s, then kill) the three places that wait for a child on the main thread before the event loop runs: the environment import in `main.rs`, the xwayland-satellite probe and the `DISPLAY` import in `xwayland.rs`. I did not remove them. For the environment: `DRIFTWM_CONFIG` and the four fixed `XDG_*` variables are now set at the top of `main`, before the log thread starts. **`WAYLAND_DISPLAY` is still set after threads exist** (its value is the socket name, and the graphics backend must not see it, see the comment in `main.rs`), so the race the research called F7 is smaller, not gone (BELIEVED tiny).
- **P9.** "Flat background" is driftwm's own `type = "none"` background (no background element; what shows is the renderer's clear colour, black `[0,0,0,1]`: SOURCE `src/backend/udev.rs`, `render_frame`). I chose it because it already exists and needs no GPU shader. Two places were changed (`init_shader_bg` and `init_default_shader_bg`).
- **P10.** The document says "if `render_frame` fails N frames in a row (for example 120, about 2 s) ... `exit(70)`". I used **time** (2 s from the first failure of an unbroken run, and at least 2 failures), per output. A good frame or an empty frame ends the run. Because driftwm does not retry a failed frame by itself (it only arms a timer; the frame is retried when something asks for a redraw), I also **ask for a redraw after every failed frame**, so the failures continue at the refresh rate and the 2 seconds can pass on an idle desktop. Only the real display path (`udev.rs`) is changed; the nested `winit` path is not. The exit status is 70 and no clean-up runs (`std::process::exit`); the log is flushed first (0.5 s at most).

**A side effect of P1 that the research had not seen.** driftwm has a test (`docs_cli_md_is_up_to_date`) that fails when `docs/cli.md` does not list every command-line option. P1 adds two options, so with P1 alone one of driftwm's own tests fails. TESTED: `cargo test --release` on the patched tree failed that one test (1,349 passed, 1 failed, 13 ignored in the binary; 327 passed in the library). I regenerated the file with the test's own command (`UPDATE_CLI_DOCS=1 cargo test docs_cli_md_is_up_to_date`) and put the change into the P1 patch; the test then passes (TESTED).

## 4. How the patches are applied in the image build

`tools/image/build-hub-parts.sh` (SOURCE: the file; TESTED by the runs in section 7):

1. Reads `image/patches/driftwm/PINS`. A **stamp** (a hash of PINS and all patch files) is kept next to the built binary; a change to any patch or pin rebuilds driftwm (before this change the build was skipped whenever a binary existed).
2. Checks out driftwm at the pinned commit, resets the folder to that commit and removes anything else (so an earlier build cannot leave patched files behind).
3. Checks that driftwm's `Cargo.lock` pins the Smithay revision written in PINS.
4. Clones Smithay, checks out that revision, resets it.
5. Applies every `0*.patch` to driftwm and every `smithay/0*.patch` to Smithay with `git apply` (strict: no fuzz, no offset guessing). **If one does not apply, the script prints which patch and exits with status 1.**
6. Adds a `[patch."https://github.com/Smithay/smithay.git"]` section to driftwm's `Cargo.toml` that points at the patched Smithay copy, and builds `cargo build --release` in the build root.
7. Checks that the build log says `Compiling smithay ... (/smithay)` (the binary really was built from the patched copy), copies the binary, writes the stamp.

TESTED (a copy of the script with a deliberately broken patch; the real build root replaced by an empty folder because the failure comes before it is used):

```
== A: a driftwm patch whose context does not match
exit status: 1
error: src/config/parse_helpers.rs: patch does not apply
driftwm patch 0004-p4-config-parser.patch does not apply to 352333a8fa1b22171492d4b71a54102045c9a19d
== B: a Smithay patch that does not apply
exit status: 1
error: src/wayland/shm/handlers.rs: patch does not apply
Smithay patch 0006-p6-shm-pool-resize-zero.patch does not apply to 4cf0b62028039661477d482ec4758b687d8f4392
== C: the Smithay pin disagrees with driftwm's Cargo.lock
exit status: 1
driftwm's Cargo.lock does not pin Smithay 0000000000000000000000000000000000000000 (see image/patches/driftwm/PINS)
```

The build with all patches took 816 s in a build root on a loaded machine (the research said about 4 minutes on an idle one); it logged `applied driftwm patch ...` for the eight driftwm patches and `applied Smithay patch 0006-...` (TESTED, `/tmp` log of the first build; the same lines appear in the image-test logs).

## 5. The compositor's settings, and the restart rule

### 5.1 Settings file: `/etc/hubos/driftwm.toml`

Part of the hub image (`image/machines/hub/rootfs/etc/hubos/driftwm.toml`), passed with `--config` in the service. Content: `[decorations]` with `shadow = false`, `corner_radius = 0`, `border_width = 0`.

- **The keys exist.** SOURCE: driftwm `config.reference.toml` lines 258 to 291 (`corner_radius`, `shadow`, `border_width` under `[decorations]`) and `src/config/toml.rs` (`Option<bool>`, `Option<i32>`). The compositor accepts the file: TESTED in the image (H1 passes with `--config /etc/hubos/driftwm.toml`; a warning would show in the compositor's log and on the screen). Per-window rules (`[[window_rules]]`) can override these keys; the file has no rules.
- **Hot reload cannot be switched off by a key. Said plainly: there is no such key.** driftwm always watches the folder of its config file with inotify and reloads when the file changes (SOURCE `src/main.rs`, the block after the comment "inotify watch instead of an mtime poll"), and has a `reload-config` action (SOURCE `config.reference.toml:427`; no default key is bound to it, SOURCE `grep` of `src/config`). What I did instead: the file is in the **read-only** image, so nothing changes it while the hub runs; a new settings file arrives only with a new image, which restarts the compositor. BELIEVED (squashfs root, `ro`); not tested that an edit attempt fails. A real switch would need a patch (question 8).

### 5.2 Restart rule (owner decision): files in `image/machines/hub/rootfs/etc/s6/sv/driftwm/` and `usr/lib/hubos/follow-driftwm`

- **`finish`** (new) runs after every end of the compositor. A clean exit (status 0) is not a crash. Any other end (a panic, `kill -9`, P10's status 70, ...) is written with its time into `/run/hub/driftwm-crashes` (in RAM); entries older than 60 s are dropped. On the **fifth** crash within 60 s it writes the marker `/run/hub/driftwm-gave-up`, prints the fixed message on the screen (`/dev/tty0`) and on the serial console, and runs `s6-svc -d .` so s6 leaves the compositor down. It never reboots anything.
- The message (fixed text): `HUB OS: the desktop (driftwm) crashed 5 times within one minute and was stopped.` / `It will not start again by itself, and the machine will NOT reboot.` / `hubd and the recovery terminal are still running.` / `To try the desktop again by hand: s6-svc -u /run/service/driftwm`.
- **`run`** (changed): `--config /etc/hubos/driftwm.toml`; if the marker exists when it starts (someone started it by hand) it removes the marker and the crash record.
- **hubd stays up.** Before, `follow-driftwm` stopped hubd whenever the compositor was gone and waited for its return. Now hubd's service sets `FOLLOW_GAVEUP=run`: when the marker exists, `follow-driftwm` starts hubd **without** the compositor and keeps it running; if the compositor is started by hand again (the marker disappears) it stops hubd so that s6 starts it again in the normal way. Waybar still follows the compositor (it is down while the compositor is down).
- **The recovery terminal** is the existing `console` service on the serial port; nothing in this change touches it.
- **Something the decision implies and I had to handle (question 10).** The confirm step treats a boot as healthy when hubd answers. With hubd now alive without the compositor, a *trial boot* of a new release whose desktop crash-loops would have been confirmed. So `confirm/run` (shared by all images) now does not count hubd as answering while the marker exists; such a trial boot times out and rolls back (the one reboot the project allows: `HUB-OS.md`, "rollback of a never-healthy trial boot"). The qemu-test image has no marker, so nothing changes there.
- **Message on the screen needs a text console.** The hub kernel had no text console (no `CONFIG_VT`, no framebuffer console), so there was no way to put text on the screen when the compositor is dead. I added six lines to `image/kernel/hub.frag` (`CONFIG_VT`, `CONFIG_FB`, `CONFIG_DRM_FBDEV_EMULATION`, `CONFIG_FRAMEBUFFER_CONSOLE`, `CONFIG_FONTS`, `CONFIG_FONT_8x16`). It is **not** the system console (`CONFIG_VT_CONSOLE` stays off; kernel messages stay on the serial port). The recovery kernel uses the same fragment. This is a change to the kernel, and it is in question 9.

Test: logic of `finish` TESTED on this machine with a fake `s6-svc`: crashes 1 to 4 leave no marker and make no `s6-svc` call; crash 5 writes the marker and calls `s6-svc -d .` once; a clean exit is not counted; old entries (more than 60 s) age out. In the virtual machine: test H6 (section 7).

## 6. Tests on the compositor itself

All on this cloud machine, nested (driftwm's `winit` backend inside Xvfb, software OpenGL), no real GPU. Scripts: `tools/image/experiments/driftwm-patches/` (`run-all.sh`, `repros.py`, `repro_shm.py`, `xvfb-run-ns.sh`), using the same layout as `docs/proposals/hub-stability/scripts/` (`HS_WORK`, with the runtime libraries, `foot` and Xvfb unpacked with `dpkg -x` under `HS_WORK/A/root`, nothing installed). The binaries: **unpatched** = driftwm at the pinned commit and Smithay `4cf0b62`, release build; **patched** = the same plus all patches except P2, built by the image build script's method (see 4); **+hooks** = also with P2; **+badshader** = with `src/shaders/dot_grid.glsl` replaced by one line of garbage (test builds only, never in an image). No core files: `ulimit -c 0` everywhere, everything under `/tmp`.

### 6.1 Build checks

See section 4. TESTED: three failure cases, all exit 1.

### 6.2 The reproductions, before and after

Command: `HS_WORK=... run-all.sh UNPATCHED PATCHED UNPATCHED+HOOKS PATCHED+HOOKS UNPATCHED+BADSHADER PATCHED+BADSHADER`. The verdict lines (TESTED, 2026-10-05):

| Reproduction | Unpatched | Patched |
|---|---|---|
| `wl_shm_pool.resize(0)` from a client (`shm`) | **compositor DIED, exit status 101** (panic at Smithay `shm/handlers.rs:190`) | **survived**, IPC answers |
| Non-ASCII colour (`"#aéaaa"`) in the config (`config`): `--check-config`, and the file saved into a running compositor | **KILLED**: `--check-config` exit 101, running compositor exit 101 (panic at `parse_helpers.rs:142`) | **survived**: `--check-config` exit 0 with "Config OK, 1 warning(s)", running compositor alive and answering |
| The log pipe is full and nobody reads it (`pipe`; pipe pre-filled with 64 KiB, `RUST_LOG=trace`, 12 windows) | **FROZEN**: the process sleeps in `anon_pipe_write`, 0 of 5 IPC requests answered (it did not even get its IPC socket up) | **not frozen**: 5 of 5 IPC requests answered in 0 to 30 ms, pipe still full (65,536 bytes) |
| A panic while the log pipe is full (`pipepanic`, with the P2 test hook) | already frozen by the pipe before a panic could be sent | the panic **ended the process** with status 101 after 0.6 s (instead of hanging in the panic message) |
| A start-up helper that hangs (`startup`; a fake `dbus-update-activation-environment` that sleeps 60 s) | **BLOCKED**: no IPC after 25 s | **started in 3.3 s**, logged `Environment import took longer than 3 s; killed it` |
| The built-in default shader does not compile (`badshader`) | **DIED**, exit 101: `Default shader must compile: ShaderCompileError` (`background.rs:748`) | **survived** with a flat background, IPC answers; logged `Default background shader failed to compile` |
| **A blocked session write** (`session`: the session file's temporary name is a FIFO nobody reads, the stand-in for a hung disk, as hub-stability T12) | **FROZEN** (`wait_for_partner`, 0 of 4 answered) | **STILL FROZEN** (`wait_for_partner`, 0 of 4 answered) |

**The last row is a failure of this task's goal, said plainly.** No patch P1 to P10 touches the session or state file writes, which `docs/proposals/hub-stability.md` section 3.2 / F5 lists as a separate problem with a different cure: keep every file the compositor writes in RAM, which the hub image already does (`XDG_STATE_HOME=/run/hub/state`, a tmpfs: SOURCE `image/machines/hub/rootfs/usr/lib/hubos/hub-env.sh`). A write to a tmpfs cannot block on a disk. So on the hub the "blocked session write" cannot happen (BELIEVED, not tested: no test makes a tmpfs write block), but the patched compositor itself is **not** protected if the state folder were on a hung disk. A real fix (write on a worker thread) is a new patch; question 7.

The `pipepanic` "before" is weak: the unpatched binary was already stuck in the pipe write at start-up (the pipe was full from its first log line), so it never reached the panic. A panic with a pipe that fills *later* was not tested on the unpatched binary.

### 6.3 Unit tests

Command (inside the build root, patched tree): `cargo test --release`. TESTED: library 327 passed; binary 1,349 passed, **1 failed** (the `docs/cli.md` test, see section 3), 13 ignored; the new tests all pass: `backend::frame_health::tests::{one_failure_never_exits, failures_over_two_seconds_exit, a_good_frame_resets_the_run}` (P10's rule with an injected clock), `bounded::tests::{a_quick_child_finishes, a_hanging_child_is_killed_at_the_limit}` (P8), `logwriter::tests::a_full_queue_drops_lines_and_never_blocks` (P7). After regenerating `docs/cli.md`: `cargo test --release docs_cli_md_is_up_to_date` passed (TESTED). I did not rerun the whole suite after that one-file change.

**What is not tested for P10:** the real path. A frame that fails on a real GPU after a reset, a monitor that is switched off or unplugged (BELIEVED to produce no failed frames, but I do not know), and what a restart looks like on real hardware are **UNKNOWN**. The restart rule of 5.2 bounds a restart loop at 5 in a minute.

### 6.4 Short runs of the stability fuzzer and soak

Scripts of `docs/proposals/hub-stability/scripts/` (`fuzzrun.sh`, `fuzz.py`, `healthcheck.py`, `soak.py`, `soakstat.py`), unchanged except that for `soak.py` the display number was changed from `:80` to `:81` on my private copy (another helper was using `:80`; Xvfb is started the way `xvfb-run-ns.sh` does). Short runs only:

- **Fuzz, 120 s each, patched binary, seeds 20 and 31:** both finished with the compositor alive and the health check passing (IPC answered, a new `foot` window mapped, a `Move` applied): 294 connections and about 53,000 requests (seed 20), 294 connections and about 50,000 requests (seed 31); memory 143 MB. The **unpatched** binary with seed 20 also survived (50,000 requests): with my protocol set (68 XML files: the Wayland core, `wayland-protocols` from Ubuntu 24.04 and the wlroots protocols from the current `master` of `wlr-protocols` on gitlab, unpinned; the research used 72) the random requests differ, so seed 20 no longer reproduces the research's shm crash. The fuzzer therefore does not show a before/after here; the `shm` row of 6.2 does.
- **Soak, 180 s, patched binary, seed 7:** 377 operations (open, close, crash, move, resize, focus, pan, zoom, bookmark, suspend, relaunch of up to 20 windows), 0 compositor restarts, 0 failed IPC probes, memory 131 to 219 MB, 69 file descriptors on average (92 at most), IPC median 0.5 ms, 26% of one core. This says nothing about growth over hours (the research soak ran for hours: `docs/proposals/hub-stability.md`).

## 7. Tests in the hub image (virtual machines)

Command (from the repository root, as in `docs/image.md`): `go test -tags qemu -count=1 -timeout 150m -v -run 'TestHubImage$' ./tools/image`. Each run builds everything from nothing (tools, kernels, base root, driftwm with the patch set, eudev, three bundles) and boots the hub image in QEMU (TCG, no KVM, virtio GPU, software OpenGL). The complete RESULTS blocks of every run are in the report that came with this change, and are not copied here.

**New in this round:** test **H6** (`tools/image/hub_test.go`): kill -9 the compositor five times, each time as soon as a new compositor process exists; 25 s later check that no compositor runs, that the marker exists and `s6-svstat` says `down`, that hubd still answers `hubd list` and has a process, that the screenshot shows the message (light pixels in the top left, the bar gone), and that the boot id is unchanged (no reboot); wait another 30 s and check that the compositor is still down and the message was also printed on the serial console; then `s6-svc -u /run/service/driftwm` and check that the desktop and bar come back, the marker is gone and hubd answers. The screenshot (not stored in the repository) shows white text on a black screen, four lines (TESTED, I looked at the PNG of run A).

| Run | Result | Notes |
|---|---|---|
| `TestHubImage` run A | **FAIL**: H0, H1, H2, H3, H3b, H3c, H6 passed; **H4 and H5 failed** (2372 s) | One **QEMU hang** in the reboot into the trial boot of slot b (H4): the serial log stopped after `HUBOS: booted entry 0008 label 'hubos-b'` and the line `HUBOS: handing over to s6-svscan` did not come in 150 s. The runner's rule (retry the step once) started the machine again; BELIEVED (the log shows the result, not the reason) the trial boot (`BootNext`) was already used up, so the machine came back on slot a release 1 and H4's check "now slot b release 2" failed; H5 failed because it started from the wrong state (its note shows `slot  release ,` empty: `hubos-ctl status` timed out, then the update went to slot b release 3 from slot a). It is the first real hang in these tests (`docs/image.md` section 3.8 had none). **The cause is UNKNOWN.** The host was busy (load average 8 to 13 from other sessions) which BELIEVED makes a timeout more likely; BELIEVED not caused by this change but NOT shown: this hub kernel is the first one with a text console, and I did not run the old kernel for comparison. The serial log of the hang was deleted with the temporary folder at the end of the run. |
| `TestHubImage` run B | **PASS**, all 9 sub-tests (2950 s), 0 hangs, 0 crashes | H4 196.8 s, H5 199.2 s, H6 85.4 s |
| `TestHubImage` run D (a third run, made because run A failed) | **PASS**, all 9 sub-tests (2212 s), 0 hangs, **1 QEMU crash** (signal 11 in the first boot, the known QEMU 8.2.2 TCG crash) retried by the runner as designed | H4 179.9 s, H5 233.1 s, H6 87.6 s |

Two earlier attempts at a run stopped for reasons of the environment, not of the code, and are not counted: one because I had deleted `/root/.cargo` (section 9), one because the machine's disk was full (`No space left on device` while compiling driftwm; the disk is shared with other sessions; I freed my own folders and started again).

H6 passed in all three runs (86.3 s, 85.4 s, 87.6 s). In run B: compositors 0 after 25 s and after 55 s, `s6-svstat` = `down (signal SIGKILL) 25 seconds, normally up, ready 24 seconds`, marker written, 5 lines in the crash record, hubd answered (pid 1154), same boot id, message on the serial console, `giving up` line printed, 6,210 light pixels in the top left of the screen, 0 bar pixels, and after `s6-svc -u`: desktop back, marker gone, hubd answering.

`TestImage` (the qemu-test image, which shares `confirm/run` and the kernel build with the hub; command `go test -tags qemu -count=1 -timeout 150m -v -run 'TestImage$' ./tools/image`): **PASS**, all sub-tests, 3,790.65 s, 0 QEMU hangs, 0 QEMU crashes (TESTED, one run). The qemu-test kernel fragment and rootfs service set are not changed by this work; the one shared change is the marker test in `confirm/run`, which never triggers there.

Plain Go checks on the final tree (TESTED): `gofmt -l .` printed nothing; `go vet ./...` printed nothing; `go test -count=1 ./...` all `ok` (the full output is in the report). `go vet -tags qemu ./tools/image/` printed nothing.

## 8. What was not verified

- Anything on real hardware: a real GPU, a real screen (the crash message is shown on a virtual screen only), real input.
- P3 (switched off), P5, and the socket handover P1 were not exercised here.
- P10 on a real GPU failure (6.3).
- A long soak or long fuzz of the patched build; only the short runs of 6.4.
- That the settings file's keys have the visual effect (no shadows, corners, borders): the file is accepted; I did not compare screenshots with and without it.
- The hub image with a real (non-software) GPU driver: the hub image test uses software drawing.

## 9. Mistake made while working (said plainly)

While cleaning up scratch folders I ran `rm -rf` on a folder that still had three of my own bind mounts left over from earlier builds (a stale mount of `~/.cargo`, `~/.rustup` and a source folder). That deleted the **contents of `/root/.cargo` and `/root/.rustup` on this machine** (the Rust toolchain and its crate caches). One image-test run in progress failed because of it (`cargo: not found`); it was started again. I reinstalled the same toolchain (rustup, `1.97.0`, the same `cargo 1.97.0 (c980f4866 2026-06-30)` as before) into the same two folders; the crate and git caches are empty and refill on the next build (needs network). Any other session that used `/root/.cargo` between about 21:39 and 21:42 UTC may have seen errors. No repository file was affected.

## 10. Questions for the owner

1. **P2 (the test hooks) stays out of every image.** The research says "never for production", the task said "apply all patches". I kept P2 in `not-applied/`. Right?
2. **P3 (catch a panic and go on) is in the binary but switched off.** The research says a caught panic can leave poisoned locks and that a counter ("exit after the first or second caught panic") must go with it; that counter is not one of P1 to P10 and I did not write it. Leave it off, or define the counter?
3. **P7.** Own small writer instead of `tracing-appender` or `s6-log`; lines are dropped (and counted) when the queue is full; I also replaced the panic hook. Acceptable? (And the log goes to standard output, not standard error as the research text says.)
4. **P8.** I time-boxed the three start-up helper calls at 3 s instead of removing them (the hub has no systemd, so they do nothing useful there). Remove instead? `WAYLAND_DISPLAY` is still set after threads exist.
5. **P9.** "Flat background" = driftwm's no-background mode (black). A particular colour?
6. **P10.** The 2 s are time, not 120 frames; I also ask for a redraw after each failed frame so the failures continue on an idle desktop; only the real display path is covered. OK?
7. **Blocked session write.** No patch of P1 to P10 covers it (section 6.2). The hub avoids it by keeping the state folder in RAM. Do you want a new patch P11 (write the session and state files on a worker thread) or is the RAM folder enough?
8. **Hot reload.** driftwm has no setting for it. I relied on the read-only config file in the image. Do you want a patch that removes the inotify watch and the `reload-config` action?
9. **Kernel change.** To show the crash message on the screen I added a text console (virtual terminal, framebuffer console, DRM fbdev emulation, one font) to the hub kernel fragment. It also grows the recovery kernel slightly. OK, or do you prefer another way to put text on the screen (for example a tiny program that draws to the framebuffer)?
10. **Confirm step and the marker.** To keep a crash-looping trial release from being confirmed, the confirm step now refuses to count hubd as healthy while the "desktop was stopped" marker exists, so that trial boot rolls back after the confirm timeout. This is a reboot (the allowed one). Is that what you want?
11. **What counts as a crash.** Any end of the compositor other than exit status 0 (including P10's status 70, and a `kill -9` as in the tests), five of them within 60 seconds. Right? And should a later successful long run clear the record (today the record is only a sliding 60-second window, which clears itself)?
12. **The message** is the four lines in 5.2, on the kernel console of the screen and on the serial console. Wording? Should Waybar's bar also show an alert (it is down with the compositor)?
13. **After the stop, hubd runs without the compositor,** so it cannot open windows; its health checks and the confirm step still work. OK?
