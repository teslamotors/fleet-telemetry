package lib

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	// CertValidity 5 years in hours
	CertValidity = 5 * 365 * 24
)

// TestCertAndKey is a struct to store cert info.
type TestCertAndKey struct {
	CertFile   string
	KeyFile    string
	Cert       *x509.Certificate
	privateKey *rsa.PrivateKey
}

// SaveCerts stores cert and key files into a given dir.
func (t *TestCertAndKey) SaveCerts(directory, name string) {
	if directory != "" {
		moveFile(t.CertFile, directory+name+".cert")
		moveFile(t.KeyFile, directory+name+".key")
	}
}

// GenerateServerTestKeyAndCert generates a test server cert/key given a signing CA.
func GenerateServerTestKeyAndCert(commonName string, sanDomains []string, sanIPs []string, parent *TestCertAndKey) (*TestCertAndKey, error) {
	return GenerateServerTestKeyAndCertWithDate(commonName, sanDomains, sanIPs, parent, time.Now())
}

// GenerateServerTestKeyAndCertWithDate generates a test server cert/key given a signing CA and given validity date
func GenerateServerTestKeyAndCertWithDate(commonName string, sanDomains []string, sanIPs []string, parent *TestCertAndKey, notBefore time.Time) (*TestCertAndKey, error) {
	privateKey, serialNumber, err := GeneratePrivateKeyAndSerialNumber()
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{"Test Co"},
		},
		NotBefore: notBefore,
		NotAfter:  notBefore.Add(time.Duration(CertValidity) * time.Hour),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}
	template.DNSNames = append(template.DNSNames, commonName)
	for _, name := range sanDomains {
		template.DNSNames = append(template.DNSNames, name)
	}

	for _, ip := range sanIPs {
		template.IPAddresses = append(template.IPAddresses, net.ParseIP(ip))
	}

	return makeTestCertAndKey(privateKey, &template, parent.Cert, parent.privateKey)
}

// GenerateRootSigningCert generates cert and key for a signing CA
func GenerateRootSigningCert(commonName string, organizationalUnit []string) (*TestCertAndKey, error) {
	privateKey, serialNumber, err := GeneratePrivateKeyAndSerialNumber()
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:         commonName,
			Organization:       []string{"Test Co"},
			OrganizationalUnit: organizationalUnit,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Duration(CertValidity) * time.Hour),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	return makeTestCertAndKey(privateKey, &template, &template, privateKey)
}

// GenerateClientTestKeyAndCert generates a test client cert/key given a signing CA
func GenerateClientTestKeyAndCert(commonName string, parent *TestCertAndKey) (*TestCertAndKey, error) {
	return GenerateClientTestKeyAndCertWithDate(commonName, parent, time.Now())
}

// GenerateClientTestKeyAndCertWithDate generates a test client cert/key given a signing CA give a start date
func GenerateClientTestKeyAndCertWithDate(commonName string, parent *TestCertAndKey, notBefore time.Time) (*TestCertAndKey, error) {
	privateKey, serialNumber, err := GeneratePrivateKeyAndSerialNumber()
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{"Test Co"},
		},
		NotBefore: notBefore,
		NotAfter:  notBefore.Add(time.Duration(CertValidity) * time.Hour),

		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	if strings.HasPrefix(commonName, "board") {
		template.Subject.OrganizationalUnit = []string{"Tesla Motors SN"}
	}

	return makeTestCertAndKey(privateKey, &template, parent.Cert, parent.privateKey)
}

// GenerateClientTestKeyAndCertWithOU generates a test client cert/key given a signing CA
func GenerateClientTestKeyAndCertWithOU(commonName, organization, organizationalUnit string, parent *TestCertAndKey) (*TestCertAndKey, error) {
	privateKey, serialNumber, err := GeneratePrivateKeyAndSerialNumber()
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:         commonName,
			Organization:       []string{organization},
			OrganizationalUnit: []string{organizationalUnit},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Duration(CertValidity) * time.Hour),

		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	return makeTestCertAndKey(privateKey, &template, parent.Cert, parent.privateKey)
}

// GeneratePrivateKeyAndSerialNumber generates a private key and serial number
func GeneratePrivateKeyAndSerialNumber() (*rsa.PrivateKey, *big.Int, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	serialNumber, err := GenerateSerialNumber()
	if err != nil {
		return nil, nil, err
	}

	return privateKey, serialNumber, nil
}

// GenerateSerialNumber creates a serial number.
func GenerateSerialNumber() (*big.Int, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}

	return serialNumber, nil
}

func makeTestCertAndKey(privateKey *rsa.PrivateKey, template *x509.Certificate, parent *x509.Certificate, parentPrivateKey *rsa.PrivateKey) (*TestCertAndKey, error) {
	derBytes, err := x509.CreateCertificate(rand.Reader, template, parent, &privateKey.PublicKey, parentPrivateKey)
	if err != nil {
		return nil, err
	}

	fullCert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, err
	}

	certOut, err := os.CreateTemp(os.TempDir(), "cert")
	if err != nil {
		return nil, err
	}
	_ = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	_ = certOut.Close()

	keyOut, err := os.CreateTemp(os.TempDir(), "key")
	if err != nil {
		return nil, err
	}

	_ = pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	_ = keyOut.Close()

	return &TestCertAndKey{CertFile: certOut.Name(), KeyFile: keyOut.Name(), Cert: fullCert, privateKey: privateKey}, nil
}

func moveFile(oldFilePath, newFilePath string) {
	copyFile(oldFilePath, newFilePath)
	if err := os.Remove(oldFilePath); err != nil {
		log.Fatal(err)
	}
	wd, _ := os.Getwd()
	relPath, err := filepath.Rel(wd, newFilePath)
	if err != nil {
		relPath = newFilePath
	}
	log.Printf("wrote %s", relPath)
}

func copyFile(oldFilePath, newFilePath string) {
	in, err := os.Open(oldFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(newFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, in)
	if err != nil {
		log.Fatal(err)
	}
	_ = out.Close()
}
