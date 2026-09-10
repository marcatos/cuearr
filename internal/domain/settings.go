package domain

type AuthSettings struct{}

type Settings struct {
	WatchDirs []string
	OutDir    string
	InPlace   bool
	Engine    string
	Auth      AuthSettings
}
