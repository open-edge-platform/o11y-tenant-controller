// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func ReadConfig(path string) (Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read file %q: %w", path, err)
	}

	var cfg Config
	if err = yaml.Unmarshal(file, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal: %w", err)
	}

	if err = cfg.validate(); err != nil {
		return Config{}, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

func (c *Config) validate() error {
	var errs []error

	if c.Controller.Channel.MaxInflightRequests <= 0 {
		errs = append(errs, fmt.Errorf("controller.channel.maxInflightRequests must be > 0 (got %d)", c.Controller.Channel.MaxInflightRequests))
	}
	if c.Job.Manager.Deletion.Rate <= 0 {
		errs = append(errs, errors.New("job.manager.deletion.rate must be > 0"))
	}
	if c.Job.Timeout <= 0 {
		errs = append(errs, errors.New("job.timeout must be > 0"))
	}
	if c.Job.Backoff.Initial <= 0 {
		errs = append(errs, errors.New("job.backoff.initial must be > 0"))
	}
	if c.Job.Backoff.Max <= 0 {
		errs = append(errs, errors.New("job.backoff.max must be > 0"))
	}
	if c.Endpoints.AlertingMonitor == "" {
		errs = append(errs, errors.New("endpoints.alertingmonitor must not be empty"))
	}
	if c.Endpoints.Loki.Write == "" {
		errs = append(errs, errors.New("endpoints.loki.write must not be empty"))
	}
	if c.Endpoints.Mimir.Ingester == "" {
		errs = append(errs, errors.New("endpoints.mimir.ingester must not be empty"))
	}

	return errors.Join(errs...)
}
