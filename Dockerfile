# ── Stage 1: build ────────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X github.com/oxgrad/knot/cmd.Version=dev" -o /knot .

# ── Stage 2a: ubuntu ──────────────────────────────────────────────────────────
FROM ubuntu:24.04 AS ubuntu
RUN apt-get update \
  && apt-get install -y --no-install-recommends git ca-certificates bash zsh neovim \
  && rm -rf /var/lib/apt/lists/*
RUN groupadd -g 1001 oxGrad && useradd -u 1001 -g 1001 -m -s /bin/zsh oxGrad
COPY --from=build /knot /usr/local/bin/knot
USER oxGrad
WORKDIR /home/oxGrad
CMD ["zsh"]

# ── Stage 2b: fedora ──────────────────────────────────────────────────────────
FROM fedora:40 AS fedora
RUN dnf install -y git ca-certificates bash zsh neovim \
  && dnf clean all
RUN groupadd -g 1001 oxGrad && useradd -u 1001 -g 1001 -m -s /bin/bash oxGrad
COPY --from=build /knot /usr/local/bin/knot
USER oxGrad
WORKDIR /home/oxGrad
CMD ["zsh"]

# ── Stage 2c: arch ────────────────────────────────────────────────────────────
FROM archlinux:latest AS arch
RUN pacman -Sy --noconfirm git ca-certificates bash zsh neovim \
  && pacman -Sc --noconfirm
RUN groupadd -g 1001 oxGrad && useradd -u 1001 -g 1001 -m -s /bin/bash oxGrad
COPY --from=build /knot /usr/local/bin/knot
USER oxGrad
WORKDIR /home/oxGrad
CMD ["zsh"]
