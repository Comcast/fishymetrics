/*
 * Copyright 2026 Comcast Cable Communications Management, LLC
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

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/comcast/fishymetrics/common"
)

const (
	credentialsRetrievalFailure = "failed to retrieve credentials"
	credentialsScriptTimeout    = 5 * time.Second
	// Script output should only contain a small {"user":"...","pass":"..."} JSON object.
	credentialsScriptMaxOutputBytes = 512
	credentialsScriptCommand        = "bmc-password"
)

var errCredentialsScriptOutputTooLarge = errors.New("credentials script output exceeded limit")

type credentialsScriptOutput struct {
	stdout []byte
	stderr []byte
}

func credentialsFromScript(ctx context.Context, script, target string) (*common.Credential, credentialsScriptOutput, error) {
	execCtx, cancel := context.WithTimeout(ctx, credentialsScriptTimeout)
	defer cancel()

	stdout := &cappedOutput{max: credentialsScriptMaxOutputBytes, cancel: cancel}
	stderr := &cappedOutput{max: credentialsScriptMaxOutputBytes, cancel: cancel}

	cmd := exec.CommandContext(execCtx, script, credentialsScriptCommand, target)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = time.Second

	runErr := cmd.Run()
	out := credentialsScriptOutput{
		stdout: append([]byte(nil), stdout.buf...),
		stderr: append([]byte(nil), stderr.buf...),
	}

	if errors.Is(stdout.err, errCredentialsScriptOutputTooLarge) || errors.Is(stderr.err, errCredentialsScriptOutputTooLarge) {
		return nil, out, errCredentialsScriptOutputTooLarge
	}

	if runErr != nil {
		if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			return nil, out, fmt.Errorf("credentials script timed out: %w", runErr)
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, out, fmt.Errorf("credentials script canceled: %w", runErr)
		}
		return nil, out, fmt.Errorf("credentials script failed: %w", runErr)
	}

	credential := &common.Credential{}
	if err := json.Unmarshal(out.stdout, credential); err != nil {
		return nil, out, fmt.Errorf("parse credentials script output: %w", err)
	}
	if credential.User == "" || credential.Pass == "" {
		return nil, out, fmt.Errorf("credentials script output must include user and pass")
	}

	return credential, out, nil
}

type cappedOutput struct {
	buf    []byte
	max    int
	cancel context.CancelFunc
	err    error
}

func (o *cappedOutput) Write(p []byte) (int, error) {
	if space := o.max - len(o.buf); len(p) > space {
		o.buf = append(o.buf, p[:space]...)
		o.err = errCredentialsScriptOutputTooLarge
		o.cancel()
		return space, errCredentialsScriptOutputTooLarge
	}
	o.buf = append(o.buf, p...)
	return len(p), nil
}

func validateCredentialsScriptTarget(target string) error {
	host, err := credentialsScriptTargetHost(target)
	if err != nil {
		return err
	}
	if isValidIPAddress(host) || isValidDomainName(host) {
		return nil
	}
	return fmt.Errorf("target must be a valid domain or IP address")
}

func credentialsScriptTargetHost(target string) (string, error) {
	if target == "" {
		return "", fmt.Errorf("target is empty")
	}
	if strings.HasPrefix(target, "-") {
		return "", fmt.Errorf("target must not start with a hyphen")
	}
	if strings.ContainsRune(target, '\x00') {
		return "", fmt.Errorf("target contains a null byte")
	}

	if strings.Contains(target, "://") {
		u, err := url.ParseRequestURI(target)
		if err != nil {
			return "", fmt.Errorf("target URL is invalid: %w", err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return "", fmt.Errorf("target URL scheme must be http or https")
		}
		if u.User != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return "", fmt.Errorf("target URL must contain only a scheme, host, and optional port")
		}
		if u.Port() != "" && !isValidPort(u.Port()) {
			return "", fmt.Errorf("target URL port is invalid")
		}
		return u.Hostname(), nil
	}

	if strings.ContainsAny(target, "/?#@") {
		return "", fmt.Errorf("target must not include URL path, query, fragment, or userinfo")
	}

	host := target
	if splitHost, port, err := net.SplitHostPort(target); err == nil {
		if !isValidPort(port) {
			return "", fmt.Errorf("target port is invalid")
		}
		if strings.HasPrefix(target, "[") && !isValidIPAddress(splitHost) {
			return "", fmt.Errorf("bracketed target host must be an IP address")
		}
		host = splitHost
	} else if strings.HasPrefix(target, "[") && strings.HasSuffix(target, "]") {
		host = strings.TrimSuffix(strings.TrimPrefix(target, "["), "]")
		if !isValidIPAddress(host) {
			return "", fmt.Errorf("bracketed target host must be an IP address")
		}
	} else if strings.Count(target, ":") == 1 {
		return "", fmt.Errorf("target port is invalid")
	}

	if host == "" {
		return "", fmt.Errorf("target host is empty")
	}
	return host, nil
}

func isValidIPAddress(host string) bool {
	_, err := netip.ParseAddr(host)
	return err == nil
}

func isValidDomainName(host string) bool {
	host = strings.TrimSuffix(host, ".")
	if host == "" || len(host) > 253 {
		return false
	}

	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, r := range label {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
				continue
			}
			return false
		}
	}

	return true
}

func isValidPort(port string) bool {
	p, err := strconv.Atoi(port)
	return err == nil && p > 0 && p <= 65535
}
