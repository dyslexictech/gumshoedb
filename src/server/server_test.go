package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"config"
)

func TestSanity(t *testing.T) {
	config := &config.Config{TableFilePath: "", NumericColumns: [][]string{{"col1", "uint8"}}}
	server := httptest.NewServer(NewServer(config))
	defer server.Close()
	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Error("Expected 404 at /")
	}
	resp.Body.Close()
}
