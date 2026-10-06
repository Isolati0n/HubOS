#!/usr/bin/env python3
"""The four reproductions of docs/proposals/hub-stability.md, run against one driftwm binary (nested, winit backend on Xvfb).

usage: repros.py CASE BIN LABEL
  CASE   shm      a client sends wl_shm_pool.resize(0)                      (Smithay panic, patch P6)
         config   a colour with a non-ASCII character in the config file    (config parser panic, patch P4):
                  --check-config on the file, and the file written into a RUNNING compositor (it reloads by itself)
         pipe     the log pipe nobody reads, so a write to it would block   (patch P7)
         pipepanic  the same, then a panic (needs a build with the test hook patch 0002; see run-all.sh)
         badshader  the built-in default shader does not compile (patch P9; needs a test build with a broken shader)
         startup  a start-up helper program that hangs; patched driftwm must not even run it (patch P8)
         reload   the config file is edited in the running compositor, and `reload-config` is sent (patch P12)
         session  the session file's temporary name is a FIFO nobody reads, a stand-in for a hung disk (T12); prints the P11 counters
         sessionfail  the temporary name is a directory: every background write fails, the counter of failed writes goes up (patch P11)
  BIN    the driftwm binary
  LABEL  printed in the verdict line (for example "unpatched" or "patched")
Prints one line per check and a final line  VERDICT <case> <label>: ...
Needs HS_WORK (same convention as docs/proposals/hub-stability/scripts: HS_WORK/A/root holds the unpacked runtime libraries,
foot and Xvfb; Xvfb must already run as :81, see run-all.sh). Runs everything inside /tmp; no core files (ulimit -c 0 is
set by run-all.sh and again here).
"""
import array, glob, json, os, resource, shutil, signal, socket, struct, subprocess, sys, time

resource.setrlimit(resource.RLIMIT_CORE, (0, 0))
case, BIN, LABEL = sys.argv[1], sys.argv[2], sys.argv[3]
S = os.environ['HS_WORK']
RT = '/tmp/dpr-' + case
shutil.rmtree(RT, ignore_errors=True)
os.makedirs(RT)
os.chmod(RT, 0o700)
L = S + '/A/root/usr/lib/x86_64-linux-gnu'
ENV = dict(os.environ)
ENV.update({'XDG_RUNTIME_DIR': RT, 'DISPLAY': ':81', 'LD_LIBRARY_PATH': L, 'LIBGL_ALWAYS_SOFTWARE': '1',
            'MESA_LOADER_DRIVER_OVERRIDE': 'swrast', '__EGL_VENDOR_LIBRARY_DIRS': S + '/A/root/usr/share/glvnd/egl_vendor.d',
            'LIBGL_DRIVERS_PATH': L + '/dri', 'GBM_BACKENDS_PATH': L + '/gbm', 'RUST_BACKTRACE': '1',
            'XDG_STATE_HOME': RT + '/state', 'XDG_DATA_HOME': RT + '/data',
            'PATH': S + '/A/root/usr/bin:' + os.environ.get('PATH', '')})
ENV.pop('WAYLAND_DISPLAY', None)
procs = []


def start(args, stdout, stderr, env=None):
    p = subprocess.Popen([BIN] + args, env=env or ENV, stdout=stdout, stderr=stderr)
    procs.append(p)
    return p


def ipc(req, timeout=3):
    s = socket.socket(socket.AF_UNIX)
    s.settimeout(timeout)
    s.connect(glob.glob(RT + '/driftwm/ipc-*.sock')[0])
    s.sendall((json.dumps(req) + '\n').encode())
    buf = b''
    while not buf.endswith(b'\n'):
        c = s.recv(1 << 20)
        if not c:
            break
        buf += c
    return buf


def wait_ipc(n=100):
    for _ in range(n):
        try:
            ipc('State')
            return True
        except Exception:
            time.sleep(0.1)
    return False


def answers(timeout=3):
    t = time.time()
    try:
        ipc('State', timeout)
        return True, (time.time() - t) * 1000
    except Exception:
        return False, (time.time() - t) * 1000


def foot(n):
    e = dict(ENV)
    e['WAYLAND_DISPLAY'] = 'wayland-1'
    p = subprocess.Popen(['foot', '--app-id=rp%d' % n, 'sleep', 'infinity'], env=e,
                         stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    procs.append(p)
    return p


def state(p):
    try:
        st = open('/proc/%d/status' % p.pid).read().split('State:')[1].split('\n')[0].strip()
        wch = open('/proc/%d/wchan' % p.pid).read()
        return st, wch
    except Exception:
        return '?', '?'


def write_counters():
    try:
        r = json.loads(ipc('State', 3).decode())
        st = r['Ok']['State']
        return {k: st[k] for k in ('failed_writes', 'pending_writes') if k in st}
    except Exception as e:
        return {'error': str(e)[:60]}


def finish(verdict):
    for p in procs:
        try:
            p.kill()
        except Exception:
            pass
    for p in procs:
        try:
            p.wait(timeout=5)
        except Exception:
            pass
    print('VERDICT %s %s: %s' % (case, LABEL, verdict))
    shutil.rmtree(RT, ignore_errors=True)


def log_lines(path, pats, n=2):
    out = []
    try:
        for line in open(path, errors='replace'):
            if any(p in line for p in pats):
                out.append(line.strip()[:160])
                if len(out) >= n:
                    break
    except Exception:
        pass
    return out


# ---------------------------------------------------------------------------------------------------------------------
if case == 'shm':
    log = RT + '/dw.log'
    lf = open(log, 'wb')
    dw = start(['--backend', 'winit', '--config', '/dev/null'], lf, lf)
    print('IPC up at start:', wait_ipc())
    r = subprocess.run([sys.executable, os.path.join(os.path.dirname(os.path.abspath(__file__)), 'repro_shm.py'),
                        RT + '/wayland-1'], capture_output=True, text=True, timeout=30)
    print('client:', r.stdout.strip() or r.stderr.strip()[:200])
    time.sleep(1.5)
    rc = dw.poll()
    ok, ms = answers()
    print('compositor exit status:', rc, '; IPC answers afterwards:', ok)
    for l in log_lines(log, ['panicked', 'TryFromInt']):
        print('log:', l)
    finish('compositor SURVIVED (still running, IPC answers)' if rc is None and ok else
           'compositor DIED (exit status %s)' % rc)

elif case == 'config':
    good = '[decorations]\nbg_color = "#aaaaaa"\n'
    bad = '[decorations]\nbg_color = "#aéaaa"\n'
    cfg = RT + '/c.toml'
    open(RT + '/bad.toml', 'w').write(bad)
    r = subprocess.run([BIN, '--check-config', '--config', RT + '/bad.toml'], env=ENV, capture_output=True, text=True, timeout=30)
    out = (r.stdout + r.stderr).strip().replace('\n', ' | ')[:200]
    print('--check-config on the bad file: exit status %s: %s' % (r.returncode, out))
    check_ok = r.returncode == 0
    open(cfg, 'w').write(good)
    log = RT + '/dw.log'
    lf = open(log, 'wb')
    dw = start(['--backend', 'winit', '--config', cfg], lf, lf)
    print('IPC up at start:', wait_ipc())
    tmp = cfg + '.new'
    open(tmp, 'w').write(bad)
    os.replace(tmp, cfg)          # an editor's atomic save: the running compositor reloads by itself
    time.sleep(3)
    rc = dw.poll()
    ok, ms = answers()
    print('running compositor after the bad file was written: exit status %s; IPC answers: %s' % (rc, ok))
    for l in log_lines(log, ['panicked', 'invalid bg_color', 'bg_color']):
        print('log:', l)
    finish('SURVIVED (check-config exit 0, running compositor alive)' if check_ok and rc is None and ok else
           'KILLED (check-config exit %s, running compositor exit %s)' % (r.returncode, rc))

elif case in ('pipe', 'pipepanic'):
    rfd, wfd = os.pipe()          # nobody reads rfd
    # fill the pipe first (64 KiB), so that every write by the compositor would block from its first log line on
    import fcntl
    fl = fcntl.fcntl(wfd, fcntl.F_GETFL)
    fcntl.fcntl(wfd, fcntl.F_SETFL, fl | os.O_NONBLOCK)
    try:
        while True:
            os.write(wfd, b'x' * 4096)
    except BlockingIOError:
        pass
    fcntl.fcntl(wfd, fcntl.F_SETFL, fl)          # blocking again (the child shares this open file description)
    env = dict(ENV)
    env['RUST_LOG'] = 'trace'
    dw = start(['--backend', 'winit', '--config', '/dev/null'], wfd, wfd, env)
    print('IPC up at start:', wait_ipc())
    for n in range(12):
        foot(n)
        time.sleep(0.3)
        try:
            ipc({'Move': {'window': None, 'to': [n * 40, n * 30]}}, 2)
        except Exception:
            pass
    time.sleep(2)
    good = 0
    for k in range(5):
        ok, ms = answers(3)
        good += ok
        print('IPC State round trip %d: %s (%.0f ms)' % (k, 'answered' if ok else 'NO ANSWER', ms))
        time.sleep(1)
    st, wch = state(dw)
    buf = array.array('i', [0])
    import fcntl, termios
    fcntl.ioctl(rfd, termios.FIONREAD, buf)
    print('process state: %s, wchan: %s, bytes waiting in the unread pipe: %d' % (st, wch.strip(), buf[0]))
    if case == 'pipe':
        finish('NOT frozen (%d of 5 IPC requests answered)' % good if good == 5 else
               'FROZEN or slow (%d of 5 IPC requests answered; wchan %s)' % (good, wch.strip()))
    else:
        # a panic while the log pipe is full: the compositor must exit (status 101), not hang in the panic message
        if good < 5:
            finish('already FROZEN by the full pipe (%d of 5 IPC requests answered), so no panic could be sent' % good)
            sys.exit(0)
        try:
            ipc({'Action': 'hs-test-panic'}, 3)
        except Exception:
            pass
        t = time.time()
        rc = None
        while time.time() - t < 8:
            rc = dw.poll()
            if rc is not None:
                break
            time.sleep(0.2)
        st, wch = state(dw)
        finish('panic ended the process (exit status %s after %.1f s)' % (rc, time.time() - t) if rc is not None else
               'HUNG in the panic message (state %s, wchan %s)' % (st, wch.strip()))

elif case == 'badshader':
    # patch P9: needs a test build whose built-in default shader (src/shaders/dot_grid.glsl) was replaced by garbage
    log = RT + '/dw.log'
    lf = open(log, 'wb')
    dw = start(['--backend', 'winit', '--config', '/dev/null'], lf, lf)
    print('IPC up at start:', wait_ipc())
    foot(0)
    time.sleep(4)
    rc = dw.poll()
    ok, ms = answers()
    print('compositor exit status:', rc, '; IPC answers:', ok)
    for l in log_lines(log, ['panicked', 'Default shader', 'Default background shader', 'flat background']):
        print('log:', l)
    finish('SURVIVED with a flat background (still running, IPC answers)' if rc is None and ok else
           'DIED (exit status %s)' % rc)

elif case == 'startup':
    # patch P8: the start-up helper calls (a fake dbus-update-activation-environment that records that it ran and sleeps 60 s)
    fake = RT + '/fakebin'
    os.makedirs(fake)
    ran = RT + '/helper-was-run'
    open(fake + '/dbus-update-activation-environment', 'w').write('#!/bin/sh\ntouch %s\nexec sleep 60\n' % ran)
    os.chmod(fake + '/dbus-update-activation-environment', 0o755)
    env = dict(ENV)
    env['PATH'] = fake + ':' + ENV['PATH']
    log = RT + '/dw.log'
    lf = open(log, 'wb')
    t0 = time.time()
    dw = start(['--backend', 'winit', '--config', '/dev/null'], lf, lf, env)
    up = False
    while time.time() - t0 < 20 and not up:
        up = wait_ipc(5)
    took = time.time() - t0
    print('IPC answered: %s after %.1f s (the helper sleeps 60 s)' % (up, took))
    time.sleep(1)
    tried = os.path.exists(ran)
    print('the helper program was started by driftwm:', tried)
    finish('started in %.1f s and did not even try the helper (patch P8 removed the call)' % took if up and not tried else
           ('started in %.1f s but tried the helper' % took if up else
            'BLOCKED: no IPC after 20 s (the main thread waits for the helper, which was started: %s)' % tried))

elif case == 'reload':
    cfg = RT + '/c.toml'
    open(cfg, 'w').write('[navigation]\ndrift = 0.5\n')
    log = RT + '/dw.log'
    lf = open(log, 'wb')
    dw = start(['--backend', 'winit', '--config', cfg], lf, lf)
    print('IPC up at start:', wait_ipc())
    tmp = cfg + '.new'
    open(tmp, 'w').write('[navigation]\ndrift = 0.75\n')
    os.replace(tmp, cfg)          # an editor's atomic save
    time.sleep(3)
    reloaded = len(log_lines(log, ['Config reloaded'], 5))
    print('"Config reloaded" lines in the log after the file was edited:', reloaded)
    try:
        r = ipc({'Action': 'reload-config'}, 3).decode().strip()[:160]
    except Exception as e:
        r = 'no answer (%s)' % e
    print('IPC action reload-config answer:', r)
    reloaded2 = len(log_lines(log, ['Config reloaded'], 5))
    ok, ms = answers()
    refused = '"Err"' in r or 'Err' in r
    finish('NO hot reload: the edit changed nothing, reload-config refused (%s), compositor alive' % r[:60] if reloaded == 0 and reloaded2 == 0 and refused and dw.poll() is None and ok else
           'HOT RELOAD works: edit reloaded %d time(s), reload-config answer: %s' % (reloaded, r[:60]))

elif case == 'session':
    D = RT + '/state'
    os.makedirs(D)
    open(D + '/c.toml', 'w').write('[session]\nrestore_windows = true\n')
    os.mkfifo(D + '/session.json.tmp')
    log = RT + '/dw.log'
    lf = open(log, 'wb')
    dw = start(['--backend', 'winit', '--config', D + '/c.toml', '--session-file', D + '/session.json'], lf, lf)
    print('IPC up at start:', wait_ipc())
    foot(0)
    time.sleep(3)
    good = 0
    for k in range(4):
        ok, ms = answers(2)
        good += ok
        print('IPC State %d: %s' % (k, 'answered (%.0f ms)' % ms if ok else 'no answer within 2 s'))
        time.sleep(1)
    st, wch = state(dw)
    print('process state: %s, wchan: %s' % (st, wch.strip()))
    cnt = write_counters()
    print('IPC state counters (patch P11): %s' % cnt)
    finish('NOT frozen (%d of 4 answered); %s' % (good, cnt) if good == 4 else
           'FROZEN (%d of 4 answered; wchan %s)' % (good, wch.strip()))

elif case == 'sessionfail':
    # patch P11: the session file's temporary name is a DIRECTORY, so every background write fails at once
    D = RT + '/state'
    os.makedirs(D + '/session.json.tmp')
    open(D + '/c.toml', 'w').write('[session]\nrestore_windows = true\n')
    log = RT + '/dw.log'
    lf = open(log, 'wb')
    dw = start(['--backend', 'winit', '--config', D + '/c.toml', '--session-file', D + '/session.json'], lf, lf)
    print('IPC up at start:', wait_ipc())
    foot(0)
    time.sleep(3)
    ok, ms = answers(2)
    cnt = write_counters()
    print('IPC answers: %s; IPC state counters (patch P11): %s' % (ok, cnt))
    for l in log_lines(log, ['failed to write']):
        print('log:', l)
    finish('IPC keeps answering and the failed writes are counted: %s' % cnt if ok and 'failed_writes' in cnt and cnt['failed_writes'] > 0 else
           'no counter or no answer: answers=%s %s' % (ok, cnt))
else:
    print('unknown case')
    sys.exit(2)
