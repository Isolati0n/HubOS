# PROPOSAL: the hub notification daemon (toasts, bar badge, list of older ones, notifications from nodes)

**Status: PROPOSAL, docs only. Nothing here is decided beyond the owner decisions in section 1, and nothing here is built into the images or into hubd.** `HUB-OS.md` wins if this file disagrees with it. Written 2026-10-06 by a helper agent (no sub-helpers). The only code is throwaway; the shell scripts I used for the `mako` tests are quoted in section 9. Every choice is a question in section 12.

**Labels on every item:**
- **TESTED**: I ran it in this build environment; the exact command and output are given.
- **SOURCE**: read in a file at a named place; read on 2026-10-06. "(repo)" means a file of this repository written by an earlier session and not re-checked by me.
- **BELIEVED**: I think it is true, I did not test it and have no source.
- **UNKNOWN**: nobody has checked; do not rely on it.

Companion of `docs/proposals/clipboard.md` (same method, same limits). The node helper API it extends is `docs/proposals/node-helper-api.md`, which still describes signed requests; the owner has since decided the hub's requests to nodes are **not** signed, and every call proposed here is unsigned.

---

## Plain-words summary

1. **What the owner asked for.** One notification daemon on the hub for the hub's own alerts ("nas is down") and for notifications that nodes send through the node helper. A small toast at the top right for about 5 seconds, a badge on the bar, and a list of older ones. Each carries a title, a body, a severity, the source machine and, optionally, a progress value. Serious ones (a node down, the compositor gave up) stay until you dismiss them.
2. **What I tested.** An existing small program, **mako** (a notification daemon for wlroots-style compositors; MIT/Expat licence per its Debian copyright file), can draw exactly this on the layer-shell: toasts stacked at the top right, a red-bordered one for a critical alert that **does not time out**, a progress bar drawn from a hint, an update of an existing toast by id, a "(3 more)" line when many arrive, a history of expired ones, and a command-line control (`makoctl`). TESTED on a headless `sway` (not on driftwm; driftwm does offer the layer-shell protocol, SOURCE (repo)). It needs D-Bus (the hub already has `dbus-daemon`) and links `libsystemd.so.0` (tolerated by `HUB-OS.md` for now). Unpacked with `dpkg -x`, nothing installed.
3. **What mako cannot do for us** (TESTED): when mako restarts, **every** notification is lost, including a critical one that was waiting and the history. And if mako is not running, a notification call just fails ("name not provided"). Therefore the daemon that **owns** the alerts must be ours (small, in RAM, with the severities and the "stay until dismissed" rule), and mako, if the owner likes it, is only the painter of the toasts.
4. **Nodes.** The node helper never calls the hub (a rule of its design). So I propose the hub **asks** each node: a long-poll call `GET /v1/notifications?after=<seq>&wait=<s>` (proposal, section 6). On the node, programs hand a notification to the helper through a local call on the node. Every notification gets a number from the node; the hub remembers the last number per machine and boot, so a retried poll gives no duplicates.
5. **A problem I found in the existing decisions.** "Compositor gave up" is one of the serious alerts. But when the compositor has given up there is **no screen** for a toast, and the bar (Waybar) is also a Wayland program and is gone too. `docs/proposals/hub-stability.md` says "show a fixed message on the panel from hubd"; the panel is the bar, which needs the compositor. So **I do not know where that message appears** (section 8, question 1). `hubd` and the daemon would keep running (SOURCE `hubd` run script), the alert would be in the list, and it would show as soon as a compositor returns; nothing else is decided.
6. **Failures (section 7).** The daemon dying, hanging or being slow never touches the bar or the screen: the bar badge shows STALE (same rule as `hubd feed`), senders give up after 500 ms and drop a queue of at most a few items, a node that does not answer is already shown as down by `hubd`. Serious alerts are re-sent to the toast program after it or the daemon restarts.
7. **Not decided:** the toast program (mako or our own), severity names and count, whether notifications survive a `hubd` or daemon restart, how nodes dedupe, stacking, whether serious alerts clear themselves when the cause goes away, who owns the "list of older ones". All in section 12.
8. **First slice (section 10):** a daemon with a store, a command-line sender, a bar badge feed and mako as painter, tested here with a fake node helper endpoint; no node side yet.

---

## 1. What the owner already decided

All SOURCE: the task text of 2026-10-06 and `HUB-OS.md`, read 2026-10-06.

- A daemon for the hub's own alerts and for notifications from nodes through the node helper API.
- Small toasts at the top right for about 5 seconds; a bar badge; a list of older ones.
- Each notification carries a title, a body, a severity, the source machine and an optional progress value.
- Serious ones (node down, compositor gave up) stay until dismissed.
- Context: no passwords, no encryption on display connections; the hub's requests to nodes are not signed; the hub's core will be a session layer (workers, presenter, conductor; native-core language not decided), so the notification daemon is a **separate process behind a small local interface**, independent of that language; the bar has a clock, the clipboard, the notification daemon and more later; one display only.
- Rules that apply: `hubd` is Go; no systemd; no web dashboard as the control plane; the hub never reboots itself for a service failure; priority order uptime and stability first; "No alerts are sent to the owner's phone" (`HUB-OS.md`, Explicitly out: Phone alerts). So the daemon must **not** forward anything to the phone. (The phone web page may show the bar; whether it shows the badge is not asked.)

---

## 2. What exists today (SOURCE, read 2026-10-06, unless marked)

| Piece | State | Source |
|---|---|---|
| `hubd`'s bar alert: the module `custom/hub` shows "N of M up", class `alert` in red when a machine is down; the tooltip lists the down machines; the feed marks STALE | exists | `waybar.json`, `internal/hub/view.go`, `internal/hub/ipc.go` |
| Panel states up, down, in recovery, unreachable | decided | `HUB-OS.md` |
| "Alerts" on the bar for a degraded service: "an alert shows on the bar; the owner decides" | decided, not built | `HUB-OS.md` |
| Compositor restart policy: 5 crashes in a minute then stop, fixed message, `hubd` and the recovery terminal stay up, never reboot | decided, not built | `HUB-OS.md`, `docs/proposals/hub-stability.md` lines 16, 300, 576 |
| `hubd` runs without the compositor after it gave up (`FOLLOW_GAVEUP=run`) | exists | `image/machines/hub/rootfs/etc/s6/sv/hubd/run` |
| D-Bus session bus (`dbus-daemon` alone) at `/run/dw/bus` | exists | `docs/proposals/hub-isolation.md` row 5; `tools/image/hub_test.go` |
| A notification daemon, `notify-send`, or any D-Bus notification client in the image | **none** (`image/packages/hub.list` has no mako and no libnotify) | `image/packages/hub.list` |
| The node helper | does not exist; design only; "answers only when asked; never calls the hub" | `docs/proposals/node-helper-api.md` 3.1 |
| Notification code anywhere in the repository | none (grep of `internal`, `cmd`, `docs` found no daemon; only mentions in the viewers research of Sunshine "tray, notifications") | grep, 2026-10-06 |

---

## 3. What I tested with mako (all TESTED, scratch folders deleted afterwards)

**Setup.** `apt-get download mako-notifier` (Ubuntu 24.04, version `1.8.0-2build2`) and the `sway`/`grim`/`wayland-utils` set from `tools/image/experiments/clipboard-bridge/README.md`; unpacked with `dpkg -x` into a scratch folder; a headless `sway` 1.9 (`WLR_BACKENDS=headless WLR_RENDERER=pixman`, 1920x1080) as the compositor; `dbus-daemon --session` on a socket in a short folder (`/tmp/cn1`; a long folder name breaks Wayland sockets, see the clipboard document); mako with this configuration (the same file for all runs, `max-visible=3` in the second):

```
anchor=top-right
layer=overlay
default-timeout=5000
max-history=50
width=380
margin=10
[urgency=critical]
default-timeout=0
border-color=#cc3333
```

**What mako needs and links (command output):**

```
$ ldd mako | grep -i 'systemd\|not found'
	libsystemd.so.0 => /lib/x86_64-linux-gnu/libsystemd.so.0
$ dpkg-deb -f mako-notifier_1.8.0-2build2_amd64.deb Depends
init-system-helpers, libc6, libcairo2, libgdk-pixbuf-2.0-0, libglib2.0-0t64, libpango-1.0-0, libpangocairo-1.0-0, libsystemd0 (>= 243), libwayland-client0, libwayland-cursor0
```

SOURCE (licence): `usr/share/doc/mako-notifier/copyright` in the unpacked package: `License: Expat` (the MIT licence). The owner's project is unlicensed and private, so a licence is not a blocker; recorded only as SOURCE.
`libsystemd0` is the plain library `HUB-OS.md` tolerates ("libsystemd0 is tolerated for now as a plain library because Waybar and dbus-daemon link it"). mako's dependency on GTK is **none** (cairo, pango, gdk-pixbuf, glib): lighter than Waybar's.

**The compositor offers what mako needs (on the headless sway):** `zwlr_layer_shell_v1` version 4. driftwm offers `zwlr_layer_shell_v1` v5 (SOURCE (repo) `docs/driftwm-findings.md` section 5, via `file-transfer.md`); whether mako draws correctly under **driftwm** is UNKNOWN (waybar and wofi do).

**Test 1: toasts, severity, progress (script `t1.sh`).** Three calls to the standard D-Bus method `org.freedesktop.Notifications.Notify` with `gdbus call` (`gdbus` is in this container; the hub image has no `gdbus`, it has `dbus-daemon` only):

```
$ gdbus call --session --dest org.freedesktop.Notifications --object-path /org/freedesktop/Notifications \
    --method org.freedesktop.Notifications.Notify "hubd" 0 "" "nas is down" "No answer for 10 s" "[]" "{'urgency': <byte 2>}" "int32 0"
(uint32 2,)
... "ai-1" ... "Copying" "to nas" "[]" "{'value': <int32 40>}" "int32 -1"   -> (uint32 3,)
$ makoctl list | jq -c '.data[0][] | {summary:.summary.data, urgency:.urgency.data, app:.["app-name"].data, id:.id.data}'
{"summary":"Copying","urgency":255,"app":"ai-1","id":3}
{"summary":"nas is down","urgency":2,"app":"hubd","id":2}
{"summary":"Job finished","urgency":255,"app":"ai-1","id":1}
```

(urgency 255 means "not set", shown as normal.) A screenshot taken with `grim` (a screenshot program for wlroots-style compositors) showed the three toasts stacked at the top right, newest on top; the "Copying" toast with a **progress bar filled to about 40%** (from the `value` hint); the critical one with a red border. After 6 seconds:

```
$ makoctl list | jq -c '.data[0][] | {summary:.summary.data}'
{"summary":"nas is down"}                       <- the critical one (timeout 0) stays; normal ones expired
$ makoctl history | jq -c '.data[0][] | {summary:.summary.data,body:.body.data,app:.["app-name"].data}'
{"summary":"Copying","body":"to nas","app":"ai-1"}
{"summary":"Job finished","body":"Render 12 done on ai-1","app":"ai-1"}
$ makoctl dismiss -a ; makoctl list                  -> empty
```

**Test 2: update by id, stacking, dismiss one, restart (script `t3.sh`):**

```
== replace by id: progress 10 -> 60 -> 100    (the second argument of Notify is replaces_id = 1)
(uint32 1,) (uint32 1,) (uint32 1,)
{"id":1,"summary":"Copying","body":"done"}      <- one toast, updated in place
== 5 more at once with max-visible=3            -> screenshot: 3 toasts and a line "(3 more)"; makoctl list shows 6 entries
== a critical one (urgency 2, timeout 0) after that: makoctl list shows 7 entries; its place on the screen was not checked
== makoctl dismiss -n 2 : the list falls from 7 to 6
== after 6 s: history length 6; the list holds only the critical one
== kill mako and start it again:   makoctl list length 0, makoctl history length 0
```

**What these results mean:**

| Fact | Label |
|---|---|
| mako draws toasts top right on the layer-shell overlay layer, newest on top | TESTED (sway) |
| Severity from the `urgency` hint (0 low, 1 normal, 2 critical) can be styled by rules; a critical one with `default-timeout=0` stays | TESTED |
| A progress bar is drawn from the `value` hint (0 to 100) | TESTED |
| Sending again with `replaces_id` updates the same toast; the id stays | TESTED |
| `max-visible` hides the rest behind "(N more)" | TESTED |
| History of expired ones: `makoctl history`, bounded by `max-history` | TESTED |
| Dismiss all, dismiss one by id | TESTED |
| **A mako restart loses every active notification (including critical) and the history** | TESTED |
| With mako not running, a `Notify` call fails with `ServiceUnknown: The name org.freedesktop.Notifications was not provided by any .service files` | TESTED |
| Whether a critical toast arriving while three others fill `max-visible` is visible, or sits behind "(N more)" | UNKNOWN (not checked: exactly the case that matters; question 8) |
| mako's `on-notify` hook (run a command on each notification) exists | SOURCE (`mako(5)`, line "on-notify = action", read 2026-10-06) |
| Whether mako works under driftwm, with fractional scale on a 4K projector, and its font and size on the projector | UNKNOWN |
| How long a notification takes from the call to being drawn on the screen | not measured |

**The toast could also be drawn by our own program** (a layer-shell client). I did not write one; it would be a bigger program than the whole daemon. Whether mako is acceptable is question 2.

---

## 4. The components (proposal)

```
 hubd (node down, recovery, ...)  ----\
 clipd (history lost, pins ...)  -----\                              Waybar: custom/notif  <-- notifctl feed
 compositor supervisor (gave up) -------> notifd (the daemon) ------>  (badge: count + worst severity)
                                          store in RAM                 
 node helper A  <-- long poll -------- poller A                       toast painter (mako, or own)
 node helper B  <-- long poll -------- poller B  (inside notifd,       <- D-Bus Notify / replaces_id
 ...                                    one per machine)               
                                                                       notifctl list / dismiss / pick  --> wofi list (older ones)
```

| # | Component | What it does | Runs as | Exists? |
|---|---|---|---|---|
| N1 | **`notifd`, the daemon** | Owns the notification store (RAM). Accepts hub-side notifications on a local socket. Runs one poller per machine for node notifications. Applies severity rules (what stays, what expires), de-duplication, the cap. Sends toasts to the painter and re-sends serious ones after a painter restart. Answers the bar feed and the list. | s6 service, user `hub` | no |
| N2 | **Pollers** | One per machine: long poll of the node helper's `GET /v1/notifications`; tracks `boot_id` and `seq`; backs off when the machine is down. Inside `notifd` or a worker of the session layer (question 3). | inside `notifd` | no |
| N3 | **Toast painter** | Draws toasts at the top right; or **mako** (TESTED). | s6 service, user `hub`, waits for the compositor | no |
| N4 | **Bar badge** | `notifctl feed` for a Waybar custom module: count of unread, the worst severity as the class, tooltip with the latest ones. Click opens the list. | child of Waybar | no |
| N5 | **List of older ones** | A wofi list (like the machine list) with source, time, title; pick one to dismiss or to mark read. An own list later. | started by click | no |
| N6 | **Senders** | `notifctl send ...` (command line, for scripts); `hubd` and others call the socket directly. | | no |
| N7 | **Node side** | The helper's `POST /v1/notifications` (local) and `GET /v1/notifications` (the hub's long poll). | the node helper | no |

**Local interface (proposal, language-independent):** a Unix stream socket `/run/hubos/notifd.sock`, mode 0600 in the folder `hubd` already uses (SOURCE `internal/hub/ipc.go`), one JSON object per line, each request with an `id`, a deadline on every call. Same pattern as `hubd`'s socket.

```json
{"id":1,"cmd":"notify","n":{ ...notification record, section 5... }}
{"id":2,"cmd":"list","state":"unread|all","limit":100}
{"id":3,"cmd":"dismiss","id":"n-000123"}       // or "all":true
{"id":4,"cmd":"read","id":"n-000123"}
{"id":5,"cmd":"feed"}                          // one line whenever the badge changes, at most one per second
{"id":6,"cmd":"resolve","key":"node-down:nas"} // the cause is gone: clears a serious one that has a key
```

If mako is the painter, `notifd` talks to it with the **freedesktop D-Bus method `Notify`** (a standard interface; `urgency` and `value` hints, `replaces_id`, `expire_timeout`; SOURCE: the freedesktop Desktop Notifications specification, **from memory, not re-read by me**; what I tested is that mako accepts these exact calls). `notifd` then needs a small D-Bus client: UNKNOWN which (Go has third-party libraries; none is chosen; the image has no `gdbus`).

---

## 5. The data formats (proposal)

### 5.1 The notification record

```json
{
  "id": "n-000123",
  "seq": 123,
  "t_ms": 1791285228327,
  "source": {"kind": "node", "machine": "ai-1", "app": "render"},
  "title": "Job finished",
  "body": "Render 12 done on ai-1",
  "severity": "info",
  "progress": null,
  "key": "render-12",
  "count": 1,
  "state": "unread",
  "sticky": false,
  "first_ms": 1791285228327,
  "dismissed_ms": null
}
```

| Field | Meaning | Limit (proposal) |
|---|---|---|
| `id` | `n-` and a counter, never reused while `notifd` runs | |
| `t_ms` | last time this notification (or an update of it) arrived, the hub's clock | |
| `source.kind` | `hub` (own alert) or `node` | |
| `source.machine` | the machine id (inventory); for `hub` kind, the part that raised it (`hubd`, `clipd`, `compositor`) is in `app` | |
| `title` | the one line shown in bold | at most 120 bytes UTF-8, cut with `…` |
| `body` | the text under it | at most 1,024 bytes |
| `severity` | a level name; **the names and how many are the owner's choice** (question 5); the proposal below uses `info`, `warning`, `serious` only so the tests can be written | |
| `progress` | a whole number 0 to 100, or `null` | |
| `key` | optional: a sender-chosen name for "the same thing"; a new notification with the same key and source **replaces** the old one (progress updates, "node down" repeated) instead of adding another | at most 64 bytes |
| `count` | how many times this key arrived | |
| `state` | `unread`, `read`, `dismissed`; a toast that timed out is `unread` in the list (the badge counts it) until read or dismissed (question 7) | |
| `sticky` | derived: true for `serious` (stays until dismissed, or until `resolve` for its key if the owner wants that, question 9) | |
| `first_ms`, `dismissed_ms` | for the list and diagnosis | |

Markup in the text is **not** interpreted (mako's `markup=0`; the bar escapes as `hubd feed` does, SOURCE `internal/hub/view.go`): text from a node must never be able to inject markup into the bar or the toast. (BELIEVED a sensible rule; the Waybar 0.9.24 markup problem is documented in `internal/hub/view.go`.)

### 5.2 Storage and cap

In RAM only in this proposal (question 6 asks whether anything must survive a `notifd` or `hubd` restart). Cap: 200 records (BELIEVED a guess); serious and unread ones are removed last; the oldest read or dismissed ones go first. A `serious` record is never evicted to make room for others, so 200 serious ones would fill the store: the cap then drops the **oldest serious** and counts it ("1 serious dropped") rather than refuse new ones (question 10).

---

## 6. The node helper API call (BELIEVED / proposal; unsigned; nothing exists)

**Why polling.** The node helper "answers only when asked; it never calls the hub" (SOURCE (repo) `node-helper-api.md` 3.1). A push from node to hub would reverse that rule (question 3). A long poll keeps the rule and costs one open request per node.

### 6.1 For the hub: `GET /v1/notifications?after=<seq>&wait=<seconds>`

```json
// request:  GET /v1/notifications?after=41&wait=20
// answer 200:
{"boot_id": "6f1c9a0e", "seq_max": 44, "dropped": 0,
 "items": [
   {"seq": 42, "t_ms": 1791285228327, "title": "Job finished", "body": "Render 12 done", "severity": "info",
    "app": "render", "progress": null, "key": "render-12"},
   {"seq": 44, "t_ms": 1791285229000, "title": "Copying", "body": "to nas", "severity": "info",
    "app": "rsync", "progress": 40, "key": "copy-9"}
 ]}
```

| Field | Meaning |
|---|---|
| `boot_id` | changes when the helper restarts or the node boots; if it differs from the hub's last value the hub restarts from `after=0` and does not treat old numbers as duplicates |
| `seq`, `seq_max` | the node's own counter, strictly increasing within one `boot_id`; `seq_max` is the newest it has |
| `dropped` | how many items the node threw away since `after` because its queue was full |
| `items[]` | up to 50 per answer; fields as in 5.1 without hub-only ones; `t_ms` is the **node's** clock (the hub also stamps arrival and uses its own for ordering) |

Behaviour: if there is something with `seq > after` the call answers at once; otherwise it waits up to `wait` seconds (**at most 25**) and answers with `items: []`. The hub passes the highest `seq` it has stored as `after`. Items are kept on the node until the hub asks for a higher `after` (or the ring is full): at most **64** items, oldest dropped, `dropped` counts them.

### 6.2 For programs on the node: `POST /v1/notifications` (loopback only)

```json
{"title": "Job finished", "body": "Render 12 done", "severity": "info", "app": "render", "progress": 100, "key": "render-12"}
```
Answer `202 {"seq": 45}`. Accepted only from the node's own loopback address (BELIEVED enough given the owner's "perfect network"; the helper checks the peer address); a command-line wrapper on the node (`hubos-notify`) would call it (not designed). Limits: body of the request at most 4 KiB; title 120 bytes; body 1,024 bytes (longer is cut, not refused); at most 20 notifications per second per node (then `429`); `severity` outside the allowed names is refused with `400 bad_request` (the helper's error format, SOURCE (repo) `node-helper-api.md` 3.4).

### 6.3 How the node's own D-Bus notifications could be picked up (not proposed, mentioned because it is how existing programs notify)

Programs that already use the standard `Notify` D-Bus call would need a small D-Bus notification server on the node feeding the helper. Each node has its own distro; whether it has D-Bus at all is not decided (the hub declares one; `HUB-OS.md`: "may be declared by any machine"). UNKNOWN; not proposed.

### 6.4 De-duplication of node notifications

1. `seq` with `boot_id`: the hub stores the last `seq` per machine and `boot_id`, and ignores `seq <=` that (TESTED nowhere; BELIEVED sufficient for retries).
2. A helper restart changes `boot_id` and restarts `seq` at 1: items that were already delivered but are still in a persisted node queue could repeat. If the node queue is in RAM (BELIEVED) they are gone with the restart, so there is nothing to repeat.
3. The same `key` from the same machine replaces the existing record (updates progress or text) instead of adding one; with no `key` each item is its own record.
4. Content duplicates (the same title and body from the same machine and app within 10 seconds) are merged into one record with `count` raised (proposal, a guess).
Whether the hub should also remember the last `seq` across its own restart is question 6 (a restart with an empty store would otherwise show a node's still-queued items again).

---

## 7. Deadlines and failure behaviour

**Rules:** every call has a deadline; nothing waits without one; a failed part is marked STALE on the bar (the rule `hubd` already has, SOURCE `HUB-OS.md`); no failure of the notification system may freeze the bar, the compositor or `hubd`'s panel; no failure may reboot the hub. **All numbers are guesses to be measured.**

| Part | What the owner sees if it dies | Hangs or is slow | Restart | Deadline (guess) |
|---|---|---|---|---|
| **`notifd`** | The bar badge shows STALE (grey) with the time of the last good state, via `notifctl feed`'s own rule (no heartbeat for 3 s). Toasts stop. Senders (`hubd`, `clipd`) keep at most **8** pending notifications and drop older ones, counted. After the restart the store is empty (RAM) unless question 6 says otherwise; `hubd` re-sends the alerts it **still** believes are true (for example "nas is down" if nas is still down) so serious ones come back. | Senders give up after 500 ms. `notifctl feed`: no heartbeat in 3 s means STALE. A hung `notifd` is not restarted by s6 (s6 restarts only dead processes): UNKNOWN who kills it (the same question as for `clipd`, clipboard.md question 13). | s6, at once | call 500 ms; heartbeat 1 s; STALE 3 s |
| **The toast painter (mako)** | No toasts. `notifd` keeps the store and the badge. **mako loses everything when it restarts (TESTED)**, so `notifd` must re-send the serious and the unread-recent ones when it sees the painter come back. | `notifd` calls the painter with a 500 ms deadline and drops the toast (not the record) on a timeout; a slow painter must not slow the store (separate sender queue, bounded) | s6; the painter waits for the compositor like Waybar | 500 ms |
| **The compositor (gave up)** | No toast and no bar (both are Wayland clients). The store still records alerts; the badge and toasts appear when a compositor returns. **Where the "fixed message" of `hub-stability.md` is shown is UNKNOWN** (question 1). | | | |
| **A node's poller** | The node's notifications stop arriving; `hubd` already shows the node down; `notifd` raises **one** serious "node down" per machine through `hubd` (not by itself), with the key `node-down:<machine>` | The long poll is abandoned after `wait` + 5 s | Backoff 1, 2, 4 ... 30 s while down; resets on success | `wait` + 5 s |
| **A node's helper restarts** | `boot_id` changes; the hub restarts from `after=0` | | | |
| **Bar feed (`notifctl feed`)** | Waybar keeps the last line; Waybar restarts the command | at most one line per second | | 1 s |
| **The list (wofi)** | Does not open: one-line message on the badge tooltip | `timeout 120`; single-instance guard (SOURCE (repo) `bar-findings.md` 325-334) | | 120 s |
| **A flood** (a node sends 1000 a second, or `hubd` flaps) | The badge count rises; toasts are limited: at most 3 visible (mako `max-visible` TESTED) plus "(N more)"; serious ones are kept even if toasts are hidden | Per-source rate limit (proposal: 20 per second per node, section 6.2; 10 per second per hub component), the excess counted in `dropped` | | |

Cross-checks with decided behaviour:
- **Flapping.** `hubd` already marks a machine down on probe results with STALE rules (SOURCE `internal/hub/view.go`). The "node down" alert should follow `hubd`'s state and not raise a new alert on every probe. Whether an extra delay (hysteresis) is wanted before the serious alert is question 9.
- **Never reboot:** a degraded service gives an alert and "the owner decides" (`HUB-OS.md`); this daemon only displays it.

---

## 8. Where serious alerts show, and the compositor problem

Facts: `HUB-OS.md` says that after 5 compositor crashes in a minute the hub "stops restarting it, shows a fixed message, keeps `hubd` and the recovery terminal up" and `docs/proposals/hub-stability.md` lines 300 and 576 say "show a fixed message on the panel from `hubd`". The panel is Waybar plus wofi (SOURCE `HUB-OS.md` Panel), both Wayland clients, so they cannot show anything with no compositor. **Nothing in the repository says what draws the message** (grep of `docs/proposals/bulletproof-compositor.md` and `hub-stability.md` for message, console and framebuffer found no answer). Candidates (none tested, none decided; question 1): the text console of the projector's output; a tiny drawing program that writes straight to the display (KMS) when no compositor is running; the recovery terminal's own banner; `hubd`'s phone web page (an HTTP page, which does not need the compositor; it exists only as a decision). All UNKNOWN.

---

## 9. Scripts I used (so the results can be repeated; they are in the scratchpad, not committed)

Setup common to both runs (a short runtime folder; `ulimit -c 0`):

```
ulimit -c 0; W=/tmp/cn1; mkdir -p $W/run; chmod 700 $W/run
export XDG_RUNTIME_DIR=$W/run WLR_BACKENDS=headless WLR_RENDERER=pixman WLR_LIBINPUT_NO_DEVICES=1
echo "output HEADLESS-1 resolution 1920x1080" > $W/sway.conf; sway -c $W/sway.conf &
export WAYLAND_DISPLAY=wayland-1
dbus-daemon --session --fork --address=unix:path=$W/run/bus; export DBUS_SESSION_BUS_ADDRESS=unix:path=$W/run/bus
mako -c $W/mako.conf &
```

A notification call: `gdbus call --session --dest org.freedesktop.Notifications --object-path /org/freedesktop/Notifications --method org.freedesktop.Notifications.Notify APP REPLACES_ID "" TITLE BODY "[]" "{'urgency': <byte 2>}" "int32 TIMEOUT"` (a plain `-1` is read by `gdbus` as an option; write `"int32 -1"`; I lost one run to that). One thing I learned: the screenshot program `grim` works against the headless `sway` through `zwlr_screencopy_manager_v1`.

**What I did not run:** mako under driftwm; mako on a 4K output; Waybar with a notification module; wofi for the older list; any node helper; any D-Bus client library; the QEMU hub image (a full run takes hours and another helper runs long jobs; the owner's one-long-job rule).

---

## 10. What can be tested, and the first slice

### 10.1 What can be tested where

| Test | Where | Label |
|---|---|---|
| Store rules: caps, severities, sticky, replace by key, de-duplication, serious never evicted first, counters | unit tests, no compositor | BELIEVED easy |
| Node long poll against a fake helper endpoint (an HTTP server with `GET /v1/notifications`): `boot_id` change, `dropped`, wait time-out, a hung server (no answer), a node going away, 20 pollers | this machine; a small Go or Python fake | BELIEVED; the fake does not exist (`tools/fakenode` only accepts and closes TCP connections, SOURCE `tools/fakenode/main.go`) |
| Toasts with mako, severities, progress, replace, stacking | this machine, headless sway | TESTED (section 3) |
| Re-send of serious alerts after a mako restart | this machine with `kill` of mako and a script | BELIEVED easy |
| Bar badge (`notifctl feed` in Waybar) and the list (wofi) | needs Waybar and wofi; the nested hub image has both | SOURCE (repo) for the pattern; not re-run |
| mako, `dbus-daemon`, s6, `follow-driftwm` services on the real image | `go test -tags qemu ... ./tools/image` (hours; not run by me); the image needs `mako-notifier` and a D-Bus client added to `image/packages/hub.list` | UNKNOWN until run |
| Behaviour under driftwm (layer, stacking with the bar, exclusive zone, 4K scale) | image run only | UNKNOWN |
| "Compositor gave up" display | no way to test until the owner decides what draws it | UNKNOWN |

### 10.2 First slice (smallest useful; proposal)

**Slice N1: store, sender, badge, painter.**
1. `notifd` with the record, RAM store, severity rules (`info` expires after 5 s on the toast; `serious` is sticky), replace by key, caps, and the socket commands `notify`, `list`, `dismiss`, `read`, `feed`.
2. `notifctl send|list|dismiss|feed`.
3. Painter: mako configured as in section 3, fed through D-Bus `Notify`; `notifd` re-sends sticky ones when mako's name appears again on the bus.
4. Waybar module `custom/notif` with `notifctl feed` (count, class by worst severity, tooltip), click opens a wofi list.
5. Hub-side senders: a command that `hubd` would call later (not part of the slice).
Tests: the store unit tests; the mako runs of section 3 as a script with expected output; kill -9 of `notifd` and of mako; a flood of 1,000 notifications (bounded memory, badge responsive).

**Slice N2: nodes.** The helper's two calls in a **fake helper** (a small HTTP server), the pollers, backoff, `boot_id`, `dropped`, de-duplication, node down/up through a fake `hubd`.

**Later:** hub-side alerts from `hubd`, `clipd` and the compositor supervisor; the "compositor gave up" display; own toast painter if mako is not wanted; persistence if the owner wants it.

---

## 11. Table of items

| # | Item | Label |
|---|---|---|
| 1 | mako 1.8.0 draws toasts at top right, stacked, newest first | TESTED (sway) |
| 2 | Critical urgency with `default-timeout=0` stays; normal ones expire; history of expired | TESTED |
| 3 | Progress bar from the `value` hint; update by `replaces_id` | TESTED |
| 4 | `max-visible` and "(N more)" | TESTED |
| 5 | mako restart loses active and history | TESTED |
| 6 | Notify with mako absent fails with ServiceUnknown | TESTED |
| 7 | mako links `libsystemd.so.0` (and cairo, pango, gdk-pixbuf, glib), not GTK | TESTED (`ldd`) |
| 8 | mako licence Expat | SOURCE (Debian copyright file) |
| 9 | mako `on-notify` hook exists | SOURCE (`mako(5)`) |
| 10 | Critical toast while 3 others are shown | UNKNOWN |
| 11 | mako under driftwm and at 4K | UNKNOWN |
| 12 | Image has `dbus-daemon`, no D-Bus client, no mako | SOURCE (repo) |
| 13 | The node helper never calls the hub | SOURCE (repo, design) |
| 14 | The long-poll call and the local post call | PROPOSAL |
| 15 | No phone alerts | SOURCE (`HUB-OS.md`) |
| 16 | What shows "compositor gave up" | UNKNOWN |
| 17 | All deadlines and caps | BELIEVED (guesses) |

## 12. Questions for the owner (batched; nothing decided)

1. **The compositor gave up.** What draws the fixed message when there is no compositor (the console, a tiny KMS drawing program, the recovery terminal, the phone page)? Without it the "serious ones stay until dismissed" rule cannot apply to that alert on the hub's screen.
2. **Toast painter.** Use mako (MIT/Expat, tested, needs D-Bus, links libsystemd0 which you tolerate), or write our own layer-shell toast program? Is a small D-Bus client library (Go or other) acceptable in the daemon?
3. **Node transport.** Long poll by the hub (my proposal; keeps "helper never calls the hub") or node pushes to the hub? Poller inside `notifd`, inside the session layer's workers, or inside `hubd`? Separate process for `notifd`, or part of `hubd`?
4. **Which hub-side alerts.** The list I know of: node down, node in recovery, hub's network suspected, a service degraded, compositor gave up, clipboard store restarted, pins not saved. Which others? Which of them are serious?
5. **Severity levels.** How many and which names (info/warning/serious? low/normal/critical? four levels?) and which of them stay until dismissed; does the colour of the badge follow the worst unread level?
6. **Persistence.** Must notifications (especially serious ones and unread ones) survive a restart of `notifd` or of `hubd` or a reboot? Where would they live (RAM only, or a file under `/config`)? Should the hub remember each node's last `seq` across a restart?
7. **When is a toast "read".** Does a toast that timed out count as unread in the badge until the owner opens the list?
8. **Toast stacking.** Newest on top (as tested) or bottom; how many visible at once; may a serious toast always show above the "(N more)" line; maximum height; width; font size on the 4K projector?
9. **Serious alerts and their cause.** Stay until dismissed literally, or also clear themselves when the cause is gone (node back up)? A delay before "node down" is raised (flapping)? Should a recovered node show an "info: nas is back"?
10. **Caps.** 200 records, 64 queued per node, 8 queued at a sender, 20 per second per node: acceptable? What if serious ones fill the store?
11. **How nodes dedupe.** Is `boot_id` + `seq` + `key` enough, and may the hub merge identical text within 10 seconds?
12. **The node-side producer.** `POST /v1/notifications` on loopback with a `hubos-notify` command: acceptable? Do nodes need to pick up standard D-Bus notifications (each node has its own distro)?
13. **Actions and sound.** Buttons on toasts (open that machine's window, dismiss)? A sound? (I assumed none; sound is mixed per node by PipeWire.)
14. **Progress.** Is a progress notification one record that is updated by key (my proposal), and when does a finished one expire (5 s after 100)?
15. **Bar and phone.** Position of the badge in the bar; whether the phone page shows it (phone alerts are out, a page view is not mentioned).
16. **Hub-image changes.** May `mako-notifier` (and a D-Bus client, and `wl-clipboard` for the clipboard document) be added to `image/packages/hub.list` for the first slice's image test?

## 13. Sources read (2026-10-06)

- This repository: `CLAUDE.md`, `HUB-OS.md`, `docs/proposals/node-helper-api.md`, `docs/proposals/hub-stability.md` (restart-loop lines), `docs/proposals/hub-isolation.md` (component table), `docs/bar-findings.md` (module and wofi parts), `internal/hub/ipc.go`, `internal/hub/view.go`, `tools/fakenode/main.go`, `image/machines/hub/rootfs/etc/hubos/waybar.json`, `image/machines/hub/rootfs/etc/s6/sv/hubd/run`, `image/packages/hub.list`, `tools/image/hub_test.go` (header).
- Ubuntu 24.04 packages unpacked with `dpkg -x`: `mako-notifier 1.8.0-2build2` (binary, `mako(5)` manual page, copyright file), `sway`, `grim`, `wayland-utils`, `wl-clipboard`, `waybar 0.9.24-1build3` (manual pages), `wofi 1.4.1-1build2` (manual pages).
- Not used: web search, summarising tools, other helpers' reports. The freedesktop Notifications specification was **not** re-read (marked "from memory" where used).
