#!/usr/bin/env python3
"""Debug PTY capture"""
import os
import pty
import time
import signal
import termios
import struct
import fcntl
import select
import subprocess

COLS = 120
ROWS = 45

def run_and_capture_raw(cmd, workdir, wait_secs=3.0):
    master_fd, slave_fd = pty.openpty()

    winsize = struct.pack('HHHH', ROWS, COLS, COLS * 8, ROWS * 16)
    fcntl.ioctl(master_fd, termios.TIOCSWINSZ, winsize)
    fcntl.ioctl(slave_fd, termios.TIOCSWINSZ, winsize)

    env = os.environ.copy()
    env['TERM'] = 'xterm-256color'
    env['COLORTERM'] = 'truecolor'
    env['LINES'] = str(ROWS)
    env['COLUMNS'] = str(COLS)

    def preexec():
        os.setsid()
        fcntl.ioctl(0, termios.TIOCSCTTY, 0)

    proc = subprocess.Popen(
        cmd,
        stdin=slave_fd,
        stdout=slave_fd,
        stderr=slave_fd,
        cwd=workdir,
        env=env,
        preexec_fn=preexec,
        close_fds=True,
    )
    os.close(slave_fd)

    output_buf = b''
    deadline = time.time() + wait_secs
    while time.time() < deadline:
        remaining = deadline - time.time()
        rlist, _, _ = select.select([master_fd], [], [], min(remaining, 0.05))
        if rlist:
            try:
                chunk = os.read(master_fd, 16384)
                if chunk:
                    output_buf += chunk
            except OSError:
                break

    try:
        os.killpg(os.getpgid(proc.pid), signal.SIGTERM)
    except Exception:
        pass
    try:
        proc.wait(timeout=2)
    except Exception:
        pass
    os.close(master_fd)

    return output_buf

import sys
cmd = sys.argv[1:]
workdir = cmd.pop(0) if cmd[0].startswith('/') and os.path.isdir(cmd[0]) else '/tmp'
# Usage: debug.py workdir cmd args...
workdir = sys.argv[1]
cmd = sys.argv[2:]

raw = run_and_capture_raw(cmd, workdir)
print(f"Captured {len(raw)} bytes")

# Show first 500 bytes as repr
print("First 200 bytes (repr):")
print(repr(raw[:200]))

# Check for alt-screen sequences
if b'\x1b[?1049h' in raw:
    print("=> Alt-screen ENTER found")
    idx = raw.find(b'\x1b[?1049h')
    print(f"  at position {idx}")
    print("Content after alt-screen enter (first 200 bytes):")
    print(repr(raw[idx+8:idx+208]))
else:
    print("=> No alt-screen sequence found")

# Try feeding to pyte and see what we get
import pyte
screen = pyte.Screen(COLS, ROWS)
stream = pyte.ByteStream(screen)
stream.feed(raw)

# Count non-empty cells
non_empty = 0
for row in range(ROWS):
    for col in range(COLS):
        try:
            c = screen.buffer[row][col]
            if c.data and c.data not in ('\x00', ' '):
                non_empty += 1
        except (KeyError, IndexError):
            pass

print(f"\npyte screen: {non_empty} non-empty cells")
if non_empty > 0:
    print("First 5 rows of screen:")
    for row in range(min(5, ROWS)):
        line = ''
        for col in range(COLS):
            try:
                c = screen.buffer[row][col]
                line += c.data if c.data and c.data != '\x00' else ' '
            except (KeyError, IndexError):
                line += ' '
        print(f"  [{row:2d}]: {repr(line[:60])}")
