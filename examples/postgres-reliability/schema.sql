-- PostgreSQL 17. Amounts are integer atomic units; the API never accepts floats.
CREATE SCHEMA lab;
CREATE TABLE lab.accounts (
  id text PRIMARY KEY,
  currency text NOT NULL,
  opening_balance bigint NOT NULL CHECK (opening_balance >= 0),
  balance bigint NOT NULL CHECK (balance >= 0)
);
CREATE TABLE lab.journals (
  id text PRIMARY KEY,
  payload jsonb NOT NULL,
  reversal_of text REFERENCES lab.journals(id),
  created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE UNIQUE INDEX one_reversal ON lab.journals(reversal_of) WHERE reversal_of IS NOT NULL;
CREATE TABLE lab.entries (
  journal_id text NOT NULL REFERENCES lab.journals(id),
  account_id text NOT NULL REFERENCES lab.accounts(id),
  currency text NOT NULL,
  delta bigint NOT NULL CHECK (delta <> 0),
  PRIMARY KEY (journal_id, account_id)
);
CREATE TABLE lab.outbox (
  event_id text PRIMARY KEY REFERENCES lab.journals(id),
  payload jsonb NOT NULL,
  published boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE FUNCTION lab.immutable_fact() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'journal/entry facts are append-only'; END $$;
CREATE TRIGGER journal_immutable BEFORE UPDATE OR DELETE ON lab.journals
FOR EACH ROW EXECUTE FUNCTION lab.immutable_fact();
CREATE TRIGGER entry_immutable BEFORE UPDATE OR DELETE ON lab.entries
FOR EACH ROW EXECUTE FUNCTION lab.immutable_fact();

-- Deferred commit guard: an interrupted/incomplete journal cannot commit.
CREATE FUNCTION lab.check_balanced() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE jid text;
BEGIN
  IF TG_TABLE_NAME = 'journals' THEN jid := NEW.id; ELSE jid := NEW.journal_id; END IF;
  IF (SELECT count(*) FROM lab.entries WHERE journal_id=jid) <> 2
     OR EXISTS (SELECT 1 FROM lab.entries WHERE journal_id=jid GROUP BY currency HAVING sum(delta)<>0)
     OR EXISTS (SELECT 1 FROM lab.entries e JOIN lab.accounts a ON a.id=e.account_id
                WHERE e.journal_id=jid AND e.currency<>a.currency) THEN
    RAISE EXCEPTION 'unbalanced journal %', jid;
  END IF;
  RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER journal_balanced AFTER INSERT ON lab.journals
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION lab.check_balanced();
CREATE CONSTRAINT TRIGGER entries_balanced AFTER INSERT ON lab.entries
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION lab.check_balanced();

CREATE FUNCTION lab.post_transfer(p_id text, p_from text, p_to text, p_currency text,
                                  p_amount bigint, p_reversal text DEFAULT NULL)
RETURNS text LANGUAGE plpgsql SET search_path=lab,pg_temp AS $$
DECLARE intent jsonb; inserted text; previous jsonb;
BEGIN
  IF p_id IS NULL OR p_id='' OR p_amount IS NULL OR p_amount<=0 OR p_from IS NULL
     OR p_to IS NULL OR p_from=p_to OR p_currency IS NULL THEN
    RAISE EXCEPTION 'invalid transfer';
  END IF;
  intent := jsonb_build_object('from',p_from,'to',p_to,'currency',p_currency,
                               'amount',p_amount,'reversal_of',p_reversal);
  INSERT INTO journals(id,payload,reversal_of) VALUES (p_id,intent,p_reversal)
    ON CONFLICT (id) DO NOTHING RETURNING id INTO inserted;
  IF inserted IS NULL THEN
    SELECT payload INTO previous FROM journals WHERE id=p_id;
    IF previous IS DISTINCT FROM intent THEN RAISE EXCEPTION 'idempotency conflict'; END IF;
    RETURN p_id;
  END IF;
  -- Stable lock order for opposing transfers; locks live until transaction end.
  PERFORM id FROM accounts WHERE id IN (p_from,p_to) ORDER BY id FOR UPDATE;
  IF (SELECT count(*) FROM accounts WHERE id IN (p_from,p_to) AND currency=p_currency)<>2 THEN
    RAISE EXCEPTION 'account/currency mismatch';
  END IF;
  UPDATE accounts SET balance=balance-p_amount
    WHERE id=p_from AND currency=p_currency AND balance>=p_amount;
  IF NOT FOUND THEN RAISE EXCEPTION 'insufficient funds'; END IF;
  UPDATE accounts SET balance=balance+p_amount WHERE id=p_to;
  INSERT INTO entries VALUES (p_id,p_from,p_currency,-p_amount),(p_id,p_to,p_currency,p_amount);
  INSERT INTO outbox(event_id,payload) VALUES (p_id,jsonb_build_object('event_id',p_id,'type','transfer.posted','version',1,'intent',intent));
  RETURN p_id;
END $$;

CREATE FUNCTION lab.transfer(p_id text,p_from text,p_to text,p_currency text,p_amount bigint)
RETURNS text LANGUAGE sql SECURITY DEFINER SET search_path=lab,pg_temp AS $$
 SELECT lab.post_transfer(p_id,p_from,p_to,p_currency,p_amount,NULL)
$$;

CREATE FUNCTION lab.reverse_transfer(p_id text,p_original text)
RETURNS text LANGUAGE plpgsql SECURITY DEFINER SET search_path=lab,pg_temp AS $$
DECLARE original journals%ROWTYPE;
BEGIN
 SELECT * INTO original FROM journals WHERE id=p_original FOR UPDATE;
 IF NOT FOUND OR original.reversal_of IS NOT NULL THEN RAISE EXCEPTION 'invalid reversal target'; END IF;
 RETURN lab.post_transfer(p_id,original.payload->>'to',original.payload->>'from',
   original.payload->>'currency',(original.payload->>'amount')::bigint,p_original);
END $$;

CREATE VIEW lab.balance_drift AS
SELECT a.id,a.balance,a.opening_balance+coalesce(sum(e.delta),0) AS rebuilt
FROM lab.accounts a LEFT JOIN lab.entries e ON e.account_id=a.id
GROUP BY a.id HAVING a.balance<>a.opening_balance+coalesce(sum(e.delta),0);

REVOKE ALL ON ALL FUNCTIONS IN SCHEMA lab FROM PUBLIC;
CREATE ROLE reliability_app NOLOGIN;
GRANT USAGE ON SCHEMA lab TO reliability_app;
GRANT SELECT ON ALL TABLES IN SCHEMA lab TO reliability_app;
GRANT EXECUTE ON FUNCTION lab.transfer(text,text,text,text,bigint),lab.reverse_transfer(text,text) TO reliability_app;
-- Explicit fixture opening snapshot. A production opening import needs signed reconciliation evidence.
INSERT INTO lab.accounts VALUES
 ('alice_available','USDC',1000,1000),('alice_held','USDC',0,0),('bob','USDC',0,0),
 ('race_source','USDC',100,100),('race_sink','USDC',0,0),('btc','BTC',0,0);
