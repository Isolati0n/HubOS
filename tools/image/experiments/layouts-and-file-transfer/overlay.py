#!/usr/bin/env python3
"""Test-only full-screen layer-shell OVERLAY (layer 3).  Logs every pointer event it gets.  Exits after SECONDS,
or (mode "oneshot") right after the first pointer.enter, printing where the pointer was.
Usage: overlay.py MODE SECONDS LOGFILE   (MODE: oneshot | stay)"""
import os, sys, mmap, time, select, struct
sys.path.insert(0, os.environ.get("PYLIB", "/nonexistent"))
from pywayland.client import Display
from pywayland.protocol.wayland import WlCompositor, WlShm, WlSeat
from pywayland.protocol.wlr_layer_shell_unstable_v1 import ZwlrLayerShellV1

mode, secs, logf = sys.argv[1], float(sys.argv[2]), sys.argv[3]
log = open(logf, "w", buffering=1)
t0 = time.time()
def L(*a): log.write("%.3f overlay %s\n" % (time.time() - t0, " ".join(str(x) for x in a)))
S = {}
d = Display(); d.connect()
reg = d.get_registry()
def g(reg, name, iface, ver):
    if iface == "wl_compositor": S["comp"] = reg.bind(name, WlCompositor, 4)
    elif iface == "wl_shm": S["shm"] = reg.bind(name, WlShm, 1)
    elif iface == "wl_seat": S["seat"] = reg.bind(name, WlSeat, 5)
    elif iface == "zwlr_layer_shell_v1": S["ls"] = reg.bind(name, ZwlrLayerShellV1, 4)
reg.dispatcher["global"] = g
d.roundtrip(); d.roundtrip()
surface = S["comp"].create_surface()
ls = S["ls"].get_layer_surface(surface, None, 3, "hubos-test-overlay")
ls.set_size(0, 0); ls.set_anchor(15); ls.set_exclusive_zone(-1); ls.set_keyboard_interactivity(0)
keep = []
done = [False]
def on_conf(ls, serial, w, h):
    ls.ack_configure(serial)
    w, h = w or 1280, h or 800
    stride = w * 4
    fd = os.memfd_create("b"); os.ftruncate(fd, stride * h)
    mm = mmap.mmap(fd, stride * h)
    mm.write(struct.pack("<I", 0x01000000) * (w * h))     # almost fully transparent
    pool = S["shm"].create_pool(fd, stride * h)
    buf = pool.create_buffer(0, w, h, stride, WlShm.format.argb8888.value)
    keep.append((mm, buf))
    surface.attach(buf, 0, 0); surface.damage(0, 0, w, h); surface.commit()
    L("mapped %dx%d" % (w, h))
ls.dispatcher["configure"] = on_conf
surface.commit()
pointer = S["seat"].get_pointer()
def on_enter(p, serial, surf, x, y):
    L("pointer.enter", "serial=%d" % serial, "x=%.0f y=%.0f" % (x, y))
    if mode == "oneshot": done[0] = True
pointer.dispatcher["enter"] = on_enter
pointer.dispatcher["leave"] = lambda p, serial, surf: L("pointer.leave")
pointer.dispatcher["motion"] = lambda p, t, x, y: L("pointer.motion", "x=%.0f y=%.0f" % (x, y))
pointer.dispatcher["button"] = lambda p, serial, t, b, s: L("pointer.button", b, s)
L("ready")
fd = d.get_fd(); end = time.time() + secs
while time.time() < end and not done[0]:
    d.dispatch(block=False); d.flush()
    rl, _, _ = select.select([fd], [], [], 0.05)
    if rl: d.dispatch(block=True)
ls.destroy(); d.flush(); L("destroyed layer surface; exit"); d.roundtrip(); d.disconnect()
