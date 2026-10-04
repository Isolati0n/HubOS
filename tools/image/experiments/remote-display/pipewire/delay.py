#!/usr/bin/env python3
"""Measure the delay of PipeWire RTP between two instances on loopback.

usage: delay.py N        (instances /tmp/p4-a and /tmp/p4-b must run: start.sh)
A burst (1 kHz tone, 50 ms) is played to node-null in instance a. The same sound is seen twice:
 tap1 = monitor of node-null in a (before the network), tap2 = monitor of hub-out in b (after RTP).
Both are read with pw-cat --record from raw stdout, and the time when each tap first shows the burst
(time.monotonic at the read) is taken. delay = tap2 - tap1. Both taps use the same pw-cat settings,
so their own capture delay is about the same and mostly cancels. Printed in milliseconds.
"""
import math, struct, subprocess, sys, threading, time, os

RATE, CH = 48000, 2
N = int(sys.argv[1]) if len(sys.argv) > 1 else 10


def start_rec(rt, target):
    env = dict(os.environ, XDG_RUNTIME_DIR=rt, PIPEWIRE_RUNTIME_DIR=rt)
    return subprocess.Popen(["pw-cat", "--record", "--rate", str(RATE), "--channels", str(CH), "--format", "s16",
                             "--latency", "128", "-P", "{ stream.capture.sink=true }", "--target", target, "-"],
                            env=env, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, bufsize=0)


def watch(p, store):
    while True:
        d = p.stdout.read(4096)
        if not d:
            return
        now = time.monotonic()
        n = len(d) // 2
        peak = max(abs(x) for x in struct.unpack("<%dh" % n, d[: n * 2]))
        if peak > 3000 and store.get("armed") and "t" not in store:
            store["t"] = now


def burst_wav(path):
    frames = b"".join(struct.pack("<hh", s, s) for s in [int(12000 * math.sin(2 * math.pi * 1000 * i / RATE)) for i in range(RATE // 20)])
    import wave
    w = wave.open(path, "wb"); w.setnchannels(2); w.setsampwidth(2); w.setframerate(RATE); w.writeframes(frames); w.close()


burst_wav("/tmp/p4-burst.wav")
t1, t2 = {}, {}
r1 = start_rec("/tmp/p4-a", "node-null")
r2 = start_rec("/tmp/p4-b", "hub-out")
threading.Thread(target=watch, args=(r1, t1), daemon=True).start()
threading.Thread(target=watch, args=(r2, t2), daemon=True).start()
time.sleep(2)
res = []
since = []  # (tap1, tap2) milliseconds after the player process was started
for i in range(N):
    t1.clear(); t2.clear(); t1["armed"] = t2["armed"] = True
    tp = time.monotonic()
    subprocess.run(["pw-cat", "--playback", "--target", "node-null", "/tmp/p4-burst.wav"],
                   env=dict(os.environ, XDG_RUNTIME_DIR="/tmp/p4-a", PIPEWIRE_RUNTIME_DIR="/tmp/p4-a"), stderr=subprocess.DEVNULL)
    time.sleep(1.5)
    if "t" in t1 and "t" in t2:
        res.append((t2["t"] - t1["t"]) * 1000)
        since.append(((t1["t"] - tp) * 1000, (t2["t"] - tp) * 1000))
    else:
        print("run %d: burst not seen (tap1 %s, tap2 %s)" % (i, "t" in t1, "t" in t2))
    t1["armed"] = t2["armed"] = False
    time.sleep(0.5)
r1.kill(); r2.kill()
if since:
    print("after player start (ms): tap1 median %.0f, tap2 median %.0f" % (sorted(x[0] for x in since)[len(since)//2], sorted(x[1] for x in since)[len(since)//2]))
if res:
    res.sort()
    print("n=%d  delay ms: min %.0f  median %.0f  max %.0f   all: %s" % (len(res), res[0], res[len(res)//2], res[-1], " ".join("%.0f" % x for x in res)))
