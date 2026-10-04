#!/usr/bin/env python3
"""peak.py TARGET [SECONDS] : peak sample (0..32767) heard on the monitor of sink TARGET in the current PipeWire instance."""
import struct, subprocess, sys, wave, os
t = float(sys.argv[2]) if len(sys.argv) > 2 else 1.0
subprocess.run(["timeout", str(t + 0.5), "pw-cat", "--record", "--rate", "48000", "--channels", "2", "--format", "s16", "-P", "{ stream.capture.sink=true }",
                "--target", sys.argv[1], "/tmp/p4-peak.wav"], stderr=subprocess.DEVNULL)
w = wave.open("/tmp/p4-peak.wav"); n = w.getnframes(); d = w.readframes(n)
print(max(abs(x) for x in struct.unpack("<%dh" % (n * 2), d)) if n else 0)
