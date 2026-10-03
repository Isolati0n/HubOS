#!/usr/bin/env python3
"""steps.py WORK DISKDIR OUTDIR STEPS : second round of the desktop experiment (docs/proposals/phase-b-desktop.md).
STEPS is a comma list of: 1 (eudev) 2 (normal user) 3 (hot-plug) 4 (focus) 5 (hubd, bar, menu). Boots the disk once and runs them in
order, printing every command and its output; screenshots go to OUTDIR. EXPERIMENT; not part of any default test."""
import re, sys, time, os
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from session import *

work, disk, out, steps = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4].split(",")
TABLET = steps == ["6"]   # the tablet run: no PS/2 mouse (i8042=off), a usb-tablet instead of the usb-mouse, QMP for absolute moves
s = Session(work, disk, out, pointer="usb-tablet" if TABLET else "usb-mouse",
            extra=["-machine", "i8042=off", "-qmp", f"unix:{out}/qmp.sock,server,nowait"] if TABLET else None)
t0 = s.t0
H = "s6-setuidgid hub env XDG_RUNTIME_DIR=/run/dw WAYLAND_DISPLAY=wayland-1 HOME=/run/hub LC_ALL=C.UTF-8 DBUS_SESSION_BUS_ADDRESS=unix:path=/run/dw/bus"
def note(*a): print(f"[{time.time()-t0:6.1f}s]", *a, flush=True)
def run(cmd, t=60):
    r = s.sh(cmd, t); print("$", cmd); print(r.split("\n", 2)[-1].rstrip() if r.startswith(cmd) else r.rstrip(), flush=True); return r
def log(): return re.sub(r"\x1b\[[0-9;]*m", "", s.text())
def shot(name):
    p = s.screendump(name); note("screenshot", name, png_stats(p)[2], "bytes"); return p
def state(): return run(f"{H} driftwm msg state")
def corner(): s.move(-4000, -4000); time.sleep(0.5)

e = s.wait(r"Starting event loop", 240); note("driftwm 'Starting event loop' seen:", e >= 0)
time.sleep(10)
if "1" in steps:
    note("STEP 1: eudev instead of the hand-written database")
    run("ps | grep -E 'udevd|seatd|driftwm' | grep -v grep")
    run("ls -l /usr/lib/x86_64-linux-gnu/libudev*; ldd /usr/local/bin/driftwm | grep -E 'udev|input|seat'; udevadm --version; ls /run/udev; ls /run/udev/data | wc -l")
    run("grep -a 'udevd:' /dev/null; for e in /sys/class/input/event*; do echo \"$(basename $e) dev=$(cat $e/dev) name=$(cat $e/device/name)\"; done")
    run("for d in $(cat /sys/class/input/event*/dev); do echo \"== /run/udev/data/c$d\"; cat /run/udev/data/c$d; done 2>&1 | head -80")
    run("libinput list-devices 2>&1 | grep -E 'Device:|Kernel:|Seat:|Capabilities:'")
    note("driftwm log lines about input devices:")
    for l in log().splitlines():
        if re.search(r"smithay::backend::libinput|Configuring (mouse|keyboard)|udevd:", l): print("   ", l[:200])
    shot("s1-first-frame")
if "2" in steps:
    note("STEP 2: normal user in the seat group")
    run("id hub; grep -E '^(hub|seat):' /etc/group; ls -ln /run/seatd.sock; ps | grep -E 'seatd|driftwm|foot|udevd' | grep -v -E 'grep|supervise'")
    run("P=$(pidof driftwm); grep -E '^(Uid|Gid|Groups)' /proc/$P/status")
    run("ls -ld /run/dw /run/hub /run/hub/state; find /run/hub/state | head -20; ls -l /run/hub/state/driftwm 2>&1 | head")
    note("session store warnings in the driftwm log:", len([l for l in log().splitlines() if "durable session store" in l]))
    run(f"{H} driftwm msg state | head -8")
if "3" in steps:
    note("STEP 3: hot-plug (QEMU monitor device_add / device_del)")
    run("(udevadm monitor --udev --property > /run/hub/udevmon.log 2>&1 &) ; sleep 1; libinput list-devices | grep -c 'Device:'; ls /dev/input")
    m = s.mark()
    s.monitor("device_add usb-kbd,id=kbd2,bus=xhci.0"); time.sleep(5)
    run("ls /dev/input; libinput list-devices | grep -E 'Device:|Kernel:|Capabilities:' | tail -9")
    run("udevadm info -q property -n /dev/input/event5 | grep -E 'ID_INPUT|DEVNAME|ID_MODEL='")
    note("driftwm log lines since the plug:"); [print("   ", l[:200]) for l in log()[m:].splitlines() if re.search(r"libinput|New device|Configuring|Device", l)]
    # sendkey goes to the NEWEST keyboard (checked with libinput debug-events), so these keys come from the hot-plugged one
    run("(libinput debug-events --show-keycodes > /run/hub/ev-kbd.log 2>&1 &); sleep 2; echo watching")
    s.type_text("touch /run/hub/hp-kbd2\n"); time.sleep(3)
    run("grep -E 'KEYBOARD_KEY' /run/hub/ev-kbd.log | awk '{print $1}' | sort | uniq -c; ls -l /run/hub/hp-kbd2 2>&1; pkill -x libinput; true")
    shot("s3-after-kbd2-typing")
    m2 = s.mark()
    s.monitor("device_add usb-mouse,id=mouse2,bus=xhci.0"); time.sleep(5)
    print(re.sub(r"\x1b\[[0-9;]*[A-Za-z]", "", s.monitor("info mice")))
    run("ls /dev/input; libinput list-devices | grep -E 'Device:|Kernel:' | tail -6")
    run("(libinput debug-events > /run/hub/ev-mouse.log 2>&1 &); sleep 2; echo watching")
    a = shot("s3-before-mouse2"); s.move(300, 200); time.sleep(1.5); b = shot("s3-after-mouse2")
    note("pointer moved (+300,+200) after the mouse was plugged in: screenshot difference box", diff_bbox(a, b))
    run("grep -E 'POINTER_MOTION' /run/hub/ev-mouse.log | awk '{print $1}' | sort | uniq -c; pkill -x libinput; true")
    note("driftwm log lines since the mouse plug:"); [print("   ", l[:200]) for l in log()[m2:].splitlines() if re.search(r"libinput|New device|Configuring|Device", l)]
    m3 = s.mark()
    s.monitor("device_del kbd2"); s.monitor("device_del mouse2"); time.sleep(5)
    run("ls /dev/input; libinput list-devices | grep -E 'Device:|Kernel:' | tail -8")
    note("log lines after the removal:"); [print("   ", l[:200]) for l in log()[m3:].splitlines() if re.search(r"libinput|Device|remov|lost", l)]
    run("grep -E '^(ACTION|DEVNAME)=' /run/hub/udevmon.log | paste - - | grep input | head -20")
    s.type_text("touch /run/hub/after-unplug\n"); time.sleep(3); run("ls -l /run/hub/after-unplug 2>&1")
if "4" in steps:
    note("STEP 4: focus switching between two windows")
    run("s6-svc -d /run/service/foot; sleep 1")
    for n in ("alpha", "beta"):
        run(f"{H} env w={n} foot -T {n} > /dev/null 2>&1 &"); time.sleep(5)
    st = state()
    ids = dict((m.group(2), m.group(1)) for m in re.finditer(r"#(\d+) \S+ \[[^\]]*\] \S+\s+\"(\w+)\"", st))
    note("window ids by title:", ids)
    run(f"{H} driftwm msg move --id {ids['alpha']} -260 0; {H} driftwm msg resize --id {ids['alpha']} 400 400; {H} driftwm msg move --id {ids['beta']} 260 0; {H} driftwm msg resize --id {ids['beta']} 400 400")
    time.sleep(2); state(); shot("s4-two-windows")
    def click(x, y, label):
        corner(); s.move(x, y); time.sleep(1); s.button(1); time.sleep(0.3); s.button(0); time.sleep(1.5)
        note("clicked", label, "at", (x, y)); 
        r = run(f"{H} driftwm msg state | grep -E '^ *\\*? *#'"); return r
    typ = s.type_text
    click(250, 320, "alpha"); typ("touch /run/hub/typed-$w-1\n"); time.sleep(3)
    shot("s4-after-alpha")
    click(770, 320, "beta"); typ("touch /run/hub/typed-$w-2\n"); time.sleep(3)
    shot("s4-after-beta")
    click(250, 320, "alpha again"); typ("touch /run/hub/typed-$w-3\n"); time.sleep(3)
    run("ls /run/hub | grep typed")
    shot("s4-after-alpha-again")
if "6" in steps:
    note("STEP 6 (extra): the absolute pointer (usb-tablet)")
    run("libinput list-devices | grep -A4 -i tablet; ls /dev/input; udevadm info -q property -n /dev/input/event2 | grep -E 'ID_INPUT|NAME|DEVNAME'")
    run("(libinput debug-events > /run/hub/ev-tab.log 2>&1 &); sleep 2; echo watching")
    pos = []
    for (x, y) in [(6000, 6000), (26000, 26000), (16384, 16384)]:
        s.abs_move(x, y); time.sleep(1.5); pos.append(shot(f"s6-tablet-{x}"))
    note("absolute moves (fractions of 32767): difference box (6000,6000)->(26000,26000):", diff_bbox(pos[0], pos[1]), "; (26000,26000)->(16384,16384):", diff_bbox(pos[1], pos[2]))
    note("expected pointer positions in pixels: (187,117), (812,506), (512,320)")
    run("grep -E 'POINTER_MOTION' /run/hub/ev-tab.log | head -6 | cut -c1-150; pkill -x libinput; true")
if "5" in steps:
    note("STEP 5: hubd open, the Waybar item and the wofi menu")
    run("/usr/local/bin/start-bar", 90); time.sleep(20)
    run("ps | grep -E 'dbus|hubd|waybar|fakenode' | grep -v grep")
    run(f"{H} hubd list 2>&1 | head -14")
    shot("s5-bar")
    run(f"{H} hubd open ai-1 2>&1"); time.sleep(8)
    state(); shot("s5-opened")
    run(f"{H} hubd list --flat 2>&1 | head -8")
    run(f"{H} hubd end ai-1 2>&1"); time.sleep(4); state()
    X, Y = int(os.environ.get("BAR_X", 40)), int(os.environ.get("BAR_Y", 15))
    corner(); s.move(X, Y); time.sleep(1); shot("s5-pointer-on-bar"); s.button(1); time.sleep(0.3); s.button(0); time.sleep(6)
    shot("s5-after-bar-click")
    run("ps | grep -E 'wofi' | grep -v grep; tail -4 /run/hub/hubd.log; tail -3 /run/hub/waybar.log")
    if os.environ.get("ROW"):
        rx, ry = [int(v) for v in os.environ["ROW"].split(",")]
        s.move(rx - X, ry - Y); time.sleep(1); shot("s5-pointer-on-row"); s.button(1); time.sleep(0.3); s.button(0); time.sleep(5)
        shot("s5-after-row-click"); state(); run("tail -6 /run/hub/hubd.log")
note("done")
print("---- serial tail ----"); print(log()[-1800:])
s.close()
