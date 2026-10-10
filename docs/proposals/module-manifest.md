# PROPOSAL: the module manifest (image modules, composition, lock file, preflight order)

**Status: PROPOSAL. Nothing here is decided, built or tested.** `HUB-OS.md` wins if this file disagrees with it. Written 2026-10-10 by the lead agent (no helpers). No code. Labels: TESTED, SOURCE, BELIEVED, UNKNOWN. **All module names, versions, commands and paths below are placeholders** (`example-*`); no real compositor, bar or package is a module name here.

## 1. Purpose
Today a machine's image is described by shell-readable files (`image/machines/NAME.build`) plus package lists and `run` scripts (SOURCE `docs/proposals/distro-workshop.md` section 2 and 6.4). The workshop proposal already proposes one **declarative machine file** with `[[package]]`, `[[source]]`, `[services]`, `[config]`, `[health]` and a **lock** (SOURCE same file, 6.2 and 6.6). This proposal adds one layer in between: a **module** is a directory that carries everything one feature needs (its packages, its service, its config, its health check, its tests), so a machine file lists modules, not loose packages and services, and every feature can be tested alone before it is combined.

## 2. Where it disagrees with the existing documents
- `distro-workshop.md` 6.2: the machine file lists `[[package]]`, `[services]` and `[config]` itself. Here those come from modules; the machine file keeps `[machine]`, `[hardware]`, `[kernel]`, `[base]`, `[update]` and `[preflight]` and gains a `modules = [...]` list. The workshop file's `[health]` ("hubd list five times") would move into the hub's modules.
- `distro-workshop.md` 6.6: the lock holds package versions and hashes, the kernel tarball hash, source hashes and tool hashes. Here it also holds modules, static tool hashes, firmware, the machine file hash and the dependency graph (section 6). Compatible: a superset.
- `distro-workshop.md` 7.1 runs fixed steps P0 to P7. Here P5 (boot in a VM and run checks) runs **module tests in dependency order**. Compatible, but P5 changes from "the distro-contract checks V2 to V10" to "V2 to V10 plus module tests" (question 1).
- The hub today sets the confirm health check in `image/rootfs/etc/s6/sv/confirm/run` (SOURCE `docs/STATE.md` section 4: five `hubd list` calls). A module's `[health]` would replace that script one day; not decided.
- `docs/STATE.md` 4 item 8: the update tool is to be rewritten in Go as one static program. "Static shared tools" below follows that.

## 3. A module (example, placeholders)
```toml
# modules/example-bar/module.toml
module  = "example-bar"
version = "1.2.0"                      # placeholder

[packages]
required = ["example-bar-pkg", "libexample1"]
optional = ["example-bar-extras"]      # installed if present in the mirror, never required

[service]
directory = "services/example-bar"     # an s6-style service directory (placeholder: the init is not decided)
name      = "example-bar"

[config]
files = ["config/example-bar.conf"]    # placed on the config partition or in the root; mode set in the file list

[health]
check             = "/usr/libexec/example-bar-health"   # a command, exit 0 = healthy
interval_s        = 5
timeout_s         = 2
failure_threshold = 3                  # failures in a row before "unhealthy"

[tests]
unit        = ["tests/unit.sh"]        # run on the build host, no VM
integration = ["tests/integration.sh"] # run inside the VM, after the module's dependencies are up
preflight   = ["tests/preflight.sh"]   # run by the preflight only

[dependencies]
modules  = ["example-compositor"]
packages = ["libexample-dev-runtime"]

[conflicts]
modules  = ["example-bar-alt"]
```
Machine file addition:
```toml
modules = ["example-compositor", "example-bar", "example-hubd"]
```
The compositor of the hub is Smithay-based (`HUB-OS.md`: driftwm); the names above are not it.

## 4. Composition (rules)
1. **Pure function:** `compose(machine file, module registry) -> (resolved module set, lock file)`. Same inputs, same outputs, byte for byte. No network, no clock, no environment.
2. **No runtime conditional includes:** a module is in the set or not. No "if hardware X" inside a module. A machine that needs a variant lists a different module (for example `example-bar-small`).
3. **Order** of the resolved set is a topological sort of the dependency graph, ties broken by module name (so it is reproducible).
4. The service list handed to the init is generated from the resolved set (SOURCE `distro-workshop.md` 6.2: `[services]` exists today as names in a `SERVICES` file).

## 5. Build-time checks (all must pass before an image is built)
1. Module names unique across the registry.
2. Every dependency present (module or package).
3. No conflict present in the resolved set.
4. Every package exists in the mirror/snapshot (SOURCE `distro-workshop.md` 6.5 rule 5 and 6.6).
5. The service directory can be created in the image (path legal, run file present, executable).
6. Every test binary/script is present and executable.
7. No dependency cycle.
8. The existing machine-file rules still hold: not a systemd base, `watchdog_timeout > confirm_timeout + 15 s`, no secret in any file (SOURCE `distro-workshop.md` 6.5). A module that brings in a banned package fails check 8.

## 6. The lock file
Written by the compose step, next to the machine file, reviewed by the owner like `lock.toml` in the workshop proposal. Contents: modules and versions; packages with versions and sha256; static tool hashes; kernel version and hash; firmware version and hash; the machine file hash; the dependency graph (edges as module -> module). Example keys (placeholder): `[[module]] name=… version=…`, `[[package]] name=… version=… sha256=…`, `[tool.node-helper] sha256=…`, `[kernel] version=… sha256=…`, `[firmware] version=… sha256=…`, `machine_file_sha256 = "…"`, `[[edge]] from="example-bar" to="example-compositor"`. A build refuses to start if the lock does not match the machine file hash.

## 7. Static shared tools
The node helper, the update tool and the recovery agent are **not modules**. Each has its own version and is built once and shared. A module manifest may say `min_version` for one of them; the **lock records the exact sha256 that was used**. A module whose `min_version` is above the tool in the lock fails the build. (The recovery agent and update tool exist today as prototypes/scripts: SOURCE `docs/STATE.md` 1.)

## 8. The preflight with modules
Order (extends `distro-workshop.md` 7.1): P0 validate (includes the checks in section 5); P1 lock; P2 build twice and compare hashes; P3 static checks; P4 driver check; **P5 boot the image in a VM and run the module tests in dependency order**: a module's `integration` and `preflight` tests run only after every module it depends on has passed. If a dependency's tests fail, the tests of everything that depends on it are **skipped and reported as skipped (not passed)**, and the build fails. P6 update-path check; P7 report.

## 9. Failure table
| What happens | Result |
|---|---|
| Two modules with the same name | Build refused at check 1 |
| Missing dependency | Refused at check 2, names the module and the dependency |
| Conflict present | Refused at check 3 |
| Package not in the mirror | Refused at check 4 (see `source-mirror.md`) |
| Cycle | Refused at check 7, lists the cycle |
| Optional package missing | Warning in the report; image builds |
| A module's integration test fails | Dependents skipped, build fails |
| A module's unit test fails | Fails before any VM boot (cheap, first) |
| Lock does not match the machine file | Build refused |
| Static tool older than `min_version` | Build refused |
| Test script not executable | Refused at check 6 |

## 10. Tests of the composer (to write when built)
1. Two machine files, one module registry: same inputs twice give the same lock (byte-identical).
2. Each of the seven build-time checks has one deliberately broken module and a test (the same style as `testdata/broken/`: SOURCE `docs/inventory-format.md`).
3. Topological order is stable when module names are shuffled.
4. A failing unit test stops before the VM boots.
5. A failing dependency skips its dependents and the report says "skipped".
6. A lock with a wrong machine file hash is refused.
7. A module that adds a banned systemd package is refused.

## 11. Five assumptions I am least sure of
1. That one-directory-per-module is enough for packages that several modules share (BELIEVED; the lock de-duplicates).
2. That module tests can run in a VM in reasonable time: today's QEMU suites take 22 to 57 minutes in software emulation (SOURCE `docs/image.md`; TESTED again 2026-10-10: TestHubImage 2423 s and 2457 s, TestImage 3281 s; results in `docs/qemu-runs/` of PR #75).
3. That health checks per module are enough to replace the hard-coded confirm check (UNKNOWN).
4. That "no conditional includes" does not make machine files repetitive (BELIEVED).
5. That the init stays swappable (the init comparison is not finished: the service directory format is a placeholder).

## 12. Questions for the owner
1. Should module tests run serially or in parallel (in separate VMs)?
2. Should any failed module test be allowed to only warn?
3. What build plus preflight time is acceptable?
4. Should there be a dry-run mode that shows the resolved set and the lock without building?
