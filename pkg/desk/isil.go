package desk

import (
	"compress/bzip2"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"emperror.dev/errors"
	"github.com/je4/Library-RFID-Apps/data"
	"github.com/je4/utils/v2/pkg/zLogger"
	"golang.org/x/exp/maps"
)

type ISILEntry struct {
	ISIL                  string `json:"isil"`
	Status                string `json:"status"`
	NameOriginal          string `json:"nameOriginal"`
	NameDE                string `json:"nameDe"`
	NameFR                string `json:"nameFr"`
	NameIT                string `json:"nameIt"`
	NameRM                string `json:"nameRm"`
	NameEN                string `json:"nameEn"`
	Mutterinstitution     string `json:"mutterinstitution"`
	VZSigel               string `json:"vzSigel"`
	VZCode                string `json:"vzCode"`
	Institutionstyp       string `json:"institutionstyp"`
	Institutionskategorie string `json:"institutionskategorie"`
	Grossregion           string `json:"grossregion"`
	Strasse               string `json:"strasse"`
	Adresszusatz          string `json:"adresszusatz"`
	Postfach              string `json:"postfach"`
	Postleitzahl          string `json:"postleitzahl"`
	Ortschaft             string `json:"ortschaft"`
	Kanton                string `json:"kanton"`
	Land                  string `json:"land"`
	Telefon               string `json:"telefon"`
	EMail                 string `json:"email"`
	Webseite              string `json:"webseite"`
	FernleiheStrasse      string `json:"fernleiheStrasse"`
	FernleiheAdresszusatz string `json:"fernleiheAdresszusatz"`
	FernleihePostfach     string `json:"fernleihePostfach"`
	FernleihePostleitzahl string `json:"fernleihePostleitzahl"`
	FernleiheOrtschaft    string `json:"fernleiheOrtschaft"`
	FernleiheTelefon      string `json:"fernleiheTelefon"`
	FernleiheMail         string `json:"fernleiheMail"`
	Startdatum            string `json:"startdatum"`
	Enddatum              string `json:"enddatum"`
	AufgegangenIn         string `json:"aufgegangenIn"`
	Verwaltungsgeschichte string `json:"verwaltungsgeschichte"`
	Bestandsbeschreibung  string `json:"bestandsbeschreibung"`
	Dewey                 string `json:"dewey"`
	Bibliotheksbestand    string `json:"bibliotheksbestand"`
	Archivbestand         string `json:"archivbestand"`
	SonstigerBestand      string `json:"sonstigerBestand"`
	OnlineKatalog         string `json:"onlineKatalog"`
	AndereFindmittel      string `json:"andereFindmittel"`
	Zugangsbedingungen    string `json:"zugangsbedingungen"`
	Zugaenglichkeitsgrad  string `json:"zugaenglichkeitsgrad"`
	Mitgliedschaft        string `json:"mitgliedschaft"`
	ZuletztGeaendert      string `json:"zuletztGeaendert"`
}

func LoadISILList(logger zLogger.ZLogger) (map[string]*ISILEntry, error) {
	isilFiles := []string{"isil-schweiz.csv.bz2"}
	result := make(map[string]*ISILEntry)

	for _, isilFile := range isilFiles {
		entries, err := loadISILFile(isilFile)
		if err != nil {
			return nil, err
		}
		maps.Copy(result, entries)
	}

	return result, nil
}

// FindISILEntry looks up an ISILEntry from the map trying multiple standard formats (e.g. "CH-000050-X", "000050-X" + country "CH").
func FindISILEntry(isilMap map[string]*ISILEntry, isil string, country string) *ISILEntry {
	if isilMap == nil {
		return nil
	}
	trimmedISIL := strings.TrimSpace(isil)
	if trimmedISIL == "" {
		return nil
	}

	// 1. Exact match
	if entry, ok := isilMap[trimmedISIL]; ok {
		return entry
	}

	// 2. Case-insensitive / normalized match if exact missed
	for k, entry := range isilMap {
		if strings.EqualFold(k, trimmedISIL) {
			return entry
		}
	}

	// 3. Try with Country prefix (e.g. CH + "-" + ISIL)
	trimmedCountry := strings.TrimSpace(country)
	if trimmedCountry != "" {
		withCountry := fmt.Sprintf("%s-%s", trimmedCountry, trimmedISIL)
		if entry, ok := isilMap[withCountry]; ok {
			return entry
		}
		for k, entry := range isilMap {
			if strings.EqualFold(k, withCountry) {
				return entry
			}
		}
	}

	// 4. Try stripping common prefixes like "ISIL-"
	if strings.HasPrefix(strings.ToUpper(trimmedISIL), "ISIL-") {
		stripped := strings.TrimPrefix(trimmedISIL[5:], "-")
		if entry := FindISILEntry(isilMap, stripped, country); entry != nil {
			return entry
		}
	}

	return nil
}

// ParseISIL extracts Country code and ISIL number from standard formatted input like "CH-000001" or "CH-ISIL-000001".
func ParseISIL(input string) (string, string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", ""
	}
	parts := strings.SplitN(input, "-", 2)
	if len(parts) == 2 && len(parts[0]) == 2 {
		return strings.ToUpper(parts[0]), strings.TrimSpace(parts[1])
	}
	return "", input
}

func loadISILFile(filename string) (map[string]*ISILEntry, error) {
	fp, err := data.WebFS.Open(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot open %s", filename)
	}
	defer fp.Close()

	csvReader := csv.NewReader(bzip2.NewReader(fp))

	heading, err := csvReader.Read()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot read header from %s", filename)
	}

	headerMap := make(map[string]int, len(heading))
	for i, head := range heading {
		headerMap[strings.TrimSpace(head)] = i
	}

	get := func(line []string, keys ...string) string {
		for _, key := range keys {
			if idx, ok := headerMap[key]; ok && idx < len(line) {
				return line[idx]
			}
		}
		return ""
	}

	entries := make(map[string]*ISILEntry)

	for recordNr := 1; ; recordNr++ {
		line, err := csvReader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, errors.Wrapf(err, "error reading record %d of %s", recordNr, filename)
		}

		if len(line) == 0 {
			continue
		}

		entry := &ISILEntry{
			ISIL:                  get(line, "ISIL"),
			Status:                get(line, "Status"),
			NameOriginal:          get(line, "Name-Original"),
			NameDE:                get(line, "Name-DE"),
			NameFR:                get(line, "Name-FR"),
			NameIT:                get(line, "Name-IT"),
			NameRM:                get(line, "Name-RM"),
			NameEN:                get(line, "Name-EN"),
			Mutterinstitution:     get(line, "Mutterinstitution"),
			VZSigel:               get(line, "VZ-Sigel"),
			VZCode:                get(line, "VZ-Code"),
			Institutionstyp:       get(line, "Institutionstyp"),
			Institutionskategorie: get(line, "Institutionskategorie"),
			Grossregion:           get(line, "Grossregion"),
			Strasse:               get(line, "Strasse"),
			Adresszusatz:          get(line, "Adresszusatz"),
			Postfach:              get(line, "Postfach"),
			Postleitzahl:          get(line, "Postleitzahl"),
			Ortschaft:             get(line, "Ortschaft"),
			Kanton:                get(line, "Kanton"),
			Land:                  get(line, "Land"),
			Telefon:               get(line, "Telefon"),
			EMail:                 get(line, "E-Mail"),
			Webseite:              get(line, "Webseite"),
			FernleiheStrasse:      get(line, "Fernleihe-Strasse"),
			FernleiheAdresszusatz: get(line, "Fernleihe-Addresszusatz", "Fernleihe-Adresszusatz"),
			FernleihePostfach:     get(line, "Fernleihe-Postfach"),
			FernleihePostleitzahl: get(line, "Fernleihe-Postleitzahl"),
			FernleiheOrtschaft:    get(line, "Fernleihe-Ortschaft"),
			FernleiheTelefon:      get(line, "Fernleihe-Telefon"),
			FernleiheMail:         get(line, "Fernleihe-Mail"),
			Startdatum:            get(line, "Startdatum"),
			Enddatum:              get(line, "Enddatum"),
			AufgegangenIn:         get(line, "Aufgegangen-In"),
			Verwaltungsgeschichte: get(line, "Verwaltungsgeschichte"),
			Bestandsbeschreibung:  get(line, "Bestandsbeschreibung"),
			Dewey:                 get(line, "Dewey"),
			Bibliotheksbestand:    get(line, "Bibliotheksbestand"),
			Archivbestand:         get(line, "Archivbestand"),
			SonstigerBestand:      get(line, "Sonstiger-Bestand"),
			OnlineKatalog:         get(line, "Online-Katalog"),
			AndereFindmittel:      get(line, "Andere-Findmittel"),
			Zugangsbedingungen:    get(line, "Zugangsbedingungen"),
			Zugaenglichkeitsgrad:  get(line, "Zugänglichkeitsgrad"),
			Mitgliedschaft:        get(line, "Mitgliedschaft"),
			ZuletztGeaendert:      get(line, "Zuletzt-Geändert"),
		}
		entries[entry.ISIL] = entry
	}

	return entries, nil
}
