package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestElasticClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "APIKey test-key" {
			t.Errorf("unexpected authorization header: %q", got)
		}
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"test","cluster_name":"test","cluster_uuid":"test","version":{"number":"8.19.7"},"tagline":"You Know, for Search"}`))
	}))
	defer server.Close()

	t.Setenv("TEST_ELASTIC_APIKEY", "test-key")
	path := filepath.Join(t.TempDir(), "desk.toml")
	data := "[elasticsearch]\nendpoint = [\"" + server.URL + "\"]\nindex = \"test-index\"\napikey = \"%%TEST_ELASTIC_APIKEY%%\"\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	conf, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if conf.ElasticSearch.Index != "test-index" {
		t.Fatalf("unexpected index: %q", conf.ElasticSearch.Index)
	}
	client, err := newElasticClient(conf.ElasticSearch)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Info().Do(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestElasticClientDisabled(t *testing.T) {
	client, err := newElasticClient(ElasticSearchConfig{})
	if err != nil || client != nil {
		t.Fatalf("expected no client and no error, got %v, %v", client, err)
	}
}
