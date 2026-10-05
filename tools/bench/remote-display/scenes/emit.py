#!/usr/bin/env python3
"""Prints lines of a text file to the terminal it runs in, over and over.  emit.py FILE LINES_PER_SECOND   (0 = as fast as possible)"""
import sys, time

lines = open(sys.argv[1], encoding="utf-8").read().splitlines()
rate = float(sys.argv[2])
out = sys.stdout
i = 0
nxt = time.monotonic()
while True:
    if rate > 0:
        nxt += 1.0 / rate
        d = nxt - time.monotonic()
        if d > 0:
            time.sleep(d)
    out.write(lines[i % len(lines)] + "\n")
    out.flush()
    i += 1
