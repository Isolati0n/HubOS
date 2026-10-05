import os,re
B='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/candidates/openrc/rootfs/etc/init.d/'
for f in os.listdir(B):
    p=B+f
    s=open(p).read()
    s=re.sub(r'output_logger="s6-log -b n5 s100000 /var/log/(\w+)"\nerror_logger="[^"]*"\n', lambda m: 'output_logger="/opt/s6/bin/s6-log -b n5 s100000 /var/log/%s"\nerror_log=/dev/null\nstart_pre() { mkdir -p /var/log/%s; }\n'%(m.group(1),m.group(1)), s)
    open(p,'w').write(s)
print(open(B+'driftwm').read())
