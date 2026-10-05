p='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/hubsim/chaos.go'
s=open(p).read()
s=s.replace('say("SAMPLE zombies=%d nproc=%d init_side_fds=%d init_side_rss_kb=%d memavail_kb=%d", z, n, fd, rss, avail)','say("SAMPLE zombies=%d nproc=%d init_side_fds=%d init_side_rss_kb=%d memavail_kb=%d init_side_cpu_ticks=%d", z, n, fd, rss, avail, cpuTicks(g))')
open(p,'w').write(s)
p='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/run-qemu.py'
s=open(p).read()
s=s.replace('''            if b"SUITE-DONE" in line:
                done = True
''','''            if b"SUITE-DONE" in line:
                done = True
            if b"BOOT-OK" in line:
                boot_ok += 1
                if a.suite in ("pid1crash", "pid1wedge") and boot_ok >= 2:
                    # the machine came back by itself after the injected PID 1 fault: that is the measurement
                    f.write(b"H# second BOOT-OK seen: the machine recovered by itself\\n")
                    done = True
                    break
''')
s=s.replace('        stuck_step = None\n','        stuck_step = None\n        boot_ok = 0\n',1)
open(p,'w').write(s)
p='/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/analyze.py'
s=open(p).read()
s=s.replace('''if k in ("zombies", "nproc", "init_side_fds", "init_side_rss_kb", "memavail_kb")})''','''if k in ("zombies", "nproc", "init_side_fds", "init_side_rss_kb", "memavail_kb", "init_side_cpu_ticks")})''')
open(p,'w').write(s)
