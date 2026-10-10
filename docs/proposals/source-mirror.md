# PROPOSAL: the source mirror on the NAS (every build input, checked by hash)

**Status: PROPOSAL. Nothing here is decided, built or tested.** `HUB-OS.md` wins if this file disagrees with it. Written 2026-10-10 by the lead agent (no helpers). No code. Labels: TESTED, SOURCE, BELIEVED, UNKNOWN. **Names, paths and the command spelling are placeholders.**

## 1. Purpose
`HUB-OS.md` says master copies of the code and every built image live on the NAS and builds will one day happen inside the cluster. Builds today fetch from the internet: an Ubuntu snapshot (`snapshot.ubuntu.com`), kernel tarballs, source tarballs, crates and Go modules (SOURCE `docs/proposals/distro-workshop.md` 2 and 6.6). The mirror holds **every input** so a build never depends on upstream being up, unchanged or honest.

## 2. What exists and what disagrees
- The workshop proposal's **lock** already records, per package, version and sha256, and per source the commit or tarball hash (SOURCE `distro-workshop.md` 6.6); this proposal supplies the place the locked files come from. No mirror, no fetch tool and no store exist in the repo (SOURCE: searched `tools/`, `cmd/`, `docs/`).
- The reproducible build test (SOURCE `docs/image.md`, T09; TESTED again 2026-10-10: PASS) shows builds can be identical when inputs are identical; today's inputs come from the network.
- **Environment note (a disagreement):** `docs/environment.md` section on outbound access records HTTP 200 from the Go proxy, kernel.org, Alpine, Debian and Ubuntu mirrors and others on 2026-10-01. The request says this environment cannot reach most upstream sites. In this session (2026-10-10) downloads from `skarnet.org`, `distfiles.alpinelinux.org`, `deb.debian.org` and `archive.ubuntu.com` did work (TESTED). So fetches from the build environment are **UNKNOWN for other hosts**, and the tests below deliberately use a local HTTP server.

## 3. The layout and the entry file
```
mirror/
  inputs/NAME/VERSION/FILE                 # the downloaded file (placeholder layout)
  inputs/NAME/VERSION/FILE.meta.toml       # placeholder example below
```
```toml
name         = "example-source"
version      = "1.2.3"
upstream_url = "https://example.invalid/example-source-1.2.3.tar.xz"   # placeholder
sha256       = "0000…(64 hex)…"
signature    = "FILE.asc"       # optional: upstream signature file stored beside it
signature_key = "…"             # optional: key id used to check it
fetched_at   = "2026-10-10T12:00:00Z"
```
Inputs covered: source tarballs, toolchains, firmware, crates, the kernel, Ubuntu packages (and the snapshot index they come from).

## 4. Commands (placeholder names)
- `mirror fetch NAME VERSION URL SHA256` downloads to a **temporary file in the same folder**, checks the hash, writes the `.meta.toml`, then **renames** into place. The hash is given by the caller (from the lock or the owner), never taken from the download.
- `mirror verify [NAME…]` re-hashes the stored files and compares with `.meta.toml`.
- `mirror status` lists entries: ok, corrupt, missing, never verified, age.

## 5. Rules
1. **Builds never fetch upstream.** A missing input fails the build and names the input.
2. **A hash mismatch** (at fetch or at verify) fails: the entry is marked **corrupt** (a marker file beside it), the file is **not** deleted and **not** re-downloaded automatically (a person decides: question 3).
3. **Upstream signatures** are checked where they exist (defence in depth); a failed signature check refuses the entry; a missing signature is recorded as "no signature upstream".
4. **The lock file of every image records the sha256 of every input used** (SOURCE `distro-workshop.md` 6.6 plus `module-manifest.md`). A build whose lock names a hash the mirror does not have fails.
5. **Weekly verification job:** the images have **no cron** (the repo uses s6; SOURCE `docs/proposals/hub-stability.md` and the image). So the job is a workshop job (`distro-workshop.md` 5.1, "Job runner") or a small service that sleeps a week between runs. It reports corruption (delivery: question 4).
6. **Retention: forever.** Nothing is deleted by the tool.
7. **Concurrency:** downloads go to a temporary file and are renamed (atomic on one filesystem); two fetches of the same file take a per-file lock so the second waits or finds it done; two different files proceed at once.

## 6. Failure table
| What happens | Result |
|---|---|
| Hash mismatch on fetch | Temp file removed; nothing placed; error names input, expected, got |
| Stored file corrupt at verify | Marked corrupt; listed by `status`; builds using it fail by name |
| Build needs an input not in the mirror | Build fails, names it; no download |
| Upstream gone | Irrelevant to builds; matters only for new inputs |
| Two fetches of the same file at once | One downloads, the other waits and reports "already present" |
| Disk full during fetch | Temp file removed; nothing placed; error |
| Network down during fetch | Error; temp file removed |
| `.meta.toml` missing or malformed | Entry treated as missing |
| Signature present but fails | Entry refused, never placed |

## 7. Tests (with an ordinary folder and a local HTTP server; none written yet)
1. `fetch` a file: placed, `.meta.toml` correct.
2. `fetch` it again: reports present, nothing changes.
3. Corrupt a stored byte, then `verify`: marked corrupt.
4. Corrupt, then a build: fails naming the input, nothing downloaded (the server log shows no request).
5. Two different files fetched at once: both placed.
6. The same file fetched twice at once: one file, no partial file, no error.
7. A clean `verify`: all ok, exit 0.
8. A missing input at build time: fails by name.
Real fetches from the internet remain **UNKNOWN** until tested elsewhere.

## 8. Five assumptions I am least sure of
1. That one flat folder tree on the NAS filesystem (OpenZFS, SOURCE `HUB-OS.md`) is enough; snapshots protect it (BELIEVED).
2. That upstream signatures exist for most inputs (UNKNOWN: many do not).
3. That "retention forever" fits the NAS size (UNKNOWN; sizes of kernel, toolchains and firmware not measured).
4. That the weekly job can run where the images have no cron (BELIEVED).
5. That the lock plus the mirror is enough to rebuild an image years later (UNKNOWN; toolchains must be in the mirror too).

## 9. Decided by the owner (2026-10-10)

- **Builds never fetch upstream** (no fallback fetching).
- **Retention is forever.**
- **Corruption alerts** go to a toast and to the workshop view. Neither exists yet, so for now the tool writes a **status file** and **exits non-zero**.
- The **off-site mechanism waits** for the off-site backup design.

## 10. Questions for the owner (still open)

1. Who clears a corrupt entry (retention is forever, so the entry stays until a person decides)?
