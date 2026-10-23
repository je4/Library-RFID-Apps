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
	Logger        zLogger.ZLogger
	StartTime     time.Time
}

// Controller manages HTTP routes, SSE streaming, and embedded frontend rendering.
type Controller struct {
	cfg       Config
	router    *gin.Engine
	server    http.Server
	templates *template.Template
	logger    zLogger.ZLogger
	hub       *desk.Hub
	startTime time.Time
}

// NewController creates and initializes the Web Controller instance.
func NewController(cfg Config) (*Controller, error) {
	if cfg.Hub == nil {
		return nil, errors.New("desk Hub cannot be nil")
	}
	if cfg.StartTime.IsZero() {
		cfg.StartTime = time.Now().UTC()
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	ctrl := &Controller{
		cfg:       cfg,
		router:    router,
		logger:    cfg.Logger,
		hub:       cfg.Hub,
		startTime: cfg.StartTime,
	}

	if err := ctrl.init(); err != nil {
		return nil, errors.Wrap(err, "failed to initialize web controller")
	}

	return ctrl, nil
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
	ctrl.router.GET("/desk", ctrl.handleIndex)
	ctrl.router.GET("/events", ctrl.handleSSE)
	ctrl.router.POST("/tags/clear", ctrl.handleClearTags)

	// REST API v1 routes
	api := ctrl.router.Group("/api/v1")
	{
		api.GET("/status", ctrl.handleAPIStatus)
		api.GET("/tags", ctrl.handleAPITags)
		api.POST("/tags/clear", ctrl.handleClearTagsJSON)
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

	// Render table row HTML fragment
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
}

// HandleTagsCleared notifies connected SSE clients that the reader field was cleared.
func (ctrl *Controller) HandleTagsCleared() {
	ctrl.hub.Broadcast(&desk.SSEMessage{
		Event: "tags_cleared",
		Data:  "cleared",
	})
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

func (ctrl *Controller) handleIndex(c *gin.Context) {
	data := ctrl.getDeskData()
	c.HTML(http.StatusOK, "desk.gohtml", data)
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
