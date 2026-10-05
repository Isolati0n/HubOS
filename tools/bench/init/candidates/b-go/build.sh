# architecture B approximated: s6-svscan is the tiny PID 1 (it only reaps and restarts the supervisor and the guard); the Go
# prototype in "direct" mode starts and watches the six services itself. NOT an init: nothing here is PID 1 except s6-svscan.
copy_s6
cp "$W/brain-go" "$R/bin/brain"
