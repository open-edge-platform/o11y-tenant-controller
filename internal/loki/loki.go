// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package loki

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/open-edge-platform/o11y-tenant-controller/internal/config"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/util"
)

type DeleteLogRequest []struct {
	RequestID string  `json:"request_id"`
	StartTime float64 `json:"start_time"`
	EndTime   float64 `json:"end_time"`
	Query     string  `json:"query"`
	Status    string  `json:"status"`
	CreatedAt float64 `json:"created_at"`
}

func CleanupTenant(ctx context.Context, urlCfg config.Loki) error {
	tenantID, ok := ctx.Value(util.ContextKeyTenantID).(string)
	if !ok {
		return fmt.Errorf("failed to retrieve %q from context", util.ContextKeyTenantID)
	}
	log.Printf("Deleting tenantID %q logs", tenantID)

	if err := flushIngesters(ctx, urlCfg, tenantID); err != nil {
		return fmt.Errorf("failed to flush ingesters for tenantID %q: %w", tenantID, err)
	}

	if err := deleteLogsRequest(ctx, urlCfg, tenantID); err != nil {
		return fmt.Errorf("failed to delete logs for tenantID %q: %w", tenantID, err)
	}

	if err := checkDeletionStatus(ctx, urlCfg, tenantID); err != nil {
		return fmt.Errorf("failed to check deletion status for tenantID %q: %w", tenantID, err)
	}

	log.Printf("TenantID %q logs deleted", tenantID)
	return nil
}

func flushIngesters(ctx context.Context, urlCfg config.Loki, tenantID string) error {
	urlRaw := fmt.Sprintf("%v/flush", urlCfg.Write)
	return util.PostReq(ctx, urlRaw, tenantID)
}

func deleteLogsRequest(ctx context.Context, urlCfg config.Loki, tenantID string) error {
	urlRaw := fmt.Sprintf("%v/loki/api/v1/delete?query={__tenant_id__=\"%v\"}&start=0000000001", urlCfg.Backend, tenantID)
	return util.PostReq(ctx, urlRaw, tenantID)
}

func checkDeletionStatus(ctx context.Context, urlCfg config.Loki, tenantID string) error {
	var deletionLogBody DeleteLogRequest

	cnt := 0
	sleepTime := urlCfg.PollingRate
	urlRaw := fmt.Sprintf("%v/loki/api/v1/delete", urlCfg.Backend)

	log.Printf("Waiting for tenantID %q logs deletion in Loki...", tenantID)
	for {
		if err := util.SleepWithContext(ctx, sleepTime); err != nil {
			return err
		}

		body, err := util.GetReq(ctx, urlRaw, tenantID)
		if err != nil {
			return err
		}

		if err := json.Unmarshal(body, &deletionLogBody); err != nil {
			return fmt.Errorf("failed to unmarshal response body: %w", err)
		}

		if len(deletionLogBody) != 0 {
			newestDelReq := deletionLogBody[len(deletionLogBody)-1]
			if urlCfg.DeleteVerifyMode == util.LooseMode {
				break
			}

			if newestDelReq.Status == "processed" {
				break
			}
			continue
		}

		// In case of empty response from loki, add new deletion request, and try checking again.
		// Every retry have longer waiting period - up to MaxPolling rate.
		log.Printf("Loki: empty deletion status response for tenant %q, retrying...", tenantID)

		err = deleteLogsRequest(ctx, urlCfg, tenantID)
		if err != nil {
			return err
		}

		sleepTime = urlCfg.MaxPollingRate
		calcTime := time.Duration(1<<cnt) * urlCfg.PollingRate
		if calcTime < urlCfg.MaxPollingRate {
			sleepTime = calcTime
			cnt++
		}
	}
	return nil
}
