# Catalog search

The `catalog` package retrieves bibliographic records by an item barcode using an existing Elasticsearch typed client. It reads book data from `_source.mapping`, not the Elasticsearch `fields` response.

## Usage

```go
client, err := elasticsearch.NewTypedClient(elasticsearch.Config{
    Addresses: []string{"https://localhost:9200"},
    APIKey: os.Getenv("ELASTIC_APIKEY"),
})
if err != nil {
    return err
}
books, err := catalog.SearchByBarcode(ctx, client, "library", barcode)
if err != nil {
    return err
}
for _, book := range books {
    fmt.Println(book.ID, book.Titles, book.Holdings)
}
```

The example assumes `ctx` and `barcode` are supplied by the caller and imports `fmt`, `os`, `github.com/elastic/go-elasticsearch/v8`, and `github.com/je4/Library-RFID-Apps/pkg/catalog`. Configure the address, index, authentication and TLS for your deployment. `ctx` controls cancellation and deadlines.

## Search and results

The mapping in `data/elastic.json` disables indexing on `mapping.location.holding.items.barcode` and copies its value to `search.identifier`. The package searches `search.identifier.keyword`, which requires the index's `ubcat_keyword` analyzer. Other identifiers also populate that search field, so each candidate is checked against the original item barcode in `_source.mapping` using an exact, case-sensitive comparison.

The result contains one `Book` per matching document. Each book includes titles, ISBNs, names, publication and physical details, notes, subjects, and record identifiers where present. Name and subject maps retain their source keys, such as authority names. Only holdings with matching items are returned, and each holding contains only those items. An empty slice means no exact item match; absence of an optional field leaves its Go zero value. `Strings` accepts either a single string or an array for name roles.

## Returned book

`SearchByBarcode` returns `([]Book, error)`. For each candidate document with at least one exactly matching item barcode, it constructs a `Book` from the following fields. Paths are relative to `_source.mapping` unless otherwise noted; no values are taken from the Elasticsearch `fields` response.

| `Book` field | Type | Source |
| --- | --- | --- |
| `ID` | `string` | Elasticsearch hit `_id` (empty if absent) |
| `Titles` | `[]Title` | `titleInfo.main` |
| `ISBNs` | `[]ISBN` | `identifier.isbn` |
| `Languages` | `[]string` | `language` |
| `Identifiers` | `[]string` | `recordIdentifier` |
| `PersonalNames` | `[]map[string]Name` | `name.personal` |
| `CorporateNames` | `[]map[string]Name` | `name.corporate` |
| `Publications` | `[]Publication` | `originInfo.event.publication` |
| `CopyrightDates` | `[]string` | `originInfo.copyrightDate` |
| `Extent` | `[]Extent` | `physicalDescription.extent` |
| `Notes` | `[]Note` | `note.general` |
| `Responsibility` | `[]Note` | `note.statementOfResponsibility` |
| `Subjects` | `Subject` | `subject` |
| `Holdings` | `[]Holding` | `location.holding`, filtered by item barcode |

The named component types contain these fields, populated from the corresponding JSON properties in each source object:

| Type | Fields |
| --- | --- |
| `Title` | `Title`, `SubTitle` |
| `ISBN` | `ID`, `Enriched`, `Qualifier` |
| `Name` | `NamePart`, `Identifier`, `Date`, `Role` |
| `Publication` | `DateString`, `Place`, `Publisher` |
| `Extent` | `Extent`, `Dimensions`, `PhysicalDetails`, `PageNumbers` |
| `Note` | `Main` |
| `Subject` | `Genre` (map of string lists), `Local` (`LocalSubject`), `Topic` (list of maps of `SubjectTerm`) |
| `LocalSubject` | `All` |
| `SubjectTerm` | `Label`, `Identifier` |
| `Holding` | `Library`, `Location`, `CallNumber`, `Items` |
| `Item` | `Barcode`, `Description`, `ItemCallNumber`, `Note` |

The candidate search may also find documents through other identifiers. For each candidate, the package decodes `_source.mapping`, retains only items whose `barcode` equals the requested barcode, and discards holdings without such items. A document without a matching holding produces no `Book`; otherwise, its remaining bibliographic fields are copied without filtering or enrichment. Multiple matching documents produce multiple books.

The client must be non-nil and the index and barcode must not be blank. Elasticsearch errors, timeouts, failed shards, missing or invalid document sources, missing totals, and searches with more candidates than returned (at most 1,000) result in an error rather than a potentially incomplete result. A successful empty result is not an error.

## Elasticsearch integration test

`TestSearchByBarcodeIntegration` performs a read-only search against a real Elasticsearch index containing the example record from `data/elastid-example.json`, using barcode `EM000008701401`. The index must have the mapping and analyzer described above, and the document must already be indexed and searchable. The test checks the record ID, title, holding, and returned item barcodes.

Set `CATALOG_TEST_ELASTICSEARCH_URL` and `CATALOG_TEST_ELASTICSEARCH_INDEX` to enable the test. Optionally set `CATALOG_TEST_ELASTICSEARCH_API_KEY` and `CATALOG_TEST_ELASTICSEARCH_CA` (path to a PEM CA certificate). For example, in PowerShell:

```powershell
$env:CATALOG_TEST_ELASTICSEARCH_URL = 'https://localhost:9200'
$env:CATALOG_TEST_ELASTICSEARCH_INDEX = 'fhnw_alma-a'
go test ./pkg/catalog -run '^TestSearchByBarcodeIntegration$' -v
```

Without either required variable the test is skipped; setting only one is an error. Credentials and server configuration are not included in the repository.