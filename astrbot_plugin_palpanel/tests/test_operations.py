import unittest

from astrbot_plugin_palpanel.operations import (
    admin_allowed,
    CooldownGuard,
    format_online_players,
    format_rooms,
    format_server_status,
    group_allowed,
    parse_wait_seconds,
)


class OperationsTests(unittest.TestCase):
    def test_wait_seconds_defaults_and_validates(self):
        self.assertEqual(parse_wait_seconds(""), 60)
        self.assertEqual(parse_wait_seconds("30"), 30)
        for value in ("4", "301", "bad"):
            with self.assertRaises((ValueError, TypeError)):
                parse_wait_seconds(value)

    def test_cooldown_is_scoped_by_user_group_and_action(self):
        guard = CooldownGuard(query_seconds=5, control_seconds=15)
        self.assertEqual(guard.retry_after("g1", "u1", "status", now=100), 0)
        self.assertEqual(guard.retry_after("g1", "u1", "status", now=101), 4)
        self.assertEqual(guard.retry_after("g1", "u2", "status", now=101), 1)
        self.assertEqual(guard.retry_after("g2", "u1", "status", now=101), 0)
        self.assertEqual(guard.retry_after("g1", "u1", "rooms", now=101), 0)
        self.assertEqual(guard.retry_after("g1", "u1", "control", control=True, now=101), 0)
        self.assertEqual(guard.retry_after("g1", "u1", "control", control=True, now=102), 14)

    def test_status_and_online_formatting(self):
        payload = {"data": {
            "server": {"container": {"exists": True, "status": "running"}},
            "info": {"server_name": "测试服", "version": "1.0"},
            "online_count": 2,
            "online_players": [{"name": "甲", "level": 10}, {"name": "乙"}],
        }}
        status = format_server_status(payload)
        self.assertIn("测试服", status)
        self.assertIn("运行中", status)
        self.assertIn("在线：2 人", status)
        online = format_online_players(payload)
        self.assertIn("甲（Lv.10）", online)
        self.assertIn("乙", online)
        self.assertEqual(
            format_online_players({"data": {"online_players": [], "players_available": False}}),
            "暂时无法查询在线玩家，请稍后重试。",
        )

    def test_group_and_admin_command_boundaries(self):
        self.assertTrue(group_allowed("20002", "20002"))
        self.assertFalse(group_allowed("20002", "30003"))
        self.assertFalse(group_allowed("", None))
        self.assertTrue(admin_allowed("10001, 10002", "10002"))
        self.assertFalse(admin_allowed("10001,10002", "10003"))

    def test_room_output_limits_results_and_characters(self):
        rooms = [{
            "name": f"国内社区服-{index}", "address": "1.2.3.4", "port": 8211,
            "players": index, "max_players": 32, "country": "CN",
        } for index in range(12)]
        result = format_rooms({"data": {"servers": rooms, "stale": True}}, limit=3, max_chars=400)
        self.assertIn("缓存数据", result)
        self.assertIn("1.2.3.4:8211", result)
        self.assertIn("仅显示前 3 条", result)
        self.assertNotIn("国内社区服-4", result)


if __name__ == "__main__":
    unittest.main()
