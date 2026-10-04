#!/usr/bin/env python3
"""recovery-list.py BASE TOOLS BUSYBOX RELEASE_FILE [PUBKEY ...] > recovery.list

Writes the initramfs list (gen_init_cpio format) of the SEPARATE recovery kernel: static busybox (all applets as
links), the recovery init, hubos-ctl, efibootmgr, signify, e2fsprogs, findfs/blkid, the udhcpc script and every
shared library they need, taken from the base root (BASE) and the unpacked tools (TOOLS). Nothing of a slot's root
is used at run time; this is a copy taken at build time. Run from the repository root."""
import os, subprocess, sys

base, tools, busybox, release = sys.argv[1:5]
update_pubs = sys.argv[5:]   # the keyring: any number of public key files
REPO = os.getcwd()
LIBDIRS = [tools + p for p in ("/usr/lib/x86_64-linux-gnu", "/lib/x86_64-linux-gnu")] + \
          [base + p for p in ("/usr/lib/x86_64-linux-gnu", "/lib/x86_64-linux-gnu")]
out, seen_dirs, seen_files = [], set(), set()

def d(path):
    parts = path.strip("/").split("/")
    for i in range(1, len(parts) + 1):
        q = "/" + "/".join(parts[:i])
        if q not in seen_dirs:
            seen_dirs.add(q); out.append(f"dir {q} 755 0 0")

def f(dst, src, mode="755"):
    if dst in seen_files: return
    d(os.path.dirname(dst)); seen_files.add(dst); out.append(f"file {dst} {src} {mode} 0 0")

def link(dst, target):
    if dst in seen_files: return
    d(os.path.dirname(dst)); seen_files.add(dst); out.append(f"slink {dst} {target} 777 0 0")

def needed(path):
    r = subprocess.run(["readelf", "-d", path], capture_output=True, text=True).stdout
    return [l.split("[")[1].split("]")[0] for l in r.splitlines() if "(NEEDED)" in l]

def find_lib(name):
    for dd in LIBDIRS:
        p = os.path.join(dd, name)
        if os.path.exists(p): return os.path.realpath(p) if not os.path.islink(p) else resolve(p)
    sys.exit("recovery-list.py: library not found: " + name)

def resolve(p):
    # follow symlinks without leaving the tree they live in (links in the base root are relative)
    return os.path.realpath(p)

done = set()
def add_binary(dst, src):
    f(dst, src)
    stack = needed(src)
    while stack:
        n = stack.pop()
        if n in done: continue
        done.add(n)
        real = find_lib(n)
        f("/usr/lib/x86_64-linux-gnu/" + n, real, "755")
        stack += needed(real)

# C library loader
ld = None
for dd in LIBDIRS:
    p = os.path.join(dd, "ld-linux-x86-64.so.2")
    if os.path.exists(p): ld = os.path.realpath(p); break
f("/lib64/ld-linux-x86-64.so.2", ld)

f("/bin/busybox", busybox)
real = {"findfs": base + "/usr/sbin/findfs", "blkid": base + "/usr/sbin/blkid", "e2fsck": base + "/usr/sbin/e2fsck",
        "mke2fs": base + "/usr/sbin/mke2fs", "tune2fs": base + "/usr/sbin/tune2fs", "resize2fs": base + "/usr/sbin/resize2fs",
        "dumpe2fs": base + "/usr/sbin/dumpe2fs", "debugfs": base + "/usr/sbin/debugfs", "chattr": base + "/usr/bin/chattr"}
for n, src in real.items(): add_binary("/sbin/" + n, src)
add_binary("/usr/sbin/efibootmgr", tools + "/bin/efibootmgr")
add_binary("/usr/bin/signify-openbsd", tools + "/bin/signify-openbsd")
for a in subprocess.run([busybox, "--list"], capture_output=True, text=True).stdout.split():
    if a in real or a == "busybox": continue
    link("/bin/" + a, "busybox")
link("/bin/sh", "busybox")
f("/init", os.path.join(REPO, "image/stage0/recovery-init"))
f("/usr/sbin/hubos-ctl", os.path.join(REPO, "image/rootfs/usr/sbin/hubos-ctl"))
f("/usr/lib/hubos/udhcpc.script", os.path.join(REPO, "image/rootfs/usr/lib/hubos/udhcpc.script"))
f("/etc/hubos-release", release, "644")
import hashlib
for k in update_pubs:
    f("/etc/hubos/keys/" + hashlib.sha256(open(k, "rb").read()).hexdigest()[:16] + ".pub", k, "644")
f("/etc/recovery.rc", os.path.join(REPO, "image/stage0/recovery.rc"), "644")
for dd in ("/dev", "/proc", "/sys", "/run", "/tmp", "/config", "/data", "/boot/efi", "/etc/hubos"): d(dd)
out.append("nod /dev/console 600 0 0 c 5 1")
out.append("nod /dev/null 666 0 0 c 1 3")
print("\n".join(out))
