# watcher: TEST STAND-IN. NOT the product watcher. The hang rule is not built.

A small Go program (`go build -trimpath -ldflags="-s -w"`, static) used only by the init comparison (`docs/proposals/init-comparison-run.md`, follow-up 4) to measure what an init does for a hung compositor once a separate watcher exists. It is **not** the product watcher and decides nothing about where the product's hang rule lives.

**Probes and rule (one line):** it probes the compositor twice per round, with driftwm's socket `state` request and with a Wayland `wl_display.sync` round trip, and applies the owner's rule of `docs/proposals/hub-stability.md` section 9.1 (a round fails only when BOTH probes fail; kill after 4 failed rounds in a row and at least 30 s since the first failed round; at most 3 kills in 10 minutes; never reboot anything).

**Other things it does.**
- The hubd `ping` is still sent every round and is written in the log as `hubd(info only)`; it takes no part in the decision.
- It restarts nothing: the init restarts what it killed. It has no restart ladder, no alert and no conductor.
- `Probe` (the `state` request) is copied unchanged from `../go/probe.go`; `ProbeSync` speaks the Wayland wire format by hand (see its comment).
- The kill budget and the "capped" state file: see the second half of this README (added with follow-up 4, item 2).

`go test ./tools/bench/init/prototypes/watcher/` tests the rule (4 rounds and 30 s, both probes must fail and one failing is not enough, an answer resets the streak, 3 kills in 10 minutes and the budget coming back) and the sync probe against a fake server.

The program of follow-up 3 (rule "compositor and hubd probes", and the `-mode compositor` variation) is kept unchanged in `../watcher-fu3/`; the rows `s6rc-par-wd`, `runit-wd`, `s6rc-par-wdc`, `runit-wdc` were measured with it.
