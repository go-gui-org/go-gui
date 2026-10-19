//go:build !js

package gui

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"syscall"
	"time"
)

// errImageDialBlocked is returned when the image fetcher refuses to
// connect to an address. errors.Is finds it through the *url.Error
// and *net.OpError wrappers http.Client adds.
var errImageDialBlocked = errors.New("image fetch: address not allowed")

// imageHTTPClient is the client defaultImageFetcher uses. It differs
// from http.DefaultClient in one way: it refuses link-local,
// multicast and unspecified addresses (#1105). Two checks do this:
//
//   - The dialer's Control hook sees the address after DNS
//     resolution, on every connection. So a redirect hop, or a DNS
//     name that resolves to 169.254.169.254, is caught the same as
//     an IP literal.
//   - With a proxy (HTTP_PROXY and friends), the dialer connects
//     only to the proxy, so Control never sees the target. The
//     Proxy hook then refuses a target URL whose host is a blocked
//     IP literal. It runs once per request, so redirects are
//     covered. A hostname is resolved by the proxy, not here: a name
//     that resolves to a link-local address through a proxy is not
//     caught. The proxy owner controls what it can reach.
var imageHTTPClient = newImageHTTPClient(http.ProxyFromEnvironment)

// newImageHTTPClient builds the guarded client. proxy is the
// transport's proxy selector; tests pass their own instead of the
// environment, which ProxyFromEnvironment reads once and caches.
func newImageHTTPClient(
	proxy func(*http.Request) (*url.URL, error),
) *http.Client {
	tr := imageBaseTransport(http.DefaultTransport)
	tr.Proxy = imageProxyGuard(proxy)
	// Same timeouts as the DialContext inside http.DefaultTransport.
	d := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   imageDialControl,
	}
	tr.DialContext = d.DialContext
	return &http.Client{Transport: tr}
}

// imageBaseTransport clones rt when it is an *http.Transport. An
// instrumentation package may have replaced http.DefaultTransport with
// another RoundTripper; an unchecked assertion here would then panic
// at package init in every binary that imports gui. The fallback copies
// the settings http.DefaultTransport ships with.
func imageBaseTransport(rt http.RoundTripper) *http.Transport {
	if t, ok := rt.(*http.Transport); ok {
		return t.Clone()
	}
	return &http.Transport{
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// imageProxyGuard wraps a proxy selector so a proxied request to a
// blocked IP literal fails before it reaches the proxy. A request
// that goes direct (nil proxy URL) is left to imageDialControl.
func imageProxyGuard(
	proxy func(*http.Request) (*url.URL, error),
) func(*http.Request) (*url.URL, error) {
	if proxy == nil {
		return nil
	}
	return func(req *http.Request) (*url.URL, error) {
		pu, err := proxy(req)
		if err != nil || pu == nil {
			return pu, err
		}
		ip, perr := netip.ParseAddr(req.URL.Hostname())
		if perr == nil && imageDialBlocked(ip) {
			return nil, fmt.Errorf("%w: %s", errImageDialBlocked, ip)
		}
		return pu, nil
	}
}

// imageDialControl is the net.Dialer.Control hook. address is the
// resolved "ip:port" the socket is about to connect to.
func imageDialControl(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return err
	}
	if imageDialBlocked(ip) {
		return fmt.Errorf("%w: %s", errImageDialBlocked, ip)
	}
	return nil
}

// imageDialBlocked reports whether the image fetcher refuses ip.
// Link-local covers the cloud metadata services (169.254.169.254,
// fe80::/10). Unspecified and multicast are never image hosts.
// Loopback and private (RFC 1918, fc00::/7) stay allowed on purpose:
// LAN apps and local dev servers serve images from them. An IPv4
// address mapped into IPv6 is checked as IPv4.
func imageDialBlocked(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified()
}
