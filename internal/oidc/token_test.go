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

package oidc

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestParseToken(t *testing.T) {
	base64Decode := func(src string) []byte {
		data, err := base64.RawURLEncoding.DecodeString(src)
		if err != nil {
			t.Fatalf("Error decoding base64 string: %v", err)
		}

		return data
	}

	tests := map[string]struct {
		idToken      string
		clientSecret string
		want         *jwt.Token
		expectError  bool
	}{
		"default": {
			idToken:      "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJHYW5nd2F5VGVzdCIsImlhdCI6MTU0MDA0NjM0NywiZXhwIjoxODg3MjAxNTQ3LCJhdWQiOiJnYW5nd2F5LmhlcHRpby5jb20iLCJzdWIiOiJnYW5nd2F5QGhlcHRpby5jb20iLCJHaXZlbk5hbWUiOiJHYW5nIiwiU3VybmFtZSI6IldheSIsIkVtYWlsIjoiZ2FuZ3dheUBoZXB0aW8uY29tIiwiR3JvdXBzIjoiZGV2LGFkbWluIn0.zNG4Dnxr76J0p4phfsAUYWunioct0krkMiunMynlQsU",
			clientSecret: "qwertyuiopasdfghjklzxcvbnm123456",
			expectError:  false,
			want: &jwt.Token{
				Raw:    "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJHYW5nd2F5VGVzdCIsImlhdCI6MTU0MDA0NjM0NywiZXhwIjoxODg3MjAxNTQ3LCJhdWQiOiJnYW5nd2F5LmhlcHRpby5jb20iLCJzdWIiOiJnYW5nd2F5QGhlcHRpby5jb20iLCJHaXZlbk5hbWUiOiJHYW5nIiwiU3VybmFtZSI6IldheSIsIkVtYWlsIjoiZ2FuZ3dheUBoZXB0aW8uY29tIiwiR3JvdXBzIjoiZGV2LGFkbWluIn0.zNG4Dnxr76J0p4phfsAUYWunioct0krkMiunMynlQsU",
				Method: jwt.SigningMethodHS256,
				Header: map[string]any{
					"typ": "JWT",
					"alg": "HS256",
				},
				Claims: jwt.MapClaims{
					"aud":       "gangway.heptio.com",
					"sub":       "gangway@heptio.com",
					"GivenName": "Gang",
					"Email":     "gangway@heptio.com",
					"Groups":    "dev,admin",
					"iat":       1.540046347e+09,
					"exp":       1.887201547e+09,
					"iss":       "GangwayTest",
					"Surname":   "Way",
				},
				Signature: base64Decode("zNG4Dnxr76J0p4phfsAUYWunioct0krkMiunMynlQsU"),
				Valid:     true,
			},
		},
		"rsa is rejected": {
			idToken:      "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWV9.EkN-DOsnsuRjRO6BxXemmJDm3HbxrbRzXglbN2S4sOkopdU4IsDxTI8jO19W_A4K8ZPJijNLis4EZsHeY559a4DFOd50_OqgHGuERTqYZyuhtF39yxJPAjUESwxk2J5k_4zM3O-vtd1Ghyo4IbqKKSy6J9mTniYJPenn5-HIirE",
			clientSecret: "",
			expectError:  true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := ParseToken(tc.idToken, tc.clientSecret)

			// If we expect an error, check that it's thrown
			if tc.expectError {
				if err == nil {
					t.Fatalf("Error was returned but not expected: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			validateToken(t, got, tc.want)
		})
	}
}

func validateToken(t *testing.T, got, want *jwt.Token) {
	t.Helper()

	if got.Valid != want.Valid {
		t.Fatalf("Valid: want: %v, got: %v", want, got)
	}
	if !bytes.Equal(got.Signature, want.Signature) {
		t.Fatalf("Signature: want: %v, got: %v", want.Signature, got.Signature)
	}
	if got.Raw != want.Raw {
		t.Fatalf("Raw: want: %v, got: %v", want, got)
	}
	if got.Method != want.Method {
		t.Fatalf("Method: want: %v, got: %v", want, got)
	}
	if !eq(got.Header, want.Header) {
		t.Fatalf("Header: want: %v, got: %v", want, got)
	}
	if !eq(got.Claims.(jwt.MapClaims), want.Claims.(jwt.MapClaims)) {
		t.Fatalf("Header: want: %v, got: %v", want, got)
	}
}

func eq(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}

	for k, v := range a {
		if w, ok := b[k]; !ok || v != w {
			return false
		}
	}

	return true
}
