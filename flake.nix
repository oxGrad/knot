{
  description = "knot — a dotfile manager";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
    in {
      devShells = forAllSystems (system:
        let pkgs = nixpkgs.legacyPackages.${system};
        in {
          default = pkgs.mkShell {
            buildInputs = with pkgs; [ go golangci-lint goreleaser git ];
            shellHook = ''
              export GOPATH="${toString ./.}/.nix-go"
              export GOMODCACHE="${toString ./.}/.nix-go/pkg/mod"
              echo "knot dev shell — $(go version)"
            '';
          };
        });
    };
}
