.PHONY: build test-ubuntu test-fedora test-arch test-all clean \
        nix-test-ubuntu nix-test-fedora nix-test-arch nix-test-all

build:
	docker compose build

test-ubuntu:
	docker compose run --rm --build knot-ubuntu

test-fedora:
	docker compose run --rm --build knot-fedora

test-arch:
	docker compose run --rm --build knot-arch

test-all:
	docker compose run --rm --build knot-ubuntu knot version
	docker compose run --rm --build knot-fedora knot version
	docker compose run --rm --build knot-arch  knot version

clean:
	docker compose down -v

nix-test-ubuntu:
	nix run .#test-ubuntu

nix-test-fedora:
	nix run .#test-fedora

nix-test-arch:
	nix run .#test-arch

nix-test-all:
	nix run .#test-ubuntu -- /usr/local/bin/knot version
	nix run .#test-fedora -- /usr/local/bin/knot version
	nix run .#test-arch   -- /usr/local/bin/knot version
