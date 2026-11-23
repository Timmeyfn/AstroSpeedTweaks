#!/usr/bin/env python3
"""Renders the dashboard landscape: layered hills receding into morning haze.
Writes hills-day.webp and hills-night.webp (same hills, moonlit) for the two themes."""
import pathlib
import numpy as np
from PIL import Image
from scipy.ndimage import gaussian_filter

W, H = 3000, 1100
out = pathlib.Path(__file__).parent
rng = np.random.default_rng(1123)
xs = np.arange(W, dtype=np.float32)
ys = np.arange(H, dtype=np.float32)[:, None]


def ridge(base, amp, seed, scales=(900, 420, 160, 60, 22), weights=(1, .55, .28, .12, .05)):
    g = np.random.default_rng(seed)
    y = np.zeros(W, np.float32)
    for s, w in zip(scales, weights):
        n = gaussian_filter(g.standard_normal(W + 2 * s).astype(np.float32), s / 2.2)[s:s + W]
        n /= np.abs(n).max() + 1e-6
        y += w * n
    y /= sum(weights)
    return base - amp * y


def lerp(a, b, t):
    return a + (b - a) * t


def render(night):
    img = np.zeros((H, W, 3), np.float32)
    t = ys / H
    if night:
        top, hor = np.array([0.035, 0.045, 0.075]), np.array([0.13, 0.15, 0.22])
    else:
        top, hor = np.array([0.80, 0.84, 0.86]), np.array([0.985, 0.90, 0.80])
    sky = lerp(top[None, :], hor[None, :], np.clip(t / 0.62, 0, 1) ** 1.3)
    img[:] = sky[:, None, :]
    # sun (day) / moon (night) glow
    sx, sy = 2080, 470
    r = np.sqrt((xs[None, :] - sx) ** 2 + (ys - sy) ** 2)
    if night:
        img += (np.exp(-r / 340) * 0.10)[..., None] * np.array([0.55, 0.62, 0.85])
        img += (np.clip((22 - r) / 1.5, 0, 1) * 0.85 + np.exp(-r / 120) * 0.10)[..., None] * np.array([0.95, 0.95, 1.0])
        # a sprinkle of stars
        st = np.zeros((H, W), np.float32)
        n = 1500
        px, py = rng.integers(0, W, n), (rng.random(n) ** 1.6 * H * 0.55).astype(int)
        st[py, px] = rng.random(n) ** 3 * 2.2
        st = gaussian_filter(st, 0.7) * 3
        img += st[..., None] * np.array([0.9, 0.93, 1.0])
    else:
        img += (np.exp(-r / 420) * 0.22)[..., None] * np.array([1.0, 0.80, 0.55])
        img += (np.exp(-(r / 60) ** 2) * 0.35)[..., None] * np.array([1.0, 0.95, 0.85])

    # hills from far to near
    layers = 8
    for i in range(layers):
        k = i / (layers - 1)                     # 0 far .. 1 near
        base = lerp(560, 1030, k ** 1.25)
        amp = lerp(70, 230, k)
        y = ridge(base, amp, 40 + i * 7)
        mask = ys >= y[None, :]
        # soft anti-aliased edge
        edge = np.clip((ys - y[None, :]) / 1.6 + 0.5, 0, 1)
        if night:
            far, near = np.array([0.16, 0.18, 0.26]), np.array([0.025, 0.03, 0.045])
        else:
            far, near = np.array([0.80, 0.80, 0.79]), np.array([0.16, 0.22, 0.20])
        col = lerp(far, near, k ** 0.9)
        # gentle shading: lit tops, darker valleys
        depth_in = np.clip((ys - y[None, :]) / (90 + 240 * k), 0, 1)
        shade = 1.0 - 0.18 * depth_in
        if not night:
            light = np.clip(1 - np.abs(xs - sx) / 2600, 0, 1)[None, :]
            shade = shade + 0.10 * (1 - depth_in) * light * (1 - k * 0.5)
        layer = col[None, None, :] * shade[..., None]
        # texture on the nearer layers
        if k > 0.45:
            tex = gaussian_filter(rng.standard_normal((H, W)).astype(np.float32), 2.2)
            layer = layer * (1 + 0.035 * k * tex[..., None])
        img = img * (1 - edge[..., None]) + layer * edge[..., None]
        # haze above this ridge (between layers)
        hz = np.exp(-np.clip(y[None, :] - ys, 0, None) / (26 + 30 * k)) * (ys < y[None, :])
        haze = np.array([0.20, 0.23, 0.32]) if night else np.array([0.99, 0.93, 0.85])
        img = img * (1 - 0.32 * (1 - k) * hz[..., None]) + haze * 0.32 * (1 - k) * hz[..., None]
    # mist band low in the valley
    mist = np.exp(-((ys - 840) / 70) ** 2) * (0.5 + 0.5 * gaussian_filter(rng.standard_normal(W), 120)[None, :] * 6)
    mist = np.clip(mist, 0, 1)
    mc = np.array([0.18, 0.21, 0.30]) if night else np.array([0.97, 0.94, 0.90])
    img = img * (1 - 0.22 * mist[..., None]) + mc * 0.22 * mist[..., None]
    img += rng.normal(0, 0.006, img.shape)  # film grain
    img = np.clip(img, 0, 1) ** (1 / 1.0)
    return Image.fromarray((img * 255 + 0.5).astype(np.uint8), "RGB")


for name, night in (("hills-day", False), ("hills-night", True)):
    im = render(night)
    im.save(out / f"{name}.webp", "WEBP", quality=82, method=6)
    im.resize((W // 4, H // 4), Image.LANCZOS).save(out / f"{name}-preview.png")
    print(name, (out / f"{name}.webp").stat().st_size)
