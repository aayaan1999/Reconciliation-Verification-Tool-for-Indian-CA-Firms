// Command dialer opens a network connection. It is never shipped: archtest builds it to
// prove the binary network check detects connection code.
package main

import "net"

func main() {
	if c, err := net.Dial("tcp", "127.0.0.1:9"); err == nil {
		c.Close()
	}
}
