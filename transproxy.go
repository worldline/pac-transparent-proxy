package main

import (
	"net"
	"net/url"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

type appConfig struct {
	logLevel       log.Level
	timeout        time.Duration
	pacFileTimeout time.Duration
	pacFileTTL     time.Duration
	port           int
	proxyPacURI    *url.URL
	tunnel         bool
}

var parser pacParser

func handleConnection(incomingConnection *net.TCPConn, config appConfig) {
	defer incomingConnection.Close()
	destinationHost, destinationPort, err := getDestinationHost(incomingConnection)
	if err != nil {
		log.Error(err)
		return
	}
	log.Debugf("New connection %s -> %s", incomingConnection.RemoteAddr().String(), destinationHost)

	//TODO avoid being called directly
	proxyRule, err := parser.findProxy("", destinationHost)
	if err != nil {
		log.Errorf("End of connection %s -> %s : %s", incomingConnection.RemoteAddr().String(), destinationHost, err)
		return
	}
	log.Tracef("Rule for %s : %s", destinationHost, proxyRule)

	destinationRoute := net.JoinHostPort(destinationHost, strconv.Itoa(destinationPort))
	err = forward(incomingConnection, destinationRoute, proxyRule, config)
	if err != nil {
		log.Errorf("End of connection %s -> %s : %s", incomingConnection.RemoteAddr().String(), destinationHost, err)
		return
	}
	log.Debugf("End of connection %s -> %s", incomingConnection.RemoteAddr().String(), destinationHost)
}

func startServer(config appConfig) {
	addr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort("", strconv.Itoa(config.port)))
	if err != nil {
		log.Fatal(err)
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	parser.init(config)

	log.Infof("Listening on %s", l.Addr().String())

	for {
		conn, err := l.AcceptTCP()
		if err != nil {
			log.Error(err)
		} else {
			go handleConnection(conn, config)
		}
	}
}

func initLogger() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:          true,
		DisableLevelTruncation: true,
		TimestampFormat:        "2006-01-02T15:04:05.000",
	})
	log.SetLevel(log.InfoLevel)
}

func main() {
	initLogger()
	cli(func(config appConfig) {
		log.SetLevel(config.logLevel)
		startServer(config)
	})
}
