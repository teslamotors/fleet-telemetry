package integration_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	types "github.com/onsi/ginkgo/v2/types"
)

func TestTelemetry(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite Tests", types.ReporterConfig{SlowSpecThreshold: 10 * time.Second})
}
