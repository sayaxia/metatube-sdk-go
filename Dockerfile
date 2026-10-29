FROM golang:alpine AS builder

# Version metadata is injected by the release workflow so that the binary
# reports the fork version (e.g. v1.4.0-ltx.1) even when the build context
# does not contain the git history.
ARG VERSION=unknown
ARG COMMIT=unknown

WORKDIR /src
COPY . /src

RUN apk add --update --no-cache --no-progress make git \
    && make server BUILD_VERSION="${VERSION#v}" BUILD_COMMIT="${COMMIT}"

FROM alpine:latest
LABEL org.opencontainers.image.licenses=Apache-2.0
LABEL org.opencontainers.image.source="https://github.com/metatube-community/metatube-sdk-go"
LABEL org.opencontainers.image.title="metatube-server (LTX fork)"

ARG VERSION=unknown
LABEL org.opencontainers.image.version="${VERSION}"

COPY --from=builder /src/build/metatube-server .

RUN apk add --update --no-cache --no-progress ca-certificates tzdata

ENV GIN_MODE=release
ENV PORT=8080
ENV TOKEN=""
ENV DSN=""
ENV REQUEST_TIMEOUT=""
ENV DB_MAX_IDLE_CONNS=0
ENV DB_MAX_OPEN_CONNS=0
ENV DB_PREPARED_STMT=0
ENV DB_AUTO_MIGRATE=0

EXPOSE 8080

ENTRYPOINT ["/metatube-server"]
