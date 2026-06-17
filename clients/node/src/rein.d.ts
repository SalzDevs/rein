// TypeScript definitions for the rein Node.js client.

export interface Result {
  exitCode: number;
  durationMs: number;
  stdout: string;
  stderr: string;
  err: string | null;
  readonly ok: boolean;
}

export interface Line {
  stream: string;
  text: string;
}

export interface RunOptions {
  timeout?: number;
  gracefulTimeout?: number;
  dir?: string;
  env?: Record<string, string>;
  pty?: boolean;
}

export interface StartOptions extends RunOptions {
  idleTimeout?: number;
  lineBuffer?: number;
}

export interface ExecOptions extends StartOptions {
  initialSize?: string;
}

export class Session {
  pid: number | null;
  result: Result | null;
  pty: boolean;
  on(event: 'line', listener: (line: Line) => void): this;
  on(event: 'started', listener: (pid: number) => void): this;
  on(event: 'exit', listener: (result: Result) => void): this;
  on(event: 'error', listener: (err: Error) => void): this;
  stop(): Promise<Result | null>;
  write(text: string): void;
  resize(rows: number, cols: number): void;
}

export function run(command: string, options?: RunOptions): Promise<Result>;
export function start(command: string, options?: StartOptions): Session;
export function exec(command: string, options?: ExecOptions): Session;
export function findRein(): string;
