/* MEASUREMENT STUB, not an init and not for the image. It does what any PID 1 must do at the very least: wait for
 * signals and reap children. Used only to compare size, memory, start time and failure behaviour of languages.
 * modes: "exit" (leave at once), "panic" (die the way a bug would), default: reap forever. */
#include <signal.h>
#include <string.h>
#include <sys/wait.h>
#include <time.h>
int main(int argc, char **argv) {
	if (argc > 1 && !strcmp(argv[1], "exit")) return 0;
	if (argc > 1 && !strcmp(argv[1], "panic")) { *(volatile int *)0 = 1; }
	sigset_t s; sigfillset(&s); sigprocmask(SIG_BLOCK, &s, 0);
	for (;;) {
		siginfo_t i; struct timespec t = {3600, 0};
		if (sigtimedwait(&s, &i, &t) < 0) continue;
		if (i.si_signo == SIGCHLD) while (waitpid(-1, 0, WNOHANG) > 0) {}
	}
}
