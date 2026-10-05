# PROPOSAL: window layouts on the hub, saved by name and restored by name

**Status: PROPOSAL, research only.** Nothing here is decided and nothing here is built. `HUB-OS.md` wins if this file disagrees with it. Written 2026-10-05. The tests used driftwm at the pinned commit `352333a8fa1b22171492d4b71a54102045c9a19d` (version 0.19.0). The test scripts and the full output of the runs are in `tools/image/experiments/layouts-and-file-transfer/` (`t/` and `results.txt`; "results L4" below means the section "L4" of that file).

**What the owner asked for:** window layouts are saved by name and restored; they live on the config partition; windows never reopen by themselves after a reboot.

**Labels on every item:**
- **TESTED**: I ran it in this build environment; the command or script, and what came out, are given (or are in `results.txt`).
- **SOURCE**: read in a file at a named place (driftwm files are at the pinned commit, read 2026-10-05; "(repo)" means a file of this repository written by an earlier session, not re-checked by me).
- **BELIEVED**: I think it is true, I did not test it and have no source.
- **UNKNOWN**: nobody has checked; do not rely on it.

---

## Plain-words summary

1. **driftwm already has a "remember my windows" feature, and it works as its manual says.** It can leave a grey stand-in (a "suspended window") where a window was. It can also keep a small file, `session.json`, up to date while you work, and after a restart it puts the stand-ins back at the same places and sizes. Nothing starts by itself; you click a stand-in or press Enter to start its program. I tested all of this in a nested driftwm, including killing the compositor with `kill -9`. (TESTED, section 2.)
2. **After `kill -9` the saved file is intact and the next start restores from it.** The file is written about one second after a window change (five seconds after panning or zooming) and is never written at exit, so the last second or so of changes is lost. The leftovers of the killed compositor are a dead control socket, a dead status file and the two Wayland socket files; the next start took no notice of them. Every program that was on the canvas lost its connection and exited (foot did; real viewers are UNKNOWN). (TESTED, section 2.3.)
3. **It does not do what the owner needs, in four ways.** (a) There are **no named layouts**: one file, one session, no "save as work" and no "load home". (b) It records a program's identity (its `app_id`), **not which machine** the window shows. Two windows of one viewer program look the same to it, and "relaunch" starts the same fixed command for both, so the machine is lost (TESTED, section 2.6). (c) The saved camera and zoom are applied only on the **real-display backend**, not in the nested one I could run, so I could not test that part (SOURCE; TESTED that the nested run ignores it). (d) The **debug build crashed** each time I relaunched a stand-in (a debug-only self-check failed); the **release build did not** (TESTED, section 2.5). Nobody has looked at why.
4. **Proposal.** Leave driftwm's own restore switches **off** on the hub (the defaults). Let **hubd own the layouts**: one small text file per layout in `/config/hubos/layouts/`, saved only when the owner asks, keyed by the **machine id** from the inventory (not by driftwm's window number or program name). "Apply layout" moves and resizes the machine windows that are **already open**, using the driftwm socket commands that hubd already uses (`move`, `resize`, `camera`, `zoom`). It never opens a window, so nothing reopens after a reboot. A layout is the same idea as the inventory's `home`, with more than one position per machine; while a layout is active it overrides `home`. (Section 5 and 6.)
5. **Open questions for the owner** are in section 9 (what a layout contains, whether it is remembered across a hub reboot, who may write to the config partition, backup, how it is started from the panel).

---

## 1. What driftwm saves and restores

**Source for this section:** `docs/session.md` and `src/session.rs`, `src/state/session_store.rs` of the pinned commit (SOURCE, read 2026-10-05).

### 1.1 The switches

| Switch (in `[session]` of driftwm's config) | Default | What it does | Label |
|---|---|---|---|
| `suspend_on_close` | off | A window closed by its program (title-bar X, quit, crash) is replaced by a stand-in at the same place. Closing with the `close-window` action or `driftwm msg close` still really closes. | SOURCE `docs/session.md`; TESTED (results L1-L3: after the program was killed the stand-in appeared; `msg close` removed it) |
| `restore_windows` | off | Windows that are still open are written to the file as you work; after a restart they come back as dormant stand-ins. A window rule `restore_windows = true/false` overrides it per program. | SOURCE; TESTED (results L1-L3, L6 E) |
| `restore_camera` | off | The saved camera position and zoom of each screen are applied at start. | SOURCE; **only the real-display backend applies it** (section 1.4) |
| `restore_bookmarks` | off | The named canvas points (bookmarks) saved in the file are put back at start. | SOURCE; TESTED (a bookmark `myplace` set before `kill -9` was listed after the restart: results L1-L3) |

Stand-ins that exist (made by `suspend-window`, the `suspend` socket command, or `suspend_on_close`) are **always** saved and restored, whatever the switches say. TESTED: with all switches at their defaults, a window suspended by hand came back after `kill -9` (results L4).

A stand-in exists only for a program driftwm can start again: the window's `app_id` must match an installed `.desktop` file (`Terminal=true` entries do not count). Otherwise the action "closes the window normally" and leaves no stand-in. SOURCE `docs/session.md`. TESTED: a window with an `app_id` that has no `.desktop` file was **not** written to the file (results L1-L3: the file held only `term-a` and `term-b`, not `nodesk`).

### 1.2 What is written, and where

The file is `session.json`, "version 2", pretty-printed JSON. It is written to `$XDG_STATE_HOME/driftwm/session.json`, else `$HOME/.local/state/driftwm/session.json`; if neither variable is set, **nothing is saved** (SOURCE `src/session.rs:106-113`). The nested (test) driftwm saves nothing unless started with `--session-file PATH`; the real-display backend always uses the default path (SOURCE `src/main.rs:181-184`). The hub image starts driftwm with `XDG_STATE_HOME=/run/hub/state`, which is memory (SOURCE (repo) `image/machines/hub/rootfs/usr/lib/hubos/hub-env.sh`).

| Saved item | Exact content | Label |
|---|---|---|
| One record per window, bottom to top (the order in the file is the stacking order) | `id` (only to tell records apart in one write), `app_id`, `desktop_id`, `display_name` (from the `.desktop` file), `position` and `size`, `origin`, `csd`, `focused` | SOURCE `src/session.rs:43-76`; TESTED (the real file in results L1-L3) |
| `position`, `size` | The **visible frame** (window plus its title bar and border); `position` is its **centre**, **Y pointing up**. The same convention as `driftwm msg state`, the window rules and the inventory's `home`. TESTED: a window set with `msg move 300 200` and `msg resize 640 480` was saved as `"position":[300,200]`, `"size":[640,480]`. | SOURCE; TESTED |
| `origin` | `explicit` (a stand-in made on purpose or by `suspend_on_close`: always restored) or `quit` (a window that was open at the last save: restored only if `restore_windows` is on for that program) | SOURCE; TESTED (both values seen) |
| `focused` | At most one record carries it; the restored stand-in gets the focus ring, and Enter on it relaunches. Applied only if the stand-in is visible at start. | SOURCE; TESTED (the focus came back in most runs; in one run it did not, probably because the stand-in was not visible at start, as the source says; not checked) |
| Per screen (key = the screen's name, e.g. `winit` in the nested run, a connector name such as `DP-1` on real hardware) | `camera` (the **top-left corner** of the view, **Y pointing down**, so *not* the same numbers as `msg camera`) and `zoom` | SOURCE `src/session.rs:61-66`, `src/state/session_store.rs` (`CameraSeed::Camera`); TESTED: view centre (120, 80) at zoom 0.8 on a 1280x800 screen was saved as `[-680.0, -580.0]` and `0.8`, which is 120 - 1280/0.8/2 and -80 - 800/0.8/2 |
| Bookmarks | name to `[x, y]` (Y up). The four default bookmarks `1` to `4` are saved too. | SOURCE; TESTED |

**Not saved:** the window title, the command line or arguments used to start the program, the window's mode (fit, fullscreen or pinned: such a window is saved at its normal size), opacity, any rule state, and which hub machine the window shows. (SOURCE: absent from the struct; TESTED: the file shows exactly these keys.)

### 1.3 When it is written

- A change to a window (create, move, resize, focus, suspend, dismiss) starts a **one second** timer; a pan or zoom starts a **five second** timer; the nearer timer wins. SOURCE `src/state/session_store.rs:43,52` and the `session_store_mark_dirty*` functions.
- **Nothing is written at exit** (on purpose, so a logout does not save a half-drained desktop). SOURCE `docs/session.md`. TESTED for the polite case: a window moved and then `kill -TERM` at once left the old position in the file (results L6 D). `kill -9` is the same code path (nothing runs at exit); SOURCE, not separately timed.
- The write is "write `session.json.tmp`, then rename it over `session.json`" **without `fsync`**, so a crash or `kill -9` loses nothing that was already written, but "a power cut at the wrong moment can leave it half-written" (SOURCE `src/session.rs:151-175`, `docs/session.md`). A file that no longer parses is renamed `session.json.corrupt.<time>` and the session starts empty; a file from a newer version is treated the same; an unreadable one becomes `session.json.unreadable.<time>`. TESTED: a garbage file and a `"version":999` file were both set aside as `session.json.corrupt.<time>` and the compositor started normally (results L6 A, B).
- Because of the temporary file, **the folder must be writable, not only the file** (SOURCE).

### 1.4 What is restored, and the part that only works on a real screen

On start, driftwm reads the file and makes a stand-in for each record that is `explicit`, or `quit` with `restore_windows` on for that `app_id`. It does not start any program (TESTED, section 2.2). Bookmarks are restored if `restore_bookmarks` is on (TESTED).

**The camera and zoom are applied only by the real-display (`udev`) backend.** `saved_camera_state()` is called in `src/backend/udev.rs` (lines 641 and 876) and nowhere else, and the winit backend has no such call (SOURCE; TESTED search: `grep -rn saved_camera_state src` finds only the definition, one comment and these two calls). TESTED: in the nested run with `restore_camera = true` the view came back at the default (camera 0, 0 and zoom 1) after a restart, and the first write after that **overwrote the saved camera with the default** (results L1-L3: `"camera":[-640.0,-400.0],"zoom":1.0`). So whether `restore_camera` works on the hub is **UNKNOWN** (no GPU here). It does not matter for the proposal, which sets the camera itself (section 5).

---

## 2. Tests in the nested driftwm

### 2.1 How they were run

- **driftwm** was built from the pinned commit (a shallow clone checked out to `352333a8...`): a debug build (`cargo build`, 2 min 45 s) and a release build (`cargo build --release`, 7 min 11 s with `-j 3` while other jobs ran). Libraries came from `apt-get install --print-uris`, `curl` and `dpkg -x` into a work folder; nothing was installed on the machine (`setup.sh`, `ud.sh`). TESTED.
- **The nested driftwm needs a parent display.** The earlier note in `docs/driftwm-findings.md` used Xvfb. Xvfb does not start here: `/usr/bin/xkbcomp` is not installed and the path is built into the server (TESTED: `Xvfb :99` printed `/usr/bin/xkbcomp: not found` and `Failed to activate virtual core keyboard`). I did not work around that (a mount trick I tried was refused by the environment's permission check and I stopped). Instead the parent is **sway 1.9 with its headless backend** (`WLR_BACKENDS=headless WLR_RENDERER=pixman`), unpacked from the Ubuntu packages the same way, and driftwm's nested backend ran on it as a Wayland client with software OpenGL. That is a different parent from the findings file, so every result is a nested-on-sway result. TESTED.
- **Test programs:** `foot` windows with fixed `--app-id` and `--title`; two small `.desktop` files (`term-a`, `term-b`) in a private `XDG_DATA_HOME` so that `suspend` and `relaunch` find a program to start.
- **Which binary:** release for everything unless a line says debug.
- A start used `--session-file $LFT/t/state/driftwm/session.json` (nested driftwm needs it to save anything). The driftwm socket was driven with `driftwm msg ...`.

### 2.2 Save, then `kill -9`, then restart (all four switches on) — results L1-L3

Script `t/t1.sh`, `t/t2.sh`, `t/t3.sh` (run together by `t/rel123.sh`). Two `.desktop`-backed windows and one without. Windows set to known places with `msg move` and `msg resize`, a bookmark added, 8 seconds of waiting, then `kill -9` of the driftwm process.

| What | Result | Label |
|---|---|---|
| The file just before the kill | Two records (`term-a` at `[300,200]` size `640x480`; `term-b` at `[-500,-300]` size `500x350`, focused), the output `winit` with camera and zoom, and the five bookmarks. The window without a `.desktop` file is not in it. | TESTED |
| After the kill | `session.json` still there and unchanged (1010 bytes); no `.tmp` file. | TESTED |
| After the restart | `driftwm msg state` listed **two windows, both with `suspended: true`, at exactly the saved positions and sizes**, in the saved order, one with the focus. No program was started: `pgrep foot` showed none of ours. | TESTED |
| Bookmarks | All five were back (`restore_bookmarks = true`). | TESTED |
| Camera and zoom | Not restored in the nested run (section 1.4). | TESTED |
| `driftwm msg relaunch term-a` | The `.desktop` command ran; the new window **took the stand-in's place and size** (`term-a [300, 200] 640x480 "First"`). | TESTED |
| Then the program was killed | The window turned into a stand-in again; the file now shows it with `origin: explicit`. | TESTED |
| `driftwm msg close --id 0` | The stand-in was dismissed and the file lost its record. | TESTED |

Pressing **Enter** on a stand-in, or **clicking** its name, was **not** tested: I had no way to inject keys or clicks into the nested compositor (no `xdotool`, no virtual keyboard). `msg relaunch` is the same relaunch function (SOURCE `docs/session.md`, "Relaunching & matching"), but the key and click paths are **UNKNOWN**.

### 2.3 What a `kill -9` leaves behind

| Left behind | Detail | Label |
|---|---|---|
| `session.json` | Intact, the last rolling save. Up to about one second of window changes, or five seconds of panning, may be missing. | TESTED (intact); SOURCE (the timing) |
| A dead control socket | `$XDG_RUNTIME_DIR/driftwm/ipc-wayland-2.sock` stays; `driftwm msg` says `Connection refused`. | TESTED |
| The status file | `$XDG_RUNTIME_DIR/driftwm/state` stays (it is also in memory, in `/run`). | TESTED |
| Two Wayland socket files | `wayland-2` and `wayland-2.lock` stay. A second start **without any cleanup** reused the name `wayland-2` and the same control socket path without trouble (results L8). The hub's driftwm start script removes them anyway (SOURCE (repo) `image/machines/hub/rootfs/etc/s6/sv/driftwm/run`). | TESTED |
| The programs on the canvas | Every `foot` window process had exited a moment later. For real viewers (remote-viewer, Moonlight, a VNC viewer) nobody has checked: **UNKNOWN**. | TESTED (foot); UNKNOWN (viewers) |
| Windows on the next start | None (nothing reopens) except stand-ins from the file, and only those the switches allow. | TESTED |

### 2.4 Default switches (all off), with `--session-file` — results L4

One window was suspended by hand, one stayed live, then `kill -9`. After the restart: **only the stand-in came back**, at `[-500,-300]` size `500x350`; the live window was gone. `msg relaunch term-a` then brought the program back into the stand-in's slot. TESTED. The defaults are therefore already the "windows never reopen by themselves" behaviour the owner wants.

A per-program rule works as documented: with `restore_windows = true` globally and a rule `restore_windows = false` for `term-b`, only `term-a` was written (results L6 E). TESTED.

### 2.5 The debug build crashed on every relaunch — results L10

With the **debug** build, `msg relaunch` of a stand-in ended the compositor each time (three runs; only the last, a stand-in made and relaunched in the same run with no restart in between, is recorded in `results.txt` as L10; the other two were earlier runs of `t/t3.sh` and `t/t4.sh` with the debug binary, whose output I did not keep): `panicked at src/state/mod.rs:1457: decoration entry for a window not on the stage`. That line is inside `verify_stage_invariants`, which exists only `#[cfg(debug_assertions)]` and runs at the end of every frame (SOURCE `src/state/mod.rs:1400-1460`, `src/render/lifecycle.rs:357`). The **release** build relaunched the same stand-ins in all runs without a crash (TESTED, results L1-L3, L4, L10). Whether the release build keeps a stale decoration entry (a small leak) is **UNKNOWN**; I did not look. Consequence: test layouts with the release build, and tell the driftwm author (the owner decides whether to file anything; I filed nothing).

### 2.6 Two windows of one program (what a viewer looks like to driftwm) — results L5

Two `foot` windows with the same `app_id` `viewer` but titles `ai-1 - viewer` and `nas - viewer`, placed at `[-400,0]` and `[400,0]`; `viewer.desktop` starts `foot --app-id=viewer --title=GENERIC-RELAUNCH`.

- The file held two records that differ **only in position and size**: no title, nothing that names the machine. TESTED.
- After `kill -9` and a restart the two stand-ins were back in the right places. `msg relaunch viewer` twice started two windows that **both** came up titled `GENERIC-RELAUNCH` (the `.desktop` file's fixed command) in the two slots. TESTED.

So for the hub, whose windows are (BELIEVED, from `HUB-OS.md` and `docs/viewers-research.md`) one viewer program with a different machine in each window, driftwm's relaunch cannot reconnect a window to its machine: the program, its arguments and the machine are not recorded. For Moonlight windows the program's `app_id` is one fixed value for every machine (SOURCE (repo) `HUB-OS.md`: "Moonlight windows are matched by their title"), so the same problem applies (BELIEVED; Moonlight itself was never run).

### 2.7 The session file under a read-only or missing place — results L6

- The folder given by `--session-file` could not be created (its parent was a regular file, which stands in for a read-only root; I did not try to mount a read-only file system): driftwm **kept running**, logged `session store unreadable (Not a directory ...); quarantining` once at start and `failed to write durable session store: Not a directory` about five seconds after the first change, and saved nothing. TESTED (results L6 C). A real read-only mount (EROFS) was not tested.
- An earlier round on the hub image saw the same warning once "on the read-only root" and none after `XDG_STATE_HOME` was moved to `/run/hub/state` (SOURCE (repo) `docs/proposals/phase-b-desktop.md:144`).
- If neither `XDG_STATE_HOME` nor `HOME` is set, nothing is saved and nothing is logged (SOURCE `src/session.rs:106-113`).

What the read-only root breaks, in one line: **only driftwm's own session file**, and only if the state folder is not on a writable place. Nothing else of driftwm needs the disk (its runtime files are in `/run`; SOURCE).

### 2.8 Window rules as a static layout, and hot reload — results L7

A `[[window_rules]]` entry can match a program by `app_id` **and** title (glob) and give a `position` and `size` for a window that is **about to open**. TESTED with three windows: titles `ai-1 - viewer` and `nas - viewer` opened inside the size given by the rules (800x600) and at positions close to, but **not exactly at**, the rule (`[-1450, 362]` for a rule of `[-1500, 400]`; `[1550, 362]` for `[1500, 400]`): an offset of +50 and -38 that I did not explain (snapping and the title bar are my guesses: BELIEVED). A window with another title ignored the rules. Changing the config file while driftwm ran was picked up without a restart (`Config reloaded` in the log) and a window opened afterwards used the new rule (also off by about the same amount). TESTED.

A rule cannot move a window that is already open. The socket's `move` can, and it is exact (`msg move 300 200` gave `[300, 200]` every time). So **rules are not a good place for layouts**; they are a second, inexact mechanism next to hubd's own placement.

### 2.9 Setting the camera and the zoom together — results L9

Asking for `camera 300 -200` and `zoom 0.8` one after the other, with no wait, ended **at a different camera** (about `16, -16`) in both orders. Waiting two seconds between the two commands, in either order, gave exactly `300, -200` and `0.8`. TESTED (`t/t12.sh`). The zoom change moves the camera while it animates. `hubd` already waits for the camera to settle after a focus (SOURCE (repo) `internal/hub/open.go`, `settledCamera`); applying a layout's view needs the same: set the zoom, wait until the camera stops, then set the camera.

---

## 3. Do named layouts exist in driftwm?

**No.** TESTED: `grep -rn -i "named layout\|save-layout\|load-layout\|layout_name" docs README.md src config.reference.toml` found only two places, both about the **keyboard** layout (`src/input/actions.rs:673` and `src/input/keyboard.rs:187-190`, which call `xkb.layout_name`). SOURCE: every other `layout` in the source and the config reference is the keyboard layout (`input.keyboard.layout`, `switch-layout`), the output arrangement, or the automatic placement code in `src/layout/`. There is **one** session file and `load_session()` is called **once**, at start (SOURCE `src/main.rs:186`, `src/state/session_store.rs:122`; `grep -rn "load_session" src` finds no other caller). The socket has no request that saves or loads a session (SOURCE `src/ipc/protocol.rs`, the `Request` enum: Camera, Zoom, Layout (keyboard), State, DebugCounters, Subscribe, Focus, Move, Resize, Opacity, Pin, Close, Suspend, Relaunch, Action, Bookmark, Screenshot).

The closest named thing is a **bookmark**: a name for a canvas point (a camera position only). It can be set and listed over the socket (`msg bookmark NAME X Y`; TESTED) and saved with `restore_bookmarks`. A bookmark holds no windows and no sizes.

A different file per layout could be had by starting driftwm with a different `--session-file`, but the file is read only at start, so a layout change would be a compositor restart that closes every viewer. Not acceptable for a hub whose windows stay open 24/7 (BELIEVED, from `HUB-OS.md`).

---

## 4. Where things live on the hub

| Thing | Where | Label |
|---|---|---|
| driftwm's own session file | `/run/hub/state/driftwm/session.json` (memory; gone at reboot). Written by the real-display backend always (default path), so it exists even with the switches off, holding at most the camera, bookmarks and any explicit stand-ins. | SOURCE (repo) `hub-env.sh`, `phase-b-desktop.md:144`; SOURCE `src/main.rs:183` |
| hubd's record of the windows it opened | a small JSON file in the runtime folder (memory), checked against driftwm before it is trusted | SOURCE (repo) `internal/hub/record.go` |
| The config partition | `ext4`, mounted **read-write** at `/config`; holds `inventory.toml`, `viewers.toml`, `desktop.env`, and boot files that stage 0 reads (`node.conf`, `state/min_version`) | SOURCE (repo) `docs/image.md`, `docs/proposals/phase-b-image.md` (section "config partition") |
| Who owns `/config/hubos` files today | `root:root`, mode 0644 for the non-secret files; the user-split proposal would add a `hubd`-owned folder `secrets/` | SOURCE (repo) `docs/proposals/hub-user-split.md` (file table) |
| The hub's driftwm config | none is provided; defaults apply (`driftwm --backend udev` with no `--config`) | SOURCE (repo) `image/machines/hub/rootfs/etc/s6/sv/driftwm/run`; TESTED search: no `config.toml` anywhere under `image/` |

Two consequences:
- With driftwm's defaults, a hub reboot loses everything of driftwm's session (its file is in memory) and **no window reopens** — which is what the owner wants. Turning on `restore_windows` would not change that on the hub as the image is built (the file is in memory), but it would change what a *driftwm restart* (not a reboot) does: the stand-ins would come back, and hubd's `Window` record does not even have a field for `suspended` (SOURCE (repo) `internal/driftwm/driftwm.go`), so hubd could not tell a stand-in from a live window (BELIEVED to be a source of confusion; not tested).
- The config partition also holds the files the machine needs to boot. A layout writer there must be **small, rare and atomic** (section 5.4). A rolling writer like driftwm's (a write every second while dragging) does not belong on that partition.

---

## 5. What hubd could add (proposal; nothing is built)

### 5.1 Principle

driftwm stays as it is, with its restore switches off. **hubd keeps the layouts**, because hubd is the only part that knows which window is which machine (it opens them and records them; SOURCE (repo) `docs/hubd-slice2.md`, `internal/hub/open.go`). A layout is "where each machine's window goes", keyed by the **machine id** of the inventory, in the **same coordinates as the inventory's `home`** (window centre, Y up, driftwm's convention).

### 5.2 The file

One file per layout, `/config/hubos/layouts/<name>.toml`. The name is the file name: lower-case letters, digits and dashes (the same rule as machine ids). Proposed content (a sketch; the format is for the owner to approve, like the inventory was):

```toml
format = 1                       # same rule as the inventory: an unknown number is refused
saved  = "2026-10-05T15:40:00Z"  # for people only

[view]                           # optional: leave it out and the view is not touched
zoom   = 1.0
camera = { x = 0, y = 0 }        # centre of the view, Y up (what `driftwm msg camera` takes)

[[window]]
machine = "ai-1"                 # an inventory id
x = -2000                        # window centre, Y up
y = 0
w = 1280                         # visible frame size
h = 720

[[window]]
machine = "nas"
x = 2000
y = 0
w = 1280
h = 720
```

Rules, copied from the inventory's way of working: unknown field is an error; `machine` must be an id in the current inventory (otherwise that entry is skipped with a plain message, so a removed machine does not break a layout); a machine twice in one layout is an error; `x y w h` are whole numbers; no secrets. A layout is about 100 bytes per machine, so 20 machines are about 2 KB.

### 5.3 Commands (names are placeholders)

| Command | What it does |
|---|---|
| `hubd layout save NAME` | Asks driftwm for `state`. For each window that hubd's record says belongs to a machine, writes the machine id, the window's centre and its size. Also writes the view (zoom and camera) if asked. Windows hubd did not open are ignored. Overwrites a layout of the same name only with `--replace`. |
| `hubd layout apply NAME` | For each entry whose machine has an **open** window: `resize`, then `move` (resizing can keep the top-left corner fixed, so `move` goes last — the order `place()` in `internal/hub/open.go` already uses), then, if the layout has a view: `zoom`, wait until the camera stops, then `camera` (section 2.9). Machines **without** an open window are not touched and **nothing is opened**. |
| `hubd layout list`, `show NAME`, `delete NAME`, `check NAME` | Plain file operations; `check` validates against the inventory without touching driftwm. |
| `hubd layout active` | Prints the active layout name or "none". |

### 5.4 Rules that keep the config partition safe

- **Written only when the owner asks** (never on a timer), so the number of writes is a handful per year.
- **Atomic and flushed:** write `NAME.toml.tmp` in the same folder, `fsync` it, rename it, `fsync` the folder (ext4 can otherwise leave an empty file after a power cut: BELIEVED, the same reason driftwm's own documentation warns about its file). driftwm's writer does not `fsync` (SOURCE) and is therefore not a model for this.
- **A size limit** (for example 64 KiB per file, 100 files) so a bug cannot fill the 256 MiB partition (BELIEVED size from `docs/proposals/phase-b-image.md`: 16 MiB minimum, 256 MiB planned).
- **A failed write changes nothing else** and is shown on the bar as one plain message. A missing or full partition must never stop hubd or the panel (the hub's first priority is uptime: the owner's latest decisions as the lead gave them to me for this task, not yet in `HUB-OS.md`).
- The layout folder must be writable by the user hubd runs as. Today that is the user `hub` and `/config/hubos` is `root:root` (SOURCE (repo)); the user-split proposal makes it `hubd`. Question 6.

### 5.5 When a machine is opened after a layout was applied

`hubd open` already places a new window at the machine's `home`. Proposal: while a layout is **active**, `open` uses the layout's position for that machine if it has one, else `home`. So "apply layout work" then clicking AI box in the panel opens its window where `work` wants it. "Active" is hubd's memory; where it is kept is question 3. Nothing about this opens a window by itself.

### 5.6 What I tested of this

Not the feature (it does not exist). The pieces it stands on, all TESTED in section 2: `state` gives each window's centre and size in the `home` convention; `move` and `resize` are exact; `camera` and `zoom` work if separated by a wait; a restart of driftwm removes all windows (nothing reopens); hubd's own record is in memory.

---

## 6. How the inventory's fixed home positions fit

- `home` stays the **default**: where a machine's window goes when no layout says otherwise. That keeps the current rules (exactly one `home` per machine, no two machines share a `home`; SOURCE (repo) `docs/inventory-format.md`).
- A layout is a **named set of overrides** of `home`. Entries need not be unique in position (the owner may stack windows on purpose) but I propose that `hubd layout check` warns when two windows of one layout overlap by more than half (question 8).
- The inventory is edited by hand and never by hubd; **a layout is written by hubd** and is not meant to be edited by hand (it may be; the format is plain text).
- Because the layout uses machine ids, renaming a machine means editing the layouts; ids are "never changed once set" (SOURCE (repo) `docs/inventory-format.md`), so this should not happen.
- A layout saved on one hub is meaningless on another with different machines; entries for missing ids are skipped.
- The two coordinate systems are the same convention, so a layout can be made by `save` and a human can read the numbers next to the inventory's. The inventory itself marks `home` units as "provisional until the canvas unit is verified" (SOURCE (repo)); a layout inherits that.

---

## 7. Alternatives I looked at and did not pick

| Alternative | Why not (first reason) | Label |
|---|---|---|
| Use driftwm's session file with `restore_windows` | Records no machine; relaunch starts one fixed command; debug build crashes on relaunch; one file, no names (sections 2.5, 2.6, 3) | TESTED |
| One driftwm session file per layout (`--session-file`) | Read only at start; changing layout would restart driftwm and close every viewer | SOURCE |
| Generate driftwm window rules from the layouts | Only for windows that are about to open; positions came out offset; cannot move open windows (section 2.8) | TESTED |
| `suspend_on_close` as a "window remembers its place" feature | The stand-in cannot reconnect to its machine, and hubd cannot tell it from a live window | TESTED (relaunch); BELIEVED (hubd confusion) |
| Bookmarks as layouts | Camera points only, no windows | TESTED |
| Patch driftwm to add named sessions | Not needed; driftwm is single-maintainer and patches have a carrying cost (`HUB-OS.md`) | SOURCE (repo) |

---

## 8. What I could not verify

- The real-display backend: whether `restore_camera` works, what it does after a hot-plug (UNKNOWN; no GPU).
- Enter and mouse-click relaunch of a stand-in (I could only use the `relaunch` socket command).
- Any real viewer (Moonlight, remote-viewer, a VNC viewer) under driftwm: what happens to it when the compositor is killed, and the `app_id` and title it reports.
- A really read-only file system under the session path (only an unusable path was tested).
- Whether the release build leaks the decoration entry that the debug build's self-check caught (section 2.5).
- Fit, fullscreen and pinned windows in a saved layout.
- More than one screen.
- Behaviour of a layout writer under power loss on the real config partition.
- Anything about hubd's code: no code was written or changed.

---

## 9. Questions for the owner (batched)

1. **What does a layout contain?** Positions and sizes of machine windows only, or also the view (zoom and camera), or also the window mode (fit)? I propose positions, sizes and an optional view; fit windows are saved at their fitted size and the mode is ignored.
2. **Does applying a layout also affect machines opened later?** Proposal: yes, until another layout is applied or the layout is cleared (section 5.5). Or only windows already open?
3. **Is the active layout remembered?** (a) in memory only, so a hub reboot returns to the inventory homes; (b) in a file on the config partition, so the same layout is active after a reboot (still no window reopens). Proposal: (a), with a driftwm restart keeping it (hubd's memory is in `/run`).
4. **Names:** free text with the machine-id rule (lower case, digits, dashes) is my proposal. Is a limit of 100 layouts fine?
5. **Backup:** the NAS backup list in `HUB-OS.md` names the inventory, `viewers.toml` and `wofi.css`. Should `/config/hubos/layouts/` join it? (That is an edit to `HUB-OS.md`, which only you decide.)
6. **Who may write `/config/hubos/layouts/`?** Today hubd runs as the user `hub` and `/config/hubos` belongs to root. Make the `layouts` folder owned by the user hubd runs as (`hub` now, `hubd` after the user split)?
7. **Panel:** only the command line first, or entries in the panel's menu (a "Layouts" group in the list launcher)? Which key chord, if any?
8. **Overlap:** warn, refuse, or say nothing when two windows of a layout overlap?
9. **Keep driftwm's `restore_*` and `suspend_on_close` switches off on the hub?** I propose yes (their defaults). If you want stand-ins for some other reason (for example a closed window leaving a marker), that needs hubd to learn the `suspended` field first.
10. **Release or debug driftwm for the hub?** The image already builds the release one (SOURCE (repo) `tools/image/build-hub-parts.sh`). Please confirm that no debug build ever ships, because of section 2.5.
11. **Should I report the debug-build crash to the driftwm author?** I filed nothing (`CLAUDE.md`: nothing is pushed to or filed on driftwm without you).

---

## 10. Table of items

| # | Item | Label |
|---|---|---|
| 1 | All switches on: windows saved with position and size, `kill -9`, restart: dormant stand-ins at the same places, nothing launched | TESTED (results L1-L3) |
| 2 | `kill -9` leaves `session.json` intact; dead socket, status file and Wayland socket files remain and are harmless | TESTED (L1-L3, L8) |
| 3 | Foot windows exit when the compositor is killed; real viewers | TESTED / UNKNOWN |
| 4 | Defaults: only explicit stand-ins survive a kill | TESTED (L4) |
| 5 | Windows without a `.desktop` file are not saved | TESTED (L1-L3) |
| 6 | `restore_bookmarks` works across `kill -9` | TESTED |
| 7 | `restore_camera` ignored in the nested backend; saved camera overwritten by the default | TESTED |
| 8 | Camera applied by the `udev` backend | SOURCE only; behaviour UNKNOWN |
| 9 | Relaunch puts the program in the stand-in's slot; debug build panics, release does not | TESTED (L1-L3, L4, L10) |
| 10 | Two windows of one `app_id` come back identical and relaunch starts one fixed command | TESTED (L5) |
| 11 | Corrupt file and newer-version file are set aside; unwritable path only logs | TESTED (L6) |
| 12 | Last second of changes is lost on SIGTERM | TESTED (L6 D); SOURCE for `kill -9` |
| 13 | No fsync, tmp-file-then-rename, the folder must be writable | SOURCE |
| 14 | Camera and zoom: set back to back they interfere, with a 2 s gap they hold | TESTED (L9) |
| 15 | Window rules place new windows near the rule position, not exactly; hot reload works | TESTED (L7) |
| 16 | No named layouts in driftwm | SOURCE + TESTED search |
| 17 | Hub image keeps driftwm's state in memory | SOURCE (repo) |
| 18 | Enter and click relaunch | UNKNOWN |
| 19 | Real read-only file system under the session path | UNKNOWN |
| 20 | Layout file, commands, atomic writes, `open` using the layout | PROPOSAL, not built |

## 11. Sources read (2026-10-05)

- driftwm at `352333a8fa1b22171492d4b71a54102045c9a19d`, read from a clone: `docs/session.md`, `docs/ipc.md`, `docs/cli.md`, `config.reference.toml` (`[session]`), `src/session.rs`, `src/state/session_store.rs`, `src/state/persistence.rs` (head), `src/state/suspended.rs` (head and `relaunch_suspended`), `src/main.rs` (lines 150-200), `src/backend/udev.rs` (the two `saved_camera_state` call sites only), `src/ipc/protocol.rs`, `src/state/mod.rs` (lines 1400-1460), `src/render/lifecycle.rs` (the end of `post_render`).
- This repository: `HUB-OS.md`, `CLAUDE.md`, `docs/driftwm-findings.md`, `docs/inventory-format.md`, `docs/image.md` and `docs/proposals/phase-b-image.md`, `phase-b-desktop.md`, `hub-user-split.md` (the lines named above), `image/machines/hub/rootfs/` (the driftwm and hubd start scripts, `hub-env.sh`), `internal/driftwm/driftwm.go`, `internal/hub/open.go` (`place`), `internal/hub/record.go`.
- Not used: web pages, search results, summarising tools, other helpers' reports.
