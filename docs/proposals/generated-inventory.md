# PROPOSAL: generate the inventory from one small file per machine, and reload it without restarting hubd

**Status: PROPOSAL. Nothing here is decided, built or tested.** `HUB-OS.md` wins if this file disagrees with it. Written 2026-10-10 by the lead agent (no helpers). No code. Labels: TESTED, SOURCE, BELIEVED, UNKNOWN. **Example names, addresses and ports are placeholders** (addresses use the documentation range 192.0.2.x, as `docs/inventory-format.md` does).

## 1. Purpose
Today the owner hand-edits one `inventory.toml` (SOURCE `docs/inventory-format.md`). The proposal: each machine has its own small **machine file**; a pure function generates `inventory.toml` from all of them; `hubd` picks up a new inventory **without restarting** and keeps the old one if the new one is bad. Roles are dropped in favour of a free-text `group`.

## 2. The machine file (the owner's fields)
One TOML file per machine in `machine_files/`:
```toml
name     = "ai-1"             # unique; the permanent label
kind     = "node"             # "hub" or "node"
address  = "192.0.2.21:8480"  # "hub address", see 3.1; placeholder port
guest_of = ""                 # empty, or the name of the host machine
group    = "generative"       # free text, display only
```
`address` is present for every non-hub machine and absent or empty for the hub.

## 3. Differences from the existing inventory format (every one I found)
SOURCE `docs/inventory-format.md`, read 2026-10-10.
| Existing inventory | This proposal | Difference |
|---|---|---|
| One file `inventory.toml` with `format = 1` and `[[machine]]` blocks | One file per machine; the generator writes the one inventory | Different authoring; the **generated** file keeps `format = 1` and the same blocks |
| `id` (permanent, `^[a-z0-9][a-z0-9-]*$`, unique) and `name` (friendly name shown in the panel) | Only `name` (unique) | Two fields become one. Proposal: the machine file `name` is the id and must follow the id pattern; a friendly display name needs another field (question 5) |
| `role` required: `hub`, `gaming`, `ai`, `desktop`, `nas`, `backup-nas`, `vm-host`, `guest` | `kind` (`hub`/`node`) and `group` (free text) | Roles are dropped. `HUB-OS.md` line 110 says "the inventory gives behaviour to hub, guest and vm-host and to nothing else": `hub` is now `kind`, `guest` is now "has a `guest_of`", **`vm-host` has no replacement** (the rule "`host` must point at a machine whose role is `vm-host`" can no longer be checked) |
| `address` (where to reach the machine, no port) | `address` with a port | The owner's field "hub address" is ambiguous. **I read it as "the address by which the hub reaches this machine"**, with a port (placeholder). The existing format keeps the port separate (`port`, optional) |
| `open` required: list of `moonlight`, `spice`, `vnc`, `ssh`, `files`, `none` | not present | **Missing.** `hubd` refuses an inventory without `open`. The generator cannot produce a valid entry from the owner's five fields alone |
| `home` required `{ x, y }`; two machines may not share a position | not present | **Missing.** Same problem; the uniqueness rule cannot be checked |
| `port`, `user`, `share`, `session` optional | not present | Optional, so no validation failure, but they would be lost |
| `host` + `lifetime` required on guests | `guest_of` only | `lifetime` (`ephemeral`/`persistent`) has no place and would be lost |
| Unknown field is an error | Same rule proposed | Same |
| Secrets never in the file | Same | Same |
**So the five fields are not enough to generate a valid inventory.** Options: (a) the machine file carries the existing optional/required fields too (`open`, `home`, `port`, `user`, `share`, `session`, `lifetime`) with the same names and rules; (b) a second source (a "layout of homes" file) supplies `home`; (c) defaults generated for `open` and `home` (positions need a placement rule). This is question 6; I do not choose.

### 3.1 The migration script
Turns an old role-based `inventory.toml` into machine files: `kind = "hub"` if role is `hub`, else `"node"`; `group` = the old role text (for `guest`, the host's group or `"guest"`); `guest_of` = old `host`; `address` = old `address` (+ `:port` if `port` was set); every other old field copied unchanged if option (a) is chosen, otherwise reported as "dropped". It prints what it dropped. It is read-only on the old file. (BELIEVED to be small; whether to keep it after migrating is question 4.)

## 4. Validation (on the machine files, before generating)
1. Names unique (and match the id pattern).
2. **Exactly one** machine with `kind = "hub"`.
3. `guest_of`, when set, names an existing machine; it must not name the machine itself.
4. **No cycles** in `guest_of` (this also rules out chains that loop).
5. `address` present for every non-hub machine.
6. Unknown field is an error; wrong type is an error with the line number; no control characters; texts that start with a dash are refused (as in the existing rules).
7. No secret-looking key (same idea as `distro-workshop.md` 6.5 rule 9).

## 5. Generation
A **pure function** `generate(machine_files/*.toml) -> inventory.toml`: no clock, no network, the same files give the same bytes (sorted by name). It runs on the hub's tools or the workshop, not in `hubd`. The generated file starts with a comment "generated; do not edit" and a hash of its inputs.

## 6. Reload without restarting `hubd`
**Today `hubd` has no reload** (SOURCE: searched `cmd/` and `internal/` for reload/SIGHUP on 2026-10-10: nothing; the inventory is read at start). Proposal: a command `hubd reload` (and the same request on the socket): **parse, validate, diff, apply**.
- **Parse and validate** with the same code as start-up. On **any** failure: keep the old inventory, log **one line** (the first error with its line number), exit non-zero for the command.
- **Diff:** machines added, removed, changed (address, group, guest_of, open, home...).
- **Apply:** added machines appear in the next check round and the list; removed machines leave the list; **an open window of a removed machine is left alone** (SOURCE `HUB-OS.md`: "If a machine drops while its window is open, leave the window alone"; the owner's rule is for drops, extended here by BELIEVED); a changed address is used from the next round; a renamed machine is a removal plus an addition (the id is permanent: SOURCE `inventory-format.md`).
- **STALE and saved layouts:** a layout that mentions a removed machine keeps the entry; applying it skips that machine with a warning (BELIEVED; question 2 of `open-everything-in-layout.md` is related).
- **Atomic:** the new inventory replaces the old one in one step; a reader never sees half.

## 7. Failure table
| What happens | Result |
|---|---|
| Duplicate name | Generation fails, names both files |
| Two hubs / no hub | Generation fails |
| `guest_of` names a missing machine | Generation fails |
| Guest cycle | Generation fails, lists the cycle |
| Non-hub machine without `address` | Generation fails |
| Generated file fails the existing `hubd` rules (for example no `open`) | Generation fails with that rule (so a bad generator is caught before `hubd` sees it) |
| Reload with a malformed file | Old inventory kept, one log line, command exits non-zero |
| Reload while a round is running | The round finishes with the old inventory; the next uses the new |
| Reload twice quickly | The second waits for the first; the last one wins |
| Machine file edited while generating | Generation reads a snapshot (copy first); the result names the hash |
| Migration meets an unknown old field | Reports it, writes nothing for that machine, exits non-zero |

## 8. Tests (the eight the owner listed, plus checks)
1. One hub plus three nodes: generates a valid inventory that `hubd` accepts.
2. Duplicate name: refused.
3. Two hubs: refused.
4. Missing host (`guest_of` names nobody): refused.
5. Guest cycle: refused.
6. Reload with an added node: it appears in the next round, no restart, windows untouched.
7. Reload with a malformed file: old inventory kept, one log line.
8. Migration from an old role-based inventory: round trip (migrate, generate) gives an inventory `hubd` accepts and prints what it dropped.
Extra checks: the generated bytes are identical on two runs; the generated file passes every existing test in `internal/inventory` (SOURCE: `testdata/broken/` has one broken file per rule).

## 9. Five assumptions I am least sure of
1. That "hub address" means the address by which the hub reaches the machine (UNKNOWN: ambiguous).
2. That dropping `vm-host` as a role loses nothing `hubd` needs (UNKNOWN: panel nesting of guests under a host uses `host`; the vm-host behaviour is not specified beyond that).
3. That `group` truly never affects display or routing (question 3).
4. That reload can be made atomic against a check round in progress without a lock that could stall the bar (BELIEVED).
5. That `home` and `open` can be defaulted or moved elsewhere without hurting the layouts (UNKNOWN).

## 10. Questions for the owner
1. Are guest chains allowed (a guest of a guest), or direct guests only?
2. A dry-run reload command that only reports the diff?
3. Should `group` ever affect routing or display?
4. Keep the migration script after migrating?
5. Is a separate friendly name needed, or is the permanent `name` also what the panel shows?
6. How should `open`, `home`, `port`, `user`, `share`, `session` and `lifetime` be supplied (section 3, options a to c)?
