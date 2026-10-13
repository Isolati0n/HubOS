# Final tables of the elimination round (all candidates), regenerated 2026-10-12 (follow-up 3) with tools/bench/init/report.py Follow-up 4 (2026-10-13): the rows for `s6rc-par-wd2` and `runit-111-wd2` and the hang tables are appended at the end of this file (generated with report.py, event_counts.py and hangtable.py; the earlier tables above are unchanged, the rows of `s6rc-par-wd`, `runit-wd`, `s6rc-par-wdc`, `runit-wdc` are the "compositor + hubd probes" and "one compositor probe only" runs, not the owner's rule).

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
