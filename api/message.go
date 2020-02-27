package api

type Request struct {
	Action string      `json:"action"`
	ReqID  string      `json:"reqid"`
	Params interface{} `json:"params"`
}

type Response struct {
	Action  string      `json:"action"`
	ReqID   string      `json:"reqid"`
	Result  bool        `json:"result"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func NewSuccessResponse(action string, message string, data interface{}) Response {
	msg := Response{
		Action:  action,
		Result:  true,
		Message: message,
		Data:    data,
	}
	return msg
}

func NewErrorResponse(action string, message string) Response {
	msg := Response{
		Action:  action,
		Result:  false,
		Message: message,
		Data:    nil,
	}
	return msg
}
