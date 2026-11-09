package web

import (
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/je4/Library-RFID-Apps/data"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var messageBundle = loadMessageBundle()

func loadMessageBundle() *i18n.Bundle {
	bundle := i18n.NewBundle(language.German)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	for _, lang := range []string{"de", "en", "fr", "it"} {
		path := "web/static/locales/" + lang + ".json"
		contents, err := data.WebFS.ReadFile(path)
		if err != nil {
			panic(err)
		}
		if _, err := bundle.ParseMessageFileBytes(contents, path); err != nil {
			panic(err)
		}
	}
	return bundle
}

func supportedLanguage(s string) bool {
	switch s {
	case "de", "en", "fr", "it":
		return true
	}
	return false
}

func requestLanguage(r *http.Request) string {
	if r == nil {
		return "de"
	}
	if cookie, err := r.Cookie("library_desk_language"); err == nil && supportedLanguage(cookie.Value) {
		return cookie.Value
	}
	tags, _, err := language.ParseAcceptLanguage(r.Header.Get("Accept-Language"))
	if err == nil {
		for _, tag := range tags {
			base, _ := tag.Base()
			if supportedLanguage(base.String()) {
				return base.String()
			}
		}
	}
	return "de"
}

func translate(lang, message string) string {
	value, err := i18n.NewLocalizer(messageBundle, lang).Localize(&i18n.LocalizeConfig{MessageID: message})
	if err != nil {
		return message
	}
	return value
}

func templateFunctions(lang string) template.FuncMap {
	return template.FuncMap{
		"tr":   func(message string) string { return translate(lang, message) },
		"lang": func() string { return lang },
	}
}

func (ctrl *Controller) requestTemplate(c *gin.Context) *template.Template {
	return ctrl.languageTemplates[requestLanguage(c.Request)]
}

func (ctrl *Controller) renderHTML(c *gin.Context, status int, name string, data any) {
	c.Render(status, render.HTML{Template: ctrl.requestTemplate(c), Name: name, Data: data})
}

func translateGerman(message string) string {
	return translate("de", message)
}
