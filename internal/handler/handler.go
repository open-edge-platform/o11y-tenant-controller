// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"log"

	"github.com/open-edge-platform/orch-library/go/pkg/tenancy"

	"github.com/open-edge-platform/o11y-tenant-controller/internal/controller"
)

// TenancyHandler implements tenancy.Handler, bridging tenancy events into the
// existing controller pipeline.
type TenancyHandler struct {
	Controller *controller.TenantController
}

func (h *TenancyHandler) HandleEvent(_ context.Context, event tenancy.Event) error {
	if event.ResourceType != "project" {
		return nil // observability controller only handles project events
	}

	orgName := ""
	if event.OrgName != nil {
		orgName = *event.OrgName
	}

	projectID := event.ResourceID.String()

	switch event.EventType {
	case "created":
		log.Printf("Received project created event for %q (project %q)", projectID, event.ResourceName)
		return h.Controller.HandleProjectEvent(projectID, event.ResourceName, orgName, controller.InitializeTenant)
	case "deleted":
		log.Printf("Received project deleted event for %q (project %q)", projectID, event.ResourceName)
		return h.Controller.HandleProjectEvent(projectID, event.ResourceName, orgName, controller.CleanupTenant)
	default:
		log.Printf("Ignoring unknown event type %q for project %q", event.EventType, projectID)
		return nil
	}
}
