package palconfig

import (
	"strings"
	"testing"
)

func TestParseSettingsWithQuotedComma(t *testing.T) {
	content := `[/Script/Pal.PalGameWorldSettings]
OptionSettings=(ServerName="My, Server",RESTAPIEnabled=True,DayTimeSpeedRate=1.5)`
	got, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got["ServerName"] != "My, Server" {
		t.Fatalf("ServerName = %q", got["ServerName"])
	}
	if got["RESTAPIEnabled"] != "True" {
		t.Fatalf("RESTAPIEnabled = %q", got["RESTAPIEnabled"])
	}
	if got["DayTimeSpeedRate"] != "1.5" {
		t.Fatalf("DayTimeSpeedRate = %q", got["DayTimeSpeedRate"])
	}
}

func TestSerializeSettingsQuotesStrings(t *testing.T) {
	got := Serialize(Settings{"ServerName": "Pal Panel", "RESTAPIEnabled": "True", "PublicPort": "8211"})
	if want := `RESTAPIEnabled=True`; !contains(got, want) {
		t.Fatalf("serialized settings missing %s in %s", want, got)
	}
	if want := `ServerName="Pal Panel"`; !contains(got, want) {
		t.Fatalf("serialized settings missing %s in %s", want, got)
	}
	if want := `PublicPort=8211`; !contains(got, want) {
		t.Fatalf("serialized settings missing %s in %s", want, got)
	}
}

func TestSerializeUsesSchemaForPasswordStrings(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "numeric", value: "123456", want: `ServerPassword="123456"`},
		{name: "boolean-looking", value: "True", want: `ServerPassword="True"`},
		{name: "none-looking", value: "None", want: `ServerPassword="None"`},
		{name: "empty", value: "", want: `ServerPassword=""`},
		{name: "escaped", value: `say "hi" \\ path,rest`, want: `ServerPassword="say \"hi\" \\\\ path,rest"`},
		{name: "unicode", value: "密碼パス", want: `ServerPassword="密碼パス"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Serialize(Settings{"ServerPassword": test.value})
			if !strings.Contains(got, test.want) {
				t.Fatalf("Serialize(ServerPassword=%q) = %s, want %s", test.value, got, test.want)
			}
		})
	}

	got := Serialize(Settings{"AdminPassword": "1e6", "RESTAPIEnabled": "true", "PublicPort": "08211"})
	for _, want := range []string{`AdminPassword="1e6"`, `RESTAPIEnabled=True`, `PublicPort=8211`} {
		if !strings.Contains(got, want) {
			t.Fatalf("schema serialization missing %s in %s", want, got)
		}
	}
}

func TestSerializeIntPreservesMaxInt64AndExactScientificIntegers(t *testing.T) {
	got := Serialize(Settings{
		"ServerPlayerMaxNum":              "9223372036854775807",
		"ServerReplicatePawnCullDistance": "1.5e4",
	})
	for _, want := range []string{
		"ServerPlayerMaxNum=9223372036854775807",
		"ServerReplicatePawnCullDistance=15000",
	} {
		if !contains(got, want) {
			t.Fatalf("Serialize() = %s, missing %s", got, want)
		}
	}
}

func TestParseAndSerializePreservesUnknownRawValue(t *testing.T) {
	input := SectionHeader + "\n" + `OptionSettings=(FutureOfficialKey="123456",ServerName="Before")` + "\n"
	document, err := ParseDocument(input)
	if err != nil {
		t.Fatal(err)
	}
	document.Settings["ServerName"] = "After"
	got := SerializeDocument(document, map[string]bool{"ServerName": true})
	if !strings.Contains(got, `FutureOfficialKey="123456"`) {
		t.Fatalf("unknown raw value changed: %s", got)
	}
}

func TestSerializeDocumentPreservesUnknownEmptyRawValue(t *testing.T) {
	document, err := ParseDocument(SectionHeader + "\nOptionSettings=(FutureOfficialKey=,ServerName=\"Before\")\n")
	if err != nil {
		t.Fatal(err)
	}
	document.Settings["ServerName"] = "After"
	got := SerializeDocument(document, map[string]bool{"ServerName": true})
	if !strings.Contains(got, "FutureOfficialKey=,") {
		t.Fatalf("unknown empty raw value changed: %s", got)
	}
}

func TestSerializeNormalizesStructuredLists(t *testing.T) {
	got := Serialize(Settings{
		"CrossplayPlatforms": "( Steam , Xbox,PS5 )",
		"DenyTechnologyList": `( "GrapplingGun" , "Laser,Turret" )`,
	})
	for _, want := range []string{
		"CrossplayPlatforms=(Steam,Xbox,PS5)",
		`DenyTechnologyList=("GrapplingGun","Laser,Turret")`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("structured list missing %s in %s", want, got)
		}
	}
}

func TestMergePreservesUnknownKeysAndDoesNotMaterializeMissingFields(t *testing.T) {
	got := Merge(Settings{"FutureOfficialKey": "keep", "ServerName": "old"}, map[string]any{"ServerName": "new"})
	if got["FutureOfficialKey"] != "keep" {
		t.Fatalf("unknown key was not preserved: %#v", got)
	}
	if _, ok := got["bEnableVoiceChat"]; ok {
		t.Fatalf("missing optional field was materialized: %#v", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
