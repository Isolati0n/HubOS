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

GOOD = '[session]\nrestore_windows = true\n'
def entries():
    return sorted(e_['app_id'] for e_ in json.load(open(SESSION))['entries'])
assert start()
e = dict(ENV); e['WAYLAND_DISPLAY'] = sockname
for n in range(1, N + 1): subprocess.Popen(['foot', f'--app-id=hubos-s{n:02d}', f'--title=s{n:02d}', 'sleep', '100000'], env=e, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, cwd=OUT)
for _ in range(200):
    ws = [w for w in st()['windows'] if w['app_id'].startswith('hubos-s')]
    if len(ws) == N: break
    time.sleep(0.25)
time.sleep(1)
for i, w in enumerate(ws): ipc({'Move': {'window': w['id'], 'to': [-1500 + (i % 5) * 600, -600 + (i // 5) * 400]}})
time.sleep(7)
print('live windows open: %d; session.json entries: %d' % (len(ws), len(entries())))
for kind, body in (('empty file', ''), ('invalid TOML', '[[[[ = =\n'), ('unknown key only', 'nonsense = 1\n'), ('valid file without [session]', '[decorations]\nbg_color = "#112233"\n')):
    open(CONF, 'w').write(GOOD); time.sleep(2)
    ipc({'Move': {'window': ws[0]['id'], 'to': [-1500, -600]}}); time.sleep(2.5)   # forces a save with the good config
    before = entries()
    open(CONF, 'w').write(body); time.sleep(0.5)
    ipc({'Move': {'window': ws[1]['id'], 'to': [-900, -600 + 50]}}); time.sleep(2.5)  # any window change while the odd config is active
    during = entries()
    open(CONF, 'w').write(GOOD); time.sleep(3)
    after = entries()
    print(f'config -> {kind}: session.json entries before={len(before)} while-odd-config-active={len(during)} after-good-config-restored={len(after)}')
proc.kill()
