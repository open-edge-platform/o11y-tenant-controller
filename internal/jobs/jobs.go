// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"sync/atomic"
	"time"

	amproto "github.com/open-edge-platform/o11y-alerting-monitor/api/v1/management"
	sreproto "github.com/open-edge-platform/o11y-sre-exporter/api/config-reloader"
	projectwatchv1 "github.com/open-edge-platform/orch-utils/tenancy-datamodel/build/apis/projectactivewatcher.edge-orchestrator.intel.com/v1"
	nexus "github.com/open-edge-platform/orch-utils/tenancy-datamodel/build/nexus-client"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-edge-platform/o11y-tenant-controller/internal/alertingmonitor"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/config"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/controller"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/loki"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/mimir"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/projects"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/sre"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/util"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/watcher"
)

type jobStatus int

var projectIDs = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "project_metadata",
		Help: "Exposes project metadata",
	}, []string{"projectId", "projectName", "orgName", "status"},
)

const (
	jobCreated jobStatus = iota
	jobInProgress
	jobCancelled
	tenantCreated
	tenantDeleted
	tenantIDsNotMatch
)

type JobManager struct {
	comSig       chan controller.CommChannel
	jobList      map[types.UID]*job
	jobCfg       config.Job
	endpointsCfg config.Endpoints
	cancelFn     context.CancelFunc
	done         chan struct{}

	amClient  amproto.ManagementClient
	sreClient sreproto.ManagementClient
}

type job struct {
	project      *nexus.RuntimeprojectRuntimeProject
	status       atomic.Int32
	jobCfg       config.Job
	endpointsCfg config.Endpoints
	cancelFn     context.CancelFunc

	amClient  amproto.ManagementClient
	sreClient sreproto.ManagementClient
}

func New(channel chan controller.CommChannel, jCfg config.Job, endpoints config.Endpoints, amConn, sreConn *grpc.ClientConn) *JobManager {
	return &JobManager{
		comSig:       channel,
		jobList:      map[types.UID]*job{},
		jobCfg:       jCfg,
		endpointsCfg: endpoints,
		done:         make(chan struct{}),
		amClient:     amproto.NewManagementClient(amConn),
		sreClient:    sreproto.NewManagementClient(sreConn),
	}
}

func (jm *JobManager) Start(ticker *time.Ticker) {
	ctx, cancel := context.WithCancel(context.Background())
	jm.cancelFn = cancel
	go func() {
		for {
			select {
			case <-jm.done:
				return
			case v := <-jm.comSig:
				switch v.Status {
				case controller.InitializeTenant:
					jm.startJob(ctx, v.Project, controller.InitializeTenant)
				case controller.CleanupTenant:
					jm.startJob(ctx, v.Project, controller.CleanupTenant)
				}
			case <-ticker.C:
				for k, job := range jm.jobList {
					status := jobStatus(job.status.Load())
					//nolint:staticcheck // Linter suggests: "QF1003: could use tagged switch on status"
					// If this suggestion were to be applied, then exhaustive linter would be unhappy
					// that there are missing cases in switch of iota type jobs.jobStatus
					if status == tenantDeleted {
						removeProjectMetadata(job.project)
						delete(jm.jobList, k)
					} else if status == tenantIDsNotMatch {
						removeProjectMetadataByID(string(k))
						delete(jm.jobList, k)
					}
				}
			}
		}
	}()
}

func (jm *JobManager) Stop() {
	if jm.cancelFn != nil {
		jm.cancelFn()
	}
	close(jm.done)
}

func (jm *JobManager) startJob(ctx context.Context, project *nexus.RuntimeprojectRuntimeProject, action controller.Action) {
	setProjectMetadata(project, action)
	job, exists := jm.jobList[project.UID]
	if exists {
		job.cancel()
		job.run(ctx, action)
	} else {
		job = newJob(project, jm.jobCfg, jm.endpointsCfg, jm.amClient, jm.sreClient)
		jm.jobList[project.UID] = job
		job.run(ctx, action)
	}
}

func newJob(project *nexus.RuntimeprojectRuntimeProject, jCfg config.Job,
	endpoints config.Endpoints, am amproto.ManagementClient, sreCl sreproto.ManagementClient) *job {
	return &job{
		project:      project,
		jobCfg:       jCfg,
		endpointsCfg: endpoints,
		amClient:     am,
		sreClient:    sreCl,
	}
}

func (j *job) run(parentCtx context.Context, action controller.Action) {
	j.status.Store(int32(jobInProgress))

	go func() {
		ctx, cancel := context.WithCancel(parentCtx)
		j.cancelFn = cancel
		defer cancel()

		switch action {
		case controller.InitializeTenant:
			j.manageTenant(ctx, j.initializeTenant, controller.InitializeTenant)
			if errors.Is(ctx.Err(), context.Canceled) {
				j.status.Store(int32(jobCancelled))
				return
			}
			j.status.Store(int32(tenantCreated))
		case controller.CleanupTenant:
			j.manageTenant(ctx, j.cleanupTenant, controller.CleanupTenant)
			if errors.Is(ctx.Err(), context.Canceled) {
				j.status.Store(int32(jobCancelled))
				return
			}
			if jobStatus(j.status.Load()) != tenantIDsNotMatch {
				j.status.Store(int32(tenantDeleted))
			}
		}
	}()
}

func (j *job) cancel() {
	if j.cancelFn != nil {
		j.cancelFn()
	}
}

func (j *job) manageTenant(parentCtx context.Context, tenantAction func(context.Context) error, action controller.Action) {
	cnt := 0
	id := j.project.UID
	ctx := context.WithValue(parentCtx, utility.ContextKeyTenantID, string(id))

	for {
		err := tenantAction(ctx)
		if err == nil {
			log.Printf("%v action for tenantID %q completed successfully", action.String(), id)
			break
		}

		if errors.As(err, &watcher.IDsDoNotMatchError{}) {
			log.Printf("%v action for tenantID %q completed successfully - watcher deleted manually", action.String(), id)
			j.status.Store(int32(tenantIDsNotMatch))
			break
		}

		log.Printf("Failed to %s: %v", action.String(), err)

		sleepTime := j.jobCfg.Backoff.Max

		calcTime := math.Pow(j.jobCfg.Backoff.TimeMultiplier, float64(cnt)) * float64(j.jobCfg.Backoff.Initial)
		if calcTime < float64(j.jobCfg.Backoff.Max) {
			sleepTime = time.Duration(calcTime)
			cnt++
		}

		err = utility.SleepWithContext(ctx, sleepTime)
		if errors.Is(err, context.Canceled) {
			log.Printf("%v action for tenantID %q cancelled", action.String(), id)
			break
		}
	}
}

func (j *job) initializeTenant(parentCtx context.Context) error {
	timedOutCtx, cancel := context.WithTimeout(parentCtx, j.jobCfg.Timeout)
	defer cancel()
	err := watcher.CreateUpdateWatcher(parentCtx, j.project,
		projectwatchv1.StatusIndicationInProgress, fmt.Sprintf("Creating tenant %q", j.project.UID))
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(timedOutCtx)

	g.Go(func() error { return alertingmonitor.InitializeTenant(ctx, j.amClient) })
	if j.jobCfg.Sre.Enabled {
		g.Go(func() error { return sre.InitializeTenant(ctx, j.sreClient) })
	}

	if err := g.Wait(); err != nil {
		return err
	}

	return watcher.CreateUpdateWatcher(parentCtx, j.project,
		projectwatchv1.StatusIndicationIdle, fmt.Sprintf("Tenant %q created", j.project.UID))
}

func (j *job) cleanupTenant(parentCtx context.Context) error {
	timedOutCtx, cancel := context.WithTimeout(parentCtx, j.jobCfg.Timeout)
	defer cancel()
	err := watcher.CreateUpdateWatcher(parentCtx, j.project,
		projectwatchv1.StatusIndicationInProgress, fmt.Sprintf("Deleting tenant %q", j.project.UID))
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(timedOutCtx)

	g.Go(func() error { return alertingmonitor.CleanupTenant(ctx, j.amClient) })
	if j.jobCfg.Sre.Enabled {
		g.Go(func() error { return sre.CleanupTenant(ctx, j.sreClient) })
	}
	g.Go(func() error { return loki.CleanupTenant(ctx, j.endpointsCfg.Loki) })
	g.Go(func() error { return mimir.CleanupTenant(ctx, j.endpointsCfg.Mimir) })

	if err := g.Wait(); err != nil {
		return err
	}

	return watcher.DeleteWatcher(parentCtx, j.project)
}

func setProjectMetadata(project *nexus.RuntimeprojectRuntimeProject, action controller.Action) {
	projectID, projectName, orgName := extractLabelsFrom(project)
	status := projects.ProjectCreated
	if action == controller.CleanupTenant {
		status = projects.ProjectDeleted
	}

	labels := prometheus.Labels{
		"projectId":   projectID,
		"projectName": projectName,
		"orgName":     orgName,
		"status":      string(status),
	}

	projectIDs.With(labels).Set(1)
	log.Printf("Added project metadata: %q", labels)
}

func removeProjectMetadata(project *nexus.RuntimeprojectRuntimeProject) {
	projectID, projectName, orgName := extractLabelsFrom(project)

	labels := prometheus.Labels{
		"projectId":   projectID,
		"projectName": projectName,
		"orgName":     orgName,
	}

	numberDeleted := projectIDs.DeletePartialMatch(labels)
	if numberDeleted > 0 {
		log.Printf("Removed project metadata: %q", labels)
	} else {
		log.Printf("Failed to remove project metadata: %q", labels)
	}
}

func removeProjectMetadataByID(projectID string) {
	labels := prometheus.Labels{
		"projectId": projectID,
	}

	numberDeleted := projectIDs.DeletePartialMatch(labels)
	if numberDeleted > 0 {
		log.Printf("Removed project metadata: %q", labels)
	} else {
		log.Printf("Failed to remove project metadata: %q", labels)
	}
}

func extractLabelsFrom(project *nexus.RuntimeprojectRuntimeProject) (projectID, projectName, orgName string) {
	orgName = ""
	if project.GetLabels() != nil {
		orgName = project.GetLabels()[utility.OrgNameLabel]
	}

	return string(project.UID), project.DisplayName(), orgName
}
