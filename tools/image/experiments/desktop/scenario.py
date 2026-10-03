#!/usr/bin/env python3
"""scenario.py MODE WORK DISKDIR OUTDIR: scripted runs of the desktop experiment (docs/proposals/phase-b-desktop.md).
MODE: noworkaround  boot, report whether libinput sees the input devices without the udev database workaround
      full          boot with the workaround; first frame, terminal, keys, mouse, driftwm msg state, screenshots
      bar           like full, then start Waybar (needs dbus-daemon) and hubd and take a screenshot
"""
import re, sys, time, os
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from session import *

ENV = "XDG_RUNTIME_DIR=/run/dw WAYLAND_DISPLAY=wayland-1"
mode, work, disk, out = sys.argv[1:5]
s = Session(work, disk, out, pointer=('usb-tablet' if mode == 'noworkaround' else 'usb-mouse'))
t0 = s.t0
def at(): return round(time.time() - t0, 1)
def note(*a): print(f"[{at():6.1f}s]", *a, flush=True)

e = s.wait(r"HUBOS: handing over to s6-svscan", 200)
note("handover line" if e >= 0 else "NO HANDOVER")
# time to first frame: driftwm's modeset line, then the first screendump that is not black (its canvas has a dot grid
# and a pointer). The probes before that are printed so the criterion can be judged.
first = None; modeset = None; probes = []
while time.time() - t0 < 200:
    if modeset is None and re.search(r"Setting new mode", s.text()): modeset = at()
    try:
        p = s.screendump("probe"); w, h, size, frac = png_stats(p)
    except Exception as ex:
        time.sleep(1); continue
    probes.append((at(), size, round(frac, 4)))
    if modeset is not None and frac > 0.001: first = at(); break
    time.sleep(1)
note("screendump probes (time, bytes, non-black fraction):", probes[-8:])
note("driftwm modeset line at", modeset, "; FIRST FRAME (non-black screendump after the modeset) at", first)
ev = s.wait(r"Starting event loop", 5)
note("driftwm 'Starting event loop' line seen:", ev >= 0)
s.screendump("01-empty-canvas")
time.sleep(5)
note("serial log, lines about devices/libinput/seatd:")
for l in re.sub(r"\x1b\[[0-9;]*m", "", s.text()).splitlines():
    if re.search(r"mkudevdb|seatd/seat.c:(219|358)|libinput|Opening device|No \[\[outputs|Using GPU|primary GPU|renderer|GL Renderer|event loop|ERROR|panicked", l):
        print("   ", l[:200])
st = s.sh(f"{ENV} driftwm msg state 2>&1 | head -n 40")
note("driftwm msg state:\n" + st)
if mode == "noworkaround":
    s.key("a"); s.move(16000, 16000); time.sleep(1)
    note("after a key and a pointer move, driftwm log tail:\n" + re.sub(r"\x1b\[[0-9;]*m", "", s.text()[-600:]))
    s.close(); sys.exit(0)

if mode == "bar":
    time.sleep(8)
    note("start-bar:\n" + s.sh("/usr/local/bin/start-bar 2>&1", 60))
    time.sleep(25)
    p = s.screendump("06-bar"); note("screenshot 06-bar", png_stats(p))
    note("processes:\n" + s.sh("ps | grep -E 'dbus|hubd|waybar' | grep -v grep"))
    note("hubd log:\n" + s.sh("tail -n 15 /run/hubos/hubd.log"))
    note("waybar log:\n" + s.sh("tail -n 25 /run/hubos/waybar.log"))
    note("driftwm msg state:\n" + s.sh(f"{ENV} driftwm msg state 2>&1 | head -n 20"))
    note("hubd list:\n" + s.sh(f"{ENV} XDG_RUNTIME_DIR=/run/dw hubd list 2>&1 | head -n 12"))
    s.close(); sys.exit(0)

# terminal window (the foot service)
time.sleep(5)
st = s.sh(f"{ENV} driftwm msg state 2>&1 | head -n 40")
note("state with the terminal:\n" + st)
p = s.screendump("02-terminal"); note("screenshot 02-terminal", png_stats(p))
# keyboard: type into the terminal. sendkey needs pacing: hold 20 ms, 0.5 s between keys (see the proposal).
NAMES = {" ": "spc", "\n": "ret", ".": "dot", "-": "minus", "/": "slash", "_": "shift-minus", "'": "apostrophe", "\\": "backslash",
         "[": "bracket_left", "?": "shift-slash", ";": "semicolon", "<": "shift-comma", "|": "shift-backslash", ";": "semicolon"}
def typ(txt):
    for ch in txt:
        k = NAMES.get(ch, ch)
        if ch.isupper(): k = "shift-" + ch.lower()
        s.monitor(f"sendkey {k} 20"); time.sleep(0.5)
typ("\n"); typ("touch /run/key-ok\n"); time.sleep(4)
note("keyboard check (file created by the typed command):", s.sh("ls -l /run/key-ok 2>&1").strip().splitlines()[-2])
p = s.screendump("03-typed"); note("screenshot 03-typed", png_stats(p))
# mouse: a relative usb-mouse. Ask foot for mouse reports, move the pointer, click, show what the terminal received.
typ("printf '\\e[?1000h\\e[?1006h'; stty -icanon -echo; head -c 20 | cat -v; stty sane\n"); time.sleep(2)
s.move(-2000, -2000); time.sleep(1); s.move(500, 250); time.sleep(1.5)
p = s.screendump("04-pointer-moved"); note("screenshot 04-pointer-moved", png_stats(p))
s.button(1); time.sleep(0.4); s.button(0); time.sleep(2)
p = s.screendump("05-click-report"); note("screenshot 05-click-report", png_stats(p))
st3 = s.sh(f"{ENV} driftwm msg state 2>&1 | head -n 12")
note("driftwm msg state at the end:\n" + st3)
print("---- serial log tail ----")
print(re.sub(r"\x1b\[[0-9;]*m", "", s.text()[-2500:]))
s.close()
