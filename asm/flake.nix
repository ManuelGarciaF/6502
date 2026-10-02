{
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { nixpkgs, ... }:
    let
      pkgs = nixpkgs.legacyPackages.x86_64-linux;

      # Build vasm with nix
      vasm = pkgs.stdenv.mkDerivation {
        pname = "vasm";
        version = "2.0f";

        src = pkgs.fetchurl {
          url = "http://phoenix.owl.de/tags/vasm2_0f.tar.gz";
          hash = "sha256-yEst4cu4eDF5X+ZKhcXZpwAqdm46fDCwotfV6Z2Hj0k=";
        };

        buildPhase = ''
          make CPU=6502 SYNTAX=std
        '';

        installPhase = ''
          mkdir -p $out/bin
          cp ./vasm6502_std $out/bin
        '';

      };
    in
    {
      devShells.x86_64-linux.default = pkgs.mkShell {
        packages = [
          pkgs.cc65
          vasm
        ];
      };
    };
}
