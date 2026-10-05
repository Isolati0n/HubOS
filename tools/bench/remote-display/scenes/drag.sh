SCENE_DESC="window drag (simulated): a 640x480 window of fine text-like strokes moves across the screen at 60 frames a second (15 s)"
SCENE_APP_TOLERANT=1
SCENE_DUR=15
scene_setup() { SCENE_APP=(benchapp drag); PROBE_X=; }
scene_run() { sleep 5; measure_window m "$SCENE_DUR"; }
