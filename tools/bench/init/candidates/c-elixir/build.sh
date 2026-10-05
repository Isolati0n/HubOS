# architecture C with the Elixir/OTP brain: the s6rc candidate plus the brain as one extra s6 service. The Erlang runtime
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
