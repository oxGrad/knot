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

      packages = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          # Cross-compile to linux/amd64 regardless of host (needed for containers)
          crossPkgs = pkgs.pkgsCross.gnu64;

          knotBin = crossPkgs.buildGoModule {
            pname = "knot";
            version = "0.1.0";
            src = pkgs.lib.cleanSource self;
            vendorHash = "sha256-pn/zMjHY934JHxTUHbYDJ5M+l2z8b6F61Z3RBTZto9g=";
            env.CGO_ENABLED = "0";
            doCheck = false;
            # go.mod requires 1.26.2; nixpkgs-unstable ships 1.26.1 — patch the minor req
            prePatch = ''
              sed -i "s/^go 1\.26\.2/go 1.26.1/" go.mod
            '';
            meta = { mainProgram = "knot"; };
          };
        in {
          default = knotBin;
        });
    };
}
