#!/usr/bin/env python3
"""Resource test: one client maps N toplevels, each with a WxH XRGB shm buffer (sparse memfd), and commits them.
Measures the compositor's VmHWM / RSS before and after.   usage: repro_bigbuf.py SOCKET PID N W H"""
import array, os, socket, struct, sys, time
sock, pid, N, Wd, Ht = sys.argv[1], int(sys.argv[2]), int(sys.argv[3]), int(sys.argv[4]), int(sys.argv[5])
def vm(k):
    for l in open(f'/proc/{pid}/status'):
        if l.startswith(k + ':'): return int(l.split()[1]) // 1024
s = socket.socket(socket.AF_UNIX); s.connect(sock); s.settimeout(5)
n = [1]
def new(): n[0] += 1; return n[0]
def msg(o, op, p=b'', fds=()):
    d = struct.pack('<II', o, ((8 + len(p)) << 16) | op) + p
    if fds: s.sendmsg([d], [(socket.SOL_SOCKET, socket.SCM_RIGHTS, array.array('i', list(fds)))])
    else: s.sendall(d)
def u(*v): return struct.pack('<' + 'I' * len(v), *v)
def st(b): b = b + b'\0'; return u(len(b)) + b + b'\0' * ((4 - len(b) % 4) % 4)
rx = b''
def events(t=1.0):
    global rx
    s.settimeout(t); out = []
    try:
        rx += s.recv(65536)
    except Exception: pass
    while len(rx) >= 8:
        o, w = struct.unpack('<II', rx[:8]); sz = w >> 16
        if len(rx) < sz: break
        out.append((o, w & 0xffff, rx[8:sz])); rx = rx[sz:]
    return out
reg = new(); msg(1, 1, u(reg)); g = {}
for o, op, b in events(1.0):
    if o == reg and op == 0:
        name, ln = struct.unpack('<II', b[:8]); g[b[8:8 + ln - 1].decode()] = name
def bind(i, v=1):
    x = new(); msg(reg, 0, u(g[i]) + st(i.encode()) + u(v, x)); return x
comp, shm, wm = bind('wl_compositor'), bind('wl_shm'), bind('xdg_wm_base', 1)
print(f'before: compositor VmRSS={vm("VmRSS")} MB VmHWM={vm("VmHWM")} MB')
t0 = time.time()
size = Wd * Ht * 4
for i in range(N):
    sf = new(); msg(comp, 0, u(sf)); x = new(); msg(wm, 2, u(x, sf)); t = new(); msg(x, 1, u(t)); msg(sf, 6)
    ser = None
    for _ in range(20):
        for o, op, b in events(0.5):
            if o == x and op == 0: ser = struct.unpack('<I', b[:4])[0]
        if ser is not None: break
    if ser is not None: msg(x, 4, u(ser))
    fd = os.memfd_create('big'); os.ftruncate(fd, size)
    pool = new(); msg(shm, 0, u(pool) + struct.pack('<i', size), [fd]); os.close(fd)
    buf = new(); msg(pool, 0, u(buf, 0, Wd, Ht, Wd * 4, 1))
    msg(sf, 1, u(buf, 0, 0)); msg(sf, 2, struct.pack('<iiii', 0, 0, Wd, Ht)); msg(sf, 6)
    events(0.2)
time.sleep(3)
print(f'after {N} windows of {Wd}x{Ht} ({size >> 20} MB each) in {time.time() - t0:.1f}s: compositor VmRSS={vm("VmRSS")} MB VmHWM={vm("VmHWM")} MB')
msg(1, 0, u(new()))
try: s.settimeout(10); d = s.recv(4096); print('compositor still answering:', len(d) > 0)
except Exception as e: print('no answer', type(e).__name__)
