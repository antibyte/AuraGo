// A strict SVG path data reader used to check every generated shape (part C draws them with Path2D, which
// silently ignores malformed data, so the generator must never emit any).

const ARGS = { M: 2, L: 2, H: 1, V: 1, C: 6, S: 4, Q: 4, T: 2, A: 7, Z: 0 };
const TOKEN = /\s*,?\s*(?:([MmLlHhVvCcSsQqTtAaZz])|(-?(?:\d+\.?\d*|\.\d+)(?:[eE][-+]?\d+)?))/y;

/** Returns {ok, error?, commands: [{cmd, args}]}. */
export function parsePath(d) {
  const commands = [];
  if (typeof d !== 'string' || !d.trim()) return { ok: false, error: 'empty path', commands };
  TOKEN.lastIndex = 0;
  let cmd = null;
  let args = [];
  let pos = 0;
  const flush = () => {
    if (cmd === null) return null;
    const need = ARGS[cmd.toUpperCase()];
    if (need === 0) {
      if (args.length) return `Z takes no arguments`;
      commands.push({ cmd, args });
      return null;
    }
    if (args.length === 0 || args.length % need !== 0) return `${cmd} has ${args.length} numbers, needs a multiple of ${need}`;
    for (let i = 0; i < args.length; i += need) commands.push({ cmd: i === 0 ? cmd : cmd === 'M' ? 'L' : cmd === 'm' ? 'l' : cmd, args: args.slice(i, i + need) });
    return null;
  };
  while (pos < d.length) {
    TOKEN.lastIndex = pos;
    const m = TOKEN.exec(d);
    if (!m) {
      if (d.slice(pos).trim() === '') break;
      return { ok: false, error: `unexpected text at ${pos}: ${d.slice(pos, pos + 12)}`, commands };
    }
    pos = TOKEN.lastIndex;
    if (m[1]) {
      const err = flush();
      if (err) return { ok: false, error: err, commands };
      if (cmd === null && m[1].toUpperCase() !== 'M') return { ok: false, error: 'path must start with M', commands };
      cmd = m[1];
      args = [];
    } else {
      if (cmd === null) return { ok: false, error: 'number before a command', commands };
      args.push(Number(m[2]));
    }
  }
  const err = flush();
  if (err) return { ok: false, error: err, commands };
  for (const c of commands) {
    if (c.args.some((v) => !Number.isFinite(v))) return { ok: false, error: 'non-finite number', commands };
    if (c.cmd.toUpperCase() === 'A' && ![0, 1].includes(c.args[3])) return { ok: false, error: 'arc large-arc flag must be 0 or 1', commands };
    if (c.cmd.toUpperCase() === 'A' && ![0, 1].includes(c.args[4])) return { ok: false, error: 'arc sweep flag must be 0 or 1', commands };
  }
  if (!commands.length) return { ok: false, error: 'no commands', commands };
  return { ok: true, commands };
}
