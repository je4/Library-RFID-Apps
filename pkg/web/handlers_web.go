package web

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/Library-RFID-Apps/pkg/rfidclient"
)

func getSkinFromCookie(c *gin.Context) string {
	if c == nil {
		return "default"
	}
	skin, err := c.Cookie("rfid_skin")
	if err != nil || skin == "" {
		return "default"
	}
	return skin
}

func getThemeModeFromCookie(c *gin.Context) string {
	if c == nil {
		return "auto"
	}
	mode, err := c.Cookie("rfid_theme_mode")
	if err != nil || mode == "" {
		return "auto"
	}
	return mode
}

func (ctrl *Controller) getDeskData(channel ...string) desk.DeskData {
	uptime := time.Since(ctrl.startTime).Truncate(time.Second).String()
	ch := ""
	if len(channel) > 0 {
		ch = channel[0]
	}
	staticPath := "static"
	if ch != "" {
		staticPath = "../static"
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
		Skin:          "default",
		StaticPath:    staticPath,
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

	var errorMsg, successMsg string
	if errorCount > 0 && successCount > 0 {
		successMsg = fmt.Sprintf("%d von %d Medien erfolgreich aktualisiert.", successCount, successCount+errorCount)
		errorMsg = fmt.Sprintf("%d von %d Medien fehlgeschlagen.", errorCount, successCount+errorCount)
	} else if errorCount > 0 && successCount == 0 {
		errorMsg = fmt.Sprintf("AFI-Schreibvorgang fehlgeschlagen (%d Fehler).", errorCount)
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
		ErrorMessage:      errorMsg,
		SuccessMessage:    successMsg,
		Skin:              "default",
		StaticPath:        "../static",
	}
}

func getThemeFromCookie(c *gin.Context) string {
	if c == nil {
		return ""
	}
	theme, err := c.Cookie("rfid_theme")
	if err != nil || theme == "" {
		return ""
	}
	return theme
}

func getThemeLightFromCookie(c *gin.Context) string {
	if c == nil {
		return "corporate"
	}
	theme, err := c.Cookie("rfid_theme_light")
	if err != nil || theme == "" {
		return "corporate"
	}
	return theme
}

func getThemeDarkFromCookie(c *gin.Context) string {
	if c == nil {
		return "dark"
	}
	theme, err := c.Cookie("rfid_theme_dark")
	if err != nil || theme == "" {
		return "dark"
	}
	return theme
}

func (ctrl *Controller) handleIndex(c *gin.Context) {
	data := ctrl.getDeskData()
	data.Skin = getSkinFromCookie(c)
	data.Theme = getThemeFromCookie(c)
	data.ThemeMode = getThemeModeFromCookie(c)
	data.ThemeLight = getThemeLightFromCookie(c)
	data.ThemeDark = getThemeDarkFromCookie(c)
	c.HTML(http.StatusOK, "index.gohtml", data)
}

func (ctrl *Controller) handleChannelIndex(c *gin.Context) {
	channel := c.Param("channel")
	data := ctrl.getDeskData(channel)
	data.Skin = getSkinFromCookie(c)
	data.Theme = getThemeFromCookie(c)
	data.ThemeMode = getThemeModeFromCookie(c)
	data.ThemeLight = getThemeLightFromCookie(c)
	data.ThemeDark = getThemeDarkFromCookie(c)
	c.HTML(http.StatusOK, "desk.gohtml", data)
}

func (ctrl *Controller) handleLoan(c *gin.Context) {
	channel := c.Param("channel")
	data := ctrl.getLoanData(channel)
	data.Skin = getSkinFromCookie(c)
	data.Theme = getThemeFromCookie(c)
	data.ThemeMode = getThemeModeFromCookie(c)
	data.ThemeLight = getThemeLightFromCookie(c)
	data.ThemeDark = getThemeDarkFromCookie(c)
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

func (ctrl *Controller) getEncodeData(channel ...string) desk.EncodeData {
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

	defaultParts := len(tags)
	if defaultParts == 0 {
		defaultParts = 1
	}

	defaultISIL := ctrl.cfg.DefaultISIL
	if defaultISIL == "" {
		defaultISIL = "CH-000001"
	}

	defaultAFI := ctrl.cfg.DefaultAFI
	if defaultAFI == "" {
		defaultAFI = rfidclient.AFISecured
	} else {
		defaultAFI = rfidclient.NormalizeAFI(defaultAFI)
	}

	return desk.EncodeData{
		ClientName:    ctrl.cfg.ClientName,
		WSBroker:      ctrl.cfg.WSBroker,
		Channels:      ctrl.cfg.Channels,
		Channel:       ch,
		DefaultReader: ctrl.cfg.DefaultReader,
		Addr:          ctrl.cfg.Addr,
		ExtAddr:       ctrl.cfg.ExtAddr,
		Uptime:        uptime,
		Tags:          tags,
		DefaultISIL:   defaultISIL,
		DefaultAFI:    defaultAFI,
		DefaultParts:  defaultParts,
		Skin:          "default",
		StaticPath:    "../static",
	}
}

func (ctrl *Controller) handleEncode(c *gin.Context) {
	channel := c.Param("channel")
	data := ctrl.getEncodeData(channel)
	data.Skin = getSkinFromCookie(c)
	data.Theme = getThemeFromCookie(c)
	data.ThemeMode = getThemeModeFromCookie(c)
	data.ThemeLight = getThemeLightFromCookie(c)
	data.ThemeDark = getThemeDarkFromCookie(c)
	c.HTML(http.StatusOK, "encode.gohtml", data)
}

func (ctrl *Controller) handleEncodeTagsFragment(c *gin.Context) {
	channel := c.Param("channel")
	data := ctrl.getEncodeData(channel)
	var buf bytes.Buffer
	if err := ctrl.templates.ExecuteTemplate(&buf, "loan_table.gohtml", data); err != nil {
		c.String(http.StatusInternalServerError, "error rendering template")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}

// WriteTagPayload defines the JSON/form payload for writing metadata to an RFID tag.
type WriteTagPayload struct {
	UID       string `json:"uid" form:"uid"`
	ItemID    string `json:"itemId" form:"itemId"`
	ISIL      string `json:"isil" form:"isil"`
	Country   string `json:"country" form:"country"`
	Parts     int    `json:"parts" form:"parts"`
	PartNo    int    `json:"partNo" form:"partNo"`
	UsageType int    `json:"usageType" form:"usageType"`
	AFI       string `json:"afi" form:"afi"`
}

func (ctrl *Controller) executeWriteTag(ctx context.Context, channel string, payload WriteTagPayload) (gin.H, int) {
	itemID := strings.TrimSpace(payload.ItemID)
	if itemID == "" {
		return gin.H{"status": "error", "error": "Barcode / ItemID darf nicht leer sein"}, http.StatusBadRequest
	}

	uid := strings.TrimSpace(payload.UID)
	country := strings.TrimSpace(payload.Country)
	isil := strings.TrimSpace(payload.ISIL)

	if country == "" && isil != "" {
		c, i := desk.ParseISIL(isil)
		if c != "" {
			country = c
			isil = i
		}
	}
	if country == "" {
		country = "CH"
	}

	parts := payload.Parts
	if parts <= 0 {
		parts = 1
	}
	partNo := payload.PartNo
	if partNo <= 0 {
		partNo = 1
	}
	if partNo > parts {
		partNo = parts
	}

	usageType := payload.UsageType
	if usageType <= 0 {
		usageType = 1
	}

	afi := strings.TrimSpace(payload.AFI)
	if afi == "" {
		afi = ctrl.cfg.DefaultAFI
	}
	if afi == "" {
		afi = rfidclient.AFISecured
	}
	if strings.EqualFold(afi, "gesichert") || strings.EqualFold(afi, "secure") {
		afi = rfidclient.AFISecured
	} else if strings.EqualFold(afi, "ungesichert") || strings.EqualFold(afi, "unsecure") {
		afi = rfidclient.AFIUnsecured
	} else {
		afi = rfidclient.NormalizeAFI(afi)
	}

	// Determine target reader and daemon
	targetReader := ctrl.cfg.DefaultReader
	var targetSource string
	var targetReaderID string
	if uid != "" {
		activeTags := ctrl.hub.GetActiveTagsForChannel(channel, ctrl.cfg.DefaultReader)
		for _, t := range activeTags {
			if strings.EqualFold(t.UID, uid) {
				targetSource = t.Source
				targetReaderID = t.ReaderID
				break
			}
		}
	}
	if targetSource == "" {
		targetSource = targetReaderID
	}
	if targetSource == "" {
		targetSource = targetReader
	}

	req := rfidclient.WriteTagRequest{
		ReaderID:  targetReaderID,
		UID:       uid,
		ItemID:    itemID,
		ISIL:      isil,
		Country:   country,
		Parts:     parts,
		PartNo:    partNo,
		UsageType: usageType,
		AFI:       afi,
	}

	if ctrl.logger != nil {
		ctrl.logger.Info().
			Str("uid", uid).
			Str("itemId", itemID).
			Str("isil", isil).
			Str("country", country).
			Int("partNo", partNo).
			Int("parts", parts).
			Str("afi", afi).
			Str("target", targetSource).
			Str("readerId", targetReaderID).
			Msg("executing WriteTag request")
	}

	if ctrl.rfidClient != nil {
		resp, err := ctrl.rfidClient.WriteTag(ctx, targetSource, req)
		if err != nil {
			if ctrl.logger != nil {
				ctrl.logger.Warn().Err(err).Str("uid", uid).Msg("WriteTag failed with error")
			}
			return gin.H{
				"status": "error",
				"error":  fmt.Sprintf("Fehler beim Schreiben des Transponders %s: %v", uid, err),
				"uid":    uid,
			}, http.StatusInternalServerError
		}
		if resp != nil && !resp.Data.IsSuccess() {
			errMsg := resp.Data.GetError()
			if errMsg == "" {
				errMsg = "Schreibfehler auf Transponder"
			}
			if ctrl.logger != nil {
				ctrl.logger.Warn().Str("uid", uid).Str("error", errMsg).Msg("WriteTag returned failure")
			}
			return gin.H{
				"status": "error",
				"error":  errMsg,
				"uid":    uid,
			}, http.StatusBadRequest
		}
	}

	// Update in-memory hub
	ctrl.hub.UpdateTagMetadata(uid, itemID, isil, country, parts, partNo, afi)

	// Broadcast updated loan table
	ctrl.broadcastLoanTable(channel)

	return gin.H{
		"status":    "success",
		"message":   fmt.Sprintf("Transponder %s erfolgreich beschrieben (Teil %d von %d).", uid, partNo, parts),
		"uid":       uid,
		"itemId":    itemID,
		"isil":      isil,
		"country":   country,
		"parts":     parts,
		"partNo":    partNo,
		"usageType": usageType,
		"afi":       afi,
	}, http.StatusOK
}

func (ctrl *Controller) handleEncodeWrite(c *gin.Context) {
	channel := c.Param("channel")
	var payload WriteTagPayload

	if c.ContentType() == "application/json" || strings.Contains(c.GetHeader("Content-Type"), "application/json") {
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Ungültiges JSON-Payload: " + err.Error()})
			return
		}
	} else {
		if err := c.ShouldBind(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Ungültige Formulardaten: " + err.Error()})
			return
		}
	}

	res, statusCode := ctrl.executeWriteTag(c.Request.Context(), channel, payload)
	c.JSON(statusCode, res)
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
		// When securing/unsecuring all tags on pad, only target tags that do not already have the requested AFI status
		for _, t := range activeTags {
			if !strings.EqualFold(t.AFI, afi) {
				targets = append(targets, t)
			}
		}
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

	if ctrl.rfidClient != nil && len(targets) > 0 {
		// Cluster targets by daemon source and reader ID so each write request addresses the correct daemon & reader
		type readerClusterKey struct {
			target   string
			readerID string
		}
		clusterMap := make(map[readerClusterKey][]string)
		for _, tag := range targets {
			if tag.UID == "" {
				continue
			}
			targetDaemon := tag.Source
			if targetDaemon == "" {
				targetDaemon = tag.ReaderID
			}
			if targetDaemon == "" {
				targetDaemon = ctrl.cfg.DefaultReader
			}
			key := readerClusterKey{
				target:   targetDaemon,
				readerID: tag.ReaderID,
			}
			clusterMap[key] = append(clusterMap[key], tag.UID)
		}

		// Execute batch write per cluster
		for key, clusterUIDs := range clusterMap {
			if len(clusterUIDs) == 0 {
				continue
			}
			hadSyncWrite = true
			clusterReq := rfidclient.NewWriteAFIBatchRequest(afi, clusterUIDs)
			clusterReq.ReaderID = key.readerID

			resp, err := ctrl.rfidClient.WriteAFI(ctx, key.target, clusterReq)
			if err != nil {
				for _, uid := range clusterUIDs {
					ctrl.hub.SetTagAFIResult(uid, afi, false, err.Error())
					writeErrors = append(writeErrors, fmt.Sprintf("%s: %s", uid, err.Error()))
				}
				if ctrl.logger != nil {
					ctrl.logger.Warn().Err(err).Str("target", key.target).Str("readerId", key.readerID).Strs("uids", clusterUIDs).Msg("failed to write batch AFI update to reader")
				}
			} else if resp != nil {
				for _, uid := range clusterUIDs {
					if resp.Data.IsTagSuccessful(uid) {
						ctrl.hub.SetTagAFIResult(uid, afi, true, "")
					} else {
						errMsg := resp.Data.GetTagError(uid)
						if errMsg == "" {
							errMsg = "AFI-Schreibfehler"
						}
						ctrl.hub.SetTagAFIResult(uid, afi, false, errMsg)
						writeErrors = append(writeErrors, fmt.Sprintf("%s: %s", uid, errMsg))
						if ctrl.logger != nil {
							ctrl.logger.Warn().Str("target", key.target).Str("readerId", key.readerID).Str("uid", uid).Str("error", errMsg).Msg("failed to write tag AFI in batch operation")
						}
					}
				}
			}
		}
	} else if ctrl.rfidClient != nil && len(targets) == 0 && len(activeTags) == 0 {
		req := rfidclient.NewWriteAFIRequest(afi)
		if ctrl.cfg.DefaultReader != "" {
			hadSyncWrite = true
			resp, err := ctrl.rfidClient.WriteAFI(ctx, ctrl.cfg.DefaultReader, req)
			if err != nil {
				writeErrors = append(writeErrors, err.Error())
			} else if resp != nil && !resp.Data.IsSuccess() {
				writeErrors = append(writeErrors, resp.Data.GetError())
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
			data.ErrorMessage = fmt.Sprintf("%d von %d Medien fehlgeschlagen.", errorCount, successCount+errorCount)
		} else if errorCount > 0 && successCount == 0 {
			if targetUID != "" {
				data.ErrorMessage = fmt.Sprintf("AFI-Schreibvorgang für Transponder %s fehlgeschlagen.", targetUID)
			} else {
				data.ErrorMessage = fmt.Sprintf("AFI-Schreibvorgang fehlgeschlagen (%d Fehler).", errorCount)
			}
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
	if err := ctrl.templates.ExecuteTemplate(&buf, "tags_table.gohtml", desk.DeskData{Channel: channel, Tags: nil, StaticPath: "../static"}); err != nil {
		c.String(http.StatusInternalServerError, "error rendering template")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}
