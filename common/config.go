package common

type ConfigParams struct {
	AccessToken string `json:"accessToken"`
	Address     string `json:"address"`
	IsRescan    bool   `json:"rescan"`
	Timestamp   int64  `json:"timestamp"`
}

type ConfigResponse struct {
	Result bool   `json:"result"`
	Msg    string `json:"msg,omitempty"`
}
