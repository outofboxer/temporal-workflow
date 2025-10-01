package config

import "encore.dev/config"

// // Granular config for the database, just for sample. We don't use DB in this project.
type DBConfig struct {
	MaxOpenConns config.Int
	MaxIdleConns config.Int
}

// Granular config for Temporal.
type TemporalConfig struct {
	Host      config.String
	Namespace config.String
	UseTLS    config.Bool
	UseAPIKey config.Bool
}

type Config struct {
	DB       DBConfig
	Temporal TemporalConfig
}
