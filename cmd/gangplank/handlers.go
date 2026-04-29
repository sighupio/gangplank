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
	"crypto/rand"
	"encoding/base64"
	"fmt"
	htmltemplate "html/template"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/oauth2"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api/v1"

	"github.com/golang-jwt/jwt/v5"
	"sigs.k8s.io/yaml"

	"github.com/sighupio/gangplank/internal/oidc"
	"github.com/sighupio/gangplank/templates"
)

const stateTokenBytes = 32

// userInfo stores information about an authenticated user.
type userInfo struct {
	ClusterName  string
	Username     string
	Claims       jwt.MapClaims
	KubeCfgUser  string
	IDToken      string
	RefreshToken string
	ClientID     string
	ClientSecret string
	IssuerURL    string
	APIServerURL string
	ClusterCA    string
	IDPCAb64     string
	HTTPPath     string
	Namespace    string
}

// homeInfo is used to store dynamic properties on.
type homeInfo struct {
	HTTPPath string
}

func (s *server) serveTemplate(tmplFile string, data any, w http.ResponseWriter) {
	tmpl := htmltemplate.New(tmplFile)

	// Use custom templates if provided
	if s.cfg.CustomHTMLTemplatesDir != "" {
		templatePath := filepath.Join(s.cfg.CustomHTMLTemplatesDir, tmplFile)
		templateData, err := os.ReadFile(templatePath)
		if err != nil {
			slog.Error("Failed to find template asset", "asset", tmplFile, "path", templatePath)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tmpl, err = tmpl.Parse(string(templateData))
		if err != nil {
			slog.Error("Failed to parse template", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		var err error

		tmpl, err = tmpl.ParseFS(templates.FS, tmplFile)
		if err != nil {
			slog.Error("Failed to parse template", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err := tmpl.ExecuteTemplate(w, tmplFile, data); err != nil {
		// Headers may already be sent, so we can only log the error.
		slog.Error("Failed to execute template", "error", err)
	}
}

func generateKubeConfig(cfg *userInfo) clientcmdapi.Config {
	// fill out kubeconfig structure
	kcfg := clientcmdapi.Config{
		Kind:           "Config",
		APIVersion:     "v1",
		CurrentContext: cfg.ClusterName,
		Clusters: []clientcmdapi.NamedCluster{
			{
				Name: cfg.ClusterName,
				Cluster: clientcmdapi.Cluster{
					Server:                   cfg.APIServerURL,
					CertificateAuthorityData: []byte(cfg.ClusterCA),
				},
			},
		},
		Contexts: []clientcmdapi.NamedContext{
			{
				Name: cfg.ClusterName,
				Context: clientcmdapi.Context{
					Cluster:   cfg.ClusterName,
					AuthInfo:  cfg.KubeCfgUser,
					Namespace: cfg.Namespace,
				},
			},
		},
		AuthInfos: []clientcmdapi.NamedAuthInfo{
			{
				Name: cfg.KubeCfgUser,
				AuthInfo: clientcmdapi.AuthInfo{
					AuthProvider: &clientcmdapi.AuthProviderConfig{
						Name: "oidc",
						Config: map[string]string{
							"client-id":                      cfg.ClientID,
							"client-secret":                  cfg.ClientSecret,
							"id-token":                       cfg.IDToken,
							"idp-issuer-url":                 cfg.IssuerURL,
							"refresh-token":                  cfg.RefreshToken,
							"idp-certificate-authority-data": cfg.IDPCAb64,
						},
					},
				},
			},
		},
	}
	return kcfg
}

// sanitizeFilename replaces characters that could cause header injection.
func sanitizeFilename(name string) string {
	return strings.NewReplacer(
		`"`, "_", `\`, "_", `/`, "_", "\n", "_", "\r", "_",
	).Replace(name)
}

func (s *server) loginRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := s.gangplankUserSession.Session.Get(r, "gangplank_id_token")
		if err != nil {
			http.Redirect(w, r, s.cfg.GetRootPathPrefix(), http.StatusTemporaryRedirect)
			return
		}

		if session.Values["id_token"] == nil {
			http.Redirect(w, r, s.cfg.GetRootPathPrefix(), http.StatusTemporaryRedirect)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *server) homeHandler(w http.ResponseWriter, _ *http.Request) {
	data := &homeInfo{
		HTTPPath: s.cfg.HTTPPath,
	}

	s.serveTemplate("home.tmpl", data, w)
}

func (s *server) loginHandler(w http.ResponseWriter, r *http.Request) {
	b := make([]byte, stateTokenBytes)
	// From the rand.Read signature: It never returns an error, and always fills b entirely.
	_, _ = rand.Read(b)
	state := url.QueryEscape(base64.StdEncoding.EncodeToString(b))

	session, err := s.gangplankUserSession.Session.Get(r, "gangplank")
	if err != nil {
		slog.Error("Got an error in login", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	session.Values["state"] = state
	err = session.Save(r, w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	audience := oauth2.SetAuthURLParam("audience", s.cfg.Audience)
	authURL := s.oauth2Cfg.AuthCodeURL(state, audience)

	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (s *server) cleanupAllSessions(w http.ResponseWriter, r *http.Request) {
	err := s.gangplankUserSession.CleanupAll(w, r, "gangplank", "gangplank_id_token", "gangplank_refresh_token")
	if err != nil {
		slog.Error("Failed to cleanup sessions", "error", err)
	}
}

func (s *server) logoutHandler(w http.ResponseWriter, r *http.Request) {
	s.cleanupAllSessions(w, r)
	http.Redirect(w, r, s.cfg.GetRootPathPrefix(), http.StatusTemporaryRedirect)
}

func (s *server) callbackHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.WithValue(r.Context(), oauth2.HTTPClient, s.transportConfig.HTTPClient)

	// load up session cookies
	session, err := s.gangplankUserSession.Session.Get(r, "gangplank")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sessionIDToken, err := s.gangplankUserSession.Session.Get(r, "gangplank_id_token")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sessionRefreshToken, err := s.gangplankUserSession.Session.Get(r, "gangplank_refresh_token")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// verify the state string
	state := r.URL.Query().Get("state")

	if state != session.Values["state"] {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	// use the access code to retrieve a token
	code := r.URL.Query().Get("code")
	token, err := s.o2token.Exchange(ctx, code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sessionIDToken.Values["id_token"] = token.Extra("id_token")
	sessionRefreshToken.Values["refresh_token"] = token.RefreshToken

	// save the session cookies
	err = session.Save(r, w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = sessionIDToken.Save(r, w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = sessionRefreshToken.Save(r, w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("%s/commandline", s.cfg.HTTPPath), http.StatusSeeOther)
}

func (s *server) commandlineHandler(w http.ResponseWriter, r *http.Request) {
	info := s.generateInfo(w, r)
	if info == nil {
		// generateInfo writes to the ResponseWriter if it encounters an error.
		// TODO(abrand): Refactor this.
		return
	}

	s.serveTemplate("commandline.tmpl", info, w)
}

func (s *server) kubeConfigHandler(w http.ResponseWriter, r *http.Request) {
	info := s.generateInfo(w, r)
	if info == nil {
		// generateInfo writes to the ResponseWriter if it encounters an error.
		// TODO(abrand): Refactor this.
		return
	}

	d, err := yaml.Marshal(generateKubeConfig(info))
	if err != nil {
		slog.Error("Error creating kubeconfig", "error", err)
		http.Error(w, "Error creating kubeconfig", http.StatusInternalServerError)
		return
	}

	filename := r.URL.Query().Get("filename")
	if filename == "" {
		filename = info.KubeCfgUser
	}
	filename = sanitizeFilename(filename)

	// tell the browser the returned content should be downloaded
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`Attachment; filename="%s"`, filename))
	//nolint:gosec // content is yaml.Marshal output served as attachment, not rendered
	if _, writeErr := w.Write(d); writeErr != nil {
		// Headers already sent, so we can only log the error.
		slog.Error("Failed to write kubeconfig response", "error", writeErr)
	}
}

func (s *server) generateInfo(w http.ResponseWriter, r *http.Request) *userInfo {
	caBytes, idpCAb64Bytes := s.readCAFiles()

	// load the session cookies
	sessionIDToken, err := s.gangplankUserSession.Session.Get(r, "gangplank_id_token")
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return nil
	}
	sessionRefreshToken, err := s.gangplankUserSession.Session.Get(r, "gangplank_refresh_token")
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return nil
	}

	idToken, ok := sessionIDToken.Values["id_token"].(string)
	if !ok {
		s.cleanupAllSessions(w, r)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return nil
	}

	refreshToken, ok := sessionRefreshToken.Values["refresh_token"].(string)
	if !ok {
		s.cleanupAllSessions(w, r)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return nil
	}

	jwtToken, err := oidc.ParseToken(idToken, s.cfg.ClientSecret)
	if err != nil {
		http.Error(w, "Could not parse JWT", http.StatusInternalServerError)
		return nil
	}

	claims, ok := jwtToken.Claims.(jwt.MapClaims)
	if !ok {
		http.Error(w, "Could not parse JWT claims", http.StatusInternalServerError)
		return nil
	}
	username, ok := claims[s.cfg.UsernameClaim].(string)
	if !ok {
		http.Error(w, "Could not parse Username claim", http.StatusInternalServerError)
		return nil
	}

	kubeCfgUser := strings.Join([]string{username, s.cfg.ClusterName}, "@")

	if s.cfg.EmailClaim != "" {
		slog.Warn(
			"Using the Email Claim config setting is deprecated. Gangplank uses `UsernameClaim@ClusterName`. This field will be removed in a future version.",
		)
	}

	issuerURL, ok := claims["iss"].(string)
	if !ok {
		http.Error(w, "Could not parse Issuer URL claim", http.StatusInternalServerError)
		return nil
	}

	if s.cfg.ClientSecret == "" {
		slog.Warn(
			"Setting an empty Client Secret should only be done if you have no other option and is an inherent security risk.",
		)
	}

	info := &userInfo{
		ClusterName:  s.cfg.ClusterName,
		Username:     username,
		Claims:       claims,
		KubeCfgUser:  kubeCfgUser,
		IDToken:      idToken,
		RefreshToken: refreshToken,
		ClientID:     s.cfg.ClientID,
		ClientSecret: s.cfg.ClientSecret,
		IssuerURL:    issuerURL,
		APIServerURL: s.cfg.APIServerURL,
		ClusterCA:    string(caBytes),
		IDPCAb64:     string(idpCAb64Bytes),
		HTTPPath:     s.cfg.HTTPPath,
		Namespace:    s.cfg.Namespace,
	}
	return info
}

func (s *server) readCAFiles() ([]byte, []byte) {
	if s.cfg.RemoveCAFromKubeconfig {
		return nil, nil
	}

	caBytes, err := os.ReadFile(s.cfg.ClusterCAPath)
	if err != nil {
		slog.Error("Failed to open CA file", "error", err)
		return nil, nil
	}

	var idpCAb64Bytes []byte
	if s.cfg.IDPCAPath != "" {
		idpBytes, idpErr := os.ReadFile(s.cfg.IDPCAPath)
		if idpErr != nil {
			slog.Error("Failed to open IDP file", "error", idpErr)
		} else {
			idpCAb64Bytes = make([]byte, base64.StdEncoding.EncodedLen(len(idpBytes)))
			base64.StdEncoding.Encode(idpCAb64Bytes, idpBytes)
		}
	}

	return caBytes, idpCAb64Bytes
}
