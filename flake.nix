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

          pullBase = { imageName, imageDigest, sha256, tag }:
            pkgs.dockerTools.pullImage {
              inherit imageName imageDigest sha256;
              finalImageName = imageName;
              finalImageTag = tag;
            };

          ubuntuBase = pullBase {
            imageName   = "ubuntu";
            imageDigest = "sha256:c4a8d5503dfb2a3eb8ab5f807da5bc69a85730fb49b5cfca2330194ebcc41c7b";
            sha256      = "sha256-4fNRgZzYvIKgzdJDOK5IH5fBkmzQQNMIDAEVoQj56Uk=";
            tag         = "24.04";
          };

          fedoraBase = pullBase {
            imageName   = "fedora";
            imageDigest = "sha256:3c86d25fef9d2001712bc3d9b091fc40cf04be4767e48f1aa3b785bf58d300ed";
            sha256      = "sha256-d8fu35TUL0V9uASZrLGXk1c2so88IviEdtWNUbU0yOw=";
            tag         = "40";
          };

          archBase = pullBase {
            imageName   = "archlinux";
            imageDigest = "sha256:5ba8bb318666baef4d33afefc0e65db80f38b23503cb8e7b150d315cc2d4d5da";
            sha256      = "sha256-BTyUgdu47go+6EQxGRim0KlknaqbguxPyhTLVL7DDbQ=";
            tag         = "latest";
          };

          mkImage = { name, base }:
            pkgs.dockerTools.buildLayeredImage {
              inherit name;
              tag = "latest";
              fromImage = base;
              config = {
                WorkingDir = "/root";
                Cmd = [ "/bin/bash" ];
              };
              extraCommands = ''
                mkdir -p usr/local/bin
                cp ${knotBin}/bin/knot usr/local/bin/knot
                chmod 755 usr/local/bin/knot
              '';
            };
        in {
          default = knotBin;
          images = {
            ubuntu = mkImage { name = "knot-ubuntu"; base = ubuntuBase; };
            fedora = mkImage { name = "knot-fedora"; base = fedoraBase; };
            arch   = mkImage { name = "knot-arch";   base = archBase;   };
          };
        });
    };
}
