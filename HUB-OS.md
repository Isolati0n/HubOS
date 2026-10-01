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

- **The hub is a client.** It never runs workloads, never stores the main file pool, and never re-encodes or proxies video. It runs only viewers: Moonlight, virt-viewer/Remmina, a terminal used as an SSH client, and the file manager used as a NAS client. Even the web browser and text editor run on a node.
- **Every workload node is purpose-built for its job.**
- **Build it ourselves when it measurably improves Hub OS.** For every part we replace, state what our version does better for this cluster and verify it. If a proven part does the job equally well, use it.
- **The Linux kernel stays.** Moonlight, driftwm, and GPU drivers depend on it.
- **glibc everywhere** (required by Steam/Proton and likely by GPU drivers).
- **Say when something is unverified.** Never present a guess as fact. Never invent a protocol.
- No systemd, ever.

---

## Physical setup

- The rack sits next to the desk.
- **Hub display:** one ultra-short-throw projector. The hub has a single view of the canvas.
- **Gaming monitors:** one or two, cabled directly to the gaming box's GPU. They are not part of the hub.
- **Input:** one keyboard and one mouse, plugged into the gaming box (see Input sharing). A spare keyboard and mouse in a drawer can be plugged into the hub in an emergency.
- **Network:** a fast wired network (10GbE-class) between machines. Every machine may reach the internet.
- **Power:** treated as unlimited, free, and never failing. No battery backup (UPS). Electricity cost is not a design factor.

---

## Machine roles (current list; more may be added later; counts TBD)

| Role | Purpose | How the hub shows it |
|---|---|---|
| Hub | Daily driver; runs only viewers | This machine runs the broker |
| Gaming box (one) | All games, Linux only | Moonlight window for casual play; competitive play on its own monitors |
| AI / GPU box | Training, inference, Blender, ParaView, CUDA | Moonlight window (Sunshine on the node) |
| General desktop node | Web browser, text editor, everyday apps | Moonlight window |
| NAS | Cluster file storage | File manager (opens on click) and/or terminal |
| Backup NAS | Local backup copy of the NAS | Listed; opened like the NAS |
| VM host | Operating system development and experiments | Its guests open with virt-viewer/Remmina or SSH |

Every node runs the Hub OS system. TrueNAS and Proxmox are **not** used.

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
- **Hub recovery mode:** a boot option that gives a bare terminal, plus rollback to the previous image from the boot menu.
- **Sound:** all audio plays through the hub (carried by the viewers). Likely PipeWire.

### Parts we write ourselves

- The image build and update tool
- `hubd` (Go): inventory, health checks, launching viewers, record of what was started
- The panel (bar item and dropdown)
- The node helper (status, start/stop session servers)
- The input forwarder
- The game picker
- The game tuner
- A bespoke file manager for Hub OS (likely; design discussion first)
- Possibly our own compositor (only after a design discussion)
- Anything else that measurably improves Hub OS (decided case by case with the owner)

---

## Hub behavior

### Windows and sessions
- Each machine opens as its **whole desktop in one window**, not individual apps.
- **Closing a window leaves the machine's session alive.** Clicking again returns to it.
- Clicking a machine that is already open **goes to its existing window**. Never open duplicates.
- **Every machine stays logged in**, so clicking lands straight on its desktop.
- Copy-paste works across all windows, including the hub.
- **No auto-reopen of windows after a hub restart.** The panel returns; windows do not.

### Canvas
- driftwm is **essential**. If it does not work on our system, we wait until it does. There is no fallback desktop.
- Writing our own compositor to replace driftwm is allowed if it is better and improves Hub OS, but only after a design discussion with the owner.
- Each machine has a **fixed home position** on the canvas, assigned in the inventory.
- Windows stay on the canvas. You can zoom in, and you can maximize a window (not fullscreen).
- Navigation is by mouse.

### Panel
- An always-visible bar item that opens a scrollable dropdown, grouped by role. Guests are nested under their host.
- The bar program is chosen once driftwm compatibility is known.
- When a machine is down, the **bar item itself shows an alert** (for example, "3 of 4 up" in red).
- A machine that is off just shows as down. No wake-on-LAN.
- If a machine drops while its window is open, leave the window alone. Only the alert changes.
- The bar shows which machine currently owns the keyboard and mouse (hub or gaming box).
- No alerts are sent to the owner's phone.

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
- The forwarder hides the chord from games, and sends a "release all keys" on every flip so no key stays stuck.
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
- **Competitive play** happens on the gaming monitors, with zero streaming delay.
- **Casual play** can happen in a Moonlight window on the hub, showing the same picker.

---

## NAS and backups

- **Full protection:** redundant disks, detection and repair of silent corruption, snapshots, ECC memory. Engineering effort and budget are not limits.
- **Filesystem:** ZFS or Btrfs, to be researched.
- **Workloads that need different tuning** (for example, AI datasets) get separately tuned areas on the same NAS. The AI box keeps its active dataset on its own fast SSD; the NAS holds the master copy.
- Opens on click in the hub's file manager. Non-Linux devices do not need access.
- **Backups:** a local backup NAS, plus a copy off-site or in another room. No encryption.

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
- A custom kernel
- TrueNAS, Proxmox
- A multi-scheduler layer (Slurm + Kubernetes + Proxmox)
- Auto-reopening windows after a hub restart
- Always-on live video for every node
- Wake-on-LAN
- Phone alerts
- Battery backup (UPS)

---

## Phases

1. **Phase A (now, phone only):** design documents, plus `hubd` with fake nodes. Finish line: the panel's tiles work for fake nodes (up/down, click opens the right program).
2. **Phase B:** the bot builds the Hub OS system and boots it in virtual machines in the cloud.
3. **Phase C (December 2026):** real hardware.

**Overall finish line (owner's words):** Hub OS boots the whole cluster. Every machine runs the Hub OS system, and the hub shows all of them as tiles that open their windows.

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

- How Hub OS jumps to, or places, a specific window in driftwm; maximize behavior
- Whether driftwm works with a status bar
- How driftwm behaves on a projector, and how readable small text is
- Whether driftwm and PipeWire run without systemd (critical: driftwm is essential and systemd is banned)
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

## Open questions for the owner

- GPU brands for each machine
- The full hardware list (research before December), including a keyboard and mouse that keep their polling rate through the forwarder
- Number of machines per role
- Soul Calibur II emulator; Minecraft Java or Bedrock
- ZFS or Btrfs
- A custom exit chord for game-style windows
- The bot's first task after this rewrite
- Security note: every machine stays logged in and can reach the internet, so incoming connections from the internet must stay blocked (except the future remote-access piece). Owner to confirm.

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
