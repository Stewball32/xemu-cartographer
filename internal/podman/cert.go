package podman

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// generateXemuCerts writes a CA cert + a server cert (signed by the CA) into
// dir, in the layout linuxserver/xemu's nginx expects:
//
//   - dir/cert.pem  — server leaf cert + CA cert concatenated (PEM bundle)
//   - dir/cert.key  — server private key
//   - dir/ca.pem    — CA cert alone, for the firefox container to import as
//     a trusted root via certutil
//   - dir/ca.key    — CA private key (kept around so re-runs can re-issue
//     server certs without rotating the trust)
//
// Why two certs and not one self-signed leaf-as-its-own-CA: NSS (Firefox's
// cert validator) refuses to validate a single cert for *both* CA and server
// roles. `certutil -V -u V` reports "Certificate type not approved for
// application" even when trust flags include both `C` and `P`. The standard
// TLS pattern — separate root CA, leaf cert signed by it — sidesteps the
// issue entirely: the CA cert is purely a CA (CA:TRUE, KeyUsage=certSign),
// the server cert is purely a leaf (CA:FALSE, EKU=serverAuth), and the chain
// validates cleanly.
//
// linuxserver/xemu's nginx init only generates a cert if /config/ssl/cert.pem
// is missing, so writing this layout ahead of container start replaces the
// image's CN=*-no-SAN cert with our SAN-pinned chain.
func generateXemuCerts(dir, hostname string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	bundlePath := filepath.Join(dir, "cert.pem")
	if _, err := os.Stat(bundlePath); err == nil {
		return nil
	}

	caKey, caDER, err := makeCA()
	if err != nil {
		return fmt.Errorf("ca: %w", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return fmt.Errorf("parse ca: %w", err)
	}

	serverKey, serverDER, err := makeServerCert(hostname, caCert, caKey)
	if err != nil {
		return fmt.Errorf("server: %w", err)
	}

	// nginx serves leaf+chain so the browser sees the full chain; trust is
	// rooted in the separately-imported CA on the firefox side.
	bundle := pemBlock("CERTIFICATE", serverDER) + pemBlock("CERTIFICATE", caDER)
	if err := os.WriteFile(bundlePath, []byte(bundle), 0o644); err != nil {
		return fmt.Errorf("write cert bundle: %w", err)
	}
	if err := writePEMFile(filepath.Join(dir, "cert.key"), "PRIVATE KEY", mustPKCS8(serverKey), 0o600); err != nil {
		return err
	}
	if err := writePEMFile(filepath.Join(dir, "ca.pem"), "CERTIFICATE", caDER, 0o644); err != nil {
		return err
	}
	if err := writePEMFile(filepath.Join(dir, "ca.key"), "PRIVATE KEY", mustPKCS8(caKey), 0o600); err != nil {
		return err
	}
	return nil
}

func makeCA() (*rsa.PrivateKey, []byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 62))
	if err != nil {
		return nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "xemu-cartographer dev CA",
			Organization: []string{"xemu-cartographer"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	return key, der, err
}

func makeServerCert(hostname string, caCert *x509.Certificate, caKey *rsa.PrivateKey) (*rsa.PrivateKey, []byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 62))
	if err != nil {
		return nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "localhost",
			Organization: []string{"xemu-cartographer"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
		DNSNames:              []string{"localhost", hostname},
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	return key, der, err
}

func mustPKCS8(key *rsa.PrivateKey) []byte {
	b, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		panic(err)
	}
	return b
}

func pemBlock(blockType string, der []byte) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der}))
}

func writePEMFile(path, blockType string, der []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	if err := pem.Encode(f, &pem.Block{Type: blockType, Bytes: der}); err != nil {
		return fmt.Errorf("pem encode %s: %w", path, err)
	}
	return nil
}
