/* pixelwatch: runs on the HUB. Watches single pixels of the hub's output through wlr-screencopy and prints one line
 * on every frame the hub compositor draws (60 per second on a headless output; so times are good to about 17 ms):   <CLOCK_REALTIME ns> <value of point 1> <value 2> ...
 * The value is the mean of R, G and B (0-255). Used for: input-to-pixel delay (one point), dropped frames
 * (16 points that make up a frame-number bar code), time to first frame (first line whose value is not the start value).
 *
 *   pixelwatch [-n NAME] [-t SECONDS] x,y [x,y ...]
 * Build: metrics/build-tools.sh. Needs the compositor to offer zwlr_screencopy_manager_v1 (sway and wlroots compositors do).
 */
#define _GNU_SOURCE
#include <fcntl.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <time.h>
#include <unistd.h>
#include <wayland-client.h>
#include "wlr-screencopy-unstable-v1-client-protocol.h"

static struct wl_shm *shm; static struct wl_output *out; static struct zwlr_screencopy_manager_v1 *mgr;
static int npts, px[64], py[64], bx, by, bw, bh, running = 1;
static struct zwlr_screencopy_frame_v1 *fr; static struct wl_buffer *buf; static uint8_t *data; static uint32_t stride, fmt;
static int have_buf, done_evt, got_ready, failed; static double stop_at;

static uint64_t now_ns(void) { struct timespec t; clock_gettime(CLOCK_REALTIME, &t); return t.tv_sec * 1000000000ull + t.tv_nsec; }

static void f_buffer(void *d, struct zwlr_screencopy_frame_v1 *f, uint32_t format, uint32_t w, uint32_t h, uint32_t s) {
	if (have_buf) return; fmt = format; stride = s;
	int fd = memfd_create("pw", 0); size_t sz = (size_t)s * h; ftruncate(fd, sz);
	data = mmap(NULL, sz, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0);
	struct wl_shm_pool *p = wl_shm_create_pool(shm, fd, sz);
	buf = wl_shm_pool_create_buffer(p, 0, w, h, s, format); wl_shm_pool_destroy(p); close(fd); have_buf = 1;
}
static void f_flags(void *d, struct zwlr_screencopy_frame_v1 *f, uint32_t fl) {}
static void f_ready(void *d, struct zwlr_screencopy_frame_v1 *f, uint32_t a, uint32_t b, uint32_t c) { got_ready = 1; }
static void f_failed(void *d, struct zwlr_screencopy_frame_v1 *f) { failed = 1; }
static void f_damage(void *d, struct zwlr_screencopy_frame_v1 *f, uint32_t x, uint32_t y, uint32_t w, uint32_t h) {}
static void f_dmabuf(void *d, struct zwlr_screencopy_frame_v1 *f, uint32_t a, uint32_t b, uint32_t c) {}
static void f_done(void *d, struct zwlr_screencopy_frame_v1 *f) { done_evt = 1; }
static const struct zwlr_screencopy_frame_v1_listener fl = { f_buffer, f_flags, f_ready, f_failed, f_damage, f_dmabuf, f_done };

static void reg(void *d, struct wl_registry *r, uint32_t id, const char *i, uint32_t v) {
	if (!strcmp(i, "wl_shm")) shm = wl_registry_bind(r, id, &wl_shm_interface, 1);
	else if (!strcmp(i, "wl_output") && !out) out = wl_registry_bind(r, id, &wl_output_interface, 1);
	else if (!strcmp(i, "zwlr_screencopy_manager_v1")) mgr = wl_registry_bind(r, id, &zwlr_screencopy_manager_v1_interface, v < 3 ? v : 3);
}
static void regrm(void *d, struct wl_registry *r, uint32_t id) {}
static const struct wl_registry_listener rl = { reg, regrm };

int main(int argc, char **argv) {
	double secs = 3600; int i = 1;
	for (; i < argc; i++) {
		if (!strcmp(argv[i], "-t") && i + 1 < argc) secs = atof(argv[++i]);
		else { int x, y; if (sscanf(argv[i], "%d,%d", &x, &y) == 2 && npts < 64) { px[npts] = x; py[npts++] = y; } }
	}
	if (!npts) { fprintf(stderr, "usage: pixelwatch [-t secs] x,y ...\n"); return 2; }
	int x0 = px[0], x1 = px[0], y0 = py[0], y1 = py[0];
	for (i = 1; i < npts; i++) { if (px[i] < x0) x0 = px[i]; if (px[i] > x1) x1 = px[i]; if (py[i] < y0) y0 = py[i]; if (py[i] > y1) y1 = py[i]; }
	bx = x0; by = y0; bw = x1 - x0 + 1; bh = y1 - y0 + 1;
	struct wl_display *dp = wl_display_connect(NULL);
	if (!dp) { fprintf(stderr, "no wayland display\n"); return 1; }
	struct wl_registry *r = wl_display_get_registry(dp); wl_registry_add_listener(r, &rl, NULL); wl_display_roundtrip(dp);
	if (!mgr || !out || !shm) { fprintf(stderr, "no screencopy\n"); return 1; }
	stop_at = now_ns() / 1e9 + secs;
	while (now_ns() / 1e9 < stop_at) {
		fr = zwlr_screencopy_manager_v1_capture_output_region(mgr, 0, out, bx, by, bw, bh);
		zwlr_screencopy_frame_v1_add_listener(fr, &fl, NULL);
		done_evt = got_ready = failed = 0;
		while (!done_evt && !failed) if (wl_display_dispatch(dp) < 0) return 1;
		if (failed) { zwlr_screencopy_frame_v1_destroy(fr); continue; }
		/* plain copy, not copy_with_damage: TESTED that copy_with_damage on sway 1.9 headless can wait forever after a window appears.
		 * A plain copy makes the compositor draw a frame (60 per second on a headless output), so samples come at the 16.7 ms frame clock. */
		zwlr_screencopy_frame_v1_copy(fr, buf);
		while (!got_ready && !failed) if (wl_display_dispatch(dp) < 0) return 1;
		if (got_ready) {
			printf("%llu", (unsigned long long)now_ns());
			for (i = 0; i < npts; i++) {
				uint8_t *p = data + (size_t)(py[i] - by) * stride + (size_t)(px[i] - bx) * 4; /* XRGB/ARGB little endian: B G R X */
				printf(" %d", (p[0] + p[1] + p[2]) / 3);
			}
			printf("\n"); fflush(stdout);
		}
		zwlr_screencopy_frame_v1_destroy(fr);
	}
	return 0;
}
