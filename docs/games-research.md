# Games and emulators: research for the gaming box

> **RESEARCH; nothing here is a decision.** Nothing was installed, built or run. Everything was read on the web on **2026-10-03** and nothing was pushed anywhere.

**Why:** `HUB-OS.md` lists the games and emulators of the gaming box (Atari to PS2, GameCube, original Xbox, Soul Calibur II netplay with the emulator still to be chosen). This file records what the projects themselves say about releases, Linux builds, netplay and latency.

## 0. How to read this file

Labels on every item: **TESTED** (I ran it here; nothing in this file is TESTED, because nothing was installed or run), **SOURCE** (read at the named place, with the date it was read), **BELIEVED** (second-hand, or I could not read the primary page), **UNKNOWN** (I did not find it; I do not guess).

Limits of how I read things, so you can judge the labels:

- **Release dates.** GitHub's release pages show dates without a year for the last twelve months, and the page-reading tool I used sometimes misread them (for mGBA it gave "09 Mar" for 0.10.5 while the feed says 2025-11-18; see section 1). So every date below is the `updated` time from the project's own **release feed** (`.../releases.atom`), written as a full date. That time is when the release entry was last updated; it can be later than the first publication (Stella 7.0: the release page says 5 Oct 2024, the feed says 2025-01-02). I give both when they differ.
- **dolphin-emu.org cannot be read by a program.** Its pages (download, netplay guide, progress reports) return HTTP 403 with a "Establishing a secure connection" page (a bot check, TESTED with `curl`). I did not try to get around it. What I say about Dolphin's *official site* therefore comes from search-engine summaries of those pages (**BELIEVED**, not word for word). What I say about Dolphin's *GitHub* pages was read directly (**SOURCE**).
- The tool that reads a page returns a summary, not the raw page. Quotes marked as quotes are as the tool returned them; I could not check them against the raw page.
- Today is 2026-10-03. "Age" below is counted from that date.

---

## 1. The emulators on the owner's list

| Emulator | Official place | Latest release (SOURCE, release feed) | Linux build and how it is distributed | Signs of being unmaintained |
|---|---|---|---|---|
| **Stella** (Atari 2600) | repo `github.com/stella-emu/stella`; site `stella-emu.github.io` | **7.0c**, feed 2025-01-02T21:14:35Z (7.0: page says 5 Oct 2024, feed 2025-01-02T21:25:12Z) | SOURCE (site, "January 3, 2025" update): a 64-bit **.deb for Ubuntu 22.04** and a **source tarball**. No Flatpak or AppImage listed on that page. | None seen. The last release is about 21 months old. Commit activity: UNKNOWN (not read). |
| **Mesen** (NES; also SNES, GB, GBA, PCE, SMS, WS) | `github.com/SourMesen/Mesen2` (**archived**); successor `github.com/nesdev-org/MesenCE` | Mesen2: **2.1.1**, 2025-07-06T07:32:33Z. MesenCE: **2.2.1**, 2026-06-05T10:18:39Z; 2.2.0, 2026-06-04T10:36:12Z | Mesen2 README (SOURCE): native Linux builds (x64, ARM64), AppImage (both), build from source. MesenCE README: Linux x64 and ARM64 development builds that need SDL2; how MesenCE ships Linux releases: **UNKNOWN**. | **Mesen2 is archived** (the page says "archived on June 4, 2026" and points to MesenCE). MesenCE's notes: "Mesen is becoming MesenCE, a Community Edition maintained by Sour and various community members". Licence GPL-3. |
| **Nestopia** (NES) | The core is at `gitlab.com/jgemu/nestopia`; `github.com/0ldsk00l/nestopia` is **Nestopia UE**, the fork with a GUI | Nestopia UE: **2.0.0**, 2026-09-13T19:58:00Z; 1.99.0, 2026-09-13T19:55:47Z; 1.53.2, 2025-08-09T22:14:49Z | The UE README (SOURCE) says this repository is for the "legacy win32 release"; Linux builds come from the "Jolly Good Emulation" project, builds at `github.com/jgemu/jgbuilds`. What those builds are (AppImage, Flatpak): **UNKNOWN**. | The UE README: "This project no longer maintains the core emulator. Please submit issues about core emulation upstream". Releases still appear (2026-09-13). Which of the two to call "official": your choice; I read both. |
| **bsnes** (SNES) | `github.com/bsnes-emu/bsnes` | tagged **v115**, 2020-03-20T13:23:38Z (about 6.5 years old); a rolling **"bsnes nightly 2026-05-23"**, 2026-05-23T06:55:53Z | README (SOURCE): nightly builds for Windows, macOS, **Linux** and FreeBSD. The file names and how they are packaged: **UNKNOWN** (the page failed to list assets). | Not unmaintained by what I read: commits on 2026-09-27 ("fix undefined behavior in RLE encode/decode size header handling"). The last *tagged* release is old; the project ships nightlies. |
| **BlastEm** (Mega Drive) | `retrodev.com/blastem` | Nightly `blastem64-1.0.1-pre-9b71c8bd2065.tar.gz`, dated **2026-Sep-26** on the nightlies page. The latest *stable* version and date: **UNKNOWN** (not on the page I got). | SOURCE (nightlies page): **Linux 64-bit and 32-bit tar.gz**, built "every night whenever there are changes in the main source repository". | None seen (a build four days before this read). |
| **Genesis Plus GX** (Mega Drive and more) | `github.com/ekeeke/Genesis-Plus-GX` (also on Bitbucket per the README) | **No GitHub release or tag** (the tags page says "There aren't any releases here"). Last commits **2026-09-23** (CD core fixes). | README (SOURCE): used through RetroArch (libretro), BizHawk and OpenEmu; builds for GameCube/Wii and RetroArch are in a "builds" directory. A standalone Linux program: **UNKNOWN** (the README lists ports, not a Linux build). | None seen (commits nine days before this read). |
| **SameBoy** (Game Boy) | `github.com/LIJI32/SameBoy`; site `sameboy.github.io` | **v1.0.3**, 2026-03-04T21:42:06Z (v1.0.2 2025-08-03T14:04:11Z); also libretro builds (v1.0.3-libretro) | README (SOURCE): build from source (`make`, target `sdl`; `sudo make install` for "Linux, BSD, and other FreeDesktop users"). A prebuilt Linux binary: **UNKNOWN**. Licence: Expat (MIT). | None seen. |
| **mGBA** (Game Boy Advance) | `github.com/mgba-emu/mgba`; site `mgba.io` | **0.10.5**, feed 2025-11-18T12:29:22Z. The release page text showed "09 Mar" without a year; **I could not settle which is right**. 0.10.4: 2024-12-08T05:18:53Z. | SOURCE (releases page and site): **AppImage** (x64, arm64) and **Ubuntu tarballs** (bionic, focal, jammy, noble); development builds for Ubuntu and others. Flatpak is not mentioned. Licence MPL-2.0. | None seen. Last release 10 to 19 months old, depending on which date is right. |
| **Mupen64Plus-Next** (N64, libretro core) | `github.com/libretro/mupen64plus-libretro-nx`; docs `docs.libretro.com/library/mupen64plus/` | **No versioned releases**: the feed holds one 2019-04-26 entry (`mupen_next_old_gliden`). Updates reach users as a RetroArch core download. Date of the latest build: **UNKNOWN**. | SOURCE (libretro docs): "Downloaded as a RetroArch core"; multiple platforms. | Commit activity: **UNKNOWN** (not read). |
| **simple64** (N64) | `github.com/simple64/simple64` | **v2024.12.1**, 2024-12-28T08:49:15Z | README (SOURCE): Linux through **Flathub**; Windows through GitHub releases. | **Archived.** README: "I (loganmc10) have decided to focus my future efforts on a new N64 emulator. This project has been archived and development has stopped." It recommends Gopher64 or RMG. (I did not research those two.) |
| **DuckStation** (PlayStation 1) | `github.com/stenzek/duckstation`; site `duckstation.org` | rolling "Latest Rolling Release" 2026-10-03T03:11:34Z; "Latest Preview Build" 2026-10-02T10:46:46Z; tag `v0.1-11894` 2026-10-03T03:08:02Z | README (SOURCE): "DuckStation is provided for x86_64/ARM32/ARM64 Linux in AppImage formats." | None. **Licence: CC-BY-NC-ND** (README: "you may redistribute **unmodified** versions of this repository, and the compiled binaries"). That is not an open-source licence (BELIEVED; I did not read the licence text) and it forbids changed copies: relevant if the image ever needs a patch. |
| **PCSX2** (PlayStation 2) | `github.com/PCSX2/pcsx2`; site `pcsx2.net` | **v2.9.94**, 2026-10-01T16:23:32Z; v2.9.93 2026-09-29; v2.9.92 2026-09-27; v2.9.91 2026-09-26 | README (SOURCE) shows a "Linux Build Status" badge, so Linux is built. How Linux is distributed (AppImage, Flatpak): **UNKNOWN** (the downloads page returned nothing I could read). Licence GPL-3.0. | None (a release two days before this read). |
| **Dolphin** (GameCube, Wii) | `github.com/dolphin-emu/dolphin`; site `dolphin-emu.org` | **2609**, 2026-09-24T03:07:54Z; 2606a 2026-08-11; 2606 2026-06-25; 2603a 2026-03-17 | BELIEVED (third-party reports of Dolphin's own announcement; the official page is unreadable, see section 0): an **official Flatpak** (x86_64 and aarch64) from the Dolphin site since release 2412, and an unofficial one on Flathub; before that there were no official Linux builds. | None. |
| **xemu** (original Xbox) | `github.com/xemu-project/xemu`; site `xemu.app` | rolling "Latest Development Build" (`pre-release`) 2026-09-29T06:27:20Z; **v0.8.136**, 2026-06-08T05:54:34Z; v0.8.135 2026-05-20; v0.8.134 2026-02-26 | SOURCE (`xemu.app/docs/download`): **AppImage** (x86_64, aarch64), **Flatpak** on Flathub (`app.xemu.xemu`), an Ubuntu 22.04+ **daily-build PPA**, build from source. | None seen. |

**Plain summary of section 1.** Two projects on your list are **archived**: Mesen2 (continued as MesenCE) and simple64. One is a **fork that says it no longer maintains the core**: Nestopia UE (the core lives at jgemu). Mupen64Plus-Next has no versioned releases at all (it is a RetroArch core). DuckStation's licence forbids redistributing changed copies. For Genesis Plus GX, SameBoy and bsnes I did not find a ready Linux program on the pages I read (source or nightlies only); that is **UNKNOWN**, not "absent".

---

## 2. Dolphin netplay: what the modes really are

Sources: the official **Netplay Guide** (`dolphin-emu.org/docs/guides/netplay-guide/`; unreadable by program, so **BELIEVED**, from a search summary of it), and Dolphin's GitHub source `Source/Core/Core/NetPlayServer.cpp` (**SOURCE**, read 2026-10-03, master branch).

| Mode | What the official guide says (BELIEVED, not verbatim) | What the code shows (SOURCE) |
|---|---|---|
| **Buffer** | The buffer is "the amount of latency added to inputs". A rule of thumb: about one pad buffer per 15 ms of latency per client. | `AdjustPadBufferSize(size)` sets `m_target_buffer_size = size` and sends a `PadBuffer` message to clients (line 754). |
| **Fair Input Delay** (the default) | The buffer is split between all players, so every player has the same input delay; best when everyone acts at once. | not read |
| **Host Input Authority** | The host has zero latency and each client is only responsible for its own buffer to the host; the clients' base latency is doubled, but with four players over long distances the total can be lower. | `SetHostInputAuthority(enable)` stores `m_host_input_authority` and tells the clients (line 778). |
| **Golf Mode** | Host Input Authority, plus the ability to swap which player has zero latency; meant for games where only one player acts at a time. "Show Golf Mode Overlay" shows a window to swap the player. Not compatible with Wii Remote netplay. | A golfer change is only allowed when `m_host_input_authority && m_settings.golf_mode`, with `m_pending_golfer` and `m_current_golfer` tracked (line 1238). |

**Lockstep.** The guide's wording and the code agree with the plain meaning: in the default mode every machine runs the same inputs on the same frame and waits for the buffer, so a late packet stops the frame. The part of the code that decides when a frame advances was **not** in the part of the file I could read, so I cannot show the lockstep loop itself (**UNKNOWN** from source; BELIEVED from the search summaries, which call Dolphin's netplay "delay-based" and say it "always syncs the emulator's internal state among everyone").

**Does any official Dolphin release have rollback?** I found **none**. What exists:

- Pull request **#12914 "WIP: Netplay: Rollback Implementation"** by WhiteTPoison, state **Draft**, created 2024-07-04, last activity shown 2024-08-20 (SOURCE, page read 2026-10-03). It describes an "incremental rollback" with savestates that store only the changes from the previous frame, and says memory protection is a problem. A contributor, JMC47, replied: "I do think things like resyncing in general using savestates (for instance, if there's a desync) would be valuable too, outside of simply implement rollback. Either way, it's always nice to see work on netplay. I wish you luck on getting this into working order!"
- **Slippi** is a *fork* of Dolphin with rollback, for Super Smash Bros. Melee (BELIEVED: the Slippi site and third-party pages; `slippi.gg/netplay` could not be read by the tool).
- The latest Dolphin release feed (2609, 2606a, 2606, 2603a) has no entry I could read that mentions rollback. I did not read the release notes of each one, so **"no official release has rollback" is BELIEVED, not shown**.

---

## 3. Slippi on Linux

**Releases.** Slippi Launcher: **2.15.1**, 2026-08-31T05:16:02Z; 2.15.0 2026-06-04; 2.14.2 2026-05-23 (SOURCE, `github.com/project-slippi/slippi-launcher/releases.atom`).

**Install route.** The site `slippi.gg` and the launcher README gave the tool nothing about Linux, so the **current official route is UNKNOWN from primary sources**. What I found:

- A third-party guide dated **2022-03-16** (Linux Gaming Central): download "an AppImage of the Slippi Launcher right on the project's home page", `chmod +x Slippi-Launcher-<version>-x86_64.AppImage`; "Slippi is not available as a Flatpak at the time of writing this" (BELIEVED, old).
- Launcher issue **#520 "Package as a flatpak"**, open, opened 2026-01-16 (SOURCE): so there was still no official Flatpak at that date, as far as an open request shows.

**Known Linux problems (SOURCE, `github.com/project-slippi/slippi-launcher/issues`, read 2026-10-03; titles as listed, none of these was reproduced):**

| Issue | Title | Opened | State |
|---|---|---|---|
| #614 | "Replays don't work on Linux" | 2026-06-13 | open |
| #520 | "Package as a flatpak" | 2026-01-16 | open |
| #459 | "Pressing 'Play' results in «Failed to load module "canberra-gtk-module"» on Fedora Silverblue 42" | 2025-07-22 | open |
| #405 | "Dolphin fails to launch game on Wayland [Upstream Dolphin]" | 2023-11-03 | open |
| #387 | "AppImage on Fedora 38 - SyntaxError: Unexpected token ( in JSON at position 0" | 2023-08-11 | open |
| #646 | "The AppImage version on Linux causes blackscreen after a while of playing" | 2026-09-11 | **closed**; the report: black screen after about 4 to 5 matches on a Steam Machine running SteamOS, audio continues slowed; the page showed no maintainer reply |

A related issue in the Ishiiruka fork (`project-slippi/Ishiiruka`, #323): "AppImage fails to open in Fedora 35 (or any glib > 2.68) due to gmodule symbol lookup error" (title only, via search).

**Frame drops.** No open launcher issue is about frame drops (SOURCE, the tool's reading of the issue list). That does **not** settle the report in `HUB-OS.md` "Unverified" (one user's report of frame drops on Linux): it stays unverified, because no one measured it here (**UNKNOWN**).

---

## 4. "Ring Out", the Soulcalibur II recompilation

Read: the repository `github.com/jackpoison-prog/RingOut`, its README (raw), its release feed and its issue list, on 2026-10-03. **Nothing was built or run (no TESTED).** Whether the claims below are true is **not checked**.

| Question | What the repository says (SOURCE) |
|---|---|
| What it is | README: "A native PC port of a GameCube fighting game (disc ID `GRSEAF`), produced by static recompilation rather than emulation: the disc's PowerPC executable is translated ahead of time into C, compiled for x86-64, and run as native code". The repository description adds: "run as native x86-64 on a Dolphin-derived runtime". The README text I received does not use the words "Soulcalibur II"; the release notes mention "the US, European and SC2 Plus discs", which fits Soulcalibur II (BELIEVED, not stated outright in what I read). |
| Licence | **GPL-2.0-or-later** (README). I did not open the licence file. |
| Game data | README: "No game data and no game code are distributed here." You supply your own disc image. |
| Platforms | Linux desktop x86-64 (`RingOut-<version>-linux-x86_64.zip`, needs `cmake`, `ninja`, `python3`, `clang` or `gcc`, a working Vulkan driver, about 1.5 GB free: the module is built on your machine); Steam Deck/SteamOS (`...-steamdeck-x86_64.zip`, also builds a module); Windows (installer or zip, nothing to compile, bundled). |
| Install on Linux | Download the release zip, then run `./RingOut` or `./setup.sh /path/to/disc.iso`. |
| Netplay | Yes. README: "working. Delay-based (not rollback) over a deterministic dual-core setup, with a lobby showing live ping and per-player game status; two peers stayed byte-identical over 6,470 frames." It uses "a direct connection (LAN, VPN or a forwarded port)"; "It is delay-based, not rollback: both peers run the same inputs on the same frame." "The host sets the input buffer — 5 frames by default, about 83 ms at 60 fps, adjustable from 1 to 20 in the lobby." "A peer holding a different disc, a differently built module or modified game data is refused at connect and told which." |
| Releases | **1.6.5**, 2026-09-27T23:26:50Z ("the European disc gets the built-in movie player, an occasional crash at launch is gone, and the US, European and SC2 Plus discs are 1.5-2.5% faster again"); 1.6.4 2026-09-27T20:19:43Z; 1.6.3 2026-09-22; 1.6.2 2026-09-17; 1.6.1 2026-09-13. Five releases in about two weeks. |
| Age and health | The oldest issue shown is #1 "steam deck", 2026-08-25, so the repository is about **five weeks old** (BELIEVED from that date). 14 open issues, including #16 "Linux won't launch" (2026-10-02), #13 "Linux Launcher Bug" (2026-09-25), #12 "Questionable performance" (2026-09-12) and #2 "error code on netplay" (2026-08-26). How many people maintain it, and who they are: **UNKNOWN**. |

**What is not known and matters for the gaming box:** whether the recompiled game's netplay survives a real network (the README's test is two peers and 6,470 frames, no network described); how its "Dolphin-derived runtime" is licensed in detail; whether the Linux build runs in this project's image (it builds a module at first start, so the image needs a C compiler or a prebuilt module: **UNKNOWN**); whether it is endorsed by the game's rights holder (**UNKNOWN**, not stated).

---

## 5. Latency features, in the maintainers' own words

| Project | Feature | What the maintainers say | Label |
|---|---|---|---|
| **RetroArch (libretro)** | **Run-Ahead** | "calculates the frames as fast as possible in the background to 'rollback' the action as close as possible to the input command requested". *Single instance:* "Disable audio and video, run a frame, Save State. Run additional frames with audio and video disabled if we want to run ahead more than one frame. Enable audio and video and run the frame we want to see. Load State." *Two instances:* "Primary core does Audio only, then saves state. Secondary core loads state, runs frames ahead discarding audio and video, then runs a frame with video only." "Run Ahead relies on save states so they need to be clean and fast enough. If a core doesn't support them, this can not work." "The higher the number of frames you are going to run ahead of emulation, the higher demands it places on your CPU." "Many of the cores do not leave audio emulation in a clean state after loading state, so you would get buzzing. Using Two-Instance mode makes the primary core not do any load states and avoids that." (`docs.libretro.com/guides/runahead/`) | SOURCE |
| | **Frame Delay** | Lets the user "postpone by a certain number of milliseconds, until the last moment that is realistically viable" the creation of the new frame and the input poll, which "decrease[s] the number of milliseconds that the new frame ... has to remain on hold", for "a slight – albeit measurable – improvement to the perceived input latency". Without the automatic setting, tuning it by hand risks "framerate drops, stutter, audio crackling and all sorts of other issues". **Automatic Frame Delay:** if Frame Delay is 0 the start point is half the frame period (8 ms at 60 Hz), otherwise the value set by hand, and it scales down by itself if the frame rate drops. (`libretro.com/index.php/retroarch-1-9-13-automatic-frame-delay/`) | SOURCE |
| | Hard GPU Sync ("Synchronization Fences"), max swapchain images | Named as "configurable latency mitigation tools" and the swapchain setting is described as "Maximum amount of configurable swap chains. Can be set from 1 to 3 depending on your video driver, your GPU, and the video context driver"; the pages I got give no further explanation of Hard GPU Sync. (`retroarch.com/?page=latency` claims "a next-frame response time (≤16ms!) achievable with RetroArch! This means zero frames of input lag is achievable." That is a claim, not measured here.) | SOURCE |
| | **Netplay** | "Netplay in RetroArch works by expecting input to come delayed from the network, then rewinding and re-playing with the delayed input to get a consistent state." It needs a deterministic core, only gamepad/analog input, and identical core and content on both sides; "Cores are expected to support serialization for proper netplay behavior". The docs add a footnote that the guarantee is "not actually a guarantee". (`docs.libretro.com/development/retroarch/netplay/`) | SOURCE |
| **Mupen64Plus-Next** | netplay | The libretro docs table marks Netplay "✕" (not supported). | SOURCE |
| **Dolphin** | frame pacing, "Rush Frame Presentation", "Smooth Frame Presentation" | From search summaries of the official progress report for release 2512 (the page is unreadable): "Rush Frame Presentation" throttles around presenting the frame as soon as it can after input is read, which "in theory reduces the time between click and photon"; with "Immediately Present XFB" it can give "a 10ms reduction in latency" in ideal cases; "Smooth Frame Presentation" delays presentation by roughly 1 to 2 ms to output frames more consistently, using previous frame times. | BELIEVED |
| | `CoreTiming: Improve frame pacing` (PR #13387, **merged 2025-03-13**, by jordan-woyak) | "Throttling is now done just before user input, statistics counting, and presentation." Presentation timing is computed on the CPU thread and sent to the GPU thread, since the "GPU-thread cannot sanely use CoreTiming". The author's numbers: frame-time variance from about "±0.22ms" to "±0.02ms". | SOURCE |
| | Vulkan closed-loop latency control (PR #12035, by nyanpasu64) | State **Draft**: when vsync is on, a thread uses `vkWaitForPresentKHR` and the emulation speed is adjusted so each frame is submitted about 4 ms before presentation; limit: "VK_KHR_present_wait lacks Windows AMD driver support". **Conflict I could not settle:** the search summary of the 2512 report describes the same mechanism as shipped, while the pull request page showed Draft. | SOURCE (PR); BELIEVED (report) |
| **PCSX2** | Frame Pacing / Latency Control settings | From search results quoting PCSX2's own settings text and pull requests (second-hand): "Optimal Frame Pacing" (off by default) "can reduce input lag at the cost of measurably higher CPU and GPU requirements"; "Sync to Host Refresh Rate" (off by default; variable-refresh users "should disable this option"); "Maximum Frame Latency" (default 2 frames): "Higher values can assist with smoothing out irregular frame times, but increase input lag"; "Skip Presenting Duplicate Frames" was moved to Emulation Settings, Frame Pacing/Latency Control (PR #12460). | BELIEVED |
| **DuckStation** | run-ahead, rewind | README: "Save state support, with runahead and rewind." No other latency feature appears in the README text I got. | SOURCE |
| **mGBA** | rewind, frame skip | README: "Rewind by holding Backquote"; "Frameskip, configurable up to 10". No run-ahead or latency feature is mentioned in the README text I got. Link cable: local only, "Networked multiplayer link cable support" is listed as planned. | SOURCE |
| **Mesen2/MesenCE, bsnes, SameBoy, Stella, xemu, BlastEm, Genesis Plus GX** | | I did not find a maintainer statement about run-ahead, frame delay or frame pacing in the pages I read. (Through RetroArch, Genesis Plus GX and Mupen64Plus-Next get RetroArch's Run-Ahead and Frame Delay; whether a given core's save states are clean enough is not something I checked.) | UNKNOWN |

**What none of this shows:** how much latency any of these saves on the gaming box's hardware. `HUB-OS.md` already says true click-to-screen delay needs external measuring hardware, and the build environment has no GPU. Every number above is a maintainer's claim.

---

## 6. Things I could not do, and questions

- Dolphin's official pages could not be read (bot check). If exact wording of the netplay guide or the progress reports matters, someone with a browser needs to copy the text.
- Not researched: Gopher64 and RMG (simple64's recommended replacements), the jgemu Nestopia core, Atari 5200/7800 and other consoles "from Atari through PS2", Minecraft, Diablo II: Resurrected.
- The Linux packaging column has many UNKNOWN cells (bsnes, SameBoy, Genesis Plus GX, PCSX2, MesenCE). Filling them needs the download pages read in a browser or an actual download, which this task did not do.
