#!/usr/bin/env python3
"""sizes.py PATH...  (files or folders)

Packs the given files into a cpio "newc" archive in memory (the format of a Linux initramfs) and prints
the sizes: files, raw bytes of file contents, archive bytes, and the archive compressed with gzip -9 (the
kernel of this project is built with CONFIG_KERNEL_GZIP=y, image/kernel/*.frag). Symbolic links are stored as links.
Standard library only. The compressed number is an ESTIMATE of what an initramfs would add to the kernel image:
the real kernel compresses the whole image (kernel and initramfs together) in one stream, and nothing here was built."""
import os, sys, zlib


def newc(entries):
    out = bytearray()
    ino = 1
    for name, mode, data in entries:
        nm = name.encode() + b"\0"
        out += b"070701" + b"".join(b"%08x" % v for v in (ino, mode, 0, 0, 1, 0, len(data), 0, 0, 0, 0, len(nm), 0))
        out += nm + b"\0" * ((4 - (110 + len(nm)) % 4) % 4)
        out += data + b"\0" * ((4 - len(data) % 4) % 4)
        ino += 1
    tail = b"TRAILER!!!\0"
    out += b"070701" + b"".join(b"%08x" % v for v in (0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, len(tail), 0))
    out += tail + b"\0" * ((4 - (110 + len(tail)) % 4) % 4)
    return bytes(out)


def collect(paths):
    entries, raw, n = [], 0, 0
    for top in paths:
        top = top.rstrip("/")
        walk = [(top, os.path.basename(top))] if not os.path.isdir(top) else []
        if os.path.isdir(top):
            for d, dirs, files in os.walk(top):
                for f in files:
                    p = os.path.join(d, f)
                    walk.append((p, os.path.join(os.path.basename(top), os.path.relpath(p, top))))
        for p, arc in walk:
            if os.path.islink(p):
                entries.append((arc, 0o120777, os.readlink(p).encode()))
            else:
                data = open(p, "rb").read()
                entries.append((arc, 0o100755 if os.access(p, os.X_OK) else 0o100644, data))
                raw += len(data)
            n += 1
    return entries, raw, n


entries, raw, n = collect(sys.argv[1:])
arc = newc(entries)
gz = len(zlib.compress(arc, 9))
print(f"files {n}  contents {raw:,} bytes  cpio archive {len(arc):,} bytes  gzip -9 of the archive {gz:,} bytes")
