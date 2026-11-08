package web

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/je4/Library-RFID-Apps/pkg/desk"
)

// InventorySessionRequest represents the payload for starting or updating an inventory session.
type InventorySessionRequest struct {
	Marker string `json:"marker" form:"marker"`
}

// InventoryRetryRequest represents the payload for retrying a failed webservice dispatch.
type InventoryRetryRequest struct {
	UID    string `json:"uid" form:"uid"`
	ItemID string `json:"itemId" form:"itemId"`
}

func (ctrl *Controller) getInventoryData(channel ...string) desk.InventoryData {
	ch := ""
	if len(channel) > 0 {
		ch = channel[0]
	}

	var marker string
	var sessionActive, isPaused bool
	var totalScans, uniqueItems, errorTags, duplicateTags int
	var items []*desk.InventoryItem
	var lastItem *desk.InventoryItem

	if ctrl.inventoryService != nil {
		sess := ctrl.inventoryService.GetSession()
		marker = sess.Marker
		sessionActive = sess.Active
		isPaused = sess.Paused
		totalScans = sess.Stats.TotalScans
		uniqueItems = sess.Stats.UniqueItems
		errorTags = sess.Stats.ErrorTags
		duplicateTags = sess.Stats.DuplicateTags
		items = sess.Items
		if len(items) > 0 {
			lastItem = items[0]
		}
	}

	return desk.InventoryData{
		ClientName:       ctrl.cfg.ClientName,
		WSBroker:         ctrl.cfg.WSBroker,
		Channels:         ctrl.cfg.Channels,
		Channel:          ch,
		Addr:             ctrl.cfg.Addr,
		ExtAddr:          ctrl.cfg.ExtAddr,
		Marker:           marker,
		SessionActive:    sessionActive,
		IsPaused:         isPaused,
		TotalScans:       totalScans,
		UniqueItems:      uniqueItems,
		ErrorTags:        errorTags,
		DuplicateTags:    duplicateTags,
		Items:            items,
		LastItem:         lastItem,
		InventoryConf:    &ctrl.cfg.Inventory,
		Skin:             "default",
		StaticPath:       "../static",
		InventoryEnabled: ctrl.cfg.Inventory.Enabled,
	}
}

func (ctrl *Controller) handleInventory(c *gin.Context) {
	if !ctrl.cfg.Inventory.Enabled || ctrl.inventoryService == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	channel := c.Param("channel")
	data := ctrl.getInventoryData(channel)
	data.Skin = getSkinFromCookie(c)
	ctrl.populateThemeInfo(c, &data.Theme, &data.ThemeMode, &data.ThemeLight, &data.ThemeDark)

	c.HTML(http.StatusOK, "inventory.gohtml", data)
}

func (ctrl *Controller) handleInventoryTagsFragment(c *gin.Context) {
	if !ctrl.cfg.Inventory.Enabled || ctrl.inventoryService == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	channel := c.Param("channel")
	data := ctrl.getInventoryData(channel)

	var buf bytes.Buffer
	if err := ctrl.templates.ExecuteTemplate(&buf, "inventory_table.gohtml", data); err != nil {
		c.String(http.StatusInternalServerError, "error rendering inventory table template: %v", err)
		return
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}

func (ctrl *Controller) handleInventoryStartSession(c *gin.Context) {
	if !ctrl.cfg.Inventory.Enabled || ctrl.inventoryService == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	var req InventorySessionRequest
	_ = c.ShouldBind(&req)

	marker := strings.TrimSpace(req.Marker)
	if marker == "" {
		c.String(http.StatusBadRequest, "Standort / Regal-Marker darf nicht leer sein")
		return
	}

	channel := c.Param("channel")
	sess := ctrl.inventoryService.StartSession(marker)
	ctrl.broadcastInventorySession(sess, channel)

	if c.GetHeader("HX-Request") == "true" {
		ctrl.handleInventoryTagsFragment(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"session": sess,
	})
}

func (ctrl *Controller) handleInventorySetMarker(c *gin.Context) {
	if !ctrl.cfg.Inventory.Enabled || ctrl.inventoryService == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	var req InventorySessionRequest
	_ = c.ShouldBind(&req)

	channel := c.Param("channel")
	marker := strings.TrimSpace(req.Marker)
	ctrl.inventoryService.SetMarker(marker)

	sess := ctrl.inventoryService.GetSession()
	ctrl.broadcastInventorySession(sess, channel)

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"marker": marker,
	})
}

func (ctrl *Controller) handleInventoryPause(c *gin.Context) {
	if !ctrl.cfg.Inventory.Enabled || ctrl.inventoryService == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	channel := c.Param("channel")
	sess := ctrl.inventoryService.PauseSession()
	ctrl.broadcastInventorySession(sess, channel)

	if c.GetHeader("HX-Request") == "true" {
		ctrl.handleInventoryTagsFragment(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"session": sess,
	})
}

func (ctrl *Controller) handleInventoryResume(c *gin.Context) {
	if !ctrl.cfg.Inventory.Enabled || ctrl.inventoryService == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	channel := c.Param("channel")
	sess := ctrl.inventoryService.ResumeSession()
	ctrl.broadcastInventorySession(sess, channel)

	if c.GetHeader("HX-Request") == "true" {
		ctrl.handleInventoryTagsFragment(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"session": sess,
	})
}

func (ctrl *Controller) handleInventoryReset(c *gin.Context) {
	if !ctrl.cfg.Inventory.Enabled || ctrl.inventoryService == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	channel := c.Param("channel")
	sess := ctrl.inventoryService.ResetSession()
	ctrl.broadcastInventorySession(sess, channel)

	if c.GetHeader("HX-Request") == "true" {
		ctrl.handleInventoryTagsFragment(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"session": sess,
	})
}

func (ctrl *Controller) handleInventoryRetry(c *gin.Context) {
	if !ctrl.cfg.Inventory.Enabled || ctrl.inventoryService == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	var req InventoryRetryRequest
	_ = c.ShouldBind(&req)

	targetID := req.UID
	if targetID == "" {
		targetID = req.ItemID
	}
	if targetID == "" {
		targetID = c.Query("uid")
	}
	if targetID == "" {
		targetID = c.Query("itemId")
	}

	if targetID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uid or itemId parameter required"})
		return
	}

	item, err := ctrl.inventoryService.RetryDispatch(targetID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"item":   item,
	})
}

// REST API v1 handlers
func (ctrl *Controller) handleAPIInventorySession(c *gin.Context) {
	if !ctrl.cfg.Inventory.Enabled || ctrl.inventoryService == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "inventory module is disabled"})
		return
	}

	sess := ctrl.inventoryService.GetSession()
	c.JSON(http.StatusOK, sess)
}

func (ctrl *Controller) handleAPIInventoryStartSession(c *gin.Context) {
	ctrl.handleInventoryStartSession(c)
}

func (ctrl *Controller) handleAPIInventoryPause(c *gin.Context) {
	ctrl.handleInventoryPause(c)
}

func (ctrl *Controller) handleAPIInventoryResume(c *gin.Context) {
	ctrl.handleInventoryResume(c)
}

func (ctrl *Controller) handleAPIInventoryReset(c *gin.Context) {
	ctrl.handleInventoryReset(c)
}

func (ctrl *Controller) handleAPIInventoryRetry(c *gin.Context) {
	ctrl.handleInventoryRetry(c)
}
