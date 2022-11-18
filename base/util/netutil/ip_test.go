package netutil

import (
	"fmt"
	"testing"
)

func TestGetAllIP(t *testing.T) {
	for _, ip := range GetAllIP() {
		fmt.Println(ip.String())
	}
}
func TestIsInternetOK(t *testing.T) {
	fmt.Println(IsInternetOK())
}

func TestGetExternalIP(t *testing.T) {
	fmt.Println(GetExternalIP())
}

func TestFilterSelfIP(t *testing.T) {
	ip, err := FilterSelfIP("172.16")
	fmt.Println(ip, err)
}
