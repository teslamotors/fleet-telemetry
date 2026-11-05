package statsd

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	sd "github.com/smira/go-statsd"
)

var _ = Describe("getTags", func() {
	It("returns one tag per label", func() {
		tags := getTags(map[string]string{"record_type": "V"})

		Expect(tags).To(Equal([]sd.Tag{sd.StringTag("record_type", "V")}))
	})

	It("returns no tags when there are no labels", func() {
		Expect(getTags(map[string]string{})).To(BeEmpty())
	})
})
