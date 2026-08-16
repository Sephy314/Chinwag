package middleware

import (
	"fmt"
	"net"

	"github.com/labstack/echo/v5"
)

// NewXFFIPExtractor returns an IP extractor that resolves the real client IP
// from X-Forwarded-For, trusting ONLY the given proxy CIDR (Traefik's source —
// the k3s pod CIDR). Loopback/link-local/private-net blanket trust is disabled
// so a client on any other network (or a non-proxy host) can NOT forge XFF to
// rotate the rate-limit key. Traefik appends the real client IP as the rightmost
// XFF hop; the extractor reads from the right and stops at the first untrusted
// IP, which is the client.
func NewXFFIPExtractor(trustedCIDR string) (echo.IPExtractor, error) {
	_, ipNet, err := net.ParseCIDR(trustedCIDR)
	if err != nil {
		return nil, fmt.Errorf("invalid GATEWAY_TRUSTED_PROXY_CIDR %q: %w", trustedCIDR, err)
	}
	return echo.ExtractIPFromXFFHeader(
		echo.TrustLoopback(false),
		echo.TrustLinkLocal(false),
		echo.TrustPrivateNet(false), // no blanket 10.x / 192.168.x trust
		echo.TrustIPRange(ipNet),    // only the proxy hop (Traefik pod CIDR)
	), nil
}
