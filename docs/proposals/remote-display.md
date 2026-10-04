# PROPOSAL: how each node's display, sound and control reach the hub

**Status: PROPOSAL. Nothing here is decided and nothing here is in the images.** `HUB-OS.md` wins if this file disagrees with it. Written 2026-10-04. The only code is experiment scripts in `tools/image/experiments/remote-display/` (loopback, fake nodes, no real machine).

**Labels on every item:**
- **TESTED**: I ran it in this build container; the exact command and output are shown (or are in `tools/image/experiments/remote-display/`).
- **SOURCE**: read in an official source; the link and the date I read it (2026-10-04) are given. Many pages were read through a fetch tool that summarises the page with a small model, so the wording is not guaranteed to be the page's own; those are marked "(summarised)". Anything that matters for a decision should be re-read by a person on the page itself.
- **BELIEVED**: I think it is true; not tested, no official source.
- **UNKNOWN**: nobody has checked; do not rely on it.

What the experiments were, so nobody over-reads them: this is a cloud container with no GPU, no sound card, no real display and no real network. Two headless **sway** compositors (wlroots) stood in for "a node" and "the hub"; I did **not** run driftwm this time (it is not built in this container and a 5-minute build was not worth it; the app-id and title a viewer shows are the same Wayland window data driftwm reports, and the values below match the earlier driftwm runs in `docs/viewers-research.md`). Weston was version 13.0.0 (Ubuntu 24.04), not the newest. Software rendering only. All "delays" and "bytes" are loopback numbers.

---

## Plain-words summary

1. **The question.** Every node except the gaming box runs its own bespoke screen program. The hub must show it in a window, carry clipboard text, and play every node's sound together with a per-node volume and mute. Which "remote display" technology does that with the least upkeep?
2. **No candidate does everything.** The picture, mouse, keyboard and text clipboard can travel in **VNC (the RFB protocol)** or **RDP**. Sound can travel inside RDP and SPICE and Sunshine, but **not** inside plain VNC, and the one RDP server I could run (Weston) has **no sound code** (TESTED, 2.4). So sound has to be handled on its own whichever picture protocol is chosen.
3. **Sound on its own works.** PipeWire can send a node's sound to the hub over the network by itself (its "RTP" modules). I ran a sender and a receiver on loopback: **about 101 ms** of delay with the default setting and **about 17 to 38 ms** with a 20 ms setting (TESTED). Each node's sound then arrives as a stream with a name the hub chooses, so the hub can find it, mute it and set its volume with `wpctl` (TESTED, both for an RTP stream and for a stream a viewer would make). Nothing ties the sound to the picture: no lip-sync (BELIEVED), which the owner said is fine.
4. **The best fit for your rules (open standard, several makers, no pairing, no version lock-step) is VNC with wayvnc** on the nodes, plus PipeWire RTP for sound, plus a small control program on each node. VNC needs no pairing and no version matching (TESTED: no pairing step was needed). Its clipboard works node to hub (TESTED). Its weak points are plain: the only hub-side viewer we use (`remote-viewer`) had its last release in November 2021 (SOURCE), it refused a self-signed certificate (TESTED), and I could not make clipboard text go **hub to node** in my set-up (UNKNOWN why; the server accepts it when a plain RFB client sends it, TESTED).
5. **RDP with Weston is the second choice and is weaker today.** It also carries the clipboard node to hub (TESTED) and lets the client choose the screen size (TESTED), but Weston's RDP server does **not check any password** (TESTED), it crashed once during a reconnect (TESTED, not reproduced), and the Linux FreeRDP client I used says on start-up that it is **deprecated** (TESTED).
6. **SPICE should not be chosen for new work.** Red Hat removed it from RHEL 9 and says VNC is the replacement (SOURCE); `virt-viewer`'s last release tag is 2021 (SOURCE). It only fits virtual machines anyway.
7. **Sunshine and Moonlight stay for the gaming box** (decided). For other nodes they need a pairing step per node, carry no clipboard back, and belong to one project family. They are the only candidate that carries GPU video and sound together.
8. **Control and status (liveness, start and stop of the session, version, power) do not belong in any of these protocols.** The smallest honest design is a small HTTP and JSON API on each node, signed with one cluster-wide key (no per-node pairing). It can be the **same API** as Part 3's recovery agent, on the same port, with a `state` field. Section 4.
9. **GPU apps (Blender, ParaView on the AI box) are not tested** (no GPU here). VNC from wayvnc copies frames through the CPU; whether that is good enough for Blender is UNKNOWN. This is the biggest open risk for the recommendation. Section 6.
10. **Questions for you are in section 8.** The most important: is a window for the AI box over VNC acceptable until December measurements, or must it be Sunshine from the start?

---

## 1. Candidates

All version and date facts were read on 2026-10-04.

### 1.1 Overview table

| | VNC with **wayvnc** | **QEMU's built-in VNC** | **Weston RDP** (+ FreeRDP client) | **Weston VNC** backend | **Sunshine + Moonlight** | **SPICE** |
|---|---|---|---|---|---|---|
| **The standard** | RFB, [RFC 6143](https://www.rfc-editor.org/rfc/rfc6143), March 2011, category **Informational**, by RealVNC staff (summarised). Newer features (TLS, extended clipboard, resize) are extensions outside that RFC. | Same RFB | RDP: [MS-RDPBCGR](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpbcgr/5073f4ed-1e93-45e1-b039-6e30c385867c), a Microsoft "Open Specification", revision 62.0 of 2026-03-09. Not an IETF or ISO standard. Microsoft says it may hold patents on implementations and that a given document may be covered by its Open Specifications Promise (same page). | Same RFB | **No published specification found** (BELIEVED: the NVIDIA GameStream protocol, re-implemented by the community; I did not find a spec document) | SPICE protocol by Red Hat; spec documents on spice-space.org. Not a standards-body standard (BELIEVED). |
| **Maintained servers** | wayvnc (any1 / Andri Yngvason's repository). Others for the same protocol: TigerVNC (Xorg `Xvnc`, `x0vncserver`, and `w0vncserver` for Wayland since 1.16.0), QEMU, Weston's backend | QEMU only | Weston RDP backend; **xrdp** (Xorg and Xvnc based); **GNOME Remote Desktop**; **KRdp** (KDE); FreeRDP's server library; Windows | Weston only | Sunshine only | QEMU (through spice-server); no other maintained server found (UNKNOWN: not searched exhaustively) |
| **Maintained clients** | `remote-viewer` (gtk-vnc), **TigerVNC viewer**, Remmina, KRDC and others (the last three not examined) | same | **FreeRDP** (`xfreerdp3`, `wlfreerdp3`, SDL3 client), Remmina, KRDC (not examined), Microsoft's own | same as VNC | **Moonlight** (moonlight-qt for PC; other Moonlight apps for phones and TVs) | `remote-viewer` (spice-gtk), `spicy` |
| **Latest release** (SOURCE) | **wayvnc v0.10.2, 2026-09-25** ([releases feed](https://github.com/any1/wayvnc/releases.atom), summarised). Library neatvnc: 0.7.1 in Ubuntu 24.04 (TESTED `wayvnc --version`); newest neatvnc not read. TigerVNC **1.16.2, 2026-03-26** ([feed](https://github.com/TigerVNC/tigervnc/releases.atom)). | **QEMU 11.1.2, 2026-09-28** ([download page](https://www.qemu.org/download/)) | Weston **16.0.0, 2026-07-14** ([releases](https://wayland.freedesktop.org/releases.html)); I ran **13.0.0**. FreeRDP **3.32.1, 2026-09-28**, 3.32.0 on 2026-09-23 ([feed](https://github.com/FreeRDP/FreeRDP/releases.atom), summarised). xrdp **v0.10.6.1, 2026-07-07** ([feed](https://github.com/neutrinolabs/xrdp/releases.atom)). | as Weston | Sunshine **v2026.1003.221627, 2026-10-03** ([feed](https://github.com/LizardByte/Sunshine/releases.atom)). Moonlight PC **v6.2.0, 2026-10-04** (the feed shows 6.1.0 on 2024-09-17 before it, so about two years between releases) ([feed](https://github.com/moonlight-stream/moonlight-qt/releases.atom)). | spice-space.org's download page lists spice server **0.16.0**, spice-gtk **0.41**, spice-vdagent **0.22.0**, **no dates** ([page](https://www.spice-space.org/download.html), summarised). `virt-viewer` newest tag **v11.0, 2021-11-18** ([tags feed](https://gitlab.com/virt-viewer/virt-viewer/-/tags?format=atom), summarised). Ubuntu 24.04 ships spice-server 0.15.1 (TESTED `apt-cache policy libspice-server1`). The dates of the spice and spice-gtk releases: **UNKNOWN**, the freedesktop GitLab refused the fetch ("Access Denied", an anti-bot page). |
| **Licence** | wayvnc: ISC (GitHub page, summarised; Ubuntu's copyright file lists ISC, Expat, CC0, Unlicense, TESTED `grep License /usr/share/doc/wayvnc/copyright`). neatvnc: ISC, BSD-3, Expat, CC0. TigerVNC: GPL (summarised) | QEMU: GPL-2.0 (BELIEVED, not read) | Weston: MIT, X11 and CC-BY-SA docs (TESTED, Ubuntu copyright file). FreeRDP: **Apache-2.0** ([LICENSE](https://raw.githubusercontent.com/FreeRDP/FreeRDP/master/LICENSE)). xrdp: Apache-2.0 (README badge, summarised) | MIT | Sunshine **GPL-3.0** ([LICENSE](https://raw.githubusercontent.com/LizardByte/Sunshine/master/LICENSE)); Moonlight **GPL-3.0** ([LICENSE](https://raw.githubusercontent.com/moonlight-stream/moonlight-qt/master/LICENSE)) | spice-gtk: LGPL-2.1+ (TESTED, Ubuntu copyright file); spice server: BELIEVED LGPL; virt-viewer: not read (the Ubuntu copyright file's "License:" line was empty in my grep) |
| **Maintainers** | wayvnc: one main author (the repository owner `any1`; README names "~andri"), summarised. TigerVNC: a community team (not examined). | the QEMU project | Weston and FreeRDP: community projects (no named company found). **FreeRDP's own log says "nobody is actively working on" `wlfreerdp3`** (TESTED, 2.3). | as Weston | Sunshine: the **LizardByte** organisation (GitHub, summarised). Moonlight: the `moonlight-stream` organisation; individual maintainers not read (UNKNOWN). | historically Red Hat (BELIEVED, not sourced); who maintains it now: **UNKNOWN** |
| **Pairing or version matching** | **None.** RFB starts with a version handshake (`RFB 003.008`, used by my test clients). No pairing. (TESTED: clients connected with no step beyond the optional password.) | None | **None** for the Weston server in my test (it asked only for a user name, and **accepted any**, section 5). RDP clients and servers negotiate their abilities (BELIEVED). | None | **Pairing by PIN per client and server** (decided by the owner as "by hand once per node for v1", `HUB-OS.md`). Version coupling between Moonlight and Sunshine: BELIEVED real but not researched here; the Moonlight 6.2.0 notes list three security fixes (summarised). | None for the protocol; password or TLS optional |
| **Signs of abandonment** | wayvnc: three releases in 2026 (April, July, September), so active. `remote-viewer`: last release 2021 (SOURCE above). | none seen | Weston and FreeRDP: both released in September 2026. **`wlfreerdp3` is deprecated** by its own authors (TESTED). Weston's RDP backend: not looked at in depth. | none seen | Active. One signal: Moonlight released 6.1.0 in September 2024 and 6.2.0 in October 2026. | **Yes.** Red Hat: SPICE "deprecated in RHEL-8.3 and is to be removed from RHEL-9" ([bug 1946939](https://bugzilla.redhat.com/show_bug.cgi?id=1946939), opened 2021-04-07, summarised). RHEL 9 documentation: "In RHEL 9, the SPICE remote display protocol is no longer supported. QXL ... has also become unsupported." and VNC is recommended, with audio playback and USB redirection listed as lost ([RHEL 9 virtualization considerations](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/9/html/considerations_in_adopting_rhel_9/assembly_virtualization_considerations-in-adopting-rhel-9), summarised). The RHEL 10 removed-features page does **not** mention SPICE (summarised; so it was already gone). virt-viewer: no release since 2021-11-18. |

### 1.2 What each carries inside the protocol

| | Picture | Mouse and keyboard | Clipboard | Sound | Resize |
|---|---|---|---|---|---|
| **RFB base** | yes | yes | **text only, Latin-1**: "no way to transfer text outside the Latin-1 character set" (RFC 6143, summarised). An **extended clipboard** (UTF-8) exists as an extension: neatvnc 0.9 added it (summarised search result, [Phoronix](https://www.phoronix.com/news/WayVNC-0.9-Wayland-VNC)). | **none in the RFC.** wayvnc: none found; a user asked for it ([issue 209](https://github.com/any1/wayvnc/issues/209), summarised search result). QEMU's VNC has an **extension**: `audiodev` "Use the specified audiodev when the VNC client requests audio transmission" ([QEMU docs](https://www.qemu.org/docs/master/system/invocation.html), summarised). | an extension, not in the RFC. TESTED: wayvnc to `remote-viewer` followed a change of the node's screen size (2.5). |
| **wayvnc** | yes | yes, through the compositor's virtual pointer and keyboard | yes, node to hub **TESTED**; hub to node: see 2.2. Uses the compositor's `wlr-data-control` ([Phoronix 0.3](https://www.phoronix.com/news/WayVNC-0.3-Released), summarised). | **no** | server to client: TESTED. Client to server: **not observed** (2.5) |
| **RDP** (the protocol) | yes | yes | yes (a separate channel, `cliprdr`) | **yes** (a separate channel, `rdpsnd`; MS-RDPEA, not read by me) | yes (display-control channel; BELIEVED) |
| **Weston 13.0.0 RDP backend** | yes | yes | node to hub **TESTED**; hub to node not observed (2.2) | **no code found** (TESTED, 2.4) | TESTED: the desktop took the size the client asked for at connect (2.5) |
| **Sunshine + Moonlight** | yes (hardware encoded) | yes | **none in the stream**; Moonlight only types the hub's clipboard text as key presses on the host (`docs/viewers-research.md`, SOURCE: moonlight-qt source) | yes, "7.1 surround" ([README](https://raw.githubusercontent.com/moonlight-stream/moonlight-qt/master/README.md), summarised) | the stream takes the size the client asks for (BELIEVED, not researched) |
| **SPICE** | yes | yes | yes, **needs the guest agent** on the VM (`docs/viewers-research.md`; libvirt's `<clipboard copypaste>` switch) | yes (the RHEL 9 text in 1.1 lists audio as lost when moving to VNC) | yes, with the agent (BELIEVED) |

---

## 2. Experiments with a headless compositor and loopback

All commands are in `tools/image/experiments/remote-display/README.md`. Setting: two headless sway instances; the "node" (`wayland-1`) runs `wayvnc` or a Weston RDP server; the "hub" (`wayland-2`) runs the viewer. The viewer connects through `tcpmeter.py`, a counting relay, because the container has no `tcpdump` or `ss`.

### 2.1 Window name and title (the hub's view)

TESTED. `hub swaymsg -t get_tree | python3 windows.py` (sway reports the same `app_id` and title driftwm would).

| Viewer command | app-id | title | size |
|---|---|---|---|
| `remote-viewer --name=hubos-test -t "Test Node" vnc://127.0.0.1:5902` | `hubos-test` | `Test Node (1)` | follows the node's screen (1024 x 815 window at first; later 800 x 647 after the node's screen became 800 x 600) |
| `wlfreerdp3 /v:127.0.0.1:3391 ... /title:"RDP Node" /wm-class:hubos-rdp` | `hubos-rdp` | `RDP Node` | 1280 x 800 |
| `wlfreerdp3 /v:127.0.0.1:3391 ...` (no name options) | `wlfreerdp` | `FreeRDP` | 1280 x 800 |

So both viewers let hubd choose the name and title, as `viewers.toml` already expects (`sets_name = true`). `remote-viewer` adds " (1)" to the title (as `docs/viewers-research.md` found). Whether `wlfreerdp3`'s `/wm-class` becomes the app-id under **driftwm** (not sway) is UNKNOWN but it is the same Wayland field.

### 2.2 Clipboard text, both ways

TESTED, `wl-copy` / `wl-paste` on each side.

| Direction | wayvnc + `remote-viewer` | Weston RDP + `wlfreerdp3` |
|---|---|---|
| node to hub | `node wl-copy "from-node-C"` then `hub wl-paste` printed `from-node-C` | `WAYLAND_DISPLAY=wl-weston wl-copy "rdp-node-text-1"` then `hub wl-paste` printed `rdp-node-text-1` |
| hub to node | **not seen.** `echo hub-first-E \| hub wl-copy` then `node wl-paste` still printed the old node text, in several tries, including after changing which window had focus | **not seen.** `node wl-paste -n` printed nothing 3 times in a row; Weston's log said `outstanding RDP data request (client to server)` |

What I can and cannot say. The wayvnc **server** does take text from a client: my own tiny RFB client (`rfb-cut-text.py 5901 raw-cut-text-2`) made `node wl-paste` print `raw-cut-text-2`, and `wl-paste --list-types` showed `text/plain;charset=utf-8` (TESTED). So the missing piece is on the viewer side or in my hub stand-in. My hub is a headless sway, which has no keyboard until something creates one; I gave it one through a second wayvnc and still did not see hub to node. Cause: **UNKNOWN**. The owner's requirement (clipboard both ways) is therefore **only half shown** here, for both candidates. It must be re-tested under driftwm with a real keyboard, with these viewers and with the TigerVNC viewer.

The wayvnc **server** also dropped the text when the client closed: the first raw test (client closed after 1.5 s) left the node with "Nothing is copied"; with the client kept open 8 s the text stayed. BELIEVED consequence: pasted text lives only while the viewer stays connected. One run only.

### 2.3 The client closes and reconnects

TESTED.

- wayvnc: `pkill -x remote-viewer`, wait 2 s: `wayvnc` still running (same pid 4589), the node's window still there (1 window). A new `remote-viewer` connected within 4 s and showed the same node screen. The node's foot window kept its content (it was a running window, count unchanged).
- Weston RDP: closing `wlfreerdp3` left `weston` running and the node's window running; a new client connected and showed the same desktop (two tries with no clipboard use, one with a clipboard copy before closing).
- **Weston crashed once.** `dmesg`: `weston[12339]: segfault at 28 ... in libwayland-server.so.0.22.0`. It happened when a client reconnect followed several hub-to-node clipboard attempts that Weston logged as `outstanding RDP data request`. I tried three more times (clipboard copy then close, paste pending then close) and could **not** reproduce it. Cause **UNKNOWN**; a Weston 13.0.0 bug is the guess (BELIEVED). Weston 16 was not tried.
- Closing the viewer window with its close button under driftwm shows a "Do you want to close the session?" dialog for `remote-viewer` (`docs/viewers-research.md`). That dialog does not exist for the node side; the session stays alive either way.

### 2.4 Weston's audio hook

TESTED:

```
$ strings /usr/lib/x86_64-linux-gnu/libweston-13/rdp-backend.so | grep -i -E "rdpsnd|audio|pulse|pipewire|cliprdr"
cliprdr_server_context_new
cliprdr_server_context_free
Virtual channel is required for clipboard, audio playback/capture
```

There is a clipboard channel server and **no** `rdpsnd` server and no PipeWire or PulseAudio call in Weston 13.0.0's RDP backend. The one string that names audio is an error message. So no audio comes out of Weston's RDP in this version (TESTED for 13.0.0; Weston 16: UNKNOWN). A search result says "audio is uncompressed PCM" and "didn't play well with the FreeRDP pulse client plugin", which looks like it refers to Microsoft's fork of Weston, not the upstream one (BELIEVED, [weston-rdp(7)](https://man.archlinux.org/man/weston-rdp.7.en) says nothing about audio).

### 2.5 Resize

TESTED.

- wayvnc, node to hub: `node swaymsg output HEADLESS-1 resolution 1024x600` and later `800x600`: the `remote-viewer` window followed (hub saw the window become 800 x 647, which is the 800 x 600 picture plus the viewer's own menu bar).
- wayvnc, hub to node (the viewer asks the node to change size): `remote-viewer --auto-resize=always`, then `swaymsg resize set 640 480` on the viewer's window: the node's screen stayed 1280 x 800. **Not observed.** (neatvnc contains a function named `nvnc_set_desktop_layout_fn`, TESTED with `strings`, so the server side may support the request; I did not test further; UNKNOWN.)
- Weston RDP: the Weston server was started `--width=1280 --height=800`; the client asked `/size:1000x700`; `wayland-info` on Weston printed `logical_width: 1000, logical_height: 700`. So the client chooses the size **at connect**. Resizing the viewer window afterwards did not change it (`wlfreerdp3`, with `+dynamic-resolution`): not observed. The man page says "By default when a client connects on the RDP backend, it will instruct weston to resize to the dimensions of the client's announced resolution" ([weston-rdp(7)](https://man.archlinux.org/man/weston-rdp.7.en), summarised).

For this project the hub window is the whole desktop and the node's screen size can be fixed per node in its config, so resize matters less than it sounds (BELIEVED).

### 2.6 Traffic: idle, scrolling window, video

TESTED. `tools/image/experiments/remote-display/traffic.sh NODE_DISPLAY STATS 20`. Bytes counted by `tcpmeter.py` on the viewer's TCP connection, over 20 s each. Scrolling: `foot` printing `/proc/uptime` in a loop (every frame differs; I first used `seq`, which gave identical frames, and threw that result away). Video: `mpv` showing a moving test pattern, 640 x 480, 30 frames per second (a test pattern, **not** a film).

```
VNC (wayvnc 0.7.2 + remote-viewer), results/traffic-vnc.txt
idle (no window changes): server->client 0 bytes in 20s = 0 kbit/s; client->server 0 bytes
scrolling text (...): server->client 4026461 bytes in 20s = 1610 kbit/s; client->server 102790 bytes
video (...640x480, 30 fps...): server->client 122378065 bytes in 20s = 48951 kbit/s; client->server 299400 bytes

RDP (Weston 13.0.0 rdp backend, pixman + wlfreerdp3), results/traffic-rdp.txt
idle (no window changes): server->client 0 bytes in 20s = 0 kbit/s; client->server 0 bytes
scrolling text (...): server->client 51095098 bytes in 20s = 20438 kbit/s; client->server 0 bytes
video (...640x480, 30 fps...): server->client 31231289 bytes in 20s = 12492 kbit/s; client->server 0 bytes
```

In words: **an idle desktop sends nothing at all** over the socket in either protocol (no keep-alive in 20 s). Busy content costs tens of megabits per second. At a dozen windows with maybe one busy, that is far below 10 GbE (BELIEVED). The two protocols behave differently (VNC cheap for text, expensive for video; RDP the other way round in this run), but the content, window sizes and encoders differ, so **do not read this as "RDP is better for video"**. The RDP numbers are for a software (pixman) server. Real encoders (H.264 in wayvnc with `-g`, RDP graphics pipeline) were not tested (UNKNOWN).

### 2.7 Weston's VNC backend

TESTED. `weston --backend=vnc ...` without a certificate stops: `The VNC backend requires a key and a certificate for TLS security (--vnc-tls-cert/--vnc-tls-key)` and `fatal: failed to create compositor backend`. With a self-signed certificate it starts, but `remote-viewer` says `vnc-session: got vnc error The certificate is not trusted` and shows an empty-titled error window (595 x 169). I did not get further, so clipboard and traffic for this backend are **UNKNOWN**. What this does show: **`remote-viewer` does not accept a self-signed VNC certificate**, so a certificate setup (a private CA the viewer trusts) would be needed (how: UNKNOWN).

---

## 3. Audio, in depth

### 3.1 (a) What sound each candidate carries and who really provides it

| Server | Sound to the hub? | Label |
|---|---|---|
| wayvnc | **No** (no audio feature; a feature request exists) | [issue 209](https://github.com/any1/wayvnc/issues/209), summarised search result |
| TigerVNC | none mentioned in its README | summarised |
| QEMU's VNC | **yes**, an extension, with `-vnc ...,audiodev=ID`; **gtk-vnc** (inside `remote-viewer`) contains audio functions (`vnc_audio_format_new`, `vnc_audio_sample_new`, TESTED with `strings libgvnc-1.0.so.0`). Whether the pair works: UNKNOWN. VMs only. | SOURCE QEMU docs; TESTED strings |
| Weston RDP 13.0.0 | **No** (2.4) | TESTED |
| **FreeRDP** | has a server-side sound channel library: `channels/rdpsnd/server` exists in its source tree ([directory](https://github.com/FreeRDP/FreeRDP/tree/master/channels/rdpsnd), summarised). A server must call it; Weston 13 does not. | SOURCE |
| **xrdp** | **yes**, but "requires to build additional modules": `pulseaudio-module-xrdp`, which does output and input redirection through PulseAudio ([xrdp README](https://raw.githubusercontent.com/neutrinolabs/xrdp/devel/README.md), [module README](https://raw.githubusercontent.com/neutrinolabs/pulseaudio-module-xrdp/devel/README.md), both summarised). PipeWire support: **not stated** in what I read. xrdp's displays are Xorg (`xorgxrdp`) or `Xvnc`; **no Wayland** mentioned. So it fits an X11 desktop, not our Wayland kiosk compositors. | SOURCE |
| **GNOME Remote Desktop** | Its release notes (versions 45 to 51) do not mention audio (summarised). **UNKNOWN.** It needs a running GNOME session (BELIEVED), which a bespoke OS does not have. | SOURCE |
| **KRdp** | audio "not mentioned in the documentation; appears unsupported" ([README](https://invent.kde.org/plasma/krdp/-/raw/master/README.md), summarised). Needs KWin (BELIEVED). | SOURCE |
| Sunshine | yes (stream audio); a host-side sink can be set | SOURCE (`docs/viewers-research.md`) |
| SPICE | yes (playback and record) | RHEL 9 text in 1.1 |

**What would capture a node's PipeWire output for an RDP server?** A server needs a hook that reads the node's sound and hands it to the sound channel. Weston 13 has none (2.4). Anything would have to be written and kept as a patch, which is the opposite of "least maintenance". **PipeWire already has the other half**: a sink-monitor capture and an RTP sender (3.2). So I recommend that no candidate's own sound path be used, except Sunshine's for the gaming box.

### 3.2 (b) PipeWire's own network sound: RTP sink and source

**What the modules do (SOURCE, [rtp-sink](https://docs.pipewire.org/page_module_rtp_sink.html), [rtp-source](https://docs.pipewire.org/page_module_rtp_source.html), [rtp-session](https://docs.pipewire.org/page_module_rtp_session.html), [rtp-sap](https://docs.pipewire.org/page_module_rtp_sap.html), all summarised):**
- `libpipewire-module-rtp-sink` sends sound as RTP. Arguments: `destination.ip` (default `224.0.0.56`, a multicast address), `destination.port` (default a random number from 46000 to 47024), `sess.media` (`audio`, `midi` or `opus`, default `audio`), `net.mtu` (1280), `sess.min-ptime` (2 ms) and `sess.max-ptime` (20 ms), `sess.latency.msec`, `net.ttl` (1). It mentions AES67 only through an option `aes67.driver-group`.
- `libpipewire-module-rtp-source` receives. `source.ip`, `source.port`, **`sess.latency.msec` default 100**, `sess.media`. "In constant latency mode (default), a DLL adjusts consumption to maintain target latency."
- `rtp-session` finds peers by Avahi/mDNS (Apple MIDI compatible for MIDI). `rtp-sap` announces and creates streams from SAP announcements, with rules (`announce-stream`, `create-stream`), SAP at `224.0.0.56:9875`.
- AES67: a config file `pipewire-aes67.conf` ships with PipeWire (TESTED: `ls /usr/share/pipewire/`). AES67 wants a shared PTP clock; I did not test it and the project does not need sample-accurate sync (latency does not matter, per the owner).

**Test.** Two PipeWire instances (`/tmp/p4-a` the node, `/tmp/p4-b` the hub), each with WirePlumber and its own session bus, PipeWire 1.0.5. The node has a null sink `node-null` and an RTP sink to `127.0.0.1:46000`; the hub has a null sink `hub-out` and an RTP source from `127.0.0.1:46000`. The config is in `pipewire/node-a.conf` and `hub-b.conf`. TESTED:

```
$ XDG_RUNTIME_DIR=/tmp/p4-a pw-link -l
node-null:monitor_FL
  |-> rtp-from-a:send_FL
...
$ XDG_RUNTIME_DIR=/tmp/p4-b pw-link -l
hub-out:playback_FL
  |<- rtp-from-a-at-hub:receive_FL
```

(The RTP sink captured the monitor of the node's default sink, so **whatever a node's apps play to its default sink is sent**. No app needs to know about RTP.)

Delay measured by `pipewire/delay.py`: a 1 kHz, 50 ms burst is played to `node-null`; the time it first shows at the node's sink monitor (before the network) is compared with the time it first shows at the hub's sink monitor (after RTP). Both taps use the same `pw-cat` settings, so their own delay mostly cancels. TESTED, `results/delay-results.txt` and `delay2.txt`:

```
rtp-source sess.latency.msec=100 (the default), raw audio:   n=10  delay ms: min 101  median 101  max 120
rtp-source sess.latency.msec=20,  raw audio:                 n=8   delay ms: min 17   median 17   max 38     (a second run: n=6, min 25 median 33 max 33)
rtp-source sess.latency.msec=5,   raw audio:                 no sound reached the hub (4 start attempts; cause UNKNOWN)
sess.media=opus:                                             delay NOT measurable; see below
```

- **Raw audio, default setting: about 101 ms.** So the default buffer is the delay. With a 20 ms setting: 17 to 38 ms on loopback. A real network would add its own jitter; with a bigger buffer you trade delay for safety. Since delay does not matter here, the default (or larger) is the safe choice (BELIEVED).
- **Opus: not measured.** The module accepted `sess.media=opus` but my detector saw sound at the hub 7 ms after the player started, which cannot be real (the decoder probably produced noise above my threshold). I did not investigate. Opus needs an Opus library in PipeWire (it loaded); whether it is worth it: raw stereo 48 kHz 16 bit is **1.536 Mbit/s per node** (calculation), about 18 Mbit/s for a dozen nodes, which is nothing on a fast wired network. So raw is enough (BELIEVED).
- **Start-up race.** Often the RTP sink printed `stream error: no target node available` (TESTED, `/tmp/p4-a/pw.log`) and no link was made: `start.sh` printed `links missing, restarting` 9 times over 7 runs (counted from the output; the 5 ms setting never worked). The sink needs the node's null sink and WirePlumber to be ready first. `start.sh` restarts until the links exist. On a real node this means: **start order matters** (PipeWire, then WirePlumber, then the sender), and something must notice and retry.
- **`pw-cli load-module` did nothing in this build**: it printed nothing and the module was not loaded (TESTED). Config drop-in files (`pipewire.conf.d/*.conf`) worked. So per-node receivers on the hub must be **config**, not a runtime command, unless SAP discovery is used (not tested).
- **Hub side: one source per node.** With unicast, each node needs its own port (46000 + n) and its own `rtp-source` entry on the hub; with the default multicast address, every receiver would hear every sender. A fixed port per node in the inventory is the simplest (BELIEVED). SAP would automate this (UNKNOWN, not tested).

### 3.3 (c) The per-node mixer: find a stream, mute it, set its volume

**WirePlumber needs a session bus.** TESTED: started alone, `wireplumber` printed `Error acquiring bus address: Cannot autolaunch D-Bus without X11 $DISPLAY` and disconnected from PipeWire. With a plain `dbus-daemon --session --address=unix:path=...` it ran. This is the same finding as Waybar's (`docs/bar-findings.md`) and fits the existing rule that the hub may declare a D-Bus session bus. A node that only **sends** sound needs PipeWire and a way to link its sender; whether it needs WirePlumber too is UNKNOWN (in my test WirePlumber made the link). `pipewire` itself ran without the bus except for two error lines about the session bus.

**A stream played by a viewer** (stand-in: `pw-cat --playback ... -P '{ application.name=remote-viewer node.name=viewer-ai }'`, which makes a stream like the PulseAudio stream a viewer would). TESTED, on the hub instance:

```
$ ./findnode.py pid 27343              # the process id of the player
50
$ ./findnode.py node-prop node.name viewer-ai
50
$ ./peak.py hub-out                    # peak sample heard on the hub's sink, test tone of height 9000
9000
$ wpctl set-volume 50 0.25 ; ./peak.py hub-out
141          (wpctl get-volume 50: "Volume: 0.25")
$ wpctl set-mute 50 1 ; ./peak.py hub-out
0            (wpctl get-volume 50: "Volume: 0.25 [MUTED]")
$ wpctl set-mute 50 0 ; wpctl set-volume 50 1.0 ; ./peak.py hub-out
9000
```

Things learned:
- The **process id is a property of the client, not of the node.** `findnode.py pid` joins the node's `client.id` to the client's `application.process.id` (from `pw-dump`). hubd knows the process id of every viewer it started (it records it), so for a viewer that plays through PulseAudio compatibility (`pipewire-pulse`) hubd can find the stream. Whether `remote-viewer`'s real stream carries the right process id and a usable name: **UNKNOWN** (the real viewer has no sound to play in this container).
- **`wpctl` percentages are on a cubic scale**: `0.25` gave a peak of 141 out of 9000 (0.25 cubed is 0.0156). A "volume slider" in the panel must say which scale it uses (BELIEVED consequence; the number is TESTED).

**A named RTP stream.** The hub's `rtp-source` creates a stream node with the `node.name` we gave it (`rtp-from-a-at-hub`), so the name is stable and no process id is needed. TESTED: a tone played at the node, `wpctl set-mute 35 1` on the RTP stream (id 35):

```
RTP heard, unmuted: peak 7000
RTP stream muted: peak 0 (Volume: 1.00 [MUTED])
RTP stream 0.5: peak 875 (Volume: 0.50)
```

**Do not rely on WirePlumber's own memory for this.** WirePlumber has a "restore-stream" script. TESTED: I set stream `viewer-ai` to volume 0.3, muted it, killed the player and started a new one with the same application name and a **different** `node.name`: `wpctl get-volume` on the new stream said `Volume: 0.30 [MUTED]`. The state file `/root/.local/state/wireplumber/restore-stream` keys streams by media role first (`Output/Audio:media.role:Music:...`), so **all streams that do not set a role share one saved volume**. The script's own key order is `media.role`, `application.id`, ..., `media.name`, `node.name` (TESTED, `grep -n -A8 "local keys" restore-stream.lua`). So per-node memory by WirePlumber would be wrong. The option `["restore-props"] = true` in `/usr/share/wireplumber/main.lua.d/40-stream-defaults.lua` (TESTED) would be switched off, and hubd would do the remembering.

**What hubd would store, where, and how it is restored (proposal):**
- Store: for each machine id, `{volume, muted}`, plus `master` (`{volume, muted}`). A small JSON or TOML file written by hubd whenever the owner changes a value. Not a secret.
- Where: **not** the record file (it is in memory and vanishes at reboot, `docs/hubd-slice2.md` section 2.2) and **not** the inventory (the owner edits that by hand and it gets the NAS backup copy). It needs persistence across reboots, so it belongs in the hub's persistent area. Which partition and whether it gets a NAS backup: **question for the owner (8, Q9)**.
- Restore: for an RTP stream, the stream exists as soon as the hub's PipeWire starts, so hubd applies the stored volume and mute at **hubd start** and once more whenever the stream node appears again (`pw-dump --monitor` or polling `pw-dump`). For an in-protocol viewer stream, hubd waits for the node with the viewer's process id to appear after it starts the viewer, then applies the values. There is a short gap in which the stream can play at the wrong volume (BELIEVED; how short: UNKNOWN). To close the gap, the stream could be created muted by a WirePlumber rule; I did not test that.
- Master and "mute all": the master is the hub's default sink (`wpctl set-volume @DEFAULT_AUDIO_SINK@ ...`, `wpctl set-mute @DEFAULT_AUDIO_SINK@ 1`). Stream volume and sink volume multiply (BELIEVED; I only tested the stream part).

### 3.4 (d) How the mixer could be shown

| Option | What | Label |
|---|---|---|
| **Waybar's own `wireplumber` module**, for the master only | options `format`, `format-muted`, `on-click`, `on-scroll-up/down`, `scroll-step`, `max-volume`; it controls the default sink only ([man page](https://raw.githubusercontent.com/Alexays/Waybar/master/man/waybar-wireplumber.5.scd), summarised). Waybar's newest release is 0.15.0 of 2026-02-06 ([feed](https://github.com/Alexays/Waybar/releases.atom), summarised); Ubuntu 24.04 has 0.9.24. **In an earlier test the module made Waybar exit** (`can't make support.system handle`, cause UNKNOWN, maybe the unpacked set-up) (`docs/bar-findings.md` section 8). | SOURCE, TESTED earlier |
| **A list in the existing menu style** (wofi), opened from a volume button on the bar | hubd prints one line per node (`AI box  70%  [muted]`); picking a line toggles mute or steps the volume (`+10`, `-10` as extra lines). Same technique as the machine list (`docs/bar-findings.md` section 6). No slider. | BELIEVED (the list technique is TESTED there; this use is not) |
| **A small layer-shell panel** (our own program) | real sliders and mute buttons per node. A new program to write and keep. | BELIEVED; nothing tested |

**Smallest honest design:** a bar button (a Waybar custom module fed by hubd, like the machine alert) that shows the master state; **click** opens the wofi-style list with one line per node plus lines for master and "mute all"; **scroll on the button** steps the master volume. All values go through hubd, which calls `wpctl`. Sliders only if you later want them. None of this was built or run.

### 3.5 (e) Does the sound stay in step with the picture?

**No, and nothing is built to make it.** BELIEVED (reasons): RFB frames carry no timestamps (RFC 6143 defines none; summarised), and the RTP stream's clock is PipeWire's own. The RTP source delays the sound by its latency setting (101 ms default, TESTED) and the picture has its own delay (unmeasured, UNKNOWN). So a video playing in a node window would show an offset of at least the sound buffer, perhaps tenths of a second; the owner said latency does not matter. A protocol that carries both (RDP, SPICE, Sunshine) can in principle keep them together (BELIEVED, not tested); the price is losing PipeWire's per-stream control, because the sound then arrives inside the viewer's own stream.

### 3.6 (f) What a dedicated audio node would need

If the hub's own sound output is not used (for example the projector's HDMI sound is poor), a small node could do only this: PipeWire and WirePlumber, a session bus (TESTED need), one `rtp-source` per node with fixed ports, the sound card, and the same small control API as every other node (section 4) so hubd can set volume and mute through `wpctl`. Each `rtp-source` follows its own sender's clock and a DLL holds the latency (SOURCE above); twelve of them into one sound card: not tested (UNKNOWN). The mixer would then live on the audio node, and the hub would not need PipeWire. The cost is one more machine that is a single point of failure for sound. I recommend **starting with the hub as the audio node** (HUB-OS.md already says "all audio plays through the hub") and keeping the control code location-independent.

---

## 4. The communication layer (beyond the display)

### 4.1 What the hub needs from each node

| Need | Is it in the display protocol? | So |
|---|---|---|
| **Liveness** ("ready" means the session server accepts connections, `HUB-OS.md`) | A TCP connect to the VNC or RDP port (what hubd does today). Whether a bare connect-and-close disturbs a real session is already on the unverified list; I did not test it. | keep the port check; add the status call |
| **Status** (image version, boot id, uptime, is the GUI running, how many viewers are connected, update pending) | No | node helper |
| **Session start and stop** (start the node's GUI and the display server; stop it) | No | node helper (`HUB-OS.md` already says "the node helper (status, start/stop session servers)") |
| **Version of the control API** | No | node helper |
| **Clipboard** | VNC: yes, text (2.2, half shown); RDP: yes; Moonlight: no | If the chosen protocol has it, nothing more. If it turns out not to work hub to node (2.2), the **clipboard bridge** in `HUB-OS.md` ("design discussion first") would be this API's job: `GET` and `PUT` a text. |
| **Audio control** | Not for the in-hub mixer (3.3): hubd controls the hub's own streams. A node never needs volume control, except if the node should be silent at the source: not needed. | none on the node |
| **Power actions** (restart, shut down, with the "this ends the session" warning) | No | node helper (later, `HUB-OS.md`) |
| **A "in recovery" signal** | No | Part 3's agent (4.4) |

### 4.2 What the node helper does (smallest list)

1. Answer `status` (JSON).
2. Start and stop the session: the display server (wayvnc or the RDP server), the audio sender, the node's GUI, in that order, with the retry the sound sender needs (3.2).
3. Later: restart and shut down.
4. Nothing else. It must not read or write files for the hub, run commands the hub names, or open a shell. **A fixed list of actions**, each implemented in the helper.

### 4.3 Smallest honest design

**A small HTTP and JSON API, hubd as the only client.** This is not a web dashboard and not a new streaming protocol: four or five calls, no page, no video.

- Calls (names are a sketch): `GET /v1/status`, `POST /v1/session/start`, `POST /v1/session/stop`, later `POST /v1/power/restart`, `POST /v1/power/shutdown`.
- **Versioning rule (proposal):** the path starts with `/v1`. Within one major version, a change may only **add** fields or calls; nobody may remove or rename one or change its meaning. A reader **ignores fields it does not know**. `status` carries `api: 1` and `min_hub: 1`. The hub speaks to a node whose major version it knows and shows "needs newer hub" for one it does not; a node refuses nothing it knows. Two majors can be served by one node at the same time during an upgrade (`/v1` and `/v2`) so the hub and the nodes can be updated in any order, which is the opposite of the lock-step the owner wants to avoid. (This is a rule I propose; no implementation exists to test it.)
- **Authentication (proposal): the same scheme as Part 3's recovery agent** ([docs/proposals/recovery-and-out-of-band.md](recovery-and-out-of-band.md) once merged; read on 2026-10-04 on the branch `recovery-oob-proposal`): the hub fetches a one-time number, signs the method, path, body hash and number with its **management private key** using `signify`, and sends the signature; the node checks it against a keyring of **public** keys. One cluster-wide key means **no per-node pairing**. The private half lives only on the hub. I did not build or test this; Part 3 tested the scheme on loopback.
- **Encryption:** signed requests give authenticity but not secrecy (Part 3 says the same). Nothing secret should travel in this API. Whether to add TLS: **question 6**.

**Where credentials would live (input for the secrets design; nothing is designed):**

| Secret | Lives | Notes |
|---|---|---|
| Hub's management **private** key | hub only, in its per-machine config area, never in git, not in the NAS backup copy (`HUB-OS.md`: secrets do not get it) | Part 3 reaches the same place |
| Management **public** keyring | every node's image or config (public, can be in the image) | rotation like the update keyring, not designed |
| VNC or RDP password, or TLS key | per node, in the node's per-machine config, **and** the hub needs the password to connect (hubd would have to pass it to the viewer without a command line): **a design problem** | the viewer reads a password from a prompt or a file (`remote-viewer` connection file, `docs/viewers-research.md`). Passing it safely is **UNKNOWN** and is the hard part of the secrets design. |
| Display-protocol certificate | node (its private key) and hub (the CA it must trust, 2.7) | |

### 4.4 One API or two (relation to Part 3's recovery agent)?

My answer: **one API, two programs, the same port.** A machine is either in recovery or running its normal system, never both, so both can listen on the same port and answer `GET /v1/status` with the same shape and a field `state` (`"recovery"` or `"running"`). The recovery agent implements the part of the API that makes sense there (status, install, clear failures, logs); the node helper implements the rest. hubd then has one client, one signing routine and one keyring format. Why not literally one program: the recovery kernel is a separate small image and has to stay tiny (Part 3 measured a 5.5 MiB Go agent against a 6.3 MiB kernel); the helper can be bigger. Part 3 is being written in parallel and is not merged; **question 7** asks you to confirm before either side fixes names.

---

## 5. Security: authentication and encryption on a private network where every machine stays logged in

The point: every node is **always logged in**, so whoever can open the display protocol has the full desktop with no further login. The display protocol's own authentication is the only gate, and the network is the second gate.

| Candidate | What it offers | Label |
|---|---|---|
| **wayvnc** | By default with no config: **no authentication, no encryption**. TESTED: `python3 rfb-security-types.py 5901` printed `security types offered: [1]` (type 1 is "None"). With a config file (`wayvnc-auth.example.conf`: `enable_auth=true`, user name, password, a TLS key and certificate, an RSA key): `security types offered: [19, 129, 5]` (TESTED). The README lists VeNCrypt (TLS with X.509), RSA-AES ("a secure authentication and encryption that's resilient to eavesdropping and MITM"), PAM, and a legacy DES method that "provides **no encryption**" (summarised, [README](https://raw.githubusercontent.com/any1/wayvnc/master/README.md)). I believe 19 is VeNCrypt and 5 is RA2 (RealVNC's RSA-AES); the number 129 I did not look up (UNKNOWN). **Which of these `remote-viewer` can use: UNKNOWN**, except that it rejected a self-signed certificate on the Weston VNC test (2.7). | TESTED, SOURCE |
| **QEMU VNC** | no auth (local socket only), a password of **at most 8 characters** (weak; not allowed in FIPS mode), TLS with X.509 through VeNCrypt (with or without checking the client's certificate), SASL (only GSSAPI "acceptable ... by modern standards"), and combinations ([QEMU VNC security](https://www.qemu.org/docs/master/system/vnc-security.html), summarised). | SOURCE |
| **Weston RDP** | **TLS only, and no password check.** TESTED: the client first asked `Username:`; after `/u:x /p:x` (no such account) it was accepted; the server has no option to name an account or password (`weston --backend=rdp --help` lists only `--rdp-tls-cert`, `--rdp-tls-key`, `--rdp4-key`). The man page says "The RDP backend supports RDP security or TLS" and that the old "RDP security ... is known to be insecure" ([weston-rdp(7)](https://man.archlinux.org/man/weston-rdp.7.en), summarised). My test used `/cert:ignore`, which turns off the client's check of the server (the client printed `[DANGER] Certificate not checked`). So **anyone who can reach the port gets the desktop**. | TESTED, SOURCE |
| **xrdp** | username and password through the system's login (BELIEVED, not read here), TLS | not examined |
| **KRdp** | one username and password combination, TLS certificate ([README](https://invent.kde.org/plasma/krdp/-/raw/master/README.md), summarised) | SOURCE |
| **Sunshine + Moonlight** | pairing with a PIN, then certificates, and an encrypted stream (BELIEVED; the pairing and credentials are in `docs/viewers-research.md` sections 1 and 2) | BELIEVED |
| **SPICE** | optional password (in libvirt it is clear text in the guest's XML, `docs/viewers-research.md`) and optional TLS | SOURCE (earlier doc) |

On a private network the realistic options for the display protocol are: **(1) none, with the network as the only protection** (any compromised machine, or a guest on the VM host, can take over any node's desktop); **(2) one password for the whole cluster** in each node's config (and the hub holds it); **(3) TLS plus password, which needs a private certificate authority** that `remote-viewer` trusts. HUB-OS.md already has the open note "incoming connections from the internet must stay blocked" (still waiting for the owner's confirmation). This is **question 5**.

---

## 6. Node side

### 6.1 Which compositor each candidate needs on the node

| Candidate | Needs on the node | Label |
|---|---|---|
| **wayvnc** | A **wlroots-based** compositor: its README says "wlroots-based Wayland compositors (Gnome, KDE and Weston are **not** supported)" (summarised). From `strings`/behaviour in my runs it uses the screen-capture protocol, a **virtual keyboard**, a **virtual pointer** and `wlr-data-control` (it ran on sway 1.9 and created `wlr_virtual_pointer_v1` and `wlr_virtual_keyboard_v1` devices, TESTED: `swaymsg -t get_inputs`). **driftwm has no virtual-pointer protocol** (`docs/driftwm-findings.md`: `grep ... src` prints nothing), so wayvnc could not move the mouse under driftwm (BELIEVED, not run). This only matters if a node ran driftwm; the hub does, nodes are separate and run what we pick. | TESTED, SOURCE, BELIEVED |
| **Weston RDP / VNC backend** | **Weston itself**, as the compositor (the backend replaces the screen). Apps connect as Wayland clients. TESTED. | TESTED |
| **QEMU VNC, SPICE** | a virtual machine; no compositor on the node's own display | |
| **Sunshine** | KMS/DRM capture, Wayland (wlroots), X11, XDG Desktop Portal or KWin screencast; "can operate on headless systems" ([README](https://raw.githubusercontent.com/LizardByte/Sunshine/master/README.md), summarised) | SOURCE |

### 6.2 Can a bespoke GUI run alone in a kiosk-style compositor?

- **Weston's kiosk shell.** TESTED: `weston --backend=rdp --shell=kiosk ...` started and ran foot as a client with the RDP client connected (2.1). The documentation says it is "a simple shell targeted at single-app/kiosk use cases" that "makes all top-level application windows fullscreen" and can place apps per output with `app-ids=` ([kiosk-shell docs](https://wayland.pages.freedesktop.org/weston/toc/kiosk-shell.html), summarised). I did not inspect whether the test window was fullscreen.
- **A wlroots kiosk compositor for wayvnc.** TESTED: plain sway with one app works (app was foot) and wayvnc captured it. Other candidates in the Ubuntu archive: `cage` 0.1.5 and `labwc` 0.7.1 (TESTED only `apt-cache policy`; neither was run). Cage's description as a single-app kiosk compositor: BELIEVED (not read here).
- **Weston has no seat until a client connects.** TESTED: starting `foot` on Weston's RDP socket before any client had connected failed with `no seats available (wl_seat interface too old?)`; after the RDP client connected it worked. The node's GUI therefore has to be started **after** the display client connects, or be able to wait, with Weston RDP. Weston's own man page says "The RDP backend is multi-seat aware, so if two clients connect ... they will get their own seat." (summarised). With wayvnc on sway this problem did not appear.

A node's bespoke GUI needs a Wayland toolkit that works on these compositors (GTK, Qt, the app's own); that is a question for each GUI, not for the remote display.

### 6.3 GPU-rendered apps (Blender, ParaView on the AI box)

SOURCE only, no GPU here, nothing tested:
- Any of the compositors above can run on a GPU through the node's DRM render device (BELIEVED); whether a **headless** compositor offers OpenGL to a client like Blender with an NVIDIA driver: **UNKNOWN**.
- wayvnc has an option `--gpu` ("Enable features that need GPU", TESTED `wayvnc --help`). What it changes (H.264 encoding, dmabuf capture) I did not test: **UNKNOWN**. Frames otherwise go through the CPU. At 49 Mbit/s for a plain 640 x 480 test video (2.6) a full-screen 4K viewport in Blender would be far more; whether the node's CPU copes: **UNKNOWN**.
- Weston's pixman renderer (the one I ran) is software; the GL renderer on a GPU was not tested.
- **Sunshine** is the one candidate built for exactly this: GPU encode on AMD, Intel and NVIDIA, "can operate on headless systems" (README, summarised). The price: pairing, no clipboard, GPL, one project.

**What it means for the AI box.** The recommendation below uses VNC for ordinary nodes, but the AI box is the one where I cannot support it with evidence. The honest options are: (a) VNC with wayvnc, measure in December on the real GPU; (b) Sunshine and Moonlight for the AI box only (the project already lists "AI / GPU box: Moonlight window (Sunshine on the node)" in `HUB-OS.md`) and accept pairing and no clipboard there; (c) both on the AI box, switchable per session. **Question 1.**

---

## 7. What would change in the repo for each option (nothing is changed now)

Today `examples/viewers.real.example.toml` has viewers `ssh`, `spice`, `vnc`, `moonlight` and the `[default_ports]` table (`moonlight = 47989`, `ssh = 22`, `files = 445`); `vnc` and `spice` have **no** default port because a VM guest's port is per guest. The inventory has `open` entries, `port`, `session`. HUB-OS.md's Health section lists Sunshine, SSH, SPICE/VNC.

| Change | wayvnc (recommended) | Weston RDP | Sunshine (as now) | SPICE |
|---|---|---|---|---|
| `viewers.toml` viewer entry | The existing `vnc` entry already fits: `remote-viewer --name={app_id} --title={title} -- vnc://{address}:{port}`. Add a second id (for example `vnc-node`) only if nodes and guests need different commands. A password cannot be passed on the command line (4.3). | A new entry, for example `rdp`: `wlfreerdp3 /v:{address}:{port} /title:{title} /wm-class:{app_id} /cert:... +clipboard` (TESTED shape, 2.1). The user name and password problem is the same. Also decide the client: `wlfreerdp3` is deprecated (2.3). | none | exists |
| `[default_ports]` | `vnc = 5900` for nodes (wayvnc's default, TESTED `wayvnc --help`: "Default: 5900"). The current table has no `vnc` on purpose for guests; nodes and guests would then share a port name, so a second name (`vnc-node`) is cleaner. | `rdp = 3389` (the RDP default; Weston's default port I did not check; I set 3390) | `moonlight = 47989` (already) | none (per guest) |
| Pairing | none | none | by hand per node (already in HUB-OS.md) | none |
| Title matching | Not needed: `--name` sets the app-id (`sets_name = true`, TESTED 2.1). Keep `title_match` for Moonlight only. | Not needed: `/wm-class` (TESTED) | `title_match = "{id} - Moonlight"` (already) | not needed |
| Inventory | `open = ["vnc"]`, `port`; the inventory's `session` field stays Sunshine only | `open = ["rdp"]` | `session` | |
| New control API | the node helper and the sound mixer (sections 3 and 4), which are the same for every option except Sunshine | same | Sunshine carries its own sound, so the per-node mixer would have to work on the **Moonlight window's** stream (found by process id, 3.3) | |
| **Unverified list in HUB-OS.md** | add: wayvnc and `remote-viewer` clipboard hub to node; certificate trust in `remote-viewer`; PipeWire RTP on the real network and sound card; WirePlumber with a session bus on the hub image; real viewers' sound streams (name and process id); GPU apps over wayvnc | add: Weston RDP crash on reconnect; no password check; `wlfreerdp3` deprecation; audio in newer Weston | existing items | |
| Out-of-repo requirements | wayvnc and PipeWire (and WirePlumber, a session bus) in the node images; a wlroots compositor on nodes | Weston, FreeRDP libraries | existing | |
| HUB-OS.md text | "Sound: all audio plays through the hub (carried by the viewers)" would need a rewrite: sound would **not** be carried by the viewers but by PipeWire RTP. Also "Clipboard and audio support in each viewer" on the unverified list. (The owner edits HUB-OS.md; I did not.) | | | |

---

## 8. Recommendation and questions

**Recommendation (a proposal, not a decision):**
1. **Display:** VNC (RFB) with **wayvnc** on each ordinary node, in a wlroots compositor that shows only that node's GUI; viewer: `remote-viewer` for now, with the TigerVNC viewer tested as a replacement because `remote-viewer` has not been released since 2021. Why: open standard, several independent servers and clients (wayvnc, TigerVNC, QEMU, Weston; remote-viewer, TigerVNC, Remmina), no pairing, no version lock-step, an idle desktop costs nothing on the wire, wayvnc is actively released.
2. **Sound:** PipeWire RTP from each node to the hub, one named stream per node, mixed by hubd with `wpctl`, remembered by hubd (not by WirePlumber). Raw audio, default 100 ms or more.
3. **Control:** one small signed HTTP and JSON API, the same shape as Part 3's recovery agent, same port.
4. **Keep Sunshine and Moonlight** for the gaming box (decided), and decide the AI box after a December measurement.
5. **Do not use SPICE** for anything new; keep it only for VM guests if virt-viewer's guests need it, or move guests to VNC as Red Hat did.
6. **RDP** stays the fallback if the clipboard hub to node cannot be made to work over VNC, but not with Weston's RDP as it is (no password check, one crash, no sound code), and not with `wlfreerdp3`.

**What this proposal does not prove:** clipboard hub to node for either protocol; resize requested by the viewer; anything on a GPU; anything on a real network or real sound hardware; the real viewers' sound streams; certificates in `remote-viewer`; driftwm as the hub compositor with any of these viewers (not run this round).

**Questions for the owner (answer before anything is built):**
1. **AI box:** is VNC (wayvnc, CPU path) acceptable there until December measurements, or must it be Sunshine and Moonlight from the start (with no clipboard back)?
2. **Clipboard:** is text only enough (no images or files)? Is "hub to node" required, or is "node to hub" the important direction? (Only node to hub was seen to work.)
3. **Viewer risk:** `remote-viewer` (last release 2021-11-18) is what the repo uses for VNC and SPICE. May I test the TigerVNC viewer as its replacement, as a separate task?
4. **Audio location:** the hub plays the sound (as `HUB-OS.md` says), not a dedicated audio node. Confirm. And is raw audio on the wire acceptable (about 1.5 Mbit/s per node)?
5. **Authentication on the display protocol:** none (network is the only gate), one cluster password, or TLS plus password with a private certificate authority? (Every node is always logged in.)
6. **Control API:** is a signed, unencrypted JSON API on the private network acceptable (nothing secret in it), or must it be TLS?
7. **One API with Part 3's recovery agent** (same port, `state` field)? Please confirm with Part 3's author before names are fixed.
8. **Name of the node helper's port** and whether it may run as a normal user (it must start the GUI and later restart the machine, so it may need more rights).
9. **Where mixer settings live** on the hub (the record file is in memory and is lost at reboot): the config area, the data area, with or without the NAS backup copy?
10. **HUB-OS.md wording:** "Sound ... (carried by the viewers)" would no longer be true under this proposal. May I draft the change for you to approve?
11. **Weston:** Weston 16 (2026-07-14) was not run. Is it worth one more round on Weston RDP with the newest version, given the other findings, or should RDP stay out of scope?
12. **Gaming box:** none of this touches it (its own monitors, decided). Confirm.

---

## 9. Sources read (all 2026-10-04) and what could not be reached

- Reached: RFC 6143; MS-RDPBCGR page; PipeWire module docs (rtp-sink, rtp-source, rtp-session, rtp-sap); the Red Hat bug 1946939 and RHEL 9 virtualization considerations page; the RHEL 10 removed-features page; QEMU download, invocation and VNC security pages; xrdp, pulseaudio-module-xrdp, KRdp, TigerVNC, Sunshine, Moonlight, wayvnc READMEs; GitHub release feeds (`releases.atom`) for wayvnc, FreeRDP, Sunshine, Moonlight, TigerVNC, xrdp, Waybar; the virt-viewer tags feed; wayland.freedesktop.org releases; Weston kiosk-shell page; Waybar's wireplumber man page; the GNOME Remote Desktop NEWS.
- **Not reachable:** the GitHub REST API (HTTP 403 from the fetch tool; the GitHub tool of this session may only read the HubOS repository); the freedesktop GitLab (spice, spice-gtk tag feeds: "Access Denied" anti-bot page); `https://www.spice-space.org/news.html` (404); `docs.pipewire.org/page_module_sap.html` (404; the right name is `page_module_rtp_sap.html`, which worked); the Weston `running-weston` page had no RDP detail; the PipeWire AES67 man page (404). Anything those pages would have said is **UNKNOWN** above.
- Summaries by the fetch tool: see the note under "Labels". Re-read before relying on a number.
