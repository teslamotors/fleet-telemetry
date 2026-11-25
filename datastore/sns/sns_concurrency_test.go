package sns

import (
	"sync"
	"testing"
	"testing/synctest"

	"github.com/aws/aws-sdk-go/aws"
	awssns "github.com/aws/aws-sdk-go/service/sns"
	. "github.com/onsi/gomega"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// synctest.Test needs *testing.T; keep deterministic blocked-operation tests
// outside the Ginkgo suite so each owns its isolated clock and goroutines.
func TestCloseCancelsRequestsAndBlockedAcknowledgements(t *testing.T) {
	for _, blocked := range []string{"request", "ack"} {
		t.Run(blocked, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				g := NewWithT(t)
				client := &fakeClient{send: func(ctx aws.Context, _ *awssns.PublishInput) (*awssns.PublishOutput, error) {
					if blocked == "request" {
						<-ctx.Done()
						return nil, ctx.Err()
					}
					return &awssns.PublishOutput{}, nil
				}}
				p := testProducer(t, client, make(chan *telemetry.Record), defaultProducerSettings())
				done := make(chan struct{})
				go func() { p.Produce(record()); close(done) }()
				// In the ACK case the fake SDK cannot block, so Produce must be
				// waiting for an ACK receiver once the bubble is durably blocked.
				synctest.Wait()
				g.Expect(client.publishedRequests()).To(HaveLen(1))
				g.Expect(done).NotTo(BeClosed())
				closed := make(chan struct{})

				go func() { _ = p.Close(); _ = p.Close(); close(closed) }()
				synctest.Wait()

				g.Expect(closed).To(BeClosed())
				g.Expect(done).To(BeClosed())
				p.Produce(record())
				g.Expect(client.publishedRequests()).To(HaveLen(1), "Produce after Close must not call the SDK")
			})
		})
	}
}

func TestConcurrentProduceAndClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g := NewWithT(t)
		client := &fakeClient{}
		p := testProducer(t, client, make(chan *telemetry.Record, 100), defaultProducerSettings())
		var workers sync.WaitGroup

		for i := 0; i < 50; i++ {
			workers.Add(1)
			go func() { defer workers.Done(); p.Produce(record()) }()
		}
		for i := 0; i < 5; i++ {
			workers.Add(1)
			go func() { defer workers.Done(); _ = p.Close() }()
		}
		workers.Wait()
		synctest.Wait()

		accepted := len(client.publishedRequests())
		g.Expect(accepted).To(BeNumerically("<=", 50))
		p.Produce(record())
		g.Expect(client.publishedRequests()).To(HaveLen(accepted), "closed producer must not accept more requests")
	})
}
