"""Draws docs/assets/harness-{light,dark}.svg. Run: python3 harness.py [out dir, default: this folder]"""
import os
import sys

THEMES = {
    "light": dict(
        text="#0b1220", muted="#5b6578", band_host="#f3f6ff", band_host_line="#c9d6fb",
        band_box="#f6f7f9", band_box_line="#d5dae3", node="#ffffff", node_line="#d0d7e2",
        arrow="#7a8599", box_inner="#ffffff", exam="#0b1220", exam_text="#ffffff",
        exam_muted="#aab4c8", lock="#16a34a", tag_bg="#e8eefe", tag_text="#2948d6",
    ),
    "dark": dict(
        text="#e6ebf5", muted="#9aa5ba", band_host="#0f1a33", band_host_line="#2a3d73",
        band_box="#121722", band_box_line="#2a3243", node="#161d2b", node_line="#33405a",
        arrow="#7d89a3", box_inner="#1a2231", exam="#e9eefb", exam_text="#0b1220",
        exam_muted="#4b5670", lock="#22c55e", tag_bg="#1b2a55", tag_text="#9ab7f9",
    ),
}

FONT = "-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif"
MONO = "ui-monospace,SFMono-Regular,Menlo,Consolas,monospace"
W, H = 1280, 600


def svg(t):
    o = []
    a = o.append
    a(f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}" '
      f'role="img" aria-label="The TaskWing harness: a lead agent plans on your machine, a fresh session writes a hidden exam, '
      f'you approve it, one agent per piece works in its own sandbox, the host joins the commits, the hidden exam runs in a clean sandbox, '
      f'and the host opens one pull request.">')
    a(f'''<defs>
<linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#0b2bd9"/><stop offset=".55" stop-color="#2f6bff"/><stop offset="1" stop-color="#6fc3ff"/></linearGradient>
<marker id="ah" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0,0 L10,5 L0,10 z" fill="{t['arrow']}"/></marker>
<marker id="ab" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0,0 L10,5 L0,10 z" fill="#2f6bff"/></marker>
<style>
text{{font-family:{FONT};fill:{t['text']}}}
.t{{font-size:16px;font-weight:600}}
.s{{font-size:12.5px;fill:{t['muted']}}}
.k{{font-family:{MONO};font-size:11.5px;letter-spacing:.08em;font-weight:600;fill:{t['muted']}}}
.l{{font-family:{MONO};font-size:11.5px;fill:{t['muted']}}}
.w{{fill:#ffffff}}
</style></defs>''')

    # Bands
    a(f'<rect x="16" y="16" width="{W-32}" height="262" rx="24" fill="{t["band_host"]}" stroke="{t["band_host_line"]}"/>')
    a(f'<text x="44" y="50" class="k">YOUR MACHINE · holds git, GitHub and your logins · pushes</text>')
    a(f'<rect x="16" y="318" width="{W-32}" height="266" rx="24" fill="{t["band_box"]}" stroke="{t["band_box_line"]}" stroke-dasharray="6 5"/>')
    a(f'<text x="44" y="352" class="k">SANDBOXES · one per agent · no GitHub login · cannot push</text>')

    def node(x, y, w, h, title, sub=None, sub2=None, step=None):
        a(f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="16" fill="{t["node"]}" stroke="{t["node_line"]}"/>')
        ty = y + (34 if sub else h / 2 + 6)
        if step:
            a(f'<text x="{x+16}" y="{y+24}" class="l">{step}</text>')
            ty += 10
        a(f'<text x="{x+16}" y="{ty}" class="t">{title}</text>')
        if sub:
            a(f'<text x="{x+16}" y="{ty+21}" class="s">{sub}</text>')
        if sub2:
            a(f'<text x="{x+16}" y="{ty+38}" class="s">{sub2}</text>')

    def pill(x, y, w, h, title, sub):
        a(f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="{h/2}" fill="url(#g)"/>')
        a(f'<text x="{x+w/2}" y="{y+h/2-2}" text-anchor="middle" class="t w">{title}</text>')
        a(f'<text x="{x+w/2}" y="{y+h/2+16}" text-anchor="middle" class="s w" style="fill:#e6eeff">{sub}</text>')

    def lock(x, y, color):
        a(f'<g transform="translate({x},{y})"><rect x="0" y="6" width="12" height="9" rx="2" fill="{color}"/>'
          f'<path d="M2.5,6 V4 a3.5,3.5 0 0 1 7,0 V6" fill="none" stroke="{color}" stroke-width="1.8"/></g>')

    def arrow(d, label=None, lx=0, ly=0, anchor="start", blue=False):
        c = "#2f6bff" if blue else t["arrow"]
        a(f'<path d="{d}" fill="none" stroke="{c}" stroke-width="1.8" marker-end="url(#{"ab" if blue else "ah"})"/>')
        if label:
            a(f'<text x="{lx}" y="{ly}" class="l" text-anchor="{anchor}">{label}</text>')

    # Host row (y 96..200)
    y0, h0 = 96, 104
    node(40, y0, 140, h0, "Your task", "one sentence", step="01")
    node(214, y0, 190, h0, "Lead agent", "plans, splits into pieces,", "pins the interface", step="02")
    node(438, y0, 200, h0, "Exam writer", "fresh session: checks", "+ hidden tests", step="03")
    lock(612, y0 + 14, t["lock"])
    pill(672, y0 + 12, 170, 80, "You approve", "the exam")
    node(876, y0, 170, h0, "Join", "host pulls commits,", "one branch", step="05")
    node(1080, y0, 160, h0, "One draft PR", "pushed by the host,", "with exam results", step="07")

    cy = y0 + h0 / 2
    arrow(f"M180,{cy} H210")
    arrow(f"M404,{cy} H434")
    arrow(f"M638,{cy} H668")
    arrow(f"M1046,{cy} H1076")

    # Workers grid (sandbox band)
    gx, gy = 470, 378
    a(f'<text x="{gx}" y="{gy+198}" class="l">04 · one agent per piece, in parallel · never sees the tests</text>')
    # The board's glyphs: ■ grey done, ▣ blue working, □ waiting, ✗ red stuck.
    # Amber "needs you" is planned (agents can't ask questions yet), so it is not drawn.
    def glyph(x, y, st):
        if st == "working":
            a(f'<rect x="{x+1}" y="{y+1}" width="16" height="16" rx="5" fill="none" stroke="#2f6bff" stroke-width="1.8"/>')
            a(f'<rect x="{x+5}" y="{y+5}" width="8" height="8" rx="2" fill="#2f6bff"/>')
        elif st == "done":
            a(f'<rect x="{x}" y="{y}" width="18" height="18" rx="5" fill="#9aa3b2"/>')
        elif st == "waiting":
            a(f'<rect x="{x+1}" y="{y+1}" width="16" height="16" rx="5" fill="none" stroke="{t["arrow"]}" stroke-width="1.6"/>')
        else:
            a(f'<path d="M{x+3},{y+3} L{x+15},{y+15} M{x+15},{y+3} L{x+3},{y+15}" stroke="#ef4444" stroke-width="2.6" stroke-linecap="round"/>')

    states = [("Agent", "working"), ("Agent", "done"), ("Agent", "working"), ("Agent", "waiting")]
    for i, (name, st) in enumerate(states):
        bx = gx + (i % 2) * 196
        by = gy + 8 + (i // 2) * 92
        a(f'<rect x="{bx}" y="{by}" width="182" height="80" rx="14" fill="{t["box_inner"]}" stroke="{t["node_line"]}"/>')
        glyph(bx + 14, by + 16, st)
        a(f'<text x="{bx+42}" y="{by+30}" class="t" style="font-size:14.5px">{name} {i+1}</text>')
        a(f'<text x="{bx+14}" y="{by+58}" class="s">own sandbox · {st}</text>')

    # Legend
    lx, ly = 44, 398
    a(f'<text x="{lx}" y="{ly}" class="k">ONE LINE PER AGENT</text>')
    for i, word in enumerate(["working", "done", "waiting", "stuck"]):
        yy = ly + 22 + i * 28
        glyph(lx, yy, word)
        a(f'<text x="{lx+30}" y="{yy+14}" class="s" style="fill:{t["text"]}">{word}</text>')
    a(f'<text x="{lx}" y="{ly + 22 + 4 * 28 + 14}" class="s">needs you: planned</text>')

    # Exam box
    ex, ey, ew, eh = 920, 386, 316, 172
    a(f'<rect x="{ex}" y="{ey}" width="{ew}" height="{eh}" rx="18" fill="{t["exam"]}"/>')
    a(f'<text x="{ex+18}" y="{ey+28}" class="l" style="fill:{t["exam_muted"]}">06 · CLEAN SANDBOX</text>')
    a(f'<text x="{ex+18}" y="{ey+56}" class="t" style="fill:{t["exam_text"]}">Hidden exam</text>')
    a(f'<text x="{ex+18}" y="{ey+76}" class="s" style="fill:{t["exam_muted"]}">runs on the joined work only</text>')
    checks = ["valid code lowers the total", "expired code is refused", "existing tests still pass"]
    for i, c in enumerate(checks):
        lock(ex + 18, ey + 92 + i * 24, t["lock"])
        a(f'<text x="{ex+40}" y="{ey+105+i*24}" class="s" style="fill:{t["exam_text"]}">{c}</text>')

    # Cross-band arrows
    arrow(f"M757,{y0+92} V{gy+4}", "one piece each", 767, 302, blue=True)
    arrow(f"M858,470 H890 Q902,470 902,458 V262 Q902,250 914,250 H940 Q952,250 952,238 V{y0+h0+4}",
          "commits only", 870, 240, anchor="start")
    arrow(f"M1000,{y0+h0} V{ey-4}", "joined work", 1010, 290)
    arrow(f"M1180,{ey} V{y0+h0+4}", "pass / fail", 1170, 302, anchor="end")

    a('</svg>')
    return "\n".join(o)


out = sys.argv[1] if len(sys.argv) > 1 else os.path.dirname(os.path.abspath(__file__))
for name, theme in THEMES.items():
    with open(os.path.join(out, f"harness-{name}.svg"), "w", encoding="utf-8") as f:
        f.write(svg(theme))
