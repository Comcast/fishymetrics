package exporter

import (
	"context"
	"net/url"
)

type proxyCtxKey string

const proxyHostKey proxyCtxKey = "proxy-host"

// WithProxyURL returns a new context that carries an override proxy URL.
func WithProxyURL(ctx context.Context, proxy string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, proxyHostKey, proxy)
}

func proxyURLFromContext(ctx context.Context) *url.URL {
	if ctx == nil {
		return nil
	}
	val := ctx.Value(proxyHostKey)
	s, _ := val.(string)
	if s == "" {
		return nil
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil
	}
	return u
}
