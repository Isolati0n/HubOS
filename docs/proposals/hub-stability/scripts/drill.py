import os
#!/usr/bin/env python3
"""Crash drill: kill -9 the compositor N times with 20 windows, restore through the stand-ins (driftwm's own relaunch, foot as the viewer),
check every cycle: time to answer, time to 20 live windows, positions, duplicates, session file health, fd/RSS of the new process.
usage: drill.py OUTDIR CYCLES"""
import json, os, random, signal, socket, subprocess, sys, time, glob, shutil, statistics
S = os.environ['HS_WORK']
OUT = sys.argv[1]; N = int(sys.argv[2])
RT = '/tmp/hsd2'
shutil.rmtree(OUT, ignore_errors=True); os.makedirs(OUT + '/data/applications'); shutil.rmtree(RT, ignore_errors=True); os.makedirs(RT); os.chmod(RT, 0o700)
L = S + '/A/root/usr/lib/x86_64-linux-gnu'
ENV = dict(os.environ)
ENV.update({'XDG_RUNTIME_DIR': RT, 'DISPLAY': ':81', 'LD_LIBRARY_PATH': L, 'LIBGL_ALWAYS_SOFTWARE': '1', 'MESA_LOADER_DRIVER_OVERRIDE': 'swrast',
            '__EGL_VENDOR_LIBRARY_DIRS': S + '/A/root/usr/share/glvnd/egl_vendor.d', 'LIBGL_DRIVERS_PATH': L + '/dri', 'GBM_BACKENDS_PATH': L + '/gbm',
            'PATH': S + '/A/root/usr/bin:' + os.environ.get('PATH', ''), 'XDG_DATA_HOME': OUT + '/data', 'XDG_DATA_DIRS': OUT + '/data:/usr/share', 'XDG_STATE_HOME': OUT + '/state'})
ENV.pop('WAYLAND_DISPLAY', None)
ids = [f'm{i:02d}' for i in range(1, 21)]
for i in ids:
    open(f'{OUT}/data/applications/hubos-{i}.desktop', 'w').write(f'[Desktop Entry]\nType=Application\nName={i}\nExec=foot --app-id=hubos-{i} --title={i} sleep infinity\nStartupWMClass=hubos-{i}\n')
open(OUT + '/c.toml', 'w').write('[session]\nrestore_windows = true\n')
SESSION = OUT + '/session.json'
LOG = open(OUT + '/drill.log', 'w')
def log(m): LOG.write(m + '\n'); LOG.flush(); print(m, flush=True)
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
dw = [None]
def start():
    for f in glob.glob(RT + '/wayland-*'):
        try: os.remove(f)
        except OSError: pass
    dw[0] = subprocess.Popen([S + '/hubstab/bin/driftwm-pristine', '--backend', 'winit', '--config', OUT + '/c.toml', '--session-file', SESSION], env=ENV,
                             stdout=open(OUT + '/dw.log', 'ab'), stderr=subprocess.STDOUT)
start()
for _ in range(200):
    try: state(); break
    except Exception: time.sleep(0.1)
e2 = dict(ENV); e2['WAYLAND_DISPLAY'] = 'wayland-1'
feet = [subprocess.Popen(['foot', f'--app-id=hubos-{i}', f'--title={i}', 'sleep', 'infinity'], env=e2, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL) for i in ids]
time.sleep(4)
rnd = random.Random(5)
def scatter():
    ws = state()['windows']
    for w in ws:
        ipc({'Move': {'window': w['id'], 'to': [rnd.randint(-2500, 2500), rnd.randint(-1500, 1500)]}})
scatter(); time.sleep(8)
res = []
for cyc in range(N):
    # churn right before the kill, so the last rolling save may be stale
    for _ in range(rnd.randint(0, 3)):
        w = rnd.choice(state()['windows']); ipc({'Move': {'window': w['id'], 'to': [rnd.randint(-2500, 2500), rnd.randint(-1500, 1500)]}})
        time.sleep(rnd.uniform(0.05, 0.4))
    time.sleep(rnd.uniform(0.0, 1.6))
    before = {w['app_id']: (w['position'], w['size']) for w in state()['windows']}
    pid = dw[0].pid
    tk = time.monotonic(); os.kill(pid, signal.SIGKILL); dw[0].wait()
    time.sleep(1.0); start()   # supervisor delay 1.0 s
    t_ans = None
    while time.monotonic() - tk < 30:
        try: s2 = state(); t_ans = time.monotonic() - tk; break
        except Exception: time.sleep(0.01)
    sus = {w['app_id']: w['id'] for w in s2['windows'] if w.get('suspended')}
    for aid, wid in sus.items(): ipc({'Relaunch': wid})
    t_live = None
    while time.monotonic() - tk < 40:
        ws = state()['windows']
        if sum(1 for w in ws if not w.get('suspended')) >= 20: t_live = time.monotonic() - tk; break
        time.sleep(0.05)
    time.sleep(1.5)
    ws = state()['windows']
    after = {w['app_id']: (w['position'], w['size']) for w in ws if not w.get('suspended')}
    moved = [a for a in before if a in after and before[a] != after[a]]
    missing = [a for a in before if a not in after]
    dup = len(ws) - len(set(w['app_id'] for w in ws))
    left = sum(1 for w in ws if w.get('suspended'))
    try:
        json.load(open(SESSION)); sess = 'ok'
    except Exception as e: sess = 'BAD ' + str(e)[:40]
    quar = len(glob.glob(OUT + '/state/driftwm/session.json.*')) + len(glob.glob(SESSION + '.*'))
    fds = len(os.listdir(f'/proc/{dw[0].pid}/fd')); rss = int([l for l in open(f'/proc/{dw[0].pid}/status') if l.startswith('VmRSS')][0].split()[1]) // 1024
    res.append((t_ans, t_live, len(moved), len(missing), dup, left, sess, fds, rss))
    log(f'cycle {cyc + 1}: answer {t_ans and round(t_ans, 2)}s live20 {t_live and round(t_live, 2)}s | windows whose place changed {len(moved)} (stale rolling save), missing {len(missing)}, duplicates {dup}, stand-ins left {left}, session file {sess}, quarantined files {quar}, new process fds {fds} rss {rss} MB')
    # the restored windows are the new baseline
    time.sleep(rnd.uniform(1, 3))
ta = [r[0] for r in res if r[0]]; tl = [r[1] for r in res if r[1]]
log(f'SUMMARY {N} cycles: answer median {statistics.median(ta):.2f}s max {max(ta):.2f}s; 20 live median {statistics.median(tl):.2f}s max {max(tl):.2f}s; '
    f'cycles with a missing window {sum(1 for r in res if r[3])}, duplicates {sum(1 for r in res if r[4])}, stand-ins left over {sum(1 for r in res if r[5])}, '
    f'cycles where some window was not at its pre-kill place {sum(1 for r in res if r[2])} (total windows {sum(r[2] for r in res)}), bad session files {sum(1 for r in res if r[6] != "ok")}')
for f in feet: f.kill()
dw[0].terminate(); dw[0].wait()
