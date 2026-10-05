| Variant (20 foot windows, 700x525 each, nested 3840x2160) | idle: CPU % of one core | pan (camera glides): CPU ms per frame event, events/s | 20 windows moving: CPU ms per frame event, events/s | RSS MB |
|---|---|---|---|---|
| min: no background shader, animation_speed 1.0, no blur | 32% | 132.7 ms, 5.7/s | 203.3 ms, 1.9/s | 278 |
| min, repeated (second run) | 27% | 140.2 ms, 5.3/s | 193.2 ms, 2.0/s | 278 |
| min with LP_NUM_THREADS=2 (llvmpipe limited to 2 threads) | 28% | 135.8 ms, 5.5/s | 184.8 ms, 2.0/s | 277 |
| default config (built-in dot-grid shader background, default animation speed) | 15% | 150.4 ms, 4.8/s | 199.7 ms, 1.9/s | 280 |
| shader: animated smoke shader background (fast_smoke.glsl) | 65% | 812.6 ms, 0.9/s | 1195.0 ms, 0.6/s | 286 |
| blur: every window blurred and 90% opaque | 91% | 1408.1 ms, 0.8/s | 1273.5 ms, 1.0/s | 489 |
| heavy: shader background + blur | 74% | 6033 ms, 0.1/s | 5880.0 ms, 0.1/s | 442 |
