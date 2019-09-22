package resolver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

type Resolver struct {
	Client         *http.Client
	ContextTimeout time.Duration
}

func NewResolver() *Resolver {
	client := &http.Client{Transport: &http.Transport{
		MaxIdleConnsPerHost: 100,
	}}
	client.Timeout = 2 * time.Second
	resolver := &Resolver{
		Client:         client,
		ContextTimeout: 2 * time.Second,
	}
	return resolver
}

func (r *Resolver) SetTimeout(time time.Duration) {
	r.Client.Timeout = time
	r.ContextTimeout = time
}

func (r *Resolver) GetRequest(uri string, query string, res interface{}) error {
	req, err := http.NewRequest(
		"GET",
		uri+query,
		nil,
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	ctx, cancel := context.WithTimeout(context.Background(), r.ContextTimeout)
	defer cancel()

	reqWithDeadline := req.WithContext(ctx)
	resp, err := r.Client.Do(reqWithDeadline)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		code := strconv.Itoa(resp.StatusCode)
		log.Info(" -> ", code, " ", query)
		return errors.New(code)
	}
	decoder := json.NewDecoder(resp.Body)
	decoder.Decode(res)
	resp.Body.Close()
	return nil
}

func (r *Resolver) PostRequest(uri string, jsonBody string, res interface{}) error {
	req, err := http.NewRequest(
		"POST",
		uri,
		bytes.NewBuffer([]byte(jsonBody)),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	reqWithDeadline := req.WithContext(ctx)
	resp, err := r.Client.Do(reqWithDeadline)
	if err != nil {
		log.Println("post err:", err)
		return err
	}
	decoder := json.NewDecoder(resp.Body)
	decoder.Decode(res)
	resp.Body.Close()
	return nil
}
