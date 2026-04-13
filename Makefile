.PHONY: build test-ubuntu test-fedora test-arch test-all clean

build:
	docker compose build

test-ubuntu:
	docker compose run --rm knot-ubuntu

test-fedora:
	docker compose run --rm knot-fedora

test-arch:
	docker compose run --rm knot-arch

test-all: build
	docker compose run --rm knot-ubuntu knot version
	docker compose run --rm knot-fedora knot version
	docker compose run --rm knot-arch  knot version

clean:
	docker compose down -v
