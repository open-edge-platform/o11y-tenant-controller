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

	"github.com/open-edge-platform/orch-library/go/pkg/tenancy"

	pb "github.com/open-edge-platform/o11y-tenant-controller/api"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/config"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/controller"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/handler"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/jobs"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/projects"
)

const controllerName = "observability-tenant-controller"

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

	tenantCtrl, err := controller.New(cfg.Controller.Channel.MaxInflightRequests, &grpcServer)
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
	defer tenantCtrl.Stop()
	if err != nil {
		log.Panicf("Failed to start tenant controller: %v", err)
	}

	ticker := time.NewTicker(cfg.Job.Manager.Deletion.Rate)
	defer ticker.Stop()

	jobManager := jobs.New(tenantCtrl.ComSig, cfg.Job, cfg.Endpoints, amConn, sreConn)
	jobManager.Start(ticker)

	// Start the tenancy poller.
	tenantManagerURL := os.Getenv("TENANT_MANAGER_URL")
	if tenantManagerURL == "" {
		tenantManagerURL = "http://tenancy-manager.orch-iam.svc.cluster.local:8080"
	}

	h := &handler.TenancyHandler{Controller: tenantCtrl}
	poller := tenancy.NewPoller(tenantManagerURL, controllerName, h,
		func(cfg *tenancy.PollerConfig) {
			cfg.OnError = func(err error, msg string) {
				log.Printf("tenancy poller: %s: %v", msg, err)
			}
		},
	)

	go func() {
		if err := poller.Run(ctx); err != nil && ctx.Err() == nil {
			log.Printf("tenancy poller stopped with error: %v", err)
		}
	}()

	<-ctx.Done()
	jobManager.Stop()
}
