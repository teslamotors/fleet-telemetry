package sqs

import (
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/aws/aws-sdk-go/aws"
	awssqs "github.com/aws/aws-sdk-go/service/sqs"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// These lifecycle tests use standard testing because synctest requires *testing.T
// and an isolated goroutine bubble to prove that an ACK send is durably blocked.
func TestCloseCancelsRequestsAndBlockedAcknowledgements(t *testing.T) {
	for _, blocked := range []string{"request", "ack"} {
		t.Run(blocked, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var sends atomic.Int32
				f := &fakeClient{send: func(ctx aws.Context, _ *awssqs.SendMessageInput) (*awssqs.SendMessageOutput, error) {
					sends.Add(1)
					if blocked == "request" {
						<-ctx.Done()
						return nil, ctx.Err()
					}
					return &awssqs.SendMessageOutput{}, nil
				}}
				p := makeTestProducer(f, make(chan *telemetry.Record), defaultProducerSettings())
				t.Cleanup(func() { _ = p.Close() })

				done := make(chan struct{})
				go func() { p.Produce(record()); close(done) }()
				// Wait until Produce is durably blocked. The successful fake API has no
				// blocking operations, so its ACK case can only be waiting for an ACK receiver.
				synctest.Wait()
				if sends.Load() != 1 {
					t.Fatal("publish did not reach SDK")
				}
				select {
				case <-done:
					t.Fatal("Produce returned before Close released its blocked operation")
				default:
				}
				closed := make(chan struct{})
				go func() { _ = p.Close(); _ = p.Close(); close(closed) }()
				synctest.Wait()
				select {
				case <-closed:
				default:
					t.Fatal("Close did not release blocked operation")
				}
				select {
				case <-done:
				default:
					t.Fatal("Produce did not finish after Close")
				}
				p.Produce(record())
				if sends.Load() != 1 {
					t.Fatal("request accepted after Close")
				}
			})
		})
	}
}
func TestConcurrentProduceAndClose(t *testing.T) {
	p := makeTestProducer(&fakeClient{}, make(chan *telemetry.Record, 100), defaultProducerSettings())
	t.Cleanup(func() { _ = p.Close() })

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
}
