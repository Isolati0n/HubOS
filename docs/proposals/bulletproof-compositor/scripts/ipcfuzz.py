#!/usr/bin/env python3
"""ipcfuzz.py SECONDS OUTDIR SEED DW_BINARY [ulimit_v_kb]
Fuzzes driftwm's IPC socket (line-delimited JSON; the interface hubd uses) with valid, extreme and malformed requests
while 6 foot windows and 3 stand-ins exist.  After every 100 requests it checks that the compositor answers.
Screenshots go only into OUTDIR/shots.  Saves the last 200 requests when the compositor dies or hangs."""
import json, os, random, resource, signal, socket, subprocess, sys, time, glob, shutil, collections
SECS = float(sys.argv[1]); OUT = sys.argv[2]; SEED = int(sys.argv[3]); DW = sys.argv[4]
ULIM = int(sys.argv[5]) if len(sys.argv) > 5 else 0
W = os.environ['W']; A = W + '/A'; RT = '/tmp/bc4ipc'; DISP = os.environ.get('SOAK_DISPLAY', ':93')
rnd = random.Random(SEED)
os.makedirs(OUT + '/shots', exist_ok=True)
L = A + '/root/usr/lib/x86_64-linux-gnu'
ENV = dict(os.environ)
ENV.update({'XDG_RUNTIME_DIR': RT, 'DISPLAY': DISP, 'LD_LIBRARY_PATH': L, 'LIBGL_ALWAYS_SOFTWARE': '1', 'MESA_LOADER_DRIVER_OVERRIDE': 'swrast',
            '__EGL_VENDOR_LIBRARY_DIRS': A + '/root/usr/share/glvnd/egl_vendor.d', 'LIBGL_DRIVERS_PATH': L + '/dri', 'GBM_BACKENDS_PATH': L + '/gbm',
            'XDG_DATA_HOME': OUT + '/data', 'PATH': A + '/root/usr/bin:' + os.environ.get('PATH', ''), 'XDG_STATE_HOME': OUT + '/state', 'RUST_BACKTRACE': '1'})
ENV.pop('WAYLAND_DISPLAY', None)
shutil.rmtree(RT, ignore_errors=True); os.makedirs(RT); os.chmod(RT, 0o700)
os.makedirs(OUT + '/data/applications', exist_ok=True)
for n in range(1, 10):
    open(f'{OUT}/data/applications/hubos-s{n:02d}.desktop', 'w').write(
        f"[Desktop Entry]\nType=Application\nName=s{n:02d}\nExec=foot --app-id=hubos-s{n:02d} --title=s{n:02d} sleep 100000\nStartupWMClass=hubos-s{n:02d}\n")
CONF = OUT + '/c.toml'; open(CONF, 'w').write('[session]\nrestore_windows = true\n')
LOG = open(OUT + '/driftwm.log', 'ab')
def pre():
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))
    if ULIM: resource.setrlimit(resource.RLIMIT_AS, (ULIM * 1024, ULIM * 1024))
proc = subprocess.Popen([DW, '--backend', 'winit', '--config', CONF, '--session-file', OUT + '/session.json'], env=ENV, stdout=LOG, stderr=LOG, cwd=OUT, preexec_fn=pre)
for _ in range(300):
    if glob.glob(RT + '/driftwm/ipc-*.sock') and glob.glob(RT + '/wayland-*'): break
    time.sleep(0.1)
sockp = glob.glob(RT + '/driftwm/ipc-*.sock')[0]
wsock = [os.path.basename(p) for p in glob.glob(RT + '/wayland-*') if not p.endswith('.lock')][0]
def conn(timeout=4.0):
    s = socket.socket(socket.AF_UNIX); s.settimeout(timeout); s.connect(sockp); return s
def rpc(req, timeout=4.0):
    s = conn(timeout)
    try:
        s.sendall((req if isinstance(req, bytes) else json.dumps(req).encode()) + b'\n'); buf = b''
        while not buf.endswith(b'\n'):
            c = s.recv(1 << 20)
            if not c: break
            buf += c
        return buf
    finally: s.close()
e = dict(ENV); e['WAYLAND_DISPLAY'] = wsock
for n in range(1, 7): subprocess.Popen(['foot', f'--app-id=hubos-s{n:02d}', f'--title=s{n:02d}', 'sleep', '100000'], env=e, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, cwd=OUT)
time.sleep(3)
ws = json.loads(rpc('State'))['Ok']['State']['windows']
for w in ws[:3]: rpc({'Suspend': w['id']})
time.sleep(1)

INTS = [0, 1, -1, 2, 100, -100, 4096, 65535, 1 << 24, 1 << 30, 0x7fffffff, -0x80000000, 0x7ffffffe, 10**12, -10**12, 10**30]
FLOATS = [0.0, -0.0, 1.0, -1.0, 0.5, 1e-9, 1e9, 1e300, -1e300, 2.5, 100.0]
STRS = ['', 'a', 'term', 'hubos-s01', 'x' * 5000, 'é漢', '\u0000', '../../x', '%s%n', 'close-window', 'spawn', 'spawn true', 'exec', 'move-window left', 'move-window left 99999999999', 'move-window  ', 'zoom-in', 'fit-window',
        'set-bookmark', 'set-bookmark a', 'goto-bookmark', 'goto-bookmark nonexistent', 'switch-layout', 'switch-layout next', 'switch-layout 99999', 'pan-viewport up', 'nudge-window right 2147483647', 'resize-window 999999999 999999999',
        'send-to-output', 'send-to-output next', 'center-nearest', 'center-nearest left', 'toggle-fullscreen', 'suspend-window', 'reload-config', 'fill-window', 'fit-window-snapped', 'home-toggle', 'cycle-windows next', 'scroll-up 1e308']
def sel():
    ids = [w['id'] for w in cur_ws] if cur_ws else [1]
    r = rnd.random()
    if r < .4: return rnd.choice(ids)
    if r < .55: return rnd.choice(INTS)
    if r < .8: return rnd.choice(['s01', 'hubos', 'term', 'zzz', '', 'S0'])
    return rnd.choice(STRS)
def opt(f):
    return None if rnd.random() < .2 else f()
def pair(g): return [g(), g()]
def gen():
    k = rnd.randrange(22)
    ii = lambda: rnd.choice(INTS) if rnd.random() < .6 else rnd.randint(-3000, 3000)
    ff = lambda: rnd.choice(FLOATS) if rnd.random() < .5 else rnd.uniform(-3000, 3000)
    if k == 0: return {'Camera': opt(lambda: pair(ff))}
    if k == 1: return {'Zoom': opt(ff)}
    if k == 2: return {'Layout': {'short': rnd.random() < .5}}
    if k in (3, 4): return rnd.choice(['State', 'DebugCounters'])
    if k == 5: return {'Focus': opt(sel)}
    if k in (6, 7): return {'Move': {'window': opt(sel), 'to': opt(lambda: pair(ii))}}
    if k in (8, 9): return {'Resize': {'window': opt(sel), 'to': opt(lambda: pair(ii))}}
    if k == 10: return {'Opacity': {'window': opt(sel), 'value': opt(ff)}}
    if k == 11: return {'Pin': {'window': opt(sel), 'value': opt(lambda: rnd.random() < .5)}}
    if k == 12: return {'Close': opt(sel)} if rnd.random() < .3 else {'Suspend': opt(sel)}
    if k == 13: return {'Relaunch': opt(sel)}
    if k in (14, 15, 16): return {'Action': rnd.choice(STRS) if rnd.random() < .7 else ' '.join(rnd.choice(STRS) for _ in range(rnd.randint(1, 3)))}
    if k == 17: return {'Bookmark': {'name': opt(lambda: rnd.choice(STRS[:8])), 'to': opt(lambda: pair(ff)), 'delete': rnd.random() < .3}}
    if k == 18:
        tgt = rnd.choice(['Viewport', 'All', {'Window': {'window': opt(sel)}}, {'Region': {'x': ii(), 'y': ii(), 'w': rnd.choice([1, 10, 100, 1000, 0, -5]), 'h': rnd.choice([1, 10, 100, 1000, 0, -5]), 'from_screen': rnd.random() < .5}}])
        return {'Screenshot': {'target': tgt, 'scale': rnd.choice([0.01, 0.1, 0.5, 1.0, 0.0, -1.0, 2.0]), 'path': OUT + '/shots/s%d.png' % rnd.randint(0, 5)}}
    if k == 19: return {'Move': {'window': sel()}}   # query form
    if k == 20: return {rnd.choice(['Nope', 'move', 'STATE', 'Camera', 'Zoom', 'Move', 'Action']): rnd.choice([1, 'x', [], {}, None, [1, 2, 3]])}
    return rnd.choice([1, 'x', [], None, True, {}, [1], 3.5])
def bad_line():
    r = rnd.random()
    if r < .25: return bytes(rnd.getrandbits(8) for _ in range(rnd.choice([1, 10, 100, 5000])))
    if r < .5: return json.dumps(gen()).encode()[:rnd.randint(0, 40)]
    if r < .7: return b'{' * rnd.choice([10, 1000, 100000])
    if r < .85: return b'"' + b'x' * rnd.choice([1000, 100000, 1000000]) + b'"'
    return json.dumps(gen()).encode().replace(b'}', b'}}', 1)
cur_ws = ws
log = collections.deque(maxlen=int(os.environ.get("IPCF_KEEP","200")))
t_end = time.time() + SECS; n = 0; errs = collections.Counter(); oks = 0; kinds = collections.Counter(); last_check = 0
def alive(): return proc.poll() is None
def refresh():
    global cur_ws
    try: cur_ws = json.loads(rpc('State'))['Ok']['State']['windows']
    except Exception: pass
died = None
while time.time() < t_end:
    try:
        if rnd.random() < .06: req = bad_line(); desc = repr(req[:200])
        else: req = gen(); desc = json.dumps(req)[:300]
        log.append(desc)
        kinds[(list(req)[0] if isinstance(req, dict) and req else type(req).__name__ if not isinstance(req, bytes) else 'raw')] += 1
        r = rpc(req)
        if b'"Ok"' in r[:8]: oks += 1
        elif r: errs[r[:60].decode(errors='replace')] += 1
    except (socket.timeout, ConnectionError, OSError) as ex:
        errs['conn:' + type(ex).__name__] += 1
    n += 1
    if n % 100 == 0:
        if not alive(): died = 'DEAD'; break
        try: rpc('State', timeout=6)
        except Exception:
            time.sleep(2)
            try: rpc('State', timeout=8)
            except Exception: died = 'HUNG' if alive() else 'DEAD'; break
        refresh()
        if n % 500 == 0:   # keep some windows alive for targets
            if len([w for w in cur_ws if w['app_id'].startswith('hubos-s')]) < 5:
                for k in range(1, 7): subprocess.Popen(['foot', f'--app-id=hubos-s{k:02d}', f'--title=s{k:02d}', 'sleep', '100000'], env=e, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, cwd=OUT)
if died is None and not alive(): died = 'DEAD'
print(f'requests={n} ok_replies={oks} error_replies={sum(errs.values())} result={"ALIVE" if died is None else died} rc={proc.poll()}')
print('top request kinds:', dict(kinds.most_common(8)))
print('top error texts:', dict(errs.most_common(6)))
if died:
    json.dump(list(log), open(f'{OUT}/ipc-crash-seed{SEED}.json', 'w'))
    print('last requests saved to', f'{OUT}/ipc-crash-seed{SEED}.json')
    if died == 'HUNG':
        os.system(f'timeout 60 gdb -p {proc.pid} -batch -ex "thread 1" -ex "bt 12" 2>&1 | grep -E "^#" | head -14 > {OUT}/hang-bt.txt')
if alive():
    print('RSS KB:', [l.split()[1] for l in open(f'/proc/{proc.pid}/status') if l.startswith('VmRSS')])
    proc.kill()
    proc.wait()
os.system('pkill -KILL -P %d 2>/dev/null' % os.getpid()) if False else None
