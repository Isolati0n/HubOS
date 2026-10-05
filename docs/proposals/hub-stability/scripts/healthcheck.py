#!/usr/bin/env python3
"""After a fuzz run: is the compositor still healthy? IPC answers, a new foot window maps, a move works. usage: healthcheck.py RUNTIME_DIR"""
import os, sys, json, socket, glob, subprocess, time
S = os.environ['HS_WORK']
RT = sys.argv[1]
def ipc(req, timeout=3):
    s = socket.socket(socket.AF_UNIX); s.settimeout(timeout)
    s.connect(glob.glob(RT + '/driftwm/ipc-*.sock')[0]); s.sendall((json.dumps(req) + '\n').encode())
    buf = b''
    while not buf.endswith(b'\n'):
        c = s.recv(1 << 20)
        if not c: break
        buf += c
    return json.loads(buf)
try:
    n0 = len(ipc('State')['Ok']['State']['windows'])
    L = S + '/A/root/usr/lib/x86_64-linux-gnu'
    e = dict(os.environ); e.update({'XDG_RUNTIME_DIR': RT, 'WAYLAND_DISPLAY': 'wayland-1', 'LD_LIBRARY_PATH': L, 'PATH': S + '/A/root/usr/bin:' + os.environ['PATH']})
    p = subprocess.Popen(['foot', '--app-id=hc-check', 'sleep', '30'], env=e, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    ok = False
    for _ in range(40):
        time.sleep(0.25)
        ws = ipc('State')['Ok']['State']['windows']
        if any(w['app_id'] == 'hc-check' for w in ws):
            ok = True; break
    r = ipc({'Move': {'window': [w['id'] for w in ws if w['app_id'] == 'hc-check'][0], 'to': [10, 10]}}) if ok else None
    print('HEALTH: ipc ok; windows before %d; a new foot window mapped: %s; move reply: %s' % (n0, ok, r))
    p.kill()
except Exception as ex:
    print('HEALTH: FAILED', type(ex).__name__, ex)
