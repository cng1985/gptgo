package skill

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

func newHubClient(timeout time.Duration, allowLoopback bool, host string) *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialHub(ctx, dialer, network, addr, allowLoopback)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("重定向过多")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("地址不可访问")
			}
			if !sameHost(req.URL.Host, host) && !sameHost(req.URL.Host, via[0].URL.Host) {
				return fmt.Errorf("地址不可访问")
			}
			return nil
		},
	}
}

func dialHub(ctx context.Context, dialer *net.Dialer, network, addr string, allowLoopback bool) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("地址无法解析")
	}
	var last error
	for _, ip := range ips {
		if blockedSkillIP(ip.IP, allowLoopback) {
			last = fmt.Errorf("地址不可访问")
			continue
		}
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
		if err == nil {
			return conn, nil
		}
		last = err
	}
	if last == nil {
		last = fmt.Errorf("地址不可访问")
	}
	return nil, last
}

func blockedSkillIP(ip net.IP, allowLoopback bool) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() {
		return !allowLoopback
	}
	return ip.IsPrivate() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

func sameHost(left, right string) bool {
	return strings.EqualFold(hostOnly(left), hostOnly(right))
}

func hostOnly(value string) string {
	if host, _, err := net.SplitHostPort(value); err == nil {
		return host
	}
	return value
}
