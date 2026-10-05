p='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/prototypes/elixir/brain.ex'
s=open(p).read()
s=s.replace('''    :ets.insert(:brain, {spec.name, false, false, 0, 0})''','''    # insert_new: a restarted watcher must not wipe what the others already know about this service
    :ets.insert_new(:brain, {spec.name, false, false, 0, 0})''')
s=s.replace('''    s = if up and pid != s.pid, do: %{s | pid: pid, started: now}, else: s
''','''    s =
      if up and pid != s.pid do
        s = if s.pid != 0 and s.last_up and not s.held, do: fail(s, now, "restarted unseen"), else: s
        %{s | pid: pid, started: now}
      else
        s
      end

''')
open(p,'w').write(s)
