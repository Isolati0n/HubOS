# Language comparison experiment (Go against Elixir)

**EXPERIMENT. Not part of any image, not used by any test of the image.** It belongs to
`docs/proposals/language-comparison.md`, which has the results, the exact commands and the labels
(TESTED / SOURCE / BELIEVED / UNKNOWN). Written 2026-10-04 by a helper agent.

The same tiny program twice: the smallest version of the recovery agent's status call.

    GET /v1/status   ->   {"release":"7","flavor":"hub","slot":"a"}

The release and flavor come from the `version=` and `flavor=` lines of a release file (on a machine
`/etc/hubos-release`), the slot from the `hubos.slot=` word of the kernel command line (on a machine `/proc/cmdline`).
Both files can be pointed at fake files, so nothing here needs a Hub OS machine.

| Folder / file | What it is |
|---|---|
| `gostatus/` | Go, standard library only (`net/http`), part of the repo's Go module. Has its own tests. |
| `elixir-status/` | Elixir 1.14 mix project, no dependencies (no Hex). `:gen_tcp` with the VM's own HTTP packet parser, JSON written by hand. Has its own ExUnit tests. |
| `measure.py` | Python 3 helper: start time, idle memory, and "kill -9 under s6, time until it answers again". |
| `sizes.py` | Packs files into an initramfs-format (cpio newc) archive in memory and prints the gzip -9 size. |

## Go

    cd <repo root>
    go test -count=1 ./tools/image/experiments/language-comparison/...
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /tmp/gostatus ./tools/image/experiments/language-comparison/gostatus
    /tmp/gostatus -listen 127.0.0.1:8481 -release-file FAKE_RELEASE -cmdline-file FAKE_CMDLINE
    curl http://127.0.0.1:8481/v1/status

`-test-crash` turns on `/v1/crash` (a panic in the handler) and `/v1/crash-goroutine` (a panic in a goroutine the handler
started). They are off by default.

## Elixir

Needs Elixir and Erlang/OTP. The results in the proposal used Ubuntu 24.04's packages (Erlang/OTP 25.3.2.8, Elixir 1.14.0),
fetched with `apt-get download` and unpacked with `dpkg -x` into a temporary folder, never installed. Ubuntu's `erl` script
has `/usr/lib/erlang` written into it, so from a temporary folder the paths in the scripts had to be rewritten with `sed`.

    cd elixir-status
    MIX_ENV=test mix test                 # uses port 8481
    MIX_ENV=prod mix release              # the release is in _build/prod/rel/status (never commit it: it holds a random cookie)
    PORT=8481 HUBOS_RELEASE_FILE=FAKE_RELEASE HUBOS_CMDLINE_FILE=FAKE_CMDLINE RELEASE_DISTRIBUTION=none \
        _build/prod/rel/status/bin/status start

`RELEASE_DISTRIBUTION=none` switches Erlang's node-to-node networking off (no epmd, no second port). Without it the
release starts epmd and listens on all interfaces; see the proposal. `HUBOS_TEST_CRASH=1` turns on `/v1/crash` and
`/v1/crash-process`.

## Measuring

    python3 measure.py start 8481 15 -- COMMAND...        # 15 starts: time to first answer, idle RSS and PSS
    python3 measure.py killtime 8481 10 /path/to/service   # service dir watched by a running s6-svscan
    python3 sizes.py FILE_OR_FOLDER...

All numbers are from a shared 4-core container, not from the target hardware.
