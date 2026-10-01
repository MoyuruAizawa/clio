package jev

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/systemone" || r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("request headers/path")
		}
		var body struct {
			Model     string  `json:"model"`
			State     Request `json:"state"`
			Questions map[string]struct {
				Type         string `json:"type"`
				Instructions string `json:"instructions"`
			} `json:"questions"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil || body.Model != "custom" || body.State.ExitCode != 8 || body.State.AgentContext != "Compile the pendant changes into a debug APK." || len(body.State.Command) != 2 || body.State.Command[0] != "./gradlew" || body.State.Command[1] != "assembleDevelopDebug" || len(body.Questions) != 3 || body.Questions["x"].Type != "noul" || body.Questions["x_include_before"].Type != "noul" || body.Questions["x_include_after"].Type != "noul" || !strings.Contains(body.Questions["x"].Instructions, "chunks[0]") || !strings.Contains(body.Questions["x"].Instructions, "agent_context") {
			t.Error("request contract")
		}
		fmt.Fprint(w, `{"answers":{"x":{"type":"noul","noul":0.75},"x_include_before":{"type":"noul","noul":0.25},"x_include_after":{"type":"noul","noul":0.75}}}`)
	}))
	defer server.Close()
	c := HTTPClient{Token: "secret", Model: "custom", URL: server.URL + "/v1/systemone"}
	s, e := c.Evaluate(context.Background(), Request{Command: []string{"./gradlew", "assembleDevelopDebug"}, AgentContext: "Compile the pendant changes into a debug APK.", ExitCode: 8, Chunks: []Chunk{{ID: "x", Text: "failed"}}})
	if e != nil || s["x"] != 0.75 {
		t.Fatal(s, e)
	}
}

func TestHTTPErrors(t *testing.T) {
	for _, body := range []string{`oops`, `{"answers":{}}`, `{"answers":{"x":{"type":"noul"}}}`, `{"answers":{"x":{"type":"noul","noul":2}}}`, `{"answers":{"x":{"type":"score","noul":0.8}}}`} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
		c := HTTPClient{URL: s.URL}
		_, e := c.Evaluate(context.Background(), Request{Chunks: []Chunk{{ID: "x"}}})
		s.Close()
		if e == nil {
			t.Fatal(body)
		}
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401); fmt.Fprint(w, "secret") }))
	defer s.Close()
	_, e := (&HTTPClient{Token: "secret", URL: s.URL}).Evaluate(context.Background(), Request{})
	if e == nil || strings.Contains(e.Error(), "secret") || !strings.Contains(e.Error(), "401") {
		t.Fatal(e)
	}
}
func TestRetryAndDeadline(t *testing.T) {
	for _, status := range []int{429, 529} {
		var calls atomic.Int32
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) < 3 {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(status)
				return
			}
			fmt.Fprint(w, `{"answers":{"x":{"type":"noul","noul":1},"x_include_before":{"type":"noul","noul":0},"x_include_after":{"type":"noul","noul":1}}}`)
		}))
		_, e := (&HTTPClient{URL: s.URL}).Evaluate(context.Background(), Request{Chunks: []Chunk{{ID: "x"}}})
		s.Close()
		if e != nil || calls.Load() != 3 {
			t.Fatal(calls.Load(), e)
		}
	}
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "10")
		w.WriteHeader(429)
	}))
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, e := (&HTTPClient{URL: s.URL}).Evaluate(ctx, Request{})
	if e == nil || calls.Load() != 1 {
		t.Fatal(e, calls.Load())
	}
}
func TestHTTPTimeoutAndRetryLimit(t *testing.T) {
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(529) }))
	_, e := (&HTTPClient{URL: s.URL}).Evaluate(context.Background(), Request{})
	s.Close()
	if e == nil || calls.Load() != 3 {
		t.Fatal(e, calls.Load())
	}
	s = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(80 * time.Millisecond) }))
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, e = (&HTTPClient{URL: s.URL}).Evaluate(ctx, Request{})
	if e == nil {
		t.Fatal("timeout not reported")
	}
}
