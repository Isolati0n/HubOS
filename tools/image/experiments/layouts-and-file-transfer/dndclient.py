#!/usr/bin/env python3
"""Test-only Wayland client (pywayland): one window that logs every pointer and data-device event it gets, and can
start a drag (role=source: on a left-button press it starts a wl_data_device drag offering text/uri-list) or accept
a drop (role=target).  Usage: dndclient.py ROLE APPID COLOR_HEX SECONDS LOGFILE"""
import os, sys, mmap, time, select, struct
sys.path.insert(0, os.environ.get("PYLIB", "/nonexistent"))
from pywayland.client import Display
from pywayland.protocol.wayland import WlCompositor, WlShm, WlSeat, WlDataDeviceManager
from pywayland.protocol.xdg_shell import XdgWmBase

role, appid, color, secs, logf = sys.argv[1], sys.argv[2], int(sys.argv[3], 16), float(sys.argv[4]), sys.argv[5]
log = open(logf, "w", buffering=1)
t0 = time.time()
def L(*a):
    log.write("%.3f %s %s\n" % (time.time() - t0, appid, " ".join(str(x) for x in a)))

S = {}
display = Display()
display.connect()
reg = display.get_registry()
def on_global(reg, name, iface, ver):
    if iface == "wl_compositor": S["comp"] = reg.bind(name, WlCompositor, min(ver, 4))
    elif iface == "wl_shm": S["shm"] = reg.bind(name, WlShm, 1)
    elif iface == "wl_seat": S["seat"] = reg.bind(name, WlSeat, min(ver, 5))
    elif iface == "wl_data_device_manager": S["ddm"] = reg.bind(name, WlDataDeviceManager, min(ver, 3))
    elif iface == "xdg_wm_base": S["wm"] = reg.bind(name, XdgWmBase, 1)
reg.dispatcher["global"] = on_global
display.roundtrip()
display.roundtrip()

surface = S["comp"].create_surface()
S["wm"].dispatcher["ping"] = lambda wm, serial: wm.pong(serial)
xs = S["wm"].get_xdg_surface(surface)
tl = xs.get_toplevel()
tl.set_app_id(appid); tl.set_title(appid)
size = [400, 300]
keep = []
def draw(w, h):
    stride = w * 4
    fd = os.memfd_create("buf"); os.ftruncate(fd, stride * h)
    mm = mmap.mmap(fd, stride * h)
    mm.write(struct.pack("<I", 0xFF000000 | color) * (w * h))
    pool = S["shm"].create_pool(fd, stride * h)
    buf = pool.create_buffer(0, w, h, stride, WlShm.format.argb8888.value)
    pool.destroy(); os.close(fd)
    keep.append((mm, buf))
    surface.attach(buf, 0, 0); surface.damage(0, 0, w, h); surface.commit()
def on_xs_configure(xs, serial):
    xs.ack_configure(serial); draw(*size)
xs.dispatcher["configure"] = on_xs_configure
def on_tl_configure(tl, w, h, states):
    if w > 0 and h > 0: size[0], size[1] = w, h
tl.dispatcher["configure"] = on_tl_configure
surface.commit()

pointer = S["seat"].get_pointer()
pointer.dispatcher["enter"] = lambda p, serial, surf, x, y: L("pointer.enter", "serial=%d" % serial, "x=%.0f y=%.0f" % (x, y))
pointer.dispatcher["leave"] = lambda p, serial, surf: L("pointer.leave", "serial=%d" % serial)
pointer.dispatcher["motion"] = lambda p, t, x, y: L("pointer.motion", "x=%.0f y=%.0f" % (x, y))
dev = S["ddm"].get_data_device(S["seat"])
state = {"serial": 0, "src": None, "offer": None}

def start_drag():
    src = S["ddm"].create_data_source()
    src.offer("text/uri-list")
    src.set_actions(WlDataDeviceManager.dnd_action.copy.value)
    def on_send(s, mime, fd):
        L("source.send", mime)
        os.write(fd, b"file:///srv/share/report.txt\r\n"); os.close(fd)
    src.dispatcher["send"] = on_send
    src.dispatcher["target"] = lambda s, mime: L("source.target", mime)
    src.dispatcher["action"] = lambda s, a: L("source.action", a)
    src.dispatcher["dnd_drop_performed"] = lambda s: L("source.dnd_drop_performed")
    src.dispatcher["dnd_finished"] = lambda s: L("source.dnd_finished")
    src.dispatcher["cancelled"] = lambda s: L("source.cancelled")
    state["src"] = src
    dev.start_drag(src, surface, None, state["serial"])
    L("start_drag", "serial=%d" % state["serial"])
def on_button(p, serial, t, button, st):
    state["serial"] = serial
    L("pointer.button", "serial=%d" % serial, "button=%d" % button, "state=%d" % st)
    if role in ("source", "src") and button == 272 and st == 1:
        start_drag()
pointer.dispatcher["button"] = on_button

def on_data_offer(d, offer):
    state["offer"] = offer
    offer.dispatcher["offer"] = lambda o, mime: L("offer.mime", mime)
dev.dispatcher["data_offer"] = on_data_offer
def on_dd_enter(d, serial, surf, x, y, offer):
    L("data_device.enter", "x=%.0f y=%.0f" % (x, y))
    if offer is not None and role == "target":
        offer.accept(serial, "text/uri-list")
        offer.set_actions(WlDataDeviceManager.dnd_action.copy.value, WlDataDeviceManager.dnd_action.copy.value)
dev.dispatcher["enter"] = on_dd_enter
dev.dispatcher["motion"] = lambda d, t, x, y: L("data_device.motion", "x=%.0f y=%.0f" % (x, y))
dev.dispatcher["leave"] = lambda d: L("data_device.leave")
def on_drop(d):
    L("data_device.drop")
    o = state["offer"]
    if o is None or role != "target": return
    r, w = os.pipe()
    o.receive("text/uri-list", w); os.close(w); display.flush()
    data = b""
    end = time.time() + 2
    while time.time() < end:
        display.dispatch(block=False) if hasattr(display, "dispatch") else None
        rl, _, _ = select.select([r], [], [], 0.2)
        if rl:
            c = os.read(r, 4096)
            if not c: break
            data += c
    L("dropped data:", repr(data))
    o.finish()
dev.dispatcher["drop"] = on_drop

L("ready role=%s" % role)
fd = display.get_fd()
end = time.time() + secs
while time.time() < end:
    display.dispatch(block=False)
    display.flush()
    rl, _, _ = select.select([fd], [], [], 0.1)
    if rl:
        display.dispatch(block=True)
L("exit")
display.disconnect()
