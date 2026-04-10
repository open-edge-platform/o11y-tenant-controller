// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"errors"
	"log"
	"math"
	"sync/atomic"
	"time"

	amproto "github.com/open-edge-platform/o11y-alerting-monitor/api/v1/management"
	sreproto "github.com/open-edge-platform/o11y-sre-exporter/api/config-reloader"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/open-edge-platform/o11y-tenant-controller/internal/alertingmonitor"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/config"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/controller"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/loki"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/mimir"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/projects"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/sre"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/util"
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
)

// projectInfo holds the identifying metadata for a project, replacing
// the former *nexus.RuntimeprojectRuntimeProject reference.
type projectInfo struct {
	ID          string
	ProjectName string
	OrgName     string
}

type JobManager struct {
	comSig       chan controller.CommChannel
	jobList      map[string]*job
	jobCfg       config.Job
	endpointsCfg config.Endpoints
	cancelFn     context.CancelFunc
	done         chan struct{}

	amClient  amproto.ManagementClient
	sreClient sreproto.ManagementClient
}

type job struct {
	project      projectInfo
	status       atomic.Int32
	jobCfg       config.Job
	endpointsCfg config.Endpoints
	cancelFn     context.CancelFunc
	resultCh     chan error // signals completion to the caller

	amClient  amproto.ManagementClient
	sreClient sreproto.ManagementClient
}

func New(channel chan controller.CommChannel, jCfg config.Job, endpoints config.Endpoints, amConn, sreConn *grpc.ClientConn) *JobManager {
	return &JobManager{
		comSig:       channel,
		jobList:      map[string]*job{},
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
					jm.startJob(ctx, v, controller.InitializeTenant)
				case controller.CleanupTenant:
					jm.startJob(ctx, v, controller.CleanupTenant)
				}
			case <-ticker.C:
				for k, job := range jm.jobList {
					status := jobStatus(job.status.Load())
					if status == tenantDeleted {
						removeProjectMetadata(job.project)
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

func (jm *JobManager) startJob(ctx context.Context, cc controller.CommChannel, action controller.Action) {
	pi := projectInfo{
		ID:          cc.ProjectID,
		ProjectName: cc.ProjectName,
		OrgName:     cc.OrgName,
	}
	setProjectMetadata(pi, action)
	j, exists := jm.jobList[cc.ProjectID]
	if exists {
		j.cancel()
		j.resultCh = cc.Result
		j.run(ctx, action)
	} else {
		j = newJob(pi, jm.jobCfg, jm.endpointsCfg, jm.amClient, jm.sreClient)
		j.resultCh = cc.Result
		jm.jobList[cc.ProjectID] = j
		j.run(ctx, action)
	}
}

func newJob(pi projectInfo, jCfg config.Job,
	endpoints config.Endpoints, am amproto.ManagementClient, sreCl sreproto.ManagementClient) *job {
	return &job{
		project:      pi,
		jobCfg:       jCfg,
		endpointsCfg: endpoints,
		amClient:     am,
		sreClient:    sreCl,
	}
}

func (j *job) run(parentCtx context.Context, action controller.Action) {
	j.status.Store(int32(jobInProgress))

	// Capture the result channel for this specific invocation so that a
	// subsequent startJob() call (which overwrites j.resultCh) cannot cause
	// the old goroutine to send its result to the new caller's channel.
	resultCh := j.resultCh

	go func() {
		ctx, cancel := context.WithCancel(parentCtx)
		j.cancelFn = cancel
		defer cancel()

		var jobErr error

		switch action {
		case controller.InitializeTenant:
			j.manageTenant(ctx, j.initializeTenant, controller.InitializeTenant)
			if errors.Is(ctx.Err(), context.Canceled) {
				j.status.Store(int32(jobCancelled))
				jobErr = ctx.Err()
			} else {
				j.status.Store(int32(tenantCreated))
			}
		case controller.CleanupTenant:
			j.manageTenant(ctx, j.cleanupTenant, controller.CleanupTenant)
			if errors.Is(ctx.Err(), context.Canceled) {
				j.status.Store(int32(jobCancelled))
				jobErr = ctx.Err()
			} else {
				j.status.Store(int32(tenantDeleted))
			}
		}

		// Signal the caller (if waiting) with the result.
		// Uses the captured channel, not j.resultCh, to avoid the race
		// where a new caller's channel receives this goroutine's result.
		if resultCh != nil {
			resultCh <- jobErr
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
	id := j.project.ID
	ctx := context.WithValue(parentCtx, utility.ContextKeyTenantID, id)

	for {
		err := tenantAction(ctx)
		if err == nil {
			log.Printf("%v action for tenantID %q completed successfully", action.String(), id)
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

	log.Printf("Creating tenant %q", j.project.ID)

	g, ctx := errgroup.WithContext(timedOutCtx)

	g.Go(func() error { return alertingmonitor.InitializeTenant(ctx, j.amClient) })
	if j.jobCfg.Sre.Enabled {
		g.Go(func() error { return sre.InitializeTenant(ctx, j.sreClient) })
	}

	if err := g.Wait(); err != nil {
		return err
	}

	log.Printf("Tenant %q created", j.project.ID)
	return nil
}

func (j *job) cleanupTenant(parentCtx context.Context) error {
	timedOutCtx, cancel := context.WithTimeout(parentCtx, j.jobCfg.Timeout)
	defer cancel()

	log.Printf("Deleting tenant %q", j.project.ID)

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

	log.Printf("Tenant %q deleted", j.project.ID)
	return nil
}

func setProjectMetadata(pi projectInfo, action controller.Action) {
	status := projects.ProjectCreated
	if action == controller.CleanupTenant {
		status = projects.ProjectDeleted
	}

	labels := prometheus.Labels{
		"projectId":   pi.ID,
		"projectName": pi.ProjectName,
		"orgName":     pi.OrgName,
		"status":      string(status),
	}

	projectIDs.With(labels).Set(1)
	log.Printf("Added project metadata: %q", labels)
}

func removeProjectMetadata(pi projectInfo) {
	labels := prometheus.Labels{
		"projectId":   pi.ID,
		"projectName": pi.ProjectName,
		"orgName":     pi.OrgName,
	}

	numberDeleted := projectIDs.DeletePartialMatch(labels)
	if numberDeleted > 0 {
		log.Printf("Removed project metadata: %q", labels)
	} else {
		log.Printf("Failed to remove project metadata: %q", labels)
	}
}
