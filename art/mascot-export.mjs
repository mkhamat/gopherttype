import { palette, moods, neutral, rig, render, ansi, svg, loopPose } from './mascot-rig.mjs';

const args = process.argv.slice(2);
const flags = new Set(['--play', '--json', '--svg', '--clip', '--rig', '--help']);
const options = new Set(['--yaw', '--pitch', '--mood', '--background']);
const values = {};
try {
  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (flags.has(arg)) values[arg] = true;
    else if (options.has(arg) && args[i + 1] && !args[i + 1].startsWith('--')) values[arg] = args[++i];
    else throw new Error('Unknown or incomplete option: ' + arg);
  }
  const modes = ['--play', '--json', '--svg', '--clip', '--rig'].filter(key => values[key]);
  if (modes.length > 1) throw new Error('Choose only one output mode.');
  for (const key of ['--yaw', '--pitch']) {
    if (values[key] !== undefined && !Number.isFinite(Number(values[key]))) throw new Error(key + ' must be a number.');
  }
  if (values['--mood'] && !moods[values['--mood']]) throw new Error('Unknown mood. Use calm, proud, worried, sleepy, or flinch.');
  if (values['--background'] && !/^#[0-9a-f]{6}$/i.test(values['--background'])) throw new Error('Background must be #RRGGBB.');
} catch (error) {
  process.stderr.write(error.message + '\n');
  process.exit(1);
}

if (values['--help']) {
  process.stdout.write('Usage: node art/mascot-export.mjs [--play | --json | --svg | --clip | --rig]\n  --yaw -35..35  --pitch 0..20  --mood calm|proud|worried|sleepy|flinch\n  --background "#RRGGBB" (matching only; terminal default remains transparent)\nDefault: one ANSI frame. --json: one cell frame. --svg: cell geometry preview.\n--clip: 12 s / 24 FPS loop.\n--rig: geometry. --play: terminal demo; Ctrl-C exits. Loops drive yaw/pitch.\n');
  process.exit(0);
}

const mood = values['--mood'] || 'calm';
const background = values['--background'] || palette[0];
const pose = { ...neutral, ...moods[mood] };
if (values['--yaw'] !== undefined) pose.yaw = Number(values['--yaw']);
if (values['--pitch'] !== undefined) pose.pitch = Number(values['--pitch']);
const metadata = {
  version: 1, width: 32, height: 12,
  palette: [null, ...palette.slice(1)], matchingBackground: background,
  attribution: 'Go gopher by Renee French, CC BY 4.0; adapted terminal art study',
};
const compact = frame => ({ pose: frame.pose, cells: frame.cells.map(c => [c.glyph, c.fg, c.bg]) });

if (values['--play']) {
  if (!process.stdout.isTTY || process.stdout.columns < 32 || process.stdout.rows < 14) {
    process.stderr.write('Playback needs a terminal at least 32 columns × 14 rows. Use --clip for file export.\n');
    process.exit(1);
  }
  process.stdout.write('\x1b[?1049h\x1b[?25l');
  let restored = false;
  const restore = () => {
    if (restored) return;
    restored = true;
    process.stdout.write('\x1b[0m\x1b[?25h\x1b[?1049l');
  };
  process.on('exit', restore);
  for (const signal of ['SIGINT', 'SIGTERM', 'SIGHUP']) process.on(signal, () => process.exit(0));
  const start = performance.now();
  const tick = () => {
    if (process.stdout.columns < 32 || process.stdout.rows < 14) {
      process.stdout.write('\x1b[H\x1b[2JNeed 32×14; Ctrl-C exits.');
    } else {
      const frame = render(loopPose((performance.now() - start) / 1000, mood), background);
      process.stdout.write('\x1b[H' + ansi(frame) + '\x1b[14;1HDraft rig · Ctrl-C exits\x1b[K');
    }
    const interval = 1000 / 24;
    const next = start + (Math.floor((performance.now() - start) / interval) + 1) * interval;
    setTimeout(tick, Math.max(0, next - performance.now()));
  };
  tick();
} else if (values['--rig']) {
  process.stdout.write(JSON.stringify({ ...metadata, neutral, moods, ...rig }, null, 2) + '\n');
} else if (values['--clip']) {
  const frames = Array.from({ length: 288 }, (_, i) => compact(render(loopPose(i / 24, mood), background)));
  process.stdout.write(JSON.stringify({ ...metadata, fps: 24, loop: true, mood, frames }) + '\n');
} else if (values['--json']) {
  process.stdout.write(JSON.stringify({ ...metadata, ...compact(render(pose, background)) }) + '\n');
} else if (values['--svg']) {
  process.stdout.write(svg(render(pose, background), background) + '\n');
} else {
  process.stdout.write(ansi(render(pose, background)) + '\n');
}
