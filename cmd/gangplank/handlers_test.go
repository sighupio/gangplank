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
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/oauth2"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api/v1"

	"sigs.k8s.io/yaml"

	"github.com/sighupio/gangplank/internal/config"
	"github.com/sighupio/gangplank/internal/session"
)

const (
	testClientSecret = "qwertyuiopasdfghjklzxcvbnm123456"
	testIDToken      = "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJHYW5nd2F5VGVzdCIsImlhdCI6MTU0MDA0NjM0NywiZXhwIjoxODg3MjAxNTQ3LCJhdWQiOiJnYW5nd2F5LmhlcHRpby5jb20iLCJzdWIiOiJnYW5nd2F5QGhlcHRpby5jb20iLCJHaXZlbk5hbWUiOiJHYW5nIiwiU3VybmFtZSI6IldheSIsIkVtYWlsIjoiZ2FuZ3dheUBoZXB0aW8uY29tIiwiR3JvdXBzIjoiZGV2LGFkbWluIn0.zNG4Dnxr76J0p4phfsAUYWunioct0krkMiunMynlQsU"
)

func newTestServer(t *testing.T) *server {
	t.Helper()

	userSession, err := session.New("test", false)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	oauth2Cfg := &oauth2.Config{
		ClientID:     "cfg.ClientID",
		ClientSecret: testClientSecret,
		RedirectURL:  "cfg.RedirectURL",
	}

	return &server{
		gangplankUserSession: userSession,
		transportConfig:      config.NewTransportConfig(""),
		oauth2Cfg:            oauth2Cfg,
		o2token:              &FakeToken{OAuth2Cfg: oauth2Cfg},
	}
}

func TestHomeHandler(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	ts := newTestServer(t)
	ts.cfg = &config.Config{
		HTTPPath: "",
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ts.homeHandler)

	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestCallbackHandler(t *testing.T) {
	tests := map[string]struct {
		params             map[string]string
		expectedStatusCode int
	}{
		"default": {
			params: map[string]string{
				"state": "Uv38ByGCZU8WP18PmmIdcpVmx00QA3xNe7sEB9Hixkk=",
				"code":  "0cj0VQzNl36e4P2L&state=jdep4ov52FeUuzWLDDtSXaF4b5%2F%2FCUJ52xlE69ehnQ8%3D",
			},
			expectedStatusCode: http.StatusSeeOther,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ts := newTestServer(t)
			ts.cfg = &config.Config{
				HTTPPath: "/foo",
			}

			rsp := NewRecorder()
			req, err := http.NewRequest(http.MethodGet, "/callback", nil)
			if err != nil {
				t.Fatal(err)
			}

			// Create request
			session, err := ts.gangplankUserSession.Session.Get(req, "gangplank")
			if err != nil {
				t.Fatalf("Error getting session: %v", err)
			}

			// Create state session variable
			session.Values["state"] = tc.params["state"]
			if err = session.Save(req, rsp); err != nil {
				t.Fatal(err)
			}

			// Add query params to request
			q := req.URL.Query()
			for k, v := range tc.params {
				q.Add(k, v)
			}
			req.URL.RawQuery = q.Encode()

			handler := http.HandlerFunc(ts.callbackHandler)

			// Call Handler
			handler.ServeHTTP(rsp, req)

			// Validate!
			if status := rsp.Code; status != tc.expectedStatusCode {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tc.expectedStatusCode)
			}
		})
	}
}
func TestCommandLineHandler(t *testing.T) {
	tests := map[string]struct {
		params                     map[string]string
		emailClaim                 string
		usernameClaim              string
		expectedStatusCode         int
		expectedUsernameInTemplate string
	}{
		"default": {
			params: map[string]string{
				"state":         "Uv38ByGCZU8WP18PmmIdcpVmx00QA3xNe7sEB9Hixkk=",
				"id_token":      testIDToken,
				"refresh_token": "bar",
				"code":          "0cj0VQzNl36e4P2L&state=jdep4ov52FeUuzWLDDtSXaF4b5%2F%2FCUJ52xlE69ehnQ8%3D",
			},
			expectedStatusCode:         http.StatusOK,
			expectedUsernameInTemplate: "gangway@heptio.com",
			emailClaim:                 "Email",
			usernameClaim:              "sub",
		},
		"incorrect username claim": {
			params: map[string]string{
				"state":         "Uv38ByGCZU8WP18PmmIdcpVmx00QA3xNe7sEB9Hixkk=",
				"id_token":      testIDToken,
				"refresh_token": "bar",
				"code":          "0cj0VQzNl36e4P2L&state=jdep4ov52FeUuzWLDDtSXaF4b5%2F%2FCUJ52xlE69ehnQ8%3D",
			},
			expectedStatusCode: http.StatusInternalServerError,
			emailClaim:         "Email",
			usernameClaim:      "meh",
		},
		"no email claim": {
			params: map[string]string{
				"state":         "Uv38ByGCZU8WP18PmmIdcpVmx00QA3xNe7sEB9Hixkk=",
				"id_token":      testIDToken,
				"refresh_token": "bar",
				"code":          "0cj0VQzNl36e4P2L&state=jdep4ov52FeUuzWLDDtSXaF4b5%2F%2FCUJ52xlE69ehnQ8%3D",
			},
			expectedStatusCode:         http.StatusOK,
			expectedUsernameInTemplate: "gangway@heptio.com@cluster1",
			usernameClaim:              "sub",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ts := newTestServer(t)
			ts.cfg = &config.Config{
				HTTPPath:      "/foo",
				EmailClaim:    tc.emailClaim,
				UsernameClaim: tc.usernameClaim,
				ClusterName:   "cluster1",
				APIServerURL:  "https://kubernetes",
				ClientSecret:  testClientSecret,
			}

			rsp := NewRecorder()
			req, err := http.NewRequest(http.MethodGet, "/callback", nil)
			if err != nil {
				t.Fatal(err)
			}

			// Create request
			session, err := ts.gangplankUserSession.Session.Get(req, "gangplank")
			if err != nil {
				t.Fatalf("Error getting session: %v", err)
			}
			sessionIDToken, err := ts.gangplankUserSession.Session.Get(req, "gangplank_id_token")
			if err != nil {
				t.Fatalf("Error getting session: %v", err)
			}
			sessionRefreshToken, err := ts.gangplankUserSession.Session.Get(req, "gangplank_refresh_token")
			if err != nil {
				t.Fatalf("Error getting session: %v", err)
			}

			// Create state session variable
			session.Values["state"] = tc.params["state"]
			sessionIDToken.Values["id_token"] = tc.params["id_token"]
			sessionRefreshToken.Values["refresh_token"] = tc.params["refresh_token"]
			if err = session.Save(req, rsp); err != nil {
				t.Fatal(err)
			}
			if err = sessionIDToken.Save(req, rsp); err != nil {
				t.Fatal(err)
			}
			if err = sessionRefreshToken.Save(req, rsp); err != nil {
				t.Fatal(err)
			}

			// Add query params to request
			q := req.URL.Query()
			for k, v := range tc.params {
				q.Add(k, v)
			}
			req.URL.RawQuery = q.Encode()

			handler := http.HandlerFunc(ts.commandlineHandler)

			// Call Handler
			handler.ServeHTTP(rsp, req)

			// Validate!
			if status := rsp.Code; status != tc.expectedStatusCode {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tc.expectedStatusCode)
			}
			// if response code is OK then check that username is correct in resultant template
			if rsp.Code == 200 {
				bodyBytes, _ := io.ReadAll(rsp.Body)
				bodyString := string(bodyBytes)
				re := regexp.MustCompile("--user=(.+)")
				found := re.FindString(bodyString)
				if !strings.Contains(found, tc.expectedUsernameInTemplate) {
					t.Errorf("template should contain --user=%s but found %s", tc.expectedUsernameInTemplate, found)
				}
			}
		})
	}
}

func TestKubeconfigHandler(t *testing.T) {
	tests := map[string]struct {
		cfg                                config.Config
		params                             map[string]string
		usernameClaim                      string
		expectedStatusCode                 int
		expectedAuthInfoName               string
		expectedAuthInfoAuthProviderConfig map[string]string
		expectedContentDisposition         string
	}{
		"default": {
			cfg: config.Config{
				UsernameClaim: "sub",
				ClusterName:   "cluster1",
				APIServerURL:  "https://kubernetes",
				ClientID:      "someClientID",
				ClientSecret:  testClientSecret,
			},
			params: map[string]string{
				"id_token":      testIDToken,
				"refresh_token": "bar",
			},
			expectedStatusCode:   http.StatusOK,
			usernameClaim:        "sub",
			expectedAuthInfoName: "gangway@heptio.com@cluster1",
			expectedAuthInfoAuthProviderConfig: map[string]string{
				"client-id":                      "someClientID",
				"client-secret":                  testClientSecret,
				"id-token":                       testIDToken,
				"refresh-token":                  "bar",
				"idp-issuer-url":                 "GangwayTest",
				"idp-certificate-authority-data": "ZHVtbXkgY2x1c3RlciBJRFAgQ0E=",
			},
			expectedContentDisposition: `Attachment; filename="gangway@heptio.com@cluster1"`,
		},
		"custom filename": {
			cfg: config.Config{
				UsernameClaim: "sub",
				ClusterName:   "cluster1",
				APIServerURL:  "https://kubernetes",
				ClientID:      "someClientID",
				ClientSecret:  testClientSecret,
			},
			params: map[string]string{
				"id_token":      testIDToken,
				"refresh_token": "bar",
				"filename":      "my-custom-cluster",
			},
			expectedStatusCode:   http.StatusOK,
			usernameClaim:        "sub",
			expectedAuthInfoName: "gangway@heptio.com@cluster1",
			expectedAuthInfoAuthProviderConfig: map[string]string{
				"client-id":                      "someClientID",
				"client-secret":                  testClientSecret,
				"id-token":                       testIDToken,
				"refresh-token":                  "bar",
				"idp-issuer-url":                 "GangwayTest",
				"idp-certificate-authority-data": "ZHVtbXkgY2x1c3RlciBJRFAgQ0E=",
			},
			expectedContentDisposition: `Attachment; filename="my-custom-cluster"`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ts := newTestServer(t)
			ts.cfg = &tc.cfg

			// Create dummy cluster CA file
			clusterCAData := "dummy cluster CA"
			f, err := os.CreateTemp(t.TempDir(), "gangplank-kubeconfig-handler-test")
			if err != nil {
				t.Fatalf("Error creating temp file: %v", err)
			}
			fmt.Fprint(f, clusterCAData)

			// Create dummy cluster IDP CA file
			idpCAData := "dummy cluster IDP CA"
			fIdp, err := os.CreateTemp(t.TempDir(), "gangplank-kubeconfig-handler-test-idp")
			if err != nil {
				t.Fatalf("Error creating temp file: %v", err)
			}
			fmt.Fprint(fIdp, idpCAData)

			ts.cfg.ClusterCAPath = f.Name()
			ts.cfg.IDPCAPath = fIdp.Name()

			rsp := NewRecorder()
			req, err := http.NewRequest(http.MethodGet, "/kubeconf", nil)
			if err != nil {
				t.Fatal(err)
			}

			// Create request
			session, err := ts.gangplankUserSession.Session.Get(req, "gangplank")
			if err != nil {
				t.Fatalf("Error getting session: %v", err)
			}
			sessionIDToken, err := ts.gangplankUserSession.Session.Get(req, "gangplank_id_token")
			if err != nil {
				t.Fatalf("Error getting session: %v", err)
			}
			sessionRefreshToken, err := ts.gangplankUserSession.Session.Get(req, "gangplank_refresh_token")
			if err != nil {
				t.Fatalf("Error getting session: %v", err)
			}

			sessionIDToken.Values["id_token"] = tc.params["id_token"]
			sessionRefreshToken.Values["refresh_token"] = tc.params["refresh_token"]
			if err = session.Save(req, rsp); err != nil {
				t.Fatal(err)
			}
			if err = sessionIDToken.Save(req, rsp); err != nil {
				t.Fatal(err)
			}
			if err = sessionRefreshToken.Save(req, rsp); err != nil {
				t.Fatal(err)
			}

			// Add query params to request
			q := req.URL.Query()
			for k, v := range tc.params {
				q.Add(k, v)
			}
			req.URL.RawQuery = q.Encode()

			handler := http.HandlerFunc(ts.kubeConfigHandler)

			// Call Handler
			handler.ServeHTTP(rsp, req)

			// Validate
			if status := rsp.Code; status != tc.expectedStatusCode {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tc.expectedStatusCode)
			}

			gotCD := rsp.Header().Get("Content-Disposition")
			if gotCD != tc.expectedContentDisposition {
				t.Errorf("Content-Disposition = %q; want %q", gotCD, tc.expectedContentDisposition)
			}

			gotCT := rsp.Header().Get("Content-Type")
			if gotCT != "application/yaml" {
				t.Errorf("Content-Type = %q; want %q", gotCT, "application/yaml")
			}

			if rsp.Code != 200 {
				t.Fatalf("expected status 200, got %d", rsp.Code)
			}

			validateKubeconfig(
				t, rsp, ts.cfg, clusterCAData,
				tc.expectedAuthInfoName, tc.expectedAuthInfoAuthProviderConfig,
			)
		})
	}
}

func validateKubeconfig(
	t *testing.T,
	rsp *httptest.ResponseRecorder,
	cfg *config.Config,
	clusterCAData string,
	expectedAuthInfoName string,
	expectedAuthProviderConfig map[string]string,
) {
	t.Helper()

	bodyBytes, readErr := io.ReadAll(rsp.Body)
	if readErr != nil {
		t.Fatalf("error reading body: %v", readErr)
	}
	kubeconfig := &clientcmdapi.Config{}
	if unmarshalErr := yaml.Unmarshal(bodyBytes, kubeconfig); unmarshalErr != nil {
		t.Fatalf("error unmarshaling response: %v", unmarshalErr)
	}

	// Validate cluster
	if len(kubeconfig.Clusters) != 1 {
		t.Fatalf("Found %d clusters in the generated kubeconfig, expected 1", len(kubeconfig.Clusters))
	}
	cluster := kubeconfig.Clusters[0]
	if cluster.Name != cfg.ClusterName {
		t.Errorf("Expected cluster name to be %q, but found %q", cfg.ClusterName, cluster.Name)
	}
	if cluster.Cluster.Server != cfg.APIServerURL {
		t.Errorf("Expected cluster server to be %q, but found %q", cfg.APIServerURL, cluster.Cluster.Server)
	}
	if string(cluster.Cluster.CertificateAuthorityData) != clusterCAData {
		t.Errorf(
			"Expected cluster CA Data %q, but got %q",
			clusterCAData, string(cluster.Cluster.CertificateAuthorityData),
		)
	}

	// Validate AuthInfo
	if len(kubeconfig.AuthInfos) != 1 {
		t.Fatalf("Found %d users in the generated kubeconfig, expected 1", len(kubeconfig.AuthInfos))
	}
	authInfo := kubeconfig.AuthInfos[0]
	if authInfo.Name != expectedAuthInfoName {
		t.Errorf("Expected AuthInfo.Name %q, but got %q", expectedAuthInfoName, authInfo.Name)
	}
	if authInfo.AuthInfo.AuthProvider.Name != "oidc" {
		t.Errorf("expected authprovider to be oidc, got %s", authInfo.AuthInfo.AuthProvider.Name)
	}
	if !reflect.DeepEqual(authInfo.AuthInfo.AuthProvider.Config, expectedAuthProviderConfig) {
		t.Errorf("Expected %v, got %v", expectedAuthProviderConfig, authInfo.AuthInfo.AuthProvider.Config)
	}

	// Validate context
	if len(kubeconfig.Contexts) != 1 {
		t.Fatalf("Found %d contexts in the generated kubeconfig, expected 1", len(kubeconfig.Contexts))
	}
	ctx := kubeconfig.Contexts[0]
	if ctx.Name != cfg.ClusterName {
		t.Errorf("Expected context name to be %q, but found %q", cfg.ClusterName, ctx.Name)
	}
	if ctx.Context.Cluster != cluster.Name {
		t.Errorf("Cluster name %q in context does not match cluster name %q", ctx.Context.Cluster, cluster.Name)
	}
	if ctx.Context.AuthInfo != authInfo.Name {
		t.Errorf("AuthInfo name %q in context does not match user name %q", ctx.Context.AuthInfo, authInfo.Name)
	}
	if kubeconfig.CurrentContext != ctx.Name {
		t.Errorf("Current context %q does not match context name %q", kubeconfig.CurrentContext, ctx.Name)
	}
}

func TestUnauthedCommandlineHandlerRedirect(t *testing.T) {
	ts := newTestServer(t)
	ts.cfg = &config.Config{}

	req, err := http.NewRequest(http.MethodGet, "/commandline", nil)
	if err != nil {
		t.Fatal(err)
	}

	session.New("test", false)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ts.commandlineHandler)

	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusTemporaryRedirect {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

// NewRecorder returns an initialized ResponseRecorder.
func NewRecorder() *httptest.ResponseRecorder {
	return &httptest.ResponseRecorder{
		HeaderMap: make(http.Header),
		Body:      new(bytes.Buffer),
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := map[string]struct {
		input    string
		expected string
	}{
		"clean": {
			input:    "kubeconfig",
			expected: "kubeconfig",
		},
		"with quotes": {
			input:    `file"name`,
			expected: "file_name",
		},
		"with backslash": {
			input:    `file\name`,
			expected: "file_name",
		},
		"with slash": {
			input:    "file/name",
			expected: "file_name",
		},
		"with newlines": {
			input:    "file\r\nname",
			expected: "file__name",
		},
		"header injection attempt": {
			input:    "file\r\nX-Injected: true",
			expected: "file__X-Injected: true",
		},
		"empty": {
			input:    "",
			expected: "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := sanitizeFilename(tc.input)
			if got != tc.expected {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

type FakeToken struct {
	OAuth2Cfg *oauth2.Config
}

// Exchange takes an oauth2 auth token and exchanges for an id_token.
func (f *FakeToken) Exchange(_ context.Context, _ string) (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken:  testIDToken,
		RefreshToken: "4567",
	}, nil
}
