# PROPOSAL: pre-mortem. "It is December and the hub has failed."

**Status: PROPOSAL / THINKING. Nothing here is decided, built or tested** unless a line says TESTED or SOURCE. `HUB-OS.md` wins if this file disagrees with it. Written 2026-10-10 by the lead agent (no helpers). No code. Labels: TESTED, SOURCE, BELIEVED, UNKNOWN. **All thresholds and numbers are placeholders.**

## 1. Purpose
Imagine the hub failed badly in December. Write the ten likeliest causes **now**, each with an early warning, a cheap experiment and a design change, so the cheap experiments can be run before the hardware arrives. "Failed" means the owner's rules are broken: the hub froze or crashed, a service failure caused a reboot, or an update left the hub unusable (SOURCE `HUB-OS.md`: uptime and stability come first).

## 2. The ten causes
### 1. A GPU driver or firmware regression
- **Cause:** a new kernel, Mesa or GPU firmware makes the GPU hang or reset; the compositor freezes or dies.
- **Early warning:** GPU reset messages in the kernel log; the compositor probe failing (SOURCE `hub-stability.md` 9.1); frame heartbeat gaps in the burn-in monitor.
- **Cheap experiment:** on the real GPU in December, trigger a reset on purpose with the debugfs file (`burn-in-tool.md`; the file name is SOURCE per the owner, BELIEVED in `hub-stability.md` G1) and watch the restart path. Before then: nothing can be tested here (no real GPU: SOURCE `hub-stability.md` "What the cloud environment cannot show").
- **Design change:** firmware is pinned **inside the slot** (BELIEVED: SOURCE `HUB-OS.md` says the distro supplies "userland, modules and firmware" and each slot carries its own kernel), so a rollback restores the old firmware together with the old kernel and userland. Adopt a new kernel or Mesa only after 4 weeks and a 72-hour soak (SOURCE `HUB-OS.md`).

### 2. A kernel memory leak
- **Cause:** a slab leak (driver, network stack) slowly eats memory; the hub is stable for days and then dies.
- **Early warning:** `Slab` and `SUnreclaim` from `/proc/meminfo` rising over days while process memory is flat.
- **Cheap experiment:** add both lines to the burn-in report (`burn-in-tool.md`) and look at the slope per day on any long run (a QEMU run with the hub kernel shows the guest kernel's slab: BELIEVED; TESTED? no).
- **Design change:** nothing to patch in advance; the report is the change. A threshold on `SUnreclaim` growth makes a slow leak visible in the probation fortnight, and the A/B rollback is the remedy.

### 3. A compositor freeze under 20 windows
- **Cause:** a stall in driftwm or Smithay under 20 always-open windows at 4K.
- **Early warning:** frame heartbeat gaps; probe rounds failing; memory of the compositor rising.
- **Cheap experiment:** the 20-window tests already exist (SOURCE `hub-stability.md`; TestHubImage step H3f in this repo kills the compositor with 20 fake windows, TESTED PASS twice on 2026-10-10: `docs/qemu-runs/` in PR #75). Add the hang probe and a 72-hour run on a test machine with real GPU in December.
- **Design change:** the owner's rules already exist: restart after a hang by probe, never reboot, stop after 5 crashes in a minute and show a message (SOURCE `HUB-OS.md`). The hang rule itself is **not built** (SOURCE `docs/STATE.md` 4 item 5), so building it is the change.

### 4. Conductor-to-worker socket problems under loss or restart storms
- **Cause:** with 20 workers the Unix sockets between the conductor and the workers jam, lose messages or loop through restarts.
- **Early warning:** seam drop counters (`seams-catalogue.md`), restart counts, conductor mailbox size.
- **Cheap experiment:** a fake-worker test with 20 processes that crash and reconnect at random, while the conductor restarts; first check that the BEAM can use SOCK_SEQPACKET and pass descriptors (UNKNOWN: `docs/research/verified-facts-2026-10-10.md`; the tiny test is Task 4 of the docs PR).
- **Design change:** Unix sockets, not Erlang distribution (the owner's choice); bounded queues and drop policies on every seam (`seams-catalogue.md` rules 2, 5); restart limits with escalation to a bar alert (SOURCE `HUB-OS.md`).

### 5. Unbounded clipboard history
- **Cause:** a bug or a huge item defeats the caps (about 500 text items plus about 100 MB: SOURCE `HUB-OS.md`) and the history eats RAM; the hub has no swap (SOURCE `hub-stability.md`).
- **Early warning:** the clipboard store's memory against its cap in the burn-in report.
- **Cheap experiment:** push 10 000 items and a stream of 1 MiB items at the store (the store does not exist yet: `clipboard.md` is a proposal), and check eviction and the memory ceiling; also run it under a cgroup limit once the hub kernel has cgroups.
- **Design change:** hard memory limit for the store process (the owner's memory-limited group for viewers does not cover it: BELIEVED), eviction by size as well as by count, items over the limit refused at the door.

### 6. A broken A/B rollback
- **Cause:** a bad release does not roll back (the recovery kernel or boot-loop breaker fails on real firmware).
- **Early warning:** a trial boot that does not confirm and the failure counter not incrementing.
- **Cheap experiment:** the QEMU suite covers it: T05 to T08, T14 (boot-loop breaker) PASS in TestImage on 2026-10-10 (TESTED in QEMU only). **On real firmware it is UNKNOWN** (SOURCE `HUB-OS.md` Unverified: EFI variable failure counter and NVRAM wear). Cheap new experiment: run the same tests under a different OVMF build and with an EFI variable store near full.
- **Design change:** the recovery kernel in every slot, installed at confirm (SOURCE `HUB-OS.md`); hardware watchdog as the last resort; an out-of-band device decided in December. Never update the hub without a person present on the first real update (BELIEVED, an operating rule, not decided).

### 7. A reconnect storm from the open-everything action
- **Cause:** pressing the open-everything button (`open-everything-in-layout.md`) starts 20 connections at once, or a down node is retried too fast, and the hub or the nodes stall.
- **Early warning:** connection counts and CPU spike in the burn-in monitor right after the button.
- **Cheap experiment:** press it 100 times in a row against fake nodes that accept slowly, with the stagger at 500 ms, 100 ms and 0 ms, and measure the compositor's frame heartbeat.
- **Design change:** the stagger, one open per interval, a single active run, backoff with jitter on every reconnect (`seams-catalogue.md` failure table).

### 8. A full log or evidence store
- **Cause:** logs or crash evidence fill a filesystem; services stall on writes (the init comparison measured disk-full behaviour for services: TESTED partly, `init-comparison.md`).
- **Early warning:** free space on the filesystem holding the logs.
- **What the repo says:** logs are in RAM and rotate (`s6-log n5 s200000`, five files of 200 000 bytes per service: SOURCE `image/rootfs/etc/s6/sv/hubd/log/run`; SOURCE `hub-stability.md` question 14 "logs live in RAM and are lost at reboot"); capped crash evidence goes to the config partition (8 boots, 64 KiB per file, 256 KiB per boot: SOURCE `HUB-OS.md`). **A separate "log partition" is not recorded in the repo**; this item therefore covers a full RAM filesystem and a full config partition.
- **Cheap experiment:** fill each of the two to 100% in a QEMU run while the desktop runs, and watch the bar and `hubd`.
- **Design change:** caps already exist; add a check that alerts at 80% (BELIEVED), and make sure no service blocks on a log write (the init comparison's disk-full step checks this).

### 9. An untested configuration (for example guest chains)
- **Cause:** the owner uses a configuration no test covered, such as a guest of a guest (`generated-inventory.md` question 1), 21 machines, or a machine with an unusual address.
- **Early warning:** none in advance; a validation rule that does not exist.
- **Cheap experiment:** a table of "allowed shapes" with one fake-node run for each; plus fuzz the inventory and machine-file parsers.
- **Design change:** make every untested shape a validation error with a plain message instead of a surprise.

### 10. An incomplete source mirror
- **Cause:** the cluster cannot rebuild an image (or the hub) years later because one input is missing or changed upstream (`source-mirror.md`).
- **Early warning:** a build that fails with "missing input" in the first rebuild test.
- **Cheap experiment:** rebuild the reference image from an empty cache using only the mirror (a local folder) and compare the hashes with the original build (T09 gives the comparison method: SOURCE `docs/image.md`).
- **Design change:** the lock lists every input; the mirror verifies weekly; builds never fetch.

## 3. The three experiments to run first
1. **Mirror-only rebuild (cause 10):** it needs no hardware, uses an existing reproducibility test and finds a silent gap early.
2. **Fake-worker storm with 20 processes, and the SOCK_SEQPACKET/descriptor test (causes 4 and 7):** cheap and it settles a UNKNOWN that the session layer design depends on.
3. **Fill the logs and the config partition under the desktop (cause 8)** and a **100-times open-everything run (cause 7)** together, because both are quick and both could stall the bar.
(Causes 1, 2 and 6 on real hardware wait for December; they need the real GPU, a long run and real firmware.)

## 4. Five assumptions I am least sure of
1. That firmware lives inside the slot so a rollback restores it (BELIEVED from the contract wording; no image in the repo has GPU firmware yet).
2. That the QEMU guest kernel's `Slab`/`SUnreclaim` says anything about real drivers (UNKNOWN).
3. That the ten causes are the likeliest (BELIEVED; the list is the owner's, I have not ranked them with data).
4. That a bounded clipboard store needs its own memory limit (BELIEVED).
5. That the config partition and RAM are the only places logs can fill (UNKNOWN: no log partition is recorded).

## 5. Questions for the owner
1. Is this the right list of ten, or should any be swapped for another cause?
2. Which of the three first experiments may be started first once the init comparison is done?
3. Should the first real update of the hub always be done with a person present?
