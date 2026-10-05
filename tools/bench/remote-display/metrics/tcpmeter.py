#!/usr/bin/env python3
"""Counting TCP relay on loopback (no tcpdump or ss in the container). Counts the bytes the way a network card would see
them (after TLS, if the link uses TLS), because it sits between the viewer and the server.

  tcpmeter.py LISTEN_PORT TARGET_PORT STATS_FILE

Every 0.2 s STATS_FILE is replaced with one line:
  <server_to_client_bytes> <client_to_server_bytes> <connections_accepted> <connections_open> <realtime_ns_of_last_accept> <realtime_ns_of_first_server_byte_of_last_connection>
The file STATS.c2s holds the first 4 KB the client sent on the latest connection.
Signals: SIGUSR1 closes every open connection but keeps listening (used to test reconnect behaviour of a viewer).
"""
import os, signal, socket, sys, threading, time

lp, tp, stats = int(sys.argv[1]), int(sys.argv[2]), sys.argv[3]
tot = [0, 0]
st = {"acc": 0, "open": 0, "last": 0, "first": 0}
conns = set()
lock = threading.Lock()


head = bytearray()  # first 4 KB the client sent on the latest connection (to read the encodings it asks for)


def pump(a, b, idx, mine):
    first = True
    try:
        while True:
            d = a.recv(262144)
            if not d:
                break
            with lock:
                tot[idx] += len(d)
                if idx == 1 and st["last"] == mine and len(head) < 4096:
                    head.extend(d[:4096 - len(head)])
                    with open(stats + ".c2s", "wb") as f:
                        f.write(head)
                if idx == 0 and first and st["last"] == mine:
                    st["first"] = time.time_ns()
                    first = False
            b.sendall(d)
    except OSError:
        pass
    for s in (a, b):
        try:
            s.shutdown(socket.SHUT_RDWR)
        except OSError:
            pass


def serve(c, t, mine):
    th = [threading.Thread(target=pump, args=(t, c, 0, mine), daemon=True), threading.Thread(target=pump, args=(c, t, 1, mine), daemon=True)]
    for x in th:
        x.start()
    for x in th:
        x.join()
    with lock:
        st["open"] -= 1
        conns.discard(c); conns.discard(t)
    c.close(); t.close()


def writer():
    while True:
        with lock:
            line = "%d %d %d %d %d %d\n" % (tot[0], tot[1], st["acc"], st["open"], st["last"], st["first"])
        with open(stats + ".tmp", "w") as f:
            f.write(line)
        os.replace(stats + ".tmp", stats)
        time.sleep(0.2)


def drop(*_):
    with lock:
        for s in list(conns):
            try:
                s.shutdown(socket.SHUT_RDWR)
            except OSError:
                pass


signal.signal(signal.SIGUSR1, drop)
threading.Thread(target=writer, daemon=True).start()
srv = socket.socket()
srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
srv.bind(("127.0.0.1", lp))
srv.listen(16)
while True:
    try:
        c, _ = srv.accept()
    except InterruptedError:
        continue
    try:
        t = socket.create_connection(("127.0.0.1", tp))
    except OSError:
        c.close()
        continue
    for s in (c, t):
        s.setsockopt(socket.IPPROTO_TCP, socket.TCP_NODELAY, 1)
    with lock:
        st["acc"] += 1; st["open"] += 1; st["last"] = time.time_ns(); st["first"] = 0
        del head[:]
        conns.add(c); conns.add(t)
        mine = st["last"]
    threading.Thread(target=serve, args=(c, t, mine), daemon=True).start()
