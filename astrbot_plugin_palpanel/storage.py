from __future__ import annotations

import hashlib
import secrets
import time
from pathlib import Path

import aiosqlite


async def _fetchone(db: aiosqlite.Connection, query: str, params: tuple = ()):
    cursor = await db.execute(query, params)
    try:
        return await cursor.fetchone()
    finally:
        await cursor.close()


class PalPanelStore:
    def __init__(self, path: Path):
        self.path = path

    async def initialize(self) -> None:
        self.path.parent.mkdir(parents=True, exist_ok=True)
        async with aiosqlite.connect(self.path) as db:
            await db.executescript(
                """
                PRAGMA journal_mode=WAL;
                PRAGMA foreign_keys=ON;
                CREATE TABLE IF NOT EXISTS accounts (
                  qq_id TEXT PRIMARY KEY, balance INTEGER NOT NULL DEFAULT 0,
                  created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
                );
                CREATE TABLE IF NOT EXISTS bindings (
                  qq_id TEXT PRIMARY KEY, player_uid TEXT NOT NULL UNIQUE, nickname TEXT NOT NULL,
                  source_fingerprint TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'active',
                  verified_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
                );
                CREATE TABLE IF NOT EXISTS binding_challenges (
                  id TEXT PRIMARY KEY, qq_id TEXT NOT NULL, player_uid TEXT NOT NULL, nickname TEXT NOT NULL,
                  code_hash TEXT NOT NULL, expires_at INTEGER NOT NULL, status TEXT NOT NULL DEFAULT 'pending',
                  created_at INTEGER NOT NULL
                );
                CREATE TABLE IF NOT EXISTS player_catalog (
                  player_uid TEXT PRIMARY KEY, nickname TEXT NOT NULL, online INTEGER NOT NULL DEFAULT 0,
                  source_fingerprint TEXT NOT NULL, updated_at INTEGER NOT NULL
                );
                CREATE TABLE IF NOT EXISTS checkins (
                  qq_id TEXT NOT NULL, local_date TEXT NOT NULL, points INTEGER NOT NULL, created_at INTEGER NOT NULL,
                  PRIMARY KEY(qq_id,local_date)
                );
                CREATE TABLE IF NOT EXISTS credit_ledger (
                  id TEXT PRIMARY KEY, qq_id TEXT NOT NULL, delta INTEGER NOT NULL, reason TEXT NOT NULL,
                  reference_id TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL
                );
                CREATE TABLE IF NOT EXISTS credit_reservations (
                  id TEXT PRIMARY KEY, qq_id TEXT NOT NULL, amount INTEGER NOT NULL, reference_id TEXT NOT NULL UNIQUE,
                  status TEXT NOT NULL, expires_at INTEGER NOT NULL, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
                );
                CREATE TABLE IF NOT EXISTS login_tickets (
                  token_hash TEXT PRIMARY KEY, qq_id TEXT NOT NULL, expires_at INTEGER NOT NULL,
                  used_at INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL
                );
                CREATE TABLE IF NOT EXISTS audit_logs (
                  id TEXT PRIMARY KEY, actor TEXT NOT NULL, action TEXT NOT NULL, target TEXT NOT NULL,
                  detail TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL
                );
                """
            )
            await db.commit()
        await self.release_expired_reservations()

    async def release_expired_reservations(self) -> int:
        now = int(time.time())
        async with aiosqlite.connect(self.path) as db:
            await db.execute("BEGIN IMMEDIATE")
            rows = await db.execute_fetchall(
                "SELECT id,qq_id,amount FROM credit_reservations WHERE status='reserved' AND expires_at<?",
                (now,),
            )
            for reservation_id, qq_id, amount in rows:
                await db.execute("UPDATE accounts SET balance=balance+?,updated_at=? WHERE qq_id=?", (int(amount), now, qq_id))
                await db.execute("UPDATE credit_reservations SET status='expired',updated_at=? WHERE id=?", (now, reservation_id))
            await db.commit()
            return len(rows)

    async def sync_catalog(self, players: list[dict], fingerprint: str) -> None:
        now = int(time.time())
        async with aiosqlite.connect(self.path) as db:
            await db.execute("BEGIN IMMEDIATE")
            await db.execute("DELETE FROM player_catalog")
            await db.executemany(
                "INSERT INTO player_catalog(player_uid,nickname,online,source_fingerprint,updated_at) VALUES(?,?,?,?,?)",
                [(str(p["player_uid"]), str(p["nickname"]), 1 if p.get("online") else 0, fingerprint, now) for p in players],
            )
            await db.execute(
                "UPDATE bindings SET status='frozen',updated_at=? WHERE player_uid NOT IN (SELECT player_uid FROM player_catalog)",
                (now,),
            )
            await db.execute(
                "UPDATE bindings SET status='active',source_fingerprint=(SELECT source_fingerprint FROM player_catalog WHERE player_uid=bindings.player_uid),updated_at=? WHERE player_uid IN (SELECT player_uid FROM player_catalog)",
                (now,),
            )
            await db.commit()

    async def player_by_nickname(self, nickname: str) -> list[dict]:
        async with aiosqlite.connect(self.path) as db:
            db.row_factory = aiosqlite.Row
            rows = await db.execute_fetchall("SELECT * FROM player_catalog WHERE nickname=? COLLATE NOCASE", (nickname.strip(),))
            return [dict(row) for row in rows]

    async def create_challenge(self, qq_id: str, player_uid: str, nickname: str, code: str, ttl: int = 300) -> str:
        challenge_id = secrets.token_hex(12)
        now = int(time.time())
        code_hash = hashlib.sha256(code.encode()).hexdigest()
        async with aiosqlite.connect(self.path) as db:
            await db.execute("UPDATE binding_challenges SET status='expired' WHERE qq_id=? AND status='pending'", (qq_id,))
            await db.execute(
                "INSERT INTO binding_challenges VALUES(?,?,?,?,?,?,?,?)",
                (challenge_id, qq_id, player_uid, nickname, code_hash, now + ttl, "pending", now),
            )
            await db.commit()
        return challenge_id

    async def confirm_challenge(self, qq_id: str, code: str) -> dict | None:
        now = int(time.time())
        code_hash = hashlib.sha256(code.encode()).hexdigest()
        async with aiosqlite.connect(self.path) as db:
            db.row_factory = aiosqlite.Row
            await db.execute("BEGIN IMMEDIATE")
            row = await _fetchone(db,
                "SELECT * FROM binding_challenges WHERE qq_id=? AND code_hash=? AND status='pending' AND expires_at>=? ORDER BY created_at DESC LIMIT 1",
                (qq_id, code_hash, now),
            )
            if row is None:
                await db.rollback()
                return None
            item = dict(row)
            await db.execute("UPDATE binding_challenges SET status='used' WHERE id=?", (item["id"],))
            await db.execute(
                "INSERT INTO bindings(qq_id,player_uid,nickname,source_fingerprint,status,verified_at,updated_at) VALUES(?,?,?,'','active',?,?) ON CONFLICT(qq_id) DO UPDATE SET player_uid=excluded.player_uid,nickname=excluded.nickname,status='active',verified_at=excluded.verified_at,updated_at=excluded.updated_at",
                (qq_id, item["player_uid"], item["nickname"], now, now),
            )
            await db.execute("INSERT OR IGNORE INTO accounts VALUES(?,0,?,?)", (qq_id, now, now))
            await db.commit()
            return item

    async def binding(self, qq_id: str) -> dict | None:
        async with aiosqlite.connect(self.path) as db:
            db.row_factory = aiosqlite.Row
            row = await _fetchone(db, "SELECT * FROM bindings WHERE qq_id=?", (qq_id,))
            return dict(row) if row else None

    async def checkin(self, qq_id: str, local_date: str, points: int) -> tuple[bool, int]:
        now = int(time.time())
        async with aiosqlite.connect(self.path) as db:
            await db.execute("BEGIN IMMEDIATE")
            await db.execute("INSERT OR IGNORE INTO accounts VALUES(?,0,?,?)", (qq_id, now, now))
            cursor = await db.execute("INSERT OR IGNORE INTO checkins VALUES(?,?,?,?)", (qq_id, local_date, points, now))
            awarded = cursor.rowcount == 1
            if awarded:
                await db.execute("UPDATE accounts SET balance=balance+?,updated_at=? WHERE qq_id=?", (points, now, qq_id))
                await db.execute("INSERT INTO credit_ledger VALUES(?,?,?,?,?,?)", (secrets.token_hex(12), qq_id, points, "daily_checkin", local_date, now))
            row = await _fetchone(db, "SELECT balance FROM accounts WHERE qq_id=?", (qq_id,))
            await db.commit()
            return awarded, int(row[0])

    async def balance(self, qq_id: str) -> int:
        async with aiosqlite.connect(self.path) as db:
            row = await _fetchone(db, "SELECT balance FROM accounts WHERE qq_id=?", (qq_id,))
            return int(row[0]) if row else 0

    async def reserve(self, qq_id: str, reference_id: str, amount: int, ttl: int = 900) -> tuple[bool, str, int]:
        await self.release_expired_reservations()
        now = int(time.time())
        reservation_id = secrets.token_hex(12)
        async with aiosqlite.connect(self.path) as db:
            await db.execute("BEGIN IMMEDIATE")
            existing = await _fetchone(db, "SELECT id,status FROM credit_reservations WHERE reference_id=?", (reference_id,))
            if existing:
                balance_row = await _fetchone(db, "SELECT balance FROM accounts WHERE qq_id=?", (qq_id,))
                balance = int(balance_row[0]) if balance_row else 0
                await db.rollback()
                return True, str(existing[0]), balance
            row = await _fetchone(db, "SELECT balance FROM accounts WHERE qq_id=?", (qq_id,))
            balance = int(row[0]) if row else 0
            if balance < amount:
                await db.rollback()
                return False, "", balance
            await db.execute("UPDATE accounts SET balance=balance-?,updated_at=? WHERE qq_id=?", (amount, now, qq_id))
            await db.execute("INSERT INTO credit_reservations VALUES(?,?,?,?,?,?,?,?)", (reservation_id, qq_id, amount, reference_id, "reserved", now + ttl, now, now))
            await db.commit()
            return True, reservation_id, balance - amount

    async def admin_binding(self, actor: str, qq_id: str, player_uid: str, nickname: str) -> None:
        now = int(time.time())
        async with aiosqlite.connect(self.path) as db:
            await db.execute("BEGIN IMMEDIATE")
            await db.execute("INSERT OR IGNORE INTO accounts VALUES(?,0,?,?)", (qq_id, now, now))
            await db.execute(
                "INSERT INTO bindings(qq_id,player_uid,nickname,source_fingerprint,status,verified_at,updated_at) VALUES(?,?,?,'','active',?,?) ON CONFLICT(qq_id) DO UPDATE SET player_uid=excluded.player_uid,nickname=excluded.nickname,status='active',verified_at=excluded.verified_at,updated_at=excluded.updated_at",
                (qq_id, player_uid, nickname, now, now),
            )
            await db.execute("INSERT INTO audit_logs VALUES(?,?,?,?,?,?)", (secrets.token_hex(12), actor, "binding.manual", qq_id, player_uid, now))
            await db.commit()

    async def set_binding_status(self, actor: str, qq_id: str, status: str) -> bool:
        now = int(time.time())
        async with aiosqlite.connect(self.path) as db:
            cursor = await db.execute("UPDATE bindings SET status=?,updated_at=? WHERE qq_id=?", (status, now, qq_id))
            changed = cursor.rowcount == 1
            if changed:
                await db.execute("INSERT INTO audit_logs VALUES(?,?,?,?,?,?)", (secrets.token_hex(12), actor, f"binding.{status}", qq_id, "", now))
            await db.commit()
            return changed

    async def adjust_points(self, actor: str, qq_id: str, delta: int, reason: str) -> int:
        now = int(time.time())
        async with aiosqlite.connect(self.path) as db:
            await db.execute("BEGIN IMMEDIATE")
            await db.execute("INSERT OR IGNORE INTO accounts VALUES(?,0,?,?)", (qq_id, now, now))
            row = await _fetchone(db, "SELECT balance FROM accounts WHERE qq_id=?", (qq_id,))
            next_balance = max(0, int(row[0]) + delta)
            applied = next_balance - int(row[0])
            await db.execute("UPDATE accounts SET balance=?,updated_at=? WHERE qq_id=?", (next_balance, now, qq_id))
            await db.execute("INSERT INTO credit_ledger VALUES(?,?,?,?,?,?)", (secrets.token_hex(12), qq_id, applied, reason, actor, now))
            await db.execute("INSERT INTO audit_logs VALUES(?,?,?,?,?,?)", (secrets.token_hex(12), actor, "credits.adjust", qq_id, f"{applied}:{reason}", now))
            await db.commit()
            return next_balance

    async def ledger(self, qq_id: str, limit: int = 10) -> list[dict]:
        async with aiosqlite.connect(self.path) as db:
            db.row_factory = aiosqlite.Row
            rows = await db.execute_fetchall(
                "SELECT delta,reason,reference_id,created_at FROM credit_ledger WHERE qq_id=? ORDER BY created_at DESC LIMIT ?",
                (qq_id, max(1, min(limit, 50))),
            )
            return [dict(row) for row in rows]

    async def settle(self, reservation_id: str, commit: bool) -> bool:
        now = int(time.time())
        async with aiosqlite.connect(self.path) as db:
            await db.execute("BEGIN IMMEDIATE")
            row = await _fetchone(db, "SELECT qq_id,amount,reference_id,status FROM credit_reservations WHERE id=?", (reservation_id,))
            if not row or row[3] != "reserved":
                await db.rollback()
                return False
            status = "committed" if commit else "released"
            if commit:
                await db.execute("INSERT INTO credit_ledger VALUES(?,?,?,?,?,?)", (secrets.token_hex(12), row[0], -int(row[1]), "breeding_solve", row[2], now))
            else:
                await db.execute("UPDATE accounts SET balance=balance+?,updated_at=? WHERE qq_id=?", (int(row[1]), now, row[0]))
            await db.execute("UPDATE credit_reservations SET status=?,updated_at=? WHERE id=?", (status, now, reservation_id))
            await db.commit()
            return True

    async def issue_ticket(self, qq_id: str, ttl: int) -> str:
        token = secrets.token_urlsafe(32)
        token_hash = hashlib.sha256(token.encode()).hexdigest()
        now = int(time.time())
        async with aiosqlite.connect(self.path) as db:
            await db.execute("INSERT INTO login_tickets VALUES(?,?,?,?,?)", (token_hash, qq_id, now + ttl, 0, now))
            await db.commit()
        return token

    async def exchange_ticket(self, token: str) -> dict | None:
        token_hash = hashlib.sha256(token.encode()).hexdigest()
        now = int(time.time())
        async with aiosqlite.connect(self.path) as db:
            db.row_factory = aiosqlite.Row
            await db.execute("BEGIN IMMEDIATE")
            row = await _fetchone(db, "SELECT qq_id FROM login_tickets WHERE token_hash=? AND used_at=0 AND expires_at>=?", (token_hash, now))
            if not row:
                await db.rollback()
                return None
            await db.execute("UPDATE login_tickets SET used_at=? WHERE token_hash=?", (now, token_hash))
            binding = await _fetchone(db, "SELECT * FROM bindings WHERE qq_id=? AND status='active'", (row[0],))
            balance = await _fetchone(db, "SELECT balance FROM accounts WHERE qq_id=?", (row[0],))
            await db.commit()
            if not binding:
                return None
            result = dict(binding)
            result["qq_id"] = row[0]
            result["balance"] = int(balance[0]) if balance else 0
            return result
