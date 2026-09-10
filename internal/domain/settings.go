package domain

const DefaultMaxRetries = 3

type AuthSettings struct {
	PasswordHash            string   `json:"password_hash,omitempty"`
	APIKey                  string   `json:"api_key,omitempty"`
	OIDCEnabled             bool     `json:"oidc_enabled,omitempty"`
	OIDCIssuer              string   `json:"oidc_issuer,omitempty"`
	OIDCClientID            string   `json:"oidc_client_id,omitempty"`
	OIDCClientSecret        string   `json:"oidc_client_secret,omitempty"`
	OIDCRedirectURL         string   `json:"oidc_redirect_url,omitempty"`
	OIDCAllowedEmailDomains []string `json:"oidc_allowed_email_domains,omitempty"`
}

type Settings struct {
	WatchDirs []string
	OutDir    string
	InPlace   bool
	Engine    string
	// MaxRetries is the maximum total number of attempts, including the first.
	// Values less than one use DefaultMaxRetries.
	MaxRetries int
	Auth       AuthSettings
}

func (s Settings) MaxAttempts() int {
	if s.MaxRetries < 1 {
		return DefaultMaxRetries
	}
	return s.MaxRetries
}
