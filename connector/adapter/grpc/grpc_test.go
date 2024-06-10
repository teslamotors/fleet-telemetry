package grpc_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	g "github.com/teslamotors/fleet-telemetry/connector/adapter/grpc"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter/noop"
	pb "github.com/teslamotors/fleet-telemetry/protos"
	"github.com/teslamotors/fleet-telemetry/test/mocks"
)

var _ = Describe("GrpcConnector", func() {
	var (
		mockCtrl   *gomock.Controller
		mockClient *mocks.MockVehicleServiceClient
		connector  *g.Connector
		logger     *logrus.Logger
	)

	BeforeEach(func() {
		logger, _ = logrus.NoOpLogger()
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mocks.NewMockVehicleServiceClient(mockCtrl)
		connector = g.NewTestGrpcConnector(mockClient, g.Config{}, noop.NewCollector(), logger)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("VinAllowed", func() {
		It("should return true if VIN is allowed", func() {
			mockClient.EXPECT().VinAllowed(
				gomock.Any(),
				&pb.VinAllowedRequest{Vin: "123"},
			).Return(&pb.VinAllowedResponse{Allowed: true}, nil)

			allowed, err := connector.VinAllowed("123")
			Expect(err).NotTo(HaveOccurred())
			Expect(allowed).To(BeTrue())
		})
		It("should return false if VIN is not allowed", func() {
			mockClient.EXPECT().VinAllowed(
				gomock.Any(),
				&pb.VinAllowedRequest{Vin: "123"},
			).Return(&pb.VinAllowedResponse{Allowed: false}, nil)

			allowed, err := connector.VinAllowed("123")
			Expect(err).NotTo(HaveOccurred())
			Expect(allowed).To(BeFalse())
		})

		It("should return an error if the client fails", func() {
			mockClient.EXPECT().VinAllowed(
				gomock.Any(),
				&pb.VinAllowedRequest{Vin: "789"},
			).Return(nil, fmt.Errorf("client failure"))

			allowed, err := connector.VinAllowed("789")
			Expect(err).To(HaveOccurred())
			Expect(allowed).To(BeFalse())
		})

		It("caches second request", func() {
			connector.Config.VinAllowed.CacheResults = true
			mockClient.EXPECT().VinAllowed(
				gomock.Any(),
				&pb.VinAllowedRequest{Vin: "789"},
			).Return(&pb.VinAllowedResponse{Allowed: false}, nil).Times(1)

			// makes grpc call
			allowed, err := connector.VinAllowed("789")
			Expect(err).ToNot(HaveOccurred())
			Expect(allowed).To(BeFalse())

			// uses cache
			allowed, err = connector.VinAllowed("789")
			Expect(err).ToNot(HaveOccurred())
			Expect(allowed).To(BeFalse())
		})
	})
})
