package integration_test

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	gcStatsPath      = "/gc_stats"
	liveProfilerPath = "/live_profiler?mode=off"
	pprofPath        = "/debug/pprof/"
)

var _ = Describe("Profiler Endpoints", func() {
	var (
		client *http.Client
	)

	BeforeEach(func() {
		cfg, err := GetTLSConfig()
		Expect(err).NotTo(HaveOccurred())

		client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: cfg,
			},
		}
	})

	Context("Service port", func() {
		DescribeTable("returns 400",
			func(path string) {
				r, err := client.Get(fmt.Sprintf("https://%s%s", serviceURL, path))
				Expect(err).NotTo(HaveOccurred())
				Expect(r.StatusCode).To(Equal(400))
			},

			Entry("for "+gcStatsPath, gcStatsPath),
			Entry("for "+liveProfilerPath, liveProfilerPath),
			Entry("for "+pprofPath, pprofPath))
	})

	Context("Profiler port", func() {
		DescribeTable("succeeds",
			func(path string, expectedCode int) {
				r, err := client.Get(fmt.Sprintf("http://%s%s", profilerURL, path))
				Expect(err).NotTo(HaveOccurred())
				Expect(r.StatusCode).To(Equal(expectedCode))
			},

			Entry("for "+gcStatsPath, gcStatsPath, 200),
			Entry("for "+liveProfilerPath, liveProfilerPath, 304), // We get 304 when the profiler is enabled, and we ask it to switch to the mode it is already in
			Entry("for "+pprofPath, pprofPath, 200),
		)
	})
})
