// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type VerifyMode string

const (
	StrictMode VerifyMode = "strict"
	LooseMode  VerifyMode = "loose"
)

type contextKey string

const (
	ContextKeyTenantID contextKey = "ContextKeyTenantID"
	AppName            string     = "observability-tenant-controller"
	OrgNameLabel       string     = "runtimeorgs.runtimeorg.edge-orchestrator.intel.com"
)

func SleepWithContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// When context is canceled fuction returns an error.
func PostReq(ctx context.Context, urlRaw string, tenantID string) error {
	u, err := url.Parse(urlRaw)
	if err != nil {
		return fmt.Errorf("failed to parse url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Scope-OrgID", tenantID)

	client := http.DefaultClient
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach endpoint %v: %w", urlRaw, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("invalid response status code '%v' for endpoint: %v", res.StatusCode, urlRaw)
	}
	return nil
}

// When context is canceled fuction returns an error.
func GetReq(ctx context.Context, urlRaw string, tenantID string) ([]byte, error) {
	u, err := url.Parse(urlRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Scope-OrgID", tenantID)

	client := http.DefaultClient
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach endpoint %v: %w", urlRaw, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		return nil, fmt.Errorf("invalid response status code '%v' for endpoint: %v", res.StatusCode, urlRaw)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}
