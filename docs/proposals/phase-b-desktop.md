# Phase B desktop: does driftwm's real-display path start in QEMU?

> **PROPOSAL, and an experiment report. Nothing in `HUB-OS.md` changed because of it.** `HUB-OS.md` wins if this file disagrees with it.

**Run:** 2026-10-03, in the build environment of `docs/environment.md` (4 CPUs, no KVM: QEMU runs in software emulation, so every time below is slow-motion and not what hardware would do). Labels: **TESTED** (command and output shown), **SOURCE** (read in a file, named), **BELIEVED** (reasoned, not run), **UNKNOWN**.

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
