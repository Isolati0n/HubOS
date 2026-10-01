# Hub OS — Project Brief

This file is the source of truth for Hub OS. If a later chat invents a web portal, a new streaming codec, a custom file manager, or streaming of gaming PCs, that chat is wrong. Read this whole file before writing code or architecture.

**Date of this snapshot:** 2026-09-30  
**Owner:** single user, not a cluster operator yet. Daily Linux use is Mint / Xubuntu. Has never used Proxmox, Moonlight, or Sunshine. Owns no cluster hardware yet. Will have AI assistants write most of the code. Hardware purchase is *not* a prerequisite for starting the software; fake nodes and local VMs are enough.

---

## One-sentence definition

Hub OS is a thin broker on a desk Linux machine: it knows the other computers in a personal rack cluster, checks that they are reachable, and opens the correct **existing native program** so that machine appears as a normal window on an infinite-canvas desktop.

## What it is not

- Not a hypervisor
- Not a job scheduler
- Not a streaming codec
- Not a from-scratch Linux distribution (v1)
- Not a from-scratch file manager
- Not a from-scratch window manager
- Not a browser dashboard
- Not a remote-play path for the gaming PCs

## Core principle (never violate)

> The hub is a client to each node’s session server. It does not render workloads itself.

The hub must not re-encode video, must not proxy pixels unless a chosen tool already works that way, and must not become the machine that runs Blender, trains models, or stores the main file pool.

---

## The human setup (intent)

A rack-scale personal cluster. Budget and noise are not design limits. Non-gaming nodes are meant to stay powered on.

The user sits at the **hub** as the daily driver. **Gaming PCs** each have their own monitor and cable for tournament play. Everything else should be reachable from the hub as a window.

**driftwm** is the intended hub desktop: an experimental infinite-canvas Wayland compositor. The screen is a camera over a large desk. Hub OS places normal program windows on that canvas. Hub OS does not replace driftwm.

Long-term wish: a window slot for every non-gaming node, and tablet/other-device access to the hub.  
v1 does **not** keep every node as a live video stream 24/7 (that is a video wall and will fail). v1: a tile per node; live video only for nodes the user actually opened.

If the hub restarts, **do not auto-reopen viewers**. The user will click again.

No Home Assistant / house dashboards in v1.

---

## Planned cluster roles

| Role | Intent | On the hub? |
|---|---|---|
| Gaming (several) | One game per machine, own HDMI monitor | **No stream for play.** May show in inventory later as “exists.” |
| Hub | Daily driver desk PC | This machine runs Hub OS |
| AI / LLM / GPU | Training, inference, Blender, ParaView, CUDA | Yes — remote desktop |
| NAS | Cluster files | Yes — file app and/or terminal |
| Experiment / custom-OS | Many small systems at once | Yes — terminal or remote desktop |
| Observability | Logs and metrics for the whole rack | Later |
| Dev / staging | Build and test software | Later |
| Compile farm | Batch builds | Later |

### Wave-1 *roles* (design against these four)

Hardware SKUs below are examples, not a purchase requirement.

1. **Hub** — desk. Strong CPU, enough RAM, a modest GPU so a few remote screens decode cleanly, fast NIC.
2. **NAS** — TrueNAS-style file server, ECC RAM, HBA in IT/pass-through mode, redundant disks.
3. **AI** — high-VRAM GPU machine running Sunshine; viewed with Moonlight.
4. **Experiment** — one host that can run several guest systems (Proxmox is the default assumption). Guests are viewed with virt-viewer / Remmina or SSH.

A 10GbE-class network is assumed between them. Do not design as if they share a slow consumer Wi-Fi hop as the happy path.

---

## Protocol map (locked)

These tools are **out of scope** for the whole project unless the owner explicitly reverses this: **Selkies, Apache Guacamole, NICE/Amazon DCV.**

| Job | Node side | Hub side (normal window) |
|---|---|---|
| AI / GPU desktop | Sunshine | Moonlight |
| Guest / experiment desktop | SPICE or VNC on the guest | virt-viewer or Remmina |
| Commands | sshd | terminal (whatever the hub already has) |
| Files | NAS share (SMB or NFS) | normal file manager already on the hub |
| A product that is only a website | that website | one browser window — only when unavoidable |

Do not invent a new remoting protocol.

---

## v1 contract (definition of done)

A user sits at the hub, sees a small panel of tiles (hub + NAS + AI + one experiment guest), clicks a tile, and the correct native program opens as a real window on driftwm (or a boring Wayland/X11 desktop if driftwm is not installed). If the target is down, the tile says so and nothing fake opens. If the hub process restarts, tiles return; windows do not come back by themselves.

That is the finish line. Nothing else is v1.

---

## Architecture

```
[ user ]
    |
[ driftwm ]          existing compositor — layout only
[ hub-panel ]        small UI: tiles, status, Open
[ hubd ]             one background program (Go)
[ inventory file ]   list of nodes
[ secrets dir ]      mode 600 files — tokens, keys, passwords
    |
    +-- ping / SSH helper on nodes
    +-- Proxmox API for guests (experiment host)
    +-- exec: moonlight | virt-viewer | remmina | terminal | file manager
    |
[ NAS ]   [ AI + Sunshine ]   [ Experiment host + guests ]
```

### Components

| Piece | Responsibility | Notes |
|---|---|---|
| driftwm | Canvas, pan/zoom, window placement | Configure; do not fork unless it is truly unusable |
| hub-panel | The only Hub OS UI | Native. Not a website. Can be a tiny toolkit app or even a scripted menu at first |
| hubd | Inventory, health, launch, record of what was started | Go, standard library + small router if an API is needed |
| inventory | Name, role, address, how to open, optional guest id | YAML or similar, one file |
| secrets | Proxmox token, SSH key path, Sunshine/Moonlight pairing leftovers | `/etc/hubos/secrets.d/` or `~/.config/hubos/secrets.d/`, mode 600 |
| node helper | Tiny script: status / start-sunshine / stop-sunshine | Called over SSH. Not a second operating system |
| existing viewers | All pixels and input | Hub OS only starts them |

### Session rules

- hubd may remember “AI is supposed to be open” for the panel. It must not spawn a pile of duplicate viewers on a second click without asking.
- Ending a session from the panel should close the local viewer. It should stop Sunshine or a guest **only** if that session was started for this click and the inventory says that is safe. Never power off the whole AI machine just to close a window.
- Hub reboot: no auto-reopen of viewers.

### Health

“Node is up” is not “Proxmox says running.” Ready means:

- AI: Sunshine is accepting connections
- Guest: SSH or the SPICE/VNC port answers
- NAS: the share or SSH answers

### Window integration

v1 launches ordinary programs. Those programs *are* the windows. No webview embeds. No custom compositor protocol.

driftwm is preferred. Fallback: any working desktop on the hub so development can continue.

---

## Data the hub must keep

**Inventory (not secret)**  
Per node: `id`, `name`, `role` (`hub` \| `ai` \| `nas` \| `experiment-host` \| `guest` \| `gaming`), `kind` (`ssh-host` \| `proxmox-vm` \| `proxmox-lxc` \| `local`), `address`, `open` method (`moonlight` \| `spice` \| `vnc` \| `ssh` \| `files` \| `none`), optional `vmid`, optional `share`.

Gaming entries: `open: none`.

**Secrets (secret)**  
Never in the inventory file. Never in git.

**Runtime state (can be SQLite or a small file)**  
Last health, last error, whether a viewer was launched and its local process id if known.

---

## Lifecycle (AI tile)

1. User clicks Open on AI.  
2. hubd reads inventory + secrets.  
3. Health check. Fail → show error.  
4. If Sunshine is down and the helper can start it, start it and wait until it listens. Timeout → fail.  
5. Start Moonlight pointed at that host.  
6. User works in that window.  
7. User clicks End → close Moonlight. Do not default to shutting down the AI box.  
8. Hub crash → user clicks Open again.

Guest tile: same, but step 4 may be “ensure Proxmox guest is running,” and step 5 is virt-viewer/Remmina or SSH.

NAS tile: skip remoting; open files or SSH.

---

## Explicit cuts (do not build)

- Selkies, Guacamole, DCV
- Browser-based control plane as the product
- Custom file manager
- Custom window manager (beyond config for driftwm)
- Custom Linux ISO as v1
- Multi-scheduler abstraction (Slurm + Kubernetes + Proxmox)
- Multi-user / fancy access control
- Tablet remote access (after desk v1 works)
- Observability stack, compile farm, twelve physical OS minis
- Always-on live video for every node
- Auto-restore of viewer windows after hub reboot
- Streaming gaming PCs for play

---

## Ground truth already established

- Open OnDemand already does “submit, wait, open a stream” for HPC. That is not the gap. The gap is a **personal desk broker** across mixed machines with **native windows**, not another web portal.
- Moonlight/Sunshine: native window, GPU encode on the node. Desktop inside a Sunshine session dies if that session is destroyed; keep the node process up while the user cares about that desktop.
- virt-viewer / Remmina / SSH: boring and sufficient for guests and the NAS.
- driftwm exists and is experimental (Wayland, infinite canvas, AI-assisted codebase). Treat it as a dependency that can break.

---

## How to work

- Prefer concrete files and commands over diagrams.
- If something is unsolved in existing tools, say so. Do not invent a protocol.
- Go for hubd unless there is a hard reason not to.
- First runnable slice: inventory + up/down + click opens a terminal to a fake node. Then viewers. Then Proxmox. Then driftwm layout polish.
- The owner is new to clusters. Explanations in the product UI must stay plain.

---

## Suggested repo layout (when code starts)

```
hubos/
  cmd/hubd/
  cmd/hub-panel/     # or a later folder if panel starts as a script
  internal/
  configs/inventory.example.yaml
  scripts/node-helper.sh
  docs/
```

No monorepo for a custom distro, compositor, and file manager.

---

*End of brief. When in doubt, do less, keep the hub a client, open a real window.*
