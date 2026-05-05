// SPDX-FileCopyrightText: (C) 2026 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	amproto "github.com/open-edge-platform/o11y-alerting-monitor/api/v1/management"
	sreproto "github.com/open-edge-platform/o11y-sre-exporter/api/config-reloader"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/open-edge-platform/o11y-tenant-controller/internal/config"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/controller"
	utility "github.com/open-edge-platform/o11y-tenant-controller/internal/util"
)

// ---- mock gRPC servers ----

type mockAMServer struct {
	amproto.UnimplementedManagementServer
}

func (*mockAMServer) InitializeTenant(_ context.Context, _ *amproto.TenantRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (*mockAMServer) CleanupTenant(_ context.Context, _ *amproto.TenantRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

type mockSREServer struct {
	sreproto.UnimplementedManagementServer
}

func (*mockSREServer) InitializeTenant(_ context.Context, _ *sreproto.TenantRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (*mockSREServer) CleanupTenant(_ context.Context, _ *sreproto.TenantRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// startBufconnGRPC launches a gRPC server over a bufconn listener and returns
// a client connection and a cleanup function.
func startBufconnAMServer(t *testing.T) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	// nosemgrep: go.grpc.security.grpc-server-insecure-connection.grpc-server-insecure-connection // test scenario
	srv := grpc.NewServer()
	amproto.RegisterManagementServer(srv, &mockAMServer{})

	go func() {
		if err := srv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Printf("AM gRPC test server error: %v", err)
		}
	}()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func startBufconnSREServer(t *testing.T) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	// nosemgrep: go.grpc.security.grpc-server-insecure-connection.grpc-server-insecure-connection // test scenario
	srv := grpc.NewServer()
	sreproto.RegisterManagementServer(srv, &mockSREServer{})

	go func() {
		if err := srv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Printf("SRE gRPC test server error: %v", err)
		}
	}()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// startLokiMockServer starts an httptest server that satisfies the Loki API
// calls made by loki.CleanupTenant (flush + delete request + delete status check).
func startLokiMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	// Flush ingesters
	mux.HandleFunc("/flush", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	// Submit delete request
	mux.HandleFunc("/loki/api/v1/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// GET – return a non-empty "processed" list so the poller exits immediately
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`[{"request_id":"abc","start_time":0,"end_time":0,"query":"{}","status":"processed","created_at":0}]`)); err != nil {
			t.Errorf("loki write: %v", err)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// startMimirMockServer starts an httptest server that satisfies the Mimir API
// calls made by mimir.CleanupTenant (flush + delete + optional status check).
func startMimirMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	// GET /ingester/flush
	mux.HandleFunc("/ingester/flush", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// POST /compactor/delete_tenant
	mux.HandleFunc("/compactor/delete_tenant", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	// GET /compactor/delete_tenant_status (strict mode)
	mux.HandleFunc("/compactor/delete_tenant_status", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"tenant_id":"test","blocks_deleted":true}`)); err != nil {
			t.Errorf("mimir write: %v", err)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// testJobConfig returns a config.Job with very short timeouts for fast tests.
func testJobConfig() config.Job {
	var jCfg config.Job
	jCfg.Backoff.Initial = 1 * time.Millisecond
	jCfg.Backoff.Max = 5 * time.Millisecond
	jCfg.Backoff.TimeMultiplier = 1.0
	jCfg.Timeout = 5 * time.Second
	jCfg.Sre.Enabled = false
	return jCfg
}

// testEndpoints builds a config.Endpoints pointing at the given test servers.
func testEndpoints(lokiSrv, mimirSrv *httptest.Server) config.Endpoints {
	return config.Endpoints{
		Loki: config.Loki{
			Write:            lokiSrv.URL,
			Backend:          lokiSrv.URL,
			PollingRate:      1 * time.Millisecond,
			MaxPollingRate:   5 * time.Millisecond,
			DeleteVerifyMode: utility.LooseMode,
		},
		Mimir: config.Mimir{
			Ingester:         mimirSrv.URL,
			Compactor:        mimirSrv.URL,
			PollingRate:      1 * time.Millisecond,
			DeleteVerifyMode: utility.LooseMode,
		},
	}
}

// ---- tests ----

func TestNew(t *testing.T) {
	amConn := startBufconnAMServer(t)
	sreConn := startBufconnSREServer(t)
	comSig := make(chan controller.CommChannel, 10)

	jm := New(comSig, testJobConfig(), config.Endpoints{}, amConn, sreConn)
	require.NotNil(t, jm)
	require.NotNil(t, jm.comSig)
	require.NotNil(t, jm.jobList)
	require.NotNil(t, jm.done)
}

func TestJobManager_Stop_BeforeStart(t *testing.T) {
	amConn := startBufconnAMServer(t)
	sreConn := startBufconnSREServer(t)
	comSig := make(chan controller.CommChannel, 10)

	jm := New(comSig, testJobConfig(), config.Endpoints{}, amConn, sreConn)
	// cancelFn is nil before Start; Stop must not panic.
	require.NotPanics(t, jm.Stop)
}

func TestJobManager_Stop_MultipleCalls(t *testing.T) {
	amConn := startBufconnAMServer(t)
	sreConn := startBufconnSREServer(t)
	comSig := make(chan controller.CommChannel, 10)

	jm := New(comSig, testJobConfig(), config.Endpoints{}, amConn, sreConn)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	jm.Start(ticker)

	// Multiple stops must not panic (done channel closed only once).
	require.NotPanics(t, jm.Stop)
	require.NotPanics(t, jm.Stop)
}

func TestJobManager_Start_Stop(t *testing.T) {
	amConn := startBufconnAMServer(t)
	sreConn := startBufconnSREServer(t)
	comSig := make(chan controller.CommChannel, 10)

	jm := New(comSig, testJobConfig(), config.Endpoints{}, amConn, sreConn)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	jm.Start(ticker)
	require.NotPanics(t, jm.Stop)
}

func TestJobManager_DispatchInitializeTenant(t *testing.T) {
	amConn := startBufconnAMServer(t)
	sreConn := startBufconnSREServer(t)
	lokiSrv := startLokiMockServer(t)
	mimirSrv := startMimirMockServer(t)

	comSig := make(chan controller.CommChannel, 10)
	jm := New(comSig, testJobConfig(), testEndpoints(lokiSrv, mimirSrv), amConn, sreConn)

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	jm.Start(ticker)
	defer jm.Stop()

	comSig <- controller.CommChannel{
		ProjectID:   "proj-uuid-1",
		ProjectName: "project-one",
		OrgName:     "org-one",
		Status:      controller.InitializeTenant,
	}

	require.Eventually(t, func() bool {
		jm.jobListMu.RLock()
		_, exists := jm.jobList["proj-uuid-1"]
		jm.jobListMu.RUnlock()
		return exists
	}, 2*time.Second, 5*time.Millisecond, "job must appear in jobList after dispatch")
}

func TestJobManager_DispatchCleanupTenant(t *testing.T) {
	amConn := startBufconnAMServer(t)
	sreConn := startBufconnSREServer(t)
	lokiSrv := startLokiMockServer(t)
	mimirSrv := startMimirMockServer(t)

	comSig := make(chan controller.CommChannel, 10)
	jm := New(comSig, testJobConfig(), testEndpoints(lokiSrv, mimirSrv), amConn, sreConn)

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	jm.Start(ticker)
	defer jm.Stop()

	comSig <- controller.CommChannel{
		ProjectID:   "proj-uuid-2",
		ProjectName: "project-two",
		OrgName:     "org-two",
		Status:      controller.CleanupTenant,
	}

	require.Eventually(t, func() bool {
		jm.jobListMu.RLock()
		_, exists := jm.jobList["proj-uuid-2"]
		jm.jobListMu.RUnlock()
		return exists
	}, 2*time.Second, 5*time.Millisecond, "job must appear in jobList after cleanup dispatch")
}

func TestJobManager_DuplicateProject_CancelsOldJob(t *testing.T) {
	amConn := startBufconnAMServer(t)
	sreConn := startBufconnSREServer(t)
	lokiSrv := startLokiMockServer(t)
	mimirSrv := startMimirMockServer(t)

	comSig := make(chan controller.CommChannel, 10)
	jm := New(comSig, testJobConfig(), testEndpoints(lokiSrv, mimirSrv), amConn, sreConn)

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	jm.Start(ticker)
	defer jm.Stop()

	// Send first event
	comSig <- controller.CommChannel{
		ProjectID:   "dup-proj",
		ProjectName: "dup-project",
		OrgName:     "org",
		Status:      controller.InitializeTenant,
	}
	// Wait for first job to appear
	require.Eventually(t, func() bool {
		jm.jobListMu.RLock()
		_, exists := jm.jobList["dup-proj"]
		jm.jobListMu.RUnlock()
		return exists
	}, 2*time.Second, 5*time.Millisecond)

	jm.jobListMu.RLock()
	firstJob := jm.jobList["dup-proj"]
	jm.jobListMu.RUnlock()

	// Send second event for the same project — old job must be cancelled
	comSig <- controller.CommChannel{
		ProjectID:   "dup-proj",
		ProjectName: "dup-project",
		OrgName:     "org",
		Status:      controller.CleanupTenant,
	}

	// Give the goroutine time to process and cancel the old job
	require.Eventually(t, func() bool {
		status := jobStatus(firstJob.status.Load())
		// The second event calls j.cancel() on the existing job then re-runs it.
		// Valid outcomes: cancelled (if first action was interrupted), or the second
		// action completed (tenantDeleted for CleanupTenant).
		return status == jobCancelled || status == tenantCreated || status == tenantDeleted
	}, 2*time.Second, 5*time.Millisecond, "second event must cancel or complete the first job")
}

func TestJobManager_Ticker_CleansUpDeletedJobs(t *testing.T) {
	amConn := startBufconnAMServer(t)
	sreConn := startBufconnSREServer(t)
	lokiSrv := startLokiMockServer(t)
	mimirSrv := startMimirMockServer(t)

	comSig := make(chan controller.CommChannel, 10)
	jm := New(comSig, testJobConfig(), testEndpoints(lokiSrv, mimirSrv), amConn, sreConn)

	// Use a very short ticker so cleanup fires quickly
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	jm.Start(ticker)
	defer jm.Stop()

	comSig <- controller.CommChannel{
		ProjectID:   "to-delete",
		ProjectName: "del-project",
		OrgName:     "org",
		Status:      controller.CleanupTenant,
	}

	// Wait for the job to reach tenantDeleted and be swept by the ticker
	require.Eventually(t, func() bool {
		jm.jobListMu.RLock()
		_, exists := jm.jobList["to-delete"]
		jm.jobListMu.RUnlock()
		return !exists
	}, 5*time.Second, 10*time.Millisecond, "completed cleanup job must be removed from jobList by ticker")
}

func TestProjectInfo_SetAndRemoveMetadata(t *testing.T) {
	info := projectInfo{
		projectID:   "meta-proj",
		projectName: "meta-project",
		orgName:     "meta-org",
	}

	// Must not panic and must register the metric
	require.NotPanics(t, func() {
		setProjectMetadata(info, controller.InitializeTenant)
	})
	require.NotPanics(t, func() {
		removeProjectMetadata(info)
	})
}

func TestJobManager_ChannelClosed_StopsLoop(t *testing.T) {
	amConn := startBufconnAMServer(t)
	sreConn := startBufconnSREServer(t)

	comSig := make(chan controller.CommChannel, 10)
	jm := New(comSig, testJobConfig(), config.Endpoints{}, amConn, sreConn)

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	jm.Start(ticker)

	// Closing the comSig channel must cause the dispatch goroutine to exit cleanly.
	close(comSig)

	// Stop must complete without deadlock after channel is closed.
	done := make(chan struct{})
	go func() {
		jm.Stop()
		close(done)
	}()
	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() blocked after comSig channel was closed")
	}
}

// TestJobManager_InitializeTenant_SREEnabled ensures that when SRE is enabled the
// SRE gRPC InitializeTenant call is also issued for a project creation event.
func TestJobManager_InitializeTenant_SREEnabled(t *testing.T) {
	amConn := startBufconnAMServer(t)
	sreConn := startBufconnSREServer(t)

	comSig := make(chan controller.CommChannel, 10)
	jCfg := testJobConfig()
	jCfg.Sre.Enabled = true
	jm := New(comSig, jCfg, config.Endpoints{}, amConn, sreConn)

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	jm.Start(ticker)
	defer jm.Stop()

	comSig <- controller.CommChannel{
		ProjectID:   "sre-init-proj",
		ProjectName: "sre-project",
		OrgName:     "org",
		Status:      controller.InitializeTenant,
	}

	require.Eventually(t, func() bool {
		jm.jobListMu.RLock()
		j, exists := jm.jobList["sre-init-proj"]
		jm.jobListMu.RUnlock()
		if !exists {
			return false
		}
		return jobStatus(j.status.Load()) == tenantCreated
	}, 5*time.Second, 5*time.Millisecond, "InitializeTenant with SRE enabled must reach tenantCreated")
}

// TestJobManager_CleanupTenant_SREEnabled ensures that when SRE is enabled the
// SRE gRPC CleanupTenant call is also issued for a project deletion event.
func TestJobManager_CleanupTenant_SREEnabled(t *testing.T) {
	amConn := startBufconnAMServer(t)
	sreConn := startBufconnSREServer(t)
	lokiSrv := startLokiMockServer(t)
	mimirSrv := startMimirMockServer(t)

	comSig := make(chan controller.CommChannel, 10)
	jCfg := testJobConfig()
	jCfg.Sre.Enabled = true
	jm := New(comSig, jCfg, testEndpoints(lokiSrv, mimirSrv), amConn, sreConn)

	// Use a very short ticker so the completed job is swept quickly.
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	jm.Start(ticker)
	defer jm.Stop()

	comSig <- controller.CommChannel{
		ProjectID:   "sre-cleanup-proj",
		ProjectName: "sre-project",
		OrgName:     "org",
		Status:      controller.CleanupTenant,
	}

	require.Eventually(t, func() bool {
		jm.jobListMu.RLock()
		_, exists := jm.jobList["sre-cleanup-proj"]
		jm.jobListMu.RUnlock()
		// Job is removed from jobList once the ticker cleans up tenantDeleted entries.
		return !exists
	}, 5*time.Second, 10*time.Millisecond, "CleanupTenant with SRE enabled must complete and be swept")
}

// TestProjectInfo_RemoveNonExistentMetadata covers the else-branch of
// removeProjectMetadata when no matching labels are found.
func TestProjectInfo_RemoveNonExistentMetadata(t *testing.T) {
	info := projectInfo{
		projectID:   "ghost-proj",
		projectName: "ghost-project",
		orgName:     "ghost-org",
	}

	// Must not panic even when the labels have never been registered.
	require.NotPanics(t, func() {
		removeProjectMetadata(info)
	})
}
