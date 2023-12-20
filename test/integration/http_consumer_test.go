package integration_test

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

type Message struct {
	topic   string
	data    []byte
	headers map[string]string
}

type TestHTTPConsumer struct {
	remote   *http.Server
	received chan *Message
}

func NewTestHTTPConsumer(addr string, logger *logrus.Logger) (*TestHTTPConsumer, error) {
	consumer := &TestHTTPConsumer{
		received: make(chan *Message, 10),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", consumer.handleRequest)

	consumer.remote = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		if err := consumer.remote.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Server error: %s", err)
		}
	}()

	return consumer, nil
}

func (c *TestHTTPConsumer) handleRequest(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "error reading body", http.StatusInternalServerError)
	}

	headers := make(map[string]string)
	for k, v := range r.Header {
		headers[strings.ToLower(k)] = v[0]
	}
	q := r.URL.Query()
	c.received <- &Message{
		topic:   telemetry.BuildTopicName(q.Get("namespace"), q.Get("type")),
		data:    bodyBytes,
		headers: headers,
	}
}

func (c *TestHTTPConsumer) NextMessage() (*Message, error) {
	if len(c.received) == 0 {
		return nil, fmt.Errorf("no http messages received")
	}
	return <-c.received, nil
}

func (c *TestHTTPConsumer) Close() {
	_ = c.remote.Close()
}
