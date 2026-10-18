CREATE SCHEMA ops;
CREATE TABLE ops.wallets(address text PRIMARY KEY, merchant text NOT NULL, kind text NOT NULL CHECK(kind IN ('eoa','erc1271')));
CREATE TABLE ops.challenges(id text PRIMARY KEY, address text NOT NULL REFERENCES ops.wallets ON DELETE CASCADE, message text NOT NULL, expires timestamptz NOT NULL);
CREATE TABLE ops.sessions(digest text PRIMARY KEY, merchant text NOT NULL, expires timestamptz NOT NULL);
CREATE TABLE ops.payments(
 id text PRIMARY KEY, merchant text NOT NULL, idem text NOT NULL, amount bigint NOT NULL CHECK(amount>0),
 asset text NOT NULL, quote_expires timestamptz NOT NULL,
 state text NOT NULL DEFAULT 'created', sequence bigint NOT NULL DEFAULT -1,
 provider_state text NOT NULL DEFAULT 'pending', UNIQUE(merchant,idem)
);
CREATE TABLE ops.events(id text PRIMARY KEY, payload jsonb NOT NULL, payment text NOT NULL REFERENCES ops.payments);
CREATE TABLE ops.audit(id bigserial PRIMARY KEY, payment text NOT NULL, action text NOT NULL, at timestamptz NOT NULL DEFAULT now(), UNIQUE(payment,action));
CREATE TABLE ops.faults(name text PRIMARY KEY, remaining integer NOT NULL);
CREATE TABLE ops.merchant_inbox(event_id text PRIMARY KEY, payload jsonb NOT NULL);
-- This is a local channel fixture, not an implementation of a regulated PSP.
CREATE TABLE ops.channel(id text PRIMARY KEY, amount bigint NOT NULL, asset text NOT NULL, state text NOT NULL DEFAULT 'pending', sequence bigint NOT NULL DEFAULT 0, refunded boolean NOT NULL DEFAULT false);
INSERT INTO lab.accounts VALUES ('channel_clearing','USDC',1000000000000,1000000000000);
