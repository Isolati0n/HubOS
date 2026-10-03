# Phase B desktop: does driftwm's real-display path start in QEMU?

> **PROPOSAL, and an experiment report. Nothing in `HUB-OS.md` changed because of it.** `HUB-OS.md` wins if this file disagrees with it.

**Run:** 2026-10-03, in the build environment of `docs/environment.md` (4 CPUs, no KVM: QEMU runs in software emulation, so every time below is slow-motion and not what hardware would do). Labels: **TESTED** (command and output shown), **SOURCE** (read in a file, named), **BELIEVED** (reasoned, not run), **UNKNOWN**.

**Second round (section 9, same day, later): eudev replaces the hand-written database, a normal user, hot-plug, focus, hubd end to end. Section 9 also corrects section 5 item 2 (the absolute pointer works) and section 5 item 1 (eudev is the answer to the udev database question).**

Scripts: `tools/image/experiments/desktop/` (README there). They are not part of the default QEMU test or the main image build.

## 1. Answer in one paragraph

**TESTED: yes, on this virtual machine.** driftwm (the pinned commit of `docs/driftwm-findings.md` section 0) started on its real-display path (`--backend udev`: DRM through seatd, GBM and EGL with Mesa's llvmpipe, libinput) inside QEMU, drew a first frame on a virtio GPU, drew a terminal (foot), took keyboard input and mouse clicks sent by the QEMU monitor, and then drew Waybar with an alert from `hubd` on top. It took one workaround that has a real design cost (section 5, item 1) and one input-device finding (section 5, item 2). It proves nothing about real GPUs, real input hardware or real monitors (section 7).

## 2. What was built (steps 1 and 2)

| Item | Value | Label |
|---|---|---|
| driftwm | commit `352333a8fa1b22171492d4b71a54102045c9a19d` (v0.19.0), `cargo build --release` in an Ubuntu 24.04 snapshot build root (`20261001T000000Z`) so it links the image's own libraries; 4 min 48 s; 105 MB with debug info, 20 MB stripped | TESTED (`out/driftwm-build.log`) |
| Packages added to the image base | `seatd libseat1 libinput10 libinput-bin libudev1 libwayland-* libxkbcommon0 xkb-data libdrm2 libgbm1 libegl1 libegl-mesa0 libgles2 libgl1-mesa-dri libglx-mesa0 mesa-vulkan-drivers libvulkan1 libpixman-1-0 libdisplay-info1 libfontconfig1 fonts-dejavu-core foot foot-terminfo wayland-utils` (step 5 adds `waybar dbus-daemon jq fonts-font-awesome`). Mesa 24.0.5, foot 1.16.2, seatd 0.8.0, Waybar 0.9.24. | TESTED |
| Root size | 128 MB squashfs without Waybar, 155 MB with it (the default test image root is far smaller; slot size in the experiment disk is 768 MiB) | TESTED |
| Kernel | Linux 6.12, the tiny base config plus `desktop.frag`; built in 279 s | TESTED |
| Services (s6) | `console`, `seatd` (`seatd -g root -l debug`), `driftwm` (`driftwm --backend udev`), `foot` | TESTED |
| QEMU | q35, OVMF, TCG, 2 CPUs, 2048 MB, `-device virtio-vga,xres=1024,yres=640`, `qemu-xhci` with `usb-kbd` and a pointer device, serial on a pipe, monitor on a unix socket | TESTED |

**systemd:** nothing from systemd runs. Two systemd *libraries* are on the image because other packages pull them in: `libudev1` (driftwm links it) and `libsystemd0` (pulled in by Waybar/dbus-daemon in the Ubuntu packages; it is a library only). This is the same question as `docs/proposals/systemd-libraries.md`; the experiment did not decide it. **TESTED** (`dpkg -l` in the base shows `libsystemd0` and `libudev1` 255.4-1ubuntu8; no `systemd` package).

## 3. Kernel options the experiment added (`desktop.frag`)

`CONFIG_EXPERT EPOLL EVENTFD SIGNALFD TIMERFD FILE_LOCKING ADVISE_SYSCALLS INOTIFY_USER SHMEM TMPFS UNIX98_PTYS DEVPTS_FS UNIX DRM DRM_KMS_HELPER DRM_VIRTIO_GPU INPUT INPUT_EVDEV INPUT_KEYBOARD INPUT_MOUSE USB_SUPPORT USB USB_PCI USB_XHCI_HCD USB_XHCI_PCI HID HID_GENERIC USB_HID HIDRAW`.

**TESTED:** the system boots, draws and takes input with exactly this set. **UNKNOWN:** which lines are strictly needed; no option was removed one at a time to find out. **BELIEVED:** `DRM_VIRTIO_GPU` and the DRM lines are for the display, `INPUT_EVDEV` and the USB/HID lines for input, the rest for Wayland clients and Mesa. Real hardware needs its own GPU driver and input drivers; the list says nothing about that.

## 4. Steps, in order (all TESTED unless stated)

**Step 2, boot and first frame.** `scenario.py full` (README). Time from QEMU start, with the Go QEMU test of the image running at the same time on the same 4 CPUs (so these times are worse than an idle machine; no idle run was made, **UNKNOWN**):

| Event | Time |
|---|---|
| s6 handover line on the serial console | 11 s |
| driftwm "Setting new mode: 1024x640" and first non-black screendump | 29 s (other runs: 26 s, 30 s, 33 s) |
| driftwm "Starting event loop" (about 5 s later, because it logs after the first mode set) | 34 s |

Log lines that show the real-display path (from `shots7/serial.log`):

```
driftwm::backend::udev: System primary GPU: /dev/dri/card0
[seatd/seat.c:219] Opening device /dev/dri/card0 for client 1 on seat0
smithay::backend::egl::display: EGL Version: (1, 5)
renderer_gles2: GL Version: "OpenGL ES 3.2 Mesa 24.0.5-1ubuntu1"
renderer_gles2: GL Renderer: "llvmpipe (LLVM 17.0.6, 128 bits)"
driftwm::backend::udev: DRM resources: 1 connectors, 1 CRTCs, 1 encoders
drm_atomic:create_surface{... mode=Mode { name: "1024x640", ... vrefresh: 75 ...}
smithay::backend::drm::surface::atomic: Setting new mode: "1024x640"
smithay::backend::libinput: Initializing a libinput backend
```

Warnings seen in every run (none stopped the start): `Erroneous EGL call didn't set EGLError`; `GAMMA_LUT atomic property unavailable ... falling back to legacy drmModeCrtcSetGamma`; `Failed to destroy old mode property blob`; `/bin/sh: systemctl: not found` (driftwm tries to run `systemctl`, harmless here and proof that nothing needs systemd); `Config hot-reload disabled: cannot watch "/root/.config/driftwm"`; `xwayland-satellite not found ... X11 apps disabled` (not installed on purpose); `failed to write durable session store: Read-only file system` (the root is read-only; driftwm wants a writable place for its session file, **a design item**).

**Step 3, screenshot.** QEMU monitor `screendump FILE -f png` (QEMU's own PNG writer). `docs/proposals/img/desktop-1-first-frame.png` (1024×640, 11 KB): a black canvas with a regular grid of white dots, a dark terminal window with a title bar reading "foot" and a close button, the text `warning: 'C' is not a UTF-8 locale` and the prompt `root@hub-desktop:/run/service/foot#`; a text-cursor-shaped pointer in the middle. Raw link in the pull request.

**Step 4, keyboard and mouse through the monitor.**
- *Keyboard.* `sendkey KEY 20` (hold 20 ms) with 0.5 s between keys, after an initial Enter. The text `touch /run/key-ok` plus Enter was typed; then over the serial shell `ls -l /run/key-ok` printed `-rw-r--r-- 1 root root 0 Oct  3 16:50 /run/key-ok`. So keys went QEMU USB keyboard → kernel → libinput → driftwm → foot → shell. Image `desktop-2-typed.png`.
- *First attempt failed in a useful way:* with the default key hold and 0.4 s spacing the terminal showed `tttouch /run/key-ok-1` and `bash: tttouch: command not found` (the first key repeated). With 20 ms hold and a leading Enter it was exact. In another early run characters were dropped. **BELIEVED:** under software emulation the key-up event arrives late and the terminal's auto-repeat starts; **UNKNOWN:** whether real keyboards show it (almost surely not).
- *Mouse.* `usb-mouse` (relative): `mouse_move -2000 -2000` (to the top-left corner), `mouse_move 500 250`, then `mouse_button 1`, `mouse_button 0`. foot was asked to report mouse clicks (`printf '\e[?1000h\e[?1006h'`) and what it received was shown with `cat -v`; the terminal printed `^[[<0;57;13M^[[<0;57;1` (press and the start of the release at cell column 57, row 13), and the pointer arrow in `desktop-3-click-report.png` is at about x=500, y=250 as sent. That shows pointer position and click reach the window under the pointer.
- *`driftwm msg state`* (text) with one terminal printed `windows 1` / `* #0 foot [0, 0] 700x525  "foot"` / `fullscreen 0` / `outputs 1` / `* Virtual-1 camera 0 -0.5 zoom 1 1024x640`. `driftwm msg --json state` prints a JSON object with `camera`, `zoom`, `windows`, `fullscreen`, `layers`, `outputs` (`"size":[1024,640]`). This is the socket `hubd` uses; it worked in every run.
- *Focus switching by click between two windows was not demonstrated* (the only click shown is a click into one window).

**Step 5, Waybar, dbus-daemon and hubd.** `scenario.py bar` runs `start-bar` from the serial shell: `dbus-daemon --session --address=unix:path=/run/dw/bus --fork`, `hubd serve --inventory /etc/hubos/inventory.toml --viewers /etc/hubos/viewers.toml --bar-height 30`, `waybar -c /etc/hubos/waybar.json`. All were running (`ps`), `hubd` logged `serving 5 machines on /run/dw/hubos/hubd.sock; driftwm at /run/dw/driftwm/ipc-wayland-1.sock`, Waybar logged `Bar configured (width: 1024, height: 30) for output: Virtual-1`, and `driftwm msg state` listed `layers 1` / `waybar` (first bar run, before the terminal was changed from fullscreen to a normal window; the second run's screenshot shows the bar). `hubd list` printed the fake machines (one DOWN, one THIS HUB, three "NOT CHECKED" because the example inventory has no ports). `desktop-4-bar.png`: a 30 px dark bar across the top; at its left a red box reading `0 of 1 up` (the hubd alert text, red because the only checked machine is down); at its right a clock `16:49:29`; below it the canvas and the foot window moved down clear of the bar. Waybar warnings: no AT-SPI bus, no portal service, `Unable to connect to the SYSTEM Bus!` (all harmless), and a stray `basic_string::_M_create` line (**UNKNOWN** cause; Waybar kept running).

## 5. Findings that need an owner decision

1. **libinput saw no input devices without a udev database; the workaround is a design item.** TESTED: with the workaround switched off (`NO_MKUDEVDB=1`), `seatd` log lines show only `Opening device /dev/dri/card0`; the display works but driftwm opens no `/dev/input/event*` at all, so there is no keyboard and no pointer (`shots-nowa2/scenario.out`). With it on, `seatd` opens `event1` to `event4`. **SOURCE/BELIEVED:** libinput asks the udev database (`/run/udev/data/c13:N`) for `ID_INPUT`, `ID_INPUT_KEYBOARD`, `ID_INPUT_MOUSE`; those entries are written by `udevd`, which we do not run (no systemd-udevd). The experiment's workaround, `overlay/usr/local/bin/mkudevdb`, writes those entries by hand at start from `/sys/class/input`. For a real machine the choices are: keep a small script like this (simple, but it must know every device kind), or run a non-systemd device manager (eudev or mdev were not tried, **UNKNOWN**). This is a decision for the owner.
2. **An absolute pointer (`usb-tablet`) did not work.** TESTED: with `usb-tablet` and `mouse_move X Y` (0 to 32767), the pointer did not follow the numbers: `0 0` left it where it was, `300 200` moved it to the lower right, `16000 16000` pinned it at the bottom right corner (1021,636). With a relative `usb-mouse` the pointer moved exactly by the numbers sent. **UNKNOWN** whether the cause is the hand-written udev entry (the tablet was tagged only as a mouse), libinput or driftwm. A first try that tagged the tablet `ID_INPUT_TABLET` made libinput refuse it ("missing tablet capabilities"). Real mice and touchpads are relative; whether a real touchscreen or tablet would work is **UNKNOWN**.
3. **driftwm wants a writable session file** (`failed to write durable session store: Read-only file system`). BELIEVED harmless for the start; needs a writable path on the image.
4. **The first terminal came up fullscreen** in the earlier runs because the experiment's foot service used `foot -F` (fullscreen); with plain `foot` it is a normal window (the screenshots use this).
5. **systemd traces:** driftwm calls `systemctl` at start (`systemctl: not found`, ignored); Waybar and dbus-daemon pull `libsystemd0` into the package set.

## 6. Items and labels (summary)

| Item | Label |
|---|---|
| driftwm udev backend starts, modesets, shows a first frame with llvmpipe on virtio-gpu in QEMU | TESTED |
| seatd (no logind) hands out the GPU and input devices to driftwm | TESTED (log lines above) |
| libinput works without udevd | **No** without the workaround (TESTED); yes with `mkudevdb` (TESTED) |
| Keyboard from the QEMU monitor reaches a Wayland client | TESTED, needs 20 ms hold and 0.5 s spacing |
| Relative mouse movement and a click reach a Wayland client | TESTED |
| Absolute pointer (tablet) | does not work as sent (TESTED); cause UNKNOWN |
| Waybar + hubd alert + dbus-daemon (no systemd) draw a bar | TESTED |
| Clicking a bar item, opening a viewer from the bar, wofi menu | not run (UNKNOWN) |
| Window focus change by click between two windows | not run |
| Waybar as a non-root user, restart when driftwm restarts (`HUB-OS.md` open items) | not run |
| Vulkan (lavapipe) in use by driftwm | not used; the package is on the image only (UNKNOWN whether driftwm would use it) |
| Time to first frame on an idle machine | UNKNOWN (measured only while another job used the CPUs: 26 to 33 s) |
| Which kernel options are strictly needed | UNKNOWN |

## 7. What this does not prove about real hardware

- A real GPU (Intel, AMD, NVIDIA) needs its own kernel driver, firmware and Mesa driver; llvmpipe is software drawing and is slow on purpose. The hub's real GPU, its outputs, refresh rate and any HDR/10-bit are untested. **UNKNOWN.**
- Real keyboards, mice, touchpads, touchscreens: only QEMU's USB devices were used, and only one pointer type works (finding 2). Hot-plug (a keyboard plugged in later) was not tried: the workaround writes the database once at start. **UNKNOWN.**
- Real monitors: EDID, several outputs, the chosen mode and scaling were not tested (QEMU gave one 1024×640 output; driftwm logged "No [[outputs]] entry ... defaulting to scale 1.0").
- Timing: no KVM, shared CPU. The 26 to 33 s to first frame says nothing about real hardware boot time.
- Security: everything ran as root. A non-root hub session with seatd groups was not tried.
- The Moonlight/virt-viewer/Remmina windows the hub is for were not run; the only client was foot.

## 8. Questions for the owner

1. Which way for libinput's device database: keep a small hand-written script, or choose a small non-systemd device manager (eudev or mdev) to test next?
2. Is it acceptable that `libudev1` and `libsystemd0` (libraries only) are on the hub image? (Same question as `docs/proposals/systemd-libraries.md`.)
3. Run the hub desktop as root or as a normal user with a seat group? (Not tested.)
4. Should the next experiment test a touchscreen/absolute pointer, hot-plug, and a click on the Waybar item, or is the mouse and keyboard result enough for now?

---

## 9. Second round (2026-10-03, later): eudev, a normal user, hot-plug, focus, hubd end to end

Same environment and labels as above (TESTED = the command and its output are in `tools/image/experiments/desktop/steps.py` and were run; SOURCE; BELIEVED; UNKNOWN). Everything ran in QEMU in software emulation, on a build machine that was also running another job, so all times are slow-motion. Scripts: `tools/image/experiments/desktop/` (`README.md` lists the commands; `steps.py` runs the steps and prints every command and its output). Nothing was added to the default QEMU test or the main image build. The kernel (`desktop.frag`) did **not** change: **no new kernel option was needed** for udevd, hot-plug or the normal user (TESTED: same fragment, rebuilt from the same inputs).

### 9.1 Step 1: eudev instead of the hand-written database

| Item | Result | Label |
|---|---|---|
| Which eudev | **eudev 3.2.14**, release tarball `https://github.com/eudev-project/eudev/releases/download/v3.2.14/eudev-3.2.14.tar.gz`, sha256 `8da4319102f24abbf7fff5ce9c416af848df163b29590e666d334cc1927f006f` (computed at the download; **not** compared with an independently published checksum) | TESTED / UNKNOWN (checksum source) |
| Licence | the programs (`udevd`, `udevadm`) are **GPL-2.0-or-later** (file header of `src/udev/udevd.c`: "either version 2 of the License, or (at your option) any later version"; the tarball's `COPYING` is the GPL v2 text); the library `libudev` is **LGPL-2.1-or-later** (header of `src/libudev/libudev.c`). Not legal advice | SOURCE |
| How it was built | in the sandbox build root (the Ubuntu 24.04 snapshot root of `build-driftwm.sh`, `gperf` added), `./configure --prefix=/usr --sysconfdir=/etc --disable-manpages --disable-kmod --disable-blkid --disable-selinux --disable-introspection --disable-hwdb --disable-static`, `make -j4`, `make DESTDIR=...install`: **21 s**, 42 files (`/usr/sbin/udevd`, `/usr/bin/udevadm`, `/usr/lib/libudev.so.1.6.3`, the helpers and the rules files in `/usr/lib/udev`). Nothing was installed on the machine | TESTED |
| Runs without systemd | `udevd` is started by an s6 service (`overlay/etc/s6/sv/udevd/run`): `udevd &`, wait for `/run/udev/control`, `udevadm trigger --type=subsystems --action=add`, `udevadm trigger --type=devices --action=add`, `udevadm settle`, then `touch /run/udev/ready` (driftwm waits for that file). The service printed `udevd: devices triggered, database in /run/udev/data (627 entries)` | TESTED |
| eudev's libudev replaces the systemd-built one | `ldd /usr/local/bin/driftwm` → `libudev.so.1 => /lib/x86_64-linux-gnu/libudev.so.1` and that file is `libudev.so.1.6.3` (eudev's; the Ubuntu one was `1.7.8`). driftwm, libinput (Ubuntu 24.04 package), libgudev and libseat all ran against it without a symbol-version error | TESTED |

The database eudev wrote (`/run/udev/data/c13:67`, the USB keyboard; `c13:68`, the USB mouse), cut:
```
S:input/by-id/usb-QEMU_QEMU_USB_Keyboard_68284-0000:00:04.0-1-event-kbd      E:ID_INPUT=1   E:ID_INPUT_KEY=1   E:ID_INPUT_KEYBOARD=1   E:ID_BUS=usb ...
S:input/by-id/usb-QEMU_QEMU_USB_Mouse_89126-0000:00:04.0-2-event-mouse       E:ID_INPUT=1   E:ID_INPUT_MOUSE=1     E:ID_BUS=usb ...
```
`libinput list-devices` (run as root in the guest, reading that database) listed five devices, all on `Seat: seat0, default`: Power Button (keyboard), QEMU USB Keyboard (keyboard), QEMU USB Mouse (pointer), AT Translated Set 2 keyboard (keyboard), ImExPS/2 Generic Explorer Mouse (pointer). The driftwm log has `Initializing a libinput backend`, then `New device "eventN"` for each and `Configuring mouse: QEMU QEMU USB Mouse (accel=0, profile=Flat ...)`. **TESTED.** So eudev builds, runs without systemd, and libinput finds the keyboard and mouse from **its** database. The hand-written script `mkudevdb` is removed from the experiment (it is in git history, PR #29).
**What this means for `docs/proposals/systemd-libraries.md`:** on the hub, `libudev1` from the systemd sources can be replaced by eudev's `libudev.so.1` for driftwm and libinput. This round did **not** touch `libsystemd0` (Waybar and dbus-daemon from the Ubuntu packages still pull it in) and did not remove the `libudev1` package from dpkg's list (the experiment overwrites its files). The hub image would carry eudev as a package we build. UNKNOWN: a second libudev user such as Mesa/gbm in other roles; eudev's rules against other hardware; how to keep eudev patched.
**Screen contents:** no screenshot for this step (the first frame is the same as in section 4).

### 9.2 Step 2: seatd and the desktop as a normal user in the seat group

What had to change (no change to driftwm itself):
1. A user `hub` (uid 1000) and a group `seat` (gid 1001, member `hub`) are made in `/etc/passwd` and `/etc/group` by `build-root.sh`.
2. `seatd -g seat` (it runs as root and opens the GPU and input devices for its clients); its socket is `srwxrwx--- root seat /run/seatd.sock`.
3. The `driftwm` and `foot` services start as `hub` with `s6-setuidgid hub` (after root has made `/run/dw` and `/run/hub/state`, mounted `/dev/pts` and `/dev/shm`). `s6-setuidgid` also sets the supplementary groups: `/proc/<driftwm pid>/status` shows `Uid: 1000 ... Groups: 1000 1001`.
4. `XDG_RUNTIME_DIR=/run/dw` (owned by `hub`, mode 0700), `HOME=/run/hub`, and **`XDG_STATE_HOME=/run/hub/state`** (owned by `hub`): driftwm then wrote its session store (`/run/hub/state/driftwm/session.json`, 201 bytes) and the log has **0** lines `failed to write durable session store` (the first round had one, on the read-only root). The folder is on a RAM disk, so it is gone at reboot (a real hub would need a place on the data partition: UNKNOWN which).
5. `udevd` stays root. `driftwm` waits for `/run/udev/ready`.
**TESTED:** `ps` shows `hub /usr/local/bin/driftwm --backend udev` and `hub foot -T first`; the first frame, keyboard and mouse worked as in the first round; the DRM device was opened for it by seatd although `hub` is not in the `video` group. UNKNOWN: a real GPU (DRM master hand-over, permissions on `/dev/dri/*`), a second seat, logging in on a VT.

### 9.3 Step 3: hot-plug (QEMU `device_add` / `device_del`)

Sequence (`steps.py 3`): `udevadm monitor --udev --property` in the background, `device_add usb-kbd,id=kbd2,bus=xhci.0`, `device_add usb-mouse,id=mouse2,bus=xhci.0`, then `device_del` for both. **TESTED**:
- *Keyboard plug:* `/dev/input/event5` appeared; `udevadm info -q property -n /dev/input/event5` printed `ID_INPUT=1`, `ID_INPUT_KEY=1`, `ID_INPUT_KEYBOARD=1`, `ID_MODEL=QEMU_USB_Keyboard`; `libinput list-devices` listed `QEMU QEMU USB Keyboard / Kernel: /dev/input/event5 / Capabilities: keyboard` (six devices instead of five).
- *Keys from the new keyboard reach driftwm and the terminal:* QEMU's monitor `sendkey` goes to the **newest** keyboard (checked: `libinput debug-events --show-keycodes` counted 46 `KEYBOARD_KEY` lines, all on `event5`, none on the old keyboard). The typed `touch /run/hub/hp-kbd2` created the file `-rw-r--r-- 1 hub hub 0 /run/hub/hp-kbd2` (owner `hub`: the shell in the `foot` window ran it). Screenshot `docs/proposals/img/desktop2-1-hotplug-keyboard.png` (1024×640, 10 KB): black canvas with the dot grid and one terminal window titled "first" whose text shows the line `$ touch /run/hub/hp-kbd2` and an empty prompt below it.
- *Mouse plug:* `event6`, `libinput list-devices` → `QEMU QEMU USB Mouse / Capabilities: pointer`. `mouse_move 300 200` (a relative move) went to the newest relative mouse (`debug-events`: `POINTER_MOTION` on `event6`) and the pointer moved: the box that differs between the screenshots before and after is (506,309)-(817,531), i.e. from the centre to about 300 px right and 200 px down.
- *Unplug:* both `device_del` removed the nodes (`ls /dev/input` back to `event0`-`event4`), `libinput list-devices` listed five devices again, `udevadm monitor` recorded `ACTION=add` for `event5` and `event6` and `ACTION=remove` for `event6` and `event5`, and typing on the old keyboard still worked (`/run/hub/after-unplug` was created).
- *driftwm's own log has no line for a hot-plug at its default level* (the filter found none after the plug or the unplug: the `New device "eventN"` lines appear only at start). So the evidence that driftwm gets the keys is the typed result, not a log line. UNKNOWN: whether a debug log level shows it.
- *A QEMU finding (not about Hub OS):* QEMU 8.2.2 **aborted (signal 6)** when asked for `input-send-event` with `"device": "kbd2"` for the hot-plugged `usb-kbd` over QMP (TESTED twice). The runs use the monitor's `sendkey` and `mouse_move` instead.

### 9.4 Step 4: focus switching between two windows

Two terminals were opened as `hub` with different titles (`foot -T alpha`, `foot -T beta`; each shell has `w=alpha` / `w=beta` in its environment), placed side by side with `driftwm msg move --id N -260 0` / `260 0` and `resize --id N 400 400` (`state` then shows `#1 foot [-260, 0] 400x400 "alpha"`, `#2 foot [260, 0] 400x400 "beta"`). The pointer was sent to the top-left corner (`mouse_move -4000 -4000`), stepped to (250,320) or (770,320), clicked (`mouse_button 1` then `0`), and then `touch /run/hub/typed-$w-N` was typed with `sendkey` (20 ms hold, 0.5 s between keys). **TESTED**, after each click `driftwm msg state` marked the clicked window with `*` and the file name told which shell had the keys:

| Click | `state` marks | File created |
|---|---|---|
| alpha at (250,320) | `* #1 foot [-260, 0] 400x400 "alpha"` | `typed-alpha-1` |
| beta at (770,320) | `* #2 foot [260, 0] 400x400 "beta"` | `typed-beta-2` |
| alpha again | `* #1 ... "alpha"` | `typed-alpha-3` |

Screenshot `docs/proposals/img/desktop2-2-focus-beta.png` (8 KB): two dark terminal windows side by side on the black dot-grid canvas, titles "alpha" (left) and "beta" (right); the left one shows `$ touch /run/hub/typed-$w-1` and an empty prompt with a hollow cursor, the right one shows `$ touch /run/hub/typed-$w-2` with a filled cursor (focused); the pointer (a text cursor) is inside "beta". (The `$w` stays literal in the picture because the line is what was typed; the shell expanded it.) *Note on the first try:* typed capital letters and `$` were not mapped, which made files named `typed--1`; the helper now maps `$` and capitals. The relative mouse is reliable for this: after a corner reset (a move far past the edge) each step lands where it is sent (flat acceleration profile).

### 9.5 Step 5: hubd end to end

Setup (`start-bar`, run from the root shell; all four programs run as `hub`): `dbus-daemon --session`, `fakenode 127.0.0.12:21002` (a fake machine "up"; nothing listens on `nas-1`'s port, so it is down), `hubd serve --inventory /etc/hubos/inventory.toml --viewers /etc/hubos/viewers.toml --bar-height 30`, `waybar`. Inventory (`overlay/etc/hubos/inventory.toml`): `hub`, `ai-1` (open `ssh`, home `{x=0, y=-100}`), `nas-1`. `viewers.toml` is the repo's example: viewer `fake` = `foot --app-id={app_id} --title={title} -- sleep infinity`, so the chosen app-id is `hubos-ai-1` and the title is the machine name "AI Box". **TESTED:**
- `hubd list` printed `ai-1 AI Box UP`, `nas-1 Storage DOWN`, `hub ... THIS HUB`; the bar showed `1 of 2 up` on a red background.
- **`hubd open ai-1`** (run as `hub` from the shell) printed `opened AI Box at home (0, -100), matched by name; the view is at y=-70.5, not -85.0 (home plus half the bar height); the bar offset may be different from what hubd expects` and `driftwm msg state` then showed `* #1 hubos-ai-1 [0, -100] 700x525 "AI Box"` and `layers 1 / waybar`, camera `0 -85.5`. So the window is at its home position (0,-100). Screenshot `docs/proposals/img/desktop2-3-hubd-open.png` (8 KB): the dark bar across the top with a red box `1 of 2 up` at its left and the clock at its right, and one terminal window titled "AI Box" below it (the edge of the first terminal shows behind it).
- `hubd end ai-1` printed `closed the local window of AI Box; the machine and its session were not touched`; the window disappeared from `state`.
- **Clicking the Waybar item:** pointer to the corner, then stepped to (40,15) and clicked: a `wofi` process started (`hubd menu`), driftwm logged `New layer surface: wofi`. **The list was empty** (only the search box) with hubd's default `wofi --lines 12`, and wofi printed `Gtk-CRITICAL ... gtk_widget_set_size_request: assertion 'height >= -1' failed`. Tried: wofi alone with `--lines 5`, with the style file, with only `--width` (all empty); with `--height 300` and no `--lines` the three lines `one two three` appeared. **Workaround:** `overlay/usr/local/bin/wofi-fixed` (a 10-line wrapper that drops `--lines N` and adds `--height 480`), used as `hubd menu --wofi /usr/local/bin/wofi-fixed` in Waybar's `on-click`. Cause UNKNOWN (wofi 1.4.1 from Ubuntu 24.04; it worked in the first investigation's nested driftwm, `docs/bar-findings.md`, so the difference is probably the real-display backend's output information). Screenshot `docs/proposals/img/desktop2-4-bar-click-menu.png` (28 KB): the bar as above; below it a white wofi panel 668 px wide with the search box (`hub`), a highlighted first row `? search by id or name...`, and the grouped list `- Down machines (1) / nas-1 Storage DOWN / - Hub (1 machine) / hub Desk Hub THIS HUB / - AI (1 machine) / ai-1 AI Box UP / - NAS (1 machine, 1 down) / nas-1 Storage DOWN`; the pointer arrow is on the bar item.
- **Clicking a wofi entry:** the pointer was stepped to (100,213) (the `ai-1` row) and clicked: the menu closed and **driftwm opened a new window** `* #2 hubos-ai-1 [0, -100] 700x525 "AI Box"` (the earlier one had been closed with `hubd end`). Screenshot `docs/proposals/img/desktop2-5-menu-row-click.png` (10 KB): the bar, the old "first" terminal behind and the new "AI Box" window in front; the pointer arrow is at the row's old place.
- The relative mouse was reliable for these two clicks (the bar-item click was made in three runs and worked each time, the row click once): move far past the top-left corner, then step to the item; both clicks hit. Not needed: `hubd menu`/`hubd pick` as the fallback.
- UNKNOWN: Waybar and hubd are started by hand here, not by s6 (the open "hub service design" item in `HUB-OS.md`); `hubd`'s note "the view is at y=-70.5, not -85.0" was printed once and is not explained.

### 9.6 An extra: the absolute pointer works (a correction of section 5, item 2)

Section 5 said an absolute pointer (`usb-tablet`) "did not work as sent" and that the cause was UNKNOWN. **Cause found (TESTED):** it was the way the test drove QEMU, not libinput, driftwm or the database. QEMU's monitor `mouse_move X Y` sends **relative** events to the first relative mouse it finds, and q35 always has a PS/2 mouse; `libinput debug-events` showed `POINTER_MOTION` with `+127.00/+127.00` on the PS/2 mouse's node for every call, whatever `mouse_set` selected. With the PS/2 devices switched off (`-machine ...,i8042=off`) and a `usb-tablet`, QMP `input-send-event` with `abs` events (no `device` argument) moved the pointer exactly: `libinput debug-events` printed `POINTER_MOTION_ABSOLUTE 18.31/18.31`, `79.35/79.35`, `50.00/50.00` for 6000, 26000 and 16384 of 32767, and the screenshots differ in the boxes (182,106)-(818,519) and (506,309)-(818,519) (expected pointer positions (187,117), (812,506), (512,320)). eudev tags the tablet `ID_INPUT=1 ID_INPUT_MOUSE=1` (no tablet tag) and libinput shows `Capabilities: pointer`. So the first-round `ID_INPUT_TABLET` workaround finding ("missing tablet capabilities") was about the hand-written tag only. UNKNOWN: a real touchscreen or pen tablet.

### 9.7 Summary of the second round

| Item | Result | Label |
|---|---|---|
| eudev 3.2.14 builds from source in the sandbox (21 s) and runs without systemd | yes | TESTED |
| libinput finds the keyboard and mouse from the database eudev wrote | yes (5 devices, `seat0`) | TESTED |
| eudev's libudev.so.1 replaces the systemd-built one for driftwm/libinput/libgudev/libseat | yes | TESTED |
| seatd + driftwm + foot as user `hub` (group `seat`), session store written | yes, 0 warnings | TESTED |
| Hot-plug: eudev and libinput pick up a keyboard and a mouse added and removed at run time; keys from the new keyboard reach the terminal | yes (driftwm logs nothing at its default level) | TESTED |
| Focus follows a click between two windows; keys go to the clicked one | yes (alpha, beta, alpha) | TESTED |
| `hubd open` places the window at its home position | `[0, -100]` | TESTED |
| Waybar item click → wofi menu → row click opens the machine | yes, with the `wofi-fixed` wrapper; with hubd's own wofi arguments the list is empty | TESTED (cause UNKNOWN) |
| Absolute pointer (tablet) | works when driven through QMP with the PS/2 devices off | TESTED |
| Several seats, a real GPU, real USB plug timing, DRM permissions, VT switching, a real touchscreen | not run | UNKNOWN |
| Waybar and hubd under s6, a persistent state folder for driftwm, `libsystemd0` removal | not done | UNKNOWN |

### 9.8 What this second round does not prove about real hardware

- A hot-plug here is QEMU creating an emulated USB device; it says nothing about real connectors, hubs, wake-up, power or the time a real device takes to enumerate.
- virtio-gpu with software drawing: seatd handing out a real GPU, DRM master hand-over, outputs appearing or disappearing, EDID, VT switching and permissions on real `/dev/dri` nodes are all untested.
- Input: only QEMU's keyboard, mouse and tablet. Real touchpads need libinput's hardware quirks (we built eudev without `hwdb`; UNKNOWN whether libinput needs it for a given touchpad).
- eudev against real hardware rules (storage, network naming, GPUs), the boot-time cost of `udevadm trigger` on a machine with many devices, and keeping a source-built eudev patched are untested.
- Timing: everything ran while another job used the CPUs. The first frame (driftwm's `Starting event loop`) came 28 to 39 s after QEMU started in these runs.
- The click tests rely on the exact relative-pointer arithmetic of QEMU plus driftwm's flat profile; a real mouse with acceleration needs a different test.

### 9.9 Questions for the owner (second round)

1. Adopt **eudev** (GPL-2.0-or-later programs, LGPL libudev) as the hub's device manager and its `libudev.so.1` in place of the one from the systemd sources? It removes the hand-written database; it is a package we would build and patch ourselves.
2. The wofi menu is empty with hubd's own arguments on this backend. Options: keep a wrapper like `wofi-fixed`; change `hubd menu` to pass `--height` and not `--lines` (a hubd change, not made); find the real cause (not done); or use another launcher (bemenu or tofi were tested in `docs/bar-findings.md` only nested). Which?
3. Where should driftwm's state folder (`XDG_STATE_HOME`) live on the hub: the data partition, or RAM (session lost at reboot)?
4. Waybar, hubd, `dbus-daemon` and (for the hub) a first terminal: as s6 services of the user `hub`, or started by a session script? This is the open "hub service design" item.
5. Do you want a second round on a real-GPU-like setup (several outputs, output hot-plug, VT switch) later, or is this enough until December?

---

## 10. Now in the image (2026-10-03, fifth round)

What the experiment found has moved into the first hub image, `image/machines/hub.build` (`docs/image.md` section 9), with a QEMU test (`TestHubImage`):
- **eudev 3.2.14** (pinned, hash checked) is built in the image build; its `libudev.so.1` replaces the systemd-built one. `udevd` runs as an s6 service; the hand-written database script does not exist any more.
- **driftwm** (pinned commit `352333a8...`) is built in the image build. `seatd` and `udevd` run as root; `dbus-daemon`, `driftwm`, `Waybar` and `hubd` run as the normal user `hub` (group `seat`). `XDG_STATE_HOME` is in RAM (`/run/hub/state`): windows are not restored after a reboot.
- Each of Waybar and hubd waits for driftwm's socket and **restarts when driftwm restarts** (`follow-driftwm`); the confirm step needs hubd, which waits for driftwm, so a release whose desktop cannot start is rolled back. The test kills driftwm and shows the bar come back; `hubd open` places a window at its home; an A/B update to a second release works and a bad one rolls back.
- **The wofi menu** (question 2 of section 9.9): the cause is a wofi 1.4.1 bug in its `--lines` path on a layer-shell surface (`docs/image.md` section 3.13); `hubd menu` now passes `--height 340` and never `--lines`; the `wofi-fixed` wrapper is gone. The hub image test checks that the list is drawn after a click on the Waybar item.
- Not in the image test (done only here): a click on a menu row, hot-plug, focus switching, the absolute pointer.
- Still UNKNOWN: a real GPU, real displays, real input devices, DRM permissions, VT switching.

