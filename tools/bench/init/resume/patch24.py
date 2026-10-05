p='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/candidates/c-elixir/rootfs/opt/brain/run'
s=open(p).read()
s=s.replace('''-eval '"Elixir.Brain":main().\'''','''-eval "'Elixir.Brain':main()."''')
s=s.replace('export ROOTDIR=','export ERL_CRASH_DUMP_SECONDS=0 ROOTDIR=')
open(p,'w').write(s)
print(s)
