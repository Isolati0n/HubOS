# PROPOSAL: "open everything" for a saved layout

**Status: PROPOSAL. Nothing here is decided, built or tested.** `HUB-OS.md` wins if this file disagrees with it. Written 2026-10-10 by the lead agent (no helpers). No code. Labels: TESTED, SOURCE, BELIEVED, UNKNOWN. **Times and limits are placeholders.**

## 1. Purpose
Windows never open by themselves after a reboot; the owner opens them (SOURCE `HUB-OS.md`, Hub behavior: "No auto-reopen of windows after a hub restart"). With 20 machines that is 20 clicks. This adds **one button next to each saved layout** that opens the machines of that layout, in order, with a pause between, then returns focus. It is **never run automatically** (that rule stays: the button is the owner's action).

## 2. What exists (SOURCE `HUB-OS.md`, `docs/proposals/driftwm-layouts.md`, `docs/hubd-slice2.md` section 18)
- Layouts are saved by name with positions, sizes and the view; `hubd layout save|apply|list|delete|clear`; a "Layouts" group in the menu.
- Applying a layout moves open windows and the view; it also governs machines opened later.
- `hubd open ID` opens a machine and places the window at its home; opening an open machine goes to its window.
- **Differences from the request:** (a) the layout file stores positions and sizes; whether it stores an "order" or the focused node is not something I verified (the stacking order is an unbuilt backlog item: `docs/STATE.md` 4 items 2 and 3), so "the saved order" must be defined (question 1 below); (b) "first non-empty frame" needs a viewer that reports frames; today windows are external viewers (VNC viewers) and `hubd` can see only that a window appeared in driftwm's list, not what is drawn. A frame check becomes possible with the session layer's workers (not built); until then "verified" means "the window exists and has a size" (BELIEVED).

## 3. The flow
1. The owner presses the button on layout L.
2. Order: the node that has focus now first, then the layout's saved order.
3. For each machine in order: if it is **down** (the last check says so), skip and mark `unavailable`; otherwise send one open request, then wait the stagger (placeholder **500 ms**, configurable) before the next. Never more than one open per stagger interval.
4. A progress line shows "opening 7 of 20" and a **Cancel** button.
5. After all reachable machines were asked, wait until each has shown a first non-empty frame (or the verification timeout, placeholder 20 s per node).
6. Restore focus to the node that had focus before (or the first reachable machine if none).
7. Status per machine: `opened`, `unavailable`, `failed` (with the last error text). No modal dialogs anywhere.

## 4. State machine
`idle -> opening -> verifying -> restoring_focus -> done`; `cancelled` and `failed` are reachable from `opening` and `verifying`.
| State | Entered when | Leaves when |
|---|---|---|
| idle | nothing running | button pressed |
| opening | button pressed | every reachable machine was asked, or Cancel |
| verifying | all asked | all verified, or timeout per node, or Cancel |
| restoring_focus | verified | focus set |
| done | focus restored | back to idle after the report is shown |
| cancelled | Cancel pressed | back to idle |
| failed | an internal error (not a single machine's failure) | back to idle |

## 5. Rules
- **Cancel** stops new open requests; opens already sent complete (they are not closed).
- **Pressing twice** does nothing for windows already open and in position; it retries the machines that were skipped or failed. While a run is active a second press is ignored.
- A machine whose window is already open is not opened again (the existing rule: no duplicates).
- Nothing is paused or closed anywhere; a failing machine never stops the run.
- A run uses the inventory as of its start (if the inventory is reloaded during a run: `generated-inventory.md`, the run keeps its list).
- Rate: at most one open per stagger interval, so 20 machines take at least 10 s plus verification.

## 6. Failure table
| What happens | Result |
|---|---|
| A machine is down | `unavailable`, skipped; the rest open |
| A machine goes down mid-run | `failed` with the error; the rest continue |
| `hubd` restarts mid-run | The run is lost; windows already open stay; pressing again continues |
| Compositor restarts mid-run | `hubd` restores windows itself (SOURCE `HUB-OS.md`); the run reports `failed` for the rest and stops |
| Verification times out for one node | That node `failed` ("no frame after 20 s"); others unaffected |
| Focus node no longer exists at the end | Focus the first reachable machine |
| Layout names a machine no longer in the inventory | Skipped, `unavailable` ("not in the inventory") |
| Button pressed with an empty layout | Nothing to do; shown as "done, 0 opened" |

## 7. The seven tests
1. All machines reachable: all open in order, focus restored.
2. One machine down: marked `unavailable`, the others open.
3. Double press: the second does nothing for open windows and retries only the skipped ones.
4. Press then cancel: no further requests after the cancel; opens already sent finish.
5. No saved focus: the first reachable machine gets focus.
6. Verification timeout for one node: that node `failed`, others `opened`.
7. Timing: no more than one open per stagger interval (fake clock).

## 8. Five assumptions I am least sure of
1. That `hubd` can issue 20 opens 500 ms apart without upsetting driftwm or the bar (UNKNOWN; the reconnect-storm risk is in `pre-mortem.md`).
2. That a "first non-empty frame" can be observed (UNKNOWN until the session layer's workers exist).
3. That "saved order" has a clear meaning in the layout file (UNKNOWN; see section 2).
4. That cancel semantics ("in-flight opens complete") are what the owner expects (BELIEVED).
5. That 500 ms is enough to avoid a burst on the nodes' wayvnc servers (UNKNOWN; there are no real nodes).

## 9. Decided by the owner (2026-10-10)

- **Focus is restored after verification.**
- **Cancel lets in-flight opens finish** (it only stops new requests).
- The **stagger is 500 ms and configurable**.
- There is **no "force open"** option.

## 10. Questions for the owner (still open)

1. Should the stagger also be settable per node (slow nodes get more time)? (Only "configurable" was answered.)
2. What does "saved order" mean: the order of the layout file, the stacking order, or an explicit list?
