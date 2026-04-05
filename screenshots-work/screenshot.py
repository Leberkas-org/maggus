#!/usr/bin/env python3
"""
Takes a screenshot of a TUI command by running it in a PTY,
capturing ANSI output, and rendering it to a PNG using pyte + Pillow.
"""

import sys
import os
import pty
import time
import signal
import termios
import struct
import fcntl
import select
import subprocess
import argparse
from PIL import Image, ImageDraw, ImageFont
import pyte

# Terminal dimensions
COLS = 120
ROWS = 45

# Font settings
FONT_SIZE = 14
PADDING = 20

# Catppuccin Mocha color palette
ANSI_COLORS = {
    0: '#45475a',  1: '#f38ba8',  2: '#a6e3a1',  3: '#f9e2af',
    4: '#89b4fa',  5: '#cba6f7',  6: '#89dceb',  7: '#cdd6f4',
    8: '#585b70',  9: '#f38ba8', 10: '#a6e3a1', 11: '#f9e2af',
    12: '#89b4fa', 13: '#cba6f7', 14: '#89dceb', 15: '#cdd6f4',
}
NAMED_COLORS = {
    'black': 0, 'red': 1, 'green': 2, 'yellow': 3,
    'blue': 4, 'magenta': 5, 'cyan': 6, 'white': 7,
    'brightblack': 8, 'brightred': 9, 'brightgreen': 10, 'brightyellow': 11,
    'brightblue': 12, 'brightmagenta': 13, 'brightcyan': 14, 'brightwhite': 15,
}

BG_COLOR = '#1e1e2e'
FG_COLOR = '#cdd6f4'


def hex_to_rgb(hex_color):
    hex_color = hex_color.lstrip('#')
    return tuple(int(hex_color[i:i+2], 16) for i in (0, 2, 4))


def ansi_256_to_rgb(code):
    if code < 16:
        return hex_to_rgb(ANSI_COLORS.get(code, '#ffffff'))
    elif code < 232:
        idx = code - 16
        b = idx % 6; g = (idx // 6) % 6; r = idx // 36
        def c(v): return 0 if v == 0 else 55 + v * 40
        return (c(r), c(g), c(b))
    else:
        v = 8 + (code - 232) * 10
        return (v, v, v)


def resolve_color(color, default_hex):
    if color is None or color == 'default':
        return hex_to_rgb(default_hex)
    if isinstance(color, int):
        return ansi_256_to_rgb(color)
    if isinstance(color, str):
        if color.isdigit():
            return ansi_256_to_rgb(int(color))
        if color in NAMED_COLORS:
            return hex_to_rgb(ANSI_COLORS[NAMED_COLORS[color]])
    return hex_to_rgb(default_hex)


def run_and_capture(cmd, workdir, wait_secs=3.0, keys_to_send=None, init_keys=None):
    """Run a command in a PTY and capture its screen state."""
    screen = pyte.Screen(COLS, ROWS)
    stream = pyte.ByteStream(screen)

    master_fd, slave_fd = pty.openpty()

    winsize = struct.pack('HHHH', ROWS, COLS, COLS * 8, ROWS * 16)
    fcntl.ioctl(master_fd, termios.TIOCSWINSZ, winsize)
    fcntl.ioctl(slave_fd, termios.TIOCSWINSZ, winsize)

    env = os.environ.copy()
    env.update({
        'TERM': 'xterm-256color', 'COLORTERM': 'truecolor',
        'LINES': str(ROWS), 'COLUMNS': str(COLS),
    })

    def preexec():
        os.setsid()
        try: fcntl.ioctl(slave_fd, termios.TIOCSCTTY, 0)
        except: pass

    proc = subprocess.Popen(
        cmd, stdin=slave_fd, stdout=slave_fd, stderr=slave_fd,
        cwd=workdir, env=env, preexec_fn=preexec, close_fds=True,
    )
    os.close(slave_fd)

    buf = b''

    def handle(chunk):
        nonlocal buf
        buf += chunk
        stream.feed(chunk)
        # Respond to cursor position query
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

    # Short initial read to detect first-run prompts
    read_for(1.0)

    # Send init_keys (e.g., 'y\r' to answer repo registration)
    if init_keys:
        for k in init_keys:
            try: os.write(master_fd, k)
            except: pass
            read_for(0.5)

    # Main wait for TUI to render
    read_for(wait_secs)

    # Send interaction keys (navigation, tab switches, etc.)
    if keys_to_send:
        for key_seq in keys_to_send:
            try: os.write(master_fd, key_seq)
            except: pass
            read_for(1.0)

    # Kill the process
    try: os.killpg(os.getpgid(proc.pid), signal.SIGTERM)
    except:
        try: proc.terminate()
        except: pass
    try: proc.wait(timeout=2)
    except:
        try: proc.kill()
        except: pass
    os.close(master_fd)
    return screen


def render_screen_to_image(screen, output_path):
    """Render a pyte Screen to a PNG image."""
    font = None
    font_bold = None
    font_paths = [
        '/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf',
        '/usr/share/fonts/truetype/liberation/LiberationMono-Regular.ttf',
        '/usr/share/fonts/truetype/ubuntu/UbuntuMono-R.ttf',
        '/usr/share/fonts/truetype/freefont/FreeMono.ttf',
    ]
    for fp in font_paths:
        if os.path.exists(fp):
            try:
                font = ImageFont.truetype(fp, FONT_SIZE)
                for bf in [fp.replace('Mono.ttf', 'Mono-Bold.ttf'),
                           fp.replace('-Regular', '-Bold'), fp.replace('R.ttf', 'B.ttf')]:
                    if os.path.exists(bf):
                        font_bold = ImageFont.truetype(bf, FONT_SIZE)
                        break
                if font_bold is None: font_bold = font
                break
            except Exception: continue

    if font is None:
        font = ImageFont.load_default()
        font_bold = font

    dummy = Image.new('RGB', (200, 50))
    bbox = ImageDraw.Draw(dummy).textbbox((0, 0), 'M', font=font)
    char_w = bbox[2] - bbox[0]
    char_h = int((bbox[3] - bbox[1]) * 1.35)

    img = Image.new('RGB', (COLS * char_w + PADDING * 2, ROWS * char_h + PADDING * 2),
                    hex_to_rgb(BG_COLOR))
    draw = ImageDraw.Draw(img)

    for row_idx in range(ROWS):
        for col_idx in range(COLS):
            try: char = screen.buffer[row_idx][col_idx]
            except (KeyError, IndexError): continue

            data = char.data if char.data and char.data != '\x00' else ' '
            bg_rgb = resolve_color(char.bg, BG_COLOR)
            fg_rgb = resolve_color(char.fg, FG_COLOR)
            if char.reverse: bg_rgb, fg_rgb = fg_rgb, bg_rgb

            x = PADDING + col_idx * char_w
            y = PADDING + row_idx * char_h

            if bg_rgb != hex_to_rgb(BG_COLOR):
                draw.rectangle([x, y, x + char_w - 1, y + char_h - 1], fill=bg_rgb)

            if data.strip():
                f = font_bold if char.bold and font_bold != font else font
                draw.text((x, y), data, fill=fg_rgb, font=f)

    img.save(output_path)
    print(f"Saved: {output_path}", file=sys.stderr)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--cmd', nargs='+', required=True)
    parser.add_argument('--workdir', default='.')
    parser.add_argument('--output', required=True)
    parser.add_argument('--wait', type=float, default=3.0)
    parser.add_argument('--init', nargs='*', default=[], help='Keys to send before main wait (e.g. y ENTER)')
    parser.add_argument('--keys', nargs='*', default=[], help='Keys to send after main wait')
    args = parser.parse_args()

    def parse_keys(klist):
        seqs = []
        for k in klist:
            if k == 'ENTER': seqs.append(b'\r')
            elif k == 'DOWN': seqs.append(b'\x1b[B')
            elif k == 'UP': seqs.append(b'\x1b[A')
            elif k == 'TAB': seqs.append(b'\t')
            elif k == 'CTRL_C': seqs.append(b'\x03')
            elif k == 'PGDOWN': seqs.append(b'\x1b[6~')
            elif k == 'PGUP': seqs.append(b'\x1b[5~')
            elif k.startswith('0x'): seqs.append(bytes.fromhex(k[2:]))
            else: seqs.append(k.encode())
        return seqs

    screen = run_and_capture(
        args.cmd, args.workdir, args.wait,
        keys_to_send=parse_keys(args.keys),
        init_keys=parse_keys(args.init),
    )
    render_screen_to_image(screen, args.output)


if __name__ == '__main__':
    main()
