#!/usr/bin/env python3
"""Does a log reader that stops reading freeze driftwm? stderr of driftwm goes into a pipe nobody drains."""
import os, subprocess, socket, json, time, glob, signal, sys
S = os.environ['HS_WORK']
RT = '/tmp/hsl'
import shutil
shutil.rmtree(RT, ignore_errors=True); os.makedirs(RT); os.chmod(RT, 0o700)
L = S + '/A/root/usr/lib/x86_64-linux-gnu'
ENV = dict(os.environ)
ENV.update({'XDG_RUNTIME_DIR': RT, 'DISPLAY': ':81', 'LD_LIBRARY_PATH': L, 'LIBGL_ALWAYS_SOFTWARE': '1', 'MESA_LOADER_DRIVER_OVERRIDE': 'swrast',
            '__EGL_VENDOR_LIBRARY_DIRS': S + '/A/root/usr/share/glvnd/egl_vendor.d', 'LIBGL_DRIVERS_PATH': L + '/dri', 'GBM_BACKENDS_PATH': L + '/gbm',
            'RUST_LOG': sys.argv[1] if len(sys.argv) > 1 else 'debug', 'PATH': S + '/A/root/usr/bin:' + os.environ.get('PATH', '')})
ENV.pop('WAYLAND_DISPLAY', None)
r, w = os.pipe()          # nobody reads r
dw = subprocess.Popen([S + '/hubstab/bin/driftwm-pristine', '--backend', 'winit', '--config', '/dev/null'], env=ENV, stdout=w, stderr=w)
def ipc(req, timeout=3):
    s = socket.socket(socket.AF_UNIX); s.settimeout(timeout)
    s.connect(glob.glob(RT + '/driftwm/ipc-*.sock')[0]); s.sendall((json.dumps(req) + '\n').encode())
    buf = b''
    while not buf.endswith(b'\n'):
        c = s.recv(1 << 20)
        if not c: break
        buf += c
    return buf
t0 = time.time()
up = False
for _ in range(100):
    try: ipc('State'); up = True; break
    except Exception: time.sleep(0.1)
print('driftwm answered IPC at start:', up, f'({time.time() - t0:.1f}s)')
e2 = dict(ENV); e2['WAYLAND_DISPLAY'] = 'wayland-1'
feet = []
for n in range(12):
    feet.append(subprocess.Popen(['foot', f'--app-id=ls{n}', 'sleep', 'infinity'], env=e2, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL))
    time.sleep(0.3)
    try: ipc({'Move': {'window': None, 'to': [n * 40, n * 30]}}, 2)
    except Exception as e: pass
time.sleep(2)
for k in range(5):
    t = time.time()
    try: ipc('State', timeout=3); print(f'IPC State round trip {k}: {(time.time() - t) * 1000:.0f} ms')
    except Exception as e: print(f'IPC State round trip {k}: FAILED/timeout after {time.time() - t:.1f}s ({type(e).__name__})')
    time.sleep(1)
st = open(f'/proc/{dw.pid}/status').read().split('State:')[1].split('\n')[0].strip()
try: wch = open(f'/proc/{dw.pid}/wchan').read()
except Exception: wch = '?'
print('process state:', st, 'wchan:', wch)
import fcntl
# how much is in the pipe
buf = bytearray(4)
import array, termios
n = array.array('i', [0]); fcntl.ioctl(r, termios.FIONREAD, n); print('bytes waiting in the undrained log pipe:', n[0])
for f in feet: f.kill()
dw.kill()
