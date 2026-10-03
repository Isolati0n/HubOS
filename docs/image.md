# The Phase B image: build and test (slices S0 to S3)

**Status:** built and tested in the build environment (`docs/environment.md`): QEMU in software emulation, **no KVM**, no real hardware. Everything here follows `docs/proposals/phase-b-image.md` and the owner's answers recorded in it ("Owner decisions (2026-10-02)"). Times are emulation times on 4 CPUs and say nothing about real machines. `HUB-OS.md` wins if anything here disagrees with it.

Labels: **TESTED** (the command is in the runner or shown below), **BELIEVED**, **UNKNOWN**.

---

## 1. What is in the repo

| Path | What |
|---|---|
| `image/machines/qemu-test.build` | the first machine's build configuration, shell-readable `KEY=VALUE`: `KERNEL_VERSION` (6.12), `KERNEL_FRAGMENT`, `MODULES` (empty), `PACKAGES`, `SNAPSHOT`, `SERVICES`, `CONFIRM_TIMEOUT`. The scripts read the kernel version, the fragment and the package list from this file; nothing is hard-wired. |
| `image/kernel/qemu-test.frag` | kernel configuration fragment, appended to `make tinyconfig` (no modules) |
| `image/packages/qemu-test.list` | the package list of the root: `s6`, `execline`, `busybox-static`, `libc6` (the prototype's list) |
| `image/stage0/` | stage 0, the program built into the kernel (`init`, `stage0.list.in`) |
| `image/rootfs/` | files copied into the root: `/sbin/init`, `/usr/sbin/hubos-ctl` (boot entries, update, confirm), the `udhcpc` script, the s6 service directories (`hubd`, `console`, `confirm`, `wd`) |
| `image/config/qemu-test/` | the config partition's `inventory.toml` and `viewers.toml` (copies of the examples) |
| `tools/image/*.sh` | `fetch-tools.sh` (unpack QEMU and friends into a temporary directory), `build-kernel.sh`, `build-base.sh` (mmdebstrap), `build-root-image.sh`, `build-disk.sh`, `build-bundle.sh`; `common.sh` is shared |
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

# keep the long build between runs (the kernel and the root are rebuilt only when their inputs change):
HUBOS_IMAGE_WORK=/tmp/hubos-image go test -tags qemu -count=1 -timeout 150m -v ./tools/image
```

The scripts can also be run by hand (`WORK=/tmp/x tools/image/fetch-tools.sh`, then `build-kernel.sh`, `build-base.sh`, `build-bundle.sh 2 good DIR PUBKEY SECKEY`, `build-disk.sh ROOT KERNEL DIR`). `MACHINE=image/machines/other.build` selects another machine file.

**How the image is built:**
1. `build-kernel.sh`: downloads `linux-$KERNEL_VERSION`, `make tinyconfig`, appends the machine's fragment (with the stage 0 list filled in), builds the `bzImage`. `KBUILD_BUILD_TIMESTAMP`, user and host are fixed so that two builds can be compared.
2. `build-base.sh`: `mmdebstrap --mode=root --variant=minbase --components=main,universe noble` with the machine's package list, from **`http://snapshot.ubuntu.com/ubuntu/$SNAPSHOT`** (a dated snapshot of the archive), with `SOURCE_DATE_EPOCH` set. Output: a tar file.
3. `build-root-image.sh`: unpacks the base, copies `image/rootfs`, the static `hubd` (`CGO_ENABLED=0 go build -trimpath`), `efibootmgr` and `signify`, the public key, writes `/etc/hubos-release`, **removes** the unit files and rules packages ship for systemd and udev (`/etc/systemd`, `/usr/lib/systemd`, `/var/lib/systemd`, `/usr/lib/udev`: inert, from `e2fsprogs`, `util-linux`, `dpkg`), and packs a squashfs (zstd; `SOURCE_DATE_EPOCH` fixes every time stamp). Bad flavors for the tests: `noinit`, `garbage`, `unhealthy`, `hang`.
4. `build-bundle.sh`: root image + kernel + a manifest (`version`, `flavor`, `arch`, **`kernel-version`**, **`modules`**, sha256 and size of both files) signed with `signify`.
5. `build-disk.sh`: a 420 MiB GPT disk (boot, slot A, slot B, config, data), partitions made as separate files and put in place with `dd` (no loop devices), plus a writable copy of OVMF's variable store.

---

## 3. The tests (`tools/image/qemu_test.go`)

One Go test, `TestImage`, with sub-tests in the order below; they share one disk. The runner drives QEMU (`q35`, OVMF, 2 vCPUs, 1 GB, virtio disk and network, an `i6300esb` watchdog) over the serial console, and serves the bundles to the guest from an HTTP server on the host (the guest sees the host as `10.0.2.2`).

| # | Proves | How | Result (final clean run) |
|---|---|---|---|
| (files) | the built root has **no systemd program, no systemd unit file or unit directory** (the unpacked squashfs is searched; `systemd`, `systemd-*`, `systemctl`, `journalctl`, `udevadm`, `udevd`, `*.service`, `*.socket`, `*.target`, `*.timer`, `/etc/systemd`, `/usr/lib/systemd`, `/lib/systemd`, udev directories) | host side | PASS. Remaining: the two libraries (below) and `deb-systemd-helper`, `deb-systemd-invoke` (Perl scripts of the Essential package `init-system-helpers`; not systemd programs) |
| 1 | the root is read-only | `touch /usr/x`, `touch /etc/x`, a write to `/var`, the root mount | PASS (12.1 s including the first boot) |
| 2 | first boot sets up the boot entries; `hubd` runs from the config partition; PID 1 and the process list have **no systemd** | `hubd check --inventory /config/hubos/inventory.toml`, `hubd list`, `cat /proc/1/comm`, `ps` | PASS (8.7 s). The run lists the libraries in the running root |
| 3 | init restarts a killed service | `kill -9` the `hubd` pid, wait, `s6-svstat`, `hubd list` | PASS (2.3 s) |
| 4 | **unsigned, tampered manifest, wrong key, tampered root, not-newer, and a signed bundle with no `kernel-version` line are all refused**, with their own messages, exit code 2, and `BootNext` unset | six bundles | PASS (4.2 s) |
| 5 | a **signed update is accepted**; the config marker survives; the floor is raised; the old bundle is refused; the kernel version in the manifest is shown | update, reboot, `hubos-ctl status`, the v1 bundle again | PASS (26.0 s: update 5.7 s, reboot to handover 14.1 s) |
| 6a to 6d | **four bad boots roll back by themselves**: no `/sbin/init` (stage 0), a garbage root (stage 0), a boot that never gets healthy (confirm timeout 40 s), an init that hangs (watchdog, 60 s) | each bundle is correctly signed; the runner waits for the failure line, then for the confirmed slot to come back | PASS: rollbacks complete after 29.6, 29.2, 93.2 and 98.8 s |
| 7 | recovery mode is a boot option | `efibootmgr -n` the recovery entry, reboot | PASS (15.7 s): `RECOVERY MODE` banner, `hubos.recovery=1`, read-only root |
| 8 | an update **interrupted at known log lines** never harms the confirmed slot, and an update still works afterwards | see 4 | PASS (97.6 s) |
| 9 | **reproducible builds**: two builds of the root tar give identical hashes (asserted); the squashfs and the kernel are reported | `build-base.sh` twice; `build-root-image.sh` twice; the kernel built again from scratch | PASS (191.9 s): tar identical, squashfs identical, kernel identical |

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

The prototype saw one start in about twelve stop silently after `BdsDxe: starting Boot0001`. The runner now waits for a known line (`HUBOS: handing over to s6-svscan`, 150 s) after every start and every reboot; if it does not come, the serial log is saved in `$WORK/hangs/`, QEMU is killed and the start is retried once, and the number of such events is printed at the end (`QEMU hangs seen and retried: N`). Result of the runs of the final code: **0 hangs** in each (see the measured runs in section 5). The cause of the earlier hang stays UNKNOWN; not seeing it in these runs does not mean it is gone.

---

## 4. The libraries that remain (temporary, by the owner's decision)

`libsystemd0` and `libudev1` are in the image. The test lists them in the built root: `/usr/lib/x86_64-linux-gnu/libsystemd.so.0(.38.0)` and `/usr/lib/x86_64-linux-gnu/libudev.so.1(.7.8)`. The owner decides about them after `docs/proposals/systemd-libraries.md`. No systemd program is installed or running (tests above). The apt pin from that document is not needed here (the package list has no package that would pull `systemd`), and it is not yet part of the build; it must be before PipeWire, WirePlumber, GTK programs are added.

---

## 5. Measured times (final clean run: new temporary directory, nothing cached)

| Step | Time |
|---|---|
| `fetch-tools.sh` (59 packages, unpacked, not installed) | 36 s |
| `build-kernel.sh` (Linux 6.12, tinyconfig + fragment, 4 CPUs) | 165 s (140 s of it the `make`); 4,117,504 bytes |
| `build-base.sh` (mmdebstrap from the snapshot) | 27 s |
| `build-bundle.sh` (static `hubd` build 3 s, root image 4 s, per bundle about 4 s) | 4 s each |
| Whole `TestImage` | 876 s (14.6 minutes), including the second kernel build of test 9 (about 150 s) |
| First boot to the handover line (QEMU start included) | 11.9 s |

QEMU hangs seen in that run: 0.

---

## 6. What cannot be tested here

Real firmware (does it keep `BootNext`/`BootOrder`, what does it do after a CMOS reset); real hardware watchdogs; Secure Boot; power loss at every point of a write on a real disk; flash wear; TPM; real network speed; any GPU, audio, input; a tap network; the static network branch of the init script (the test uses DHCP only); the NAS backup job; the "refuse update while busy" hook; role images other than the qemu-test machine; out-of-tree modules (none exist in this slice). Performance and boot time on hardware.

## 7. Changes from the prototype (and why)

- The root is made with `mmdebstrap` pinned to a dated snapshot, as the owner decided (the prototype used `debootstrap` against the live archive for its tests and `mmdebstrap` for the reproducibility check).
- Unit files and rules of systemd and udev are removed from the root after the build; otherwise the "no systemd unit directory" check cannot pass (they come from `e2fsprogs`, `util-linux`, `dpkg`).
- `hubos-ctl update` refuses a manifest with no `kernel-version`, and prints a line at each step (the write is in 4 MiB pieces) so the runner can interrupt it at known lines.
- The runner waits for known log lines instead of fixed seconds, and retries a hung start once.
- `libbsd0` (needed by `signify`) is unpacked with the tools, not taken from the host.
