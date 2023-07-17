package main

import (
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/teslamotors/fleet-telemetry/messages"
	"github.com/teslamotors/fleet-telemetry/tools/lib"
)

const (
	testCertsDirectory = "/test/integration/test-certs/"
)

var (
	clientCaName string

	serverIDs string
	clientIDs string

	directory  string
	rootCAName string

	sANIPAddress  string
	sANDomainName string
)

func main() {
	currentDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	loadFlag(currentDir + testCertsDirectory)
	flag.Parse()

	generateCertificateBundle()
}

func getCertFilename(cert *x509.Certificate) string {
	clientType, clientName, _ := messages.CreateIdentityFromCert(cert)
	if clientType == "" {
		log.Fatalf("invalid certificate type: %v", cert.Issuer)
	}
	clientType = strings.TrimSuffix(clientType, " CA")
	clientType = strings.Replace(strings.ToLower(clientType), " ", "_", -1)
	if clientName == "" {
		return clientType
	}
	if strings.HasSuffix(clientName, " CA") {
		clientName = "CA"
	}
	return fmt.Sprintf("%s.%s", clientType, clientName)
}

func generateCertificateBundle() {
	log.Printf("Creating Root Signing Cert for: %s", clientCaName)
	/***** Create Root CA certificate ****/
	rootCA, err := lib.GenerateRootSigningCert(clientCaName, nil)
	if err != nil {
		log.Fatal(err)
	}

	sanIPs := strings.Split(sANIPAddress, ",")
	sanDomains := strings.Split(sANDomainName, ",")

	for _, iServerID := range strings.Split(serverIDs, ",") {
		serverCert, err := lib.GenerateServerTestKeyAndCert(iServerID, sanDomains, sanIPs, rootCA)
		if err != nil {
			log.Fatal(err)
		}
		serverCert.SaveCerts(directory, getCertFilename(serverCert.Cert))
	}

	/***** Create client certificate ****/
	for _, iClientID := range strings.Split(clientIDs, ",") {
		clientCert, err := lib.GenerateClientTestKeyAndCert(iClientID, rootCA)
		if err != nil {
			log.Fatal(err)
		}
		clientCert.SaveCerts(directory, getCertFilename(clientCert.Cert))
	}

	rootCAName = getCertFilename(rootCA.Cert)
	rootCA.SaveCerts(directory, rootCAName)
}

func loadFlag(path string) {
	flag.StringVar(&sANIPAddress, "san-ip-address", "", "IP Address for the Subject Alternative Name, comma delimited list accepted")
	flag.StringVar(&sANDomainName, "san-domain", "", "DNS name for the Subject Alternative Name, comma delimited list accepted")
	flag.StringVar(&clientCaName, "client-ca-name", "Tesla Motors Products CA", "this will determine the clientType for the client certificate")
	flag.StringVar(&serverIDs, "server-ids", "app", "serverIDs of the generated certificate (can be CSV for multiple)")
	flag.StringVar(&clientIDs, "client-ids", "device-1", "clientIDs of the generated certificate (can be CSV for multiple certs)")
	flag.StringVar(&directory, "directory", path, "directory to store certs")
	flag.IntVar(&lib.CertValidity, "cert-validity", 365*24, "certificate validity in hours")
}
