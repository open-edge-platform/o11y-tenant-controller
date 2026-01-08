// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package watcher

import (
	"context"
	"testing"

	projectwatchv1 "github.com/open-edge-platform/orch-utils/tenancy-datamodel/build/apis/projectactivewatcher.edge-orchestrator.intel.com/v1"
	runtimev1 "github.com/open-edge-platform/orch-utils/tenancy-datamodel/build/apis/runtime.edge-orchestrator.intel.com/v1"
	folderv1 "github.com/open-edge-platform/orch-utils/tenancy-datamodel/build/apis/runtimefolder.edge-orchestrator.intel.com/v1"
	orgv1 "github.com/open-edge-platform/orch-utils/tenancy-datamodel/build/apis/runtimeorg.edge-orchestrator.intel.com/v1"
	projectv1 "github.com/open-edge-platform/orch-utils/tenancy-datamodel/build/apis/runtimeproject.edge-orchestrator.intel.com/v1"
	nexus "github.com/open-edge-platform/orch-utils/tenancy-datamodel/build/nexus-client"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-edge-platform/o11y-tenant-controller/internal/util"
)

func TestUpdateCreation(t *testing.T) {
	t.Run("Nil project - error expected", func(t *testing.T) {
		err := CreateUpdateWatcher(t.Context(), nil, projectwatchv1.StatusIndicationIdle, "test")
		require.ErrorContains(t, err, "failed to create/update watcher: project cannot be nil", "CreateWatcher function didn't return expected error")
	})

	t.Run("Valid project - no error expected", func(t *testing.T) {
		project, err := prepareProject(t)
		require.NoError(t, err, "Error creating project")

		err = CreateUpdateWatcher(t.Context(), project, projectwatchv1.StatusIndicationIdle, "test")
		require.NoError(t, err, "CreateWatcher function returned an error")
	})
}

func TestCreation(t *testing.T) {
	t.Run("Nil project - error expected", func(t *testing.T) {
		err := createWatcher(t.Context(), nil, projectwatchv1.StatusIndicationIdle, "test")
		require.ErrorContains(t, err, "failed to create watcher: project cannot be nil", "CreateWatcher function didn't return expected error")
	})

	t.Run("Valid project - no error expected", func(t *testing.T) {
		project, err := prepareProject(t)
		require.NoError(t, err, "Error creating project")

		err = createWatcher(t.Context(), project, projectwatchv1.StatusIndicationIdle, "test")
		require.NoError(t, err, "CreateWatcher function returned an error")
	})
}

func TestUpdate(t *testing.T) {
	t.Run("Nil watcher, context with value - error expected", func(t *testing.T) {
		ctx := context.WithValue(t.Context(), utility.ContextKeyTenantID, "foo")

		err := updateWatcher(ctx, nil, projectwatchv1.StatusIndicationIdle, "foo", "test")
		require.ErrorContains(t, err, "failed to update watcher: watcher cannot be nil", "UpdateWatcher function didn't return expected error")
	})

	t.Run("Nil watcher, context without value - error expected", func(t *testing.T) {
		err := updateWatcher(t.Context(), nil, projectwatchv1.StatusIndicationIdle, "foo", "test")
		require.ErrorContains(t, err, "failed to update watcher: watcher cannot be nil", "UpdateWatcher function didn't return expected error")
	})

	t.Run("Valid watcher, context without value - error expected", func(t *testing.T) {
		watcher, err := prepareWatcher(t, "foo")
		require.NoError(t, err, "Error creating project")

		err = updateWatcher(t.Context(), watcher, projectwatchv1.StatusIndicationIdle, "foo", "test")
		require.ErrorContains(t, err, "from context", "UpdateWatcher functiondidn't return expected error")
	})

	t.Run("Valid watcher - no error expected", func(t *testing.T) {
		watcher, err := prepareWatcher(t, "foo")
		require.NoError(t, err, "Error creating project")

		ctx := context.WithValue(t.Context(), utility.ContextKeyTenantID, "foo")

		err = updateWatcher(ctx, watcher, projectwatchv1.StatusIndicationIdle, "foo", "test")
		require.NoError(t, err, "UpdateWatcher function returned an error")
	})
}
func TestDeletion(t *testing.T) {
	t.Run("Nil project - error expected", func(t *testing.T) {
		err := DeleteWatcher(t.Context(), nil)
		require.ErrorContains(t, err, "failed to delete watcher: project cannot be nil", "DeleteWatcher function didn't return expected error")
	})

	t.Run("Valid project - no error expected", func(t *testing.T) {
		project, err := prepareProject(t)
		require.NoError(t, err, "Error creating project")

		ctx := context.WithValue(t.Context(), utility.ContextKeyTenantID, "foo")

		err = DeleteWatcher(ctx, project)
		require.NoError(t, err, "DeleteWatcher function returned an error")
	})
}

func TestCheckWatcher(t *testing.T) {
	t.Run("Nil watcher, value in context - error expected", func(t *testing.T) {
		ctx := context.WithValue(t.Context(), utility.ContextKeyTenantID, "foo")
		err := checkWatcher(ctx, nil)
		require.ErrorContains(t, err, "watcher cannot be nil", "CheckWatcher function didn't return expected error")
	})

	t.Run("Valid watcher, no value in context - error expected", func(t *testing.T) {
		watcher, err := prepareWatcher(t, "foo")
		require.NoError(t, err, "Error creating project")

		err = checkWatcher(t.Context(), watcher)
		require.ErrorContains(t, err, "from context", "CheckWatcher function didn't return expected error")
	})

	t.Run("Valid watcher, ids dont match - no error expected", func(t *testing.T) {
		watcher, err := prepareWatcher(t, "bar")
		require.NoError(t, err, "Error creating project")

		ctx := context.WithValue(t.Context(), utility.ContextKeyTenantID, "foo")

		err = checkWatcher(ctx, watcher)
		require.ErrorAs(t, err, &IDsDoNotMatchError{}, "CheckWatcher didn't return an IDsDoNotMatchError")
	})

	t.Run("Valid watcher - no error expected", func(t *testing.T) {
		watcher, err := prepareWatcher(t, "foo")
		require.NoError(t, err, "Error creating project")

		ctx := context.WithValue(t.Context(), utility.ContextKeyTenantID, "foo")

		err = checkWatcher(ctx, watcher)
		require.NoError(t, err, "CheckWatcher function returned an error")
	})
}

func prepareProject(t *testing.T) (*nexus.RuntimeprojectRuntimeProject, error) {
	client := nexus.NewFakeClient()
	runtime, err := client.TenancyMultiTenancy().AddRuntime(t.Context(), &runtimev1.Runtime{})
	if err != nil {
		return nil, err
	}
	org, err := runtime.AddOrgs(t.Context(), &orgv1.RuntimeOrg{})
	if err != nil {
		return nil, err
	}
	folder, err := org.AddFolders(t.Context(), &folderv1.RuntimeFolder{})
	if err != nil {
		return nil, err
	}
	project, err := folder.AddProjects(t.Context(), &projectv1.RuntimeProject{})
	if err != nil {
		return nil, err
	}
	return project, nil
}

func prepareWatcher(t *testing.T, projUID string) (*nexus.ProjectactivewatcherProjectActiveWatcher, error) {
	client := nexus.NewFakeClient()
	runtime, err := client.TenancyMultiTenancy().AddRuntime(t.Context(), &runtimev1.Runtime{})
	if err != nil {
		return nil, err
	}
	org, err := runtime.AddOrgs(t.Context(), &orgv1.RuntimeOrg{})
	if err != nil {
		return nil, err
	}
	folder, err := org.AddFolders(t.Context(), &folderv1.RuntimeFolder{})
	if err != nil {
		return nil, err
	}
	specProj := &projectv1.RuntimeProject{}
	specProj.UID = types.UID(projUID)
	project, err := folder.AddProjects(t.Context(), specProj)
	if err != nil {
		return nil, err
	}
	watcher, err := project.AddActiveWatchers(t.Context(), &projectwatchv1.ProjectActiveWatcher{})
	if err != nil {
		return nil, err
	}
	return watcher, nil
}
