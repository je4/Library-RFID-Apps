package web

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/je4/Library-RFID-Apps/data"
	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/Library-RFID-Apps/pkg/rfidclient"
	"github.com/je4/utils/v2/pkg/zLogger"
)

// Config defines settings for the web dashboard server.
type Config struct {
	Addr          string
	ExtAddr       string
	ClientName    string
	WSBroker      string
	Channels      []string
	DefaultReader string
	Hub           *desk.Hub
	RFIDClient    rfidclient.Client
	Logger        zLogger.ZLogger
	StartTime     time.Time
}

// Controller manages HTTP routes, SSE streaming, and embedded frontend rendering.
type Controller struct {
	cfg        Config
	router     *gin.Engine
	server     http.Server
	templates  *template.Template
	logger     zLogger.ZLogger
	hub        *desk.Hub
	rfidClient rfidclient.Client
	startTime  time.Time
}

// NewController creates and initializes the Web Controller instance.
func NewController(cfg Config) (*Controller, error) {
	if cfg.Hub == nil {
		return nil, errors.New("desk Hub cannot be nil")
	}
	if cfg.StartTime.IsZero() {
		cfg.StartTime = time.Now().UTC()
	}

	if cfg.DefaultReader != "" && cfg.Hub.GetConfiguredReader() == "" {
		cfg.Hub.SetConfiguredReader(cfg.DefaultReader)
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	ctrl := &Controller{
		cfg:        cfg,
		router:     router,
		logger:     cfg.Logger,
		hub:        cfg.Hub,
		rfidClient: cfg.RFIDClient,
		startTime:  cfg.StartTime,
	}

	if err := ctrl.init(); err != nil {
		return nil, errors.Wrap(err, "failed to initialize web controller")
	}

	return ctrl, nil
}

// SetRFIDClient configures or updates the RFID client used for sending AFI commands.
func (ctrl *Controller) SetRFIDClient(client rfidclient.Client) {
	ctrl.rfidClient = client
}

func (ctrl *Controller) init() error {
	ctrl.router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "HX-Request", "HX-Target", "HX-Current-URL"},
		AllowCredentials: true,
	}))

	// Mount embedded static assets on /static/*
	subStatic, err := fs.Sub(data.WebFS, "web/static")
	if err != nil {
		return errors.Wrap(err, "cannot get sub filesystem for static assets")
	}
	ctrl.router.StaticFS("/static", http.FS(subStatic))

	// Parse embedded HTML templates
	tmpl, err := template.ParseFS(data.WebFS, "web/templates/*.gohtml")
	if err != nil {
		return errors.Wrap(err, "cannot parse embedded web templates")
	}
	ctrl.templates = tmpl
	ctrl.router.SetHTMLTemplate(tmpl)

	// Web Dashboard UI routes
	ctrl.router.GET("/", ctrl.handleIndex)
	ctrl.router.GET("/loan", ctrl.handleLoan)
	ctrl.router.GET("/loan/tags", ctrl.handleLoanTagsFragment)
	ctrl.router.POST("/loan/afi/secure", ctrl.handleLoanAFISecure)
	ctrl.router.POST("/loan/afi/unsecure", ctrl.handleLoanAFIUnsecure)
	ctrl.router.POST("/loan/afi/single", ctrl.handleLoanAFISingle)
	ctrl.router.GET("/events", ctrl.handleSSE)
	ctrl.router.POST("/tags/clear", ctrl.handleClearTags)

	// REST API v1 routes
	api := ctrl.router.Group("/api/v1")
	{
		api.GET("/status", ctrl.handleAPIStatus)
		api.GET("/tags", ctrl.handleAPITags)
		api.POST("/tags/clear", ctrl.handleClearTagsJSON)
		api.GET("/loan/tags", ctrl.handleAPILoanTags)
		api.POST("/loan/afi", ctrl.handleAPILoanAFI)
	}

	ctrl.server = http.Server{
		Addr:    ctrl.cfg.Addr,
		Handler: ctrl.router,
	}

	return nil
}

// Start launches the HTTP server in a background goroutine.
func (ctrl *Controller) Start(wg *sync.WaitGroup) {
	if wg != nil {
		wg.Add(1)
	}
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		if ctrl.logger != nil {
			ctrl.logger.Info().Str("addr", ctrl.cfg.Addr).Msg("starting rfid-desk web server")
		}
		if err := ctrl.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			if ctrl.logger != nil {
				ctrl.logger.Error().Err(err).Msg("rfid-desk web server encountered an error")
			}
		}
	}()
}

// GracefulStop shuts down the HTTP server.
func (ctrl *Controller) GracefulStop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := ctrl.server.Shutdown(ctx); err != nil {
		if ctrl.logger != nil {
			ctrl.logger.Warn().Err(err).Msg("error shutting down web server")
		}
	}
}

// HandleTagRead processes an incoming scan event, persists it in the hub,
// and broadcasts the rendered HTML fragment via SSE.
func (ctrl *Controller) HandleTagRead(tag *rfidclient.TagReadEvent) {
	if tag == nil {
		return
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
	})

	// Broadcast updated active loan media list
	ctrl.broadcastLoanTable()
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

	if resp.IsSuccess() {
		if resp.UID != "" {
			ctrl.hub.SetTagAFIResult(resp.UID, resp.AFI, true, "")
		} else {
			ctrl.hub.UpdateActiveAFI(resp.AFI)
		}
	} else {
		errMsg := resp.GetError()
		if errMsg == "" {
			errMsg = "Unbekannter Fehler beim Schreiben der AFI"
		}
		if resp.UID != "" {
			ctrl.hub.SetTagAFIResult(resp.UID, resp.AFI, false, errMsg)
		}
	}

	data := ctrl.getLoanData()
	statusName := rfidclient.AFISecurityStatus(resp.AFI)
	if resp.IsSuccess() {
		if resp.UID != "" {
			data.SuccessMessage = fmt.Sprintf("AFI erfolgreich auf %s (%s) für Transponder %s gesetzt.", statusName, resp.AFI, resp.UID)
		} else {
			data.SuccessMessage = fmt.Sprintf("AFI erfolgreich auf %s (%s) gesetzt.", statusName, resp.AFI)
		}
	} else {
		errMsg := resp.GetError()
		if errMsg == "" {
			errMsg = "Unbekannter Fehler beim Schreiben der AFI"
		}
		if resp.UID != "" {
			data.ErrorMessage = fmt.Sprintf("Transponder %s: %s", resp.UID, errMsg)
		} else {
			data.ErrorMessage = errMsg
		}
	}

	ctrl.broadcastLoanTableWithData(data)
}

func (ctrl *Controller) broadcastLoanTableWithData(data desk.LoanData) {
	var buf bytes.Buffer
	if err := ctrl.templates.ExecuteTemplate(&buf, "loan_table.gohtml", data); err != nil {
		if ctrl.logger != nil {
			ctrl.logger.Error().Err(err).Msg("failed to render loan_table template for SSE")
		}
		return
	}
	ctrl.hub.Broadcast(&desk.SSEMessage{
		Event: "loan_tags",
		Data:  buf.String(),
	})
}

func (ctrl *Controller) broadcastLoanTable() {
	ctrl.broadcastLoanTableWithData(ctrl.getLoanData())
}

func (ctrl *Controller) getDeskData() desk.DeskData {
	uptime := time.Since(ctrl.startTime).Truncate(time.Second).String()
	return desk.DeskData{
		ClientName:    ctrl.cfg.ClientName,
		WSBroker:      ctrl.cfg.WSBroker,
		Channels:      ctrl.cfg.Channels,
		DefaultReader: ctrl.cfg.DefaultReader,
		Addr:          ctrl.cfg.Addr,
		ExtAddr:       ctrl.cfg.ExtAddr,
		Uptime:        uptime,
		Tags:          ctrl.hub.GetTags(),
	}
}

func (ctrl *Controller) getLoanData() desk.LoanData {
	uptime := time.Since(ctrl.startTime).Truncate(time.Second).String()
	tags := ctrl.hub.GetActiveTags(ctrl.cfg.DefaultReader)
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
	c.HTML(http.StatusOK, "desk.gohtml", data)
}

func (ctrl *Controller) handleLoan(c *gin.Context) {
	data := ctrl.getLoanData()
	c.HTML(http.StatusOK, "loan.gohtml", data)
}

func (ctrl *Controller) handleLoanTagsFragment(c *gin.Context) {
	data := ctrl.getLoanData()
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

	activeTags := ctrl.hub.GetActiveTags(ctrl.cfg.DefaultReader)
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

	if ctrl.rfidClient != nil {
		if len(targets) == 0 {
			req := rfidclient.NewWriteAFIRequest(afi)
			for _, ch := range ctrl.cfg.Channels {
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
			for _, tag := range targets {
				if tag.UID == "" {
					continue
				}
				uidReq := rfidclient.NewWriteAFIRequest(afi, tag.UID)
				for _, ch := range ctrl.cfg.Channels {
					if strings.TrimSpace(ch) != "" {
						if err := ctrl.rfidClient.BroadcastAFI(ctx, ch, uidReq); err != nil && ctrl.logger != nil {
							ctrl.logger.Warn().Err(err).Str("channel", ch).Str("uid", tag.UID).Msg("failed to broadcast tag AFI update to channel")
						}
					}
				}

				targetReader := tag.ClientID
				if targetReader == "" {
					targetReader = ctrl.cfg.DefaultReader
				}

				if targetReader != "" {
					hadSyncWrite = true
					resp, err := ctrl.rfidClient.WriteAFI(ctx, targetReader, uidReq)
					if err != nil {
						ctrl.hub.SetTagAFIResult(tag.UID, afi, false, err.Error())
						writeErrors = append(writeErrors, fmt.Sprintf("%s: %s", tag.UID, err.Error()))
						if ctrl.logger != nil {
							ctrl.logger.Warn().Err(err).Str("reader", targetReader).Str("uid", tag.UID).Msg("failed to write tag AFI update to reader")
						}
					} else if resp != nil && !resp.IsSuccess() {
						errMsg := resp.GetError()
						if errMsg == "" {
							errMsg = "AFI-Schreibfehler"
						}
						ctrl.hub.SetTagAFIResult(tag.UID, afi, false, errMsg)
						writeErrors = append(writeErrors, fmt.Sprintf("%s: %s", tag.UID, errMsg))
						if ctrl.logger != nil {
							ctrl.logger.Warn().Str("reader", targetReader).Str("uid", tag.UID).Str("error", errMsg).Msg("failed to write tag AFI update to reader")
						}
					} else {
						ctrl.hub.SetTagAFIResult(tag.UID, afi, true, "")
					}
				}
			}
		}
	}

	data := ctrl.getLoanData()
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
		ctrl.broadcastLoanTableWithData(data)
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

func (ctrl *Controller) handleAPILoanTags(c *gin.Context) {
	active := ctrl.hub.GetActiveTags(ctrl.cfg.DefaultReader)
	c.JSON(http.StatusOK, active)
}

// LoanAFIRequestPayload defines the JSON payload for setting AFI via API.
type LoanAFIRequestPayload struct {
	AFI    string   `json:"afi"`
	Action string   `json:"action"`
	UIDs   []string `json:"uids"`
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

	ctx := c.Request.Context()
	activeTags := ctrl.hub.GetActiveTags(ctrl.cfg.DefaultReader)
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

	var writeErrors []string
	hadSyncWrite := false
	if ctrl.rfidClient != nil {
		if len(uids) == 0 {
			req := rfidclient.NewWriteAFIRequest(afi)
			for _, ch := range ctrl.cfg.Channels {
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
			for _, uid := range uids {
				uidReq := rfidclient.NewWriteAFIRequest(afi, uid)
				for _, ch := range ctrl.cfg.Channels {
					if strings.TrimSpace(ch) != "" {
						_ = ctrl.rfidClient.BroadcastAFI(ctx, ch, uidReq)
					}
				}

				targetReader := ctrl.cfg.DefaultReader
				if tag, ok := tagMap[uid]; ok && tag.ClientID != "" {
					targetReader = tag.ClientID
				}

				if targetReader != "" {
					hadSyncWrite = true
					resp, err := ctrl.rfidClient.WriteAFI(ctx, targetReader, uidReq)
					if err != nil {
						ctrl.hub.SetTagAFIResult(uid, afi, false, err.Error())
						writeErrors = append(writeErrors, fmt.Sprintf("%s: %s", uid, err.Error()))
					} else if resp != nil && !resp.IsSuccess() {
						errMsg := resp.GetError()
						if errMsg == "" {
							errMsg = "AFI-Schreibfehler"
						}
						ctrl.hub.SetTagAFIResult(uid, afi, false, errMsg)
						writeErrors = append(writeErrors, fmt.Sprintf("%s: %s", uid, errMsg))
					} else {
						ctrl.hub.SetTagAFIResult(uid, afi, true, "")
					}
				}
			}
		}
	}

	if hadSyncWrite {
		ctrl.broadcastLoanTable()
	}

	data := ctrl.getLoanData()
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

func (ctrl *Controller) handleClearTags(c *gin.Context) {
	ctrl.hub.Clear()
	// Return empty table template for HTMX replacement
	var buf bytes.Buffer
	if err := ctrl.templates.ExecuteTemplate(&buf, "tags_table.gohtml", desk.DeskData{Tags: nil}); err != nil {
		c.String(http.StatusInternalServerError, "error rendering template")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}

func (ctrl *Controller) handleClearTagsJSON(c *gin.Context) {
	ctrl.hub.Clear()
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "all tags cleared"})
}

func (ctrl *Controller) handleAPITags(c *gin.Context) {
	c.JSON(http.StatusOK, ctrl.hub.GetTags())
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

	msgChan := ctrl.hub.Subscribe()
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
