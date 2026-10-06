#!/usr/bin/env python3
"""fuzz2.py - stateful Wayland client fuzzer for the bulletproof-compositor research (2026-10-05..).
Differences from hub-stability/scripts/fuzz.py:
  * answers xdg_wm_base.ping, acks xdg_surface/layer_surface configure serials (so windows really map),
  * hand-written scenarios that reach deep states (shm buffers attached+committed, toplevels, popups with
    positioners, subsurfaces, viewport, layer surfaces, data device selection and drag, activation tokens,
    pointer constraints, cursor surfaces, decorations, ...), mixed with generic random requests whose
    arguments use valid enum values / existing objects most of the time,
  * connection modes: normal, noread (never reads events), torn (partial messages), garbage (random bytes),
    flip (valid stream with random bit flips), fdflood (many fds in one message),
  * coverage table: per 'interface.request' how often it was sent and how often the connection survived it,
  * full per-connection replay log (raw bytes) for minimal reproductions.
usage: fuzz2.py SOCKET COMPOSITOR_PID SECONDS OUTDIR SEED [PROTO_DIR]
env FUZZ_SKIP=iface.req,iface.req   requests never sent
"""
import os, sys, socket, struct, random, time, glob, array, json, collections
import xml.etree.ElementTree as ET

SOCK, PID, SECS, OUT, SEED = sys.argv[1], int(sys.argv[2]), float(sys.argv[3]), sys.argv[4], int(sys.argv[5])
PROTO = sys.argv[6] if len(sys.argv) > 6 else os.environ.get('FUZZ_PROTO', '')
rnd = random.Random(SEED)
os.makedirs(OUT, exist_ok=True)
SKIP = set(filter(None, os.environ.get('FUZZ_SKIP', '').split(',')))
MODES = os.environ.get('FUZZ_MODES', 'normal:70,noread:6,torn:5,garbage:4,flip:10,fdflood:5')

ifaces = {}
for f in glob.glob(PROTO + '/*.xml'):
    try: root = ET.parse(f).getroot()
    except Exception: continue
    for i in root.iter('interface'):
        d = {'version': int(i.get('version', '1')), 'req': [], 'ev': [], 'enums': {}}
        for e in i.findall('enum'):
            d['enums'][e.get('name')] = [int(x.get('value'), 0) for x in e.findall('entry')]
        for kind, key in (('request', 'req'), ('event', 'ev')):
            for m in i.findall(kind):
                args = [(a.get('name'), a.get('type'), a.get('interface'), a.get('allow-null') == 'true', a.get('enum')) for a in m.findall('arg')]
                d[key].append((m.get('name'), args, m.get('type') == 'destructor'))
        ifaces[i.get('name')] = d

def enum_vals(iface, ename):
    if not ename: return None
    if '.' in ename: iface, ename = ename.split('.', 1)
    return ifaces.get(iface, {}).get('enums', {}).get(ename)

STRS = [b'', b'a', b'hubos-fuzz', b'x' * 300, b'\xff\xfe\xc3\x28', b'%s%n%x', b'../../etc/passwd', b'A' * 5000, 'é漢字'.encode(), b'text/plain', b'text/plain;charset=utf-8', b'image/png']
INTS = [0, 0, 1, 1, 2, 3, 4, 10, 16, 100, 255, 256, 640, 1000, 4096, 65535, -1, 0x7fffffff, -0x80000000, 1 << 24, 8192, 16384]
UINTS = [0, 1, 2, 3, 4, 8, 16, 255, 256, 4096, 65535, 0xffffffff, 0x7fffffff, 0x80000000, 1 << 24]

cov_sent = collections.Counter(); cov_ok = collections.Counter(); cov_err = collections.Counter(); errors_seen = collections.Counter()

def pick_mode():
    items = [(m.split(':')[0], int(m.split(':')[1])) for m in MODES.split(',')]
    r = rnd.randrange(sum(w for _, w in items))
    for m, w in items:
        if r < w: return m
        r -= w
    return 'normal'

def enc_str(b):
    b = b + b'\0'
    return struct.pack('<I', len(b)) + b + b'\0' * ((4 - len(b) % 4) % 4)
def enc_arr(b):
    return struct.pack('<I', len(b)) + b + b'\0' * ((4 - len(b) % 4) % 4)

class Conn:
    def __init__(self, mode):
        self.s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.s.connect(SOCK); self.s.setblocking(False)
        self.mode = mode
        self.objs = {1: 'wl_display'}; self.next_id = 2
        self.rx = b''; self.log = []; self.dead = False; self.nsent = 0
        self.globals = []; self.serials = {}  # obj -> last configure serial
        self.pending_ping = []; self.last_req = None; self.error = None
        self.surfaces = []; self.shm_buf_ok = []; self.cb_done = False
        self.fdq = []

    # ----- wire
    def send_raw(self, data, fds=(), key=None, tag=None):
        self.log.append((data.hex(), [os.fstat(f).st_size for f in fds], tag, key))
        try:
            if fds: self.s.sendmsg([data], [(socket.SOL_SOCKET, socket.SCM_RIGHTS, array.array('i', list(fds)))])
            else: self.s.send(data)
            self.nsent += 1
            if key: cov_sent[key] += 1; self.last_req = key
        except BlockingIOError: pass
        except OSError: self.dead = True

    def send(self, obj, opcode, payload, fds=(), tag=None):
        n = self.objs.get(obj)
        key = None
        if n in ifaces and opcode < len(ifaces[n]['req']): key = n + '.' + ifaces[n]['req'][opcode][0]
        if key in SKIP: return
        data = struct.pack('<II', obj, ((8 + len(payload)) << 16) | opcode) + payload
        self.send_raw(data, fds, key, tag)
        if key and ifaces[n]['req'][opcode][2]: self.objs.pop(obj, None)

    def alloc(self, iface):
        i = self.next_id; self.next_id += 1; self.objs[i] = iface; return i

    def first(self, n):
        l = [i for i, x in self.objs.items() if x == n]
        return l[0] if l else None
    def some(self, n):
        l = [i for i, x in self.objs.items() if x == n]
        return rnd.choice(l) if l else None

    def pump(self):
        if self.mode == 'noread': return
        for _ in range(8):
            try:
                data, anc, flags, addr = self.s.recvmsg(65536, 4096)
            except BlockingIOError: return
            except OSError: self.dead = True; return
            for level, typ, d in anc:
                if level == socket.SOL_SOCKET and typ == socket.SCM_RIGHTS:
                    n = len(d) // 4
                    for fd in array.array('i', d[:n * 4]): os.close(fd)
            if not data: self.dead = True; return
            self.rx += data
            while len(self.rx) >= 8:
                obj, w = struct.unpack('<II', self.rx[:8]); size, op = w >> 16, w & 0xffff
                if size < 8 or len(self.rx) < size: break
                body = self.rx[8:size]; self.rx = self.rx[size:]
                try: self.on_event(obj, op, body)
                except Exception: pass

    def parse_args(self, iface, op, body):
        evs = ifaces.get(iface, {}).get('ev', [])
        out = []; off = 0
        if op >= len(evs): return out
        for (an, at, ai, nl, en) in evs[op][1]:
            if at in ('int', 'uint', 'fixed', 'object'):
                out.append(struct.unpack('<I', body[off:off + 4])[0]); off += 4
            elif at == 'new_id':
                nid = struct.unpack('<I', body[off:off + 4])[0]; off += 4
                if ai: self.objs[nid] = ai
                out.append(nid)
            elif at in ('string', 'array'):
                ln = struct.unpack('<I', body[off:off + 4])[0]; off += 4 + ((ln + 3) & ~3); out.append(ln)
        return out

    def on_event(self, obj, op, body):
        iface = self.objs.get(obj)
        if iface is None: return
        if iface == 'wl_display':
            if op == 1: self.objs.pop(struct.unpack('<I', body[:4])[0], None); return
            if op == 0:
                oid, code = struct.unpack('<II', body[:8])
                ln = struct.unpack('<I', body[8:12])[0]; msg = body[12:12 + ln - 1].decode(errors='replace')
                oi = self.objs.get(oid, '?')
                cov_err[(self.last_req or '?')] += 1
                errors_seen[f'{oi}#{code}: {msg[:60]}'] += 1
                self.dead = True; return
        if iface == 'wl_registry' and op == 0:
            name, ln = struct.unpack('<II', body[:8]); sname = body[8:8 + ln - 1].decode(errors='replace')
            pad = (ln + 3) & ~3; ver = struct.unpack('<I', body[8 + pad:12 + pad])[0]
            self.globals.append((name, sname, ver)); return
        if iface == 'wl_callback': self.cb_done = True; self.objs.pop(obj, None); return
        a = self.parse_args(iface, op, body)
        evn = ifaces.get(iface, {}).get('ev', [])
        en = evn[op][0] if op < len(evn) else ''
        if iface == 'xdg_wm_base' and en == 'ping' and rnd.random() < 0.9: self.send(obj, 3, struct.pack('<I', a[0]), tag='serial')
        elif iface == 'xdg_surface' and en == 'configure': self.serials[obj] = a[0]; (rnd.random() < 0.85) and self.send(obj, 4, struct.pack('<I', a[0]), tag='serial')
        elif iface == 'zwlr_layer_surface_v1' and en == 'configure': self.serials[obj] = a[0]; (rnd.random() < 0.85) and self.send(obj, 6, struct.pack('<I', a[0]), tag='serial')

# ------------------------------------------------------------------ argument generation
def make_fd(size=None):
    fd = os.memfd_create('fz')
    if size is None:
        k = rnd.random()
        size = rnd.choice([0, 1, 4096, 65536, 1 << 20, 4 << 20, 1 << 30]) if k < .6 else rnd.randint(0, 1 << 16)
    try:
        os.ftruncate(fd, size)
        if 0 < size <= (1 << 20): os.write(fd, bytes(rnd.getrandbits(8) for _ in range(min(size, 512))))
    except OSError: pass
    return fd

def gen_args(c, iface, args, fds):
    out = b''
    for (an, at, ai, nl, en) in args:
        ev = enum_vals(iface, en)
        if at == 'int':
            out += struct.pack('<i', rnd.choice(ev) if ev and rnd.random() < .8 else (rnd.choice(INTS) if rnd.random() < .7 else rnd.randint(-5000, 5000)))
        elif at == 'uint':
            out += struct.pack('<I', rnd.choice(ev) if ev and rnd.random() < .8 else (rnd.choice(UINTS) if rnd.random() < .7 else rnd.randint(0, 5000)))
        elif at == 'fixed': out += struct.pack('<i', rnd.choice(INTS) * (rnd.choice([1, 256])))
        elif at == 'string':
            out += struct.pack('<I', 0) if (nl and rnd.random() < .3) else enc_str(rnd.choice(STRS))
        elif at == 'array': out += enc_arr(bytes(rnd.getrandbits(8) for _ in range(rnd.choice([0, 4, 8, 12, 64]))))
        elif at == 'fd': fds.append(make_fd())
        elif at == 'object':
            cands = [i for i, n in c.objs.items() if n == ai] if ai else list(c.objs)
            if cands and rnd.random() < .92: out += struct.pack('<I', rnd.choice(cands))
            elif nl and rnd.random() < .5: out += struct.pack('<I', 0)
            else: out += struct.pack('<I', rnd.choice([0, 1, 2, 50, 0xffffffff, rnd.randint(1, c.next_id + 5)]))
        elif at == 'new_id':
            out += struct.pack('<I', c.alloc(ai)) if ai else enc_str(b'wl_output') + struct.pack('<II', 1, c.alloc('wl_output'))
    return out

def req_index(n, name):
    for i, r in enumerate(ifaces[n]['req']):
        if r[0] == name: return i
    return None

def call(c, obj, name, *vals, fds=()):
    """hand-built request: vals are python ints/bytes already encoded per arg order"""
    n = c.objs[obj]; op = req_index(n, name)
    if op is None: return
    payload = b''
    for v in vals:
        if isinstance(v, bytes): payload += v
        elif isinstance(v, int): payload += struct.pack('<I', v & 0xffffffff)
    c.send(obj, op, payload, fds)

def S(b): return enc_str(b)

def roundtrip(c, timeout=1.0):
    cb = c.alloc('wl_callback'); c.cb_done = False
    c.send(1, 0, struct.pack('<I', cb))
    t0 = time.time()
    saved = c.mode
    if c.mode == 'noread': c.mode = 'tmp'
    while not c.cb_done and not c.dead and time.time() - t0 < timeout:
        c.pump()
        if not c.cb_done: time.sleep(0.002)
    c.mode = saved
    return c.cb_done

def setup(c):
    rid = c.alloc('wl_registry')
    c.send(1, 1, struct.pack('<I', rid))
    roundtrip(c, 2.0)
    if c.dead: return
    for (name, sname, ver) in c.globals:
        if sname in ifaces:
            v = min(ver, ifaces[sname]['version']); nid = c.alloc(sname)
            c.send(rid, 0, struct.pack('<I', name) + enc_str(sname.encode()) + struct.pack('<II', v, nid))
    seat = c.first('wl_seat')
    if seat:
        for nm, ifc in (('get_pointer', 'wl_pointer'), ('get_keyboard', 'wl_keyboard'), ('get_touch', 'wl_touch')):
            i = c.alloc(ifc); call(c, seat, nm, i)
    c.pump()

# ------------------------------------------------------------------ scenarios (valid-ish stateful actions)
def mk_buffer(c, w=None, h=None):
    shm = c.first('wl_shm')
    if not shm: return None
    w = w or rnd.choice([1, 16, 64, 200, 300, 640]); h = h or rnd.choice([1, 16, 64, 100, 200, 480])
    stride = w * 4; size = stride * h
    fd = make_fd(size + rnd.choice([0, 0, 0, 4096]))
    pool = c.alloc('wl_shm_pool'); call(c, shm, 'create_pool', pool, 0, size, fds=[fd]) if False else None
    # create_pool(new_id, fd, size): fd carries no payload bytes
    op = req_index('wl_shm', 'create_pool')
    c.send(shm, op, struct.pack('<I', pool) + struct.pack('<i', size), [fd]); os.close(fd)
    buf = c.alloc('wl_buffer')
    call(c, pool, 'create_buffer', buf, 0, w, h, stride, rnd.choice([0, 1]))
    if rnd.random() < .5: call(c, pool, 'destroy')
    return buf

def sc_surface(c):
    comp = c.first('wl_compositor')
    if not comp: return
    s = c.alloc('wl_surface'); call(c, comp, 'create_surface', s); c.surfaces.append(s)
    return s

def sc_toplevel(c):
    wm = c.first('xdg_wm_base'); s = sc_surface(c)
    if not (wm and s): return
    x = c.alloc('xdg_surface'); call(c, wm, 'get_xdg_surface', x, s)
    t = c.alloc('xdg_toplevel'); call(c, x, 'get_toplevel', t)
    call(c, s, 'commit')
    if rnd.random() < .9: roundtrip(c)
    b = mk_buffer(c)
    if b:
        call(c, s, 'attach', b, 0, 0); call(c, s, 'damage', 0, 0, 100, 100); call(c, s, 'commit')

def sc_toplevel_req(c):
    t = c.some('xdg_toplevel')
    if not t: return
    r = rnd.random()
    if r < .15: call(c, t, 'set_title', S(rnd.choice(STRS)))
    elif r < .3: call(c, t, 'set_app_id', S(rnd.choice([b'hubos-m1', b'foot', b'', b'x' * 200])))
    elif r < .4: call(c, t, 'set_fullscreen', c.some('wl_output') or 0)
    elif r < .45: call(c, t, 'unset_fullscreen')
    elif r < .55: call(c, t, 'set_maximized')
    elif r < .6: call(c, t, 'unset_maximized')
    elif r < .65: call(c, t, 'set_minimized')
    elif r < .72: call(c, t, 'set_min_size', rnd.choice(INTS), rnd.choice(INTS))
    elif r < .8: call(c, t, 'set_max_size', rnd.choice(INTS), rnd.choice(INTS))
    elif r < .88:
        seat = c.first('wl_seat');
        if seat: call(c, t, 'move', seat, rnd.choice(UINTS))
    elif r < .95:
        seat = c.first('wl_seat')
        if seat: call(c, t, 'resize', seat, rnd.choice(UINTS), rnd.choice([0, 1, 2, 4, 5, 6, 8, 9, 10, 99]))
    else: call(c, t, 'set_parent', c.some('xdg_toplevel') or 0)

def sc_popup(c):
    wm = c.first('xdg_wm_base'); s = sc_surface(c)
    if not (wm and s): return
    x = c.alloc('xdg_surface'); call(c, wm, 'get_xdg_surface', x, s)
    pos = c.alloc('xdg_positioner'); call(c, wm, 'create_positioner', pos)
    call(c, pos, 'set_size', rnd.choice(INTS), rnd.choice(INTS)); call(c, pos, 'set_anchor_rect', rnd.choice(INTS), rnd.choice(INTS), rnd.choice(INTS), rnd.choice(INTS))
    call(c, pos, 'set_anchor', rnd.randint(0, 9)); call(c, pos, 'set_gravity', rnd.randint(0, 9)); call(c, pos, 'set_constraint_adjustment', rnd.choice(UINTS))
    call(c, pos, 'set_offset', rnd.choice(INTS), rnd.choice(INTS))
    if rnd.random() < .4: call(c, pos, 'set_reactive')
    if rnd.random() < .3: call(c, pos, 'set_parent_size', rnd.choice(INTS), rnd.choice(INTS))
    parent = c.some('xdg_surface')
    if os.environ.get('FUZZ_AVOID_CYCLE') and parent == x:
        others = [i for i, n in c.objs.items() if n == 'xdg_surface' and i != x]
        parent = rnd.choice(others) if others else 0
    p = c.alloc('xdg_popup'); call(c, x, 'get_popup', p, parent or 0, pos)
    if rnd.random() < .5:
        seat = c.first('wl_seat')
        if seat: call(c, p, 'grab', seat, rnd.choice(UINTS))
    call(c, s, 'commit')
    b = mk_buffer(c)
    if b: call(c, s, 'attach', b, 0, 0); call(c, s, 'commit')
    if rnd.random() < .3:
        pos2 = c.alloc('xdg_positioner'); call(c, wm, 'create_positioner', pos2); call(c, pos2, 'set_size', 50, 50); call(c, pos2, 'set_anchor_rect', 0, 0, 10, 10)
        call(c, p, 'reposition', pos2, rnd.choice(UINTS))

def sc_subsurface(c):
    sub = c.first('wl_subcompositor'); parent = rnd.choice(c.surfaces) if c.surfaces else None
    s = sc_surface(c)
    if not (sub and parent and s): return
    ss = c.alloc('wl_subsurface'); call(c, sub, 'get_subsurface', ss, s, parent)
    r = rnd.random()
    call(c, ss, 'set_position', rnd.choice(INTS), rnd.choice(INTS))
    if r < .3: call(c, ss, 'set_desync')
    elif r < .5: call(c, ss, 'set_sync')
    elif r < .7: call(c, ss, 'place_above', rnd.choice(c.surfaces))
    elif r < .9: call(c, ss, 'place_below', rnd.choice(c.surfaces))
    b = mk_buffer(c, 32, 32)
    if b: call(c, s, 'attach', b, 0, 0); call(c, s, 'commit'); call(c, parent, 'commit')

def sc_attach(c):
    if not c.surfaces: return
    s = rnd.choice(c.surfaces); r = rnd.random()
    if r < .5:
        b = mk_buffer(c)
        if b: call(c, s, 'attach', b, rnd.choice([0, 0, 5, -5]), rnd.choice([0, 0, 5, -5]))
        call(c, s, 'damage_buffer', 0, 0, rnd.choice([1, 100, 100000]), rnd.choice([1, 100, 100000])); call(c, s, 'commit')
    elif r < .6: call(c, s, 'attach', 0, 0, 0); call(c, s, 'commit')
    elif r < .7: call(c, s, 'set_buffer_scale', rnd.choice([0, 1, 2, 3, -1, 100])); call(c, s, 'commit')
    elif r < .8: call(c, s, 'set_buffer_transform', rnd.randint(0, 9)); call(c, s, 'commit')
    elif r < .9:
        comp = c.first('wl_compositor')
        reg = c.alloc('wl_region'); call(c, comp, 'create_region', reg)
        call(c, reg, rnd.choice(['add', 'subtract']), rnd.choice(INTS), rnd.choice(INTS), rnd.choice(INTS), rnd.choice(INTS))
        call(c, s, rnd.choice(['set_input_region', 'set_opaque_region']), reg); call(c, s, 'commit')
        if rnd.random() < .5: call(c, reg, 'destroy')
    else:
        fr = c.alloc('wl_callback'); call(c, s, 'frame', fr); call(c, s, 'commit')

def sc_viewport(c):
    vp = c.first('wp_viewporter')
    if not (vp and c.surfaces): return
    s = rnd.choice(c.surfaces); v = c.alloc('wp_viewport'); call(c, vp, 'get_viewport', v, s)
    call(c, v, 'set_destination', rnd.choice(INTS), rnd.choice(INTS))
    call(c, v, 'set_source', rnd.choice(INTS) * 256, rnd.choice(INTS) * 256, rnd.choice(INTS) * 256, rnd.choice(INTS) * 256)
    call(c, s, 'commit')

def sc_fractional(c):
    fs = c.first('wp_fractional_scale_manager_v1')
    if fs and c.surfaces:
        o = c.alloc('wp_fractional_scale_v1'); call(c, fs, 'get_fractional_scale', o, rnd.choice(c.surfaces))

def sc_layer(c):
    ls = c.first('zwlr_layer_shell_v1'); s = sc_surface(c)
    if not (ls and s): return
    l = c.alloc('zwlr_layer_surface_v1')
    call(c, ls, 'get_layer_surface', l, s, c.first('wl_output') or 0, rnd.randint(0, 5), S(rnd.choice([b'bar', b'x', b''])))
    call(c, l, 'set_size', rnd.choice([0, 100, 1000, 100000]), rnd.choice([0, 30, 1000])); call(c, l, 'set_anchor', rnd.randint(0, 20))
    call(c, l, 'set_exclusive_zone', rnd.choice(INTS)); call(c, l, 'set_margin', rnd.choice(INTS), rnd.choice(INTS), rnd.choice(INTS), rnd.choice(INTS))
    call(c, l, 'set_keyboard_interactivity', rnd.randint(0, 3))
    call(c, s, 'commit'); roundtrip(c)
    b = mk_buffer(c, 100, 30)
    if b: call(c, s, 'attach', b, 0, 0); call(c, s, 'commit')
    if rnd.random() < .3:
        wm = c.first('xdg_wm_base'); x = c.alloc('xdg_surface'); s2 = sc_surface(c)
        call(c, wm, 'get_xdg_surface', x, s2)
        pos = c.alloc('xdg_positioner'); call(c, wm, 'create_positioner', pos); call(c, pos, 'set_size', 20, 20); call(c, pos, 'set_anchor_rect', 0, 0, 5, 5)
        p = c.alloc('xdg_popup'); call(c, x, 'get_popup', 0, pos) if False else None
        call(c, l, 'get_popup', p)

def sc_data(c):
    dm = c.first('wl_data_device_manager'); seat = c.first('wl_seat')
    if not (dm and seat): return
    r = rnd.random()
    if r < .4:
        src = c.alloc('wl_data_source'); call(c, dm, 'create_data_source', src); call(c, src, 'offer', S(rnd.choice(STRS)))
        dd = c.some('wl_data_device')
        if not dd: dd = c.alloc('wl_data_device'); call(c, dm, 'get_data_device', dd, seat)
        call(c, dd, 'set_selection', src, rnd.choice(UINTS + [1, 2, 3]))
    elif r < .7 and c.surfaces:
        src = c.alloc('wl_data_source'); call(c, dm, 'create_data_source', src); call(c, src, 'offer', S(b'text/plain'))
        call(c, src, 'set_actions', rnd.randint(0, 9))
        dd = c.some('wl_data_device') or c.alloc('wl_data_device')
        if dd not in [i for i, n in c.objs.items() if n == 'wl_data_device' and i != dd] and dd == c.next_id - 1: call(c, dm, 'get_data_device', dd, seat)
        icon = rnd.choice(c.surfaces) if rnd.random() < .5 else 0
        call(c, dd, 'start_drag', src, rnd.choice(c.surfaces), icon, rnd.choice(UINTS))
    else:
        ps = c.first('zwp_primary_selection_device_manager_v1')
        if ps:
            src = c.alloc('zwp_primary_selection_source_v1'); call(c, ps, 'create_source', src); call(c, src, 'offer', S(b'text/plain'))
            dev = c.alloc('zwp_primary_selection_device_v1'); call(c, ps, 'get_device', dev, seat); call(c, dev, 'set_selection', src, rnd.choice(UINTS))

def sc_activation(c):
    am = c.first('xdg_activation_v1')
    if not am: return
    t = c.alloc('xdg_activation_token_v1'); call(c, am, 'get_activation_token', t)
    if c.surfaces: call(c, t, 'set_surface', rnd.choice(c.surfaces))
    call(c, t, 'set_app_id', S(rnd.choice(STRS))); call(c, t, 'set_serial', rnd.choice(UINTS), c.first('wl_seat') or 0); call(c, t, 'commit')
    if c.surfaces: call(c, am, 'activate', S(rnd.choice(STRS + [b'0'])), rnd.choice(c.surfaces))

def sc_constraints(c):
    pc = c.first('zwp_pointer_constraints_v1'); ptr = c.first('wl_pointer')
    if pc and ptr and c.surfaces:
        o = c.alloc(rnd.choice(['zwp_locked_pointer_v1', 'zwp_confined_pointer_v1']))
        reg = 0
        if rnd.random() < .5:
            comp = c.first('wl_compositor'); reg = c.alloc('wl_region'); call(c, comp, 'create_region', reg); call(c, reg, 'add', 0, 0, 50, 50)
        call(c, pc, 'lock_pointer' if o and c.objs[o] == 'zwp_locked_pointer_v1' else 'confine_pointer', o, rnd.choice(c.surfaces), ptr, reg, rnd.randint(0, 3))
        if c.objs[o] == 'zwp_locked_pointer_v1' and rnd.random() < .5: call(c, o, 'set_cursor_position_hint', rnd.choice(INTS) * 256, rnd.choice(INTS) * 256);
        if rnd.random() < .5 and c.surfaces: call(c, rnd.choice(c.surfaces), 'commit')
    rp = c.first('zwp_relative_pointer_manager_v1')
    if rp and ptr:
        o = c.alloc('zwp_relative_pointer_v1'); call(c, rp, 'get_relative_pointer', o, ptr)

def sc_cursor(c):
    ptr = c.first('wl_pointer')
    if ptr and c.surfaces:
        call(c, ptr, 'set_cursor', rnd.choice(UINTS), rnd.choice(c.surfaces + [0]), rnd.choice(INTS), rnd.choice(INTS))
    cs = c.first('wp_cursor_shape_manager_v1')
    if cs and ptr:
        d = c.alloc('wp_cursor_shape_device_v1'); call(c, cs, 'get_pointer', d, ptr); call(c, d, 'set_shape', rnd.choice(UINTS), rnd.randint(0, 40))

def sc_decoration(c):
    dm = c.first('zxdg_decoration_manager_v1'); t = c.some('xdg_toplevel')
    if dm and t:
        d = c.alloc('zxdg_toplevel_decoration_v1'); call(c, dm, 'get_toplevel_decoration', d, t); call(c, d, rnd.choice(['set_mode', 'unset_mode']), rnd.randint(0, 3)) if False else call(c, d, 'set_mode', rnd.randint(0, 3))

def sc_destroy_random(c):
    cands = [i for i, n in c.objs.items() if n in ifaces and i != 1 and any(r[2] for r in ifaces[n]['req'])]
    if not cands: return
    o = rnd.choice(cands); n = c.objs[o]
    ops = [i for i, r in enumerate(ifaces[n]['req']) if r[2]]
    c.send(o, rnd.choice(ops), b'')
    c.surfaces = [s for s in c.surfaces if s in c.objs]

SCEN = [(sc_toplevel, 6), (sc_toplevel_req, 8), (sc_popup, 5), (sc_subsurface, 4), (sc_attach, 8), (sc_viewport, 2), (sc_fractional, 1), (sc_layer, 3),
        (sc_data, 4), (sc_activation, 2), (sc_constraints, 2), (sc_cursor, 2), (sc_decoration, 2), (sc_destroy_random, 6)]
SC_TOT = sum(w for _, w in SCEN)
def scenario(c):
    r = rnd.randrange(SC_TOT)
    for f, w in SCEN:
        if r < w: f(c); return
        r -= w

def generic(c):
    byif = collections.defaultdict(list)
    for i, n in c.objs.items():
        if n in ifaces and ifaces[n]['req'] and n not in ('wl_registry', 'wl_display'): byif[n].append(i)
    if not byif: return
    n = rnd.choice(list(byif)); oid = rnd.choice(byif[n]); reqs = ifaces[n]['req']; op = rnd.randrange(len(reqs))
    if (n + '.' + reqs[op][0]) in SKIP: return
    if rnd.random() < 0.005: op = len(reqs) + rnd.randint(0, 3)
    fds = []
    payload = gen_args(c, n, reqs[op][1], fds) if op < len(reqs) else b''
    if rnd.random() < 0.01: payload = payload[: rnd.randint(0, len(payload))]  # short payload
    if rnd.random() < 0.01: payload += bytes(rnd.getrandbits(8) for _ in range(rnd.choice([1, 3, 4, 64])))  # trailing junk
    c.send(oid, op, payload, fds)
    for fd in fds:
        try: os.close(fd)
        except OSError: pass

def alive():
    try: os.kill(PID, 0); return True
    except OSError: return False

def run_conn(c, lim, t_end):
    n = 0
    while not c.dead and n < lim and time.time() < t_end:
        m = c.mode
        if m == 'garbage':
            c.send_raw(bytes(rnd.getrandbits(8) for _ in range(rnd.choice([1, 7, 8, 64, 1000, 70000]))))
        elif m == 'torn':
            msg = struct.pack('<II', 1, (12 << 16) | 0)  # wl_display.sync header, new_id missing / partial
            c.send_raw(msg[:rnd.randint(1, 7)]); time.sleep(rnd.random() * 0.05)
            if rnd.random() < .3: c.send_raw(struct.pack('<II', 2, (0xffff << 16) | 1))  # absurd size
        elif m == 'fdflood':
            fds = [make_fd(0) for _ in range(rnd.choice([30, 200, 300]))]
            try: c.s.sendmsg([struct.pack('<II', 1, (8 << 16) | 99)], [(socket.SOL_SOCKET, socket.SCM_RIGHTS, array.array('i', fds))])
            except OSError: c.dead = True
            for f in fds: os.close(f)
        else:
            try: (scenario if rnd.random() < .6 else generic)(c)
            except (struct.error, KeyError, IndexError, TypeError, ValueError, OSError): pass
            if m == 'flip' and c.log and rnd.random() < .05:
                h, nf, _t, _k = c.log[-1]; b = bytearray.fromhex(h)
                if b: b[rnd.randrange(len(b))] ^= 1 << rnd.randrange(8); c.send_raw(bytes(b))
        n += 1
        if m != 'noread' and n % 5 == 0: c.pump()
        if n % 40 == 0 and not alive(): break
    return n

def probe(timeout=4.0):
    # fresh connection, wl_display.sync, wait for callback.done: is the event loop turning?
    try:
        q = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM); q.settimeout(timeout); q.connect(SOCK)
        q.sendall(struct.pack('<III', 1, (12 << 16) | 0, 2))
        buf = b''; t0 = time.time()
        while time.time() - t0 < timeout:
            buf += q.recv(4096)
            if len(buf) >= 8 and struct.unpack('<I', buf[:4])[0] == 2: q.close(); return True
        q.close(); return False
    except Exception: return False

recent_logs = collections.deque(maxlen=4)
GLOBALS_SEEN = set()
t_end = time.time() + SECS
conns = sent = kills = 0
while time.time() < t_end:
    try: c = Conn(pick_mode())
    except OSError:
        if not alive(): break
        time.sleep(0.2); continue
    conns += 1
    setup(c)
    GLOBALS_SEEN.update((g[1], g[2]) for g in c.globals)
    n = run_conn(c, rnd.choice([100, 400, 1500, 4000]), t_end)
    if c.mode == 'noread': time.sleep(rnd.random() * 0.2)
    if c.dead and c.mode != 'noread':
        c.dead = False; m_ = c.mode; c.pump(); c.dead = True or c.dead
    sent += c.nsent
    if c.dead: kills += 1
    for k_ in list(c.log[-1:]): pass
    if not alive():
        json.dump({'seed': SEED, 'conn': conns, 'mode': c.mode, 'log': c.log}, open(f'{OUT}/crash-seed{SEED}-conn{conns}.json', 'w'))
        print(f'COMPOSITOR DEAD after {conns} connections, {sent} messages; mode={c.mode} last={c.last_req}; log saved', flush=True)
        break
    recent_logs.append((conns, c.mode, list(c.log)))
    if alive() and not probe():
        time.sleep(1.0)
        if not probe(6.0):
            json.dump({'seed': SEED, 'conn': conns, 'hang': True, 'logs': [{'conn': a, 'mode': b, 'log': l} for a, b, l in recent_logs]}, open(f'{OUT}/hang-seed{SEED}-conn{conns}.json', 'w'))
            print(f'COMPOSITOR HUNG (event loop not answering wl_display.sync for 10 s) after connection {conns} mode={c.mode} last={c.last_req}; logs of last 4 connections saved', flush=True)
            break
    # a clean (non-error) survival of the last request counts as 'accepted'
    if not c.dead and c.last_req: cov_ok[c.last_req] += 1
    try: c.s.close()
    except Exception: pass
    if conns % 50 == 0: print(f'conns={conns} sent={sent} killed_by_compositor={kills} alive={alive()}', flush=True)
print(f'done conns={conns} sent={sent} killed_by_compositor={kills} alive={alive()}', flush=True)
json.dump({'sent': cov_sent, 'err': cov_err, 'errors': errors_seen, 'conns': conns, 'globals': sorted(GLOBALS_SEEN), 'modes': MODES}, open(f'{OUT}/coverage-seed{SEED}.json', 'w'))
