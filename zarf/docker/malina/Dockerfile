# syntax=docker/dockerfile:1.7

FROM node:22-bookworm-slim AS bui
WORKDIR /src/cmd/server/api/frontends/bui
COPY cmd/server/api/frontends/bui/package.json cmd/server/api/frontends/bui/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY cmd/server/api/frontends/bui/ ./
RUN npm run build

FROM debian:trixie-slim AS native
ARG SD_ARCHIVE_URL=https://github.com/leejet/stable-diffusion.cpp/releases/download/master-669-2d40a8b/sd-master-2d40a8b-bin-Linux-Ubuntu-24.04-x86_64.zip
ARG SD_ARCHIVE_SHA256=e9d8942b85f32282a749fd97c65c15674c37aee9ea08ebc738fc6e3f524143ca
RUN apt-get update && \
    apt-get install --no-install-recommends -y ca-certificates curl unzip && \
    rm -rf /var/lib/apt/lists/* && \
    curl --fail --location --retry 3 --output /tmp/stable-diffusion.zip "${SD_ARCHIVE_URL}" && \
    echo "${SD_ARCHIVE_SHA256}  /tmp/stable-diffusion.zip" | sha256sum --check --strict && \
    mkdir -p /out/lib && \
    unzip -q /tmp/stable-diffusion.zip libstable-diffusion.so -d /out/lib && \
    rm -f /tmp/stable-diffusion.zip

FROM golang:1.26.5-bookworm AS build
ARG TARGETARCH
RUN case "${TARGETARCH}" in \
      amd64) ;; \
      *) echo "unsupported TARGETARCH=${TARGETARCH}: this image supports linux/amd64 CPU only" >&2; exit 1 ;; \
    esac
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
COPY --from=bui /src/cmd/server/api/services/malina/static/site/ ./cmd/server/api/services/malina/static/site/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH="${TARGETARCH}" \
    go build -trimpath -ldflags='-s -w' -o /out/malina ./cmd/malina

FROM debian:trixie-slim AS runtime
RUN apt-get update && \
    apt-get install --no-install-recommends -y ca-certificates curl libgomp1 libstdc++6 && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd --system malina && \
    useradd --system --gid malina --create-home malina
COPY --from=build /out/malina /usr/local/bin/malina
COPY --from=native /out/lib/ /opt/malina/lib/
ENV MALINA_LIB=/opt/malina/lib \
    LD_LIBRARY_PATH=/opt/malina/lib
USER malina
EXPOSE 8080
VOLUME ["/models"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl --fail --silent --show-error http://127.0.0.1:8080/healthz >/dev/null || exit 1
ENTRYPOINT ["malina"]
CMD ["server", "start", "--host", "0.0.0.0:8080"]
