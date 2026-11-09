package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"testing"

	"github.com/je4/Library-RFID-Apps/data"
)

func TestTemplateTranslationCatalog(t *testing.T) {
	catalogs := make(map[string]map[string]string)
	for _, lang := range []string{"de", "en", "fr", "it"} {
		contents, err := data.WebFS.ReadFile("web/static/locales/" + lang + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var catalog map[string]string
		if err := json.Unmarshal(contents, &catalog); err != nil {
			t.Fatal(err)
		}
		catalogs[lang] = catalog
	}
	format := regexp.MustCompile(`%[sdv]`)
	for key, german := range catalogs["de"] {
		for _, lang := range []string{"en", "fr", "it"} {
			value, ok := catalogs[lang][key]
			if !ok || value == "" {
				t.Errorf("%s: missing %q", lang, key)
			} else if !reflect.DeepEqual(format.FindAllString(german, -1), format.FindAllString(value, -1)) {
				t.Errorf("%s: format mismatch for %q: %q", lang, key, value)
			}
		}
	}
	files, err := filepath.Glob("../../data/web/templates/*.gohtml")
	if err != nil || len(files) == 0 {
		t.Fatalf("template files: %v (found %d)", err, len(files))
	}
	call := regexp.MustCompile(`\btr\s+("(?:\\.|[^"\\])*")`)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range call.FindAllSubmatch(data, -1) {
			key, err := strconv.Unquote(string(match[1]))
			if err != nil {
				t.Fatal(err)
			}
			for _, lang := range []string{"de", "en", "fr", "it"} {
				value := catalogs[lang][key]
				if value == "" {
					t.Errorf("%s: %s missing %q", file, lang, key)
				} else if !reflect.DeepEqual(format.FindAllString(catalogs["de"][key], -1), format.FindAllString(value, -1)) {
					t.Errorf("%s: %s format mismatch for %q: %q", file, lang, key, value)
				}
			}
		}
	}
}
