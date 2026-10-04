#!/usr/bin/env python3
"""Counting TCP relay for the remote-display experiments (loopback only).

usage: tcpmeter.py LISTEN_PORT TARGET_PORT STATS_FILE
Relays 127.0.0.1:LISTEN_PORT to 127.0.0.1:TARGET_PORT and, every second, writes
"server_to_client_bytes client_to_server_bytes" (totals, all connections) to STATS_FILE.
It does not change the data. It exists because no tcpdump or ss is in the build container.
"""
import os, socket, sys, threading, time

lp, tp, stats = int(sys.argv[1]), int(sys.argv[2]), sys.argv[3]
tot = [0, 0]  # [target->client, client->target]
lock = threading.Lock()


def pump(a, b, idx):
    try:
        while True:
            d = a.recv(65536)
            if not d:
                break
            with lock:
                tot[idx] += len(d)
            b.sendall(d)
    except OSError:
        pass
    for s in (a, b):
        try:
            s.shutdown(socket.SHUT_RDWR)
        except OSError:
            pass


def writer():
    while True:
        with open(stats + ".tmp", "w") as f:
            f.write("%d %d\n" % tuple(tot))
        os.replace(stats + ".tmp", stats)  # atomic, so a reader never sees half a file
        time.sleep(0.5)


threading.Thread(target=writer, daemon=True).start()
srv = socket.socket()
srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
srv.bind(("127.0.0.1", lp))
srv.listen(5)
while True:
    c, _ = srv.accept()
    try:
        t = socket.create_connection(("127.0.0.1", tp))
    except OSError:
        c.close()
        continue
    threading.Thread(target=pump, args=(t, c, 0), daemon=True).start()
    threading.Thread(target=pump, args=(c, t, 1), daemon=True).start()
