#!/usr/bin/env python3
"""Minimal reproducer: wl_shm_pool.resize(0) -> Smithay (rev 4cf0b62) handlers.rs:190 unwrap() on NonZeroUsize::try_from(0) -> compositor panics.
usage: repro_shm.py /path/to/wayland-socket"""
import socket, struct, os, sys, array, time
s = socket.socket(socket.AF_UNIX); s.connect(sys.argv[1])
def send(obj, op, payload, fds=()):
    m = struct.pack('<II', obj, ((8 + len(payload)) << 16) | op) + payload
    if fds: s.sendmsg([m], [(socket.SOL_SOCKET, socket.SCM_RIGHTS, array.array('i', fds))])
    else: s.send(m)
def enc(b): b = b + b'\0'; return struct.pack('<I', len(b)) + b + b'\0' * ((4 - len(b) % 4) % 4)
send(1, 1, struct.pack('<I', 2))                       # wl_display.get_registry -> id 2
time.sleep(0.3); data = s.recv(65536)
# parse globals to find wl_shm
off = 0; shm = None
while off + 8 <= len(data):
    o, w = struct.unpack('<II', data[off:off + 8]); size = w >> 16; op = w & 0xffff
    if o == 2 and op == 0:
        name, ln = struct.unpack('<II', data[off + 8:off + 16]); iface = data[off + 16:off + 16 + ln - 1].decode()
        ver = struct.unpack('<I', data[off + 16 + ((ln + 3) & ~3):off + 20 + ((ln + 3) & ~3)])[0]
        if iface == 'wl_shm': shm = (name, ver)
    off += size
send(2, 0, struct.pack('<I', shm[0]) + enc(b'wl_shm') + struct.pack('<II', 1, 3))      # bind wl_shm as id 3
fd = os.memfd_create('p'); os.ftruncate(fd, 4096)
send(3, 0, struct.pack('<Ii', 4, 4096), [fd])                                          # wl_shm.create_pool(id 4, fd, 4096)
send(4, 2, struct.pack('<i', 0))                                                       # wl_shm_pool.resize(0)
time.sleep(0.5)
print('sent wl_shm_pool.resize(0)')
