from __future__ import annotations

import time
from dataclasses import dataclass
from typing import Any


def unwrap(payload: dict[str, Any]) -> Any:
    return payload.get("data", payload)


def parse_wait_seconds(value: str | int | None, default: int = 60) -> int:
    if value is None or str(value).strip() == "":
        return default
    wait = int(str(value).strip())
    if wait < 5 or wait > 300:
        raise ValueError("倒计时必须在 5 到 300 秒之间")
    return wait


@dataclass
class CooldownGuard:
    query_seconds: float = 5
    control_seconds: float = 15

    def __post_init__(self) -> None:
        self._seen: dict[tuple[str, str, str], float] = {}

    def retry_after(self, group_id: str, user_id: str, action: str, *, control: bool = False, now: float | None = None) -> int:
        current = time.monotonic() if now is None else now
        window = self.control_seconds if control else self.query_seconds
        key = (str(group_id), str(user_id), action)
        group_key = (str(group_id), "*", action)
        last = max(self._seen.get(key, -1e12), self._seen.get(group_key, -1e12))
        remaining = window - (current - last)
        if remaining > 0:
            return max(1, int(remaining + 0.999))
        self._seen[key] = current
        # A short group-wide floor prevents bursts from many accounts without
        # making normal use wait for the full per-user cooldown.
        self._seen[group_key] = current - max(0.0, window - min(2.0, window))
        cutoff = current - max(self.query_seconds, self.control_seconds) * 2
        self._seen = {key: seen for key, seen in self._seen.items() if seen >= cutoff}
        return 0


def format_server_status(payload: dict[str, Any]) -> str:
    data = unwrap(payload)
    server = data.get("server", {}) if isinstance(data, dict) else {}
    container = server.get("container", {}) if isinstance(server, dict) else {}
    state = str(container.get("status", "unknown"))
    running = bool(container.get("exists")) and state.lower() in {"running", "restarting", "created"}
    info = data.get("info", {}) if isinstance(data, dict) else {}
    if not isinstance(info, dict):
        info = {}
    name = info.get("servername") or info.get("serverName") or "Palworld 服务器"
    online = int(data.get("online_count", 0) or 0) if isinstance(data, dict) else 0
    version = info.get("version") or "未知"
    return f"{name}\n状态：{'运行中' if running else '已停止'}（{state}）\n在线：{online} 人\n版本：{version}"


def format_online_players(payload: dict[str, Any], max_chars: int = 1800) -> str:
    data = unwrap(payload)
    players = data.get("online_players", []) if isinstance(data, dict) else []
    if not players:
        error = data.get("players_error") if isinstance(data, dict) else None
        return "当前没有在线玩家。" if not error else "暂时无法查询在线玩家，请稍后重试。"
    lines = [f"当前在线 {len(players)} 人："]
    for index, player in enumerate(players, 1):
        name = str(player.get("name") or player.get("player_id") or "未知玩家")
        level = player.get("level")
        suffix = f"（Lv.{level}）" if level not in (None, "") else ""
        lines.append(f"{index}. {name}{suffix}")
    return truncate_text("\n".join(lines), max_chars)


def format_rooms(payload: dict[str, Any], limit: int = 10, max_chars: int = 1800) -> str:
    data = unwrap(payload)
    if isinstance(data, dict):
        rooms = data.get("servers") or data.get("items") or []
        stale = bool(data.get("stale"))
    else:
        rooms, stale = data if isinstance(data, list) else [], False
    if not rooms:
        return "没有找到匹配的可发现社区服务器。"
    lines = ["可发现社区服务器" + ("（缓存数据）" if stale else "") + "："]
    for index, room in enumerate(rooms[: max(1, limit)], 1):
        name = str(room.get("name") or "未命名服务器")
        host = room.get("address") or room.get("ip") or ""
        port = room.get("port") or room.get("query_port") or ""
        address = f"{host}:{port}" if host and port else str(host)
        players = room.get("players", room.get("current_players", 0))
        capacity = room.get("max_players", room.get("capacity", "?"))
        country = room.get("country") or room.get("country_code") or "--"
        lines.append(f"{index}. {name}\n   {address}｜{players}/{capacity}｜{country}")
    if len(rooms) > limit:
        lines.append(f"仅显示前 {limit} 条，请增加关键词缩小范围。")
    return truncate_text("\n".join(lines), max_chars)


def truncate_text(value: str, max_chars: int) -> str:
    if max_chars < 32:
        max_chars = 32
    if len(value) <= max_chars:
        return value
    return value[: max_chars - 12].rstrip() + "\n…内容已截断"

