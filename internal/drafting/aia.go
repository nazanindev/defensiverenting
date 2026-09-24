package drafting

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Some official sites serve only their own certificate and leave out the
// intermediate that links it to a trusted root (ilga.gov, 2026-09-23: every
// Illinois statute quote failed with "certificate signed by unknown
// authority", so no approval touching one could ever pass the publish gate).
// Browsers repair this by fetching the missing intermediate from the address
// the certificate names (its Authority Information Access URL); Go does not.
// fetchClient does the same repair, and nothing more: the chain it builds is
// verified against the system roots exactly as usual, with the host name
// checked. A certificate that does not chain to a trusted root is still
// refused.

// aiaRoots is the trust store the check verifies against; nil means the
// system roots. Tests point it at their own root.
var aiaRoots *x509.CertPool

// aiaFetch reads one intermediate certificate from an AIA URL. Swappable in
// tests.
var aiaFetch = func(url string) ([]byte, error) {
	c := &http.Client{Timeout: 5 * time.Second}
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<10))
}

// aiaCache keeps intermediates already fetched, keyed by URL.
var aiaCache sync.Map

// verifyWithAIA verifies the server's chain as crypto/tls would, and when the
// only failure is a missing intermediate, fetches it (at most 3 hops) and
// verifies again.
func verifyWithAIA(cs tls.ConnectionState) error {
	if len(cs.PeerCertificates) == 0 {
		return errors.New("no server certificate")
	}
	leaf := cs.PeerCertificates[0]
	inter := x509.NewCertPool()
	for _, c := range cs.PeerCertificates[1:] {
		inter.AddCert(c)
	}
	opts := x509.VerifyOptions{DNSName: cs.ServerName, Roots: aiaRoots, Intermediates: inter}
	_, err := leaf.Verify(opts)
	if err == nil {
		return nil
	}
	var unknown x509.UnknownAuthorityError
	if !errors.As(err, &unknown) {
		return err
	}
	last := cs.PeerCertificates[len(cs.PeerCertificates)-1]
	for hop := 0; hop < 3; hop++ {
		next, ferr := fetchIssuer(last)
		if ferr != nil {
			return err
		}
		inter.AddCert(next)
		if _, verr := leaf.Verify(opts); verr == nil {
			return nil
		}
		last = next
	}
	return err
}

func fetchIssuer(c *x509.Certificate) (*x509.Certificate, error) {
	for _, url := range c.IssuingCertificateURL {
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			continue
		}
		if v, ok := aiaCache.Load(url); ok {
			return v.(*x509.Certificate), nil
		}
		der, err := aiaFetch(url)
		if err != nil {
			continue
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			continue
		}
		aiaCache.Store(url, cert)
		return cert, nil
	}
	return nil, errors.New("no fetchable issuer certificate")
}

// fetchClient is the HTTP client source fetches use.
func fetchClient(timeout time.Duration) *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.TLSClientConfig = &tls.Config{
		// Verification is not skipped: VerifyConnection runs the full chain
		// and host name check itself, adding fetched intermediates when a
		// server left them out.
		InsecureSkipVerify: true, //nolint:gosec // verified in VerifyConnection
		VerifyConnection:   verifyWithAIA,
	}
	return &http.Client{Timeout: timeout, Transport: t}
}
