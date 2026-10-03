# Phase B, first slice: a tiny bootable Hub OS image

> **PROPOSAL; the owner's answers of 2026-10-02 are in the section 'Owner decisions'.**

**Written:** 2026-10-02. **Status of the evidence:** everything marked TESTED was run in the build environment described in `docs/environment.md` (4 CPUs, 16 GB, about 30 GB of disk, **no KVM**: virtual machines run in software emulation, so all times below are emulation times and say nothing about real hardware). Nothing here was run on real hardware. `HUB-OS.md` wins if anything here disagrees with it.

Labels, as in the other documents: **TESTED** (the command and its output are shown or listed in section 13), **SOURCE** (read in a document or source file, link given), **BELIEVED** (reasoned or remembered, not run), **UNKNOWN**.

Nothing was installed on the machine. Packages were downloaded with `apt-get download`/`--print-uris` into a temporary directory (`/tmp/pb`) and unpacked with `dpkg -x`, as in `docs/environment.md` section 1.8. Everything was deleted afterwards. The repo gets only this file: the scripts that were used are printed in the appendices (they are test scaffolding that lived in `/tmp`, not proposed repo code).

## Owner decisions (2026-10-02)

The owner approved my recommendation for all 25 choices of section 12, **except choice 2** (the systemd libraries), and with the additions on choices 19 and 24. The recommendations below are copied from section 12.

1. **Base builder.** Approved: `mmdebstrap` on Ubuntu 24.04 with a pinned snapshot date (25 s; the desktop software we need later is in the archive).
2. **The two systemd libraries.** **Temporary: keep `libsystemd0` and `libudev1` in the prototype image, undecided; the owner decides after `docs/proposals/systemd-libraries.md`.**
3. **Ubuntu or Debian.** Approved: Ubuntu 24.04 (already used for the other documents).
4. **Init.** Approved: s6 with plain `run` scripts now, s6-rc built from source later.
5. **Where PID 1 comes from.** Approved: a small `/sbin/init` shell script that ends in `exec s6-svscan` (as tested).
6. **Boot method.** Approved: UEFI only, EFI-stub kernel, firmware `BootNext`/`BootOrder`, no bootloader.
7. **BIOS support.** Approved: none.
8. **Stage 0 inside the kernel.** Approved: yes (busybox + a 32-line script).
9. **Root file system format.** Approved: squashfs with zstd (tested).
10. **Partition plan.** Approved: (5 partitions, sizes in 4.1). *Alternative:* add a separate `/var/log` partition, or shrink the boot partition.
11. **Signing tool.** Approved: `signify-openbsd`.
12. **What is signed.** Approved: a manifest of hashes with a version.
13. **Rollback protection.** Approved: strictly increasing version plus a floor on the config partition.
14. **Config format.** Approved: `node.conf` (shell) now, `node.toml` later.
15. **Update transport.** Approved: HTTP pull from the hub, started by the owner.
16. **Health definition for "boot succeeded".** Approved: `hubd` answers its socket five times in a row.
17. **Timeouts.** Approved: confirm timeout 120 s and watchdog heartbeat 120 s on real hardware (the tests used 40 s and 60 s).
18. **Recovery.** Approved: a firmware boot entry (reachable from the firmware's boot menu).
19. **Kernel.** Approved, as: **Linux 6.12 LTS as the default kernel; each machine's build config names its kernel version, config fragment and modules; slice 1 has no modules.**
20. **Language for the base layer.** Approved: shell for slices 0 to 3, then decide.
21. **Test runner.** Approved: a Go test (like the rest of the repo) that drives QEMU over a pipe.
22. **Where the code goes.** Approved: `tools/image/` for the build and test scripts, `image/` for the files that go into the root (init, service scripts), and `docs/` for this file once approved.
23. **Persistent logs.** Approved: `/var/log` on the data partition.
24. **Key custody.** Approved: an offline key on your computer for now. **No private key is ever committed; tests generate throwaway keys at run time in a temporary directory.**
25. **A Buildroot image for tiny roles.** Approved: look at it again only for a later tiny role (for example a pure NAS or a guest); not for the hub.

Other decisions the owner made on the same day are in `HUB-OS.md` (independent per-machine kernels, out-of-tree modules only when declared, pinned versions, OpenZFS on the NAS and backup NAS).

---


---

## 0. Summary

**What was built and run (TESTED):** a 420 MB (sparse) UEFI disk image that boots under QEMU/OVMF in software emulation, with:

- a **read-only squashfs root** (`touch /usr/x` fails with `Read-only file system`);
- a separate **config partition** that holds `node.conf`, `inventory.toml` and `viewers.toml`, and that survived an update;
- **two root slots A and B** and **no bootloader**: the kernel is an EFI-stub file on the boot partition, the firmware's own `BootNext` and `BootOrder` variables pick the slot, set with `efibootmgr`;
- an **update tool** (`hubos-ctl update`, 96 lines of shell) that fetches a bundle over HTTP, checks an **ed25519 signature** (`signify`) on a manifest of hashes, refuses unsigned, tampered, wrong-key and not-newer bundles, writes the *other* slot, and sets `BootNext`;
- **three layers of rollback** that all worked: (1) a stage-0 program built into the kernel refuses a slot that has no root or no `/sbin/init` and reboots at once; (2) a "boot succeeded" step that gives up after a timeout and reboots; (3) a hardware watchdog that resets a machine whose init hangs;
- **s6** (the `s6-svscan` supervisor) as PID 1's supervisor, restarting a killed service in under a second;
- **hubd** (static Go build from this repo, 3.7 MB) supervised by s6, reading its inventory from the config partition;
- a **recovery** boot entry that gives a bare terminal.

All the tests in section 8 passed in the final run (test 9, the interrupted update, is a weak test; see there). Earlier runs failed for reasons that are listed in section 13 (they are the useful part: several design points came from them).

**Recommendations (each is a question for you in section 12):**

1. Build the root file system with **mmdebstrap on the Ubuntu 24.04 archive** (25 s, bit-identical output twice with `SOURCE_DATE_EPOCH`, pinned by `snapshot.ubuntu.com`), not Buildroot and not by hand. Buildroot did build a 2.9 MB glibc-and-s6 root here in 28 minutes (TESTED), but it has no packages for Waybar, wofi, virt-viewer, Remmina, Moonlight, Sunshine or driftwm (TESTED: package directory listing); hand-building everything is the most work.
2. **s6** with plain `run` scripts now, and **s6-rc** (built from source, 24 s for the whole stack) when ordering between services is needed. **dinit** is in neither Ubuntu 24.04 nor Debian (TESTED) and its source could not be fetched here (GitHub downloads return 403), so it stays an untested alternative.
3. **UEFI only, EFI-stub kernel, firmware `BootNext`/`BootOrder`**, no GRUB. GRUB with `grubenv` was checked only as far as building its pieces (section 4).
4. A **stage 0 built into the kernel** that mounts the slot, arms the watchdog and reboots on any problem. This was added because a test showed that a slot without `/sbin/init` does not panic; the kernel quietly starts `/bin/sh` and the machine sits there forever.
5. **Sign a manifest of hashes with `signify`** (ed25519), keep a strictly increasing version number, and keep a "floor" on the config partition.
6. Declarative per-machine config: a sh-readable `node.conf` for slice 1; `node.toml` later.

**Honest limits:** nothing here is verified on real hardware or real firmware; boot time and performance cannot be measured; the update was interrupted by killing QEMU only once; one start of the test machine out of about twelve hung silently between the firmware and the kernel and was not reproduced (cause UNKNOWN).

**One thing for you to decide first:** the Ubuntu base pulls in two libraries from the systemd source package (`libsystemd0`, `libudev1`). No systemd program is installed and none runs, but the rule says "never use systemd, under any circumstances". Section 1 says what pulls them in and what dropping them would cost.

---

## 1. The build approach: how to make the root file system

The requirement (`HUB-OS.md`): glibc, no systemd, immutable, declarative, "build it ourselves when it measurably improves Hub OS", and later a long list of desktop software.

### 1.1 Comparison

| | (a) Ubuntu/Debian root with debootstrap or mmdebstrap | (b) Buildroot with a glibc toolchain | (c) our own scripts, package by package | (d) others found |
|---|---|---|---|---|
| **Needs** | `debootstrap` or `mmdebstrap` (both in the Ubuntu archive), root or user namespaces, a mirror | host tools (`rsync`, `gcc`, `make`, ...; `rsync` was missing here), 2,962 package recipes in the tree, downloads from gnu.org, kernel.org and (through a mirror fallback) GitHub | a C compiler, the sources, a build script per package | see below |
| **Runs here?** | **Yes, TESTED** | **Yes, TESTED** (a minimal configuration; see 1.2) | **Yes for a small set, TESTED**: the whole s6 stack from source in 24 s | Not tried |
| **Size of result** | TESTED: 96 packages, 206 MB raw, **101 MB** after removing apt's caches, squashfs **34.2 MB** (zstd, 3.4 s) or **31.6 MB** (xz, 6.2 s). With `mmdebstrap`: 83 MB directory | TESTED: busybox + glibc 2.41 + the s6 family: target tree **9.5 MB** (346 files), squashfs (zstd) **2.9 MB**. This is far less than Ubuntu's 34 MB because it has no `apt`, PAM, perl-base, util-linux, coreutils and so on; our update tool, `signify` and `efibootmgr` were not in it | smallest possible; BELIEVED a few MB for init plus busybox | - |
| **Time** | TESTED: `debootstrap` 60 s; `mmdebstrap` 25 s | TESTED: **1,671 s (27.9 minutes)** from nothing to a root file system, on 4 CPUs (about 100 steps of host tools in 5 minutes, then GCC twice and glibc, then the packages). The build tree used 7.4 GB. Each further package adds to every full rebuild | TESTED: skalibs 13 s, execline 3 s, s6 5 s, s6-rc 2 s, s6-linux-init 1 s. Mesa, Qt, FFmpeg, PipeWire would take hours (BELIEVED) | - |
| **Reproducible and declarative** | TESTED: two `mmdebstrap` runs with `SOURCE_DATE_EPOCH=1790000000` gave **bit-identical** tar files (sha256 `7f3ff52e...` both). The package list is one command line (declarative); to freeze it over time, a dated snapshot of the archive can be used (TESTED: `https://snapshot.ubuntu.com/ubuntu/20261001T000000Z/...` answers 200) | One defconfig file (declarative). Reproducible builds are a Buildroot option (BELIEVED, not tested: I did not build twice) | Only as much as we write | - |
| **Security updates later** | Rebuild the image from the current archive (or a newer snapshot); Ubuntu 24.04 gets security updates through its normal channel (BELIEVED: Canonical's published schedule runs to 2029; not checked here). We never patch by hand | Move to a new Buildroot release (quarterly, plus a yearly long-term one: BELIEVED); every package is rebuilt from source (the 28-minute build is repeated) | **We** watch every upstream and rebuild; the biggest cost | - |
| **Later software** | Archive has foot, Waybar, wofi, PipeWire, WirePlumber, Weston, sway, virt-viewer, Remmina, seatd, dbus (`docs/environment.md`, and TESTED here for seatd 0.8.0, dbus-daemon 1.14.10). Moonlight, Sunshine, driftwm are in no archive and need building anyway | TESTED (package directory listing): has pipewire, wireplumber, mesa3d, wlroots, sway, weston, foot, seatd, libinput, qt6, SDL2, ffmpeg, spice, libvirt, pcmanfm, openssh, tmux, Go, Rust, efibootmgr, the s6 family. **Has no** Waybar, wofi, virt-viewer, Remmina, Moonlight, Sunshine, driftwm, dinit | we write all of it | - |
| **Our time** | low | medium to high (learn Buildroot; write about 7 package recipes; wait about half an hour per full build) | very high | - |

**(d) Others.** *Debian* instead of Ubuntu: TESTED by reading its package index: trixie, forky and sid all have `s6` (2.13.1.0 / 2.15.1.0) and `runit`, but **none** has `dinit`, `s6-rc` or `s6-linux-init`; so Debian gains nothing over Ubuntu here. *Alpine* is musl (excluded: glibc is required). *Void Linux* (glibc and runit, no systemd): reachable (`repo-default.voidlinux.org` answers 200) but **not tried**. *Devuan* (Debian without systemd): its server did not answer from here (HTTP code 000); not tried. *Yocto*, *Nix*, *Guix*: not tried; BELIEVED heavy. `mkosi` is in the Ubuntu archive, but BELIEVED to be built around systemd tools, so not considered.

### 1.2 What happened with Buildroot (TESTED)

`make hubos_defconfig` accepted a glibc, s6, s6-rc, s6-linux-init, ext2 configuration (Appendix: commands in 13.4). The first `make` stopped at once with `You must install 'rsync' on your build machine` (a host dependency; I unpacked `rsync` into the temporary directory). A first attempt with a squashfs-zstd root stopped at `host-lz4` because GitHub archive downloads return 403 here; with an ext2 root the build went on, because Buildroot falls back to its own mirror (`sources.buildroot.net`) when the first address fails (the log shows one `404`, then success). It **finished**: exit 0 after **1,671 s (27.9 minutes)**, giving a `rootfs.ext2` image (62.9 MB, mostly empty space), a target tree of 9.5 MB and, packed by `mksquashfs -comp zstd`, **2.9 MB**. It contains glibc 2.41 (Buildroot's own toolchain), busybox, skalibs, execline, s6, s6-rc, s6-linux-init and an unconfigured skeleton. I did **not** put it in a disk and boot it: the init scripts, `hubd`, the update tool, `efibootmgr` and `signify` are not in it, so its boot is UNKNOWN (BELIEVED to work with the same stage 0 and the same disk layout).

### 1.3 The systemd libraries in the Ubuntu base (TESTED)

I asked `debootstrap` to exclude `systemd`, `systemd-sysv`, `libsystemd0`, `systemd-dev` and `udev`. The result has **no** systemd program or unit, but two libraries from the systemd source package are installed, because other packages depend on them (read from `/var/lib/dpkg/status` of the result): `libsystemd0` is needed by `apt`, `libapt-pkg6.0t64`, `libpam-modules-bin`, `libproc2-0` and `procps`; `libudev1` is needed by `libapt-pkg6.0t64`. Both are plain shared libraries; nothing starts. Options: (1) accept them as libraries (my recommendation: this is not "using systemd"); (2) remove them in the image build: an immutable image does not need `apt`, `dpkg`-based upgrades or `procps` at run time, so those could be deleted after the build (and PAM replaced by busybox `login`), which would drop both libraries; this removes also the ability to run `apt` on a node, which is the point of an immutable image but must be your choice; (3) rebuild packages without systemd support (we would carry patches; not recommended). **Your decision.**

### 1.4 Recommendation and what it means for "build it ourselves"

**Recommend (a) with `mmdebstrap` on Ubuntu 24.04**, a pinned snapshot date, a package list file in the repo, and `SOURCE_DATE_EPOCH` set. Reasons: it runs here, it is the fastest to a bootable image, the desktop software we need later is in the archive, and security fixes arrive by rebuilding.

What it means for "build it ourselves when it measurably improves Hub OS": we build **only the pieces where we can show an improvement**. Today those are: `hubd` (ours), the update tool and stage 0 (ours, 96 + 32 lines), the kernel configuration (ours, 67 lines; the kernel source is upstream), and the glue (init script, service scripts). Everything else is the distribution's. For each later replacement (for example our own init, or a replacement for Waybar) the rule in `HUB-OS.md` stays: state what ours does better and verify it. **Not recommended now:** (b) because 7 or more packages would have to be written and maintained, and (c) because it multiplies the work for no measured gain. Buildroot stays an option for a *tiny* role image later (for example a pure NAS), where its small size matters; that is BELIEVED, not tested.

---

## 2. The kernel

### 2.1 The smallest configuration that boots (TESTED)

`make tinyconfig` plus a 70-line fragment (Appendix A) gives a kernel that boots under QEMU with UEFI, a virtio disk and a virtio network card, a squashfs root, a vfat boot partition, ext4 for config and data, a watchdog, and an initramfs built into the kernel image. It has **no modules** (`=m` count: 0; `=y` count: 821).

| Measure | Value |
|---|---|
| Source | Linux 6.12 (the version used in `docs/environment.md`); source tree 1.8 GB |
| First full build, 4 CPUs | **169 s** until the last step failed (the host had no `zstd` program for the kernel's own compression) plus **15 s** to finish with gzip: about **3 minutes** in all. (The `defconfig` build in `docs/environment.md` took 441 s.) |
| Rebuilds after a small change | 6 to 15 s |
| `bzImage` | 3.0 MB without stage 0, **4.1 MB with stage 0** (a 1.1 MB static busybox inside) |
| Kernel file name on the boot partition | `EFI/hubos/kernel-a.efi`, `kernel-b.efi` (the `bzImage` itself: it has an EFI stub, `CONFIG_EFI_STUB=y`) |

Features the update scheme needs: `CONFIG_EFI`, `EFI_STUB`, `EFIVAR_FS` (to write boot variables), `SQUASHFS` with `SQUASHFS_ZSTD`, `VFAT_FS`, GPT partition tables with partition names (`EFI_PARTITION`), `DEVTMPFS`, a built-in initramfs (`BLK_DEV_INITRD`, `INITRAMFS_SOURCE`), `WATCHDOG` with `WATCHDOG_NOWAYOUT` and a driver, `PACKET` (raw sockets for DHCP), Unix sockets (s6), `PANIC_TIMEOUT=-1` with `panic=5` on the command line.

Two facts found by running it:

- **The built-in command line is added in front of the one the firmware passes, not instead of it** (TESTED: `/proc/cmdline` showed both). The last `root=` and `hubos.slot=` win. Stage 0 and the init script therefore take the **last** `hubos.slot=`.
- **Without an initramfs, a kernel with no `/sbin/init` starts `/bin/sh`** (TESTED: the machine sat at a shell prompt, with no reboot). That is why stage 0 exists (section 4).

### 2.2 What Hub OS will need on real hardware (a list; none of it built)

- **GPU:** DRM/KMS and the vendor drivers (AMD `amdgpu`, Intel `i915`/`xe`, NVIDIA's proprietary module or `nouveau`; GPU brands are an open question in `HUB-OS.md`), framebuffer console, firmware blobs from `linux-firmware` (the root image must carry them: licence and size to be checked).
- **Audio:** ALSA with HD-Audio, USB audio; PipeWire needs the usual core options.
- **Input:** `INPUT_EVDEV`, HID, USB HID, USB xHCI; **`INPUT_UINPUT`** for the input forwarder (`/dev/uinput` is absent in this build environment).
- **KVM** for the VM host: `KVM`, `KVM_INTEL`/`KVM_AMD`, `VHOST_NET`, `TUN`, `BRIDGE`, `VFIO`/IOMMU.
- **Network:** drivers for the 10GbE-class cards that are bought (for example `ixgbe`, `i40e`, `mlx5`, `atlantic`, `igc`, `r8169`); bridging for the VM host.
- **Storage:** `BLK_DEV_NVME`, `SATA_AHCI`, USB mass storage; the NAS needs ZFS (out-of-tree module) or Btrfs (in tree), RAID/device-mapper options.
- **Process isolation** that Proton/Steam, Flatpak-like tools and containers use: namespaces, cgroups, seccomp.
- **Real-time and tuning** for the gaming box: `PREEMPT_RT`, `NO_HZ_FULL`, isolcpus (`HUB-OS.md`: "real-time kernel, reserved CPU cores").
- **Hardware watchdog drivers** for real boards (`iTCO_wdt` on Intel, `sp5100_tco` on AMD: BELIEVED) instead of QEMU's `i6300esb`; **TPM** (`TCG_TPM`) if you want a hardware counter later.
- **Modules or not:** the tiny kernel has no module support. The NVIDIA module and ZFS need it. Whether the production kernel is monolithic or modular is a question (section 12).

---

## 3. The init

### 3.1 What is available (TESTED unless marked)

| | Ubuntu 24.04 | Debian trixie / forky / sid | Upstream source |
|---|---|---|---|
| **s6** (supervision) | `s6` 2.12.0.3 and `execline` 2.9.4.0 in **universe** (not in `main`: `debootstrap` needs `--components=main,universe`) | s6 2.13.1.0 / 2.15.1.0 | skarnet.org: skalibs 2.15.1.0, execline 2.9.9.2, s6 2.15.1.0 |
| **s6-rc** (dependencies), **s6-linux-init** | **not in the archive** | **not in the archive** | s6-rc 0.7.0.0, s6-linux-init 1.2.0.2: **built here from source** |
| **dinit** | **not in the archive** (`apt-cache policy dinit`: no candidate) | **not in the archive** | version 0.22.1 (beta) per its web page ([davmac.org/projects/dinit](https://davmac.org/projects/dinit/), SOURCE). Source downloads from GitHub return 403 here, so **dinit was not built or run** |
| **runit** | yes (2.1.2) | yes | - |

The s6 stack from source (TESTED, `/tmp/pb/skarnet/build.sh`, Appendix E): skalibs 13 s, execline 3 s, s6 5 s, s6-rc 2 s, s6-linux-init 1 s: **24 s in all**, static binaries (`file` says statically linked), 148 programs, 116 MB unstripped (BELIEVED a few MB after `strip`; not done).

### 3.2 How a service and its dependencies are written

**s6 (what the prototype uses).** A service is a directory with an executable `run` file; an optional `log/run` makes a log service; a file named `down` means "do not start yet". Example, the real `hubd` service of the prototype:

```
/etc/s6/sv/hubd/run
    #!/bin/sh
    exec 2>&1
    mkdir -p /run/hubos; chown hub:hub /run/hubos; chmod 700 /run/hubos
    exec s6-setuidgid hub /usr/bin/hubd serve --inventory /config/hubos/inventory.toml \
         --viewers /config/hubos/viewers.toml --socket /run/hubos/hubd.sock
/etc/s6/sv/hubd/log/run
    #!/bin/sh
    exec s6-log n5 s200000 /var/log/hubd
```

`s6-supervise` restarts the process when it exits. Dependencies and start order are **not** part of plain s6; the prototype starts what the config lists and lets each service wait for what it needs (the `confirm` service polls `hubd`).

**s6-rc (when ordering is needed).** A source directory per service with a `type` file (`oneshot`, `longrun` or `bundle`), a `dependencies.d/` directory with one empty file per dependency, and `run` or `up`/`down` scripts ([s6-rc-compile](https://skarnet.org/software/s6-rc/s6-rc-compile.html), SOURCE; the format was read, not run here). It compiles to a database that `s6-rc` brings up in dependency order. BELIEVED fit for the later chain `seatd` then `driftwm` then `waybar` and `hubd`.

**dinit.** One file per service, `type = process` or `scripted`, `command = ...`, `depends-on = other`, `waits-for = other`, `restart = true` ([dinit-service man page](https://raw.githubusercontent.com/davmac314/dinit/master/doc/manpages/dinit-service.5.m4), SOURCE: it documents `restart`, `restart-delay`, `restart-limit-count`, `depends-on`, `waits-for`). Not run.

### 3.3 How it behaved as PID 1 in QEMU (TESTED)

The prototype's `/sbin/init` is a 53-line shell script (Appendix B) that mounts `/proc`, `/sys`, `/dev`, and tmpfs for `/run`, `/tmp` and `/var`, mounts the config partition, applies the config, copies the chosen service directories into `/run/service`, and then does `exec s6-svscan /run/service`. So **`s6-svscan` is PID 1** after that point (`.s6-svscan/crash` and `finish` scripts reboot the machine if it ever exits).

- Boot to "handing over to s6-svscan": **9 to 13 s** under emulation (first boot of a fresh disk 11.8 to 13.2 s).
- `kill -9` of `hubd` (pid 104): 2 s later `s6-svstat` showed `up (pid 236) 1 seconds`, i.e. a new process had started within a second, and `hubd list` answered again. Test 3, 2.4 s.
- Logging: `s6-log` writes to `/var/log/hubd/current` (a tmpfs: lost at reboot; see section 6 for a persistent place).
- Console: `getty`-less; a service runs `cttyhack /bin/sh +m` on the serial console (test aid; a real hub has a tty login).

### 3.4 Supervising hubd, Waybar and the future driftwm (BELIEVED; not run)

The `HUB-OS.md` open question is "restart Waybar and hubd when driftwm restarts". With plain s6, give `driftwm` a `finish` script that sends them a restart: `s6-svc -t /run/service/waybar /run/service/hubd`. With s6-rc, make `waybar` and `hubd` depend on `driftwm` and `driftwm` depend on `seatd` (Ubuntu has `seatd` 0.8.0 and `dbus-daemon`, TESTED by `apt-cache policy`), and run `s6-rc -u change` after a driftwm restart. Waybar needs a D-Bus session bus (`HUB-OS.md` allows `dbus-daemon` alone). `hubd` already survives a driftwm restart by itself (it reconnects: `docs/hubd-slice2.md`), so for `hubd` the restart is only needed if you want it re-adopting windows with a clean state.

### 3.5 Recommendation

**s6 now (plain `run` scripts, as tested), s6-rc later** when real ordering is needed (build it from source in the image build; 24 s). It keeps the init swappable: the services are ordinary executables, so a move to dinit or runit means rewriting only the small `run` wrappers. Why not dinit: it may well be nicer to write, but it is not packaged in Ubuntu or Debian and I could not fetch its source here, so I cannot say anything measured about it. **Your decision.**

---

## 4. The disk layout and the A/B scheme

### 4.1 Partitions (the prototype's sizes, and what I would start with on real machines)

All five are GPT partitions found **by their GPT name**, never by number, so the same scripts work on `/dev/vda`, `/dev/nvme0n1` or `/dev/sda`. (TESTED: stage 0 finds them through `/sys/class/block/*/uevent`, `PARTNAME=`; the init script uses `findfs PARTLABEL=...`.)

| # | GPT name | Prototype | Proposed on real hardware (UNVERIFIED) | File system | Mounted | Holds |
|---|---|---|---|---|---|---|
| 1 | `hubos-esp` | 64 MiB | 512 MiB | FAT32 | only while an update copies a kernel | `EFI/BOOT/BOOTX64.EFI` (the fallback copy of the slot-A kernel), `EFI/hubos/kernel-a.efi`, `kernel-b.efi` |
| 2 | `hubos-root-a` | 64 MiB | 4 GiB (the hub), per role | **squashfs** (zstd), written raw | read-only, as the root | the system |
| 3 | `hubos-root-b` | 64 MiB | same as A | squashfs | when it is the booted slot | the system |
| 4 | `hubos-config` | 16 MiB | 256 MiB | ext4 | read-write at `/config` | `hubos/node.conf`, `inventory.toml`, `viewers.toml`, `state/min_version`; later `secrets/` |
| 5 | `hubos-data` | 160 MiB | the rest | ext4 | read-write at `/data` | update staging (`/data/update`), later logs and local data |

The slot partition is larger than the image; the extra bytes are ignored by squashfs. **squashfs, not erofs:** erofs is also in the archive (`mkfs.erofs` 1.7.1) and in the kernel, and BELIEVED to work the same, but only squashfs was run, so squashfs is the proposal. An 'overlay' on top of the root was not needed: `/run`, `/tmp` and `/var` are tmpfs, and the little that must persist lives on the config and data partitions. The disk image was built **without loop devices**: each partition as its own file (`mkfs.vfat`, `mkfs.ext4 -d DIR`, `mksquashfs`), a partition table with `sfdisk`, and `dd` of each file at its offset (Appendix C).

### 4.2 How the image is written to a slot (TESTED)

`hubos-ctl update BASE` (Appendix B) does, in this order, and stops at the first problem:

1. fetch `manifest` and `manifest.sig` from `BASE` (HTTP with busybox `wget`, or a directory);
2. verify the signature, then the version rules (section 5);
3. fetch `kernel.efi` and `rootfs.sqsh` into `/data/update` and check their sha256 against the **signed** manifest;
4. `dd` the root image onto the **other** slot's partition (`conv=fsync`) and check by reading the same number of bytes back and hashing them;
5. mount the boot partition, copy the kernel as `kernel-X.efi.new`, `sync`, rename it over `kernel-X.efi`, unmount;
6. make sure the boot entries exist; set `BootNext` to the other slot's entry.

The running slot is never written. It takes **7.6 to 8.0 s** under emulation for a 39 MB bundle over QEMU's network from the host.

### 4.3 How the boot picks a slot and falls back

Three firmware entries (created on first boot with `efibootmgr`, TESTED): `hubos-a`, `hubos-b`, `hubos-recovery`. Each points at a kernel file on the boot partition and carries its own kernel command line as the entry's load options (UCS-2 `-u`), for example `console=ttyS0 ro rootfstype=squashfs panic=5 loglevel=4 root=PARTLABEL=hubos-root-b hubos.slot=b`.

- `BootOrder` = the **confirmed** slot first, then the other. A normal boot follows it.
- An update sets `BootNext` = the new slot's entry. UEFI deletes `BootNext` before it boots it (BELIEVED from the specification; observed: the status line after each trial boot showed no `BootNext`), so it is a **one-time** trial: if that boot never ends in "confirmed", the next boot follows `BootOrder` and lands in the old slot. No counter is needed.
- "Boot succeeded" is a mark that **only a healthy new system can set**: the `confirm` service waits until `hubd` answers its socket five times in a row, one second apart, then runs `hubos-ctl confirm`, which sets `BootOrder` = (this slot, the other) and raises the version floor (section 5), and then starts the steady watchdog feeder. If `hubd` is not healthy within `CONFIRM_TIMEOUT` seconds (40 in the tests), it reboots (`reboot -f`) and the firmware falls back.
- Failures that never reach `confirm` are caught by **stage 0** (section 4.4) and the **watchdog**.

### 4.4 Stage 0, and the watchdog

Stage 0 is a 32-line shell script plus busybox, compiled into the kernel as its initramfs (Appendix A; the cpio list must contain `/dev/console`, otherwise the kernel prints `unable to open an initial console` and reboots in a loop: TESTED). It: mounts `/proc`, `/sys`, `/dev`; opens `/dev/watchdog` and closes it again (with `NOWAYOUT` the countdown keeps running; the heartbeat is set on the built-in command line: `i6300esb.heartbeat=60`); finds the slot's partition by GPT name (retrying up to 20 s, because the disk shows up a moment after the kernel starts: TESTED, the first version failed on this); mounts it; checks that `/sbin/init` exists; and runs `switch_root`. If the mount or the check fails it prints a line and runs `reboot -f`.

The steady feeder is a `wd` service (`watchdog -F -t 5 -T 60 /dev/watchdog`) that the `confirm` step starts. If PID 1 is alive but the machine is stuck, nothing feeds the watchdog and the hardware resets it.

### 4.5 Comparison with GRUB and its environment block

| | Firmware variables + EFI-stub kernel (**tried**) | GRUB with `grubenv` |
|---|---|---|
| Extra code on the boot partition | none | `grubx64.efi`: **TESTED** that `grub-mkimage` builds one (676 KB, modules `part_gpt fat ext2 normal linux search loadenv configfile`) from the Ubuntu packages; **not booted** |
| Where the slot state lives | the firmware's NVRAM (`BootNext`, `BootOrder`) | a 1024-byte `grubenv` file on the boot partition. **TESTED**: `grub-editenv grubenv set next=b tries=1` and `list` worked |
| One-time trial and a counter | built in (`BootNext` is single use) | written by hand in `grub.cfg` (decrement `tries` before booting the trial; reset it from Linux with `grub-editenv`) - BELIEVED standard, **not run** |
| BIOS machines | no | yes (a separate `grub-pc` image; **not tried**) |
| Depends on | a firmware that stores variables and honours `BootNext` and `BootOrder` - **UNKNOWN on the real boards** (BELIEVED fine on most) | a bootloader that we must also update, sign and secure |
| Loses its state when | the NVRAM is cleared (CMOS reset). Then the fallback file `BOOTX64.EFI` boots slot A and `ensure-entries` recreates the entries (**TESTED**: the very first boot uses that fallback). Risk: the fallback boots slot A even if B was the confirmed one - see risks | the `grubenv` file is damaged |
| What I would pick | **this**, for the first slice | keep as the plan B if a real board misbehaves |

**systemd-boot and anything from systemd are excluded** by the rules (it is in the archive; not used).

### 4.6 BIOS against UEFI

**Assumption:** the real machines are UEFI. Modern desktop and server boards are, and some no longer offer BIOS mode (BELIEVED). The prototype is UEFI only (OVMF 2024.02, `OVMF_CODE_4M.fd` plus a writable copy of `OVMF_VARS_4M.fd`, machine type `q35`). A BIOS boot under QEMU (SeaBIOS is available) was **not tried**; it would need GRUB. Secure Boot was not used (`OVMF_CODE_4M.secboot.fd` exists; the kernel is unsigned); see the risks.

### 4.7 What was run

See section 8 for the whole test list. The "deliberately failing boot" cases are tests 6, 6b, 7 and 7b. In each, a **correctly signed** bundle with a bad image was installed, the machine rebooted, and the old slot came back by itself.

---

## 5. Signing

### 5.1 What is signed

**A manifest, not the image.** The manifest is a small text file (Appendix D prints the script that makes it):

```
hubos-bundle 1
version 2
flavor good
arch x86_64
kernel.efi sha256 df228c7ba552760fd08d942ea86470b5c9daa1e4518f094c6096030c4811f2e0 size 3007488
rootfs.sqsh sha256 2e4f771d3e9cc9c6fe1ecc64de9dcf2cb7e7b5f1046a0712893865af442ae553 size 35930112
```

The signature covers the manifest; the manifest covers the files by hash. Reason: BELIEVED that OpenSSL's raw ed25519 signing reads the whole message into memory (so it is awkward for 4 GB images); signing 240 bytes is instant and the hash check streams.

### 5.2 Tools compared (TESTED, all three on the same manifest)

| Tool | Archive | Sign and verify | Tampered manifest | Notes |
|---|---|---|---|---|
| `signify-openbsd` 31 | yes (universe) | `signify-openbsd -S -s k.sec -m manifest -x manifest.sig`; `-V -p k.pub ...` prints `Signature Verified`, exit 0 | `signify-openbsd: signature verification failed`, exit 1 | public key file 102 bytes, signature 138 bytes; needs `libbsd` and `libmd`; **recommended** (smallest, simplest, ed25519) |
| `minisign` 0.11 | yes | `Signature and comment signature verified` | `Signature verification failed`, exit 1 | also signs a "trusted comment" (could carry the version); needs libsodium |
| `openssl` ed25519 | yes (already present) | `Signature Verified Successfully` | `Signature Verification Failure`, exit 1 | the tool of `docs/environment.md` 1.12; larger; fine for a small manifest |

A tiny Go verifier would also be easy (the standard library has `crypto/ed25519`), but "a proven part does the job equally well" (`HUB-OS.md`), so it is not proposed.

### 5.3 The rules the update tool enforces (TESTED, test 4 and 5)

| Case | Result seen |
|---|---|
| bundle without `manifest.sig` | `REFUSED: no signature (unsigned bundle)`, exit 2 |
| manifest changed after signing (`version 2` became `version 9`) | `REFUSED: bad signature (manifest not signed by the owner's key, or changed)` |
| manifest signed with another key | the same message |
| `rootfs.sqsh` with one changed byte | `REFUSED: rootfs.sqsh does not match the signed hash (tampered or damaged)` (before anything is written) |
| signed bundle of version 1 while version 1 runs | `REFUSED: version 1 is not newer than the running version 1` |
| signed bundle of version 1 while version 2 runs (a replay) | `REFUSED: version 1 is not newer than the running version 2` |
| good bundle of version 2 | accepted; after the new slot is confirmed, `min_version=2` is stored |

In all refused cases `BootNext` stayed unset and the other slot was not touched.

### 5.4 Rollback protection

Two numbers: the running image's `version` (in `/etc/hubos-release`) and the **floor** (`/config/hubos/state/min_version`, raised to the running version when a boot is confirmed). A bundle must be **strictly newer than the running version** and **not below the floor**. **Limit (stated, not hidden):** the floor sits on a writable partition; someone with disk access can lower it. A TPM monotonic counter would be the hardware answer (BELIEVED; not tried). Another gap: nothing stops the **old slot** from booting; the init of a slot does not check its own version against the floor (not implemented, and listed in the plan).

### 5.5 Where the public key lives, and the first install

The public key (`k.pub`) is a file **inside the image** (`/etc/hubos/update.pub`), so an update can only be accepted if the *running* system trusts the key. The **first install** gets it from the installer image (a later slice: network boot). Key rotation: ship a new image, signed with the old key, that contains the new public key; the next update is then signed with the new key. **Nothing in this slice protects the first install against a tampered installer**: Secure Boot is the usual answer; not attempted.

### 5.6 Where the private key is kept (an input for the secrets design, not a decision)

The private key must **never** be on a cluster node and never in git. `HUB-OS.md` says builds will one day happen on a machine inside the cluster; that machine must not hold the signing key in the clear. Options for you to weigh later: an offline key on your own computer (sign by hand each release), a hardware token, or a separate small "signer" step that takes a manifest and returns a signature, with the key protected by a passphrase kept apart. The prototype key was created with `signify-openbsd -G -n` (no passphrase) in a temporary directory and deleted.

---

## 6. The declarative config

### 6.1 What it contains and where it lives

All of it is on the **config partition** (`hubos-config`, mounted at `/config`), outside the image, as `HUB-OS.md` requires. Slice 1 files, as run:

```
/config/hubos/node.conf        # sh-readable key=value, read by /sbin/init
/config/hubos/inventory.toml   # the hub's inventory (examples/inventory.example.toml was used)
/config/hubos/viewers.toml     # (examples/viewers.example.toml was used)
/config/hubos/state/min_version  # written by the update tool, not by hand
```

Example `node.conf` (the one the tests used):

```
NAME=hub-qemu              # host name
NET=dhcp                   # dhcp or static
IFACE=eth0
SERVICES="hubd console confirm wd"   # which service directories to start
CONFIRM_TIMEOUT=40         # seconds before an unhealthy trial boot gives up
# static would also set:  ADDR=192.0.2.10/24  GW=192.0.2.1
```

Proposed later fields (not built): `ROLE`, `UPDATE_URL`, a tuning profile name (the files live in `/config/hubos/tuning/`), `HUBD_ARGS`, authorized SSH keys. **Format:** a shell-readable file is the smallest thing that works; `node.toml` (read by a small Go helper, using the TOML library `hubd` already uses) is the likely answer once there are lists and tables. A question for you.

### 6.2 How it is applied at boot (TESTED)

`/sbin/init` mounts the config partition, sources `node.conf`, sets the host name, brings up the network, copies the listed service directories into `/run/service`, and starts `s6-svscan`. To change something: edit the file and reboot (an `hubos-ctl apply` for services is possible later). Booting a different slot does not touch it (TESTED: a marker file written before the update was still there in the new slot).

### 6.3 Backup to the NAS (design; not built, not tested)

`HUB-OS.md`: the inventory, `viewers.toml` and `wofi.css` get a NAS backup copy; **secrets do not**. Proposal: one more s6 service that, on a schedule, copies `/config/hubos` (without `secrets/`) to the NAS over ssh (`rsync` or `tar` through `ssh`). Restore = write the files back to a fresh config partition. The tool, the schedule and the NAS side are not designed.

### 6.4 Which fields are secrets, and where they would go (inputs only)

SSH host private keys; any password or token; Sunshine's web-UI login and its pairing state; Moonlight's client certificate and key and the paired hosts' certificates (`docs/viewers-research.md`); the update **private** key (never on a node). Proposed home: a `secrets/` directory on the config partition, mode 0700, owned by the user that needs it, **excluded from the NAS backup**, never in the inventory and never in git. Everything about how they get there (first install, rotation, who can read them) is the separate secrets design.

---

## 7. Network

- **First slice:** DHCP **or** static, chosen by `NET` in `node.conf`. **TESTED:** DHCP with busybox `udhcpc` against QEMU's built-in DHCP server (lease `10.0.2.15/24`, gateway `10.0.2.2`, DNS written to `/run/resolv.conf`). **Not run:** the static branch (the code is in Appendix B).
- **Tools:** busybox `ip` and `udhcpc` (already in the image as `busybox-static`); a 7-line `udhcpc` script. No NetworkManager, no systemd-networkd.
- **Kernel:** `CONFIG_PACKET=y` is required (without it `udhcpc` fails with `socket: Address family not supported by protocol`: TESTED).
- **Name:** `hostname "$NAME"` from the config. No mDNS.
- **QEMU:** user-mode networking, `-netdev user,id=n0 -device virtio-net-pci,netdev=n0,romfile=` (the empty `romfile=` is needed because the iPXE ROM is not in the unpacked tree). The guest reaches the host at `10.0.2.2`; the tests serve bundles from a Python `http.server` on the host and fetch them at `http://10.0.2.2:8000/...`. A **tap** network (so another machine could talk to the guest) was **not tried**; it needs `/dev/net/tun` and a bridge (BELIEVED possible here as root).
- **Later:** network boot is separate (`docs/environment.md` 1.11: QEMU's built-in TFTP plus the iPXE ROM worked).

---

## 8. The first slice's tests

Tests 1 to 8 are run by `slice1_tests.py` and test 9 by `t_pl.py` (Appendix E) against one disk image, in order, under QEMU TCG (2 vCPUs, 1 GB, OVMF). Times are wall-clock under emulation, with other jobs running on the same machine part of the time, so treat them as rough.

| # | Rule it proves | How it is run | Result | Time |
|---|---|---|---|---|
| 1 | Immutable root | in the booted guest: `touch /usr/x`, `touch /etc/x`, write to `/var`, show the root mount | `Read-only file system` for `/usr` and `/etc`; `/var` (tmpfs) writable; root is `squashfs` | 0.4 s |
| 2 | First boot sets itself up; **hubd runs from the config partition** | boot a fresh disk; run `hubd check --inventory /config/hubos/inventory.toml` and `hubd list --socket /run/hubos/hubd.sock` | boot entries created; hubd answers (`Down machines`, `Hub`, ... groups from the config partition's inventory) | first boot to handover 13.2 s; test 9.2 s |
| 3 | **Init restarts a killed service** | `s6-svstat`, `kill -9` the pid, wait 2 s, `s6-svstat`, `hubd list` | pid 104 replaced by pid 236 (`up ... 1 seconds`); hubd answers | 2.4 s |
| 4 | **Unsigned, tampered, wrong key, replay refused** | `hubos-ctl update` for five bad bundles | all five `REFUSED` with the messages in 5.3; `BootNext` unset | 5.4 s |
| 5 | **Signed update accepted, config survives** | update to v2, reboot, check status and a marker file; try the v1 bundle again | booted slot B (`slot=b release=2`), `BootOrder` 0008 first, `min_version=2`, marker `survive-me` still there; the old bundle `REFUSED` | update 7.6 s, reboot to handover 25 s, test 40.4 s |
| 6 | **Bad boot rolls back (a signed bundle without `/sbin/init`)** | update to v3, reboot | stage 0: `slot a has no /sbin/init; rebooting`; machine back in slot B | rollback 42.2 s |
| 6b | **Bad boot rolls back (a signed bundle whose root is garbage)** | update to v4 (2 MB of zeros as the root) | stage 0: `cannot mount the root of slot a; rebooting`; back in B | 45.6 s |
| 7 | **Bad boot rolls back (boots, never healthy)** | update to v5 whose `hubd` never answers | `confirm: hubd did not become healthy in 40s; this boot FAILED; rebooting into the other slot`; back in B | 70.5 s |
| 7b | **Bad boot rolls back (init hangs)** | update to v6 whose init is `sleep` | no confirm, no feeder: the watchdog (60 s) resets; back in B | 90.7 s |
| 8 | **Hub recovery mode is a boot option** | `efibootmgr -n` the recovery entry, reboot | banner `RECOVERY MODE (bare terminal; root is read-only; nothing else started)`, `hubos.recovery=1` on the command line, read-only root | 14.7 s |
| 9 | An update **interrupted by power loss** leaves the confirmed slot booting | start an update, `kill -9` QEMU 4 s later, start again | booted `slot=b release=2` with the old order. **Weak test:** I do not know whether the `dd` had started by then | not timed |

**What cannot be tested here:** real firmware (does it honour `BootNext`? clear entries on CMOS reset?); a real hardware watchdog; Secure Boot; power loss at every point of a write (one kill only); flash wear; TPM; disk and network speed; any GPU, audio, input.

**Not yet tested although in the brief for the first slice:** the refuse-update-while-busy hook and the other roles (both "later" in your list); the NAS backup; the static network branch; a tap network.

---

## 9. Size and time budgets

| Item | Measured (TESTED, emulation, 4 CPUs) | Note |
|---|---|---|
| Kernel, first full build | about 3 min (169 s + 15 s) | 6 to 15 s for a small change; `defconfig` was 441 s |
| Root file system (`mmdebstrap`) | 25 s (directory 83 MB); `debootstrap` 60 s | plus 3.4 s for `mksquashfs -comp zstd` |
| Bundle (kernel + root + manifest + signature), from the finished tree | 5 to 6 s per bundle | the script `mkbundle.sh` |
| **Full rebuild from nothing** (download of the kernel source and the packages not counted) | **about 4 to 5 minutes** | BELIEVED to grow to tens of minutes once the real desktop software is in |
| Image sizes | kernel **4.1 MB**; root **35.9 MB** squashfs (zstd; 31.6 MB with xz); bundle **40 MB**; disk image 420 MiB sparse (309 MB touched by the tests) | a hub image with driftwm, Waybar, wofi, PipeWire, Mesa, Qt, virt-viewer will be much bigger: UNKNOWN, BELIEVED 1 to 3 GB |
| Boot under emulation | firmware + kernel + stage 0 + init to "handing over to s6-svscan": **9 to 13 s** on a fresh start; **15 to 25 s** after a reboot (the firmware runs twice in QEMU) | says nothing about hardware |
| Update | 7.6 to 8.0 s for 39 MB | local HTTP |
| Rollback | stage 0 refusal 42 to 46 s; unhealthy trial 70 s; hang 91 s | each includes the update's own reboot; the watchdog heartbeat (60 s) and `CONFIRM_TIMEOUT` (40 s) are the knobs |
| Disk used by the whole exercise in `/tmp` | about 4 GB for the prototype (kernel tree 1.8 GB; three test disks 0.9 GB; bundles 0.23 GB; unpacked packages) plus **7.4 GB** for the Buildroot tree | the prototype needs about 5 GB; Buildroot alone needs more than that; everything was deleted afterwards |

---

## 10. Risks, and what this does not cover

**Nothing here is verified on real hardware.** Beyond that:

1. **Real firmware may not behave.** The scheme depends on `BootNext` and `BootOrder`. Some boards reset or reorder them, drop entries on a CMOS reset, or refuse unsigned loaders with Secure Boot on. UNKNOWN until the December hardware. Plan B is GRUB with `grubenv` (section 4.5).
2. **The fallback path can boot the wrong slot.** If the NVRAM is cleared, the firmware boots `EFI/BOOT/BOOTX64.EFI`, which is slot A's kernel with a built-in `root=...-a`. If B was the confirmed slot, an older image comes up. The slot's init does not yet check its own version against the floor, and the update tool does not yet keep `BOOTX64.EFI` equal to the confirmed kernel. Both are in the plan.
3. **No verified boot.** The signature is checked when an update is installed, not at every boot. Someone who can write the disk can change a slot. `dm-verity` with a signed root hash and Secure Boot are the usual answers (BELIEVED; not tried).
4. **The version floor sits on a writable partition** (5.4).
5. **The trust root is inside the image**, and the first install trusts its installer (5.5). No answer yet to a "freeze" attack (a server that keeps offering the old signed bundle, so a node never updates). Bundles go over **plain HTTP**: integrity comes from the signature, there is no confidentiality.
6. **Watchdog differences.** QEMU's `i6300esb` is not what real boards have. A board with no usable watchdog loses the "hang" rollback (stage 0 prints `no watchdog device`). A 60 s heartbeat may be too short or too long for a real boot (disk spin-up, GPU driver, network wait): to be tuned per machine. The timer starts in stage 0 and is fed only after `confirm`.
7. **The one unexplained hang.** One start of the test machine in about twelve stopped after the line `BdsDxe: starting Boot0001` with no kernel output for several minutes, and then ran fine on the next start with the same files. Cause UNKNOWN. It could be QEMU, OVMF or the kernel; it has to be watched for.
8. **Shell scripts as the base layer.** `/sbin/init` (53 lines) and `hubos-ctl` (96 lines) are POSIX shell with thin error handling. They worked, and they are easy to read, but a typo in them can brick a slot; a small Go program (like `hubd`) with tests may be the better home later. A question for you.
9. **The kernel is ours to patch.** A tiny custom kernel means we follow kernel security releases ourselves (the distribution would otherwise do it). 6.12 is a long-term series (BELIEVED). A distribution kernel would be big and need modules.
10. **Reproducibility is shown only for the root tar** (two `mmdebstrap` runs gave the same bytes). The kernel and the squashfs/disk assembly were not compared; the kernel needs its build timestamp and user fixed (UNKNOWN).
11. **The update was interrupted only once** (kill of QEMU at 4 s), and it is not known whether the write had started.
12. **A failed version is not remembered.** After a rollback the node would try the same bad version again if the hub offered it. A "failed" mark is not designed.
13. **Not covered:** the "refuse to update or restart while a game or long job is running" hook; role-specific images (gaming, AI, NAS, VM host); the whole desktop (driftwm, Waybar, Moonlight, PipeWire, Mesa); GPU, audio, input; the NAS backup job; scheduled or owner-triggered updates from the hub (`HUB-OS.md`: "updates come from the hub over the network, only when the owner chooses"); persistent logs; Secure Boot; TPM; network boot of the first install (tested earlier on its own, not joined to this).
14. **Ubuntu 24.04 as the base:** standard support ends in 2029 (BELIEVED, not checked); a move to the next release is a project of its own.
15. **`libsystemd0` and `libudev1`** are in the image (section 1.3). Your decision.

---

## 11. A step-by-step plan

Each slice is small and has its own test. Time estimates are guesses for one person working with a bot; all tests run under QEMU in this build environment.

| Slice | What | Test that proves it | Estimate |
|---|---|---|---|
| **S0** The smallest thing that boots | Kernel (tiny config + stage 0) and a squashfs root holding only busybox, started with `qemu -kernel` (no firmware, no disk table) | boots to a shell; `touch /x` fails with `Read-only file system` | 0.5 day |
| **S1** Disk and config | UEFI disk image (GPT: boot, slot A, config, data), the EFI-stub kernel, `/sbin/init`, s6, config partition, `hubd` as a service | tests 1, 2, 3 | 1 day |
| **S2** Two slots and signed updates | slot B, `hubos-ctl update`, the manifest, `signify`, the version rules | tests 4, 5 | 1 day |
| **S3** Rollback | stage 0 checks, the `confirm` step, the watchdog feeder, the recovery entry | tests 6, 6b, 7, 7b, 8 | 1 day |
| **S4** Put it in the repo | `tools/image/` with the package list, the kernel fragment, the assembly scripts and a test runner (shell or Go: your choice); a CI-sized `make image` that finishes in a few minutes | the test list passes from a clean checkout | 1 to 2 days |
| **S5** Harden | the slot's init checks its version against the floor; the update tool keeps `BOOTX64.EFI` in step; a "failed version" mark; kill-at-every-step power-loss test; reproducible kernel build | new tests for each | 2 days |
| **S6** Key custody and installer | the signing step with the key off the nodes; first install by network boot (iPXE, tested separately) ending in a signed image | install from nothing in QEMU | 2 days |
| **S7** Hooks and roles | the refuse-while-busy hook; one more role image (for example a NAS or guest image) | a test that holds a fake "busy" flag | 2 days |
| **S8** The desktop | driftwm, Waybar, wofi, PipeWire, seatd, dbus in the image (s6-rc by then) | needs a virtual GPU; not available here (UNKNOWN) | open |
| **S9** Real hardware | Phase C | December 2026 | - |

---

## 12. Every choice I made, as a question for you

Each has my recommendation and the alternative.

1. **Base builder.** *Recommend:* `mmdebstrap` on Ubuntu 24.04 with a pinned snapshot date (25 s; the desktop software we need later is in the archive). *Alternative:* Buildroot (a minimal root is 2.9 MB instead of 34 MB, but a build takes 28 minutes and 7.4 GB, and it has no package for 7 programs we need) or hand-built.
2. **The two systemd libraries.** *Recommend:* accept `libsystemd0` and `libudev1` as libraries in the base. *Alternative:* delete `apt`, `procps` and PAM from the image so that neither is installed.
3. **Ubuntu or Debian.** *Recommend:* Ubuntu 24.04 (already used for the other documents). *Alternative:* Debian trixie (also has s6, no dinit, no s6-rc).
4. **Init.** *Recommend:* s6 with plain `run` scripts now, s6-rc built from source later. *Alternative:* dinit (not packaged anywhere I could check; not tested) or runit.
5. **Where PID 1 comes from.** *Recommend:* a small `/sbin/init` shell script that ends in `exec s6-svscan` (as tested). *Alternative:* `s6-linux-init` (built from source here, not run).
6. **Boot method.** *Recommend:* UEFI only, EFI-stub kernel, firmware `BootNext`/`BootOrder`, no bootloader. *Alternative:* GRUB with `grubenv`.
7. **BIOS support.** *Recommend:* none. *Alternative:* add GRUB-pc later if a machine needs it.
8. **Stage 0 inside the kernel.** *Recommend:* yes (busybox + a 32-line script). *Alternative:* no stage 0 and `root=PARTLABEL=` on the command line, which is simpler but has the "kernel starts `/bin/sh`" hang and no watchdog arming.
9. **Root file system format.** *Recommend:* squashfs with zstd (tested). *Alternative:* erofs (not tested).
10. **Partition plan** (5 partitions, sizes in 4.1). *Alternative:* add a separate `/var/log` partition, or shrink the boot partition.
11. **Signing tool.** *Recommend:* `signify-openbsd`. *Alternative:* minisign or openssl (both tested), or a Go verifier of our own.
12. **What is signed.** *Recommend:* a manifest of hashes with a version. *Alternative:* sign the whole image (needs a streaming signature tool).
13. **Rollback protection.** *Recommend:* strictly increasing version plus a floor on the config partition. *Alternative:* a TPM counter (later) or no floor.
14. **Config format.** *Recommend:* `node.conf` (shell) now, `node.toml` later. *Alternative:* TOML from the start.
15. **Update transport.** *Recommend:* HTTP pull from the hub, started by the owner. *Alternative:* push over ssh, or a USB stick.
16. **Health definition for "boot succeeded".** *Recommend:* `hubd` answers its socket five times in a row. *Alternative:* a role-specific check list (for example "Sunshine accepts connections" on the gaming box).
17. **Timeouts.** *Recommend:* confirm timeout 120 s and watchdog heartbeat 120 s on real hardware (the tests used 40 s and 60 s). *Alternative:* tune per machine.
18. **Recovery.** *Recommend:* a firmware boot entry (reachable from the firmware's boot menu). *Alternative:* a GRUB menu item.
19. **Kernel.** *Recommend:* Linux 6.12 long-term, `tinyconfig` plus a fragment kept in the repo, no modules in slice 1. *Alternative:* a distribution kernel with modules (bigger, easier for GPUs).
20. **Language for the base layer.** *Recommend:* shell for slices 0 to 3, then decide. *Alternative:* write `hubos-ctl` in Go from the start (tested like `hubd`).
21. **Test runner.** *Recommend:* a Go test (like the rest of the repo) that drives QEMU over a pipe. *Alternative:* keep the Python harness as a tool (Python is on the build machine).
22. **Where the code goes.** The prototype is **not** in the repo. *Recommend:* `tools/image/` for the build and test scripts, `image/` for the files that go into the root (init, service scripts), and `docs/` for this file once approved. *Alternative:* a separate repository.
23. **Persistent logs.** *Recommend:* `/var/log` on the data partition. *Alternative:* tmpfs only plus a log shipper to the hub.
24. **Key custody** (5.6). *Recommend:* an offline key on your computer for now. *Alternative:* a hardware token or a signer service later.
25. **A Buildroot image for tiny roles.** A minimal Buildroot root is 2.9 MB. *Recommend:* look at it again only for a later tiny role (for example a pure NAS or a guest); not for the hub. *Alternative:* I boot the Buildroot image I built and add the update tool to it, as a second prototype.

---

## 13. What I tried (exact commands)

Working directory `/tmp/pb` (deleted at the end). `$A=/tmp/pb`. Times are what the commands printed.

### 13.1 Unpacking the tools (nothing installed)

```
A=/tmp/pb; mkdir -p $A/apt/lists/partial $A/apt/cache/archives/partial $A/debs $A/root
O="-o Dir::State::lists=$A/apt/lists -o Dir::Cache=$A/apt/cache -o APT::Sandbox::User=root"
apt-get $O update
apt-get $O install --print-uris -y --no-install-recommends \
  qemu-system-x86 seabios qemu-system-data ovmf ipxe-qemu efibootmgr efivar busybox-static squashfs-tools \
  s6 execline dosfstools mtools fdisk erofs-utils signify-openbsd minisign flex cpio libelf-dev libelf1t64 \
  debootstrap mmdebstrap grub-efi-amd64-bin grub-common libssl-dev bc libzstd-dev xz-utils \
  | grep -oE "^'[^']+'" | tr -d "'" > $A/uris.txt           # 66 packages
cd $A/debs && while read u; do curl -sS -O "$u"; done < $A/uris.txt
for d in $A/debs/*.deb; do dpkg -x $d $A/root; done
# env.sh
export LD_LIBRARY_PATH=$A/root/usr/lib/x86_64-linux-gnu:$A/root/lib/x86_64-linux-gnu:$A/root/usr/lib
export QEMU_MODULE_DIR=$A/root/usr/lib/x86_64-linux-gnu/qemu
export PATH=$A/root/usr/bin:$A/root/usr/sbin:$A/root/sbin:$A/root/bin:$PATH
QEMU="$A/root/usr/bin/qemu-system-x86_64 -L $A/root/usr/share/qemu -L $A/root/usr/share/seabios"
$QEMU --version        # QEMU emulator version 8.2.2 (Debian 1:8.2.2+ds-0ubuntu1.18)
```

### 13.2 Package availability

```
apt-cache $O policy s6 s6-rc s6-linux-init execline skalibs dinit runit busybox-static debootstrap mmdebstrap \
  squashfs-tools erofs-utils ovmf efibootmgr grub-efi-amd64-bin signify-openbsd minisign seatd dbus-daemon
#   s6 2.12.0.3-1build1, execline 2.9.4.0-1build2, runit 2.1.2-59ubuntu1; s6-rc, s6-linux-init, skalibs, dinit: no candidate
for r in trixie forky sid; do curl -sS -L https://deb.debian.org/debian/dists/$r/main/binary-amd64/Packages.xz -o pk-$r.xz; \
  xz -dc pk-$r.xz | awk '/^Package: (dinit|s6|s6-rc|s6-linux-init|runit|seatd|...)$/{p=$2} /^Version:/{if(p){print p"="$2;p=""}}'; done
#   trixie: s6=2.13.1.0-1 runit=2.2.0-3 ...   forky/sid: s6=2.15.1.0-1 ...   dinit, s6-rc, s6-linux-init: absent in all three
```

### 13.3 The root file system, way (a)

```
export DEBOOTSTRAP_DIR=$A/root/usr/share/debootstrap
time debootstrap --variant=minbase --include=s6,execline,busybox-static,libc6 \
  --exclude=systemd,systemd-sysv,libsystemd0,systemd-dev,udev --components=main,universe \
  noble /tmp/pb/rootfs-a http://archive.ubuntu.com/ubuntu            # exit 0, 60 s
#   (first try with --components=main: "E: Couldn't find these debs: s6 execline" - they are in universe)
du -sh rootfs-a                                                     # 206M; after removing var/lib/apt/lists and var/cache/apt: 101M
mksquashfs rootfs-a a-zstd.sqsh -comp zstd -quiet -noappend         # 3.4 s -> 34,246,656 bytes
mksquashfs rootfs-a a-xz.sqsh   -comp xz   -quiet -noappend         # 6.2 s -> 31,580,160 bytes
export PERL5LIB=$A/root/usr/share/perl5:$A/root/usr/lib/x86_64-linux-gnu/perl5/5.38:$A/root/usr/lib/x86_64-linux-gnu/perl5
mmdebstrap --mode=root --variant=minbase --include=s6,execline,busybox-static --components=main,universe \
  noble /tmp/pb/mm-rootfs http://archive.ubuntu.com/ubuntu           # success in 25.1 s, 83M
for i in 1 2; do SOURCE_DATE_EPOCH=1790000000 mmdebstrap --mode=root --variant=minbase \
  --include=s6,execline,busybox-static --components=main,universe noble /tmp/pb/mm$i.tar http://archive.ubuntu.com/ubuntu; \
  sha256sum mm$i.tar; done                                            # both 7f3ff52e09ad29cdea6b333b..., 77,790,720 bytes
curl -sS -o /dev/null -L -w "%{http_code}\n" https://snapshot.ubuntu.com/ubuntu/20261001T000000Z/dists/noble/Release   # 200
# Debian trixie: debootstrap --no-check-gpg ... --include=dinit,s6,s6-rc,s6-linux-init trixie ...
#   "E: Couldn't find these debs: dinit s6-linux-init s6-rc"
```

### 13.4 Buildroot, way (b) (finished: 27.9 minutes)

```
curl -sS -o buildroot.tar.xz https://buildroot.org/downloads/buildroot-2025.02.7.tar.xz     # 5.8 MB
# configs/hubos_defconfig: BR2_x86_64=y BR2_TOOLCHAIN_BUILDROOT_GLIBC=y BR2_INIT_NONE=y BR2_PACKAGE_SKALIBS=y
#   BR2_PACKAGE_EXECLINE=y BR2_PACKAGE_S6=y BR2_PACKAGE_S6_RC=y BR2_PACKAGE_S6_LINUX_INIT=y BR2_TARGET_ROOTFS_EXT2=y
make hubos_defconfig && make -j4        # 1st: "You must install 'rsync' on your build machine" (9 s)
# unpack rsync, libpopt; rerun: a squashfs-zstd configuration stopped at host-lz4 (GitHub 403); with ext2 it went on,
#   one 404 on the first mirror, then the fallback mirror worked; ~100 steps (host tools) in about 5 minutes
# build finished: "exit 0 seconds 1671"; output/images/rootfs.ext2 62,914,560 bytes; du -sh output/target = 9.5M;
#   mksquashfs output/target br-rootfs.sqsh -comp zstd  ->  2,912,256 bytes; glibc "GNU C Library (Buildroot) stable release version 2.41"; du -sh br = 7.4G
for p in pipewire wireplumber mesa3d wlroots sway weston foot waybar wofi qt6 libinput seatd sdl2 ffmpeg efibootmgr \
  s6 s6-rc s6-linux-init dinit runit moonlight sunshine driftwm virt-viewer remmina openssh tmux go rust; do ls -d package/$p; done
#   no package directory for: waybar wofi dinit runit moonlight sunshine driftwm virt-viewer remmina
```

### 13.5 The s6 stack from source, way (c)

```
for p in skalibs execline s6 s6-rc s6-linux-init; do curl -sS -O https://skarnet.org/software/$p/$p-<version>.tar.gz; done
./configure --prefix=/tmp/pb/skout --disable-shared --enable-static --enable-static-libc \
  --with-sysdeps=... --with-include=... --with-lib=...  &&  make -j4 && make install      # per package, see Appendix E
#   skalibs 13s  execline 3s  s6 5s  s6-rc 2s  s6-linux-init 1s   total 24s; 148 programs, statically linked
```

### 13.6 The kernel

```
curl -sS -o linux-6.12.tar.xz https://cdn.kernel.org/pub/linux/kernel/v6.x/linux-6.12.tar.xz && tar -xf linux-6.12.tar.xz
cd linux-6.12 && make tinyconfig HOSTCFLAGS="-I$A/root/usr/include" HOSTLDFLAGS="-L$A/root/usr/lib/x86_64-linux-gnu"
cat hubos-kernel.frag >> .config && make olddefconfig ...
time make -j4 bzImage ...    # 169 s, then "zstd: not found" (host tool missing) -> set KERNEL_GZIP -> 15 s more: bzImage 2,991,104 bytes
# + CONFIG_PACKET (9 s), + stage 0 initramfs (11 s; 4,117,504 bytes)
```

### 13.7 Signing

```
signify-openbsd -G -n -p k.pub -s k.sec -c "hubos test key"
signify-openbsd -S -s k.sec -m manifest -x manifest.sig && signify-openbsd -V -p k.pub -m manifest -x manifest.sig   # Signature Verified, exit 0
sed 's/version=2/version=1/' manifest > m2; signify-openbsd -V -p k.pub -m m2 -x manifest.sig                         # signature verification failed, exit 1
minisign -G -W -p m.pub -s m.sec -f; minisign -S -s m.sec -m manifest -x manifest.minisig; minisign -V -p m.pub -m manifest -x manifest.minisig
openssl genpkey -algorithm ed25519 -out o.key; openssl pkey -in o.key -pubout -out o.pub
openssl pkeyutl -sign -inkey o.key -rawin -in manifest -out manifest.osig; openssl pkeyutl -verify -pubin -inkey o.pub -rawin -in manifest -sigfile manifest.osig
```

### 13.8 GRUB pieces

```
grub-editenv grubenv create && grub-editenv grubenv set slot=a next= tries=0 && grub-editenv grubenv set next=b tries=1 && grub-editenv grubenv list   # slot=a next=b tries=1; file 1024 bytes
grub-mkimage -O x86_64-efi -d $A/root/usr/lib/grub/x86_64-efi -o grubx64.efi -p /EFI/hubos part_gpt fat ext2 normal linux echo test search search_label configfile loadenv efi_gop   # 675,840 bytes
#   (with "squashfs" in the module list: "cannot open squashfs.mod" - GRUB 2.12 has no squashfs module; the kernels sit on the FAT partition anyway)
```

### 13.9 The disk, the bundles and the QEMU runs

```
./mkroot.sh 1 good work/root-1.sqsh                       # Appendix C
./mkdisk.sh work/root-1.sqsh kernel-6.12-hubos.efi /tmp/pb/vmt     # Appendix C
for spec in "1 good" "2 good" "3 noinit" "4 garbage" "5 unhealthy" "6 hang"; do set -- $spec; ./mkbundle.sh $1 $2 bundles/v$1-$2; done   # Appendix D
# QEMU (inside vm.py, Appendix E):
qemu-system-x86_64 -machine q35,smm=off -accel tcg -smp 2 -m 1024 -nographic \
  -drive if=pflash,format=raw,unit=0,readonly=on,file=$A/root/usr/share/OVMF/OVMF_CODE_4M.fd \
  -drive if=pflash,format=raw,unit=1,file=vmt/vars.fd \
  -drive file=vmt/disk.img,if=none,id=d0,format=raw -device virtio-blk-pci,drive=d0 \
  -netdev user,id=n0 -device virtio-net-pci,netdev=n0,romfile= \
  -device i6300esb -watchdog-action reset -serial stdio -monitor none
python3 slice1_tests.py > slice1.log       # Appendix E; about 8 minutes
```

### 13.10 What went wrong on the way (kept because they shaped the design)

| What happened | Cause | Fix / lesson |
|---|---|---|
| kernel build ended with `zstd: not found` | host tool missing | use gzip for the kernel image |
| QEMU: `failed to find romfile "efi-virtio.rom"` | iPXE ROMs not in the unpacked tree | `romfile=` (empty) |
| `mkdir: cannot create directory '/config': Read-only file system` | mount points missing in the read-only image | create them in the image build |
| `ip: not found`, `setsid: failed to execute cttyhack` | busybox applets not linked | symlink the applets in `/usr/local/bin`, put it in `PATH` |
| `udhcpc: socket: Address family not supported by protocol` | kernel without `CONFIG_PACKET` | enable it |
| `Malformed BootOrder order` and duplicate boot entries | my `entry_num` pattern did not match efibootmgr's tab-separated lines | match `[[:space:]]` after the name |
| the new slot reported `slot=a` after booting B | the built-in kernel command line is placed **before** the entry's, both `hubos.slot=` are visible | take the last one |
| after a trial boot `BootOrder` already had the new slot first, and a failed trial returned to the **failed** slot | creating the (missing) recovery entry puts it first in `BootOrder`; my fix-up then rewrote the order with the running slot first | remember `BootOrder` before creating entries and restore it |
| the bundle was refused with `No space left on device` | staging area of 40 MiB for a 39 MB bundle | data partition 160 MiB |
| a slot without `/sbin/init` did not reboot | the kernel falls back to `/bin/sh` | stage 0 |
| stage 0: `unable to open an initial console`, reboot loop | an initramfs built from a directory has no device nodes | build it from a cpio list with `nod /dev/console` |
| stage 0: `cannot mount the root of slot a` | the disk appears after stage 0 starts | retry for 20 s; find the partition by `PARTNAME` in sysfs |
| `watchdog: watchdog0: watchdog did not stop!` printed at every reboot | `NOWAYOUT` is on, by design | expected message |
| `pkill -f` killed my own shell (exit 144) | pattern matched the shell's command line | use `pkill -x` and pid files |
| one start hung after `BdsDxe: starting Boot0001` | UNKNOWN | not reproduced |

---

## Appendix: the test scaffolding, as it was run

These files lived in `/tmp/pb` and are **not** part of the repo. They are printed so that the results above can be repeated and checked. Paths are those of the temporary directory.

### A. Kernel


**Kernel configuration fragment (appended to `make tinyconfig`, then `make olddefconfig`)**

```
# Hub OS phase B slice 1: smallest kernel that boots under QEMU (UEFI, virtio disk + net)
CONFIG_64BIT=y
CONFIG_SMP=y
CONFIG_PRINTK=y
CONFIG_TTY=y
CONFIG_SERIAL_8250=y
CONFIG_SERIAL_8250_CONSOLE=y
CONFIG_BLK_DEV_INITRD=y
CONFIG_RD_GZIP=y
CONFIG_RD_ZSTD=y
CONFIG_BINFMT_ELF=y
CONFIG_BINFMT_SCRIPT=y
CONFIG_PROC_FS=y
CONFIG_SYSFS=y
CONFIG_DEVTMPFS=y
CONFIG_DEVTMPFS_MOUNT=y
CONFIG_TMPFS=y
CONFIG_MULTIUSER=y
CONFIG_FILE_LOCKING=y
CONFIG_FUTEX=y
CONFIG_EPOLL=y
CONFIG_SIGNALFD=y
CONFIG_TIMERFD=y
CONFIG_EVENTFD=y
CONFIG_UNIX=y
CONFIG_BLOCK=y
CONFIG_PCI=y
CONFIG_PCI_MSI=y
CONFIG_ACPI=y
CONFIG_VIRTIO_MENU=y
CONFIG_VIRTIO_PCI=y
CONFIG_VIRTIO_BLK=y
CONFIG_NET=y
CONFIG_INET=y
CONFIG_NETDEVICES=y
CONFIG_NET_CORE=y
CONFIG_VIRTIO_NET=y
CONFIG_EXT4_FS=y
CONFIG_SQUASHFS=y
CONFIG_SQUASHFS_ZSTD=y
CONFIG_SQUASHFS_XZ=y
CONFIG_MISC_FILESYSTEMS=y
CONFIG_VFAT_FS=y
CONFIG_FAT_FS=y
CONFIG_NLS_CODEPAGE_437=y
CONFIG_NLS_ISO8859_1=y
CONFIG_PARTITION_ADVANCED=y
CONFIG_MSDOS_PARTITION=y
CONFIG_EFI_PARTITION=y
CONFIG_EFI=y
CONFIG_EFI_STUB=y
CONFIG_EFIVAR_FS=y
CONFIG_WATCHDOG=y
CONFIG_I6300ESB_WDT=y
CONFIG_HW_RANDOM=y
CONFIG_HW_RANDOM_VIRTIO=y
CONFIG_RTC_CLASS=y
CONFIG_RTC_DRV_CMOS=y
CONFIG_MAGIC_SYSRQ=y
CONFIG_PANIC_TIMEOUT=-1
CONFIG_CRYPTO=y
CONFIG_CRYPTO_CRC32C=y
CONFIG_PACKET=y
CONFIG_INITRAMFS_SOURCE="/tmp/pb/stage0.list"
CONFIG_WATCHDOG_NOWAYOUT=y
CONFIG_KERNEL_GZIP=y
CONFIG_CMDLINE_BOOL=y
CONFIG_CMDLINE="console=ttyS0 ro rootfstype=squashfs root=PARTLABEL=hubos-root-a hubos.slot=a panic=5 loglevel=4 i6300esb.heartbeat=60"
```


**`/tmp/pb/stage0.list` (the initramfs list built into the kernel)**

```
dir /bin 755 0 0
file /bin/busybox /tmp/pb/stage0/bin/busybox 755 0 0
file /init /tmp/pb/stage0/init 755 0 0
dir /dev 755 0 0
nod /dev/console 600 0 0 c 5 1
nod /dev/null 666 0 0 c 1 3
dir /proc 755 0 0
dir /sys 755 0 0
dir /newroot 755 0 0
```


**`/tmp/pb/stage0/init` (stage 0)**

```sh
#!/bin/busybox sh
# Hub OS stage 0: built into the kernel. Mounts the slot's root, arms the
# hardware watchdog, hands over to the slot's /sbin/init. If anything is wrong
# it reboots at once (the firmware then uses the other slot).
/bin/busybox --install -s /bin
export PATH=/bin
mount -t proc proc /proc
mount -t sysfs sysfs /sys
mount -t devtmpfs devtmpfs /dev
echo "STAGE0: start"
# Arm the watchdog: opening it starts the countdown (heartbeat is set on the
# kernel command line); with nowayout, closing it does not stop it. The slot's
# confirm step starts the steady feeder once the boot is healthy.
if [ -c /dev/watchdog ]; then exec 3>/dev/watchdog; exec 3>&-; echo "STAGE0: watchdog armed"; else echo "STAGE0: no watchdog device"; fi
SLOT=$(sed -n 's/.*hubos\.slot=\([ab]\).*/\1/p' /proc/cmdline)
[ -n "$SLOT" ] || SLOT=a
# Find the slot's partition by its GPT name through sysfs (no udev). The disk
# may appear a moment after the kernel starts, so retry for up to 20 s.
DEV=
i=0
while [ -z "$DEV" ] && [ $i -lt 40 ]; do
  for u in /sys/class/block/*/uevent; do
    if grep -q "^PARTNAME=hubos-root-$SLOT\$" "$u" 2>/dev/null; then DEV=/dev/$(basename $(dirname $u)); break; fi
  done
  [ -n "$DEV" ] || { sleep 0.5; i=$((i+1)); }
done
if [ -z "$DEV" ] || ! mount -t squashfs -o ro "$DEV" /newroot; then
  echo "STAGE0: cannot mount the root of slot $SLOT; rebooting"; sleep 1; reboot -f
fi
if [ ! -x /newroot/sbin/init ]; then echo "STAGE0: slot $SLOT has no /sbin/init; rebooting"; sleep 1; reboot -f; fi
echo "STAGE0: switching to slot $SLOT"
exec switch_root /newroot /sbin/init
```

### B. Files inside the root image


**`/sbin/init` (`usr/sbin/init`)**

```sh
#!/bin/sh
# Hub OS stage 1 (PID 1). Mounts the scratch areas, mounts the config
# partition, applies the config, then becomes s6-svscan. Slice 1 proposal;
# not a decision. No systemd.
PATH=/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
export PATH
mount -t proc proc /proc
mount -t sysfs sysfs /sys
mount -t devtmpfs devtmpfs /dev 2>/dev/null
mount -t tmpfs -o mode=0755 tmpfs /run
mount -t tmpfs -o mode=1777 tmpfs /tmp
mount -t tmpfs -o mode=0755 tmpfs /var
mkdir -p /var/log /var/tmp /run/service /config /data /boot/efi
mount -t efivarfs efivarfs /sys/firmware/efi/efivars 2>/dev/null
echo "HUBOS: stage1 start, cmdline: $(cat /proc/cmdline)"
case " $(cat /proc/cmdline) " in
  *" hubos.recovery=1 "*)
    echo "HUBOS: RECOVERY MODE (bare terminal; root is read-only; nothing else started)"
    cat /etc/hubos-release
    exec setsid cttyhack /bin/sh ;;
esac
# The kernel has a built-in command line (for the fallback boot path) and the
# firmware adds the entry's own; the LAST hubos.slot= is the one that counts.
SLOT=$(sed -n 's/.*hubos\.slot=\([ab]\).*/\1/p' /proc/cmdline)
[ -n "$SLOT" ] || SLOT=unknown
echo "$SLOT" > /run/hubos-slot
CFG=$(findfs PARTLABEL=hubos-config 2>/dev/null)
DATA=$(findfs PARTLABEL=hubos-data 2>/dev/null)
ESP=$(findfs PARTLABEL=hubos-esp 2>/dev/null)
[ -n "$CFG" ]  && mount -o noatime "$CFG" /config  || echo "HUBOS: no config partition"
[ -n "$DATA" ] && mount -o noatime "$DATA" /data   || echo "HUBOS: no data partition"
echo "HUBOS: efi: $(efibootmgr 2>/dev/null | grep -E '^Boot(Current|Next|Order)' | tr '\n' ' ')"
echo "HUBOS: slot=$SLOT release: $(cat /etc/hubos-release | tr '\n' ' ')"
# --- apply the declarative config (slice 1: a sh-sourceable file) ---
NAME=hubos-node; NET=dhcp; IFACE=eth0; ADDR=; GW=; SERVICES="hubd console confirm wd"
[ -f /config/hubos/node.conf ] && . /config/hubos/node.conf
hostname "$NAME"
ip link set lo up
ip link set "$IFACE" up 2>/dev/null
if [ "$NET" = static ]; then
  ip addr add "$ADDR" dev "$IFACE"; [ -n "$GW" ] && ip route add default via "$GW"
else
  udhcpc -i "$IFACE" -n -q -s /usr/lib/hubos/udhcpc.script >/run/udhcpc.log 2>&1 || echo "HUBOS: dhcp failed"
fi
echo "HUBOS: node=$NAME net=$NET $(ip -4 addr show "$IFACE" 2>/dev/null | awk '/inet /{print $2}')"
# --- services: copy the chosen ones into the supervised directory ---
for s in $SERVICES; do cp -a /etc/s6/sv/$s /run/service/$s; done
mkdir -p /run/service/.s6-svscan
printf '#!/bin/sh\necho "HUBOS: s6-svscan finished; rebooting"\nreboot -f\n' > /run/service/.s6-svscan/finish
printf '#!/bin/sh\necho "HUBOS: s6-svscan crashed; rebooting"\nreboot -f\n' > /run/service/.s6-svscan/crash
chmod +x /run/service/.s6-svscan/finish /run/service/.s6-svscan/crash
echo "HUBOS: handing over to s6-svscan"
exec s6-svscan /run/service
```


**`/usr/sbin/hubos-ctl`**

```sh
#!/bin/sh
# hubos-ctl: boot entries, update, confirm. Slice 1 proposal (POSIX sh); not a decision.
# Subcommands:  status | ensure-entries | update BASE | confirm
PATH=/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
DISK=${HUBOS_DISK:-/dev/vda}
PUB=/etc/hubos/update.pub
STATE=/config/hubos/state
SLOT=$(cat /run/hubos-slot 2>/dev/null)
OTHER=a; [ "$SLOT" = a ] && OTHER=b
RUNVER=$(sed -n 's/^version=//p' /etc/hubos-release)
CMD_COMMON="console=ttyS0 ro rootfstype=squashfs panic=5 loglevel=4"
die() { echo "hubos-ctl: $*" >&2; exit 2; }

entry_num() { efibootmgr 2>/dev/null | sed -n "s/^Boot\([0-9A-F]\{4\}\)\*\{0,1\}[[:space:]]$1[[:space:]].*/\1/p" | head -1; }

ensure_entries() {
  [ -d /sys/firmware/efi/efivars ] || die "no EFI variables (not booted by UEFI?)"
  # Creating an entry puts it first in BootOrder, so remember the order and
  # put it back afterwards (new entries go to the end).
  oldorder=$(efibootmgr 2>/dev/null | sed -n 's/^BootOrder: //p')
  created=
  for s in a b; do
    [ -n "$(entry_num hubos-$s)" ] && continue
    efibootmgr -q -c -d $DISK -p 1 -L hubos-$s -l "\\EFI\\hubos\\kernel-$s.efi" -u "$CMD_COMMON root=PARTLABEL=hubos-root-$s hubos.slot=$s" || die "cannot create entry $s"
    created=1
  done
  [ -n "$(entry_num hubos-recovery)" ] || { efibootmgr -q -c -d $DISK -p 1 -L hubos-recovery -l "\\EFI\\hubos\\kernel-$SLOT.efi" -u "$CMD_COMMON root=PARTLABEL=hubos-root-$SLOT hubos.recovery=1 hubos.slot=$SLOT" || die "cannot create recovery entry"; created=1; }
  [ -n "$created" ] || return 0
  a=$(entry_num hubos-a); b=$(entry_num hubos-b); r=$(entry_num hubos-recovery)
  if [ -n "$oldorder" ] && echo "$oldorder" | grep -q "$a" && echo "$oldorder" | grep -q "$b"; then
    efibootmgr -q -o "$oldorder" || die "cannot restore BootOrder"        # a confirmed order exists: keep it
  else
    efibootmgr -q -o "$(entry_num hubos-$SLOT),$(entry_num hubos-$OTHER)"  # first install: the running slot first
  fi
}

status() {
  echo "slot=$SLOT release=$RUNVER"; efibootmgr 2>/dev/null | sed -n '/^Boot\(Current\|Next\|Order\)/p; /^Boot[0-9A-F]*\*\{0,1\}[[:space:]]hubos/s/\t.*//p'
  echo "min_version=$(cat $STATE/min_version 2>/dev/null || echo none)"
}

fetch() { # fetch BASE NAME DEST
  case "$1" in
    http://*) wget -q -O "$3" "$1/$2" || return 1 ;;
    *)        cp "$1/$2" "$3" || return 1 ;;
  esac
}

update() {
  BASE=$1; [ -n "$BASE" ] || die "usage: hubos-ctl update BASE"
  [ -n "$SLOT" ] && [ "$SLOT" != unknown ] || die "running slot unknown"
  W=/data/update; rm -rf $W; mkdir -p $W
  echo "update: fetching manifest from $BASE"
  fetch "$BASE" manifest $W/manifest || die "no manifest at $BASE"
  fetch "$BASE" manifest.sig $W/manifest.sig || die "REFUSED: no signature (unsigned bundle)"
  signify-openbsd -V -p $PUB -m $W/manifest -x $W/manifest.sig >/dev/null 2>&1 || die "REFUSED: bad signature (manifest not signed by the owner's key, or changed)"
  echo "update: signature OK"
  NEWVER=$(sed -n 's/^version //p' $W/manifest)
  [ -n "$NEWVER" ] || die "REFUSED: manifest has no version"
  MIN=$(cat $STATE/min_version 2>/dev/null || echo 0)
  [ "$NEWVER" -gt "$RUNVER" ] 2>/dev/null || die "REFUSED: version $NEWVER is not newer than the running version $RUNVER"
  [ "$NEWVER" -ge "$MIN" ] 2>/dev/null || die "REFUSED: version $NEWVER is below the floor $MIN"
  for f in kernel.efi rootfs.sqsh; do
    want=$(sed -n "s/^$f sha256 \([0-9a-f]*\) size .*/\1/p" $W/manifest); [ -n "$want" ] || die "REFUSED: manifest does not list $f"
    echo "update: fetching $f"; fetch "$BASE" $f $W/$f || die "cannot fetch $f"
    got=$(sha256sum $W/$f | cut -d' ' -f1)
    [ "$got" = "$want" ] || die "REFUSED: $f does not match the signed hash (tampered or damaged)"
  done
  echo "update: hashes OK, version $NEWVER, writing to slot $OTHER"
  DEV=$(findfs PARTLABEL=hubos-root-$OTHER) || die "no partition hubos-root-$OTHER"
  SZ=$(stat -c %s $W/rootfs.sqsh)
  dd if=$W/rootfs.sqsh of=$DEV bs=1M conv=fsync 2>/dev/null || die "write failed"
  [ "$(head -c $SZ $DEV | sha256sum | cut -d' ' -f1)" = "$(sed -n 's/^rootfs.sqsh sha256 \([0-9a-f]*\) .*/\1/p' $W/manifest)" ] || die "read-back check failed"
  mount -t vfat "$(findfs PARTLABEL=hubos-esp)" /boot/efi || die "cannot mount the boot partition"
  cp $W/kernel.efi /boot/efi/EFI/hubos/kernel-$OTHER.efi.new && sync && mv /boot/efi/EFI/hubos/kernel-$OTHER.efi.new /boot/efi/EFI/hubos/kernel-$OTHER.efi && sync
  umount /boot/efi
  ensure_entries
  efibootmgr -q -n "$(entry_num hubos-$OTHER)" || die "cannot set BootNext"
  rm -rf $W
  echo "update: installed version $NEWVER in slot $OTHER; BootNext set; reboot to try it"
}

confirm() {
  cur=$(efibootmgr | sed -n 's/^BootCurrent: //p'); first=$(efibootmgr | sed -n 's/^BootOrder: \([0-9A-F]*\).*/\1/p')
  mine=$(entry_num hubos-$SLOT)
  if [ "$cur" = "$mine" ] && [ "$first" = "$mine" ]; then echo "confirm: slot $SLOT is already the confirmed slot"; return 0; fi
  [ "$cur" = "$mine" ] || die "confirm: BootCurrent $cur is not slot $SLOT's entry ($mine); not confirming"
  efibootmgr -q -o "$mine,$(entry_num hubos-$OTHER)" || die "cannot set BootOrder"
  mkdir -p $STATE; echo "$RUNVER" > $STATE/min_version; sync
  echo "confirm: boot of slot $SLOT (version $RUNVER) confirmed; floor raised to $RUNVER"
}

case "$1" in
  status) status;; ensure-entries) ensure_entries;; update) update "$2";; confirm) confirm;;
  *) echo "usage: hubos-ctl status|ensure-entries|update BASE|confirm"; exit 1;;
esac
```


**`/usr/lib/hubos/udhcpc.script`**

```sh
#!/bin/sh
case "$1" in
  bound|renew)
    ip addr flush dev "$interface" 2>/dev/null
    ip addr add "$ip/${mask:-24}" dev "$interface"
    [ -n "$router" ] && ip route add default via "${router%% *}"
    [ -n "$dns" ] && for d in $dns; do echo "nameserver $d"; done > /run/resolv.conf ;;
esac
```


**`/etc/s6/sv/hubd/run`**

```sh
#!/bin/sh
exec 2>&1
mkdir -p /run/hubos; chown hub:hub /run/hubos; chmod 700 /run/hubos
exec s6-setuidgid hub /usr/bin/hubd serve --inventory /config/hubos/inventory.toml --viewers /config/hubos/viewers.toml --socket /run/hubos/hubd.sock
```


**`/etc/s6/sv/hubd/log/run`**

```sh
#!/bin/sh
exec s6-log n5 s200000 /var/log/hubd
```


**`/etc/s6/sv/console/run`**

```sh
#!/bin/sh
exec cttyhack /bin/sh +m
```


**`/etc/s6/sv/confirm/run`**

```sh
#!/bin/sh
# Boot succeeded = hubd answers on its socket 5 times in a row, 1 s apart.
# Gives up after HUBOS_CONFIRM_TIMEOUT seconds (default 60) and reboots, so the
# firmware falls back to the other slot (BootOrder was not changed).
exec 2>&1
T=${HUBOS_CONFIRM_TIMEOUT:-60}; ok=0; n=0
[ -f /config/hubos/node.conf ] && . /config/hubos/node.conf
T=${CONFIRM_TIMEOUT:-$T}
ensure() { hubos-ctl ensure-entries; }
ensure
while [ $n -lt $T ]; do
  if /usr/bin/hubd list --socket /run/hubos/hubd.sock >/dev/null 2>&1; then ok=$((ok+1)); else ok=0; fi
  [ $ok -ge 5 ] && break
  n=$((n+1)); sleep 1
done
if [ $ok -ge 5 ]; then hubos-ctl confirm; s6-svc -u /run/service/wd; s6-svc -O . ; exit 0; fi
echo "confirm: hubd did not become healthy in ${T}s; this boot FAILED; rebooting into the other slot"
sync; reboot -f
```


**`/etc/s6/sv/wd/run` (with an empty file `down` next to it)**

```sh
#!/bin/sh
# steady-state watchdog feeder; started by the confirm step once the boot is healthy
exec watchdog -F -t 5 -T 60 /dev/watchdog
```

### C. Building the root and the disk


**`mkroot.sh`**

```bash
#!/bin/bash
# mkroot.sh VERSION FLAVOR OUT.sqsh   (FLAVOR: good | noinit | unhealthy)
set -e
source /tmp/pb/env.sh
V=$1; F=$2; OUT=$3
S=/tmp/pb/work/stage-$V-$F
rm -rf $S; cp -a /tmp/pb/rootfs-a $S
cp -a /tmp/pb/src/rootfs-extra/. $S/
cp /tmp/pb/hubd $S/usr/bin/hubd
cp /tmp/pb/root/bin/efibootmgr $S/usr/sbin/efibootmgr
cp -a /tmp/pb/root/usr/lib/x86_64-linux-gnu/libefivar.so.1* /tmp/pb/root/usr/lib/x86_64-linux-gnu/libefiboot.so.1* $S/usr/lib/x86_64-linux-gnu/
cp /tmp/pb/root/bin/signify-openbsd $S/usr/bin/signify-openbsd
cp -a /lib/x86_64-linux-gnu/libbsd.so.0* $S/usr/lib/x86_64-linux-gnu/
mkdir -p $S/etc/hubos $S/usr/local/bin $S/config $S/data $S/boot/efi $S/run $S/tmp $S/var
cp /tmp/pb/sig/k.pub $S/etc/hubos/update.pub
for a in ip udhcpc wget cttyhack reboot halt poweroff hostname stat awk head sha256sum getty watchdog; do ln -sf /bin/busybox $S/usr/local/bin/$a; done
printf 'version=%s\nflavor=%s\nbuilt=reproducible-test\n' "$V" "$F" > $S/etc/hubos-release
echo 'hub:x:1000:1000:hub:/run/hubos:/bin/false' >> $S/etc/passwd
echo 'hub:x:1000:' >> $S/etc/group
echo 'root::0:0:root:/root:/bin/sh' > $S/etc/passwd.new; grep -v '^root:' $S/etc/passwd >> $S/etc/passwd.new; mv $S/etc/passwd.new $S/etc/passwd
ln -sfn /usr/lib/hubos/udhcpc.script $S/usr/share/udhcpc.script 2>/dev/null || true
case $F in
  noinit) rm -f $S/usr/sbin/init ;;
  hang) printf '#!/bin/sh\nexec sleep 100000\n' > $S/usr/sbin/init ;;
  unhealthy) printf '#!/bin/sh\nexec sleep 100000\n' > $S/etc/s6/sv/hubd/run ;;
esac
if [ "$F" = garbage ]; then head -c 2097152 /dev/zero > $OUT; else mksquashfs $S $OUT -comp zstd -quiet -noappend >/dev/null; fi
rm -rf $S
ls -la $OUT | awk '{print "rootfs", $5, "bytes"}'
```


**`mkdisk.sh`**

```bash
#!/bin/bash
# mkdisk.sh ROOT.sqsh KERNEL OUTDIR  -> OUTDIR/disk.img and OUTDIR/vars.fd
set -e
source /tmp/pb/env.sh
R=$1; K=$2; D=$3; W=/tmp/pb/work
mkdir -p $D
rm -f $D/disk.img $D/esp.img $D/cfg.img $D/data.img
# partition images
truncate -s 64M $D/esp.img; mkfs.vfat -F 32 -n ESP $D/esp.img >/dev/null
export MTOOLS_SKIP_CHECK=1
mmd -i $D/esp.img ::/EFI ::/EFI/BOOT ::/EFI/hubos
mcopy -i $D/esp.img $K ::/EFI/BOOT/BOOTX64.EFI
mcopy -i $D/esp.img $K ::/EFI/hubos/kernel-a.efi
truncate -s 16M $D/cfg.img; mkfs.ext4 -q -L hubos-config -d $W/cfg $D/cfg.img
truncate -s 160M $D/data.img; mkfs.ext4 -q -L hubos-data $D/data.img
truncate -s 420M $D/disk.img
sfdisk -q $D/disk.img <<PT
label: gpt
start=2048, size=64MiB, type=U, name="hubos-esp"
size=64MiB, type=L, name="hubos-root-a"
size=64MiB, type=L, name="hubos-root-b"
size=16MiB, type=L, name="hubos-config"
size=160MiB, type=L, name="hubos-data"
PT
off() { sfdisk -J $D/disk.img | python3 -c "import json,sys; p=json.load(sys.stdin)['partitiontable']['partitions'][$1]; print(p['start'])"; }
dd if=$D/esp.img  of=$D/disk.img bs=512 seek=$(off 0) conv=notrunc status=none
dd if=$R          of=$D/disk.img bs=512 seek=$(off 1) conv=notrunc status=none
dd if=$D/cfg.img  of=$D/disk.img bs=512 seek=$(off 3) conv=notrunc status=none
dd if=$D/data.img of=$D/disk.img bs=512 seek=$(off 4) conv=notrunc status=none
rm -f $D/esp.img $D/cfg.img $D/data.img
cp /tmp/pb/root/usr/share/OVMF/OVMF_VARS_4M.fd $D/vars.fd
sfdisk -l $D/disk.img | tail -7
```

### D. Bundles and signing


**`mkbundle.sh`**

```bash
#!/bin/bash
# mkbundle.sh VERSION FLAVOR OUTDIR  -> OUTDIR/{manifest,manifest.sig,kernel.efi,rootfs.sqsh}
set -e
source /tmp/pb/env.sh
V=$1; F=$2; D=$3; mkdir -p $D
/tmp/pb/mkroot.sh $V $F $D/rootfs.sqsh >/dev/null
cp /tmp/pb/kernel-6.12-hubos.efi $D/kernel.efi
{
 echo "hubos-bundle 1"; echo "version $V"; echo "flavor $F"; echo "arch x86_64"
 for f in kernel.efi rootfs.sqsh; do echo "$f sha256 $(sha256sum $D/$f | cut -d' ' -f1) size $(stat -c %s $D/$f)"; done
} > $D/manifest
signify-openbsd -S -s ${KEY:-/tmp/pb/sig/k.sec} -m $D/manifest -x $D/manifest.sig
```


The refused variants were made from `v2-good` with hard links: `v2-unsigned` (no `manifest.sig`), `v2-badroot` (one byte of `rootfs.sqsh` changed with `dd`), `v2-badmanifest` (`sed 's/^version 2/version 9/'` on the manifest, old signature), `v2-wrongkey` (signed with a second key), and `v1-good` (a correctly signed older version).

### E. The QEMU harness, the tests, and the s6 build


**`vm.py`**

```python
import subprocess, threading, time, re, os, sys, queue
A='/tmp/pb'
ENV=dict(os.environ)
ENV['LD_LIBRARY_PATH']=f'{A}/root/usr/lib/x86_64-linux-gnu:{A}/root/lib/x86_64-linux-gnu:{A}/root/usr/lib'
ENV['QEMU_MODULE_DIR']=f'{A}/root/usr/lib/x86_64-linux-gnu/qemu'
class VM:
    def __init__(self, d, extra=(), mem=1024, log=None):
        self.d=d; self.buf=''; self.lock=threading.Lock(); self.t0=time.time()
        self.log=open(log or f'{d}/serial.log','a')
        cmd=[f'{A}/root/usr/bin/qemu-system-x86_64','-L',f'{A}/root/usr/share/qemu','-L',f'{A}/root/usr/share/seabios',
             '-machine','q35,smm=off','-accel','tcg','-smp','2','-m',str(mem),'-nographic','-display','none',
             '-drive',f'if=pflash,format=raw,unit=0,readonly=on,file={A}/root/usr/share/OVMF/OVMF_CODE_4M.fd',
             '-drive',f'if=pflash,format=raw,unit=1,file={d}/vars.fd',
             '-drive',f'file={d}/disk.img,if=none,id=d0,format=raw','-device','virtio-blk-pci,drive=d0',
             '-netdev','user,id=n0','-device','virtio-net-pci,netdev=n0,romfile=',
             '-device','i6300esb','-watchdog-action','reset','-serial','stdio','-monitor','none']+list(extra)
        self.p=subprocess.Popen(cmd,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,env=ENV)
        threading.Thread(target=self._rd,daemon=True).start()
    def _rd(self):
        while True:
            b=self.p.stdout.read1(4096)
            if not b: break
            s=b.decode('utf8','replace')
            with self.lock:
                self.buf+=s; self.log.write(f'[{time.time()-self.t0:7.1f}] '+s.replace('\r','') if False else s.replace('\r','')); self.log.flush()
    def wait(self, pat, timeout=120, start=0):
        end=time.time()+timeout
        while time.time()<end:
            with self.lock: m=re.search(pat,self.buf[start:])
            if m: return start+m.end()
            if self.p.poll() is not None: break
            time.sleep(0.2)
        return None
    def mark(self):
        with self.lock: return len(self.buf)
    def sh(self, cmd, timeout=60):
        tag=f'__D{int(time.time()*1000)%100000000}__'
        m=self.mark()
        self.p.stdin.write((cmd+f'\necho {tag}$?\n').encode()); self.p.stdin.flush()
        e=self.wait(re.escape(tag)+r'(\d+)',timeout,m)
        if e is None: return None, '<timeout>'
        with self.lock: seg=self.buf[m:e]
        mm=re.search(re.escape(tag)+r'(\d+)',seg)
        out=seg[:mm.start()]
        # strip the echoed command line(s)
        lines=out.split('\n')
        lines=[l for l in lines if tag not in l]
        return int(mm.group(1)), '\n'.join(lines).strip('\n')
    def stop(self):
        try: self.p.kill()
        except: pass
        self.p.wait()
```


**`slice1_tests.py`**

```python
import sys,time,re,threading,http.server,socketserver,functools,os,shutil,subprocess
sys.path.insert(0,'/tmp/pb')
from vm import VM
D='/tmp/pb/vmt'
# http server for bundles (host side, reached from the guest as 10.0.2.2:8000)
class Q(http.server.SimpleHTTPRequestHandler):
    def log_message(self,*a): pass
H=functools.partial(Q,directory='/tmp/pb/bundles')
socketserver.TCPServer.allow_reuse_address=True
srv=socketserver.ThreadingTCPServer(('0.0.0.0',8000),H); threading.Thread(target=srv.serve_forever,daemon=True).start()
BASE='http://10.0.2.2:8000'
results=[]
def T(name):
    print(f'\n=== {name}',flush=True); return time.time()
def ok(name,cond,t0,note=''):
    results.append((name,'PASS' if cond else 'FAIL',time.time()-t0,note)); print(f'--> {"PASS" if cond else "FAIL"} ({time.time()-t0:.1f}s) {note}',flush=True)
def boot(v,start=0,timeout=240):
    e=v.wait(r'HUBOS: handing over to s6-svscan',timeout,start)
    if e is None: return None
    time.sleep(2.5); v.p.stdin.write(b'stty -echo\n'); v.p.stdin.flush(); time.sleep(0.5)
    return e
def run(v,c,timeout=90):
    rc,out=v.sh(c,timeout); print(f'$ {c}\n{out}' + (f'\n[rc={rc}]' if rc else ''),flush=True); return rc,out
# fresh disk with release 1 in slot A
shutil.rmtree(D,ignore_errors=True)
subprocess.run(['/tmp/pb/mkdisk.sh','/tmp/pb/work/root-1.sqsh','/tmp/pb/kernel-6.12-hubos.efi',D],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,env={**os.environ})
v=VM(D); t0=time.time()
assert boot(v), 'first boot failed'
print(f'first boot to handover: {time.time()-t0:.1f}s',flush=True)

t=T('1. read-only root')
rc1,o=run(v,'touch /usr/x 2>&1; echo rc=$?; touch /etc/x 2>&1; echo rc=$?; echo ok > /var/y && echo var-writable; mount | grep " on / "')
ok('1 read-only root', 'Read-only file system' in o and 'var-writable' in o and 'squashfs' in o, t)

t=T('2. first boot creates boot entries, confirms, hubd runs from the config partition')
v.wait(r'ensure|confirm:',15)
time.sleep(1)
run(v,'hubos-ctl status; ls -la /config/hubos; hubd check --inventory /config/hubos/inventory.toml | tail -3; hubd list --socket /run/hubos/hubd.sock | sed -n 1,4p; ps | grep "[h]ubd serve" | head -2')
rc,o=run(v,'hubd list --socket /run/hubos/hubd.sock | head -3')
ok('7 hubd from config partition', rc==0 and 'Down machines' in o or 'Hub' in o, t)

t=T('3. init restarts a killed service')
rc,o=run(v,"s6-svstat /run/service/hubd; OLD=$(s6-svstat /run/service/hubd | sed 's/.*pid \\([0-9]*\\).*/\\1/'); echo killing $OLD; kill -9 $OLD; sleep 2; s6-svstat /run/service/hubd; hubd list --socket /run/hubos/hubd.sock >/dev/null && echo hubd-answers-again")
ok('6 init restarts a killed service', 'hubd-answers-again' in o, t)

# marker on the config partition to prove it survives an update
run(v,'echo survive-me > /config/hubos/marker; sync')

t=T('4. unsigned, tampered, wrong key, replay: all refused')
refused=[]
for name,exp in [('v2-unsigned','no signature'),('v2-badmanifest','bad signature'),('v2-wrongkey','bad signature'),('v2-badroot','tampered or damaged'),('v1-good','not newer than')]:
    rc,o=run(v,f'hubos-ctl update {BASE}/{name} 2>&1; echo END')
    refused.append(('REFUSED' in o and exp in o))
rc,o=run(v,'hubos-ctl status | grep -E "BootNext|BootOrder|slot="; head -c 16 /dev/vda3 | od -c | head -2')
ok('5 unsigned/tampered refused', all(refused) and 'BootNext' not in o, t, str(refused))

t=T('5. signed update accepted: write to slot B, reboot, boot B, confirm')
t1=time.time()
rc,o=run(v,f'hubos-ctl update {BASE}/v2-good 2>&1',timeout=300)
print(f'update took {time.time()-t1:.1f}s',flush=True)
run(v,'hubos-ctl status')
m=v.mark(); v.p.stdin.write(b'sync; reboot -f\n'); v.p.stdin.flush()
tb=time.time(); e=boot(v,m); print(f'reboot to handover: {time.time()-tb:.1f}s',flush=True)
v.wait(r'confirm: boot of slot b.*confirmed|FAILED',90,m); time.sleep(1)
rc,o=run(v,'hubos-ctl status; cat /config/hubos/marker; cat /etc/hubos-release | head -1')
rc,o2=run(v,f'hubos-ctl update {BASE}/v1-good 2>&1')
ok('3/2 signed update accepted, config survives', 'slot=b release=2' in o and 'survive-me' in o and 'BootOrder: 0008,0007' in o and 'min_version=2' in o and 'REFUSED' in o2 and 'not newer' in o2, t)

def bad_update(label,bundle,expect_re,rollback_wait,extra_ok=lambda b:True):
    global v
    t=T(label)
    rc,o=run(v,f'hubos-ctl update {BASE}/{bundle} 2>&1',timeout=300)
    m=v.mark(); v.p.stdin.write(b'sync; reboot -f\n'); v.p.stdin.flush()
    tb=time.time()
    got=v.wait(expect_re,rollback_wait,m)
    print(f'saw {expect_re!r}:', got is not None, f'after {time.time()-tb:.1f}s',flush=True)
    e=boot(v,m,timeout=400)
    print(f'back in the confirmed slot after {time.time()-tb:.1f}s (handover reached: {e is not None})',flush=True)
    time.sleep(10)
    rc,o=run(v,'hubos-ctl status; cat /run/hubos-slot')
    ok(label, got is not None and 'slot=b release=2' in o and 'BootOrder: 0008,0007' in o, t, f'rollback took {time.time()-tb:.1f}s')

bad_update('6. signed update with no /sbin/init: stage 0 refuses it and reboots','v3-noinit',r'STAGE0: slot a has no /sbin/init; rebooting',200)
bad_update('6b. signed update whose root is garbage: stage 0 cannot mount it and reboots','v4-garbage',r'STAGE0: cannot mount the root of slot a; rebooting',200)
bad_update('7. signed update that boots but never gets healthy: confirm times out and reboots','v5-unhealthy',r'this boot FAILED',200)
bad_update('7b. signed update whose init hangs: the hardware watchdog resets the machine','v6-hang',r'STAGE0: switching to slot a',200)

t=T('8. recovery mode (boot option)')
run(v,'efibootmgr -n $(efibootmgr | sed -n "s/^Boot\\([0-9A-F]*\\)\\*\\? hubos-recovery.*/\\1/p" | head -1) | head -2')
m=v.mark(); v.p.stdin.write(b'sync; reboot -f\n'); v.p.stdin.flush()
e=v.wait(r'RECOVERY MODE',200,m); time.sleep(2)
print('recovery banner seen:', e is not None,flush=True)
v.p.stdin.write(b'stty -echo\n'); v.p.stdin.flush(); time.sleep(0.5)
rc,o=run(v,'cat /proc/cmdline; ps | head -6; touch /usr/x 2>&1; ls /run/service 2>&1 | head -2')
ok('8 recovery mode', e is not None and 'hubos.recovery=1' in o and 'Read-only' in o, t)
m=v.mark(); v.p.stdin.write(b'sync; reboot -f\n'); v.p.stdin.flush()
boot(v,m); v.wait(r'confirm: slot b is already the confirmed slot',60,m)
rc,o=run(v,'hubos-ctl status | head -3')
v.stop()
print('\nRESULTS'); 
for r in results: print(f'{r[1]:5s} {r[2]:6.1f}s  {r[0]}  {r[3]}')
```


**`skarnet/build.sh`**

```bash
#!/bin/bash
set -e
P=/tmp/pb/skout; mkdir -p $P
s0=$(date +%s)
for p in skalibs execline s6 s6-rc s6-linux-init; do
  t=$(ls $p-*.tar.gz | head -1); d=${t%.tar.gz}; s=$(date +%s)
  tar -xzf $t; cd $d
  ./configure --prefix=$P --disable-shared --enable-static --enable-static-libc --with-sysdeps=$P/lib/skalibs/sysdeps --with-include=$P/include --with-lib=$P/lib --with-lib=$P/lib/skalibs >/tmp/pb/skarnet/$p.conf.log 2>&1
  make -j4 >/tmp/pb/skarnet/$p.make.log 2>&1 && make install >/dev/null 2>&1
  cd ..; echo "$p: $(( $(date +%s) - s ))s"
done
echo "total: $(( $(date +%s) - s0 ))s"
```

**`t_pl.py`** (test 9, the interrupted update)

```python
import sys,time,threading,http.server,socketserver,functools; sys.path.insert(0,'/tmp/pb')
from vm import VM
D='/tmp/pb/vmt'
class Q(http.server.SimpleHTTPRequestHandler):
    def log_message(self,*a): pass
socketserver.TCPServer.allow_reuse_address=True
srv=socketserver.ThreadingTCPServer(('0.0.0.0',8000),functools.partial(Q,directory='/tmp/pb/bundles')); threading.Thread(target=srv.serve_forever,daemon=True).start()
def boot(v,m=0):
    assert v.wait(r'HUBOS: handing over',200,m); time.sleep(2.5); v.p.stdin.write(b'stty -echo\n'); v.p.stdin.flush(); time.sleep(.5)
v=VM(D); boot(v); time.sleep(12)
print(v.sh('hubos-ctl status | head -3')[1])
v.p.stdin.write(b'hubos-ctl update http://10.0.2.2:8000/v6-hang > /tmp/u.log 2>&1 &\n'); v.p.stdin.flush()
v.wait(r'zzzz-never',4)   # 4 s: download done, the write to the other slot is under way
print('--- killing QEMU (power loss) 4 s into the update'); v.p.kill(); v.p.wait()
v2=VM(D,log='/tmp/pb/vmt/serial2.log'); boot(v2); time.sleep(10)
print(v2.sh('hubos-ctl status | head -4; head -c 8 /dev/vda2 | od -c | head -1')[1])
v2.stop()
```
