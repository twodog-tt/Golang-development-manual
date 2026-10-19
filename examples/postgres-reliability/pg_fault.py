"""Local fixture: intercept COMMIT completion before exposing it to the caller.

PostgreSQL v3 over docker-exec/nc pipes; no host port or third-party driver.
Only the fault injector sees CommandComplete(COMMIT). The caller gets UNKNOWN.
"""
import os
import select
import struct
import subprocess
import time


class CommitOutcomeUnknown(RuntimeError):
    pass


def commit_without_ack(container, database, query):
    child = subprocess.Popen(["docker", "exec", "-i", container, "nc", "127.0.0.1", "5432"],
                             stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                             bufsize=0)
    deadline = time.monotonic() + 15

    def read(size):
        data = b""
        while len(data) < size:
            timeout = deadline - time.monotonic()
            if timeout <= 0 or not select.select([child.stdout], [], [], timeout)[0]:
                raise TimeoutError("waiting for PostgreSQL fault point")
            part = os.read(child.stdout.fileno(), size - len(data))
            if not part:
                raise RuntimeError("PostgreSQL transport ended before COMMIT")
            data += part
        return data

    def message():
        kind = read(1)
        size = struct.unpack("!I", read(4))[0]
        if not 4 <= size <= 1024 * 1024:
            raise RuntimeError("invalid PostgreSQL frame")
        payload = read(size - 4)
        if kind == b"E":
            raise RuntimeError("PostgreSQL rejected fault transaction: " + payload.decode(errors="replace"))
        return kind, payload

    def send(data):
        # Unbuffered pipes may accept only part of a frame.
        while data:
            written = os.write(child.stdin.fileno(), data)
            data = data[written:]

    try:
        startup = struct.pack("!I", 196608) + b"user\0postgres\0database\0" + database.encode() + b"\0\0"
        send(struct.pack("!I", len(startup) + 4) + startup)
        while message()[0] != b"Z":
            pass
        body = ("BEGIN; " + query + "; COMMIT;").encode() + b"\0"
        send(b"Q" + struct.pack("!I", len(body) + 4) + body)
        while True:
            kind, payload = message()
            if kind == b"C" and payload == b"COMMIT\0":
                raise CommitOutcomeUnknown("fixture dropped COMMIT acknowledgement")
    finally:
        child.stdin.close()
        child.stdout.close()
        child.terminate()
        try:
            child.wait(timeout=5)
        except subprocess.TimeoutExpired:
            child.kill()
            child.wait()
        child.stderr.close()
