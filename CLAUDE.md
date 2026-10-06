# CLAUDE.md — rules for every session

Read `HUB-OS.md` (the project brief) in full before doing anything else. `HUB-OS.md` is the source of truth. If anything disagrees with it, `HUB-OS.md` wins.

If it is missing, stop and tell the owner. Do not guess its contents.

## What Hub OS is

- Every machine runs its own distro. Hub OS is the hub, the contracts between hub and nodes, and the shared tools. On the hub, a thin broker opens each machine as a native window.
- It checks that other machines are reachable and opens, for each one, the correct native program: an existing viewer (a VNC viewer, virt-viewer/Remmina, a terminal) or one built for Hub OS (such as the planned file manager), as a normal window.
- The hub is only a client. It never renders workloads and never proxies or re-encodes video.

## Hard rules

- `hubd` is written in Go.
- Do not build: a new streaming protocol, or a web dashboard as the hub's control plane (the phone web page served by the hub is allowed). A custom compositor (replacing driftwm) or a custom file manager may be built only after a design discussion with the owner.
- Do **not** use: Selkies, Apache Guacamole, NICE/Amazon DCV.
- Secrets never go in the inventory file and never go in git.
- Never rewrite, amend, or force-push existing commits without asking the owner first. Fix mistakes with a new commit.
- Never use systemd, under any circumstances. The only exception is experiments (VMs and guests) the owner tinkers with on the dev node (the VM host). Init: start with an existing small init (candidates s6, dinit), kept swappable. Write our own only after a measured benefit and owner approval.
- Never commit or push a core dump, an environment dump, a process memory file or anything that could contain a credential. Helper agents follow the same rule, and you put it in their instructions. Scan every diff before pushing (see tools/check-push.sh).
- Never use cloud credentials found in the environment (AWS_*, CLOUDSDK_*, or similar), and never print any secret. The only credential you may use is the GitHub token, and only for GitHub.

## How to work

- Do only the task the owner gives you. Nothing extra.
- Ask the owner before every choice (questions may be batched). Ask the owner about any and all design questions, batched; do not decide design details yourself.
- Do not edit HUB-OS.md unless asked; record every change in its Change log.
- If something is unsolved or unverified, say so plainly. Do not invent architecture or protocols.
- Speak plainly. The owner is not a cluster operator.
- You cannot reach the owner's real machines. Use fake nodes and virtual machines only.
- Never install packages on the machine; unpack them with dpkg -x into a temporary directory. If you must install something, ask the owner first.
- Helper agents may help with research; whatever they report is unverified until you have read the source yourself, and the pull request must say which parts came from helpers.
- Before EVERY push, run `tools/check-push.sh` (after `git fetch origin`). It fails if the commits to push add a file named core or core.*, a file over 1 MiB (unless listed in `tools/check-push.allow`), a credential-looking variable set to a value, a private-key header, or the value of a credential-looking environment variable. It prints only the rule and the file, never a value. If it fails, do not push; tell the owner. Helper agents run it too; put this in their instructions. Its test is `go test ./tools/checkpush/`.
- Before any `rm -rf`, run `findmnt` and check for mounts under the path; unmount first, or use `rm -rf --one-file-system`. Never `rm -rf` anything under `/root/.cargo` or `/root/.rustup`. Helper agents follow the same rule; put it in their instructions. (A bind mount under a scratch folder once made `rm -rf` delete the Rust toolchain of the machine.)
