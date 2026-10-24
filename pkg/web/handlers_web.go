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

func (ctrl *Controller) getDeskData(channel ...string) desk.DeskData {
	uptime := time.Since(ctrl.startTime).Truncate(time.Second).String()
	ch := ""
	if len(channel) > 0 {
		ch = channel[0]
	}
	var tags []*desk.TagItem
	if ch != "" {
		tags = ctrl.hub.GetTags(ch)
	} else {
		tags = ctrl.hub.GetTags()
	}
	return desk.DeskData{
		ClientName:    ctrl.cfg.ClientName,
		WSBroker:      ctrl.cfg.WSBroker,
		Channels:      ctrl.cfg.Channels,
		Channel:       ch,
		DefaultReader: ctrl.cfg.DefaultReader,
		Addr:          ctrl.cfg.Addr,
		ExtAddr:       ctrl.cfg.ExtAddr,
		Uptime:        uptime,
		Tags:          tags,
	}
}

func (ctrl *Controller) getLoanData(channel ...string) desk.LoanData {
	uptime := time.Since(ctrl.startTime).Truncate(time.Second).String()
	ch := ""
	if len(channel) > 0 {
		ch = channel[0]
	}
	var tags []*desk.TagItem
	if ch != "" {
		tags = ctrl.hub.GetActiveTagsForChannel(ch, ctrl.cfg.DefaultReader)
	} else {
		tags = ctrl.hub.GetActiveTags(ctrl.cfg.DefaultReader)
	}
	successCount := 0
	errorCount := 0
	var failedItems []string
	for _, item := range tags {
		if item.AFIWriteStatus == "success" {
			successCount++
		} else if item.AFIWriteStatus == "error" {
			errorCount++
			label := item.ItemID
			if label == "" {
				label = item.UID
			}
			if item.AFIWriteError != "" {
				failedItems = append(failedItems, fmt.Sprintf("%s (%s)", label, item.AFIWriteError))
			} else {
				failedItems = append(failedItems, label)
			}
		}
	}

	return desk.LoanData{
		ClientName:        ctrl.cfg.ClientName,
		WSBroker:          ctrl.cfg.WSBroker,
		Channels:          ctrl.cfg.Channels,
		Channel:           ch,
		DefaultReader:     ctrl.cfg.DefaultReader,
		Addr:              ctrl.cfg.Addr,
		ExtAddr:           ctrl.cfg.ExtAddr,
		Uptime:            uptime,
		Tags:              tags,
		WriteSuccessCount: successCount,
		WriteErrorCount:   errorCount,
		FailedItems:       failedItems,
	}
}

func (ctrl *Controller) handleIndex(c *gin.Context) {
	data := ctrl.getDeskData()
	c.HTML(http.StatusOK, "index.gohtml", data)
}

func (ctrl *Controller) handleChannelIndex(c *gin.Context) {
	channel := c.Param("channel")
	data := ctrl.getDeskData(channel)
	c.HTML(http.StatusOK, "desk.gohtml", data)
}

func (ctrl *Controller) handleLoan(c *gin.Context) {
	channel := c.Param("channel")
	data := ctrl.getLoanData(channel)
	c.HTML(http.StatusOK, "loan.gohtml", data)
}

func (ctrl *Controller) handleLoanTagsFragment(c *gin.Context) {
	channel := c.Param("channel")
	data := ctrl.getLoanData(channel)
	var buf bytes.Buffer
	if err := ctrl.templates.ExecuteTemplate(&buf, "loan_table.gohtml", data); err != nil {
		c.String(http.StatusInternalServerError, "error rendering template")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}

func (ctrl *Controller) setAFIAndRespond(c *gin.Context, afi string) {
	ctx := c.Request.Context()
	targetUID := strings.TrimSpace(c.Query("uid"))
	if targetUID == "" {
		targetUID = strings.TrimSpace(c.PostForm("uid"))
	}
	if targetUID == "" {
		var reqBody struct {
			UID string `json:"uid"`
		}
		if err := c.ShouldBindJSON(&reqBody); err == nil && reqBody.UID != "" {
			targetUID = strings.TrimSpace(reqBody.UID)
		}
	}

	channel := c.Param("channel")
	activeTags := ctrl.hub.GetActiveTagsForChannel(channel, ctrl.cfg.DefaultReader)
	var targets []*desk.TagItem
	if targetUID != "" {
		for _, t := range activeTags {
			if t.UID == targetUID {
				targets = append(targets, t)
				break
			}
		}
		if len(targets) == 0 {
			targets = append(targets, &desk.TagItem{UID: targetUID})
		}
	} else {
		targets = activeTags
	}

	uids := make([]string, 0, len(targets))
	for _, t := range targets {
		if t.UID != "" {
			uids = append(uids, t.UID)
		}
	}

	if ctrl.logger != nil {
		ctrl.logger.Info().
			Str("afi", afi).
			Str("status", rfidclient.AFISecurityStatus(afi)).
			Int("targets", len(uids)).
			Strs("uids", uids).
			Msg("AFI security update triggered from web desk")
	}

	// Mark targeted tags as pending
	if len(uids) > 0 {
		ctrl.hub.SetTagAFIPending(uids...)
	}

	hadSyncWrite := false
	var writeErrors []string

	targetChannels := ctrl.cfg.Channels
	if channel != "" {
		targetChannels = []string{channel}
	}

	if ctrl.rfidClient != nil {
		if len(targets) == 0 {
			req := rfidclient.NewWriteAFIRequest(afi)
			for _, ch := range targetChannels {
				if strings.TrimSpace(ch) != "" {
					_ = ctrl.rfidClient.BroadcastAFI(ctx, ch, req)
				}
			}
			if ctrl.cfg.DefaultReader != "" {
				hadSyncWrite = true
				resp, err := ctrl.rfidClient.WriteAFI(ctx, ctrl.cfg.DefaultReader, req)
				if err != nil {
					writeErrors = append(writeErrors, err.Error())
				} else if resp != nil && !resp.IsSuccess() {
					writeErrors = append(writeErrors, resp.GetError())
				}
			}
		} else {
			// Broadcast batch write to configured channels
			batchBroadcastReq := rfidclient.NewWriteAFIBatchRequest(afi, uids)
			for _, ch := range targetChannels {
				if strings.TrimSpace(ch) != "" {
					if err := ctrl.rfidClient.BroadcastAFI(ctx, ch, batchBroadcastReq); err != nil && ctrl.logger != nil {
						ctrl.logger.Warn().Err(err).Str("channel", ch).Msg("failed to broadcast batch AFI update to channel")
					}
				}
			}

			// Cluster targets by reader client ID so each write request addresses only one reader
			readerClusters := make(map[string][]string)
			for _, tag := range targets {
				if tag.UID == "" {
					continue
				}
				targetReader := tag.ClientID
				if targetReader == "" {
					targetReader = ctrl.cfg.DefaultReader
				}
				if targetReader != "" {
					readerClusters[targetReader] = append(readerClusters[targetReader], tag.UID)
				}
			}

			// Execute batch write per reader cluster
			for targetReader, clusterUIDs := range readerClusters {
				if len(clusterUIDs) == 0 {
					continue
				}
				hadSyncWrite = true
				clusterReq := rfidclient.NewWriteAFIBatchRequest(afi, clusterUIDs)

				resp, err := ctrl.rfidClient.WriteAFI(ctx, targetReader, clusterReq)
				if err != nil {
					for _, uid := range clusterUIDs {
						ctrl.hub.SetTagAFIResult(uid, afi, false, err.Error())
						writeErrors = append(writeErrors, fmt.Sprintf("%s: %s", uid, err.Error()))
					}
					if ctrl.logger != nil {
						ctrl.logger.Warn().Err(err).Str("reader", targetReader).Strs("uids", clusterUIDs).Msg("failed to write batch AFI update to reader")
					}
				} else if resp != nil {
					for _, uid := range clusterUIDs {
						if resp.IsTagSuccessful(uid) {
							ctrl.hub.SetTagAFIResult(uid, afi, true, "")
						} else {
							errMsg := resp.GetTagError(uid)
							if errMsg == "" {
								errMsg = "AFI-Schreibfehler"
							}
							ctrl.hub.SetTagAFIResult(uid, afi, false, errMsg)
							writeErrors = append(writeErrors, fmt.Sprintf("%s: %s", uid, errMsg))
							if ctrl.logger != nil {
								ctrl.logger.Warn().Str("reader", targetReader).Str("uid", uid).Str("error", errMsg).Msg("failed to write tag AFI in batch operation")
							}
						}
					}
				}
			}
		}
	}

	data := ctrl.getLoanData(channel)
	successCount := 0
	errorCount := 0
	var failedItems []string
	for _, item := range data.Tags {
		if item.AFIWriteStatus == "success" {
			successCount++
		} else if item.AFIWriteStatus == "error" {
			errorCount++
			label := item.ItemID
			if label == "" {
				label = item.UID
			}
			if item.AFIWriteError != "" {
				failedItems = append(failedItems, fmt.Sprintf("%s: %s", label, item.AFIWriteError))
			} else {
				failedItems = append(failedItems, label)
			}
		}
	}
	data.WriteSuccessCount = successCount
	data.WriteErrorCount = errorCount
	data.FailedItems = failedItems

	statusName := rfidclient.AFISecurityStatus(afi)
	if hadSyncWrite {
		if errorCount == 0 && successCount > 0 {
			if targetUID != "" {
				data.SuccessMessage = fmt.Sprintf("AFI erfolgreich auf %s (%s) für Transponder %s gesetzt.", statusName, afi, targetUID)
			} else {
				data.SuccessMessage = fmt.Sprintf("AFI erfolgreich auf %s (%s) für alle %d Medien gesetzt.", statusName, afi, successCount)
			}
		} else if errorCount > 0 && successCount > 0 {
			data.SuccessMessage = fmt.Sprintf("%d von %d Medien erfolgreich aktualisiert.", successCount, successCount+errorCount)
			data.ErrorMessage = fmt.Sprintf("%d fehlgeschlagen: %s", errorCount, strings.Join(failedItems, "; "))
		} else if errorCount > 0 && successCount == 0 {
			data.ErrorMessage = fmt.Sprintf("AFI-Schreibvorgang fehlgeschlagen (%d Fehler): %s", errorCount, strings.Join(failedItems, "; "))
		} else if len(writeErrors) > 0 {
			data.ErrorMessage = strings.Join(writeErrors, "; ")
		} else {
			data.SuccessMessage = fmt.Sprintf("AFI erfolgreich auf %s (%s) gesetzt.", statusName, afi)
		}
		// Broadcast updated loan table via SSE
		ctrl.broadcastLoanTableWithData(data, channel)
	} else {
		// Asynchronous / broadcast write
		if targetUID != "" {
			data.SuccessMessage = fmt.Sprintf("AFI-Schreibbefehl für %s (%s) für Transponder %s an Broker übermittelt.", statusName, afi, targetUID)
		} else {
			data.SuccessMessage = fmt.Sprintf("AFI-Schreibbefehl für %s (%s) an Broker übermittelt.", statusName, afi)
		}
	}

	// Return updated table fragment for HTMX swap
	var buf bytes.Buffer
	if err := ctrl.templates.ExecuteTemplate(&buf, "loan_table.gohtml", data); err != nil {
		c.String(http.StatusInternalServerError, "error rendering template")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}

func (ctrl *Controller) handleLoanAFISecure(c *gin.Context) {
	ctrl.setAFIAndRespond(c, rfidclient.AFISecured)
}

func (ctrl *Controller) handleLoanAFIUnsecure(c *gin.Context) {
	ctrl.setAFIAndRespond(c, rfidclient.AFIUnsecured)
}

func (ctrl *Controller) handleLoanAFISingle(c *gin.Context) {
	afi := strings.TrimSpace(c.Query("afi"))
	if afi == "" {
		afi = strings.TrimSpace(c.PostForm("afi"))
	}
	action := strings.TrimSpace(c.Query("action"))
	if action == "" {
		action = strings.TrimSpace(c.PostForm("action"))
	}
	if afi == "" {
		if strings.EqualFold(action, "unsecure") || strings.EqualFold(action, "entsichern") {
			afi = rfidclient.AFIUnsecured
		} else {
			afi = rfidclient.AFISecured
		}
	}
	ctrl.setAFIAndRespond(c, afi)
}

func (ctrl *Controller) handleClearTags(c *gin.Context) {
	channel := c.Param("channel")
	if channel != "" {
		ctrl.hub.ClearForChannel(channel)
	} else {
		ctrl.hub.Clear()
	}
	// Return empty table template for HTMX replacement
	var buf bytes.Buffer
	if err := ctrl.templates.ExecuteTemplate(&buf, "tags_table.gohtml", desk.DeskData{Channel: channel, Tags: nil}); err != nil {
		c.String(http.StatusInternalServerError, "error rendering template")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}
