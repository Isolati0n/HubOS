p='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/prototypes/elixir/brain.ex'
s=open(p).read()
hook='''  # TEST HOOK: a bug in the policy code of one service's watcher. OTP ends only this process and restarts it.
  def handle_cast(:boom, _s), do: raise("test hook: bug in the policy code")

'''
s=s.replace(hook,'')
s=s.replace('''  defp needs_ok?(s)''','''  @impl true
'''+hook.rstrip('\n').replace('  def handle_cast','  def handle_cast')+'''

  defp needs_ok?(s)''')
open(p,'w').write(s)
