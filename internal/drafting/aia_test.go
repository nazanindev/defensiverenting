package drafting

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func newCert(t *testing.T, tmpl *x509.Certificate, parent *testCA) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, parentCert := key, tmpl
	if parent != nil {
		signer, parentCert = parent.key, parent.cert
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parentCert, &key.PublicKey, signer)
	if err != nil {
		t.Fatal(err)
	}
	c, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return &testCA{cert: c, key: key}
}

// A server that sends only its leaf certificate (ilga.gov, 2026-09-23) is
// fetched once the missing intermediate is fetched from its AIA URL; a leaf
// from an untrusted root is still refused.
func TestFetchClient_repairsMissingIntermediate(t *testing.T) {
	now := time.Now()
	root := newCert(t, &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Test Root"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign}, nil)
	inter := newCert(t, &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "Test Intermediate"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign}, root)
	leaf := newCert(t, &x509.Certificate{SerialNumber: big.NewInt(3), Subject: pkix.Name{CommonName: "127.0.0.1"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IssuingCertificateURL: []string{"http://aia.test/inter.crt"}}, inter)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) }))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{leaf.cert.Raw}, PrivateKey: leaf.key}}}
	srv.StartTLS()
	defer srv.Close()

	pool := x509.NewCertPool()
	pool.AddCert(root.cert)
	aiaRoots = pool
	defer func() { aiaRoots = nil }()
	fetched := 0
	aiaFetch = func(url string) ([]byte, error) { fetched++; return inter.cert.Raw, nil }
	aiaCache.Range(func(k, _ any) bool { aiaCache.Delete(k); return true })

	resp, err := fetchClient(5 * time.Second).Get(srv.URL)
	if err != nil {
		t.Fatalf("leaf-only chain was not repaired: %v", err)
	}
	resp.Body.Close()
	if fetched != 1 {
		t.Errorf("intermediate fetched %d times, want 1", fetched)
	}

	// Untrusted root: repair cannot make it trusted.
	aiaRoots = x509.NewCertPool()
	aiaCache.Range(func(k, _ any) bool { aiaCache.Delete(k); return true })
	if _, err := fetchClient(5 * time.Second).Get(srv.URL); err == nil {
		t.Error("a chain to an untrusted root was accepted")
	}
}
