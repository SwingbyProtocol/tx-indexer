package api

import (
	"net/http"

	"github.com/ant0ine/go-json-rest/rest"
	log "github.com/sirupsen/logrus"
)

type Rest struct {
	api     *rest.Api
	listen  string
	actions []*Action
}

func NewREST(conf *APIConfig) *Rest {
	re := &Rest{
		api:     rest.NewApi(),
		listen:  conf.ListenREST,
		actions: conf.Actions,
	}
	re.api.Use(&rest.CorsMiddleware{
		OriginValidator: func(origin string, request *rest.Request) bool {
			return true
		},
	})
	return re
}

func (re *Rest) Start() {
	routes := []*rest.Route{}
	for _, action := range re.actions {
		switch action.method {
		case "REST:GET":
			routes = append(routes, rest.Get(action.key, action.HandlerREST))
			break
		case "REST:POST":
			routes = append(routes, rest.Post(action.key, action.HandlerREST))
		default:
			break
		}
	}
	// Default
	routes = append(routes, rest.Get("/", onKeep))
	restRouter, err := rest.MakeRouter(routes...)
	if err != nil {
		log.Fatal(err)
	}
	re.api.SetApp(restRouter)
	go func() {
		err := http.ListenAndServe(re.listen, re.api.MakeHandler())
		if err != nil {
			log.Fatal(err)
		}
	}()
	log.Infof("REST api listen: %s", re.listen)
}

func onKeep(w rest.ResponseWriter, r *rest.Request) {
	w.WriteHeader(http.StatusOK)
	w.WriteJson("status OK")
	return
}
