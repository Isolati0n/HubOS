# PROPOSAL: the distro workshop (create, build, test, promote, roll back) and the declarative machine file with a preflight

**Status: PROPOSAL.** Nothing here is decided, built into an image, or a change to `HUB-OS.md`. `HUB-OS.md` wins if this file disagrees with it. Written 2026-10-05 on branch `hub-isolation-and-workshop` (from `origin/main`). The owner decided that the workshop exists; everything else below (stages, interfaces, file format, names) is a recommendation, and every choice that is the owner's is collected in section 10. No code for the workshop was written. The only thing run for this document is a handful of small checks, listed in section 9.

**Labels on every item** (as in the other proposals):

- **TESTED**: I ran it in this session; the command and the result are written next to it (section 9 collects them).
- **SOURCE**: I read it in a file of this repository or in a document, on 2026-10-05. The path is given.
- **BELIEVED**: my reasoning or memory. Not tested, not read in a source.
- **UNKNOWN**: nobody has checked; do not rely on it.
- **DESIGN**: a recommendation made here for the owner to decide. It is a proposal, not a fact about the repository.

**Helpers:** no helper agents were used. Everything below I read or ran myself.

---

## 0. Plain-words summary

1. **What the workshop is (SOURCE: the owner's decision, passed on in the task).** A set of screens in the VM host's own window (the hub only shows that window). In it the owner can make a new distro (or a new version of one), build it, try it in a virtual machine, and only then put it on a real machine. If the real machine does not like it, it goes back to what it had. Virtual machines always come before real hardware.
2. **Most of the machinery already exists.** The repository already builds a bootable image, writes a signed update bundle, boots it in QEMU, runs about thirty checks (update, rollback, recovery, boot-loop breaker, key rotation) and has a small program (`hubos-ctl`) that installs a bundle into the other slot and rolls back when the new system never says "I am healthy". (SOURCE: `docs/image.md`, `tools/image/`, `image/rootfs/usr/sbin/hubos-ctl`.) What does **not** exist: a way to run those steps for a machine other than the two hard-coded ones, a place that keeps the results, a small service the GUI can talk to, and any description of a machine's hardware.
3. **Recommendation (DESIGN): five stages with a gate between each.** Create (a machine file), Build (reproducible, twice), Test (in a VM, the distro-contract checks), Preflight (one report: how sure are we that this will boot on the real machine), Promote (the signed bundle goes to the machine's other slot as a trial boot; the machine confirms or rolls back by itself). Rolling back is always available: automatically (the trial never became healthy) or by the owner's decision (`hubos-ctl rollback`).
4. **The declarative machine file (DESIGN).** One file describes one machine's whole system: its hardware, kernel, base, packages (with hashes), services and settings. Today the same facts are spread over a `.build` file, a package list, a kernel fragment, a config folder and a service list (SOURCE: `image/machines/*.build`). The tools already read the kernel version, the kernel fragment, the module list, the package list, the snapshot date, the service list and the timeouts. They do **not** read any hardware description, any package hash, any kernel source hash, or any config-file content from a machine file (section 6.4). Section 6 sketches the file and shows the reference image written in it.
5. **The preflight (DESIGN).** Before anything goes to a real machine: check the file; build the image twice and compare; boot it in a VM with the same kernel binary and the same settings; run the distro-contract checks; check that the kernel contains the drivers the declared hardware needs; and write a "boot-confidence" report. The report says, area by area, what was proven in the VM, what was only checked by reading, and what a VM cannot show at all.
6. **What a VM cannot prove, said plainly (section 7.3).** That the real GPU starts and draws, that the real network card links up and gets an address, that the real disk controller is seen and gets the device name the tools expect, that the real watchdog chip exists in the kernel, that sensors and firmware behave. Section 9 shows that QEMU 8.2.2 has stand-ins for a generic NVMe disk, SATA and a few Intel network cards, but none for an AMD GPU, a Realtek network card or the AMD watchdog (TESTED). So **the first boot on real hardware always goes into the trial slot with automatic rollback**, and the report must never say "will boot".
7. **Rule (SOURCE: the owner, in the task): no machine runs systemd.** Every distro base the workshop offers must be non-systemd. The preflight refuses a machine file whose base or package list brings systemd in. (The repository's current distro-contract proposal says nodes may use systemd; this task's owner rule is newer and wins for the workshop.)
8. **First slice buildable here (section 8):** a command-line wrapper that creates a machine file from a template, builds and tests the reference image (`qemu-test`), and writes a results folder. It needs one real change first: the Go QEMU tests are hard-wired to two machines and must take the machine as an input. Not built here.
9. **Eleven questions for the owner are in section 10.** The biggest: who signs a promoted bundle when the real signing key never touches a network (section 5.6), and whether the workshop's package pins are the Ubuntu snapshot date plus a hash lock, or something stricter.

---

## 1. Constraints (SOURCE)

| Constraint | Where it comes from | Label |
|---|---|---|
| The workshop's GUI is in the VM host's own GUI; the hub only shows that window. The GUI is not built in this task. | the task; `HUB-OS.md` (VM host: "its own bespoke GUI in a window") | SOURCE |
| VMs always before real hardware. | the task | SOURCE |
| No machine runs systemd; distro bases must be non-systemd. | the task (owner rule); `HUB-OS.md` (the older wording is "the hub and the project's reference images") | SOURCE |
| The hub is a client; it runs no workloads, so builds and VMs run on the VM host (or another node), never on the hub. | `HUB-OS.md` core principles | SOURCE |
| A/B update and recovery plumbing are required on every machine; this project's tools are the reference implementation. | `HUB-OS.md` "What Hub OS is" | SOURCE |
| Master copies of the code and every built image live on the NAS. | `HUB-OS.md` | SOURCE |
| The update signing key's private half lives only on a device never connected to a network; until December only throwaway test keys exist. | `HUB-OS.md` | SOURCE |
| Hub design assumes a perfect network; the workshop's design effort goes to correctness of builds and tests, not to bandwidth or latency. | `HUB-OS.md` | SOURCE |
| Do not invent a new streaming protocol, no web dashboard as the hub's control plane. | `CLAUDE.md` | SOURCE |
| Secrets never go in git or in the machine file. | `CLAUDE.md` | SOURCE |
| QEMU here runs without KVM (software emulation): timing, GPU and performance cannot be measured in VMs here. | `docs/image.md` section 6; `HUB-OS.md` Phases | SOURCE |

---

## 2. What already exists and can be reused (SOURCE, read 2026-10-05 at `origin/main`)

| Piece | What it does today | What is missing for the workshop |
|---|---|---|
| `image/machines/NAME.build` (two files: `qemu-test.build`, `hub.build`) | Shell-readable `KEY=VALUE` file: kernel version and fragment, module list, package list file, snapshot date, service list, timeouts, recovery version and command line, overlay folder, config folder, extra parts, partition sizes, desktop environment. Read with `.` by the scripts (`tools/image/common.sh` `load_machine`). | No hardware part, no hashes, no config content, comments are the only documentation. A key that is missing is not an error unless it is one of four (`KERNEL_VERSION`, `KERNEL_FRAGMENT`, `PACKAGES`, `SNAPSHOT`). |
| `tools/image/fetch-tools.sh` | Downloads build and test tools (QEMU, OVMF, mmdebstrap, squashfs-tools, s6, signify, ...) as packages and unpacks them with `dpkg -x` into `$WORK/tools/root`. Nothing is installed. | Tool versions are not pinned or hashed (it takes what the Ubuntu archive offers that day: BELIEVED from the script, which runs `apt-get install --print-uris` against the current index). |
| `tools/image/build-kernel.sh` | Builds three kernels (slot a, slot b, recovery) from kernel.org source with `make tinyconfig` plus the machine's fragment; stage 0 built in; skips work when the inputs' stamp is unchanged; fixed build user, host and time for reproducibility. | The kernel tarball is downloaded with `curl` and **not checked against a hash** (SOURCE: the script has no sha256 check for `linux-$V.tar.xz`). |
| `tools/image/build-base.sh` | `mmdebstrap` of Ubuntu 24.04 (`noble`) from `snapshot.ubuntu.com/ubuntu/$SNAPSHOT` with the machine's package names, plus the apt pin that forbids the systemd packages. | Pins by **date**, not by per-package version and hash. The only ready-made base is Ubuntu (glibc, `mmdebstrap`). |
| `tools/image/build-root-image.sh`, `strip-root.sh`, `check-libs.sh`, `build-bundle.sh` | Make the squashfs root and the signed bundle (`manifest`, `manifest.sig`, two slot kernels, `rootfs.sqsh`). The build fails if a file has an unresolved library or a systemd program is left. | `build-root-image.sh` still has hub-specific code (`EXTRA_PARTS=hub`) inside the script. Flavors `unhealthy`, `hang`, `noinit`, `garbage` are test-only. |
| `tools/image/build-disk.sh` | Makes the GPT disk (boot, slot A, slot B, config, data) and the OVMF variable file; writes `node.conf` from the machine file. | Disk layout is fixed (five partitions). |
| `tools/image/qemu_test.go`, `hub_test.go`, `agent_test.go` (build tag `qemu`) | QEMU runner with hang and crash retry, throwaway keys, a host HTTP server for bundles, a serial-console driver, a monitor driver; about thirty checks (read-only root, first boot, bad bundles refused, signed update, rollback, recovery, boot-loop breaker, per-machine timeouts, key rotation, reproducible builds, ...). Whole `TestImage` took 52 to 57 minutes, `TestHubImage` 22 to 26 minutes in software emulation (SOURCE `docs/image.md` section 5). | **Hard-wired to two machines** (`TestImage` uses the default `qemu-test.build`; `TestHubImage` sets `image/machines/hub.build`). The tests build everything inside one temporary folder and throw the results away (or keep only the work folder when `HUBOS_IMAGE_WORK` / `HUBOS_HUB_WORK` is set); there is no results store and no machine-readable result (the output is `go test -v` text). |
| `image/rootfs/usr/sbin/hubos-ctl` (POSIX shell, ~290 lines) | `status`, `ensure-entries`, `update BASE [SLOT]`, `confirm`, `rollback REASON`, `clear-failures`. Verifies signature and hashes, writes the other slot, sets `BootNext`, confirm raises the floor and clears the failure counter. Works in the recovery kernel too. | Hard-codes the disk `/dev/vda` (`HUBOS_DISK` overrides it) (SOURCE `hubos-ctl` line 8; `docs/proposals/distro-contract.md` C1). Fetches bundles over plain `http://`. |
| Recovery kernel and recovery agent | The recovery kernel (separate, installed at confirm) exists and is tested. The network recovery agent (`GET /v1/status`, `POST /v1/install` (image-signature check only), `POST /v1/clear-failures`, `GET /v1/logs`) exists **only** as an experiment in `tools/image/experiments/recoveryagent/` and a TEST recovery kernel (test T18). | The agent is not in any image. A promotion that needs "install into a named slot from the hub" uses it, so it cannot be used in the real flow yet. |
| `docs/proposals/distro-contract.md` | What a distro must provide (items C1 to C18), a 12-check conformance test sketch (V1 to V12), 16 open questions. | It is a proposal: nothing in it was built. It still allows systemd on nodes (this task's rule overrides that for the workshop). |
| `docs/proposals/phase-b-image.md`, `docs/image.md` | Background and the test catalogue of the images. | - |
| `tools/fakenode/`, `tools/geninv/` | Stand-ins for machines (a listener on a port), used by the hub test. | Could serve as the "health stand-in" for node images (distro-contract Q16). |
| Reproducibility | Two builds of the root give identical hashes (test T09, SOURCE `docs/image.md` section 3); `SOURCE_DATE_EPOCH` fixed (`tools/image/common.sh`). | Proven for the reference images only. |

**Not found in the repository (SOURCE: searched `tools/`, `internal/`, `cmd/`, `docs/` on 2026-10-05):** any hardware profile format; any workshop, VM-host management code; a results store; a lock file for packages; any driver-versus-hardware check. The VM host's own management software does not exist yet (`HUB-OS.md` lists "how much management software the VM host needs on top of KVM" as unverified).

---

## 3. The flow: five stages and the gates between them (DESIGN)

```
 Create ──► Build ──► Test (VM) ──► Preflight ──► Promote (trial slot) ──► Confirmed
   │          │           │             │               │                      │
 machine   image +     contract      one report;     real machine           (or) rolled back
  file     bundle,     checks in     "what is        boots the new          automatically,
           twice,      QEMU          proven, what    slot ONCE; no          or by the owner
           compared                  is not"         confirm = back
```

A stage can be repeated. A later stage never starts if an earlier one failed for the same machine file hash. The gate rules below are DESIGN.

### 3.1 Create

- **What the owner does:** picks a template ("the reference image", "the hub", "an empty machine on base X"), names the machine, and edits the machine file (section 6) in the GUI, or by hand in a text editor.
- **What the workshop does:** writes `machines/NAME.toml` (the machine file) and an empty history. It checks the file at once (rules in section 6.5): a wrong file is refused with a plain message that names the line.
- **Gate to leave:** the file passes validation. Nothing else.
- **Existing parts:** none for the file (it is new). The `.build` files and the package/kernel-fragment files are the nearest thing, and the first slice can generate them from the machine file (section 8).

### 3.2 Build

- **What happens:** resolve every input to an exact version and hash (the lock, section 6.6); fetch the sources and packages the lock names; build the three kernels, the base root, the root image and the signed bundle (with a **throwaway test key**; see 5.6); do it **twice** in clean folders and compare the hashes.
- **Gate:** both builds give the same hashes for the kernels, the root and the manifest (the existing test T09 does this for the reference image: SOURCE `docs/image.md` section 3). If they differ the workshop shows which file differs and stops (BELIEVED to be the most common early failure for a new distro base; not measured).
- **Existing parts:** `build-kernel.sh`, `build-base.sh`, `build-root-image.sh`, `build-bundle.sh`, `build-disk.sh`. Missing: running them with the machine file as input and with a lock, and the double build as a step (it exists only inside the Go test).
- **Time:** reference image about 3.5 minutes for the three kernels (4 CPUs), 35 s for the base root, 13 s per bundle, from a warm tool folder (SOURCE `docs/image.md` section 5). A new distro base (a different libc, a source build) will take far longer; unknown (UNKNOWN).

### 3.3 Test (in a VM)

- **What happens:** boot the built image in QEMU (UEFI, a virtual disk, a watchdog device) and run the conformance checks of `docs/proposals/distro-contract.md` section 4 (V1 to V10; V11 and V12 wait for the node helper and the display stack). For a machine with a desktop (the hub) add the machine's own tests (the hub's H0 to H5).
- **Gate:** every check passes. A failing check keeps its evidence (the serial log, the failing command, screenshots for GUI machines) in the results store.
- **Existing parts:** all of V1 to V10 have an equivalent in `tools/image/qemu_test.go` for the reference root (SOURCE `distro-contract.md` section 4 table). Missing: running them against any machine file and any root image, and writing results as data.
- **VM stand-ins for the machine's hardware:** the VM should use the machine file's declared hardware class where QEMU has a stand-in (NVMe disk, SATA, an Intel network card) and say so in the report where it has none (section 7.3).

### 3.4 Preflight

- Section 7. It is the last stage in a VM. It produces **one report** the owner reads before deciding to promote. It re-uses the results of Build and Test of the **same machine-file hash**; it does not repeat them if nothing changed.

### 3.5 Promote

- **What happens:** the owner presses "Promote to MACHINE". The workshop (a) asks for a **signature** on the manifest (5.6), (b) puts the signed bundle on the NAS release store, (c) tells the machine to install it into its **other slot** and boot it once (the existing flow: `hubos-ctl update` sets `BootNext`; the new system must say "healthy" within the confirm timeout, else the machine reboots into the old slot: SOURCE `docs/image.md`, tests T05 to T06), and (d) shows the machine's state while it does. The hub never installs anything by itself.
- **How the install is triggered:** two ways exist in the plan. (1) The machine's own helper pulls the bundle (`hubos-ctl update http://...`) when asked. (2) The network recovery agent installs into a named slot when the machine is in recovery (planned; only a test version exists). The "ask the running machine to update itself" call is **not in the node helper API proposal** (`docs/proposals/node-helper-api.md` section 4 lists session, audio and clipboard calls; no update call): UNKNOWN how the workshop asks a running node; see question 4.
- **Gate:** the owner has read the preflight report **of this exact bundle hash**, and the machine is not busy (HUB-OS: "a machine refuses to update or restart while a game or long job is running"; SOURCE `HUB-OS.md`, the refuse-while-busy hook itself is not built: SOURCE `docs/image.md` section 6).
- **The rule that makes real hardware safe:** the first boot of a new release is **always** a trial boot of the other slot; the previous slot and the recovery kernel stay as they were. (SOURCE: the A/B design; TESTED only in QEMU: `docs/image.md`.)

### 3.6 Roll back

- **Automatic:** the trial boot never became healthy -> the confirm step reboots (`sync; reboot -f`) and the firmware falls back to the old slot because `BootOrder` was not changed; a hung init is caught by the watchdog the same way; three failed boots in a row start the recovery kernel (boot-loop breaker). (SOURCE: `distro-contract.md` C6 to C11; TESTED in QEMU: `docs/image.md` tests T05, T06, T14.)
- **Deliberate:** the owner presses "Roll back MACHINE". The workshop runs `hubos-ctl rollback REASON` on that machine (SOURCE: the command exists; the floor file `min_version` is lowered only by this command: `distro-contract.md` C18).
- **Recorded:** every promote and every rollback writes a line in the machine's history in the results store (5.4) with the bundle hash and the reason.
- **Not covered:** a release that boots and confirms but is wrong in a way the health check cannot see (for example no sound). The health check for a non-hub node is undefined today (SOURCE: `distro-contract.md` Q4). The workshop can only show what the check proves.

---

## 4. Where it runs (DESIGN)

All of it runs on the **VM host** (the machine whose job is OS development). The hub shows the VM host's GUI window and does nothing else. Builds and VMs are heavy; the hub never runs them (SOURCE: `HUB-OS.md`). The VM host's own distro is the owner's choice and its own business; the workshop needs only: a Linux kernel with KVM (for speed on real hardware), `qemu-system-x86_64`, enough disk for the work folders, and (until the tool folder is pinned) network access to the package snapshot.

Two kinds of VM are needed and they are different things (BELIEVED, from the repository: the image tests use UEFI + a raw disk + the watchdog device + serial; the "guests" of the VM host's GUI use libvirt-style management and remote-viewer):

1. **Test VMs** made by the workshop from a built image: short-lived, driven over the serial line, thrown away. This is what `qemu_test.go` does today.
2. **The owner's guests** (HUB-OS: ephemeral or persistent, opened with remote-viewer). A candidate distro may also be run as a guest for hands-on use before promotion. Not designed here.

---

## 5. Back-end design: a small API on the VM host (DESIGN)

Nothing in this section exists. Names are placeholders. I keep it as small as I can and I do not invent a network protocol: the GUI runs on the same machine as the service, so the first version has **no network surface at all**.

### 5.1 Parts

| Part | Job | Reuses |
|---|---|---|
| **Workshop service** (`workshopd`, a placeholder name; Go like `hubd`: BELIEVED fit, the owner's rule is only that `hubd` is Go) | Owns the results store and the job queue; validates machine files; starts jobs; answers the GUI. Does **no** building itself. | `internal/inventory`-style file reading (SOURCE: the repo already has a TOML reader with "unknown field is an error" rules) |
| **Job runner** | Runs one job at a time as a child process in its own work folder: lock, build, test, preflight. Kills it on its time limit. | `tools/image/*.sh` unchanged |
| **Test runner** | Boots a built image in QEMU and runs the checks; writes a result file per check. | `tools/image/qemu_test.go` made parametric |
| **Results store** | Folders and small JSON files (5.4). | - |
| **Promoter** | Signs (5.6), copies the bundle to the NAS, asks the target machine to install, watches its state, records the outcome. | `hubos-ctl`, the recovery agent API, the node helper API |
| **GUI** | Reads from and writes to the service only. Not built here. | - |

### 5.2 The API between the GUI and the service (DESIGN)

- **Transport:** a Unix socket owned by the VM host's desktop user (the same way `hubd.sock` works: SOURCE `internal/hub/ipc.go`, one JSON line in, answers out). BELIEVED to be enough because the GUI is on the same machine. If the owner later wants the workshop from another machine, that is a separate decision (question 3).
- **Jobs, not long calls.** Builds take minutes to hours, so a request returns a job id at once and the GUI asks later. Every call has a deadline (the rules of `docs/proposals/hub-isolation.md` section 7 apply here too: a deadline on every call, one owner per piece of state, versioned messages).
- **Calls (placeholders):**

| Call | Does | Returns |
|---|---|---|
| `machines.list`, `machines.get NAME` | Lists machine files with the state of their last build, test and preflight. | summaries |
| `machines.put NAME <file text>` | Validates and stores a machine file (atomic write, the old one kept in the history). | ok, or the list of errors with line numbers |
| `job.start {kind, machine, file_hash}` | `kind` is `lock`, `build`, `test`, `preflight` or `promote`. Refuses if the machine file changed since `file_hash`. | job id |
| `job.get ID`, `job.log ID [from]` | State (`queued`, `running`, `passed`, `failed`, `cancelled`, `timeout`) and the log text. | state, log |
| `job.cancel ID` | Stops the job's processes. | ok |
| `results.get MACHINE [build]` | The preflight report and the per-check results. | JSON |
| `promote.state MACHINE` | Target machine state: slot, release, trial or confirmed, and the result of the last promote. | JSON |
| `rollback MACHINE REASON` | Runs the rollback command through the machine's API. | ok |

- **Every message carries `"v": 1`**; a service that does not know the version refuses it by name (the same idea as `format = 1` in the inventory file: SOURCE `docs/inventory-format.md`).
- **One job of a kind per machine at a time**; builds of different machines may queue.

### 5.3 Build recipes

The recipe for the reference base is what the scripts already do. For the first version the **machine file is compiled into the existing inputs** (a `.build` file, a package list, the kernel fragment, a config folder) in the job's work folder, and the existing scripts run unchanged. This keeps the tested path and adds no new build language. (DESIGN; the compile step is the first code to write.)

A **distro base** is a recipe with three parts: how to make the root tree (`mmdebstrap` for Ubuntu and Debian: exists; `apk` for Alpine, `xbps` for Void, `dnf --installroot`, `pacstrap`: named in `distro-contract.md` section 2.2 but nothing was run), how to pin it (a dated archive where one exists: Debian, Ubuntu, Arch per `distro-contract.md`; otherwise a lock of package hashes), and which init it brings. Offering a base in the GUI means: it passes the conformance checks once, and it is **non-systemd** (the task's rule). Which bases qualify is the owner's choice (question 6); from `distro-contract.md` section 2.2 the non-systemd candidates are Alpine (OpenRC), Void (runit), Devuan (sysvinit, OpenRC or runit), Chimera (dinit), Gentoo (OpenRC), and Ubuntu or Debian with the apt pin the repository already uses for the hub (TESTED for Ubuntu 24.04: `docs/image.md` section 4). Whether each of them builds a root that passes the checks is UNKNOWN: nothing but Ubuntu was built.

### 5.4 The results store

A folder tree on the VM host, copied to the NAS when a run ends (HUB-OS: the master copies of every built image live on the NAS). All files are written the safe way (write a temporary file in the same folder, flush it, rename it, flush the folder), and each result folder has a `SHA256SUMS` file. (DESIGN. The hub-stability work found that driftwm's own writer does not flush and that `hubd`'s record file is written without a flush or a checksum: SOURCE `docs/proposals/driftwm-layouts.md` 5.4 and `internal/hub/record.go`. The workshop should not copy that.)

```
workshop/
  machines/NAME.toml                  the machine file (current)
  machines/NAME.history/              every earlier version, by hash
  runs/NAME/<machine-file-hash>/<run-id>/
      lock.toml                       exact versions and hashes that were used
      build-1.log, build-2.log        the two builds
      hashes.json                     kernels, root, manifest: build 1 and build 2
      bundle/                         manifest, kernels, rootfs.sqsh (signed with a TEST key)
      test/<check>.json               per check: name, result, seconds, evidence file names
      test/evidence/                  serial log, screenshots, the failing command
      preflight.json  preflight.txt   the boot-confidence report (machine and human form)
      SHA256SUMS
  promotions/NAME.log                 one line per promote or rollback: time, bundle hash, who, result
```

- A **result file never changes** after the run ends. A new run gets a new folder.
- Size cap per run and number of runs kept per machine are settings (question 9). The images are a few hundred MB each for the hub (the hub root is 152 to 160 MB: SOURCE `docs/proposals/from-scratch-hub.md`, `image/machines/hub.build`).

### 5.5 The test runner

Today the checks are Go tests that print `go test -v` text. DESIGN: make the machine, the root image and the work folder **inputs** (distro-contract Q15 asks the owner whether this is the form he wants), keep the retry logic for QEMU hangs and crashes (SOURCE: `qemu_test.go` `bootVM`, `noteHang`, `noteCrash`), and write one JSON file per check. A check is `{name, group (contract item), result, seconds, note, evidence[]}`. The existing check names (T01 ..., H0 ...) become the check names. The run records QEMU hangs and crashes it retried, so a "pass after retry" is visible and not hidden (the existing test already counts them).

### 5.6 Promotion and signing

- **Through the A/B update (the normal route):** the machine installs a signed bundle into its other slot and boots it once. SOURCE: the update tool and its tests.
- **Through the recovery agent (a machine that does not boot):** installs into a named slot from the hub. SOURCE: design in `docs/proposals/recovery-and-out-of-band.md`; only a test agent exists.
- **The signing problem (SOURCE: `HUB-OS.md`):** the owner's update signing key lives on a device that is never connected to a network. So the workshop **cannot** sign a bundle for a real machine by itself. Workshop builds in VMs use a **throwaway key** (as the existing tests do); a bundle for a real machine must be signed offline. DESIGN options, all needing the owner (question 1): (a) the workshop writes the unsigned manifest to a USB stick (or a QR-sized hash file), the owner signs it on the offline device and puts the signature back; (b) a second, lower-trust key for "workshop trial releases" that machines accept only in a trial slot (this changes the keyring rules: **not** proposed here, only named); (c) in December, while there is no real key, throwaway keys are used for everything and nothing here is secure (HUB-OS: security is out of scope for now).

---

## 6. The declarative machine file (DESIGN)

### 6.1 What it is

One file per machine that describes the machine's whole operating system, so that the same file always gives the same image, and so that a check can read the file and say what is wrong before anything is built. It is **declarative**: it says what the machine should be, not the steps to build it. The steps are the workshop's job.

Format: **TOML**, like the inventory (`docs/inventory-format.md`) and `viewers.toml`, with the same rules where they apply: a `format = 1` line, an unknown field is an error, a wrong type is an error with the line number. (DESIGN. TOML was chosen only because the repository already reads it with those rules; question 7.)

### 6.2 Sections

| Section | Says | Read by the tools today? |
|---|---|---|
| `[machine]` | id, purpose (a sentence), architecture, role label (text only), which release series | partly: the id becomes `NAME` in `node.conf` (SOURCE `build-disk.sh`, `NAME=hub-qemu` is hard-coded there, not read from the build file: so **not read**) |
| `[hardware]` | The hardware this machine has or will have: firmware, CPU family, memory, disks (bus, controller), network cards (chip), GPU (chip), watchdog (chip), sound, input. Each entry says what to emulate in a VM (if QEMU has a stand-in) and which kernel driver symbol is needed. | **No.** Nothing reads any hardware description. |
| `[kernel]` | version, the fragment (or the list of options), source hash, modules (out-of-tree), reason if it differs from the default, extra command line | version, fragment and module list: **yes** (`KERNEL_VERSION`, `KERNEL_FRAGMENT`, `MODULES`); source hash: **no** |
| `[base]` | distro family, release, builder, archive snapshot date, libc, init | snapshot date: **yes** (`SNAPSHOT`); the rest is implied (Ubuntu `noble`, `mmdebstrap`, s6 are fixed in the scripts) |
| `[[package]]` | name, version, sha256 of the file, why it is there | names: **yes** (`PACKAGES` file, one name per line); versions and hashes: **no** |
| `[[source]]` | things built from source: name, URL, commit or tarball hash, build command id, output path | **no** as data: driftwm's commit and eudev's tarball hash are written inside `tools/image/build-hub-parts.sh` (SOURCE) |
| `[services]` | the services started, in order or by dependency, with the user each runs as | list of names: **yes** (`SERVICES`); users and dependencies are inside the `run` scripts, not the build file |
| `[config]` | files placed on the config partition: name, content or source path, mode | the inventory, `viewers.toml` and `desktop.env` come from `CONFIG_DIR` and `DESKTOP_ENV` (**yes**); other files: no |
| `[update]` | confirm timeout, watchdog timeout, boot-fail limit, recovery version and command line, slot size, data size | timeouts, limit, recovery version/command line, sizes: **yes** (checked by `check-timers.sh`) |
| `[health]` | what "this boot is healthy" means: a command and how many times in a row | **partly**: the check is hard-coded in `image/rootfs/etc/s6/sv/confirm/run` (`hubd list` five times) |
| `[preflight]` | the checks to run, the VM stand-ins, the time limits | **no** |

### 6.3 Worked example: the reference image (`qemu-test`)

This is `image/machines/qemu-test.build` rewritten as a machine file. Every value comes from the repository files named in the comments. Values the repository does not have are marked `"<filled by lock>"` or `"<UNKNOWN>"`: **I did not invent any hash or version**.

```toml
format = 1

[machine]
id      = "qemu-test"
purpose = "the reference test machine: boots, updates, rolls back, recovers; no desktop"   # qemu-test.build header
arch    = "x86_64"                                                                         # manifest line "arch x86_64"
role    = "test"                                                                           # a text label only (HUB-OS.md)

[hardware]                      # NEW. What the tests' QEMU command line already provides (tools/image/qemu_test.go startVM)
firmware  = "uefi"              # OVMF_CODE_4M.fd + OVMF_VARS_4M.fd
cpu       = { family = "any", cores = 2 }
memory_mib = 1024
[[hardware.disk]]
id = "system"
bus = "virtio-blk"              # qemu -device virtio-blk-pci
needs_kernel = ["VIRTIO_BLK"]   # image/kernel/qemu-test.frag has CONFIG_VIRTIO_BLK=y
vm_standin = "virtio-blk-pci"
[[hardware.nic]]
id = "eth0"
chip = "virtio-net"
needs_kernel = ["VIRTIO_NET"]
vm_standin = "virtio-net-pci"
[hardware.watchdog]
chip = "i6300esb"               # qemu -device i6300esb ; stage 0 arms it
needs_kernel = ["I6300ESB_WDT", "WATCHDOG_NOWAYOUT"]
vm_standin = "i6300esb"

[kernel]
version  = "6.12"                                    # KERNEL_VERSION
source_sha256 = "<UNKNOWN: build-kernel.sh does not check one; the lock step would record it>"
fragment = "image/kernel/qemu-test.frag"             # KERNEL_FRAGMENT
modules  = []                                        # MODULES= (none; no tool builds out-of-tree modules yet)
# default kernel: this machine uses the build default, so no 'reason' line is needed (HUB-OS.md: a machine that overrides it gives a reason)

[base]
family   = "ubuntu"
release  = "noble"
builder  = "mmdebstrap"                              # tools/image/build-base.sh
snapshot = "20261001T000000Z"                        # SNAPSHOT
libc     = "glibc"
init     = "s6"                                      # image/rootfs/usr/sbin/init ends in exec s6-svscan
systemd  = "banned"                                  # image/apt/no-systemd.pref ; the build test fails if any systemd program is present

packages_file = "image/packages/qemu-test.list"      # PACKAGES (names only; see [[package]] for the lock)
# [[package]] entries are generated by `lock`, one per resolved package. The versions below are what the hub build (same snapshot) in this
# session installed from the snapshot (TESTED: read from var/lib/dpkg/status of the built base root); the hashes are NOT known
# (the built root keeps no package index or .deb files), so they stay placeholders:
[[package]]
name = "s6"
version = "2.12.0.3-1build1"
sha256 = "<filled by lock>"
[[package]]
name = "execline"
version = "2.9.4.0-1build2"
sha256 = "<filled by lock>"
[[package]]
name = "busybox-static"
version = "1:1.36.1-6ubuntu3"
sha256 = "<filled by lock>"
[[package]]
name = "libc6"
version = "2.39-0ubuntu8"
sha256 = "<filled by lock>"

[services]
start = ["hubd", "console", "confirm", "wd"]         # SERVICES (image/rootfs/etc/s6/sv/<name>/run)

[health]
# today hard-coded in image/rootfs/etc/s6/sv/confirm/run
command = "hubd list --socket /run/hubos/hubd.sock"
passes_in_a_row = 5
interval_seconds = 1

[update]
confirm_timeout_seconds  = 30                        # CONFIRM_TIMEOUT   (test values; real machines 120)
watchdog_timeout_seconds = 60                        # WATCHDOG_TIMEOUT  (test values; real machines 180)
boot_fail_limit          = 3                         # BOOT_FAIL_LIMIT
recovery_version         = 1                         # RECOVERY_VERSION
recovery_cmdline         = "console=ttyS0 ro loglevel=4 panic=5"
slot_mib = 64                                        # default in build-disk.sh (SLOT_MIB unset in qemu-test.build)
data_mib = 160

[config]
dir = "image/config/qemu-test"                       # inventory.toml, viewers.toml (CONFIG_DIR default in build-disk.sh)

[preflight]
checks = ["V1","V2","V3","V4","V5","V6","V7","V8","V9","V10"]   # distro-contract.md section 4
build_twice = true
vm_standins = "all-declared"
```

For the **hub** the same file would add `[[hardware.gpu]]` (in the test: `virtio-vga`, kernel symbol `DRM_VIRTIO_GPU`; on the real hub an AMD chip, symbol `DRM_AMDGPU`, firmware blobs: UNKNOWN until December), a `[[source]]` entry for driftwm (commit `352333a8fa1b22171492d4b71a54102045c9a19d`, SOURCE `tools/image/build-hub-parts.sh`) and for eudev (version `3.2.14`, tarball sha256 `8da4319102f24abbf7fff5ce9c416af848df163b29590e666d334cc1927f006f`, same file), `[services]` with the hub's nine services and their users, and `[config]` files `desktop.env`, `waybar.json`, `wofi.css` (SOURCE: `image/machines/hub.build`, `image/machines/hub/rootfs/etc/hubos/`).

### 6.4 Which parts the existing tools already read (summary)

| Part of the file | Existing reader | Needs new code |
|---|---|---|
| kernel version, fragment, module list | `build-kernel.sh`, `build-bundle.sh` (manifest `kernel-version`, `modules`) | the machine-file reader that writes the `.build` file |
| package names, snapshot date | `build-base.sh`, `build-hub-parts.sh` | versions and hashes (the lock) |
| services | `build-disk.sh` -> `node.conf` -> stage-1 `init` | users, dependencies |
| timeouts, boot-fail limit, recovery version/command line, sizes | `check-timers.sh`, `build-kernel.sh`, `build-disk.sh`, stage 0 | nothing |
| config directory, desktop environment | `build-disk.sh` | content of other files |
| source builds (driftwm, eudev) | `build-hub-parts.sh` (values hard-coded inside) | moving them into data |
| health check | `confirm/run` (hard-coded) | making it a setting |
| hardware | nobody | everything (profile table, VM stand-ins, driver check) |
| package hashes, kernel source hash, tool hashes | nobody | the lock step |
| preflight section | nobody | the preflight |

### 6.5 Validation rules (the first stage of preflight; DESIGN)

Same style as the inventory validator (SOURCE: `docs/inventory-format.md`, `testdata/broken/`): each rule has a deliberately broken sample file in the tests.

1. `format` is known; unknown field is an error; wrong type is an error with the line.
2. `[machine].id` is a valid lower-case id, unique among machine files.
3. The base is not a systemd base: the base family is on the allowed list, `init` is not `systemd`, and no package name in the lock is one of the banned ones (`systemd`, `systemd-sysv`, `libpam-systemd`, `dbus-user-session`, `udev`, `systemd-timesyncd`, `systemd-resolved`: SOURCE `HUB-OS.md`, `image/apt/no-systemd.pref`).
4. `watchdog_timeout > confirm_timeout + 15 s` (SOURCE: `check-timers.sh`, reuse the script).
5. Every `[[package]]` has a version and a 64-hex sha256; no duplicates; the lock has an entry for every name in the package list.
6. `[kernel].source_sha256` is present and 64-hex (once the build checks it).
7. Every `needs_kernel` symbol in `[hardware]` exists in the machine's finished kernel `.config` as `=y` (or `=m` with a module listed): this is the driver check of section 7.
8. Every service named in `[services]` has a `run` file in the root or the overlay.
9. No secret: a key whose name matches `password|secret|token|key` with a non-empty value is refused (SOURCE rule: `CLAUDE.md` "secrets never go in the inventory file"; the exact pattern is DESIGN).
10. If `[kernel].version` differs from the build default, `reason` must be present (SOURCE: `HUB-OS.md` kernel paragraph).

### 6.6 The lock

The `lock` job turns the file's names into exact things: for each package, the version and the sha256 of the `.deb` (or the archive's file) from the pinned snapshot's index; for the kernel the tarball sha256; for each `[[source]]` the commit or tarball hash; for the build tools the versions and hashes that `fetch-tools.sh` took. It writes `lock.toml` into the run folder and, when the owner accepts it, back next to the machine file (so the lock is in git and reviewable). The build then **fails if any downloaded file's hash differs from the lock**. This is what "pinned packages with hashes" means in practice. (DESIGN. UNKNOWN: whether the snapshot's package index carries the sha256 of every file in a form `mmdebstrap` can verify; I did not check the snapshot service's index format.)

---

## 7. The preflight (DESIGN)

### 7.1 Steps

Run in this order; stop at the first step marked STOP.

| # | Step | Result | STOP if it fails |
|---|---|---|---|
| P0 | **Validate** the machine file (6.5). | file ok / list of errors | yes |
| P1 | **Lock**: resolve every input to a version and a hash; compare with the lock in git if there is one. | lock; a diff if it changed | yes if a hash changed with the same version |
| P2 | **Build twice** from clean folders with the lock; compare every output hash (kernels, root, manifest). | same / differs, which file | yes |
| P3 | **Static checks on the built image**: no systemd program or unit (existing `checkNoSystemd`), every executable's libraries found (existing `check-libs.sh`), `/sbin/init`, `/etc/hubos-release`, the keyring, the recovery kernel are present (distro-contract V1). | pass / fail list | yes |
| P4 | **Driver check**: read the finished kernel `.config` and compare with `[hardware]` (7.2). | per device: present built-in / present as module / MISSING | yes on MISSING; "UNKNOWN symbol" is a warning |
| P5 | **Boot in a VM with the machine's real kernel and config**: the same `kernel-a.efi` that would be shipped, the real `node.conf` and config partition, the declared devices replaced by QEMU stand-ins where one exists (7.3). Run V2 to V10 (boot, settings, confirm, rollback of an unhealthy second release, hung init, feeder, signed update, refusals, recovery). | per check | yes |
| P6 | **Update-path check**: from a fresh install of the previous confirmed release of this machine (if there is one), update to this release, confirm; then update to a deliberately unhealthy one and see the rollback (reuses V4, V5, V8). | pass / fail | yes |
| P7 | **Report** (7.4). | `preflight.json`, `preflight.txt` | - |

Time: P2 doubles the build time; P5 to P6 are the 22 to 57 minute test suites of today in software emulation (SOURCE `docs/image.md` section 5), less with KVM (BELIEVED, and the VM host has hardware virtualisation: UNKNOWN until it exists). A preflight is therefore a job of one to several hours, not a click.

### 7.2 The driver check (what it can and cannot say)

- **What it does (DESIGN; the idea tested in section 9):** for each hardware entry the file lists the kernel option symbols the driver needs. The check reads the **finished** kernel configuration (`$WORK/out/kernel.config` is written by `build-kernel.sh`: SOURCE `build-kernel.sh` header) and reports each symbol as `y`, `m` or missing. Stage 0 cannot load modules (distro-contract C6: "drivers for the machine's disk, network and watchdog built in (no modules in stage 0)"), so for the **disk, the network card used by the recovery kernel, and the watchdog** only `=y` counts.
- **What it cannot say:** whether the listed symbols are the right ones for the real chip. That is a table somebody must write from the kernel's own documentation and then confirm on the real machine. For the real AMD boards the table is UNKNOWN until the boards are chosen (December). The check proves "the kernel was built with what the file says it needs", never "this chip works".
- **Firmware blobs** (GPU, some network cards) are files outside the kernel. The file needs a `firmware = [...]` list per device, and the check looks for the files in the image. Not designed further here (UNKNOWN).
- **Hardware the file forgets.** The check can only test what is declared. A second check, on the real machine's first boot in recovery or the trial slot, can list the PCI and USB devices the kernel sees and compare them with the file (BELIEVED to be practical: `lspci`-style listing of `/sys/bus/pci/devices`); the result is stored with the promotion. Not built.

### 7.3 What a VM cannot prove (plainly)

| Thing | What the VM can show | What it cannot show | Label |
|---|---|---|---|
| **Real GPU** | The kernel's DRM stack starts with a virtual GPU (`virtio-vga`); a software-drawn desktop works (the hub test does this). | That the real AMD GPU's driver loads, its firmware is present, it drives the projector, it survives a GPU reset, decode and rendering speed, heat. QEMU 8.2.2 lists no AMD GPU device. | SOURCE `docs/image.md` section 6; TESTED (section 9: no match) |
| **Real network card** | The network stack and DHCP work with `virtio-net`; QEMU also offers Intel `e1000`, `e1000e`, `igb`, `rtl8139` and `vmxnet3` stand-ins. | That the real chip's driver binds, its firmware loads, the link comes up at 10 Gb/s, wake and offload quirks. QEMU lists no Realtek r8169/r8125 or Intel 10G device. | TESTED (section 9) |
| **Real disk controller** | A generic NVMe disk (`-device nvme`) and SATA (`ich9-ahci`) can replace `virtio-blk`, proving the kernel has those drivers and that tools do not assume `/dev/vda`. | Real controller quirks, real device naming on the board, sector size, power-loss behaviour, wear, real speed. **Known problem:** `hubos-ctl` assumes `/dev/vda` unless `HUBOS_DISK` is set (SOURCE `hubos-ctl` line 8, distro-contract C1), so the first NVMe preflight will fail on exactly this, which is a good thing to find in a VM. | TESTED (section 9: devices exist); SOURCE (`hubos-ctl`) |
| **Real watchdog** | The reset logic with `i6300esb`. | That the board's watchdog chip has a driver in the kernel (`sp5100_tco` is BELIEVED for AMD boards; not confirmed in `hub-stability.md` either), that it can be armed from stage 0, that its timeout range fits. QEMU 8.2.2 offers `i6300esb` and `ib700`, nothing AMD. | SOURCE `docs/proposals/hub-stability.md` H5; TESTED (section 9) |
| **Firmware (UEFI)** | The boot entries, `BootNext`, `BootOrder` and the failure counter work in OVMF. | That the real firmware keeps those variables, how full its variable store gets, boot-menu behaviour, a CMOS reset. | SOURCE `docs/image.md` section 6 |
| **Sensors, fans, ECC, PCIe errors, power** | nothing | all of it | BELIEVED |
| **Timing and performance** | nothing reliable (software emulation) | boot time, jitter, frame times | SOURCE `HUB-OS.md` Phase B |
| **USB devices** (keyboard, mouse) | `qemu-xhci` with `usb-kbd` and `usb-mouse` (the hub test uses them). | a real controller's quirks | SOURCE `hub_test.go` |

So the report can honestly say: **"this image boots, updates and rolls back in a virtual machine with generic devices, and the kernel contains the options the file says the hardware needs"**. It cannot say "it will boot on the real machine".

### 7.4 The boot-confidence report (DESIGN)

One page, plain words, written next to a JSON form. Each row has one of four marks. The wording of the overall line is fixed so it cannot over-promise.

```
Machine: gaming-1      Release: 12      Bundle sha256: 3f9a…      Machine file: 91c0…
Built twice, same result:                     YES   (kernel a, kernel b, root, manifest)
No systemd anywhere in the image:             YES
Booted in a VM with this machine's own kernel and settings:   YES  (2 CPUs, UEFI, NVMe disk, e1000e network, i6300esb watchdog)
Update, rollback, recovery, boot-loop breaker in the VM:      YES  (V1 to V10: 10 of 10)

Checked by reading only:
  Disk driver in the kernel (NVMe): built in .................. YES
  Network driver (r8169): built in ........................... MISSING   <-- stops the promotion
  GPU driver (amdgpu) and its firmware files in the image .... CONFIG yes, firmware: not checked

NOT proven by a VM (no stand-in exists in QEMU 8.2.2):
  the real GPU, the real network chip, the real watchdog chip, the real firmware, sensors, speed

Overall: NOT READY / READY FOR A TRIAL BOOT ON THE REAL MACHINE
         ("ready for a trial" means: the first boot will go into the spare slot and come back by itself if it fails.
          It does not mean the machine will boot.)
```

Marks: `PROVEN-IN-VM`, `CHECKED-BY-READING`, `NOT-PROVABLE-IN-VM`, `FAILED`. The overall line is `READY FOR A TRIAL BOOT` only if there is no `FAILED` and every `needs_kernel` symbol is present. It is **never** "will boot".

### 7.5 After the preflight: the first boot on real hardware

Always a trial boot in the spare slot with the automatic rollback described in 3.6; this is not optional and the GUI has no button to skip it (DESIGN). On the very first install of a new machine (no previous slot) there is nothing to roll back to, so the first install goes through network boot and the recovery kernel (SOURCE: `HUB-OS.md` "first install of a new machine: network boot"; UNKNOWN: how the workshop would drive a first install; question 5). What the trial boot adds on the real machine that the VM could not: a listing of the devices the kernel found, compared with `[hardware]` (7.2), saved with the promotion record.

---

## 8. A first slice buildable here (described, not built)

**Goal:** from a command line on the build machine: create, build and test the **reference image** from a machine file, and keep the results. No GUI, no service, no promotion.

**Commands (placeholders):**

```
workshop create NAME --from qemu-test     # copy the template machine file to machines/NAME.toml and check it
workshop check  NAME                      # P0: validate (rules 6.5)
workshop lock   NAME                      # P1: write lock.toml (versions and hashes)
workshop build  NAME                      # compile the file into .build, package list, fragment; run the build scripts twice; compare
workshop test   NAME [--checks V1..V10]   # boot in QEMU, run the checks, write test/*.json
workshop preflight NAME                   # P0 to P7 in order; write preflight.txt and .json
```

**What it needs (in order):**

1. A reader for the machine file (Go, next to `internal/inventory`, same rules): small, the part with tests of broken files is the bulk of the work. (BELIEVED: a day or two for a person; nothing measured.)
2. A compile step: machine file -> `.build` + package list + fragment + config folder in a work folder (the scripts then run unchanged, `MACHINE=` already selects a build file: SOURCE `common.sh`).
3. Make `qemu_test.go` take the machine (`MACHINE`) and an output folder as input, and write a JSON result per check. This changes a tested file, so it needs the owner's approval of the approach (distro-contract Q15) and a full QEMU run (about 52 minutes here) before merging (SOURCE `docs/image.md` section 2: the QEMU tests are run by hand before merging any change to `tools/image/`).
4. The lock step, which needs the snapshot index format (UNKNOWN today).
5. The driver check: a table from hardware chip to kernel symbols plus a function that reads `kernel.config` (section 9 shows a throwaway version).

**Trivial enough to build now?** No. Step 3 changes tested code and takes an hour of QEMU to verify; steps 1 and 2 are real code. Section 9 contains the only part I ran: the driver check idea, as a throwaway script.

**What it does not do:** promotion, signing for real machines, any distro other than Ubuntu 24.04, any hardware class other than QEMU's.

---

## 9. What I ran for this document (all TESTED, 2026-10-05, QEMU 8.2.2 tools unpacked by `tools/image/fetch-tools.sh` into a temporary folder; no package was installed)

**9.1 What QEMU 8.2.2 offers as stand-ins (TESTED).** Command: `qemu-system-x86_64 -L <tools>/usr/share/qemu -device help` with the QEMU unpacked by `fetch-tools.sh`. Result: 421 devices listed. Present: `nvme` ("Non-Volatile Memory Express"), `ich9-ahci` (SATA), `megasas`, `usb-storage`, `qemu-xhci`; network `e1000`, `e1000e` (Intel 82574L), `igb` (Intel 82576), `rtl8139`, `vmxnet3`; GPU `virtio-gpu-pci`, `virtio-vga`; watchdog `i6300esb` and `ib700`. A case-insensitive search of the list for `r8169|rtl8169|ixgbe|i40e|amdgpu|radeon|sp5100|k10temp|nouveau|nvidia` found **0** matches. So a generic NVMe disk and an Intel network card can be tried in a VM; the machine's real Realtek or 10 Gb/s network chip, its AMD or NVIDIA GPU and the AMD watchdog cannot.

**9.2 The driver check, as a throwaway script (TESTED).** Files: `tools/image/experiments/hub-isolation/driver-check.sh` (about 25 lines of shell), `profile-hub-qemu.txt`, `profile-example-amd-board.txt`. The kernel `.config` is the hub image's own, built in this session by `tools/image/build-kernel.sh` (`kernel.config`, 3224 lines). Commands and output:

```
$ sh driver-check.sh kernel.config profile-hub-qemu.txt        # exit status 0
disk virtio-blk            CONFIG_VIRTIO_BLK      y
disk virtio-blk            CONFIG_VIRTIO_PCI      y
network virtio-net         CONFIG_VIRTIO_NET      y
watchdog i6300esb          CONFIG_I6300ESB_WDT    y
watchdog i6300esb          CONFIG_WATCHDOG_NOWAYOUT y
gpu virtio-vga             CONFIG_DRM             y
gpu virtio-vga             CONFIG_DRM_VIRTIO_GPU  y
usb xhci keyboard/mouse    CONFIG_USB_XHCI_HCD    y
usb xhci keyboard/mouse    CONFIG_USB_XHCI_PCI    y
usb xhci keyboard/mouse    CONFIG_USB_HID         y
usb xhci keyboard/mouse    CONFIG_INPUT_EVDEV     y

$ sh driver-check.sh kernel.config profile-example-amd-board.txt   # exit status 1
nvme disk                  CONFIG_BLK_DEV_NVME    MISSING
nvme disk                  CONFIG_NVME_CORE       MISSING
network Realtek r8169      CONFIG_R8169           MISSING
watchdog AMD FCH TCO       CONFIG_SP5100_TCO      MISSING
watchdog AMD FCH TCO       CONFIG_WATCHDOG_NOWAYOUT y
gpu AMD amdgpu             CONFIG_DRM_AMDGPU      MISSING
```

What this shows and does not show: the check works as an idea and is easy (a symbol lookup in the finished config). The first profile is the hub image's own QEMU devices, so it passes by construction. **The second profile is an invention:** I wrote it to show the output when drivers are absent; its option names are from my memory (BELIEVED, not checked in the kernel's `Kconfig`) and no board is chosen. The table that maps a real chip to its option names does not exist and must be written and checked against the kernel's documentation and the real machine in December. The repository's own hub kernel has none of those four drivers, which is the expected state today (`image/kernel/hub.frag` only covers QEMU's devices; SOURCE `distro-contract.md` section 3.2 item 6).

**9.3 Other facts read while checking (TESTED unless said):** the hub kernel `.config` has `# CONFIG_CGROUPS is not set`, `# CONFIG_PSI is not set` and `# CONFIG_USERFAULTFD is not set` (grep of the file above); the package versions in the section 6.3 example are those in `var/lib/dpkg/status` of the base root built in this session; the built root keeps no package index and no `.deb` files, so a package hash cannot be recovered from it afterwards (the lock step has to record them while building).

**9.3b A reproducibility hint (TESTED, weak).** The hub image's slot kernels built in this session (a fresh container, a different day's run) have the first 16 hex digits of their SHA-256 `a 51dce7dc065a7d17`, `b 6195ec7024e1b520`, the same digits an earlier hub-test log from another helper's session shows (`build-kernel.sh ... sha256 a 51dce7dc065a7d17 b 6195ec7024e1b520`). It is a hint for the kernels only, and the recovery kernel differs because it carries the run's own throwaway key (as `docs/image.md` section 5 already says). It does not replace the double-build comparison of section 7.1 step P2.

**9.4 What I did not run:** the machine-file reader, the compile step, the lock, the preflight as a whole, any workshop service, any second distro base.

---

## 10. Questions for the owner (nothing is decided here)

1. **Signing.** The real update key never touches a network. How should a workshop-built bundle for a real machine get its signature: (a) the owner signs on the offline device each time (a stick or a hash file carries the manifest across), (b) a separate lower-trust "workshop trial" key that machines accept only for a trial slot (this weakens the single-key rule), or (c) throwaway keys only until the real key exists? (Section 5.6.)
2. **Is the lock enough?** Pin = the Ubuntu snapshot date plus a hash for every package, the kernel tarball and every source build (section 6.6), and fail the build on any mismatch. Or do you want the build tools pinned the same way (`fetch-tools.sh` takes whatever the archive offers today)?
3. **Local only?** The workshop service is reachable only on the VM host (a local socket), because the GUI is there. Do you ever want to drive it from another machine? That would need the signed-request scheme of the node helper and a decision about the port.
4. **How does a running machine get told to update?** The node helper API as designed has no update call, and the recovery agent installs only when the machine is in recovery. May the workshop add an "update now from this URL" call to the node helper API (and a "busy, refuse" answer), or should the owner start the update at the machine?
5. **First install of a new machine.** Network boot is the owner's decision. Should the workshop be able to serve the first image to a machine that boots from the network, or is that a separate tool?
6. **Which bases may the workshop offer?** Only Ubuntu 24.04 with the systemd pin (the only one built and tested) for now, or also try Debian, Alpine, Void, Devuan, Chimera, Gentoo (each is a separate piece of work and none was built)? Remember the rule: no systemd on any machine.
7. **File format.** TOML like the inventory, with the file split into one machine file plus a generated lock file? Or one file that contains the lock too? Or something else?
8. **Health check for machines other than the hub.** The preflight and the confirm step need a definition of "this boot is healthy" per machine (distro-contract Q4). What should the default be for a node: the node helper answers, the display server's port accepts, both, or a command in the machine file (section 6.2 `[health]`)?
9. **How long to keep results.** Number of runs per machine and total size on the VM host; the NAS keeps the master copy of every promoted bundle. A hub-sized run folder is a few hundred MB.
10. **Who may press Promote.** Only the owner. Should it also be refused automatically while the target is busy (a game or long job running) once the busy hook exists?
11. **Extra preflight tests with real value in a VM**: a second boot-loop and power-cut test (kill QEMU in the middle of an update; the existing test T08 does this for the reference), and a soak (hours) with the final kernel. Do you want them in every preflight (hours) or only on request?

---

## 11. What I did not verify

- **Nothing in sections 3 to 8 is built.** The workshop service, the API, the machine-file reader, the compile step, the lock step, the preflight and the report do not exist. Names, call shapes and the file layout are recommendations.
- **No distro other than Ubuntu 24.04 was built or run** for this document. Statements about other bases come from `docs/proposals/distro-contract.md`, which has the same limit.
- **The snapshot service's package index** was not read, so whether per-package sha256 values can be checked by `mmdebstrap` from it is UNKNOWN (section 6.6).
- **The VM host** and its management software do not exist; the "hardware virtualisation makes it faster" statement is BELIEVED.
- **Real hardware** was not available and cannot be (the task and `HUB-OS.md`). Everything in section 7.3 about what a VM cannot show is from reading the repository's own limits and from the QEMU device list in section 9; the claim that a particular real chip needs a particular kernel symbol is UNKNOWN until the boards are chosen.
- **Time estimates** for new distro bases, for the lock and for the preflight on the VM host are guesses, except the reference-image numbers copied from `docs/image.md` section 5.
- **No helper agents were used.**
