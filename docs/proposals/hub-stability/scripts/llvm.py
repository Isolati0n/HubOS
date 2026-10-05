import os
#!/usr/bin/env python3
"""llvmpipe cost of 20 windows at 3840x2160 in the nested driftwm (software GL = Mesa llvmpipe).
usage: llvm.py OUTDIR VARIANT [LP_NUM_THREADS]     VARIANT: min | default | blur | shader | heavy
Measures driftwm CPU (utime+stime), RSS, achieved frames per second (Subscribe events) in three phases:
idle (30 s), camera glides (30 s), windows moving (30 s)."""
import json, os, random, signal, socket, subprocess, sys, time, glob, shutil, threading
S = os.environ['HS_WORK']
H = S + '/hubstab'
OUT = sys.argv[1]; VAR = sys.argv[2]; LPT = sys.argv[3] if len(sys.argv) > 3 else ''
DW = os.environ.get('DWBIN', H + '/bin/driftwm-pristine')
RT = '/tmp/hsv'; DISP = ':79'
shutil.rmtree(OUT, ignore_errors=True); os.makedirs(OUT); shutil.rmtree(RT, ignore_errors=True); os.makedirs(RT); os.chmod(RT, 0o700)
L = S + '/A/root/usr/lib/x86_64-linux-gnu'
ENV = dict(os.environ)
ENV.update({'XDG_RUNTIME_DIR': RT, 'DISPLAY': DISP, 'LD_LIBRARY_PATH': L, 'LIBGL_ALWAYS_SOFTWARE': '1', 'MESA_LOADER_DRIVER_OVERRIDE': 'swrast',
            '__EGL_VENDOR_LIBRARY_DIRS': S + '/A/root/usr/share/glvnd/egl_vendor.d', 'LIBGL_DRIVERS_PATH': L + '/dri', 'GBM_BACKENDS_PATH': L + '/gbm',
            'PATH': S + '/A/root/usr/bin:' + os.environ.get('PATH', '')})
ENV.pop('WAYLAND_DISPLAY', None)
if LPT: ENV['LP_NUM_THREADS'] = LPT
W = H + '/dw/extras/wallpapers/animated/fast_smoke.glsl'
CONFS = {
 'min': '[background]\ntype = "none"\n[effects]\nanimation_speed = 1.0\n',
 'default': '',
 'blur': '[decorations]\nblur = true\nopacity = 0.9\nopacity_focused = 0.9\n',
 'shader': f'[background]\ntype = "shader"\npath = "{W}"\n',
 'heavy': f'[background]\ntype = "shader"\npath = "{W}"\n[decorations]\nblur = true\nopacity = 0.9\nopacity_focused = 0.9\n',
}
open(OUT + '/c.toml', 'w').write(CONFS[VAR])
lf = open(OUT + '/dw.log', 'wb')
dw = subprocess.Popen([DW, '--backend', 'winit', '--config', OUT + '/c.toml'], env=ENV, stdout=lf, stderr=lf)
def ipcs(req, timeout=10):
    s = socket.socket(socket.AF_UNIX); s.settimeout(timeout)
    s.connect(glob.glob(RT + '/driftwm/ipc-*.sock')[0]); s.sendall((json.dumps(req) + '\n').encode())
    buf = b''
    while not buf.endswith(b'\n'):
        c = s.recv(1 << 20)
        if not c: break
        buf += c
    s.close()
    return json.loads(buf)
for _ in range(300):
    try: ipcs('State'); break
    except Exception: time.sleep(0.1)
sock = [os.path.basename(p) for p in glob.glob(RT + '/wayland-*') if not p.endswith('.lock')][0]
# the nested window starts 1280x800: make it fill the 3840x2160 X screen
for wid in subprocess.run(['xdotool', 'search', '--onlyvisible', '--name', ''], env=ENV, capture_output=True, text=True).stdout.split():
    subprocess.run(['xdotool', 'windowmove', wid, '0', '0'], env=ENV); subprocess.run(['xdotool', 'windowsize', wid, '3840', '2160'], env=ENV)
time.sleep(1.5)
e2 = dict(ENV); e2['WAYLAND_DISPLAY'] = sock
CMD = "i=0; while :; do echo line $i $(date +%T) and some more text to make the terminal do real work; i=$((i+1)); sleep 1; done"
feet = []
for n in range(20):
    feet.append(subprocess.Popen(['foot', f'--app-id=hubos-v{n:02d}', f'--title=v{n:02d}', 'sh', '-c', CMD], env=e2, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL))
    time.sleep(0.15)
time.sleep(4)
ws = ipcs('State')['Ok']['State']['windows']
st = ipcs('State')['Ok']['State']
print('outputs', st['outputs'], 'windows', len(ws), flush=True)
for n, w in enumerate(sorted(ws, key=lambda w: w['app_id'])):
    c, r = n % 5, n // 5
    ipcs({'Resize': {'window': w['id'], 'to': [700, 525]}})
    ipcs({'Move': {'window': w['id'], 'to': [-1400 + 700 * c, 790 - 525 * r]}})
ipcs({'Camera': [0.0, 0.0]}); ipcs({'Zoom': 1.0})
time.sleep(4)
def cpu(pid):
    f = open(f'/proc/{pid}/stat').read().rsplit(')', 1)[1].split()
    return (int(f[11]) + int(f[12])) / 100.0
def rss(pid):
    for l in open(f'/proc/{pid}/status'):
        if l.startswith('VmRSS'): return int(l.split()[1]) // 1024
frames = {'n': 0}; stop = {'v': False}
def subscriber():
    s = socket.socket(socket.AF_UNIX); s.settimeout(1)
    s.connect(glob.glob(RT + '/driftwm/ipc-*.sock')[0]); s.sendall(b'"Subscribe"\n')
    buf = b''
    while not stop['v']:
        try:
            d = s.recv(1 << 20)
            if not d: break
            buf += d
            while b'\n' in buf:
                line, buf = buf.split(b'\n', 1)
                if line.startswith(b'{"State"'): frames['n'] += 1
        except socket.timeout: pass
threading.Thread(target=subscriber, daemon=True).start()
time.sleep(1)
res = {}
def phase(name, secs, driver=None):
    c0, f0, t0 = cpu(dw.pid), frames['n'], time.time()
    xc0 = None
    if driver: th = threading.Thread(target=driver, args=(secs,)); th.start()
    time.sleep(secs)
    if driver: th.join()
    dt = time.time() - t0
    c1, f1 = cpu(dw.pid), frames['n']
    res[name] = {'secs': round(dt, 1), 'cpu_s': round(c1 - c0, 2), 'cpu_pct_of_one_core': round(100 * (c1 - c0) / dt, 1),
                 'frames': f1 - f0, 'fps': round((f1 - f0) / dt, 1), 'rss_mb': rss(dw.pid)}
    if f1 - f0: res[name]['cpu_ms_per_frame'] = round(1000 * (c1 - c0) / (f1 - f0), 1)
    print(name, res[name], flush=True)
def glide(secs):
    t = time.time(); k = 0
    while time.time() - t < secs - 1:
        ipcs({'Camera': [(-1 if k % 2 else 1) * 700.0, (1 if k % 3 else -1) * 400.0]}); k += 1; time.sleep(1.2)
def moving(secs):
    t = time.time(); r = random.Random(3)
    ws = ipcs('State')['Ok']['State']['windows']
    while time.time() - t < secs - 1:
        w = r.choice(ws); ipcs({'Move': {'window': w['id'], 'to': [r.randint(-1500, 1500), r.randint(-800, 800)]}}); time.sleep(0.5)
phase('idle', 30)
phase('camera_glides', 30, glide)
phase('windows_moving', 30, moving)
if os.environ.get('HOG'):
    for nice in (0, 19):
        hogs = [subprocess.Popen(['nice', '-n', str(nice), 'python3', '-c', 'while True: pass']) for _ in range(8)]
        time.sleep(1)
        phase(f'camera_glides_with_8_cpu_hogs_nice{nice}', 30, glide)
        t = time.time(); ipcs('State'); res[f'ipc_state_ms_with_hogs_nice{nice}'] = round((time.time() - t) * 1000, 1)
        for h in hogs: h.kill()
        time.sleep(1)
lat = []
for k in range(8):
    t = time.time()
    try: r = ipcs({'Screenshot': {'target': 'Viewport', 'scale': 0.05, 'path': RT + '/probe.png'}}, timeout=30)
    except Exception as ex: r = str(ex)
    lat.append(round((time.time() - t) * 1000));
res['render_probe_ms_scale0.05'] = lat; res['render_probe_reply'] = str(r)[:120]
t = time.time(); ipcs('State'); res['ipc_state_ms'] = round((time.time() - t) * 1000, 1)
res['config'] = VAR; res['lp_threads'] = LPT or 'default(4 cores)'
xv = subprocess.run(['pgrep', '-x', 'Xvfb'], capture_output=True, text=True).stdout.split()
json.dump(res, open(OUT + '/result.json', 'w'), indent=1)
stop['v'] = True
for p in feet: p.kill()
dw.terminate(); dw.wait()
