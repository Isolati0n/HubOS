#!/usr/bin/env python3
"""replay.py SOCKET LOG.json [--keep i,j,k | --range a:b] [--pid PID]
Replays the raw messages saved by fuzz2.py (crash-*.json: one connection; hang-*.json: the last connections).
Replies to xdg_wm_base.ping and acks xdg_surface configure like the fuzzer did (needed to reach the same state).
Exit code 0 = compositor still answering a wl_display.sync afterwards, 3 = dead, 4 = hung."""
import array, json, os, socket, struct, sys, time
sock, path = sys.argv[1], sys.argv[2]
keep = None; rng = None; pid = None
args = sys.argv[3:]
while args:
    a = args.pop(0)
    if a == '--keep': keep = [int(x) for x in args.pop(0).split(',') if x != '']
    elif a == '--range': x, y = args.pop(0).split(':'); rng = (int(x), int(y))
    elif a == '--pid': pid = int(args.pop(0))
d = json.load(open(path))
convs = d['logs'] if 'logs' in d else [{'log': d['log']}]
def alive():
    if pid is None: return True
    try: os.kill(pid, 0); return True
    except OSError: return False
def probe(timeout=4.0):
    try:
        q = socket.socket(socket.AF_UNIX); q.settimeout(timeout); q.connect(sock); q.sendall(struct.pack('<III', 1, (12 << 16) | 0, 2))
        buf = b''; t0 = time.time()
        while time.time() - t0 < timeout:
            buf += q.recv(4096)
            if len(buf) >= 8 and struct.unpack('<I', buf[:4])[0] == 2: return True
        return False
    except Exception: return False
total = 0
for ci, cv in enumerate(convs):
    log = cv['log']
    idx = list(range(len(log)))
    if keep is not None and ci == len(convs) - 1: idx = [i for i in keep if i < len(log)]
    if rng is not None and ci == len(convs) - 1: idx = idx[rng[0]:rng[1]]
    s = socket.socket(socket.AF_UNIX); s.connect(sock); s.setblocking(False)
    last_serial = {}; rxbuf = b''
    for i in idx:
        h, fdsz = log[i][0], log[i][1]
        tag = log[i][2] if len(log[i]) > 2 else None
        data = bytes.fromhex(h)
        if tag == 'serial':
            # the serial in an ack / pong was copied from an event of the original run: use the latest one of this run
            o = struct.unpack('<I', data[:4])[0]
            if o in last_serial: data = data[:8] + struct.pack('<I', last_serial[o]) + data[12:]
        fds = []
        for sz in fdsz:
            fd = os.memfd_create('rp')
            try: os.ftruncate(fd, sz)
            except OSError: pass
            fds.append(fd)
        try:
            if fds: s.sendmsg([data], [(socket.SOL_SOCKET, socket.SCM_RIGHTS, array.array('i', fds))])
            else: s.send(data)
        except (BlockingIOError, OSError): pass
        for fd in fds: os.close(fd)
        total += 1
        # drain events and answer ping/configure
        try:
            while True:
                ev = s.recv(65536)
                if not ev: break
                rxbuf += ev
                while len(rxbuf) >= 8:
                    o_, w_ = struct.unpack('<II', rxbuf[:8]); sz_ = w_ >> 16
                    if sz_ < 8 or len(rxbuf) < sz_: break
                    if (w_ & 0xffff) == 0 and sz_ >= 12: last_serial[o_] = struct.unpack('<I', rxbuf[8:12])[0]
                    rxbuf = rxbuf[sz_:]
        except (BlockingIOError, OSError): pass
        if not alive(): print(f'DEAD after message {i} of connection {ci}'); sys.exit(3)
        if i % 8 == 0: time.sleep(0.005)
    s.close()
time.sleep(0.5)
if not alive(): print('DEAD at the end'); sys.exit(3)
if probe(): print(f'replayed {total} messages: compositor alive and answering'); sys.exit(0)
print(f'replayed {total} messages: compositor HUNG'); sys.exit(4)
