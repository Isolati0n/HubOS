# Black box recorder for failed boots (proposal)

**Written:** 2026-10-05. **Status:** proposal and experiments only. Nothing here changes the image, the kernel configuration or `HUB-OS.md`. Every decision in section 8 is the owner's.

## Owner decisions (2026-10-05)

These answer the questions in section 8. They are the owner's decisions; the lead recorded them from the owner's message. Recorded in `HUB-OS.md` as well.

- efi-pstore stays OFF (it fills the firmware variable store and disables the boot-loop breaker).
- ramoops is used, with about 1 MiB of reserved RAM per machine.
- Stage 0 and the recovery kernel may mount the config partition read-write to save evidence.
- Caps: 8 boots, 64 KiB per file and 256 KiB per boot.
- The hub shows a machine's logs only while the machine is in recovery and does not store them.
- Evidence is also saved after the first crash following a clean boot (counter 0).

## 0. How to read this

Every claim carries exactly one label:

- **TESTED** : a command was run in this session and gave the result written here. The command and the output (shortened where said) are in the text. All tests ran in QEMU 8.2.2 (software emulation, no KVM) with OVMF (the Ubuntu 24.04 build, `OVMF_CODE_4M.fd` and `OVMF_VARS_4M.fd`) and a Linux 6.12 kernel built for these tests. QEMU is not a real machine.
- **SOURCE** : read in a document or in program source; the link and the date I read it are given. For the kernel I read the 6.12 source tarball, `https://cdn.kernel.org/pub/linux/kernel/v6.x/linux-6.12.tar.xz`, on 2026-10-05, and name the file inside it. Source says what the code is written to do; it is not a test.
- **BELIEVED** : my reasoning or my proposal. Not tested.
- **UNKNOWN** : I do not know, and could not find out here.

In plain words: a "black box recorder" is a place where a machine writes down what it was doing, so that after it crashes or hangs and restarts, someone can read what happened. The owner cannot sit in front of a crashed machine (HUB-OS.md: no BMC, recovery is built in), so this evidence has to be saved by the machine itself and handed to the hub over the recovery agent.

Helpers: no helper agents were used. Everything below I ran or read myself.

## 1. The short version

1. **The repo's kernel records nothing today.** TESTED (section 2): the finished `.config` of both `qemu-test.frag` and `hub.frag` has `# CONFIG_PSTORE is not set`. Without it, there is no pstore, no ramoops and no efi-pstore.
2. **Two kinds of "pstore" storage exist and only one can be active at a time.** SOURCE (`fs/pstore/platform.c`) and TESTED (section 4.5, r6): with both built in and ramoops configured, the kernel says `pstore: backend 'ramoops' already in use: ignoring 'efi_pstore'`.
3. **RAM (ramoops)** keeps the whole kernel log (even when the kernel only hangs), survives a reset inside the same QEMU process (panic reboot, watchdog reset, reboot), and is **lost** when QEMU is killed and started again or the machine is powered off. TESTED (sections 4.2 to 4.5).
4. **UEFI variables (efi-pstore)** keep only the kernel log of a **panic**, split into many small pieces, and **survive everything**, including a killed QEMU and a new QEMU on the same OVMF variable file. TESTED (section 3). They record **nothing** when the machine is reset by the watchdog without a panic. TESTED (section 3.2).
5. **Danger for the boot-loop breaker.** In a loop of panics with efi-pstore on, the OVMF variable store filled after 24 panics (336 records) and then **a 1-byte variable such as the failure counter could not be created any more**. TESTED (section 3.5). The boot-loop breaker lives in EFI variables, so efi-pstore can switch it off. This is the strongest reason to prefer the RAM backend plus a copy to disk.
6. **The place in RAM matters.** Of five addresses tried, four survived a warm reset and one (480 MiB of a 512 MiB machine) did not. TESTED (section 4.1). I do not know why. On real hardware the right place has to be found by testing: UNKNOWN.
7. **What a program prints with plain `echo` is not recorded.** Only kernel messages (printk) are. Stage 0 must write its progress lines to `/dev/kmsg`. TESTED (section 6.2).
8. **A save step on the config partition works.** The prototype copied the pstore files into a capped folder, emptied pstore, and a **new QEMU** (cold boot) read them back from the config disk. TESTED (section 6).
9. **`GET /v1/logs` already exists in the prototype agent and already needs a signature.** I propose its content and limits (section 7). BELIEVED.
10. **Real hardware is untested**: whether RAM survives a real warm reset, how big the real firmware variable store is, whether a real watchdog reset keeps RAM. UNKNOWN (section 9).

## 2. What the repo's kernel has

**TESTED.** I rebuilt only the `.config` the way `tools/image/build-kernel.sh` does (`make tinyconfig`, append the fragment, `make olddefconfig`), for kernel 6.12 (the version in `image/machines/qemu-test.build`), without compiling:

```
WORK=<dir with linux-6.12 and tools/root> tools/image/experiments/black-box/config-check.sh qemu-test
WORK=<dir with linux-6.12 and tools/root> tools/image/experiments/black-box/config-check.sh hub
```

Result (shortened to the relevant lines; the same for `qemu-test` and `hub`):

```
# CONFIG_PSTORE is not set
# CONFIG_KEXEC is not set
# CONFIG_FIRMWARE_MEMMAP is not set
# CONFIG_PANIC_ON_OOPS is not set
CONFIG_EFI=y
CONFIG_EFIVAR_FS=y
CONFIG_WATCHDOG=y
CONFIG_I6300ESB_WDT=y
CONFIG_WATCHDOG_NOWAYOUT=y
CONFIG_MAGIC_SYSRQ=y
CONFIG_PANIC_TIMEOUT=-1
CONFIG_LOG_BUF_SHIFT=17
== PSTORE lines counted: 0 ==
```

So: `CONFIG_PSTORE*`, `CONFIG_PSTORE_RAM` and `CONFIG_EFI_VARS_PSTORE` are all absent. `CONFIG_EFIVAR_FS=y` is there (that is what the boot-loop counter uses). The built-in command line has `panic=5` (SOURCE: `image/kernel/qemu-test.frag`, line `CONFIG_CMDLINE=...`), so a panic restarts the machine after 5 seconds. The kernel log buffer is 128 KiB (`LOG_BUF_SHIFT=17`).

**Which options are needed** (SOURCE: `fs/pstore/Kconfig`, `drivers/firmware/efi/Kconfig`, 6.12): `CONFIG_PSTORE=y`; `CONFIG_PSTORE_CONSOLE=y` (every kernel message, also when nothing crashes); `CONFIG_PSTORE_RAM=y` (it pulls in the Reed-Solomon code for error repair); optional `CONFIG_PSTORE_PMSG=y` (a device `/dev/pmsg0` for programs to leave notes); `CONFIG_EFI_VARS_PSTORE` depends only on `PSTORE` and is `default y`, so turning on PSTORE turns it on too unless switched off. `CONFIG_PSTORE_COMPRESS` is `default y`.

The lab kernel I used is exactly the repo's tinyconfig plus `qemu-test.frag` plus these lines (`tools/image/experiments/black-box/lab.frag`):

```
CONFIG_PSTORE=y  CONFIG_PSTORE_CONSOLE=y  CONFIG_PSTORE_PMSG=y  CONFIG_PSTORE_RAM=y
CONFIG_EFI_VARS_PSTORE=y  CONFIG_PRINTK_TIME=y  CONFIG_PROC_SYSCTL=y
```

Its only other change is a shorter built-in command line (`console=ttyS0 loglevel=7`, no `panic=`, no `root=`) so the experiments choose `panic=` themselves, and a tiny init (`lab-init`) in place of stage 0. **The size cost of these options in the real kernel was not measured: UNKNOWN.**

## 3. UEFI variables (efi-pstore)

### 3.1 What it is (SOURCE)

`drivers/firmware/efi/efi-pstore.c` (6.12): every record is one UEFI variable with the Linux crash GUID `cfc8fc79-be2e-4ddc-97f0-9f98bfe298a0`, attributes non-volatile + boot-service + runtime, named `dump-type<T>-<part>-<count>-<time>-<C|D>`. The backend's flags are `PSTORE_FLAGS_DMESG` only: it stores **kernel log dumps (panic/oops), not the live console, not user messages**. Each variable is at most `record_size` bytes (default and minimum 1024; module parameter `efi_pstore.record_size`). A switch `efi_pstore.pstore_disable` and a Kconfig `EFI_VARS_PSTORE_DEFAULT_DISABLE` exist. The kernel also refuses writes that would leave less than 5120 bytes free in the store (`arch/x86/platform/efi/quirks.c`, `EFI_MIN_RESERVE 5120`).

### 3.2 Commands and results (all TESTED)

All use `tools/image/experiments/black-box/scenarios.sh NAME`. One scenario = a fresh copy of `OVMF_VARS_4M.fd` and a fresh 1 MiB raw file that serves as a boot counter. The full QEMU line of every run is written to `cmd-NN.txt` by the harness; for example (efi scenarios have no `memmap`/`ramoops` words):

```
qemu-system-x86_64 -L $WORK/tools/root/usr/share/qemu -L $WORK/tools/root/usr/share/seabios -machine q35,smm=off -accel tcg -smp 2 -m 512 -nographic -display none \
  -drive if=pflash,format=raw,unit=0,readonly=on,file=$WORK/tools/root/usr/share/OVMF/OVMF_CODE_4M.fd \
  -drive if=pflash,format=raw,unit=1,file=$WORK/sc-e1/vars.fd \
  -drive file=$WORK/sc-e1/disk.img,if=none,id=d0,format=raw -device virtio-blk-pci,drive=d0 \
  -nic none -drive file=$WORK/sc-e1/cfg.img,if=none,id=d1,format=raw -device virtio-blk-pci,drive=d1 \
  -device i6300esb -watchdog-action reset -kernel $WORK/lab/bzImage \
  -append "console=ttyS0 loglevel=7 panic=5 bb.act=panic bb.end=off" -serial file:$WORK/sc-e1/serial-01.log -monitor none
```

(QEMU's `-kernel` makes OVMF start the kernel through its EFI stub; the guest log says `efi: EFI v2.7 by Ubuntu distribution of EDK II`, so EFI runtime services are present.)

| Scenario | What happens | Result |
|---|---|---|
| **e1** panic, kernel restarts itself (`panic=5`) in the same QEMU | boot 1 panics (`echo c > /proc/sysrq-trigger`); boot 2 lists `/sys/fs/pstore` | `backend in use: efi_pstore`; **14 files** `dmesg-efi_pstore-...001` to `...014`, 1624 to 1705 bytes each. File 1 starts `Panic#1 Part1` and ends with `Kernel panic - not syncing: sysrq triggered crash`. |
| **e2** watchdog reset, no panic (`bb.act=wdt`, `i6300esb.heartbeat=15`, `-device i6300esb -watchdog-action reset`) | boot 1 opens `/dev/watchdog` and never feeds it | the machine reset (boot number 2 appears) but **0 pstore files and 0 crash-GUID variables**. A hang or an unfed watchdog leaves no efi-pstore record. |
| **e3** panic with `panic=0` (halt), 3 s later `kill -9` of that QEMU's PID, then a **new QEMU on the same `vars.fd`** | | **15 files** found in the new QEMU (process kill survived). |
| **e4** (cold boot) after e1: new QEMU on e1's `vars.fd`, `bb.erase=1` | | the 14 records of e1 are still there; the guest then `rm`s them; its own `efivars` listing still said 14 right after (stale view). |
| **e6** one more new QEMU on the same file | | **0 files**: erasing through the pstore files really removes the variables. |
| **e5** fresh `vars.fd` | | nothing there. |

So: efi-pstore **survives** a process kill and a new QEMU (as long as the variable file is kept), it only records **panics**, and one panic costs **14 to 15 variables** here (about 23 KiB of record text; 23585 bytes in the e8 run).

### 3.3 How big is the store (TESTED, OVMF only)

`bb.act=fill` writes 1000-byte variables through efivarfs until the firmware refuses:

```
BB: FILL-DONE: 223 variables of 1000 bytes were accepted before the firmware said no (variable 224 failed)
BB: after filling, a 1-byte variable could still be created
```

The OVMF variable file is 540672 bytes (`stat -c %s vars.fd`). Note: efivarfs needs attributes and data in **one** `write()`; my first try wrote them in two and got 0 (lesson recorded in `lab-init`).

### 3.4 A boot loop of panics (TESTED, OVMF only)

Scenario **e9** (`panic=1 bb.act=panic bb.loop=40 bb.probe=1`, nobody erases anything; every boot also tries to create a 1-byte variable named like the boot-loop counter, GUID `a0b7c0f5-...`, attributes 7). I stopped it at boot 27 (killed my own QEMU by its PID). Extract of the guest's own lines:

```
boot number 2   15 pstore file(s)    probe: ... worked
boot number 12  160 pstore file(s)   probe: ... worked
boot number 24  333 pstore file(s)   probe: ... worked
boot number 25  336 pstore file(s)   probe: creating the counter-like variable FAILED
boot number 26  336 pstore file(s)   probe: ... FAILED
boot number 27  336 pstore file(s)   probe: ... FAILED
```

In plain words: after 24 panics the firmware's variable store was full; **later panics were no longer recorded (336 stays 336) and the 1-byte counter variable could not be written.** `image/stage0/init` handles a failed write by printing `WARNING cannot write the failure counter; the boot-loop breaker is OFF for this boot; booting normally` (SOURCE: repo, `image/stage0/init`), so a full store turns the breaker off. This is OVMF's store; **a real board's store may be smaller or larger: UNKNOWN.**

### 3.5 What I conclude (BELIEVED)

efi-pstore is the only one of the two that survives a power cut, but it records only panics, it uses the same small store as the boot-loop counter, and a boot loop is exactly the situation where it fills up. **I propose not to rely on it** and to switch it off in the image kernel (`CONFIG_EFI_VARS_PSTORE=n`, or `efi_pstore.pstore_disable=1`), unless the owner wants "panic text survives a power cut" more than the safety of the counter. The decision is question 1.

## 4. RAM (ramoops)

### 4.1 What it is, and where to put it (SOURCE and TESTED)

SOURCE (`Documentation/admin-guide/ramoops.rst`, `fs/pstore/ram.c`, `fs/pstore/ram_core.c`, 6.12): ramoops keeps records in a block of RAM that the kernel does not use. Parameters: `ramoops.mem_address`, `ramoops.mem_size`, `ramoops.record_size`, `ramoops.console_size`, `ramoops.pmsg_size`, `ramoops.ecc`, `ramoops.max_reason`, `ramoops.mem_name`. Zones: several "dmesg" zones (one per dump, panic and by default also oops), one "console" zone (a circular copy of every printk line), optional "pmsg" and "ftrace". Dump zones are cleared at boot after the old content is saved for reading; the console zone is not cleared, it just continues. The kernel must be told that the memory is reserved, e.g. `memmap=1M$256M` (the `$` marks reserved) or `mem=...`, or `reserve_mem=1M:4096:oops ramoops.mem_name=oops` (kernel 6.10 and newer; the documentation itself says `reserve_mem` "may not always allocate memory in the same location, and cannot be relied upon").

Which address survives? **TESTED** with scenario `ra` (`ADDRS=... scenarios-ram.sh ra`, one run per address, 512 MiB guest, panic then `panic=5` restart, count of pstore files seen at boot 2):

```
== address 0x08000000: boot 2 sees: 3 pstore file(s); total bytes: 51050
== address 0x10000000: boot 2 sees: 3 pstore file(s); total bytes: 50883
== address 0x18000000: boot 2 sees: 3 pstore file(s); total bytes: 50883
== address 0x1c000000: boot 2 sees: 3 pstore file(s); total bytes: 50883
== address 0x1e000000: boot 2 sees: 0 pstore file(s)   (ramoops: uncorrectable error in header, x10)
```

So 128, 256, 384 and 448 MiB worked and 480 MiB did not. **Why: UNKNOWN.** BELIEVED (not checked): the firmware or the kernel loader uses that part of memory before the kernel can reserve it. This is why the address needs to be found by testing on each real board (UNKNOWN there), and why a quick check "is the old content valid?" has to be part of any real design. The other tests used 256 MiB.

### 4.2 Panic, kernel restarts itself (r1; TESTED)

```
bash tools/image/experiments/black-box/scenarios-ram.sh r1
  -append "console=ttyS0 loglevel=7 panic=5 memmap=1M$256M ramoops.mem_address=0x10000000 ramoops.mem_size=0x100000 ramoops.record_size=0x10000 ramoops.console_size=0x40000 ramoops.pmsg_size=0x10000 ramoops.ecc=1 bb.act=panic bb.end=off"
```

Boot 1 lines: `pstore: Registered ramoops as persistent store backend`, `ramoops: using 0x100000@0x10000000, ecc: 16`, `pstore: backend 'ramoops' already in use: ignoring 'efi_pstore'`. Boot 2:

```
-r--r--r-- 1 0 0 24338 console-ramoops-0
-r--r--r-- 1 0 0 26397 dmesg-ramoops-0
-r--r--r-- 1 0 0    80 pmsg-ramoops-0
console-ramoops-0 starts "[ 0.000000] Linux version 6.12.0 ..." and ends "[ 3.516247] Rebooting in 5 seconds.."  then "ECC: No errors detected"
dmesg-ramoops-0  starts "Panic#1 Part1" and ends "<0>[ 3.491035] Kernel Offset: ..."
pmsg-ramoops-0   "BB-PMSG: a message written by user space during boot 1"
```

The whole boot log of the crashed boot (24 KB) and the panic dump (26 KB) were kept, in one file each (the 64 KiB record size), not in 14 pieces.

### 4.3 Watchdog reset, no panic (r2; TESTED)

`-append "... panic=0 i6300esb.heartbeat=15 <ramoops words> bb.act=wdt bb.end=off"`, `-device i6300esb -watchdog-action reset`. Boot 2 found `console-ramoops-0` (24171 bytes) and `pmsg-ramoops-0`, and **no `dmesg-` file** (no panic happened, so no dump was written). The console file is the evidence of a hang. **Only this kind of test**: the kernel was alive and only the program that should feed the watchdog was missing. **A kernel that is itself frozen was not tested** (it would be the same watchdog path, but I did not run it): UNKNOWN.

### 4.4 Process kill, new QEMU (r3) and cold boot (r4; TESTED)

r3: panic with `panic=0`, `kill -9` of that QEMU's PID, new QEMU with the same `vars.fd` and disk: `0 pstore file(s); total bytes: 0`. r4: after r1, a clean `poweroff -f`, new QEMU on the same files: `0 pstore file(s)`. **RAM content does not survive a new QEMU process.** The QEMU RAM is created fresh; a real power cut would be the same (BELIEVED, not tested on hardware).

### 4.5 Other results (TESTED)

- **r5**, `reserve_mem=1M:4096:oops ramoops.mem_name=oops` and no address: worked (3 files at boot 2: 22399, 24380, 80 bytes). Only one boot configuration tried; the kernel text says it can move: BELIEVED it needs a test on each board.
- **r7**, a **healthy** `reboot -f` with no crash: boot 2 still found `console-ramoops-0` (24220 bytes) and `pmsg-ramoops-0`. So **every boot leaves a console record**, crash or not. The save step must therefore decide by the failure counter, not by "files exist".
- **r6**, both backends: `backend in use: ramoops`, no crash-GUID variables written.

### 4.6 What the RAM backend does not give (BELIEVED / SOURCE)

Nothing survives a power cut or a firmware that clears memory (the 480 MiB case shows firmware can clobber memory). `max_reason` defaults to oops and panic (SOURCE: `fs/pstore/ram.c`); I did not test an oops that does not panic (the image has `CONFIG_PANIC_ON_OOPS` off).

## 5. A watchdog reset in QEMU is a warm reset (TESTED, limited)

In e2 and r2 the i6300esb with `-watchdog-action reset` restarted the VM inside the same QEMU process, and RAM (at 256 MiB) survived, as in a panic restart. QEMU's reset does not wipe guest RAM, and OVMF did not wipe it either at the addresses that worked. **Whether a real watchdog reset (a chipset reset or a power-good drop) keeps RAM is UNKNOWN.**

## 6. What stage 0 and the recovery kernel could save

### 6.1 The idea (BELIEVED)

Stage 0 is the first program that runs (`image/stage0/init`); it already reads the failure counter before it adds one. If the counter says "the last boot did not finish" (N is 1 or more), then pstore holds the evidence of the failed boot. Stage 0 should, **before it does anything else that could fail**: mount pstore, copy its files into a capped folder on the config partition, then empty pstore. Proposed folder: `/config/hubos/blackbox/NNNN/` (a running number, because the clock of a crashed machine is not trusted), with `meta.txt`:

- the pstore files (`console-ramoops-0`, `dmesg-ramoops-0`, `pmsg-ramoops-0`), the **end** of each file kept if cut;
- `failed_boots_before_this_boot` (the counter, which stage 0 already has), kernel release, pstore backend, bytes saved, uptime;
- the boot-loop counter value is also in `meta.txt`; the counter itself stays where it is (EFI variables).

Limits proposed (BELIEVED, tuned by nothing): newest **8** folders, **64 KiB** per file, **256 KiB** per folder; so the config partition never holds more than about 2 MiB of evidence.

**Why every boot has to be handled:** the console zone is not cleared at boot, but the kernel reads its content into memory only at start, and (SOURCE: `fs/pstore/ram_core.c`) the dump zones are cleared once read, and a new crash overwrites old dumps. So if boot k+1 does not save what boot k left, the next crash can overwrite it.

### 6.2 Prototype (TESTED)

Files: `tools/image/experiments/black-box/blackbox-save.sh` (BusyBox sh only), the lab `lab-init` calls it with `bb.save=1`. Scenario **s1** (ramoops crash, `panic=5`; boot 2 mounts the second disk, an ext4 image standing in for the config partition, read-write; saves; unmounts):

```
BLACKBOX: saved 3 pstore file(s), 51351 bytes, into /cfg/hubos/blackbox/0001 (keeping the newest 8 boots)
```

Scenario **s2**: a **new QEMU** on the same disks (cold boot; RAM gone, `0 pstore file(s)`): `BLACKBOX: pstore is empty, nothing to save`, and the saved folder `0001` was still on the config disk (`sequence=0001`, `failed_boots_before_this_boot=1`, `pstore_backend=ramoops`, `bytes_saved=51786`, then its three files).

**Two things the test showed that I had not expected:**

1. **Plain `echo` from stage 0 is not recorded.** A line written with `echo` to the console (`BB-ONLY-STDOUT`) was absent from `console-ramoops-0`; a line written to `/dev/kmsg` (`BB-KMSG`) was present, in the console file and in the dmesg dump. Stage 0 prints its progress with `echo "STAGE0: ..."`, so it must also write those to `/dev/kmsg` (a one-line change in a helper). TESTED.
2. **The kernel log can hold secrets someone logs.** A test line `password=hunter2 token: abc123XYZ and Secret_Key=s3cr3t` written to `/dev/kmsg` was in the saved files; see the redaction test in section 7.

### 6.3 Where it must be written (BELIEVED, open)

Stage 0 mounts the config partition **read-only** today and the recovery kernel too (SOURCE: `image/stage0/init`, `image/stage0/recovery-init`). Saving needs a read-write mount for a moment. Two options: (A) stage 0 mounts it read-write only when there is something to save; risk: a damaged filesystem in the very boot where things go wrong (not tested); (B) stage 0 only copies pstore into RAM and the slot's `init` writes it later: simpler for the config partition, but loses the evidence when the slot never reaches `init`, which is the failure that matters most. The recovery kernel could in addition save to the **data** partition, which it already mounts read-write (SOURCE: `image/stage0/recovery-init`). Question 3. **Not tested:** a full or read-only config partition, power loss during the save, a crash loop writing every few seconds (wear and time).

## 7. `GET /v1/logs` of the recovery agent

SOURCE (repo, `tools/image/experiments/recoveryagent/agent.go`, `backend_hubos.go`): `GET /v1/logs` exists, **requires a signed request** (the test table in `agent_test.go` sends it without a signature and expects a refusal), and the real backend returns the **last 4096 bytes of `/run/recovery-agent.log`** as plain text. Nothing of the black box is in it today.

**Proposal (all BELIEVED, none implemented in the agent):**

- **Signed, not open.** Reason: kernel logs show hardware layout, addresses and software versions, and sometimes more. The agent already has the signature path, and `HUB-OS.md` says the API is signed requests with one-time nonces.
- **Format:** `text/plain; charset=utf-8`, sections in this order: a header line; the agent log tail (as today); then the saved boot folders **newest first**, each with its `meta.txt` and its files after a `### filename` line. Plain text so the owner can read it without a tool.
- **Size limit:** 64 KiB by default for the whole answer (the prototype script cuts with `head -c`), newest content first; a query `?boot=NNNN` for one older folder. Reason for 64 KiB: a 50 KB boot (the lab's) fits. The number is a guess: BELIEVED.
- **No secrets:** (1) no secret ever goes on the kernel command line or into a kernel message by Hub OS (a rule for our own code); (2) a filter on the way out as a second line of defence. TESTED on invented secrets: `tools/image/experiments/black-box/blackbox-logs.sh` printed

```
FAKE password=[REDACTED] token: [REDACTED] and Secret_Key=[REDACTED]
```

  for the line above, in both the console and the dmesg file. **This only catches words I thought of** (`pass(word)`, `secret`, `token`, `apikey`, `private key`, `credential` followed by `=`, `:` or a space). A secret with another name passes. BELIEVED not enough alone. The binary `pmsg` and dump files are `tr -d '\000'` filtered only.
- **Clear:** a signed `POST /v1/blackbox/clear` to delete the folders after the hub has them (not designed; the existing `clear-failures` is the model).
- The hub would show "last failed boot: ..." only when the agent says the machine is in recovery; hubd does not store the logs (HUB-OS.md: the hub is a client) unless the owner says so. Question 5.

## 8. Recommendation and questions

**Recommendation (BELIEVED):**

1. Add `CONFIG_PSTORE`, `PSTORE_CONSOLE`, `PSTORE_RAM` (and `PSTORE_PMSG`) to the slot and recovery kernels; switch efi-pstore off.
2. Reserve a small RAM block (1 MiB is enough here) and **test its address on each board** in December; use `memmap=` with a tested address, `reserve_mem` only if it proves stable.
3. Stage 0 writes its progress to `/dev/kmsg` and saves pstore to the config partition when the failure counter is 1 or more.
4. The recovery agent serves the saved folders on `GET /v1/logs`, signed, 64 KiB, redacted.

**Questions for the owner (this document decided none of them):**

1. Keep efi-pstore off (this proposal) or on (panic text survives a power cut, but can fill the variable store and stop the boot-loop counter)?
2. Is 1 MiB of RAM reserved on every machine acceptable? (the cost on machines with plenty of memory is BELIEVED to be small; not measured)
3. May stage 0 mount the config partition read-write for the save (option A in 6.3), or should the evidence go to the data partition or into RAM first?
4. How much evidence to keep: 8 boots of at most 256 KiB, as proposed?
5. Should hubd ever keep a copy of a machine's logs on the hub, or only show them while the machine is in recovery?
6. Should the recovery kernel arm a stricter rule, like saving even when the counter is 0 (a first crash after a clean boot)? Today with counter 0 nothing is saved by this proposal.

## 9. What needs real hardware (all UNKNOWN)

- Whether RAM contents survive a real warm reset, a watchdog reset and a power-good drop on the chosen AMD boards, and whether the firmware clears memory at start (the 480 MiB case shows firmware or loader can clobber a place; the cause is not known).
- Whether the chosen address stays valid after a firmware update or a change of memory modules.
- The size of the firmware's variable store, whether it is shared with the boot-loop counter, how much slower or worse the flash gets (wear) when records are written, and how the firmware handles a full store. OVMF's numbers (section 3) are not the board's.
- A real hardware watchdog (the tests used QEMU's i6300esb only).
- ECC memory behaviour with ramoops (the software ECC of ramoops was on in the tests and reported `No errors detected`; a real memory-controller ECC with a reset is separate).
- The time and risk of saving to the config partition on a real disk in a crash loop.

## 10. What I did not verify, plainly

- Nothing ran on real hardware. QEMU TCG only; QEMU did not crash in the runs reported here.
- A kernel that is frozen inside (not just a missing watchdog feeder), an oops that does not panic, `PSTORE_FTRACE`, and `printk.always_kmsg_dump` were not tested.
- The cause of the failing 480 MiB address.
- The size of the kernel with pstore, and the effect on boot time.
- The prototype scripts are not wired into stage 0, the recovery agent, `build-kernel.sh` or any test. The main image and `HUB-OS.md` are unchanged. The existing suite (`go test -tags qemu`) was not run.
- Redaction is tested on made-up secrets only.
- Source labels for kernel files come from the 6.12 tarball, not from the web pages (I did not fetch any web page).

## 11. Files

`tools/image/experiments/black-box/`: `config-check.sh` (section 2), `lab.frag`, `build-lab.sh`, `lab-init`, `lib.sh`, `scenarios.sh` (efi: e1 to e9), `scenarios-ram.sh` (ramoops: ra, r1 to r7, s1, s2), `blackbox-save.sh`, `blackbox-logs.sh`. To repeat: `WORK=<dir> tools/image/fetch-tools.sh`, download `linux-6.12.tar.xz` into `$WORK` and unpack it, `WORK=<dir> tools/image/experiments/black-box/build-lab.sh` (about 4 minutes), then `WORK=<dir> bash .../scenarios.sh e1` and so on. Logs and images stay out of git.
