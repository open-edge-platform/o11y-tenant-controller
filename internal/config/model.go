// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"time"

	"github.com/open-edge-platform/o11y-tenant-controller/internal/util"
)

type Config struct {
	Endpoints  Endpoints `yaml:"endpoints"`
	Controller struct {
		Channel struct {
			MaxInflightRequests int `yaml:"maxInflightRequests"`
		} `yaml:"channel"`
		CreateDeleteWatcherTimeout time.Duration `yaml:"createDeleteWatcherTimeout"`
	} `yaml:"controller"`
	Job Job `yaml:"job"`
}

type Job struct {
	Manager struct {
		Deletion struct {
			Rate time.Duration `yaml:"rate"`
		} `yaml:"deletion"`
	} `yaml:"manager"`
	Backoff struct {
		Initial        time.Duration `yaml:"initial"`
		Max            time.Duration `yaml:"max"`
		TimeMultiplier float64       `yaml:"timeMultiplier"`
	} `yaml:"backoff"`
	Timeout time.Duration `yaml:"timeout"`
	Sre     struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"sre"`
}

type Mimir struct {
	Ingester         string             `yaml:"ingester"`
	Compactor        string             `yaml:"compactor"`
	PollingRate      time.Duration      `yaml:"pollingRate"`
	DeleteVerifyMode utility.VerifyMode `yaml:"deleteVerifyMode"`
}

type Loki struct {
	Write            string             `yaml:"write"`
	Backend          string             `yaml:"backend"`
	PollingRate      time.Duration      `yaml:"pollingRate"`
	MaxPollingRate   time.Duration      `yaml:"maxPollingRate"`
	DeleteVerifyMode utility.VerifyMode `yaml:"deleteVerifyMode"`
}

type Endpoints struct {
	AlertingMonitor string `yaml:"alertingmonitor"`
	Sre             string `yaml:"sre"`
	Mimir           Mimir  `yaml:"mimir"`
	Loki            Loki   `yaml:"loki"`
}
