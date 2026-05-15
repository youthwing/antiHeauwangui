// Package proxy runs a local HTTPS MITM proxy. It signs leaf certificates with
// the supplied CA, so any client trusting that CA will accept the man-in-the-
// middle connection. Only requests whose Host matches TargetDomain are
// intercepted; everything else is tunneled through unchanged.
package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/elazarl/goproxy"

	"wangui-tokengrab/internal/ca"
)

// Hit is what the proxy emits when it sees a Bearer token in an Authorization
// header on a target-domain request.
type Hit struct {
	Token string
	Host  string
	Path  string
}

type Server struct {
	Addr         string  // e.g. "127.0.0.1:8888"
	CA           *ca.CA  // forge leaf certs with this
	TargetDomain string  // e.g. "xhbcs.henau.edu.cn"
	OnHit        func(Hit)

	srv     *http.Server
	stopped chan struct{}
	once    sync.Once
}

func New(addr string, caObj *ca.CA, target string, onHit func(Hit)) *Server {
	return &Server{
		Addr:         addr,
		CA:           caObj,
		TargetDomain: target,
		OnHit:        onHit,
		stopped:      make(chan struct{}),
	}
}

// Start spins up the proxy in a goroutine. Returns immediately.
func (s *Server) Start() error {
	// Replace goproxy's built-in CA with ours so MITM leaf certs are signed
	// by the certificate the host has just been told to trust.
	goproxy.GoproxyCa = tls.Certificate{
		Certificate: [][]byte{s.CA.DER},
		PrivateKey:  s.CA.PrivKey,
		Leaf:        s.CA.Cert,
	}
	tlsConf := goproxy.TLSConfigFromCA(&goproxy.GoproxyCa)
	goproxy.OkConnect = &goproxy.ConnectAction{Action: goproxy.ConnectAccept, TLSConfig: tlsConf}
	goproxy.MitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectMitm, TLSConfig: tlsConf}
	goproxy.HTTPMitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectHTTPMitm, TLSConfig: tlsConf}
	goproxy.RejectConnect = &goproxy.ConnectAction{Action: goproxy.ConnectReject, TLSConfig: tlsConf}

	p := goproxy.NewProxyHttpServer()
	p.Verbose = false

	// Match Host header against the target domain (with optional :port).
	targetRe := regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(s.TargetDomain) + `(?::\d+)?$`)

	// Only MITM the target domain; for everything else, default goproxy
	// behaviour is to tunnel CONNECTs through unchanged.
	p.OnRequest(goproxy.ReqHostMatches(targetRe)).HandleConnect(goproxy.AlwaysMitm)

	// Inspect requests after TLS termination.
	p.OnRequest(goproxy.ReqHostMatches(targetRe)).DoFunc(
		func(req *http.Request, _ *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			if auth := req.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
				if token != "" && s.OnHit != nil {
					// Fire in goroutine so we don't block the request path
					go s.OnHit(Hit{Token: token, Host: req.Host, Path: req.URL.Path})
				}
			}
			return req, nil
		},
	)

	s.srv = &http.Server{
		Addr:    s.Addr,
		Handler: p,
	}
	go func() {
		err := s.srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("tokengrab proxy: %v\n", err)
		}
		close(s.stopped)
	}()
	return nil
}

// Stop performs a graceful shutdown with a short timeout.
func (s *Server) Stop() {
	s.once.Do(func() {
		if s.srv == nil {
			close(s.stopped)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.srv.Shutdown(ctx)
	})
	select {
	case <-s.stopped:
	case <-time.After(3 * time.Second):
		// give up — caller will exit
	}
}
