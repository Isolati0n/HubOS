import os
B='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/'
p=B+'run-qemu.py'
s=open(p).read()
s=s.replace('''        r, _, _ = select.select([p.stdout], [], [], 5)
        if not r:
            continue''','''        r, _, _ = select.select([p.stdout], [], [], 5)
        if a.suite == "trial" and trial_seen and time.monotonic() - trial_seen > 20:
            f.write(b"H# trial finished\\n"); done = True; break
        if not r:
            continue''')
open(p,'w').write(s)

def w(c,txt):
    d=B+'candidates/'+c+'/rootfs/ops/'
    os.makedirs(d,exist_ok=True)
    open(d+'healthy','w').write(txt)
    os.chmod(d+'healthy',0o755)
svc='seatd udevd dbus driftwm waybar hubd'
w('s6plain',f"#!/bin/sh\nfor s in {svc}; do s6-svstat /run/service/$s | grep -q '^up' || exit 1; done\n")
w('s6rc',f"#!/bin/sh\nfor s in {svc}; do s6-svstat /run/service/$s | grep -q '^up.*, ready ' || exit 1; done\n")
w('runit',f"#!/bin/sh\nfor s in {svc}; do sv check /run/service/$s >/dev/null || exit 1; done\n")
w('dinit',f"#!/bin/sh\nfor s in {svc}; do /opt/dinit/bin/dinitctl is-started $s >/dev/null 2>&1 || exit 1; done\n")
w('openrc',f"#!/bin/sh\nfor s in {svc}; do rc-service $s status >/dev/null 2>&1 || exit 1; done\n")
brain="""#!/bin/sh
o=$(hubsim ctl /run/hubos/brain.sock status) || exit 1
[ "$(echo "$o" | wc -l)" -eq 6 ] || exit 1
echo "$o" | grep -v 'state=running up=true' >/dev/null && exit 1
exit 0
"""
for c in ('c-go','c-elixir','b-go'):
    w(c,brain)
