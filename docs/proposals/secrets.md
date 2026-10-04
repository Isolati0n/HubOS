# PROPOSAL: secrets and keys

**Status: PROPOSAL. Nothing here is decided and nothing here is in the images.** `HUB-OS.md` wins if this file disagrees with it. Written 2026-10-04 by the owner's lead (not by a helper agent), using the repository's own documents and the sources named below. This is the secrets design the project has owed since the start (`HUB-OS.md`: "secrets handling is a separate step").

**Labels on every item:**
- **TESTED**: run in this build environment; the command and output are given (or it is a test in `docs/image.md`).
- **SOURCE**: read in a primary source; link and date read (2026-10-04). Pages were read as raw page text with `curl`, not through a summarising tool.
- **BELIEVED**: I think it is true; not tested, no primary source read.
- **UNKNOWN**: nobody has checked; do not rely on it.
- **DESIGN**: a proposal made here; it is a choice for the owner, not a fact.

---

## Plain-words summary

1. **There are only a few secrets that matter, and one that matters far more than the rest.** The *update signing key* decides what every machine will run. If someone steals it they can install their own software on every machine. Everything else (display passwords, the hub's management key, pairing material) lets an intruder on the home network do damage to one machine or to the hub's control of it, but not rewrite the whole cluster.
2. **Proposal for the update key (DESIGN):** keep its private half on a machine or device that is *never connected to a network*, protected by a passphrase. The hub only ever sees the *public* half. Until the hardware exists (December) only throwaway test keys are used; do not generate the real key on the phone.
3. **Each node gets its own display password** (already decided in `HUB-OS.md`). The hub keeps a copy of every node's password so it can open the window; the node keeps its own. These live on the config partition of each machine, never in git, never in the inventory, never on a command line.
4. **The hub has one "management key"** it uses to sign requests to nodes (clipboard, session start/stop, recovery install). Its private half lives on the hub's config partition; nodes hold only the public half. Honest weak point: the desktop programs and hubd run as the same user (`hub`), so a compromised viewer could read these files. Section 6 lists this as a question.
5. **The config partition is not encrypted.** Anyone who can take the disk, or reach the recovery shell (which has no login), can read the secrets on it. The design relies on the rack being next to the desk (physical security) and on the isolated network, not on disk encryption. That is a deliberate, stated limit.
6. **What is backed up:** the inventory, `viewers.toml` and `wofi.css` go to the NAS backup. **Secrets do not.** Display passwords can be made again; the update private key cannot, so it needs its own offline backup (section 5).
7. **Not decided and needing you:** where the offline signing device comes from, whether to add a hardware token later, whether to split the `hub` user, and who may rotate what. Section 7.

---

## 1. Rules that apply to every secret (from `CLAUDE.md` and `HUB-OS.md`, SOURCE: this repository)

- Secrets never go in git and never in the inventory file. The inventory and `viewers.toml` are *not* secret (`docs/inventory-format.md`).
- Per-machine settings, "inventory, secrets", live on the config partition, outside the image (`HUB-OS.md`, Hub OS system section).
- Images contain only *public* keys (`/etc/hubos/keys/*.pub`, section 3.12 of `docs/image.md`).
- The recovery kernel and the boot partition are open (FAT, readable by anyone with the disk). **No secret is ever stored there** (`docs/proposals/recovery-and-out-of-band.md`).
- Proposed additional rules (DESIGN): one key per purpose, never reused; a secret is created where it is used or on the offline device, never on a node's behalf by the bot; no secret in a command line (section 4); a leak has a written cost (section 2) and a written way out.

## 2. Every secret and key

Columns: who creates it; where it lives at rest; who may read it; how it is handed to a program; how it rotates; what a leak costs. "Config partition" is the ext4 partition `hubos-config` of that machine (not encrypted).

| # | Secret | Creates | At rest | Who may read | Handed to a program | Rotation | What a leak costs |
|---|---|---|---|---|---|---|---|
| 1 | **Update signing key, private half** (signify Ed25519) | the owner, once, on the offline device (DESIGN) | the offline device only; an encrypted backup copy elsewhere (section 5); **never** on the hub, a node or a network machine | the owner (passphrase) | typed passphrase at the `signify -S` prompt; no automation | two-release rotation with a second key, procedure in `docs/image.md` 3.12 (TESTED, tests K1 and K2) | **Total.** Whoever has it can sign a release every machine accepts. The only way out is a release signed with the *old* key that carries a new key, so a thief with the key can race the owner. See section 5 |
| 2 | Update key, **public** half(s) | derived from #1 | in every root and in the recovery kernel (`/etc/hubos/keys/*.pub`) | everyone (not secret) | file | with #1 | none |
| 3 | **Management key, private half** (hub's signing key for requests to nodes and to the recovery agent) | the hub's setup step, on the hub (DESIGN); a different key from #1 | hub config partition, a root-owned folder, mode 0600 (DESIGN) | root, and hubd | hubd reads the file at start; never a command line | the node side accepts every `*.pub` in `/etc/hubos/mgmt`, like the update keyring, so a new key is shipped in a release first, then switched, then the old public key is dropped (DESIGN; same procedure as #1) | An attacker who has it and can reach the network can: install a *signed* bundle through the recovery agent (still needs the update key's signature, so no unsigned code), clear the failure counter, start and stop sessions, push clipboard text, read logs. **Cannot** make a machine run unsigned software |
| 4 | **Management key, public half** | derived from #3 | in the images of nodes (`/etc/hubos/mgmt`) | everyone | file | with #3 | none |
| 5 | **Display credentials per node** (wayvnc user name and password; `HUB-OS.md`: per node) | a small secrets tool run on the hub when a node is provisioned (DESIGN); a long random string | the node's config partition (for wayvnc) **and** a copy on the hub's config partition (so the hub can open the window); never in the inventory | root and the wayvnc process on the node; root and hubd on the hub | wayvnc: a config file in a root-only folder (round 2: the smallest login is `enable_auth`, `username`, `password` in wayvnc's config, SOURCE: `docs/proposals/remote-display.md` round 2, helper-written, unverified). Viewer: environment variable, or a mode-0600 connection file in a RAM folder (TigerVNC reads `VNC_USERNAME`/`VNC_PASSWORD`, `remote-viewer` a `.vv` file; helper-reported, not re-checked by the lead) | generate a new one, write it to the node (signed node-helper call, or by hand) and to the hub, restart wayvnc; open sessions drop | Whoever has it and can reach the node's port sees and controls that node's whole desktop (every node stays logged in). One node, not the cluster |
| 6 | **Display TLS key and certificate per node** (only if TLS is chosen over RSA-AES; round 2 says TLS with a self-signed certificate works) | the secrets tool, with the node's address in the certificate | node config partition (key, mode 0600); the certificate (public) also on the hub | root + wayvnc | file paths in wayvnc's config | new pair, same steps as #5 | the same as #5 (impersonating the node to the hub) |
| 7 | **Node helper and recovery agent**: private keys | none exist: nodes only *verify* with the public management key (#4); the one-time nonces live in RAM | (nothing at rest) | - | - | - | nothing to leak on the node. The recovery agent *cannot* hold a secret anyway (open boot partition) |
| 8 | **Sunshine** (gaming box): web-UI user name and password, `sunshine_state.json`, its certificate and private key, the list of paired clients | the owner, by hand, at first start (`docs/viewers-research.md`, SOURCE: Sunshine `getting_started.md`) | the gaming box's Sunshine config folder (`~/.config/sunshine` by default) | the user running Sunshine | Sunshine reads its own files | change the web-UI password in the web UI; unpair and re-pair clients by hand | Stream or take over the gaming session; web-UI admin. One machine |
| 9 | **Moonlight** client certificate and private key (hub) | created by Moonlight at first pairing | the hub user's Moonlight settings file (`docs/viewers-research.md`) | the hub user (`hub`) | Moonlight reads its own settings | re-pair | An attacker with it can pair-impersonate the hub to the gaming box |
| 10 | **Pairing PIN** | shown by the client, typed once | nowhere (transient) | the owner | typed on the host's web UI | one-time | none after pairing |
| 11 | **NAS / ZFS keys** | **none planned**: `HUB-OS.md` says backups have no encryption and OpenZFS native encryption is not in the plan | - | - | - | - | If encryption is added later it needs its own section here: a loss of its key loses the data |
| 12 | **Backup copy access** (NAS to backup NAS, and the off-site copy) | the owner, when the backup job is designed (not designed yet) | the credential for the job: a dedicated key on the machine that *starts* the copy | root on that machine | file | new key, old one removed on the receiving side | If the sending side's credential allows writing or deleting on the backup, a leak lets an intruder destroy the backups too. DESIGN: the backup NAS *pulls*, and a copy off-site is append-only or kept offline (BELIEVED to be the safer pattern; not designed) |
| 13 | **BMC and out-of-band device credentials** | **none planned** ("no proprietary BMC firmware"; out-of-band hardware is decided in December) | if a device is chosen later: the hub's config partition, in the same folder as #3 | root, hubd | same as #5 | change the vendor default at once (the PiKVM documents default logins, per `docs/proposals/recovery-and-out-of-band.md`, helper-written, not re-read by the lead) | full console control of the machine it is wired to |
| 14 | **SSH keys**, if any remain | the hub, one key pair per purpose (DESIGN) | hub config partition, mode 0600; public halves in the images of the nodes that accept them | root, hubd | `ssh -i FILE` is a path, not a secret on the command line | new pair, public key shipped in a release | login as the account the key maps to. `HUB-OS.md` now has no terminal windows, so ssh is only for health checks that need no login (a TCP connect) and maybe the backup job; if neither needs it, there are no SSH keys at all |
| 15 | **Game accounts** (Battle.net for Diablo II, Steam) | the owner, typed into those programs on the gaming box | the programs' own folders on the gaming box's data area | the gaming user | the programs' own login | the owner changes the password at the service | account theft. Not managed by Hub OS; the NAS save copy must not include these folders (DESIGN) |
| 16 | **Remote access** (the future locked-down piece for non-gaming nodes) | not designed | - | - | - | - | If built it needs its own login and encryption (`HUB-OS.md`); add a row then |
| 17 | **Developer-time tokens** (the GitHub token the bot uses; any cloud credentials found in the environment) | not part of the cluster | outside the repo and outside the images | the bot session | the harness | by the owner at GitHub | `CLAUDE.md`: only the GitHub token may be used, only for GitHub; cloud credentials are never used; no secret is printed |
| 18 | **Local accounts and passwords on machines** | none: finished images have no `login`, `passwd` or PAM modules (`docs/image.md`) | - | - | - | - | The recovery shell has **no login** (`docs/image.md` 3.7). Physical or serial access to a machine is full access to it |

**Not secret, so not in this table:** the inventory, `viewers.toml`, `wofi.css`, the public keys, the machine ids and addresses, the release manifests (they are signed, not secret).

## 3. Where things live (DESIGN)

**On the hub, config partition** (`/config/hubos/`): `inventory.toml`, `viewers.toml` (backed up, not secret) and a folder `secrets/` with mode 0700, owner root:
- `secrets/mgmt/management.sec` (#3), mode 0600
- `secrets/display/<machine id>` (#5, #6), one small file per node, mode 0600
- `secrets/ssh/` (#14), if it exists at all

**On a node, config partition:** `secrets/display` (#5, #6), mode 0600, root only. Nothing else. The node's images carry only public keys.

**On the offline device:** the update key's private half (#1) in a signify secret-key file protected by a passphrase.

**Never:** git, the inventory, the boot (EFI) partition, the recovery kernel, a command line, a log file, the NAS backup, a phone.

**What a node's config partition tells an attacker:** its own display credential. It does *not* tell them the others' credentials, the update private key or the management private key.

## 4. How a secret is handed to a program (without a command line)

- **A command line is visible to every user on the machine** through `ps` and `/proc/PID/cmdline` (BELIEVED, standard Linux behaviour; not re-read for this document). So no secret is passed as an argument.
- **The environment is not private from the same user or from root.** `/proc/PID/environ` is readable subject to a ptrace access check (SOURCE: [proc_pid_environ(5)](https://man7.org/linux/man-pages/man5/proc_pid_environ.5.html), read 2026-10-04: "Permission to access this file is governed by a ptrace access mode PTRACE_MODE_READ_FSCREDS check"). An environment variable is better than an argument, but it is only as private as the process's user.
- **Files with mode 0600 in a root-only folder** are the default (#3, #5, #6, #14). A *service* can turn files into environment variables at start with `s6-envdir` (SOURCE: [s6-envdir](https://skarnet.org/software/s6/s6-envdir.html), read 2026-10-04: it reads a folder and adds one variable per file, the file's content as the value). That keeps the secret out of the service's command line and out of the image.
- **RAM folders** (`/run/...`, tmpfs) for copies that a program needs only while it runs (for example a `.vv` connection file), removed when the window closes (DESIGN).
- **A passphrase prompt** for the update key (#1): `signify -S` asks for it. `signify -G` asks for a passphrase when it makes the key pair unless `-n` is given (SOURCE: [signify(1)](https://man.openbsd.org/signify), read 2026-10-04). The test suite uses `-n` for throwaway keys only (`tools/image/qemu_test.go`).

## 5. The offline update key

**The problem.** One owner, only a phone today, a desk and a rack in December. The key must exist somewhere safe, be usable at release time, and survive a lost phone, a dead laptop or a fire. It must never be on a machine that is on the cluster's network.

**Options (each label says how much is known):**

| Option | What it is | Good | Bad / unknown |
|---|---|---|---|
| A. **An encrypted signify key on a never-networked machine** (an old laptop, or a small board with no network cable) | `signify -G` on that machine with a passphrase; sign by carrying the manifest on a USB stick, signing, carrying the `.sig` back | works today with the tool the images already use (TESTED: `signify-openbsd` is what `hubos-ctl` verifies with, tests K1/K2); no new code; cheap | the passphrase is the only protection of a stolen disk; USB sticks are a path for malware (keep the stick for manifests only, SOURCE: none read, BELIEVED standard advice) |
| B. **A hardware token** (for example a YubiKey) | the private key never leaves the token; each signature needs a touch and a PIN | a stolen laptop is not a stolen key | `signify` itself cannot use a token (signify's secret key is a file; SOURCE: signify(1)). YubiKey PIV supports Ed25519 on firmware 5.7 and up (SOURCE: [Yubico PIV introduction](https://developers.yubico.com/PIV/Introduction/YubiKey_and_PIV.html), read 2026-10-04). The signature format is a plain Ed25519 signature wrapped with a key number (see `tools/image/experiments/recoveryagent/signify.go`), so a small program could wrap a token's signature into a signify file. **UNKNOWN:** whether a token's Ed25519 signature is the same (pure Ed25519, RFC 8032) signature signify expects; nothing was tested (no hardware here); the program does not exist |
| C. **Phone only** | generate and keep the key on the phone | nothing to buy | the phone is a networked, app-running device; **UNKNOWN** whether signify can run there (not examined). Not recommended for the real key |

**Proposal (DESIGN):** use option A in December (cheap, uses the existing tool, no new code), with a second encrypted copy of the secret-key file on a USB stick kept in another room, and the passphrase kept separately from both. Consider option B later as an upgrade; it needs a test with real hardware and a small wrapper, and it should not be started before the owner has the token. **Until December only throwaway test keys exist**, as in the test suite. The real key must be generated on the offline device, never on the phone and never by the bot.

**Release step (DESIGN):** the build machine inside the cluster builds a bundle and its unsigned manifest; the manifest travels by USB stick to the offline device; the owner signs it with the passphrase; the signed manifest travels back; the hub publishes it. The signing key and the build machine never meet.

**If the key is lost:** no new release can be accepted by machines that only know that key. Machines keep running their current release. A new key means a first install again (network boot, then the new key is in the image); the recovery kernel also carries the keyring (`docs/image.md` 3.12). So a lost key costs a re-install of every machine, not their data (the data partition survives a re-flash only if the installer is built to keep it: UNKNOWN, not tested).

**If the key is stolen:** the thief can sign releases that every machine accepts until the owner ships a release, signed with the old key, that drops it (the rotation in `docs/image.md` 3.12). If the thief also uses the key, it is a race. The floor (`min_version`, TESTED in `docs/image.md` 3.6) stops a *rollback* but not a new bad release with a high number. This is why the key must be offline and why the rotation procedure should be rehearsed once a year (DESIGN).

**What TUF-style rules would add** (SOURCE: [The Update Framework specification](https://theupdateframework.github.io/specification/latest/), read 2026-10-04):
- **Roles with several keys and a threshold.** The spec says the framework must support roles with multiple keys and threshold (quorum) trust, so that compromise of fewer keys than the threshold does not compromise clients, and that the root role's private keys must be kept offline. For one owner this would mean, for example, 2 signatures out of 3 keys (one on a token, one on the offline laptop, one in a safe). **Cost:** `hubos-ctl` and the recovery kernel verify one signature today; they would have to verify several (code change, more tests), and the owner needs three devices. **Benefit:** one stolen or lost key is not enough. **DESIGN:** not now; revisit after the first year if the single key feels too risky.
- **Expiry.** TUF metadata carries an `expires` time and clients check it against the current time (the spec fixes the update start time at the start of the workflow, section 5.1). **Our machines have no trusted clock** (the recovery agent design uses one-time nonces for that reason). So expiry would need a clock source we do not have; the existing `min_version` floor is the monotonic substitute. A freeze attack (someone keeps serving an old release) is not a threat while the owner's own hub is the only update source. **DESIGN:** no expiry.
- **Separate roles** (root, targets, snapshot, timestamp): not needed for one owner and one repository; the signed manifest already lists the hashes of everything.

**`NOTES-update-security.md`:** the task named this file as possible background. I searched the repository (including history) and the machine's filesystem and did not find it, so **I could not read it**. If it holds decisions that disagree with this section, it should win once it is in the repository.

## 6. Back up, and what does not

| Item | Backed up? | Where | If lost |
|---|---|---|---|
| Inventory, `viewers.toml`, `wofi.css` | **yes**, NAS backup copy (`HUB-OS.md`) | the NAS and the backup NAS | restored from the backup |
| Mixer settings (decided: hub config partition, included in the NAS backup) | **yes** | same | restored |
| Display credentials (#5, #6) | **no** | - | generate new ones and write them to node and hub (a re-provisioning step) |
| Management key (#3) | **no** | - | generate a new one on the hub; ship its public half in a release (or re-provision); old nodes accept the old key until then |
| Update private key (#1) | **its own offline backup only** (section 5), never the NAS backup | the offline device and one encrypted copy elsewhere | see "If the key is lost" |
| Sunshine / Moonlight pairing (#8, #9) | **no** | - | re-pair by hand |
| Game saves | yes, copied to the NAS by the gaming box (`HUB-OS.md`) | NAS | restored |
| Game account credentials (#15) | **no** | - | the owner logs in again |

A rule that keeps this simple (DESIGN): the NAS backup job copies an explicit *list* of files (the three above and the saves), not whole folders, so a secret cannot slip into a backup by living in a backed-up folder.

## 7. What is still open (questions for the owner)

1. **The offline device.** Which machine or board becomes the never-networked signing machine in December, and where is the second encrypted copy kept? (Section 5, option A.)
2. **Hardware token.** Do you want a YubiKey-class token considered later (option B)? It means buying hardware before December to test the signature wrapper.
3. **Threshold signing.** Is "one offline key plus a rehearsed rotation" enough, or do you want 2-of-3 signing from the start (it changes `hubos-ctl` and the recovery kernel)?
4. **The `hub` user.** hubd and the desktop programs (driftwm, Waybar, viewers) run as the same user `hub` (`HUB-OS.md`: "services run as the normal user hub"). A compromised viewer could then read the management key and every display credential. Options: a second user for hubd's secret-holding part, or a small root-owned helper that signs requests for hubd. Which level of separation do you want? (Not tested; the current plan is the weak one.)
5. **Config partition encryption.** The config partition is plain. Accept physical access as full access (the rack is next to the desk), or look at encrypting it (needs a place for the key; TPM use is UNKNOWN)?
6. **Display password handling.** Which viewer is used decides how the password reaches it (environment variable, connection file, none). Until that is decided the secrets tool cannot be written. Also: one password per node (decided) but should the hub's copy be a separate file per node (proposed) or one file?
7. **Who rotates.** Is rotation always the owner's explicit action (proposed), or may the hub rotate display credentials by itself on a schedule through the signed node-helper API?
8. **Backup job credential (#12).** Pull from the backup NAS, push from the NAS, or something else? Not designed yet. And the off-site copy: online (credential needed) or offline (disk carried)?
9. **Game accounts.** Confirm that the gaming box's save copy to the NAS must exclude the account folders, and that Hub OS does not manage those logins.
10. **A secrets tool.** May a later task write a small Go tool that makes display credentials and TLS certificates on the hub and writes them to the right files? It would be the only code that ever sees a secret in clear.
