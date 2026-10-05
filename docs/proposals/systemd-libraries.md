# The two systemd libraries in the images: `libsystemd0` and `libudev1`

> Scope: this concerns the hub and the project's reference images; a node's distro may use systemd.

> **PROPOSAL / RESEARCH; nothing here is a decision.** The owner decides (open question in `HUB-OS.md`).

**Written:** 2026-10-03. **Why:** `docs/proposals/phase-b-image.md` found that the Ubuntu 24.04 base pulls in two *libraries* from the systemd source package, although no systemd program is installed or runs. The owner's rule is "no systemd, ever". This file collects what is needed to decide.

Labels: **TESTED** (the command and what it printed are in section 8), **SOURCE** (read in a document or source file; link given), **BELIEVED**, **UNKNOWN**. Nothing was installed on the machine: packages were downloaded into a temporary directory (`/tmp/sl`) and unpacked with `dpkg -x`, a base root was made there with `debootstrap`, and everything was deleted afterwards. All results are for **Ubuntu 24.04 ("noble")** with the package versions of 2026-10-03 (`libsystemd0` and `libudev1` 255.4-1ubuntu8.17).

---

## 0. Summary

1. **Both libraries are needed, as hard package dependencies, by very little that has to run, and by a lot of what makes up the hub's desktop.** Headless things (`foot`, `seatd`, `busybox`, `s6`, `efibootmgr`, `e2fsprogs`, the Wayland/GPU libraries, `sshd` itself) need neither at the program level (TESTED with `readelf`). The pieces that do link them are `tmux`, `dbus-daemon`/`libdbus-1`, `libseat1`, `libinput`, PipeWire's protocol modules, `libpulse`, `libvte`, GTK 3 (through `libcolord`), `waybar`, `apt`, PAM's modules, `procps` and three tools of `util-linux`.
2. **Ubuntu's `sshd` does not link `libsystemd`** (TESTED: its `NEEDED` list has no systemd or udev library, and `sshd -t` passes with both libraries deleted). It is only the *package* dependencies (`libpam-modules`, `procps`, and `openssh-client` → `libfido2` → `libudev1`) that pull the libraries in.
3. **A real danger, TESTED:** installing `wireplumber`, `pipewire`, `wofi` or `virt-viewer` with plain `apt` on a base root **also installs the `systemd` package itself, `systemd-sysv`, `libpam-systemd` and `dbus-user-session`**, through the dependency `default-dbus-session-bus | dbus-session-bus`. This is stopped by pinning those packages to -1 (then `apt` takes `dbus-x11` instead; TESTED in a simulation), and the image build must test for it. This matters more than the two libraries.
4. **`libudev` without systemd:** none of `libudev-zero`, `eudev`, `mdevd` is in the Ubuntu 24.04 or Debian trixie/forky archives (TESTED by reading their package indexes). `libudev-zero` builds from source in **1 second** (42 KB), exports 80 of systemd's 92 `libudev` function names, and **all** the `udev_*` functions used by `libinput`, `libgudev`, `waybar`, `libapt`, PipeWire's ALSA and V4L2 plugins, `lsblk` and `findmnt` are among them (TESTED). `lsblk` and `findmnt` also ran with it in place, printing a harmless "no version information available" warning.
5. **`libsystemd0` has no drop-in replacement** that I could find; the way out is to rebuild each package without it. The ones I tried: `tmux` (6 s; upstream builds without systemd by default), `seatd`/`libseat` (2 s, option `-Dlibseat-logind=disabled`). For `dbus`, PipeWire, `waybar` the source has an option (SOURCE) but I did not build them.
6. **Whether `libinput` or a Smithay-based compositor such as driftwm works with `libudev-zero` I could not find out** from documents (UNKNOWN). Symbols match; nothing was run with input devices (there are none here).
7. **Deleting `apt`, `dpkg`, `procps` and PAM** from a headless image (TESTED, link level): `apt`, `procps`, `login`, `passwd` and all of PAM's modules can go with **nothing left with an unresolved library** among the files that remain. `dpkg` is an Essential package that other Essential ones depend on (package-level only); deleting its files after the build is possible.
8. **Recommendation:** keep the two libraries for now (option 1), **enforce the program part of the rule by pin and by a test**, delete what is useless anyway (apt, PAM, procps) from the images, and make the headless role images (NAS, backup NAS, VM host, guests) free of both libraries later, where it costs a handful of small rebuilds. Rebuilding the whole desktop stack for the hub is the expensive part; decide that when the desktop slice starts.

---

## 1. Which packages need `libsystemd0` and `libudev1`

### 1.1 Method

- **Package level** (`apt-cache` style, TESTED): I parsed the Ubuntu 24.04 package index (`apt-cache dumpavail`, 87,145 records) and followed `Depends`/`Pre-Depends` (and, in a second pass, `Recommends`) to find the shortest chain from each package to each library (`/tmp/sl/graph.py`, section 8). "**Hard**" = a chain of `Depends`/`Pre-Depends`; "**optional**" = the chain needs a `Recommends`. A `*` in a chain means an edge with alternatives (`a | b`) where I followed the first one; apt may choose another.
- **Program level** (TESTED): I unpacked the real `.deb` files and read the `NEEDED` entries of each binary and library with `readelf -d`. This is what is actually linked, whatever the package metadata says.

### 1.2 Results

| Package | `libsystemd0` | `libudev1` | What the real files link (TESTED) |
|---|---|---|---|
| `openssh-server` | **hard**, through `libpam-modules` | **hard**, through `openssh-client` → `libfido2-1` | `sshd`: **neither** |
| `openssh-client` | hard, through `passwd` → `libpam-modules` | hard, through `libfido2-1` | `ssh`: neither; `ssh-sk-helper` links `libfido2` → `libudev` |
| `tmux` | **hard, direct** | none | `tmux` links `libsystemd.so.0` |
| `dbus-daemon` | **hard, direct** (also `libdbus-1-3`) | none | `dbus-daemon`, `dbus-run-session`, `libdbus-1.so.3` link `libsystemd.so.0` |
| `seatd` | none (`Recommends: libseat1`) | none | `seatd`: libc only |
| `libseat1` | **hard, direct** (`Depends: seatd | logind, libc6, libsystemd0`) | none | `libseat.so.1` links `libsystemd.so.0` (logind backend) |
| `pipewire`, `libpipewire-0.3-modules` | **hard**, through `libpipewire-0.3-modules` | **hard**, through `libspa-0.2-modules` | `pipewire` binary: neither; modules `protocol-native`, `protocol-pulse`, `spa-journal` link libsystemd; `spa-alsa`, `spa-v4l2` link libudev |
| `wireplumber` | hard, through `libwireplumber-0.4-0` | hard, through `libspa-0.2-modules` | `wireplumber` and `libwireplumber-0.4.so.0`: **neither** (the package says "hard", the files I unpacked do not link it; UNKNOWN why) |
| `foot` | none | none | `foot`: neither |
| `waybar` | hard, through `libpulse0` | **hard, direct** | `waybar` links `libudev.so.1`; `libpulsecommon` links libsystemd |
| `wofi` | hard, through GTK 3 → `libatk-bridge` → `libdbus-1-3` | hard, through GTK 3 → `libcolord2` | `wofi`: neither (its libraries do) |
| `virt-viewer` | hard, through `libvte-2.91-0` | hard, through GTK 3 → `libcolord2` | `remote-viewer`: neither; `libvte` links libsystemd |
| `remmina` | same as `virt-viewer` | same | `remmina`: neither |
| `pcmanfm` | hard, through `libfm4t64` → `libdbus-glib` → `libdbus-1-3`; also optional through `Recommends: gvfs-backends` | hard, through GTK 3 → `libcolord2` | `pcmanfm`: neither |
| `libinput10` | none | **hard, direct** (and through `libwacom9` → `libgudev-1.0-0`) | `libinput.so.10`, `libgudev-1.0.so.0` and the helpers in `libinput-bin` link `libudev.so.1` |
| `libwayland-server0`, `libwayland-client0`, `libxkbcommon0`, `libdrm2`, `libgbm1`, `libegl1`, `libgles2`, `libgl1-mesa-dri`, `libpixman-1-0`, `libdisplay-info1` (what a Smithay compositor needs besides libinput and libseat) | none | none | not scanned (no chain in the package graph) |
| `bash`, `coreutils`, `dpkg`, `e2fsprogs`, `mount`, `busybox-static`, `s6`, `execline`, `efibootmgr`, `efivar`, `dosfstools`, `squashfs-tools` | none | none | no chain |
| `apt` | **hard, direct** | hard, through `libapt-pkg6.0t64` | `libapt-pkg.so.6` links both; `apt/methods/http` links libsystemd |
| `procps` | **hard, direct** (also `libproc2-0`) | none | `libproc2.so.0` links libsystemd; `w` links it |
| `util-linux` (Essential) | **hard, direct** | **hard, direct** | `lslogins` → libsystemd; `lsblk`, `findmnt` → libudev |
| `bsdutils` (Essential) | hard | none | `logger` → libsystemd |
| `libpam-modules`, `libpam-modules-bin` (and so `login`, `passwd`, `libpam-runtime`) | **hard, direct** | none | `pam_issue.so`, `pam_timestamp_check` link libsystemd; `login`: neither |
| `zfsutils-linux`, `libzfs4linux` (the NAS) | none | **hard, direct** | not unpacked |
| `qemu-system-x86`, `libvirt-daemon` (the VM host) | hard (through `libbrlapi0.8` or `libpcap0.8t64` → `libdbus-1-3`) | **hard, direct** | not unpacked |
| `hubd` (Go, static) | none | none | `ldd`: not a dynamic executable (`phase-b-image.md`) |

Not in the table because they are not in the archive: Moonlight, Sunshine and driftwm; driftwm's `Cargo.toml` lists the Smithay features `backend_udev`, `backend_libinput` and `backend_session_libseat` ([Cargo.toml lines 14 to 26](https://raw.githubusercontent.com/malbiruk/driftwm/352333a8fa1b22171492d4b71a54102045c9a19d/Cargo.toml), SOURCE), so a built driftwm will link `libudev`, `libinput` and `libseat` (BELIEVED, from those feature names; not rebuilt this time).

### 1.3 Closure view (TESTED, package level)

Following hard `Depends` from two sets of roots (`/tmp/sl/closure.py`):

- **Set A, no GTK** (sshd, tmux, dbus-daemon, seatd, libseat1, pipewire, wireplumber, foot, libinput, the compositor libraries, busybox, s6, execline, efibootmgr, util-linux, mount, e2fsprogs, squashfs-tools): **229 packages**; 15 depend directly on `libsystemd0` (`dbus`, `dbus-daemon`, `libdbus-1-3`, `libpam-modules`, `libpam-modules-bin`, `libpipewire-0.3-modules`, `libproc2-0`, `libpulse0`, `libseat1`, `libspa-0.2-modules`, `libwireplumber-0.4-0`, `procps`, `systemd`, `tmux`, `util-linux`) and 7 on `libudev1` (`libdevmapper1.02.1`, `libfido2-1`, `libgudev-1.0-0`, `libinput-bin`, `libinput10`, `libspa-0.2-modules`, `util-linux`). (`systemd` is in the list because of the trap in 1.4.)
- **Set B, GTK applications** (wofi, virt-viewer, remmina, pcmanfm, waybar): **287 packages**; 13 on `libsystemd0` (adds `libpolkit-gobject-1-0`, `libvte-2.91-0`) and 8 on `libudev1` (adds `libcolord2`, `libusb-1.0-0`, `waybar`).

### 1.4 The trap: apt installs real systemd programs (TESTED)

```
apt-get -s install --no-install-recommends wireplumber   # same for pipewire, wofi, virt-viewer
  -> also installs: systemd systemd-sysv dbus-system-bus-common libpam-systemd dbus-user-session
apt-get -s install --no-install-recommends dbus-daemon | openssh-server | tmux seatd foot   # nothing of the kind
```

The cause is that `wireplumber` (and `dconf-service`, which GTK pulls in) depends on `default-dbus-session-bus | dbus-session-bus`; the "default" one is provided by `dbus-user-session`, which depends on `libpam-systemd` and `systemd`. `dbus-x11` provides `dbus-session-bus` instead. With a pin that forbids the systemd programs, the solver takes `dbus-x11` (TESTED, simulation of the whole set `wireplumber pipewire wofi virt-viewer remmina pcmanfm waybar foot tmux seatd openssh-server dbus-daemon libinput10`):

```
Package: systemd systemd-sysv libpam-systemd dbus-user-session udev systemd-timesyncd systemd-resolved
Pin: release *
Pin-Priority: -1
```

The simulation printed `dbus-x11` and none of the pinned packages. This is a simulation (`apt-get -s`); it was not installed. `mmdebstrap` can take such a preferences file (BELIEVED; not tried), or `dbus-x11` can be listed first.

### 1.5 Hard against optional

- **Hard (package `Depends`):** everything marked "hard" above. Most are "hard" only because Debian/Ubuntu build the package with systemd support switched on (for `sd_notify`, socket activation, the journal, logind). The *program* works without it.
- **Optional (`Recommends`):** `seatd` → `libseat1`; `pcmanfm` → `gvfs-backends` → `gvfs-daemons`; `e2fsprogs` → `libuuid1` → `uuid-runtime` (second pass, TESTED); `openssh-server` → `default-logind | logind | libpam-systemd` (so `libpam-systemd` is a recommendation of sshd, not a dependency).

---

## 2. Does Ubuntu's `sshd` link `libsystemd`?

**No (TESTED).** From the unpacked `openssh-server` 1:9.6p1-3ubuntu13.19:

```
readelf -d usr/sbin/sshd | grep NEEDED
  libcrypt.so.1 libwrap.so.0 libaudit.so.1 libpam.so.0 libselinux.so.1 libgssapi_krb5.so.2
  libkrb5.so.3 libcom_err.so.2 libcrypto.so.3 libz.so.1 libc.so.6
```

On a root with `libsystemd0`, `libudev1`, `libpam-modules`, `procps` and PAM's modules **deleted**, `ldd /usr/sbin/sshd` shows nothing missing, and `sshd -t` passes both with `UsePAM no` and with `UsePAM yes` (a configuration test only; **no login was attempted**). The one binary that does need libudev is `/usr/lib/openssh/ssh-sk-helper` (it links `libfido2`, which links `libudev`); it is only used for FIDO security keys and can be deleted. The package metadata still says `openssh-server` → `libpam-modules` → `libsystemd0`; in an image those files are simply not copied.

Alternatives, for completeness:

| | What | Systemd libraries | Costs |
|---|---|---|---|
| Ubuntu `sshd`, files stripped as above | the real binary, deleting `ssh-sk-helper`, PAM modules and so on | none **linked** (TESTED) | a post-build step that must be kept right; `sshd` still links `libpam.so.0` (the core library, which has no systemd dependency) |
| `sshd` built without PAM/libsystemd | rebuild the Ubuntu source package | none | one more package to rebuild and track (BELIEVED an hour for the first build; not tried) |
| **Dropbear** 2022.83-4 (`dropbear-bin`, in the archive; Debian forky has 2026.94) | small SSH server | none (TESTED: `NEEDED` is `libtomcrypt libtommath libz libcrypt libc`; the `dropbear` binary is 168 KB) | its man page lists public-key login (RSA, DSS, ECDSA, Ed25519), password options and port forwarding switches; **it does not mention an SFTP server** (TESTED: `grep -i sftp` found nothing), so `sftp` and `scp`-over-SFTP would need OpenSSH's `sftp-server` separately: UNKNOWN whether that works with it; no PAM and no GSSAPI (BELIEVED, not tested) |

---

## 3. What can provide `libudev` without systemd

### 3.1 In the archives (TESTED: package indexes read)

| Name | Ubuntu 24.04 noble | Debian trixie | Debian forky |
|---|---|---|---|
| `libudev-zero` | **not there** | **not there** | **not there** |
| `eudev`, `libeudev1` | **not there** | **not there** | **not there** |
| `mdevd` | **not there** | **not there** | **not there** |
| `libudev1`, `udev` (from the systemd source package) | 255.4 | 257.13 | 262 |
| `libudev0` | 200-1 (an old, different ABI) | 200-1+b2 | 200-1+b3 |
| `elogind`, `libelogind0`, `libpam-elogind` | **not there** | 255.17 | 255.17 |
| `busybox`, `busybox-static` (has `mdev`) | 1.36.1 | 1.37.0 | 1.38.0 |
| `dropbear` | 2022.83 | 2025.89 | 2026.94 |

All three non-systemd udev options therefore have to be **built from source** by us. (Devuan, Alpine and Void ship them; not checked here.)

### 3.2 What each is, and how it is meant to be used (SOURCE)

- **`libudev-zero`** ([README](https://raw.githubusercontent.com/illiliti/libudev-zero/master/README.md)): a "mostly ABI/API compatible libudev alternative that works with any device manager". It has no dependencies beyond libc, can be linked statically, and notes that some legacy functions (the `udev_queue` API) are not implemented. For hotplug it re-broadcasts netlink uevents to a netlink group, which needs help from a device manager (the README names `mdevd` as one that does it by design: `mdevd -O 0x4`) or from a helper.
- **`mdevd`** ([skarnet.org/software/mdevd](https://skarnet.org/software/mdevd/)): a daemon that manages kernel hotplug events, using the same configuration file as busybox `mdev`; it does not fork per event. It is a device *manager*, not a libudev.
- **`eudev`** ([README](https://raw.githubusercontent.com/eudev-project/eudev/master/README.md)): "a standalone dynamic and persistent device naming support (aka userspace devfs) daemon that runs independently from the init system"; a fork of udev from systemd to isolate it from systemd; started by Gentoo developers, now run by Alpine, Devuan and Gentoo contributors; it provides both the daemon and a `libudev`.
- **`elogind`** is logind split out of systemd (Debian has it). It is **not** a libudev replacement and is part of the systemd source tree's lineage; `HUB-OS.md` does not need it because `seatd` does the seat job (SOURCE: [seatd README](https://raw.githubusercontent.com/kennylevinsen/seatd/master/README.md): seatd "joins the ranks of logind and direct session management" for compositors such as sway).

### 3.3 Does it work with libinput and a Smithay compositor?

- **Symbols (TESTED).** I built `libudev-zero` from the upstream `master` source files (Makefile, five `.c` files, two headers; no tarball, because GitHub archives return 403 here): 1 s, 42,352 bytes, soname `libudev.so.1`, no dependency but libc. It exports **80** function names; systemd's `libudev.so.1` exports **92**. The 17 names that are missing from `libudev-zero` are `udev_device_has_current_tag`, `udev_get_userdata`, `udev_set_userdata`, `udev_util_encode_string` and thirteen `udev_queue_*` functions. For each library below I listed the `udev_*` functions it imports (`nm -D --undefined-only`) and compared:

  | Consumer | `udev_*` functions used | Missing from libudev-zero |
  |---|---|---|
  | `libinput.so.10` (1.25) | 29 | none |
  | `libgudev-1.0.so.0` (libinput's `libwacom` uses it) | 51 | none |
  | `waybar` | 19 | none |
  | `libapt-pkg.so.6` | 12 | none |
  | PipeWire `libspa-alsa.so` | 22 | none |
  | PipeWire `libspa-v4l2.so` | 24 | none |
  | `lsblk` | 8 | none |
  | `findmnt` | 5 | none |

- **Run (TESTED, one case).** In a copy of the base root I replaced `libudev.so.1` by the `libudev-zero` build. `lsblk -o NAME,SIZE,TYPE` and `findmnt` then ran and printed the correct devices and mounts. Both print `libudev.so.1: no version information available (required by ...)`: the Ubuntu binaries ask for symbol versions (`@LIBUDEV_183`) that `libudev-zero` does not define. It is only a warning; distributions that use `libudev-zero` rebuild their packages against it instead (BELIEVED, not checked).
- **Input devices without udev rules (SOURCE).** `libudev-zero` fills in the properties itself: `udev_device.c` sets `ID_INPUT`, `ID_INPUT_TOUCHSCREEN`, `ID_INPUT_TABLET` and others from sysfs (lines 447 to 476 of the file I built). The real udev does it with rule files, which are absent without a udev daemon.
- **libinput and driftwm with it: UNKNOWN.** I looked in the `libudev-zero` README (it names no consumer) and in driftwm's `Cargo.toml`. I found **no statement** that libinput, Smithay or driftwm are known to work with `libudev-zero`, `eudev` or `mdevd`, and I could not search the project's issue trackers from here (GitHub API is blocked). The symbol result above is necessary, not sufficient. A real test needs input devices and a compositor under a virtual GPU, which this environment lacks.
- **Hotplug: UNKNOWN.** Without `udevd`, devices come from `devtmpfs` (the kernel creates nodes) and hotplug events need `mdevd` or `eudev`; whether a Hub OS image needs hotplug (keyboards unplugged and plugged) was not tested.

---

## 4. What deleting `apt`, `dpkg`, `procps` and PAM from an image would break

Base: the 96-package Ubuntu root of the prototype (`debootstrap --variant=minbase --include=s6,execline,busybox-static,libc6 --components=main,universe noble`, 59 s), TESTED.

### 4.1 Who depends on them (installed packages only)

| Package | Installed packages that depend on it | Essential? |
|---|---|---|
| `apt` | `libapt-pkg6.0t64` (and nothing else) | no |
| `procps` | `libproc2-0` | no |
| `dpkg` | `perl-base`, `libapt-pkg6.0t64`, `dash`, `base-files` | **yes** (Essential) |
| `login` | none (`login` is Essential itself) | **yes** |
| `passwd` | none | no |
| `libpam-modules`, `libpam-modules-bin`, `libpam-runtime` | `login`, `passwd`, `libpam-runtime` | no |
| `libpam0g` | `util-linux`, `passwd`, `login`, `libcrypt1`, the PAM packages | no |
| `util-linux` | (Essential; provides `mount`'s companions, `findfs`, `blkid`, `losetup`, `switch_root`, `su`, `lsblk`, `findmnt`, `lslogins`) | **yes** |

The Essential list of the base is: `base-files base-passwd bash bsdutils coreutils dash debianutils diffutils dpkg findutils grep gzip hostname init-system-helpers libc-bin login ncurses-base ncurses-bin perl-base sed sysvinit-utils tar util-linux`. Essential packages cannot be removed with `apt`; but an *image* is not managed by `dpkg`/`apt` after it is built, so the build can delete their **files** (leaving a `dpkg` status file that no longer matches, which does not matter for an immutable image).

### 4.2 What actually breaks (TESTED, link level)

A script deleted the files of the listed packages from a copy of the root (`dpkg -L PACKAGE`) and then ran `ldd` on every ELF file left in `/usr`, `/bin`, `/sbin` and `/lib`, printing any "not found". The checker works: deleting `libseccomp2` makes `/usr/lib/apt/methods/*` report `libseccomp.so.2 => not found`.

| Step | Removed | Unresolved libraries afterwards |
|---|---|---|
| A | `apt`, `libapt-pkg6.0t64`, `procps`, `libproc2-0`, `libpam-runtime`, `libpam-modules`, `libpam-modules-bin`, `login`, `passwd` | **none** |
| A, what still links the two libraries | | `lslogins` (util-linux) → `libsystemd`; `lsblk`, `findmnt` (util-linux) → `libudev`; `logger` (bsdutils) → `libsystemd` |
| B | additionally the four binaries `lslogins`, `lsblk`, `findmnt`, `logger`, then the packages `libsystemd0` and `libudev1` | **none**; no file of either library is left in the root |

This is only the **dynamic-linking check**: nothing was run in the stripped root beyond what is stated for `sshd -t` in section 2. `libpam0g` (the core PAM library) stays because `util-linux` (`su`) and `sshd` link it; it has no systemd dependency.

### 4.3 What replaces them

Busybox 1.36.1 in the image has these applets (TESTED, `busybox --list`): `ps`, `kill`, `pidof`, `login`, `su`, `passwd`, `findfs`, `mount`, `umount`, `losetup`, `switch_root`, `sha256sum`, `dd`, `head`, `stat`, `sed`, `awk`, `mkdir`, `hostname`, `reboot`. It has **no** `pgrep` and **no** `blkid`. The prototype's own scripts use `findfs PARTLABEL=...` (busybox's `findfs` may not understand `PARTLABEL=`: UNKNOWN, not tested; stage 0 avoids the question by reading sysfs `PARTNAME=`), `mount`, `sha256sum`, `dd`, `stat`, `sed`, `awk`, `hostname`, `reboot`, `kill`, `ps`. A stripped image would take `login`/`su`/`passwd` and `ps` from busybox; **the `login`/`passwd` pair without PAM is only meaningful if nobody needs PAM's features** (account lockout, password rules, `pam_limits`; none are used by the prototype). `sshd` keeps its dependency on `libpam.so.0` but, with `UsePAM no`, needs no PAM *modules* (BELIEVED; section 2 tested only the configuration check).

**What would break in real use (BELIEVED, not tested):** nothing can install or upgrade packages on a node (the point of an immutable image); `dpkg -l`-style inventory tools stop working (the status file is stale); scripts that call `pgrep` or `blkid` need the real tools or a rewrite.

---

## 5. The options

The owner's rule: "no systemd, ever": (a) **no systemd program installed**, (b) **none running**, (c) **no systemd library**.

| | **1. Accept the two libraries** | **2. Strip what can be stripped, replace the rest with non-systemd libraries** | **3. Rebuild packages ourselves** |
|---|---|---|---|
| What it is | Keep `libsystemd0` and `libudev1` in the images as plain libraries. Block the systemd *programs* with the apt pin of 1.4 and test for them. | Delete `apt`, PAM, `procps`, `ssh-sk-helper`, four `util-linux`/`bsdutils` tools in every image (TESTED to link cleanly). Replace `libudev1` by a `libudev-zero` build where an image needs the udev API. Images that still contain a package with a systemd link keep `libsystemd0`. | Rebuild each package that links a systemd library, with the systemd option switched off, from Ubuntu's source package (or upstream), and keep rebuilding it when Ubuntu issues a security update. |
| Cost | none (this is what the prototype does) | low for headless roles (see below); a build step plus a test; `libudev-zero` is one more thing to build and track (1 s to build) | each package: fetch source, change the build options, rebuild, test, track updates. Measured: `tmux` 6 s, `seatd` 2 s, `libudev-zero` 1 s. **Not measured:** `dbus`, PipeWire, WirePlumber, GTK-related libraries (`libcolord`, `libvte`, `polkit`), PulseAudio's client library, `waybar` |
| Risk | the rule is read as "no systemd code at all" and the images contain code built from the systemd source package (it does not start or do anything) | **hub image:** the GTK, D-Bus and PipeWire stacks still link `libsystemd0` and `libudev1`, so option 2 alone does not remove them there; a `libudev-zero` swap gives the "no version information" warning and is untested with `libinput` in a real compositor (UNKNOWN) | we carry patches and a rebuild pipeline for several packages; a missed security update is ours; unknown build problems (not tried for the big ones); breaking a desktop component in ways only hardware shows |
| (a) no systemd program installed | **true**, if the pin and the test are kept (without them it is **false** for any image with PipeWire, WirePlumber, wofi or virt-viewer: TESTED 1.4) | true (same pin) | true |
| (b) none running | true (nothing starts it: s6 is the init) | true | true |
| (c) no systemd library | **false**: two libraries remain | **true for headless role images** after the strip, if `tmux` is rebuilt (`sshd`, busybox, s6 need nothing; `zfsutils-linux` and QEMU/libvirt need `libudev1`: `libudev-zero` untested with them). **False for the hub** until its desktop stack is rebuilt | true everywhere, at the price above |

**Where each role stands (BELIEVED, from sections 1 and 4; not built):**

- *Hub:* needs PipeWire, WirePlumber, `dbus-daemon`, `libseat`, `libinput`, GTK (wofi, virt-viewer), Waybar, driftwm: option 1, or option 3 for about eight source packages (`dbus`, `pipewire`, `wireplumber`, `libpulse`, `libvte`, `polkit`, `libcolord`/`libgudev` via a `libudev-zero` swap, `waybar`).
- *NAS and backup NAS:* `sshd`, `tmux`, busybox, s6, OpenZFS userland (`zfsutils-linux` depends directly on `libudev1`): option 2 plus `libudev-zero`, or option 1.
- *VM host:* adds QEMU and libvirt, which depend on `libudev1` directly and on `libsystemd0` through `libdbus-1-3`, `libbrlapi0.8`: the same.
- *Guests:* depend on what runs in them; BELIEVED the cheapest case.

### Recommendation

**Option 1 now, with the program part enforced; option 2 for the headless role images when they are built; decide option 3 for the hub when the desktop slice starts.**

Why: the two libraries do nothing without a program that calls them; what the rule protects (no systemd init, no systemd service manager, no logind, no journal) is protected by the pin and a test, which is cheap and which I could verify. Option 3 for the hub is real work (about eight packages, none of them built here, with security updates for each) and I cannot say today whether the desktop software works the same without those libraries; the decision is better taken when there is a running desktop to test against. Option 2 is almost free for headless roles and shrinks every image (apt, PAM and procps are useless in an immutable image anyway). The honest answer to "does the rule hold" is: **(a) and (b) yes, (c) no, until option 2 or 3 is carried out.**

---

## 6. What this does not cover

- No package was built except `tmux` (upstream 3.4 source), `seatd` 0.8.0 (Ubuntu source tarball) and `libudev-zero` (upstream master source files). Nothing was installed; the `apt` runs were simulations (`-s`).
- No real login over `sshd`, no run of any desktop program, no input devices, no GPU, no hotplug.
- `libudev-zero` was not compared with `eudev` or `mdevd`; those were only read about. Their versions in other distributions, and how they behave with `zfs`, `libvirt` or `qemu`, are UNKNOWN.
- Ubuntu 24.04 package versions change; the dependency results are for the index of 2026-10-03.

---

## 7. Questions for the owner

1. Does "no systemd, ever" cover **libraries** built from the systemd source package (`libsystemd0`, `libudev1`)? (If no: option 1 is enough. If yes: which roles first?)
2. Do you want the apt pin (and a build test that fails if `systemd`, `systemd-sysv`, `libpam-systemd`, `dbus-user-session` or `udev` is in an image) in the image build? I recommend yes.
3. Do you accept that images do not contain `apt`, `dpkg`-based tools, PAM modules or `procps` (deleted after the build)? I recommend yes.
4. For the NAS and VM host: is building `libudev-zero` (and a device manager such as `mdevd` if hotplug is needed) acceptable, given that nobody has shown OpenZFS or QEMU working with it?
5. For the hub: do you want me to try option 3 on the desktop stack in the desktop slice (a virtual-GPU test is not available here), or stay on option 1?

---

## 8. What I tried (exact commands)

Working directory `/tmp/sl` (deleted at the end). Unpacking and index handling as in `docs/environment.md` 1.8:

```
A=/tmp/sl; mkdir -p $A/apt/lists/partial $A/apt/cache/archives/partial $A/debs $A/root
O="-o Dir::State::lists=$A/apt/lists -o Dir::Cache=$A/apt/cache -o APT::Sandbox::User=root -o Dir::State::status=/dev/null"
apt-get $O update
apt-cache $O dumpavail > avail.txt                       # 87,145 package records
apt-get $O download PKG                                  # then: dpkg -x PKG*.deb $A/root
```

Package-level dependency chains (`graph.py` parses `avail.txt`, follows `Depends` and `Pre-Depends`, then adds `Recommends`; prints the shortest chain to the goal):

```
python3 graph.py libsystemd0 openssh-server tmux dbus-daemon seatd libseat1 pipewire wireplumber foot waybar wofi \
  virt-viewer remmina libinput10 pcmanfm libwayland-server0 libxkbcommon0 libdrm2 libgbm1 libegl1 libgles2 \
  libgl1-mesa-dri libpixman-1-0 libdisplay-info1 apt dpkg bash coreutils util-linux mount procps login passwd \
  libpam-modules libpam-modules-bin libpam-runtime e2fsprogs busybox-static s6 execline efibootmgr efivar \
  dosfstools squashfs-tools libblkid1 libmount1 libudev1          # and the same with the goal libudev1
python3 closure.py                                                 # the two closures of 1.3
```

Direct links in the real files:

```
apt-get $O download openssh-server openssh-client openssh-sftp-server tmux dbus-daemon dbus-bin seatd libseat1 pipewire \
  libpipewire-0.3-0t64 libpipewire-0.3-modules libspa-0.2-modules wireplumber foot waybar wofi virt-viewer remmina \
  libinput10 libinput-bin pcmanfm libfm4t64 dropbear-bin libpam-modules libpam-modules-bin procps libproc2-0 apt \
  libapt-pkg6.0t64 util-linux login passwd libdbus-1-3 libpulse0 libvte-2.91-0 libgtk-3-0t64 pipewire-bin \
  libwireplumber-0.4-0 libwacom9 libgudev-1.0-0 libevdev2 libmtdev1t64
for d in *.deb; do dpkg -x $d $A/root; done
readelf -d root/usr/sbin/sshd | grep NEEDED                       # section 2
for f in $(find root -type f); do readelf -d $f | grep NEEDED | grep -oE "lib(systemd|udev)\.so\.[0-9]" ...; done   # the table of 1.2
```

The apt trap:

```
S="-o Dir::State::status=/tmp/sl/rootfs/var/lib/dpkg/status"      # the prototype-like base root's package list
apt-get $O $S -s install --no-install-recommends wireplumber       # also: systemd systemd-sysv dbus-system-bus-common libpam-systemd dbus-user-session
apt-get $O $S -o Dir::Etc::preferences=/tmp/sl/prefs -s install --no-install-recommends wireplumber pipewire wofi virt-viewer \
  remmina pcmanfm waybar foot tmux seatd openssh-server dbus-daemon libinput10                            # prints dbus-x11 only
```

The base root and the stripping:

```
export DEBOOTSTRAP_DIR=/tmp/sl/tools/usr/share/debootstrap; PATH=/tmp/sl/tools/usr/sbin:$PATH
debootstrap --variant=minbase --include=s6,execline,busybox-static,libc6 --components=main,universe noble /tmp/sl/rootfs http://archive.ubuntu.com/ubuntu   # 59 s, 96 packages
chroot rootfs dpkg-query -W -f='${Package} ${Essential}\n'; chroot rootfs apt-cache rdepends --installed X          # 4.1
./strip.sh rootfs-a apt libapt-pkg6.0t64 procps libproc2-0 libpam-runtime libpam-modules libpam-modules-bin login passwd   # step A: no unresolved
rm rootfs-a/usr/bin/{lslogins,lsblk,findmnt,logger}; ./strip.sh rootfs-a libsystemd0 libudev1                              # step B: no unresolved
./strip.sh rootfs-t libseccomp2                                                                                           # the checker's sanity test
```

(`strip.sh` removes `dpkg -L PACKAGE` files, then runs `ldd` over every ELF in `/usr /bin /sbin /lib` inside the chroot and prints lines with "not found".)

sshd and Dropbear on the stripped root:

```
apt-get $O $S install --print-uris --no-install-recommends openssh-server        # 15 packages; unpacked except PAM, procps, systemd libs
chroot rootfs-b ldd /usr/sbin/sshd | grep -E "not found|systemd|udev"           # nothing
chroot rootfs-b /usr/sbin/sshd -t -f /etc/ssh/sshd_config                        # "UsePAM no": OK; "UsePAM yes" with no PAM modules: OK
chroot rootfs-b ldd /usr/lib/openssh/ssh-sk-helper | grep udev                    # libudev.so.1 => not found
readelf -d dbx/usr/sbin/dropbear | grep NEEDED                                    # libtomcrypt libtommath libz libcrypt libc
zcat dbx/usr/share/man/man8/dropbear.8.gz | grep -i sftp                          # no match
```

Rebuilds:

```
tmux 3.4 (archive.ubuntu.com/ubuntu/pool/main/t/tmux/tmux_3.4.orig.tar.gz): ./configure && make -j4   # 6 s; NEEDED: libevent_core libm libresolv libc
seatd 0.8.0 (pool/universe/s/seatd/seatd_0.8.0.orig.tar.gz): meson setup build -Dlibseat-logind=disabled -Dlibseat-builtin=disabled
  -Dserver=enabled -Dman-pages=disabled && compile    # 2 s; libseat.so.1 and seatd NEEDED: libc only
libudev-zero (raw.githubusercontent.com/illiliti/libudev-zero/master: Makefile udev*.c udev*.h libudev.pc.in): make    # 1 s, libudev.so.1 42,352 bytes
nm -D --defined-only libudev.so.1 | awk '$2=="T"{print $3}' | grep ^udev_ | sort    # 80 names (systemd's: 92)
nm -D --undefined-only CONSUMER | awk '{print $2}' | sed 's/@.*//' | grep ^udev_ | sort -u    # compared with comm -23
cp uz/libudev.so.1 rootfs-z/usr/lib/x86_64-linux-gnu/libudev.so.1.7.8; mount proc, sys, dev; chroot rootfs-z lsblk; findmnt    # worked, with the "no version information" warning
```

Sources read: [libudev-zero README](https://raw.githubusercontent.com/illiliti/libudev-zero/master/README.md), [eudev README](https://raw.githubusercontent.com/eudev-project/eudev/master/README.md), [mdevd](https://skarnet.org/software/mdevd/), [seatd README](https://raw.githubusercontent.com/kennylevinsen/seatd/master/README.md) and [its `meson_options.txt`](https://raw.githubusercontent.com/kennylevinsen/seatd/master/meson_options.txt) (option `libseat-logind`: `auto`, `disabled`, `elogind` or `systemd`), [PipeWire `meson_options.txt`](https://gitlab.freedesktop.org/pipewire/pipewire/-/raw/master/meson_options.txt) (options `libsystemd` "Enable code that depends on libsystemd" and `logind` with provider `libelogind` or `libsystemd`), [dbus 1.14 `configure.ac`](https://gitlab.freedesktop.org/dbus/dbus/-/raw/dbus-1.14/configure.ac) (`--enable-systemd`, default `auto`), [Waybar `meson.build`](https://raw.githubusercontent.com/Alexays/Waybar/master/meson.build) (feature options `libudev` and `systemd`).
