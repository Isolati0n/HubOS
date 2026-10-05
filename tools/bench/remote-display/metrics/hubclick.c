/* hubclick: runs on the HUB. Creates a virtual mouse (wlr-virtual-pointer) on the hub compositor, moves it to the middle
 * of the output and then, for every line "click" read from stdin, presses and releases the left button and prints
 * "<CLOCK_REALTIME ns>" taken just before the press is sent. With HUBCLICK_WIGGLE set in the environment the pointer is moved 1 pixel and back first. This is how the input-to-pixel test makes a click on the hub.
 * Build: metrics/build-tools.sh.   Usage: hubclick WIDTH HEIGHT
 */
#define _GNU_SOURCE
#include <linux/input-event-codes.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>
#include <wayland-client.h>
#include <sys/mman.h>
#include <xkbcommon/xkbcommon.h>
#include "wlr-virtual-pointer-unstable-v1-client-protocol.h"
#include "virtual-keyboard-unstable-v1-client-protocol.h"

static struct zwlr_virtual_pointer_manager_v1 *mgr; static struct wl_seat *seat; static struct zwp_virtual_keyboard_manager_v1 *kmgr;
static void reg(void *d, struct wl_registry *r, uint32_t id, const char *i, uint32_t v) {
	if (!strcmp(i, "zwlr_virtual_pointer_manager_v1")) mgr = wl_registry_bind(r, id, &zwlr_virtual_pointer_manager_v1_interface, v < 2 ? v : 2);
	else if (!strcmp(i, "zwp_virtual_keyboard_manager_v1")) kmgr = wl_registry_bind(r, id, &zwp_virtual_keyboard_manager_v1_interface, 1);
	else if (!strcmp(i, "wl_seat") && !seat) seat = wl_registry_bind(r, id, &wl_seat_interface, 1);
}
static void regrm(void *d, struct wl_registry *r, uint32_t id) {}
static const struct wl_registry_listener rl = { reg, regrm };
static uint64_t now_ns(void) { struct timespec t; clock_gettime(CLOCK_REALTIME, &t); return t.tv_sec * 1000000000ull + t.tv_nsec; }
static uint32_t ms(void) { struct timespec t; clock_gettime(CLOCK_MONOTONIC, &t); return t.tv_sec * 1000 + t.tv_nsec / 1000000; }

int main(int argc, char **argv) {
	int w = argc > 2 ? atoi(argv[1]) : 1920, h = argc > 2 ? atoi(argv[2]) : 1080;
	struct wl_display *dp = wl_display_connect(NULL);
	if (!dp) { fprintf(stderr, "no wayland display\n"); return 1; }
	struct wl_registry *r = wl_display_get_registry(dp); wl_registry_add_listener(r, &rl, NULL); wl_display_roundtrip(dp);
	if (!mgr) { fprintf(stderr, "no virtual pointer\n"); return 1; }
	/* a virtual keyboard (US layout) so that the hub seat has a keyboard and gives the focused window keyboard focus; it sends no keys */
	if (kmgr) {
		struct xkb_context *ctx = xkb_context_new(XKB_CONTEXT_NO_FLAGS);
		struct xkb_keymap *km = ctx ? xkb_keymap_new_from_names(ctx, NULL, XKB_KEYMAP_COMPILE_NO_FLAGS) : NULL;
		char *str = km ? xkb_keymap_get_as_string(km, XKB_KEYMAP_FORMAT_TEXT_V1) : NULL;
		if (str) {
			size_t n = strlen(str) + 1; int fd = memfd_create("km", 0);
			if (fd >= 0 && ftruncate(fd, n) == 0) {
				void *m = mmap(NULL, n, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0); memcpy(m, str, n);
				struct zwp_virtual_keyboard_v1 *kb = zwp_virtual_keyboard_manager_v1_create_virtual_keyboard(kmgr, seat);
				zwp_virtual_keyboard_v1_keymap(kb, WL_KEYBOARD_KEYMAP_FORMAT_XKB_V1, fd, n);
			}
		} else fprintf(stderr, "no keymap (xkb data missing?): continuing without a virtual keyboard\n");
	}
	struct zwlr_virtual_pointer_v1 *p = zwlr_virtual_pointer_manager_v1_create_virtual_pointer(mgr, seat);
	zwlr_virtual_pointer_v1_motion_absolute(p, ms(), w / 2, h / 2, w, h); zwlr_virtual_pointer_v1_frame(p);
	wl_display_roundtrip(dp);
	char line[64];
	while (fgets(line, sizeof line, stdin)) {
		if (strncmp(line, "click", 5)) continue;
		if (getenv("HUBCLICK_WIGGLE")) { /* some viewers only send the position with a pointer motion: move one pixel, then back, before the button */
			zwlr_virtual_pointer_v1_motion_absolute(p, ms(), w / 2 + 1, h / 2, w, h); zwlr_virtual_pointer_v1_frame(p);
			wl_display_roundtrip(dp); usleep(20000);
			zwlr_virtual_pointer_v1_motion_absolute(p, ms(), w / 2, h / 2, w, h); zwlr_virtual_pointer_v1_frame(p);
			wl_display_roundtrip(dp); usleep(20000);
		}
		uint64_t t0 = now_ns();
		zwlr_virtual_pointer_v1_button(p, ms(), BTN_LEFT, WL_POINTER_BUTTON_STATE_PRESSED); zwlr_virtual_pointer_v1_frame(p);
		wl_display_flush(dp);
		printf("%llu\n", (unsigned long long)t0); fflush(stdout);
		usleep(30000);
		zwlr_virtual_pointer_v1_button(p, ms(), BTN_LEFT, WL_POINTER_BUTTON_STATE_RELEASED); zwlr_virtual_pointer_v1_frame(p);
		wl_display_roundtrip(dp);
	}
	return 0;
}
