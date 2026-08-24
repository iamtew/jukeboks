package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
)

func TestIsNormalShutdownError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "server closed", err: http.ErrServerClosed, want: true},
		{name: "context canceled", err: context.Canceled, want: true},
		{name: "net closed", err: net.ErrClosed, want: true},
		{name: "use of closed network connection", err: errors.New("use of closed network connection"), want: true},
		{name: "other error", err: errors.New("boom"), want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNormalShutdownError(tc.err); got != tc.want {
				t.Fatalf("isNormalShutdownError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
