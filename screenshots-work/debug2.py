#!/usr/bin/env python3
"""Debug PTY capture with key responses"""
import os, pty, time, signal, termios, struct, fcntl, select, subprocess, sys
import pyte

COLS, ROWS = 120, 45

def run_capture(cmd, workdir, wait=3.0, init_keys=None):
    master_fd, slave_fd = pty.openpty()

    winsize = struct.pack('HHHH', ROWS, COLS, COLS * 8, ROWS * 16)
    fcntl.ioctl(master_fd, termios.TIOCSWINSZ, winsize)
    fcntl.ioctl(slave_fd, termios.TIOCSWINSZ, winsize)

    env = os.environ.copy()
    env.update({'TERM': 'xterm-256color', 'COLORTERM': 'truecolor',
                'LINES': str(ROWS), 'COLUMNS': str(COLS)})

    screen = pyte.Screen(COLS, ROWS)
    stream = pyte.ByteStream(screen)

    def preexec():
        os.setsid()
        try: fcntl.ioctl(slave_fd, termios.TIOCSCTTY, 0)
        except: pass

    proc = subprocess.Popen(cmd, stdin=slave_fd, stdout=slave_fd, stderr=slave_fd,
                            cwd=workdir, env=env, preexec_fn=preexec, close_fds=True)
    os.close(slave_fd)

    raw_all = b''
    buf = b''

    def handle(chunk):
        nonlocal buf, raw_all
        raw_all += chunk
        buf += chunk
        stream.feed(chunk)
        # Handle cursor position query
        while b'\x1b[6n' in buf:
            try: os.write(master_fd, f'\x1b[{screen.cursor.y+1};{screen.cursor.x+1}R'.encode())
            except: pass
            buf = buf.replace(b'\x1b[6n', b'', 1)

    def read_for(t):
        dl = time.time() + t
        while time.time() < dl:
            r, _, _ = select.select([master_fd], [], [], min(dl - time.time(), 0.05))
            if r:
                try:
                    c = os.read(master_fd, 16384)
                    if c: handle(c)
                except OSError: return

    # Initial boot
    read_for(1.0)

    # Send initial keys if any
    if init_keys:
        for k in init_keys:
            try: os.write(master_fd, k)
            except: pass
            read_for(0.5)

    # Main wait
    read_for(wait)

    try: os.killpg(os.getpgid(proc.pid), signal.SIGTERM)
    except: pass
    try: proc.wait(timeout=2)
    except: pass
    os.close(master_fd)
    return screen, raw_all

workdir = sys.argv[1]
cmd = sys.argv[2:]
screen, raw = run_capture(cmd, workdir, wait=3.0, init_keys=[b'y\r'])

# Save raw to file
with open('/mnt/c/c/maggus/screenshots-work/raw_output.bin', 'wb') as f:
    f.write(raw)
print(f"Raw: {len(raw)} bytes, saved to raw_output.bin")

# Check screen
non_empty = sum(1 for r in range(ROWS) for c in range(COLS)
                if screen.buffer.get(r, {}).get(c, pyte.screens.Char(' ')).data not in ('\x00', ' '))
print(f"Screen non-empty: {non_empty} cells")

# Print first 10 rows
for row in range(min(10, ROWS)):
    line = ''
    for col in range(COLS):
        try: ch = screen.buffer[row][col]; line += ch.data if ch.data and ch.data != '\x00' else ' '
        except: line += ' '
    stripped = line.rstrip()
    if stripped:
        print(f"  row{row:2d}: {repr(stripped[:80])}")

# Check for alt-screen in raw
if b'\x1b[?1049h' in raw:
    pos = raw.find(b'\x1b[?1049h')
    print(f"\nAlt-screen ENTER at pos {pos} of {len(raw)}")
    print("Bytes before:", repr(raw[max(0,pos-20):pos]))
    print("Bytes after:", repr(raw[pos:pos+100]))
else:
    print("\nNo alt-screen escape found in raw output")
