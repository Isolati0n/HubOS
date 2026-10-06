#!/usr/bin/env python3
"""leakloop.py SOCKET PID N : N times connect, map a toplevel with a 64x64 shm buffer + a popup + a subsurface, commit, disconnect.
Prints the compositor's VmRSS / open fds / threads every N/10 connections (a leak per connection would show as a straight line)."""
import array, os, socket, struct, sys, time
sock, pid, N = sys.argv[1], int(sys.argv[2]), int(sys.argv[3])
def vm(k):
    for l in open(f'/proc/{pid}/status'):
        if l.startswith(k + ':'): return int(l.split()[1])
def one():
    s = socket.socket(socket.AF_UNIX); s.connect(sock); s.settimeout(3); n = [1]; rx = [b'']
    def new(): n[0] += 1; return n[0]
    def u(*v): return struct.pack('<' + 'I' * len(v), *v)
    def msg(o, op, p=b'', fds=()):
        d = struct.pack('<II', o, ((8 + len(p)) << 16) | op) + p
        if fds: s.sendmsg([d], [(socket.SOL_SOCKET, socket.SCM_RIGHTS, array.array('i', list(fds)))])
        else: s.sendall(d)
    def events():
        try: rx[0] += s.recv(65536)
        except Exception: pass
        out = []
        while len(rx[0]) >= 8:
            o, w = struct.unpack('<II', rx[0][:8]); sz = w >> 16
            if len(rx[0]) < sz: break
            out.append((o, w & 0xffff, rx[0][8:sz])); rx[0] = rx[0][sz:]
        return out
    def st(b): b = b + b'\0'; return u(len(b)) + b + b'\0' * ((4 - len(b) % 4) % 4)
    reg = new(); msg(1, 1, u(reg)); g = {}
    for o, op, b in events():
        if o == reg and op == 0:
            name, ln = struct.unpack('<II', b[:8]); g[b[8:8 + ln - 1].decode()] = name
    def bind(i, v=1):
        x = new(); msg(reg, 0, u(g[i]) + st(i.encode()) + u(v, x)); return x
    comp, shm, wm, sub = bind('wl_compositor'), bind('wl_shm'), bind('xdg_wm_base', 1), bind('wl_subcompositor')
    sf = new(); msg(comp, 0, u(sf)); x = new(); msg(wm, 2, u(x, sf)); t = new(); msg(x, 1, u(t)); msg(t, 2, st(b'leak')); msg(sf, 6)
    ser = None
    for _ in range(10):
        for o, op, b in events():
            if o == x and op == 0: ser = struct.unpack('<I', b[:4])[0]
        if ser is not None: break
        time.sleep(0.005)
    if ser is not None: msg(x, 4, u(ser))
    fd = os.memfd_create('l'); os.ftruncate(fd, 64 * 64 * 4)
    pool = new(); msg(shm, 0, u(pool) + struct.pack('<i', 64 * 64 * 4), [fd]); os.close(fd)
    buf = new(); msg(pool, 0, u(buf, 0, 64, 64, 256, 1))
    msg(sf, 1, u(buf, 0, 0)); msg(sf, 2, struct.pack('<iiii', 0, 0, 64, 64)); msg(sf, 6)
    # a subsurface
    ch = new(); msg(comp, 0, u(ch)); ss = new(); msg(sub, 1, u(ss, ch, sf)); msg(ch, 1, u(buf, 0, 0)); msg(ch, 6); msg(sf, 6)
    # a popup
    pos = new(); msg(wm, 1, u(pos)); msg(pos, 1, u(20, 20)); msg(pos, 2, u(0, 0, 5, 5))
    ps = new(); msg(comp, 0, u(ps)); px = new(); msg(wm, 2, u(px, ps)); pp = new(); msg(px, 2, u(pp, x, pos)); msg(ps, 6)
    time.sleep(0.01); events()
    s.close()
t0 = time.time()
print(f'start: VmRSS={vm("VmRSS")//1024} MB fds={len(os.listdir(f"/proc/{pid}/fd"))} threads={vm("Threads")}')
for i in range(1, N + 1):
    one()
    if i % (N // 10) == 0:
        time.sleep(0.5)
        print(f'{i:6d} connections ({time.time() - t0:5.0f}s): VmRSS={vm("VmRSS")/1024:7.1f} MB  VmHWM={vm("VmHWM")/1024:7.1f} MB  fds={len(os.listdir(f"/proc/{pid}/fd"))} threads={vm("Threads")}', flush=True)
