// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package sre

import (
	"context"
	"fmt"
	"log"

	proto "github.com/open-edge-platform/o11y-sre-exporter/api/config-reloader"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/open-edge-platform/o11y-tenant-controller/internal/util"
)

func InitializeTenant(ctx context.Context, sre proto.ManagementClient) error {
	tenantID, ok := ctx.Value(utility.ContextKeyTenantID).(string)
	if !ok {
		return fmt.Errorf("failed to retrieve %q from context", utility.ContextKeyTenantID)
	}

	log.Printf("Creating tenantID %q in sre-exporter", tenantID)
	_, err := sre.InitializeTenant(ctx, &proto.TenantRequest{Tenant: tenantID})
	if status.Code(err) == codes.AlreadyExists {
		log.Printf("TenantID %q already initialized in sre-exporter", tenantID)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to initialize tenantID %q in sre-exporter: %w", tenantID, err)
	}

	log.Printf("TenantID %q initialized in sre-exporter", tenantID)
	return nil
}

func CleanupTenant(ctx context.Context, sre proto.ManagementClient) error {
	tenantID, ok := ctx.Value(utility.ContextKeyTenantID).(string)
	if !ok {
		return fmt.Errorf("failed to retrieve %q from context", utility.ContextKeyTenantID)
	}

	log.Printf("Deleting tenantID %q in sre-exporter", tenantID)
	_, err := sre.CleanupTenant(ctx, &proto.TenantRequest{Tenant: tenantID})
	if status.Code(err) == codes.NotFound {
		log.Printf("TenantID %q already deleted in sre-exporter", tenantID)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to delete tenantID %q in sre-exporter: %w", tenantID, err)
	}

	log.Printf("TenantID %q deleted in sre-exporter", tenantID)
	return nil
}
