// Copyright 2017-present SIGHUP s.r.l
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/justinas/alice"
	"golang.org/x/oauth2"

	"github.com/sighupio/gangplank/internal/config"
	"github.com/sighupio/gangplank/internal/oidc"
	"github.com/sighupio/gangplank/internal/session"
	"github.com/sighupio/gangplank/static"
)

const httpServerTimeout = 10 * time.Second

type server struct {
	cfg                  *config.Config
	oauth2Cfg            *oauth2.Config
	o2token              oidc.OAuth2Token
	gangplankUserSession *session.Session
	transportConfig      *config.TransportConfig
}

// wrapper function for http logging.
func httpLogger(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer slog.Debug("HTTP log", "method", r.Method, "url", r.URL, "remote-addr", r.RemoteAddr)
		fn(w, r)
	}
}

func newServer(cfgFile string) (*server, error) {
	cfg, err := config.NewConfig(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("could not parse config file: %w", err)
	}

	oauth2Cfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Scopes:       cfg.Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  cfg.AuthorizeURL,
			TokenURL: cfg.TokenURL,
		},
	}

	userSession, err := session.New(cfg.SessionSecurityKey, cfg.ServeTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &server{
		cfg:                  cfg,
		oauth2Cfg:            oauth2Cfg,
		o2token:              &oidc.Token{OAuth2Cfg: oauth2Cfg},
		transportConfig:      config.NewTransportConfig(cfg.TrustedCAPath),
		gangplankUserSession: userSession,
	}, nil
}

func main() {
	cfgFile := flag.String("config", "", "The config file to use.")
	logLevel := flag.String("log-level", "info", "The log level to use. (debug, info, warn, error)")
	flag.Parse()

	logLevelVar := new(slog.LevelVar)

	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevelVar})
	slog.SetDefault(slog.New(h))

	if err := logLevelVar.UnmarshalText([]byte(*logLevel)); err != nil {
		slog.Error("Could not parse log level", "error", err)
		os.Exit(1)
	}

	s, err := newServer(*cfgFile)
	if err != nil {
		slog.Error("Could not initialize server", "error", err)
		os.Exit(1)
	}

	loginRequiredHandlers := alice.New(s.loginRequired)

	http.HandleFunc(s.cfg.GetRootPathPrefix(), httpLogger(s.homeHandler))
	http.HandleFunc(
		fmt.Sprintf("%s/static/", s.cfg.HTTPPath),
		httpLogger(http.StripPrefix(fmt.Sprintf("%s/static/", s.cfg.HTTPPath), http.FileServerFS(static.FS)).ServeHTTP),
	)
	http.HandleFunc(fmt.Sprintf("%s/login", s.cfg.HTTPPath), httpLogger(s.loginHandler))
	http.HandleFunc(fmt.Sprintf("%s/callback", s.cfg.HTTPPath), httpLogger(s.callbackHandler))

	// middleware'd routes
	http.Handle(fmt.Sprintf("%s/logout", s.cfg.HTTPPath), loginRequiredHandlers.ThenFunc(s.logoutHandler))
	http.Handle(fmt.Sprintf("%s/commandline", s.cfg.HTTPPath), loginRequiredHandlers.ThenFunc(s.commandlineHandler))
	http.Handle(fmt.Sprintf("%s/kubeconf", s.cfg.HTTPPath), loginRequiredHandlers.ThenFunc(s.kubeConfigHandler))

	bindAddr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	// create http server with timeouts
	httpServer := &http.Server{
		Addr:         bindAddr,
		ReadTimeout:  httpServerTimeout,
		WriteTimeout: httpServerTimeout,
	}

	if s.cfg.ServeTLS {
		// update http server with TLS config
		httpServer.TLSConfig = &tls.Config{
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			},
			MinVersion: tls.VersionTLS12,
		}
	}

	// start up the http server
	go func() {
		slog.Info("Gangplank started", "address", bindAddr)

		// exit with FATAL logging why we could not start
		// example: FATA[0000] listen tcp 0.0.0.0:8080: bind: address already in use
		if s.cfg.ServeTLS {
			if serveErr := httpServer.ListenAndServeTLS(s.cfg.CertFile, s.cfg.KeyFile); serveErr != nil {
				slog.Error("Could not start HTTPS server", "error", serveErr)
				os.Exit(1)
			}
		} else {
			if serveErr := httpServer.ListenAndServe(); serveErr != nil {
				slog.Error("Could not start HTTP server", "error", serveErr)
				os.Exit(1)
			}
		}
	}()

	gracefulShutdown(httpServer)
}

func gracefulShutdown(srv *http.Server) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	slog.Info("Shutdown signal received, exiting")
	ctx, cancel := context.WithTimeout(context.Background(), httpServerTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Error shutting down HTTP server", "error", err)
	}
}
