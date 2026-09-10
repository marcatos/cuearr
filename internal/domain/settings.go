package domain

type AuthSettings struct {
	PasswordHash string `json:"password_hash,omitempty"`
	APIKey       string `json:"api_key,omitempty"`
}

type Settings struct {
	WatchDirs []string
	OutDir    string
	InPlace   bool
	Engine    string
	Auth      AuthSettings
}
