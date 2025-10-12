package browser

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/glitchedgitz/grroxy-db/certs"
	"github.com/glitchedgitz/grroxy-db/save"
)

func GenerateCert(configPath string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	certs, err := certs.New(&certs.Options{
		CacheSize: 256,
		Directory: path.Join(homeDir, ".config", "grroxy"),
	})
	if err != nil {
		return "", err
	}
	_, ca := certs.GetCA()
	reader := bytes.NewReader(ca)
	bf := bufio.NewReader(reader)
	respbody, err := io.ReadAll(bf)

	filePath := path.Join(configPath, "cacert.crt")
	save.WriteFile(filePath, respbody)

	if err != nil {
		return "", err
	}

	return filePath, nil
}

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
