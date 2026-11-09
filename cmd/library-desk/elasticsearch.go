package main

import elasticsearch "github.com/elastic/go-elasticsearch/v8"

func newElasticClient(conf ElasticSearchConfig) (*elasticsearch.TypedClient, error) {
	if len(conf.Endpoint) == 0 {
		return nil, nil
	}

	return elasticsearch.NewTypedClient(elasticsearch.Config{
		Addresses:     conf.Endpoint,
		APIKey:        string(conf.APIKey),
		RetryOnStatus: []int{502, 503, 504, 429},
		MaxRetries:    5,
	})
}
