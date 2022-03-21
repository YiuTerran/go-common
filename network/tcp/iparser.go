package tcp

// IParser parser的接口，开放给外部自定义
//用于从tcp流数据中分离出整段信息
type IParser interface {
	Read(conn *Conn) ([]byte, error)
	Write(conn *Conn, args ...[]byte) error
}

// DirectlyWrite 直接写入字节码，工具函数
func DirectlyWrite(conn *Conn, args ...[]byte) error {
	var msgLen uint32
	for i := 0; i < len(args); i++ {
		msgLen += uint32(len(args[i]))
	}

	msg := make([]byte, msgLen)
	var l int
	for i := 0; i < len(args); i++ {
		copy(msg[l:], args[i])
		l += len(args[i])
	}
	conn.Write(msg)
	return nil
}
