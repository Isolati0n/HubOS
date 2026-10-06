#!/usr/bin/env python3
"""soak2.py - long soak for the bulletproof-compositor research (2026-10-05).
Nested driftwm (winit backend in Xvfb, software GL), up to 20 foot windows, random faults:
open / close / move / resize / focus / camera / zoom / bookmark / suspend / relaunch, client crash,
client freeze (SIGSTOP), misbehaving protocol clients (fuzz2.py bursts, no-read floods), config reload
attempts (valid, bad, garbage, deleted), layout save / apply (hubd-style snapshot via IPC),
compositor kill -9 + restore (quiescent = must be exactly equal; random moment = must equal a recent
snapshot), session.json corruption before a restart, hang detection with SIGSTOP / long freezes.
Records every 60 s: RSS, HWM, fds, threads, cpu, IPC latency, windows, load, counters.
usage: soak2.py SECONDS OUTDIR SEED DW_BINARY
Run with ulimit -c 0, nice -n 19, outside the repository.
"""
import json, os, random, signal, socket, subprocess, sys, time, glob, struct, collections, shutil

DURATION = float(sys.argv[1]); OUT = sys.argv[2]; SEED = int(sys.argv[3]); DW = sys.argv[4]
W = os.environ['W']
A = W + '/A'
RT = os.environ.get('SOAK_RT', '/tmp/bc4soak')
DISP = os.environ.get('SOAK_DISPLAY', ':90')
rnd = random.Random(SEED)
os.makedirs(OUT, exist_ok=True)
LOG = open(OUT + '/driftwm.log', 'ab')
EV = open(OUT + '/events.log', 'a')
CSV = open(OUT + '/samples.csv', 'a')
if CSV.tell() == 0:
    CSV.write('t,pid,rss_kb,hwm_kb,fds,threads,cpu_ticks,ipc_ms,windows,standins,starts,foot_alive,load1,xvfb_rss_kb,log_kb,session_bytes,session_entries\n')
SESSION = OUT + '/session.json'
CONF = OUT + '/soak.toml'
GOODCONF = '[session]\nrestore_windows = true\nrestore_bookmarks = true\n'
L = A + '/root/usr/lib/x86_64-linux-gnu'
ENV = dict(os.environ)
ENV.update({'XDG_RUNTIME_DIR': RT, 'DISPLAY': DISP, 'LD_LIBRARY_PATH': L, 'LIBGL_ALWAYS_SOFTWARE': '1', 'MESA_LOADER_DRIVER_OVERRIDE': 'swrast',
            '__EGL_VENDOR_LIBRARY_DIRS': A + '/root/usr/share/glvnd/egl_vendor.d', 'LIBGL_DRIVERS_PATH': L + '/dri', 'GBM_BACKENDS_PATH': L + '/gbm',
            'XDG_DATA_HOME': OUT + '/data', 'RUST_BACKTRACE': '1', 'RUST_LOG': 'info', 'PATH': A + '/root/usr/bin:' + os.environ.get('PATH', ''),
            'XDG_STATE_HOME': OUT + '/state'})
ENV.pop('WAYLAND_DISPLAY', None)
shutil.rmtree(RT, ignore_errors=True); os.makedirs(RT); os.chmod(RT, 0o700)
os.makedirs(OUT + '/data/applications', exist_ok=True)
os.makedirs(OUT + '/fuzz', exist_ok=True)
CMD = "i=0; while :; do echo line $i $(date +%T); i=$((i+1)); sleep 2; done"
for n in range(1, 21):
    open(f'{OUT}/data/applications/hubos-s{n:02d}.desktop', 'w').write(
        f"[Desktop Entry]\nType=Application\nName=s{n:02d}\nExec=foot --app-id=hubos-s{n:02d} --title=s{n:02d} sleep 100000\nStartupWMClass=hubos-s{n:02d}\n")
open(CONF, 'w').write(GOODCONF)
_r = subprocess.run([DW, '--check-config', '--config', CONF], env=ENV, capture_output=True, text=True)
assert 'Config OK' in (_r.stdout + _r.stderr), 'soak config is not valid: ' + _r.stdout + _r.stderr

T0 = time.time()
def now(): return time.time() - T0
def ev(msg):
    EV.write(f'{time.strftime("%m-%d %H:%M:%S")} t={now():.0f} {msg}\n'); EV.flush()

stats = collections.Counter()
xvfb = None
def ensure_xvfb():
    global xvfb
    if os.path.exists('/tmp/.X11-unix/X' + DISP[1:]): return
    # Xvfb in a private mount namespace where /usr/bin also holds xkbcomp (see xvfb.sh); lives as long as the soak
    xvfb = subprocess.Popen(['bash', W + '/xvfb.sh', DISP, '1920x1080x24'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, start_new_session=True)
    for _ in range(100):
        if os.path.exists('/tmp/.X11-unix/X' + DISP[1:]): break
        time.sleep(0.2)

dw = None; sock_name = None; starts = 0; expected_kill = False
def start_dw():
    global dw, sock_name, starts
    for f in glob.glob(RT + '/wayland-*') + glob.glob(RT + '/driftwm/*'):
        try: os.remove(f)
        except OSError: pass
    dw = subprocess.Popen([DW, '--backend', 'winit', '--config', CONF, '--session-file', SESSION], env=ENV, stdout=LOG, stderr=LOG, cwd=OUT)
    starts += 1
    t0 = time.time()
    while time.time() - t0 < 40:
        if dw.poll() is not None: break
        socks = [p for p in glob.glob(RT + '/wayland-*') if not p.endswith('.lock')]
        if socks and glob.glob(RT + '/driftwm/ipc-*.sock'):
            sock_name = os.path.basename(socks[0]); break
        time.sleep(0.1)
    ev(f'driftwm started pid={dw.pid} sock={sock_name} startup={time.time()-t0:.2f}s')

def ipc(req, timeout=5.0):
    p = glob.glob(RT + '/driftwm/ipc-*.sock')
    if not p: raise RuntimeError('no ipc socket')
    s = socket.socket(socket.AF_UNIX); s.settimeout(timeout)
    try:
        s.connect(p[0]); s.sendall((json.dumps(req) + '\n').encode())
        buf = b''
        while not buf.endswith(b'\n'):
            c = s.recv(1 << 20)
            if not c: break
            buf += c
    finally: s.close()
    return json.loads(buf)

def wl_sync(timeout=3.0):
    """Wayland round trip: wl_display.sync, wait for the callback done event."""
    s = socket.socket(socket.AF_UNIX); s.settimeout(timeout)
    try:
        s.connect(RT + '/' + sock_name)
        s.sendall(struct.pack('<IIII', 1, (12 << 16) | 0, 2, 0)[:12])
        buf = b''; t0 = time.time()
        while time.time() - t0 < timeout:
            buf += s.recv(4096)
            if len(buf) >= 12 and struct.unpack('<I', buf[:4])[0] == 2: return True
        return False
    except Exception: return False
    finally: s.close()

def state():
    return ipc('State')['Ok']['State']

feet = {}; stopped = {}
def spawn_foot(n):
    aid = f'hubos-s{n:02d}'
    if aid in feet and feet[aid].poll() is None: return
    e = dict(ENV); e['WAYLAND_DISPLAY'] = sock_name
    feet[aid] = subprocess.Popen(['foot', f'--app-id={aid}', f'--title=s{n:02d}', 'sh', '-c', CMD], env=e, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, cwd=OUT)

def xdo(*a):
    try: subprocess.run([A + '/root/usr/bin/xdotool', *a], env=ENV, timeout=5, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    except Exception: pass

def proc_stats(pid):
    try:
        st = open(f'/proc/{pid}/status').read(); d = {}
        for k in ('VmRSS', 'VmHWM', 'Threads'):
            for ln in st.split('\n'):
                if ln.startswith(k + ':'): d[k] = int(ln.split()[1])
        fds = len(os.listdir(f'/proc/{pid}/fd'))
        f = open(f'/proc/{pid}/stat').read().rsplit(')', 1)[1].split()
        return d['VmRSS'], d['VmHWM'], fds, d['Threads'], int(f[11]) + int(f[12])
    except Exception: return None

def rss_of(proc):
    try: return proc_stats(proc.pid)[0]
    except Exception: return 0

ring = collections.deque(maxlen=60)   # (t, {app_id: (pos,size)}, [app_id order])
def snap(st):
    ws = [w for w in st['windows'] if w['app_id'].startswith('hubos-s')]
    return {w['app_id']: (tuple(w['position']), tuple(w['size'])) for w in ws}, [w['app_id'] for w in ws]

layouts = {}
def layout_save():
    st = state(); m, order = snap(st); layouts['L%d' % rnd.randint(0, 3)] = m; return 'layout-save'
def layout_apply():
    if not layouts: return 'layout-apply(none)'
    k = rnd.choice(list(layouts)); m = layouts[k]; st = state()
    for w in st['windows']:
        if w['app_id'] in m:
            (p, s) = m[w['app_id']]
            ipc({'Move': {'window': w['id'], 'to': list(p)}}); ipc({'Resize': {'window': w['id'], 'to': list(s)}})
    return 'layout-apply'

def kill_dw(why):
    global expected_kill
    expected_kill = True
    try: dw.send_signal(signal.SIGKILL)
    except Exception: pass
    try: dw.wait(10)
    except Exception: pass
    ev(f'kill -9 compositor ({why})')

def reap_clients():
    for p in feet.values():
        if p.poll() is None:
            try: p.kill()
            except Exception: pass
    for p in feet.values():
        try: p.wait(5)
        except Exception: pass
    feet.clear(); stopped.clear()

def restart_and_verify(pre_snap, pre_order, quiescent, tkill, why):
    """after a death of the compositor: restart after 1.0 s, check session file, stand-ins, relaunch and geometry."""
    global expected_kill
    reap_clients()
    # session file validity right after the death (before restart rewrites it)
    sess_ok = None
    try:
        j = json.load(open(SESSION)); sess_ok = (j.get('version') == 2)
    except FileNotFoundError: sess_ok = 'missing'
    except Exception as e: sess_ok = 'INVALID:' + type(e).__name__
    time.sleep(1.0)
    t_restart = time.time()
    start_dw()
    if dw.poll() is not None:
        ev(f'!!! restart failed rc={dw.returncode}'); stats['restart_failed'] += 1; return
    t_ipc = None
    for _ in range(100):
        try:
            st = state(); t_ipc = time.time() - t_restart; break
        except Exception: time.sleep(0.1)
    if t_ipc is None:
        ev('!!! IPC not answering after restart'); stats['restart_ipc_fail'] += 1; return
    standins = [w for w in st['windows'] if w.get('suspended')]
    got = {w['app_id']: (tuple(w['position']), tuple(w['size'])) for w in standins}
    got_order = [w['app_id'] for w in standins]
    res = []
    if quiescent:
        exact = (got == pre_snap)
        order_ok = (got_order == pre_order)
        res.append(f'quiescent exact={exact} order_ok={order_ok} expected={len(pre_snap)} got={len(got)}')
        stats['q_kills'] += 1
        if not exact: stats['q_inexact'] += 1; ev('INEXACT diff: ' + json.dumps({'missing': [k for k in pre_snap if k not in got], 'extra': [k for k in got if k not in pre_snap],
                                                                                'changed': {k: [pre_snap[k], got[k]] for k in got if k in pre_snap and got[k] != pre_snap[k]}})[:1500])
        if not order_ok: stats['q_order_diff'] += 1
    else:
        # every restored window must equal what some recent snapshot had for that app
        bad = []
        recent = [r for r in ring if tkill - r[0] < 12]
        for k, v in got.items():
            if not any(k in r[1] and r[1][k] == v for r in recent): bad.append(k)
        res.append(f'random-moment kill: restored={len(got)} not-matching-any-recent-snapshot={len(bad)}')
        stats['r_kills'] += 1
        if bad: stats['r_mismatch'] += 1; ev('RANDOM-KILL MISMATCH ' + json.dumps(bad)[:300])
    # relaunch every stand-in and compare live geometry
    t_rl = time.time(); live_ok = 0; live_bad = 0
    for w in standins[:20]:
        try: ipc({'Relaunch': w['id']})
        except Exception as e: ev(f'relaunch error {e}')
    deadline = time.time() + 12
    while time.time() < deadline:
        try: st2 = state()
        except Exception: break
        live = {w['app_id']: (tuple(w['position']), tuple(w['size'])) for w in st2['windows'] if w['app_id'].startswith('hubos-s') and not w.get('suspended')}
        if len(live) >= len(got): break
        time.sleep(0.2)
    time.sleep(3.0)   # let the new windows settle in their stand-ins' places before comparing
    try:
        live = {w['app_id']: (tuple(w['position']), tuple(w['size'])) for w in state()['windows'] if w['app_id'].startswith('hubos-s') and not w.get('suspended')}
    except Exception: pass
    try:
        for k, v in got.items():
            if k in live and live[k] == v: live_ok += 1
            else: live_bad += 1
    except NameError: live_bad = len(got)
    stats['live_ok'] += live_ok; stats['live_bad'] += live_bad
    res.append(f'relaunched live same-geometry={live_ok} different/missing={live_bad} in {time.time()-t_rl:.1f}s; ipc back after {t_ipc:.2f}s; session.json before restart: {sess_ok}')
    if str(sess_ok).startswith('INVALID'): stats['session_invalid'] += 1
    expected_kill = False
    ev(f'RESTORE ({why}): ' + '; '.join(res))

def snapshot_now():
    st = state(); m, o = snap(st); return m, o

def op_kill(quiescent):
    global expected_kill
    if quiescent:
        time.sleep(7)   # > 5 s camera debounce: the saved file is up to date
        m, o = snapshot_now()
        if not m: return 'kill(skip: no windows)'
    else:
        m, o = {}, []
    tk = now()
    kill_dw('quiescent' if quiescent else 'random moment')
    restart_and_verify(m, o, quiescent, tk, 'quiescent' if quiescent else 'random')
    return 'kill+restore'

def op_corrupt_session():
    global expected_kill
    time.sleep(7)
    kill_dw('before session corruption')
    reap_clients()
    kind = rnd.choice(['truncated', 'empty', 'garbage', 'nul', 'huge', 'wrongversion', 'dir', 'tmpleft'])
    good = open(SESSION, 'rb').read() if os.path.isfile(SESSION) else b'{}'
    try:
        if kind == 'dir':
            os.rename(SESSION, SESSION + '.keep'); os.mkdir(SESSION)
        elif kind == 'tmpleft': open(SESSION + '.tmp', 'wb').write(good[:len(good) // 2])
        else:
            data = {'truncated': good[:len(good) // 2], 'empty': b'', 'garbage': bytes(rnd.getrandbits(8) for _ in range(5000)), 'nul': b'\0' * 4096,
                    'huge': b'{"version":2,"saved_at":0,"entries":[' + b','.join([b'{"id":1,"app_id":"hubos-s01","desktop_id":"hubos-s01.desktop","display_name":"x","position":[0,0],"size":[400,300],"origin":"explicit"}'] * 30000) + b'],"outputs":{}}',
                    'wrongversion': good.replace(b'"version": 2', b'"version": 99')}[kind]
            open(SESSION, 'wb').write(data)
    except Exception as e: ev(f'corrupt setup error {e}')
    time.sleep(1)
    start_dw(); ok = False
    for _ in range(100):
        try: state(); ok = True; break
        except Exception: time.sleep(0.2)
    alive = dw.poll() is None
    q = len(glob.glob(SESSION + '.corrupt.*')) + len(glob.glob(SESSION + '.unreadable.*'))
    ev(f'SESSION-CORRUPT {kind}: compositor alive={alive} ipc_ok={ok} quarantined_files={q}')
    stats['corrupt_tests'] += 1
    if not (alive and ok): stats['corrupt_fail'] += 1
    if kind == 'dir':
        try: os.rmdir(SESSION); os.rename(SESSION + '.keep', SESSION)
        except Exception: pass
    for f in glob.glob(SESSION + '.corrupt.*') + glob.glob(SESSION + '.unreadable.*'): os.remove(f)
    return 'corrupt-session ' + kind

def op_config():
    kinds = ['valid', 'badcolor', 'multibyte', 'garbage', 'empty', 'deleted', 'badtoml', 'bigfile', 'unknownkeys', 'partial']
    k = rnd.choice(kinds)
    body = {'valid': GOODCONF + '\n[effects]\nanimation_speed = 0.8\n', 'badcolor': GOODCONF + '\n[decorations]\nbg_color = "#zzzzzz"\n',
            'multibyte': GOODCONF + '\n[decorations]\nbg_color = "#aéaaa"\n', 'garbage': None, 'empty': '', 'deleted': None,
            'badtoml': '[[[[ = =\n', 'bigfile': GOODCONF + '\n' + '# pad\n' * 200000, 'unknownkeys': GOODCONF + '\nnonsense = 1\n[foo]\nbar = [1,2\n', 'partial': GOODCONF[:40]}[k]
    try:
        if k == 'garbage': open(CONF, 'wb').write(bytes(rnd.getrandbits(8) for _ in range(3000)))
        elif k == 'deleted': os.remove(CONF)
        elif k == 'partial':
            f = open(CONF, 'w'); f.write(body[:10]); f.flush(); time.sleep(0.05); f.write(body[10:]); f.close()
        else: open(CONF, 'w').write(body)
    except Exception as e: ev(f'config write error {e}')
    time.sleep(1.5)
    ok = dw.poll() is None
    if ok:
        try: state()
        except Exception: ok = False
    stats['config_tests'] += 1
    if not ok: stats['config_fail'] += 1; ev(f'!!! config {k}: compositor not answering/dead')
    open(CONF, 'w').write(GOODCONF)
    time.sleep(1.0)
    # (found by configwipe.py: while a config without restore_windows is active, the next session write drops the live
    # windows from session.json, and restoring the config does not bring them back until a window changes: nudge one)
    try:
        ws = [w for w in state()['windows'] if w['app_id'].startswith('hubos-s') and not w.get('suspended')]
        if ws:
            w = rnd.choice(ws); ipc({'Move': {'window': w['id'], 'to': [w['position'][0] + 1, w['position'][1]]}})
    except Exception: pass
    ev(f'config test {k}: compositor_ok={ok}')
    return 'config-' + k

fz = None
def op_fuzz():
    global fz
    if fz is not None and fz.poll() is None: return 'fuzz(busy)'
    mode = rnd.choice(['', 'noread:100', 'garbage:50,torn:50', 'normal:100', 'fdflood:100'])
    e = dict(os.environ); e['W'] = W
    if mode: e['FUZZ_MODES'] = mode
    e['FUZZ_SKIP'] = ''
    fz = subprocess.Popen([sys.executable, W + '/fuzz2.py', RT + '/' + sock_name, str(dw.pid), str(rnd.choice([6, 12, 20])), OUT + '/fuzz', str(rnd.randint(1, 10**6)), W + '/proto/all'],
                          env=e, stdout=open(OUT + '/fuzz/last.out', 'ab'), stderr=subprocess.STDOUT, cwd=OUT)
    stats['fuzz_bursts'] += 1
    return 'fuzz-burst ' + (mode or 'default')

def op():
    r = rnd.random(); st = state()
    m, o = snap(st); ring.append((now(), m, o))
    wins = [w for w in st['windows'] if w['app_id'].startswith('hubos-s')]
    live = [w for w in wins if not w.get('suspended')]
    if r < 0.26 and len(wins) < 20:
        free = [n for n in range(1, 21) if f'hubos-s{n:02d}' not in [w['app_id'] for w in wins]]
        if free: n = rnd.choice(free); spawn_foot(n); return f'open s{n:02d}'
    if r < 0.31 and wins:
        w = rnd.choice(wins); ipc({'Close': w['id']}); return 'close'
    if r < 0.34 and feet:
        a = rnd.choice(list(feet)); p = feet[a]
        if p.poll() is None: p.kill(); return 'crash-client'
    if r < 0.36 and feet:
        a = rnd.choice(list(feet)); p = feet[a]
        if p.poll() is None and a not in stopped: os.kill(p.pid, signal.SIGSTOP); stopped[a] = time.time(); return 'freeze-client'
    if r < 0.48 and wins:
        w = rnd.choice(wins); ipc({'Move': {'window': w['id'], 'to': [rnd.randint(-3000, 3000), rnd.randint(-2000, 2000)]}}); return 'move'
    if r < 0.57 and wins:
        w = rnd.choice(wins); ipc({'Resize': {'window': w['id'], 'to': [rnd.randint(150, 900), rnd.randint(100, 700)]}}); return 'resize'
    if r < 0.63 and wins:
        w = rnd.choice(wins); ipc({'Focus': w['id']}); return 'focus'
    if r < 0.67: ipc({'Camera': [rnd.uniform(-2500, 2500), rnd.uniform(-1500, 1500)]}); return 'camera'
    if r < 0.70: ipc({'Zoom': rnd.choice([0.2, 0.4, 0.7, 1.0])}); return 'zoom'
    if r < 0.73: ipc({'Bookmark': {'name': 'b%d' % rnd.randint(0, 5), 'to': [rnd.uniform(-2000, 2000), rnd.uniform(-1000, 1000)], 'delete': False}}); return 'bookmark'
    if r < 0.75: ipc({'Bookmark': {'name': 'b%d' % rnd.randint(0, 5), 'to': None, 'delete': True}}); return 'bookmark-del'
    if r < 0.79 and live: w = rnd.choice(live); ipc({'Suspend': w['id']}); return 'suspend'
    if r < 0.83:
        sus = [w for w in wins if w.get('suspended')]
        if sus: ipc({'Relaunch': rnd.choice(sus)['id']}); return 'relaunch'
    if r < 0.85: return layout_save()
    if r < 0.87: return layout_apply()
    if r < 0.89: return op_fuzz()
    if r < 0.895: return op_config()
    x, y = rnd.randint(50, 1850), rnd.randint(50, 1000)
    xdo('mousemove', str(x), str(y)); k = rnd.random()
    if k < 0.4: xdo('click', rnd.choice(['1', '2', '3', '4', '5']))
    elif k < 0.6: xdo('keydown', 'Alt_L', 'mousedown', '1', 'mousemove_relative', '--', str(rnd.randint(-200, 200)), str(rnd.randint(-200, 200)), 'mouseup', '1', 'keyup', 'Alt_L')
    elif k < 0.8: xdo('key', rnd.choice(['a', 'Return', 'space', 'super+Left', 'alt+Tab']))
    return 'mouse/key'

# ------------------------------------------------------------------ main
ensure_xvfb(); start_dw()
ev(f'soak2 start duration={DURATION}s seed={SEED} binary={DW}')
next_sample = 0; ops = 0; fail_since = None
next_kill_q = now() + rnd.uniform(300, 900); next_kill_r = now() + rnd.uniform(600, 1500); next_corrupt = now() + rnd.uniform(1800, 3600)
next_counters = now() + 600; next_hang_probe = now() + 30; next_config = now() + 120
if os.environ.get('SOAK_FAST'):
    next_kill_q = now() + 30; next_kill_r = now() + 60; next_corrupt = now() + 90; next_config = now() + 15; next_counters = now() + 20
while now() < DURATION:
    t = now()
    if dw.poll() is not None:
        rc = dw.returncode
        ev(f'!!! UNEXPECTED COMPOSITOR EXIT rc={rc} (not injected) at t={t:.0f}'); stats['UNEXPECTED_EXITS'] += 1
        try: open(OUT + f'/crash-{int(t)}.log', 'wb').write(open(OUT + '/driftwm.log', 'rb').read()[-30000:])
        except Exception: pass
        restart_and_verify({}, [], False, t, 'after unexpected exit')
        continue
    for a in list(stopped):
        if time.time() - stopped[a] > 25:
            try: os.kill(feet[a].pid, signal.SIGCONT)
            except Exception: pass
            del stopped[a]
    try:
        if t >= next_kill_q: next_kill_q = t + rnd.uniform(600, 1500); what = op_kill(True)
        elif t >= next_kill_r: next_kill_r = t + rnd.uniform(400, 1200); what = op_kill(False)
        elif t >= next_corrupt: next_corrupt = t + rnd.uniform(1800, 3600); what = op_corrupt_session()
        elif t >= next_config: next_config = t + rnd.uniform(60, 240); what = op_config()
        else: what = op()
        ops += 1; stats['ops'] += 1
    except Exception as e:
        stats['op_errors'] += 1; what = None
        if dw.poll() is None: ev(f'op error: {type(e).__name__} {str(e)[:100]}')
    # hang probe (outside the compositor): both probes must fail for 30 s
    if t >= next_hang_probe and dw.poll() is None:
        next_hang_probe = t + 10
        ok_ipc = True
        try: state()
        except Exception: ok_ipc = False
        ok_wl = wl_sync() if ok_ipc is False or True else True
        if not ok_ipc and not ok_wl:
            fail_since = fail_since or t
            if t - fail_since >= 30:
                ev(f'!!! HANG detected (both probes failing for {t - fail_since:.0f}s): evidence ' + open(f'/proc/{dw.pid}/status').read().split('\n')[2])
                stats['HANGS'] += 1; fail_since = None
                m, o = ({}, []); kill_dw('hang'); restart_and_verify(m, o, False, t, 'after hang kill')
        else:
            if fail_since: ev(f'probe failure cleared after {t - fail_since:.1f}s'); stats['probe_blips'] += 1
            fail_since = None
    if t >= next_counters and dw.poll() is None:
        next_counters = t + 600
        try: ev('debug-counters ' + json.dumps(ipc('DebugCounters')['Ok']['DebugCounters']))
        except Exception: pass
    if t >= next_sample:
        next_sample = t + 60
        t0 = time.time(); ms = -1; nw = ns = -1
        try:
            st = state(); ms = (time.time() - t0) * 1000
            nw = len([w for w in st['windows'] if w['app_id'].startswith('hubos-s')]); ns = len([w for w in st['windows'] if w.get('suspended')])
        except Exception as e: ev(f'IPC probe failed at sample: {type(e).__name__}')
        ps = proc_stats(dw.pid)
        alive = sum(1 for p in feet.values() if p.poll() is None)
        try: logkb = os.path.getsize(OUT + '/driftwm.log') // 1024
        except Exception: logkb = 0
        if logkb > 300000: LOG.truncate(0); ev('driftwm.log truncated (size cap 300 MB)')
        try:
            sb = os.path.getsize(SESSION); se = len(json.load(open(SESSION))['entries'])
        except Exception: sb = se = -1
        if ps:
            CSV.write(f'{t:.0f},{dw.pid},{ps[0]},{ps[1]},{ps[2]},{ps[3]},{ps[4]},{ms:.1f},{nw},{ns},{starts},{alive},{os.getloadavg()[0]:.2f},{rss_of(xvfb) if xvfb else 0},{logkb},{sb},{se}\n'); CSV.flush()
        json.dump(dict(stats), open(OUT + '/stats.json', 'w'))
    time.sleep(rnd.uniform(0.3, 1.5))
ev(f'soak2 end ops={ops} stats={dict(stats)}')
json.dump(dict(stats), open(OUT + '/stats.json', 'w'))
reap_clients()
dw.terminate()
try: dw.wait(10)
except Exception: dw.kill()
if xvfb:
    xvfb.terminate()
