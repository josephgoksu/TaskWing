"""Draws the README visuals: swarm-{light,dark}.svg and why-{light,dark}.svg.
Run: python3 docs/assets/readme_visuals.py [out dir, default: this folder]"""
import math
import os
import sys

FONT = "-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif"
MONO = "ui-monospace,SFMono-Regular,Menlo,Consolas,monospace"

# the site's agent colours: work blue, done grey, needs-you amber, pass green
THEMES = {
    "light": dict(text="#12151c", muted="#5a6170", idle="#cdd2da", work="#2f6bff", done="#a9afba",
                  needs="#e09a12", pass_="#17924f", card="#ffffff", line="#e2e5ea", link="#2f6bff",
                  tint_blue="#eaf0ff", tint_green="#e6f4ec", tint_amber="#fcf1dc", shadow="#12151c"),
    "dark": dict(text="#e6ebf5", muted="#9aa5ba", idle="#2b3240", work="#4f7dff", done="#5d6676",
                 needs="#e8a530", pass_="#2fb36b", card="#161b22", line="#30363d", link="#6f93ff",
                 tint_blue="#16224a", tint_green="#11301f", tint_amber="#3a2b0e", shadow="#000000"),
}

ICON = {
    "pr": '<circle cx="6" cy="6" r="2.5"/><circle cx="6" cy="18" r="2.5"/><circle cx="18" cy="18" r="2.5"/><path d="M6 8.5v7M18 15.5V10a3 3 0 0 0-3-3h-4"/><path d="m13 4.5-2.5 2.5L13 9.5"/>',
    "box": '<path d="M21 8 12 3 3 8v8l9 5 9-5z"/><path d="M3 8l9 5 9-5M12 13v8"/>',
    "shield": '<path d="M12 2.5 20 6v6c0 5-3.4 8.6-8 10-4.6-1.4-8-5-8-10V6z"/><path d="m8.5 12 2.5 2.5 4.5-5"/>',
    "terminal": '<rect x="3" y="4" width="18" height="16" rx="2.5"/><path d="m7 9 3 3-3 3M12.5 15H17"/>',
    "plug": '<path d="M9 2v5M15 2v5M6 7h12v4a6 6 0 0 1-12 0zM12 17v5"/>',
}


class Rand:
    def __init__(self, seed):
        self.s = seed

    def __call__(self):
        self.s = (self.s * 16807) % 2147483647
        return self.s / 2147483647


def rr(x, y, w, h, r, extra):
    return f'<rect x="{x:.1f}" y="{y:.1f}" width="{w:.1f}" height="{h:.1f}" rx="{r}" {extra}/>'


def text_w(t, size):  # rough width of monospace text
    return len(t) * size * 0.61


def swarm(t):
    W, H, gap, sq = 1280, 420, 22, 10
    cols, rows = W // gap, H // gap
    ox, oy = (W - (cols - 1) * gap) / 2, (H - (rows - 1) * gap) / 2
    lead = (ox + (cols // 2) * gap, oy + (rows // 2) * gap)
    labels = [("api/codes.go", -7, -5), ("checkout/apply.ts", 7, -6), ("db/migrations", -9, 4),
              ("web/CouponField.tsx", 8, 4), ("tests/e2e", -2, -7), ("docs/discounts.md", 1, 7),
              ("auth/session.go", 15, -1), ("cart/total.go", -16, -1)]
    lab = {(round(lead[0] + dx * gap, 1), round(lead[1] + dy * gap, 1)): n for n, dx, dy in labels}
    needs = (round(lead[0] + 12 * gap, 1), round(lead[1] + 6 * gap, 1))
    R = Rand(11)
    o = []
    a = o.append
    counts = {"work": 0, "done": 0}
    cards = []
    idle, filled = [], {"work": [], "done": [], "needs": []}
    for j in range(rows):
        for i in range(cols):
            x, y = round(ox + i * gap, 1), round(oy + j * gap, 1)
            if (x, y) == (round(lead[0], 1), round(lead[1], 1)):
                continue
            if (x, y) in lab:
                cards.append((x, y, lab[(x, y)]))
                continue
            if (x, y) == needs:
                filled["needs"].append((x, y))
                continue
            d = math.hypot((x - lead[0]) / 1.35, y - lead[1]) / gap
            r = R()
            pw = max(0.0, 0.6 - d * 0.042)
            pd = 0.12 + max(0.0, 0.22 - d * 0.01)
            st = "work" if r < pw else "done" if r < pw + pd else "idle"
            if st == "idle":
                idle.append((x, y))
            else:
                filled[st].append((x, y))
                counts[st] += 1
    counts["work"] += len(cards)
    a(f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}" role="img" '
      f'aria-label="A swarm of coding agents: a lead agent in the middle sends work to hundreds of agents, each square is one agent in its own sandbox. '
      f'{counts["work"]} working, {counts["done"]} done, 1 needs you.">')
    a(f'''<defs>
<linearGradient id="fade" x1="0" x2="1"><stop offset="0" stop-color="#fff" stop-opacity="0"/><stop offset=".14" stop-color="#fff"/><stop offset=".86" stop-color="#fff"/><stop offset="1" stop-color="#fff" stop-opacity="0"/></linearGradient>
<linearGradient id="fadev" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#fff" stop-opacity="0"/><stop offset=".12" stop-color="#fff"/><stop offset=".88" stop-color="#fff"/><stop offset="1" stop-color="#fff" stop-opacity="0"/></linearGradient>
<mask id="m"><rect width="{W}" height="{H}" fill="url(#fade)"/></mask>
<mask id="mv"><rect width="{W}" height="{H}" fill="url(#fadev)"/></mask>
<radialGradient id="glow"><stop offset="0" stop-color="{t['work']}" stop-opacity=".28"/><stop offset="1" stop-color="{t['work']}" stop-opacity="0"/></radialGradient>
<filter id="sh" x="-20%" y="-40%" width="140%" height="180%"><feDropShadow dx="0" dy="2" stdDeviation="3" flood-color="{t['shadow']}" flood-opacity=".14"/></filter>
<style>text{{font-family:{MONO};font-size:14px;fill:{t['text']}}} .hud{{font-size:14px}}</style>
</defs>''')
    a('<g mask="url(#m)"><g mask="url(#mv)">')
    a(f'<g fill="none" stroke="{t["idle"]}" stroke-width="1.1">' + "".join(rr(x - sq / 2 + .55, y - sq / 2 + .55, sq - 1.1, sq - 1.1, 2.5, "") for x, y in idle) + "</g>")
    for st in ("done", "work", "needs"):
        col = t[st]
        a(f'<g fill="{col}">' + "".join(rr(x - sq / 2, y - sq / 2, sq, sq, 2.5, "") for x, y in filled[st]) + "</g>")
    a("</g></g>")
    # links from the lead to the labelled agents
    a(f'<g fill="none" stroke="{t["link"]}" stroke-opacity=".55" stroke-width="1.3" stroke-dasharray="3 5">')
    for x, y, _ in cards:
        mx = (lead[0] + x) / 2 + (y - lead[1]) * 0.18
        my = (lead[1] + y) / 2 - (x - lead[0]) * 0.18
        a(f'<path d="M{lead[0]:.1f} {lead[1]:.1f} Q{mx:.1f} {my:.1f} {x:.1f} {y:.1f}"/>')
    a("</g>")
    for x, y, n in cards:
        a(rr(x - 11, y - 11, 22, 22, 7, f'fill="{t["work"]}" fill-opacity=".18"'))
        a(rr(x - 8, y - 8, 16, 16, 5, f'fill="{t["work"]}"'))
        w = text_w(n, 14) + 20
        lx = x + 16 if x + 16 + w < W - 20 else x - 16 - w
        a(rr(lx, y - 14, w, 28, 7, f'fill="{t["card"]}" stroke="{t["line"]}" filter="url(#sh)"'))
        a(f'<text x="{lx + 10:.1f}" y="{y + 5:.1f}">{n}</text>')
    # the lead
    lx, ly = lead
    a(f'<circle cx="{lx}" cy="{ly}" r="64" fill="url(#glow)"/>')
    a(rr(lx - 25, ly - 25, 50, 50, 14, f'fill="none" stroke="{t["work"]}" stroke-opacity=".4" stroke-width="1.4"'))
    a(rr(lx - 16, ly - 16, 32, 32, 9, f'fill="{t["work"]}"'))
    a(rr(lx - 5, ly - 5, 10, 10, 2.5, 'fill="#fff"'))
    lw = text_w("Lead agent", 14) + 22
    a(rr(lx - lw / 2, ly + 32, lw, 28, 14, f'fill="{t["card"]}" stroke="{t["line"]}" filter="url(#sh)"'))
    a(f'<text x="{lx:.1f}" y="{ly + 51:.1f}" text-anchor="middle" style="font-weight:600">Lead agent</text>')
    # counts, like the site's hero
    chips = [("work", f'{counts["work"]} working'), ("done", f'{counts["done"]} done'), ("needs", "1 needs you")]
    cx = W - 24
    for key, label in reversed(chips):
        w = text_w(label, 14) + 38
        cx -= w
        a(rr(cx, 14, w, 32, 16, f'fill="{t["card"]}" stroke="{t["line"]}"'))
        a(rr(cx + 13, 25.5, 10, 10, 2.5, f'fill="{t[key]}"'))
        a(f'<text class="hud" x="{cx + 30:.1f}" y="35">{label}</text>')
        cx -= 8
    a("</svg>")
    return "\n".join(o)


def why(t):
    W, H = 1280, 270
    cards = [
        ("pr", "pass_", "tint_green", "One task in", ["One tested pull", "request out"]),
        ("box", "work", "tint_blue", "A sandbox each", ["No GitHub login", "in any box"]),
        ("shield", "pass_", "tint_green", "Hidden exam", ["Tests written first,", "unseen by agents"]),
        ("terminal", "work", "tint_blue", "Your agent", ["Your logins, no", "API key of ours"]),
        ("plug", "needs", "tint_amber", "Plugs in", ["Bring your sandbox", "and agent CLIs"]),
    ]
    gap = 16
    cw = (W - gap * (len(cards) - 1)) / len(cards)
    o = []
    a = o.append
    a(f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}" role="img" '
      f'aria-label="Why TaskWing: one task in, one tested pull request out; every agent in its own sandbox; a hidden exam; your own agent and logins; plugs into your sandbox and agent tools.">')
    a(f'''<defs><filter id="sh" x="-10%" y="-10%" width="120%" height="130%"><feDropShadow dx="0" dy="3" stdDeviation="5" flood-color="{t['shadow']}" flood-opacity=".08"/></filter>
<style>text{{font-family:{FONT};fill:{t['text']}}} .h{{font-size:25px;font-weight:600;letter-spacing:-.015em}} .s{{font-size:18px;fill:{t['muted']}}}</style></defs>''')
    for k, (ic, col, tint, head, sub) in enumerate(cards):
        x = k * (cw + gap)
        a(rr(x + 1, 1, cw - 2, H - 10, 16, f'fill="{t["card"]}" stroke="{t["line"]}" filter="url(#sh)"'))
        a(rr(x + 24, 26, 58, 58, 15, f'fill="{t[tint]}"'))
        a(f'<g transform="translate({x + 37:.1f} 39)" fill="none" stroke="{t[col]}" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">'
          f'<svg width="32" height="32" viewBox="0 0 24 24">{ICON[ic]}</svg></g>')
        a(f'<text class="h" x="{x + 24:.1f}" y="132">{head}</text>')
        for i, line in enumerate(sub):
            a(f'<text class="s" x="{x + 24:.1f}" y="{168 + i * 25}">{line}</text>')
        # a row of little agent squares as a footer motif
        for i in range(6):
            fill = t[col] if i < 4 else "none"
            stroke = "none" if i < 4 else t["idle"]
            a(rr(x + 24 + i * 15, H - 40, 9, 9, 2.2, f'fill="{fill}" stroke="{stroke}" fill-opacity="{1 if i < 4 else 0}"'))
    a("</svg>")
    return "\n".join(o)


def main():
    out = sys.argv[1] if len(sys.argv) > 1 else os.path.dirname(os.path.abspath(__file__))
    for name, fn in (("swarm", swarm), ("why", why)):
        for theme, t in THEMES.items():
            with open(os.path.join(out, f"{name}-{theme}.svg"), "w") as f:
                f.write(fn(t) + "\n")


if __name__ == "__main__":
    main()
