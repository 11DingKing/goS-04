package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Config holds all tunable parameters for the patrol platform.
type Config struct {
	HTTPAddr                string        `json:"-"`
	TrackInterval           time.Duration `json:"-"`
	ClockInMaxGap           time.Duration `json:"-"`
	Level1Timeout           time.Duration `json:"-"`
	ClaimTimeout            time.Duration `json:"-"`
	AcceptTimeout           time.Duration `json:"-"`
	EscalationCheckInterval time.Duration `json:"-"`
	TimeoutCheckInterval    time.Duration `json:"-"`
}

type rawConfig struct {
	HTTPAddr                string `json:"http_addr"`
	TrackInterval           string `json:"track_interval"`
	ClockInMaxGap           string `json:"clock_in_max_gap"`
	Level1Timeout           string `json:"level1_timeout"`
	ClaimTimeout            string `json:"claim_timeout"`
	AcceptTimeout           string `json:"accept_timeout"`
	EscalationCheckInterval string `json:"escalation_check_interval"`
	TimeoutCheckInterval    string `json:"timeout_check_interval"`
}

// Default returns production-ready default configuration.
func Default() *Config {
	return &Config{
		HTTPAddr:                ":58839",
		TrackInterval:           15 * time.Minute,
		ClockInMaxGap:           6 * time.Hour,
		Level1Timeout:           2 * time.Hour,
		ClaimTimeout:            30 * time.Minute,
		AcceptTimeout:           30 * time.Minute,
		EscalationCheckInterval: time.Minute,
		TimeoutCheckInterval:    time.Minute,
	}
}

// Load reads a JSON configuration file, falling back to defaults for
// missing or unparseable fields. If the file does not exist, defaults are used.
func Load(path string) (*Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var raw rawConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if raw.HTTPAddr != "" {
		cfg.HTTPAddr = raw.HTTPAddr
	}
	overrides := []struct {
		src string
		dst *time.Duration
	}{
		{raw.TrackInterval, &cfg.TrackInterval},
		{raw.ClockInMaxGap, &cfg.ClockInMaxGap},
		{raw.Level1Timeout, &cfg.Level1Timeout},
		{raw.ClaimTimeout, &cfg.ClaimTimeout},
		{raw.AcceptTimeout, &cfg.AcceptTimeout},
		{raw.EscalationCheckInterval, &cfg.EscalationCheckInterval},
		{raw.TimeoutCheckInterval, &cfg.TimeoutCheckInterval},
	}
	for _, o := range overrides {
		if o.src == "" {
			continue
		}
		d, err := time.ParseDuration(o.src)
		if err != nil {
			return nil, fmt.Errorf("invalid duration %q: %w", o.src, err)
		}
		if d <= 0 {
			return nil, fmt.Errorf("duration %q must be positive", o.src)
		}
		*o.dst = d
	}
	return cfg, nil
}
