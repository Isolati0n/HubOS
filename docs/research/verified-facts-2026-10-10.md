# Verified facts, 2026-10-10

Facts as given by the owner, each with its source and label. Nothing else is added here.

## SOURCE (page checked 2026-10-10)

- **s6-supervise:** minimum 1-second delay between two ./run spawns (https://skarnet.org/software/s6/s6-supervise.html). Whether it is configurable: not stated there.
- **s6-rc:** its overview page (https://skarnet.org/software/s6-rc/overview.html) and command page (https://skarnet.org/software/s6-rc/s6-rc.html) do not say what happens to dependents when a dependency dies on its own. A failed transition stops the transitions that depend on it. **UNKNOWN:** restart of dependents after a crash; the init-comparison test is the only evidence.
- **runit runsv:** if ./run or ./finish exits immediately, it waits one second before restarting (https://smarden.org/runit/runsv.8.html).
- **dinit:** restart-delay default 0.2 s; restart-limit-interval default 10 s; restart-limit-count default 3 (https://davmac.org/projects/dinit/man-pages-html/dinit-service.5.html). After 3 restarts in 10 s it stops restarting.
- **OpenRC supervise-daemon:** respawn immediately by default; respawn_max default 10; respawn_period unset (https://github.com/OpenRC/openrc/blob/master/supervise-daemon-guide.md).
- **Erlang/OTP socket module** (since OTP 22.0) has a 'rights' control-message type for sending and receiving, but the page does not name SCM_RIGHTS and does not say whether domain local with type seqpacket works (https://erlang.org/doc/apps/kernel/socket.html). **UNKNOWN** until tested.
- **Go:** "unixpacket" is a documented network name for net.Dial; UnixConn has ReadMsgUnix and WriteMsgUnix (https://pkg.go.dev/net). **BELIEVED**, not seen on that page: Listen accepts "unixpacket", and UnixRights is in the syscall package.
- **memfd_create:** F_SEAL_WRITE requires unmapping any writable shared mapping first; F_SEAL_FUTURE_WRITE blocks new writes while existing writable mappings remain (https://man7.org/linux/man-pages/man2/memfd_create.2.html). The page's SIGBUS warning is about an untrusted peer shrinking the region, which F_SEAL_SHRINK prevents.
- **RFC 6143:** incremental = 0 means the server sends the whole area; non-zero means it sends only when something changes; the server must not send unsolicited updates (section 7.5.3). Raw encoding sends pixels in scan-line order in the negotiated pixel format (section 7.7.1) (https://www.rfc-editor.org/rfc/rfc6143).
- **efivarfs:** variables that are not well-known standard ones are created immutable; the flag must be cleared to delete them; deleting non-standard variables is the risky case; no write limit is stated (https://docs.kernel.org/filesystems/efivarfs.html). **UNKNOWN:** real-firmware behaviour (QEMU's firmware does not show it).
- **PSI:** per-cgroup pressure tracking needs CONFIG_CGROUPS=y and cgroup2 mounted (https://docs.kernel.org/accounting/psi.html). **BELIEVED:** CONFIG_PSI is the option and the system-wide /proc/pressure files do not need cgroups.
- **Smithay** (https://docs.rs/smithay/latest/smithay/wayland/) has modules for keyboard_shortcuts_inhibit, pointer_constraints, relative_pointer, viewporter, fractional_scale, idle_inhibit, text_input, selection, xdg_activation, cursor_shape, security_context and others. Not confirmed there: xdg decoration.
- **RDNA4 (Radeon RX 9070 series):** the Phoronix launch review (https://www.phoronix.com/review/amd-radeon-rx9070-linux, March 2025) recommends "Linux 6.12 LTS and newer", better 6.13, and Mesa 25.0 or newer. That is older evidence; test on the real card in December.

## NOT VERIFIED (do not treat as true)

- A CVE number for an amdgpu MES ring bug given by DeepSeek.
- DeepSeek's claim that Remmina forces the X11 backend (an Arch bug titled "remmina clipboard sync is broken on Wayland" exists, FS#76249, details unread).
- Any NIC driver support claims.
