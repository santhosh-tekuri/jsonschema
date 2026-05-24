{
  pkgs,
  go
}:
pkgs.buildGoApplication {
  inherit go;

  pname = "jv";
  version = "0.7.0";
  src = ./cmd/jv;
  modules = ./cmd/jv/govendor.toml;

  localReplaces = {
    "github.com/santhosh-tekuri/jsonschema/v6" = ./.;
  };
}