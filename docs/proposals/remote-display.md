# PROPOSAL: how each node's display, sound and control reach the hub

> **OWNER DECISION (2026-10-06): no passwords and no encryption on the display connections.** Every password, TLS, RSA-AES or VeNCrypt setup described below (sections 4.4, 5 and round 2 R2.3) is "none used": the hub's viewer will use RFB security type None, and every node's wayvnc listens on the cluster address only. The text below stays as the test record of what is possible. The hub's requests to nodes (the control API) are also not signed. The questions "Authentication on the display protocol" and "Control API" are answered: none, and unsigned and unencrypted.

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
5. **Authentication on the display protocol:** ANSWERED (owner, 2026-10-06): none, no password and no encryption; wayvnc listens on the cluster address only.
6. **Control API:** ANSWERED (owner, 2026-10-06): an unsigned, unencrypted JSON API on the private network; no TLS.
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

---

# Round 2 (written 2026-10-04 by a helper agent; unverified until the owner's lead has read the sources)

**What this round was.** Four follow-up jobs: (1) why clipboard text from the hub did not reach the node, (2) the TigerVNC viewer as a replacement for `remote-viewer`, (3) the smallest wayvnc login and encryption set-up that needs no certificate authority, (4) a test under the real hub compositor, driftwm, with hubd's window matching. Same four labels as above: **TESTED**, **SOURCE** (link and date read), **BELIEVED**, **UNKNOWN**. Every SOURCE below was read by the helper itself on 2026-10-04 from the primary file (raw source file, git history, Launchpad's JSON API); the few pages that went through the summarising fetch tool or a search engine are marked "(summarised)". Round 1's text above is left as it was; section R2.7 lists what it changes.

**What was and was not the set-up.** Cloud container, no GPU, no real network, loopback only, software rendering. Nothing was installed: every program was downloaded as a `.deb` from the Ubuntu 24.04 archive (or built from source) and unpacked under `/tmp/r3-*`, then deleted (commands and helper scripts: `tools/image/experiments/remote-display/round2/`). Versions: sway 1.9 (wlroots 0.17), wayvnc 0.7.2 with neatvnc 0.7.1 (Ubuntu 24.04), **and** wayvnc v0.10.2 with neatvnc v1.0.3 built from the release tags, `remote-viewer` 11.0 with gtk-vnc 1.3.1 (Ubuntu 24.04), TigerVNC viewer 1.13.1 (Ubuntu 24.04), driftwm at the pinned commit `352333a8` built as in `docs/driftwm-findings.md` section 0 (nested, `winit` backend inside Xvfb), xwayland-satellite built from git commit `b5690b56` (2026-09-30), Xwayland 23.2.6. **The "node" was a headless sway; the "hub" was either sway on the wlroots X11 backend (it has a real keyboard, which round 1's headless hub did not) or driftwm. Nothing was run on a real screen, real GPU or real network.**

---

## R2.1 Clipboard from the hub to a node

### Short answer

**Why it failed in round 1: `remote-viewer` (virt-viewer 11.0 with gtk-vnc 1.3.1) never sends the hub's clipboard to the server at all. The server and the node compositor were fine.** The TigerVNC viewer does send it, in both directions, **but only while its window has keyboard focus**, and in the set-up that driftwm needs (an X11 program through xwayland-satellite) a copy made while the window was not focused did not arrive later. Non-ASCII text is garbled with wayvnc 0.7.2; it works with wayvnc 0.10.2.

### What was checked, in order

| # | Question | Result | Label |
|---|---|---|---|
| 1 | Does the node compositor offer the data-control protocol that wayvnc needs? | sway 1.9: `zwlr_data_control_manager_v1` version 2, and no `ext_data_control`: `wayland-info` on the node. driftwm (hub side): both `zwlr_data_control_manager_v1` v2 and `ext_data_control_manager_v1` v1. | TESTED (`wayland-info \| grep data_control`) |
| 2 | Does wayvnc use it? | wayvnc 0.7.2 contains `zwlr_data_control_manager_v1`, `set_selection`, `set_primary_selection` (`strings`). Source: wayvnc master (`src/main.c` lines 719-721) handles a `wlr_manager` **or** an `ext_manager` per client. The git history says ext-data-control support came in with commit `ec86b48` (2026-02-19), first tagged **v0.10.0** (2026-04-27); the first clipboard code is `3ee9aac` (2020-09-16). | TESTED (strings); SOURCE: [wayvnc main.c](https://raw.githubusercontent.com/any1/wayvnc/master/src/main.c) and `git log` of a clone of [any1/wayvnc](https://github.com/any1/wayvnc), read 2026-10-04 |
| 3 | Does the server send the node's text to a client? | Yes. A tiny RFB client (`round2/rfb-listen.py`) received `ServerCutText b'listen-test-1'` after `wl-copy` on the node. (It arrived twice; why twice: UNKNOWN.) | TESTED |
| 4 | Does the server take text from a client? | Yes (round 1, `rfb-cut-text.py`), and again with TigerVNC: `ClientCutText b'hub-X-2'` in the spy log, then `wl-paste` on the node printed `hub-X-2`. | TESTED |
| 5 | Does `remote-viewer` ever send a `ClientCutText`? | **No.** A relay (`round2/rfb-spy.py`) logged the viewer's whole conversation: `SetPixelFormat`, `SetEncodings` (19 encodings, **no** extended-clipboard pseudo-encoding), pointer and key messages, and **no clipboard message, in any of several tries**, with the hub window focused, with a real keyboard, after typing a key, with `wl-copy` and `wl-copy --primary`. The program does not even import the function: `nm -D /usr/bin/remote-viewer \| grep vnc_display` lists 21 `vnc_display_*` functions and **not** `vnc_display_client_cut_text`. | TESTED |
| 6 | Is that a bug or a missing feature? | A missing feature. virt-viewer 11.0 only listens for the server's text: `src/virt-viewer-session-vnc.c` has `virt_viewer_session_vnc_cut_text` (server to hub) and no call that sends text. Same in virt-viewer's master today. gtk-vnc 1.3.1 and **1.5.0 (2025-02-07, the newest release)** have no code that sends the local clipboard by itself; gtk-vnc **master** has it since **2025-10-12** (commits `e334dd43` extended clipboard, `348452c1` client-to-server notification, `6c8d8691` flush on focus-in) and it uses the **PRIMARY** selection (the selected text), not the normal clipboard (`send_primary_selection`, `src/vncdisplay.c`). So even a future gtk-vnc would need the text to be *selected*, and virt-viewer would need a release built on it. Ubuntu's newest series that I could see ships gtk-vnc 1.5.0 and virt-viewer 11.0-4 (see R2.2), so none of them has it. | SOURCE: [virt-viewer-session-vnc.c v11.0](https://gitlab.com/virt-viewer/virt-viewer/-/raw/v11.0/src/virt-viewer-session-vnc.c) and [master](https://gitlab.com/virt-viewer/virt-viewer/-/raw/master/src/virt-viewer-session-vnc.c); [gtk-vnc vncdisplay.c v1.3.1](https://gitlab.gnome.org/GNOME/gtk-vnc/-/raw/v1.3.1/src/vncdisplay.c), [v1.5.0](https://gitlab.gnome.org/GNOME/gtk-vnc/-/raw/v1.5.0/src/vncdisplay.c), [master](https://gitlab.gnome.org/GNOME/gtk-vnc/-/raw/master/src/vncdisplay.c); GNOME GitLab commit API for `src/vncdisplay.c`; tags API; all read 2026-10-04 |
| 7 | And node to hub with `remote-viewer`? | Works: `rv-node-text-A` on the node, `wl-paste` on the hub printed `rv-node-text-A` (hub = sway, and again under driftwm). In round 1 one run seemed to fail; in this round the same direction failed once after I had just copied hub text, and worked in every other run; the cause of that one miss: UNKNOWN. | TESTED |
| 8 | TigerVNC viewer 1.13.1 | It asks the server for the extended-clipboard encoding (`SetEncodings` shows `ExtendedClipboard`; wayvnc 0.7.2 / neatvnc 0.7.1 does not offer it, so plain cut text is used). Hub to node: the spy log shows `ClientCutText b'hub-X-2'`. Node to hub: `xclip -o` on the hub's X display printed the node's text. Both directions also work through Xwayland under sway, and through xwayland-satellite under **driftwm** (`wl-paste` on the hub printed `dw-node-1`; the node printed `dw-hub-1`). | TESTED |
| 9 | Focus | **The TigerVNC viewer only exchanges clipboard while its window is focused.** The viewer's own debug log says `Got notification of new clipboard on server whilst not focused, will request data later` and, when focus returns, `Focus regained after remote clipboard change, requesting data`. The source says the same: `Viewport::handleClipboardAnnounce` stores a pending flag when `!hasFocus()`, `flushPendingClipboard()` runs on `FL_FOCUS`. In the first tests the text never moved because the window had no focus. | TESTED; SOURCE: [Viewport.cxx v1.13.1](https://raw.githubusercontent.com/TigerVNC/tigervnc/v1.13.1/vncviewer/Viewport.cxx), read 2026-10-04 |
| 10 | Hub to node when the window was **not** focused at the moment of copying (driftwm, xwayland-satellite, 2 runs) | **Did not arrive**, not even after the window was focused again. After the second run the X side of the bridge showed the new text (`xclip -o` printed `hubside-5`) but the viewer had not been told, and it sent the previous text instead. Node to hub with the window unfocused did arrive when focus came back. The cause (the viewer, xwayland-satellite, or Xwayland) is **UNKNOWN**. Under plain X11 (Xvfb, no bridge) I did not test the unfocused copy. | TESTED (2 runs); cause UNKNOWN |
| 11 | Non-ASCII text, wayvnc 0.7.2 + neatvnc 0.7.1 | Garbled both ways, even "é". Node to hub: `café ✓ 日本` arrived as `cafÃ©...` (the server sends UTF-8 bytes as if they were Latin-1). Hub to node: `naïve ✓ 日` arrived as `na\357ve ??` (invalid UTF-8 and question marks). Cause: plain cut text in RFB is Latin-1 only (round 1, RFC 6143). `remote-viewer` is the same on the receiving side: virt-viewer converts the received text from `iso8859-1` to UTF-8 (`virt-viewer-app.c` line 1658, v11.0) and, with wayvnc 0.10.2 + neatvnc 1.0.3, `café ✓` still arrived as `cafÃ© â`. | TESTED; SOURCE: [virt-viewer-app.c v11.0](https://gitlab.com/virt-viewer/virt-viewer/-/raw/v11.0/src/virt-viewer-app.c) |
| 12 | Non-ASCII text, wayvnc **v0.10.2** + neatvnc **v1.0.3** (built from the tags) + TigerVNC 1.13.1 | **Works both ways:** `café ✓ 日本` and `naïve ✓ 日` arrived unchanged. neatvnc added the extended clipboard (UTF-8) in commit `e27d1a6` (2024-08-22), first tagged v0.9.0. A later commit `67c722d` (2026-06-29, "Convert clipboard text to and from UTF-8" for plain clients) is on neatvnc's master and in **no** tag yet. | TESTED; SOURCE: `git log`/`git tag --contains` in a clone of [any1/neatvnc](https://github.com/any1/neatvnc), 2026-10-04 |

### Exact settings that worked (hub to node and node to hub)

- Node: wayvnc (any version tested) on a wlroots compositor with data-control; **no clipboard option is needed**. wayvnc has none: its man page and `--help` list none. (The only related rule in its source: `--disable-input` also turns clipboard off, commit `5ed57b9`, 2022-02-09.)
- Hub: **TigerVNC viewer** (`xtigervncviewer` in Ubuntu), default settings. Its clipboard parameters are all on by default: `SendClipboard=1`, `AcceptClipboard=1`, `SendPrimary=1`, `SetPrimary=1` (`xtigervncviewer -h`). The window **must have keyboard focus**.
- With wayvnc 0.7.2: only plain ASCII text. For other letters use **wayvnc v0.10.x** (neatvnc 0.9 or newer).
- Not available for any version: hub to node with `remote-viewer`.

### Things round 1 guessed that are now explained

- "pasted text lives only while the viewer stays connected" (round 1, 2.2): not re-tested. UNKNOWN.
- The hub stand-in in round 1 had no keyboard; in this round the stand-in had one. That did not change `remote-viewer`'s behaviour, which confirms the viewer, not the stand-in, was the cause.

---

## R2.2 TigerVNC viewer as the replacement for `remote-viewer`

| Item | Finding | Label |
|---|---|---|
| **What it is** | An X11 program (FLTK 1.3 toolkit). Ubuntu's package `tigervnc-viewer` installs one program, `/usr/bin/xtigervncviewer` (the plain name `vncviewer` is a Debian alternatives link, not unpacked here). Licence GPL-2+ (Ubuntu's copyright file). Depends include `libfltk1.3`, `libx11-6`, `libxrandr2`, `libgnutls30t64`, `libavcodec60`, `libswscale7`; no systemd library. 1.1 MB installed. | TESTED (`dpkg-deb -I`, `ldd`) |
| **Where in the Ubuntu archive** | Source package `tigervnc`, component **universe**: noble (24.04) 1.13.1+dfsg-2build2; plucky 1.14.1; questing 1.15.0+dfsg-2; resolute 1.15.0+dfsg-2build1; stonking 1.15.0+dfsg-2.1 (2026-07-11). Upstream's newest tag is **v1.16.2**, so Ubuntu is one minor release behind. For comparison: wayvnc noble 0.7.2, plucky/questing/resolute 0.9.1, stonking 0.10.1 (universe); neatvnc noble 0.7.1, resolute 0.9.1, stonking 1.0.1; virt-viewer noble 11.0-3build2, stonking 11.0-4; gtk-vnc noble 1.3.1, resolute/stonking 1.5.0. | SOURCE: Launchpad API, `https://api.launchpad.net/devel/ubuntu/+archive/primary?ws.op=getPublishedSources&source_name=<name>&status=Published&exact_match=true`, read 2026-10-04; upstream tags by `git ls-remote --tags` on [TigerVNC/tigervnc](https://github.com/TigerVNC/tigervnc), [any1/wayvnc](https://github.com/any1/wayvnc), [any1/neatvnc](https://github.com/any1/neatvnc), 2026-10-04 |
| **Wayland status** | **No native Wayland.** Upstream master's viewer opens the X display directly (`fl_open_display()` and `XkbSetDetectableAutoRepeat(fl_display, ...)`, `vncviewer/vncviewer.cxx` lines 742-743), and 1.16 added only a Wayland **server** (`w0vncserver`, which is for sharing a Wayland desktop; not what we need). On the hub it runs through Xwayland. | SOURCE: [vncviewer.cxx master](https://raw.githubusercontent.com/TigerVNC/tigervnc/master/vncviewer/vncviewer.cxx), 2026-10-04. The w0vncserver / "native viewer is still X11" wording of the release notes and news pages: (summarised) search result and fetch result; I did not read the release notes themselves, and the fetch tool's dates for them (2025) disagree with the tags, so treat those as unreliable |
| **Under driftwm** | driftwm has no X11 support of its own. Its log says: `xwayland-satellite not found ... X11 apps disabled`. With **xwayland-satellite** on `PATH`, driftwm started it (`spawned xwayland-satellite pid=... on :1`) and the viewer ran as a window. xwayland-satellite is **not in the Ubuntu archive and not on crates.io** (`cargo install xwayland-satellite` said `could not find ... in registry crates-io`; the driftwm findings doc assumes it can be installed that way, which is wrong today). I built it from git (`Supreeeme/xwayland-satellite`, commit `b5690b56`, 2026-09-30): 1 min 27 s with 2 jobs, needs `libxcb-cursor`. It starts `Xwayland` (Ubuntu package `xwayland` 23.2.6; its dependency list has no systemd library) and **Xwayland calls `/usr/bin/xkbcomp` by a fixed path**, so the image would also need `x11-xkb-utils`. Net cost of choosing TigerVNC on the hub: three extra things in the hub image (Xwayland, xkbcomp, a Rust-built xwayland-satellite). | TESTED (build, run); the claim about the findings doc: SOURCE (`docs/driftwm-findings.md` section 12, last rows, read 2026-10-04) |
| **App-id** | **Cannot be chosen.** `xtigervncviewer -name x -title y` prints the usage text and exits; the option list (`-h`) has no name, class or title option. As an X11 program its window class is `TigerVNC Viewer` (TESTED with `xprop`: `WM_CLASS = "TigerVNC Viewer", "TigerVNC Viewer"`), and under driftwm + xwayland-satellite the Wayland app-id is `TigerVNC Viewer` (TESTED, `driftwm msg state`). Under sway's own Xwayland there is no app-id at all (`app_id=None`, class in `window_properties`). Note: upstream's source calls `Fl_Window::default_xclass("vncviewer")` (line 341 of `vncviewer.cxx` v1.13.1) yet the Ubuntu build showed `TigerVNC Viewer`; why: UNKNOWN. So the class can differ between builds: do not match on it. | TESTED; SOURCE [vncviewer.cxx v1.13.1](https://raw.githubusercontent.com/TigerVNC/tigervnc/v1.13.1/vncviewer/vncviewer.cxx) |
| **Title** | **Set by the server, not by hubd.** The window title is the server's desktop name followed by ` - TigerVNC`. wayvnc 0.7.2 hard-codes the name `WayVNC` (`src/main.c` line 752 of v0.7.2), so every node would be titled `WayVNC - TigerVNC`. wayvnc **v0.10.0 and newer** have `-n/--name` (commit `630ed4a`, 2025-06-06) and `wayvncctl set-desktop-name` (`f4d0518`, 2025-06-10). TESTED with v0.10.2: `wayvnc -n gui-b ...` gave the title `gui-b - TigerVNC`, and **hubd matched it** with `title_match = "{id} - TigerVNC"` (`sets_name = false`), recorded `matched by title`. No `(1)` suffix is added (unlike remote-viewer). | TESTED; SOURCE [wayvnc main.c v0.7.2](https://raw.githubusercontent.com/any1/wayvnc/v0.7.2/src/main.c) and git history, 2026-10-04 |
| **Window matching with hubd under driftwm** | See R2.4. Because the app-id is shared, only title matching can tell two TigerVNC windows apart; that needs each node's desktop name set to its machine id. | TESTED |
| **Error dialogs** | With nothing listening the viewer shows a small window (446 x 154, class `TigerVNC Viewer`, title `TigerVNC Viewer`) with an error and a way to retry. Its title differs from the real window's (`<name> - TigerVNC`), so hubd's `title_match` does **not** take the error dialog for the machine's window (an advantage over `remote-viewer`, whose error dialog carries the chosen app-id). A wrong password: the same kind of dialog (`Authentication failure: Invalid username or password`, title `TigerVNC Viewer`). | TESTED (sway + Xwayland for the refused connection; Xvfb for the wrong password) |
| **Closing the window** | Under driftwm, `driftwm msg close TigerVNC` closed it at once, **no "close the session?" dialog** (unlike `remote-viewer`), the process exited, wayvnc and the node's windows stayed up, hubd showed the machine as not open and a new `hubd open` reconnected. | TESTED |
| **Reconnect** | **No automatic reconnect in this setting.** When wayvnc was killed the viewer printed `End of stream` and exited with status 0, **without a dialog** (2 runs); the manual lists `-ReconnectOnError` ("Give a dialog on connection problems...", default 1) but that dialog did not appear for a dropped connection. When wayvnc was restarted, hubd's `open` made a fresh window. The node side keeps running. | TESTED |
| **Shared or not** | A second non-shared viewer **disconnects the first.** With `remote-viewer` open, starting `xtigervncviewer` (default `Shared=0`) made `remote-viewer` disappear (checked with the process list, 1 run). With `-Shared` both stayed. Good for "never two windows of one node" but a trap if someone opens a second viewer by hand. | TESTED |
| **Resize** | With wayvnc 0.7.2 the viewer's request to resize the node's screen fails (`SetDesktopSize failed: 1` in its log). With wayvnc **0.10.2** it works: the node's headless output followed the viewer window (`1016x753` at connect, `700x475` after `driftwm msg resize 700 500`). The manual's `-RemoteResize` and wayvnc's `-R/--disable-resizing` switch this off. For a node whose screen has a fixed size, switch it off on one side. | TESTED |
| **Credentials without a command line** | The man page: "You can also add `VNC_USERNAME` and `VNC_PASSWORD` to environment variables" (`xtigervncviewer(1)` in the package). TESTED (that is how every login test ran). `-PasswordFile` only covers the old VNC password, not a user name. The environment of a process is readable only by the same user and root (BELIEVED, not tested); the command line is readable by everyone. | TESTED; BELIEVED |
| **Clipboard** | R2.1. | |

---

## R2.3 wayvnc login and encryption without a certificate authority

### The two smallest set-ups (both tested with wayvnc 0.7.2 **and** v0.10.2)

**A. RSA-AES, no files at all (three essential lines: `enable_auth`, `username`, `password`; `address` and `port` are optional).** `round2/wayvnc-rsa-min.conf`:

```
address=127.0.0.1
port=5931
enable_auth=true
username=hubos
password=<the password>
```

Start with `wayvnc -C that.conf`. The server then offers only the security types **129 and 5** (RSA-AES-256 and RSA-AES; `rfb-security-types.py` printed `[129, 5]`), no "None". The man page of 0.7.2 says `enable_auth` "requires also setting certificate_file, private_key_file, username and password"; **that is wrong for the RSA-AES case**: it started without any file and without a warning (TESTED). Where the server's RSA key comes from in that case (made fresh at each start, or something else): UNKNOWN. With `rsa_private_key_file=` set (a key made with `openssl genrsa -traditional`) it offered the same two types. wayvnc's README tells you to make the key with `ssh-keygen -m pem -t rsa -N ""` (SOURCE: [README v0.10.2](https://github.com/any1/wayvnc/tree/v0.10.2), cloned and read 2026-10-04); I used openssl and it worked.

**B. TLS with a self-signed certificate (the same three login lines plus two file lines, and one command to make the certificate).** `round2/wayvnc-tls.conf` adds `private_key_file=` and `certificate_file=`. Make the certificate **with the address (or name) the viewer will use in its subjectAltName**, otherwise the viewer complains about the name:

```
openssl req -x509 -newkey rsa:2048 -nodes -keyout tls.key -out tls.crt -days 3650 \
   -subj "/CN=127.0.0.1" -addext "subjectAltName=IP:127.0.0.1,DNS:node-test"
```

The server then offers `[19, 129, 5]` (19 is VeNCrypt, which carries TLS). TigerVNC chose `VeNCrypt` and the sub-type `X509Plain` (user name and password inside TLS). The README suggests an EC key (`secp384r1`); I tested an RSA 2048 certificate only. No certificate authority is involved: the certificate signs itself, and the viewer is told to trust that one file. Password check works: a wrong password gave `Authentication failure: Invalid username or password`.

### Is it really encrypted?

TESTED, loopback, one run each, with a relay that records every byte (`round2/rfb-dump.py`) and a clipboard text sent from the TigerVNC viewer:

| Link | Text found in the recorded bytes |
|---|---|
| no login (`security type 1`) | **6 times** (and the whole picture data in the clear: 249 KB) |
| TLS (VeNCrypt) | 0 times |
| RSA-AES | 0 times |

The text still reached the node through both encrypted links (`wl-paste` on the node printed it). So on the wire a password-less wayvnc shows every clipboard text and every key you type; an authenticated one does not. (I did not try to attack the encryption; only "the text is not visible" was checked.)

### What each viewer does with each set-up

| | TigerVNC viewer 1.13.1 | `remote-viewer` 11.0 (gtk-vnc 1.3.1) |
|---|---|---|
| **RSA-AES** | Works with the user name and password from `VNC_USERNAME`/`VNC_PASSWORD`. **But it opens a dialog "Server key fingerprint ... press Yes" at every connection** and remembers nothing (more than six connections, with a fresh key made at each server start and with a fixed key file: the dialog came every time). An unattended open is therefore not possible; a person (or a script pressing Return) must confirm each time. `RA2` is not in `remote-viewer`. | **Not supported.** `remote-viewer` showed an error dialog "Unable to connect to the graphic server" (screenshot read). Its library names RA2 in its constants but the viewer failed (TESTED; the reason is not logged). |
| **TLS, self-signed** | Without any setting: two dialogs on the first connection ("Certificate hostname mismatch" if the name does not match, then "Unknown certificate issuer"); after you accept, the certificate is stored in `~/.vnc/x509_known_hosts` and the **next connection has no dialog** (TESTED with a certificate whose name matched). With the certificate given as the trusted authority, `-X509CA /path/tls.crt`, **no dialog at all**, even on a fresh home folder (TESTED). A name mismatch asks every time. | **Works**, in one try: certificate placed as the trusted authority, login from a connection file (below). It trusts files only in fixed places: `<system config dir>/pki/CA/cacert.pem` or `<home>/.pki/CA/cacert.pem`, where `<home>` is the home folder from the **password file entry of the current user**, not `$HOME` (SOURCE: `src/vncconnection.c` v1.3.1, `vnc_connection_set_credential_x509`, read 2026-10-04; and TESTED: with `$HOME` pointing at a folder holding the file it said `The certificate is not trusted`; with the file in `/root/.pki/CA/cacert.pem` it connected). Without a file it falls back to the system trust store. The other `remote-viewer` cases (name mismatch, wrong password, the certificate bundle) were **not** run: the tool's safety check refused a second write into the real home folder, so I stopped and removed the file I had made. UNKNOWN. |
| **Window title / app-id with login** | unchanged by the login | the connection file's `title=` gave `Test Node (1)`; `--name=hubos-test` kept the app-id |

### Where the credential would live (input for the secrets design; nothing is designed)

| Secret or public file | Where | Notes |
|---|---|---|
| Node: user name, password, TLS private key | the node's own per-machine config area, mode 0600, **never in git, never in the NAS copy** (`HUB-OS.md`: "Secrets do not [get the NAS backup copy]") | wayvnc reads them from its config file (`-C`); there is no way to give the password by environment or file descriptor that I found (UNKNOWN: not searched in the source). `wayvnc` also links PAM (`libpam0g` in its dependency list); `HUB-OS.md` says finished images do not contain PAM modules, so build wayvnc without PAM (`-Dpam=disabled` built fine, TESTED for v0.10.2) |
| Node: certificate (public) | next to the key on the node; a copy on the hub | public, can go in the image or the config area |
| Hub: the same user name and password, so hubd can open the viewer | the hub's per-machine config area, mode 0600, outside git and outside the NAS copy | **how hubd hands it to the viewer without a command line:** TigerVNC: the environment variables `VNC_USERNAME` and `VNC_PASSWORD` (TESTED); `remote-viewer`: a connection file with `username=` and `password=` (TESTED once; it must be mode 0600) written by hubd just before the start and removed after (BELIEVED safe enough; not designed). `hubd` today cannot do either: it starts the command from `viewers.toml` with placeholders only, and has no secret store. That is a **hubd change** to be designed with the owner. |
| Hub: the node's certificate as the trusted authority | TigerVNC: any path (`-X509CA`); `remote-viewer`: one fixed file `~/.pki/CA/cacert.pem` for **all** nodes | with one self-signed certificate per node, `remote-viewer` would need all of them in that single file (BELIEVED to work as a bundle; not tested), or **one cluster certificate** with every node's name in its subjectAltName and the same key copied to every node (simplest; but then one stolen key impersonates every node) |
| One password for the cluster, or one per node | decision for the owner | one password is the least work and gives away every node if it leaks |

**My recommendation for the display protocol's login:** TLS with a self-signed certificate (set-up B), because it is the only one both viewers accept and the TigerVNC viewer then opens without any dialog. RSA-AES (set-up A) needs no file on the node and is a good fallback for tests, but TigerVNC asks for a click every time and `remote-viewer` cannot do it.

---

## R2.4 The hub side under driftwm: window id, title, app-id and hubd's matching

TESTED. driftwm (pinned commit 352333a8, nested, software rendering) on Xvfb; `hubd serve` and `hubd open` from this repository's `cmd/hubd` (built with `go build ./cmd/hubd`; no Go code changed); `round2/hub-inventory.toml` and `round2/hub-viewers.toml` (two fake machines, both pointing at the same wayvnc on loopback). Window data from `driftwm msg state`.

| Viewer (command) | driftwm id | app-id | title | size | hubd said |
|---|---|---|---|---|---|
| `remote-viewer --name=hubos-gui-a --title="Node A" -- vnc://127.0.0.1:5901` (`sets_name = true`) | `#0` first run (the number goes up with each new window; `#13` by the end; numbers are not reused) | `hubos-gui-a` | `Node A (1)` | 1022 x 800 after hubd shrank it from 1022 x 814 "to stay clear of the bar" | `opened Node A at home (-1500, 0), matched by name` |
| `xtigervncviewer 127.0.0.1::5901` against wayvnc 0.7.2 (`sets_name = false`, `title_match = "WayVNC - TigerVNC"`) | `#1` | `TigerVNC Viewer` | `WayVNC - TigerVNC` | 1024 x 725 | `opened Node B at home (1500, 0), matched by title` |
| same, against wayvnc 0.10.2 started with `-n gui-b` (`title_match = "{id} - TigerVNC"`) | `#7`, `#9`, `#12` (reopened) | `TigerVNC Viewer` | `gui-b - TigerVNC` | 1016 x 778 | `matched by title` |

Other facts from the same runs:

- Both viewers were **placed at their home positions** by hubd through driftwm's socket and the camera followed (`camera -1500 7`); the placement worked for the X11 program too.
- driftwm reports **no window id of the program's own** other than its running number; app-id plus title is all there is, as round 1 assumed.
- The app-id of the TigerVNC viewer is **shared by all its windows**, so two nodes in TigerVNC windows can be told apart only by title (R2.2).
- Opening TigerVNC (default, not shared) for a machine while a `remote-viewer` window for the same wayvnc was open dropped the `remote-viewer` window (R2.2). This happened in my own test because both fake machines pointed at the same wayvnc; real machines have one viewer each.
- driftwm advertises data-control (wlr v2 and ext v1), primary selection and the virtual keyboard protocol (`wayland-info`, TESTED); no virtual pointer, as before.
- Nested driftwm needed a keyboard: it took key events from the Xvfb window, so `xdotool` could drive it. Clipboard from the hub to the node worked with the TigerVNC window focused (R2.1 row 8).
- Not tested under driftwm: Moonlight, a real GPU, fractional scale, a real display backend.

---

## R2.5 Which wayvnc version the images need

All results here that depend on newer behaviour came from wayvnc **v0.10.2 + neatvnc v1.0.3 built from source** (`-Dpam=disabled -Dman-pages=disabled -Dscreencopy-dmabuf=disabled -Dneatvnc:h264=disabled -Dneatvnc:gbm=disabled`, aml 1.0.0 as a subproject; built with 2 jobs in a few minutes (not timed); H.264 and GPU capture were left out and are UNTESTED). Facts:

| Need | Version | Label |
|---|---|---|
| UTF-8 clipboard (extended clipboard) | neatvnc >= 0.9.0 (wayvnc 0.9.x or newer; Ubuntu questing/resolute have 0.9.1) | TESTED with 1.0.3; SOURCE for the first tag |
| `--name` / `wayvncctl set-desktop-name` (for the TigerVNC title) | wayvnc >= **0.10.0** (Ubuntu stonking has 0.10.1; **no earlier series**) | SOURCE `git tag --contains 630ed4a` |
| client-requested screen resize works | worked with 0.10.2; with 0.7.2 it failed | TESTED |
| ext-data-control | wayvnc >= 0.10.0; needed only if a node compositor offers `ext_data_control` but not `zwlr_data_control` (sway 1.9 offers the wlr one; which compositors offer only ext: UNKNOWN) | SOURCE |
| Ubuntu 24.04 (what the repo's image base uses) | wayvnc 0.7.2, neatvnc 0.7.1 (**too old for all of the above**) | TESTED, SOURCE (Launchpad) |

So a node image based on the 24.04 snapshot would have to carry a **newer wayvnc and neatvnc taken from a newer Ubuntu series or built from source**. That is an image-builder decision (a pinned source build is what `HUB-OS.md` already does for driftwm). Not decided here.

---

## R2.6 Recommendation update (still a proposal) and what is not proven

1. **Display protocol stays VNC with wayvnc** on the nodes, but at **wayvnc v0.10 or newer**, not the 24.04 package: only that gives UTF-8 clipboard, a settable desktop name and resizing.
2. **Hub viewer for ordinary nodes: the TigerVNC viewer is the only one tested that carries the clipboard hub to node**, and the only one that opens an encrypted, password-protected session with no dialog. Its costs, each tested: it is an X11 program, so the hub image needs Xwayland, xkbcomp and xwayland-satellite (built from git); its app-id is fixed, so hubd must match by title and every node must name its desktop after its machine id; the clipboard works only while its window is focused (and a copy made while unfocused did not arrive in my test); with wayvnc 0.7.2 it garbles non-ASCII text.
3. **`remote-viewer` stays for VM guests** (SPICE and VNC; the owner's decision) but is **not enough for nodes** while the clipboard hub to node is required: it never sends it, and no release of virt-viewer or gtk-vnc contains a fix.
4. **Login and encryption:** TLS with a self-signed certificate plus user name and password (R2.3, set-up B). Nothing on the display link should be left unauthenticated: a no-password wayvnc showed clipboard text in the clear.
5. **A change in hubd is needed for any of this** (not made): hand a user name and password to the viewer through the environment or a private file, and `title_match` with `{id}` already exists. The credential store is the open secrets design.
6. Everything in sections 3, 4 and 6 of round 1 (sound by PipeWire RTP, the node helper, GPU apps) is unchanged and not re-examined.

**Not proven this round (all UNKNOWN):** a real Wayland-native VNC viewer with clipboard hub to node (the candidates: gtk-vnc master, unreleased; `any1/wlvncc`, whose README calls it "a work-in-progress implementation ... Expect bugs and missing features" and which has no Ubuntu package [Launchpad listing empty, 2026-10-04]; neither was run); the unfocused-copy failure's cause; `remote-viewer` with a name-mismatched certificate or a wrong password; a certificate bundle in `~/.pki/CA/cacert.pem`; EC keys; H.264 or GPU capture in wayvnc; long-running behaviour; many windows at once; real hardware; the clipboard with several nodes open at once (only one window can be focused, so clipboard to a node you are not looking at cannot work, BELIEVED).

---

## R2.7 What this round changes in round 1's text (round 1 is left as written)

- Plain-words item 4 and section 2.2: "UNKNOWN why [clipboard hub to node did not work]" is now answered: `remote-viewer` has no such feature (R2.1). "RDP stays the fallback if the clipboard cannot be made to work over VNC" is no longer needed: it works with the TigerVNC viewer.
- Section 2.7: "`remote-viewer` does not accept a self-signed VNC certificate" is true only without a trust file; with the certificate in the fixed `~/.pki/CA/cacert.pem` it did (R2.3).
- Section 5, wayvnc row: the security types are now identified: 19 VeNCrypt (TLS), 129 and 5 RSA-AES; "which of these `remote-viewer` can use" is answered: VeNCrypt only; TigerVNC: both.
- Section 7: `vnc` entry. A TigerVNC entry would be `["xtigervncviewer", "{address}::{port}"]` with `sets_name = false` and `title_match = "{id} - TigerVNC"` (TESTED shape); the credentials need the hubd change above.
- Section 8, recommendation 1 ("viewer: `remote-viewer` for now, with the TigerVNC viewer tested as a replacement"): now tested; see R2.6.
- Section 8, question 3 (may I test the TigerVNC viewer): done.
- Section 6.1: "driftwm has no virtual-pointer protocol" still stands; this round used a headless sway as the node.

---

## R2.8 Questions for the owner

1. **Hub viewer for nodes.** The TigerVNC viewer is the only one that does clipboard hub to node today, but it needs Xwayland, xkbcomp and xwayland-satellite in the hub image and matches by title only. Accept that cost? Or wait for a Wayland-native viewer with the clipboard (none exists today)? Or accept node to hub only, and keep `remote-viewer`?
2. **Clipboard rule.** Is "the node's window must be focused for the clipboard to move" acceptable (it is how the TigerVNC viewer is built), given that a copy made while the window was not focused did not arrive in my test?
3. **Which wayvnc.** Nodes need wayvnc 0.10 or newer. May the image builder build wayvnc and neatvnc from source at pinned tags (as for driftwm), or take them from a newer Ubuntu series? Which pinned tags: v0.10.2 and v1.0.3 are what I tested.
4. **Display login.** TLS with a self-signed certificate plus password: one certificate and one password for the whole cluster, or one per node? (One is simpler and gives everything away if it leaks.)
5. **Secrets path.** hubd must hand a user name and password to the viewer. Environment variable (TigerVNC) and a private connection file (`remote-viewer`) both work. May a later task design this (the hub's config area, mode 0600, outside git and the NAS copy)?
6. **Screen size.** The TigerVNC viewer makes the node's screen follow the window (with wayvnc 0.10). Do you want the node's screen size fixed per node (then it must be switched off with `RemoteResize=0` or `wayvnc -R`), or following the window?
7. **Shared or not.** A second non-shared viewer kicks the first. Keep the default (one viewer per node), or pass `-Shared` so that a second window never kills the first?
8. **Node naming.** Every node's wayvnc must be started with `--name <machine id>`. OK to make that part of the node helper's session start?
9. **`docs/driftwm-findings.md`** says xwayland-satellite can be installed with `cargo install`; that is wrong today (not on crates.io). May a later task correct that file?
10. **`HUB-OS.md`** (not edited): the Unverified list would gain "TigerVNC viewer clipboard needs focus; an unfocused copy did not arrive", "remote-viewer never sends the clipboard to a VNC server", "Xwayland and xwayland-satellite on the hub image", "wayvnc >= 0.10 on the nodes". May I draft the change for you to approve?

---

## R2.9 Sources read for round 2 (all 2026-10-04)

Read by the helper itself (raw files, git history or JSON; not summarised): gtk-vnc `src/vncdisplay.c`, `src/vncconnection.c` at v1.3.1, v1.5.0 and master, and GNOME GitLab's commit and tag lists; virt-viewer `src/virt-viewer-session-vnc.c`, `src/virt-viewer-app.c` (v11.0) and the master copy, and GitLab's tag list; wayvnc `src/main.c` (v0.7.2 and master), the README at v0.10.2, and the git history of [any1/wayvnc](https://github.com/any1/wayvnc) and [any1/neatvnc](https://github.com/any1/neatvnc) (cloned); TigerVNC `vncviewer/vncviewer.cxx` (v1.13.1 and master), `vncviewer/Viewport.cxx` (v1.13.1) and the tag list from `git ls-remote`; the man pages and `-h` text of the unpacked packages; Launchpad's package-publishing API; the [wlvncc README](https://raw.githubusercontent.com/any1/wlvncc/master/README.md).

Through a summarising tool: the TigerVNC releases page (fetch tool) and one web search about TigerVNC 1.16 and Wayland (linuxiac.com, phoronix.com, 9to5linux.com in the result list); I used them only for the sentence that 1.16 added `w0vncserver`, which is not needed for any conclusion.

Could not be reached: the GitHub REST API and the GitHub release atom feeds (the session's proxy answers that those paths are not available); so release **dates** for wayvnc, neatvnc and TigerVNC come from git tag dates only where stated, and TigerVNC's 1.16.x dates are as round 1 gave them.
