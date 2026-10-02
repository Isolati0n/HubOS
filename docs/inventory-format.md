# Inventory file format

**Status:** approved by the owner, 2026-10-01. `HUB-OS.md` wins if anything here disagrees with it.

The inventory is the list of machines `hubd` knows about. It is one plain-text file. It is **not secret**, and secrets never go in it.

The real inventory lives on the hub's own disk, in its per-machine config outside the system image. The NAS holds a backup copy. It is never committed to this repo. The repo holds only a fake example: `examples/inventory.example.toml`.

---

## The file

- **Name:** `inventory.toml`
- **Type:** TOML (simple `key = value` text, comments allowed with `#`).
- **First line that matters:** `format = 1`. This is the version of the layout. If the layout changes later, the number goes up so `hubd` can tell old files from new ones.
- **Machines:** one block per machine, each starting with `[[machine]]`. All blocks sit in one flat list. Guests are not written inside their host's block; they point to it by id. (The panel still shows guests nested under their host.)

## Fields

| Field | Required? | Meaning |
|---|---|---|
| `id` | required | Short permanent label. Lowercase letters, digits and dashes only, starting with a letter or digit (`^[a-z0-9][a-z0-9-]*$`). Unique. Never changed once set. Other entries refer to a machine by its id. |
| `name` | required | Friendly name shown in the panel. |
| `role` | required | Exactly one of: `hub`, `gaming`, `ai`, `desktop`, `nas`, `backup-nas`, `vm-host`, `guest`. |
| `address` | required | Where to reach the machine: a name (like `nas-1.lan`) or a number address. No port in it. |
| `open` | required | A list of one or more of: `moonlight`, `spice`, `vnc`, `ssh`, `files`, `none`. The **first** entry is what a click opens. The rest are extra options. |
| `home` | required | The machine's fixed spot on the canvas, written `{ x = ..., y = ... }`. |
| `port` | optional | For programs that need a port other than their default. |
| `user` | optional | Login name. Never a password or key. |
| `share` | optional | The shared folder a `files` opening should use. |
| `host` | guests only | The id of the VM host this guest runs on. |
| `lifetime` | guests only | `ephemeral` (resets to a clean copy) or `persistent` (keeps its changes). |

## Rules `hubd` must enforce when it reads the file

- The `format` line must be present, and its value must be one that `hubd` supports (currently only `1`). Otherwise `hubd` refuses to run and names the version it found and the versions it supports.
- `id` must be unique and use only lowercase letters, digits and dashes.
- `open` must not be empty.
- `none` is allowed only as the sole entry in `open` (for example `["none"]`, never `["none", "ssh"]`).
- `share` is allowed only when `open` includes `files`, and is an error otherwise.
- `host` and `lifetime` are required on guests, and refused on every other role.
- `host` must point at a machine whose role is `vm-host`.
- Two machines may not share the same `home` position.
- Exactly one hub is required: exactly one machine must have the role `hub`. A file with no hub, or with more than one, is an error.
- An unknown field is an error. `hubd` stops and names the field.
- `port`, when present, must be a whole number from 1 to 65535.
- The `x` and `y` of `home` must be whole numbers. This is provisional until the canvas unit is verified.
- `lifetime` must be exactly `ephemeral` or `persistent`.
- `id` must start with a letter or digit, not a dash. `address`, `user` and `share` must not start with a dash (a text that starts with a dash could be read as an option by the program it is given to).
- `id`, `name`, `user`, `share` and `address` must not contain control characters or line breaks (tabs, newlines, and the like). hubd puts these texts in menus, logs and viewer command lines, one argument each; a line break would break a menu line.
- Secrets never appear in this file.

## Unverified

- **`home` convention (provisional).** `home` uses driftwm's own convention: the **centre of the window**, with **Y pointing up**. This is read from driftwm's documents and checked only in a nested test window, not on a real screen. It stays provisional until tested on a real screen.
- **Canvas unit size.** What one unit of `home` means (pixels or something else) is unknown until driftwm is tested. The numbers in the example are placeholders.
- **Guest address and port.** A guest's console is likely reached through the VM host's address plus a port. So what `address` and `port` mean for a guest stays provisional until the VM host is designed.

## Decisions recorded

- Go's standard library does not read TOML, so a third-party TOML library will be needed. The owner approves using one. When coding starts, the bot proposes which library and asks the owner first.
- Secrets handling is a separate, later step. It is not part of this format.
- The real inventory lives on the hub's own disk, in its per-machine config outside the system image. The NAS holds a backup copy. It is never committed to this repo. The example uses fake names and `192.0.2.x` addresses (a range reserved for documentation) only.
