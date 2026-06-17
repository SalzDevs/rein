// Tests for the rein Node.js client.
//
// These tests require the rein binary to be installed and on
// $PATH (or set $REIN_BIN to its location).

'use strict';

const { spawn, spawnSync } = require('child_process');
const test = require('node:test');
const assert = require('node:assert');

const { run, start, exec, findRein } = require('../src/rein');

function reinAvailable() {
  try {
    const r = spawnSync(findRein(), ['version'], { encoding: 'utf8' });
    return r.status === 0 && r.stdout.includes('rein');
  } catch (e) {
    return false;
  }
}

const skip = !reinAvailable();

test('run: basic', { skip }, async () => {
  const result = await run('echo hello');
  assert.equal(result.exitCode, 0);
  assert.match(result.stdout, /hello/);
  assert.equal(result.ok, true);
});

test('run: non-zero exit', { skip }, async () => {
  const result = await run('exit 7');
  assert.equal(result.exitCode, 7);
  assert.equal(result.ok, false);
});

test('run: timeout', { skip }, async () => {
  const start = Date.now();
  const result = await run('sleep 5', { timeout: 0.2 });
  const elapsed = Date.now() - start;
  assert.ok(elapsed < 2000, `expected quick timeout, took ${elapsed}ms`);
  assert.notEqual(result.exitCode, 0);
});

test('run: env', { skip }, async () => {
  const result = await run('echo $REIN_TEST_VAR', {
    env: { REIN_TEST_VAR: 'from-env' },
  });
  assert.match(result.stdout, /from-env/);
});

test('start: streams lines', { skip }, async () => {
  const session = start('echo line-1; echo line-2; echo line-3');
  const lines = [];
  session.on('line', (l) => lines.push(l.text));
  await new Promise((resolve) => {
    session.on('exit', resolve);
  });
  assert.ok(lines.includes('line-1'));
  assert.ok(lines.includes('line-2'));
  assert.ok(lines.includes('line-3'));
});

test('start: stop', { skip }, async () => {
  const session = start('sleep 30');
  const result = await session.stop();
  assert.notEqual(result.exitCode, 0);
});

test('exec: input', { skip }, async () => {
  const session = exec('cat');
  const seen = new Promise((resolve) => {
    session.on('line', (l) => {
      if (l.text === 'hello-from-input') {
        resolve();
      }
    });
  });
  session.write('hello-from-input\n');
  await seen;
  await session.stop();
});
