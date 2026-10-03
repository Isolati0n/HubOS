# Input sharing: prior art and two designs for the forwarder

> **RESEARCH; nothing here is a decision.** Nothing was installed, built or run, and nothing was pushed anywhere except this file's branch. Everything was read on the web or in this repository on **2026-10-03**.

**Why:** `HUB-OS.md` ("Input sharing") says one keyboard and mouse plug into the gaming box, the chord **Super + Ctrl + Shift** flips input between the gaming box and the hub, and "our own forwarder" sends keystrokes and mouse movement over the network while input is on the hub. It also lists "the input forwarder approach (reading raw input devices and creating virtual ones)" as unverified. This file collects prior art and compares two ways to build the gaming-box side.

## 0. Labels and limits

**TESTED** (I ran it here), **SOURCE** (read at the named place on the named date), **BELIEVED** (second-hand, or the primary page could not be read), **UNKNOWN** (not found; I do not guess).

- The tool that reads web pages returns a summary, not the raw page. Text in quotation marks is as it returned it; I could not check it against the raw page. Dates come from release feeds (`.../releases.atom`) where I give a year.
- `/dev/uinput` and `/dev/input` do not exist in the build environment (TESTED, `ls -l /dev/uinput /dev/input`: "No such file or directory"). So **no forwarder design could be run here**; this matches the existing line in `HUB-OS.md`. Every statement about behaviour below is read, not tested.
- The kernel source quoted below is the `master` branch of `torvalds/linux` as served on 2026-10-03, **not** the kernel version the images will use (6.12, `image/machines/qemu-test.build`). Whether these functions are the same in 6.12: UNKNOWN (not compared).

---

## 1. Lan Mouse

A software KVM written in Rust. It is the project that Deskflow's FAQ calls a "fellow open source project": the FAQ says "We would love to see compatibility between our fellow open source projects, Lan Mouse and Input Leap. This idea is discussed occasionally in the communities for all of our projects, so it may happen in the not-too-distant future." (SOURCE, `github.com/deskflow/deskflow/wiki/Project-FAQ`.)

| Question | What I found | Label |
|---|---|---|
| **Last release and date** | GitHub: only **rolling builds** named `main-<commit>`; the newest, `main-f1b96bd`, **2026-09-26T12:11:12Z** (then `main-f36b366` 2026-09-24, `main-5f7e113` 2026-09-24). The tags page shows no version-numbered tags. crates.io: newest version **0.11.0, published 2026-06-12**, before it 0.10.0 (2024-11-07) and 0.9.1 (2024-07-30). | SOURCE (release feed; `crates.io/api/v1/crates/lan-mouse`) |
| **Licence** | GPL-3.0-or-later (crates.io field); the repository's `LICENSE` file starts "GNU GENERAL PUBLIC LICENSE Version 3, 29 June 2007". | SOURCE |
| **How it sends (grabs input) on Wayland** | README: for **wlroots-based** compositors the capture backend "creates a single pixel wide window on the edges of Displays to capture the cursor using the layer-shell protocol". For **GNOME ≥ 45 and KDE Plasma ≥ 6.1** it uses **libei** (and the layer-shell is also listed). X11 capture is "work in progress". There is no evdev (`/dev/input`) capture backend in the README I read. | SOURCE (README, raw, read 2026-10-03) |
| **How it receives (injects input) on Wayland** | README: wlroots: the **wlr-virtual-pointer** and **virtual-keyboard** protocols; KDE/GNOME: **libei** and the **freedesktop remote-desktop portal**; X11, Windows, macOS: native backends. A `uinput` backend: **not shown** in the README text I got (UNKNOWN whether one exists). | SOURCE / UNKNOWN |
| **Needs a portal?** | **Only on GNOME and KDE.** The libei and remote-desktop-portal backends depend on the desktop's portal. On wlroots compositors the README names layer-shell and the virtual-input protocols and does not mention a portal. | SOURCE (README); the reading "no portal on wlroots" is my reading of what the README does *not* say: BELIEVED |
| **Needs D-Bus?** | The README text I got does not mention D-Bus (not shown). The portal route is D-Bus by nature (BELIEVED). | UNKNOWN / BELIEVED |
| **Needs a particular compositor?** | README: fully supported are GNOME ≥ 45, KDE Plasma ≥ 6.1, "most wlroots-based compositors (Sway ≥ 1.8, Hyprland, Wayfire)", Windows and macOS. Wayfire "requires recent versions and shortcuts-inhibit plugin". "Sway/wlroots: Modifier keys may not function properly in certain configurations." | SOURCE |
| **Other facts that matter** | Traffic is encrypted ("DTLS implementation provided by WebRTC.rs"; "no mitigations ... for timing side-channel attacks"). UDP port 4242 must be open. A `release_bind` setting names the keys that give input back, written as key names such as `[ "KeyA", "KeyS", "KeyD", "KeyF" ]`; no fixed default is shown. A daemon mode exists, with a **systemd service file** in the repository (`service/lan-mouse.service`); this project never uses systemd, so that file would not be used. | SOURCE |
| **Known problems with games** | Issue **#132** (open, 2024-05-13, label `windows`): "Some programs with mouse capture (like FPS games) fail to prevent the virtual cursor from moving & leaving the screen"; the reporter saw it in Fortnite and not in Team Fortress 2, Fall Guys, Minecraft or a browser; no maintainer reply shown. Issue **#332** (open, 2025-10-24): "Steam Deck Support in Game mode and Desktop mode (Client) - Flatpak needed?". Both are about the *receiving* side or a client; none is about games running on the *sending* machine. | SOURCE (issue pages, read 2026-10-03) |
| **Known problems with stuck keys** | **#369** "release_bind not clearing modifiers?" (opened 2026-01-30, **closed**, fixed by pull request #371): with `release_bind` = `Meta+z` on Linux the Meta modifier stayed active after the bind fired, so `1` acted as `Meta+1`; the reporter "couldn't type [their] password" at the lock screen. **#357** "[Linux client + MacOS server] Sticky modifier keys" (opened 2025-12-12, closed): on KDE/Wayland, modifiers stuck down after combinations like Meta+Tab. **#199** "MacOS Client: Globe Modifier sometimes gets sent / stuck" (closed 2024-10-27). I did not see the fixes. | SOURCE (titles and reports); fixes UNKNOWN |

**What this means for Hub OS (my reading, BELIEVED):** Lan Mouse's model is "the cursor leaves a screen edge", a layer-shell edge window on the sender or libei capture. The Hub OS flip is a **key chord** on a keyboard plugged into a gaming box that has no desktop environment described. Lan Mouse's README lists no evdev capture, so I do not see a way to use it as the sender as it is. Its `release_bind`, and the three stuck-modifier issues, are still useful evidence that *releasing modifiers at a flip is a real problem in this kind of software*.

**On the hub side (receiver), driftwm:** `docs/driftwm-findings.md` section 5 (TESTED there) lists 47 Wayland globals. It has **no** `zwlr_virtual_pointer_manager_v1` (a source search found nothing), but it **does** list `zwp_virtual_keyboard_manager_v1` v1 (section 5 of that file: "Yes ... a vendored copy of Smithay's with compositor key bindings added"; listed: TESTED). So on driftwm Lan Mouse's wlroots receive route (virtual pointer plus virtual keyboard) has a keyboard protocol but **no pointer protocol**: it cannot move the pointer or press mouse buttons, and the keyboard half alone is not enough for the flip. The libei and portal backends need an EIS server or a RemoteDesktop portal; whether driftwm offers either: **UNKNOWN** (the findings do not mention libei; they say the compositor does not use PipeWire, so portal screen-casting is not there). A `uinput` device created by our own program on the hub does not depend on any of this (BELIEVED, and untestable here without `/dev/uinput`).

---

## 2. Deskflow's clipboard on Wayland, and driftwm

**What Deskflow's release notes say (SOURCE, `github.com/deskflow/deskflow`, read 2026-10-03):**

- **v1.27.0** (feed `updated` 2026-10-01T12:23:16Z): "Clipboard support for Wayland (input capture and remote desktop portals)" (pull request #9431); "fix(wayland): Prevent clipboard causing portal reprompts if the backend is incorrectly reporting clipboard state" (#10124); plus Wayland fixes for edge barriers, scrolling, cursor coordinates, held mouse buttons and locked modifiers. This is the **portal** route (libei and libportal).
- **v1.26.0** (the page shows "16 Feb 13:37" with no year; the feed date was not read): "refactor: WlClipboard, close not kill running copy process on exit" (#9380) and "fix: Restore clipboard sharing after 5344182" (#9307). `WlClipboard` is the **wl-clipboard** route.
- Issue **#10135** (opened 2026-09-07, **closed as not planned**), titled "Wayland client (wl-clipboard path): never sends clipboard to server (ClipboardChanged has no client handler) + dock flicker from wl-paste polling on GNOME": the reporter says `WlClipboard::monitorClipboard()` runs `wl-paste --list-types` every second; "since mutter lacks `wlr-data-control` support, each poll creates a focus-grabbing window"; and a Wayland **client** never sends its clipboard to the server (server-to-client works). No maintainer reply was shown.

**Would the wl-clipboard path work on a compositor that advertises `zwlr_data_control_manager_v1` and `ext_data_control_manager_v1`, as driftwm does?** driftwm does advertise both (SOURCE, `docs/driftwm-findings.md` section 5: `zwlr_data_control_manager_v1 v2`, `ext_data_control_manager_v1 v1`).

- **Receiving a clipboard from another machine into the compositor** (server to client): I would expect it to work, because the path runs `wl-copy`/`wl-paste` and wl-clipboard uses the wlr data-control protocol, which driftwm offers. **BELIEVED**: I did not read wl-clipboard's source or run it. The mutter flicker problem in #10135 comes from a compositor *without* data-control; driftwm has it.
- **Sending the clipboard of the driftwm machine to the other machine** (client to server): **No, by the issue above**, as of that report: the wl-clipboard path "never sends clipboard to server" (closed as not planned, so no fix is expected from that thread). If Deskflow's own bug is as described, it does not depend on the compositor.
- **ext-data-control:** wl-clipboard issue **#242** "Support ext-data-control wayland-protocol (replacing wlr-data-control)" (opened 2024-11-11, **closed**; a pull request #255 is linked; no maintainer reply shown). Whether the latest wl-clipboard release speaks `ext-data-control` is **UNKNOWN**. It makes no difference for driftwm, which offers both.
- **Whole Deskflow on driftwm:** Deskflow's Wayland *input* uses **libei and libportal** (its FAQ: "The libei and libportal libraries enable Wayland support" as summarised by a search engine; BELIEVED). Whether driftwm gives libei something to talk to is UNKNOWN (section 1). So even if the clipboard worked, **input** through Deskflow on driftwm is not shown to work.

---

## 3. evdev grabbing: where it is documented

| Statement | Where | Label |
|---|---|---|
| The ioctl exists: `#define EVIOCGRAB _IOW('E', 0x90, int) /* Grab/Release device */`, next to `EVIOCREVOKE` ("Revoke device access") | `include/uapi/linux/input.h` (`github.com/torvalds/linux`, master, read 2026-10-03) | SOURCE |
| **Only one grab at a time:** `evdev_grab()` returns **`-EBUSY`** if the device is already grabbed; otherwise it calls `input_grab_device()` and records the client. `evdev_ungrab()` returns `-EINVAL` if the caller is not the grabber. | `drivers/input/evdev.c` lines 363 to 386 (same source) | SOURCE |
| **A grabbed device delivers events to the grabber only; an ungrabbed device delivers every event to every client:** in `evdev_events()`: `client = rcu_dereference(evdev->grab); if (client) evdev_pass_values(client, ...); else list_for_each_entry_rcu(client, &evdev->client_list, node) evdev_pass_values(client, ...)` | `drivers/input/evdev.c` lines 432 to 448 | SOURCE |
| libevdev's wording: grabbing "prevents other clients (including kernel-internal ones such as rfkill) from receiving events from this device"; grabbing an already grabbed device (by the same handle) is a no-op | `freedesktop.org/software/libevdev/doc/` (`libevdev_grab`); the page itself returned 403, so this is a search engine's quotation | BELIEVED |
| **Kernel documentation page** `docs.kernel.org/input/input.html`: it describes evdev as "the generic input event interface" that passes events "straight to the program, with timestamps". I **did not find** `EVIOCGRAB`, a second reader, or `uinput` on that page. | the page, read 2026-10-03 | SOURCE (absence on that page) |
| **uinput** in the kernel documentation: "uinput is a kernel module that makes it possible to emulate input devices from userspace. By writing to /dev/uinput (or /dev/input/uinput) device, a process can create a virtual input device with specific capabilities." Setup with `UI_DEV_SETUP` then `UI_DEV_CREATE`; keys with `UI_SET_EVBIT`/`UI_SET_KEYBIT`; mice with `EV_REL` (`REL_X`, `REL_Y`); events written with `write()` and a `SYN_REPORT` after each sequence. Permissions, autorepeat and LEDs/force feedback are **not** covered on that page. | `docs.kernel.org/input/uinput.html`, read 2026-10-03 | SOURCE |

**So, a second program reading the same device:** by the code, if nobody has grabbed it, **every reader gets every event** (so a forwarder can read the keyboard that the compositor and the games also read, without disturbing them). The kernel *documentation* I read does not say this in words; the code does.

---

## 4. Two designs for the forwarder on the gaming box

Common to both: the forwarder reads the real keyboard and mouse as evdev devices, detects the chord, and while input is on the hub sends events over the network; the hub creates virtual devices (or another injection route, section 1) and sends "release all keys" on each flip, as `HUB-OS.md` says.

### Design 1: always grab, re-inject locally through uinput when not forwarding

The forwarder holds `EVIOCGRAB` on the real devices all the time. When input belongs to the gaming box it writes every event to its own uinput keyboard and mouse; when it belongs to the hub it sends them over the network instead.

**Known (with sources):**

- The grab works as intended by the code: while grabbed, only the forwarder gets events (section 3).
- **The chord can be hidden from games completely**, because nothing reaches the games unless the forwarder re-injects it (follows from the grab semantics, SOURCE for the semantics; BELIEVED for "completely": no game was run).
- uinput can create virtual keyboards and mice from userspace (section 3, SOURCE).
- **Prior art does exactly this:** `keyd` ("a flexible system wide daemon which remaps keys using kernel level input primitives (evdev, uinput)"; "Speed (a hand tuned input loop written in C that takes <<1ms)"; works in a virtual terminal and under X, sway and GNOME) and `evsieve` (a plain pass-through "has effectively accomplished nothing besides adding about 0.15ms of latency"). Both numbers are the authors' own claims; I did not measure anything (SOURCE for what they say).
- A grab by another program makes ours fail with `-EBUSY` (section 3), and ours blocks theirs.

**Unknown:**

- **The real added delay on the gaming box**, with a mouse at its full polling rate (1000 Hz, or higher), CPU core isolation and a real-time kernel as `HUB-OS.md` plans. The authors' figures above are for other programs on other machines. **UNKNOWN; cannot be measured here** (no `/dev/uinput`, no hardware, and `HUB-OS.md` notes true click-to-screen delay needs external measuring hardware).
- Whether the **polling rate survives**: a mouse reporting at 8000 Hz gives the forwarder an event stream that it must write out event by event; `HUB-OS.md` already asks for "a keyboard and mouse that keep their polling rate through the forwarder". **UNKNOWN.**
- Whether games and the compositor treat a virtual device the same as the real one: device name, vendor and product ids, the mouse's extra buttons and DPI keys, LEDs (caps lock) and the kernel's autorepeat. The forwarder must create its virtual devices with matching capabilities; whether anything is lost: **UNKNOWN**.
- Event **timestamps**: BELIEVED that events written to uinput get a new timestamp at write time, so a game that cares about the original timestamp would see the forwarder's; I did not read that. **UNKNOWN.**
- **What happens if the forwarder crashes while holding the grab.** BELIEVED: the kernel drops the grab when the file is closed, and the virtual devices disappear with the process, which leaves the physical keyboard direct again but could leave a key logically down in the compositor. Not read, not tested. The spare keyboard on the hub (`HUB-OS.md`) is the stated fallback. **UNKNOWN.**
- Whether the gaming box's own boot (picker, compositor) is able to start before the forwarder grabs; a keyboard that is grabbed but not yet re-injected is dead. **UNKNOWN**; a design question, not a research one.
- Stuck keys at the start: the first grab happens at boot with no key held (BELIEVED harmless).

### Design 2: grab only while forwarding

The forwarder reads the devices ungrabbed (a second reader). When the chord completes it grabs the devices and sends events to the hub; at the next chord it ungrabs.

**Known (with sources):**

- **The gaming path stays direct while input is on the gaming box:** an ungrabbed device delivers every event to every client, so the compositor and games read the device as they do now; the forwarder is just one more reader (section 3, SOURCE code). Nothing is added to the gaming path.
- **The chord is also seen by the gaming box** when it is pressed (as you said): the forwarder cannot hide it, because it only learns of it as the same events arrive. What the gaming box does with Super, Ctrl and Shift held at that moment (the Windows/Super key in a game, a compositor shortcut): **UNKNOWN** (not tested).
- **The stuck-modifier hazard is documented:** `python-evdev` issue #4 (2013-01-07): a program grabbed the keyboard when Left Ctrl went down and ungrabbed it when Ctrl was released; afterwards "the system behaves as if LCtrl were still held down". `evsieve` handles this by default: with `grab=auto` it "will not grab the device until no keys are in the 'down' state", while `grab=force` "will immediately grab the event device as soon as evsieve starts", which "can cause stuck key states if a key is pressed during startup" (SOURCE for both texts). The mechanism, by the code in section 3: once the device is grabbed, the gaming box's clients get no more events, so a key-up for a key they saw go down never arrives.
- So for Design 2, a flip *to* the hub has to **wait until the chord keys are released** before grabbing (as `grab=auto` does), or else send key-up events to the gaming box some other way (not possible without a uinput device, which is Design 1's cost). A flip *back* has the opposite hazard: keys held on the hub side when input returns; `HUB-OS.md` already specifies the "release all keys" on each flip for the machine that was just left.
- Prior art for "grab and ungrab by a chord" exists as a pattern (the python-evdev issue is an example); I did not find a project that does exactly this for a keyboard-and-mouse flip.

**Unknown:**

- The **gap at the moment of the grab**: events between the chord's last key and the `EVIOCGRAB` call still go to the gaming box (some mouse movement). The size of the gap and whether it matters: **UNKNOWN**.
- Whether the mouse needs special handling: a mouse button held at the flip, relative motion left in the kernel's queue. **UNKNOWN.**
- What the gaming box's compositor and libinput do with a key that is released after an ungrab without having seen it pressed (the flip back). **UNKNOWN** (BELIEVED harmless; not read).
- Whether another program on the gaming box already holds a grab (then `EVIOCGRAB` fails with `-EBUSY`). **UNKNOWN** until the real image exists.
- Whether the hub-side injection (uinput on the hub) works with driftwm's input handling: the same open question as in section 1; **UNKNOWN** here.

### Side by side

| | Design 1: always grab, re-inject | Design 2: grab only while forwarding |
|---|---|---|
| Gaming path when input is on the gaming box | **one more hop** (forwarder, then uinput); size UNKNOWN | **direct**, nothing in between |
| Chord visible to games | **hidden** | **seen** by the gaming box when pressed |
| Stuck keys on the gaming box | possible only if the forwarder's own re-injection loses a release; one place to get it right | **documented hazard** at each grab; avoided by grabbing only when no key is down, which delays the flip until the chord is released |
| If the forwarder dies | the keyboard may be dead until it restarts or the grab drops (BELIEVED the kernel drops it); UNKNOWN | the keyboard keeps working (it was never grabbed), unless it died while grabbed |
| Needs `/dev/uinput` on the gaming box | **yes** | no (the hub needs it for its own injection either way, or another route) |
| Can be tested in the build environment | no (`/dev/uinput` absent, TESTED) | no (no input devices at all) |

---

## 5. What was not done

- Nothing was installed, built or run. Lan Mouse, Deskflow, keyd and evsieve were read, not tried.
- I did not read Lan Mouse's source, so whether a `uinput` or evdev backend exists beyond the README is UNKNOWN.
- I did not read the Linux 6.12 version of `evdev.c`.
- I did not look for other software KVMs (Input Leap, Barrier, Synergy, ShareMouse) beyond their names appearing in search results.
- I did not look at how Moonlight/Sunshine forward input, which is a third route for casual play (`HUB-OS.md` already uses Moonlight for that).
