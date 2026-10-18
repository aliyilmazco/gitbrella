# Contributing

Thanks for helping. Bug reports with a reproduction are as welcome as code.

## Before you start

- Read [docs/design.md](docs/design.md). Its rules are not negotiable:
  `check` writes nothing and downloads nothing, every git call goes through
  `internal/gitx`, and every change made by `fix`, `move-out` or `undo` is
  journaled and reversible.
- For anything larger than a small fix, open an issue first so the approach
  can be agreed before you write it.
- Security problems go through [SECURITY.md](SECURITY.md), not a public issue.

## Setup

You need Go 1.25 or newer and git 2.31 or newer, on macOS or Linux.

```sh
git clone https://github.com/aliyilmazco/gitbrella
cd gitbrella
go build ./cmd/gitbrella
```

## Checks

CI runs these; run them before you open a pull request:

```sh
gofmt -l .                       # must print nothing
go vet ./...
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go vet ./...
go test -race ./...
go run honnef.co/go/tools/cmd/staticcheck@v0.8.2 ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
```

CI also runs the tests with git 2.31. A test that needs a newer git must skip
on an older one instead of failing.

To show that a command changes nothing in a real synced folder, record the
folder before and compare after:

```sh
go run ./tools/snapshot save before.snap.json ~/path/to/repo
gitbrella check ~/path/to/repo
go run ./tools/snapshot same before.snap.json ~/path/to/repo
```

## Pull requests

- One change per pull request, with a test that fails without it.
- Keep the `--json` output backward compatible; a breaking change needs a new
  `schemaVersion`.
- Update README.md when a command, flag, finding or exit status changes.
- Write commit messages in the imperative ("fix: …" is not required).

By contributing you agree that your work is released under the
[MIT License](LICENSE).
