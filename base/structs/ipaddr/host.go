package ipaddr

import "fmt"

/**
  *  @author tryao
  *  @date 2022/05/05 14:15
**/

type Address struct {
	Type int //0,4或者6, 0:域名，4：ipv4, 6：ipv6
	IP   string
	Port int
}

func (addr *Address) String() string {
	return fmt.Sprintf("%s:%d", addr.IP, addr.Port)
}
