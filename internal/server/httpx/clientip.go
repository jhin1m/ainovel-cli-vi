package httpx

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ClientIP trả về IP của client. Header CF-Connecting-IP / X-Forwarded-For chỉ được
// tin khi peer trực tiếp (RemoteAddr) nằm trong danh sách proxy tin cậy; ngược lại
// ai cũng giả được IP để né rate limit.
func ClientIP(r *http.Request, trusted []netip.Prefix) netip.Addr {
	peer := remoteAddr(r)
	if !peer.IsValid() || !inPrefixes(peer, trusted) {
		return peer
	}
	if v := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); v != "" {
		if a, err := netip.ParseAddr(v); err == nil {
			return a.Unmap()
		}
	}
	// X-Forwarded-For: đi từ phải sang trái, bỏ qua các proxy tin cậy.
	if xff := r.Header.Values("X-Forwarded-For"); len(xff) > 0 {
		parts := strings.Split(strings.Join(xff, ","), ",")
		for i := len(parts) - 1; i >= 0; i-- {
			a, err := netip.ParseAddr(strings.TrimSpace(parts[i]))
			if err != nil {
				break
			}
			a = a.Unmap()
			if !inPrefixes(a, trusted) {
				return a
			}
		}
	}
	return peer
}

func remoteAddr(r *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	a, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return a.Unmap()
}

func inPrefixes(a netip.Addr, prefixes []netip.Prefix) bool {
	for _, p := range prefixes {
		if p.Contains(a) {
			return true
		}
	}
	return false
}
