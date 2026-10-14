# watcher-fu3: the follow-up 3 test watcher, kept UNCHANGED. TEST STAND-IN. NOT the product watcher. The hang rule is not built.

**Kept for reproducibility only.** Its probes are the compositor `state` request and the hubd `ping`; that pairing is NOT the owner's rule (section 9.1 of `hub-stability.md` pairs two probes aimed at the compositor). The rows `s6rc-par-wd` and `runit-wd` (mode `both`) and `s6rc-par-wdc` and `runit-wdc` (mode `compositor`) were measured with this program. The current program is `../watcher/`.

A small Go program (`go build -trimpath -ldflags="-s -w"`, static) used only by the init comparison (`docs/proposals/init-comparison-run.md`, follow-up 3, item 3) to measure what an init does for a hung compositor once a separate watcher exists. It is **not** the product watcher and decides nothing about where the product's hang rule lives.

**What it does (and nothing else).** Every second it asks the stand-in compositor (`/run/dw/wayland-1`, request `state`) and the stand-in hubd (`/run/hubos/hubd.sock`, request `ping`) and logs one line per round. After **4 failed rounds in a row and at least 30 seconds since the first failed round**, with **both** probes failing, it sends SIGKILL to the compositor process (found by its `comm` name). At most **3 kills in 10 minutes**; the fourth is logged as `CAP` and not made. It never reboots anything and restarts nothing: the init restarts what it killed. It has no restart ladder, no alert and no conductor. It is supervised by the init like any other service.

**The probes** are not new: `Probe` is copied unchanged from `../go/probe.go` (the brain prototype). The brain asks the compositor with it; for hubd the same function is used with the request `ping` that the benchmark harness already sends to hubd (`hubsim/chaos.go`, `why()`).

**`-mode`.** `both` (default, the owner's wording): a round fails only when both probes fail. `compositor`: a round fails when the compositor probe fails, whatever hubd says. The second mode is a variation that exists only because of a property of the stand-ins (see the run log): the stand-in hubd keeps answering `ping` while the compositor is frozen, so "both fail" never happens for a frozen stand-in compositor.

`go test ./tools/bench/init/prototypes/watcher/` tests the rule (4 rounds and 30 s, both probes, answer resets the streak, 3 kills in 10 minutes and the budget coming back).
