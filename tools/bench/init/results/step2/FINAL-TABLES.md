# Final tables of the elimination round (all candidates), regenerated 2026-10-12 (follow-up 3) with tools/bench/init/report.py Follow-up 4 (2026-10-13): the rows for `s6rc-par-wd2` and `runit-111-wd2` and the hang tables are appended at the end of this file (generated with report.py, event_counts.py and hangtable.py; the earlier tables above are unchanged, the rows of `s6rc-par-wd`, `runit-wd`, `s6rc-par-wdc`, `runit-wdc` are the "compositor + hubd probes" and "one compositor probe only" runs, not the owner's rule). Follow-up 7 (2026-10-14): the tables of Parts A, B and C are appended at the very end. Follow-up 6 (2026-10-14): the tables of Parts B, C and D are appended at the very end. Follow-up 5 (2026-10-13): the rows for `s6rc-par-wd3` and `runit-111-wd3` (latch version) are appended at the very end; everything above is unchanged.

Notes: (1) a step that appears in several cycles because the machine rebooted itself (runit) is counted once, from its first cycle; runit's logs are merged (first run, rerun from step 31, rerun from step 38). (2) s6plain is the full repeat run (`s6plain-repeat/`). (3) The boot rows of openrc-par and openrc-doc are the re-measurement (`openrc-remeasure/`) made after the memory counter was fixed; the earlier OpenRC memory figures (586, 548 and, for the first openrc run, 664 kB) held PID 1 only. (4) `openrc-doc2` has only trial boots. (5) For the event counts of each column and why they differ, see the run log (follow-up 2, item 1). (6) "alert=yes/no" in the storm rows is the harness's own alert file. (7) Added in follow-up 3: `runit-111np` (runit-111 without -P); `s6rc-par-wd` and `runit-wd` (s6rc-par and runit-111 plus the TEST STAND-IN watcher in its literal 'both probes failing' mode; two more events, steps 44-45: kill the watcher); `s6rc-par-wdc` and `runit-wdc` (the same images; the watcher acts on the compositor probe alone: a labelled variation, not the owner's rule). The watcher candidates' fault tables count the harness's steps 1-45; their boot, memory and CPU figures do not include the watcher in the init-side PSS.


### Boot, memory, idle CPU

| candidate | boots | supervisor start to desktop, s: median (min-max) | PSS of PID 1 + supervisors + policy layer, kB | init-side CPU ticks in 30 s idle | whole VM busy / idle ticks in 30 s (2 vCPU) |
|---|---|---|---|---|---|
| s6plain | 3 | 5.8 (5.8-5.8) | 2466 | 0 | 447 / 5445 |
| s6rc | 3 | 6.4 (6.3-6.4) | 2670 | 1 | 214 / 5674 |
| s6rc-par | 3 | 7.9 (7.6-8.0) | 2674 | 1 | 571 / 5330 |
| runit | 3 | 6.1 (6.1-6.3) | 1880 | 1 | 568 / 5283 |
| runit-111 | 3 | 6.2 (5.9-6.2) | 1907 | 1 | 491 / 5357 |
| runit-111np | 3 | 6.6 (6.5-6.6) | 1921 | 2 | 592 / 5268 |
| b-go | 3 | 7.3 (7.2-7.4) | 5028 | 29 | 204 / 5667 |
| c-go | 3 | 6.4 (6.3-6.7) | 10962 | 267 | 699 / 5059 |
| c-elixir | 3 | 6.3 (6.2-6.5) | 35574 | 234 | 629 / 5158 |
| dinit | 3 | 5.0 (4.8-5.3) | 2096 | 12 | 164 / 5728 |
| dinit-par | 3 | 6.0 (6.0-6.1) | 2096 | 12 | 461 / 5392 |
| dinit-sm | 3 | 5.9 (5.9-6.0) | 2100 | 11 | 433 / 5439 |
| dinit-ms | 3 | 6.2 (6.1-6.3) | 2092 | 13 | 481 / 5379 |
| dinit-wf | 3 | 6.1 (6.1-6.3) | 2096 | 12 | 453 / 5416 |
| openrc | 3 | 8.4 (8.4-9.6) | 664 | 0 | 538 / 5277 |
| openrc-par | 3 | 10.1 (10.1-10.9) | 3036 | 6 | 911 / 4879 |
| openrc-doc | 3 | 10.2 (9.7-10.2) | 3056 | 4 | 881 / 4884 |
| openrc-doc2 | 0 | n/a (n/a-n/a) | n/a | n/a | n/a / n/a |
| s6rc-par-wd | 3 | 7.7 (7.4-7.7) | 2728 | 1 | 643 / 5217 |
| runit-wd | 3 | 6.4 (6.0-6.5) | 1979 | 1 | 583 / 5288 |
| s6rc-par-wdc | 3 | 7.4 (7.1-7.5) | 2732 | 1 | 629 / 5232 |
| runit-wdc | 3 | 6.2 (5.8-6.5) | 2001 | 2 | 657 / 5176 |

### PID 1 killed or wedged

| candidate | PID 1 killed: panic to desktop again, s | PID 1 wedged: injection to desktop again, s (watchdog 30 s) |
|---|---|---|
| s6plain | 17, 17 | 46 (guard noticed after 2) |
| s6rc | 20, 20 | 51 (guard noticed after nan) |
| s6rc-par | 21, 20 | 49 (guard noticed after nan) |
| runit | 16, 15 | 48 (guard noticed after 2) |
| runit-111 | 17, 16 | 47 (guard noticed after 2) |
| runit-111np | 17, 16 | 47 (guard noticed after 2) |
| b-go | 18, 18 | 46 (guard noticed after 0) |
| c-go | 20, 21 | 51 (guard noticed after nan) |
| c-elixir | 21, 22 | 52 (guard noticed after nan) |
| dinit | 15, 18 | 46 (guard noticed after nan) |
| dinit-par | 15, 15 | 45 (guard noticed after nan) |
| dinit-sm | 16, 16 | 46 (guard noticed after nan) |
| dinit-ms | 16, 16 | 46 (guard noticed after nan) |
| dinit-wf | 16, 18 | 45 (guard noticed after nan) |
| openrc | 20, 22 | 51 (guard noticed after 2) |
| openrc-par | 21, 27 | 51 (guard noticed after 1) |
| openrc-doc | 20, 20 | 50 (guard noticed after 1) |
| openrc-doc2 | n/a | n/a |
| s6rc-par-wd | 22, 24 | 51 (guard noticed after nan) |
| runit-wd | 17, 16 | 45 (guard noticed after 2) |
| s6rc-par-wdc | 21, 20 | 49 (guard noticed after nan) |
| runit-wdc | 16, 16 | 46 (guard noticed after 2) |

### Update trial boot

| candidate | trial: healthy | trial: driftwm cannot start | trial: driftwm wedged 2 s after it first worked |
|---|---|---|---|
| s6plain | CONFIRMED after 15.9 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 16.0 s (FALSE: driftwm stopped) |
| s6rc | CONFIRMED after 22.3 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 21.8 s (FALSE: driftwm stopped) |
| s6rc-par | CONFIRMED after 23.1 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 22.7 s (FALSE: driftwm stopped) |
| runit | CONFIRMED after 19.8 s | CONFIRMED after 19.4 s (FALSE: waybar not running) | CONFIRMED after 19.6 s (FALSE: driftwm stopped) |
| runit-111 | CONFIRMED after 19.3 s | CONFIRMED after 19.4 s (FALSE: driftwm not running) | CONFIRMED after 19.7 s (FALSE: driftwm stopped) |
| runit-111np | CONFIRMED after 20.7 s | CONFIRMED after 21.0 s (FALSE: waybar not running) | CONFIRMED after 20.7 s (FALSE: driftwm stopped) |
| b-go | CONFIRMED after 23.5 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 37.5 s |
| c-go | CONFIRMED after 23.3 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 41.7 s |
| c-elixir | CONFIRMED after 28.9 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 32.6 s |
| dinit | CONFIRMED after 20.2 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 20.0 s (FALSE: driftwm stopped) |
| dinit-par | CONFIRMED after 21.2 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 20.8 s (FALSE: driftwm stopped) |
| dinit-sm | CONFIRMED after 21.4 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 21.0 s (FALSE: driftwm stopped) |
| dinit-ms | CONFIRMED after 21.4 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 21.2 s (FALSE: driftwm stopped) |
| dinit-wf | CONFIRMED after 21.1 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 20.9 s (FALSE: driftwm stopped) |
| openrc | ROLLBACK reboot at 60 s, guard kept feeding | ROLLBACK reboot at 60 s, guard kept feeding | ROLLBACK reboot at 60 s, guard kept feeding |
| openrc-par | ROLLBACK reboot at 60 s, guard kept feeding | ROLLBACK reboot at 60 s, guard kept feeding | ROLLBACK reboot at 60 s, guard kept feeding |
| openrc-doc | ROLLBACK reboot at 60 s, guard kept feeding | ROLLBACK reboot at 60 s, guard kept feeding | ROLLBACK reboot at 60 s, guard kept feeding |
| openrc-doc2 | CONFIRMED after 27.0 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 27.1 s (FALSE: driftwm not running) |
| s6rc-par-wd | CONFIRMED after 23.1 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 23.0 s (FALSE: driftwm stopped) |
| runit-wd | CONFIRMED after 20.2 s | CONFIRMED after 20.2 s (FALSE: driftwm not running) | CONFIRMED after 19.8 s (FALSE: driftwm stopped) |
| s6rc-par-wdc | CONFIRMED after 23.4 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 22.8 s (FALSE: driftwm stopped) |
| runit-wdc | CONFIRMED after 20.1 s | CONFIRMED after 19.9 s (FALSE: waybar not running) | CONFIRMED after 19.7 s (FALSE: driftwm stopped) |

### Fault injection

| fault: median recovery s (worst) | s6plain | s6rc | s6rc-par | runit | runit-111 | runit-111np | b-go | c-go | c-elixir | dinit | dinit-par | dinit-sm | dinit-ms | dinit-wf | openrc | openrc-par | openrc-doc | openrc-doc2 | s6rc-par-wd | runit-wd | s6rc-par-wdc | runit-wdc |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| kill (any of 6) | 2.8 (max 6.5) | 0.6 (max 1.9); 3/15 needed a person | 1.9 (max 5.0) | 1.9 (max 4.2) | 1.8 (max 4.2) | 1.8 (max 4.4) | 3.6 (max 6.7) | 3.1 (max 8.9) | 2.8 (max 11.1) | 0.8 (max 5.7) | 2.0 (max 6.9) | 1.7 (max 2.3) | 1.8 (max 3.9) | 1.8 (max 4.0) | 1.9 (max 2.9); 6/18 needed a person | 3.1 (max 7.2) | 2.8 (max 17.7) | n/a | 1.9 (max 5.1) | 1.8 (max 4.4) | 1.9 (max 5.6) | 1.9 (max 4.3) |
| hang-stop | -; 3/3 needed a person | n/a | -; 3/3 needed a person | -; 3/3 needed a person | -; 3/3 needed a person | -; 3/3 needed a person | 10.9 (max 15.1) | 9.7 (max 13.7) | 14.2 (max 15.4) | -; 3/3 needed a person | -; 3/3 needed a person | n/a | -; 3/3 needed a person | -; 3/3 needed a person | -; 3/3 needed a person | -; 3/3 needed a person | 20.1 (max 20.2) | n/a | -; 3/3 needed a person | -; 3/3 needed a person | 36.5 (max 36.5); 1/3 needed a person | 36.8 (max 36.9); 1/3 needed a person |
| hang-spin | 0.1 (max 0.1) | 0.1 (max 0.1) | 0.1 (max 0.1); 2/3 needed a person | 0.1 (max 0.1); 1/3 needed a person | -; 3/3 needed a person | -; 3/3 needed a person | 0.1 (max 0.2) | 12.8 (max 14.0) | 14.4 (max 14.4) | -; 3/3 needed a person | 0.1 (max 0.1); 1/3 needed a person | 0.0 (max 0.0) | 0.1 (max 0.1) | 0.1 (max 0.1) | 0.2 (max 0.2); 1/3 needed a person | 0.1 (max 15.7) | 0.1 (max 20.3) | n/a | 0.1 (max 0.1); 2/3 needed a person | 0.1 (max 0.1); 2/3 needed a person | 0.2 (max 38.0) | 35.8 (max 35.8); 2/3 needed a person |
| dep-slow | 21.1 (max 21.1) | n/a | 20.4 (max 20.4) | 19.7 (max 19.7) | 18.8 (max 18.8) | 20.1 (max 20.1) | 18.9 (max 18.9) | 18.9 (max 18.9) | 20.5 (max 20.5) | 32.9 (max 32.9) | 33.9 (max 33.9) | n/a | n/a | 18.9 (max 18.9) | -; 1/1 needed a person | 22.6 (max 22.6) | 21.5 (max 21.5) | n/a | 21.2 (max 21.2) | 19.1 (max 19.1) | 20.5 (max 20.5) | 18.7 (max 18.7) |
| dep-fail | 5.7 (max 5.7) | n/a | 4.7 (max 4.7) | 5.3 (max 5.3) | 3.9 (max 3.9) | 5.4 (max 5.4) | 3.9 (max 3.9) | 3.9 (max 3.9) | 6.3 (max 6.3) | n/a | n/a | n/a | n/a | n/a | n/a | -; 1/1 needed a person | 6.1 (max 6.1) | n/a | 4.5 (max 4.5) | 5.7 (max 5.7) | 6.5 (max 6.5) | 4.6 (max 4.6) |
| disk-full | 0.1 (max 0.1) | n/a | 0.2 (max 0.2) | 0.2 (max 0.2) | 0.1 (max 0.1) | 0.2 (max 0.2) | 0.1 (max 0.1) | 0.2 (max 0.2) | 0.2 (max 0.2) | n/a | n/a | n/a | 0.1 (max 0.1) | 0.1 (max 0.1) | 0.1 (max 0.1) | 3.4 (max 6.8) | 0.1 (max 0.1) | n/a | 0.2 (max 0.2) | 0.2 (max 0.2) | 0.2 (max 0.2) | 0.2 (max 0.2) |
| supervisor-killed | 0.9 (max 16.6) | n/a | 4.6 (max 18.1) | 0.2 (max 0.3) | 0.1 (max 5.4) | 0.1 (max 3.5) | n/a | 0.2 (max 7.5); 1/5 needed a person | 0.2 (max 17.8); 1/5 needed a person | n/a | n/a | n/a | n/a | n/a | 0.1 (max 0.1) | 0.1 (max 0.1) | 0.1 (max 0.1) | n/a | 2.1 (max 9.5) | 0.2 (max 3.2) | 1.4 (max 7.3) | 0.2 (max 2.8) |
| scanner-killed | n/a | n/a | n/a | n/a | 0.1 (max 0.1) | 9.0 (max 9.0) | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | 0.2 (max 0.2) | n/a | 0.1 (max 0.1) |
| brain-killed | n/a | n/a | n/a | n/a | n/a | n/a | 0.1 (max 0.1) | 0.2 (max 0.2) | 0.1 (max 0.2) | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a |
| clock-3600 | 6.4 (max 6.4) | n/a | 4.7 (max 4.7) | 5.0 (max 5.0) | 0.1 (max 0.1) | 5.1 (max 5.1) | 3.4 (max 3.4) | 3.4 (max 3.4) | 5.7 (max 5.7) | n/a | n/a | n/a | 3.9 (max 3.9) | 3.7 (max 3.7) | -; 1/1 needed a person | 6.5 (max 6.5) | 5.8 (max 5.8) | n/a | 4.4 (max 4.4) | 0.1 (max 0.1) | 4.7 (max 4.7) | 4.6 (max 4.6) |
| clock+86400 | 6.0 (max 6.0) | n/a | 4.5 (max 4.5) | 4.0 (max 4.0) | 3.2 (max 3.2) | 3.5 (max 3.5) | 4.7 (max 4.7) | 0.2 (max 0.2) | 3.2 (max 3.2) | n/a | n/a | n/a | 3.6 (max 3.6) | 3.9 (max 3.9) | -; 1/1 needed a person | 7.3 (max 7.3) | 6.0 (max 6.0) | n/a | 4.6 (max 4.6) | 0.1 (max 0.1) | 4.6 (max 4.6) | 4.1 (max 4.1) |
| oom | 0.1 (max 0.1) | n/a | 0.1 (max 0.1) | 0.1 (max 0.1) | 0.1 (max 0.1) | 0.1 (max 0.1) | 0.1 (max 0.1) | 0.2 (max 0.2) | 0.1 (max 0.1) | n/a | n/a | n/a | 0.1 (max 0.1) | 0.1 (max 0.1) | 0.1 (max 0.1) | 0.1 (max 0.1) | 0.1 (max 0.1) | n/a | 0.1 (max 0.1) | 0.2 (max 0.2) | 0.2 (max 0.2) | 0.1 (max 0.1) |

| measure | s6plain | s6rc | s6rc-par | runit | runit-111 | runit-111np | b-go | c-go | c-elixir | dinit | dinit-par | dinit-sm | dinit-ms | dinit-wf | openrc | openrc-par | openrc-doc | openrc-doc2 | s6rc-par-wd | runit-wd | s6rc-par-wdc | runit-wdc |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| crash storm on hubd, 40 s | 18 starts; 5 in last 10 s; alert=no | 35 starts; 8 in last 10 s; alert=no | 18 starts; 5 in last 10 s; alert=no | 33 starts; 9 in last 10 s; alert=no | 33 starts; 8 in last 10 s; alert=no | 33 starts; 9 in last 10 s; alert=no | 38 starts; 10 in last 10 s; alert=no | 17 starts; 2 in last 10 s; alert=yes | 21 starts; 2 in last 10 s; alert=yes | 1 starts; 0 in last 10 s; alert=no | 1 starts; 0 in last 10 s; alert=no | n/a | 1 starts; 0 in last 10 s; alert=no | 1 starts; 0 in last 10 s; alert=no | 10 starts; 0 in last 10 s; alert=no | 8 starts; 0 in last 10 s; alert=no | 16 starts; 5 in last 10 s; alert=no | n/a | 18 starts; 4 in last 10 s; alert=no | 33 starts; 8 in last 10 s; alert=no | 17 starts; 4 in last 10 s; alert=no | 33 starts; 8 in last 10 s; alert=no |
| crash storm on driftwm, 40 s | 26 starts; 7 in last 10 s; alert=no | 26 starts; 6 in last 10 s; alert=no | 26 starts; 6 in last 10 s; alert=no | 26 starts; 7 in last 10 s; alert=no | 27 starts; 7 in last 10 s; alert=no | 26 starts; 7 in last 10 s; alert=no | 39 starts; 10 in last 10 s; alert=no | 11 starts; 2 in last 10 s; alert=yes | 10 starts; 1 in last 10 s; alert=yes | 1 starts; 0 in last 10 s; alert=no | 1 starts; 0 in last 10 s; alert=no | n/a | 1 starts; 0 in last 10 s; alert=no | 1 starts; 0 in last 10 s; alert=no | 10 starts; 0 in last 10 s; alert=no | 11 starts; 0 in last 10 s; alert=no | 20 starts; 5 in last 10 s; alert=no | n/a | 27 starts; 7 in last 10 s; alert=no | 25 starts; 6 in last 10 s; alert=no | 26 starts; 6 in last 10 s; alert=no | 26 starts; 6 in last 10 s; alert=no |
| seatd slow: starts of driftwm / waybar / hubd while waiting | 11 / 6 / 2 | n/a | 11 / 7 / 3 | 11 / 8 / 4 | 11 / 7 / 2 | 11 / 9 / 2 | 1 / 1 / 1 | 2 / 10 / 1 | 3 / 19 / 1 | 1 / 1 / 1 | 1 / 1 / 1 | n/a | n/a | 1 / 1 / 1 | 9 / 11 / 1 | 8 / 5 / 2 | 9 / 6 / 1 | n/a | 11 / 6 / 4 | 11 / 9 / 2 | 11 / 6 / 3 | 11 / 8 / 3 |
| seatd fail: starts of driftwm / waybar / hubd while waiting | 15 / 7 / 2 | n/a | 14 / 7 / 2 | 15 / 12 / 2 | 15 / 11 / 2 | 15 / 11 / 3 | 1 / 1 / 1 | 2 / 23 / 1 | 3 / 2 / 1 | n/a | n/a | n/a | n/a | n/a | n/a | 3 / 3 / 2 | 11 / 7 / 2 | n/a | 14 / 7 / 3 | 15 / 11 / 2 | 15 / 8 / 2 | 15 / 10 / 2 |

| unrecovered by itself, count | s6plain | s6rc | s6rc-par | runit | runit-111 | runit-111np | b-go | c-go | c-elixir | dinit | dinit-par | dinit-sm | dinit-ms | dinit-wf | openrc | openrc-par | openrc-doc | openrc-doc2 | s6rc-par-wd | runit-wd | s6rc-par-wdc | runit-wdc |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| after-clock-3600 (alert=0) | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| after-supervisor-killed (alert=0) | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 6 | 6 | 6 | 0 | 0 | 0 | 0 | 0 |
| clock+86400 (alert=0) | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| clock-3600 (alert=0) | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| dep-fail (alert=0) | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 1 | 0 | 1 | 1 | 1 | 1 | 0 | 0 | 0 | 0 | 0 | 0 |
| dep-slow (alert=0) | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| hang-spin (alert=0) | 0 | 0 | 2 | 1 | 3 | 3 | 0 | 0 | 0 | 3 | 1 | 0 | 1 | 0 | 1 | 0 | 0 | 0 | 2 | 2 | 0 | 2 |
| hang-stop (alert=0) | 3 | 3 | 3 | 3 | 3 | 3 | 0 | 0 | 0 | 3 | 3 | 2 | 3 | 3 | 3 | 3 | 0 | 0 | 3 | 3 | 1 | 1 |
| kill (alert=0) | 0 | 6 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 5 | 5 | 6 | 0 | 0 | 6 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| kill (alert=1) | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| storm (alert=0) | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 2 | 2 | 0 | 2 | 2 | 2 | 2 | 0 | 0 | 0 | 0 | 0 | 0 |
| storm (alert=1) | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| supervisor-killed (alert=1) | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 2 | 2 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |

---

# Follow-up 4 rows (2026-10-13): the owner's rule (probes: driftwm `state` and Wayland `wl_display.sync`, both must fail; 4 rounds and 30 s; 3 kills in 10 minutes; budget in a file). TEST STAND-IN watcher, not the product watcher.

Source logs: the 3-repetition full suites `s6rc-par-wd2/s6rc-par-wd2-faults-75.log` and `runit-111-wd2/runit-111-wd2-faults-85.log`, boots `*-boot-71..73` / `*-boot-81..83`. PID 1 killed or wedged and the trial boots were **not run** for these two candidates (the init and the boot path are those of `s6rc-par` and `runit-111`; not re-measured). In the fault table the rows `hang-stop` and `hang-spin` count hangs 4 to 6 of the suite (capped by the kill budget) as "needed a person"; the spaced hang runs and the budget runs are in the hang tables below.

### Boot, memory, idle CPU

| candidate | boots | supervisor start to desktop, s: median (min-max) | PSS of PID 1 + supervisors + policy layer, kB | init-side CPU ticks in 30 s idle | whole VM busy / idle ticks in 30 s (2 vCPU) |
|---|---|---|---|---|---|
| s6rc-par-wd2 | 3 | 8.0 (7.8-8.9) | 2728 | 1 | 814 / 4989 |
| runit-111-wd2 | 3 | 6.8 (6.7-7.3) | 1987 | 2 | 896 / 4915 |

### PID 1 killed or wedged

| candidate | PID 1 killed: panic to desktop again, s | PID 1 wedged: injection to desktop again, s (watchdog 30 s) |
|---|---|---|
| s6rc-par-wd2 | n/a | n/a |
| runit-111-wd2 | n/a | n/a |

### Update trial boot

| candidate | trial: healthy | trial: driftwm cannot start | trial: driftwm wedged 2 s after it first worked |
|---|---|---|---|
| s6rc-par-wd2 | n/a | n/a | n/a |
| runit-111-wd2 | n/a | n/a | n/a |

### Fault injection

| fault: median recovery s (worst) | s6rc-par-wd2 | runit-111-wd2 |
|---|---|---|
| kill (any of 6) | 1.9 (max 5.8) | 1.9 (max 4.5) |
| hang-stop | 38.0 (max 38.4); 1/3 needed a person | 36.2 (max 36.5); 1/3 needed a person |
| hang-spin | 37.0 (max 37.0); 2/3 needed a person | 36.4 (max 36.4); 2/3 needed a person |
| dep-slow | 20.7 (max 20.7) | 19.4 (max 19.4) |
| dep-fail | 5.6 (max 5.6) | 5.5 (max 5.5) |
| disk-full | 0.2 (max 0.2) | 0.2 (max 0.3) |
| supervisor-killed | 0.2 (max 14.3) | 0.2 (max 4.5) |
| scanner-killed | n/a | 0.2 (max 0.2) |
| brain-killed | n/a | n/a |
| clock-3600 | 1.0 (max 1.0) | 5.8 (max 5.8) |
| clock+86400 | 5.3 (max 5.3) | 3.7 (max 3.7) |
| oom | 0.1 (max 0.1) | 0.1 (max 0.1) |

| measure | s6rc-par-wd2 | runit-111-wd2 |
|---|---|---|
| crash storm on hubd, 40 s | 18 starts; 5 in last 10 s; alert=no | 32 starts; 8 in last 10 s; alert=no |
| crash storm on driftwm, 40 s | 24 starts; 6 in last 10 s; alert=no | 25 starts; 7 in last 10 s; alert=no |
| seatd slow: starts of driftwm / waybar / hubd while waiting | 10 / 6 / 3 | 10 / 10 / 2 |
| seatd fail: starts of driftwm / waybar / hubd while waiting | 13 / 8 / 3 | 14 / 12 / 2 |

| unrecovered by itself, count | s6rc-par-wd2 | runit-111-wd2 |
|---|---|---|
| hang-spin (alert=0) | 2 | 2 |
| hang-stop (alert=0) | 1 | 1 |

### Events defined / ran, of the 3-repetition full suites (event_counts.py)

| candidate | events the script defines for it | events that ran (EV lines) | lost to stuck states | never reached | other difference | stuck steps (fault) | last step reached | launches | script finished | of the 36 common events: ran / needed a person / stuck |
|---|---|---|---|---|---|---|---|---|---|---|
| s6rc-par-wd2 | 50 | 50 | 0 | 0 | 0 | - | 45 | 1 | yes | 36 / 3 / 0 |
| runit-111-wd2 | 52 | 52 | 0 | 0 | 0 | - | 45 | 1 | yes | 36 / 3 / 0 |

### Hangs: one table per log (hangtable.py)

#### results/step2/s6rc-par-wd2/s6rc-par-wd2-faults-75.log
| n | shape | arm delay s | fault to kill s | fault to desktop s | windows back | needed a person | watcher state file at the end |
|---|---|---|---|---|---|---|---|
| 19 | stop | 0.00 | 32.60 | 38.40 | yes | no | - |
| 20 | spin | 0.03 | 32.35 | 37.03 | yes | no | - |
| 21 | stop | 0.00 | 32.70 | 37.64 | yes | no | - |
| 22 | spin | 0.47 | 63.70 | 65.82 | yes | YES | capped 2026-10-13T00:29:32Z |
| 23 | stop | 0.00 | 60.68 | 64.67 | yes | YES | capped 2026-10-13T00:29:32Z |
| 24 | spin | 0.24 | 64.10 | 65.18 | yes | YES | capped 2026-10-13T00:29:32Z |
    CH t=302.6 UNRECOVERED ev=22 fault=hang-spin target=driftwm alert=0 why="driftwm does not answer (no answer)"
    CH t=373.5 UNRECOVERED ev=23 fault=hang-stop target=driftwm alert=0 why="driftwm stopped"
    CH t=444.5 UNRECOVERED ev=24 fault=hang-spin target=driftwm alert=0 why="driftwm does not answer (no answer)"
    CH t=494.9 STORM target=hubd dur=40 starts=18 starts_last10s=5 cpu_ticks_init_side=18 bar-up
    CH t=540.9 STORM target=driftwm dur=40 starts=24 starts_last10s=6 cpu_ticks_init_side=49 bar-down
    CH t=797.9 EV ev=47 fault=oom target=hog rec=0.09 ok=1 fixed=0 hog_killed=1 services_lost=[] pid1_state=S
    CH t=806.4 EV ev=48 fault=oom target=hog rec=0.09 ok=1 fixed=0 hog_killed=1 services_lost=[] pid1_state=S

#### results/step2/s6rc-par-wd2/s6rc-par-wd2-hangs-52.log
| n | shape | arm delay s | fault to kill s | fault to desktop s | windows back | needed a person | watcher state file at the end |
|---|---|---|---|---|---|---|---|
| 1 | spin | 0.22 | 32.53 | 37.39 | yes | no | - |
| 2 | stop | 0.00 | 32.10 | 37.84 | yes | no | - |
| 3 | deadlock | 0.12 | 32.88 | 38.67 | yes | no | - |
| 4 | stop | 0.00 | 32.10 | 37.26 | yes | no | - |
| 5 | spin | 0.23 | 33.35 | 38.87 | yes | no | - |
| 6 | deadlock | 0.28 | 32.19 | 37.54 | yes | no | - |
| 7 | deadlock | 0.47 | 32.70 | 37.53 | yes | no | - |
| 8 | stop | 0.00 | 32.35 | 38.14 | yes | no | - |
| 9 | spin | 0.39 | 32.23 | 36.80 | yes | no | - |

#### results/step2/s6rc-par-wd2/s6rc-par-wd2-hangs-57.log
| n | shape | arm delay s | fault to kill s | fault to desktop s | windows back | needed a person | watcher state file at the end |
|---|---|---|---|---|---|---|---|
| 1 | stop | 0.00 | 32.17 | 37.38 | yes | no | - |
| 2 | spin | 0.02 | 32.56 | 37.74 | yes | no | - |
| 3 | deadlock | 0.23 | 31.87 | 36.88 | yes | no | - |

#### results/step2/s6rc-par-wd2/s6rc-par-wd2-budget-1.log
| n | shape | arm delay s | fault to kill s | fault to desktop s | windows back | needed a person | watcher state file at the end |
|---|---|---|---|---|---|---|---|
| 1 | stop | 0.00 | 32.52 | 38.14 | yes | no | - |
| 3 | spin | 0.49 | 32.55 | 37.26 | yes | no | - |
| 5 | deadlock | 0.32 | 32.33 | 37.84 | yes | no | - |
| 9 | deadlock | 0.39 | 33.09 | 37.92 | yes | no | - |

#### results/step2/runit-111-wd2/runit-111-wd2-faults-85.log
| n | shape | arm delay s | fault to kill s | fault to desktop s | windows back | needed a person | watcher state file at the end |
|---|---|---|---|---|---|---|---|
| 19 | stop | 0.00 | 32.57 | 36.52 | yes | no | - |
| 20 | spin | 0.41 | 32.78 | 36.38 | yes | no | - |
| 21 | stop | 0.00 | 31.90 | 35.87 | yes | no | - |
| 22 | spin | 0.34 | 63.27 | 63.37 | yes | YES | capped 2026-10-13T00:43:05Z |
| 23 | stop | 0.00 | 61.01 | 64.31 | yes | YES | capped 2026-10-13T00:43:05Z |
| 24 | spin | 0.08 | 64.23 | 64.04 | yes | YES | capped 2026-10-13T00:43:05Z |
    CH t=290.8 UNRECOVERED ev=22 fault=hang-spin target=driftwm alert=0 why="driftwm does not answer (no answer)"
    CH t=360.0 UNRECOVERED ev=23 fault=hang-stop target=driftwm alert=0 why="driftwm stopped"
    CH t=431.0 UNRECOVERED ev=24 fault=hang-spin target=driftwm alert=0 why="driftwm does not answer (no answer)"
    CH t=481.3 STORM target=hubd dur=40 starts=32 starts_last10s=8 cpu_ticks_init_side=23 bar-up
    CH t=525.8 STORM target=driftwm dur=40 starts=25 starts_last10s=7 cpu_ticks_init_side=46 bar-down
    CH t=748.9 EV ev=49 fault=oom target=hog rec=0.12 ok=1 fixed=0 hog_killed=1 services_lost=[] pid1_state=S
    CH t=758.2 EV ev=50 fault=oom target=hog rec=0.12 ok=1 fixed=0 hog_killed=1 services_lost=[] pid1_state=S

#### results/step2/runit-111-wd2/runit-111-wd2-hangs-62.log
| n | shape | arm delay s | fault to kill s | fault to desktop s | windows back | needed a person | watcher state file at the end |
|---|---|---|---|---|---|---|---|
| 1 | stop | 0.00 | 32.48 | 36.64 | yes | no | - |
| 2 | deadlock | 0.50 | 32.10 | 35.60 | yes | no | - |
| 3 | spin | 0.04 | 32.30 | 36.61 | yes | no | - |
| 4 | deadlock | 0.01 | 32.05 | 36.44 | yes | no | - |
| 5 | spin | 0.07 | 32.28 | 36.59 | yes | no | - |
| 6 | stop | 0.00 | 32.24 | 36.53 | yes | no | - |
| 7 | deadlock | 0.33 | 32.42 | 37.13 | yes | no | - |
| 8 | stop | 0.00 | 32.10 | 35.71 | yes | no | - |
| 9 | spin | 0.21 | 32.09 | 35.64 | yes | no | - |

#### results/step2/runit-111-wd2/runit-111-wd2-hangs-67.log
| n | shape | arm delay s | fault to kill s | fault to desktop s | windows back | needed a person | watcher state file at the end |
|---|---|---|---|---|---|---|---|
| 1 | stop | 0.00 | 32.26 | 37.03 | yes | no | - |
| 2 | deadlock | 0.14 | 31.94 | 36.32 | yes | no | - |
| 3 | spin | 0.01 | 32.33 | 36.33 | yes | no | - |

#### results/step2/runit-111-wd2/runit-111-wd2-budget-1.log
| n | shape | arm delay s | fault to kill s | fault to desktop s | windows back | needed a person | watcher state file at the end |
|---|---|---|---|---|---|---|---|
| 1 | stop | 0.00 | 32.61 | 36.38 | yes | no | - |
| 3 | spin | 0.31 | 32.47 | 36.35 | yes | no | - |
| 5 | deadlock | 0.17 | 32.57 | 36.33 | yes | no | - |
| 9 | deadlock | 0.32 | 32.62 | 37.23 | yes | no | - |

### Lock-probe, runit-111np (logcheck.py)

`results/step2/runit-111np/runit-111np-lockprobe-41.log`
```
TOTAL gaps inside the numbered runs: 0; repeated lines: 0
driftwm other lines: {'starting pid N': 4, 'ready': 4, 'already running, exiting': 1}
waybar other lines: {'starting pid N': 11, 'ready': 7, 'compositor connection lost, exiting': 2, 'no compositor: dial unix /run/dw/wayland-N: connect: connection refused': 4, 'already running, exiting': 1}
```
`results/step2/runit-111np/runit-111np-lockprobe-42.log`
```
TOTAL gaps inside the numbered runs: 0; repeated lines: 0
driftwm other lines: {'starting pid N': 13, 'ready': 13, 'already running, exiting': 9}
waybar other lines: {'starting pid N': 38, 'ready': 25, 'compositor connection lost, exiting': 9, 'no compositor: dial unix /run/dw/wayland-N: connect: connection refused': 13, 'already running, exiting': 4}
```

### Soak, runit-111np, one 2-hour chunk (seed 7010; `soak/runit-111np-fu4chunk1.log`)

```
== /home/user/HubOS/tools/bench/init/results/step2/soak/runit-111np-fu4chunk1.log: 2 launch(es), first 600 s of each left out
TOTAL          slope   -87.75 kB per 1000 restarts (std.err  37.16)  noise sd   82.9 kB  range 1776..2680 kB  samples 110  restarts covered 749
run            slope    +8.92 kB per 1000 restarts (std.err  12.12)  noise sd   27.0 kB  range 0..283 kB  samples 110  restarts covered 749
runit          slope    -1.89 kB per 1000 restarts (std.err   2.30)  noise sd    5.1 kB  range 180..238 kB  samples 110  restarts covered 749
runsv          slope   -58.09 kB per 1000 restarts (std.err  21.03)  noise sd   46.9 kB  range 1349..1878 kB  samples 110  restarts covered 749
runsvdir       slope   -36.70 kB per 1000 restarts (std.err   2.83)  noise sd    6.3 kB  range 211..281 kB  samples 110  restarts covered 749
STUCK states in chunk: 1; SOAK-END lines: 1; launches: 2; self-reboots (reboot: Restarting system): 0; events that needed a person (ok=0): 34; UNRECOVERED lines: 35
```

# Follow-up 5 rows (2026-10-13): latch version of the test watcher (`s6rc-par-wd3`, `runit-111-wd3`). TEST STAND-IN watcher, not the product watcher.

Rule and decisions: see the run log, follow-up 5. Probes: driftwm `state` (5 s) and Wayland `wl_display.sync` (2 s), both must fail; 4 rounds and 30 s; 3 kills in 10 minutes; at the 4th wanted kill the watcher latches until a person clears it; it kills only the process it recorded at the first failed round. Source logs: the 3-repetition full suites `s6rc-par-wd3/s6rc-par-wd3-faults-106.log` and `runit-111-wd3/runit-111-wd3-faults-116.log`. Boot rows, PID 1 killed or wedged and trial boots were **not run** for these two candidates (same init and boot path as `s6rc-par` and `runit-111`). The `-wd2` rows above are unchanged (window version).

### Fault injection

| fault: median recovery s (worst) | s6rc-par-wd3 | runit-111-wd3 |
|---|---|---|
| kill (any of 6) | 1.9 (max 4.7) | 1.8 (max 4.1) |
| hang-stop | 40.6 (max 41.4); 1/3 needed a person | 39.8 (max 40.0); 1/3 needed a person |
| hang-spin | 40.9 (max 40.9); 2/3 needed a person | 39.0 (max 39.0); 2/3 needed a person |
| dep-slow | 20.7 (max 20.7) | 19.6 (max 19.6) |
| dep-fail | 5.5 (max 5.5) | 4.1 (max 4.1) |
| disk-full | 0.2 (max 0.2) | 0.1 (max 0.2) |
| supervisor-killed | 0.2 (max 14.4) | 0.2 (max 3.0) |
| scanner-killed | n/a | 0.2 (max 0.2) |
| brain-killed | n/a | n/a |
| clock-3600 | 5.6 (max 5.6) | 4.3 (max 4.3) |
| clock+86400 | 4.6 (max 4.6) | 4.1 (max 4.1) |
| oom | 0.1 (max 0.1) | 0.1 (max 0.1) |

| measure | s6rc-par-wd3 | runit-111-wd3 |
|---|---|---|
| crash storm on hubd, 40 s | 18 starts; 5 in last 10 s; alert=no | 33 starts; 8 in last 10 s; alert=no |
| crash storm on driftwm, 40 s | 26 starts; 7 in last 10 s; alert=no | 26 starts; 6 in last 10 s; alert=no |
| seatd slow: starts of driftwm / waybar / hubd while waiting | 11 / 7 / 3 | 11 / 9 / 3 |
| seatd fail: starts of driftwm / waybar / hubd while waiting | 14 / 7 / 2 | 15 / 12 / 2 |

| unrecovered by itself, count | s6rc-par-wd3 | runit-111-wd3 |
|---|---|---|
| hang-spin (alert=0) | 2 | 2 |
| hang-stop (alert=0) | 1 | 1 |

### Events defined / ran, of the 3-repetition full suites (event_counts.py)

| candidate | events the script defines for it | events that ran (EV lines) | lost to stuck states | never reached | other difference | stuck steps (fault) | last step reached | launches | script finished | of the 36 common events: ran / needed a person / stuck |
|---|---|---|---|---|---|---|---|---|---|---|
| s6rc-par-wd3 | 50 | 50 | 0 | 0 | 0 | - | 45 | 1 | yes | 36 / 3 / 0 |
| runit-111-wd3 | 52 | 52 | 0 | 0 | 0 | - | 45 | 1 | yes | 36 / 3 / 0 |

### Hangs: one table per log (hangtable.py)

#### results/step2/s6rc-par-wd3/s6rc-par-wd3-faults-106.log

| n | shape | arm delay s | fault to kill s | fault to desktop s | windows back | needed a person | watcher state file at the end |
|---|---|---|---|---|---|---|---|
| 19 | stop | 0.00 | 36.17 | 41.40 | yes | no | - |
| 20 | spin | 0.06 | 35.82 | 40.87 | yes | no | - |
| 21 | stop | 0.00 | 35.37 | 39.72 | yes | no | - |
| 22 | spin | 0.36 | 63.78 | 64.63 | yes | YES | - |
| 23 | stop | 0.00 | 60.80 | 64.68 | yes | YES | - |
| 24 | spin | 0.23 | 63.54 | 65.62 | yes | YES | - |
    CH t=308.2 UNRECOVERED ev=22 fault=hang-spin target=driftwm alert=0 why="driftwm does not answer (no answer)"
    CH t=377.6 UNRECOVERED ev=23 fault=hang-stop target=driftwm alert=0 why="driftwm stopped"
    CH t=448.2 UNRECOVERED ev=24 fault=hang-spin target=driftwm alert=0 why="driftwm does not answer (no answer)"
    CH t=499.2 STORM target=hubd dur=40 starts=18 starts_last10s=5 cpu_ticks_init_side=17 bar-up
    CH t=544.5 STORM target=driftwm dur=40 starts=26 starts_last10s=7 cpu_ticks_init_side=44 bar-down
    CH t=775.6 EV ev=47 fault=oom target=hog rec=0.09 ok=1 fixed=0 hog_killed=1 services_lost=[] pid1_state=S
    CH t=783.3 EV ev=48 fault=oom target=hog rec=0.09 ok=1 fixed=0 hog_killed=1 services_lost=[] pid1_state=S

#### results/step2/s6rc-par-wd3/s6rc-par-wd3-hangsP-104.log

| n | shape | arm delay s | fault to kill s | fault to desktop s | windows back | needed a person | watcher state file at the end |
|---|---|---|---|---|---|---|---|
| 1 | deadlock | 0.32 | 36.22 | 40.83 | yes | no | - |
| 2 | spin | 0.24 | 36.25 | 40.85 | yes | no | - |
| 3 | stop | 0.00 | 35.52 | 40.44 | yes | no | - |
| 4 | spin | 0.28 | 36.02 | 41.43 | yes | no | - |
| 5 | stop | 0.00 | 36.21 | 40.50 | yes | no | - |
| 6 | deadlock | 0.26 | 35.32 | 39.99 | yes | no | - |
| 7 | spin | 0.28 | 36.08 | 40.48 | yes | no | - |
| 8 | deadlock | 0.20 | 35.46 | 40.09 | yes | no | - |
| 9 | stop | 0.00 | 35.52 | 39.89 | yes | no | - |

#### results/step2/s6rc-par-wd3/s6rc-par-wd3-storm3-105.log

| n | shape | arm delay s | fault to kill s | fault to desktop s | windows back | needed a person | watcher state file at the end |
|---|---|---|---|---|---|---|---|
| 4 | stop | 0.00 | 35.45 | 40.74 | yes | no | - |
| 5 | spin | 0.07 | 35.49 | 40.47 | yes | no | - |
| 7 | deadlock | 0.38 | 36.03 | 41.06 | yes | no | - |
    CH t=52.8 STORM target=driftwm dur=40 starts=26 starts_last10s=7 cpu_ticks_init_side=44 bar-down
    CH t=105.7 STORM target=driftwm dur=40 starts=26 starts_last10s=6 cpu_ticks_init_side=44 bar-down
    CH t=156.5 STORM target=driftwm dur=40 starts=26 starts_last10s=6 cpu_ticks_init_side=39 bar-down
    CH t=296.1 STORM target=driftwm dur=40 starts=26 starts_last10s=6 cpu_ticks_init_side=42 bar-down

#### results/step2/runit-111-wd3/runit-111-wd3-faults-116.log

| n | shape | arm delay s | fault to kill s | fault to desktop s | windows back | needed a person | watcher state file at the end |
|---|---|---|---|---|---|---|---|
| 19 | stop | 0.00 | 36.21 | 39.98 | yes | no | - |
| 20 | spin | 0.41 | 35.61 | 39.01 | yes | no | - |
| 21 | stop | 0.00 | 35.39 | 39.61 | yes | no | - |
| 22 | spin | 0.08 | 64.14 | 64.41 | yes | YES | - |
| 23 | stop | 0.00 | 60.81 | 64.61 | yes | YES | - |
| 24 | spin | 0.08 | 62.86 | 63.07 | yes | YES | - |
    CH t=298.3 UNRECOVERED ev=22 fault=hang-spin target=driftwm alert=0 why="driftwm does not answer (no answer)"
    CH t=368.0 UNRECOVERED ev=23 fault=hang-stop target=driftwm alert=0 why="driftwm stopped"
    CH t=437.6 UNRECOVERED ev=24 fault=hang-spin target=driftwm alert=0 why="driftwm does not answer (no answer)"
    CH t=487.5 STORM target=hubd dur=40 starts=33 starts_last10s=8 cpu_ticks_init_side=19 bar-up
    CH t=532.0 STORM target=driftwm dur=40 starts=26 starts_last10s=6 cpu_ticks_init_side=43 bar-down
    CH t=748.4 EV ev=49 fault=oom target=hog rec=0.13 ok=1 fixed=0 hog_killed=1 services_lost=[] pid1_state=S
    CH t=757.5 EV ev=50 fault=oom target=hog rec=0.09 ok=1 fixed=0 hog_killed=1 services_lost=[] pid1_state=S

#### results/step2/runit-111-wd3/runit-111-wd3-hangsP-114.log

| n | shape | arm delay s | fault to kill s | fault to desktop s | windows back | needed a person | watcher state file at the end |
|---|---|---|---|---|---|---|---|
| 1 | stop | 0.00 | 36.01 | 40.18 | yes | no | - |
| 2 | deadlock | 0.30 | 35.53 | 38.97 | yes | no | - |
| 3 | spin | 0.47 | 36.23 | 39.80 | yes | no | - |
| 4 | stop | 0.00 | 35.93 | 39.86 | yes | no | - |
| 5 | spin | 0.25 | 35.75 | 39.63 | yes | no | - |
| 6 | deadlock | 0.04 | 35.30 | 38.74 | yes | no | - |
| 7 | spin | 0.25 | 36.81 | 40.88 | yes | no | - |
| 8 | deadlock | 0.16 | 35.68 | 39.08 | yes | no | - |
| 9 | stop | 0.00 | 35.60 | 40.25 | yes | no | - |

#### results/step2/runit-111-wd3/runit-111-wd3-storm3-115.log

| n | shape | arm delay s | fault to kill s | fault to desktop s | windows back | needed a person | watcher state file at the end |
|---|---|---|---|---|---|---|---|
| 4 | stop | 0.00 | 35.79 | 39.72 | yes | no | - |
| 5 | spin | 0.38 | 35.55 | 39.63 | yes | no | - |
| 7 | deadlock | 0.34 | 36.28 | 40.31 | yes | no | - |
    CH t=51.5 STORM target=driftwm dur=40 starts=26 starts_last10s=6 cpu_ticks_init_side=49 bar-down
    CH t=102.5 STORM target=driftwm dur=40 starts=26 starts_last10s=6 cpu_ticks_init_side=46 bar-down
    CH t=153.9 STORM target=driftwm dur=40 starts=26 starts_last10s=6 cpu_ticks_init_side=46 bar-down
    CH t=291.5 STORM target=driftwm dur=40 starts=27 starts_last10s=7 cpu_ticks_init_side=42 bar-down

### Key lines of the latch, state-file, storm and slow tests (grep of the logs; duplicates from the console and standard error copies removed)

#### results/step2/s6rc-par-wd3/s6rc-par-wd3-latch-102.log

```
watcher: KILL driftwm pid=215 start=895
CH t=53.8 LATCH-TEST after hang 1: watcher_state="" budget_entries=1 latch=0
watcher: KILL driftwm pid=718 start=5445
CH t=98.8 LATCH-TEST after hang 2: watcher_state="" budget_entries=2 latch=0
watcher: KILL driftwm pid=1225 start=9961
CH t=142.0 LATCH-TEST after hang 3: watcher_state="" budget_entries=3 latch=0
CH t=145.0 LATCH-TEST hang 4 injected (SIGSTOP of driftwm pid=1721); no help from the harness; watcher_state="" budget_entries=3 latch=0
watcher: LATCH set: the rule wants a kill but 3 kills were made in the last 10 minutes; stopped until a person clears it
CH t=180.3 LATCH-TEST state file after 35.3s: "capped 2026-10-13T19:13:13Z"; latch file=true; driftwm pid=1721 State: T (stopped); budget="hubos-watcher-budget v1 clock=boottime unit=ns | kill 49351662737 | kill 94408407627 | kill 138346495467 | end 3"
CH t=306.1 LATCH-TEST (b) init restarted the watcher in 1.08s (ok=true); after: latch="hubos-watcher-latch v1\nboot_ns 181334406587\nutc 2026-10-13T19:13:13Z\nend" state="capped 2026-10-13T19:13:13Z" budget="hubos-watcher-budget v1 clock=boottime unit=ns | kill 49351662737 | kill 94408407627 | kill 138346495467 | end 3" frozen=t
CH t=1140.9 LATCH-TEST (a) held 961s, 475 samples (every 2 s): not frozen in 0, latch missing in 0, state file missing in 0; uptime now 1147
CH t=1140.9 PERSON-COMMAND *** the harness plays the person: running `watcher reset` (after the hold) ***
CH t=1140.9 PERSON-COMMAND output: "watcher reset: budget emptied, latch and state file removed"; now watcher_state="" budget_entries=0 latch=0
watcher: KILL driftwm pid=1721 start=14343
CH t=1147.7 LATCH-TEST after the person-command: reset to kill 2.13s, reset to desktop back 6.82s (desktop ok=true); state file present=false; watcher_state="" budget_entries=1 latch=0
```

#### results/step2/s6rc-par-wd3/s6rc-par-wd3-latchc-103.log

```
watcher: KILL driftwm pid=214 start=860
CH t=53.0 LATCH-TEST after hang 1: watcher_state="" budget_entries=1 latch=0
watcher: KILL driftwm pid=719 start=5409
CH t=97.1 LATCH-TEST after hang 2: watcher_state="" budget_entries=2 latch=0
watcher: KILL driftwm pid=1210 start=9835
CH t=141.1 LATCH-TEST after hang 3: watcher_state="" budget_entries=3 latch=0
CH t=144.1 LATCH-TEST(c) hang 4 injected (SIGSTOP of driftwm pid=1712); no help from the harness; watcher_state="" budget_entries=3 latch=0
watcher: LATCH set: the rule wants a kill but 3 kills were made in the last 10 minutes; stopped until a person clears it
CH t=180.1 LATCH-TEST(c) state file after 36.1s: "capped 2026-10-13T19:32:31Z"; latch file=true; driftwm pid=1712 State: T (stopped); budget="hubos-watcher-budget v1 clock=boottime unit=ns | kill 48951240189 | kill 93060415069 | kill 136945995589 | end 3"
CH t=190.2 LATCH-TEST(c) still frozen=true, latch=true, state="capped 2026-10-13T19:32:31Z"
CH t=190.2 LATCH-TEST(c) SIGCONT sent; state file removed after 0.05s (present now=false); latch still set=true; compositor State: S (sleeping) pid same=true
CH t=190.5 LATCH-TEST(c) desktop ok=true with the latch still set=true
CH t=195.5 LATCH-TEST(c) hung AGAIN (SIGSTOP of the same process pid=1712); no help
CH t=231.1 LATCH-TEST(c) state file came back after 35.6s: "capped 2026-10-13T19:33:22Z"; latch=true
CH t=317.2 LATCH-TEST(c) 120 s after the second hang: frozen=true (pid same=true, State T (stopped)) state="capped 2026-10-13T19:33:22Z" latch=true; NOT killed=true
CH t=317.2 PERSON-COMMAND *** the harness plays the person: running `watcher reset` (after the second hang) ***
CH t=317.3 PERSON-COMMAND output: "watcher reset: budget emptied, latch and state file removed"; now watcher_state="" budget_entries=0 latch=0
watcher: KILL driftwm pid=1712 start=14202
CH t=327.2 LATCH-TEST after the person-command: reset to kill 4.59s, reset to desktop back 9.92s (desktop ok=true); state file present=false; watcher_state="" budget_entries=1 latch=0
CH t=330.2 LATCH-TEST(d) hang injected (SIGSTOP pid=3727); the watcher will be killed 15 s into the streak; watcher_state="" budget_entries=1 latch=0
CH t=346.3 LATCH-TEST(d) watcher killed at 15.0s after the fault; init restarted it in 1.11s
watcher: KILL driftwm pid=3727 start=32771
CH t=387.2 LATCH-TEST(d) from the watcher's restart to the kill: 36.19s (expect about 30 s); fault to kill 52.32s; fault to desktop back 56.99s (ok=true); watcher_state="" budget_entries=2 latch=0
```

#### results/step2/runit-111-wd3/runit-111-wd3-latch-112.log

```
watcher: KILL driftwm pid=128 start=304
CH t=51.6 LATCH-TEST after hang 1: watcher_state="" budget_entries=1 latch=0
watcher: KILL driftwm pid=694 start=4993
CH t=95.1 LATCH-TEST after hang 2: watcher_state="" budget_entries=2 latch=0
watcher: KILL driftwm pid=1191 start=9410
CH t=138.0 LATCH-TEST after hang 3: watcher_state="" budget_entries=3 latch=0
CH t=141.0 LATCH-TEST hang 4 injected (SIGSTOP of driftwm pid=1687); no help from the harness; watcher_state="" budget_entries=3 latch=0
watcher: LATCH set: the rule wants a kill but 3 kills were made in the last 10 minutes; stopped until a person clears it
CH t=176.3 LATCH-TEST state file after 35.3s: "capped 2026-10-13T20:08:56Z"; latch file=true; driftwm pid=1687 State: T (stopped); budget="hubos-watcher-budget v1 clock=boottime unit=ns | kill 44842631280 | kill 88982277170 | kill 131848859610 | end 3"
CH t=297.0 LATCH-TEST (b) killing the WATCHER (SIGKILL) while latched and frozen; before: latch="hubos-watcher-latch v1\nboot_ns 173861321710\nutc 2026-10-13T20:08:56Z\nend" state="capped 2026-10-13T20:08:56Z" budget="hubos-watcher-budget v1 clock=boottime unit=ns | kill 44842631280 | kill 88982277170 | kill 131848859610 | end 3
CH t=301.2 LATCH-TEST (b) init restarted the watcher in 0.21s (ok=true); after: latch="hubos-watcher-latch v1\nboot_ns 173861321710\nutc 2026-10-13T20:08:56Z\nend" state="capped 2026-10-13T20:08:56Z" budget="hubos-watcher-budget v1 clock=boottime unit=ns | kill 44842631280 | kill 88982277170 | kill 131848859610 | end 3" frozen=t
CH t=1137.7 LATCH-TEST (a) held 961s, 476 samples (every 2 s): not frozen in 0, latch missing in 0, state file missing in 0; uptime now 1140
CH t=1137.7 PERSON-COMMAND *** the harness plays the person: running `watcher reset` (after the hold) ***
CH t=1137.8 PERSON-COMMAND output: "watcher reset: budget emptied, latch and state file removed"; now watcher_state="" budget_entries=0 latch=0
watcher: KILL driftwm pid=1687 start=13692
CH t=1142.3 LATCH-TEST after the person-command: reset to kill 0.38s, reset to desktop back 4.61s (desktop ok=true); state file present=false; watcher_state="" budget_entries=1 latch=0
```

#### results/step2/runit-111-wd3/runit-111-wd3-latchc-113.log

```
watcher: KILL driftwm pid=133 start=361
CH t=51.0 LATCH-TEST after hang 1: watcher_state="" budget_entries=1 latch=0
watcher: KILL driftwm pid=699 start=5045
CH t=94.1 LATCH-TEST after hang 2: watcher_state="" budget_entries=2 latch=0
watcher: KILL driftwm pid=1178 start=9350
CH t=137.2 LATCH-TEST after hang 3: watcher_state="" budget_entries=3 latch=0
CH t=140.2 LATCH-TEST(c) hang 4 injected (SIGSTOP of driftwm pid=1665); no help from the harness; watcher_state="" budget_entries=3 latch=0
watcher: LATCH set: the rule wants a kill but 3 kills were made in the last 10 minutes; stopped until a person clears it
CH t=176.2 LATCH-TEST(c) state file after 36.0s: "capped 2026-10-13T20:28:05Z"; latch file=true; driftwm pid=1665 State: T (stopped); budget="hubos-watcher-budget v1 clock=boottime unit=ns | kill 45359478609 | kill 88409584359 | kill 131378905829 | end 3"
CH t=186.3 LATCH-TEST(c) still frozen=true, latch=true, state="capped 2026-10-13T20:28:05Z"
CH t=186.3 LATCH-TEST(c) SIGCONT sent; state file removed after 0.05s (present now=false); latch still set=true; compositor State: S (sleeping) pid same=true
CH t=186.6 LATCH-TEST(c) desktop ok=true with the latch still set=true
CH t=191.6 LATCH-TEST(c) hung AGAIN (SIGSTOP of the same process pid=1665); no help
CH t=227.3 LATCH-TEST(c) state file came back after 35.6s: "capped 2026-10-13T20:28:56Z"; latch=true
CH t=313.4 LATCH-TEST(c) 120 s after the second hang: frozen=true (pid same=true, State T (stopped)) state="capped 2026-10-13T20:28:56Z" latch=true; NOT killed=true
CH t=313.4 PERSON-COMMAND *** the harness plays the person: running `watcher reset` (after the second hang) ***
CH t=313.4 PERSON-COMMAND output: "watcher reset: budget emptied, latch and state file removed"; now watcher_state="" budget_entries=0 latch=0
watcher: KILL driftwm pid=1665 start=13647
CH t=321.6 LATCH-TEST after the person-command: reset to kill 4.44s, reset to desktop back 8.19s (desktop ok=true); state file present=false; watcher_state="" budget_entries=1 latch=0
CH t=324.6 LATCH-TEST(d) hang injected (SIGSTOP pid=3676); the watcher will be killed 15 s into the streak; watcher_state="" budget_entries=1 latch=0
CH t=339.7 LATCH-TEST(d) watcher killed at 15.0s after the fault; init restarted it in 0.07s
watcher: KILL driftwm pid=3676 start=32106
CH t=379.7 LATCH-TEST(d) from the watcher's restart to the kill: 36.26s (expect about 30 s); fault to kill 51.34s; fault to desktop back 55.17s (ok=true); watcher_state="" budget_entries=2 latch=0
```

#### results/step2/s6rc-par-wd3/s6rc-par-wd3-slow-101.log

```
CH t=12.5 SLOW-TEST variant=both start: driftwm pid=214 watcher_state="" budget_entries=0 latch=0
CH t=72.8 SLOW-TEST variant=both end of the 60 s: driftwm pid=214 (same=true) watcher_state="" budget_entries=0 latch=0
CH t=83.2 SLOW-TEST variant=both 10 s after the end: pid same=true, desktop ok=true (0.1s) watcher_state="" budget_entries=0 latch=0
CH t=93.2 SLOW-TEST variant=sync start: driftwm pid=214 watcher_state="" budget_entries=0 latch=0
CH t=153.6 SLOW-TEST variant=sync end of the 60 s: driftwm pid=214 (same=true) watcher_state="" budget_entries=0 latch=0
CH t=163.9 SLOW-TEST variant=sync 10 s after the end: pid same=true, desktop ok=true (0.1s) watcher_state="" budget_entries=0 latch=0
```

#### results/step2/runit-111-wd3/runit-111-wd3-slow-111.log

```
CH t=10.9 SLOW-TEST variant=both start: driftwm pid=125 watcher_state="" budget_entries=0 latch=0
CH t=71.2 SLOW-TEST variant=both end of the 60 s: driftwm pid=125 (same=true) watcher_state="" budget_entries=0 latch=0
CH t=81.5 SLOW-TEST variant=both 10 s after the end: pid same=true, desktop ok=true (0.1s) watcher_state="" budget_entries=0 latch=0
CH t=91.5 SLOW-TEST variant=sync start: driftwm pid=125 watcher_state="" budget_entries=0 latch=0
CH t=151.7 SLOW-TEST variant=sync end of the 60 s: driftwm pid=125 (same=true) watcher_state="" budget_entries=0 latch=0
CH t=162.0 SLOW-TEST variant=sync 10 s after the end: pid same=true, desktop ok=true (0.1s) watcher_state="" budget_entries=0 latch=0
```

#### results/step2/s6rc-par-wd3/s6rc-par-wd3-storm3-105.log

```
CH t=12.4 STORM-TEST 1 start: watcher_state="" budget_entries=0 latch=0
watcher: SKIP: process changed (recorded none, now pid=929 start=5040): no kill, nothing counted, new streak with the new process
CH t=62.2 STORM-TEST 1 end: watcher_state="" budget_entries=0 latch=0 budget=""
CH t=65.2 STORM-TEST 2 start: watcher_state="" budget_entries=0 latch=0
watcher: SKIP: process changed (recorded none, now none): no kill, nothing counted, new streak with the new process
CH t=113.2 STORM-TEST 2 end: watcher_state="" budget_entries=0 latch=0 budget=""
CH t=116.2 STORM-TEST 3 start: watcher_state="" budget_entries=0 latch=0
CH t=165.0 STORM-TEST 3 end: watcher_state="" budget_entries=0 latch=0 budget=""
CH t=168.0 STORM-TEST two real hangs, then a storm, then a real hang
watcher: KILL driftwm pid=3074 start=16328
watcher: KILL driftwm pid=3609 start=21008
CH t=255.7 STORM-TEST after two real hang kills: watcher_state="" budget_entries=2 latch=0 budget="hubos-watcher-budget v1 clock=boottime unit=ns | kill 204928291421 | kill 248945393541 | end 2"
watcher: SKIP: process changed (recorded pid=4237 start=26253, now none): no kill, nothing counted, new streak with the new process
CH t=304.1 STORM-TEST storm after two real hang kills, end: watcher_state="" budget_entries=2 latch=0 budget="hubos-watcher-budget v1 clock=boottime unit=ns | kill 204928291421 | kill 248945393541 | end 2"
watcher: KILL driftwm pid=5019 start=30262
CH t=348.8 STORM-TEST real hang after the storm: watcher_state="" budget_entries=3 latch=0 budget="hubos-watcher-budget v1 clock=boottime unit=ns | kill 204928291421 | kill 248945393541 | kill 344988803741 | end 3"
```

#### results/step2/runit-111-wd3/runit-111-wd3-storm3-115.log

```
CH t=10.9 STORM-TEST 1 start: watcher_state="" budget_entries=0 latch=0
watcher: SKIP: process changed (recorded pid=313 start=1453, now pid=1042 start=4535): no kill, nothing counted, new streak with the new process
CH t=58.9 STORM-TEST 1 end: watcher_state="" budget_entries=0 latch=0 budget=""
CH t=61.9 STORM-TEST 2 start: watcher_state="" budget_entries=0 latch=0
watcher: SKIP: process changed (recorded pid=1411 start=6556, now none): no kill, nothing counted, new streak with the new process
CH t=110.3 STORM-TEST 2 end: watcher_state="" budget_entries=0 latch=0 budget=""
CH t=113.3 STORM-TEST 3 start: watcher_state="" budget_entries=0 latch=0
watcher: SKIP: process changed (recorded none, now none): no kill, nothing counted, new streak with the new process
CH t=161.8 STORM-TEST 3 end: watcher_state="" budget_entries=0 latch=0 budget=""
CH t=164.8 STORM-TEST two real hangs, then a storm, then a real hang
watcher: KILL driftwm pid=3474 start=15840
watcher: KILL driftwm pid=3995 start=20405
CH t=251.1 STORM-TEST after two real hang kills: watcher_state="" budget_entries=2 latch=0 budget="hubos-watcher-budget v1 clock=boottime unit=ns | kill 198916423865 | kill 242047703935 | end 2"
watcher: SKIP: process changed (recorded pid=4609 start=25462, now pid=5295 start=28450): no kill, nothing counted, new streak with the new process
CH t=299.0 STORM-TEST storm after two real hang kills, end: watcher_state="" budget_entries=2 latch=0 budget="hubos-watcher-budget v1 clock=boottime unit=ns | kill 198916423865 | kill 242047703935 | end 2"
watcher: KILL driftwm pid=5563 start=29542
CH t=342.9 STORM-TEST real hang after the storm: watcher_state="" budget_entries=3 latch=0 budget="hubos-watcher-budget v1 clock=boottime unit=ns | kill 198916423865 | kill 242047703935 | kill 336972009425 | end 3"
```


---

# Follow-up 6 (2026-10-14): Parts B, C, D

Generated by `tools/bench/init/followup6.py` from the logs in `results/step2/s6rc-par-wd4/`, `s6rc-par-wd4a/` and `s6rc-par-confirm/`. Everything above is unchanged. No ranking, no points. Explanations and caveats: `docs/proposals/init-comparison-run.md`, "Follow-up 6".

## Part B: the latch clears itself (s6rc-par-wd4)

| log | setting | state file gone after resume | latch cleared after resume | note |
|---|---|---|---|---|
| clear-full-205.log | 3600 s (default) | 0.21 s | 3600.82 s |  |
| clear-i-201.log | 120 s | 0.20 s | 120.99 s | later hang (budget had room) killed 35.58 s after the fault |
| clear-ii-202.log | 120 s | 0.20 s | 181.7 s (host clock; the harness line is missing from the console log) | sync late from 50.2 s to 60.3 s after the resume |
| clear-iii-203.log | 120 s | 0.21 s | 183.31 s | watcher killed at 60 s, restarted by the init at 61.1 s |
| clear-iv-204.log | 120 s | - | not cleared | after 300 s frozen: latch present=true |

## Part C: restart-limit alert (s6rc-par-wd4a)

| log | starts after the kill, before the stop | time to the stop | alert text | other services kept their process (true/false) | hub-retry | watcher while stopped |
|---|---|---|---|---|---|---|
| alert-storm-driftwm-301.log | 4 | 5.47 s | driftwm stopped: tried 5 times in 60 s | seatd:true,udevd:true,dbus:true,driftwm:false,waybar:false,hubd:false,watcher:true | true after 4.94 s | 149 ticks in 140 s; state file=''; latch=false |
| alert-storm-driftwm-311.log | 4 | 5.39 s | driftwm stopped: tried 5 times in 60 s | seatd:true,udevd:true,dbus:true,driftwm:false,waybar:false,hubd:false,watcher:true | true after 5.27 s | 159 ticks in 140 s; state file=''; latch=false |
| alert-storm-hubd-313.log | 4 | 8.13 s | hubd stopped: tried 5 times in 60 s | seatd:true,udevd:true,dbus:true,driftwm:true,waybar:true,hubd:false,watcher:true | true after 2.15 s | 83 ticks in 55 s; state file=''; latch=false |
| alert-storm-seatd-314.log | 4 | 3.88 s | seatd stopped: tried 5 times in 60 s | seatd:false,udevd:true,dbus:true,driftwm:false,waybar:false,hubd:false,watcher:true | false after 60.23 s | 77 ticks in 70 s; state file=''; latch=false |
| alert-storm-seatd-321.log | 4 | 3.81 s | seatd stopped: tried 5 times in 60 s | seatd:false,udevd:true,dbus:true,driftwm:false,waybar:false,hubd:false,watcher:true | false after 60.11 s; 2nd retry: true after 4.90 s | 79 ticks in 70 s; state file=''; latch=false |
| alert-storm-waybar-312.log | 4 | 8.03 s | waybar stopped: tried 5 times in 60 s | seatd:true,udevd:true,dbus:true,driftwm:true,waybar:false,hubd:true,watcher:true | true after 1.58 s | 99 ticks in 55 s; state file=''; latch=false |

**s6rc-par-wd4a-alert-mustnot-315.log**: t=143.5 MUSTNOT part 1 result after 132s: alert exists=false; svdt driftwm: @400000006acec4cb2241e0d4 signal SIGKILL | @400000006acec4f724546220 signal SIGKILL | @400000006acec52320686bc0 signal SIGKILL; budget="hubos-watcher-budg; t=238.7 MUSTNOT driftwm 4-deaths result: alert exists=true text="driftwm stopped: tried 5 times in 60 s"; svstat="down (exitcode 1) 92 seconds, normally up, ready 92 seconds"; desktop ok=false after 60.0s; svdt: @400000006acec4cb2; t=308.8 MUSTNOT spread 1 of 5: killed driftwm, desktop back ok=false after 60.0s; alert exists=true; t=378.9 MUSTNOT spread 2 of 5: killed driftwm, desktop back ok=false after 60.0s; alert exists=true; t=448.8 MUSTNOT spread 3 of 5: killed driftwm, desktop back ok=false after 60.0s; alert exists=true; t=518.9 MUSTNOT spread 4 of 5: killed driftwm, desktop back ok=false after 60.0s; alert exists=true; t=588.9 MUSTNOT spread 5 of 5: killed driftwm, desktop back ok=false after 60.0s; alert exists=true

**s6rc-par-wd4a-alert-mustnot1-322.log**: t=143.9 MUSTNOT part 1 result after 132s: alert exists=false; desktop why=""; svstat="up (pid 1780 pgid 1780) 7 seconds, ready 5 seconds"; svdt driftwm: @400000006acec7dc320d537c signal SIGKILL | @400000006acec808389da30e signal S

**s6rc-par-wd4a-alert-mustnot2-323.log**: t=27.3 MUSTNOT driftwm 4-deaths result: alert exists=false text=""; svstat="up (pid 477 pgid 477) 10 seconds, ready 8 seconds"; desktop ok=true after 6.0s; svdt: @400000006acec8510bbb30c2 signal SIGKILL | @400000006acec8513a0ea3e6

**s6rc-par-wd4a-alert-mustnot3-324.log**: t=156.7 MUSTNOT spread 3 of 5: killed driftwm, desktop back ok=true after 4.7s; alert exists=false; t=17.3 MUSTNOT spread 1 of 5: killed driftwm, desktop back ok=true after 5.3s; alert exists=false; t=227.3 MUSTNOT spread 4 of 5: killed driftwm, desktop back ok=true after 5.3s; alert exists=false; t=296.6 MUSTNOT spread 5 of 5: killed driftwm, desktop back ok=true after 4.6s; alert exists=false; t=87.3 MUSTNOT spread 2 of 5: killed driftwm, desktop back ok=true after 5.2s; alert exists=false


## Part D: strict trial-boot confirm (s6rc-par-confirm)

| scenario | seed | verdict | injection | after the verdict |
|---|---|---|---|---|
| crash | 402 | ROLLED BACK (60 s); true state 'driftwm not running'; probe rounds run 0 | - |  |
| crash | 407 | ROLLED BACK (60 s); true state 'driftwm not running'; probe rounds run 0 | - |  |
| crash | 412 | ROLLED BACK (60 s); true state 'driftwm not running'; probe rounds run 0 | - |  |
| hang | 403 | ROLLED BACK (60 s); true state 'driftwm stopped'; probe rounds run 5 | wedge injected 2.3 s after first working |  |
| hang | 408 | ROLLED BACK (60 s); true state 'driftwm stopped'; probe rounds run 5 | wedge injected 2.2 s after first working |  |
| hang | 413 | ROLLED BACK (60 s); true state 'driftwm stopped'; probe rounds run 5 | wedge injected 2.3 s after first working |  |
| hang30 | 405 | CONFIRMED at 26.9 s (plain condition at 22.3 s, strict added 4.6 s) | wedge injected 30.0 s after first working | true state 3 s after a late wedge: 'driftwm stopped' |
| hang30 | 410 | CONFIRMED at 26.8 s (plain condition at 22.3 s, strict added 4.5 s) | wedge injected 30.2 s after first working | true state 3 s after a late wedge: 'driftwm stopped' |
| hang30 | 415 | CONFIRMED at 27.2 s (plain condition at 22.6 s, strict added 4.6 s) | wedge injected 30.2 s after first working | true state 3 s after a late wedge: 'driftwm stopped' |
| hang6 | 404 | ROLLED BACK (60 s); true state 'driftwm stopped'; probe rounds run 5 | wedge injected 6.9 s after first working |  |
| hang6 | 409 | ROLLED BACK (60 s); true state 'driftwm stopped'; probe rounds run 5 | wedge injected 6.7 s after first working |  |
| hang6 | 414 | ROLLED BACK (60 s); true state 'driftwm stopped'; probe rounds run 5 | wedge injected 6.8 s after first working |  |
| healthy | 401 | CONFIRMED at 27.0 s (plain condition at 22.5 s, strict added 4.5 s) | - |  |
| healthy | 406 | CONFIRMED at 27.1 s (plain condition at 22.5 s, strict added 4.5 s) | - |  |
| healthy | 411 | CONFIRMED at 26.6 s (plain condition at 22.2 s, strict added 4.5 s) | - |  |


---

# Follow-up 7 (2026-10-14): Parts A, B, C

Generated by `tools/bench/init/followup6.py` from the logs in `results/step2/s6rc-par-wd4a/`, `s6rc-par-wd4b/` and `s6rc-par-wd4c/`. Everything above is unchanged. No ranking, no points. Explanations: `docs/proposals/init-comparison-run.md`, "Follow-up 7".

## Follow-up 7, Part A: clock jumps on the death tally (s6rc-par-wd4a, each case in a fresh VM)

| case | what | stopped | starts after the first kill | alert text | span of the last 5 deaths in the tally at the end of the run (c3: 126 s at the 5th death) | span on the boot clock (first kill to last death or start seen) |
|---|---|---|---|---|---|---|
| c1 | 4 deaths, +600 s, 1 more death | no | 5 |  | 617 s | 17.4 s |
| c2 | 5 deaths, -3600 s after the 2nd start | yes | 4 | driftwm stopped: tried 5 times in 60 s | -3594 s | 5.3 s |
| c3 | 5 deaths, +120 s after the 2nd start | yes | 6 | driftwm stopped: tried 5 times in 60 s | 7 s | 8.4 s |
| c4 | control, no jump | yes | 4 | driftwm stopped: tried 5 times in 60 s | 6 s | 5.5 s |

## Follow-up 7, Part C: one hub-retry also retries dependents (s6rc-par-wd4b)

- RETRY7 harmless-1 (waybar, no alert, no dependents): `/bin/hub-retry waybar` took 0.09s; output: "hub-retry: nothing to retry: waybar has no alert and none of its dependents has one. State: up (pid 238 pgid 238) 7 seconds, ready 5 seconds\nexit=0"; alert files: []; pids: seatd=180 udevd=184 dbus=188 driftwm=215 waybar=248 hubd=249 watcher=271
- RETRY7 harmless-2 (unknown name): `/bin/hub-retry nosuchthing` took 0.02s; output: "hub-retry: 'nosuchthing' is not a service I know\nexit=2"; alert files: []; pids: seatd=180 udevd=184 dbus=188 driftwm=215 waybar=248 hubd=249 watcher=271
- RETRY7 harmless-3 (no argument): `/bin/hub-retry` took 0.03s; output: "usage: hub-retry SERVICE\nexit=2"; alert files: []; pids: seatd=180 udevd=184 dbus=188 driftwm=215 waybar=248 hubd=249 watcher=271
- RETRY7 seatd storm over: starts=4; alert files BEFORE the retry: [driftwm,seatd] (seatd: "seatd stopped: tried 5 times in 60 s", driftwm: "driftwm stopped: tried 5 times in 60 s"); svstat: seatd: down (exitcode 1) 36 seconds, normally up, ready 36 seconds | udevd: up (pid 184 pgid 184) 51 seconds, ready 50 seconds | dbus: up (pid 188 pgid 188) 51 seconds, ready 51 seconds | driftwm: down (exitcode 1) 34 seconds, normally up, ready 34 seconds | waybar: up (pid 549 pgid 549) 34 seconds | hubd: up (pid 578 pgid 578) 33 seconds; why="seatd not running"
- RETRY7 dependent-with-stopped-parent (driftwm, 1st call): `/bin/hub-retry driftwm` took 0.05s; output: "hub-retry: driftwm waits for seatd, which is stopped: seatd stopped: tried 5 times in 60 s. Nothing done. Run: hub-retry seatd\nexit=1"; alert files: [driftwm,seatd]; pids: seatd=0 udevd=184 dbus=188 driftwm=0 waybar=0 hubd=0 watcher=271
- RETRY7 dependent-with-stopped-parent (driftwm, 2nd call): `/bin/hub-retry driftwm` took 0.05s; output: "hub-retry: driftwm waits for seatd, which is stopped: seatd stopped: tried 5 times in 60 s. Nothing done. Run: hub-retry seatd\nexit=1"; alert files: [driftwm,seatd]; pids: seatd=0 udevd=184 dbus=188 driftwm=0 waybar=0 hubd=0 watcher=271
- RETRY7 the one command (seatd): `/bin/hub-retry seatd` took 2.19s; output: "hub-retry: starting seatd (seatd stopped: tried 5 times in 60 s)\nhub-retry: starting driftwm (driftwm stopped: tried 5 times in 60 s)\nhub-retry: seatd is up\nhub-retry: driftwm is up\nexit=0"; alert files: []; pids: seatd=964 udevd=184 dbus=188 driftwm=976 waybar=0 hubd=0 watcher=271
- RETRY7 RESULT: the command returned after 2.19s; desktop ok=true 2.43s after it returned (4.91s after it started); alert files AFTER: []; pids seatd=964 udevd=184 dbus=188 driftwm=976 waybar=1057 hubd=1045 watcher=271; svstat: seatd: up (pid 964 pgid 964) 4 seconds, ready 4 seconds | udevd: up (pid 184 pgid 184) 57 seconds, ready 55 seconds | dbus: up (pid 188 pgid 188) 57 seconds, ready 57 seconds | driftwm: up (pid 976 pgid 976) 4 seconds, ready 2 seconds | waybar: up (pid 1040 pgid 1040) 1 seconds | hubd: up (pid 1035 pgid 1035) 2 seconds, ready 0 seconds
- RETRY7 harmless-4 (seatd again, nothing stopped now): `/bin/hub-retry seatd` took 0.07s; output: "hub-retry: nothing to retry: seatd has no alert and none of its dependents has one. State: up (pid 964 pgid 964) 5 seconds, ready 4 seconds\nexit=0"; alert files: []; pids: seatd=964 udevd=184 dbus=188 driftwm=976 waybar=1057 hubd=1045 watcher=271

## Follow-up 7, Part B: hubd and the stopped compositor (s6rc-par-wd4c)

**s6rc-par-wd4c-alert-followB-521.log**

- FOLLOWB start: pids seatd=184 udevd=183 dbus=188 driftwm=214 waybar=247 hubd=238 watcher=266; hubd ping="ok"; alert files []; marker exists=false
- FOLLOWB 1. driftwm stopped by the limit after 5.40s; alert="driftwm stopped: tried 5 times in 60 s"; marker exists=false
- FOLLOWB 2. 20 s later: hubd pid 238 -> 238 (same=true) state S (sleeping); hubd ping="stale"; waybar pid 247 -> 0; svstat of hubd: up (pid 238 pgid 238) 32 seconds, ready 31 seconds; hubd log tail: hubd: heartbeat pid=238 seq=58 | hubd: heartbeat pid=238 seq=59 | hubd: heartbeat pid=238 seq=60 | hubd: heartbeat pid=238 seq=61 | hubd: heartbeat pid=238 seq=62 | hubd: heartbeat pid=238 seq=63
- FOLLOWB 3. hubd killed while driftwm is stopped, NO marker: 15 s later hubd pid=0 (0 = none); svstat: up (pid 580 pgid 580) 15 seconds; hubd log tail: hubd: heartbeat pid=238 seq=59 | hubd: heartbeat pid=238 seq=60 | hubd: heartbeat pid=238 seq=61 | hubd: heartbeat pid=238 seq=62 | hubd: heartbeat pid=238 seq=63 | restart-limit: hubd death 1 in the tally (the limit looks at the last 5)
- FOLLOWB 4. the harness writes /run/hub/driftwm-gave-up (standing in for the image's driftwm finish script)
- FOLLOWB 4. 4 s later: hubd pid=0; svstat: up (pid 812 pgid 812) 0 seconds; hubd log tail: follow-driftwm: the compositor was stopped for good (5 crashes in a minute); running policy without it | hubd: starting pid 786 | hubd: no compositor: dial unix /run/dw/wayland-1: connect: connection refused | follow-driftwm: policy ended | restart-limit: hubd death 4 in the tally (the limit looks at the last 5) | follow-driftwm: the compositor was stopped for good (5 crashes in a minute); running policy without it
- FOLLOWB 5. marker removed (the image's driftwm run script does it); hub-retry driftwm
- FOLLOWB 5. hub-retry output=""; driftwm pid=838; desktop ok=false after 40.45s (why="hubd not running"); pids before seatd=184 udevd=183 dbus=188 driftwm=0 waybar=0 hubd=0 watcher=266 / after seatd=184 udevd=183 dbus=188 driftwm=838 waybar=886 hubd=0 watcher=266; hubd ping="dial unix /run/hubos/hubd.sock: connect: connection refused"; alert files [hubd]; hubd log tail: follow-driftwm: the compositor was stopped for good (5 crashes in a minute); running policy without it | hubd: starting pid 815 | hubd: no compositor: dial unix /run/dw/wayland-1: connect: connection refused | follow-driftwm: policy ended | restart-limit: hubd: last 5 deaths span 19 s | restart-limit: hubd stopped for good (al
**s6rc-par-wd4c-alert-followB2-522.log**

- FOLLOWB start: pids seatd=179 udevd=183 dbus=189 driftwm=215 waybar=248 hubd=239 watcher=266; hubd ping="ok"; alert files []; marker exists=false
- FOLLOWB 1. driftwm stopped by the limit after 5.36s; alert="driftwm stopped: tried 5 times in 60 s"; marker exists=false
- FOLLOWB 2. 20 s later: hubd pid 239 -> 239 (same=true) state S (sleeping); hubd ping="stale"; waybar pid 248 -> 0; svstat of hubd: up (pid 239 pgid 239) 32 seconds, ready 30 seconds; hubd log tail: hubd: heartbeat pid=239 seq=57 | hubd: heartbeat pid=239 seq=58 | hubd: heartbeat pid=239 seq=59 | hubd: heartbeat pid=239 seq=60 | hubd: heartbeat pid=239 seq=61 | hubd: heartbeat pid=239 seq=62
- FOLLOWB2 hub-retry driftwm output=""; driftwm pid=575; desktop ok=false after 40.24s (why="clients missing: ok clients=waybar"); hubd pid 239 -> 239 (same=true); hubd ping="stale"; pids before seatd=179 udevd=183 dbus=189 driftwm=0 waybar=0 hubd=239 watcher=266 / after seatd=179 udevd=183 dbus=189 driftwm=575 waybar=614 hubd=239 watcher=266; alert files []; hubd log tail: hubd: heartbeat pid=239 seq=138 | hubd: heartbeat pid=239 seq=139 | hubd: heartbeat pid=239 seq=140 | hubd: heartbeat pid=239 seq=141 | hubd: heartbeat pid=239 seq=142 | hubd: heartbeat pid=239 seq=143
