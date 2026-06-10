package main

import "testing"

func TestLocalBaseURL(t *testing.T) {
	tests := []struct {
		addr string
		want string
	}{
		{addr: ":18080", want: "http://localhost:18080"},
		{addr: "127.0.0.1:8080", want: "http://127.0.0.1:8080"},
	}

	for _, tt := range tests {
		if got := localBaseURL(tt.addr); got != tt.want {
			t.Fatalf("localBaseURL(%q) = %q, want %q", tt.addr, got, tt.want)
		}
	}
}
