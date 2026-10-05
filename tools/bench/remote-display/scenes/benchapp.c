/* benchapp: the scripted test window that runs on the NODE for the scenes that need exact content.
 * Plain Wayland (wl_shm + xdg-shell), no toolkit. Build: scenes/build-tools.sh
 *
 *   benchapp idle          fixed picture, never redraws (idle desktop)
 *   benchapp drag          a 640x480 picture moves along a fixed path at 60 frames per second (simulated window drag)
 *   benchapp key           a square at the top-left turns white/black on every key press or mouse click;
 *                          prints "<CLOCK_REALTIME ns> key" when the event arrives (input-to-pixel test)
 *
 * Fills the whole output (fullscreen). Prints "<ns> frame <n>" lines on stderr in drag mode (frame pacing log).
 */
#define _GNU_SOURCE
#include <errno.h>
#include <fcntl.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <time.h>
#include <unistd.h>
#include <wayland-client.h>
#include "xdg-shell-client-protocol.h"

static struct wl_compositor *comp; static struct wl_shm *shm; static struct xdg_wm_base *wm; static struct wl_seat *seat;
static struct wl_surface *surf; static struct xdg_toplevel *top; static struct wl_keyboard *kbd; static struct wl_pointer *ptr;
static int W = 0, H = 0, configured = 0, mode; /* 0 idle 1 drag 2 key */
enum { IDLE, DRAG, KEY };
struct buf { struct wl_buffer *b; uint32_t *px; int busy, w, h; } bufs[2];
static int toggled = 0; static long frame = 0; static int running = 1;

static uint64_t now_ns(void) { struct timespec t; clock_gettime(CLOCK_REALTIME, &t); return t.tv_sec * 1000000000ull + t.tv_nsec; }

static void release(void *d, struct wl_buffer *b) { ((struct buf *)d)->busy = 0; }
static const struct wl_buffer_listener bl = { release };

static int mkbuf(struct buf *s, int w, int h) {
	size_t sz = (size_t)w * h * 4; int fd = memfd_create("benchapp", 0);
	if (fd < 0 || ftruncate(fd, sz)) return -1;
	s->px = mmap(NULL, sz, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0);
	struct wl_shm_pool *p = wl_shm_create_pool(shm, fd, sz);
	s->b = wl_shm_pool_create_buffer(p, 0, w, h, w * 4, WL_SHM_FORMAT_XRGB8888);
	wl_shm_pool_destroy(p); close(fd); s->w = w; s->h = h; s->busy = 0;
	wl_buffer_add_listener(s->b, &bl, s);
	return 0;
}

/* a fixed "text-like" picture so that a lossy encoder has something hard to chew: fine black strokes on white */
static uint32_t pic(int x, int y) {
	int cell = ((x / 3) * 7 + (y / 5) * 13 + (x / 9) * (y / 11)) & 15;
	int stroke = ((x % 8) < 2) || ((y % 16) == 3) || ((x * 3 + y * 5) % 37 == 0);
	if (cell < 6 && stroke) return 0x00101010;
	return 0x00f4f4f0;
}

static void draw(struct buf *s) {
	int w = s->w, h = s->h;
	if (mode == DRAG) {
		for (int y = 0; y < h; y++) for (int x = 0; x < w; x++) s->px[y * w + x] = 0x00203040 | ((x ^ y) & 0x0f);
		int bw = 640, bh = 480; if (bw > w) bw = w; if (bh > h) bh = h;
		/* triangle-wave path, 4 seconds per leg at 60 fps, deterministic from the frame number */
		long t = frame % 480; int span_x = w - bw, span_y = h - bh;
		int px = (int)(span_x * (t < 240 ? t : 480 - t) / 240), py = (int)(span_y * (t < 240 ? t : 480 - t) / 240);
		for (int y = 0; y < bh; y++) for (int x = 0; x < bw; x++) s->px[(py + y) * w + px + x] = pic(x, y);
	} else if (mode == KEY) {
		for (int y = 0; y < h; y++) for (int x = 0; x < w; x++) s->px[y * w + x] = pic(x, y);
		uint32_t c = toggled ? 0x00ffffff : 0x00000000;
		for (int y = 0; y < 64 && y < h; y++) for (int x = 0; x < 64 && x < w; x++) s->px[y * w + x] = c;
		/* a solid light-blue patch in the bottom-right corner: the "picture has arrived" probe of the time-to-first-frame test */
		for (int y = h - 64; y < h; y++) for (int x = w - 64; x < w; x++) if (x >= 0 && y >= 0) s->px[y * w + x] = 0x00b0c8f0; /* RGB 176,200,240: mean 205, not white, so an error dialog cannot be mistaken for the picture */
	} else {
		for (int y = 0; y < h; y++) for (int x = 0; x < w; x++) s->px[y * w + x] = pic(x, y);
	}
}

static void commit(void);
static void fcb(void *d, struct wl_callback *cb, uint32_t t) { wl_callback_destroy(cb); frame++; commit(); }
static const struct wl_callback_listener cbl = { fcb };

static void commit(void) {
	if (!configured) return;
	if (W <= 0 || H <= 0) { W = 640; H = 480; } /* first configure of a fullscreen window may say 0x0: map small, the real size follows */
	struct buf *s = NULL;
	for (int i = 0; i < 2; i++) if (!bufs[i].busy && bufs[i].w == W && bufs[i].h == H) { s = &bufs[i]; break; }
	if (!s) for (int i = 0; i < 2; i++) if (!bufs[i].busy) {
		if (bufs[i].b) { wl_buffer_destroy(bufs[i].b); munmap(bufs[i].px, (size_t)bufs[i].w * bufs[i].h * 4); }
		if (mkbuf(&bufs[i], W, H)) exit(1);
		s = &bufs[i]; break;
	}
	if (!s) { if (mode == DRAG) { struct wl_callback *cb = wl_surface_frame(surf); wl_callback_add_listener(cb, &cbl, NULL); wl_surface_commit(surf); } return; }
	draw(s); s->busy = 1;
	wl_surface_attach(surf, s->b, 0, 0); wl_surface_damage_buffer(surf, 0, 0, W, H);
	if (mode == DRAG) { struct wl_callback *cb = wl_surface_frame(surf); wl_callback_add_listener(cb, &cbl, NULL);
		fprintf(stderr, "%llu frame %ld\n", (unsigned long long)now_ns(), frame); }
	wl_surface_commit(surf);
}

static void tl_conf(void *d, struct xdg_toplevel *t, int32_t w, int32_t h, struct wl_array *st) { if (w > 0 && h > 0) { W = w; H = h; } }
static void tl_close(void *d, struct xdg_toplevel *t) { running = 0; }
static const struct xdg_toplevel_listener tll = { tl_conf, tl_close };
static void xs_conf(void *d, struct xdg_surface *x, uint32_t serial) {
	xdg_surface_ack_configure(x, serial); int first = !configured; configured = 1;
	if (first || mode != DRAG) commit();
}
static const struct xdg_surface_listener xsl = { xs_conf };
static void wm_ping(void *d, struct xdg_wm_base *w, uint32_t s) { xdg_wm_base_pong(w, s); }
static const struct xdg_wm_base_listener wml = { wm_ping };

static void k_keymap(void *d, struct wl_keyboard *k, uint32_t f, int32_t fd, uint32_t s) { close(fd); }
static void k_enter(void *d, struct wl_keyboard *k, uint32_t s, struct wl_surface *sf, struct wl_array *a) {}
static void k_leave(void *d, struct wl_keyboard *k, uint32_t s, struct wl_surface *sf) {}
static void k_key(void *d, struct wl_keyboard *k, uint32_t s, uint32_t t, uint32_t key, uint32_t st) {
	if (st != 1) return;
	fprintf(stderr, "%llu key\n", (unsigned long long)now_ns()); toggled = !toggled; commit();
}
static void k_mod(void *d, struct wl_keyboard *k, uint32_t s, uint32_t a, uint32_t b, uint32_t c, uint32_t g) {}
static void k_rep(void *d, struct wl_keyboard *k, int32_t r, int32_t dl) {}
static const struct wl_keyboard_listener kl = { k_keymap, k_enter, k_leave, k_key, k_mod, k_rep };
static void p_enter(void *d, struct wl_pointer *p, uint32_t s, struct wl_surface *sf, wl_fixed_t x, wl_fixed_t y) {}
static void p_leave(void *d, struct wl_pointer *p, uint32_t s, struct wl_surface *sf) {}
static void p_motion(void *d, struct wl_pointer *p, uint32_t t, wl_fixed_t x, wl_fixed_t y) {}
static void p_button(void *d, struct wl_pointer *p, uint32_t s, uint32_t t, uint32_t b, uint32_t st) {
	if (st != 1) return;
	fprintf(stderr, "%llu key\n", (unsigned long long)now_ns()); toggled = !toggled; commit();
}
static void p_axis(void *d, struct wl_pointer *p, uint32_t t, uint32_t a, wl_fixed_t v) {}
static const struct wl_pointer_listener pl = { p_enter, p_leave, p_motion, p_button, p_axis };
static void s_caps(void *d, struct wl_seat *s, uint32_t c) {
	if ((c & WL_SEAT_CAPABILITY_KEYBOARD) && !kbd) { kbd = wl_seat_get_keyboard(s); wl_keyboard_add_listener(kbd, &kl, NULL); }
	if ((c & WL_SEAT_CAPABILITY_POINTER) && !ptr) { ptr = wl_seat_get_pointer(s); wl_pointer_add_listener(ptr, &pl, NULL); }
}
static void s_name(void *d, struct wl_seat *s, const char *n) {}
static const struct wl_seat_listener sl = { s_caps, s_name };

static void reg(void *d, struct wl_registry *r, uint32_t id, const char *i, uint32_t v) {
	if (!strcmp(i, "wl_compositor")) comp = wl_registry_bind(r, id, &wl_compositor_interface, 4);
	else if (!strcmp(i, "wl_shm")) shm = wl_registry_bind(r, id, &wl_shm_interface, 1);
	else if (!strcmp(i, "xdg_wm_base")) { wm = wl_registry_bind(r, id, &xdg_wm_base_interface, 1); xdg_wm_base_add_listener(wm, &wml, NULL); }
	else if (!strcmp(i, "wl_seat")) { seat = wl_registry_bind(r, id, &wl_seat_interface, 1); wl_seat_add_listener(seat, &sl, NULL); }
}
static void regrm(void *d, struct wl_registry *r, uint32_t id) {}
static const struct wl_registry_listener rl = { reg, regrm };

int main(int argc, char **argv) {
	const char *m = argc > 1 ? argv[1] : "idle";
	mode = !strcmp(m, "drag") ? DRAG : !strcmp(m, "key") ? KEY : IDLE;
	struct wl_display *dp = wl_display_connect(NULL);
	if (!dp) { fprintf(stderr, "no wayland display\n"); return 1; }
	struct wl_registry *r = wl_display_get_registry(dp); wl_registry_add_listener(r, &rl, NULL);
	wl_display_roundtrip(dp);
	if (!comp || !shm || !wm) { fprintf(stderr, "missing globals\n"); return 1; }
	surf = wl_compositor_create_surface(comp);
	struct xdg_surface *xs = xdg_wm_base_get_xdg_surface(wm, surf); xdg_surface_add_listener(xs, &xsl, NULL);
	top = xdg_surface_get_toplevel(xs);
	xdg_toplevel_add_listener(top, &tll, NULL); xdg_toplevel_set_title(top, "benchapp"); xdg_toplevel_set_app_id(top, "benchapp");
	xdg_toplevel_set_fullscreen(top, NULL);
	wl_surface_commit(surf);
	while (running && wl_display_dispatch(dp) != -1) {}
	return 0;
}
