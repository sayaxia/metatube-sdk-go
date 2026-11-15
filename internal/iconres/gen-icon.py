#!/usr/bin/env python3
"""Generate MetaTube's Windows icon resources (run from anywhere).

Outputs, all relative to the repository root:

  internal/iconres/metatube.png   256px master image (embedded by the panel)
  internal/iconres/metatube.ico   multi-size ICO (BMP/DIB entries, 32bpp BGRA)
  cmd/panel/icon_windows_amd64.syso
  cmd/server/icon_windows_amd64.syso

The .syso files are standard COFF objects holding a .rsrc section. The Go
linker picks them up automatically on windows/amd64 (and ignores them on other
targets), so no post-processing of the built .exe is needed. The panel also
embeds the same ICO via go:embed for its window/taskbar/tray icons.

Usage:
  python internal/iconres/gen-icon.py                   # rebuild from the master PNG
  python internal/iconres/gen-icon.py --download        # refresh master PNG from upstream first
  python internal/iconres/gen-icon.py --source logo.png # use a different master image
  python internal/iconres/gen-icon.py --windres PATH    # explicit MinGW-w64 windres
  python internal/iconres/gen-icon.py --ico-only        # skip .syso generation

Requires Pillow; .syso generation additionally needs GNU windres (pe-x86-64).
"""

from __future__ import annotations

import argparse
import shutil
import ssl
import struct
import subprocess
import sys
import tempfile
import urllib.request
from pathlib import Path

from PIL import Image, ImageChops, ImageDraw, ImageFilter

LOGO_URL = "https://metatube-community.github.io/images/logo-vertical-dark.png"
MASTER_SIZE = 256
# Windows asks for these sizes depending on view / DPI. 256 is encoded as 0 in
# the ICO directory, as required by the format.
ICON_SIZES = (16, 20, 24, 32, 40, 48, 64, 128, 256)

# Windows does not round ICO contents for us, so the corners are cut into the
# pixels themselves. A generous radius keeps them obvious even in a 16px tray
# icon (4px there), instead of looking like a sharp square.
ICON_CORNER_RADIUS = 0.25

HERE = Path(__file__).resolve().parent
REPO = HERE.parents[1]
MASTER = HERE / "metatube.png"
ICO = HERE / "metatube.ico"
SYSO_TARGETS = (
    REPO / "cmd" / "panel" / "icon_windows_amd64.syso",
    REPO / "cmd" / "server" / "icon_windows_amd64.syso",
)


def rel(p: Path) -> Path:
    try:
        return p.relative_to(REPO)
    except ValueError:
        return p


def download_logo(url: str, dst: Path) -> None:
    print(f"[download] {url}")
    # Windows schannel occasionally fails offline CRL checks; do not let that
    # break a reproducible build. Try a direct connection first, then fall back
    # to whatever proxy the environment provides.
    ctx = ssl.create_default_context()
    ctx.check_hostname = False
    ctx.verify_mode = ssl.CERT_NONE
    attempts = [
        urllib.request.build_opener(
            urllib.request.ProxyHandler({}), urllib.request.HTTPSHandler(context=ctx)
        ),
        urllib.request.build_opener(urllib.request.HTTPSHandler(context=ctx)),
    ]
    last = None
    for opener in attempts:
        try:
            with opener.open(url, timeout=30) as r, dst.open("wb") as f:
                shutil.copyfileobj(r, f)
            return
        except Exception as e:  # noqa: BLE001 - network fallthrough by design
            last = e
    raise RuntimeError(f"无法下载 logo：{last}；可用 --source 指定本地图片")


def make_master(src: Path) -> Image.Image:
    im = Image.open(src).convert("RGBA")
    if im.size != (MASTER_SIZE, MASTER_SIZE):
        im = im.resize((MASTER_SIZE, MASTER_SIZE), Image.Resampling.LANCZOS)
    im.save(MASTER, optimize=True)
    print(f"[png] {rel(MASTER)}  {im.size[0]}x{im.size[1]}")
    return im


def rounded_alpha(size: int) -> Image.Image:
    """4x supersampled rounded-square alpha mask for one icon size."""
    ss = 4
    side = size * ss
    mask = Image.new("L", (side, side), 0)
    draw = ImageDraw.Draw(mask)
    radius = max(1, round(side * ICON_CORNER_RADIUS))
    draw.rounded_rectangle((0, 0, side - 1, side - 1), radius=radius, fill=255)
    return mask.resize((size, size), Image.Resampling.LANCZOS)


def dib_entry(im: Image.Image, size: int) -> bytes:
    """Encode one ICO image as BITMAPINFOHEADER + BGRA XOR + AND mask."""
    scaled = im.resize((size, size), Image.Resampling.LANCZOS)
    # A touch of sharpening keeps the lettering legible at 16/20 px.
    if size <= 48:
        scaled = scaled.filter(ImageFilter.UnsharpMask(radius=0.6, percent=60, threshold=2))
    scaled.putalpha(ImageChops.multiply(scaled.getchannel("A"), rounded_alpha(size)))
    rgba = scaled.tobytes()  # row-major RGBA

    xor = bytearray()
    for y in range(size - 1, -1, -1):  # DIB rows are bottom-up
        row = y * size * 4
        for x in range(size):
            i = row + x * 4
            xor += bytes((rgba[i + 2], rgba[i + 1], rgba[i], rgba[i + 3]))

    mask_row = ((size + 31) // 32) * 4
    mask = bytearray(mask_row * size)  # alpha channel carries transparency
    for y in range(size - 1, -1, -1):
        for x in range(size):
            if rgba[(y * size + x) * 4 + 3] == 0:
                bit_y = size - 1 - y
                mask[bit_y * mask_row + x // 8] |= 0x80 >> (x % 8)

    hdr = struct.pack(
        "<IiiHHIIiiII",
        40,         # biSize
        size,       # biWidth
        size * 2,   # biHeight: XOR + AND
        1,          # biPlanes
        32,         # biBitCount
        0,          # biCompression = BI_RGB
        len(xor),   # biSizeImage
        0, 0, 0, 0,
    )
    return hdr + bytes(xor) + bytes(mask)


def make_ico(im: Image.Image) -> None:
    blobs = [(s, dib_entry(im, s)) for s in ICON_SIZES]
    out = bytearray(struct.pack("<HHH", 0, 1, len(blobs)))
    offset = 6 + 16 * len(blobs)
    for size, _ in blobs:
        out += struct.pack("<BBBBHHII", size % 256, size % 256, 0, 0, 1, 32, 0, 0)
    for i, (_, data) in enumerate(blobs):
        struct.pack_into("<II", out, 6 + 16 * i + 8, len(data), offset)
        offset += len(data)
    for _, data in blobs:
        out += data
    ICO.write_bytes(bytes(out))
    print(f"[ico] {rel(ICO)}  {len(out)} bytes, sizes={','.join(map(str, ICON_SIZES))}")


def find_windres(explicit: str | None) -> str:
    if explicit:
        return explicit
    found = shutil.which("windres")
    if found:
        return found
    raise SystemExit(
        "windres not found. Install MinGW-w64 binutils (or pass --windres PATH), "
        "then re-run; metatube.ico has already been generated."
    )


def make_syso(windres: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        tmp = Path(td)
        shutil.copyfile(ICO, tmp / "metatube.ico")
        (tmp / "icon.rc").write_text('1 ICON "metatube.ico"\n', encoding="ascii")
        syso = tmp / "icon_windows_amd64.syso"
        cmd = [
            windres, "-J", "rc", "-O", "coff", "-F", "pe-x86-64",
            "-i", "icon.rc", "-o", syso.name,
        ]
        print("[windres]", " ".join(cmd))
        subprocess.run(cmd, cwd=tmp, check=True)
        data = syso.read_bytes()
        for target in SYSO_TARGETS:
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(data)
            print(f"[syso] {rel(target)}  {len(data)} bytes")


def verify_ico() -> None:
    """Sanity-check the generated ICO directory and every DIB header."""
    data = ICO.read_bytes()
    assert struct.unpack_from("<HHH", data) == (0, 1, len(ICON_SIZES))
    for i, size in enumerate(ICON_SIZES):
        ent = 6 + 16 * i
        w, h, _, _, planes, bpp, length, offset = struct.unpack_from("<BBBBHHII", data, ent)
        hdr_size, width, height = struct.unpack_from("<Iii", data, offset)
        assert (w, h) == (size % 256, size % 256), (size, w, h)
        assert planes == 1 and bpp == 32 and hdr_size == 40
        assert width == size and height == size * 2, (size, width, height)
        assert offset + length <= len(data)
    print("[verify] ICO directory and DIB headers OK")


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--download", action="store_true", help="refresh the master PNG from upstream")
    ap.add_argument("--source", type=Path, help="build the master PNG from this image")
    ap.add_argument("--windres", help="path to windres.exe (pe-x86-64 capable)")
    ap.add_argument("--ico-only", action="store_true", help="skip .syso generation")
    args = ap.parse_args()

    if args.source is not None:
        im = make_master(args.source)
    elif args.download or not MASTER.exists():
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td) / "logo.png"
            download_logo(LOGO_URL, tmp)
            im = make_master(tmp)
    else:
        im = Image.open(MASTER).convert("RGBA")
        print(f"[png] reuse {rel(MASTER)}")

    make_ico(im)
    verify_ico()
    if not args.ico_only:
        make_syso(find_windres(args.windres))
    print("[done]")
    return 0


if __name__ == "__main__":
    sys.exit(main())
