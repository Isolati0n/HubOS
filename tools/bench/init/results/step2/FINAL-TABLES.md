### Boot, memory, idle CPU

| candidate | boots | supervisor start to desktop, s: median (min-max) | PSS of PID 1 + supervisors + policy layer, kB | init-side CPU ticks in 30 s idle | whole VM busy / idle ticks in 30 s (2 vCPU) |
|---|---|---|---|---|---|
| s6plain | 3 | 6.1 (6.0-6.2) | 2454 | 1 | 512 / 5372 |
| s6rc-par | 3 | 7.9 (7.6-8.0) | 2674 | 1 | 571 / 5330 |
| runit | 3 | 6.1 (6.1-6.3) | 1880 | 1 | 568 / 5283 |
| b-go | 3 | 7.3 (7.2-7.4) | 5028 | 29 | 204 / 5667 |
| c-go | 3 | 6.4 (6.3-6.7) | 10962 | 267 | 699 / 5059 |
| c-elixir | 3 | 6.3 (6.2-6.5) | 35574 | 234 | 629 / 5158 |
| dinit-par | 3 | 6.0 (6.0-6.1) | 2096 | 12 | 461 / 5392 |
| dinit-ms | 3 | 6.2 (6.1-6.3) | 2092 | 13 | 481 / 5379 |
| openrc-par | 3 | 9.2 (9.2-9.7) | 586 | 1 | 784 / 5029 |
| openrc-doc | 3 | 8.9 (8.9-8.9) | 548 | 1 | 742 / 5085 |
| openrc-doc2 | 0 | n/a (n/a-n/a) | n/a | n/a | n/a / n/a |

### PID 1 killed or wedged

| candidate | PID 1 killed: panic to desktop again, s | PID 1 wedged: injection to desktop again, s (watchdog 30 s) |
|---|---|---|
| s6plain | 18, 17 | 47 (guard noticed after 2) |
| s6rc-par | 21, 20 | 49 (guard noticed after nan) |
| runit | 16, 15 | 48 (guard noticed after 2) |
| b-go | 18, 18 | 46 (guard noticed after 0) |
| c-go | 20, 21 | 51 (guard noticed after nan) |
| c-elixir | 21, 22 | 52 (guard noticed after nan) |
| dinit-par | 15, 15 | 45 (guard noticed after nan) |
| dinit-ms | 16, 16 | 46 (guard noticed after nan) |
| openrc-par | 21, 27 | 51 (guard noticed after 1) |
| openrc-doc | 20, 20 | 50 (guard noticed after 1) |
| openrc-doc2 | n/a | n/a |

### Update trial boot

| candidate | trial: healthy | trial: driftwm cannot start | trial: driftwm wedged 2 s after it first worked |
|---|---|---|---|
| s6plain | CONFIRMED after 16.6 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 15.9 s (FALSE: driftwm stopped) |
| s6rc-par | CONFIRMED after 23.1 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 22.7 s (FALSE: driftwm stopped) |
| runit | CONFIRMED after 19.8 s | CONFIRMED after 19.4 s (FALSE: waybar not running) | CONFIRMED after 19.6 s (FALSE: driftwm stopped) |
| b-go | CONFIRMED after 23.5 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 37.5 s |
| c-go | CONFIRMED after 23.3 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 41.7 s |
| c-elixir | CONFIRMED after 28.9 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 32.6 s |
| dinit-par | CONFIRMED after 21.2 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 20.8 s (FALSE: driftwm stopped) |
| dinit-ms | CONFIRMED after 21.4 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 21.2 s (FALSE: driftwm stopped) |
| openrc-par | ROLLBACK reboot at 60 s, guard kept feeding | ROLLBACK reboot at 60 s, guard kept feeding | ROLLBACK reboot at 60 s, guard kept feeding |
| openrc-doc | ROLLBACK reboot at 60 s, guard kept feeding | ROLLBACK reboot at 60 s, guard kept feeding | ROLLBACK reboot at 60 s, guard kept feeding |
| openrc-doc2 | CONFIRMED after 27.0 s | ROLLBACK reboot at 60 s, guard kept feeding | CONFIRMED after 27.1 s (FALSE: driftwm not running) |

### Fault injection

| fault: median recovery s (worst) | s6plain | s6rc-par | runit | b-go | c-go | c-elixir | dinit-par | dinit-ms | openrc-par | openrc-doc | openrc-doc2 |
|---|---|---|---|---|---|---|---|---|---|---|---|
| kill (any of 6) | 2.9 (max 6.4) | 1.9 (max 5.0) | 1.9 (max 4.3) | 3.6 (max 6.7) | 3.1 (max 8.9) | 2.8 (max 11.1) | 2.0 (max 6.9) | 1.8 (max 3.9) | 3.1 (max 7.2) | 2.8 (max 17.7) | n/a |
| hang-stop | -; 3/3 needed a person | -; 3/3 needed a person | -; 6/6 needed a person | 10.9 (max 15.1) | 9.7 (max 13.7) | 14.2 (max 15.4) | -; 3/3 needed a person | -; 3/3 needed a person | -; 3/3 needed a person | 20.1 (max 20.2) | n/a |
| hang-spin | -; 3/3 needed a person | 0.1 (max 0.1); 2/3 needed a person | 0.1 (max 0.1); 3/6 needed a person | 0.1 (max 0.2) | 12.8 (max 14.0) | 14.4 (max 14.4) | 0.1 (max 0.1); 1/3 needed a person | 0.1 (max 0.1) | 0.1 (max 15.7) | 0.1 (max 20.3) | n/a |
| dep-slow | 21.5 (max 21.5) | 20.4 (max 20.4) | 19.4 (max 19.7) | 18.9 (max 18.9) | 18.9 (max 18.9) | 20.5 (max 20.5) | 33.9 (max 33.9) | n/a | 22.6 (max 22.6) | 21.5 (max 21.5) | n/a |
| dep-fail | 6.0 (max 6.0) | 4.7 (max 4.7) | 5.0 (max 5.3) | 3.9 (max 3.9) | 3.9 (max 3.9) | 6.3 (max 6.3) | n/a | n/a | -; 1/1 needed a person | 6.1 (max 6.1) | n/a |
| disk-full | 0.2 (max 0.2) | 0.2 (max 0.2) | 0.2 (max 0.2) | 0.1 (max 0.1) | 0.2 (max 0.2) | 0.2 (max 0.2) | n/a | 0.1 (max 0.1) | 3.4 (max 6.8) | 0.1 (max 0.1) | n/a |
| supervisor-killed | 4.8 (max 25.4) | 4.6 (max 18.1) | 0.2 (max 0.3) | n/a | 0.2 (max 7.5); 1/5 needed a person | 0.2 (max 17.8); 1/5 needed a person | n/a | n/a | 0.1 (max 0.1) | 0.1 (max 0.1) | n/a |
| scanner-killed | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a |
| brain-killed | n/a | n/a | n/a | 0.1 (max 0.1) | 0.2 (max 0.2) | 0.1 (max 0.2) | n/a | n/a | n/a | n/a | n/a |
| clock-3600 | n/a | 4.7 (max 4.7) | n/a | 3.4 (max 3.4) | 3.4 (max 3.4) | 5.7 (max 5.7) | n/a | 3.9 (max 3.9) | 6.5 (max 6.5) | 5.8 (max 5.8) | n/a |
| clock+86400 | n/a | 4.5 (max 4.5) | n/a | 4.7 (max 4.7) | 0.2 (max 0.2) | 3.2 (max 3.2) | n/a | 3.6 (max 3.6) | 7.3 (max 7.3) | 6.0 (max 6.0) | n/a |
| oom | n/a | 0.1 (max 0.1) | n/a | 0.1 (max 0.1) | 0.2 (max 0.2) | 0.1 (max 0.1) | n/a | 0.1 (max 0.1) | 0.1 (max 0.1) | 0.1 (max 0.1) | n/a |

| measure | s6plain | s6rc-par | runit | b-go | c-go | c-elixir | dinit-par | dinit-ms | openrc-par | openrc-doc | openrc-doc2 |
|---|---|---|---|---|---|---|---|---|---|---|---|
| crash storm on hubd, 40 s | 18 starts; 5 in last 10 s; alert=no | 18 starts; 5 in last 10 s; alert=no | 33 starts; 9 in last 10 s; alert=no | 38 starts; 10 in last 10 s; alert=no | 17 starts; 2 in last 10 s; alert=yes | 21 starts; 2 in last 10 s; alert=yes | 1 starts; 0 in last 10 s; alert=no | 1 starts; 0 in last 10 s; alert=no | 8 starts; 0 in last 10 s; alert=no | 16 starts; 5 in last 10 s; alert=no | n/a |
| crash storm on driftwm, 40 s | 25 starts; 6 in last 10 s; alert=no | 26 starts; 6 in last 10 s; alert=no | 26 starts; 7 in last 10 s; alert=no | 39 starts; 10 in last 10 s; alert=no | 11 starts; 2 in last 10 s; alert=yes | 10 starts; 1 in last 10 s; alert=yes | 1 starts; 0 in last 10 s; alert=no | 1 starts; 0 in last 10 s; alert=no | 11 starts; 0 in last 10 s; alert=no | 20 starts; 5 in last 10 s; alert=no | n/a |
| seatd slow: starts of driftwm / waybar / hubd while waiting | 11 / 7 / 3 | 11 / 7 / 3 | 11 / 8 / 4 | 1 / 1 / 1 | 2 / 10 / 1 | 3 / 19 / 1 | 1 / 1 / 1 | n/a | 8 / 5 / 2 | 9 / 6 / 1 | n/a |
| seatd fail: starts of driftwm / waybar / hubd while waiting | 14 / 8 / 2 | 14 / 7 / 2 | 15 / 12 / 2 | 1 / 1 / 1 | 2 / 23 / 1 | 3 / 2 / 1 | n/a | n/a | 3 / 3 / 2 | 11 / 7 / 2 | n/a |

| unrecovered by itself, count | s6plain | s6rc-par | runit | b-go | c-go | c-elixir | dinit-par | dinit-ms | openrc-par | openrc-doc | openrc-doc2 |
|---|---|---|---|---|---|---|---|---|---|---|---|
| after-supervisor-killed (alert=0) | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 6 | 6 | 0 |
| dep-fail (alert=0) | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 1 | 1 | 0 | 0 |
| dep-slow (alert=0) | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 |
| hang-spin (alert=0) | 3 | 2 | 3 | 0 | 0 | 0 | 1 | 1 | 0 | 0 | 0 |
| hang-stop (alert=0) | 3 | 3 | 6 | 0 | 0 | 0 | 3 | 3 | 3 | 0 | 0 |
| kill (alert=0) | 0 | 0 | 0 | 0 | 0 | 0 | 5 | 0 | 0 | 0 | 0 |
| kill (alert=1) | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 |
| storm (alert=0) | 0 | 0 | 0 | 0 | 0 | 0 | 2 | 2 | 2 | 0 | 0 |
| storm (alert=1) | 0 | 0 | 0 | 0 | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| supervisor-killed (alert=1) | 0 | 0 | 0 | 0 | 2 | 2 | 0 | 0 | 0 | 0 | 0 |
