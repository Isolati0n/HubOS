# fdpass: passing a file descriptor from Go to Erlang (TESTED, 2026-10-11)

**What it shows.** A Go program can hand an open descriptor to an Erlang process over a Unix `SOCK_SEQPACKET` socket, using an `SCM_RIGHTS` control message, and the Erlang side can read it with the OTP `socket` module (control message type `rights`) and use it.

**How the test works** (`fdpass_test.go`, receiver in `receiver.escript`):
1. Erlang binds and listens on a Unix seqpacket socket, prints `READY`, accepts, and calls `socket:recvmsg/5`.
2. Go makes a `socketpair`, sends one end with `WriteMsgUnix(..., syscall.UnixRights(fd))`, then closes its own copy of that end.
3. Erlang takes the integer from the `rights` control message, turns it into a socket with `socket:open(Fd, ...)`, writes one line, and closes it.
4. Go reads the line from its end of the pair, then sees end-of-file (proof that the only other copy lived in the Erlang process and was closed there).

**Result:** passes (`go test ./tools/fdpass/`, 0.17 s). Versions: Erlang/OTP 25 (erts 13.2.2.5), Go 1.24.7 as in `go.mod`. Erlang came from the Ubuntu 24.04 packages unpacked with `dpkg -x` into a scratch folder (nothing was installed on the machine). I ran it through `erl -noshell -run escript start -extra ...` because the unpacked `escript` launcher has a hard-coded `/usr/lib/erlang` path; the test uses `escript` from `PATH` or the program named in `ESCRIPT` and is skipped when none exists.

**Not shown / UNKNOWN:** only one descriptor per message was sent; no test of several descriptors, of a closed or broken receiver, of a very slow receiver, or of other OTP versions. In OTP 25 `socket:recvmsg/5` takes flags as a list (`[]`), not an integer: an integer gives a `badarg`. This is a test, not part of `hubd`; no design decision is made by it.
