/*
 * Copyright 2023 Comcast Cable Communications Management, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package common

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/comcast/fishymetrics/config"
	"go.uber.org/zap"
)

var (
	IgnoredDevices = make(map[string]IgnoredDevice)
)

type host struct {
	H string `json:"host"`
}

type IgnoredDevice struct {
	Name              string
	Endpoint          string
	Model             string
	CredentialProfile string
}

func TestConn(w http.ResponseWriter, r *http.Request) {
	var h host
	var path string
	response := make(map[string]interface{})
	response["connectionTest"] = false

	log = zap.L()

	body, err := getBody(r)
	if err != nil {
		response["error"] = err.Error()
		resp, _ := marshalResponse(&response, r)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(resp)
		return
	}

	err = unmarshalBody(body, &h, r)
	if err != nil {
		log.Error("failed to unmarshal body from frontend", zap.Error(err), zap.String("path", r.URL.Path))
		response["error"] = err.Error()
		resp, _ := marshalResponse(&response, r)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(resp)
		return
	}

	if _, ok := IgnoredDevices[h.H]; !ok {
		log.Error("missing host from ignored hosts list", zap.Error(err), zap.String("path", r.URL.Path))
		response["error"] = "missing host from ignored hosts list"
		resp, _ := marshalResponse(&response, r)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(resp)
		return
	}
	path = IgnoredDevices[h.H].Endpoint
	credProfile := IgnoredDevices[h.H].CredentialProfile
	// get credentials from vault
	credential, err := ChassisCreds.GetCredentials(context.Background(), credProfile, h.H)
	if err != nil {
		log.Error("issue retrieving credentials from vault using target "+h.H, zap.Error(err), zap.String("path", r.URL.Path))
		response["error"] = err.Error()
		resp, _ := marshalResponse(&response, r)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(resp)
		return
	}

	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		log.Error("failed to build test connection request", zap.Error(err), zap.String("path", r.URL.Path))
		response["error"] = err.Error()
		resp, _ := marshalResponse(&response, r)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(resp)
		return
	}
	req.SetBasicAuth(credential.User, credential.Pass)

	tr := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 3 * time.Second,
		}).Dial,
		MaxIdleConns:          1,
		MaxConnsPerHost:       1,
		MaxIdleConnsPerHost:   1,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.GetConfig().SSLVerify,
		},
		TLSHandshakeTimeout: 10 * time.Second,
	}

	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: tr,
	}

	res, err := client.Do(req)
	if err != nil {
		log.Error("request failed for test connection call", zap.Error(err), zap.String("path", r.URL.Path))
		response["error"] = err.Error()
		resp, _ := marshalResponse(&response, r)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(resp)
		return
	}

	if res.StatusCode != 401 {
		response["connectionTest"] = true
	} else {
		response["error"] = res.Status
	}

	resp, _ := marshalResponse(&response, r)
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

func RemoveHost(w http.ResponseWriter, r *http.Request) {
	var h host

	log = zap.L()

	body, err := getBody(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	err = unmarshalBody(body, &h, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	delete(IgnoredDevices, h.H)
	log.Info("remove host " + h.H + " from ignored list")
	w.WriteHeader(http.StatusOK)
}

func getBody(r *http.Request) ([]byte, error) {
	var body []byte
	log = zap.L()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("could not parse request body", zap.Error(err), zap.String("path", r.URL.Path))
		return body, err
	}
	return body, nil
}

func unmarshalBody(b []byte, h *host, r *http.Request) error {
	var err error
	log = zap.L()

	err = json.Unmarshal(b, h)
	if err != nil {
		log.Error("could not unmarshal host struct", zap.Error(err), zap.String("path", r.URL.Path))
		return err
	}
	return nil
}

func marshalResponse(p *map[string]interface{}, r *http.Request) ([]byte, error) {
	var resp []byte
	log = zap.L()

	resp, err := json.Marshal(p)
	if err != nil {
		log.Error("could not marshal response", zap.Error(err), zap.String("path", r.URL.Path))
		return resp, err
	}
	return resp, nil
}
