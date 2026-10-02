# Viewers research

**Researched: 2026-10-02 — build environment; will change.** This file records what the official documentation, man pages, `--help` output and source code say about the programs Hub OS would start as viewers, and what happened when the ones that are in the Ubuntu archive were run under driftwm. `HUB-OS.md` wins if anything here disagrees with it. It is the input for the real `viewers.toml` (`docs/hubd-slice2.md`) and, later, for the secrets design.

**Nothing was verified on hardware.** Everything that was run used the nested, software-rendered driftwm of `docs/driftwm-findings.md`, on a virtual screen, in the cloud build environment. No real screen, GPU, network or input device. No real Moonlight or Sunshine was run.

## How this was done

- **Sources** (all read-only; nothing was pushed, forked or filed anywhere): the `moonlight-qt` repository at commit `8369d1a0e11b999d4d1598f62ca5f6dea49602fb` (2026-09-27), the Sunshine repository at `dfa9e884978398f23d2f63dd987b9d554fa4d68e` (2026-09-30), the Moonlight documentation wiki at `77beab7e7bfdb9b110a84a6b665781a40a110af7` (2025-09-05), the Remmina wiki at `612efa23b197dee94fe33c449d1283ea3c251b04` (2025-04-26), the Ubuntu 24.04 ("noble") man pages and `--help` output of the programs below, and the libvirt, QEMU, SPICE, Samba and OpenSSH documents linked where used. The repositories were cloned shallow with `git clone --depth 1` from their public addresses (the `add_repo` tool was not needed). Quotes are limited to a few words; everything else is paraphrased.
- **Tests.** The packages for `remmina` (with the VNC, SPICE and RDP plugins), `virt-viewer` (`remote-viewer`), `openssh-client`, `pcmanfm` and its libraries were downloaded from the Ubuntu 24.04 archive (`apt-get install --print-uris`, `curl`) and unpacked with `dpkg -x` into a temporary directory. **Nothing was installed.** Remmina looks for its plug-ins in a fixed system folder, so it was started inside a private mount namespace (`unshare -m`, then `mount --bind` of the unpacked plug-in folder onto that path); the mount vanished with the process and the folder never existed outside it. `foot` 1.16.2 comes from the archive too. The fake machine was a 30-line Python program speaking just enough of the VNC protocol (RFB 3.8, no security, a blank 640x480 screen with a chosen name) on `127.0.0.1:5997`. Window names were read with the same JSON request that `driftwm msg state` makes (`"State"` on driftwm's socket). Everything created outside the repository was deleted afterwards.
- **Labels**, one per item: **TESTED** (the command is shown and the result is what it printed), **SOURCE** (read in the official documents or code; link, and file and line where there is one), **BELIEVED** (reasoned from the above, not run), **UNKNOWN**.
- **Not tried:** building Moonlight or Sunshine. `moonlight-qt` needs the Qt 6 SDK, SDL2, FFmpeg, libva, libvdpau, libdrm and several git submodules ([README.md lines 55-80](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/README.md#L55-L80)); Sunshine needs a much larger tool chain ([docs/building.md](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/docs/building.md)). Neither is a quick build, so for both the window names are **SOURCE** only. **TESTED:** `apt-cache policy moonlight-qt sunshine` printed nothing for both, i.e. neither is in the Ubuntu 24.04 archive.

---

## Summary

1. **Four of the six can be given a window name per machine; Moonlight and the file manager cannot.** `foot --app-id=…`, `remote-viewer --name=…` and `remmina --name=…` all put the chosen text into the Wayland app-id (TESTED, all three under driftwm). Moonlight sets its own fixed app-id `com.moonlight_stream.Moonlight` in code and there is no option to change it (SOURCE). `pcmanfm` ignored `--name` (TESTED).
2. **So Moonlight must be matched by comparison (`sets_name = false`), and two Moonlight windows cannot be told apart by their app-id.** Their titles differ: the stream window is titled with the host's name followed by "- Moonlight" (SOURCE), and the host's name is Sunshine's `sunshine_name` (default: the PC's host name), not our inventory name. hubd matches by app-id only today.
3. **Remmina is single-instance and a second start opens a new tab in the first window**, then exits (TESTED). That is the "clean exit, window comes later" case that the late-window rule was made for. A different `--gapplication-app-id` per machine gives separate instances (TESTED), but then the app-id Remmina uses for each machine's window is the one given with `--name` (TESTED).
4. **remote-viewer is not single-instance** (two processes, two windows, TESTED), its title gets " (1)" added (TESTED), and closing its window asks "Do you want to close the session?" first (TESTED, screenshot). Remmina asks too ("Are you sure you want to close 2 active connections…", TESTED).
5. **A window appears even when the connection fails** for remote-viewer (a small error window with an empty title), Remmina (a normal window with an error line inside), and foot (the window disappears at once unless `--hold`). TESTED for each.
6. **Closing a Moonlight window does not end the app on the host** unless "quit after" is on, and it is off by default (SOURCE: moonlight-qt `streamingpreferences.cpp:137`; FAQ). Pressing Ctrl+Alt+Shift+Q or closing the window leaves the game running on the Sunshine host; "Quit App" in Moonlight ends it (FAQ).
7. **Sunshine's ports are all offsets from one base port, 47989.** TCP 47984 (HTTPS pairing, -5), 47989 (HTTP, 0), 47990 (web UI, +1), 48010 (RTSP, +21); UDP 47998 (video, +9), 47999 (control, +10), 48000 (audio, +11) (SOURCE, code). For a bare TCP connect check, **47989** is the one that stands for "Sunshine is up" (BELIEVED: it is the HTTP server that starts with Sunshine, and the base port is the one every client first talks to).
8. **SPICE and VNC console ports are not fixed.** With libvirt a guest's port is either set in the guest's XML or auto-assigned from 5900 upwards (SOURCE); with plain QEMU the person chooses it (`-spice port=…`, VNC display N = port 5900+N). So the inventory must hold the port per guest; there is no default hubd can rely on.
9. **Moonlight's "release the mouse" shortcut (Ctrl+Alt+Shift+Z) is enabled only when Moonlight believes a desktop environment is running**; under a Wayland compositor it does (SOURCE: it counts a running Wayland or X11 session). It can be forced with an environment variable. A keyboard-capture setting decides whether system shortcuts go to the stream (SOURCE).
10. **Secrets.** Moonlight keeps its client certificate and private key in its Qt settings file; Sunshine keeps its certificate, key, web-UI credentials and the list of paired clients in its config folder; Remmina keeps profiles with an optional (weakly) encrypted password; none of these may enter the inventory or git (details in the sections, listed again in "Input for the secrets design").

---

## 1. Moonlight (moonlight-qt) on Linux

**Role:** the client for the gaming box, the AI box and the general desktop. Source: [moonlight-qt](https://github.com/moonlight-stream/moonlight-qt/tree/8369d1a0e11b999d4d1598f62ca5f6dea49602fb), wiki [Setup Guide](https://github.com/moonlight-stream/moonlight-docs/wiki/Setup-Guide).

### a) Command line

- **Form (SOURCE):** `moonlight stream <host> "<app>" [options]` (file [`app/cli/commandlineparser.cpp` lines 334-373](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/cli/commandlineparser.cpp#L334-L373)). `<host>` is a computer name, UUID or IP address; `<app>` is the name of an app that the host lists (for Sunshine these come from its Applications page). Related commands: `moonlight pair <host> [--pin NNNN]`, `moonlight list <host>`, `moonlight quit <host>` (lines 205-262, 548-553 of the same file).
- **Options we would need (SOURCE, same file):** `--display-mode windowed|borderless|fullscreen` (lines 298-302; `windowed` is the one that keeps a normal window), `--resolution WxH` or `--720/--1080/--1440/--4K`, `--fps`, `--bitrate`, `--video-codec`, `--video-decoder`, `--capture-system-keys never|fullscreen|always` (lines 319-323), `--quit-after` / `--no-quit-after`, `--absolute-mouse`, `--audio-on-host`, `--keep-awake`, `--performance-overlay`. **There is no option for a port, a title or an app-id.**
- **Host and port (SOURCE):** the host string is looked up in the list of hosts Moonlight already knows, by name, UUID, address or "address:port" (`app/backend/computerseeker.cpp` [lines 50-66](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/backend/computerseeker.cpp#L50-L66)); if it is not known, Moonlight tries to add it as a new host with the default port (lines 22-36, [`computermanager.cpp` 728-741](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/backend/computermanager.cpp#L728-L741): a URL form `host:port` is accepted there). **The host must already be paired**; otherwise the command fails with a message that says to open Moonlight and pair first (`app/cli/startstream.cpp` [lines 82-94](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/cli/startstream.cpp#L82-L94)). The search for the host gives up after 30 seconds (`startstream.cpp` line 9).
- **App name:** UNKNOWN which name hub OS should use. Our inventory has no field for the app/session a machine streams. `viewers.toml` would need a new placeholder (see the proposal).
- **App-id:** **fixed.** `main.cpp` sets the desktop file name and the window class to `com.moonlight_stream.Moonlight` ([lines 928-931](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/main.cpp#L928-L931)); because it is written into the process environment after start-up, setting the variable from outside does not change it (BELIEVED, from that code order; not run). **Title:** the host's name followed by " - Moonlight" ([`app/streaming/session.cpp` lines 1842-1846](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/streaming/session.cpp#L1842-L1846)); the host's name is what Sunshine calls `sunshine_name` (default: the PC's host name; [configuration.md line 158](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/docs/configuration.md#L158)). It is the same for every window of that host and differs between hosts.

### b) App-id and title it reports

- **SOURCE only** (not built; see above): app-id `com.moonlight_stream.Moonlight`, title "<host name> - Moonlight". On Wayland the stream window is an SDL window, and Moonlight picks SDL's Wayland driver when Qt runs on Wayland ([`main.cpp` lines 870-884](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/main.cpp#L870-L884)). A plain Moonlight start without the `stream` command opens the game-list window (QML) with the same fixed app-id (BELIEVED; the code sets it once for the whole process).
- **If the connection fails:** UNKNOWN (not run). The code reports a failure through the `stream` command's error path and exits (`startstream.cpp`), so a window may never appear (BELIEVED).

### c) When the window is closed

- Closing the stream window ends the **stream**; the **app keeps running on the host** unless "quit after session" is set. `--quit-after` is off by default (SOURCE: [`streamingpreferences.cpp` line 137](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/settings/streamingpreferences.cpp#L137)). The FAQ says that when you exit the stream or close Moonlight the session stays on the host so you can pick it up again from another device, and that to end it you choose "Quit App" (which forcefully ends the app) ([FAQ lines 88-94](https://github.com/moonlight-stream/moonlight-docs/wiki/Frequently-Asked-Questions)). The Setup Guide lists Ctrl+Alt+Shift+Q as "quit the streaming session, leaving the game running on the host PC" ([Setup-Guide.md lines 345-353](https://github.com/moonlight-stream/moonlight-docs/wiki/Setup-Guide)).
- This matches the Hub OS rule that closing a window leaves the machine's session alive (SOURCE for Moonlight; Sunshine's own side: see section 2).

### d) Single instance

- **No instance lock was found.** A search of the Moonlight application code for lock-file, shared-memory, single-application and "already running" wording found nothing (TESTED search: `grep -rniE "QLockFile|QSharedMemory|SingleApplication|already running|another instance|single instance"` in `app/`, no output). So a second `moonlight stream` for the same host would start its own process and try its own session (BELIEVED). What the Sunshine host does with a second client for the same app is UNKNOWN. **The late-window rule's "clean exit" case does not apply to Moonlight** (the process stays alive while streaming).

### e) Ports

- **Moonlight → host (SOURCE):** HTTP 47989 and HTTPS 47984 by default ([`app/backend/nvaddress.h` lines 5-6](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/backend/nvaddress.h#L5-L6)); a different port is stored per host as a "manual port" (`nvcomputer.cpp` lines 17-18, 38-39) and is given when the host is added by hand. The stream itself uses further UDP and TCP ports that depend on the host (section 2). There is **no `--port` option on the `stream` command**; a non-default port has to be stored beforehand (UNKNOWN whether `moonlight stream host:port` stores it; the code path for unknown hosts above suggests yes, BELIEVED).

### f) What it needs on the hub (no systemd, glibc only)

- **Build/run libraries (SOURCE, README lines 55-80):** Qt 6 (base, declarative/QML, SVG, Wayland platform plug-in), SDL2 and SDL2_ttf, FFmpeg 4.0+ (libavcodec, libavformat, libswscale), libopus, OpenSSL, libva, libvdpau, libdrm, libxkbcommon, EGL/GL, Wayland protocols; optional Vulkan renderer needs libplacebo 7.349+ and FFmpeg 6.1+.
- **Display:** Wayland or X11; it also has a no-desktop mode (EGLFS/KMSDRM) for embedded builds (SOURCE, `main.cpp` 870-890, README). Under Wayland the window is an SDL Wayland window (SOURCE above).
- **GPU decoding (SOURCE):** renderers for VAAPI, VDPAU, DRM/V4L2, CUDA, EGL and Vulkan are in `app/streaming/video/ffmpeg-renderers/`; the wiki says hardware decoding differs between package types and some need extra driver packages ([Fixing Hardware Decoding Problems, lines 19-29](https://github.com/moonlight-stream/moonlight-docs/wiki/Fixing-Hardware-Decoding-Problems)).
- **Audio (SOURCE):** through SDL's audio (`app/streaming/audio/renderers/sdlaud.cpp`); which sound server SDL uses (PipeWire or PulseAudio) is chosen by SDL (BELIEVED; not run).
- **Input:** SDL keyboard/mouse/gamepad (SOURCE); keyboard capture rules in (h).
- **systemd / D-Bus:** a search of the application code for systemd, logind and D-Bus found nothing (TESTED search: `grep -rniE "systemd|logind|dbus"` in `app/`, no output, submodules excluded). Qt itself may use D-Bus (e.g. for themes); not checked (UNKNOWN).
- **Ubuntu 24.04 archive:** **not there** (TESTED: `apt-cache policy moonlight-qt` printed nothing). The project's own packages are Flatpak, Snap and AppImage and distribution packages; the wiki also lists them (Fixing Hardware Decoding Problems, lines 19-29). Building from source is the route for a Hub OS image (README). The Setup Guide names Debian 11, Ubuntu 22.04 and Fedora 38 and newer as supported Linux systems ([line 21](https://github.com/moonlight-stream/moonlight-docs/wiki/Setup-Guide)).

### g) Pairing and credentials

- **Pairing (SOURCE):** Moonlight asks the host for a pairing with a 4-digit PIN that it shows; the PIN is typed on the host. For Sunshine: log in to the web UI, open "PIN", type the PIN and a name for the device ([Sunshine getting_started.md lines 703-708](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/docs/getting_started.md#L703-L708)). The command line form is `moonlight pair <host> --pin NNNN` (`commandlineparser.cpp` line 262).
- **What Moonlight stores (SOURCE):** a unique client id, a certificate (PEM) and a private key, in Qt settings under the names `uniqueid`, `certificate` and `key` ([`app/backend/identitymanager.cpp` lines 11-13, 103-104](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/backend/identitymanager.cpp#L11-L13)), plus per host: its name, UUID, addresses, ports, the host's certificate (`nvcomputer.cpp`). The organisation and application names are "Moonlight Game Streaming Project" and "Moonlight" (`main.cpp` 431-433). Qt's default file for this on Linux is under `~/.config/` (BELIEVED from Qt's documented `QSettings` behaviour; not run).
- **Must stay out of git and the inventory:** the private key, the certificate, the client id, and the PIN while pairing.

### h) Clipboard, audio, keyboard capture

- **Shortcuts (SOURCE, Setup-Guide lines 345-353 and `app/streaming/input/input.cpp` lines 82-120):** Ctrl+Alt+Shift+**Z** toggles mouse and keyboard capture (this is the "release" shortcut); +**Q** quits the stream (game keeps running); +**X** fullscreen/windowed; +**S** stats overlay; +**M** mouse mode; +**V** types the host-side clipboard text on the host; +**D** minimise the window; +**C** and +**L** cursor options.
- **Z only works when a "desktop environment" is believed to run:** `KeyComboUngrabInput.enabled = WMUtils::isRunningDesktopEnvironment()` ([`input.cpp` line 93](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/streaming/input/input.cpp#L90-L93)), which is true when a Wayland or X11 session is running, and can be forced with the environment variable `HAS_DESKTOP_ENVIRONMENT` ([`app/wm.cpp` lines 177-206](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/wm.cpp#L177-L206)). Under driftwm that is true, so the shortcut should work (BELIEVED; not run with a real Moonlight).
- **Clipboard:** there is no shared clipboard in the stream; "Ctrl+Alt+Shift+V" types the **local** clipboard text as key presses on the host ([`keyboard.cpp` lines 104-135](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/streaming/input/keyboard.cpp#L104-L135)). The brief's "copy-paste works across all windows" is therefore **not provided by Moonlight itself** for text coming back from the host (UNKNOWN whether Sunshine has a host-to-client clipboard; none was found in the Sunshine documents read).
- **Keyboard capture setting (SOURCE):** `--capture-system-keys never|fullscreen|always`: whether keys like Alt+Tab or the Super key go to the stream. Its default depends on the stored setting (not read). The Sunshine configuration notes that Wayland does not let clients capture the Windows/Super key ([configuration.md line 762](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/docs/configuration.md#L762), host side).
- **Audio:** the host streams audio to the client by default; `--audio-on-host` keeps the sound on the host instead (SOURCE, option list). HUB-OS.md plans all audio through the hub, i.e. the default.
- **Driftwm interaction (BELIEVED, from `docs/driftwm-findings.md`):** Moonlight would need the per-window `pass_keys` rule for keys, and the window's app-id is the fixed one above, so the rule would apply to all Moonlight windows.

---

### Check of the proposed moonlight command (SOURCE)

Added in a later round. Checked against the same `moonlight-qt` commit as above (`8369d1a0e11b999d4d1598f62ca5f6dea49602fb`), read-only, shallow fetch of that one commit; nothing was built or run. The command that was proposed (in `examples/viewers.real.example.toml`):

`moonlight stream {address} {session} --display-mode windowed --no-quit-after --capture-system-keys never`

**Result: every part exists. Nothing differs, so the example was not changed.** Each item below is SOURCE.

| Part | Exists? | What it accepts | Where |
|---|---|---|---|
| `stream <host> "<app>"` | yes | `stream` is the action; the second word is the host (name, UUID or IP address), the third the app name. Fewer than three words is an error ("Host not provided" / "App not provided", exit 1). The word `stream` is looked for among the positional words, wherever it stands. | [`cli/commandlineparser.cpp` lines 334-338](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/cli/commandlineparser.cpp#L334-L338), [`cli/commandlineparser.cpp` lines 513-523](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/cli/commandlineparser.cpp#L513-L523), [`cli/commandlineparser.cpp` lines 181-193](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/cli/commandlineparser.cpp#L181-L193) |
| `--display-mode windowed` | yes | a choice of exactly `fullscreen`, `windowed`, `borderless` (letter case ignored); anything else is an error "Invalid display-mode choice" | [`cli/commandlineparser.cpp` lines 298-302](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/cli/commandlineparser.cpp#L298-L302), [`cli/commandlineparser.cpp` line 353](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/cli/commandlineparser.cpp#L353), [`cli/commandlineparser.cpp` lines 99-105](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/cli/commandlineparser.cpp#L99-L105), [`cli/commandlineparser.cpp` lines 434-436](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/cli/commandlineparser.cpp#L434-L436) |
| `--no-quit-after` | yes | `--quit-after` and `--no-quit-after` are a pair made by one helper (a flag with no value). The option means "quit the **app on the host** after the session"; the default is off (`false`), so `--no-quit-after` only restates the default. It is not the same as Moonlight itself exiting. | [`cli/commandlineparser.cpp` lines 122-126](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/cli/commandlineparser.cpp#L122-L126), [`cli/commandlineparser.cpp` line 356](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/cli/commandlineparser.cpp#L356), [`cli/commandlineparser.cpp` lines 449-450](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/cli/commandlineparser.cpp#L449-L450), [`settings/streamingpreferences.cpp` line 137](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/settings/streamingpreferences.cpp#L137) |
| `--capture-system-keys never` | yes | a choice of exactly `never`, `fullscreen`, `always` (letter case ignored); `never` maps to "off" | [`cli/commandlineparser.cpp` lines 319-323](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/cli/commandlineparser.cpp#L319-L323), [`cli/commandlineparser.cpp` line 371](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/cli/commandlineparser.cpp#L371), [`cli/commandlineparser.cpp` lines 494-497](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/cli/commandlineparser.cpp#L494-L497) |
| Options **after** the positional words | accepted by the parser as far as the code shows | The code adds options and positional words to one Qt `QCommandLineParser` and does not switch it to "options only before the words". Qt's default is to accept options anywhere (BELIEVED, from Qt's documented default; the Qt source was not read and nothing was run). | [`cli/commandlineparser.cpp` lines 330-377](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/cli/commandlineparser.cpp#L330-L377) |
| Host **without a port** | yes | If the host text matches a host Moonlight already knows (by name, UUID, address or "address:port"), that host is used. If not, Moonlight adds it by hand and, when no port is given, uses the default **47989**. A port may be given in the text (`host:port`). So `{address}` alone is fine for a Sunshine on its default port. | [`backend/computerseeker.cpp` lines 22-36](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/backend/computerseeker.cpp#L22-L36), [`backend/computerseeker.cpp` lines 50-66](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/backend/computerseeker.cpp#L50-L66), [`backend/computermanager.cpp` lines 728-741](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/backend/computermanager.cpp#L728-L741), [`backend/nvaddress.h` line 5](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/backend/nvaddress.h#L5) |
| App name match | yes | the app name is compared with the host's app list, **letter case ignored**; it must be one of the apps Sunshine lists | [`cli/startstream.cpp` lines 143-151](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/cli/startstream.cpp#L143-L151) |

Things the check found that matter for hubd (all SOURCE, none run):

1. **Two windows, not one.** `moonlight stream` first shows Moonlight's ordinary main window with the text "Establishing connection to PC..." and later "Loading app list...", and hides it when the stream starts ([`gui/main.qml` lines 35-48](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/gui/main.qml#L35-L48), [`gui/CliStartStreamSegue.qml` lines 6-13](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/gui/CliStartStreamSegue.qml#L6-L13), [`gui/StreamSegue.qml` lines 41-42](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/gui/StreamSegue.qml#L41-L42)). That window carries the same fixed app-id as the stream window. Its title is not set in the QML; it is therefore believed to be the application name "Moonlight" (BELIEVED; the application name is set in [`main.cpp` line 433](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/main.cpp#L433)). The stream window is a separate SDL window titled "<host name> - Moonlight" ([`streaming/session.cpp` lines 1842-1846](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/streaming/session.cpp#L1842-L1846)). This is a second reason (besides the fixed app-id) why matching by comparison would be ambiguous for Moonlight, and why `title_match` is the right way. A window titled just "Moonlight" is not the stream window and is ignored by `title_match`.
2. **An error shows as a dialog in that first window**, not as a missing window: unpaired host ("has not been paired. Please open Moonlight to pair before streaming"), host not found, app not found, and the like ([`cli/startstream.cpp` lines 82-94](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/cli/startstream.cpp#L82-L94), [`gui/CliStartStreamSegue.qml` lines 25-29](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/gui/CliStartStreamSegue.qml#L25-L29)); on closing the dialog Moonlight exits. So on a failed start hubd sees a window with the fixed app-id and the title "Moonlight", never one titled "<id> - Moonlight"; it will end as "no window appeared" or as the late-window state, with the viewer's process still running until the dialog is closed. That is UNVERIFIED in practice.
3. **Another app already running on the host:** if the host is streaming a different app, Moonlight asks (in that window) whether to quit it ("Are you sure you want to quit ...?") before it starts the new one ([`cli/startstream.cpp` lines 103-109](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/cli/startstream.cpp#L103-L109), [`gui/CliStartStreamSegue.qml` lines 31-34](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/gui/CliStartStreamSegue.qml#L31-L34)). The stream window then comes only after someone answers; this is another reason for the 30-second `window_wait` and the late grace.
4. **The title uses the host's own name** (the name Sunshine reports), not the text given on the command line ([`streaming/session.cpp` lines 1842-1846](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/streaming/session.cpp#L1842-L1846)). The `{id} - Moonlight` rule therefore needs each node's Sunshine name to equal its machine id (already on the Unverified list).
5. **The host must already be paired** ([`cli/startstream.cpp` lines 82-94](https://github.com/moonlight-stream/moonlight-qt/blob/8369d1a0e11b999d4d1598f62ca5f6dea49602fb/app/cli/startstream.cpp#L82-L94)); pairing is not part of this command (section 1, pairing).

No flag in the proposed command is missing or different, so there is no question about the flags. Not checked: whether the command works against a real Sunshine; the Qt behaviour for options after positional words.

---

## 2. Sunshine

**Role:** the stream host on the gaming box, AI box and general desktop. Source: [Sunshine](https://github.com/LizardByte/Sunshine/tree/dfa9e884978398f23d2f63dd987b9d554fa4d68e), documents under `docs/`, site [docs.lizardbyte.dev](https://docs.lizardbyte.dev/projects/sunshine/latest/).

### a) Command line

- Sunshine is a server, not a viewer, so there is nothing to open. It is started once on the machine (any init; the packaged `systemctl --user` unit is only one way, [getting_started.md lines 548-564](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/docs/getting_started.md#L548-L564)), takes a config file path as its argument (`sunshine --help`, [lines 721-727](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/docs/getting_started.md#L721-L727)), and resets the web-UI login with `sunshine --creds {new-username} {new-password}` ([troubleshooting.md line 10](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/docs/troubleshooting.md#L10)).
- Name shown in Moonlight: `sunshine_name`, default the PC's host name (configuration.md line 158).

### b) App-id and title

- Not a window program for the user. **UNKNOWN** whether it opens any window (it has a tray icon option, `system_tray`, [config.cpp line 1865](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/src/config.cpp#L1865)); not run.

### c) What happens when the client closes

- Per the Moonlight FAQ: the session stays running on the host after the client exits or the window is closed; "Quit App" ends it ([FAQ lines 88-94](https://github.com/moonlight-stream/moonlight-docs/wiki/Frequently-Asked-Questions)). On Sunshine's side, each app entry has `auto-detach` (default true), `wait-all` (default true) and `exit-timeout` (default 5) settings that decide what happens when an app's process exits or a quit is requested ([Apps.vue lines 282-304 and 802-804](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/src_assets/common/assets/web/Apps.vue#L282-L304); the documents under `docs/` do not describe them in prose — UNKNOWN beyond the field names and defaults).

### d) Single instance

- Not applicable for viewing. For streaming, how Sunshine treats a second client or a second session for the same app: UNKNOWN (the RTSP server counts active sessions, `src/rtsp.cpp` lines 700-720, but no rule was read that refuses a second one).

### e) Ports — every one, with its offset from the base port

The base port is the `port` setting, default **47989**, allowed range 1029-65514 ([configuration.md lines 1611-1637](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/docs/configuration.md#L1611-L1637)); every other port is "base + offset" (`net::map_port`, [`src/network.cpp` lines 245-249](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/src/network.cpp#L245-L249)). With the default base:

| Purpose | Protocol | Offset | Port | Source |
|---|---|---|---|---|
| HTTPS (pairing and encrypted requests) | TCP | -5 | 47984 | [`src/nvhttp.h` line 52](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/src/nvhttp.h#L52) |
| HTTP (the base port: server info, launch) | TCP | 0 | 47989 | [`src/nvhttp.h` line 47](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/src/nvhttp.h#L47) |
| Web UI (HTTPS, self-signed) | TCP | +1 | 47990 | [`src/confighttp.h` line 29](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/src/confighttp.h#L29); [getting_started.md line 684](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/docs/getting_started.md#L684) |
| Video | UDP | +9 | 47998 | [`src/stream.h` line 19](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/src/stream.h#L19) |
| Control | UDP | +10 | 47999 | `src/stream.h` line 20 |
| Audio | UDP | +11 | 48000 | `src/stream.h` line 21 |
| RTSP setup | TCP | +21 | 48010 | [`src/rtsp.h` line 15](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/src/rtsp.h#L15) |

(The UDP ports are inferred as UDP from "UDP stream" in the code comments; the offsets are SOURCE. Whether more ports exist for features not read — for example a microphone channel — is UNKNOWN; none was found in the files above.)

- **Which one for a bare TCP connect check:** the **base port, 47989 (TCP)**. BELIEVED: it is served by the HTTP server that starts with Sunshine ([`nvhttp.cpp` lines 1616-1617, 1726](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/src/nvhttp.cpp#L1616-L1617)) and is where every Moonlight client first connects. The web UI port (+1) means "the web UI is up", which is a different thing. 47984 and 48010 would only prove their own servers. Whether a bare connect-and-close upsets Sunshine is UNKNOWN (already on the unverified list in `HUB-OS.md`).
- **Changing it:** set `port = N` in the config file or in the web UI; every port above moves with it.

### f) What it needs (host side, for the nodes)

- **Capture on Linux (SOURCE, [configuration.md lines 2194-2250](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/docs/configuration.md#L2194-L2250)):** NvFBC (NVIDIA, X11), `wlr` (wlroots compositors, via wlr-screencopy), `kms` (needs `cap_sys_admin`), `kwin`, `x11`; the first that works is used. HDR needs the KMS method (getting_started.md line 800).
- **Input:** virtual input through `/dev/uinput`; the user must be in the `input` group (getting_started.md lines 536-538, 763).
- **Audio:** it captures from an audio sink; the documents show how to find the sink name with PulseAudio or PipeWire's PulseAudio tools (configuration.md line 793 on).
- **glibc:** the AppImage is built on Ubuntu 22.04 and needs glibc 2.35 or newer (getting_started.md lines 128-131).
- **systemd:** not required to run; a user service is offered (lines 548-564). Whether it needs D-Bus (tray, notifications) when started without a session: UNKNOWN.
- **Ubuntu 24.04 archive:** **not there** (TESTED: `apt-cache policy sunshine`, no output). The project provides releases, an AppImage, Flatpak and distribution packages; building from source is the other route ([building.md](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/docs/building.md)).

### g) Pairing and credentials

- **Pairing:** the client shows a PIN; on the host it is entered in the web UI ("PIN", plus a device name) ([getting_started.md lines 703-708](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/docs/getting_started.md#L703-L708)) or through the web API `POST /api/pin` ([api.md line 88](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/docs/api.md#L88)); clients are un-paired with `POST /api/clients/unpair` or `unpair-all` (api.md lines 58-62).
- **First run:** the web UI asks for a user name and password to be created, and the document warns to note them ([getting_started.md lines 684-692](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/docs/getting_started.md#L684-L692)). They are kept in `credentials_file`, default `sunshine_state.json` ([configuration.md lines 1909-1930](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/docs/configuration.md#L1909-L1930)), which is also the default for `file_state` (lines 2006-2025: "current state of Sunshine").
- **Keys and certificate:** the settings `pkey` and `cert` name the private key and certificate used for the web UI and for pairing; RSA-2048 is the compatible choice (lines 1956-2005). The default config folder on Linux is `~/.config/sunshine` ([configuration.md lines 22-32](https://github.com/LizardByte/Sunshine/blob/dfa9e884978398f23d2f63dd987b9d554fa4d68e/docs/configuration.md#L22-L32)). The file names inside it and the exact place the list of paired clients lives: UNKNOWN beyond the above (not read in the code).
- **Must stay out of git and the inventory:** the private key, the certificate, `sunshine_state.json` (it holds credentials), the list of paired clients, the web-UI user name and password, any PIN.

### h) Clipboard, audio, keyboard capture

- Nothing about a host-to-client clipboard was found in the documents read. Audio: streamed to the client by default; host-side audio sinks are configurable (configuration.md line 793 on). The Sunshine document says Wayland does not let clients capture the Windows key (line 762).

---

## 3. virt-viewer / remote-viewer (SPICE and VNC)

**Role:** the viewer for VM guests (`spice`, `vnc`). Package `virt-viewer` 11.0-3build2 in Ubuntu 24.04 (TESTED: `apt-cache policy virt-viewer`). Man page: [remote-viewer(1)](https://manpages.ubuntu.com/manpages/noble/man1/remote-viewer.1.html) (also `remote-viewer --help-all`, TESTED); project: [gitlab.com/virt-viewer/virt-viewer](https://gitlab.com/virt-viewer/virt-viewer).

### a) Command line

- **Form (SOURCE, man page synopsis):** `remote-viewer [OPTIONS] -- [URI]` where the URI is `spice://host:port`, `vnc://host:port`, an oVirt URI, or a connection file. Examples in the man page: `remote-viewer spice://makai:5900`, `remote-viewer vnc://tsingy:5900`.
- **Options (TESTED `--help-all`, SOURCE man page):** `-t, --title TITLE` (window title), `-f, --full-screen`, `-z, --zoom PCT`, `--auto-resize <always|never>`, `-k, --kiosk` (fullscreen with minimal UI) and `--kiosk-quit=<never|on-disconnect>`, `-H, --hotkeys` (for example `release-cursor=ctrl+alt`), `-s, --shared`, `--cursor local|auto`, SPICE options such as `--spice-disable-audio`, `--spice-shared-dir`.
- **App-id:** **yes, through GTK's `--name`** (listed under "GTK+ Options" in `--help-all` as "Program name as used by the window manager"). **TESTED:** `remote-viewer --name=hubos-ai-1 -t "AI Box" vnc://127.0.0.1:5997` → driftwm shows `app_id='hubos-ai-1'`. `--class` made no difference to the app-id (TESTED: with `--class=hubosclass --name=hubosname` the app-id was `hubosname`). Different name per machine: yes, one `--name` per command.
- **Title:** `-t "AI Box"` → TESTED title **`AI Box (1)`** (the " (1)" is added by remote-viewer, the display number); without `-t` the title is the guest's name from the server plus " (1)" (TESTED with the fake server named `fake-guest`: `fake-guest (1)`).

### b) App-id and title it really reports (TESTED, nested driftwm)

| Command | After 2 s, driftwm `state` |
|---|---|
| `remote-viewer vnc://127.0.0.1:5997` (fake guest named fake-guest) | `app_id='remote-viewer' title='fake-guest (1)' size=[1024, 815]` |
| `remote-viewer --name=hubos-ai-1 -t "AI Box" vnc://127.0.0.1:5997` | `app_id='hubos-ai-1' title='AI Box (1)'` |
| `remote-viewer vnc://127.0.0.1:5999` (**nothing listening**) | one small window `app_id='remote-viewer' title='' size=[595, 169]`; its log line: `vnc-session: got vnc error Unable to connect to 127.0.0.1:5999: Connection refused` |
| `remote-viewer --name=hubos-x -t X -- spice://127.0.0.1:5989` (**nothing listening**) | one small window `app_id='hubos-x' title='' size=[655, 113]` (the error dialog; the title option is not used for it) |

- **A window appears even when the connection fails**, but it is a small error dialog with an **empty title**, not the viewer window. hubd's comparison matching would take that dialog for the machine's window. With `--name` it carries the chosen app-id (so name matching would match the dialog too).

### c) Closing the window

- Closing the viewer window (driftwm `close`) first shows a dialog: **"Do you want to close the session?"** with "Do not ask me again" and Cancel/OK (TESTED, screenshot described: the dialog appears as a second window `size=[393, 155]` with the same app-id and an empty title, over the viewer window). Only OK closes. Nothing was said by the documents about what happens to the guest: closing the viewer does not stop the guest (BELIEVED: the viewer is only a client; the man page describes it as a client).
- `hubd end` would therefore see "still open" until someone presses OK; this is the case `hubd end` already reports as "asked the window to close, but it is still open".

### d) Single instance

- **No.** **TESTED:** two `remote-viewer --name=hubos-ai-1 vnc://127.0.0.1:5997` started four seconds apart gave **two processes and two windows**, both with app-id `hubos-ai-1` and title `fake-guest (1)`. (This is also the duplicate-name case hubd warns about.)

### e) Ports

- **SPICE and VNC console ports** are not fixed: **libvirt** — the `port` attribute is the TCP port, `-1` or `autoport='yes'` means "auto-allocated", and auto-allocation uses a range that starts at 5900 and ends at 65535 by default (`remote_display_port_min/max` in `qemu.conf`) ([libvirt formatdomain, graphics](https://libvirt.org/formatdomain.html#graphical-framebuffers); [qemu.conf.in lines 495-502](https://gitlab.com/libvirt/libvirt/-/blob/master/src/qemu/qemu.conf.in#L495-L502), fetched 2026-10-01). So a guest's port is **assigned per guest by the host** unless the guest's XML fixes it; `virsh domdisplay` prints the address ([SPICE user manual](https://www.spice-space.org/spice-user-manual.html)). **Plain QEMU:** VNC display N listens on TCP 5900+N by convention ([QEMU invocation, `-vnc`](https://www.qemu.org/docs/master/system/invocation.html)); `-spice port=…` takes the port from the person who starts it (the SPICE manual's examples use 3001, and 5900 with TLS on 5901).
- Hub OS will run its own management on KVM (HUB-OS.md, VM host), so the choice (fixed per guest or auto) is a design decision for the VM host; the inventory's `port` per guest is what the health check and the viewer need either way.

### f) What it needs on the hub

- Packages (TESTED, apt): `virt-viewer` 11.0 depends on GTK 3, `gtk-vnc` and `spice-gtk` libraries; direct dependencies on systemd: none (TESTED: `apt-cache depends virt-viewer | grep -i systemd`, no output; indirect ones through other libraries were not checked). GTK ran on Wayland under driftwm with `GDK_BACKEND=wayland` (TESTED). It needs the session D-Bus only for accessibility warnings (the AT-SPI warning was printed and ignored in every run). Audio through PulseAudio/GStreamer (BELIEVED, spice-gtk; not run). In the Ubuntu 24.04 archive: **yes**.

### g) Pairing and credentials

- No pairing. A VNC or SPICE password is given at the prompt or in a connection file; the libvirt `passwd` attribute is a clear-text password in the guest's XML (libvirt document above). **Out of git and the inventory:** any VNC/SPICE password and any connection file that holds one.

### h) Clipboard, audio, keyboard capture

- Release the mouse: default Ctrl_L+Alt_L, changeable with `--hotkeys` (man page). Hotkeys only work when the guest display widget does not have focus unless set with `--hotkeys`. Clipboard sharing is possible with the SPICE guest agent; the libvirt document shows a `<clipboard copypaste='no'/>` switch for SPICE. Audio: SPICE supports it (`--spice-disable-audio` exists); VNC through this client: UNKNOWN.

---

## 4. Remmina

**Role:** an alternative viewer for VNC and SPICE guests (and RDP, SSH, which Hub OS does not plan). Package `remmina` 1.4.43+dfsg-0ubuntu0.24.04.2 plus `remmina-plugin-vnc`, `-spice`, `-rdp` (TESTED: `apt-cache policy`). Man page [remmina(1)](https://manpages.ubuntu.com/manpages/noble/man1/remmina.1.html); [Remmina wiki](https://gitlab.com/Remmina/Remmina/-/wikis/home).

### a) Command line

- **Form (TESTED `remmina --help-all`, SOURCE wiki [command line examples](https://gitlab.com/Remmina/Remmina/-/wikis/Usage/Remmina-command-line-examples)):** `remmina -c FILE.remmina`, or a quick-connect URI: `remmina -c vnc://server`, `rdp://user@server`, `ssh://user@server`, also `spice://`. `--enable-fullscreen`, `--disable-toolbar`, `--no-tray-icon`, `--disable-news`, `--disable-stats`, `--enable-extra-hardening` (turns off the closing confirmation, among other things), `-k` kiosk, `-q` quit a running instance.
- **App-id:** **default `org.remmina.Remmina`** (TESTED). **`--name=<x>` changes it** (TESTED: `--name=hubos-a` → `app_id='hubos-a'`). **`--gapplication-app-id=<id>` changes the unique application id** ("Override the application's ID", `--help-all`), which decides whether a second start is a separate instance (see d).
- **Title (TESTED):** the connection window is titled with the profile name, which for a quick-connect URI is `host:port` (`127.0.0.1:5997`). With a `.remmina` profile it is the profile's name field (BELIEVED, from the profile format in the wiki [Remmina Config File Options](https://gitlab.com/Remmina/Remmina/-/wikis/Remmina-Config-File-Options); not run).

### b) What it really reports (TESTED, nested driftwm, plug-ins in a private mount namespace)

| Command | driftwm `state` |
|---|---|
| `remmina -c vnc://127.0.0.1:5997` | `app_id='org.remmina.Remmina' title='127.0.0.1:5997' size=[640, 518]` |
| `remmina --name=hubos-a --gapplication-app-id=org.hubos.a -c vnc://127.0.0.1:5997`, then the same with `hubos-b` / `org.hubos.b` | two windows: `app_id='hubos-a'` and `app_id='hubos-b'`, both titled `127.0.0.1:5997` |
| `remmina -c vnc://127.0.0.1:5990` (**nothing listening**) | a normal window `app_id='org.remmina.Remmina' title='127.0.0.1:5990' size=[640, 518]`; **inside it** the line "Unable to connect to VNC server" and a Close button (screenshot described) |
| the same without the plug-ins available | a small dialog `size=[402, 113]`, text "Install the VNC protocol plugin first." |

- A **window always appears**, even when the connection fails; the error is inside the window.
- Remmina's log said it runs "without a secret plugin" (passwords saved in a less secure way) — TESTED output line.

### c) Closing the window

- Closing asks: **"Are you sure you want to close 2 active connections in the current window?"** (TESTED, screenshot), with No/Yes. The wiki preferences list "Confirm before closing multiple tabs" ([Remmina Preferences, line 29](https://gitlab.com/Remmina/Remmina/-/wikis/Remmina-Preferences)); `--enable-extra-hardening` disables the confirmation (`--help-all`). Whether closing leaves the guest running: the guest is a server; a viewer closing does not stop it (BELIEVED).

### d) Single instance — **yes**

- **TESTED:** a second `remmina -c vnc://127.0.0.1:5997` while the first runs: the **second process exited**, no second window opened, and the first window now had **two tabs** ("close 2 active connections"). So the default behaviour is a hand-over: a clean exit of the second process and a new tab in the old window. This is exactly the clean-exit case the late-window rule keeps waiting for.
- With a different `--gapplication-app-id` per machine the instances are separate (TESTED: two processes, two windows). With only `--name` different and the same application id, the second start was handed over and exited (TESTED). A GApplication id has to be a valid one; an id such as `org.hubos.9-lives` (an element starting with a digit) may be refused (BELIEVED from GLib's rules; not run) — the inventory allows ids that start with a digit.

### e) Ports

- Standard protocol ports: VNC 5900+display, RDP 3389 (the wiki's debugging page checks `3389` with nmap), SSH 22. Remmina's own default for each plug-in was not read (UNKNOWN); a URI with an explicit port always works (TESTED with 5997 and 5990).

### f) What it needs on the hub

- GTK 3, and one plug-in package per protocol (TESTED: they are separate packages); the RDP plug-in failed to load in the test because `libfuse3.so.3` was not unpacked (a test set-up gap, not a finding about Remmina). Remmina on Wayland: the wiki says some plug-ins need GtkSocket (X11 embedding), which does not exist on Wayland, so under pure Wayland those protocols do not embed; run with `GDK_BACKEND=x11` under XWayland if needed ([Usage FAQ, line 34](https://gitlab.com/Remmina/Remmina/-/wikis/Usage/Remmina-Usage-FAQ)). VNC and SPICE ran fine on Wayland in the test. It wanted the session D-Bus (it is a GApplication, and the hand-over uses it). In the archive: **yes**. Direct systemd dependency: none (TESTED: `apt-cache depends remmina | grep -i systemd`, no output).

### g) Pairing and credentials

- No pairing. Profiles are files `*.remmina` in `$XDG_DATA_HOME/remmina` (default `~/.local/share/remmina`) ([Usage FAQ, line 6](https://gitlab.com/Remmina/Remmina/-/wikis/Usage/Remmina-Usage-FAQ)). A password in a URI or profile is encrypted with a default algorithm that the wiki itself calls not safe, and it suggests a keyring instead ([command line examples](https://gitlab.com/Remmina/Remmina/-/wikis/Usage/Remmina-command-line-examples)). **Out of git and the inventory:** profiles with passwords, URIs with passwords, `remmina.pref` (holds the encryption secret).

### h) Clipboard, audio, keyboard

- Not read beyond the options above. UNKNOWN.

---

## 5. foot as the SSH terminal, with ssh

**Role:** the terminal for machines opened with `ssh`. `foot` 1.16.2-2ubuntu0.1 and `openssh-client` 1:9.6p1 are in the Ubuntu 24.04 archive (TESTED: `apt-cache policy`). Man pages: [foot(1)](https://manpages.ubuntu.com/manpages/noble/man1/foot.1.html), [ssh(1)](https://man.openbsd.org/ssh), [ssh_config(5)](https://man.openbsd.org/ssh_config).

### a) Command line

- **Form:** `foot --app-id=<id> --title=<text> -- ssh -p <port> -l <user> -- <host>`. `foot --help` (TESTED) lists `-a,--app-id=ID` ("window application ID", default `foot`), `-T,--title=TITLE`, `-H,--hold`, `-w,--window-size-pixels`, `-F`/`-m`; everything after `--` is the command. The man page says the same: `--app-id` sets the Wayland window's app-id; the default is `foot` (SOURCE: foot(1)).
- **Per machine:** yes — both app-id and title are options, so each machine gets its own.
- **ssh options:** `-p port` (default 22: [ssh_config(5), Port](https://man.openbsd.org/ssh_config#Port), also in the unpacked man page, TESTED), `-l user`, `-o Key=value`.
- **`--` before the host matters (TESTED):** `ssh -p 5995 -o BatchMode=yes -o ConnectTimeout=2 -- -oProxyCommand=touch` was refused with `hostname contains invalid characters`, and no file was created; without `--`, `ssh … -oProxyCommand="…" user@h` took the text as an option (its output was the connection failure of a proxy that does not connect). The inventory now refuses addresses starting with a dash; `--` is the second lock.

### b) What it reports (TESTED, nested driftwm)

- `foot --app-id=hubos-ai-1 --title="AI Box" --hold -- ssh -p 5995 -o ConnectTimeout=3 -o BatchMode=yes user@127.0.0.1` (nothing listening) → `app_id='hubos-ai-1' title='AI Box' size=[700, 525]`; the window shows the ssh error `ssh: connect to host 127.0.0.1 port 5995: Connection refused` (screenshot described).
- **The same command without `--hold`: the window disappears at once** (no window in the list after 2 s and after 5 s). So a failed ssh leaves no window and no message unless `--hold`.
- Two foot windows started with the same app-id gave two processes and two windows with that app-id (TESTED); `foot` is not single-instance (`footclient` plus `foot --server` is the shared mode; not used).

### c) Closing the window

- **TESTED:** a script started under foot received **SIGTERM** when the window was closed through driftwm, and foot exited. So closing the terminal ends the `ssh` process, which ends the remote login session (BELIEVED for the remote side: ssh exits, the server closes the session; any `tmux`/`screen` on the machine would survive; not run against a real server). This differs from HUB-OS.md's rule "closing a window leaves the machine's session alive" for desktop-style windows; for a terminal the session ends, unless the owner runs a terminal multiplexer on the machine.

### d) Single instance

- None (TESTED above).

### e) Ports

- SSH: **22/TCP** by default ([ssh_config(5)](https://man.openbsd.org/ssh_config#Port)); changed with `-p` or the inventory `port`.

### f) What it needs

- `foot`: Wayland (it needs a compositor), fontconfig/freetype, libutf8proc; no systemd dependency was observed (it ran under our compositor without a session manager; TESTED). `openssh-client`: glibc/OpenSSL. Audio, GPU: none. Both in the Ubuntu 24.04 archive.

### g) Credentials

- ssh keys and passphrases: `~/.ssh` (BELIEVED, ssh(1)); the inventory holds only a `user`. **Out of git and the inventory:** private keys, known-hosts decisions are not secret but are per-hub state.

### h) Clipboard/keyboard

- foot has its own clipboard shortcuts (not read here). Keys pass to the focused terminal; driftwm shortcuts remain (see `docs/driftwm-findings.md`).

---

## 6. A plain file manager opening an SMB share

**Role:** the stand-in for the planned file manager (`files`). `HUB-OS.md` plans a bespoke file manager "after a design discussion"; this section only records what a general-purpose one does. `pcmanfm` 1.3.2-4build2, `thunar` 4.18.8, `nautilus` 46.4 and `gvfs-backends` 1.54.4 are in the Ubuntu 24.04 archive (TESTED: `apt-cache policy`). Only `pcmanfm` was run.

### a) Command line

- `pcmanfm [OPTIONS] [FILE1, FILE2, …]` with an `smb://host/share` URI as the location (TESTED `pcmanfm --help`; [pcmanfm(1)](https://manpages.ubuntu.com/manpages/noble/man1/pcmanfm.1.html)). Options include `-n/--new-win`, `--role=ROLE`, `-p/--profile`. **There is no app-id or title option**: `--name=hubos-nas-1` did not change the app-id (TESTED, still `pcmanfm`). SMB itself is handled by GVfs (the desktop's virtual file-system layer) — [GVfs](https://gitlab.gnome.org/GNOME/gvfs) lists SMB among its back-ends (README). The port cannot be given in the URI form we used (UNKNOWN; standard port only).

### b) What it reports (TESTED)

- `pcmanfm smb://127.0.0.1/pool` with no SMB server and no GVfs service: one small window `app_id='pcmanfm' title='Error' size=[321, 123]`. `pcmanfm /var` → `app_id='pcmanfm' title='var' size=[628, 505]` (the title is the folder name). With a working SMB mount the title would be the share or folder name (BELIEVED).

### c) Closing the window

- Closing a file-manager window does not end anything on the NAS; mounts made by GVfs stay until unmounted (BELIEVED, GVfs behaviour; not run). The NAS session ends when the mount does.

### d) Single instance

- **Yes in practice.** TESTED: two commands (`pcmanfm /tmp`, then `pcmanfm /var`) both returned at once, **one `pcmanfm` process remained**, and the later command's window (`var`) appeared next to the earlier one. So the process hubd starts may exit with status 0 immediately and the window belongs to a process that was running before: exactly the case the late-window rule's clean-exit handling is for. A fixed app-id (`pcmanfm`) means name matching is impossible; comparison matching needs the window to appear after the command.

### e) Ports

- SMB over TCP **445**; NetBIOS session 139 (TCP) is the old route ([Samba AD DC port usage, table: "SMB over TCP 445 tcp", "NetBIOS Session 139 tcp"](https://wiki.samba.org/index.php/Samba_AD_DC_Port_Usage); a plain Samba file server uses the same two). NFS and SFTP are different (SFTP is ssh on 22).

### f) What it needs

- GTK 3 and GVfs with the SMB back-end (`gvfs-backends`, libsmbclient). In the test the GVfs service could not start (no D-Bus activation files for the unpacked packages), which is a test set-up limit, not a finding. Wayland: GTK 3 ran on Wayland (TESTED). No systemd dependency observed.

### g) Credentials

- SMB user name and password: typed in the file manager's dialog, optionally saved in the desktop keyring (BELIEVED). The inventory holds only `user` and `share`. **Out of git and the inventory:** the SMB password and any saved credential file.

### h) Clipboard/keyboard

- Not read. UNKNOWN.

---

## Input for the later "secrets" design

Only what the documents say; no mechanism is proposed here.

| Program | Secret material | Where it lives (per the sources) |
|---|---|---|
| Moonlight | client private key, client certificate, client id; hosts' certificates; the pairing PIN while pairing | Qt settings, names `key`, `certificate`, `uniqueid` (identitymanager.cpp); the file location is Qt's default (UNKNOWN exactly) |
| Sunshine | server private key and certificate; web-UI user name and password; list of paired clients | `~/.config/sunshine` by default; files named by `pkey`, `cert`, `credentials_file`, `file_state` (default `sunshine_state.json`) |
| virt-viewer | VNC/SPICE passwords; connection files that contain them | typed at a prompt, or in a connection file; libvirt's `passwd` in the guest XML is clear text |
| Remmina | profiles with passwords; its encryption secret | `$XDG_DATA_HOME/remmina/*.remmina`; `remmina.pref`; the wiki calls the default encryption not safe |
| ssh | private keys and passphrases | `~/.ssh` |
| SMB | user name and password for the share | the file manager's dialog or keyring |

`HUB-OS.md` says secrets never go in the inventory or in git and that per-machine secrets live in the per-machine config outside the image. These programs each keep their secrets in their own files, so the secrets design has to decide which of those files are part of the per-machine config and its NAS backup (the NAS backup rule says secrets do not get the NAS backup copy).

---

## PROPOSED, UNVERIFIED on hardware, for the owner to approve

### Proposal 1 — a `viewers.toml` entry for each real viewer

Written in the format of `docs/hubd-slice2.md` section 2.1. Every value comes from the sections above; anything that could not be confirmed is marked `PLACEHOLDER` and must be decided before use. **None of this has been run against a real machine.** `moonlight` is shown with the tested limit that its window cannot carry a chosen name.

```toml
format = 1

# --- ssh machines: foot terminal running ssh ----------------------------------------
# TESTED: foot sets app-id and title; ssh stops option-like hosts after "--".
# {user} and {port} are required: a machine without them is refused by hubd.
[[viewer]]
id          = "ssh-foot"
programs    = ["ssh"]
command     = ["foot", "--app-id={app_id}", "--title={title}", "--hold", "--",
               "ssh", "-p", "{port}", "-l", "{user}", "--", "{address}"]
sets_name   = true
# --hold keeps the window (with the error) when ssh fails; a failed ssh would
# otherwise leave no window and no message (TESTED).

# --- spice guests ----------------------------------------------------------------------
# TESTED (with vnc://): --name sets the app-id, -t sets the title (shown with " (1)").
# A spice:// connection itself was not run (no SPICE server was available).
[[viewer]]
id          = "remote-viewer-spice"
programs    = ["spice"]
command     = ["remote-viewer", "--name={app_id}", "--title={title}", "--", "spice://{address}:{port}"]
sets_name   = true

# --- vnc guests --------------------------------------------------------------------------
[[viewer]]
id          = "remote-viewer-vnc"
programs    = ["vnc"]
command     = ["remote-viewer", "--name={app_id}", "--title={title}", "--", "vnc://{address}:{port}"]
sets_name   = true

# --- moonlight machines (gaming, ai, desktop) ----------------------------------------
# SOURCE only (not run). The window cannot carry a chosen name (fixed app-id
# com.moonlight_stream.Moonlight), so it is matched by comparison. The host
# must already be paired. {session} is NOT an existing placeholder: it stands for
# the Sunshine app name to stream, which the inventory does not hold yet.
# --capture-system-keys and --quit-after are the values we think fit; the owner decides.
[[viewer]]
id          = "moonlight"
programs    = ["moonlight"]
command     = ["moonlight", "stream", "{address}:{port}", "{session}",
               "--display-mode", "windowed", "--no-quit-after",
               "--capture-system-keys", "never"]
sets_name   = false
window_wait = "30s"      # PLACEHOLDER: the search for the host alone may take up to 30 s (startstream.cpp)
late_grace  = "60s"      # PLACEHOLDER

# --- files (SMB share) ---------------------------------------------------------------------
# NOT PROPOSED as is. A generic file manager has a fixed app-id and hands over to a
# running copy (TESTED with pcmanfm). The planned Hub OS file manager is a separate
# design discussion (HUB-OS.md). Shown only so the shape is on record:
# [[viewer]]
# id          = "files-pcmanfm"
# programs    = ["files"]
# command     = ["pcmanfm", "smb://{address}/{share}"]
# sets_name   = false
```

- **Remmina** is left out on purpose: the same guests are covered by `remote-viewer`, and Remmina's default is a hand-over into one shared window (TESTED). If the owner prefers it, the entry would be `["remmina", "--name={app_id}", "--gapplication-app-id=org.hubos.m{id}", "-c", "vnc://{address}:{port}"]` with `sets_name = true`, and **three things to settle first**: a GApplication id built from a machine id may be invalid when the id starts with a digit, a close asks for confirmation, and there is no `--` protection for the URI.
- **Needs from hubd (not built):** matching by **title** for Moonlight (the title is "<host name> - Moonlight", where the host name is Sunshine's `sunshine_name`, which would have to be set to our machine name on every node); a `{session}` placeholder or inventory field; per-viewer `--hold`-like behaviour already exists for foot.

### Proposal 2 — default ports per kind of machine, for the health check

hubd today has **no** default ports (owner rule). This table is the input for a decision to give it some, per `open` program. Source column gives the evidence; "TCP connect" is the check slice 1 makes (`internal/probe`).

| Kind of machine | `open` first entry | Check TCP port | Why | Source |
|---|---|---|---|---|
| gaming box, AI box, general desktop (Sunshine) | `moonlight` | **47989** (base port; moves with Sunshine's `port` setting) | the HTTP server every Moonlight client talks to first; the other TCP ports prove other servers | Sunshine `nvhttp.h` line 47, `configuration.md` lines 1611-1637 (BELIEVED that it is up whenever Sunshine runs) |
| VM host, ssh machines | `ssh` | **22** | OpenSSH default | ssh_config(5) (TESTED in the unpacked man page) |
| NAS, backup NAS (files) | `files` | **445** | SMB over TCP | Samba port table (SOURCE link above) |
| NAS, backup NAS, if opened by ssh/sftp | `ssh` | **22** | OpenSSH default | ssh_config(5) |
| VM guest, SPICE | `spice` | **none to default**: per guest (assigned from 5900 upwards by libvirt's auto-allocation, or fixed by our VM host management) | no fixed port exists | libvirt formatdomain; `qemu.conf.in` lines 495-502; SPICE user manual |
| VM guest, VNC | `vnc` | **none to default**: per guest; QEMU's convention is 5900 + display number | as above | libvirt formatdomain; QEMU invocation (`-vnc`) |
| the hub | `none` | not checked | it is the machine hubd runs on | HUB-OS.md |

---

## Unknowns still open

- A real Moonlight and a real Sunshine were never run: the app-id and title (and what happens when the host is unreachable or unpaired) are SOURCE only; whether two streams to the same host can run, and what Sunshine does then, is UNKNOWN.
- The exact app name to stream (`{session}`), and whether to give every Sunshine node one fixed app.
- Where Qt keeps Moonlight's settings file on a no-desktop Hub OS image, and where exactly Sunshine keeps its paired-client list.
- SPICE over `remote-viewer` was not run end to end; Remmina's SPICE/RDP plug-ins were not run.
- Whether the Moonlight window reacts to driftwm's `close` the way the other viewers do (it may end the stream at once, or ask).
- Clipboard: Moonlight types local text on the host; whether host-to-client copy exists in Sunshine/Moonlight at all.
- Whether a bare TCP connect-and-close to Sunshine's 47989, SPICE or VNC upsets those programs (already in `HUB-OS.md`).
- Wayland-only behaviour of the file manager's SMB access without GVfs services started by hand.

## Questions for the owner

1. Moonlight's window cannot carry a chosen name and its title is the **host's** name (Sunshine's `sunshine_name`), not ours. Do you want hubd to learn **title matching**, with `sunshine_name` set to our machine name on every node? Otherwise two Moonlight windows can only be told apart by order of arrival.
2. Which Sunshine app name should each machine stream (a `session` field in the inventory, or one fixed name)?
3. For **ssh machines**: a terminal ends the remote session when closed (TESTED: foot sends SIGTERM to ssh). Accept that, or run a terminal multiplexer on the machines so a closed window keeps the session?
4. For **spice/vnc guests**: `remote-viewer` (not single-instance, asks "close the session?") or Remmina (single-instance, tabs)? The proposal uses `remote-viewer`.
5. Should the health check get **default ports per program** from Proposal 2 (47989, 22, 445), with the inventory `port` overriding, or stay "no defaults"?
