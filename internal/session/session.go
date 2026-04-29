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

package session

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"fmt"
	"net/http"
)

const (
	salt             = "MkmfuPNHnZBBivy0L0aW"
	pbkdf2Iterations = 4096
	pbkdf2KeyLength  = 96
)

// Session defines a Gangplank session.
type Session struct {
	Session *CustomCookieStore
}

// New inits a Session with CookieStore.
func New(sessionSecurityKey string, secureCookies bool) (*Session, error) {
	signingKey, encryptionKey, err := generateSessionKeys(sessionSecurityKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate session keys: %w", err)
	}

	store, err := NewCustomCookieStore(secureCookies, signingKey, encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie store: %w", err)
	}

	return &Session{
		Session: store,
	}, nil
}

// generateSessionKeys creates a signed encryption key for the cookie store.
func generateSessionKeys(sessionSecurityKey string) ([]byte, []byte, error) {
	// Take the configured security key and generate 96 bytes of data. This is
	// used as the signing and encryption keys for the cookie store.  For details
	// on the PBKDF2 function: https://en.wikipedia.org/wiki/PBKDF2
	derivedKey, err := pbkdf2.Key(
		sha256.New,
		sessionSecurityKey,
		[]byte(salt),
		pbkdf2Iterations, pbkdf2KeyLength)
	if err != nil {
		return nil, nil, err
	}

	return derivedKey[:64], derivedKey[64:], nil
}

// Cleanup removes a single session from the store.
func (s *Session) Cleanup(w http.ResponseWriter, r *http.Request, name string) error {
	session, err := s.Session.Get(r, name)
	if err != nil {
		return fmt.Errorf("failed to get session %q: %w", name, err)
	}
	session.Options.MaxAge = -1
	if saveErr := session.Save(r, w); saveErr != nil {
		return fmt.Errorf("failed to save session %q: %w", name, saveErr)
	}
	return nil
}

// CleanupAll removes multiple sessions from the store.
func (s *Session) CleanupAll(w http.ResponseWriter, r *http.Request, names ...string) error {
	for _, name := range names {
		if err := s.Cleanup(w, r, name); err != nil {
			return err
		}
	}
	return nil
}
