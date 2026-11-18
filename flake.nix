{
  description = "JSON schema compilation and validation";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";

    go-overlay = {
      url = "github:purpleclay/go-overlay";
      inputs = {
        nixpkgs.follows = "nixpkgs";
        flake-utils.follows = "flake-utils";
      };
    };
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
    go-overlay,
    ...
  }:
    flake-utils.lib.eachDefaultSystem (
      system: let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [go-overlay.overlays.default];
        };
        go = pkgs.go-bin.fromGoMod ./go.work;
      in
        with pkgs; {
          packages.default = callPackage ./default.nix {inherit go;};

          devShells.default = mkShell {
            buildInputs = [
              alejandra
              (go.withTools [
                { name = "golangci-lint"; version = "1.57.2"; } 
                "gofumpt"
                "gopls"
              ])
              go-overlay.packages.${system}.govendor
            ];
          };
        }
    );
}
