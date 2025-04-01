// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package alertingmonitor

import (
	"context"
	"fmt"
	"log"

	proto "github.com/open-edge-platform/o11y-alerting-monitor/api/v1/management"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/open-edge-platform/o11y-tenant-controller/internal/util"
)

func InitializeTenant(ctx context.Context, am proto.ManagementClient) error {
	tenantID, ok := ctx.Value(util.ContextKeyTenantID).(string)
	if !ok {
		return fmt.Errorf("failed to retrieve %q from context", util.ContextKeyTenantID)
	}

	log.Printf("Creating tenantID %q in alerting monitor", tenantID)
	_, err := am.InitializeTenant(ctx, &proto.TenantRequest{Tenant: tenantID})
	if status.Code(err) == codes.AlreadyExists {
		log.Printf("TenantID %q already initialized in alerting monitor", tenantID)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to initialize tenantID %q in alerting monitor: %w", tenantID, err)
	}

	log.Printf("TenantID %q initialized in alerting monitor", tenantID)
	return nil
}

func CleanupTenant(ctx context.Context, am proto.ManagementClient) error {
	tenantID, ok := ctx.Value(util.ContextKeyTenantID).(string)
	if !ok {
		return fmt.Errorf("failed to retrieve %q from context", util.ContextKeyTenantID)
	}

	log.Printf("Deleting tenantID %q in alerting monitor", tenantID)
	_, err := am.CleanupTenant(ctx, &proto.TenantRequest{Tenant: tenantID})
	if status.Code(err) == codes.NotFound {
		log.Printf("TenantID %q already deleted in alerting monitor", tenantID)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to delete tenantID %q in alerting monitor: %w", tenantID, err)
	}

	log.Printf("TenantID %q deleted in alerting monitor", tenantID)
	return nil
}
