# PROPOSAL: moving files between nodes (a file clipboard, drag and drop, the transfer, and what each node GUI must do)

**Status: PROPOSAL, research only.** Nothing here is decided and nothing here is built into the images or into hubd. `HUB-OS.md` wins if this file disagrees with it. Written 2026-10-05. The experiments are in `tools/image/experiments/layouts-and-file-transfer/` (test programs and scripts under `t/` and `ft/`; the full output of one complete run is `results.txt`; "results D2" below means the section "D2" of that file). This document builds on `docs/proposals/node-helper-api.md` (the node helper API and the text clipboard bridge).

**Labels on every item:**
- **TESTED**: I ran it in this build environment; the script and what came out are given (or are in `results.txt`).
- **SOURCE**: read in a file at a named place, date read 2026-10-05. "(repo)" means a file of this repository written by an earlier session and not re-checked by me.
- **BELIEVED**: I think it is true, I did not test it and have no source.
- **UNKNOWN**: nobody has checked; do not rely on it.

**The owner's assumptions for this study:** network speed and network security are assumed perfect; the hub only authorizes a transfer and never carries the bytes. So this document does not measure real-network speed and does not argue about attackers; where a safety check is cheap I still say so.

---

## Plain-words summary

1. **The smallest design that I think is honest** is a "file clipboard" in two steps, like the text clipboard bridge that already exists on paper. The owner highlights files in one node's window and presses a hub key or bar button ("copy files from here"); later, in another node's window, presses "paste files here". The hub asks the first node what is highlighted, asks the second node which folder is open, signs one small one-time ticket per file or folder, and tells the second node to fetch them. **The second node pulls the bytes straight from the first node. The hub only passes the tickets** (a few hundred bytes each). I built a toy version with two fake nodes on one machine and it worked, including a folder, odd file names and a 20 MB file (section 2, results F5).
2. **Real drag and drop between node windows is a different, harder thing.** On the hub a node's window is just a picture of the node's screen (VNC). The drag happens inside the node, so the hub's drag-and-drop machinery (Wayland's) is never involved. When the owner presses the mouse in one window, **that window keeps the mouse until the button is let go**, even when the pointer is over another window. I tested this: the other window sees nothing until the release (section 3, results D1). Three ways to build a gesture on top of that were compared: **a driftwm patch, an invisible full-screen "overlay" window, or a key-armed overlay.** I tested the two overlay variants: an overlay shown *during* a drag learns only where the button was let go; one shown *before* the press sees the whole drag, including every position (results D3, D4). I do not recommend building drag and drop first; I recommend copy and paste first and then, if you want the gesture, the key-armed overlay in hubd. That is a design choice for you (section 3.5).
3. **For moving the bytes I compared seven ways** (section 4). My pick for copying files and folders is **plain HTTP with one-time signed tickets, served by the node helper that already exists on every node**: no new daemon, no mount, works the same on every distro, the hub keeps control. I tested a toy of it: valid ticket works once, a second use, an expired ticket, a changed ticket, a path outside the shared folder and a ticket for another node are all refused; folders travel as a tar stream; a 1 GiB file went through with the sender using about 9 MB of memory; an interrupted download resumes with a **new** ticket. `rsync` as a one-shot daemon and `sftp` also worked in tests but neither is one-time by itself. NFS and SMB are mounts, not copies; I did not test them. `zfs send` is the right tool for moving a whole dataset between the NAS and the backup NAS and for nothing else; I could not test it here (no ZFS).
4. **What every node GUI must do is small:** two commands it must answer (`selection`: "which files are highlighted?", `destination`: "which folder is open?") and two optional ones (`destination-at`: "which folder is under this point?", `refresh`: "new files arrived"). It is an executable that prints one line of JSON, so any distro and any language can provide it. I wrote a **conformance test** (a single static program) with 18 checks; the reference adapter passes all 18 and **each of 14 deliberately broken adapters fails the check meant to catch it** (section 5). The test cannot prove that a real GUI really reports what the user highlighted; for that there is a ten-point manual list.
5. **What went wrong or could not be tested** is in section 7: one resumed download of the toy ended short once in five runs (cause unknown); NFS, SMB, `zfs send`, `sshd` and any real viewer were not tested; I could not try a drag that crosses two real node windows.
6. **Questions for the owner** are in section 8.

---

## 1. Where this fits (what is already decided)

All SOURCE (repo, `HUB-OS.md` and `docs/proposals/node-helper-api.md`, read 2026-10-05), except the point about distros, which is from the lead's instructions for this task (the owner's latest decisions, not yet written into `HUB-OS.md`):
- The hub is a client: it never carries workload data. Nodes show their whole desktop in one window; there are no terminals; each node has its own bespoke GUI.
- Every node runs the **node helper**: a small program that answers signed HTTP requests from `hubd` (one cluster management key; the hub holds the private half; nodes hold public keys). It "answers only when asked; it never calls the hub". The API is versioned `/v1`, add-only.
- Text clipboard: node to hub through the display protocol (wayvnc), hub to node through the helper (`PUT /v1/clipboard`), started by the owner (button or key chord), never automatic.
- Each node may run its own distro (systemd and musl allowed on nodes, per the owner's latest note); so nothing here may depend on one distro's file manager, service manager or libc.
- `hubd` is Go. No new streaming protocol, no web dashboard as the control plane.

---

## 2. Part 1: the file clipboard

### 2.1 Why not the Wayland clipboard

A file manager normally puts a list of file addresses on the Wayland clipboard as the type `text/uri-list`. I put such a list on a compositor with `wl-copy --type text/uri-list` and looked at it:

- `wl-copy` offered **`text/uri-list` and also `text/plain`, `text/plain;charset=utf-8`, `TEXT`, `STRING`, `UTF8_STRING`**; reading it as plain text returned the `file:///...` lines. TESTED (results F1).
- The display program wayvnc asks the node's clipboard for `text/plain;charset=utf-8` and sends what it gets to the hub's viewer (SOURCE (repo) `docs/proposals/node-helper-api.md` section 8.1, from wayvnc v0.10.2 `src/data-control.c`; not re-read by me).
- So if a node's file manager also offers plain text for a file copy, the **file addresses would travel to the hub's text clipboard**. For `wl-copy` that is TESTED; for real GNOME, KDE or Thunar file managers I believe they offer more than one type including text (BELIEVED, from memory of how those programs behave; not checked).

Therefore: **file references stay out of the Wayland clipboard.** The node GUI keeps its own selection and tells the helper about it through the small adapter of section 5. The text clipboard bridge is untouched.

### 2.2 The flow (proposal)

Actors: `hubd` on the hub; helper A and GUI A on the source node; helper B and GUI B on the target node.

| Step | Who | What | Label |
|---|---|---|---|
| 1 | owner | highlights files in node A's window; presses the hub key or bar button "copy files" | proposal |
| 2 | hubd | finds the machine of the focused window (the same lookup as for the text push) and asks helper A: `GET /v1/files/selection` (signed) | proposal; the lookup is BELIEVED, not built (repo `node-helper-api.md` 8.3) |
| 3 | helper A | runs the adapter `selection`, returns `{"items":[{"path","name","kind","bytes"}]}`; hubd keeps the list in memory with the machine id and the time (not the bytes, not on disk) | prototype of the adapter TESTED (results F5) |
| 4 | owner | in node B's window presses "paste files" | proposal |
| 5 | hubd | asks helper B: `GET /v1/files/destination` (signed) | proposal |
| 6 | helper B | runs the adapter `destination`, returns `{"dir":"/abs/path"}` or `{"dir":""}` if no folder is open (then hubd shows a plain message and stops) | TESTED (results F4, F5) |
| 7 | hubd | for each item signs one **ticket** (section 2.3): "node A may give this path to node B once, before this time" | prototype TESTED (results F2) |
| 8 | hubd | tells helper B: `POST /v1/files/receive` with the sender's address and the list of tickets (a few hundred bytes each) | proposal |
| 9 | helper B | for each ticket: `GET http://A:port/t/<ticket>`; writes into the folder with the receiver rules of 2.4 | prototype TESTED (results F5) |
| 10 | helper A | checks the ticket, streams the file, or a folder as a tar stream | prototype TESTED (results F2) |
| 11 | hubd | polls helper B: `GET /v1/files/transfers/<id>` until done or failed; shows "5 files, 20 MB, ai-1" or the error on the bar | proposal |
| 12 | helper B | optionally calls the adapter `refresh` so the GUI shows the new files | TESTED (the adapter side only) |

**The hub never reads or forwards file bytes**: it handles lists and tickets. In my toy this is true by construction (the "hub" part of `t/te2e.sh` only runs the adapters, `jq` and the signing command). I did not measure the traffic of the hub's network card (loopback only), so "the hub carries no bytes" is by design, not by measurement.

The calls follow the helper API's rules (signed with the management key, nonce, versioned, add-only, errors with a `code`), but I did not write them into `node-helper-api.md` and I did not implement them.

### 2.3 The ticket (one-time signed URL)

My prototype ticket is `base64url(JSON) "." base64url(signature)`, the JSON being `{"v":1,"sender":"nas","receiver":"ai-1","path":"/srv/share/a.txt","exp":<unix seconds>,"nonce":"<16 random bytes in hex>"}`, signed with an Ed25519 key. The production scheme would reuse the helper API's signing (the same management key and the `signify`-compatible signature, SOURCE (repo) `node-helper-api.md` 3.3) instead of my plain Ed25519 key file. What the sending helper checks, in this order: the signature against its key list; `sender` is itself; the expiry; the path (after resolving links) is inside one of the folders the helper is configured to share; the nonce was not used before. Then it streams. Tested results (all TESTED, results F2, `t/tft.sh`, program `ft/main.go`):

| Test | Result |
|---|---|
| Valid ticket, small file | 200; content identical; the file's modification time comes through in `Last-Modified` |
| Same ticket again | 403 `token already used` |
| Expired ticket | 403 `expired` |
| One character of the ticket changed | 403 `bad signature` |
| Ticket made for another sender | 403 |
| Correctly signed ticket for `/etc/hostname`, outside the shared folder | 403 `path outside the shared root` |
| Correctly signed ticket for a link inside the folder that points outside | 403 (the link is resolved before the check) |
| Names `-leading dash.txt`, `my file (1) ünï 日本.txt`, one containing a line break | all identical after transfer (the name travels inside the signed ticket and as a header, never on a command line) |
| A folder (tar stream): empty directory, nested file, mode 0755 script, a link inside, a link pointing outside | file list identical; mode kept; **both links are carried as links** (the link to `/etc/hostname` came out as a link to `/etc/hostname`). The receiver must therefore never follow links when it extracts (rule R5 below), or the sender must leave links out. |
| 1 GiB file on loopback | 1.9 s in one run, 2.8 s in another; the sending program's peak memory was about 9.5 MB (`VmHWM`). The speed is meaningless for a real network. |
| Interrupted download, resume | The same ticket with a `Range` header is refused (403: the ticket is used up). A **new** ticket with `Range: bytes=N-` continued the file and the result was identical in 4 runs of 5. |
| One flaky run | In the one complete run recorded in `results.txt` the resumed file came out **212 MB instead of 1 GiB** and `cmp` said `EOF`. In the other four runs (one before, three after) it was identical. I do not know why (the machine was shared with other jobs). **Consequence for the design: the receiver must compare the size it received with the size announced in the list (`bytes`) before it renames the file into place, and fail loudly if they differ.** |

What the toy does not do: keep the used-nonce list across a restart of the sender (memory only: after a restart a ticket could be used once more until it expires), limit the number of parallel downloads, authenticate the receiver (anyone with the ticket can fetch it; with the owner's "perfect network" assumption that is acceptable), or talk TLS.

### 2.4 Rules for the receiving helper (it, not the GUI, writes the files)

Putting these in the helper means they are written and tested once, not once per GUI.

| # | Rule | Label |
|---|---|---|
| R1 | Write only inside the folder the adapter named; ask the adapter again at receive time, do not take a folder from the hub | proposal |
| R2 | Refuse names that are empty, `.` or `..`, or contain `/` or a NUL | the toy's shell code has the check for `/`, empty, `.`, `..`; **no test fed it such a name**; NUL not handled |
| R3 | Write to a hidden temporary name in the same folder, check the size against the announced size, then rename into place (atomic on one file system) | TESTED (toy) |
| R4 | If the name exists, use `name (2)`, `name (3)`, never overwrite | TESTED (toy: second paste gave `data.bin (2)`, `proj (2)`) |
| R5 | Extracting a folder: never follow or create links that leave the destination; refuse absolute names and `..` | the toy used `tar --no-same-owner --no-same-permissions`; the link case above shows it is **not enough**; the real helper does its own extraction in Go and checks every name. Not built. |
| R6 | The files belong to the GUI's user, not to the helper's user | UNKNOWN how (question 6) |
| R7 | A folder that is half written is removed (hidden temporary name) | the toy writes into a hidden temporary folder and renames it; the failure case was **not** tested, and a crash mid-transfer leaves a hidden `.hubos-part-*` that nothing cleans yet |
| R8 | Report progress and the final state to `GET /v1/files/transfers/<id>`; keep the last few for a minute | proposal |

### 2.5 What the hub triggers and how (same open choices as the text push)

The text push in `node-helper-api.md` 8.3 offers: a bar button, a key chord (driftwm's `spawn` action runs a command, SOURCE (repo) there), or both. The same applies here. The target of "copy" is the machine of the focused window; the target of "paste" likewise. The key chord goes to driftwm first (SOURCE (repo) `docs/driftwm-findings.md` section 6: compositor shortcuts take precedence unless a window rule passes keys). Nothing here changes that. The choice is question 5.

---

## 3. Part 2: drag and drop across windows

### 3.1 How drag and drop works between two Wayland programs

SOURCE: `wayland.xml` of libwayland 1.22.0 (Ubuntu `libwayland-dev 1.22.0-2.1build1`, unpacked and read 2026-10-05), interfaces `wl_data_device`, `wl_data_source`, `wl_data_offer`:
- The source program asks the compositor to start a drag with `wl_data_device.start_drag(source, origin_surface, icon, serial)`. "The origin surface is the surface where the drag originates **and the client must have an active implicit grab that matches the serial**."
- The compositor then sends the drag's `enter`, `motion` and `leave` events to whatever program's surface is under the pointer, with a `wl_data_offer` that lists the types the source offers; the target accepts a type, and at release the compositor sends `drop`. The target asks the source for the bytes with `wl_data_offer.receive(type, fd)`; the source writes into the file descriptor; the target ends with `finish`. The bytes go between the two programs, not through the compositor.

TESTED under driftwm (results D2; programs `dndclient.py`, `vptr.py`): two small clients, A offering `text/uri-list` and B accepting it, with the pointer pressed in A, moved across to B and released. A logged `start_drag`, then `data_device.leave` when the pointer left it; B logged `offer.mime text/uri-list`, `data_device.enter`, `motion`, `drop`, and read `b'file:///srv/share/report.txt\r\n'`; A logged `dnd_drop_performed`, `send` and `dnd_finished`. **So driftwm routes a real Wayland drag between two clients correctly.** (driftwm hands the drag to Smithay's drag-and-drop grab: SOURCE `src/handlers/mod.rs:145-193`.)

### 3.2 Why this is not what happens to a node window

A node's window on the hub is a viewer showing the node's screen as pixels. The mouse button, the movement and the release are sent to the node as plain pointer events. The *node's* GUI starts and ends the drag inside the node's own compositor. **The hub's compositor never sees a drag-and-drop**, only a button held down while the pointer moves. (SOURCE (repo) `HUB-OS.md`: the hub "runs nothing but viewers"; display protocol VNC. Whether a viewer such as `remote-viewer` with SPICE can take a file dropped on its window and hand it to a guest, as `spice-gtk` does, is BELIEVED from memory and is about VM guests, not about nodes using VNC.)

### 3.3 Why the viewer "holds" the pointer (the implicit grab)

While a mouse button is down, a Wayland compositor keeps sending the pointer's events to the window where the button went down, even if the pointer is moved over a different window. TESTED (results D1): window A (a plain client that starts no drag, red) at screen x 140 to 540 and window B (blue) to the right; the pointer pressed in A, moved in steps to a point over B, released there.

```
A  pointer.button state=1
A  pointer.motion x=260 ... x=360 ... x=460 ... x=560 ... x=660 ... x=760 ... x=800   (A is 400 wide: x up to 800 means far outside A)
A  pointer.button state=0     then   A  pointer.leave
B  pointer.enter  (only after the release)
```

B received **nothing** during the drag. A kept receiving motion with coordinates far outside its own surface. driftwm's own focus marker stayed on A. Consequences:
- A viewer in the middle of a drag keeps getting coordinates beyond its window. What a real viewer does with them (clamp to the edge, ignore, keep sending) is UNKNOWN; I used test clients, not remote-viewer, TigerVNC or Moonlight.
- If the owner drags something in node A's GUI toward node B's window, node A's GUI sees the pointer reach the edge of its screen and stay there; a release then happens "at the edge of node A's screen", which may be a button or a panel. BELIEVED; not tested; an accident risk for any plain pointer-based design.
- Nobody except the compositor sees where the button is let go. A program on the hub learns it only through the compositor or through a window that gets the pointer after the release (section 3.4).

### 3.4 What driftwm can see, and what an outside program can see

| Who | Can see | Label |
|---|---|---|
| driftwm itself | every pointer event (it reads the input devices), every window's place and size, the pointer focus, and, for a Wayland-native drag, the data source (`dnd_requested` receives it) | SOURCE `src/handlers/mod.rs:145-193`, `src/input/pointer.rs` |
| An outside program using driftwm's socket | window list with places and sizes, camera, zoom, focus. **Not the pointer position and not the buttons.** The only event the socket pushes is a whole-state snapshot (`Event::State`). | SOURCE `src/ipc/protocol.rs` (`Request`, `Event`); TESTED (`driftwm msg state` output has no pointer field) |
| A layer-shell client (an overlay) | Only what the compositor sends to its surface: pointer events when the pointer is over it and no other window holds a grab | TESTED (section 3.5) |
| A program reading `/dev/input` on the hub | Raw relative motion, no absolute position, and no knowledge of the compositor's acceleration or clamping. How the shared keyboard and mouse reach the hub (the forwarder) is not built. | BELIEVED; UNKNOWN for the forwarder |

### 3.5 Three ways to build a drag gesture, compared

An overlay here means a transparent, full-screen window from the layer-shell protocol (driftwm supports it: `zwlr_layer_shell_v1` v5, SOURCE (repo) `docs/driftwm-findings.md` section 5), placed in the topmost layer, able to receive the pointer. I wrote one in Python (`overlay.py`; test-only).

**Test D3 (overlay shown during the drag; results D3).** Press in A. Then map the overlay (it came up, 1280x800). Move the pointer over B and release. The overlay got **no event at all** while the button was down (A kept the grab). **At the release the overlay received `pointer.enter` at exactly the release point** (x=940, y=420, the point I injected), and A received the button release. So an overlay that appears after a drag has started learns **where** the drag ended and nothing about the path. It cannot draw a highlight that follows the pointer, because it gets no motion.

**Test D4 (overlay shown before the press, "armed"; results D4).** The overlay was mapped first. Then press, move, release. The overlay received the press, **every motion with its position**, and the release at x=940 y=420. Windows A and B received **nothing** except A's `pointer.leave` when the overlay appeared. So an armed overlay sees the whole gesture and can draw while it happens, **at the cost that it takes the mouse for itself**: the click that starts the gesture is not delivered to the window below. Looking up the window under a point worked in this test: with the camera at 0, 0, zoom 1 and a 1280x800 screen, screen point (940, 420) is canvas x = 940 - 640 = 300 and y = 400 - 420 = -20 (Y up), which is inside B (centre 300, 0; size 400x300 from `driftwm msg state`). With another zoom, camera or a bar the formula changes (BELIEVED; not tested).

| | A. Patch driftwm | B. Overlay shown at the start of a drag | C. Armed overlay (key-armed hub gesture) |
|---|---|---|---|
| How it works | Add to driftwm a socket request such as "arm" and an event "button released over window W at x, y" | `hubd` maps the overlay when node A says "a file drag started" | The owner presses a key chord ("send files"); a driftwm binding runs a command that maps the overlay; the owner presses in A, drags, releases over B; the overlay tells `hubd` |
| What it learns | Press and release places (and every motion if wanted) | Only the release place | Press, every motion, release |
| Feedback while dragging | The compositor can draw a highlight | None (the overlay gets no motion) | The overlay can draw (not tested) |
| Needs from the node | A "drag started" signal (or arming), plus `selection` and `destination x,y` | A "drag started" signal **from the node to the hub**, which the helper API does not have (the helper never calls the hub; `hubd` would have to poll or hold a long request) | Nothing but `selection` and `destination x,y`: the drag only chooses source and target windows; the files are what is highlighted in A |
| Tested | No | Yes (D3) | Yes (D4) |
| Cost | Rust in a 77,000-line, single-maintainer, pre-1.0 compositor that we must keep patched against upstream (SOURCE (repo) `HUB-OS.md`: "expect to carry patches"); the project's own tests are expected for a change (its `AGENTS.md`, BELIEVED from the findings file). Effort: UNKNOWN; I did not write it. | Small; one more program in hubd; but the timing problem above | Small; one more program in hubd plus a key binding; the odd part for the owner is the extra key press |
| Risk | Merge pain on every driftwm update | A late signal means a missed drop | While armed, no other click works; needs a time-out and a way to cancel (the overlay needs keyboard input for Esc: `set_keyboard_interactivity` was **not** tested) |
| Works with a viewer that is not a Wayland drag source | Yes | Yes | Yes |

Also considered: reading the mouse from `/dev/input` on the hub gives only relative motion, so it cannot find the window under the pointer (BELIEVED); not pursued.

**My recommendation (a design choice, so the owner decides):** build the file clipboard first. If the gesture is wanted afterwards, build **C**, because it needs nothing from the nodes beyond what the clipboard needs, it needs no driftwm patch, and I tested its two hard parts (the overlay sees the whole drag; the release point maps to a window). I would not patch driftwm for this.

What C does not give: a drag that starts on one particular file under the pointer (the files are the highlighted ones), and the familiar drag icon.

### 3.6 What I could not test

A drag that crosses two real node windows; what real viewers send when the pointer leaves their window; keyboard input to the overlay (to cancel it); drawing in the overlay; the overlay with zoom below 1, with the bar, or with more than one screen; the hub's real input path (forwarder). The pointer for these tests was injected into a headless sway with the `zwlr_virtual_pointer_v1` protocol (a test-only injector, `vptr.py`), so the events are as the compositor sees them from a real mouse, but no real input device was involved.

---

## 4. Part 3: the transfer itself

Network speed and security are assumed perfect (the owner's rule). The hub only authorizes: it never carries the bytes. What each way **needs** and **costs** is what differs.

| | HTTP, plain | **HTTP + one-time signed ticket (pick)** | rsync | SFTP | SMB | NFS | `zfs send` |
|---|---|---|---|---|---|---|---|
| What the sender runs | a file server on a fixed folder | the same, but the helper (already there) serves a ticket path | an rsync daemon, or `sshd` + `rsync` | `sshd` with its sftp subsystem (or a bare `sftp-server`) | Samba or `ksmbd` (a server and its config) | a kernel NFS server or `nfs-ganesha`, exports file | `zfs send` (ZFS on the sender) |
| What the receiver runs | `curl`-like client (any language) | the helper's own downloader | the `rsync` client | an sftp client | a kernel CIFS mount (root) | a kernel NFS mount (root) | `zfs receive` (ZFS on the receiver) |
| Hub's part | tells the receiver to fetch | **signs one ticket per item** | would have to create a one-time password or key per transfer and start a daemon | the same, with sshd accounts or keys | create a share and a user, then remove them | create an export, then remove it | tells the sender to run a fixed command |
| One-time? | no | **yes** (nonce, expiry) | no, unless the daemon is stopped after (TESTED: password worked twice) | no (keys/accounts) | no | no | no |
| Folders, modes, links | needs a tar or zip layer | tar stream, modes kept, links kept (TESTED) | yes, built in (TESTED) | yes (`get -r`, `-p`) | yes (it is a file system) | yes | whole datasets and snapshots only |
| Resume | `Range` | `Range` with a **new** ticket (TESTED) | `--partial --append-verify` (TESTED) | `reget` (TESTED) | per copy tool | per copy tool | `zfs send -t <token>` (SOURCE) |
| New code we write | a server | a small ticket check in the helper (the toy is 172 lines of Go) | a supervisor that starts and stops one daemon per transfer | the same, plus account handling | much config, plus mount handling | much config, plus mount handling | a fixed action with a dataset list |
| Tested here | no (the toy without tickets would be the same) | **yes** | **yes** (daemon on loopback, `rsync 3.2.7`) | **partly** (a bare `sftp-server` driven by `sftp -D`; no `sshd`) | no | no | no |

Notes, each with its label:

- **Plain HTTP** is the same as the pick without the ticket. Without tickets the node would serve a folder to anyone who asks, and the hub would no longer decide per transfer, which breaks "the hub authorizes". Under the owner's "security is perfect" assumption the only loss is that rule. BELIEVED; the toy's ticket is the only difference.
- **HTTP with tickets (TESTED, section 2.3).** The node helper already listens on one port (`8480` proposed) and has the hub's public key. The ticket is the helper API's own signing idea applied to a download URL. Cost on every node: an extra route in the helper and the list of folders it may share. It adds nothing to install, so it is the same on every distro. The toy needed only Go's standard library.
- **rsync (TESTED, results F3, `t/trs.sh`).** A one-shot `rsync --daemon` on a private port with one read-only module, one user and a password file: wrong password refused (`@ERROR: auth failed`); a copy of a folder kept modes and links; `-L` (follow links) on a link pointing outside the module failed with `Invalid cross-device link`; the same password worked a **second** time (nothing makes it one-time); an interrupted 512 MB copy continued with `--partial --append-verify` and was identical; after the daemon was stopped, connections were refused. Two details I hit: the daemon runs as `nobody` by default and could not read my files until I set `uid = root` (it needs the right user on a real node), and one daemon per transfer means the helper must start, watch and stop a process and pick a free port. `rsync` over `ssh` needs `sshd` on the sender with an account and key for this purpose; I did not start `sshd` (it needs a privilege-separation folder `/run/sshd` which does not exist here and I did not create). rsync is not guaranteed on every node image (BELIEVED).
- **SFTP (TESTED partly, results F3).** `sftp -D <path to sftp-server>` ran a local server process: `ls -l`, `get`, `get -p` (kept the executable bit) and `reget` of a partial file worked and the `reget` result was identical; `put` ran without an error (I did not compare its result). This shows the protocol and its tool do the job; it does **not** test `sshd`, accounts or one-time access over a network. One-time access would need a temporary key or account per transfer (BELIEVED).
- **SMB and NFS (not tested).** These are mounts, not copies: the receiver mounts the sender's folder, copies, unmounts. Needs a server on the sender and mount privileges (root, or a mount helper) on the receiver, plus user-id mapping, ports and stale-mount handling when a node restarts (BELIEVED, from general knowledge). Neither gives a one-time ticket; the hub would have to create and remove an export or share around each transfer. They are the heaviest to set up on mixed distros and they are the only ones where a node going down in the middle can leave a hung mount on the other node (BELIEVED). SMB and the NAS check ports are already in the health table of `HUB-OS.md` (the NAS "share or SSH answers"), so a share on the NAS may exist for other reasons; that is a different use.
- **`zfs send` (not tested; no ZFS here).** SOURCE: OpenZFS manual pages `zfs-send(8)` and `zfs-recv(8)` (fetched from `openzfs.github.io/openzfs-docs/man/master/8/` and read 2026-10-05): `zfs send` writes a stream of a snapshot; `-R` sends a whole tree of snapshots and properties; `-i`/`-I` send increments; `-w` sends encrypted data raw; `zfs send -t <receive_resume_token>` resumes an interrupted receive that was started with `zfs receive -s`, and `-s` needs the pool feature `extensible_dataset`. It needs ZFS on both ends, so it applies only between the NAS and the backup NAS (and any node that gets ZFS), the unit is a **snapshot of a dataset**, not a file, and it needs permission to run `zfs` (root or delegated with `zfs allow`: BELIEVED). A sender-side pipe to `zfs receive` on the other node needs a channel (ssh or a raw socket; BELIEVED). It is the right tool for the existing plan to keep the backup NAS current and to move an AI dataset between ZFS machines, and it belongs to its own fixed action in the helper (with dataset names from the node's config, never from the request), not to the file clipboard. Proposal only.

**Pick:** HTTP with one-time tickets for the file clipboard (files and folders); `zfs send` as a separate fixed action for dataset moves between ZFS nodes. rsync, SFTP, SMB and NFS are not needed for this, and the first two stay as tools the owner can use by hand.

---

## 5. Part 4: what each node GUI must implement, and the conformance test

### 5.1 The adapter (proposal)

Nodes run different distros and different GUI programs; they cannot share code. So the contract is a **command**: an executable named `hubos-files` on the GUI user's `PATH`, started by the node helper, speaking JSON on standard input and standard output. Any language works (shell, Go, Python, Rust). It runs as the GUI's user and reads the GUI's state however that GUI allows (its own socket, a file, D-Bus); how is not our concern.

| Verb | Required? | Input | Output on standard output (one JSON line) |
|---|---|---|---|
| `capabilities` | yes | none | `{"api":1,"verbs":["selection","destination","destination-at","refresh"]}` (list only what is implemented; `selection` and `destination` must be there) |
| `selection` | yes | none | `{"items":[{"path":"/abs/a.txt","name":"a.txt","kind":"file","bytes":123}, ...]}`; the files and folders **highlighted** in the GUI's focused file view; `"items":[]` if none |
| `destination` | yes | none | `{"dir":"/abs/folder"}` for the folder the focused file view shows, or `{"dir":""}` if no folder is open |
| `destination-at X Y` | optional | X and Y: a point in the node's screen in pixels | `{"dir":"/abs/folder"}` of the folder under that point, else the same as `destination` |
| `refresh` | optional | `{"paths":["/abs/new1","..."]}` on standard input | `{"ok":true}`; the GUI shows the new files (may do nothing if it already watches the folder) |

Rules every verb must obey:
1. Exit 0 with exactly one JSON document (and a line break) on standard output and nothing else there; log text goes to standard error. A missing or empty list is `[]`, never `null` or absent.
2. An unknown verb: exit 2 and `{"code":"unknown_verb","error":"..."}`. Another failure: exit 1 and `{"code":"...","error":"..."}`. "Nothing selected" and "no folder open" are **answers**, not failures.
3. Paths are absolute, clean (no `..`, no `//`), valid UTF-8, existing; `name` is the last part of `path`; `kind` is `file` or `dir`; `bytes` is the real size of a file, or the sum of the sizes inside a folder.
4. **Links are not offered.** (The helper refuses them anyway.)
5. `selection`, `destination` and `destination-at` change nothing (no files, no GUI state), give the same answer twice in a row, and answer within **2 seconds**. Five at once must all be answered.
6. **Test mode.** If the environment variable `HUBOS_FILES_FIXTURE=<folder>` is set, the adapter answers from that folder and not from the GUI: highlighted items are the entries of `<folder>/selection/`; the open folder is `<folder>/destination`, or none if the file `<folder>/no-destination` exists; for `destination-at`, `<folder>/destination-at.json` is a list of `{"x0","y0","x1","y1","dir"}` rectangles. It is a few lines of code and lets the conformance test run without a GUI.

Which meaning `selection` should have (highlighted, or "copied with the GUI's own Copy command") is question 4; I wrote "highlighted" because it needs no hidden state in the GUI.

### 5.2 The conformance test

`conform ADAPTER [ARGS...]` (`ft/conform/main.go`, 306 lines of Go, standard library only, builds to one static 3.7 MB program that runs on any Linux distro, glibc or musl: `file` says "statically linked"). It builds a fixture folder with awkward names and runs the adapter many times. Checks:

| Check | What it verifies |
|---|---|
| C1, C1b | `capabilities`: exit 0, one JSON document, `api` is 1, lists `selection` and `destination` |
| C2 | empty selection gives exactly `"items": []` |
| C3a-d | selection of six items (names with spaces, `ünï 日本`, a leading dash, a line break, an empty file, a folder with files in it, plus a link to `/etc/hostname`): exit 0; every path absolute, clean, valid UTF-8, existing; `name` equals the last part; kind and size match the real file; every regular file and folder is listed; the link is **not** |
| C4 | standard output is one JSON line and nothing else |
| C5 | answered within 2 seconds |
| C6, C7 | the same answer twice; nothing in the fixture changed (names, sizes, times) |
| C8, C9 | `destination` is an absolute existing folder; nothing changed |
| C10 | with no folder open: exit 0 and `{"dir":""}` |
| C11 | `destination-at`, if advertised: inside a rectangle gives that folder, outside gives a non-empty fallback |
| C12 | `refresh`, if advertised: exit 0 and `{"ok":true}` |
| C13 | unknown verb: exit 2 and a JSON error with a `code` |
| C14 | five parallel `selection` calls all answered |

**Results (TESTED, `t/tconf.sh`, results F4).** The reference adapter (`ft/adapter/main.go`, in fixture mode) passes **18 of 18**. The same adapter with one deliberate fault each (set by the variable `BAD`) fails exactly the checks that should catch it:

| Fault | Failed checks |
|---|---|
| paths relative | C3b |
| a `..` inside a path | C3b |
| lists a file that does not exist | C2, C3b |
| wrong byte count | C3b |
| invalid UTF-8 in a name | C3b, C3c |
| prints a debug line before the JSON | C1, C1b, C2, C3a, C3c, C4, C8, C10, C14 |
| `"items": null` for an empty selection | C2 |
| takes 5 seconds | C1, C1b, C2, C3a, C3c, C4, C5, C8, C10, C13, C14 |
| offers a link as a file | C3b, C3d |
| writes a file while answering | C7, C9 |
| `destination` returns a file | C8 |
| "no folder" answered as an error | C10 |
| `destination-at` ignores the point | C11 |
| exits 3 after printing valid JSON | C1, C1b, C2, C3a, C8, C10, C14 |

All 14 are caught. This proves the **format and safety rules** of the adapter. It does **not** prove that a real GUI reports what its user highlighted; a fixture mode that always answers from a folder passes without a GUI.

### 5.3 The manual acceptance list for a real GUI (not automated; proposal)

Run the conformance test on the node first, then check by hand:
1. Highlight three files in the GUI; `hubos-files selection` names exactly those three within 2 seconds; clear the highlight, it returns `[]`.
2. Highlight a folder; it is listed with `"kind":"dir"`.
3. Open a folder view; `destination` names it. Close all views; it returns `{"dir":""}`.
4. With two views open, the answer is for the one the owner last used.
5. Files named with a line break, a leading dash, emoji, or 255 bytes of name come back byte for byte.
6. A file that was deleted after highlighting is not listed, and nothing crashes.
7. A file the GUI user cannot read is still listed; the transfer later fails with a clear message on the hub (check the message).
8. After a paste, the new files appear in the GUI within 2 seconds (by `refresh` or by the GUI's own watching).
9. Kill and restart the GUI: the adapter answers with empty answers, not with an error.
10. Run `conform hubos-files` on the node's own distro and attach the output.

---

## 6. What I ran (all TESTED unless marked)

- **The nested hub compositor:** driftwm at the pinned commit, release build (7 min 11 s), nested on **headless sway 1.9** (software rendering), because the usual virtual X server does not start here (`/usr/bin/xkbcomp` missing). Same setup as `docs/proposals/driftwm-layouts.md` section 2.1.
- **Test clients (Python, pywayland 0.4.18, installed with `pip install --target` into the work folder):** `dndclient.py` (a window that logs every pointer and drag event; can start a drag or accept a drop), `overlay.py` (a layer-shell overlay), `vptr.py` (injects the pointer into sway with `zwlr_virtual_pointer_v1`). The bindings for the two wlroots protocols were generated from small XML files I wrote from memory of those protocols (`wlr-*.xml`); since the programs worked as the logs show, the opcodes were right, but those files are **my reconstruction, not the official ones**.
- **Transfer prototypes (Go, standard library only):** `ft/main.go` (ticket server and signer), `ft/adapter/main.go`, `ft/conform/main.go`. Built with `go build` into the work folder.
- **Tools unpacked, not installed:** `rsync 3.2.7`, `openssh-client 9.6p1` (for `sftp`) and `openssh-sftp-server` (Ubuntu 24.04 packages, `dpkg -x`); `wl-clipboard`, `sway` with their libraries.
- **Run everything:** `tools/image/experiments/layouts-and-file-transfer/README.md`; one full run's output is `results.txt` (sections L = layouts, D = drag, F = file transfer).
- Helpers or summarising tools: none. All sources were read by me.

## 7. What I could not test or did not look at

- NFS, SMB (Samba or ksmbd), `zfs send` and `zfs receive`, and `sshd` (no ZFS here; no `/run/sshd`; I did not set up servers I could not hold to account).
- Any real viewer (remote-viewer, TigerVNC, a wlroots VNC client, Moonlight): what they do with the pointer outside their window; whether a VNC viewer drops a file (VNC has no such feature: BELIEVED).
- Drag from node to node with real node GUIs; the node GUIs do not exist yet.
- Anything with more than one node over a real network; TLS; timing; the helper as a service.
- The ticket server's memory of used tickets across a restart, and parallel downloads.
- Keyboard input and drawing in the overlay; zoom below 1; the bar; more than one screen.
- The flaky resume (section 2.3).
- Whether `hubd` can reliably find the machine of the focused window (it is a design in `node-helper-api.md`, not built).
- I did not edit `node-helper-api.md` or `HUB-OS.md`.

## 8. Questions for the owner (batched)

1. **Order of work:** file clipboard first, drag gesture later (my proposal), or not at all?
2. **Drag gesture, if wanted:** is the key-armed overlay (press a key chord, then drag from one window to another) acceptable, or does it have to be a plain drag with no extra key? A plain drag needs either a driftwm patch (a carried change to the compositor) or a node-to-hub signal that the helper API does not have today.
3. **Ticket signing:** reuse the helper API's signature (the management key, `signify`-compatible) for the file tickets, as I propose, or a separate key? And may the node helper open a second route (`/t/<ticket>`) on the same port?
4. **What "copy" means:** the files **highlighted** in the GUI when you press the hub's key (my proposal), or the GUI's own copy buffer after you press its Copy command?
5. **Trigger:** one bar button for copy and one for paste, a key chord for each (for example Super+Shift+C and Super+Shift+V, to be checked against driftwm's bindings), or a "send to ..." pick list? The same question as for the text push.
6. **Who owns the files on the receiving node:** the GUI's user (the helper must then act as that user) or the helper's user? Also: may the helper run as the GUI user from the start?
7. **Links inside a folder:** keep them as links, skip them, or refuse the whole folder? (The toy keeps them; the receiver must then never follow them.)
8. **Overwrite policy:** `name (2)` (my proposal), ask, or overwrite?
9. **Move and delete:** copy only, or also "move" (delete the source after a verified copy)? I propose copy only for v1.
10. **Size and number limits** for one paste (for example 100 items, no byte limit).
11. **Dataset moves:** is a fixed `zfs send` action between the NAS and the backup NAS wanted, and who chooses which datasets (a list in each node's config, never in the request)?
12. **Conformance:** is the test and the manual list above enough to accept a node GUI, or do you want it run on real hardware each release?
13. **Help for the viewer decision:** if the hub viewer is one we write (an open choice in `HUB-OS.md`), it could add a real drop target (it would see a drop on its own window). Do you want that considered in the viewer decision?

## 9. Table of items

| # | Item | Label |
|---|---|---|
| 1 | `wl-copy --type text/uri-list` also offers plain-text types, so file addresses would reach the hub's text clipboard | TESTED (F1) |
| 2 | Real file managers offer text types for a file copy | BELIEVED |
| 3 | wayvnc forwards `text/plain;charset=utf-8` to the viewer | SOURCE (repo, not re-read) |
| 4 | Wayland drag and drop between two clients works under driftwm | TESTED (D2) |
| 5 | Implicit grab: while a button is down only the pressed window gets events, with coordinates outside it | TESTED (D1) |
| 6 | driftwm's socket has no pointer or button information | SOURCE + TESTED |
| 7 | Overlay shown during the drag learns only the release point | TESTED (D3) |
| 8 | Overlay shown before the press sees press, all motion and release; windows below get nothing | TESTED (D4) |
| 9 | Window under a point from the `state` data | TESTED at camera 0,0 zoom 1; BELIEVED otherwise |
| 10 | Patching driftwm is small | UNKNOWN (not written) |
| 11 | One-time signed tickets: valid once; reuse, expiry, tamper, wrong sender, outside folder, outside link all refused | TESTED (F2) |
| 12 | Folder as tar stream keeps modes and links; links must not be followed on extraction | TESTED (F2) |
| 13 | 1 GiB: sender memory about 9.5 MB | TESTED (F2); speed meaningless |
| 14 | Resume needs a new ticket; once a resumed file ended short | TESTED (F2); cause UNKNOWN |
| 15 | rsync one-shot daemon: auth, folders, modes, links, `-L` refused, password reusable, resume | TESTED (F3) |
| 16 | `sftp-server` via `sftp -D`: list, get, get -p, put, reget | TESTED (F3); `sshd` not tested |
| 17 | SMB, NFS properties | BELIEVED |
| 18 | `zfs send` options, resume token, `-s` needs `extensible_dataset` | SOURCE (man pages); not run |
| 19 | End to end with two fake nodes: names, folder, 20 MB file, second paste does not overwrite, old ticket refused | TESTED (F5) |
| 20 | Adapter conformance test: good adapter 18/18; 14 faulty adapters all caught | TESTED (F4) |
| 21 | A real GUI satisfies the adapter contract | UNKNOWN (no GUI) |
| 22 | The new API calls (`/v1/files/...`) and the hub-side triggers | PROPOSAL, not built |

## 10. Sources read (2026-10-05)

- This repository: `HUB-OS.md`, `CLAUDE.md`, `docs/proposals/node-helper-api.md` (whole), `docs/driftwm-findings.md`, `docs/proposals/phase-b-desktop.md`, `docs/proposals/hub-user-split.md` (file table), `internal/driftwm/driftwm.go`.
- driftwm at `352333a8fa1b22171492d4b71a54102045c9a19d`: `src/handlers/mod.rs` (lines 130-200), `src/ipc/protocol.rs` (`Request`, `Event`), `docs/ipc.md`.
- `wayland.xml` of libwayland 1.22.0 (`libwayland-dev 1.22.0-2.1build1`, unpacked).
- OpenZFS manual pages `zfs-send(8)` and `zfs-recv(8)`, https://openzfs.github.io/openzfs-docs/man/master/8/ (downloaded with `curl`, HTML converted to text, read).
- Manual behaviour of `rsync 3.2.7`, `sftp` and `sftp-server` from running them.
- Not used: web search, summarising tools, other helpers' reports.
