package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
)

const sseVariantsMarker = "<!--library-desk-i18n:"

type loanMessage struct {
	id   string
	args []any
	raw  string
}

func message(id string, args ...any) loanMessage { return loanMessage{id: id, args: args} }

func afiStatusMessage(afi string) loanMessage {
	switch libraryclient.AFISecurityStatus(afi) {
	case "Gesichert":
		return message("afi.secured")
	case "Entsichert":
		return message("afi.unsecured")
	default:
		return loanMessage{}
	}
}

func (m loanMessage) text(lang string) string {
	if m.raw != "" {
		return m.raw
	}
	if m.id == "" {
		return ""
	}
	args := make([]any, len(m.args))
	for i, arg := range m.args {
		if nested, ok := arg.(loanMessage); ok {
			args[i] = nested.text(lang)
		} else {
			args[i] = arg
		}
	}
	text := translate(lang, m.id)
	if len(args) == 0 {
		return text
	}
	return fmt.Sprintf(text, args...)
}

func loanCountMessages(successCount, errorCount int) (loanMessage, loanMessage) {
	if errorCount > 0 && successCount > 0 {
		return message("afi.itemsUpdatedCount", successCount, successCount+errorCount), message("afi.itemsFailedCount", errorCount, successCount+errorCount)
	}
	if errorCount > 0 {
		return loanMessage{}, message("afi.writeErrors", errorCount)
	}
	return loanMessage{}, loanMessage{}
}

func localizeLoanMessages(data desk.LoanData, lang string, success, failure loanMessage) desk.LoanData {
	data.SuccessMessage = success.text(lang)
	data.ErrorMessage = failure.text(lang)
	return data
}

// Pack variants alongside the German payload; the HTTP stream removes this metadata.
func packSSEVariants(data string, variants map[string]string, jsonPayload bool) string {
	encoded, err := json.Marshal(variants)
	if err != nil || len(variants) == 0 {
		return data
	}
	value := base64.StdEncoding.EncodeToString(encoded)
	if jsonPayload && strings.HasSuffix(data, "}") {
		return data[:len(data)-1] + `,"_libraryDeskI18n":"` + value + `"}`
	}
	return data + sseVariantsMarker + value + "-->"
}

func localizedSSEData(event, data, lang string) string {
	var encoded string
	if event == "inventory_tag" {
		var payload map[string]json.RawMessage
		if json.Unmarshal([]byte(data), &payload) == nil && payload["_libraryDeskI18n"] != nil {
			_ = json.Unmarshal(payload["_libraryDeskI18n"], &encoded)
			delete(payload, "_libraryDeskI18n")
			if clean, err := json.Marshal(payload); err == nil {
				data = string(clean)
			}
		}
	} else if (event == "tag_read" || event == "loan_tags") && strings.Contains(data, sseVariantsMarker) && strings.HasSuffix(data, "-->") {
		i := strings.LastIndex(data, sseVariantsMarker)
		encoded = strings.TrimSuffix(data[i+len(sseVariantsMarker):], "-->")
		data = data[:i]
	}
	if decoded, err := base64.StdEncoding.DecodeString(encoded); err == nil {
		var variants map[string]string
		if json.Unmarshal(decoded, &variants) == nil && variants[lang] != "" {
			return variants[lang]
		}
	}
	return data
}

func (ctrl *Controller) sseTemplateVariants(name string, data any) map[string]string {
	variants := make(map[string]string)
	for lang, tmpl := range ctrl.languageTemplates {
		if lang == "de" {
			continue
		}
		var buf bytes.Buffer
		if tmpl.ExecuteTemplate(&buf, name, data) == nil {
			variants[lang] = buf.String()
		}
	}
	return variants
}

// HandleTagRead processes an incoming scan event, persists it in the hub,
// feeds active inventory sessions if enabled, and broadcasts the rendered HTML fragment via SSE.
func (ctrl *Controller) HandleTagRead(tag *libraryclient.TagReadEvent) {
	if tag == nil {
		return
	}
	if tag.Channel == "" && len(ctrl.cfg.Channels) == 1 {
		tag.Channel = ctrl.cfg.Channels[0]
	}

	if ctrl.inventoryService != nil {
		ctrl.inventoryService.ProcessTagRead(tag)
	}

	item := ctrl.hub.AddTag(tag)

	if ctrl.templates == nil {
		return
	}

	// Render table row HTML fragment for desk stream
	var buf bytes.Buffer
	if err := ctrl.templates.ExecuteTemplate(&buf, "tag_row.gohtml", item); err != nil {
		if ctrl.logger != nil {
			ctrl.logger.Error().Err(err).Msg("failed to render tag_row template for SSE")
		}
		return
	}

	ctrl.hub.Broadcast(&desk.SSEMessage{
		Event: "tag_read",
		Data:  packSSEVariants(buf.String(), ctrl.sseTemplateVariants("tag_row.gohtml", item), false),
		ID:    item.ID,
	}, item.Channel)

	// Broadcast updated active loan media list
	ctrl.broadcastLoanTable(item.Channel)
}

// broadcastInventoryTag renders the inventory table row fragment and broadcasts the tag read and stats via SSE.
func (ctrl *Controller) broadcastInventoryTag(payload InventoryTagEventPayload, channel string) {
	if ctrl == nil || ctrl.hub == nil {
		return
	}

	if ctrl.templates != nil && payload.Item != nil {
		var buf bytes.Buffer
		if err := ctrl.templates.ExecuteTemplate(&buf, "inventory_row.gohtml", payload.Item); err == nil {
			payload.HTMLRow = buf.String()
		} else {
			if ctrl.logger != nil {
				ctrl.logger.Error().Err(err).Msg("error rendering inventory_row.gohtml template")
			}
		}
	}

	dataBytes, err := json.Marshal(payload)
	if err == nil {
		variants := make(map[string]string)
		var rows map[string]string
		if payload.Item != nil {
			rows = ctrl.sseTemplateVariants("inventory_row.gohtml", payload.Item)
		}
		for lang, row := range rows {
			localized := payload
			localized.HTMLRow = row
			if b, err := json.Marshal(localized); err == nil {
				variants[lang] = string(b)
			}
		}
		ctrl.hub.Broadcast(&desk.SSEMessage{
			Event: "inventory_tag",
			Data:  packSSEVariants(string(dataBytes), variants, true),
		}, channel)
	}

	statsBytes, err := json.Marshal(payload.Stats)
	if err == nil {
		ctrl.hub.Broadcast(&desk.SSEMessage{
			Event: "inventory_stats",
			Data:  string(statsBytes),
		}, channel)
	}
}

// broadcastInventorySession broadcasts session lifecycle changes (start, pause, resume, reset) via SSE.
func (ctrl *Controller) broadcastInventorySession(session desk.InventorySession, channel ...string) {
	if ctrl == nil || ctrl.hub == nil {
		return
	}
	dataBytes, err := json.Marshal(session)
	if err != nil {
		return
	}
	ch := ""
	if len(channel) > 0 {
		ch = channel[0]
	}
	ctrl.hub.Broadcast(&desk.SSEMessage{
		Event: "inventory_session",
		Data:  string(dataBytes),
	}, ch)
}

// HandleTagsCleared notifies connected SSE clients that the reader field was cleared
// and updates the active media list for the loan view.
func (ctrl *Controller) HandleTagsCleared() {
	ctrl.hub.ClearActive()
	ctrl.hub.Broadcast(&desk.SSEMessage{
		Event: "tags_cleared",
		Data:  "cleared",
	})
	ctrl.broadcastLoanTable()
}

// HandleWSActivity notifies connected SSE clients of incoming WebSocket broker activity.
func (ctrl *Controller) HandleWSActivity() {
	if ctrl == nil || ctrl.hub == nil {
		return
	}
	ctrl.hub.Broadcast(&desk.SSEMessage{
		Event: "ws_activity",
		Data:  "ping",
	})
}

// HandleBarcodeRead processes an incoming barcode scan event from barcode-headless
// and broadcasts the barcode string via SSE to web clients.
func (ctrl *Controller) HandleBarcodeRead(evt *libraryclient.BarcodeReadEvent) {
	if evt == nil {
		return
	}
	barcode := strings.TrimSpace(evt.Data.GetBarcode())
	if barcode == "" {
		barcode = strings.TrimSpace(evt.Data.ItemID)
	}
	if barcode == "" {
		return
	}

	ch := evt.Channel
	if ch == "" && len(ctrl.cfg.Channels) == 1 {
		ch = ctrl.cfg.Channels[0]
	}

	ctrl.hub.Broadcast(&desk.SSEMessage{
		Event: "barcode_read",
		Data:  barcode,
		ID:    evt.ID,
	}, ch)
	if ch != "" {
		ctrl.hub.Broadcast(&desk.SSEMessage{
			Event: "barcode_read",
			Data:  barcode,
			ID:    evt.ID,
		}, "")
	}
}

// HandleWriteAFIResponse processes an incoming AFI write response event from an RFID reader daemon.
func (ctrl *Controller) HandleWriteAFIResponse(resp *libraryclient.WriteAFIResponse) {
	if resp == nil {
		return
	}

	statusName := afiStatusMessage(resp.Data.AFI)
	var successMsg, errorMsg loanMessage

	if len(resp.Data.Results) > 0 {
		successCount := 0
		errorCount := 0
		var failedList []string
		for _, res := range resp.Data.Results {
			if res.IsSuccess() {
				successCount++
				ctrl.hub.SetTagAFIResult(res.UID, resp.Data.AFI, true, "")
			} else {
				errorCount++
				errMsg := res.Error
				if errMsg == "" {
					errMsg = resp.Data.GetError()
				}
				if errMsg == "" {
					errMsg = translateGerman("afi.writeError")
				}
				ctrl.hub.SetTagAFIResult(res.UID, resp.Data.AFI, false, errMsg)
				failedList = append(failedList, fmt.Sprintf("%s (%s)", res.UID, errMsg))
			}
		}
		if errorCount == 0 && successCount > 0 {
			successMsg = message("afi.setItemsSuccess", statusName, resp.Data.AFI, successCount)
		} else if errorCount > 0 && successCount > 0 {
			successMsg, errorMsg = loanCountMessages(successCount, errorCount)
		} else if errorCount > 0 && successCount == 0 {
			errorMsg = message("afi.writeErrors", errorCount)
		}
	} else if len(resp.Data.SuccessfulUIDs) > 0 || len(resp.Data.FailedUIDs) > 0 {
		for _, uid := range resp.Data.SuccessfulUIDs {
			ctrl.hub.SetTagAFIResult(uid, resp.Data.AFI, true, "")
		}
		var failedList []string
		for uid, failErr := range resp.Data.FailedUIDs {
			errMsg := failErr
			if errMsg == "" {
				errMsg = resp.Data.GetError()
			}
			if errMsg == "" {
				errMsg = translateGerman("afi.writeError")
			}
			ctrl.hub.SetTagAFIResult(uid, resp.Data.AFI, false, errMsg)
			failedList = append(failedList, fmt.Sprintf("%s (%s)", uid, errMsg))
		}
		successCount := len(resp.Data.SuccessfulUIDs)
		errorCount := len(resp.Data.FailedUIDs)
		if errorCount == 0 && successCount > 0 {
			successMsg = message("afi.setItemsSuccess", statusName, resp.Data.AFI, successCount)
		} else if errorCount > 0 && successCount > 0 {
			successMsg, errorMsg = loanCountMessages(successCount, errorCount)
		} else if errorCount > 0 && successCount == 0 {
			errorMsg = message("afi.writeErrors", errorCount)
		}
	} else if resp.Data.IsSuccess() {
		if resp.Data.UID != "" {
			ctrl.hub.SetTagAFIResult(resp.Data.UID, resp.Data.AFI, true, "")
			successMsg = message("afi.setTagSuccess", statusName, resp.Data.AFI, resp.Data.UID)
		} else {
			ctrl.hub.UpdateActiveAFI(resp.Data.AFI)
			successMsg = message("afi.setSuccess", statusName, resp.Data.AFI)
		}
	} else {
		errMsg := resp.Data.GetError()
		var localizedError any = errMsg
		if errMsg == "" {
			localizedError = message("afi.unknownWriteError")
			errMsg = translateGerman("afi.unknownWriteError")
		}
		if resp.Data.UID != "" {
			ctrl.hub.SetTagAFIResult(resp.Data.UID, resp.Data.AFI, false, errMsg)
			errorMsg = message("tag.message", resp.Data.UID, localizedError)
		} else if len(resp.Data.UIDs) > 0 {
			for _, uid := range resp.Data.UIDs {
				ctrl.hub.SetTagAFIResult(uid, resp.Data.AFI, false, errMsg)
			}
			errorMsg = message("afi.itemsWriteFailed", len(resp.Data.UIDs))
		} else {
			if nested, ok := localizedError.(loanMessage); ok {
				errorMsg = nested
			} else {
				errorMsg = loanMessage{raw: errMsg}
			}
		}
	}

	if len(ctrl.cfg.Channels) > 0 {
		for _, ch := range ctrl.cfg.Channels {
			loanData := ctrl.getLoanData(ch)
			ctrl.broadcastLoanTableMessages(loanData, successMsg, errorMsg, ch)
		}
	}
	globalData := ctrl.getLoanData("")
	ctrl.broadcastLoanTableMessages(globalData, successMsg, errorMsg, "")
}

func (ctrl *Controller) broadcastLoanTableWithData(data desk.LoanData, channel ...string) {
	success, failure := loanCountMessages(data.WriteSuccessCount, data.WriteErrorCount)
	ctrl.broadcastLoanTableMessages(data, success, failure, channel...)
}

func (ctrl *Controller) broadcastLoanTableMessages(data desk.LoanData, success, failure loanMessage, channel ...string) {
	if ctrl.templates == nil {
		return
	}
	var buf bytes.Buffer
	if err := ctrl.templates.ExecuteTemplate(&buf, "loan_table.gohtml", localizeLoanMessages(data, "de", success, failure)); err != nil {
		if ctrl.logger != nil {
			ctrl.logger.Error().Err(err).Msg("failed to render loan_table template for SSE")
		}
		return
	}
	ch := data.Channel
	if len(channel) > 0 && channel[0] != "" {
		ch = channel[0]
	}
	variants := make(map[string]string)
	for lang, tmpl := range ctrl.languageTemplates {
		if lang == "de" {
			continue
		}
		var localized bytes.Buffer
		if tmpl.ExecuteTemplate(&localized, "loan_table.gohtml", localizeLoanMessages(data, lang, success, failure)) == nil {
			variants[lang] = localized.String()
		}
	}
	ctrl.hub.Broadcast(&desk.SSEMessage{
		Event: "loan_tags",
		Data:  packSSEVariants(buf.String(), variants, false),
	}, ch)
}

func (ctrl *Controller) broadcastLoanTable(channel ...string) {
	if len(channel) > 0 && channel[0] != "" {
		ctrl.broadcastLoanTableWithData(ctrl.getLoanData(channel[0]), channel[0])
		ctrl.broadcastLoanTableWithData(ctrl.getLoanData(""), "")
		return
	}
	if len(ctrl.cfg.Channels) > 0 {
		for _, ch := range ctrl.cfg.Channels {
			ctrl.broadcastLoanTableWithData(ctrl.getLoanData(ch), ch)
		}
	}
	ctrl.broadcastLoanTableWithData(ctrl.getLoanData(""), "")
}

func (ctrl *Controller) handleSSE(c *gin.Context) {
	w := c.Writer
	flusher, ok := w.(http.Flusher)
	if !ok {
		c.String(http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Send initial connection comment & flush immediately
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	channel := c.Param("channel")
	lang := requestLanguage(c.Request)
	msgChan := ctrl.hub.Subscribe(channel)
	defer ctrl.hub.Unsubscribe(msgChan)

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	reqCtx := c.Request.Context()

	for {
		select {
		case <-reqCtx.Done():
			return
		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case msg, open := <-msgChan:
			if !open {
				return
			}
			if msg == nil {
				continue
			}

			if msg.ID != "" {
				fmt.Fprintf(w, "id: %s\n", msg.ID)
			}
			if msg.Event != "" {
				fmt.Fprintf(w, "event: %s\n", msg.Event)
			}

			// Output each line with 'data: ' prefix
			lines := strings.Split(localizedSSEData(msg.Event, msg.Data, lang), "\n")
			for _, line := range lines {
				trimmed := strings.TrimRight(line, "\r")
				if trimmed != "" {
					fmt.Fprintf(w, "data: %s\n", trimmed)
				}
			}
			fmt.Fprintf(w, "\n")
			flusher.Flush()
		}
	}
}
