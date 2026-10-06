#!/usr/bin/env python3
"""repro_ipc.py WAYLAND_SOCKET VARIANT : one IPC request that may kill the compositor.  Variants: shot_neg_scale, shot_zero_scale, shot_region_h0_negscale, shot_region_zero, shot_region_neg, shot_huge_scale"""
import glob, json, os, socket, sys, time
wl, var = sys.argv[1], sys.argv[2]
RT = os.path.dirname(wl)
OUTD = os.environ.get('REPRO_OUT', '/tmp')
p = OUTD + '/repro-shot.png'
V = {
 'shot_neg_scale': {'Screenshot': {'target': 'Viewport', 'scale': -1.0, 'path': p}},
 'shot_zero_scale': {'Screenshot': {'target': 'Viewport', 'scale': 0.0, 'path': p}},
 'shot_region_h0_negscale': {'Screenshot': {'target': {'Region': {'x': 1, 'y': 1480, 'w': 1000, 'h': 0, 'from_screen': True}}, 'scale': -1.0, 'path': p}},
 'shot_region_zero': {'Screenshot': {'target': {'Region': {'x': 0, 'y': 0, 'w': 0, 'h': 0, 'from_screen': False}}, 'scale': 1.0, 'path': p}},
 'shot_region_neg': {'Screenshot': {'target': {'Region': {'x': 0, 'y': 0, 'w': -5, 'h': -5, 'from_screen': False}}, 'scale': 1.0, 'path': p}},
 'shot_region_h0': {'Screenshot': {'target': {'Region': {'x': 0, 'y': 0, 'w': 1000, 'h': 0, 'from_screen': False}}, 'scale': 1.0, 'path': p}},
 'shot_huge_scale': {'Screenshot': {'target': 'Viewport', 'scale': 1e6, 'path': p}},
}
def rpc(req):
    s = socket.socket(socket.AF_UNIX); s.settimeout(8)
    s.connect(glob.glob(RT + '/driftwm/ipc-*.sock')[0])
    s.sendall((json.dumps(req) + '\n').encode())
    try: return s.recv(2000000).decode().strip()
    except Exception as e: return 'no reply: ' + type(e).__name__
if var.startswith('nav_far'):
    import subprocess
    env = dict(os.environ); env['WAYLAND_DISPLAY'] = os.path.basename(wl); env['XDG_RUNTIME_DIR'] = RT
    for i in range(2): subprocess.Popen(['foot', f'--app-id=repro{i}', 'sleep', '100'], env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    time.sleep(3)
    ws_ = json.loads(rpc('State'))['Ok']['State']['windows']
    print('Move ->', rpc({'Move': {'window': ws_[0]['id'], 'to': [65535, 2147483647]}})[:80])
    print('Action center-nearest left ->', rpc({'Action': 'center-nearest left'})[:100])
    time.sleep(1)
    print('State after ->', rpc('State')[:40])
elif var.startswith('standin_resize'):
    import subprocess
    env = dict(os.environ); env['WAYLAND_DISPLAY'] = os.path.basename(wl); env['XDG_RUNTIME_DIR'] = RT
    d_ = os.environ.get('XDG_DATA_HOME', '/tmp') + '/applications'; os.makedirs(d_, exist_ok=True)
    open(d_ + '/repro.desktop', 'w').write("[Desktop Entry]\nType=Application\nName=repro\nExec=foot --app-id=repro sleep 100\nStartupWMClass=repro\n")
    time.sleep(1)
    subprocess.Popen(['foot', '--app-id=repro', 'sleep', '100'], env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    time.sleep(3)
    wid = json.loads(rpc('State'))['Ok']['State']['windows'][0]['id']
    print('Suspend ->', rpc({'Suspend': wid})[:80])
    time.sleep(1)
    n_ = int(var.split('_')[-1])
    print('Resize stand-in ->', rpc({'Resize': {'window': wid, 'to': [n_, n_]}})[:100])
    time.sleep(2)
    print('State after ->', rpc('State')[:40])
elif var.startswith('resize_big'):
    import subprocess
    env = dict(os.environ); env['WAYLAND_DISPLAY'] = os.path.basename(wl); env['XDG_RUNTIME_DIR'] = RT
    os.makedirs(os.environ.get('XDG_DATA_HOME', '/tmp') + '/applications', exist_ok=True)
    subprocess.Popen(['foot', '--app-id=repro', 'sleep', '100'], env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    time.sleep(3)
    wid = json.loads(rpc('State'))['Ok']['State']['windows'][0]['id']
    n_ = {'resize_big_live_65535': 65535, 'resize_big_live_32768': 32768, 'resize_big_live_20000': 20000, 'resize_big_live_16384': 16384}.get(var, 65535)
    print('Resize ->', rpc({'Resize': {'window': wid, 'to': [n_, n_]}})[:100])
    time.sleep(2)
    print('State after ->', rpc('State')[:40])
elif var.startswith('two_far'):
    import subprocess
    env = dict(os.environ); env['WAYLAND_DISPLAY'] = os.path.basename(wl); env['XDG_RUNTIME_DIR'] = RT
    for i in range(2): subprocess.Popen(['foot', f'--app-id=repro{i}', 'sleep', '100'], env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    time.sleep(3)
    ws_ = json.loads(rpc('State'))['Ok']['State']['windows']
    far = {'two_far_max': 2147483647, 'two_far_1e9': 1000000000, 'two_far_1e6': 1000000}[var]
    print('Move A ->', rpc({'Move': {'window': ws_[0]['id'], 'to': [-far, 0]}})[:70])
    print('Move B ->', rpc({'Move': {'window': ws_[1]['id'], 'to': [far, 0]}})[:70])
    print('Screenshot All ->', rpc({'Screenshot': {'target': 'All', 'scale': 0.1, 'path': p}})[:160])
    print('State after ->', rpc('State')[:40])
elif var.startswith('move_then_'):
    import subprocess
    env = dict(os.environ); env['WAYLAND_DISPLAY'] = os.path.basename(wl); env['XDG_RUNTIME_DIR'] = RT
    subprocess.Popen(['foot', '--app-id=repro', 'sleep', '100'], env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    time.sleep(3)
    wid = json.loads(rpc('State'))['Ok']['State']['windows'][0]['id']
    x = {'move_then_shot_all': 2147483647, 'move_then_shot_all_1e9': 1000000000, 'move_then_shot_all_1e6': 1000000}[var]
    print('Move ->', rpc({'Move': {'window': wid, 'to': [x, x]}})[:100])
    print('Screenshot All ->', rpc({'Screenshot': {'target': 'All', 'scale': 0.1, 'path': p}})[:200])
    print('State after ->', rpc('State')[:60])
else:
    print(var, '->', rpc(V[var])[:150])
