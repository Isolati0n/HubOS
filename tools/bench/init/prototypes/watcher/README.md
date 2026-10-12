# watcher: TEST STAND-IN. NOT the product watcher. The hang rule is not built.

A small Go program (`go build -trimpath -ldflags="-s -w"`, static) used only by the init comparison (`docs/proposals/init-comparison-run.md`, follow-up 4) to measure what an init does for a hung compositor once a separate watcher exists. It is **not** the product watcher and decides nothing about where the product's hang rule lives.

**Probes and rule (one line):** it probes the compositor twice per round, with driftwm's socket `state` request and with a Wayland `wl_display.sync` round trip, and applies the owner's rule of `docs/proposals/hub-stability.md` section 9.1 (a round fails only when BOTH probes fail; kill after 4 failed rounds in a row and at least 30 s since the first failed round; at most 3 kills in 10 minutes; never reboot anything).

**Other things it does.**
- The hubd `ping` is still sent every round and is written in the log as `hubd(info only)`; it takes no part in the decision.
- It restarts nothing: the init restarts what it killed. It has no restart ladder, no alert and no conductor.
- `Probe` (the `state` request) is copied unchanged from `../go/probe.go`; `ProbeSync` speaks the Wayland wire format by hand (see its comment).

**Kill budget in a file (owner decision 4).** The times of the kills made in the last 10 minutes are kept in `/run/hubos/watcher.budget` (`/run` is the in-memory area), so a restarted watcher does not get a fresh budget. Every kill is written at once, by writing a temporary file next to it and renaming it over the real one. Clock: `CLOCK_BOOTTIME` (time since the kernel started): it is not changed when someone sets the wall clock (the fault suite sets it back an hour and forward a day), it can be read again by a new process, and it restarts from zero only at a reboot, which also empties `/run`, so file and clock always belong to the same boot. On start the watcher reads the file; if it is missing or damaged (bad header, a line that is not a time, a time in the future, wrong count, text after the end line) it starts with an empty budget and logs that it did. The 30-second streak is NOT kept: a restarted watcher starts counting the 30 s again.

**State file (owner decision 3).** When the cap blocks a kill, the watcher writes one line `capped <UTC time>` to `/run/hubos/watcher.state`, and removes the file in the first round in which the budget has room again (kills are allowed). This is only the hook a bar would read; there is no bar or panel here.

`go test ./tools/bench/init/prototypes/watcher/` tests the rule (4 rounds and 30 s, both probes must fail and one failing is not enough, an answer resets the streak, 3 kills in 10 minutes and the budget coming back), the budget file (survives a restart, a damaged file does not crash it, entries older than 10 minutes drop out), the state file, and the sync probe against a fake server.

The program of follow-up 3 (rule "compositor and hubd probes", and the `-mode compositor` variation) is kept unchanged in `../watcher-fu3/`; the rows `s6rc-par-wd`, `runit-wd`, `s6rc-par-wdc`, `runit-wdc` were measured with it.
