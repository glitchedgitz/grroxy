package rawproxy

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// GenerateMITMCA generates a new MITM CA certificate and private key
func GenerateMITMCA(dir string) (*MitmCA, string, string, error) {
	// Generate a new RSA key for CA
	caKey, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		return nil, "", "", err
	}
	serial := big.NewInt(0).SetUint64(uint64(time.Now().UnixNano()))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization:       []string{"GRROXY CA"},
			OrganizationalUnit: []string{"MITM"},
			CommonName:         "GRROXY MITM CA",
		},
		NotBefore:             time.Now().Add(-10 * time.Minute),
		NotAfter:              time.Now().AddDate(5, 0, 0),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	// Set Subject Key Identifier (recommended for CA)
	pubKeyDER, err := x509.MarshalPKIXPublicKey(&caKey.PublicKey)
	if err == nil {
		h := sha1.Sum(pubKeyDER)
		tmpl.SubjectKeyId = h[:]
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, "", "", err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(caKey)})

	certPath := filepath.Join(dir, "ca.crt")
	keyPath := filepath.Join(dir, "ca.key")
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return nil, "", "", err
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return nil, "", "", err
	}

	s, err := LoadMITMCA(certPath, keyPath)
	if err != nil {
		return nil, "", "", err
	}
	return s, certPath, keyPath, nil
}

// FileExists checks if a file exists
func FileExists(p string) bool {
	if _, err := os.Stat(p); err == nil {
		return true
	}
	return false
}
