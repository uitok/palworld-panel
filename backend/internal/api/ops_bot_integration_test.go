package api

import "testing"

func TestNormalizeAstrBotPlayersBoundsAndNormalizesFields(t *testing.T) {
	players := normalizeAstrBotPlayers(map[string]any{"players": []any{
		map[string]any{"name": "Alice", "playerId": "player-1", "userId": "steam-1", "level": float64(12)},
		"invalid",
		map[string]any{"nickname": "Bob", "player_id": "player-2", "user_id": "steam-2"},
	}})
	if len(players) != 2 {
		t.Fatalf("players = %#v", players)
	}
	if players[0]["name"] != "Alice" || players[0]["player_id"] != "player-1" || players[0]["user_id"] != "steam-1" {
		t.Fatalf("first player = %#v", players[0])
	}
	if players[1]["name"] != "Bob" || players[1]["player_id"] != "player-2" || players[1]["user_id"] != "steam-2" {
		t.Fatalf("second player = %#v", players[1])
	}
}

func TestFirstAstrBotStringSkipsEmptyValues(t *testing.T) {
	values := map[string]any{"first": " ", "second": "value"}
	if got := firstAstrBotString(values, "first", "second"); got != "value" {
		t.Fatalf("firstAstrBotString = %q", got)
	}
}
