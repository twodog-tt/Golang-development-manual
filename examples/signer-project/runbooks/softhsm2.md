# SoftHSM2 PKCS#11 sandbox runbook

SoftHSM2 is a software token. Its key objects live in a Docker volume and are
protected by operating-system controls and a demo PIN, not by tamper-resistant
hardware. This sandbox validates PKCS#11 integration; it is not a hardware
security boundary.

All commands run from `examples/signer-project/`.

## Static validation

This command parses and normalizes the Compose model without contacting the
Docker daemon:

```bash
docker compose config --quiet
```

The Go package compiles in the default suite. Its live integration test skips
unless an initialized token and module path are supplied.

## Build and run

```bash
docker compose build softhsm-signer
docker compose run --rm softhsm-signer
```

The entrypoint creates a token on first use, then `crypto11` finds or generates
a non-extractable P-256 ECDSA key, runs the durable fence flow, and verifies the
DER-encoded signature. Named volumes preserve both token objects and bbolt state
across container restarts.

Treat `softhsm_tokens` and `signer_state` as one recovery unit for a logical key.
If the token volume is lost while the fence DB survives, completed historical
receipts remain readable and verifiable with their embedded old public key, but
a newly generated object must not continue as the old key. The fence store binds
the algorithm and encoded public key on the first completed request and rejects
a replacement identity before releasing its result. Restore the correct token or
perform an explicit, audited key migration; do not delete fence state to bypass
the mismatch.

Running the same command again returns the persisted receipt for the same
request binding. To advance ownership and create a new request:

```bash
docker compose run --rm \
  -e SIGNER_OWNER=worker-hsm-next \
  -e SIGNER_EPOCH=2 \
  -e SIGNER_REQUEST_ID=hsm-request-2 \
  -e SIGNER_MESSAGE='second payload' \
  softhsm-signer
```

An epoch-1 request is stale after that commit. Do not scale this service: bbolt
and its file lock are deliberately used as a single-active-instance store, not
as replicated signer HA.

## Run the optional host integration test

Initialize a SoftHSM2 token on the host, then set:

```bash
export SIGNER_PROJECT_PKCS11_MODULE=/absolute/path/to/libsofthsm2.so
export SIGNER_PROJECT_PKCS11_TOKEN=signer-project
export SIGNER_PROJECT_PKCS11_PIN=123456
go test -run TestSoftHSMIntegration ./backend/pkcs11
```

该测试不是只调用一次 `crypto.Signer`：它通过 bbolt fence 完成签名，关闭并重新打开
token/session 与数据库，确认相同 request 返回持久 receipt，再用同一 HSM key 接受更高 epoch，
最后验证旧 epoch 被拒绝。

Library paths vary by platform and package manager. The container entrypoint
discovers the Debian path automatically.

## Reset sandbox data

The following intentionally destroys the demo token and fence history:

```bash
docker compose down --volumes
```

Never use that reset pattern for production signing material or audit state.
