package web

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/Library-RFID-Apps/pkg/rfidclient"
)

// HandleTagRead processes an incoming scan event, persists it in the hub,
// and broadcasts the rendered HTML fragment via SSE.
func (ctrl *Controller) HandleTagRead(tag *rfidclient.TagReadEvent) {
	if tag == nil {
		return
	}
	if tag.Channel == "" && len(ctrl.cfg.Channels) == 1 {
		tag.Channel = ctrl.cfg.Channels[0]
	}
	item := ctrl.hub.AddTag(tag)

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
		Data:  buf.String(),
		ID:    item.ID,
	}, item.Channel)

	// Broadcast updated active loan media list
	ctrl.broadcastLoanTable(item.Channel)
}

// HandleTagsCleared notifies connected SSE clients that the reader field was cleared
// and updates the active media list for the loan view.
func (ctrl *Controller) HandleTagsCleared() {
	ctrl.hub.ClearActive(ctrl.cfg.DefaultReader)
	ctrl.hub.Broadcast(&desk.SSEMessage{
		Event: "tags_cleared",
		Data:  "cleared",
	})
	ctrl.broadcastLoanTable()
}

// HandleWriteAFIResponse processes an incoming AFI write response event from an RFID reader daemon.
func (ctrl *Controller) HandleWriteAFIResponse(resp *rfidclient.WriteAFIResponse) {
	if resp == nil {
		return
	}

	statusName := rfidclient.AFISecurityStatus(resp.Data.AFI)
	var successMsg, errorMsg string

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
					errMsg = "AFI-Schreibfehler"
				}
				ctrl.hub.SetTagAFIResult(res.UID, resp.Data.AFI, false, errMsg)
				failedList = append(failedList, fmt.Sprintf("%s (%s)", res.UID, errMsg))
			}
		}
		if errorCount == 0 && successCount > 0 {
			successMsg = fmt.Sprintf("AFI erfolgreich auf %s (%s) für %d Medien gesetzt.", statusName, resp.Data.AFI, successCount)
		} else if errorCount > 0 && successCount > 0 {
			successMsg = fmt.Sprintf("%d von %d Medien erfolgreich aktualisiert.", successCount, successCount+errorCount)
			errorMsg = fmt.Sprintf("%d von %d Medien fehlgeschlagen.", errorCount, successCount+errorCount)
		} else if errorCount > 0 && successCount == 0 {
			errorMsg = fmt.Sprintf("AFI-Schreibvorgang fehlgeschlagen (%d Fehler).", errorCount)
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
				errMsg = "AFI-Schreibfehler"
			}
			ctrl.hub.SetTagAFIResult(uid, resp.Data.AFI, false, errMsg)
			failedList = append(failedList, fmt.Sprintf("%s (%s)", uid, errMsg))
		}
		successCount := len(resp.Data.SuccessfulUIDs)
		errorCount := len(resp.Data.FailedUIDs)
		if errorCount == 0 && successCount > 0 {
			successMsg = fmt.Sprintf("AFI erfolgreich auf %s (%s) für %d Medien gesetzt.", statusName, resp.Data.AFI, successCount)
		} else if errorCount > 0 && successCount > 0 {
			successMsg = fmt.Sprintf("%d von %d Medien erfolgreich aktualisiert.", successCount, successCount+errorCount)
			errorMsg = fmt.Sprintf("%d von %d Medien fehlgeschlagen.", errorCount, successCount+errorCount)
		} else if errorCount > 0 && successCount == 0 {
			errorMsg = fmt.Sprintf("AFI-Schreibvorgang fehlgeschlagen (%d Fehler).", errorCount)
		}
	} else if resp.Data.IsSuccess() {
		if resp.Data.UID != "" {
			ctrl.hub.SetTagAFIResult(resp.Data.UID, resp.Data.AFI, true, "")
			successMsg = fmt.Sprintf("AFI erfolgreich auf %s (%s) für Transponder %s gesetzt.", statusName, resp.Data.AFI, resp.Data.UID)
		} else {
			ctrl.hub.UpdateActiveAFI(resp.Data.AFI)
			successMsg = fmt.Sprintf("AFI erfolgreich auf %s (%s) gesetzt.", statusName, resp.Data.AFI)
		}
	} else {
		errMsg := resp.Data.GetError()
		if errMsg == "" {
			errMsg = "Unbekannter Fehler beim Schreiben der AFI"
		}
		if resp.Data.UID != "" {
			ctrl.hub.SetTagAFIResult(resp.Data.UID, resp.Data.AFI, false, errMsg)
			errorMsg = fmt.Sprintf("Transponder %s: %s", resp.Data.UID, errMsg)
		} else if len(resp.Data.UIDs) > 0 {
			for _, uid := range resp.Data.UIDs {
				ctrl.hub.SetTagAFIResult(uid, resp.Data.AFI, false, errMsg)
			}
			errorMsg = fmt.Sprintf("AFI-Schreibvorgang für %d Medien fehlgeschlagen.", len(resp.Data.UIDs))
		} else {
			errorMsg = errMsg
		}
	}

	if len(ctrl.cfg.Channels) > 0 {
		for _, ch := range ctrl.cfg.Channels {
			loanData := ctrl.getLoanData(ch)
			loanData.SuccessMessage = successMsg
			loanData.ErrorMessage = errorMsg
			ctrl.broadcastLoanTableWithData(loanData, ch)
		}
	}
	globalData := ctrl.getLoanData("")
	globalData.SuccessMessage = successMsg
	globalData.ErrorMessage = errorMsg
	ctrl.broadcastLoanTableWithData(globalData, "")
}

func (ctrl *Controller) broadcastLoanTableWithData(data desk.LoanData, channel ...string) {
	var buf bytes.Buffer
	if err := ctrl.templates.ExecuteTemplate(&buf, "loan_table.gohtml", data); err != nil {
		if ctrl.logger != nil {
			ctrl.logger.Error().Err(err).Msg("failed to render loan_table template for SSE")
		}
		return
	}
	ch := data.Channel
	if len(channel) > 0 && channel[0] != "" {
		ch = channel[0]
	}
	ctrl.hub.Broadcast(&desk.SSEMessage{
		Event: "loan_tags",
		Data:  buf.String(),
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
			lines := strings.Split(msg.Data, "\n")
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
