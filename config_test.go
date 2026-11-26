package main

import "testing"

func TestLoadTMDBConfig(t *testing.T) {
	t.Setenv("ICEDREEL_TMDB_APIKEY", "")
	t.Setenv("ICEDREEL_TMDB_TOKEN", "")
	t.Setenv("ICEDSUB_TMDB_APIKEY", "legacy-key")
	t.Setenv("ICEDSUB_TMDB_TOKEN", "legacy-token")

	cfg := loadTMDBConfig()
	if cfg.APIKey != "" || cfg.Token != "" {
		t.Fatalf("ICEDSUB_* must be ignored, got %+v", cfg)
	}

	t.Setenv("ICEDREEL_TMDB_APIKEY", "new-key")
	cfg = loadTMDBConfig()
	if cfg.APIKey != "new-key" || cfg.Token != "" {
		t.Fatalf("unexpected config: %+v", cfg)
	}

	t.Setenv("ICEDREEL_TMDB_TOKEN", "new-token")
	cfg = loadTMDBConfig()
	if cfg.APIKey != "new-key" || cfg.Token != "new-token" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}
