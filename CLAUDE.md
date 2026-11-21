# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`flakehash` is a single Go CLI (`cmd/flakehash`) that patches stale Nix
fixed-output-derivation hashes (`vendorHash`, `npmDepsHash`, `pnpmDeps.hash`,
...). It runs `nix build <target>`, parses the `specified`/`got` hash out of
Nix's hash-mismatch error, patches it into the given file(s), and rebuilds —
repeating per target until the build succeeds or `--max-iterations` is
exhausted. See README.md for CLI usage.

## Commands

```bash
make build       # build ./bin/flakehash for the host platform
make build-all    # cross-compile linux/darwin, amd64/arm64 into ./bin
make test         # go test -race -coverprofile=coverage.out ./...
make lint         # golangci-lint run ./...
make lint-fix     # golangci-lint run --fix ./...
make audit        # govulncheck ./...
make scan         # grype against ./bin (needs build-all first; needs nix develop)
make clean        # remove bin/ and coverage artifacts
```

Run a single test: `go test -run TestFixTarget/gives_up_after_max_iterations ./internal/nixhash/...`

`grype` and `govulncheck` aren't ambient tools — run `make scan`/`make audit`
inside `nix develop` (the flake's devShell provides `go`, `golangci-lint`,
`govulncheck`, `grype`, `git-cliff`).

There are no third-party Go dependencies (stdlib only) — no `go.sum` exists,
and `make dependencies`/`make install` are no-ops beyond `go mod download`/`tidy`.

## Architecture

- `cmd/flakehash/main.go` — flag parsing and wiring only. No business logic.
- `internal/nixhash/nixhash.go` — all the logic:
  - `Build` runs the real `nix build` subprocess; `BuildFunc` is the
    injectable seam (`FixTarget(ctx, target, files, maxIterations, build)`)
    that lets tests drive `FixTarget` without shelling out to `nix`.
  - `hashMismatchRe` requires Nix's own `hash mismatch in fixed-output
    derivation` preamble immediately before the `specified:`/`got:` pair —
    it deliberately does NOT match those tokens anywhere in the output,
    since a build's own stdout/stderr is otherwise untrusted text that a
    malicious or broken derivation could forge to make `flakehash` rewrite
    a hash to an attacker-chosen value.
  - `tailBuffer` caps captured subprocess output at 1 MiB (keeping the
    tail, since Nix's error appears near the end) so a runaway build log
    can't exhaust memory.
  - `patchFile` writes via `writeFileAtomic` (temp file in the same
    directory + `os.Rename`), chmod'd to the original file's mode before
    the rename — never an in-place truncate, so a crash mid-write can't
    corrupt the target Nix file.
- `internal/meta/meta.go` — `Name`/`Version` constants. `Version` is a
  hardcoded string kept in sync BY HAND with `flake.nix`'s
  `packages.default.version` — there is no git-describe/ldflags derivation.
  When cutting a release, bump both together.

### Nix packaging

`flake.nix` builds `packages.default` via `buildGoModule` with
`vendorHash = null` (no third-party deps to vendor) and exposes
`nixosModules.default` (from `nix/module.nix`) as a `programs.flakehash.enable`
toggle — it only adds the package to `environment.systemPackages`, since this
is a one-shot CLI, not a daemon.

### CI/CD (`.forgejo/`)

Composite actions (`native-build`, `native-scan`, `native-test`, `release`)
run everything inside `nix develop` on a self-hosted `native` runner.
`build.yaml` runs on every push; `release.yaml` is a manual
`workflow_dispatch` with a `version` input that must match
`internal/meta.Version`/`flake.nix`'s version — it builds all four
platform binaries, scans them with `grype`, and uploads `bin/` as Forgejo
release assets. There is intentionally no OCI/Docker build step.
