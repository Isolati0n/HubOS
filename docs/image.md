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
| `image/rootfs/` | files copied into the root: `/sbin/init`, `/usr/sbin/hubos-ctl` (boot entries, update, confirm, rollback), the `udhcpc` script, the s6 service directories (`hubd`, `console`, `confirm`, `wd`) |
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

| # | Proves | How | Result (run A / run B; both fresh; all PASS; 0 hangs) |
|---|---|---|---|
| (files) | the built root has **no systemd program, no unit file or unit directory, none of the seven banned packages installed, and the apt pin file**; the **stripped** root has no file of `apt`, `libapt-pkg6.0t64`, `procps`, `libproc2-0`, `libpam-runtime`, `libpam-modules`, `libpam-modules-bin`, `login`, `passwd` and still has the two libraries | the unpacked squashfs is searched; dpkg's status file and file lists are read; the pin file is compared with `image/apt/no-systemd.pref` | PASS 1.4 / 1.5 s. Remaining: the two libraries (section 4) and `deb-systemd-helper`, `deb-systemd-invoke` (Perl scripts of an Essential package; not systemd programs) |
| (files) | **no unresolved library** (`ldd` inside the root) and the check **can fail** | `check-libs.sh`; then `libmount.so.1` is deleted from the copy and the check must report it | PASS 33.4 / 16.0 s |
| 1 | the root is read-only | `touch /usr/x`, `touch /etc/x`, a write to `/var`, the root mount | PASS 15.3 / 14.1 s (includes the first boot) |
| 2 | first boot sets up the boot entries; `hubd` runs from the config partition; PID 1 is s6-svscan; **0 processes named systemd** | `hubd check`, `hubd list`, `/proc/1/comm`, `ps`, `/proc/*/comm` | PASS 10.8 / 9.1 s |
| A1 | **none of the a, b and recovery boot entries has load options**; the recovery entry points at `kernel-recovery.efi` and is in `BootOrder` after the two slots; slot a boots with its own kernel | `efibootmgr -v`, `/proc/cmdline` | PASS 0.8 / 0.7 s: `BootOrder 0007,0008,0009` |
| 3 | init restarts a killed service | `kill -9` the `hubd` pid, `s6-svstat` | PASS 2.3 / 2.2 s |
| 4 | unsigned, tampered manifest, wrong key, tampered root, not-newer, no `kernel-version`, and manifests listing only one kernel are refused with their own messages, exit 2, **nothing written** | eight bundles; the first 4 MiB of slot b hashed before and after | PASS 11.5 / 9.2 s |
| 5 | a **signed update is accepted**; config survives; the floor is raised; the old bundle is refused; the kernel version is shown | update, reboot, `hubos-ctl status`, the v1 bundle again | PASS 101.3 / 50.5 s (update 10.0 / 7.9 s, reboot to handover 76.7 / 36.4 s) |
| A1 (slot b) | slot b boots with its own kernel (installed by the update) | `/proc/cmdline` | PASS 0.4 / 0.3 s |
| R0 | **the recovery entry is in `BootOrder`, so OVMF keeps it** across the next boot; nothing has to recreate it | the entries stage 1 prints at boot; `efibootmgr -v` | PASS: present at the boot of slot b: `[hubos-a hubos-b hubos-recovery]`; it points at `kernel-recovery.efi` |
| R1 | **recovery after an update** boots the separate recovery kernel; `hubos-ctl status` runs in its shell | `BootNext`, reboot, commands in the recovery shell | PASS 13.4 / 21.1 s: `console=ttyS0 ro loglevel=4 panic=5 \| version=recovery-1 flavor=recovery kernel-version=6.12` |
| 6a to 6d | four bad boots roll back by themselves: no `/sbin/init`, a garbage root, a boot that never gets healthy (confirm timeout 30 s), an init that hangs (watchdog 60 s) | correctly signed bundles; wait for the failure line, then for the confirmed slot | PASS: rollback complete after 46.9 / 53.2, 38.2 / 35.5, 73.0 / 116.2 and 94.0 / 93.7 s. In 6c the recovery entry pointed at `kernel-recovery.efi` during the unconfirmed trial |
| R2 | **recovery after a rollback** is the same recovery kernel as after the update | recovery booted after 6a (slot a holds the bad release 3) | PASS 48.8 / 38.4 s: identical command line and release line |
| 7 | **recovery mode** with an entry that has no load options (banner, bare terminal, `hubos-ctl status`) after the rollbacks and failed trials | `efibootmgr -v`, `BootNext`, reboot | PASS 16.9 / 12.4 s |
| 8 | an update **interrupted at known log lines** never harms the confirmed slot; a clean update works afterwards | section 3.2 | PASS 129.3 / 126.0 s |
| B1 | **confirm refuses a slot whose release (2) is below the floor (7)**: message, exit 2, `BootOrder` and floor unchanged | slot b booted by hand with `BootNext`; the automatic confirm output; a manual `hubos-ctl confirm` | PASS 118.4 / 33.9 s: automatic confirm refused, manual rc=2, slot b did not become first |
| A | **the floor only goes up**: floor stays 7 and release 5 is refused | `cat min_version`, `hubos-ctl update` of release 5 | PASS (with B1 and the next row: 119.1 / 34.5 s) |
| B | an update with a **full boot partition** fails cleanly; the old kernel is untouched and still boots | partition filled until `df` shows almost nothing; update; hashes before and after; `BootNext` to slot b | PASS: `FAILED: not enough free space on the boot partition (have 512 bytes, need 5166080); nothing was written, ...`, rc=2 |
| B2 | **`hubos-ctl rollback REASON`**: refuses without a reason; logs old and new floor and the reason; floor 7 -> 2; the running slot is first | the command, `min_version`, `efibootmgr`, `rollback.log` | PASS 2.0 / 2.0 s: `... rollback: slot b release 2, floor 7 -> 2, BootOrder first: 0008, reason: test: go back to release 2 on purpose` |
| C1 | the **recovery kernel is updated** by a bundle with a newer `recovery-version`; a manifest without one works and leaves recovery alone; the same version is left alone | bundles v9 (none), v10 (version 2), v11 (version 2 again) | PASS 28.3 / 25.9 s: installed version 1 -> 1 -> 2 -> 2 |
| C2 | after that update the machine boots release 11 in slot a and recovery (the new file) boots the same way | reboot; `BootNext` to recovery | PASS 131.5 / 114.6 s |
| C3 | **recovery boots with both slot roots destroyed**; its shell runs `hubos-ctl status`, `e2fsck -fn`, `findfs`/`blkid`, `ip` and `wget` | the first 4 MiB of both roots zeroed; `BootNext` | PASS 23.1 / 15.1 s |
| C4 | OVMF: a first entry that **loads and then reboots** is retried forever; the second and third entries are never reached | `BootOrder` a, b, recovery; both roots destroyed; 100 s of serial log counted | PASS: starts `hubos-a` 14, no load failures, no other entry (229.4 / 229.7 s with C5) |
| C5 | OVMF: a first entry whose **file is missing** falls through to the second; after the second loads and fails, the third (recovery) is still never reached | `kernel-a.efi` removed; 100 s counted | PASS: `hubos-a` failed to load 14 times, `hubos-b` started 14 times, recovery never |
| 9 | reproducible builds: two builds of the root tar give identical hashes (asserted); the squashfs and **all three kernels** are reported | `build-base.sh` twice; `build-root-image.sh` twice; the kernels built again | PASS 365.8 / 309.4 s: tar, squashfs and kernels identical |
| hook | the hang-retry code | `HUBOS_TEST_FORCE_HANG=once` and `=twice` (section 3.8) | not run again this round (the hook code is unchanged); the final runs listed 0 hangs |

Note on the times: run A was slowed by a second job on the same CPUs (the desktop experiment of `docs/proposals/phase-b-desktop.md`), run B only for its first minutes; compare the tests, not the seconds.

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

### 3.6 The update floor: only up, confirm refuses below it, `rollback` goes down on purpose

- **The floor only goes up.** `hubos-ctl confirm` writes the larger of the old floor and the running release into `/config/hubos/state/min_version`.
- **Confirm refuses a slot whose release is below the floor** (`confirm: REFUSED: release 2 of slot b is below the floor 7; BootOrder and the floor are unchanged (to go back on purpose: hubos-ctl rollback REASON)`, exit code 2). Nothing changes: the slot is not made first in `BootOrder` and the floor is not touched. Test B1: after the rollbacks and the interrupted updates (floor 7, release 7 in slot a) the older slot b (release 2) is booted by hand with `BootNext`; its automatic confirm step prints the refusal, a manual `hubos-ctl confirm` returns 2, slot b did not become first in `BootOrder`, the floor is still 7, and an update to release 5 is refused (`below the floor 7`; test A).
- **`hubos-ctl rollback REASON`** is what the owner runs *on purpose* to go back to the running older release. It needs a reason (without one it prints usage and returns 2), needs the running slot to have been booted through its own entry, **lowers the floor to the running release**, **makes the running slot first in `BootOrder`**, and **logs** one line in `/config/hubos/state/rollback.log` with the time, slot, release, `floor 7 -> 2`, the new first entry and the reason. Test B2: usage without a reason; with a reason the output is `rollback: floor 7 -> 2; slot b is now first in BootOrder; reason logged: ...`, `min_version` is 2, slot b is first, and the log line holds the reason. It does not install anything and does not touch the other slot. (The clock of the test image is whatever the firmware gave it; the log's time is only as good as that.)
- A rollback boot by the firmware (a failed trial) never changes the floor, because it boots a slot that is already confirmed.

### 3.7 The separate recovery kernel

**What it is.** `kernel-recovery.efi` is a third kernel built with the same configuration as the slot kernels but with another initramfs and a command line that has no `root=` and no `hubos.slot=` (`console=ttyS0 ro loglevel=4 panic=5`). Its stage 0 (`image/stage0/recovery-init`) mounts nothing of either slot: it mounts `/proc`, `/sys`, `/dev`, `efivarfs`, the config partition **read-only**, brings up the network with DHCP, prints `HUBOS: RECOVERY MODE ...` and leaves a bare terminal (`exec setsid cttyhack /bin/sh`). It does **not** arm the watchdog and never reboots by itself. Its initramfs (6.3 MB kernel in total) holds busybox (all applets), `hubos-ctl`, `efibootmgr`, `signify-openbsd`, e2fsprogs (`e2fsck`, `mke2fs`, `tune2fs`, `resize2fs`, `dumpe2fs`, `debugfs`), `findfs`, `blkid`, the network script and the libraries they need, copied from the base root and the unpacked tools at build time (`tools/image/recovery-list.py`). BusyBox's shell prefers its own applets over files in `PATH`, and its `findfs` and `mke2fs` are not the full programs (busybox `findfs` has no `PARTLABEL=`), so `/etc/recovery.rc` (read through `ENV`) aliases `findfs`, `mke2fs` and `blkid` to the real ones, and `hubos-ctl` calls `/sbin/findfs` by path.

**The entry.** The `hubos-recovery` boot entry **always** points at `\EFI\hubos\kernel-recovery.efi`, with **no load options**. `ensure-entries` deletes and recreates a recovery entry that points anywhere else (made by an earlier image). The entry sits in `BootOrder` **directly after the two slot entries** (`BootOrder: 0007,0008,0009,...` in the tests). `confirm` and `rollback` write `mine,other,recovery`.

**What this simplifies (answers to "is keeping it in BootOrder or recreating it at confirm still needed?").**
- *Recreating the recovery entry at every confirm is no longer needed and is gone.* In the last round OVMF was seen to delete the recovery entry at each reboot (it was not in `BootOrder`) so the confirm step had to recreate it; with the entry in `BootOrder` the OVMF run keeps it (test R0: the entries present when slot b booted after the update were `hubos-a hubos-b hubos-recovery`). The cause was inferred from the two observations (the entry vanished when it was outside `BootOrder`, stays when inside); no test isolated it further. `ensure-entries` still runs at every boot but returns at once when everything is in place; it matters for the first install and for repairs.
- *Keeping it in `BootOrder`: yes, for that reason.* It also puts recovery after the slots for a firmware that falls through (section 3.9).
- The old `HUBOS: RECOVERY MODE` branch in `/sbin/init` and the label check are gone; a slot kernel is never started by the recovery entry.

**Which slot recovery boots after an update and after a rollback: none.** Recovery does not use a slot at all, so the question has one answer: it is the same recovery kernel after the first boot, after an update to the other slot, during an unconfirmed trial boot, after a rollback and with both slot roots destroyed. The tests compare the command line and the release line of the recovery shell and they are identical every time (`console=ttyS0 ro loglevel=4 panic=5 | version=recovery-1 ...`): R1 after an update, R2 after a rollback (slot a holding a rejected release), 7 after four failed trials, C2 after a recovery-kernel update, C3 with both roots garbage. During the unconfirmed trial of test 6c the entry points at `kernel-recovery.efi`, not at the trial slot. So **recovery cannot boot a stale or unconfirmed slot any more.** What it can still be is stale *itself*: it is a copy made at build time and changes only when a bundle carries a newer `recovery-version` (below). Of the three fixes proposed last round, (c) (own recovery kernel) and (b) (recovery in `BootOrder`) are built; (a) (re-point the entry at the confirmed slot) is no longer needed because the entry no longer names a slot.

**Updating the recovery kernel.** The manifest may carry `recovery-version N` and a `kernel-recovery.efi` line (hash and size). `hubos-ctl update`:
- *manifest without either line:* the update works as before and says `the manifest has no recovery kernel; recovery left alone` (test C1, bundle 9);
- *one of the two lines only:* `REFUSED: the manifest lists the recovery kernel only in part`; a non-numeric version is refused;
- *N greater than the installed version* (`\EFI\hubos\recovery.version` on the boot partition, written by `build-disk.sh` for the first image): the recovery kernel is fetched, its hash is checked with the others, and it is installed **after the slot kernel** (copy to `kernel-recovery.efi.new`, sync, `mv`, then the version file) before `BootNext` is set; the free-space check counts it. Test C1: version 1 -> 2 with a changed file (the runner appends 16 bytes to the kernel and re-signs): the new hash is on the boot partition;
- *N not greater:* `not newer than the installed 2; left alone`, nothing is copied (test C1, bundle 11).
The recovery kernel is not signed itself beyond the manifest's hash, and it has **no copy of the update public key**, so `hubos-ctl update` does not work from recovery (`running slot unknown`); the shell has the tools (`wget`, `signify-openbsd`, `e2fsck`, `mke2fs`, `findfs`, `ip`) for a repair by hand. An update from recovery that writes a chosen slot is not built (a question).

**Tests (all in section 3 table):** R0 (entry persists), R1/R2/7 (same recovery kernel; `hubos-ctl status` runs in its shell; no load options), C1/C2 (updates and a boot of the new file after release 11 is confirmed), C3 (**both slot roots destroyed** by zeroing their first 4 MiB, then a `sysrq` reboot into the recovery entry via `BootNext`: recovery comes up, `hubos-ctl status`, `e2fsck -fn` of the config partition, `findfs`/`blkid`, `ip` and `wget` (it downloaded the host's manifest over DHCP) all work).

**What this does not show:** a real firmware (OVMF only); a recovery boot on a machine whose ESP is full or whose config partition is damaged (the shell would still come up; not tested); a keyboard (the tests use the serial console); the network path other than QEMU's DHCP.

### 3.8 One-kernel manifests are refused, and the hang hook

**One-kernel manifests (test 4).** Two correctly signed bundles are made from release 2 with one kernel line removed from the manifest: `v2-onekernel-a` (no `kernel-b.efi` line) and `v2-onekernel-b` (no `kernel-a.efi` line). Both are refused with `REFUSED: manifest does not list kernel-b.efi` (resp. `kernel-a.efi`), exit code 2, before any rootfs is fetched. The test also checks that the output has no `writing to slot` or `wrote` line and that the first 4 MiB of the other slot's root have the same hash before and after the eight offered bundles.

**The hang hook (`HUBOS_TEST_FORCE_HANG`).** For proving the retry code without waiting for a real hang. `HUBOS_TEST_FORCE_HANG=once` makes the **first boot** of the run count as a hang (QEMU is really started, then killed, without waiting for the handover line); `=twice` makes the first boot **and its retry** count as hangs. Results (run with the final code, work directory kept):

- `once`: `RESULTS (QEMU hangs seen and retried: 1; every hang: hang 1 in step "first boot [forced by HUBOS_TEST_FORCE_HANG]" (serial log .../hangs/hang-1.log))`; the retry booted, tests 1, 2, A1 and 3 passed (`--- PASS: TestImage`).
- `twice`: `RESULTS (QEMU hangs seen and retried: 2; every hang: hang 1 in step "first boot [forced ...]" ...; hang 2 in step "first boot (retry) [forced ...]" ...)`, and `the machine hung twice in the same step (first boot); the run fails`; every test that needs the machine is skipped (`run stopped by a repeated hang`) and `--- FAIL: TestImage`.

The hook only covers the boot path of the first boot; the other places that call the same retry code (after a reboot, after an interruption, after a rollback) share it but were not forced separately.

### 3.9 A boot-loop breaker: proposal only (nothing is built)

**What the firmware does (TESTED with OVMF, tests C4 and C5; BELIEVED for real firmware).** The claim "UEFI falls through to the next `BootOrder` entry only when the loader cannot load the file, not when a loaded kernel then fails" is **correct for OVMF**:
- *C4:* both slot roots were destroyed (so every slot kernel loads and then reboots at once: `STAGE0: cannot mount the root of slot a; rebooting`). With `BootOrder: a, b, recovery` and no `BootNext`, the firmware started `hubos-a` **14 times in 100 seconds** and `hubos-b` and `hubos-recovery` **never**.
- *C5:* the file `kernel-a.efi` was then removed from the boot partition (the loader cannot load it). The firmware logged `failed to load Boot0007 "hubos-a"` **14 times** and started `hubos-b` **14 times**, and `hubos-recovery` **never**: after `hubos-b` loaded and then failed, the firmware went back to the start of the list instead of on to recovery.
So the firmware moves down the list only on a load failure, and a kernel that loads and then fails sends it round the same entry forever. Putting recovery at the end of `BootOrder` helps only when the slot kernels' *files* are missing. Fallback to the other slot after a failed boot works today only for a **trial** boot (`BootNext` is used up on the first attempt, so the next boot is the confirmed slot). If the **confirmed** slot itself starts failing (a bad config on the data/config partition, a damaged root, a hardware problem), the machine loops until someone intervenes. The watchdog does not help: it only turns a hang into a reboot.

**Proposal.** A failure counter that stage 0 of every slot kernel keeps and the healthy confirm step resets:
1. *Where.* A file on the config partition (`/config/hubos/state/boot-failures`) or an EFI variable (written through `efivarfs`, which stage 0 can mount). The file needs stage 0 to mount the config partition read-write (ext4) before it mounts the root, one more thing that can fail; the EFI variable needs no filesystem but is non-volatile storage written on every boot and is lost if the firmware clears its variables.
2. *Count.* Stage 0 adds one at the start of every boot and syncs. The confirm step (healthy, as today) and a clean shutdown set it to zero. A watchdog reset, a power loss or a stage 0 refusal therefore leave it raised. A trial boot that fails counts as 1 and the confirmed slot's good boot then resets it.
3. *Limit.* After N consecutive failed boots (N in the machine's build config, for example 3) stage 0 does **not** mount the root: it sets `BootNext` to the recovery entry and reboots, so the separate recovery kernel (which exists now) comes up with a bare terminal. Needs `efibootmgr` (and its two libraries, about 1 MB) in the slot kernels' initramfs, or a few lines of shell that write the `BootNext` variable directly.
4. *Afterwards.* The counter stays at N until the owner (or a repair step) clears it with a `hubos-ctl` command, so recovery is not left by accident into the loop again.

**What it needs, as questions for the owner (not decided):** (a) N, and whether a trial boot's failure should count; (b) file on the config partition or EFI variable; (c) whether stage 0 should send the machine to recovery or first try the *other* slot (flip `BootOrder`) and only then recovery; (d) whether a recovery shell on a rack machine with no keyboard is useful at all, or recovery should instead fetch and install the last good bundle by itself from the hub (needs the update key in the recovery kernel and a rule for which bundle); (e) whether a machine that cannot reach anything (no network, no keyboard) should stay in the loop, power off, or keep trying; (f) real firmware: BootNext handling and the fall-through rule are untested outside OVMF; (g) how the hub learns that a node is in recovery (`HUB-OS.md` says no phone alerts; the hub shows the node as down).

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
| `fetch-tools.sh` (59 packages, unpacked, not installed) | 34 s | 31 s |
| `build-kernel.sh` (Linux 6.12, kernels a, b and recovery, 4 CPUs) | 239 s (304 s in the script's own count); 4,117,504 bytes each for a and b, 6,325,248 for the recovery kernel | 198 s (260 s) |
| `build-base.sh` (mmdebstrap from the snapshot) | about 36 s | about 37 s |
| `build-bundle.sh` (per bundle: root image, strip, `ldd` check) | about 10 s each | about 10 s each |
| Whole `TestImage` | **2220 s** (37.0 minutes; slowed by a parallel job) | **1837 s** (30.6 minutes) |
| First boot to the handover line (QEMU start included) | 14.8 s | 13.8 s |

QEMU hangs seen in both runs: **0**. The three kernel hashes were the same in both runs (`a 7525247660fcf1de`, `b 5c1ca9d6de129d97`, `recovery e353ed80f165f01e`, first 16 hex digits). Boot and rollback times vary by tens of seconds between runs (software emulation). The runs are longer than the earlier ones (23 minutes) because of the recovery-kernel, floor and firmware tests (C4 and C5 alone take 229 s: 100 s of counting each).

---

## 6. What cannot be tested here

Real firmware (does it keep `BootNext`/`BootOrder`, what does it do after a CMOS reset); real hardware watchdogs; Secure Boot; power loss at every point of a write on a real disk; flash wear; TPM; real network speed; any GPU, audio, input; a tap network; the static network branch of the init script (the test uses DHCP only); the NAS backup job; the "refuse update while busy" hook; role images other than the qemu-test machine; out-of-tree modules (none exist in this slice). Performance and boot time on hardware. Also: whether a real firmware falls through the same way as OVMF (section 3.9); how a real keyboard and screen behave in the recovery shell (the tests use the serial console); a real hang (the retry code is exercised only through the test hook); that the **apt pin** refuses a package (nothing in this package list triggers it); `sshd` or any login without PAM modules.

## 7. Changes from the prototype (and why)

- The root is made with `mmdebstrap` pinned to a dated snapshot, as the owner decided (the prototype used `debootstrap` against the live archive for its tests and `mmdebstrap` for the reproducibility check).
- Unit files and rules of systemd and udev are removed from the root after the build; otherwise the "no systemd unit directory" check cannot pass (they come from `e2fsprogs`, `util-linux`, `dpkg`).
- `hubos-ctl update` refuses a manifest with no `kernel-version`, and prints a line at each step (the write is in 4 MiB pieces) so the runner can interrupt it at known lines.
- The runner waits for known log lines instead of fixed seconds, and retries a hung start once.
- `libbsd0` (needed by `signify`) is unpacked with the tools, not taken from the host.
- (2026-10-03) One kernel per slot, the manifest lists both and the update installs the other slot's; the a and b entries have no load options; free-space check before anything is written; apt pin, strip step and `ldd` check in the build; build tests for banned packages, unit directories and a `systemd` process; the runner lists every hang and fails the run on a second hang in the same step; the QEMU tests are run by hand before merging a change to `image/` or `tools/image/`.
- (2026-10-03, second round) The floor only goes up; recovery is recognised by the entry label, not by load options; one-kernel manifests are refused; the hang retry is exercised by `HUBOS_TEST_FORCE_HANG`; the recovery entry's behaviour (dropped by OVMF at reboot, recreated by the confirm step for the running slot) is tested and documented, with fixes proposed but not built.
- (2026-10-03, third round) A separate recovery kernel (`kernel-recovery.efi`, own initramfs, no root, no slot) replaces the recovery-by-label branch of `/sbin/init`; the recovery entry always points at it, has no load options and sits in `BootOrder` after the slots; the manifest may carry `recovery-version` and the update tool installs a newer recovery kernel; `confirm` refuses a slot below the floor; new `hubos-ctl rollback REASON`; the OVMF fall-through is measured and a boot-loop breaker is proposed (section 3.9).

---

## 8. Things noticed while building this

- (Fixed 2026-10-03, section 3.6) `hubos-ctl confirm` used to be able to lower the floor.
- (Fixed 2026-10-03, section 3.7) recovery used to depend on load options, then on the slot that booted last. It is now its own kernel and needs no slot.
- (Open, section 3.9) a confirmed slot that loads and then fails loops forever on OVMF; a boot-loop breaker is proposed, not built.
- Busybox's shell and applets shadow the real `findfs` (no `PARTLABEL=`) and `mke2fs`; the first recovery tests failed on this until `hubos-ctl` called `/sbin/findfs` by path and `recovery.rc` aliased the real programs.
- `mmdebstrap --variant=minbase` gives a root without `apt`; `docs/proposals/phase-b-image.md` and the research document assumed it would be there.
- Test A boots the older slot by hand with `BootNext`; that makes slot b the first entry in `BootOrder` (it confirms itself). The floor stays, but the **slot order** is now "older release first". A rule that a confirmed boot may not make an older release the preferred slot is not built (UNKNOWN whether it is wanted).
