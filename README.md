# flakehash

`flakehash` patches stale Nix fixed-output-derivation hashes (`vendorHash`,
`npmDepsHash`, `pnpmDeps.hash`, ...) after an upstream manifest changes. It
builds a target, extracts the `specified`/`got` hash from the resulting Nix
error, patches it into the file, and rebuilds — repeating per target until
the build succeeds or it gives up.

## Usage

```bash
flakehash [--file flake.nix] [--max-iterations 10] <target> [target...]
```

| Flag               | Default     | Description                     |
| ------------------ | ----------- | ------------------------------- |
| `--file`           | `flake.nix` | Nix file to patch; repeatable   |
| `--max-iterations` | `10`        | Max rebuild attempts per target |
| `--version`        | —           | Print version and exit          |

Each target is a flake attribute passed straight to `nix build` (e.g. `.#default`).

```bash
$ flakehash .#default
.#default: sha256-oldHash... -> sha256-newHash...
```

## Installation

### Nix

```bash
nix build git+https://git.myservermanager.com/varakh/flakehash
```

Or import the Nix package:

```nix
{ inputs, pkgs, ... }:
{
  environment.systemPackages = [ inputs.flakehash.packages.${pkgs.stdenv.hostPlatform.system}.default ];

  # or, for home-manager: home.packages = [ ... ];
}
```

### Go

```bash
go install git.myservermanager.com/varakh/flakehash/cmd/flakehash@latest
```

## Development

See the `Makefile` for build/test/lint/scan targets, and run `nix develop`
for a shell with the tooling they need.

## Release process

Version is a hardcoded constant, kept in sync by hand in
`internal/meta.Version` and `flake.nix`'s `version`. Bump both, commit, then
trigger the `release` workflow (Forgejo Actions, manual dispatch) with the
matching version — it builds, scans, and publishes the binaries as a
Forgejo release.
