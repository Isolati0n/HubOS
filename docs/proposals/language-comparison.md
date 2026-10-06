# PROPOSAL: Go or Erlang/Elixir for the recovery agent and the node helper (data only)

**Status: PROPOSAL / MEASUREMENTS. Nothing here is decided and nothing here is in the images.** `HUB-OS.md` wins if this file disagrees with it (it says: `hubd` is Go; "Erlang/Elixir for uptime: parked; revisit later. v1 uses Go."). This file does not decide anything. It says what a small experiment showed. Written 2026-10-04 by a helper agent. **Unverified until the owner's lead has read the sources.**

The experiment is in `tools/image/experiments/language-comparison/` (see its `README.md`). Nothing in the main image, the image tests or `HUB-OS.md` was changed.

**Labels used on every item:**
- **TESTED**: I ran it in this build environment; the command and output are shown (or named in the section 9 list).
- **SOURCE**: read in an official source; link and the date I read it (2026-10-04). Every web page below was fetched with `curl` and read as the page's own text (not through a summarising tool), so none is marked "(summarised)". Exception: the Erlang pages are the **current** documentation (OTP 29.1.1); the experiment used OTP 25.
- **BELIEVED**: I think it is true, I did not test it and have no official source.
- **UNKNOWN**: nobody has checked; do not rely on it.

**All measurements come from one shared 4-core container (Linux 6.18, 16 GB RAM, nothing else heavy running), not from the target hardware.** Where a number was repeated, min / median / max are given.

---

## 1. Plain-words summary

1. **What I built.** The same tiny program twice: `GET /v1/status` returning the machine's release and slot as JSON (`{"release":"7","flavor":"hub","slot":"a"}`). Release from an `/etc/hubos-release`-style file, slot from `hubos.slot=` on a kernel-command-line-style file (both faked here). Once in Go (standard library only, static, built like hubd), once in Elixir (a `mix release` with its own Erlang runtime, no dependencies). Both give byte-identical answers.
2. **Size.** Go binary 5.7 MB (2.4 MB compressed). The Elixir release is 11.2 MB (5.4 MB compressed), plus about 3 MB of C++ and terminal libraries that the recovery kernel does not have today.
3. **Memory and start.** Go: 5.7 MB used, answers 4 ms after start. Elixir: 34 MB to 109 MB used (depending on one setting), answers 0.36 s to 0.9 s after start.
4. **A request handler that crashes.** Both survive: Go's web server catches a panic in the handler, Erlang kills only that connection's process. The difference: in Go, a crash in a *helper goroutine* kills the whole program; in Elixir the same kind of crash is harmless.
5. **The whole program killed (`kill -9`) under s6.** Both come back by themselves. Go answered again after about 1.0 s, Elixir after about 1.6 s. Most of both is s6's own wait of one second.
6. **What Elixir needs on the host.** The C libraries libc, libm, libgcc_s, libstdc++, libtinfo and libz; a shell and seven small tools (all in busybox); no writable folder at all if Erlang's networking between machines is switched off. With it on (the default) it starts `epmd`, listens on two ports on all network interfaces, and carries a secret "cookie" that shows in the process list.
7. **The Ubuntu Erlang package is not clean for this project.** `erlang-base` ships systemd unit files and depends on `libsystemd0`; its `epmd` program is linked to `libsystemd.so.0`. A `mix release` without `epmd` has none of this (TESTED).
8. **Hot code upgrade and Erlang's networking would add little and break some Hub OS rules** (section 6).
9. **Recovery kernel.** Estimated 6.3 MB today; about 8.7 MB with the Go status program, about 12.6 MB with the Elixir release. Estimates, not built kernels.
10. **Does this change the recommendation to keep Go?** The measured numbers all favour Go for the recovery agent. Elixir's real advantage (crashes stay inside one process) is real but small for a program this size, and Go plus s6 already recovers. The data does **not** argue for changing the recommendation. It also says nothing about long-running behaviour, load or the larger programs; those were not tested (section 8).

---

## 2. What was built

| | Go | Elixir |
|---|---|---|
| Files | `gostatus/main.go`, `main_test.go` | `elixir-status/lib/status/{application,listener,handler}.ex`, `mix.exs`, `test/` |
| Lines (code, tests) | 107 + 89 | 173 (+ 26 in mix.exs) + 26 |
| Libraries | Go standard library only (`net/http`, `encoding/json`) | none (no Hex). Erlang's `gen_tcp` with `packet: :http_bin` (the VM parses the HTTP request line and headers), JSON written by hand because OTP 25 has no `:json` module (BELIEVED: it arrived in OTP 27; I did not check) |
| Versions | Go 1.24.7 (the repo's) | Elixir 1.14.0 (`1.14.0.dfsg-2`), Erlang/OTP 25, erts 13.2.2.5 (`1:25.3.2.8+dfsg-1ubuntu4.7`), both from the Ubuntu 24.04 archive |
| Build | `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"` | `MIX_ENV=prod mix release` (`include_erts: true`, `strip_beams: true`, default) |
| Own tests | `go test`: 4 tests pass | `mix test`: 3 tests pass |

**TESTED: both answer the same.**

```
$ curl -si http://127.0.0.1:8481/v1/status        (Go)
HTTP/1.1 200 OK
Content-Type: application/json
Date: Sun, 04 Oct 2026 13:26:44 GMT
Content-Length: 41

{"release":"7","flavor":"hub","slot":"a"}

$ curl -si http://127.0.0.1:8481/v1/status        (Elixir release)
HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: 41
Connection: close

{"release":"7","flavor":"hub","slot":"a"}
```

(The fake files held `version=7`, `flavor=hub` and a command line with `hubos.slot=a`.) Differences between the two that are **not** equal: the Go server keeps connections alive and adds a `Date` header; the Elixir one closes after each answer. Odd input (invalid UTF-8, `<` or `&` in the release file) would be escaped differently by the two JSON writers; not tested.

**How the packages were obtained, without installing anything (TESTED).** A private apt configuration pointed at `/tmp/lang-apt` (lists and cache), the package files were fetched with `apt-get download` through the proxy and unpacked with `dpkg -x` into `/tmp/lang-root`:

```
Candidate: elixir 1.14.0.dfsg-2     erlang-base 1:25.3.2.8+dfsg-1ubuntu4.7     s6 2.12.0.3-1build1
$ erl -noshell -eval 'io:format("OTP ~s ~s~n",[erlang:system_info(otp_release), erlang:system_info(version)]), halt().'
OTP 25 13.2.2.5
$ elixir --version | tail -1
Elixir 1.14.0 (compiled with Erlang/OTP 24)
```

Ubuntu's `erl` script has `ROOTDIR=/usr/lib/erlang` written into it (it ignores `ERL_ROOTDIR`), so the scripts in the temporary folder were rewritten with `sed` to point at it. Hex (the package server) was not used and not tried. Newer Erlang/Elixir than Ubuntu's would have to be built by us; not tried.

---

## 3. Sizes (TESTED)

Go and Elixir builds are in `/tmp`; numbers from `ls -l`, `du -sb` and `sizes.py` (which packs files as a cpio "newc" archive, the initramfs format, and compresses with zlib level 9, the same algorithm as `gzip -9`).

| | Files | Raw bytes | gzip -9 |
|---|---|---|---|
| Go status program | 1 | **5,693,624** | **2,363,264** |
| Elixir release, as `mix release` makes it | 601 | **11,195,501** | **5,408,631** |
| Elixir release, helper programs removed (see below) | 589 | 10,493,410 | 5,137,472 |
| Libraries the Elixir runtime needs on top (libstdc++, libtinfo, libz, libgcc_s) | 4 | 3,096,576 | 1,111,258 |
| Elixir trimmed release + those four libraries | 593 | 13,589,986 | 6,247,705 |

`xz -9` of the Go binary alone: 1,875,588 bytes (for reference; the kernel is built with gzip, section 7).

What is inside the Elixir release (raw bytes, `du -sb`): `erts` 5.1 MB (`beam.smp`, the virtual machine, is 4.27 MB of it), `lib/elixir` 1.9 MB, `lib/stdlib` 1.8 MB, `lib/compiler` 0.98 MB, `lib/kernel` 0.89 MB, `lib/sasl` 0.16 MB, `lib/iex` 0.14 MB, `lib/logger` 0.06 MB, our own app 7 KB.

"Helper programs removed" means deleting `erl_call erlc escript typer yielding_c_fun dyn_erl heart run_erl to_erl start start_erl epmd` from `erts-13.2.2.5/bin`. **TESTED: the trimmed copy still starts and answers** (with `RELEASE_DISTRIBUTION=none`). I did not trim anything inside `lib/`, so a smaller release is probably possible; how much is UNKNOWN.

Libraries are from this container's Ubuntu 24.04 (`libstdc++.so.6.0.33` 2,592,224 bytes, `libtinfo` 208,328, `libz` 113,000, `libgcc_s` 183,024). `libc` (2,125,328) and `libm` (952,616) are not counted because the recovery kernel already carries libc for its tools; whether it already has `libm` is UNKNOWN (BELIEVED: not needed by busybox, maybe by e2fsprogs).

**If the real agent needed Erlang's `:crypto`** (for the Ed25519 signature check that the Go prototype does with the standard library): `crypto.so` is linked to `libcrypto.so.3` (TESTED with `readelf`), which is 5,309,400 bytes raw and 1,930,187 gzip -9 in this container. That is not in any number above. The status-only program does not need it. Whether OTP 25's `crypto` can verify signify's Ed25519 signatures was not tested (BELIEVED yes).

---

## 4. What each needs on the host

### 4.1 Go (TESTED)

```
$ file /tmp/lang-go-status
ELF 64-bit LSB executable, x86-64, version 1 (SYSV), statically linked, ... stripped
$ ldd /tmp/lang-go-status
	not a dynamic executable
```

No libraries, no shell, no helper programs, no writable folder (the program only reads two files), no secret. It needs a kernel with TCP and the usual futex/epoll (hubd already needs them).

### 4.2 Elixir release (TESTED unless noted)

- **Libraries.** `beam.smp` is linked to libtinfo, libz, libstdc++, libm, libgcc_s and libc (`readelf -d`, `ldd`). **`beam.smp` does not link `libsystemd`.** The other programs in the release are small C programs; **`epmd` does link `libsystemd.so.0`** (see 5).
- **A shell and tools.** The release start script `bin/status` is a `/bin/sh` script. `strace -f` of a start showed it running: `basename cat cut dirname grep readlink sed` (each called by the scripts; other branches of the script also use `od awk dd date mkdir`). The Ubuntu busybox 1.36.1 (`busybox-static`, unpacked, not installed) has all of them as applets. **Run with `env -i`, a PATH containing only busybox applet links, and busybox's own `sh` as the interpreter of every script, the release started and answered** (`{"release":"7","flavor":"hub","slot":"a"}`). That is the closest I could get to the recovery kernel without building one; the real recovery kernel was not used.
- **Writable folders: none, with networking off.** I started it as user `nobody` (uid 65534) with `HOME=/nonexistent`, and under `strace -f` the only file opened for writing was `/dev/null`; no folder was created. The release's `tmp` folder (`RELEASE_TMP`) is only written when a runtime-config file is used (`bin/status` source; SOURCE: [Mix.Tasks.Release, 1.14.0](https://hexdocs.pm/mix/1.14.0/Mix.Tasks.Release.html), read 2026-10-04: "The computed config file will be written to 'tmp' directory inside the release every time the system boots"). The `daemon` mode also writes `RELEASE_TMP/log` and a pipe (same page); the `start` mode used here does not. The release folder is read-only to the program.
- **`epmd` and the Erlang cookie.** With the default settings (`RELEASE_DISTRIBUTION` not set, which means `sname`):

```
$ python3 /tmp/lang-listen.py beam.smp epmd        (lists listening TCP sockets by program, from /proc)
/proc/net/tcp port 4369  addr 00000000  ('epmd')
/proc/net/tcp port 8481  addr 0100007F  ('beam.smp')     <- our HTTP port, localhost only
/proc/net/tcp port 35737 addr 00000000  ('beam.smp')     <- Erlang distribution, ALL interfaces
$ epmd -names
epmd: up and running on port 4369 with data:
name status at port 35737
```

  So by default the release starts a helper daemon (`epmd`, listening on port 4369 on all interfaces, left running after the program stops until it is killed) and opens a second port on all interfaces. **The cookie** (a shared secret that decides who may connect) is made at build time as a random string and stored in `releases/COOKIE` (56 characters here; SOURCE: Mix.Tasks.Release, read 2026-10-04: "a random cookie is written to the releases/COOKIE file when the first release is assembled"). At start the script passes it on the command line, and **it is visible in `ps` to every user on the machine** (TESTED: the process list showed `-setcookie <56 characters>`; the value is not reproduced here; it belonged to a throwaway test build that was deleted). No `.erlang.cookie` file was written in `HOME`.
- **With `RELEASE_DISTRIBUTION=none`:** no `epmd`, no second port, only our port 8481 on 127.0.0.1 (same `lang-listen.py` check). The cookie file is still created at build time and still passed on the command line (BELIEVED harmless; not examined).
- **Two builds of the same source differ in exactly one file: `releases/COOKIE`** (TESTED: SHA-256 of every file of two `mix release` runs compared; 1 of 601 files differs; the Go binary built twice has the same hash `e471917f0c43d6ab...`). Hub OS compares image hashes between builds (`docs/image.md`), so a random cookie would have to be fixed or removed. Setting a fixed cookie in `mix.exs` would put a secret in git (forbidden); `RELEASE_COOKIE` can be given at run time (SOURCE: the same page). Which of these to use is a question (section 10).
- **Kernel options.** The test kernel's fragment already has `CONFIG_FUTEX`, `EPOLL`, `SIGNALFD`, `TIMERFD`, `EVENTFD`, `UNIX`, `INET`, `PROC_FS`, `BINFMT_ELF` (`image/kernel/qemu-test.frag`, read). Whether the Erlang VM runs on that exact kernel was **not tested** (UNKNOWN). The experiment ran on this container's kernel.

---

## 5. systemd and the Ubuntu Erlang packages (TESTED)

The rule is "never systemd". What the Ubuntu 24.04 packages bring:

```
$ dpkg-deb -I erlang-base_1:25.3.2.8+dfsg-1ubuntu4.7_amd64.deb | grep Depends
 Depends: procps, adduser, libc6 (>= 2.38), libgcc-s1 (>= 12), libstdc++6 (>= 11), libsystemd0, libtinfo6 (>= 6), zlib1g (>= 1:1.2.8)
$ find /tmp/lang-root -iname '*systemd*'
./usr/lib/systemd                      -> contains system/epmd.service and system/epmd.socket
$ readelf -d erts-13.2.2.5/bin/epmd | grep NEEDED
 Shared library: [libsystemd.so.0]      (and `epmd -systemd` is one of its options)
$ grep -n systemd erlang-base's postinst
 deb-systemd-helper unmask 'epmd.service' ...   (the package's install script uses systemd helpers)
```

- `erlang-base` **depends on `libsystemd0`** (a library), on `procps` and on `adduser`. The image build deletes `procps` and `passwd` after the build and tolerates `libsystemd0` "for now" (HUB-OS.md); the build test fails if a systemd unit directory is in an image, and **the package contains `usr/lib/systemd/system/epmd.service` and `epmd.socket`**. So the package must not be unpacked into an image as it is.
- The VM itself (`beam.smp`) does not link libsystemd. **A `mix release` copies only `erts-*/bin` and the needed `lib/` folders**: the release I built contains no file with "systemd" in its name or text (`find` and `grep -rIl`: 0 hits). `epmd` inside it still links `libsystemd.so.0`; with distribution off it is not needed and the trimmed copy ran without it. So a design that uses a release with distribution off adds **no** systemd program, unit file or libsystemd0 dependency (TESTED on the files; nothing was put in an image).
- Elixir 1.14's package: `Depends: erlang-base, erlang-crypto, erlang-inets, erlang-parsetools, erlang-public-key, erlang-tools` (from `apt-cache depends`); nothing about systemd was found in it beyond what `erlang-base` brings.

---

## 6. Hot code upgrade and Erlang distribution in an immutable-image system

### 6.1 Hot code upgrade

**What it is (SOURCE).** [Release Handling, Erlang System Documentation](https://www.erlang.org/doc/system/release_handling.html), read 2026-10-04 (this is the OTP 29.1.1 page; OTP 25 may differ in details, UNKNOWN): the OTP application `SASL` gives "a framework for upgrading and downgrading between different versions of an entire release in runtime"; offline `systools` builds scripts and packages, online `release_handler` unpacks and installs them; a minimal system needs Kernel, STDLIB and SASL. Each modified application needs a hand-written `.appup` file (how to upgrade and downgrade) and a `relup` is built from those. The package is copied to `$ROOT/releases`, unpacked, installed; "the system keeps information about which versions are old and permanent in the files `$ROOT/releases/RELEASES` and `$ROOT/releases/start_erl.data`". [Code Replacement](https://www.erlang.org/doc/system/code_loading.html), read the same day: a module exists in "current" and "old" versions; loading a third kills processes still running the oldest.

**Elixir (SOURCE).** [Mix.Tasks.Release, Elixir 1.14.0](https://hexdocs.pm/mix/1.14.0/Mix.Tasks.Release.html), read 2026-10-04: "this feature is not supported out of the box by Elixir releases. The reason we don't provide hot code upgrades is because they are very complicated to perform in practice ... most teams can use other techniques ... such as Blue/Green deployments, Canary deployments, Rolling deployments ... hot upgrades are rarely a viable option." Also: `include_erts: false` "disables hot code upgrades".

**What it would add for Hub OS (BELIEVED, my reasoning, not tested):**
- The recovery agent: nothing. The recovery kernel is a RAM image, replaced as a whole file at the confirm step (`docs/image.md` 3.7) and started fresh at every boot.
- The node helper in a slot: it could be replaced without a restart. But a machine already "refuses to update or restart while a game or long job is running" (HUB-OS.md), and an update is a reboot into the other slot anyway.

**What it would break or strain (BELIEVED):**
- **Two sources of truth.** The image is signed and hashed and "an update is written to a second slot". Code loaded into a running node by `release_handler` is on disk under `$ROOT/releases` and in memory, but not in the signed manifest. The root would have to be writable (the release's `RELEASES` and `start_erl.data` files are rewritten, SOURCE above), which the immutable root is meant not to be.
- A hot-loaded change is lost at reboot or kept depending on `make_permanent`; both are states the A/B rollback does not know about.
- It needs hand-written `.appup` files per release, and Elixir's own authors advise against it (SOURCE above).
- Nothing in the experiment tests it. UNKNOWN whether it works on the image's read-only root at all.

### 6.2 Distribution (nodes talking to each other)

**SOURCE** (all read 2026-10-04, OTP 29.1.1 pages):
- [Distributed Erlang](https://www.erlang.org/doc/system/distributed.html): "communication between nodes is by default in clear text. If you need strong security, please see Using TLS for Erlang Distribution"; the default random cookie "is not very unpredictable"; cookies "are never transferred; instead, they are compared using hashed challenges, although not in a cryptographically secure manner"; at start a node looks for `.erlang.cookie` in the home folder and creates it (mode 400, random string) if there is none.
- [Distribution Protocol](https://www.erlang.org/doc/apps/erts/erl_dist_protocol.html): "The Erlang Distribution protocol is not by itself secure and does not aim to be so"; a node finds another's port through EPMD, "By default the EPMD listens on port 4369"; the handshake is "not entirely safe, as it is vulnerable against takeover attacks, but it is a tradeoff between fair safety and performance"; MD5 is used for the digest; a node with a wrong cookie is disconnected after 1 second.

**What it would add (BELIEVED unless stated):** a built-in way for hubd (if it were Erlang, which it is not) or another node to call functions on the node helper; remote shell into a running node (`bin/status remote`, SOURCE: Mix.Tasks.Release lists the `remote` command); several nodes supervising each other. hubd is Go, so hubd could not use it without a Go implementation of the protocol (UNKNOWN whether one exists; not looked for). HUB-OS.md also says: "Never invent a protocol"; the distribution protocol is existing, but a new dependency.

**What it would break or strain:**
- **A second, always-open network surface** (TESTED, section 4.2): `epmd` on 4369 and a random port, both on all interfaces. Hub OS already has an HTTP+JSON API for the agent (`docs/proposals/recovery-and-out-of-band.md`) with plain requests (request signing was removed on 2026-10-06; the image signature stays).
- **A shared secret that lives on the machine.** The recovery kernel is on the open boot partition and must hold public keys only ("secrets never in git", `recovery-and-out-of-band.md` 2.3). A cookie there would be readable by anyone with the disk, so **distribution would have to stay off in recovery** (as in my test). In a slot, the cookie would be a per-machine secret that has no design yet.
- The cookie appears in the process list (TESTED) and is not reproducible between builds (TESTED, 4.2).
- Clear text, MD5, takeover-vulnerable (SOURCE above). TLS distribution exists but needs certificates and private keys on every node; no design for that.
- Anyone who has the cookie and can reach the port can normally run code on the node (BELIEVED, widely stated; I did not read a sentence on it in the pages above, so treat this as unverified).

---

## 7. The recovery kernel with each (ESTIMATE, nothing was built)

**How the recovery kernel is built (SOURCE: repo, read).** `tools/image/build-kernel.sh` builds a kernel from `tinyconfig` plus a fragment; the recovery kernel's initramfs is the list written by `tools/image/recovery-list.py` (static busybox with all applets, `hubos-ctl`, `efibootmgr`, `signify-openbsd`, e2fsprogs, the update keyring and the libraries they need) and is built **into** the kernel (`CONFIG_INITRAMFS_SOURCE`, `image/kernel/*.frag`). The fragment has **`CONFIG_KERNEL_GZIP=y`**, so the whole bzImage (kernel and the initramfs inside it) is compressed with gzip. `docs/image.md` 3.7 and the recovery proposal give the recovery kernel as **6,333,440 bytes (6.3 MB)** today.

**Method.** Added files from section 3, compressed on their own with gzip level 9, added to 6,333,440. This assumes the added files compress the same way inside the kernel's single stream and that the kernel's own size does not change. The real number will differ by some hundreds of kilobytes either way (UNKNOWN how much).

| Recovery kernel carrying | Added raw (in RAM once unpacked) | Added compressed | Estimated recovery kernel |
|---|---|---|---|
| Today (no agent) | 0 | 0 | 6.33 MB |
| + Go status program | 5.69 MB | 2.36 MB | **about 8.7 MB** (+37%) |
| + Elixir release (as built) and the 4 libraries | 14.29 MB | 6.52 MB | **about 12.9 MB** (+103%) |
| + Elixir trimmed release and the 4 libraries | 13.59 MB | 6.25 MB | **about 12.6 MB** (+99%) |
| (+ `libcrypto` if the Elixir agent needed `:crypto`) | +5.31 MB | +1.93 MB | about 14.5 MB |

For comparison, the recovery proposal measured the full Go prototype agent (with the signature check) at 5,816,504 bytes; the status-only program here is 5,693,624, so the signature code adds little and the size is mostly Go's HTTP library (BELIEVED, not split).

The initramfs is unpacked into RAM at boot: add the "raw" column to the running memory in section 8. The RAM of the real machines is UNKNOWN (no hardware list yet).

---

## 8. Runtime measurements (TESTED, this container)

Commands: `python3 measure.py start 8481 N -- COMMAND` (starts the program N times, polls with a new HTTP request every 2 ms until it answers 200, makes three requests, waits 4 seconds, reads `/proc/PID/smaps_rollup` for the program and its children and for `epmd`, kills it); `measure.py killtime`. The program is started from a Python parent in a new session; the Elixir figures include the `sh` start scripts.

| | Time from start to first 200 answer | Idle RSS (sum of the processes) | Idle PSS (shared pages split fairly) |
|---|---|---|---|
| **Go** (n=15) | min 2.8, median **4.2**, max 6.0 ms | 5,736 / **5,860** / 6,148 KiB | 5,728 / **5,852** / 6,140 KiB |
| **Elixir, `RELEASE_DISTRIBUTION=none`** (n=15) | 523.9 / **641.1** / 1071.1 ms | 105,788 / **108,764** / 111,156 KiB | 84,240 / **87,288** / 89,722 KiB |
| Elixir, default (distribution on, `epmd`) (n=10) | 659.1 / **889.5** / 1023.2 ms | 105,448 / **108,264** / 111,048 KiB | 83,372 / **86,112** / 88,992 KiB |
| Elixir, none, `ERL_FLAGS="+S 1:1 +A 1"` (one scheduler) (n=10) | 706.3 / **812.2** / 994.6 ms | 104,404 / **104,894** / 106,504 KiB | 82,868 / **83,469** / 84,897 KiB |
| Elixir, none, **`RELEASE_MODE=interactive`** (n=8) | 268.5 / **357.2** / 559.0 ms | 44,260 / **44,718** / 46,632 KiB | 33,325 / **33,746** / 35,680 KiB |

Processes in the Elixir runs: `beam.smp` (about 105 MB RSS), `erl_child_setup` (1.8 MB), and with distribution on `epmd` (1 MB); Go: one process.

Reading it: the Erlang VM loads all of its code at start by default ("embedded" mode); `RELEASE_MODE=interactive` loads code on demand, which cut both start time and memory by more than half here (SOURCE: Mix.Tasks.Release, read 2026-10-04: embedded "increases boot time but it means the runtime will respond faster"; "interactive if you need to decrease boot time and reduce memory usage on boot"). I did not test what the first requests cost in interactive mode, nor whether memory grows later. Idle memory was read once, 4 seconds after start; **nothing was measured under load or over days** (UNKNOWN).

### 8.1 What happens when a request handler crashes (TESTED)

A test-only endpoint is compiled in but off unless asked (`-test-crash` for Go, `HUBOS_TEST_CRASH=1` for Elixir).

**Go**, `GET /v1/crash` (the handler calls `panic`):

```
$ curl -sS http://127.0.0.1:8481/v1/crash
curl: (52) Empty reply from server
$ curl -s http://127.0.0.1:8481/v1/status
{"release":"7","flavor":"hub","slot":"a"}
server log: http: panic serving 127.0.0.1:56260: test crash in a request handler
            goroutine 19 [running]: ... (stack trace)
```

The web server's own code caught the panic: that one connection got no answer; the next request worked. This is `net/http`'s behaviour (BELIEVED to be documented; I confirmed it by running).

**Go**, `GET /v1/crash-goroutine` (the handler starts a goroutine that panics, which `net/http` cannot catch):

```
$ curl -sS http://127.0.0.1:8481/v1/crash-goroutine
curl: (52) Empty reply from server
$ curl -sS -m 2 http://127.0.0.1:8481/v1/status
curl: (7) Failed to connect to 127.0.0.1 port 8481 after 0 ms: Couldn't connect to server
log: panic: test crash in a goroutine started by a handler      go exit code 2
```

The whole program died with exit code 2. Under s6 that is a restart (8.2). The lesson: in Go the code in the handler must not start unrecovered goroutines.

**Elixir**, `GET /v1/crash` (the handler process raises):

```
$ curl -sS http://127.0.0.1:8481/v1/crash
curl: (52) Empty reply from server
$ curl -s http://127.0.0.1:8481/v1/status
{"release":"7","flavor":"hub","slot":"a"}
log: [error] Task #PID<0.640.0> started from #PID<0.638.0> terminating
     ** (RuntimeError) test crash in a request handler ...
```

and `GET /v1/crash-process` (the handler starts another process that raises): the answer was `{"note":"survived"}` HTTP 200, the log shows the second process's error, and the VM's process id was the same before and after (`12863`). Each connection has its own process under a `Task.Supervisor`; a crash ends only that process and is logged. What I did **not** test: a crash of the listener process (it is restarted by the supervisor in the code; not exercised); a handler that loops forever; a handler that allocates without limit (BELIEVED: Erlang can cap one process's heap, Go cannot; not read, not tested).

### 8.2 What happens when the whole program is killed, under s6 (TESTED)

s6 2.12.0.3 (Ubuntu `s6`, unpacked with `dpkg -x`, not installed), a throwaway scan folder in `/tmp/lang-s6/scan` with two services whose `run` scripts `exec` the program (`s6-svscan` was started from a copy in which the compiled-in path `/usr/bin/s6-supervise` was replaced by a path of the same length in `/tmp`, because the Ubuntu build looks for `s6-supervise` only there). Method: wait until the service answers, wait 2 s, `kill -9` the pid that `s6-svstat` reports, time until `/v1/status` answers again. Ten kills each (`measure.py killtime`).

| | kill -9 to answering again | processes left behind |
|---|---|---|
| Go | min 1007.7, median **1010.4**, max 1027.5 ms | 0 |
| Elixir (`RELEASE_DISTRIBUTION=none`, embedded mode) | min 1504.2, median **1614.7**, max 2396.4 ms | 0 (`erl_child_setup` exited with its parent) |

Nearly all of Go's 1.0 s is s6 itself: SOURCE: [s6-supervise](https://skarnet.org/software/s6/s6-supervise.html), read 2026-10-04: "There is a minimum 1-second delay between two ./run spawns ... If the service has been ready for more than one second, it will restart immediately, but if it is not ready when it dies, s6-supervise will always pause for 1 second". Our services ran 2 s before the kill but had no readiness notification, and the delay was still 1 s: I did not find out why (UNKNOWN; it may be that "ready" needs a `notification-fd`). Elixir's extra 0.6 s is its start time (section 8, median 641 ms). The `kill -9` of the BEAM also kills nothing else that matters here: a `kill -9` of only the main process left no orphan (checked).

**Not applicable in the recovery kernel today.** Recovery's init is a shell script that ends in a bare shell (`image/stage0/recovery-init`); it has no s6. So a killed agent in recovery would stay dead whichever language it is written in (SOURCE: repo; BELIEVED consequence). Erlang's own supervisors restart processes inside the VM but not the VM itself (BELIEVED; `heart` can restart it and was removed from the trimmed copy). Adding s6 to the recovery kernel is a question (section 10).

---

## 9. What was run, in one place

- Worktree `/home/user/wt-lang`, branch `language-comparison`; everything else under `/tmp/lang-*` (packages, build output, fake files, logs), deleted at the end; nothing installed (no `apt install`); the temporary apt state was private.
- `go test -count=1 ./tools/image/experiments/language-comparison/...`: pass; `mix test` and `mix format --check-formatted` pass; the repo's `gofmt -l .`, `go vet ./...` and `go test -count=1 ./...` output are in the pull request and the final report.
- Pages read in full by me with `curl` (no summarising tool): `erlang.org/doc/system/{distributed,release_handling,code_loading}.html`, `erlang.org/doc/apps/erts/erl_dist_protocol.html`, `hexdocs.pm/mix/1.14.0/Mix.Tasks.Release.html`, `skarnet.org/software/s6/s6-supervise.html`; I searched them with a script for the passages quoted, and the quotes above are from that text.
- **Not tested:** the real recovery agent in Elixir (signature check, install); anything inside the real recovery kernel; the Erlang VM on the image's kernel; load, long runs, memory growth; a crash of the Elixir listener; Hex; a newer OTP than 25 (the docs read are for 29.1.1); a trimmed `lib/`; `heart`; the Go side of the Erlang distribution protocol; time and memory on real hardware.

---

## 10. Does this change the recommendation to keep Go?

I am not deciding. The data shows:

- For a **status endpoint** (and, by the earlier prototype, probably the whole recovery agent): Go is smaller (5.7 against 10.5 to 14 MB raw; 2.4 against 5.1 to 6.5 MB compressed), starts about 85 to 210 times sooner (4 ms against 357 to 889 ms), uses about 6 MB against 34 to 109 MB, needs no libraries, no shell and no secret, and builds to the same bytes twice. The Elixir release needs six shared libraries, a shell with seven tools, and switching distribution off to avoid two open ports and a secret on the command line.
- Elixir's best measured point is isolation: a handler crash, including in a helper process, leaves everything else running. Go's `net/http` also survives a handler panic; only an unrecovered panic in a goroutine the handler starts kills the program, and then s6 restarts it in about 1 s (Elixir: about 1.6 s). For a program whose job is four calls, the measured difference is small.
- The recovery kernel would grow by an estimated 2.4 MB (Go) against 6.3 to 6.5 MB (Elixir), and it has no process supervisor today, so the "uptime" advantage of the Erlang VM would not apply to a killed VM there.
- The Ubuntu Erlang package carries systemd unit files and `libsystemd0` dependencies; a self-contained release avoids them but needs care (`epmd`, cookie).
- Hot code upgrade and distribution add little and conflict with the immutable, signed, A/B design (section 6).
- **This does not test** what "Erlang for uptime" is really about (many processes, long runs, failure under load, the larger node helper). The experiment cannot say anything about that. If the owner wants that question answered, it needs a larger test.

**The data does not change the recommendation to keep Go for v1.** The parked idea stays parked.

### Questions for the owner

1. Is a recovery kernel of 8.7 MB (Go) or 12.6 MB (Elixir), estimated, acceptable, or is there a size limit (the boot partition's free space is tested at confirm; I did not look at its size)?
2. Should the node helper's "uptime" question get its own larger test (more than a status call: a crash storm, a long run, memory over days), or stay parked?
3. If a BEAM release is ever used: distribution off everywhere (my test setting), or on in slots with a design for the cookie? And may a build contain a fixed or removed `releases/COOKIE` (it makes builds differ otherwise)?
4. Should the recovery kernel get a process supervisor (for example s6) for the agent? Today a killed agent stays dead in either language.
5. May I measure the real full recovery agent (with the signature check) in Elixir, which needs `:crypto` and so `libcrypto` (+5.3 MB raw, +1.9 MB compressed, measured), or is the size already answer enough?
6. Should I try the Elixir/Erlang that is newer than Ubuntu's (OTP 25 is old; the docs I read are for OTP 29.1.1)? That would mean building it ourselves.
7. Do you want Hex (package server) tested through the proxy? I used no dependencies and did not try it.
