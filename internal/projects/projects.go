// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package projects

import (
	"log"
	"sync"

	"google.golang.org/grpc"

	pb "github.com/open-edge-platform/o11y-tenant-controller/api"
)

type ProjectData struct {
	ProjectName string
	OrgID       string
	Status      projectStatus
}

type projectStatus string

const (
	ProjectCreated projectStatus = "Created"
	ProjectDeleted projectStatus = "Deleted"
)

type Server struct {
	pb.UnimplementedProjectServiceServer

	GrpcServer *grpc.Server
	Port       int

	Mu       *sync.RWMutex
	Projects map[string]ProjectData

	Clients *sync.Map
}

func (s *Server) StreamProjectUpdates(_ *pb.EmptyRequest, stream pb.ProjectService_StreamProjectUpdatesServer) error {
	updateChan := make(chan struct{}, 1)

	// Add this client's channel to the list of clients
	s.Clients.Store(updateChan, struct{}{})

	// Ensure the client is removed when it disconnects
	defer func() {
		log.Printf("Client disconnected from StreamProjectUpdates")
		s.Clients.Delete(updateChan)
		// The channel is not closed explicitly on purpose to avoid panics when sending to a closed channel
	}()

	// Send the current state to the client when it first connects
	if err := s.SendCurrentStateToClient(stream); err != nil {
		return err
	}

	for {
		select {
		case <-updateChan:
			if err := s.SendCurrentStateToClient(stream); err != nil {
				return err
			}
		case <-stream.Context().Done():
			log.Printf("StreamProjectUpdates has been closed")
			return stream.Context().Err()
		}
	}
}

func (s *Server) SendCurrentStateToClient(stream pb.ProjectService_StreamProjectUpdatesServer) error {
	s.Mu.RLock()
	projectEntries := make([]*pb.ProjectEntry, 0, len(s.Projects))
	for key, project := range s.Projects {
		projectEntries = append(projectEntries, &pb.ProjectEntry{
			Key: key,
			Data: &pb.ProjectData{
				ProjectName: project.ProjectName,
				OrgName:     project.OrgID,
				Status:      string(project.Status),
			},
		})
	}
	s.Mu.RUnlock()

	projectUpdate := &pb.ProjectUpdate{Projects: projectEntries}
	return stream.Send(projectUpdate)
}

func (s *Server) BroadcastUpdate() {
	s.Clients.Range(func(key, _ interface{}) bool {
		updateChan, ok := key.(chan struct{})
		if !ok {
			log.Printf("Client channel is not of type chan struct{}")
			return true
		}

		select {
		case updateChan <- struct{}{}:
		default:
			// Ideally this should never happen
			log.Printf("Client channel is full, skipping update")
		}

		return true
	})
}
