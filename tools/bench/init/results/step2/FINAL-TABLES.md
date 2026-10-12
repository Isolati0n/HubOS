# Final tables of the elimination round (all candidates), regenerated 2026-10-12 with tools/bench/init/report.py

Notes: (1) a step that appears in several cycles because the machine rebooted itself (runit) is counted once, from its first cycle; runit's logs are merged (first run, rerun from step 31, rerun from step 38). (2) s6plain is the full repeat run (`s6plain-repeat/`). (3) The boot rows of openrc-par and openrc-doc are the re-measurement (`openrc-remeasure/`) made after the memory counter was fixed; the earlier OpenRC memory figures (586, 548 and, for the first openrc run, 664 kB) held PID 1 only. (4) `openrc-doc2` has only trial boots. (5) For the event counts of each column and why they differ, see the run log (follow-up 2, item 1). (6) "alert=yes/no" in the storm rows is the harness's own alert file.

### Boot, memory, idle CPU

| candidate | boots | supervisor start to desktop, s: median (min-max) | PSS of PID 1 + supervisors + policy layer, kB | init-side CPU ticks in 30 s idle | whole VM busy / idle ticks in 30 s (2 vCPU) |
|---|---|---|---|---|---|
| s6plain | 3 | 5.8 (5.8-5.8) | 2466 | 0 | 447 / 5445 |
| s6rc | 3 | 6.4 (6.3-6.4) | 2670 | 1 | 214 / 5674 |
| s6rc-par | 3 | 7.9 (7.6-8.0) | 2674 | 1 | 571 / 5330 |
| runit | 3 | 6.1 (6.1-6.3) | 1880 | 1 | 568 / 5283 |
| runit-111 | 3 | 6.2 (5.9-6.2) | 1907 | 1 | 491 / 5357 |
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

### PID 1 killed or wedged

| candidate | PID 1 killed: panic to desktop again, s | PID 1 wedged: injection to desktop again, s (watchdog 30 s) |
|---|---|---|
| s6plain | 17, 17 | 46 (guard noticed after 2) |
| s6rc | 20, 20 | 51 (guard noticed after nan) |
| s6rc-par | 21, 20 | 49 (guard noticed after nan) |
| runit | 16, 15 | 48 (guard noticed after 2) |
| runit-111 | 17, 16 | 47 (guard noticed after 2) |
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

### Update trial boot

| candidate | trial: healthy | trial: driftwm cannot start | trial: driftwm wedged 2 s after it first worked |
|---|---|---|---|
| s6plain | CONFIRMED after 15.9 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 16.0 s (FALSE: driftwm stopped) |
| s6rc | CONFIRMED after 22.3 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 21.8 s (FALSE: driftwm stopped) |
| s6rc-par | CONFIRMED after 23.1 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 22.7 s (FALSE: driftwm stopped) |
| runit | CONFIRMED after 19.8 s | CONFIRMED after 19.4 s (FALSE: waybar not running) | CONFIRMED after 19.6 s (FALSE: driftwm stopped) |
| runit-111 | CONFIRMED after 19.3 s | CONFIRMED after 19.4 s (FALSE: driftwm not running) | CONFIRMED after 19.7 s (FALSE: driftwm stopped) |
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

### Fault injection

| fault: median recovery s (worst) | s6plain | s6rc | s6rc-par | runit | runit-111 | b-go | c-go | c-elixir | dinit | dinit-par | dinit-sm | dinit-ms | dinit-wf | openrc | openrc-par | openrc-doc | openrc-doc2 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| kill (any of 6) | 2.8 (max 6.5) | 0.6 (max 1.9); 3/15 needed a person | 1.9 (max 5.0) | 1.9 (max 4.2) | 1.8 (max 4.2) | 3.6 (max 6.7) | 3.1 (max 8.9) | 2.8 (max 11.1) | 0.8 (max 5.7) | 2.0 (max 6.9) | 1.7 (max 2.3) | 1.8 (max 3.9) | 1.8 (max 4.0) | 1.9 (max 2.9); 6/18 needed a person | 3.1 (max 7.2) | 2.8 (max 17.7) | n/a |
| hang-stop | -; 3/3 needed a person | n/a | -; 3/3 needed a person | -; 3/3 needed a person | -; 3/3 needed a person | 10.9 (max 15.1) | 9.7 (max 13.7) | 14.2 (max 15.4) | -; 3/3 needed a person | -; 3/3 needed a person | n/a | -; 3/3 needed a person | -; 3/3 needed a person | -; 3/3 needed a person | -; 3/3 needed a person | 20.1 (max 20.2) | n/a |
| hang-spin | 0.1 (max 0.1) | 0.1 (max 0.1) | 0.1 (max 0.1); 2/3 needed a person | 0.1 (max 0.1); 1/3 needed a person | -; 3/3 needed a person | 0.1 (max 0.2) | 12.8 (max 14.0) | 14.4 (max 14.4) | -; 3/3 needed a person | 0.1 (max 0.1); 1/3 needed a person | 0.0 (max 0.0) | 0.1 (max 0.1) | 0.1 (max 0.1) | 0.2 (max 0.2); 1/3 needed a person | 0.1 (max 15.7) | 0.1 (max 20.3) | n/a |
| dep-slow | 21.1 (max 21.1) | n/a | 20.4 (max 20.4) | 19.7 (max 19.7) | 18.8 (max 18.8) | 18.9 (max 18.9) | 18.9 (max 18.9) | 20.5 (max 20.5) | 32.9 (max 32.9) | 33.9 (max 33.9) | n/a | n/a | 18.9 (max 18.9) | -; 1/1 needed a person | 22.6 (max 22.6) | 21.5 (max 21.5) | n/a |
| dep-fail | 5.7 (max 5.7) | n/a | 4.7 (max 4.7) | 5.3 (max 5.3) | 3.9 (max 3.9) | 3.9 (max 3.9) | 3.9 (max 3.9) | 6.3 (max 6.3) | n/a | n/a | n/a | n/a | n/a | n/a | -; 1/1 needed a person | 6.1 (max 6.1) | n/a |
| disk-full | 0.1 (max 0.1) | n/a | 0.2 (max 0.2) | 0.2 (max 0.2) | 0.1 (max 0.1) | 0.1 (max 0.1) | 0.2 (max 0.2) | 0.2 (max 0.2) | n/a | n/a | n/a | 0.1 (max 0.1) | 0.1 (max 0.1) | 0.1 (max 0.1) | 3.4 (max 6.8) | 0.1 (max 0.1) | n/a |
| supervisor-killed | 0.9 (max 16.6) | n/a | 4.6 (max 18.1) | 0.2 (max 0.3) | 0.1 (max 5.4) | n/a | 0.2 (max 7.5); 1/5 needed a person | 0.2 (max 17.8); 1/5 needed a person | n/a | n/a | n/a | n/a | n/a | 0.1 (max 0.1) | 0.1 (max 0.1) | 0.1 (max 0.1) | n/a |
| scanner-killed | n/a | n/a | n/a | n/a | 0.1 (max 0.1) | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a |
| brain-killed | n/a | n/a | n/a | n/a | n/a | 0.1 (max 0.1) | 0.2 (max 0.2) | 0.1 (max 0.2) | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a |
| clock-3600 | 6.4 (max 6.4) | n/a | 4.7 (max 4.7) | 5.0 (max 5.0) | 0.1 (max 0.1) | 3.4 (max 3.4) | 3.4 (max 3.4) | 5.7 (max 5.7) | n/a | n/a | n/a | 3.9 (max 3.9) | 3.7 (max 3.7) | -; 1/1 needed a person | 6.5 (max 6.5) | 5.8 (max 5.8) | n/a |
| clock+86400 | 6.0 (max 6.0) | n/a | 4.5 (max 4.5) | 4.0 (max 4.0) | 3.2 (max 3.2) | 4.7 (max 4.7) | 0.2 (max 0.2) | 3.2 (max 3.2) | n/a | n/a | n/a | 3.6 (max 3.6) | 3.9 (max 3.9) | -; 1/1 needed a person | 7.3 (max 7.3) | 6.0 (max 6.0) | n/a |
| oom | 0.1 (max 0.1) | n/a | 0.1 (max 0.1) | 0.1 (max 0.1) | 0.1 (max 0.1) | 0.1 (max 0.1) | 0.2 (max 0.2) | 0.1 (max 0.1) | n/a | n/a | n/a | 0.1 (max 0.1) | 0.1 (max 0.1) | 0.1 (max 0.1) | 0.1 (max 0.1) | 0.1 (max 0.1) | n/a |

| measure | s6plain | s6rc | s6rc-par | runit | runit-111 | b-go | c-go | c-elixir | dinit | dinit-par | dinit-sm | dinit-ms | dinit-wf | openrc | openrc-par | openrc-doc | openrc-doc2 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| crash storm on hubd, 40 s | 18 starts; 5 in last 10 s; alert=no | 35 starts; 8 in last 10 s; alert=no | 18 starts; 5 in last 10 s; alert=no | 33 starts; 9 in last 10 s; alert=no | 33 starts; 8 in last 10 s; alert=no | 38 starts; 10 in last 10 s; alert=no | 17 starts; 2 in last 10 s; alert=yes | 21 starts; 2 in last 10 s; alert=yes | 1 starts; 0 in last 10 s; alert=no | 1 starts; 0 in last 10 s; alert=no | n/a | 1 starts; 0 in last 10 s; alert=no | 1 starts; 0 in last 10 s; alert=no | 10 starts; 0 in last 10 s; alert=no | 8 starts; 0 in last 10 s; alert=no | 16 starts; 5 in last 10 s; alert=no | n/a |
| crash storm on driftwm, 40 s | 26 starts; 7 in last 10 s; alert=no | 26 starts; 6 in last 10 s; alert=no | 26 starts; 6 in last 10 s; alert=no | 26 starts; 7 in last 10 s; alert=no | 27 starts; 7 in last 10 s; alert=no | 39 starts; 10 in last 10 s; alert=no | 11 starts; 2 in last 10 s; alert=yes | 10 starts; 1 in last 10 s; alert=yes | 1 starts; 0 in last 10 s; alert=no | 1 starts; 0 in last 10 s; alert=no | n/a | 1 starts; 0 in last 10 s; alert=no | 1 starts; 0 in last 10 s; alert=no | 10 starts; 0 in last 10 s; alert=no | 11 starts; 0 in last 10 s; alert=no | 20 starts; 5 in last 10 s; alert=no | n/a |
| seatd slow: starts of driftwm / waybar / hubd while waiting | 11 / 6 / 2 | n/a | 11 / 7 / 3 | 11 / 8 / 4 | 11 / 7 / 2 | 1 / 1 / 1 | 2 / 10 / 1 | 3 / 19 / 1 | 1 / 1 / 1 | 1 / 1 / 1 | n/a | n/a | 1 / 1 / 1 | 9 / 11 / 1 | 8 / 5 / 2 | 9 / 6 / 1 | n/a |
| seatd fail: starts of driftwm / waybar / hubd while waiting | 15 / 7 / 2 | n/a | 14 / 7 / 2 | 15 / 12 / 2 | 15 / 11 / 2 | 1 / 1 / 1 | 2 / 23 / 1 | 3 / 2 / 1 | n/a | n/a | n/a | n/a | n/a | n/a | 3 / 3 / 2 | 11 / 7 / 2 | n/a |

| unrecovered by itself, count | s6plain | s6rc | s6rc-par | runit | runit-111 | b-go | c-go | c-elixir | dinit | dinit-par | dinit-sm | dinit-ms | dinit-wf | openrc | openrc-par | openrc-doc | openrc-doc2 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| after-clock-3600 (alert=0) | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| after-supervisor-killed (alert=0) | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 6 | 6 | 6 | 0 |
| clock+86400 (alert=0) | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 |
| clock-3600 (alert=0) | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 |
| dep-fail (alert=0) | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 1 | 0 | 1 | 1 | 1 | 1 | 0 | 0 |
| dep-slow (alert=0) | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 1 | 0 | 0 | 0 |
| hang-spin (alert=0) | 0 | 0 | 2 | 1 | 3 | 0 | 0 | 0 | 3 | 1 | 0 | 1 | 0 | 1 | 0 | 0 | 0 |
| hang-stop (alert=0) | 3 | 3 | 3 | 3 | 3 | 0 | 0 | 0 | 3 | 3 | 2 | 3 | 3 | 3 | 3 | 0 | 0 |
| kill (alert=0) | 0 | 6 | 0 | 0 | 0 | 0 | 0 | 0 | 5 | 5 | 6 | 0 | 0 | 6 | 0 | 0 | 0 |
| kill (alert=1) | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| storm (alert=0) | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 2 | 2 | 0 | 2 | 2 | 2 | 2 | 0 | 0 |
| storm (alert=1) | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| supervisor-killed (alert=1) | 0 | 0 | 0 | 0 | 0 | 0 | 2 | 2 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
