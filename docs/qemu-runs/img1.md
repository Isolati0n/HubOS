# TestImage run 1 on main (TESTED)

Start: START 2026-10-10T14:10:56Z uptime=8110.25
End: exit=0 2026-10-10T15:05:37Z uptime=11391.22
Code: main 2b9173e plus CLAUDE.md only; command: go test -tags qemu ./tools/image/ -run '^TestImage$' -v -count=1
Note: the test saw 1 QEMU hang in step reboot and retried it by its own rule (hang log was in the temp folder, not kept); no crashes. Final result PASS.

```
        RESULTS (QEMU hangs seen and retried: 1; every hang: 
          hang 1 in step "reboot" (serial log /tmp/hubos-image-1346519672/hangs/hang-1.log); QEMU crashes (died from a signal) seen and retried: 0; every crash: none)
        PASS     1.6s  root has no systemd program, unit directory or banned package (and the apt pin was in the build)  
        PASS     0.0s  stripped root: no apt, libapt, procps, libproc2, PAM modules, login, passwd file left; the two systemd libraries kept  
        PASS    14.9s  ldd finds no unresolved library in /usr, /bin, /sbin, /lib (and the checker fails when a library is removed)  
        PASS    15.9s  1 read-only root  
        PASS     0.0s  no process named systemd runs in the booted image  counted in /proc/*/comm
        PASS     9.7s  2 first boot; hubd runs from the config partition; PID 1 is s6-svscan, no systemd process  
        PASS     0.7s  A1 the a, b and recovery boot entries are all created WITHOUT load options; the recovery entry points at kernel-recovery.efi and is in BootOrder after the two slots; slot a boots with its own kernel  BootOrder 0007,0008,0009
        PASS     2.3s  3 init restarts a killed service  
        PASS    10.2s  4 unsigned, tampered, wrong-key, replayed/not-newer, no-kernel-version and ONE-kernel manifests refused; nothing written (slot b unchanged)  8 bundles
        PASS   213.4s  5 signed update accepted; config survives; floor raised; old bundle refused; kernel version in the manifest shown  update 8.9 s, reboot to handover 196.4 s
        PASS     0.3s  A1 slot b boots with its own kernel (kernel-b.efi installed by the update; /proc/cmdline shows hubos.slot=b and its own root)  
        PASS     0.0s  R0 the recovery entry is in BootOrder, so it is still there when the firmware (OVMF) starts the next boot; nothing has to recreate it  entries present at the boot of slot b: [hubos-a hubos-b hubos-recovery]; it points at kernel-recovery.efi
        PASS    14.0s  R1 recovery after an update: boots the separate recovery kernel (no slot, no root in its command line) and `hubos-ctl status` runs in its shell  console=ttyS0 ro loglevel=4 panic=5 | version=recovery-1 flavor=recovery kernel-version=6.12
        PASS    39.2s  6a bad boot rolls back: signed bundle with no /sbin/init (stage 0 refuses it)  failure line seen after 17.1 s; rollback complete after 39.2 s; failure counter 0/3 after the rollback (recovery not triggered)
        PASS    37.8s  R2 recovery after a rollback boots the SAME recovery kernel as after the update, whatever slot a holds  same as after the update: true; afterwards slot b release 2 confirmed
        PASS    36.7s  6b bad boot rolls back: signed bundle whose root is garbage (stage 0 cannot mount it)  failure line seen after 15.1 s; rollback complete after 36.7 s; failure counter 0/3 after the rollback (recovery not triggered)
        PASS    97.7s  6c bad boot rolls back: boots but never gets healthy (confirm times out)  failure line seen after 76.7 s; rollback complete after 97.7 s; failure counter 0/3 after the rollback (recovery not triggered); during the unconfirmed trial boot the recovery entry pointed at File(\EFI\hubos\kernel-recovery.efi)
        PASS   129.5s  6d bad boot rolls back: init hangs (the watchdog resets the machine)  failure line seen after 46.5 s; rollback complete after 129.5 s; failure counter 0/3 after the rollback (recovery not triggered)
        PASS    13.1s  7 recovery mode: the recovery entry has NO load options and boots the separate recovery kernel (banner, bare terminal, hubos-ctl status)  after rollbacks and failed trials: still the same recovery boot
        PASS   147.5s  8 an update interrupted at 4 known log lines never harms the confirmed slot; a clean update still works  kill -9 of QEMU at each line
        PASS    70.4s  B1 confirm refuses a slot whose release (2) is below the floor (7): message, exit 2, BootOrder and floor unchanged  automatic confirm step refused: true; manual confirm rc=2; slot b first in BootOrder: false
        PASS    71.1s  A the floor only goes up: the older slot (release 2) booted by hand after the rollbacks; the floor stays 7 and release 5 is refused  floor 7
        PASS    71.1s  B update with a full boot partition fails cleanly; kernel-b.efi and slot b's root unchanged; the old kernel still boots  rc=2; old kernel booted slot b release 2
        PASS     2.2s  B2 hubos-ctl rollback REASON: refuses without a reason; logs old and new floor and the reason; floor 7 -> 2; the running slot is first in BootOrder  rc=0; log: 2026-10-10T14:37:33Z rollback: slot b release 2, floor 7 -> 2, BootOrder first: 0008, reason: test: go back to release 2 on purpose
        PASS   206.3s  C1 the recovery kernel is installed at the confirm step: a release without one leaves recovery alone; the update itself never touches the boot partition; a newer recovery-version is installed after the healthy boot; the same file is left alone  installed version 1 -> 1 (update 9, confirm) -> 1 (update 10: unchanged until its confirm) -> 2 -> 2 (update 11)
        PASS    81.3s  C2 a trial boot that never confirms leaves the old recovery kernel in place (release 12 carries recovery version 3 but was rolled back)  boot partition before the trial 2, after the rollback 2 (still the version-2 file); back on slot a release 11
        PASS     7.2s  C3 a signed manifest whose recovery hash does not match the recovery kernel inside the new root is refused; BootNext is not set  rc=2
        PASS    66.7s  D2 boot time: a pair in the node config with watchdog <= confirm + 15 s is refused with a message and the defaults (120 s, 180 s) are used; a good pair (20/50) is used by stage 0, the confirm step and the watchdog feeder  refused pair 30/40 -> watchdog feeder 'watchdog -F -t 5 -T 180'; good pair 20/50 -> 'watchdog -F -t 5 -T 50'
        PASS    26.4s  C4 recovery boots with BOTH slot roots garbage, the same way as before; its shell runs hubos-ctl status, e2fsck, findfs/blkid, ip and wget, and holds the update public key  sig: console=ttyS0 ro loglevel=4 panic=5 | version=recovery-1 flavor=recovery kernel-version=6.12
        PASS   181.0s  W1 recovery arms the hardware watchdog with the machine's timeout and feeds it: the feeder runs with -T 60, a normal recovery stays up longer than the timeout (uptime 88 s), and with the feeder killed the machine resets and goes through stage 0 and back to recovery  feeder command line ok: true; reset 67 s after the kill; back in the recovery shell: true
        PASS     4.4s  R3 recovery (whose kernel has key 1 only) refuses an unsigned, a tampered, a below-the-floor and a key-2-signed bundle (floor 11); neither root and no BootNext was touched  4 bundles offered in the recovery shell
        PASS    26.9s  R4 recovery installs a signed bundle (release 13) into slot a with both roots garbage; the machine then boots it and confirms it  rc=0; afterwards slot a release 13, confirmed true, failure counter 0/3
        PASS    60.5s  P2 the failure counter survives kill -9 of QEMU (the variable is in the firmware's store), a normal update trial does not trip the breaker, and the healthy boot afterwards clears it  counter before 0/3; stage 0 read 1 after the kill; after the confirm 0/3; slot a release 13
        PASS   332.9s  P1 three kinds of failing boot (garbage root, no init, never healthy) each end in the recovery shell after 3 failed boots in a row; stage 0 counted 0,1,2,3; the shell shows the counter 3/3  garbage root: stage 0 counts 0,1,2,3, 3 failing boots, recovery after 46 s; no init: stage 0 counts 0,1,2,3, 3 failing boots, recovery after 56 s; never healthy: stage 0 counts 0,1,2,3, 3 failing boots, recovery after 218 s
        PASS    27.6s  P3 a good boot after recovery works: the install from the recovery shell clears the counter, the machine boots the release and confirms it  counter in recovery after the install 0/3; afterwards slot a release 13, confirmed true, counter 0/3
        PASS    88.8s  K1 key rotation: key 1 is accepted; a release signed with key 1 that carries key 2 is accepted and its confirm installs a recovery kernel that knows both; after that a bundle signed with key 2 is accepted (and drops key 1 with a recovery kernel for key 2); then a bundle signed with key 1 is refused  keyring files 1 -> 2 -> 1; key-2 bundle before the rotation rc=2; recovery kernel on the boot partition version 4 -> 5; key-1 bundle after the drop rc=2
        PASS    42.3s  K2 the recovery shell uses the keyring of the recovery kernel on the boot partition (key 2 only after the rotation): it refuses the key-1 bundle and installs a key-2 bundle into slot b, which boots and confirms  key-1 bundle rc=2, key-2 bundle rc=0; afterwards slot b release 24 confirmed true
        PASS   138.5s  C5 the recovery kernel install rule at confirm: a changed file at the same version installs (a rotation release that forgot to bump it); a lower version never installs; the same file is left alone  boot partition: version 5 file 21cd250d7024 -> 5 e5a6483e1295 (same version, new file) -> 5 e5a6483e1295 (lower version 4: unchanged) -> 5 e5a6483e1295 (same file: unchanged)
        PASS    49.7s  T18 the recovery agent inside a TEST recovery kernel: GET /v1/status from the host answers recovery; requests are not signed; install requests for an unsigned, another-key, tampered or below-the-floor bundle are refused by the image check and slot b is unchanged; the install request for the correctly signed bundle installs it into slot b and the machine boots and confirms it  agent killed: down seen true, answering again after 1.3 s (supervisor restart loop); status 200 {"api":1,"min_hub":1,"machine":"hub-qemu","state":"recovery","recovery_release":"recovery-1","boot_failures":0,"failure_limit":3}; refused bundles (HTTP code): unsigned 500, other key 500, tampered manifest 500, below the floor 500 (slot b unchanged true); logs 200, clear-failures 200 (no signature needed); install 200; booted slot b release 28 confirmed true; recovery kernel with the agent 8885248 bytes, without 6333440 bytes (+2551808); normal recovery kernel back on the boot partition: true
        PASS    34.6s  T19 the restart loop does not hide a crash loop: after 5 short runs in a row (each under 30 s) the supervisor logs that it gives up and leaves the agent down; the recovery shell keeps working  short-run lines 5, gave up true, no agent left true, /v1/status after that: -1
        PASS   271.2s  9 two builds of the root give identical hashes (root tar asserted; squashfs and the three kernels reported)  tar true, squashfs true, kernels true
--- PASS: TestImage (3280.52s)
    --- PASS: TestImage/systemd_programs_absent_from_the_root (16.68s)
    --- PASS: TestImage/T01_read_only_root (15.85s)
    --- PASS: TestImage/T02_first_boot_hubd_from_config_partition (10.14s)
    --- PASS: TestImage/T02b_slot_entries_have_no_load_options (0.71s)
    --- PASS: TestImage/T03_init_restarts_a_killed_service (2.31s)
    --- PASS: TestImage/T04_bad_bundles_are_refused (10.19s)
    --- PASS: TestImage/T05_signed_update_accepted (213.41s)
    --- PASS: TestImage/T05b_slot_b_boots_with_its_own_kernel (0.30s)
    --- PASS: TestImage/T05c_recovery_after_an_update (34.61s)
    --- PASS: TestImage/6a_bad_boot_rolls_back:_signed_bundle_with_no_/sbin/init_(stage_0_refuses_it) (39.15s)
    --- PASS: TestImage/T06a2_recovery_after_a_rollback (37.85s)
    --- PASS: TestImage/6b_bad_boot_rolls_back:_signed_bundle_whose_root_is_garbage_(stage_0_cannot_mount_it) (36.74s)
    --- PASS: TestImage/6c_bad_boot_rolls_back:_boots_but_never_gets_healthy_(confirm_times_out) (97.73s)
    --- PASS: TestImage/6d_bad_boot_rolls_back:_init_hangs_(the_watchdog_resets_the_machine) (129.50s)
    --- PASS: TestImage/T07_recovery_mode (33.54s)
    --- PASS: TestImage/T08_interrupted_update (147.45s)
    --- PASS: TestImage/T10_no_free_space_on_the_boot_partition (71.11s)
    --- PASS: TestImage/T11_rollback_command (2.21s)
    --- PASS: TestImage/T12_recovery_kernel_installed_at_confirm (294.85s)
    --- PASS: TestImage/T15_per_machine_timeouts (66.78s)
    --- PASS: TestImage/T13_recovery_installs_a_bundle (238.67s)
    --- PASS: TestImage/T14_boot_loop_breaker (420.98s)
    --- PASS: TestImage/T16_key_rotation (131.14s)
    --- PASS: TestImage/T17_recovery_install_rule (138.55s)
    --- PASS: TestImage/T18_recovery_agent_in_a_test_recovery_kernel (49.74s)
    --- PASS: TestImage/T19_recovery_agent_restart_limit (55.13s)
    --- PASS: TestImage/T09_reproducible_builds (271.20s)
PASS
ok  	hubos/tools/image	3280.530s
```
