# PROPOSAL AND EXPERIMENT: level of detail for many remote windows (Part 4)

**Status: PROPOSAL plus measurements. Nothing here is decided, nothing is in an image, and nothing here changes `HUB-OS.md`.** `HUB-OS.md` wins if this file disagrees with it. Written 2026-10-05 by a helper agent (Claude, session `session_01Kb6L66wDxnkQ4pMAxMZR4E`); the lead opens the pull request. The only code added is the test tools in `tools/bench/remote-display/lod/` (loopback, fake nodes, no real machine). I used no sub-helpers.

**Labels on every item** (same as `docs/proposals/remote-display.md` and `docs/proposals/remote-display-benchmarks.md`):
- **TESTED**: I ran it in this build container; the command is given (section 9 has all of them) and the raw output is in `tools/bench/remote-display/lod/results/`.
- **SOURCE**: read in a primary file; the file path (or link), the exact version or commit, and the date read (2026-10-05) are given. I read these as raw files (`curl` of the raw file or a clone). **I used no summarising fetch tool and no search engine in this part.** A claim of the form "I found no code that does X" is a search of the files named, not a proof; it is marked "SOURCE (negative search)".
- **BELIEVED**: reasoning or memory, not run and not read in a source. Said so each time.
- **UNKNOWN**: nobody checked.

**What this is not.** This container has 4 virtual cores shared with another job, no GPU, no real display, no real network (loopback only). Every number is a software-rendering number. A "node" is a headless `sway` compositor with a `foot` terminal showing a moving scene, captured by wayvnc. The hub's compositor in the viewer tests is also `sway`, **not driftwm** (driftwm is not built here). The numbers say which request style costs more than which, and which behaviours exist. They do not say how fast anything is on the December hardware. Each number is one run (N = 1) of 3 to 10 seconds, except where a count of trials is given. Run-to-run noise: byte counts repeated within a few percent (compare the first and last `normal` row of a table, which are the same request style), but **CPU figures moved by up to 8 points between runs** because the container is shared (wayvnc `normal` was 30, 32 and 33 to 36 % in three runs of the scroll scene), so differences of less than about 8 points of CPU are not evidence.

Parts that came from earlier helpers and that I did **not** re-run: the freeze test and the resize test in `docs/proposals/remote-display-benchmarks.md` (sections 5.5 and 5.6), quoted below with "earlier report".

---

## 1. Plain-words summary (the ten findings that matter most)

1. **The idea is sound and the protocol allows it.** A viewer can spend less on a window that nobody looks at: ask the node for fewer pictures, none at all, or a smaller picture. In the VNC protocol (RFB) the viewer is the one that asks for pictures, so pausing and slowing down are the viewer's job. The protocol text even says a client "may want to regulate the rate" of its requests (SOURCE, section 3.1).
2. **Not asking for pictures really stops the data (TESTED).** With the viewer's requests stopped, the node sent **0 bytes**. Asking at most once per 0.1 s gave 10 pictures per second and about one third of the bytes; once per second gave 1 picture per second and about 1/30 of the bytes. Section 5.1.
3. **It does not stop the work on the node (TESTED + SOURCE).** wayvnc keeps taking pictures of the node's screen and checks every one for changes, whether or not any viewer asked. With the scrolling scene, wayvnc used 25 to 32 % of a core with no requests at all (three runs), against 30 to 36 % with normal requests. At most the compressing and sending is saved, and in one run the difference was inside the noise. So "pause" saves **network and hub work**, not node work.
4. **What does save node work:** wayvnc's `--max-fps` option (33 % of a core at 30 fps, 11 % at 10 fps, 6 % at 5 fps, TESTED), no client connected at all (32 % down to 0.5 %, TESTED, section 5.5), and a smaller screen (TESTED). But `--max-fps` can only be set when wayvnc starts, and it applies to every viewer (SOURCE).
5. **"Smaller size" means the node really changes its screen size (TESTED + SOURCE).** There is no "send me a scaled copy" in VNC. The viewer asks the node to change its own screen size (`SetDesktopSize`). wayvnc agreed (TESTED) and its CPU fell from 33 % to 10 % to 4 % at a quarter and a sixteenth of the pixels, but **all programs on the node saw a new screen and re-arranged their windows**. wayvnc only does this for virtual ("headless") outputs (SOURCE). This sits badly with the brief's "screen size fixed per node in v1".
6. **The TigerVNC viewer already resizes the node by itself, by default (SOURCE + TESTED).** Its `RemoteResize` setting is on by default; when the window was made smaller, the node's screen was changed to the window size (TESTED). It must be switched off (`-RemoteResize=0`) unless that is wanted.
7. **No stock viewer stops asking when its window is hidden (TESTED for two, SOURCE for the others).** TigerVNC and wlvncc kept receiving the same data with the window hidden in the compositor (51.0 vs 50.5 kB/s and 35.1 vs 35.0 kB/s, sway scratchpad). I found no such code in remote-viewer / spice-gtk / gtk-vnc. `xfreerdp` (X11) does send RDP's "suppress output" when its window is minimized or hidden; `wlfreerdp` does not (SOURCE).
8. **ContinuousUpdates (message 150) is the wrong tool for slowing down (TESTED).** Once switched on, the server decides the rate (30 per second here) and the viewer cannot slow it; it can only turn it off, or give a region that has no changes. TigerVNC switches it on for the whole screen as soon as the server offers it (TESTED). A region without changes gave 0 bytes (TESTED), but wayvnc still captured at full rate.
9. **A client `Fence` message can crash wayvnc 0.10.2 / neatvnc 1.0.3 (TESTED, 5 of 8 trials, the same with and without ordering flags).** If a Fence is sent while an update request is still waiting, neatvnc stops with a failed internal check (`Assertion '!client->is_blocked_by_fence' failed`). Fence is therefore unusable as a "ping" for a pause design on this server version, unless continuous updates are on. Whether a release build (without internal checks) behaves differently is UNKNOWN. Section 5.6.
10. **driftwm already tells a window how visible it is, once a second (SOURCE).** In the pinned driftwm commit, a window on screen gets a "frame callback" at the screen's rate and a window completely off screen gets one about every 995 ms. A viewer that asks for the next picture only after its frame callback would slow itself down for far windows with no extra signalling. **No existing viewer does that** (wlvncc: no frame callbacks, SOURCE). That is the best argument for an own viewer (section 7), and it is BELIEVED, not tested (no driftwm here). Cheaper steps exist first (section 8).

Questions for the owner are in section 10; the biggest: may "smaller" change the node's real screen size, and do you want a design discussion about an own viewer?

---

## 2. The problem, in plain words

The hub shows up to 20 machines at once, each a whole desktop in its own window (`HUB-OS.md`, "Windows can stay open 24/7"). Most windows are not being looked at: they are far away on the canvas, behind other windows, or tiny. Three savings are possible:

| Saving | Meaning | Who gains |
|---|---|---|
| **Pause** | Ask for no pictures from a window nobody can see. | Network, hub CPU/GPU (decoding), node (only the compressing and sending part, see finding 3) |
| **Slow** | Fewer pictures per second for far or background windows. | The same |
| **Small** | A smaller picture for a small window. | Network, hub, and the node (fewer pixels to capture and check) |

A fourth, not asked for: **lower picture quality** (JPEG quality) for far windows. neatvnc reads the client's quality request on every `SetEncodings` message (SOURCE: neatvnc `src/server.c:839` and `:879`, v1.0.3, 783437a); I did not test its effect (BELIEVED to reduce bytes of the "tight" encoding only; the "zrle" encoding I used has no quality).

The cluster's brief says the hub must never re-encode or proxy video (`HUB-OS.md`, Core principles). Everything below respects that: only the hub's viewer talks to the node.

---

## 3. What the protocol offers (RFB) and what wayvnc/neatvnc do

Versions: **wayvnc v0.10.2** (`bb837459c75fd3c267d48af170ac7ed2566f67e5`, 2026-09-25) and **neatvnc v1.0.3** (`783437adc56c2d7ce2cac1ec584acad798594267`, 2026-10-03), the tags `HUB-OS.md` pins. Source files below are from these two checkouts (cloned 2026-10-05 by the harness `setup.sh`; I compared the copies the lead left in `/tmp/lod` with the checkouts: identical). The protocol text is `rfbproto.rst` from `github.com/rfbproto/rfbproto`, branch master as of 2026-10-05 (identical to the lead's copy in `/tmp/lod/rfb.rst`; commit hash not recorded).

### 3.1 The five tools, one table

| Tool | What the protocol says (SOURCE: rfbproto master, 2026-10-05) | What neatvnc / wayvnc do (SOURCE unless marked) | Result in my tests |
|---|---|---|---|
| **Paced requests** (`FramebufferUpdateRequest`, message 3, incremental) | The server sends an update when there are changes in the area, "an indefinite period" later; "the client may want to regulate the rate at which it sends incremental FramebufferUpdateRequests" (section `FramebufferUpdateRequest`). | neatvnc counts requests (`n_pending_requests`) and sends one update per request, when there is damage (`src/server.c:1140-1239`, `:1242-1281`). **The region in an incremental request is ignored** (`:1262`). | TESTED: request every 0.1 s gives 9.8 updates/s; every 1 s gives 1.0 updates/s (section 5.1). |
| **No requests** (pause) | Nothing is sent without a request (unless continuous updates are on). | neatvnc keeps a per-client "damage" region that grows while nothing is sent; it is sent whole with the next request (`:1218-1220`, `nvnc__damage_region` `:2965`). wayvnc keeps capturing and neatvnc keeps checking every frame for changes while any client is connected (`src/display.c:104-121` neatvnc; wayvnc `src/main.c:1488-1533`). wayvnc stops capturing only when **no** client is connected (`src/main.c:1739-1741`). | TESTED: 0 bytes; wayvnc CPU only slightly lower (5.1). The first picture after the pause arrived within 30 ms (loopback) and carried the whole accumulated change (5.4). |
| **ContinuousUpdates** (message 150, on/off, with a region) | "If enable-flag is non-zero, then the server can start sending FramebufferUpdate messages as needed for the area" given; the server "must ignore all incremental update requests" while it is on; "if zero, ... must immediately send EndOfContinuousUpdates". | `src/server.c:1732-1759`. On: sends as damage arrives, inside the region only **as a test of whether to send**; the update itself carries all accumulated damage (`:1101-1121` decide, `:1218` take whole `client->damage`), BELIEVED from reading, not tested with damage both inside and outside. Off: answers with EndOfContinuousUpdates. | TESTED: on gives the server's full rate (30/s); a region without changes gives 0 bytes; off gives 0 bytes and one EndOfContinuousUpdates (5.2). |
| **SetDesktopSize / ExtendedDesktopSize** (message 251 / pseudo-encoding -308) | "Requests a change of desktop size"; the server answers with an ExtendedDesktopSize rectangle (initiator 1, status 0 = success). | neatvnc answers at once with status 4 ("request forwarded") if the application's callback accepts (`src/server.c:1920-1966`, `:2008-2045`). wayvnc's callback (`src/main.c:822-840`, `:783-820`) changes the **output** with `wlr-output-management`, **only if the output is virtual ("HEADLESS-", "NOOP-")** (`src/output-management.c:326-331`: otherwise "not resizing output ...: not a headless one") and only if resizing is not disabled with `-R` (`src/main.c:2506`). The size change itself then arrives as a second ExtendedDesktopSize (initiator 0, status 0). | TESTED on a headless sway output: 1280x720 to 640x360 to 320x180 and back worked (5.3). |
| **Fence** (message 248, pseudo-encoding -312) | A synchronisation marker; "BlockBefore: all messages preceding this one must have finished processing before the response is sent." | neatvnc uses it for its own bandwidth estimate (a ping after every frame; `:790-822`, `:2135-2179`, and a "frame dropped" rule at `:1189-1199`). A client Fence is handled in `:2099-2133`: if a request is still waiting it sets a block flag and leaves the message unread; `process_pending_fence` (`:2742-2759`) then asserts that the flag is clear. | TESTED: client Fence while a request waits: **wayvnc aborted in 5 of 8 trials**; with no request waiting, or with continuous updates on, the answer took 0.1 to 18 ms (5.6). |

Two more server-side facts that matter for pacing:
- **wayvnc's own frame cap**: `-f/--max-fps <fps>`, default 30 (`src/main.c:2458`, `:2483`). It is an option of the program, read once at start (`:2563`); `wayvncctl` has no command to change it (SOURCE: `src/ctl-commands.c` lists help, version, event-receive, client-list, client-disconnect, set-desktop-name, output-list, output-cycle, output-set, wayvnc-exit). The capture is limited to twice that rate and the send to that rate (`:2146`, `:1520-1532`).
- **neatvnc's own throttle**: it estimates the link speed with Fence pings and drops frames when more data is in flight than the link carries in about 33 ms (`src/server.c:1189-1199`, "Exceeded bandwidth limit. Dropping frame."). On loopback with the huge "raw" encoding I saw this rule fire (232 times in one 9 s run) and, after a phase change in my client, the stream stalled and did not recover within 4 s (section 5.7). I did not find out whether the cause is the server or my client (UNKNOWN). The "zrle" encoding I used for all other numbers never triggered it.

### 3.2 Build mode matters (found by reading, not yet tested)

Both neatvnc and wayvnc define `NDEBUG` (switching all `assert` calls off) when the Meson `buildtype` is anything but `debug` or `debugoptimized` (SOURCE: `meson.build`, neatvnc lines 24-26, wayvnc line 40). Meson's default is `debug`, so my build (like the harness recipe) has the checks **on**. A release build would have them off, and then the Fence case in 3.1 would not abort but would continue in an inconsistent state (BELIEVED; UNKNOWN what it does). The image recipe has to choose, and the choice changes how failures look. This is a question for the owner (section 10).

---

## 4. What I measured

### 4.1 Set-up (TESTED)

- Programs: unpacked `.deb` files (`dpkg -x`, nothing installed) and source builds in a private folder, made by the harness `tools/bench/remote-display/setup.sh` of branch `origin/bench-remote-display-clean` with its package list cut down to what the node needs (the harness also puts the build tool `meson` into the private folder with `pip install --target`; nothing goes into the system Python). Versions: wayvnc `v0.10.2-bb83745`, neatvnc `v1.0.3-783437a`, sway 1.9, foot 1.16.2, TigerVNC viewer 1.13.1 (`xtigervncviewer`), Xwayland 23.2.6, wlvncc `cc0abf87c37920540f2439a556e6a480c28f8f46` (2026-04-29). Section 9 has the exact commands.
- Node (`lod/node.sh`): headless sway at 1280 x 720 (software renderer), a `foot` terminal fullscreen on it, and `wayvnc -n lodnode -o HEADLESS-1 127.0.0.1 5901` (no password, loopback). Three scenes: **clock** (a small patch of text changing about 30 times a second: little data), **scroll** (`seq` output scrolling the whole window as fast as foot draws: about 30 changed full pictures per second, the heavy case), **idle** (nothing changes).
- Test client (`lod/rfbprobe.py`, new, Python standard library only): speaks RFB 3.8 with security type None, requests "zrle" (or "raw") encoding, reads the message framing and **skips the picture data** (it does not draw). It counts bytes the server sent, updates (a `FramebufferUpdate` with picture data), and reads `/proc/<pid>/stat` for the CPU of wayvnc, sway and foot. It runs a list of "phases" on one connection, so a pause can be followed by a resume.
- CPU percentages are percent of one core over the phase. All three processes (node compositor, terminal, wayvnc) share the same 4 cores with my client and with another job, so small differences (a few points) are noise.

### 4.2 What failed or could not be tested (plain list)

- **A real GPU path, H.264, and every real network effect** (latency, loss, a 10GbE link): UNKNOWN. wayvnc was built without H.264 and without GPU capture (harness recipe), so only software "zrle"/"tight" paths ran.
- **driftwm** was not run. Everything about driftwm is SOURCE (pinned commit `352333a8fa1b22171492d4b71a54102045c9a19d`, `src/render/lifecycle.rs`, read 2026-10-05).
- **Hub-side decoding cost** of a window was not measured (my client skips decoding). That is where a pause would save the most on the hub; UNKNOWN how much.
- **remote-viewer / spice-gtk, gtk-vnc and FreeRDP were not run in this part.** They are SOURCE only here. (The earlier report ran them for other questions.)
- **Weston** was not run in this part; SOURCE only.
- My **raw-encoding runs stalled** after a phase change (section 5.7); I did not find out why.
- The one case where the node's screen got a **non-virtual output** (a real monitor) cannot be tested here; the refusal is SOURCE.
- Whether the **TigerVNC fence replies** or other client messages change server behaviour under pause: not examined beyond the counts in 5.8.
- The clock scene's byte rate in later `normal` phases was sometimes lower than in the first (31 down to 18 to 25 kB/s) for the same request style; I did not find why (UNKNOWN). Compare phases inside one scene by pictures per second, and bytes only between rows of similar position.

## 5. Results (TESTED unless marked)

Every table below is the raw output of `lod/rfbprobe.py`, copied from `tools/bench/remote-display/lod/results/`. Columns: `secs` phase length; `kB/s` bytes the server sent per second (1 kB = 1000 bytes, everything on the wire including framing); `upd/s` pictures (updates with picture data) per second; `requests` update requests my client sent; `first` time from the start of the phase to the first picture; `1st kB` size of that first picture; `cpu%` percent of one core used by sway (node compositor), foot (the scene program) and wayvnc over the phase; `size` the node's screen size at the start and end.

### 5.1 Request pacing and "no requests"

Phases: `normal` (ask again at once after every picture), `paced@0.1` (ask at most every 0.1 s), `paced@1` (every 1 s), `none` (send no request), `normal` again.

**Scene scroll (heavy):**
```
scene=scroll size=1280x720 wayvnc_args=''
cmd: rfbprobe.py --port 5901 --phases normal:6,paced@0.1:6,paced@1:6,none:6,normal:6  --pid sway=24874 --pid foot=24887 --pid wayvnc=24901
# server RFB 003.008 name='lodnode' start size=1280x720 encoding=zrle bpp=32
phase              secs      kB/s    upd/s  updates requests   first  1st kB  cpu%                     size
normal             6.01     344.5     29.8      179      180  0.018s    12.0  sway=9.3 foot=59.8 wayvnc=33.3 1280x720
paced@0.1           6.0     114.4      9.8       59       58  0.004s    11.6  sway=11.3 foot=62.0 wayvnc=33.1 1280x720
paced@1             6.0      11.8      1.0        6        6  0.961s    11.5  sway=9.3 foot=57.7 wayvnc=28.0 1280x720
none                6.0       0.0      0.0        0        0       -       -  sway=10.5 foot=62.5 wayvnc=32.1 1280x720
normal              6.0     345.5     29.8      179      180  0.004s    11.9  sway=10.0 foot=60.3 wayvnc=35.6 1280x720
```

**Scene clock (light):**
```
scene=clock size=1280x720 wayvnc_args=''
cmd: rfbprobe.py --port 5901 --phases normal:6,paced@0.1:6,paced@1:6,none:6,normal:6  --pid sway=23520 --pid foot=23538 --pid wayvnc=23581
# server RFB 003.008 name='lodnode' start size=1280x720 encoding=zrle bpp=32
phase              secs      kB/s    upd/s  updates requests   first  1st kB  cpu%                     size
normal              6.0      30.7     30.0      180      181  0.010s     6.7  sway=3.5 foot=0.8 wayvnc=2.2 1280x720
paced@0.1           6.0       9.8      9.8       59       58  0.017s     1.0  sway=3.3 foot=1.2 wayvnc=1.7 1280x720
paced@1             6.0       0.9      0.8        5        5  0.994s     1.1  sway=3.5 foot=1.0 wayvnc=0.8 1280x720
none                6.0       0.0      0.0        0        0       -       -  sway=3.5 foot=1.0 wayvnc=1.0 1280x720
normal              6.0      24.8     30.0      180      181  0.001s     1.1  sway=3.7 foot=1.0 wayvnc=2.3 1280x720
```

**Scene idle (nothing changes):**
```
scene=idle size=1280x720 wayvnc_args=''
cmd: rfbprobe.py --port 5901 --phases normal:6,paced@0.1:6,paced@1:6,none:6,normal:6  --pid sway=24995 --pid foot=25008 --pid wayvnc=25020
# server RFB 003.008 name='lodnode' start size=1280x720 encoding=zrle bpp=32
phase              secs      kB/s    upd/s  updates requests   first  1st kB  cpu%                     size
normal              6.0       1.1      0.2        1        2  0.006s     6.4  sway=0.0 foot=0.0 wayvnc=0.2 1280x720
paced@0.1           6.0       0.0      0.0        0        0       -       -  sway=0.0 foot=0.0 wayvnc=0.0 1280x720
paced@1             6.0       0.0      0.0        0        0       -       -  sway=0.0 foot=0.0 wayvnc=0.0 1280x720
none                6.0       0.0      0.0        0        0       -       -  sway=0.0 foot=0.0 wayvnc=0.0 1280x720
normal              6.0       0.0      0.0        0        0       -       -  sway=0.0 foot=0.0 wayvnc=0.0 1280x720
```

Reading (all TESTED):
- Bytes follow the request rate almost exactly: about 30, 10, 1, 0 pictures per second gave about 340, 114, 12, 0 kB/s in the scroll scene.
- The **node's CPU hardly moves** with the request rate. In the scroll scene, wayvnc used 33 to 36 % (normal), 33 and 28 % (paced at 0.1 s and 1 s) and 32 % (none) in the kept run; two earlier runs of the same command (outputs not kept) gave 30 to 32 % for `normal` and 25.5 and 24.7 % for `none`. So at most 5 to 8 points of a core (the compressing and sending) are saved, and one run showed no saving. The terminal (foot, about 60 % of a core, it is the scene) and sway (9 to 11 %) are not affected. SOURCE explains it: neatvnc runs its change check on every captured frame while any client is connected (`neatvnc/src/display.c:115`), and wayvnc captures at up to twice `--max-fps` in any case.
- With the idle scene nothing is sent whatever the client does, and wayvnc/sway/foot use 0 % (TESTED): an unchanged screen costs nothing in either direction.
- An earlier report (`remote-display-benchmarks.md` 5.5, not re-run by me) froze each viewer process with `SIGSTOP` at 1080p: bytes went to 0 for every viewer, wayvnc kept using about 50 % of a core against 65 to 82 % running, and the data flowed again after `SIGCONT` (TESTED there). That is "pausing" without any change to the viewer; section 8 uses it.

### 5.2 ContinuousUpdates

Phases: `normal`, `cu` (switch continuous updates on for the whole screen, then send no requests), `curegion@600+400+100+100` (on, for a 100 x 100 region where nothing changes), `curegion@0+0+300+100` (on, for a region that contains the changing clock), `cuoff` (switch off, then send no requests), `normal`.

**Scene clock:**
```
scene=clock size=1280x720 wayvnc_args=''
cmd: rfbprobe.py --port 5901 --phases normal:5,cu:6,curegion@600+400+100+100:6,curegion@0+0+300+100:6,cuoff:6,normal:5  --pid sway=25181 --pid foot=25194 --pid wayvnc=25252
# server RFB 003.008 name='lodnode' start size=1280x720 encoding=zrle bpp=32
phase              secs      kB/s    upd/s  updates requests   first  1st kB  cpu%                     size
normal              5.0      31.0     30.0      150      151  0.008s     6.7  sway=3.6 foot=1.0 wayvnc=2.4 1280x720
cu                 6.01      29.3     29.6      178        0  0.007s     0.8  sway=3.5 foot=1.0 wayvnc=2.3 1280x720
curegion@600+400+100+100    6.0       0.2      0.2        1        0  0.029s     1.0  sway=3.8 foot=1.2 wayvnc=1.2 1280x720
curegion@0+0+300+100    6.0      30.7     30.0      180        0  0.001s     1.1  sway=3.7 foot=1.0 wayvnc=2.3 1280x720
cuoff               6.0       0.2      0.2        1        0  0.001s     1.2  sway=3.5 foot=1.0 wayvnc=1.0 1280x720  eocu=1
normal              5.0      18.3     30.2      151      152  0.001s     1.3  sway=3.4 foot=1.0 wayvnc=2.0 1280x720
```

**Scene scroll:**
```
scene=scroll size=1280x720 wayvnc_args=''
cmd: rfbprobe.py --port 5901 --phases normal:5,cu:6,curegion@600+400+100+100:6,curegion@0+0+300+100:6,cuoff:6,normal:5  --pid sway=26617 --pid foot=26634 --pid wayvnc=26650
# server RFB 003.008 name='lodnode' start size=1280x720 encoding=zrle bpp=32
phase              secs      kB/s    upd/s  updates requests   first  1st kB  cpu%                     size
normal              5.0     337.0     29.8      149      150  0.024s    11.9  sway=9.4 foot=59.8 wayvnc=34.0 1280x720
cu                  6.0     345.9     29.8      179        0  0.009s    11.4  sway=9.2 foot=61.8 wayvnc=32.8 1280x720
curegion@600+400+100+100    6.0       0.0      0.0        0        0       -       -  sway=9.8 foot=61.6 wayvnc=28.3 1280x720
curegion@0+0+300+100    6.0     347.2     29.8      179        0  0.003s    11.5  sway=9.8 foot=59.5 wayvnc=34.3 1280x720
cuoff               6.0       1.9      0.2        1        0  0.014s    11.6  sway=10.0 foot=61.8 wayvnc=29.8 1280x720  eocu=1
normal              5.0     346.0     30.0      150      151  0.003s    11.8  sway=10.2 foot=59.4 wayvnc=34.2 1280x720
```

Reading (TESTED): with continuous updates on, the server alone sets the pace (30 pictures per second, the same as `normal`) and my client sent **0 requests**. A region where nothing changes gives 0 to 0.2 kB/s (in the clock scene one picture arrived when the region was set), so a window can be "parked" with a small region far from any change; but this is a trick: wayvnc kept capturing at full rate (28 % in the scroll scene against 33 to 34 % with the region on the changes), and the node's cost is the same as with `none`, and the TigerVNC viewer cannot be asked to do it (it switches continuous updates on for the whole screen, section 6.1). `cuoff` gave at most one last picture (the one that was in flight) and exactly one `EndOfContinuousUpdates` (the `eocu=1`). Continuous updates **remove** the viewer's ability to slow the server down; the spec text says the server ignores incremental requests while it is on (SOURCE).

### 5.3 A smaller screen (SetDesktopSize)

Phases: `normal`, then `resize@640x360`, `resize@320x180`, `resize@1280x720` (each: send SetDesktopSize for one screen of that size, then ask for pictures normally), `normal`.

**Scene scroll:**
```
scene=scroll size=1280x720 wayvnc_args=''
cmd: rfbprobe.py --port 5901 --phases normal:5,resize@640x360:6,resize@320x180:6,resize@1280x720:6,normal:3  --pid sway=26825 --pid foot=26838 --pid wayvnc=26854
# server RFB 003.008 name='lodnode' start size=1280x720 encoding=zrle bpp=32
phase              secs      kB/s    upd/s  updates requests   first  1st kB  cpu%                     size
normal              5.0     343.3     29.8      149      150  0.011s    12.4  sway=9.4 foot=60.4 wayvnc=33.0 1280x720
resize@640x360      6.0     169.4     29.8      179      181  0.008s    11.8  sway=3.5 foot=51.0 wayvnc=10.2 1280x720->640x360  pseudo_only=1
resize@320x180      6.0      97.2     30.2      181      183  0.027s     5.6  sway=1.8 foot=49.1 wayvnc=4.0 640x360->320x180  pseudo_only=1
resize@1280x720     6.0     340.9     30.2      181      183  0.005s     3.0  sway=9.8 foot=60.3 wayvnc=34.7 320x180->1280x720  pseudo_only=1
normal              3.0     343.9     29.7       89       89  0.025s    11.7  sway=9.0 foot=60.3 wayvnc=30.7 1280x720
# event +-0.99s EndOfContinuousUpdates received
# event +-0.99s ExtendedDesktopSize rect: initiator=0 status=0 size=1280x720
# event +5.00s SetDesktopSize sent 640x360 (screen id 0)
# event +5.01s ExtendedDesktopSize rect: initiator=1 status=4 size=640x360
# event +5.15s ExtendedDesktopSize rect: initiator=0 status=0 size=640x360
# event +5.15s ExtendedDesktopSize rect: initiator=0 status=0 size=640x360
# event +11.01s SetDesktopSize sent 320x180 (screen id 0)
# event +11.01s ExtendedDesktopSize rect: initiator=1 status=4 size=320x180
# event +11.07s ExtendedDesktopSize rect: initiator=0 status=0 size=320x180
# event +11.07s ExtendedDesktopSize rect: initiator=0 status=0 size=320x180
# event +17.01s SetDesktopSize sent 1280x720 (screen id 0)
# event +17.01s ExtendedDesktopSize rect: initiator=1 status=4 size=1280x720
# event +17.06s ExtendedDesktopSize rect: initiator=0 status=0 size=1280x720
# event +17.07s ExtendedDesktopSize rect: initiator=0 status=0 size=1280x720
```

**Scene clock:**
```
scene=clock size=1280x720 wayvnc_args=''
cmd: rfbprobe.py --port 5901 --phases normal:5,resize@640x360:6,resize@320x180:6,resize@1280x720:6,normal:3  --pid sway=27008 --pid foot=27021 --pid wayvnc=27064
# server RFB 003.008 name='lodnode' start size=1280x720 encoding=zrle bpp=32
phase              secs      kB/s    upd/s  updates requests   first  1st kB  cpu%                     size
normal             5.01      31.0     30.0      150      151  0.006s     6.8  sway=3.2 foot=1.0 wayvnc=2.4 1280x720
resize@640x360      6.0      32.9     30.3      182      184  0.009s     0.9  sway=2.0 foot=0.8 wayvnc=2.5 1280x720->640x360  pseudo_only=1
resize@320x180      6.0      25.9     30.2      181      183  0.030s     1.0  sway=1.3 foot=1.2 wayvnc=2.2 640x360->320x180  pseudo_only=1
resize@1280x720     6.0      33.9     30.3      182      184  0.009s     0.9  sway=3.8 foot=1.2 wayvnc=2.8 320x180->1280x720  pseudo_only=1
normal              3.0      31.4     30.0       90       90  0.023s     1.0  sway=3.7 foot=1.0 wayvnc=2.3 1280x720
```

Reading (TESTED):
- wayvnc accepted all three requests on the headless output. The immediate answer was `initiator=1 status=4` ("request forwarded") and the real change followed, twice, as `initiator=0 status=0` 50 to 150 ms later (see the events under the table).
- In the scroll scene the **wayvnc CPU fell from 33 % to 10 % to 4 %** and sway from 9.4 % to 3.5 % to 1.8 % at one quarter and one sixteenth of the pixels: roughly in proportion to the number of pixels. Bytes fell less (343 to 169 to 97 kB/s) because the terminal re-lays its text at a smaller size, so each picture is a similar amount of text per pixel (a property of this scene).
- foot's own use fell only from 60 % to 49 to 51 % (it still has to scroll text). In the clock scene (a small changing patch) there was almost no saving.
- **The node's programs saw a new screen**: foot re-flowed. This is a real change of the node's desktop, not a scaled copy. Brief: "the screen size is fixed per node in v1" (`HUB-OS.md`); this conflicts with it. A node whose screen is a real monitor is refused by wayvnc (SOURCE, `output-management.c:326`; not testable here).
- The earlier report (5.6, not re-run) also found that, with the same wayvnc on sway, the TigerVNC viewer made the node change its screen to the window size (960 x 540 from a half-size window), `remote-viewer` made it 1920 x 1033, and that `wlvncc` and the FreeRDP clients only scaled the picture.
- Pausing and resizing together was not tested.

### 5.4 Resume: how long until the picture comes back, and how big is it

Phases: `normal` 3 s, `none` 10 s, `normal` 3 s, `none` 3 s, `paced@1` 3 s, `normal` 3 s.

**Scene scroll:**
```
scene=scroll size=1280x720 wayvnc_args=''
cmd: rfbprobe.py --port 5901 --phases normal:3,none:10,normal:3,none:3,paced@1:3,normal:3  --pid sway=30489 --pid foot=30502 --pid wayvnc=30522
# server RFB 003.008 name='lodnode' start size=1280x720 encoding=zrle bpp=32
phase              secs      kB/s    upd/s  updates requests   first  1st kB  cpu%                     size
normal              3.0     342.9     30.0       90       91  0.015s    12.2  sway=8.7 foot=61.3 wayvnc=32.0 1280x720
none               10.0       1.1      0.1        1        0  0.020s    11.1  sway=9.2 foot=61.5 wayvnc=28.4 1280x720
normal              3.0     345.5     30.3       91       92  0.002s    12.4  sway=9.0 foot=60.2 wayvnc=33.3 1280x720
none                3.0       3.9      0.3        1        0  0.025s    11.8  sway=8.3 foot=60.6 wayvnc=25.0 1280x720
paced@1             3.0      11.5      1.0        3        3  0.007s    11.3  sway=9.0 foot=60.3 wayvnc=26.3 1280x720
normal              3.0     346.2     30.0       90       91  0.003s    11.4  sway=9.3 foot=60.6 wayvnc=32.9 1280x720
```

**Scene clock:**
```
scene=clock size=1280x720 wayvnc_args=''
cmd: rfbprobe.py --port 5901 --phases normal:3,none:10,normal:3,none:3,paced@1:3,normal:3  --pid sway=29257 --pid foot=29270 --pid wayvnc=29321
# server RFB 003.008 name='lodnode' start size=1280x720 encoding=zrle bpp=32
phase              secs      kB/s    upd/s  updates requests   first  1st kB  cpu%                     size
normal              3.0      31.1     30.0       90       91  0.008s     6.8  sway=3.7 foot=1.0 wayvnc=2.7 1280x720
none               10.0       0.1      0.1        1        0  0.007s     0.9  sway=3.4 foot=0.9 wayvnc=0.9 1280x720
normal              3.0      31.1     30.0       90       91  0.001s     1.1  sway=3.3 foot=1.3 wayvnc=2.3 1280x720
none                3.0       0.3      0.3        1        0  0.007s     1.0  sway=3.7 foot=0.7 wayvnc=1.0 1280x720
paced@1             3.0       1.0      1.0        3        3  0.006s     1.0  sway=3.3 foot=1.3 wayvnc=1.0 1280x720
normal              3.0      32.0     30.3       91       92  0.001s     0.9  sway=3.3 foot=1.0 wayvnc=2.7 1280x720
```

Reading (TESTED): after 10 s of no requests, the first picture came within 2 to 30 ms (loopback, so add the real network round trip). Its size (`1st kB`) is that of one normal picture: neatvnc sends the **accumulated change as one update**, not 10 seconds of pictures. A paused window therefore costs one picture on resume, never a backlog. (The `none` rows show one picture of about 1 to 12 kB: that is the answer to the request that was still waiting when the phase began.)

### 5.5 Server work with no client at all

```
scene=scroll size=1280x720 wayvnc_args=''
cpu% over 6 s WITH a client (normal requests): sway=8.5 foot=59.7 wayvnc=32.0
cpu% over 6 s with NO client connected:        sway=7.0 foot=63.7 wayvnc=0.5
```

Reading (TESTED): wayvnc used 32 % of a core with a client and **0.5 % with none**; sway and foot (the scene) carried on as before. This is the only state in which wayvnc stops capturing (SOURCE `wayvnc/src/main.c:1739-1741`: "Stopping screen capture" when the client count reaches 0). It also means a viewer that **disconnects** a hidden window frees the node completely, at the price of a new connection when the window comes back (BELIEVED to take well under a second on a LAN; the handshake was not timed; the brief's "sessions are non-shared" means the node can serve only one hub viewer at a time anyway, SOURCE `HUB-OS.md`).

### 5.6 Fence

First, the effect of wayvnc's own frame cap (`wayvnc -f 30 | 10 | 5`, scroll scene, phases `normal:6,none:6`):

```
scene=scroll size=1280x720 wayvnc_args='-f 30'
cmd: rfbprobe.py --port 5901 --phases normal:6,none:6  --pid sway=28798 --pid foot=28811 --pid wayvnc=28827
# server RFB 003.008 name='lodnode' start size=1280x720 encoding=zrle bpp=32
phase              secs      kB/s    upd/s  updates requests   first  1st kB  cpu%                     size
normal              6.0     345.1     29.8      179      180  0.032s    13.3  sway=9.3 foot=60.8 wayvnc=33.0 1280x720
none                6.0       1.9      0.2        1        0  0.005s    11.6  sway=10.2 foot=58.5 wayvnc=27.8 1280x720
```
```
scene=scroll size=1280x720 wayvnc_args='-f 10'
cmd: rfbprobe.py --port 5901 --phases normal:6,none:6  --pid sway=28934 --pid foot=28947 --pid wayvnc=28959
# server RFB 003.008 name='lodnode' start size=1280x720 encoding=zrle bpp=32
phase              secs      kB/s    upd/s  updates requests   first  1st kB  cpu%                     size
normal              6.0     118.0     10.2       61       62  0.011s    12.1  sway=8.3 foot=60.5 wayvnc=10.8 1280x720
none                6.0       1.9      0.2        1        0  0.047s    11.6  sway=7.3 foot=60.5 wayvnc=8.8 1280x720
```
```
scene=scroll size=1280x720 wayvnc_args='-f 5'
cmd: rfbprobe.py --port 5901 --phases normal:6,none:6  --pid sway=29076 --pid foot=29089 --pid wayvnc=29109
# server RFB 003.008 name='lodnode' start size=1280x720 encoding=zrle bpp=32
phase              secs      kB/s    upd/s  updates requests   first  1st kB  cpu%                     size
normal              6.0      57.1      5.2       31       32  0.008s    12.4  sway=6.7 foot=61.3 wayvnc=6.0 1280x720
none                6.0       2.1      0.2        1        0  0.044s    12.4  sway=7.3 foot=63.3 wayvnc=4.7 1280x720
```

Reading (TESTED): wayvnc's CPU went from 33 % (30 fps) to 11 % (10 fps) to 6 % (5 fps), and in the `none` phase it still used 28 %, 9 % and 5 %: the server-side cap is the only one of the pacing tools that lowers the node's cost, and it is global.

Now Fence. A client Fence request with flag BlockBefore, once a second, while the client has stopped asking:

After normal requests (scroll scene; the request that was waiting when the phase started is still waiting). **In this kept run wayvnc aborted** (the last line is its message; my client saw "Connection reset by peer"):
```
scene=scroll size=1280x720 wayvnc_args=''
cmd: rfbprobe.py --port 5901 --phases normal:3,fence:5  --pid sway=28170 --pid foot=28183 --pid wayvnc=28199
Traceback (most recent call last):
  File "tools/bench/remote-display/lod/rfbprobe.py", line 398, in <module>
    main()
  File "tools/bench/remote-display/lod/rfbprobe.py", line 394, in main
    pr.run()
  File "tools/bench/remote-display/lod/rfbprobe.py", line 335, in run
    self.pump(sel, 0.005)
  File "tools/bench/remote-display/lod/rfbprobe.py", line 349, in pump
    d = self.sock.recv(1 << 20)
        ^^^^^^^^^^^^^^^^^^^^^^^
ConnectionResetError: [Errno 104] Connection reset by peer
# server log lines that mention an assertion or abort:
wayvnc: ../subprojects/neatvnc/src/server.c:2745: process_pending_fence: Assertion `!client->is_blocked_by_fence' failed.
```

Idle scene, normal requests before it: the Fence is **not answered at all** (empty `fence_rtt_ms`), because the server waits for the earlier update request to be answered and the idle screen never changes:
```
scene=idle size=1280x720 wayvnc_args=''
cmd: rfbprobe.py --port 5901 --phases normal:3,fence:5  --pid sway=28253 --pid foot=28266 --pid wayvnc=28282
# server RFB 003.008 name='lodnode' start size=1280x720 encoding=zrle bpp=32
phase              secs      kB/s    upd/s  updates requests   first  1st kB  cpu%                     size
normal              3.0       2.1      0.3        1        2  0.006s     6.4  sway=0.0 foot=0.0 wayvnc=0.3 1280x720
fence               5.0       0.0      0.0        0        0       -       -  sway=0.0 foot=0.0 wayvnc=0.0 1280x720  fence_rtt_ms=[]
```

With continuous updates on (no request can be waiting), the Fence is answered at once:
```
scene=idle size=1280x720 wayvnc_args=''
cmd: rfbprobe.py --port 5901 --phases cu:3,fence:5  --pid sway=28360 --pid foot=28373 --pid wayvnc=28389
# server RFB 003.008 name='lodnode' start size=1280x720 encoding=zrle bpp=32
phase              secs      kB/s    upd/s  updates requests   first  1st kB  cpu%                     size
cu                  3.0       2.1      0.3        1        0  0.010s     6.4  sway=0.0 foot=0.0 wayvnc=0.3 1280x720
fence               5.0       0.0      0.0        0        0       -       -  sway=0.0 foot=0.0 wayvnc=0.0 1280x720  fence_rtt_ms=[0.3, 0.2, 0.2, 0.2, 0.2]
```
```
scene=scroll size=1280x720 wayvnc_args=''
cmd: rfbprobe.py --port 5901 --phases cu:2,fence:4  --pid sway=28567 --pid foot=28580 --pid wayvnc=28596
# server RFB 003.008 name='lodnode' start size=1280x720 encoding=zrle bpp=32
phase              secs      kB/s    upd/s  updates requests   first  1st kB  cpu%                     size
cu                  2.0     337.5     30.0       60        0  0.022s    13.1  sway=10.0 foot=59.4 wayvnc=37.0 1280x720
fence               4.0     345.9     29.7      119        0  0.023s    12.2  sway=10.2 foot=60.5 wayvnc=36.5 1280x720  fence_rtt_ms=[17.9, 13.6, 0.3, 2.1]
```

After switching continuous updates off:
```
scene=scroll size=1280x720 wayvnc_args=''
cmd: rfbprobe.py --port 5901 --phases cu:2,cuoff:1,fence:4  --pid sway=28471 --pid foot=28484 --pid wayvnc=28496
# server RFB 003.008 name='lodnode' start size=1280x720 encoding=zrle bpp=32
phase              secs      kB/s    upd/s  updates requests   first  1st kB  cpu%                     size
cu                  2.0     336.8     29.9       60        0  0.026s    11.9  sway=9.5 foot=57.9 wayvnc=32.9 1280x720
cuoff               1.0       0.0      0.0        0        0       -       -  sway=10.0 foot=61.0 wayvnc=27.0 1280x720  eocu=1
fence               4.0       0.0      0.0        0        0       -       -  sway=10.0 foot=62.2 wayvnc=31.5 1280x720  fence_rtt_ms=[0.2, 0.2, 0.2, 2.6]
```

The crash, repeated 8 times on fresh nodes (`lod/fence-trials.sh 8`, phases `normal:2,fence:3`):
```
trial 1: ok   
trial 2: ok   
trial 3: CRASHED   wayvnc: ../subprojects/neatvnc/src/server.c:2745: process_pending_fence: Assertion `!client->is_blocked_by_fence' failed
trial 4: ok   
trial 5: CRASHED   wayvnc: ../subprojects/neatvnc/src/server.c:2745: process_pending_fence: Assertion `!client->is_blocked_by_fence' failed
trial 6: CRASHED   wayvnc: ../subprojects/neatvnc/src/server.c:2745: process_pending_fence: Assertion `!client->is_blocked_by_fence' failed
trial 7: CRASHED   wayvnc: ../subprojects/neatvnc/src/server.c:2745: process_pending_fence: Assertion `!client->is_blocked_by_fence' failed
trial 8: CRASHED   wayvnc: ../subprojects/neatvnc/src/server.c:2745: process_pending_fence: Assertion `!client->is_blocked_by_fence' failed
phases=normal:2,fence:3  wayvnc aborted in 5 of 8 trials
```

Reading: (TESTED) a client Fence while a request is waiting either waits until the request is answered (spec-consistent: BlockBefore means "everything before me has finished", and a request is finished only when answered; idle screen = never) or, if the answer is racing with it, **aborts wayvnc**. Which of the two happens depends on timing. (SOURCE) the abort is the assert in `process_pending_fence`. A pause design must therefore not use client Fence on this server, or only with continuous updates on. The same 8 trials with Fence flags 0 (no ordering at all, `lod/fence-trials.sh 8 normal:2,fence:3 0`, result file `fence-trials-flags0.txt`) also aborted wayvnc 5 times in 8, as expected from the code, whose first check ignores the flags (`:2099-2110`, SOURCE).
Fence as a way to learn "the server has stopped sending" after a pause is therefore not available; the usable signal is simpler: **the first update after the pause**.

### 5.7 Raw encoding (the stall I could not explain)

```
scene=scroll size=1280x720 wayvnc_args=''
cmd: rfbprobe.py --port 5901 --enc raw --phases normal:6,paced@0.1:6,none:3  --pid sway=28670 --pid foot=28683 --pid wayvnc=28695
# server RFB 003.008 name='lodnode' start size=1280x720 encoding=raw bpp=32
phase              secs      kB/s    upd/s  updates requests   first  1st kB  cpu%                     size
normal              6.0    1320.0      2.8       17       18  0.049s  3686.4  sway=9.8 foot=59.3 wayvnc=27.2 1280x720
paced@0.1           6.0       0.0      0.0        0        0       -       -  sway=9.8 foot=60.5 wayvnc=28.3 1280x720
none               3.01       0.0      0.0        0        0       -       -  sway=9.6 foot=60.2 wayvnc=29.6 1280x720
```

With "raw" (3.7 MB per picture) the server's own bandwidth rule dropped frames (232 "Exceeded bandwidth limit" lines in a 9 s run with debug logging), the rate was 3 to 7 pictures per second, and after the switch to `paced@0.1` the connection delivered **nothing** for 4 to 6 s. I do not know whether a bug is in neatvnc's bookkeeping, in my reply to the server's Fence pings, or in my phase logic (UNKNOWN). It does not affect the other numbers, which all use "zrle", but it shows that the server has its **own** pacing that a viewer's pacing meets.

### 5.8 Two stock viewers, window made smaller, hidden, shown again

Test (`lod/viewer-test.sh`): a second headless sway is the hub; the viewer is opened full size; made a floating 640 x 360 window; moved to the sway scratchpad (hidden: sway says `visible False`); shown again. A counting relay (`tools/bench/remote-display/metrics/tcpmeter.py` from the clean branch) sits between viewer and wayvnc. The scene is the clock (30 changes a second, so any pause would show clearly). (The shell-error lines at the end of the outputs name the script `tigervnc-test.sh`: that is the same script before I renamed it `viewer-test.sh` and added the `VIEWER` switch for wlvncc.)

**TigerVNC 1.13.1 (`xtigervncviewer`, through Xwayland):**
```
# TigerVNC viewer test; wayvnc log lines about the viewer:
DEBUG: ../subprojects/neatvnc/src/server.c: 885: Client 0x5654dae286c0 set encodings: cursor,desktop-size,extended-desktop-size,qemu-led-state,vmware-led-state,desktop-name,extended-clipboard,continuo
Info: Choosing tight encoding for client 0x5654dae286c0
DEBUG: ../subprojects/neatvnc/src/server.c: 1974: Sending extended desktop resize rect: 1280x720: success
# window as opened:
  window: lodnode - TigerVNC rect 1280 x 720 visible True floating con
full size, visible                     50.5 kB/s server->client
# after floating + resize to 640x360:
  window: lodnode - TigerVNC rect 640 x 360 visible True floating floating_con
DEBUG: ../subprojects/neatvnc/src/server.c: 1974: Sending extended desktop resize rect: 1280x720: success
DEBUG: ../subprojects/neatvnc/src/server.c: 2032: Client requested resize to 640x360, result: 4
DEBUG: ../subprojects/neatvnc/src/server.c: 1974: Sending extended desktop resize rect: 640x360: request forwarded
DEBUG: ../subprojects/neatvnc/src/server.c: 1974: Sending extended desktop resize rect: 640x360: success
window 640x360 (after 3 s settle)      50.1 kB/s server->client
# after move to scratchpad (hidden):
  window: lodnode - TigerVNC rect 640 x 360 visible False floating floating_con
hidden (scratchpad)                    51.0 kB/s server->client
# after scratchpad show:
  window: lodnode - TigerVNC rect 640 x 360 visible True floating floating_con
shown again                            51.9 kB/s server->client
# client messages seen in the first 4 KB the viewer sent (type:count):
   {'SetEncodings': 1, 'FBUpdateRequest': 2, 'ClientCutText': 1, 'Fence': 230, 'EnableContinuousUpdates': 2, 'PointerEvent': 1} first messages: ['SetEncodings', 'FBUpdateRequest(incremental=0)', 'ClientCutText', 'Fence', 'FBUpdateRequest(incremental=1)', 'EnableContinuousUpdates(enable=1,region=(0, 0, 1280, 720))', 'PointerEvent', 'EnableContinuousUpdates(enable=1,region=(0, 0, 1280, 720))', 'Fence', 'Fence', 'Fence', 'Fence']
./tigervnc-test.sh: line 97: 16638 Segmentation fault      XDG_RUNTIME_DIR=$HRT sway -c "$HRT/sway.conf" > $D/hubsway.log 2>&1
```

**wlvncc `cc0abf8` (Wayland native):**
```
# TigerVNC viewer test; wayvnc log lines about the viewer:
DEBUG: ../subprojects/neatvnc/src/server.c: 885: Client 0x5642d2cec6c0 set encodings: tight,zrle,copyrect,hextile,rre,raw,desktop-size,qemu-extended-key-event
Info: Choosing tight encoding for client 0x5642d2cec6c0
# window as opened:
  window: lodnode rect 1280 x 720 visible True floating con
full size, visible                     35.0 kB/s server->client
# after floating + resize to 640x360:
  window: lodnode rect 640 x 360 visible True floating floating_con
window 640x360 (after 3 s settle)      34.9 kB/s server->client
# after move to scratchpad (hidden):
  window: lodnode rect 640 x 360 visible False floating floating_con
hidden (scratchpad)                    35.1 kB/s server->client
# after scratchpad show:
  window: lodnode rect 640 x 360 visible True floating floating_con
shown again                            36.4 kB/s server->client
# client messages seen in the first 4 KB the viewer sent (type:count):
   {'SetPixelFormat': 1, 'SetEncodings': 1, 'FBUpdateRequest': 397} first messages: ['SetPixelFormat', 'SetEncodings', 'FBUpdateRequest(incremental=0)', 'FBUpdateRequest(incremental=1)', 'FBUpdateRequest(incremental=1)', 'FBUpdateRequest(incremental=1)', 'FBUpdateRequest(incremental=1)', 'FBUpdateRequest(incremental=1)', 'FBUpdateRequest(incremental=1)', 'FBUpdateRequest(incremental=1)', 'FBUpdateRequest(incremental=1)', 'FBUpdateRequest(incremental=1)']
./viewer-test.sh: line 100: 18426 Segmentation fault      XDG_RUNTIME_DIR=$HRT sway -c "$HRT/sway.conf" > $D/hubsway.log 2>&1
```

Reading (TESTED):
- **Neither viewer pauses when hidden**: 51.0 vs 50.5 kB/s (TigerVNC), 35.1 vs 35.0 kB/s (wlvncc). (This is sway's scratchpad; driftwm may behave differently, UNKNOWN.)
- **TigerVNC resized the node** to its window (wayvnc log: "Client requested resize to 640x360", answer 4 then success) without being asked to. Its messages (first 4 KB the viewer sent): SetEncodings (with fence, continuous-updates and extended-desktop-size), a full non-incremental request, then `EnableContinuousUpdates(enable=1, whole screen)`, and many Fence messages (230 in the first 4 KB; BELIEVED to be its replies to neatvnc's ping after every frame, SOURCE `server.c:790-822`; I did not check the request bit of each). It asked for "tight" encoding.
- **wlvncc** sent one SetPixelFormat, one SetEncodings (tight, zrle, copyrect, hextile, rre, raw, desktop-size; **not** extended-desktop-size, fence or continuous-updates) and then **a new incremental request after every update** (397 requests in the first 4 KB). It never resizes the node.
- The segmentation fault line at the end of the output is the hub's sway exiting after `swaymsg exit`; no core file is made (`ulimit -c 0`).

---

## 6. What each viewer can do (the hub side)

Versions: TigerVNC 1.13.1 (tag `v1.13.1`; the files in `/tmp/lod` are byte-identical to the tag, checked), gtk-vnc 1.3.1 (`gitlab.gnome.org/GNOME/gtk-vnc` tag `v1.3.1`), spice-gtk 0.42 (`gitlab.freedesktop.org/spice/spice-gtk` tag `v0.42`), virt-viewer 11.0 (`gitlab.com/virt-viewer/virt-viewer` tag `v11.0`), wlvncc `cc0abf8` (2026-04-29), FreeRDP 3.32.0 (`github.com/FreeRDP/FreeRDP` tag `3.32.0`), Weston 13.0.0 (`gitlab.freedesktop.org/wayland/weston` tag `13.0.0`). These are the versions of the Ubuntu 24.04 packages the earlier harness used, except spice-gtk (its Ubuntu version number was not looked up: UNKNOWN whether it is 0.42) and wlvncc (built from source).

### 6.1 Viewers speaking VNC

| Viewer | Pause when hidden | Slow / pace | Smaller node screen | Other | Label |
|---|---|---|---|---|---|
| **wlvncc** (`any1/wlvncc`, Wayland native) | **No.** Asks again after every update (`src/rfbproto.c:2350`; `src/vnc.c:303-310`); no Wayland frame callbacks at all (`src/main.c`: no `wl_surface_frame`, negative search); hidden window kept receiving (5.8). | **No.** No option, no continuous updates (no `ContinuousUpdates` or `Fence` in `src/` outside the vendored `include/rfbproto.h`, negative search; it does not request those encodings, 5.8). | **No.** The code to send SetDesktopSize exists in the bundled library (`src/rfbproto.c:1664`, `SendExtDesktopSize`) but nothing in `src/main.c` calls it, and it does not request `extended-desktop-size` (5.8). It scales the picture to the window with a Wayland viewport. | Single window per process; no reconnect (earlier report 6). | SOURCE + TESTED |
| **TigerVNC viewer** (`xtigervncviewer`, X11, needs Xwayland) | **No** (5.8). I found no minimize/hide handling in `vncviewer/CConn.cxx`, `DesktopWindow.cxx`, `Viewport.cxx` (negative search; only a menu item that minimizes the window). | **No** control; switches on continuous updates for the whole screen as soon as the server offers them (`common/rfb/CConnection.cxx:527-531`; TESTED 5.8), so the server sets the pace. Without them it asks again after every update (`:772-815`). | **Yes, automatic.** `RemoteResize` default **true** (`vncviewer/parameters.cxx:127-130`): half a second after the window size changes it sends SetDesktopSize (`DesktopWindow.cxx:660-680`, `:1284-1410`). TESTED 5.8 and in the earlier report 5.6. Switch off with `-RemoteResize=0`. A fixed size can be requested with `-DesktopSize=WxH` (`:1241-1255`). | Runs through Xwayland, which `HUB-OS.md` says the hub does not run; needs `/usr/bin/xkbcomp` (earlier report 7). | SOURCE + TESTED |
| **remote-viewer** with VNC (gtk-vnc 1.3.1) | I found no hide handling (negative search of `src/vncdisplay.c`). | **No** continuous updates, no fence code at all (`grep` of `src/vncconnection.c` finds none). | Has `SetDesktopSize` (`src/vncconnection.c:96`, `:6597`) and a `allow_resize` property (`src/vncdisplay.c`); earlier report 5.6 TESTED: the node got 1920 x 1033 (the drawing area), the window stayed full size. | | SOURCE (negative) + earlier TESTED |
| **remote-viewer** with SPICE (spice-gtk 0.42, virt-viewer 11.0) | I found no hide handling in `src/spice-widget.c` (negative; it only switches its own widget stack). Display data is **pushed** by the server; there is no per-picture request to hold back. | SPICE has a "stream report" the viewer sends about dropped video frames (`src/channel-display.c:1296-1310`, `:1494-1540`): the server may lower the video quality or bit rate (BELIEVED; the server side was not read). Not a rate limit the user can set. | The viewer can ask the **guest** to change resolution or switch a monitor off, through the guest agent: `spice_main_channel_send_monitor_config` (`src/channel-main.c:1116`), `spice_main_channel_update_display_enabled` (`:3145`), used by virt-viewer's auto-resize (`src/virt-viewer-display-spice.c:37-46`, `:217-276`, `:101`). Needs the SPICE agent in the guest. | Meant for the VM host's guests only (`HUB-OS.md`). | SOURCE |

### 6.2 Viewers speaking RDP (not planned by the brief; listed because the task asks)

RDP has two built-in equivalents:
- **Suppress output** (the client tells the server to stop sending screen updates; the "resume" form carries the rectangle to refresh). On the wire: a data PDU `SUPPRESS_OUTPUT` with `allowDisplayUpdates` 0 or 1 and, for 1, left/top/right/bottom (FreeRDP `libfreerdp/core/update.c:1438-1475`). A server may ignore it (FreeRDP's server says "ignoring suppress output request from client" when its setting is off, `:2631-2634`). (SOURCE, FreeRDP 3.32.0.)
- **Display control channel / monitor layout** (the client tells the server its monitor sizes): FreeRDP's client sends it from `channels/disp/client/disp_main.c`, clamping each monitor to 200..8192 pixels per side (`:105-118`); the Wayland client waits at least 500 ms between two resizes (`client/Wayland/wlf_disp.c:30`). (SOURCE.) Which Linux RDP servers honour either message: Weston does (below); others UNKNOWN.

| Item | Result | Label |
|---|---|---|
| `xfreerdp3` (X11) | Sends suppress output **on** when its window is unmapped or minimized, **off** when mapped (`client/X11/xf_event.c:940`, `:979`, `:1168`). So a hidden window pauses by itself. | SOURCE |
| `wlfreerdp3` (Wayland, "deprecated") | I found no suppress-output call in `client/Wayland/*` (negative search). Sends display-control layout on window configure events (`wlf_disp.c`, `wlfreerdp.c:445-447`). | SOURCE (negative) |
| Weston RDP backend 13.0.0 (server) | Honours suppress output: it clears a flag and then sends no updates to that peer (`libweston/backend-rdp/rdp.c:1572-1581`, used at `:294-296`); the area in the resume message is **not used** (the handler ignores it), and I found no refresh handler (`RefreshRect = TRUE` only is set, `:1702`), so BELIEVED a resumed window shows stale pixels until something changes. Accepts the monitor layout (`rdpdisp.c:152-281`; `:1716` unless started with `no_clients_resize`). Weston keeps repainting its output while suppressed (`:285-299`). The earlier report 5.5 found a frozen RDP client made Weston's RDP node use 0 % CPU, because the compositor stops sending frame callbacks to its application. | SOURCE + earlier TESTED |
| Weston VNC backend 13.0.0 (server, uses neatvnc **0.7.1**, earlier report) | Repaints whenever there are peers (`libweston/backend-vnc/vnc.c:1007-1030`); with no peer it powers the output off. Accepts SetDesktopSize if the output was created resizeable (`:393-415`). | SOURCE |

### 6.3 What the hub's compositor gives a viewer (driftwm, SOURCE only)

Pinned commit `352333a8fa1b22171492d4b71a54102045c9a19d`, `src/render/lifecycle.rs` (read 2026-10-05): in `post_render` every window gets Wayland frame callbacks (`window.send_frame`); a window whose rectangle overlaps the visible part of the canvas gets them at the render rate, and **a window completely off the visible canvas rectangle gets one at most every 995 ms** (`FRAME_CALLBACK_THROTTLE`, `:20`; `:249-275`), and a timer in `src/main.rs:381-390` repeats this once a second when nothing is rendering. The comment says why: zero callbacks starve a client's buffer cycle and disconnect it. Visibility is decided from the **camera rectangle only**; I found nothing in this file that considers a window hidden behind another window (negative search of the one file). Whether driftwm sends the Wayland `xdg_toplevel` "suspended" state (xdg-shell version 6) is UNKNOWN (the files I fetched do not show it). The word `suspended` in `docs/driftwm-findings.md` is driftwm's own stand-in-window feature, not that state.

BELIEVED consequences (nothing here was run):
1. A viewer that asks for the next RFB picture only after its `wl_surface.frame` callback arrived would get about 1 picture a second for windows off screen and the screen's rate for on-screen windows, with no message from `hubd` at all.
2. `hubd` already reads each window's position, size, focus and `suspended` flag through the driftwm socket (`docs/driftwm-findings.md` line 241, TESTED there), so it can compute any richer state (small, far, background) itself.
3. Pointer, keyboard and clipboard traffic is separate and unaffected by a paused picture (BELIEVED; with pauses by `SIGSTOP` they would be affected, see 8.1).

---

## 7. An own viewer: what it would add and what it would cost (BELIEVED throughout; propose, do not build)

`CLAUDE.md`: a custom compositor or file manager "may be built only after a design discussion with the owner"; a viewer is not named there, but the same care applies. **I have built nothing of this and I am not proposing to start.** The size estimates are my reasoning, not measurements.

What an own Wayland viewer (a small program, one window per node) could do that the stock viewers cannot (BELIEVED, each one follows from sections 3 and 5):

| Added ability | Why stock viewers cannot | Worth |
|---|---|---|
| Ask for the next picture only after the window's frame callback; slow automatically when driftwm marks the window off screen | wlvncc/TigerVNC/gtk-vnc ask at once; none uses frame callbacks | Biggest: free "far window = 1 per second" |
| Pause on command from `hubd` (a small local socket), resume with one non-incremental request | none has a control channel | High |
| Choose the size policy per window: never resize the node, scale only, or resize only for windows marked "thumbnail" | TigerVNC resizes by default and cannot be told per window; wlvncc never resizes | Medium (needs the owner's answer to Q1) |
| Lower JPEG quality for far windows (re-send `SetEncodings`, neatvnc reads it again, SOURCE 3.1) | none does at run time (BELIEVED) | Low to medium |
| Never use client Fence; avoid the crash in 5.6 | TigerVNC is BELIEVED to send only replies (5.8), so not an issue today | None now |
| Take over from wlvncc's missing clipboard, reconnect and title control (earlier report 6) | wlvncc has no clipboard and no reconnect | Possibly the stronger reason; outside this document |

Ways to get there, cheapest first (BELIEVED costs, in work I would expect, not in time promised):

| Option | What it is | Cost (BELIEVED) | Risk |
|---|---|---|---|
| **A. No viewer change** | Use wayvnc `--max-fps` per node at start, `-RemoteResize=0` on TigerVNC, and `hubd` pausing with `SIGSTOP`/`SIGCONT` (section 8.1). | Days; `hubd` code only | See 8.1 |
| **B. Patch wlvncc** | Add (1) next request after frame callback, (2) a pause/resume signal (e.g. `SIGUSR1`), using its existing libvncclient copy. | About a week for someone who knows the code; patches carried by us; its licence mix is ISC plus GPL-2-or-later in practice (earlier report 6) | Single-maintainer; "work in progress" per its README |
| **C. New viewer in C on libvncclient** | Wayland client with viewport, input, clipboard, the above. | Weeks; decoders come with the library | We own a viewer |
| **D. New viewer in Go** | Own RFB client (the framing in `rfbprobe.py` is about 300 lines; decoders for zrle and tight are the real work) and a Wayland front end. | Weeks to months; Go Wayland client libraries exist but I did not check their maturity (UNKNOWN); GPU upload and decode speed unknown | Largest |

neatvnc is a **server** library only (its public header `include/neatvnc.h` offers `nvnc_new`, display, client and damage calls; wlvncc depends on `aml` only, `meson.build`), so an own viewer cannot be built "on neatvnc". It would use libvncclient (as wlvncc does) or its own code. (SOURCE: the two files named; the conclusion is mine.)

---

## 8. Proposal: states and policy

This is a suggestion to react to, not a decision. Rates and thresholds are placeholders for the owner to set.

### 8.1 What can be done today without an own viewer

| Goal | How | What it needs | Known problems |
|---|---|---|---|
| Pause a window nobody can see | `hubd` sends `SIGSTOP` to that window's viewer process when driftwm's `state` says the window is wholly off screen (or hidden), and `SIGCONT` when it returns | `hubd` already starts the viewer and records its process (HUB-OS.md "record of what was started"); the earlier report TESTED the freeze for all viewers: bytes 0, data flows again after `SIGCONT` | The viewer also stops handling **input**, **clipboard** and its own window (it cannot redraw or answer Wayland pings); driftwm's reaction to a stopped client is UNKNOWN; neatvnc's send buffer fills (TCP back-pressure) and wayvnc kept using about 50 % of a core against 65 to 82 % while running (earlier report, 1080p). While stopped, a dead node is not noticed by the viewer (`hubd`'s own health checks still run). |
| Slow far windows | wayvnc `--max-fps 10` etc. for the whole node | The node's start script (the image recipe) | Applies to every viewer of that node, and to the focused window too; no change while running (SOURCE) |
| Stop the node resizing | `-RemoteResize=0` for TigerVNC; wlvncc never resizes; remote-viewer: earlier report | `viewers.toml` arguments | None known |
| Free the node completely | Close the viewer (disconnect) for windows that have been off screen for a long time | `hubd` closes and re-opens the window | Brings the window back with a reconnect delay; the brief says closing a window leaves the node's session alive (fine) but "Never open duplicates"; the window's canvas place must be kept |

### 8.2 Suggested states for the policy (needs an own viewer, or option B, to be exact)

| State (decided by `hubd` from driftwm `state`) | Picture rate | Size | Quality |
|---|---|---|---|
| **Focused** | the node's maximum (30) | full | full |
| **Visible, not focused** | 10 per second | full | full |
| **Visible but small** (area below a threshold the owner picks) | 10 per second | full; **a smaller node screen only if the owner agrees** (Q1) | lower |
| **Far or off screen** | 1 per second (driftwm's own heartbeat gives this, 6.3) or paused with one refresh every 10 s | unchanged | lowest |
| **Hidden / no window** | no requests; viewer disconnected after a long time | unchanged | none |

Expected effect, from section 5.1 (TESTED, scroll scene, per window): 30 to 10 to 1 to 0 pictures per second cost 340 to 114 to 12 to 0 kB/s on the wire. With 20 windows and one focused, 4 visible and 15 far (my own illustrative split), the wire would carry about 340 + 4 x 114 + 15 x 12 = 976 kB/s instead of 20 x 340 = 6800 kB/s: **about one seventh** (BELIEVED arithmetic on TESTED numbers for the heaviest scene; real desktops change far less). It would change nothing on the nodes' capture cost (finding 3) unless `--max-fps` is also lowered.

---

## 9. The exact commands (TESTED)

Everything is under a private folder `BENCH_TMP`; nothing is installed on the machine. Run from the repository root of this branch. `ulimit -c 0` is set in every script, so no core files are made.

```
# 1. harness (branch bench-remote-display-clean): unpack the .deb files with dpkg -x and build wayvnc v0.10.2, neatvnc v1.0.3, aml, wlvncc
git archive origin/bench-remote-display-clean tools/bench/remote-display | tar -x -C /tmp/lodw/h
#    I edited the copy's setup.sh to shorten DEBS= to: sway foot grim fonts-dejavu-core wlr-randr libwayland-dev libpixman-1-dev libdrm-dev
#    libxkbcommon-dev libjansson-dev libgnutls28-dev libturbojpeg0-dev zlib1g-dev libwayland-bin wayland-protocols libegl-dev libgbm-dev
#    libgles-dev libavcodec-dev libavutil-dev libswscale-dev libavfilter-dev libvncserver-dev  (nothing else needed for the node tests)
BENCH_TMP=/tmp/lodw/bench /tmp/lodw/h/tools/bench/remote-display/setup.sh all
# 2. extra programs for the viewer test only (apt-get --print-uris with the same private apt folder, curl, dpkg -x):
#    tigervnc-viewer xwayland x11-xkb-utils
# 3. the experiments of sections 5.1 to 5.7
BENCH_TMP=/tmp/lodw/bench tools/bench/remote-display/lod/run-lod.sh              # all; or: run-lod.sh pacing cu resize fence enc maxfps resume
BENCH_TMP=/tmp/lodw/bench tools/bench/remote-display/lod/disconnect.sh           # 5.5
BENCH_TMP=/tmp/lodw/bench tools/bench/remote-display/lod/fence-trials.sh 8       # 5.6 crash count
# 4. the viewer test (5.8); TCPMETER is metrics/tcpmeter.py from the clean branch
VIEWER=tiger  BENCH_TMP=/tmp/lodw/bench TCPMETER=/tmp/lodw/h/tools/bench/remote-display/metrics/tcpmeter.py tools/bench/remote-display/lod/viewer-test.sh
VIEWER=wlvncc BENCH_TMP=/tmp/lodw/bench TCPMETER=/tmp/lodw/h/tools/bench/remote-display/metrics/tcpmeter.py tools/bench/remote-display/lod/viewer-test.sh
# one probe by hand, node already running on port 5901:
python3 tools/bench/remote-display/lod/rfbprobe.py --port 5901 --phases 'normal:6,paced@0.1:6,paced@1:6,none:6,normal:6' --pid wayvnc=<pid> --pid sway=<pid>
```

`rfbprobe.py` phase names: `normal`, `paced@SECONDS`, `none`, `cu`, `cuoff`, `curegion@X+Y+W+H`, `resize@WxH`, `fence` (each `NAME:SECONDS`, comma separated); options `--enc zrle|raw`, `--fence-flags N`, `--pid NAME=PID`, `--json FILE`.

The harness is on another branch, so the tools in `lod/` are not runnable from `origin/main` alone; they only need `BENCH_TMP` to hold the unpacked programs.

---

## 10. Questions for the owner

1. **May "smaller" mean a really smaller screen on the node?** It makes programs on the node re-arrange their windows (5.3) and conflicts with "the screen size is fixed per node in v1". If no: the only savings are pause and slower, plus `--max-fps`; the TigerVNC viewer then needs `-RemoteResize=0`.
2. **Which window states and rates** do you want (the table in 8.2 is only a starting point), and is "far window = 1 picture a second" acceptable?
3. **Which first step?** Option A in 8.1 (days, no viewer change, with the `SIGSTOP` side effects), option B (patch wlvncc), or a **design discussion about an own viewer** (section 7, as `CLAUDE.md` requires)? I recommend A first to find out how much matters, but this is your choice.
4. **Release or debug build of wayvnc and neatvnc in the image?** It decides whether the Fence problem (5.6) aborts the server or does something unknown (3.2).
5. **Should the Fence problem be reported to the neatvnc author?** I have not contacted anyone.
6. **Do the real nodes' wayvnc capture a virtual (headless) output or a real monitor?** wayvnc only resizes virtual outputs (SOURCE); for a real monitor the "small" saving is impossible.
7. Do you want me to **measure the hub side** (decoding cost per window), which needs the real GPU in December and an own or patched viewer to pause against?

---

## 11. What I did not verify (plain list)

- Anything on real hardware, over a real network, with a GPU, with H.264, or with driftwm. All driftwm statements are source reading.
- The hub-side cost (decoding, GPU upload) and whether pausing saves it.
- Whether a stopped (`SIGSTOP`) viewer upsets driftwm; whether driftwm sends `xdg_toplevel` "suspended".
- A release (no assertions) build of wayvnc/neatvnc; how long a reconnect takes; JPEG quality as a lever.
- remote-viewer (SPICE and VNC), FreeRDP and Weston were read, not run, in this part. The server side of SPICE's stream report was not read.
- Raw encoding stall cause; damage inside and outside a continuous-update region at once.
- Run-to-run noise (N = 1 for all tables except the Fence trials, N = 8).
- Whether the Ubuntu package of spice-gtk is 0.42.

## 12. Sources read (all 2026-10-05; raw files)

- neatvnc v1.0.3 `783437adc56c2d7ce2cac1ec584acad798594267` (2026-10-03): `src/server.c`, `src/display.c`, `meson.build`, `include/neatvnc.h`. wayvnc v0.10.2 `bb837459c75fd3c267d48af170ac7ed2566f67e5` (2026-09-25): `src/main.c`, `src/output-management.c`, `src/screencopy.c`, `src/ctl-commands.c`, `meson.build`.
- wlvncc `cc0abf87c37920540f2439a556e6a480c28f8f46` (2026-04-29): `src/main.c`, `src/vnc.c`, `src/rfbproto.c`, `meson.build`, `README.md`.
- rfbproto master, `rfbproto.rst` (2026-10-05; identical to `/tmp/lod/rfb.rst`).
- TigerVNC `v1.13.1`: `common/rfb/CConnection.cxx`, `vncviewer/CConn.cxx`, `vncviewer/DesktopWindow.cxx`, `vncviewer/Viewport.cxx`, `vncviewer/parameters.cxx`. gtk-vnc `v1.3.1`: `src/vncconnection.c`, `src/vncdisplay.c`. spice-gtk `v0.42`: `src/spice-widget.c`, `src/channel-display.c`, `src/channel-main.c`. virt-viewer `v11.0`: `src/virt-viewer-display-spice.c`.
- FreeRDP `3.32.0`: `libfreerdp/core/update.c`, `client/X11/xf_event.c`, `client/Wayland/wlfreerdp.c`, `client/Wayland/wlf_disp.c`, `channels/disp/client/disp_main.c`. Weston `13.0.0`: `libweston/backend-rdp/rdp.c`, `rdpdisp.c`, `libweston/backend-vnc/vnc.c`.
- driftwm `352333a8fa1b22171492d4b71a54102045c9a19d`: `src/render/lifecycle.rs`, `src/main.rs`.
- Earlier report: `docs/proposals/remote-display-benchmarks.md` (branch `origin/bench-remote-display-clean`), sections 5.5, 5.6, 6, 7.

