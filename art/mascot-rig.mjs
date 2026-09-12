export const palette = ['#202e27', '#347b91', '#529aaf', '#70b7c8', '#8dcddb', '#b2e1e8', '#ffffff', '#10272f', '#efd0a0'];
export const moods = {
  calm: { eyeOpen: 0.95, lift: 0.06 },
  proud: { eyeOpen: 1, lift: 0.10 },
  worried: { eyeOpen: 0.80, lift: -0.035 },
  sleepy: { eyeOpen: 0.20, lift: 0 },
  flinch: { eyeOpen: 0.08, lift: 0 },
};
export const neutral = { yaw: 0, pitch: 0, eyeOpen: 0.95, lift: 0.06, bob: 0 };
export const rig = {
  size: [32, 12],
  samples: [128, 96],
  camera: { center: [-0.025, 0.15], width: 3.2 },
  pivot: [0, 0.14, 0],
  ellipsoids: [
    { id: 'body', center: [0, -0.78, -0.16], radii: [0.86, 0.42, 0.48], role: 'fur', fixed: true },
    { id: 'head', center: [0, 0.10, 0], radii: [1.00, 0.90, 0.64], role: 'fur' },
    { id: 'cheeks', center: [0, -0.50, -0.04], radii: [0.93, 0.70, 0.62], role: 'fur' },
    { id: 'ear-left', center: [-0.91, 0.53, -0.08], radii: [0.225, 0.23, 0.15], role: 'fur' },
    { id: 'ear-right', center: [0.91, 0.53, -0.08], radii: [0.225, 0.23, 0.15], role: 'fur' },
    { id: 'ear-inner-left', center: [-0.945, 0.54, 0.065], radii: [0.085, 0.095, 0.035], role: 'ear' },
    { id: 'ear-inner-right', center: [0.945, 0.54, 0.065], radii: [0.085, 0.095, 0.035], role: 'ear' },
    { id: 'eye-left', center: [-0.385, 0.45, 0.63], radii: [0.32, 0.325, 0.13], role: 'eye' },
    { id: 'eye-right', center: [0.385, 0.45, 0.63], radii: [0.32, 0.325, 0.13], role: 'eye' },
    { id: 'pupil-left', center: [-0.435, 0.425, 0.78], radii: [0.085, 0.095, 0.025], role: 'pupil', eye: 'eye-left' },
    { id: 'pupil-right', center: [0.335, 0.425, 0.78], radii: [0.085, 0.095, 0.025], role: 'pupil', eye: 'eye-right' },
    { id: 'muzzle-left', center: [-0.095, -0.02, 0.72], radii: [0.17, 0.115, 0.09], role: 'muzzle' },
    { id: 'muzzle-right', center: [0.095, -0.02, 0.72], radii: [0.17, 0.115, 0.09], role: 'muzzle' },
    { id: 'mouth', center: [0, -0.145, 0.735], radii: [0.185, 0.055, 0.035], role: 'mouth' },
    { id: 'nose', center: [0, 0.16, 0.79], radii: [0.14, 0.075, 0.07], role: 'nose' },
  ],
  teeth: [
    { id: 'tooth-divider', center: [0, -0.235, 0.745], radii: [0.026, 0.12, 0.03], role: 'seam', corner: 0.008 },
    { id: 'tooth-left', center: [-0.10, -0.235, 0.745], radii: [0.075, 0.105, 0.045], role: 'tooth', corner: 0.035 },
    { id: 'tooth-right', center: [0.10, -0.235, 0.745], radii: [0.075, 0.105, 0.045], role: 'tooth', corner: 0.035 },
  ],
};

const dot = (a, b) => a.reduce((sum, x, i) => sum + x * b[i], 0);
const add = (a, b) => a.map((x, i) => x + b[i]);
const sub = (a, b) => a.map((x, i) => x - b[i]);
const scale = (a, s) => a.map(x => x * s);
const unit = a => scale(a, 1 / Math.sqrt(dot(a, a)));
const clamp = (x, lo, hi) => Math.min(hi, Math.max(lo, x));
const light = unit([-0.55, 0.75, 1]);
export const quadrants = [' ', '▘', '▝', '▀', '▖', '▌', '▞', '▛', '▗', '▚', '▐', '▜', '▄', '▙', '▟', '█'];
const weight = role => ['pupil', 'nose', 'mouth', 'tooth', 'seam', 'ear'].includes(role) ? 2 : role === 'muzzle' ? 1.5 : 1;
const rgb = hex => [1, 3, 5].map(i => parseInt(hex.slice(i, i + 2), 16));

function rotation(pose) {
  const yaw = pose.yaw * Math.PI / 180, pitch = pose.pitch * Math.PI / 180;
  const cy = Math.cos(yaw), sy = Math.sin(yaw), cp = Math.cos(pitch), sp = Math.sin(pitch);
  return {
    forward([x, y, z]) {
      const yy = cp * y - sp * z, zz = sp * y + cp * z;
      return [cy * x + sy * zz, yy, -sy * x + cy * zz];
    },
    inverse([x, y, z]) {
      const xx = cy * x - sy * z, zz = sy * x + cy * z;
      return [xx, cp * y + sp * zz, -sp * y + cp * zz];
    },
  };
}

function intersect(origin, direction, part) {
  const o = sub(origin, part.center), r = part.radii;
  if (part.corner) {
    if (Math.abs(direction[2]) < 1e-10) return null;
    const t = (r[2] - o[2]) / direction[2];
    if (t <= 0) return null;
    const point = add(origin, scale(direction, t));
    const x = Math.abs(point[0] - part.center[0]) - r[0] + part.corner;
    const y = Math.abs(point[1] - part.center[1]) - r[1] + part.corner;
    if (Math.hypot(Math.max(x, 0), Math.max(y, 0)) + Math.min(Math.max(x, y), 0) > part.corner) return null;
    return { t, point, normal: [0, 0, 1], part };
  }
  const oo = o.map((x, i) => x / r[i]), dd = direction.map((x, i) => x / r[i]);
  const a = dot(dd, dd), b = 2 * dot(oo, dd), c = dot(oo, oo) - 1;
  const disc = b * b - 4 * a * c;
  if (disc < 0) return null;
  const root = Math.sqrt(disc);
  let t = (-b - root) / (2 * a);
  if (t <= 0) t = (-b + root) / (2 * a);
  if (t <= 0) return null;
  const point = add(origin, scale(direction, t));
  const normal = unit(sub(point, part.center).map((x, i) => x / (r[i] * r[i])));
  return { t, point, normal, part };
}

function material(hit, pose, rotate) {
  const { part, point } = hit;
  const localNormal = part.role === 'eye' || part.role === 'pupil' || ['body', 'head', 'cheeks'].includes(part.id)
    ? unit([point[0], (point[1] + 0.24 - (part.fixed ? rig.pivot[1] : 0)) / 1.56, point[2] / 0.41])
    : hit.normal;
  const normal = part.fixed ? localNormal : rotate.forward(localNormal);
  const shade = clamp(1 + Math.floor((0.18 + 0.82 * Math.max(0, dot(normal, light))) * 4.99), 1, 5);
  let color = shade, role = part.role;
  if (role === 'eye' || role === 'pupil') {
    const eye = part.eye ? rig.ellipsoids.find(eye => eye.id === part.eye) : part;
    const lid = eye.center[1] + eye.radii[1] * (-1 + 2 * pose.eyeOpen);
    if (point[1] > lid) role = 'fur';
    else color = role === 'pupil' ? 7 : 6;
  } else if (role === 'nose' || role === 'ear' || role === 'mouth' || role === 'seam') color = 7;
  else if (role === 'muzzle' || role === 'tooth') {
    color = role === 'muzzle' ? 8 : 6;
  }
  return { color, role };
}

function sampleScene(pose) {
  const rotate = rotation(pose), direction = [0, 0, -1];
  const localDirection = rotate.inverse(direction);
  const pivot = add(rig.pivot, [0, pose.bob, 0]);
  const parts = [...rig.ellipsoids, ...rig.teeth].map(part => part.role === 'muzzle' || part.role === 'mouth'
    ? { ...part, center: add(part.center, [0, (pose.lift - neutral.lift) * 0.6, 0]) }
    : part);
  const samples = [], [width, height] = rig.samples, scale = width / rig.camera.width;
  for (let sy = 0; sy < height; sy++) {
    for (let sx = 0; sx < width; sx++) {
      const origin = [rig.camera.center[0] + (sx + 0.5 - width / 2) / scale, rig.camera.center[1] + (height / 2 - sy - 0.5) / scale, 3];
      const localOrigin = rotate.inverse(sub(origin, pivot));
      let nearest = null;
      for (const part of parts) {
        const hit = intersect(part.fixed ? origin : localOrigin, part.fixed ? direction : localDirection, part);
        if (hit && (!nearest || hit.t < nearest.t)) nearest = hit;
      }
      samples.push(nearest ? material(nearest, pose, rotate) : { color: 0, role: 'background' });
    }
  }
  return samples;
}

function encode(quarters, distance) {
  const colors = [...new Set(quarters.flat().map(s => s.color))].sort((a, b) => a - b);
  if (colors.length === 1) return { glyph: ' ', fg: colors[0], bg: colors[0] };
  const errors = quarters.map(samples => colors.map(color => samples.reduce((sum, s) => sum + weight(s.role) * distance[s.color][color], 0)));
  let best = Infinity, fg = 0, bg = 0, mask = 0;
  for (let i = 0; i < colors.length; i++) {
    for (let j = i + 1; j < colors.length; j++) {
      if (colors[0] === 0 && colors[i] !== 0) continue;
      let error = 0, bits = 0;
      for (let q = 0; q < 4; q++) {
        error += Math.min(errors[q][i], errors[q][j]);
        if (errors[q][j] < errors[q][i]) bits |= 1 << q;
      }
      if (error < best) { best = error; bg = colors[i]; fg = colors[j]; mask = bits; }
    }
  }
  return { glyph: quadrants[mask], fg, bg };
}

export function render(input = {}, background = palette[0]) {
  const pose = { ...neutral, ...input };
  for (const [key, lo, hi] of [['yaw', -35, 35], ['pitch', 0, 20], ['eyeOpen', 0, 1], ['lift', -0.035, 0.10], ['bob', -0.06, 0.06]]) {
    pose[key] = Number.isFinite(pose[key]) ? clamp(pose[key], lo, hi) : neutral[key];
  }
  const colors = palette.map((hex, i) => rgb(i === 0 ? background : hex));
  const distance = colors.map(a => colors.map(b => a.reduce((sum, x, i) => sum + (x - b[i]) ** 2, 0)));
  const samples = sampleScene(pose), cells = [];
  const [width, height] = rig.size, sw = rig.samples[0], cw = sw / width, ch = rig.samples[1] / height;
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      const quarters = [[], [], [], []];
      for (let dy = 0; dy < ch; dy++) {
        for (let dx = 0; dx < cw; dx++) quarters[(dy < ch / 2 ? 0 : 2) + (dx < cw / 2 ? 0 : 1)].push(samples[(y * ch + dy) * sw + x * cw + dx]);
      }
      cells.push(encode(quarters, distance));
    }
  }
  return { pose, cells };
}

export function ansi(frame) {
  const channel = (index, foreground) => index === 0 ? (foreground ? '39' : '49') : `${foreground ? 38 : 48};2;${rgb(palette[index]).join(';')}`;
  const rows = [];
  for (let y = 0; y < 12; y++) {
    let row = '', previous = '';
    for (const cell of frame.cells.slice(y * 32, (y + 1) * 32)) {
      const style = `${channel(cell.fg, true)};${channel(cell.bg, false)}`;
      if (style !== previous) { row += `\x1b[${style}m`; previous = style; }
      row += cell.glyph;
    }
    rows.push(row + '\x1b[0m');
  }
  return rows.join('\n');
}

export function svg(frame, background = palette[0]) {
  const shapes = [
    '<svg xmlns="http://www.w3.org/2000/svg" width="512" height="384" viewBox="0 0 512 384">',
    '<title>Go gopher — 32 × 12 terminal-cell study</title>',
    '<desc>Procedural cyan gopher with round eyes, a black oval nose, tan muzzle and paired buck teeth. Cell geometry preview; terminal font rendering may differ. Original Go gopher by Renee French, CC BY 4.0.</desc>',
  ];
  const rect = (x, y, w, h, fill) => shapes.push(`<rect x="${x}" y="${y}" width="${w}" height="${h}" fill="${fill}"/>`);
  rect(0, 0, 512, 384, background);
  frame.cells.forEach((cell, i) => {
    const x = i % 32 * 16, y = Math.floor(i / 32) * 32;
    const fg = cell.fg ? palette[cell.fg] : background;
    if (cell.bg) rect(x, y, 16, 32, palette[cell.bg]);
    if (cell.glyph === ' ' || cell.fg === cell.bg) return;
    const quadrant = quadrants.indexOf(cell.glyph);
    if (quadrant < 0) throw new Error('Unsupported cell glyph: ' + cell.glyph);
    for (let q = 0; q < 4; q++) {
      if (quadrant & 1 << q) rect(x + q % 2 * 8, y + Math.floor(q / 2) * 16, 8, 16, fg);
    }
  });
  return [...shapes, '</svg>'].join('\n');
}

export function loopPose(seconds, mood = 'calm') {
  const blinkPhase = seconds % 4;
  const blink = blinkPhase < 0.14 ? Math.abs(blinkPhase / 0.07 - 1) : 1;
  return {
    yaw: 30 * Math.sin(seconds * Math.PI / 3),
    pitch: 12 + 6 * Math.sin(seconds * Math.PI / 6),
    ...moods[mood],
    eyeOpen: (moods[mood] || moods.calm).eyeOpen * blink,
    bob: mood === 'proud' ? 0.025 * Math.sin(seconds * Math.PI * 4) : 0,
  };
}
