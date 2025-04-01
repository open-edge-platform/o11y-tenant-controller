// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	projectwatcherv1 "github.com/open-edge-platform/orch-utils/tenancy-datamodel/build/apis/projectwatcher.edge-orchestrator.intel.com/v1"
	nexus "github.com/open-edge-platform/orch-utils/tenancy-datamodel/build/nexus-client"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"github.com/open-edge-platform/o11y-tenant-controller/internal/projects"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/util"
)

type Action int

const (
	InitializeTenant Action = iota
	CleanupTenant
)

type TenantController struct {
	ComSig         chan CommChannel
	client         *nexus.Clientset
	server         *http.Server
	watcherTimeout time.Duration

	grpcServer projects.Server
}

type CommChannel struct {
	Project *nexus.RuntimeprojectRuntimeProject
	Status  Action
}

func New(buffer int, watcherTimeout time.Duration, grpcServer *projects.Server) (*TenantController, error) {
	c, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to read kubernetes service account token: %w", err)
	}

	client, err := nexus.NewForConfig(c)
	if err != nil {
		return nil, fmt.Errorf("failed to open communication with the kubernetes server: %w", err)
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
		ComSig:         make(chan CommChannel, buffer),
		client:         client,
		server:         server,
		watcherTimeout: watcherTimeout,
		grpcServer:     *grpcServer,
	}, nil
}

func (tc *TenantController) Start() error {
	if err := tc.addProjectWatcher(); err != nil {
		return fmt.Errorf("failed to create project watcher: %w", err)
	}

	if _, err := tc.client.TenancyMultiTenancy().Runtime().Orgs("*").Folders("*").Projects("*").RegisterAddCallback(tc.addHandler); err != nil {
		return fmt.Errorf("unable to register project creation callback: %w", err)
	}

	if _, err := tc.client.TenancyMultiTenancy().Runtime().Orgs("*").Folders("*").Projects("*").RegisterUpdateCallback(tc.updateHandler); err != nil {
		return fmt.Errorf("unable to register project update callback: %w", err)
	}

	// Callback for project watcher deletion is safeguard for unintended project watcher deletion eg. during tenant controller update.
	if _, err := tc.client.TenancyMultiTenancy().Config().ProjectWatchers(util.AppName).RegisterDeleteCallback(tc.projectWatcherDeleteHandler); err != nil {
		return fmt.Errorf("unable to register project watcher delete callback: %w", err)
	}

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
	tc.client.UnsubscribeAll()

	if err := tc.deleteProjectWatcher(); err != nil {
		log.Printf("Failed to delete watcher: %v", err)
	}
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

// Callback for project watcher deletion is safeguard for unintended project watcher deletion eg. during tenant controller update.
func (tc *TenantController) projectWatcherDeleteHandler(_ *nexus.ProjectwatcherProjectWatcher) {
	err := tc.addProjectWatcher()
	if err != nil {
		log.Print(err)
	}
}

func (tc *TenantController) addProjectWatcher() error {
	ctx, cancel := context.WithTimeout(context.Background(), tc.watcherTimeout)
	defer cancel()

	_, err := tc.client.TenancyMultiTenancy().Config().AddProjectWatchers(ctx, &projectwatcherv1.ProjectWatcher{ObjectMeta: metav1.ObjectMeta{
		Name: util.AppName,
	}})

	if nexus.IsAlreadyExists(err) {
		log.Print("Project watcher already exists")
	} else if err != nil {
		return err
	}
	return nil
}

func (tc *TenantController) deleteProjectWatcher() error {
	ctx, cancel := context.WithTimeout(context.Background(), tc.watcherTimeout)
	defer cancel()

	err := tc.client.TenancyMultiTenancy().Config().DeleteProjectWatchers(ctx, util.AppName)

	if nexus.IsChildNotFound(err) {
		log.Print("Project watcher already deleted")
	} else if err != nil {
		return err
	}
	return nil
}

func (tc *TenantController) addHandler(project *nexus.RuntimeprojectRuntimeProject) {
	log.Printf("Project %q added", project.ObjectMeta.UID)
	pd := projects.ProjectData{
		ProjectName: project.DisplayName(),
		OrgID:       project.GetLabels()[util.OrgNameLabel],
	}

	if project.Spec.Deleted {
		tc.ComSig <- CommChannel{project, CleanupTenant}
		pd.Status = projects.ProjectDeleted
	} else {
		tc.ComSig <- CommChannel{project, InitializeTenant}
		pd.Status = projects.ProjectCreated
	}

	tc.grpcServer.Mu.Lock()
	tc.grpcServer.Projects[string(project.ObjectMeta.UID)] = pd
	tc.grpcServer.Mu.Unlock()
	tc.grpcServer.BroadcastUpdate()
}

func (tc *TenantController) updateHandler(_, project *nexus.RuntimeprojectRuntimeProject) {
	log.Printf("Project %q updated", project.ObjectMeta.UID)
	pd := projects.ProjectData{
		ProjectName: project.DisplayName(),
		OrgID:       project.GetLabels()[util.OrgNameLabel],
	}

	if project.Spec.Deleted {
		tc.ComSig <- CommChannel{project, CleanupTenant}
		pd.Status = projects.ProjectDeleted
	} else {
		tc.ComSig <- CommChannel{project, InitializeTenant}
		pd.Status = projects.ProjectCreated
	}

	tc.grpcServer.Mu.Lock()
	tc.grpcServer.Projects[string(project.ObjectMeta.UID)] = pd
	tc.grpcServer.Mu.Unlock()
	tc.grpcServer.BroadcastUpdate()
}
