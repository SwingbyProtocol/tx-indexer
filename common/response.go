package common

type Response struct {
	Result bool   `json:"result"`
	Msg    string `json:"msg,omitempty"`
}
