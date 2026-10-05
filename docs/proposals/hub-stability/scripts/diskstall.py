#!/usr/bin/env python3
"""A stand-in for a hung disk: the session file's temporary name is a FIFO nobody reads, so open(2) for writing blocks
(like a write to a stalled disk). Does the compositor's event loop freeze?"""
import os, subprocess, socket, json, time, glob, shutil
S = os.environ['HS_WORK']
RT = '/tmp/hsd'; D = '/tmp/hsd-state'
for p in (RT, D): shutil.rmtree(p, ignore_errors=True); os.makedirs(p); os.chmod(p, 0o700)
L = S + '/A/root/usr/lib/x86_64-linux-gnu'
ENV = dict(os.environ)
ENV.update({'XDG_RUNTIME_DIR': RT, 'DISPLAY': ':81', 'LD_LIBRARY_PATH': L, 'LIBGL_ALWAYS_SOFTWARE': '1', 'MESA_LOADER_DRIVER_OVERRIDE': 'swrast',
            '__EGL_VENDOR_LIBRARY_DIRS': S + '/A/root/usr/share/glvnd/egl_vendor.d', 'LIBGL_DRIVERS_PATH': L + '/dri', 'GBM_BACKENDS_PATH': L + '/gbm',
            'PATH': S + '/A/root/usr/bin:' + os.environ.get('PATH', '')})
ENV.pop('WAYLAND_DISPLAY', None)
open(D + '/c.toml', 'w').write('[session]\nrestore_windows = true\n')
os.mkfifo(D + '/session.json.tmp')
lf = open(D + '/dw.log', 'wb')
dw = subprocess.Popen([S + '/hubstab/bin/driftwm-pristine', '--backend', 'winit', '--config', D + '/c.toml', '--session-file', D + '/session.json'], env=ENV, stdout=lf, stderr=lf)
def ipc(req, timeout=3):
    s = socket.socket(socket.AF_UNIX); s.settimeout(timeout)
    s.connect(glob.glob(RT + '/driftwm/ipc-*.sock')[0]); s.sendall((json.dumps(req) + '\n').encode())
    buf = b''
    while not buf.endswith(b'\n'):
        c = s.recv(1 << 20)
        if not c: break
        buf += c
    return buf
for _ in range(100):
    try: ipc('State'); break
    except Exception: time.sleep(0.1)
print('IPC up; now one window is opened (this makes driftwm queue a session write)')
e2 = dict(ENV); e2['WAYLAND_DISPLAY'] = 'wayland-1'
f = subprocess.Popen(['foot', '--app-id=ds1', 'sleep', 'infinity'], env=e2, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
time.sleep(3)
for k in range(4):
    t = time.time()
    try: ipc('State', timeout=2); print(f'IPC State {k}: {(time.time() - t) * 1000:.0f} ms')
    except Exception as e: print(f'IPC State {k}: no answer within 2 s ({type(e).__name__})')
    time.sleep(1)
st = open(f'/proc/{dw.pid}/status').read().split('State:')[1].split('\n')[0].strip()
print('process state:', st, '; wchan:', open(f'/proc/{dw.pid}/wchan').read())
f.kill(); dw.kill()
