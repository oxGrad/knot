# ── Stage 1: build ────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X github.com/oxgrad/knot/cmd.Version=dev" -o /knot .

# ── Stage 2a: ubuntu ──────────────────────────────────────────────────────────
FROM ubuntu:24.04 AS ubuntu
RUN apt-get update \
    && apt-get install -y --no-install-recommends git ca-certificates bash \
    && rm -rf /var/lib/apt/lists/*
RUN groupadd -g 1000 knot && useradd -u 1000 -g 1000 -m -s /bin/bash knot
COPY --from=build /knot /usr/local/bin/knot
ENV KNOT_DIR=/home/knot/dotfiles
USER knot
WORKDIR /home/knot
CMD ["bash"]

# ── Stage 2b: fedora ──────────────────────────────────────────────────────────
FROM fedora:40 AS fedora
RUN dnf install -y git ca-certificates bash \
    && dnf clean all
RUN groupadd -g 1000 knot && useradd -u 1000 -g 1000 -m -s /bin/bash knot
COPY --from=build /knot /usr/local/bin/knot
ENV KNOT_DIR=/home/knot/dotfiles
USER knot
WORKDIR /home/knot
CMD ["bash"]

# ── Stage 2c: arch ────────────────────────────────────────────────────────────
FROM archlinux:latest AS arch
RUN pacman -Sy --noconfirm git ca-certificates bash \
    && pacman -Sc --noconfirm
RUN groupadd -g 1000 knot && useradd -u 1000 -g 1000 -m -s /bin/bash knot
COPY --from=build /knot /usr/local/bin/knot
ENV KNOT_DIR=/home/knot/dotfiles
USER knot
WORKDIR /home/knot
CMD ["bash"]
