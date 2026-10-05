# Experiments: window layouts and file transfer (research, not the production features)

Written 2026-10-05 for `docs/proposals/driftwm-layouts.md` and `docs/proposals/file-transfer.md`. Nothing here is part of an image or of hubd. Everything runs in a work folder (`LFT`) outside the repository; nothing is installed on the machine (packages are unpacked with `dpkg -x`; pywayland goes in with `pip install --target`).

`results.txt` is the output of one complete run (`t/runall.sh`; its section L10 was added afterwards by running `t/t14.sh` with the debug and the release build). Its sections: **L** = driftwm session and layouts, **D** = drag and drop, **F** = file transfer.

## What is here

| Path | What |
|---|---|
| `setup.sh`, `ud.sh`, `getxfer.sh`, `pip.sh`, `gen.sh`, `build-go.sh` | unpack the libraries and tools into `$LFT/a`, the libudev headers, rsync and the sftp tools, pywayland, the two protocol bindings, and build the Go programs |
| `clone.sh`, `buildrel.sh`, `builddebug.sh` | fetch driftwm and build it at the pinned commit `352333a8fa1b22171492d4b71a54102045c9a19d` (release 7 min, debug 3 min). `clone.sh` stops if it cannot reach that commit. |
| `t/dw.sh` | the nested-driftwm environment: a headless sway as parent, driftwm nested on it, a private `XDG_STATE_HOME` and `XDG_DATA_HOME` |
| `t/up.sh`, `t/vp-up.sh` | start the sway parent; start the pointer injector |
| `t/t1.sh` ... `t/t14.sh`, `t/rel123.sh` | layouts and session tests (L1 to L10 in `results.txt`) |
| `t/t5.sh`, `t/t6.sh`, `t/run56.sh`, `t/t10.sh`, `t/t13.sh` | drag and drop and overlay tests (D1 to D4) |
| `t/tclip.sh`, `t/tft.sh`, `t/trs.sh`, `t/tconf.sh`, `t/te2e.sh` | file clipboard type, one-time tickets, rsync and sftp, conformance test, end to end (F1 to F5) |
| `dndclient.py`, `overlay.py`, `vptr.py`, `wlr-*.xml` | test-only Wayland clients and the (hand-written, unofficial) protocol XML for two wlroots protocols |
| `ft/main.go`, `ft/adapter/main.go`, `ft/conform/main.go` | the ticket server and signer (172 lines), the reference adapter with faults (170), the conformance test (306); standard library only |

## How to run

```
export LFT=/some/empty/folder          # a work folder outside the repository, about 3 GB with the builds
cp -r tools/image/experiments/layouts-and-file-transfer/. $LFT/
cd $LFT
bash setup.sh && bash ud.sh && bash pip.sh && bash gen.sh && bash getxfer.sh
bash clone.sh
bash buildrel.sh && bash builddebug.sh && bash build-go.sh
bash t/up.sh && bash t/vp-up.sh          # sway parent and pointer injector, left running
DWBIN=$LFT/target-rel/release/driftwm bash t/runall.sh > run.log 2>&1
```

`ulimit -c 0` is set in the scripts; run them outside the repository so no core file can land in it. Kill your own processes by process id when finished (the scripts write `t/*.pid`).

## Known limits

- The parent is a headless sway, not the virtual X display of `docs/driftwm-findings.md` (that one does not start here: `/usr/bin/xkbcomp` is missing).
- Other helpers' processes (`hubos-sNN` windows) can show up in `pgrep` lists; the scripts filter them.
- `wlr-virtual-pointer-unstable-v1.xml` and `wlr-layer-shell-unstable-v1.xml` were written from memory of the protocols and are **not** the official files.
- The timings are from a shared machine and mean nothing for a real network.
