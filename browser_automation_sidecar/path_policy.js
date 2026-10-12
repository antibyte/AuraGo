const fs = require('fs/promises');
const path = require('path');

function resolveUnder(root, relPath) {
  if (!relPath || typeof relPath !== 'string') throw new Error('path is required');
  const full = path.resolve(root, relPath);
  const normalizedRoot = path.resolve(root);
  if (full !== normalizedRoot && !full.startsWith(normalizedRoot + path.sep)) {
    throw new Error('path escapes allowed root');
  }
  return full;
}

async function assertNoSymlink(root, full) {
  const normalizedRoot = path.resolve(root);
  let current = normalizedRoot;
  const info = await fs.lstat(current).catch((error) => {
    if (error && error.code === 'ENOENT') return null;
    throw error;
  });
  if (info && info.isSymbolicLink()) throw new Error('path must not use a symlink');
  const rel = path.relative(normalizedRoot, full);
  if (rel.startsWith('..')) throw new Error('path escapes allowed root');
  for (const part of rel.split(path.sep)) {
    if (!part || part === '.') continue;
    current = path.join(current, part);
    let stat;
    try {
      stat = await fs.lstat(current);
    } catch (error) {
      if (error && error.code === 'ENOENT') return;
      throw error;
    }
    if (stat.isSymbolicLink()) throw new Error('path must not use a symlink');
  }
}

async function resolveInside(root, relPath) {
  const full = resolveUnder(root, relPath);
  await assertNoSymlink(root, full);
  return full;
}

module.exports = { resolveUnder, resolveInside };
