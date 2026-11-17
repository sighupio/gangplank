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
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestHomeHandler(t *testing.T) {
	t.Parallel()

	t.Run("default embedded template", func(t *testing.T) {
		t.Parallel()

		ts := newTestServer(t)
		ts.cfg = &config.Config{}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()

		ts.homeHandler(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}
	})

	t.Run("custom template", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		homeTmpl := "{{ .ClusterName }} {{ .HTTPPath }}"
		if err := os.WriteFile(filepath.Join(tempDir, "home.tmpl"), []byte(homeTmpl), 0644); err != nil {
			t.Fatal(err)
		}

		ts := newTestServer(t)
		ts.cfg = &config.Config{
			ClusterName:            "cluster1",
			HTTPPath:               "/foo",
			CustomHTMLTemplatesDir: tempDir,
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()

		ts.homeHandler(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}
		if want := "cluster1 /foo"; rr.Body.String() != want {
			t.Errorf("expected body to be %q, got %q", want, rr.Body.String())
		}
	})
}

func TestStaticHandler(t *testing.T) {
	t.Parallel()

	t.Run("default embedded static", func(t *testing.T) {
		t.Parallel()

		ts := newTestServer(t)
		ts.cfg = &config.Config{}

		req := httptest.NewRequest(http.MethodGet, "/style.css", nil)
		rr := httptest.NewRecorder()

		ts.staticHandler().ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}
		if !strings.Contains(rr.Body.String(), "euiHeader") {
			t.Errorf("expected body to contain euiHeader style, got %s", rr.Body.String())
		}
	})

	t.Run("custom static directory", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		customCSS := "/* custom stylesheet */ body { background-color: #000; }"
		if err := os.WriteFile(filepath.Join(tempDir, "style.css"), []byte(customCSS), 0644); err != nil {
			t.Fatal(err)
		}

		ts := newTestServer(t)
		ts.cfg = &config.Config{
			CustomStaticDir: tempDir,
		}

		req := httptest.NewRequest(http.MethodGet, "/style.css", nil)
		rr := httptest.NewRecorder()

		ts.staticHandler().ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}
		if rr.Body.String() != customCSS {
			t.Errorf("expected body to be %q, got %q", customCSS, rr.Body.String())
		}
	})
}

func TestCallbackHandler(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			ts := newTestServer(t)
			ts.cfg = &config.Config{
				HTTPPath: "/foo",
			}

			rsp := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/callback", nil)

			// Create request
			sess, err := ts.gangplankUserSession.Session.Get(req, "gangplank")
			if err != nil {
				t.Fatalf("Error getting session: %v", err)
			}

			// Create state session variable
			sess.Values["state"] = tc.params["state"]
			if err = sess.Save(req, rsp); err != nil {
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
	t.Parallel()

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
			t.Parallel()

			ts := newTestServer(t)
			ts.cfg = &config.Config{
				HTTPPath:      "/foo",
				EmailClaim:    tc.emailClaim,
				UsernameClaim: tc.usernameClaim,
				ClusterName:   "cluster1",
				APIServerURL:  "https://kubernetes",
				ClientSecret:  testClientSecret,
			}

			rsp, req := newRequestWithSessions(t, ts, "/callback", tc.params)

			handler := http.HandlerFunc(ts.commandlineHandler)

			// Call Handler
			handler.ServeHTTP(rsp, req)

			// Validate!
			if status := rsp.Code; status != tc.expectedStatusCode {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tc.expectedStatusCode)
			}
			// if response code is OK then check that username is correct in resultant template
			if rsp.Code == 200 {
				assertUsernameInTemplate(t, rsp, tc.expectedUsernameInTemplate)
			}
		})
	}
}

func TestKubeconfigHandler(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			ts := newTestServer(t)
			ts.cfg = &tc.cfg

			// Create dummy cluster CA and cluster IDP CA files
			clusterCAData := "dummy cluster CA"
			ts.cfg.ClusterCAPath = writeTempFile(t, "gangplank-kubeconfig-handler-test", clusterCAData)
			ts.cfg.IDPCAPath = writeTempFile(t, "gangplank-kubeconfig-handler-test-idp", "dummy cluster IDP CA")

			rsp, req := newRequestWithSessions(t, ts, "/kubeconf", tc.params)

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

func TestUnauthedCommandlineHandlerRedirect(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)
	ts.cfg = &config.Config{}

	req := httptest.NewRequest(http.MethodGet, "/commandline", nil)

	session.New("test", false)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ts.commandlineHandler)

	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusTemporaryRedirect {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestSanitizeFilename(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			got := sanitizeFilename(tc.input)
			if got != tc.expected {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

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

// newRequestWithSessions builds a GET request for target with params as query parameters, and stores the
// state, id_token and refresh_token params (when present) in the gangplank sessions.
func newRequestWithSessions(
	t *testing.T,
	ts *server,
	target string,
	params map[string]string,
) (*httptest.ResponseRecorder, *http.Request) {
	t.Helper()

	rsp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)

	sessionValues := []struct {
		name string
		key  string
	}{
		{name: "gangplank", key: "state"},
		{name: "gangplank_id_token", key: "id_token"},
		{name: "gangplank_refresh_token", key: "refresh_token"},
	}
	for _, sv := range sessionValues {
		sess, getErr := ts.gangplankUserSession.Session.Get(req, sv.name)
		if getErr != nil {
			t.Fatalf("Error getting session: %v", getErr)
		}
		if value, ok := params[sv.key]; ok {
			sess.Values[sv.key] = value
		}
		if saveErr := sess.Save(req, rsp); saveErr != nil {
			t.Fatal(saveErr)
		}
	}

	q := req.URL.Query()
	for k, v := range params {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	return rsp, req
}

func writeTempFile(t *testing.T, name, data string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("Error creating temp file: %v", err)
	}

	return path
}

func assertUsernameInTemplate(t *testing.T, rsp *httptest.ResponseRecorder, expectedUsername string) {
	t.Helper()

	bodyBytes, err := io.ReadAll(rsp.Body)
	if err != nil {
		t.Fatalf("error reading body: %v", err)
	}
	found := regexp.MustCompile("--user=(.+)").FindString(string(bodyBytes))
	if !strings.Contains(found, expectedUsername) {
		t.Errorf("template should contain --user=%s but found %s", expectedUsername, found)
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
	if !maps.Equal(authInfo.AuthInfo.AuthProvider.Config, expectedAuthProviderConfig) {
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
