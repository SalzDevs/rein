# rein Python client

Python wrapper for the [rein](https://github.com/SalzDevs/rein) CLI.

## Install

```bash
# Install the rein CLI binary first
go install github.com/SalzDevs/rein/cmd/rein@latest

# Then install this package
pip install -e .
```

## Usage

### One-shot commands

```python
from rein import Rein

rein = Rein()
result = rein.run("echo hello")
print(result.stdout)  # "hello\n"
print(result.ok)      # True
```

### Long-running commands

```python
from rein import Rein

rein = Rein()
with rein.start("npm run dev", idle_timeout=120) as session:
    for line in session.lines():
        print(f"[{line.stream}] {line.text}")
```

### Interactive PTY commands

```python
from rein import Rein

rein = Rein()
with rein.exec("sudo apt update") as session:
    for line in session.lines():
        if "password" in line.text.lower():
            session.write("hunter2\n")
        else:
            print(line.text)
```

## API

### `Rein`

```python
class Rein:
    def __init__(self, rein_path: Optional[str] = None): ...
    def run(self, command, *, timeout=None, working_dir=None, env=None, pty=False) -> Result: ...
    def start(self, command, *, timeout=None, idle_timeout=None, working_dir=None, env=None, pty=False) -> Session: ...
    def exec(self, command, *, timeout=None, idle_timeout=None, working_dir=None, env=None, initial_size="24x80") -> Session: ...
```

### `Result`

```python
@dataclass
class Result:
    exit_code: int
    duration_ms: int
    stdout: str = ""
    stderr: str = ""
    err: Optional[str] = None
```

### `Session`

```python
class Session:
    def lines(self) -> Iterator[Line]: ...
    def pid(self) -> Optional[int]: ...
    def wait(self) -> Result: ...
    def stop(self) -> Result: ...
    def write(self, text: str) -> None: ...  # exec only
    def resize(self, rows: int, cols: int) -> None: ...  # exec only
```

## Testing

```bash
# Run tests (requires rein binary on $PATH)
cd clients/python
python -m pytest tests/ -v
# or
python -m unittest tests.test_client
```

## License

MIT.
