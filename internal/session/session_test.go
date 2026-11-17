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
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"

	"github.com/sighupio/gangplank/internal/session"
)

func TestInitSessionStore(t *testing.T) {
	t.Parallel()

	s, err := session.New("testing", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Session == nil {
		t.Errorf("Session Store is nil. Did not get initialized")
		return
	}
}

func TestCleanupSession(t *testing.T) {
	t.Parallel()

	s, err := session.New("testing", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sess := &sessions.Session{}
	// create a test http server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, _ = s.Session.Get(r, "gangplank")
		s.Cleanup(w, r, "gangplank")
	}))
	defer ts.Close()
	_, err = http.Get(ts.URL)
	if err != nil {
		t.Fatalf("Error getting from test server: %v", err)
	}
	if sess.Options.MaxAge != -1 {
		t.Errorf("Session was not reset. Have max age of %d. Should have -1", sess.Options.MaxAge)
	}
}
