package integration_test

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/teslamotors/fleet-telemetry/config"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter/noop"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/server/streaming"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

func configureLocalAWSEnvironment() {
	GinkgoHelper()
	// These values are test fixtures, never credentials from the host machine.
	GinkgoT().Setenv("AWS_ACCESS_KEY_ID", fakeAWSID)
	GinkgoT().Setenv("AWS_SECRET_ACCESS_KEY", fakeAWSSecret)
	GinkgoT().Setenv("AWS_SESSION_TOKEN", fakeAWSToken)
	GinkgoT().Setenv("AWS_REGION", fakeAWSRegion)
	GinkgoT().Setenv("AWS_EC2_METADATA_DISABLED", "true")
	GinkgoT().Setenv("AWS_SHARED_CREDENTIALS_FILE", "/dev/null")
	GinkgoT().Setenv("AWS_CONFIG_FILE", "/dev/null")
}

func localAWSSession() *session.Session {
	GinkgoHelper()
	sess, err := session.NewSession(aws.NewConfig().WithRegion(fakeAWSRegion).
		WithEndpoint(kinesisHost).WithCredentials(credentials.NewStaticCredentials(fakeAWSID, fakeAWSSecret, fakeAWSToken)).
		WithHTTPClient(&http.Client{Timeout: 5 * time.Second}).WithMaxRetries(0))
	Expect(err).NotTo(HaveOccurred())
	return sess
}

// startAWSDispatcherSocket exercises the real configuration, TLS, dispatcher and ACK paths.
func startAWSDispatcherSocket(conf *config.Config) (*websocket.Conn, *test.Hook) {
	GinkgoHelper()
	conf.MetricCollector = noop.NewCollector()
	conf.AckChan = make(chan *telemetry.Record)
	conf.TLS = &config.TLS{CAFile: caClient, ServerCert: "./test-certs/vehicle_device.app.cert", ServerKey: "./test-certs/vehicle_device.app.key"}
	logger, hook := logrus.NoOpLogger()
	handler := airbrake.NewAirbrakeHandler(nil)
	producers, rules, err := conf.ConfigureProducers(handler, logger, false)
	Expect(err).NotTo(HaveOccurred())
	registry := streaming.NewSocketRegistry()
	server, _, err := streaming.InitServer(conf, handler, rules, logger, registry)
	Expect(err).NotTo(HaveOccurred())
	tlsConfig, err := conf.ExtractServiceTLSConfig(logger)
	Expect(err).NotTo(HaveOccurred())
	cert, err := tls.LoadX509KeyPair(conf.TLS.ServerCert, conf.TLS.ServerKey)
	Expect(err).NotTo(HaveOccurred())
	tlsConfig.Certificates = []tls.Certificate{cert}
	srv := httptest.NewUnstartedServer(server.Handler)
	srv.TLS = tlsConfig
	srv.StartTLS()
	var conn *websocket.Conn
	DeferCleanup(func() {
		if conn != nil {
			_ = conn.Close()
		}
		for _, producer := range producers {
			Expect(producer.Close()).To(Succeed())
		}
		Eventually(registry.NumConnectedSockets, 5*time.Second).Should(BeZero())
		close(conf.AckChan) // Producers have stopped; let the server's ACK worker exit.
		srv.Close()
	})
	clientTLS, err := GetTLSConfig()
	Expect(err).NotTo(HaveOccurred())
	clientTLS.ServerName = "app"
	dialer := websocket.Dialer{TLSClientConfig: clientTLS, HandshakeTimeout: 5 * time.Second}
	headers := http.Header{"Version": []string{deviceClientVersion}, "X-Network-Interface": []string{"wifi"}}
	conn, _, err = dialer.Dial("wss"+strings.TrimPrefix(srv.URL, "https"), headers)
	Expect(err).NotTo(HaveOccurred())
	Eventually(registry.NumConnectedSockets, 5*time.Second).Should(Equal(1))
	return conn, hook
}

// dispatcherTestQueue owns a uniquely named queue and its idempotent cleanup.
type dispatcherTestQueue struct {
	client  *sqs.SQS
	name    string
	url     *string
	deleted bool
}

func createDispatcherQueue(client *sqs.SQS) *dispatcherTestQueue {
	GinkgoHelper()
	q := &dispatcherTestQueue{client: client, name: "fleet-telemetry-" + uuid.NewString()}
	out, err := client.CreateQueue(&sqs.CreateQueueInput{QueueName: aws.String(q.name)})
	Expect(err).NotTo(HaveOccurred())
	q.url = out.QueueUrl
	DeferCleanup(q.delete)
	return q
}

func (q *dispatcherTestQueue) delete() {
	GinkgoHelper()
	if q.deleted {
		return
	}
	_, err := q.client.DeleteQueue(&sqs.DeleteQueueInput{QueueUrl: q.url})
	Expect(err).NotTo(HaveOccurred())
	q.deleted = true
}

func (q *dispatcherTestQueue) receive() *sqs.Message {
	GinkgoHelper()
	var message *sqs.Message
	Eventually(func() error {
		out, err := q.client.ReceiveMessage(&sqs.ReceiveMessageInput{
			QueueUrl: q.url, MaxNumberOfMessages: aws.Int64(1), WaitTimeSeconds: aws.Int64(1),
			MessageAttributeNames: aws.StringSlice([]string{"All"}),
		})
		if err != nil {
			return err
		}
		if len(out.Messages) == 0 {
			return fmt.Errorf("queue %s is empty", q.name)
		}
		message = out.Messages[0]
		return nil
	}, 10*time.Second, 100*time.Millisecond).Should(Succeed())
	return message
}

func expectVehicleQueueMessage(message *sqs.Message) {
	GinkgoHelper()
	decoded, err := base64.StdEncoding.DecodeString(aws.StringValue(message.Body))
	Expect(err).NotTo(HaveOccurred())
	VerifyMessageBody(decoded, vehicleName)
	for key, want := range map[string]string{
		"txtype": "V", "txid": txid, "vin": deviceID, "device_client_version": deviceClientVersion,
	} {
		Expect(message.MessageAttributes).To(HaveKey(key))
		Expect(aws.StringValue(message.MessageAttributes[key].StringValue)).To(Equal(want), "attribute %s", key)
	}
}

func expectVehicleAck(conn *websocket.Conn) {
	GinkgoHelper()
	Expect(conn.SetReadDeadline(time.Now().Add(5 * time.Second))).To(Succeed())
	verifyAckMessage(conn, "V")
}

// First observe the actual publish error so a slow publish cannot make this pass.
// The read timeout leaves this connection unusable; callers only close it afterward.
func expectPublishFailureWithoutAck(conn *websocket.Conn, hook *test.Hook, errorMessage string) {
	GinkgoHelper()
	Eventually(func() bool {
		for _, entry := range hook.AllEntries() {
			if entry.Message == errorMessage {
				return true
			}
		}
		return false
	}, 5*time.Second).Should(BeTrue(), "expected dispatcher error %s before checking ACK absence", errorMessage)
	Expect(conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))).To(Succeed())
	_, _, err := conn.ReadMessage()
	Expect(err).To(HaveOccurred())
	networkError, ok := err.(net.Error)
	Expect(ok).To(BeTrue())
	Expect(networkError.Timeout()).To(BeTrue(), "no successful ACK should arrive after a rejected publish")
}
