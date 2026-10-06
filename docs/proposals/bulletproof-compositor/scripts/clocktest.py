#!/usr/bin/env python3
"""clocktest.py - clock-jump test of the nested compositor with libfaketime (LD_PRELOAD).
Phase R: only CLOCK_REALTIME jumps (this is what NTP steps, a dead RTC battery or a manual date change do; Linux's CLOCK_MONOTONIC never jumps).
Phase M: CLOCK_MONOTONIC is faked too (out of spec for Linux, a stress test of every timer in the program).
After every jump: does IPC answer, does a window move reach session.json (the 1 s debounce), does a camera animation finish, is session 'saved_at' = the faked wall clock.
usage: clocktest.py OUTDIR DW_BINARY
"""
import json, os, signal, socket, subprocess, sys, time, glob, shutil
OUT = sys.argv[1]; DW = sys.argv[2]
W = os.environ['W']; A = W + '/A'; RT = '/tmp/bc4clk'; DISP = os.environ.get('SOAK_DISPLAY', ':93')
os.makedirs(OUT, exist_ok=True)
L = A + '/root/usr/lib/x86_64-linux-gnu'
FT = L + '/faketime/libfaketimeMT.so.1'
RC = OUT + '/ft.rc'
ENVB = dict(os.environ)
ENVB.update({'XDG_RUNTIME_DIR': RT, 'DISPLAY': DISP, 'LD_LIBRARY_PATH': L, 'LIBGL_ALWAYS_SOFTWARE': '1', 'MESA_LOADER_DRIVER_OVERRIDE': 'swrast',
             '__EGL_VENDOR_LIBRARY_DIRS': A + '/root/usr/share/glvnd/egl_vendor.d', 'LIBGL_DRIVERS_PATH': L + '/dri', 'GBM_BACKENDS_PATH': L + '/gbm',
             'XDG_DATA_HOME': OUT + '/data', 'RUST_BACKTRACE': '1', 'PATH': A + '/root/usr/bin:' + os.environ.get('PATH', ''), 'XDG_STATE_HOME': OUT + '/state'})
ENVB.pop('WAYLAND_DISPLAY', None)
shutil.rmtree(RT, ignore_errors=True); os.makedirs(RT); os.chmod(RT, 0o700)
os.makedirs(OUT + '/data/applications', exist_ok=True)
for n in range(1, 6):
    open(f'{OUT}/data/applications/hubos-s{n:02d}.desktop', 'w').write(
        f"[Desktop Entry]\nType=Application\nName=s{n:02d}\nExec=foot --app-id=hubos-s{n:02d} --title=s{n:02d} sleep 100000\nStartupWMClass=hubos-s{n:02d}\n")
CONF = OUT + '/c.toml'; open(CONF, 'w').write('[session]\nrestore_windows = true\n')
SESSION = OUT + '/session.json'
LOG = open(OUT + '/driftwm.log', 'ab')
REPORT = open(OUT + '/report.txt', 'w')
def rep(m): REPORT.write(m + '\n'); REPORT.flush(); print(m, flush=True)

def ipc(req, timeout=5.0):
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

proc = None
def start(monotonic_faked):
    global proc
    for f in glob.glob(RT + '/wayland-*') + glob.glob(RT + '/driftwm/*'):
        try: os.remove(f)
        except OSError: pass
    open(RC, 'w').write('+0')
    e = dict(ENVB); e.update({'LD_PRELOAD': FT, 'FAKETIME_TIMESTAMP_FILE': RC, 'FAKETIME_NO_CACHE': '1'})
    if not monotonic_faked: e['FAKETIME_DONT_FAKE_MONOTONIC'] = '1'
    proc = subprocess.Popen([DW, '--backend', 'winit', '--config', CONF, '--session-file', SESSION], env=e, stdout=LOG, stderr=LOG, cwd=OUT)
    t0 = time.time()
    while time.time() - t0 < 60:
        if proc.poll() is not None: return False
        if glob.glob(RT + '/driftwm/ipc-*.sock'):
            try: st(); return True
            except Exception: pass
        time.sleep(0.1)
    return False
def kill():
    try: proc.kill()
    except Exception: pass
    try: proc.wait(10)
    except Exception: pass

# seed: 5 stand-ins
assert start(False)
sock = [os.path.basename(p) for p in glob.glob(RT + '/wayland-*') if not p.endswith('.lock')][0]
e = dict(ENVB); e['WAYLAND_DISPLAY'] = sock
for n in range(1, 6): subprocess.Popen(['foot', f'--app-id=hubos-s{n:02d}', f'--title=s{n:02d}', 'sleep', '100000'], env=e, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, cwd=OUT)
for _ in range(100):
    ws = [w for w in st()['windows'] if w['app_id'].startswith('hubos-s')]
    if len(ws) == 5: break
    time.sleep(0.2)
for i, w in enumerate(ws): ipc({'Move': {'window': w['id'], 'to': [-800 + i * 400, 100 * i]}})
time.sleep(0.5)
for w in ws: ipc({'Suspend': w['id']})
time.sleep(7); kill(); time.sleep(0.5)

def saved_at():
    try: return json.load(open(SESSION))['saved_at']
    except Exception: return None
def entries():
    try: return {e['app_id']: (tuple(e['position']), tuple(e['size'])) for e in json.load(open(SESSION))['entries']}
    except Exception: return None

JUMPS = [('+1 h', '+3600'), ('-1 h', '-3600'), ('+10 years', '+315360000'), ('-30 years (to 1996)', '-946080000'), ('to 1970-01-01 00:00:10', '@1970-01-01 00:00:10'),
         ('to 2038-01-19 03:14:00 (just before 2^31)', '@2038-01-19 03:14:00'), ('to 2106-02-07 06:28:00 (just before 2^32)', '@2106-02-07 06:28:00'),
         ('to year 2300', '@2300-01-01 00:00:00'), ('+1 s', '+1'), ('-1 s', '-1'), ('back to real time', '+0')]
def phase(name, monotonic_faked):
    rep(f'\n=== {name} ===')
    if not start(monotonic_faked): rep('!!! start failed'); return
    t = time.time()
    standins = [w for w in st()['windows'] if w.get('suspended')]
    rep(f'started; restored stand-ins: {len(standins)}')
    for label, val in JUMPS:
        open(RC, 'w').write(val)
        time.sleep(0.4)
        res = {'jump': label}
        if proc.poll() is not None: rep(f'jump {label}: !!! COMPOSITOR DIED rc={proc.returncode}'); return
        # 1. IPC latency
        t0 = time.time()
        try: ws = [w for w in st()['windows'] if w.get('suspended')]; res['ipc_ms'] = round((time.time() - t0) * 1000, 1)
        except Exception as ex: res['ipc'] = 'FAILED ' + type(ex).__name__; rep(json.dumps(res)); continue
        # 2. debounced save: move a stand-in, wait for session.json to contain the new position
        w = ws[0]; target = [-1500 + int(time.time()) % 700, 777]
        before = entries()
        ipc({'Move': {'window': w['id'], 'to': target}})
        t1 = time.time(); done = None
        while time.time() - t1 < 12:
            en = entries()
            if en and en.get(w['app_id']) and list(en[w['app_id']][0]) == target: done = time.time() - t1; break
            time.sleep(0.05)
        res['save_latency_s'] = round(done, 2) if done is not None else 'NOT SAVED within 12 s'
        res['saved_at'] = saved_at()
        # 3. camera animation finishes?
        try:
            cam = [float(time.time() % 500), 50.0]
            ipc({'Camera': cam}); t2 = time.time(); reached = None
            while time.time() - t2 < 8:
                c = st()['camera']
                if abs(c[0] - cam[0]) < 1.5 and abs(c[1] - cam[1]) < 1.5: reached = time.time() - t2; break
                time.sleep(0.05)
            res['camera_reached_s'] = round(reached, 2) if reached is not None else 'NOT within 8 s'
        except Exception as ex: res['camera'] = 'ERR ' + type(ex).__name__
        rep(json.dumps(res))
    # rapid flapping +-0.5..2 s 40 times
    ok = True
    for i in range(40):
        open(RC, 'w').write(('+' if i % 2 == 0 else '-') + str(1 + i % 3)); time.sleep(0.1)
    try: st(); rep('flapping +-1..3 s x40: compositor alive, IPC answers')
    except Exception as ex: rep('flapping: IPC FAILED ' + type(ex).__name__)
    open(RC, 'w').write('+0')
    # kill -9 + restore while the clock is wrong
    open(RC, 'w').write('@2300-01-01 00:00:00'); time.sleep(0.3)
    kill(); time.sleep(0.5)
    ok2 = start(monotonic_faked)
    rep(f'kill -9 with the clock in year 2300, restart under real time: started={ok2}, stand-ins restored={len([w for w in st()["windows"] if w.get("suspended")]) if ok2 else "n/a"}')
    kill()

phase('Phase R: CLOCK_REALTIME jumps only (monotonic clock real)', False)
phase('Phase M: CLOCK_MONOTONIC faked as well (out of spec, stress test)', True)
rep('\nlog lines with panic: ' + str(sum(1 for l in open(OUT + '/driftwm.log', errors='replace') if 'panicked' in l)))
