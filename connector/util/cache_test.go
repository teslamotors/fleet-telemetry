package util_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/connector/util"
)

var _ = Describe("Cache", func() {
	var c *util.Cache[string]

	BeforeEach(func() {
		c = util.NewCache[string](5 * time.Millisecond)
	})

	AfterEach(func() {
		c.Close()
	})

	Describe("Set and Get", func() {
		It("should store and retrieve a value", func() {
			c.Set("key1", "value1", time.Minute)
			val, ok := c.Get("key1")
			Expect(ok).To(BeTrue())
			Expect(*val).To(Equal("value1"))
		})

		It("should return not ok for a non-existent key", func() {
			_, ok := c.Get("non-existent")
			Expect(ok).To(BeFalse())
		})
	})

	Describe("Delete", func() {
		It("should delete a value", func() {
			c.Set("key1", "value1", time.Minute)
			c.Delete("key1")
			_, ok := c.Get("key1")
			Expect(ok).To(BeFalse())
		})
	})

	Describe("Clear", func() {
		It("should clear all values", func() {
			c.Set("key1", "value1", time.Minute)
			c.Set("key2", "value2", time.Minute)
			c.Clear()
			_, ok1 := c.Get("key1")
			_, ok2 := c.Get("key2")
			Expect(ok1).To(BeFalse())
			Expect(ok2).To(BeFalse())
		})
	})

	Describe("DeleteExpired", func() {
		It("should delete expired values", func() {
			c.Set("key1", "value1", time.Millisecond)
			Eventually(func() bool {
				_, ok := c.Get("key1")
				return ok
			}).Should(BeFalse())
		})

		It("should not delete non-expired values", func() {
			c.Set("key1", "value1", time.Minute)
			c.DeleteExpired()
			val, ok := c.Get("key1")
			Expect(ok).To(BeTrue())
			Expect(*val).To(Equal("value1"))
		})
	})

	Describe("Close", func() {
		JustAfterEach(func() {
			// give the regular AfterEach a cache to close
			c = util.NewCache[string](5 * time.Millisecond)
		})

		It("should stop the cleanup goroutine", func() {
			c.Set("key1", "value1", time.Millisecond)
			Eventually(func() bool {
				_, ok := c.Get("key1")
				return ok
			}).Should(BeFalse())
			c.Close()
			c.Set("key2", "value2", time.Minute)
			time.Sleep(10 * time.Millisecond)
			_, ok := c.Get("key2")
			Expect(ok).To(BeTrue())
		})
	})
})
