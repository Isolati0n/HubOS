# Remote display experiments (EXPERIMENT; not part of any test or image)

Part 4 of the remote-display proposal: `docs/proposals/remote-display.md` (read that first; it has the results and the labels). Nothing here is run by `go test` or by any build script. Nothing is installed in an image. Everything was run in the cloud build container (no GPU, no real display, no real sound card), as root, on loopback only.

Packages used (Ubuntu 24.04, `apt-get install`): `sway wayvnc weston freerdp3-wayland virt-viewer pipewire pipewire-bin pipewire-pulse wireplumber xdotool wl-clipboard foot grim wtype xvfb sox mpv wayland-utils`. Versions: sway 1.9, wayvnc 0.7.2, neatvnc 0.7.1, weston 13.0.0, FreeRDP 3.32.0 (`wlfreerdp3`), virt-viewer 11.0, PipeWire 1.0.5, WirePlumber 0.4.17.

## Files

| File | What it is |
|---|---|
| `tcpmeter.py` | counting TCP relay on loopback (no tcpdump or ss in the container); writes the byte totals each way to a file every 0.5 s |
| `traffic.sh` | bytes per second on the display socket: idle, scrolling text, video (results in `results/traffic-*.txt`) |
| `windows.py` | prints app-id, title and size of every window from `swaymsg -t get_tree` |
| `hold-input.py` | keeps one VNC connection open so wayvnc keeps its virtual keyboard (a headless sway has no keyboard seat otherwise, and Wayland clients get no clipboard events without one) |
| `rfb-cut-text.py`, `rfb-security-types.py` | tiny RFB clients: send clipboard text; list the security types a server offers |
| `wayvnc-auth.example.conf` | wayvnc config with password login, TLS and RSA-AES keys (test password only) |
| `pipewire/start.sh MEDIA LATENCY_MS` | two PipeWire instances (`/tmp/p4-a` the node, `/tmp/p4-b` the hub) each with WirePlumber and its own session bus; an RTP sink on the node, an RTP source on the hub; `stop.sh` stops them |
| `pipewire/node-a.conf`, `hub-b.conf` | the PipeWire config drop-ins `start.sh` copies in |
| `pipewire/delay.py N` | measures the RTP delay (see the proposal, section 6.2) |
| `pipewire/findnode.py`, `peak.py` | find a PipeWire node by property or by process id; measure the peak heard on a sink |

## Order of the commands (all with `XDG_RUNTIME_DIR=/tmp/p4-run`, mode 0700)

```
# two headless wlroots compositors: "node" (wayland-1) and "hub" (wayland-2); sway.conf only sets the output size
WLR_BACKENDS=headless WLR_RENDERER=pixman WLR_LIBINPUT_NO_DEVICES=1 sway -c sway.conf     # node, then again for the hub
WAYLAND_DISPLAY=wayland-1 wayvnc -o HEADLESS-1 127.0.0.1 5901                              # node's VNC server
python3 tcpmeter.py 5902 5901 stats.txt &                                                  # meter between viewer and server
WAYLAND_DISPLAY=wayland-2 wayvnc -S hubctl.sock -o HEADLESS-1 127.0.0.1 5903 &             # hub only: gives the hub a keyboard seat
python3 hold-input.py 5903 &
GDK_BACKEND=wayland WAYLAND_DISPLAY=wayland-2 remote-viewer --name=hubos-test -t "Test Node" vnc://127.0.0.1:5902
./traffic.sh wayland-1 stats.txt 20
# Weston RDP
openssl req -x509 -newkey rsa:2048 -nodes -keyout rdp.key -out rdp.crt -days 1 -subj /CN=test
weston --backend=rdp --renderer=pixman --shell=kiosk --no-config --width=1280 --height=800 --port=3390 --address=127.0.0.1 --rdp-tls-cert=rdp.crt --rdp-tls-key=rdp.key --socket=wl-weston
WAYLAND_DISPLAY=wayland-2 wlfreerdp3 /v:127.0.0.1:3391 /u:x /p:x /cert:ignore /sec:tls +clipboard /title:"RDP Node" /wm-class:hubos-rdp /size:1000x700
# PipeWire
pipewire/start.sh audio 20 && python3 pipewire/delay.py 10 ; pipewire/stop.sh
```

The traffic script counts only what the test windows (app-id `load`) cause. The video is mpv playing `av://lavfi:testsrc2=size=640x480:rate=30` (a moving test pattern), not a film.

## Clean-up

Kill the processes by name or process id (`pkill -x sway`, `-x wayvnc`, `-x weston`, `-x wlfreerdp3`, `-x remote-viewer`, `-x pipewire`, `-x wireplumber`, `-x dbus-daemon`), then `rm -rf /tmp/p4-*` and `~/.local/state/wireplumber` (WirePlumber writes the stream volumes it remembers there).
