-- Loaded into another database; no shared transaction with the producer.
CREATE TABLE recovery_control(id boolean PRIMARY KEY CHECK(id), paused boolean NOT NULL);
INSERT INTO recovery_control VALUES(true,false);
CREATE TABLE recovery_audit (
 consumer_id text NOT NULL, event_id text NOT NULL,
 payload jsonb NOT NULL, receipt jsonb NOT NULL,
 PRIMARY KEY(consumer_id,event_id)
);
CREATE TABLE inbox (
 consumer_id text NOT NULL,
 event_id text NOT NULL,
 payload jsonb NOT NULL,
 PRIMARY KEY(consumer_id,event_id)
);
CREATE TABLE projection (
 consumer_id text NOT NULL,
 currency text NOT NULL,
 total_atomic numeric NOT NULL DEFAULT 0,
 event_count bigint NOT NULL DEFAULT 0,
 PRIMARY KEY (consumer_id,currency)
);
CREATE FUNCTION consume(p_consumer text,p_id text,p_payload jsonb)
RETURNS boolean LANGUAGE plpgsql AS $$
DECLARE inserted text; previous jsonb;
BEGIN
 -- The isolated recovery fixture pauses consumers before exposing the clone.
 PERFORM id FROM recovery_control WHERE id FOR SHARE;
 IF COALESCE((SELECT paused FROM recovery_control WHERE id),true) THEN
   RAISE EXCEPTION 'consumer paused for recovery or control missing';
 END IF;
 IF p_id IS NULL OR p_consumer IS NULL OR p_payload->>'event_id' IS DISTINCT FROM p_id
    OR p_payload->>'type' IS DISTINCT FROM 'transfer.posted'
    OR p_payload->>'version' IS DISTINCT FROM '1'
    OR coalesce(p_payload->'intent'->>'currency','')=''
    OR (p_payload->'intent'->>'amount') IS NULL
    OR (p_payload->'intent'->>'amount')::bigint<=0 THEN
   RAISE EXCEPTION 'invalid event';
 END IF;
 INSERT INTO inbox VALUES(p_consumer,p_id,p_payload)
   ON CONFLICT DO NOTHING RETURNING event_id INTO inserted;
 IF inserted IS NULL THEN
   SELECT payload INTO previous FROM inbox WHERE consumer_id=p_consumer AND event_id=p_id;
   IF previous IS DISTINCT FROM p_payload THEN RAISE EXCEPTION 'event identity conflict'; END IF;
   RETURN false;
 END IF;
 INSERT INTO projection VALUES(p_consumer,p_payload->'intent'->>'currency',(p_payload->'intent'->>'amount')::bigint,1)
   ON CONFLICT(consumer_id,currency) DO UPDATE
   SET total_atomic=projection.total_atomic+excluded.total_atomic,event_count=projection.event_count+1;
 RETURN true;
END $$;

-- Operator-only procedure in this disposable fixture (all connections are the
-- lab owner). External receipts must first be fetched/verified by the operator;
-- this SQL equality check is not authentication of a third-party receipt.
CREATE FUNCTION recover_consume(p_consumer text,p_id text,p_payload jsonb,p_receipt jsonb)
RETURNS boolean LANGUAGE plpgsql AS $$
DECLARE applied boolean;
BEGIN
 PERFORM id FROM recovery_control WHERE id FOR UPDATE;
 IF NOT COALESCE((SELECT paused FROM recovery_control WHERE id),false) THEN
   RAISE EXCEPTION 'recovery replay requires paused consumers';
 END IF;
 IF p_receipt IS DISTINCT FROM p_payload THEN RAISE EXCEPTION 'recovery receipt conflict'; END IF;
 -- Only this transaction sees the temporary unpause. Ordinary consumers wait
 -- on the row lock and then still see paused=true until reconciliation ends.
 UPDATE recovery_control SET paused=false WHERE id;
 applied := consume(p_consumer,p_id,p_payload);
 INSERT INTO recovery_audit VALUES(p_consumer,p_id,p_payload,p_receipt) ON CONFLICT DO NOTHING;
 IF EXISTS(SELECT 1 FROM recovery_audit WHERE consumer_id=p_consumer AND event_id=p_id
           AND (payload IS DISTINCT FROM p_payload OR receipt IS DISTINCT FROM p_receipt)) THEN
   RAISE EXCEPTION 'recovery audit conflict';
 END IF;
 UPDATE recovery_control SET paused=true WHERE id;
 RETURN applied;
END $$;
