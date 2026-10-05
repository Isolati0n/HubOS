#!/usr/bin/env python3
"""Test-only pointer injector for the headless sway parent: zwlr_virtual_pointer_v1.  Reads lines from a FIFO:
   abs X Y | press | release | quit      (press/release = left button).  Output extent 1280x800."""
import os, sys, time
sys.path.insert(0, os.environ.get("PYLIB", "/nonexistent"))
from pywayland.client import Display
from pywayland.protocol.wayland import WlSeat
from pywayland.protocol.wlr_virtual_pointer_unstable_v1 import ZwlrVirtualPointerManagerV1

fifo = sys.argv[1]
d = Display(); d.connect()
reg = d.get_registry()
S = {}
def g(reg, name, iface, ver):
    if iface == "wl_seat": S["seat"] = reg.bind(name, WlSeat, 1)
    elif iface == "zwlr_virtual_pointer_manager_v1": S["mgr"] = reg.bind(name, ZwlrVirtualPointerManagerV1, 1)
reg.dispatcher["global"] = g
d.roundtrip(); d.roundtrip()
vp = S["mgr"].create_virtual_pointer(S["seat"])
d.roundtrip()
def now(): return int(time.monotonic() * 1000) & 0xFFFFFFFF
print("vptr ready", flush=True)
while True:
    with open(fifo) as f:
        for line in f:
            p = line.split()
            if not p: continue
            if p[0] == "abs":
                vp.motion_absolute(now(), int(p[1]), int(p[2]), 1280, 800); vp.frame()
            elif p[0] == "press":
                vp.button(now(), 272, 1); vp.frame()
            elif p[0] == "release":
                vp.button(now(), 272, 0); vp.frame()
            elif p[0] == "quit":
                d.flush(); sys.exit(0)
            d.flush()
