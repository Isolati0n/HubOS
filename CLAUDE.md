# CLAUDE.md — rules for every session

Read `HUB-OS.md` (the project brief) in full before doing anything else. `HUB-OS.md` is the source of truth. If anything disagrees with it, `HUB-OS.md` wins.

If it is missing, stop and tell the owner. Do not guess its contents.

## What Hub OS is

- Hub OS is a custom Linux-based system for every machine in the cluster, plus a thin broker on the hub that opens each machine as a native window.
- It checks that other machines are reachable and opens, for each one, the correct native program: an existing viewer (Moonlight, virt-viewer/Remmina, a terminal) or one built for Hub OS (such as the planned file manager), as a normal window.
- The hub is only a client. It never renders workloads and never proxies or re-encodes video.

## Hard rules

- `hubd` is written in Go.
- Do not build: a new streaming protocol, or a web dashboard as the hub's control plane. A custom compositor (replacing driftwm) or a custom file manager may be built only after a design discussion with the owner.
- Do **not** use: Selkies, Apache Guacamole, NICE/Amazon DCV.
- Secrets never go in the inventory file and never go in git.
- Never rewrite, amend, or force-push existing commits without asking the owner first. Fix mistakes with a new commit.
- Never use systemd, under any circumstances. Init: start with an existing small init (candidates s6, dinit), kept swappable. Write our own only after a measured benefit and owner approval.

## How to work

- Do only the task the owner gives you. Nothing extra.
- Ask the owner before every choice (questions may be batched).
- Do not edit HUB-OS.md unless asked; record every change in its Change log.
- If something is unsolved or unverified, say so plainly. Do not invent architecture or protocols.
- Speak plainly. The owner is not a cluster operator.
- You cannot reach the owner's real machines. Use fake nodes and virtual machines only.
