# Build environment

**Audited: 2026-10-01 — build environment; will change.**

This is what the bot's cloud session could and could not do when it was audited. Nothing here describes the owner's real hardware. If this file disagrees with `HUB-OS.md`, `HUB-OS.md` wins.

Every item is labelled with exactly one of:

- **TESTED** — the command shown was run in the audit session and gave the result written here. Run it again to check it still holds.
- **BELIEVED** — thought to be true, not tested. The reason is given.
- **UNKNOWN** — not known.

## The short version

- There is **no `/dev/kvm`**. Virtual machines run in software emulation only, so they are slow, and speed, delay and jitter cannot be measured here.
- The machine is **ephemeral**. Anything not pushed to GitHub is lost when the session ends.
- Outbound **ports 22 and 53 are blocked**. Other GitHub repositories are blocked until they are attached to the session.
- This sandbox's own address is in **192.0.2.0/24**, the same range the example inventory uses. **Tests must never rely on 192.0.2.x being unreachable.**
- The repository is **public**. Nothing secret may go in git.

## How to repeat the commands

Commands that make files use `$A`, an empty temporary directory outside the repo:

```
A=$(mktemp -d)
```

Delete it when finished. The commands below that unpack packages never install anything on the machine.

---

## 1. Can do (tested)

### 1.1 The machine

**TESTED.**

```
uname -a; head -4 /etc/os-release; nproc; free -m; df -h /
```

Result: Linux 6.18.44 (a Firecracker virtual machine; the kernel command line contains `--firecracker-init`), Ubuntu 24.04.4 LTS, 4 CPUs (Intel Xeon 2.1 GHz), 16 GB memory, no swap, root disk about 30 GB available, running as root.

### 1.2 Go builds, tests and cross-builds

**TESTED.**

```
go version
gofmt -l .
go vet ./...
go test -count=1 ./...
GOOS=linux GOARCH=arm64 go build -o /dev/null ./cmd/hubd
GOOS=linux GOARCH=riscv64 go build -o /dev/null ./cmd/hubd
```

Result: Go 1.24.7, nothing listed by gofmt, vet clean, all tests pass, both cross-builds succeed.

### 1.3 Tools present and missing

**TESTED.**

```
for t in gcc make git python3 perl clang cmake ninja gdb docker node cargo curl openssl gpg bison mkfs.ext4 unshare qemu-system-x86_64 qemu-img flex cpio mksquashfs xorriso debootstrap zstd ssh sshd minisign dnsmasq ip ping lspci lsusb aarch64-linux-gnu-gcc; do printf "%-26s %s\n" "$t" "$(command -v $t || echo MISSING)"; done
```

Present: gcc, make, git, python3, perl, clang, cmake, ninja, gdb, docker (client only), node, cargo, curl, openssl, gpg, bison, mkfs.ext4, unshare. Missing: qemu, flex, cpio, mksquashfs, xorriso, debootstrap, zstd, ssh, sshd, minisign, dnsmasq, ip, ping, lspci, lsusb, aarch64-linux-gnu-gcc.

### 1.4 Reaching web sites

**TESTED.**

```
for u in https://github.com https://api.github.com https://proxy.golang.org https://kernel.org https://cdn.kernel.org/pub/linux/kernel/v6.x/ https://dl-cdn.alpinelinux.org/alpine/latest-stable/releases/x86_64/ https://deb.debian.org/debian/ http://deb.debian.org/debian/ https://archive.ubuntu.com/ubuntu/ https://mirrors.kernel.org/ https://example.com https://pypi.org/simple/ https://crates.io https://registry.npmjs.org/ https://raw.githubusercontent.com/Isolati0n/HubOS/main/CLAUDE.md https://ghcr.io https://registry-1.docker.io/v2/ https://download.qemu.org https://sunshinestream.io https://github.com/LizardByte/Sunshine/releases https://pipewire.org https://nixos.org https://cache.nixos.org/nix-cache-info https://www.google.com; do printf "%-85s " "$u"; curl -sS -o /dev/null --max-time 20 -w "%{http_code} %{time_total}s\n" -I "$u" 2>&1 | tail -1; done
```

Result: 200 for the Go proxy, kernel.org, cdn.kernel.org, the Alpine, Debian (https and http) and Ubuntu mirrors, mirrors.kernel.org, example.com, pypi.org, registry.npmjs.org, raw.githubusercontent.com (this repo), download.qemu.org, pipewire.org, nixos.org, cache.nixos.org and google.com. Not 200: `https://github.com` and `https://api.github.com` gave 400 to this HEAD request (a GET to github.com gave 200); `https://crates.io` 403 (the crates index below is fine); `https://ghcr.io` 405; `https://registry-1.docker.io/v2/` 401 (it wants a login); `https://sunshinestream.io` failed (000); the LizardByte/Sunshine releases page gave 403 (see 3.4).

Also **TESTED**: `curl -sS --max-time 15 -o /dev/null -w "%{http_code}\n" https://index.crates.io/config.json` and `https://static.crates.io/crates/libc/libc-0.2.150.crate` both give 200.

Also **TESTED**: `getent hosts github.com kernel.org` resolves both.

### 1.5 Downloading large files

**TESTED.**

```
curl -sS --max-time 280 -o linux-6.12.tar.xz -w "http=%{http_code} bytes=%{size_download} time=%{time_total}s\n" https://cdn.kernel.org/pub/linux/kernel/v6.x/linux-6.12.tar.xz
```

Result: 147,906,904 bytes in 2.4 seconds.

### 1.6 Listening and connecting on the same machine

**TESTED.**

```
python3 - <<'EOF'
import socket
for bind in ["127.0.0.1","127.0.0.77","0.0.0.0"]:
    s=socket.socket(); s.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,1); s.bind((bind,0)); s.listen(); port=s.getsockname()[1]
    target="127.0.0.1" if bind=="0.0.0.0" else bind
    c=socket.create_connection((target,port),timeout=2); a,_=s.accept(); c.sendall(b"hi"); print(bind,port,a.recv(2)); c.close(); a.close(); s.close()
s=socket.socket(); s.bind(('192.0.2.2',0)); s.listen(); p=s.getsockname()[1]
socket.create_connection(('192.0.2.2',p),timeout=2); print('connect to own eth0 address ok')
EOF
```

Result: every connection works, including to the machine's own non-loopback address. The `hubd` tests rely on this: they use `127.0.0.x` addresses and ports the operating system hands out.

### 1.7 Compiling a Linux kernel

**TESTED.** A full x86-64 `defconfig` build of Linux 6.12 took 441 seconds on the 4 CPUs. `flex` and `cpio` are not installed, so they were unpacked from packages into `$A` without installing anything:

```
mkdir -p $A/debs $A/root && cd $A/debs
apt-get download flex libfl2 cpio
apt-get download libelf-dev libelf1t64
for d in *.deb; do dpkg -x $d $A/root; done
ln -sf /usr/lib/x86_64-linux-gnu/libelf.so.1 $A/root/usr/lib/x86_64-linux-gnu/libelf.so
cd $A
curl -sS -o linux-6.12.tar.xz https://cdn.kernel.org/pub/linux/kernel/v6.x/linux-6.12.tar.xz
tar -xf linux-6.12.tar.xz
export PATH=$A/root/usr/bin:$PATH LD_LIBRARY_PATH=$A/root/usr/lib/x86_64-linux-gnu
cd linux-6.12
make defconfig HOSTCFLAGS="-I$A/root/usr/include" HOSTLDFLAGS="-L$A/root/usr/lib/x86_64-linux-gnu"
time make -j4 bzImage HOSTCFLAGS="-I$A/root/usr/include" HOSTLDFLAGS="-L$A/root/usr/lib/x86_64-linux-gnu"
```

Result: `Kernel: arch/x86/boot/bzImage is ready`, 13,464,576 bytes. The source tree used 1.6 GB. Peak memory was not measured. Without `flex`, `make defconfig` fails with `flex: not found` (also **TESTED**). If a download in this recipe returns 404, the package list in the image is out of date; see 1.8.

### 1.8 Fetching packages without installing them

**TESTED.** The package index in the image was out of date: the listed QEMU build `8.2.2+ds-0ubuntu1.17` returned 404, while `…1.18` exists. A fresh index kept in a temporary directory fixed it, without touching system files:

```
mkdir -p $A/apt/lists/partial $A/apt/cache/archives/partial
O="-o Dir::State::lists=$A/apt/lists -o Dir::Cache=$A/apt/cache -o APT::Sandbox::User=root"
apt-get $O update
apt-get $O install --print-uris -y --no-install-recommends qemu-system-x86 seabios qemu-system-data
```

Package availability checked with `apt-cache $O policy NAME`:

- In the Ubuntu 24.04 archive: pipewire 1.0.5, wireplumber, weston 13, sway, cage, labwc, libwlroots-dev, xwayland, s6 2.12.0.3, runit, squashfs-tools, qemu-utils, ovmf, swtpm, minisign, signify-openbsd, cryptsetup, debootstrap, busybox-static, dracut, mkosi, grub-efi-amd64-bin, systemd-boot, openssh-server.
- Not in the archive: moonlight-qt, sunshine, dinit.

### 1.9 Building small filesystem images

**TESTED.**

```
truncate -s 16M $A/t.img && mkfs.ext4 -q $A/t.img
mkdir -p $A/mnt && mount -o loop $A/t.img $A/mnt && mount | grep $A/mnt; umount $A/mnt
mkdir -p $A/mnt2 && mount -o ro,loop $A/t.img $A/mnt2 && touch $A/mnt2/x; umount $A/mnt2
```

Result: the loop mount works. A read-only mount refuses writes: `touch: cannot touch 'mnt2/x': Read-only file system`.

This machine's own kernel lists ext4, squashfs, erofs and overlay in `/proc/filesystems` (**TESTED**: `cat /proc/filesystems`). The `defconfig` kernel built in 1.7 has ext4 but not squashfs, overlay or erofs (**TESTED**: `grep -E 'CONFIG_(SQUASHFS|OVERLAY_FS|EROFS_FS|EXT4_FS)=' linux-6.12/.config` shows only `CONFIG_EXT4_FS=y`).

### 1.10 Running QEMU in software emulation

**TESTED.** QEMU is not installed. It was unpacked from packages into `$A`:

```
mkdir -p $A/qdebs $A/qroot && cd $A/qdebs
apt-get $O install --print-uris -y --no-install-recommends qemu-system-x86 seabios qemu-system-data 2>&1 | grep -oE "^'[^']+'" | tr -d "'" > $A/qemu-uris.txt
while read u; do curl -sS --max-time 120 -O "$u"; done < $A/qemu-uris.txt
for d in *.deb; do dpkg -x "$d" $A/qroot; done
```

A tiny test program as the guest's first process (`init.c`), turned into an initramfs (the `cpio` unpacked in 1.7 must be on `PATH`):

```
#include <stdio.h>
#include <fcntl.h>
#include <unistd.h>
#include <sys/mount.h>
#include <sys/stat.h>
#include <sys/utsname.h>
#include <sys/reboot.h>
int main(void) {
    mkdir("/dev", 0755);
    mount("devtmpfs", "/dev", "devtmpfs", 0, NULL);
    int fd = open("/dev/console", O_RDWR);
    if (fd >= 0) { dup2(fd, 0); dup2(fd, 1); dup2(fd, 2); }
    struct utsname u; uname(&u);
    printf("HELLO FROM GUEST: %s %s %s\n", u.sysname, u.release, u.machine);
    fflush(stdout);
    sync();
    reboot(RB_POWER_OFF);
    return 0;
}
```

```
gcc -static -O2 -o init init.c
echo init | cpio -o -H newc > $A/initramfs.cpio   # run in the directory that holds init
```

Boot it with the kernel from 1.7:

```
export LD_LIBRARY_PATH=$A/qroot/usr/lib/x86_64-linux-gnu:$A/qroot/lib/x86_64-linux-gnu QEMU_MODULE_DIR=$A/qroot/usr/lib/x86_64-linux-gnu/qemu
Q=$A/qroot/usr/bin/qemu-system-x86_64
$Q -L $A/qroot/usr/share/qemu -L $A/qroot/usr/share/seabios -machine pc -accel tcg -smp 2 -m 512 -nographic -no-reboot -nic none -kernel $A/linux-6.12/arch/x86/boot/bzImage -initrd $A/initramfs.cpio -append "console=ttyS0 panic=-1"
```

Result: the guest printed `HELLO FROM GUEST: Linux 6.12.0 x86_64` and powered off. Total time 3.6 seconds. QEMU needed `QEMU_MODULE_DIR` and `-nic none` to run from unpacked packages.

### 1.11 Network booting a guest (inside QEMU only)

**TESTED.** QEMU's built-in TFTP server and the iPXE boot ROM (from the `ipxe-qemu` package, unpacked with the QEMU files in 1.10):

```
mkdir -p $A/tftp && cp $A/linux-6.12/arch/x86/boot/bzImage $A/initramfs.cpio $A/tftp/
printf '#!ipxe\necho NETBOOT: script fetched by iPXE over TFTP\nkernel tftp://10.0.2.2/bzImage console=ttyS0 panic=-1\ninitrd tftp://10.0.2.2/initramfs.cpio\nboot\n' > $A/tftp/boot.ipxe
ROM=$(find $A/qroot -name pxe-e1000.rom | head -1)
$Q -L $A/qroot/usr/share/qemu -L $A/qroot/usr/share/seabios -machine pc -accel tcg -smp 2 -m 768 -nographic -no-reboot -boot n -netdev user,id=n0,tftp=$A/tftp,bootfile=boot.ipxe -device e1000,netdev=n0,romfile=$ROM
```

Result: iPXE got an address, fetched the script, the kernel and the initramfs over TFTP, and the guest printed `HELLO FROM GUEST: Linux 6.12.0 x86_64` (about 11.6 seconds).

### 1.12 Signing and checking a file

**TESTED.**

```
openssl genpkey -algorithm ed25519 -out sign.key
openssl pkey -in sign.key -pubout -out sign.pub
openssl pkeyutl -sign -inkey sign.key -rawin -in bzImage -out bzImage.sig
openssl pkeyutl -verify -pubin -inkey sign.pub -rawin -in bzImage -sigfile bzImage.sig
cp bzImage tampered && printf 'X' | dd of=tampered bs=1 seek=1000 conv=notrunc
openssl pkeyutl -verify -pubin -inkey sign.pub -rawin -in tampered -sigfile bzImage.sig
```

Result: `Signature Verified Successfully` for the original; `Signature Verification Failure` (exit 1) for the changed file. This proves only that the tool detects a changed file. It does not prove that a machine refuses an unsigned image.

### 1.13 PipeWire without systemd

**TESTED.** PID 1 is not systemd, and `[ -d /run/systemd/system ]` is false. PipeWire and Weston were unpacked into `$A/pwroot` the same way as QEMU (`apt-get $O install --print-uris -y --no-install-recommends pipewire pipewire-bin libspa-0.2-modules weston`, download, `dpkg -x`):

```
R=$A/pwroot; export XDG_RUNTIME_DIR=$A/xdg; mkdir -p $XDG_RUNTIME_DIR; chmod 700 $XDG_RUNTIME_DIR
export LD_LIBRARY_PATH=$R/usr/lib/x86_64-linux-gnu:$R/lib/x86_64-linux-gnu
export SPA_PLUGIN_DIR=$R/usr/lib/x86_64-linux-gnu/spa-0.2 PIPEWIRE_MODULE_DIR=$R/usr/lib/x86_64-linux-gnu/pipewire-0.3 PIPEWIRE_CONFIG_DIR=$R/usr/share/pipewire
$R/usr/bin/pipewire & sleep 3; $R/usr/bin/pw-cli info 0
```

Result: PipeWire 1.0.5 ran and answered (`id: 0`, `version: "1.0.5"`). It logged only missing-D-Bus errors. No sound device exists here, so no audio was tested, and WirePlumber was not run.

### 1.14 A Wayland compositor with no screen

**TESTED.** Weston 13 (unpacked as in 1.13) on the headless backend with the software renderer:

```
export LD_LIBRARY_PATH=$R/usr/lib/x86_64-linux-gnu:$R/lib/x86_64-linux-gnu:$R/usr/lib/x86_64-linux-gnu/weston
L=$R/usr/lib/x86_64-linux-gnu
export WESTON_MODULE_MAP="headless-backend.so=$L/libweston-13/headless-backend.so;desktop-shell.so=$L/weston/desktop-shell.so;kiosk-shell.so=$L/weston/kiosk-shell.so"
$R/usr/bin/weston --backend=headless --renderer=pixman --socket=wl-audit --idle-time=0 --shell=kiosk-shell.so &
sleep 4; WAYLAND_DISPLAY=wl-audit timeout 4 $R/usr/bin/weston-simple-shm
```

Result: the log says `Using Pixman renderer`, the socket `wl-audit` appears, and the client keeps running until the timeout (exit 124). This is on this machine, not inside a virtual machine.

### 1.15 Namespaces

**TESTED.** `unshare -n -- true && echo netns ok; unshare -U -- true && echo userns ok` — both print their message.

### 1.16 Background jobs

**TESTED.** A command started with `nohup ./build.sh &` (the kernel build in 1.7) kept running across many later commands, and a notification arrived when a waiting job finished. Whether such jobs survive between the owner's messages is **UNKNOWN**.

### 1.17 Web search

**TESTED.** The WebSearch tool, query `driftwm Wayland compositor infinite canvas`. It returned results. They describe driftwm as a Rust and Smithay Wayland compositor (taken from the search results; its source was not read).

### 1.18 GitHub: what worked

**TESTED**, all in the audit session:

- Push to the designated branch: `git push -u origin ccr-f9374241-r0yar7` (a fast-forward).
- Open a pull request: tool `mcp__github__create_pull_request` (pull requests 3 and 4).
- Merge a pull request with a merge commit, when told to: tool `mcp__github__merge_pull_request` with `merge_method: merge` (pull requests 2, 3, 4).
- Read-only calls: tools `mcp__github__get_me`, `mcp__github__list_branches`, `mcp__github__list_repository_collaborators`, `mcp__claude-code-remote__list_repos`.
- Token metadata (prints no secret):
  ```
  curl -sS --max-time 20 -D - -o /dev/null -H "Authorization: Bearer $GITHUB_TOKEN" https://api.github.com/user | grep -iE '^(HTTP|x-oauth-scopes|github-authentication-token-expiration)'
  curl -sS --max-time 20 -H "Authorization: Bearer $GITHUB_TOKEN" https://api.github.com/repos/Isolati0n/HubOS | python3 -c "import sys,json; d=json.load(sys.stdin); print({k:d.get(k) for k in ['full_name','private','default_branch','permissions']})"
  ```
  Result: the token expired at 2026-10-02 04:38:26 UTC; no OAuth scopes listed; the repository is public (`private: False`); GitHub reports `admin: True` for the user. Whether the token may use all of that is **UNKNOWN**.

---

## 2. Needs real hardware or a real test

Each item says what is missing. Items marked TESTED here are the part that was tested; the rest is not.

| Item | Label | What is missing |
|---|---|---|
| driftwm running at all | UNKNOWN | Its source is on GitHub, which is blocked until the repository is attached with `add_repo` (see section 3). Needs a build with cargo (the crates index is reachable, section 1.4) and a way to run it: a nested window, a virtual machine with a graphics device, or a real GPU. Whether it has a headless mode is unknown. Whether it works without systemd is the open question in `HUB-OS.md`. |
| PipeWire playing sound | UNKNOWN | The daemon itself ran without systemd (1.13, TESTED). No sound device exists here. Needs a virtual machine with a virtual sound card, or real hardware. WirePlumber was not run. |
| Moonlight and Sunshine streaming | UNKNOWN | Neither is in the Ubuntu archive (1.8). Sunshine's GitHub releases are blocked unless attached. There is no GPU, so no hardware video encoder. Needs real hardware. |
| A Wayland compositor inside a virtual machine | UNKNOWN | It ran on this machine without a screen (1.14, TESTED). Inside a QEMU guest it needs a root filesystem, a graphics device and a kernel with graphics support, and is slow without KVM. |
| Network boot on a real network | UNKNOWN | It worked inside QEMU (1.11, TESTED). A real network needs DHCP and TFTP servers (`dnsmasq` is in the archive but not installed) and a real machine. UEFI network boot was not tried. |
| A machine refusing an unsigned image | UNKNOWN | Signing and checking a file worked (1.12, TESTED). Not tested: the machine enforcing it at boot or update, key storage, or secure boot. |
| Read-only root with a second update slot | UNKNOWN | A read-only mount of a file works (1.9, TESTED). Not tested: a booted read-only root, the switch between two slots, rollback. Needs a bootloader that is not systemd's, a disk image, and QEMU with UEFI firmware (`ovmf` is in the archive). |
| A bootable disk image with a bootloader | BELIEVED | Believed possible: grub-efi and ovmf are in the archive (1.8). Not tried. |
| Speed of a whole operating system under emulation | UNKNOWN | Only a tiny boot was timed (3.6 s, 1.10). Believed much slower than with KVM. |
| Performance, latency and jitter | BELIEVED | Believed to be unmeasurable under software emulation or in this sandbox, because emulated timing is not real timing. These tests wait for real hardware (Phase C). |
| Input forwarder testing | UNKNOWN | `/dev/uinput` is absent (section 3). A test needs a virtual machine whose kernel is built with uinput, and whether the `defconfig` kernel of 1.7 includes uinput was not checked. |
| hubd on a real network | UNKNOWN | The 2 s and 5 s limits are guesses, and whether a bare TCP connect disturbs a real Sunshine, SPICE or VNC session was not tested. |
| Docker | UNKNOWN | The client is installed (1.3). `dockerd` exists at `/usr/bin/dockerd` but no daemon is running (`docker ps` cannot connect to `/var/run/docker.sock`). Starting it was not tried. |
| Installing packages system-wide with `apt` | BELIEVED | Believed to work, since the session runs as root and the mirror answers. Not tested; the audit only downloaded and unpacked packages. |
| Cross-compiling C with clang | UNKNOWN | clang is installed; cross-targets were not tried. Go cross-builds work (1.2). |
| Seeing a screen or taking a screenshot | UNKNOWN | A Chromium browser is said to be preinstalled (system prompt). Not tested. |
| Fetching a web page with the WebFetch tool | UNKNOWN | The tool exists; not used. |
| Creating a new branch name, a new repository, deleting a branch | BELIEVED | Tools exist for some (`create_branch`, `create_repository`). My instructions limit work to the designated branch. Not tested. |
| Changing repository settings, default branch, visibility or secrets | UNKNOWN | No tool for these was seen. Not tested. |
| Reading a private repository | UNKNOWN | The account lists two repositories, both public. |
| Attaching another repository | BELIEVED | `add_repo` exists. Not tested. |
| Scheduling work, starting helper sessions, running tasks in parallel | BELIEVED | Tools exist (ScheduleWakeup, Cron tools, `send_later`, `create_trigger`, `create_session`, the Agent tool). Not used. Several tool calls at once did work (TESTED, many times in the session). |
| How long a session can run | UNKNOWN | The token expiry in 1.18 suggests about 8 hours (BELIEVED). |
| Longest single command | BELIEVED | About 2 minutes by default, 10 minutes at most in the foreground, longer in the background. This comes from the tool description. The maximum was not tried. |
| How much conversation I can hold | UNKNOWN | The system prompt says old context is summarized when it grows long. |
| Inbound connections from outside | UNKNOWN | Believed not possible: the machine sits behind a gateway (section 3). Not tested from outside. |
| Outbound UDP | UNKNOWN | Not tested. |
| Commit signing is accepted by GitHub | UNKNOWN | Git is configured to sign commits with an SSH key. Whether GitHub shows them as verified was not checked. |

---

## 3. Cannot do here

### 3.1 No hardware virtualization

**TESTED.**

```
ls -l /dev/kvm
grep -cE ' (vmx|svm) ' /proc/cpuinfo
zcat /proc/config.gz | grep -E 'CONFIG_KVM|CONFIG_VIRTUALIZATION'
cat /proc/misc
$Q -accel kvm -machine q35 -display none -S -monitor none -serial none
```

Result: `ls: cannot access '/dev/kvm'`; 0 CPUs with `vmx` or `svm`; `CONFIG_KVM=y`, `# CONFIG_KVM_INTEL is not set`, `# CONFIG_KVM_AMD is not set`, `CONFIG_KVM_PVM=y`; no `kvm` line in `/proc/misc`; QEMU says `failed to initialize kvm: No such file or directory`. Virtual machines therefore run in software emulation only (`-accel tcg`). Nested virtualization is not available. Whether the `PVM` option could be made usable is **UNKNOWN**.

### 3.2 No graphics, sound, input or USB devices

**TESTED.**

```
ls -d /dev/dri /dev/snd /dev/input /dev/fb0 /dev/uinput /dev/uhid /dev/vhost-net /sys/class/drm /sys/class/sound /sys/bus/usb/devices
```

Result: none of these exist. `/dev/fuse`, `/dev/loop-control` and `/dev/net/tun` do exist. There is no GPU, no screen, no sound card, no keyboard or mouse device and no USB. `/dev/uinput` is absent, so the input forwarder cannot be tested directly here.

### 3.3 Network limits

**TESTED.**

```
t() { timeout 8 bash -c "exec 3<>/dev/tcp/$1/$2" 2>/dev/null && echo "$1:$2 CONNECTED" || echo "$1:$2 no connection (rc $?)"; }
t 1.1.1.1 53; t 1.1.1.1 443; t 8.8.8.8 53; t github.com 22; t github.com 443; t kernel.org 443; t example.com 80
```

Result: connections to ports 443 and 80 work; **ports 22 and 53 time out** (no SSH, no direct DNS to outside servers; name lookups still work through the system resolver, 1.4). All web traffic goes through an egress gateway that re-signs TLS (**TESTED**: `echo | openssl s_client -connect github.com:443 -servername github.com | grep -E 'issuer'` shows `Anthropic … Egress Gateway`). `ping` and `ip` are not installed.

Hosts the gateway refuses: `curl -sS -o /dev/null https://sunshinestream.io` fails with `CONNECT tunnel failed, response 502` (**TESTED**).

### 3.4 Other GitHub repositories are blocked until attached

**TESTED.**

```
curl -sS --max-time 20 https://github.com/LizardByte/Sunshine/releases
```

Result: `{"message":"GitHub access to this repository is not enabled for this session. Use add_repo to request access…"}`. The same gate applies to other repositories, public or not, until they are attached to the session. `list_repos` shows the account has `Isolati0n/HubOS` and `Isolati0n/city`.

### 3.5 The machine is ephemeral

**BELIEVED.** The system prompt says the session lives in a temporary container that is reclaimed after inactivity or when the session ends. Evidence: `uptime` showed the machine had been up about 6 minutes when the audit began, and `/home/user/HubOS` was a fresh clone. **Anything not committed and pushed to GitHub is lost.** Large downloads, built kernels and images must be rebuilt in each session.

### 3.6 Session and token limits

- The GitHub token expires: **TESTED**, see 1.18 (it expired at 2026-10-02 04:38:26 UTC when read).
- Disk space is a per-session allowance. **BELIEVED** from the system prompt. `df -h /` showed 30 GB available, and 12 GB used at one point in the audit.
- Memory is 16 GB and CPUs are 4 (1.1, **TESTED**).

### 3.7 This sandbox is in 192.0.2.0/24

**TESTED.**

```
hostname -I
cat /proc/net/route
```

Result: the machine's own address is `192.0.2.2`, with the gateway at `192.0.2.1` (`/proc/net/route` shows `eth0 000200C0 … 00FFFFFF`, which is 192.0.2.0/24). Connecting to the machine's own `192.0.2.2` worked (1.6). `examples/inventory.example.toml` uses 192.0.2.x addresses and calls them "never reachable". **In this sandbox they are not all unreachable. Tests must never rely on 192.0.2.x being unreachable.** Use `127.0.0.x` addresses and ports the operating system hands out, as the `hubd` tests do. The one probe of `192.0.2.41:5930` did time out (`DOWN (no answer within 2s)`), but that result is not a guarantee.

### 3.8 Tools missing by default

**TESTED** (1.3). No `qemu`, `flex`, `cpio`, `mksquashfs`, `debootstrap`, `ssh`, `sshd`, `ip`, `ping`, `lspci`, `lsusb`, `aarch64-linux-gnu-gcc`. They can be unpacked from the Ubuntu archive into a temporary directory (1.7, 1.8, 1.10).

### 3.9 Real machines and systemd

- The owner's real machines cannot be reached. **BELIEVED**: CLAUDE.md says to use fake nodes and virtual machines only; no real machine is reachable from here.
- systemd is never used. The `systemctl` binary is installed on this image, but systemd is not running (**TESTED**: `cat /proc/1/comm` prints `process_api`).
- Credentials for other cloud services exist in the environment. They must not be used (see `CLAUDE.md`).

---

## What to do about it

- Phase B images start very small, and software emulation is expected to be slow (`HUB-OS.md`, Phases).
- Everything worth keeping must be committed and pushed in the same session.
- Repeat the checks in this file before trusting any of it: this environment will change.
