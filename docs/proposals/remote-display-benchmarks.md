# EXPERIMENT AND REPORT: remote display benchmarks (Part 3)

**Status: measurements and a repeatable harness. Nothing here is decided and nothing here is in an image.** `HUB-OS.md` wins if this file disagrees with it. Written 2026-10-05 by a helper agent (Claude, session `session_01Kb6L66wDxnkQ4pMAxMZR4E`); the lead opens the pull request. The harness is `tools/bench/remote-display/`; the raw results are `tools/bench/remote-display/results/raw/` (one text file per run) and `tools/bench/remote-display/results/summary.md` (all tables, generated).

**Labels on every item** (same as `docs/proposals/remote-display.md`):
- **TESTED**: I ran it in this build container; the command is in section 10 or the item itself, and the output is in `results/raw/`.
- **SOURCE**: read in a primary file (raw source file or git history); the link or path and the date read (2026-10-05) are given. I read these through `curl` on raw files or in a local clone. **I used no summarising fetch tool and no search engine in this part.**
- **BELIEVED**: reasoning or memory, not run and not read in a source. Said so each time.
- **UNKNOWN**: nobody checked.

**What this is not.** This container has 4 virtual cores, no GPU, no real display, no real network (loopback only). **Every number is a software-rendering number on shared cores.** It says which stack costs more than which, and which behaviours exist. It does not say how fast anything is on the December hardware. Section 4 lists what cannot be measured here and why.

Parts of this report that came from earlier helper agents and were **not re-read by me**: the Phase B/round 1/round 2 findings that are quoted from `docs/proposals/remote-display.md` and `docs/driftwm-findings.md` (each is marked "repo doc"). Everything else was done in this session. I used no sub-helpers.

---

## 1. Plain-words summary (the ten findings that matter most)

The numbers are for 1080p, wayvnc on sway unless another stack is named; the tables in section 5 have every cell with N, min, median and max.

1. **An idle window costs nothing.** In all four stacks and all five viewers an unchanged screen sent **0 bytes** in 15 s and used **0 % CPU** on the node (TESTED, section 5.1). Twenty idle sessions into one hub cost the hub **1 % of one core (wlvncc) or 0 % (the others)** (TESTED, section 8). Memory is the cost, not CPU.
2. **Moving content is expensive in software, and the viewer you choose changes the hub's cost by more than a factor of four.** Scrolling text at 30 lines a second cost the hub 25 % of a core with **wlvncc**, 39 % with the **TigerVNC viewer**, and **110 % (a whole core, saturated) with `remote-viewer`**; the node paid 1.0 to 1.3 cores in all three (TESTED, table "scroll, 1080p"). With 19 idle windows and one busy one, the hub paid 19 %, 35 % and 79 % (TESTED, section 8).
3. **`remote-viewer` cannot keep up with busy windows.** At 1080p it showed 20 to 24 of 30 video frames a second and dropped **22 to 39 %** of them; wlvncc dropped 5 to 16 %, the TigerVNC viewer 0 to 6 % and Weston RDP none (TESTED, "video: dropped frames"). It is the only viewer that is exactly lossless in the text test (SSIM 1.0); it also never sends the clipboard hub to node (neither does wlvncc) (TESTED).
4. **Only the TigerVNC viewer and `xfreerdp3` carry the clipboard both ways, including non-ASCII text, and both are X11 programs.** `remote-viewer` carries node to hub only and garbles non-ASCII text; **wlvncc has no clipboard code at all** (SOURCE and TESTED); `wlfreerdp3` carries node to hub only; Weston's VNC backend has no clipboard (SOURCE and TESTED). The TigerVNC viewer needs Xwayland and `xkbcomp`, and 20 TigerVNC windows took about **2.7 GB** of memory on the hub against about 1.0 GB for wlvncc and 1.5 GB for `remote-viewer` (TESTED, section 8).
5. **Weston's RDP is the smoothest and the slowest to react.** It dropped no video frames and its text was close to lossless (SSIM 0.998, PSNR 42 dB), but a click took **67 ms (xfreerdp3) or 131 ms (wlfreerdp3)** to show on the hub against **43 ms for wayvnc** and **26 ms for Weston's VNC** at 1080p; at 4K these were 262/341 ms against 130 ms and 67 ms (TESTED). Its encoder is the CPU only: the Weston 13.0 and 16.0 source has no H.264 or GPU path (SOURCE, section 9). `wlfreerdp3` is marked deprecated by its authors (TESTED).
6. **wayvnc is capped at 30 frames a second by default and uses about a core for moving content.** It is the stack with the most complete features (clipboard, desktop name, client-asked resize) but at 1080p its input-to-pixel delay was 43 ms and it kept 5 to 11 % of the video frames from showing (TESTED; the cap is `-f`, default 30, SOURCE `src/main.c:2482`). At 4K the same stack took 130 ms and the frame rate fell (the hub saw 5 to 7 % CPU in the drag test, meaning few frames arrived).
7. **No viewer reconnects by itself; started again by hand, they are back in 0.1 to 0.5 s (except wlvncc against Weston VNC, 1.1 s, because of its login command).** After the connection was cut, **none of the five viewers opened a new connection within 15 s** (TESTED); starting a fresh viewer showed the node's screen again in 110 ms (wlvncc), 127 to 140 ms (TigerVNC), 145 to 164 ms (FreeRDP) or 343 to 499 ms (`remote-viewer`) on wayvnc and sway (TESTED). hubd must restart the viewer itself, which `HUB-OS.md` already says ("closing a window leaves the session alive").
8. **A frozen viewer does not make wayvnc quiet; it does make Weston RDP quiet.** With the viewer stopped, wayvnc sent 0 bytes but its node CPU stayed at about 50 % (it was 65 to 82 % while the viewer ran); Weston VNC fell from 60 to 70 % to 28 to 31 %; **Weston RDP fell to 0 % and its application stopped too** (TESTED, "does the server go quiet").
9. **What happens when a viewer asks for a smaller screen differs a lot.** The TigerVNC viewer made both wayvnc and Weston VNC shrink the node's screen to 960 x 540; `remote-viewer` made wayvnc and Weston VNC change it to 1920 x 1033; wlvncc and both FreeRDP clients (as I started them, without `/dynamic-resolution`) only scaled the picture in the window (TESTED). **A viewer that asks cage 0.1.5 for a new size crashes cage** (TESTED); cage also cannot run at 1080p or 4K here and has no clipboard (no data-control protocol) (TESTED).
10. **The GPU question is open and cannot be measured here.** neatvnc's H.264 encoder (VAAPI or V4L2) is only used when frames are GPU buffers and the viewer asks for H.264; wlvncc decodes it with VAAPI, the TigerVNC viewer in software, `remote-viewer` not at all; Weston's RDP has no GPU path (SOURCE, section 9). I give no numbers for it.

**What I recommend you read first:** the short table in section 5.0, then section 6 (wlvncc), then section 12 (questions). I do not choose a protocol here; `HUB-OS.md` and the owner decide.

---

## 2. What was built

Everything is in `tools/bench/remote-display/`. Nothing here is run by `go test` (except the small SSIM program's unit test) or by any build script, and nothing goes into an image.

```
tools/bench/remote-display/
  setup.sh            downloads the .deb files, unpacks them with dpkg -x into /tmp/bench, builds wayvnc, neatvnc, aml, wlvncc from pinned tags
  env.sh              PATH / LD_LIBRARY_PATH for the private tool folder (nothing is installed)
  run.sh              ONE measurement:  run.sh STACK CLIENT SCENE RES [RUN]      (RES = 720p | 1080p | 4k)
  run-all.sh          a part of the matrix, many run.sh calls (skips finished cells)
  campaign.sh         the exact campaign behind this document (resumable)
  scale.sh            the 20-session test
  summarize.py        raw results -> results/summary.md (min / median / max, N)
  versions.sh         exact versions
  stacks/             ONE FILE PER NODE STACK     wayvnc-sway.sh  wayvnc-cage.sh  weston-rdp.sh  weston-vnc.sh
  clients/            ONE FILE PER VIEWER         wlvncc.sh  tigervnc.sh  remote-viewer.sh  freerdp-wl.sh  freerdp-x11.sh
  scenes/             idle scroll term drag video  (the load)  and  latency fidelity reconnect quiet smallsize clipboard resize  (the questions)
  metrics/            pixelwatch.c  hubclick.c  tcpmeter.py  procstat.py  analyze.py  build-tools.sh
  scenes/benchapp.c   the test window that runs on the node
  ssim/               Go program (standard library only) + unit test: SSIM and PSNR of two PNG pictures
  lib/                common.sh (helpers)  ovl.sh (private mount overlay, see 3.3)
  results/raw/        one text file per run (key=value); results/summary.md is made from them
```

**Adding a stack or a client is a one-file change.** A stack file defines `STACK_DESC`, `STACK_PROTO` (vnc or rdp), `STACK_CAN_RESIZE`, and three functions: `stack_start IDX W H APP...`, `stack_capture FILE.png` and `stack_versions` (the contract is written at the top of `stacks/wayvnc-sway.sh`; `stack_resize` only if the stack can). A client file defines `CLIENT_DESC`, `CLIENT_PROTO`, `client_version` and `client_start HOST PORT W H`. `run-all.sh` and `run.sh` find the files by name, so a new file is used by the next run. (TESTED: the four stacks and five clients here are each one such file; the cage stack was added after the sway stack by copying that file and changing about ten lines.)

**How a run works.**

1. A second headless **sway** is the **hub**. Its output has the size of the node's screen and every window on it is made fullscreen, so one hub pixel is one node pixel. (It stands in for driftwm, which was not built for this work; see 4.)
2. The **node** is the stack under test, started with the scene's program (`foot` for text, `mpv` for video, `benchapp` for the others).
3. A counting relay (`tcpmeter.py`) sits between viewer and server on loopback and counts the bytes in both directions (after encryption, if any). It also keeps the first 4 KB the viewer sent, to read which picture encodings it asked for.
4. The viewer is started on the hub. Time to first frame is the time from starting the viewer to the first hub frame in which one pixel has the node's colour (measured by `pixelwatch`, 60 samples a second, so good to about 17 ms).
5. The scene runs. CPU and memory come from `/proc` (utime + stime of the process and its children, memory is resident set size) at the start and end of the window.
6. The result is one small text file; the run is not repeated if the file exists.


---

## 3. Set-up: how the programs were obtained and exactly which versions

### 3.1 Nothing was installed

Every program was downloaded as a `.deb` file (the URLs come from `apt-get install --print-uris`, which installs nothing; fetched with `curl` through the session's proxy from the Ubuntu 24.04 "noble" archive) and unpacked with `dpkg -x` into `/tmp/bench/a/root` (TESTED, `setup.sh debs`). wayvnc, neatvnc, aml and wlvncc were built from source with meson (installed with `pip install --target /tmp/bench/py`, a private folder) and ninja into `/tmp/bench/prefix` (TESTED, `setup.sh build`). Programs run with `PATH` and `LD_LIBRARY_PATH` pointing there. The C helpers (`benchapp`, `pixelwatch`, `hubclick`) are compiled by `metrics/build-tools.sh` against the unpacked headers. All of it is under `/tmp/bench` and is deleted at the end (section 10).

### 3.2 Exact versions (output of `./versions.sh`, 2026-10-05)

```
# versions used, 2026-10-05
kernel: Linux 6.18.44-fc-v70   cpu: Intel(R) Xeon(R) Processor @ 2.10GHz   cores: 4
deb sway 1.9-1build2
deb cage 0.1.5+20240127-2build1
deb weston 13.0.0-4build3
deb libweston-13-0 13.0.0-4build3
deb libwlroots12t64 0.17.1-2.1build1
deb libneatvnc0 0.7.1+dfsg-2build3
deb freerdp3-wayland 3.32.0+dfsg-0ubuntu0.24.04.1
deb freerdp3-x11 3.32.0+dfsg-0ubuntu0.24.04.1
deb libfreerdp3-3 3.32.0+dfsg-0ubuntu0.24.04.1
deb tigervnc-viewer 1.13.1+dfsg-2build2
deb virt-viewer 11.0-3build2
deb libgtk-vnc-2.0-0 1.3.1-1build2
deb foot 1.16.2-2ubuntu0.1
deb mpv 0.37.0-1ubuntu4
deb grim 1.4.0+ds-2build2
deb wl-clipboard 2.2.1-1build1
deb wlr-randr 0.3.0-1
deb xwayland 2:23.2.6-1ubuntu0.8
deb x11-xkb-utils 7.7+8build2
source wayvnc v0.10.2 bb837459c75fd3c267d48af170ac7ed2566f67e5 2026-09-25
source neatvnc v1.0.3 783437adc56c2d7ce2cac1ec584acad798594267 2026-10-03
source aml v1.0.0 685035c9830aa89df02a43df89b644690bd885f5 2025-07-27
source wlvncc with-aml-v0.3.0-2-gcc0abf8 cc0abf87c37920540f2439a556e6a480c28f8f46 2026-04-29
built: wayvnc: v0.10.2-bb83745 (HEAD) neatvnc: v1.0.3-783437a (HEAD) aml: v1.0.0-0-g685035c (HEAD) 
tools: go version go1.24.7 linux/amd64 ; gcc (Ubuntu 13.3.0-6ubuntu2~24.04.1) 13.3.0 ; python Python 3.11.15 ; ffmpeg 6.1.1-3ubuntu5
```

- wayvnc and neatvnc are the **pinned tags** `HUB-OS.md` names: **wayvnc v0.10.2** (`bb837459c75fd3c267d48af170ac7ed2566f67e5`, 2026-09-25) and **neatvnc v1.0.3** (`783437adc56c2d7ce2cac1ec584acad798594267`, 2026-10-03), aml v1.0.0, built with `-Dpam=disabled -Dman-pages=disabled -Dscreencopy-dmabuf=disabled -Dneatvnc:h264=disabled -Dneatvnc:gbm=disabled -Dneatvnc:examples=false -Dtests=false` (TESTED: `wayvnc --version` prints `v0.10.2-bb83745`, `neatvnc: v1.0.3-783437a`). H.264 and the GPU capture path are therefore **off**.
- **Weston's VNC backend uses Ubuntu's neatvnc 0.7.1** (`libneatvnc0 0.7.1+dfsg-2build3`), not 1.0.3: `readelf -d vnc-backend.so` lists `NEEDED libneatvnc.so.0` and our build's soname is `libneatvnc.so.1` (TESTED). So the two VNC servers differ in age as well as in compositor.
- wlvncc is the commit `cc0abf87c37920540f2439a556e6a480c28f8f46` (no release tag exists; section 6).
- Weston is the Ubuntu package 13.0.0; Weston 16.0.0 (2026-07-14, repo doc) was **not run**. FreeRDP is 3.32.0. The cage version is 0.1.5. The wlroots under sway and cage is 0.17.1.
- The hub's compositor is sway 1.9, **not driftwm** (section 4.2).

### 3.3 The private mount overlay (`lib/ovl.sh`)

Some programs look for files at fixed paths. Instead of writing to `/usr` or `/etc`, every run starts in a private mount namespace (`unshare -m`) with two read-only overlays that vanish with the process (TESTED: `ls /usr/bin/xkbcomp` fails outside, works inside):

1. `/usr/bin` also shows the unpacked programs, because **Xwayland and cage start `/usr/bin/Xwayland` by fixed path and Xwayland starts `/usr/bin/xkbcomp` by fixed path** (TESTED, section 7).
2. `/etc` also shows (a) `pam.d/weston-remote-access`, a **test-only** file that accepts every password: Weston's VNC backend checks the login with PAM service `weston-remote-access` and has no switch to turn that off (SOURCE: Weston 13.0 `libweston/backend-vnc/vnc.c`, `vnc_handle_auth` → `weston_authenticate_user`, which calls `pam_start("weston-remote-access", ...)`; it also refuses any user name other than the one it runs as, `pw->pw_uid != getuid()`); (b) `pki/CA/cacert.pem`, the self-signed test certificate, because `remote-viewer` (gtk-vnc 1.3.1) looks for trusted certificates only in `/etc/pki/CA/cacert.pem` and `~/.pki/CA/cacert.pem` (TESTED with `strace`: it opens `/root/.pki/CA/cacert.pem`, the home from the password file, and ignores `$HOME`).

The test passwords (`bench`) and the certificate are test values, not secrets; nothing secret is in the repository.

---

## 4. Method, and what this set-up cannot tell you

**TESTED set-up.** A cloud container: 4 virtual cores (Intel Xeon @ 2.10 GHz, `nproc` = 4), 16 GB memory, no GPU (`/dev/dri` does not exist), no real network (loopback only), Linux 6.18.44. Nothing else heavy ran during the measurements; the machine's load average before each run is in every raw file (`load_start`) and is repeated in `results/summary.md`. The lead did light document work in the same container; no other benchmark ran. Node and hub share the same 4 cores, so a heavy node can slow the hub and the other way round.

**What was measured, exactly.**

| Metric | How | Resolution / limit |
|---|---|---|
| Bytes per second on the socket | `tcpmeter.py` counts every byte server to viewer and viewer to server on loopback over the scene window (12 to 15 s, after a 4 to 5 s settle) | exact; after TLS if the link uses TLS |
| CPU and memory, node and hub | `/proc/<pid>/stat` utime + stime of each group and its children at the start and the end of the window; resident set size at the end. Groups: node server, node compositor, node program ("app"), hub viewer, hub compositor (with its Xwayland) | percent of one core (4 cores = 400); the 100 Hz tick, so about 1 % |
| Time to first frame | from starting the viewer to the first hub frame whose probe pixel has the node's colour (`pixelwatch`, wlr-screencopy) | one hub frame, 16.7 ms. On Weston stacks the scene program only starts after the viewer has connected (Weston has no seat before), except in the `latency` scene whose program tolerates that; so for Weston stacks only the `latency` figure is comparable |
| Input-to-pixel delay | `hubclick` makes a virtual mouse click on the hub and stamps the time; `benchapp` on the node stamps the time the click arrives; `pixelwatch` on the hub stamps the first frame in which a 64 x 64 square has turned from black to white or the reverse. 20 clicks, 0.7 s apart, after a warm-up click. All stamps are CLOCK_REALTIME of this one machine | 16.7 ms steps (hub frame clock); includes the hub's own input path, the viewer, the relay, the server, the node compositor and the node program's redraw, and back |
| Dropped frames | `mpv` plays a 1280 x 720, 30 fps test pattern; the top 40 rows carry the frame number as 16 black/white blocks; `pixelwatch` reads the 16 blocks on every hub frame for 10 s; frames never seen = dropped | counts frames never shown; does not see frames shown twice or late |
| Text fidelity | a screenshot of the node (its own `grim` or `weston-screenshooter`) and a screenshot of the hub (`grim`) of the same page of text; `tools/bench/remote-display/ssim` computes SSIM (8 x 8 windows on luma, step 4) and PSNR (all colour values) | exact for the two pictures; see 4.2 for what it does not say |
| Reconnect | (1) the viewer is killed and started again: time to the first frame; (2) the connection is cut (the relay closes it): is the same viewer still alive after 15 s, did it open a new connection, did the picture return | (2) is a yes or no |
| Does the server go quiet | a moving window runs; the viewer process and its children are frozen with SIGSTOP; bytes and node CPU in the 8 s window after 6 s of settling, compared with the 8 s before | the frozen viewer stops reading, so TCP back-pressure builds up: this tests the server's reaction to a viewer that stops, not a viewer that politely sends no update request |
| Client asks for a smaller size | the viewer window is taken out of fullscreen and set to half size; node screen size and hub window size are read before and 5 s after | |
| Scale | 20 nodes and 20 viewers, section 8 | |

### 4.1 What cannot be measured without a GPU (or a real network, or real machines)

Plainly: **this study cannot say how fast any of this is on the real hardware or the real network.** The numbers are software-rendering numbers on shared cores.

- **GPU capture and GPU encode (H.264 and the like).** neatvnc only uses its H.264 encoder for frames that are GPU buffers (section 9). With no GPU every frame is a CPU buffer, so wayvnc was built and run with that path switched off, and the Open H.264 path of wlvncc and TigerVNC was never used. **How a GPU encode path changes node CPU, bytes per second, delay and picture quality: UNKNOWN (not measured).**
- **Hardware decode on the hub** (wlvncc uses VAAPI for H.264, section 9): UNKNOWN. How many windows the hub's GPU can decode at once (HUB-OS.md "Unverified"): UNKNOWN.
- **Compositor behaviour on a real GPU**: sway and Weston used the pixman (software) renderer, hub and node alike. Nothing here says anything about driftwm's own speed.
- **A real network**: loopback has no delay, no loss and no bandwidth limit. The bytes per second are what would cross a link; whether a link carries them is for the December hardware. Input-to-pixel delay here is **only the software path**; add the network.
- **Several nodes at once on separate machines**: the scale test runs 20 nodes on the same 4 cores as the hub.
- **Audio** (not part of this task).
- **True click-to-screen delay**: the clock of `pixelwatch` is the hub compositor's repaint (60 Hz); a camera pointed at a screen would add the display's own delay (as `HUB-OS.md` says for the gaming box).
- **Colour accuracy, tearing, vsync, cursor shape, multi-monitor**: not looked at.

### 4.2 What the numbers do not say (read before quoting a table)

- The hub is a **sway** (wlroots), **not driftwm**. Window matching under driftwm was done in remote-display round 2 and is not repeated.
- The node program matters: `foot` (text) and `benchapp` (a window of fine strokes) are test loads, not a real desktop. The "scrolling" scene prints 30 lines a second, the "terminal flood" prints as fast as the program can (it uses a whole core of its own). The "window drag" is a **simulated** drag: a 640 x 480 window of strokes is redrawn at a new place 60 times a second inside a fullscreen window; the compositor does not move a window. It produces the same kind of screen change; it does not test the compositor's window moving code.
- **wayvnc has a default limit of 30 frames a second** (`-f, --max-fps`, default 30; SOURCE `src/main.c:2482-2484`, v0.10.2). The video is 30 fps, the drag is 60 fps. Weston has no such option that I found (UNKNOWN: Weston's frame rate limit).
- The text-fidelity pictures are **one page of black text on a light-blue background**. SSIM and PSNR compare two screenshots; they measure how far the viewer's picture is from the node's, not whether the text is readable. Text readability was not judged by a person.
- SSIM here is my own small implementation (8 x 8 equal-weight windows on luma, step 4). Do not compare its values with numbers from other tools. Its unit test checks identical pictures (SSIM 1, PSNR infinite), a known PSNR (48.13 dB for a difference of 1 in every value), that a blur lowers SSIM, that an inverted picture is low, and the size-mismatch error.
- Several tools (`remote-viewer`, `TigerVNC`) use their own defaults for picture quality (`QualityLevel`, `CompressLevel`). They are recorded in the table "encodings the VNC viewers asked for" and were **not tuned**. A different quality setting would move bytes per second and fidelity. The default of each viewer was used because that is what hubd would start.
- Weston's VNC and RDP backends were run with **a test-only login** (any password is accepted; section 3.3). Real logins cost a little more at connect time and nothing during the session (BELIEVED).
- `N` is the number of finished runs in a cell; where N is 1 the "min / median / max" is one number repeated.
- The first run in a cell may be slower than later ones because files are not yet in the page cache (UNKNOWN how much).



---

## 5. Results

### 5.0 The short table

How to read: every table is **min / median / max over N finished runs**; N is in the column "N". CPU 100 = one core fully busy. "Node w/o app" leaves out the test program (the terminal or `mpv`), so it is the cost of the remote display plus the compositor. "kbit/s" is server to viewer on the socket. All of these are TESTED with `campaign.sh`; the raw files are `results/raw/STACK__CLIENT__SCENE__RES__rN.txt`. The machine's load average before each run is in the file (`load_start`).

Stacks: **wayvnc-sway** = headless sway + wayvnc/neatvnc (the stack `HUB-OS.md` plans); **wayvnc-cage** = headless cage + wayvnc (720p only); **weston-vnc** = Weston VNC backend; **weston-rdp** = Weston RDP backend. Viewers: **wlvncc**, **tigervnc** (the X11 viewer, through Xwayland), **remote-viewer**, **freerdp-wl** (`wlfreerdp3`), **freerdp-x11** (`xfreerdp3`, through Xwayland).

#### short table, 1080p, medians
One row per stack and viewer; every number is the median of N = 3 runs (clipboard: one run) taken from the longer tables below. "hub CPU" is the hub's viewer plus the hub compositor in the scrolling-text scene (100 = one full core). Cage is left out (720p only).

| stack | viewer | scrolling text kbit/s | hub CPU % (scroll) | video frames dropped % | click to pixel ms | first frame ms | text SSIM / PSNR dB | clipboard |
|---|---|---|---|---|---|---|---|---|
| wayvnc-sway | remote-viewer | 29963 | 110 | 26.8 | 50 | 521 | 1.0000 / inf | node to hub, ASCII only (UTF-8 garbled) |
| wayvnc-sway | tigervnc | 86285 | 39 | 5.3 | 43 | 195 | 0.9930 / 37.0 | both ways, UTF-8 ok |
| wayvnc-sway | wlvncc | 57386 | 25 | 7.0 | 43 | 158 | 0.9723 / 31.6 | none |
| weston-rdp | freerdp-wl | 71421 | 89 | 0.0 | 131 | 355 | 0.9977 / 42.4 | node to hub, UTF-8 ok |
| weston-rdp | freerdp-x11 | 82001 | 106 | 0.0 | 67 | 304 | 0.9977 / 42.4 | both ways, UTF-8 ok |
| weston-vnc | remote-viewer | 29737 | 111 | 22.8 | 32 | 425 | 1.0000 / inf | none |
| weston-vnc | tigervnc | 92302 | 43 | 0.0 | 26 | 219 | 0.9930 / 37.0 | none |
| weston-vnc | wlvncc | 62459 | 28 | 15.3 | 25 | 1205 | 0.9723 / 31.6 | none |


### 5.1 Load scenes at 1080p (1920 x 1080), N = 3

#### idle, 1080p
| stack | client | N | kbit/s | node CPU % | node w/o app CPU % | hub CPU % | node RSS MB | hub RSS MB |
|---|---|---|---|---|---|---|---|---|
| wayvnc-sway | remote-viewer | 3 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 1 / 1 / 1 | 106 / 106 / 107 | 95 / 95 / 95 |
| wayvnc-sway | tigervnc | 3 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 1 / 1 / 1 | 107 / 107 / 119 | 70 / 70 / 70 |
| wayvnc-sway | wlvncc | 3 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 1 / 1 / 1 | 107 / 107 / 107 | 92 / 92 / 100 |
| weston-rdp | freerdp-wl | 3 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 1 / 1 / 1 | 94 / 94 / 94 | 160 / 160 / 160 |
| weston-rdp | freerdp-x11 | 3 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 1 / 1 / 1 | 94 / 94 / 94 | 122 / 123 / 123 |
| weston-vnc | remote-viewer | 3 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 1 / 1 / 1 | 110 / 110 / 110 | 96 / 96 / 96 |
| weston-vnc | tigervnc | 3 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 1 / 1 / 1 | 111 / 111 / 111 | 75 / 75 / 75 |
| weston-vnc | wlvncc | 3 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 1 / 1 / 1 | 110 / 111 / 111 | 101 / 101 / 101 |


**Idle:** zero bytes and zero CPU everywhere (TESTED). The memory columns are resident memory of the node (server + compositor + terminal) and of the hub (viewer + hub compositor; includes a 3 MB shell wrapper per viewer).

#### scroll, 1080p
| stack | client | N | kbit/s | node CPU % | node w/o app CPU % | hub CPU % | node RSS MB | hub RSS MB |
|---|---|---|---|---|---|---|---|---|
| wayvnc-sway | remote-viewer | 3 | 28585 / 29963 / 31210 | 132 / 134 / 134 | 125 / 127 / 127 | 110 / 110 / 110 | 138 / 138 / 138 | 94 / 95 / 95 |
| wayvnc-sway | tigervnc | 3 | 84950 / 86285 / 86425 | 98 / 99 / 101 | 91 / 92 / 94 | 39 / 39 / 40 | 138 / 138 / 139 | 78 / 78 / 78 |
| wayvnc-sway | wlvncc | 3 | 56595 / 57386 / 58249 | 97 / 99 / 100 | 90 / 92 / 92 | 24 / 25 / 26 | 138 / 138 / 138 | 108 / 108 / 108 |
| weston-rdp | freerdp-wl | 3 | 69319 / 71421 / 71541 | 117 / 127 / 128 | 111 / 120 / 121 | 88 / 89 / 90 | 103 / 103 / 103 | 160 / 161 / 161 |
| weston-rdp | freerdp-x11 | 3 | 81826 / 82001 / 83269 | 130 / 131 / 134 | 123 / 124 / 127 | 104 / 106 / 108 | 103 / 103 / 103 | 130 / 130 / 131 |
| weston-vnc | remote-viewer | 3 | 29380 / 29737 / 30589 | 77 / 78 / 79 | 70 / 70 / 72 | 110 / 111 / 111 | 126 / 126 / 127 | 96 / 96 / 96 |
| weston-vnc | tigervnc | 3 | 92259 / 92302 / 93546 | 51 / 52 / 53 | 44 / 45 / 46 | 43 / 43 / 44 | 119 / 119 / 119 | 83 / 83 / 83 |
| weston-vnc | wlvncc | 3 | 62331 / 62459 / 62484 | 50 / 51 / 51 | 43 / 43 / 44 | 27 / 28 / 28 | 119 / 119 / 119 | 109 / 109 / 109 |


#### term, 1080p
| stack | client | N | kbit/s | node CPU % | node w/o app CPU % | hub CPU % | node RSS MB | hub RSS MB |
|---|---|---|---|---|---|---|---|---|
| wayvnc-sway | remote-viewer | 3 | 24926 / 25913 / 28081 | 206 / 210 / 211 | 109 / 109 / 114 | 103 / 104 / 104 | 142 / 142 / 142 | 95 / 103 / 103 |
| wayvnc-sway | tigervnc | 3 | 77883 / 79349 / 83084 | 207 / 210 / 211 | 86 / 89 / 91 | 32 / 32 / 33 | 142 / 142 / 143 | 78 / 78 / 78 |
| wayvnc-sway | wlvncc | 3 | 53539 / 53772 / 54469 | 215 / 215 / 216 | 88 / 91 / 91 | 23 / 24 / 24 | 142 / 142 / 142 | 108 / 108 / 108 |
| weston-rdp | freerdp-wl | 3 | 49932 / 51341 / 51568 | 189 / 190 / 196 | 77 / 78 / 84 | 62 / 63 / 65 | 106 / 106 / 106 | 160 / 160 / 160 |
| weston-rdp | freerdp-x11 | 3 | 63560 / 63751 / 63961 | 195 / 196 / 197 | 90 / 92 / 93 | 75 / 77 / 78 | 106 / 106 / 106 | 130 / 131 / 131 |
| weston-vnc | remote-viewer | 3 | 25136 / 25396 / 25694 | 177 / 178 / 179 | 62 / 63 / 67 | 103 / 103 / 106 | 130 / 130 / 130 | 95 / 102 / 104 |
| weston-vnc | tigervnc | 3 | 125644 / 136645 / 142709 | 178 / 182 / 182 | 61 / 63 / 65 | 51 / 52 / 53 | 131 / 131 / 131 | 82 / 83 / 83 |
| weston-vnc | wlvncc | 3 | 105355 / 108578 / 108723 | 184 / 189 / 190 | 61 / 63 / 66 | 39 / 40 / 41 | 122 / 122 / 130 | 109 / 109 / 109 |


#### drag, 1080p
| stack | client | N | kbit/s | node CPU % | node w/o app CPU % | hub CPU % | node RSS MB | hub RSS MB |
|---|---|---|---|---|---|---|---|---|
| wayvnc-sway | remote-viewer | 3 | 14567 / 15014 / 15401 | 105 / 111 / 115 | 92 / 96 / 99 | 79 / 82 / 88 | 118 / 118 / 118 | 95 / 95 / 95 |
| wayvnc-sway | tigervnc | 3 | 57276 / 57546 / 59948 | 94 / 101 / 102 | 80 / 86 / 86 | 20 / 21 / 21 | 103 / 118 / 119 | 62 / 62 / 62 |
| wayvnc-sway | wlvncc | 3 | 37512 / 37543 / 37597 | 94 / 99 / 101 | 79 / 83 / 85 | 13 / 14 / 14 | 102 / 102 / 103 | 108 / 108 / 108 |
| weston-rdp | freerdp-wl | 3 | 68185 / 68190 / 70868 | 114 / 115 / 116 | 106 / 106 / 108 | 80 / 81 / 82 | 82 / 82 / 82 | 137 / 137 / 137 |
| weston-rdp | freerdp-x11 | 3 | 96172 / 96942 / 102260 | 158 / 160 / 162 | 146 / 148 / 151 | 110 / 111 / 112 | 83 / 83 / 83 | 114 / 114 / 114 |
| weston-vnc | remote-viewer | 3 | 23985 / 24270 / 29920 | 87 / 89 / 92 | 71 / 72 / 77 | 104 / 105 / 105 | 106 / 106 / 106 | 96 / 96 / 96 |
| weston-vnc | tigervnc | 3 | 125618 / 126303 / 127842 | 72 / 74 / 77 | 56 / 58 / 60 | 42 / 44 / 44 | 99 / 99 / 100 | 67 / 67 / 67 |
| weston-vnc | wlvncc | 3 | 82890 / 83606 / 84175 | 68 / 71 / 74 | 53 / 55 / 57 | 26 / 26 / 27 | 98 / 98 / 99 | 109 / 109 / 109 |


#### video, 1080p
| stack | client | N | kbit/s | node CPU % | node w/o app CPU % | hub CPU % | node RSS MB | hub RSS MB |
|---|---|---|---|---|---|---|---|---|
| wayvnc-sway | remote-viewer | 3 | 89696 / 96955 / 98263 | 168 / 173 / 176 | 133 / 139 / 141 | 104 / 104 / 104 | 230 / 231 / 232 | 95 / 96 / 102 |
| wayvnc-sway | tigervnc | 3 | 40261 / 40826 / 40966 | 115 / 120 / 122 | 81 / 85 / 87 | 24 / 25 / 25 | 228 / 228 / 228 | 62 / 62 / 63 |
| wayvnc-sway | wlvncc | 3 | 31199 / 32049 / 32311 | 114 / 114 / 120 | 80 / 81 / 85 | 22 / 24 / 25 | 228 / 228 / 228 | 108 / 108 / 108 |
| weston-rdp | freerdp-wl | 3 | 44592 / 45166 / 45166 | 132 / 134 / 137 | 98 / 100 / 102 | 71 / 73 / 73 | 190 / 190 / 190 | 136 / 136 / 136 |
| weston-rdp | freerdp-x11 | 3 | 45166 / 45177 / 45181 | 130 / 135 / 136 | 97 / 100 / 101 | 72 / 76 / 76 | 191 / 191 / 191 | 114 / 114 / 114 |
| weston-vnc | remote-viewer | 3 | 88252 / 91092 / 92719 | 118 / 121 / 122 | 83 / 86 / 87 | 104 / 104 / 104 | 219 / 219 / 219 | 96 / 99 / 99 |
| weston-vnc | tigervnc | 3 | 43329 / 43446 / 43453 | 70 / 71 / 73 | 36 / 36 / 37 | 26 / 27 / 27 | 208 / 208 / 208 | 67 / 67 / 68 |
| weston-vnc | wlvncc | 3 | 33568 / 33649 / 33669 | 68 / 68 / 72 | 34 / 34 / 36 | 24 / 24 / 26 | 208 / 208 / 208 | 109 / 109 / 110 |


In words (TESTED): for the text scenes (scrolling and flood) the TigerVNC viewer received the most bytes (79 to 137 Mbit/s), wlvncc 54 to 109, Weston RDP 51 to 82, and `remote-viewer` the least (25 to 30 Mbit/s); **fewer bytes did not mean less work**: `remote-viewer` received the fewest bytes and used a whole hub core to decode them. On the node, Weston VNC needed about half the CPU of wayvnc on sway (scrolling text, node without the test program: 43 to 70 % against 92 to 127 %), but Weston 13 used the older neatvnc 0.7.1 (section 3.2) and sway runs a heavier compositor than Weston's kiosk shell; the difference cannot be split between these causes with these runs (UNKNOWN).

### 5.2 Latency, dropped frames and text fidelity

#### latency: time to first frame and input-to-pixel, 1080p
Resolution of the pixel watcher: one hub frame, 16.7 ms.

| stack | client | N | first frame ms | click to node ms | input to pixel ms (median of 20 clicks) | input to pixel ms (worst click) |
|---|---|---|---|---|---|---|
| wayvnc-sway | remote-viewer | 3 | 401 / 521 / 634 | 0.8 / 0.9 / 0.9 | 48.6 / 49.5 / 51.2 | 57.9 / 60.7 / 61.1 |
| wayvnc-sway | tigervnc | 3 | 188 / 195 / 221 | 1.6 / 1.6 / 1.7 | 42.9 / 43.1 / 43.9 | 55.9 / 57.2 / 60.5 |
| wayvnc-sway | wlvncc | 3 | 156 / 158 / 173 | 1.3 / 1.3 / 1.4 | 41.5 / 42.9 / 44.6 | 48.7 / 50.1 / 265.4 |
| weston-rdp | freerdp-wl | 3 | 353 / 355 / 355 | 1.3 / 1.3 / 1.4 | 130.7 / 131.3 / 133.2 | 221.2 / 226.4 / 235.4 |
| weston-rdp | freerdp-x11 | 3 | 304 / 304 / 318 | 1.4 / 1.5 / 1.5 | 65.5 / 67.1 / 71.6 | 110.6 / 120.7 / 182.4 |
| weston-vnc | remote-viewer | 3 | 392 / 425 / 489 | 0.8 / 0.8 / 0.8 | 31.1 / 32.0 / 32.7 | 51.9 / 52.3 / 56.9 |
| weston-vnc | tigervnc | 3 | 178 / 219 / 253 | 1.5 / 1.5 / 1.6 | 26.0 / 26.2 / 31.4 | 47.0 / 47.3 / 50.9 |
| weston-vnc | wlvncc | 3 | 1173 / 1205 / 1242 | 1.2 / 1.3 / 1.3 | 23.8 / 25.1 / 25.8 | 44.6 / 47.1 / 51.2 |


Input-to-pixel is a click on the hub to a changed pixel on the hub; "click to node" is the part until the node's program receives the click (1 to 2 ms everywhere). The delay includes the hub's input path, the viewer, the relay, the server, the node compositor and the node program's redraw, and back. The 60 Hz hub frame clock adds up to 16.7 ms to the measured time; wayvnc's 30 fps limit adds up to 33 ms (BELIEVED, explains why wayvnc is slower than Weston VNC, not tested). In this scene the test window is started before the viewer on every stack (it copes with a seat that appears later), so the first-frame figures here are comparable. In the other scenes the Weston stacks start the scene program after the viewer connects, so their first-frame figures in the raw files are not comparable and are not used. The `wlvncc` + Weston VNC cell is slow (about 1.2 s) because of the login command (`-A`) and TLS.

#### video: dropped frames, 1080p
| stack | client | N | shown fps (of 30) | dropped frames % |
|---|---|---|---|---|
| wayvnc-sway | remote-viewer | 3 | 20.3 / 22.4 / 22.9 | 25.2 / 26.8 / 31.4 |
| wayvnc-sway | tigervnc | 3 | 28.3 / 28.5 / 28.5 | 5.3 / 5.3 / 6.0 |
| wayvnc-sway | wlvncc | 3 | 26.8 / 28.1 / 28.5 | 5.0 / 7.0 / 11.0 |
| weston-rdp | freerdp-wl | 3 | 30.0 / 30.1 / 30.1 | 0.0 / 0.0 / 0.3 |
| weston-rdp | freerdp-x11 | 3 | 30.1 / 30.1 / 30.1 | 0.0 / 0.0 / 0.0 |
| weston-vnc | remote-viewer | 3 | 22.9 / 23.7 / 23.7 | 22.5 / 22.8 / 38.8 |
| weston-vnc | tigervnc | 3 | 30.0 / 30.1 / 30.2 | 0.0 / 0.0 / 0.3 |
| weston-vnc | wlvncc | 3 | 25.4 / 25.4 / 26.4 | 12.0 / 15.3 / 15.9 |


"Shown fps" counts different frame numbers the hub displayed per second of a 30 fps clip; "dropped" counts frame numbers never displayed. It does not count frames shown late.

#### text fidelity, 1080p
SSIM and PSNR of a screenshot of the hub against the node's own screenshot (tools/bench/remote-display/ssim). 1.0 and inf mean identical pictures.

| stack | client | N | SSIM whole screen | PSNR whole dB | SSIM text area | PSNR text area dB |
|---|---|---|---|---|---|---|
| wayvnc-sway | remote-viewer | 3 | 1.0000 / 1.0000 / 1.0000 | inf / inf / inf | 1.0000 / 1.0000 / 1.0000 | inf / inf / inf |
| wayvnc-sway | tigervnc | 3 | 0.9930 / 0.9930 / 0.9930 | 37.0 / 37.0 / 37.0 | 0.9882 / 0.9882 / 0.9882 | 34.9 / 34.9 / 34.9 |
| wayvnc-sway | wlvncc | 3 | 0.9723 / 0.9723 / 0.9723 | 31.6 / 31.6 / 31.6 | 0.9538 / 0.9538 / 0.9538 | 29.5 / 29.5 / 29.5 |
| weston-rdp | freerdp-wl | 3 | 0.9977 / 0.9977 / 0.9977 | 42.4 / 42.4 / 42.4 | 0.9963 / 0.9963 / 0.9963 | 40.7 / 40.7 / 40.7 |
| weston-rdp | freerdp-x11 | 3 | 0.9977 / 0.9977 / 0.9977 | 42.4 / 42.4 / 42.4 | 0.9963 / 0.9963 / 0.9963 | 40.7 / 40.7 / 40.7 |
| weston-vnc | remote-viewer | 3 | 1.0000 / 1.0000 / 1.0000 | inf / inf / inf | 1.0000 / 1.0000 / 1.0000 | inf / inf / inf |
| weston-vnc | tigervnc | 3 | 0.9930 / 0.9930 / 0.9930 | 37.0 / 37.0 / 37.0 | 0.9882 / 0.9882 / 0.9882 | 34.9 / 34.9 / 34.9 |
| weston-vnc | wlvncc | 3 | 0.9723 / 0.9723 / 0.9723 | 31.6 / 31.6 / 31.6 | 0.9538 / 0.9538 / 0.9538 | 29.5 / 29.5 / 29.5 |


#### text fidelity, 4k
SSIM and PSNR of a screenshot of the hub against the node's own screenshot (tools/bench/remote-display/ssim). 1.0 and inf mean identical pictures.

| stack | client | N | SSIM whole screen | PSNR whole dB | SSIM text area | PSNR text area dB |
|---|---|---|---|---|---|---|
| wayvnc-sway | remote-viewer | 2 | 1.0000 / 1.0000 / 1.0000 | inf / inf / inf | 1.0000 / 1.0000 / 1.0000 | inf / inf / inf |
| wayvnc-sway | tigervnc | 2 | 0.9964 / 0.9964 / 0.9964 | 39.9 / 39.9 / 39.9 | 0.9941 / 0.9941 / 0.9941 | 37.8 / 37.8 / 37.8 |
| wayvnc-sway | wlvncc | 2 | 0.9861 / 0.9861 / 0.9861 | 34.4 / 34.4 / 34.4 | 0.9768 / 0.9768 / 0.9768 | 32.4 / 32.4 / 32.4 |
| weston-rdp | freerdp-wl | 2 | 0.9987 / 0.9987 / 0.9987 | 44.3 / 44.3 / 44.3 | 0.9980 / 0.9980 / 0.9980 | 42.9 / 42.9 / 42.9 |
| weston-rdp | freerdp-x11 | 2 | 0.9987 / 0.9987 / 0.9987 | 44.3 / 44.3 / 44.3 | 0.9980 / 0.9980 / 0.9980 | 42.9 / 42.9 / 42.9 |
| weston-vnc | remote-viewer | 2 | 1.0000 / 1.0000 / 1.0000 | inf / inf / inf | 1.0000 / 1.0000 / 1.0000 | inf / inf / inf |
| weston-vnc | tigervnc | 2 | 0.9964 / 0.9964 / 0.9964 | 39.9 / 39.9 / 39.9 | 0.9941 / 0.9941 / 0.9941 | 37.8 / 37.8 / 37.8 |
| weston-vnc | wlvncc | 2 | 0.9861 / 0.9861 / 0.9861 | 34.4 / 34.4 / 34.4 | 0.9768 / 0.9768 / 0.9768 | 32.4 / 32.4 / 32.4 |


SSIM near 1 and PSNR above about 40 dB mean the viewer's picture is almost the node's own picture. **`remote-viewer` is lossless for text** (its viewer asks for Tight without a quality level; it printed exactly the node's pixels); **Weston RDP (RemoteFX/NSCodec or bitmap, it is not stated which was used: UNKNOWN) is next**; the **TigerVNC viewer** (quality level 8 in its request) is third; **wlvncc** (quality level 5, the viewer's default) is last. The server is the same in every VNC row, so this difference is the viewer's requested quality, not the server (BELIEVED, consistent with identical wayvnc/Weston numbers for the same viewer). At 4K every number is better (BELIEVED: the same 12-point text is a smaller part of a larger blank picture). **Whether the text is readable was not judged by a person.**

The 1080p cell for wlvncc on Weston VNC uses a **reference screenshot taken on the sway stack**: Weston's own capture never finished with wlvncc (see 7.5). TESTED justification: the same text page on sway and on Weston is pixel-identical (the lossless `remote-viewer` picture of a Weston node against a sway screenshot gave SSIM 1.0, PSNR inf).

### 5.3 4K (3840 x 2160), N = 2

#### idle, 4k
| stack | client | N | kbit/s | node CPU % | node w/o app CPU % | hub CPU % | node RSS MB | hub RSS MB |
|---|---|---|---|---|---|---|---|---|
| wayvnc-sway | remote-viewer | 2 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 1 / 1 / 1 | 272 / 288 / 304 | 190 / 190 / 191 |
| wayvnc-sway | tigervnc | 2 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 1 / 1 / 1 | 356 / 356 / 357 | 141 / 141 / 141 |
| wayvnc-sway | wlvncc | 2 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 1 / 1 / 1 | 305 / 305 / 305 | 234 / 234 / 234 |
| weston-rdp | freerdp-wl | 2 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 1 / 1 / 1 | 251 / 251 / 251 | 482 / 482 / 482 |
| weston-rdp | freerdp-x11 | 2 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 1 / 1 / 1 | 251 / 251 / 251 | 325 / 325 / 325 |
| weston-vnc | remote-viewer | 2 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 1 / 1 / 1 | 260 / 260 / 260 | 191 / 193 / 195 |
| weston-vnc | tigervnc | 2 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 1 / 1 / 1 | 262 / 262 / 262 | 146 / 146 / 146 |
| weston-vnc | wlvncc | 2 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 1 / 1 / 1 | 262 / 262 / 262 | 267 / 267 / 268 |


#### scroll, 4k
| stack | client | N | kbit/s | node CPU % | node w/o app CPU % | hub CPU % | node RSS MB | hub RSS MB |
|---|---|---|---|---|---|---|---|---|
| wayvnc-sway | remote-viewer | 2 | 11713 / 12688 / 13664 | 163 / 164 / 166 | 141 / 142 / 144 | 112 / 112 / 112 | 408 / 408 / 408 | 190 / 195 / 201 |
| wayvnc-sway | tigervnc | 2 | 63761 / 64140 / 64520 | 157 / 158 / 159 | 135 / 136 / 137 | 28 / 28 / 28 | 410 / 410 / 410 | 173 / 173 / 173 |
| wayvnc-sway | wlvncc | 2 | 41224 / 41798 / 42372 | 156 / 157 / 158 | 133 / 134 / 135 | 18 / 18 / 18 | 408 / 408 / 408 | 297 / 297 / 298 |
| weston-rdp | freerdp-wl | 2 | 54403 / 54632 / 54862 | 141 / 144 / 147 | 133 / 135 / 138 | 107 / 108 / 108 | 261 / 261 / 261 | 482 / 482 / 482 |
| weston-rdp | freerdp-x11 | 2 | 74937 / 75766 / 76595 | 174 / 174 / 174 | 163 / 163 / 163 | 138 / 138 / 138 | 261 / 261 / 261 | 357 / 357 / 357 |
| weston-vnc | remote-viewer | 2 | 14489 / 14636 / 14783 | 99 / 102 / 104 | 78 / 80 / 82 | 112 / 112 / 112 | 301 / 301 / 301 | 227 / 228 / 229 |
| weston-vnc | tigervnc | 2 | 154308 / 158233 / 162158 | 127 / 128 / 128 | 106 / 106 / 106 | 68 / 69 / 69 | 272 / 272 / 272 | 178 / 178 / 178 |
| weston-vnc | wlvncc | 2 | 122316 / 125420 / 128523 | 132 / 134 / 136 | 111 / 112 / 114 | 52 / 52 / 53 | 271 / 271 / 271 | 299 / 299 / 299 |


#### drag, 4k
| stack | client | N | kbit/s | node CPU % | node w/o app CPU % | hub CPU % | node RSS MB | hub RSS MB |
|---|---|---|---|---|---|---|---|---|
| wayvnc-sway | remote-viewer | 2 | 5134 / 5279 / 5424 | 149 / 151 / 152 | 122 / 122 / 122 | 86 / 86 / 87 | 386 / 386 / 386 | 190 / 194 / 198 |
| wayvnc-sway | tigervnc | 2 | 19236 / 20360 / 21484 | 146 / 146 / 146 | 116 / 117 / 118 | 7 / 7 / 8 | 391 / 391 / 391 | 109 / 109 / 110 |
| wayvnc-sway | wlvncc | 2 | 12702 / 13534 / 14367 | 146 / 146 / 146 | 116 / 118 / 119 | 5 / 5 / 5 | 325 / 325 / 325 | 297 / 297 / 297 |
| weston-rdp | freerdp-wl | 2 | 59006 / 59279 / 59552 | 126 / 127 / 127 | 119 / 119 / 120 | 90 / 91 / 92 | 239 / 239 / 239 | 387 / 387 / 387 |
| weston-rdp | freerdp-x11 | 2 | 87419 / 88200 / 88980 | 179 / 179 / 179 | 168 / 169 / 169 | 123 / 123 / 124 | 240 / 240 / 240 | 293 / 293 / 293 |
| weston-vnc | remote-viewer | 2 | 11851 / 11924 / 11997 | 101 / 101 / 102 | 77 / 78 / 78 | 108 / 109 / 110 | 279 / 279 / 279 | 191 / 193 / 194 |
| weston-vnc | tigervnc | 2 | 69496 / 70149 / 70801 | 105 / 106 / 108 | 79 / 81 / 82 | 24 / 25 / 25 | 281 / 281 / 281 | 114 / 114 / 114 |
| weston-vnc | wlvncc | 2 | 47084 / 49891 / 52697 | 104 / 105 / 105 | 79 / 79 / 80 | 17 / 18 / 18 | 280 / 280 / 280 | 299 / 299 / 299 |


#### latency: time to first frame and input-to-pixel, 4k
Resolution of the pixel watcher: one hub frame, 16.7 ms.

| stack | client | N | first frame ms | click to node ms | input to pixel ms (median of 20 clicks) | input to pixel ms (worst click) |
|---|---|---|---|---|---|---|
| wayvnc-sway | remote-viewer | 2 | 1249 / 1809 / 2369 | 0.8 / 0.8 / 0.8 | 150.5 / 150.8 / 151.2 | 196.2 / 196.4 / 196.6 |
| wayvnc-sway | tigervnc | 2 | 395 / 438 / 481 | 1.5 / 1.6 / 1.6 | 130.3 / 130.4 / 130.4 | 157.5 / 166.2 / 174.9 |
| wayvnc-sway | wlvncc | 2 | 422 / 428 / 435 | 1.3 / 1.4 / 1.4 | 127.8 / 129.2 / 130.6 | 169.8 / 173.6 / 177.4 |
| weston-rdp | freerdp-wl | 2 | 780 / 828 / 876 | 1.2 / 1.2 / 1.3 | 260.9 / 262.2 / 263.5 | 285.9 / 296.9 / 307.9 |
| weston-rdp | freerdp-x11 | 2 | 561 / 565 / 569 | 1.4 / 1.4 / 1.5 | 323.3 / 340.8 / 358.2 | 352.0 / 388.4 / 424.8 |
| weston-vnc | remote-viewer | 2 | 1345 / 1476 / 1608 | 0.8 / 0.9 / 0.9 | 88.1 / 91.8 / 95.4 | 108.0 / 162.2 / 216.4 |
| weston-vnc | tigervnc | 2 | 417 / 490 / 563 | 1.5 / 1.5 / 1.5 | 65.4 / 67.5 / 69.6 | 98.5 / 101.2 / 103.9 |
| weston-vnc | wlvncc | 2 | 1401 / 1410 / 1418 | 1.3 / 1.3 / 1.3 | 66.0 / 68.3 / 70.7 | 87.9 / 88.7 / 89.4 |


At 4K the node cannot draw and encode fast enough: wayvnc's drag test sent **fewer bytes than at 1080p** (5 to 20 Mbit/s against 15 to 58) and the hub's wlvncc and TigerVNC were nearly idle (5 and 7 % CPU), which means **few frames arrived**; the delay of a click rose from 43 ms to 130 ms. Video and the terminal flood were not run at 4K (time).

### 5.4 The kiosk compositor (cage), 720p only, N = 3

#### scroll, 720p
| stack | client | N | kbit/s | node CPU % | node w/o app CPU % | hub CPU % | node RSS MB | hub RSS MB |
|---|---|---|---|---|---|---|---|---|
| wayvnc-cage | remote-viewer | 3 | 23583 / 23813 / 23923 | 78 / 82 / 83 | 75 / 78 / 79 | 83 / 87 / 87 | 73 / 73 / 73 | 77 / 77 / 78 |
| wayvnc-cage | tigervnc | 3 | 58265 / 58275 / 58284 | 52 / 53 / 55 | 48 / 49 / 50 | 28 / 28 / 28 | 73 / 73 / 74 | 60 / 60 / 60 |
| wayvnc-cage | wlvncc | 3 | 39692 / 39709 / 39811 | 50 / 54 / 54 | 46 / 49 / 49 | 17 / 18 / 19 | 73 / 73 / 73 | 72 / 72 / 72 |


#### video, 720p
| stack | client | N | kbit/s | node CPU % | node w/o app CPU % | hub CPU % | node RSS MB | hub RSS MB |
|---|---|---|---|---|---|---|---|---|
| wayvnc-cage | remote-viewer | 3 | 60254 / 60624 / 60662 | 99 / 103 / 108 | 80 / 82 / 86 | 67 / 70 / 76 | 153 / 154 / 161 | 77 / 77 / 78 |
| wayvnc-cage | tigervnc | 3 | 24348 / 24379 / 24444 | 62 / 63 / 67 | 41 / 42 / 45 | 16 / 16 / 16 | 151 / 152 / 153 | 53 / 53 / 53 |
| wayvnc-cage | wlvncc | 3 | 18384 / 18401 / 18465 | 62 / 66 / 68 | 42 / 45 / 46 | 11 / 12 / 13 | 151 / 153 / 153 | 72 / 72 / 73 |


#### latency: time to first frame and input-to-pixel, 720p
Resolution of the pixel watcher: one hub frame, 16.7 ms.

| stack | client | N | first frame ms | click to node ms | input to pixel ms (median of 20 clicks) | input to pixel ms (worst click) |
|---|---|---|---|---|---|---|
| wayvnc-cage | remote-viewer | 3 | 316 / 400 / 414 | 0.9 / 0.9 / 0.9 | 25.8 / 27.4 / 28.3 | 34.1 / 39.2 / 42.3 |
| wayvnc-cage | tigervnc | 3 | 155 / 155 / 157 | 1.6 / 1.6 / 1.6 | 24.3 / 25.8 / 25.8 | 35.6 / 36.3 / 37.7 |
| wayvnc-cage | wlvncc | 3 | 105 / 105 / 121 | 1.0 / 1.1 / 1.1 | 44.3 / 44.9 / 46.6 | 52.8 / 56.1 / 56.7 |


#### video: dropped frames, 720p
| stack | client | N | shown fps (of 30) | dropped frames % |
|---|---|---|---|---|
| wayvnc-cage | remote-viewer | 3 | 29.7 / 29.9 / 30.0 | 0.3 / 0.3 / 1.0 |
| wayvnc-cage | tigervnc | 3 | 30.0 / 30.0 / 30.1 | 0.0 / 0.3 / 0.3 |
| wayvnc-cage | wlvncc | 3 | 30.0 / 30.0 / 30.1 | 0.3 / 0.3 / 0.3 |


The cage stack can only run at 1280 x 720 (TESTED, 7.1). Its cells are **not comparable to the 1080p rows** (fewer pixels). With wlvncc, no click reached the node unless the hub pointer moved one pixel first: three runs without it matched 0 of 20 clicks; the three runs in the table were made with `HUBCLICK_WIGGLE=1` (the raw files say so). Cause: UNKNOWN (BELIEVED: wlvncc only sends a pointer position with a motion).

### 5.5 Does the server go quiet when the viewer stops asking? (N = 1)

#### does the server go quiet when the viewer stops asking (frozen viewer)
| stack | client | res | run | status | running_s2c_kbit_s | frozen_s2c_kbit_s | resumed_s2c_kbit_s | running_node_srv_cpu | frozen_node_srv_cpu | running_node_comp_cpu | frozen_node_comp_cpu | running_node_app_cpu | frozen_node_app_cpu |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| wayvnc-cage | remote-viewer | 720p | 1 | ok | 16361.6 | 0.0 | 16201.5 | 54.9 | 26.4 | 5.6 | 6.0 | 10.0 | 10.2 |
| wayvnc-cage | tigervnc | 720p | 1 | ok | 63435.2 | 0.0 | 63294.7 | 39.4 | 24.9 | 5.6 | 5.8 | 10.6 | 10.1 |
| wayvnc-cage | wlvncc | 720p | 1 | ok | 41848.2 | 0.0 | 41640.2 | 40.8 | 25.6 | 6.2 | 6.3 | 11.2 | 10.6 |
| wayvnc-sway | remote-viewer | 1080p | 1 | ok | 14481.8 | 0.0 | 15662.9 | 82.0 | 51.9 | 18.2 | 17.9 | 16.8 | 16.7 |
| wayvnc-sway | tigervnc | 1080p | 1 | ok | 57643.9 | 0.0 | 58134.2 | 65.4 | 51.1 | 18.0 | 18.1 | 15.4 | 15.9 |
| wayvnc-sway | wlvncc | 1080p | 1 | ok | 36607.4 | 0.0 | 36825.3 | 67.6 | 49.1 | 19.1 | 18.4 | 16.5 | 16.0 |
| weston-rdp | freerdp-wl | 1080p | 1 | ok | 67094.2 | 0.0 | 69182.7 | 108.0 | 0.0 | gone | gone | 8.8 | 0.0 |
| weston-rdp | freerdp-x11 | 1080p | 1 | ok | 95925.9 | 0.0 | 92438.7 | 149.4 | 0.0 | gone | gone | 11.9 | 0.0 |
| weston-vnc | remote-viewer | 1080p | 1 | ok | 24798.3 | 0.0 | 23666.1 | 72.3 | 28.3 | gone | gone | 16.2 | 16.2 |
| weston-vnc | tigervnc | 1080p | 1 | ok | 124740.7 | 0.0 | 121667.4 | 62.1 | 30.7 | gone | gone | 16.3 | 16.3 |
| weston-vnc | wlvncc | 1080p | 1 | ok | 81320.4 | 0.0 | 82083.2 | 58.9 | 29.1 | gone | gone | 16.2 | 16.2 |


"gone" = the compositor and server are one process (Weston), counted once under "node srv". Every viewer here was frozen with SIGSTOP, so the viewer stopped reading and no update requests were sent. In all stacks **the bytes went to 0** (TCP back-pressure). The CPU is the difference: wayvnc kept encoding or waiting (about 50 % of a core, BELIEVED: it keeps capturing frames while its output buffer is full; not looked into), Weston VNC dropped to about 30 %, **Weston RDP to 0 and its application stopped as well** (no frame callbacks), after the viewer resumed all stacks sent the same bytes as before (TESTED).

### 5.6 The viewer is asked to become smaller (N = 1)

#### the viewer is asked to become smaller
| stack | client | res | run | status | node_size_before | node_size_after | hub_window_before | hub_window_after | viewer_alive_after | hub_window_screenshot |
|---|---|---|---|---|---|---|---|---|---|---|
| wayvnc-cage | remote-viewer | 720p | 1 | ok | 1280x720 | unknown | 1280x720 |  | 0 | 1280x720 |
| wayvnc-cage | tigervnc | 720p | 1 | ok | 1280x720 | unknown | 1280x720 |  | 0 | 1280x720 |
| wayvnc-cage | wlvncc | 720p | 1 | ok | 1280x720 | 1280x720 | 1280x720 | 640x360 | 1 | 1280x720 |
| wayvnc-sway | remote-viewer | 1080p | 1 | ok | 1920x1080 | 1920x1033 | 1920x1080 | 1920x1080 | 1 | 1920x1080 |
| wayvnc-sway | tigervnc | 1080p | 1 | ok | 1920x1080 | 960x540 | 1920x1080 | 960x540 | 1 | 1920x1080 |
| wayvnc-sway | wlvncc | 1080p | 1 | ok | 1920x1080 | 1920x1080 | 1920x1080 | 960x540 | 1 | 1920x1080 |
| weston-rdp | freerdp-wl | 1080p | 1 | ok | 1920x1080 | 1920x1080 | 1920x1080 | 960x540 | 1 | 1920x1080 |
| weston-rdp | freerdp-x11 | 1080p | 1 | ok | 1920x1080 | 1920x1080 | 1920x1080 | 960x540 | 1 | 1920x1080 |
| weston-vnc | remote-viewer | 1080p | 1 | ok | 1920x1080 | 1920x1033 | 1920x1080 | 1920x1080 | 1 | 1920x1080 |
| weston-vnc | tigervnc | 1080p | 1 | ok | 1920x1080 | 960x540 | 1920x1080 | 960x540 | 1 | 1920x1080 |
| weston-vnc | wlvncc | 1080p | 1 | ok | 1920x1080 | 1920x1080 | 1920x1080 | 960x540 | 1 | 1920x1080 |


How: the viewer's window is taken out of fullscreen and set to half size (`swaymsg`); 5 s later the node's screen size is read (`wayland-info`, xdg-output). **Reading:** `tigervnc` made wayvnc (sway) and Weston's VNC backend change the node's screen to 960 x 540; `remote-viewer` made wayvnc and Weston VNC change it to 1920 x 1033 and kept its own window full size (BELIEVED: it asks for its drawing area, not the half size; not investigated); wlvncc and the FreeRDP clients scaled the picture only (FreeRDP's `/dynamic-resolution` was not given, so its resize request path is **UNKNOWN**; Weston's RDP backend does contain a display-control handler, SOURCE `libweston/backend-rdp/rdpdisp.c`). **For cage with TigerVNC or `remote-viewer` the node died** ("unknown"/empty cells): cage 0.1.5 aborts with `Assertion 'solo->scene_output != so' failed` (TESTED, `results/raw/wayvnc-cage__*__smallsize__*`).

### 5.7 The node changes its own screen size (N = 1)

#### the node changes its own screen size
| stack | client | res | run | status | resize_down_s2c_bytes | resize_viewer_alive_after_down | resize_up_s2c_bytes | resize_after_ssim | resize_viewer_alive_end |
|---|---|---|---|---|---|---|---|---|---|
| wayvnc-sway | remote-viewer | 1080p | 1 | ok | 643401 | 1 | 0 | ssim=1.000000 psnr=inf width=1920 height=1080 | 1 |
| wayvnc-sway | tigervnc | 1080p | 1 | ok | 441786 | 1 | 1339322 | ssim=0.992954 psnr=37.03 width=1920 height=1080 | 1 |
| wayvnc-sway | wlvncc | 1080p | 1 | ok | 473526 | 1 | 1568764 | ssim=0.972288 psnr=31.63 width=1920 height=1080 | 1 |
| weston-rdp | freerdp-wl | 1080p | 1 | unsupported:stack cannot change the node screen size |  |  |  |  |  |
| weston-rdp | freerdp-x11 | 1080p | 1 | unsupported:stack cannot change the node screen size |  |  |  |  |  |
| weston-vnc | remote-viewer | 1080p | 1 | unsupported:stack cannot change the node screen size |  |  |  |  |  |
| weston-vnc | tigervnc | 1080p | 1 | unsupported:stack cannot change the node screen size |  |  |  |  |  |
| weston-vnc | wlvncc | 1080p | 1 | unsupported:stack cannot change the node screen size |  |  |  |  |  |


The node's screen went to half size and back (sway only; Weston and cage have no command for it here). All three viewers on wayvnc survived and showed the right picture again (SSIM equal to the normal result for that viewer). `resize_up_s2c_bytes = 0` for `remote-viewer` means no bytes in the measured 3 s window after the change (BELIEVED: its lossless picture arrived earlier or later than the window; not investigated).

### 5.8 Reconnect (N = 2)

#### reconnect
| stack | client | res | run | status | reconnect_new_viewer_ms | link_cut_viewer_alive_after_15s | link_cut_new_connections_in_15s | link_cut_picture_back_ms |
|---|---|---|---|---|---|---|---|---|
| wayvnc-cage | remote-viewer | 720p | 1 | ok | 284 | 0 | 0 | none |
| wayvnc-cage | tigervnc | 720p | 1 | ok | 101 | 0 | 0 | none |
| wayvnc-cage | wlvncc | 720p | 1 | ok | 75 | 0 | 0 | none |
| wayvnc-sway | remote-viewer | 1080p | 1 | ok | 343 | 0 | 0 | none |
| wayvnc-sway | remote-viewer | 1080p | 2 | ok | 499 | 0 | 0 | none |
| wayvnc-sway | tigervnc | 1080p | 1 | ok | 127 | 0 | 0 | none |
| wayvnc-sway | tigervnc | 1080p | 2 | ok | 140 | 0 | 0 | none |
| wayvnc-sway | wlvncc | 1080p | 1 | ok | 114 | 0 | 0 | none |
| wayvnc-sway | wlvncc | 1080p | 2 | ok | 110 | 0 | 0 | none |
| weston-rdp | freerdp-wl | 1080p | 1 | ok | 152 | 0 | 0 | none |
| weston-rdp | freerdp-wl | 1080p | 2 | ok | 164 | 0 | 0 | none |
| weston-rdp | freerdp-x11 | 1080p | 1 | ok | 162 | 0 | 0 | none |
| weston-rdp | freerdp-x11 | 1080p | 2 | ok | 145 | 0 | 0 | none |
| weston-vnc | remote-viewer | 1080p | 1 | ok | 395 | 0 | 0 | none |
| weston-vnc | remote-viewer | 1080p | 2 | ok | 386 | 0 | 0 | none |
| weston-vnc | tigervnc | 1080p | 1 | ok | 143 | 1 | 0 | none |
| weston-vnc | tigervnc | 1080p | 2 | ok | 140 | 1 | 0 | none |
| weston-vnc | wlvncc | 1080p | 1 | ok | 1122 | 1 | 0 | 0 |
| weston-vnc | wlvncc | 1080p | 2 | ok | 1125 | 1 | 0 | 15 |


`reconnect_new_viewer_ms` is the time from starting a new viewer (after the old one was killed) to the node's picture; `link_cut_*` is the second test: the connection is closed from the middle (the byte counter), then 15 s are watched. **No viewer opened a new connection by itself (0 in every row).** On sway/wayvnc and for FreeRDP the viewer process **ended** (alive = 0). For `tigervnc` and `wlvncc` against Weston VNC the process stayed alive without a picture (alive = 1): BELIEVED an error dialog (TigerVNC) or a hung read (wlvncc); not looked into. Weston stayed up in every case: no segfault of Weston appears in `dmesg` for the whole campaign (TESTED: `dmesg | grep -i segfault` shows only three sway crashes from my first debugging session, caused by a wrong `XKB_CONFIG_ROOT` that I removed before any measurement; round 1 of `remote-display.md` saw Weston crash once).

### 5.9 Clipboard text, both ways (N = 1)

#### clipboard text, both ways (exact = the receiving side got precisely the text that was copied)
| stack | client | res | run | status | clip_node_to_hub_ascii_exact | clip_node_to_hub_utf8_exact | clip_hub_to_node_ascii_exact | clip_hub_to_node_utf8_exact | clip_node_to_hub_utf8 |
|---|---|---|---|---|---|---|---|---|---|
| wayvnc-cage | remote-viewer | 720p | 1 | ok | no | no | no | no | '' |
| wayvnc-cage | tigervnc | 720p | 1 | ok | no | no | no | no | '' |
| wayvnc-cage | wlvncc | 720p | 1 | ok | no | no | no | no | '' |
| wayvnc-sway | remote-viewer | 1080p | 1 | ok | yes | no | no | no | 'cafÃ© â\x9c\x93 æ\x97¥æ\x9c¬ naÃ¯ve' |
| wayvnc-sway | tigervnc | 1080p | 1 | ok | yes | yes | yes | yes | 'café ✓ 日本 naïve' |
| wayvnc-sway | wlvncc | 1080p | 1 | ok | no | no | no | no | '' |
| weston-rdp | freerdp-wl | 1080p | 1 | ok | yes | yes | no | no | 'café ✓ 日本 naïve' |
| weston-rdp | freerdp-x11 | 1080p | 1 | ok | yes | yes | yes | yes | 'café ✓ 日本 naïve' |
| weston-vnc | remote-viewer | 1080p | 1 | ok | no | no | no | no | '' |
| weston-vnc | tigervnc | 1080p | 1 | ok | no | no | no | no | '' |
| weston-vnc | wlvncc | 1080p | 1 | ok | no | no | no | no | '' |


The test copies `hubos-ascii-test-1` and `café ✓ 日本 naïve` on one side with `wl-copy` and reads it on the other with `wl-paste`; the hub had a virtual keyboard (so the viewer's window had keyboard focus; `hubclick`). **TigerVNC on wayvnc: both ways, exact, UTF-8 too. `xfreerdp3` on Weston RDP: both ways, exact. `wlfreerdp3`: node to hub only. `remote-viewer`: node to hub ASCII only, UTF-8 comes out as Latin-1 mojibake (`cafÃ©`) even with neatvnc 1.0.3; hub to node never. wlvncc: nothing. Weston VNC: nothing for any viewer. cage: nothing for any viewer.** The empty `''` means `wl-paste` returned nothing. Cage offers no data-control protocol, so wayvnc cannot see the selection there (TESTED: `wayland-info` on cage lists screencopy, export-dmabuf, output-management, virtual keyboard/pointer, and no data-control). Round 2 (repo doc) found that the TigerVNC viewer only exchanges the clipboard while its window has focus, and that a copy made while unfocused did not arrive; this test always had focus.

### 5.10 Which picture encodings the viewers ask for (TESTED, first connection of each run)

#### encodings the VNC viewers asked for (in the order sent; plain connections only)
| stack | client | encodings |
|---|---|---|
| wayvnc-cage | remote-viewer | Tight,ExtendedClipboard,SubsampLevel0,-261,ExtendedDesktopSize,DesktopSize,DesktopName,LastRect,1464686185,-259,ContinuousUpdates,Cursor,XCursor,-257,ZRLE,Hextile,RRE,CopyRect,Raw |
| wayvnc-cage | tigervnc | ContinuousUpdates,1464686180,Cursor,XCursor,1464686182,DesktopSize,ExtendedDesktopSize,-261,1464686184,DesktopName,LastRect,-1063131698,ExtendedMouseButtons,Fence,SubsampLevel0,Tight,CopyRect,OpenH264,ZRLE,Hextile,RRE,CopyRect,Raw,CompressLevel2,QualityLevel8 |
| wayvnc-cage | wlvncc | Tight,ZRLE,9,-65527,CopyRect,Hextile,Zlib,CoRRE,RRE,Raw,CompressLevel3,QualityLevel5,-131072,DesktopSize,LastRect,-131071,-131070,-131069,ExtendedClipboard,SubsampLevel0,-1000 |
| wayvnc-sway | remote-viewer | Tight,ExtendedClipboard,SubsampLevel0,-261,ExtendedDesktopSize,DesktopSize,DesktopName,LastRect,1464686185,-259,ContinuousUpdates,Cursor,XCursor,-257,ZRLE,Hextile,RRE,CopyRect,Raw |
| wayvnc-sway | tigervnc | ContinuousUpdates,1464686180,Cursor,XCursor,1464686182,DesktopSize,ExtendedDesktopSize,-261,1464686184,DesktopName,LastRect,-1063131698,ExtendedMouseButtons,Fence,SubsampLevel0,Tight,CopyRect,OpenH264,ZRLE,Hextile,RRE,CopyRect,Raw,CompressLevel2,QualityLevel8 |
| wayvnc-sway | wlvncc | Tight,ZRLE,9,-65527,CopyRect,Hextile,Zlib,CoRRE,RRE,Raw,CompressLevel3,QualityLevel5,-131072,DesktopSize,LastRect,-131071,-131070,-131069,ExtendedClipboard,SubsampLevel0,-1000 |
| weston-vnc | remote-viewer |  **(differs between runs)** 102105859 / unreadable (TLS or not plain RFB) |
| weston-vnc | tigervnc |  **(differs between runs)** 102105859 / unreadable (TLS or not plain RFB) |
| weston-vnc | wlvncc |  **(differs between runs)** 102105859 / unreadable (TLS or not plain RFB) |

Numbers are encodings this table does not know the name of. Weston's VNC backend uses TLS from the start, so the list cannot be read there.


neatvnc picks the first of Raw / Tight / ZRLE in the viewer's list (SOURCE `src/server.c:2775-2800`), so **all three viewers got Tight** from wayvnc. The TigerVNC viewer also lists encoding 50 (Open H.264) after Tight; it would only matter with GPU frames (section 9).

---

## 6. wlvncc, read at a pinned commit

Source: `https://github.com/any1/wlvncc`, cloned 2026-10-05, checked out at **`cc0abf87c37920540f2439a556e6a480c28f8f46`** (2026-04-29, "Add support for explicit linear dmabuf modifiers"). It was built from that commit (TESTED, `tools/bench/remote-display/setup.sh`). Everything marked SOURCE below is a file and line in that checkout.

| Question | Answer | Label |
|---|---|---|
| Last release / last commit | No release tag except `with-aml-v0.3.0` (an old marker, not a release of the viewer). 150 commits; first 2020-07-09; last 2026-04-29. 127 of the commits are by the wayvnc author (Andri Yngvason), 13 by one other person. The README still says "work-in-progress implementation ... Expect bugs and missing features". | TESTED (`git log`, `git tag`) |
| Window app-id | **Chosen by the caller:** `-a, --app-id=<name>`, default `wlvncc` (`src/main.c:128`, `:1112`). TESTED: with `-a bench-view` sway reported app-id `bench-view`. | SOURCE + TESTED |
| Window title | **Not chosen by the caller.** The title is the **server's desktop name**, set once when the window is made (`window_create(app_id, vnc_client_get_desktop_name(client))`, `src/main.c:814`). TESTED: a wayvnc started with `-n node1` gave title `node1` (no suffix). So hubd can match by app-id (set by hubd) and the title is the machine id when every node's wayvnc is started with `--name <machine id>`, as `HUB-OS.md` already requires. | SOURCE + TESTED |
| Clipboard, node to hub | **None.** The library part has a callback for the server's text (`GotXCutText`, `src/vnc.c:129,248`) and `vnc.h` has a `cut_text` hook, but `src/main.c` never sets that hook, and no file uses a Wayland data-device or data-control protocol (`grep -n "wl_data\|data_offer\|cut_text" src/main.c src/seat.c` finds nothing). | SOURCE; behaviour TESTED in the `clipboard` scene (table "clipboard text" in section 5.9) |
| Clipboard, hub to node | **None**, same reason: `vnc_client_send_cut_text` (`src/vnc.c:451`) is never called from `main.c`. | SOURCE; TESTED as above |
| Non-ASCII text | Clipboard: nothing to carry. Key presses: the viewer sends the key symbols it gets from xkbcommon (`src/main.c:792-804`); typing non-ASCII was **not tested**. | SOURCE for clipboard; typing UNKNOWN |
| Credentials: how given | Three ways, none on the command line by default: (1) it asks on the terminal (`ReadLine("User")`, `ReadLineNoEcho("Password")`, `src/vncviewer.c`); (2) **`-A, --auth-command=<cmd>`** runs a shell command and reads the user name (first line) and password (second line) from its output (`src/vncviewer.c:95-200`; the repository ships `scripts/auth-script.sh`, a zenity dialog); (3) none for servers with security type None. A password can therefore stay out of the process list if the command reads it from a file or a helper. **In this benchmark the test login (`root` / `bench`, accepted by a test-only PAM file) is written inside the `-A` command text, so it IS in the process list there; that is a test shortcut.** | SOURCE; used in TESTED runs against Weston |
| Server certificate | `-t, --tls-cert <file>`: the CA file used to check the server (`src/vncviewer.c:ReadX509Creds`); default `/etc/ssl/cert.pem`. | SOURCE; TESTED with Weston's TLS |
| Reconnect | **None.** When the connection ends, `vnc_client_process` returns an error and the program sets `do_run = false` and exits (`src/main.c:896-900`); closing the window also exits (`:542`). hubd would have to start it again. | SOURCE; TESTED in the `reconnect` scene |
| Several sessions at once | One connection per process, one window per process; run several processes. TESTED with 20 at once (section 7). | SOURCE + TESTED |
| Resize | The window's size is only used to **scale** the picture (`wp_viewport_set_destination`, `src/main.c:527`). The code to ask the server for a new size exists in the bundled library (`SetDesktopSize`, `src/rfbproto.c:1666`) but `main.c` never calls it. | SOURCE; TESTED in `smallsize` |
| Needs | Wayland with `xdg-shell`, `wp_viewporter`, `wl_subcompositor` (asserted at start, `main.c`); `-s` selects a software renderer. Without `-s` it tries EGL and dmabuf (not possible here). Optional `xdg-decoration`, shortcut inhibit (`-i`). | SOURCE |
| H.264 | `open-h264` (RFB encoding 50) is decoded with libavcodec through **VAAPI** and drawn by EGL (`src/open-h264.c:97-216`); it is refused with `-s` ("Open H.264 encoding won't work without EGL", `main.c`). So it cannot run in this container. | SOURCE |
| Licence | The repository's own code is **ISC** (`meson.build: license: 'ISC'`, `COPYING`). It also contains a copy of LibVNCClient (`src/rfbproto.c`, `sockets.c`, `tls_*.c`, `sasl.c`, `crypto_*.c`, ...) whose headers say **GNU GPL version 2 or (at your option) later**, and the repository includes `COPYING.GPL`. So the **built program is, in practice, GPL-2-or-later** (BELIEVED: my reading of the file headers, not legal advice). | SOURCE (file headers); conclusion BELIEVED |
| Packaging | Not in the Ubuntu 24.04 archive (round 2 looked at Launchpad on 2026-10-04: no package). It must be built; it needs `aml`, libwayland, xkbcommon, pixman, libdrm, gbm, EGL, GLES2, libavcodec (compile-time even with `-s`). | SOURCE (`meson.build`); round 2 Launchpad |


### 6.1 The other viewers, same questions (short form; the first row of each is TESTED in this harness)

| Question | `remote-viewer` 11.0 | TigerVNC viewer 1.13.1 | FreeRDP 3.32.0 |
|---|---|---|---|
| Window app-id / title (default flags of this harness) | app-id `remote-viewer`, title `node1 (1)` (the server's name plus " (1)"); `--name` chooses the app-id (repo doc, round 1 TESTED) | **no app-id** (X11 class `TigerVNC Viewer`); title `node1 - TigerVNC` (server's name); cannot be chosen by the caller (TESTED; repo doc round 2) | `wlfreerdp3`: `/wm-class` and `/title` set both (TESTED: `bench-rdp`); `xfreerdp3`: X11 class `xfreerdp`, `/title` sets the title |
| Clipboard | node to hub only; UTF-8 garbled (TESTED, 5.9) | both ways, UTF-8 exact, while focused (TESTED) | `xfreerdp3` both ways, `wlfreerdp3` node to hub only (TESTED) |
| Credentials | `.vv` file with the password (mode 0600) or a dialog; **not** the command line (TESTED: URL credentials were ignored) | environment `VNC_USERNAME`, `VNC_PASSWORD` (TESTED here and in round 2) | `/u:` and `/p:` **on the command line**, or `/from-stdin`; I used the command line with test values (UNKNOWN: whether FreeRDP 3 hides them from `ps`) |
| Reconnect | none (TESTED, 5.8) | none (TESTED); the manual lists `-ReconnectOnError`, which did not act on a cut link (repo doc round 2 TESTED) | none (TESTED) |
| Several at once | yes, one process each (TESTED with 20) | yes, TESTED with 20; but a second non-shared viewer disconnects the first on the same server (repo doc round 2) | not tested |
| Licence | LGPL-2.1+ (spice-gtk, repo doc round 1); virt-viewer's own: not read (UNKNOWN) | GPL-2+ (repo doc round 2) | Apache-2.0 (repo doc round 1) |

---

## 7. Combinations that cannot work, and what I tried

1. **cage at 1080p and 4K (TESTED).** `wlr-randr --output HEADLESS-1 --custom-mode 1920x1080` on a running headless cage 0.1.5 makes cage abort with `wlr_scene_output_layout_add_output: Assertion 'solo->scene_output != so' failed`. The headless output is fixed at 1280 x 720, so the cage stack runs only at 720p (`STACK_ONLY_RES=720p` in its file). The same abort happens when a viewer asks the node to resize (5.6). I did not look for a cage fix or a newer cage (UNKNOWN whether a newer cage has the bug).
2. **The TigerVNC viewer needs Xwayland and `xkbcomp` (TESTED).** Hub without Xwayland: `Can't open display:` and sway logs `Cannot find Xwayland binary "/usr/bin/Xwayland"`. Hub with Xwayland but no `/usr/bin/xkbcomp`: Xwayland prints `sh: 1: /usr/bin/xkbcomp: not found` and `XKB: Failed to compile keymap`, and the viewer says `Can't open display: :0`. (The test: a hub sway started inside `unshare -m` with an overlay of `/usr/bin` that holds only a link to `Xwayland`, then `xtigervncviewer 127.0.0.1::5999`; the outputs quoted are from that run.) With both, it ran as a fullscreen X11 window. HUB-OS.md says the hub runs no X11 stack; this is the price of that viewer.
3. **FreeRDP needs a Wayland or X surface (TESTED).** The Ubuntu 24.04 archive has `freerdp3-wayland` (`wlfreerdp3`, prints "has been deprecated" at every start) and `freerdp3-x11` (`xfreerdp3`); an SDL client is not in the archive (`apt-cache search freerdp3` lists `libfreerdp3-3`, `freerdp3-dev`, `freerdp3-shadow-x11`, `freerdp3-wayland`, `freerdp3-x11`). There is no headless FreeRDP client in the package set, so the hub compositor is needed (it is here).
4. **Weston's VNC backend needs a certificate, a PAM service and the exact user name (TESTED; SOURCE 3.3).** Without PAM configuration it refuses every login; with a service file that accepts everything, a user name other than the process's own (`root` here) is refused (`VNC: wrong user 'bench'` in Weston's log). `wlvncc` needs `-t <cert>` and `-A <command>`; the TigerVNC viewer needs `-X509CA` and the environment variables; `remote-viewer` needs the certificate at `/etc/pki/CA/cacert.pem` and a `.vv` file. A real hub would need an account and PAM on every Weston node (UNKNOWN how that fits Hub OS: PAM modules are deleted from the images).
5. **Weston's own screenshot sometimes never finishes (TESTED).** `weston-screenshooter` needs the output to repaint, and Weston's VNC backend repaints only while the viewer's update requests allow it: it finished with `remote-viewer` and the TigerVNC viewer and not with wlvncc on an idle screen (3 of 3 and 2 of 2 runs failed; the retry with a sway reference is described in 5.2). It also never finishes with no viewer connected.
6. **Weston has no seat until a viewer connects (TESTED).** `foot` exits with `no seats available (wl_seat interface too old?)` if started first. The harness starts the scene program after the viewer on the Weston stacks (except for the test window that tolerates it).
7. **wlvncc on cage needs a pointer move before a click (TESTED, 5.4).**
8. **`remote-viewer` and Weston's VNC (TESTED).** It first said `The certificate is not trusted` with the certificate in `$HOME/.pki/CA`; see 3.3. URL credentials (`vnc://user:pass@host`) are ignored and it waits for a dialog.
9. **Not tried:** Weston 16 (the server in the Ubuntu 24.04 archive is 13.0.0); `wlfreerdp3`/`xfreerdp3` with `/dynamic-resolution`, `/gfx` or other graphics options; the SDL FreeRDP client; sway or cage with wayvnc's `-g` (needs a GPU); TLS or RSA-AES logins on wayvnc (round 2 did); driftwm; Sunshine/Moonlight; audio; `x11vnc`/`TigerVNC` servers.

---

## 8. Scale test: 20 simultaneous idle sessions into one hub

**Set-up (TESTED, `scale.sh`):** 20 headless-sway nodes, each at 1080p with wayvnc and a terminal showing a static page (ports 5901 to 5920), one hub sway with a 4K output, 20 viewers (one per node, started 0.7 s apart) tiled in it (not fullscreen), settle 15 s, then 30 s of measurement ("idle"), then one node starts printing 30 lines a second for 20 s ("19 idle + 1 busy"). Three runs per viewer. Everything runs on the same 4 cores. The node side was identical for all viewers (about 1.1 GB for 20 nodes and 0 % CPU when idle).

#### scale test: 20 sessions (20 nodes) into one hub
Memory is PSS (shared libraries counted once) of the processes named; it can miss memory that the hub's Xwayland or compositor keeps in shared buffers, so the last column gives the whole machine's view: how much `MemAvailable` fell with 20 nodes and 20 viewers, minus how much it fell with the 20 nodes alone (792 MB, median of 3 runs). "alive" = viewers still running when the measurement started. CPU is percent of one core. min / median / max over the runs.

| stack | client | sessions | N | alive | phase | hub viewers CPU % | hub compositor CPU % | hub viewers PSS MB | hub compositor PSS MB | all 20 nodes CPU % | all 20 nodes PSS MB | hub side, whole-machine MB |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| wayvnc-sway | remote-viewer | 20 | 3 | 20/20/20 | 20 idle | 0.0 / 0.0 / 0.0 | 0.3 / 0.4 / 0.4 | 1156 / 1156 / 1156 | 37 / 37 / 37 | 0.0 / 0.0 / 0.0 | 1076 / 1076 / 1076 | 1376 / 1462 / 1475 |
| wayvnc-sway | remote-viewer | 20 | 3 | 20/20/20 | 19 idle + 1 busy (30 lines/s) | 75.7 / 79.0 / 81.5 | 0.3 / 0.3 / 0.3 | 1156 / 1156 / 1156 | 37 / 37 / 37 | 125.3 / 127.5 / 131.4 | 1092 / 1092 / 1092 | (same machine state) |
| wayvnc-sway | tigervnc | 20 | 3 | 20/20/20 | 20 idle | 0.0 / 0.0 / 0.0 | 0.4 / 0.4 / 0.4 | 186 / 186 / 187 | 37 / 37 / 37 | 0.0 / 0.0 / 0.0 | 1084 / 1087 / 1108 | 2677 / 2693 / 2697 |
| wayvnc-sway | tigervnc | 20 | 3 | 20/20/20 | 19 idle + 1 busy (30 lines/s) | 34.4 / 34.6 / 36.2 | 0.2 / 0.3 / 0.3 | 186 / 186 / 187 | 37 / 37 / 37 | 89.2 / 90.1 / 99.1 | 1099 / 1102 / 1116 | (same machine state) |
| wayvnc-sway | wlvncc | 20 | 3 | 20/20/20 | 20 idle | 1.1 / 1.2 / 1.2 | 0.0 / 0.0 / 0.0 | 588 / 648 / 652 | 109 / 112 / 117 | 0.0 / 0.0 / 0.0 | 1080 / 1080 / 1082 | 934 / 1007 / 1012 |
| wayvnc-sway | wlvncc | 20 | 3 | 20/20/20 | 19 idle + 1 busy (30 lines/s) | 18.9 / 19.2 / 20.5 | 0.8 / 0.8 / 0.9 | 584 / 644 / 644 | 116 / 116 / 121 | 88.4 / 91.8 / 93.0 | 1095 / 1095 / 1097 | (same machine state) |


**Reading (TESTED):** with all 20 sessions idle the hub spent **about 1 % of one core with wlvncc and about 0 % with the others**; the hub compositor about 0.4 %. Memory is where they differ: by the whole-machine measure the hub side took about **1.0 GB with wlvncc (about 50 MB per session), 1.5 GB with `remote-viewer` (73 MB) and 2.7 GB with the TigerVNC viewer (135 MB, mostly Xwayland, which the PSS of the named processes does not show)**. One busy session among the 19 idle ones cost the hub 19 % (wlvncc), 35 % (TigerVNC) or 79 % (`remote-viewer`) of a core. The 80 node processes (20 compositors, 20 servers, 20 terminals and their shells) are on the same machine; their CPU in the idle phase was 0 %. What this test does **not** show: 20 viewers on 20 *busy* windows (UNKNOWN: by the scroll table one busy window costs the hub 25 to 110 % with a VNC viewer, so 20 would not fit on 4 cores in software; GPU decode is the open question in section 9), 20 separate machines and a real network, or driftwm.

---

## 9. How a GPU encode path could change the ranking (SOURCE only: nothing in this section was measured)

All items were read on 2026-10-05 from the primary file at the stated version. No numbers are given because none were measured.

| Item | What the source says | Label |
|---|---|---|
| neatvnc H.264 encoders | Two implementations are compiled in if available: V4L2 mem-to-mem (`src/enc/h264/v4l2m2m-impl.c`) and FFmpeg with **VAAPI** (`src/enc/h264/ffmpeg-impl.c`: the filter graph is `hwmap=mode=direct:derive_device=vaapi,scale_vaapi=format=nv12:mode=fast`, the encoder is found by name `h264_vaapi`, line 384). The build option is `h264` (`meson_options.txt`, default `auto`). neatvnc v1.0.3 = `783437adc56c2d7ce2cac1ec584acad798594267`. | SOURCE (files in the tag) |
| When neatvnc uses it | Only if the viewer asks for the encoding **Open H.264 (RFB encoding number 50)** and **every frame is a GPU buffer** (`NVNC_BUFFER_GBM_BO`); the comment in `src/server.c:2787` is "h264 is useless for sw frames". Otherwise it falls back to raw / tight / ZRLE. So without a GPU capture path the H.264 encoder is never used. | SOURCE (`src/server.c:2775-2800`) |
| wayvnc switch | `-g, --gpu` "Enable features that require GPU" (`wayvnc.scd`); in the code it turns on the linux-dmabuf screencopy (`src/main.c:2147`). The 24.04 package is too old anyway (0.7.2). | SOURCE (v0.10.2 = `bb837459c75fd3c267d48af170ac7ed2566f67e5`) |
| Which viewers can decode it | **wlvncc**: yes, with libavcodec **through VAAPI** and draws with EGL (`src/open-h264.c:97-216`); refuses it with `-s` ("Open H.264 encoding won't work without EGL", `main.c`). **TigerVNC viewer**: its encodings list contains 50 (TESTED, table "encodings the VNC viewers asked for"); upstream's decoder `common/rfb/H264LibavDecoderContext.cxx` calls `avcodec_find_decoder(AV_CODEC_ID_H264)` and the file has no hardware-acceleration calls (grep for `hwaccel`, `hw_device`, `vaapi` found nothing), so it decodes in software on the hub (BELIEVED from that one file at v1.16.2; the Ubuntu 1.13.1 build was not read). **remote-viewer / gtk-vnc 1.3.1**: no H.264 (`strings` finds no `h264` in `libgvnc-1.0.so.0`, and it does not link libavcodec; TESTED). | SOURCE; TESTED for gtk-vnc |
| Weston RDP | The RDP backend of **Weston 13.0 and 16.0** sends pictures as **RemoteFX (RLGR3)**, **NSCodec** or plain bitmaps, encoded on the CPU (`libweston/backend-rdp/rdp.c`: `rdp_peer_refresh_rfx`, `rdp_peer_refresh_nsc`, `rdp_peer_refresh_raw`); a search of the file for `h264`, `avc`, `gfx` finds nothing in either version. So the RDP stack here **has no GPU encode path to switch on** (not in 13.0, not in 16.0, as far as the backend source shows). It does have a display-control handler for client-asked layouts (`rdpdisp.c`). | SOURCE (gitlab.freedesktop.org, tags 13.0 and 16.0) |
| Weston VNC | Uses neatvnc (`nvnc_*` calls in `libweston/backend-vnc/vnc.c`), so the same neatvnc rules apply; whether Weston hands it GPU buffers: UNKNOWN. | SOURCE; GPU part UNKNOWN |
| Sunshine / NVENC | Sunshine picks "the first encoder that is available"; the choices are `nvenc` (NVIDIA), `quicksync` (Intel), `amdvce` (AMD), `vaapi` (AMD, Intel), `vulkan` (AMD, Intel, NVIDIA; Linux), `software` (CPU) (Sunshine `docs/configuration.md`, option `encoder`, master as of 2026-10-05). NVENC has its own options: preset, two-pass, spatial AQ, VBV increase, realtime scheduling on Windows, split encode over several NVENC units (same file). The document gives **no speed or delay numbers**. | SOURCE (raw file read via curl, 94 408 bytes) |

**How the ranking could change (BELIEVED; this is reasoning, not a measurement):**

1. With wayvnc/neatvnc the **node CPU** for moving content (video, drag, scrolling) is where the biggest change would be: today it is the server's own tight or ZRLE encoding on CPU cores (the `node server` column in the tables); an H.264 encoder on a GPU moves that work off the CPU cores and usually makes the stream smaller for natural video. Whether that helps **text** is the open question: H.264 is a video codec and blurs fine text at low bit rates (BELIEVED; not measured here; the SSIM tables for tight/ZRLE are the base to compare against).
2. The **hub side** would change in the opposite way: today the hub spends CPU on software decoding of tight/ZRLE/RemoteFX in the viewer. With H.264, wlvncc could decode in hardware (VAAPI) but it then also **needs EGL**, so it leaves its `-s` software mode; the TigerVNC viewer would decode in software, which for 20 windows could cost more hub CPU than today (UNKNOWN). So the hub-side ranking of viewers could reverse. How many simultaneous hardware decodes the hub's GPU gives is on HUB-OS.md's unverified list.
3. **Weston RDP** has no GPU path in the source, so on a GPU machine it would not get better while the VNC stack could. RDP's advantage in the current tables (if any) is therefore a software-only advantage.
4. **Sunshine/Moonlight** has hardware encoders as its normal path (NVENC, VAAPI, AMF, Quick Sync, Vulkan; software is only a fallback); it was not part of this benchmark (owner's list), so no software number exists for it here.
5. Input-to-pixel delay: an H.264 pipeline adds an encoder queue and a decoder queue (BELIEVED); how large with these programs: UNKNOWN.



---

## 10. The exact commands to repeat everything

```
cd tools/bench/remote-display
./setup.sh all                      # downloads .deb files (apt-get download via the proxy), dpkg -x into /tmp/bench, builds wayvnc v0.10.2, neatvnc v1.0.3, aml v1.0.0, wlvncc cc0abf8
./metrics/build-tools.sh            # benchapp, pixelwatch, hubclick (C, gcc) and the ssim Go program
./scenes/gen-media.sh               # the text corpus and the test video (fixed content)

./run.sh wayvnc-sway wlvncc idle 1080p 1          # one measurement; result in results/raw/
./run-all.sh                                      # everything, 1080p, 3 runs (STACKS= CLIENTS= SCENES= RESES= RUNS= narrow it)
./campaign.sh                                     # the campaign behind this document (about 5 hours on 4 cores); resumable
./scale.sh wayvnc-sway wlvncc 20 1080p 1          # 20 sessions into one hub
./summarize.py && ./versions.sh > results/versions.txt
```

Everything lives under `/tmp/bench` (`BENCH_TMP=...` moves it). `rm -rf /tmp/bench` removes it all; nothing was installed on the machine. The only things outside `/tmp/bench` are the X11 socket files that Xwayland leaves in `/tmp/.X11-unix` (deleted at the end).

The repository checks (run from the repository root):

```
gofmt -l .
go vet ./...
go test -count=1 ./...
```


---

## 11. Sources read (all 2026-10-05) and what came from where

- **Local git clones read in full for the cited parts** (commit in section 3.2): `any1/wayvnc` v0.10.2, `any1/neatvnc` v1.0.3, `any1/aml` v1.0.0, `any1/wlvncc` `cc0abf8`. Files named in the text.
- **Raw files fetched with curl** (no summarising tool): Weston `libweston/backend-rdp/rdp.c`, `rdpdisp.c`, `rdpclip.c` at tags 13.0 and 16.0, `libweston/backend-vnc/vnc.c` and `libweston/auth.c` at 13.0 from `https://gitlab.freedesktop.org/wayland/weston/-/raw/<tag>/...`; TigerVNC `common/rfb/encodings.h`, `H264Decoder.cxx`, `H264LibavDecoderContext.cxx` at v1.13.1 and v1.16.2 from `https://raw.githubusercontent.com/TigerVNC/tigervnc/<tag>/...`; Sunshine `docs/configuration.md` (master, 94 408 bytes) from `https://raw.githubusercontent.com/LizardByte/Sunshine/master/docs/configuration.md`.
- **Repo documents** quoted without re-reading their sources (their authors were earlier helper agents; the owner's lead has not necessarily confirmed them): `docs/proposals/remote-display.md` (rounds 1 and 2), `docs/driftwm-findings.md`, `docs/proposals/node-helper-api.md`. Items I re-TESTED here are marked TESTED; those I did not are marked "repo doc".
- **From memory, not read in any source (BELIEVED):** the statement that H.264 blurs fine text at low bit rates; the general expectation that a GPU encoder moves encoder CPU load to the GPU; that real logins cost little during a session; the causes given with "BELIEVED" in section 5. Nothing came from a search engine or a summarising fetch tool.
- **GitHub API access** was refused for Sunshine (not enabled for this session), so the Sunshine document list was not read; only the one raw file above.

---

## 12. Questions for the owner

1. **Which hub viewer should hubd start for a normal node?** The measured facts: the TigerVNC viewer is the only Wayland-world-compatible choice that carries the clipboard both ways with UTF-8, but it needs Xwayland, `xkbcomp` and xwayland-satellite on the hub (HUB-OS.md says the hub runs no X11 stack) and takes 2.7x the hub memory of wlvncc at 20 windows. wlvncc is the lightest and Wayland-native but has no clipboard, no reconnect and lower default picture quality. `remote-viewer` is slow on busy windows and has no hub-to-node clipboard. Do you want to keep "no X11 stack" and accept a node-to-hub-only clipboard through wlvncc plus the clipboard bridge of `docs/proposals/node-helper-api.md`?
2. **May we change wlvncc** (for example, to add a clipboard and a reconnect loop, or a fixed title option)? It is ISC with a GPL-2+ bundled library, so the changed program would be GPL-2+ (BELIEVED). A custom viewer needs a design discussion first (CLAUDE.md).
3. **wayvnc's 30 frame limit:** is `-f 60` allowed to be measured? (I left the default.)
4. **Viewer picture quality:** the defaults differ (TigerVNC level 8, wlvncc level 5, `remote-viewer` lossless). Should hubd pass a quality setting, and which? It trades bandwidth and CPU for text sharpness; the tables show the cost of each default.
5. **Do you want the Weston stacks developed further?** Weston VNC needs a real account and PAM on each node and has no clipboard; Weston RDP has the slowest reaction (67 to 131 ms) and no GPU path, but the smoothest video and an `xfreerdp3` clipboard. The test used a PAM file that accepts any password; that is not acceptable on a real node.
6. **Is cage worth any more time?** It cannot do 1080p or 4K here, it crashes when a viewer resizes, and it exposes no clipboard to wayvnc (all TESTED). I would stop using it, unless a newer cage fixes this (UNKNOWN).
7. **December:** may this harness be reused unchanged on the real hardware (with `wayvnc -g` and H.264 turned on)? It would need only a GPU capable of VAAPI on the nodes and the hub (BELIEVED).
8. **A repeat with Weston 16 and FreeRDP options (`/gfx`, `/dynamic-resolution`)?** I did not run them (7.9).
9. **Should the harness be wired into a Make/CI step?** It takes about five hours; I did not wire it into anything.
