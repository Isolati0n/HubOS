# PROPOSAL: the focus core of the session layer (stage 1, part A)

**Status: PROPOSAL.** Nothing here is built. `HUB-OS.md` wins if this file disagrees with it (it is not edited by this work). Written 2026-10-10 by the lead agent (no helpers). Labels: TESTED, SOURCE (a repo file or page, named), BELIEVED, UNKNOWN. **Every constant marked "placeholder" is a placeholder, not a decision.**

## 0. What this is
The session layer (SOURCE `HUB-OS.md`, "The session layer", owner direction 2026-10-06, nothing built) has one worker per machine and a presenter that "draws the frames and sends input back". This file is about one small part: **which machine receives keyboard and mouse input at a given moment, and what happens to input in flight when that changes**. It is a pure function

    step(state, event) -> (new_state, actions)

All I/O (sockets, frames, timers, the compositor) stays outside and only feeds events in and carries actions out. It lives in the **native core** (workers and presenter), not in the Elixir conductor, so that input still works when the conductor is down (owner decision 5 below).

Not in this work: the conductor, the parser and frame-copy bake-off, integration, the 24-hour soak, the compositor, `hubd`, the clipboard, the frame path (Raw RFB from the node to the worker and the shared-memory frame handoff from worker to presenter stay as decided), the keyboard forwarder on the gaming box.

## 1. The existing rules this must agree with (SOURCE)
| Where | Rule |
|---|---|
| `HUB-OS.md` The session layer | One worker per machine (connection, frame in shared memory, sound stream, clipboard, volume); a presenter draws frames and sends input back; an unreachable machine keeps its last frame, marked stale; RFB key events by keysym; reconnect with backoff when a node's server disappears; the native core language is NOT decided (bake-off: Rust with Kani, Ada with SPARK, Zig). |
| `HUB-OS.md` Screen and bar (line 266) | "While a node window has focus, only the keyboard-flip chord and one menu key go to the hub." |
| `HUB-OS.md` Input sharing (lines 305-313) | One keyboard and mouse on the gaming box; **Super + Ctrl + Shift** flips input between the gaming box and the hub; when input is on the hub the gaming box forwards events with our own forwarder; the forwarder sends "release all keys" on every flip so no key stays stuck. |
| `HUB-OS.md` Windows and sessions | Clicking a machine that is already open goes to its existing window; if a machine drops while its window is open, leave the window alone; only the alert changes. |
| `docs/input-sharing-research.md` | The forwarder reads evdev devices; the hub creates virtual devices and sends release-all on each flip; the stuck-modifier hazard at each grab is documented (section 6). |
| `HUB-OS.md` Run schedule | Stage 1 of the session layer is the bake-off, the conductor and integration; the 24-hour soak is separate and later. |

## 2. The core
### 2.1 Types and limits
- `MAX_MACHINES = 20` (SOURCE `HUB-OS.md`: at most 20 machines). A destination is a machine id `0..MAX_MACHINES-1` or `HUB`. Model checking and proofs may use a smaller bound (the TLA+ model uses 3 machines; proofs state their own bound).
- **Placeholders (owner question 1):** `MAX_KEYS = 32` keys held at once, `MAX_BUTTONS = 8` mouse buttons held at once, `VERIFY_TIMEOUT` a named constant (placeholder 2000 ms; the real value is measured on the real hub in December; QEMU only checks that the core recovers correctly). Fixed-size structures only: no heap, no garbage collector.

### 2.2 State
- `focus`: one machine id or `HUB`.
- `epoch`: a counter that never decreases (saturating at its maximum).
- `gen[m]`: a counter per machine, bumped on each worker restart.
- `conn[m]`: up or down.
- `held`: the keys and the mouse buttons currently held, each with the **machine that received its press** (the destination at delivery).
- `waiting`: none, or the machine whose focus change is waiting for verification.
- `owed[m]`: a release-all is owed to machine `m` (see 2.5).

### 2.3 Events (inputs to `step`)
- `Input(kind, code, dest, gen, epoch)` with kind one of key-down, key-up, button-down, button-up, motion, **chord** (the keyboard-flip chord) or **menu key**. `dest`, `gen` and `epoch` are the values the sender stamped when it made the event.
- `FocusRequest(target)`; `VerifyOk(m, epoch)`; `VerifyTimeout(m, epoch)`.
- `WorkerRestart(m)`; `Disconnect(m)`; `Reconnect(m)`.

### 2.4 Actions (outputs)
`Deliver(dest, input)`, `DeliverHub(input)`, `Drop(input)`, `Flash(m)` (the window flashes), `SynthKeyUp(m, key)`, `SynthButtonUp(m, button)`, `ReleaseAll(m)`, `Stale(m)` (mark the frame stale / not stale are the I/O shell's job and not core actions).

### 2.5 Rules
1. **Delivery.** An input to machine `m` is delivered (`Deliver`) only if `m = focus`, its `gen = gen[m]`, its `epoch = epoch` and `conn[m]` is up, and `waiting` is none. Otherwise it is dropped (`Drop`), and `Flash(m)` is emitted if `conn[m]` is down. A press that is delivered is recorded in `held` with `m`; a release removes it. If `held` is full the press is dropped (owner question 1).
2. **Hub path.** The flip chord and the menu key are never subject to rule 1: they are delivered to the hub (`DeliverHub`) in every state. While the focus is `HUB`, all input is delivered to the hub.
3. **Focus change to `m`** (`FocusRequest`): bump `epoch` **first**, so events in flight drop at once. For every key **and every mouse button** held on the old machine: emit `SynthKeyUp` / `SynthButtonUp` (synthesized releases, **never a replay of real events**); if the old machine is disconnected, set `owed[old]` instead (the release is sent at reconnect). Clear `held`. Set `waiting = m`; the focus is `HUB` until verification (nothing is delivered to `m` until it succeeds).
4. **Verification.** `VerifyOk(m, e)` counts only if `waiting = m`, `e = epoch` and `conn[m]` is up: then `focus = m`, `waiting = none`. `VerifyTimeout(m, e)` (same checks) or a `Disconnect(m)` of the waiting machine: `focus = HUB`, `waiting = none`. What verification **is** (what the worker checks) is owner question 2.
5. **Worker restart of `m`.** Bump `gen[m]`; clear `held` for `m`; `conn[m]` becomes down (the new worker has not connected yet); `owed[m]` is set. If `m` was the focus or the waiting target: bump `epoch`, `focus = HUB`, `waiting = m` (return only after verification).
6. **Disconnect(m)** sets `conn[m]` down. If `m` was the focus, the focus falls back to the hub, bumping `epoch`, and the held keys on `m` become `owed[m]`.
7. **Reconnect(m)** (with or without a worker restart): `conn[m]` up, emit `ReleaseAll(m)` (owner decision 7, the same as the forwarder's flip rule), clear `owed[m]`.

## 3. Invariants, checked against `HUB-OS.md`
| # | As asked | Check against `HUB-OS.md`, and my rewording |
|---|---|---|
| 1 | No stale delivery: every delivered event matched focus, generation and epoch at delivery time | Consistent. Stated on the `Deliver` actions of one step. |
| 2 | At most one destination: in every step, the actions deliver input to at most one machine, and that machine is the focus | Consistent. A property of the actions, not of `state`. The hub path (`DeliverHub`) is separate: a step may emit one `Deliver` to a node **or** one `DeliverHub` for one input, never two destinations for one input. Synthesized releases go to the old machine, not to the focus; they are not "input" for this invariant (stated in the model). |
| 3 | No stuck keys | Consistent with line 310 (the forwarder's release-all). The core adds: after any focus change, restart or reconnect `held` holds nothing for the old machine and every held item had a release emitted, **or** `owed` is set and `ReleaseAll` is emitted at reconnect. Model: a ghost set of keys each node believes down is empty whenever the machine is connected and nothing is owed. |
| 4 | The epoch never decreases | Consistent. Saturating add (no wrap). |
| 5 | The hub is always reachable on the hub-side path | Consistent with line 266 (only the chord and one menu key go to the hub). Reworded to: the chord and the menu key are **delivered to the hub in every state**, including when the focused node is dead, waiting, or a restart is in progress. Covers only events that arrive at the hub (owner decision 8). |
| 6 | Bounded transition | A **model property**: with an urgent timeout (the model's clock stops while a verification is overdue), `waiting` is none again within a fixed number of ticks. It holds **only if the I/O shell delivers `VerifyTimeout` when the timer fires** (an assumption about the shell, not proved). The real-time bound is UNKNOWN until measured on the real hub in December. |

## 4. Differences and gaps (everything I found)
1. **Order of focus change versus verification.** `HUB-OS.md` has no verification rule at all. The core adds one (owner decision 6). It means a window click does not give instant keyboard focus: input drops until verification passes; the usability cost is UNKNOWN.
2. **Who sends `FocusRequest`.** The compositor (driftwm) owns window focus today and `hubd` can focus a window through driftwm's socket (SOURCE `HUB-OS.md` line 245). Neither is to be changed. The path from "a node window got focus in the compositor" to a `FocusRequest` event in the presenter is **not recorded**; it is part of the I/O shell and an owner question (3).
3. **Who stamps `gen` and `epoch` on an input.** The stamps are what make late events droppable. How the presenter learns the current `epoch` and `gen[m]` (a published value, a reply from the core) is **not recorded** (owner question 4).
4. **The forwarder.** The forwarder on the gaming box sends release-all on a flip (SOURCE line 310) and is out of the model until December (owner decision 8). The core's synthesized releases are on the hub side and do not replace it; both may send releases for the same key (harmless, BELIEVED).
5. **Reconnect handling.** `HUB-OS.md` says a node's server reconnects with backoff; it says nothing about keys at reconnect. The core sends `ReleaseAll(m)` on every reconnect (owner decision 7).
6. **Window flash.** "Flash when `conn[m]` is down" is new; `HUB-OS.md` says only that a dropped machine's window is left alone and the alert changes. A flash is an extra visual cue; the compositor is not changed, so the presenter would draw it (UNKNOWN whether acceptable).
7. **The menu key.** "One menu key" (line 266) is not named anywhere in the repo. The core treats it as a constant event kind; which physical key it is: not recorded (owner question 5).
8. **Motion events.** The rules speak of keys and buttons; motion is treated like other input for delivery and is not recorded in `held`. Scroll events: treated as button events (BELIEVED, owner question 6).
9. **Worker restart while no focus.** The restart bumps `gen[m]` only; there is no focus effect (consistent with the request).
10. **Gates.** The bake-off gates in `HUB-OS.md` (ten million fuzz cases per parser with zero crashes and hangs, a proof of no panics for the parser and frame copy, handoff stalls, recovery times, 24-hour soak) are kept unchanged. This work adds: the invariants hold after every step during fuzzing (at least ten million cases per candidate), and where the tool can prove them (Kani, SPARK) they are proved within stated bounds.
11. **No conflict that stops an item** was found between the owner's rules and `HUB-OS.md`.

## 5. Owner decisions (2026-10-10)
1. Held keys on a focus change: the old machine receives synthesized releases, never replayed real events.
2. Held mouse buttons get the same treatment as keys.
3. The epoch bumps at the start of a focus change.
4. Model language: TLA+ with the TLC model checker; Lean only if a later property needs it. TLC needs a Java runtime and the `tla2tools` jar: no package install (unpack with `dpkg -x` into a temporary directory; the jar from the TLA+ project's GitHub releases). If that is not possible the model part stops and the rest continues.
5. The focus step function lives in the native core (not the Elixir conductor).
6. On an ordinary focus change the new machine is verified before it receives input; events drop until it passes; a timeout falls back to the hub.
7. A plain reconnect sends release-all to the node.
8. Invariant 5 covers only the hub-side input path (events arriving at the hub). The keyboard forwarder on the gaming box stays out of the model until December.
9. The same step function is implemented in ALL THREE bake-off candidates: Rust with Kani, Ada with SPARK (only if it installs without root), Zig. A tool that cannot be installed is recorded UNKNOWN with the exact failure; never skipped silently.

## 6. Plan (this file is step 1; no code before it is pushed)
2. TLA+ model, 3 machines, `docs/models/focus-core/`, checked with TLC (invariants 1 to 5, 6 as a bounded-step property). 3. A shared trace suite derived from the model. 4. One implementation per candidate under `tools/session/focus-core/<language>/`. 5. Fuzz (and prove where the tool can) with the invariants checked after every step. 6. Pilot per candidate (30 to 60 min), then chunks of 2 to 4 hours with a new seed, committed per chunk, one long job at a time. 7. A comparison table; no winner is picked.
Long jobs (TLC runs, fuzzing) start only when no other long job runs; the init comparison (PR #76) is running now, so those wait for it.

## 7. Questions for the owner (nothing here stops the decided parts)
1. Limits: are `MAX_KEYS = 32` and `MAX_BUTTONS = 8` acceptable placeholders, and what should happen to a press that does not fit (drop it, as written)?
2. What does "verified" mean for a machine (the connection is up and the next framebuffer update arrived? a ping to the worker? something else)? The model keeps it abstract (`VerifyOk`/`VerifyTimeout`).
3. How does the compositor's window focus become a `FocusRequest`?
4. How does the presenter learn the current epoch and generation to stamp inputs?
5. Which physical key is the "one menu key"?
6. Are scroll events button events for `held`?
7. Is a window flash acceptable as the cue for input dropped on a down machine?
