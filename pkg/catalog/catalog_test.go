package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
)

type testMatchQuery struct {
	Query string `json:"query"`
}

type testQuery struct {
	Match map[string]testMatchQuery `json:"match"`
}

type testSearchBody struct {
	Query testQuery `json:"query"`
	Track bool      `json:"track_total_hits"`
}

func TestSearchByBarcode(t *testing.T) {
	data, err := os.ReadFile("../../data/elastid-example.json")
	if os.IsNotExist(err) {
		data = []byte(`{"_id":"example","_source":{"mapping":{"titleInfo":{"main":[{"title":"Picasso and Braque"}]},"identifier":{"isbn":[{"id":"0810961172"},{},{},{},{}]},"name":{"personal":[{"gnd":{"namePart":"Picasso, Pablo"}},{},{},{},{},{},{}]},"originInfo":{"event":{"publication":[{"dateString":"[1992]"}]}},"location":{"holding":[{"callNumber":"KP 502","items":[{"barcode":"EM000008701401"}]}]}}}}`)
	} else if err != nil {
		t.Fatal(err)
	}
	var example struct {
		Source json.RawMessage `json:"_source"`
		ID     string          `json:"_id"`
	}
	if err := json.Unmarshal(data, &example); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/fhnw_alma-a/_search" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		var body testSearchBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Query.Match["search.identifier.keyword"].Query != "EM000008701401" || !body.Track {
			t.Errorf("unexpected query: %+v", body)
		}
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"timed_out":false,"_shards":{"total":1,"successful":1,"failed":0},"hits":{"total":{"value":2,"relation":"eq"},"hits":[{"_index":"fhnw_alma-a","_id":"other","_source":{"mapping":{"recordIdentifier":["EM000008701401"]}}},{"_index":"fhnw_alma-a","_id":%q,"_source":%s}]}}`, example.ID, example.Source)
	}))
	defer server.Close()
	client, err := elasticsearch.NewTypedClient(elasticsearch.Config{Addresses: []string{server.URL}})
	if err != nil {
		t.Fatal(err)
	}
	books, err := SearchByBarcode(context.Background(), client, "fhnw_alma-a", "EM000008701401")
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 1 {
		t.Fatalf("got %d books", len(books))
	}
	b := books[0]
	if b.ID != example.ID || len(b.Titles) != 1 || b.Titles[0].Title != "Picasso and Braque" || len(b.ISBNs) != 5 || b.ISBNs[0].ID != "0810961172" || len(b.Publications) != 1 || b.Publications[0].DateString != "[1992]" || len(b.PersonalNames) != 7 || b.PersonalNames[0]["gnd"].NamePart != "Picasso, Pablo" || len(b.Holdings) != 1 || b.Holdings[0].CallNumber != "KP 502" || b.Holdings[0].Items[0].Barcode != "EM000008701401" {
		t.Fatalf("unexpected book: %+v", b)
	}
}

func TestSearchErrors(t *testing.T) {
	for _, tc := range []struct {
		name, response string
		status         int
		wantError      bool
	}{
		{"empty", `{"hits":{"total":{"value":0,"relation":"eq"},"hits":[]}}`, 200, false},
		{"truncated", `{"hits":{"total":{"value":1001,"relation":"eq"},"hits":[]}}`, 200, true},
		{"timeout", `{"timed_out":true,"hits":{"total":{"value":0,"relation":"eq"},"hits":[]}}`, 200, true},
		{"missing source", `{"hits":{"total":{"value":1,"relation":"eq"},"hits":[{"_index":"x"}]}}`, 200, true},
		{"server error", `{"error":{"type":"error","reason":"failure"},"status":400}`, 400, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Elastic-Product", "Elasticsearch")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.response)
			}))
			defer server.Close()
			client, err := elasticsearch.NewTypedClient(elasticsearch.Config{Addresses: []string{server.URL}})
			if err != nil {
				t.Fatal(err)
			}
			books, err := SearchByBarcode(context.Background(), client, "x", "abc")
			if (err != nil) != tc.wantError {
				t.Fatalf("books=%v err=%v", books, err)
			}
		})
	}
	if _, err := SearchByBarcode(context.Background(), nil, "x", "abc"); err == nil {
		t.Fatal("expected invalid client error")
	}
}
