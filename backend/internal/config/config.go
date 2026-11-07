package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr        string
	DatabaseDSN     string
	SolanaRPCURL    string
	SolanaProgramID string
	USDCMint        string
	AllowedOrigins  []string
	CookieName      string
	CookieSecure    bool
	SessionTTL      time.Duration
	ChallengeTTL    time.Duration
	ReconcileEvery  time.Duration
}

func Load() Config {
	return Config{
		HTTPAddr:        value("PACT_HTTP_ADDR", ":8080"),
		DatabaseDSN:     value("PACT_DATABASE_DSN", "file:pact.db"),
		SolanaRPCURL:    value("PACT_SOLANA_RPC_URL", "http://127.0.0.1:8899"),
		SolanaProgramID: os.Getenv("PACT_SOLANA_PROGRAM_ID"),
		USDCMint:        os.Getenv("PACT_USDC_MINT"),
		AllowedOrigins:  csv("PACT_ALLOWED_ORIGINS", "http://localhost:5173"),
		CookieName:      value("PACT_SESSION_COOKIE", "pact_session"),
		CookieSecure:    boolValue("PACT_COOKIE_SECURE", false),
		SessionTTL:      durationValue("PACT_SESSION_TTL", 7*24*time.Hour),
		ChallengeTTL:    durationValue("PACT_CHALLENGE_TTL", 5*time.Minute),
		ReconcileEvery:  durationValue("PACT_RECONCILE_INTERVAL", 15*time.Second),
	}
}

func value(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func csv(key, fallback string) []string {
	values := strings.Split(value(key, fallback), ",")
	result := make([]string, 0, len(values))
	for _, item := range values {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}

func boolValue(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}

func durationValue(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
