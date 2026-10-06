#!/usr/bin/env python3
"""Minimal reproduction: wl_subsurface.place_above(its own surface) panics Smithay's PrivateSurfaceData::reorder
(tree.rs: 'insertion index (is 2) should be <= len (is 1)').  usage: repro_subsurface.py SOCKET"""
import socket, struct, sys, time
s = socket.socket(socket.AF_UNIX); s.connect(sys.argv[1]); s.settimeout(3)
n = [1]
def new(): n[0] += 1; return n[0]
def msg(o, op, p=b''): s.sendall(struct.pack('<II', o, ((8 + len(p)) << 16) | op) + p)
def u(*v): return struct.pack('<' + 'I' * len(v), *v)
def st(b): b = b + b'\0'; return u(len(b)) + b + b'\0' * ((4 - len(b) % 4) % 4)
reg = new(); msg(1, 1, u(reg)); time.sleep(0.5); data = s.recv(65536); g = {}; off = 0
while off + 8 <= len(data):
    o, w = struct.unpack('<II', data[off:off + 8]); sz = w >> 16
    if o == reg and (w & 0xffff) == 0:
        name, ln = struct.unpack('<II', data[off + 8:off + 16]); iface = data[off + 16:off + 16 + ln - 1].decode(); g[iface] = name
    off += sz
def bind(i):
    x = new(); msg(reg, 0, u(g[i]) + st(i.encode()) + u(1, x)); return x
comp, sub = bind('wl_compositor'), bind('wl_subcompositor')
parent, child = new(), new(); msg(comp, 0, u(parent)); msg(comp, 0, u(child))
ss = new(); msg(sub, 1, u(ss, child, parent))      # wl_subcompositor.get_subsurface(new_id, surface, parent)
msg(ss, 2, u(child))                                # wl_subsurface.place_above(sibling = the subsurface's own wl_surface)
msg(1, 0, u(new()))                                 # wl_display.sync
try: r = s.recv(4096); print('answer from compositor:', len(r), 'bytes; ' + ('error event' if len(r) > 12 and struct.unpack('<I', r[:4])[0] == 1 else 'sync/other'))
except Exception as e: print('no answer:', type(e).__name__, '(compositor dead or hung)')
