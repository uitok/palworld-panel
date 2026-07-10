package server

import (
	"strings"
	"testing"

	"palpanel/internal/appconfig"
)

func TestStartupArgsForBothRuntimes(t *testing.T) {
	cfg := appconfig.Config{GamePort: 8211, ServerDir: `D:\PalServer`}
	startup := StartupConfig{
		Port:                        8000,
		Players:                     24,
		PublicLobby:                 true,
		PublicIP:                    "203.0.113.10",
		PublicPort:                  9000,
		LogFormat:                   "json",
		UsePerfThreads:              true,
		NoAsyncLoadingThread:        true,
		UseMultithreadForDS:         true,
		NumberOfWorkerThreadsServer: 8,
		WorkshopDir:                 `D:\PalServer\Mods\Workshop`,
	}
	args := strings.Join(startup.Args(cfg), " ")
	for _, want := range []string{
		"-port=8000",
		"-players=24",
		"-enable-gamedata-api",
		"-publiclobby",
		"-publicip=203.0.113.10",
		"-publicport=9000",
		"-logformat=json",
		"-useperfthreads",
		"-NoAsyncLoadingThread",
		"-UseMultithreadForDS",
		"-NumberOfWorkerThreadsServer=8",
		`-workshopdir=D:\PalServer\Mods\Workshop`,
		"-log",
		"-stdout",
		"-FullStdOutLogOutput",
	} {
		if !strings.Contains(args, want) {
			t.Fatalf("args missing %s in %s", want, args)
		}
	}
}

func TestAppendUniqueArgsDeduplicatesCaseInsensitivelyWithoutRemovingLogFormat(t *testing.T) {
	args := appendUniqueArgs([]string{"-LOG", "-stdout", "-logformat=json"}, "-log", "-STDOUT", "-FullStdOutLogOutput")
	joined := strings.Join(args, " ")
	if strings.Count(strings.ToLower(joined), "-log ") != 1 || strings.Count(strings.ToLower(joined), "-stdout") != 1 {
		t.Fatalf("flags were not deduplicated: %s", joined)
	}
	if !strings.Contains(joined, "-logformat=json") || !strings.Contains(joined, "-FullStdOutLogOutput") {
		t.Fatalf("required logging args missing: %s", joined)
	}
}

func TestStartupValidation(t *testing.T) {
	got := StartupConfig{Port: 70000, Players: 0, LogFormat: "xml"}.Validate()
	if !hasErrors(got) {
		t.Fatalf("expected validation errors: %#v", got)
	}
}
