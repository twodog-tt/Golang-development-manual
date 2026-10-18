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
import time
import uuid

HERE = Path(__file__).resolve().parent


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
        self.sql("UPDATE lab.accounts SET balance=balance+1 WHERE id='bob';")
        require(self.sql("SELECT count(*) FROM lab.balance_drift;") == "1", "drift not detected")
        self.sql("BEGIN; LOCK TABLE lab.accounts IN EXCLUSIVE MODE; UPDATE lab.accounts a SET balance=opening_balance+coalesce((SELECT sum(e.delta) FROM lab.entries e WHERE e.account_id=a.id),0); COMMIT;")
        self.assert_balances()
        self.record("balance-drift-detected-and-rebuilt-from-entries")

    def redis_cmd(self, *args):
        return command(["docker", "exec", self.redis, "redis-cli", "--json", *args]).stdout.strip()

    def outbox(self, redis_image):
        self.start(self.redis, redis_image, ["redis-server", "--appendonly", "yes", "--appendfsync", "always"])
        self.wait(lambda: command(["docker", "exec", self.redis, "redis-cli", "ping"], check=False).stdout.strip() == "PONG", "Redis")
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

    def recovery(self):
        self.sql("SELECT lab.transfer('restore-kept','alice_available','bob','USDC',7);")
        self.shell("pg_basebackup -U postgres -D /lab/base -X stream -c fast\npg_verifybackup /lab/base\n")
        # Ensure target is strictly after the base backup and in its own WAL segment.
        self.sql("SELECT pg_switch_wal();")
        self.sql("SELECT lab.transfer('after-base','alice_available','bob','USDC',3);")
        target_segment = self.sql("SELECT pg_walfile_name(pg_current_wal_insert_lsn());")
        self.sql("SELECT pg_create_restore_point('before_delete');")
        before_count = self.sql("SELECT count(*) FROM lab.journals;")
        before_balance = self.sql("SELECT balance FROM lab.accounts WHERE id='bob';")
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

        self.shell(f"cp -a /lab/archive /lab/broken-archive\nrm /lab/broken-archive/{target_segment}\n")
        failed = restore("missingwal", 5434, "broken-archive")
        # pg_ctl may return as soon as consistent hot standby accepts connections;
        # inspect postmaster exit/log rather than equating readiness with reaching target.
        self.wait(lambda: "recovery ended before configured recovery target was reached" in self.shell("cat /lab/missingwal.log\n").stdout,
                  "missing WAL failure", seconds=20)
        require(self.sql("SELECT 1;", port=5434, check=False).returncode != 0, "missing WAL cluster became writable")
        self.record("missing-wal-fails-before-recovery-target", missing_segment=target_segment, pg_ctl_exit=failed.returncode)

    def cleanup(self):
        for name in reversed(self.resources):
            # Exact unique names only; -v removes only this run's anonymous image volume.
            command(["docker", "rm", "-fv", name], check=False)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--only", choices=["all", "ledger", "outbox", "recovery"], default="all")
    parser.add_argument("--report", type=Path, help="write actual run evidence as JSON")
    args = parser.parse_args()
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
            lab.recovery()
        images = {image: json.loads(command(["docker", "image", "inspect", image]).stdout)[0]["Id"]
                  for image in [pg_image] + ([redis_image] if args.only in ("all", "outbox") else [])}
        report = dict(run_id=lab.token, postgres_version=lab.sql("SELECT version();", "postgres"),
                      verified_at=datetime.now(timezone.utc).isoformat(), image_ids=images,
                      source_sha256={name: hashlib.sha256((HERE/name).read_bytes()).hexdigest()
                                     for name in ("schema.sql", "consumer.sql", "verify.py")},
                      scenarios=lab.evidence, passed=True)
        if args.report:
            args.report.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
        print(json.dumps(dict(passed=True, scenarios=len(lab.evidence)), ensure_ascii=False))
    except BaseException:
        for name in lab.resources:
            print(command(["docker", "logs", "--tail", "35", name], check=False).stderr)
        raise
    finally:
        lab.cleanup()


if __name__ == "__main__":
    main()
