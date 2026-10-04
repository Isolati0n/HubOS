"""RFB spy relay (EXPERIMENT). Sits between a viewer and a VNC server on loopback and writes one line per
CLIENT-to-server message to a log file: message type, and for the clipboard messages the text or the flags.
It only parses the client side (SetPixelFormat, SetEncodings, FramebufferUpdateRequest, KeyEvent, PointerEvent,
ClientCutText, SetDesktopSize and the QEMU/extended messages are skipped by length where the length is known).
It understands security type None only (it forwards the handshake bytes untouched and does not follow
authentication; with a password it logs just 'raw bytes' counts).
usage: rfb-spy.py LISTEN_PORT SERVER_PORT LOGFILE"""
import socket, struct, sys, threading, time

lp, sp, logf = int(sys.argv[1]), int(sys.argv[2]), sys.argv[3]
log = open(logf, "a", buffering=1)
def say(*a): log.write("%.2f %s\n" % (time.time() % 1000, " ".join(map(str, a))))

ENC = {0xC0A1E5CE: "ExtendedClipboard", 0xFFFFFECC: "ExtDesktopSize(-308)", 0xFFFFFF21: "DesktopSize(-223)",
       0xFFFFFEC6: "LastRect(-314)", 0xFFFFFF11: "Cursor(-239)", 0xFFFFFFFF: "x"}

def parse_client(buf, state):
    # returns bytes consumed or 0 if need more
    while True:
        if state["hs"] < 3:       # version(12) / security choice(1) / ClientInit(1)
            need = [12, 1, 1][state["hs"]]
            if len(buf) < need: return
            say("handshake", ["version " + buf[:12].decode(errors="replace").strip(), "security-choice %d" % buf[0], "ClientInit"][state["hs"]])
            del buf[:need]; state["hs"] += 1; continue
        if not buf: return
        t = buf[0]
        if t == 0:   n = 20
        elif t == 2:
            if len(buf) < 4: return
            c = struct.unpack(">H", buf[2:4])[0]; n = 4 + 4 * c
            if len(buf) < n: return
            encs = struct.unpack(">%di" % c, buf[4:n]) if c else ()
            names = [ENC.get(e & 0xFFFFFFFF, str(e)) for e in encs]
            say("SetEncodings", "n=%d" % c, "ExtendedClipboard" if any(e & 0xFFFFFFFF == 0xC0A1E5CE for e in encs) else "no-extended-clipboard", ",".join(names[:14]))
        elif t == 3: n = 10
        elif t == 4: n = 8
        elif t == 5: n = 6
        elif t == 6:
            if len(buf) < 8: return
            ln = struct.unpack(">i", buf[4:8])[0]
            n = 8 + abs(ln)
            if len(buf) < n: return
            if ln >= 0: say("ClientCutText", repr(bytes(buf[8:n])[:80]))
            else:
                flags = struct.unpack(">I", buf[8:12])[0]
                say("ExtendedClipboard(client)", "flags=0x%08x" % flags, "len=%d" % abs(ln))
        elif t == 255:   # QEMU extended key event: 255, 0, down(2), keysym(4), keycode(4)
            n = 12
        elif t == 251: n = 8
        elif t == 250: n = 1
        else:
            say("UNKNOWN client message type", t); buf.clear(); return
        if len(buf) < n: return
        if t in (4,): say("KeyEvent", "down=%d" % buf[1], "keysym=0x%x" % struct.unpack(">I", buf[4:8])[0])
        elif t == 5: pass
        elif t == 255: say('QEMUExtKeyEvent', 'down=%d' % struct.unpack('>H', buf[2:4])[0], 'keysym=0x%x' % struct.unpack('>I', buf[4:8])[0])
        elif t == 3: pass
        elif t in (251,): say("SetDesktopSize-ish type", t)
        elif t == 0: say("SetPixelFormat")
        del buf[:n]

def pump(a, b, client_side):
    buf = bytearray(); state = {"hs": 0}
    try:
        while True:
            d = a.recv(65536)
            if not d: break
            if client_side:
                buf += d; parse_client(buf, state)
            b.sendall(d)
    except OSError: pass
    for s in (a, b):
        try: s.shutdown(socket.SHUT_RDWR)
        except OSError: pass

srv = socket.socket(); srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
srv.bind(("127.0.0.1", lp)); srv.listen(5)
while True:
    c, _ = srv.accept(); say("viewer connected")
    s = socket.create_connection(("127.0.0.1", sp))
    threading.Thread(target=pump, args=(c, s, True), daemon=True).start()
    threading.Thread(target=pump, args=(s, c, False), daemon=True).start()
