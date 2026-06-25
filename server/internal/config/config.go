package config

import "os"

// Config — конфигурация cmd/api. Секреты/настройки только из окружения (вне репо).
// cmd/api НЕ читает GOSZAKUP_TOKEN (изоляция: токен только у importer).
type Config struct {
	DatabaseURL  string // postgres DSN (pgx)
	Addr         string // адрес прослушивания HTTP
	RegistryRoot string // корень registry (methodology_params.vN.yaml читается в рантайме для seed/инварианта)
}

func Load() Config {
	return Config{
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		Addr:         envOr("API_ADDR", ":8080"),
		RegistryRoot: envOr("REGISTRY_ROOT", "registry"),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
