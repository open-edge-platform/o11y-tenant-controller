// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package alertingmonitor_test

import (
	"context"
	"errors"
	"log"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	proto "github.com/open-edge-platform/o11y-alerting-monitor/api/v1/management"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/open-edge-platform/o11y-tenant-controller/internal/alertingmonitor"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/util"
)

var (
	lis        *bufconn.Listener
	client     proto.ManagementClient
	server     *grpc.Server
	mockServer *mockManagementServer
)

type mockManagementServer struct {
	proto.UnimplementedManagementServer
	tenants       map[string]bool
	simulateError bool
	errorToReturn error
}

func newMockManagementServer() *mockManagementServer {
	return &mockManagementServer{
		tenants: make(map[string]bool),
	}
}

func (m *mockManagementServer) InitializeTenant(_ context.Context, req *proto.TenantRequest) (*emptypb.Empty, error) {
	if m.simulateError {
		return nil, m.errorToReturn
	}
	tenantID := req.GetTenant()

	if tenantID == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant name")
	}

	if m.tenants[tenantID] {
		return nil, status.Error(codes.AlreadyExists, "tenant already exists")
	}

	m.tenants[tenantID] = true
	return nil, nil
}

func (m *mockManagementServer) CleanupTenant(_ context.Context, req *proto.TenantRequest) (*emptypb.Empty, error) {
	if m.simulateError {
		return nil, m.errorToReturn
	}
	tenantID := req.GetTenant()

	if tenantID == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant name")
	}

	if !m.tenants[tenantID] {
		return nil, status.Error(codes.NotFound, "tenant not found")
	}

	delete(m.tenants, tenantID)
	return nil, nil
}

var _ = Describe("AlertingMonitor", Ordered, func() {
	BeforeAll(func() {
		lis = bufconn.Listen(1024 * 1024)

		// Create and register the mock server
		server = grpc.NewServer()
		mockServer = newMockManagementServer()
		proto.RegisterManagementServer(server, mockServer)

		go func() {
			defer GinkgoRecover()
			if err := server.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
				log.Printf("Error serving server: %v", err)
			}
		}()

		conn, err := grpc.NewClient(
			"passthrough://bufnet",
			grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
				return lis.Dial()
			}),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		Expect(err).ToNot(HaveOccurred())
		client = proto.NewManagementClient(conn)
	})

	AfterAll(func() {
		server.Stop()
		Expect(lis.Close()).To(Succeed())
	})

	BeforeEach(func() {
		// Reset mock server state
		mockServer.tenants = make(map[string]bool)
		mockServer.simulateError = false
		mockServer.errorToReturn = nil
	})

	It("Initialize tenant - context has no tenant value", func() {
		err := alertingmonitor.InitializeTenant(context.Background(), client)
		Expect(err).Should(HaveOccurred())
	})

	It("Initialize tenant with valid name", func() {
		ctx := context.WithValue(context.Background(), util.ContextKeyTenantID, "NewTenant")
		err := alertingmonitor.InitializeTenant(ctx, client)
		Expect(err).ShouldNot(HaveOccurred())
	})

	It("Initialize tenant with valid name - tenant already exists", func() {
		ctx := context.WithValue(context.Background(), util.ContextKeyTenantID, "ExistingTenant")

		// First initialization should succeed
		err := alertingmonitor.InitializeTenant(ctx, client)
		Expect(err).ShouldNot(HaveOccurred())

		// Second initialization should also succeed, as AlreadyExists is marked as success
		err = alertingmonitor.InitializeTenant(ctx, client)
		Expect(err).ShouldNot(HaveOccurred())
	})

	It("Initialize tenant with invalid (empty) name - error returned when server validates tenant name", func() {
		tenant := ""

		ctx := context.WithValue(context.Background(), util.ContextKeyTenantID, tenant)
		err := alertingmonitor.InitializeTenant(ctx, client)
		Expect(err).Should(HaveOccurred())

		// Check that the error code is InvalidArgument
		grpcStatus, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(grpcStatus.Code()).To(Equal(codes.InvalidArgument))
		Expect(grpcStatus.Message()).To(ContainSubstring("invalid tenant name"))
	})

	It("Initialize tenant with valid name - unexpected error", func() {
		// Simulate an error during initialization
		mockServer.simulateError = true
		mockServer.errorToReturn = status.Error(codes.Internal, "unexpected server error")

		ctx := context.WithValue(context.Background(), util.ContextKeyTenantID, "NewTenant")
		err := alertingmonitor.InitializeTenant(ctx, client)
		Expect(err).Should(HaveOccurred())

		// Check that the error code is Internal
		grpcStatus, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(grpcStatus.Code()).To(Equal(codes.Internal))
		Expect(grpcStatus.Message()).To(ContainSubstring("unexpected server error"))
	})

	It("Cleanup tenant - context has no tenant value", func() {
		err := alertingmonitor.CleanupTenant(context.Background(), client)
		Expect(err).Should(HaveOccurred())
	})

	It("Cleanup tenant with valid name", func() {
		ctx := context.WithValue(context.Background(), util.ContextKeyTenantID, "NewTenant")
		// Initialize first so the map is not empty
		err := alertingmonitor.InitializeTenant(ctx, client)
		Expect(err).ShouldNot(HaveOccurred())

		// Now, cleanup the tenant
		err = alertingmonitor.CleanupTenant(ctx, client)
		Expect(err).ShouldNot(HaveOccurred())
	})

	It("Cleanup tenant with valid name - tenant has already been deleted", func() {
		ctx := context.WithValue(context.Background(), util.ContextKeyTenantID, "NewTenant")
		err := alertingmonitor.InitializeTenant(ctx, client)
		Expect(err).ShouldNot(HaveOccurred())

		err = alertingmonitor.CleanupTenant(ctx, client)
		Expect(err).ShouldNot(HaveOccurred())

		// A second cleanup should also result in success
		err = alertingmonitor.CleanupTenant(ctx, client)
		Expect(err).ShouldNot(HaveOccurred())
	})

	It("Cleanup tenant with invalid (empty) name - error returned when server validates tenant name", func() {
		tenant := ""

		ctx := context.WithValue(context.Background(), util.ContextKeyTenantID, tenant)
		err := alertingmonitor.CleanupTenant(ctx, client)
		Expect(err).Should(HaveOccurred())

		// Check that the error code is InvalidArgument
		grpcStatus, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(grpcStatus.Code()).To(Equal(codes.InvalidArgument))
		Expect(grpcStatus.Message()).To(ContainSubstring("invalid tenant name"))
	})

	It("Cleanup tenant with valid name - unexpected error", func() {
		// Simulate an error during initialization
		mockServer.simulateError = true
		mockServer.errorToReturn = status.Error(codes.Internal, "unexpected server error")

		ctx := context.WithValue(context.Background(), util.ContextKeyTenantID, "NewTenant")
		err := alertingmonitor.CleanupTenant(ctx, client)
		Expect(err).Should(HaveOccurred())

		// Check that the error code is Internal
		grpcStatus, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(grpcStatus.Code()).To(Equal(codes.Internal))
		Expect(grpcStatus.Message()).To(ContainSubstring("unexpected server error"))
	})
})
