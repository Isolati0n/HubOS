#!/usr/bin/env python3
"""Throwaway experiment for docs/proposals/clipboard.md. NOT the session layer, NOT a design.

A fake RFB node (security None, Raw only, RFB Extended Clipboard, text format only, the message flow that
neatvnc v1.0.3 uses: server notify -> client request -> server provide; client notify -> server request ->
client provide) and a minimal stand-in for a session worker's clipboard part. It runs on loopback, uses
only the Python standard library, and finishes in a few seconds. Run:   python3 -I fakerfb.py
It checks the clipboard message layout against rfbproto.rst (Extended Clipboard) and neatvnc's source; it
does not prove that real wayvnc behaves the same (wayvnc was not run).
"""
import socket, struct, threading, time, zlib, hashlib, sys

CAPS, REQUEST, PEEK, NOTIFY, PROVIDE = 1 << 24, 1 << 25, 1 << 26, 1 << 27, 1 << 28
TEXT = 1
ENC_EXT_CLIP = -1063131698


def recvn(s, n):
    b = b""
    while len(b) < n:
        c = s.recv(n - len(b))
        if not c:
            raise EOFError("closed")
        b += c
    return b


def ext_msg(mtype, flags, payload=b""):
    return bytes([mtype, 0, 0, 0]) + struct.pack(">i", -(4 + len(payload))) + struct.pack(">I", flags) + payload


def provide_payload(text):
    b = text.encode("utf-8")
    return zlib.compress(struct.pack(">I", len(b) + 1) + b + b"\0")


def parse_provide(zl):
    d = zlib.decompress(zl)
    size = struct.unpack(">I", d[:4])[0]
    assert d[4 + size - 1] == 0, "text must end with NUL"
    return d[4:4 + size - 1].decode("utf-8").replace("\r\n", "\n").replace("\r", "\n")


def read_ext(s):
    """After the 4 header bytes of a cut-text message: returns (flags, payload) or ('text', bytes)."""
    n = struct.unpack(">i", recvn(s, 4))[0]
    if n >= 0:
        return None, recvn(s, n)
    body = recvn(s, -n)
    return struct.unpack(">I", body[:4])[0], body[4:]


class FakeNode:
    """Pretends to be wayvnc + the node's clipboard. mode: normal | silent (never answers requests) | cut (closes mid-message)."""

    def __init__(self, mode="normal"):
        self.mode = mode
        self.clip = None          # the node's clipboard text
        self.got = []             # texts the hub pushed
        self.lsock = socket.socket(); self.lsock.bind(("127.0.0.1", 0)); self.lsock.listen(4)
        self.port = self.lsock.getsockname()[1]
        self.conn = None
        self.send_lock = threading.Lock()
        self.echo_after_push = False  # emulate a second wayvnc client re-announcing a pushed text
        threading.Thread(target=self.serve, daemon=True).start()

    def send(self, b):
        with self.send_lock:
            self.conn.sendall(b)

    def node_copy(self, text):
        self.clip = text
        self.send(ext_msg(3, NOTIFY | TEXT))

    def serve(self):
        s, _ = self.lsock.accept(); self.conn = s
        s.sendall(b"RFB 003.008\n"); recvn(s, 12)
        s.sendall(bytes([1, 1])); recvn(s, 1); s.sendall(struct.pack(">I", 0))
        recvn(s, 1)
        pf = struct.pack(">BBBBHHHBBB3x", 32, 24, 0, 1, 255, 255, 255, 16, 8, 0, )
        name = b"fake-node"
        s.sendall(struct.pack(">HH", 64, 64) + pf + struct.pack(">I", len(name)) + name)
        try:
            while True:
                t = recvn(s, 1)[0]
                if t == 0: recvn(s, 19)
                elif t == 2:
                    _, n = struct.unpack(">BH", recvn(s, 3)); encs = struct.unpack(">%di" % n, recvn(s, 4 * n))
                    if ENC_EXT_CLIP in encs:
                        self.send(ext_msg(3, CAPS | TEXT | REQUEST | PEEK | NOTIFY | PROVIDE, struct.pack(">I", 0)))
                elif t == 3: recvn(s, 9)
                elif t == 4: recvn(s, 7)
                elif t == 5: recvn(s, 5)
                elif t == 6:
                    recvn(s, 3); flags, pl = read_ext(s)
                    if self.mode == "silent": continue
                    if flags is None: continue
                    if flags & CAPS: continue
                    if flags & REQUEST and self.clip is not None:
                        if self.mode == "cut":
                            s.sendall(ext_msg(3, PROVIDE | TEXT, provide_payload(self.clip))[:20]); s.close(); return
                        self.send(ext_msg(3, PROVIDE | TEXT, provide_payload(self.clip)))
                    elif flags & NOTIFY and flags & TEXT:
                        self.send(ext_msg(3, REQUEST | TEXT))
                    elif flags & PROVIDE:
                        txt = parse_provide(pl); self.got.append(txt); self.clip = txt
                        if self.echo_after_push:
                            self.send(ext_msg(3, NOTIFY | TEXT))
        except (EOFError, OSError):
            pass


class Worker:
    """The clipboard half of a session worker: records every node copy, can push, guards against echo,
    and never waits without a deadline."""

    def __init__(self, port, request_timeout=1.0, echo_window=3.0):
        self.rt, self.echo_window = request_timeout, echo_window
        self.history = []          # dicts: text, sha, t, note
        self.pushed = {}           # sha -> time of our push
        self.last_ok = time.time(); self.stale = False
        self.lock = threading.Lock(); self.pending = None
        self.s = socket.create_connection(("127.0.0.1", port), timeout=2)
        self.s.settimeout(0.2)
        s = self.s
        recvn(s, 12); s.sendall(b"RFB 003.008\n")
        n = recvn(s, 1)[0]; recvn(s, n); s.sendall(bytes([1]))
        assert struct.unpack(">I", recvn(s, 4))[0] == 0
        s.sendall(bytes([1]))
        recvn(s, 4 + 16); nl = struct.unpack(">I", recvn(s, 4))[0]; self.name = recvn(s, nl).decode()
        s.sendall(struct.pack(">BxH", 2, 2) + struct.pack(">ii", 0, ENC_EXT_CLIP))
        self.alive = True
        threading.Thread(target=self.loop, daemon=True).start()
        threading.Thread(target=self.watchdog, daemon=True).start()

    def send(self, b):
        with self.lock:
            self.s.sendall(b)

    def push(self, text):
        self.pushed[hashlib.sha256(text.encode()).hexdigest()] = time.time()
        self.pending_push = text
        self.send(struct.pack(">Bxxxi", 6, -4) + struct.pack(">I", NOTIFY | TEXT))

    def watchdog(self):
        while self.alive:
            time.sleep(0.1)
            if self.pending and time.time() > self.pending:
                self.history.append(dict(text=None, sha=None, t=time.time(), note="unavailable: node did not answer the request in %.1f s" % self.rt))
                self.pending = None; self.stale = True

    def loop(self):
        s = self.s
        while self.alive:
            try:
                h = s.recv(1)
                if not h: break
            except socket.timeout:
                continue
            except OSError:
                break
            try:
                s.settimeout(2)
                if h[0] != 3:
                    break
                recvn(s, 3); flags, pl = read_ext(s)
                self.last_ok = time.time(); self.stale = False
                if flags is None: continue
                if flags & CAPS:
                    self.send(struct.pack(">Bxxxi", 6, -8) + struct.pack(">II", CAPS | TEXT | REQUEST | NOTIFY | PROVIDE, 0)); continue
                if flags & NOTIFY and flags & TEXT:
                    self.pending = time.time() + self.rt
                    self.send(struct.pack(">Bxxxi", 6, -4) + struct.pack(">I", REQUEST | TEXT))
                elif flags & REQUEST:
                    self.send(bytes([6, 0, 0, 0]) + struct.pack(">i", -(4 + len(zp := provide_payload(self.pending_push)))) + struct.pack(">I", PROVIDE | TEXT) + zp)
                elif flags & PROVIDE:
                    self.pending = None
                    txt = parse_provide(pl); sha = hashlib.sha256(txt.encode()).hexdigest(); now = time.time()
                    note = ""
                    if sha in self.pushed and now - self.pushed[sha] < self.echo_window:
                        note = "echo of our own push: not a new history entry"
                    self.history.append(dict(text=txt, sha=sha, t=now, note=note))
                s.settimeout(0.2)
            except (EOFError, OSError, zlib.error, AssertionError) as e:
                self.history.append(dict(text=None, sha=None, t=time.time(), note="connection lost or bad message: %r" % (e,)))
                self.stale = True; break
        self.alive = False


def check(name, ok, extra=""):
    print(("PASS " if ok else "FAIL ") + name + (("  " + extra) if extra else ""))
    return ok


def wait(cond, secs=3.0):
    t = time.time()
    while time.time() - t < secs:
        if cond(): return True
        time.sleep(0.02)
    return False


def main():
    ok = True
    # 1 node copy arrives (UTF-8, CRLF turned into LF)
    n = FakeNode(); w = Worker(n.port); time.sleep(0.2)
    n.node_copy("café ✓ 日本\r\nline2")
    ok &= check("1 node copy reaches the worker, UTF-8 kept, CRLF -> LF", wait(lambda: len(w.history) == 1) and w.history[0]["text"] == "café ✓ 日本\nline2", repr(w.history[:1]))
    # 2 hub push reaches the node
    w.push("from the hub ✓")
    ok &= check("2 hub push reaches the node", wait(lambda: n.got == ["from the hub ✓"]), repr(n.got))
    # 3 echo: a second wayvnc client re-announces the pushed text; the worker must not log a new entry
    n.echo_after_push = True
    before = len(w.history)
    w.push("pushed twice-visible")
    wait(lambda: len(w.history) > before)
    last = w.history[-1] if len(w.history) > before else {}
    ok &= check("3 echo of a push is marked, not stored as a new copy", "echo" in last.get("note", ""), repr(last))
    w.alive = False
    # 4 a 9 MB text (under neatvnc's 10,000,000 byte wall)
    n = FakeNode(); w = Worker(n.port, request_timeout=5); time.sleep(0.2)
    big = ("x" * 99 + "\n") * 90000
    t = time.time(); n.node_copy(big)
    ok &= check("4 9,000,000-byte text received", wait(lambda: len(w.history) == 1, 10) and w.history[0]["text"] == big, "%.2f s" % (time.time() - t))
    w.alive = False
    # 5 node never answers a request: deadline, marked unavailable, stale
    n = FakeNode("silent"); w = Worker(n.port, request_timeout=0.5); time.sleep(0.2)
    n.node_copy("never delivered")
    ok &= check("5 silent node: request times out, entry marked unavailable, session marked stale",
                wait(lambda: len(w.history) == 1, 3) and w.history[0]["text"] is None and w.stale, repr(w.history[:1]))
    w.alive = False
    # 6 connection cut in the middle of a message
    n = FakeNode("cut"); w = Worker(n.port); time.sleep(0.2)
    n.node_copy("cut me")
    ok &= check("6 connection cut mid-message: worker notices, marks stale, does not hang", wait(lambda: any("lost" in h["note"] for h in w.history), 4) and w.stale, repr(w.history[:2]))
    print("ALL PASS" if ok else "SOME FAILED")
    sys.exit(0 if ok else 1)


main()
