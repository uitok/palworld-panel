package palconfig

import "testing"

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
