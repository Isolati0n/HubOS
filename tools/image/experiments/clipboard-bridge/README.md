# Clipboard bridge experiment

An experiment for `docs/proposals/node-helper-api.md`. It is **not** the production node helper and is in no image. It uses only the Go standard library and the `wl-copy` / `wl-paste` programs from the `wl-clipboard` package.

What it shows: moving plain text between two Wayland sessions, one standing in for the hub and one for a node, on loopback, including the "echo" case where a copy on one side comes back from the other side.

## Files

| File | What |
|---|---|
| `bridge.go` | The rules (size, empty, UTF-8, echo guard), the `wl-copy` / `wl-paste` wrapper, `Emit` (used by `wl-paste --watch`). |
| `api.go` | The clipboard part of the node API: `GET` and `PUT /v1/clipboard`. No signing (the signed message is in the proposal and was tested in `../recoveryagent/`). |
| `main.go` | Commands: `node-api`, `hub-watch`, `hub-push`, `hub-pull`, `relay`, `emit`. |
| `bridge_test.go` | Unit tests (no compositor needed) and one test that uses a real compositor when `CB_BIN` and `CB_HUB` are set. |
| `compositors.sh` | Starts or stops two headless `sway` compositors. |
| `scenarios.sh` | Runs every experiment and prints each command with its output. |
| `results.txt` | The output of one full run of `scenarios.sh` (2026-10-04). |

## How to run it (nothing is installed on the machine)

```
mkdir -p /tmp/nh-debs /tmp/nh-root && cd /tmp/nh-debs
apt-get download sway wl-clipboard libwlroots12t64 wayland-utils libevdev2 libinput10 \
  libwayland-server0 libxcb-icccm4 libegl1 libgles2 libglvnd0 libseat1 libdisplay-info1 \
  libliftoff0 libxcb-render-util0 libxcb-xinput0 libxcb-composite0 libxcb-ewmh2 libxcb-res0 \
  libmtdev1t64 libwacom9 libgudev-1.0-0 libgl1-mesa-dri libdrm2 libgbm1 libxcb-xkb1
for f in *.deb; do dpkg -x $f /tmp/nh-root; done
cd <repository>
tools/image/experiments/clipboard-bridge/compositors.sh start   # two headless sway, /tmp/nh-hub and /tmp/nh-node
tools/image/experiments/clipboard-bridge/scenarios.sh           # or one letter: A B C D E F
tools/image/experiments/clipboard-bridge/compositors.sh stop
rm -rf /tmp/nh-debs /tmp/nh-root /tmp/nh-hub /tmp/nh-node
CB_BIN=/tmp/nh-root/usr/bin CB_HUB=/tmp/nh-hub LD_LIBRARY_PATH=/tmp/nh-root/usr/lib/x86_64-linux-gnu \
  go test -count=1 -run Real ./tools/image/experiments/clipboard-bridge/     # needs the compositors running
```

(`apt-get download` only fetches the files; Ubuntu 24.04 packages were used. `go test` without the two variables skips the compositor test and says so.) Scenario `E` ends by killing the node compositor on purpose; start the compositors again before another run.

## The stand-ins, and what they are not

- **Hub** = one headless `sway` 1.9 (wlroots 0.17). The real hub is driftwm; `wl-paste --watch` under driftwm was **not** run.
- **Node** = another headless `sway`. The real display server on the node is wayvnc; **wayvnc was not run**. The `relay` command stands in for "wayvnc on the node plus the viewer on the hub": it watches the node's clipboard and copies every change to the hub's clipboard, and (like wayvnc, `src/data-control.c` v0.10.2) it ignores empty text. It adds no delay, no Latin-1 and no focus rule, which a real viewer may have.
- The API has no signature here.
