#!/usr/bin/env python3
"""Disposable PostgreSQL/Redis fault lab. Python standard library + Docker only."""
import argparse
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import time
import uuid

from pg_fault import CommitOutcomeUnknown, commit_without_ack

HERE = Path(__file__).resolve().parent
SOURCE_FILES = ("schema.sql", "consumer.sql", "debit.sql", "pg_fault.py", "verify.py", "test_verify.py")


def write_report(path, report):
    """Publish a complete JSON file; never leave a partial success artifact."""
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(mode="w", dir=path.parent, prefix=path.name + ".", delete=False) as out:
            temporary = Path(out.name)
            json.dump(report, out, ensure_ascii=False, indent=2)
            out.write("\n")
            out.flush()
            os.fsync(out.fileno())
        os.replace(temporary, path)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def command(args, data=None, timeout=60, check=True):
    result = subprocess.run(args, input=data, text=True, capture_output=True, timeout=timeout)
    if check and result.returncode:
        raise RuntimeError(f"command failed: {' '.join(args)}\n{result.stdout}\n{result.stderr}")
    return result


def literal(value):
    return "'" + str(value).replace("'", "''") + "'"


def require(condition, message):
    if not condition:
        raise AssertionError(message)


class Lab:
    def __init__(self):
        self.token = uuid.uuid4().hex[:12]
        self.pg = "knowledge-pg-" + self.token
        self.redis = "knowledge-stream-" + self.token
        self.resources = []
        self.evidence = []

    def start(self, name, image, args):
        command(["docker", "image", "inspect", image])
        # Record ownership before run, so interrupted initialization is cleaned up.
        self.resources.append(name)
        opts = ["docker", "run", "-d", "--name", name, "--label", "knowledge-reliability-lab=" + self.token]
        if name == self.pg:
            opts += ["-e", "POSTGRES_HOST_AUTH_METHOD=trust"]
        command(opts + [image] + args)

    def sql(self, query, db="ledger", port=5432, check=True):
        r = command(["docker", "exec", "-i", self.pg, "psql", "-X", "-v", "ON_ERROR_STOP=1", "-At"]
                    + (["-h", "/tmp"] if port != 5432 else []) + [
                     "-U", "postgres", "-p", str(port), "-d", db], query, check=check)
        return r.stdout.strip() if check else r

    def shell(self, script, check=True):
        return command(["docker", "exec", "-i", "-u", "postgres", self.pg, "sh", "-eu"], script, check=check)

    def wait(self, probe, label, seconds=25):
        end = time.monotonic() + seconds
        while time.monotonic() < end:
            if probe():
                return
            time.sleep(.1)
        raise RuntimeError("timeout waiting for " + label)

    def record(self, name, **details):
        row = dict(scenario=name, passed=True, **details)
        self.evidence.append(row)
        print(json.dumps(row, ensure_ascii=False), flush=True)

    def setup(self, pg_image):
        self.start(self.pg, pg_image, ["-c", "wal_level=replica", "-c", "archive_mode=on", "-c",
                   "archive_command=if test -f /lab/archive/%f; then cmp -s %p /lab/archive/%f; else cp %p /lab/archive/%f; fi"])
        self.wait(lambda: command(["docker", "exec", self.pg, "pg_isready", "-U", "postgres"], check=False).returncode == 0,
                  "PostgreSQL")
        # pg_isready can see the entrypoint's temporary server; wait for TCP too.
        self.wait(lambda: command(["docker", "exec", self.pg, "pg_isready", "-h", "127.0.0.1", "-U", "postgres"], check=False).returncode == 0,
                  "final PostgreSQL server")
        command(["docker", "exec", "-u", "root", self.pg, "sh", "-eu", "-c", "mkdir -p /lab/archive; chown -R postgres:postgres /lab"])
        self.sql("CREATE DATABASE ledger;", "postgres")
        self.sql("CREATE DATABASE consumer;", "postgres")
        self.sql((HERE / "schema.sql").read_text())
        self.sql((HERE / "consumer.sql").read_text(), "consumer")

    def expect_error(self, query, contains, db="ledger"):
        result = self.sql(query, db, check=False)
        require(result.returncode != 0 and contains in result.stderr,
                f"expected {contains!r}, got {result.stdout!r} {result.stderr!r}")

    def assert_balances(self):
        require(self.sql("SELECT count(*) FROM lab.balance_drift;") == "0", "balance projection drift")
        require(self.sql("SELECT count(*) FROM (SELECT journal_id,currency FROM lab.entries GROUP BY 1,2 HAVING sum(delta)<>0) s;") == "0", "unbalanced entries")

    def ledger(self):
        debit = (HERE / "debit.sql").read_text().strip().rstrip(";")
        cases = ["-10", "0", "0.4", "NULL", "101"]
        sql = """BEGIN; CREATE TEMP TABLE account_balance(
          tenant_id bigint,account_id bigint,currency text,
          available numeric(38,0) NOT NULL CHECK(available>=0),version bigint NOT NULL);
          INSERT INTO account_balance VALUES(1,1,'USDC',100,0);
          PREPARE checked_debit(numeric,bigint,bigint,text) AS """ + debit + ";"
        for value in cases:
            sql += f"EXECUTE checked_debit({value},1,1,'USDC'); SELECT 'balance='||available||':'||version FROM account_balance;"
        sql += "EXECUTE checked_debit(25,1,1,'USDC'); SELECT 'balance='||available||':'||version FROM account_balance; ROLLBACK;"
        balances = [line for line in self.sql(sql).splitlines() if line.startswith("balance=")]
        require(balances == ["balance=100:0"] * len(cases) + ["balance=75:1"], "documented debit accepted invalid atomic amount")
        self.record("debit-rejects-negative-zero-fraction-null-and-overspend", rejected=len(cases))

        q = "SELECT lab.transfer('pay-1','alice_available','bob','USDC',100);"
        with ThreadPoolExecutor(max_workers=4) as pool:
            list(pool.map(lambda _: self.sql(q), range(4)))
        require(self.sql("SELECT balance FROM lab.accounts WHERE id='bob';") == "100", "duplicate debit")
        self.expect_error("SELECT lab.transfer('pay-1','alice_available','bob','USDC',101);", "idempotency conflict")
        self.expect_error("SELECT lab.transfer('wrong-chain','alice_available','btc','USDC',1);", "account/currency mismatch")
        self.expect_error("SELECT lab.transfer('negative','alice_available','bob','USDC',-1);", "invalid transfer")
        self.record("idempotency-payload-binding", concurrent_retries=4)

        def spend(i):
            return self.sql(f"SELECT lab.transfer('race-{i}','race_source','race_sink','USDC',80);", check=False).returncode
        with ThreadPoolExecutor(max_workers=2) as pool:
            codes = list(pool.map(spend, range(2)))
        require(sum(c == 0 for c in codes) == 1, "concurrent overspend accepted")
        require(self.sql("SELECT balance FROM lab.accounts WHERE id='race_source';") == "20", "overspend balance")
        self.record("concurrent-insufficient-funds", accepted=1, rejected=1)

        self.sql("SELECT lab.transfer('freeze-1','alice_available','alice_held','USDC',200);")
        self.sql("SELECT lab.reverse_transfer('release-1','freeze-1');")
        self.sql("SELECT lab.reverse_transfer('release-1','freeze-1');")
        self.expect_error("SELECT lab.reverse_transfer('release-2','freeze-1');", "one_reversal")
        require(self.sql("SELECT balance FROM lab.accounts WHERE id='alice_held';") == "0", "release not atomic")
        self.expect_error("UPDATE lab.entries SET delta=1 WHERE journal_id='pay-1';", "append-only")
        self.expect_error("SET ROLE reliability_app; UPDATE lab.accounts SET balance=1;", "permission denied")
        self.expect_error("SET ROLE reliability_app; SELECT lab.post_transfer('hack','bob','alice_available','USDC',1,NULL);", "permission denied")
        self.expect_error("BEGIN; INSERT INTO lab.journals(id,payload) VALUES ('incomplete','{}'); COMMIT;", "unbalanced journal")
        self.sql("SET ROLE reliability_app; SELECT lab.transfer('app-allowed','alice_available','bob','USDC',1); SELECT lab.reverse_transfer('app-reversed','app-allowed');")
        require(self.sql("SELECT balance FROM lab.accounts WHERE id='bob';") == "100", "application-role transfer/reversal failed")
        self.record("freeze-release-reversal-and-commit-guards")

        # Kill a real connection after function execution and before COMMIT.
        with ThreadPoolExecutor(max_workers=1) as pool:
            future = pool.submit(self.sql, "SET application_name='fault-ledger'; BEGIN; SELECT lab.transfer('killed','alice_available','bob','USDC',50); SELECT pg_sleep(30); COMMIT;", "ledger", 5432, False)
            self.wait(lambda: self.sql("SELECT count(*) FROM pg_stat_activity WHERE application_name='fault-ledger' AND wait_event='PgSleep';") == "1", "transaction fault point")
            self.sql("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE application_name='fault-ledger';")
            require(future.result().returncode != 0, "backend survived injected termination")
        require(self.sql("SELECT count(*) FROM lab.journals WHERE id='killed';") == "0", "uncommitted journal leaked")
        require(self.sql("SELECT count(*) FROM lab.outbox WHERE event_id='killed';") == "0", "uncommitted outbox leaked")
        self.assert_balances()
        self.record("backend-termination-rolls-back-ledger-and-outbox")

        before = self.sql("SELECT balance FROM lab.accounts WHERE id='bob';")
        query = "SELECT lab.transfer('commit-unknown','alice_available','bob','USDC',13)"
        try:
            commit_without_ack(self.pg, "ledger", query)
            raise AssertionError("COMMIT acknowledgement was not dropped")
        except CommitOutcomeUnknown:
            pass
        require(self.sql("SELECT count(*) FROM lab.journals WHERE id='commit-unknown';") == "1", "server did not commit before lost ACK")
        with ThreadPoolExecutor(max_workers=4) as pool:
            list(pool.map(lambda _: self.sql(query), range(4)))
        require(self.sql("SELECT balance FROM lab.accounts WHERE id='bob';") == str(int(before) + 13), "unknown COMMIT debited twice")
        require(self.sql("SELECT count(*) FROM lab.entries WHERE journal_id='commit-unknown';") == "2", "duplicate COMMIT entries")
        require(self.sql("SELECT count(*) FROM lab.outbox WHERE event_id='commit-unknown';") == "1", "duplicate COMMIT outbox")
        self.record("commit-ack-loss-queries-and-retries-original-intent", retries=4, journals=1, entries=2, outbox=1)

        self.sql("INSERT INTO lab.accounts VALUES('reversal-source','USDC',20,20),('reversal-recipient','USDC',0,0),('reversal-spent','USDC',0,0);")
        self.sql("SELECT lab.transfer('reversal-original','reversal-source','reversal-recipient','USDC',20); SELECT lab.transfer('reversal-consumed','reversal-recipient','reversal-spent','USDC',20);")
        self.expect_error("SELECT lab.reverse_transfer('reversal-insufficient','reversal-original');", "insufficient funds")
        require(self.sql("SELECT count(*) FROM lab.journals WHERE reversal_of='reversal-original';") == "0", "failed reversal left journal")
        require(self.sql("SELECT count(*) FROM lab.entries WHERE journal_id='reversal-insufficient';") == "0", "failed reversal left entries")
        require(self.sql("SELECT count(*) FROM lab.outbox WHERE event_id='reversal-insufficient';") == "0", "failed reversal left outbox")
        require(self.sql("SELECT balance FROM lab.accounts WHERE id='reversal-spent';") == "20", "failed reversal changed funds")
        self.record("insufficient-reversal-rolls-back-all-facts")
        self.sql("UPDATE lab.accounts SET balance=balance+1 WHERE id='bob';")
        require(self.sql("SELECT count(*) FROM lab.balance_drift;") == "1", "drift not detected")
        self.sql("BEGIN; LOCK TABLE lab.accounts IN EXCLUSIVE MODE; UPDATE lab.accounts a SET balance=opening_balance+coalesce((SELECT sum(e.delta) FROM lab.entries e WHERE e.account_id=a.id),0); COMMIT;")
        self.assert_balances()
        self.record("balance-drift-detected-and-rebuilt-from-entries")

    def redis_cmd(self, *args):
        return command(["docker", "exec", self.redis, "redis-cli", "--json", *args]).stdout.strip()

    def start_redis(self, redis_image):
        if self.redis in self.resources:
            return
        self.start(self.redis, redis_image, ["redis-server", "--appendonly", "yes", "--appendfsync", "always"])
        self.wait(lambda: command(["docker", "exec", self.redis, "redis-cli", "ping"], check=False).stdout.strip() == "PONG", "Redis")

    def publish(self, stream, event_id, payload, port=5432, operator=False):
        paused = self.sql("SELECT paused FROM recovery_control WHERE id;", "consumer", port)
        require(paused == "f" or operator, "publisher paused for recovery")
        return json.loads(self.redis_cmd("XADD", stream, "*", "event_id", event_id, "payload", payload))

    def delivered(self, stream, group, worker):
        value = json.loads(self.redis_cmd("XREADGROUP", "GROUP", group, worker, "COUNT", "1", "STREAMS", stream, ">"))
        require(value and value.get(stream), "expected broker delivery")
        entry, fields = value[stream][0]
        fields = dict(zip(fields[::2], fields[1::2]))
        return entry, fields["event_id"], fields["payload"]

    def consume(self, consumer, event_id, payload, port=5432):
        return self.sql("SELECT consume(" + ",".join(map(literal, (consumer, event_id, payload))) + "::jsonb);", "consumer", port)

    def external_effect(self, event_id, payload):
        # An independent, durable idempotent sink fixture, outside PostgreSQL PITR.
        # The Lua transaction binds original content and effect atomically.
        body = json.dumps(json.loads(payload), sort_keys=True, separators=(",", ":"))
        script = """local old=redis.call('HGET',KEYS[1],ARGV[1]);
        if old then if old==ARGV[2] then return 0 else return -1 end end;
        redis.call('HSET',KEYS[1],ARGV[1],ARGV[2]);
        redis.call('HINCRBY',KEYS[2],ARGV[1],1); return 1"""
        result = json.loads(self.redis_cmd("EVAL", script, "2", "external:receipts", "external:effects", event_id, body))
        require(result in (0, 1), "external identity conflict")
        return result

    def external_receipt(self, event_id):
        value = json.loads(self.redis_cmd("HGET", "external:receipts", event_id))
        return json.loads(value) if value is not None else None

    def payout_unknown(self):
        # Small outbound-payment fixture: no automatic release on missing evidence.
        self.sql("""CREATE TABLE lab.payout_cases(id text PRIMARY KEY,state text NOT NULL);
          INSERT INTO lab.accounts VALUES('payout-available','USDC',100,100),('payout-held','USDC',0,0),('payout-clearing','USDC',0,0);
          BEGIN; INSERT INTO lab.payout_cases VALUES('payout-1','held');
          SELECT lab.transfer('payout-hold','payout-available','payout-held','USDC',25); COMMIT;""")
        intent = {"event_id": "payout-1", "amount": 25, "currency": "USDC", "outcome": "executed"}

        def resolve(action):
            receipt = self.external_receipt("payout-1")
            if receipt is None:
                self.sql("UPDATE lab.payout_cases SET state='unknown' WHERE id='payout-1';")
                require(action != "release", "unknown payout cannot release reserved funds")
                return "unknown"
            require(receipt == intent, "payout receipt does not match original intent")
            require(action != "release", "executed payout cannot release reserved funds")
            self.sql("BEGIN; SELECT lab.transfer('payout-settle','payout-held','payout-clearing','USDC',25); UPDATE lab.payout_cases SET state='settled' WHERE id='payout-1'; COMMIT;")
            return "settled"

        # Provider accepted the original request, but its response was lost.
        self.external_effect("payout-1", json.dumps(intent))
        self.redis_cmd("RENAME", "external:receipts", "external:temporarily-unavailable")
        try:
            try:
                resolve("release")
                raise AssertionError("unknown payout released funds")
            except AssertionError as exc:
                require(str(exc) == "unknown payout cannot release reserved funds", "unexpected payout failure")
            require(self.sql("SELECT balance FROM lab.accounts WHERE id='payout-held';") == "25", "unknown payout lost its hold")
        finally:
            self.redis_cmd("RENAME", "external:temporarily-unavailable", "external:receipts")
        require(self.external_effect("payout-1", json.dumps(intent)) == 0, "same payout executed twice")
        require(resolve("settle") == "settled" and resolve("settle") == "settled", "payout did not reconcile")
        require(self.sql("SELECT balance FROM lab.accounts WHERE id='payout-clearing';") == "25", "duplicate payout accounting")
        require(self.sql("SELECT balance FROM lab.accounts WHERE id='payout-available';") == "75", "payout reserve was released")
        require(self.sql("SELECT balance FROM lab.accounts WHERE id='payout-held';") == "0", "settled payout remains held")
        require(self.sql("SELECT count(*) FROM lab.journals WHERE id='payout-settle';") == "1", "duplicate payout journal")
        require(json.loads(self.redis_cmd("HGET", "external:effects", "payout-1")) == "1", "duplicate external payout")
        self.record("unknown-payout-keeps-reserve-and-reconciles-original-id", reserved_while_unknown=25, external_effects=1)

    def outbox(self, redis_image):
        self.start_redis(redis_image)
        self.redis_cmd("XGROUP", "CREATE", "transfers", "accounting", "0", "MKSTREAM")
        self.sql("SELECT lab.transfer('bus-1','alice_available','bob','USDC',10);")
        payload = self.sql("SELECT payload::text FROM lab.outbox WHERE event_id='bus-1';")

        def publish():
            return json.loads(self.redis_cmd("XADD", "transfers", "*", "event_id", "bus-1", "payload", payload))

        def consume(body=payload):
            return self.sql("SELECT consume('accounting','bus-1'," + literal(body) + "::jsonb);", "consumer")

        first = publish()  # broker accepted; relay dies before published=true
        require(self.sql("SELECT published FROM lab.outbox WHERE event_id='bus-1';") == "f", "fault point lost")
        second = publish()  # restarted relay publishes same business event with a new stream ID
        self.sql("UPDATE lab.outbox SET published=true WHERE event_id='bus-1';")
        delivered = json.loads(self.redis_cmd("XREADGROUP", "GROUP", "accounting", "worker-dead", "COUNT", "1", "STREAMS", "transfers", ">"))
        require(delivered["transfers"][0][0] == first, "wrong first delivery")
        body_fields = delivered["transfers"][0][1]
        body = dict(zip(body_fields[::2], body_fields[1::2]))["payload"]
        # Consumer transaction rolls back, message remains pending.
        self.expect_error("BEGIN; SELECT consume('accounting','bus-1'," + literal(body) + "::jsonb); SELECT 1/0; COMMIT;", "division by zero", "consumer")
        require(self.sql("SELECT count(*) FROM inbox;", "consumer") == "0", "inbox escaped rollback")
        claimed = json.loads(self.redis_cmd("XCLAIM", "transfers", "accounting", "worker-restarted", "0", first))
        require(claimed[0][0] == first, "pending delivery not reclaimed")
        require(consume(body) == "t", "first transaction failed")
        # DB committed but worker died before XACK; reclaim and deduplicate.
        self.redis_cmd("XCLAIM", "transfers", "accounting", "worker-again", "0", first)
        require(consume(body) == "f", "committed event applied twice")
        self.redis_cmd("XACK", "transfers", "accounting", first)
        next_delivery = json.loads(self.redis_cmd("XREADGROUP", "GROUP", "accounting", "worker-again", "COUNT", "1", "STREAMS", "transfers", ">"))
        require(next_delivery["transfers"][0][0] == second, "relay duplicate absent")
        require(consume() == "f", "new stream ID bypassed inbox")
        self.redis_cmd("XACK", "transfers", "accounting", second)
        require(json.loads(self.redis_cmd("XPENDING", "transfers", "accounting"))[0] == 0, "unacked messages remain")
        require(self.sql("SELECT event_count||':'||total_atomic FROM projection WHERE consumer_id='accounting' AND currency='USDC';", "consumer") == "1:10", "projection not effect-once")
        altered = json.loads(payload); altered["intent"]["amount"] = 999
        self.expect_error("SELECT consume('accounting','bus-1'," + literal(json.dumps(altered)) + "::jsonb);", "event identity conflict", "consumer")
        self.assert_balances()
        self.record("outbox-relay-crash-after-broker-ack", stream_messages=2, business_events=1)
        self.record("inbox-rollback-and-crash-before-xack", projection_count=1, pending=0)

        same = json.loads(payload); same["event_id"] = "bus-concurrent"
        same_body = json.dumps(same)
        with ThreadPoolExecutor(max_workers=8) as pool:
            values = list(pool.map(lambda _: self.consume("concurrent", same["event_id"], same_body), range(8)))
        require(values.count("t") == 1 and values.count("f") == 7, "concurrent event applied more than once")
        require(self.sql("SELECT event_count||':'||total_atomic FROM projection WHERE consumer_id='concurrent';", "consumer") == "1:10", "concurrent projection mismatch")
        self.record("concurrent-inbox-same-content", workers=8, applied=1, duplicates=7)

        conflict = {**same, "event_id": "bus-conflicting"}
        bodies = [json.dumps({**conflict, "intent": {**conflict["intent"], "amount": amount}}) for amount in (10, 11)]
        def competing(index):
            query = "SELECT consume('conflicting','bus-conflicting'," + literal(bodies[index % 2]) + "::jsonb);"
            return self.sql(query, "consumer", check=False)
        with ThreadPoolExecutor(max_workers=8) as pool:
            results = list(pool.map(competing, range(8)))
        require(sum(r.returncode == 0 and r.stdout.strip() == "t" for r in results) == 1, "conflicting consumers had multiple winners")
        require(sum(r.returncode == 0 and r.stdout.strip() == "f" for r in results) == 3, "matching retries were not deduplicated")
        require(sum(r.returncode != 0 and "event identity conflict" in r.stderr for r in results) == 4, "conflicting payload not rejected")
        require(self.sql("SELECT p.event_count=1 AND p.total_atomic=(i.payload->'intent'->>'amount')::bigint FROM projection p JOIN inbox i USING(consumer_id) WHERE p.consumer_id='conflicting';", "consumer") == "t", "conflict corrupted projection")
        self.record("concurrent-inbox-conflicting-content", winners=1, duplicates=3, conflicts=4)

        # A broker entry is lost after the producer already recorded published.
        self.redis_cmd("XGROUP", "CREATE", "lost-transfers", "lost-group", "0", "MKSTREAM")
        self.sql("SELECT lab.transfer('bus-lost','alice_available','bob','USDC',3);")
        lost_payload = self.sql("SELECT payload::text FROM lab.outbox WHERE event_id='bus-lost';")
        lost = self.publish("lost-transfers", "bus-lost", lost_payload)
        self.sql("UPDATE lab.outbox SET published=true WHERE event_id='bus-lost';")
        self.redis_cmd("XDEL", "lost-transfers", lost)
        require(json.loads(self.redis_cmd("XREADGROUP", "GROUP", "lost-group", "repair", "STREAMS", "lost-transfers", ">")) is None, "lost entry still delivered")
        require(self.sql("SELECT count(*) FROM inbox WHERE consumer_id='lost-consumer' AND event_id='bus-lost';", "consumer") == "0", "lost event unexpectedly consumed")
        # Reconcile retained producer facts against this projection's Inbox.
        self.publish("lost-transfers", "bus-lost", lost_payload)
        entry, event_id, body = self.delivered("lost-transfers", "lost-group", "repair")
        require(self.consume("lost-consumer", event_id, body) == "t", "lost event not repaired")
        self.redis_cmd("XACK", "lost-transfers", "lost-group", entry)
        self.publish("lost-transfers", "bus-lost", lost_payload)
        entry, event_id, body = self.delivered("lost-transfers", "lost-group", "repair")
        require(self.consume("lost-consumer", event_id, body) == "f", "repair retry applied event twice")
        self.redis_cmd("XACK", "lost-transfers", "lost-group", entry)
        require(self.sql("SELECT event_count||':'||total_atomic FROM projection WHERE consumer_id='lost-consumer';", "consumer") == "1:3", "repaired projection mismatch")
        require(json.loads(self.redis_cmd("XPENDING", "lost-transfers", "lost-group"))[0] == 0, "repair left pending messages")
        self.record("published-message-loss-reconciles-from-outbox-and-inbox", repaired=1, projection_count=1)
        self.payout_unknown()
        self.assert_balances()

    def replay_recovered(self, event_id, pending=None, interrupt=False):
        """Explicit operator replay into the isolated restored instance only."""
        require(self.sql("SELECT paused FROM recovery_control WHERE id;", "consumer", 5433) == "t", "recovery replay requires isolation")
        body = self.sql("SELECT o.payload::text FROM lab.outbox o JOIN lab.journals j ON j.id=o.event_id WHERE o.event_id=" + literal(event_id), port=5433)
        require(bool(body), "missing restored producer fact; manual accounting review required")
        receipt = self.external_receipt(event_id)
        require(receipt is not None, "external receipt unavailable; keep recovery paused")
        require(receipt == json.loads(body), "external receipt conflicts with restored intent")
        if pending is None:
            self.publish("restore-transfers", event_id, body, port=5433, operator=True)
            entry, received_id, received_body = self.delivered("restore-transfers", "restore-group", "operator-replay")
        else:
            claimed = json.loads(self.redis_cmd("XCLAIM", "restore-transfers", "restore-group", "operator-replay", "0", pending))
            require(len(claimed) == 1 and claimed[0][0] == pending, "recovery PEL entry is missing")
            entry, fields = claimed[0]
            fields = dict(zip(fields[::2], fields[1::2]))
            received_id, received_body = fields["event_id"], fields["payload"]
        require(received_id == event_id and json.loads(received_body) == receipt, "broker recovery payload conflict")
        require(self.external_effect(event_id, body) == 0, "recovery repeated external effect")
        args = ",".join((literal("restore-consumer"), literal(event_id), literal(body) + "::jsonb", literal(json.dumps(receipt)) + "::jsonb"))
        self.sql("SELECT recover_consume(" + args + ");", "consumer", 5433)
        if interrupt:
            raise ConnectionError("fixture: recovery consumer committed before XACK/outbox mark")
        self.redis_cmd("XACK", "restore-transfers", "restore-group", entry)
        updated = self.sql("UPDATE lab.outbox SET published=true WHERE event_id=" + literal(event_id) + " AND payload=" + literal(body) + "::jsonb RETURNING event_id;", port=5433)
        require(event_id in updated.splitlines(), "restored outbox changed during reconciliation")

    def recovery(self, redis_image):
        self.start_redis(redis_image)
        self.redis_cmd("XGROUP", "CREATE", "restore-transfers", "restore-group", "0", "MKSTREAM")
        restore_events = ("restore-acked", "restore-pending")
        self.sql("SELECT lab.transfer('restore-acked','alice_available','bob','USDC',5); SELECT lab.transfer('restore-pending','alice_available','bob','USDC',4);")
        self.sql("SELECT lab.transfer('restore-kept','alice_available','bob','USDC',7);")
        self.shell("pg_basebackup -U postgres -D /lab/base -X stream -c fast\npg_verifybackup /lab/base\n")
        # Ensure target is strictly after the base backup and in its own WAL segment.
        self.sql("SELECT pg_switch_wal();")
        self.sql("SELECT lab.transfer('after-base','alice_available','bob','USDC',3);")
        target_segment = self.sql("SELECT pg_walfile_name(pg_current_wal_insert_lsn());")
        self.sql("SELECT pg_create_restore_point('before_delete');")
        before_count = self.sql("SELECT count(*) FROM lab.journals;")
        before_balance = self.sql("SELECT balance FROM lab.accounts WHERE id='bob';")
        pending = None
        for event_id in restore_events:
            body = self.sql("SELECT payload::text FROM lab.outbox WHERE event_id=" + literal(event_id))
            self.publish("restore-transfers", event_id, body)
            entry, received_id, received_body = self.delivered("restore-transfers", "restore-group", "before-restore")
            require(received_id == event_id, "wrong pre-restore delivery")
            require(self.external_effect(event_id, received_body) == 1, "pre-restore external effect missing")
            require(self.consume("restore-consumer", event_id, received_body) == "t", "pre-restore consumer missing")
            self.sql("UPDATE lab.outbox SET published=true WHERE event_id=" + literal(event_id))
            if event_id == "restore-acked":
                self.redis_cmd("XACK", "restore-transfers", "restore-group", entry)
            else:
                pending = entry  # Effect + Inbox committed; XACK was lost.
        self.sql("SELECT lab.transfer('after-target','alice_available','bob','USDC',9);")
        self.sql("TRUNCATE lab.entries,lab.outbox,lab.journals CASCADE;")
        require(self.sql("SELECT count(*) FROM lab.journals;") == "0", "destructive fixture did not run")
        self.sql("SELECT pg_switch_wal();")
        # File existence alone can race with an in-progress archive copy.
        # Wait until PostgreSQL has observed successful archival of this segment.
        self.wait(lambda: self.sql("SELECT coalesce(last_archived_wal >= " + literal(target_segment) + ",false) FROM pg_stat_archiver;") == "t", "WAL archive completion")

        def restore(directory, port, archive):
            # No ports are published. New data directories and sockets stay in this disposable container.
            return self.shell(f"""cp -a /lab/base /lab/{directory}
find /lab/{directory}/pg_wal -maxdepth 1 -type f -delete
cat >> /lab/{directory}/postgresql.auto.conf <<'CONF'
port = {port}
listen_addresses = ''
unix_socket_directories = '/tmp'
archive_mode = off
restore_command = 'cp /lab/{archive}/%f %p'
recovery_target_name = 'before_delete'
recovery_target_action = 'promote'
recovery_target_timeline = 'current'
CONF
touch /lab/{directory}/recovery.signal
pg_ctl -D /lab/{directory} -l /lab/{directory}.log -w -t 15 start
""", check=False)

        start = time.monotonic()
        restored = restore("restored", 5433, "archive")
        require(restored.returncode == 0, "restore failed: " + self.shell("cat /lab/restored.log\n").stdout)
        self.wait(lambda: self.sql("SELECT pg_is_in_recovery();", port=5433, check=False).stdout.strip() == "f", "restore promotion")
        require(self.sql("SELECT count(*) FROM lab.journals;", port=5433) == before_count, "wrong restore journal count")
        require(self.sql("SELECT balance FROM lab.accounts WHERE id='bob';", port=5433) == before_balance, "wrong restore balance")
        require(self.sql("SELECT count(*) FROM lab.journals WHERE id='after-target';", port=5433) == "0", "replayed past target")
        require(self.sql("SELECT count(*) FROM lab.journals WHERE id='after-base';", port=5433) == "1", "base-only restore falsely passed")
        require(self.sql("SELECT count(*) FROM lab.balance_drift;", port=5433) == "0", "recovered ledger inconsistent")
        self.record("pitr-restores-before-delete", journal_count=int(before_count), bob_balance_atomic=int(before_balance), recovery_seconds=round(time.monotonic()-start,3))

        # Both producer and consumer databases rolled back, Redis did not.
        # The clone exposes Unix sockets only; pause before starting any worker.
        self.sql("UPDATE recovery_control SET paused=true WHERE id;", "consumer", 5433)
        require(self.sql("SELECT count(*) FROM inbox WHERE consumer_id='restore-consumer';", "consumer", 5433) == "0", "PITR unexpectedly retained future Inbox")
        require(self.sql("SELECT count(*) FROM lab.outbox WHERE event_id IN ('restore-acked','restore-pending') AND published;", port=5433) == "0", "PITR retained future published flags")
        require(json.loads(self.redis_cmd("XPENDING", "restore-transfers", "restore-group"))[0] == 1, "external PEL was rolled back")
        body = self.sql("SELECT payload::text FROM lab.outbox WHERE event_id='restore-acked';", port=5433)
        # A missing singleton must fail closed, not bypass IF through SQL NULL.
        for call, error in (
            ("consume('restore-consumer','restore-acked'," + literal(body) + "::jsonb)", "control missing"),
            ("recover_consume('restore-consumer','restore-acked'," + literal(body) + "::jsonb," + literal(body) + "::jsonb)", "requires paused consumers"),
        ):
            result = self.sql("BEGIN; DELETE FROM recovery_control; SELECT " + call + "; COMMIT;", "consumer", 5433, check=False)
            require(result.returncode != 0 and error in result.stderr, "missing recovery control failed open")
        require(self.sql("SELECT paused FROM recovery_control WHERE id;", "consumer", 5433) == "t", "failed control probe did not roll back")
        require(self.sql("SELECT count(*) FROM inbox WHERE consumer_id='restore-consumer';", "consumer", 5433) == "0", "missing control allowed consumption")
        result = self.sql("SELECT consume('restore-consumer','restore-acked'," + literal(body) + "::jsonb);", "consumer", 5433, check=False)
        require(result.returncode != 0 and "paused for recovery" in result.stderr, "ordinary consumer bypassed recovery gate")
        wrong_receipt = json.loads(body); wrong_receipt["intent"]["amount"] = 999
        result = self.sql("SELECT recover_consume('restore-consumer','restore-acked'," + literal(body) + "::jsonb," + literal(json.dumps(wrong_receipt)) + "::jsonb);", "consumer", 5433, check=False)
        require(result.returncode != 0 and "receipt conflict" in result.stderr, "mismatched recovery receipt accepted")
        require(self.sql("SELECT paused FROM recovery_control WHERE id;", "consumer", 5433) == "t", "failed recovery opened gate")
        require(self.sql("SELECT count(*) FROM inbox WHERE consumer_id='restore-consumer';", "consumer", 5433) == "0", "failed recovery wrote Inbox")
        try:
            self.replay_recovered("after-target")
            raise AssertionError("missing producer fact accepted")
        except AssertionError as exc:
            require("missing restored producer fact" in str(exc), "unexpected missing-fact failure")
        # Hide, rather than erase, the durable receipt to inject unavailability.
        self.redis_cmd("RENAME", "external:receipts", "external:temporarily-unavailable")
        try:
            try:
                self.replay_recovered("restore-acked")
                raise AssertionError("missing external receipt accepted")
            except AssertionError as exc:
                require("external receipt unavailable" in str(exc), "unexpected receipt failure")
        finally:
            self.redis_cmd("RENAME", "external:temporarily-unavailable", "external:receipts")
        try:
            self.publish("restore-transfers", "restore-acked", body, port=5433)
            raise AssertionError("ordinary publisher bypassed recovery gate")
        except AssertionError as exc:
            require(str(exc) == "publisher paused for recovery", "unexpected publisher failure")
        try:
            self.replay_recovered("restore-acked", interrupt=True)
            raise AssertionError("recovery interruption was not injected")
        except ConnectionError:
            pass
        # Redis PEL is independent evidence after losing the replay response.
        interrupted = []
        for row in json.loads(self.redis_cmd("XPENDING", "restore-transfers", "restore-group", "-", "+", "100")):
            _, fields = json.loads(self.redis_cmd("XRANGE", "restore-transfers", row[0], row[0]))[0]
            if dict(zip(fields[::2], fields[1::2]))["event_id"] == "restore-acked":
                interrupted.append(row[0])
        require(len(interrupted) == 1, "interrupted repair missing from PEL")
        require(self.sql("SELECT paused FROM recovery_control WHERE id;", "consumer", 5433) == "t", "interrupted replay opened gate")
        self.replay_recovered("restore-acked", interrupted[0])
        self.replay_recovered("restore-pending", pending)
        self.replay_recovered("restore-acked")  # Repeat the operator operation.
        require(self.sql("SELECT event_count||':'||total_atomic FROM projection WHERE consumer_id='restore-consumer';", "consumer", 5433) == "2:9", "recovery projection not rebuilt exactly once")
        require(self.sql("SELECT count(*) FROM recovery_audit WHERE consumer_id='restore-consumer';", "consumer", 5433) == "2", "recovery audit missing")
        require(self.sql("SELECT count(*) FROM lab.outbox WHERE event_id IN ('restore-acked','restore-pending') AND published;", port=5433) == "2", "recovery outbox not reconciled")
        require(json.loads(self.redis_cmd("XPENDING", "restore-transfers", "restore-group"))[0] == 0, "recovery left pending entries")
        for event_id in restore_events:
            require(json.loads(self.redis_cmd("HGET", "external:effects", event_id)) == "1", "PITR duplicated external effect")
        require(self.sql("SELECT count(*) FROM lab.balance_drift;", port=5433) == "0", "replay changed financial facts")
        require(self.sql("SELECT count(*) FROM lab.journals;", port=5433) == before_count, "replay duplicated financial journals")
        require(self.sql("SELECT balance FROM lab.accounts WHERE id='bob';", port=5433) == before_balance, "replay changed financial balance")
        self.sql("UPDATE recovery_control SET paused=false WHERE id;", "consumer", 5433)
        self.record("pitr-reconciles-published-inbox-pel-and-external-receipts", replayed_events=2, external_effects=2, pending=0, repeated_replay_safe=True, replay_commit_ack_loss=True, financial_journals_unchanged=True)

        self.shell(f"cp -a /lab/archive /lab/broken-archive\nrm /lab/broken-archive/{target_segment}\n")
        failed = restore("missingwal", 5434, "broken-archive")
        # pg_ctl may return as soon as consistent hot standby accepts connections;
        # inspect postmaster exit/log rather than equating readiness with reaching target.
        self.wait(lambda: "recovery ended before configured recovery target was reached" in self.shell("cat /lab/missingwal.log\n").stdout,
                  "missing WAL failure", seconds=20)
        require(self.sql("SELECT 1;", port=5434, check=False).returncode != 0, "missing WAL cluster became writable")
        self.record("missing-wal-fails-before-recovery-target", missing_segment=target_segment, pg_ctl_exit=failed.returncode)

    def cleanup(self):
        failed = []
        for name in reversed(self.resources):
            # Exact unique names only; -v removes only this run's anonymous image volume.
            result = command(["docker", "rm", "-fv", name], check=False)
            if result.returncode and "No such container" not in result.stderr:
                failed.append(name)
        if failed:
            raise RuntimeError("cleanup failed for owned containers: " + ", ".join(failed))


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--only", choices=["all", "ledger", "outbox", "recovery"], default="all")
    parser.add_argument("--report", type=Path, help="write actual run evidence as JSON")
    args = parser.parse_args(argv)
    if args.report:
        args.report.parent.mkdir(parents=True, exist_ok=True)
        args.report.unlink(missing_ok=True)
    lab = Lab()
    pg_image = os.environ.get("LAB_PG_IMAGE", "postgres:17-alpine")
    redis_image = os.environ.get("LAB_REDIS_IMAGE", "redis:7.4-alpine")
    try:
        lab.setup(pg_image)
        if args.only in ("all", "ledger"):
            lab.ledger()
        if args.only in ("all", "outbox"):
            lab.outbox(redis_image)
        if args.only in ("all", "recovery"):
            lab.recovery(redis_image)
        images = {image: json.loads(command(["docker", "image", "inspect", image]).stdout)[0]["Id"]
                  for image in [pg_image] + ([redis_image] if args.only in ("all", "outbox", "recovery") else [])}
        report = dict(run_id=lab.token, selection=args.only, postgres_version=lab.sql("SELECT version();", "postgres"),
                      verified_at=datetime.now(timezone.utc).isoformat(), image_ids=images,
                      source_sha256={name: hashlib.sha256((HERE/name).read_bytes()).hexdigest()
                                     for name in SOURCE_FILES},
                      scenarios=lab.evidence, passed=True)
    except BaseException:
        for name in lab.resources:
            print(command(["docker", "logs", "--tail", "35", name], check=False).stderr)
        raise
    finally:
        lab.cleanup()
    # Cleanup failure is also a failed run; do not publish success before it.
    if args.report:
        write_report(args.report, report)
    print(json.dumps(dict(passed=True, scenarios=len(lab.evidence)), ensure_ascii=False))


if __name__ == "__main__":
    main()
