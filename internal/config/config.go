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

package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/kelseyhightower/envconfig"
)

// Config the configuration field for gangplank
type Config struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`

	ClusterName            string   `yaml:"clusterName" envconfig:"cluster_name"`
	AuthorizeURL           string   `yaml:"authorizeURL" envconfig:"authorize_url"`
	TokenURL               string   `yaml:"tokenURL" envconfig:"token_url"`
	ClientID               string   `yaml:"clientID" envconfig:"client_id"`
	ClientSecret           string   `yaml:"clientSecret" envconfig:"client_secret"`
	AllowEmptyClientSecret bool     `yaml:"allowEmptyClientSecret" envconfig:"allow_empty_client_secret"`
	Audience               string   `yaml:"audience" envconfig:"audience"`
	RedirectURL            string   `yaml:"redirectURL" envconfig:"redirect_url"`
	Scopes                 []string `yaml:"scopes" envconfig:"scopes"`
	UsernameClaim          string   `yaml:"usernameClaim" envconfig:"username_claim"`
	EmailClaim             string   `yaml:"emailClaim" envconfig:"email_claim"`
	ServeTLS               bool     `yaml:"serveTLS" envconfig:"serve_tls"`
	CertFile               string   `yaml:"certFile" envconfig:"cert_file"`
	KeyFile                string   `yaml:"keyFile" envconfig:"key_file"`
	APIServerURL           string   `yaml:"apiServerURL" envconfig:"apiserver_url"`
	ClusterCAPath          string   `yaml:"clusterCAPath" envconfig:"cluster_ca_path"`
	IDPCAPath              string   `yaml:"idpCAPath" envconfig:"idp_ca_path"`
	TrustedCAPath          string   `yaml:"trustedCAPath" envconfig:"trusted_ca_path"`
	HTTPPath               string   `yaml:"httpPath" envconfig:"http_path"`

	SessionSecurityKey     string `yaml:"sessionSecurityKey" envconfig:"SESSION_SECURITY_KEY"`
	CustomHTMLTemplatesDir string `yaml:"customHTMLTemplatesDir" envconfig:"custom_http_templates_dir"`

	RemoveCAFromKubeconfig bool   `yaml:"removeCAFromKubeconfig" envconfig:"remove_ca_from_kubeconfig"`
	Namespace              string `yaml:"namespace" envconfig:"namespace"`
}

// NewConfig returns a Config struct from serialized config file
func NewConfig(configFile string) (*Config, error) {

	cfg := &Config{
		Host:                   "0.0.0.0",
		Port:                   8080,
		AllowEmptyClientSecret: false,
		Scopes:                 []string{"openid", "profile", "email", "offline_access"},
		UsernameClaim:          "nickname",
		EmailClaim:             "",
		ServeTLS:               false,
		CertFile:               "/etc/gangplank/tls/tls.crt",
		KeyFile:                "/etc/gangplank/tls/tls.key",
		ClusterCAPath:          "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
		IDPCAPath:              "",
		HTTPPath:               "",
		RemoveCAFromKubeconfig: false,
	}

	if configFile != "" {
		data, err := os.ReadFile(configFile)
		if err != nil {
			return nil, err
		}

		err = yaml.Unmarshal([]byte(data), cfg)
		if err != nil {
			return nil, err
		}
	}

	err := envconfig.Process("gangplank_config", cfg)
	if err != nil {
		return nil, err
	}

	err = cfg.Validate()
	if err != nil {
		return nil, err
	}

	// Check for trailing slash on HTTPPath and remove
	cfg.HTTPPath = strings.TrimRight(cfg.HTTPPath, "/")

	return cfg, nil
}

// Validate verifies all properties of config struct are intialized
func (cfg *Config) Validate() error {
	checks := []struct {
		bad    bool
		errMsg string
	}{
		{cfg.AuthorizeURL == "", "no authorizeURL specified"},
		{cfg.TokenURL == "", "no tokenURL specified"},
		{cfg.ClientID == "", "no clientID specified"},
		{cfg.ClientSecret == "" && !cfg.AllowEmptyClientSecret, "no clientSecret specified"},
		{cfg.RedirectURL == "", "no redirectURL specified"},
		{cfg.SessionSecurityKey == "", "no SessionSecurityKey specified"},
		{cfg.APIServerURL == "", "no apiServerURL specified"},
	}

	for _, check := range checks {
		if check.bad {
			return fmt.Errorf("invalid config: %s", check.errMsg)
		}
	}
	return nil
}

// GetRootPathPrefix returns '/' if no prefix is specified, otherwise returns the configured path
func (cfg *Config) GetRootPathPrefix() string {
	if len(cfg.HTTPPath) == 0 {
		return "/"
	}

	return strings.TrimRight(cfg.HTTPPath, "/")
}
