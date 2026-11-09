package catalog

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
)

func TestSearchByBarcodeIntegration(t *testing.T) {
	address, index := os.Getenv("CATALOG_TEST_ELASTICSEARCH_URL"), os.Getenv("CATALOG_TEST_ELASTICSEARCH_INDEX")
	if address == "" && index == "" {
		t.Skip("set CATALOG_TEST_ELASTICSEARCH_URL and CATALOG_TEST_ELASTICSEARCH_INDEX to run the Elasticsearch integration test")
	}
	if address == "" || index == "" {
		t.Fatal("both CATALOG_TEST_ELASTICSEARCH_URL and CATALOG_TEST_ELASTICSEARCH_INDEX are required")
	}
	config := elasticsearch.Config{
		Addresses: []string{address},
		APIKey:    os.Getenv("CATALOG_TEST_ELASTICSEARCH_API_KEY"),
	}
	if caPath := os.Getenv("CATALOG_TEST_ELASTICSEARCH_CA"); caPath != "" {
		ca, err := os.ReadFile(caPath)
		if err != nil {
			t.Fatalf("read Elasticsearch CA certificate: %v", err)
		}
		config.CACert = ca
	}
	client, err := elasticsearch.NewTypedClient(config)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	const barcode = "EM000008701401"
	books, err := SearchByBarcode(ctx, client, index, barcode)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, book := range books {
		for _, holding := range book.Holdings {
			for _, item := range holding.Items {
				if item.Barcode != barcode {
					t.Fatalf("unexpected barcode %q in result", item.Barcode)
				}
			}
		}
		if book.ID != "oai:alma.41SLSP_FNW:990048097120205518" {
			continue
		}
		if len(book.Titles) == 0 || book.Titles[0].Title != "Picasso and Braque" {
			t.Fatalf("unexpected titles: %+v", book.Titles)
		}
		for _, holding := range book.Holdings {
			if holding.Library == "Mediathek Basel" && holding.CallNumber == "KP 502" && len(holding.Items) > 0 {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("example record and holding not found among %d books", len(books))
	}
}
