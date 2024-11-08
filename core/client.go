package core

import (
	"bufio"
	"io"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/shadowsocks/go-shadowsocks2/core"
)

const (
	Version = "1.0"
)

func createShadowsocksAddress(targetAddr string) ([]byte, error) {
	ip1 := strings.Split(targetAddr, ":")
	if net.ParseIP(ip1[0]) == nil {
		// 0x03 表示目标地址是域名
		addrBytes := []byte(ip1[0])
		address := append([]byte{0x03, byte(len(addrBytes))}, addrBytes...)
		// 在域名后面加上端口（假设是 443 或 80）
		address = append(address, []byte{0x01, 0xBB}...) // 使用 0x00 0x00 表示端口（这里只是示例，实际上你需要根据目标来修改端口）
		//address = append(address, []byte{0x00, 0x50}...)
		return address, nil
	}
	// 如果目标地址是 IP 地址
	ip := net.ParseIP(ip1[0])
	if ip != nil {
		// 0x01 表示目标地址是 IP 地址
		log.Println(ip.To4())
		address := append([]byte{0x01}, ip.To4()...)
		address = append(address, []byte{0x1F, 0x91}...) // 假设端口为 443 或 80
		return address, nil
	}

	return nil, nil
}

func connectToShadowsocks(targetAddr, ssServerAddr, ssPassword, ssCipher string) (net.Conn, error) {
	// 初始化 Shadowsocks 加密方法
	cipher, err := core.PickCipher(ssCipher, nil, ssPassword)
	if err != nil {
		return nil, err
	}

	// 连接到 Shadowsocks 服务器
	rawConn, err := net.Dial("tcp", ssServerAddr)
	if err != nil {
		return nil, err
	}

	// 使用加密创建一个 Shadowsocks 连接
	conn := cipher.StreamConn(rawConn)
	// 获取目标地址协议格式
	address, err := createShadowsocksAddress(targetAddr)
	if err != nil {
		conn.Close()
		return nil, err
	}

	// 通过加密连接发送目标地址
	_, err = conn.Write(address)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// 处理 HTTP 请求，通过 Shadowsocks 隧道转发到目标服务器
func handleHTTP(conn net.Conn, ssServerAddr, ssPassword, ssCipher string) {
	defer conn.Close()
	req, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		log.Println(err)
		return
	}
	req.Header.Del("Proxy-Authorization")
	// 获取目标地址
	targetAddr := req.Host
	log.Println("targetAddr:", targetAddr)
	// 连接到目标服务器，通过 Shadowsocks
	c, err := connectToShadowsocks(targetAddr, ssServerAddr, ssPassword, ssCipher)
	if err != nil {
		log.Printf("连接 Shadowsocks 服务器失败: %v", err)
		//	http.Error(w, "无法连接到目标服务器", http.StatusServiceUnavailable)
		return
	}
	defer c.Close()
	if req.Method == "CONNECT" {
		b := []byte("HTTP/1.1 200 Connection established\r\n" +
			"Proxy-Agent: goway/" + Version + "\r\n\r\n")

		if _, err := conn.Write(b); err != nil {
			return
		}
	} else {
		req.Header.Del("Proxy-Connection")
		req.Header.Set("Connection", "Keep-Alive")
		// 将请求写入 Shadowsocks 连接
		if err := req.Write(c); err != nil {
			log.Printf("发送请求失败: %v", err)
			//http.Error(w, "请求发送失败", http.StatusInternalServerError)
			return
		}
	}
	Transport(conn, c)
}

func pipe(dst io.Writer, src io.Reader, ch chan<- error) {
	_, err := io.Copy(dst, src)
	ch <- err
}

func Transport(conn1, conn2 net.Conn) (err error) {
	rChan := make(chan error, 1)
	wChan := make(chan error, 1)

	go pipe(conn1, conn2, wChan)
	go pipe(conn2, conn1, rChan)

	select {
	case err = <-wChan:
		//log.Println("w exit", err)
	case err = <-rChan:
		//log.Println("r exit", err)
	}

	return
}

func handleHttp(conn net.Conn) {
	req, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		log.Println(err)
		return
	}
	log.Println(req.Host)
	log.Println(req.Method)
}

func Client() {
	// Shadowsocks 服务器配置
	ssServerAddr := "127.0.0.1:8388" // 替换为你的 Shadowsocks 服务器地址和端口
	ssPassword := "123456"           // 替换为你的 Shadowsocks 密码
	ssCipher := "AEAD_AES_128_GCM"   // 替换为你的 Shadowsocks 加密方式

	// 本地 HTTP 代理监听地址
	//listenAddr := "127.0.0.1:8888"

	ln, err := net.Listen("tcp", ":8082")
	if err != nil {
		log.Fatal(err)
	}

	defer ln.Close()
	log.Printf("HTTP 代理启动，监听地址 %s，转发到 Shadowsocks 服务器 %s", ":8082", ssServerAddr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go handleHTTP(conn, ssServerAddr, ssPassword, ssCipher)
	}

	// // 设置 HTTP 处理函数
	// http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
	// 	handleHTTP(w, req, ssServerAddr, ssPassword, ssCipher)
	// })

	// log.Printf("HTTP 代理启动，监听地址 %s，转发到 Shadowsocks 服务器 %s", listenAddr, ssServerAddr)

	// // 启动 HTTP 服务器
	// if err := http.ListenAndServe(listenAddr, nil); err != nil {
	// 	log.Fatalf("服务器启动失败: %v", err)
	// }
}
