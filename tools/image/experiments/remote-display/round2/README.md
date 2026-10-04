# Remote display, round 2 (EXPERIMENT; not part of any test or image)

Results and labels: `docs/proposals/remote-display.md`, section "Round 2". Nothing here is run by `go test` or any build script. Everything ran in the cloud build container, as root, on loopback only. **Nothing was installed**: every program was downloaded as a `.deb` (or built from source) and unpacked into a folder under `/tmp/r3-*`, run with `PATH` and `LD_LIBRARY_PATH` pointing there, and the folders were deleted at the end.

| File | What it is |
|---|---|
| `rfb-spy.py LISTEN SERVER LOG` | relay between a viewer and a VNC server (security type None only); logs each client-to-server message type, the encodings the viewer asks for, and every `ClientCutText` |
| `rfb-listen.py PORT SECONDS` | tiny RFB client that prints every `ServerCutText` the server sends |
| `rfb-dump.py LISTEN SERVER OUT` | relay that writes every byte to a file (used to grep for a clipboard text and show whether a link is encrypted) |
| `ovl.sh COMMAND...` | runs a command in a **private mount namespace** where `/usr/bin` also shows the unpacked `xkbcomp` and `Xwayland` (Xvfb and Xwayland call `/usr/bin/xkbcomp` by fixed path). The mount vanishes with the process; nothing is written to `/usr/bin` |
| `wayvnc-rsa-min.conf`, `wayvnc-tls.conf`, `remote-viewer-tls.vv` | the login configurations that were tested (test password only, not a secret) |
| `hub-inventory.toml`, `hub-viewers.toml` | the two-node inventory and viewers table used for the driftwm + hubd test (`remote-viewer` and `xtigervncviewer`) |

## How it was set up (all paths are examples)

```
# 1. download and unpack (no install). Same method as docs/driftwm-findings.md section 0.
A=/tmp/r3-a; O="-o Dir::State::lists=$A/apt/lists -o Dir::Cache=$A/apt/cache -o APT::Sandbox::User=root"
apt-get $O update
apt-get $O install --print-uris -y --no-install-recommends sway wayvnc libneatvnc0 tigervnc-viewer virt-viewer foot \
   wl-clipboard wtype grim xvfb xdotool wayland-utils xwayland xclip | grep -oE "^'[^']+'" | tr -d "'" > uris.txt
# curl -O each URL, dpkg -x each .deb into $A/root, then:  PATH=$A/root/usr/bin:$PATH LD_LIBRARY_PATH=$A/root/usr/lib/x86_64-linux-gnu
# 2. node = headless sway + wayvnc;  hub = sway on the wlroots X11 backend inside Xvfb (gives the hub a real keyboard), or driftwm (winit backend inside Xvfb)
WLR_BACKENDS=headless WLR_RENDERER=pixman WLR_LIBINPUT_NO_DEVICES=1 sway -c sway.conf          # node
WAYLAND_DISPLAY=wayland-1 wayvnc -L debug -o HEADLESS-1 127.0.0.1 5901
./ovl.sh Xvfb :90 -screen 0 1280x800x24 -nolisten tcp -xkbdir /usr/share/X11/xkb
DISPLAY=:90 WLR_BACKENDS=x11 WLR_RENDERER=pixman ./ovl.sh sway -c sway.conf                    # hub stand-in (sway)
# 3. driftwm at the pinned commit, built as in docs/driftwm-findings.md section 0 (3 min 40 s with 2 jobs), run:
DISPLAY=:92 ./ovl.sh driftwm --backend winit --config /dev/null          # xwayland-satellite must be on PATH for X11 programs
# 4. xwayland-satellite (not on crates.io; built from git b5690b56d749526a05db9b9268d58bf8f700957f, 2026-09-30, 1 min 27 s):
cargo build -j2       # needs libxcb-cursor-dev, libxcb-image0-dev, libxcb-render-util0-dev unpacked as for driftwm
# 5. newer wayvnc and neatvnc, built from the release tags (meson from pip into a --target folder; aml, neatvnc as meson subprojects):
python3 -m mesonbuild.mesonmain setup build -Dtests=false -Dpam=disabled -Dman-pages=disabled -Dscreencopy-dmabuf=disabled \
   -Dneatvnc:h264=disabled -Dneatvnc:gbm=disabled -Dneatvnc:examples=false     # wayvnc v0.10.2, neatvnc v1.0.3, aml 1.0.0
ninja -C build -j2
```

Self-signed certificate whose name matches the address the viewer uses (no certificate authority):

```
openssl req -x509 -newkey rsa:2048 -nodes -keyout tls2.key -out tls2.crt -days 3650 \
   -subj "/CN=127.0.0.1" -addext "subjectAltName=IP:127.0.0.1,DNS:node-test"
```

TigerVNC viewer with the certificate trusted and the login passed without a command line:

```
VNC_USERNAME=hubos VNC_PASSWORD=... xtigervncviewer -X509CA tls2.crt 127.0.0.1::5933
```

## Clean-up

`pkill -x sway`, `-x wayvnc`, `-x driftwm`, `-x xwayland-satellite`, `-x Xwayland`, `-x Xvfb`, `-x remote-viewer`, `-x xtigervncviewer`, `-x hubd`; then delete the `/tmp/r3-*` folders.
