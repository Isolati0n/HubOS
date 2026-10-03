# Desktop experiment (EXPERIMENT; not part of the default test or the main image build)

Question: does driftwm's real-display path (DRM, udev, libinput, libseat) start in QEMU with software rendering, and what does a desktop need around it? Results: `docs/proposals/phase-b-desktop.md` (first round: sections 1 to 8; second round: section 9).

Nothing here is run by `go test ./tools/image` or by `tools/image/build-*.sh` with the default machine. Everything needs root (mmdebstrap, mount, mkfs, chroot), network (Ubuntu snapshot, crates.io, github.com) and a Rust toolchain on the host. Nothing is installed on the host; everything goes under `$WORK`.

## Files

| File | What it is |
|---|---|
| `desktop.build` | machine file (kernel fragment, package list, snapshot, services `console udevd seatd driftwm foot`; `BOOT_FAIL_LIMIT=1000` because the experiment has no confirm service to clear the failed-boot counter) |
| `desktop.frag` | kernel options added to the tiny base kernel (DRM, virtio-gpu, USB, HID, evdev, epoll and friends) |
| `desktop.list` | runtime packages (seatd, libinput and `libinput-tools`, Mesa llvmpipe and lavapipe, Wayland and xkbcommon libraries, fonts, foot, Waybar, wofi, dbus-daemon) |
| `build-driftwm.sh` | builds driftwm (release) at the pinned commit inside a throw-away build root (the same build root builds eudev) |
| `build-eudev.sh` | builds **eudev 3.2.14** from its release tarball in that build root; result in `$WORK/out/eudev-root` |
| `build-root.sh` | desktop root image (list + `image/rootfs` + `overlay/` + driftwm + eudev + hubd + fakenode + efibootmgr); makes the user `hub` and the group `seat` |
| `build-disk.sh` | GPT disk with 768 MiB root slots |
| `overlay/` | s6 services `udevd`, `seatd`, `driftwm`, `foot` (the last two run as the normal user `hub`); `start-bar` (second-round step 5); inventory, viewers.toml, wofi style and Waybar config; `wofi-fixed` (a wrapper, see the proposal) |
| `session.py` | starts QEMU (TCG, UEFI, virtio-vga, USB keyboard and mouse or tablet, serial on a pipe, monitor socket); `screendump`, `sendkey` (20 ms hold, 0.5 s gap), `mouse_move`, `mouse_button`, `abs_move` (QMP), PNG difference helpers |
| `steps.py` | the second-round steps: `1` eudev, `2` normal user, `3` hot-plug, `4` focus, `5` hubd/bar/menu, `6` absolute pointer. Prints every command and its output |

(The first round's `scenario.py` and the hand-written udev database script `mkudevdb` are gone: eudev replaced the script, `steps.py` the scenario. They are in git history, PR #29.)

## Order (commands used; `WORK` is any empty folder with about 8 GB free; `tools` is the unpacked build tools of `fetch-tools.sh`)

```
export WORK=/tmp/dk MACHINE=tools/image/experiments/desktop/desktop.build
D=tools/image/experiments/desktop
tools/image/fetch-tools.sh
tools/image/build-base.sh                      # Ubuntu base from desktop.list, pinned snapshot
$D/build-driftwm.sh                            # about 5 minutes (4 CPUs); also makes the build root
$D/build-eudev.sh                              # about 20 seconds
UPDATE_PUB=some-public-key tools/image/build-kernel.sh      # the recovery kernel part needs a public key file
$D/build-root.sh $WORK/out/desktop-root.sqsh
$D/build-disk.sh $WORK/out/desktop-root.sqsh $WORK/out/kernel-a.efi $WORK/vm
python3 $D/steps.py $WORK $WORK/vm $WORK/out1 1,2,3,4,5      # screenshots and the printed log
python3 $D/steps.py $WORK $WORK/vm $WORK/out6 6              # the tablet run (own QEMU options)
ROW=100,213 python3 $D/steps.py $WORK $WORK/vm $WORK/out5 5  # step 5 clicks the menu row at pixel (100,213)
```
`session.py` also works alone: `python3 -i session.py WORK DISKDIR OUTDIR` gives an object `s` with `s.sh()`, `s.screendump()`, `s.key()`, `s.move()`. The QEMU programs are taken from `$WORK/tools/root`.
