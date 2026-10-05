#!/bin/bash
# Run a command in a private mount namespace (unshare -m) with two read-only overlays that vanish with the process.
# Nothing is written to /usr or /etc:
#  1. /usr/bin also shows the unpacked programs (Xwayland and cage call /usr/bin/Xwayland and xkbcomp by a fixed path)
#  2. /etc also shows two extra things from $BENCH_TMP/etc-extra:
#     - pam.d/weston-remote-access: a test-only PAM service that lets every password pass (Weston's VNC backend checks user and
#       password with PAM and has no switch to turn that off; the test machine has no real account to check)
#     - pki/CA/cacert.pem: the node's self-signed test certificate, because remote-viewer (gtk-vnc) only looks for its trusted
#       certificate in the fixed places /etc/pki/CA/cacert.pem and ~/.pki/CA/cacert.pem (TESTED with strace; $HOME is ignored)
. "$(dirname "$0")/../env.sh"
E=$BENCH_TMP/etc-extra; mkdir -p "$E/pam.d" "$E/pki/CA" "$BENCH_TMP/certs"
printf '#%%PAM-1.0\nauth    required pam_permit.so\naccount required pam_permit.so\n' > "$E/pam.d/weston-remote-access"
[ -f "$BENCH_TMP/certs/node.crt" ] || openssl req -x509 -newkey rsa:2048 -nodes -keyout "$BENCH_TMP/certs/node.key" -out "$BENCH_TMP/certs/node.crt" \
  -days 30 -subj "/CN=127.0.0.1" -addext "subjectAltName=IP:127.0.0.1" 2>/dev/null
cp "$BENCH_TMP/certs/node.crt" "$E/pki/CA/cacert.pem"
exec unshare -m sh -c 'mount -t overlay overlay -o lowerdir='"$BENCH_ROOT"'/usr/bin:/usr/bin /usr/bin && mount -t overlay overlay -o lowerdir='"$E"':/etc /etc && exec "$@"' sh "$@"
