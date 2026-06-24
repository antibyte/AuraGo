const { spawn } = require('child_process');
const path = require('path');

function expectStartupFailure(name, env, outputPattern) {
  return new Promise((resolve, reject) => {
    const child = spawn(process.execPath, [path.join(__dirname, 'server.js')], {
      cwd: __dirname,
      env: {
        ...process.env,
        PORT: '0',
        AURAGO_BROWSER_AUTOMATION_TOKEN: '',
        AURAGO_BROWSER_AUTOMATION_ALLOW_UNAUTH: '',
        ...env,
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
      reject(new Error(`${name}: server stayed running`));
    }, 1500);

    child.on('exit', (code) => {
      clearTimeout(timer);
      if (code === 0) {
        reject(new Error(`${name}: server exited successfully`));
        return;
      }
      if (!outputPattern.test(output)) {
        reject(new Error(`${name}: startup failure did not include expected message. Output:\n${output}`));
        return;
      }
      resolve();
    });
  });
}

(async () => {
  await expectStartupFailure(
    'missing token',
    {},
    /AURAGO_BROWSER_AUTOMATION_TOKEN is required/
  );

  await expectStartupFailure(
    'placeholder token',
    { AURAGO_BROWSER_AUTOMATION_TOKEN: 'change_me_please' },
    /known placeholder value/
  );
})().catch((error) => {
  console.error(error.message || error);
  process.exit(1);
});
