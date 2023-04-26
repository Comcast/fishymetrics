package common

import (
	"context"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

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

func Fetch(uri, metricType, host string, client *retryablehttp.Client) func() ([]byte, string, error) {
	var resp *http.Response
	var credential *Credential
	var err error
	retryCount := 0

	return func() ([]byte, string, error) {
		// Add a 100 milliseconds delay in between requests because cisco devices respond in a non idiomatic manner
		time.Sleep(100 * time.Millisecond)
		req := BuildRequest(uri, host)
		resp, err = DoRequest(client, req)
		if err != nil {
			return nil, metricType, err
		}
		defer resp.Body.Close()
		if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
			if resp.StatusCode == http.StatusNotFound {
				for retryCount < 3 && resp.StatusCode == http.StatusNotFound {
					time.Sleep(client.RetryWaitMin)
					resp, err = DoRequest(client, req)
					retryCount = retryCount + 1
				}
				if err != nil {
					return nil, metricType, err
				} else if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
					return nil, metricType, fmt.Errorf("HTTP status %d", resp.StatusCode)
				}
			} else if resp.StatusCode == http.StatusUnauthorized {
				// Credentials may have rotated, go to vault and get the latest
				credential, err = ChassisCreds.GetCredentials(context.Background(), host)
				if err != nil {
					return nil, metricType, fmt.Errorf("issue retrieving credentials from vault using target: %s", host)
				}
				ChassisCreds.Set(host, credential)

				// build new request with updated credentials
				req = BuildRequest(uri, host)

				time.Sleep(client.RetryWaitMin)
				resp, err = DoRequest(client, req)
				if err != nil {
					return nil, metricType, fmt.Errorf("Retry DoRequest failed - " + err.Error())
				}
				if resp.StatusCode == http.StatusUnauthorized {
					return nil, metricType, fmt.Errorf("HTTP status %d", resp.StatusCode)
				}
			} else {
				return nil, metricType, fmt.Errorf("HTTP status %d", resp.StatusCode)
			}
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, metricType, fmt.Errorf("Error reading Response Body - " + err.Error())
		}
		return body, metricType, nil
	}
}

func FetchXML(uri, classId, target string, client *retryablehttp.Client) func() ([]byte, string, error) {
	var body []byte
	return func() ([]byte, string, error) {
		cookie, err := IMCLogin(uri, target, client)
		if err != nil {
			return body, classId, err
		}
		defer IMCLogout(uri, cookie, client)

		body, err := IMCPost(uri, classId, cookie, client)
		if err != nil {
			return body, classId, err
		}

		return body, classId, nil
	}
}

func IMCPost(uri, classId, cookie string, client *retryablehttp.Client) ([]byte, error) {
	req := BuildIMCRequest(uri, classId, cookie)

	resp, err := DoRequest(client, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading Response Body - " + err.Error())
	}

	return body, nil
}

func IMCLogin(uri, target string, client *retryablehttp.Client) (string, error) {
	req := BuildIMCLogin(uri, target)
	var aaaLogin AaaLogin

	resp, err := DoRequest(client, req)
	if err != nil {
		return aaaLogin.OutCookie, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
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

	resp, err := DoRequest(client, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

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

	if c, ok := ChassisCreds.Get(host); ok {
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

func BuildRequest(uri, host string) *retryablehttp.Request {
	var user, password string

	if c, ok := ChassisCreds.Get(host); ok {
		credential := c
		user = credential.User
		password = credential.Pass
	} else {
		// use statically configured credentials
		user = config.GetConfig().User
		password = config.GetConfig().Pass
	}

	req, _ := retryablehttp.NewRequest(http.MethodGet, uri, nil)
	req.SetBasicAuth(user, password)

	return req
}

func DoRequest(client *retryablehttp.Client, req *retryablehttp.Request) (*http.Response, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
