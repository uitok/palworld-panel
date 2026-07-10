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
