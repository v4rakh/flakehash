{
  description = "flakehash recomputes stale fixed-output-derivation hashes in Nix files";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
  };

  outputs =
    inputs@{
      flake-parts,
      nixpkgs,
      self,
      ...
    }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [
        "x86_64-linux"
        "aarch64-linux"
      ];

      perSystem =
        { pkgs, ... }:
        {
          packages.default = pkgs.buildGoModule {
            pname = "flakehash";
            # Keep in sync with internal/meta.Version.
            version = "0.1.1";
            src = ./.;
            ldflags = [
              "-s"
              "-w"
            ];
            # No third-party Go dependencies (stdlib only, no go.sum).
            vendorHash = null;
          };

          devShells.default = pkgs.mkShell {
            packages = with pkgs; [
              git-cliff
              gnumake
              go
              golangci-lint
              govulncheck
              grype
            ];
          };
        };

      flake = {
        nixosModules.default = import ./nix/module.nix {
          inherit self;
          lib = nixpkgs.lib;
        };
      };
    };
}
