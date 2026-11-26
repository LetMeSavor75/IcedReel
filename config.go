package main

import "os"

type tmdbConfig struct {
	APIKey string
	Token  string
}

func (c tmdbConfig) configured() bool {
	return c.APIKey != "" || c.Token != ""
}

func loadTMDBConfig() tmdbConfig {
	return tmdbConfig{
		APIKey: os.Getenv("ICEDREEL_TMDB_APIKEY"),
		Token:  os.Getenv("ICEDREEL_TMDB_TOKEN"),
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
