/*
 * Copyright 2024 Comcast Cable Communications Management, LLC
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

package nuova

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/comcast/fishymetrics/common"
	"github.com/comcast/fishymetrics/config"
	"github.com/hashicorp/go-retryablehttp"
)

type GenericMetricPayload struct {
	XMLName        xml.Name `xml:"configResolveClass"`
	Cookie         string   `xml:"cookie,attr"`
	InHierarchical string   `xml:"inHierarchical,attr"`
	ClassId        string   `xml:"classId,attr"`
}

type AaaLogin struct {
	XMLName          xml.Name `xml:"aaaLogin"`
	Cookie           string   `xml:"cookie,attr"`
	Response         string   `xml:"response,attr"`
	OutCookie        string   `xml:"outCookie,attr"`
	OutRefreshPeriod string   `xml:"outRefreshPeriod,attr"`
	OutPriv          string   `xml:"outPriv,attr"`
	OutSessionId     string   `xml:"outSessionId,attr"`
	OutVersion       string   `xml:"outVersion,attr"`
	ErrorCode        string   `xml:"errorCode,attr,omitempty"`
	ErrorDescr       string   `xml:"errorDescr,attr,omitempty"`
}

type AaaLoginPayload struct {
	XMLName    xml.Name `xml:"aaaLogin"`
	InName     string   `xml:"inName,attr"`
	InPassword string   `xml:"inPassword,attr"`
}

type AaaLogout struct {
	XMLName    xml.Name `xml:"aaaLogout"`
	Cookie     string   `xml:"cookie,attr"`
	Response   string   `xml:"response,attr"`
	OutStatus  string   `xml:"outStatus,attr"`
	ErrorCode  string   `xml:"errorCode,attr,omitempty"`
	ErrorDescr string   `xml:"errorDescr,attr,omitempty"`
}

type AaaLogoutPayload struct {
	XMLName  xml.Name `xml:"aaaLogout"`
	Cookie   string   `xml:"cookie,attr"`
	InCookie string   `xml:"inCookie,attr"`
}

func checkRaidController(url, host string, client *retryablehttp.Client) (bool, error) {
	var resp *http.Response
	var err error
	retryCount := 0
	req := common.BuildRequest(url, host)

	resp, err = common.DoRequest(client, req)
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()
	if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
		if resp.StatusCode == http.StatusNotFound {
			for retryCount < 3 && resp.StatusCode == http.StatusNotFound {
				time.Sleep(client.RetryWaitMin)
				resp, err = common.DoRequest(client, req)
				if err != nil {
					return false, nil
				}
				defer common.EmptyAndCloseBody(resp)
				retryCount = retryCount + 1
			}
			if err != nil {
				return false, nil
			} else if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
				return false, nil
			}
		} else {
			return false, nil
		}
	}

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return true, fmt.Errorf("Error reading Response Body - " + err.Error())
	}

	return true, nil
}

func FetchXML(uri, classId, target string, client *retryablehttp.Client) func() ([]byte, error) {
	var body []byte
	return func() ([]byte, error) {
		cookie, err := IMCLogin(uri, target, client)
		if err != nil {
			return body, err
		}
		defer IMCLogout(uri, cookie, client)

		body, err := IMCPost(uri, classId, cookie, client)
		if err != nil {
			return body, err
		}

		return body, nil
	}
}

func IMCPost(uri, classId, cookie string, client *retryablehttp.Client) ([]byte, error) {
	req := BuildIMCRequest(uri, classId, cookie)

	resp, err := common.DoRequest(client, req)
	if err != nil {
		return nil, err
	}
	defer common.EmptyAndCloseBody(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading Response Body - " + err.Error())
	}

	return body, nil
}

func IMCLogin(uri, target string, client *retryablehttp.Client) (string, error) {
	req := BuildIMCLogin(uri, target)
	var aaaLogin AaaLogin

	resp, err := common.DoRequest(client, req)
	if err != nil {
		return aaaLogin.OutCookie, err
	}
	defer common.EmptyAndCloseBody(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return aaaLogin.OutCookie, fmt.Errorf("Error reading Response Body - " + err.Error())
	}

	err = xml.Unmarshal(body, &aaaLogin)
	if err != nil {
		return aaaLogin.OutCookie, fmt.Errorf("error unmarshalling UCS chassis aaaLogin struct - " + err.Error())
	}

	if aaaLogin.ErrorCode != "" {
		return aaaLogin.OutCookie, fmt.Errorf("error failed to login to UCS chassis using IMC xmlapi - errorCode: %s, errorDescr: %s", aaaLogin.ErrorCode, aaaLogin.ErrorDescr)
	}

	return aaaLogin.OutCookie, nil
}

func IMCLogout(uri, cookie string, client *retryablehttp.Client) error {
	req := BuildIMCLogout(uri, cookie)

	resp, err := common.DoRequest(client, req)
	if err != nil {
		return err
	}
	defer common.EmptyAndCloseBody(resp)

	return nil
}

func BuildIMCRequest(uri, classId, cookie string) *retryablehttp.Request {

	payload := &GenericMetricPayload{
		Cookie:         cookie,
		InHierarchical: "false",
		ClassId:        classId,
	}

	bytes, _ := xml.Marshal(payload)

	req, _ := retryablehttp.NewRequest(http.MethodPost, uri, bytes)

	return req
}

func BuildIMCLogin(uri, host string) *retryablehttp.Request {
	var user, password string

	if c, ok := common.ChassisCreds.Get(host); ok {
		credential := c
		user = credential.User
		password = credential.Pass
	} else {
		// use statically configured credentials
		user = config.GetConfig().User
		password = config.GetConfig().Pass
	}

	payload := &AaaLoginPayload{
		InName:     user,
		InPassword: password,
	}

	bytes, _ := xml.Marshal(payload)

	req, _ := retryablehttp.NewRequest(http.MethodPost, uri, bytes)

	return req
}

func BuildIMCLogout(uri, cookie string) *retryablehttp.Request {

	payload := &AaaLogoutPayload{
		Cookie:   cookie,
		InCookie: cookie,
	}

	bytes, _ := xml.Marshal(payload)

	req, _ := retryablehttp.NewRequest(http.MethodPost, uri, bytes)

	return req
}
