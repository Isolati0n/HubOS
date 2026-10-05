#!/usr/bin/env python3
"""Small analysis steps over pixelwatch logs. Standard library only.

  analyze.py ttff   LOG T0_NS EXPECT TOL            ms from T0 to the first frame whose pixel is within TOL of EXPECT ("none" if never)
  analyze.py input  LOG CLICKS APPLOG                per click: hub click -> node got the event -> hub pixel changed; prints key=value lines
  analyze.py bars   LOG T_START_NS T_END_NS          decode the 16-point frame-number bar code; prints frames seen, span, dropped
  analyze.py first  LOG T0_NS EXPECT TOL             like ttff but prints the absolute time (ns)
pixelwatch log line:  <realtime ns> <value 1> <value 2> ...
"""
import statistics, sys


def lines(path):
    out = []
    try:
        for ln in open(path):
            p = ln.split()
            if len(p) >= 2 and p[0].isdigit():
                out.append((int(p[0]), [int(x) for x in p[1:]]))
    except OSError:
        pass
    return out


def ttff(log, t0, expect, tol):
    for t, v in lines(log):
        if t >= t0 and abs(v[0] - expect) <= tol:
            return t
    return None


def inputs(log, clicks, applog):
    L = lines(log)
    cl = [int(x) for x in open(clicks).read().split()]
    ak = []
    for ln in open(applog):
        p = ln.split()
        if len(p) == 2 and p[1] == "key" and p[0].isdigit():
            ak.append(int(p[0]))
    res = []
    for i, t0 in enumerate(cl):
        nxt = cl[i + 1] if i + 1 < len(cl) else t0 + 3_000_000_000
        before = [v[0] for t, v in L if t < t0][-1:] or [None]
        if before[0] is None:
            continue
        b = before[0]
        t2 = next((t for t, v in L if t0 <= t < nxt and abs(v[0] - b) > 100), None)
        t1 = next((t for t in ak if t0 <= t < nxt), None)
        if t2 is None or t1 is None:
            continue
        res.append(((t1 - t0) / 1e6, (t2 - t1) / 1e6, (t2 - t0) / 1e6))
    return res, len(cl)


def bars(log, ts, te):
    seen = []
    for t, v in lines(log):
        if ts <= t <= te and len(v) >= 16:
            n = 0
            for k in range(16):
                if v[k] >= 128:
                    n |= 1 << k
            seen.append((t, n))
    if not seen:
        return None
    nums = [n for _, n in seen]
    uniq = sorted(set(nums))
    # frame numbers only go up inside the window (the clip is longer than the window); a value out of order is a decode error
    good = [n for n in uniq if uniq[0] <= n <= uniq[0] + 30 * 60]
    span = good[-1] - good[0] + 1
    return {"hub_repaints": len(seen), "distinct_frames_seen": len(good), "span_frames": span,
            "dropped_frames": span - len(good), "dropped_pct": round(100 * (span - len(good)) / span, 1),
            "seconds": round((seen[-1][0] - seen[0][0]) / 1e9, 2),
            "shown_fps": round(len(good) / max((seen[-1][0] - seen[0][0]) / 1e9, 1e-9), 1)}


NAMES = {0: "Raw", 1: "CopyRect", 2: "RRE", 4: "CoRRE", 5: "Hextile", 6: "Zlib", 7: "Tight", 8: "ZlibHex", 15: "TRLE", 16: "ZRLE",
         21: "JPEG", 50: "OpenH264", -223: "DesktopSize", -239: "Cursor", -308: "ExtendedDesktopSize", -307: "DesktopName",
         -224: "LastRect", -247: "QualityLevel0", -258: "SubsampLevel0", -314: "ContinuousUpdates", -312: "Fence", -309: "ExtendedClipboard",
         -313: "ExtendedMouseButtons", -316: "Xvp", -240: "XCursor", -305: "GII", -306: "Pointer-ish"}


def encodings(path):
    """Read the encodings list a VNC client sent (plain RFB 3.8, security type None only)."""
    try:
        d = open(path, "rb").read()
    except OSError:
        return None
    import struct
    if not d.startswith(b"RFB 003."):
        return None              # TLS from the first byte (Weston's VNC backend): cannot be read on the relay
    i = 12                       # protocol version string
    if len(d) <= i:
        return None
    if len(d) <= i or d[i] != 1:
        return None              # a security type other than None (Weston: VeNCrypt, then TLS): the rest is encrypted on the relay
    i += 1                       # the client's chosen security type (None: nothing more is sent)
    i += 1                       # ClientInit (shared flag)
    while i < len(d):
        t = d[i]
        if t == 0:
            i += 20
        elif t == 2:
            n = struct.unpack(">H", d[i + 2:i + 4])[0]
            vals = struct.unpack(">%di" % n, d[i + 4:i + 4 + 4 * n]) if len(d) >= i + 4 + 4 * n else ()
            out = []
            for v in vals:
                if -32 <= v <= -23: nm = "QualityLevel%d" % (v + 32)
                elif -256 <= v <= -247: nm = "CompressLevel%d" % (v + 256)
                else: nm = NAMES.get(v, str(v))
                out.append(nm)
            return out
        else:
            return None
    return None


def stat3(xs):
    return "%.1f %.1f %.1f" % (min(xs), statistics.median(xs), max(xs))


if __name__ == "__main__":
    c = sys.argv[1]
    if c in ("ttff", "first"):
        t = ttff(sys.argv[2], int(sys.argv[3]), int(sys.argv[4]), int(sys.argv[5]))
        print("none" if t is None else (t - int(sys.argv[3])) // 1000000 if c == "ttff" else t)
    elif c == "input":
        r, n = inputs(sys.argv[2], sys.argv[3], sys.argv[4])
        print("input_clicks=%d" % n)
        print("input_matched=%d" % len(r))
        if r:
            print("input_to_node_ms_min_med_max=%s" % stat3([x[0] for x in r]))
            print("node_to_hub_pixel_ms_min_med_max=%s" % stat3([x[1] for x in r]))
            print("input_to_pixel_ms_min_med_max=%s" % stat3([x[2] for x in r]))
    elif c == "encodings":
        e = encodings(sys.argv[2])
        print("client_encodings=" + (",".join(e) if e else "unreadable (TLS or not plain RFB)"))
    elif c == "bars":
        r = bars(sys.argv[2], int(sys.argv[3]), int(sys.argv[4]))
        if r is None:
            print("bars_error=no_frames")
        else:
            for k, v in r.items():
                print("video_%s=%s" % (k, v))
