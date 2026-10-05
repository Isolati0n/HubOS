p='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/candidates/runit/build.sh'
s=open(p).read()
if 'follow-driftwm' not in s:
    s+='cp "$HERE/../../../image/machines/hub/rootfs/usr/lib/hubos/follow-driftwm" "$R/bin/follow-driftwm"; chmod +x "$R/bin/follow-driftwm"\n'
open(p,'w').write(s)
