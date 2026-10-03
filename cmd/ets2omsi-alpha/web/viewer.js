// ETS2OMSI 3D viewer: textured, lit, game-like preview of the converted car.
// Internal axes: X lateral, Y longitudinal, Z up (same as the converter scene).
'use strict';

function createRenderer(canvas, d) {
  const gl = canvas.getContext('webgl', {antialias: true, alpha: false, premultipliedAlpha: false});
  if (!gl) return {destroy() {}};
  const aniso = gl.getExtension('EXT_texture_filter_anisotropic') || gl.getExtension('WEBKIT_EXT_texture_filter_anisotropic');

  // ---------- shaders ----------
  const carVS = `
    attribute vec3 p; attribute vec3 n; attribute vec2 uv;
    uniform mat4 vp;
    varying vec3 wp; varying vec3 wn; varying vec2 vuv;
    void main(){ wp=p; wn=n; vuv=uv; gl_Position=vp*vec4(p,1.0); }`;
  const carFS = `
    precision mediump float;
    varying vec3 wp; varying vec3 wn; varying vec2 vuv;
    uniform sampler2D tex; uniform float hasTex; uniform vec3 baseColor;
    uniform float alphaMode; uniform float gloss; uniform float metal; uniform float emissive; uniform float hl;
    uniform vec3 eye; uniform vec3 sunDir;
    vec3 sky(vec3 r){
      float t=clamp(r.z*0.5+0.5,0.0,1.0);
      vec3 horizon=vec3(0.80,0.84,0.88), zenith=vec3(0.36,0.55,0.80), ground=vec3(0.22,0.21,0.20);
      return r.z>0.0 ? mix(horizon,zenith,pow(r.z,0.6)) : mix(horizon,ground,pow(-r.z,0.4));
    }
    void main(){
      vec4 t = hasTex>0.5 ? texture2D(tex,vuv) : vec4(baseColor,1.0);
      vec3 albedo = pow(t.rgb, vec3(2.2));
      float a = alphaMode>0.5 ? t.a : 1.0;
      vec3 N = normalize(wn);
      vec3 V = normalize(eye-wp);
      if (dot(N,V)<0.0) N=-N;                 // two-sided lighting
      vec3 L = normalize(sunDir);
      float ndl = max(dot(N,L),0.0);
      vec3 H = normalize(L+V);
      float spec = pow(max(dot(N,H),0.0), mix(16.0,180.0,gloss)) * gloss;
      float fres = pow(1.0-max(dot(N,V),0.0),5.0);
      vec3 amb = mix(vec3(0.20,0.19,0.18), vec3(0.42,0.47,0.55), N.z*0.5+0.5);
      vec3 env = sky(reflect(-V,N));
      float refl = clamp(mix(0.04,0.9,metal)*gloss + fres*gloss*0.6, 0.0, 1.0);
      vec3 col = albedo*(amb + vec3(1.0,0.97,0.92)*ndl*1.15);
      col = mix(col, env*mix(vec3(1.0),albedo,metal), refl*0.55);
      col += vec3(1.0,0.98,0.94)*spec*0.9;
      col += albedo*emissive;
      // contact darkening near the ground
      col *= mix(0.55,1.0,clamp(wp.z*2.2+0.1,0.0,1.0));
      col = col/(col+vec3(0.85))*1.55;          // soft tone map
      col = mix(col, vec3(1.0,0.35,0.0), hl*0.6);  // selected material
      gl_FragColor = vec4(pow(col,vec3(1.0/2.2)), a);
    }`;
  const groundVS = `attribute vec2 g; uniform mat4 vp; varying vec2 gp; void main(){ gp=g; gl_Position=vp*vec4(g,0.0,1.0); }`;
  const groundFS = `
    precision mediump float; varying vec2 gp; uniform vec2 hs; uniform float radius;
    void main(){
      float d=length(gp);
      vec2 q=gp/(hs+vec2(0.25,0.35));
      float sh=smoothstep(1.25,0.35,length(q));
      vec3 base=mix(vec3(0.46,0.47,0.48),vec3(0.36,0.37,0.38),smoothstep(0.0,radius,d));
      vec2 f=abs(fract(gp)-0.5);
      float grid=smoothstep(0.488,0.5,max(f.x,f.y))*0.03;
      vec3 c=base*(1.0-0.7*sh)-grid;
      float fade=smoothstep(radius,radius*0.55,d);
      gl_FragColor=vec4(c,fade);
    }`;
  const skyVS = `attribute vec2 q; varying vec2 sq; void main(){ sq=q; gl_Position=vec4(q,0.9999,1.0); }`;
  const skyFS = `precision mediump float; varying vec2 sq;
    void main(){ float t=sq.y*0.5+0.5; vec3 c=mix(vec3(0.62,0.66,0.70),vec3(0.30,0.45,0.66),pow(t,0.9)); gl_FragColor=vec4(c,1.0); }`;

  const carP = link(gl, carVS, carFS), groundP = link(gl, groundVS, groundFS), skyP = link(gl, skyVS, skyFS);

  // ---------- geometry grouped by material ----------
  const mats = (d.materials && d.materials.length) ? d.materials : [{name: 'default', class: 'body', color: [.6, .6, .65]}];
  const groups = mats.map(() => ({pos: [], nor: [], uv: []}));
  const tm = d.triangle_materials || [];
  for (let ti = 0; ti < tm.length; ti++) {
    const mi = Math.min(tm[ti] || 0, groups.length - 1), g = groups[mi];
    for (let k = 0; k < 3; k++) {
      const vi = d.indices[ti * 3 + k];
      g.pos.push(d.positions[vi * 3], d.positions[vi * 3 + 1], d.positions[vi * 3 + 2]);
      g.nor.push(d.normals[vi * 3] ?? 0, d.normals[vi * 3 + 1] ?? 0, d.normals[vi * 3 + 2] ?? 1);
      g.uv.push(d.uvs ? d.uvs[vi * 2] ?? 0 : 0, d.uvs ? d.uvs[vi * 2 + 1] ?? 0 : 0);
    }
  }
  const buffers = [];
  function buf(data) { const b = gl.createBuffer(); gl.bindBuffer(gl.ARRAY_BUFFER, b); gl.bufferData(gl.ARRAY_BUFFER, new Float32Array(data), gl.STATIC_DRAW); buffers.push(b); return b; }
  const textures = [];
  const texCache = {};
  function loadTexture(name) {
    if (!name || !d.texture_set) return null;
    if (texCache[name]) return texCache[name];
    const entry = {tex: null};
    texCache[name] = entry;
    const img = new Image();
    img.onload = () => {
      const t = gl.createTexture();
      gl.bindTexture(gl.TEXTURE_2D, t);
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, img);
      const pot = x => (x & (x - 1)) === 0;
      if (pot(img.width) && pot(img.height)) {
        gl.generateMipmap(gl.TEXTURE_2D);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT);
      } else {
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
      }
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
      if (aniso) gl.texParameterf(gl.TEXTURE_2D, aniso.TEXTURE_MAX_ANISOTROPY_EXT, Math.min(8, gl.getParameter(aniso.MAX_TEXTURE_MAX_ANISOTROPY_EXT)));
      textures.push(t);
      entry.tex = t;
    };
    img.src = '/api/texture?set=' + encodeURIComponent(d.texture_set) + '&name=' + encodeURIComponent(name);
    return entry;
  }
  const surface = {
    glass: {gloss: .95, metal: .1}, chrome: {gloss: .9, metal: .95}, rubber: {gloss: .08, metal: 0},
    light: {gloss: .8, metal: .2}, paint: {gloss: .75, metal: .15}, body: {gloss: .6, metal: .1},
  };
  const draws = groups.map((g, i) => {
    if (!g.pos.length) return null;
    const m = mats[i] || {};
    const cls = m.class || 'body';
    return {
      mi: i,
      count: g.pos.length / 3, p: buf(g.pos), n: buf(g.nor), uv: buf(g.uv),
      tex: loadTexture(m.texture), color: m.color || [.6, .6, .65],
      alpha: !!m.alpha || cls === 'glass', cls, ...(surface[cls] || surface.body),
    };
  }).filter(Boolean);
  const groundR = Math.max(8, (d.length || 4) * 2.2);
  const groundBuf = buf([-groundR, -groundR, groundR, -groundR, groundR, groundR, -groundR, -groundR, groundR, groundR, -groundR, groundR]);
  const skyBuf = buf([-1, -1, 1, -1, 1, 1, -1, -1, 1, 1, -1, 1]);

  function attr(prog, name, b, size) {
    const a = gl.getAttribLocation(prog, name);
    if (a < 0) return;
    gl.bindBuffer(gl.ARRAY_BUFFER, b);
    gl.enableVertexAttribArray(a);
    gl.vertexAttribPointer(a, size, gl.FLOAT, false, 0, 0);
  }
  function u(prog, name) { return gl.getUniformLocation(prog, name); }

  // ---------- camera ----------
  const H = d.height || 1.4, target0 = [0, 0, H * 0.45];
  const dist0 = Math.max(4.5, Math.max(d.length || 4, d.width || 1.8) * 1.25);
  let yaw = 0.75, pitch = 0.22, dist = dist0, target = target0.slice(), auto = true;
  let drag = 0, lx = 0, ly = 0, raf = 0, highlight = -1;
  canvas.oncontextmenu = e => e.preventDefault();
  canvas.onmousedown = e => { drag = e.button === 2 || e.shiftKey ? 2 : 1; lx = e.clientX; ly = e.clientY; auto = false; };
  const up = () => { drag = 0; };
  window.addEventListener('mouseup', up);
  canvas.onmousemove = e => {
    if (!drag) return;
    const dx = e.clientX - lx, dy = e.clientY - ly; lx = e.clientX; ly = e.clientY;
    if (drag === 1) { yaw -= dx * .008; pitch = Math.max(-0.05, Math.min(1.35, pitch + dy * .006)); }
    else { const s = dist * .0016, cx = Math.cos(yaw), sx = Math.sin(yaw); target[0] -= (dx * cx) * s; target[1] -= (-dx * sx) * s; target[2] = Math.max(0, target[2] + dy * s); }
  };
  canvas.onwheel = e => { e.preventDefault(); dist = Math.max(1.2, Math.min(60, dist * (1 + Math.sign(e.deltaY) * .09))); };
  canvas.ondblclick = () => { yaw = 0.75; pitch = 0.22; dist = dist0; target = target0.slice(); auto = true; };

  const sun = norm([0.45, -0.35, 0.82]);
  function draw() {
    resizeCanvas(canvas);
    gl.viewport(0, 0, canvas.width, canvas.height);
    if (auto) yaw += 0.003;
    const eye = [target[0] + dist * Math.cos(pitch) * Math.sin(yaw), target[1] - dist * Math.cos(pitch) * Math.cos(yaw), target[2] + dist * Math.sin(pitch)];
    const proj = perspective(38 * Math.PI / 180, canvas.width / Math.max(1, canvas.height), .05, 400);
    const vp = mul(proj, lookAt(eye, target, [0, 0, 1]));
    gl.clearColor(.5, .55, .6, 1);
    gl.clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT);
    gl.disable(gl.CULL_FACE);

    // sky
    gl.disable(gl.DEPTH_TEST);
    gl.useProgram(skyP); attr(skyP, 'q', skyBuf, 2); gl.drawArrays(gl.TRIANGLES, 0, 6);

    // ground with soft shadow
    gl.enable(gl.DEPTH_TEST); gl.depthMask(true);
    gl.enable(gl.BLEND); gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);
    gl.useProgram(groundP); attr(groundP, 'g', groundBuf, 2);
    gl.uniformMatrix4fv(u(groundP, 'vp'), false, new Float32Array(vp));
    gl.uniform2f(u(groundP, 'hs'), (d.width || 1.8) / 2, (d.length || 4.5) / 2);
    gl.uniform1f(u(groundP, 'radius'), groundR);
    gl.drawArrays(gl.TRIANGLES, 0, 6);

    // car: opaque first, then transparent (glass) without depth writes
    gl.useProgram(carP);
    gl.uniformMatrix4fv(u(carP, 'vp'), false, new Float32Array(vp));
    gl.uniform3fv(u(carP, 'eye'), eye); gl.uniform3fv(u(carP, 'sunDir'), sun);
    gl.uniform1i(u(carP, 'tex'), 0);
    for (const pass of [false, true]) {
      if (pass) { gl.enable(gl.BLEND); gl.depthMask(false); } else { gl.disable(gl.BLEND); gl.depthMask(true); }
      for (const dr of draws) {
        if (dr.alpha !== pass) continue;
        attr(carP, 'p', dr.p, 3); attr(carP, 'n', dr.n, 3); attr(carP, 'uv', dr.uv, 2);
        const t = dr.tex && dr.tex.tex;
        gl.activeTexture(gl.TEXTURE0); gl.bindTexture(gl.TEXTURE_2D, t || null);
        gl.uniform1f(u(carP, 'hasTex'), t ? 1 : 0);
        gl.uniform3fv(u(carP, 'baseColor'), dr.color);
        gl.uniform1f(u(carP, 'alphaMode'), dr.alpha ? 1 : 0);
        gl.uniform1f(u(carP, 'gloss'), dr.gloss); gl.uniform1f(u(carP, 'metal'), dr.metal);
        gl.uniform1f(u(carP, 'emissive'), dr.cls === 'light' ? 0.25 : 0);
        gl.uniform1f(u(carP, 'hl'), dr.mi === highlight ? 1 : 0);
        gl.drawArrays(gl.TRIANGLES, 0, dr.count);
      }
    }
    gl.depthMask(true);
    raf = requestAnimationFrame(draw);
  }
  draw();
  return {
    setHighlight(mi) { highlight = mi; },
    destroy() {
      cancelAnimationFrame(raf);
      window.removeEventListener('mouseup', up);
      canvas.onmousedown = canvas.onmousemove = canvas.onwheel = canvas.ondblclick = canvas.oncontextmenu = null;
      for (const b of buffers) gl.deleteBuffer(b);
      for (const t of textures) gl.deleteTexture(t);
    },
  };
}

// ---------- WebGL / matrix helpers ----------
function shader(gl,t,src){const s=gl.createShader(t);gl.shaderSource(s,src);gl.compileShader(s);if(!gl.getShaderParameter(s,gl.COMPILE_STATUS))throw new Error(gl.getShaderInfoLog(s));return s}function link(gl,v,f){const p=gl.createProgram();gl.attachShader(p,shader(gl,gl.VERTEX_SHADER,v));gl.attachShader(p,shader(gl,gl.FRAGMENT_SHADER,f));gl.linkProgram(p);if(!gl.getProgramParameter(p,gl.LINK_STATUS))throw new Error(gl.getProgramInfoLog(p));return p}function bindAttr(gl,p,n,data){const b=gl.createBuffer();gl.bindBuffer(gl.ARRAY_BUFFER,b);gl.bufferData(gl.ARRAY_BUFFER,new Float32Array(data),gl.STATIC_DRAW);const a=gl.getAttribLocation(p,n);gl.enableVertexAttribArray(a);gl.vertexAttribPointer(a,3,gl.FLOAT,false,0,0)}function resizeCanvas(c){const q=devicePixelRatio||1,w=Math.floor(c.clientWidth*q),h=Math.floor(c.clientHeight*q);if(c.width!==w||c.height!==h){c.width=w;c.height=h}}function perspective(f,a,n,fa){const q=1/Math.tan(f/2),nf=1/(n-fa);return[q/a,0,0,0,0,q,0,0,0,0,(fa+n)*nf,-1,0,0,2*fa*n*nf,0]}function rotationX(a){const c=Math.cos(a),s=Math.sin(a);return[1,0,0,0,0,c,s,0,0,-s,c,0,0,0,0,1]}function rotationZ(a){const c=Math.cos(a),s=Math.sin(a);return[c,s,0,0,-s,c,0,0,0,0,1,0,0,0,0,1]}function mul(a,b){const o=new Array(16).fill(0);for(let c=0;c<4;c++)for(let r=0;r<4;r++)for(let k=0;k<4;k++)o[c*4+r]+=a[k*4+r]*b[c*4+k];return o}function lookAt(e,t,u){const z=norm(sub(e,t)),x=norm(cross(u,z)),y=cross(z,x);return[x[0],y[0],z[0],0,x[1],y[1],z[1],0,x[2],y[2],z[2],0,-dot(x,e),-dot(y,e),-dot(z,e),1]}function sub(a,b){return[a[0]-b[0],a[1]-b[1],a[2]-b[2]]}function dot(a,b){return a[0]*b[0]+a[1]*b[1]+a[2]*b[2]}function cross(a,b){return[a[1]*b[2]-a[2]*b[1],a[2]*b[0]-a[0]*b[2],a[0]*b[1]-a[1]*b[0]]}function norm(a){const l=Math.hypot(...a)||1;return a.map(x=>x/l)}
