import os
#!/usr/bin/env python3
"""(iv) Does a viewer present the xdg-activation token that driftwm hands to a relaunched app?
Method: suspend a window of app-id hubos-tk, then change the stand-in's launch command so the NEW program uses a
DIFFERENT app-id. The 5 s app-id fallback can then never match, so the window can only be adopted into the stand-in's
slot if the program presents the token. usage: tokentest.py OUTDIR 'launch command with {AID} placeholder' ['command for first window']"""
import json, os, socket, subprocess, sys, time, glob, shutil
S = os.environ['HS_WORK']
H = S + '/hubstab'
OUT = sys.argv[1]; CMD2 = sys.argv[2]; CMD1 = sys.argv[3] if len(sys.argv) > 3 else CMD2
RT = '/tmp/hsz'
shutil.rmtree(OUT, ignore_errors=True); os.makedirs(OUT + '/data/applications'); os.makedirs(RT, exist_ok=True); os.chmod(RT, 0o700)
L = S + '/A/root/usr/lib/x86_64-linux-gnu'
extra = os.environ.get('EXTRA_LIB', '')
ENV = dict(os.environ)
ENV.update({'XDG_RUNTIME_DIR': RT, 'DISPLAY': ':81', 'LD_LIBRARY_PATH': L + (':' + extra if extra else ''), 'LIBGL_ALWAYS_SOFTWARE': '1',
            'MESA_LOADER_DRIVER_OVERRIDE': 'swrast', '__EGL_VENDOR_LIBRARY_DIRS': S + '/A/root/usr/share/glvnd/egl_vendor.d',
            'LIBGL_DRIVERS_PATH': L + '/dri', 'GBM_BACKENDS_PATH': L + '/gbm', 'RUST_LOG': 'debug',
            'PATH': S + '/A/root/usr/bin:' + os.environ.get('PATH', ''), 'XDG_DATA_HOME': OUT + '/data', 'XDG_DATA_DIRS': OUT + '/data:/usr/share',
            'XDG_STATE_HOME': OUT + '/state'})
ENV.pop('WAYLAND_DISPLAY', None)
for f in glob.glob(RT + '/wayland-*'):
    try: os.remove(f)
    except OSError: pass
def entry(cmd):
    try: os.remove(OUT + '/data/applications/hubos-tk.desktop')   # driftwm re-scans only when the directory changes
    except OSError: pass
    time.sleep(0.05)
    open(OUT + '/data/applications/hubos-tk.desktop', 'w').write(f'[Desktop Entry]\nType=Application\nName=tk\nExec={cmd}\nStartupWMClass=hubos-tk\n')
entry(CMD1.replace('{AID}', 'hubos-tk'))
open(OUT + '/c.toml', 'w').write('[session]\nrestore_windows = true\n')
lf = open(OUT + '/dw.log', 'wb')
dw = subprocess.Popen([H + '/bin/driftwm-pristine', '--backend', 'winit', '--config', OUT + '/c.toml', '--session-file', OUT + '/s.json'], env=ENV, stdout=lf, stderr=lf)
def ipc(req):
    s = socket.socket(socket.AF_UNIX); s.settimeout(5)
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
e2 = dict(ENV); e2['WAYLAND_DISPLAY'] = [os.path.basename(p) for p in glob.glob(RT + '/wayland-*') if not p.endswith('.lock')][0]
first = subprocess.Popen(CMD1.replace('{AID}', 'hubos-tk').split(), env=e2, stdout=open(OUT + '/p1.log', 'wb'), stderr=subprocess.STDOUT)
def wins(): return ipc('State')['Ok']['State']['windows']
t = time.time()
while time.time() - t < 25 and not wins(): time.sleep(0.2)
w0 = wins()
print('first window:', [(w['id'], w['app_id'], w['title'][:20], w['suspended']) for w in w0])
if not w0: print('NO WINDOW from the first launch'); first.kill(); dw.terminate(); sys.exit(2)
r = ipc({'Suspend': w0[0]['id']}); print('suspend:', r)
time.sleep(1.5)
try: first.wait(5)
except Exception: first.kill()
w1 = wins(); print('after suspend:', [(w['id'], w['app_id'], w['suspended'], w['position'], w['size']) for w in w1])
pos = [(w['position'], w['size']) for w in w1]
entry(CMD2.replace('{AID}', 'zz-different-appid'))
time.sleep(0.5)
print('relaunch:', ipc({'Relaunch': w1[0]['id']}))
time.sleep(9)   # longer than the 5 s fallback window
w2 = wins()
print('after relaunch (9 s):', [(w['id'], w['app_id'], w['suspended'], w['position'], w['size']) for w in w2])
adopted = any(w['app_id'] == 'zz-different-appid' and not w['suspended'] and (w['position'], w['size']) == pos[0] for w in w2)
print('RESULT: token presented and window adopted into the stand-in slot:', adopted)
for l in open(OUT + '/dw.log', errors='replace'):
    if 'ctivation' in l or 'dopt' in l or 'elaunch' in l: print('LOG', l.strip()[:200])
dw.terminate()
