package netutil

import (
	"bytes"
	"errors"
	"github.com/YiuTerran/go-common/base/util/httputil"
	"math/big"
	"net"
	"net/http"
	"regexp"
	"strings"
)

const (
	TypeDomain = 0
	TypeIPV4   = 4
	TypeIPV6   = 6

	regexIPV6   = `^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))$`
	regexIPV4   = `^(((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.|$)){4})`
	regexDomain = `^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9][a-z0-9-]{0,61}[a-z0-9]$`
)

var (
	reV6, _     = regexp.Compile(regexIPV6)
	reV4, _     = regexp.Compile(regexIPV4)
	reDomain, _ = regexp.Compile(regexDomain)
)

// NetAddr2IpPort Golang内置网络地址转为常用的ip端口形式
func NetAddr2IpPort(addr net.Addr) (ip string, port int) {
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
	return err == nil
}

// GetAllIP 获取所有网卡的ip
func GetAllIP() []net.IP {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	result := make([]net.IP, 0, 1)
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip != nil && !ip.IsLoopback() && ip.To4() != nil {
				result = append(result, ip)
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
	r, err := httputil.NewRequest().Get("https://myexternalip.com/raw")
	if err != nil || !r.IsSuccess() {
		return ""
	}
	return string(r.Bytes())
}

func FilterSelfIP(prefix string) (net.IP, error) {
	ips := GetAllIP()
	if len(ips) == 0 {
		return nil, errors.New("can't resolve ip")
	}
	for _, ip := range ips {
		if ip.IsLoopback() {
			continue
		}
		if prefix != "" {
			if strings.HasPrefix(ip.String(), prefix) {
				return ip, nil
			}
		} else if ip.IsPrivate() {
			return ip, nil
		}
	}
	return ips[0], errors.New("can't get specified ip")
}

func GuessAddrType(addr string) int {
	bs := []byte(addr)
	if reV4.Match(bs) {
		return TypeIPV4
	} else if reV6.Match(bs) {
		return TypeIPV6
	} else if reDomain.Match(bs) {
		return TypeDomain
	} else {
		return -1
	}
}
