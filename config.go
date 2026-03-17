package main

import (
	"bufio"
	"os"
	"strings"
)

type Config struct {
	Port               string
	MongoURI           string
	GoogleClientID     string
	GoogleClientSecret string
	OpenClawURL        string
	OpenClawToken      string
	JWTSecret          string
	CORSOrigins        []string
}

func LoadConfig() *Config {
	loadDotEnv(".env")

	origins := strings.Split(getEnv("CORS_ORIGINS", "http://localhost:3000,http://localhost:5173"), ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}

	return &Config{
		Port:               getEnv("PORT", "8080"),
		MongoURI:           getEnv("MONGODB_URI", ""),
		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		OpenClawURL:        getEnv("OPENCLAW_URL", "http://localhost:18789/v1/chat/completions"),
		OpenClawToken:      getEnv("OPENCLAW_TOKEN", ""),
		JWTSecret:          getEnv("JWT_SECRET", "erebus-secret-change-me"),
		CORSOrigins:        origins,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}
