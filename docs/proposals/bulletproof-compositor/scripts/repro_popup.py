#!/usr/bin/env python3
"""Minimal reproductions for popup-parent bugs (Smithay find_popup_root_surface / get_popup_toplevel_coords).
usage: repro_popup.py SOCKET CASE      CASE = self | pair | nullparent
Raw Wayland wire protocol, no libraries.  Prints what the compositor did to this client."""
import socket, struct, sys, time
sock, case = sys.argv[1], sys.argv[2]
s = socket.socket(socket.AF_UNIX); s.connect(sock); s.settimeout(3)
nid = [1]
def new():
    nid[0] += 1; return nid[0]
def msg(obj, op, payload=b''): s.sendall(struct.pack('<II', obj, ((8 + len(payload)) << 16) | op) + payload)
def u(*v): return struct.pack('<' + 'I' * len(v), *v)
def st(b): b = b + b'\0'; return u(len(b)) + b + b'\0' * ((4 - len(b) % 4) % 4)
def recv_all(t=0.5):
    s.settimeout(t); out = b''
    try:
        while True:
            d = s.recv(65536)
            if not d: out += b'<EOF>'; break
            out += d
    except (socket.timeout, TimeoutError): pass
    return out
reg = new(); msg(1, 1, u(reg))
data = recv_all(1.0); g = {}; off = 0
while off + 8 <= len(data) and not data[off:].startswith(b'<EOF>'):
    o, w = struct.unpack('<II', data[off:off + 8]); size = w >> 16
    if o == reg and (w & 0xffff) == 0:
        name, ln = struct.unpack('<II', data[off + 8:off + 16]); iface = data[off + 16:off + 16 + ln - 1].decode(); pad = (ln + 3) & ~3
        ver = struct.unpack('<I', data[off + 16 + pad:off + 20 + pad])[0]; g[iface] = (name, ver)
    off += size
def bind(iface):
    name, ver = g[iface]; i = new(); msg(reg, 0, u(name) + st(iface.encode()) + u(min(ver, 4) if iface == 'xdg_wm_base' else 1, i)); return i
comp, wm = bind('wl_compositor'), bind('xdg_wm_base')
def surface_xdg():
    sf = new(); msg(comp, 0, u(sf)); x = new(); msg(wm, 2, u(x, sf)); return sf, x
def positioner():
    p = new(); msg(wm, 1, u(p)); msg(p, 1, u(10, 10)); msg(p, 2, u(0, 0, 5, 5)); return p
def popup(x, parent_x, pos):
    po = new(); msg(x, 2, u(po, parent_x, pos)); return po
t0 = time.time()
if case == 'self':
    # a popup whose parent is its own xdg_surface
    sf, x = surface_xdg(); popup(x, x, positioner())
elif case == 'pair':
    # A's parent is B's xdg_surface (B has no role yet); then B becomes a popup whose parent is A
    sfa, xa = surface_xdg(); sfb, xb = surface_xdg()
    popup(xa, xb, positioner()); popup(xb, xa, positioner())
elif case == 'nullparent':
    # A is a popup with no parent; B is a popup whose parent is A
    sfa, xa = surface_xdg(); sfb, xb = surface_xdg()
    popup(xa, 0, positioner()); popup(xb, xa, positioner())
msg(1, 0, u(new()))   # wl_display.sync
r = recv_all(4.0)
print(f'case={case}: after {time.time() - t0:.1f}s the compositor answered with {len(r)} bytes: {"EOF/closed" if b"<EOF>" in r else ("sync reply received" if len(r) else "NOTHING (hung or dead?)")}')
