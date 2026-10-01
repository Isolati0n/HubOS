# Bar findings

**Researched: 2026-10-01 — build environment; will change.** This file records what was run to find out how a bar (Waybar) and a list menu behave with driftwm. `HUB-OS.md` wins if anything here disagrees with it. It builds on `docs/driftwm-findings.md` (called "the driftwm findings" below) and `docs/environment.md`.

- **driftwm:** `malbiruk/driftwm` at the pinned commit `352333a8fa1b22171492d4b71a54102045c9a19d` (version 0.19.0), built and run nested on a virtual X display with a software renderer, exactly as in section 0 of the driftwm findings. **Nothing was run on a real screen, GPU or input device.** driftwm was only read and run. Nothing was pushed to it, forked, or filed on it, and it was not changed.
- **Waybar:** two versions were run. `v0.9.24` is what Ubuntu 24.04's archive has. `v0.12.0` is what Debian 13's archive has, run inside a throw-away Debian 13 root directory (`chroot`) to find out what a newer Waybar adds.
- **Screenshots** were taken with `grim` and looked at as images. They are not stored in this repository. What they showed is described in words.

Every item is labelled with exactly one of:

- **TESTED** — the command shown was run in this session and gave the result written here.
- **BELIEVED** — taken from reading, or thought true, and not run. The reason is given.
- **UNKNOWN** — not known.

---

## Summary: working options for "bar item with an alert, plus a scrollable grouped dropdown"

Nothing is chosen here. The owner decides.

**The alert part works the same in every option.** A Waybar `custom` module runs a program that prints one line of JSON. The text changes, a CSS class makes it red, a click runs a command, and an outside program can refresh it at once. All of it was run (sections 3 and 4). The bar reserves screen space, so `fit-window` stops below it (section 2). It does not change where windows sit on the canvas (section 7).

**The dropdown part has four options:**

| Option | Opens on click | Scrolls with 30+ entries | Headings | Run a command on pick | Search | Placed under the bar | Main limit |
|---|---|---|---|---|---|---|---|
| **A. Bar item + a list launcher** (`wofi`, `tofi`, `fuzzel`, `bemenu` in Ubuntu 24.04), started by the module's `on-click` | Yes (run end to end with `wofi`) | **Yes** in all four (arrow keys). `wofi` also shows a scrollbar | Shown as ordinary lines. **Picking one prints it**; the handler has to ignore it | Yes: the pick is printed; a script acts on it | Yes, as you type | `wofi`, `tofi`: yes. `fuzzel` 1.9.2: **no, always centred**. `bemenu`: full-width strip at the top, over the bar | Headings are not greyed or skipped by the launcher. `fuzzel` and `bemenu` need a UTF-8 locale for non-ASCII headings. `wofi` re-orders by past picks unless told not to |
| **B. Waybar's own menu** (`menu` on a custom module) | Yes | **No** in the test: a menu taller than the screen was cut off and could not be scrolled by wheel, edge hover or keys. 30 entries (about 1,100 px) did not fit an 800 px screen | **Yes**, greyed and not clickable, with separators | Yes, per entry (`menu-actions`) | No | Yes (opens under the pointer, starts at the top of the screen) | **Needs Waybar 0.10 or later. Ubuntu 24.04's 0.9.24 ignores the option.** Only 0.12.0 (Debian 13) was run |
| **C. Tooltip on hover** | On hover | No | Plain multi-line text | No | No | Yes | Read-only text, not a menu |
| **D. `wlr/taskbar` module** (lists windows, not machines) | Click | Not tested with many windows | n/a | Click focuses that window and pans to it | No | Part of the bar | Lists windows, not the machine list `HUB-OS.md` describes |

A fifth way, not tried: a bespoke panel written by us (`HUB-OS.md` lists "The panel (bar item and dropdown)" among the parts we write ourselves).

**Facts that apply to every option:**

1. **Waybar needs a D-Bus session bus.** Both versions exit with status 1 without one (section 1). A plain `dbus-daemon --session`, with no systemd, was enough. Waybar does not need systemd.
2. **Fullscreen hides the bar** (section 2).
3. **Keyboard input goes to the focused window, not to a Waybar menu.** The `Down` keys that were meant for a Waybar menu landed in the terminal below (section 5). The list launchers take the keyboard themselves.
4. **Updates can be pushed**: a signal refresh took 17 ms; a `tail -F` feed took under half a second (section 4).

---

## 0. Set-up, and what changed from section 0 of the driftwm findings

The same method: unpack packages into a temporary directory, install nothing. `A` is an empty temporary directory outside the repo.

```
A=$(mktemp -d)
mkdir -p $A/apt/lists/partial $A/apt/cache/archives/partial $A/debs $A/root
O="-o Dir::State::lists=$A/apt/lists -o Dir::Cache=$A/apt/cache -o APT::Sandbox::User=root"
apt-get $O update
```

What changed, and what stayed the same:

- **Pinned commit (changed).** A shallow clone gives whatever `main` is today, so the exact commit was fetched instead. It worked; the proxy allows fetching a commit by its hash:
  ```
  mkdir -p $A/driftwm && cd $A/driftwm && git init -q && git remote add origin https://github.com/malbiruk/driftwm
  GIT_LFS_SKIP_SMUDGE=1 git fetch --depth 1 origin 352333a8fa1b22171492d4b71a54102045c9a19d
  GIT_LFS_SKIP_SMUDGE=1 git checkout -q FETCH_HEAD
  git log -1 --format='%H %ad %s' --date=short ; grep -m1 '^version' Cargo.toml
  ```
  Output: `352333a8fa1b22171492d4b71a54102045c9a19d 2026-09-22 docs: add DriftGlide to Community section in README (#287)` and `version = "0.19.0"`.
- **Package list (changed).** One download list for everything:
  ```
  apt-get $O install --print-uris -y --no-install-recommends \
    libwayland-dev libxkbcommon-dev libinput-dev libudev-dev libseat-dev libgbm-dev libegl-dev libgles-dev libdrm-dev libdisplay-info-dev \
    libegl1 libgles2 libgbm1 libgl1-mesa-dri libwayland-egl1 libegl-mesa0 libglx-mesa0 xkb-data libxcb1-dev \
    libgudev-1.0-0 libxkbcommon-x11-0 libxcb-xkb1 libxcursor1 libxrandr2 libxi6 \
    xvfb xserver-common foot libfcft4t64 libutf8proc3 wayland-utils xdotool libxdo3 grim fuzzel \
    waybar wofi bemenu tofi jq socat fonts-dejavu-core fonts-font-awesome \
    | grep -oE "^'[^']+'" | tr -d "'" | sort -u > $A/uris-all.txt
  ```
  New in this list: `waybar wofi bemenu tofi jq socat fonts-dejavu-core fonts-font-awesome`. Packages built from the **systemd** source (`/s/systemd/` in the URL) were not downloaded, except the **headers and pkg-config file** of `libudev-dev`. `ncurses-*`, `libncurses*`, `libtinfo6`, `gir1.2-gudev` and `libgudev-1.0-dev` were also left out. Each URL was fetched with `curl -o` and unpacked with `dpkg -x ... $A/root`. Then the same pkg-config and symbolic-link repair as the driftwm findings section 0.
- **Environment (same, plus two lines).** Same variables as before, with `XDG_RUNTIME_DIR=/tmp/dwx` (a short path is required). Added: `export XDG_DATA_DIRS=$A/root/usr/share:/usr/local/share:/usr/share`.
- **Build (same).** `cargo build` in the pinned checkout, **95 seconds**, exit 0. **TESTED**
- **`WAYLAND_DISPLAY` must be unset when starting driftwm.** If the shell still has `WAYLAND_DISPLAY=wayland-1` from an earlier session, driftwm picks its nested Wayland mode and stops with `NoCompositor`. The start script uses `env -u WAYLAND_DISPLAY DISPLAY=:99 ...`. **TESTED** (that failure happened once in this session).
- **A session D-Bus is needed for Waybar** (section 1):
  ```
  dbus-daemon --session --address=unix:path=/tmp/dwx/bus --fork --nopidfile --print-pid
  export DBUS_SESSION_BUS_ADDRESS=unix:path=/tmp/dwx/bus
  ```
- **`bemenu` needs its plug-in folder named** in this unpacked set-up: `export BEMENU_BACKEND=wayland BEMENU_RENDERERS=$A/root/usr/lib/x86_64-linux-gnu/bemenu`. Without it `bemenu` exited with status 1 and printed nothing (**TESTED**).
- **UTF-8 locale.** `export LC_ALL=C.UTF-8` for the Waybar and launcher runs (see section 6 for why).

The start script used for every run (Xvfb, driftwm nested, two terminal windows, X focus to driftwm's window):

```
Xvfb :99 -screen 0 1280x800x24 -nolisten tcp &
env -u WAYLAND_DISPLAY DISPLAY=:99 $A/target/debug/driftwm --backend winit --config /dev/null > $A/dw.log 2>&1 &
export WAYLAND_DISPLAY=wayland-1 DISPLAY=:99
foot --app-id=term-a --title=First sleep 3000 &
foot --app-id=term-b --title=Second sleep 3000 &
xdotool windowfocus $(xdotool search --name '.' | head -1)
alias dw="$A/target/debug/driftwm"
```

Waybar's files for most runs, in `/tmp/dwx/wb/`:

```
# hub.sh -- prints one JSON object; the text comes from a file so a test can change it
#!/bin/bash
echo "ran $(date +%T.%N)" >> /tmp/dwx/wb/runs.log
s=$(cat /tmp/dwx/wb/state 2>/dev/null || echo "4 of 4 up")
if [ "$s" = "4 of 4 up" ]; then cls=ok; else cls=alert; fi
printf '{"text":"%s","class":"%s","tooltip":"hub state: %s\\nupdated %s"}\n' "$s" "$cls" "$s" "$(date +%T)"

# config.json
{ "layer": "top", "position": "top", "height": 30,
  "modules-left": ["custom/hub"], "modules-center": [], "modules-right": ["clock"],
  "custom/hub": { "exec": "/tmp/dwx/wb/hub.sh", "return-type": "json", "interval": 2, "signal": 8,
                  "on-click": "echo \"clicked $(date +%T)\" >> /tmp/dwx/wb/clicks.log" },
  "clock": { "format": "{:%H:%M:%S}", "interval": 1 } }

# style.css
* { font-family: "DejaVu Sans"; font-size: 14px; }
window#waybar { background: #202020; color: #ffffff; }
#custom-hub { padding: 0 12px; }
#custom-hub.ok { background: #2e7d32; color: #ffffff; }
#custom-hub.alert { background: #c62828; color: #ffffff; }
#clock { padding: 0 12px; }
```

(`hub.sh` was made executable and `echo "4 of 4 up" > /tmp/dwx/wb/state` was run first. In the file the tooltip's line break is written `\\n` inside the `printf` format, so the JSON contains `\n`, which the tooltip shows as a line break.)

---

## 1. Does Waybar run under driftwm with no systemd and no D-Bus?

**Version:** `waybar --version` → `Waybar v0.9.24`. **TESTED**

**No D-Bus: no.** Both runs exit with status 1 and no bar appears:

```
# run in the foreground with no DBUS_SESSION_BUS_ADDRESS at all
timeout 8 waybar -c /tmp/dwx/wb/config.json -s /tmp/dwx/wb/style.css -l debug ; echo "exit: $?"
```
Output: `[info] Using configuration file …` then `[error] Failed to execute child process “dbus-launch” (No such file or directory)`, `exit: 1`. **TESTED**

```
# a session-bus address that points at nothing
DBUS_SESSION_BUS_ADDRESS=unix:path=/tmp/dwx/no-such-bus timeout 10 waybar -c /tmp/dwx/wb/config.json -s /tmp/dwx/wb/style.css -l debug
```
Output: `[error] Could not connect: No such file or directory`, `exit: 1`. **TESTED.** Waybar 0.12.0 (Debian 13) did the same: `[error] Could not connect: No such file or directory`, `exit: 1`. **TESTED**

**With a plain session bus and no systemd: yes.** `dbus-daemon 1.14.10`, started as in section 0, then:

```
waybar -c /tmp/dwx/wb/config.json -s /tmp/dwx/wb/style.css -l debug > $A/waybar3.log 2>&1 &
dw msg state | grep -E '^layers' -A1
```
Result: running, `[info] Bar configured (width: 1280, height: 30) for output: winit`, and driftwm's `state` lists `layers 1` / `waybar`. **TESTED**

**Errors logged in that run, and whether they matter:**

| Message | Matters? | Label |
|---|---|---|
| `dbind-WARNING … AT-SPI: Error retrieving accessibility bus address … org.a11y.Bus was not provided by any .service files` | No. Screen-reader support only. | TESTED that the bar works with it. Meaning: BELIEVED |
| `[info] Unable to receive desktop appearance: … org.freedesktop.portal.Desktop was not provided` | No. Only dark/light preference. | TESTED that the bar works with it |
| `basic_string::_M_create` (a bare line, no prefix) | Cause **UNKNOWN**. No visible effect: the bar drew correctly. | TESTED |
| `[warning] Unable to connect to the SYSTEM Bus!...` | Only for modules that need the system bus (Bluetooth, UPower, network manager). Not for the bar, clock or custom modules. | TESTED that the bar works; module list in section 8 |
| `[debug] Cmd exited with code 0` (every 2 seconds) | No. The custom module's script ran. | TESTED |

The packaged Waybar did **not** need systemd. There is no systemd on this machine (pid 1 is `process_api`, `docs/environment.md`).

---

## 2. Does the bar take screen space?

**Yes for driftwm's own "fit" actions. No for hand placement.**

Numbers from `driftwm msg state` on the same 1280×800 virtual screen, two terminal windows:

| Action | Without the bar | With the bar (30 px high, top) | Label |
|---|---|---|---|
| `dw msg focus term-a; dw msg action fit-window` | window `1280x800`, camera `0 -0` | window **`1280x770`**, camera **`0 15`** | TESTED |
| same on `term-b` (at `[25, -25]`) | not run | `1280x770`, camera `25 -10` | TESTED |
| `dw msg action toggle-fullscreen` on `term-a` | not run | `state` shows `fullscreen 1`; **the screenshot shows no bar** | TESTED |
| `dw msg move 0 150 --id 1` (top edge above the screen top) | not run | accepted: `0 150`; the window's top slides **under** the bar, and the bar is drawn **on top** of it (screenshot) | TESTED |
| `dw msg resize 1600 1000 --id 1` | not run | accepted: `1600 1000` | TESTED |

Screenshots looked at: with `fit-window` the window fills the area below the bar exactly (its title bar starts right under the bar, no gap, nothing hidden). In fullscreen the terminal fills the whole screen and the bar is gone. After the `move 0 150` the window's title bar is hidden behind the bar.

Commands: `grim $A/s2-fit.png`, `grim $A/s3-fs.png`, `grim $A/s4-move.png`.

What this means:

- driftwm treats the bar as an **exclusive zone** (the area the bar reserves). Maximize-like actions use the area below it. (Reading the code to confirm how: **not done**, so why is **BELIEVED**: layer-shell exclusive zone.)
- Windows are **not stopped** from going under the bar. `hubd` placing a window with a large `y` can slide it partly under the bar.
- **Fullscreen hides the bar.** Bar alerts would not be visible while any window is fullscreen. `HUB-OS.md` (Canvas section) says "maximize" means driftwm's fit-to-viewport, not fullscreen.

---

## 3. A custom module

**Yes, the text updates, and a CSS class changes its colour.**

```
echo "3 of 4 up" > /tmp/dwx/wb/state ; sleep 3.5 ; grim $A/s5-alert.png
/tmp/dwx/wb/hub.sh        # prints {"text":"3 of 4 up","class":"alert","tooltip":"hub state: 3 of 4 up\nupdated 22:02:00"}
```
**TESTED.** The module is a timer (`"interval": 2`); the log shows the script ran every 2 seconds.

Screenshots looked at: with the file saying `4 of 4 up` the module is a **green** box at the left end of the bar reading "4 of 4 up". After the change it is a **red** box reading "3 of 4 up" (`#custom-hub.alert`). The clock stays at the right end.

**Tooltip:** hovering the module shows a small box with the JSON `tooltip` text on two lines ("hub state: 2 of 4 up" / "updated 22:03:03"). **TESTED** (seen in the screenshot taken after `xdotool mousemove 40 15`).

**Pushed updates with no timer** (a feed file; the module runs `tail -n1 -F` and each new line is one update). Config line: `"exec": "tail -n1 -F /tmp/dwx/wb/feed", "return-type": "json"`, no `interval`:

```
echo '{"text":"4 of 4 up","class":"ok","tooltip":"all up"}' > /tmp/dwx/wb/feed     # before Waybar starts
echo '{"text":"1 of 4 up","class":"alert","tooltip":"3 down"}' >> /tmp/dwx/wb/feed ; sleep 0.5 ; grim $A/s27-feed-b.png
```
Result: half a second after the line was appended, the module read **red "1 of 4 up"**. **TESTED**

---

## 4. Clicks, and refreshing from outside

**A click runs a command: yes.**

```
xdotool mousemove 40 15 ; xdotool click 1 ; cat /tmp/dwx/wb/clicks.log      # → clicked 22:02:12
xdotool mousemove 1235 15 ; xdotool click 1      # on the clock: not bound, no new line
xdotool mousemove 600 15 ; xdotool click 1       # empty bar: no new line
```
**TESTED.** The terminal below the bar kept keyboard focus (`dw msg state` still marked it with `*`). `on-click-right`, `on-click-middle`, `on-scroll-up` and `on-scroll-down` are option names in the program (`strings -n 3 $(which waybar) | grep -E '^(on-click-right|on-click-middle|on-scroll-up|on-scroll-down|signal|exec-if|exec-on-event)$'`), not run. **BELIEVED**

**An outside program can refresh the module at once: yes, with the `signal` option.** Config `"signal": 8` and `"interval": 3600`:

```
rm -f /tmp/dwx/wb/runs.log ; … start Waybar …            # script ran once at start
sleep 6 ; wc -l < /tmp/dwx/wb/runs.log                    # still 1: the timer is far away
echo "2 of 4 up" > /tmp/dwx/wb/state ; pkill -RTMIN+8 waybar ; sleep 0.6 ; tail -1 /tmp/dwx/wb/runs.log
```
Result: signal sent `22:02:36.788`, script ran `22:02:36.805` (**17 ms later**), and the screenshot shows the red "2 of 4 up". **TESTED**

---

## 5. A dropdown in Waybar itself

**Waybar 0.9.24 (Ubuntu 24.04): no menu on a custom module.** A config with `"menu": "on-click"`, `"menu-file"` and `"menu-actions"` (a small GTK menu file, four entries) started without complaint, and a left click and a right click opened nothing. `strings -n 3 $(which waybar) | grep -E '^menu(-file|-actions)?$'` finds none of the three option names. **TESTED** (behaviour), **BELIEVED** (cause: the option does not exist in this version).

**Waybar 0.12.0 (Debian 13): yes, a menu opens, with limits.** Run inside a Debian 13 root directory built with `debootstrap` (Ubuntu's script, `--no-check-gpg`, test only), with `/tmp/dwx` bind-mounted into it so Waybar can reach driftwm's socket and the session bus:

```
DEBOOTSTRAP_DIR=$A/dbsroot/usr/share/debootstrap $A/dbsroot/usr/sbin/debootstrap --variant=minbase --no-check-gpg \
  --include=waybar,wofi,fonts-dejavu-core,dbus-daemon,librsvg2-common,adwaita-icon-theme,procps,jq trixie $A/trixie http://deb.debian.org/debian
mount -t proc proc $A/trixie/proc ; mkdir -p $A/trixie/tmp/dwx ; mount --bind /tmp/dwx $A/trixie/tmp/dwx
chroot $A/trixie /bin/sh -c 'apt-get install -y --no-install-recommends -f dbus-x11 libspdlog1.15-fmt10 ; dpkg --configure -a ; waybar --version'
```
Output: `Waybar v0.12.0`, no `systemd` package installed in it (`dpkg -l | grep -cE "^ii +(systemd|systemd-sysv|libpam-systemd) "` → `0`). debootstrap left the packages unconfigured (`waybar depends on libspdlog1.15-fmt10` missing); the second command fixed it. **TESTED**

The menu file is a GTK menu with greyed headings, separators and 30 entries (44 children); the actions are `echo <id> >> /tmp/dwx/wb/menu-picks.log` per entry. Generated by a short python script; its pattern per group:

```
<child><object class="GtkMenuItem" id="hdr-guests"><property name="label">Guests of vmhost-1</property><property name="sensitive">False</property></object></child>
<child><object class="GtkSeparatorMenuItem"/></child>
<child><object class="GtkMenuItem" id="guest-01"><property name="label">   guest-01   Scratch guest 1   UP</property></object></child>
```
and the module config `"menu": "on-click", "menu-file": "/tmp/dwx/wb/menu36.xml", "menu-actions": { "guest-01": "echo guest-01 >> …", … }`. Waybar run as:

```
chroot $A/trixie /usr/bin/env XDG_RUNTIME_DIR=/tmp/dwx WAYLAND_DISPLAY=wayland-1 DBUS_SESSION_BUS_ADDRESS=unix:path=/tmp/dwx/bus LC_ALL=C.UTF-8 waybar -c /tmp/dwx/wb/config-menu36.json -s /tmp/dwx/wb/style.css
```

| Question | Result | Label and command |
|---|---|---|
| Does it open on click? | **Yes.** `xdotool mousemove 40 15; xdotool click 1` → a white menu opens at the top left, starting at the top of the screen, about 325 px wide. | TESTED |
| Can it show group headings? | **Yes.** Headings are greyed, not clickable, followed by a separator line. Clicking a heading did nothing (no log line, menu stayed open). | TESTED: `xdotool mousemove 120 143; xdotool click 1` |
| Can an entry run a command? | **Yes.** Clicking `guest-10` wrote `guest-10` to `menu-picks.log` and closed the menu. | TESTED: `xdotool mousemove 150 673; xdotool click 1; cat menu-picks.log` |
| **Can it scroll with 30+ entries?** | **No, not in this test.** The menu is about 1,100 px tall on an 800 px screen. It starts at the top and runs off the bottom; entries from about `guest-15` down are off screen. Mouse wheel over the menu (`xdotool click 5` ×15 and ×8), hovering the bottom edge for 3 seconds, and the `Down` key (×25) did **not** move it. | TESTED. The `Down` keys landed in the terminal below the bar (it printed `^[[B` repeatedly): **keyboard input is not given to the menu**. Why GTK did not scroll or shrink the menu: UNKNOWN |
| Search? | No search in a GTK menu. | BELIEVED |
| Hover highlight? | Yes; the entry under the pointer is highlighted blue. | TESTED |

So 30 entries do not fit and there is no way to reach the lowest ones. About 28 rows fit on a 800 px screen at this font size (BELIEVED, from the 25 px row height seen in the screenshots). A screen with more pixels, smaller text, or groups folded into **submenus** might fit (submenu: BELIEVED GTK supports it; not run).

Waybar 0.12.0 also needs a session bus (section 1) and also shows the AT-SPI and portal messages.

---

## 6. A list menu instead

A list of 37 lines: 7 headings and 30 entries, grouped by role (Gaming, AI, Desktop, NAS, Backup NAS, VM host, Guests of vmhost-1 with 20 guests), made by:

```
python3 - <<'EOF'
groups=[("Gaming",[("gaming-1","Gaming Box","UP")]),("AI",[("ai-1","AI Box","UP"),("ai-2","AI Box 2","DOWN")]),
("Desktop",[("desktop-1","General Desktop","UP"),("desktop-2","Desktop 2","UP"),("desktop-3","Desktop 3","DOWN")]),
("NAS",[("nas-1","Storage","UP"),("nas-2","Storage 2","UP")]),("Backup NAS",[("backup-nas-1","Backup Storage","UP")]),
("VM host",[("vmhost-1","VM Host","UP")]),("Guests of vmhost-1",[("guest-%02d"%i,"Scratch guest %d"%i,"UP" if i%4 else "DOWN") for i in range(1,21)])]
lines=[]
for g,items in groups:
    lines.append("── %s ──"%g)
    for i,n,s in items: lines.append("   %-14s %-18s %s"%(i,n,s))
open("list.txt","w").write("\n".join(lines)+"\n")
EOF
sed 's/── \(.*\) ──/== \1 ==/' list.txt > list-ascii.txt        # the same list with ASCII headings
```

Each launcher was started with the list on its standard input and its output in a file. Then `grim`, twenty `xdotool key Down`, `grim`, `xdotool type "nas-2"`, `grim`, `xdotool key Return`, and the file read. All used `export LC_ALL=C.UTF-8` unless noted.

| | **fuzzel 1.9.2** | **wofi 1.4.1** | **tofi 0.9.1** | **bemenu 0.6.15** |
|---|---|---|---|---|
| Command | `fuzzel --dmenu --lines 12 --width 40 --prompt "hub> "` | `wofi --dmenu --lines 12 --width 480 --location top_left --yoffset 30 --prompt hub --insensitive` | `tofi --anchor=top-left --margin-top=30 --width=480 --height=380 --prompt-text="hub> " --num-results=12` | `bemenu -l 12 -p "hub>" --ignorecase` (with `BEMENU_BACKEND`/`BEMENU_RENDERERS`, section 0) |
| Opens (a layer-shell surface in `dw msg state`) | Yes (`layers 2`, `launcher`) | Yes (`wofi`) | Yes (`launcher`) | Yes (`menu`) |
| Scrolls (`Down` ×20) | **Yes**, 12 rows visible, moved down to `guest-14` | **Yes**, with a **scrollbar**, moved to `guest-04` in view | **Yes** | **Yes**, showed `guest-01`…`guest-07` |
| Search (typed `nas`/`nas-2`) | **Yes**, filtered live, matched text coloured | **Yes** | **Yes** | **Yes** |
| Pick prints the line | **Yes.** Typing `nas` then Enter printed `   backup-nas-1   Backup Storage     UP`: the highlight stayed on the old row (the third), so the **filtered list did not reset the selection to the first match** | **Yes.** `nas-2` + Enter printed `   nas-2          Storage 2          UP` | **Yes** (`nas-2`) | **Yes** (`nas-2`) |
| A heading line picked | **Prints the heading** (`== Gaming ==`, ASCII list; `Up` then Enter) | **Prints the heading** (`── Gaming ──`, with the cache turned off) | not run | not run |
| Closes after a pick | Yes (gone within a second) | Yes | Yes | Yes |
| Placed near the bar? | **No.** Always centred. `--anchor` is rejected (`error: --anchor: invalid option`) and `--help` lists no position option. | **Yes.** `top_left` + `--yoffset 30` put it at y = 60: the bar's reserved 30 px **plus** the offset. `--yoffset 0` sits right under the bar. | **Yes.** Top left, just under the bar (the box starts at about y = 33) | **At the very top, full width, over the bar** (the first row is at y = 0) |
| Other | It uses a pale cream theme | Default look is white, small | Default font is large (24 px) and a narrow `--width` cuts text | Dark strip, text small |

All four: **TESTED** (opening, scrolling, search, pick printing). The headings column: only fuzzel and wofi were run; the other two are **BELIEVED** to behave the same, since they print the chosen line.

**Things that go wrong, all found by running:**

1. **Non-ASCII headings vanish under the plain "C" locale.** With `LANG=C`, `fuzzel --dmenu` showed the 30 entries and **none** of the 7 `── Group ──` lines. With `LC_ALL=C.UTF-8` they appeared. With ASCII `== Group ==` lines they appear in both. **TESTED:** `env LANG=C fuzzel --dmenu … < list.txt` (no headings) against `env LC_ALL=C.UTF-8 fuzzel --dmenu … < list.txt` (headings). The cause is **BELIEVED** to be fuzzel dropping lines it cannot convert.
2. **`wofi` re-orders the list by what was picked before.** After one earlier pick of `nas-2`, the next run showed `nas-2` as the **first** row, above the headings. `ls ~/.cache | grep wofi` showed `wofi-dmenu`. With `--cache-file /dev/null` the first row was `── Gaming ──`, the original order. **TESTED**
3. **A heading can be picked.** The launcher does not grey it or skip it; whatever receives the printed line must ignore lines it knows are headings. **TESTED** (fuzzel, wofi).
4. **The launcher takes the keyboard**, so typing goes to it, not to the terminal. In an early run `fuzzel` failed to start (the `--anchor` mistake) and 20 `Down` keys and `nas` landed in the terminal instead. **TESTED**
5. **A mouse click on an entry** was not tested. **UNKNOWN**

**Started from the bar, end to end (`wofi`).** The module's `on-click` runs a script that guards against a second copy and logs the pick:

```
# menu.sh
export LC_ALL=C.UTF-8
pgrep -x wofi >/dev/null && exit 0
sel=$(wofi --dmenu --lines 12 --width 480 --location top_left --yoffset 0 --prompt "hub" --insensitive < /tmp/dwx/wb/list.txt)
echo "picked: [$sel] at $(date +%T)" >> /tmp/dwx/wb/menu-picks.log
```
Config `"on-click": "/tmp/dwx/wb/menu.sh"`. Result: a click on the module opened the wofi list right under the bar; a second click while it was open started no second copy (`pgrep -c -x wofi` stayed `1`); typing `guest-12` and Enter wrote `picked: [   guest-12       Scratch guest 12   DOWN] at 22:06:59`. **TESTED**

`rofi` is in the Ubuntu 24.04 archive as 1.7.5 (version only checked with `apt-cache policy rofi`); it is an X11 program, not tried. **BELIEVED** (not Wayland-native in this version).

---

## 7. Does the bar change the canvas coordinates?

**Window coordinates: no. Camera and default placement: yes, by half the bar height.**

| Check | Without the bar | With the bar | Label |
|---|---|---|---|
| `term-a` moved to canvas `[0, 0]`, camera `0 0`: its position in `dw msg state` | `[0, 0]` | `[0, 0]` | TESTED |
| Where that window sits on the screen. In the screenshots, at screen column x = 640 the window's title bar starts at row **138** and its body ends at row **662** | rows 138…662 | rows 138…662 | TESTED: `grim $A/m-with.png`, `grim $A/m-without.png`, then `python3 $A/pngbox.py` (a small PNG reader that lists colour runs down one column) |
| A third window opened at camera `0 0` (default placement) | `[50, -50]` | `[0, -15]` | TESTED: `foot --app-id=term-c … &` then `dw msg state` |
| A window rule `position = [300, -100]`, `size = [400, 300]` for `term-d` | window `[300, -100] 400x300`, camera `300 -100` | window `[300, -100] 400x300`, camera **`300 -85`** | TESTED: `dw --config /tmp/dwx/rule.toml …` with that rule, then `foot --app-id=term-d …` |
| `fit-window` | camera `0 -0` | camera `0 15` | TESTED (section 2) |

Meaning for `hubd`:

- A window placed with `dw msg move X Y --id N`, or with a window rule `position`, lands on the **same canvas coordinates** and the **same screen spot** with or without a bar. The "fixed home position" of each machine is therefore not shifted by the bar.
- What changes is where driftwm **centres the camera** and where it puts a new window when nothing says otherwise: the centre of the **usable area**, which is half the bar height away from the screen centre (15 px for a 30 px bar). Anything that reads `camera` should expect that offset.
- Which edge the bar is on (top, bottom, left, right) was only tested at the top. A bar at another edge is **BELIEVED** to shift by the same amount in the matching direction. **UNKNOWN**

---

## 8. What the bar depends on: systemd, D-Bus, a tray, PipeWire, a battery

**Systemd:** nothing needed. **D-Bus session bus:** needed to start (section 1). Waybar never needed the **system** bus for the cases below, but warns about it.

Each module run alone in a bar with the clock (`waybar -c one.json …` with `"modules-left": ["<module>"]`, three and a half seconds each). `RUNNING` means the program was still alive afterwards:

```
for m in wlr/taskbar cpu memory disk temperature network battery backlight idle_inhibitor pulseaudio wireplumber upower mpris bluetooth tray; do
  printf '{"layer":"top","position":"top","height":30,"modules-left":["%s"],"modules-right":["clock"],"clock":{"format":"{:%%H:%%M:%%S}","interval":1}}' "$m" > one.json
  waybar -c one.json -s style.css -l info > one.log 2>&1 & sleep 3.5 ; pgrep -x waybar ; pkill -x waybar
done
```

| Module | After 3.5 s | What it logged | Needs | Label |
|---|---|---|---|---|
| `custom/*`, `clock` | RUNNING | nothing relevant | nothing | TESTED |
| `wlr/taskbar` | RUNNING | `Requested height: 30 is less than the minimum height: 34 required by the modules` (use height 34) | driftwm's foreign-toplevel protocol | TESTED |
| `cpu`, `memory`, `disk`, `idle_inhibitor`, `pulseaudio`, `tray` | RUNNING | only the system-bus warning | `/proc`, `/sys`; for `tray` and `pulseaudio` real apps/servers to show anything | TESTED that they run; their output was not looked at |
| `temperature` | RUNNING | `Disabling module "temperature", Can't open /sys/class/thermal/thermal_zone0/temp` | a thermal sensor | TESTED |
| `battery` | RUNNING | `No batteries.` | a battery | TESTED |
| `backlight` | RUNNING | `Disabling module "backlight", No backlight found` | a backlight | TESTED |
| `network` | RUNNING | `[error] Can't open RFKILL control device`, `Can't resolve nl80211 interface` | netlink / Wi-Fi hardware | TESTED |
| `upower` | RUNNING | `Disabling module "upower", Unable to create UPower client!` | the UPower service (system bus) | TESTED |
| `mpris` | RUNNING | `[error] mpris[playerctld]: GDBus.Error:… ServiceUnknown` | a media player on the session bus | TESTED |
| `wireplumber` | **EXITED** | `[E] pw.loop … can't make support.system handle: No such file or directory` | PipeWire's plug-in folder. This may be this unpacked set-up (no `SPA_PLUGIN_DIR`), not the module; **not retried with it set** | TESTED (exit); cause UNKNOWN |
| `bluetooth` | **EXITED** | `Can't open RFKILL control device`, `g_dbus_object_manager_client_new_for_bus_sync() failed: Could not connect` | the **system** D-Bus and BlueZ | TESTED |

A bar with **all** of those listed in one config exited as soon as it started, because of `wireplumber` and `bluetooth`. **TESTED** (config `config-all.json`).

**`wlr/taskbar` works with driftwm**: `dw msg state` and the bar both listed `Second` and `First`; clicking `Second` in the bar focused that window and moved the camera to it (camera `300 -100` → `0 16.5`). **TESTED:** `xdotool mousemove 36 16; xdotool click 1`.

For a hub that shows only a custom alert module, a clock and a launcher, nothing in the "Needs" column above is required beyond the session bus. **BELIEVED**, from the custom-module and clock runs.

**Battery:** none in the test machine; the module disables itself and the bar goes on. **TESTED.**

---

## 9. What could not be checked here, and what each needs

| Not checked | What it needs |
|---|---|
| Anything on a real screen: bar height and text size on the projector, bar readability from the seat, tooltip placement | The projector, or a display of the same pixel size |
| A bar at the bottom or at a side; two bars; a bar on a second monitor | A config with `"position": "bottom"` etc., and for the monitor a second output |
| Waybar's menu with groups folded into **submenus** to fit 30+ machines, and why the long menu did not scroll | Further runs on Waybar 0.12.0 (and a look at how driftwm places popup menus: `src/handlers/xdg_shell.rs`, not read) |
| Waybar newer than 0.12.0 | A newer userland; Debian 13 was the newest available with the tools used |
| Clicking an entry in `wofi`, `tofi`, `fuzzel` or `bemenu` with the mouse; `tofi`/`bemenu` heading picks | More runs with `xdotool` |
| Whether the `wlr/taskbar` list is useful with many windows | A run with 30+ windows |
| Keeping the launcher open on a particular output; behaviour with two monitors | Hardware or a second virtual output |
| `wireplumber` and PipeWire in the bar on the real image | The Hub OS image, with PipeWire running, and `SPA_PLUGIN_DIR` set if needed |
| `tray` with real tray programs | Programs that put icons in a tray |
| Waybar as a **non-root user**, and its start under an init such as `s6` or `dinit` | A VM with the real service set-up |
| Whether Waybar and the session bus can be started by a small service script on Hub OS | Writing and running that script |
| Waybar on `musl`, other architectures | A different toolchain |
| A release build of driftwm with the bar (the runs used the debug build) | `cargo build --release` on the target hardware |
| The `rofi` list menu | Either `rofi` with Wayland support, or Xwayland support (`xwayland-satellite`, not installed) |

---

## 10. Questions for the owner

1. Which of the dropdown options in the summary, if any, should be worked out further? The alert part is the same in all of them.
2. Is a D-Bus session bus acceptable on the Hub OS image? Waybar will not start without one. `HUB-OS.md` bans systemd, not D-Bus.
3. How many machines must the dropdown show at once? A Waybar 0.12 menu fits about 28 rows on an 800 px screen; the list launchers scroll.
4. Is it acceptable that fullscreen hides the bar and its alert? The plan uses "maximize" (driftwm's fit) for ordinary windows; game-style windows may be fullscreen.
5. Should the bar be at the top, as tested, or at another edge?
6. Which list launcher, if any, is preferred? `fuzzel` 1.9.2 cannot sit next to the bar, `wofi` and `tofi` can.
7. May a newer Waybar (0.12.0 or later) be built for Hub OS instead of the 0.9.24 in the Ubuntu archive? That decides whether option B exists at all.
