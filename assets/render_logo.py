#!/usr/bin/env python3
"""Render the brain ASCII logo to a transparent PNG with a
cyan→purple gradient. The key trick is using Menlo's *actual*
full-block bbox (not font-size) as the line height so rows of
box-drawing chars tessellate with zero gaps."""

from PIL import Image, ImageDraw, ImageFont

ART = [
    "██████╗ ██████╗  █████╗ ██╗███╗   ██╗",
    "██╔══██╗██╔══██╗██╔══██╗██║████╗  ██║",
    "██████╔╝██████╔╝███████║██║██╔██╗ ██║",
    "██╔══██╗██╔══██╗██╔══██║██║██║╚██╗██║",
    "██████╔╝██║  ██║██║  ██║██║██║ ╚████║",
    "╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝╚═╝  ╚═══╝",
]

FONT_PATH = "/System/Library/Fonts/Menlo.ttc"
FONT_SIZE = 120
PADDING = 48

# Cyan → purple, matching internal/ui/logo.go
START = (34, 211, 238)
END = (168, 85, 247)

font = ImageFont.truetype(FONT_PATH, FONT_SIZE)

# Measure the solid-block glyph. getbbox returns (l, t, r, b) for a
# single glyph — we use its FULL height as line height so successive
# rows of █ characters stack with no gap. Same for width.
left, top, right, bottom = font.getbbox("█")
cell_w = right - left
cell_h = bottom - top

# Use getmetrics to get ascent — that's where the glyph top sits
# relative to the baseline we draw at.
ascent, descent = font.getmetrics()

max_chars = max(len(line) for line in ART)
text_w = cell_w * max_chars
text_h = cell_h * len(ART)

img_w = text_w + 2 * PADDING
img_h = text_h + 2 * PADDING

# Single-channel mask: draw every row at its own y, with y = top of
# the next cell. Because we pass anchor="lt" each row's top aligns to
# the y we specify, so cell_h spacing produces a gapless stack.
mask = Image.new("L", (img_w, img_h), 0)
draw = ImageDraw.Draw(mask)
for i, line in enumerate(ART):
    y = PADDING + i * cell_h
    draw.text((PADDING, y), line, font=font, fill=255, anchor="lt")

# Horizontal gradient across the whole image.
grad_row = Image.new("RGB", (img_w, 1))
for x in range(img_w):
    t = x / max(img_w - 1, 1)
    r = round(START[0] + (END[0] - START[0]) * t)
    g = round(START[1] + (END[1] - START[1]) * t)
    b = round(START[2] + (END[2] - START[2]) * t)
    grad_row.putpixel((x, 0), (r, g, b))
gradient = grad_row.resize((img_w, img_h))

# Composite the gradient through the mask onto a transparent canvas.
result = Image.new("RGBA", (img_w, img_h), (0, 0, 0, 0))
result.paste(gradient, (0, 0), mask)

out = "/Users/ugurcanaytar/Desktop/Projects/BrainGo/brain/assets/logo.png"
result.save(out, optimize=True)
print(f"wrote {out} ({img_w}×{img_h})")
print(f"cell: {cell_w}×{cell_h}, ascent={ascent}, descent={descent}")
