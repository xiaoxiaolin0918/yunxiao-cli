package client

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

const writeNetworkHintPrefix = "check before retry:"

// IsTransientNetworkError reports connection-level failures (reset, timeout, EOF),
// not HTTP 4xx/5xx APIError bodies. Used to decide GET retries and write-side dedupe hints (#47).
func IsTransientNetworkError(err error) bool {
	if err == nil {
		return false
	}
	var ae *APIError
	if errors.As(err, &ae) {
		return false
	}
	var ne net.Error
	if errors.As(err, &ne) {
		if ne.Timeout() {
			return true
		}
	}
	var op *net.OpError
	if errors.As(err, &op) {
		return true
	}
	s := strings.ToLower(err.Error())
	needles := []string{
		"connection reset",
		"forcibly closed",
		"wsarecv",
		"broken pipe",
		"i/o timeout",
		"tls handshake timeout",
		"unexpected eof",
		"connection refused",
		"client connection lost",
		"use of closed network connection",
		"no such host",
		"network is unreachable",
	}
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

// stripWriteNetworkHint unwraps prior AnnotateWriteNetworkError layers so a more
// specific hint can replace a generic one (avoids stacked "check before retry:" lines).
func stripWriteNetworkHint(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	idx := strings.Index(msg, "\n"+writeNetworkHintPrefix)
	if idx < 0 {
		return err
	}
	base := strings.TrimSpace(msg[:idx])
	if base == "" {
		return err
	}
	return errors.New(base)
}

// AnnotateWriteNetworkError appends a dedupe hint when a non-idempotent call failed at the network layer.
// If err already contains a check-before-retry line, it is replaced with the new hint (more specific wins).
func AnnotateWriteNetworkError(err error, method, hint string) error {
	if err == nil || !IsTransientNetworkError(err) {
		return err
	}
	m := strings.ToUpper(strings.TrimSpace(method))
	if m == "GET" || m == "HEAD" {
		return err
	}
	base := stripWriteNetworkHint(err)
	hint = strings.TrimSpace(hint)
	if hint == "" {
		hint = "search existing resources before retrying (server may have applied the write)"
	}
	return fmt.Errorf("%w\n%s %s", base, writeNetworkHintPrefix, hint)
}
