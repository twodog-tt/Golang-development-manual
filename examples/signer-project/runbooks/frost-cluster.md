# Cross-process FROST cluster runbook

This work package runs Taurus FROST Taproot DKG and BIP-340 signing across
independent participant processes. The intended three-party topology is:

```text
operator/control plane
        |
        +---------- authenticated DKG/sign commands ----------+
        |                         |                            |
   participant alice        participant bob             participant carol
   alice share only         bob share only               carol share only
        \                         |                            /
         \---- mTLS + Taurus protocol.Message bytes ---------/
                         coordinator
                    validation + routing only
```

The coordinator never constructs or loads a `frost.TaprootConfig`. It retains
public session metadata, message hashes, and opaque Taurus message bytes in
memory. Every `protocol.Message` crosses the HTTP boundary using the upstream
`MarshalBinary` representation. The coordinator decodes only the Taurus
message header needed to validate session, protocol, sender, recipient, SSID,
round, and replay identity; it does not decode the round payload into a share.

## Security properties and honest boundaries

- Each participant process is configured with one party ID and one share file.
  Its DKG result is checked to belong to that party before persistence.
- DKG uses `SaveNew`: an existing share is not overwritten. Installation uses
  a same-directory temporary file, `fsync`, mode `0600`, and an atomic
  no-replace hard-link operation. The parent directory must deny group/other
  access.
- Request bodies, response bodies, and marshaled Taurus messages are bounded.
  Coordinator inboxes and total unique messages per session are bounded as
  well.
- Session metadata is registered before a ceremony. Participants fetch and
  compare the registered key ID, sorted party set, threshold, operation kind,
  digest, expiry, and 32-byte random session ID before constructing a handler.
- The router enforces the Taurus Taproot DKG/sign protocol ID, authenticated
  sender, legal recipient, one bound SSID, legal FROST round, and canonical
  binary encoding.
- Exact message replays are SHA-256 deduplicated before enqueue. A retry after
  an ambiguous HTTP failure is therefore idempotent within the active
  coordinator process.
- A participant accepts a routed message only if the local Taurus handler also
  accepts its session/protocol/party headers. It maintains a second
  per-ceremony replay set.
- A DKG/sign session ID is synchronously committed to a private bbolt ledger
  before a Taurus handler is constructed. Failed ceremonies remain burned
  across participant restarts; a repeated control request cannot resume or
  reuse a nonce-bearing transcript.
- The pinned Taurus release has a known implementation defect where
  `Message.UnmarshalBinary` swallows CBOR errors. The router and participants
  bypass it with strict fxamacker CBOR decoding, reject duplicate map keys,
  tags, indefinite-length values and unknown fields, then require the exact
  upstream `MarshalBinary` form.
- Only one DKG or signing ceremony runs concurrently in one participant
  process. A missing party therefore ends in a bounded context deadline rather
  than an unbounded wait.
- Completed signatures are locally verified as BIP-340 before they leave a
  participant.

These controls do **not** make the sample a fully audited custody service:

- The coordinator queue and replay set are in memory. A coordinator restart
  aborts all active ceremonies. Never resume one; register a fresh random
  session ID and restart the entire DKG/signing attempt.
- The coordinator's message replay set is not durable across restart, but each
  participant's bbolt ledger is durable. After any coordinator restart, abort
  the ceremony and allocate a fresh random session ID; attempts to reuse the
  old ID are rejected by every participant that had reserved it.
- The ledger is a single-participant local database, not distributed
  consensus. Restoring an old ledger snapshot can forget burned IDs. Recovery
  must also rotate to a new globally unique session namespace/control-plane
  generation and must never resume an old signing ceremony.
- The coordinator terminates transport security and can observe transcript
  bytes and routing metadata. It does not receive a resulting private share,
  but this is not an oblivious relay.
- This package has not added application-level signatures around Taurus
  messages. It relies on mutually authenticated transport, session binding,
  and the Taurus protocol's own verification.
- Go cannot guarantee that copies of private scalar bytes are zeroized from
  memory. Process isolation, locked memory, hardened hosts, and crash-dump
  policy remain deployment responsibilities.
- The pinned Taurus pre-release dependency must receive an independent
  cryptographic/code audit before custody use.

## Share protection modes

The share envelope records its protection mode explicitly:

| Mode | Behavior | Actual boundary |
| --- | --- | --- |
| `plaintext` | JSON share payload is only base64-encoded by the outer envelope | File mode and host access control only |
| `aes-256-gcm-static` | Payload is authenticated/encrypted with a 32-byte static key | Helps against disclosure of the share file alone |

The static AES key is loaded into the same participant process. If it is stored
on the same host, an attacker that compromises the process or can read both
files can recover the share. This is **not** HSM/KMS isolation. A production
deployment should use envelope encryption with a non-exportable KMS/HSM key,
attested workload identity, rotation/version metadata, and an audited recovery
procedure.

The share file and session ledger are one recovery unit. Never restore the
share while discarding or rolling back its ledger and then accept historical
session IDs. bbolt also relies on local file-lock and fsync semantics; do not
place the ledger on an eventually consistent network filesystem.

The key file accepts 32 raw bytes or 64 lowercase hexadecimal characters and
must deny group/other access:

```bash
install -d -m 0700 /var/lib/frost/alice
openssl rand -hex 32 > /var/lib/frost/alice/share-key
chmod 0600 /var/lib/frost/alice/share-key
```

## Production transport: mTLS first

Use a private CA or workload-identity issuer. Coordinator and participant
control servers require and verify client certificates with TLS 1.3. A
certificate URI SAN is used as its certificate identity when present; Common
Name is only the fallback.

Map URI SANs to short Taurus party IDs, because Taurus party IDs are limited to
32 bytes and are polynomial interpolation identifiers:

```json
{
  "spiffe://custody.example/mpc/alice": "alice",
  "spiffe://custody.example/mpc/bob": "bob",
  "spiffe://custody.example/mpc/carol": "carol",
  "spiffe://custody.example/operator": "operator"
}
```

Save the mapping as a private file and start the router:

```bash
chmod 0600 /etc/frost/mtls-identities.json

go run ./cmd/mpc-coordinator \
  -listen 10.20.0.10:9443 \
  -tls-cert /etc/frost/coordinator-server.pem \
  -tls-key /etc/frost/coordinator-server-key.pem \
  -tls-client-ca /etc/frost/workload-ca.pem \
  -mtls-identity-map-file /etc/frost/mtls-identities.json \
  -admin-identities operator
```

Use separate server and coordinator-client certificates when certificate EKUs
are separated. Example participant:

```bash
go run ./cmd/mpc-participant \
  -id alice \
  -listen 10.20.1.11:9543 \
  -coordinator https://coordinator.mpc.internal:9443 \
  -share-file /var/lib/frost/alice/taproot-share.json \
  -session-ledger /var/lib/frost/alice/sessions.db \
  -share-key-file /var/lib/frost/alice/share-key \
  -tls-cert /etc/frost/alice-control-server.pem \
  -tls-key /etc/frost/alice-control-server-key.pem \
  -tls-client-ca /etc/frost/operator-ca.pem \
  -mtls-control-identity-map-file /etc/frost/operator-identities.json \
  -coordinator-client-cert /etc/frost/alice-coordinator-client.pem \
  -coordinator-client-key /etc/frost/alice-coordinator-client-key.pem \
  -coordinator-ca /etc/frost/coordinator-ca.pem \
  -coordinator-server-name coordinator.mpc.internal \
  -admin-identities operator
```

Apply network policy so participants can reach only the coordinator relay
port, operators can reach only participant control ports and the coordinator
session-registration endpoint, and participant hosts cannot read one another's
storage.

## Controlled local test mode

Loopback token mode exists for tests and laptop exercises only. It is rejected
when the TCP peer is not loopback. It is not a production alternative to mTLS:
the bearer token has no hardware-backed identity, attestation, revocation
channel, or protection from a compromised local host.

Create files without putting tokens on process command lines:

```bash
workdir="$(mktemp -d)"
chmod 0700 "$workdir"

cat > "$workdir/coordinator-tokens.json" <<'JSON'
{
  "operator": "operator-token-000000000000000000000000",
  "alice": "alice-token-00000000000000000000000000",
  "bob": "bob-token-0000000000000000000000000000",
  "carol": "carol-token-00000000000000000000000000"
}
JSON

cat > "$workdir/control-tokens.json" <<'JSON'
{"operator": "operator-token-000000000000000000000000"}
JSON

printf '%s\n' 'operator-token-000000000000000000000000' > "$workdir/operator.token"
printf '%s\n' 'alice-token-00000000000000000000000000' > "$workdir/alice.token"
printf '%s\n' 'bob-token-0000000000000000000000000000' > "$workdir/bob.token"
printf '%s\n' 'carol-token-00000000000000000000000000' > "$workdir/carol.token"
chmod 0600 "$workdir"/*

for party in alice bob carol; do
  install -d -m 0700 "$workdir/$party"
done
```

Start the coordinator from `examples/signer-project`:

```bash
go run ./cmd/mpc-coordinator \
  -listen 127.0.0.1:9443 \
  -loopback-token-file "$workdir/coordinator-tokens.json"
```

Start three participant processes in separate terminals:

```bash
go run ./cmd/mpc-participant \
  -id alice \
  -listen 127.0.0.1:9541 \
  -coordinator http://127.0.0.1:9443 \
  -share-file "$workdir/alice/share.json" \
  -session-ledger "$workdir/alice/sessions.db" \
  -loopback-control-token-file "$workdir/control-tokens.json" \
  -loopback-coordinator-token-file "$workdir/alice.token"
```

Repeat for Bob on `9542` and Carol on `9543`, with their own share paths and
coordinator-token files.

### Register and run three-party DKG

Generate a fresh session ID:

```bash
dkg_session="$(openssl rand -hex 32)"
```

Register public routing metadata:

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $(cat "$workdir/operator.token")" \
  -H 'Content-Type: application/json' \
  -d "{
    \"id\":\"$dkg_session\",
    \"kind\":\"taproot-dkg\",
    \"key_id\":\"cluster-key\",
    \"parties\":[\"alice\",\"bob\",\"carol\"],
    \"threshold\":1
  }" \
  http://127.0.0.1:9443/v1/sessions
```

All participant commands must begin concurrently:

```bash
for port in 9541 9542 9543; do
  curl --fail-with-body \
    -H "Authorization: Bearer $(cat "$workdir/operator.token")" \
    -H 'Content-Type: application/json' \
    -d "{
      \"session_id\":\"$dkg_session\",
      \"key_id\":\"cluster-key\",
      \"parties\":[\"alice\",\"bob\",\"carol\"],
      \"threshold\":1
    }" \
    "http://127.0.0.1:$port/v1/dkg" &
done
wait
```

Each response contains the same x-only public key and only that process's party
ID. No API returns the private share.

### Register and run two-of-three signing

The API signs an already hashed 32-byte digest:

```bash
sign_session="$(openssl rand -hex 32)"
digest="$(printf '%s' 'frost cluster test' | openssl dgst -sha256 -binary | xxd -p -c 256)"

curl --fail-with-body \
  -H "Authorization: Bearer $(cat "$workdir/operator.token")" \
  -H 'Content-Type: application/json' \
  -d "{
    \"id\":\"$sign_session\",
    \"kind\":\"taproot-sign\",
    \"key_id\":\"cluster-key\",
    \"parties\":[\"alice\",\"bob\"],
    \"threshold\":1,
    \"digest_hex\":\"$digest\"
  }" \
  http://127.0.0.1:9443/v1/sessions
```

Trigger Alice and Bob concurrently:

```bash
for port in 9541 9542; do
  curl --fail-with-body \
    -H "Authorization: Bearer $(cat "$workdir/operator.token")" \
    -H 'Content-Type: application/json' \
    -d "{
      \"session_id\":\"$sign_session\",
      \"key_id\":\"cluster-key\",
      \"signers\":[\"alice\",\"bob\"],
      \"digest_hex\":\"$digest\"
    }" \
    "http://127.0.0.1:$port/v1/sign" &
done
wait
```

Both participants verify and return the same 64-byte BIP-340 signature.

## Failure handling

- Participant unavailable or network partition: the remaining parties return
  a protocol deadline error. Do not persist a partial DKG result and do not
  retry the same signing session.
- HTTP response lost after message acceptance: relay retry is safe within the
  active coordinator because the canonical message bytes are deduplicated.
- Wrong sender, recipient, protocol, SSID, session member, or non-canonical
  message: the coordinator rejects it before enqueue.
- Queue full: the sender receives a retryable `503`; repeated failure should
  abort the ceremony and create a new session.
- Coordinator restart: all ceremonies are aborted. A new coordinator must not
  reconstruct an old signing session from participant messages; participant
  ledgers reject the old session ID after process restart as well.
- Existing share during DKG: participant returns `409`; investigate the key ID
  and recovery state instead of deleting the old share.

## Verification

Run from `examples/signer-project`:

```bash
go test -count=1 ./backend/frostcluster ./cmd/mpc-participant ./cmd/mpc-coordinator
go test -count=1 -race ./backend/frostcluster
go vet ./backend/frostcluster ./cmd/mpc-participant ./cmd/mpc-coordinator
```

The package tests use both three independent `httptest` participant servers
and a second topology with one coordinator plus three participant OS
subprocesses. They exercise three-party DKG, Alice+Bob two-of-three BIP-340
signing, a missing Carol deadline, wrong sender/recipient/protocol/SSID, wire
and control-request replay rejection, body limits, share permissions,
plaintext/static-encryption boundaries, strict malformed-CBOR rejection,
durable session replay rejection across reopen, and tamper rejection. A
separate HTTPS integration test performs a real TLS 1.3 mutual-authentication
handshake with a temporary CA, verifies URI-SAN identity mapping, rejects a
missing client certificate at the TLS layer, and rejects a trusted but
unmapped certificate at the application layer.
