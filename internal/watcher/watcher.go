// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package watcher

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	projectwatchv1 "github.com/open-edge-platform/orch-utils/tenancy-datamodel/build/apis/projectactivewatcher.edge-orchestrator.intel.com/v1"
	nexus "github.com/open-edge-platform/orch-utils/tenancy-datamodel/build/nexus-client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-edge-platform/o11y-tenant-controller/internal/util"
)

// IDsDoNotMatchError is returned when project ID within context don't match with ID from actual project CR.
type IDsDoNotMatchError struct {
	msg string
}

func (e IDsDoNotMatchError) Error() string {
	return e.msg
}

func CreateUpdateWatcher(ctx context.Context, project *nexus.RuntimeprojectRuntimeProject, status projectwatchv1.ActiveWatcherStatus, message string) error {
	if project == nil {
		return errors.New("failed to create/update watcher: project cannot be nil")
	}

	watcher, err := project.GetActiveWatchers(ctx, util.AppName)
	if err != nil && !nexus.IsChildNotFound(err) && !nexus.IsNotFound(err) {
		return fmt.Errorf("failed to delete watcher for tenant %q: %w", project.ObjectMeta.UID, err)
	}
	if nexus.IsChildNotFound(err) || nexus.IsNotFound(err) {
		return createWatcher(ctx, project, status, message)
	}
	return updateWatcher(ctx, watcher, status, string(project.ObjectMeta.UID), message)
}

func DeleteWatcher(ctx context.Context, project *nexus.RuntimeprojectRuntimeProject) error {
	if project == nil {
		return errors.New("failed to delete watcher: project cannot be nil")
	}

	// Get watcher to check tenantID in message
	watcher, err := project.GetActiveWatchers(ctx, util.AppName)
	if err != nil && !nexus.IsChildNotFound(err) && !nexus.IsNotFound(err) {
		return fmt.Errorf("failed to delete watcher for tenant %q: %w", project.ObjectMeta.UID, err)
	}

	if nexus.IsNotFound(err) || nexus.IsChildNotFound(err) {
		log.Printf("Watcher already deleted for tenantID %q", project.ObjectMeta.UID)
		return nil
	}

	if err := checkWatcher(ctx, watcher); err != nil {
		return err
	}

	log.Printf("Deleting watcher for tenantID %q", project.ObjectMeta.UID)
	err = project.DeleteActiveWatchers(ctx, util.AppName)

	if nexus.IsNotFound(err) || nexus.IsChildNotFound(err) {
		log.Printf("Watcher already deleted for tenantID %q", project.ObjectMeta.UID)
	} else if err != nil {
		return fmt.Errorf("failed to delete watcher for tenantID %q: %w", project.ObjectMeta.UID, err)
	}

	log.Printf("Watcher for tenantID %q deleted", project.ObjectMeta.UID)
	return nil
}

func checkWatcher(ctx context.Context, watcher *nexus.ProjectactivewatcherProjectActiveWatcher) error {
	tenantID, ok := ctx.Value(util.ContextKeyTenantID).(string)
	if !ok {
		return fmt.Errorf("failed to retrieve %q from context", util.ContextKeyTenantID)
	}

	if watcher == nil {
		return fmt.Errorf("failed to check watcher for tenant %q: watcher cannot be nil", tenantID)
	}

	parentProject, err := watcher.GetParent(ctx)
	if err != nil && !nexus.IsChildNotFound(err) && !nexus.IsNotFound(err) {
		return fmt.Errorf("failed to get watcher parent for tenant %q", tenantID)
	}

	if parentProject == nil {
		return IDsDoNotMatchError{fmt.Sprintf("actual and job's project IDs do not match for tenant %q", tenantID)}
	}

	if tenantID != string(parentProject.UID) {
		return IDsDoNotMatchError{fmt.Sprintf("actual and job's project IDs do not match for tenant %q", tenantID)}
	}
	return nil
}

func createWatcher(ctx context.Context, project *nexus.RuntimeprojectRuntimeProject, status projectwatchv1.ActiveWatcherStatus, message string) error {
	if project == nil {
		return errors.New("failed to create watcher: project cannot be nil")
	}

	log.Printf("Creating watcher for tenantID %q", project.ObjectMeta.UID)
	_, err := project.AddActiveWatchers(ctx, &projectwatchv1.ProjectActiveWatcher{
		ObjectMeta: metav1.ObjectMeta{
			Name: util.AppName,
		},
		Spec: projectwatchv1.ProjectActiveWatcherSpec{
			StatusIndicator: status,
			TimeStamp:       safeUnixTime(),
			Message:         message,
		},
	})

	if err != nil {
		return fmt.Errorf("failed to create watcher for tenantID %q: %w", project.ObjectMeta.UID, err)
	}

	log.Printf("Watcher for tenantID %q created", project.ObjectMeta.UID)
	return nil
}

func updateWatcher(ctx context.Context, watcher *nexus.ProjectactivewatcherProjectActiveWatcher,
	status projectwatchv1.ActiveWatcherStatus, tenantID, message string) error {
	if watcher == nil {
		return errors.New("failed to update watcher: watcher cannot be nil")
	}

	err := checkWatcher(ctx, watcher)
	if err != nil {
		if errors.As(err, &IDsDoNotMatchError{}) {
			log.Printf("Skipped updating watcher for tenantID %q - actual and job's project IDs do not match", tenantID)
			return nil
		}
		return err
	}

	log.Printf("Updating watcher for tenantID %q", tenantID)
	watcher.Spec.StatusIndicator = status
	watcher.Spec.Message = message
	watcher.Spec.TimeStamp = safeUnixTime()
	err = watcher.Update(ctx)
	if err != nil {
		return fmt.Errorf("failed to update ProjectActiveWatcher for tenant %q: %w", tenantID, err)
	}
	return nil
}

func safeUnixTime() uint64 {
	t := time.Now().Unix()
	if t < 0 {
		return 0
	}
	return uint64(t)
}
