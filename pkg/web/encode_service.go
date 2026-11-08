package web

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
	"github.com/je4/utils/v2/pkg/zLogger"
)

// WriteTagRequestPayload encapsulates data for writing ISO 28560-2 RFID tag metadata.
type WriteTagRequestPayload struct {
	UID       string `json:"uid" form:"uid"`
	ItemID    string `json:"itemId" form:"itemId"`
	ISIL      string `json:"isil" form:"isil"`
	Country   string `json:"country" form:"country"`
	Parts     int    `json:"parts" form:"parts"`
	PartNo    int    `json:"partNo" form:"partNo"`
	UsageType int    `json:"usageType" form:"usageType"`
	AFI       string `json:"afi" form:"afi"`
	Clear     bool   `json:"clear" form:"clear"`
}

// WriteTagResult contains the outcome of an encoding operation.
type WriteTagResult struct {
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	Error     string `json:"error,omitempty"`
	UID       string `json:"uid"`
	ItemID    string `json:"itemId,omitempty"`
	ISIL      string `json:"isil,omitempty"`
	Country   string `json:"country,omitempty"`
	Parts     int    `json:"parts,omitempty"`
	PartNo    int    `json:"partNo,omitempty"`
	UsageType int    `json:"usageType,omitempty"`
	AFI       string `json:"afi,omitempty"`
}

// EncodeService handles ISO 28560-2 RFID tag encoding, payload validation,
// reader routing, and state synchronization with the Hub.
type EncodeService struct {
	hub         *desk.Hub
	rfidClient  libraryclient.Client
	defaultISIL string
	defaultAFI  string
	channels    []string
	logger      zLogger.ZLogger
}

// NewEncodeService creates a new EncodeService instance.
func NewEncodeService(
	hub *desk.Hub,
	rfidClient libraryclient.Client,
	defaultISIL string,
	defaultAFI string,
	channels []string,
	logger zLogger.ZLogger,
) *EncodeService {
	return &EncodeService{
		hub:         hub,
		rfidClient:  rfidClient,
		defaultISIL: defaultISIL,
		defaultAFI:  defaultAFI,
		channels:    channels,
		logger:      logger,
	}
}

// ValidateAndNormalize sanitizes and checks input parameters for RFID encoding or clearing.
func (s *EncodeService) ValidateAndNormalize(payload *WriteTagRequestPayload) error {
	if payload == nil {
		return fmt.Errorf("payload darf nicht leer sein")
	}

	payload.UID = strings.TrimSpace(payload.UID)

	if payload.Clear {
		payload.ItemID = ""
		payload.ISIL = ""
		payload.Country = ""
		payload.Parts = 1
		payload.PartNo = 1
		payload.UsageType = 1

		afi := strings.TrimSpace(payload.AFI)
		if afi == "" {
			payload.AFI = libraryclient.AFIUnsecured
		} else if strings.EqualFold(afi, "gesichert") || strings.EqualFold(afi, "secure") {
			payload.AFI = libraryclient.AFISecured
		} else if strings.EqualFold(afi, "ungesichert") || strings.EqualFold(afi, "unsecure") {
			payload.AFI = libraryclient.AFIUnsecured
		} else {
			payload.AFI = libraryclient.NormalizeAFI(afi)
		}
		return nil
	}

	payload.ItemID = strings.TrimSpace(payload.ItemID)
	if payload.ItemID == "" {
		return fmt.Errorf("Barcode / ItemID darf nicht leer sein")
	}

	payload.Country = strings.TrimSpace(payload.Country)
	payload.ISIL = strings.TrimSpace(payload.ISIL)

	if payload.Country == "" && payload.ISIL != "" {
		c, i := desk.ParseISIL(payload.ISIL)
		if c != "" {
			payload.Country = c
			payload.ISIL = i
		}
	}
	if payload.Country == "" {
		payload.Country = "CH"
	}

	if payload.Parts <= 0 {
		payload.Parts = 1
	}
	if payload.PartNo <= 0 {
		payload.PartNo = 1
	}
	if payload.PartNo > payload.Parts {
		payload.PartNo = payload.Parts
	}

	if payload.UsageType <= 0 {
		payload.UsageType = 1
	}

	afi := strings.TrimSpace(payload.AFI)
	if afi == "" {
		afi = s.defaultAFI
	}
	if afi == "" {
		afi = libraryclient.AFISecured
	}
	if strings.EqualFold(afi, "gesichert") || strings.EqualFold(afi, "secure") || strings.EqualFold(afi, "07") {
		payload.AFI = libraryclient.AFISecured
	} else if strings.EqualFold(afi, "entsichert") || strings.EqualFold(afi, "unsecure") || strings.EqualFold(afi, "c2") {
		payload.AFI = libraryclient.AFIUnsecured
	} else {
		payload.AFI = libraryclient.NormalizeAFI(afi)
	}

	return nil
}

// ResolveTarget determines the appropriate target daemon source and reader ID for a tag UID.
func (s *EncodeService) ResolveTarget(channel string, uid string) (targetSource string, targetReaderID string, resolvedUID string) {
	resolvedUID = strings.TrimSpace(uid)

	if resolvedUID != "" && s.hub != nil {
		activeTags := s.hub.GetActiveTagsForChannel(channel, "*")
		for _, t := range activeTags {
			if strings.EqualFold(t.UID, resolvedUID) {
				targetSource = t.Source
				targetReaderID = t.ReaderID
				break
			}
		}
		if targetSource == "" && targetReaderID == "" {
			for _, t := range s.hub.GetTags(channel) {
				if strings.EqualFold(t.UID, resolvedUID) {
					targetSource = t.Source
					targetReaderID = t.ReaderID
					break
				}
			}
		}
		if targetSource == "" && targetReaderID == "" {
			for _, t := range s.hub.GetTags("") {
				if strings.EqualFold(t.UID, resolvedUID) {
					targetSource = t.Source
					targetReaderID = t.ReaderID
					break
				}
			}
		}
	} else if resolvedUID == "" && s.hub != nil {
		activeTags := s.hub.GetActiveTagsForChannel(channel, "*")
		if len(activeTags) > 0 {
			resolvedUID = activeTags[0].UID
			targetSource = activeTags[0].Source
			targetReaderID = activeTags[0].ReaderID
		}
	}

	if targetSource == "" {
		targetSource = targetReaderID
	}
	if targetSource == "" {
		targetSource = channel
	}
	if targetSource == "" && len(s.channels) > 0 {
		targetSource = s.channels[0]
	}

	return targetSource, targetReaderID, resolvedUID
}

// ExecuteWriteTag validates the payload, sends the write command to the RFID client,
// and updates the Hub's in-memory tag metadata.
func (s *EncodeService) ExecuteWriteTag(ctx context.Context, channel string, payload WriteTagRequestPayload) (WriteTagResult, int) {
	if err := s.ValidateAndNormalize(&payload); err != nil {
		return WriteTagResult{
			Status: "error",
			Error:  err.Error(),
			UID:    payload.UID,
		}, http.StatusBadRequest
	}

	targetSource, targetReaderID, uid := s.ResolveTarget(channel, payload.UID)

	req := libraryclient.WriteTagRequest{
		ReaderID:  targetReaderID,
		UID:       uid,
		ItemID:    payload.ItemID,
		ISIL:      payload.ISIL,
		Country:   payload.Country,
		Parts:     payload.Parts,
		PartNo:    payload.PartNo,
		UsageType: payload.UsageType,
		AFI:       payload.AFI,
	}

	if s.logger != nil {
		s.logger.Info().
			Str("uid", uid).
			Str("itemId", payload.ItemID).
			Str("isil", payload.ISIL).
			Str("country", payload.Country).
			Int("partNo", payload.PartNo).
			Int("parts", payload.Parts).
			Str("afi", payload.AFI).
			Bool("clear", payload.Clear).
			Str("target", targetSource).
			Str("readerId", targetReaderID).
			Msg("executing EncodeService.WriteTag request")
	}

	if s.rfidClient != nil {
		resp, err := s.rfidClient.WriteTag(ctx, targetSource, req)
		if err != nil {
			if s.logger != nil {
				s.logger.Warn().Err(err).Str("uid", uid).Msg("WriteTag failed with error")
			}
			return WriteTagResult{
				Status: "error",
				Error:  fmt.Sprintf("Fehler beim Schreiben des Transponders %s: %v", uid, err),
				UID:    uid,
			}, http.StatusInternalServerError
		}
		if resp != nil && !resp.Data.IsSuccess() {
			errMsg := resp.Data.GetError()
			if errMsg == "" {
				errMsg = "Schreibfehler auf Transponder"
			}
			if s.logger != nil {
				s.logger.Warn().Str("uid", uid).Str("error", errMsg).Msg("WriteTag returned failure")
			}
			return WriteTagResult{
				Status: "error",
				Error:  errMsg,
				UID:    uid,
			}, http.StatusBadRequest
		}
	}

	// Synchronize with Hub state
	if s.hub != nil {
		if payload.Clear {
			s.hub.ClearTagMetadata(uid, payload.AFI)
		} else {
			s.hub.UpdateTagMetadata(uid, payload.ItemID, payload.ISIL, payload.Country, payload.Parts, payload.PartNo, payload.AFI)
		}
	}

	msg := fmt.Sprintf("Transponder %s erfolgreich beschrieben (Teil %d von %d).", uid, payload.PartNo, payload.Parts)
	if payload.Clear {
		msg = fmt.Sprintf("Transponder %s erfolgreich geleert (Felder gelöscht, Status: Entsichert).", uid)
	}

	return WriteTagResult{
		Status:    "success",
		Message:   msg,
		UID:       uid,
		ItemID:    payload.ItemID,
		ISIL:      payload.ISIL,
		Country:   payload.Country,
		Parts:     payload.Parts,
		PartNo:    payload.PartNo,
		UsageType: payload.UsageType,
		AFI:       payload.AFI,
	}, http.StatusOK
}

// ConvertToGinH converts WriteTagResult to gin.H representation for response serialization.
func (r WriteTagResult) ToGinH() gin.H {
	h := gin.H{
		"status": r.Status,
		"uid":    r.UID,
	}
	if r.Message != "" {
		h["message"] = r.Message
	}
	if r.Error != "" {
		h["error"] = r.Error
	}
	if r.ItemID != "" {
		h["itemId"] = r.ItemID
	}
	if r.ISIL != "" {
		h["isil"] = r.ISIL
	}
	if r.Country != "" {
		h["country"] = r.Country
	}
	if r.Parts > 0 {
		h["parts"] = r.Parts
	}
	if r.PartNo > 0 {
		h["partNo"] = r.PartNo
	}
	if r.UsageType > 0 {
		h["usageType"] = r.UsageType
	}
	if r.AFI != "" {
		h["afi"] = r.AFI
	}
	return h
}
