# PROPOSAL: the node helper API and the clipboard bridge

Requests are not signed or encrypted (owner decision, 2026-10-06); only the image signature is checked.

**Status: PROPOSAL.** Nothing here is decided and nothing here is in the images. `HUB-OS.md` wins if this file disagrees with it. The only code is a small experiment in `tools/image/experiments/clipboard-bridge/` (loopback only, two headless compositors, no signing, not the production helper). Written 2026-10-04.

**Labels used on every item:**
- **TESTED**: I ran it in this build environment; the exact command and output are shown (or are in `tools/image/experiments/clipboard-bridge/results.txt`).
- **SOURCE**: read in a primary source (a file, a manual page or a repository); the link or path and the date I read it (2026-10-04) are given. Items marked "(repo)" were written by earlier sessions of this project and were not run again by me.
- **BELIEVED**: I think it is true but did not test it and have no source.
- **UNKNOWN**: nobody has checked; do not rely on it.

---

## Plain-words summary

1. **What this is.** Every node will run a small program, the "node helper". The hub's `hubd` is its only client. This document is the smallest list of questions `hubd` may ask it and the things it may tell it to do: say how you are (`live`, `status`), start or stop the screen session, restart the sound sender, and move a piece of text into or out of the node's clipboard. It is a short list of fixed actions. The helper cannot be told to run a command.
2. **One API with the recovery agent.** The recovery agent (the small program in the recovery kernel) has an API of plain HTTP and JSON requests (SOURCE: the prototype, read 2026-10-06). The helper uses the same style. They are one API: both answer `GET /v1/status` with a field `state` that is `"recovery"` or `"running"`, and with the machine id. A machine is only ever in one of the two states, so both programs can use the same port, and `hubd` needs one client. Nothing in a request is signed or encrypted. The only signature that matters is the image signature, which the recovery agent's install request still checks (through `hubos-ctl update`).
3. **Clipboard, the short version.** Text only, regular copy-and-paste only. Node to hub already works through the display program (wayvnc), not through the helper. Hub to node goes through the helper: when the owner presses a button or a key chord, `hubd` reads the hub's clipboard with `wl-paste`, and sends it to the helper, which puts it into the node's clipboard with `wl-copy`. I recommend that the push happens **only on that explicit action**, not automatically on every copy. Reasons: no copy loops are possible by construction, and a password you copy on the hub does not silently land on twenty machines.
4. **Copy loops are real, and I tested them.** With two headless compositors I made the loop happen on purpose: with automatic push and no rule, the text bounced hub, node, hub, node about seven times a second until my own request limit stopped it after 20 rounds. Two cheap rules each stop it after one round: (a) the node does nothing if the new text equals what its clipboard already holds; (b) the hub remembers a fingerprint (hash) of the last text it sent or got and ignores a change that equals it. A third rule, a limit of 20 sets per 10 seconds, bounds anything the first two miss. A "source marker" inside the clipboard itself is **not** possible with `wl-copy` (it can only offer text types).
5. **Limits I tested.** Text up to 1 MiB (1,048,576 bytes) went through the API in about 0.03 s; one byte more is refused with HTTP 413. `wl-copy` and `wl-paste` alone moved 100 MiB in under a second in the headless test, so the limit is a choice, not a technical wall. The display program wayvnc has a hard wall of its own: neatvnc 1.0.3 closes the connection of a viewer that sends more than 10,000,000 bytes of clipboard text (SOURCE).
6. **What `wl-copy` leaves behind.** `wl-copy` starts a background process that holds the text until something else is copied. TESTED: if the helper is killed (`kill -9`) the text stays pasteable; if the node's compositor restarts the text is gone. After 100 pushes only one holder is alive; the old ones exit and wait to be collected by the init program (here: PID 1 of the container did collect them, but slowly; on Hub OS the init must do it, BELIEVED for s6).
7. **Sound.** The node helper does not control volume. The hub does, with `wpctl` on the hub's own PipeWire stream for that node (round 1 of the remote-display study tested this). The helper only reports whether the node's sound sender is running and can restart it, because the sender has a known start-up race.
8. **Things I could not test:** wayvnc itself (the real path from the node's clipboard to the hub's viewer), driftwm as the hub compositor, the real viewer, any real node image or network. These are marked UNKNOWN in the table in section 10.
9. **A problem to read before you decide:** the only hub viewer that is left after your decision "no X11 stack on the hub" is `remote-viewer`. Round 2 of the remote-display study found it garbles non-ASCII text coming from the node (even with wayvnc 0.10.2). My push would make the node's clipboard change, wayvnc would send that text to the viewer, and the viewer would then **overwrite the hub's own clipboard with the garbled copy** (BELIEVED from putting those two findings together; not tested because wayvnc and the viewer were not run). Question 3 below.
10. **Questions for the owner** are in section 12. The main ones: whether the calls need any limit on how often they are made (there are no nonces now); button or key chord (or both) and what the target is; the size limit; the port.

---

## 1. What the owner already decided (this design follows it)

All SOURCE: the task text of 2026-10-04 and `HUB-OS.md`; not discussed again.

- Display on the nodes: **wayvnc v0.10.2 with neatvnc v1.0.3**, built from source at pinned tags, started with `--name <machine id>`, non-shared sessions, screen size fixed per node.
- Node to hub clipboard works through the display protocol. Hub to node clipboard goes through a **clipboard bridge**, through the node helper's API.
- The hub does not run an X11 compatibility stack for a viewer.
- Display credentials are per node.
- Requests are not signed or encrypted (owner decision, 2026-10-06: no management key, no challenge, no TLS). The image signature (the update key) is separate and unchanged (`docs/proposals/secrets.md`).
- Sound is PipeWire RTP with per-node mute and volume on the hub; `hubd` stores the values.
- The recovery agent listens on the normal network; its address comes from a DHCP reservation.
- No systemd. `hubd` is Go. No new streaming protocol; no web dashboard as the control plane.

---

## 2. One API with the recovery agent

**SOURCE (repo, read 2026-10-06):** the recovery agent prototype on the branch `recovery-agent-no-signing` (PR #69): `tools/image/experiments/recoveryagent/agent.go`, `main.go`, `backend_hubos.go`, `agent_test.go`; `docs/proposals/recovery-and-out-of-band.md`; `docs/proposals/remote-display.md` 4.3. `HUB-OS.md` says the same: one port (8480) and one API for the node helper and the recovery agent, with a state field, and `GET /v1/live` as the check that the helper answers.

**Proposal: one API, two programs, one port.** Every call is a plain request. No call needs a header, a key or a number from an earlier call.

| | Recovery agent (in the recovery kernel) | Node helper (in the normal system) |
|---|---|---|
| `state` | `"recovery"` | `"running"` |
| Calls | `GET /v1/status`, `GET /v1/logs`, `POST /v1/clear-failures`, `POST /v1/install`; `GET /v1/live` is design only (not in the prototype yet) | `GET /v1/live`, `GET /v1/status`, `POST /v1/session/start`, `POST /v1/session/stop`, `GET /v1/audio`, `POST /v1/audio/restart`, `GET /v1/clipboard`, `PUT /v1/clipboard` |
| Calls it does not have | all the helper's | all the recovery agent's. The design answers `404` with code `not_in_this_state`; the prototype answers `404` `{"error":"no such endpoint"}` for any path it does not know (SOURCE, `agent.go`) |
| Size | must stay tiny (the TEST recovery kernel with the agent is 8,893,440 bytes against 6,333,440 without, `docs/image.md` note on T18, SOURCE repo) | can be bigger |

Why one port: a machine is either in recovery or running, never both (BELIEVED: nothing runs the helper in the recovery kernel; the helper is in the slot's root). `hubd` asks `GET /v1/status` at the machine's inventory address and reads `state`. The recovery agent's `status` answer today (SOURCE, `agent.go`) has: `api` (1), `min_hub` (1), `machine`, `state`, `recovery_release`, `boot_failures`, `failure_limit`. `api` and `min_hub` follow the add-only versioning rule in 3.2. `machine` is `NAME=` in the node config, else the host name; **UNKNOWN:** whether that string is the same as the inventory id (nothing makes them equal today).

**What the recovery agent's install request does (SOURCE, `agent.go` and `backend_hubos.go`).** `POST /v1/install` takes `{"Slot":"a|b","BaseURL":"http..."}` (at most 4096 bytes; a slot other than a or b, a URL that does not start with `http`, or a body that is not that JSON gets `400`). It runs `hubos-ctl update BASE SLOT`, one install at a time. `hubos-ctl` is what refuses a bundle that is not correctly signed: the manifest signature against the update keyring, the floor rule, the hashes. The agent adds no check of its own. An install that `hubos-ctl` refuses is answered `500` with the program's output (which contains its `REFUSED` line). The tests of these refusals are listed in `recovery-and-out-of-band.md` 2.3.

**Port:** the experiment uses `8480` as its default (SOURCE, `recoveryagent/main.go`), and the QEMU test in `tools/image` reaches the agent on 8480; no port is chosen anywhere else in the project (`recovery-and-out-of-band.md` 5.3). I suggest `8480` for both, to be confirmed (question 2).

**What I did not do:** change the recovery agent or its code.

## 3. Rules for every call

### 3.1 Transport

- HTTP/1.1 and JSON (`Content-Type: application/json`, UTF-8). `hubd` is the only client. No web page, no streaming. It is a control API of a few fixed calls, not a dashboard and not a streaming protocol.
- **No TLS and no signing** (owner decision, 2026-10-06). Requests and answers cross the network in plain text and nobody checks who sent them. This matters more here than in recovery: **clipboard text can be a password**, anyone on the wire can read it, and anyone who can reach the port can read (`GET /v1/clipboard`) or set the node's clipboard. The owner's rule is that the network is assumed perfect and security is not a design concern; I record the fact and add nothing.
- The helper answers only when asked; it never calls the hub.

### 3.2 Versioning (reused from `remote-display.md` 4.3, a rule I proposed there; nothing implements it yet)

- The path starts with `/v1`. Inside one major version a change may only **add** fields or calls. Nobody may remove or rename one, or change its meaning.
- A reader **ignores fields it does not know** (TESTED in the experiment: a `PUT` with an extra field `"future":1` is accepted, scenario A5 in `results.txt`).
- `GET /v1/status` carries `api` (the major version this program speaks) and `min_hub` (the lowest hub major version it works with). A hub that does not know a node's major version shows "needs newer hub". A node can serve `/v1` and `/v2` at the same time during an upgrade, so hub and nodes can be updated in any order.

### 3.3 Errors

Every non-200 answer has this body (the recovery agent prototype sends `error` only; `code` is an add-only addition in the design):

```json
{"code": "too_large", "error": "request body too large"}
```

| HTTP | `code` | When |
|---|---|---|
| 400 | `bad_request` | The body is not the JSON the call needs. |
| 404 | `not_found` | No such path. |
| 404 | `not_in_this_state` | A call that exists only in the other state (recovery vs running). |
| 405 | `method_not_allowed` | Wrong method. |
| 409 | `busy` | A start, stop or restart is already running, or a long job is running and `force` was not given. |
| 413 | `too_large` | Body or text over the limit (section 3.4). |
| 422 | `bad_text` | Text is empty, not valid UTF-8, or has a NUL byte. |
| 429 | `rate_limited` | Too many calls (section 3.4). A `Retry-After` header should be added (not in the experiment). |
| 500 | `internal` | The helper failed. |
| 503 | `clipboard_unavailable` | No compositor, or no data-control protocol on it. |
| 503 | `no_compositor` | The node's compositor is not running (session start refused). |

**TESTED in the experiment** (scenario A5, exact output in `results.txt`): 400 for `not json`; 422 for `{"text":""}`, a NUL (`\u0000`) and a raw invalid UTF-8 byte; 405 for `DELETE`; 404 for another path; 413 for a body or text over the limit; 429 after the rate limit. The others are design only. **SOURCE (prototype, read 2026-10-06):** the recovery agent answers 400 (bad install request), 404 (unknown path, no `code`), 500 (install or clear-failures failed, with the program's output); it has no 405, 413 or 429 and no `code` field. Adding them is an open design step, not decided.

### 3.4 Size limits and rate limits

| Item | Limit | Label |
|---|---|---|
| Body of any call except the clipboard | 4096 bytes | SOURCE (repo): the recovery agent's `maxBody` |
| Clipboard text | **1 MiB = 1,048,576 bytes of UTF-8** (bytes, not letters), proposed | TESTED that the limit is enforced at both sides (1,048,576 passes, 1,048,577 gets 413) |
| Clipboard request body | text limit x 6 + 4096 bytes, because JSON may write one control byte as six (`\u0001`) | TESTED: 174,762 control characters, a body of 1,048,583 bytes, accepted |
| Install requests in the recovery agent | one at a time (a second one is refused while one runs) | SOURCE (prototype `backend_hubos.go`, `TryLock`); the answer code for the refusal is not written down; not tested here |
| Calls per sender address (all calls) | **no limit is designed.** The old design limited open calls to 20 per second per sender and bounded the changing calls with the nonce rules; without nonces nothing does the second job. | UNKNOWN: **question 1** |
| Clipboard sets | 20 per 10 seconds, then 429 | TESTED in the experiment (the number is a guess) |
| Session start / stop / audio restart | 1 per 10 seconds, proposed | not tested |

Why 1 MiB: an ordinary paste of text (code, a log excerpt) is far smaller. The two programs on the path accept far more: `wl-copy` plus `wl-paste` moved 100 MiB in 0.82 s on a headless compositor (TESTED, scenario B2), and the wall in the display program is 10,000,000 bytes (SOURCE below), so the limit is the owner's choice. Question 4.

### 3.5 Logging

Proposed (not built): the helper writes **one line per request** to its standard error, which the init's logger collects (s6-log, BELIEVED) with time, sender address, method and path, the HTTP result and the time taken. For the clipboard it logs **the number of bytes and the first 8 hex digits of the hash, never the text**. It never logs a password or a certificate key. Each session start and stop logs the `reason` text the hub gave. `hubd` should log its side the same way. A refused request is logged too: that is how a person sees something odd. (The recovery agent prototype keeps its own log in `/run/recovery-agent.log`; `GET /v1/logs` returns the last 4096 bytes of it; SOURCE `backend_hubos.go`.)

---

## 4. The calls

All bodies are JSON. Every call is plain: no header is needed. Examples are what the call would return; apart from the clipboard (TESTED shapes, section 8.7) nothing was run.

### 4.1 `GET /v1/live` -- liveness

A tiny answer for the check loop: "the helper process is alive and answering". It does **not** say the display works.

```json
{"live": true, "state": "running"}
```

Why separate from `status`: `hubd` polls often; `status` costs more and may ask other programs. Why not just the display port: "ready" in `HUB-OS.md` means the session server accepts connections (the TCP check `hubd` does today). The helper being alive and the display being ready are two different facts; `status` reports the second as seen from inside the node.

### 4.2 `GET /v1/status`

Running:

```json
{
  "api": 1, "min_hub": 1,
  "machine": "ai-1",
  "state": "running",
  "image_release": "hub-2026.10.04-1",
  "slot": "a",
  "boot_id": "6f1c9a0e-...",
  "uptime_s": 86400,
  "session": {"state": "running", "since_s": 3600},
  "display": {"server": "wayvnc", "ready": true, "viewers": 1},
  "gui": {"state": "running"},
  "audio": {"sender": "running", "destination": "10.0.0.2:46003"},
  "busy": {"long_job": false, "reason": ""},
  "update_pending": false,
  "clipboard": {"supported": true, "max_text": 1048576}
}
```

Recovery (what the agent answers today, SOURCE `agent.go`, read 2026-10-06):

```json
{"api": 1, "min_hub": 1, "machine": "ai-1", "state": "recovery",
 "recovery_release": "recovery-1", "boot_failures": 3, "failure_limit": 3}
```

`session.state` is one of `stopped`, `starting`, `running`, `stopping`, `failed`. `viewers` comes from wayvnc's control socket (`wayvncctl`, BELIEVED: not run). `busy.long_job` is how a machine "refuses to update or restart while a game or long job is running" (`HUB-OS.md`); the helper learns about it from a small file or call that the job program sets (not designed). `update_pending` and the image fields come from `hubos-ctl status` (SOURCE, repo: the command exists; its output format was not checked for this document). For the recovery agent the liveness call is `GET /v1/live` (4.1); the prototype does not have it yet.

### 4.3 `POST /v1/session/start` and `POST /v1/session/stop`

Request (all fields optional):

```json
{"reason": "owner pressed Restart session"}
```

Answer `202` (the work continues; the hub polls `status`):

```json
{"session": "starting"}
```

Already in that state: `200 {"session":"running"}` (so a retry is harmless). While another start or stop runs: `409 busy`. Stop with a long job running: `409 busy` unless `{"force": true}`. Mapping to wayvnc and the GUI is in section 6.

### 4.4 `GET /v1/audio` and `POST /v1/audio/restart`

```json
{"sender": "running", "destination": "10.0.0.2:46003", "stream": "rtp-ai-1"}
```

`restart` answers `202 {"sender":"starting"}`. There is **no volume or mute call** on the node. Section 7.

### 4.5 `GET /v1/clipboard` -- pull

```json
{"present": true, "text": "from the node", "bytes": 13, "sha256": "6553973e...19a92"}
```

Nothing copied, or the copied thing is not text: `{"present": false, "text": ""}`. Text over the limit: `413 too_large`. (TESTED, scenarios A3, B.)

### 4.6 `PUT /v1/clipboard` -- push

```json
{"text": "from the hub: café ✓ 日本", "origin": "hub"}
```

Answer:

```json
{"result": "set", "bytes": 30, "sha256": "..."}
```

`result` is `set` or `unchanged` (the node already held exactly that text, so nothing was done). `origin` is only for the log. `sha256` is the SHA-256 of the text after turning CRLF into LF; the hub can remember it instead of the text. Errors: 413, 422, 429, 503 as in 3.3. (TESTED, scenarios A, A2, A4, A5.)

### 4.7 Calls that are not here

Restart and shut down (`HUB-OS.md` "Power actions", after the first slice): `POST /v1/power/restart` and `/shutdown`, a `reason`, the same `busy` rule. Not designed further. Anything else is not in this API.

---

## 5. What the helper may and may not do

**May:**
- Read facts it can get without asking the hub for anything: its own state, `hubos-ctl status`, the init's view of three named services, the wayvnc control socket.
- Start and stop **three named services** from a fixed list in its own per-machine config (section 6), through the init.
- Restart the sound sender (one of the three).
- Read and set the node compositor's **regular text selection**, with `wl-paste` and `wl-copy` (TESTED, section 8).
- Later: restart or shut down the machine by calling the one existing tool with fixed arguments.

**May not:**
- Run a command, or any part of a command line, named by a request. The only request data that reaches another program is (a) the clipboard text, which goes to `wl-copy` **standard input** and never onto a command line (so a text like `-n` or `--help` is just text; TESTED that stdin works, the program does `wl-copy --type text/plain;charset=utf-8` with the text on stdin), and (b) enum values and numbers that the helper checks itself.
- Read or write any file a request names. Open a shell or a terminal. Change its keyring, config, certificates or passwords. Install or update anything. Touch the **primary** selection, non-text clipboard content, or the keyboard or mouse.
- Answer a call it does not know. Log secrets (3.5). Call the hub or another node.

The recovery agent has the same shape (a fixed list: status, install, clear failures, logs), so the same review applies to both.

**Not solved:** the node helper itself is a program with the ability to stop the node's screen and read its clipboard, and any machine that can reach its port may ask for that (requests are not signed; owner decision, 2026-10-06). A hacked hub or any other machine on the network can do everything in this list to every node; that is true of the recovery agent too, except that its install request refuses any bundle the owner's update key did not sign.

---

## 6. How session start and stop map to wayvnc and the GUI

**Proposal (nothing built).** A "session" has three parts, in a fixed table in the node's own config (not in the request):

| Order on start | Part | What it is | Example of what the init starts |
|---|---|---|---|
| 1 | display server | wayvnc on the node's compositor | `wayvnc -n <machine id> -R -C /etc/hubos/wayvnc.conf <address>` |
| 2 | sound sender | PipeWire RTP sender for this node | the node's PipeWire config with an RTP sink |
| 3 | the GUI | the node's bespoke program, shown in the window | the program for this role |

Stop is the reverse order. The node's **compositor** and the init itself are always running (the machine "stays logged in", `HUB-OS.md`); the helper does not start the compositor. If it is not running, `start` answers `503 no_compositor`.

- **`--name <machine id>`**: SOURCE: `wayvnc --help` text in `src/main.c` of v0.10.2, option `-n, --name <name>`, "Set the desktop name", default "WayVNC" (read 2026-10-04). The viewer window title depends on it (round 2, SOURCE repo).
- **Screen size fixed:** two parts: the compositor's output has a fixed size in its config, and wayvnc gets `-R, --disable-resizing` ("Disable automatic resizing", same file) so a viewer cannot change it. Whether `-R` is enough was not tested by me (round 2 saw resizing work with 0.10.2 when it was *not* disabled).
- **Non-shared sessions:** in neatvnc the viewer sends a "shared" flag when it connects; the code at `src/server.c:684-707` handles it (SOURCE, v1.0.3). Round 2 (repo) saw that a second non-shared viewer kicks the first. The helper needs no option for this; whether the *server* can insist on it was not checked.
- **Do not use `-d, --disable-input`:** in wayvnc's source history it also turns the clipboard off (round 2, SOURCE repo, commit `5ed57b9`).
- **Credentials per node:** wayvnc's own config file (`-C`) names the user, password and certificate files; they live in the node's per-machine config area, never in git and never in the request (round 2 R2.3, repo).
- **The init:** `HUB-OS.md` keeps the init swappable (s6 or dinit). `hubos-ctl` has no service commands today (SOURCE: `image/rootfs/usr/sbin/hubos-ctl`, usage line "status|ensure-entries|update|confirm|rollback|clear-failures"). So the helper needs a thin, init-neutral command for "up NAME" and "down NAME". **UNKNOWN:** it does not exist.
- **Retry for sound:** round 1 (TESTED, repo) found the RTP sender often fails at start-up with "no target node available" and needs restarting until its links exist. The helper's `session/start` therefore waits for the sound links (bounded retries, then `audio.sender = "failed"` and the session still counts as running). The bound is not chosen.

What `start` returns before the parts are up: `202`. The hub polls `status`; `session.state` becomes `running` when all three are up.

---

## 7. Audio control: who does what

**Hub side does the control; the node helper only reports and restarts.** Reason: `HUB-OS.md` puts each node's mute and volume on the hub, and round 1 found the stream to control is the hub's **own** `rtp-source` stream (SOURCE, repo, `remote-display.md` 3.3 and 3.6, TESTED there with PipeWire 1.0.5 and `wpctl`; not run again here):

| Step | Who | How | Label |
|---|---|---|---|
| Send the node's sound | node | PipeWire `rtp-sink` on the node's default sink, to the hub's address and a port | TESTED (repo, round 1) |
| Receive it as a **named** stream | hub | one `rtp-source` per node in the hub's PipeWire **config** (a drop-in file), `node.name` fixed per machine (round 1 used `rtp-from-a-at-hub`), a fixed port per node from the inventory. `pw-cli load-module` did nothing in round 1, so this is config, not a runtime command | TESTED (repo) |
| Find the stream | hub (`hubd`) | read `pw-dump`, take the object whose `node.name` is that name (round 1 `findnode.py`) | TESTED (repo) |
| Set volume and mute | hub (`hubd`) | `wpctl set-volume <id> <0.0-1.0>`, `wpctl set-mute <id> 0\|1`, `wpctl get-volume <id>`; master volume with `@DEFAULT_AUDIO_SINK@` | TESTED (repo): `wpctl set-volume 35 0.5` gave peak 875 of 7000; mute gave 0 |
| Remember the values | hub (`hubd`) | `{volume, muted}` per machine id plus a master entry, applied at `hubd` start and whenever the stream appears again; not WirePlumber's own memory | design (repo, round 1); where the file lives is open |
| Restart a failed sender | node helper | `POST /v1/audio/restart` | design |
| Say whether the sender runs | node helper | `GET /v1/audio` and `status.audio` | design |

Two warnings carried over from round 1: `wpctl` volume is a **cubic scale** (0.25 gave a peak of 141 of 9000, TESTED, repo), so the slider must say which scale it uses; and there is a short gap after a stream appears in which it plays at the wrong volume (BELIEVED; size unknown).

A node never needs its own volume control, except to be silent at the source, which is not planned. If one is ever wanted, `PUT /v1/audio` with `{"mute": true}` can be added without breaking anything (add-only).

---

## 8. The clipboard bridge

### 8.1 The path

```
node -> hub:  node app --copy--> node compositor --data-control--> wayvnc --RFB--> viewer on the hub --> hub clipboard
hub -> node:  hub clipboard --wl-paste--> hubd --HTTP PUT--> node helper --wl-copy--> node compositor
```

Node to hub is the display protocol's job (decided). Hub to node is the bridge. The two directions are separate and use different mechanisms, which is what makes loops possible (8.6).

**What wayvnc does with the node's clipboard (SOURCE, read 2026-10-04):** [`src/data-control.c` of wayvnc v0.10.2](https://raw.githubusercontent.com/any1/wayvnc/v0.10.2/src/data-control.c), read from a download of the file:
- It watches the node compositor's selection with `wlr-data-control` (or `ext-data-control`) and asks for the type `text/plain;charset=utf-8` (`self->mime_type`). It reads the whole text into memory with **no size limit in this file**, then sends it to the viewer with `nvnc_send_cut_text`.
- Text arriving **from** a viewer is ignored if empty ("Ignoring empty clipboard from VNC client"), and otherwise sets **both** the regular and the primary selection of the node.
- It offers a marker type `x-wayvnc-client-<random 8 hex>` on the selections **it** sets and ignores a selection that carries its own marker. That stops wayvnc echoing what **wayvnc** put there; it does not stop an echo of what **our helper** put there, because our `wl-copy` has no such marker.

neatvnc v1.0.3 (SOURCE: clone of [any1/neatvnc](https://github.com/any1/neatvnc) at tag v1.0.3, commit `783437a`, read 2026-10-04): `include/server.h:41` `#define MAX_CUT_TEXT_SIZE 10000000`; `src/server.c:1597-1604` and `1673-1678`: a client message over that size makes the server **close that client's connection**. Incoming CRLF is turned into LF (`crlf_to_lf`, `src/server.c:1471`, used on extended-clipboard text at line 1560).

### 8.2 How the hub reads its clipboard

- **Program:** `wl-paste -n --type text` (one shot). `-n` stops it adding a newline; `--type text` makes it refuse images and files. TESTED (the Go test and scenario C).
- **Which protocol:** `wl-paste --watch` "requires a compositor that supports the wlroots data-control protocol" (SOURCE: wl-clipboard 2.2.1 manual page `wl-paste(1)`, read in the unpacked package). The one-shot form works without it but pops up a tiny window as a hack (same manual, BUGS), which on a tiling compositor can flash or hang. TESTED with `strings`: the 2.2.1 `wl-paste` binary contains `zwlr_data_control_manager_v1` and **no** `ext_data_control` string.
- **driftwm advertises** `zwlr_data_control_manager_v1` v2 and `ext_data_control_manager_v1` v1 (SOURCE (repo): `docs/driftwm-findings.md` lines 288-300, listed by `wayland-info`; round 2 saw it again). So wl-clipboard 2.2.1 should work on the hub through the wlr one. **UNKNOWN:** I did not run `wl-paste` under driftwm, and did not build it.
- **No `--watch` in production** (recommended): `hubd` calls `wl-paste` only when the owner triggers a push. `--watch` was used in the experiment only to measure the loops.
- `hubd` runs as the normal user in the seat group (`HUB-OS.md`), so it can reach driftwm's Wayland socket with `WAYLAND_DISPLAY` and `XDG_RUNTIME_DIR` set (BELIEVED; `hubd` already talks to driftwm's own socket).

### 8.3 What triggers a hub-to-node push (the owner chooses)

| Option | How it would work | What I know | Label |
|---|---|---|---|
| **A. Panel button** | A Waybar custom module (or a line in the wofi list) runs `hubd clipboard push` | Clicking the bar does not move keyboard focus: with a terminal below the bar, `dw msg state` still marked the terminal as focused after the click (repo, `docs/bar-findings.md` line 227, TESTED there). So "push to the focused node window" is possible from a bar button. | TESTED (focus, repo); the command does not exist |
| **B. Key chord** | A driftwm binding: `"mod+v" = "spawn hubd clipboard push"`. `spawn <cmd>` exists and "runs a command without loading cursor" (SOURCE: [driftwm `config.reference.toml`](https://raw.githubusercontent.com/malbiruk/driftwm/352333a8fa1b22171492d4b71a54102045c9a19d/config.reference.toml) at the pinned commit `352333a8`, lines 397-401 and 480). `mod+v` is **not** used by the default bindings (I read all default `mod+` bindings in the same file). | A chord is not seen by the node window as long as driftwm handles it; a window rule with `pass_keys = true` would forward it to the node instead (`docs/driftwm-findings.md` section 6, repo). Node windows are desktop-style, not `pass_keys = true`, so this should work. | SOURCE; BELIEVED that it works with the viewer's keyboard grab (not run) |
| **C. Both** | Same command, two ways to start it | Cheapest to add once one exists | |
| **D. Automatic on every change** | `wl-paste --watch` on the hub | See 8.6. Not recommended. | TESTED loops |

**The target:** the machine whose window has focus. `hubd` already keeps a record of which driftwm window belongs to which machine (`HUB-OS.md`, `docs/hubd-slice2.md`); the focused window comes from driftwm's socket. **BELIEVED**, not built. A bar button could instead open the machine list ("push to: AI box, NAS, ..."), which needs no focus rule but one more click. Pushing to a node whose window is **not** focused works with this design (the helper needs no focus), which the TigerVNC-style viewer path could not do (round 2: it needs focus, repo). TESTED in the sense that headless `wl-copy` needs no focused window.

`hubd clipboard push` would: read the hub clipboard, refuse empty/non-text/too big with a plain message on the bar, `PUT` it, and show "sent 28 bytes to ai-1" or the error. **Not built.**

### 8.4 How the node helper sets its clipboard

`wl-copy --type "text/plain;charset=utf-8"` with the text on **standard input**, regular selection only. TESTED (Go test `TestRealWlClipboard`, scenarios A and F).

- **Force the type.** Without `--type`, `wl-copy` looks at the content to guess a type, and it calls `xdg-mime` and `file` for that. TESTED (scenario G (4)): with a clean `PATH` (neither program found) it still offered `text/plain`, `text/plain;charset=utf-8`, ...; with the programs present, text that looks like HTML was offered first as `text/html` and a shell script as `application/x-shellscript`. wayvnc asks for `text/plain;charset=utf-8`, and `wl-copy` offered that in every case I tried, but a forced type keeps it from depending on two programs a node image may not have. TESTED: forced type offers `text/plain;charset=utf-8, text/plain, TEXT, STRING, UTF8_STRING` (scenario F).
- **No `-p`.** The primary selection ("selected text") is never touched. TESTED: a `wl-paste --watch` on the regular selection saw nothing when only the primary changed, and the other way round (scenarios C, C2). Changing the primary on every selection would also be noise for the viewer.
- **Do nothing if the text is already there.** `SetIfChanged` reads the current text and returns `unchanged` if equal. TESTED (scenario G (1)): without it, setting identical text again **wakes every watcher** (`wl-paste --watch` ran its command twice for two identical `wl-copy one`); with it, the second push changed nothing (A2: `{"result":"unchanged"}`).
- **wayvnc will see the new text** (it watches the same selection) and send it to the viewer on the hub. That is the echo path (8.6). It cannot be switched off from the helper.

### 8.5 Size, text only, empty text, non-ASCII

| Case | Behaviour | Label |
|---|---|---|
| Limit | 1 MiB text; 413 over it on push and on pull; the hub refuses before sending | TESTED (B) |
| Text only | `wl-paste --type text` refuses other content: with an image copied it exits 1 with `Clipboard content is not available as requested type "text"`; a `--watch` with `--type text` ran no command for an image copy (scenario G (3)). The API takes only a JSON string. | TESTED |
| Empty text | Refused on push (`422 bad_text`); the hub-side trigger with an empty clipboard does nothing and says so. `wl-copy` with empty input **does** make an empty selection and `wl-copy --clear` makes none. `wl-paste` tells them apart (exit 0 with nothing for empty text; exit 1 with "Nothing is copied" for none), and so does a `--watch` command, through the variable `CLIPBOARD_STATE` (`data` with 0 bytes for empty text, `nil` for none). wayvnc itself ignores empty text from a client (SOURCE above). So clearing the node's clipboard from the hub is not offered. | TESTED (A4, G); SOURCE |
| Non-ASCII | `café ✓ 日本` round-trips through `wl-copy`/`wl-paste` unchanged and counts as 16 bytes (the limit is in bytes). The API refuses invalid UTF-8 (`encoding/json` alone would turn a bad byte into U+FFFD silently; the program checks the raw body first) and NUL. | TESTED |
| Non-ASCII over the **display** protocol | wayvnc 0.10.2 + neatvnc 1.0.3 works both ways with the TigerVNC viewer; with `remote-viewer` it arrived garbled | SOURCE (repo, round 2 rows 11-12; not run again) |
| Line endings | The guard treats CRLF and LF as the same text (hash after CRLF to LF). neatvnc turns incoming CRLF into LF (SOURCE above); whether a viewer adds CRLF on the way to the hub: UNKNOWN | TESTED (D5 with a stand-in that adds CRLF); UNKNOWN for real viewers |
| Primary vs regular | Regular only (8.4) | TESTED |
| Sensitive text | The `CLIPBOARD_STATE=sensitive` hint exists in wl-paste's manual, but "currently wl-clipboard only ever sets `data` or `nil`" (SOURCE: `wl-paste(1)`). Password managers mark secrets with a special type (BELIEVED to be `x-kde-passwordManagerHint`; from memory, not checked); `hubd` could refuse to push when `wl-paste --list-types` shows it. Not tested. | SOURCE; BELIEVED |

### 8.6 Loops (clipboard echo) and what stops them

**How a loop happens.** The helper's `wl-copy` changes the node's clipboard. wayvnc sees that as a change of the node's clipboard and sends it to the hub's viewer. The viewer sets the hub's clipboard. If the hub is watching its clipboard and pushes every change, that push is a new `wl-copy` on the node, and it all starts again. (BELIEVED to be the real path; wayvnc and the viewer were not run. The experiment's `relay` command replaces wayvnc plus viewer.)

**Experiment (TESTED, scenario D, `results.txt`).** `hub-watch -auto` pushes every hub change to the node API; `relay` copies every node change back to the hub; the text "ping" is copied once on the hub. Results over 3 seconds:

| Rules switched on | hub events | copies by the relay | Result |
|---|---|---|---|
| none (`-no-guard`, node `-always-set`) | 22 | 20 | **Loop.** About 7 rounds a second, stopped only by the node API's 20-per-10-seconds limit (`pushes answered 429: 1`) |
| hub remembers the hash of the last text (node `-always-set`) | 3 | 1 | Stops after one round: `event 3: 4 bytes ignored: same as last sent or received` |
| node does nothing for equal text (no hub rule) | 3 | 1 | Stops after one round: the second push answered `"result":"unchanged"`, so the node's clipboard did not change again |
| both | 3 | 1 | Stops after one round |
| both, with a stand-in that changes LF to CRLF on the way (D5) | 3 | 1 | The hub rule hashes the text with CRLF turned into LF, so `two\nlines` (9 bytes) and `two\r\nlines` (10 bytes) count as equal and the loop stops. Afterwards the hub clipboard really holds the CRLF text (one CR counted). |

The same four rules as a Go test with no compositor (`TestEchoLoopAndTheRulesThatStopIt`, `go test`; the output is in the final report of this task): 50 node sets (the cap) with no rule, 1 with either rule.

**Rules, in the order I would use them:**
1. **No automatic push.** Push only on the owner's action (8.3). Then there is no loop to stop, because nothing reacts to the echo. (Reasoned; this is also what the experiment's `hub-watch` without `-auto` does: it logs the echo and pushes nothing.)
2. **Node: do nothing if the text is unchanged** (8.4). Stateless; survives a helper restart. Costs one `wl-paste` per push.
3. **Hub: remember the hash of the last text sent to a node or received from it, and ignore an equal change** (`Guard`). The hash is taken after CRLF to LF. It is needed only if automatic push is ever added.
4. **A rate limit** (20 sets per 10 s) as the last bound. TESTED that it stops a loop that the other rules do not (first row of the table).

**A source marker inside the clipboard is not possible with `wl-copy`**: it offers one text type and the usual aliases, nothing else (scenario F). wayvnc uses a marker (above) because it speaks the data-control protocol itself. For us to add one, the helper would need to speak that protocol directly (a Wayland client written in Go with the standard library only, or a C helper), which is far more code than `wl-copy`. **Not built, not needed** with rules 1 to 4. The API does carry `origin` for the log.

**Limit of the rules:** a hash rule fails if the text is changed on the way (for example by Latin-1 conversion in a viewer: round 2 saw `café` become `cafÃ©` with `remote-viewer`). Then the "echo" is a different text and the hub rule does not recognise it. Rules 1 and 4 still hold.

### 8.7 What the node sends back

- To a `PUT`: only `{"result": "set"|"unchanged", "bytes": N, "sha256": "..."}`.
- To a `GET /v1/clipboard` (a pull): the text, its length and hash. The hub copies it into its own clipboard with `wl-copy`. This is a fallback for when the display path did not deliver (for example the hub's viewer window is not focused or the viewer does not send it); the owner triggers it like a push. TESTED (scenario A3, `hub-pull`).
- Nothing else. The helper does **not** push the node's clipboard to the hub by itself: that is wayvnc's job (decided), and a second path would add another loop.
- The **echo of our own push** also arrives at the hub through the display path (8.6). Expect the hub clipboard to be set again to the same text right after a push; with `remote-viewer` it may be set to a **garbled** version if the text has non-ASCII letters (SOURCE (repo): round 2 row 11, for node to hub; BELIEVED here for the echo; question 3).

### 8.8 What `wl-copy` leaves running; helper restarts

`wl-copy` forks a background process that serves the selection until another program takes it over (SOURCE: `wl-copy(1)`: "By default, `wl-copy` forks and serves data requests in the background"; `-o` serves one paste and exits; `-f` stays in the foreground).

Scenario E (TESTED, `results.txt`):

```
before: wl-copy holders 0, zombies 4
100 sets done. wl-copy holders alive: 1, zombies: 93
 4819     1 SN   wl-copy --type text/plain;charset=utf-8
E2: kill -9 the helper: node paste after helper was killed: t100        (the holder, parent PID 1, kept serving)
E3: new helper starts: same text -> "unchanged"; new text -> "set"; wl-copy processes now: 1
E4: wl-copy -o: alive before paste 1, alive after first paste 0, second paste: Nothing is copied
E5: the node compositor killed: "wl_display_dispatch: Broken pipe"; wl-copy processes alive: 0
```

What it means:
- **One holder at a time.** Each new `wl-copy` replaces the previous selection; the old holder exits. After 100 pushes one is alive.
- **A dead holder is a zombie until its parent collects it.** The old holders' parent is gone (they were orphaned when `wl-copy`'s first process exited), so they are children of PID 1. In this container PID 1 is the environment's `process_api`, and the zombie count rose to 93 and fell later (`ps` listing showed none a few minutes after). **On Hub OS, the init must collect orphans.** BELIEVED that s6's PID 1 does (s6-svscan, from memory), **UNKNOWN** for dinit. This is a requirement to test in the image, not a finding about `wl-copy`. The helper's own `Wait` on the first `wl-copy` process is needed so that one does not become a zombie of the helper (the experiment does it with `cmd.Run`, and its stdout/stderr must not be Go pipes or `Wait` never returns; both noted in the code).
- **Helper restart:** the text survives (TESTED, E2). The new helper must not assume the clipboard is empty; it reads it (`unchanged` rule). If the init kills the helper's whole process group or control group, the holder dies with it, and the node's clipboard becomes empty: **UNKNOWN** how s6 or dinit stop a service (BELIEVED that s6 signals only the service's own process; not tested). Starting the helper's service so that it does not take the holder down is an image detail.
- **Compositor restart:** the text is gone and the holder exits (TESTED, E5). The helper cannot restore it; the owner pushes again.
- **`-f`** (hold the process ourselves and kill it on the next set) is an alternative: the helper would own the process and could clean up exactly; I did not test it, because the default has been enough.

---

## 9. What I ran

- **Environment:** the same cloud container as earlier studies: no GPU, no real network, loopback only, no packages installed. Programs came from `apt-get download` (Ubuntu 24.04 packages `sway 1.9-1build2`, `libwlroots12t64 0.17.1-2.1build1`, `wl-clipboard 2.2.1-1build1`, `wayland-utils`, and their libraries, listed in the README of the experiment), unpacked with `dpkg -x` into `/tmp/nh-root`, run from there with `PATH` and `LD_LIBRARY_PATH`, and deleted afterwards (also the working directories under `/tmp/nh-*` and the worktree).
- **Two compositors:** two `sway` 1.9 with `WLR_BACKENDS=headless WLR_RENDERER=pixman`, each in its own `XDG_RUNTIME_DIR` (`/tmp/nh-hub`, `/tmp/nh-node`), standing in for the hub and a node. `wayland-info` on both (TESTED): `wl_data_device_manager` v3, `zwlr_data_control_manager_v1` v2, `zwp_primary_selection_device_manager_v1` v1; **no** `ext_data_control`.
- **The program** is in `tools/image/experiments/clipboard-bridge/` (see its README): Go, standard library only, calling `wl-copy` and `wl-paste`. Its unit tests need no compositor; one test uses a real compositor when asked. Commands and exact outputs of `gofmt`, `go vet` and `go test` are in the final report of this task and are repeated in the pull request.
- **`scenarios.sh`** printed every command with its output into `results.txt` (194 lines, one full run). Scenarios: A push/pull/plain cases/API errors/rate limit; B sizes; C primary vs regular; D loops; E leftovers and restarts; F types.

Things the experiment does **not** do: `wayvnc`; a viewer; `driftwm`; a real network; more than one node; a long run. It has no request checking of any kind (it listens on loopback).

---

## 10. TESTED, SOURCE, BELIEVED, UNKNOWN: the table

| # | Item | Label | Where |
|---|---|---|---|
| 1 | `wl-copy` sets, `wl-paste` reads text between two headless compositors, loopback | TESTED | `results.txt` A, Go test `TestRealWlClipboard` |
| 2 | UTF-8 `café ✓ 日本` round-trips through `wl-copy`/`wl-paste` and the API | TESTED | A, Go test |
| 3 | Empty text and cleared clipboard are different for `wl-paste` and for a `--watch` command (`CLIPBOARD_STATE` `data` with 0 bytes versus `nil`); empty push refused (422) | TESTED | A4, G |
| 4 | Primary and regular selections are independent; the bridge uses regular only | TESTED | C, C2 |
| 5 | Size limit enforced both ways (1,048,576 ok, 1,048,577 gets 413); 100 MiB through `wl-copy`/`wl-paste` in 0.82 s | TESTED | B, B2 |
| 6 | `wl-paste --watch` runs once at start and once per change, **also when the same text is set again** | TESTED | G (1), D |
| 7 | Echo loop with no rule; stopped by each of: node "unchanged" rule, hub hash rule, rate limit | TESTED (with a stand-in for wayvnc + viewer) | D |
| 8 | CRLF/LF difference does not defeat the hash rule | TESTED (stand-in) | D5, Go test |
| 9 | `wl-copy` holder survives a helper `kill -9`; dies with the compositor; one alive at a time | TESTED | E |
| 10 | Forced `--type` keeps the offered types fixed; guessing depends on `xdg-mime` and `file` | TESTED | F, G (4) |
| 11 | JSON error codes and statuses for the clipboard calls | TESTED | A5, Go tests |
| 12 | wayvnc ignores empty client text; sets both selections; has its own marker; reads node text with no size limit in `data-control.c` | SOURCE | wayvnc v0.10.2 `src/data-control.c` |
| 13 | neatvnc closes a client over 10,000,000 bytes of cut text | SOURCE | neatvnc v1.0.3 `include/server.h`, `src/server.c` |
| 14 | wayvnc `-n/--name`, `-R/--disable-resizing`, `-C` exist in v0.10.2 | SOURCE | `src/main.c` option table |
| 15 | driftwm `spawn <cmd>` action; `mod+v` free in default bindings | SOURCE | `config.reference.toml` at `352333a8` |
| 16 | driftwm advertises `zwlr_data_control_manager_v1` v2 and ext v1 | SOURCE (repo, TESTED there) | `docs/driftwm-findings.md` |
| 17 | wl-clipboard 2.2.1 `--watch` needs wlr data-control; fallback pops up a window; `CLIPBOARD_STATE` only `data`/`nil` | SOURCE | `wl-paste(1)`, `wl-clipboard(1)` |
| 18 | Clicking the Waybar module does not take keyboard focus | SOURCE (repo, TESTED there) | `docs/bar-findings.md` |
| 19 | PipeWire RTP named stream, `wpctl` volume and mute, cubic scale | SOURCE (repo, TESTED there) | `remote-display.md` 3.2, 3.3 |
| 20 | The recovery agent's install request refuses a bundle that is not correctly signed (no signature, another key, changed manifest, below the floor); status, logs and clear-failures need no header | SOURCE (repo; the refusals TESTED there, see `recovery-and-out-of-band.md` 2.3) | `recovery-and-out-of-band.md` 2.3 |
| 21 | `remote-viewer` garbles non-ASCII from the node; never sends hub text | SOURCE (repo, TESTED there) | `remote-display.md` R2.1 rows 5, 11 |
| 22 | The real echo path (helper `wl-copy` -> wayvnc -> viewer -> hub clipboard) behaves like the stand-in | **UNKNOWN** | needs wayvnc and the viewer |
| 23 | The echo overwrites the hub's clipboard with a garbled copy when the viewer is `remote-viewer` and the text is non-ASCII | BELIEVED | follows from 21 and 22 |
| 24 | `wl-paste` (one-shot and `--watch`) under driftwm | **UNKNOWN** | not built |
| 25 | A driftwm chord with `spawn` works while a `remote-viewer` window has focus | BELIEVED | |
| 26 | `hubd clipboard push`, the focus-to-machine lookup, the bar button | **UNKNOWN** | not built |
| 27 | `PUT`/`GET` for the helper and the full round trip hub to node | **UNKNOWN** | not built |
| 28 | A node helper binary; running as an s6/dinit service; stopping a service does not kill the `wl-copy` holder | **UNKNOWN** | |
| 29 | The init collects the orphaned `wl-copy` holders (no zombies pile up) | BELIEVED for s6, UNKNOWN for dinit | here PID 1 collected them slowly |
| 30 | A thin init-neutral "service up/down" command | **UNKNOWN** | does not exist |
| 31 | Session start/stop order, sound retry, `-R` keeps the size fixed | **UNKNOWN** | design only |
| 32 | `wayvncctl` gives the number of viewers | BELIEVED | not run |
| 33 | Password-manager hint type is `x-kde-passwordManagerHint` | BELIEVED (from memory) | |
| 34 | Compositors that offer `ext_data_control` but not `zwlr_data_control` (wl-clipboard 2.2.1 would fall back to the window hack) | UNKNOWN | the node compositor is not chosen; sway 1.9 offers the wlr one |
| 35 | Behaviour with many nodes at once, a long run, a real network | **UNKNOWN** | |

---

## 11. Sources read (2026-10-04)

Read by me on the primary file:
- `HUB-OS.md`, `CLAUDE.md`, `docs/proposals/recovery-and-out-of-band.md`, `docs/proposals/remote-display.md` (rounds 1 and 2), `docs/driftwm-findings.md` (clipboard and keybinding parts), `docs/bar-findings.md` (the focus line), `tools/image/experiments/recoveryagent/agent.go`, `backend_hubos.go`, `main.go` (this repository; the three agent files read on the branch `recovery-agent-no-signing`, 2026-10-06).
- [wayvnc `src/data-control.c` and `src/main.c`, tag v0.10.2](https://raw.githubusercontent.com/any1/wayvnc/v0.10.2/src/data-control.c) (downloaded with `curl`, whole files).
- [neatvnc tag v1.0.3](https://github.com/any1/neatvnc) (a shallow `git clone`; `include/server.h`, `src/server.c`).
- [driftwm `config.reference.toml`](https://raw.githubusercontent.com/malbiruk/driftwm/352333a8fa1b22171492d4b71a54102045c9a19d/config.reference.toml) at the pinned commit.
- Manual pages `wl-copy(1)`, `wl-paste(1)`, `wl-clipboard(1)` from the unpacked `wl-clipboard 2.2.1-1build1` package.

Not read from a primary source (nothing in this document relies on them): web search results, summarising fetch tools. None were used for this document.

---

## 12. Questions for the owner

1. **Limits on how often calls may be made.** Without nonces nothing bounds the number of calls a sender can make (the old text limited open calls to 20 per second per sender and bounded the rest through the nonce rules). Do you want a limit per sender for all calls (for example the 20 per second I guessed for the old open calls), only the limits already in 3.4 (clipboard sets, session start/stop), or none? I did not design one. Also: what should the recovery agent answer when a second install request arrives while one runs (the prototype refuses it; the code is not chosen), and should the recovery agent also get `GET /v1/live`?
2. **Port.** One port for the recovery agent and the helper. Is `8480` fine, or do you want another? Should it go into `viewers.toml` `[default_ports]`?
3. **The viewer and the echo.** After your decision "no X11 stack on the hub", `remote-viewer` is the viewer for nodes (it cannot send hub text to a node; round 2). It garbled non-ASCII from the node in round 2. If that also happens to the echo of a push, the hub's clipboard is changed to a garbled copy right after every push of non-ASCII text. Do you accept that, or should I look for a different Wayland-native viewer, or have `hubd` put the original text back after a push (a hack; not tested)?
4. **Size limit.** 1 MiB of text per push, or smaller (for example 256 KiB), or larger (up to just below 10,000,000)?
5. **Trigger.** Button on the bar, key chord, or both? If a chord: is `Super+V` acceptable? What should the target be: the machine whose window is focused, or a pick from the list?
6. **Automatic push.** Do you ever want copy on the hub to arrive on a node without pressing anything? I recommend no (passwords would land on every node). If yes, only to the focused window's machine?
7. **Pull.** Do you want the pull (node to hub, on request) as a fallback button, or only the display path?
8. **Service control.** May I add a thin init-neutral "service up/down" step to the image (outside this task) so the helper never needs to know whether s6 or dinit runs?
9. **`HUB-OS.md`.** Per your instruction I did not edit it. If you accept parts of this, its Unverified list could gain: "wayvnc/viewer clipboard echo path", "wl-paste under driftwm", "node helper start order and sound retry", "orphaned wl-copy holders collected by the init". May I draft it?

---

## History

An earlier version of this document (up to 2026-10-05) had every changing call signed by the hub with a management key and a one-time number fetched from `GET /v1/challenge`, and asked whether to add TLS. The owner decided on 2026-10-06 that requests are not signed and not encrypted and that only the image signature stays. That text was removed; it can be read in the git history of this file before the commit that made this change. The recovery agent prototype was changed to match (PR #69).
