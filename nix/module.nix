{ self, lib }:
{ config, pkgs, ... }:
let
  cfg = config.programs.flakehash;
in
{
  options.programs.flakehash = {
    enable = lib.mkEnableOption "flakehash, a CLI that patches stale Nix fixed-output-derivation hashes";
  };

  config = lib.mkIf cfg.enable {
    environment.systemPackages = [ self.packages.${pkgs.system}.default ];
  };
}
