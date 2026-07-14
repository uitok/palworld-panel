package pallocalize

import "testing"

func TestChineseCatalogAndFallbacks(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Cattiva", got: PalName("PinkCat"), want: "捣蛋猫"},
		{name: "Teafant", got: PalName("ganesha"), want: "壶小象"},
		{name: "item", got: ItemName("Stone"), want: "石头"},
		{name: "passive", got: PassiveName("CraftSpeed_up3"), want: "卓绝技艺"},
		{name: "unknown Pal", got: PalName("FuturePal_1"), want: "FuturePal_1"},
		{name: "guild", got: GuildName("Unnamed Guild"), want: "未命名公会"},
		{name: "base", got: BaseName("新規生成拠点テンプレート名0(仮)"), want: "未命名据点"},
		{name: "Palbox", got: MapObjectName("PalBoxV2"), want: "帕鲁终端"},
		{name: "rock", got: MapObjectName("DamagableRock0011"), want: "可采集岩石"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("got %q, want %q", test.got, test.want)
			}
		})
	}
}

func TestSearchItemsUsesIDsAndChineseNamesWithLimits(t *testing.T) {
	byID := SearchItems("ExplosiveBullet", 10)
	if len(byID) != 1 || byID[0].ID != "ExplosiveBullet" || byID[0].Name != "火箭弹" || byID[0].Icon != "explosivebullet" {
		t.Fatalf("ID search = %#v", byID)
	}
	byName := SearchItems("金币", 10)
	if len(byName) != 1 || byName[0].ID != "Money" {
		t.Fatalf("name search = %#v", byName)
	}
	if got := SearchItems("", 2); len(got) != 2 || got[0].ID > got[1].ID {
		t.Fatalf("limited catalog = %#v", got)
	}
}
