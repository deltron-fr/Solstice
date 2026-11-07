package config

import "os"

type Config struct {
	HTTPAddr        string
	DatabaseDSN     string
	SolanaRPCURL    string
	SolanaProgramID string
}

func Load() Config {
	return Config{
		HTTPAddr:        value("PACT_HTTP_ADDR", ":8080"),
		DatabaseDSN:     value("PACT_DATABASE_DSN", "file:pact.db"),
		SolanaRPCURL:    value("PACT_SOLANA_RPC_URL", "https://api.devnet.solana.com"),
		SolanaProgramID: os.Getenv("PACT_SOLANA_PROGRAM_ID"),
	}
}

func value(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
