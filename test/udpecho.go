package main

import (
	"fmt"
	"net"
)

const (
	UDP_PORT = ":8080"
)

func main() {
	// 创建 UDP 地址结构
	addr, err := net.ResolveUDPAddr("udp", UDP_PORT)
	if err != nil {
		fmt.Println("Error resolving address:", err)
		return
	}

	// 监听 UDP 端口
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		fmt.Println("Error listening:", err)
		return
	}
	defer conn.Close()
	fmt.Printf("UDP Echo Server listening on %s\n", UDP_PORT)

	buffer := make([]byte, 1024)

	for {
		// 读取客户端数据
		n, clientAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading:", err)
			continue
		}

		message := string(buffer[:n])
		fmt.Printf("Received from %s: %s", clientAddr, message)

		// 回显数据给客户端
		_, err = conn.WriteToUDP([]byte(message), clientAddr)
		if err != nil {
			fmt.Println("Error writing:", err)
		}
	}
}
