#!/usr/bin/env python3
"""relaunchtwice.py OUTDIR DW_BINARY [N] [ROUNDS]
Restore chain: stand-ins -> Relaunch all (live windows adopt the stand-ins) -> wait -> compare live geometry with session.json
-> kill -9 -> restart -> compare restored stand-ins with the live geometry before the kill.  Repeated ROUNDS times with the same session file.
This is the 'restore twice' case: does a second crash restore the same picture as the first?"""
import json, os, signal, socket, subprocess, sys, time, glob, shutil
OUT = sys.argv[1]; DW = sys.argv[2]; N = int(sys.argv[3]) if len(sys.argv) > 3 else 12; ROUNDS = int(sys.argv[4]) if len(sys.argv) > 4 else 3
W = os.environ['W']; A = W + '/A'; RT = '/tmp/bc4rt2'; DISP = os.environ.get('SOAK_DISPLAY', ':93')
os.makedirs(OUT, exist_ok=True)
L = A + '/root/usr/lib/x86_64-linux-gnu'
ENV = dict(os.environ)
ENV.update({'XDG_RUNTIME_DIR': RT, 'DISPLAY': DISP, 'LD_LIBRARY_PATH': L, 'LIBGL_ALWAYS_SOFTWARE': '1', 'MESA_LOADER_DRIVER_OVERRIDE': 'swrast',
            '__EGL_VENDOR_LIBRARY_DIRS': A + '/root/usr/share/glvnd/egl_vendor.d', 'LIBGL_DRIVERS_PATH': L + '/dri', 'GBM_BACKENDS_PATH': L + '/gbm',
            'XDG_DATA_HOME': OUT + '/data', 'PATH': A + '/root/usr/bin:' + os.environ.get('PATH', ''), 'XDG_STATE_HOME': OUT + '/state'})
ENV.pop('WAYLAND_DISPLAY', None)
shutil.rmtree(RT, ignore_errors=True); os.makedirs(RT); os.chmod(RT, 0o700)
os.makedirs(OUT + '/data/applications', exist_ok=True)
for n in range(1, N + 1):
    open(f'{OUT}/data/applications/hubos-s{n:02d}.desktop', 'w').write(
        f"[Desktop Entry]\nType=Application\nName=s{n:02d}\nExec=foot --app-id=hubos-s{n:02d} --title=s{n:02d} sleep 100000\nStartupWMClass=hubos-s{n:02d}\n")
CONF = OUT + '/c.toml'; open(CONF, 'w').write('[session]\nrestore_windows = true\n')
SESSION = OUT + '/session.json'; LOG = open(OUT + '/driftwm.log', 'ab')
def ipc(req, timeout=5.0):
    s = socket.socket(socket.AF_UNIX); s.settimeout(timeout)
    try:
        s.connect(glob.glob(RT + '/driftwm/ipc-*.sock')[0]); s.sendall((json.dumps(req) + '\n').encode()); buf = b''
        while not buf.endswith(b'\n'):
            c = s.recv(1 << 20)
            if not c: break
            buf += c
    finally: s.close()
    return json.loads(buf)
def st(): return ipc('State')['Ok']['State']
def geom(ws, live=None):
    return {w['app_id']: (tuple(w['position']), tuple(w['size'])) for w in ws if w['app_id'].startswith('hubos-s') and (live is None or bool(w.get('suspended')) != live)}
proc = None; sockname = None
def start():
    global proc, sockname
    for f in glob.glob(RT + '/wayland-*') + glob.glob(RT + '/driftwm/*'):
        try: os.remove(f)
        except OSError: pass
    proc = subprocess.Popen([DW, '--backend', 'winit', '--config', CONF, '--session-file', SESSION], env=ENV, stdout=LOG, stderr=LOG, cwd=OUT)
    for _ in range(300):
        socks = [p for p in glob.glob(RT + '/wayland-*') if not p.endswith('.lock')]
        if socks and glob.glob(RT + '/driftwm/ipc-*.sock'):
            sockname = os.path.basename(socks[0])
            try: st(); return True
            except Exception: pass
        time.sleep(0.1)
    return False
def kill():
    proc.kill(); proc.wait()
    time.sleep(0.5)
assert start()
e = dict(ENV); e['WAYLAND_DISPLAY'] = sockname
for n in range(1, N + 1): subprocess.Popen(['foot', f'--app-id=hubos-s{n:02d}', f'--title=s{n:02d}', 'sleep', '100000'], env=e, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, cwd=OUT)
for _ in range(200):
    ws = [w for w in st()['windows'] if w['app_id'].startswith('hubos-s')]
    if len(ws) == N: break
    time.sleep(0.25)
time.sleep(1)
for i, w in enumerate(ws): ipc({'Move': {'window': w['id'], 'to': [-1500 + (i % 5) * 600, -600 + (i // 5) * 400]}})
time.sleep(1)
for w in st()['windows']:
    if w['app_id'].startswith('hubos-s'): ipc({'Suspend': w['id']})
time.sleep(7)
base = geom(st()['windows'], live=False)
print(f'seed: {len(base)} stand-ins; reference geometry = the stand-ins')
kill()
for rnd in range(1, ROUNDS + 1):
    assert start()
    s0 = geom(st()['windows'], live=False)
    sess = {e_['app_id']: (tuple(e_['position']), tuple(e_['size'])) for e_ in json.load(open(SESSION))['entries']}
    print(f'round {rnd}: after restart, stand-ins equal to the reference: {sum(1 for k, v in s0.items() if base.get(k) == v)}/{len(base)}; session.json equal: {sum(1 for k, v in sess.items() if base.get(k) == v)}/{len(sess)}')
    for w in st()['windows']:
        if w.get('suspended'): ipc({'Relaunch': w['id']})
    for _ in range(150):
        live = geom(st()['windows'], live=True)
        if len(live) == N: break
        time.sleep(0.2)
    time.sleep(8)
    live = geom(st()['windows'], live=True)
    sess = {e_['app_id']: (tuple(e_['position']), tuple(e_['size'])) for e_ in json.load(open(SESSION))['entries']}
    d1 = {k: [base.get(k), v] for k, v in live.items() if base.get(k) != v}
    d2 = {k: [live.get(k), v] for k, v in sess.items() if live.get(k) != v}
    print(f'round {rnd}: live windows equal to the reference: {len(live) - len(d1)}/{len(live)}; session.json (8 s after the relaunch) equal to the live windows: {len(sess) - len(d2)}/{len(sess)}')
    if d1: print('   live differs:', json.dumps(d1)[:400])
    if d2: print('   session.json differs from live:', json.dumps(d2)[:400])
    kill()
proc.kill()
