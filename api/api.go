package api

type API struct {
	Rest *Rest
	Ws   *Websocket
}

type APIConfig struct {
	ListenREST string
	ListenWS   string
	Actions    []*Action
}

func (apiConf *APIConfig) AddAction(action *Action) {
	apiConf.Actions = append(apiConf.Actions, action)
}

func NewAPI(conf *APIConfig) *API {
	api := &API{
		Rest: NewREST(conf),
		Ws:   NewWebsocket(conf),
	}
	return api
}

func (api *API) Start() {
	api.Rest.Start()
	api.Ws.Start()
}
