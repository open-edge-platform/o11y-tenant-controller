// SPDX-FileCopyrightText: (C) 2026 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"sync"
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
	utility "github.com/open-edge-platform/o11y-tenant-controller/internal/util"
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

// projectInfo holds the immutable metadata for a project that a job works on.
type projectInfo struct {
	projectID   string
	projectName string
	orgName     string
}

// JobManager reads from the controller's CommChannel and runs parallel provisioning
// jobs per project. Multiple events for the same project cancel the previous job
// and start a new one, preserving the original parallel-per-project design.
type JobManager struct {
	comSig       chan controller.CommChannel
	jobList      map[string]*job
	jobListMu    sync.RWMutex
	jobCfg       config.Job
	endpointsCfg config.Endpoints
	cancelFn     context.CancelFunc
	done         chan struct{}
	stopOnce     sync.Once

	amClient  amproto.ManagementClient
	sreClient sreproto.ManagementClient
}

type job struct {
	info         projectInfo
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
			case v, ok := <-jm.comSig:
				if !ok {
					return
				}
				switch v.Status {
				case controller.InitializeTenant:
					jm.startJob(ctx, v, controller.InitializeTenant)
				case controller.CleanupTenant:
					jm.startJob(ctx, v, controller.CleanupTenant)
				}
			case <-ticker.C:
				jm.jobListMu.Lock()
				for k, j := range jm.jobList {
					if jobStatus(j.status.Load()) == tenantDeleted {
						removeProjectMetadata(j.info)
						delete(jm.jobList, k)
					}
				}
				jm.jobListMu.Unlock()
			}
		}
	}()
}

func (jm *JobManager) Stop() {
	jm.stopOnce.Do(func() {
		if jm.cancelFn != nil {
			jm.cancelFn()
		}
		close(jm.done)
	})
}

func (jm *JobManager) startJob(ctx context.Context, msg controller.CommChannel, action controller.Action) {
	info := projectInfo{
		projectID:   msg.ProjectID,
		projectName: msg.ProjectName,
		orgName:     msg.OrgName,
	}
	setProjectMetadata(info, action)

	jm.jobListMu.Lock()
	defer jm.jobListMu.Unlock()

	j, exists := jm.jobList[msg.ProjectID]
	if exists {
		j.cancel()
		j.run(ctx, action)
	} else {
		j = newJob(info, jm.jobCfg, jm.endpointsCfg, jm.amClient, jm.sreClient)
		jm.jobList[msg.ProjectID] = j
		j.run(ctx, action)
	}
}

func newJob(info projectInfo, jCfg config.Job, endpoints config.Endpoints,
	am amproto.ManagementClient, sreCl sreproto.ManagementClient) *job {
	return &job{
		info:         info,
		jobCfg:       jCfg,
		endpointsCfg: endpoints,
		amClient:     am,
		sreClient:    sreCl,
	}
}

func (j *job) run(parentCtx context.Context, action controller.Action) {
	// Create the child context before spawning the goroutine so that j.cancelFn
	// is set synchronously under the caller's jobListMu lock, eliminating the
	// data race between j.cancel() and the goroutine setting j.cancelFn.
	ctx, cancel := context.WithCancel(parentCtx)
	j.cancelFn = cancel
	j.status.Store(int32(jobInProgress))

	go func() {
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
			j.status.Store(int32(tenantDeleted))
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
	id := j.info.projectID
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

	g, ctx := errgroup.WithContext(timedOutCtx)

	g.Go(func() error { return alertingmonitor.InitializeTenant(ctx, j.amClient) })
	if j.jobCfg.Sre.Enabled {
		g.Go(func() error { return sre.InitializeTenant(ctx, j.sreClient) })
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("initialize tenant %q: %w", j.info.projectID, err)
	}
	return nil
}

func (j *job) cleanupTenant(parentCtx context.Context) error {
	timedOutCtx, cancel := context.WithTimeout(parentCtx, j.jobCfg.Timeout)
	defer cancel()

	g, ctx := errgroup.WithContext(timedOutCtx)

	g.Go(func() error { return alertingmonitor.CleanupTenant(ctx, j.amClient) })
	if j.jobCfg.Sre.Enabled {
		g.Go(func() error { return sre.CleanupTenant(ctx, j.sreClient) })
	}
	g.Go(func() error { return loki.CleanupTenant(ctx, j.endpointsCfg.Loki) })
	g.Go(func() error { return mimir.CleanupTenant(ctx, j.endpointsCfg.Mimir) })

	if err := g.Wait(); err != nil {
		return fmt.Errorf("cleanup tenant %q: %w", j.info.projectID, err)
	}
	return nil
}

func setProjectMetadata(info projectInfo, action controller.Action) {
	status := projects.ProjectCreated
	if action == controller.CleanupTenant {
		status = projects.ProjectDeleted
	}

	labels := prometheus.Labels{
		"projectId":   info.projectID,
		"projectName": info.projectName,
		"orgName":     info.orgName,
		"status":      string(status),
	}

	projectIDs.With(labels).Set(1)
	log.Printf("Added project metadata: %q", labels)
}

func removeProjectMetadata(info projectInfo) {
	labels := prometheus.Labels{
		"projectId":   info.projectID,
		"projectName": info.projectName,
		"orgName":     info.orgName,
	}

	numberDeleted := projectIDs.DeletePartialMatch(labels)
	if numberDeleted > 0 {
		log.Printf("Removed project metadata: %q", labels)
	} else {
		log.Printf("Failed to remove project metadata: %q", labels)
	}
}
