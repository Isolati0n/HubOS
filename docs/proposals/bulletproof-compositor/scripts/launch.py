#!/usr/bin/env python3
"""launch.py LABEL ENVKV... -- command...   : starts the command detached (own session), nice 19, core files off, output in LABEL.out; prints its pid."""
import os, resource, subprocess, sys
label = sys.argv[1]
i = sys.argv.index('--')
envkv = sys.argv[2:i]; cmd = sys.argv[i + 1:]
W = os.environ['W']
env = dict(os.environ)
for kv in envkv:
    k, v = kv.split('=', 1); env[k] = v
def pre():
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))
    os.nice(19)
out = open(f'{W}/{label}.out', 'ab')
p = subprocess.Popen(cmd, env=env, stdout=out, stderr=subprocess.STDOUT, stdin=subprocess.DEVNULL, start_new_session=True, preexec_fn=pre, cwd=W)
open(f'{W}/{label}.pid', 'w').write(str(p.pid))
print('started', label, 'pid', p.pid)
