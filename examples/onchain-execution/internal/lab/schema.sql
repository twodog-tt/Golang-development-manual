CREATE TABLE IF NOT EXISTS intents (
 id text PRIMARY KEY, payload text NOT NULL, nonce bigint UNIQUE NOT NULL,
 state text NOT NULL DEFAULT 'signed', review_required boolean NOT NULL DEFAULT false, block_hash text, block_height bigint,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS attempts (
 intent text REFERENCES intents(id), seq integer NOT NULL, hash text UNIQUE NOT NULL,
 raw bytea NOT NULL, gas_price numeric(78,0) NOT NULL,
 PRIMARY KEY(intent,seq)
);
CREATE TABLE IF NOT EXISTS raw_blocks (
 hash text PRIMARY KEY, height bigint NOT NULL, parent text NOT NULL
);
CREATE TABLE IF NOT EXISTS raw_logs (
 block_hash text REFERENCES raw_blocks(hash), tx_hash text, log_index bigint,
 height bigint NOT NULL, address text NOT NULL, kind text NOT NULL,
 payload jsonb NOT NULL, decoder text NOT NULL DEFAULT 'v1',
 PRIMARY KEY(block_hash,tx_hash,log_index)
);
CREATE TABLE IF NOT EXISTS canonical (
 height bigint PRIMARY KEY, hash text UNIQUE NOT NULL REFERENCES raw_blocks(hash)
);
CREATE TABLE IF NOT EXISTS index_meta (
 id integer PRIMARY KEY CHECK(id=1), height bigint NOT NULL DEFAULT -1,
 hash text NOT NULL DEFAULT '', epoch bigint NOT NULL DEFAULT 0
);
INSERT INTO index_meta(id) VALUES(1) ON CONFLICT DO NOTHING;
CREATE TABLE IF NOT EXISTS quality_errors (
 id bigserial PRIMARY KEY, observed_at timestamptz NOT NULL DEFAULT now(), detail text NOT NULL
);

-- Local operator-only capabilities; no HTTP mutation endpoint is exposed.
CREATE TABLE IF NOT EXISTS intent_cancellations (
 intent text PRIMARY KEY REFERENCES intents(id), hash text UNIQUE NOT NULL,
 raw bytea NOT NULL, nonce bigint NOT NULL, gas_price numeric(78,0) NOT NULL,
 plan text UNIQUE NOT NULL
);
CREATE TABLE IF NOT EXISTS retired_intents (
 id text PRIMARY KEY, plan text NOT NULL, at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS recovery_plans (
 id text PRIMARY KEY, digest text NOT NULL, document jsonb NOT NULL,
 expires timestamptz NOT NULL, applied boolean NOT NULL DEFAULT false,
 operator text, ticket text, result jsonb
);
