# Desktop experiment (EXPERIMENT; not part of the default test or the main image build)

Question: does driftwm's real-display path (DRM, udev, libinput, libseat) start in QEMU with software rendering? Results: `docs/proposals/phase-b-desktop.md`.

Nothing here is run by `go test ./tools/image` or by `tools/image/build-*.sh` with the default machine. Everything needs root (mmdebstrap, mount, mkfs), network (Ubuntu snapshot, crates.io, github.com) and a Rust toolchain on the host. Nothing is installed on the host; everything goes under `$WORK`.

## Files

| File | What it is |
|---|---|
| `desktop.build` | machine file (kernel fragment, package list, snapshot, services `console seatd driftwm foot`) |
| `desktop.frag` | kernel options added to the tiny base kernel (DRM, virtio-gpu, USB, HID, evdev, epoll and friends) |
| `desktop.list` | runtime packages (seatd, libinput, libudev1, Mesa llvmpipe and lavapipe, Wayland and xkbcommon libraries, fonts, foot, Waybar, dbus-daemon) |
| `build-driftwm.sh` | builds driftwm (release) at the pinned commit inside a throw-away build root |
| `build-root.sh` | desktop root image (list + `image/rootfs` + `overlay/` + driftwm + hubd + efibootmgr) |
| `build-disk.sh` | GPT disk with 768 MiB root slots; `NO_MKUDEVDB=1` leaves the udev-database workaround off |
| `overlay/` | s6 services `seatd`, `driftwm`, `foot`; `mkudevdb` (the workaround); `start-bar`; fake inventory, Waybar config |
| `session.py` | starts QEMU (TCG, UEFI, virtio-vga, USB keyboard and mouse or tablet, serial on a pipe, monitor socket); `screendump`, `sendkey`, `mouse_move`, `mouse_button` |
| `scenario.py` | the scripted runs: `noworkaround`, `full`, `bar` |

## Order (commands used; `WORK` is any empty folder with about 8 GB free)

```
export WORK=/tmp/dk MACHINE=tools/image/experiments/desktop/desktop.build
D=tools/image/experiments/desktop
tools/image/build-base.sh                      # Ubuntu base from desktop.list, pinned snapshot
tools/image/build-kernel.sh                    # kernel 6.12 with desktop.frag
$D/build-driftwm.sh                            # about 5 minutes (4 CPUs)
$D/build-root.sh $WORK/out/desktop-root.sqsh
$D/build-disk.sh $WORK/out/desktop-root.sqsh $WORK/out/kernel-a.efi $WORK/vm
python3 $D/scenario.py full $WORK $WORK/vm $WORK/shots
python3 $D/scenario.py bar  $WORK $WORK/vm $WORK/shots-bar
NO_MKUDEVDB=1 $D/build-disk.sh $WORK/out/desktop-root.sqsh $WORK/out/kernel-a.efi $WORK/vm-nowa
python3 $D/scenario.py noworkaround $WORK $WORK/vm-nowa $WORK/shots-nowa
```

`session.py` also works alone: `python3 -i session.py WORK DISKDIR OUTDIR` gives an object `s` with `s.sh()`, `s.screendump()`, `s.key()`, `s.move()`.
The QEMU programs are taken from `$WORK/tools/root` (the build tools unpacked as in `tools/image/fetch-tools.sh`).
