# hubd second slice

**Section 11 (second review) supersedes earlier statements about STALE, the check cap and single click.** **Built and measured: 2026-10-01 — build environment; will change. Includes the follow-ups the owner decided after the first review (section 10).** `HUB-OS.md` wins if anything here disagrees with it. This file describes what the second slice of `hubd` does, how it was tested, what was measured at 100 and at 5000 machines, and what is still unverified. It builds on `docs/inventory-format.md`, `docs/driftwm-findings.md` and `docs/bar-findings.md`.

Everything here ran in the cloud build environment (`docs/environment.md`): a nested driftwm (software rendering, on a virtual X display), Waybar 0.9.24, wofi 1.4.1, `foot` as the **fake viewer**, and fake machines made of tiny listeners on this computer's own addresses. **No real Moonlight, virt-viewer or Remmina was run, no real screen, no real network, no real hardware.**

Labels used: **TESTED** (the command shown was run and gave the result written), **BELIEVED** (from reading, not run), **UNKNOWN**.

---

## 1. What it does

`hubd` is now one background program (`hubd serve`) that holds the state, plus small commands that talk to it over a socket.

| Command | What it does |
|---|---|
| `hubd serve` | The background program. Checks machines in rounds, follows driftwm, answers the other commands. |
| `hubd feed` | Prints one JSON line for Waybar's custom module (`text`, `class`, `tooltip`) now and again each time it changes. If `hubd serve` is not running it prints an alert line and keeps trying. |
| `hubd list [--flat] [--filter TEXT]` | Prints the menu lines (section 4). |
| `hubd menu` | The bar item's `on-click`. Shows the list in wofi, acts on the pick, and shows the list again at once when the pick was a group heading or the search. A second click while a menu is open does nothing. |
| `hubd pick LINE` | Acts on one picked line (section 4). Anything that is not a machine line, a group heading or a switch line is ignored. |
| `hubd open ID` | Starts the machine's viewer, or goes to its window if one is open. Refuses a machine that is down and says so. |
| `hubd end ID` | Closes only the local window. Never touches the machine or its session. |
| `hubd forget ID` | Drops the "window not identified" block of a machine (section 3). Closes nothing, moves nothing, does not touch the machine. |
| `hubd check` (or `hubd` alone) | Slice 1: reads the inventory, checks every machine once, prints the table. Since the follow-up it checks with the same bounded, file-limit-aware probing as the daemon (section 10); the table, wording and exit codes are the same. |

`hubd menu` also takes `--height PIXELS` (340: the search box and about 12 rows; **hubd never passes `--lines` to wofi**, see the note at the end of this section), `--wofi PROGRAM`, `--style FILE` (wofi style, used only if the file exists; default `/etc/hubos/wofi.css`), `--width PIXELS` (720), `--single-click` (the default: one click picks) and `--no-single-click` (pick with a double click, like wofi's own default).

Common flags: `--socket PATH` (default `$XDG_RUNTIME_DIR/hubos/hubd.sock`). `serve` also takes: `--inventory PATH` (default `/etc/hubos/inventory.toml`), `--viewers PATH` (default: `viewers.toml` in the inventory's folder), `--driftwm-socket PATH`, `--bar-height PIXELS`, `--probe-cap N` (0 = automatic, see section 11), `--probe-interval D`, `--probe-timeout D`, `--window-wait D`, `--fold N`, `--list-max N`, `--down-max N`, `--tooltip-cap N`, `--message-ttl D`, `--log-rounds`.

Exit codes of `open` and `end`: 0 when it did what was asked (`open` also counts "went to the existing window"), 1 otherwise, with the reason printed in plain words.

### Rules that were followed

- **Viewers are started with an argument list, never through a shell.** Text from the inventory is put into exactly one argument and is never pasted into a command string. TESTED by `TestArgsOneArgumentEach` and `TestInventoryTextStaysOneArgument` (a name full of quotes, `;`, `$(…)` and `{port}` stays one argument and is not substituted a second time).
- **The first entry of `open` decides what a click opens** (`m.Open[0]`).
- **No default ports.** A machine with no port is "not checked". Opening it works only if its viewer command needs no `{port}`; otherwise `hubd` says so and does not guess.
- **No real viewer command lines.** `examples/viewers.example.toml` has the fake viewer only.
- **hubd and driftwm run as the same user.** Both sockets are mode 0600, in a 0700 folder.
- **Windows are never reopened after a hub restart.** The record is in a memory-backed folder (gone at reboot) and `hubd` only adopts windows that still exist.
- **A started viewer's output goes to a size-capped log** (section 2.5).
- **The hub keeps no windows of its own.** wofi and Waybar are screen-edge surfaces, not windows.

---

## 2. File formats

### 2.1 `viewers.toml` (next to the inventory; never secret; never in git except the example)

```
format = 1

[default_ports]                                                  # optional table, before the first [[viewer]]: program = port (whole numbers 1 to 65535)
ssh = 22                                                         # used for {port} and for the up/down check when a machine has no port of its own

[[viewer]]
id        = "fake"
programs  = ["moonlight", "spice", "vnc", "ssh", "files"]      # inventory "open" entries this viewer serves
command   = ["foot", "--app-id={app_id}", "--title={title}", "--", "sleep", "infinity"]
sets_name = true                                                # the command makes the window carry {app_id}
window_wait = "10s"                                              # optional: how long to wait for this viewer's window (default: the hub's 10 s)
late_grace  = "60s"                                              # optional: how long to keep waiting after that (default: the hub's 60 s)
title_match  = "{id} - Moonlight"                                # optional, only with sets_name = false: find the window by this exact title (section 14)
```

- `command` is a list: the first entry is the program (fixed text, no placeholders); each other entry is one argument.
- Placeholders: `{id} {name} {address} {port} {user} {share} {session}` from the inventory, `{app_id}` (= `hubos-<id>`), `{title}` (= the machine's name). `{port}` is the machine's own port, otherwise `default_ports[the machine's first open entry]`. A placeholder for a value the machine lacks (no port and no `default_ports` entry, no `user`, `share` or `session`) makes that machine unopenable with a message (`<id> has no session in the inventory, and its viewer command needs one`); it is never guessed. Substituted text is never scanned again.
- Checked at start-up: unknown fields, unknown placeholders (in `command` and in `title_match`), empty `programs`, `none` as a program, one program served by two viewers, `sets_name = true` without `{app_id}`, `title_match` together with `sets_name = true`, and in `[default_ports]` a key that is not a program (`none` too) or a value that is not a whole number from 1 to 65535 (plain-words messages such as `default_ports: the port for "ssh" must be a whole number from 1 to 65535 (found: 0)`). The per-viewer `default_port` of an earlier draft does not exist: it is an unknown-field error. A bad file stops `hubd serve` before anything starts.
- `sets_name = true` means: find the window by its name `hubos-<id>`. `false` means: find it by comparing the window list before and after the launch (section 3).

Test viewers (in the repo, test only): `testdata/viewers/ambiguous.toml` (opens two windows at once), `testdata/viewers/ignores-name.toml` (ignores the name), `testdata/viewers/chatty.toml` (prints to standard output and error first). Example menu style: `examples/wofi.style.css`.

### 2.2 The record file

`$XDG_RUNTIME_DIR/hubos/hubd.record.json` (next to the socket; mode 0600; no secrets):

```
{"driftwm":"2032245:1790894787.45811432","windows":[{"machine":"nas-1","window":1,"app_id":"hubos-nas-1","title":"Storage","by":"name"}]}
```

`driftwm` is the identity of the driftwm it belongs to: the inode and change time of driftwm's socket file, which is made anew each time driftwm starts. **BELIEVED** to differ between runs; TESTED that it differed after one restart (`2032178:…` before, `2032245:…` after). `by` is `name` or `comparison`.

### 2.3 The socket

One JSON request per line, one JSON reply per line (`internal/hub/ipc.go`). `feed` is answered with an endless stream of status lines, each exactly what Waybar reads.

### 2.4 Status line (what Waybar reads)

```
{"text":"4750 of 5000 up","class":"alert","tooltip":"250 down: ai 119, …, ai 59 and 240 more"}
```

- `text`: `N of M up`. The hub and machines that are not checked are in neither number. **While the first round runs: `checking... N of M done`** (no partial up and down numbers). When the status is stale: `STALE: N of M up`.
- `class`: `ok`, or `alert` when a machine is down, when a message is showing, or when the status is STALE.
- `tooltip`, top to bottom: the latest message (for `message-ttl`), "driftwm is not reachable…", `STALE: no check result for 30s (limit: 3 intervals = 30s)` (the age in 10 s steps, so the tooltip changes at most every 10 s), `first check round in progress` (first round only), `K down: <names>` (at most `tooltip-cap` names, then "and J more"), `N not checked` (or `all machines up`), `last check round took Z`, and, when that round took longer than two intervals, a plain note: `note: that is longer than two intervals (20s); a bigger check cap or interval may fit this many machines better`.
- **Rounds and STALE.** A check round starts one interval after the previous round **finished**. The status is **STALE when no check result has arrived for three intervals** (any result counts, also those inside a long round). A long round alone is only a note, not an alert. The feed looks again every second, so STALE appears even when nothing else changes, and it **sends at most one line per second**, the first at once.
- **Markup.** Waybar 0.9.24 reads the bar text and the tooltip as Pango markup, so `hubd feed` escapes `&`, `<`, `>`, `"` and `'` (section 11). Use the module without Waybar's own `escape` option.

### 2.5 Viewer logs

Each started viewer's standard output and error go to `$XDG_RUNTIME_DIR/hubos/hubd.viewer-logs/viewer-<id>.log` (folder mode 0700, files 0600; the runtime folder is memory-backed, so logs are gone at reboot). The viewer writes to the file directly, not through hubd, so a viewer keeps running — and logging — if hubd is killed. The first line of each start is `--- <time> started: <program>` (the program name only, never the arguments, which contain inventory text).

**Cap and rotation (128 KiB per log, 64 MiB for all logs together).** When a viewer starts and its log is over the cap, the old log is moved to `viewer-<id>.log.1` (replacing an older one) and a new log starts. While hubd runs it checks every 5 seconds (and at start-up); a log over the cap is copied to `.1` and emptied, and the viewer goes on writing from the start. So a machine keeps at most about 2 x the cap. **Total cap:** when all the logs together are over 64 MiB, the oldest are removed first: rotated copies (`.1`) are deleted, then current logs, oldest first, are emptied but kept (a viewer may still be writing to one, and deleting it would not free the space until the viewer exits). **While hubd is not running nothing trims**, so a chatty viewer can grow its log until hubd runs again. When a viewer exits with an error before showing a window, the message names its log file.

---

## 3. How a window is matched, placed and ended

### State of one machine (the state machine)

```
              open, viewer chosen, not down
   idle ───────────────────────────────────────▶ starting ──┬── one new window ──▶ idle + window recorded ("open")
    ▲                                                        ├── none within window-wait ──▶ idle (message: no window appeared)
    │                                                        ├── viewer exits with an error ─▶ idle (message)
    │  end                                                   └── two or more new windows ──▶ unmatched
    └── (unmatched only) hubd forget ID, or the menu line "x forget unknown window for <name> (<id>)" ──────┘
 open (window recorded) ── window closes (driftwm tells hubd) ──▶ idle
 open ── end ──▶ driftwm closes the window ──▶ idle
 any state ── driftwm goes away ──▶ idle (record cleared)
```

- `open` while **starting**: does nothing, says "already opening" (exit 1). This is the double-click guard. TESTED live (two `hubd open nas-1` at the same moment: one opened it, the other printed `Storage is already opening; nothing done`) and in `TestDoubleClickWhileStartingDoesNothing`.
- `open` while **open**: goes to the window (`focus` pans the view to it and raises it). Never a second window. Allowed even if the machine has since gone down.
- `open` while **unmatched**: refused until it is forgotten. `end` also refuses (it does not know which window to close). `hubd forget ID` (or the menu line `x forget unknown window for <name> (<id>)`) clears the block; it closes nothing, moves nothing and does not touch the machine, so the windows the viewer left behind stay and are closed by hand. TESTED live and in `TestAmbiguousMatchDoesNothing`.
- One launch at a time across the whole hub (a lock), so "the window that is new" is never confused with another launch of ours.

### Matching (design item 2)

1. Snapshot driftwm's window ids. 2. Start the viewer (own process group, no input or output, so it outlives `hubd`). 3. Poll `state` every 100 ms for new window ids until `window-wait`. 4. When the first appears, wait `settle` (500 ms) and look again, to catch a second one.

- `sets_name = true`: the new window must be named `hubos-<id>`. A new window with another name is reported in the failure message ("this viewer may ignore the chosen name; set `sets_name = false`").
- `sets_name = false`: any new window counts.
- `sets_name = false` with `title_match`: only a new window whose title is **exactly** the rendered `title_match` counts (section 14).
- Exactly one candidate → matched. **More than one → nothing is moved, focused, closed or recorded**, the machine becomes `unmatched`, and the message says what appeared. Zero → "no window appeared within …".
- driftwm's `state` has no process id, so matching by process is not possible.

### Placement (design item 4)

After a match: `move` to the machine's `home` (window centre, Y up — `docs/inventory-format.md`), then `focus`. If the window is taller than the viewport minus `--bar-height`, or wider than the viewport, it is shrunk with `resize` and moved again (so a placed window is never under the bar). With a bar height set, `hubd` waits for driftwm's pan animation to stop, reads the camera, and says so in the message if it is not `home y + bar height / 2`. If the home spot is taken by another window, the new one is placed on top (the home is fixed; there is no nudging).

### Ending

`end` asks driftwm to close the window (`close --id`), waits up to 3 s for it to go, then drops the record. If the window stays (the program may be asking something), it says so and forces nothing. It never kills a process and never contacts the machine.

### After restarts (design item 3)

| What restarts | What happens | Test |
|---|---|---|
| `hubd` only | New `hubd` adopts windows: from the record file if it belongs to the same driftwm (the entry must still exist with the same app_id and title), and windows named `hubos-<known id>` if exactly one has that name, and, for a title-matched viewer, a window whose title is exactly the machine's title if exactly one has it (section 14). It starts nothing. | TESTED live: `kill -9`, start again, `foot` count unchanged, list shows `[open]`, `open` goes to the window. Unit: `TestRestartAdoptsAndOpensNothing` |
| driftwm | The stream from driftwm ends; the record is cleared (file too); the bar shows "driftwm is not reachable" until it is back. All viewers die with it (live: `foot` count 0). | TESTED live; unit: `TestDriftwmRestartClearsRecord` |
| The hub | The runtime folder is empty. Nothing is reopened. | BELIEVED (the folder is memory-backed on Linux); not tested by rebooting |

---

## 4. The menu list

```
! text                                 a message; ignored if picked
x forget unknown window for <name> (<id>)   clears the block on that machine (only offered when there is one)
? search by id or name...              ask for text, then show matches (flat, down first)
- Down machines (250)                  every machine that is down, at most 50 lines, then "   ! and 200 more down machines; use the search"
< back to groups                       back from the search
- AI (250 machines, 12 down)           open group; picking it folds it
+ Desktop (500 machines, 25 down)      folded group; picking it opens it, and the menu reopens at once
 ○ ai-19            ai 19                    DOWN         machine line, window closed: empty dot, then the id
 ● ai-20            ai 20                    UP           machine line, window open: filled dot, then the id
   - Guests of vmhost-1 (50 machines)       nested under the host (3 more spaces)
    ● guest-1          Guest 1                  UP
```

(Before section 17 the machine lines had no dot, three spaces of indent, and ended with ` [open]` when a window was open: `   guest-1          Guest 1                  UP [open]`. Section 17 has the change.)

- A **Down machines** group is at the top (cap `--down-max`, 50, then a line saying how many more); it can be folded like any group. Groups by role (Hub, Gaming, AI, Desktop, NAS, Backup NAS, VM host) follow; guests nested under their host; **down first** inside a group, then not checked, then up.
- **Nothing is hidden without a line that says so**: a folded group shows its heading with the count and how many are down; a capped list ends with a `! N more …` line.
- **Typed search and the 1000-line cap change the approved design.** The design said wofi's own search would find any machine. It cannot: wofi only searches the lines it is given, and it cannot be given thousands of lines (section 7.5). So `? search by id or name...` asks for text, hubd filters (case-insensitive, id or name), and wofi shows the matches. This was approved by the owner.
- A group with more than `--fold` machines starts folded. Fold choices are kept in `hubd`'s memory (gone when it restarts).
- No list has more than `--list-max` machine lines; the rest are replaced by a `! N more … not shown` line.
- **Pick** (`hubd pick LINE`): acts only if the line is exactly a switch line, a group heading, or an **indented** machine line whose first word (after the open marker, if there is one) is an existing id. Anything else — a heading with an unknown name, an unknown id, an id without indent, a `!` line, an empty line — does nothing. TESTED: `TestPickIgnoresHeadingsUnknownAndEmptyLines` (a heading, an unknown line, an empty line, and six more).
- Status texts: `UP`, `DOWN`, `checking...`, `NOT CHECKED (no port in the inventory and no default port for <program>)`, `NOT CHECKED (nothing to open)`, `NOT CHECKED (out of file handles)`, `THIS HUB`, plus ` [opening]`, ` [waiting for its window]`, ` [window not identified]`. A window that is open is shown by the filled dot at the start of the line, not by a text (section 17; the old ` [open]` text is gone).
- wofi is always started with `--cache-file /dev/null` (otherwise it re-orders by past picks) and `LC_ALL=C.UTF-8`. With `examples/wofi.style.css` as `/etc/hubos/wofi.css` the list uses a fixed-width font (DejaVu Sans Mono) so the columns line up (TESTED, screenshot: id, name and status columns aligned).

---

## 5. What was run (live, with the output)

Set-up (same method as `docs/driftwm-findings.md` section 0 and `docs/bar-findings.md` section 0, nothing installed, everything under `/tmp/hs2` and `/tmp/dwx`): driftwm built at the pinned commit `352333a8…`; Xvfb 1280x800; `dbus-daemon --session`; Waybar 0.9.24 with `"exec": "hubd feed"`, `"return-type": "json"`, `"on-click": "hubd menu"` and a 30 px bar; wofi 1.4.1.

| What | Commands | Result |
|---|---|---|
| Feed changes when a fake node stops and starts | `hubd feed` in the background; `pkill -f "fakenode 127.0.0.14:21004"`; later start it again | `22:44:50.6 {"text":"6 of 6 up","class":"ok","tooltip":"1 not checked"}` → stop at 51.6 → `22:44:52.5 {"text":"5 of 6 up","class":"alert","tooltip":"1 down: Storage\n1 not checked"}` → start at 56.6 → `22:44:58.5 {"text":"6 of 6 up","class":"ok",…}`. Round interval was 2 s. |
| Bar and list | screenshots | Green box "6 of 6 up" at the left end of the bar, clock at the right. A click opens wofi right under the bar: a search box, then `? search by id or name...` highlighted, `- Hub (1 machine)`, `hub  Desk Hub  THIS HUB`, `- Gaming (1 machine)`, … 12 rows visible, a scrollbar when there are more. A second click on the bar item while it was open started no second wofi. |
| Open on an up machine | typed `ai-1` + Enter in wofi | wofi closed; a terminal window titled "AI Box" opened at screen centre; `state`: `hubos-ai-1 pos [2000, -1500]` (its home); camera `[2000, -1485.5]` = home y + 15 (half the 30 px bar); list shows `UP [open]`. |
| Open again | `hubd open ai-1` | `went to the open window of AI Box`; still one window and one `foot` process. |
| Double click | two `hubd open nas-1` at once | one: `opened Storage at home (-2000, 0), matched by name`; the other: `Storage is already opening; nothing done`, exit 1. One window. |
| Open on a down machine | stop the vmhost fake node, wait a round, `hubd open vmhost-1` | `VM Host is down, not opened`, exit 1, no viewer started. Bar turns red "5 of 6 up"; hovering shows `VM Host is down, not opened / 1 down: VM Host / 1 not checked`; the next menu's first line is `! VM Host is down, not opened`. |
| End | `hubd end ai-1` | `closed the local window of AI Box; the machine and its session were not touched`. The nas-1 window stayed; the ai-1 fake node still accepted connections. Ending twice: `hubd has no window open for AI Box`, exit 1. |
| `hubd` killed and restarted | `pkill -9`, start again | log shows it serving; `foot` count unchanged (1); list: `nas-1 … UP [open]`; `hubd open nas-1` → `went to the open window of Storage`. |
| driftwm restarted | `pkill -x driftwm`, start it again | Windows and Waybar died with it. While away: status `driftwm is not reachable: windows cannot be opened`, `hubd open ai-1` → `driftwm is not reachable, so no window can be opened`. Record file: windows `null`. After restart: list shows no `[open]`, new record has the new driftwm identity, `open` works again. |
| Ambiguous match | `--viewers testdata/viewers/ambiguous.toml`, `hubd open scratch-os` | `2 new windows appeared while opening Scratch OS (test guest) (#2 "two-a", #1 "two-b"); I cannot tell which one is its window, so I left them all alone and placed and recorded nothing`. Both windows stayed where driftwm put them (a screenshot shows "Two A" over "Two B"). Second try refused with `…close it by hand, then run: hubd forget scratch-os` (the first version said `hubd end`; see section 10). |
| A viewer that ignores the name | `--viewers testdata/viewers/ignores-name.toml`, another window already open, `hubd open backup-nas-1` | `opened Backup Storage at home (-2000, 1500), matched by comparison`; the other window untouched; after `kill -9` and restart the window was adopted from the record file. |
| Long runtime folder | `XDG_RUNTIME_DIR=/tmp/a-very-long-…/xxxx…` (112 bytes) `hubd serve` and `hubd list` | `hubd: the socket path is 128 bytes long (…); Linux allows at most 107. Use a shorter XDG_RUNTIME_DIR (for example /run/user/1000), or give --socket PATH`, exit 1, nothing started. With a short `--socket` it serves, and warns that driftwm's own socket path (139 bytes) is too long: driftwm itself fails there (`docs/driftwm-findings.md` section 0). |
| Mouse in wofi | one click, then a double click on a machine row | One click only highlights the row. A **double click** picks it (the viewer opened). |
| wofi single click (follow-up) | `wofi --dmenu … -D single_click=true`, one click with `xdotool mousemove 100 80; xdotool click 1` on the third of three lines | **wofi 1.4.1 has it**, but as a config option, not a command-line flag (`wofi --help` has no such flag; `man wofi` lists `single_click=BOOL`, "Default is false"). Default run: the row is highlighted, nothing printed, wofi stays open. With `-D single_click=true`: wofi printed `gamma three` and closed. `hubd menu --single-click` passes it; default off (owner to decide). |
| Fixed-width font (follow-up) | `hubd menu --style examples/wofi.style.css` (DejaVu Sans Mono 14 px), screenshot | The id, name and status columns line up; the `- Down machines (2)` group is the first group, with `vmhost-1` and `scratch-os`, both DOWN. |
| Down machines group (follow-up) | stop two fake nodes | Bar red "4 of 6 up". Menu: search line, `- Down machines (2)` with both machines, then the role groups (the same two machines appear again in VM host). |
| Viewer log (follow-up) | `--viewers testdata/viewers/chatty.toml`, `hubd open ai-1`, `kill -9` hubd | `ls -la`: folder `drwx------`, `viewer-ai-1.log` `-rw-------`. Log: `--- 2026-10-01T23:11:17Z started: sh`, `viewer says hello`, `viewer complains` (standard error), and foot's own warning. After `kill -9` of hubd the viewer was still running (foot count 1). A viewer that exits at once: `sh exited (exit status 3) before showing a window for AI Box; its output is in /tmp/dwx/hubos/hubd.viewer-logs/viewer-ai-1.log`. |
| Forget (follow-up) | `--viewers testdata/viewers/ambiguous.toml`: open (two windows), open again, `end`, `pick "x forget …(no-such)"`, `forget` | Second open: `…close it by hand, then run: hubd forget scratch-os`. `end`: `hubd does not know which window is the one of Scratch OS (test guest), so it will not close any…`, exit 1. Menu shows `x forget unknown window for Scratch OS (test guest) (scratch-os)`. The pick with an unknown id: `there is no machine "no-such"`. `forget`: `forgot the unidentified window …; no window was closed or moved and the machine was not contacted`; foot count still 2. Forgetting again: `nothing to forget…`, exit 1. |

---

## 6. Tests

Run from the repo root:

```
gofmt -l .
go vet ./...
go test ./...
go test -race ./...                                           # the same with the race detector
HUBOS_DRIFTWM_TEST=1 go test -v -run Real ./internal/driftwm  # needs a running driftwm; skipped otherwise
```

New test files: `internal/hub/hub_test.go` (open, again, double click, down, not checked, end, restart adopts, driftwm restart, user closes the window, ambiguous, name ignored, no window, too tall, one-argument safety, one launch at a time), `internal/hub/view_test.go` (groups, nesting, folding, down first, flat list, filter, cap, pick, status line, stale, file-handle cap), `internal/hub/ipc_test.go` (socket, feed only on change, long path, stale socket file), `internal/viewers/viewers_test.go`, `internal/driftwm/driftwm_test.go` (fake socket), `internal/probe/probe_test.go` (cap; out of file handles is never "down"), `internal/inventory/inventory_test.go` (control characters) with broken file 26, `tools/geninv/main_test.go`, and `cmd/hubd/main_test.go` (`hubd check` equals the slice 1 output). Follow-up tests: `TestCheckWith5000MachinesAndSmallFileLimitHasNoFalseDown` (`cmd/hubd`; builds hubd, geninv and fakenode, starts 4750 fake machines, runs `ulimit -n 1024; hubd check` on 5000 generated machines: exactly the 250 machines without a fake node are DOWN, none says "too many open files", summary `4750 of 5000 up`; skipped with `-short`), `TestStaleRuleAt5000MachinesWithSlowAndFastRounds` and `TestRoundStartsOneIntervalAfterTheLastFinished` (fake clock and fake checker, 5000 machines), `TestFeedShowsStaleWhenTimePassesWithNoEvent`, `TestDownMachinesGroupAtTheTopWithCapAndMoreLine`, `TestDownGroupAt5000Machines`, forget in `TestAmbiguousMatchDoesNothing`, `internal/hub/viewerlog_test.go` (output and error in the log, mode 0600/0700, no arguments in the log, rotation at start, trim of a running writer with no hole, failure message names the log), `cmd/hubd/menu_test.go` (the menu loop: list, pick search, ask for text, filtered list, pick; launcher gets `--style`, `--width`, `-D single_click=true` only when asked). **All slice 1 tests still pass.** (The old `TestSlowRoundsAreNotStale` was replaced: the owner's rule is now the rule.)

---

## 7. Scale: 100 machines, and 5000 as the stretch

All inventories are made at run time by `tools/geninv` into `/tmp/hs2` and are never committed. Fake machines are listeners from `tools/fakenode`: each machine has its own address `127.1.x.y` and port 21000 (a fake port; no real port is meant). Every 20th machine is left without a listener, so it is **down**.

```
go build -o /tmp/hs2/hubd ./cmd/hubd; go build -o /tmp/hs2/geninv ./tools/geninv; go build -o /tmp/hs2/fakenode ./tools/fakenode
geninv -n 100  -inventory /tmp/hs2/s100/inventory.toml  -nodes /tmp/hs2/s100/nodes.txt          # 95 machines UP, 5 DOWN
geninv -n 5000 -inventory /tmp/hs2/s5000/inventory.toml -nodes /tmp/hs2/s5000/nodes.txt         # 4750 UP, 250 DOWN
geninv -n 5000 -unreachable -inventory /tmp/hs2/s5000u/inventory.toml -nodes /dev/null          # 192.0.2.x: nothing answers, every check hits its time limit
fakenode $(cat /tmp/hs2/s5000/nodes.txt)                                                        # 4750 listeners in one process
hubd serve --inventory /tmp/hs2/s5000/inventory.toml --probe-cap 200 --probe-interval 5s --log-rounds
```

(`/tmp/hs2/measure.sh`, `sizes.sh`, `wofitest2.py` that took the numbers are small throw-away scripts; their commands are the ones above plus `ls /proc/PID/fd`, `VmRSS`/`VmHWM` from `/proc/PID/status`, and the timings below.)

The sandbox: 4 CPUs, 16 GB, `ulimit -n` 20000. Unreachable addresses really time out here (a plain connect to `192.0.2.55:21000` took the full 3 s limit), so that case is the worst case: every machine off or cut off.

### 7.1 Reading and checking the inventory

`HUBOS_BENCH_INVENTORY=<file> go test -run XXX -bench Parse -benchtime 20x ./internal/inventory`

| Machines | File | Time to read, parse and validate |
|---|---|---|
| 100 | 1,067 lines, 17,490 bytes | **0.64 ms** |
| 5000 | 52,907 lines, 910,217 bytes | **28.6 ms** |

### 7.2 Checking every machine (one "round")

| Case | Cap | Round time | Peak open files | hubd memory (VmRSS) |
|---|---|---|---|---|
| 100, local fakes (95 up, 5 down) | 200 | 5 ms (then 2 ms) | 8 | 8.6 MB |
| 5000, local fakes (4750 up, 250 down) | 200 | 117 ms (then 82 ms) | 119–153 | 18–20 MB |
| 100, none reachable (all hit the 2 s limit) | 200 | 2.003 s | 106 | 9.2 MB |
| 5000, none reachable | 200 | **39.5 s** | 208 | 20.4 MB |
| 5000, none reachable | 500 | **17.9 s** | 508 | 25.9 MB |
| 5000, none reachable | 1000 | **10.0 s** | 1008 | 34.8 MB |

(The 200 case ran faster than the plain arithmetic 5000/200 x 2 s = 50 s; why is **UNKNOWN**. The 1000 case matches it exactly.) Command: `/tmp/hs2/measure.sh DIR CAP INTERVAL ULIMIT ROUNDS` (starts `hubd serve … --log-rounds` in a subshell with `ulimit -n`, samples `/proc/PID/fd` every 10 ms).

Rerun after the follow-up (rounds now start one interval after the last one finished; same commands): 100 machines, local fakes: 7, 8 and 2 ms; 5000, local fakes: 176, 135 and 116 ms, peak open files 168, memory 18.5 MB; 5000 none reachable: cap 1000 → 10.09, 10.04, 10.03 and 10.03 s; cap 200 → 39.7 s and 41.1 s.

How fast a change shows on the bar: with a 2 s interval, a fake node stopped at `:51.6` showed at `:52.5` (0.9 s), and restarted at `:56.6` showed at `:58.5` (1.9 s) — up to one interval plus the round.

### 7.3 Open files, and what happens above the limit

`hubd` reads `ulimit -n` at start-up and prints it. It never lets the cap exceed 80% of the limit (the rest is kept for sockets to Waybar, the menu, driftwm and logs; first version: the limit minus 64). Command: `/tmp/hs2/measure.sh /tmp/hs2/s5000 200 5s <ULIMIT> 2`.

| `ulimit -n` | Check cap used | Round time | Result |
|---|---|---|---|
| 20000 | 200 | 112 ms | 4750 up, 250 down (correct) |
| 1024 (a common default) | 200 | 114 ms | correct |
| 256 | **192** (lowered, and it says so) | 101 ms | correct |
| 100 | **36** (lowered, and it says so) | 75 ms | correct |

If a check still runs out of file handles (another program is using them), the result is "not checked", never "down": TESTED in `TestOutOfFileHandlesIsReportedAsUnchecked` (limit set to 40 inside the test: 78 of 200 checks came back unchecked, **0 wrongly down**).

**Slice 1 `hubd check`, before and after the follow-up.** Before, it started one check per machine at once. At 5000 machines (4750 up), by `ulimit -n`: 20000 → 4750 up (correct); 1024 → **2106 up, 2,784 wrongly DOWN** (`socket: too many open files`); 256 → **3994 up, wrong**. After the follow-up it uses the same bounded probing (cap 200, never above the file limit minus 64) and a check that cannot be made is `NOT CHECKED`, never `DOWN`:

| `ulimit -n` | Command | Result |
|---|---|---|
| 20000 | `( ulimit -n 20000; hubd check --inventory s5000/inventory.toml )` | 4750 of 5000 up, 250 DOWN, 0 "too many open files", 0 NOT CHECKED; 285 ms |
| 1024 | same | **4750 of 5000 up, 250 DOWN, 0 false**; 238 ms |
| 256 | same | **4750 of 5000 up, 250 DOWN, 0 false**; 258 ms |

The automatic test is `TestCheckWith5000MachinesAndSmallFileLimitHasNoFalseDown`. With the daemon (rerun): `ulimit -n` 1024 → cap 200, round 152 ms, correct; 256 → cap 192, 170 ms, correct; 100 → cap 36, 173 ms, correct.

### 7.4 What hubd sends out

| | 100 machines | 5000 machines |
|---|---|---|
| Status line | 135 bytes (was 108; now ends with the last round's length) | 182 bytes (was 153) |
| Tooltip part | 82 bytes (5 names) | 126 bytes (10 names, "and 240 more"); bounded by `tooltip-cap` |
| `hubd list` (grouped, large groups folded, with the Down machines group) | 39 lines, 1,648 bytes, 7 ms | 62 lines, 2,897 bytes, 8 ms (50 down lines, "and 200 more", folded groups with their counts) |
| `hubd list --flat`, no cap | 102 lines, 4,881 bytes, 6 ms | 5,002 lines, 240,571 bytes, 14 ms |
| `hubd list --flat`, capped at `list-max` 1000 | — | 1,002 lines, 48,585 bytes, 7 ms |

### 7.5 wofi

wofi was started on the list, the time until it **stopped using the CPU** was read from `/proc/PID/stat` (a picture that looks "finished" is not proof: with 5000 lines the screen showed only the empty search box for 18 s). Then 30 `Down` keys and a typed search were sent with `xdotool`. Command: `python3 /tmp/hs2/wofitest2.py LISTFILE NAME TEXT`. One run each; resolution about 50–100 ms.

| List given to wofi | Lines | wofi busy at start-up | wofi memory | 30 x Down | Typed search settles |
|---|---|---|---|---|---|
| 100-machine grouped | 33 | 0.15 s | 29 MB | scrolls | 0.05 s |
| 100-machine flat | 102 | 0.15–0.30 s | 29 MB | scrolls | 0.1 s |
| flat, 250 | 251 | 0.30 s | 31 MB | | 0.05 s |
| flat, 500 | 501 | 0.30 s | 33 MB | | 0.1 s |
| flat, 1000 | 1001 | **0.50 s** | 39 MB | | 0.1 s |
| flat, 2000 | 2001 | **1.96 s** | 49 MB | | 0.25 s |
| flat, 5000 | 5001 | **17.8–18.6 s** | 81 MB | 0.5 s | 1.1–1.2 s |
| 5000-machine grouped (folded) | 10 | 0.10 s | 28 MB | | 0.15 s |
| 5000, flat capped at 1000 (what `hubd menu` shows) | 1002 | 0.50 s | | | 0.05 s |

Rerun after the follow-up (with the fixed-width font style, 14 px): the grouped 5000-machine list (62 lines) is ready after 0.30 s and a typed search settles in 0.05 s; the capped flat list (1,002 lines) needs 0.91 s (it was 0.50 s with the default font) and a search settles in 0.10 s.

wofi's start-up grows much faster than the number of lines (1000 → 0.5 s, 2000 → 2 s, 5000 → 18 s), which is why every list is capped. With the cap, the 5000-machine case worked end to end through the bar: the group `AI (250 machines, 12 down)` opened with its 12 down machines first; the search prompt took `guest-2077`; the one match was picked; a window titled "Guest 2077" opened at its home `(8400, 9300)`.

### 7.6 Where 5000 holds, and where it does not

**Holds:** reading the inventory (29 ms); a check round with machines that answer or refuse (about 0.1 s); memory (20–35 MB); the status line and tooltip (a few hundred bytes); the bar alert; opening a machine; the folded menu and the typed search; staying inside a small `ulimit -n`.

**Holds only with the right cap:** a round when many machines are off and silent. At cap 200 a full round takes about 40 s, so a newly-down machine can take that long to show. Cap 1000 brings it to 10 s but needs `ulimit -n` of at least 1100. 

**Does not hold:** wofi with thousands of lines (18 s), so no list may be longer than about 1000 lines.

**STALE at 5000 with every machine silent (tested live, section 10):** with cap 200 a round takes about 40 s, a new round starts 10 s after one finishes, so the last finished round is up to 50 s old and the status shows `STALE` for about 20 s of every 50. With cap 1000 (10 s rounds) it never does. That is the owner's rule working as written, not a bug; the cap or the interval has to fit the machine count.

**Not measured:** 5000 real windows (only a handful of fake windows were opened); the load of 5000 viewers; a real network; release builds.

---

## 8. The numbers

The ten numbers proposed in the first review were **approved by the owner as proposed**. All are options of `hubd serve` and can be changed without code. Added by the follow-up: Down machines group cap **50** (owner's number, `--down-max`), viewer log cap **128 KiB** per log with one rotated copy (proposed, section 2.5), STALE after **3 intervals** (owner's rule).

| Setting | Proposed | Why (from the measurements above) |
|---|---|---|
| Check cap (`--probe-cap`) | **200; 1000 above 1000 machines; at most 80% of `ulimit -n`** (second review) | 100 machines: a worst-case round (all silent) is 2.0 s. At 5000 it is 40 s (cap 200), 18 s (500), 10 s (1000); open files are cap + 8; fine under the usual 1024. For a cluster beyond ~1000 machines use 1000 and raise `ulimit -n` to 2048. |
| Check interval (`--probe-interval`) | **10 s** | A change shows within one interval plus the round (0.9–1.9 s seen with 2 s). 5000 machines at 10 s is 500 connects a second, which the sandbox handled in 0.1 s per round. |
| Per-machine limit (`--check-timeout`) | **2 s** (unchanged from slice 1) | Not measured on a real network (still a guess; HUB-OS.md lists it). |
| Window wait (`--window-wait`) | **10 s** | The fake viewer's window appeared in well under a second; opening took about 0.8 s end to end (0.5 s is the settle wait). Real viewers are unknown, so this is generous. |
| Settle (after the first window) | **500 ms** | To catch a second window of the same launch. Fixed in code (`Settings.Settle`), not a flag. |
| Fold threshold (`--fold`) | **12** | wofi shows 12 rows without scrolling; a group of 12 or fewer is never folded. |
| List maximum (`--list-max`) | **1000** | wofi starts in 0.5 s at 1000 lines, 2 s at 2000, 18 s at 5000. |
| Tooltip cap (`--tooltip-cap`) | **10** | Keeps the tooltip near 100–200 bytes at any size. |
| Message time (`--message-ttl`) | **15 s** | How long a refused-open message stays on the bar item and at the top of the menu. A guess. |
| Close wait | **3 s** (fixed in code) | The fake window closed at once. A guess for real viewers. |
| Stale after | **3 intervals** without any check result (owner's rule, changed in the second review) | Section 11. |

---

## 9. Unverified

- **Real viewers.** The command lines, window names (app_id and title), whether they accept a chosen name, how they behave on a close request, and whether closing one leaves the Sunshine session alive: mostly UNKNOWN. Only `foot` was used with hubd. `docs/viewers-research.md` has what was read and tested about each viewer; `examples/viewers.real.example.toml` has the command shapes the owner chose, all UNVERIFIED on hardware. `examples/viewers.example.toml` stays the fake viewer (used by tests).
- **Matching by comparison with real viewers** (a viewer that hands over to an already running copy and shows its window late, or shows a splash first). The timing is a guess (`window-wait`); the two-window case is refused, not guessed.
- **Window ids.** Whether driftwm re-uses a window id after it closes is UNKNOWN. The record is checked against the window's name and title to guard against it.
- **The driftwm identity** (socket inode and change time) as a way to tell driftwm runs apart: TESTED once with a restart; BELIEVED beyond that.
- **Camera offset by half the bar height**: TESTED for a bar at the top (camera y = home y + 15 for a 30 px bar). Other edges and other bar heights: UNKNOWN.
- **Home convention** (window centre, Y up): read from driftwm's own documents and checked by `move X Y` then `state` in the nested window. Not tested on a real screen (`docs/inventory-format.md`).
- **Waybar as a non-root user under s6 or dinit**, and `hubd serve` under them (including setting `ulimit -n` and `--bar-height`): not done. Waybar and every viewer die when driftwm restarts; something has to start Waybar again (it did not come back by itself).
- **wofi on the projector**: readability. The fixed-width font style is tested only in the nested window. The style file is optional; without it the columns do not line up.
- **Viewer logs**: only `sh` and `foot` were run, so the log of a real Moonlight/virt-viewer/Remmina (size, content, secrets in it) is UNKNOWN. Nothing trims while hubd is not running.
- **STALE while a long first round runs**: before any round has finished the text is `checking...`, then partial counts appear as answers arrive (for example `0 of 3034 up` growing to `0 of 5000 up`); nothing marks those counts as partial.
- **Mouse in wofi**: a double click picks, a single click only highlights unless `-D single_click=true` is given (`hubd menu` has it on by default since section 11; `--no-single-click` turns it off).
- **A real network**: the 2 s limit, the cost of 5000 connects every 10 s to real machines, and whether a bare connect disturbs a real Sunshine/SPICE/VNC (already on the list in `HUB-OS.md`).
- **5000 real windows**, a release build of driftwm, GPU rendering.
- **Several vm-hosts, several hubs of windows, two monitors.**
- **Reboot behaviour** of the record folder: BELIEVED, not tested.
- **Why the 5000 case at cap 200 took 39.5 s instead of 50 s.**

---

## 10. Follow-ups after the first review

Decided by the owner and built on the same branch, as new commits:

1. **Typed search and the 1000-line cap**: documented in sections 4 and 7.5. They change the approved design "wofi search over everything".
2. **`hubd check` is bounded and file-limit aware** (section 7.3). A check that cannot be made is NOT CHECKED, never DOWN. Test: 5000 generated machines, `ulimit -n 1024`, zero false DOWN.
3. **Down machines group** at the top of the list (cap 50, then "and N more"); other groups stay folded above the fold threshold; nothing hidden without a visible line.
4. **Fixed-width font** through a style file, and the **single-click** result (section 5): wofi 1.4.1 has it as `-D single_click=true`.
5. **`hubd forget ID`** and the menu line `x forget unknown window for <name> (<id>)`; `end` refuses an unidentified machine.
6. **Refused-open message**: kept as built.
7. **Viewer logs** (section 2.5).
8. **Rounds and STALE**: a round starts one interval after the previous finished; STALE after three intervals; the tooltip shows the last round's length. Tested with the 5000 case: unit tests with a fake clock (`TestStaleRuleAt5000MachinesWithSlowAndFastRounds`) and live:

| 5000 machines, none reachable, interval 10 s | Round | Feed |
|---|---|---|
| cap 1000 | 10.0 s | rounds end every 20 s; never STALE; the tooltip ends `last check round took 10.087s` |
| cap 200 | 39.7 s, then 41.1 s | `t+40s … last check round took 40.141s`; `t+70s STALE: 0 of 5000 up … STALE: the last finished check round ended 30s ago (limit: 3 intervals = 30s)`; the STALE line repeats every second (the age changes) until `t+91s`, when round 2 finished and the line is fresh again (`took 41.355s`) |

A bug found on the way: STALE comes from the clock, not from an event, so the first version never showed it (the feed only sent a line when something changed). The feed now looks again every second (`TestFeedShowsStaleWhenTimePassesWithNoEvent`).

---

## 11. Second review: decisions, safety fixes, and what was found

### 11.1 Owner decisions built

1. **STALE** means: **no check result has arrived for three intervals.** A round longer than two intervals gets a plain note in the tooltip, not an alert; the length of the last round is always shown. Tested at 5000 machines with caps 200 and 1000 (fake clock: `TestStaleRuleAt5000MachinesCap1000`, `TestLongRoundAt5000MachinesCap200IsANoteNotAnAlert`; live below).
2. **First round:** `checking... N of M done` instead of partial counts (`TestFirstRoundShowsHowManyAreDoneNotPartialCounts`), and **at most one feed line per second** (`TestFeedSendsAtMostOneLinePerSecondDuringTheFirstRound`).
3. **Single click is on by default** (`-D single_click=true` is passed to wofi); `--no-single-click` goes back to a double click.
4. `/etc/hubos/wofi.css` is a backed-up per-machine file (`HUB-OS.md`, NAS and backups).
5. **Viewer logs:** 128 KiB per log with one rotated copy, plus **64 MiB for all logs together**, oldest removed first (`TestTotalLogCapRemovesTheOldestFirst`: six files, cap 6000 → nothing; 4500 → the two rotated copies go; 2500 → the two oldest current logs are emptied and kept; far under the cap → nothing).
6. While STALE, the age is shown in **10 s steps**, so the tooltip changes at most every 10 s (tested: the same tooltip at 31 s and 38 s, a new one at 41 s).
7. **Probe cap:** default 200; **1000 when the inventory has more than 1000 machines**; **always at most 80% of `ulimit -n`**. Big clusters want `ulimit -n` of at least 2048 (1000 / 0.8 = 1250, rounded up to a usual value). `hubd serve` prints the limit and the cap, and says so when it lowered the cap. `hubd check` uses the same rule.

### 11.2 Safety fixes

1. **Inventory (rules and broken files 27, 28):** `address`, `user` and `share` must not start with a dash; `id` must match `^[a-z0-9][a-z0-9-]*$`. Messages: `nas-1: address "-oProxyCommand=x" must not start with a dash`; `-nas: id "-nas" must start with a letter or digit, not a dash`. Documented in `docs/inventory-format.md`. `examples/viewers.example.toml` recommends `--` before arguments derived from the inventory where the viewer supports it.
2. **Socket:** a leftover file at the socket path is removed **only if it is a socket** (a plain file, a link or a folder there is never removed; hubd refuses and says what it found). hubd also refuses a socket folder that is **not owned by the user running it or not mode 0700**, with the fix in the message (`chmod 700 …`). Tests: `TestListenNeverRemovesWhatIsNotASocket` (file with content kept, link target kept, folder refused, a real leftover socket replaced), `TestListenRefusesAFolderThatIsNotOwnedOrNotMode0700` (0755, 0770 and 0500 refused, 0700 accepted; the other-owner case runs only as root: `chown 12345`).
3. **Markup** (tested with the installed Waybar 0.9.24 and wofi 1.4.1, names `A<b>bold</b>`, `Fish & Chips`, `Say "hi"`, `It's &amp; <i>done</i>`):
   - **Bar text:** it holds only counts, the word STALE and `checking... N of M done`; no machine name ever goes into it, so it cannot garble.
   - **Tooltip, before the fix: garbled.** The tooltip box appeared **blank** (a tiny empty rounded box). Waybar's log: `Failed to set text '…' from markup due to error parsing markup: … Entity did not end with a semicolon; most likely you used an ampersand character without intending to start an entity — escape ampersand as &amp;`. The same blank box with Waybar's documented `"escape": true` option (the manual says only "Option to enable escaping of script output"): `Entity name “ Chips, Say "hi", It's &amp” is not known`. So the documented option did not fix the tooltip in 0.9.24.
   - **Fix:** `hubd feed` escapes `&`, `<`, `>`, `"` and `'` itself (`&amp; &lt; &gt; &quot; &#39;`), and the module is used without `"escape"`. After the fix the tooltip shows, in words: `Fish & Chips is down, not opened` / `4 down: A<b>bold</b>, Fish & Chips, Say "hi", It's &amp; <i>done</i>` / `last check round took 0s` — every character literal, no bold or italic, no errors in the log (0 markup errors with `escape` false and with `escape` true). (`TestStatusLineJSONEscapesMarkupForWaybar`.)
   - **wofi list:** plain text by default (no `--allow-markup`), so all four names show literally: `A<b>bold</b>`, `Fish & Chips`, `Say "hi"`, `It's &amp; <i>done</i>`, columns aligned. Nothing garbled.
4. **Orphan viewer** (a viewer whose window appears after the window wait), recorded as it is **today** in `TestOrphanViewerWindowAfterTheWait` (window wait 300 ms, settle 500 ms as in production):
   - **A. The window appears after the failure; the owner clicks again later.** First click: `no window appeared for A within 300ms`; the machine shows idle, nothing recorded. The first viewer's window then appears (`#0 hubos-a`), unrecorded and unknown to hubd. Second click: a **second viewer is started**; its window (`#1 hubos-a`) is recorded; hubd says `opened A at home (1, 1), matched by name`. **Two viewers and two windows exist, both named `hubos-a`; hubd knows only #1.** Nothing warns about #0.
   - **B. The late window arrives while the second click waits, and the second viewer is quick** (inside the 500 ms settle): two new windows with the same name → `2 new windows appeared while opening A (#0 "hubos-a", #1 "hubos-a"); I cannot tell which one is its window…`; the machine is blocked until `hubd forget`.
   - **C. The same, but the second viewer is slow** (slower than the settle): hubd takes the **first** viewer's late window (#0) for the second viewer and says `opened A … matched by name`; the second viewer's window (#1) appears later and is unrecorded. Two windows, hubd knows #0.
   - Behaviour not changed in that review; the fix is in section 12.

### 11.3 Measurements after these changes

(Same commands as section 7; automatic cap.)

| Case | Result |
|---|---|
| 100, local fakes, cap 200 | rounds 7, 18, 3 ms; peak open files 67; memory 8.8 MB; list 39 lines / 1,648 bytes; status line 135 bytes |
| 5000, local fakes, **cap 1000 (automatic)** | rounds 224, 164, 167 ms; peak open files 981; memory 32.9 MB (was 18.5 MB at cap 200); list 62 lines / 2,897 bytes; flat list 1,002 lines; status line 182 bytes |
| 5000, `ulimit -n` 1024 | cap **819** (80%); round 183 / 138 ms; correct (4750 up, 250 down); peak open files 769 |
| 5000, `ulimit -n` 256 | cap **204**; round 130 / 118 ms; correct; peak open files 133 |
| 5000, `ulimit -n` 100 | cap **80**; round 128 / 132 ms; correct; peak open files 74 |
| `hubd check`, 5000, `ulimit -n` 20000 / 1024 / 256 | 4750 up, 250 DOWN, 0 false, 0 NOT CHECKED; 306 / 286 / 271 ms |
| 5000, none reachable, **cap 1000 (automatic)**, interval 10 s, live feed over 75 s | rounds 10.07, 10.02, 10.03, 10.02 s; **14 feed lines in all, never STALE**; first round: one line per second at most (11 lines in the first 15 s, one per second); `checking... 10 of 5000 done` at 0 s, `59` at 1.6 s, `1018` at 2.6 s; then `0 of 5000 up`, alert, tooltip ending `last check round took 10.029s` (no note: 10 s is under two intervals) |
| 5000, none reachable, **cap 200**, interval 10 s, live feed over 135 s | rounds 41.3 s and 40.6 s; **43 feed lines, never STALE** (results kept arriving); first round: at most one line per second (15 lines in the first 15 s); the tooltip ends `last check round took 41.329s` / `note: that is longer than two intervals (20s); a bigger check cap or interval may fit this many machines better`; class `alert` only because every machine is down |

In section 10 the same case (cap 200) was STALE for about 20 s of every 50 under the old rule; under the new rule it never is.

---

## 12. Third review: the late-window state, and three flags

### 12.1 Slow viewers (the orphan viewer, fixed)

**`window_wait` in `viewers.toml`** (optional, per viewer, a positive duration such as `"10s"` or `"1m30s"`; default is the hub's `--window-wait`, 10 s). A bad value stops `hubd serve` before anything starts (`window_wait "soon" must be a positive duration like "10s" or "1m30s"`). Documented in `examples/viewers.example.toml`.

**What happens when the window wait runs out.**
- If the viewer process has **exited** (also with status 0), it is the old failure: `no window appeared for X within 10s`, with the log path.
- If the process is **still alive**, the machine enters the **late-window state** for the grace period (`--late-grace`, default 60 s, a hub setting; 0 turns it off):
  - `hubd open` answers `no window appeared for X within 1s; the viewer is still running, so hubd keeps waiting for its window for up to 20s (…; hubd forget ID stops the wait)` (exit 1). The list shows `[waiting for its window]`.
  - A new `open` says `the viewer for X is still starting; waiting for its window` and **starts nothing**.
  - A window that appears is **adopted automatically** and placed at home: matched by name for viewers with `sets_name = true`; for the others, exactly one new unclaimed window (several → the machine becomes unidentified, nothing is touched). The bar and menu say `opened X at home (…), matched by name (its window came late)`.
  - If the **process exits first**: failure, `sh exited (exit status 3) before showing a window for X; its output is in <log>`; the machine is idle again.
  - If the **grace ends with the process alive and no window**: the existing "unidentified" state (`the viewer for X is still running but no window appeared during the 20s of waiting after the window wait; hubd stopped waiting and killed nothing. Close the viewer by hand, then run: hubd forget X`). **No process is ever killed.** `hubd forget X` is the way out, also during the late state (it stops the wait; the viewer is not touched).
  - While a machine is late and its viewer is matched **by comparison**, no other comparison-matched viewer is started (`…its window cannot be told apart from this one's; wait for it…`), because a late window could not be told from the new one's.
- **Same-name warning.** If two or more windows carry the same `hubos-<id>` name, the menu's first line and the tooltip say `WARNING: 2 windows are named hubos-m5 (Up <u>one</u> & 'ok'); hubd knows window #1; close the extra one by hand`, and the bar item turns red. `forget` and `end` keep working on the window hubd knows (`end` closes only that one).

**Cases A, B and C of the review, now.** (`internal/hub/late_test.go`, window wait 300 ms, settle 500 ms; case A also live with `foot` behind a 3 s delay.)

| Case | Before the fix | Now |
|---|---|---|
| A. The window shows up after the wait; the owner clicks again | A second viewer started; two windows named `hubos-a`; hubd knew only the second | The click during the late state starts nothing; the late window is adopted at home; a click afterwards says `went to the open window`; **1 viewer, 1 window** (`TestLateCaseA_NoSecondViewer`) |
| B. The late window arrives while the second click waits; the second viewer is quick | Two viewers; `cannot tell which one`; the machine was blocked | The second click starts nothing; **1 viewer, 1 window**, adopted (`TestLateCaseB_SecondClickStartsNothing`) |
| C. The same with a slow second viewer | The first viewer's late window was taken for the second viewer's; the second window stayed unknown | There is no second viewer, so the window that comes is the right one: window #0 recorded, **1 viewer, 1 window** after 1.8 s more (`TestLateCaseC_NoWrongWindowAdopted`) |

More tests: the grace ends (unidentified, nothing killed, forget works), the viewer exits first (log path, can open again), forget during the wait, a late window for a comparison-matched viewer (the window that was there before is ignored; a second comparison viewer is refused), two same-name late windows (not guessed), per-viewer `window_wait` over the hub default, the same-name warning.

Live run (the fake inventory, `testdata/viewers/late.toml`: window 3 s after start, `window_wait` 1 s, grace 20 s): first click → the late message after 1.0 s (exit 1); second click → `still starting`; list `[waiting for its window]`; 4 s later `[open]`, one `foot`, window at `[2000, -1500]` (its home); third click `went to the open window`. Grace test (viewer sleeping 30 s, grace 4 s): after 5.5 s the menu shows the `killed nothing` message and `x forget unknown window…`; a click is refused; `forget` works; the viewer process was still alive before and after. Viewer exiting at 3 s: the failure message with the log path on the bar tooltip and the menu.

### 12.2 Flags

- **`--no-escape`** (`hubd serve`): do not escape the status line. By default hubd escapes `&`, `<`, `>`, `"` and `'` in the bar text and tooltip, because Waybar 0.9.24 reads them as markup (section 11). **A newer Waybar that escapes by itself would show the default's escaped text double-escaped** (`&amp;` instead of `&`), so use `--no-escape` with such a Waybar. **With Waybar 0.9.24 `--no-escape` is wrong for names with `&` or `<`**: tested live, the tooltip is a blank box and Waybar's log says `Failed to set text … from markup due to error parsing markup … Entity did not end with a semicolon` (10 errors in 3 s). Tested: `TestNoEscapeFlagLeavesTheTextAlone` (default escapes, the flag does not).
- **`--check-timeout D`** (`hubd serve` and `hubd check`; replaces `--probe-timeout`): the per-machine limit, default 2 s. It must be more than 0 and at most 1 minute; for `hubd check` it must also not be longer than that command's total limit (5 s). Otherwise hubd stops with `--check-timeout must be more than 0`, `… at most 1m0s` or `… longer than the total limit of this command (5s)`. Tested in `TestCheckTimeoutFlagIsValidated`. Effect: see 12.4.
- **`--late-grace D`**: see 12.1. `--window-wait` must be more than 0 and `--probe-interval` more than 0.

### 12.3 STALE tooltip

`STALE: no check result for 40s (limit: 3 intervals = 30s); the last check round finished 40s ago`, both ages in 10 s steps; before any round has finished: `…; no check round has finished yet` (`TestStaleTooltipShowsBothAges`).

### 12.4 Measurements after these changes

(Same commands as section 7; `/tmp/hs2/measure.sh`.)

| Case | Result |
|---|---|
| 100, local fakes | rounds 7, 2, 3 ms; peak open files 8; memory 8.8 MB; list 39 lines, 1,648 bytes; status line 135 bytes |
| 5000, local fakes, automatic cap 1000 | rounds 223, 134, 137 ms; peak open files 908; memory 31.6 MB; list 62 lines, 2,897 bytes; status line 182 bytes |
| 5000, local fakes, `--check-timeout 500ms` | rounds 205, 145 ms (no change: local checks end at once) |
| 5000, none reachable, cap 1000, `--check-timeout 2s` (default) | round **10.07 s**; peak open files 1007; memory 32.8 MB |
| 5000, none reachable, cap 1000, `--check-timeout 1s` | round **5.03 s**; peak open files 1007; memory 33.1 MB |
| 100, none reachable, cap 200, `--check-timeout 1s` | round 1.004 s (2.003 s at the default) |
| `hubd check`, 100 none reachable | 1,024 ms with `--check-timeout 1s`; 2,027 ms at the default |

So the round time with silent machines is (machines / cap) x timeout, as expected: the timeout halves it. A shorter timeout on a LAN is safe only where a live machine answers a connect well within it (a real network was not measured).

### 12.5 Not changed, not tested

- The late-window state with **real** viewers: how long Moonlight, virt-viewer or Remmina really take to show a window is UNKNOWN; `window_wait` and `--late-grace` are guesses (10 s and 60 s as decided).
- A late window of a comparison-matched viewer while **other, unrelated programs** also open windows (not launched by hubd): hubd would take the single new window for the viewer's. This is the same limit as the normal comparison path.
- The same-name warning counts only windows named `hubos-<id>` for machines in the inventory.

---

## 13. Fourth review: late-window follow-ups

1. **`hubd end` on a machine in the late state** is refused, in plain words: `the viewer for A is still starting, so there is no window to close. hubd end cannot cancel a viewer (hubd never kills one). To stop waiting for its window, run: hubd forget a` (`TestEndOnALateMachineRefusesAndSaysWhy`; nothing is closed and the state does not change).
2. **A quiet late state.** While a machine is late:
   - the menu has a line `x stop waiting for <name>'s window (<id>)` that does what `hubd forget` does (it stops the wait; the viewer is not touched, nothing is closed or moved);
   - the tooltip has `waiting for <name>'s window (up to N s more)`, N rounded **up** to 10 s steps (25 s left shows 30, 14 s shows 20), so it changes at most every 10 s;
   - **the bar item is not red** for this, and the first "no window yet" answer is no longer put on the bar as a red message (it is still printed by `hubd open`, exit 1).
   Tested in `TestLateMachineShowsStopWaitingLineAndANonRedTooltip`: class `ok`, tooltip line, the menu line, picking it, and that a pick with an unknown id does nothing.
3. **`late_grace` per viewer** in `viewers.toml` (optional, a positive duration; default is the hub's `--late-grace`, 60 s). Same validation as `window_wait`: `late_grace "long" must be a positive duration like "60s" or "2m"`. It is in `examples/viewers.example.toml`. Tests: `TestLateGrace` (parse, defaults, bad values, a bare number refused) and `TestPerViewerLateGraceOverridesTheHubSetting` (a 600 ms viewer grace ends in under 3 s although the hub setting is 1 minute; a viewer without it uses the hub's).
4. **A clean exit (status 0) keeps waiting.** Before the window appears, a viewer process that exits with status 0 (maybe a hand-over to a copy that is already running) no longer fails at once: hubd goes on waiting through the late grace and adopts the window if it comes (message: `…the viewer process has exited cleanly (it may have handed over to a copy that is already running), so hubd keeps waiting…`). A non-zero exit is still a failure at once, with the log path. If the grace ends with no window, the machine is "unidentified" as before (`TestCleanExitKeepsWaitingNonZeroExitFailsAtOnce`: clean exit then a late window is adopted; clean exit and no window ends unidentified; status 3 fails in under 2 s with the log path).
5. **`hubd serve --ignore-app-id APPID`** (repeatable; default none): a window with one of these app-ids is **never a candidate when windows are matched by comparison** (not for viewers that set their name, which match by name). Use it for programs of your own that open windows now and then (a menu, a notification pop-up) and could otherwise be taken for a viewer's window or make the match ambiguous. It applies to the normal wait and to the late state. Exact match on the app-id. Tests: `TestIgnoredAppIDsAreNeverCandidatesForComparisonMatching` (two new windows are ambiguous without the list; with it the right window is chosen and the ignored one is never moved; same in the late path) and `TestIgnoreAppIDFlagIsRepeatable` (the flag).

**Live checks** (nested driftwm, Waybar, wofi, `foot`; the viewers are the test tables in `testdata/viewers/`):
- *Late viewer, window 40 s away:* `hubd open ai-1` → the late message (exit 1); `hubd end ai-1` → `the viewer for AI Box is still starting, so there is no window to close. hubd end cannot cancel a viewer…` (exit 1). The feed line is `{"text":"6 of 6 up","class":"ok","tooltip":"waiting for AI Box's window (up to 60 s more)\n1 not checked\nlast check round took 1ms"}`. Screenshots in words: the bar item is **green** "6 of 6 up"; hovering shows a small box with three lines, `waiting for AI Box's window (up to 60 s more)`, `1 not checked`, `last check round took 1ms`; the menu's first, highlighted line is `x stop waiting for AI Box's window (ai-1)`, and the row of ai-1 reads `UP [waiting for its window]`.
- *Hand-over viewer* (`testdata/viewers/handover.toml`: the process exits at once with status 0, the window comes 3 s later): `hubd open ai-1` → `no window appeared for AI Box within 1s; the viewer process has exited cleanly (it may have handed over to a copy that is already running), so hubd keeps waiting for its window for up to 20s` (exit 1); 4 s later the row says `UP [open]`, one `foot`, window at `[2000, -1500]` (its home).
- *`--ignore-app-id`* (a viewer that opens a pop-up window `popup` and its own window `whatever` at once, matched by comparison): without the flag `2 new windows appeared … (#2 "whatever", #1 "popup"); I cannot tell which one is its window…`; with `--ignore-app-id popup`: `opened AI Box at home (2000, -1500), matched by comparison`.

**Measurements after these changes** (same commands as section 7; they are not affected by this change, and were rerun to be sure): 100 machines, local: rounds 9, 10, 3 ms, memory 9.2 MB, list 39 lines / 1,648 bytes, status line 135 bytes. 5000, local, automatic cap 1000: rounds 220, 138, 144 ms, peak open files 959, memory 30.0 MB, list 62 lines / 2,897 bytes, status line 182 bytes. 5000, none reachable, cap 1000: round 10.10 s, peak open files 1007, memory 31.5 MB.

---

## 14. Fifth review: owner decisions on real viewers

Built on the branch of the fourth review (section 13), as new commits.

### 14.1 The clean-exit bug in the late wait (fixed)

Section 13 item 4 said a clean exit keeps waiting. That held only when the viewer had **already** exited cleanly when the window wait ran out. If the viewer was still running at that moment and exited with status 0 **during** the late grace, `watchLate` treated it as a failure and ended the wait. Now a clean exit during the grace is ignored (the wait goes on until the grace ends or a window appears); a non-zero exit during the grace is still a failure with the log path. Tests: `TestCleanExitDuringTheGraceKeepsWaitingAndAdoptsTheLateWindow` (clean exit 400 ms after the grace began, window 1.2 s later: adopted, one viewer; clean exit and no window: the wait ends only when the grace ends, not at the exit), `TestNonZeroExitDuringTheGraceIsAFailureWithTheLogPath`. The first test fails when the fix is taken out (checked by hand).

### 14.2 Inventory: `session`

Optional text, the name of the Sunshine app to stream. Same rules as `share`: no control characters or line breaks, must not start with a dash, and only allowed when `open` includes `moonlight`. Broken files 29, 30 and 31. See `docs/inventory-format.md`.

### 14.3 viewers.toml

- `[default_ports]` (optional table, section 15.1): `{port}` falls back to the entry for the machine's first `open` entry when the machine has no port. An error only when neither exists. It is also the port of the up/down check (14.4).
- `{session}`: the machine's `session`; if missing: `<id> has no session in the inventory, and its viewer command needs one` (nothing is started).
- `title_match` (optional, only with `sets_name = false`): a template such as `"{id} - Moonlight"`. The candidate windows are only those whose title equals the rendered text **exactly**, and the machine's window is recorded as matched by **title**. `sets_name = true` together with `title_match` is refused at start-up. If no window has the title but another new window appeared, the failure message says so (`its title is "Moonlight", not "a - Moonlight": check title_match`).
- Because the windows of title-matched machines can be told apart, **the late-window comparison lock does not apply to them**: a title-matched machine that is late does not block comparison-matched viewers, and two title-matched machines can be opened at the same moment (they do not wait for each other; the one-launch-at-a-time lock is only for viewers matched by comparison or by name). The other way round, a comparison-matched viewer ignores windows whose title belongs to a title-matched machine, so it cannot take one for its own.
- After a hubd restart (and when hubd starts), a title-matched machine adopts a window by title only when **exactly one** window has its title and no other machine has the same title. Two or more windows with the title: nothing is adopted and the existing duplicate warning is shown, now worded `WARNING: 2 windows are titled "a - Moonlight" (A); hubd knows none of them; close the extra one by hand`. When opening, two new windows with the title: nothing is touched, the machine becomes "unidentified" (`hubd forget` is the way out), as for the other kinds.

Tests (`internal/hub/title_test.go`; the fake viewer sets the fixed app-id `com.moonlight_stream.Moonlight` and the title `<id> - Moonlight` the way Moonlight would): `TestTitleMatchTwoMachinesOpenedAtOnceGetTheirOwnWindows`, `TestTitleMatchTwoWindowsWithTheSameTitleGiveAWarningAndNoGuess`, `TestTitleMatchAdoptionAfterHubdIsKilled` (a new hub with no record file; the record path also), `TestTitleMatchLateWindowIsAdoptedAndDoesNotLockComparisonViewers` (both directions), `TestTitleMatchNoWindowWithTheRightTitleSaysSo`, `TestMachineWithoutSessionIsRefusedBeforeAnythingStarts`; and in `internal/viewers` `TestDefaultPortsSessionAndTitleMatch`. These use the fake compositor and a fake launcher, **not** a real `foot` under driftwm (not run in that round; section 16 is the live check).

### 14.4 Default check ports

(Superseded by section 15: the default now comes from a `[default_ports]` table, not from a viewer. The rest of this item still holds.) The check port of a machine is its own `port`, otherwise the default for its **first** `open` entry. If neither exists the machine is `not checked` with the reason `no port in the inventory and no default port for <program>`. Applied in `hubd serve` and in `hubd check`. `hubd check` reads `viewers.toml` for this: `--viewers PATH`, default next to the inventory. A missing file at the default place is not an error (machines without a port are then not checked); a file that was asked for and is missing, or one that is invalid, stops `hubd check` (exit 2). Spice and vnc have no default in the real example (a guest's console port is set per guest); a guest whose first entry is `ssh` would get the ssh default, which is provisional like the rest of the guest address rules. Tests: `TestCheckUsesTheViewersDefaultPort` (`cmd/hubd`, now `TestCheckUsesTheDefaultPortsTable`), `TestCheckPortIsTheMachinesOwnOrTheViewersDefault` (now `…OrTheDefaultPortsTable`) (`internal/hub`), `TestCheckPortRules` (`internal/viewers`).

### 14.5 Real viewer examples

`examples/viewers.real.example.toml`: ssh (foot with a tmux session), spice and vnc (remote-viewer), moonlight, in the exact shapes the owner chose. Every entry says UNVERIFIED on hardware. Tested: the file loads and validates; each command renders the right argument list for a sample machine; for the ssh shape, `ssh -G` from the unpacked openssh-client 9.6p1 package (nothing installed) reads host, port, user and "request a terminal" as intended, and a hostile address after `--` is treated as a host name, not an option. **Not tested:** the tmux part (`tmux new-session -A -s hubos` needs a real server), Moonlight (never run), remote-viewer against a real guest. `go test` skips the `ssh -G` test unless an `ssh` is on the PATH or `SSH_BIN` names one.

### 14.6 Measurements (rerun then, with the per-viewer default ports; section 15.4 has the new ones)

`tools/geninv -viewers FILE` writes a viewers table with a `[default_ports]` table and leaves the port out of every machine except guests. Commands as in section 7; script `measure.sh` (throw-away). Sandbox as before.

| Case | Result |
|---|---|
| 100, own ports | rounds 4 (first, fake nodes still starting), 2, 3 ms; peak open files 6; memory 8.5 MB; list 39 lines / 1,648 bytes |
| 100, **default ports** | rounds 5, 3, 2 ms; 95 up, 5 down, 0 not checked; peak open files 6; memory 8.2 MB; list 39 lines / 1,648 bytes |
| 5000, own ports, cap 1000 | rounds 171, 119, 132 ms; peak open files 984; memory 30.6 MB (peak 32.3); list 62 lines / 2,897 bytes |
| 5000, **default ports** (3,948 machines without a port), cap 1000 | rounds 170, 131, 130 ms; 4750 up, 250 down, 0 not checked; peak open files 856; memory 31.7 MB; list 62 lines / 2,897 bytes |
| 5000, default ports, `ulimit -n 1024` | rounds 142, 132 ms; peak open files 708 |
| `hubd check`, 5000, default ports, `ulimit -n` 20000 / 1024 / 256 | 4750 of 5000 up; 317 / 318 / 251 ms |
| `hubd check`, 100, default ports | 95 of 100 up; 16 ms |
| 5000, none reachable, default ports, cap 1000 | rounds 10.05 s and 10.02 s; peak open files 1006; memory 37.1 MB |

Nothing got slower: the default is looked up once when hubd starts. The status line is longer than in section 13 only because driftwm was not running in this run (`driftwm is not reachable: windows cannot be opened` is part of the tooltip).

### 14.7 Not changed, not tested

- A live run with `foot` as a fake Moonlight under driftwm (two machines opened at once, adoption after `kill -9`) was **not** done in this round; the tests above use a fake compositor. The window names and titles in them are what Moonlight is read to use (docs/viewers-research.md), not something seen.
- Moonlight, Sunshine, the tmux session, remote-viewer against a real guest: UNVERIFIED on hardware.
- A title-matched window whose title changes while it is open (Moonlight might change it): UNKNOWN; hubd records the title at the moment it is matched and checks the window by its window id and app-id afterwards.

---

## 15. Sixth review: the default-ports table, and what was checked

### 15.1 `[default_ports]` replaces the per-viewer `default_port`

Section 14 had a `default_port` field on each viewer. hubd is unreleased, so format 1 changed instead of getting a second field: the **per-viewer `default_port` is gone and is now an unknown-field error** (`line 7: unknown field "viewer.default_port"`).

`viewers.toml` has an optional top-level table, before the first `[[viewer]]`:

```
[default_ports]
moonlight = 47989
ssh       = 22
files     = 445
```

- Keys are programs: `moonlight`, `spice`, `vnc`, `ssh`, `files`. `none` is not allowed; any other key is an error (`default_ports: "remmina" is not a program that has a port (one of: moonlight, spice, vnc, ssh, files)`). Values are whole numbers from 1 to 65535 (`default_ports: the port for "ssh" must be a whole number from 1 to 65535 (found: 0)`; text, a decimal number, a negative number and 65536 are refused the same way).
- `{port}` is the machine's own port, otherwise the table entry for the machine's **first** `open` entry; an error (`<id> has no port in the inventory, and its viewer command needs one`) only when neither exists.
- The up/down check (`hubd serve` and `hubd check`) uses the same rule, and a machine with neither is `not checked (no port in the inventory and no default port for <program>)`. The entry belongs to the program, not to a viewer: an entry needs no viewer (a NAS opened with `files` is checked on the `files` entry even if no viewer serves `files` yet; opening it still needs a viewer).
- Spice and vnc have no entry in `examples/viewers.real.example.toml`: a guest's console port is set per guest, so the inventory gives it, and a guest with spice and no port is `not checked`.
- `examples/viewers.real.example.toml` sets `moonlight = 47989`, `ssh = 22`, `files = 445` with the comment that these ports come from `docs/viewers-research.md` and are UNVERIFIED on hardware. There is still **no `files` viewer entry** in it (which file manager and command is open; the 445 entry is only the check port).
- `tools/geninv -viewers FILE` now writes a `[default_ports]` table (moonlight, files and ssh = the fake port 21000).

Tests: `TestDefaultPortsSessionAndTitleMatch` and `TestCheckPortRules` (`internal/viewers`: table values, first open entry decides, own port wins, errors in plain words, `default_port` on a viewer refused, a table entry needs no viewer), `TestCheckPortIsTheMachinesOwnOrTheDefaultPortsTable` (`internal/hub`: moonlight machine on 47989, own port 4000 wins, a files NAS on 445, a guest with spice and no port is "not checked" and counted so, not as down), `TestCheckUsesTheDefaultPortsTable` (`cmd/hubd`, real sockets: the same, through `hubd check`, plus a missing or broken viewers file and `none` in the table), `TestRealExampleLoadsAndRendersTheChosenCommandLines` (the real example's table), `TestDefaultPortsMode` (`tools/geninv`).

### 15.2 `hubd check` without a viewers file

If there is no `viewers.toml` at the default place (next to the inventory), `hubd check` prints **one line** to standard error and goes on with exit code 0: `hubd: no viewers file was found at <path> (the default place), so machines without a port in the inventory are not checked`. An explicit `--viewers PATH` that is missing, or any viewers file that is invalid, is still exit code 2. Test: `TestCheckWithoutAViewersFileSaysSoInOneLine` (one line only; nothing is printed when a file exists).

### 15.3 Check of the proposed moonlight command

Read against the moonlight-qt source at the commit named in `docs/viewers-research.md`: every flag of the proposed command exists and accepts the value used; a host without a port is accepted (Moonlight adds a new host with the default port 47989). Details with file and line are in `docs/viewers-research.md`, section "Check of the proposed moonlight command". Findings that matter here: Moonlight first shows a status window (same app-id, title believed to be "Moonlight") and an error dialog in that same window if the start fails; the stream window title uses the host's own name; another running app on the host triggers a question first. The example was not changed.

### 15.4 Measurements (rerun now; same commands as section 7 and 14.6)

| Case | Result |
|---|---|
| 100, own ports | rounds 4, 4, 2 ms; 95 up, 5 down; peak open files 6; memory 9.1 MB; list 39 lines / 1,648 bytes |
| 100, **default ports** (table) | rounds 3, 2, 4 ms; 95 up, 5 down, 0 not checked; peak open files 6; memory 8.8 MB; list 39 lines / 1,648 bytes |
| 5000, own ports, cap 1000 | rounds 180, 151, 159 ms; peak open files 910; memory 32.9 MB; list 62 lines / 2,897 bytes |
| 5000, **default ports** (3,948 machines without a port), cap 1000 | rounds 159, 122, 144 ms; 4750 up, 250 down, 0 not checked; peak open files 726; memory 36.0 MB; list 62 lines / 2,897 bytes |
| 5000, default ports, `ulimit -n 1024` | rounds 172, 147 ms; peak open files 708; memory 29.4 MB |
| `hubd check`, 5000, default ports, `ulimit -n` 20000 / 1024 / 256 | 4750 of 5000 up; 349 / 325 / 281 ms |
| `hubd check`, 100, default ports | 95 of 100 up; 29 ms |
| 5000, none reachable, default ports, cap 1000 | rounds 10.06 s and 10.03 s; peak open files 1006; memory 34.3 MB; list 62 lines / 2,916 bytes |

The table is read once when hubd starts; nothing in a round got slower. (The numbers differ a little from section 14.6 run to run; memory with defaults was 31.7 MB there and 36.0 MB here, which is within the spread of the 5000 runs, 29 to 36 MB.)

---

## 16. Live check, title matching

**What this is.** foot stands in for Moonlight under a nested driftwm (software rendering, no GPU), using the method of `docs/driftwm-findings.md` section 0: driftwm at the pinned commit `352333a8fa1b22171492d4b71a54102045c9a19d`, built from source; packages from the Ubuntu archive only downloaded and unpacked (nothing installed); a virtual X display (Xvfb :99, 1280x800); `--backend winit --config /dev/null`; everything in `/tmp/hs3`, a short runtime directory `/tmp/dwx`, deleted afterwards. Moonlight itself was **not** run; foot only sets the window name and title that Moonlight is read to set. The fake machines are four `tools/fakenode` listeners on `127.0.0.31` to `127.0.0.34`, port 21000.

Set-up used (the `moonlight` viewer has the shape of `examples/viewers.real.example.toml`: `sets_name = false`, `title_match`, `[default_ports]`; the machines `ai-1` and `desk-1` have no `port`, so the check uses the table and they are UP):

```
[default_ports]
moonlight = 21000

[[viewer]]
id           = "moonlight"
programs     = ["moonlight"]
command      = ["foot", "--app-id=com.moonlight_stream.Moonlight", "--title={id} - Moonlight", "--", "sleep", "infinity"]
sets_name    = false
title_match  = "{id} - Moonlight"
window_wait  = "5s"
late_grace   = "30s"
# + two more viewers for the cases below (slowtitle, twin), same title_match
```

Commands to start it:

```
Xvfb :99 -screen 0 1280x800x24 -nolisten tcp &
DISPLAY=:99 $A/target/debug/driftwm --backend winit --config /dev/null &
export WAYLAND_DISPLAY=wayland-1 DISPLAY=:99
fakenode 127.0.0.31:21000 127.0.0.32:21000 127.0.0.33:21000 127.0.0.34:21000 &
hubd serve --inventory live/inventory.toml --viewers live/viewers.toml --socket /tmp/hm/h.sock --probe-interval 2s --bar-height 0 &
```

(`hubd list` then shows ai-1, desk-1, slow-title and twin as UP.)

### 16.1 Two machines opened at the same moment

```
$ driftwm msg state          (before: no windows)
windows 0
$ hubd open --socket S ai-1 &  hubd open --socket S desk-1 &  wait      (same moment)
opened Desktop One at home (450, 250), matched by title
opened AI Box at home (-450, 250), matched by title
  [desk-1 exit 0 after .830615137s]
  [ai-1 exit 0 after .832518805s]

$ driftwm msg state
camera 123.37903160065866 141.53172479395903
zoom 1
layout  (us)
windows 2
  * #0 com.moonlight_stream.Moonlight [450, 250] 700x525  "desk-1 - Moonlight"
    #1 com.moonlight_stream.Moonlight [-450, 250] 700x525  "ai-1 - Moonlight"
fullscreen 0
pinned 0
layers 0
canvas-layers 0
outputs 1
  * winit camera 123.37903160065866 141.53172479395903 zoom 1 1280x800

$ hubd list
   ai-1             AI Box                   UP [open]
   desk-1           Desktop One              UP [open]
```

Both windows have the **same** app-id; each was found by its title and put at its own home (`ai-1` at (-450, 250), `desk-1` at (450, 250); both opens took 0.83 s, neither waited for the other). Screenshot (`driftwm msg screenshot`) in words: two dark terminal windows with a title bar "ai-1 - Moonlight" (left, cut off by the screen edge) and "desk-1 - Moonlight" (right), side by side with a gap between them, each showing foot's one-line locale warning; the dotted canvas background around them.

### 16.2 `kill -9` of hubd, then a restart

The record file was removed first, so only the windows themselves are left to go by (with the record file, the record path also adopts: unit test `TestTitleMatchAdoptionAfterHubdIsKilled`).

```
$ pgrep -c -x foot
2
$ kill -9 $(pgrep -n -x hubd)
9462
$ rm the old record file so only the windows are left: rm /tmp/hm/h.record.json
h.record.json
h.viewer-logs

$ hubd serve ... (restart)
hubd: serving 5 machines on /tmp/hm/h.sock; driftwm at /tmp/dwx/driftwm/ipc-wayland-1.sock
hubd: open-file limit 20000; check cap 200 (asked for 200); round every 2s; per-machine limit 2s

$ hubd list
   ai-1             AI Box                   UP [open]
   desk-1           Desktop One              UP [open]
   slow-title       Slow Title               UP
   twin             Twin Windows             UP
$ pgrep -c -x foot  (nothing was started)
2
$ driftwm msg state
windows 2
  * #0 com.moonlight_stream.Moonlight [450, 250] 700x525  "desk-1 - Moonlight"
    #1 com.moonlight_stream.Moonlight [-450, 250] 700x525  "ai-1 - Moonlight"
fullscreen 0
pinned 0
$ hubd open --socket S ai-1
went to the open window of AI Box
  [exit 0]
$ pgrep -c -x foot
2
```

(The pid `9462` printed right after the `kill -9` is `pgrep` seeing the process while it was still being cleaned up; `ps` a moment later showed only the new hubd.) Both windows were adopted by title, nothing was started (2 foot processes before and after), and `hubd open ai-1` went to the existing window.

### 16.3 A window whose title is set one second after it appears

The viewer `slowtitle` starts foot titled "Moonlight" and sets the title "slow-title - Moonlight" a second later (an escape sequence from a shell in the window), as Moonlight may do.

```
$ hubd open --socket S slow-title   (window appears titled "Moonlight", gets "slow-title - Moonlight" about 1 s later)
  t+0.3s: 
  t+0.6s:  * #2 APPID [-425, 225] 700x525 "Moonlight"
  t+0.9s:  * #2 APPID [-425, 225] 700x525 "Moonlight"
  t+1.2s:  * #2 APPID [-425, 225] 700x525 "Moonlight"
  t+1.5s:  * #2 APPID [-425, 225] 700x525 "slow-title - Moonlight"
opened Slow Title at home (-450, -250), matched by title
  [exit 0 after 1.555778099s]
  t+1.8s:  * #2 APPID [-450, -250] 700x525 "slow-title - Moonlight"
  t+2.1s:  * #2 APPID [-450, -250] 700x525 "slow-title - Moonlight"
  t+2.4s:  * #2 APPID [-450, -250] 700x525 "slow-title - Moonlight"
  t+2.7s:  * #2 APPID [-450, -250] 700x525 "slow-title - Moonlight"
  t+3.0s:  * #2 APPID [-450, -250] 700x525 "slow-title - Moonlight"
  t+3.3s:  * #2 APPID [-450, -250] 700x525 "slow-title - Moonlight"
  t+3.6s:  * #2 APPID [-450, -250] 700x525 "slow-title - Moonlight"
  t+3.9s:  * #2 APPID [-450, -250] 700x525 "slow-title - Moonlight"
  t+4.2s:  * #2 APPID [-450, -250] 700x525 "slow-title - Moonlight"
$ hubd list
   slow-title       Slow Title               UP [open]
$ driftwm msg state
windows 3
  * #2 com.moonlight_stream.Moonlight [-450, -250] 700x525  "slow-title - Moonlight"
    #0 com.moonlight_stream.Moonlight [450, 250] 700x525  "desk-1 - Moonlight"
    #1 com.moonlight_stream.Moonlight [-450, 250] 700x525  "ai-1 - Moonlight"
fullscreen 0
pinned 0
$ hubd feed (first line)
{"text":"4 of 4 up","class":"ok","tooltip":"all machines up\nlast check round took 1ms"}
```

What hubd does: the window titled "Moonlight" is **not** a candidate (the title is not the rendered `title_match`), so it keeps looking; when the title changes, the window becomes the match, is placed at home, and the open answers `matched by title` after 1.56 s. Nothing was moved while the title was still wrong (the window stayed at the compositor's spot (-425, 225)). A title that stays wrong would end in `no window appeared … its title is "Moonlight", not "…": check title_match`, or, if the viewer is still running, in the late-window state (unit test `TestTitleMatchNoWindowWithTheRightTitleSaysSo`). **Not tested live:** a title that changes *after* the window was matched; hubd then goes on using the window id (UNKNOWN for Moonlight).

### 16.4 Two windows with the same title

One launch of the viewer `twin` makes two foot windows titled "twin - Moonlight".

```
$ hubd open --socket S twin   (one launch makes two windows titled "twin - Moonlight")
2 new windows are titled "twin - Moonlight" while opening Twin Windows (#4 "com.moonlight_stream.Moonlight", #3 "com.moonlight_stream.Moonlight"); I cannot tell which one is its window, so I left them all alone and placed and recorded nothing
  [exit 1]
$ driftwm msg state
windows 5
  * #4 com.moonlight_stream.Moonlight [-400, -300] 700x525  "twin - Moonlight"
    #0 com.moonlight_stream.Moonlight [450, 250] 700x525  "desk-1 - Moonlight"
    #1 com.moonlight_stream.Moonlight [-450, 250] 700x525  "ai-1 - Moonlight"
    #2 com.moonlight_stream.Moonlight [-450, -250] 700x525  "slow-title - Moonlight"
    #3 com.moonlight_stream.Moonlight [-425, -275] 700x525  "twin - Moonlight"
fullscreen 0
pinned 0
layers 0
$ hubd list
! 2 new windows are titled "twin - Moonlight" while opening Twin Windows (#4 "com.moonlight_stream.Moonlight", #3 "com.moonlight_stream.Moonlight"); I cannot tell which one is its window, so I left them all alone and placed and recorded nothing
! WARNING: 2 windows are titled "twin - Moonlight" (Twin Windows); hubd knows none of them; close the extra one by hand
x forget unknown window for Twin Windows (twin)
   twin             Twin Windows             UP [window not identified]
$ hubd feed (first line)
{"text":"4 of 4 up","class":"alert","tooltip":"2 new windows are titled \u0026quot;twin - Moonlight\u0026quot; while opening Twin Windows (#4 \u0026quot;com.moonlight_stream.Moonlight\u0026quot;, #3 \u0026quot;com.moonlight_stream.Moonlight\u0026quot;); I cannot tell which one is its window, so I left them all alone and placed and recorded nothing\nWARNING: 2 windows are titled \u0026quot;twin - Moonlight\u0026quot; (Twin Windows); hubd knows none of them; close the extra one by hand\nlast check round took 1ms"}
$ hubd open --socket S twin   (again)
a viewer for Twin Windows was started but its window could not be told apart; close it by hand, then run: hubd forget twin
  [exit 1]
(nothing was moved: both windows are still at their opening spot above)
```

hubd left both windows where driftwm put them ((-425, -275) and (-400, -300), driftwm's own stagger, not hubd's home for `twin`), recorded nothing, put a plain message and a warning in the menu and tooltip (the tooltip text is HTML-escaped for Waybar: `&quot;` appears as `\u0026quot;` in the feed line), and a second `hubd open twin` is refused until `hubd forget twin`.

After a `kill -9` and restart of hubd with those two windows still there, and then an extra window titled like an already-open machine:

```
$ kill -9 hubd; rm the record file; start hubd again   (windows: ai-1, desk-1, slow-title, and two "twin - Moonlight")
$ hubd list
! WARNING: 2 windows are titled "twin - Moonlight" (Twin Windows); hubd knows none of them; close the extra one by hand
   ai-1             AI Box                   UP [open]
   desk-1           Desktop One              UP [open]
   slow-title       Slow Title               UP [open]
   twin             Twin Windows             UP
$ pgrep -c -x foot (nothing started; 5 windows = 5 foot, each twin pair is two)
5

$ foot --app-id=com.moonlight_stream.Moonlight --title="desk-1 - Moonlight" -- sleep infinity &    (an extra window with an adopted machine title)
$ hubd list
! WARNING: 2 windows are titled "desk-1 - Moonlight" (Desktop One); hubd knows window #0; close the extra one by hand
! WARNING: 2 windows are titled "twin - Moonlight" (Twin Windows); hubd knows none of them; close the extra one by hand
   desk-1           Desktop One              UP [open]
$ hubd open --socket S desk-1
went to the open window of Desktop One
  [exit 0]
$ driftwm msg state
windows 6
  * #0 com.moonlight_stream.Moonlight [450, 250] 700x525  "desk-1 - Moonlight"
    #1 com.moonlight_stream.Moonlight [-450, 250] 700x525  "ai-1 - Moonlight"
    #2 com.moonlight_stream.Moonlight [-450, -250] 700x525  "slow-title - Moonlight"
    #3 com.moonlight_stream.Moonlight [-425, -275] 700x525  "twin - Moonlight"
    #4 com.moonlight_stream.Moonlight [-400, -300] 700x525  "twin - Moonlight"
    #5 com.moonlight_stream.Moonlight [-375, -325] 700x525  "desk-1 - Moonlight"
fullscreen 0
pinned 0
```

hubd adopted the three machines that have exactly one window with their title (`ai-1`, `desk-1`, `slow-title`), **did not** adopt `twin` (two windows) and showed the warning, started nothing (5 foot processes before and after), and when an extra window titled "desk-1 - Moonlight" was added the warning appeared for `desk-1` too (`hubd knows window #0`) while `open desk-1` still went to window #0. Screenshot in words (after the extra window): a pile of overlapping title bars at the left, each a "- Moonlight" window — "slow-title", two "twin", and "desk-1" in front with foot's locale warning — and one more empty window at the top right.

### 16.5 What this did not show

- Moonlight, Sunshine, or a real network were not involved; the app-id and titles are what the source says (`docs/viewers-research.md`).
- The case "a late title-matched window adopted in the late state" and "the late comparison lock does not apply" are covered by unit tests only (`TestTitleMatchLateWindowIsAdoptedAndDoesNotLockComparisonViewers`).
- Waybar and wofi were not started in this run; the feed line was read with `hubd feed`.

**Note (2026-10-03, later): why hubd passes `--height` and not `--lines`.** wofi 1.4.1 with `--lines 12` showed only its search box under driftwm's real display backend (a Wayland layer-shell surface). Evidence (`WAYLAND_DEBUG=1` on the guest): wofi asks the compositor for 720x1, then 720x5, prints `Gtk-CRITICAL ... gtk_widget_set_size_request: assertion 'height >= -1' failed` (its code computes `max_height * lines + 5` while `max_height` is still 0, then asks for a list height of 5 minus the search box's height, a negative number), and later asks for 720x281, which driftwm grants (`configure(720, 281)`), yet no row is drawn. With `--height 480` the surface is sized once (720x480) and the list is drawn. The first investigation (above) ran wofi on a virtual X display, where wofi does not use layer-shell and none of this code runs, so it never saw the problem. This is a wofi 1.4.1 behaviour (the negative request is in wofi's own source, `src/wofi.c`); it is not known whether newer wofi fixes it. TESTED in `cmd/hubd/menu_test.go` (hubd never passes `--lines`) and in the hub image test (the list is drawn).

---

## 17. The open marker (2026-10-05)

**Owner decision (already made):** the list marks every machine whose window is open with a **filled dot** and every other machine with an **empty dot**. Open machines stay in their role groups; the order inside a group is not changed by the marker.

### 17.1 What changed

| | Before | After |
|---|---|---|
| Machine line | `   ai-1             AI Box                   UP [open]` | ` ● ai-1             AI Box                   UP` |
| Machine line, window not open | `   desk-2           Desk Two                 UP` | ` ○ desk-2           Desk Two                 UP` |
| Guest line (6 spaces of indent before) | `      guest-1          Guest 1                  UP [open]` | `    ● guest-1          Guest 1                  UP` |

- **Marker characters:** `●` U+25CF BLACK CIRCLE (open) and `○` U+25CB WHITE CIRCLE (not open), constants `hub.MarkerOpen` and `hub.MarkerClosed` in `internal/hub/view.go`.
- **Where:** the marker takes the last two columns of the indent (marker, one space), so the id, name and status columns are exactly where they were. Nothing else on the line moved.
- **The old ` [open]` text is gone.** The dot replaces it (two signs for one fact would be noise). Owner: say if you want both.
- **What "open" means:** hubd holds a window for the machine (the same fact that makes a pick go to that window instead of starting a viewer). A machine in the late-window state (`[waiting for its window]`) or with an unidentified window is not open until its window is recorded.
- **Every machine line has a marker**, also the hub itself and a machine with nothing to open: those always show the empty dot. (Owner: say if the hub line should have no dot.)
- **Down machines group, search results and the flat list** use the same line, so they carry the marker too.
- **Order:** unchanged. Down first, then not checked, then up; inside those, inventory order. An open machine does not move. (No strong reason to change it was found, so no change is proposed; TESTED in `TestMarkerOpenIsFilledClosedIsEmpty`.)
- **The bar item's feed (`hubd feed`) is not changed.** It carries counts and the down machines' names, not a machine list, so there is nothing to mark. (Question for the owner below.)

### 17.2 Search and picking still work (checked how wofi matches and how the line is read back)

- **Typed search** (`? search by id or name...`) is done by hubd, not by wofi: hubd filters machines by id and name only (case-insensitive) and sends wofi the matching lines. The marker is not part of what is searched. TESTED: `TestSearchStillMatchesByNameAndIdWithTheMarker` (searching `box` finds both "A Box" and "B Box", each with its own marker; searching for a dot character finds nothing).
- **wofi's own filter** (typing into the menu while it shows the list) matches the text of each line (wofi 1.4.1 default mode "contains", started with `--insensitive`; BELIEVED from its manual, not read in source). The marker does not hide a name from it. TESTED in the hub image test H3c: typing `two` (only the *name* of `desk-2`, "Desk Two") leaves exactly the line `○ desk-2   Desk Two   UP`; Enter picks it and hubd opens `desk-2` at its home.
- **Reading the picked line back (`Pick`):** wofi prints the whole line. hubd takes the line, requires it to be indented, takes the first word, and if that word is `●` or `○` it skips it; the next word is the machine id. A line without a marker (a script, an older menu) is still accepted. A marker alone, a marker with an unknown id, a marker line with no indent, or a marker followed by a name instead of an id is ignored. TESTED: `TestPickAMarkedLineGoesToTheWindowOrOpens`.
- **Font:** the dots are in DejaVu Sans Mono (the font of `wofi.css`); TESTED on the hub image's screenshots that both draw as a dot, the filled one a little bigger than the empty one. Another font could lack them (UNKNOWN).

### 17.3 A pick of an open machine, and windows that belong to no machine

- **Focus and view:** a pick of an open machine calls driftwm's `focus --id` (which raises and focuses the window and pans the view to it unless it is already fully visible, `docs/driftwm-findings.md` section 4). It starts no viewer. TESTED with the real driftwm in the hub image (H3c): with `ai-1` far off screen (view at x=3000, window at x=0) a click on its row made `ai-1` the focused window and the view moved to x=0; with `desk-1` placed exactly over `ai-1` and focused, a click on the `ai-1` row made `ai-1` the focused, top window again (driftwm prints the windows top first). Two viewers (foot) were running before and after.
- **Windows of no machine never appear in the list:** the list is built from the inventory only, so the menu program (wofi), the bar (Waybar) and any other window cannot be in it. They are also never taken for a machine's window (the bar and the menu are layer surfaces, not windows, in driftwm: `driftwm msg state` lists the bar under `layers`). TESTED: `TestWindowsOfNoMachineNeverAppearInTheList` (windows named `wofi`, `waybar`, `foot` and `hubos-nobody` are in driftwm; the list has none of them and no machine looks open).

### 17.4 Tests

Unit tests (`internal/hub/marker_test.go`): `TestMarkerOpenIsFilledClosedIsEmpty`, `TestMarkerWindowClosedByHandTurnsEmpty` (also `hubd end`), `TestMarkerWindowAppearsLate`, `TestMarkerWithTheDuplicateTitleWarning` (both a known window with a duplicate, filled, and two same-titled windows hubd cannot tell apart, empty), `TestPickAMarkedLineGoesToTheWindowOrOpens`, `TestSearchStillMatchesByNameAndIdWithTheMarker`, `TestWindowsOfNoMachineNeverAppearInTheList`, `TestPickingAnOpenMachineFocusesItsWindowAndMovesTheView` (the fake driftwm models focus: raise, and pan unless in view). Existing tests that quoted machine lines were changed to the new lines (`view_test.go`, `ipc_test.go`).

Hub image test H3c (`tools/image/hub_test.go`, build tag `qemu`): two fake viewer windows (ai-1 and desk-1, homes 3000 canvas units apart), a third machine (desk-2) not open; the bar click opens the menu; the list (screenshot and `hubd list`) shows two filled dots and the others empty; a click on the open ai-1 row while its window is off screen, then while it is behind desk-1's window, focuses it and brings it into view; `hubd end desk-1` turns its dot empty; typing `two` and Enter opens desk-2. The test inventory (`image/config/hub/inventory.toml`) got two machines for this, `desk-1` and `desk-2` (role desktop, fake listeners started by the test). The runs are in `docs/image.md` style RESULTS blocks, given to the lead with this change.

### 17.5 Not shown, and questions for the owner

- Not shown: a real Moonlight/virt-viewer window (only the foot stand-in); the marker on a projector; a font without the dots; a window that the owner moved by hand and the pick (the view still goes to it, by the same driftwm call, but only the cases above were run); a pick of an open machine whose window is behind a *fullscreen* window.
- Question: keep ` [open]` as well as the dot? (Now: dot only.)
- Question: should the hub's own line have no dot? (Now: empty dot.)
- Question: should the bar item show how many windows are open (for example in the tooltip)? Nothing was added, because it is a design choice.
