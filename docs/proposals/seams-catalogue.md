# PROPOSAL: the seams catalogue (every boundary between two Hub OS parts, and the rules each must follow)

**Status: PROPOSAL. Nothing here is decided, built or tested** unless a line says SOURCE (read in a repo file, named) or TESTED. `HUB-OS.md` wins if this file disagrees with it. Written 2026-10-10 by the lead agent (no helpers). No code. **Names of sockets, paths, schema ids, message fields and limits are placeholders unless a SOURCE is named.** No protocol is invented: where a seam has no definition in the repo, the entry says "not recorded" and lists what is missing.

## 1. Purpose
A **seam** is a boundary where two parts talk. Writing each one down once (who, how, how fast, what happens when the other side is gone) lets the parts be built and tested alone. **A seam is PUBLIC only if the owner might swap one side independently of the other; otherwise it is PRIVATE** (both sides ship together and are version-locked).

## 2. Rules every seam must satisfy
1. Failure on one side never crashes the other side.
2. Every request/response seam has a **deadline**; every one-way seam has a **drop policy** (what is dropped when the reader is slow or absent). No unbounded queue anywhere.
3. PRIVATE seams are **version-locked**: both sides carry the same build id and refuse to talk when it differs.
4. PUBLIC seams carry a **schema id in the first message** and an unknown version is rejected cleanly (a named error, no crash, no guessing). Where the other side's protocol already has its own version step (RFB, HTTP path `/v1`, PipeWire), that step is the schema id and no second one is added.
5. Every seam defines behaviour when the other side is **absent** (not started, crashed, hung).
6. Logs are **rate-limited per seam** (placeholder: at most one line per distinct message per 10 s, with a count).
Where the repo already says something about a rule, it is quoted in the seam; otherwise it is a proposal.

## 3. Summary table
| # | Seam | Class | Transport | Style | State in the repo |
|---|---|---|---|---|---|
| 1 | hubd feed to the panel | PUBLIC | Unix stream socket | line JSON, a full state line each time | exists |
| 2 | worker to presenter | PRIVATE | memfd + descriptor over a Unix socket | frame handoff | not recorded |
| 3 | worker to node | PUBLIC | TCP, RFB | Raw rectangles | direction recorded, nothing built |
| 4 | conductor to worker | PRIVATE | Unix SOCK_SEQPACKET (not Erlang distribution) | request/response + events | not recorded |
| 5 | conductor to the init | PRIVATE | the init's own control tool | command | init not decided |
| 6 | watcher to compositor and hubd | PRIVATE | existing sockets | probes | partly recorded, not built |
| 7 | worker to clipboard history | PRIVATE | Unix stream socket | one-way push | proposal in `clipboard.md` |
| 8a | producers to the toast daemon (hub side) | PRIVATE | Unix stream socket | line JSON | proposal in `notifications.md` |
| 8b | nodes to the hub for notifications | PUBLIC | HTTP long poll; `hubos-notify` on the node | JSON | proposal in `notifications.md` |
| 9 | mixer to PipeWire | PUBLIC | PipeWire's own interface | PipeWire's | not recorded |
| 10 | workshop to node helper | PUBLIC | HTTP | JSON | partly (update call missing) |

## 4. The seams

### 1. hubd feed to the panel (PUBLIC: a bespoke panel may replace Waybar later: SOURCE `HUB-OS.md`, Panel)
- **Producer / consumer:** `hubd` (server) to the panel module, today Waybar's `custom/hub` running `hubd feed --socket /run/hubos/hubd.sock` (SOURCE `docs/proposals/clipboard.md` table, `docs/hubd-slice2.md`).
- **Transport:** Unix stream socket, one JSON object per line.
- **Message style (disagreement):** the owner's list says "snapshot plus updates". **Today every line is a complete state** `{"text","class","tooltip"}`; there are no separate update messages. The feed writes at most one line per second and never an identical line (SOURCE `clipboard.md` 55-56, `hubd-slice2.md`). Keep it; "snapshot plus updates" would be a change (question 3).
- **Version policy:** none today. Proposal: the first line after connect carries `{"schema":"hub-feed-v1"}` (placeholder id) so a panel can refuse an unknown version.
- **Deadline / drop policy:** one-way; if the reader is slow, `hubd` drops older lines and keeps the newest (BELIEVED: at most one line per second makes this easy).
- **Failure behaviour:** `hubd` gone: the panel keeps the last state and marks it **STALE** (SOURCE `HUB-OS.md`, `hubd-slice2.md`). Panel gone: `hubd` ignores the closed connection. The panel can reconnect without `hubd` restarting (assumed yes by the owner; TESTED? not recorded: `hubd feed` as a client reconnecting is not in the tests I read: UNKNOWN).

### 2. worker to presenter (PRIVATE)
- **Parts:** one worker per machine (connection, frame in shared memory, sound, clipboard, volume) to the presenter that draws frames and sends input back (SOURCE `HUB-OS.md`, The session layer; `docs/STATE.md` 3.2).
- **Transport (the owner's text):** frame buffers in shared memory made with `memfd`; the descriptor goes over a Unix socket. **Not recorded in the repo:** the buffer state machine, sealing, the socket type, the number of buffers, the message layout. The design review that covered these was given in chat and is not a repo document (`hub-os-md-audit.md` item 4 in the docs PR lists this). Facts about the system calls are in `docs/research/verified-facts-2026-10-10.md` (memfd seals: SOURCE; Go/Erlang descriptor passing: UNKNOWN/BELIEVED).
- **Style:** small control messages ("frame ready", "frame released", input events); frames are not copied through the socket.
- **Version policy:** version-locked build id in the first message.
- **Deadline / drop:** the worker never waits for the presenter: if no free buffer is returned in time the worker overwrites its newest pending frame (drop policy: newest wins). Input events from the presenter: bounded queue, drop oldest key-repeat first (placeholder).
- **Absent:** presenter gone: the worker keeps its node connection and its last frame, and waits for a presenter; worker gone: the presenter keeps the last frame, marked stale (SOURCE `HUB-OS.md`: "An unreachable machine keeps its last frame, marked stale").
- **Not catalogued:** `HUB-OS.md` says "a local interface lets other displays attach". That would be a different, possibly PUBLIC seam; it is not defined anywhere (not recorded).

### 3. worker to node (PUBLIC: any RFB server could be on the node)
- **Protocol:** RFB over TCP, Raw rectangles, security type None, RFB Extended Clipboard (UTF-8), no ContinuousUpdates, no Fence, key events by keysym, reconnect with backoff (SOURCE `HUB-OS.md`, The session layer; RFC 6143, see `verified-facts-2026-10-10.md`).
- **Version policy:** RFB's own ProtocolVersion step is the schema id (rule 4). An unknown server version: refuse by RFC 6143 section 7.1.1.
- **Deadlines:** connect, handshake and update-request deadlines are placeholders (5 s, 5 s, 5 s); the incremental flag semantics are in the verified facts note (a server must not send unsolicited updates).
- **Absent:** server gone: the worker reconnects with backoff (placeholder 1 s doubling to 30 s) and the window keeps the last frame, marked stale.
- **Disagreement:** `docs/proposals/remote-display.md` summary item 7 still says Sunshine/Moonlight stay for the gaming box; `HUB-OS.md` (later) says they are not planned.

### 4. conductor to worker (PRIVATE)
- **Parts:** the Elixir/OTP conductor supervises workers (restart limits, escalation to a bar alert, stale state: SOURCE `HUB-OS.md`).
- **Transport (the owner's text):** Unix **SOCK_SEQPACKET** sockets, **not** Erlang distribution. Whether Erlang's `socket` module can do seqpacket and pass descriptors is UNKNOWN until tested (`verified-facts-2026-10-10.md`); a tiny test is planned in the docs PR (Task 4).
- **Style:** commands (start, stop, status, ping, shutdown) with a request id and a reply; events (state changes) from the worker.
- **Version policy:** build id (rule 3).
- **Deadlines:** every command has a deadline (placeholder 2 s; ping 1 s); a worker that misses 3 pings is hung (placeholder numbers).
- **Absent:** conductor gone: workers keep running with their last state and wait for a new conductor (they never exit because it left); worker gone: the conductor restarts it within its restart limit, then raises an alert (SOURCE `HUB-OS.md`). What stops a restarted worker from obeying an old conductor (fencing) was discussed in chat: not recorded.

### 5. conductor to the init (PRIVATE)
- **What it does:** start/stop/restart/inspect hub services (for example restart a worker's service, or the compositor).
- **Transport:** the init's own control tool. **BELIEVED, not checked against a decision:** s6's `s6-svc` and `s6-svstat` (the repo's confirm script already runs `s6-svc -u` and `-O`: SOURCE `image/rootfs/etc/s6/sv/confirm/run`; the s6 control options were read on skarnet.org on 2026-10-06 in my own earlier notes; there is no repo file for it). **The init is not decided** (the init comparison is running; `init-comparison.md`), and the owner wants an init-neutral "service up/down" command (SOURCE `docs/STATE.md` section 4 item 7, which does not exist yet). Proposal: the conductor talks to that command, not to s6 directly.
- **Deadlines:** each call has a timeout (placeholder 5 s); a missing tool or a failing call is an error to log and escalate, not to retry in a loop.
- **Absent:** the init's tool missing: the conductor reports "cannot control services" as a bar alert and changes nothing.

### 6. watcher to compositor and hubd (PRIVATE)
- **What the repo records (SOURCE `docs/proposals/hub-stability.md` 9.1, TESTED there in the cloud environment):** two probes of the compositor: driftwm's IPC `state` request (5 s deadline) and a Wayland `wl_display.sync` round trip (2 s); **both must fail**; kill only after at least 4 failed rounds and at least 30 s since the first failure; at most 3 kills in 10 minutes, then stop restarting and show a message, never reboot; guards against memory and CPU pressure are proposed; every kill is logged with evidence.
- **Not recorded:** how hubd is probed (today the boot confirm step asks `hubd list` five times: SOURCE `docs/STATE.md` 4), where the watcher lives (the hang rule waits for the init comparison: `HUB-OS.md`), the watcher's own supervision.
- **Class:** PRIVATE (it speaks each target's own existing socket; no new protocol). **Absent:** compositor absent: nothing to probe, the init restarts it; watcher dead: the init restarts the watcher; neither reboots the machine.

### 7. worker to the clipboard history (PRIVATE)
- **Repo (SOURCE `docs/proposals/clipboard.md`):** the store `clipd` listens on `/run/hubos/clipd.sock` (proposal); workers connect with `hello`, push items with `add` (an item record with data inline), and the store can send `set` to a worker for a targeted paste; the store never connects out.
- **Owner's text:** one-way push, text only from nodes in v1, hubd does not relay.
- **Disagreements:** (a) `clipboard.md` also has the store sending `set` to workers (paste to a node), so capture is one-way but the socket is not; (b) its item record covers images and file references; "text only in v1" is a narrowing of the owner's 2026-10-06 decisions; (c) hub-to-node paste is the node helper's `PUT /v1/clipboard` (SOURCE `node-helper-api.md` 4.6), which `hubd` or the store would call; "hubd does not relay" holds for node-to-hub capture, and hub-to-node needs an owner answer (question 4).
- **Drop policy:** the worker's queue to `clipd` is bounded (placeholder 8 items, newest kept); an item over 1 MiB is not sent (SOURCE `HUB-OS.md` open question: size limit 1 MiB for the push).
- **Absent:** `clipd` gone: the worker drops new items, counts them, reconnects with backoff; copying on the node itself is never affected.

### 8a. producers to the toast daemon, hub side (PRIVATE)
- **Repo (SOURCE `notifications.md`):** `notifd` listens on a Unix stream socket (proposal path `/run/hubos/notifd.sock`), one JSON object per line, a request id and a deadline; senders are `notifctl send` and `hubd`.
- **Drop policy:** a producer that cannot reach `notifd` drops after a short bounded wait (placeholder 1 s) except "serious" ones, which are re-sent when it returns (BELIEVED; question 5). **Absent:** `notifd` gone: nothing shows; `hubd` keeps its bar alert independently.

### 8b. nodes to the hub for notifications (PUBLIC: every node's distro implements it)
- **Repo (SOURCE `notifications.md` 6):** the hub polls `GET /v1/notifications?after=<seq>&wait=<s>` (long poll) on each node's helper; programs on the node call `POST /v1/notifications` on loopback, through a wrapper `hubos-notify` (the repo says "would call it (not designed)").
- **Who polls:** the owner's text says the conductor; `notifications.md` question 3 leaves it open between `notifd`, the workers and `hubd`. The conductor does not exist yet.
- **Version policy:** the HTTP path `/v1` and `api` in `/v1/status` are the schema id (rule 4). **Deadline:** the long poll has a wait of at most the helper's limit and the hub's request deadline is wait + a margin. **Absent:** node down: the poller backs off, and one serious "node down" is raised through `hubd` (SOURCE `notifications.md` section on failures).

### 9. mixer to PipeWire (PUBLIC: PipeWire is a standard interface)
- **Repo:** the hub plays every node's sound mixed by PipeWire, with per-node mute and volume and a master (SOURCE `HUB-OS.md`); **matching a PipeWire stream to a node (by process id or stream name) is unverified** (SOURCE `HUB-OS.md`, Unverified); PipeWire is not in the image yet (SOURCE `from-scratch-hub.md`).
- **Not recorded:** which PipeWire interface the mixer uses (library or command line tools), and the mixer's own process. **Version policy:** PipeWire's own. **Absent:** PipeWire gone: the mixer shows "no sound server" and nothing else changes; the mixer gone: sound plays at the last volumes.

### 10. workshop to node helper (PUBLIC)
- **Repo (SOURCE `distro-workshop.md` 3.5):** the call to ask a running node to "update now from this URL" is **not in `node-helper-api.md`** (UNKNOWN how the workshop asks a running node). The recovery agent's `POST /v1/install` with `{"Slot","BaseURL"}` exists only in recovery (SOURCE `node-helper-api.md` 2).
- **Proposal for the missing call (placeholder name):** `POST /v1/update` with `{"base_url": "…", "release": "…"}`; answer `202` and the progress through `GET /v1/status` (`update_pending`), never a long-held request. **"Busy, refuse":** the existing error table has `409 busy` for "a long job is running and `force` was not given" (SOURCE `node-helper-api.md` 3.3); the refuse-while-busy hook itself is not built (SOURCE `docs/image.md` 6).
- **Version policy:** the `/v1` path and `api`/`min_hub` in status (SOURCE 3.2). **Absent:** node down: the workshop reports it and does not retry in a loop; a promoted release always starts as a trial boot with automatic rollback (SOURCE `distro-workshop.md` 3.5), independent of this seam.

## 5. Failure table (all seams)
| Event | Required behaviour |
|---|---|
| One side crashes | The other keeps running and shows stale/last state; no cascade |
| One side hangs | Deadline (request/response) or drop (one-way) fires; the other side logs once, then rate-limited |
| Version mismatch, PRIVATE | Both refuse by name; the conductor reports it as an alert |
| Version mismatch, PUBLIC | Clean rejection with the supported versions named |
| Reader slow | Drop policy applies; drop count is a counter, visible in the burn-in report (`burn-in-tool.md`) |
| Message too large | Rejected, counted, connection kept |
| Reconnect storm | Backoff with jitter (placeholder 1 s doubling to 30 s) |

## 6. Tests (when built)
1. For each seam: kill the producer; the consumer keeps running and shows stale state.
2. For each seam: kill or SIGSTOP the consumer; the producer never blocks longer than its deadline or drop policy.
3. PRIVATE seam with a different build id: both refuse by name.
4. PUBLIC seam with an unknown schema id: clean named rejection.
5. Flood a one-way seam: the queue stays bounded; the drop counter rises.
6. Absent-side start order: start each side first, in both orders; both come up.
7. Log flood: 1000 identical errors give a handful of lines.

## 7. Five assumptions I am least sure of
1. That the conductor will exist and own node polling (BELIEVED; today `hubd` is the only client of the node helper).
2. That Erlang can use SOCK_SEQPACKET and descriptors (UNKNOWN).
3. That the init-neutral service command will be built (it is on the backlog; the init is not chosen).
4. That "newest frame wins" is enough and no frame ever needs to be delivered (BELIEVED).
5. That the feed can stay a full-state line forever without a size problem at 5000 machines (BELIEVED: `hubd list` is 2.9 KB at 5000 machines; SOURCE `hubd-slice2.md` 7.4).

## 8. Questions for the owner
1. Should PUBLIC seams have a formal compatibility test suite?
2. Should the panel be able to reconnect without `hubd` restarting? (Assumed yes.)
3. Keep the feed as a full-state line each time, or change to "snapshot plus updates"?
4. Hub-to-node paste: is the node helper's `PUT /v1/clipboard` called by `hubd`, by the store, or by a worker?
5. Should serious notifications be re-sent when the toast daemon returns?
6. Which seams should get a written schema id now and which later?
