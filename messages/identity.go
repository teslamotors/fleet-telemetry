package messages

import (
	"crypto/x509"
	"errors"
	"fmt"
	"strings"
)

var (

	// ErrUnauthorizedCert returned if cert cannot be turned into identity
	ErrUnauthorizedCert = errors.New("unauthorized certificate")

	// ErrExcludedCert returned if cert is in excludelist
	ErrExcludedCert = errors.New("excluded certificate")
)

var (
	knownIssuers = map[string]string{
		"TeslaMotors":              "vehicle_device",
		"Tesla Issuing CA":         "vehicle_device",
		"Tesla Motors Products CA": "vehicle_device",
	}

	knownOIDIssuers = map[string]struct{}{
		"Tesla Motors Product Issuing CA":           {},
		"Tesla Motors Product RSA Issuing CA":       {},
		"Tesla Motors GF3 Product Issuing CA":       {},
		"Tesla Motors GF3 Product RSA Issuing CA":   {},
		"Tesla Energy Product Issuing CA":           {},
		"Tesla Energy GF0 Product Issuing CA":       {},
		"Tesla Motors GF0 Product Issuing CA":       {},
		"Tesla Motors GF Austin Product Issuing CA": {},
		"Tesla Motors GF Berlin Product Issuing CA": {},
		"Tesla Product Access Issuing CA":           {},
		"Tesla China Product Access Issuing CA":     {},
		"Tesla GF0 Product Access Issuing CA":       {},
		"Tesla GF3 Product Access Issuing CA":       {},
		"Tesla GF Austin Product Access Issuing CA": {},
		"Tesla GF Berlin Product Access Issuing CA": {},
	}

	oidMap = map[string]string{
		// Eng
		"1.3.6.1.4.1.49279.2.4.1":  "vehicle_device",
		"1.3.6.1.4.1.49279.2.4.11": "vehicle_board_device",
		"1.3.6.1.4.1.49279.2.4.2":  "energy_device",
		"1.3.6.1.4.1.49279.2.4.12": "das_device",
		"1.3.6.1.4.1.49279.2.4.14": "vehicle_service",
		"1.3.6.1.4.1.49279.2.4.15": "das_service",
		"1.3.6.1.4.1.49279.2.4.16": "energy_service",
		"1.3.6.1.4.1.49279.2.4.28": "robotics_device",

		// Prod
		"1.3.6.1.4.1.49279.2.5.1":  "vehicle_device",
		"1.3.6.1.4.1.49279.2.5.11": "vehicle_board_device",
		"1.3.6.1.4.1.49279.2.5.2":  "energy_device",
		"1.3.6.1.4.1.49279.2.5.12": "das_device",
		"1.3.6.1.4.1.49279.2.5.14": "vehicle_service",
		"1.3.6.1.4.1.49279.2.5.15": "das_service",
		"1.3.6.1.4.1.49279.2.5.16": "energy_service",
		"1.3.6.1.4.1.49279.2.5.28": "robotics_device",
	}

	ouMap = map[string]string{
		"Solar Inverter":      "solar_inverter_device",
		"Gen3 Wall Connector": "wall_connector_device",
	}

	// list of oids that we won't create identities for
	excludedOIDs = map[string]string{
		"1.3.6.1.4.1.49279.2.5.1.1": "WifiEapCertOID",
	}
)

// CreateIdentityFromCert given the X509 Cert return a deviceID
func CreateIdentityFromCert(fullCert *x509.Certificate) (clientType, deviceID string, err error) {
	deviceID = strings.Replace(fullCert.Subject.CommonName, ".", "-", -1)
	if _, ok := knownOIDIssuers[fullCert.Issuer.CommonName]; ok {
		return createIdentifyFromOID(fullCert, deviceID)
	}

	var ok bool
	clientType, ok = knownIssuers[fullCert.Issuer.CommonName]
	if !ok {
		return "", "", ErrUnauthorizedCert
	}

	if fullCert.Issuer.CommonName == "Tesla Motors Products CA" && len(fullCert.Subject.OrganizationalUnit) > 0 &&
		fullCert.Subject.OrganizationalUnit[0] == "Tesla Motors SN" {
		clientType = "vehicle_board_device"
	}

	return clientType, deviceID, nil
}

func createIdentifyFromOID(fullCert *x509.Certificate, deviceID string) (string, string, error) {
	for _, oid := range fullCert.UnknownExtKeyUsage {
		oidStr := oid.String()
		if _, ok := excludedOIDs[oidStr]; ok {
			return "", "", ErrExcludedCert
		}

		if clientType, ok := oidMap[oidStr]; ok {
			if ok, overrideClientType, overrideDeviceID := createIdentityFromOU(fullCert, deviceID); ok {
				return overrideClientType, overrideDeviceID, nil
			}

			return clientType, deviceID, nil
		}
	}

	return "", "", ErrUnauthorizedCert
}

func createIdentityFromOU(fullCert *x509.Certificate, deviceID string) (bool, string, string) {
	for _, ou := range fullCert.Subject.OrganizationalUnit {
		if clientType, ok := ouMap[ou]; ok {
			return true, clientType, deviceID
		}
	}

	return false, "", ""
}

// BuildClientID given an org unit and a device id, return clientID in dot notation
func BuildClientID(clientType, deviceID string) string {
	return fmt.Sprintf("%s.%s", clientType, deviceID)
}

// ParseSenderID splits a sender id of the form client_type.device_id into its constituent parts
func ParseSenderID(senderID string) (string, string) {
	parts := strings.SplitN(senderID, ".", 2)
	if len(parts) < 2 {
		return parts[0], ""
	}

	return parts[0], parts[1]
}
