# Hub OS — Project Brief

This file is the source of truth for Hub OS. If any other file, chat, or session disagrees with it, this file wins. Only the owner changes it, through the design discussion, and every change is recorded in the Change log at the bottom.

**Rewritten:** 2026-10-01 (replaces the 2026-09-30 brief)

---

## The owner and the situation

- Single owner. New to running clusters. Explanations in the product and in sessions must stay plain.
- Right now the owner has only a phone. No PCs, no cluster hardware, no access to either.
- All hardware will be bought at once in December 2026.
- Until then, work only on what does not need hardware: design, documents, code tested against fake nodes, and our own system booted inside virtual machines in the cloud.
- The bot (Claude Code) builds. Design is decided between the owner and Claude (manager), then handed to the bot.

---

## What Hub OS is

Hub OS has two parts:

1. **A custom Linux-based operating system** that runs on every machine in a personal rack cluster. Each machine is purpose-built for its one role.
2. **On the hub** (the machine at the desk), a thin broker that checks which machines are reachable and opens each machine's display as a normal native window on the driftwm infinite canvas.

The hub is the owner's daily driver. Every capability is outsourced to a specialized node. The hub runs nothing but viewers.

## Core principles

- **The hub is a client.** It never runs workloads, never stores the main file pool, and never re-encodes or proxies video. It runs only viewers (remote display clients); which protocol non-gaming nodes use is being researched (see docs/proposals/remote-display.md). Even the web browser and text editor run on a node. There are no terminal windows: every node has its own bespoke GUI, shown in a window on the hub.
- **Every workload node is purpose-built for its job.**
- **Build it ourselves when it measurably improves Hub OS.** For every part we replace, state what our version does better for this cluster and verify it. If a proven part does the job equally well, use it.
- **The Linux kernel stays.** Moonlight, driftwm, and GPU drivers depend on it.
- **glibc everywhere** (required by Steam/Proton and likely by GPU drivers).
- **Say when something is unverified.** Never present a guess as fact. Never invent a protocol.
- No systemd, ever.
- **Scale:** real-world reference is at most 20 machines; hardware, cabling and the hub's open windows are sized for 20. The panel and hubd keep headroom for 100 machines; 5000 is a stretch target: it must not break, and the measured numbers are recorded before anything is claimed.

---

## Physical setup

- The rack sits next to the desk.
- **Hub display:** one ultra-short-throw projector. The hub has a single view of the canvas.
- **Gaming monitors:** one or two, cabled directly to the gaming box's GPU. They are not part of the hub.
- **Input:** one keyboard and one mouse, plugged into the gaming box (see Input sharing). A spare keyboard and mouse in a drawer can be plugged into the hub in an emergency.
- **Network:** a fast wired network (10GbE-class) between machines. Every machine may reach the internet.
- **Power:** treated as unlimited, free, and never failing. No battery backup (UPS). Electricity cost is not a design factor.
- **Machines:** every machine uses an AMD CPU; GPUs are chosen per machine. No proprietary BMC firmware is used anywhere: boards are not required to have a BMC and their BMCs are not used. Recovery is built into Hub OS (hardware watchdog, boot-loop breaker, A/B rollback, and a recovery kernel with a network recovery agent that hubd talks to). Out-of-band control (power cycle, screen, BIOS) for a machine whose firmware or kernel is dead needs hardware wired to that machine; whether to have it, and which open device (a PiKVM-class device, a relay or a switched power strip), is decided in December. Any such device sits on its own isolated management network. Out-of-band devices, if any, sit on an isolated management network; the recovery agent lives on the normal network.

---

## Machine roles (current list; more may be added later; counts TBD) (real-world reference: at most 20 machines)

| Role | Purpose | How the hub shows it |
|---|---|---|
| Hub | Daily driver; runs only viewers | This machine runs the broker |
| Gaming box (one) | All games, Linux only | Moonlight window for casual play; competitive play on its own monitors |
| AI / GPU box | Training, inference, Blender, ParaView, CUDA | Moonlight window (Sunshine on the node) |
| General desktop node | Web browser, text editor, everyday apps | Moonlight window |
| NAS | Cluster file storage | Its own bespoke GUI in a window (the bespoke file manager is part of it) |
| Backup NAS | Local backup copy of the NAS | Listed; opened like the NAS |
| VM host | Operating system development and experiments | Its own bespoke GUI in a window; its guests open with remote-viewer (virt-viewer) |

Every node runs the Hub OS system. TrueNAS and Proxmox are **not** used. The hub stays a thin client: the browser and everyday apps run on the general desktop node (owner decision, 2026-10-04).

---

## The Hub OS system (all machines)

- **Immutable and declarative.** One configuration describes each machine.
- **Whole-system image.** An update is written to a second slot and the machine reboots into it. If it fails, the machine boots the previous slot.
- **Per-machine settings live outside the image** in a small config: tuning profile, inventory, secrets. Changing them does not require a new image.
- **Each node keeps its own system on its own disk.** A node can restart without the hub.
- **Updates come from the hub over the network, only when the owner chooses.**
- **First install of a new machine: network boot.**
- **Images are digitally signed.** A machine refuses an image that is not signed by the owner.
- **A machine refuses to update or restart while a game or long job is running.**
- **Master copies of the code and every built image live on the NAS.** GitHub is a convenience mirror. Long term, builds happen on a machine inside the cluster, so the cluster can rebuild itself if GitHub disappears.
- **Init:** start with an existing small init (candidates: s6, dinit), kept swappable. Write our own only once a measurable benefit is shown. systemd is never used, under any circumstances.
- systemd programs are never installed or run. The hub uses eudev's libudev (not the systemd-built one); libsystemd0 is tolerated for now as a plain library because Waybar and dbus-daemon link it. Headless role images may drop it later with rebuilt packages. See docs/proposals/systemd-libraries.md and docs/proposals/phase-b-desktop.md.
- The image build pins systemd, systemd-sysv, libpam-systemd, dbus-user-session, udev, systemd-timesyncd and systemd-resolved to never install, and a build test fails if any of them, or any systemd unit directory, is in an image.
- Finished images do not contain apt, PAM modules, procps, login and passwd (dpkg stays for now; it is an Essential package, so only its files could be deleted later). They are deleted after the build, and a test checks that no remaining file has an unresolved library. Nodes are never changed with a package manager.
- **Hub recovery mode:** a boot option that gives a bare terminal, plus rollback to the previous image from the boot menu.
- Recovery is a separate kernel (kernel-recovery.efi) that needs neither slot's root; its shell can install a signed bundle into a slot the owner names (manually; automatic repair for machines without a keyboard is a proposal only).
- A boot-loop breaker counts failed boots in an EFI variable: stage 0 increments it at every boot, the confirm step clears it, and after N failures (default 3, per-machine setting) stage 0 starts the recovery shell instead of booting.
- The recovery kernel arms the hardware watchdog and the network recovery agent feeds it.
- The network recovery agent gets its address from a DHCP reservation per machine; its API is signed requests with one-time nonces, one API with the node helper (a state field says recovery or running). Reinstall is a button only, never automatic; the default release is the last one that machine confirmed, into the slot that failed.
- wayvnc and neatvnc are built from source at pinned tags (wayvnc v0.10.2 with neatvnc v1.0.3 or newer; Ubuntu 24.04's 0.7.2 garbles non-ASCII clipboard text); every node's wayvnc is started with --name <machine id>; sessions are non-shared; the screen size is fixed per node in v1.
- Display credentials are per node (the secrets design decides how they are made and stored); the management key's private half lives on the hub's config partition, readable only by root and hubd, never on a node; nodes hold only public keys.
- The recovery agent is supervised by a plain shell restart loop in the recovery kernel. The boot partition of real machines is 512 MiB.
- Requests to nodes and to the recovery agent name the target machine id in the signed text. The node helper and the recovery agent share one port (8480, to be checked against the IANA registry) and one API. The control link is signed but not encrypted on the isolated wired network for v1; TLS is revisited when the secrets tool exists.
- The update signing key's private half lives only on a device that is never connected to a network, protected by a passphrase, with an encrypted second copy elsewhere; until December only throwaway test keys exist. One offline key plus a rehearsed two-release rotation is enough for v1. Display credentials are one file per node on the hub; rotation is always the owner's explicit action. The config partition is not encrypted (physical access is full access). Game account logins are not managed by Hub OS and are excluded from the NAS save copy. The hub user is split: hubd (holds the management key and display credentials) and a desktop user (viewers), with a launcher handing credentials to viewers (design in docs/proposals/hub-user-split.md).
- The recovery kernel is carried inside every slot's root, listed by hash in the manifest, and installed at the confirm step after a healthy boot, never during the update.
- The confirm timeout and the watchdog timeout are per-machine settings (real defaults 120 s and 180 s; the test image uses 30 s and 60 s); a check refuses a configuration where the watchdog does not exceed the confirm timeout plus a margin.
- Update keys: the update tool and the recovery kernel accept any key in a keyring (/etc/hubos/keys/*.pub). A release signed with the old key can carry the next key; the old key is dropped only in a later release.
- **Sound:** every node's sound plays on the hub at the same time, mixed by PipeWire; each node has its own mute and volume control, plus a master volume and a mute-all control. How audio travels (inside the display protocol or as its own stream) is being researched; see docs/proposals/remote-display.md.
- A D-Bus session bus (dbus-daemon alone, no systemd) may be declared by any machine's configuration; the hub needs one because Waybar will not start without it.
- Kernels are independent per machine. Each machine's configuration names its own kernel (version, build options, extra modules); nothing is inherited from a role. Roles are text labels only: the inventory gives behaviour to hub, guest and vm-host and to nothing else. One default kernel is set in the build configuration; a machine that overrides it records the reason in its config.
- Out-of-tree kernel modules (NVIDIA's driver, OpenZFS) are allowed only when a machine's configuration declares them. They are built together with that machine's kernel from one pinned source version; if the build fails, no image is produced. They are listed in the signed manifest and covered by the boot tests.
- Kernel and graphics-stack versions are pinned and tested before rollout; nothing tracks 'latest'. Each image boots in a test machine before it is offered. The NAS and the backup NAS are updated a week apart, and the backup NAS only after the main one has run well.

### Parts we write ourselves

- The image build and update tool
- `hubd` (Go): inventory, health checks, launching viewers, record of what was started
- The panel (bar item and dropdown)
- The node helper (status, start/stop session servers)
- The input forwarder
- The game picker
- The game tuner
- A bespoke file manager for Hub OS (likely; design discussion first)
- A clipboard bridge between the hub and the machines (design discussion first)
- Possibly our own compositor (only after a design discussion)
- Anything else that measurably improves Hub OS (decided case by case with the owner)

---

## Hub behavior

### Windows and sessions
- Each machine opens as its **whole desktop in one window**, not individual apps.
- **Closing a window leaves the machine's session alive.** Clicking again returns to it.
- Clicking a machine that is already open **goes to its existing window**. Never open duplicates.
- **Every machine stays logged in**, so clicking lands straight on its desktop.
- Clipboard: text only, both directions wanted. Node to hub works through the display protocol; hub to node goes through a clipboard bridge via the node helper's API (design discussion first). The hub does not run an X11 compatibility stack for a viewer.
- **No auto-reopen of windows after a hub restart.** The panel returns; windows do not.
- Moonlight windows are matched by their title '<machine id> - Moonlight'; every node sets Sunshine's name to its machine id. The inventory's optional session names the Sunshine app to stream.
- Windows can stay open 24/7. The hub is designed for up to 20 always-open windows (the real-world reference); the owner can open and close them at will.
- VM guests are opened with remote-viewer (virt-viewer), not Remmina.

### Canvas
- driftwm is **essential**. If it does not work on our system, we wait until it does. There is no fallback desktop.
- Writing our own compositor to replace driftwm is allowed if it is better and improves Hub OS, but only after a design discussion with the owner.
- Each machine has a **fixed home position** on the canvas, assigned in the inventory.
- Windows stay on the canvas. driftwm never magnifies: zoom stops at 100%, so 'zoom in' only means back to native size. Larger text comes from the output's scale setting and from window size, and is untested. 'Maximize' means driftwm's fit-to-viewport.
- Navigation is by mouse.
- The hub desktop is meant to run as a normal user in the seat group, not as root (untested).
- Hub desktop: driftwm's session state lives in RAM (windows are never restored after a reboot). Waybar, hubd, driftwm and dbus-daemon are s6 services run as the normal user hub; seatd and udevd run as root (they open the GPU, input devices and netlink). Waybar and hubd wait for driftwm's socket and restart when it returns. No terminal starts by itself.
- hubd keeps windows it places clear of the bar, and expects the camera to be offset by half the bar height.
- hubd controls driftwm through its local socket (list windows, place a window, move the view, focus, resize, fit). The socket is only for the same user. See docs/driftwm-findings.md.
- driftwm is pinned to one exact commit (352333a8fa1b22171492d4b71a54102045c9a19d, version 0.19.0). It is GPL-3.0-or-later; anything changed in it is published under that licence. It is a single-maintainer, pre-1.0, AI-built project, so expect to carry patches. Game-style windows use the per-window pass_keys = true rule; no patch for the shortcut-inhibit protocol for now.

### Panel
- An always-visible bar item that opens a scrollable dropdown, grouped by role. Guests are nested under their host.
- The bar is Waybar, on the top edge. The alert is a Waybar custom module fed by hubd. The dropdown is a list launcher (wofi) opened by a click on the bar item and filled by hubd; lines that are group headings are ignored if picked. A bespoke panel may replace this later. See docs/bar-findings.md.
- Fullscreen hides the bar and its alert, so Moonlight windows use driftwm's fit-to-viewport by default, not fullscreen.
- When a machine is down, the **bar item itself shows an alert** (for example, "3 of 4 up" in red).
- A machine that is off just shows as down. No wake-on-LAN.
- If a machine drops while its window is open, leave the window alone. Only the alert changes.
- The bar shows which machine currently owns the keyboard and mouse (hub or gaming box).
- No alerts are sent to the owner's phone.
- Panel states: up, down (that machine does not answer), in recovery, unreachable (the hub suspects its own network, for example many machines failing at once).
- A master volume button on the bar opens a mixer that lists every node's sound with its own volume slider and mute button, plus a master volume and a mute-all control. Each node's volume and mute are remembered and restored when its window opens; a node with no saved state starts unmuted at the default volume (owner to confirm).

### Leaving a window
- **Desktop-style windows** (AI box, general desktop, NAS, VM guests): the pointer stays free; leave by clicking outside the window.
- **Game-style windows** (casual play through the hub): for v1, use Moonlight's built-in Ctrl + Alt + Shift + Z to release mouse and keyboard. A custom exit chord may come later, after testing how driftwm handles shortcuts.

### Power actions (after the first working slice)
- Each machine gets Restart and Shut down, with a confirmation that names what will be lost (for example, "this ends the open desktop session on the AI box").

### Adding machines
- v1: hand-edit the inventory file. A guided "add machine" step may come later.

---

## Input sharing

- One keyboard and one mouse plug into the **gaming box**, so the gaming path has nothing in between.
- **Super + Ctrl + Shift** flips input between the gaming box and the hub.
- When input is on the hub, the gaming box forwards keystrokes and mouse movement over the network with **our own forwarder**.
- The forwarder sends a "release all keys" on every flip so no key stays stuck. Whether it also hides the chord from games depends on the forwarder design (always grab and re-inject locally, or grab only while forwarding); that choice is decided after the December measurements; see docs/input-sharing-research.md.
- The forwarder must pass modifier keys held together with mouse buttons, because driftwm's mouse bindings need Alt or Super.
- After a restart, input goes to the hub by default.
- If the gaming box is down, use the spare keyboard and mouse on the hub.

---

## Gaming box

- **Linux only.**
- **Games:** Diablo II: Resurrected (through Proton; needs a Battle.net account), Slippi (Super Smash Bros. Melee netplay), emulators for every console from Atari through PS2, GameCube, and original Xbox, Soul Calibur II netplay (emulator TBD), and Minecraft (edition TBD).
- **Design goal:** remove I/O chokepoints and bottlenecks, and minimize jitter. Network latency is ignored in this design.
- **Boots straight into a game picker:** a menu of games, each with its own optimized profile. Nothing else runs.
- **Two layers of tuning:**
  - Per machine, set at boot (real-time kernel, reserved CPU cores, and similar), measured on the actual hardware.
  - Per game, set at launch (CPU governor, priority, core assignment, emulator settings).
- **Game tuner:** an advanced built-in tuner that measures and suggests settings. It writes the same profile files the owner can edit by hand. When delay and smoothness conflict, the default is the **steadiest picture**, overridable per game. The profile file format comes first; the tuner comes after the first working version.
- The tuner can measure what the box can see (frame-time consistency, scheduling delay). True click-to-screen delay needs external measuring hardware.
- **Game files live on the box's own SSD.** Saves are copied to the NAS automatically.
- Emulators ship inside the gaming box's sealed image as pinned upstream binaries (AppImage or standalone), listed with their SHA-256 in the machine's build config and run with extract-and-run (no FUSE). They are built from source only where no suitable binary exists or a patch is needed. No Flatpak. Steam, game files and Proton prefixes live on the box's own SSD data area.
- N64: RMG by default (Gopher64 is the second candidate to test in December). NES: MesenCE (its Linux release format is still to be checked).
- The gaming-box kernel configuration enables user namespaces (Steam's container runtime needs them).
- **Competitive play** happens on the gaming monitors, with zero streaming delay.
- **Casual play** can happen in a Moonlight window on the hub, showing the same picker.

---

## NAS and backups

- **Full protection:** redundant disks, detection and repair of silent corruption, snapshots, ECC memory. Engineering effort and budget are not limits.
- **Filesystem:** OpenZFS on the NAS and the backup NAS (an out-of-tree module declared in their configs, on a kernel version OpenZFS supports). A restore from the backup NAS must be tested before either machine is trusted. Btrfs stays the fallback if OpenZFS cannot be built for a needed kernel.
- **Workloads that need different tuning** (for example, AI datasets) get separately tuned areas on the same NAS. The AI box keeps its active dataset on its own fast SSD; the NAS holds the master copy.
- Opens on click in the hub's file manager. Non-Linux devices do not need access.
- **Backups:** a local backup NAS, plus a copy off-site or in another room. No encryption.
- The inventory and viewers.toml get the NAS backup copy. Secrets do not; secrets handling is a separate step.
- /etc/hubos/wofi.css (the menu look) is a backed-up per-machine file too, and gets the NAS backup copy next to the inventory and viewers.toml.

## VM host

- Runs both operating systems the owner writes and existing ones.
- Built on Linux's built-in virtual machine support (KVM), with our own management.
- **Per guest:** ephemeral (reset to a clean copy) or persistent (keeps changes, can be custom-built for a workload).
- A guest whose workload needs it can move to its own hardware node.

## Remote access

- Allowed for **non-gaming nodes only**, after the first working version, as a separate locked-down piece with a real login and encryption. It may use a web interface; this is the only exception to the no-web rule.

---

## Explicitly out

- Selkies, Apache Guacamole, NICE/Amazon DCV
- Any new streaming protocol
- A web dashboard as the hub's control plane (except the remote-access exception above)
- A kernel written from scratch (Hub OS uses upstream Linux, built per machine)
- TrueNAS, Proxmox
- A multi-scheduler layer (Slurm + Kubernetes + Proxmox)
- Auto-reopening windows after a hub restart
- Wake-on-LAN
- Phone alerts
- Battery backup (UPS)

---

## Phases

1. **Phase A (now, phone only):** design documents, plus `hubd` with fake nodes. Finish line: the panel's tiles work for fake nodes (up/down, click opens the right program).
2. **Phase B:** the bot builds the Hub OS system and boots it in virtual machines in the cloud.
   - Runs under software emulation only (no hardware virtualization in the build environment). Use very small images first. Performance, latency and jitter cannot be measured there; those tests wait for Phase C.
3. **Phase C (December 2026):** real hardware.

**Overall finish line (owner's words):** Hub OS boots the whole cluster. Every machine runs the Hub OS system, and the hub shows all of them as tiles that open their windows.

## Health: what "ready" means

Ready means the machine's session server is accepting connections:

- Moonlight machines (gaming, AI, desktop): Sunshine accepts connections.
- Guests: SSH or the SPICE/VNC port answers.
- NAS and backup NAS: the share or SSH answers.
- VM host: SSH answers.
- The hub is the machine hubd runs on and is not checked.

"Machine is up" never means "the hypervisor says running". Default check ports come from the [default_ports] table in viewers.toml (examples/viewers.real.example.toml: Sunshine 47989, ssh 22, SMB 445; unverified on hardware). VM guests have no default; the inventory gives the port.

## Repo layout

Directory level only. Run `git ls-files` for the full list.

```
HubOS/
├── CLAUDE.md, HUB-OS.md              (rules for every session; this brief)
├── go.mod, go.sum                    (one Go module for the whole repo)
├── docs/                             (design notes and research; each file names its sources and labels)
│   └── proposals/                    (proposals, not decisions: the Phase B image, the systemd libraries)
├── examples/                         (example inventory, viewers.toml files and the menu style; documentation addresses only)
├── cmd/hubd/                         (the hubd program: flags, wiring, slice 1 and slice 2 commands)
├── internal/
│   ├── driftwm/                      (driftwm socket client)
│   ├── hub/                          (state, checks, menu list, open/end, record, socket)
│   ├── inventory/                    (read and validate the inventory; no network)
│   ├── probe/                        (up/down checks; no inventory knowledge)
│   └── viewers/                      (viewers.toml reader)
├── testdata/
│   ├── broken/                       (one deliberately broken inventory per rule, and a README.md)
│   └── viewers/                      (test viewer tables and a fake two-window viewer)
├── tools/
│   ├── fakenode/                     (tiny program that pretends to be a machine)
│   ├── geninv/                       (writes big fake inventories at run time for scale tests)
│   └── image/                        (Phase B build scripts and the QEMU test runner, build tag "qemu")
└── image/                            (what goes inside the machine images)
    ├── apt/                          (apt pin: the systemd packages are never installed)
    ├── config/                       (a machine's config partition files)
    ├── kernel/                       (kernel configuration fragments)
    ├── machines/                     (machine build configurations)
    ├── packages/                     (package lists)
    ├── rootfs/                       (files copied into the root: /sbin/init, hubos-ctl, s6 service directories)
    └── stage0/                       (stage 0, built into the kernel)
```

---

## Rules for the bot

- **Ask the owner before every choice.** Questions may be batched.
- Do only the task given. Do not redesign.
- Never invent architecture or protocols. Mark anything unverified.
- Secrets never go in the inventory or in git.
- Speak plainly.
- Do not edit this file unless the owner asks, and record every change in the Change log.

---

## Unverified — must be tested

- How driftwm behaves on a projector, and how readable small text is
- Whether PipeWire runs with driftwm on the real Hub OS image (driftwm itself ran without systemd in the build environment)
- How Moonlight and driftwm hand keyboard and mouse input back and forth
- Whether a Sunshine session stays alive after the viewer closes
- What the gaming monitors show while the gaming box is being streamed
- Clipboard and audio support in each viewer
- Streaming delay on the real network and hardware
- The input forwarder approach (reading raw input devices and creating virtual ones)
- How much management software the VM host needs on top of KVM
- Whether NVIDIA's driver requires glibc
- Which per-game settings can change at launch without side effects
- A report of Slippi dropping frames on Linux where Windows was smooth (one user's report)
- Whether a bare TCP connect-then-close disturbs a real Sunshine, SPICE or VNC session
- The hubd time limits (2 s per machine, 5 s total) are guesses, not measured on a real network
- Whether /etc/hubos/inventory.toml is where per-machine config will live on the Hub OS image
- Whether the input forwarder can be tested at all: /dev/uinput is absent in the build environment (a test would need a virtual machine with a kernel built with uinput)
- driftwm's real-display backend on a real or virtual GPU (only the nested software mode was run)
- The window names and titles real Moonlight reports (remote-viewer, Remmina, foot and pcmanfm were tested under driftwm; Moonlight is read from source only)
- Moonlight opens a status window first, with the fixed app-id and a believed title 'Moonlight'; where it appears and what it does is unknown (source only); hubd ignores it
- Pointer constraints with a real Moonlight session
- Text readability on the projector, and output scale
- A release build of driftwm: speed and memory
- Waybar as a non-root user under s6 or dinit
- wofi single-click selection was tested with xdotool in the build environment only
- hubd, the panel and wofi at 100 and at 5000 machines
- Copy-paste across windows: Moonlight does not share the clipboard from the machine back to the hub (it only types the hub's clipboard text on the machine); see docs/viewers-research.md
- remote-viewer shows a small error dialog that carries the chosen app-id when the connection fails; hubd cannot tell it from the viewer window
- Moonlight window title '<id> - Moonlight' with Sunshine's name set to the machine id (read from source; Moonlight was never run)
- tmux new-session -A -s hubos over ssh (not run against a real server)
- libudev-zero with libinput, driftwm, OpenZFS and QEMU/libvirt (symbols match; nothing was run with them)
- Real logins through sshd without PAM modules (only sshd -t was run)
- EFI variable failure counter and NVRAM wear on real firmware
- Recovery installing a bundle from its own shell
- libinput's device database without systemd (eudev) and the hub desktop as a normal user
- eudev's libudev with libinput and driftwm on real hardware (QEMU only)
- Pinned emulator AppImages run on the image's glibc with extract-and-run (nothing was run)
- Steam and Proton on the image: 32-bit libraries, user namespaces, no systemd
- Hub-to-node clipboard: remote-viewer never sends clipboard text; the TigerVNC viewer does only while its window has focus and needs Xwayland (not planned)
- wayvnc 0.10 or newer with neatvnc 0.9 or newer needs a source build on Ubuntu 24.04
- The recovery agent inside the real recovery kernel, supervised, on real hardware
- The real clipboard echo path (the node helper's wl-copy, wayvnc, the viewer, the hub clipboard) behaves like the stand-in used in tests
- wl-paste (one-shot and --watch) under driftwm
- hubd clipboard push, the focused-window-to-machine lookup and the bar button (not built)
- Signed PUT and GET for the node helper, the full signed round trip, and a node helper binary run as an s6 or dinit service (not built)
- The init collects the orphaned wl-copy holder processes (believed for s6, unknown for dinit)
- A thin init-neutral "service up/down" command (does not exist)
- Node helper session start and stop order, the sound retry, and wayvnc -R keeping the screen size fixed (design only)
- Compositors that offer ext_data_control but not zwlr_data_control (wl-clipboard 2.2.1 would fall back to its window hack)
- Node helper behaviour with many nodes at once, in a long run, on a real network, with TLS
- Hub-to-node clipboard echo through remote-viewer overwrites the hub clipboard with a garbled copy (believed, not tested)
- Per-node audio control: matching a PipeWire stream to a node (by process id or by stream name)
- How many simultaneous hardware decodes the hub's GPU sustains with 20 open windows
- The load of continuous remote-display encoding on each node
- The hub's confirm and watchdog timeouts (90 s and 120 s) are test values; real hubs use the defaults until measured
- A PiKVM-class device on the chosen boards: BIOS visibility, keyboard in the BIOS, front-panel wiring

## Open questions for the owner

- GPU brands for each machine
- The full hardware list (research before December), including a keyboard and mouse that keep their polling rate through the forwarder
- Number of machines per role
- Minecraft Java or Bedrock
- Soul Calibur II netplay: Dolphin (GameCube, delay-based) or Ring Out (a recompiled port, delay-based, five weeks old); decide in December
- A custom exit chord for game-style windows
- Security note: every machine stays logged in and can reach the internet, so incoming connections from the internet must stay blocked (except the future remote-access piece). Owner to confirm.
- Hub service design (s6 or dinit): restart Waybar and hubd when driftwm restarts; set --bar-height and ulimit -n
- Display protocol for non-gaming nodes: the plan is VNC with wayvnc on the nodes, PipeWire RTP for sound (raw), and one signed control API; the AI box uses VNC until the December measurements, with Sunshine as its fallback; RDP is out of scope for now (docs/proposals/remote-display.md)
- Clipboard: text only, both directions wanted; node to hub works in tests, hub to node is unresolved
- Where the mixer settings live: the hub's config partition, included in the NAS backup
- Which additional nodes to build, and when (suggested, not decided: a hardware test bench of two or three cheap boards, a build and test node with KVM, a release and netboot node, a spare hub)
- Whether the no-systemd rule covers appliances that are not Hub OS machines (for example a PiKVM, which runs its own Linux); a December decision
- Where each key and credential lives and how it rotates (see docs/proposals/secrets.md)
- Clipboard push: a bar button and a key chord (the chord chosen after checking driftwm's bindings); the target is the focused window's machine, with a pick from the list; a pull button as fallback; no automatic push; size limit 1 MiB; hubd ignores a differing echo of the text it just pushed
- Which viewer the hub uses (remote-viewer, TigerVNC, wlvncc or one we write; see docs/proposals/remote-display-benchmarks.md)
- The clipboard bridge and the node helper API (see docs/proposals/node-helper-api.md)
- The network recovery agent in the recovery kernel: protocol, authentication, what hubd can ask it, and whether an automatic repair mode exists (see docs/proposals/recovery-and-out-of-band.md)
- Out-of-band hardware per machine (power cycle, screen, BIOS): none, a PiKVM-class device, a relay or a switched power strip; decided in December
- Moonlight pairing is by hand once per node for v1; secrets design later (pairing is scriptable: moonlight pair --pin and Sunshine's PIN API)
- AI box: NVIDIA with CUDA, or AMD with ROCm (December hardware decision)
- Forwarder design: always grab and re-inject, or grab only while forwarding (decided after December measurements)
- N64: confirm RMG against Gopher64 in a December test
- MesenCE's Linux release format (AppImage or other): the owner checks
- Whether the Battle.net desktop app is needed for the Steam edition of Diablo II: test in December
- Whether Steam runs without systemd on the image (32-bit libraries, user namespaces): test in December
- Unattended recovery for machines without a keyboard: after December, when the boards' serial and BMC options are known

---

## Parked ideas

- Erlang/Elixir for uptime: parked; revisit later. v1 uses Go.

---

## Change log

- **2026-09-30:** Original brief.
- **2026-10-01:** Full rewrite after design discussion. Major changes from the original:
  - A custom Linux-based system for every machine is now in scope (the original ruled out a custom distro).
  - Gaming is streamed to the hub for casual play; competitive play stays on direct monitors (the original ruled out streaming gaming PCs).
  - One gaming box replaces several gaming PCs.
  - TrueNAS and Proxmox removed; NAS and VM host run Hub OS.
  - No fallback desktop: driftwm is essential.
  - Hub uses a single projector instead of multiple monitors.
  - A custom init is allowed if justified; start with an existing one.
- **2026-10-01 (later):** systemd banned permanently; custom compositor and custom file manager allowed after design discussion; docs/DECISIONS.md removed; Erlang/Elixir recorded as parked.
- **2026-10-01 (later):** Inventory file format approved; see docs/inventory-format.md.
- **2026-10-01 (later):** Inventory format amended: real inventory lives on the hub's own disk with a NAS backup copy; added validation rules; defined behavior for unknown format versions.
- **2026-10-01 (later):** Added health definition and repo layout; hubd first-slice design approved; exactly one hub required.
- **2026-10-01 (later):** hubd first slice reviewed; extra validation rules documented; hub excluded from the up count; unverified list extended.
- **2026-10-01 (later):** Build environment audited; findings in docs/environment.md; Phase B limits recorded.
- **2026-10-01 (later):** driftwm investigated; findings in docs/driftwm-findings.md; zoom wording corrected (never magnifies); modifier requirement added for the forwarder; unverified list updated.
- **2026-10-01 (later):** Bar investigated; findings in docs/bar-findings.md; panel decisions recorded (Waybar alert plus wofi list, top edge, fit instead of fullscreen, D-Bus session bus allowed); scale target 100, stretch 5000.
- **2026-10-01 (later):** hubd second slice: feed, list, menu, pick, open, end; scale tested at 100 and 5000; see docs/hubd-slice2.md.
- **2026-10-01 (later):** hubd slice 2 follow-ups: bounded check, down-machines group, forget command, viewer logs, stale-round rule.
- **2026-10-01 (later):** hubd slice 2 reviewed and merged; STALE rule, first-round display, safety fixes and viewer-log cap.
- **2026-10-01 (later):** hubd slice 2: late-window state for slow viewers, per-viewer window_wait, escape and timeout flags.
- **2026-10-01 (later):** hubd slice 2: late-window follow-ups (end message, stop-waiting menu line, per-viewer late_grace, clean exit keeps waiting, ignore list).
- **2026-10-01 (later):** owner decisions on viewers recorded: title matching for Moonlight, session field, tmux for ssh, remote-viewer for guests, default check ports, clipboard limit accepted for v1.
- **2026-10-01 (later):** Unverified list: Moonlight clipboard limit and the remote-viewer error dialog added.
- **2026-10-01 (later):** default check ports moved to a [default_ports] table; HUB-OS.md contradictions fixed (guest viewer, clipboard line, repo layout).
- **2026-10-01 (later):** HUB-OS.md cleaned up (duplicate clipboard line, Moonlight unverified items); NAS opens with ssh until the bespoke file manager; pairing by hand for v1.
- **2026-10-02:** Owner decisions: independent kernels per machine, out-of-tree modules only when a machine declares them, kernel pinning and staged NAS updates, OpenZFS on the NAS and backup NAS; Phase B choices recorded in docs/proposals/phase-b-image.md.
- **2026-10-02:** Phase B slices S0 to S3 built in the repo: tools/image, image/, a QEMU test runner; see docs/image.md.
- **2026-10-03:** Owner decisions on the systemd libraries and image rules: programs never, libraries tolerated for now, apt pin and build test, apt/PAM/procps deleted after the build.
- **2026-10-03:** Phase B images: per-slot kernels (slot choice independent of firmware load options), free-space check, systemd pin and strip step.
- **2026-10-03:** Reviewed the games and input research; forwarder design left open until December; emulator packaging recorded as open; apt bullet reworded.
- **2026-10-03:** Phase B images: floor only rises, recovery detected by BootCurrent, one-kernel manifest refused, hang-retry exercised.
- **2026-10-03:** Games research reviewed: emulators ship as pinned binaries inside the gaming-box image, user namespaces in the gaming-box kernel, D-Bus session bus declarable per machine, N64 default RMG, layout simplified.
- **2026-10-03:** Phase B images: separate recovery kernel, confirm refuses below the floor, rollback command.
- **2026-10-03:** Recovery and boot-loop decisions recorded: separate recovery kernel updated at confirm, failure counter in an EFI variable (N=3), per-machine timeouts, normal-user hub desktop.
- **2026-10-03:** Phase B images: recovery update at confirm, recovery can install, boot-loop breaker, per-machine timeouts.
- **2026-10-03:** First hub image: keyring, hubd menu fix, hub.build with the desktop services and its QEMU test.
- **2026-10-03:** Hub desktop decisions recorded: eudev, services as the normal user, driftwm state in RAM, key keyring.
- **2026-10-04:** Phase B images: recovery installs on a changed hash, menu row click tested, QEMU crash retry.
- **2026-10-04:** AMD CPUs only, no proprietary BMC firmware (recovery built into Hub OS; out-of-band hardware decided in December), no terminals (every node has its own bespoke GUI), always-open windows on the hub, all nodes' sound at once with per-node mute and volume, hub services corrected (seatd and udevd run as root).
- **2026-10-04:** Phase B images: recovery kernel arms the watchdog; recovery agent tested inside a test recovery kernel.
- **2026-10-04:** Scale reference 20 machines; hub stays a thin client; extra nodes suggested; recovery and display protocol decisions recorded; CLAUDE.md rules for packages and helper agents.
- **2026-10-04:** Phase B images: test recovery agent supervised by a restart loop.
- **2026-10-04:** Round 2 decisions recorded: clipboard bridge for hub to node, wayvnc built from source, per-node display credentials, management key on the hub, recovery agent supervision, Go stays (Erlang parked).
- **2026-10-04:** Round 3 decisions recorded: signed text names the machine, shared port, signing key custody, per-node credential files, user split, clipboard push rules.
