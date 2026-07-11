# Upstream baseline

PkuHoleStudio was forked from `dfshfghj/PKUHoleTUI` at:

- Commit: `f9d6221e16b1659a453866f3980c30c0cb8067e6`
- Tag in this repository: `upstream-pkuholetui-f9d6221`
- Baseline date: 2026-07-12
- Toolchain: Go 1.26.5, CGO enabled, MSYS2 UCRT64 GCC 16.1.0

## Verification

- 30 Go test files and 378 top-level `Test*` functions were discovered.
- 12 live API tests in `internal/client/treehole_live_test.go` and
  `internal/crawler/crawler_live_test.go` skip unless
  `PKUHOLE_LIVE_TEST=1` is set.
- `go build ./cmd` passed.
- `go build -tags withserver ./cmd` passed.
- A direct `go test -count=1 ./...` was interrupted only because Windows
  Application Control blocked the temporary `internal/models` test binary.
  Compiling that package with `go test -c` and running the identical six tests
  from the repository path passed. Running the full command through a local
  test-exec wrapper that copies generated binaries out of Go's temporary
  directory also passed every package. This is an environment path-policy
  limitation, not a source failure.

No SQLite database existed in the upstream repository or local checkout, so
there was no real legacy database to preserve. Migration coverage will use a
deterministic database generated from the upstream schema as a compatibility
fixture.
