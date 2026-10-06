#!/usr/bin/env python3
"""hangdrill.py OUTDIR DW_BINARY - a REAL hang (the popup-parent loop found by the fuzzer, no test hook) against the hub-stability
hang policy: probe every 2 s with two independent probes (IPC 'State', Wayland wl_display.sync); kill -9 only after both fail for
30 s; restart after 1 s; stand-ins come back; relaunch the stand-ins; compare geometry.  Reports the timeline.
Needs the pristine (or any build without Smithay P7)."""
import json, os, signal, socket, struct, subprocess, sys, time, glob, shutil
OUT = sys.argv[1]; DW = sys.argv[2]
W = os.environ['W']; A = W + '/A'; RT = '/tmp/bc4hd'; DISP = os.environ.get('SOAK_DISPLAY', ':93')
os.makedirs(OUT, exist_ok=True)
L = A + '/root/usr/lib/x86_64-linux-gnu'
ENV = dict(os.environ)
ENV.update({'XDG_RUNTIME_DIR': RT, 'DISPLAY': DISP, 'LD_LIBRARY_PATH': L, 'LIBGL_ALWAYS_SOFTWARE': '1', 'MESA_LOADER_DRIVER_OVERRIDE': 'swrast',
            '__EGL_VENDOR_LIBRARY_DIRS': A + '/root/usr/share/glvnd/egl_vendor.d', 'LIBGL_DRIVERS_PATH': L + '/dri', 'GBM_BACKENDS_PATH': L + '/gbm',
            'XDG_DATA_HOME': OUT + '/data', 'PATH': A + '/root/usr/bin:' + os.environ.get('PATH', ''), 'XDG_STATE_HOME': OUT + '/state'})
ENV.pop('WAYLAND_DISPLAY', None)
shutil.rmtree(RT, ignore_errors=True); os.makedirs(RT); os.chmod(RT, 0o700)
os.makedirs(OUT + '/data/applications', exist_ok=True)
N = 20
for n in range(1, N + 1):
    open(f'{OUT}/data/applications/hubos-s{n:02d}.desktop', 'w').write(
        f"[Desktop Entry]\nType=Application\nName=s{n:02d}\nExec=foot --app-id=hubos-s{n:02d} --title=s{n:02d} sleep 100000\nStartupWMClass=hubos-s{n:02d}\n")
CONF = OUT + '/c.toml'; open(CONF, 'w').write('[session]\nrestore_windows = true\n')
SESSION = OUT + '/session.json'
LOG = open(OUT + '/driftwm.log', 'ab')
T0 = time.time()
def rep(m): print(f'{time.time() - T0:7.2f}s {m}', flush=True)
def ipc(req, timeout=3.0):
    p = glob.glob(RT + '/driftwm/ipc-*.sock')
    s = socket.socket(socket.AF_UNIX); s.settimeout(timeout)
    try:
        s.connect(p[0]); s.sendall((json.dumps(req) + '\n').encode()); buf = b''
        while not buf.endswith(b'\n'):
            c = s.recv(1 << 20)
            if not c: break
            buf += c
    finally: s.close()
    return json.loads(buf)
def st(): return ipc('State')['Ok']['State']
sockname = None
def wl_sync(timeout=2.0):
    s = socket.socket(socket.AF_UNIX); s.settimeout(timeout)
    try:
        s.connect(RT + '/' + sockname); s.sendall(struct.pack('<III', 1, (12 << 16) | 0, 2)); buf = b''; t0 = time.time()
        while time.time() - t0 < timeout:
            buf += s.recv(4096)
            if len(buf) >= 8 and struct.unpack('<I', buf[:4])[0] == 2: return True
        return False
    except Exception: return False
    finally: s.close()
proc = None
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
assert start()
e = dict(ENV); e['WAYLAND_DISPLAY'] = sockname
for n in range(1, N + 1): subprocess.Popen(['foot', f'--app-id=hubos-s{n:02d}', f'--title=s{n:02d}', 'sleep', '100000'], env=e, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, cwd=OUT)
for _ in range(200):
    ws = [w for w in st()['windows'] if w['app_id'].startswith('hubos-s')]
    if len(ws) == N: break
    time.sleep(0.25)
for i, w in enumerate(ws): ipc({'Move': {'window': w['id'], 'to': [-1500 + (i % 5) * 600, -600 + (i // 5) * 400]}}); ipc({'Resize': {'window': w['id'], 'to': [500, 300]}})
time.sleep(1.0)
before = {w['app_id']: (tuple(w['position']), tuple(w['size'])) for w in st()['windows'] if w['app_id'].startswith('hubos-s')}
for w in st()['windows']:
    if w['app_id'].startswith('hubos-s'): ipc({'Suspend': w['id']})   # stand-ins are what a crash leaves behind
time.sleep(7)
# NOW: kill nothing; instead restore from stand-ins, relaunch, then trigger the hang with a live client
for w in st()['windows']:
    if w['app_id'].startswith('hubos-s'): ipc({'Relaunch': w['id']})
for _ in range(100):
    if len([w for w in st()['windows'] if w['app_id'].startswith('hubos-s') and not w.get('suspended')]) == N: break
    time.sleep(0.2)
time.sleep(8)   # let the debounced session write (1 s) and the clients settle
before = {w['app_id']: (tuple(w['position']), tuple(w['size'])) for w in st()['windows'] if w['app_id'].startswith('hubos-s') and not w.get('suspended')}
rep(f'{N} live windows up; triggering the hang with the 5-line popup client (repro_popup.py self)')
t_trigger = time.time()
subprocess.Popen([sys.executable, W + '/repro_popup.py', RT + '/' + sockname, 'self'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
fail_since = None; rounds = 0; killed_at = None
while True:
    time.sleep(2.0)
    ok_ipc = True
    try: st()
    except Exception: ok_ipc = False
    ok_wl = wl_sync()
    if not ok_ipc and not ok_wl:
        rounds += 1; fail_since = fail_since or time.time()
        rep(f'probe round failed ({rounds}): ipc={ok_ipc} wl_sync={ok_wl}')
        if time.time() - fail_since >= 30 and rounds >= 4:
            status = open(f'/proc/{proc.pid}/status').read().split('\n')[2]
            rep(f'HUNG for {time.time() - fail_since:.1f}s: {status}; kill -9')
            proc.kill(); proc.wait(); killed_at = time.time(); break
    else:
        if fail_since is None and time.time() - t_trigger > 20:
            rep('no hang happened (probes fine 20 s after the trigger): this build is not affected'); sys.exit(0)
if True:
    for f in glob.glob(OUT + '/*.x'): pass
    time.sleep(1.0)
    start(); t_up = time.time()
    standins = [w for w in st()['windows'] if w.get('suspended')]
    got = {w['app_id']: (tuple(w['position']), tuple(w['size'])) for w in standins}
    rep(f'restarted: IPC back {t_up - killed_at:.2f}s after the kill (incl. the 1.0 s restart delay); stand-ins restored: {len(standins)}; geometry equal to before: {sum(1 for k, v in got.items() if before.get(k) == v)}/{len(before)}')
    rep('stand-in vs before differences: ' + json.dumps({k: [before.get(k), v] for k, v in got.items() if before.get(k) != v})[:600])
    for w in standins: ipc({'Relaunch': w['id']})
    for _ in range(100):
        live = {w['app_id']: (tuple(w['position']), tuple(w['size'])) for w in st()['windows'] if w['app_id'].startswith('hubos-s') and not w.get('suspended')}
        if len(live) == N: break
        time.sleep(0.1)
    rep(f'all windows live again {time.time() - killed_at:.2f}s after the kill; live geometry equal to before: {sum(1 for k, v in live.items() if before.get(k) == v)}/{len(before)}')
    rep('live vs before differences: ' + json.dumps({k: [before.get(k), v] for k, v in live.items() if before.get(k) != v})[:600])
    rep(f'total time from the hang trigger to everything back: {time.time() - t_trigger:.1f}s (of which {killed_at - t_trigger:.1f}s detection window)')
proc.kill()
