// Plan dry run: applies the Create, Edit and Append blocks of the part files, in task order, to a scratch tree.
// Usage: node dry.js <partsDir> <tree> [stopTask] [--check]   --check parses and matches in memory, writes nothing.
const fs = require('fs');
const path = require('path');
const { execFileSync } = require('child_process');

const [partsDir, tree, stopArg, flag] = process.argv.slice(2);
const stop = stopArg && stopArg !== '-' ? stopArg : null;
const check = flag === '--check';
const ORDER = 'B0 A1 A2 A3 A4 A5 A6 A7 A7a A8 A8a A9 A10 B1 B2 B3 B4 B5 C1 C2 C3 C4 C4a C5 C6 C7a C7 C7b C8 C9 C10 D1 D2 D3 D4 D5 D6 D6a D7 E2 E3 E3a E4 F1 F2 F3 F4'.split(' ');

function parseTasks() {
  const tasks = {};
  for (const f of fs.readdirSync(partsDir).filter((n) => /^part-.*\.md$/.test(n)).sort()) {
    const lines = fs.readFileSync(path.join(partsDir, f), 'utf8').replace(/\r\n/g, '\n').split('\n');
    let cur = null;
    let i = 0;
    const readFence = (start) => {
      // start is the index of the opening fence. Returns [text, nextIndex].
      const open = lines[start];
      const m = open.match(/^(\s*)(`{3,}|~{3,})/);
      const indent = m[1];
      const fence = m[2];
      const body = [];
      let j = start + 1;
      for (; j < lines.length; j++) {
        const t = lines[j];
        const trimmed = t.trim();
        if (trimmed === fence && t.startsWith(indent)) break;
        body.push(indent && t.startsWith(indent) ? t.slice(indent.length) : t);
      }
      return [body.join('\n'), j + 1];
    };
    const nextFence = (from) => {
      let j = from;
      while (j < lines.length && !/^\s*(`{3,}|~{3,})/.test(lines[j])) j++;
      return j < lines.length ? j : -1;
    };
    while (i < lines.length) {
      const l = lines[i];
      const h = l.match(/^### Task ([A-Za-z0-9]+):/);
      if (h) { cur = { id: h[1], file: f, line: i + 1, ops: [] }; tasks[cur.id] = cur; i++; continue; }
      if (/^\s*(`{3,}|~{3,})/.test(l)) { const r = readFence(i); i = r[1]; continue; }
      if (!cur) { i++; continue; }
      const t = l.trim();
      let m;
      if ((m = t.match(/^\*\*Create\*\* `([^`]+)`/))) {
        const a = nextFence(i + 1);
        const r = readFence(a);
        cur.ops.push({ kind: 'create', path: m[1], text: r[0], line: i + 1 });
        i = r[1];
        continue;
      }
      if ((m = t.match(/^\*\*Append to\*\* `([^`]+)`/))) {
        const a = nextFence(i + 1);
        const r = readFence(a);
        cur.ops.push({ kind: 'append', path: m[1], text: r[0], line: i + 1 });
        i = r[1];
        continue;
      }
      if ((m = t.match(/^\*\*Edit\*\* `([^`]+)`/))) {
        const a = nextFence(i + 1);
        const r = readFence(a);
        const b = nextFence(r[1]);
        const between = lines.slice(r[1], b).join(' ');
        if (!/New:/.test(between)) throw new Error(`${f}:${i + 1} Edit without New`);
        const s = readFence(b);
        cur.ops.push({ kind: 'edit', path: m[1], old: r[0], new: s[0], line: i + 1 });
        i = s[1];
        continue;
      }
      i++;
    }
  }
  return tasks;
}

const tasks = parseTasks();
const problems = [];
for (const id of ORDER) if (!tasks[id]) problems.push(`task ${id}: missing from the parts`);
for (const id of Object.keys(tasks)) if (!ORDER.includes(id)) problems.push(`task ${id}: not in the order`);

const mem = new Map();
const baseCache = new Map();
const repo = path.dirname(path.dirname(path.dirname(path.dirname(partsDir.replace(/[\\/]$/, '')))));
function baseRead(p) {
  if (baseCache.has(p)) return baseCache.get(p);
  let v = null;
  if (check) {
    try { v = execFileSync('git', ['-C', repo, 'show', 'HEAD:' + p], { encoding: 'utf8', maxBuffer: 1 << 28, stdio: ['ignore', 'pipe', 'ignore'] }).replace(/\r\n/g, '\n'); } catch (_) { v = null; }
  } else {
    const full = path.join(tree, p);
    if (fs.existsSync(full)) v = fs.readFileSync(full, 'utf8').replace(/\r\n/g, '\n');
  }
  baseCache.set(p, v);
  return v;
}
const read = (p) => (mem.has(p) ? mem.get(p) : baseRead(p));
const count = (s, sub) => { let n = 0; let i = 0; while ((i = s.indexOf(sub, i)) >= 0) { n++; i += sub.length || 1; } return n; };

let applied = 0;
const touched = new Set();
for (const id of ORDER) {
  const t = tasks[id];
  if (t) {
    for (const op of t.ops) {
      const where = `${id} ${t.file}:${op.line} ${op.path}`;
      if (/^docs\//.test(op.path) && op.path.includes('/evidence/')) continue;
      const cur = read(op.path);
      if (op.kind === 'create') {
        if (cur !== null) problems.push(`${where}: Create on a file that exists`);
        mem.set(op.path, op.text.endsWith('\n') ? op.text : op.text + '\n');
      } else if (op.kind === 'append') {
        if (cur === null) { problems.push(`${where}: Append to a missing file`); continue; }
        mem.set(op.path, cur.replace(/\n?$/, '\n') + (op.text.endsWith('\n') ? op.text : op.text + '\n'));
      } else {
        if (cur === null) { problems.push(`${where}: Edit of a missing file`); continue; }
        const n = count(cur, op.old);
        if (n !== 1) {
          const firstLine = op.old.split('\n').find((x) => x.trim()) || '';
          problems.push(`${where}: old text matches ${n} times; first line: ${firstLine.trim().slice(0, 100)}`);
          continue;
        }
        mem.set(op.path, cur.replace(op.old, () => op.new));
      }
      touched.add(op.path);
      applied++;
    }
  }
  if (id === stop) break;
}

if (!check) {
  for (const [p, text] of mem) {
    const full = path.join(tree, p);
    fs.mkdirSync(path.dirname(full), { recursive: true });
    fs.writeFileSync(full, text);
  }
}
console.log(`applied ${applied} changes to ${touched.size} files, ${problems.length} problems`);
for (const p of problems) console.log(' - ' + p);
