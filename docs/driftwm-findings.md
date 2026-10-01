# driftwm findings

**Researched: 2026-10-01 — build environment; will change.** This file records what was read in driftwm's source and what was run. `HUB-OS.md` wins if anything here disagrees with it.

- **What was examined:** `malbiruk/driftwm`, version `0.19.0`, commit `352333a8fa1b22171492d4b71a54102045c9a19d` (2026-09-22), read from a shallow clone at `/home/user/malbiruk/driftwm`. The clone is outside this repository. It was only read. Nothing was pushed to, forked from, or filed on driftwm.
- **Where it was run:** the cloud build environment described in `docs/environment.md` (no GPU, no `/dev/kvm`, no display, no systemd). Everything that was run used a software-rendered nested window on a virtual X display. **Nothing was run on a real screen or real input devices.**
- **File and line references** are in the driftwm clone at that commit, written `path:line`.

Every item is labelled with exactly one of:

- **TESTED** — the command shown was run in this session and gave the result written here.
- **BELIEVED** — taken from reading the code or documents, or thought to be true, and not run. The reason is given.
- **UNKNOWN** — not known.

A "TESTED" label on a code search means only that the search was run and gave that output. What the code *does* is **BELIEVED** unless it was also run.

---

## Summary: can we use driftwm as is, with small changes, or not?

**Provisional answer: yes, as is, with configuration only. No code change was found to be necessary for what `HUB-OS.md` asks of the hub's canvas. It is not proven on a real display.**

Reasons it fits:

1. **No systemd needed.** It ran with no systemd, no D-Bus and no seat daemon. It touches systemd only through two harmless start-up shell commands and a notification that does nothing when unset (section 2).
2. **Everything `hubd` needs is scriptable.** A local socket gives list windows, place a window at a canvas position, move the camera to a window or position, focus (the code also raises the window; not seen on screen), resize, and fit a window. All were run (section 4).
3. **Status bar support.** Layer-shell is supported and two real layer-shell programs mapped (section 5). Waybar itself was not run.
4. **Mouse-only navigation works.** Pan, zoom, move, resize, fit, fullscreen and "click a small window to jump to it" were all run with only a mouse and modifier keys (section 7).
5. **The keyboard can be handed to Moonlight.** A per-window `pass_keys` rule works (section 6). The `keyboard-shortcuts-inhibit` protocol is advertised but, by the code, **not honoured**, so the rule is the only way.
6. **The code is tested.** 2,033 tests passed, none failed, and the "soak" test passed (section 1).
7. **It is GPU-agnostic and distro-agnostic in code.** No vendor assumptions beyond opt-in quirks (section 10).

Reasons for caution — each could change the answer:

1. **Real hardware path is unproven.** The real-display backend (`--backend udev`) needs a DRM device. This environment has none. It reached "session created" and then stopped with `No GPUs found` (section 3). Whether it drives the projector correctly is **UNKNOWN** until Phase C or a VM with a virtual GPU.
2. **No magnification, ever.** Zoom is capped at 100% (`src/canvas.rs:12`). Zooming out shrinks the client's pixels. Nothing enlarges text. Small text on the projector has to be fixed with output scale or app font size (section 8).
3. **A single maintainer, young, AI-built, pre-1.0.** 94% of 1,244 commits are by one person, the first commit is 2026-02-23, the README says "primarily built with AI", and the version is 0.19.0 (section 1). Expect churn. We may need to carry patches or pin a commit.
4. **It builds against an unreleased Smithay.** `Cargo.toml:14` pins a Smithay git revision. A build needs GitHub access or the project's vendor tarball (section 11).
5. **Defaults assume a laptop with a trackpad.** All default gestures, and default keys that start `brightnessctl`, `wpctl`, `playerctl`, `swaylock` and `grim` (section 10). This is configuration, not code.
6. **Licence is GPL-3.0-or-later** (`Cargo.toml:6`). Shipping it is fine. Anything we change in it must be published under the same licence. `hubd` runs as a separate program and talks to it over a socket. This is not legal advice (**BELIEVED**).

What would turn "as is" into "small changes": if the owner wants the shortcut-inhibit protocol honoured (so Moonlight takes the keyboard without a per-app rule), a small patch in `src/input/keyboard.rs` is needed (section 6). If a real-hardware test fails, the patch would be in `src/backend/udev.rs`.

---

## 0. How the tests were run

The commands in each section depend on this set-up. Run it first. Nothing is installed on the machine: packages are downloaded and unpacked into a directory of your choice, called `$A` here. This is the same method as `docs/environment.md`.

```
A=$(mktemp -d)                     # an empty directory outside the repo
mkdir -p $A/apt/lists/partial $A/apt/cache/archives/partial $A/debs $A/root
O="-o Dir::State::lists=$A/apt/lists -o Dir::Cache=$A/apt/cache -o APT::Sandbox::User=root"
apt-get $O update
```

List, download and unpack the packages (the first set builds driftwm; the others are test tools). The packages were fetched with `curl -O` from the URLs `apt-get ... --print-uris` printed, then unpacked with `dpkg -x`:

```
apt-get $O install --print-uris -y --no-install-recommends \
  libwayland-dev libxkbcommon-dev libinput-dev libudev-dev libseat-dev libgbm-dev libegl-dev \
  libgles-dev libdrm-dev libdisplay-info-dev libegl1 libgles2 libgbm1 libgl1-mesa-dri \
  libwayland-egl1 libegl-mesa0 libglx-mesa0 xkb-data libxcb1-dev \
  | grep -oE "^'[^']+'" | tr -d "'" > $A/uris.txt
# then, for each URL: curl -sS -O "$url"   (into $A/debs)      and:   for d in $A/debs/*.deb; do dpkg -x $d $A/root; done
```

Notes on that list, so the result can be repeated:

- Packages built from the **systemd** source (`systemd`, `libsystemd*`, `libpam-systemd`, `systemd-dev`, `systemd-sysv`, `libudev1`) and `libgudev` were left out of the first pass: `grep -vE '/(systemd|libpam-systemd|systemd-dev|systemd-sysv|libsystemd|gir1\.2|libgudev|libudev1_)'`. That filter also removed `libudev-dev`. Its **headers and pkg-config file only** were unpacked by hand. The machine's own `libudev.so.1` is linked at run time.
- `libgudev-1.0-0` was added afterwards by hand because `libwacom` (needed by `libinput`) needs it. Without it the binary stopped with `error while loading shared libraries: libgudev-1.0.so.0`.
- These were added later, each the same way: `libxkbcommon-x11-0 libxcb-xkb1` (winit's X11 mode stops with `Library libxkbcommon-x11.so could not be loaded` without them), `xvfb xserver-common`, `foot libfcft4t64 libutf8proc3`, `wayland-utils`, `xdotool libxdo3`, `swaybg wlr-randr grim fuzzel`.

Point pkg-config and the loader at the unpacked files, and make the software renderer the only one used:

```
L=$A/root/usr/lib/x86_64-linux-gnu
find $A/root -name '*.pc' | while read p; do sed -i "s#^prefix=/usr#prefix=$A/root/usr#; s#^libdir=/usr#libdir=$A/root/usr#; s#^includedir=/usr#includedir=$A/root/usr#" $p; done
for f in $(find $L -maxdepth 1 -xtype l); do b=$(basename $(readlink $f)); [ -e /usr/lib/x86_64-linux-gnu/$b ] && ln -sf /usr/lib/x86_64-linux-gnu/$b $f; done
ln -sf /usr/lib/x86_64-linux-gnu/libudev.so.1 $L/libudev.so
export PKG_CONFIG_PATH=$L/pkgconfig:$A/root/usr/share/pkgconfig
export LD_LIBRARY_PATH=$L CARGO_TARGET_DIR=$A/target CARGO_NET_GIT_FETCH_WITH_CLI=true C_INCLUDE_PATH=$A/root/usr/include
export PATH=$A/root/usr/bin:$PATH
export __EGL_VENDOR_LIBRARY_DIRS=$A/root/usr/share/glvnd/egl_vendor.d LIBGL_DRIVERS_PATH=$L/dri GBM_BACKENDS_PATH=$L/gbm
export LIBGL_ALWAYS_SOFTWARE=1 MESA_LOADER_DRIVER_OVERRIDE=swrast
export XDG_RUNTIME_DIR=/tmp/dwx; mkdir -p $XDG_RUNTIME_DIR; chmod 700 $XDG_RUNTIME_DIR
```

The runtime directory must be a **short path**. With a long one, driftwm logged `IPC server failed to start: path must be shorter than SUN_LEN` (**TESTED**, `src/ipc` socket path under `$XDG_RUNTIME_DIR/driftwm/`).

Get the source (read-only, shallow, no Git LFS):

```
GIT_LFS_SKIP_SMUDGE=1 git clone --depth 1 https://github.com/malbiruk/driftwm /home/user/malbiruk/driftwm
git -C /home/user/malbiruk/driftwm fetch --depth=3000 origin main      # more history, for section 1
```

Run it nested on a virtual screen, with two terminal windows to play with (this is the "run" set-up used in later sections):

```
Xvfb :99 -screen 0 1280x800x24 -nolisten tcp &
DISPLAY=:99 $A/target/debug/driftwm --backend winit --config /dev/null > $A/dw.log 2>&1 &
export WAYLAND_DISPLAY=wayland-1 DISPLAY=:99
foot --app-id=term-a --title=First sleep 1500 &
foot --app-id=term-b --title=Second sleep 1500 &
xdotool windowfocus 2097154        # the id of driftwm's window, from:  xdotool search --name '.'
alias dw="$A/target/debug/driftwm"
```

---

## 1. What it is

| Item | Finding | Label |
|---|---|---|
| Language | Rust, edition 2024, needs Rust 1.88+ (`Cargo.toml:4-5`). Built here with rustc 1.97.0. | TESTED |
| Built on | **Smithay**, the Rust Wayland compositor toolkit, at a pinned *git revision*, not a release: `rev = "4cf0b62028039661477d482ec4758b687d8f4392"` (`Cargo.toml:14` and `:26`). Smithay features used: winit, udev, drm, gbm, libinput, libseat, egl, gles renderer, desktop (`Cargo.toml:14-25`). | BELIEVED (read) |
| Other main libraries | `libdisplay-info` 0.3, `cosmic-text` (title-bar text), `calloop` (event loop), `clap`, `serde`/`toml`, `inotify`, `sd-notify`, `image`, `tiff`, `xcursor` (`Cargo.toml:27-45`). 364 packages in `Cargo.lock`. | TESTED: `grep -c '^name = ' Cargo.lock` |
| Version | `0.19.0` (`Cargo.toml:3`). Release tags up to `v0.19.0`. Pre-1.0. | TESTED: `git ls-remote --tags origin \| grep -v '\^{}' \| sed -E 's#.*refs/tags/##' \| sort -V \| tail -6` gives `v0.17.0 v0.17.1 v0.17.2 v0.17.3 v0.18.0 v0.19.0` (34 tags in all: `git ls-remote --tags origin \| grep -vc '\^{}'`) |
| Licence | GPL-3.0-or-later (`Cargo.toml:6`, `LICENSE`). Copyright 2026 Klim Kostiuk. | BELIEVED (read) |
| Size | 203 Rust files. Compositor code 77,493 lines, in-process test fixture 53,956 lines, integration tests 5,912 lines. | TESTED: `git ls-files '*.rs' \| grep -v '^src/tests/' \| grep -v '^tests/' \| xargs cat \| wc -l`; `git ls-files 'src/tests/*.rs' \| xargs cat \| wc -l`; `git ls-files 'tests/*.rs' \| xargs cat \| wc -l` |
| Age and activity | 1,244 commits, first 2026-02-23, last 2026-09-22 (nine days before this research). Monthly commits: 34, 190, 121, 89, 130, 471, 117, 92 (Feb–Sep). | TESTED: `git rev-list --count HEAD`; `git log --format='%ad' --date=format:%Y-%m \| sort \| uniq -c` |
| Contributors | 29 author names (30 e-mail addresses). One maintainer account `malbiruk` has 1,137 commits and `Klim Kostiuk` has 35 more; 27 other names made 72 commits. `Klim Kostiuk` is the author in `Cargo.toml:7`, so the two are probably the same person (**BELIEVED**). In September 2026: 93 commits by `malbiruk`, 4 by others. | TESTED: `git log --format='%an' \| sort \| uniq -c \| sort -rn`; `git log --format='%an' \| sort -u \| wc -l`; `git log --since=2026-09-01 --format='%an' \| sort \| uniq -c` |
| Open issues, stars, users | Not read: GitHub's API is closed for repositories that are not attached to the session. | UNKNOWN |
| AI-assisted | The README states it (`README.md:21`: "This is experimental software, primarily built with AI."). So does `CONTRIBUTING.md:3` ("primarily an AI-assisted learning project"). The repository ships `AGENTS.md` (111 lines of instructions for AI coding agents) and a one-line `CLAUDE.md` that imports it. No `Co-authored-by:` line names an AI tool in the 1,244 commits. Nine commit messages contain one of the words claude, copilot, codex, gemini, chatgpt, AI, LLM or agent: six are changes to `CLAUDE.md`, `AGENTS.md` or the README, three are unrelated blur work. What share of the code is AI-written. | TESTED: `git log --format='%B' \| grep -ciE 'co-authored-by:.*(claude\|copilot\|gpt\|gemini\|codex\|cursor\|anthropic\|openai)'` prints `0`. Share of code: UNKNOWN. The nine: `git log -i -E --grep='claude\|chatgpt\|codex\|copilot\|gemini\|\bAI\b\|\bLLM\b\|agent' --format='%ad %h %s' --date=short` |
| Tests | Yes. In the checkout: `cargo test -- --include-ignored --skip soak` ran **2,033 tests, 2,033 passed, 0 failed** in 78 s, including the tests that start a real `foot` against the compositor and the ones that draw with a software OpenGL renderer. The separate soak test (churns hundreds of windows and checks for leaks) also passed. The project's own CI runs formatting, `cargo check --locked`, `clippy -D warnings`, and the tests on Ubuntu 24.04 (`.github/workflows/ci.yml:19-69`). | TESTED: `cd /home/user/malbiruk/driftwm && cargo test -- --include-ignored --skip soak`; `cargo test --bin driftwm -- --include-ignored --test-threads=1 soak` |

How the tests are organised (`dev/docs/testing.md`): unit tests inside the code, integration tests in `tests/`, random-input ("proptest") tests, and an in-process fixture in `src/tests/` that runs the whole compositor with no display and real Wayland test clients. This fixture is the only "headless" mode (section 3).

---

## 2. Systemd, logind, D-Bus, and how it reaches the screen and input

**Does it need systemd, logind or D-Bus?** **No.** It ran with none of them (**TESTED**, below). It *uses* them only in the places listed here, none of which stop it.

| Where | What it does | Result without systemd | Label |
|---|---|---|---|
| `src/main.rs:257-261` | At start it runs `/bin/sh -c "systemctl --user import-environment WAYLAND_DISPLAY XDG_CURRENT_DESKTOP XDG_SESSION_TYPE XDG_SESSION_DESKTOP; hash dbus-update-activation-environment 2>/dev/null && dbus-update-activation-environment <same names>"`. The result is only logged (`:262-273`). | Printed `Failed to connect to bus: No such file or directory` and `dbus-update-activation-environment: error: unable to connect to D-Bus: …`; the compositor carried on. | TESTED: the run command in section 0, then `sed -E 's/\x1b\[[0-9;]*m//g' $A/dw.log \| grep -E 'WARN\|ERROR\|Failed\|bus'` |
| `src/main.rs:279-281` | Sends "ready" to systemd with the pure-Rust `sd-notify` crate (`Cargo.toml:45`). | No warning was logged: with no notification socket the call does nothing. | TESTED: `grep -c READY $A/dw.log` printed `0` |
| `src/xwayland.rs:167-168` | After starting the X11 helper, runs `dbus-update-activation-environment DISPLAY` if that command exists. | Helper not installed in the test, so not reached. | BELIEVED (read) |
| `resources/driftwm.service`, `resources/driftwm-shutdown.target`, `Makefile:19-20` | systemd user units, installed by `make install`. | Not needed. We would not install them. | BELIEVED (read) |
| `resources/driftwm-session:29-62` | Session wrapper for display managers. Uses `systemctl --user` if `systemctl` exists, **else** `exec driftwm --backend udev` (the comment at `:59` names runit and OpenRC). | Non-systemd branch is just one line. | BELIEVED (read) |
| `Cargo.lock` | No D-Bus, logind, zbus or PipeWire crate. The only systemd-related crate is `sd-notify`. | | TESTED: `grep -E '^name = ".*(dbus\|zbus\|logind\|systemd\|pipewire\|pulse\|alsa).*"' Cargo.lock` prints only `sd-notify` |

On this image `systemctl` exists as a program but systemd is not running (pid 1 is `process_api`). If `systemctl` were absent the shell would print `not found` once and go on (**BELIEVED**).

**Libraries the compiled program loads.** Its own list: `libdisplay-info, libgbm, libseat, libudev, libinput, libxkbcommon, libgcc_s, libm, libc`. No systemd library appears in that list. **TESTED:** `readelf -d $A/target/debug/driftwm | grep NEEDED`. Two of those, as packaged by Ubuntu, pull in more: `libseat.so.1` needs `libsystemd.so.0` (for its logind support), and `libinput.so.10` needs `libwacom` which needs `libgudev`. **TESTED:** `for f in libseat.so.1 libinput.so.10; do readelf -d $L/$f \| grep NEEDED; done`. A Hub OS build of libseat without logind support would not need `libsystemd` (**BELIEVED**: `strings $L/libseat.so.1 \| grep -iE 'seatd\|logind\|builtin'` shows three backends are compiled in).

**How it gets the screen and input.** Only through **libseat**. `src/backend/udev.rs:414` creates a `LibSeatSession`; `:426` lists DRM devices with udev; `:471` opens each GPU through the session; `:608-609` hands the same session to libinput for input devices; `Cargo.toml:14-25` enables only the libseat session feature. `src/state/mod.rs:923` keeps the session for VT switching. A search found no other session type: **TESTED:** `grep -rn 'LibSeat' src --include=*.rs | grep -v '^src/tests'` prints only `src/backend/udev.rs:23`, `:414` and `src/state/mod.rs:121`, `:923`. What that means (there is no direct-open fallback) is **BELIEVED**.

**Can it run with `seatd` or with no session manager?** Four real runs on the real-display backend, none with a DRM device present:

```
driftwm --backend udev --config /dev/null                      # default libseat choice
LIBSEAT_BACKEND=seatd   driftwm --backend udev --config /dev/null
LIBSEAT_BACKEND=builtin driftwm --backend udev --config /dev/null
LIBSEAT_BACKEND=logind  driftwm --backend udev --config /dev/null
```

| Run | Result | Label |
|---|---|---|
| default | `Session created on seat: seat0`, then `GPU candidates: []` and `Error: "No GPUs found"` (exit 1). Neither seatd nor logind was available; libseat created a session on its own. | TESTED |
| `seatd` (no seatd running) | `Failed to create session … No such file or directory (os error 2)` | TESTED |
| `builtin` | Same as the default: session created, then `No GPUs found`. | TESTED |
| `logind` (no systemd) | `Failed to create session … No data available (os error 61)` | TESTED |

So a session can be created with no seat daemon and no systemd (as root, in this container). That is libseat's built-in backend (**BELIEVED**: its `strings` output includes `Started embedded seatd`). Running with a real `seatd` as a non-root user is the normal arrangement. **That path was not run** (no daemon was started).

VT switching (Ctrl+Alt+F1..F12) is handled in the compositor through the session (`src/input/keyboard.rs:121-132`) and always stays there, even when a window is passing keys (section 6). **BELIEVED**; pressing the combination in the nested test did nothing and did not crash it (**TESTED**: `xdotool key ctrl+alt+F2`, then `pgrep -x driftwm`).

---

## 3. Backends

**Two.** `src/main.rs:151-157` picks `winit` if `WAYLAND_DISPLAY` or `DISPLAY` is set, otherwise `udev`; `--backend` overrides. `src/main.rs:189-195`: `"udev"` is the real-display backend, **any other name** starts the nested one. **TESTED:** `driftwm --backend headless` started the winit code (log line `Initializing a winit backend`).

| Backend | What it is | Runs in the build environment? | Label |
|---|---|---|---|
| `winit` (nested) | A window inside another display server: X11 or Wayland (`src/backend/winit.rs`, 317 lines). | **Yes, on a virtual X display.** Details below. Inside a Wayland parent: not run. | X11: TESTED. Wayland parent: BELIEVED (`README.md` "Running"; same winit code) |
| `udev` (real display) | Drives a GPU directly through DRM/KMS with GBM and EGL, libinput for input, libseat for access (`src/backend/udev.rs`, 1,941 lines). | **No.** There is no `/dev/dri` here (`docs/environment.md`). It reached "session created" and stopped at `No GPUs found`. | TESTED (the failure above) |
| Headless | **No such backend at run time.** The only "headless" is the test fixture, where the compositor runs with `backend = None` and Wayland test clients over socket pairs (`dev/docs/testing.md`, `src/tests/headless.rs`). | Not a mode you can start. | BELIEVED (read), TESTED that other names fall through to winit |
| Others | None found in `src/backend/` (`cvt.rs`, `gamma.rs`, `udev.rs`, `winit.rs`, `mod.rs`). | | TESTED: `git ls-files src/backend` |

**Build.** Under the set-up in section 0:

```
cd /home/user/malbiruk/driftwm && cargo build
```

Result: `Finished 'dev' profile` after **103 seconds**, exit 0. The Ubuntu `libdisplay-info` 0.1.1 satisfied the `libdisplay-info` 0.3.0 crate. The debug program is 401 MB (not a release build). Smithay was fetched from GitHub by cargo with `CARGO_NET_GIT_FETCH_WITH_CLI=true`. **TESTED.**

**Run (winit on a virtual X display, software OpenGL).** With the "run" commands of section 0. First lines of the log:

```
Loaded config from /dev/null
Initializing a winit backend
Successfully selected EGL platform: PLATFORM_X11_KHR
EGL Initialized  /  EGL Version: (1, 5)
Creating new wl_output  output="winit"
Listening on WAYLAND_DISPLAY=wayland-1
IPC socket started at /tmp/dwx/driftwm/ipc-wayland-1.sock
xwayland-satellite not found … X11 apps disabled
Starting event loop — launch apps with: WAYLAND_DISPLAY=wayland-1 <app>
```

It then opened two `foot` windows, drew them with title bars on a dot-grid canvas, and answered every command in section 4. **TESTED.** A screenshot with `grim` is a 1280×800 PNG showing the two windows (**TESTED**: `grim $A/grim.png`).

Noise seen in the run (none stopped it): `error setting XSETTINGS` and `XRandR reported that the display's 0mm in size` (Xvfb); `[EGL] 0x300d (BAD_SURFACE) eglQuerySurface` repeated about every 5 seconds (cause **UNKNOWN**, **BELIEVED** to be this virtual-display set-up); the output's transform is reported as `flipped-180` by design (`src/backend/winit.rs:44`). Idle load of the debug build with software rendering: 169 MB resident, about 4.8% of one CPU over 10 seconds (**TESTED**: `grep VmRSS /proc/$(pgrep -x driftwm)/status`; counter difference of `/proc/$pid/stat` fields 14+15 over 10 s). These figures say nothing about a release build on a real GPU (**UNKNOWN**).

---

## 4. Control from outside

driftwm runs a small server on a Unix socket. The program `driftwm msg` is its client.

- **Socket:** `$XDG_RUNTIME_DIR/driftwm/ipc-<WAYLAND_DISPLAY>.sock`, mode `0600` (`docs/ipc.md` "Wire protocol"; `src/ipc/mod.rs:59`). `DRIFTWM_SOCKET` points a client elsewhere (`src/ipc/client.rs:305`). **TESTED:** the log line above; `python3` read `os.stat(sock).st_mode & 0o777` → `0o600`.
- **Wire format:** one JSON request per line, one JSON reply per line (`{"Ok":…}` or `{"Err":"…"}`). **TESTED**, no `driftwm` binary involved:

```
python3 - <<'EOF'
import socket,os
s=socket.socket(socket.AF_UNIX); s.connect(os.environ["XDG_RUNTIME_DIR"]+"/driftwm/ipc-wayland-1.sock")
for req in ['"State"','{"Zoom":null}','{"Move":{"window":1,"to":[300,100]}}','{"Focus":99}']:
    s.sendall((req+"\n").encode()); print(">",req); print("<",s.makefile().readline().strip())
EOF
```

Replies: the full state, `{"Ok":{"Zoom":1.0}}`, `{"Ok":{"Position":{"x":300,"y":100}}}`, and `{"Err":"no window with id 99"}`.

- **The socket is a full control surface.** `action` can run programs, quit and reload the config (`docs/cli.md`, `msg action`). That is why it is `0600`. **BELIEVED** (docs); not tested as another user.
- **State file:** `$XDG_RUNTIME_DIR/driftwm/state`, throttled to about 10 Hz (`docs/ipc.md` "State file"). **BELIEVED**; not read in the test.

### Every command

From `docs/cli.md` (generated from the program's own argument definitions, `docs/cli.md:3-4`). All commands accept `--json`. Window commands pick a window with `--id <n>` (from `state`) or by `app_id` substring (case-insensitive). Positions are the centre of the visible frame, **Y pointing up**.

Top-level program options: `--backend <udev|winit>`, `--config <PATH>`, `--check-config`, `--session-file <PATH>`, `-V/--version`.

| Command | Arguments | What it does | Label |
|---|---|---|---|
| `state` | none | Camera, zoom, every window (`id`, `app_id`, `title`, `position`, `size`, `is_focused`, `is_widget`, `suspended`, `mode`), layer-shell names, outputs. | TESTED: `dw msg state`, `dw msg --json state` |
| `subscribe` | none | Streams a whole-state snapshot on every change. | TESTED: `timeout 3 dw msg --json subscribe` while running `dw msg camera 200 100` produced 14 event lines |
| `focus` | `[APP_ID]` `[--id ID]` | Prints the focused window, or focuses one. Pans the camera to it unless already fully visible. | TESTED: `dw msg focus --id 0`, `dw msg focus` |
| `move` | `[X] [Y]` `[--id ID]` | Gets or sets a window's canvas position. | TESTED: `dw msg move -400 200 --id 0` → `-400 200`; `dw msg move --id 0`; `dw msg move 1 2 --id 99` → error `no window with id 99`, exit 1 |
| `resize` | `[WIDTH] [HEIGHT]` `[--id ID]` | Gets or sets the visible-frame size, clamped to the client's limits. | TESTED: `dw msg resize 640 480 --id 0` → `640 480` |
| `close` | `[APP_ID]` `[--id ID]` | Closes a window. | Documented, not run: BELIEVED |
| `opacity` | `[VALUE]` `[--id ID]` | Gets or sets 0–1 opacity. | Documented, not run: BELIEVED |
| `pin` | `[on\|off]` `[--id ID]` | Gets or sets "pinned to the screen". | Documented, not run: BELIEVED |
| `suspend` | `[APP_ID]` `[--id ID]` | Replaces a window by a stand-in that can bring the app back. | Documented, not run: BELIEVED |
| `relaunch` | `[APP_ID]` `[--id ID]` | Relaunches a stand-in's app. | Documented, not run: BELIEVED |
| `camera` | `[X] [Y]` | Gets or animates the viewport to a canvas point. | TESTED: `dw msg camera` → `25 -25.5`; `dw msg camera 1500 900` → `1500 900` |
| `zoom` | `[LEVEL]` | Gets or sets zoom (animated, clamped). | TESTED: `dw msg zoom 0.5` → `0.5` |
| `bookmark` | `[NAME] [X] [Y]` `[--delete]` | Lists, gets, sets or deletes named canvas points. | TESTED: `dw msg bookmark home 0 0`; `dw msg bookmark` listed `1`…`4` and `home` |
| `layout` | `[--short]` | Prints the active keyboard layout. | TESTED: printed an empty line in this nested run, while `state` shows `layout  (us)`; reason UNKNOWN |
| `action` | `<SPEC>...` | Runs any config action (the same text as in a key binding). | TESTED: `dw msg action fit-window` → `ok`; `dw msg action nonsense-action` → `unknown action: nonsense-action`, exit 1 |
| `screenshot` | `[--scale S] [-o FILE]`, subcommands `window [APP_ID] [--id]`, `all`, `region <X Y W H> [--from-screen]` | Renders a PNG of the canvas, off-screen parts included. | TESTED (viewport form only): `dw msg screenshot -o $A/shot.png` made a 31,887-byte PNG. `window`, `all`, `region`: BELIEVED |
| `debug-counters` | none | Internal collection sizes; keys unstable. | Documented, not run: BELIEVED |

### Can an outside program do each of these?

| Question | Answer | Label and command |
|---|---|---|
| (a) Place a window at a given canvas position | **Yes.** `move X Y --id N`. Also by a `position = [x, y]` window rule when the window first opens (`config.reference.toml:847-849`). | TESTED: `dw msg move -400 200 --id 0` then `dw msg state` showed `[-400, 200]`. The rule: BELIEVED, not run |
| (b) Move the view to a given window or position | **Yes.** `camera X Y` (position). `focus --id N` (window; pans to it). Bookmarks via `action go-to-bookmark NAME`. | TESTED: `dw msg camera 1500 900`; `dw msg focus --id 0` made the camera `-400 200`. `go-to-bookmark`: BELIEVED |
| (c) Focus or raise a given window | **Focus: yes.** **Raise: yes by the code**: `focus` calls `raise_and_focus` (`src/ipc/mod.rs:496-499`; defined `src/state/keyboard_focus.rs:48`), and `move` and `resize` raise too (`src/ipc/mod.rs:731-736`). | Focus: TESTED (`dw msg focus --id 0`, then `state` listed it first with `*`). Raise: BELIEVED (read), not seen on screen |
| (d) List windows | **Yes.** `state`, `--json state`, `subscribe`. | TESTED |
| (e) Set a window's size or maximize it | **Size: yes.** `resize W H --id N`. **Maximize: yes**, as "fit": `action fit-window` makes the window fill the viewport and a second use restores it. | TESTED: `dw msg resize 640 480 --id 0`; `dw msg action fit-window` made `term-b` `1280x800` with `mode` `"Fit"` |

Limits worth knowing:

- A window is found by `app_id` or title. What Moonlight, virt-viewer, Remmina and ssh terminals report as `app_id` is **UNKNOWN** (not run here). `hubd` can also watch `subscribe` for a new window id after it starts a viewer (**BELIEVED**).
- `move` and `resize` refuse pinned and fullscreen windows, and `resize` refuses widgets (`docs/ipc.md` "Coordinates").
- `camera` and `zoom` writes leave fullscreen first (`docs/ipc.md`).

---

## 5. Wayland protocols

**TESTED**, the list of everything the running compositor offers, from a client:

```
wayland-info | grep -E "^interface:" | sed -E "s/^interface: '([^']+)', +version: +([0-9]+).*/\1 v\2/" | sort
```

47 globals:

```
ext_background_effect_manager_v1 v1       ext_data_control_manager_v1 v1
ext_foreign_toplevel_image_capture_source_manager_v1 v1       ext_foreign_toplevel_list_v1 v1
ext_idle_notifier_v1 v2                   ext_image_copy_capture_manager_v1 v1
ext_output_image_capture_source_manager_v1 v1       ext_session_lock_manager_v1 v1
ext_workspace_manager_v1 v1               wl_compositor v6
wl_data_device_manager v3                 wl_output v4
wl_seat v9                                wl_shm v2
wl_subcompositor v1                       wp_content_type_manager_v1 v1
wp_cursor_shape_manager_v1 v2             wp_fractional_scale_manager_v1 v1
wp_presentation v2                        wp_security_context_manager_v1 v1
wp_single_pixel_buffer_manager_v1 v1      wp_viewporter v1
xdg_activation_v1 v1                      xdg_wm_base v7
xdg_wm_dialog_v1 v1                       zwlr_data_control_manager_v1 v2
zwlr_foreign_toplevel_manager_v1 v3       zwlr_gamma_control_manager_v1 v1
zwlr_layer_shell_v1 v5                    zwlr_output_manager_v1 v4
zwlr_output_power_manager_v1 v1           zwlr_screencopy_manager_v1 v3
zwp_idle_inhibit_manager_v1 v1            zwp_input_method_manager_v2 v1
zwp_keyboard_shortcuts_inhibit_manager_v1 v1       zwp_linux_dmabuf_v1 v3
zwp_pointer_constraints_v1 v1             zwp_pointer_gestures_v1 v3
zwp_primary_selection_device_manager_v1 v1       zwp_relative_pointer_manager_v1 v1
zwp_tablet_manager_v2 v1                  zwp_text_input_manager_v3 v1
zwp_virtual_keyboard_manager_v1 v1        zxdg_decoration_manager_v1 v1
zxdg_exporter_v2 v1                       zxdg_importer_v2 v1
zxdg_output_manager_v1 v3
```

The code that creates them is `src/state/init.rs:92-204`. Several are written in driftwm itself under `src/protocols/` (`foreign_toplevel`, `ext_workspace`, `screencopy`, `image_copy_capture`, `output_management`, `output_power`, `gamma_control`, `virtual_keyboard`). The `README.md:253` claim is "40+ Wayland protocols". The privileged protocols (layer shell, foreign toplevel, screencopy, virtual keyboard, data control, session lock and others) are hidden only from clients that connected through a security-context sandbox (`src/state/mod.rs:1099-1103`, `src/handlers/mod.rs:436`). **BELIEVED** (read).

The ones asked about:

| Protocol | Supported? | Label and evidence |
|---|---|---|
| **layer-shell** (needed for a status bar like Waybar) | **Yes**, `zwlr_layer_shell_v1` v5. Two real layer-shell programs mapped: `swaybg -c '#336699'` made `state` report `layers 1`; `fuzzel --dmenu` made it `layers 2`. **Waybar itself was not run**, and neither was a program that reserves screen space (exclusive zone). | Protocol: TESTED. Waybar and exclusive zones: UNKNOWN |
| **keyboard-shortcuts-inhibit** | **Advertised** (`zwp_keyboard_shortcuts_inhibit_manager_v1` v1) and a request is accepted and marked active (`src/handlers/mod.rs:402-426`). **But the key-handling code never asks whether shortcuts are inhibited.** | Advertised: TESTED. Not consulted: TESTED search `grep -rn 'keyboard_shortcuts_inhibited\|KeyboardShortcutsInhibitorSeat' src` finds nothing; the effect (a client's request does not stop compositor shortcuts) is BELIEVED from `src/input/keyboard.rs:84-183`. Not run with a client that sends the request. |
| **xdg-activation** | **Yes**, `xdg_activation_v1` v1 (`src/state/init.rs:117`). | Listed: TESTED. Behaviour: BELIEVED |
| **foreign-toplevel management** | **Yes**, `zwlr_foreign_toplevel_manager_v1` v3 and `ext_foreign_toplevel_list_v1` v1. The README says docks and taskbars see every window and clicking one pans to it (`README.md:225-227`). | Listed: TESTED. No taskbar run: behaviour UNKNOWN |
| **pointer constraints** | **Yes**, `zwp_pointer_constraints_v1` v1, with a handler written for pan/warp cases (`src/handlers/mod.rs:359-399`). | Listed: TESTED. With a game or Moonlight: UNKNOWN |
| **relative pointer** | **Yes**, `zwp_relative_pointer_manager_v1` v1. | Listed: TESTED |
| **virtual keyboard** | **Yes**, `zwp_virtual_keyboard_manager_v1` v1, a vendored copy of Smithay's with compositor key bindings added (`src/protocols/virtual_keyboard.rs:1-14`). | Listed: TESTED |
| **virtual pointer** | **No.** `zwlr_virtual_pointer_manager_v1` is not in the list, and nothing in the source mentions it. | TESTED: `grep -rniE 'virtual_pointer\|virtual-pointer\|VirtualPointer' src` prints nothing |

Also present and relevant: `zwp_pointer_gestures_v1` (unbound gestures are forwarded to the focused app, `config.reference.toml:583`), `zwlr_output_manager_v1` (**TESTED**: `wlr-randr` listed the one output, `1280x800 px, 60.000000 Hz`), `zwlr_screencopy_manager_v1` (**TESTED**: `grim $A/grim.png` made a 1280×800 PNG), `ext_session_lock_manager_v1`, `zwp_idle_inhibit_manager_v1`, `ext_idle_notifier_v1`, `zwp_linux_dmabuf_v1` v3, `wp_fractional_scale_manager_v1`, `wp_security_context_manager_v1`, `zwp_text_input_manager_v3`, `zwp_input_method_manager_v2`, `zwp_tablet_manager_v2`. Not offered (absent from the list): virtual pointer, KDE server-decoration, DRM lease, tearing control, DRM sync objects, drag-and-drop of toplevels. **TESTED** by their absence from `wayland-info`.

X11 programs go through a separate helper, `xwayland-satellite` (≥ 0.7), which must be installed and is started at launch (`src/xwayland.rs:1-60`, `dev/docs/caveats.md` "X11 apps run through xwayland-satellite"). It was not installed here: driftwm logged a warning and ran on (**TESTED**).

---

## 6. Keyboard shortcuts

**How they are set.** In the TOML file under `[keybindings]`: `"mod+key" = "action"`, with `mod` meaning the setting `mod_key` (`super` by default, or `alt` or `mod3`). Bare modifier chords are "tap" bindings. Bindings merge with the built-in ones; `"none"` removes one; `[bindings] disable_defaults = ["keys", "mouse", "gestures", "touch"]` drops a whole group (`config.reference.toml:379-507`). The full action list is `config.reference.toml:397-434`. Hot-reload of the file uses inotify (`src/main.rs:295-378`, **BELIEVED**, not run). **TESTED:** `dw --config /tmp/dwx/test.toml --check-config` printed `Config OK`.

**Can a client such as Moonlight take all keys?** Yes, with a rule, per window.

```
# /tmp/dwx/test.toml
[[window_rules]]
app_id = "term-a"
pass_keys = ["mod+q"]
```

- `pass_keys = true` forwards **every** key to the focused window except the VT-switch combos (`src/input/keyboard.rs:121-153`; `docs/window-rules.md` `pass_keys`).
- `pass_keys = ["combo", …]` forwards **only** those combos; all other compositor shortcuts stay active.
- `pass_mouse` is the same for the `[mouse.*]` bindings.

**TESTED** (nested, `xdotool` sends the keys; focus first with `dw msg focus term-a`, give driftwm's X window focus with `xdotool windowfocus 2097154`):

| Setting on `term-a` | Key | Result |
|---|---|---|
| `pass_keys = ["mod+q"]` | `xdotool key super+q` | `term-a` **stayed open** (key forwarded). |
| same | `xdotool key super+m` | `term-a` became `Fit` (compositor still handled it). |
| none, on `term-b` | `xdotool key super+q` | `term-b` **closed**. |
| `pass_keys = true` | `xdotool key super+m`, then `super+q` | Nothing happened to the window in either case (both keys forwarded). |

**Can the compositor keep chosen shortcuts while the client takes the rest?** **No, not in one rule.** The two forms are "only these are forwarded" and "everything is forwarded". There is no "everything except these" form in the documented syntax (`config.reference.toml:931-935`). **BELIEVED** (read). To keep a few shortcuts you would have to list every other combination as the `pass_keys` list, which is impractical. With `pass_keys = true` the way out is the mouse: click outside the window (as `HUB-OS.md` already plans).

**The shortcut-inhibit protocol does not do this job** (section 5): a client may ask, and driftwm accepts, but the key handler does not check. A real Moonlight request was not tried. If the owner wants the protocol honoured, that is a small change at `src/input/keyboard.rs:84-183`: ask the seat whether shortcuts are inhibited for the focused surface (Smithay provides `KeyboardShortcutsInhibitorSeat::keyboard_shortcuts_inhibited`) before looking up bindings. **BELIEVED**; not written.

---

## 7. Mouse-only use

**Mostly yes.** Pan, zoom, move, resize, fit, fullscreen and jump-to-window were all run with an ordinary mouse (left, middle, right buttons and wheel) plus the held modifier keys the default bindings need. The test used `xdotool` to press buttons, wheel and move; the compositor reads those as ordinary pointer events.

| Action | Mouse form (default bindings, `config.reference.toml:509-570`) | Label and command |
|---|---|---|
| Pan | Left button drag on **empty canvas** | TESTED: `xdotool mousemove 1200 100 mousedown 1`, three `mousemove_relative -- -100 0`, `mouseup 1` moved the camera x from `25` to `325` |
| Pan over a window | `mod`(Super) + left drag | TESTED: `xdotool keydown super; … mousedown 1; … mouseup 1` moved the camera x `0 → 200` |
| Zoom out / in | Wheel on empty canvas | TESTED: `xdotool click 5` six times → zoom `0.75`. `xdotool click 4` at zoom 1.0 changed nothing (the 100% cap) |
| Zoom over a window | `mod` + wheel | TESTED: `xdotool keydown super; xdotool click 5` ×4 → zoom `0.826` |
| Move a window | `alt` + left drag | TESTED: `[25, -25]` → `[175, -125]` |
| Resize a window | `alt` + right drag | TESTED: `xdotool keydown alt … mousedown 3`, two `mousemove_relative -- 40 40`, `mouseup 3`; the size changed from `700x525` to `700x445` |
| Fit / maximize | `alt` + middle click (again restores) | TESTED: `mode` `Fit`, `1280x800`, then back to `Normal` |
| Fullscreen | `mod` + middle click | TESTED: `fullscreen 1`, then `0` |
| Jump to a window by clicking | With `[zoom] interact_min = 0.9`, a left click on a window while zoomed out below that centres on it and **resets zoom to 1**. | TESTED: zoom `0.5`, click on the far window → camera `1500 -0.5`, zoom `1` |
| Jump to nearest window | `mod`+`ctrl` + left drag (direction of the drag) | The test run was not conclusive: UNKNOWN |

**Trackpad-only?** The gestures (3- and 4-finger swipe, pinch, hold) are trackpad/touch only, but each has a key or mouse form (`README.md` sections "Pan & zoom" to "Touchscreen", lines 25-130), apart from a few that default to keys only:

| Gesture action | Default non-gesture form | Mouse-only form |
|---|---|---|
| Zoom-to-fit (overview) | key `mod+w` | none by default. Can be bound to a hot corner (`config.reference.toml:822-829`) or to the wheel / a button (`:535-537`). BELIEVED, not run |
| Home toggle | key `mod+a` | same |
| Center window | key `mod+c` | the click-to-jump setting above |
| Bookmarks 1–4 | keys `mod+1..4` | same as the first row |
| Cycle windows | `alt+tab` | none; click a window instead |

Caveat for the Hub OS plan: most mouse bindings need a **held modifier** (Alt or Super). `HUB-OS.md` says one keyboard and one mouse are shared by forwarding. Whether the forwarder can hold modifiers together with mouse buttons is **UNKNOWN** (the forwarder does not exist yet). The no-modifier forms are empty-canvas left-drag (pan) and empty-canvas wheel (zoom), plus click-to-jump.

---

## 8. Zoom and text

- **How zoom is drawn.** The window's existing picture is **stretched by the GPU**; the program is not re-drawn at the new size. `src/render/mod.rs:241-246` wraps each window's render element in `PixelSnapRescaleElement::from_element(elem, origin, zoom)`. The client is never told the zoom: its only scale hint is the output's fractional scale (`src/handlers/mod.rs:232-239`). **BELIEVED** (read).
- **Filter.** Smithay's renderer defaults to bilinear (`Linear`) for shrinking and growing (Smithay `src/backend/renderer/gles/mod.rs:734-735`). driftwm never changes it (`grep -rn 'downscale_filter\|upscale_filter' src` prints nothing) and no mipmap code was found. So heavy zoom-out is a plain bilinear shrink and will look rough. **BELIEVED**.
- **Zoom limits.** `MAX_ZOOM = 1.0`, with the comment "100% — native resolution, no magnification" (`src/canvas.rs:11-12`). The minimum depends on the layout: half of what fits all windows (`src/canvas.rs:458-467`). **TESTED:** with two windows, `dw msg zoom 0.3` left the zoom at `0.5`. A mouse-wheel zoom-in at 1.0 changed nothing (section 7).
- **What it looks like.** **TESTED:** `grim` screenshots of the same two `foot` windows at zoom 1.0 (`grim $A/grim.png`) and at zoom 0.5 (`dw msg zoom 0.5`, then `grim $A/z050.png`), both looked at as images. At 0.5 the text is half-size but still readable on a 1280×800 virtual screen. Readability on the real projector and from a seat is **UNKNOWN**.
- **What it means for a projector.** Text can only get smaller than native, never larger, from the compositor's zoom. To make text bigger the options are (a) the output's fractional `scale` (`config.reference.toml:785`, per output) so clients draw larger text (**BELIEVED**: windows are told the output scale; not run on a real output), or (b) the apps' own font size.
- **Per-window scaling.** **None.** The window-rule fields (`config.reference.toml:841-946`) contain no scale field, and the scale hint is per output. **BELIEVED.**
- **Other drawing notes.** Backgrounds are shaders (a dot grid by default), drawn in canvas space; windows get a title bar (server-side decoration) unless the client draws its own (`AGENTS.md:98`). Not relevant to the Hub OS plan except as noise.

---

## 9. Configuration

- **Format:** TOML. **Location:** `$XDG_CONFIG_HOME/driftwm/config.toml`, else `~/.config/driftwm/config.toml`; `--config PATH` or `DRIFTWM_CONFIG` override (`src/config/toml.rs:447-461`). There is **no system-wide path**. A missing file means built-in defaults. A partial file merges with the defaults. `driftwm --check-config` validates and exits. **TESTED:** `XDG_CONFIG_HOME=$A/cfg dw --check-config` → `Config OK`, exit 0; `dw --config /home/user/malbiruk/driftwm/config.reference.toml --check-config` → `Config OK`, exit 0.
- **Why this suits Hub OS:** the file can live with the machine's other per-machine settings and be named with `--config`. (**BELIEVED**.)
- **What can be set** (`config.reference.toml`, 988 lines; `docs/config.md` is generated from it):

| Section | Examples |
|---|---|
| General (`:15-85`) | `mod_key`, `focus_follows_mouse`, `window_placement`, `focus_placement`, `autostart` |
| `[session]` (`:87`) | suspend on close, restore windows/camera/bookmarks |
| `[env]` (`:116`) | environment for child programs |
| `[input.keyboard/trackpad/mouse/touch/tablet]` (`:125-171`) | layout, repeat, trackpad acceleration/tap, mouse speed, touch mapping |
| `[cursor]`, `[navigation]`, `[zoom]`, `[snap]` (`:173-256`) | speeds, momentum, anchors, bookmarks, edge pan, zoom step, snap distances |
| `[decorations]`, `[effects]`, `[background]` (`:258-377`) | title bar, borders, shadows, blur, wallpaper/shader |
| `[bindings]`, `[keybindings]`, `[mouse*]`, `[gestures*]`, `[touch*]` (`:379-711`) | every binding |
| `[xwayland]` (`:713`), `[backend]` (`:721`), `[output.outline]` (`:750`) | X11 helper path, hardware quirks, outline of other monitors |
| `[[outputs]]` (`:755-829`) | per monitor: `scale`, `transform`, `position`, `mode` (`"WxH@Hz"` allowed), `focus_placement`, hot corners |
| `[[window_rules]]` (`:831-988`) | match by `app_id`/`title` (exact, glob or `/regex/`); `position`, `size`, `fullscreen`, `focus_on_open`, `widget`, `pinned_to_screen`, `decoration`, `opacity`, `blur`, borders, `output`, `pass_keys`, `pass_mouse`, `layer_order`, `suspend_on_close`, `restore_windows` |

A "fixed home position per machine" (`HUB-OS.md`) maps onto `position` in a window rule keyed on the viewer's `app_id` or title, or onto `driftwm msg move` after the window appears. **BELIEVED**; the `move` form was run (section 4).

---

## 10. Assumptions about laptops, touchpads, batteries, GPUs and distributions

- **Laptop and trackpad: yes in design and defaults, not in the code's logic.** `README.md:16`: "Designed with laptops in mind: navigation and window management are trackpad-first". The default gesture bindings are trackpad ones (`config.reference.toml:572-638`), trackpad speeds default to 1.5 (`:179`), and the tablet setting falls back to "the internal panel" (`:168-171`). All of it is configuration. **BELIEVED** (read).
- **Battery, brightness, lid:** none in the compositor. A search of the source found no battery or backlight code; `XF86MonBrightness*` and the volume keys only *start external programs*: `brightnessctl`, `wpctl`, `playerctl`, `swaylock`, `grim`, `wl-copy` (`config.reference.toml:480-493`, `src/config/defaults.rs:317-331`). If those programs are missing the key does nothing visible. **TESTED search:** `grep -rniE 'battery|brightness|backlight|\blid\b|laptop' src --include=*.rs` prints only configuration names and comments. The battery/Wi-Fi/Bluetooth widgets belong to the optional QML "home dashboard" in `extras/` (`README.md` "Example setup"), not to the compositor.
- **Default terminal and launcher:** the first found of `foot, alacritty, ptyxis, kitty, wezterm, gnome-terminal, konsole` and `fuzzel, wofi, rofi, bemenu-run, wmenu-run, tofi-drun, mew-run` (`config.reference.toml:437-438`). Overridable.
- **GPU model:** no logic depends on a vendor. There are comments about Apple (`src/backend/udev.rs:124`) and NVIDIA (`:514`) and an opt-in `[backend]` section of quirks for NVIDIA (`config.reference.toml:721-748`). The code picks the "primary" GPU, then tries each other GPU until one has a connected display (`src/backend/udev.rs:425-493`), so **a display must be connected when it starts** (**BELIEVED**). **TESTED search:** `grep -rniE 'nvidia|amdgpu|i915|radeon|asahi|panfrost|nouveau' src --include=*.rs | grep -v '^src/tests'` finds only those comments.
- **Distribution:** no distribution is assumed in code. The README lists Arch, Fedora, Debian/Ubuntu and Nix build steps (`README.md:334-365`), CI uses Ubuntu 24.04. **TESTED search:** `grep -rniE 'fedora|ubuntu|debian|pacman|dnf|/etc/os-release' src Makefile resources` finds nothing.

---

## 11. What could stop it on glibc, no systemd, s6 or dinit, and PipeWire

| Concern | Finding | Label |
|---|---|---|
| glibc | Builds and runs on glibc (Ubuntu 24.04). musl was not tried. | glibc: TESTED. musl: UNKNOWN |
| No systemd | Runs. See section 2. | TESTED |
| s6 / dinit | Nothing in driftwm depends on an init. The one thing a service needs is what `resources/driftwm-session:58-62` does in its non-systemd branch: log in as a user, set the environment, `exec driftwm --backend udev`. We would need a run script that creates `XDG_RUNTIME_DIR` (a short path), starts `seatd` (or runs as a user libseat can serve), and sets `LIBSEAT_BACKEND`. No such script was written or run. | BELIEVED |
| PipeWire | The compositor does not use PipeWire (no crate, no code; only the default volume keys start `wpctl`). Screen-casting through portals needs PipeWire but `HUB-OS.md` does not use it (Moonlight and Sunshine capture on the nodes). PipeWire ran on this machine without systemd (`docs/environment.md`, section 1.13). | TESTED (no PipeWire crate: the `Cargo.lock` search in section 2) |
| `libudev` | Needs a library with the `libudev` interface, as udev or `eudev` provide. Whether a non-systemd one works with the Rust `udev` crate was not tried. | UNKNOWN |
| Build inputs | Rust 1.88+ (`README.md:336`); a Smithay *git* revision (`Cargo.toml:14`), so a network build needs GitHub, or the offline route: every release has a `driftwm-<version>-vendor.tar.xz` with all dependencies (`README.md` "Offline build"). The Hub OS image builder would need Rust or a pre-built program. | BELIEVED (read), build with network: TESTED |
| Runtime graphics | Needs EGL, GBM and a DRM device. Tested only with Mesa's software renderer on a virtual display. | UNKNOWN on real GPUs |
| Real-display start | Needs a seat, a GPU with a connected display, and (for a non-root user) a seat daemon. | UNKNOWN beyond the four failures in section 2 |

---

## 12. Things not checked here, and what each would need

| Not checked | What it needs |
|---|---|
| The real-display backend driving a screen or projector | A machine with a GPU and display, or a virtual machine with a virtual GPU (for example `virtio-gpu`) whose kernel has DRM and input support. Under QEMU emulation with no KVM (`docs/environment.md`) this would be slow but possible (**BELIEVED**). |
| Input from real devices through libinput; hot-plug of screens | The same. |
| Pointer constraints, relative pointer and shortcut-inhibit with a real Moonlight/Sunshine session | A Moonlight client, a Sunshine host and a GPU. Neither package is in the Ubuntu archive. |
| Waybar, and any bar that reserves space (exclusive zones) | Install Waybar (31 packages) and run it; not done here. |
| `app_id` and titles of Moonlight, virt-viewer/Remmina, an ssh terminal | Those programs run against the compositor. |
| A Wayland parent for the nested backend | A Wayland compositor with working OpenGL. |
| Fractional output scale and the look of text on the projector | The projector, or at least a display with the same pixel size. |
| A release build's speed, memory and idle CPU | `cargo build --release` on the target hardware. The debug build's figures in section 3 do not apply. |
| `seatd` as the seat manager; running as a non-root user | Start `seatd`, a user in its group. |
| s6 / dinit start-up script | Write and run it in a VM. |
| musl; other architectures | A different toolchain. |
| Multi-monitor, touch and tablet | Hardware. |
| Config hot-reload; session save/restore; `suspend-window` | Run with a `.desktop` entry present and edit the config while it runs. |
| The `xwayland-satellite` helper | Install it (not packaged for Debian/Ubuntu per the README, `README.md:440`; `cargo install --locked xwayland-satellite`). |
| How much driftwm will change | Follow its releases; pin a commit. |

## 13. Questions to settle with the owner

1. Is it acceptable that driftwm is a single-maintainer, pre-1.0, AI-built project, with the pinned-commit and maybe carried-patches cost that implies?
2. Should keyboard-shortcut-inhibit be honoured by a small patch, or is the per-window `pass_keys` rule enough?
3. Is a "no magnification" compositor acceptable for the projector, given text can only be enlarged by output scale or app fonts?
4. Is a short test in a virtual machine with a virtual GPU wanted before December, or does the real-display test wait for Phase C?
5. Which Moonlight, virt-viewer/Remmina and terminal programs will be used, so their window names can be tested?
6. Is Waybar (or another bar) the intended status bar, so it can be run against driftwm?
