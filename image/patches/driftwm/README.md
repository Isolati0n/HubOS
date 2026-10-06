# The patch set of the hub's compositor (driftwm and Smithay)

The hub image builds driftwm at one pinned commit and Smithay (the library under it) at one pinned revision (`PINS`),
and applies every patch below, in the order of the file names (`tools/image/build-hub-parts.sh`; `git apply`, no fuzz).
**The build stops with an error if a patch does not apply** or if driftwm's `Cargo.lock` no longer names the pinned Smithay
revision. Nothing here is sent to the driftwm or Smithay authors (owner decision). The licence of driftwm is
GPL-3.0-or-later; the patches are published under the same licence.

Plain words: P1 to P12 and D6 to D8 are the hub's own set; "Smithay P7 to P11" and "D1 to D5" come from the
bulletproof-compositor research (`docs/proposals/bulletproof-compositor.md`, its findings are named BC-1 to BC-14).
The research called its Smithay patches "P7 to P11", which collides with this set's driftwm P7 to P11. **In file names
and here the Smithay ones are `smithay-pN`; a bare `pN` is always a driftwm patch.**

Full reasons, tests and results: `docs/proposals/driftwm-patches.md`. Owner decisions are quoted from that file (section 10 and
section 3.3) and from the owner's decisions of 2026-10-06 for this adoption ("round 3").

## driftwm patches (applied to `DRIFTWM_COMMIT`, `0*.patch`)

| File | What it does | Fixes | Owner decision |
|---|---|---|---|
| `0001-p1-wayland-socket-fd.patch` | driftwm can adopt a Wayland socket opened by a parent (`--socket`, `--wayland-fd`); also regenerates `docs/cli.md` | hub-stability plan C (socket handover); unused by the hub's service | hub-stability: patches P1 to P10 are carried |
| `not-applied/0002-p2-test-hooks.patch` | test hooks that hang or panic on purpose | (test only) | never in an image |
| `0003-p3-catch-panics.patch` | catches a panic and goes on **only if** `HS_CATCH_PANICS` is set | hub-stability (panics in the dispatch and in IPC connections) | **stays OFF**: the service does not set the variable. A panic ends the compositor, s6 restarts it, `finish` counts it (5 in a minute: stop) |
| `0004-p4-config-parser.patch` | a colour with a non-ASCII letter no longer panics | hub-stability F3 | carried |
| `0005-p5-poisoned-output-state.patch` | `output_state()` survives a poisoned lock | hub-stability | carried |
| `0007-p7-nonblocking-log-writer.patch` | log lines go through a queue and a writer thread: a stuck log reader cannot freeze the loop | hub-stability F4 (a stopped log reader freezes the loop) | owner 2026-10-06: own small writer is OK |
| `0008-p8-startup-shellouts.patch` | removes the `systemctl`/`dbus-update-activation-environment` start-up calls, time-boxes the xwayland-satellite probe | hub-stability F6 and F7 (start-up helpers block; `set_var` with threads running) | owner 2026-10-06: remove, not time-box |
| `0009-p9-flat-background.patch` | a default shader that does not compile gives a flat background | hub-stability (the default background shader is a `.expect`; found by the bad-shader test) | owner 2026-10-06: black is fine |
| `0010-p10-exit-on-failed-frames.patch` | every frame of an output fails for 2 s: log once and **exit 70** so the service restarts the compositor | hub-stability 8.2 (lost GPU; bulletproof A5) | owner 2026-10-06 OK; owner decision 8 of this adoption: a GPU reset or lost device **exits and restarts** the compositor, hubd restores the windows |
| `0011-p11-files-on-a-worker-thread.patch` | session and state files are written by a worker thread (`bgwrite.rs`), never on the event loop | hub-stability F5 (a blocked session or state file freezes the loop) | owner 2026-10-06; decision 9 of this adoption (fsync from the worker thread) is **question 1 in the PR: not in this patch** (it does not fsync today) |
| `0012-p12-no-config-hot-reload.patch` | removes config hot reload (the inotify watch, `reload-config`, the dead code behind it) | BC-6/BC-7 reach the running compositor only through hot reload | owner 2026-10-06 (hot reload is off on the hub) |
| `0013-d1-session-entry-cap.patch` | a `session.json` with more than 512 entries is set aside like a corrupt file | **BC-5** (30,000 entries: 9.3 GB, killed at every start) | decision 3: 512 session entries |
| `0014-d2-config-nul.patch` | a NUL in a key binding is an error, in a keyboard setting a warning | **BC-6** (panic in `--check-config`, at start-up and at reload) | adopted (decision 3 batch) |
| `0015-d3-region-overflow.patch` | saturating arithmetic for corner rectangles built from a client's opaque region | **BC-4** (debug-build overflow) | adopted |
| `0016-d4-decoration-buffer-size.patch` | the CPU-drawn stand-in chrome is limited to 16,384 x 8,192 pixels per side (before scale) | **BC-12** (IPC `Resize` of a stand-in to 32768: `capacity overflow`, exit 101) | decision 3: 256 MB per stand-in |
| `0017-d5-saturating-geometry.patch` | saturating and floating-point sums in the canvas and region helpers | **BC-13** (far `Move`, then a navigation action: `f64::clamp` panic), part of **BC-11** | decision 4 (an IPC request never crashes) |
| `0018-d6-cluster-shifts-overflow.patch` | **new here.** `resolve_cluster_shifts` uses saturating arithmetic; four regression tests | the rest of **BC-11** (overflow on add and on negate in `layout::cluster::resolve_cluster_shifts`, found by the `hsfuzz geom` run; D5 did not cover it) | decision 4 |
| `0019-d7-bc12-bc13-regression-tests.patch` | **new here.** Unit tests (tests only, no code) for BC-12 and BC-13 | BC-12, BC-13 | "must have repro tests"; the live-compositor reproductions are in `tools/image/experiments/driftwm-patches/` |
| `0020-d8-hide-unused-protocols.patch` | **new here.** The Wayland protocols nothing on the hub uses are not shown to any client (session lock and others; list and evidence in `docs/proposals/driftwm-patches.md` section 3.1) | **BC-10** (a locker that dies leaves the outputs blank) and less code reachable by clients | decision 7 |

## Smithay patches (applied to `SMITHAY_REV`, `smithay/0*.patch`; driftwm is built against this patched copy)

| File | What it does | Fixes | Owner decision |
|---|---|---|---|
| `smithay/0006-p6-shm-pool-resize-zero.patch` | `wl_shm_pool.resize(0)` is a protocol error, not a panic | hub-stability F1 | carried |
| `smithay/0007-smithay-p7-popup-parent-cycle.patch` | a popup whose parent chain contains itself is refused (`invalid_popup_parent`); chain walks stop after 256 steps | **BC-1** (permanent hang, SIGTERM ignored) | decision 4: protocol error disconnects the client |
| `smithay/0008-smithay-p8-subsurface-self-reorder.patch` | `place_above/below` the surface itself is `bad_surface` | **BC-2** (panic, exit 101) | decision 4 |
| `smithay/0009-smithay-p9-absurd-geometry.patch` | viewport destination above 2^24 is `bad_value`; subsurface offsets clamped to plus or minus 2^24 | **BC-3** (Mesa abort, "stack smashing") | decision 3: 2^24 |
| `smithay/0010-smithay-p10-positioner-clamp.patch` | every `xdg_positioner` number clamped to plus or minus 2^24 | **BC-4** | decision 3 |
| `smithay/0011-smithay-p11-negative-sizes.patch` | negative sizes in `wl_region` and `wl_surface.damage` ignored; negative min/max size is `invalid_size`; layer-surface size limited to 2^24 | **BC-4** | decision 3, 4 |

## Overlaps between the two groups (checked, nothing dropped)

Apply order is `0001`..`0020` then Smithay `0006`..`0011`. All of them apply cleanly in that order to the pins (TESTED, see the
PR). Same file touched by two patches, and which one is kept:

- `src/session.rs`: P11 (writes) and D1 (the read-side cap). Different functions; **both kept**. A session file is capped on
  read (D1) and written by the worker thread (P11).
- `src/config/mod.rs`, `src/config/parse.rs`: P4 (non-ASCII colour), D2 (NUL), P12 (hot reload removed). Three different
  inputs; **all kept**. After P12 the NUL panic of BC-6 can no longer happen at a reload (there is none); D2 still protects
  start-up and `--check-config`.
- `src/canvas.rs`, `src/region.rs`: only D5 (and D7's tests). No overlap.
- `src/main.rs`: P1, P3, P7, P8, P11, P12 (different places; all apply).
- Smithay `src/wayland/compositor/handlers.rs`: smithay-p9 (subsurface offset) and smithay-p11 (damage and regions):
  different requests; both kept.
- **Two fixes of the same thing:** none found. The nearest case is the hub-stability P10 (driftwm: exit after 2 s of failed
  frames) against the research's A5 wish ("rebuild the renderer in place"): only P10 exists; the in-place rebuild was
  never written.

## Not patches, but part of the same adoption

`tools/image/check-driftwm-config.sh` (build check of the baked settings file: strict, unknown fields refused,
`restore_windows = false` required), `image/machines/hub/rootfs/usr/lib/hubos/driftwm-config` (run-time choice of the last good
file, with an alert), and hubd's stacking-order restore (`internal/hub/restore.go`). See `docs/proposals/driftwm-patches.md`.
