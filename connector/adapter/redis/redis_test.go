package redis_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"

	r "github.com/teslamotors/fleet-telemetry/connector/adapter/redis"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter/noop"
)

var _ = Describe("RedisConnector", func() {
	var (
		mock      redismock.ClientMock
		client    *redis.Client
		connector *r.Connector
		config    r.Config
	)

	BeforeEach(func() {
		client, mock = redismock.NewClientMock()
		config = r.Config{
			VinAllowed: r.VinAllowed{
				Prefix: "vin_allowed:",
			},
		}

		connector = r.NewTestRedisConnector(client, config)
	})

	AfterEach(func() {
		mock.ClearExpect()
	})

	Describe("NewRedisConnector", func() {
		It("should create a new RedisConnector with default prefix", func() {
			connector, err := r.NewConnector(config, noop.NewCollector(), &logrus.Logger{})
			Expect(err).NotTo(HaveOccurred())
			Expect(connector).NotTo(BeNil())
		})
	})

	Describe("VinAllowed", func() {
		Context("with default prefix", func() {
			It("should return true if VIN is allowed", func() {
				mock.ExpectExists("vin_allowed:123").SetVal(1)

				allowed, err := connector.VinAllowed("123")
				Expect(err).NotTo(HaveOccurred())
				Expect(allowed).To(BeTrue())
			})

			It("should return false if VIN is not allowed", func() {
				mock.ExpectExists("vin_allowed:456").SetVal(0)

				allowed, err := connector.VinAllowed("456")
				Expect(err).NotTo(HaveOccurred())
				Expect(allowed).To(BeFalse())
			})
		})

		Context("with custom prefix", func() {
			BeforeEach(func() {
				connector.Config.VinAllowed.Prefix = "other_prefix:"
			})
			It("should use custom prefix", func() {
				mock.ExpectExists("other_prefix:123").SetVal(0)

				allowed, err := connector.VinAllowed("123")
				Expect(err).NotTo(HaveOccurred())
				Expect(allowed).To(BeFalse())
			})
		})
	})
})
