package main

import (
	"context"
	"fmt"
	"net"
	"time"
)

const (
	serverAddr = "127.0.0.1:8001" // UDP 服务器地址
)

func main() {
	// 创建带超时的上下文 (10 秒)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 解析服务器地址
	udpAddr, err := net.ResolveUDPAddr("udp4", serverAddr)
	if err != nil {
		fmt.Println("解析地址失败:", err)
		return
	}

	// 创建 UDP 连接
	conn, err := net.DialUDP("udp4", nil, udpAddr)
	if err != nil {
		fmt.Println("连接服务器失败:", err)
		return
	}
	defer conn.Close()
	fmt.Printf("已连接到 UDP 服务器 %s (运行 10 秒后自动停止)\n", serverAddr)

	// 启动协程接收回显
	go receiveEcho(ctx, conn)

	// 创建定时器 (每秒发送一次)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 发送带时区的时间数据
			currentTime := time.Now().Format(time.RFC3339)
			message := []byte("TIME: " + currentTime)

			if _, err := conn.Write(message); err != nil {
				fmt.Println("发送失败:", err)
				continue
			}
			fmt.Printf("[%s] 已发送\n", currentTime)

		case <-ctx.Done(): // 10 秒后触发
			fmt.Println("\n运行完成，程序退出")
			return
		}
	}
}

// 接收服务器回显 (带退出控制)
func receiveEcho(ctx context.Context, conn *net.UDPConn) {
	buf := make([]byte, 1024)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// 设置读取超时避免永久阻塞
			conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))

			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue // 忽略超时错误
				}
				fmt.Println("接收失败:", err)
				return
			}
			fmt.Printf("收到回显: %s\n", string(buf[:n]))
		}
	}
}
