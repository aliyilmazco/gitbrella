## What and why

<!-- What this changes and the problem it solves. Link the issue: Fixes #123 -->

## Checklist

- [ ] A test fails without this change and passes with it
- [ ] `gofmt -l .` prints nothing, `go vet ./...` and `go test -race ./...` pass
- [ ] `check` still writes nothing and downloads nothing (docs/design.md)
- [ ] README.md updated if a command, flag, finding or exit status changed
