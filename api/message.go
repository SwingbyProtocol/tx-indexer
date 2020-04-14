package api

type MsgWsRequest struct {
	Action string      `json:"action"`
	ReqID  string      `json:"reqid"`
	Params interface{} `json:"params"`
}

type MsgWSResponse struct {
	Action  string      `json:"action"`
	ReqID   string      `json:"reqid"`
	Result  bool        `json:"result"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type Params struct {
	Address   string `json:"address"`
	Txid      string `json:"txid"`
	Hex       string `json:"hex"`
	Type      string `json:"type"`
	Mempool   bool   `json:"mempool"`
	NoDetails bool   `json:"nodetails"`
	TimeFrom  int64  `json:"time_from"`
	TimeTo    int64  `json:"time_to"`
}

type Response struct {
	Action  string      `json:"action"`
	Result  bool        `json:"result"`
	Height  int64       `json:"height"`
	Message string      `json:"message"`
	Txs     interface{} `json:"txs"`
}

func NewSuccessResponse(action string, message string, data interface{}) MsgWSResponse {
	msg := MsgWSResponse{
		Action:  action,
		Result:  true,
		Message: message,
		Data:    data,
	}
	return msg
}

func NewErrorResponse(action string, message string) MsgWSResponse {
	msg := MsgWSResponse{
		Action:  action,
		Result:  false,
		Message: message,
		Data:    nil,
	}
	return msg
}
