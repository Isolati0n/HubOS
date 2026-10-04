#!/usr/bin/env python3
"""Measurements for docs/proposals/language-comparison.md. Standard library only.

  measure.py start PORT N -- CMD...        start CMD N times; time until GET /v1/status answers 200; then idle memory
  measure.py killtime PORT N SERVICEDIR    SERVICEDIR is a service directory that a running s6-svscan watches.
                                           kill -9 the service's main process N times and time until /v1/status
                                           answers again (s6-svstat must be on PATH)

Every number is from the machine it was run on (a shared container in this project), not from the target hardware."""
import os, signal, socket, statistics, subprocess, sys, time


def get(port, path="/v1/status", timeout=1.0):
    try:
        s = socket.create_connection(("127.0.0.1", port), timeout=timeout)
        s.sendall(f"GET {path} HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n".encode())
        data = b""
        while True:
            c = s.recv(4096)
            if not c:
                break
            data += c
        s.close()
        head, _, body = data.partition(b"\r\n\r\n")
        return int(head.split()[1]), body.decode()
    except Exception:
        return None, ""


def wait_up(port, limit=30.0):
    t0 = time.perf_counter()
    while time.perf_counter() - t0 < limit:
        code, _ = get(port, timeout=0.2)
        if code == 200:
            return time.perf_counter() - t0
        time.sleep(0.002)
    return None


def children(pid):
    out, todo = [], [pid]
    while todo:
        p = todo.pop()
        out.append(p)
        for d in os.listdir("/proc"):
            if d.isdigit():
                try:
                    st = open(f"/proc/{d}/stat").read()
                    if int(st[st.rindex(")") + 2:].split()[1]) == p:
                        todo.append(int(d))
                except Exception:
                    pass
    return out


def comm(pid):
    try:
        return open(f"/proc/{pid}/comm").read().strip()
    except Exception:
        return "?"


def mem(pids):
    """RSS and PSS in KiB per process (PSS shares library pages fairly between processes)."""
    rows = []
    for p in pids:
        try:
            rss = pss = 0
            for line in open(f"/proc/{p}/smaps_rollup"):
                if line.startswith("Rss:"):
                    rss = int(line.split()[1])
                elif line.startswith("Pss:"):
                    pss = int(line.split()[1])
            rows.append((p, comm(p), rss, pss))
        except Exception:
            pass
    return rows


def summarise(name, xs, unit):
    print(f"{name}: min {min(xs):.1f} median {statistics.median(xs):.1f} max {max(xs):.1f} {unit} (n={len(xs)})")


def cmd_start(port, n, argv):
    times, rss, pss, procs = [], [], [], None
    for i in range(n):
        p = subprocess.Popen(argv, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, start_new_session=True)
        t = wait_up(port)
        if t is None:
            print("did not answer"); os.killpg(p.pid, signal.SIGKILL); sys.exit(1)
        times.append(t * 1000)
        for _ in range(3):
            get(port)
        time.sleep(4)  # idle
        pids = children(p.pid)
        # epmd detaches from its parent; add it (and only it) by name
        for d in os.listdir("/proc"):
            if d.isdigit() and comm(int(d)) == "epmd" and int(d) not in pids:
                pids.append(int(d))
        rows = mem(pids)
        rss.append(sum(r[2] for r in rows)); pss.append(sum(r[3] for r in rows))
        procs = rows
        os.killpg(p.pid, signal.SIGKILL)
        p.wait()
        subprocess.run(["pkill", "-x", "epmd"])
        time.sleep(0.3)
    summarise("time to first 200 answer", times, "ms")
    summarise("idle RSS, sum of the program's processes", rss, "KiB")
    summarise("idle PSS, sum of the program's processes", pss, "KiB")
    print("processes in the last run (pid, name, RSS KiB, PSS KiB):")
    for r in procs:
        print("  ", r)


def svpid(svc):
    out = subprocess.run(["s6-svstat", svc], capture_output=True, text=True).stdout
    if not out.startswith("up (pid "):
        return None
    return int(out.split("pid ")[1].split(")")[0])


def cmd_killtime(port, n, svc):
    times, left = [], []
    for i in range(n):
        while svpid(svc) is None or get(port, timeout=0.2)[0] != 200:
            time.sleep(0.05)
        time.sleep(2)  # s6-supervise waits so that a service is not started twice within one second; stay clear of that
        pid = svpid(svc)
        before = children(pid)
        t0 = time.perf_counter()
        os.kill(pid, signal.SIGKILL)
        t = wait_up(port, 60.0)
        times.append(t * 1000)
        time.sleep(0.5)
        left.append(sum(1 for q in before if q != pid and os.path.exists(f"/proc/{q}") and open(f"/proc/{q}/stat").read().split()[2] != "Z"))
        print(f"  run {i + 1}: new pid {svpid(svc)} (old {pid}), answered again after {times[-1]:.0f} ms, old child processes still alive: {left[-1]}")
    summarise("kill -9 to answering again", times, "ms")


def main():
    if sys.argv[1] == "killtime":
        cmd_killtime(int(sys.argv[2]), int(sys.argv[3]), sys.argv[4])
    elif sys.argv[1] == "start":
        i = sys.argv.index("--")
        cmd_start(int(sys.argv[2]), int(sys.argv[3]), sys.argv[i + 1:])
    else:
        sys.exit(__doc__)


main()
