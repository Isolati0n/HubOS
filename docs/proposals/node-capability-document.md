# PROPOSAL: the node capability document (what a node says it can do)

**Status: PROPOSAL. Nothing here is decided, built or tested.** `HUB-OS.md` wins if this file disagrees with it. Written 2026-10-10 by the lead agent (no helpers). No code. Labels: TESTED, SOURCE (read in a repo file or page, named), BELIEVED, UNKNOWN. **Every example value (schema string, ports, versions, names, sizes) is a placeholder, not a decision.** No protocol is invented beyond one plain request that follows the existing node helper rules.

## 1. Purpose
Today `hubd` learns two things about a node: "answers on its display port" and, once the node helper exists, the JSON of `GET /v1/live` and `GET /v1/status` (SOURCE `docs/proposals/node-helper-api.md` 4.1, 4.2). Nodes run different distros (`HUB-OS.md`: every machine runs its own distro), so one node may have audio, a text clipboard, a file manager and notifications, and another may not. The capability document is one small text file per node, published by the node helper, saying which of those a node has. The hub then shows or hides a feature per node **quietly**, without trying and failing. It is not a new streaming protocol, it carries no key and no signature (the owner removed request signing, 2026-10-06; SOURCE `HUB-OS.md`).

## 2. Where it disagrees with the existing documents
- `node-helper-api.md` speaks **JSON** (`GET /v1/status`, `api`/`min_hub` fields for versioning, section 3.2). This proposal asks for **TOML** and a `schema` string. Reason given by the owner; the cost is two formats on one helper. If the owner prefers, the same content could be JSON (question 5 below).
- `/v1/status` already carries `machine`, `state`, `image_release`, `slot`, `uptime_s`, `busy`, `update_pending`, `clipboard.supported` and `display.ready`. The new `health`, `update` and `display` sections overlap it. Two sources for one fact can disagree. Proposal: on the node both are produced from the same data in the same helper; the hub treats the capability document as the only source for "which features exist" and `/v1/live` as the only source for "is the helper answering" (question 6).
- The helper "never calls the hub" (SOURCE `node-helper-api.md` 3.1). This proposal keeps that: the hub asks.
- `status.state` there is `running`/`recovery`; health here is `healthy`/`degraded`/`critical`. They are different facts (is it in recovery vs how well is it) and both are kept.
- The recovery agent shares port and API with the helper (SOURCE `node-helper-api.md` section 2). In recovery the node has no capability document; the hub shows recovery from `/v1/status` (`state: recovery`). The `[recovery]` section here is what a **running** node says about its recovery kernel.

## 3. The call
`GET /v1/capabilities` on the node helper's port (placeholder: 8480 is only the port the repo reserves for the helper, "to be checked against the IANA registry": SOURCE `HUB-OS.md`). Answer: the TOML below, `Content-Type: application/toml`, at most 64 KiB (placeholder limit). Plain HTTP, no signing, like every other helper call.

## 4. The format (example, all values placeholders)
```toml
schema      = "node-capability-v1"   # placeholder schema id
node_name   = "ai-1"                 # must equal the inventory id
generation  = 17                     # counter, see rules

[display]                            # required
kind    = "vnc"                      # placeholder set: "vnc" | "none"
width   = 3840
height  = 2160
scale   = 1
refresh = 60

[health]                             # required
status    = "healthy"                # "healthy" | "degraded" | "critical"
uptime_s  = 86400
load_1m   = 0.42
mem_total_kb     = 33554432
mem_available_kb = 28000000
disk_free_kb     = 120000000

[update]                             # required
agent_version = "0.1.0"
slot          = "a"                  # "a" | "b"
trial_active  = false
last_result   = "ok"                 # free text, short

[recovery]                           # required
agent_version = "0.1.0"
can_rollback  = true

# Optional sections (absent = the feature is off for this node):
[audio]            # sender = "running"
[clipboard_text]   # max_bytes = 1048576
[file_manager]     # kind = "name of the node's own file manager (free text)"
[notifications]    # source = "hubos-notify"
[power]            # restart = true, shutdown = true
[node_status]
text  = "nightly job running"
level = "info"                       # "info" | "warn" | "error"
```
The four **required** sections are `display`, `health`, `update`, `recovery`. `node_status` is free text plus one of `info`, `warn`, `error`; the hub shows it, never acts on it.

## 5. Rules
1. **Unknown `schema`:** the node is treated as having **only the four required sections**; nothing optional is used; the hub logs one line **once per node per change of the schema string**. (The required sections are read with the v1 meaning; BELIEVED enough for a v2 that only adds; if v2 changes them, a v1 reader may be wrong: question 7.)
2. **Unknown sections and unknown keys are ignored** (the same rule as `node-helper-api.md` 3.2 for JSON fields; TESTED there for JSON only).
3. **A missing optional section** just turns that feature off for that node, quietly (no log, no alert).
4. **A known schema with a missing or wrong required section** is a bad document: the last good one stays, one log line per node per change, and the poll counts as failed for the staleness timer.
5. **Who polls: `hubd`** (recommendation). Reasons: `hubd` already owns the inventory, the check loop and the bar feed with its STALE marker (SOURCE `docs/hubd-slice2.md`); `node-helper-api.md` 3.1 says `hubd` is the only client of the helper; the conductor of the session layer **does not exist** (SOURCE `docs/STATE.md` 3.2). `notifications.md` question 3 leaves the place of node pollers open (notifd, workers or hubd). Other parts (workers, the notification daemon) then ask `hubd` over its existing local socket for the cached document, so the node sees one poller. If the session layer's conductor is built and the owner wants it to own node state, the poll moves there; the document does not change.
6. **Interval 30 s** (the owner's figure), per node, with the poll for different nodes spread over the interval so 20 nodes are not all asked in the same second (BELIEVED to be better; question 1).
7. **A capability counts as gone only after two consecutive successful polls without it.** A poll that fails (no answer, bad document) changes nothing about capabilities; it only moves the staleness timer.
8. **The last good document is cached** (RAM) and used until replaced.
9. **After 5 minutes without a good document**, the node's `health` shows `unknown`. Nothing else changes: capabilities stay, windows stay open.
10. **Last frame:** a node window keeps its last frame, marked stale, and is never paused or closed because of this (SOURCE `HUB-OS.md`, session layer: "An unreachable machine keeps its last frame, marked stale"). That is the session layer's rule; the document only supplies the "unknown" health text.
11. **Time:** the hub stamps each document with **its own receipt time**. The node's clock is never used for staleness. `generation` is only a change counter: it goes up whenever any value except `uptime_s`, `load_1m`, `mem_available_kb`, `disk_free_kb` changes; if it goes **down**, the helper or the node restarted and the new document is simply accepted.
12. **No key, no signature, no secret** in this document. A key named like a password or token is a bad document (same pattern idea as `distro-workshop.md` 6.5 rule 9).
13. **Size and rate:** at most 64 KiB (placeholder); one outstanding poll per node; a node that answers slower than the deadline (placeholder 5 s) counts as failed.

## 6. Failure table
| What happens | What the hub does | Label |
|---|---|---|
| Node down or no answer | Poll failed; staleness timer runs; `hubd` already shows "down" from its own check | BELIEVED |
| Answer is not valid TOML | Bad document: keep last good, one log line per change | BELIEVED |
| Unknown `schema` | Only the four required sections are used; one log line per node per change | owner rule |
| Unknown section or key | Ignored | owner rule |
| Optional section missing | Feature off, quiet | owner rule |
| Required section missing, known schema | Bad document (rule 4) | BELIEVED |
| Capability vanishes for one poll only | Still present (rule 7) | owner rule |
| `node_name` differs from the inventory id | Bad document; the inventory wins; one log line | BELIEVED |
| `generation` goes down | Accept the document (restart) | BELIEVED |
| Node clock wrong | No effect (hub receipt time) | owner rule |
| 5 minutes without a good document | `health` = `unknown`, nothing else | owner rule |
| Document larger than the limit | Bad document | BELIEVED |
| Node answers in recovery | No capability document; use `/v1/status` | SOURCE `node-helper-api.md` 2 |

## 7. Tests (to write when built; none exists)
1. Valid document with every section: all features on.
2. Only the four required sections: optional features off, no log lines.
3. Unknown schema string: only required sections used, exactly one log line, a second poll adds none, a changed string adds one.
4. Unknown section and unknown key: ignored.
5. Optional section removed for one poll: still on; removed for two polls: off.
6. Poll failures only: capabilities unchanged, `health` = `unknown` after 5 minutes (fake clock).
7. Malformed TOML and a missing required section: last good kept, one log line.
8. `generation` goes backwards: new document accepted.
9. Node clock one day wrong: no effect.
10. 20 fake nodes, the polls spread over 30 s (no more than a few per second).
11. Oversized document refused.
12. A document with a key named `password`: refused.

## 8. Five assumptions I am least sure of
1. That `hubd` is the right poller (BELIEVED; depends on whether the conductor will own node state).
2. That 30 s is fast enough for every feature (UNKNOWN; a node entering `critical` or a trial boot may deserve faster).
3. That two TOML/JSON formats on the helper are acceptable (BELIEVED; the owner chose TOML).
4. That an old reader is safe with a newer schema that keeps the four required sections (UNKNOWN).
5. That the TOML parser the repo will use (a third-party library is approved but not chosen: SOURCE `docs/inventory-format.md`) handles hostile input safely (UNKNOWN; fuzz it).

## 9. Decided by the owner (2026-10-10)

- The poll interval stays **30 seconds** (the 30 s question is answered: not too slow).
- **One global staleness policy**, not one per capability.
- **Polling only**: no push requests from nodes.
- A **"force capability present" switch exists in the debug image only**.

## 10. Questions for the owner (still open)

1. Is it TOML or JSON for this document (the helper's other calls are JSON)?
2. Should `/v1/status` and this document be merged so there is one source for the overlapping fields?
3. Should a new schema version be allowed to change the meaning of the four required sections?
