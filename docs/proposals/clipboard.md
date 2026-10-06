# PROPOSAL: the hub clipboard (history, picker, targeted paste, rich items, bar item)

**Status: PROPOSAL, docs only. Nothing here is decided beyond the owner decisions listed in section 1, and nothing here is built into the images or into hubd.** `HUB-OS.md` wins if this file disagrees with it. Written 2026-10-06 by a helper agent. The only code is two throwaway test scripts in `tools/image/experiments/clipboard-notifications/` (`fakerfb.py`, run once; the shell scripts I used for the `wl-copy` tests are quoted in section 12). Where this document says "proposal" or "default", the owner has not chosen; section 14 lists every choice as a question.

**Labels on every item:**
- **TESTED**: I ran it in this build environment; the exact command and output are given (or the script is named).
- **SOURCE**: read in a file at a named place; the date read (2026-10-06) is given. "(repo)" means a file of this repository written by an earlier session and not re-checked by me.
- **BELIEVED**: I think it is true, I did not test it and have no source.
- **UNKNOWN**: nobody has checked; do not rely on it.

**Helpers and tools:** I used no sub-helpers. Every source below was read by me on the primary file (downloaded with `curl` from the raw file URL or unpacked with `dpkg -x`). No package was installed on the machine; programs were unpacked into a scratch folder and run from there, and the scratch folders in `/tmp` were deleted at the end.

This document builds on `docs/proposals/node-helper-api.md` (text clipboard bridge, loops), `docs/proposals/file-transfer.md` (file clipboard, tickets), `docs/proposals/hub-isolation.md`, `docs/proposals/hub-stability.md`, `docs/bar-findings.md` and `docs/viewers-research.md`. Its companion is `docs/proposals/notifications.md`.

---

## Plain-words summary

1. **What the owner asked for.** One clipboard "memory" for the whole cluster, shown as an item on the bar. Every copy on any machine (and on the hub) goes into a list that remembers where it came from and when. You open the list with a key chord, search it, and send an entry to one machine, several machines, or the hub, either as it was or as plain text. Pictures show a small preview. Files are only remembered as "these files, on that machine"; they move from machine to machine only when you paste, and the hub never carries the bytes. A copy on a node is **never** sent to another machine on its own.
2. **How I would split it (proposal).** Four small separate programs, so that one hanging cannot freeze the bar or the screen: the **store** (keeps the list in RAM, saves only the pinned entries to disk), the **capture parts** (one inside each session worker for each machine, plus one watching the hub's own clipboard), the **picker** (a list on the screen), and the **bar item** (a small feed to Waybar, built the same way as `hubd feed` already is). They talk over local sockets with small JSON messages, so the language of the session layer's core does not matter (that language is not decided).
3. **Two facts I found in the source that change the design.** (a) The display program on the nodes (wayvnc with neatvnc) can carry **text only** through the RFB clipboard: neatvnc advertises only the "text" format and wayvnc passes only `text/plain;charset=utf-8` (SOURCE). So **images and file references from nodes cannot arrive through the display connection**; they need a new call in the node helper API (a proposal in section 6). (b) Every viewer connected to wayvnc gets its **own** clipboard watcher in wayvnc, and every watcher broadcasts to **all** viewers (SOURCE). With two viewers (the hub and the phone) one copy on the node produces **two** announcements, and a paste made through one viewer comes back to the other as a "new copy" (BELIEVED from the source, not run). So the store must remove duplicates and recognise echoes of what the hub itself sent.
4. **Echo loops (what I tested).** With a fake node that speaks the RFB Extended Clipboard messages, an echo of a text the hub had just sent was recognised and not stored as a new entry (TESTED against my own stand-in, which proves only that my design is consistent, not that real wayvnc behaves the same). Because node copies are never forwarded automatically, a real loop cannot start; the echo rule only keeps the history clean.
5. **What happens when something fails (proposal, section 8).** The bar item never waits for the store: it answers from its last known state and shows it marked STALE. The picker asks the store with a 500 ms limit and says "history not answering" instead of freezing. A session worker that dies is restarted by the conductor; its machine is marked "stale" in the list, and on reconnect it asks the node for the current clipboard once ("peek") so a copy made during the gap is not lost. If the store dies, the list is empty after the restart (history is RAM only); the pinned entries come back from disk and the owner gets a notification that history was lost.
6. **Size (proposal).** About 500 text entries and about 100 MB for pictures and file references. At the limit the oldest unpinned entry of that kind goes first. An item that is too big is not stored, but a one-line record ("12 MB from ai-1, too large") is, so you can see it happened. The 500 text entries have no byte limit yet: with the helper API's 1 MiB per item the worst case is 500 MiB of RAM. That needs an owner decision (question 12).
7. **What I could and could not test here.** TESTED: `wl-copy`/`wl-paste` behaviour with text, PNG and `text/uri-list` (types, change events, cleared selection, killed holder, 6 MB and 25 MB images, plain-text paste, a watcher eating a paste-once item); the RFB Extended Clipboard message flow against a fake server, including a 9 MB text, a silent node and a connection cut in the middle. NOT tested: real wayvnc and neatvnc, the session layer (does not exist), driftwm, Waybar and wofi (tested earlier by other sessions, not re-run), the QEMU hub image (a full run takes hours and other helpers use the machine).
8. **The first slice that can be built and tested here** is in section 13: the store plus a hub-clipboard watcher plus a command-line client, text only, with unit tests for eviction, pins on disk and echo, then the bar item and picker, then the fake-node worker.
9. **Questions for the owner** (batched): section 14. The main ones: how images and files get from nodes at all (new helper calls?); whether a node copy also changes the hub's own clipboard; the chord and the key for plain paste; limits; whether "paste" also types Ctrl+V on the node.

---

## 1. What the owner already decided (this document follows it)

All SOURCE: the task text of 2026-10-06 and `HUB-OS.md` (read 2026-10-06); not discussed again.

**Clipboard (the owner's list):**
- A bar item. History of every copy on any node and on the hub, with source machine and time, searchable.
- Targeted paste to one or several nodes.
- Rich items: UTF-8 text; images with preview; files as references that move node to node only on paste; the hub never carries the file bytes.
- A quick picker chord. Paste as plain text. No echo loops.
- History in RAM; pins saved to disk. About 500 text items plus about 100 MB for images and file references.
- Node copies are always captured into history and never pushed to another node without the owner's action.

**Context decisions:** no display passwords and no encryption on display connections; the hub's requests to nodes are not signed; the hub's core will be a session layer (one worker process per machine with connection, frame in shared memory, sound stream, clipboard, volume; a presenter that draws frames and sends input back; a conductor in Elixir/OTP supervising the workers; `hubd` stays in Go; frames are uncompressed Raw rectangles, security type None, RFB Extended Clipboard (UTF-8), no ContinuousUpdates, no Fence, key events by keysym, reconnect with backoff); the native-core language is not decided (bake-off later), so the clipboard components are separate processes behind small local interfaces; every node screen is 4K 60 fps uncompressed; the bar has a clock, the clipboard, the notification daemon and more later; one display only; the hub never carries file bytes.

**Rules that apply:** `hubd` is Go; no systemd; no new streaming protocol; no web dashboard as the hub's control plane; the hub is a client (it runs only viewers); the hub must not freeze (priority 1 is uptime and stability, `HUB-OS.md`); secrets never in git.

**One document that is now out of date:** `docs/proposals/node-helper-api.md` describes signed requests with one-time nonces. The owner has since decided that the hub's requests to nodes are not signed. I did not edit that file. Every call proposed here is unsigned.

---

## 2. What exists today (SOURCE, read 2026-10-06, unless marked)

| Piece | State | Source |
|---|---|---|
| Waybar on the top edge, 30 px; one custom module `custom/hub` that runs `hubd feed --socket /run/hubos/hubd.sock` (streams JSON lines) and `hubd menu` on click; a `clock` module | exists | `image/machines/hub/rootfs/etc/hubos/waybar.json` |
| The feed line is `{"text","class","tooltip"}`; the feed writes at most one line per second, never an identical line, and keeps the last state marked STALE if `hubd` is gone | exists | `internal/hub/view.go`, `internal/hub/ipc.go` function `feed` |
| wofi as the list launcher, started per click, a fixed `--height`, never `--lines` (a sizing bug on driftwm), a CSS file `/etc/hubos/wofi.css` | exists | `cmd/hubd/slice2.go` lines 374-471; `docs/bar-findings.md` |
| A clipboard bridge prototype (text only, Go, `wl-copy`/`wl-paste`, echo guard, signing not included) | experiment only | `tools/image/experiments/clipboard-bridge/` |
| Session layer, conductor, workers, presenter | **do not exist**; owner decision only | task text |
| Node helper | **does not exist**; a design in `docs/proposals/node-helper-api.md` | repo |
| wl-clipboard in the hub image | **not in** `image/packages/hub.list` (the list has wayland-utils, foot, waybar, wofi, jq, dbus-daemon) | `image/packages/hub.list` |
| Services as s6 services run by user `hub`, waiting for driftwm with `follow-driftwm` | exist | `image/machines/hub/rootfs/etc/s6/sv/*/run`, `docs/proposals/hub-isolation.md` section 2 |
| Layout files: a SHA-256 of the compact JSON inside each file, written to a temporary file, flushed, renamed over, a bad file moved aside ("quarantined") | exist | `internal/hub/layout.go` lines 26-300 |
| D-Bus session bus (`dbus-daemon` alone) at `/run/dw/bus` | exists | `docs/proposals/hub-isolation.md` row 5; `tools/image/hub_test.go` `hubEnv` |

---

## 3. What the display connection can and cannot carry (the facts under everything)

All SOURCE unless marked, from the files I downloaded on 2026-10-06 into a scratch folder: neatvnc `v1.0.3` `src/server.c`, `include/rfb-proto.h`, `include/neatvnc.h`; wayvnc `v0.10.2` `src/data-control.c`, `src/main.c`; the RFB specification `rfbproto.rst` (master), section "Extended Clipboard Pseudo-Encoding".

1. **The RFB Extended Clipboard format list** is text (UTF-8), rtf, html, dib (a Windows bitmap) and files; "files" is "currently reserved but not defined". Text uses CRLF line ends and ends with a NUL byte. (`rfbproto.rst`, lines 4777-4925.)
2. **neatvnc offers only the text format.** Its capability message sets `RFB_EXT_CLIPBOARD_FORMAT_TEXT` and all four actions (`send_ext_clipboard_caps`, `src/server.c:1409-1426`); a request or provide for any other format is ignored (`src/server.c:1644-1658`). Its public call to send a clipboard to viewers is `nvnc_send_cut_text(server, const char* text, uint32_t len)`: text only (`include/neatvnc.h:782`). **So no image and no file list can come from a node through wayvnc.**
3. **wayvnc only watches for the type `text/plain;charset=utf-8`** (`self->mime_type`, `src/data-control.c:618`). A node program that copies only `image/png` makes **no** event on the hub. (TESTED indirectly: `wl-copy --type image/png` offers only `image/png`; section 12, T1.)
4. **The flow neatvnc uses (always notify, then request, then provide).** It tells viewers "do not send unsolicited text" (`max_unsolicited_text_size = 0`, `src/server.c:1409-1426`). A node copy therefore produces a **Notify** (no text inside); the viewer must send a **Request**; the server then sends the **Provide** (zlib-compressed: a 32-bit length, the text, a NUL). In the other direction the hub sends a Notify, the server answers with a Request, and the hub sends the Provide. TESTED against my own stand-in (section 12, F1-F6); SOURCE for the real server.
5. **The server keeps one provide buffer: the latest text** (`ext_clipboard_save_provide_msg`, `src/server.c:1808-1852`). Two copies on the node so quickly that the second arrives before the hub's Request is answered give the hub only the second. BELIEVED consequence: a very fast pair of copies collapses into one entry.
6. **A Peek action exists.** The hub can ask "what formats do you have now" and gets a Notify (`src/server.c:1649-1651`). This is how a restarted worker can ask for the current node clipboard once (proposal, section 8). Not run against real neatvnc.
7. **CRLF.** neatvnc turns CRLF into LF for text it receives from a viewer (`crlf_to_lf`, `src/server.c:1471`, used at line 1560). Text it **sends** is the node's text as it is (LF), not CRLF as the specification says (`ext_clipboard_save_provide_msg` copies the bytes). So the store must treat CRLF and LF as the same text when comparing (TESTED in the stand-in, F1).
8. **Size walls.** A viewer message above `MAX_CUT_TEXT_SIZE` = 10,000,000 bytes makes neatvnc close that viewer's connection (`include/server.h:41`, `src/server.c:1597-1604`, `1673-1678`; SOURCE (repo, `node-helper-api.md` 8.1)). wayvnc reads the node's selection with no size limit in `data-control.c`. The specification's default maximum for text from a client is 20 MiB, and some implementations 10 MiB. So **the hub must never send more than 10,000,000 bytes of text to a node through RFB**, or the node closes the session: this is a hard cap for the "paste to node" path, whatever limit the owner chooses.
9. **One watcher per viewer, broadcast to all viewers.** In wayvnc each connected viewer has its own `data_control` object (`struct wayvnc_client`, `client_init_data_control`, `src/main.c:1954-1969`), with its own random marker type `x-wayvnc-client-<8 hex>` (`src/data-control.c:618-621`). A selection that carries a viewer's **own** marker is ignored by that viewer's watcher (`is_own_offer`, `src/data-control.c:206-254`), but **not** by the other viewers' watchers. A received selection is sent with `nvnc_send_cut_text`, which notifies **all** viewers (`src/data-control.c:92`, `src/server.c:1867-1900`). BELIEVED consequences (not run, because wayvnc was not run): with the hub and the phone both connected, (a) one copy on the node reaches the hub **twice**; (b) a text the hub pushes through its viewer sets the node's selection with the hub viewer's marker, the phone viewer's watcher does not ignore it and broadcasts it, and the hub's own viewer receives it as an announcement of "a new copy" (an echo). With a single viewer there is no echo from wayvnc.
10. **wayvnc does not report that the node's clipboard was cleared.** It sends only non-empty text (`src/data-control.c:90-93`), and a cleared or lost selection produces no message. So the store cannot know a node's clipboard went empty. (SOURCE.)
11. **wayvnc sets the node clipboard from viewer text for both the regular and the primary selection** and ignores empty text (SOURCE (repo) `node-helper-api.md` 8.1, from `src/data-control.c`).
12. **The old viewer problem (focus, `remote-viewer` never sends text) does not apply** to the hub's own session layer, which will speak the messages itself: BELIEVED (the problems were in third-party viewers; `docs/proposals/remote-display.md` R2.1, SOURCE (repo)).

---

## 4. The components (proposal)

```
 node A clipboard      node B clipboard                         hub desktop
   |                      |                                        |
 wayvnc (text only)     wayvnc                              wl-paste --watch
   | RFB ext. clipboard   |                                        |
 session worker A      session worker B                      hub watcher (capture)
   |  \                   |  \                                     |
   |   \ local socket ----+---\--------------- clipd (the store) ---+
   |                                              |  ^  \
   |<--- "set node clipboard" commands -----------+  |   \ pins file on /config
   |                                                  |
 node helper (images, file refs: section 6)           |  local socket
                                           clipctl feed ---> Waybar module (bar item)
                                           clip-pick   ---> wofi list (picker), actions
```

| # | Component | What it does | Runs as | Exists? |
|---|---|---|---|---|
| C1 | **`clipd`, the store** | Keeps the list in RAM (items, previews, pins). Applies the limits and eviction. Writes pins to disk. Answers `list`, `get`, `pin`, `delete`, `paste`. Is the only owner of history. | its own s6 service, user `hub` | no |
| C2 | **Capture in each session worker** | Part of the session layer's worker for machine X. On a node Notify: request, receive, send an `add` to `clipd`. On a `set` command from `clipd`: send Notify, answer the Request with the Provide. Reports "connected / stale". | the worker (language not decided) | no |
| C3 | **Hub watcher** | Watches the hub's own Wayland clipboard and sends an `add` to `clipd`; sets the hub clipboard on a `paste to hub` command. | its own s6 service, user `hub` (needs the Wayland socket, so it waits for the compositor like `hubd` does) | no |
| C4 | **Helper-API capture (images, file references)** | For machines whose helper offers it (section 6): fetches non-text items. May live in the worker or in `clipd`; open (question 4). | open | no |
| C5 | **Picker** | A list on the screen: shows items, search, pick; then an action (to hub, to node(s), plain). Closes by itself after a limit. | started by the chord or the bar; not a service | no |
| C6 | **Bar item** | `clipctl feed` prints JSON lines for a Waybar custom module (same method as `hubd feed`); click opens the picker. | child of Waybar | no |
| C7 | **File reference move** | On "paste files to X": ask the node helper for the destination, send tickets, tell node X to fetch from node A. Uses `docs/proposals/file-transfer.md` unchanged. | inside `clipd` or `hubd` (open, question 4) | no |

Why separate processes (reasoned, BELIEVED to match the isolation work): `docs/proposals/hub-isolation.md` question 7 asks whether the clipboard client should be a separate process; its observation is that nothing may sit on the path of the compositor or of `hubd`'s panel (SOURCE (repo), lines 142 and 322). A store that hangs must not freeze the bar or `hubd`.

**What the interface looks like (proposal, not a protocol anyone must keep):** a Unix stream socket `/run/hubos/clipd.sock`, mode 0600, in the folder `/run/hubos` that `hubd` already uses (SOURCE `internal/hub/ipc.go`). Each message is one JSON object on one line, each request has an `id`, each reply echoes it. A client sets a deadline on every call. The same pattern as `hubd`'s socket (SOURCE `internal/hub/ipc.go`, `Call` dials with a 2 s timeout). Any language can speak it.

Messages (all BELIEVED/proposal):

```json
{"id":1,"cmd":"add","item":{ ...item record, section 5.1, with "data" inline... }}
{"id":2,"cmd":"list","filter":"error 5","limit":50,"kinds":["text","image","files"]}
{"id":3,"cmd":"get","item":"c-000123","as":"text"}
{"id":4,"cmd":"pin","item":"c-000123","on":true}
{"id":5,"cmd":"paste","item":"c-000123","to":["hub","ai-1","desk-2"],"plain":false}
{"id":6,"cmd":"feed"}                       // the connection then receives a line whenever the summary changes
{"id":7,"cmd":"status"}
```

and, for workers and the hub watcher (they connect to the store, the store never connects out):

```json
{"cmd":"hello","who":"worker","machine":"ai-1"}
// store -> worker:
{"cmd":"set","req":"p-77","item":"c-000123","kind":"text","data":"..."}
// worker -> store:
{"cmd":"set-result","req":"p-77","result":"ok|timeout|node_gone|too_large"}
```

---

## 5. The data formats (proposal)

### 5.1 The item record

```json
{
  "id": "c-000123",
  "seq": 123,
  "t_ms": 1791285228327,
  "source": {"kind": "node", "machine": "ai-1"},
  "kind": "text",
  "mime": "text/plain;charset=utf-8",
  "bytes": 28,
  "sha256": "cc2cf0c4d01a...",
  "state": "ok",
  "preview": "café ✓ 日本 line2",
  "data_ref": "mem",
  "pinned": false,
  "count": 1,
  "last_t_ms": 1791285228327,
  "echo_of": null,
  "note": ""
}
```

| Field | Meaning | Limits |
|---|---|---|
| `id` | `c-` and a counter that never repeats while `clipd` runs; after a restart it starts again from the highest pinned id + 1 | at most 16 bytes |
| `t_ms` / `last_t_ms` | first and most recent time this exact content was copied on this source (Unix milliseconds, the hub's clock) | |
| `source.kind` / `machine` | `node` or `hub`; the machine id from the inventory (the same text rule as everywhere: free text under the machine-id rule) | machine id as in the inventory |
| `kind` | `text`, `image`, `files` | |
| `mime` | text: `text/plain;charset=utf-8`; image: `image/png` first (question 8); files: `application/x-hubos-file-refs` (a name I made up as a label, not a registered type) | |
| `bytes`, `sha256` | size and hash of the content after CRLF to LF (text) | |
| `state` | `ok`, `unavailable` (the node did not answer in time), `too_large` (only this record is kept, no data), `evicted` is never shown (an evicted item is gone) | |
| `preview` | text: the first 200 characters with line breaks shown as `↵`; image: a short label such as `PNG 1920x1080 6.2 MB` | text at most 800 bytes |
| `thumb` | image only: id of a small preview image in the store (size is question 9) | |
| `files` | `files` kind only: `{"machine":"nas","items":[{"path":"/abs/a.txt","name":"a.txt","kind":"file","bytes":123}]}`, the same shape as the file-transfer adapter's `selection` answer (SOURCE (repo) `file-transfer.md` 5.1) | at most 100 entries (the file-transfer open question 10; question 11 here) |
| `pinned` | saved to disk | |
| `count` | how often the same content was seen from the same source in a row (de-duplication, section 9) | |
| `echo_of` | set only on records the store keeps for diagnosis (question 14); normally an echo is not stored | |

An item's `data` (the text, the PNG bytes) is held separately from the record so a `list` answer stays small: a `list` answer carries records and previews only; `get` carries the data.

### 5.2 The pins file

Proposal, modelled on the layout files that already exist (SOURCE `internal/hub/layout.go`): JSON with the SHA-256 of its own compact content inside, written to a temporary file in the same folder, flushed, renamed over, the folder flushed; a file that fails the checksum is renamed aside and the store starts with no pins and says so (notification, `notifications.md`).

```json
{"format": 1, "saved_ms": 1791285228327, "next_id": 124,
 "items": [ {"id":"c-000100","t_ms":...,"source":{...},"kind":"text","bytes":28,"sha256":"...","pinned":true,"text":"..."},
            {"id":"c-000101","kind":"image","bytes":612345,"sha256":"ab12...","blob":"blobs/ab12....png"} ],
 "sha256": "<sha-256 of the compact JSON of everything above>"}
```

- File: `/config/hubos/clipboard/pins.json`, blobs beside it in `blobs/<sha256>`; folder owned by the user that runs `clipd` (same pattern as `/config/hubos/layouts`, SOURCE `hub/rootfs/etc/s6/sv/hubd/run`). Whether it is in the NAS backup list is question 15.
- A `files` pin stores only the reference list, never bytes.
- Sizes and the pin limit are questions (10, 11). Proposal default (not decided): at most 50 pins, pins file at most 20 MB.
- The write happens in its own thread with a deadline; a failed write keeps the pin in RAM, marks the bar "pins not saved" and sends a notification (section 8). It never blocks `add`, `list` or `paste`.

---

## 6. Where non-text items come from (the gap in section 3)

Because the display connection carries text only (section 3, items 2 and 3), the owner's "images" and "files" need another path from nodes. **I propose, I do not decide:** two optional calls in the node helper API, add-only under its `/v1` rule (SOURCE (repo) `node-helper-api.md` 3.2). **All of this is BELIEVED/proposal: no helper exists.** Unsigned, per the owner's decision.

| Call | Method | Meaning |
|---|---|---|
| `/v1/clipboard/changes` | `GET`, query `after=<seq>&wait=<seconds>` | Long poll: returns when the node clipboard has a change newer than `after`, or after `wait` seconds (at most 25) with an empty list. The answer carries **descriptions, not bytes**. |
| `/v1/clipboard/item` | `GET`, query `seq=<seq>&type=<mime>` | The bytes of one type of one change (images). Refused with `413 too_large` over the limit. |

```json
// GET /v1/clipboard/changes?after=41&wait=20
{"boot_id":"6f1c9a0e","seq_max":43,"dropped":0,
 "changes":[
   {"seq":42,"t_ms":1791285228327,"types":[{"mime":"text/plain;charset=utf-8","bytes":28},{"mime":"image/png","bytes":612345}]},
   {"seq":43,"t_ms":1791285229001,"types":[{"mime":"text/uri-list","bytes":53}],
    "files":[{"path":"/srv/a.txt","name":"a.txt","kind":"file","bytes":123}]}
 ]}
```

Fields: `boot_id` (changes when the helper restarts, so the hub can tell "same counter" from "counter reset"), `seq_max`, `dropped` (changes lost because the node kept only its last 32; the hub shows "N copies missed"), per change `seq`, `t_ms` (the node's clock; the hub also stamps arrival), `types` (mime and size), optional `files`.

Limits (proposal): at most 32 changes kept on the node; `wait` at most 25 s; a call that cannot complete in `wait` + 5 s is abandoned by the hub; an image over the per-item cap (question 12) is described but `item` answers `413`; `GET /v1/clipboard/item` for one type at a time.

**What this costs and what it conflicts with (stated, not resolved):**
- The helper "answers only when asked; it never calls the hub" (SOURCE (repo) `node-helper-api.md` 3.1). A long poll respects that; it keeps one open request per node. With 20 nodes that is 20 requests in flight. (Question 4.)
- On the node, the helper would watch its own Wayland clipboard with `wl-paste --watch` (BELIEVED to work on the node's compositor; the same limits as section 12) or, for file copies, depend on the GUI's adapter (`file-transfer.md` 5.1). Whether a node's file manager **tells** the helper about every file copy, or only when the owner presses the hub's "copy files" action (the file-transfer design), is open: the file-transfer design (SOURCE (repo) `file-transfer.md` 2.2) keeps file references **out** of the Wayland clipboard and has the hub ask the adapter when the owner presses a key. Under that design file items are created by the owner's action on the hub, not captured from every node copy. The owner's wording ("node copies are always captured") could be read either way. (Question 5.)
- Text from nodes does **not** need this call; it arrives through the worker (C2). If the helper's text types are also listed in `changes`, text would arrive twice (once through RFB, once here): the store's de-duplication (section 9) covers it, but a cleaner rule is "text over RFB, only non-text over the helper" (question 4).

---

## 7. The picker, the chord, paste targets and plain text

### 7.1 Picker (proposal: first a wofi list; an own picker later)

- **wofi dmenu** already works as the machine list (SOURCE `cmd/hubd/slice2.go`; `docs/bar-findings.md`): list on stdin, the picked line on stdout, search is built in (wofi `--matching` modes `contains`, `multi-contains`, `fuzzy`; `--insensitive`; SOURCE `wofi(1)` from `wofi 1.4.1-1build2`, unpacked 2026-10-06).
- A line per item, single line only: `ai-1   12:03   café ✓ 日本 line2` plus an index token at the start, because a picked line comes back as text (TESTED earlier for headings: "a heading can be picked", `docs/bar-findings.md` 320-321). Multi-line texts are shown on one line with `↵`; long ones are cut at about 100 characters (the line, not the stored text).
- **wofi has a feature to show images in dmenu lines** (an "image escape" in the line, enabled with `--allow-images`; `wofi(1)` says "Allows image escape sequences to be processed and rendered"). The exact syntax and how big the picture is were **not** tested (I did not run wofi; it needs GTK libraries I did not unpack). **UNKNOWN** whether a thumbnail per line works under driftwm and how large it can be. An own picker (a layer-shell program) is the alternative (question 9).
- **wofi remembers picks and reorders by them** unless given `--cache-file /dev/null` (SOURCE (repo) `docs/bar-findings.md` 320): history order must be ours, so the picker passes that. (SOURCE (repo); it is the same lesson as for the machine list.)
- **Only one picker at a time:** the bar script guards with `pgrep -x wofi` (SOURCE (repo) `bar-findings.md` 325-334).
- **A ceiling:** the picker runs under `timeout` (proposal: 120 s) so an abandoned list closes.
- wofi cannot select several lines (BELIEVED: I know of no multi-select in wofi 1.4.1; not tested). So "to several nodes" is two steps (pick the item, then pick targets one by one or choose "all open machines"). Question 7.

### 7.2 The chord

driftwm runs a command on a key chord with the `spawn` action; `mod+v` is not used by the default bindings (SOURCE (repo) `node-helper-api.md` 8.3, from driftwm `config.reference.toml` at the pinned commit; not re-read by me). `docs/driftwm-findings.md` (SOURCE (repo)) says compositor shortcuts take precedence over the window's keys unless a window rule sets `pass_keys = true`. **Which chord, and a second key for plain paste, are the owner's choice** (question 1); `HUB-OS.md` also says the chord is chosen after checking driftwm's bindings, which I did not repeat.

### 7.3 Targeted paste

What "paste to node X" means here is **set node X's clipboard** (proposal): `clipd` sends `set` to worker X, the worker does Notify / Request / Provide (section 3, item 4), and the owner then presses the usual paste key in X's window. Whether the hub should also send Ctrl+V to the node (key events by keysym exist in the session layer, task text) is open (question 6). Rules:

| Target | How | Limit |
|---|---|---|
| hub | `wl-copy` with the stored type | per-item cap |
| a node, text | worker: RFB Notify, answer the Request with the Provide | **at most 10,000,000 bytes** (section 3, item 8), lower by the owner's cap |
| a node, image | helper API (a call that sets a non-text clipboard does not exist; proposal `PUT /v1/clipboard/item`, section 6 style) | UNKNOWN; question 4 |
| a node, files | the file-transfer flow: destination, tickets, receive (SOURCE (repo) `file-transfer.md` 2.2) | as there; the hub carries no bytes |
| several nodes | one `set` per node, in parallel, each with its own deadline; the result per node is shown ("ok on ai-1, timeout on nas") | each at most the deadline of section 8 |

An image or file item pasted as plain text is shown as its description (a text label or the file paths, one per line); an image has no plain-text form, so the plain option is greyed out in the picker (proposal; question 3).

### 7.4 Paste as plain text

The store keeps the text of an item as UTF-8 with no formatting (the owner's list has no rich text type), so for text items "plain" changes little: it strips what a "rich" paste would add. What is stored from a hub copy that offered both `text/html` and `text/plain` is the plain text only (question 3, which also asks whether HTML should be kept). Plain paste on a node pastes the same text; the node's own application decides whether it keeps formatting. UNKNOWN: whether there is any difference the owner expects. I list it as a question rather than invent a rule.

---

## 8. Deadlines and failure behaviour

**Rules for the whole design (proposal):** every call between two parts has a deadline; nothing waits without one; a part that fails is marked **STALE** (the same mark `hubd` already uses, SOURCE `HUB-OS.md`: "the bar and menu keep the last known state marked STALE while hubd is down"); a failed part never makes the bar or the picker freeze. **Every number in the next table is a guess to be measured, not a decision or a measurement.** (BELIEVED; the only measured numbers are the loopback results in section 12.)

| Part | What the owner sees if it dies | If it hangs or is slow | Restart | Deadline (guess) |
|---|---|---|---|---|
| **`clipd` (store)** | The bar item shows "clipboard: history unavailable" in the alert colour, marked STALE with the time of the last good state. After the restart the list is empty (RAM only), the pins come back from disk, and a notification says "history was lost, pins kept" (needs `notifd`). Workers and the hub watcher keep a **small** queue (last 8 items, at most 16 MiB) and send it when `clipd` is back; older ones are dropped and counted ("N copies missed"). | `clipctl feed` and the picker give up after 500 ms and show "history not answering"; the bar line is marked STALE after 3 s without a heartbeat. A hung store is not restarted by s6 (s6 restarts only a dead process): a liveness probe that kills it is needed; **UNKNOWN who does it** (question 13). | s6 restarts it at once (SOURCE (repo): s6 restarts a dead service); a crash storm ends in the init's "degraded and alert" rule (`HUB-OS.md`, init comparison) | call 500 ms; heartbeat 1 s; STALE after 3 s |
| **A session worker** | The conductor restarts it with backoff (`HUB-OS.md`: reconnect with backoff). That machine's window shows the session layer's own "reconnecting" state; in the clipboard list its entries stay, and the machine is marked stale ("last seen 12:03"). New copies on it during the gap are lost **except the latest**: on reconnect the worker sends a Peek and requests the current text once (section 3, item 6; BELIEVED to work, not run on real neatvnc). That one is recorded with a note "after reconnect". | A node that never answers a Request: the entry is recorded as `unavailable` after the deadline and the session is marked stale in the list; **the worker never blocks its frame or sound work on the clipboard** (a rule for the session layer's design; question 13). TESTED in the stand-in: silent node gives an `unavailable` entry after the 0.5 s test deadline (F5); a cut connection is noticed and marked stale (F6). | the conductor | Notify to Request: 2 s; Request to Provide: 2 s + 1 s per MiB of announced size; push: 5 s |
| **Hub watcher** | Copies made on the hub are not captured; the bar item shows "hub copy watcher down" (small marker, not red). | A `wl-paste --watch` hangs: BELIEVED harmless to the screen because it is a separate process; its output is read with a deadline | s6; on restart its first event is the current selection and is dropped if equal to the last stored one (TESTED: `--watch` runs once at start for the existing selection, section 12 T2) | read 2 s |
| **The picker (wofi)** | Nothing on the screen (it did not open) and a one-line message on the bar | `timeout 120` closes it; the picker never holds `clipd` (it asks, shows, then asks again to act) | started again by the next chord | list 500 ms; whole picker 120 s |
| **Bar feed (`clipctl feed`)** | Waybar restarts a custom module's command (`restart-interval`; SOURCE `waybar-custom(5)` from Waybar 0.9.24, read 2026-10-06); the bar shows the last line | the feed writes at most one line per second (same rule as `hubd feed`) | Waybar | 1 s |
| **The pins file** | A pin that cannot be saved stays in RAM and the bar shows "pins not saved". A bad file at start is renamed aside, the store starts with no pins, a notification says so. | The write runs in its own thread with a 5 s deadline; `add` and `list` never wait for it | at the next change | 5 s |
| **Hub clipboard set (`wl-copy`)** | "could not set the hub clipboard" on the bar | `wl-copy` is run with a 2 s deadline and its holder process is not waited for beyond that; TESTED that the holder stays and exits when the next copy replaces it (SOURCE (repo) `node-helper-api.md` 8.8; the init must collect orphans: BELIEVED for s6) | | 2 s |
| **A paste to a node** | The picker action reports per node: "ok" or "ai-1: no answer in 5 s". A node that is down is skipped at once ("nas is down") because `hubd` knows. | the worker answers `timeout` at the deadline | | 5 s per node, nodes in parallel |
| **The helper-API call** (images, files) | The machine's non-text copies are missed; the list shows the machine with a "no image capture" mark | long poll `wait` + 5 s | the hub retries with backoff | `wait` + 5 s |

Two cross-checks with decisions already taken:
- **The compositor gave up** (5 crashes in a minute, then no restart; `hubd` and the recovery terminal stay up, SOURCE `HUB-OS.md`): there is **no Wayland display**, so the picker, the bar and the hub watcher cannot work. `clipd` and `hubd` keep running (the `hubd` run script starts it without the compositor in that state, SOURCE `image/machines/hub/rootfs/etc/s6/sv/hubd/run`, `FOLLOW_GAVEUP=run`). The clipboard has nothing to show until the compositor returns. UNKNOWN whether `clipd` should wait for it like `hubd` (question 13).
- **The hub never reboots itself for a service failure** (SOURCE `HUB-OS.md`): no failure in this document may lead to a reboot; the only outcome is an alert.

---

## 9. Limits, sizes, eviction, huge items, de-duplication

### 9.1 Budget (the owner's numbers, my reading)

"About 500 text items plus about 100 MB for images and file references." I read this as two separate budgets: **text: at most 500 entries**; **images + file references: at most about 100 MB of data together**. File references are tiny (a list of paths, a few hundred bytes), so in practice the 100 MB is for images. (Question 12 asks the owner to confirm, and asks for a byte cap on the text, which the owner has not given.)

### 9.2 Rules (proposal)

| Situation | Rule (default; not decided) |
|---|---|
| New text and 500 text entries exist | The oldest **unpinned** text entry is removed first. |
| New image and the image budget would be exceeded | The oldest unpinned image or file entries are removed until it fits. |
| Everything of that kind is pinned | The new item is **not stored**; the bar shows "clipboard full of pins" and a notification is sent once. (Alternative: evict the oldest pin: not proposed because it destroys what the owner asked to keep.) |
| A single item above the per-item cap | **Not stored.** A record with `state: too_large`, size and source is stored (a few bytes), so the list says "12 MB from ai-1: too large". |
| A single item above the whole budget | Same as above. |
| "Oldest" | By `last_t_ms`: an entry copied again moves to the top. |
| Per-item caps | Text: 1 MiB (proposed in `node-helper-api.md`; the RFB path has a hard wall of 10,000,000 bytes, section 3 item 8). Image: 25 MB (a 4K noise PNG I made is 24.9 MB; real screenshots are far smaller; **a guess**). Files: 100 entries. All three are questions. |
| Memory of the store | At worst: 500 x 1 MiB text + 100 MB images = about 600 MiB of data (plus previews and thumbnails). With a 256 KiB text cap about 225 MiB. **Whether that fits** the "viewers at most 25% of RAM" group or needs its own limit is UNKNOWN (question 12). Proposal: `clipd` runs in a memory-limited group and, when it reaches the limit, evicts early; it must not be allowed to take the viewers' memory. |
| Thumbnails | Made when an image is stored; the size is a question (9); counted in the 100 MB. Who decodes the PNG is not decided (BELIEVED: needs an image library; no such library is chosen). |

### 9.3 De-duplication (proposal)

- Same source (machine or hub) + same `sha256` within **2 seconds** = one entry (this removes the double announcement of section 3 item 9a). Count not increased.
- Same source + same `sha256` as the **previous** entry from that source: do not make a new entry; update `last_t_ms` and `count`.
- The same content from a different machine is a separate entry (the source is part of the record).
- Whether an old identical entry anywhere in the list is merged (a global de-duplication) is a question (14).

### 9.4 Echo-loop prevention

What can start a loop: a text that arrives at a node's clipboard (our own push) is announced by wayvnc again (section 3 item 9b) and would be recorded as a "new copy from the node"; if anything pushed new copies automatically, the next push would start the cycle. **Because node copies are never forwarded automatically (owner decision), there is no loop to stop;** the rules below keep the history clean and are a second line of defence if automatic forwarding is ever added.

| # | Rule | Label |
|---|---|---|
| E1 | **No automatic push of any item to any node.** Pushes come only from the picker or a chord. | owner decision; it removes the loop by construction |
| E2 | **A push registry in `clipd`:** for every push, remember (target machine, `sha256`, time) for 3 seconds. A node copy from that machine with the same hash within the window is marked `echo_of` the pushed item and is **not stored as a new entry**. | TESTED against my stand-in (F3); real wayvnc: BELIEVED, section 3 item 9 |
| E3 | **Hub side:** when `clipd` sets the hub clipboard with `wl-copy`, remember the hash for 3 seconds; the hub watcher's event with the same hash inside the window is dropped. | BELIEVED (from the `wl-paste --watch` events seen in section 12: every `wl-copy` makes an event, also for identical text) |
| E4 | **Compare after turning CRLF into LF** and the trailing line break of the paste program removed, so a viewer that changes line ends does not defeat the rule. | TESTED for CRLF in the stand-in (F1) and in the earlier bridge experiment (SOURCE (repo) `node-helper-api.md` 8.6, D5) |
| E5 | **A rate limit:** more than 20 `add` messages from one source in 10 seconds are dropped and counted. | the number is a guess; BELIEVED harmless |
| E6 | **A source marker inside the clipboard** is not possible with `wl-copy` (it offers one type plus text aliases; SOURCE (repo) `node-helper-api.md` 8.6 TESTED; here TESTED again: `wl-copy` text offers `text/plain`, `text/plain;charset=utf-8`, `TEXT`, `STRING`, `UTF8_STRING`, section 12 T1). An own small Wayland client could add one. Not proposed. | TESTED |
| Known limit | A rule by hash fails if a viewer **changes** the text (the earlier study saw `café` become `cafÃ©` with `remote-viewer`, SOURCE (repo) `remote-display.md` R2.1). With the session layer's own RFB code this should not happen: BELIEVED. | |

Whether a node copy also **changes the hub's own clipboard** (as it does today through a viewer) is open (question 2). If it does, E3 is needed on every node copy; if it does not, the hub clipboard changes only by the owner's picker and by programs on the hub.

---

## 10. The bar item (a Waybar custom module)

**How it works today (SOURCE (repo), TESTED there):** a custom module whose `exec` is a long-running program printing one JSON object per line (`text`, `class`, `tooltip`); Waybar updates when a line arrives (`docs/bar-findings.md` section 3: a `tail -F` feed updated in under half a second; a signal update took 17 ms). `on-click` runs a command; clicking the bar does not take keyboard focus away from the window below (TESTED there). `on-click-right`, `on-click-middle`, `on-scroll-up` and `on-scroll-down` exist as option names (same file). `restart-interval` re-runs a script that exits (SOURCE `waybar-custom(5)`, Waybar 0.9.24, read 2026-10-06). I did **not** run Waybar again.

**Proposal for the item (nothing built):**

```json
"custom/clip": { "exec": "/usr/bin/clipctl feed --socket /run/hubos/clipd.sock", "return-type": "json", "format": "{}",
                 "on-click": "/usr/bin/clipctl pick", "on-click-right": "/usr/bin/clipctl menu" }
```

- Text: a short label plus the count (for example `clip 37`); class `ok`, `stale` or `alert`; tooltip: the last 5 entries (one line each, with machine and age), escaped for Pango markup the way `hubd feed` escapes (SOURCE `internal/hub/view.go`: "the text and tooltip are escaped for markup, so the module must be configured without Waybar's own escape").
- What the bar shows when things are wrong: `stale` (grey) when `clipd` has not answered for 3 s; `alert` (red) when the store is gone, pins cannot be saved, or the store is full of pins. A bar item that is red for a fault the owner cannot fix would train them to ignore it: question 14 asks what counts as red.
- Modules and CSS names in `waybar.css` get `#custom-clip` (same pattern as `#custom-hub`).
- Where the module sits in the bar (left, centre, right) and the order with the notification item are the owner's (question 1).
- `hubd feed` marks STALE after three probe intervals without a result (SOURCE `internal/hub/view.go` `Counts.Stale`); the clipboard feed copies that behaviour.

---

## 11. File references moving node to node (use of the file-transfer proposal, unchanged)

**SOURCE (repo, `docs/proposals/file-transfer.md`, not re-read in full again; read 2026-10-06 once):**
1. The owner highlights files in node A's window and presses the hub's "copy files" action. `hubd` asks A's helper `GET /v1/files/selection` and keeps the **list** (paths, names, kinds, sizes) with the machine id and time. No bytes.
2. At "paste files" in node B's window, `hubd` asks B's helper for the open folder (`destination`), signs a one-time ticket per item (under the new decision the signing of requests is gone; whether the **ticket** keeps its signature is a question for that proposal: the owner said requests are not signed, and a ticket is not a request, but I did not decide), and tells B to fetch from A. A and B transfer directly.
3. The receiver writes into the open folder with the rules R1-R8 (no overwrite, `name (2)`, a temporary name, size check).

**What this document adds:** a `files` item in the history records (machine, list, time) and shows it in the picker as "5 files from nas (ai-1 selection) 12:03"; "paste to X" starts step 2 for the chosen target. A `files` item **expires in the sense that the source files may have changed**: before step 2 the hub asks A to confirm that each path still exists with the same size; otherwise the paste fails with a plain message ("a.txt changed on nas"). BELIEVED/proposal.

**Not solved here:** which node GUI reports what is highlighted (the adapter, UNKNOWN until a GUI exists); the drag gesture (file-transfer section 3, separate). The file bytes **never** pass through `clipd` or the hub.

---

## 12. What I ran (all TESTED unless marked; scratch work in the scratchpad folder and `/tmp/cn1`, `/tmp/cn2`, deleted afterwards)

**Environment.** Ubuntu 24.04.4 container, no GPU, loopback only, nothing installed. Programs from `apt-get download` (Ubuntu 24.04: `sway 1.9`, `wl-clipboard 2.2.1-1build1`, `mako-notifier 1.8.0-2build2`, `grim`, `wayland-utils`, wlroots and the other libraries listed in `tools/image/experiments/clipboard-bridge/README.md`; `wofi 1.4.1-1build2` and `waybar 0.9.24-1build3` for their manual pages only), unpacked with `dpkg -x` into a scratch folder and run from there. The headless compositor is `sway` with `WLR_BACKENDS=headless WLR_RENDERER=pixman`, a stand-in for the hub's driftwm (driftwm itself was **not** run; `wl-paste` under driftwm is still UNKNOWN, as in `HUB-OS.md`). `ulimit -c 0` in every shell. The runtime folder has to be short (`/tmp/cn1`): my first attempt in a long folder name failed with "failed to create display" because a Unix socket path is limited to about 108 characters (a lesson for the test scripts).

**Compositor globals (command `wayland-info | grep ...` on the headless sway):**

```
interface: 'zwlr_layer_shell_v1',          version:  4, name:  9
interface: 'zwlr_screencopy_manager_v1',   version:  3, name: 24
interface: 'zwlr_data_control_manager_v1', version:  2, name: 25
interface: 'wl_output',                    version:  4, name: 45
```

**T1. Types a copy offers (script `t2.sh`, scratch folder):**

```
$ echo -n hello | wl-copy; wl-paste -l
text/plain / text/plain;charset=utf-8 / TEXT / STRING / UTF8_STRING
$ wl-copy --type image/png < small.png; wl-paste -l
image/png
$ printf 'file:///srv/a.txt\r\n' | wl-copy --type text/uri-list; wl-paste -l
text/uri-list / text/plain / text/plain;charset=utf-8 / TEXT / STRING / UTF8_STRING
```

Meaning: a PNG copy offers only `image/png` (so wayvnc, which wants `text/plain;charset=utf-8`, never sees it); a `text/uri-list` copy also offers the plain-text types (so a file copy by a program that does this would leak `file://` lines into the text clipboard; the same finding as `file-transfer.md` F1, TESTED again).

**T2. Change events of `wl-paste --watch` (a script that prints `CLIPBOARD_STATE` and the byte count):**

```
event state=data bytes=19      <- the item already there at start (the uri-list)
event state=data bytes=3       <- "one"
event state=data bytes=3       <- "one" copied again: an event again
event state=data bytes=3       <- "two"
event state=data bytes=410     <- the 64x64 PNG
event state=nil  bytes=0       <- wl-copy --clear
event state=data bytes=9       <- wl-copy -f "held-text"
event state=nil  bytes=0       <- the holder was killed: the selection is gone
```

Meaning: (1) a watcher always gets one event for what is already on the clipboard when it starts; (2) copying identical content again makes another event (the store must de-duplicate, section 9.3); (3) a cleared selection and a dead holder both give `nil`: **a clipboard manager on the hub can see that the thing it holds vanished** (for example a program closes) and may choose to re-offer the last item; whether it should is question 2; (4) the watcher received the PNG bytes (410) because the default type is the first one offered.

**T3. Image through `wl-copy`/`wl-paste` (6.2 MB and 24.9 MB noise PNGs):**

```
big  identical; copy 0.043 s, paste 0.062 s, bytes 6223838
huge identical; copy 0.197 s, paste 0.123 s, bytes 24892993
```

**T4. A watcher reading a 24.9 MB image:** `event state=data bytes=24892993` (one event, the data went through the watcher's command).

**T5. Plain text without the extra line break:** `printf 'a\nb' | wl-copy; wl-paste -n | od -c` printed `a \n b` and nothing after (3 bytes). `-n` is what makes "paste as plain text" byte-exact.

**T6. A watcher eats a paste-once item.** `echo -n once | wl-copy --paste-once` with a `wl-paste --watch` running: the next `wl-paste` printed nothing and the one after said `Nothing is copied`. The watcher's own read counted as the one paste. **Consequence (TESTED, a surprise):** a history watcher on the hub will consume the single paste of any hub program that offers a one-time item (BELIEVED: password managers do), so the owner's real paste fails. The store must either not capture on the hub with a `--watch` that reads data eagerly, or accept this; question 14.

**F1-F6. The RFB Extended Clipboard flow against a fake server.** `tools/image/experiments/clipboard-notifications/fakerfb.py` (Python standard library only, about 240 lines, throwaway; it is both the fake node and a stand-in for the worker's clipboard part, written by me, so a pass shows self-consistency and agreement with the specification and neatvnc's source as I read them, **not** compatibility with real wayvnc). Command and output:

```
$ python3 -I tools/image/experiments/clipboard-notifications/fakerfb.py
PASS 1 node copy reaches the worker, UTF-8 kept, CRLF -> LF
PASS 2 hub push reaches the node
PASS 3 echo of a push is marked, not stored as a new copy
PASS 4 9,000,000-byte text received  0.15 s
PASS 5 silent node: request times out, entry marked unavailable, session marked stale
PASS 6 connection cut mid-message: worker notices, marks stale, does not hang
ALL PASS
```

(F1 to F6 are rows 1 to 6.) It does the handshake (RFB 3.8, security None, ServerInit, `SetEncodings` with the Extended Clipboard pseudo-encoding `-1063131698`), the server's caps message, Notify / Request / Provide in both directions with the zlib payload and the trailing NUL, and signed (negative) lengths. Loopback timings (0.15 s for 9 MB) say nothing about a real network.

**What I did not run:** wayvnc, neatvnc, `remote-viewer`, driftwm, Waybar, wofi (only its manual page), foot, the QEMU hub image, a long run, a second RFB client.

---

## 13. What can be tested in the nested hub image, and the first slice

### 13.1 What can be tested where

| Test | Where it can run | Label |
|---|---|---|
| Store rules: eviction at 500 text entries, the 100 MB budget, pins first, too-large records, de-duplication, push registry (echo), CRLF, pin file write, checksum, corrupt file quarantined, restart keeps pins | plain unit tests, no compositor, on this machine | BELIEVED easy; the model for the pin file is `internal/hub/layout_test.go` (SOURCE (repo)) |
| Worker capture against a fake RFB node (Notify / Request / Provide, 9 MB text, silent node, cut connection, a second viewer's echo) | this machine; `fakerfb.py` is the model; a Go or native fake server is the real one | TESTED for the stand-in (section 12) |
| Hub watcher and hub clipboard set with `wl-paste --watch` / `wl-copy` | this machine on headless sway (TESTED T1-T6); in the hub image only once `wl-clipboard` is added to `image/packages/hub.list` (not in it today) | TESTED here; image: BELIEVED |
| Bar item (`clipctl feed` through Waybar) and the picker (wofi) | needs Waybar + wofi + a layer-shell compositor: the nested hub image has them (SOURCE `hub.list`); earlier sessions ran the same pattern for `hubd feed` and `hubd menu` | SOURCE (repo); not re-run |
| Everything on the real image (s6 services, `follow-driftwm`, memory group, kill -9 of `clipd`, driftwm instead of sway) | `go test -tags qemu -count=1 -timeout 150m -v ./tools/image` builds and boots the hub image under QEMU in TCG (SOURCE `tools/image/hub_test.go`, header comment: the run is over two hours long). **I did not run it:** other helpers run QEMU tests and the rules say to keep my runs short. | UNKNOWN until run |
| A real wayvnc plus a fake application that copies | needs wayvnc built from source at `v0.10.2` and neatvnc `v1.0.3` (not built in this session) and a node compositor; possible later in the same QEMU machinery with a second guest | UNKNOWN |
| Two viewers on one wayvnc (duplicate and echo behaviour, section 3 item 9) | same as above | UNKNOWN: **this is the test that matters most for E2** |

Fake nodes that exist: `tools/fakenode` only accepts and closes TCP connections (SOURCE `tools/fakenode/main.go`: "It sends and reads nothing"); a fake RFB node with Extended Clipboard does **not** exist in the repository (my `fakerfb.py` is a throwaway test script).

### 13.2 First slice (smallest useful thing; proposal)

**Slice C1: store, hub watcher, command line (text only).**
- `clipd` with the item record, RAM store, limits and eviction, de-duplication, push registry; Unix socket with `add`, `list`, `get`, `pin`, `delete`, `status`.
- Hub watcher: `wl-paste --watch` with a tiny wrapper that reads `CLIPBOARD_STATE` and the data (section 12 T2), sends `add`.
- `clipctl list | get | paste --to hub | pin`.
- Pins file with checksum and atomic write.
- Tests: the unit list above; the `wl-copy`/`wl-paste` tests on headless sway (T1-T6 as a script with expected output); a kill -9 of `clipd` and restart keeps the pins and loses the history.

**Slice C2: bar item and picker.** `clipctl feed` with STALE after 3 s; Waybar module in a test config; wofi picker with the single-instance guard, `timeout`, `--cache-file /dev/null`; actions `to hub` and `plain`. Tests: feed output when `clipd` is stopped, hung (SIGSTOP) and back; the picker returns after 500 ms of silence with a message; a click on the module opens the picker (the earlier xdotool-like click test pattern, SOURCE (repo) `docs/image.md` "The wofi menu").

**Slice C3: node text through a worker.** Fake RFB node (Go) with Extended Clipboard; a worker stand-in speaking the flow of section 3; `add` from it; `paste --to ai-1` through the worker; echo registry; duplicate announcement from a second viewer simulated by the fake node; deadline tests (silent node, cut connection, 10,000,000-byte wall).

**Later slices (not first):** images from the hub; images from nodes through the helper API; file references with the file-transfer flow; own picker with thumbnails; Ctrl+V sending; phone web page view (not asked).

**What the first slice does not need:** the session layer, the conductor, the node helper, wayvnc or driftwm. It can be built and tested here, in Go (the repository has Go 1.24 and the existing experiment is Go) **if the owner accepts Go for the throwaway first slice**; the interfaces do not depend on that (question 4). I did not check that a Rust or Elixir toolchain would build the slice here (`cargo` exists on the machine; `elixir` does not: TESTED with `which`).

---

## 14. Questions for the owner (batched; nothing here is decided)

**Chord, keys, bar**
1. **Chords and bar position.** Which chord opens the picker? Which key (in the picker or a second chord) pastes as plain text? Where does the clipboard item sit in the bar and in which order with the notifications item? (I only know that `mod+v` is not used by driftwm's default bindings, SOURCE (repo); I did not check the other chords in the hub's config.)
2. **Does a node copy change the hub clipboard?** Today a viewer would do it. Options: (A) never: the hub clipboard changes only when the owner picks or a hub program copies; (B) yes, automatically (then the hub watcher must ignore it; section 9.4 E3). Also: when the program that holds the hub clipboard exits (the selection becomes empty, T2), should the hub re-offer the last item?
3. **Plain text and rich text.** Should HTML or RTF from hub programs be stored at all, or only the plain text? What does "paste as plain text" mean for an image (disabled) and for files (paths as text lines)?

**Architecture**
4. **Who captures non-text from nodes.** wayvnc/neatvnc carry text only (section 3). Are the proposed helper calls (`/v1/clipboard/changes` long poll, `/v1/clipboard/item`) acceptable, which means 20 open long polls and breaks the helper's "never called by the node" feel only mildly (it still never calls the hub)? Or is text-only from nodes enough for v1? Should the store, the worker or `hubd` own the helper polling? Which language for the first slice (Go is the only one I can test here with the repository's tooling; the interfaces do not depend on it)? Separate process for the store, or inside `hubd` (`hub-isolation.md` question 7)?
5. **How a file copy on a node becomes a history entry.** Only when the owner presses the hub's "copy files" action (the file-transfer design), or also when the node's GUI says "files copied" (needs a helper-to-hub path)?
6. **Does "paste to a node" also type Ctrl+V** into the node's focused window (key events by keysym exist), or only set the node's clipboard so the owner pastes?
7. **Several targets.** wofi cannot multi-select (BELIEVED). Is two-step picking acceptable (pick the item, then pick "all open machines" or a list), or do you want an own picker (needed anyway for image thumbnails)?

**Limits and look**
8. **Image formats.** PNG only, or also JPEG/BMP? (Clipboards from some programs offer others; `wl-copy` offers whatever the source offers.) Which one is kept when several are offered?
9. **Preview size.** Thumbnail size in pixels, and whether the first picker may be text-only until an own picker exists.
10. **Pin limit and pin file size.** (Default shown: 50 pins, 20 MB.)
11. **File reference limit** per item (default shown: 100 entries) and how old a `files` item may be before it is refused.
12. **Byte limits.** A byte cap for the 500 text entries (or a per-item cap: 1 MiB gives 500 MiB worst case); the per-item cap for images (default shown 25 MB); whether `clipd` gets its own memory limit group. Confirm that "about 100 MB" is one budget for images plus file references.
13. **Failure handling choices.** Who kills a hung `clipd` (a probe in the feed, the supervisor, or `hubd`)? Should a worker's clipboard part be allowed to be disabled automatically if it misbehaves, so frames and sound are never affected? Should `clipd` run while the compositor is given up?
14. **History hygiene.** Should copies marked as secrets (a password manager's hint type, BELIEVED `x-kde-passwordManagerHint`, from memory) be left out? A history in RAM holds every password copied on the hub; pins on disk hold what you pin. Should echoes be shown in the list (greyed) or hidden? Global or consecutive-only de-duplication? What counts as red on the bar? Is the `--paste-once` consumption (T6) acceptable, or should the hub watcher avoid reading data until the owner opens the picker (it would then miss copies)?
15. **Pins file and backups.** Is `/config/hubos/clipboard` in the NAS backup list (pinned items may hold secrets and large images)?
16. **Notifications link.** The clipboard sends "history lost, pins kept", "pins not saved", "clipboard full of pins" to the notification daemon (`notifications.md`). Which severities?

---

## 15. Table of items

| # | Item | Label |
|---|---|---|
| 1 | neatvnc advertises only the text format; no other formats are handled | SOURCE (neatvnc v1.0.3 `src/server.c:1409-1426, 1644-1658`) |
| 2 | wayvnc watches only `text/plain;charset=utf-8` | SOURCE (wayvnc v0.10.2 `src/data-control.c:618`) |
| 3 | Notify, Request, Provide; server never sends unsolicited text (max size 0) | SOURCE; stand-in TESTED |
| 4 | One provide buffer: rapid copies collapse | SOURCE reading; behaviour BELIEVED |
| 5 | Each viewer has its own watcher and every watcher broadcasts to all viewers; duplicates and cross-viewer echo with two viewers | SOURCE reading; behaviour BELIEVED (not run) |
| 6 | Cleared node clipboard is not reported | SOURCE |
| 7 | Peek action exists | SOURCE; use after reconnect BELIEVED |
| 8 | 10,000,000-byte wall closes the viewer's connection | SOURCE |
| 9 | RFB Extended Clipboard layout (flags, caps, zlib provide, NUL, CRLF) | SOURCE (`rfbproto.rst`); stand-in TESTED |
| 10 | Fake RFB node + worker stand-in: UTF-8, push, echo, 9 MB, silent node, cut connection | TESTED (my own stand-in; not real wayvnc) |
| 11 | `wl-copy`/`wl-paste` types, events, clear, killed holder, 6 MB and 25 MB images, plain paste | TESTED (headless sway 1.9, wl-clipboard 2.2.1) |
| 12 | A history watcher consumes `--paste-once` | TESTED |
| 13 | mako, wofi, Waybar manual-page facts | SOURCE (unpacked packages) |
| 14 | wofi image lines in dmenu, multi-select | UNKNOWN / BELIEVED not available |
| 15 | wl-paste under driftwm | UNKNOWN |
| 16 | Hub image has no wl-clipboard | SOURCE (`image/packages/hub.list`) |
| 17 | Helper calls for images and file changes | PROPOSAL, BELIEVED workable |
| 18 | Sizes, deadlines, caps in this document | BELIEVED (guesses) except the loopback timings |
| 19 | The session layer's clipboard behaviour (reconnect Peek, no blocking of frames) | PROPOSAL; UNKNOWN until built |

## 16. Sources read (2026-10-06)

- This repository: `CLAUDE.md`, `HUB-OS.md`, `docs/proposals/node-helper-api.md`, `docs/proposals/file-transfer.md`, `docs/proposals/hub-isolation.md` (component table), `docs/proposals/hub-stability.md` (section 7.3 and the restart-loop lines), `docs/proposals/remote-display.md` (clipboard parts), `docs/viewers-research.md` (clipboard lines), `docs/bar-findings.md` (grep of module and wofi parts), `internal/hub/ipc.go`, `internal/hub/view.go`, `internal/hub/layout.go`, `cmd/hubd/slice2.go` (wofi part), `tools/image/hub_test.go` (header and test names), `tools/fakenode/main.go`, `tools/image/experiments/clipboard-bridge/README.md`, `image/machines/hub/rootfs/etc/hubos/waybar.json`, `waybar.css`, `driftwm.toml`, `image/machines/hub/rootfs/etc/s6/sv/hubd/run`, `waybar/run`, `image/packages/hub.list`.
- neatvnc `v1.0.3`: `src/server.c`, `include/rfb-proto.h`, `include/neatvnc.h`; wayvnc `v0.10.2`: `src/data-control.c`, `src/main.c` (downloaded with `curl` from the raw file URLs, whole files); `rfbproto.rst` of the RFB specification (master), section "Extended Clipboard Pseudo-Encoding".
- Manual pages from unpacked Ubuntu 24.04 packages: `wl-copy(1)`, `wl-paste(1)` (wl-clipboard 2.2.1), `wofi(1)`, `wofi(7)` (wofi 1.4.1), `waybar-custom(5)` (waybar 0.9.24).
- Not used: web search, summarising tools, other helpers' reports.
