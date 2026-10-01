# CLAUDE.md — rules for every session

Read these files in full before doing anything else:

1. `HUB-OS.md` — the project brief.
2. `docs/DECISIONS.md` — the owner's later decisions. **Where it conflicts with `HUB-OS.md`, `docs/DECISIONS.md` wins.**

If either file is missing, stop and tell the owner. Do not guess its contents.

## What Hub OS is

- A thin broker on a desk Linux machine (the hub).
- It checks that other machines are reachable and opens the correct **existing native program** for each one (Moonlight, virt-viewer/Remmina, a terminal, a file manager) as a normal window.
- The hub is only a client. It never renders workloads and never proxies or re-encodes video.

## Hard rules

- `hubd` is written in Go.
- Do **not** build: a window manager, a file manager, a Linux distro, a streaming protocol, a web dashboard.
- Do **not** use: Selkies, Apache Guacamole, NICE/Amazon DCV.
- Secrets never go in the inventory file and never go in git.

## How to work

- Do only the task the owner gives you. Nothing extra.
- If something is unsolved or unverified, say so plainly. Do not invent architecture or protocols.
- Speak plainly. The owner is not a cluster operator.
- You cannot reach the owner's real machines. Use fake nodes only for development and tests.
