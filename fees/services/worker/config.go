package worker

import "encore.dev/config"

// Granular config for Temporal.
type TemporalConfig struct {
	Host      config.String
	Namespace config.String
}

type Config struct {
	Temporal TemporalConfig
}
