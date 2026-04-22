// SPDX-FileCopyrightText: (C) 2026 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/open-edge-platform/orch-library/go/pkg/tenancy"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/open-edge-platform/o11y-tenant-controller/internal/projects"
	utility "github.com/open-edge-platform/o11y-tenant-controller/internal/util"
)

const defaultTenantManagerURL = "http://tenancy-manager.orch-iam:8080"

type Action int

const (
	InitializeTenant Action = iota
	CleanupTenant
)

// CommChannel carries a tenancy event and the derived action to the JobManager.
type CommChannel struct {
	ProjectID   string
	ProjectName string
	OrgName     string
	Status      Action
}

// TenantController listens for tenancy events and dispatches jobs to the
// JobManager via the ComSig channel.
type TenantController struct {
	ComSig   chan CommChannel
	server   *http.Server
	ctx      context.Context
	cancel   context.CancelFunc
	stopOnce sync.Once
	startMu  sync.Mutex
	started  bool
}

func New(buffer int, grpcServer *projects.Server) (*TenantController, error) {
	if grpcServer == nil {
		return nil, fmt.Errorf("grpcServer must not be nil")
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:         ":9273",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	return &TenantController{
		ComSig: make(chan CommChannel, buffer),
		server: server,
	}, nil
}

// Start launches the tenancy event poller and the Prometheus metrics server.
// It must be called exactly once; subsequent calls return an error.
func (tc *TenantController) Start(grpcServer *projects.Server) error {
	tc.startMu.Lock()
	if tc.started {
		tc.startMu.Unlock()
		return fmt.Errorf("TenantController already started")
	}
	tc.started = true
	tc.startMu.Unlock()

	tenantManagerURL := os.Getenv("TENANT_MANAGER_URL")
	if tenantManagerURL == "" {
		tenantManagerURL = defaultTenantManagerURL
	}

	ctx, cancel := context.WithCancel(context.Background())
	tc.ctx = ctx
	tc.cancel = cancel

	handler := &eventHandler{comSig: tc.ComSig, ctx: ctx, grpcServer: grpcServer}

	poller, err := tenancy.NewPoller(tenantManagerURL, utility.AppName, handler,
		func(cfg *tenancy.PollerConfig) {
			cfg.OnError = func(err error, msg string) {
				log.Printf("tenancy poller error: %s: %v", msg, err)
			}
		},
	)
	if err != nil {
		return fmt.Errorf("create tenancy poller: %w", err)
	}

	go func() {
		if err := poller.Run(tc.ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("tenancy poller stopped unexpectedly: %v", err)
		}
	}()

	go func() {
		if err := tc.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Prometheus server error: %v", err)
		}
	}()

	log.Printf("Tenant controller started (controller=%s url=%s)", utility.AppName, tenantManagerURL)
	return nil
}

// Stop shuts down the poller and Prometheus server. Safe to call multiple times.
func (tc *TenantController) Stop() {
	tc.stopOnce.Do(func() {
		log.Print("Tenant controller stopping")

		if tc.cancel != nil {
			tc.cancel()
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tc.server.Shutdown(ctx); err != nil {
			log.Printf("Prometheus server shutdown error: %v", err)
		}
	})
}

func (a Action) String() string {
	switch a {
	case InitializeTenant:
		return "InitializeTenant"
	case CleanupTenant:
		return "CleanupTenant"
	default:
		return fmt.Sprintf("Action(%d)", int(a))
	}
}

// eventHandler implements tenancy.Handler and routes project events to the job
// channel. It only handles project events — org events are silently ignored.
type eventHandler struct {
	comSig     chan CommChannel
	ctx        context.Context
	grpcServer *projects.Server
}

func (h *eventHandler) HandleEvent(_ context.Context, event tenancy.Event) error {
	if event.ResourceType != tenancy.ResourceTypeProject {
		return nil
	}

	projectID := event.ResourceID.String()
	projectName := event.ResourceName
	orgName := ""
	if event.OrgName != nil {
		orgName = *event.OrgName
	}

	var action Action
	switch event.EventType {
	case tenancy.EventTypeCreated:
		action = InitializeTenant
	case tenancy.EventTypeDeleted:
		action = CleanupTenant
	default:
		log.Printf("ignoring unrecognised event type %q for project %q", event.EventType, projectID)
		return nil
	}

	log.Printf("Project %q %s", projectID, event.EventType)

	pd := projects.ProjectData{
		ProjectName: projectName,
		OrgID:       orgName,
	}
	if action == InitializeTenant {
		pd.Status = projects.ProjectCreated
	} else {
		pd.Status = projects.ProjectDeleted
	}
	h.grpcServer.Mu.Lock()
	h.grpcServer.Projects[projectID] = pd
	h.grpcServer.Mu.Unlock()
	h.grpcServer.BroadcastUpdate()

	select {
	case h.comSig <- CommChannel{
		ProjectID:   projectID,
		ProjectName: projectName,
		OrgName:     orgName,
		Status:      action,
	}:
	case <-h.ctx.Done():
		return h.ctx.Err()
	}
	return nil
}

