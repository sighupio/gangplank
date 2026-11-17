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

package session_test

import (
	"net/http"
	"testing"

	"github.com/sighupio/gangplank/internal/session"
)

func TestNewCustomCookieStoreOptionsHTTPS(t *testing.T) {
	t.Parallel()

	s, err := session.NewCustomCookieStore(true, []byte("test-signing-key-32bytes!!"), []byte("test-encryption-key"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertCookieOptions(t, s, true, http.SameSiteLaxMode)
}

func TestNewCustomCookieStoreOptionsHTTP(t *testing.T) {
	t.Parallel()

	s, err := session.NewCustomCookieStore(false, []byte("test-signing-key-32bytes!!"), []byte("test-encryption-key"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertCookieOptions(t, s, false, http.SameSiteLaxMode)
}

func assertCookieOptions(t *testing.T, s *session.CustomCookieStore, wantSecure bool, wantSameSite http.SameSite) {
	t.Helper()
	if s.Options == nil {
		t.Fatal("Options is nil")
	}
	if s.Options.Secure != wantSecure {
		t.Errorf("Secure = %v, want %v", s.Options.Secure, wantSecure)
	}
	if s.Options.SameSite != wantSameSite {
		t.Errorf("SameSite = %v, want %v", s.Options.SameSite, wantSameSite)
	}
}
