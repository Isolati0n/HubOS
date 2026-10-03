# Games and emulators, second round: N64, MesenCE, build and run needs, Steam, Flatpak

> **RESEARCH; nothing here is a decision.** Nothing was installed or run except package-index queries. Everything was read on the web or in this repository on **2026-10-03**. Follows `docs/games-research.md`, whose "how to read this file" section applies here too.

Labels: **TESTED** (I ran it here; the command is shown), **SOURCE** (read at the named place on that date), **BELIEVED** (second-hand, or the primary page could not be read), **UNKNOWN** (not found; I do not guess).

Limits, so the labels can be judged:

- Dates are the `updated` time of each project's release feed (`.../releases.atom`), which can be later than the first publication.
- The page-reading tool returns a summary, not the raw page. Text in quotation marks is as it returned it.
- `dolphin-emu.org`, `gitlab.steamos.cloud` and some other sites block programs (bot checks), so Valve's own container-runtime document could not be read directly; I used the copy of its known-issues file on GitHub instead.
- **TESTED items about Ubuntu** come from the package index of this build machine (Ubuntu 24.04.4 LTS, components main, universe, restricted, multiverse, index fetched 2026-10-03): `apt-cache policy PACKAGE` and `apt-cache search --names-only WORD`, with the index kept in a temporary directory. "In the archive" below means that index; it says nothing about PPAs or other archives.

---

## 1. N64: Gopher64 and RMG

| | **Gopher64** | **RMG** (Rosalie's Mupen GUI) |
|---|---|---|
| Official repository | `github.com/gopher64/gopher64` | `github.com/Rosalie241/RMG` |
| Latest release (SOURCE, feed) | **v1.1.36**, 2026-08-21T11:32:55Z; v1.1.34 2026-07-09; v1.1.32 2026-06-25; v1.1.30 2026-06-21. Notes of 1.1.36: "Add GPU hints on Windows to help select dedicated GPU", updated Rust/SDL, netplay authentication, rumble and VRU fixes. | **v0.9.0**, 2026-06-14T18:03:07Z; v0.8.9 2026-01-23; v0.8.8 2025-11-16; v0.8.7 2025-11-06. Notes of 0.9.0: fixes for zlib 1.3.1 build issues, audio fixes for the AppImage, a screenshot crash on Linux, "dispatcher server support removed". |
| Licence (SOURCE, README) | **GPLv3**, "with adaptations from mupen64plus and ares projects" | **GPL-3.0** ("GNU General Public License v3.0") |
| What it is | "a cross-platform N64 emulator" written in **Rust**; README features: netplay, homebrew support, upscaling, CRT shaders, CPU overclocking emulation, cheats, savestates, RetroAchievements. Platforms: Windows, macOS, Linux, Android. | "a free and open-source mupen64plus front-end written in C++" with a Qt user interface; platforms: Windows and Linux. |
| Linux builds and how they ship (SOURCE, READMEs) | A **standalone executable** on the GitHub releases page (no format details read) and a **Flatpak** (`flatpak install flathub io.github.gopher64.gopher64`). | **AppImage** on GitHub Releases, **Flatpak** on Flathub, an Arch User Repository package, a Fedora COPR repository. |
| Netplay, what kind (SOURCE) | Gopher64's wiki (Netplay Guide): input delay, **no rollback**: "Gopher64 uses GGRS, rollback is not currently supported". In the lobby "a number is recommended to you ... based on your ping to the other players"; it can be changed in game (Alt+[ and Alt+], player 1 controls it); "Setting this too low will cause the game to freeze as it waits for inputs from other players." Networking is "always a full mesh P2P"; if a direct connection fails it "will automatically fall back to using a TURN server"; WebRTC with STUN/TURN, "port forwarding is not necessary". (A search summary adds that matchmaking and NAT traversal use a server, inputs go peer to peer.) | RMG's wiki describes two ways to connect: port forwarding with a separate `rmg-netplay-server` and a JSON server list, or tunnelling software (Radmin VPN). **The kind of netplay (input delay, buffer, lockstep or rollback) is not stated on that page: UNKNOWN.** |
| Maintenance signs | Five releases between 2026-06-21 and 2026-08-21; the README has contribution guidelines that stress human-written code. No sign of being unmaintained. | Releases roughly every one to four months (2025-11 to 2026-06). No explicit status statement. No sign of being unmaintained, but the cadence is slower. |
| In the Ubuntu 24.04 archive (TESTED) | **No** (`apt-cache search gopher` finds no emulator). | **No** (`apt-cache search rmg` finds none). The archive has other Mupen64Plus pieces: `mupen64plus-qt` 1.15-1build2, `mupen64plus-ui-console` 2.5.9+26+g3ad5cbb-3 and the core `libmupen64plus2`/`libmupen64plus-dev`; these are the old Mupen64Plus program, not RMG. |

Not read: Gopher64's Vulkan requirement (the README says nothing about it: **UNKNOWN**), how the "Linux standalone executable" is linked (static or against the host's libraries: **UNKNOWN**), and the Flathub pages of either program.

**Also on the owner's earlier list:** `docs/games-research.md` found that simple64 is archived and that Mupen64Plus-Next (a RetroArch core) has no netplay according to the libretro docs table. Neither is repeated here.

---

## 2. MesenCE and the jgemu Nestopia builds

### MesenCE (NES, and SNES, GB, GBA, PCE, SMS/GG, WonderSwan)

- **Repository:** `github.com/nesdev-org/MesenCE`. **Latest release:** Mesen 2.2.1, 2026-06-05T10:18:39Z; 2.2.0 2026-06-04T10:36:12Z (SOURCE, feed). Its notes: "Mesen is becoming MesenCE, a Community Edition maintained by Sour and various community members"; 2.2.0 "has about 150 changes in total, and 2.2.1 contains a hotfix for iNES ROMs".
- **Licence:** GPL V3, copyright "Sour and contributors" through 2026 (SOURCE, README).
- **How Linux builds ship:** the README names "Linux x64" and "Linux ARM64" **development builds** (nightly links). The release notes say "For the **macOS** and **Linux** builds, **SDL2** must be manually installed first." **No AppImage or Flatpak is mentioned** in the README or in the release notes I read. The file names of the release assets could not be read (the assets list failed to load): **UNKNOWN** what format the Linux release files have. (The old, archived Mesen2 repository did list AppImages; that is not MesenCE.)
- **Maintenance signs:** only two releases exist (2026-06-04 and 2026-06-05), both right after the original Mesen2 repository was archived on 2026-06-04. The community-edition model is new; its record is too short to judge. No sign of being unmaintained yet.

### The jgemu Nestopia builds

- `github.com/jgemu/jgbuilds` is described as "Standalone builds for Jolly Good emulators using The Quality Tea Frontend". The README says the builds support "Windows 10+, macOS 15+, and **Ubuntu 26.04+**". The build formats on Linux (AppImage, tarball) were **not shown: UNKNOWN**.
- Releases (SOURCE, feed): `nestopia 2.0.0` 2026-09-13T17:07:44Z, "QTea built against nestopia 2.0.0"; also `bsnes 2.1.2` (2026-09-13T16:41:46Z), `gambatte 0.6.1` (2026-09-19T15:56:38Z), and others (`cega`, `jollycv`, `geolith`). "QTea" is the Quality Tea frontend that wraps the emulator core.
- The core: `gitlab.com/jgemu/nestopia`, "a maintained fork of Nestopia", created 2020-06-09, 515 commits, 9 tagged releases, GPL-2.0-or-later, built with `make` against the **Jolly Good API** and **SDL** (SOURCE, GitLab project page).
- What matters for you: a build made for **Ubuntu 26.04 or newer** may need a newer glibc than Ubuntu 24.04 has. Whether the images will use 24.04's glibc 2.39 or something newer: this is not decided; the check that the program starts on the image's glibc is **UNKNOWN** (nothing was run).
- Ubuntu 24.04's archive has the old `nestopia` 1.52.0-1build2 and `libretro-nestopia` 1.52.0+20230102.gitcb1e24e-1 (TESTED), not 2.0.0.

---

## 3. Building each emulator from source, and what each AppImage needs

All the build systems below were read from the projects' own documents (named per row). None needs systemd to build. **Build dependencies that matter for the "no systemd" rule:** several projects list `libudev-dev`, `libdbus-1-dev` or `libinput-dev`; those link `libudev1` and `libdbus-1`, and `docs/proposals/systemd-libraries.md` found that `libdbus-1-3` links `libsystemd0` and `libinput` links `libudev1` (TESTED there). So building or running them means keeping the two tolerated libraries.

| Emulator | Build system and compilers | Main libraries | Qt / SDL / Vulkan / X server or Wayland | In Ubuntu 24.04's archive (TESTED) | Linux AppImage / other format |
|---|---|---|---|---|---|
| **Dolphin** | CMake (**3.25 or newer for Dolphin 2606-214 and newer**), a C++ compiler "with C++23 support for Dolphin 2512-116 and newer" (SOURCE, Dolphin wiki "Building for Linux"). Example: `cmake --preset ninja-release-x64 -DCMAKE_C_COMPILER=gcc-13 -DCMAKE_CXX_COMPILER=g++-13`. | The wiki's apt command installs about 50 packages (OpenGL, X11, EGL, FFmpeg, SDL3, Qt6, compression libraries). "Libraries will silently fall back to their vendored counterpart if they cannot be found on the system." | **Qt6 is optional** (`ENABLE_QT`; `qt6-base-dev`, `qt6-base-private-dev`, `qt6-svg-dev`), X11 optional; **Vulkan is not mentioned** in the documented requirements (so whether a Vulkan backend needs extra headers: **UNKNOWN**). | **No** package named `dolphin-emu` (the name `dolphin` is KDE's file manager). | No AppImage found. BELIEVED: the project's own Linux release is a Flatpak (see `docs/games-research.md`). |
| **PCSX2** | CMake with **clang and lld**, Ninja; the guide says "PCSX2 no longer supports the gcc compiler, as it has transitioned to clang/llvm" (SOURCE, `pcsx2.net/docs/advanced/building`). | It builds its own dependencies with `.github/workflows/scripts/linux/build-dependencies-qt.sh`: **Qt 6.11.2**, SDL3 3.4.16, FFmpeg 9.0.1 (as the script says), Vulkan headers 1.4.328.1, shaderc 2026.2, libjpeg-turbo, libpng, libwebp, LZ4, Zstandard, KDDockWidgets, others (SOURCE, script read 2026-10-03). The guide's Ubuntu apt list also names `libasound2-dev`, `libaio-dev`, FFmpeg dev packages, libcurl, and "many X11/Wayland libraries". | Qt6 required; Vulkan headers; both **Wayland and X11** dev libraries. | **No** (`pcsxr` is an unrelated, old emulator). | Release assets could not be read: AppImage or Flatpak: **UNKNOWN**. |
| **DuckStation** | CMake with **clang and lld**, Ninja; the README downloads "prebuilt dependencies from a dedicated repository to `dep/prebuilt`" (so a build is **not offline** as documented), output `./build-release/bin/duckstation-qt` (SOURCE, README). | The Ubuntu list includes `libdbus-1-dev`, `libudev-dev`, `libgudev-1.0-dev`, `libinput-dev`, `libevdev-dev`, `libpipewire-0.3-dev`, `libpulse-dev`, `libwayland-dev`, `libx11-dev`, many `libxcb-*-dev`, `libgtk-3-dev`, `nasm`, `llvm`. | Qt (the front end is `duckstation-qt`); "Vulkan, OpenGL, X11, and Wayland". | **No.** | **AppImage** x86_64, ARM32, ARM64 (README). |
| **xemu** | `build.sh` configures a QEMU tree: `--target-list=i386-softmmu`, `--extra-cflags="-DXBOX=1 -Wno-error=redundant-decls"`, `--disable-werror`, then `make` of `qemu-system-i386`; the product is `dist/xemu`, **not** an AppImage (SOURCE, `build.sh` read 2026-10-03). | `build.sh` lists no packages. The build documentation page I tried (`xemu.app/docs/building/`) returned 404: **the dependency list is UNKNOWN**. | OpenGL (it is a QEMU with a GL renderer); the exact needs: **UNKNOWN**. | **No** (`gxemul` is an unrelated emulator). | **AppImage** x86_64 and aarch64, **Flatpak** `app.xemu.xemu` on Flathub, an Ubuntu 22.04+ daily-build **PPA** (SOURCE, `xemu.app/docs/download`). |
| **mGBA** | CMake **3.1 or newer**; GCC, Clang or Visual Studio 2019 work (SOURCE, README). `mkdir build; cd build; cmake -DCMAKE_INSTALL_PREFIX:PATH=/usr ..; make`. | Optional: zlib, libpng, libedit, FFmpeg, libzip, SQLite3, libelf, Lua, json-c. | A **Qt front end ("heavy-weight", the README says Qt 5)** or an **SDL front end** ("light-weight", SDL 2). The README does not mention OpenGL, X11 or Wayland. | **Yes:** `mgba-qt` and `mgba-sdl` **0.10.2+dfsg-1.1build3**, and `libretro-mgba` the same (the project's current release is 0.10.5, so the archive is older). | **AppImage** x64 and arm64 on the releases page; Ubuntu tarballs for 18.04, 20.04, 22.04, 24.04 (SOURCE, `mgba.io/downloads.html`). |
| **MesenCE** | `make` (Clang by default; `USE_GCC=true make` for GCC; "Mesen usually runs faster when built with Clang"); needs "a version of Clang or GCC that supports C++17" and **"SDL2 and the .NET 10 SDK"** (SOURCE, `COMPILING.md`). | SDL2; the .NET 10 SDK is for the user interface. Apt package names and X11/Wayland/GTK needs: not stated (**UNKNOWN**). | SDL2 required at run time ("must be manually installed"). | **No.** | Not in the README or release notes read: **UNKNOWN** (section 2). |
| **bsnes** | Instructions not found in the README text I got: **UNKNOWN**. (Its repository describes nightly builds for Windows, macOS, Linux and FreeBSD.) | **UNKNOWN.** | **UNKNOWN.** | **No** bsnes. Only `libretro-bsnes-mercury-*` 094+git20220807-8build1, an old fork. | Nightly asset names not readable: **UNKNOWN**. The jgemu project also builds a `bsnes 2.1.2` (section 2). |
| **SameBoy** | `make` (target `sdl` is the default on Linux); "clang (Recommended; required for macOS) or GCC"; **rgbds** for the boot ROMs (SOURCE, README). `sudo make install`. | `libsdl2`, `libpng`; OpenGL 3.2 or later only for optional frame blending and some scaling filters. | SDL 2; the README mentions no GTK, X11 or Wayland. | **No.** | No prebuilt Linux binary found: **UNKNOWN**. |
| **Stella** | `make`; **"GNU g++-13 or Clang 19 (with C++23 support)"** (SOURCE, user guide, current version); the developer build page was not read. | SDL2, libpng, zlib, SQLite (from the archive's package, TESTED, below). | SDL 2 only. | **Yes:** `stella` **6.7.1+dfsg-1build2**, depends on `libsdl2-2.0-0`, `libpng16-16t64`, `libsqlite3-0`, `zlib1g` (the project's latest is 7.0c). | A 64-bit **.deb for Ubuntu 22.04** and a source tarball (`stella-emu.github.io/downloads.html`). |
| **BlastEm** | The project is in Mercurial (not read). Build page `retrodev.com/blastem/trac/wiki/Build` returned 404: **UNKNOWN**. | **UNKNOWN.** | **UNKNOWN** (the program is SDL2 and OpenGL, from memory: BELIEVED, not read). | **Yes:** `blastem` **0.6.3.4-1build1** (old). | Nightly **tar.gz** for Linux 64 and 32 bit (`retrodev.com/blastem/nightlies/`). |
| **Gopher64** (N64) | **Rust**, `cargo build --release` after a recursive clone and submodule update (SOURCE, README). | SDL3 libraries, `clang` and `llvm` (on Ubuntu 25). | SDL3; Vulkan not mentioned (**UNKNOWN**). | **No.** | Standalone executable; Flatpak (section 1). |
| **RMG** (N64) | CMake, C++; dependencies Qt6, SDL3, libusb, hidapi, libsamplerate, speexdsp, freetype, Vulkan headers, nasm (SOURCE, README). | as listed | **Qt6**, SDL3, Vulkan headers | **No** (see section 1). | AppImage, Flatpak, AUR, Fedora COPR. |

**What an AppImage needs at run time (SOURCE, AppImage documentation `docs.appimage.org`, read 2026-10-03):**

- **FUSE 2.** "AppImages require 'Filesystem in Userspace (FUSE)' ... specifically FUSE 2.x (provided by the `libfuse2` package). In Ubuntu 24.04, the `libfuse2` package was renamed to `libfuse2t64`." The archive has `libfuse2t64` 2.9.9-8.1build1 and also `fuse3` 3.14.0-5build1 (TESTED); a type-2 AppImage needs the FUSE 2 library, not only fuse3 (BELIEVED, from the first quote). `/dev/fuse` and a `fusermount` program must exist (BELIEVED; not stated in the page I read).
- **Without FUSE:** run with `--appimage-extract-and-run`, or set `APPIMAGE_EXTRACT_AND_RUN=1`, or unpack with `--appimage-extract`. The documentation warns that "Extracting an AppImage to run its contents is pretty expensive. It should only be done if you have no other options left." Extraction needs a writable place for the files (on this image `/tmp` is a tmpfs; BELIEVED enough space is a question, UNKNOWN).
- **Host libraries:** an AppImage assumes "basic libraries that can be assumed to be part of every target system (e.g., the C standard library or graphics libraries)", with an excludelist (glibc, zlib, GLib and similar stay on the host); "Applications should be built on the oldest possible system, allowing them to run on newer system." So the image needs **glibc at least as new as the one the AppImage was built on**, and the **GPU drivers and graphics libraries** on the host (Mesa, Vulkan, the NVIDIA driver). Which AppImage was built on which system: UNKNOWN (not read for any of them).
- **Qt-based AppImages** (DuckStation, mGBA's Qt build, RMG) normally bundle Qt (BELIEVED; not verified for each).

---

## 4. Diablo II: Resurrected through Steam and Proton, on a glibc image with no systemd, snap or Flatpak

**Which edition.** The Steam page (`store.steampowered.com/app/2536520`) is titled **"Diablo II: Resurrected – Infernal Edition"**, release date **February 11, 2026** on Steam (SOURCE, page read 2026-10-03). Its system requirements list **Windows 10 (64-bit)** only; Linux and SteamOS are not mentioned, and no Steam Deck status is shown on the page I read. A news article (PC Gamer, second-hand, BELIEVED) says the game is on Steam and Steam Deck Verified. On Linux, therefore, the game runs through Proton, which is what `HUB-OS.md` assumes.

**Is Battle.net needed for the Steam edition?** The store page says: "Internet connection, Battle.net® Account, and Battle.net® desktop app required to play." (SOURCE.) Players on the Steam discussions say that only a **linked Battle.net account** is needed and that the desktop app is not, and that the game uses a licence token valid for 30 days which needs the account to be online to renew (BELIEVED: forum posts, not Blizzard or Valve; contradicts the store page on the app). **So: the account is needed (SOURCE); whether the desktop app is needed: the store says yes, users say no; UNKNOWN which is right.** If the app were needed it would be a Windows program run under Wine, which no document I read covers.

**What Valve's documents say Steam needs on the host:**

| Need | What the document says | Label |
|---|---|---|
| Distribution and CPU | `ValveSoftware/steam-for-linux` README: "Latest Ubuntu or Ubuntu LTS with a 64-bit (`x86_64`, `AMD64`) Linux kernel"; 1 GHz Pentium 4 or AMD Opteron or better; 512 MB RAM; 5 GB free space. | SOURCE |
| 32-bit libraries | "Requires 64-bit *and* 32-bit (`i386`, `IA32`) glibc" and "Requires latest 64-bit *and* 32-bit graphics drivers". "Starting from 2025-08-15, glibc 2.31 or newer will be required." | SOURCE |
| Container runtime and bubblewrap | `ValveSoftware/steam-runtime` known-issues file: the container runtime (pressure-vessel) "requires the same user namespace capabilities as Flatpak"; on older systems "either the `bubblewrap` package from the OS must be installed, or the `kernel.unprivileged_userns_clone` sysctl parameter must be set to 1". Search summaries of the same file add "bwrap requires the ability to create user namespaces. On some kernels this is disabled by default." Valve's own container-runtime document (`gitlab.steamos.cloud/.../container-runtime.md`) is behind a bot check and was **not** read. | SOURCE (known-issues file); BELIEVED (the rest) |
| User namespaces in this project's kernel | The kernel fragment `image/kernel/qemu-test.frag` is a tinyconfig; nothing in this repository enables `CONFIG_USER_NS`. A gaming-box kernel would need it (the Gentoo wiki, third-party, lists `CONFIG_USER_NS` "for Proton compatibility mode"). | SOURCE (repository); BELIEVED (Gentoo wiki) |
| Snap | "Using the Steam container runtimes from inside a Snap sandbox is relatively fragile, because its AppArmor profile depends on specific paths and operations." Not relevant to an image with no snap. | SOURCE |
| Vulkan drivers | Proton games use Vulkan (BELIEVED). Gamescope's README (Valve) says AMD needs Mesa 20.3+, Intel Mesa 21.2+, and that "Gamescope requires Vulkan support" (via a search summary of the README: BELIEVED). | BELIEVED |
| Desktop or Gamescope | The Steam client is a graphical program; I found **no Valve document in the pages I could read that says whether it needs an X server or a Wayland compositor**: **UNKNOWN** (BELIEVED: it runs on X11 or Xwayland). Gamescope is a Valve micro-compositor with a standalone DRM mode and a nested mode and is **optional** (BELIEVED, same README). | UNKNOWN / BELIEVED |
| Tools in the Ubuntu 24.04 archive (TESTED) | `steam-installer` **1:1.0.0.79~ds-2** (multiverse), depends on `steam-libs` (= same version), `steam-libs-i386` and `zenity | yad`; `bubblewrap` **0.9.0-1ubuntu0.3**; `libfuse2t64`; **no** `gamescope`, **no** `steam` or `steam-launcher` package. | TESTED |

**Does Steam run without systemd?** I found **no Valve statement either way** (UNKNOWN as an official answer). Third-party: the Gentoo wiki's Steam page documents OpenRC as well as systemd, and lists the kernel and sysctl needs (tmpfs for `/dev/shm`, `CONFIG_COMPAT_32BIT_TIME`, `CONFIG_USER_NS`, `CONFIG_NTSYNC` for GE-Proton or Proton 11+, `CONFIG_INPUT_UINPUT` for controllers; file descriptor limit 524288, `vm.max_map_count` 1048576); an Artix (OpenRC) forum thread reports a `libsystemd` requirement when installing Steam from a distribution package. Both are BELIEVED, not tested. The Steam client **uses** `libsystemd0` if the distribution package depends on it; the tolerated libraries (`HUB-OS.md`) would cover that.

**Where Steam keeps its files.** From search summaries of community guides (BELIEVED; not a Valve document): the client in `~/.local/share/Steam`, games in `steamapps/common/<name>`, Proton prefixes (Wine prefixes) in `steamapps/compatdata/<AppID>/pfx`. For `HUB-OS.md` this means the immutable image needs a writable per-user area on the gaming box's own SSD ("game files live on the box's own SSD"); where and how large (the game needs 43 GB per the store page): a design question.

---

## 5. Flatpak on an image with no systemd

Official sources (SOURCE, read 2026-10-03):

- **Flatpak FAQ** (`flatpak.org/faq/`): "**Is Flatpak tied to systemd?** No. Versions of Flatpak before 0.6.10 relied on systemd for cgroups setup, but this is no longer required." And: "Flatpak is designed to run inside a desktop session and relies on certain session services, such as a **D-Bus session bus** and, optionally, a **systemd `--user` instance**."
- **Flatpak documentation, "Under the hood"** (`docs.flatpak.org/en/latest/under-the-hood.html`) lists what Flatpak builds on: **bubblewrap**, "**systemd** to set up cgroups for sandboxes", **D-Bus**, and **OSTree**. This page says systemd without "optionally"; **it disagrees with the FAQ**, and I could not tell which is newer. Treat the FAQ as the more specific statement.
- **Flatpak NEWS** (1.16.1, 2025-05-10): "Make systemd scopes easier to match to Flatpak app instances, by using the instance ID instead of the top-level process ID in the scope name." So systemd scopes are used when present (BELIEVED optional, per the FAQ).
- Release feed: 1.19.2 and 1.19.1 (2026-09-28), 1.18.4 (2026-09-28), 1.18.3 (2026-09-22). Ubuntu 24.04's archive has Flatpak **1.14.6-1ubuntu0.1** (TESTED).
- Valve's note that the Steam container runtime has "the same user namespace requirements as Flatpak" (section 4) means both need unprivileged user namespaces (or a setuid bubblewrap).

**Answer, per the documentation:** Flatpak needs **bubblewrap and a D-Bus session bus** on the host; systemd is **not required** (FAQ), but one page of the documentation still lists it. I did **not** find in the pages I read whether the **system helper** (installing for all users) needs polkit or the system D-Bus: **UNKNOWN**. Third-party commentary (linuxiac, OSNews, a Devuan forum; BELIEVED) discusses a proposed `systemd-appd` that "would introduce a dependency on systemd into Flatpak"; I did not verify that proposal or its status in Flatpak's own repository. Nothing was installed or run, so none of this is TESTED beyond the package version.

For `HUB-OS.md`: the hub allows a D-Bus **session** bus (`dbus-daemon` alone); the gaming box and the other nodes have no D-Bus rule yet, so a Flatpak route would need that decided first.

---

## 6. What was not done

- Nothing was installed, built or run; no AppImage was opened to see how it was built.
- Not read: bsnes's and BlastEm's build instructions, xemu's dependency list, Valve's container-runtime document, Proton's own wiki, Gopher64's Flathub page, the contents of any release asset.
- RMG's netplay kind, Gopher64's Vulkan need and the format of MesenCE's Linux releases are UNKNOWN as written above.
