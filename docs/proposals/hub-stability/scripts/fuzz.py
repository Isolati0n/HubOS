#!/usr/bin/env python3
"""Structure-aware Wayland protocol fuzzer (own tool, written 2026-10-05 for the hub-stability research).
Reads the protocol XML files, connects to a compositor socket, binds every global it knows, then sends random
but well-framed requests (valid object ids most of the time, random/edge-case arguments, memfd file descriptors).
Detects a dead compositor by pid, logs the last messages to a file for replay by seed.
usage: fuzz.py SOCKET_PATH COMPOSITOR_PID SECONDS OUTDIR SEED
"""
import os, sys, socket, struct, random, time, glob, array, signal, json
import xml.etree.ElementTree as ET

SOCK, PID, SECS, OUT, SEED = sys.argv[1], int(sys.argv[2]), float(sys.argv[3]), sys.argv[4], int(sys.argv[5])
S = os.environ['HS_WORK']
rnd = random.Random(SEED)
os.makedirs(OUT, exist_ok=True)

ifaces = {}
files = glob.glob(S + '/hubstab/proto/all/*.xml')
for f in files:
    try:
        root = ET.parse(f).getroot()
    except Exception:
        continue
    for i in root.iter('interface'):
        d = {'version': int(i.get('version', '1')), 'req': [], 'ev': []}
        for kind, key in (('request', 'req'), ('event', 'ev')):
            for m in i.findall(kind):
                args = []
                for a in m.findall('arg'):
                    args.append((a.get('name'), a.get('type'), a.get('interface'), a.get('allow-null') == 'true'))
                d[key].append((m.get('name'), args, m.get('type') == 'destructor'))
        ifaces[i.get('name')] = d
print('loaded interfaces:', len(ifaces), 'from', len(files), 'files', flush=True)

SKIP = set(filter(None, os.environ.get('FUZZ_SKIP', '').split(',')))   # requests already known to trip a debug_assert
STRS = [b'', b'a', b'hubos-fuzz', b'x' * 300, b'\xff\xfe\xc3\x28', b'%s%n%x', b'../../etc/passwd', b'A' * 5000, 'é漢字'.encode()]
INTS = [0, 0, 1, 1, 2, 3, 4, 10, 16, 100, 255, 256, 640, 1000, 4096, 65535, -1, 0x7fffffff, -0x80000000, 1 << 24]
UINTS = [0, 1, 2, 3, 4, 8, 16, 255, 256, 4096, 65535, 0xffffffff, 0x7fffffff, 0x80000000, 1 << 24]

class Conn:
    def __init__(self):
        self.s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.s.connect(SOCK)
        self.s.setblocking(False)
        self.objs = {1: 'wl_display'}
        self.next_id = 2
        self.rx = b''
        self.log = []
        self.dead = False
        self.nsent = 0

    def send(self, obj, opcode, payload, fds=()):
        msg = struct.pack('<II', obj, ((8 + len(payload)) << 16) | opcode) + payload
        self.log.append((obj, opcode, payload.hex()[:200], len(fds)))
        if len(self.log) > 300: self.log.pop(0)
        try:
            if fds:
                self.s.sendmsg([msg], [(socket.SOL_SOCKET, socket.SCM_RIGHTS, array.array('i', fds))])
            else:
                self.s.send(msg)
            self.nsent += 1
        except BlockingIOError:
            pass
        except (BrokenPipeError, ConnectionResetError, OSError):
            self.dead = True

    def alloc(self, iface):
        i = self.next_id; self.next_id += 1; self.objs[i] = iface; return i

    def pump(self):
        try:
            data, anc, flags, addr = self.s.recvmsg(65536, 4096)
            for level, typ, d in anc:
                if level == socket.SOL_SOCKET and typ == socket.SCM_RIGHTS:
                    n = len(d) // 4
                    for fd in array.array('i', d[:n * 4]):
                        os.close(fd)
            if not data:
                self.dead = True; return
            self.rx += data
        except BlockingIOError:
            return
        except (ConnectionResetError, OSError):
            self.dead = True; return
        events = []
        while len(self.rx) >= 8:
            obj, w = struct.unpack('<II', self.rx[:8])
            size, op = w >> 16, w & 0xffff
            if size < 8 or len(self.rx) < size: break
            body = self.rx[8:size]; self.rx = self.rx[size:]
            events.append((obj, op, body))
            self.on_event(obj, op, body)
        return events

    def on_event(self, obj, op, body):
        iface = self.objs.get(obj)
        if iface is None: return
        evs = ifaces.get(iface, {}).get('ev', [])
        if iface == 'wl_display' and op == 1:
            (oid,) = struct.unpack('<I', body[:4]); self.objs.pop(oid, None); return
        if iface == 'wl_display' and op == 0:
            self.dead = True; return
        if iface == 'wl_registry' and op == 0:
            name, ln = struct.unpack('<II', body[:8])
            sname = body[8:8 + ln - 1].decode(errors='replace')
            pad = (ln + 3) & ~3
            (ver,) = struct.unpack('<I', body[8 + pad:12 + pad])
            self.globals.append((name, sname, ver)); return
        if op < len(evs):
            off = 0
            for (an, at, ai, nl) in evs[op][1]:
                try:
                    if at in ('int', 'uint', 'fixed', 'object'): off += 4
                    elif at == 'new_id':
                        (nid,) = struct.unpack('<I', body[off:off + 4]); off += 4
                        if ai: self.objs[nid] = ai
                    elif at in ('string', 'array'):
                        (ln,) = struct.unpack('<I', body[off:off + 4]); off += 4 + ((ln + 3) & ~3)
                except struct.error:
                    break

def enc_str(b):
    b = b + b'\0'
    return struct.pack('<I', len(b)) + b + b'\0' * ((4 - len(b) % 4) % 4)

def enc_arr(b):
    return struct.pack('<I', len(b)) + b + b'\0' * ((4 - len(b) % 4) % 4)

def make_fd():
    fd = os.memfd_create('fz')
    k = rnd.random()
    size = rnd.choice([0, 1, 4096, 65536, 1 << 20, 4 << 20, 1 << 30]) if k < .9 else rnd.randint(0, 1 << 16)
    try:
        os.ftruncate(fd, size)
        if size and size <= (1 << 20):
            os.write(fd, bytes(rnd.getrandbits(8) for _ in range(min(size, 256))))
    except OSError:
        pass
    return fd

def gen_args(c, args, fds):
    out = b''
    for (an, at, ai, nl) in args:
        if at == 'int': out += struct.pack('<i', rnd.choice(INTS) if rnd.random() < .7 else rnd.randint(-5000, 5000))
        elif at == 'uint': out += struct.pack('<I', rnd.choice(UINTS) if rnd.random() < .7 else rnd.randint(0, 5000))
        elif at == 'fixed': out += struct.pack('<i', rnd.choice(INTS))
        elif at == 'string':
            out += enc_str(rnd.choice(STRS)) if not (nl and rnd.random() < .3) else struct.pack('<I', 0)
        elif at == 'array': out += enc_arr(bytes(rnd.getrandbits(8) for _ in range(rnd.choice([0, 4, 8, 12, 64]))))
        elif at == 'fd':
            fd = make_fd(); fds.append(fd)
        elif at == 'object':
            cands = [i for i, n in c.objs.items() if n == ai] if ai else list(c.objs)
            if cands and rnd.random() < .9: out += struct.pack('<I', rnd.choice(cands))
            elif nl and rnd.random() < .5: out += struct.pack('<I', 0)
            else: out += struct.pack('<I', rnd.choice([0, 1, 2, 50, 0xffffffff, rnd.randint(1, c.next_id + 5)]))
        elif at == 'new_id':
            if ai: out += struct.pack('<I', c.alloc(ai))
            else:  # generic new_id (wl_registry.bind): interface string, version, id
                out += enc_str(b'wl_output') + struct.pack('<II', 1, c.alloc('wl_output'))
    return out

def setup(c):
    c.globals = []
    rid = c.alloc('wl_registry')
    c.send(1, 1, struct.pack('<I', rid))
    t0 = time.time()
    while time.time() - t0 < 0.4:
        c.pump(); time.sleep(0.005)
        if c.dead: return
    for (name, sname, ver) in c.globals:
        if sname in ifaces:
            v = min(ver, ifaces[sname]['version'])
            nid = c.alloc(sname)
            c.send(rid, 0, struct.pack('<I', name) + enc_str(sname.encode()) + struct.pack('<II', v, nid))
    # a plausible valid start: surface -> xdg toplevel, so later requests hit real windows
    def first(n):
        l = [i for i, x in c.objs.items() if x == n]
        return l[0] if l else None
    comp, wm = first('wl_compositor'), first('xdg_wm_base')
    if comp and wm:
        sid = c.alloc('wl_surface'); c.send(comp, 0, struct.pack('<I', sid))
        xid = c.alloc('xdg_surface'); c.send(wm, 2, struct.pack('<II', xid, sid))
        tid = c.alloc('xdg_toplevel'); c.send(xid, 1, struct.pack('<I', tid))
        c.send(sid, 6, b'')  # commit
    c.pump()

def step(c):
    cands = [(i, n) for i, n in c.objs.items() if n in ifaces and ifaces[n]['req']]
    if not cands: return
    # bias toward objects that are not the display/registry
    oid, n = rnd.choice(cands)
    reqs = ifaces[n]['req']
    op = rnd.randrange(len(reqs))
    if op < len(reqs) and (n + '.' + reqs[op][0]) in SKIP:
        return
    if rnd.random() < 0.005: op = len(reqs) + rnd.randint(0, 3)  # out of range opcode
    fds = []
    if op < len(reqs):
        payload = gen_args(c, reqs[op][1], fds)
        if reqs[op][2]:  # destructor: object dies (client side forgets after delete_id)
            pass
    else:
        payload = b''
    if oid == 1 and op == 0 and rnd.random() < .9:  # avoid wl_display.sync spam? keep
        pass
    c.send(oid, op, payload, fds)
    for fd in fds: os.close(fd)

def alive():
    try:
        os.kill(PID, 0); return True
    except OSError:
        return False

t_end = time.time() + SECS
conns = 0; sent = 0; kills = 0
stats = {'seed': SEED, 'start': time.time()}
while time.time() < t_end:
    try:
        c = Conn()
    except OSError:
        if not alive(): break
        time.sleep(0.2); continue
    conns += 1
    setup(c)
    n = 0
    lim = rnd.choice([200, 1000, 5000])
    while not c.dead and n < lim and time.time() < t_end:
        step(c); n += 1
        if n % 7 == 0: c.pump()
        if n % 50 == 0 and not alive(): break
    sent += c.nsent
    if c.dead: kills += 1
    if not alive():
        json.dump({'seed': SEED, 'conn': conns, 'log': c.log[-300:]}, open(f'{OUT}/crash-seed{SEED}-conn{conns}.json', 'w'))
        print(f'COMPOSITOR DEAD after {conns} connections, {sent} messages; log saved', flush=True)
        break
    try: c.s.close()
    except Exception: pass
    if conns % 20 == 0:
        print(f'conns={conns} sent={sent} clients_killed_by_compositor={kills} alive={alive()}', flush=True)
print(f'done conns={conns} sent={sent} killed_by_compositor={kills} alive={alive()}', flush=True)
