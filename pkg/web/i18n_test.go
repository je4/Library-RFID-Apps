package web

import (
	"bytes"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestLanguage(t *testing.T) {
	for _, tc := range []struct{ cookie, header, want string }{
		{"", "", "de"},
		{"fr", "en-US,en;q=0.8", "fr"},
		{"invalid", "it-CH,it;q=0.8,en;q=0.5", "it"},
		{"", "en;q=0.3, fr;q=0.9", "fr"},
		{"", "en;q=0, es", "de"},
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Language", tc.header)
		if tc.cookie != "" {
			req.AddCookie(&http.Cookie{Name: "library_desk_language", Value: tc.cookie})
		}
		if got := requestLanguage(req); got != tc.want {
			t.Errorf("cookie %q header %q: got %q, want %q", tc.cookie, tc.header, got, tc.want)
		}
	}
}

func TestGermanTemplateTranslation(t *testing.T) {
	tmpl, err := template.New("test").Funcs(templateFunctions("de")).Parse(`{{tr "navigation.loanReturn"}}`)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := tmpl.Execute(&output, nil); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "Ausleihe &amp; Rückgabe"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCatalogTranslation(t *testing.T) {
	for _, tc := range []struct{ lang, want string }{
		{"de", "Ausleihe & Rückgabe"},
		{"en", "Loans & returns"},
		{"fr", "Prêt et retour"},
		{"it", "Prestito e restituzione"},
	} {
		if got := translate(tc.lang, "navigation.loanReturn"); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.lang, got, tc.want)
		}
	}
	if got := translate("es", "navigation.loanReturn"); got != "Ausleihe & Rückgabe" {
		t.Errorf("fallback: got %q", got)
	}
	if got := translate("en", "missing.key"); got != "missing.key" {
		t.Errorf("missing key: got %q", got)
	}
}

func TestLoanMessageLanguages(t *testing.T) {
	success, failure := loanCountMessages(2, 1)
	for _, tc := range []struct{ lang, success, failure string }{
		{"de", "2 von 3 Medien erfolgreich aktualisiert.", "1 von 3 Medien fehlgeschlagen."},
		{"en", "2 of 3 items updated successfully.", "1 of 3 items failed."},
		{"fr", "2 documents sur 3 mis à jour avec succès.", "1 documents sur 3 en échec."},
	} {
		if got := success.text(tc.lang); got != tc.success {
			t.Errorf("%s success: %q", tc.lang, got)
		}
		if got := failure.text(tc.lang); got != tc.failure {
			t.Errorf("%s failure: %q", tc.lang, got)
		}
	}
}

func TestLocalizedSSEData(t *testing.T) {
	for _, tc := range []struct {
		event, data, french string
		json                bool
	}{
		{"loan_tags", "<p>Deutsch</p>", "<p>Français</p>", false},
		{"tag_read", "<tr>Deutsch</tr>", "<tr>Français</tr>", false},
		{"inventory_tag", `{"htmlRow":"Deutsch"}`, `{"htmlRow":"Français"}`, true},
	} {
		packed := packSSEVariants(tc.data, map[string]string{"fr": tc.french}, tc.json)
		if got := localizedSSEData(tc.event, packed, "fr"); got != tc.french {
			t.Errorf("%s French: got %q", tc.event, got)
		}
		if got := localizedSSEData(tc.event, packed, "de"); got != tc.data {
			t.Errorf("%s fallback: got %q", tc.event, got)
		}
	}
	plain := "barcode<!--library-desk-i18n:bad-->"
	if got := localizedSSEData("barcode_read", plain, "fr"); got != plain {
		t.Errorf("barcode changed: %q", got)
	}
}
