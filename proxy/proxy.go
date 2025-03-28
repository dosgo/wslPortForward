package proxy

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/dosgo/wslPortForward/config"
)

const (
	TCP_TIMEOUT = 5 * time.Minute // TCP连接空闲超时
	UDP_TIMEOUT = 2 * time.Minute // UDP会话空闲超时
)

var udpNat sync.Map

func StartPoxy(conf *config.Conf, reboot bool) {
	if reboot {
		for _, v := range conf.Configs {
			if v.Listener != nil {
				v.Listener.Close()
			}
		}
	}
	var wslIP = ""
	if conf.AutoUseWslIp {
		wslIP = config.GetWslIP()
	}
	var err error
	for _, v := range conf.Configs {
		if conf.AutoUseWslIp {
			targetAddrs := strings.Split(v.TargetAddr, ":")
			if targetAddrs[0] == "127.0.0.1" && wslIP != "" {
				v.TargetAddr = wslIP + ":" + targetAddrs[1]
			}
		}

		if v.Protocol == "tcp" {
			v.Listener, err = StartTCPServer(fmt.Sprintf("0.0.0.0:%d", v.ListenPort), v.TargetAddr)
			if err == nil {
				v.Status = true
			}
		}
		if v.Protocol == "udp" {
			v.UdpConn, err = StartUDPServer(fmt.Sprintf("0.0.0.0:%d", v.ListenPort), v.TargetAddr)
			if err == nil {
				v.Status = true
			}
		}
	}
}

func StartTCPServer(listenAddr, targetAddr string) (net.Listener, error) {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Printf("TCP proxy %s -> %s err:%v\r\n", err)
		return nil, err
	}
	log.Printf("TCP proxy %s -> %s ok\r\n", listenAddr, targetAddr)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("TCP Accept err: %v\r\n", err)
				break
			}

			go handleTCPConnection(conn, targetAddr)
		}
	}()
	return listener, nil
}

func handleTCPConnection(src net.Conn, targetAddr string) {
	defer src.Close()

	// 带超时的目标连接
	dst, err := net.DialTimeout("tcp", targetAddr, 5*time.Second)
	if err != nil {
		log.Printf("TCP connect err: %v\r\n", err)
		return
	}
	defer dst.Close()

	// 双向带超时的数据转发
	go pipeWithTimeout(src, dst, TCP_TIMEOUT)
	pipeWithTimeout(dst, src, TCP_TIMEOUT)
}

// --------------------- UDP 代理实现 ---------------------
func StartUDPServer(listenAddr, targetAddr string) (*net.UDPConn, error) {
	srcAddr, _ := net.ResolveUDPAddr("udp", listenAddr)
	listener, err := net.ListenUDP("udp", srcAddr)
	if err != nil {
		log.Printf("UDP proxy %s -> %s err:%v\r\n", err)
		return nil, err
	}

	log.Printf("UDP proxy  %s -> %s ok\r\n", listenAddr, targetAddr)

	buf := make([]byte, 65507) // UDP 最大报文长度
	go func() {
		for {
			// 读取客户端数据
			n, clientAddr, err := listener.ReadFromUDP(buf)
			if err != nil {
				log.Printf("UDP read err: %v\r\n", err)
				break
			}

			localConn, ok := udpNat.Load(clientAddr.String())
			if ok {
				localConn.(net.Conn).Write(buf[:n])
			} else {
				go handleUDPPacket(listener, clientAddr, buf[:n], targetAddr)
			}
		}
	}()
	return listener, nil
}

func handleUDPPacket(conn *net.UDPConn, clientAddr *net.UDPAddr, data []byte, targetAddr string) {
	// 创建或复用目标连接
	targetConn, err := net.Dial("udp", targetAddr)
	if err != nil {
		log.Printf("UDP connect err: %v\r\n", err)
		return
	}
	defer targetConn.Close()
	udpNat.Store(clientAddr.String(), targetConn)
	defer udpNat.Delete(clientAddr.String())
	// 转发到目标
	if _, err := targetConn.Write(data); err != nil {
		log.Printf("UDP Forward err : %v\r\n", err)
		return
	}

	for {
		// 等待响应并回传
		resp := make([]byte, 65507)
		targetConn.SetReadDeadline(time.Now().Add(UDP_TIMEOUT))
		n, err := targetConn.Read(resp)
		if err != nil {
			if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
				log.Printf("UDP read err: %v\r\n", err)
			}
			return
		}

		if _, err := conn.WriteToUDP(resp[:n], clientAddr); err != nil {
			log.Printf("UDP write err: %v\r\n", err)
		}
	}
}

// --------------------- 通用工具函数 ---------------------
func pipeWithTimeout(dst, src net.Conn, timeout time.Duration) {
	buf := make([]byte, 32*1024) // 32KB 缓冲区
	for {
		// 设置读取超时
		src.SetReadDeadline(time.Now().Add(timeout))
		n, err := src.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				log.Printf("read time out : %s\r\n", src.RemoteAddr())
			}
			break
		}

		// 设置写入超时
		dst.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if _, err := dst.Write(buf[:n]); err != nil {
			log.Printf("write err: %v\r\n", err)
			break
		}
	}
}
