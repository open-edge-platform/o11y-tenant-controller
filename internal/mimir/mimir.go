// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package mimir

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/open-edge-platform/o11y-tenant-controller/internal/config"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/util"
)

type deleteStatus struct {
	TenantID      string `json:"tenant_id"`
	BlocksDeleted bool   `json:"blocks_deleted"`
}

func CleanupTenant(ctx context.Context, urlCfg config.Mimir) error {
	tenantID, ok := ctx.Value(utility.ContextKeyTenantID).(string)
	if !ok {
		return fmt.Errorf("failed to retrieve %q from context", utility.ContextKeyTenantID)
	}
	log.Printf("Deleting tenantID %q metrics", tenantID)

	err := flushIngesters(ctx, urlCfg, tenantID)
	if err != nil {
		return fmt.Errorf("failed to flush ingesters for tenantID %q: %w", tenantID, err)
	}

	err = deleteMetricsRequest(ctx, urlCfg, tenantID)
	if err != nil {
		return fmt.Errorf("failed to delete metrics for tenantID %q: %w", tenantID, err)
	}

	if urlCfg.DeleteVerifyMode == utility.StrictMode {
		err = checkDeletionStatus(ctx, urlCfg, tenantID)
		if err != nil {
			return fmt.Errorf("failed to check deletion status for tenantID %q: %w", tenantID, err)
		}
	}

	log.Printf("TenantID %q metrics deleted", tenantID)
	return nil
}

func flushIngesters(ctx context.Context, urlCfg config.Mimir, tenantID string) error {
	urlRaw := fmt.Sprintf("%v/ingester/flush?wait=true", urlCfg.Ingester)
	_, err := utility.GetReq(ctx, urlRaw, tenantID)
	return err
}

func deleteMetricsRequest(ctx context.Context, urlCfg config.Mimir, tenantID string) error {
	urlRaw := fmt.Sprintf("%v/compactor/delete_tenant", urlCfg.Compactor)
	return utility.PostReq(ctx, urlRaw, tenantID)
}

func checkDeletionStatus(ctx context.Context, urlCfg config.Mimir, tenantID string) error {
	var deletionStatusBody deleteStatus
	urlRaw := fmt.Sprintf("%v/compactor/delete_tenant_status", urlCfg.Compactor)

	for {
		body, err := utility.GetReq(ctx, urlRaw, tenantID)
		if err != nil {
			return err
		}

		if err := json.Unmarshal(body, &deletionStatusBody); err != nil {
			return fmt.Errorf("failed to unmarshal response body: %w", err)
		}

		if deletionStatusBody.BlocksDeleted {
			break
		}

		if err := utility.SleepWithContext(ctx, urlCfg.PollingRate); err != nil {
			return err
		}
	}
	return nil
}
