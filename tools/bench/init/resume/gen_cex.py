import os, re
B = "/home/user/HubOS/.claude/worktrees/agent-a23944a7cde3c4db0/tools/bench/init/candidates/"

def w(cand, path, text, exe=True):
    p = os.path.join(B, cand, path)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    open(p, "w").write(text)
    if exe:
        os.chmod(p, 0o755)

c = "c-elixir"
w(c, "build.sh", """# architecture C with the Elixir/OTP brain: the s6rc candidate plus the brain as one extra s6 service. The Erlang runtime
# comes from the Ubuntu 24.04 erlang-base .deb unpacked with dpkg -x (OTP 25, not installed); distribution is off.
. "$HERE/candidates/s6rc/build.sh"
cp -a "$HERE/candidates/s6rc/rootfs/." "$R/"
E=$T/usr/lib/erlang; D=/usr/lib/erlang
for b in beam.smp erlexec erl_child_setup inet_gethost; do addbin "$E/erts-13.2.2.5/bin/$b" "$D/erts-13.2.2.5/bin/$b"; done
mkdir -p "$R$D/bin" "$R/opt/brain/ebin" "$R/usr/lib/elixir/lib/elixir"
cp "$E/bin/start_clean.boot" "$R$D/bin/"
for l in kernel-8.5.4.2 stdlib-4.3.1.3; do mkdir -p "$R$D/lib/$l"; cp -a "$E/lib/$l/ebin" "$R$D/lib/$l/"; done
cp -a "$T/usr/lib/elixir/lib/elixir/ebin" "$R/usr/lib/elixir/lib/elixir/"
cp "$W"/ebin/*.beam "$R/opt/brain/ebin/"
""", False)
src = open(B + "c-go/rootfs/cand/boot.sh").read()
src = src.replace("c-go", "c-elixir").replace("brain(go)", "brain(elixir)")
src = src.replace("printf '#!/bin/sh\\nexec 2>&1\\nexec brain -mode s6 -scan /run/service\\n' > /run/service/brain/run",
 "cp /opt/brain/run /run/service/brain/run")
w(c, "rootfs/cand/boot.sh", src, False)
w(c, "rootfs/opt/brain/run", """#!/bin/sh
exec 2>&1
export ROOTDIR=/usr/lib/erlang BINDIR=/usr/lib/erlang/erts-13.2.2.5/bin EMU=beam PROGNAME=erl HOME=/root
# one scheduler, no busy-waiting (the VM would otherwise burn CPU while idle); distribution off: no epmd, no cookie, no open port
exec /usr/lib/erlang/erts-13.2.2.5/bin/erlexec +S 1:1 +A 1 +sbwt none +sbwtdcpu none +sbwtdio none -noshell -noinput -boot start_clean -pa /opt/brain/ebin -pa /usr/lib/elixir/lib/elixir/ebin -eval '"Elixir.Brain":main().'
""")
w(c, "rootfs/ops/fix", """#!/bin/sh
hubsim ctl /run/hubos/brain.sock "retry $1" 2>/dev/null
s6-rc -l /run/s6-rc -u change top 2>/dev/null
s6-svc -u /run/service/$1 2>/dev/null
""")
w(c, "rootfs/ops/status", "#!/bin/sh\nhubsim ctl /run/hubos/brain.sock status\n")
w(c, "cmdline", "hub.cand=c-elixir hub.sup=s6-supervise hub.brain=beam.smp\n", False)

# use the shared control client in the Go candidates too
for cand in ("c-go", "b-go"):
    for f in ("rootfs/ops/fix", "rootfs/ops/status"):
        p = B + cand + "/" + f
        t = open(p).read().replace("brain ctl ", "hubsim ctl /run/hubos/brain.sock ")
        t = t.replace('hubsim ctl /run/hubos/brain.sock retry $1', 'hubsim ctl /run/hubos/brain.sock "retry $1"')
        open(p, "w").write(t)
