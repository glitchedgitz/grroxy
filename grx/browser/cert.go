package browser

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
)

// GetSPKIFingerprint calculates the SHA-256 SPKI fingerprint of a certificate
// This is used by Chrome to ignore certificate errors for our CA
func GetSPKIFingerprint(certPath string) (string, error) {
	// Read the certificate file
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return "", fmt.Errorf("failed to read certificate file: %v", err)
	}

	// Decode the PEM block
	block, _ := pem.Decode(certData)
	if block == nil || block.Type != "CERTIFICATE" {
		return "", fmt.Errorf("failed to decode PEM block containing certificate")
	}

	// Parse the certificate
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse certificate: %v", err)
	}

	// Calculate SHA-256 hash of the SubjectPublicKeyInfo
	spkiHash := sha256.Sum256(cert.RawSubjectPublicKeyInfo)

	// Convert hash to base64 encoded string (the format Chrome expects)
	fingerprint := base64.StdEncoding.EncodeToString(spkiHash[:])

	return fingerprint, nil
}
