p='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/analyze.py'
s=open(p).read()
s=s.replace('''        elif any(k in line for k in (" STORM ", " DEPSTARTS ", " OOMSCORE ", "ALERT-FILE")):''','''        elif " IDLE-CPU " in line:
            idle.append(line.split("CH ", 1)[-1])
        elif " MEM pid=" in line:
            d = kv(line)
            mem[d["comm"]] += int(d["pss_kb"])
        elif any(k in line for k in (" STORM ", " DEPSTARTS ", " OOMSCORE ", "ALERT-FILE")):''')
s=s.replace('''    alerts = 0
''','''    alerts = 0
    idle = []
    mem = collections.Counter()
''')
s=s.replace('''    print(f"  VM boots in log''','''    for l in idle[:2]:
        print("  " + l[:220])
    if mem:
        print("  PSS kB by program (PID 1, supervisors, policy layer): " + ", ".join(f"{k}={v}" for k, v in mem.most_common()) + f"  total={sum(mem.values())}")
    print(f"  VM boots in log''')
open(p,'w').write(s)
