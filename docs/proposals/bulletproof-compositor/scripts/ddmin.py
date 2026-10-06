#!/usr/bin/env python3
"""ddmin.py BIN LOG.json LABEL [NCONN] [MARKER]  - delta-debug the messages of the last NCONN-th connection.
Oracle: the compositor process dies (or hangs) while replay.py runs (exit 3 or 4 from replay.py, or process dead afterwards).
Output: minimal list of message indices, and a hex dump + decoded text of the minimal message list."""
import json, os, subprocess, sys, time, glob, shutil, struct
BIN, LOG, LABEL = sys.argv[1], sys.argv[2], sys.argv[3]
NCONN = int(sys.argv[4]) if len(sys.argv) > 4 else 1
MARKER = sys.argv[5] if len(sys.argv) > 5 else ''
W = os.environ['W']; A = W + '/A'; L = A + '/root/usr/lib/x86_64-linux-gnu'
d = json.load(open(LOG))
convs = d['logs'] if 'logs' in d else [{'log': d['log']}]
use = convs[-NCONN:]
last = use[-1]['log']
RT = f'/tmp/bc4dd-{LABEL}'
def trial(keep):
    shutil.rmtree(RT, ignore_errors=True); os.makedirs(RT); os.chmod(RT, 0o700)
    e = dict(os.environ); e.update({'XDG_RUNTIME_DIR': RT, 'DISPLAY': ':90', 'LD_LIBRARY_PATH': L, 'LIBGL_ALWAYS_SOFTWARE': '1', 'MESA_LOADER_DRIVER_OVERRIDE': 'swrast',
        '__EGL_VENDOR_LIBRARY_DIRS': A + '/root/usr/share/glvnd/egl_vendor.d', 'LIBGL_DRIVERS_PATH': L + '/dri', 'GBM_BACKENDS_PATH': L + '/gbm', 'RUST_BACKTRACE': '0',
        'XDG_STATE_HOME': RT + '/st', 'XDG_DATA_HOME': RT + '/da', 'PATH': A + '/root/usr/bin:' + os.environ['PATH']})
    e.pop('WAYLAND_DISPLAY', None)
    open(RT + '/c.toml', 'w').write('')
    p = subprocess.Popen([BIN, '--backend', 'winit', '--config', RT + '/c.toml'], env=e, stdout=open(RT + '/log', 'w'), stderr=subprocess.STDOUT, cwd=RT)
    t0 = time.time()
    while not os.path.exists(RT + '/wayland-1') and time.time() - t0 < 30: time.sleep(0.05)
    time.sleep(0.3)
    # earlier connections are replayed in full
    sub = {'logs': [{'log': c['log']} for c in use[:-1]] + [{'log': last}]}
    json.dump(sub, open(RT + '/in.json', 'w'))
    keepstr = ','.join(str(i) for i in keep)
    r = subprocess.run([sys.executable, W + '/replay.py', RT + '/wayland-1', RT + '/in.json', '--keep', keepstr or '-1', '--pid', str(p.pid)], capture_output=True, text=True, timeout=120)
    time.sleep(0.3)
    dead = p.poll() is not None
    res = 'dead' if (dead or r.returncode == 3) else ('hung' if r.returncode == 4 else 'ok')
    if MARKER:
        time.sleep(0.2)
        if MARKER not in open(RT + '/log', errors='replace').read(): res = 'ok'
    try: p.kill(); p.wait(5)
    except Exception: pass
    return res
base = list(range(len(last)))
t = trial(base)
print(f'baseline with all {len(base)} messages of the last {NCONN} connection(s): {t}', flush=True)
if t == 'ok': print('NOT REPRODUCED'); sys.exit(1)
target = t
cur = base; n = 2
trials = 1
while len(cur) >= 2:
    chunk = max(1, len(cur) // n); subsets = [cur[i:i + chunk] for i in range(0, len(cur), chunk)]
    reduced = False
    for sub in subsets:
        comp = [x for x in cur if x not in sub]
        if not comp: continue
        trials += 1
        if trial(comp) == target:
            cur = comp; n = max(n - 1, 2); reduced = True; break
    if not reduced:
        if n >= len(cur): break
        n = min(len(cur), n * 2)
    print(f'  {len(cur)} messages left after {trials} trials', flush=True)
print(f'MINIMAL ({target}): {len(cur)} messages, indices {cur}')
for i in cur:
    h, fds = last[i][0], last[i][1]
    b = bytes.fromhex(h); o, w = struct.unpack('<II', b[:8])
    print(f'  [{i}] {last[i][3] if len(last[i]) > 3 else ""} object={o} opcode={w & 0xffff} size={w >> 16} payload={b[8:].hex()} fds={fds}')
json.dump({'logs': [{'log': [last[i] for i in cur]}]}, open(f'{W}/min-{LABEL}.json', 'w'))
shutil.rmtree(RT, ignore_errors=True)
