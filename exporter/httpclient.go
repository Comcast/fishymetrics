package exporter

import (
    "context"
    "crypto/tls"
    "net"
    "net/http"
    "net/url"
    "time"

    "github.com/comcast/fishymetrics/config"
    "github.com/hashicorp/go-retryablehttp"
)

// NewHTTPClient builds a retryablehttp client honoring proxy from context override,
// otherwise falling back to standard HTTP(S)_PROXY/NO_PROXY environment variables.
func NewHTTPClient(ctx context.Context) *retryablehttp.Client {
    tr := &http.Transport{
        Dial: (&net.Dialer{Timeout: 3 * time.Second}).Dial,
        Proxy:                 http.ProxyFromEnvironment,
        MaxIdleConns:          1,
        MaxConnsPerHost:       1,
        MaxIdleConnsPerHost:   1,
        IdleConnTimeout:       90 * time.Second,
        ExpectContinueTimeout: 1 * time.Second,
        TLSClientConfig: &tls.Config{
            InsecureSkipVerify: config.GetConfig().SSLVerify,
            Renegotiation:      tls.RenegotiateOnceAsClient,
        },
        TLSHandshakeTimeout: 10 * time.Second,
    }

    if p := proxyURLFromContext(ctx); p != nil {
        proxy := *p
        tr.Proxy = func(r *http.Request) (*url.URL, error) { return &proxy, nil }
    }

    retryClient := retryablehttp.NewClient()
    retryClient.CheckRetry = retryablehttp.ErrorPropagatedRetryPolicy
    retryClient.HTTPClient.Transport = tr
    retryClient.HTTPClient.Timeout = 30 * time.Second
    retryClient.Logger = nil
    retryClient.RetryWaitMin = 2 * time.Second
    retryClient.RetryWaitMax = 2 * time.Second
    retryClient.RetryMax = 2

    return retryClient
}

