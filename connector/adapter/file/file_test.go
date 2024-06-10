package file_test

import (
	"encoding/json"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	f "github.com/teslamotors/fleet-telemetry/connector/adapter/file"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter/noop"
)

func saveFileBytes(testFilePath string, b []byte, connector *f.Connector) {
	before := connector.LastUpdate

	err := os.WriteFile(testFilePath, b, 0644)
	Expect(err).NotTo(HaveOccurred())

	Eventually(func() int64 {
		return connector.LastUpdate.UnixNano()
	}).Should(BeNumerically(">", before.UnixNano()))
}

func saveFile(testFilePath string, testData f.Data, connector *f.Connector) {
	jsonData, err := json.Marshal(testData)
	Expect(err).NotTo(HaveOccurred())
	saveFileBytes(testFilePath, jsonData, connector)
}

var _ = Describe("Connector", func() {
	var (
		testFilePath  string
		testConnector *f.Connector
		logger        *logrus.Logger
	)

	BeforeEach(func() {
		logger, _ = logrus.NoOpLogger()
		file, err := os.CreateTemp("/tmp", "test-file-*.json")
		Expect(err).NotTo(HaveOccurred())
		defer file.Close()

		testFilePath = file.Name()

		testData := f.Data{
			AllowedVins: []string{"VIN1", "VIN2"},
		}
		jsonData, err := json.Marshal(testData)
		Expect(err).NotTo(HaveOccurred())

		err = os.WriteFile(testFilePath, jsonData, 0644)
		Expect(err).NotTo(HaveOccurred())

		connector, err := f.NewConnector(f.Config{Path: testFilePath}, &noop.Collector{}, logger)
		Expect(err).NotTo(HaveOccurred())
		testConnector = connector
	})

	AfterEach(func() {
		os.Remove(testFilePath)
		testConnector.Close()
	})

	Describe("VinAllowed", func() {
		Context("with vins in the file", func() {
			It("should return true for allowed VIN", func() {
				Expect(testConnector.VinAllowed("VIN1")).To(BeTrue())
			})

			It("should return false for not allowed VIN", func() {
				Expect(testConnector.VinAllowed("VIN3")).To(BeFalse())
			})

			It("should update after file change", func() {
				Expect(testConnector.VinAllowed("VIN3")).To(BeFalse())

				saveFile(testFilePath, f.Data{
					AllowedVins: []string{"VIN3"},
				}, testConnector)

				Eventually(func() {
					Expect(testConnector.VinAllowed("VIN3")).To(BeTrue())
					Expect(testConnector.VinAllowed("VIN1")).To(BeFalse())
				})
			})
		})

		Context("with no vins in the file", func() {
			JustBeforeEach(func() {
				saveFile(testFilePath, f.Data{}, testConnector)
			})

			It("allows all vins", func() {
				for i := 0; i < 5; i++ {
					Expect(testConnector.VinAllowed(fmt.Sprintf("VIN%d", i))).To(BeTrue())
				}
			})
		})

		Context("with invalid file", func() {
			JustBeforeEach(func() {
				err := os.WriteFile(testFilePath, []byte("test"), 0644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("keeps prior rules", func() {
				Expect(testConnector.VinAllowed("VIN1")).To(BeTrue())
				Expect(testConnector.VinAllowed("VIN2")).To(BeTrue())
				Expect(testConnector.VinAllowed("VIN3")).To(BeFalse())
			})
		})
	})
})
