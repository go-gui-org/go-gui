//go:build !js

package gui

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"sync/atomic"
	"testing"
)

// TestImageDialBlocked pins which addresses the image fetcher refuses
// to dial (#1105). Link-local holds cloud metadata services
// (169.254.169.254) and has no use as an image host. Loopback and
// RFC 1918 stay open: LAN and local-server apps load images from them.
func TestImageDialBlocked(t *testing.T) {
	tests := []struct {
		addr    string
		blocked bool
	}{
		{"169.254.169.254", true},
		{"169.254.0.1", true},
		{"fe80::1", true},
		{"::ffff:169.254.169.254", true},
		{"0.0.0.0", true},
		{"::", true},
		{"224.0.0.1", true},
		{"ff02::1", true},
		{"127.0.0.1", false},
		{"::1", false},
		{"10.0.0.5", false},
		{"192.168.1.10", false},
		{"172.16.0.1", false},
		{"93.184.216.34", false},
		{"2606:2800:220:1::", false},
	}
	for _, tt := range tests {
		got := imageDialBlocked(netip.MustParseAddr(tt.addr))
		if got != tt.blocked {
			t.Errorf("imageDialBlocked(%s) = %v, want %v",
				tt.addr, got, tt.blocked)
		}
	}
}

// TestImageClientRefusesLinkLocalRedirect is the #1105
// regression for SSRF: a public image host that redirects to the
// cloud metadata address must not get the fetcher to connect there.
// The guard sits in the dialer, so it runs again on every redirect
// hop. Before the fix the client followed the redirect and dialed.
// The client is built with no proxy: an HTTP_PROXY on the runner would
// send the redirect to the proxy and the dial guard would never run.
func TestImageClientRefusesLinkLocalRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r,
				"http://169.254.169.254/latest/meta-data/",
				http.StatusFound)
		}))
	t.Cleanup(srv.Close)

	resp, err := imageGet(t, newImageHTTPClient(nil), srv.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("redirect to 169.254.169.254 was followed")
	}
	if !errors.Is(err, errImageDialBlocked) {
		t.Fatalf("err = %v, want errImageDialBlocked", err)
	}
}

// TestImageClientRefusesLinkLocalDirect covers the first hop:
// a src that names a link-local address is refused before connect.
func TestImageClientRefusesLinkLocalDirect(t *testing.T) {
	resp, err := imageGet(t, newImageHTTPClient(nil),
		"http://[fe80::1]/pixel.png")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("fetch to fe80::1 was not refused")
	}
	if !errors.Is(err, errImageDialBlocked) {
		t.Fatalf("err = %v, want errImageDialBlocked", err)
	}
}

// imageGet sends a GET with the test's context.
func imageGet(
	t *testing.T, c *http.Client, target string,
) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(
		t.Context(), http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	return c.Do(req)
}

// testImageProxy starts a forward proxy that answers every request
// itself with a tiny PNG and counts what it received.
func testImageProxy(t *testing.T) (*url.URL, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			hits.Add(1)
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("x"))
		}))
	t.Cleanup(srv.Close)
	pu, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return pu, &hits
}

// TestImageClientProxyRefusesLinkLocal covers the proxy path of
// #1105: with a proxy, the dialer only connects to the proxy, so the
// Control hook cannot see the target. The proxy guard must refuse a
// link-local target before the proxy is contacted.
func TestImageClientProxyRefusesLinkLocal(t *testing.T) {
	pu, hits := testImageProxy(t)
	c := newImageHTTPClient(http.ProxyURL(pu))

	resp, err := imageGet(t, c, "http://169.254.169.254/latest/")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("proxied fetch to 169.254.169.254 was not refused")
	}
	if !errors.Is(err, errImageDialBlocked) {
		t.Fatalf("err = %v, want errImageDialBlocked", err)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("proxy received %d requests, want 0", n)
	}
}

// TestImageClientProxyAllowsPublic checks the guard does not break
// ordinary proxied fetches.
func TestImageClientProxyAllowsPublic(t *testing.T) {
	pu, hits := testImageProxy(t)
	c := newImageHTTPClient(http.ProxyURL(pu))

	resp, err := imageGet(t, c, "http://example.test/p.png")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if n := hits.Load(); n != 1 {
		t.Fatalf("proxy received %d requests, want 1", n)
	}
}

// roundTripFunc is a RoundTripper that is not an *http.Transport.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// TestImageBaseTransportNonTransport: an instrumented
// http.DefaultTransport must not panic the client build. Before the
// fix an unchecked type assertion ran in a package var initializer.
func TestImageBaseTransportNonTransport(t *testing.T) {
	rt := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("unused")
	})
	tr := imageBaseTransport(rt)
	if tr == nil {
		t.Fatal("nil transport")
	}
	if tr.TLSHandshakeTimeout == 0 {
		t.Error("fallback transport has no TLS handshake timeout")
	}
}
