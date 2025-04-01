// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/open-edge-platform/o11y-tenant-controller/api"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/config"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/controller"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/jobs"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/projects"
)

func main() {
	cfgFilePath := flag.String("config", "", "path to the config file")
	flag.Parse()

	cfg, err := config.ReadConfig(*cfgFilePath)
	if err != nil {
		log.Panicf("Failed to load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	grpcServer := projects.Server{
		GrpcServer: grpc.NewServer(),
		Port:       50051,
		Mu:         &sync.RWMutex{},
		Projects:   make(map[string]projects.ProjectData),
		Clients:    &sync.Map{},
	}

	lis, err := net.Listen("tcp", ":"+strconv.Itoa(grpcServer.Port))
	if err != nil {
		log.Panicf("Failed to listen: %v", err)
	}
	pb.RegisterProjectServiceServer(grpcServer.GrpcServer, &grpcServer)

	go func() {
		if err := grpcServer.GrpcServer.Serve(lis); err != nil {
			log.Printf("gRPC server failed to serve: %v", err)
			stop()
		}
	}()
	log.Printf("gRPC server listening on port %d", grpcServer.Port)

	tenantCtrl, err := controller.New(cfg.Controller.Channel.MaxInflightRequests, cfg.Controller.CreateDeleteWatcherTimeout, &grpcServer)
	if err != nil {
		log.Panicf("Failed to create tenant controller: %v", err)
	}

	amConn, err := grpc.NewClient(cfg.Endpoints.AlertingMonitor,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithConnectParams(grpc.ConnectParams{Backoff: backoff.DefaultConfig}),
	)
	if err != nil {
		log.Panicf("Failed to create alerting monitor gRPC client: %v", err)
	}
	defer amConn.Close()

	sreConn, err := grpc.NewClient(cfg.Endpoints.Sre,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithConnectParams(grpc.ConnectParams{Backoff: backoff.DefaultConfig}),
	)
	if err != nil {
		log.Panicf("Failed to create sre-exporter gRPC client: %v", err)
	}

	err = tenantCtrl.Start()
	// defer before checking error done on purpose - to ensure cleanup (Start may fail for reasons other than an error at addProjectWatcher).
	defer tenantCtrl.Stop()
	if err != nil {
		log.Panicf("Failed to start tenant controller: %v", err)
	}

	ticker := time.NewTicker(cfg.Job.Manager.Deletion.Rate)
	defer ticker.Stop()

	jobManager := jobs.New(tenantCtrl.ComSig, cfg.Job, cfg.Endpoints, amConn, sreConn)
	jobManager.Start(ticker)

	<-ctx.Done()
	jobManager.Stop()
}
