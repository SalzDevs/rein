// rein client - Node.js wrapper for the rein CLI.
//
// rein is a small tool that runs shell commands the way AI agents
// actually need: with timeouts that work, signals that propagate,
// and process trees that get cleaned up.
//
// This package is a thin Node.js wrapper around the rein CLI
// binary. It speaks NDJSON over stdio and exposes a Promise-based
// API. Install the rein binary first (go install
// github.com/SalzDevs/rein/cmd/rein@latest) and then
// `npm install rein-client`.

'use strict';

const { spawn } = require('child_process');
const readline = require('readline');
const { EventEmitter } = require('events');

/**
 * Find the rein binary. Looks at $REIN_BIN, then PATH.
 */
function findRein() {
  if (process.env.REIN_BIN) {
    return process.env.REIN_BIN;
  }
  return 'rein';
}

/**
 * Run a one-shot command and return a promise that resolves to
 * a Result.
 */
async function run(command, options = {}) {
  const rein = findRein();
  const args = [rein, 'run'];
  if (options.timeout != null) args.push('--timeout', `${Math.round(options.timeout * 1000)}ms`);
  if (options.gracefulTimeout != null) args.push('--graceful-timeout', `${Math.round(options.gracefulTimeout * 1000)}ms`);
  if (options.dir) args.push('--dir', options.dir);
  if (options.env) {
    for (const [k, v] of Object.entries(options.env)) {
      args.push('--env', `${k}=${v}`);
    }
  }
  if (options.pty) args.push('--pty');
  args.push(command);

  return new Promise((resolve, reject) => {
    const proc = spawn(args[0], args.slice(1), { stdio: ['ignore', 'pipe', 'pipe'] });
    let out = '';
    let err = '';
    proc.stdout.on('data', (d) => { out += d.toString(); });
    proc.stderr.on('data', (d) => { err += d.toString(); });
    proc.on('error', reject);
    proc.on('close', () => {
      const line = out.trim();
      if (!line) {
        reject(new Error(`rein run failed: ${err}`));
        return;
      }
      try {
        const msg = JSON.parse(line);
        resolve({
          exitCode: msg.exit_code || 0,
          durationMs: msg.duration_ms || 0,
          stdout: msg.stdout || '',
          stderr: msg.stderr || '',
          err: msg.err || null,
          get ok() { return this.exitCode === 0; },
        });
      } catch (e) {
        reject(new Error(`rein run produced invalid NDJSON: ${line}: ${err}`));
      }
    });
  });
}

/**
 * Session is an EventEmitter that streams output lines from a
 * long-running rein process. Emit 'line' for each line, 'exit'
 * with the final Result, and 'error' on protocol errors.
 */
class Session extends EventEmitter {
  constructor(proc, pty) {
    super();
    this.proc = proc;
    this.pty = pty;
    this.pid = null;
    this.result = null;
    this._closed = false;

    const rl = readline.createInterface({ input: proc.stdout });
    rl.on('line', (raw) => {
      if (!raw) return;
      let msg;
      try {
        msg = JSON.parse(raw);
      } catch (e) {
        return;
      }
      this._handle(msg);
    });

    proc.stderr.on('data', (d) => {
      // rein doesn't normally write to stderr; if it does, it's
      // a fatal error.
      const text = d.toString().trim();
      if (text) this.emit('error', new Error(text));
    });

    proc.on('close', () => {
      this._closed = true;
    });
  }

  _handle(msg) {
    const t = msg.type;
    if (t === 'line') {
      this.emit('line', { stream: msg.stream, text: msg.text });
    } else if (t === 'started') {
      this.pid = msg.pid;
      this.emit('started', msg.pid);
    } else if (t === 'exit') {
      this.result = {
        exitCode: msg.exit_code || 0,
        durationMs: msg.duration_ms || 0,
        stdout: msg.stdout || '',
        stderr: msg.stderr || '',
        err: msg.err || null,
      };
      this.emit('exit', this.result);
    } else if (t === 'error') {
      this.result = {
        exitCode: -1,
        durationMs: 0,
        err: msg.err,
      };
      this.emit('error', new Error(msg.err || 'rein error'));
    }
  }

  stop() {
    if (this._closed) return Promise.resolve(this.result);
    return new Promise((resolve) => {
      this.once('exit', () => resolve(this.result));
      try {
        this.proc.stdin.write(JSON.stringify({ type: 'stop' }) + '\n');
      } catch (e) {
        // Process is already dead; the close handler will fire.
      }
    });
  }

  write(text) {
    if (!this.pty) {
      throw new Error('write() requires exec() (PTY); use start() for non-PTY sessions');
    }
    this.proc.stdin.write(JSON.stringify({ type: 'input', text }) + '\n');
  }

  resize(rows, cols) {
    if (!this.pty) {
      throw new Error('resize() requires exec() (PTY)');
    }
    this.proc.stdin.write(JSON.stringify({ type: 'resize', rows, cols }) + '\n');
  }
}

/**
 * Start a long-running command and return a Session.
 */
function start(command, options = {}) {
  const rein = findRein();
  const args = [rein, 'start'];
  if (options.timeout != null) args.push('--timeout', `${Math.round(options.timeout * 1000)}ms`);
  if (options.idleTimeout != null) args.push('--idle-timeout', `${Math.round(options.idleTimeout * 1000)}ms`);
  if (options.gracefulTimeout != null) args.push('--graceful-timeout', `${Math.round(options.gracefulTimeout * 1000)}ms`);
  if (options.dir) args.push('--dir', options.dir);
  if (options.env) {
    for (const [k, v] of Object.entries(options.env)) {
      args.push('--env', `${k}=${v}`);
    }
  }
  if (options.pty) args.push('--pty');
  if (options.lineBuffer != null) args.push('--line-buffer', String(options.lineBuffer));
  args.push(command);

  const proc = spawn(args[0], args.slice(1), { stdio: ['pipe', 'pipe', 'pipe'] });
  return new Session(proc, !!options.pty);
}

/**
 * Start a command with a PTY, return a Session for interactive use.
 */
function exec(command, options = {}) {
  const rein = findRein();
  const args = [rein, 'exec'];
  if (options.timeout != null) args.push('--timeout', `${Math.round(options.timeout * 1000)}ms`);
  if (options.idleTimeout != null) args.push('--idle-timeout', `${Math.round(options.idleTimeout * 1000)}ms`);
  if (options.gracefulTimeout != null) args.push('--graceful-timeout', `${Math.round(options.gracefulTimeout * 1000)}ms`);
  if (options.dir) args.push('--dir', options.dir);
  if (options.env) {
    for (const [k, v] of Object.entries(options.env)) {
      args.push('--env', `${k}=${v}`);
    }
  }
  args.push('--initial-size', options.initialSize || '24x80');
  args.push(command);

  const proc = spawn(args[0], args.slice(1), { stdio: ['pipe', 'pipe', 'pipe'] });
  return new Session(proc, true);
}

module.exports = { run, start, exec, Session, findRein };
