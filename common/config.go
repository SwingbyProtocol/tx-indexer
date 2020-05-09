package common

type ConfigParams struct {
	AccessToken string `json:"accessToken"`
	TargetToken string `json:"targetToken"`
	Address     string `json:"address"`
	IsRescan    bool   `json:"rescan"`
	Timestamp   int64  `json:"timestamp"`
}
