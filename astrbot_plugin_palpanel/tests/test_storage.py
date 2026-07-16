import asyncio
import tempfile
import unittest
from pathlib import Path

from astrbot_plugin_palpanel.storage import PalPanelStore


class StorageTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.store = PalPanelStore(Path(self.temp.name) / "palpanel.sqlite3")
        await self.store.initialize()

    async def asyncTearDown(self):
        self.temp.cleanup()

    async def test_checkin_is_unique_under_concurrency(self):
        results = await asyncio.gather(*(self.store.checkin("10001", "2026-07-16", 10) for _ in range(4)))
        self.assertEqual(sum(1 for awarded, _ in results if awarded), 1)
        self.assertEqual(await self.store.balance("10001"), 10)

    async def test_reservation_commit_and_release_are_idempotent(self):
        await self.store.adjust_points("test", "10001", 10, "seed")
        ok, reservation, balance = await self.store.reserve("10001", "solve-1", 1)
        self.assertTrue(ok)
        self.assertEqual(balance, 9)
        self.assertTrue(await self.store.settle(reservation, False))
        self.assertFalse(await self.store.settle(reservation, False))
        self.assertEqual(await self.store.balance("10001"), 10)

        ok, reservation, _ = await self.store.reserve("10001", "solve-2", 1)
        self.assertTrue(ok)
        self.assertTrue(await self.store.settle(reservation, True))
        self.assertEqual(await self.store.balance("10001"), 9)
        ledger = await self.store.ledger("10001")
        self.assertTrue(any(item["reason"] == "breeding_solve" and item["delta"] == -1 for item in ledger))

    async def test_ticket_is_single_use_and_binding_freezes_by_player_uid(self):
        await self.store.sync_catalog([{"player_uid": "uid-1", "nickname": "玩家", "online": True}], "save-a")
        await self.store.admin_binding("test", "10001", "uid-1", "玩家")
        token = await self.store.issue_ticket("10001", 300)
        identity = await self.store.exchange_ticket(token)
        self.assertEqual(identity["player_uid"], "uid-1")
        self.assertIsNone(await self.store.exchange_ticket(token))

        await self.store.sync_catalog([{"player_uid": "uid-2", "nickname": "玩家", "online": False}], "save-b")
        binding = await self.store.binding("10001")
        self.assertEqual(binding["status"], "frozen")


if __name__ == "__main__":
    unittest.main()
