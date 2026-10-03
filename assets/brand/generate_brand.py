# Generates logo.svg and banner.svg (render to PNG with any browser). Font: Anton (SIL OFL 1.1, fonts/).
import math, base64, sys
GREEN="#8CF03C"; BG="#202328"; WHITE="#F3F4F2"
font_b64=base64.b64encode(open(__import__('os').path.join(__import__('os').path.dirname(__file__),'fonts','Anton-Regular.ttf'),'rb').read()).decode()

def gear(cx,cy,r_out,r_in,teeth,tw=0.42):
    pts=[]
    for i in range(teeth):
        a0=2*math.pi*i/teeth
        step=2*math.pi/teeth
        for a,r in [(a0-step*tw/2*1.15,r_in),(a0-step*tw/2*.8,r_out),(a0+step*tw/2*.8,r_out),(a0+step*tw/2*1.15,r_in)]:
            pts.append((cx+r*math.cos(a),cy+r*math.sin(a)))
    return "M"+" L".join(f"{x:.1f} {y:.1f}" for x,y in pts)+" Z"

def icon(stroke=13):
    # drawn in a 512 box, content ~[64..452]
    s=stroke
    g=[]
    gx,gy=338,186
    g.append(f'<path d="{gear(gx,gy,112,90,11)}" fill="{BG}" stroke="{GREEN}" stroke-width="{s}" stroke-linejoin="round"/>')
    g.append(f'<circle cx="{gx}" cy="{gy}" r="40" fill="none" stroke="{GREEN}" stroke-width="{s}"/>')
    # floppy disk
    g.append(f'<path d="M78 74 H244 L300 130 V336 a16 16 0 0 1 -16 16 H94 a16 16 0 0 1 -16 -16 V90 a16 16 0 0 1 16 -16 Z" fill="{BG}" stroke="{GREEN}" stroke-width="{s}" stroke-linejoin="round"/>')
    g.append(f'<path d="M122 74 V138 H238 V74" fill="none" stroke="{GREEN}" stroke-width="{s}" stroke-linejoin="round"/>')
    g.append(f'<path d="M206 94 V118" stroke="{GREEN}" stroke-width="{s}" stroke-linecap="round"/>')
    g.append(f'<path d="M120 202 H236 M120 240 H236 M120 278 H200" stroke="{GREEN}" stroke-width="{s}" stroke-linecap="round"/>')
    # steering wheel
    cx,cy,R=318,326,128
    g.append(f'<circle cx="{cx}" cy="{cy}" r="{R+s+4}" fill="{BG}"/>')
    g.append(f'<circle cx="{cx}" cy="{cy}" r="{R}" fill="none" stroke="{GREEN}" stroke-width="{s}"/>')
    g.append(f'<circle cx="{cx}" cy="{cy}" r="{R-32}" fill="none" stroke="{GREEN}" stroke-width="{s}"/>')
    # horizontal spoke band with hub, and lower spoke
    g.append(f'<path d="M{cx-96} {cy-14} Q{cx} {cy-40} {cx+96} {cy-14}" fill="none" stroke="{GREEN}" stroke-width="{s}" stroke-linecap="round"/>')
    g.append(f'<path d="M{cx-96} {cy+14} Q{cx-50} {cy+2} {cx-34} {cy+14}" fill="none" stroke="{GREEN}" stroke-width="{s}" stroke-linecap="round"/>')
    g.append(f'<path d="M{cx+96} {cy+14} Q{cx+50} {cy+2} {cx+34} {cy+14}" fill="none" stroke="{GREEN}" stroke-width="{s}" stroke-linecap="round"/>')
    g.append(f'<circle cx="{cx}" cy="{cy+8}" r="30" fill="none" stroke="{GREEN}" stroke-width="{s}"/>')
    g.append(f'<path d="M{cx-18} {cy+34} L{cx-22} {cy+94} M{cx+18} {cy+34} L{cx+22} {cy+94}" stroke="{GREEN}" stroke-width="{s}" stroke-linecap="round"/>')
    g.append(f'<text x="{cx-60}" y="{cy+74}" font-family="DejaVu Sans Mono, Consolas, monospace" font-weight="700" font-size="46" fill="{GREEN}" text-anchor="middle">{{</text>')
    g.append(f'<text x="{cx+60}" y="{cy+74}" font-family="DejaVu Sans Mono, Consolas, monospace" font-weight="700" font-size="46" fill="{GREEN}" text-anchor="middle">}}</text>')
    return "\n".join(g)

def defs():
    return f'''<defs>
  <style>@font-face{{font-family:"AntonEmb";src:url(data:font/ttf;base64,{font_b64}) format("truetype");}}</style>
  <filter id="glow" x="-30%" y="-30%" width="160%" height="160%">
    <feGaussianBlur in="SourceGraphic" stdDeviation="5" result="b1"/>
    <feGaussianBlur in="SourceGraphic" stdDeviation="14" result="b2"/>
    <feComponentTransfer in="b2" result="b2o"><feFuncA type="linear" slope=".75"/></feComponentTransfer>
    <feMerge><feMergeNode in="b2o"/><feMergeNode in="b1"/><feMergeNode in="SourceGraphic"/></feMerge>
  </filter>
  <filter id="glowText" x="-20%" y="-30%" width="140%" height="160%">
    <feGaussianBlur in="SourceGraphic" stdDeviation="3" result="b1"/>
    <feGaussianBlur in="SourceGraphic" stdDeviation="9" result="b2"/>
    <feComponentTransfer in="b2" result="b2o"><feFuncA type="linear" slope=".55"/></feComponentTransfer>
    <feMerge><feMergeNode in="b2o"/><feMergeNode in="b1"/><feMergeNode in="SourceGraphic"/></feMerge>
  </filter>
  <filter id="grunge" x="0" y="0" width="100%" height="100%">
    <feTurbulence type="fractalNoise" baseFrequency="1.7" numOctaves="2" seed="7" result="n"/>
    <feColorMatrix in="n" type="matrix" values="0 0 0 0 0  0 0 0 0 0  0 0 0 0 0  0 0 0 -16 12.3" result="m"/>
    <feComposite in="SourceGraphic" in2="m" operator="in"/>
  </filter>
</defs>'''

def logo():
    # square app icon: dark tile + neon frame + icon
    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="512" height="512">
{defs()}
<rect x="0" y="0" width="512" height="512" rx="104" fill="{BG}"/>
<g filter="url(#glow)">
<rect x="28" y="28" width="456" height="456" rx="82" fill="none" stroke="{GREEN}" stroke-width="14"/>
<g transform="translate(40 34) scale(.84)">{icon()}</g>
</g>
</svg>'''

def mark():
    # icon alone (transparent), used inside the wordmark
    return f'<g filter="url(#glow)">{icon()}</g>'

def banner(w=1280,h=400, bg=True):
    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {w} {h}" width="{w}" height="{h}">
{defs()}
{'<rect width="%d" height="%d" fill="%s"/>'%(w,h,BG) if bg else ''}
<g filter="url(#glow)">
  <rect x="86" y="88" width="224" height="224" rx="36" fill="none" stroke="{GREEN}" stroke-width="9"/>
  <g transform="translate(98 96) scale(.40)">{icon(16)}</g>
</g>
<g font-family="AntonEmb, Anton, Impact, sans-serif" font-size="168">
  <text x="372" y="282" fill="{WHITE}" filter="url(#grunge)" letter-spacing="1">ETS</text>
  <g transform="translate(598 96) scale(.42)">{mark()}</g>
  <text x="808" y="282" fill="{WHITE}" filter="url(#grunge)">2</text>
  <text x="902" y="282" fill="{GREEN}" filter="url(#glowText)" letter-spacing="1">OMSI</text>
</g>
</svg>'''

open('logo.svg','w').write(logo())
open('banner.svg','w').write(banner())
