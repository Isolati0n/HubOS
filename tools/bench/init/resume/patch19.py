B='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/'
p=B+'run-qemu.py'
s=open(p).read()
s=s.replace('''            r, _, _ = select.select([p.stdout], [], [], 5)
            if not r:
                continue''','''            r, _, _ = select.select([p.stdout], [], [], 5)
            if a.suite == "trial" and trial_seen and time.monotonic() - trial_seen > 20:
                f.write(b"H# trial finished\\n"); done = True; break
            if not r:
                continue''')
open(p,'w').write(s)
p=B+'candidates/s6rc/build.sh'
s=open(p).read()
import re
s=re.sub(r'for b in s6-fdholder-daemon .*?; do \[ -x "\$W/skel/bin/\$b" \] && cp "\$W/skel/bin/\$b" "\$R/opt/s6/bin/"; done','cp "$W"/skel/bin/* "$R/opt/s6/bin/"   # all of the suite: s6-rc and s6-svc -w need several helper programs',s,flags=re.S)
open(p,'w').write(s)
print(s)
