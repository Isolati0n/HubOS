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

`hubd menu` also takes `--wofi PROGRAM`, `--style FILE` (wofi style, used only if the file exists; default `/etc/hubos/wofi.css`), `--width PIXELS` (720), `--single-click` (the default: one click picks) and `--no-single-click` (pick with a double click, like wofi's own default).

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

[[viewer]]
id        = "fake"
programs  = ["moonlight", "spice", "vnc", "ssh", "files"]      # inventory "open" entries this viewer serves
command   = ["foot", "--app-id={app_id}", "--title={title}", "--", "sleep", "infinity"]
sets_name = true                                                # the command makes the window carry {app_id}
window_wait = "10s"                                              # optional: how long to wait for this viewer's window (default: the hub's 10 s)
```

- `command` is a list: the first entry is the program (fixed text, no placeholders); each other entry is one argument.
- Placeholders: `{id} {name} {address} {port} {user} {share}` from the inventory, `{app_id}` (= `hubos-<id>`), `{title}` (= the machine's name). A placeholder for a value the machine lacks makes that machine unopenable with a message; it is never guessed. Substituted text is never scanned again.
- Checked at start-up: unknown fields, unknown placeholders, empty `programs`, `none` as a program, one program served by two viewers, `sets_name = true` without `{app_id}`. A bad file stops `hubd serve` before anything starts.
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
- Exactly one candidate → matched. **More than one → nothing is moved, focused, closed or recorded**, the machine becomes `unmatched`, and the message says what appeared. Zero → "no window appeared within …".
- driftwm's `state` has no process id, so matching by process is not possible.

### Placement (design item 4)

After a match: `move` to the machine's `home` (window centre, Y up — `docs/inventory-format.md`), then `focus`. If the window is taller than the viewport minus `--bar-height`, or wider than the viewport, it is shrunk with `resize` and moved again (so a placed window is never under the bar). With a bar height set, `hubd` waits for driftwm's pan animation to stop, reads the camera, and says so in the message if it is not `home y + bar height / 2`. If the home spot is taken by another window, the new one is placed on top (the home is fixed; there is no nudging).

### Ending

`end` asks driftwm to close the window (`close --id`), waits up to 3 s for it to go, then drops the record. If the window stays (the program may be asking something), it says so and forces nothing. It never kills a process and never contacts the machine.

### After restarts (design item 3)

| What restarts | What happens | Test |
|---|---|---|
| `hubd` only | New `hubd` adopts windows: from the record file if it belongs to the same driftwm (the entry must still exist with the same app_id and title), and windows named `hubos-<known id>` if exactly one has that name. It starts nothing. | TESTED live: `kill -9`, start again, `foot` count unchanged, list shows `[open]`, `open` goes to the window. Unit: `TestRestartAdoptsAndOpensNothing` |
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
   ai-19            ai 19                    DOWN         machine line: the first word is the id
   - Guests of vmhost-1 (50 machines)       nested under the host (3 more spaces)
      guest-1          Guest 1                  UP [open]
```

- A **Down machines** group is at the top (cap `--down-max`, 50, then a line saying how many more); it can be folded like any group. Groups by role (Hub, Gaming, AI, Desktop, NAS, Backup NAS, VM host) follow; guests nested under their host; **down first** inside a group, then not checked, then up.
- **Nothing is hidden without a line that says so**: a folded group shows its heading with the count and how many are down; a capped list ends with a `! N more …` line.
- **Typed search and the 1000-line cap change the approved design.** The design said wofi's own search would find any machine. It cannot: wofi only searches the lines it is given, and it cannot be given thousands of lines (section 7.5). So `? search by id or name...` asks for text, hubd filters (case-insensitive, id or name), and wofi shows the matches. This was approved by the owner.
- A group with more than `--fold` machines starts folded. Fold choices are kept in `hubd`'s memory (gone when it restarts).
- No list has more than `--list-max` machine lines; the rest are replaced by a `! N more … not shown` line.
- **Pick** (`hubd pick LINE`): acts only if the line is exactly a switch line, a group heading, or an **indented** machine line whose first word is an existing id. Anything else — a heading with an unknown name, an unknown id, an id without indent, a `!` line, an empty line — does nothing. TESTED: `TestPickIgnoresHeadingsUnknownAndEmptyLines` (a heading, an unknown line, an empty line, and six more).
- Status texts: `UP`, `DOWN`, `checking...`, `NOT CHECKED (no port in the inventory)`, `NOT CHECKED (nothing to open)`, `NOT CHECKED (out of file handles)`, `THIS HUB`, plus ` [open]`, ` [opening]`, ` [window not identified]`.
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

- **Real viewers.** The command lines, window names (app_id and title), whether they accept a chosen name, how they behave on a close request, and whether closing one leaves the Sunshine session alive: all UNKNOWN. Only `foot` was used. `viewers.toml` has no Moonlight, virt-viewer or Remmina entry for that reason.
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
