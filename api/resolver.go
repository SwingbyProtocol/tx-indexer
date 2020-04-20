package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	httpReqTimeout = 300 * time.Second
)

type Resolver struct {
	URI            string
	Client         *http.Client
	ContextTimeout time.Duration
}

func NewResolver(uri string, conns int) *Resolver {
	client := &http.Client{Transport: &http.Transport{
		MaxIdleConnsPerHost: conns,
	}}
	client.Timeout = 4 * time.Second
	resolver := &Resolver{
		URI:            uri,
		Client:         client,
		ContextTimeout: 4 * time.Second,
	}
	return resolver
}

func (r *Resolver) SetTimeout(time time.Duration) {
	r.Client.Timeout = time
	r.ContextTimeout = time
}

func (r *Resolver) GetRequest(query string, res interface{}) error {
	req, err := http.NewRequest(
		"GET",
		r.URI+query,
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
		return errors.New(" -> " + code + " " + query)
	}
	decoder := json.NewDecoder(resp.Body)
	decoder.Decode(res)
	resp.Body.Close()
	return nil
}

func (r *Resolver) PostRequest(query string, jsonBody string, res interface{}) error {
	req, err := http.NewRequest(
		"POST",
		r.URI+query,
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

func Post(baseUri, endpoint string, query map[string]string, body io.Reader) ([]byte, error) {
	return request(baseUri, "POST", endpoint, query, body)
}

func Get(baseUri, endpoint string, query map[string]string) ([]byte, error) {
	return request(baseUri, "GET", endpoint, query, nil)
}

func request(baseUri, method, endpoint string, query map[string]string, body io.Reader) ([]byte, error) {
	ctx, _ := context.WithTimeout(context.Background(), httpReqTimeout)
	baseUri = strings.TrimSuffix(baseUri, "/")
	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s%s", baseUri, endpoint), body)
	if err != nil {
		return nil, err
	}
	if method == "POST" {
		req.Header.Set("Content-Type", "application/json")
	}
	// create query string
	q := req.URL.Query()
	for k, v := range query {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()
	client := &http.Client{}
	// send get request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 && resp.StatusCode > 299 {
		return nil, fmt.Errorf("expected a status code of 2xx but got %d", resp.StatusCode)
	}
	// read response
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return respBody, nil
}
