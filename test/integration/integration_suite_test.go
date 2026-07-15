package integration_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	types "github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
)

func TestTelemetry(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite Tests", types.ReporterConfig{})
}
