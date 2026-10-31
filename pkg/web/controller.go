package web

import (
	"context"
	"html/template"
	"io/fs"
	"net/http"
	"slices"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/je4/Library-RFID-Apps/data"
	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
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
	DefaultISIL   string
	DefaultAFI    string
	Hub           *desk.Hub
	RFIDClient    libraryclient.Client
	ISILList      map[string]*desk.ISILEntry
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
	rfidClient libraryclient.Client
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

	if cfg.ISILList != nil && cfg.Hub.GetISILList() == nil {
		cfg.Hub.SetISILList(cfg.ISILList)
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

// SetRFIDClient configures or updates the RFID/Library client used for sending AFI commands.
func (ctrl *Controller) SetRFIDClient(client libraryclient.Client) {
	ctrl.rfidClient = client
}

func (ctrl *Controller) init() error {
	ctrl.router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Content-Length", "Accept", "Accept-Encoding", "X-CSRF-Token", "Authorization", "Cache-Control", "X-Requested-With", "SOAPAction", "HX-Request", "HX-Target", "HX-Current-URL"},
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

	// Web Index (Channel Selection) route
	ctrl.router.GET("/", ctrl.handleIndex)

	// Global Alma RFID SOAP Service routes (/TagService)
	tagServiceGroup := ctrl.router.Group("/TagService")
	{
		tagServiceGroup.POST("", ctrl.handleSOAPTagService)
		tagServiceGroup.POST("/", ctrl.handleSOAPTagService)
		tagServiceGroup.GET("", ctrl.handleSOAPWSDL)
		tagServiceGroup.GET("/", ctrl.handleSOAPWSDL)
		tagServiceGroup.OPTIONS("", func(c *gin.Context) { c.Status(http.StatusOK) })
		tagServiceGroup.OPTIONS("/", func(c *gin.Context) { c.Status(http.StatusOK) })
	}

	channelGroup := ctrl.router.Group("/:channel")
	{
		channelGroup.Use(func(c *gin.Context) {
			ch := c.Param("channel")
			if ch != "" && !ctrl.isValidChannel(ch) {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
			c.Next()
		})

		// Web Dashboard UI routes
		channelGroup.StaticFS("/static", http.FS(subStatic))
		channelGroup.GET("", ctrl.handleChannelIndex)
		channelGroup.GET("/", ctrl.handleChannelIndex)
		channelGroup.GET("/loan", ctrl.handleLoan)
		channelGroup.GET("/loan/tags", ctrl.handleLoanTagsFragment)
		channelGroup.POST("/loan/afi/secure", ctrl.handleLoanAFISecure)
		channelGroup.POST("/loan/afi/unsecure", ctrl.handleLoanAFIUnsecure)
		channelGroup.POST("/loan/afi/single", ctrl.handleLoanAFISingle)
		channelGroup.GET("/encode", ctrl.handleEncode)
		channelGroup.GET("/encode/tags", ctrl.handleEncodeTagsFragment)
		channelGroup.POST("/encode/write", ctrl.handleEncodeWrite)
		channelGroup.GET("/events", ctrl.handleSSE)
		channelGroup.POST("/tags/clear", ctrl.handleClearTags)

		// Channel-specific Alma RFID SOAP Service routes (/:channel/TagService)
		channelTagGroup := channelGroup.Group("/TagService")
		{
			channelTagGroup.POST("", ctrl.handleSOAPTagService)
			channelTagGroup.POST("/", ctrl.handleSOAPTagService)
			channelTagGroup.GET("", ctrl.handleSOAPWSDL)
			channelTagGroup.GET("/", ctrl.handleSOAPWSDL)
			channelTagGroup.OPTIONS("", func(c *gin.Context) { c.Status(http.StatusOK) })
			channelTagGroup.OPTIONS("/", func(c *gin.Context) { c.Status(http.StatusOK) })
		}
	}

	// Global events and tags clear routes
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
		api.POST("/encode/write", ctrl.handleAPIEncodeWrite)
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
			ctrl.logger.Info().Str("addr", ctrl.cfg.Addr).Msg("starting library-desk web server")
		}
		if err := ctrl.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			if ctrl.logger != nil {
				ctrl.logger.Error().Err(err).Msg("library-desk web server encountered an error")
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

func (ctrl *Controller) isValidChannel(ch string) bool {
	if len(ctrl.cfg.Channels) == 0 {
		return true
	}
	return slices.Contains(ctrl.cfg.Channels, ch)
}
