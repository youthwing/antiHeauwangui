// Package ca generates and persists the MITM root CA. The first run creates
// a fresh CA, encrypts the private key with DPAPI, and stores both on disk;
// subsequent runs load the same CA so the Windows trust prompt only ever
// shows up once.
package ca

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// CA bundles a long-lived root CA: a self-signed cert + RSA-2048 private key.
type CA struct {
	Cert    *x509.Certificate
	DER     []byte // raw cert DER
	PEM     []byte // PEM-encoded cert
	PrivKey *rsa.PrivateKey
}

const (
	certFile = "ca.crt"
	keyFile  = "ca.key.dpapi" // private key, DPAPI-encrypted
)

// Generate creates a fresh CA valid for 5 years. Intended to be persisted.
func Generate() (*CA, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate ca key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "wangui-tokengrab persistent CA",
			Organization: []string{"wangui internal"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().AddDate(5, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &priv.PublicKey, priv)
	if err != nil {
		return nil, fmt.Errorf("create ca cert: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parse ca cert: %w", err)
	}
	return &CA{
		Cert:    cert,
		DER:     der,
		PEM:     pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		PrivKey: priv,
	}, nil
}

// LoadOrGenerate reads a previously-saved CA from dir. If neither file is
// present (or either fails to decode), generates a fresh CA and writes it
// back to dir. The second return value is true when a new CA was generated
// — caller should then call Install() to add it to the Windows trust store.
func LoadOrGenerate(dir string) (*CA, bool, error) {
	c, err := Load(dir)
	if err == nil {
		return c, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		// Files exist but are corrupted / undecryptable. Overwrite.
		// (Common reason: user copied profile across machines, DPAPI key
		// differs, decrypt fails.)
	}
	c, err = Generate()
	if err != nil {
		return nil, false, err
	}
	if err := c.Save(dir); err != nil {
		return nil, false, fmt.Errorf("save ca: %w", err)
	}
	return c, true, nil
}

// Load reads ca.crt + ca.key.dpapi from dir.
func Load(dir string) (*CA, error) {
	crtBytes, err := os.ReadFile(filepath.Join(dir, certFile))
	if err != nil {
		return nil, err
	}
	certBlock, _ := pem.Decode(crtBytes)
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("ca.crt: invalid PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse ca cert: %w", err)
	}

	keyEnc, err := os.ReadFile(filepath.Join(dir, keyFile))
	if err != nil {
		return nil, err
	}
	keyPEM, err := dpapiDecrypt(keyEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt ca key: %w", err)
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil || keyBlock.Type != "RSA PRIVATE KEY" {
		return nil, fmt.Errorf("ca.key: invalid PEM")
	}
	priv, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse ca key: %w", err)
	}
	return &CA{
		Cert:    cert,
		DER:     certBlock.Bytes,
		PEM:     crtBytes,
		PrivKey: priv,
	}, nil
}

// Save writes the CA to dir. Creates dir if missing.
func (c *CA) Save(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir %q: %w", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, certFile), c.PEM, 0o600); err != nil {
		return err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(c.PrivKey),
	})
	enc, err := dpapiEncrypt(keyPEM)
	if err != nil {
		return fmt.Errorf("encrypt ca key: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, keyFile), enc, 0o600)
}

// Erase removes ca.crt + ca.key.dpapi from dir. Used by the explicit
// "uninstall persistent CA" path in the UI.
func Erase(dir string) error {
	_ = os.Remove(filepath.Join(dir, certFile))
	_ = os.Remove(filepath.Join(dir, keyFile))
	return nil
}
