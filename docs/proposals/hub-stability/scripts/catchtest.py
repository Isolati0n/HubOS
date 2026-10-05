#!/usr/bin/env python3
"""After a CAUGHT panic (patched driftwm, HS_CATCH_PANICS=1): is the compositor still healthy?
usage: catchtest.py BIN"""
import os, subprocess, socket, json, time, glob, shutil, sys
S = os.environ['HS_WORK']
RT = '/tmp/hsc'; BIN = sys.argv[1]
shutil.rmtree(RT, ignore_errors=True); os.makedirs(RT); os.chmod(RT, 0o700)
L = S + '/A/root/usr/lib/x86_64-linux-gnu'
ENV = dict(os.environ)
ENV.update({'XDG_RUNTIME_DIR': RT, 'DISPLAY': ':81', 'LD_LIBRARY_PATH': L, 'LIBGL_ALWAYS_SOFTWARE': '1', 'MESA_LOADER_DRIVER_OVERRIDE': 'swrast',
            '__EGL_VENDOR_LIBRARY_DIRS': S + '/A/root/usr/share/glvnd/egl_vendor.d', 'LIBGL_DRIVERS_PATH': L + '/dri', 'GBM_BACKENDS_PATH': L + '/gbm',
            'HS_CATCH_PANICS': '1', 'RUST_BACKTRACE': '0', 'PATH': S + '/A/root/usr/bin:' + os.environ.get('PATH', '')})
ENV.pop('WAYLAND_DISPLAY', None)
lf = open(RT + '/dw.log', 'wb')
dw = subprocess.Popen([BIN, '--backend', 'winit', '--config', '/dev/null'], env=ENV, stdout=lf, stderr=lf)
def ipc(req, timeout=3):
    s = socket.socket(socket.AF_UNIX); s.settimeout(timeout)
    s.connect(glob.glob(RT + '/driftwm/ipc-*.sock')[0]); s.sendall((json.dumps(req) + '\n').encode())
    buf = b''
    while not buf.endswith(b'\n'):
        c = s.recv(1 << 20)
        if not c: break
        buf += c
    return json.loads(buf)
for _ in range(100):
    try: ipc('State'); break
    except Exception: time.sleep(0.1)
e2 = dict(ENV); e2['WAYLAND_DISPLAY'] = 'wayland-1'
def foot(n): return subprocess.Popen(['foot', f'--app-id=ct{n}', 'sleep', 'infinity'], env=e2, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
feet = [foot(0), foot(1)]
time.sleep(3)
print('windows before:', len(ipc('State')['Ok']['State']['windows']))
def cpu():
    f = open(f'/proc/{dw.pid}/stat').read().rsplit(')', 1)[1].split(); return (int(f[11]) + int(f[12])) / 100
for k in range(20):
    subprocess.run(['python3', S + '/hs/repro_shm.py', RT + '/wayland-1'], capture_output=True)
time.sleep(1)
c0 = cpu(); time.sleep(5); c1 = cpu()
print('driftwm alive after 20 caught panics:', dw.poll() is None, '; CPU over 5 s idle: %.2f s' % (c1 - c0))
feet.append(foot(2)); time.sleep(3)
ws = ipc('State')['Ok']['State']['windows']
print('windows after (a new foot was started after the panics):', [(w['app_id']) for w in ws])
print('foot A,B still running:', feet[0].poll() is None, feet[1].poll() is None)
t = time.time(); r = ipc({'Move': {'window': ws[0]['id'], 'to': [100, 100]}}); print('IPC Move works:', r, f'{(time.time() - t) * 1000:.0f} ms')
logs = open(RT + '/dw.log', errors='replace').read()
print('PANIC CAUGHT lines:', logs.count('PANIC CAUGHT'), '; panicked lines:', logs.count('panicked at'))
for f in feet: f.kill()
dw.terminate()
