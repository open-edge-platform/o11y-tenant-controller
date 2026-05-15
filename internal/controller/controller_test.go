// SPDX-FileCopyrightText: (C) 2026 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/open-edge-platform/orch-library/go/pkg/tenancy"
	"github.com/stretchr/testify/require"

	"github.com/open-edge-platform/o11y-tenant-controller/internal/projects"
)

// newTestServer creates a minimal projects.Server suitable for use in tests.
func newTestServer(t *testing.T) *projects.Server {
	t.Helper()
	return &projects.Server{
		Mu:       new(sync.RWMutex),
		Projects: make(map[string]projects.ProjectData),
		Clients:  new(sync.Map),
	}
}

func TestNew_NilGRPCServer(t *testing.T) {
	tc, err := New(10, nil)
	require.Error(t, err)
	require.Nil(t, tc)
}

func TestNew_Valid(t *testing.T) {
	srv := newTestServer(t)
	tc, err := New(10, srv)
	require.NoError(t, err)
	require.NotNil(t, tc)
	require.NotNil(t, tc.ComSig)
}

func TestStop_BeforeStart(t *testing.T) {
	srv := newTestServer(t)
	tc, err := New(10, srv)
	require.NoError(t, err)
	// Stop before Start must not panic: cancel is nil, server was never serving.
	require.NotPanics(t, tc.Stop)
}

func TestStop_MultipleCalls(t *testing.T) {
	srv := newTestServer(t)
	tc, err := New(10, srv)
	require.NoError(t, err)
	// Multiple calls must not panic (channel must not be closed twice).
	require.NotPanics(t, tc.Stop)
	require.NotPanics(t, tc.Stop)
}

func TestStart_AlreadyStarted(t *testing.T) {
	// Start() after Stop() is re-entered via start guard: second call returns error.
	// We can't call Start() without a real tenancy poller, so we manipulate the
	// started flag directly to unit-test the guard path.
	srv := newTestServer(t)
	tc, err := New(10, srv)
	require.NoError(t, err)
	tc.startMu.Lock()
	tc.started = true
	tc.startMu.Unlock()

	err = tc.Start(srv)
	require.ErrorContains(t, err, "already started")
}

func TestAction_String(t *testing.T) {
	require.Equal(t, "InitializeTenant", InitializeTenant.String())
	require.Equal(t, "CleanupTenant", CleanupTenant.String())
	// Unknown value must not panic.
	require.Equal(t, "Action(99)", Action(99).String())
}

func TestEventHandler_HandleEvent_OrgEvent_Ignored(t *testing.T) {
	srv := newTestServer(t)
	comSig := make(chan CommChannel, 10)
	h := &eventHandler{comSig: comSig, ctx: context.Background(), grpcServer: srv}

	orgName := "my-org"
	orgID := uuid.New()
	event := tenancy.Event{
		ResourceType: tenancy.ResourceTypeOrg,
		EventType:    tenancy.EventTypeCreated,
		ResourceID:   uuid.New(),
		ResourceName: "org-1",
		OrgID:        &orgID,
		OrgName:      &orgName,
	}

	err := h.HandleEvent(context.Background(), event)
	require.NoError(t, err)
	require.Empty(t, comSig, "org events must not be dispatched to comSig")
}

func TestEventHandler_HandleEvent_ProjectCreated(t *testing.T) {
	srv := newTestServer(t)
	comSig := make(chan CommChannel, 10)
	h := &eventHandler{comSig: comSig, ctx: context.Background(), grpcServer: srv}

	projectID := uuid.New()
	orgName := "my-org"
	orgID := uuid.New()
	event := tenancy.Event{
		ResourceType: tenancy.ResourceTypeProject,
		EventType:    tenancy.EventTypeCreated,
		ResourceID:   projectID,
		ResourceName: "project-1",
		OrgID:        &orgID,
		OrgName:      &orgName,
	}

	err := h.HandleEvent(context.Background(), event)
	require.NoError(t, err)
	require.Len(t, comSig, 1)

	msg := <-comSig
	require.Equal(t, projectID.String(), msg.ProjectID)
	require.Equal(t, "project-1", msg.ProjectName)
	require.Equal(t, "my-org", msg.OrgName)
	require.Equal(t, InitializeTenant, msg.Status)

	srv.Mu.RLock()
	pd, ok := srv.Projects[projectID.String()]
	srv.Mu.RUnlock()
	require.True(t, ok, "project must be stored in grpcServer.Projects")
	require.Equal(t, "project-1", pd.ProjectName)
	require.Equal(t, projects.ProjectCreated, pd.Status)
}

func TestEventHandler_HandleEvent_ProjectDeleted(t *testing.T) {
	srv := newTestServer(t)
	comSig := make(chan CommChannel, 10)
	h := &eventHandler{comSig: comSig, ctx: context.Background(), grpcServer: srv}

	projectID := uuid.New()
	orgName := "my-org"
	orgID := uuid.New()
	event := tenancy.Event{
		ResourceType: tenancy.ResourceTypeProject,
		EventType:    tenancy.EventTypeDeleted,
		ResourceID:   projectID,
		ResourceName: "project-2",
		OrgID:        &orgID,
		OrgName:      &orgName,
	}

	err := h.HandleEvent(context.Background(), event)
	require.NoError(t, err)
	require.Len(t, comSig, 1)

	msg := <-comSig
	require.Equal(t, projectID.String(), msg.ProjectID)
	require.Equal(t, "project-2", msg.ProjectName)
	require.Equal(t, "my-org", msg.OrgName)
	require.Equal(t, CleanupTenant, msg.Status)

	srv.Mu.RLock()
	pd, ok := srv.Projects[projectID.String()]
	srv.Mu.RUnlock()
	require.True(t, ok, "project must be stored in grpcServer.Projects")
	require.Equal(t, projects.ProjectDeleted, pd.Status)
}

func TestEventHandler_HandleEvent_UnknownEventType_Ignored(t *testing.T) {
	srv := newTestServer(t)
	comSig := make(chan CommChannel, 10)
	h := &eventHandler{comSig: comSig, ctx: context.Background(), grpcServer: srv}

	event := tenancy.Event{
		ResourceType: tenancy.ResourceTypeProject,
		EventType:    "unknown-event",
		ResourceID:   uuid.New(),
		ResourceName: "project-x",
	}

	err := h.HandleEvent(context.Background(), event)
	require.NoError(t, err)
	require.Empty(t, comSig, "unknown event types must not be dispatched")

	srv.Mu.RLock()
	projectCount := len(srv.Projects)
	srv.Mu.RUnlock()
	require.Zero(t, projectCount, "unknown events must not update grpcServer.Projects")
}

func TestEventHandler_HandleEvent_NilOrgName(t *testing.T) {
	srv := newTestServer(t)
	comSig := make(chan CommChannel, 10)
	h := &eventHandler{comSig: comSig, ctx: context.Background(), grpcServer: srv}

	event := tenancy.Event{
		ResourceType: tenancy.ResourceTypeProject,
		EventType:    tenancy.EventTypeCreated,
		ResourceID:   uuid.New(),
		ResourceName: "project-nil-org",
		OrgName:      nil,
	}

	err := h.HandleEvent(context.Background(), event)
	require.NoError(t, err)
	require.Len(t, comSig, 1)

	msg := <-comSig
	require.Empty(t, msg.OrgName, "nil OrgName must yield empty string in CommChannel")
}

func TestEventHandler_HandleEvent_MultipleProjects(t *testing.T) {
	srv := newTestServer(t)
	comSig := make(chan CommChannel, 20)
	h := &eventHandler{comSig: comSig, ctx: context.Background(), grpcServer: srv}

	orgName := "my-org"
	orgID := uuid.New()
	const numProjects = 5

	for i := 0; i < numProjects; i++ {
		event := tenancy.Event{
			ResourceType: tenancy.ResourceTypeProject,
			EventType:    tenancy.EventTypeCreated,
			ResourceID:   uuid.New(),
			ResourceName: "project",
			OrgID:        &orgID,
			OrgName:      &orgName,
		}
		err := h.HandleEvent(context.Background(), event)
		require.NoError(t, err)
	}

	require.Len(t, comSig, numProjects)

	srv.Mu.RLock()
	stored := len(srv.Projects)
	srv.Mu.RUnlock()
	require.Equal(t, numProjects, stored)
}

func TestEventHandler_HandleEvent_OrgNameStoredInProjectData(t *testing.T) {
	srv := newTestServer(t)
	comSig := make(chan CommChannel, 10)
	h := &eventHandler{comSig: comSig, ctx: context.Background(), grpcServer: srv}

	projectID := uuid.New()
	orgName := "expected-org"
	orgID := uuid.New()
	event := tenancy.Event{
		ResourceType: tenancy.ResourceTypeProject,
		EventType:    tenancy.EventTypeCreated,
		ResourceID:   projectID,
		ResourceName: "proj",
		OrgID:        &orgID,
		OrgName:      &orgName,
	}

	err := h.HandleEvent(context.Background(), event)
	require.NoError(t, err)

	srv.Mu.RLock()
	pd := srv.Projects[projectID.String()]
	srv.Mu.RUnlock()
	// OrgName is stored in the OrgID field of ProjectData per current mapping
	require.Equal(t, "expected-org", pd.OrgID)
}

// TestEventHandler_HandleEvent_ContextCancelled verifies that HandleEvent returns
// ctx.Err() instead of blocking when the controller context is cancelled (e.g.
// the channel buffer is full and Stop() has been called).
func TestEventHandler_HandleEvent_ContextCancelled(t *testing.T) {
	srv := newTestServer(t)
	// Zero-capacity channel so the send would block without the ctx guard.
	comSig := make(chan CommChannel)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel so ctx.Done() fires immediately
	h := &eventHandler{comSig: comSig, ctx: ctx, grpcServer: srv}

	projectID := uuid.New()
	orgName := "org"
	orgID := uuid.New()
	event := tenancy.Event{
		ResourceType: tenancy.ResourceTypeProject,
		EventType:    tenancy.EventTypeCreated,
		ResourceID:   projectID,
		ResourceName: "proj-x",
		OrgID:        &orgID,
		OrgName:      &orgName,
	}

	err := h.HandleEvent(context.Background(), event)
	require.ErrorIs(t, err, context.Canceled, "must return context.Canceled when channel is full and ctx is done")
	require.Empty(t, comSig, "no message must be queued")
}
