// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/open-edge-platform/o11y-tenant-controller/internal/projects"
)

type Action int

const (
	InitializeTenant Action = iota
	CleanupTenant
)

type TenantController struct {
	ComSig chan CommChannel
	server *http.Server

	grpcServer projects.Server
}

type CommChannel struct {
	ProjectID   string
	ProjectName string
	OrgName     string
	Status      Action
	Result      chan error // If non-nil, the job sends its result here when done.
}

func New(buffer int, grpcServer *projects.Server) (*TenantController, error) {
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
		ComSig:     make(chan CommChannel, buffer),
		server:     server,
		grpcServer: *grpcServer,
	}, nil
}

func (tc *TenantController) Start() error {
	go func() {
		if err := tc.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Prometheus server error: %v", err)
		}
	}()

	log.Print("Tenant controller starting")
	return nil
}

func (tc *TenantController) Stop() {
	log.Print("Tenant controller stopping")

	stopped := make(chan struct{})
	go func() {
		tc.grpcServer.GrpcServer.GracefulStop()
		close(stopped)
	}()

	dur := 5 * time.Second
	t := time.NewTimer(dur)
	select {
	case <-t.C:
		log.Printf("gRPC server did not stop in %v, stopping forcefully", dur)
		tc.grpcServer.GrpcServer.Stop()
	case <-stopped:
		t.Stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tc.server.Shutdown(ctx); err != nil {
		log.Printf("Prometheus server shutdown error: %v", err)
	}

	close(tc.ComSig)
}

func (a Action) String() string {
	return [...]string{"InitializeTenant", "CleanupTenant"}[a]
}

// HandleProjectEvent is called by the tenancy handler to dispatch a project
// event into the job pipeline and block until the job completes, returning
// any error so the Poller can report accurate status.
func (tc *TenantController) HandleProjectEvent(projectID, projectName, orgName string, action Action) error {
	log.Printf("Project %q event: %s", projectID, action)
	pd := projects.ProjectData{
		ProjectName: projectName,
		OrgID:       orgName,
	}

	if action == CleanupTenant {
		pd.Status = projects.ProjectDeleted
	} else {
		pd.Status = projects.ProjectCreated
	}

	resultCh := make(chan error, 1)
	tc.ComSig <- CommChannel{
		ProjectID:   projectID,
		ProjectName: projectName,
		OrgName:     orgName,
		Status:      action,
		Result:      resultCh,
	}

	tc.grpcServer.Mu.Lock()
	tc.grpcServer.Projects[projectID] = pd
	tc.grpcServer.Mu.Unlock()
	tc.grpcServer.BroadcastUpdate()

	return <-resultCh
}
