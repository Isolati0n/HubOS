#!/usr/bin/env python3
"""rfbprobe: a tiny raw RFB (VNC) client that measures what a server sends under different request styles.

It does NOT draw anything and does NOT decode pictures. It reads the message framing, counts bytes and
updates, and skips the picture data. Python standard library only. No secrets: it only speaks to servers
with security type None (a test node on 127.0.0.1).

Written for docs/proposals/remote-display-lod.md (level of detail: pause, smaller size, lower frame rate).

usage:
  rfbprobe.py --port 5901 --phases 'normal:8,none:8,normal:8' [--pid wayvnc=1234 --pid sway=1200] [--enc zrle|raw]

A phase is NAME[@PARAM]:SECONDS. The connection stays open across phases, so you can see what happens when a
client stops asking and then asks again.

  normal            ask again at once after every update (what simple viewers do)
  paced@0.1         ask again, but never more often than every 0.1 s (a frame-rate cap made by the client)
  none              send no request at all (the client "pauses"); any request already outstanding is still answered
  cu                send EnableContinuousUpdates(on) for the whole screen, then no requests
  cuoff             send EnableContinuousUpdates(off), then no requests
  curegion@x+y+w+h  EnableContinuousUpdates(on) for a small region only
  resize@WxH        send SetDesktopSize (one screen) asking the node to render at WxH, then behave like 'normal'
  fence             like 'none', but send a Fence request every second and time the answer

Output: one line per phase (bytes/s, pixel updates/s, CPU of the named processes), then a summary of events.
"""
import argparse, os, selectors, socket, struct, sys, time, json

CLK = os.sysconf('SC_CLK_TCK')

ENC = {'raw': 0, 'zrle': 16}
PSEUDO_EXT_DESKTOP = -308
PSEUDO_DESKTOP = -223
PSEUDO_FENCE = -312
PSEUDO_CU = -313


def cpu_ticks(pid):
    try:
        with open('/proc/%d/stat' % pid) as f:
            s = f.read()
        rest = s[s.rindex(')') + 2:].split()
        return int(rest[11]) + int(rest[12])  # utime + stime
    except Exception:
        return None


class Probe:
    def __init__(self, a):
        self.a = a
        self.sock = socket.create_connection((a.host, a.port), timeout=10)
        self.sock.setsockopt(socket.IPPROTO_TCP, socket.TCP_NODELAY, 1)
        self.buf = b''
        self.total_bytes = 0
        self.w = self.h = 0
        self.screen_id = 0
        self.bpp = 32
        self.mode = 'none'
        self.paced_gap = 0.0
        self.outstanding = 0
        self.last_req = 0.0
        self.next_req_at = None
        self.events = []
        self.fence_sent = {}
        self.fence_seq = 0
        self.cur = None
        self.results = []
        self.handshake()

    # ---- connection set-up ----
    def readn(self, n):
        d = b''
        while len(d) < n:
            c = self.sock.recv(n - len(d))
            if not c:
                raise EOFError('server closed during handshake')
            d += c
        return d

    def handshake(self):
        ver = self.readn(12)
        self.server_version = ver.decode().strip()
        self.sock.sendall(b'RFB 003.008\n')
        n = self.readn(1)[0]
        types = list(self.readn(n))
        if 1 not in types:
            raise SystemExit('server does not offer security type None; offered %r' % types)
        self.sock.sendall(bytes([1]))
        res = struct.unpack('>I', self.readn(4))[0]
        if res != 0:
            raise SystemExit('security result %d' % res)
        self.sock.sendall(bytes([0]))  # ClientInit: not shared
        w, h = struct.unpack('>HH', self.readn(4))
        pf = self.readn(16)
        self.bpp = pf[0]
        nl = struct.unpack('>I', self.readn(4))[0]
        self.name = self.readn(nl).decode(errors='replace')
        self.w, self.h = w, h
        encs = [ENC[self.a.enc], 0 if self.a.enc != 'raw' else 16, PSEUDO_EXT_DESKTOP, PSEUDO_DESKTOP, PSEUDO_FENCE, PSEUDO_CU]
        msg = struct.pack('>BxH', 2, len(encs)) + b''.join(struct.pack('>i', e) for e in encs)
        self.sock.sendall(msg)
        self.sock.setblocking(False)

    # ---- client messages ----
    def send_request(self, incremental, rect=None):
        x, y, w, h = rect or (0, 0, self.w, self.h)
        self.sock.sendall(struct.pack('>BBHHHH', 3, 1 if incremental else 0, x, y, w, h))
        self.outstanding += 1
        self.last_req = time.monotonic()
        if self.cur:
            self.cur['requests'] += 1

    def send_cu(self, on, rect=(0, 0, 0, 0)):
        self.sock.sendall(struct.pack('>BBHHHH', 150, 1 if on else 0, *rect))

    def send_set_desktop_size(self, w, h):
        msg = struct.pack('>BxHHBx', 251, w, h, 1)
        msg += struct.pack('>IHHHHI', self.screen_id, 0, 0, w, h, 0)
        self.sock.sendall(msg)
        self.events.append((time.monotonic(), 'SetDesktopSize sent %dx%d (screen id %d)' % (w, h, self.screen_id)))

    def send_fence(self):
        self.fence_seq += 1
        payload = struct.pack('>I', self.fence_seq)
        # flags: bit 0 BlockBefore (default), 0 = no ordering at all. Request bit (31) must be set for a request.
        self.sock.sendall(struct.pack('>BxxxIB', 248, (1 << 31) | self.a.fence_flags, len(payload)) + payload)
        self.fence_sent[self.fence_seq] = time.monotonic()

    # ---- server message parser ----
    def parse(self):
        """Parse as many complete messages from self.buf as possible."""
        b = self.buf
        pos = 0
        while True:
            if len(b) - pos < 1:
                break
            t = b[pos]
            if t == 0:  # FramebufferUpdate
                if len(b) - pos < 4:
                    break
                nrects = struct.unpack_from('>H', b, pos + 2)[0]
                p = pos + 4
                ok = True
                rects = []
                for _ in range(nrects):
                    if len(b) - p < 12:
                        ok = False
                        break
                    x, y, w, h, enc = struct.unpack_from('>HHHHi', b, p)
                    p += 12
                    if enc == 0:
                        need = w * h * (self.bpp // 8)
                        if len(b) - p < need:
                            ok = False
                            break
                        p += need
                    elif enc == 16:
                        if len(b) - p < 4:
                            ok = False
                            break
                        ln = struct.unpack_from('>I', b, p)[0]
                        if len(b) - p - 4 < ln:
                            ok = False
                            break
                        p += 4 + ln
                    elif enc == PSEUDO_EXT_DESKTOP:
                        if len(b) - p < 4:
                            ok = False
                            break
                        ns = b[p]
                        if len(b) - p < 4 + 16 * ns:
                            ok = False
                            break
                        if ns:
                            self.screen_id = struct.unpack_from('>I', b, p + 4)[0]
                        p += 4 + 16 * ns
                    elif enc == PSEUDO_DESKTOP:
                        pass
                    else:
                        raise SystemExit('unexpected encoding %d in a rectangle' % enc)
                    rects.append((x, y, w, h, enc))
                if not ok:
                    break
                self.on_update(rects, p - pos)
                pos = p
            elif t == 2:  # Bell
                pos += 1
            elif t == 3:  # ServerCutText
                if len(b) - pos < 8:
                    break
                ln = struct.unpack_from('>i', b, pos + 4)[0]
                ln = abs(ln)
                if len(b) - pos < 8 + ln:
                    break
                pos += 8 + ln
            elif t == 150:  # EndOfContinuousUpdates
                pos += 1
                self.events.append((time.monotonic(), 'EndOfContinuousUpdates received'))
                if self.cur:
                    self.cur['eocu'] += 1
            elif t == 248:  # Fence
                if len(b) - pos < 9:
                    break
                flags = struct.unpack_from('>I', b, pos + 4)[0]
                ln = b[pos + 8]
                if len(b) - pos < 9 + ln:
                    break
                payload = b[pos + 9:pos + 9 + ln]
                pos += 9 + ln
                if flags & (1 << 31):  # a request from the server: answer it (flags without the request bit)
                    reply = flags & 7
                    self.sock.sendall(struct.pack('>BxxxIB', 248, reply, ln) + payload)
                    if self.cur:
                        self.cur['server_fences'] += 1
                else:
                    if ln == 4:
                        seq = struct.unpack('>I', payload)[0]
                        t0 = self.fence_sent.pop(seq, None)
                        if t0 is not None and self.cur is not None:
                            self.cur['fence_rtt_ms'].append(round((time.monotonic() - t0) * 1000, 1))
            else:
                raise SystemExit('unexpected server message type %d' % t)
        self.buf = b[pos:]

    def on_update(self, rects, nbytes):
        now = time.monotonic()
        self.outstanding = max(0, self.outstanding - 1)
        pix = [r for r in rects if r[4] in (0, 16)]
        eds = [r for r in rects if r[4] == PSEUDO_EXT_DESKTOP]
        for r in rects:
            if r[4] in (PSEUDO_EXT_DESKTOP, PSEUDO_DESKTOP):
                changed = (r[2], r[3]) != (self.w, self.h)
                if r[4] == PSEUDO_EXT_DESKTOP:
                    self.events.append((now, 'ExtendedDesktopSize rect: initiator=%d status=%d size=%dx%d' % (r[0], r[1], r[2], r[3])))
                if r[4] == PSEUDO_EXT_DESKTOP and r[1] != 0:
                    continue  # error result: size not changed
                if changed:
                    self.w, self.h = r[2], r[3]
                    if self.mode in ('normal', 'paced'):
                        # a viewer asks for a whole new picture after a size change
                        self.send_request(False)
                        self.outstanding = max(0, self.outstanding)
        c = self.cur
        if c is not None:
            if pix:
                c['updates'] += 1
                c['rects'] += len(pix)
                if c['first_pix'] is None:
                    c['first_pix'] = now - c['t0']
                    c['first_bytes'] = nbytes
                c['last_pix'] = now
            else:
                c['pseudo_only'] += 1
        # what to do next
        if self.mode == 'normal':
            self.send_request(True)
        elif self.mode == 'paced':
            self.next_req_at = max(now, self.last_req + self.paced_gap)

    # ---- phases ----
    def begin_phase(self, spec):
        name, _, rest = spec.partition(':')
        secs = float(rest)
        base, _, param = name.partition('@')
        now = time.monotonic()
        self.cur = dict(name=name, secs=secs, t0=now, bytes0=self.total_bytes, updates=0, rects=0, pseudo_only=0,
                        requests=0, eocu=0, server_fences=0, first_pix=None, last_pix=None, fence_rtt_ms=[],
                        cpu0={k: cpu_ticks(p) for k, p in self.a.pids.items()}, size0=(self.w, self.h))
        self.next_req_at = None
        self.phase_base = base
        if base == 'normal':
            self.mode = 'normal'
            if self.outstanding == 0:
                self.send_request(True)
        elif base == 'paced':
            self.mode = 'paced'
            self.paced_gap = float(param)
            if self.outstanding == 0:
                self.next_req_at = max(now, self.last_req + self.paced_gap)
        elif base == 'none':
            self.mode = 'none'
        elif base == 'cu':
            self.mode = 'cu'
            self.send_cu(True, (0, 0, self.w, self.h))
        elif base == 'cuoff':
            self.mode = 'none'
            self.send_cu(False)
        elif base == 'curegion':
            self.mode = 'cu'
            x, y, w, h = [int(v) for v in param.split('+')]
            self.send_cu(True, (x, y, w, h))
        elif base == 'resize':
            self.mode = 'normal'
            w, h = [int(v) for v in param.lower().split('x')]
            self.send_set_desktop_size(w, h)
            if self.outstanding == 0:
                self.send_request(True)
        elif base == 'fence':
            self.mode = 'none'
            self.next_fence = now
        else:
            raise SystemExit('unknown phase %r' % base)
        self.phase_end = now + secs

    def end_phase(self):
        c = self.cur
        now = time.monotonic()
        dur = now - c['t0']
        cpu = {}
        for k, p in self.a.pids.items():
            t1 = cpu_ticks(p)
            if t1 is not None and c['cpu0'][k] is not None:
                cpu[k] = round(100.0 * (t1 - c['cpu0'][k]) / CLK / dur, 1)
        c.update(dur=round(dur, 2), bytes=self.total_bytes - c['bytes0'], cpu=cpu, size1=(self.w, self.h))
        c['kBps'] = round(c['bytes'] / 1000.0 / dur, 1)
        c['ups'] = round(c['updates'] / dur, 1)
        c['first_kB'] = None if c.get('first_bytes') is None else round(c['first_bytes'] / 1000.0, 1)
        c['first_pix_s'] = None if c['first_pix'] is None else round(c['first_pix'], 3)
        self.results.append(c)
        self.cur = None

    def run(self):
        sel = selectors.DefaultSelector()
        sel.register(self.sock, selectors.EVENT_READ)
        specs = self.a.phases.split(',')
        # first, one full (non-incremental) request so we get a first picture, then settle for 1 s
        self.send_request(False)
        t_end = time.monotonic() + 1.0
        while time.monotonic() < t_end:
            self.pump(sel, 0.01)
        self.outstanding = 0 if self.outstanding < 0 else self.outstanding
        for spec in specs:
            self.begin_phase(spec)
            while time.monotonic() < self.phase_end:
                self.pump(sel, 0.005)
                now = time.monotonic()
                if self.mode == 'paced' and self.next_req_at is not None and now >= self.next_req_at:
                    self.next_req_at = None
                    self.send_request(True)
                if self.phase_base == 'fence' and now >= self.next_fence:
                    self.next_fence = now + 1.0
                    self.send_fence()
            self.end_phase()
        self.report()

    def pump(self, sel, timeout):
        for _ in sel.select(timeout):
            try:
                d = self.sock.recv(1 << 20)
            except BlockingIOError:
                continue
            if not d:
                raise SystemExit('server closed the connection')
            self.total_bytes += len(d)
            self.buf += d
            self.parse()

    def report(self):
        print('# server %s name=%r start size=%s encoding=%s bpp=%d' % (self.server_version, self.name, self.a.start_size, self.a.enc, self.bpp))
        hdr = '%-16s %6s %9s %8s %8s %8s %7s %7s  %-24s %s'
        print(hdr % ('phase', 'secs', 'kB/s', 'upd/s', 'updates', 'requests', 'first', '1st kB', 'cpu%', 'size'))
        for c in self.results:
            cpu = ' '.join('%s=%s' % kv for kv in c['cpu'].items())
            fp = '-' if c['first_pix_s'] is None else '%.3fs' % c['first_pix_s']
            sz = '%dx%d' % c['size1'] if c['size1'] == c['size0'] else '%dx%d->%dx%d' % (c['size0'] + c['size1'])
            print(hdr % (c['name'], c['dur'], c['kBps'], c['ups'], c['updates'], c['requests'], fp, '-' if c['first_kB'] is None else c['first_kB'], cpu, sz)
                  + ('  eocu=%d' % c['eocu'] if c['eocu'] else '')
                  + ('  fence_rtt_ms=%s' % c['fence_rtt_ms'] if c['name'].startswith('fence') else '')
                  + ('  pseudo_only=%d' % c['pseudo_only'] if c['pseudo_only'] else ''))
        for t, e in self.events:
            print('# event +%.2fs %s' % (t - self.results[0]['t0'], e))
        if self.a.json:
            with open(self.a.json, 'w') as f:
                json.dump(self.results, f, indent=1, default=str)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--host', default='127.0.0.1')
    ap.add_argument('--port', type=int, required=True)
    ap.add_argument('--phases', required=True)
    ap.add_argument('--enc', default='zrle', choices=sorted(ENC))
    ap.add_argument('--pid', action='append', default=[], help='NAME=PID, process whose CPU use is measured')
    ap.add_argument('--fence-flags', type=int, default=1, help='ordering bits of the Fence request: 1=BlockBefore, 0=none')
    ap.add_argument('--json')
    a = ap.parse_args()
    a.pids = {}
    for p in a.pids_raw if hasattr(a, 'pids_raw') else a.pid:
        k, _, v = p.partition('=')
        a.pids[k] = int(v)
    a.start_size = '?'
    pr = Probe(a)
    a.start_size = '%dx%d' % (pr.w, pr.h)
    pr.run()


if __name__ == '__main__':
    main()
