package netutil

import (
	"bytes"
	"github.com/YiuTerran/go-common/base/util/httputil"
	"math/big"
	"net"
	"net/http"
)

func NetAddr2IPPort(addr net.Addr) (ip string, port int) {
	switch addr := addr.(type) {
	case *net.UDPAddr:
		ip = addr.IP.String()
		port = addr.Port
	case *net.TCPAddr:
		ip = addr.IP.String()
		port = addr.Port
	}
	return
}

// IsInternetOK 网络是否正常
func IsInternetOK() (ok bool) {
	_, err := http.Get("https://www.google.cn/generate_204")
	if err != nil {
		return false
	}
	return true
}

// GetAllIP 获取所有网卡的ip
func GetAllIP() []net.IP {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	result := make([]net.IP, 0, 1)
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip, ok := addr.(*net.IPNet)
			if ok && !ip.IP.IsLoopback() && ip.IP.To4() != nil {
				result = append(result, ip.IP)
			}
		}
	}
	return result
}

// IsPublicIP 是否公网IP
func IsPublicIP(IP net.IP) bool {
	if IP.IsLoopback() || IP.IsLinkLocalMulticast() || IP.IsLinkLocalUnicast() {
		return false
	}
	if ip4 := IP.To4(); ip4 != nil {
		switch true {
		case ip4[0] == 10:
			return false
		case ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31:
			return false
		case ip4[0] == 192 && ip4[1] == 168:
			return false
		default:
			return true
		}
	}
	return false
}

// InetAton IPV4转整数
func InetAton(ip net.IP) int64 {
	ipv4Int := big.NewInt(0)
	ipv4Int.SetBytes(ip.To4())
	return ipv4Int.Int64()
}

// InetNtoa 整数转IPV4
func InetNtoa(ipnr int64) net.IP {
	var bs [4]byte
	bs[0] = byte(ipnr & 0xFF)
	bs[1] = byte((ipnr >> 8) & 0xFF)
	bs[2] = byte((ipnr >> 16) & 0xFF)
	bs[3] = byte((ipnr >> 24) & 0xFF)
	return net.IPv4(bs[3], bs[2], bs[1], bs[0])
}

// IpBetween 判断test是否在from和to之间的网段里
func IpBetween(from net.IP, to net.IP, test net.IP) bool {
	if from == nil || to == nil || test == nil {
		return false
	}

	from16 := from.To16()
	to16 := to.To16()
	test16 := test.To16()
	if from16 == nil || to16 == nil || test16 == nil {
		return false
	}

	if bytes.Compare(test16, from16) >= 0 && bytes.Compare(test16, to16) <= 0 {
		return true
	}
	return false
}

// GetExternalIP 获取公网IP
func GetExternalIP() string {
	r, err := httputil.Request().Get("https://myexternalip.com/raw")
	if err != nil || !r.IsSuccess() {
		return ""
	}
	return string(r.Bytes())
}
