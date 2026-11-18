{
  pkgs,
  go,
}: let
  version = "0.7.0";
in
pkgs.buildGoApplication {
  inherit go version;

  pname = "jv";
  src = ./cmd/jv;
  modules = ./cmd/jv/govendor.toml;
  ldflags = ["-X main.version=${version}"];

  localReplaces = {
    "github.com/santhosh-tekuri/jsonschema/v6" = ./.;
  };
}