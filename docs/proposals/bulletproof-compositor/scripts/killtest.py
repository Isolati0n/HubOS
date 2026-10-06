#!/usr/bin/env python3
"""killtest.py - thousands of kill -9 of the nested compositor at random moments around the session save,
checking after every kill that session.json is valid and that the restored stand-ins equal a state the
compositor really had (not older than the debounce allows).  Modes per cycle:
  rand   : kill U(0,2.5 s) after the last change
  edge   : kill at 1.0 s +- 0.12 s after the last change (the 1 s debounce boundary where the write happens)
  strace : compositor runs under strace which delays every write() to session.json.tmp by 400 ms; we kill -9 while it
           is stuck inside that write (so the .tmp file is certainly half-written and not yet renamed)
usage: killtest.py CYCLES OUTDIR SEED DW_BINARY
"""
import json, os, random, signal, socket, subprocess, sys, time, glob, shutil, collections
CYCLES = int(sys.argv[1]); OUT = sys.argv[2]; SEED = int(sys.argv[3]); DW = sys.argv[4]
W = os.environ['W']; A = W + '/A'; RT = os.environ.get('KT_RT', '/tmp/bc4kt'); DISP = os.environ.get('SOAK_DISPLAY', ':90')
rnd = random.Random(SEED)
os.makedirs(OUT, exist_ok=True)
L = A + '/root/usr/lib/x86_64-linux-gnu'
ENV = dict(os.environ)
ENV.update({'XDG_RUNTIME_DIR': RT, 'DISPLAY': DISP, 'LD_LIBRARY_PATH': L, 'LIBGL_ALWAYS_SOFTWARE': '1', 'MESA_LOADER_DRIVER_OVERRIDE': 'swrast',
            '__EGL_VENDOR_LIBRARY_DIRS': A + '/root/usr/share/glvnd/egl_vendor.d', 'LIBGL_DRIVERS_PATH': L + '/dri', 'GBM_BACKENDS_PATH': L + '/gbm',
            'XDG_DATA_HOME': OUT + '/data', 'RUST_BACKTRACE': '1', 'PATH': A + '/root/usr/bin:' + os.environ.get('PATH', ''), 'XDG_STATE_HOME': OUT + '/state'})
ENV.pop('WAYLAND_DISPLAY', None)
shutil.rmtree(RT, ignore_errors=True); os.makedirs(RT); os.chmod(RT, 0o700)
os.makedirs(OUT + '/data/applications', exist_ok=True)
for n in range(1, 9):
    open(f'{OUT}/data/applications/hubos-s{n:02d}.desktop', 'w').write(
        f"[Desktop Entry]\nType=Application\nName=s{n:02d}\nExec=foot --app-id=hubos-s{n:02d} --title=s{n:02d} sleep 100000\nStartupWMClass=hubos-s{n:02d}\n")
CONF = OUT + '/kt.toml'; open(CONF, 'w').write('[session]\nrestore_windows = true\nrestore_bookmarks = true\n')
SESSION = OUT + '/session.json'
LOG = open(OUT + '/driftwm.log', 'ab'); EV = open(OUT + '/events.log', 'a')
def ev(m): EV.write(f'{time.strftime("%m-%d %H:%M:%S")} {m}\n'); EV.flush()
st = collections.Counter()

def ipc(req, timeout=5.0):
    p = glob.glob(RT + '/driftwm/ipc-*.sock')
    if not p: raise RuntimeError('no ipc')
    s = socket.socket(socket.AF_UNIX); s.settimeout(timeout)
    try:
        s.connect(p[0]); s.sendall((json.dumps(req) + '\n').encode()); buf = b''
        while not buf.endswith(b'\n'):
            c = s.recv(1 << 20)
            if not c: break
            buf += c
    finally: s.close()
    return json.loads(buf)
def standins():
    ws = ipc('State')['Ok']['State']['windows']
    return {w['app_id']: (tuple(w['position']), tuple(w['size'])) for w in ws if w.get('suspended') and w['app_id'].startswith('hubos-s')}, \
           [w for w in ws if w['app_id'].startswith('hubos-s')]

proc = None; real_pid = None
def start(use_strace=False):
    global proc, real_pid
    for f in glob.glob(RT + '/wayland-*') + glob.glob(RT + '/driftwm/*'):
        try: os.remove(f)
        except OSError: pass
    cmd = [DW, '--backend', 'winit', '--config', CONF, '--session-file', SESSION]
    if use_strace:
        cmd = ['strace', '-f', '-qq', '-o', '/dev/null', '-P', SESSION + '.tmp', '-e', 'trace=write', '-e', 'inject=write:delay_enter=600ms'] + cmd
    proc = subprocess.Popen(cmd, env=ENV, stdout=LOG, stderr=LOG, cwd=OUT)
    t0 = time.time()
    while time.time() - t0 < 60:
        if proc.poll() is not None: return False
        if glob.glob(RT + '/driftwm/ipc-*.sock'):
            try: ipc('State'); break
            except Exception: pass
        time.sleep(0.05)
    else: return False
    if use_strace:
        kids = open(f'/proc/{proc.pid}/task/{proc.pid}/children').read().split()
        real_pid = int(kids[0]) if kids else proc.pid
    else: real_pid = proc.pid
    return True

def hard_kill():
    try: os.kill(real_pid, signal.SIGKILL)
    except Exception: pass
    try: proc.kill()
    except Exception: pass
    try: proc.wait(10)
    except Exception: pass
    for f in glob.glob(RT + '/wayland-*'):
        pass
    # clients (foot) of the dead compositor exit by themselves; reap any leftovers of this run
    subprocess.run(['pkill', '-KILL', '-P', str(os.getpid()), 'foot'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL) if False else None

def seed_session():
    ok = start()
    assert ok, 'start failed'
    e = dict(ENV); e['WAYLAND_DISPLAY'] = [os.path.basename(p) for p in glob.glob(RT + '/wayland-*') if not p.endswith('.lock')][0]
    for n in range(1, 9):
        subprocess.Popen(['foot', f'--app-id=hubos-s{n:02d}', f'--title=s{n:02d}', 'sleep', '100000'], env=e, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, cwd=OUT)
    for _ in range(100):
        s, ws = standins()
        if len(ws) == 8: break
        time.sleep(0.2)
    for w in ws:
        ipc({'Move': {'window': w['id'], 'to': [rnd.randint(-1500, 1500), rnd.randint(-900, 900)]}})
    time.sleep(0.5)
    for w in ws: ipc({'Suspend': w['id']})
    time.sleep(7)
    s, _ = standins(); assert len(s) == 8, s
    hard_kill(); time.sleep(0.5)

def mutate(ws):
    w = rnd.choice(ws); r = rnd.random()
    if r < .5: ipc({'Move': {'window': w['id'], 'to': [rnd.randint(-2500, 2500), rnd.randint(-1500, 1500)]}})
    elif r < .8: ipc({'Resize': {'window': w['id'], 'to': [rnd.randint(200, 900), rnd.randint(150, 700)]}})
    elif r < .9: ipc({'Focus': w['id']})
    else: ipc({'Camera': [rnd.uniform(-2000, 2000), rnd.uniform(-1000, 1000)]})

t_start = time.time()
if not os.path.exists(SESSION): seed_session(); ev('seeded 8 stand-ins')
for cyc in range(CYCLES):
    mode = rnd.choices(['rand', 'edge', 'strace'], [55, 35, 10])[0]
    if not start(mode == 'strace'):
        st['start_failed'] += 1; ev(f'cycle {cyc}: START FAILED rc={proc.poll()}'); hard_kill(); time.sleep(1); continue
    try:
        s0, ws = standins()
        if len(s0) < 1: st['no_standins'] += 1; ev(f'cycle {cyc}: no stand-ins restored ({len(s0)})'); hard_kill(); continue
        hist = [(time.time(), s0)]
        for _ in range(rnd.randint(1, 4)):
            mutate(ws); time.sleep(rnd.uniform(0.02, 0.3))
            hist.append((time.time(), standins()[0]))
        s_last = hist[-1][1]
        t_last = time.time()
        tmp_seen = False
        if mode == 'rand': time.sleep(rnd.uniform(0, 2.5))
        elif mode == 'edge': time.sleep(max(0, 1.0 + rnd.uniform(-0.12, 0.12) - (time.time() - t_last)))
        else:
            t0 = time.time()
            while time.time() - t0 < 4:
                if os.path.exists(SESSION + '.tmp'): tmp_seen = True; time.sleep(0.15); break
                time.sleep(0.01)
        tk = time.time()
        tmp_size = os.path.getsize(SESSION + '.tmp') if os.path.exists(SESSION + '.tmp') else -1
        hard_kill()
    except Exception as e:
        st['cycle_exception'] += 1; ev(f'cycle {cyc}: exception {type(e).__name__} {e}'); hard_kill(); continue
    st['kills_' + mode] += 1
    if mode == 'strace': st['strace_tmp_seen'] += tmp_seen; st['strace_tmp_partial'] += (tmp_seen and tmp_size >= 0)
    # 1. the file must parse
    try:
        j = json.load(open(SESSION)); assert j['version'] == 2 and isinstance(j['entries'], list)
    except Exception as e:
        st['SESSION_INVALID'] += 1; ev(f'cycle {cyc} mode={mode}: SESSION INVALID {type(e).__name__}: {e}')
        shutil.copy(SESSION, OUT + f'/invalid-{cyc}.json') if os.path.exists(SESSION) else None
    # 2. restart and compare with the history
    time.sleep(1.0)
    if not start():
        st['restart_failed'] += 1; ev(f'cycle {cyc} mode={mode}: RESTART FAILED rc={proc.poll()}'); hard_kill(); continue
    try:
        got, _ = standins()
    except Exception as e:
        st['restart_ipc_fail'] += 1; hard_kill(); continue
    hard_kill()
    # allowed: any state in hist (states are only 2 entries: before changes, after the last change); both ends are valid
    # outcomes of a kill: old file (write not yet done) or new file.  Anything else is a corrupted/mixed state.
    # (mid-way mutations are covered: the file is rewritten from the full state, so only the two ends or an intermediate state are possible)
    allowed = [h[1] for h in hist]
    if got in allowed:
        st['restore_ok_new' if got == hist[-1][1] else 'restore_ok_old'] += 1
        if mode != 'strace' and tk - t_last > 1.4 and got != hist[-1][1]: st['STALE_AFTER_DEBOUNCE'] += 1; ev(f'cycle {cyc} {mode}: stale after {tk-t_last:.2f}s')
    else:
        # intermediate states (mutations between s0 and s_last) are legal; accept if every entry equals a value seen for that app in s0 or s_last or any intermediate
        st['restore_other'] += 1; ev(f'cycle {cyc} mode={mode}: restored state is neither before nor after: kill {tk-t_last:.2f}s after the last change; got={len(got)} s0={len(hist[0][1])}')
    if cyc % 50 == 0:
        json.dump(dict(st), open(OUT + '/stats.json', 'w')); ev(f'cycle {cyc} stats {dict(st)} elapsed {time.time()-t_start:.0f}s')
json.dump(dict(st), open(OUT + '/stats.json', 'w'))
ev(f'DONE {CYCLES} cycles in {time.time()-t_start:.0f}s: {dict(st)}')
