#!/usr/bin/env python3
"""CPU and memory of named process groups, from /proc (no extra tools in the container).

  procstat.py snap  FILE  label=pid[,pid] ...   write a snapshot (time, CPU ticks, memory) of each group and all its children
  procstat.py diff  FILE1 FILE2                  print  label cpu_percent_of_one_core rss_mb hwm_mb procs pss_mb   per group

CPU percent is 100 for one fully busy core (so 4 busy cores = 400). rss_mb and hwm_mb are taken from the second snapshot
(current and peak resident memory, summed over the group; pss_mb = proportional set size, which counts a shared library once for all the processes that use it, so it is the right sum for many processes). A group whose processes are gone is reported as gone.
"""
import json, os, sys, time

CLK = os.sysconf("SC_CLK_TCK")


def children_map():
    kids = {}
    for d in os.listdir("/proc"):
        if d.isdigit():
            try:
                st = open("/proc/%s/stat" % d).read()
                ppid = int(st[st.rindex(")") + 2:].split()[1])
                kids.setdefault(ppid, []).append(int(d))
            except (OSError, ValueError):
                pass
    return kids


def tree(pid, kids):
    out, todo = [], [pid]
    while todo:
        p = todo.pop()
        out.append(p)
        todo += kids.get(p, [])
    return out


def one(pid):
    try:
        st = open("/proc/%d/stat" % pid).read()
        f = st[st.rindex(")") + 2:].split()
        ticks = int(f[11]) + int(f[12])  # utime + stime (fields 14 and 15)
        rss = hwm = 0
        pss = 0
        try:
            for line in open("/proc/%d/smaps_rollup" % pid):
                if line.startswith("Pss:"):
                    pss = int(line.split()[1])
        except OSError:
            pass
        for line in open("/proc/%d/status" % pid):
            if line.startswith("VmRSS:"):
                rss = int(line.split()[1])
            elif line.startswith("VmHWM:"):
                hwm = int(line.split()[1])
        return ticks, rss, hwm, pss
    except (OSError, ValueError, IndexError):
        return None


def snap(path, groups):
    kids = children_map()
    res = {"t": time.time(), "groups": {}}
    seen = set()  # a process counts in the first group that claims it (list the app group before the compositor that started it)
    for g in groups:
        label, pids = g.split("=", 1)
        ticks = rss = hwm = pss = n = 0
        for pid in pids.split(","):
            if not pid.isdigit():
                continue
            for p in tree(int(pid), kids):
                if p in seen:
                    continue
                seen.add(p)
                r = one(p)
                if r:
                    ticks += r[0]; rss += r[1]; hwm += r[2]; pss += r[3]; n += 1
        res["groups"][label] = {"ticks": ticks, "rss_kb": rss, "hwm_kb": hwm, "pss_kb": pss, "n": n}
    json.dump(res, open(path, "w"))


def diff(p1, p2):
    a, b = json.load(open(p1)), json.load(open(p2))
    dt = b["t"] - a["t"]
    for label, g in b["groups"].items():
        h = a["groups"].get(label)
        if g["n"] == 0 or h is None:
            print(label, "gone", "0", "0", "0", "0", "0"); continue
        cpu = (g["ticks"] - h["ticks"]) / CLK / dt * 100
        print(label, "%.1f" % cpu, "%.1f" % (g["rss_kb"] / 1024), "%.1f" % (g["hwm_kb"] / 1024), g["n"], "%.1f" % (g.get("pss_kb", 0) / 1024))


if __name__ == "__main__":
    if sys.argv[1] == "snap":
        snap(sys.argv[2], sys.argv[3:])
    else:
        diff(sys.argv[2], sys.argv[3])
