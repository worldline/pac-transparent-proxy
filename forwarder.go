package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
)

var re = regexp.MustCompile(`^([A-Z]+)(\s+([^\;]+\:[0-9]{1,5}))?;?`)

func pipe(c1 io.Writer, c2 io.Reader, finished chan<- error) {
	_, err := io.Copy(c1, c2)
	finished <- err
}

func duplex(c1 io.ReadWriteCloser, c2 io.ReadWriteCloser) error {
	finished := make(chan error)
	go pipe(c1, c2, finished)
	go pipe(c2, c1, finished)
	err := <-finished
	c1.Close()
	c2.Close()
	<-finished
	return err
}

func computeProxyProtocol(rule string) (string, error) {
	switch rule {
	case "PROXY", "HTTP":
		return "http", nil
	}

	return "", fmt.Errorf("%s is not a supported proxy protocol", rule)
}

func forward(incomingConnection net.Conn, destinationHost string, rawProxyURL string, config appConfig) error {
	parts := re.FindStringSubmatch(rawProxyURL)
	if len(parts[1]) == 0 || parts[1] == "DIRECT" {
		// DIRECT
		return forwardDirect(incomingConnection, destinationHost, config)
	}
	proxyProtocol, err := computeProxyProtocol(parts[1])
	if err != nil {
		return err
	}
	proxyURL := url.URL{
		Scheme: proxyProtocol,
		Host:   parts[3],
	}

	if proxyProtocol == "http" && !config.tunnel {
		addr, err := net.ResolveTCPAddr("tcp", destinationHost)
		if err != nil {
			return err
		}
		if addr.Port == 80 {
			return forwardHTTPviaProxy(incomingConnection, destinationHost, &proxyURL, config)
		}
	}

	return forwardTunnel(incomingConnection, destinationHost, &proxyURL, config)
}

func forwardHTTPviaProxy(incomingConnection net.Conn, destinationHost string, proxyURL *url.URL, config appConfig) error {
	conn, err := net.DialTimeout("tcp", proxyURL.Host, config.timeout)
	if err != nil {
		return err
	}
	defer conn.Close()

	bufIncomingReader := bufio.NewReader(incomingConnection)
	bufConnReader := bufio.NewReader(conn)

	for {
		httpRequest, err := http.ReadRequest(bufIncomingReader)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		httpRequest.URL.Scheme = "http"
		err = httpRequest.WriteProxy(conn)
		if err != nil {
			return err
		}
		resp, err := http.ReadResponse(bufConnReader, httpRequest)
		if err != nil {
			return err
		}
		err = resp.Write(incomingConnection)
		if err != nil {
			return err
		}
	}
}

func forwardDirect(incomingConnection net.Conn, destinationHost string, config appConfig) error {
	conn, err := net.DialTimeout("tcp", destinationHost, config.timeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	return duplex(incomingConnection, conn)
}

func forwardTunnel(incomingConnection net.Conn, destinationHost string, proxyURL *url.URL, config appConfig) error {
	conn, err := net.DialTimeout("tcp", proxyURL.Host, config.timeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	httpRequest := &http.Request{
		Method: "CONNECT",
		URL: &url.URL{
			Scheme: "http",
			Host:   destinationHost,
		},
	}
	err = httpRequest.Write(conn)
	if err != nil {
		return err
	}
	httpResp, err := http.ReadResponse(bufio.NewReader(conn), httpRequest)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		return fmt.Errorf("proxy returned status: %d", httpResp.StatusCode)
	}

	return duplex(incomingConnection, conn)
}
