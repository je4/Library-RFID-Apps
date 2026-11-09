package web

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/je4/Library-RFID-Apps/pkg/catalog"
	"github.com/maypok86/otter/v2"
)

// SourceRecordMapping associates institution names and view IDs with a network.
type SourceRecordMapping struct {
	Network      string            `toml:"network" json:"network"`
	Institutions map[string]string `toml:"institutions" json:"institutions"`
}

type catalogLoader struct {
	search func(context.Context, string) ([]catalog.Book, error)
}

func (l catalogLoader) Load(ctx context.Context, barcode string) ([]catalog.Book, error) {
	return l.search(ctx, barcode)
}

func (l catalogLoader) Reload(ctx context.Context, barcode string, _ []catalog.Book) ([]catalog.Book, error) {
	return l.Load(ctx, barcode)
}

type catalogService struct {
	cache  *otter.Cache[string, []catalog.Book]
	loader catalogLoader
}

func newCatalogService(search func(context.Context, string) ([]catalog.Book, error)) *catalogService {
	return &catalogService{
		cache: otter.Must(&otter.Options[string, []catalog.Book]{
			MaximumSize:      10000,
			ExpiryCalculator: otter.ExpiryWriting[string, []catalog.Book](15 * time.Minute),
		}),
		loader: catalogLoader{search: search},
	}
}

func (ctrl *Controller) handleCatalog(c *gin.Context) {
	if ctrl.catalogService == nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	barcode := strings.TrimSpace(c.Query("barcode"))
	if barcode == "" || len(barcode) > 256 {
		c.Status(http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	books, err := ctrl.catalogService.cache.Get(ctx, barcode, ctrl.catalogService.loader)
	if err != nil {
		if ctrl.logger != nil {
			ctrl.logger.Warn().Err(err).Msg("catalog search failed")
		}
		c.Status(http.StatusBadGateway)
		return
	}
	if books == nil {
		books = []catalog.Book{}
	}
	c.JSON(http.StatusOK, gin.H{"books": books, "sourceRecords": ctrl.cfg.SourceRecords})
}
