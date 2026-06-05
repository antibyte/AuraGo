const { spawn } = require('child_process');
const path = require('path');

const child = spawn(process.execPath, [path.join(__dirname, 'server.js')], {
  cwd: __dirname,
  env: {
    ...process.env,
    PORT: '0',
    AURAGO_BROWSER_AUTOMATION_TOKEN: '',
    AURAGO_BROWSER_AUTOMATION_ALLOW_UNAUTH: '',
  },
  stdio: ['ignore', 'pipe', 'pipe'],
});

let output = '';
child.stdout.on('data', (chunk) => {
  output += chunk.toString();
});
child.stderr.on('data', (chunk) => {
  output += chunk.toString();
});

const timer = setTimeout(() => {
  child.kill('SIGTERM');
  console.error('server stayed running without AURAGO_BROWSER_AUTOMATION_TOKEN');
  process.exit(1);
}, 1500);

child.on('exit', (code) => {
  clearTimeout(timer);
  if (code === 0) {
    console.error('server exited successfully without AURAGO_BROWSER_AUTOMATION_TOKEN');
    process.exit(1);
  }
  if (!/AURAGO_BROWSER_AUTOMATION_TOKEN/.test(output)) {
    console.error('server exit did not explain the missing token');
    process.exit(1);
  }
});
