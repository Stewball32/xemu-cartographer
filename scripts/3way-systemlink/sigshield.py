#!/usr/bin/env python3
"""sigshield.py — exec a child with the Cowork per-Bash-call teardown signals
BLOCKED (not just ignored), in a fresh session.

Rationale: a plain detached xemu dies because QEMU installs its OWN SIGTERM/SIGINT
handler (graceful shutdown), which overrides any pre-exec SIG_IGN. But a *blocked*
signal is never delivered to a handler at all — it stays pending. The block mask
is inherited across execve and QEMU does not unblock these, so the watchdog's
SIGTERM/SIGSTKFLT to xemu's PID stays pending and xemu keeps running. Same proven
trick padpool.py uses for the virtual pads, applied to xemu via an exec shim.

Usage: sigshield.py --pidfile FILE -- <cmd> [args...]
"""
import os, signal, sys

def main():
    argv = sys.argv[1:]
    pidfile = None
    if argv and argv[0] == "--pidfile":
        pidfile = argv[1]; argv = argv[2:]
    if argv and argv[0] == "--":
        argv = argv[1:]
    if not argv:
        sys.exit("sigshield: no command given")
    # Block the teardown signals; inherited across execve.
    try:
        signal.pthread_sigmask(signal.SIG_BLOCK,
                               {signal.SIGTERM, signal.SIGINT, signal.SIGHUP,
                                signal.SIGQUIT, signal.SIGSTKFLT, signal.SIGPIPE})
    except Exception as ex:
        sys.stderr.write("sigshield: sigmask failed: %r\n" % ex)
    # New session: escape the Bash call's process group/session.
    try:
        os.setsid()
    except OSError:
        pass
    if pidfile:
        try:
            with open(pidfile, "w") as f:
                f.write(str(os.getpid()))
        except Exception:
            pass
    try:
        os.execvp(argv[0], argv)
    except Exception as ex:
        sys.stderr.write("sigshield: exec %r failed: %r\n" % (argv, ex))
        sys.exit(127)

if __name__ == "__main__":
    main()
