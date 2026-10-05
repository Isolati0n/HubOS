import os
#!/usr/bin/env python3
"""Plan A test: nested driftwm + real hubd + 20 fake machines (fakenode) + foot viewers.
kill -9 the compositor, let a supervisor restart it (s6-like delay), then relaunch the viewers via the stand-ins.
usage: planA.py OUTDIR MODE [DWBIN] [RELAUNCH=all|stagger|hubd-direct]
MODE: kill9 | sigstop | busyloop | deadlock | panic
Everything is outside the repository. ulimit -c 0 set by caller."""
import json, os, signal, socket, subprocess, sys, threading, time, glob

S = os.environ['HS_WORK']
H = S + '/hubstab'
OUT = sys.argv[1]; MODE = sys.argv[2]
DWBIN = sys.argv[3] if len(sys.argv) > 3 else H + '/bin/driftwm-pristine'
RELAUNCH = sys.argv[4] if len(sys.argv) > 4 else 'all'
RESTART_DELAY = float(os.environ.get('RESTART_DELAY', '1.0'))   # s6-like minimum delay between restarts
RT = '/tmp/hsy'; DISP = ':81'
os.makedirs(OUT, exist_ok=True); os.makedirs(RT, exist_ok=True); os.chmod(RT, 0o700)
os.makedirs('/tmp/hsy-tok', exist_ok=True)
L = S + '/A/root/usr/lib/x86_64-linux-gnu'
ENV = dict(os.environ)
ENV.update({'XDG_RUNTIME_DIR': RT, 'DISPLAY': DISP, 'LD_LIBRARY_PATH': L, 'LIBGL_ALWAYS_SOFTWARE': '1',
            'MESA_LOADER_DRIVER_OVERRIDE': 'swrast', '__EGL_VENDOR_LIBRARY_DIRS': S + '/A/root/usr/share/glvnd/egl_vendor.d',
            'LIBGL_DRIVERS_PATH': L + '/dri', 'GBM_BACKENDS_PATH': L + '/gbm', 'RUST_BACKTRACE': '1',
            'PATH': S + '/A/root/usr/bin:' + H + '/bin:' + os.environ.get('PATH', ''),
            'XDG_DATA_HOME': OUT + '/data', 'XDG_STATE_HOME': OUT + '/state', 'WAYLAND_DISPLAY': 'wayland-1',
            'XDG_DATA_DIRS': OUT + '/data:/usr/share'})
LOGF = open(OUT + '/timeline.log', 'a')
T0 = time.monotonic()
def log(msg):
    LOGF.write(f'{time.monotonic() - T0:9.3f} {msg}\n'); LOGF.flush(); print(f'{time.monotonic() - T0:9.3f} {msg}', flush=True)

ids = [l.split('"')[1] for l in open(H + '/pa/inventory.toml') if l.startswith('id =')]
mids = [i for i in ids if i != 'hub']
os.makedirs(OUT + '/data/applications', exist_ok=True)
for i in ([] if os.environ.get("EXEC_MODE") == "none" else mids):
    open(f'{OUT}/data/applications/hubos-{i}.desktop', 'w').write(
        f"[Desktop Entry]\nType=Application\nName={i}\nExec=" + (f"{H}/pa/open.sh {i}" if os.environ.get("EXEC_MODE", "hubd") == "hubd" else f"foot --app-id=hubos-{i} --title={i} sleep infinity") + f"\nStartupWMClass=hubos-{i}\n")
CONF = OUT + '/driftwm.toml'
open(CONF, 'w').write('[session]\nrestore_windows = true\n')
SESSION = OUT + '/session.json'
HUBD = H + '/bin/' + ('hubd-proto' if RELAUNCH.startswith('hubd-proto') else 'hubd')
ENV['HUBD_BIN'] = HUBD
if RELAUNCH.startswith('hubd-proto'):
    ENV['HUBD_PROTO_RESTORE'] = RELAUNCH.split('-')[-1]

def ipc(req, timeout=3.0):
    p = glob.glob(RT + '/driftwm/ipc-*.sock')
    if not p: raise RuntimeError('no ipc socket')
    s = socket.socket(socket.AF_UNIX); s.settimeout(timeout)
    s.connect(p[0]); s.sendall((json.dumps(req) + '\n').encode())
    buf = b''
    while not buf.endswith(b'\n'):
        c = s.recv(1 << 20)
        if not c: break
        buf += c
    s.close()
    return json.loads(buf)
def state():
    return ipc('State')['Ok']['State']

dw = {'p': None, 'n': 0, 'stop': False}
def start_dw():
    dw['n'] += 1
    lf = open(f'{OUT}/driftwm-{dw["n"]}.log', 'wb')
    dw['p'] = subprocess.Popen([DWBIN, '--backend', 'winit', '--config', CONF, '--session-file', SESSION], env={k: v for k, v in ENV.items() if k != 'WAYLAND_DISPLAY'}, stdout=lf, stderr=lf)
    log(f'driftwm #{dw["n"]} spawned pid={dw["p"].pid}')
def supervisor():
    while not dw['stop']:
        p = dw['p']
        if p is not None and p.poll() is not None:
            log(f'driftwm pid={p.pid} exited rc={p.returncode}')
            dw['p'] = None
            time.sleep(RESTART_DELAY)
            if not dw['stop']: start_dw()
        time.sleep(0.01)

procs = []
fk = subprocess.Popen([H + '/bin/fakenode'] + [l.strip() for l in open(H + '/pa/nodes.txt')], env=ENV, stdout=subprocess.DEVNULL); procs.append(fk)
for f in glob.glob(RT + '/wayland-*'):
    try: os.remove(f)
    except OSError: pass
start_dw()
threading.Thread(target=supervisor, daemon=True).start()
def wait_ipc(tmo=30):
    t = time.time()
    while time.time() - t < tmo:
        try: return state()
        except Exception: time.sleep(0.02)
    return None
assert wait_ipc(), 'driftwm never answered'
hub = subprocess.Popen([HUBD, 'serve', '--inventory', H + '/pa/inventory.toml', '--viewers', H + '/pa/viewers.toml',
                        '--bar-height', '0', '--probe-interval', '1s'], env=ENV, stdout=open(OUT + '/hubd.log', 'wb'), stderr=subprocess.STDOUT)
procs.append(hub)
time.sleep(3)
def hubd(*a, timeout=120):
    return subprocess.run([HUBD, *a], env=ENV, capture_output=True, text=True, timeout=timeout)
log('hubd list: ' + hubd('list').stdout.replace('\n', ' | ')[:300])

# ---- phase 1: open all machines with hubd (the normal way)
t = time.time()
for i in mids:
    r = hubd('open', i)
    if r.returncode != 0: log(f'open {i}: {r.stdout.strip()} {r.stderr.strip()}')
log(f'opened {len(mids)} machines in {time.time() - t:.1f}s')
time.sleep(1)
st = state()
saved = {w['app_id']: (w['position'], w['size']) for w in st['windows']}
log(f'windows before: {len(saved)}  sample: {list(saved.items())[:2]}')
time.sleep(7)   # session file debounce (about 1 s after a change, 5 s after pan/zoom)
try: log('session file bytes: %d' % os.path.getsize(SESSION))
except OSError: log('NO session file')

def live_viewers():
    out = subprocess.run(['pgrep', '-P', str(hub.pid), '-x', 'foot'], capture_output=True, text=True).stdout.split()
    return len(out)

def failure():
    if MODE == 'kill9':
        os.kill(dw['p'].pid, signal.SIGKILL)
    elif MODE == 'sigstop':
        os.kill(dw['p'].pid, signal.SIGSTOP)
    else:
        name = {'busyloop': 'hs-test-busyloop', 'deadlock': 'hs-test-deadlock', 'panic': 'hs-test-panic'}[MODE]
        threading.Thread(target=lambda: ipc({'Action': name}, timeout=300), daemon=True).start()

feet_before = live_viewers()
log(f'viewers before failure: {feet_before}')
tk = time.monotonic(); T_FAIL = tk
log(f'=== FAILURE {MODE} injected')
failure()

if MODE in ('sigstop', 'busyloop', 'deadlock'):
    # hang probe test (policy: kill and restart; never reboot)
    pid = dw['p'].pid
    PROBE_EVERY = 2.0; WINDOW = float(os.environ.get('HANG_WINDOW', '30')); misses = 0; first = None
    while True:
        t1 = time.monotonic()
        try:
            state(); ok = True
        except Exception as e:
            ok = False
        wl_ok = None
        # wl_display sync round trip: connect to the wayland socket and send wl_display.sync, wait for the callback done
        try:
            s = socket.socket(socket.AF_UNIX); s.settimeout(2.0); s.connect(RT + '/wayland-1')
            s.sendall(b'\x01\x00\x00\x00\x00\x00\x0c\x00\x02\x00\x00\x00')  # wl_display.sync(new_id=2)
            d = s.recv(64); wl_ok = len(d) >= 12; s.close()
        except Exception:
            wl_ok = False
        if ok and wl_ok:
            misses = 0; first = None
        else:
            misses += 1; first = first or t1
        log(f'probe: ipc={ok} wl_sync={wl_ok} misses={misses}')
        if first and (time.monotonic() - first) >= WINDOW and misses >= 4:
            log(f'HUNG: {misses} consecutive missed probes over {time.monotonic() - first:.1f}s; evidence: ipc/wl_sync failing; killing pid {pid} (SIGKILL) and letting the supervisor restart')
            try:
                st_ = open(f'/proc/{pid}/status').read(); log('evidence /proc status State: ' + [l for l in st_.split('\n') if l.startswith('State')][0])
                log('evidence wchan: ' + open(f'/proc/{pid}/wchan').read())
                log('evidence cpu ticks: ' + open(f'/proc/{pid}/stat').read().split(')')[1].split()[11])
            except Exception as e: log(f'evidence read failed {e}')
            T_KILL = time.monotonic()
            if MODE == 'sigstop':
                os.kill(pid, signal.SIGCONT)
            os.kill(pid, signal.SIGKILL)
            break
        time.sleep(PROBE_EVERY)
    tk = T_KILL
    log(f'hang detection took {T_KILL - T_FAIL:.1f}s from injection to kill')

# wait for the restarted compositor
t_dead = None
while time.monotonic() - tk < 60:
    n = live_viewers()
    if n == 0 and t_dead is None:
        t_dead = time.monotonic() - tk; log(f'all viewers gone {t_dead:.3f}s after the kill')
    if dw['p'] is not None and dw['p'].poll() is None:
        try:
            s2 = state(); break
        except Exception: pass
    time.sleep(0.01)
t_ipc = time.monotonic() - tk
log(f'new compositor answers IPC {t_ipc:.3f}s after the kill')
# stand-ins
t_si = None
for _ in range(0 if os.environ.get("EXEC_MODE") == "none" else 2000):
    try:
        s2 = state()
        sus = [w for w in s2['windows'] if w.get('suspended')]
        if len(sus) >= len(saved): t_si = time.monotonic() - tk; break
    except Exception: pass
    time.sleep(0.01)
s2 = state() if os.environ.get("EXEC_MODE") == "none" else s2
sus = [] if os.environ.get("EXEC_MODE") == "none" else sus
log("stand-ins visible: %s" % (len(sus) if t_si else ("0 (none expected: no desktop entries)" if os.environ.get("EXEC_MODE") == "none" else "NOT ALL")))
json.dump({'before': saved, 'after_standins': {w['app_id']: (w['position'], w['size'], w.get('mode')) for w in s2['windows']}}, open(OUT + '/geom.json', 'w'))

# ---- relaunch
t_rel = time.monotonic()
wins = {w['app_id']: w['id'] for w in s2['windows'] if w['app_id'] in saved}
if os.environ.get('EXEC_MODE') == 'none': wins = {aid: -1 for aid in saved}
suswins = {w['app_id']: w['id'] for w in s2['windows'] if w.get('suspended')}
log(f'relaunch strategy {RELAUNCH}; {len(wins)} stand-ins')
done = {}
def watch_live():
    while len(done) < len(wins) and time.monotonic() - t_rel < 120:
        try:
            cur = state()['windows']
            for w in cur:
                if w['app_id'] in wins and not w.get('suspended') and w['app_id'] not in done:
                    done[w['app_id']] = (time.monotonic() - t_rel, w['id'], w['position'], w['size'])
        except Exception: pass
        time.sleep(0.05)
wt = threading.Thread(target=watch_live); wt.start()
if RELAUNCH == 'all':
    for aid, wid in suswins.items(): ipc({'Relaunch': wid})
elif RELAUNCH == 'stagger':
    for aid, wid in suswins.items():
        ipc({'Relaunch': wid}); n0 = len(done)
        t_ = time.time()
        while len(done) <= n0 and time.time() - t_ < 10: time.sleep(0.02)
wt.join()
log(f'(offset of the relaunch start from the kill: {t_rel - tk:.3f}s) relaunch: {len(done)}/{len(wins)} windows live; first {min(v[0] for v in done.values()) if done else None}; last {max(v[0] for v in done.values()) if done else None}')
time.sleep(2.0)
fin = {w['app_id']: w for w in state()['windows']}
bad = []
for aid, (dt, wid, pos, size) in done.items():
    b = saved.get(aid); f = fin.get(aid)
    if b and f and (b[0] != f['position'] or b[1] != f['size']): bad.append((aid, b, (f['position'], f['size'])))
same_id = sum(1 for aid, (dt, wid, pos, size) in done.items() if wins.get(aid) == wid)
log(f'geometry check after settling 2 s: {len(done) - len(bad)} same position+size as before the crash, {len(bad)} different: {bad[:3]}; window id kept from the stand-in: {same_id}/{len(done)}')
final = state()['windows']
log(f'final windows: {len(final)} (suspended left: {len([w for w in final if w.get("suspended")])}); window count by app: duplicates={len(final) - len(set(w["app_id"] for w in final))}')
json.dump({'done': done}, open(OUT + '/relaunch.json', 'w'))
for q in subprocess.run(['pgrep', '-P', str(hub.pid), '-x', 'foot'], capture_output=True, text=True).stdout.split(): os.kill(int(q), signal.SIGKILL)
for p in procs:
    p.terminate()
dw['stop'] = True
if dw['p']: dw['p'].terminate()
