#!/usr/bin/env python3
"""Does an extreme wl_surface.damage / damage_buffer / buffer_scale / viewport make the compositor die?
usage: repro_damage.py SOCKET PID [variant-name]   (without a name: tries every variant until the compositor dies)"""
import array, os, socket, struct, sys, time
sock, pid = sys.argv[1], int(sys.argv[2]); only = sys.argv[3] if len(sys.argv) > 3 else None
IMAX, IMIN = 0x7fffffff, -0x80000000
def alive():
    try: os.kill(pid, 0); return True
    except OSError: return False
class C:
    def __init__(s):
        s.s = socket.socket(socket.AF_UNIX); s.s.connect(sock); s.s.settimeout(3); s.n = 1; s.rx = b''; s.g = {}
        s.reg = s.new(); s.msg(1, 1, u(s.reg))
        for o, op, b in s.events(1.0):
            if o == s.reg and op == 0:
                name, ln = struct.unpack('<II', b[:8]); s.g[b[8:8 + ln - 1].decode()] = name
    def new(s): s.n += 1; return s.n
    def msg(s, o, op, p=b'', fds=()):
        d = struct.pack('<II', o, ((8 + len(p)) << 16) | op) + p
        if fds: s.s.sendmsg([d], [(socket.SOL_SOCKET, socket.SCM_RIGHTS, array.array('i', list(fds)))])
        else: s.s.sendall(d)
    def events(s, t=0.5):
        s.s.settimeout(t)
        try: s.rx += s.s.recv(65536)
        except Exception: pass
        out = []
        while len(s.rx) >= 8:
            o, w = struct.unpack('<II', s.rx[:8]); sz = w >> 16
            if len(s.rx) < sz: break
            out.append((o, w & 0xffff, s.rx[8:sz])); s.rx = s.rx[sz:]
        return out
    def bind(s, i, v=1):
        x = s.new(); s.msg(s.reg, 0, u(s.g[i]) + st(i.encode()) + u(v, x)); return x
def u(*v): return struct.pack('<' + 'I' * len(v), *[x & 0xffffffff for x in v])
def st(b): b = b + b'\0'; return u(len(b)) + b + b'\0' * ((4 - len(b) % 4) % 4)
VARIANTS = {
    'damage_imax': ('damage', (0, 0, IMAX, IMAX)), 'damage_neg': ('damage', (-1, -1, -1, -1)), 'damage_imin': ('damage', (IMIN, IMIN, IMAX, IMAX)),
    'damage_buffer_imax': ('damage_buffer', (0, 0, IMAX, IMAX)), 'damage_buffer_neg': ('damage_buffer', (-5, -5, -7, -7)), 'damage_big_offset': ('damage', (IMAX - 3, IMAX - 3, 10, 10)),
    'damage_zero': ('damage', (0, 0, 0, 0)), 'scale_big': ('scale', (1000000,)), 'scale_zero': ('scale', (0,)), 'scale_neg': ('scale', (-3,)),
    'viewport_dst_big': ('vpdst', (IMAX, IMAX)), 'viewport_src_big': ('vpsrc', (0, 0, IMAX, IMAX)), 'attach_offset_big': ('attach_off', (IMAX, IMAX)),
}
for n_ in (4096, 16384, 32768, 65536, 1 << 20, 1 << 24, 1 << 28):
    VARIANTS[f'vpdst_{n_}'] = ('vpdst', (n_, n_))
for (w_, h_) in ((65536, 1), (1 << 20, 1), (1 << 24, 1), (1 << 27, 1), (1, 1 << 24), (1, 1 << 27), (32768, 32768)):
    VARIANTS[f'buf_{w_}x{h_}'] = ('buf', (w_, h_))
for v_ in (IMAX, 1 << 30, 1 << 24):
    VARIANTS[f'subpos_{v_}'] = ('subpos', (v_, v_))
    VARIANTS[f'popupoff_{v_}'] = ('popupoff', (v_, v_))
    VARIANTS[f'layermargin_{v_}'] = ('layermargin', (v_, v_))
names = [only] if only else list(VARIANTS)
for name in names:
    if not alive(): print('compositor already dead before', name); break
    kind, args = VARIANTS[name]
    c = C()
    comp, shm, wm = c.bind('wl_compositor', 4), c.bind('wl_shm'), c.bind('xdg_wm_base')
    vp = c.bind('wp_viewporter') if kind.startswith('vp') and 'wp_viewporter' in c.g else None
    sf = c.new(); c.msg(comp, 0, u(sf)); x = c.new(); c.msg(wm, 2, u(x, sf)); t = c.new(); c.msg(x, 1, u(t)); c.msg(sf, 6)
    ser = None
    for _ in range(10):
        for o, op, b in c.events(0.5):
            if o == x and op == 0: ser = struct.unpack('<I', b[:4])[0]
        if ser is not None: break
    if ser is not None: c.msg(x, 4, u(ser))
    W_, H_ = (args if kind == 'buf' else (200, 150)); size = W_ * H_ * 4
    fd = os.memfd_create('d'); os.ftruncate(fd, size)
    pool = c.new(); c.msg(shm, 0, u(pool) + struct.pack('<i', size), [fd]); os.close(fd)
    buf = c.new(); c.msg(pool, 0, u(buf, 0, W_, H_, W_ * 4, 1))
    c.msg(sf, 1, u(buf, 0, 0))
    if kind == 'damage': c.msg(sf, 2, u(*args))
    elif kind == 'damage_buffer': c.msg(sf, 9, u(*args))
    elif kind == 'scale': c.msg(sf, 8, u(*args))
    elif kind == 'attach_off': c.msg(sf, 1, u(buf, *args))
    elif kind in ('vpdst', 'vpsrc') and vp:
        v = c.new(); c.msg(vp, 1, u(v, sf)); c.msg(v, 1 if kind == 'vpsrc' else 2, u(*args))
    c.msg(sf, 6)
    sg = lambda a: a if a < 0x80000000 else a - (1 << 32)
    if kind == 'subpos' and 'wl_subcompositor' in c.g:
        sub = c.bind('wl_subcompositor'); child = c.new(); c.msg(comp, 0, u(child)); ss = c.new(); c.msg(sub, 1, u(ss, child, sf))
        c.msg(ss, 1, struct.pack('<ii', *[sg(a) for a in args])); c.msg(ss, 5)
        fd2 = os.memfd_create('d2'); os.ftruncate(fd2, 64 * 64 * 4); pool2 = c.new(); c.msg(shm, 0, u(pool2) + struct.pack('<i', 64 * 64 * 4), [fd2]); os.close(fd2)
        b2 = c.new(); c.msg(pool2, 0, u(b2, 0, 64, 64, 256, 1)); c.msg(child, 1, u(b2, 0, 0)); c.msg(child, 6); c.msg(sf, 6)
    if kind == 'popupoff':
        pos = c.new(); c.msg(wm, 1, u(pos)); c.msg(pos, 1, u(50, 50)); c.msg(pos, 2, u(0, 0, 5, 5)); c.msg(pos, 6, struct.pack('<ii', *[sg(a) for a in args]))
        s2 = c.new(); c.msg(comp, 0, u(s2)); x2 = c.new(); c.msg(wm, 2, u(x2, s2)); p2 = c.new(); c.msg(x2, 2, u(p2, x, pos)); c.msg(s2, 6)
        c.events(0.5)
        fd2 = os.memfd_create('d3'); os.ftruncate(fd2, 50 * 50 * 4); pool2 = c.new(); c.msg(shm, 0, u(pool2) + struct.pack('<i', 50 * 50 * 4), [fd2]); os.close(fd2)
        b2 = c.new(); c.msg(pool2, 0, u(b2, 0, 50, 50, 200, 1)); c.msg(s2, 1, u(b2, 0, 0)); c.msg(s2, 6)
    if kind == 'layermargin' and 'zwlr_layer_shell_v1' in c.g:
        ls = c.bind('zwlr_layer_shell_v1', 1); s3 = c.new(); c.msg(comp, 0, u(s3)); l3 = c.new()
        c.msg(ls, 0, u(l3, s3, 0, 2) + st(b'x')); c.msg(l3, 0, u(100, 30)); c.msg(l3, 3, struct.pack('<iiii', *[sg(a) for a in (args[0], args[1], args[0], args[1])])); c.msg(s3, 6)
        c.events(0.5)
    time.sleep(1.5)
    ok = alive()
    print(f'{name:22s} -> compositor {"alive" if ok else "DEAD"}', flush=True)
    try: c.s.close()
    except Exception: pass
    if not ok: break
