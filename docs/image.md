# The Phase B image: build and test (slices S0 to S3, per-slot kernels)

**Status:** built and tested in the build environment (`docs/environment.md`): QEMU in software emulation, **no KVM**, no real hardware. Everything here follows `docs/proposals/phase-b-image.md` and the owner's answers recorded in it ("Owner decisions (2026-10-02)"). Times are emulation times on 4 CPUs and say nothing about real machines. `HUB-OS.md` wins if anything here disagrees with it.

Labels: **TESTED** (the command is in the runner or shown below), **BELIEVED**, **UNKNOWN**.

---

## 1. What is in the repo

| Path | What |
|---|---|
| `image/machines/qemu-test.build` | the first machine's build configuration, shell-readable `KEY=VALUE`: `KERNEL_VERSION` (6.12), `KERNEL_FRAGMENT`, `MODULES` (empty), `PACKAGES`, `SNAPSHOT`, `SERVICES`, `CONFIRM_TIMEOUT`. The scripts read the kernel version, the fragment and the package list from this file; nothing is hard-wired. |
| `image/kernel/qemu-test.frag` | kernel configuration fragment, appended to `make tinyconfig` (no modules). Its built-in command line has `@SLOT@` in it: the kernel is built **once per slot** (section 3.4) |
| `image/apt/no-systemd.pref` | the apt pin that keeps `systemd`, `systemd-sysv`, `libpam-systemd`, `dbus-user-session`, `udev`, `systemd-timesyncd` and `systemd-resolved` from ever being installed (from `docs/proposals/systemd-libraries.md` section 1.4); copied into the root before the first package is installed |
| `image/packages/qemu-test.list` | the package list of the root: `s6`, `execline`, `busybox-static`, `libc6` (the prototype's list) |
| `image/stage0/` | stage 0, the program built into the kernel (`init`, `stage0.list.in`) |
| `image/rootfs/` | files copied into the root: `/sbin/init`, `/usr/sbin/hubos-ctl` (boot entries, update, confirm), the `udhcpc` script, the s6 service directories (`hubd`, `console`, `confirm`, `wd`) |
| `image/config/qemu-test/` | the config partition's `inventory.toml` and `viewers.toml` (copies of the examples) |
| `tools/image/*.sh` | `fetch-tools.sh` (unpack QEMU and friends into a temporary directory), `build-kernel.sh`, `build-base.sh` (mmdebstrap), `build-root-image.sh`, `build-disk.sh`, `build-bundle.sh`; `strip-root.sh` (deletes apt, PAM modules, procps, login, passwd files from a finished root) and `check-libs.sh` (`ldd` inside the root); `common.sh` is shared |
| `tools/image/qemu_test.go` | the test runner (Go, build tag `qemu`) |

**No key is in the repo.** The runner makes two throwaway signing key pairs (`signify-openbsd -G -n`) in a temporary directory at run time and builds every bundle with them. The public key of the first pair is built **into the test images only** (`/etc/hubos/update.pub`); the second pair is the "wrong key". Both are deleted with the temporary directory.

---

## 2. How to build and run

Needs: Linux, root (QEMU runs without KVM; `mmdebstrap` runs in `--mode=root`), Go, `gcc`, `make`, `flex` and friends are unpacked by the first script. **Nothing is installed**: `fetch-tools.sh` downloads the packages and unpacks them into `$WORK/tools/root` with `dpkg -x`. Disk: about 5 GB in `$WORK`.

```
# the normal tests (do NOT run the image build):
go test -count=1 ./...

# the image tests (build tag "qemu"); everything below happens inside one temporary directory:
go test -tags qemu -count=1 -timeout 150m -v ./tools/image

# THE QEMU TESTS ARE RUN BY HAND before merging any change to image/ or tools/image/ (they are not part of
# `go test ./...`, they take 12 to 20 minutes, and they need root).

# keep the long build between runs (the kernel and the root are rebuilt only when their inputs change):
HUBOS_IMAGE_WORK=/tmp/hubos-image go test -tags qemu -count=1 -timeout 150m -v ./tools/image
```

The scripts can also be run by hand (`WORK=/tmp/x tools/image/fetch-tools.sh`, then `build-kernel.sh`, `build-base.sh`, `build-bundle.sh 2 good DIR PUBKEY SECKEY`, `build-disk.sh ROOT KERNEL DIR`). `MACHINE=image/machines/other.build` selects another machine file.

**How the image is built:**
1. `build-kernel.sh`: downloads `linux-$KERNEL_VERSION`, `make tinyconfig`, appends the machine's fragment (with the stage 0 list and slot `a` filled in), builds the `bzImage` as `kernel-a.efi`; then changes only the slot name in the built-in command line (`hubos-root-a hubos.slot=a` to `hubos-root-b hubos.slot=b`) and builds again as `kernel-b.efi` (the second build recompiles only what depends on the command line). `KBUILD_BUILD_TIMESTAMP`, user and host are fixed so that two builds can be compared.
2. `build-base.sh`: `mmdebstrap --mode=root --variant=minbase --components=main,universe noble` with the machine's package list, from **`http://snapshot.ubuntu.com/ubuntu/$SNAPSHOT`** (a dated snapshot of the archive), with `SOURCE_DATE_EPOCH` set and, through a setup hook, `image/apt/no-systemd.pref` placed in `/etc/apt/preferences.d/` before the first package is installed. Output: a tar file.
3. `build-root-image.sh`: unpacks the base, copies `image/rootfs`, the static `hubd` (`CGO_ENABLED=0 go build -trimpath`), `efibootmgr` and `signify`, the public key, writes `/etc/hubos-release`, **removes** the unit files and rules packages ship for systemd and udev (`/etc/systemd`, `/usr/lib/systemd`, `/var/lib/systemd`, `/usr/lib/udev`: inert, from `e2fsprogs`, `util-linux`, `dpkg`), then **strips** the root (`strip-root.sh`: the files of `apt`, `libapt-pkg6.0t64`, `procps`, `libproc2-0`, `libpam-runtime`, `libpam-modules`, `libpam-modules-bin`, `login` and `passwd`, taken from dpkg's own file lists; busybox supplies `ps` and `pidof`), runs `check-libs.sh` (**the build fails** if `ldd`, run inside the root, finds an unresolved library in any ELF file under `/usr`, `/bin`, `/sbin` or `/lib`), and packs a squashfs (zstd; `SOURCE_DATE_EPOCH` fixes every time stamp). Bad flavors for the tests: `noinit`, `garbage`, `unhealthy`, `hang`.
4. `build-bundle.sh`: root image + **both kernels** (`kernel-a.efi`, `kernel-b.efi`) + a manifest (`version`, `flavor`, `arch`, **`kernel-version`**, **`modules`**, sha256 and size of the root and of **both kernels**) signed with `signify`.
5. `build-disk.sh`: a 420 MiB GPT disk (boot, slot A, slot B, config, data), partitions made as separate files and put in place with `dd` (no loop devices), plus a writable copy of OVMF's variable store.

---

## 3. The tests (`tools/image/qemu_test.go`)

One Go test, `TestImage`, with sub-tests in the order below; they share one disk. The runner drives QEMU (`q35`, OVMF, 2 vCPUs, 1 GB, virtio disk and network, an `i6300esb` watchdog) over the serial console, and serves the bundles to the guest from an HTTP server on the host (the guest sees the host as `10.0.2.2`).

| # | Proves | How | Result (run A / run B; both fresh, all PASS, 0 hangs) |
|---|---|---|---|
| (files) | the built root has **no systemd program, no unit file or unit directory, none of the seven banned packages installed, and the apt pin file** in it | the unpacked squashfs is searched; dpkg's status file is read; the pin file is compared with `image/apt/no-systemd.pref` | PASS 1.1 / 0.9 s. Remaining: the two libraries (below) and `deb-systemd-helper`, `deb-systemd-invoke` (Perl scripts of the Essential package `init-system-helpers`; not systemd programs) |
| (files) | the **stripped** root has no file of `apt`, `libapt-pkg6.0t64`, `procps`, `libproc2-0`, `libpam-runtime`, `libpam-modules`, `libpam-modules-bin`, `login`, `passwd`, and still has the two libraries | dpkg's file lists against the root | PASS (apt and libapt are not installed in this root at all) |
| (files) | **no unresolved library** (`ldd` inside the root, `/usr`, `/bin`, `/sbin`, `/lib`) and the check **can fail** | `check-libs.sh`; then `libmount.so.1` is deleted from the copy and the check must report it | PASS 11.4 / 11.2 s; 714 ELF files checked |
| 1 | the root is read-only | `touch /usr/x`, `touch /etc/x`, a write to `/var`, the root mount | PASS (11.7 / 11.9 s including the first boot) |
| 2 | first boot sets up the boot entries; `hubd` runs from the config partition; PID 1 and the process list have **no systemd** | `hubd check --inventory /config/hubos/inventory.toml`, `hubd list`, `cat /proc/1/comm`, `ps`; then `/proc/*/comm` is counted for names starting with `systemd` | PASS (8.6 / 8.4 s); **0 processes named systemd** |
| A1 | the **a and b boot entries have no load options**, and slot a boots with its own kernel | `efibootmgr -v` (each line ends at `File(\EFI\hubos\kernel-a.efi)`; the recovery line has the load options as hex after the path), `/proc/cmdline` | PASS: `console=ttyS0 ro rootfstype=squashfs root=PARTLABEL=hubos-root-a hubos.slot=a panic=5 loglevel=4 i6300esb.heartbeat=60` |
| 3 | init restarts a killed service | `kill -9` the `hubd` pid, wait, `s6-svstat`, `hubd list` | PASS (2.2 s) |
| 4 | unsigned, tampered manifest, wrong key, tampered root, not-newer, and a signed bundle with no `kernel-version` line are all refused, with their own messages, exit code 2, and `BootNext` unset | six bundles | PASS (4.5 / 4.6 s) |
| 5 | a **signed update is accepted**; the config marker survives; the floor is raised; the old bundle is refused; the kernel version in the manifest is shown | update, reboot, `hubos-ctl status`, the v1 bundle again | PASS (24.4 / 28.6 s: update 5.6 / 7.5 s, reboot to handover 12.6 / 15.0 s) |
| A1 (slot b) | **slot b boots with its own kernel** (installed by the update; no load options) | `/proc/cmdline` in slot b | PASS: `... root=PARTLABEL=hubos-root-b hubos.slot=b ...`, `BootCurrent` is the `hubos-b` entry |
| 6a to 6d | four bad boots roll back by themselves: no `/sbin/init` (stage 0), a garbage root (stage 0), a boot that never gets healthy (confirm timeout 40 s), an init that hangs (watchdog, 60 s) | each bundle is correctly signed; the runner waits for the failure line, then for the confirmed slot to come back | PASS: rollbacks complete after 26.8 / 28.1, 39.7 / 26.0, 105.1 / 93.7 and 88.2 / 109.5 s |
| 7 | recovery mode is a boot option | `efibootmgr -n` the recovery entry, reboot | PASS (10.2 / 12.1 s): `RECOVERY MODE` banner, `hubos.recovery=1`, read-only root |
| A2 | **the recovery entry without load options boots a normal slot** | a copy of the recovery entry is made with no `-u`, `BootNext` points at it, reboot | PASS (18.1 / 22.2 s). It printed: `STAGE0: switching to slot b`, `HUBOS: slot=b release: version=2 ...`, then `hubos-ctl: confirm: BootCurrent 000A is not slot b's entry (0008); not confirming`. No `RECOVERY MODE` banner, `/proc/cmdline` without `hubos.recovery`, services (`confirm console hubd wd`) running. So such a firmware gives a **working normal system that never confirms itself**, and no recovery. |
| 8 | an update **interrupted at known log lines** never harms the confirmed slot, and an update still works afterwards | see 3.2 | PASS (116.4 / 116.3 s) |
| B | an update with a **full boot partition** fails cleanly; the old kernel is untouched and still boots | the partition is filled with a file until `df` shows 2 free blocks; `hubos-ctl update` of a newer bundle; hashes of `kernel-b.efi` and of slot b's root before and after; `BootNext` to slot b and reboot | PASS (39.1 / 45.6 s): `FAILED: not enough free space on the boot partition (have 2048 bytes, need 5166080); nothing was written, ...`, exit 2, no `.new` file, no `BootNext`, hashes equal, slot b booted release 2 with `hubos.slot=b`. **Side effect to know about:** that last boot makes slot b the confirmed slot and **lowers the floor to 2** (see section 8). |
| 9 | reproducible builds: two builds of the root tar give identical hashes (asserted); the squashfs and **both kernels** are reported | `build-base.sh` twice; `build-root-image.sh` twice; the kernels built again from scratch | PASS (216.7 / 219.1 s): tar identical, squashfs identical, kernel a and kernel b identical |

### 3.1 Kernel pinning

The manifest carries `kernel-version 6.12` (from the machine file) and a `modules` line (empty). `hubos-ctl update` prints both (`update: kernel version 6.12, modules:`) and **refuses a bundle whose signed manifest has no `kernel-version` line** (`REFUSED: manifest has no kernel version line`; test 4, bundle `v2-nokernelversion`). The module list is only printed in this slice: there are no modules to build or check. The rule in `HUB-OS.md` (out-of-tree modules are listed in the signed manifest and covered by the boot tests) has no test yet beyond that.

### 3.2 The interrupted update (test 8)

The update tool prints a line at each step, and the runner waits for a **log line** and then sends `SIGKILL` to QEMU:

| Interrupted at the line | Meaning |
|---|---|
| `update: fetching rootfs.sqsh` | during the download |
| `update: wrote 8 of N MiB to slot a` | during the write to the other slot (the write is in 4 MiB pieces with a line after each) |
| `update: root written and verified` | after the write, before the kernel is copied |
| `update: kernel installed` | after the kernel copy, before `BootNext` is set |

After each kill QEMU is started again on the same disk and variable store. Every time the machine booted the confirmed slot (`slot=b release=2`, first in `BootOrder`, no `BootNext`). Then a clean update to release 7 was made: the machine rebooted into slot A, release 7, and confirmed it.

**What this does not show:** `kill -9` of QEMU stops the guest; the host still has everything the guest already wrote (QEMU's cache), so it tests the update's **order of steps**, not what a real disk does when the power fails mid-write.

### 3.3 The unexplained hang

The prototype saw one start in about twelve stop silently after `BdsDxe: starting Boot0001`. The runner waits for a known line (`HUBOS: handing over to s6-svscan`, 150 s) after every start and every reboot. **Rule:** if the line does not come, the hang is recorded (its number, the step it happened in and the saved serial log in `$WORK/hangs/`), QEMU is killed and that step is retried **once**; **a second hang in the same step fails the run**. At the end the runner prints `QEMU hangs seen and retried: N; every hang: ...` with every hang listed. The hang-and-retry code was **not exercised** (no hang happened in any run), so it is untested; the cause of the earlier hang stays UNKNOWN, and not seeing it does not mean it is gone.

### 3.4 One kernel per slot (the slot does not depend on the firmware's load options)

Before: each boot entry carried its command line as load options, while the kernel's built-in command line named slot a. A firmware that drops load options would have booted slot b's kernel with slot a's root. Now:

- `build-kernel.sh` builds **two kernels**; each has its own slot and root built in: `kernel-a.efi` (`root=PARTLABEL=hubos-root-a hubos.slot=a`) and `kernel-b.efi` (`...-b ... hubos.slot=b`). Everything else is identical; both files are 4,117,504 bytes and their hashes differ.
- The bundle carries both; the signed manifest lists both with hash and size. `hubos-ctl update` refuses a manifest that does not list both, but **fetches and installs only the other slot's kernel** (`kernel-$OTHER.efi`).
- The **a and b boot entries have no load options**. Only the **recovery** entry has load options (`hubos.recovery=1`). **A firmware that drops load options would therefore boot a normal slot instead of recovery**; the recovery boot option would silently stop working while everything else still boots. Test A2 shows exactly what happens (below). Whether a real firmware does this: UNKNOWN (OVMF keeps them).
- A machine whose entries were made by an earlier image (with load options) keeps those entries; `ensure-entries` only creates missing entries. No such machine exists yet.

### 3.5 Free space on the boot partition

`hubos-ctl update` now mounts the boot partition and checks its free space **right after the hashes are verified and before anything is written**: it needs the size of the new kernel plus 1 MiB. If there is not enough room it stops with `FAILED: not enough free space on the boot partition (have N bytes, need M); nothing was written, the existing kernel and slot X are untouched` (exit code 2), without touching the other slot's root, the existing kernel or the boot entries. A leftover `*.new` file of an interrupted earlier update (a temporary copy, never the kernel itself) is removed first. The partition is unmounted when the tool exits, on every path. The copy itself still goes to `kernel-X.efi.new` and is moved into place only after a sync; if the copy fails the temporary file is removed.

---

## 4. systemd rules in the image, and the libraries that remain

**The rules (owner's decisions of 2026-10-03, `HUB-OS.md`):** systemd programs are never installed or run; `libsystemd0` and `libudev1` are tolerated for now as plain libraries; the build pins the systemd packages to never install and tests for them; finished images have no apt, PAM modules or procps.

- **The pin.** `image/apt/no-systemd.pref` (the text of `docs/proposals/systemd-libraries.md` section 1.4) is put into `/etc/apt/preferences.d/hubos-no-systemd` before the first package is installed. The test compares the file in the built root with the file in the repository and checks that it lists all seven packages with `Pin-Priority: -1`. **What this does not show:** the package list of this machine (`s6`, `execline`, `busybox-static`, `libc6`) pulls in none of the pinned packages, so the pin has never *had to refuse* anything here; its effect (apt taking `dbus-x11` instead of `dbus-user-session`) was only simulated in the research document, not in this build. It matters when PipeWire, WirePlumber or GTK programs are added.
- **The build test.** It fails if any of `systemd`, `systemd-sysv`, `libpam-systemd`, `dbus-user-session`, `udev`, `systemd-timesyncd`, `systemd-resolved` is marked installed in the root's dpkg status; if a systemd program (`systemd`, `systemd-*`, `systemctl`, `journalctl`, `udevadm`, `udevd`), a unit file (`*.service`, `*.socket`, `*.target`, `*.timer`) or a unit or udev directory (`/etc/systemd`, `/usr/lib/systemd`, `/lib/systemd`, `/etc/udev`, `/usr/lib/udev`, `/lib/udev`) is in the root; and, in the booted image, if any process has a name starting with `systemd` (`/proc/*/comm`).
- **The strip step.** `mmdebstrap --variant=minbase` produced a root that has **no `apt` and no `libapt-pkg6.0t64`** at all (the research document tested a `debootstrap` root, which has both), so for those two the step has nothing to remove and says so. It removes the files of `procps`, `libproc2-0`, `libpam-runtime`, `libpam-modules`, `libpam-modules-bin`, `login` and `passwd` (hundreds of files, including their manuals). It does **not** do step B of the research document: `lslogins`, `lsblk`, `findmnt`, `logger` stay, so do `libsystemd0` and `libudev1`. `dpkg`, its status file and `libpam0g` stay.
- **The library check.** `check-libs.sh` runs `ldd` inside the root on every ELF file in `/usr` (and `/bin`, `/sbin`, `/lib` where they are real directories; here they are links into `/usr`). The build fails on any `not found`. The test also shows the check can fail: it deletes `libmount.so.1` from an unpacked copy and expects `libmount.so.1 => not found`. **This is only link-level.** Nothing proves the stripped programs work beyond what the QEMU tests do.
- **What the two libraries still do.** They are in the root (`/usr/lib/x86_64-linux-gnu/libsystemd.so.0(.38.0)` and `libudev.so.1(.7.8)`); the test lists them. They are kept by decision.

**Not tested:** real logins through `sshd` without PAM modules (no `sshd` is in this image; the research document ran only `sshd -t`); `su`, `login` or password change with no PAM modules.

---

## 5. Measured times (two fresh runs: new temporary directory, nothing cached)

| Step | Run A | Run B |
|---|---|---|
| `fetch-tools.sh` (59 packages, unpacked, not installed) | 31 s | 32 s |
| `build-kernel.sh` (Linux 6.12, both slots, 4 CPUs) | 172 s (148 s of `make`; the slot b build adds only a few seconds); 4,117,504 bytes each | 174 s |
| `build-base.sh` (mmdebstrap from the snapshot) | 35 s | 27 s |
| `build-bundle.sh` (per bundle: root image 4 s, strip and `ldd` check about 6 s) | 10 s each (the check is new) | 10 s each |
| Whole `TestImage` | **1092 s** (18.2 minutes) | **1108 s** (18.5 minutes) |
| First boot to the handover line (QEMU start included) | 11.5 s | 11.6 s |

QEMU hangs seen in both runs: **0**. Boot and rollback times vary by tens of seconds between runs (software emulation).

---|---|
| `fetch-tools.sh` (59 packages, unpacked, not installed) | 36 s |
| `build-kernel.sh` (Linux 6.12, tinyconfig + fragment, 4 CPUs) | 165 s (140 s of it the `make`); 4,117,504 bytes |
| `build-base.sh` (mmdebstrap from the snapshot) | 27 s |
| `build-bundle.sh` (static `hubd` build 3 s, root image 4 s, per bundle about 4 s) | 4 s each |
| Whole `TestImage` | 876 s (14.6 minutes), including the second kernel build of test 9 (about 150 s) |
| First boot to the handover line (QEMU start included) | 11.9 s |

QEMU hangs seen in that run: 0.

---

## 6. What cannot be tested here

Real firmware (does it keep `BootNext`/`BootOrder`, what does it do after a CMOS reset); real hardware watchdogs; Secure Boot; power loss at every point of a write on a real disk; flash wear; TPM; real network speed; any GPU, audio, input; a tap network; the static network branch of the init script (the test uses DHCP only); the NAS backup job; the "refuse update while busy" hook; role images other than the qemu-test machine; out-of-tree modules (none exist in this slice). Performance and boot time on hardware. Also: whether a **real firmware drops boot-entry load options** (OVMF keeps them; the test makes entries without them and an extra recovery-like entry without them, which is what a dropping firmware would produce, but that is a simulation of the result, not of the firmware); that the **hang-and-retry** code ever runs (no hang happened); that the **apt pin** refuses a package (nothing in this package list triggers it); `sshd` or any login without PAM modules.

## 7. Changes from the prototype (and why)

- The root is made with `mmdebstrap` pinned to a dated snapshot, as the owner decided (the prototype used `debootstrap` against the live archive for its tests and `mmdebstrap` for the reproducibility check).
- Unit files and rules of systemd and udev are removed from the root after the build; otherwise the "no systemd unit directory" check cannot pass (they come from `e2fsprogs`, `util-linux`, `dpkg`).
- `hubos-ctl update` refuses a manifest with no `kernel-version`, and prints a line at each step (the write is in 4 MiB pieces) so the runner can interrupt it at known lines.
- The runner waits for known log lines instead of fixed seconds, and retries a hung start once.
- `libbsd0` (needed by `signify`) is unpacked with the tools, not taken from the host.
- (2026-10-03) One kernel per slot, the manifest lists both and the update installs the other slot's; the a and b entries have no load options; free-space check before anything is written; apt pin, strip step and `ldd` check in the build; build tests for banned packages, unit directories and a `systemd` process; the runner lists every hang and fails the run on a second hang in the same step; the QEMU tests are run by hand before merging a change to `image/` or `tools/image/`.

---

## 8. Things noticed while building this (not changed, for the owner)

- `hubos-ctl confirm` writes the running release into the floor (`min_version`) whatever its value. Booting the **older** slot by hand (as test B does at its end) therefore **lowers** the floor (from 7 to 2 in that test), so a release between the two could be installed again. Whether the floor should only ever go up is a design question.
- The recovery entry's load options are the only thing that makes recovery recovery. A firmware that drops them gives a normal boot that never confirms (test A2).
- `mmdebstrap --variant=minbase` gives a root without `apt`; `docs/proposals/phase-b-image.md` and the research document assumed it would be there.
