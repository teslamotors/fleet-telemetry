package streaming

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/teslamotors/fleet-telemetry/config"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

func TestRespondToVehicle_DoesNotBlockWhenWriterDone(t *testing.T) {
	logger, _ := logrus.NoOpLogger()
	reqID := &telemetry.RequestIdentity{DeviceID: "test-device", SenderID: "vehicle_device.test"}
	conf := &config.Config{
		MetricCollector: nil,
	}

	sm := NewSocketManager(context.Background(), reqID, nil, conf, logger)

	// Fill writeChan buffer to full capacity (1000 items)
	for i := 0; i < 1000; i++ {
		sm.writeChan <- SocketMessage{MsgType: 2, Txid: "test", Msg: []byte("payload")}
	}
	if len(sm.writeChan) != 1000 {
		t.Fatalf("expected writeChan to be full (1000 items), got %d", len(sm.writeChan))
	}

	// Simulate writer exiting and closing writerDone
	close(sm.writerDone)

	// Calling respondToVehicle now must not block on full writeChan
	done := make(chan struct{})
	go func() {
		rec := &telemetry.Record{Txid: "overflow-tx", TxType: "test"}
		sm.respondToVehicle(rec, errors.New("test error"))
		close(done)
	}()

	select {
	case <-done:
		// Success: returned without blocking
	case <-time.After(500 * time.Millisecond):
		t.Fatal("respondToVehicle blocked on full writeChan after writer exited")
	}

	if len(sm.writeChan) != 1000 {
		t.Fatalf("expected writeChan to remain at 1000 items, got %d", len(sm.writeChan))
	}
}

func TestTryRespondToVehicle_DoesNotBlockOnFullBufferWithLiveWriter(t *testing.T) {
	logger, _ := logrus.NoOpLogger()
	reqID := &telemetry.RequestIdentity{DeviceID: "test-device", SenderID: "vehicle_device.test"}
	conf := &config.Config{
		MetricCollector: nil,
	}

	sm := NewSocketManager(context.Background(), reqID, nil, conf, logger)

	// Fill writeChan buffer to full capacity (1000 items)
	for i := 0; i < 1000; i++ {
		sm.writeChan <- SocketMessage{MsgType: 2, Txid: "test", Msg: []byte("payload")}
	}
	if len(sm.writeChan) != 1000 {
		t.Fatalf("expected writeChan to be full (1000 items), got %d", len(sm.writeChan))
	}

	// Writer is still alive (writerDone not closed), but buffer is full
	rec := &telemetry.Record{Txid: "overflow-tx", TxType: "test"}

	done := make(chan bool)
	go func() {
		ok := sm.tryRespondToVehicle(rec, nil)
		done <- ok
	}()

	select {
	case ok := <-done:
		if ok {
			t.Fatal("expected tryRespondToVehicle to return false on full buffer")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("tryRespondToVehicle blocked on full buffer with live writer")
	}
}
