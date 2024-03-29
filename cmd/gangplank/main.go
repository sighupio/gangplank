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
	"github.com/sighupio/gangplank/internal/config"
	"github.com/sighupio/gangplank/internal/oidc"
	"github.com/sighupio/gangplank/internal/session"
	"github.com/sighupio/gangplank/static"
	"golang.org/x/oauth2"
)

var cfg *config.Config
var oauth2Cfg *oauth2.Config
var o2token oidc.OAuth2Token
var gangplankUserSession *session.Session
var transportConfig *config.TransportConfig

// wrapper function for http logging
func httpLogger(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer slog.Debug("HTTP log", "method", r.Method, "url", r.URL, "remote-addr", r.RemoteAddr)
		fn(w, r)
	}
}

func main() {
	cfgFile := flag.String("config", "", "The config file to use.")
	logLevel := flag.String("log-level", "info", "The log level to use. (debug, info, warn, error)")
	flag.Parse()

	var logLevelVar = new(slog.LevelVar)

	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevelVar})
	slog.SetDefault(slog.New(h))

	if err := logLevelVar.UnmarshalText([]byte(*logLevel)); err != nil {
		slog.Error("Could not parse log level", "error", err)
		os.Exit(1)
	}

	var err error
	cfg, err = config.NewConfig(*cfgFile)
	if err != nil {
		slog.Error("Could not parse config file", "error", err)
		os.Exit(1)
	}

	oauth2Cfg = &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Scopes:       cfg.Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  cfg.AuthorizeURL,
			TokenURL: cfg.TokenURL,
		},
	}

	o2token = &oidc.Token{
		OAuth2Cfg: oauth2Cfg,
	}

	transportConfig = config.NewTransportConfig(cfg.TrustedCAPath)
	gangplankUserSession = session.New(cfg.SessionSecurityKey)

	loginRequiredHandlers := alice.New(loginRequired)

	http.HandleFunc(cfg.GetRootPathPrefix(), httpLogger(homeHandler))
	http.HandleFunc(fmt.Sprintf("%s/static/", cfg.HTTPPath), httpLogger(http.StripPrefix(fmt.Sprintf("%s/static/", cfg.HTTPPath), http.FileServerFS(static.FS)).ServeHTTP))
	http.HandleFunc(fmt.Sprintf("%s/login", cfg.HTTPPath), httpLogger(loginHandler))
	http.HandleFunc(fmt.Sprintf("%s/callback", cfg.HTTPPath), httpLogger(callbackHandler))

	// middleware'd routes
	http.Handle(fmt.Sprintf("%s/logout", cfg.HTTPPath), loginRequiredHandlers.ThenFunc(logoutHandler))
	http.Handle(fmt.Sprintf("%s/commandline", cfg.HTTPPath), loginRequiredHandlers.ThenFunc(commandlineHandler))
	http.Handle(fmt.Sprintf("%s/kubeconf", cfg.HTTPPath), loginRequiredHandlers.ThenFunc(kubeConfigHandler))

	bindAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	// create http server with timeouts
	httpServer := &http.Server{
		Addr:         bindAddr,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	if cfg.ServeTLS {
		// update http server with TLS config
		httpServer.TLSConfig = &tls.Config{
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			},
			PreferServerCipherSuites: true,
			MinVersion:               tls.VersionTLS12,
		}
	}

	// start up the http server
	go func() {
		slog.Info("Gangplank started", "address", bindAddr)

		// exit with FATAL logging why we could not start
		// example: FATA[0000] listen tcp 0.0.0.0:8080: bind: address already in use
		if cfg.ServeTLS {
			if err := httpServer.ListenAndServeTLS(cfg.CertFile, cfg.KeyFile); err != nil {
				slog.Error("Could not start HTTPS server", "error", err)
				os.Exit(1)
			}
		} else {
			if err := httpServer.ListenAndServe(); err != nil {
				slog.Error("Could not start HTTP server", "error", err)
				os.Exit(1)
			}
		}
	}()

	// create channel listening for signals so we can have graceful shutdowns
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	slog.Info("Shutdown signal received, exiting")
	// close the HTTP server
	httpServer.Shutdown(context.Background())
}
