package api

import (
	"strings"

	"github.com/google/uuid"
)

type requestHeaderer interface {
	Header(key string) string
}

func newRequestID(c requestHeaderer) string {
	if h := strings.TrimSpace(c.Header("X-Request-ID")); h != "" {
		return h
	}
	return uuid.NewString()
}
