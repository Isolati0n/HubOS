#!/usr/bin/env python3
"""ipcmin.py BIN CRASH.json OUTDIR : delta-debug the saved IPC requests of ipcfuzz.py (same set-up: 6 foot windows, 3 stand-ins).
Oracle: the compositor process dies.  Prints the minimal request list."""
import json, os, resource, shutil, socket, subprocess, sys, time, glob
BIN, LOGF, OUT = sys.argv[1], sys.argv[2], sys.argv[3]
W = os.environ['W']; A = W + '/A'; RT = '/tmp/bc4ipcmin'; DISP = os.environ.get('SOAK_DISPLAY', ':93')
L = A + '/root/usr/lib/x86_64-linux-gnu'
reqs = json.load(open(LOGF))
os.makedirs(OUT, exist_ok=True)
def trial(sub):
    shutil.rmtree(RT, ignore_errors=True); os.makedirs(RT); os.chmod(RT, 0o700)
    env = dict(os.environ)
    env.update({'XDG_RUNTIME_DIR': RT, 'DISPLAY': DISP, 'LD_LIBRARY_PATH': L, 'LIBGL_ALWAYS_SOFTWARE': '1', 'MESA_LOADER_DRIVER_OVERRIDE': 'swrast',
                '__EGL_VENDOR_LIBRARY_DIRS': A + '/root/usr/share/glvnd/egl_vendor.d', 'LIBGL_DRIVERS_PATH': L + '/dri', 'GBM_BACKENDS_PATH': L + '/gbm',
                'XDG_DATA_HOME': OUT + '/data', 'PATH': A + '/root/usr/bin:' + os.environ.get('PATH', ''), 'XDG_STATE_HOME': OUT + '/state'})
    env.pop('WAYLAND_DISPLAY', None)
    os.makedirs(OUT + '/data/applications', exist_ok=True)
    for n in range(1, 10):
        open(f'{OUT}/data/applications/hubos-s{n:02d}.desktop', 'w').write(f"[Desktop Entry]\nType=Application\nName=s{n:02d}\nExec=foot --app-id=hubos-s{n:02d} --title=s{n:02d} sleep 100000\nStartupWMClass=hubos-s{n:02d}\n")
    open(OUT + '/c.toml', 'w').write('[session]\nrestore_windows = true\n')
    shutil.rmtree(OUT + '/state', ignore_errors=True)
    p = subprocess.Popen([BIN, '--backend', 'winit', '--config', OUT + '/c.toml', '--session-file', OUT + '/session.json'], env=env, stdout=open(OUT + '/dw.log', 'w'), stderr=subprocess.STDOUT, cwd=OUT,
                         preexec_fn=lambda: (resource.setrlimit(resource.RLIMIT_CORE, (0, 0)), (resource.setrlimit(resource.RLIMIT_AS, (int(os.environ["IPCMIN_ULIM"])*1024,)*2) if os.environ.get("IPCMIN_ULIM") else None)))
    for _ in range(900):
        if glob.glob(RT + '/driftwm/ipc-*.sock') and glob.glob(RT + '/wayland-*'): break
        time.sleep(0.1)
    if not (glob.glob(RT + '/driftwm/ipc-*.sock') and glob.glob(RT + '/wayland-*')):
        try: p.kill(); p.wait(5)
        except Exception: pass
        return False
    sp = glob.glob(RT + '/driftwm/ipc-*.sock')[0]; wl = [os.path.basename(x) for x in glob.glob(RT + '/wayland-*') if not x.endswith('.lock')][0]
    def rpc(r, t=4):
        s = socket.socket(socket.AF_UNIX); s.settimeout(t); s.connect(sp)
        try:
            s.sendall((r if isinstance(r, bytes) else r.encode()) + b'\n'); b = b''
            while not b.endswith(b'\n'):
                c = s.recv(1 << 20)
                if not c: break
                b += c
            return b
        finally: s.close()
    e = dict(env); e['WAYLAND_DISPLAY'] = wl
    fs = [subprocess.Popen(['foot', f'--app-id=hubos-s{n:02d}', f'--title=s{n:02d}', 'sleep', '100000'], env=e, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, cwd=OUT) for n in range(1, 7)]
    time.sleep(3)
    ws = json.loads(rpc('"State"'))['Ok']['State']['windows']
    for w in ws[:3]: rpc(json.dumps({'Suspend': w['id']}))
    time.sleep(1)
    for r in sub:
        if p.poll() is not None: break
        try:
            if r.startswith("b'"): continue   # raw bytes are shown as repr in the log; skip
            rpc(r)
        except Exception: pass
    time.sleep(0.5)
    dead = p.poll() is not None
    try: p.kill(); p.wait(5)
    except Exception: pass
    for f in fs:
        try: f.kill()
        except Exception: pass
    return dead
if not trial(reqs): print('NOT REPRODUCED with all', len(reqs), 'requests'); sys.exit(1)
cur = list(reqs); n = 2
print('reproduced with', len(cur), 'requests', flush=True)
while len(cur) >= 2:
    chunk = max(1, len(cur) // n); subs = [cur[i:i + chunk] for i in range(0, len(cur), chunk)]; red = False
    for sub in subs:
        comp = [x for x in cur if x not in sub] if False else cur[:cur.index(sub[0])] + cur[cur.index(sub[0]) + len(sub):]
        if comp and trial(comp): cur = comp; n = max(n - 1, 2); red = True; break
    if not red:
        if n >= len(cur): break
        n = min(len(cur), n * 2)
    print('  ', len(cur), 'requests left', flush=True)
print('MINIMAL:'); [print('  ', x[:300]) for x in cur]
json.dump(cur, open(OUT + '/ipc-min.json', 'w'))
