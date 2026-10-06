# The driftwm patch set in the hub image (what was built, how it is applied, how it was tested)

**Written:** 2026-10-05. **Status:** built and tested in virtual machines and on a nested compositor; nothing here was run on real hardware. **Updated 2026-10-06** after the owner's answers (section 10): P8 now removes the helper calls, and P11 and P12 are new. **Updated again 2026-10-06 (section 11):** the research patches (Smithay P7 to P11, driftwm D1 to D5) are in the image, with the fixes for BC-11 to BC-13, config checks, hidden protocols and the stacking-order restore; see section 11 first for what is current. This document says what was *done*; the reasons for the patches are in `docs/proposals/hub-stability.md` (section 5, "Patch set"). Every open choice is in section 10.

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
| P8 | **The start-up helper calls are removed** (`systemctl --user import-environment` and `dbus-update-activation-environment`, in `main.rs` and for `DISPLAY` in `xwayland.rs`); the xwayland-satellite probe is waited for at most 3 seconds, then killed; the environment variables whose values are known at start are set before the first thread starts. | +100 / -33 (see the file) | yes | TESTED (6.2, `startup`; unit tests) |
| P9 | If the built-in default background shader does not compile, the background is flat (driftwm's existing "no background" mode) instead of a panic. | +25 / -5 | yes | TESTED (6.2, `badshader`, with a deliberately broken shader) |
| P10 | If every frame of an output fails for 2 seconds in a row, log once and exit with status 70, so the service restarts the compositor. | +103 / -0 | yes | unit tests only (6.3). The real path (a lost GPU) is **UNKNOWN** |
| P11 | **New.** The session file and the runtime state file are written by a worker thread (`bgwrite.rs`), never on the event loop. While the worker is stuck, later writes of the same file replace the waiting one (memory does not grow). The IPC `state` reply gains two numbers, `failed_writes` and `pending_writes` (see below). The compositor's own tests switch the writer to inline mode. | +380 (file `0011-...`) | yes | TESTED (6.2, `session`, `sessionfail`; unit tests) |
| P12 | **New** (number chosen by me). Config hot reload is removed: the inotify watch on the config file's folder is deleted, the `reload-config` action is refused by the parser, and the dead code behind it (the `Action::ReloadConfig` variant and its uses, the file-reading `reload_config` function) is removed. Its line is removed from the config reference (a driftwm test requires every documented action to exist). `reload_config_from_contents` stays (tests use it). | +190 (file `0012-...`) | yes | TESTED (6.2, `reload`) |

**What I implemented for P7 to P10, and where the document is not exact** (all in question list, section 10):

- **P7.** The document says "a log writer that cannot block the loop (or `s6-log`), 5 lines or config". I wrote a writer of my own with the standard library only (no new crate): the compositor thread does `try_send` into a queue of 4096 lines and never waits; one thread writes to a private copy of file descriptor 1. When the queue is full, the line is dropped and counted, and the next line that does get written says how many were lost. I did not use `tracing-appender` (a new dependency) or `s6-log` (the compositor is started by a service without a log service, and `s6-log` reading its pipe would not remove the problem if it stopped). Two facts the document does not state: (a) **driftwm's log lines go to standard output, not standard error** (TESTED: stdout 13,522 bytes and stderr 192 bytes in 4 seconds of start-up, for both the unpatched and the patched binary; `tracing_subscriber::fmt()` writes to stdout by default); the hub's service joins the two with `exec 2>&1` so there is no difference there. (b) The default panic message is written to standard error by the thread that panics, and would block on a full pipe, turning a crash into a freeze; so P7 also replaces the panic hook by one that logs the panic through the same queue and waits at most 0.5 s for it to be written. This second part is my addition, and the research did not ask for it.
- **P8 (changed after the owner's decision 4).** The document said "remove or time-box the start-up shell-outs; set the environment before threads start". The owner decided: **remove** the helper calls (the hub has no systemd and starts its own D-Bus). Removed: the environment import in `main.rs` and the `DISPLAY` import in `xwayland.rs`. Kept and time-boxed (3 s, then kill): the xwayland-satellite probe (it checks that a program exists; not a systemd helper). For the environment: `DRIFTWM_CONFIG` and the four fixed `XDG_*` variables are set at the top of `main`, before the log thread starts. **`WAYLAND_DISPLAY` is still set after threads exist** (its value is the socket name, and the graphics backend must not see it), so the race the research called F7 is smaller, not gone (BELIEVED tiny). The earlier version of this patch (time-box) was in the first round and is replaced.
- **P11.** The research (F5) listed the session and state file writes on the event-loop thread as a separate freeze. Implemented: `src/bgwrite.rs` (standard library only): `submit(path, bytes)` puts the finished bytes in a mailbox and returns; one worker thread creates the folder, writes `path.tmp` and renames. The state file's content caches are now updated without waiting for the write (before, only after a successful rename), so a failed write is logged by the worker and fixed by the next change, not retried at once. Before exit the main thread gives the worker 0.5 s. The folder creation for the state file moved to the worker too. **Counters (owner decision, round 2).** `bgwrite::failed_writes()` counts writes that ended with an error (logged by the worker as `failed to write ...`); `bgwrite::pending_writes()` counts writes waiting or in progress. Both are in `StateInfo`, so they are in `driftwm msg state` and in the pushed state events. **Format change, said plainly:** JSON: two new keys `failed_writes` and `pending_writes` (unsigned numbers, `serde(default)`, so readers that do not know them and old snapshots without them still work); text output of `driftwm msg state`: two new **last** lines `failed-writes N` and `pending-writes N`, after the `outputs` lines (nothing that parses the older lines by prefix is disturbed; the hub image test reads this text). A write that blocks for ever (the FIFO case) is not a failed write, it never ends: it shows as `pending_writes` 1 that does not go down; a write that fails (the temporary name is a directory) raises `failed_writes`. `docs/ipc.md` was not changed (it lists the fields only as examples).
- **P12.** The owner asked for a patch that removes the hot reload (the inotify watch) and the `reload-config` action; in round 2 also the dead code. Implemented: the watch block in `main.rs` is deleted; `parse_action("reload-config")` returns an error (a key binding with it shows a warning and `driftwm msg action reload-config` is rejected); the `Action::ReloadConfig` variant, its two uses and the `reload_config` function (reads the file) are removed; the function that applies a config from text stays for the tests. The words `reload-config` still appear in two doc comments of driftwm (the generated `docs/cli.md` and `docs/ipc.md` say the socket's `action` can `reload-config`); I did not change them (changing the clap comment means regenerating `docs/cli.md`).
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
- **Hot reload cannot be switched off by a key. Said plainly: there is no such key.** driftwm always watched the folder of its config file with inotify and has a `reload-config` action (SOURCE `src/main.rs`, `config.reference.toml:427` in the pinned source). The owner decided on a **patch** (P12) that removes both; the read-only file in the image stays as well (BELIEVED, not tested that an edit of a file in the read-only image fails). TESTED (6.2, `reload`): with P12 an edit of the config file in the running compositor changes nothing and `reload-config` is refused.

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

Command: `HS_WORK=... run-all.sh UNPATCHED PATCHED [UNPATCHED+HOOKS PATCHED+HOOKS [UNPATCHED+BADSHADER PATCHED+BADSHADER]]`. The verdict lines (TESTED, 2026-10-05 for `pipepanic` and `badshader`, 2026-10-06 for the others, the second round on the final patch set `0001`-`0012`):

| Reproduction | Unpatched | Patched |
|---|---|---|
| `wl_shm_pool.resize(0)` from a client (`shm`) | **compositor DIED, exit status 101** (panic at Smithay `shm/handlers.rs:190`) | **survived**, IPC answers |
| Non-ASCII colour (`"#aéaaa"`) in the config (`config`): `--check-config`, and the file saved into a running compositor | **KILLED**: `--check-config` exit 101, running compositor exit 101 (panic at `parse_helpers.rs:142`) | **survived**: `--check-config` exit 0 with "Config OK, 1 warning(s)", running compositor alive and answering |
| The log pipe is full and nobody reads it (`pipe`; pipe pre-filled with 64 KiB, `RUST_LOG=trace`, 12 windows) | **FROZEN**: the process sleeps in `anon_pipe_write`, 0 of 5 IPC requests answered (it did not even get its IPC socket up) | **not frozen**: 5 of 5 IPC requests answered in 0 to 30 ms, pipe still full (65,536 bytes) |
| A panic while the log pipe is full (`pipepanic`, with the P2 test hook) | already frozen by the pipe before a panic could be sent | the panic **ended the process** with status 101 after 0.6 s (instead of hanging in the panic message) |
| The start-up helper calls (`startup`; a fake `dbus-update-activation-environment` that records that it ran and sleeps 60 s) | **BLOCKED**: no IPC after 25 s; the helper was started | **started in 0.2 s and did not even try the helper** (the fake helper never ran) |
| The built-in default shader does not compile (`badshader`) | **DIED**, exit 101: `Default shader must compile: ShaderCompileError` (`background.rs:748`) | **survived** with a flat background, IPC answers; logged `Default background shader failed to compile` |
| A blocked session write (`session`: the session file's temporary name is a FIFO nobody reads, the stand-in for a hung disk, hub-stability T12) | **FROZEN** (`wait_for_partner`, 0 of 4 answered) | **not frozen** (P11): 4 of 4 answered in 0 ms; IPC state shows `pending_writes: 1, failed_writes: 0` (the stuck write) |
| A failing session write (`sessionfail`: the temporary name is a directory, so every background write fails at once) | no counter (the keys do not exist) | IPC keeps answering; `failed_writes: 1, pending_writes: 0` |
| Config hot reload (`reload`: the config file is saved into the running compositor; then `reload-config` over IPC) | `Config reloaded` is logged after the edit; `reload-config` answers `Ok` | the edit changes nothing (no log line); `reload-config` answers `Err("reload-config is not available (config hot reload is removed, Hub OS patch P12)")`; compositor alive |

In the first round the `session` row was still frozen with the patched build (no patch covered it); P11 closes it. The first-round `pipepanic` and `badshader` rows below were measured with the first-round builds and were not repeated for P11/P12 (the patches behind them did not change; the `badshader` and `pipepanic` binaries were not rebuilt).

The `pipepanic` "before" is weak: the unpatched binary was already stuck in the pipe write at start-up (the pipe was full from its first log line), so it never reached the panic. A panic with a pipe that fills *later* was not tested on the unpatched binary.

### 6.3 Unit tests

Command (inside the build root, in a private mount namespace, final patched tree): `cargo test --release`. TESTED (2026-10-06, round 2 final tree): library 331 passed; binary 1,350 passed, 0 failed, 13 ignored; the integration tests (config reference included) and doc tests all `ok`. New tests: `backend::frame_health::tests::*` (P10), `bounded::tests::*` (P8), `logwriter::tests::*` (P7), `bgwrite::tests::{a_normal_write_arrives, a_failed_write_is_counted, a_stuck_write_shows_as_pending, a_blocked_write_never_blocks_the_caller_and_the_newest_bytes_win}` (P11; the stuck and blocked tests use a FIFO as the temporary file, the failed one a directory). Failures found on the way and fixed: first P11 broke 20 session tests (`set_inline` for the tests); P12 needed the `reload-config` line removed from `config.reference.toml`.

**What is not tested for P10:** the real path. A frame that fails on a real GPU after a reset, a monitor that is switched off or unplugged (BELIEVED to produce no failed frames, but I do not know), and what a restart looks like on real hardware are **UNKNOWN**. The restart rule of 5.2 bounds a restart loop at 5 in a minute.

### 6.4 Short runs of the stability fuzzer and soak

Scripts of `docs/proposals/hub-stability/scripts/` (`fuzzrun.sh`, `fuzz.py`, `healthcheck.py`, `soak.py`, `soakstat.py`); for `soak.py` the display number was changed from `:80` to `:81` in my private copy. Short runs only, final patched binary (P1 to P12 except P2), TESTED 2026-10-06:

- **Fuzz, 120 s each, seeds 20 and 31:** both finished with the compositor alive and the health check passing (IPC answered, a new `foot` window mapped, a `Move` applied): about 20,000 requests and 298 connections per seed (fewer requests than in the first round, about 50,000, on a slower, busier machine; the protocol set has 68 XML files, wlr protocols from the gitlab master, unpinned; the research used 72); memory 142 MB.
- **Soak, 180 s, seed 7:** 381 operations (open, close, crash, move, resize, focus, pan, zoom, bookmark, suspend, relaunch of up to 20 windows), 0 compositor restarts, 0 failed IPC probes, memory 133 to 217 MB, 66 file descriptors on average (85 at most), IPC median 0.3 ms, 29% of one core. This says nothing about growth over hours. P11's worker thread was running during the soak (the session file is written about once a second while windows move); the thread count was 12 to 13.

### 6.5 Proposal for P3's counter (a proposal only, not built, not switched on)

Written after the soak, as the owner asked. P3 catches a panic and goes on when `HS_CATCH_PANICS` is set, but the fuzz in the research showed that a caught panic can leave a poisoned lock and the next pass of the loop panics outside the catch. **Proposal (BELIEVED, not tested):** keep a counter of caught panics in a static; on the **first** caught panic log it, flush the log (0.5 s at most) and set a flag; the main loop (the closure given to `event_loop.run`) checks the flag at the start of its next pass and exits with status 71 (so the service counts it as a crash, section 5.2); the **second** caught panic within 60 s exits at once. In words: the compositor survives one bad request long enough to write its log and the session file (P11's flush), and then restarts cleanly instead of running on with half-updated state. The reason not to exit inside the catch: the state the session file is built from may be the half-updated one; the exit in the next loop pass happens after the worker has written what was already queued. Open: whether to switch P3 on at all (owner decision 2: stays off).

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

### 7.1 Second round (2026-10-06): the final head, with PR #63's tests

Head `c9c3c96` (this branch on top of main with PR #64 and PR #63 merged; sub-tests H3d, H3e, H3f are PR #63's). Same command. Results:

| Run | Result | Notes |
|---|---|---|
| `TestHubImage` N1 | **FAIL**, 11 of 12 sub-tests passed (2,032.56 s), 0 hangs, 0 crashes | **H4 failed**: `update rc=0; now slot b release 2, confirmed true, counter 0/3; driftwm's event loop started before the confirm line: false; bar pixels 27895, alert 2138`. The update, the trial boot and the confirm worked (slot b release 2, confirmed, counter 0/3, desktop and bar drawn); the failing check is that the line `Starting event loop` was **not in the serial log at all** when the confirm line (at byte 12,330 of the boot log) was already there. BELIEVED cause: since P7 the compositor's log lines are written by a thread and may reach the serial console later than other programs' lines, so the test's assumption (log order = real order) is no longer sure; this is a flaw of the check, not of the boot. NOT proven: I could not see the full boot log (it is deleted with the temporary folder), and the same check passed in three earlier runs (B, D, N2). Not changed: I did not edit the test. |
| `TestHubImage` N2 | **PASS**, 12 of 12 (2,053.10 s), 0 hangs, 0 crashes | H3f 447.7 s, H4 81.2 s, H5 200.4 s, H6 85.2 s |
| `TestImage` N3 | **PASS** (3,632.22 s), **1 QEMU hang** (retried) | hang in the step "rollback after 6c" (the qemu-test image: no text console, no patched compositor). All sub-tests PASS. |

Kernel sizes printed by these runs: `kernels (a, b, recovery) built ...: 4473856 bytes, ... recovery 6689792 bytes` (N1 and N2).

### 7.2 The hang of run A: what the evidence says

- **The serial log of the hang is gone.** The runner saves it into the run's temporary folder (`.../hangs/hang-1.log`), which the test deletes at the end; I looked (`/tmp/hubos-hub-2671855772`, `find` for `hang-1.log`) and it does not exist. Its content is UNKNOWN beyond the last lines the test printed: `STAGE0: switching to slot b`, `HUBOS: stage1 start ...`, `HUBOS: booted entry 0008 label 'hubos-b'`, then nothing for 150 s.
- **It is not the hang described in `docs/image.md` 3.3** (a start that stops silently after `BdsDxe: starting Boot0001`, i.e. in the firmware before stage 0). Run A's hang happened after stage 0 and stage 1 had printed, inside the first lines of PID 1's script. The two are different places.
- **The same kind of "no known log line in time" stop happened in a run that has none of this change's parts:** `TestImage` N3 (qemu-test kernel fragment, no text console, no patched compositor) had a hang in the step "rollback after 6c" in the same round; its last printed lines were a complete boot up to `confirm: hubd did not become healthy in 30s; this boot FAILED; rebooting`, so there the runner's 150 s wait for the handover line expired although the guest was alive (a slow or stalled start under a busy host, BELIEVED). So the runner's hang detector fires without this PR.
- **Counts with the new kernel (text console):** hub runs A, B, D, N1, N2 = 5 runs, each with 1 or 2 trial-boot reboots (H4, H5); one hang (A). Runs on the old kernel from the first round on 2026-10-05 (hubA to hubG, TestImage): 0 hangs. This is too little to say anything about a difference (5 runs vs 7), and there is **no direct comparison**: I did not run the old kernel and the new kernel one after the other for a few reboots each, because it needs a disk-image harness I did not build; I say so plainly.
- **Conclusion (BELIEVED, not proven):** a stall of the QEMU guest or of the runner's timing, not caused by this PR, because (a) the same detector fired on an image without the PR's kernel option, (b) the stop point differs from the known firmware one but is in code this PR did not change (stage 1), (c) four later hub runs with the same kernel did the same two reboots without a hang. UNKNOWN: the real cause.

### 7.3 Round 3 (2026-10-06): H4 check changed, the hang comparison, the final runs

**H4 changed** (owner decision). The order condition ("driftwm's event loop started before the confirm line", which compared positions in the serial log) is dropped. The check "confirmed after hubd answered" stays. New: the compositor answered its IPC before the confirm. Evidence that does not depend on the serial log: the confirm step (`image/rootfs/etc/s6/sv/confirm/run`) runs an optional hook `/usr/lib/hubos/confirm-evidence` right before it confirms; the hub image has one (`image/machines/hub/rootfs/usr/lib/hubos/confirm-evidence`), which runs `driftwm msg state` as the user hub (10 s limit) and writes `/run/hubos/confirm-evidence` (`time=` and `driftwm-state=answered|failed`) and `/run/hubos/confirm-state.txt` (the answer). H4 reads these files; it needs `driftwm-state=answered` and a `camera` line. The hook never fails the boot and never changes whether a boot is confirmed. `confirm/run` is shared with the qemu-test image (there is no hook there), so `TestImage` was run again.

**Hang comparison** (`tools/image/hangcompare_test.go`, a throwaway test, `-run TestHangCompare`; summaries in `tools/image/experiments/hang-compare/`). Machine quiet (no other long job; checked with `ps` and `uptime` before; the other helper had finished). Each cycle: a fresh disk, first boot (slot a), `hubos-ctl update` of the bundle h2-good into slot b, `reboot -f`, then one wait of 150 s for the handover line of the trial boot (no retry), then the confirm. Everything is the same in both runs (same root, same bundles, same QEMU settings, same host) except the kernel: **new** = the current `hub.frag` (slot kernels 4,473,856 bytes, recovery 6,689,792); **old** = `hub.frag` without `CONFIG_VT`, `CONFIG_FB`, `CONFIG_DRM_FBDEV_EMULATION`, `CONFIG_FRAMEBUFFER_CONSOLE`, `CONFIG_FONTS`, `CONFIG_FONT_8x16` (4,445,184 and 6,661,120). TESTED result: **new kernel: 10 trial boots, 10 ok, 0 hangs, 0 crashes; old kernel: 10 ok, 0 hangs, 0 crashes**; trial boot to confirm 47 to 87 s (new) and 42 to 77 s (old); every trial boot ended confirmed on slot b release 2. All 20 serial logs are kept (scratchpad folder `hangcmp/`, 1.2 MB, not committed; the two summaries are). This **does not reproduce** the hang of run A on either kernel, and shows no sign that the text console causes it. It does not prove the stall cannot happen (20 boots; the run-A stall was 1 in about 10 trial boots of that session: UNKNOWN whether the rate is the same).

**Final runs on head 84fb69f** (same code as the head with the summaries only): `TestHubImage` **PASS** 12 of 12 (2,233.40 s) and **PASS** 12 of 12 (2,314.62 s), 0 hangs, 0 crashes in both; H4 82.0 s and 86.2 s with the new check. `TestImage` **PASS** (3,297.17 s), 1 QEMU crash (signal 11, the known QEMU 8.2.2 TCG crash, in the step "recovery after the watchdog reset") retried by the runner. A first attempt at `TestImage` was killed by a restart of the container after 39 s and is not counted. driftwm: `cargo test --release` lib 331 passed, bin 1,350 passed, 0 failed.

## 8. What was not verified

- Anything on real hardware: a real GPU, a real screen (the crash message is shown on a virtual screen only), real input.
- P3 (switched off), P5, and the socket handover P1 were not exercised here.
- P10 on a real GPU failure (6.3).
- A long soak or long fuzz of the patched build; only the short runs of 6.4.
- That the settings file's keys have the visual effect (no shadows, corners, borders): the file is accepted; I did not compare screenshots with and without it.
- The hub image with a real (non-software) GPU driver: the hub image test uses software drawing.

## 9. Mistake made while working (said plainly)

While cleaning up scratch folders I ran `rm -rf` on a folder that still had three of my own bind mounts left over from earlier builds (a stale mount of `~/.cargo`, `~/.rustup` and a source folder). That deleted the **contents of `/root/.cargo` and `/root/.rustup` on this machine** (the Rust toolchain and its crate caches). One image-test run in progress failed because of it (`cargo: not found`); it was started again. I reinstalled the same toolchain (rustup, `1.97.0`, the same `cargo 1.97.0 (c980f4866 2026-06-30)` as before) into the same two folders; the crate and git caches are empty and refill on the next build (needs network). Any other session that used `/root/.cargo` between about 21:39 and 21:42 UTC may have seen errors. No repository file was affected.

**Where the Rust toolchain came from (section 9).** `rustup-init` was downloaded from `https://static.rust-lang.org/rustup/dist/x86_64-unknown-linux-gnu/rustup-init` (the official site) and run with `-y --default-toolchain 1.97.0 --profile default --no-modify-path`. **I did not verify a checksum myself**; the lead verified the installed rustup binary against the hash published at static.rust-lang.org. The downloaded file was deleted afterwards. New rule since then (owner): before any `rm -rf`, run `findmnt` and check for mounts under the path, use `rm -rf --one-file-system`, never delete under `/root/.cargo` or `/root/.rustup`. The second round's builds run in a **private mount namespace** (`unshare -m`), so no bind mount can outlive a command.

## 10. Owner decisions (2026-10-06) and what is still open

The owner answered the 13 questions of the first round on PR #64. What each answer changed:

1. **P2 (test hooks) stays out of every image.** Unchanged (`not-applied/`).
2. **P3 stays off.** After the soak a proposal for the counter ("exit after the first or second caught panic") is written in 6.5; it is not built and not enabled.
3. **P7 as implemented is OK** (own small writer, standard output). Unchanged.
4. **P8: remove the start-up helper calls instead of time-boxing them.** Done (section 3). Test: with the fake hanging `dbus-update-activation-environment` the patched compositor does not even try it (6.2).
5. **P9: black is fine.** Unchanged.
6. **P10 OK as implemented.** Unchanged.
7. **Blocked session write: P11**, session and state files written by a worker thread. Done; before/after with a FIFO as `session.json.tmp` (6.2).
8. **Hot reload: a patch that removes the inotify watch and the `reload-config` action.** Done as **P12** (the number is mine); test (6.2).
9. **The kernel text console stays; report the size change.** TESTED (from the `kernels (a, b, recovery) built` lines of the test runs; the old fragment: five runs on 2026-10-05 before the change, all identical; the new fragment: every run since): slot kernels `kernel-a.efi` and `kernel-b.efi` **4,445,184 bytes before, 4,473,856 bytes after (+28,672, +0.64 %)**; `kernel-recovery.efi` **6,661,120 bytes before, 6,689,792 bytes after (+28,672, +0.43 %)**. (The test recovery kernel with the agent, built by `TestImage` from another fragment, is not affected: the qemu-test fragment was not changed.) I did not rebuild the old fragment for this report; the numbers are the ones the runs printed (older runs on the same code base for the "before").
10. **The confirm step refuses while the marker exists: yes.** Unchanged.
11. **Crash = any end except exit 0, sliding window of 60 s and 5: yes.** Unchanged.
12. **Message wording kept; no bar alert.** The exact four lines, as printed on the screen and on the serial console:

```
HUB OS: the desktop (driftwm) crashed 5 times within one minute and was stopped.
It will not start again by itself, and the machine will NOT reboot.
hubd and the recovery terminal are still running.
To try the desktop again by hand: s6-svc -u /run/service/driftwm
```

13. **hubd running without the compositor: yes.** Unchanged. (After PR #63 hubd is started through `follow-driftwm --once`, and keeps `FOLLOW_GAVEUP=run`.)

**The hang of run A.** See section 7: the serial log of the hang was deleted with the temporary folder, so its content is gone and the cause is UNKNOWN; the evidence is in section 7.

**Still open (new questions):**

A. P11 means the state-file content caches are updated before the write is known to have succeeded (section 3). A write that fails is logged and repaired by the next change, not retried at once. Acceptable?
B. P12 leaves the dead `Action::ReloadConfig` code in place to keep the patch small. Remove it as well?
C. The `pipepanic` and `badshader` tests were not rerun on the P11/P12 build (their patches did not change). Rerun on request.

## 11. Adoption of the research patches (round 4, 2026-10-06; PR #71)

**What this section is.** The owner decided to adopt the patches of the bulletproof-compositor research (`docs/proposals/bulletproof-compositor.md`) into the hub image, together with a set of owner decisions numbered 3 to 14 in that task. This section says what was built, the decisions as applied, the protocols that are now hidden and the evidence, the tests, and every question that is still open. Everything is labelled TESTED, SOURCE, BELIEVED or UNKNOWN as above. **Written by a helper agent of the lead.** No sub-helpers were used; everything here I ran or read myself (2026-10-06).

### 11.1 The patches, and what is new

The complete list, one line per patch with the BC finding it fixes and the owner decision, is `image/patches/driftwm/README.md` (SOURCE: that file). In short:

| Group | Files | Status |
|---|---|---|
| driftwm P1 to P12 | `0001` to `0012` | as before (sections 3 to 10) |
| Smithay P6 | `smithay/0006` | as before |
| **Smithay P7 to P11** (the research's names; BC-1 to BC-4) | `smithay/0007` to `0011`, named `smithay-pN` in the file name because driftwm already has a P7 to P11 | adopted unchanged from `docs/proposals/bulletproof-compositor/patches/` (the research folder is kept as the record) |
| **driftwm D1 to D5** (BC-4, BC-5, BC-6, BC-11 part, BC-12, BC-13) | `0013` to `0017` | adopted unchanged |
| **D6** (BC-11, the part D5 left) | `0018` | new: `resolve_cluster_shifts` uses saturating arithmetic (`saturating_neg`, `saturating_add`), plus four unit tests |
| **D7** (repro tests of BC-12 and BC-13) | `0019` | new: tests only |
| **D8** (hidden protocols, owner decision 7) | `0020` | new, see 11.3 |
| **D9** (docs without `reload-config`) | `0021` | new, see 11.7 |
| **D10** (fsync of the session file, owner decision 9) | `0022` | new, see 11.5 |

**Applying and failing.** `tools/image/build-hub-parts.sh` already applied every `0*.patch` and every `smithay/0*.patch` with `git apply` (no fuzz) and stopped the build with the name of the patch if one did not apply; that code did not need a change, the new files are found by the same globs and are part of the stamp that decides whether driftwm is rebuilt. TESTED: all 22 driftwm patches (without the not-applied `0002`) and 6 Smithay patches apply in order to the pins `352333a8...` and `4cf0b620...` (the log of the image build prints `applied driftwm patch ...` for each). The earlier test of the failing case (section 4: three broken-patch cases, exit 1) still describes the script; I did not repeat it because the script's patch loop is unchanged.

**Overlaps (checked, nothing dropped).** Written out in `image/patches/driftwm/README.md`. The ones that matter: `src/session.rs` is touched by P11 (writes) and D1 (the cap on reading), and after this round by D10 (fsync); `src/config/*` by P4 (non-ASCII colour), D2 (NUL) and P12 (no hot reload). They fix different inputs, so **all are kept**. Nothing fixes the same thing in two ways. One thing that looks like a collision is not one: the research's "Smithay P10" (positioner clamp) and this set's driftwm P10 (exit on failed frames) are different patches; the names differ in the file names (`smithay-p10-...` against `p10-exit-on-failed-frames`). The research built on `hub-stability`'s P1 to P6 only, so the order "P1 to P12, then D1 to D5" was never built before this round: TESTED here (all apply, `cargo test --release` passes, 11.8).

### 11.2 Owner decisions applied, and what each one meant in the code

| # | Decision (owner, as given in the task) | What was done | Label |
|---|---|---|---|
| 3 | Keep the helper's limits: 2^24 for sizes and offsets, 512 session entries, 256 MB per stand-in | The patches carry exactly those numbers (smithay-p9/p10/p11: 2^24; D1: 512; D4: 8192 x 8192 x 4 = 256 MB). Not changed. | SOURCE (the patch files) |
| 4 | A protocol error disconnects the client for client mistakes; an error REPLY for hubd's requests; never a crash | Smithay P7 (`invalid_popup_parent`), P8 (`bad_surface`), P9 (`bad_value`), P11 (`invalid_size`) are protocol errors, so only that client is disconnected (TESTED: 11.9, the popup, subsurface and geometry reproductions leave the compositor answering). For the IPC: BC-12 and BC-13 now answer (the stand-in is clamped, the navigation action works) instead of crashing; BC-11 cannot panic. The IPC fuzzer on the final build: 55,046 requests in two runs, 0 crashes (11.9). **Not claimed:** that every IPC request returns an error reply; the IPC fuzzer shows no crash, not that every reply is an error. BC-14 and BC-9 (memory) are not patched. | TESTED |
| 5 | The compositor config is baked into the image: an unknown field fails the IMAGE BUILD; at run time an unknown field falls back to the last good config with an alert; `restore_windows` explicitly OFF, checked at build time | 11.4 | TESTED |
| 7 | Hide the protocols nothing on the hub uses, starting with session lock | 11.3 | TESTED (hub image) |
| 8 | A GPU reset or lost device exits and restarts the compositor; the hang rule stays | 11.6 | SOURCE / UNKNOWN |
| 9 | fsync the session files from the worker thread (P11's writer) | D10, 11.5 | TESTED (unit tests); power cut UNKNOWN |
| 10 | Restart on any panic, never carry on | 11.6 | SOURCE |
| 14 | The window stacking order is part of "restore exact" | hubd, 11.7 | TESTED (unit tests, H3f) |

### 11.3 Protocols the hub does not offer (patch D8), who would use each, and the evidence

**How it is done (SOURCE: `0020-d8-hide-unused-protocols.patch`).** Smithay's constructors for these protocols take a filter function that decides which clients may see the global. driftwm passes `client_is_unrestricted` (everybody, unless the client came through a security context). D8 passes a new `client_never` for the protocols below, so **no client sees the global and cannot bind it**; the compositor code behind them stays in the binary. Turning one back on is one word in `src/state/init.rs` (and a new image). In a **test build** of driftwm (`cfg!(test)`) `client_never` is the ordinary filter: driftwm's own 88 tests of session lock, virtual keyboard, workspaces, layer-frame gating and tablet bind these protocols through a test client, and with the globals hidden they failed (TESTED: first run of the patched tree, 88 failed, 1,264 passed). With the test exception all pass (11.8), and the hub test H1b checks the **shipped** behaviour in the virtual machine (below). So the unit tests keep the hidden code covered but do not themselves prove the hiding; the image test does.

| Interface (protocol) | Who would use it | Why the hub does not need it | Evidence |
|---|---|---|---|
| `ext_session_lock_manager_v1` (session lock) | a screen locker (swaylock) | no locker is in the image or planned (HUB-OS.md: no lock); **BC-10**: if the locking client dies the outputs stay blank "with no way back short of a VT switch" (the author's own comment, SOURCE research 4) | STATIC (list below), H1b |
| `zwlr_screencopy_manager_v1` (screen copy), `ext_image_copy_capture_manager_v1`, `ext_output_image_capture_source_manager_v1`, `ext_foreign_toplevel_image_capture_source_manager_v1` (the newer capture API) | screenshot and recording tools (grim, wf-recorder), screen-sharing | the hub runs viewers only; the screen servers (wayvnc) run on the NODES; driftwm has its own `driftwm msg screenshot` on the IPC | STATIC, H1b |
| `zwlr_output_manager_v1` | output configuration tools (wlr-randr, kanshi) | one fixed output, set by driftwm's own config | STATIC, H1b |
| `zwlr_output_power_manager_v1` | screen blanking tools (wlopm), idle daemons | the hub never blanks (projector; no idle daemon in the image) | STATIC, H1b |
| `zwlr_gamma_control_manager_v1` | colour temperature tools (gammastep) | none installed | STATIC, H1b |
| `zwp_virtual_keyboard_manager_v1` | on-screen keyboards, `wtype` | none installed. **Note:** the planned input forwarder is described as reading raw input devices and creating virtual ones (HUB-OS.md, "Unverified"), which is `/dev/uinput`, not this protocol. If the forwarder's design ever uses this protocol it has to be turned back on. | STATIC, H1b |
| `zwp_input_method_manager_v2` | input method editors (fcitx5, ibus) | no text entry program runs on the hub | STATIC, H1b |
| `wp_security_context_manager_v1` | sandbox launchers (Flatpak) | "No Flatpak" (HUB-OS.md) | STATIC, H1b |
| `zwlr_foreign_toplevel_manager_v1`, `ext_foreign_toplevel_list_v1` | taskbars | Waybar's bar has only the `custom/hub` and `clock` modules (`etc/hubos/waybar.json`); hubd reads the windows over driftwm's own socket | STATIC, H1b |
| `ext_workspace_manager_v1` | workspace switchers | no workspace module in the bar | STATIC, H1b |

**Evidence in two parts.** STATIC (TESTED, `tools/image/experiments/driftwm-patches/protocol-users.sh` over the unpacked root of the hub image, files under `usr/bin`, `usr/sbin`, `usr/local/bin`, `usr/lib`, driftwm itself excluded; output in `tools/image/experiments/driftwm-patches/results-2026-10-06/protocol-users.txt`): **13 of the 14 interface names are in no file of the image**, so nothing on the image can ask for them. The 14th, `zwlr_foreign_toplevel_manager_v1`, is in `/usr/bin/waybar` only (its optional taskbar module; the hub's `waybar.json` does not load it). RUN-TIME: the hub image test **H1b** runs `wayland-info` against the running compositor and requires that none of the 14 is listed and that the core ones (`wl_compositor`, `wl_shm`, `wl_seat`, `xdg_wm_base`, `zwlr_layer_shell_v1`, `zxdg_decoration_manager_v1`, `wp_viewporter`) are; and every other hub test (bar drawn by Waybar, the wofi menu and its clicks, foot windows, 20 windows, restores) passes with the globals hidden (11.10). A try to record what foot, Waybar and wofi actually bind (a second Waybar and wofi started by hand with `WAYLAND_DEBUG=1`) made the test machine stop answering on its console; I removed that part and did not find out why (UNKNOWN), so there is **no run-time bind trace**; the static scan and the working bar and menu are the evidence.

**Not hidden, on purpose.** `zwlr_data_control` and `ext_data_control` (the planned clipboard bridge uses `wl-copy` and `wl-paste`), `zwp_keyboard_shortcuts_inhibit` (VNC and SPICE viewers ask to keep the keys), pointer constraints and relative pointer (viewers), idle inhibit, fractional scale, viewporter, cursor shape, xdg-activation, xdg-decoration, layer shell (Waybar, wofi), and the core ones. Some protocols cannot be hidden this way because Smithay's constructor has no filter (tablet, xdg-foreign, text-input, content-type, background-effect, pointer gestures, idle notify): they stay on, and I judged none of them worth removing code for. That is a choice of mine, not an owner decision; it is question 9.

### 11.4 The settings file: build check, run-time fallback, `restore_windows` off (decision 5)

**Does driftwm reject unknown fields?** Yes, and no new code was needed for the check: every table of the config has `#[serde(default, deny_unknown_fields)]` (SOURCE `src/config/toml.rs`), and `driftwm --check-config --config FILE` prints the parse error and exits 1 (SOURCE `src/main.rs`; TESTED earlier in the research, `results/config-tests.txt`, `[effects] blur = false`). A bad VALUE (out of range) is not an error but a warning: `Config OK, N warning(s)` and exit 0.

**Build check (SOURCE `tools/image/check-driftwm-config.sh`, called from `tools/image/build-root-image.sh` for the hub).** It runs the finished binary inside the finished image root (`chroot`; the root has every library) on `/etc/hubos/driftwm.toml` and **fails the build** (exit 1, nothing is built) if: (1) the output is anything but exactly `Config OK` (so an unknown field, a bad TOML **and a warning** all fail; strict for a baked file), or (2) the `[session]` table does not contain `restore_windows = false`. The settings file in the image now says so explicitly (`image/machines/hub/rootfs/etc/hubos/driftwm.toml`). TESTED: `go test ./tools/image/ -run BuildCheck` (a fake driftwm that rejects the word `bogus`): good file accepted; an unknown field, a warning, a missing line, `restore_windows = true` and the line in the wrong table are all refused with a clear message; the hub's real file is accepted; a missing file is refused. TESTED with the **real** driftwm: see the first lines of 11.10 (the hub image build ran the check as part of every image).

**Why `restore_windows = false` and what BC-8 means now.** BC-8 (a config without `restore_windows = true` erases the saved windows from `session.json` at the next window change) is about driftwm's own window restore, which the hub does not use: hubd restores the windows (owner decision, HUB-OS.md), `session.json` lives in RAM (`XDG_STATE_HOME=/run/hub/state`), and the compositor starts with an empty session after a reboot by design. So BC-8 cannot harm the hub; the explicit `false` makes the file say what the hub relies on instead of relying on the default, and the build refuses a file that turns it on. BC-7 (one unknown field makes the whole file ignored at start) is closed for the baked file by the build check and for run time by the next paragraph.

**Run-time fallback (SOURCE `image/machines/hub/rootfs/usr/lib/hubos/driftwm-config`, used by `etc/s6/sv/driftwm/run`).** At every start of the compositor the service runs `driftwm --check-config` on the image's file. Good: it is used, and a copy is kept on the config partition as **`/config/hubos/driftwm.last-good.toml`**. Not good (an unknown field or a broken TOML): the last good copy is used if it passes; if there is none, the compositor starts **with its built-in defaults** (driftwm's own behaviour today, but now with an alert). The alert is: the fixed line `HUB OS ALERT: the desktop settings file ... is not valid ...; the desktop starts with ...` on the serial console and in the service log, and the file `/run/hub/driftwm-config-fallback` (in RAM) with the same line; the file is removed at the next start with a good file. TESTED: `go test ./tools/image/ -run RuntimeConfig` (fake driftwm): bad file and no copy gives defaults and an alert; good file is used and copied, alert removed; bad file with a copy uses the copy and does not overwrite it; a file with only a warning is used. TESTED in the hub image: H1b checks that the compositor was started with `--config /etc/hubos/driftwm.toml`, that the last good copy exists and is identical, and that no alert file exists. **Not tested in the image: the bad-file path** (the image file is read-only and the build refuses a bad one, so the path cannot be reached without a hand-made image; the script itself is tested on the host).

**Choices I made (simplest and reversible; all are questions 1 and 2):** the last good copy lives on the config partition and not in `/run` (a copy in RAM would never exist after a boot, and the image's file can only be bad after a boot); with no last good copy the compositor starts with defaults and does not refuse to start; **the alert is NOT on the bar**: the existing gave-up message (section 5.2) also goes to the screen and the serial console only, the bar alert is hubd's status line and hubd does not read any alert file. Putting it on the bar is a small change in hubd; I did not make it because it needs your decision.

### 11.5 `fsync` of the session file from the worker thread (decision 9, patch D10)

SOURCE `0022-d10-fsync-session-file.patch`. `session::write_async` now hands the bytes to the worker thread of P11 with a "durable" mark; the worker writes `session.json.tmp`, **`fsync`s the file, renames it, then `fsync`s the folder** (the folder sync is best effort). The event loop never waits. A newer plain write for the same path does not remove the durable mark of a waiting one (unit test). The state file in RAM is not synced. If the disk stalls the worker waits like for any other write, and the P11 counters (`pending_writes`) show it. TESTED: unit tests `a_durable_write_arrives_and_leaves_no_temporary_file` and `a_plain_write_does_not_cancel_a_waiting_durable_one`, and the P11 reproductions again (11.9: a blocked write still does not freeze the compositor). **UNKNOWN: the cost on a real disk and what a real power cut leaves** (needs hardware; research 7.5 and 10 of the bulletproof document). On the hub `session.json` is in RAM, so on the hub itself the `fsync` has nothing to protect; it matters only if `XDG_STATE_HOME` is ever put on a disk.

### 11.6 GPU reset, the hang rule, panics (decisions 8 and 10)

- **GPU reset or lost device (decision 8). What P10 does (SOURCE `src/backend/frame_health.rs`, `udev.rs`):** if every frame of an output fails for 2 seconds in a row (at least two failures), it logs once and exits with status 70, and the service restarts the compositor; hubd then restores the windows (section 5.2, hubd). That is the decision "exit and restart". **What is not there:** driftwm has no code that recognises a DRM or EGL "device lost" event as such (SOURCE: `grep` for lost, reset and robust in `udev.rs` finds nothing relevant); the exit happens only if the frames fail. A reset that makes `render_frame` hang instead of fail, or draw garbage without an error, is not caught by P10. UNKNOWN on a real GPU (no GPU here; December).
- **The hang rule ("kill after 4 failed rounds and 30 s with both probes failing, at most 3 kills in 10 minutes"). Does it exist in code today? No.** SOURCE: `grep -rn` over `internal/`, `cmd/`, `image/` (rootfs, machines, stage0) and `tools/` finds the rule only in `HUB-OS.md` (the hub stability decisions) and in the research documents (`docs/proposals/hub-stability.md` and the bulletproof document, where the soak driver and `hangdrill.py` contain a probe loop that is test code, not part of the image). Nothing in hubd or in an s6 service probes the compositor for a hang or kills it. **I did not build it** (instruction: only if it already exists). Consequence TESTED in the research (BC-1): an unpatched hang ignores SIGTERM and needs `kill -9`; with Smithay P7 the known permanent hang is gone, but a hang from another cause would stay until someone kills the compositor. It is question 4.
- **Panics (decision 10).** P3 stays off (the service does not set `HS_CATCH_PANICS`). No `panic = "abort"` was set: the release profile is Cargo's default, so a panic **unwinds**. How a panic ends: a panic on the **main thread** (the event loop, IPC handling, all Wayland handlers) ends the process with status 101 after the panic hook (P7: logs the message through the non-blocking writer and waits up to 0.5 s for it) ; s6 restarts the service; `finish` counts it as a crash (5 in 60 s: stop, section 5.2); hubd restores the windows. TESTED (`pipepanic` reproduction: "panic ended the process, exit status 101 after 0.6 s"). **A panic in a helper thread does not end the process**: it only ends that thread. Threads in the compositor are the log writer (P7), the file writer (P11, new `driftwm-files`), the font scan, the desktop-entry scan and the render tile workers (SOURCE: `grep` for `thread::Builder` in `src/text.rs`, `src/bgwrite.rs`, `src/logwriter.rs`, `src/state/suspended.rs`, `src/render/tile_worker.rs`). A dead file writer would stop session and state writes without restarting the compositor, and `pending_writes` would grow; a dead log writer would stop the log. That is "carry on" for those threads. `panic = "abort"` would end the process on every panic in every thread, which is what "restart on any panic, never carry on" says, but it also removes unwinding for driftwm's own `catch_unwind` (one driftwm test relies on `catch_unwind`; whether Smithay or Mesa callbacks depend on unwinding is UNKNOWN). I did not change the profile because the decision does not say whether helper threads count; **question 5**.

### 11.7 hubd: the stacking order is restored (decision 14), and the generated docs

**What was found about driftwm.** `driftwm msg state` and the IPC `State` list the windows in z-order, bottom to top, **but move the focused window to the front of the list** ("Focused window first, so consumers can read windows[0] as the focused one", SOURCE `src/state/persistence.rs`, `window_inventory`). So the position of the focused window is not in the answer. The only request that raises a window is `Focus` (it focuses and raises, `raise_and_focus`, and pans the view unless the window is fully in view; SOURCE `src/ipc/mod.rs`, `cmd_focus`); there is no separate raise. `Move` does not raise (SOURCE: the code comment "without the re-raise `map_window` would carry").

**What hubd does now (SOURCE `internal/driftwm/driftwm.go` `StackOrder`, `internal/hub/restore.go`).** (1) `State.StackOrder()` reads the list and **takes the focused window to be on top** (every way of focusing a window raises it: TESTED for `Focus`; BELIEVED for a click). (2) Every snapshot hubd sees updates `lastOrder`, the machine ids bottom to top (windows that are not a machine's, such as a terminal, are left out; the order is not updated while a restore is running, like the view). (3) When the compositor goes away the order is copied to the restore list. (4) After every window of a restore is back and before the view is put back, hubd calls `Focus` for each machine window from the bottom up, so the last one ends on top and has the keyboard focus; the view is set afterwards because focusing pans it. A failure is part of the message "the desktop restarted: ...". TESTED: four unit tests (`TestRestoreBringsTheStackingOrderBack`, `...KeepsTheOrderAcrossTwoCrashes`, `...OrderSkipsAWindowClosedOnPurpose`, `...SetsTheViewAfterTheStackingOrder`), written against a fake compositor that lists windows the way driftwm does (focused first); with the apply step stubbed out all four fail. The hub image test H3f now sets a random stacking order of the 20 windows (focus in random order, then the view back) and checks, after every kill cycle, that the order read from `driftwm msg state` is equal (same rule for the focused window) (11.10).

**Limits (said plainly).** The assumption "the focused window is on top" can be wrong if the window with the keyboard focus is below another machine window (BELIEVED not to happen through hubd, a click or `Focus`; not proven for every path). A driftwm change that reports the true order (an extra field in `WindowInfo`) would remove the assumption; I did not make it (it changes the IPC; question 6). **Saved layouts do not store the stacking order** (the layout file format has no field for it and this task named the restore); question 7.

**Generated docs (SOURCE `0021-d9-docs-without-reload-config.patch`).** driftwm's own test regenerates only `docs/cli.md` (`UPDATE_CLI_DOCS=1 cargo test docs_cli_md_is_up_to_date`; the source of the text is the doc comments of the clap definitions, so the comment in `src/ipc/client.rs` was edited and the file regenerated). `docs/ipc.md` and `docs/shaders.md` are **not generated**; they are hand-written, so I edited the sentences by hand (`reload-config` in the warning box of `ipc.md`; the "Reloading after edits" paragraph of `shaders.md`). After this, `grep reload-config docs README.md` finds nothing (the words remain in two source comments and in P12's error message, which says why the action is refused).

**Does `cli_docs::docs_cli_md_is_up_to_date` fail on the pristine tree?** The bulletproof research saw it fail on its patched tree and did not know about the pristine one. TESTED here, pristine tree (driftwm `352333a`, Smithay `4cf0b62`, no patch), same build root as the image build:

```
cargo test --release docs_cli_md_is_up_to_date
     Running unittests src/main.rs (/target/release/deps/driftwm-376c28b371a413e5)
running 1 test
test tests::cli_docs::docs_cli_md_is_up_to_date ... ok
test result: ok. 1 passed; 0 failed; 0 ignored; 0 measured; 1356 filtered out; finished in 0.00s
```

**It passes on the pristine tree and on the final patched tree** (11.8). So the failure the research saw was caused by its own tree or environment, not by driftwm itself (BELIEVED: its working tree had `docs/cli.md` from another state; not investigated, since it does not reproduce). The earlier failure in this document (section 3, P1 adds two options) was real and is fixed in P1 itself.

### 11.8 driftwm's own tests

TESTED, `cargo test --release --no-fail-fast` in the image build root (private mount namespace, shared target folder), on the tree made by applying all patches to the pins exactly as the image build does (`/tmp` copy, identical to the build's tree):

```
lib      test result: ok. 339 passed; 0 failed; 0 ignored
bin      test result: ok. 1352 passed; 0 failed; 13 ignored
integration tests (canvas_navigation 25, canvas_transforms 30, canvas_zoom 25, config_bindings 24, config_docs 1, config_parse 78,
  config_reference 10, config_robustness 3, config_toml 57, snap 18, window_rules 79): all ok, 0 failed
doc tests: ok
```

(The pristine tree had 327 library and 1,356 binary tests; the difference is the tests of the patches.) New tests that pass: `layout::cluster::tests::bc11_*` (4), `canvas::tests::bc13_*` (2), `decorations::tests::bc12_*` (2), `bgwrite::tests::a_durable_write_...` and `a_plain_write_does_not_cancel_...` (2). **Before the fixes:** the same eight tests (BC-11, BC-12, BC-13) added to the **pristine tree** (no other change): `cargo test --release bc1`: **6 of 8 fail** (`bc13_closest_point...`: `f64::clamp` panic; `bc11_extreme_height_delta...` left `(0, -2147483648)` right `(0, 2147483647)`; `bc11_extreme_width_delta...` the same on x; `bc11_primary_push...` left 852516351 right 2147483647; `bc12_...body_fill` panics at `decorations.rs:380` and `...title_bar` with `capacity overflow`), 2 pass (the push test and the bounding-box test do not detect the old code in a release build; they are kept as extra cover; they panic only in a debug build with overflow checks). With the fixes: all 8 pass.

### 11.9 Reproductions before and after, and the short fuzz runs (all TESTED, 2026-10-06)

**Method.** The nested compositor (winit backend on Xvfb, software OpenGL, `foot` windows) as in section 6, with a runtime folder made from the image's build root plus Xvfb, xkbcomp, foot, `libxkbcommon-x11` and the Mesa software drivers unpacked with `dpkg -x` (nothing installed; deleted afterwards). **Unpatched** = driftwm `352333a` + Smithay `4cf0b62`, release build in the same build root. **Patched** = `$WORK/out/driftwm` of the hub image build (all patches, the very file that goes into the image, before `strip`). Commands: `tools/image/experiments/driftwm-patches/run-bc.sh UNPATCHED PATCHED` (the research's minimal reproductions: `docs/proposals/bulletproof-compositor/scripts/verify.sh`) and `run-all.sh UNPATCHED PATCHED UNPATCHED+HOOKS PATCHED+HOOKS UNPATCHED+BADSHADER PATCHED+BADSHADER` (the `hub-stability` reproductions, section 6). One environment problem on the way: the first try had no `libxkbcommon-x11`, so every compositor died at start with a panic in `xkbcommon-dl`; the numbers below are from the second try.

| Finding | Reproduction | Unpatched | Patched |
|---|---|---|---|
| BC-1 popup that is its own parent / two popups naming each other | `repro_popup.py self`, `pair` | **HUNG** at 100 % CPU (both) | alive and answering |
| BC-2 `place_above` itself | `repro_subsurface.py` | **DEAD** (exit status 1 in this run; the research saw 101) | alive, the client got an error event |
| BC-3 viewport destination 16384 x 2^31-1; subsurface offset 2^31-1, 2^30 | `repro_damage.py` (3 variants) | **DEAD**, exit 134 (all 3) | alive (all 3) |
| BC-6 NUL in a key binding (`--check-config`) | `nulcfg.sh` | **panic**, exit 101 | exit 0 |
| BC-6 NUL in the keyboard layout, at start / at reload | `nulstart.sh` | **DEAD** (start: status 101; reload: status 101) | alive (at reload: there is no reload, P12) |
| BC-12 IPC `Resize` of a stand-in to 32768 x 32768 | `repro_ipc.py standin_resize_32768` | **DEAD**, exit 101 (`capacity overflow`) | alive and answering |
| BC-13 IPC `Move` to (65535, 2147483647), then `center-nearest left` | `repro_ipc.py nav_far` | **DEAD**, exit 101 (`f64::clamp`: min > max) | alive and answering |
| BC-11 overflow in `resolve_cluster_shifts` | unit tests `bc11_*` (pure logic: no live reproduction) | 3 of 4 tests fail (11.8) | all 4 pass |
| BC-5 (30,000 session entries), BC-4 (debug-build overflows), BC-9, BC-10, BC-14 | not re-run here (BC-5 and BC-4 are covered by the research; BC-9, BC-10, BC-14 have no patch or are hidden) | | |
| F1 `wl_shm_pool.resize(0)` (`shm`) | `repros.py shm` | **DIED**, exit 101 | **survived**, IPC answers |
| F3 non-ASCII colour (`config`) | `repros.py config` | **KILLED** (check-config exit 101, running compositor exit 101) | **survived** (check-config exit 0) |
| F4 log pipe full (`pipe`) | `repros.py pipe` | **FROZEN** (0 of 5 IPC requests answered; sleeping in `anon_pipe_write`) | **not frozen** (5 of 5) |
| F4 + a panic (`pipepanic`, test hook build) | `repros.py pipepanic` | already frozen by the pipe (0 of 5), so no panic could be sent | the panic **ended the process**, exit 101 after 0.6 s |
| F6 start-up helper that hangs (`startup`) | `repros.py startup` | **BLOCKED**, no IPC after 20 s, the helper was started | started in 0.2 s, the helper was not even tried |
| default shader does not compile (`badshader`, test build) | `repros.py badshader` | **DIED**, exit 101 | **survived**, flat background, IPC answers |
| hot reload (`reload`) | `repros.py reload` | works, `reload-config` answers `Ok` | no hot reload; `reload-config` refused; compositor alive |
| blocked session write (`session`) | `repros.py session` | **FROZEN** (0 of 4 answered) | **not frozen** (4 of 4); `pending_writes: 1` (this is the build with D10: the worker is stuck in `open` on the FIFO, as before) |
| failing session write (`sessionfail`) | `repros.py sessionfail` | no counter | IPC answers; `failed_writes: 1, pending_writes: 0` |

The two reproductions the owner asked to be rerun, **`pipepanic` and `badshader`**, ran on the final patched build (the hooks build and the broken-shader build are the final patch set plus `not-applied/0002-p2-test-hooks.patch`, and plus a deliberately broken `src/shaders/dot_grid.glsl`): rows above, same verdicts as in section 6.2. The raw outputs are the output of the two commands; the full text of both is in the PR description.

**Short fuzz runs, final patched build** (the research's `fuzz2.py` and `ipcfuzz.py`, 68 protocol XML files: wayland, wayland-protocols from the Ubuntu package, the wlroots protocols from the wlroots repository master, unpinned; the globals that D8 hides cannot be bound, so the fuzzer cannot exercise those protocols on this build):

| Run | Result |
|---|---|
| Wayland fuzzer, **unpatched**, 1 seed, 100 s | **HUNG** after 13 connections (the loop did not answer for 10 s; SIGTERM ignored) |
| Wayland fuzzer, **patched**, 2 seeds x 120 s | seed 1101: 1,279 connections, 75,984 requests, alive, IPC answers, SIGTERM exits in 0.2 s; seed 1102: 1,731 connections, 101,569 requests, same. **0 crashes, 0 hangs** |
| IPC fuzzer, **unpatched**, seed 11, 120 s | **DEAD**, exit 101, after 600 requests |
| IPC fuzzer, **patched**, seeds 11 and 12, 120 s each | 41,884 and 13,162 requests, alive at the end, memory 169 MB and 135 MB. **0 crashes** |

Total fuzzing time on the final build: 8 minutes. This is a smoke test, **not** the 24 hours the research proposes (criterion A2).

### 11.10 Tests in the hub image (virtual machines)

**PENDING**: the runs are in progress; this section is filled in when they finish.

### 11.11 The hang comparison, and why its test is gone

The throwaway test `tools/image/hangcompare_test.go` and its two summary files (`tools/image/experiments/hang-compare/`) were **removed in this round** (owner instruction, after the result was copied here). Result (TESTED, 2026-10-06, written down in section 7.3 in words; the numbers are copied here from the summary files before they were deleted): each cycle was a fresh disk, first boot, `hubos-ctl update` of the bundle h2-good into slot b, `reboot -f`, one wait of 150 s for the handover line of the trial boot (no retry), then the confirm; the only difference between the two runs was the kernel.

| Cycle | New kernel (`hub.frag` with the text console: slot kernels 4,473,856 bytes, recovery 6,689,792) | Old kernel (without `CONFIG_VT`, `CONFIG_FB`, `CONFIG_DRM_FBDEV_EMULATION`, `CONFIG_FRAMEBUFFER_CONSOLE`, `CONFIG_FONTS`, `CONFIG_FONT_8x16`: 4,445,184 and 6,661,120) |
|---|---|---|
| 1 | trial boot ok after 67 s | 72 s |
| 2 | 87 s | 57 s |
| 3 | 63 s | 70 s |
| 4 | 47 s | 45 s |
| 5 | 63 s | 59 s |
| 6 | 63 s | 46 s |
| 7 | 61 s | 77 s |
| 8 | 49 s | 42 s |
| 9 | 72 s | 46 s |
| 10 | 56 s | 62 s |
| result | **10 trial boots, 10 ok, 0 hangs, 0 crashes**; every one confirmed on slot b release 2 | **10 ok, 0 hangs, 0 crashes**; every one confirmed on slot b release 2 |

The 20 serial logs were kept in the scratchpad folder `hangcmp/` of that session (1.2 MB); they were **not committed** and are not available to this round (the folder belonged to the earlier helper). The comparison did not reproduce the hang of run A on either kernel and gives no sign that the text console causes it; 20 boots cannot prove the stall cannot happen (the run-A stall was 1 in about 10 trial boots of that session; UNKNOWN whether the rate is the same).

### 11.12 Questions for the owner (nothing here is decided; I picked the simplest reversible default where the text leaves it open)

1. **Bar alert for a settings-file fallback.** Today the alert is a line on the serial console, in the service log and the file `/run/hub/driftwm-config-fallback`; nothing shows on the bar, like the gave-up message. Do you want hubd to read that file and turn the bar item red with the line in its tooltip (a small hubd change)? (11.4)
2. **Last good settings copy:** `/config/hubos/driftwm.last-good.toml` on the config partition, updated at every start with a good file; with no copy the compositor starts with driftwm's built-in defaults (it does not refuse to start). Acceptable, or should it refuse / use another place? It is one more small file on the config partition that the NAS backup list does not know about. (11.4)
3. **Hidden protocols (11.3).** Is the list right? Especially: `zwp_virtual_keyboard` (hidden; if the input forwarder will use this protocol instead of `/dev/uinput`, it must stay on), and the ones I did not hide because Smithay gives no filter for them (tablet, xdg-foreign, text-input, content-type, background-effect, pointer gestures, idle notify). Hiding those would mean deleting code; do you want that?
4. **The hang rule is not in the code (11.6).** `HUB-OS.md` and the research describe "kill after 4 failed rounds and 30 s with both probes failing, at most 3 kills in 10 minutes", and the research soak driver has a probe loop, but **nothing in hubd or the image does it**. I did not build it. Should it be built, and where: in hubd (it already talks to the socket and the service state) or as a separate small s6 service?
5. **Panics in helper threads (11.6).** The compositor's main thread panic ends the process (status 101, restart, crash counter). A panic in a helper thread (log writer, file writer, font scan, desktop-entry scan, render tile workers) only ends that thread and the compositor carries on. Do you want `panic = "abort"` in the release profile so that **every** panic ends the process? (A small build change; it turns off unwinding for the whole binary and its libraries.)
6. **Stacking order assumption (11.7).** hubd takes the focused window to be on top because driftwm lists it first. A tiny driftwm patch (an extra field with the real position, additive in the IPC `State`) would remove the assumption. Do you want it?
7. **Saved layouts do not store the stacking order** (file format unchanged; overlapping windows are only a warning). Add it (an optional field, old files stay valid)?
8. **GPU reset (11.6).** P10 exits after 2 s of failed frames. It does not detect a reset that makes frames hang or draw garbage without an error. Accept until December, or add a probe (needs the hang rule, question 4)?
9. **`fsync` cost and meaning.** On the hub `session.json` is in RAM, so D10 protects nothing there; it matters only if `XDG_STATE_HOME` moves to a disk. Keep it anyway? (11.5)
10. **Still unpatched findings of the research:** BC-9 (a client's buffers cost about 8 times their size, no per-client limit), BC-14 (a failed allocation aborts the compositor under a memory ceiling), BC-10 is closed by hiding the protocol, the audit of Smithay's roughly 1,400 `unwrap` sites, and the 24-hour fuzz and 72-hour soak gates. None was in this task. Which of them, and when?

### 11.13 What was not verified

- Anything on real hardware (GPU, a real screen, a real disk, a power cut, real input).
- That `driftwm-config`'s fallback path works inside the image (it is tested on the host with a fake driftwm; the image's own file is good and the build refuses a bad one).
- The stacking order in a real session with a mouse: the unit tests use a fake compositor; H3f uses `driftwm msg focus` to set the order.
- The full 24-hour fuzz, a soak, miri and the sanitizers on the final patch set (the research ran them on its own set; this round ran 8 minutes of fuzzing on the final build).
- Upstream: nothing was reported to the driftwm or Smithay authors (owner decision).
- BC-5 and BC-4 reproductions were not rerun on the final build.
- That the hidden protocols are not used by the viewers that are not yet in the image (virt-viewer, a VNC viewer): the evidence is for foot, Waybar, wofi and the image's other programs; a viewer that needs one of them would show up as a failure and the fix is one word in `init.rs`.
