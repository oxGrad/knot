.PHONY: build test-ubuntu test-fedora test-arch test-all clean

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
