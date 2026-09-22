package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port                    string
	Storage                 string
	DBPath                  string
	PreStocksAPIURL         string
	JupiterBaseURL          string
	JupiterAPIKey           string
	JupiterImpactIsFraction bool
	SolanaRPCURL            string
	USDCMint                string
	ScanInterval            time.Duration
	AdminToken              string
	TelegramBotToken        string
}

func Load() (Config, error) {
	cfg := Config{
		Port:                    getEnv("PORT", "8080"),
		Storage:                 getEnv("STORAGE", "sqlite"),
		DBPath:                  getEnv("DB_PATH", "premark.db"),
		PreStocksAPIURL:         getEnv("PRESTOCKS_API_URL", "https://prestocks.com/api/prestocks"),
		JupiterBaseURL:          getEnv("JUPITER_BASE_URL", "https://lite-api.jup.ag/swap/v1"),
		JupiterAPIKey:           getEnv("JUPITER_API_KEY", ""),
		JupiterImpactIsFraction: true,
		SolanaRPCURL:            getEnv("SOLANA_RPC_URL", "https://api.mainnet-beta.solana.com"),
		USDCMint:                getEnv("USDC_MINT", "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"),
		ScanInterval:            60 * time.Second,
		AdminToken:              getEnv("ADMIN_TOKEN", ""),
		TelegramBotToken:        getEnv("TELEGRAM_BOT_TOKEN", ""),
	}

	if p, err := strconv.Atoi(cfg.Port); err != nil || p < 1 || p > 65535 {
		return Config{}, fmt.Errorf("invalid PORT %q: must be between 1 and 65535", cfg.Port)
	}

	if cfg.Storage != "sqlite" && cfg.Storage != "memory" {
		return Config{}, fmt.Errorf("invalid STORAGE %q: must be 'sqlite' or 'memory'", cfg.Storage)
	}

	if rawImpact := os.Getenv("JUPITER_IMPACT_IS_FRACTION"); rawImpact != "" {
		val, err := strconv.ParseBool(strings.TrimSpace(rawImpact))
		if err != nil {
			return Config{}, fmt.Errorf("invalid JUPITER_IMPACT_IS_FRACTION %q: %w", rawImpact, err)
		}
		cfg.JupiterImpactIsFraction = val
	}

	if rawInterval := os.Getenv("SCAN_INTERVAL"); rawInterval != "" {
		d, err := time.ParseDuration(strings.TrimSpace(rawInterval))
		if err != nil {
			return Config{}, fmt.Errorf("invalid SCAN_INTERVAL %q: %w", rawInterval, err)
		}
		if d < 5*time.Second {
			return Config{}, fmt.Errorf("invalid SCAN_INTERVAL %v: must be >= 5s", d)
		}
		cfg.ScanInterval = d
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return strings.TrimSpace(val)
	}
	return defaultVal
}
