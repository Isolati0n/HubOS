#!/usr/bin/env python3
"""session.py: drive the desktop experiment VM (EXPERIMENT; docs/proposals/phase-b-desktop.md).

Starts QEMU (software emulation, UEFI, virtio-vga, USB keyboard and tablet, serial on a pipe, HMP monitor on a unix
socket) on a disk made by build-disk.sh, and offers: waiting for serial lines, typing into the serial shell,
monitor commands (screendump, sendkey, mouse_move, mouse_button) and a PNG saver. Run it with python3 -i, or use
the scenarios in scenario.py.  Usage: WORK=... session.py DISKDIR OUTDIR
"""
import os, re, socket, struct, subprocess, sys, threading, time, zlib

class Session:
    def __init__(self, work, diskdir, outdir, mem=2048, res="1024x640", extra=None, pointer="usb-tablet"):
        self.work, self.disk, self.out = work, diskdir, outdir
        os.makedirs(outdir, exist_ok=True)
        T = f"{work}/tools/root"
        self.env = dict(os.environ, LD_LIBRARY_PATH=f"{T}/usr/lib/x86_64-linux-gnu:{T}/lib/x86_64-linux-gnu:{T}/usr/lib",
                        QEMU_MODULE_DIR=f"{T}/usr/lib/x86_64-linux-gnu/qemu")
        xres, yres = res.split("x")
        self.mon_path = f"{outdir}/mon.sock"
        if os.path.exists(self.mon_path): os.remove(self.mon_path)
        args = [f"{T}/usr/bin/qemu-system-x86_64", "-L", f"{T}/usr/share/qemu", "-L", f"{T}/usr/share/seabios",
                "-machine", "q35,smm=off", "-accel", "tcg", "-smp", "2", "-m", str(mem), "-display", "none",
                "-drive", f"if=pflash,format=raw,unit=0,readonly=on,file={T}/usr/share/OVMF/OVMF_CODE_4M.fd",
                "-drive", f"if=pflash,format=raw,unit=1,file={diskdir}/vars.fd",
                "-drive", f"file={diskdir}/disk.img,if=none,id=d0,format=raw", "-device", "virtio-blk-pci,drive=d0",
                "-netdev", "user,id=n0", "-device", "virtio-net-pci,netdev=n0,romfile=",
                "-device", f"virtio-vga,xres={xres},yres={yres}",
                "-device", "qemu-xhci,id=xhci", "-device", "usb-kbd,bus=xhci.0", "-device", f"{pointer},bus=xhci.0",
                "-serial", "stdio", "-monitor", f"unix:{self.mon_path},server,nowait"] + (extra or [])
        self.t0 = time.time()
        self.p = subprocess.Popen(args, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, env=self.env)
        self.buf = ""; self.lock = threading.Lock()
        self.log = open(f"{outdir}/serial.log", "w")
        threading.Thread(target=self._read, daemon=True).start()
        self.mon = None
        for _ in range(100):
            try:
                self.mon = socket.socket(socket.AF_UNIX); self.mon.connect(self.mon_path); break
            except OSError: time.sleep(0.1)
        self.mon.settimeout(5); self._mon_read()

    def _read(self):
        while True:
            b = self.p.stdout.read1(4096)
            if not b: return
            s = b.decode("utf-8", "replace").replace("\r", "")
            with self.lock: self.buf += s
            self.log.write(s); self.log.flush()

    def mark(self):
        with self.lock: return len(self.buf)

    def text(self, frm=0):
        with self.lock: return self.buf[frm:]

    def wait(self, pat, timeout=120, frm=0):
        rx = re.compile(pat); end = time.time() + timeout
        while time.time() < end:
            m = rx.search(self.text(frm))
            if m: return frm + m.end()
            if self.p.poll() is not None: return -1
            time.sleep(0.2)
        return -1

    def sh(self, cmd, timeout=60):
        tag = f"__T{int(time.time()*1000) % 10**9}__"; m = self.mark()
        self.p.stdin.write(f"{cmd}\necho {tag}$?\n".encode()); self.p.stdin.flush()
        e = self.wait(re.escape(tag) + r"\d+", timeout, m)
        t = self.text(m)
        return t if e < 0 else t[: e - m]

    def _mon_read(self):
        data = b""
        try:
            while True:
                c = self.mon.recv(65536)
                if not c: break
                data += c
                if data.rstrip().endswith(b"(qemu)"): break
        except socket.timeout: pass
        return data.decode("utf-8", "replace")

    def monitor(self, cmd):
        self.mon.sendall((cmd + "\n").encode()); return self._mon_read()

    def screendump(self, name):
        """Saves $OUT/name.png (QEMU's own PNG writer; falls back to PPM + zlib)."""
        path = f"{self.out}/{name}.png"
        r = self.monitor(f"screendump {path} -f png")
        if not os.path.exists(path) or os.path.getsize(path) == 0:
            ppm = f"{self.out}/{name}.ppm"; self.monitor(f"screendump {ppm}"); ppm_to_png(ppm, path); os.remove(ppm)
        return path

    def abs_move(self, x, y):
        """Absolute pointer position 0..32767 through QMP input-send-event (no device given). Needs the QEMU option
        -qmp unix:OUTDIR/qmp.sock,server,nowait (steps.py adds it for the tablet run). HMP mouse_move cannot do this: it
        sends relative events to the PS/2 mouse unless that is switched off, and then the tablet still gets nothing."""
        import json
        if getattr(self, "q", None) is None:
            self.q = socket.socket(socket.AF_UNIX); self.q.settimeout(10); self.q.connect(f"{self.out}/qmp.sock")
            self.q.recv(4096); self.q.sendall(b'{"execute":"qmp_capabilities"}\n'); self.q.recv(4096)
        ev = [{"type": "abs", "data": {"axis": "x", "value": x}}, {"type": "abs", "data": {"axis": "y", "value": y}}]
        self.q.sendall(json.dumps({"execute": "input-send-event", "arguments": {"events": ev}}).encode() + b"\n"); return self.q.recv(4096)
    def key(self, k): self.monitor(f"sendkey {k}")
    def move(self, x, y): self.monitor(f"mouse_move {x} {y}")      # absolute 0..32767 with usb-tablet
    def button(self, mask): self.monitor(f"mouse_button {mask}")   # 1 = left, 0 = release
    def type_text(self, s):
        """Types through the monitor's sendkey: a 20 ms hold and 0.5 s between keys (the defaults repeated and dropped keys)."""
        names = {" ": "spc", "\n": "ret", ".": "dot", "-": "minus", "/": "slash", "=": "equal", ":": "shift-semicolon", "_": "shift-minus", "$": "shift-4"}
        for ch in s:
            k = names.get(ch, ch)
            if ch.isupper(): k = "shift-" + ch.lower()
            self.monitor(f"sendkey {k} 20"); time.sleep(0.5)

    def close(self):
        try: self.monitor("quit")
        except Exception: pass
        try: self.p.kill()
        except Exception: pass

def png_stats(path):
    """(width, height, bytes, fraction of non-black samples) of a PNG written by QEMU (8-bit RGB/RGBA)."""
    d = open(path, "rb").read(); w, h = struct.unpack(">II", d[16:24]); ct = d[25]
    idat = b""; i = 8
    while i < len(d):
        n = struct.unpack(">I", d[i:i+4])[0]; typ = d[i+4:i+8]
        if typ == b"IDAT": idat += d[i+8:i+8+n]
        i += 12 + n
    raw = zlib.decompress(idat); bpp = {2: 3, 6: 4}.get(ct, 3); stride = 1 + w * bpp
    nz = tot = 0
    for y in range(0, h, 16):
        row = raw[y*stride+1: y*stride+stride]       # filter bytes ignored: only a rough check for "not all black"
        for x in range(0, len(row) - bpp, bpp * 8):
            tot += 1; nz += any(row[x:x+3])
    return w, h, len(d), (nz / tot if tot else 0)

def png_rows(path):
    """(width, height, bytes per pixel, rows) of a PNG written by QEMU (8-bit RGB/RGBA, all filter types)."""
    d = open(path, "rb").read(); w, h = struct.unpack(">II", d[16:24]); ct = d[25]; bpp = {2: 3, 6: 4}[ct]
    idat = b""; i = 8
    while i < len(d):
        n = struct.unpack(">I", d[i:i+4])[0]
        if d[i+4:i+8] == b"IDAT": idat += d[i+8:i+8+n]
        i += 12 + n
    raw = zlib.decompress(idat); st = 1 + w * bpp; prev = bytearray(w * bpp); rows = []
    for y in range(h):
        f = raw[y*st]; r = bytearray(raw[y*st+1:(y+1)*st])
        for x in range(len(r)):
            a = r[x-bpp] if x >= bpp else 0; b = prev[x]; c = prev[x-bpp] if x >= bpp else 0
            if f == 1: r[x] = (r[x] + a) & 255
            elif f == 2: r[x] = (r[x] + b) & 255
            elif f == 3: r[x] = (r[x] + ((a + b) >> 1)) & 255
            elif f == 4:
                p_ = a + b - c; pa, pb, pc = abs(p_-a), abs(p_-b), abs(p_-c)
                r[x] = (r[x] + (a if pa <= pb and pa <= pc else b if pb <= pc else c)) & 255
        rows.append(r); prev = r
    return w, h, bpp, rows

def diff_bbox(a, b, y0=0):
    """Bounding box (x0, y0, x1, y1) of the pixels that differ between two screenshots (rows from y0 down), or None."""
    w, h, bpp, ra = png_rows(a); _, _, _, rb = png_rows(b); xs = []; ys = []
    for y in range(y0, h):
        if ra[y] != rb[y]:
            for x in range(w):
                if ra[y][x*bpp:x*bpp+3] != rb[y][x*bpp:x*bpp+3]: xs.append(x); ys.append(y)
    return (min(xs), min(ys), max(xs), max(ys)) if xs else None

def ppm_to_png(ppm, png):
    d = open(ppm, "rb").read(); parts = d.split(b"\n", 3); w, h = map(int, parts[1].split()); px = parts[3]
    raw = b"".join(b"\x00" + px[y*w*3:(y+1)*w*3] for y in range(h))
    def chunk(t, c): return struct.pack(">I", len(c)) + t + c + struct.pack(">I", zlib.crc32(t + c) & 0xffffffff)
    open(png, "wb").write(b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 2, 0, 0, 0)) + chunk(b"IDAT", zlib.compress(raw, 9)) + chunk(b"IEND", b""))

if __name__ == "__main__":
    work, disk, out = sys.argv[1:4]
    s = Session(work, disk, out)
    print("QEMU started; use s.sh(), s.screendump(), s.key() ...")
    import code; code.interact(local={"s": s, "png_stats": png_stats})
