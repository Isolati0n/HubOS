# The Phase B image: build and test (slices S0 to S3, per-slot kernels, separate recovery kernel)

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
| `image/stage0/` | stage 0, the program built into the slot kernels (`init`, `stage0.list.in`), and the **recovery kernel's** init (`recovery-init`) and shell setup (`recovery.rc`) |
| `image/rootfs/` | files copied into the root: `/sbin/init`, `/usr/sbin/hubos-ctl` (boot entries, update, confirm), the `udhcpc` script, the s6 service directories (`hubd`, `console`, `confirm`, `wd`) |
| `image/config/qemu-test/` | the config partition's `inventory.toml` and `viewers.toml` (copies of the examples) |
| `tools/image/*.sh` | `fetch-tools.sh` (unpack QEMU and friends into a temporary directory), `build-kernel.sh`, `build-base.sh` (mmdebstrap), `build-root-image.sh`, `build-disk.sh`, `build-bundle.sh`; `strip-root.sh` (deletes apt, PAM modules, procps, login, passwd files from a finished root), `check-libs.sh` (`ldd` inside the root) and `recovery-list.py` (the file list of the recovery kernel's initramfs); `common.sh` is shared |
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
1. `build-kernel.sh`: downloads `linux-$KERNEL_VERSION`, `make tinyconfig`, appends the machine's fragment (with the stage 0 list and slot `a` filled in), builds the `bzImage` as `kernel-a.efi`; then changes only the slot name in the built-in command line (`hubos-root-a hubos.slot=a` to `hubos-root-b hubos.slot=b`) and builds again as `kernel-b.efi`; then swaps the initramfs for the **recovery** one (`recovery-list.py`: busybox, `hubos-ctl`, `efibootmgr`, `signify`, e2fsprogs, `findfs`/`blkid`, the network script and the libraries they need, copied from the base root and the unpacked tools) and the command line for one without `root=` and builds `kernel-recovery.efi`. The later builds recompile only what depends on the changed parts. The base root is built first if it is not there. `KBUILD_BUILD_TIMESTAMP`, user and host are fixed so that two builds can be compared.
2. `build-base.sh`: `mmdebstrap --mode=root --variant=minbase --components=main,universe noble` with the machine's package list, from **`http://snapshot.ubuntu.com/ubuntu/$SNAPSHOT`** (a dated snapshot of the archive), with `SOURCE_DATE_EPOCH` set and, through a setup hook, `image/apt/no-systemd.pref` placed in `/etc/apt/preferences.d/` before the first package is installed. Output: a tar file.
3. `build-root-image.sh`: unpacks the base, copies `image/rootfs`, the static `hubd` (`CGO_ENABLED=0 go build -trimpath`), `efibootmgr` and `signify`, the public key, writes `/etc/hubos-release`, **removes** the unit files and rules packages ship for systemd and udev (`/etc/systemd`, `/usr/lib/systemd`, `/var/lib/systemd`, `/usr/lib/udev`: inert, from `e2fsprogs`, `util-linux`, `dpkg`), then **strips** the root (`strip-root.sh`: the files of `apt`, `libapt-pkg6.0t64`, `procps`, `libproc2-0`, `libpam-runtime`, `libpam-modules`, `libpam-modules-bin`, `login` and `passwd`, taken from dpkg's own file lists; busybox supplies `ps` and `pidof`), runs `check-libs.sh` (**the build fails** if `ldd`, run inside the root, finds an unresolved library in any ELF file under `/usr`, `/bin`, `/sbin` or `/lib`), and packs a squashfs (zstd; `SOURCE_DATE_EPOCH` fixes every time stamp). Bad flavors for the tests: `noinit`, `garbage`, `unhealthy`, `hang`.
4. `build-bundle.sh`: root image + **all three kernels** (`kernel-a.efi`, `kernel-b.efi`, `kernel-recovery.efi`) + a manifest (`version`, `flavor`, `arch`, **`kernel-version`**, **`modules`**, **`recovery-version`**, sha256 and size of the root and of every kernel) signed with `signify`.
5. `build-disk.sh`: a 420 MiB GPT disk (boot, slot A, slot B, config, data), partitions made as separate files and put in place with `dd` (no loop devices), plus a writable copy of OVMF's variable store. The boot partition gets `kernel-a.efi`, the recovery kernel (`kernel-recovery.efi`, with `recovery.version`) and the fallback loader.

---

## 3. The tests (`tools/image/qemu_test.go`)

One Go test, `TestImage`, with sub-tests in the order below; they share one disk. The runner drives QEMU (`q35`, OVMF, 2 vCPUs, 1 GB, virtio disk and network, an `i6300esb` watchdog) over the serial console, and serves the bundles to the guest from an HTTP server on the host (the guest sees the host as `10.0.2.2`).

| # | Proves | How | Result (run A / run B; both fresh, all PASS, 0 hangs) |
|---|---|---|---|
| (files) | the built root has **no systemd program, no unit file or unit directory, none of the seven banned packages installed, and the apt pin file** in it | the unpacked squashfs is searched; dpkg's status file is read; the pin file is compared with `image/apt/no-systemd.pref` | PASS 0.9 / 1.0 s. Remaining: the two libraries (section 4) and `deb-systemd-helper`, `deb-systemd-invoke` (Perl scripts of the Essential package `init-system-helpers`; not systemd programs) |
| (files) | the **stripped** root has no file of `apt`, `libapt-pkg6.0t64`, `procps`, `libproc2-0`, `libpam-runtime`, `libpam-modules`, `libpam-modules-bin`, `login`, `passwd`, and still has the two libraries | dpkg's file lists against the root | PASS (apt and libapt are not installed in this root at all) |
| (files) | **no unresolved library** (`ldd` inside the root, `/usr`, `/bin`, `/sbin`, `/lib`) and the check **can fail** | `check-libs.sh`; then `libmount.so.1` is deleted from the copy and the check must report it | PASS 15.7 / 15.5 s; 714 ELF files checked |
| 1 | the root is read-only | `touch /usr/x`, `touch /etc/x`, a write to `/var`, the root mount | PASS (15.1 / 14.2 s including the first boot) |
| 2 | first boot sets up the boot entries; `hubd` runs from the config partition; PID 1 and the process list have **no systemd** | `hubd check`, `hubd list`, `cat /proc/1/comm`, `ps`; then `/proc/*/comm` is counted for names starting with `systemd` | PASS (9.1 / 9.0 s); **0 processes named systemd** |
| A1 | **none of the a, b and recovery boot entries has load options**, and slot a boots with its own kernel | `efibootmgr -v` (each line ends at `File(\EFI\hubos\kernel-X.efi)`), `/proc/cmdline` | PASS: `console=ttyS0 ro rootfstype=squashfs root=PARTLABEL=hubos-root-a hubos.slot=a panic=5 loglevel=4 i6300esb.heartbeat=60` |
| 3 | init restarts a killed service | `kill -9` the `hubd` pid, wait, `s6-svstat`, `hubd list` | PASS (2.2 s) |
| 4 | unsigned, tampered manifest, wrong key, tampered root, not-newer, no `kernel-version` line, and **manifests listing only one kernel** are refused with their own messages, exit code 2, **nothing written** | eight bundles; the first 4 MiB of slot b are hashed before and after | PASS (8.3 / 8.5 s) |
| 5 | a **signed update is accepted**; the config marker survives; the floor is raised; the old bundle is refused; the kernel version in the manifest is shown | update, reboot, `hubos-ctl status`, the v1 bundle again | PASS (48.9 / 87.5 s: update 7.1 / 8.2 s, reboot to handover 34.4 / 72.8 s; the reboot time varies a lot) |
| A1 (slot b) | **slot b boots with its own kernel** (installed by the update; no load options) | `/proc/cmdline` in slot b | PASS: `... root=PARTLABEL=hubos-root-b hubos.slot=b ...` |
| R0 | **OVMF drops the recovery entry** at the next boot; the confirm step recreates it for the running slot | the line stage 1 prints before anything can recreate an entry; `efibootmgr -v` afterwards | PASS: entries present at the boot of slot b: `[hubos-a hubos-b]`; afterwards the recovery entry points at `kernel-b.efi` |
| R1 | **recovery after an update** | `BootNext` to the recovery entry, reboot, commands in the recovery shell | PASS (22.1 / 23.3 s): recovery booted **slot b release 2**, the confirmed slot |
| 6a to 6d | four bad boots roll back by themselves: no `/sbin/init` (stage 0), a garbage root (stage 0), a boot that never gets healthy (confirm timeout 40 s), an init that hangs (watchdog, 60 s) | each bundle is correctly signed; the runner waits for the failure line, then for the confirmed slot to come back | PASS: rollbacks complete after 31.3 / 38.7, 41.5 / 42.7, 114.7 / 82.6 and 116.3 / 118.9 s. In 6c, **during the unconfirmed trial boot of slot a the recovery entry pointed at `kernel-a.efi`** (the trial slot) |
| R2 | **recovery after a rollback** boots the confirmed slot | after 6a (slot a holds the bad release 3): recovery entry booted | PASS (43.1 / 31.6 s): recovery booted **slot b release 2**, not the slot holding the rolled-back release |
| 7 | **recovery mode with an entry that has no load options** | `efibootmgr -v` shows none; `BootNext`; reboot; banner, `/proc/cmdline` without `hubos.recovery`, read-only root, `HUBOS: booted entry 0009 label 'hubos-recovery'` | PASS (11.2 / 15.7 s) |
| 8 | an update **interrupted at known log lines** never harms the confirmed slot, and an update still works afterwards | see 3.2 | PASS (113.0 / 144.5 s) |
| A | **the floor only goes up**: the older slot (release 2) booted and confirmed by hand after the rollbacks; the floor stays 7; release 5 is refused (`below the floor 7`) | `cat /config/hubos/state/min_version`, then `hubos-ctl update` of release 5 | PASS (49.2 / 48.5 s, together with B) |
| B | an update with a **full boot partition** fails cleanly; the old kernel is untouched and still boots | the partition is filled until `df` shows 2 free blocks; `hubos-ctl update` of a newer bundle; hashes of `kernel-b.efi` and of slot b's root before and after; `BootNext` to slot b and reboot | PASS: `FAILED: not enough free space on the boot partition (have 2048 bytes, need 5166080); nothing was written, ...`, exit 2, no `.new` file, no `BootNext`, hashes equal, slot b booted release 2 |
| 9 | reproducible builds: two builds of the root tar give identical hashes (asserted); the squashfs and **both kernels** are reported | `build-base.sh` twice; `build-root-image.sh` twice; the kernels built again from scratch | PASS (255.5 / 267.0 s): tar, squashfs, kernel a and kernel b identical |
| hook | the hang-retry code | `HUBOS_TEST_FORCE_HANG=once` and `=twice` (section 3.8) | `once`: 1 hang listed, run passes; `twice`: 2 hangs listed, run fails |

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

The prototype saw one start in about twelve stop silently after `BdsDxe: starting Boot0001`. The runner waits for a known line (`HUBOS: handing over to s6-svscan`, 150 s) after every start and every reboot. **Rule:** if the line does not come, the hang is recorded (its number, the step it happened in and the saved serial log in `$WORK/hangs/`), QEMU is killed and that step is retried **once**; **a second hang in the same step fails the run**. At the end the runner prints `QEMU hangs seen and retried: N; every hang: ...` with every hang listed. No real hang has happened in any run. The retry code is exercised by a **test hook** (section 3.8). The cause of the earlier hang stays UNKNOWN, and not seeing it does not mean it is gone.

### 3.4 One kernel per slot (the slot does not depend on the firmware's load options)

Before: each boot entry carried its command line as load options, while the kernel's built-in command line named slot a. A firmware that drops load options would have booted slot b's kernel with slot a's root. Now:

- `build-kernel.sh` builds **two kernels**; each has its own slot and root built in: `kernel-a.efi` (`root=PARTLABEL=hubos-root-a hubos.slot=a`) and `kernel-b.efi` (`...-b ... hubos.slot=b`). Everything else is identical; both files are 4,117,504 bytes and their hashes differ.
- The bundle carries both; the signed manifest lists both with hash and size. `hubos-ctl update` refuses a manifest that does not list both, but **fetches and installs only the other slot's kernel** (`kernel-$OTHER.efi`).
- **None of the three boot entries (a, b, recovery) has load options.** The recovery entry boots a **separate recovery kernel** (section 3.7), so it needs no command line either.
- A machine whose entries were made by an earlier image (with load options) keeps those entries; `ensure-entries` only creates missing entries. No such machine exists yet.

### 3.5 Free space on the boot partition

`hubos-ctl update` now mounts the boot partition and checks its free space **right after the hashes are verified and before anything is written**: it needs the size of the new kernel plus 1 MiB. If there is not enough room it stops with `FAILED: not enough free space on the boot partition (have N bytes, need M); nothing was written, the existing kernel and slot X are untouched` (exit code 2), without touching the other slot's root, the existing kernel or the boot entries. A leftover `*.new` file of an interrupted earlier update (a temporary copy, never the kernel itself) is removed first. The partition is unmounted when the tool exits, on every path. The copy itself still goes to `kernel-X.efi.new` and is moved into place only after a sync; if the copy fails the temporary file is removed.

### 3.6 The update floor only goes up

`hubos-ctl confirm` used to write the running release into `/config/hubos/state/min_version` whatever the old value was, so booting an **older** slot by hand lowered the floor and made a release in between installable again. Now it writes the **larger** of the old floor and the running release and says so (`confirm: boot of slot b (version 2) confirmed; floor stays at 7 (it only goes up)`). Test A: after the rollbacks and the interrupted updates the floor is 7 (release 7 is running in slot a); the older slot b (release 2) is booted by hand and confirms itself; the floor is **still 7**, and an update to release 5 is refused (`REFUSED: version 5 is below the floor 7`). A rollback boot by itself never changed the floor, because it boots a slot that is already confirmed.

### 3.7 The recovery entry

**How recovery is recognised.** Stage 1 (`/sbin/init`) asks the firmware which entry it booted (`efibootmgr`: `BootCurrent`) and enters recovery mode (banner, bare terminal, read-only root, nothing else started) **if that entry is labelled `hubos-recovery`**. It prints `HUBOS: booted entry 0009 label 'hubos-recovery'`. The command line is no longer looked at. Test 7: the recovery entry has no load options (`efibootmgr -v` shows the path and nothing after it), `/proc/cmdline` has no `hubos.recovery`, and the machine still comes up in recovery mode. (A machine whose recovery entry was made by an earlier image with the load options still works: the label is the same.)

**Which slot's kernel the recovery entry boots, and when it is made (TESTED, with OVMF).** The entry points at `kernel-X.efi` of the slot that was **running when the entry was created** (`ensure_entries` in `hubos-ctl`). What the tests showed:

- **OVMF deletes the recovery entry at the next boot.** It is not in `BootOrder` (the a and b entries are). Stage 1 now prints `HUBOS: entries present at boot: ...` before anything can recreate an entry; at the boot of slot b after the update it printed `hubos-a hubos-b`, with no recovery entry (test R0). The **confirm step recreates it on every boot**, and it runs `ensure-entries` *before* it checks health. So the entry always points at the slot that **booted last**.
- **After an update:** the trial boot of the new slot recreates the entry for the new slot. Test 6c shows that during an **unconfirmed** trial boot (hubd never healthy) the recovery entry points at `kernel-a.efi`, the trial slot. A recovery boot at that moment would run the unconfirmed release. After the update is confirmed it is the confirmed slot (test R1: recovery booted slot b release 2 after the update to b was confirmed).
- **After a rollback:** the rollback boot is a boot of the confirmed slot, which recreates the entry for itself. Test R2: after the rollback from the bad release 3 in slot a, the recovery entry booted **slot b, release 2** (the confirmed one), not the slot holding the rolled-back release.
- **Stale or unconfirmed cases that are possible:** (1) recovery during a trial boot (above); (2) a trial boot that fails **before** the confirm step runs (stage 0 refuses the root, init hangs) never recreates the entry, so no recovery entry exists at all for that boot and the next one (OVMF has dropped the old one): the machine returns to the confirmed slot, which recreates it, so this only matters if the confirmed slot cannot boot either; (3) a firmware that **keeps** entries not in `BootOrder` (real firmware may; UNKNOWN) would keep an entry pointing at a failed trial slot after the rollback until the confirmed slot's confirm step recreates it (it only recreates a *missing* entry, so a kept stale entry would stay stale: `ensure_entries` does not check where an existing recovery entry points); (4) if neither slot can reach the confirm step, **there is no recovery entry**, which is exactly when it is needed.

**Proposed fixes (not built; for the owner to choose, see the questions in the reply):** (a) make `ensure_entries` re-point an existing recovery entry whenever it points at a different slot, and create/re-point it **only from a confirmed boot** (in `hubos-ctl confirm`), so it always names the confirmed slot; (b) put the recovery entry **at the end of `BootOrder`** so the firmware keeps it (a machine whose both slots fail would then fall into recovery by itself instead of looping); (c) give recovery its own small kernel **built to run recovery from stage 0** (a recovery shell inside the initramfs that mounts nothing it does not have to), so it depends on neither slot's root.

### 3.8 One-kernel manifests are refused, and the hang hook

**One-kernel manifests (test 4).** Two correctly signed bundles are made from release 2 with one kernel line removed from the manifest: `v2-onekernel-a` (no `kernel-b.efi` line) and `v2-onekernel-b` (no `kernel-a.efi` line). Both are refused with `REFUSED: manifest does not list kernel-b.efi` (resp. `kernel-a.efi`), exit code 2, before any rootfs is fetched. The test also checks that the output has no `writing to slot` or `wrote` line and that the first 4 MiB of the other slot's root have the same hash before and after the eight offered bundles.

**The hang hook (`HUBOS_TEST_FORCE_HANG`).** For proving the retry code without waiting for a real hang. `HUBOS_TEST_FORCE_HANG=once` makes the **first boot** of the run count as a hang (QEMU is really started, then killed, without waiting for the handover line); `=twice` makes the first boot **and its retry** count as hangs. Results (run with the final code, work directory kept):

- `once`: `RESULTS (QEMU hangs seen and retried: 1; every hang: hang 1 in step "first boot [forced by HUBOS_TEST_FORCE_HANG]" (serial log .../hangs/hang-1.log))`; the retry booted, tests 1, 2, A1 and 3 passed (`--- PASS: TestImage`).
- `twice`: `RESULTS (QEMU hangs seen and retried: 2; every hang: hang 1 in step "first boot [forced ...]" ...; hang 2 in step "first boot (retry) [forced ...]" ...)`, and `the machine hung twice in the same step (first boot); the run fails`; every test that needs the machine is skipped (`run stopped by a repeated hang`) and `--- FAIL: TestImage`.

The hook only covers the boot path of the first boot; the other places that call the same retry code (after a reboot, after an interruption, after a rollback) share it but were not forced separately.

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
| `fetch-tools.sh` (59 packages, unpacked, not installed) | 35 s | 36 s |
| `build-kernel.sh` (Linux 6.12, both slots, 4 CPUs) | 219 s (186 s of `make`; the slot b build adds only a few seconds); 4,117,504 bytes each | 208 s |
| `build-base.sh` (mmdebstrap from the snapshot) | 35 s | 35 s |
| `build-bundle.sh` (per bundle: root image, strip, `ldd` check) | about 10 s each | about 10 s each |
| Whole `TestImage` | **1357 s** (22.6 minutes) | **1399 s** (23.3 minutes) |
| First boot to the handover line (QEMU start included) | 14.7 s | 13.9 s |

QEMU hangs seen in both runs: **0**. Boot and rollback times vary by tens of seconds between runs (software emulation). The runs are slower than the earlier ones (18 minutes) because of the new tests (recovery boots, the full-partition test).

---|---|---|
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

Real firmware (does it keep `BootNext`/`BootOrder`, what does it do after a CMOS reset); real hardware watchdogs; Secure Boot; power loss at every point of a write on a real disk; flash wear; TPM; real network speed; any GPU, audio, input; a tap network; the static network branch of the init script (the test uses DHCP only); the NAS backup job; the "refuse update while busy" hook; role images other than the qemu-test machine; out-of-tree modules (none exist in this slice). Performance and boot time on hardware. Also: whether a **real firmware keeps or drops** boot entries that are not in `BootOrder` (OVMF drops them, section 3.7); whether a real firmware keeps boot-entry load options (nothing depends on them any more); a real hang (the retry code is exercised only through the test hook); that the **apt pin** refuses a package (nothing in this package list triggers it); `sshd` or any login without PAM modules.

## 7. Changes from the prototype (and why)

- The root is made with `mmdebstrap` pinned to a dated snapshot, as the owner decided (the prototype used `debootstrap` against the live archive for its tests and `mmdebstrap` for the reproducibility check).
- Unit files and rules of systemd and udev are removed from the root after the build; otherwise the "no systemd unit directory" check cannot pass (they come from `e2fsprogs`, `util-linux`, `dpkg`).
- `hubos-ctl update` refuses a manifest with no `kernel-version`, and prints a line at each step (the write is in 4 MiB pieces) so the runner can interrupt it at known lines.
- The runner waits for known log lines instead of fixed seconds, and retries a hung start once.
- `libbsd0` (needed by `signify`) is unpacked with the tools, not taken from the host.
- (2026-10-03) One kernel per slot, the manifest lists both and the update installs the other slot's; the a and b entries have no load options; free-space check before anything is written; apt pin, strip step and `ldd` check in the build; build tests for banned packages, unit directories and a `systemd` process; the runner lists every hang and fails the run on a second hang in the same step; the QEMU tests are run by hand before merging a change to `image/` or `tools/image/`.
- (2026-10-03, second round) The floor only goes up; recovery is recognised by the entry label, not by load options; one-kernel manifests are refused; the hang retry is exercised by `HUBOS_TEST_FORCE_HANG`; the recovery entry's behaviour (dropped by OVMF at reboot, recreated by the confirm step for the running slot) is tested and documented, with fixes proposed but not built.

---

## 8. Things noticed while building this

- (Fixed 2026-10-03, section 3.6) `hubos-ctl confirm` used to be able to lower the floor.
- (Fixed 2026-10-03, section 3.7) recovery used to depend on load options. What remains open is which slot recovery boots and the missing entry when neither slot reaches the confirm step.
- `mmdebstrap --variant=minbase` gives a root without `apt`; `docs/proposals/phase-b-image.md` and the research document assumed it would be there.
- Test A boots the older slot by hand with `BootNext`; that makes slot b the first entry in `BootOrder` (it confirms itself). The floor stays, but the **slot order** is now "older release first". A rule that a confirmed boot may not make an older release the preferred slot is not built (UNKNOWN whether it is wanted).
