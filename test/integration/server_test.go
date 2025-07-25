package integration_test

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/websocket"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/teslamotors/fleet-telemetry/messages/tesla"
	"github.com/teslamotors/fleet-telemetry/protos"
)

const (
	txid                = "integration-test-txid"
	messageID           = "integration-test-message-id"
	senderID            = "vehicle_device.device-1"
	deviceType          = "vehicle_device"
	deviceClientVersion = "1.0.0"

	serviceURL    = "app:4443"
	statusURL     = "app:8080"
	profilerURL   = "app:4269"
	prometheusURL = "app:9090"

	clientCert = "./test-certs/vehicle_device.device-1.cert"
	clientKey  = "./test-certs/vehicle_device.device-1.key"
	caClient   = "./test-certs/vehicle_device.CA.cert"
)

func GenerateVehicleMessage(id, vehicleName, location string, timestamp *timestamppb.Timestamp) []byte {
	senderID := fmt.Sprintf("%s.%s", deviceType, id)
	return tesla.FlatbuffersStreamToBytes([]byte(senderID), []byte("V"), []byte(txid), generatePayload(vehicleName, location, timestamp), 1, []byte(messageID), []byte(deviceType), []byte(id), uint64(time.Now().UnixMilli()))
}

func generatePayload(vehicleName, location string, timestamp *timestamppb.Timestamp) []byte {
	var data []*protos.Datum
	data = append(data, &protos.Datum{
		Key: protos.Field_VehicleName,
		Value: &protos.Value{
			Value: &protos.Value_StringValue{
				StringValue: vehicleName,
			},
		},
	}, &protos.Datum{
		Key: protos.Field_Location,
		Value: &protos.Value{
			Value: &protos.Value_StringValue{
				StringValue: location,
			},
		},
	})
	payload, err := proto.Marshal(&protos.Payload{
		Data:      data,
		CreatedAt: timestamp,
	})
	Expect(err).NotTo(HaveOccurred())
	return payload
}

// CreateWebSocket creates a websocket with https protocol
func CreateWebSocket(tlsConfig *tls.Config) *websocket.Conn {
	u := url.URL{Scheme: "wss", Host: serviceURL, Path: "/"}

	tlsDialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
		TLSClientConfig:  tlsConfig,
	}
	headers := http.Header{}
	headers.Add("X-Network-Interface", "wifi")
	headers.Add("Version", deviceClientVersion)
	c, _, err := tlsDialer.Dial(u.String(), headers)
	Expect(err).NotTo(HaveOccurred())
	return c
}

// GetTLSConfig returns a TLSConfig object from cert, key and optional client chain files.
func GetTLSConfig() (*tls.Config, error) {
	var cert tls.Certificate
	certFilePath, err := filepath.Abs(clientCert)
	if err != nil {
		return nil, err
	}
	keyFilePath, err := filepath.Abs(clientKey)
	if err != nil {
		return nil, err
	}
	caFilePath, err := filepath.Abs(caClient)
	if err != nil {
		return nil, err
	}

	cert, err = tls.LoadX509KeyPair(certFilePath, keyFilePath)
	if err != nil {
		return nil, fmt.Errorf("can't properly load cert pair (%s, %s): %s", certFilePath, keyFilePath, err.Error())
	}

	clientCertPool := x509.NewCertPool()
	if caFilePath != "" {
		clientCACert, err := os.ReadFile(caFilePath)
		if err != nil {
			return nil, fmt.Errorf("can't properly load ca cert (%s): %s", caFilePath, err.Error())
		}
		_ = clientCertPool.AppendCertsFromPEM(clientCACert)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      clientCertPool,
	}

	return tlsConfig, nil
}
