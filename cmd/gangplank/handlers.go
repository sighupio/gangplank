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
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/golang-jwt/jwt/v5"
	"github.com/sighupio/gangplank/internal/oidc"
	"github.com/sighupio/gangplank/templates"
	"golang.org/x/oauth2"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api/v1"
)

const (
	templatesBase = "/templates"
)

// userInfo stores information about an authenticated user
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
	HTTPPath     string
	Namespace    string
}

// homeInfo is used to store dynamic properties on
type homeInfo struct {
	HTTPPath string
}

func serveTemplate(tmplFile string, data interface{}, w http.ResponseWriter) {
	tmpl := htmltemplate.New(tmplFile)

	// Use custom templates if provided
	if cfg.CustomHTMLTemplatesDir != "" {
		templatePath := filepath.Join(cfg.CustomHTMLTemplatesDir, tmplFile)
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
		}
	}

	tmpl.ExecuteTemplate(w, tmplFile, data)
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
							"client-id":      cfg.ClientID,
							"client-secret":  cfg.ClientSecret,
							"id-token":       cfg.IDToken,
							"idp-issuer-url": cfg.IssuerURL,
							"refresh-token":  cfg.RefreshToken,
						},
					},
				},
			},
		},
	}
	return kcfg
}

func loginRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := gangplankUserSession.Session.Get(r, "gangplank_id_token")
		if err != nil {
			http.Redirect(w, r, cfg.GetRootPathPrefix(), http.StatusTemporaryRedirect)
			return
		}

		if session.Values["id_token"] == nil {
			http.Redirect(w, r, cfg.GetRootPathPrefix(), http.StatusTemporaryRedirect)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	data := &homeInfo{
		HTTPPath: cfg.HTTPPath,
	}

	serveTemplate("home.tmpl", data, w)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {

	b := make([]byte, 32)
	rand.Read(b)
	state := url.QueryEscape(base64.StdEncoding.EncodeToString(b))

	session, err := gangplankUserSession.Session.Get(r, "gangplank")
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

	audience := oauth2.SetAuthURLParam("audience", cfg.Audience)
	url := oauth2Cfg.AuthCodeURL(state, audience)

	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	gangplankUserSession.Cleanup(w, r, "gangplank")
	gangplankUserSession.Cleanup(w, r, "gangplank_id_token")
	gangplankUserSession.Cleanup(w, r, "gangplank_refresh_token")
	http.Redirect(w, r, cfg.GetRootPathPrefix(), http.StatusTemporaryRedirect)
}

func callbackHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.WithValue(r.Context(), oauth2.HTTPClient, transportConfig.HTTPClient)

	// load up session cookies
	session, err := gangplankUserSession.Session.Get(r, "gangplank")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sessionIDToken, err := gangplankUserSession.Session.Get(r, "gangplank_id_token")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sessionRefreshToken, err := gangplankUserSession.Session.Get(r, "gangplank_refresh_token")
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
	token, err := o2token.Exchange(ctx, code)
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

	http.Redirect(w, r, fmt.Sprintf("%s/commandline", cfg.HTTPPath), http.StatusSeeOther)
}

func commandlineHandler(w http.ResponseWriter, r *http.Request) {
	info := generateInfo(w, r)
	if info == nil {
		// generateInfo writes to the ResponseWriter if it encounters an error.
		// TODO(abrand): Refactor this.
		return
	}

	serveTemplate("commandline.tmpl", info, w)
}

func kubeConfigHandler(w http.ResponseWriter, r *http.Request) {
	info := generateInfo(w, r)
	if info == nil {
		// generateInfo writes to the ResponseWriter if it encounters an error.
		// TODO(abrand): Refactor this.
		return
	}

	d, err := yaml.Marshal(generateKubeConfig(info))
	if err != nil {
		slog.Error("Error creating kubeconfig", "error", err.Error())
		http.Error(w, "Error creating kubeconfig", http.StatusInternalServerError)
		return
	}

	// tell the browser the returned content should be downloaded
	w.Header().Add("Content-Disposition", "Attachment")
	w.Write(d)
}

func generateInfo(w http.ResponseWriter, r *http.Request) *userInfo {
	// read in public ca.crt to output in commandline copy/paste commands
	file, err := os.Open(cfg.ClusterCAPath)
	if err != nil {
		// let us know that we couldn't open the file. This only cause missing output
		// does not impact actual function of program
		slog.Error("Failed to open CA file", "error", err)
	}
	defer file.Close()
	caBytes, err := io.ReadAll(file)
	if err != nil {
		slog.Warn("Could not read CA file", "error", err)
	}

	if cfg.RemoveCAFromKubeconfig {
		caBytes = []byte{}
	}

	// load the session cookies
	sessionIDToken, err := gangplankUserSession.Session.Get(r, "gangplank_id_token")
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return nil
	}
	sessionRefreshToken, err := gangplankUserSession.Session.Get(r, "gangplank_refresh_token")
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return nil
	}

	idToken, ok := sessionIDToken.Values["id_token"].(string)
	if !ok {
		gangplankUserSession.Cleanup(w, r, "gangplank")
		gangplankUserSession.Cleanup(w, r, "gangplank_id_token")
		gangplankUserSession.Cleanup(w, r, "gangplank_refresh_token")

		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return nil
	}

	refreshToken, ok := sessionRefreshToken.Values["refresh_token"].(string)
	if !ok {
		gangplankUserSession.Cleanup(w, r, "gangplank")
		gangplankUserSession.Cleanup(w, r, "gangplank_id_token")
		gangplankUserSession.Cleanup(w, r, "gangplank_refresh_token")

		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return nil
	}

	jwtToken, err := oidc.ParseToken(idToken, cfg.ClientSecret)
	if err != nil {
		http.Error(w, "Could not parse JWT", http.StatusInternalServerError)
		return nil
	}

	claims := jwtToken.Claims.(jwt.MapClaims)
	username, ok := claims[cfg.UsernameClaim].(string)
	if !ok {
		http.Error(w, "Could not parse Username claim", http.StatusInternalServerError)
		return nil
	}

	kubeCfgUser := strings.Join([]string{username, cfg.ClusterName}, "@")

	if cfg.EmailClaim != "" {
		slog.Warn("Using the Email Claim config setting is deprecated. Gangplank uses `UsernameClaim@ClusterName`. This field will be removed in a future version.")
	}

	issuerURL, ok := claims["iss"].(string)
	if !ok {
		http.Error(w, "Could not parse Issuer URL claim", http.StatusInternalServerError)
		return nil
	}

	if cfg.ClientSecret == "" {
		slog.Warn("Setting an empty Client Secret should only be done if you have no other option and is an inherent security risk.")
	}

	info := &userInfo{
		ClusterName:  cfg.ClusterName,
		Username:     username,
		Claims:       claims,
		KubeCfgUser:  kubeCfgUser,
		IDToken:      idToken,
		RefreshToken: refreshToken,
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		IssuerURL:    issuerURL,
		APIServerURL: cfg.APIServerURL,
		ClusterCA:    string(caBytes),
		HTTPPath:     cfg.HTTPPath,
		Namespace:    cfg.Namespace,
	}
	return info
}
