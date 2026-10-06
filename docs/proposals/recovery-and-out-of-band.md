# PROPOSAL: network recovery agent and out-of-band options

Requests are not signed or encrypted (owner decision, 2026-10-06); only the image signature is checked.

**Status: PROPOSAL.** Nothing here is decided and nothing here is in the images. `HUB-OS.md` wins if this file disagrees with it. The only code is a small experiment in `tools/image/experiments/recoveryagent/` (a fake backend, and a real backend that runs `hubos-ctl`, used only in a TEST recovery kernel). Written 2026-10-04; rewritten 2026-10-06 after the owner removed request signing (see the History at the end).

**Labels used on every item:**
- **TESTED**: I ran it in this build environment; the exact command and output are shown.
- **SOURCE**: read in an official source; the link and the date I read it (2026-10-04) are given. Several web pages were read through a fetch tool that summarises the page with a small model, so the wording is not guaranteed to be the page's own; those are marked "(summarised)". Anything that matters for a decision should be re-read by a person on the page itself.
- **BELIEVED**: I think it is true but I did not test it and have no official source.
- **UNKNOWN**: nobody has checked; do not rely on it.

---

## Plain-words summary

1. **The problem.** Today, if a rack machine cannot boot, the recovery kernel starts a bare shell and waits for a person at a keyboard. Rack machines have no keyboard or screen (the recovery shell on a screen is also untested; the tests use a serial line). So recovery as built needs the owner to carry a keyboard and monitor to the rack.
2. **Layer 1, in software (cheap, no new hardware).** Put a small network program (the "recovery agent") in the recovery kernel. It answers "I am in recovery", and it can install a bundle into a named slot, clear the failure counter and hand over logs. Requests are plain: nobody signs them. What protects the machine is the **image signature**: the install request runs `hubos-ctl update`, which refuses any bundle that is not correctly signed by the owner. The hub then shows "in recovery" in the panel and can offer "reinstall". This helps when the **firmware and the recovery kernel work**.
3. **What I built to check the idea.** A tiny Go program with the four calls (status, install, clear failures, logs). Its unit tests pass (TESTED, section 2.6) and it runs on loopback. In a TEST recovery kernel under QEMU it ran earlier in a version that signed requests (`docs/image.md` test T18); the unsigned version of that QEMU test has **not been run yet** (PR #69 says so). It is **not** in any real image. Whether it works on real hardware is **not tested**.
4. **Layer 2, out-of-band (hardware).** For a machine whose firmware or kernel is dead, software on that machine cannot help. The options are: do nothing and walk over (free, fits "the rack is next to the desk"); a relay or switched power strip (can power cycle, sees nothing); serial console servers (see boot text and the recovery shell, need a serial port on each board); PiKVM (sees the screen, types, powers, mounts a virtual disk; the most capable and most expensive, about two cables per machine plus a switch box per four machines). The owner decides in December.
5. **Proprietary BMCs** (the management chips on server boards) run closed firmware. One of them (AMI MegaRAC) had a bug that let anyone on the network skip the login; it is in the US government's list of vulnerabilities attackers are using. OpenBMC, the open alternative, exists but only for server boards, and it is built on systemd, which this project bans.
6. **Biggest honest limits.** None of these gives board sensors (temperatures, fans); those would have to come from the OS through the node helper. A PiKVM also holds full control of every machine it is wired to, so it needs its own isolated management network and secret handling that is not designed yet.
7. **Questions for the owner** are at the end (section 6). The ones that shape the design most: which "power cycle" device (if any) in December, whether a PiKVM running its own Linux (probably with systemd) is acceptable as an appliance, and what "down" versus "unreachable" should mean on the panel.

---

## 1. What exists today (the starting point)

All items in this section are **SOURCE: this repository** (`docs/image.md`, `image/stage0/recovery-init`, read 2026-10-04), not tested again by me.

- The recovery kernel (`kernel-recovery.efi`) is a separate kernel with its own initramfs: static busybox with all applets, `hubos-ctl`, `efibootmgr`, `signify-openbsd`, the update keyring, e2fsprogs. It mounts the config partition read-only and the data partition read-write, runs DHCP on `eth0`, prints a banner and leaves a bare shell on the console (`image/stage0/recovery-init`).
- It does **not** arm the watchdog and never reboots by itself (`docs/image.md` 3.7). So a recovery kernel that hangs stays hung. (BELIEVED consequence: the hardware watchdog does not help in recovery unless something arms it. Not tested.)
- `hubos-ctl update BASE [SLOT]` already works in the recovery shell: signature check against the keyring, floor check, writes the named slot, sets `BootNext`, clears the failure counter (tests R3, R4, P3 in `docs/image.md`).
- Stage 0 sends a machine to recovery after N failed boots (default 3).
- `docs/image.md` 3.11 already lists an "unattended recovery" idea and its open questions. This document continues it.
- The update keyring holds **public** keys only; the private update key is offline (`docs/image.md` 3.12, "Keep both secret keys out of the repo and off the machines").

**TESTED** (what the recovery kernel's busybox can already do, from the Ubuntu 24.04 package the build uses, unpacked with `dpkg -x`, not installed):

```
$ busybox --list | grep -E "^(httpd|nc|wget|ssl_client|tcpsvd|sha256sum|base64)$"
base64 httpd nc sha256sum ssl_client wget
$ busybox | head -1
BusyBox v1.36.1 (Ubuntu 1:1.36.1-6ubuntu3.1) multi-call binary.
```

(`tcpsvd` is not in that package's busybox.) The recovery kernel is built from the build's own busybox, so this shows the Ubuntu package has these applets, not that the image's copy does; the list in `recovery-list.py` says "all applets" (SOURCE: repo).

---

## 2. Layer 1: the network recovery agent (in software)

### 2.1 What it must do

| Request | What the hub sends | What it does | Label |
|---|---|---|---|
| `GET /v1/status` | nothing | Says: machine id, state `recovery`, recovery kernel release, failure counter and limit, and the API fields `api` and `min_hub`. This is the "machine in recovery" signal. | prototype TESTED (unit tests; the earlier QEMU test T18 saw `state: recovery` through a forwarded port) |
| `POST /v1/install` | `{"Slot":"a\|b","BaseURL":"http..."}` | Runs the existing `hubos-ctl update BASE SLOT`. The bundle's own manifest signature (update keyring), the floor rule and the hashes are checked by `hubos-ctl` as always; the agent adds nothing. A refusal is answered `500` with `hubos-ctl`'s `REFUSED` line. | unit test with a fake `hubos-ctl` TESTED; the real refusals are `hubos-ctl`'s (TESTED in `docs/image.md` tests 4 and R3); the QEMU test of the unsigned agent is **not yet run** |
| `POST /v1/clear-failures` | nothing | Runs `hubos-ctl clear-failures` | unit test TESTED, backend fake |
| `GET /v1/logs` | nothing | Returns the last 4096 bytes of the agent's own log (`/run/recovery-agent.log`) | unit test TESTED, backend fake |

Why one check is enough for install: the **bundle signature** says "this software is the owner's", so whoever asks (the hub, or anything else that can reach the port) cannot make the machine run software the owner did not sign. Anyone who can reach the port can still ask for a reinstall of a signed release at or above the floor (see risk 5 in 2.5). (BELIEVED from reading `hubos-ctl`; the checks themselves were tested in R3.)

**What the prototype does not do:** the install call is synchronous. The real one takes several seconds to minutes (8.9 s for a tiny image in QEMU, `docs/image.md` test 5; a 160 MB hub root would be longer), so the real agent should answer "accepted" at once and let the hub poll status. Not built.

### 2.2 How hubd would reach it: alternatives compared

| Option | Fits the recovery kernel? | Auth | Cost / risk | Verdict |
|---|---|---|---|---|
| **A. Small Go program, HTTP + JSON** (what I prototyped) | Needs one more static binary. **TESTED:** `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"` gave 5,816,504 bytes (about 5.5 MiB) for the first prototype (the agent is a 6.1 MB static Go program in the TEST kernel, `docs/image.md` T18 note). The recovery kernel is 6,333,440 bytes today and the TEST kernel with the agent 8,893,440 bytes (SOURCE, `docs/image.md` section 5 and T18; the kernel compresses its initramfs, so the file grows by less than the program). | None on requests; the bundle check, see 2.3 | hubd is Go already, so the hub side reuses `net/http`. Size is the cost. | **Recommended as the first design**, because hubd's client side is trivial. |
| **B. busybox `httpd` + CGI shell scripts** | No new binary: `httpd` is a busybox applet (TESTED list above); `hubos-ctl` is already inside. | None on requests; the install check is `hubos-ctl`'s. | Parsing HTTP and JSON in shell is easy to get wrong; I did **not** build or test this. Busybox `httpd` CGI behaviour (timeouts, concurrency) not checked. | Real alternative if 5.5 MiB is too much. UNKNOWN until built. |
| **C. SSH** (dropbear or openssh in the recovery kernel) | Needs a new daemon and host keys. The recovery kernel is on the open boot partition (`docs/image.md` 3.11), so a host key stored there is readable by anyone with the disk, and the image would need a per-machine private key: that is the secrets problem again. | Public-key login of the hub | Gives a full shell, which is more power than the four jobs need. Host-key handling is unsolved. | Rejected for now. A shell is more than needed and a per-machine secret in recovery is hard. |
| **D. A tiny raw TCP "line" protocol on a port, with `nc`** | Fits (`nc` is in busybox). | Would have to be invented. | CLAUDE.md: "Do not invent protocols." | Rejected. |
| **E. gRPC / protobuf** | Needs a big library. | TLS | Far bigger than A for four calls. | Rejected. |
| **F. Do nothing in the network: use the serial layer for recovery** | Needs layer 2C wiring on every machine. | Physical | Costs cables per machine and needs serial ports on the boards (UNKNOWN until the board list). | Complements A; does not replace it. |

HTTP+JSON here is not a new streaming protocol or a web dashboard. It is a four-call control API that only `hubd` talks to; there is no web page.

### 2.3 What is checked (the image signature only)

**Requests are not signed and not encrypted** (owner decision, 2026-10-06). There is no management key, no challenge, no one-time number and no TLS. Anyone who can reach the agent's port can call any of the four calls. (An earlier version signed requests; see the History at the end.)

**What is still checked: the image.** `POST /v1/install` runs `hubos-ctl update BASE SLOT`. `hubos-ctl` refuses a bundle that is not correctly signed by the owner's update key: the manifest signature is checked against the update keyring (`/etc/hubos/keys/*.pub`, public keys only), then the floor rule (a release not newer than the floor is refused), then the hashes. The agent runs one install at a time and adds no check of its own (SOURCE: `agent.go`, `backend_hubos.go` on the branch `recovery-agent-no-signing`, read 2026-10-06).

**Constraint that still holds.** The recovery kernel is on the open boot partition, so it can hold **public keys only**, never a private key or a password (decided rule: secrets never in git, and `docs/image.md` 3.11 says the same for recovery). The update keyring is such a public key list; its rotation is the update-key rotation of `docs/image.md` 3.12 (a release signed with the old key can carry the next key; the old key is dropped only in a later release).

**Evidence.**
- TESTED (me, 2026-10-06, the branch `recovery-agent-no-signing` unpacked into a temporary folder, `go test -count=1 -v ./tools/image/experiments/recoveryagent/`): `TestStatusShowsRecovery`, `TestCallsNeedNoSignature`, `TestBadInstallRequestRefused`, `TestInstallRefusedByHubosCtlIsAnError`, `TestHubosBackend` all PASS (`ok hubos/tools/image/experiments/recoveryagent 0.034s`). The install-refusal test uses a fake `hubos-ctl` script that prints `REFUSED: bad signature` and exits with an error: the agent answers `500` with that text.
- SOURCE (repo): the real refusals (no signature, another key, a changed manifest, below the floor) are `hubos-ctl`'s and were tested in `docs/image.md` tests 4 and R3. The branch's QEMU test T18 now checks the same four refusals through the agent and that the correctly signed bundle installs; **it has not been run yet** (PR #69), so through the agent in a real recovery kernel these are UNVERIFIED. The earlier signed-request version of T18 passed in two full runs (`docs/image.md`): `GET /v1/status` answered `recovery`, a correctly signed install put release 28 into slot b and the machine booted and confirmed it, and the agent came back 1.3 s after `kill -9` (the supervisor restart loop). Those parts do not depend on request signing, but they were run before the change.

**What this does not protect.** The agent cannot prove its own identity to the hub, and nobody can prove theirs to the agent. A fake agent on the network could lie about status; it cannot make the hub install anything. Anyone who can reach the port can read the status and logs, clear the failure counter, or ask for an install of a signed bundle (a signed bundle at or above the floor is the only thing the machine will accept). The owner's rule is that the network is assumed perfect and security is not a design concern; I record the fact and add nothing. How often calls may be made is a question (13).

**No clock needed.** (BELIEVED: no battery-backed time is guaranteed on every board; recovery does not set the time; UNKNOWN on real boards.) Nothing in the unsigned design uses time.

### 2.4 How it announces itself ("machine in recovery" in the panel)

**Proposal: no announcement is sent. The hub looks.** hubd already probes machines (HUB-OS.md "Health"). For a machine whose normal check fails, hubd also tries the recovery agent's status call at the machine's inventory address. If it answers `state: "recovery"`, the panel shows "in recovery" (with the failure counter). Reasons: nothing new for the machine to get wrong; no hub address needed inside the recovery kernel (which the broken config partition may not hold); no message that can be spoofed in the direction machine to hub.

Open problem (UNKNOWN): the recovery kernel gets its address from DHCP (`recovery-init`, DHCP only). hubd looks at the inventory address, so the DHCP server must give each machine the same address every time (a "reservation"), or recovery must read a static address from somewhere. The config partition may be what is broken. **Question 4 in section 6.**

A broadcast beacon (the machine announces itself with a UDP packet) is possible and would work without knowing the address, but it adds an unauthenticated message and needs a listener; not recommended for a first version. Not tested.

### 2.5 Automatic repair mode: what it would do, and the risks

What it would do (**not built, only a design**): after the agent is up and a delay passes with nobody having acted, the recovery kernel asks the hub for a bundle (or the hub, seeing "in recovery", pushes an install) and runs the existing install, then reboots.

**Recommendation: the hub decides, the machine does not.** The machine in recovery does nothing by itself; hubd sees "in recovery" and the owner presses "reinstall" in the panel (or, later, hubd does it for machines marked automatic in the inventory). Reasons: the machine in recovery knows the least (its config may be broken); the hub has the signed bundle list and the owner's intent.

Risks (continuing `docs/image.md` 3.11):
1. **An install overwrites a slot.** It is the one that failed, but the other slot may hold the only working release. Rule: install only into the slot that failed, never the other, unless both failed (then nothing is left to lose). Needs the agent to know which slot failed; the stage 0 counter does not record this today (SOURCE: repo; BELIEVED).
2. **A bad release above the floor is installed without a human looking.** The floor stops old releases, not bad new ones.
3. **Loop:** release fails the same way, recovery reinstalls it, forever. Needs an attempt limit that survives reboots. Where it lives is open (the data partition is read-write in recovery, BELIEVED usable; EFI variables are the other option).
4. **Nothing in the requests or answers is authenticated** (section 2.3): a fake agent could receive an install request from the hub. It would only receive a URL for a bundle; the bundle is public anyway.
5. **Anyone who can reach the port can start an install**, not only the hub, and every install writes a slot and costs a reboot. What can be installed is bounded by the image signature and the floor; the flash wear and the downtime are not bounded. Limit blast radius: one machine per request (the agent runs one install at a time), and a rate limit in hubd or in the agent (not designed: question 13).
6. **The hub itself down:** nothing answers; machines wait in recovery. Accepted by the owner's own words (the rack is next to the desk), but it is a single point of failure.
7. **Watchdog:** recovery does not arm one (section 1); a hung recovery would stay hung. Whether the agent should arm and feed the watchdog is **question 7**.

### 2.6 What is testable in QEMU now, and what was done

| Piece | State | Label |
|---|---|---|
| The calls and their answers: status shows recovery with `api` and `min_hub`; logs, clear-failures and install need no header; `/v1/challenge` is a 404; bad install requests (slot c, a non-http URL, nonsense) are refused with 400 and never reach the backend; a failing `hubos-ctl` is a 500 | Done in `tools/image/experiments/recoveryagent/` (branch `recovery-agent-no-signing`) | **TESTED** (2.3) |
| The real backend (`HubosBackend`) with a fake `hubos-ctl` script: machine id from `NAME=`, release, failure counter, install into slot b only, one install at a time | Done (`TestHubosBackend`) | **TESTED** (2.3) |
| Live run on loopback with `curl` | Done, below | **TESTED** |
| The agent as a static binary inside a TEST recovery kernel under QEMU, reachable through a host port forward: status, restart loop (`kill -9`, back after 1.3 s, five quick kills make the loop give up), a real install through `hubos-ctl` into slot b, then boot and confirm | Done **in the signed-request version** (`docs/image.md` T18 and T19, SOURCE repo, PASS in two runs). The unsigned version of the same tests is changed on the branch and **not run yet** | TESTED (old version); UNVERIFIED (new version) |
| The agent in the normal recovery kernel, in a real image | Not done: only the TEST kernel carries it | UNKNOWN |
| A real board's network in recovery, real DHCP reservation, time to come up | Cannot be tested here | UNKNOWN |

Live run (the agent of the branch `recovery-agent-no-signing` built with `CGO_ENABLED=0 go build`, fake backend, listening on `127.0.0.1:18480`, run by me on 2026-10-06 in a scratch folder, stopped afterwards). Real commands and outputs:

```
$ curl -X GET /v1/status
{"api":1,"min_hub":1,"machine":"fake-1","state":"recovery","recovery_release":"recovery-1","boot_failures":3,"failure_limit":3}
 [HTTP 200]
$ curl -X GET /v1/logs
FAKE log line 1
FAKE log line 2
 [HTTP 200]
$ curl -X POST /v1/clear-failures
{"result":"cleared"}
 [HTTP 200]
$ curl -X POST /v1/install   (body {"Slot":"a","BaseURL":"http://10.0.2.2:8000/bundle"})
{"output":"FAKE: would run: hubos-ctl update http://10.0.2.2:8000/bundle a","result":"installed"}
 [HTTP 200]
$ curl -X POST /v1/install   (body {"Slot":"c","BaseURL":"http://x"})
{"error":"need JSON {\"Slot\":\"a|b\",\"BaseURL\":\"http...\"}"}
 [HTTP 400]
$ curl -X GET /v1/challenge
{"error":"no such endpoint"}
 [HTTP 404]
$ curl -X GET /v1/live
{"error":"no such endpoint"}
 [HTTP 404]
```

(`GET /v1/live` is not in the prototype yet; `docs/proposals/node-helper-api.md` has it in the design. The fake backend's install changes nothing.)

### 2.7 How the next step would be tested (described, not run)

1. Build the agent static (`CGO_ENABLED=0`), add it to `recovery-list.py`'s file list, start it from `recovery-init` after DHCP. (Done for the TEST kernel: `docs/image.md` T18.)
2. Boot QEMU into recovery with a host port forward (`-netdev user,hostfwd=tcp::18480-:8480`), as the runner already forwards nothing but serves bundles to the guest from the host (`docs/image.md` section 3).
3. From the host: `curl` status; an install request naming a bundle served by the runner's HTTP server (the guest sees the host as `10.0.2.2`); then the existing tests' check that the machine boots and confirms.
4. Check the cost: kernel size, recovery boot time to "agent answering".

---

## 3. Layer 2: out-of-band hardware (for a dead firmware or kernel)

"Out-of-band" means reaching a machine by a path that does not depend on that machine's own software. Layer 1 needs firmware, a kernel, a network card and the recovery kernel all working. Layer 2 is for when they are not. **The decision is the owner's, in December.** Constraints from the owner: no proprietary BMC firmware; every machine is always on; the rack is next to the desk; power never fails; no wake-on-LAN.

### 3.1 Summary table

Costs are in **cables and parts**, per machine and for twelve machines, because the owner has no hardware yet. Money figures are only given where a source gives them.

| | A. Nothing (walk over) | B. Switched power or relay | C. Serial console server | D. PiKVM-class |
|---|---|---|---|---|
| Wired to each machine | nothing | mains plug into a strip outlet (power cut), **or** 2 wires to the board's power-button header | a serial cable from a serial port or header on the board to a USB-serial adapter or serial card on a console host | HDMI cable + USB cable (+ optional Ethernet-style cable to an ATX board on the front-panel header) |
| Can see | nothing | nothing (maybe power on/off through the relay's own state) | boot text, kernel messages, the recovery shell if `console=ttyS0`; BIOS only if the board has serial redirection (UNKNOWN per board) | the screen, including BIOS (with caveats, 3.5) |
| Can do | the owner plugs a keyboard and monitor in, presses the PSU switch | power cycle; power button press | type into the console | type, mouse, virtual CD or USB disk, power and reset (with ATX board) |
| Cannot | anything remote | see or type; recover a bad install | power cycle; see graphics; BIOS usually | sensors; 4K; game-speed video; a dead PiKVM itself |
| For 12 machines | 0 | 12 outlets or 12 relays | 12 cables + adapters; a console host | 12 HDMI + 12 USB, 3 switch boxes + 1 PiKVM (section 3.5) |
| Security risk | none added | a controller on the network that can cut power to everything | console server holds a keyboard into every machine | holds full control of every machine |
| Licence | n/a | Tasmota GPL-3.0; others vary | ser2net GPL-2.0; conserver BSD-3-Clause | PiKVM software GPL-3.0 |

### 3.2 A. Do nothing: walk over, hardware watchdog, PSU switch

- **What it is.** The rack is next to the desk (owner decision). A dead machine is fixed by walking over with the spare keyboard and a monitor (HUB-OS.md: a spare keyboard and mouse are in a drawer), or by flipping the power supply's switch.
- **Hardware watchdog.** SOURCE: [Linux kernel watchdog API](https://docs.kernel.org/watchdog/watchdog-api.html), read 2026-10-04 (summarised): a hardware timer resets the machine if the program that feeds it stops; with "nowayout" it cannot be switched off once started; it can only reboot the whole machine and cannot tell a shutdown from a failure. In Hub OS the slot kernels arm it (SOURCE: repo, docs/image.md 3.10), the recovery kernel does not (section 1). So the watchdog turns a **hang after boot** into a reboot, and that is all; it does nothing for a machine stuck in firmware before the OS starts (BELIEVED), or for a hung recovery.
- **PSU switch.** Turning the supply off and on cuts power. Whether the board then powers itself on is a firmware setting ("restore on AC power loss"), which differs per board (UNKNOWN until the board list; many boards default to staying off, BELIEVED). A machine that stays off after a switch flip also needs the front power button.
- **Cost.** Zero parts. The cost is the owner's time and the fact that nothing works while the owner is away. Unlimited machines.
- **Security / maintenance / licence.** Nothing added. Nothing to update.

### 3.3 B. Switched power strip or relay board, open protocols

- **What it needs wired.**
  - *Mains cut:* the machine's power cord goes into a controllable outlet. This cycles power only if the board powers itself back on (UNKNOWN per board, see 3.2). Mains voltage: **the owner should not wire this by hand**; buy a finished unit. (BELIEVED safety advice, not from a source.)
  - *Power-button press:* a relay across the two pins of the board's front-panel power-button header. Low voltage, no mains. Needs a free or shared header; works like pressing the button (BELIEVED).
- **Open protocols and open software.**
  - [Tasmota](https://github.com/arendst/Tasmota): alternative firmware for ESP8266/ESP32 devices, local control over MQTT, HTTP, serial; GPL-3.0 (SOURCE: README and LICENSE.txt, read 2026-10-04). The HTTP form is `http://<ip>/cm?cmnd=<command>` (SOURCE: search result summary, 2026-10-04; not read on Tasmota's own docs page). Many cheap Tasmota-capable plugs are Wi-Fi only; a Wi-Fi device on the management network is a weaker design than a wired one (BELIEVED). Ethernet ESP32 boards exist (BELIEVED; not checked).
  - [usbrelay](https://github.com/darrylb123/usbrelay): command-line control of cheap USB HID relay boards with 1, 2, 4 or 8 relays, 10 A 250 VAC double-throw (SOURCE: its README, read 2026-10-04). Licence not read: UNKNOWN. Needs a host computer with USB that stays reachable (the hub or a small helper).
  - SNMP or vendor-protocol switched PDUs are open *protocols* but usually run closed vendor firmware (BELIEVED; the owner's rule is no proprietary BMC firmware, and whether a PDU counts is **question 3**).
- **Cannot see or do.** Nothing about screen, console, BIOS or a bad install. It only turns power off and on, so it helps with a **hung** machine and does not help with a machine that boots into a bad state over and over.
- **Cost for twelve machines.** Twelve outlets or twelve relay channels (for example two 8-relay USB boards or a few Tasmota devices) plus cables. No source gives a price; **UNKNOWN**. Mains outlets add a 12-way strip's worth of cabling behind the rack.
- **Security.** The controller can cut power to the whole rack. Its network protocol (Tasmota HTTP) has optional password only (BELIEVED, not checked). Keep it on the isolated management network.
- **Maintenance.** Firmware updates for each controller (OTA for Tasmota, SOURCE README); a USB relay needs no updates, but the host it plugs into does.
- **Licence.** Tasmota GPL-3.0. usbrelay UNKNOWN.

### 3.4 C. Serial console servers (ser2net, conserver)

- **What they are.** [ser2net](https://github.com/cminyard/ser2net) "allow[s] connections between gensio accepters and gensio connectors", normally a network connection to a serial port, with options for Telnet/RFC2217 and encryption through the gensio library (SOURCE: README.rst, read 2026-10-04; GPL-2.0 by `COPYING`). [conserver](https://github.com/bstansell/conserver) lets several people watch a serial console, logs it, and gives one person write access at a time (SOURCE: README.md, read 2026-10-04; BSD 3-Clause by `LICENSE`).
- **What it needs wired.** On each machine: a serial port, and the kernel told to use it as console. SOURCE: [Linux serial console](https://docs.kernel.org/admin-guide/serial-console.html), read 2026-10-04 (summarised): serial support must be built into the kernel; `console=ttyS0,115200`; shows kernel messages and a login prompt if a getty runs. The test images already use `console=ttyS0` (SOURCE: repo). On the console host: one USB-serial adapter per machine, or a multi-port serial card, or a small network-connected console box (the console host is then another machine that must itself stay up).
- **The big unknown.** **Do the owner's boards have a serial port (a header or a connector)?** UNKNOWN until the December board list. Many consumer boards have none, and a USB-serial dongle plugged into the machine does not show boot text (BELIEVED). Many server boards have a serial header or "console redirection" in the firmware setup, so the firmware menu can also come over serial, but this differs per board (BELIEVED).
- **Can do.** Watch the boot, see why a kernel panics, type into the recovery shell (already built; the tests drive it over serial). Logs of every boot kept by conserver, which is useful for "why did it fail".
- **Cannot do.** Power cycle; show graphics; BIOS on boards without serial redirection; recover a machine whose firmware is dead.
- **Cost for twelve machines.** Twelve serial cables (null-modem or header-to-DB9, depending on the board; BELIEVED), twelve adapters or a card with enough ports, one console host. No sourced prices; UNKNOWN.
- **Security.** ser2net's plain TCP mode is unencrypted (SOURCE README: use telnet or gensio options deliberately); the console server holds a keyboard into every machine. Bind to the management network only. Passwords: the recovery shell today has no login (`exec setsid cttyhack /bin/sh`, SOURCE: repo), so **anyone who reaches the serial console has root on a machine in recovery**. A console server therefore makes the management network the only protection.
- **Maintenance.** Two small packages on one host; updating that host.
- **Licence.** GPL-2.0 (ser2net), BSD-3-Clause (conserver).

### 3.5 D. PiKVM-class devices

All items are **SOURCE** unless marked, read 2026-10-04, **(summarised)** where the page was read through the summarising fetch tool.

- **What it is.** Open-source software for an IP-KVM (a box that gives remote screen, keyboard, mouse and more) on a Raspberry Pi, GPL-3.0: [pikvm/pikvm](https://github.com/pikvm/pikvm) and [pikvm/kvmd](https://github.com/pikvm/kvmd) (LICENSE files read: GNU GPL version 3). Documentation: [docs.pikvm.org](https://docs.pikvm.org/). The README lists: HDMI capture, USB keyboard and mouse emulation, virtual CD/DVD and flash drive (bootable on some Pi models only), ATX control, IPMI and Redfish, GPIO and USB relay control (README, summarised).
- **What it needs wired to each machine.**
  - HDMI cable from the machine's video output to the PiKVM or switch input; a USB cable from the PiKVM or switch to a USB port on the machine (keyboard, mouse and disk emulation go through it). [PiKVM Switch page](https://docs.pikvm.org/switch/) (summarised): per target an HDMI 2.0 cable (50 cm minimum), a USB-C cable, and an optional straight Ethernet-type cable for ATX.
  - Optional **ATX board**: wired between the case's front-panel connector and the board's header; gives power and reset control and reads the power and disk LEDs ([ATX page](https://docs.pikvm.org/atx_board/), summarised). Not usable on Macs. Polarity matters (same page).
  - A **machine with no HDMI output** (many server and headless boards have only a BMC's VGA, or nothing) would need a video source; UNKNOWN per board. A machine whose GPU is a gaming or compute card may not output anything during early boot to every port (BELIEVED; not checked).
- **Multi-machine support.** The PiKVM Switch: one box takes 4 targets; up to five boxes can be daisy-chained for 20 ports; it works with PiKVM V4 Plus, V3 and DIY V1/V2, **not** with the V4 Mini; only **one target at a time** gets video and USB control, while ATX status and control are available for all targets together ([switch page](https://docs.pikvm.org/switch/), [multiport page](https://docs.pikvm.org/multiport/), both summarised). Some third-party HDMI/USB switches work with PiKVM, with listed quirks (multiport page).
- **What it cannot see or do.**
  - **No board sensors.** The IPMI page makes no mention of sensors (summarised), and the project describes nothing like a board's temperature or fan readings. A PiKVM is not a board management chip. **Sensors would come from the OS through the node helper** (HUB-OS.md "Parts we write ourselves"). A dead OS therefore means no sensors at all in any of these options. (BELIEVED for PiKVM not providing board sensors; the absence is not stated outright by the sources I read.)
  - Video: 1080p at most through the HDMI-CSI bridge; 4K unsupported; not suitable for gaming ([FAQ](https://docs.pikvm.org/faq/), summarised).
  - **BIOS caveats** ([FAQ](https://docs.pikvm.org/faq/) and [V4 page](https://docs.pikvm.org/v4/), both summarised): wrong or glitchy resolution in BIOS/UEFI fixable by enabling CSM; no video in GRUB2 fixable by switching UEFI video mode to legacy; some HP and Dell boards have trouble detecting it in the BIOS; some firmware cannot see USB hubs at boot, so a direct connection is needed. Whether the owner's boards have these problems: UNKNOWN until tested in December.
  - Virtual disk boot works only on some Raspberry Pi models (README, summarised).
- **IPMI and Redfish.** [IPMI page](https://docs.pikvm.org/ipmi/) (summarised): power status and on, Redfish reset/ForceOff, serial-over-LAN through a USB-serial adapter; **"we strongly recommend that you DO NOT USE [IPMI] outside of trusted networks"**; passwords for IPMI are stored in plain text; Redfish uses HTTP Basic auth; it recommends Redfish or the KVMD API instead. For hubd, the **KVMD HTTP API** is the likelier route (the docs list an HTTP API reference; I did not read it, UNKNOWN).
- **Serial.** PiKVM V3 and V4 have a built-in USB-UART, and V4 Plus an RJ45 serial port ([V4 page](https://docs.pikvm.org/v4/), summarised; search result on V3, summarised). One serial port per PiKVM, so serial through a switch box for twelve machines is **UNKNOWN**.
- **Cost for twelve machines (arithmetic on sourced prices).** SOURCE: [pikvm.org/buy](https://pikvm.org/buy) read 2026-10-04 (summarised): prices from resellers excluding VAT in USD: V4 Plus lowest 335.89, V4 Mini 235.59, V3 238.25, PiKVM Switch lowest 235.59; ATX board: no price shown. For 12 targets: one V4 Plus (335.89) + 3 switches (3 x 235.59 = 706.77) = **1,042.66 USD** before cables, ATX boards and VAT, and before any shipping. For 20 machines: 5 switches, so 335.89 + 1,177.95 = 1,513.84 USD; beyond 20 machines a second PiKVM plus its switches is needed (the 20-port limit is per PiKVM chain, BELIEVED from the switch page). The README says DIY builds cost "$30 to $100" (README, read 2026-10-04); that predates these switch prices and does not include the multi-port switch, so I do not use it for the totals. All figures are as of the page I read and will move before December.
- **Cables for twelve machines.** 12 HDMI + 12 USB-C (+ 12 Ethernet-type cables if ATX boards are wanted), plus 3 uplink cables between switches and the PiKVM and its power. So about 24 to 36 cables behind a rack, plus 12 ATX boards if power control is wanted.
- **Security: it holds full control.** Anyone with its login can type on every machine, change BIOS settings, boot a virtual disk and cut power. Sources: default logins are `root`/`root` for Linux and `admin`/`admin` for the KVM and "must be changed immediately"; two-factor sign-in covers the web interface and API but **not** SSH or root; the docs recommend it for any internet exposure ([auth page](https://docs.pikvm.org/auth/), summarised). The software's own advice does not include a firewall design; mine: a dedicated management network with no route to the internet or to the normal network except from the hub (BELIEVED to be the right design; **not tested**). Cheap relays and consoles belong on the same network.
- **Updates and maintenance.** The OS is updated with `pikvm-update` and runs mostly read-only; changes need `rw`/`ro` ([cheat sheet](https://docs.pikvm.org/cheatsheet/), summarised). The page suggests an Arch Linux ARM base (the summary says the docs reference `pacman`; BELIEVED, not confirmed). Updates are manual per device, so the device must be updated by the owner on a schedule, and a device left unpatched is the weakest point in the rack (same reason BMCs hurt, section 4).
- **Licence.** Software GPL-3.0 (LICENSE read). The hardware is sold by the project's company; whether the V4 or the Pi hardware itself contains **closed firmware blobs** (the Raspberry Pi boot firmware is closed, BELIEVED) matters to the owner's "no proprietary BMC firmware" rule: **question 2**. Also, a PiKVM runs its own Linux, which very likely uses systemd (BELIEVED, Arch Linux ARM); Hub OS bans systemd on **its machines**, and whether an appliance that is not a Hub OS machine is allowed is **question 1**. I do not assume yes.
- **"Down" for a PiKVM itself.** If the PiKVM dies, the machines are fine, but the owner loses layer 2 until it is replaced (BELIEVED).

### 3.6 What is the same across all of layer 2

- **None gives sensors** (temperature, fans, voltages). OS-side node helper only. If the OS is dead there are no readings.
- **The management network.** Everything in B, C, D can break the machines it touches. One isolated network (a separate switch or a separate VLAN), reachable only from the hub, is the minimum. Whether the hub then needs a second network port is **question 6**.
- **Cost of doing none:** the walk to the rack, no more.

---

## 4. Proprietary BMCs, for the record

- **What they are.** A BMC (baseboard management controller) is a small computer soldered on many server boards. It runs its own firmware, has its own network port, and works while the main machine is off or dead: power control, screen and keyboard over the network, sensors. It sits underneath the operating system, so it sees and controls everything (SOURCE: [Eclypsium on CVE-2024-54085](https://eclypsium.com/blog/bmc-vulnerability-cve-2024-05485-cisa-known-exploited-vulnerabilities/), summarised, published 2025-06-26, read 2026-10-04: BMCs operate "below operating systems, hypervisors, and security controls", and compromise can persist where normal security tools do not look).
- **Why we avoid them.** The firmware is closed (the owner's decision), so the owner cannot read it, patch it, or build it. The vendor's schedule decides when a bug is fixed, and a board's BMC is rarely updated (BELIEVED). A flaw gives full control of the machine, below anything Hub OS does.
- **The AMI MegaRAC flaw: CVE-2024-54085.**
  - **TESTED / SOURCE (CISA feed):** the Known Exploited Vulnerabilities list, downloaded 2026-10-04 (catalogue version 2026.10.02):
    ```
    $ curl -sS -o kev.json https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json
    "cveID": "CVE-2024-54085", "vendorProject": "AMI", "product": "MegaRAC SPx",
    "vulnerabilityName": "AMI MegaRAC SPx Authentication Bypass by Spoofing Vulnerability",
    "dateAdded": "2025-06-25", "dueDate": "2025-07-16", "knownRansomwareCampaignUse": "Unknown",
    "shortDescription": "AMI MegaRAC SPx contains an authentication bypass by spoofing vulnerability in the Redfish Host Interface. ..."
    "cwes": ["CWE-290"]
    ```
    (fields copied from the downloaded file; the entry's notes point to AMI's advisory AMI-SA-2025003 and NetApp's advisory.)
  - **TESTED / SOURCE (NVD API):** `curl -sS "https://services.nvd.nist.gov/rest/json/cves/2.0?cveId=CVE-2024-54085"`, read 2026-10-04: published 2025-03-11; text: "AMI's SPx contains a vulnerability in the BMC where an Attacker may bypass authentication remotely through the Redfish Host Interface. A successful exploitation of this vulnerability may lead to a loss of confidentiality, integrity, and/or availability."; CVSS 3.1 base score 9.8 (`AV:N/AC:L/PR:N/UI:N`), CVSS 4.0 base score 10.0.
  - **SOURCE (summarised):** the same Eclypsium page and a search-result summary on 2026-10-04: the trick is a forged `Host` or `X-Server-Addr` HTTP header that makes the BMC believe a request comes from the host machine; versions named in a search summary (12.0 to 12.6 and 13.0 to 13.4) were **not** confirmed on a primary page (BELIEVED); the Eclypsium article says this is the first BMC flaw in the KEV list. Affected vendors named in a search summary: HPE, Asus, ASRock Rack (BELIEVED; not confirmed on the primary advisory). I could not read AMI's PDF advisory (not tried).
  - What this shows: an attacker on the management network needed no password, and "isolated network" was the only defence for an unpatched machine.
- **The owner's decision stands:** no proprietary BMC firmware.

### 4.1 Is OpenBMC a realistic alternative?

All **SOURCE**, read 2026-10-04 from the raw files in [openbmc/openbmc](https://github.com/openbmc/openbmc):

- README: "OpenBMC is a Linux distribution for management controllers used in devices such as servers, top of rack switches or RAID appliances. It uses Yocto, OpenEmbedded, **systemd**, and D-Bus ..." So OpenBMC's own firmware is built on systemd. `CLAUDE.md` says "Never use systemd, under any circumstances." The BMC is not a Hub OS machine, but the rule says never; a BMC running systemd would fall under it unless the owner says otherwise. **Question 1.**
- `meta-phosphor/docs/supported-machines.md`: "some code which has been contributed ... The specific level of functionality supported is unknown and has no level of warranty, promise of future development, or bug-fix guarantee." Listed families: AMD (daytonax, ethanolx), Ampere, ASRock (e3c246d4i, e3c256d4i, romed8hm3, spc621d8hm3, x570d4u), Facebook/Meta, HPE (dl360, dl385), IBM, Intel s2600wf, Supermicro x11spi, Tyan, Quanta, Wistron, evaluation boards (evb-ast2500, evb-ast2600, evb-npcm750, ...) and others. Those listed names are server, datacenter and evaluation boards.
- OpenBMC needs a BMC chip on the board (ASPEED AST2500/2600 or Nuvoton NPCM, from the `evb-*` names; BELIEVED, not stated outright in what I read). A consumer desktop board has no such chip, so OpenBMC cannot run on it (BELIEVED).
- **So:** realistic only if the owner buys one of the listed server-class boards (for example an ASRock Rack board in the list; I believe "e3c246d4i" and its relatives are ASRock Rack products, BELIEVED), and even then: the board ships with the vendor's closed BMC firmware, replacing it means flashing the chip (risk of a dead board: UNKNOWN), functionality is "unknown ... no warranty" per the project's own list, and OpenBMC uses systemd. The GPU boxes and the gaming box (consumer parts) do not fit. **Not recommended**; recorded because the owner asked.

---

## 5. How hubd would use the layers: the smallest honest design

Nothing here is built. Do not read it as a spec; it is the smallest idea that does not pretend.

### 5.1 Panel states

hubd already knows "up" (normal check passes) and "down" (it does not), HUB-OS.md "Health". Proposal adds two:

| State | How hubd decides | Label |
|---|---|---|
| **up** | The normal check passes (today's rule). | existing |
| **in recovery** | The normal check fails **and** the recovery agent at the machine's address answers a status call with `state: "recovery"`. | design; prototype TESTED for the status reply only |
| **down** | The normal check fails and the recovery agent does not answer, **while other machines answer**. A machine that is off shows as this (HUB-OS.md: no wake-on-LAN). | design |
| **unreachable** | Nothing answers from any machine (including a quick check that the hub's own network port is up). Means "probably the hub or the network, not these machines", so the panel does not show twenty red machines. | design, a heuristic; BELIEVED reasonable, **not tested**; **question 5** (the owner may want "unreachable" to mean something else) |

Extra cost: one more check, only for machines that already failed the normal check. At 100 machines with 10 down, that is 10 extra short probes per round; not measured. The existing hubd time limits (2 s per machine, 5 s total) are themselves unmeasured guesses (HUB-OS.md "Unverified").

### 5.2 Actions

| Action | Available when | What happens | Label |
|---|---|---|---|
| **Reinstall** | state "in recovery" | hubd asks the agent (2.3) to install a named signed bundle into a named slot, then watches the agent's status for progress, then waits for the machine to come up. The owner confirms, naming what will be overwritten, like the existing power actions (HUB-OS.md "Power actions"). | design |
| **Power cycle** | only if the machine has a device defined (a relay, a strip outlet or a PiKVM ATX board) | hubd calls that device's own interface. | design; the inventory field does not exist; the form is not decided (**question 3**) |
| Clear failure counter | state "in recovery" | plain request | design |
| Show logs | state "in recovery" | plain request (`GET /v1/logs`) | design |

With no device defined, a dead machine shows "down" and the owner walks over. That is the honest default.

### 5.3 Where addresses and credentials come from (an input for the secrets design)

- **Addresses.** The inventory already holds each machine's address (no secrets in it, HUB-OS.md). The recovery agent is reached at the **same address** (needs a DHCP reservation, section 2.4) on port **8480**, shared with the node helper (`node-helper-api.md` section 2; to be confirmed; not yet in `default_ports`). A power device or PiKVM needs its **own** address; where it lives in the inventory is open: probably a field on the machine naming the device and its address, **not decided** (question 3).
- **Credentials the hub would hold (all secrets; none in the inventory or git):**
  1. a login for each power device (Tasmota password, if set);
  2. a login for each PiKVM (its KVM user) and API;
  3. the two-factor secret if used for PiKVM.
  (The recovery agent needs none: requests are not signed. The update private key stays offline.) Where these live on the hub (a file readable only by the hub user, not on the NAS backup, not in the image) is for the secrets design. The compromise of the hub's disk then means: power cycling and, with a PiKVM, full control of every machine. That is the cost of option D and should be weighed in December. **Not designed.**
- **Which bundle and where from:** the hub (or the NAS) serves it; the base URL goes into the install request. Which release (newest, or the floor's own) is a question (question 8); the floor forbids older ones (SOURCE: repo).

### 5.4 What stays a question

See section 6. In short: the device choice, whether an appliance with systemd or closed firmware blobs is allowed, how to get the recovery address, what "unreachable" means, automatic repair or button, and what to do about the watchdog in recovery.

---

## 6. Questions for the owner

1. **systemd on appliances.** The rule says never systemd. PiKVM's OS (BELIEVED Arch Linux ARM) and OpenBMC (SOURCE: README) use it. Is an appliance that is not a Hub OS machine allowed to run it, or does the rule rule out PiKVM and OpenBMC entirely?
2. **"No proprietary BMC firmware."** Does a PiKVM (an open-source program on a Raspberry Pi, whose own boot firmware is closed, BELIEVED) count as acceptable, or only fully open firmware?
3. **Power cycle device.** For December: none (walk over), a relay or switched outlets (which open protocol, wired or Wi-Fi?), or PiKVM with ATX boards? And may I define a small optional per-machine field in the inventory for "how to power cycle this one"? (I have not invented its format.)
4. **Address in recovery.** May the recovery kernel rely on a DHCP reservation per machine (set on the router), or should recovery read a static address from the config partition (which might be broken)?
5. **Panel wording.** Is my split of "down" (this machine only) and "unreachable" (nothing answers, probably the hub or the network) what you want?
6. **Management network.** An isolated second network (own switch, hub gets a second port) for the agent, relays, console server and any PiKVM, or the same network as everything else (weaker)?
7. **Watchdog in recovery.** Should the recovery kernel (or its agent) arm the hardware watchdog, so a hung recovery reboots back into the boot loop breaker? Today it does not.
8. **Reinstall policy.** Button only (my recommendation), or hubd automatic repair for machines you mark? Which release to install: the floor's own, the newest, or the last confirmed? Which slot: only the one that failed?
9. **Agent form.** The Go HTTP+JSON agent costs about 5.5 MiB in the recovery kernel (measured as a binary; kernel effect unmeasured). Is that acceptable, or should I build the busybox `httpd` shell version first?
10. **Serial ports.** When you choose boards in December, should a serial header or console redirection be a requirement for the rack machines, so layer 2C is possible?
11. **Recovery login.** The recovery shell has no login today. With a serial console server or PiKVM, anyone who can reach it has root on a machine in recovery. Add a password (a secret on the open boot partition, so only a weak deterrent) or accept the isolated network as the protection?
12. **HUB-OS.md.** Per your instruction I did not edit it. If you accept any of this, the brief and its Change log need updating by you or by me when you ask.
13. **Limits on calls to the agent.** Requests are plain and anyone who can reach the port can call it, so nothing but the agent's own rule (one install at a time) bounds how often installs, clear-failures or status calls are made. Do you want a rate limit (in the agent, or only in hubd), or none? What should the agent answer to a second install request while one runs (the prototype refuses it; the answer code is not written down)? I designed nothing.

---

## 7. What was run, in one place

- Worktree: `/home/user/wt-p3` (removed at the end), branch `recovery-oob-proposal`.
- `gofmt -l .`, `go vet ./...`, `go test -count=1 ./...`: output is in the pull request and in the final report. The new package is `hubos/tools/image/experiments/recoveryagent` (standard library only, no new dependency).
- The prototype's tests (names): status shows recovery with `api` and `min_hub`; the calls need no signature and `/v1/challenge` is a 404; a bad install request is refused with 400 before the backend; an install refused by `hubos-ctl` is a 500 with its text; the real backend against a fake `hubos-ctl` script. I ran them on 2026-10-06 (2.3).
- **Not tested:** anything on real hardware; the unsigned agent inside a recovery kernel under QEMU (not yet run); the agent in a real image; the busybox-only alternative; any layer-2 device; every claim marked BELIEVED or UNKNOWN above.
- Pages read through the summarising fetch tool (wording not guaranteed): all `docs.pikvm.org` pages, `pikvm.org/buy`, the GitHub repo pages for PiKVM, OpenBMC and Tasmota, the Eclypsium article, the kernel serial-console and watchdog pages, the OpenBSD signify man page. Pages read as raw files with `curl`: the CISA feed, the NVD API, OpenBMC's README and supported-machines list, ser2net's README and COPYING, conserver's README and LICENSE, PiKVM's LICENSE and README, Tasmota's LICENSE and README, usbrelay's README, signify.c. Not reached: NVD's web page and CISA's catalogue web page (the tool could not show the entry; the feed and API were used instead), the PiKVM `serial_port` and `atx_control` docs pages (404; other pages used), AMI's PDF advisory (not tried).

---

## Owner decisions (2026-10-06, round 8)

- The "compositor gave up" message (the compositor crashed 5 times in a minute and was stopped) is drawn by the kernel text console on the projector, because the bar dies with the compositor; hubd and the recovery terminal stay up. Recorded in `HUB-OS.md` and `docs/proposals/driftwm-patches.md` 11.12a.
- The recovery agent's requests are not signed (owner decision, 2026-10-06); `GET /v1/logs` stays open.

## History

An earlier version of this document (up to 2026-10-05) had every changing request to the recovery agent signed by the hub with a separate management key and a one-time number fetched from `GET /v1/challenge` (later with the machine id in the signed text), and discussed TLS and the key's custody. The owner decided on 2026-10-06 that requests are not signed and not encrypted and that only the image signature stays. That text and its live-run transcript were removed; they can be read in the git history of this file before the commit that made this change. The prototype was changed to match (PR #69).
