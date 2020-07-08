package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/jackwakefield/gopac"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/singleflight"
)

type pacParser struct {
	pool                  *sync.Pool
	singleflightGroup     singleflight.Group
	pacFileURI            *url.URL
	pacFileTTL            time.Duration
	lastUpdateDate        time.Time
	currentPacFileContent []byte
	lastModifiedHeader    string
	httpClient            *http.Client
	lastError             error
}

func (p *pacParser) createPacParserInstance(pacFileContent []byte) (*gopac.Parser, error) {
	// Load PAC file
	parser := new(gopac.Parser)
	err := parser.ParseBytes(pacFileContent)
	return parser, err
}

func (p *pacParser) mustRefresh() bool {
	return p.lastUpdateDate.Add(p.pacFileTTL).Before(time.Now())
}

func (p *pacParser) fallbackOnError(err error) (interface{}, error) {
	logMessage := "An error happened while retrieving PAC file, no proxy will be used : %s"
	if p.lastError == nil || p.lastError.Error() != err.Error() {
		log.Errorf(logMessage, err)
	} else {
		log.Debugf(logMessage, err)
	}
	p.lastError = err
	p.currentPacFileContent = nil
	p.pool = nil
	p.lastModifiedHeader = ""
	return nil, nil
}

func (p *pacParser) refreshPacFile() (interface{}, error) {
	// Check again, to be sure that pool has not been refreshed since check in findProxy()
	if !p.mustRefresh() {
		return p.pool, nil
	}

	defer func() { p.lastUpdateDate = time.Now() }()

	// Retrieve Pac File
	req := http.Request{
		Method: http.MethodGet,
		URL:    p.pacFileURI,
		Header: http.Header{},
	}
	if len(p.lastModifiedHeader) > 0 {
		req.Header.Set("If-Modified-Since", p.lastModifiedHeader)
	}
	response, err := p.httpClient.Do(&req)
	// Connection error
	if err != nil {
		return p.fallbackOnError(err)
	}
	defer response.Body.Close()

	// 304 Not changed
	if p.pool != nil && response.StatusCode == http.StatusNotModified {
		log.Debug("PAC file didn't change (HTTP 304), do nothing")
		p.lastError = nil
		return p.pool, nil
	}

	// Invalid status codes
	if response.StatusCode != http.StatusOK {
		return p.fallbackOnError(fmt.Errorf("invalid status while reading PAC file: %d", response.StatusCode))
	}
	p.lastModifiedHeader = response.Header.Get("last-modified")

	// Parse body
	pacFileContent, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return p.fallbackOnError(err)
	}

	// Same file: do nothing
	if bytes.Compare(pacFileContent, p.currentPacFileContent) == 0 {
		log.Debug("PAC file didn't change (same content), do nothing")
		p.lastError = nil
		return p.pool, nil
	}

	// Parse PAC file
	parser, err := p.createPacParserInstance(pacFileContent)
	if err != nil {
		return p.fallbackOnError(err)
	}
	p.currentPacFileContent = pacFileContent
	pool := &sync.Pool{
		New: func() interface{} {
			parser, _ := p.createPacParserInstance(pacFileContent)
			return parser
		},
	}
	pool.Put(parser)
	p.pool = pool
	p.lastError = nil
	log.Infof("New PAC file loaded successfully")
	return pool, nil
}

func (p *pacParser) getRefreshedPool() *sync.Pool {
	pool, _, _ := p.singleflightGroup.Do("pacFile", p.refreshPacFile)
	if pool == nil {
		return nil
	}
	return pool.(*sync.Pool)
}

func (p *pacParser) init(config appConfig) {
	p.lastUpdateDate = time.Unix(0, 0)
	p.pacFileURI = config.proxyPacURI
	p.pacFileTTL = config.pacFileTTL

	t := &http.Transport{}
	t.RegisterProtocol("file", http.NewFileTransport(http.Dir("/")))
	p.httpClient = &http.Client{Transport: t, Timeout: config.pacFileTimeout}
}

func (p *pacParser) findProxy(url string, host string) (string, error) {
	currentPool := p.pool
	if p.mustRefresh() {
		currentPool = p.getRefreshedPool()
	}
	// No PAC File : use DIRECT connection
	if currentPool == nil {
		return "DIRECT", nil
	}
	parser := currentPool.Get().(*gopac.Parser)
	defer currentPool.Put(parser)
	return parser.FindProxy(url, host)
}
