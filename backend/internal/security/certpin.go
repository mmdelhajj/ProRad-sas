package security

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"time"
)

// CertPinningMode controls how certificate pinning behaves
// "strict" = block on mismatch, "warn" = log warning but allow, "disabled" = no check
var CertPinningMode = "warn"

// Pinned certificate fingerprints for license server
// These are SHA-256 hashes of the server's public key (SPKI)
var pinnedFingerprints = []string{
	"894384e149285334fbae471b4b01b80030f954a004613e0293bb2bf550db5829", // license.proxrad.com (Cloudflare)
}

// LicenseServerHost is the expected hostname
const LicenseServerHost = "license.proxrad.com"

// CreatePinnedHTTPClient creates an HTTP client with certificate pinning
func CreatePinnedHTTPClient() *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
			VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
				// If pinning disabled or no fingerprints, skip
				if CertPinningMode == "disabled" || len(pinnedFingerprints) == 0 {
					return nil
				}

				for _, rawCert := range rawCerts {
					cert, err := x509.ParseCertificate(rawCert)
					if err != nil {
						continue
					}

					// Calculate SPKI fingerprint
					fingerprint := calculateSPKIFingerprint(cert)

					// Check against pinned fingerprints
					for _, pinned := range pinnedFingerprints {
						if fingerprint == pinned {
							return nil // Match found
						}
					}
				}

				// Certificate mismatch detected
				if CertPinningMode == "warn" {
					log.Println("WARNING: License server certificate fingerprint mismatch - possible MITM attack")
					return nil // Allow connection but log warning
				}

				return errors.New("certificate pinning validation failed - possible MITM attack")
			},
		},
		DisableKeepAlives: false,
		MaxIdleConns:      10,
		IdleConnTimeout:   30 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
}

// calculateSPKIFingerprint calculates SHA-256 fingerprint of certificate's public key
func calculateSPKIFingerprint(cert *x509.Certificate) string {
	// Hash the Subject Public Key Info
	spkiHash := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return hex.EncodeToString(spkiHash[:])
}

// GetCertificateFingerprint connects to a server and returns its certificate fingerprint
// Use this to get the fingerprint to add to pinnedFingerprints
func GetCertificateFingerprint(host string) (string, error) {
	conn, err := tls.Dial("tcp", host+":443", &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return "", err
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return "", errors.New("no certificates found")
	}

	fingerprint := calculateSPKIFingerprint(certs[0])
	return fingerprint, nil
}

// ValidateServerCertificate validates a server's certificate against pinned fingerprints
func ValidateServerCertificate(host string) error {
	if len(pinnedFingerprints) == 0 {
		return nil // No pinning configured
	}

	fingerprint, err := GetCertificateFingerprint(host)
	if err != nil {
		return err
	}

	for _, pinned := range pinnedFingerprints {
		if fingerprint == pinned {
			return nil
		}
	}

	return errors.New("certificate fingerprint mismatch")
}
