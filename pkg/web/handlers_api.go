package web

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/Library-RFID-Apps/pkg/rfidclient"
)

// LoanAFIRequestPayload defines the JSON payload for setting AFI via API.
type LoanAFIRequestPayload struct {
	AFI    string   `json:"afi"`
	Action string   `json:"action"`
	UIDs   []string `json:"uids"`
}

func (ctrl *Controller) handleAPIStatus(c *gin.Context) {
	uptime := time.Since(ctrl.startTime).Truncate(time.Second).String()
	c.JSON(http.StatusOK, gin.H{
		"app":           "rfid-desk",
		"clientName":    ctrl.cfg.ClientName,
		"wsbroker":      ctrl.cfg.WSBroker,
		"channels":      ctrl.cfg.Channels,
		"defaultReader": ctrl.cfg.DefaultReader,
		"uptime":        uptime,
		"tagsCount":     ctrl.hub.Count(),
	})
}

func (ctrl *Controller) handleAPITags(c *gin.Context) {
	channel := c.Query("channel")
	c.JSON(http.StatusOK, ctrl.hub.GetTags(channel))
}

func (ctrl *Controller) handleClearTagsJSON(c *gin.Context) {
	channel := c.Query("channel")
	if channel != "" {
		ctrl.hub.ClearForChannel(channel)
	} else {
		ctrl.hub.Clear()
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "all tags cleared"})
}

func (ctrl *Controller) handleAPILoanTags(c *gin.Context) {
	channel := c.Query("channel")
	active := ctrl.hub.GetActiveTagsForChannel(channel, ctrl.cfg.DefaultReader)
	c.JSON(http.StatusOK, active)
}

func (ctrl *Controller) handleAPILoanAFI(c *gin.Context) {
	var payload LoanAFIRequestPayload
	_ = c.ShouldBindJSON(&payload)

	afi := payload.AFI
	if afi == "" {
		if strings.EqualFold(payload.Action, "secure") || strings.EqualFold(payload.Action, "sichern") {
			afi = rfidclient.AFISecured
		} else if strings.EqualFold(payload.Action, "unsecure") || strings.EqualFold(payload.Action, "entsichern") {
			afi = rfidclient.AFIUnsecured
		} else {
			afi = rfidclient.AFISecured
		}
	}

	channel := c.Query("channel")
	ctx := c.Request.Context()
	activeTags := ctrl.hub.GetActiveTagsForChannel(channel, ctrl.cfg.DefaultReader)
	tagMap := make(map[string]*desk.TagItem)
	for _, t := range activeTags {
		if t.UID != "" {
			tagMap[t.UID] = t
		}
	}

	uids := payload.UIDs
	if len(uids) == 0 {
		for _, t := range activeTags {
			if t.UID != "" {
				uids = append(uids, t.UID)
			}
		}
	}

	if ctrl.logger != nil {
		ctrl.logger.Info().
			Str("afi", afi).
			Str("status", rfidclient.AFISecurityStatus(afi)).
			Int("tagsCount", len(uids)).
			Strs("uids", uids).
			Msg("AFI security update triggered via API")
	}

	if len(uids) > 0 {
		ctrl.hub.SetTagAFIPending(uids...)
	}

	targetChannels := ctrl.cfg.Channels
	if channel != "" {
		targetChannels = []string{channel}
	}

	var writeErrors []string
	hadSyncWrite := false
	if ctrl.rfidClient != nil {
		if len(uids) == 0 {
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
					_ = ctrl.rfidClient.BroadcastAFI(ctx, ch, batchBroadcastReq)
				}
			}

			// Cluster targets by reader client ID so each write request addresses only one reader
			readerClusters := make(map[string][]string)
			for _, uid := range uids {
				targetReader := ctrl.cfg.DefaultReader
				if tag, ok := tagMap[uid]; ok && tag.ReaderID != "" {
					targetReader = tag.ReaderID
				}
				if targetReader != "" {
					readerClusters[targetReader] = append(readerClusters[targetReader], uid)
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
						ctrl.logger.Warn().Err(err).Str("reader", targetReader).Strs("uids", clusterUIDs).Msg("failed to write batch AFI update to reader via API")
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
								ctrl.logger.Warn().Str("reader", targetReader).Str("uid", uid).Str("error", errMsg).Msg("failed to write tag AFI in batch operation via API")
							}
						}
					}
				}
			}
		}
	}

	if hadSyncWrite {
		ctrl.broadcastLoanTable(channel)
	}

	data := ctrl.getLoanData(channel)
	if len(writeErrors) > 0 && data.WriteSuccessCount == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":       "error",
			"error":        strings.Join(writeErrors, "; "),
			"errors":       writeErrors,
			"afi":          afi,
			"affected":     len(uids),
			"uids":         uids,
			"successCount": 0,
			"errorCount":   data.WriteErrorCount,
			"tags":         data.Tags,
		})
		return
	}

	status := "ok"
	if len(writeErrors) > 0 && data.WriteSuccessCount > 0 {
		status = "partial"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       status,
		"afi":          afi,
		"affected":     len(uids),
		"uids":         uids,
		"successCount": data.WriteSuccessCount,
		"errorCount":   data.WriteErrorCount,
		"errors":       writeErrors,
		"tags":         data.Tags,
	})
}

func (ctrl *Controller) handleAPIEncodeWrite(c *gin.Context) {
	channel := c.Query("channel")
	var payload WriteTagPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Ungültiges JSON-Payload: " + err.Error()})
		return
	}

	res, statusCode := ctrl.executeWriteTag(c.Request.Context(), channel, payload)
	c.JSON(statusCode, res)
}
