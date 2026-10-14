# PROPOSAL: one command for a person to clear a stopped service or the watcher latch (design note only)

**Status: PROPOSAL / DESIGN NOTE. Nothing is built. Nothing here is in the image, in hubd or in the product.** Written 2026-10-14 for the owner (init comparison, follow-up 7, Part D). `HUB-OS.md` wins if this file disagrees with it. Labels: **TESTED** (a command was run; the log is named), **SOURCE** (read in a file, named), **BELIEVED**, **UNKNOWN**. "The command" below is a working description only; the name, the place and the spelling are not decided (see "What is still open").

## 1. What exists today (test hooks in the init-comparison bench, not product)

| Hook | What it does | Where it is | Label |
|---|---|---|---|
| `hub-retry SERVICE` (follow-up 6, `s6rc-par-wd4a`) | removes the alert file of the service, clears the supervisor's death tally (`s6-svdt-clear`), starts it (`s6-svc -u`) | `tools/bench/init/candidates/s6rc-par-wd4a/rootfs/bin/hub-retry` | TESTED (run log, follow-up 6, Part C) |
| `hub-retry SERVICE` (follow-up 7, `s6rc-par-wd4b`) | the same, and for every dependent that has its own alert, in dependency order, each after the one before is up and ready; refuses (says so, does nothing) when a parent of SERVICE is still stopped | `.../s6rc-par-wd4b/rootfs/bin/hub-retry` | TESTED (run log, follow-up 7, Part C) |
| `watcher reset` | empties the kill-budget file, then removes the latch file, then removes the state file (`capped ...`) of the test watcher | `tools/bench/init/prototypes/watcher/watcher.go` (`Reset`) | TESTED (follow-ups 5 and 6) |

A person has to run these two today as root in a shell of the virtual machine; who may run them and from where is not designed.

## 2. One command, one name

The person types (or clicks) one thing and a service name; they should not need to know that there are an alert file, a death tally, a latch and a budget. Three cases, all with the same shape:

1. **A service stopped by the restart limit** (5 deaths in 60 s; alert file written by the finish-script helper). The command: removes the alert, clears the death tally, starts the service, waits until it is up and ready (a timeout of about 20 s). This is what `hub-retry` does.
2. **The watcher latch** (the watcher wanted a 4th kill of the compositor within 10 minutes and stopped; the compositor may be frozen, not dead). The command: clears the latch, the "capped" message and the kill budget (as `watcher reset` does; the budget is emptied, so the watcher may kill up to 3 more times in the next 10 minutes). The watcher then kills a frozen compositor within one round (TESTED: 0.4 to 4.6 s in follow-up 5; 1.28 s in follow-up 6 case iv). Which name this uses is open (below): the latch belongs to the compositor, so the name `driftwm` is the natural one, with `watcher` accepted too.
3. **A dependency that took its dependents down** (the seatd case: seatd stops after 5 deaths, driftwm loses its seat and is stopped by the same rule, so two alerts exist). One command with the name `seatd` retries seatd, and then every dependent that has its own alert, in dependency order (TESTED in follow-up 7, Part C). Retrying only the dependent while its parent is stopped does nothing and says which parent to retry.

## 3. What it prints (plain sentences, one per line, in the owner's style; exit code in brackets)

- **Nothing is wrong** (no alert, no latch, the service runs): `driftwm: nothing to do, it is running.` [exit 0]. Nothing is started, stopped or removed (TESTED for the bench hook: follow-up 7, Part C, "harmless" calls).
- **Name unknown:** `foo is not a service on this machine. Known: seatd udevd dbus driftwm waybar hubd.` [exit 2]. (The bench hook prints a shorter line.)
- **Stopped by the limit, retry works:** `driftwm was stopped (tried 5 times in 60 s). Started it again; it is up.` and one line per dependent that was also retried. [exit 0]
- **Parent still stopped:** `driftwm waits for seatd, which is stopped (seatd stopped: tried 5 times in 60 s). Nothing done. Retry seatd first.` [exit 1] (TESTED as a bench hook, twice in a row, no loop.)
- **Still unhealthy after the retry:** `driftwm was started but is not healthy after 20 s: <the supervisor's one-line state>. Its alert will come back if it crashes 5 times in a minute again. Look at the service log: /var/log/driftwm/current.` [exit 1]. The alert file is not removed a second time and nothing is retried in a loop.
- **Latch cleared:** `The watcher was stopped after 3 kills in 10 minutes. Cleared. It will look at the compositor again now.` [exit 0]

## 4. Where a person could reach it on the hub (options only; nothing decided)

1. **A command on the hub's recovery terminal or over ssh** (the recovery console is the one exception to "no terminals", `HUB-OS.md`). Always available, also when the compositor is stopped. Needs the person at a keyboard or on ssh.
2. **A hubd command** (for example a sub-command of the hubd binary that talks to hubd's socket). hubd is meant to stay up when the compositor is stopped (see the follow-driftwm mode in the run log, follow-up 7, Part B); the bar and the phone page could call the same thing.
3. **A bar button** (Waybar custom module fed by hubd, `HUB-OS.md` "Panel"). The bar dies with the compositor (`HUB-OS.md`, "the bar dies with the compositor"), so this cannot be the only way for the driftwm case.
4. **A button on the phone web page served by the hub** (allowed, `HUB-OS.md` "Phone access"). Works when the compositor is stopped if hubd or the page server runs.
5. **The kernel text console message** ("the compositor gave up", already drawn by the image's driftwm finish script, SOURCE `image/machines/hub/rootfs/etc/s6/sv/driftwm/finish`) could print the command to type.

## 5. What is still open (questions for the owner are in the run log, follow-up 7)

- The name and the place of the command (section 4), and whether the bar, hubd and the phone page call it or implement it again.
- Who may run it. The supervisor's control sockets belong to root; the hub's desktop runs as the user `hub`. UNKNOWN how the permission would be given.
- The alert file: path (`/run/hubos/alerts/SERVICE` in the bench), format (one sentence), and who reads it. **Not built: hubd, the bar or the phone page reading it.**
- **A gap between the bench helper and the image:** the image's `follow-driftwm` keeps hubd running without the compositor only when the marker `/run/hub/driftwm-gave-up` exists (written by the image's own driftwm finish script and removed by its run script); the bench helper writes only the alert file (run log, follow-up 7, Part B). One of them has to learn the other's file.
- The latch case: one name for both the service and the watcher, or two names. Whether the command empties the kill budget (the test hook does) or only the latch.
- The restart-limit helper counts on the supervisor's death tally, which uses the wall clock (run log, follow-up 7, Part A): a clock step during a storm gives a wrong answer in two of the three jump cases. Open until the owner decides whether to change it.
- What the s6-rc database itself thinks of a service stopped with exit code 125: UNKNOWN (not queried). A later `s6-rc` change might act on it.
- Logging: where the command writes what it did (the bench hooks only print).
- A service stopped on purpose by a person (not by the limit) is a different path; the command must not undo it (BELIEVED to be handled by "no alert: nothing to do"; the bench hook prints the state and does nothing, TESTED for a running service only).
- Confirmation: none is proposed (the command only starts what the rule stopped); the owner may want one for the latch case because the budget is emptied.
