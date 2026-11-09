package web

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
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
		ClientName:       ctrl.cfg.ClientName,
		WSBroker:         ctrl.cfg.WSBroker,
		Channels:         ctrl.cfg.Channels,
		Channel:          ch,
		Addr:             ctrl.cfg.Addr,
		ExtAddr:          ctrl.cfg.ExtAddr,
		Tags:             tags,
		Skin:             "default",
		StaticPath:       staticPath,
		InventoryEnabled: ctrl.cfg.Inventory.Enabled,
	}
}

func (ctrl *Controller) getLoanData(channel ...string) desk.LoanData {
	ch := ""
	if len(channel) > 0 {
		ch = channel[0]
	}
	var tags []*desk.TagItem
	if ch != "" {
		tags = ctrl.hub.GetActiveTagsForChannel(ch, "*")
	} else {
		tags = ctrl.hub.GetActiveTags("*")
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

	data := desk.LoanData{
		ClientName:        ctrl.cfg.ClientName,
		WSBroker:          ctrl.cfg.WSBroker,
		Channels:          ctrl.cfg.Channels,
		Channel:           ch,
		Addr:              ctrl.cfg.Addr,
		ExtAddr:           ctrl.cfg.ExtAddr,
		Tags:              tags,
		WriteSuccessCount: successCount,
		WriteErrorCount:   errorCount,
		FailedItems:       failedItems,
		Skin:              "default",
		StaticPath:        "../static",
		InventoryEnabled:  ctrl.cfg.Inventory.Enabled,
	}
	success, failure := loanCountMessages(successCount, errorCount)
	return localizeLoanMessages(data, "de", success, failure)
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

func (ctrl *Controller) populateThemeInfo(c *gin.Context, theme *string, themeMode *string, themeLight *string, themeDark *string) {
	if ctrl.cfg.ThemeLight != "" && ctrl.cfg.ThemeDark != "" {
		*themeLight = ctrl.cfg.ThemeLight
		*themeDark = ctrl.cfg.ThemeDark
		mode := getThemeModeFromCookie(c)
		*themeMode = mode
		if mode == "light" || mode == "lightmode" {
			*theme = ctrl.cfg.ThemeLight
		} else if mode == "dark" || mode == "darkmode" {
			*theme = ctrl.cfg.ThemeDark
		} else {
			*theme = ""
		}
	} else {
		*theme = getThemeFromCookie(c)
		*themeMode = getThemeModeFromCookie(c)
		*themeLight = ""
		*themeDark = ""
	}
}

func (ctrl *Controller) handleIndex(c *gin.Context) {
	data := ctrl.getDeskData()
	data.Skin = getSkinFromCookie(c)
	ctrl.populateThemeInfo(c, &data.Theme, &data.ThemeMode, &data.ThemeLight, &data.ThemeDark)
	ctrl.renderHTML(c, http.StatusOK, "index.gohtml", data)
}

func (ctrl *Controller) handleChannelIndex(c *gin.Context) {
	channel := c.Param("channel")
	data := ctrl.getDeskData(channel)
	data.Skin = getSkinFromCookie(c)
	ctrl.populateThemeInfo(c, &data.Theme, &data.ThemeMode, &data.ThemeLight, &data.ThemeDark)
	ctrl.renderHTML(c, http.StatusOK, "desk.gohtml", data)
}

func (ctrl *Controller) handleLoan(c *gin.Context) {
	channel := c.Param("channel")
	data := ctrl.getLoanData(channel)
	success, failure := loanCountMessages(data.WriteSuccessCount, data.WriteErrorCount)
	data = localizeLoanMessages(data, requestLanguage(c.Request), success, failure)
	data.Skin = getSkinFromCookie(c)
	ctrl.populateThemeInfo(c, &data.Theme, &data.ThemeMode, &data.ThemeLight, &data.ThemeDark)
	ctrl.renderHTML(c, http.StatusOK, "loan.gohtml", data)
}

func (ctrl *Controller) handleLoanTagsFragment(c *gin.Context) {
	channel := c.Param("channel")
	data := ctrl.getLoanData(channel)
	success, failure := loanCountMessages(data.WriteSuccessCount, data.WriteErrorCount)
	data = localizeLoanMessages(data, requestLanguage(c.Request), success, failure)
	var buf bytes.Buffer
	if err := ctrl.requestTemplate(c).ExecuteTemplate(&buf, "loan_table.gohtml", data); err != nil {
		c.String(http.StatusInternalServerError, "error rendering template")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}

func (ctrl *Controller) getEncodeData(channel ...string) desk.EncodeData {
	ch := ""
	if len(channel) > 0 {
		ch = channel[0]
	}
	var tags []*desk.TagItem
	if ch != "" {
		tags = ctrl.hub.GetActiveTagsForChannel(ch, "*")
	} else {
		tags = ctrl.hub.GetActiveTags("*")
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
		defaultAFI = libraryclient.AFISecured
	} else {
		defaultAFI = libraryclient.NormalizeAFI(defaultAFI)
	}

	return desk.EncodeData{
		ClientName:       ctrl.cfg.ClientName,
		WSBroker:         ctrl.cfg.WSBroker,
		Channels:         ctrl.cfg.Channels,
		Channel:          ch,
		Addr:             ctrl.cfg.Addr,
		ExtAddr:          ctrl.cfg.ExtAddr,
		Tags:             tags,
		DefaultISIL:      defaultISIL,
		DefaultAFI:       defaultAFI,
		DefaultParts:     defaultParts,
		Skin:             "default",
		StaticPath:       "../static",
		InventoryEnabled: ctrl.cfg.Inventory.Enabled,
	}
}

func (ctrl *Controller) handleEncode(c *gin.Context) {
	channel := c.Param("channel")
	data := ctrl.getEncodeData(channel)
	data.Skin = getSkinFromCookie(c)
	ctrl.populateThemeInfo(c, &data.Theme, &data.ThemeMode, &data.ThemeLight, &data.ThemeDark)
	ctrl.renderHTML(c, http.StatusOK, "encode.gohtml", data)
}

func (ctrl *Controller) handleEncodeTagsFragment(c *gin.Context) {
	channel := c.Param("channel")
	data := ctrl.getEncodeData(channel)
	var buf bytes.Buffer
	if err := ctrl.requestTemplate(c).ExecuteTemplate(&buf, "loan_table.gohtml", data); err != nil {
		c.String(http.StatusInternalServerError, "error rendering template")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}

// WriteTagPayload defines the JSON/form payload for writing metadata to an RFID tag.
type WriteTagPayload = WriteTagRequestPayload

func (ctrl *Controller) executeWriteTag(ctx context.Context, channel string, payload WriteTagPayload, language ...string) (gin.H, int) {
	if ctrl.encodeService == nil {
		ctrl.encodeService = NewEncodeService(
			ctrl.hub,
			ctrl.rfidClient,
			ctrl.cfg.DefaultISIL,
			ctrl.cfg.DefaultAFI,
			ctrl.cfg.Channels,
			ctrl.logger,
		)
	}

	lang := "de"
	if len(language) > 0 {
		lang = language[0]
	}
	res, statusCode := ctrl.encodeService.executeWriteTag(ctx, channel, payload, lang)
	if statusCode == http.StatusOK {
		ctrl.broadcastLoanTable(channel)
	}
	return res.ToGinH(), statusCode
}

func (ctrl *Controller) handleEncodeWrite(c *gin.Context) {
	channel := c.Param("channel")
	var payload WriteTagPayload

	if c.ContentType() == "application/json" || strings.Contains(c.GetHeader("Content-Type"), "application/json") {
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": fmt.Sprintf(translate(requestLanguage(c.Request), "api.invalidJson"), err.Error())})
			return
		}
	} else {
		if err := c.ShouldBind(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": fmt.Sprintf(translate(requestLanguage(c.Request), "api.invalidForm"), err.Error())})
			return
		}
	}

	res, statusCode := ctrl.executeWriteTag(c.Request.Context(), channel, payload, requestLanguage(c.Request))
	c.JSON(statusCode, res)
}

func (ctrl *Controller) handleEncodeClear(c *gin.Context) {
	channel := c.Param("channel")
	var payload WriteTagPayload

	if c.ContentType() == "application/json" || strings.Contains(c.GetHeader("Content-Type"), "application/json") {
		_ = c.ShouldBindJSON(&payload)
	} else {
		_ = c.ShouldBind(&payload)
	}

	targetUID := strings.TrimSpace(c.Query("uid"))
	if targetUID != "" && payload.UID == "" {
		payload.UID = targetUID
	}

	payload.Clear = true
	if payload.AFI == "" {
		payload.AFI = libraryclient.AFIUnsecured
	}

	res, statusCode := ctrl.executeWriteTag(c.Request.Context(), channel, payload, requestLanguage(c.Request))
	if c.GetHeader("HX-Request") == "true" {
		data := ctrl.getLoanData(channel)
		if statusCode == http.StatusOK {
			if payload.UID != "" {
				data.SuccessMessage = fmt.Sprintf(translate(requestLanguage(c.Request), "tag.clearTagSuccess"), payload.UID)
			} else {
				data.SuccessMessage = translate(requestLanguage(c.Request), "tag.clearSuccess")
			}
		} else {
			if errStr, ok := res["error"].(string); ok && errStr != "" {
				data.ErrorMessage = errStr
			} else {
				data.ErrorMessage = translate(requestLanguage(c.Request), "tag.clearError")
			}
		}
		var buf bytes.Buffer
		if err := ctrl.requestTemplate(c).ExecuteTemplate(&buf, "loan_table.gohtml", data); err == nil {
			c.Data(statusCode, "text/html; charset=utf-8", buf.Bytes())
			return
		}
	}
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
	activeTags := ctrl.hub.GetActiveTagsForChannel(channel, "*")
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
			Str("status", libraryclient.AFISecurityStatus(afi)).
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
				targetDaemon = channel
			}
			if targetDaemon == "" && len(ctrl.cfg.Channels) > 0 {
				targetDaemon = ctrl.cfg.Channels[0]
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
			clusterReq := libraryclient.NewWriteAFIBatchRequest(afi, clusterUIDs)
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
							errMsg = translateGerman("afi.writeError")
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
		req := libraryclient.NewWriteAFIRequest(afi)
		if channel != "" {
			hadSyncWrite = true
			resp, err := ctrl.rfidClient.WriteAFI(ctx, channel, req)
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

	statusName := afiStatusMessage(afi)
	var successMsg, errorMsg loanMessage
	if hadSyncWrite {
		if errorCount == 0 && successCount > 0 {
			if targetUID != "" {
				successMsg = message("afi.setTagSuccess", statusName, afi, targetUID)
			} else {
				successMsg = message("afi.setAllSuccess", statusName, afi, successCount)
			}
		} else if errorCount > 0 && successCount > 0 {
			successMsg, errorMsg = loanCountMessages(successCount, errorCount)
		} else if errorCount > 0 && successCount == 0 {
			if targetUID != "" {
				errorMsg = message("afi.tagWriteFailed", targetUID)
			} else {
				errorMsg = message("afi.writeErrors", errorCount)
			}
		} else if len(writeErrors) > 0 {
			errorMsg = loanMessage{raw: strings.Join(writeErrors, "; ")}
		} else {
			successMsg = message("afi.setSuccess", statusName, afi)
		}
		// Broadcast updated loan table via SSE
		ctrl.broadcastLoanTableMessages(data, successMsg, errorMsg, channel)
	} else {
		// Asynchronous / broadcast write
		if targetUID != "" {
			successMsg = message("afi.tagCommandSent", statusName, afi, targetUID)
		} else {
			successMsg = message("afi.commandSent", statusName, afi)
		}
	}

	data = localizeLoanMessages(data, requestLanguage(c.Request), successMsg, errorMsg)
	// Return updated table fragment for HTMX swap
	var buf bytes.Buffer
	if err := ctrl.requestTemplate(c).ExecuteTemplate(&buf, "loan_table.gohtml", data); err != nil {
		c.String(http.StatusInternalServerError, "error rendering template")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}

func (ctrl *Controller) handleLoanAFISecure(c *gin.Context) {
	ctrl.setAFIAndRespond(c, libraryclient.AFISecured)
}

func (ctrl *Controller) handleLoanAFIUnsecure(c *gin.Context) {
	ctrl.setAFIAndRespond(c, libraryclient.AFIUnsecured)
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
			afi = libraryclient.AFIUnsecured
		} else {
			afi = libraryclient.AFISecured
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
	if err := ctrl.requestTemplate(c).ExecuteTemplate(&buf, "tags_table.gohtml", desk.DeskData{Channel: channel, Tags: nil, StaticPath: "../static"}); err != nil {
		c.String(http.StatusInternalServerError, "error rendering template")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}
