D='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/prototypes/'
p=D+'go/backend.go'
s=open(p).read()
s=s.replace('`^up \\(pid (\\d+)\\)`','`^up \\(pid (\\d+)`')
open(p,'w').write(s)
p=D+'elixir/brain.ex'
s=open(p).read()
s=s.replace('~r/^up \\(pid (\\d+)\\)/','~r/^up \\(pid (\\d+)/')
open(p,'w').write(s)
print([l for l in open(D+'go/backend.go') if 's6up' in l])
print([l for l in open(D+'elixir/brain.ex') if 'Regex.run' in l])
