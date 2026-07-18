FROM golang:1.25.0-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -trimpath -o /out/pkcs11-demo ./cmd/pkcs11-demo

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates softhsm2 \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/pkcs11-demo /usr/local/bin/pkcs11-demo
COPY docker/softhsm/entrypoint.sh /usr/local/bin/softhsm-entrypoint
RUN chmod 0755 /usr/local/bin/softhsm-entrypoint

ENV SOFTHSM2_CONF=/etc/softhsm/softhsm2.conf \
    SOFTHSM2_TOKEN_DIR=/var/lib/softhsm/tokens \
    SIGNER_DB=/var/lib/signer/fence.db

ENTRYPOINT ["/usr/local/bin/softhsm-entrypoint"]
