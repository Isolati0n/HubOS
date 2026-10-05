import os
#!/usr/bin/env python3
"""Plan C test: wl-restart (real, from github.com/Ferdi265/wl-restart) holds the Wayland socket and restarts the
PATCHED driftwm (--socket NAME --wayland-fd FD adoption). A Qt 6.8.2 client with QT_WAYLAND_RECONNECT=1 should survive.
usage: planC.py OUTDIR [standin|nostandin] [reconnect|noreconnect]"""
import json, os, signal, socket, subprocess, sys, time, glob, shutil
S = os.environ['HS_WORK']
H = S + '/hubstab'
OUT = sys.argv[1]; STANDIN = (sys.argv[2] if len(sys.argv) > 2 else 'standin') == 'standin'
RECONNECT = (sys.argv[3] if len(sys.argv) > 3 else 'reconnect') == 'reconnect'
RT = '/tmp/hsw'
shutil.rmtree(OUT, ignore_errors=True); os.makedirs(OUT + '/data/applications'); shutil.rmtree(RT, ignore_errors=True); os.makedirs(RT); os.chmod(RT, 0o700)
L = S + '/A/root/usr/lib/x86_64-linux-gnu'
ENV = dict(os.environ)
ENV.update({'XDG_RUNTIME_DIR': RT, 'DISPLAY': ':81', 'LD_LIBRARY_PATH': L, 'LIBGL_ALWAYS_SOFTWARE': '1',
            'MESA_LOADER_DRIVER_OVERRIDE': 'swrast', '__EGL_VENDOR_LIBRARY_DIRS': S + '/A/root/usr/share/glvnd/egl_vendor.d',
            'LIBGL_DRIVERS_PATH': L + '/dri', 'GBM_BACKENDS_PATH': L + '/gbm', 'RUST_BACKTRACE': '1',
            'PATH': S + '/A/root/usr/bin:' + os.environ.get('PATH', ''), 'XDG_DATA_HOME': OUT + '/data',
            'XDG_DATA_DIRS': OUT + '/data:/usr/share', 'XDG_STATE_HOME': OUT + '/state'})
ENV.pop('WAYLAND_DISPLAY', None)
T0 = time.monotonic()
LOGF = open(OUT + '/timeline.log', 'a')
def log(m):
    LOGF.write(f'{time.monotonic() - T0:9.3f} {m}\n'); LOGF.flush(); print(f'{time.monotonic() - T0:9.3f} {m}', flush=True)
if STANDIN:
    open(OUT + '/data/applications/hubos-q1.desktop', 'w').write(f'[Desktop Entry]\nType=Application\nName=q1\nExec=/bin/true\nStartupWMClass=hubos-q1\n')
open(OUT + '/c.toml', 'w').write('[session]\nrestore_windows = true\n')
lf = open(OUT + '/wlr.log', 'wb')
wlr = subprocess.Popen([H + '/bin/wl-restart', '-n', '5', '--', H + '/bin/driftwm-patched', '--backend', 'winit', '--config', OUT + '/c.toml',
                        '--session-file', OUT + '/s.json'], env=ENV, stdout=lf, stderr=lf)
def ipc(req, timeout=3):
    s = socket.socket(socket.AF_UNIX); s.settimeout(timeout)
    s.connect(glob.glob(RT + '/driftwm/ipc-*.sock')[0]); s.sendall((json.dumps(req) + '\n').encode())
    buf = b''
    while not buf.endswith(b'\n'):
        c = s.recv(1 << 20)
        if not c: break
        buf += c
    return json.loads(buf)
def state(): return ipc('State')['Ok']['State']
for _ in range(200):
    try: state(); break
    except Exception: time.sleep(0.1)
socks = [os.path.basename(p) for p in glob.glob(RT + '/wayland-*') if not p.endswith('.lock')]
log(f'wl-restart pid={wlr.pid}; socket(s) in runtime dir: {socks}; children: {subprocess.run(["pgrep", "-P", str(wlr.pid)], capture_output=True, text=True).stdout.split()}')
e = dict(ENV); e['WAYLAND_DISPLAY'] = socks[0]
if RECONNECT: e['QT_WAYLAND_RECONNECT'] = '1'
qlog = open(OUT + '/qtc.log', 'wb')
qt = subprocess.Popen(['bash', S + '/hs/qtrun.sh', 'q1'], env=e, stdout=qlog, stderr=subprocess.STDOUT)
t = time.time()
while time.time() - t < 20:
    ws = state()['windows']
    if ws: break
    time.sleep(0.1)
before = [(w['id'], w['app_id'], w['position'], w['size'], w['suspended']) for w in state()['windows']]
ipc({'Move': {'window': before[0][0], 'to': [555, -222]}}); ipc({'Resize': {'window': before[0][0], 'to': [640, 360]}})
time.sleep(2)
before = [(w['id'], w['app_id'], w['position'], w['size'], w['suspended']) for w in state()['windows']]
log(f'window before: {before}')
time.sleep(3)
drv = subprocess.run(['pgrep', '-P', str(wlr.pid)], capture_output=True, text=True).stdout.split()
dpid = int(drv[0])
tk = time.monotonic()
os.kill(dpid, signal.SIGKILL); log(f'=== kill -9 driftwm pid {dpid}')
# watch the Qt client output and the new compositor
t_ipc = t_win = None
seen = None
while time.monotonic() - tk < 40:
    if t_ipc is None:
        try:
            st = state(); t_ipc = time.monotonic() - tk; log(f'new compositor answers IPC after {t_ipc:.3f}s')
        except Exception: pass
    if t_ipc is not None:
        try:
            ws = state()['windows']
            live = [w for w in ws if not w['suspended']]
            if live and t_win is None:
                t_win = time.monotonic() - tk
                log(f'live window back after {t_win:.3f}s: {[(w["id"], w["app_id"], w["position"], w["size"]) for w in ws]}')
                break
        except Exception: pass
    time.sleep(0.02)
time.sleep(5)
ws = state()['windows']
log(f'final windows: {[(w["id"], w["app_id"], w["position"], w["size"], w["suspended"]) for w in ws]}')
log(f'Qt client still running: {qt.poll() is None}  (exit code {qt.poll()})')
ticks = [l for l in open(OUT + '/qtc.log', errors='replace') if 'tick' in l]
log(f'Qt client ticks total {len(ticks)}; last: {ticks[-1].strip() if ticks else None}')
gaps = []
ts = [int(l.split('t=')[1].split()[0]) for l in ticks]
for a, b in zip(ts, ts[1:]):
    if b - a > 1500: gaps.append((b - a) / 1000)
log(f'gaps in the Qt client tick stream (>1.5 s): {gaps}')
log('wl-restart log: ' + open(OUT + '/wlr.log', errors='replace').read()[-600:].replace('\n', ' | '))
qt.terminate(); wlr.terminate()
time.sleep(1)
for q in subprocess.run(['pgrep', '-P', str(wlr.pid)], capture_output=True, text=True).stdout.split(): os.kill(int(q), signal.SIGTERM)
