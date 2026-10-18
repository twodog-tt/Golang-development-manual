-- Loaded into another database; no shared transaction with the producer.
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
