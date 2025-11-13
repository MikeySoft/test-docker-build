package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRunSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{Timeout: time.Second}
	if code := run(server.URL, time.Second, http.StatusOK, client); code != 0 {
		t.Fatalf("run success expected exit code 0, got %d", code)
	}
}

func TestRunFailureStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &http.Client{Timeout: time.Second}
	if code := run(server.URL, time.Second, http.StatusOK, client); code != 1 {
		t.Fatalf("run expected exit code 1 for bad status, got %d", code)
	}
}

func TestRunBadURL(t *testing.T) {
	client := &http.Client{Timeout: time.Second}
	if code := run(":", time.Second, http.StatusOK, client); code != 2 {
		t.Fatalf("run expected exit code 2 for bad request, got %d", code)
	}
}
