// Development-only parity fixture generator for the native Go renderer.
//
// This file imports the frozen rig and emits named reference sets for ticket
// 01 (small set) and ticket 02 (full static grid, named boundaries and the
// 288-frame demonstration). It is NOT part of the runtime or the Go build:
// normal `go build`/`go test` must never execute Node. Generate to temporary
// destinations first, review them, then copy approved data into
// internal/app/mascot/testdata/{rig-v1,loop-v1}.json.
//
//   node art/mascot-fixtures.mjs [--rig PATH] [--loop PATH] [--check-clip] [--pretty]
//
//   --rig PATH     output for the static grid + boundary cases  (default /tmp/mascot-rig-v1.json)
//   --loop PATH    output for the 288-frame demonstration      (default /tmp/mascot-loop-v1.json)
//   --check-clip   compare regenerated demo cells to art/mascot-loop.json and report drift
//   --pretty       indent the JSON (larger; the committed fixtures are compact)
//
// JSON output is compact by default because the full matrix is large; the
// schema itself is unchanged from ticket 01.

import { createHash } from 'node:crypto';
import { readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

import { palette, moods, neutral, render, loopPose } from './mascot-rig.mjs';

const args = process.argv.slice(2);
let rigOut = '/tmp/mascot-rig-v1.json';
let loopOut = '/tmp/mascot-loop-v1.json';
let checkClip = false;
let pretty = false;
for (let i = 0; i < args.length; i++) {
  if (args[i] === '--rig' && args[i + 1]) rigOut = args[++i];
  else if (args[i] === '--loop' && args[i + 1]) loopOut = args[++i];
  else if (args[i] === '--check-clip') checkClip = true;
  else if (args[i] === '--pretty') pretty = true;
  else if (args[i] === '--help') {
    process.stdout.write('Usage: node art/mascot-fixtures.mjs [--rig PATH] [--loop PATH] [--check-clip] [--pretty]\n');
    process.exit(0);
  } else {
    process.stderr.write('Unknown option: ' + args[i] + '\n');
    process.exit(1);
  }
}

const here = dirname(fileURLToPath(import.meta.url));
const rigPath = join(here, 'mascot-rig.mjs');
const clipPath = join(here, 'mascot-loop.json');
const sha256 = path => createHash('sha256').update(readFileSync(path)).digest('hex');
const dump = value => JSON.stringify(value, null, pretty ? 2 : 0) + '\n';

const compactPose = pose => ({
  yaw: pose.yaw,
  pitch: pose.pitch,
  eyeOpen: pose.eyeOpen,
  lift: pose.lift,
  bob: pose.bob,
});

// A case stores the merged (full) input the renderer actually sees, the
// sanitized output pose and the 384 [glyph,fg,bg] triples.
function makeCase(name, partial, background) {
  const input = { ...neutral, ...partial };
  const frame = render(partial, background);
  return {
    name,
    input: compactPose(input),
    pose: compactPose(frame.pose),
    matchingBackground: background,
    cells: frame.cells.map(c => [c.glyph, c.fg, c.bg]),
  };
}

const provenance = () => ({
  schema: 1,
  generator: 'art/mascot-fixtures.mjs',
  generatorSHA256: sha256(fileURLToPath(import.meta.url)),
  rig: 'art/mascot-rig.mjs',
  rigSHA256: sha256(rigPath),
  node: process.version,
  command: process.argv.join(' '),
  width: 32,
  height: 12,
  palette: [null, ...palette.slice(1)],
});

// ---------------------------------------------------------------------------
// Static grid: yaw × pitch × mood × background = 5 × 3 × 5 × 5 = 375 cases.
// Deterministic names/order; each case carries its own matching background.
// ---------------------------------------------------------------------------
const gridYaw = [-35, -17.5, 0, 17.5, 35];
const gridPitch = [0, 14, 20];
const gridMoods = ['calm', 'proud', 'worried', 'sleepy', 'flinch'];
const gridBackgrounds = ['#202e27', '#faf8f0', '#000000', '#161b22', '#ffffff'];
const num = x => String(x);
const tight = hex => hex.slice(1);

const gridCases = [];
for (const yaw of gridYaw) {
  for (const pitch of gridPitch) {
    for (const mood of gridMoods) {
      for (const background of gridBackgrounds) {
        const name = `grid-y${num(yaw)}-p${num(pitch)}-${mood}-${tight(background)}`;
        gridCases.push(makeCase(name, { yaw, pitch, ...moods[mood] }, background));
      }
    }
  }
}

// ---------------------------------------------------------------------------
// Named boundary cases: combined axes, bob limits, apertures, lift bounds,
// fractional angles, omitted JS fields and finite out-of-range inputs. These
// are individually named, not another unbounded product. `omitted` records
// which fields the JS call left out (neutral, not a Go zero value).
// ---------------------------------------------------------------------------
const BOUNDARY_BG = '#161b22';
const boundary = [
  // bob limits at neutral angles.
  ['bob-neg-limit', { bob: -0.06 }, ['yaw', 'pitch', 'eyeOpen', 'lift']],
  ['bob-zero', { bob: 0 }, ['yaw', 'pitch', 'eyeOpen', 'lift']],
  ['bob-pos-limit', { bob: 0.06 }, ['yaw', 'pitch', 'eyeOpen', 'lift']],
  // combined yaw/pitch with bob.
  ['combo-yaw35-pitch20-bob-neg', { yaw: 35, pitch: 20, bob: -0.06 }, ['eyeOpen', 'lift']],
  ['combo-yawneg35-pitch0-bob-pos', { yaw: -35, pitch: 0, bob: 0.06 }, ['eyeOpen', 'lift']],
  ['combo-yaw17.5-pitch14-bob0', { yaw: 17.5, pitch: 14, bob: 0 }, ['eyeOpen', 'lift']],
  ['combo-yawneg17.5-pitch20-bob0', { yaw: -17.5, pitch: 20, bob: 0 }, ['eyeOpen', 'lift']],
  // eye apertures used by moods plus 0/1 limits.
  ['eye-0', { eyeOpen: 0 }, ['yaw', 'pitch', 'lift', 'bob']],
  ['eye-0.08', { eyeOpen: 0.08 }, ['yaw', 'pitch', 'lift', 'bob']],
  ['eye-0.20', { eyeOpen: 0.20 }, ['yaw', 'pitch', 'lift', 'bob']],
  ['eye-0.50', { eyeOpen: 0.50 }, ['yaw', 'pitch', 'lift', 'bob']],
  ['eye-0.80', { eyeOpen: 0.80 }, ['yaw', 'pitch', 'lift', 'bob']],
  ['eye-0.95', { eyeOpen: 0.95 }, ['yaw', 'pitch', 'lift', 'bob']],
  ['eye-1', { eyeOpen: 1 }, ['yaw', 'pitch', 'lift', 'bob']],
  // lift limits and flat mouth.
  ['lift-low-limit', { lift: -0.035 }, ['yaw', 'pitch', 'eyeOpen', 'bob']],
  ['lift-flat', { lift: 0 }, ['yaw', 'pitch', 'eyeOpen', 'bob']],
  ['lift-high-limit', { lift: 0.10 }, ['yaw', 'pitch', 'eyeOpen', 'bob']],
  // fractional / off-grid angles.
  ['frac-yaw12.34-pitch7.77', { yaw: 12.34, pitch: 7.77 }, ['eyeOpen', 'lift', 'bob']],
  ['frac-yawneg22.4-pitch10.5', { yaw: -22.4, pitch: 10.5 }, ['eyeOpen', 'lift', 'bob']],
  ['frac-all', { yaw: 33.33, pitch: 19.99, eyeOpen: 0.5123, lift: 0.0345, bob: 0.0123 }, []],
  // omitted fields: JS render merges neutral, so every omitted field is neutral.
  ['omitted-all', {}, ['yaw', 'pitch', 'eyeOpen', 'lift', 'bob']],
  ['omitted-yaw', { yaw: 20 }, ['pitch', 'eyeOpen', 'lift', 'bob']],
  ['omitted-lift', { lift: 0 }, ['yaw', 'pitch', 'eyeOpen', 'bob']],
  ['omitted-eye', { eyeOpen: 1 }, ['yaw', 'pitch', 'lift', 'bob']],
  // finite out-of-range inputs clamp to the bounds.
  ['over-yaw', { yaw: 100 }, ['pitch', 'eyeOpen', 'lift', 'bob']],
  ['under-yaw', { yaw: -100 }, ['pitch', 'eyeOpen', 'lift', 'bob']],
  ['over-pitch', { pitch: 50 }, ['yaw', 'eyeOpen', 'lift', 'bob']],
  ['under-pitch', { pitch: -50 }, ['yaw', 'eyeOpen', 'lift', 'bob']],
  ['over-eye', { eyeOpen: 2 }, ['yaw', 'pitch', 'lift', 'bob']],
  ['under-eye', { eyeOpen: -1 }, ['yaw', 'pitch', 'lift', 'bob']],
  ['over-lift', { lift: 5 }, ['yaw', 'pitch', 'eyeOpen', 'bob']],
  ['under-lift', { lift: -5 }, ['yaw', 'pitch', 'eyeOpen', 'bob']],
  ['over-bob', { bob: 1 }, ['yaw', 'pitch', 'eyeOpen', 'lift']],
  ['under-bob', { bob: -1 }, ['yaw', 'pitch', 'eyeOpen', 'lift']],
  ['over-all', { yaw: 999, pitch: 999, eyeOpen: 999, lift: 999, bob: 999 }, []],
  ['under-all', { yaw: -999, pitch: -999, eyeOpen: -999, lift: -999, bob: -999 }, []],
];

const boundaryCases = boundary.map(([name, partial, omitted]) => {
  const kase = makeCase(`bnd-${name}`, partial, BOUNDARY_BG);
  kase.omitted = omitted;
  return kase;
});

const rigManifest = {
  ...provenance(),
  set: 'rig-v1',
  note: 'static grid (375) + named boundary cases; palette[0] null, matching RGB per case',
  cases: [...gridCases, ...boundaryCases],
};

// ---------------------------------------------------------------------------
// Demonstration: render(loopPose(i/24,'calm')) for i=0..287 at the clip's
// matching background. The clip is a reference input, never runtime playback.
// t=0 and t=12 are stored separately as the seam; the duplicate final frame is
// NOT appended.
// ---------------------------------------------------------------------------
const LOOP_BG = '#202e27';
const loopFrames = Array.from({ length: 288 }, (_, i) => {
  const input = loopPose(i / 24, 'calm');
  const frame = render(input, LOOP_BG);
  return { i, input: compactPose(input), pose: compactPose(frame.pose), cells: frame.cells.map(c => [c.glyph, c.fg, c.bg]) };
});
const seam = [0, 12].map(t => {
  const input = loopPose(t, 'calm');
  const frame = render(input, LOOP_BG);
  return { t, input: compactPose(input), pose: compactPose(frame.pose), cells: frame.cells.map(c => [c.glyph, c.fg, c.bg]) };
});

const loopManifest = {
  ...provenance(),
  set: 'loop-v1',
  matchingBackground: LOOP_BG,
  fps: 24,
  loop: true,
  mood: 'calm',
  frames: loopFrames,
  seam,
};

// ---------------------------------------------------------------------------
// Independent JS sanitization check for NaN/Inf (JSON cannot carry them) plus
// the documented omitted-field mapping (neutral, not a zero value).
// ---------------------------------------------------------------------------
function selfCheck() {
  const eq = (a, b, what) => {
    if (JSON.stringify(a) !== JSON.stringify(b)) throw new Error(`self-check failed: ${what}: ${JSON.stringify(a)} != ${JSON.stringify(b)}`);
  };
  const neutralPose = compactPose(render({}).pose);
  eq(render({ yaw: NaN, pitch: Infinity, eyeOpen: -Infinity, lift: NaN, bob: Infinity }).pose, neutralPose, 'NaN/Inf -> neutral');
  eq(render({ yaw: 100, pitch: 50, eyeOpen: 2, lift: 5, bob: 1 }).pose,
    { yaw: 35, pitch: 20, eyeOpen: 1, lift: 0.10, bob: 0.06 }, 'finite out-of-range clamps');
  eq(render({ yaw: -100, pitch: -50, eyeOpen: -1, lift: -5, bob: -1 }).pose,
    { yaw: -35, pitch: 0, eyeOpen: 0, lift: -0.035, bob: -0.06 }, 'finite out-of-range low clamps');
  // An omitted field is neutral: render({yaw:0}) must keep neutral pitch/eye/lift.
  eq(render({ yaw: 0 }).pose, neutralPose, 'omitted fields stay neutral');
  return neutralPose;
}

const neutralPose = selfCheck();

// ---------------------------------------------------------------------------
// Optional drift check against the existing clip (never rewrites it).
// ---------------------------------------------------------------------------
function checkAgainstClip() {
  const clip = JSON.parse(readFileSync(clipPath, 'utf8'));
  if (clip.frames.length !== loopFrames.length) {
    throw new Error(`clip has ${clip.frames.length} frames, regenerated ${loopFrames.length}`);
  }
  let frameDiffs = 0, cellDiffs = 0, firstAt = -1;
  for (let i = 0; i < loopFrames.length; i++) {
    const a = clip.frames[i], b = loopFrames[i];
    if (a.pose.yaw !== b.pose.yaw || a.pose.pitch !== b.pose.pitch || a.pose.eyeOpen !== b.pose.eyeOpen || a.pose.lift !== b.pose.lift || a.pose.bob !== b.pose.bob) {
      frameDiffs++;
    }
    for (let c = 0; c < a.cells.length; c++) {
      const x = a.cells[c], y = b.cells[c];
      if (x[0] !== y[0] || x[1] !== y[1] || x[2] !== y[2]) {
        if (firstAt < 0) firstAt = i * 384 + c;
        cellDiffs++;
      }
    }
  }
  return { frames: clip.frames.length, frameDiffs, cellDiffs, firstAt };
}

writeFileSync(rigOut, dump(rigManifest));
writeFileSync(loopOut, dump(loopManifest));
process.stdout.write(`wrote ${rigOut} (${gridCases.length} grid + ${boundaryCases.length} boundary cases)\n`);
process.stdout.write(`wrote ${loopOut} (${loopFrames.length} frames + ${seam.length} seam, rig ${rigManifest.rigSHA256.slice(0, 12)})\n`);
process.stdout.write(`js self-check ok (neutral ${JSON.stringify(neutralPose)})\n`);

if (checkClip) {
  const d = checkAgainstClip();
  process.stdout.write(`clip drift: frames=${d.frameDiffs} cells=${d.cellDiffs} firstAt=${d.firstAt} of ${d.frames}\n`);
  if (d.frameDiffs !== 0 || d.cellDiffs !== 0) process.exitCode = 2;
}
