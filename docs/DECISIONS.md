# Decisions

Owner decisions made after `HUB-OS.md` was written.
**Where this file conflicts with `HUB-OS.md`, this file wins.**

## Decisions

### Windows and sessions
- Each machine opens as its **whole desktop in one window**, not as individual apps.
- Closing a window leaves that machine's session alive. Clicking the machine again returns to it.
- Clicking a machine that is already open goes to its existing window. Never open a duplicate.
- If a machine drops while its window is open, leave the window alone. Only the alert changes.
- Every machine stays logged in. The network is assumed to be a trusted private network.

### Panel
- The panel is an always-visible **bar item** with a scrollable dropdown, grouped by role.
- Guests are nested under their host.
- Which bar to use (Waybar or something else) is decided later.
- When any machine is down, the bar item itself shows an alert (for example "3 of 4 up" in red).
- A machine that is powered off just shows as down. No wake-on-LAN.

### Inventory
- In v1, machines are added by hand-editing the inventory file.

### Desktop
- Copy-paste works across all windows.
- All sound plays through the hub.
- The hub has two or more monitors.
- Each machine has a fixed home position on the driftwm canvas.
- Windows stay on the canvas. They can be maximized (not fullscreen) and moved to their own monitor.

### Gaming PCs — REVERSAL
- Gaming PCs are normal machines, streamed to the hub with Sunshine/Moonlight, the same way as the AI box.
- Competitive play still uses each gaming PC's own monitor over direct HDMI.
- The old "do not stream gaming PCs" cuts in `HUB-OS.md` no longer apply.

### Experiments
- OS experiments run on one dedicated VM host. Its guests are listed under it.

### Power actions
- Each machine gets Restart and Shut down actions, with a confirmation that names what will be lost.
- This comes **after** v1's first slice.

### Out of scope as design factors
- Electricity and power cost are not design factors.
- Erlang/Elixir is parked. Stay with Go and systemd.

## Where this overrides HUB-OS.md

| HUB-OS.md says | Now |
|---|---|
| "Not a remote-play path for the gaming PCs"; cut "Streaming gaming PCs for play"; gaming role "No stream for play"; gaming entries `open: none` | Gaming PCs are streamed with Sunshine/Moonlight like the AI box. Competitive play still on direct HDMI. |
| "A small panel of tiles", "a tile per node" | An always-visible bar item with a dropdown grouped by role; guests nested under their host. |
| "Must not spawn duplicate viewers on a second click **without asking**" | Never open duplicates. A second click goes to the existing window. No asking. |
| "Ending a session from the panel should close the local viewer" | Closing a window must leave the machine's session alive, so clicking again returns to it. (Whether Sunshine actually keeps the session alive is still unverified — see below.) |

## Open / unverified — do not assume these work

- How Hub OS jumps to, or places, a specific window in driftwm. How maximize and multi-monitor behave in driftwm.
- How Moonlight and driftwm share keyboard shortcuts. (Parked.)
- Whether Sunshine sessions stay alive after the viewer (Moonlight) closes.
- Whether the NAS is opened on click or kept permanently mounted. (Undecided.)
