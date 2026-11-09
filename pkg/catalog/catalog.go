// Package catalog retrieves bibliographic records by item barcode.
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

// Book contains bibliographic data from _source.mapping and only the holdings
// and items whose barcode matches the search. ID is the Elasticsearch document ID.
type Book struct {
	ID             string            `json:"id"`
	Titles         []Title           `json:"titles,omitempty"`
	ISBNs          []ISBN            `json:"isbns,omitempty"`
	Languages      []string          `json:"languages,omitempty"`
	Identifiers    []string          `json:"identifiers,omitempty"`
	PersonalNames  []map[string]Name `json:"personalNames,omitempty"`
	CorporateNames []map[string]Name `json:"corporateNames,omitempty"`
	Publications   []Publication     `json:"publications,omitempty"`
	CopyrightDates []string          `json:"copyrightDates,omitempty"`
	Extent         []Extent          `json:"extent,omitempty"`
	Notes          []Note            `json:"notes,omitempty"`
	Responsibility []Note            `json:"responsibility,omitempty"`
	Subjects       Subject           `json:"subjects,omitempty"`
	Holdings       []Holding         `json:"holdings,omitempty"`
}

// Title is a main title and its optional subtitle.
type Title struct {
	Title    string `json:"title"`
	SubTitle string `json:"subTitle,omitempty"`
}

// ISBN contains an identifier and any enriched value or qualifiers.
type ISBN struct {
	ID        string   `json:"id"`
	Enriched  string   `json:"enriched,omitempty"`
	Qualifier []string `json:"qualifier,omitempty"`
}

// Name describes a personal or corporate name from a named authority source.
type Name struct {
	NamePart   string  `json:"namePart"`
	Identifier string  `json:"identifier,omitempty"`
	Date       string  `json:"date,omitempty"`
	Role       Strings `json:"role,omitempty"`
}

// Strings accepts either a single value or an array, as Elasticsearch mappings
// do not constrain whether a field in _source is multi-valued.
type Strings []string

// UnmarshalJSON accepts a string, an array of strings, or null.
func (s *Strings) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*s = nil
		return nil
	}
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*s = []string{single}
		return nil
	}
	return json.Unmarshal(data, (*[]string)(s))
}

// Publication describes a publication date, places, and publishers.
type Publication struct {
	DateString string   `json:"dateString,omitempty"`
	Place      []string `json:"place,omitempty"`
	Publisher  []string `json:"publisher,omitempty"`
}

// Extent describes the physical extent of a catalog record.
type Extent struct {
	Extent          string `json:"extent,omitempty"`
	Dimensions      string `json:"dimensions,omitempty"`
	PhysicalDetails string `json:"physicalDetails,omitempty"`
	PageNumbers     []int  `json:"pageNumbers,omitempty"`
}

// Note contains the main text of a catalog note.
type Note struct {
	Main string `json:"main"`
}

// Subject groups genre, local, and topical subject data.
type Subject struct {
	Genre map[string][]string      `json:"genre,omitempty"`
	Local LocalSubject             `json:"local,omitempty"`
	Topic []map[string]SubjectTerm `json:"topic,omitempty"`
}

// LocalSubject contains locally assigned subject strings.
type LocalSubject struct {
	All []string `json:"all,omitempty"`
}

// SubjectTerm contains the label and identifier of a topical subject.
type SubjectTerm struct {
	Label      string `json:"label,omitempty"`
	Identifier string `json:"identifier,omitempty"`
}

// Item describes an individual copy within a holding.
type Item struct {
	Barcode        string `json:"barcode"`
	Description    string `json:"description,omitempty"`
	ItemCallNumber string `json:"itemCallNumber,omitempty"`
	Note           string `json:"note,omitempty"`
}

// Holding identifies a library location and its matching items.
type Holding struct {
	Library    string `json:"library,omitempty"`
	Location   string `json:"location,omitempty"`
	CallNumber string `json:"callNumber,omitempty"`
	Items      []Item `json:"items,omitempty"`
}

type source struct {
	Mapping sourceMapping `json:"mapping"`
}

type sourceMapping struct {
	TitleInfo           sourceTitleInfo           `json:"titleInfo"`
	Identifier          sourceIdentifier          `json:"identifier"`
	Language            []string                  `json:"language"`
	RecordIdentifier    []string                  `json:"recordIdentifier"`
	Name                sourceName                `json:"name"`
	OriginInfo          sourceOriginInfo          `json:"originInfo"`
	PhysicalDescription sourcePhysicalDescription `json:"physicalDescription"`
	Note                sourceNote                `json:"note"`
	Subject             Subject                   `json:"subject"`
	Location            sourceLocation            `json:"location"`
}

type sourceTitleInfo struct {
	Main []Title `json:"main"`
}
type sourceIdentifier struct {
	ISBN []ISBN `json:"isbn"`
}
type sourceName struct {
	Personal  []map[string]Name `json:"personal"`
	Corporate []map[string]Name `json:"corporate"`
}
type sourceOriginInfo struct {
	CopyrightDate []string    `json:"copyrightDate"`
	Event         sourceEvent `json:"event"`
}
type sourceEvent struct {
	Publication []Publication `json:"publication"`
}
type sourcePhysicalDescription struct {
	Extent []Extent `json:"extent"`
}
type sourceNote struct {
	General                   []Note `json:"general"`
	StatementOfResponsibility []Note `json:"statementOfResponsibility"`
}
type sourceLocation struct {
	Holding []Holding `json:"holding"`
}

// SearchByBarcode returns all matching books, or an empty slice when none exist.
// The mapping's barcode is not indexed, so candidates are retrieved via its
// copy_to field and verified against the original barcode in _source.mapping.
// The client, index, and barcode must be supplied. An error is returned for
// incomplete searches, missing or invalid sources, and Elasticsearch failures.
func SearchByBarcode(ctx context.Context, client *elasticsearch.TypedClient, index, barcode string) ([]Book, error) {
	if client == nil || strings.TrimSpace(index) == "" || strings.TrimSpace(barcode) == "" {
		return nil, fmt.Errorf("catalog: client, index and barcode are required")
	}
	const limit = 1000
	response, err := client.Search().Index(index).Query(&types.Query{Match: map[string]types.MatchQuery{
		"search.identifier.keyword": {Query: barcode},
	}}).Size(limit).TrackTotalHits(true).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("catalog: search barcode: %w", err)
	}
	if response.TimedOut || response.Shards_.Failed > 0 {
		return nil, fmt.Errorf("catalog: incomplete Elasticsearch search")
	}
	if response.Hits.Total == nil || response.Hits.Total.Value > int64(len(response.Hits.Hits)) {
		return nil, fmt.Errorf("catalog: too many or uncounted search candidates")
	}
	books := make([]Book, 0)
	for _, hit := range response.Hits.Hits {
		var doc source
		if len(hit.Source_) == 0 || string(hit.Source_) == "null" {
			return nil, fmt.Errorf("catalog: missing document source")
		}
		if err := json.Unmarshal(hit.Source_, &doc); err != nil {
			return nil, fmt.Errorf("catalog: decode document: %w", err)
		}
		var holdings []Holding
		for _, holding := range doc.Mapping.Location.Holding {
			var items []Item
			for _, item := range holding.Items {
				if item.Barcode == barcode {
					items = append(items, item)
				}
			}
			if len(items) > 0 {
				holding.Items = items
				holdings = append(holdings, holding)
			}
		}
		if len(holdings) == 0 {
			continue
		}
		book := Book{Titles: doc.Mapping.TitleInfo.Main, ISBNs: doc.Mapping.Identifier.ISBN, Languages: doc.Mapping.Language, Identifiers: doc.Mapping.RecordIdentifier, PersonalNames: doc.Mapping.Name.Personal, CorporateNames: doc.Mapping.Name.Corporate, Publications: doc.Mapping.OriginInfo.Event.Publication, CopyrightDates: doc.Mapping.OriginInfo.CopyrightDate, Extent: doc.Mapping.PhysicalDescription.Extent, Notes: doc.Mapping.Note.General, Responsibility: doc.Mapping.Note.StatementOfResponsibility, Subjects: doc.Mapping.Subject, Holdings: holdings}
		if hit.Id_ != nil {
			book.ID = *hit.Id_
		}
		books = append(books, book)
	}
	return books, nil
}
