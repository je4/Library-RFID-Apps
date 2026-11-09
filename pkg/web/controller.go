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
	"github.com/je4/Library-RFID-Apps/pkg/catalog"
	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
	"github.com/je4/utils/v2/pkg/zLogger"
)

// Config defines settings for the web dashboard server.
type Config struct {
	CatalogSearch func(context.Context, string) ([]catalog.Book, error)
	Addr          string
	ExtAddr       string
	ClientName    string
	WSBroker      string
	Channels      []string
	DefaultISIL   string
	DefaultAFI    string
	ThemeLight    string
	ThemeDark     string
	Inventory     desk.InventoryConfig
	Hub           *desk.Hub
	RFIDClient    libraryclient.Client
	ISILList      map[string]*desk.ISILEntry
	Logger        zLogger.ZLogger
	SOAPOnly      bool
}

// Controller manages HTTP routes, SSE streaming, and embedded frontend rendering.
type Controller struct {
	cfg              Config
	router           *gin.Engine
	server           http.Server
	templates        *template.Template
	logger           zLogger.ZLogger
	hub              *desk.Hub
	rfidClient       libraryclient.Client
	encodeService    *EncodeService
	inventoryService *InventoryService
	catalogService   *catalogService
}

// NewController creates and initializes the Web Controller instance.
func NewController(cfg Config) (*Controller, error) {
	if cfg.Hub == nil {
		return nil, errors.New("desk Hub cannot be nil")
	}

	if cfg.ISILList != nil && cfg.Hub.GetISILList() == nil {
		cfg.Hub.SetISILList(cfg.ISILList)
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	encodeService := NewEncodeService(
		cfg.Hub,
		cfg.RFIDClient,
		cfg.DefaultISIL,
		cfg.DefaultAFI,
		cfg.Channels,
		cfg.Logger,
	)

	var invService *InventoryService
	if cfg.Inventory.Enabled {
		invService = NewInventoryService(
			cfg.Inventory,
			cfg.ClientName,
			cfg.Hub,
			nil,
			cfg.Logger,
		)
	}

	ctrl := &Controller{
		cfg:              cfg,
		router:           router,
		logger:           cfg.Logger,
		hub:              cfg.Hub,
		rfidClient:       cfg.RFIDClient,
		encodeService:    encodeService,
		inventoryService: invService,
	}
	if cfg.CatalogSearch != nil {
		ctrl.catalogService = newCatalogService(cfg.CatalogSearch)
	}

	if invService != nil {
		invService.SetOnTagEvent(ctrl.broadcastInventoryTag)
	}

	if err := ctrl.init(); err != nil {
		return nil, errors.Wrap(err, "failed to initialize web controller")
	}

	return ctrl, nil
}

// SetRFIDClient configures or updates the RFID/Library client used for sending AFI commands.
func (ctrl *Controller) SetRFIDClient(client libraryclient.Client) {
	ctrl.rfidClient = client
	if ctrl.encodeService != nil {
		ctrl.encodeService.rfidClient = client
	}
}

func (ctrl *Controller) init() error {
	ctrl.router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Content-Length", "Accept", "Accept-Encoding", "X-CSRF-Token", "Authorization", "Cache-Control", "X-Requested-With", "SOAPAction", "HX-Request", "HX-Target", "HX-Current-URL"},
		AllowCredentials: true,
	}))

	var subStatic fs.FS
	if !ctrl.cfg.SOAPOnly {
		// Mount embedded static assets on /static/*
		var err error
		subStatic, err = fs.Sub(data.WebFS, "web/static")
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
	}

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

		if !ctrl.cfg.SOAPOnly {
			// Web Dashboard UI routes
			channelGroup.StaticFS("/static", http.FS(subStatic))
			channelGroup.GET("", ctrl.handleChannelIndex)
			channelGroup.GET("/", ctrl.handleChannelIndex)
			channelGroup.GET("/loan", ctrl.handleLoan)
			channelGroup.GET("/catalog", ctrl.handleCatalog)
			channelGroup.GET("/loan/tags", ctrl.handleLoanTagsFragment)
			channelGroup.POST("/loan/afi/secure", ctrl.handleLoanAFISecure)
			channelGroup.POST("/loan/afi/unsecure", ctrl.handleLoanAFIUnsecure)
			channelGroup.POST("/loan/afi/single", ctrl.handleLoanAFISingle)
			channelGroup.POST("/loan/clear", ctrl.handleEncodeClear)
			channelGroup.GET("/encode", ctrl.handleEncode)
			channelGroup.GET("/encode/tags", ctrl.handleEncodeTagsFragment)
			channelGroup.POST("/encode/write", ctrl.handleEncodeWrite)
			channelGroup.POST("/encode/clear", ctrl.handleEncodeClear)
			if ctrl.cfg.Inventory.Enabled {
				channelGroup.GET("/inventory", ctrl.handleInventory)
				channelGroup.GET("/inventory/tags", ctrl.handleInventoryTagsFragment)
				channelGroup.POST("/inventory/session", ctrl.handleInventoryStartSession)
				channelGroup.POST("/inventory/marker", ctrl.handleInventorySetMarker)
				channelGroup.POST("/inventory/pause", ctrl.handleInventoryPause)
				channelGroup.POST("/inventory/resume", ctrl.handleInventoryResume)
				channelGroup.POST("/inventory/reset", ctrl.handleInventoryReset)
				channelGroup.POST("/inventory/retry", ctrl.handleInventoryRetry)
			}
			channelGroup.GET("/events", ctrl.handleSSE)
			channelGroup.GET("/loan/events", ctrl.handleSSE)
			channelGroup.GET("/encode/events", ctrl.handleSSE)
			if ctrl.cfg.Inventory.Enabled {
				channelGroup.GET("/inventory/events", ctrl.handleSSE)
			}
			channelGroup.POST("/tags/clear", ctrl.handleClearTags)
		}

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

	if !ctrl.cfg.SOAPOnly {
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
			api.POST("/encode/clear", ctrl.handleAPIEncodeClear)
			if ctrl.cfg.Inventory.Enabled {
				api.GET("/inventory/session", ctrl.handleAPIInventorySession)
				api.POST("/inventory/session", ctrl.handleAPIInventoryStartSession)
				api.POST("/inventory/pause", ctrl.handleAPIInventoryPause)
				api.POST("/inventory/resume", ctrl.handleAPIInventoryResume)
				api.POST("/inventory/reset", ctrl.handleAPIInventoryReset)
				api.POST("/inventory/retry", ctrl.handleAPIInventoryRetry)
			}
		}
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
			serverDesc := "library-desk web server"
			if ctrl.cfg.SOAPOnly {
				serverDesc = "alma-soap proxy server"
			}
			ctrl.logger.Info().Str("addr", ctrl.cfg.Addr).Msg("starting " + serverDesc)
		}
		if err := ctrl.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			if ctrl.logger != nil {
				serverDesc := "library-desk web server"
				if ctrl.cfg.SOAPOnly {
					serverDesc = "alma-soap proxy server"
				}
				ctrl.logger.Error().Err(err).Msg(serverDesc + " encountered an error")
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
