# PROPOSAL: network recovery agent and out-of-band options

> **OWNER DECISION (2026-10-06): the hub's requests are NOT signed.** There is no management key, no challenge, no signature and no one-time nonce on requests to the recovery agent or the node helper, and no TLS. Wherever this document says "signed", "signature", "nonce", "challenge", "HubOS-Sig" or "management key" about a REQUEST, read it as "not used"; those sections are history (they describe what was prototyped and tested). The IMAGE signature is unchanged: the install request still refuses anything that is not a correctly signed image bundle (the manifest signature, the update keyring and the floor rule). The recovery agent prototype is being changed to match (a separate pull request).

**Status: PROPOSAL.** Nothing here is decided and nothing here is in the images. `HUB-OS.md` wins if this file disagrees with it. The only code is a small experiment in `tools/image/experiments/recoveryagent/` (loopback only, fake backend). Written 2026-10-04.

**Labels used on every item:**
- **TESTED**: I ran it in this build environment; the exact command and output are shown.
- **SOURCE**: read in an official source; the link and the date I read it (2026-10-04) are given. Several web pages were read through a fetch tool that summarises the page with a small model, so the wording is not guaranteed to be the page's own; those are marked "(summarised)". Anything that matters for a decision should be re-read by a person on the page itself.
- **BELIEVED**: I think it is true but I did not test it and have no official source.
- **UNKNOWN**: nobody has checked; do not rely on it.

---

## Plain-words summary

1. **The problem.** Today, if a rack machine cannot boot, the recovery kernel starts a bare shell and waits for a person at a keyboard. Rack machines have no keyboard or screen (the recovery shell on a screen is also untested; the tests use a serial line). So recovery as built needs the owner to carry a keyboard and monitor to the rack.
2. **Layer 1, in software (cheap, no new hardware).** Put a small network program (the "recovery agent") in the recovery kernel. It answers "I am in recovery", and, only for requests signed by the hub, it can install a signed bundle into a named slot, clear the failure counter and hand over logs. The hub then shows "in recovery" in the panel and can offer "reinstall". This helps when the **firmware and the recovery kernel work**.
3. **What I built to check the idea.** A tiny Go program that does the status call and the "signed request" check, tested against the real `signify-openbsd` program (the same tool the update keyring uses). It works on loopback. It is **not** in any image and its "install" is a fake. Whether it works inside the real recovery kernel is **not tested**.
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

| Request | Needs a signature from the hub? | What it does | Label |
|---|---|---|---|
| Status | No (read-only, low value) | Says: machine name, state `recovery`, recovery kernel release, failure counter and limit. This is the "machine in recovery" signal. | prototype TESTED on loopback with a fake backend |
| Install a signed bundle into slot a or b | Yes | Runs the existing `hubos-ctl update BASE SLOT`. The bundle's own manifest signature (update keyring) is checked by `hubos-ctl` as always. | request check TESTED; the real install through the agent is **not** built or tested |
| Clear the failure counter | Yes | Runs `hubos-ctl clear-failures` | request check TESTED; backend fake |
| Report logs | Yes | Returns recent log text (what `hubos-ctl` printed, kernel log) | request check TESTED; backend fake |

Why two layers of checking for install: the **hub signature** says "the owner's hub asked for this" (so a stranger on the network cannot make the machine download things or wear out its flash). The **bundle signature** says "this software is the owner's" (so the hub, or the server it points at, cannot install anything the owner did not sign). Both are needed; neither replaces the other. A hub that is hacked can ask for a reinstall of any signed release at or above the floor, but cannot make the machine run unsigned software (BELIEVED from reading `hubos-ctl`; the checks themselves were tested in R3).

**What the prototype does not do:** the install call is synchronous. The real one takes several seconds to minutes (8.9 s for a tiny image in QEMU, `docs/image.md` test 5; a 160 MB hub root would be longer), so the real agent should answer "accepted" at once and let the hub poll status. Not built.

### 2.2 How hubd would reach it: alternatives compared

| Option | Fits the recovery kernel? | Auth | Cost / risk | Verdict |
|---|---|---|---|---|
| **A. Small Go program, HTTP + JSON** (what I prototyped) | Needs one more static binary. **TESTED:** `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"` gives 5,816,504 bytes (about 5.5 MiB). The recovery kernel is 6,333,440 bytes today (`docs/image.md` section 5), so this would roughly double it if stored uncompressed; the real effect on the kernel size is **UNKNOWN** (initramfs compression not checked). | Signed requests, see 2.3 | hubd is Go already, so the hub side reuses `net/http` and the same signing code. Size is the cost. | **Recommended as the first design**, because hubd's client side is trivial and the signing code is shared. |
| **B. busybox `httpd` + CGI shell scripts + `signify-openbsd`** | No new binary: `httpd` is a busybox applet (TESTED list above), `signify-openbsd` and `sha256sum` are already inside. | Same signed-request scheme done in shell with `signify -V`. | Parsing HTTP and JSON in shell is easy to get wrong; I did **not** build or test this. Busybox `httpd` CGI behaviour (timeouts, concurrency) not checked. | Real alternative if 5.5 MiB is too much. UNKNOWN until built. |
| **C. SSH** (dropbear or openssh in the recovery kernel) | Needs a new daemon and host keys. The recovery kernel is on the open boot partition (`docs/image.md` 3.11), so a host key stored there is readable by anyone with the disk, and the image would need a per-machine private key: that is the secrets problem again. | Public-key login of the hub | Gives a full shell, which is more power than the four jobs need. Host-key handling is unsolved. | Rejected for now. A shell is more than needed and a per-machine secret in recovery is hard. |
| **D. A tiny raw TCP "line" protocol on a port, with `nc`** | Fits (`nc` is in busybox). | Would have to be invented. | CLAUDE.md: "Do not invent protocols." | Rejected. |
| **E. gRPC / protobuf** | Needs a big library. | TLS | Far bigger than A for four calls. | Rejected. |
| **F. Do nothing in the network: use the serial layer for recovery** | Needs layer 2C wiring on every machine. | Physical | Costs cables per machine and needs serial ports on the boards (UNKNOWN until the board list). | Complements A; does not replace it. |

HTTP+JSON here is not a new streaming protocol or a web dashboard. It is a four-call control API that only `hubd` talks to; there is no web page.

### 2.3 How it authenticates (and what the recovery kernel can afford) -- REQUEST SIGNING REMOVED by the owner, 2026-10-06; only the image-bundle check stays

**Constraints.**
1. The recovery kernel is on the open boot partition, so it can hold **public keys only**, never a private key or a password (decided rule: secrets never in git, and `docs/image.md` 3.11 says the same for recovery).
2. It has **no trusted clock** (BELIEVED: no battery-backed time is guaranteed on every board; recovery does not set the time; UNKNOWN on real boards). So signed timestamps are unusable.
3. It cannot afford a large TLS library unless it is in the Go program (see below).

**The scheme in the prototype (TESTED on loopback).**
- The hub asks `GET /v1/challenge` and receives a random one-time number (a "nonce").
- The hub signs this exact text with its **management private key**, using `signify-openbsd -S`:
  six lines, one per line: `hubos-recovery-v1`, `machine=ID` (the target machine id; **new in round 3**), method, path and query, nonce, SHA-256 of the body. The agent builds the text with **its own** machine id (the `NAME=` in its node config; the hub signs for the id it means to talk to), so a request signed for another machine fails. UNKNOWN: whether `NAME=` is the same string as the inventory id; today nothing makes them equal.
- The request carries `Authorization: HubOS-Sig nonce=HEX, sig=BASE64` (the second line of signify's `.sig` file); it may also carry `machine=ID`, and if that is not this machine's name the agent refuses at once with `403 this request is for another machine` (a clearer message than a failed signature; the nonce is still used up).
- The agent checks the nonce was issued by it, is unused and is under 30 seconds old (it uses a clock that only needs to run forward, not be correct), then checks the signature against every public key in its **management keyring**. The nonce is used up whatever the outcome. At most 16 challenges are open at once.
- The signature covers the machine id, method, path, body hash and nonce, so a captured request cannot be replayed, redirected to another path or **another machine**, or given a different body (each is a unit test, including "signed for another machine id is refused"). `GET /v1/status` also carries the add-only fields `api` (this API's version, 1) and `min_hub` (the lowest hub API version it works with, 1).
- **Round 3 change** (the helper agent that wrote `docs/proposals/node-helper-api.md` noticed that the old text did not name the machine, so a challenge could be relayed from one node to another): TESTED in `tools/image/experiments/recoveryagent` (`go test`, including the real `signify-openbsd` interop test, run with the program unpacked into a temporary folder); the QEMU test T18 was changed to sign with the machine line and was **not re-run** (UNVERIFIED) unless the pull request says otherwise.

(REMOVED by the owner, 2026-10-06: there is no management key. The paragraph below is history.) **A separate management key, not the update key.** The update key signs releases and its private half is offline. The hub must sign requests at run time, so it would need its private key online. That key must not be the update key. So: **a second key pair, the management key**, whose public half goes into the recovery kernel (and a public keyring file like `/etc/hubos/keys`), and whose private half lives only on the hub (an input for the secrets design, section 5.3). Rotation could copy the update-key procedure (docs/image.md 3.12); not designed here.

**TLS.** The signed requests give authenticity and replay protection but **not secrecy**: anyone on the wire can read status and logs. In the recovery kernel nothing secret should be in either (rule above), and the management network is meant to be isolated. A TLS server would need a certificate for each machine; the matching private key would have to live on the open boot partition, which is exactly what we cannot afford, and a certificate the hub cannot check proves nothing. So TLS adds cost and little in recovery. Whether the Go `crypto/tls` size matters was not measured (UNKNOWN). Also: **the agent cannot prove its own identity to the hub** (no secret to sign with). A fake agent on the management network could lie about status; it cannot make the hub install anything. This is a real weakness, stated plainly; the mitigation is network isolation (section 3) and the fact that actions are limited to what the hub asks and the bundle signature allows.

**Interop was checked against the real tool.** `signify` format: SOURCE: [signify.c, portable version](https://raw.githubusercontent.com/aperezdc/signify/master/signify.c), read 2026-10-04: a signature file is a comment line and the base64 of `"Ed"` + 8-byte key number + 64-byte Ed25519 signature; a public key is `"Ed"` + key number + 32-byte key. [OpenBSD man page](https://man.openbsd.org/signify.1), read 2026-10-04 (summarised): Ed25519.

```
$ PATH=/tmp/p3-x/path:$PATH go test -count=1 -v -run Interop ./tools/image/experiments/recoveryagent/
=== RUN   TestRealSignifyInterop
--- PASS: TestRealSignifyInterop (0.01s)
PASS
ok  	hubos/tools/image/experiments/recoveryagent	0.015s
```

(`/tmp/p3-x/path` held a link to `signify-openbsd` unpacked from the Ubuntu package `signify-openbsd 31-3` with `dpkg -x`; nothing was installed. Without that program on `PATH` the test is skipped and says so.)

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
4. **The status answer is unauthenticated both ways** (section 2.3): a fake agent could receive an install request from the hub. It would only receive a URL for a bundle; the bundle is public anyway.
5. **A network-triggered install from the hub's address** means whoever holds the hub's management key can reinstall every machine at once. Limit blast radius: one machine per request, rate limit in hubd.
6. **The hub itself down:** nothing answers; machines wait in recovery. Accepted by the owner's own words (the rack is next to the desk), but it is a single point of failure.
7. **Watchdog:** recovery does not arm one (section 1); a hung recovery would stay hung. Whether the agent should arm and feed the watchdog is **question 7**.

### 2.6 What is testable in QEMU now, and what was done

| Piece | State | Label |
|---|---|---|
| The request-check logic, status call, one-time nonces, keyring of two keys, replay, tampered body or path, wrong key, expiry, challenge flood limit | Done in `tools/image/experiments/recoveryagent/` (library tests with `httptest`) | **TESTED** (output in section 7) |
| Interop with real `signify-openbsd` signatures | Done | **TESTED** (2.3) |
| Live run on loopback with `curl` and `signify-openbsd` doing the hub's signing | Done, below | **TESTED** |
| The agent as a static binary inside the real recovery kernel; reachable through QEMU's user networking with a host port forward; hubd or curl from the host sees `recovery` | **Not done.** Described in 2.7. Needs a change to `recovery-list.py`, i.e. the image, which this task does not touch. | UNKNOWN |
| A real install through the agent (`hubos-ctl update` run by the agent) | Not done | UNKNOWN |
| A real board's network in recovery, real DHCP reservation, time to come up | Cannot be tested here | UNKNOWN |

Live run (the agent built from the experiment, a throwaway management key made by `signify-openbsd -G -n`, all in `/tmp/p3-x`, deleted afterwards). Real commands and outputs (the signing helper is a shell function that fetches a nonce, builds the text above, signs it with `signify-openbsd -S`, and calls `curl`):

```
$ curl /v1/status
{"machine":"fake-1","state":"recovery","recovery_release":"recovery-1","boot_failures":3,"failure_limit":3}
$ curl -X POST /v1/clear-failures        (no signature)
{"error":"signed request required (GET /v1/challenge first)"}
 [HTTP 401]
$ signed clear-failures
{"result":"cleared"}
 [HTTP 200]
$ signed install   (body {"Slot":"a","BaseURL":"http://10.0.2.2:8000/bundle"})
{"output":"FAKE: would run: hubos-ctl update http://10.0.2.2:8000/bundle a","result":"installed"}
 [HTTP 200]
$ signed logs
FAKE log line 1
FAKE log line 2
 [HTTP 200]
$ replay of the last nonce
{"error":"unknown, used or expired nonce"}
 [HTTP 401]
$ signed with a key the agent does not know
{"error":"signature not accepted"}
 [HTTP 401]
```

### 2.7 How the next step would be tested (described, not run)

1. Build the agent static (`CGO_ENABLED=0`), add it and the management public key to `recovery-list.py`'s file list, start it from `recovery-init` after DHCP.
2. Boot QEMU into recovery with a host port forward (`-netdev user,hostfwd=tcp::18480-:8480`), as the runner already forwards nothing but serves bundles to the guest from the host (`docs/image.md` section 3).
3. From the host: `curl` status; signed `install` with a bundle served by the runner's HTTP server (the guest sees the host as `10.0.2.2`); then the existing tests' check that the machine boots and confirms.
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
| **Reinstall** | state "in recovery" | hubd signs a request (2.3) and asks the agent to install a named signed bundle into a named slot, then watches the agent's status for progress, then waits for the machine to come up. The owner confirms, naming what will be overwritten, like the existing power actions (HUB-OS.md "Power actions"). | design |
| **Power cycle** | only if the machine has a device defined (a relay, a strip outlet or a PiKVM ATX board) | hubd calls that device's own interface. | design; the inventory field does not exist; the form is not decided (**question 3**) |
| Clear failure counter | state "in recovery" | signed request | design |
| Show logs | state "in recovery" | signed request | design |

With no device defined, a dead machine shows "down" and the owner walks over. That is the honest default.

### 5.3 Where addresses and credentials come from (an input for the secrets design)

- **Addresses.** The inventory already holds each machine's address (no secrets in it, HUB-OS.md). The recovery agent is reached at the **same address** (needs a DHCP reservation, section 2.4) on a fixed port (the port is not chosen; not in `default_ports` yet). A power device or PiKVM needs its **own** address; where it lives in the inventory is open: probably a field on the machine naming the device and its address, **not decided** (question 3).
- **Credentials the hub would hold (all secrets; none in the inventory or git):**
  1. the **management private key** that signs recovery requests (new; the update private key stays offline);
  2. a login for each power device (Tasmota password, if set);
  3. a login for each PiKVM (its KVM user) and API;
  4. the two-factor secret if used for PiKVM.
  Where these live on the hub (a file readable only by the hub user, not on the NAS backup, not in the image) is for the secrets design. The compromise of the hub's disk then means: reinstall of any signed release, power cycling, and, with a PiKVM, full control of every machine. That is the cost of option D and should be weighed in December. **Not designed.**
- **In the recovery kernel:** the management **public** key(s), in the same way as the update keyring. Rotation: not designed.
- **Which bundle and where from:** the hub (or the NAS) serves it; the base URL goes into the signed request. Which release (newest, or the floor's own) is a question (question 8); the floor forbids older ones (SOURCE: repo).

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

---

## 7. What was run, in one place

- Worktree: `/home/user/wt-p3` (removed at the end), branch `recovery-oob-proposal`.
- `gofmt -l .`, `go vet ./...`, `go test -count=1 ./...`: output is in the pull request and in the final report. The new package is `hubos/tools/image/experiments/recoveryagent` (standard library only, no new dependency).
- The prototype's tests (names): status needs no signature and shows recovery; mutating calls need a signature; a signed install works once and is bound to method, path and body; a wrong key and a made-up nonce are refused; a second key in the keyring works; a nonce expires; challenges are limited; a bad install request is refused after authentication; garbage key files are rejected; real `signify-openbsd` interop (skipped when the program is not on `PATH`).
- **Not tested:** anything on real hardware; the agent inside the recovery kernel; a real install through it; the busybox-only alternative; TLS; any layer-2 device; every claim marked BELIEVED or UNKNOWN above.
- Pages read through the summarising fetch tool (wording not guaranteed): all `docs.pikvm.org` pages, `pikvm.org/buy`, the GitHub repo pages for PiKVM, OpenBMC and Tasmota, the Eclypsium article, the kernel serial-console and watchdog pages, the OpenBSD signify man page. Pages read as raw files with `curl`: the CISA feed, the NVD API, OpenBMC's README and supported-machines list, ser2net's README and COPYING, conserver's README and LICENSE, PiKVM's LICENSE and README, Tasmota's LICENSE and README, usbrelay's README, signify.c. Not reached: NVD's web page and CISA's catalogue web page (the tool could not show the entry; the feed and API were used instead), the PiKVM `serial_port` and `atx_control` docs pages (404; other pages used), AMI's PDF advisory (not tried).
