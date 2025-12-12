package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time" // 引入 time 包用于设置 KeepAlive 时间间隔
)

const (
	socks5Version                = 0x05
	authNoAuthenticationRequired = 0x00
	authNoAcceptableMethods      = 0xff
	connectCmd                   = 0x01
	ipv4Addr                     = 0x01
	domainAddr                   = 0x03
	ipv6Addr                     = 0x04
	repSuccess                   = 0x00
	repServerFailure             = 0x01
	repHostUnreachable           = 0x04
)

func main() {
	port := flag.String("port", "50440", "The port number for the SOCKS5 proxy to listen on")
	flag.Parse()
	listener, err := net.Listen("tcp", ":"+*port)
	if err != nil {
		fmt.Printf("Failed to listen on port %s: %v\n", *port, err)
		return
	}
	fmt.Printf("SOCKS5 proxy server started successfully on port: %s\n", *port)
	for {
		clientConn, err := listener.Accept()
		if err != nil {
			continue
		}
		go handleConnection(clientConn)
	}
}

func handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	// 1. 设置客户端连接的 KeepAlive
	// 防止客户端异常断电或中间防火墙因连接空闲而切断连接
	if tcpConn, ok := clientConn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)                    // 禁用 Nagle 算法，降低延迟
		tcpConn.SetKeepAlive(true)                  // 开启保活
		tcpConn.SetKeepAlivePeriod(3 * time.Minute) // 每 3 分钟探测一次
	}

	reader := bufio.NewReader(clientConn)
	if err := handleHandshake(reader, clientConn); err != nil {
		return
	}
	host, port, err := handleRequest(reader)
	if err != nil {
		writeReply(clientConn, repServerFailure, nil)
		return
	}
	destAddr := net.JoinHostPort(host, strconv.Itoa(port))
	targetConn, err := net.Dial("tcp", destAddr)
	if err != nil {
		writeReply(clientConn, repHostUnreachable, nil)
		return
	}
	defer targetConn.Close()

	// 2. 设置目标服务器连接的 KeepAlive
	// 同样为了防止与目标服务器的长连接被中间设备切断
	if tcpConn, ok := targetConn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(3 * time.Minute)
	}

	if err := writeReply(clientConn, repSuccess, targetConn.LocalAddr()); err != nil {
		return
	}
	if reader.Buffered() > 0 {
		if _, err := reader.WriteTo(targetConn); err != nil {
			return
		}
	}
	proxyData(clientConn, targetConn)
}

func handleHandshake(reader *bufio.Reader, writer io.Writer) error {
	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return err
	}
	if header[0] != socks5Version {
		return errors.New("unsupported SOCKS version")
	}
	nmethods := int(header[1])
	methods := make([]byte, nmethods)
	if _, err := io.ReadFull(reader, methods); err != nil {
		return err
	}
	var acceptable bool
	for _, method := range methods {
		if method == authNoAuthenticationRequired {
			acceptable = true
			break
		}
	}
	if !acceptable {
		writer.Write([]byte{socks5Version, authNoAcceptableMethods})
		return errors.New("no acceptable authentication methods")
	}
	_, err := writer.Write([]byte{socks5Version, authNoAuthenticationRequired})
	return err
}

func handleRequest(reader *bufio.Reader) (host string, port int, err error) {
	reqHeader := make([]byte, 4)
	if _, err := io.ReadFull(reader, reqHeader); err != nil {
		return "", 0, err
	}
	if reqHeader[0] != socks5Version {
		return "", 0, errors.New("invalid request version")
	}
	if reqHeader[1] != connectCmd {
		return "", 0, errors.New("unsupported command")
	}
	addrType := reqHeader[3]
	switch addrType {
	case ipv4Addr:
		addr := make([]byte, net.IPv4len)
		if _, err := io.ReadFull(reader, addr); err != nil {
			return "", 0, err
		}
		host = net.IP(addr).String()
	case domainAddr:
		domainLen, err := reader.ReadByte()
		if err != nil {
			return "", 0, err
		}
		domain := make([]byte, domainLen)
		if _, err := io.ReadFull(reader, domain); err != nil {
			return "", 0, err
		}
		host = string(domain)
	case ipv6Addr:
		addr := make([]byte, net.IPv6len)
		if _, err := io.ReadFull(reader, addr); err != nil {
			return "", 0, err
		}
		host = net.IP(addr).String()
	default:
		return "", 0, errors.New("unsupported address type")
	}
	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(reader, portBytes); err != nil {
		return "", 0, err
	}
	port = int(binary.BigEndian.Uint16(portBytes))
	return host, port, nil
}

func writeReply(writer io.Writer, rep byte, addr net.Addr) error {
	var ip net.IP
	var port uint16
	if addr != nil {
		tcpAddr, ok := addr.(*net.TCPAddr)
		if !ok {
			return errors.New("address is not a TCP address")
		}
		ip = tcpAddr.IP
		port = uint16(tcpAddr.Port)
	}
	res := []byte{socks5Version, rep, 0x00, ipv4Addr, 0, 0, 0, 0, 0, 0}
	if ip != nil {
		if ipv4 := ip.To4(); ipv4 != nil {
			res = make([]byte, 0, 4+net.IPv4len+2)
			res = append(res, socks5Version, rep, 0x00, ipv4Addr)
			res = append(res, ipv4...)
		} else {
			res = make([]byte, 0, 4+net.IPv6len+2)
			res = append(res, socks5Version, rep, 0x00, ipv6Addr)
			res = append(res, ip...)
		}
		portBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(portBytes, port)
		res = append(res, portBytes...)
	}
	_, err := writer.Write(res)
	return err
}

func proxyData(clientConn net.Conn, targetConn net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(targetConn, clientConn)
		if tcpConn, ok := targetConn.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()
	go func() {
		defer wg.Done()
		io.Copy(clientConn, targetConn)
		if tcpConn, ok := clientConn.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()
	wg.Wait()
}
