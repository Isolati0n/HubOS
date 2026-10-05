SCENE_DESC="does the server go quiet when the viewer stops asking? A moving window runs; the viewer process is frozen (SIGSTOP) and the node's bytes and CPU are measured while it is frozen"
SCENE_APP_TOLERANT=1
SCENE_DUR=8
scene_setup() { SCENE_APP=(benchapp drag); PROBE_X=; }
scene_run() {
  sleep 5
  measure_window running "$SCENE_DUR"
  signal_tree STOP "$CLIENT_PID"            # the viewer stops reading and stops sending update requests
  sleep 6                              # let the socket buffers fill up
  measure_window frozen "$SCENE_DUR"
  signal_tree CONT "$CLIENT_PID"
  sleep 3
  measure_window resumed 5
}
