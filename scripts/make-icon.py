"""生成应用图标（灯泡）。
用法：python scripts/make-icon.py
依赖：Pillow

输出：
  internal/tray/assets/bulb.ico   托盘图标（16~64，经典 DIB/BMP 格式，CreateIconFromResourceEx 100% 兼容）
  assets/app.ico                  可执行文件图标（含 256，交给 rsrc 编译进 .syso）
  docs/icon-preview.png           256 预览图
"""
import os
import struct

from PIL import Image, ImageDraw

ROOT = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
TRAY_ICO = os.path.join(ROOT, "internal", "tray", "assets", "bulb.ico")
APP_ICO = os.path.join(ROOT, "assets", "app.ico")
PREVIEW = os.path.join(ROOT, "docs", "icon-preview.png")

S = 512
AMBER = (255, 197, 61, 255)
AMBER_DARK = (214, 152, 18, 255)
SOCKET = (168, 176, 189, 255)
SOCKET_DARK = (128, 136, 150, 255)
BASE = (138, 146, 160, 255)


def build_master():
    """绘制 512x512 母图。"""
    img = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)
    d.ellipse([88, 42, 424, 378], fill=AMBER, outline=AMBER_DARK, width=14)

    hl = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    ImageDraw.Draw(hl).ellipse([160, 108, 268, 216], fill=(255, 255, 255, 110))
    img = Image.alpha_composite(img, hl)

    d = ImageDraw.Draw(img)
    d.rounded_rectangle([196, 372, 316, 424], radius=16, fill=SOCKET, outline=SOCKET_DARK, width=8)
    d.rounded_rectangle([212, 430, 300, 470], radius=10, fill=BASE)
    return img


def dib_bytes(img):
    """把 RGBA 图像编码成 ICO 内部使用的 BMP(DIB) 结构：BITMAPINFOHEADER + XOR 位图 + AND 掩码。"""
    w, h = img.size
    px = img.load()
    xor = bytearray()
    for y in range(h - 1, -1, -1):          # 自下而上
        for x in range(w):
            r, g, b, a = px[x, y]
            xor += bytes((b, g, r, a))      # BGRA
    row_bytes = ((w + 31) // 32) * 4        # AND 掩码每行 4 字节对齐
    and_mask = bytes(row_bytes * h)         # 32 位图靠 alpha，掩码全 0
    size_image = len(xor) + len(and_mask)
    header = struct.pack("<IiiHHIIiiII", 40, w, h * 2, 1, 32, 0, size_image, 0, 0, 0, 0)
    return header + bytes(xor) + and_mask


def write_ico(path, images):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    datas = [dib_bytes(im) for im in images]
    n = len(images)
    out = struct.pack("<HHH", 0, 1, n)      # reserved / type=icon / count
    offset = 6 + 16 * n
    entries = b""
    for im, data in zip(images, datas):
        w, h = im.size
        entries += struct.pack("<BBBBHHII", w if w < 256 else 0, h if h < 256 else 0,
                               0, 0, 1, 32, len(data), offset)
        offset += len(data)
    with open(path, "wb") as f:
        f.write(out + entries + b"".join(datas))


def report(path):
    with open(path, "rb") as f:
        data = f.read()
    count = int.from_bytes(data[4:6], "little")
    print(f"{path}  ({len(data):,} 字节, {count} 个尺寸)")
    for i in range(count):
        off = 6 + i * 16
        w = data[off] or 256
        n = int.from_bytes(data[off + 8:off + 12], "little")
        img_off = int.from_bytes(data[off + 12:off + 16], "little")
        kind = "PNG" if data[img_off:img_off + 4] == b"\x89PNG" else "BMP"
        print(f"   {w:>3}x{w:<3} {n:>8,} 字节  {kind}")


def main():
    master = build_master()

    def resized(sizes):
        return [master.resize((s, s), Image.LANCZOS) for s in sizes]

    write_ico(TRAY_ICO, resized([16, 20, 24, 32, 48, 64]))
    write_ico(APP_ICO, resized([16, 20, 24, 32, 48, 64, 128, 256]))

    os.makedirs(os.path.dirname(PREVIEW), exist_ok=True)
    master.resize((256, 256), Image.LANCZOS).save(PREVIEW)

    report(TRAY_ICO)
    report(APP_ICO)
    print(f"预览: {PREVIEW}")


if __name__ == "__main__":
    main()
