package main

import (
	"net/http"
	"testing"
	"time"
)

func TestNewServerUsesBoundedTimeouts(t *testing.T) {
	server := newServer("127.0.0.1:0", http.NewServeMux())
	if server.Addr != "127.0.0.1:0" {
		t.Fatalf("address: got %q", server.Addr)
	}
	if server.ReadHeaderTimeout <= 0 || server.ReadTimeout <= 0 || server.WriteTimeout <= 0 || server.IdleTimeout <= 0 {
		t.Fatalf("timeouts must be bounded: %+v", server)
	}
	if server.WriteTimeout < time.Minute {
		t.Fatalf("write timeout too short for bounded analysis: %s", server.WriteTimeout)
	}
}
