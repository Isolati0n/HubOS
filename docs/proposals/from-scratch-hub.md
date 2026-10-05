# The from-scratch hub: what is in the hub image today, what building it ourselves would take, and a staged way there

> **PROPOSAL AND RESEARCH. Nothing here is a decision and nothing in `HUB-OS.md` changed.** `HUB-OS.md` wins if this file disagrees with it. The main image was not changed. The owner's decision behind it (relayed by the lead, 2026-10-05, not yet in `HUB-OS.md`): the hub is eventually built from scratch, every component chosen, built from source and optimised for the hub's one use; the kernel stays upstream Linux, built for the hub; the libc is glibc. Hub priorities: (1) uptime and stability, (2) integration, (3) optimisation for the hub's one use, (4) building it ourselves only when it measurably wins, (5) simplicity.

**Written:** 2026-10-05. **Branch:** `from-scratch-hub`. **Part 5 of the research round; documents only.**

Labels on every item: **TESTED** (the command ran here and the output is shown or named), **SOURCE** (read in a file or web page, named, date read 2026-10-05), **BELIEVED** (reasoned, not run), **UNKNOWN**. No sub-helper agent was used for this document; everything marked SOURCE I read myself. Where the build host's load matters: the 4 CPUs were shared with other helpers' builds the whole time, so every time below is slower than an idle machine would give.

---

## 0. Summary in plain words

**What the hub image is today.** It is an Ubuntu 24.04 root built from 288 ready-made packages, plus two programs we build from source (driftwm and eudev), plus our own parts (hubd, the init and update scripts). Squashed and compressed it is **152 MB** (TESTED); unpacked it is about **541 MiB in 16,501 files**. Only a small part of that is the hub's real work.

**Where the weight is.** Four blocks make up most of it (sizes are what the packages declare, TESTED):

| Block | What it is | Size installed | Why it is there |
|---|---|---|---|
| Mesa and LLVM | the graphics drivers (OpenGL, Vulkan) and the compiler inside them | about **207 MiB** | driftwm draws with OpenGL; the Vulkan drivers have no listed reason (UNKNOWN) |
| GTK 3 and friends | the toolkit under Waybar and wofi, plus icon themes, ICU, printing, Kerberos libraries and more | about **132 MiB** | Waybar (the bar), wofi (the dropdown) and `libinput-tools` all need it |
| The Ubuntu base | coreutils, bash, perl, util-linux, dpkg, openssl and so on | about **79 MiB** | came with the package system |
| Python 3 | pulled in by `libinput-tools` and by `mesa-vulkan-drivers` | about **27 MiB** | nothing in the hub runs Python by design |

What the hub actually needs for its one job (a compositor, a bar, a list, a few viewers, sound) is far smaller. driftwm itself, stripped, is 20 MB and links only nine libraries (TESTED).

**What I tested.** (a) I built the real hub root with the repo's own tools and measured it. (b) I built a **from-scratch glibc 2.39, busybox 1.37 and the s6 family** from tarballs with pinned SHA-256 hashes, outside the repo, with the Ubuntu build host as the bootstrap. It ran inside a `chroot` with nothing else around it, s6 supervised a service, and the finished stripped root is **7.8 MiB, 2.9 MB squashed** (TESTED). Building glibc took **585 s** and busybox about a minute. I built glibc a second time and busybox a second time and got **bit-identical files** (TESTED).

**What it costs to go all the way.** The hub path needs roughly **40 source packages** instead of 288 (BELIEVED, my list in section 3). Most of them are small and ordinary to build. The hard parts are: **Mesa with LLVM** (hours to build, BELIEVED), the **Rust toolchain and 364 crates for driftwm**, **GPU firmware** (blobs, not source, so "all from source" cannot be literally true for an AMD GPU), and **replacing Waybar and wofi** with our own panel (real new code).

**The base.** Four ways were compared (section 4). The repo's tools only need one thing from the base step: a directory tree with the root in it, so any of them plugs in where `build-base.sh` is today. My reading of the measurements (not a decision): our own scripts are the smallest and give the most control but we carry every recipe and every version bump; Buildroot already has recipes for most of the plumbing (but not driftwm, Waybar, wofi or dinit) and a hash file per package; Yocto has the most recipes and the best bookkeeping but is the heaviest to learn and run and I did not run it.

**The plan** (section 5) takes the biggest trees first and keeps the existing QEMU tests passing at every stage: first trim what is not needed, then replace the GTK tree with our own panel, then build the graphics stack and the small plumbing from source, then swap the base. **Fourteen questions for the owner are in section 6.**

---

## 1. What I did, and what I could not do

| Step | Result | Label |
|---|---|---|
| Unpack the repo's build tools (`tools/image/fetch-tools.sh`) | 59 packages, 38 s | TESTED |
| Build the hub base root from `image/packages/hub.list` (`build-base.sh`, snapshot `20261001T000000Z`) | 180 s, **288 packages**, base.tar sha256 `17f8d01f915607193ceb4e280a35ed9624b4998e07a5ecba9f4a18c88b5b446e` | TESTED |
| Build driftwm (pinned commit) and eudev (`build-hub-parts.sh`) | build root 3 min, driftwm **7 min 6 s** (427 s counted by the script), eudev **39 s**; shared CPU | TESTED |
| Put the root together like `build-root-image.sh` does (rootfs overlay, driftwm stripped, eudev, `strip-root.sh`) and squash it with zstd | **152,297,472 bytes**, 553,808 KiB unpacked, 16,501 files | TESTED (steps copied by hand into a script; see 2.1) |
| From-scratch glibc 2.39, busybox 1.37.0, skalibs, execline, s6 from pinned tarballs | section 3.2 | TESTED |
| Same build run twice | glibc `libc.so.6`, `libm.so.6`, `ld-linux-x86-64.so.2` and `busybox` are bit-identical | TESTED (same directory path, same host) |

**Not done, said plainly:**
- I did **not** run `build-root-image.sh` itself, because it also needs the kernel build and the recovery kernel (a long build I did not repeat). I repeated its copy steps by hand, so the 152 MB leaves out `hubd`, `efibootmgr`, `signify`, the recovery kernel and the keyring. The repo's own comment says "about 160 MB" for the hub root (SOURCE `image/machines/hub.build`), which fits.
- I did **not** run the QEMU tests (`TestImage`, `TestHubImage`; 12 to 20 minutes each, by hand). The main image was not changed, so they have nothing new to test yet. The plan in section 5 says which one runs at which stage.
- I did **not** boot the from-scratch root in a virtual machine. I ran it with `chroot` only (the host kernel). Its boot with stage 0 and s6 as pid 1 is UNKNOWN (BELIEVED to work; the Buildroot root of the earlier test was not booted either).
- I did **not** build Mesa, LLVM, Rust, wayland or libinput from source. Their build times in this document are BELIEVED, not measured.
- I did **not** run Buildroot or Yocto; Buildroot's numbers come from the earlier test in `docs/proposals/phase-b-image.md` section 1.2 (SOURCE), Yocto has none.
- GPU, projector, real hardware: none of it is reachable.

---

## 2. Inventory of today's hub image

### 2.1 How the image is made today (SOURCE: `image/machines/hub.build`, `image/packages/hub.list`, `tools/image/*.sh`, read 2026-10-05)

1. `build-base.sh`: `mmdebstrap` (Ubuntu 24.04 "noble", dated snapshot `20261001T000000Z`) installs the 36 names in `hub.list`; apt resolves them to **288 packages** (TESTED).
2. `build-hub-parts.sh`: driftwm and eudev built from source inside a second throw-away Ubuntu root with `-dev` packages (about 1.3 GB as a tar, TESTED), the host's Rust toolchain bind-mounted in.
3. `build-root-image.sh`: copies `image/rootfs`, the hub overlay, static `hubd`, `efibootmgr`, `signify`, the keyring and the recovery kernel; `strip-root.sh` deletes apt, PAM, procps, login, passwd; `check-libs.sh` runs `ldd` on every ELF file; `mksquashfs -comp zstd`.
4. The kernel is already built from source by `build-kernel.sh` (Linux 6.12, `tinyconfig` plus a fragment).

### 2.2 The numbers (TESTED unless marked)

| Measure | Value |
|---|---|
| Packages in the base | 288 (36 asked for in `hub.list`, 252 pulled in) |
| Sum of "Installed-Size" of those packages | 494,576 KiB (483 MiB) |
| Base directory, `du` | 526 MiB before, 541 MiB after adding driftwm, eudev, overlay and deleting apt/PAM/procps (driftwm adds 20 MB; the strip step removed 776 files) |
| Files in the finished root | 16,501 |
| Squashfs, zstd | **152.3 MB** |
| Largest single files | `libLLVM-17.so.1` 123.7 MB; Mesa's one driver file `*_dri.so` 33.7 MB (the same file under 13 names: hard links, so counted once); `libicudata` 30.8 MB; driftwm 20.2 MB stripped (105 MB with debug info); Intel Vulkan 18.4 MB and 13.7 MB; Radeon Vulkan 10.7 MB; lavapipe 8.4 MB; `libgtk-3` 8.1 MB; `python3.12` 8.0 MB |
| Not present | Qt, FFmpeg, GStreamer, any Python beyond the interpreter and four small modules (TESTED: none of those package names are among the 288) |

### 2.3 By block (sizes TESTED from dpkg; the grouping of a package into a block is by name pattern and is BELIEVED approximate)

| Block | MiB | Packages | Pulled in by |
|---|---|---|---|
| Mesa + LLVM + drivers | 206.6 | 40 | `libgl1-mesa-dri`, `mesa-vulkan-drivers`, `libegl-mesa0`, `libgbm1` |
| GTK 3 stack (GTK, gtkmm, glibmm, cairo, pango, ICU, CUPS, Kerberos, GnuTLS, icon themes 38 MiB, ...) | 131.7 | 101 | Waybar, wofi, `libinput-tools` |
| Ubuntu base (libc6, perl-base, coreutils, openssl, dpkg, util-linux, e2fsprogs, bash, ...) | 78.7 | 72 | `minbase` and the package system |
| Python 3.12 | 27.0 | 18 | `libinput-tools`; **also `mesa-vulkan-drivers` (its Depends line lists `python3:any`)** |
| Audio client libraries (PipeWire, PulseAudio, JACK, WirePlumber, ALSA, codecs) | 12.8 | 19 | Waybar |
| Waybar, wofi, foot (the programs themselves) | 6.8 | 5 | `hub.list` |
| Fonts (DejaVu, Font Awesome) | 6.7 | 7 | the bar |
| Compositor-side libraries (libinput, xkbcommon, wayland, seat, udev) | 6.4 | 13 | driftwm |
| s6, execline, busybox, jq, dbus-daemon | 6.2 | 13 | init, scripts, Waybar |

**Heavy trees in detail (TESTED, computed from dpkg `Depends` lines):**

- **Waybar alone** pulls 178 packages (200 MiB counting the shared base). Of those, **40 packages (26 MiB) are used by nothing else in the image**, among them gtkmm, glibmm, `libspa-0.2-modules`, `libjack`, `libpulse`, `libwireplumber`, `libsamplerate`, `libspdlog`, `liblua5.4`, `libmpdclient`, `libupower`, `libplayerctl`. Waybar links 36 libraries directly; wofi 10; foot 9; dbus-daemon 8; driftwm 9.
- **GTK 3** is shared by Waybar, wofi and `libinput-tools`: removing only one of them frees almost nothing. All three together own **111 packages, 105 MiB** that nothing else uses (icon themes alone 38 MiB).
- **`libinput-tools`** (it holds the `libinput` command and the debug tools; `libinput-bin` holds only documentation, SOURCE `dpkg` file lists) pulls GTK 3, cairo and Python 3 for debug programs. Its reason to be in the image is not written down (UNKNOWN; BELIEVED a debugging aid).
- **Mesa + LLVM:** `libllvm17t64` is 120.8 MiB. `libicu74` (35.9 MiB) arrives through `libxml2`, which `libllvm17t64` and `shared-mime-info` need. The hub has AMD GPUs only (SOURCE `HUB-OS.md`), yet the image carries Intel (`i915`, `crocus`, `iris`, `vulkan_intel`), Nouveau, Zink, D3D12, VMware and r300/r600 drivers (the DRI ones are one hard-linked file, so they cost nothing extra; the Vulkan ones do cost: Intel 32 MB, lavapipe 8 MB, virtio 1 MB).
- **Python:** 27 MiB for a handful of debug scripts and a build-time dependency of the Vulkan package.
- **`libsystemd0` (1 MiB) is present** (pulled by Waybar's and dbus-daemon's packages) and `libudev1` is replaced by eudev's by the build (SOURCE `HUB-OS.md`, `docs/proposals/systemd-libraries.md`).

### 2.4 Component table: version, reason, what it pulls in, size

Sizes are dpkg "Installed-Size" in KiB unless marked. "Why" comes from the repo documents (`docs/proposals/phase-b-desktop.md`, `docs/bar-findings.md`); where no reason is written down I say UNKNOWN.

| Component | Version | Why it is there | What it pulls in | Size |
|---|---|---|---|---|
| Linux kernel | 6.12 (built by `build-kernel.sh`, config `image/kernel/hub.frag`) | the kernel | nothing at run time | not measured here |
| glibc (`libc6`) | 2.39-0ubuntu8 | the libc | nothing | 13,430 |
| Ubuntu base userland: bash, coreutils, util-linux, perl-base, dpkg, e2fsprogs, openssl, ncurses, tzdata, ... | 5.2.21, 9.4, 2.39.3, 5.38.2, 1.22.6, 1.47.0, 3.0.13, ... | came with `minbase`; scripts use some of it | 72 packages | 78.7 MiB together |
| busybox-static | 1.36.1 | applets: `ip`, `udhcpc`, `watchdog`, `ps` | none (static) | 2,123 |
| s6 / execline | 2.12.0.3 / 2.9.4.0 | init and service supervision | skalibs (static inside) | 1,141 / 784 |
| `hubos-ctl`, `init`, stage 0, recovery scripts | ours | update, boot, rollback | busybox sh | 290 + 56 + 113 + 49 lines of shell (SOURCE `wc -l`) |
| hubd | ours, Go | broker | none (static) | about 4,000 lines of Go without tests (SOURCE `wc -l` of `cmd` and `internal`) |
| eudev | 3.2.14, from source (hash pinned) | udev database for libinput | libc only | 42 files, built in 39 s |
| seatd, libseat1 | 0.8.0-1 | seat access for driftwm | libc | 100 + 83 |
| driftwm | 0.19.0 commit `352333a8`, from source | the compositor | 9 direct libraries: libdisplay-info, libgbm, libseat, libudev, libinput, libxkbcommon, libgcc_s, libm, libc; loads EGL/GLES at run time (BELIEVED, not in its NEEDED list) | 20,223,816 bytes stripped |
| libinput10, libinput-bin | 1.25.0 | input for driftwm | libevdev, libmtdev, libwacom, libudev | 371 + 144 |
| libinput-tools | 1.25.0 | `libinput` command and debug tools (reason UNKNOWN) | **GTK 3, cairo, Python 3, python3-libevdev, python3-pyudev** | 528 + Python |
| libwayland-*, libxkbcommon0, xkb-data, libdrm2, libpixman, libdisplay-info1, fontconfig, fonts-dejavu-core | 1.22.0, 1.6.0, 2.41, 2.4.120, 0.42.2, 0.1.1, 2.15.0, 2.37 | compositor path, fonts | small; xkb-data is 4.1 MiB | about 8 MiB together |
| Mesa: libgbm1, libegl-mesa0, libgl1-mesa-dri, libglx-mesa0, libgles2, libegl1 | 24.0.5 (libglvnd 1.7.0) | OpenGL ES for driftwm (Mesa software rendering `llvmpipe` in the test VM, `LIBGL_ALWAYS_SOFTWARE=1` in `hub.build`) | **LLVM 17.0.6 (120.8 MiB), libicu, libxml2, libelf, X11 and XCB libraries** | 33,020 for the drivers |
| mesa-vulkan-drivers, libvulkan1 | 24.0.5 / 1.3.275 | reason not written down (UNKNOWN; driftwm does not link Vulkan) | **python3**, LLVM | 52,679 + 530 |
| Waybar | 0.9.24-1build3 | the bar (SOURCE `docs/bar-findings.md`) | 178 packages, GTK 3, gtkmm, jsoncpp, spdlog, fmt, PipeWire/Pulse/JACK client libraries, libnl, upower, dbus, libsystemd0 | 1,794 + 26 MiB of its own libraries |
| wofi | 1.4.1-1build2 | the dropdown list | GTK 3 (shared) | 175 |
| foot, foot-terminfo | 1.16.2 | test and emergency terminal (HUB-OS: no terminal starts by itself) | fcft, utf8proc, `ncurses-term` 4.3 MiB | 606 + 26 + 4,341 |
| dbus-daemon | 1.14.10-4ubuntu4 | Waybar will not start without a session bus (SOURCE `HUB-OS.md`) | expat, libsystemd0 | 369 |
| jq, wayland-utils, fonts-font-awesome | 1.7.1, 1.2.0, 4.7 | scripts, test, bar icons | libonig | 112 + 68 + 1,369 |

**Not in the image yet but planned (SOURCE `HUB-OS.md`, `docs/viewers-research.md`, `docs/proposals/remote-display-benchmarks.md`), and likely to become the next heavy trees:** PipeWire and WirePlumber (the mixer); the viewers: Moonlight (`moonlight-qt` needs Qt 6, SDL2, FFmpeg, libva, libvdpau, libdrm and several submodules, SOURCE `docs/viewers-research.md` line 12), `remote-viewer` (GTK 3 and SPICE, BELIEVED), wlvncc or TigerVNC (which viewer is undecided, SOURCE `HUB-OS.md` open questions); a clipboard tool (`wl-clipboard`); the bespoke file manager; the input-sharing helpers. **If these are added with distribution packages, Qt, FFmpeg and a second GTK stack arrive, and the totals in 2.3 grow considerably (BELIEVED).** This is the strongest practical argument for deciding the viewer set before the plumbing.

---

## 3. Component by component: from source, leaner choices, our own, real work or plumbing

**Which parts are the hub's real work** (this is what the hub is for; nobody else will write it for us): **driftwm** (we carry patches against a single-maintainer pre-1.0 project, SOURCE `HUB-OS.md`), **hubd** and the **session layer** (window matching, "go to the existing window", records), the **panel** (bar item, dropdown, alert) and the **audio mixer**, the **image build and update tool** with stage 0 and recovery, and the later **file manager**, **clipboard bridge** and **input forwarder**. **Everything else is plumbing:** kernel, glibc, the toolchain, busybox, s6, eudev, seatd, libinput, wayland libraries, xkbcommon, libdrm, Mesa and LLVM, fonts, PipeWire, D-Bus. Plumbing should be built the cheapest, most boring way that is reproducible; real work is where our time and our tests should go.

### 3.1 Can it be built from source here, with what, and what is leaner (BELIEVED unless marked; "T" = TESTED)

| Component | From source here? Needs | Leaner or alternative | Own replacement realistic? | Kind |
|---|---|---|---|---|
| Linux 6.12 | Yes, already (SOURCE `phase-b-desktop.md`: 279 s) gcc, make, flex, bison, libelf, bc, openssl headers, rsync for `headers_install` (T: **rsync was missing on this host**, I unpacked it with `dpkg -x`) | tinyconfig fragment is already lean | no (owner: upstream) | plumbing |
| Toolchain (binutils, gcc) | Yes. LFS builds it in two passes with the host as bootstrap. T: the **host gcc 13.3 of Ubuntu has hardening defaults that broke glibc** (see 3.2) | none | no | plumbing |
| glibc 2.39 | **Yes (T, 585 s, 3 jobs)**: gawk, bison, python3, make, gcc, kernel headers | trim: no nscd, no locales, no gconv, stripped `libc.so.6` is 1.9 MB (T) | no | plumbing |
| busybox 1.37.0 | **Yes (T, 55 to 68 s)**; bit-identical twice (T) | toybox; or only the applets we use (`ip`, `udhcpc`, `watchdog`, `ps`, `sh`...) | no | plumbing |
| skalibs 2.14.4.0, execline 2.9.7.0, s6 2.13.2.0 | **Yes (T)**: a C compiler only; execline 10 s, s6 17 s. T: their `configure` defaults to **static** linking and needed `--disable-allstatic` for a shared build | `s6-rc` and `s6-linux-init` exist upstream (earlier test: seconds); dinit is the other candidate (in no Ubuntu archive, SOURCE `phase-b-image.md`) | init pieces: BELIEVED realistic and small (init is 56 lines of shell today); owner's rule: only after a measured benefit | plumbing |
| eudev 3.2.14 | Yes (T, 39 s): gperf, libc | `mdevd`, `libudev-zero` (builds in 1 s, SOURCE `systemd-libraries.md`; untested with libinput on hardware) | no | plumbing |
| seatd / libseat 0.8.0 | Yes (T in `systemd-libraries.md`, 2 s): meson, ninja | none needed | no | plumbing |
| libinput 1.25 with libevdev, mtdev | Yes: meson; build **without** `debug-gui`, `tests`, and without libwacom unless a tablet matters (BELIEVED option names; source not read). This removes GTK and Python from this path | none | no | plumbing |
| wayland, wayland-protocols, libxkbcommon (xkeyboard-config data in place of `xkb-data`), libdrm (AMD only), pixman, libdisplay-info, expat, libffi, zlib | Yes, all small meson or autotools builds | none | no | plumbing |
| fonts and text: freetype, harfbuzz, fontconfig (or none if the panel loads font files directly) | Yes | a panel that loads one font file directly skips fontconfig (BELIEVED) | no | plumbing |
| **Mesa 24.0.5 or newer, with LLVM** | Yes in principle (meson, LLVM, libelf, python3 + mako at build time, glslang/spirv tools only for Vulkan). **Not built here.** BELIEVED hours (earlier doc: "Mesa, Qt, FFmpeg, PipeWire would take hours", SOURCE `phase-b-image.md`) | **Only AMD:** `-Dgallium-drivers=radeonsi` plus `llvmpipe` (the test VM needs software drawing; without llvmpipe the QEMU hub test has nothing to draw with, or only the very slow `softpipe`); Vulkan only if something uses it. LLVM built with the AMDGPU and X86 targets only (BELIEVED to cut `libLLVM` a lot, size not measured). **Mesa main has an option `amd-use-llvm` (default true) and radeonsi can be configured without LLVM** (SOURCE: `meson.build` lines 57 and 250, `meson.options` line 438, gitlab.freedesktop.org/mesa/mesa main, read 2026-10-05). Whether radeonsi without LLVM is good enough on the owner's GPU is UNKNOWN | no | plumbing (biggest) |
| GPU firmware (amdgpu) | **Not source.** Binary blobs from the `linux-firmware` repository | none; load only the files for the chosen GPU | no | plumbing, **blocks "everything from source"** |
| **driftwm** | **Yes (T, 7 min 6 s, shared CPU)**: rustc and cargo (edition 2024), clang and libclang and cmake (the repo's build root installs them; which crate needs them is UNKNOWN), the dev libraries, network for crates. 364 packages in `Cargo.lock`, 229 compiled (T). `smithay` is a **git dependency** pinned to a revision. The snapshot's `rustc` is too old, so the repo bind-mounts the host's rustup toolchain | the Rust toolchain itself from source needs LLVM and hours (BELIEVED); the practical alternative is the official pinned binary toolchain with its hash (question 4) | **an own compositor is allowed only after a design discussion** (SOURCE `CLAUDE.md`); driftwm is on the order of 100,000 lines of Rust (a `wc` over `src/` gave about 129,000, which may double count) built on smithay; a rewrite is a very large job (BELIEVED) | **real work** |
| **hubd** | yes (T, Go, static, already built by the repo) Go toolchain | none | it is ours | **real work** |
| **Waybar** 0.9.24 | Yes with meson, but it needs GTK 3, gtkmm 3, glibmm, jsoncpp, spdlog, fmt, libsigc++ and more; most modules can be switched off at build time (BELIEVED option list not read). Build **needs** D-Bus session bus at run time | **yambar** (C, pixman and fcft, no GTK; script modules; click handlers) and similar bars. BELIEVED from my memory of the projects; **not read, not built, not tested here**. Whether any of them can show hubd's JSON alert and open a dropdown on click is UNKNOWN | **Yes, realistic** (see 3.3) | real work |
| **wofi** 1.4.1 | Yes; GTK 3 | **fuzzel**, **tofi**, **bemenu** (no GTK; BELIEVED, not tested). `HUB-OS.md` says a click on the bar item opens the list and hubd fills it | **Yes, realistic**, part of our panel | real work |
| foot | Yes; fcft, utf8proc, tllist, wayland | keep; or drop from the hub image if the recovery shell is enough | no | plumbing |
| dbus-daemon | Yes (autotools or meson, expat). Build with `--disable-systemd` to drop `libsystemd0` (BELIEVED; `systemd-libraries.md` tried this idea for other packages) | **disappears if the panel is ours.** PipeWire/WirePlumber may or may not need it (UNKNOWN) | no | plumbing |
| PipeWire, WirePlumber (not in the image yet) | Yes (meson; BELIEVED a few dozen packages' worth of options; builds in minutes, not hours) | the mixer is our own UI on top | the **mixer** is real work; the server is plumbing | mixed |
| Viewers (Moonlight, remote-viewer, wlvncc/TigerVNC) | Moonlight: Qt 6, SDL2, FFmpeg: hours (BELIEVED). remote-viewer: GTK 3, SPICE, GStreamer (BELIEVED). wlvncc: small, built from source in the benchmark (SOURCE `remote-display-benchmarks.md`) | wlvncc was the cheapest in CPU: 25 % of a core against 110 % for `remote-viewer` in the scrolling test (SOURCE, TESTED by that document) | **a viewer of our own** was named as an option by the owner (SOURCE `HUB-OS.md` open question) | real work if chosen |

### 3.2 The from-scratch test in detail (TESTED)

**Method.** `/tmp/fsh/build.sh` (kept outside the repo; the commands are in the Appendix). Host: Ubuntu 24.04, gcc 13.3.0, 4 CPUs shared. Six source tarballs downloaded over HTTPS and checked against pinned SHA-256 values before use (the script stops on a mismatch):

| Tarball | Size | SHA-256 | Where the value comes from |
|---|---|---|---|
| linux-6.12.112.tar.xz | 148.6 MB | `164dc9d1f6c93c61a15e1f071c48379b467f2b17c469cce7223471968208ed03` | **matches** `sha256sums.asc` at cdn.kernel.org (SOURCE, read 2026-10-05) |
| glibc-2.39.tar.xz | 18.5 MB | `f77bd47cf8170c57365ae7bf86696c118adb3b120d3259c64c502d3dc1e2d926` | computed after download; the `.sig` file was downloaded but **not verified** (no keyring). Trust on first use |
| busybox-1.37.0.tar.bz2 | 2.6 MB | `3311dff32e746499f4df0d5df04d7eb396382d7e108bb9250e7b519b837043a4` | computed after download; not compared with an upstream value |
| skalibs-2.14.4.0.tar.gz | 248 KB | `0e626261848cc920738f92fd50a24c14b21e30306dfed97b8435369f4bae00a5` | same |
| execline-2.9.7.0.tar.gz | 117 KB | `73c9160efc994078d8ea5480f9161bfd1b3cf0b61f7faab704ab1898517d0207` | same |
| s6-2.13.2.0.tar.gz | 261 KB | `c5114b8042716bb70691406931acb0e2796d83b41cbfb5c8068dce7a02f99a45` | same |

Everything not listed was used as it is on the host: gcc, binutils, make, bison, perl, python3; and, unpacked with `dpkg -x` (no install): **gawk 5.2.1 and rsync 3.2.7** (neither was on this host).

**Steps and times.**

| Step | Time | Note |
|---|---|---|
| Verify the six hashes | under 1 s | |
| Kernel headers (`make headers_install`) | 397 s | almost all of it unpacking the 150 MB tarball and the header scripts under shared CPU; needs `rsync` |
| glibc 2.39 (`configure`, `make -j3`, `make DESTDIR=... install`) | **585 s** (first good run), 1,090 s the second time because the machine was busier | `--prefix=/usr`, `--enable-kernel=5.4`, no nscd, no crypt, no profile, no selinux |
| busybox 1.37.0 (`defconfig` without `tc`, dynamic, `--sysroot`) | 55 to 68 s | 1,063,528 bytes |
| skalibs, execline, s6 (shared, dynamic) | execline 10 s, s6 17 s | skalibs a few seconds (not timed in the last run) |
| Hand assembly of the root | seconds | copy the shared libraries, `ld.so`, the binaries, busybox applet links |

**What went wrong first (these are the real costs of "style of Linux From Scratch" with an Ubuntu host):**
1. `make headers_install` failed with error 127: **rsync is not installed** on the host.
2. glibc `configure` stopped: **gawk missing** (the host has only mawk).
3. glibc `make` failed in `syslog.c` with "inlining failed ... always_inline": **Ubuntu's gcc turns on `_FORTIFY_SOURCE` by default**, which glibc cannot be built with. Fix: `CFLAGS="-g -O2 -U_FORTIFY_SOURCE"`. This is the reason the Linux From Scratch book builds its own clean compiler first. I did not build a clean compiler; with Ubuntu's gcc as the compiler, the result also depends on the host's compiler defaults (PIE, stack protector, CET): BELIEVED, not checked.
4. glibc `make` failed in `nss/makedb.c`: **selinux header not found** (configure found the host's libselinux). Fix: `--without-selinux`.
5. A full disk (the shared `/tmp` filled up; other helpers' builds) stopped one glibc run. Fix: delete my old build trees.
6. The s6 family's `configure` links **statically by default** (`allstatic`), which does not work when only shared libraries were built; `--disable-allstatic` fixed it.

**Result.**

| Measure | Value |
|---|---|
| Programs in the root | busybox and its applet links, 129 ELF files (s6, execline, glibc helpers), all with interpreter `/lib64/ld-linux-x86-64.so.2` and nothing else (T, `readelf`) |
| `libc.so.6` run inside the root | "GNU C Library (GNU libc) stable release version 2.39." (T) |
| `ld.so --list` of `s6-svscan` and `busybox` inside the root | only `libskarnet`, `libc`, `libm`, `libresolv` and the loader, all from `/usr/lib` of the new root (T) |
| `busybox sh` and `execlineb` | ran (T) |
| `s6-svscan` supervising one service for 3 seconds | the service ran and s6 restarted it three times (T) |
| Size, unstripped (glibc built with `-g`) | 24,028 KiB, 145 files, squashfs not measured |
| Size, stripped | **7,972 KiB unpacked, 145 files, squashfs zstd 2,899,968 bytes (2.9 MB)**; `libc.so.6` 1.89 MB; busybox 1.06 MB (T) |
| Reproducibility | `libc.so.6` sha256 `084c9364ee1a230cf21aeaae0a91ac6987b8a59086de42a0876e7a8ab030b600`, `libm.so.6` `17eea31e...4c0e`, `ld-linux-x86-64.so.2` `f99811b8...b371`, and busybox `cb618380c0c058c4f204ef05a581c78cdd39342b3abf277aa4af40b103ee08a5` were **identical in two separate builds** (T). Same host, same directory path, same `SOURCE_DATE_EPOCH`; a different build path was not tried (UNKNOWN) |
| Equal to the earlier Buildroot result? | the earlier test gave 9.5 MB unpacked and 2.9 MB squashfs with glibc 2.41 (SOURCE `phase-b-image.md` 1.2); mine is 2.9 MB squashfs too, with glibc 2.39, with about 11 minutes of compiling against the earlier 28 minutes (host gcc as bootstrap, no compiler build) |

This proves that the plumbing at the bottom (libc, shell, init) builds quickly and reproducibly. It proves **nothing** about Mesa, LLVM, Rust or the other 30 packages.

### 3.3 Our own panel instead of Waybar and wofi: what is realistic

(BELIEVED throughout; nothing here was built.)

What the hub needs from the panel (SOURCE `HUB-OS.md`): an always-visible bar item with an alert ("3 of 4 up" in red), a click opens a scrollable list grouped by role with guests nested, a master volume button opens a mixer with a slider and mute per node, a clock, an indicator of who owns the keyboard. Today that is Waybar (one custom module and a clock in `waybar.json`) plus wofi opened by `hubd menu`. **Waybar and wofi are used for a small part of their features; the cost is GTK 3, about 130 MiB and a D-Bus session bus.**

A panel of our own is a Wayland client that speaks `wlr-layer-shell` (the bar on the top edge), draws text and rectangles, and reads hubd's socket. The pieces: a Wayland client library (C libwayland; or pure Go; or Rust's smithay-client-toolkit), text drawing (fcft or freetype + harfbuzz + pixman; the hard part is correct text, not shapes), a pop-up surface for the dropdown and mixer (xdg-popup or a second layer surface), keyboard and pointer handling, scrolling. It does not need a toolkit. Whether to write it in Go (same language as hubd, fewer toolchains in the image) or C or Rust is a design question for the owner (question 6). **Realistic: yes. Size: the largest single piece of new code in this document** (my estimate, no measurement). Benefit that can be verified: removes the GTK block (132 MiB, 101 packages), the audio client libraries, D-Bus and `libsystemd0`, removes a process that starts on every click (wofi), and makes the mixer a part of the same program. Costs: text rendering and pointer/keyboard edge cases are ours to fix; Waybar's polish is lost.

**Leaner existing programs** (yambar, fuzzel, tofi, bemenu, ...) would drop GTK without new code, but I have not checked that any of them can do the alert and the dropdown the way `docs/bar-findings.md` needs. They are a cheaper first step than writing our own, **if** they fit (UNKNOWN until read and tried). Note the owner's rule 4: build it ourselves only when it measurably wins; a leaner existing program is not "us building it" but also not what Ubuntu ships.

### 3.4 A viewer or session manager instead of `remote-viewer`

`HUB-OS.md` leaves the viewer open for non-gaming nodes and keeps `remote-viewer` for VM guests. In the benchmark `remote-viewer` used a full core for scrolling text, dropped 22 to 39 % of video frames at 1080p and never sends the clipboard hub to node; wlvncc used 25 % of a core and dropped 5 to 16 % (SOURCE and TESTED by `docs/proposals/remote-display-benchmarks.md`, items 2 and 3). A viewer of our own (a VNC client on neatvnc's or libvncclient's code, or a wlvncc fork) is realistic in the sense that wlvncc exists, is small and is built from source already; **the clipboard code is missing from wlvncc (SOURCE, same document, item 4)**, which is exactly what the clipboard bridge design needs. That points to "fork or extend a small viewer" rather than "write one from zero". For Moonlight, the only client for Sunshine, a replacement is not realistic (BELIEVED: it is a protocol implementation plus hardware decoding; the owner forbids a new streaming protocol, not a new client, but a client is large). Session management (matching windows, "go to the existing window") is hubd's job today and stays real work.

### 3.5 Our own init pieces

The init is 56 lines of shell around s6 and `hubos-ctl` is 290 lines; stage 0 is 113 lines. They are ours already. Replacing s6 itself needs "a measured benefit and owner approval" (SOURCE `CLAUDE.md`); I found none to measure. What a from-scratch base changes is only where the s6 binaries come from (source, 3 packages, 30 s to build, T).

---

## 4. The base: ways to produce a from-scratch glibc root

The repo's tools only need a **directory tree** from this step: `build-base.sh` writes `$WORK/base` and `base.tar`, and `build-root-image.sh` copies it, overlays `image/rootfs`, strips, checks libraries and squashes it (SOURCE, read above). The kernel, stage 0, A/B slots, bundles, signing and the QEMU tests do not look inside the root except for the tests' checks (no systemd unit directory, no unresolved library). So every option below plugs in at one seam: replace `build-base.sh` with a script that produces the same directory from a source manifest. `strip-root.sh` would no longer be needed (nothing to delete). `check-libs.sh` uses `ldd`, which a busybox-only root does not have; it would call `ld.so --list` instead (T: that works, see 3.2).

| | (A) Our own scripts, Linux-From-Scratch style, Ubuntu host as bootstrap | (B) Buildroot with an external tree | (C) Yocto / OpenEmbedded | (D) others: Void's xbps-src, Gentoo, Nix/Guix |
|---|---|---|---|---|
| **What it is** | shell recipes per package, a hash manifest, a build order | a defconfig plus `BR2_EXTERNAL` directory with our recipes | BitBake, layers, recipes, `local.conf` | other source-based distributions |
| **Tested here** | **Yes**: glibc, busybox, s6 root (section 3.2) | not now; earlier: 27.9 min, 7.4 GB tree, 9.5 MB root, 2.9 MB squashfs, glibc 2.41, **no** Waybar/wofi/driftwm/PipeWire packages then (SOURCE `phase-b-image.md`) | **No** | **No** (package pages only) |
| **Packages that already exist for our list** | none; we write every recipe | **SOURCE** (`package/Config.in` at tag 2026.08, gitlab.com/buildroot.org/buildroot, read 2026-10-05): foot, seatd, pipewire, wireplumber, mesa3d, libinput, eudev, s6, s6-rc, wayland, wlroots, sway, dbus, libxkbcommon, fontconfig, spice. **Not present**: waybar, wofi, driftwm, dinit, virt-viewer (Rust packages live in another file, not checked) | **SOURCE** (layers.openembedded.org API, read 2026-10-05; the aggregate index, which layer each hit belongs to was not resolved): seatd 0.9.3, eudev 3.2.14, mesa 26.2.2, wayland 1.26.0, weston 16.0.0, pipewire 1.6.8, libinput 1.31.3, foot, sway, labwc, wlroots, virt-viewer, waybar (several versions). **None found**: wofi, driftwm, s6, skalibs, dinit | **SOURCE** (page or template status 200 or 404, read 2026-10-05): Void srcpkgs has wofi, foot, seatd, mesa, wlroots, dinit, s6, eudev, libinput, pipewire, virt-viewer, wayvnc, **404 for waybar (name may differ) and driftwm**. Gentoo: waybar, wofi, foot, seatd, mesa, s6, libinput, pipewire, virt-viewer, wayvnc; **404 for driftwm and eudev** |
| **How a package set is declared** | our manifest file (name, version, sha256, build flags); we design it | one `defconfig` plus `.hash` files that Buildroot checks per package (BELIEVED, `.hash` files exist for every package) | `IMAGE_INSTALL` in a config, layer revisions pinned by a tool such as kas (BELIEVED) | Void: template per package; Gentoo: USE flags and profiles; Nix: a lock file (BELIEVED) |
| **Reproducibility** | what we write: busybox and glibc bit-identical twice (T); no tooling for the rest | `BR2_REPRODUCIBLE` option exists (BELIEVED; the earlier test did not build twice) | a stated goal with hash equivalence and a reproducibility test (BELIEVED; not checked) | Nix and Guix: designed for it (BELIEVED) |
| **Build time** | glibc 10 min, plumbing about 3 min, **Mesa/LLVM hours** (BELIEVED), Rust toolchain: use a pinned binary or hours | 28 min for the minimal root (T earlier); Mesa+LLVM, driftwm toolchain add hours (BELIEVED) | longest first build (BELIEVED, many hours), shared state cache makes later builds fast (BELIEVED) | not measured |
| **Disk use** | small: my whole test tree was 0.9 GB of sources and builds, sys tree 116 MB | 7.4 GB for the minimal root (T earlier) | tens of GB (BELIEVED, not measured) | not measured |
| **Taking later updates** | we watch every upstream and bump it; per-package control; **stability, not security, drives it** (owner: security is not a concern) | move to a new Buildroot release (quarterly; 2026.08 and 2026.05.3 exist, SOURCE `buildroot.org/downloads`) or bump one package in the external tree | follow a supported series: **6.0 "wrynose" and 5.0 "scarthgap" are listed as supported** (SOURCE docs.yoctoproject.org/releases.html, read 2026-10-05); built-in CVE check tooling (BELIEVED) | depends on the distribution |
| **Fit with the repo's tools** | best: one seam, plain shell like the rest of `tools/image` | good: output is a root tree or tar (BELIEVED); A/B, signing and tests unchanged | acceptable: output is an image; we would use only the root tree. Heavy toolchain beside a repo that is small | Nix store layout is not a normal `/usr` root (BELIEVED), the largest change |
| **Effort (my estimate, BELIEVED)** | high at first, then steady: one recipe per package (about 40) and the machinery (ordering, patch handling, cache, source manifest, licence list) | medium: learn Buildroot; write recipes for driftwm, the panel, hubd, a viewer, a few missing libraries | highest to learn; recipes mostly exist | not assessed |
| **Matches "optimised for the hub's one use"** | full control of every flag | full control through the config, Buildroot's own tuning options | full control | USE flags (Gentoo) are the most natural for this |
| **Risk to uptime** | every recipe is ours to get right | upstream-tested recipes for the plumbing | upstream-tested recipes | n/a |

**What the measurements point to (not a decision):**
- For the **bottom of the stack** (glibc, busybox, s6) option A is fast, small and was reproducible in my test, but it needed six fixes before it worked, because the Ubuntu host's compiler is configured differently from what glibc expects. A pure Linux-From-Scratch toolchain pass would avoid some of them at the cost of building gcc (the earlier Buildroot test did this: about 28 minutes in total, BELIEVED mostly the compiler twice).
- For the **middle** (wayland, libinput, Mesa, LLVM, PipeWire) every option needs the same upstream builds; Buildroot and Yocto already have tested recipes, option A has none. This is where option A costs the most of our time and where Buildroot or Yocto save it.
- For the **top** (driftwm, hubd, panel, viewer, mixer) every option needs our own recipes. Buildroot's Go and Cargo support exist (BELIEVED, not tested); driftwm needs vendored crates in any case for a reproducible, offline build (BELIEVED).
- Nothing here can remove the **firmware blobs**.

Which of these to choose, or a mix (for example Buildroot for the middle and our own scripts for the top), is question 1.

---

## 5. A staged plan, today's image to a from-scratch hub

**Rules for every stage.** The stage is a new image built the normal way and offered through the A/B update; if it fails to boot or to confirm, the machine goes back to the previous slot (SOURCE `HUB-OS.md`). So a failed stage costs a test run, not a dead hub. At the end of each stage `TestImage` and `TestHubImage` (`go test -tags qemu -count=1 -timeout 150m -v ./tools/image`, 12 to 20 minutes, by hand, SOURCE `docs/image.md`) must pass without changing a test, plus the new checks listed. Costs are in bot sessions (my guess, BELIEVED) and compute time; "owner time" is the decisions and the December checks on real hardware.

Order: biggest dependency trees first, but cheap removals of unused things go before expensive rebuilds.

| Stage | What changes | Removes | Test (besides the two QEMU tests) | Cost |
|---|---|---|---|---|
| **S0: measure and record** | add a size-and-reason report to the image build (per package: size, who needs it, why it is on the list) and a source manifest format (`name version url sha256`) that eudev and driftwm already fit | nothing | the report is produced; a build fails if a manifest hash does not match (the eudev check in `build-hub-parts.sh` is the model) | small, 1 session |
| **S1: trim without rebuilding** | delete unused files in the existing strip step: Intel/Nouveau/virtio/lavapipe Vulkan files (about 41 MB), drop `libinput-tools` (if the `libinput` tool is not needed in the image; asks question 8), possibly `mesa-vulkan-drivers` and `libvulkan1` with it (only if nothing uses Vulkan, UNKNOWN: driftwm does not link it); that also removes **Python (27 MiB) and GTK's third user** | up to about 70 MiB (BELIEVED, to be measured), Python | `check-libs.sh` (already) says nothing is left unresolved; the hub QEMU test (bar pixels, hubd menu click) still passes | small, 1 session, the safest first step |
| **S2: our own panel (bar, dropdown, mixer)** | new program, started by s6 as the user `hub`; Waybar, wofi, dbus-daemon, fonts-font-awesome removed | the whole GTK block (about 132 MiB, 101 packages), audio client libraries (13 MiB), D-Bus, and `libsystemd0` (the last systemd library the owner tolerates for Waybar) | the existing hub QEMU test checks bar pixels and the click-through to the menu; it must be extended to the new program (that is test work) and a new check "no `libsystemd` in the root" is added | **large: the biggest new code in the plan** (3.3). 4 to 8 sessions (BELIEVED), design discussion first (the panel is on the owner's list of parts we write; its design is question 6) |
| **S3: build the graphics path from source** | Mesa (AMD + llvmpipe), LLVM (needed targets), libdrm, libgbm, wayland, xkbcommon, libinput, seatd, pixman, libdisplay-info, freetype/harfbuzz, built from pinned tarballs in the build root and installed over the Ubuntu ones; `xkb-data` replaced by xkeyboard-config | Ubuntu's Mesa/LLVM block (about 207 MiB → target to be measured) and 8 MiB of small libraries; ICU and libxml2 if LLVM is configured without them | driftwm still starts on the virtual GPU (llvmpipe) in the hub QEMU test; libraries match driftwm's NEEDED list; a **real AMD GPU cannot be tested until December** (UNKNOWN) | **medium to large, mostly waiting**: LLVM and Mesa builds take hours (BELIEVED), 3 to 5 sessions |
| **S4: our own toolchain, userland and init pieces** | glibc, busybox, skalibs/execline/s6, eudev built from the manifest into a new base; coreutils, bash, util-linux, perl, dpkg, e2fsprogs, openssl removed from the image (their use by scripts replaced by busybox applets; `hubos-ctl` and the stage 0 scripts already use busybox sh, to be checked) | the Ubuntu base (about 79 MiB, 72 packages) and `dpkg`'s database | `check-libs.sh` using `ld.so --list`; the two QEMU tests; bit-identical rebuild of every component built twice (as in 3.2) | medium, 3 to 4 sessions; the hardest part is finding every place an Ubuntu tool is used (UNKNOWN until tried) |
| **S5: the base builder** | replace `mmdebstrap` by the chosen builder (question 1); apt, the snapshot date and the pin file disappear | the whole package system | full rebuild twice gives identical roots; build time and disk recorded | depends on the builder: A high, B medium, C highest (4.) |
| **S6: viewers, PipeWire, file manager from source** | these arrive from source from the start, never as Ubuntu packages | prevents Qt/FFmpeg/GTK trees from entering | the QEMU test grows a check per viewer window title (existing design) | per component |
| **S7: driftwm toolchain and vendored crates** | pinned Rust binary toolchain (or from source, question 4), `cargo vendor` of the 364 crates with their checksums stored with the manifest, offline build | the network at build time, host Rust bind-mount | rebuild twice identical; build offline | small to medium, 1 to 2 sessions |
| **S8: tuning the hub's own flags (only if measured)** | compiler flags (`-march` for the owner's AMD CPU), LTO, link-time garbage collection | maybe seconds of start time, megabytes of memory | **measure first** (start time to first bar, resident memory with 20 windows); do it only if it measurably wins (rule 4) | small, but only after December |

Stages S1 and S2 have the best ratio of weight removed to effort; S3 removes the most weight but costs the most waiting and cannot be fully tested without real hardware; S4 and S5 are mechanical and become safe only after S0's checks exist. S6 and S7 can happen at any time after S0 and should be decided before the viewers are added to the Ubuntu image.

**Hub uptime during all this (priority 1):** each stage changes one block; the two QEMU tests and the A/B rollback protect the hub; no stage changes how the hub restarts services (the hub never reboots itself because of a service failure, SOURCE relayed rules). What is **not** covered: the QEMU tests run in software emulation without KVM and cannot show a graphics crash, a memory leak after weeks, or GPU driver faults. A from-scratch Mesa is the largest stability risk in the plan (BELIEVED), because the Ubuntu one has had years of distribution testing on AMD hardware; the December hardware test must run S3's image for a long time before it replaces the Ubuntu one on the real hub.

---

## 6. Questions for the owner

1. **Base builder.** Our own scripts (A), Buildroot with an external tree (B), Yocto (C), or a mix (for example Buildroot for the middle and our own scripts for hub-specific parts)? Section 4 lists what each costs; I tested only A.
2. **GPU firmware and CPU microcode.** An AMD GPU cannot start without binary firmware files from `linux-firmware`, and a CPU may want a microcode update. "Everything built from source" cannot include them. Are firmware blobs allowed in the hub image (the owner already refused proprietary BMC firmware)? Which GPU does the hub have (open question in `HUB-OS.md`)?
3. **Which glibc version.** The image has 2.39 (Ubuntu 24.04), my test built 2.39, the earlier Buildroot test used 2.41. Do you want to stay on 2.39 for the hub, or follow a newer release (a newer one needs checking against the gaming box and nodes only if they must match)?
4. **Rust toolchain for driftwm.** Build rustc from source (needs LLVM, hours), or take the official pinned binary toolchain with its hash (a bootstrap binary, not built by us)? The same question for Go (hubd) and for the compiler used to bootstrap everything (Ubuntu's gcc today, or a clean one we build first).
5. **How strict is "from source"?** Are pinned official binaries with a SHA-256 allowed anywhere (Rust, Go, firmware), as the gaming box already does for emulators? And is trust-on-first-use for a source tarball's hash acceptable, or must each hash be checked against upstream's signature (my test only checked the kernel's)?
6. **The panel.** (a) Try an existing small bar and list launcher first (yambar, fuzzel or similar) or go straight to our own? (b) If our own: Go, C or Rust? (c) Is the mixer part of the same program (my assumption) or a separate one?
7. **Vulkan.** Does anything on the hub need Vulkan (`mesa-vulkan-drivers` is in the list with no reason)? If not, it goes in stage S1 and removes Python as well.
8. **`libinput-tools`.** Is it needed in the hub image (it pulls GTK 3 and Python for debug tools)? If it is wanted for support work, may it be built without the GUI and Python parts?
9. **Test GPU path.** The QEMU tests draw with Mesa's software renderer (`llvmpipe`), which needs LLVM. A from-scratch Mesa without LLVM would leave the test VM with the very slow `softpipe`. Do you accept keeping LLVM in the test image only, with a smaller LLVM-free build for the real hub? (This makes the test image differ from the real image; I cannot judge the risk.)
10. **Which viewers.** The biggest future dependency trees (Qt 6, FFmpeg, GTK 3 with SPICE) come with Moonlight and `remote-viewer`. Please decide the viewer set before stage S6; one of them (wlvncc) is small but has no clipboard code. Fork a small viewer, or keep the large ones?
11. **Compiler flags.** Optimising for the owner's AMD CPU (`-march`, LTO) makes images tied to that hardware and to the QEMU test CPU. Only after December measurements, or never?
12. **Who watches upstream?** With 40 sources instead of 288 packages someone must read release notes and decide bumps. Is that the bot, with each bump a reviewed pull request that must pass the QEMU tests? (Security is not a design concern, so I assume the driver is stability fixes only.)
13. **Kernel updates.** Linux 6.12 is a long-term series; the image pins it. Follow its point releases (the kernel tarball I used, 6.12.112, is the newest I saw on 2026-10-05) as part of the same manifest?
14. **Order of stages.** Is "trim, panel, graphics, base" the order you want, or should the viewer set and the base builder (S5/S6) come before the panel?

---

## 7. What I did not verify (list)

- Build times and sizes of Mesa, LLVM, Rust, PipeWire, wayland, libinput, Buildroot with our recipes, Yocto: BELIEVED, not measured.
- That a root built with this base boots under stage 0 and s6 as pid 1 in QEMU.
- That the options I name for libinput, Waybar, dbus and Mesa exist and do what I say (only Mesa's `amd-use-llvm` was read).
- That yambar, fuzzel, tofi or bemenu can do the alert and the dropdown.
- The reason `mesa-vulkan-drivers`, `libvulkan1` and `libinput-tools` are in `hub.list`.
- That the glibc and busybox hashes equal upstream's published values; that bit-identical output also holds for a different build path or a different host.
- The real size of the finished hub root with `hubd`, kernel and recovery kernel; the QEMU tests on the current head.
- Everything about real hardware.

## 8. Sources and files

- Repository files read on 2026-10-05: `HUB-OS.md`, `CLAUDE.md`, `image/machines/hub.build`, `image/packages/hub.list`, `image/kernel/hub.frag`, `image/machines/hub/rootfs/...`, `tools/image/*.sh`, `docs/image.md`, `docs/proposals/phase-b-image.md`, `phase-b-desktop.md`, `systemd-libraries.md`, `remote-display-benchmarks.md`, `docs/viewers-research.md`.
- Web, read on 2026-10-05: `cdn.kernel.org/pub/linux/kernel/v6.x/sha256sums.asc`; `gitlab.com/buildroot.org/buildroot` tag 2026.08 `package/Config.in` and `package/mesa3d/mesa3d.mk`; `buildroot.org/downloads`; `docs.yoctoproject.org/releases.html`; `layers.openembedded.org/layerindex/api/recipes/`; `gitlab.freedesktop.org/mesa/mesa` main `meson.build` and `meson.options`; `raw.githubusercontent.com/void-linux/void-packages` templates; `packages.gentoo.org`; source tarballs at `ftp.gnu.org`, `busybox.net`, `skarnet.org`, `cdn.kernel.org`.
- Temporary directories (`/tmp/hubwork`, `/tmp/fsh`) and everything built in them were deleted when finished. Nothing was installed on the machine; `rsync`, `gawk` and the repo's tools were unpacked with `dpkg -x` into the temporary directory.

## Appendix: the from-scratch test, condensed (outside the repo; paths shortened)

```
# 1. fetch and check (sha256 values in section 3.2)
curl -sSLO https://ftp.gnu.org/gnu/glibc/glibc-2.39.tar.xz            # and the five others
echo "<sha256>  <file>" | sha256sum -c -
# 2. kernel headers (needs rsync)
make -C linux-6.12.112 headers_install INSTALL_HDR_PATH=$SYS/usr
# 3. glibc (needs gawk, bison, python3; the host gcc needs -U_FORTIFY_SOURCE)
mkdir obj && cd obj
CFLAGS="-g -O2 -U_FORTIFY_SOURCE" ../glibc-2.39/configure --prefix=/usr libc_cv_slibdir=/usr/lib \
  --with-headers=$SYS/usr/include --enable-kernel=5.4 --disable-werror --disable-nscd \
  --disable-build-nscd --disable-profile --disable-crypt --without-selinux
make -j3 && make DESTDIR=$SYS install
# 4. busybox, skalibs, execline, s6 with the new sysroot as the compiler's root
SCC="gcc --sysroot=$SYS -O2"
( cd busybox-1.37.0 && make defconfig && make CC="$SCC" && make CC="$SCC" CONFIG_PREFIX=$OUT install )
( cd skalibs-2.14.4.0 && CC="$SCC" ./configure --prefix=/usr --libdir=/usr/lib --enable-shared --disable-static && make && make DESTDIR=$SYS install )
for p in execline-2.9.7.0 s6-2.13.2.0; do ( cd $p && CC="$SCC" ./configure --prefix=/usr --enable-shared --disable-static \
  --disable-allstatic --with-sysdeps=$SYS/usr/lib/skalibs/sysdeps --with-include=$SYS/usr/include \
  --with-lib=$SYS/usr/lib --with-dynlib=$SYS/usr/lib && make && make DESTDIR=$SYS install ); done
# 5. assemble (shared libraries, ld.so, binaries, busybox links), strip, check, squash
strip --strip-unneeded <each ELF file>        # keep --strip-debug for ld.so
chroot $ROOT /lib64/ld-linux-x86-64.so.2 --list /usr/bin/s6-svscan
chroot $ROOT /usr/bin/busybox sh -c 'timeout 3 /usr/bin/s6-svscan /run/scan'
mksquashfs $ROOT root.sqsh -comp zstd -noappend -all-root          # 2,899,968 bytes
```

The repo's own measurement steps (base root, hub parts, squash) are the commands in section 2.1 run through `tools/image/common.sh` with `WORK=<temp dir>` and `MACHINE=image/machines/hub.build`.
