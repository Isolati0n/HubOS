p='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/hubsim/chaos.go'
s=open(p).read()
s=s.replace('''		say("UNRECOVERED ev=%d fault=%s target=%s why=%q", evn, fault, target, r.reason)''','''		say("UNRECOVERED ev=%d fault=%s target=%s alert=%d why=%q", evn, fault, target, b2i(exists("/run/hubos/alert")), r.reason)''')
open(p,'w').write(s)
