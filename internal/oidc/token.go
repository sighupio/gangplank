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
	"context"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

// OAuth2Token is an interface used when exchanging an id_token for an access token.
type OAuth2Token interface {
	Exchange(ctx context.Context, code string) (*oauth2.Token, error)
}

// Token is an implementation of OAuth2Token Interface.
type Token struct {
	OAuth2Cfg *oauth2.Config
}

// ParseToken returns a jwt token from an idToken by parsing it without signature verification.
// Signature verification is handled by the OIDC provider, this function only extracts claims.
func ParseToken(idToken, _ string) (*jwt.Token, error) {
	parser := jwt.NewParser()

	token, _, err := parser.ParseUnverified(idToken, jwt.MapClaims{})
	if err != nil {
		return nil, err
	}

	return token, nil
}

// Exchange takes an oauth2 auth token and exchanges for an id_token.
func (t *Token) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return t.OAuth2Cfg.Exchange(ctx, code)
}
