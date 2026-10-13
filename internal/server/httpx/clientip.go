package httpx

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// Header chứa IP client mà proxy tin cậy đặt vào.
const (
	HeaderXForwardedFor   = "X-Forwarded-For"
	HeaderCFConnectingIP  = "CF-Connecting-IP"
	defaultClientIPHeader = HeaderXForwardedFor
)

// ClientIP trả về IP của client. Header IP chỉ được tin khi peer trực tiếp
// (RemoteAddr) nằm trong danh sách proxy tin cậy, và chỉ đúng một header được
// cấu hình — không rơi về header khác, vì proxy đứng trước (vd. nginx) chuyển
// nguyên header client tự đặt và ai cũng giả được IP để né rate limit.
func ClientIP(r *http.Request, trusted []netip.Prefix, header string) netip.Addr {
	peer := remoteAddr(r)
	if !peer.IsValid() || !inPrefixes(peer, trusted) {
		return peer
	}
	if header == "" {
		header = defaultClientIPHeader
	}
	if strings.EqualFold(header, HeaderXForwardedFor) {
		return fromForwardedFor(r, trusted, peer)
	}
	if a, ok := parseIP(r.Header.Get(header)); ok {
		return a
	}
	return peer
}

// fromForwardedFor đi từ phải sang trái, bỏ qua các proxy tin cậy; IP đầu tiên
// không thuộc proxy là client.
func fromForwardedFor(r *http.Request, trusted []netip.Prefix, peer netip.Addr) netip.Addr {
	xff := r.Header.Values(HeaderXForwardedFor)
	if len(xff) == 0 {
		return peer
	}
	parts := strings.Split(strings.Join(xff, ","), ",")
	for i := len(parts) - 1; i >= 0; i-- {
		a, ok := parseIP(parts[i])
		if !ok {
			// Phần tử rác do chính proxy tin cậy chuyển tới: không đoán tiếp.
			return peer
		}
		if !inPrefixes(a, trusted) {
			return a
		}
	}
	return peer
}

// parseIP nhận "1.2.3.4", "1.2.3.4:5678", "[::1]:80".
func parseIP(s string) (netip.Addr, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return netip.Addr{}, false
	}
	if a, err := netip.ParseAddr(s); err == nil {
		return a.Unmap(), true
	}
	if ap, err := netip.ParseAddrPort(s); err == nil {
		return ap.Addr().Unmap(), true
	}
	return netip.Addr{}, false
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
