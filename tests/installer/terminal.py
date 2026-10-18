#!/usr/bin/env python3
"""Linux test helper: feed protected stdin to a child controlling PTY, echo disabled."""
import errno
import os
import pty
import select
import sys
import termios
import time

data = sys.stdin.buffer.read()
pid, fd = pty.fork()
if pid == 0:
    attrs = termios.tcgetattr(0)
    attrs[3] &= ~termios.ECHO
    termios.tcsetattr(0, termios.TCSANOW, attrs)
    os.execvp(sys.argv[1], sys.argv[1:])
# Wait for child's first prompt: guarantees terminal echo has been disabled.
sent = False
deadline = time.monotonic() + 600
while True:
    if time.monotonic() > deadline:
        os.kill(pid, 9)
        raise SystemExit('PTY child timed out')
    if not select.select([fd], [], [], 1)[0]:
        continue
    try:
        chunk = os.read(fd, 65536)
    except OSError as exc:
        if exc.errno == errno.EIO:
            break
        raise
    if not chunk:
        break
    sys.stdout.buffer.write(chunk)
    sys.stdout.buffer.flush()
    if not sent:
        os.write(fd, data)
        sent = True
_, status = os.waitpid(pid, 0)
raise SystemExit(os.waitstatus_to_exitcode(status))
