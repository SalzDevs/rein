# rein Node.js client

Node.js wrapper for the [rein](https://github.com/SalzDevs/rein) CLI.

## Install

```bash
# Install the rein CLI binary first
go install github.com/SalzDevs/rein/cmd/rein@latest

# Then install this package
npm install rein-client
```

## Usage

### One-shot commands

```js
const { run } = require('rein-client');

const result = await run('echo hello');
console.log(result.stdout); // "hello\n"
console.log(result.ok);     // true
```

### Long-running commands

```js
const { start } = require('rein-client');

const session = start('npm run dev', { idleTimeout: 120 });
session.on('line', (line) => {
  console.log(`[${line.stream}] ${line.text}`);
});
session.on('exit', (result) => {
  console.log(`exit ${result.exitCode}`);
});
```

### Interactive PTY commands

```js
const { exec } = require('rein-client');

const session = exec('sudo apt update');
session.on('line', (line) => {
  if (line.text.toLowerCase().includes('password')) {
    session.write('hunter2\n');
  } else {
    console.log(line.text);
  }
});
```

## API

### `run(command, options?) => Promise<Result>`

### `start(command, options?) => Session`

### `exec(command, options?) => Session`

### `Session`

Events:
- `'line'` — `{ stream, text }` for each output line
- `'started'` — `pid` when the process is running
- `'exit'` — `Result` when the process exits
- `'error'` — `Error` on protocol errors

Methods:
- `stop()` — `Promise<Result>` — stop the process gracefully
- `write(text)` — write to PTY stdin (exec only)
- `resize(rows, cols)` — resize the PTY window (exec only)

### Options

```js
{
  timeout: 30,            // seconds
  idleTimeout: 120,       // seconds (start/exec only)
  gracefulTimeout: 5,     // seconds
  dir: '/path',            // working directory
  env: { KEY: 'value' },  // environment
  pty: true,               // allocate a PTY (start only; always true for exec)
  lineBuffer: 4096,       // (start only)
  initialSize: '24x80',   // (exec only)
}
```

## Testing

```bash
# Run tests (requires rein binary on $PATH)
cd clients/node
npm test
```

## License

MIT.
