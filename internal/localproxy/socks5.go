package localproxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"flowpilot/internal/models"

	xproxy "golang.org/x/net/proxy"
)

func handleSOCKS5Client(client net.Conn, upstream models.ProxyConfig, localUsername, localPassword string) error {
	_ = client.SetDeadline(time.Now().Add(30 * time.Second))
	if err := performSOCKS5Handshake(client, localUsername, localPassword); err != nil {
		return err
	}
	target, err := readSOCKS5ConnectRequest(client)
	if err != nil {
		return err
	}
	upstreamConn, err := dialViaUpstream(client.RemoteAddr().String(), upstream, target)
	if err != nil {
		_, _ = client.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return err
	}
	defer upstreamConn.Close()
	_ = client.SetDeadline(time.Time{})
	_, _ = client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})

	errCh := make(chan error, 2)
	go func() { _, err := io.Copy(upstreamConn, client); errCh <- err }()
	go func() { _, err := io.Copy(client, upstreamConn); errCh <- err }()
	<-errCh
	return nil
}

func performSOCKS5Handshake(conn net.Conn, username, password string) error {
	head := make([]byte, 2)
	if _, err := io.ReadFull(conn, head); err != nil {
		return fmt.Errorf("read socks5 header: %w", err)
	}
	if head[0] != 0x05 {
		return fmt.Errorf("unsupported socks version %d", head[0])
	}
	methods := make([]byte, int(head[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return fmt.Errorf("read socks5 methods: %w", err)
	}
	method := byte(0xFF)
	for _, candidate := range methods {
		if username != "" && candidate == 0x02 {
			method = 0x02
			break
		}
		if username == "" && candidate == 0x00 {
			method = 0x00
		}
	}
	if _, err := conn.Write([]byte{0x05, method}); err != nil {
		return err
	}
	if method == 0xFF {
		return fmt.Errorf("no acceptable socks5 auth method")
	}
	if method == 0x02 {
		return authenticateSOCKS5UserPass(conn, username, password)
	}
	return nil
}

func authenticateSOCKS5UserPass(conn net.Conn, expectedUser, expectedPass string) error {
	head := make([]byte, 2)
	if _, err := io.ReadFull(conn, head); err != nil {
		return fmt.Errorf("read socks5 auth header: %w", err)
	}
	if head[0] != 0x01 {
		return fmt.Errorf("unsupported socks5 auth version %d", head[0])
	}
	userBytes := make([]byte, int(head[1]))
	if _, err := io.ReadFull(conn, userBytes); err != nil {
		return fmt.Errorf("read socks5 auth username: %w", err)
	}
	passLen := make([]byte, 1)
	if _, err := io.ReadFull(conn, passLen); err != nil {
		return fmt.Errorf("read socks5 auth password length: %w", err)
	}
	passBytes := make([]byte, int(passLen[0]))
	if _, err := io.ReadFull(conn, passBytes); err != nil {
		return fmt.Errorf("read socks5 auth password: %w", err)
	}
	if string(userBytes) != expectedUser || string(passBytes) != expectedPass {
		_, _ = conn.Write([]byte{0x01, 0x01})
		return fmt.Errorf("invalid socks5 credentials")
	}
	_, err := conn.Write([]byte{0x01, 0x00})
	return err
}

func readSOCKS5ConnectRequest(conn net.Conn) (string, error) {
	req := make([]byte, 4)
	if _, err := io.ReadFull(conn, req); err != nil {
		return "", fmt.Errorf("read socks5 request: %w", err)
	}
	if req[0] != 0x05 || req[1] != 0x01 {
		return "", fmt.Errorf("unsupported socks5 command")
	}
	var host string
	switch req[3] {
	case 0x01:
		addr := make([]byte, 4)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return "", err
		}
		host = net.IP(addr).String()
	case 0x03:
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return "", err
		}
		addr := make([]byte, int(lenBuf[0]))
		if _, err := io.ReadFull(conn, addr); err != nil {
			return "", err
		}
		host = string(addr)
	case 0x04:
		addr := make([]byte, 16)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return "", err
		}
		host = net.IP(addr).String()
	default:
		return "", fmt.Errorf("unsupported socks5 address type")
	}
	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBuf); err != nil {
		return "", err
	}
	port := int(portBuf[0])<<8 | int(portBuf[1])
	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}

func dialViaUpstream(_ string, upstream models.ProxyConfig, target string) (net.Conn, error) {
	switch upstream.Protocol {
	case models.ProxySOCKS5:
		var auth *xproxy.Auth
		if upstream.Username != "" {
			auth = &xproxy.Auth{User: upstream.Username, Password: upstream.Password}
		}
		dialer, err := xproxy.SOCKS5("tcp", upstream.Server, auth, xproxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("build upstream socks5 dialer: %w", err)
		}
		conn, err := dialer.Dial("tcp", target)
		if err != nil {
			return nil, fmt.Errorf("dial via upstream socks5: %w", err)
		}
		return conn, nil
	case models.ProxyHTTPS:
		return dialViaHTTPConnect(upstream, target, true)
	case models.ProxyHTTP, "":
		return dialViaHTTPConnect(upstream, target, false)
	default:
		return nil, fmt.Errorf("unsupported upstream proxy protocol: %s", upstream.Protocol)
	}
}

func dialViaHTTPConnect(upstream models.ProxyConfig, target string, useTLS bool) (net.Conn, error) {
	var conn net.Conn
	var err error
	if useTLS {
		host, _, splitErr := net.SplitHostPort(upstream.Server)
		if splitErr != nil {
			host = upstream.Server
		}
		conn, err = tls.Dial("tcp", upstream.Server, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	} else {
		dialer := net.Dialer{Timeout: 15 * time.Second}
		conn, err = dialer.DialContext(context.Background(), "tcp", upstream.Server)
	}
	if err != nil {
		return nil, fmt.Errorf("dial upstream proxy: %w", err)
	}

	if err := sendHTTPConnect(conn, upstream, target); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func sendHTTPConnect(conn net.Conn, upstream models.ProxyConfig, target string) error {
	builder := strings.Builder{}
	builder.WriteString("CONNECT ")
	builder.WriteString(target)
	builder.WriteString(" HTTP/1.1\r\nHost: ")
	builder.WriteString(target)
	builder.WriteString("\r\nProxy-Connection: Keep-Alive\r\n")
	if upstream.Username != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(upstream.Username + ":" + upstream.Password))
		builder.WriteString("Proxy-Authorization: Basic ")
		builder.WriteString(auth)
		builder.WriteString("\r\n")
	}
	builder.WriteString("\r\n")
	if _, err := io.WriteString(conn, builder.String()); err != nil {
		return fmt.Errorf("write connect request: %w", err)
	}
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil {
		return fmt.Errorf("read connect response: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upstream connect status: %s", resp.Status)
	}
	return nil
}
