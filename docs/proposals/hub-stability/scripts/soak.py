import os
#!/usr/bin/env python3
"""Soak driver for the nested driftwm (hub-stability research, 2026-10-05).
Keeps up to 20 foot windows, does random opens/closes/crashes/moves/resizes/focus/pan/zoom/
bookmark/suspend/relaunch, mouse input via xdotool, samples driftwm RSS/fds/threads/cpu and IPC latency.
Everything runs OUTSIDE the repo; ulimit -c 0 is set by the wrapper.
usage: soak.py SECONDS OUTDIR [seed]
"""
import json, os, random, signal, socket, subprocess, sys, time, glob

S = os.environ['HS_WORK']
DW = S + '/hubstab/target/release/driftwm'
RT = '/tmp/hsx'
DURATION = float(sys.argv[1]); OUT = sys.argv[2]
SEED = int(sys.argv[3]) if len(sys.argv) > 3 else 1
rnd = random.Random(SEED)
os.makedirs(OUT, exist_ok=True)
APPS = OUT + '/apps'
os.makedirs(APPS, exist_ok=True)
SESSION = OUT + '/session.json'
LOG = open(OUT + '/driftwm.log', 'ab')
EV = open(OUT + '/events.log', 'a')
CSV = open(OUT + '/samples.csv', 'a')
if CSV.tell() == 0:
    CSV.write('t,pid,rss_kb,hwm_kb,fds,threads,cpu_ticks,ipc_ms,windows,standins,restarts,foot_alive\n')

L = S + '/A/root/usr/lib/x86_64-linux-gnu'
ENV = dict(os.environ)
ENV.update({
    'XDG_RUNTIME_DIR': RT, 'DISPLAY': ':80', 'LD_LIBRARY_PATH': L,
    'LIBGL_ALWAYS_SOFTWARE': '1', 'MESA_LOADER_DRIVER_OVERRIDE': 'swrast',
    '__EGL_VENDOR_LIBRARY_DIRS': S + '/A/root/usr/share/glvnd/egl_vendor.d',
    'LIBGL_DRIVERS_PATH': L + '/dri', 'GBM_BACKENDS_PATH': L + '/gbm',
    'XDG_DATA_HOME': OUT + '/data', 'RUST_BACKTRACE': '1', 'RUST_LOG': 'info',
    'PATH': S + '/A/root/usr/bin:' + os.environ.get('PATH', ''),
    'XDG_STATE_HOME': OUT + '/state',
})
ENV.pop('WAYLAND_DISPLAY', None)
os.makedirs(RT, exist_ok=True); os.chmod(RT, 0o700)
os.makedirs(OUT + '/data/applications', exist_ok=True)

CMD = "i=0; while :; do echo line $i $(date +%T); i=$((i+1)); sleep 2; done"
for n in range(1, 21):
    with open(f'{OUT}/data/applications/hubos-s{n:02d}.desktop', 'w') as f:
        f.write(f"[Desktop Entry]\nType=Application\nName=s{n:02d}\nExec=foot --app-id=hubos-s{n:02d} --title=s{n:02d} sh -c '{CMD}'\nStartupWMClass=hubos-s{n:02d}\n")

CONF = OUT + '/soak.toml'
open(CONF, 'w').write('[session]\nrestore_windows = true\nrestore_bookmarks = true\n')

def ev(msg):
    EV.write(f'{time.strftime("%H:%M:%S")} {msg}\n'); EV.flush()

xvfb = None
def ensure_xvfb():
    global xvfb
    if os.path.exists('/tmp/.X11-unix/X80'):
        return
    xvfb = subprocess.Popen([S + '/A/root/usr/bin/Xvfb', ':80', '-screen', '0', '1920x1080x24', '-nolisten', 'tcp'],
                            env=ENV, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    time.sleep(2)

dw = None; restarts = 0; sock_name = None
def start_dw():
    global dw, sock_name
    for f in glob.glob(RT + '/wayland-*'):
        try: os.remove(f)
        except OSError: pass
    dw = subprocess.Popen([DW, '--backend', 'winit', '--config', CONF, '--session-file', SESSION],
                          env=ENV, stdout=LOG, stderr=LOG)
    t0 = time.time()
    while time.time() - t0 < 30:
        socks = [p for p in glob.glob(RT + '/wayland-*') if not p.endswith('.lock')]
        if socks and glob.glob(RT + '/driftwm/ipc-*.sock'):
            sock_name = os.path.basename(socks[0]); break
        time.sleep(0.2)
    ev(f'driftwm started pid={dw.pid} sock={sock_name}')

def ipc(req, timeout=5.0):
    p = glob.glob(RT + '/driftwm/ipc-*.sock')
    if not p: raise RuntimeError('no ipc socket')
    s = socket.socket(socket.AF_UNIX); s.settimeout(timeout)
    s.connect(p[0]); s.sendall((json.dumps(req) + '\n').encode())
    buf = b''
    while not buf.endswith(b'\n'):
        c = s.recv(65536)
        if not c: break
        buf += c
    s.close()
    return json.loads(buf)

feet = {}  # app_id -> Popen
stopped = {}
def spawn_foot(n):
    aid = f'hubos-s{n:02d}'
    if aid in feet and feet[aid].poll() is None: return
    e = dict(ENV); e['WAYLAND_DISPLAY'] = sock_name
    feet[aid] = subprocess.Popen(['foot', f'--app-id={aid}', f'--title=s{n:02d}', 'sh', '-c', CMD], env=e,
                                 stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

def state():
    r = ipc('State')
    return r['Ok']['State']

def xdo(*a):
    try:
        subprocess.run([S + '/A/root/usr/bin/xdotool', *a], env=ENV, timeout=5,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    except Exception: pass

def proc_stats(pid):
    try:
        st = open(f'/proc/{pid}/status').read()
        d = {}
        for k in ('VmRSS', 'VmHWM', 'Threads'):
            for ln in st.split('\n'):
                if ln.startswith(k + ':'): d[k] = int(ln.split()[1])
        fds = len(os.listdir(f'/proc/{pid}/fd'))
        f = open(f'/proc/{pid}/stat').read().rsplit(')', 1)[1].split()
        return d['VmRSS'], d['VmHWM'], fds, d['Threads'], int(f[11]) + int(f[12])
    except Exception:
        return None

def op():
    r = rnd.random()
    st = state()
    wins = [w for w in st['windows'] if w['app_id'].startswith('hubos-s')]
    live = [w for w in wins if not w.get('suspended')]
    if r < 0.30 and len(wins) < 20:
        free = [n for n in range(1, 21) if f'hubos-s{n:02d}' not in [w['app_id'] for w in wins]]
        if free: n = rnd.choice(free); spawn_foot(n); return f'open s{n:02d}'
    if r < 0.36 and wins:
        w = rnd.choice(wins); ipc({'Close': w['id']}); return f'close {w["app_id"]}'
    if r < 0.40 and feet:
        a = rnd.choice(list(feet)); p = feet[a]
        if p.poll() is None: p.kill(); return f'crash-client {a}'
    if r < 0.43 and feet:
        a = rnd.choice(list(feet)); p = feet[a]
        if p.poll() is None:
            os.kill(p.pid, signal.SIGSTOP); stopped[a] = time.time(); return f'freeze-client {a}'
    if r < 0.56 and wins:
        w = rnd.choice(wins); ipc({'Move': {'window': w['id'], 'to': [rnd.randint(-3000, 3000), rnd.randint(-2000, 2000)]}}); return f'move {w["id"]}'
    if r < 0.66 and wins:
        w = rnd.choice(wins); ipc({'Resize': {'window': w['id'], 'to': [rnd.randint(150, 900), rnd.randint(100, 700)]}}); return f'resize {w["id"]}'
    if r < 0.73 and wins:
        w = rnd.choice(wins); ipc({'Focus': w['id']}); return f'focus {w["id"]}'
    if r < 0.78:
        ipc({'Camera': [rnd.uniform(-2500, 2500), rnd.uniform(-1500, 1500)]}); return 'camera'
    if r < 0.82:
        ipc({'Zoom': rnd.choice([0.2, 0.4, 0.7, 1.0])}); return 'zoom'
    if r < 0.86:
        nm = 'b%d' % rnd.randint(0, 5)
        ipc({'Bookmark': {'name': nm, 'to': [rnd.uniform(-2000, 2000), rnd.uniform(-1000, 1000)], 'delete': False}}); return 'bookmark-set'
    if r < 0.88:
        ipc({'Bookmark': {'name': 'b%d' % rnd.randint(0, 5), 'to': None, 'delete': True}}); return 'bookmark-del'
    if r < 0.93 and live:
        w = rnd.choice(live); ipc({'Suspend': w['id']}); return f'suspend {w["app_id"]}'
    if r < 0.97:
        sus = [w for w in wins if w.get('suspended')]
        if sus: w = rnd.choice(sus); ipc({'Relaunch': w['id']}); return f'relaunch {w["app_id"]}'
    # mouse input into the nested window
    x, y = rnd.randint(50, 1850), rnd.randint(50, 1000)
    xdo('mousemove', str(x), str(y));
    k = rnd.random()
    if k < 0.4: xdo('click', rnd.choice(['1', '2', '3', '4', '5']))
    elif k < 0.6: xdo('keydown', 'Alt_L', 'mousedown', '1', 'mousemove_relative', '--', str(rnd.randint(-200, 200)), str(rnd.randint(-200, 200)), 'mouseup', '1', 'keyup', 'Alt_L')
    elif k < 0.8: xdo('key', rnd.choice(['a', 'Return', 'space', 'super+Left', 'alt+Tab']))
    return 'mouse/key'

ensure_xvfb()
start_dw()
t_start = time.time(); next_sample = 0; ops = 0; last_ok = time.time()
ev(f'soak start duration={DURATION}s seed={SEED}')
while time.time() - t_start < DURATION:
    now = time.time()
    if dw.poll() is not None:
        ev(f'!!! COMPOSITOR DIED rc={dw.returncode} after {now - t_start:.0f}s')
        restarts += 1
        for p in feet.values():
            if p.poll() is None: p.kill()
        feet.clear()
        time.sleep(1); start_dw()
        continue
    for a in list(stopped):
        if now - stopped[a] > 20:
            try: os.kill(feet[a].pid, signal.SIGCONT)
            except Exception: pass
            del stopped[a]
    try:
        what = op(); ops += 1
    except Exception as e:
        ev(f'op error: {type(e).__name__} {e}')
        what = None
    if now >= next_sample:
        next_sample = now + 20
        t0 = time.time()
        try:
            st = state(); ms = (time.time() - t0) * 1000
            nw = len([w for w in st['windows'] if w['app_id'].startswith('hubos-s')])
            ns = len([w for w in st['windows'] if w.get('suspended')])
        except Exception as e:
            ms = -1; nw = ns = -1; ev(f'IPC probe failed: {e}')
        ps = proc_stats(dw.pid)
        alive = sum(1 for p in feet.values() if p.poll() is None)
        if ps:
            CSV.write(f'{now - t_start:.0f},{dw.pid},{ps[0]},{ps[1]},{ps[2]},{ps[3]},{ps[4]},{ms:.1f},{nw},{ns},{restarts},{alive}\n'); CSV.flush()
    time.sleep(rnd.uniform(0.1, 0.9))
ev(f'soak end ops={ops} restarts={restarts}')
for p in feet.values():
    if p.poll() is None: p.kill()
dw.terminate()
try: dw.wait(10)
except Exception: dw.kill()
