p='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/prototypes/elixir/brain.ex'
s=open(p).read()
s=s.replace('''  def via(name), do: {:global, {:watcher, name}} |> then(fn _ -> String.to_atom("w_" <> name) end)''','''  def via(name), do: String.to_atom("w_" <> name)''')
s=s.replace('''    if up and pid != s.pid, do: :ok
''','')
open(p,'w').write(s)
import os
W='/tmp/claude-0/-home-user-HubOS/19ba1693-e641-5e2a-a3db-c8037607fada/scratchpad/ib'
os.makedirs(W+'/hostbin',exist_ok=True)
open(W+'/hostbin/erl','w').write('''#!/bin/sh
# host wrapper: the Ubuntu erl script has ROOTDIR=/usr/lib/erlang written in; this points it at the unpacked copy
E='''+W+'''/root/usr/lib/erlang
export ROOTDIR=$E BINDIR=$E/erts-13.2.2.5/bin EMU=beam PROGNAME=erl
exec $E/erts-13.2.2.5/bin/erlexec "$@"
''')
os.chmod(W+'/hostbin/erl',0o755)
open(W+'/elx.sh','w').write('''#!/bin/bash
. '''+W+'''/env.sh
export PATH=$W/hostbin:$T/usr/lib/elixir/bin:$PATH
export LD_LIBRARY_PATH=$T/usr/lib/x86_64-linux-gnu:$T/lib/x86_64-linux-gnu
export ELIXIR_ERL_OPTIONS="+S 1:1"
"$@"
''')
os.chmod(W+'/elx.sh',0o755)
