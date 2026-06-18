# Contributing to rein

Thanks for your interest in contributing! rein is a small project
and contributions of any size are welcome.

## How to contribute

1. **Open an issue first.** For anything beyond a typo or
   one-line fix, please [open an issue](https://github.com/SalzDevs/rein/issues/new)
   describing the problem or the feature you want to add. We can
   discuss the approach before you spend time on a PR.

2. **Fork and create a branch.** Branch names should be
   descriptive: `fix-idle-timer-race`, `add-windows-job-object-test`,
   etc.

3. **Write tests.** Every new feature or bug fix should come with
   tests. The test suite is `go test ./...` from the project root.

4. **Keep commits focused.** One feature per commit. Write
   descriptive commit messages in the imperative mood:
   `fix: prevent nil pointer in resize`, not `fixed stuff`.

5. **Run the test suite before submitting.** All tests must pass
   with `-race`. New tests for new features are required.

6. **Open a PR.** Reference the issue you opened. Describe what
   you changed and why.

## Development setup

```bash
git clone https://github.com/SalzDevs/rein
cd rein
go test ./...
```

Go 1.22 or later is required.

## Code style

- `gofmt` and `go vet` must be clean.
- Comments on exported types and functions are required (the
  README is generated from these via `pkg.go.dev`).
- Tests live next to the code they test (`foo.go` and `foo_test.go`
  in the same package).

## Reporting bugs

Use the [bug report template](https://github.com/SalzDevs/rein/issues/new?template=bug.md).
Please include:
- A minimal reproduction (the shortest command line that triggers
  the bug)
- Your OS and Go version
- The actual vs expected output

## Suggesting features

Use the [feature request template](https://github.com/SalzDevs/rein/issues/new?template=feature.md).
Please describe the use case first, then the proposed solution.
