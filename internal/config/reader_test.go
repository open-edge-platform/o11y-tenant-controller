// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-edge-platform/o11y-tenant-controller/internal/util"
)

func TestReadConfig(t *testing.T) {
	t.Run("Valid config file", func(t *testing.T) {
		configFile, err := ReadConfig("testdata/test_config.yaml")
		require.NoError(t, err)
		require.Equal(t, 20, configFile.Controller.Channel.MaxInflightRequests, "Config value different from expected")
		require.Equal(t, 30*time.Minute, configFile.Job.Timeout, "Config value different from expected")
		require.Equal(t, time.Minute, configFile.Job.Manager.Deletion.Rate, "Config value different from expected")
		require.Equal(t, 10*time.Second, configFile.Job.Backoff.Initial, "Config value different from expected")
		require.Equal(t, 10*time.Minute, configFile.Job.Backoff.Max, "Config value different from expected")
		require.InEpsilon(t, 1.6, configFile.Job.Backoff.TimeMultiplier, 0, "Config value different from expected")
		require.Equal(t, "http://localhost:3100", configFile.Endpoints.Loki.Write, "Config value different from expected")
		require.Equal(t, "http://localhost:3100", configFile.Endpoints.Loki.Backend, "Config value different from expected")
		require.Equal(t, 20*time.Second, configFile.Endpoints.Loki.PollingRate, "Config value different from expected")
		require.Equal(t, time.Minute, configFile.Endpoints.Loki.MaxPollingRate, "Config value different from expected")
		require.Equal(t, util.LooseMode, configFile.Endpoints.Loki.DeleteVerifyMode, "Config value different from expected")
		require.Equal(t, "http://localhost:8080", configFile.Endpoints.Mimir.Compactor, "Config value different from expected")
		require.Equal(t, "http://localhost:8080", configFile.Endpoints.Mimir.Ingester, "Config value different from expected")
		require.Equal(t, 20*time.Second, configFile.Endpoints.Mimir.PollingRate, "Config value different from expected")
		require.Equal(t, util.LooseMode, configFile.Endpoints.Mimir.DeleteVerifyMode, "Config value different from expected")
		require.Equal(t, "http://localhost:8080", configFile.Endpoints.AlertingMonitor, "Config value different from expected")
		require.Equal(t, 10*time.Minute, configFile.Controller.CreateDeleteWatcherTimeout, "Config value different from expected")
		require.Equal(t, "http://localhost:8080", configFile.Endpoints.Sre, "Config value different from expected")
		require.True(t, configFile.Job.Sre.Enabled, "Config value different from expected")
	})
	t.Run("Invalid config file name", func(t *testing.T) {
		_, err := ReadConfig("testdata/invalid_file_name.yaml")
		require.Error(t, err)
	})
	t.Run("Invalid config file", func(t *testing.T) {
		_, err := ReadConfig("testdata/test_config_malformed.yaml")
		require.Error(t, err)
	})
}
