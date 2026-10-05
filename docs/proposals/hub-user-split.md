# PROPOSAL: splitting the hub into a desktop user and a hubd user

**Status: PROPOSAL. Nothing here is built, nothing is decided, and nothing is in the images.** `HUB-OS.md` wins if this file disagrees with it. Written 2026-10-05 by the owner's lead (not by a helper agent) from the repository and the primary sources named below. It answers `docs/proposals/secrets.md` question 4 and the `HUB-OS.md` line "The hub user is split: hubd ... and a desktop user (viewers), with a launcher handing credentials to viewers".

**Labels:** **TESTED** (a command and its output, or a repository test), **SOURCE** (a document or file, with link or path, read 2026-10-05), **BELIEVED**, **UNKNOWN**, **DESIGN** (a proposal made here, for the owner to decide).

---

## Plain-words summary

1. **Today one user, `hub`, runs everything on the hub**: the compositor (driftwm), the bar (Waybar), the menu, the viewers and hubd. Any of those programs can read any file the others can read. The files that will matter are the *management key* (it signs requests to every node) and the *display passwords* of every node (`docs/proposals/secrets.md`). So a flaw in one viewer could today hand an attacker the keys to the whole cluster.
2. **Proposal (DESIGN): two users.** `hubd` (new) holds the secrets and does the network work. `hub` (the existing desktop user) keeps the screen: driftwm, Waybar, the viewers. A small program running as `hub`, the **launcher** (I call it `hubd desk`), starts viewers when `hubd` asks and hands each viewer only the one password it needs.
3. **What this does protect:** the management key never exists in the desktop user's reach, and the display passwords of machines whose windows are *not* open stay out of reach of a hijacked viewer, Waybar or terminal.
4. **What it does not protect (said plainly):** the password of a machine whose window is *open* has to be inside that viewer, and a hijacked program running as the same user `hub` can probably read another `hub` process's memory or environment (the kernel configuration has no Yama ptrace restriction, BELIEVED). So a hijacked viewer can still leak *its own* node's password and the other open viewers' passwords. The split shrinks the damage from "all keys" to "the currently open windows".
5. **The hard part is driftwm's socket**: it accepts only its own user (mode 0600, SOURCE `docs/driftwm-findings.md`). hubd uses it to place windows. A `hubd` user cannot, so window placement and viewer starting move to the launcher; hubd only *asks*. The code already has the seams for this (`Launcher` and `Compositor` in `internal/hub`, SOURCE: `hub.go`).
6. **Credentials reach a viewer in the way that viewer supports** (section 5): `remote-viewer`: a mode-0600 connection file that deletes itself (`delete-this-file`, SOURCE); TigerVNC: environment variables `VNC_USERNAME` and `VNC_PASSWORD` (SOURCE); other viewers: UNKNOWN until the benchmark document (`docs/proposals/remote-display-benchmarks.md`) has read them. Never a command line.
7. **What it costs:** a new user, a new program (the launcher, in the same Go module), a new s6 service, about a dozen file-mode rules, and changes to the hub QEMU tests H0 to H5 (section 8). Not built; the estimate of work is in section 9.

---

## 1. What exists today (SOURCE: this repository, read 2026-10-05)

- **One user.** `hub` (uid 1000, group `seat` 1001), home `/run/hubos`, shell `/bin/false` (`tools/image/build-root-image.sh` lines 38 and 57-58). The s6 services `dbus`, `driftwm`, `waybar` and `hubd` all run as `hub` through `s6-setuidgid hub env $HUBENV ...` (`image/machines/hub/rootfs/etc/s6/sv/*/run`). `seatd` and `udevd` run as root (decided in `HUB-OS.md`).
- **The environment** is built by `hub-env.sh`: `XDG_RUNTIME_DIR=/run/dw` (mode 0700, owner `hub`), `/run/hub/state`, `/run/hubos` (0700, owner `hub`).
- **hubd is one process** (`hubd serve`), which does everything: reads the inventory and `viewers.toml` from `/config/hubos`, probes the machines, keeps the panel state, starts viewers itself (`NewExecLauncher` in `internal/hub/viewerlog.go` runs the viewer with `exec.Command`, in its own process group, output to a log file), finds and places their windows by talking to driftwm's socket, and serves a socket `/run/hubos/hubd.sock` that the bar, the menu and the confirm step use (`hubd feed`, `list`, `menu`, `pick`, `open`, `end`, `forget`).
- **Seams in the code.** `internal/hub/hub.go` defines `type Launcher func(machineID string, args []string) (*Proc, error)` and `type Compositor interface { State; Move; Resize; Focus; Close; Identity; Subscribe }` ("*driftwm.Client satisfies it; tests use a fake"). hubd's own logic only uses these two interfaces.
- **driftwm's socket** is `$XDG_RUNTIME_DIR/driftwm/ipc-<display>.sock`, mode `0600`; "it is a full control surface" (it can run programs; SOURCE `docs/driftwm-findings.md` lines 216 and 230; "not tested as another user").
- **The confirm step** (root) asks hubd on `/run/hubos/hubd.sock` five times before it confirms a boot (`image/rootfs/etc/s6/sv/confirm/run`).
- **The hub QEMU test** runs commands as the desktop user: `asHub()` in `tools/image/hub_test.go` is `s6-setuidgid hub env ...`.
- **The kernel configuration** of the hub (`image/kernel/*.frag`) has no `SECURITY`, `YAMA` or `PTRACE` option. BELIEVED consequence: a process may `ptrace` another process of the same user, and read its `/proc/PID/environ`. Not tested.

## 2. What the split must protect, and from whom

| Asset | Today (one user) | After the split (DESIGN) |
|---|---|---|
| Management private key | readable by every `hub` program | readable only by user `hubd` (and root) |
| Display credentials of all nodes | readable by every `hub` program | readable only by user `hubd` (and root); a launched viewer gets only its own node's |
| The inventory and `viewers.toml` | readable (not secret) | unchanged, world-readable |
| The update signing key | not on the hub (offline) | unchanged |
| Control of driftwm | any `hub` program | the desktop user only (unchanged, by driftwm's design) |

**From whom:** a hijacked viewer (a bug in `remote-viewer`, TigerVNC, a decoder), a hijacked Waybar or wofi, a hijacked terminal. Not from root, and not from someone who takes the disk (the config partition is not encrypted, `secrets.md` question 5).

## 3. The proposal in one picture (DESIGN)

```
 owner / bar / menu  --(hubd.sock: open, end, list, feed, pick)-->  hubd   (user hubd)
                                                                       |  holds: management key, display credentials,
                                                                       |  inventory, panel state, node API calls
                                                                       |
                                       desk.sock (JSON lines, peer-checked)
                                                                       v
                                                    hubd desk  (user hub, group seat)  the launcher
                                                      starts viewers, holds the driftwm client,
                                                      runs wl-paste / wl-copy for the clipboard bridge
                                                                       |
                                      driftwm socket (0600, user hub)  +  Wayland  +  the viewers (user hub)
```

- **`hubd` (core), user `hubd`.** Everything that needs the network or a secret: probes, the node helper and recovery agent calls (signed with the management key), the display credential store, the panel state, `hubd.sock` for the bar and menu.
- **`hubd desk` (launcher), user `hub`.** A second mode of the same binary. It owns everything that needs the Wayland session or driftwm's socket: starting and ending viewers, finding and placing their windows, reading and setting the hub clipboard for the clipboard bridge, running wofi for the menu. It listens on `desk.sock`.
- **Who calls whom.** hubd's logic uses `Launcher` and `Compositor`. In the core those become *remote* implementations that send one JSON line to `desk.sock` and read the answer (a stream for `Subscribe`). In the launcher they are the existing `NewExecLauncher` and `driftwm.Client`. The window-matching rules stay in hubd's logic (unchanged); only the two edges move. (DESIGN; the amount of change is an estimate, section 9.)

## 4. Files, owners and modes (DESIGN)

New users and groups (next to `hub` and `seat`): user `hubd` (uid 1002, home `/var/empty` or `/run/hubd`, shell `/bin/false`), group `hubd`, and a shared group `hubctl` with members `hub` and `hubd` (only for the two sockets).

| Path | Owner : group | Mode | Contents / who needs it |
|---|---|---|---|
| `/config/hubos/secrets/` | `hubd:hubd` | 0700 | folder for secrets (hubd only; root can read) |
| `/config/hubos/secrets/mgmt/management.sec` | `hubd:hubd` | 0600 | management private key |
| `/config/hubos/secrets/display/<machine id>` | `hubd:hubd` | 0600 | one display credential file per node (user name, password; for TLS also the key) |
| `/config/hubos/inventory.toml`, `viewers.toml`, `desktop.env` | `root:root` | 0644 | not secret; hubd and the desktop read them |
| `/run/hubos/` | `hubd:hubctl` | 0750 | holds `hubd.sock` |
| `/run/hubos/hubd.sock` | `hubd:hubctl` | 0660 | the bar, the menu and the confirm step connect; hubd checks the peer's user id with `SO_PEERCRED` (it allows `hub` and root only) |
| `/run/deskd/` | `hub:hubctl` | 0750 | holds `desk.sock` |
| `/run/deskd/desk.sock` | `hub:hubctl` | 0660 | only user `hubd` is accepted (peer check) |
| `/run/dw/` (XDG_RUNTIME_DIR, driftwm and the Wayland socket) | `hub:hub` | 0700 | unchanged |
| `/run/dw/launch/` | `hub:hub` | 0700 | per-launch files for viewers, removed after the start (section 5) |
| viewer logs | `hub:hub` | 0700 | the launcher writes them (today hubd does) |

`SO_PEERCRED` on a Unix socket gives the connecting process's user id (BELIEVED standard Linux behaviour, `unix(7)`; not read for this document). A password is never written into `/config` by the desktop user; only the core and the secrets tool write there.

**`/config` is one partition** (`ext4`, mounted at `/config`). Mode bits on it are real file modes; there is no per-folder mount trick needed.

## 5. How a credential reaches a viewer (without a command line)

The core sends the launcher `{"op":"start","machine":"ai-1","argv":[...],"cred":{...}}` over `desk.sock` (the credential is in memory in both processes and goes over the local socket only). The launcher then gives the viewer its credential in the way *that viewer* supports. Compare:

| Method | How it works | Who can see it | Fits |
|---|---|---|---|
| **A. Command line** | `viewer --password=...` | every user, through `ps` and `/proc/PID/cmdline` (BELIEVED, standard) | **ruled out** |
| **B. Environment variable** | the launcher starts the viewer with the variable set | the same user and root, through `/proc/PID/environ`, subject to the kernel's ptrace access check (SOURCE: [proc_pid_environ(5)](https://man7.org/linux/man-pages/man5/proc_pid_environ.5.html), read 2026-10-04). Not other users | **TigerVNC**: "You can also add 'VNC_USERNAME' and 'VNC_PASSWORD' to environment variables" (SOURCE: [vncviewer man page](https://tigervnc.org/doc/vncviewer.html), read 2026-10-05) |
| **C. A mode-0600 file in a runtime folder, deleted after the start** | the launcher writes `/run/dw/launch/<id>.vv` (0600), starts the viewer with the path, and removes it | the same user and root, only while the file exists | **remote-viewer**: the connection file has `username` and `password` keys, and `delete-this-file` ("Set to 1 for the client to remove this connection file", SOURCE: [remote-viewer.pod, virt-viewer v11.0](https://gitlab.com/virt-viewer/virt-viewer/-/raw/v11.0/man/remote-viewer.pod), read 2026-10-05). UNKNOWN: whether it removes the file *before* it asks for a password; test it. Also the round-2 helper's result: a `.vv` file worked for TLS (`docs/proposals/remote-display.md`, helper-written) |
| **D. A file the viewer reads as its password file** | `-PasswordFile` | file readers | **TigerVNC** has `-PasswordFile` (SOURCE: same man page) for the *VncAuth* password file in the server's obfuscated format; wayvnc's RSA-AES login (user name and password) is not that format, so B is the option for the planned wayvnc login. UNKNOWN: whether TigerVNC's `Plain`/RSA-AES schemes read the file |
| **E. Standard input or an inherited file descriptor** | the launcher pipes the secret to the viewer | only the viewer (and the same user through ptrace) | FreeRDP has an option to read credentials from standard input (BELIEVED from memory; not read, RDP is out of scope for now). **wlvncc**: UNKNOWN, the benchmark round reads it (`docs/proposals/remote-display-benchmarks.md`). No viewer in use is known to support E |
| **F. The viewer asks a prompt and the owner types it** | a dialog | the owner | works with any viewer; defeats "stays logged in, one click"; fallback only |
| **G. Nothing per launch** | the secret is in the program's own settings | that program's user | **Moonlight** (pairing keys in its settings) and **ssh** (a key file path, no secret in the arguments) |

**Proposal (DESIGN):** per viewer, the first row that works from this order: the viewer's own file-that-deletes-itself (C), then an environment variable (B), then a prompt (F). Concretely: `remote-viewer` uses C; TigerVNC uses B; Moonlight and ssh use G. A viewer we write ourselves could take the credential on a socket it inherits (E), which is the only method here that never touches an environment or a file; that is an argument for a hub-written viewer, weighed in `docs/proposals/remote-display-lod.md` and the benchmark document.

**Residual risk of every method:** while the viewer runs, the credential is in its memory. A hijacked program of user `hub` can probably read it (section 1, no Yama).

## 6. The s6 services after the split (DESIGN)

| Service | User | Starts after | Changes |
|---|---|---|---|
| `udevd`, `seatd` | root | - | none |
| `dbus` | `hub` | - | none |
| `driftwm` | `hub` (group `seat`) | seatd, udevd | none |
| `waybar` | `hub` | driftwm (`follow-driftwm`) | none (its module runs `hubd feed` over `hubd.sock`) |
| **`desk`** (new) | `hub` | driftwm (`follow-driftwm`) | `hubd desk`: starts, owns `desk.sock`; stops when driftwm is replaced, like Waybar and hubd today; its viewers die with it (own process group) |
| `hubd` | **`hubd`** | `desk` (waits for `desk.sock`; stops if the launcher process is replaced, so the panel never shows stale windows) | `s6-setuidgid hubd`, no `HUBENV` Wayland variables, reads `/config/hubos/secrets` |
| `confirm` | root | - | unchanged: asks `hubd.sock` |

The chain `confirm` -> `hubd` -> `desk` -> `driftwm` keeps the rule "a release whose desktop cannot start is never confirmed" (`docs/image.md` section 9). `follow-driftwm` is generalised to "follow this process" (a small change in the script, TESTED to need fixing once already: `docs/image.md`, the `pidof` bug).

`hub-env.sh` creates `/run/hubos` as `hub:hub` today; it would create `/run/hubos` for `hubd:hubctl` and `/run/deskd` for the launcher.

## 7. Alternatives considered

1. **Stay with one user and mode 0600 files** (today). Simple; a hijacked viewer reads everything. Rejected by the owner's decision to split.
2. **A root-owned signing helper only.** A tiny root daemon holds the management key and signs on request; hubd stays one user. Protects the management key well, small change; does **not** protect the display credentials (the hub has to hand them to viewers, so they sit with hubd). Cheaper first step: DESIGN option, if the full split is too much work before December.
3. **The split proposed here.** Protects both, at the cost of section 9.
4. **Containers or namespaces per viewer.** More isolation (a viewer could not see other processes), more machinery than the project wants; not examined.

## 8. What breaks in the existing hub image test (SOURCE: `tools/image/hub_test.go`, read 2026-10-05)

- **H0** (the hub root): checks that the six desktop services exist and that "user hub in group seat". It would need: user `hubd`, group `hubctl`, the seventh service `desk`, the modes of section 4 (add checks: the secrets folder is 0700 `hubd`; the desktop user cannot read it).
- **H1** (first boot): checks that "driftwm, Waybar and hubd run as the user hub". After the split `hubd` runs as `hubd`; add `desk` as `hub`; the check on the process owners changes.
- **H2** (kill driftwm): counts the pid changes of Waybar and hubd after the restart; it would also have to check `desk`.
- **H3, H3b** (`hubd open`, menu, row click): run `hubd open` as the desktop user through `hubd.sock`, which still works if the socket is reachable (group `hubctl`, peer check allows `hub`). The window placement now goes hubd -> launcher -> driftwm; the test result (window at its home) stays the same.
- **H4, H5** (A/B update, bad release): the confirm step asks `hubd.sock` as root (root can always connect). H5 breaks hubd by a flavour of the release (`unhealthy`); the same still works. A new variant should break only the launcher.
- **New tests to add** (all possible in QEMU): the desktop user cannot read the management key or any display credential; user `hubd` cannot open `/run/dw` or driftwm's socket; killing the launcher gets the desktop's windows closed and hubd restarted; a launch hands the viewer its credential and the file in `/run/dw/launch` is gone afterwards.
- **The base (`qemu-test`) machine** also runs `hubd` as `hub` (`image/rootfs/etc/s6/sv/hubd/run`); it has no desktop, so it needs no launcher. UNKNOWN: whether it should keep running hubd as `hub` (simplest) or switch to `hubd`; a choice for the owner.
- **`internal/hub` unit tests** use fakes for `Launcher` and `Compositor`; the remote versions need their own tests (a fake `desk.sock`).

## 9. Work, if the owner accepts (ESTIMATE, BELIEVED, nothing measured)

Go: a `desk` subcommand with a small request/response protocol, a remote `Launcher` and `Compositor` for the core, the credential store reader, peer-credential checks, tests (the biggest piece is the compositor subscription over the socket). Image: two users, three folders and their modes, one new s6 service, the generalised `follow-driftwm`, and the test changes in section 8. No new package. The secrets tool of `secrets.md` question 10 writes `/config/hubos/secrets/` as user `hubd`.

## 10. Questions for the owner

1. **Full split or the cheaper first step?** The full two-user split (section 3) or only a root-owned signing helper for the management key (section 7, option 2) now and the rest later?
2. **Is "the launcher decides" enough?** The launcher gets a credential and starts the viewer; a hijacked desktop program that can send `open` on `hubd.sock` can make hubd start any viewer, but never receives the credential itself (it can read the viewer's memory, section 1). Accept?
3. **The base `qemu-test` machine:** keep hubd as `hub` (no desktop there) or switch it to `hubd` too?
4. **Viewer choice** decides section 5 for the real viewer (`remote-viewer`, TigerVNC, wlvncc, one we write). Until the benchmark document, only the two rows marked SOURCE are known.
5. **Ptrace limits.** Do you want the hub kernel to get a ptrace restriction (Yama `ptrace_scope`) so that a hijacked `hub` program cannot read the memory of another? Needs a kernel option and a test; its effect on debugging tools is UNKNOWN.
