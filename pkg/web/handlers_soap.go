package web

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/je4/Library-RFID-Apps/pkg/alma"
	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/Library-RFID-Apps/pkg/rfidclient"
)

// sendSOAPResponse serializes and sends a SOAP envelope response with appropriate headers.
func sendSOAPResponse(c *gin.Context, payload any) {
	envelope := alma.NewSOAPEnvelopeResponse(payload)

	c.Header("Content-Type", "text/xml; charset=utf-8")
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write([]byte(xml.Header))
	_ = xml.NewEncoder(c.Writer).Encode(envelope)
}

func (ctrl *Controller) handleSOAPTagService(c *gin.Context) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		if ctrl.logger != nil {
			ctrl.logger.Warn().Err(err).Str("clientIP", c.ClientIP()).Msg("Alma SOAP: Fehler beim Lesen des Bodys")
		}
		c.String(http.StatusBadRequest, "Fehler beim Lesen des Bodys")
		return
	}

	soapAction := c.GetHeader("SOAPAction")
	if ctrl.logger != nil {
		ctrl.logger.Info().
			Str("clientIP", c.ClientIP()).
			Str("path", c.Request.URL.Path).
			Str("soapAction", soapAction).
			Str("body", string(bodyBytes)).
			Msg("Alma SOAP Request empfangen")
	}

	var env alma.Envelope
	if err := xml.Unmarshal(bodyBytes, &env); err != nil {
		if ctrl.logger != nil {
			ctrl.logger.Warn().Err(err).Str("body", string(bodyBytes)).Msg("Ungültiges XML/SOAP empfangen")
		}
		c.String(http.StatusBadRequest, "Ungültiges XML/SOAP")
		return
	}

	c.Header("Content-Type", "text/xml; charset=utf-8")

	if alma.IsConnectReader(env.Body.Content) {
		ctrl.handleSOAPConnectReader(c)
	} else if alma.IsIsOnline(env.Body.Content) {
		ctrl.handleSOAPIsOnline(c)
	} else if alma.IsPing(env.Body.Content) {
		ctrl.handleSOAPPing(c)
	} else if alma.IsSetTagSecurity(env.Body.Content) {
		ctrl.handleSOAPSetTagSecurity(c, env.Body.Content)
	} else if alma.IsGetItems(env.Body.Content) {
		ctrl.handleSOAPGetItems(c)
	} else if alma.IsGetTag(env.Body.Content) {
		ctrl.handleSOAPGetTag(c)
	} else if alma.IsWriteItems(env.Body.Content) {
		var writeReq alma.WriteItemsRequest
		if err := xml.Unmarshal(env.Body.Content, &writeReq); err != nil {
			if ctrl.logger != nil {
				ctrl.logger.Warn().Err(err).Msg("Fehler beim Deserialisieren von WriteItemsRequest")
			}
		}
		ctrl.handleSOAPWriteItems(c, &writeReq)
	} else if alma.IsWriteTag(env.Body.Content) {
		ctrl.handleSOAPWriteTag(c, env.Body.Content)
	} else {
		if ctrl.logger != nil {
			ctrl.logger.Warn().
				Str("clientIP", c.ClientIP()).
				Str("soapAction", soapAction).
				Str("innerXML", string(env.Body.Content)).
				Msg("Alma SOAP: Unbekannte SOAP Action empfangen")
		}
		c.String(http.StatusBadRequest, "Unbekannte SOAP Action")
	}
}

func (ctrl *Controller) handleSOAPConnectReader(c *gin.Context) {
	if ctrl.logger != nil {
		ctrl.logger.Info().Msg("Alma SOAP: ConnectReader aufgerufen")
	}
	resp := alma.ConnectReaderResponse{
		ConnectReaderResult: true,
	}
	sendSOAPResponse(c, resp)
}

func (ctrl *Controller) handleSOAPIsOnline(c *gin.Context) {
	if ctrl.logger != nil {
		ctrl.logger.Info().Msg("Alma SOAP: IsOnline aufgerufen -> Online")
	}
	resp := alma.IsOnlineResponse{
		IsOnlineResult: "Online",
	}
	sendSOAPResponse(c, resp)
}

func (ctrl *Controller) handleSOAPPing(c *gin.Context) {
	if ctrl.logger != nil {
		ctrl.logger.Info().Msg("Alma SOAP: Ping aufgerufen -> OK")
	}
	resp := alma.PingResponse{
		PingResult: "OK",
	}
	sendSOAPResponse(c, resp)
}

func (ctrl *Controller) handleSOAPGetItems(c *gin.Context) {
	channel := c.Param("channel")
	if channel == "" {
		channel = c.Query("channel")
	}

	activeTags := ctrl.hub.GetActiveTagsForChannel(channel, ctrl.cfg.DefaultReader)

	var items []alma.ArbiterItem
	for _, tag := range activeTags {
		itemIdentifier := tag.ItemID
		if itemIdentifier == "" {
			itemIdentifier = tag.UID
		}
		isSecuredStr := "No"
		isTagSecuredStr := "false"
		if alma.SecurityStatusToAFI(alma.AFIToSecurityStatus(tag.AFI)) == rfidclient.AFISecured {
			isSecuredStr = "Yes"
			isTagSecuredStr = "true"
		}

		setNumber := "1"
		if tag.PartNo > 0 {
			setNumber = fmt.Sprintf("%d", tag.PartNo)
		}
		setSize := "1"
		if tag.Parts > 0 {
			setSize = fmt.Sprintf("%d", tag.Parts)
		}
		mediaType := "0"
		if tag.UsageType > 0 {
			mediaType = fmt.Sprintf("%d", tag.UsageType)
		}

		fields := []alma.ArbiterTagField{
			{Name: "ItemID", Type: "String", Value: itemIdentifier},
			{Name: "SetNumber", Type: "UnsignedInteger32", Value: setNumber},
			{Name: "SetSize", Type: "UnsignedInteger32", Value: setSize},
			{Name: "MediaType", Type: "UnsignedInteger32", Value: mediaType},
			{Name: "BranchID", Type: "UnsignedInteger32", Value: "1"},
			{Name: "LibraryID", Type: "UnsignedInteger32", Value: "1"},
			{Name: "MagMedia", Type: "UnsignedInteger32", Value: "0"},
			{Name: "CustomData", Type: "UnsignedInteger32", Value: "0"},
		}

		arbiterTag := alma.ArbiterTag{
			ID:           tag.UID,
			IsTagSecured: isTagSecuredStr,
			TagFormat:    "ISO 28560-2",
			Type:         "Item",
		}

		items = append(items, alma.ArbiterItem{
			Fields:        fields,
			ID:            itemIdentifier,
			IsSecured:     isSecuredStr,
			IsValid:       "True",
			TagFormatName: "ISO 28560-2",
			Type:          "Item",
			Tag:           arbiterTag,
		})
	}

	resp := alma.GetItemsResponse{
		GetItemsResult: alma.GetItemsResult{
			XmlnsA: alma.NamespaceArbiter,
			XmlnsI: alma.NamespaceXMLSchemaInst,
			Items:  items,
		},
	}

	if ctrl.logger != nil {
		ctrl.logger.Info().
			Int("count", len(items)).
			Interface("items", items).
			Msg("Alma SOAP GetItems: 3M Arbiter Tags erfolgreich abgefragt")
	}

	sendSOAPResponse(c, resp)
}

func (ctrl *Controller) handleSOAPGetTag(c *gin.Context) {
	channel := c.Param("channel")
	if channel == "" {
		channel = c.Query("channel")
	}

	activeTags := ctrl.hub.GetActiveTagsForChannel(channel, ctrl.cfg.DefaultReader)

	if len(activeTags) > 0 {
		firstTag := activeTags[0]
		itemIdentifier := firstTag.ItemID
		if itemIdentifier == "" {
			itemIdentifier = firstTag.UID
		}

		secStatus := alma.AFIToSecurityStatus(firstTag.AFI)

		resp := alma.GetTagResponse{
			Success:        true,
			ItemIdentifier: itemIdentifier,
			SecurityStatus: secStatus,
		}

		if ctrl.logger != nil {
			ctrl.logger.Info().
				Str("itemIdentifier", itemIdentifier).
				Str("uid", firstTag.UID).
				Str("afi", firstTag.AFI).
				Str("securityStatus", secStatus).
				Msg("Alma SOAP GetTag: Tag erfolgreich abgefragt")
		}

		sendSOAPResponse(c, resp)
		return
	}

	// No active tags on antenna pad
	resp := alma.GetTagResponse{
		Success:        false,
		ItemIdentifier: "",
		SecurityStatus: alma.SecurityStatusUndefined,
	}

	if ctrl.logger != nil {
		ctrl.logger.Info().Msg("Alma SOAP GetTag: Keine aktiven Tags auf dem Reader")
	}

	sendSOAPResponse(c, resp)
}

func (ctrl *Controller) handleSOAPSetTagSecurity(c *gin.Context, rawBody []byte) {
	req := alma.ParseSetTagSecurityRequest(rawBody)
	channel := c.Param("channel")
	if channel == "" {
		channel = c.Query("channel")
	}

	targetAFI := rfidclient.AFIUnsecured
	if req.IsSecured {
		targetAFI = rfidclient.AFISecured
	}

	if ctrl.logger != nil {
		ctrl.logger.Info().
			Str("tagId", req.TagID).
			Str("barcode", req.Barcode).
			Bool("isSecured", req.IsSecured).
			Str("targetAFI", targetAFI).
			Msg("Alma SOAP SetTagSecurity empfangen")
	}

	activeTags := ctrl.hub.GetActiveTagsForChannel(channel, ctrl.cfg.DefaultReader)
	var targetTag *desk.TagItem
	if req.TagID != "" {
		for _, t := range activeTags {
			if strings.EqualFold(t.UID, req.TagID) {
				targetTag = t
				break
			}
		}
	}
	if targetTag == nil && req.Barcode != "" {
		for _, t := range activeTags {
			if strings.EqualFold(t.ItemID, req.Barcode) {
				targetTag = t
				break
			}
		}
	}
	if targetTag == nil && len(activeTags) > 0 {
		targetTag = activeTags[0]
	}

	ctx := c.Request.Context()
	targetReader := ctrl.cfg.DefaultReader
	var targetSource string
	var targetReaderID string
	if targetTag != nil {
		targetSource = targetTag.Source
		targetReaderID = targetTag.ReaderID
	} else if len(activeTags) > 0 {
		targetSource = activeTags[0].Source
		targetReaderID = activeTags[0].ReaderID
	}
	if targetSource == "" {
		targetSource = targetReaderID
	}
	if targetSource == "" {
		targetSource = targetReader
	}
	success := false

	if targetTag != nil {
		targetUID := targetTag.UID
		if ctrl.rfidClient != nil {
			afiReq := rfidclient.NewWriteAFIRequest(targetAFI, targetUID)
			afiReq.ReaderID = targetReaderID
			if targetSource != "" && targetSource != "all" {
				afiResp, err := ctrl.rfidClient.WriteAFI(ctx, targetSource, afiReq)
				if err == nil && afiResp != nil && afiResp.Data.IsSuccess() {
					success = true
					ctrl.hub.UpdateActiveAFI(targetAFI, targetUID)
				} else if ctrl.logger != nil {
					ctrl.logger.Warn().Err(err).Interface("resp", afiResp).Str("target", targetSource).Str("readerId", targetReaderID).Msg("Alma SOAP SetTagSecurity: AFI-Schreiben fehlgeschlagen")
				}
			} else if targetReader != "" {
				afiResp, err := ctrl.rfidClient.WriteAFI(ctx, targetReader, afiReq)
				if err == nil && afiResp != nil && afiResp.Data.IsSuccess() {
					success = true
					ctrl.hub.UpdateActiveAFI(targetAFI, targetUID)
				}
			} else {
				ctrl.hub.UpdateActiveAFI(targetAFI, targetUID)
				success = true
			}
		} else {
			ctrl.hub.UpdateActiveAFI(targetAFI, targetUID)
			success = true
		}
	} else if ctrl.rfidClient != nil {
		targetUID := req.TagID
		var afiReq rfidclient.WriteAFIRequest
		if targetUID != "" {
			afiReq = rfidclient.NewWriteAFIRequest(targetAFI, targetUID)
		} else {
			afiReq = rfidclient.NewWriteAFIRequest(targetAFI)
		}
		afiReq.ReaderID = targetReaderID
		if targetSource != "" && targetSource != "all" {
			afiResp, err := ctrl.rfidClient.WriteAFI(ctx, targetSource, afiReq)
			if err == nil && afiResp != nil && afiResp.Data.IsSuccess() {
				success = true
				if targetUID != "" {
					ctrl.hub.UpdateActiveAFI(targetAFI, targetUID)
				} else {
					ctrl.hub.UpdateActiveAFI(targetAFI)
				}
			}
		} else if targetReader != "" {
			afiResp, err := ctrl.rfidClient.WriteAFI(ctx, targetReader, afiReq)
			if err == nil && afiResp != nil && afiResp.Data.IsSuccess() {
				success = true
				if targetUID != "" {
					ctrl.hub.UpdateActiveAFI(targetAFI, targetUID)
				} else {
					ctrl.hub.UpdateActiveAFI(targetAFI)
				}
			}
		} else {
			if targetUID != "" {
				ctrl.hub.UpdateActiveAFI(targetAFI, targetUID)
			} else {
				ctrl.hub.UpdateActiveAFI(targetAFI)
			}
			success = true
		}
	} else {
		success = true
	}

	ctrl.broadcastLoanTable(channel)

	resp := alma.SetTagSecurityResponse{
		SetTagSecurityResult: success,
	}
	sendSOAPResponse(c, resp)
}

func (ctrl *Controller) handleSOAPWriteItems(c *gin.Context, req *alma.WriteItemsRequest) {
	success := ctrl.executeSOAPWrite(c, req.ItemIdentifier, req.SecurityStatus, "WriteItems")
	resp := alma.WriteItemsResponse{
		WriteItemsResult: success,
	}
	sendSOAPResponse(c, resp)
}

func (ctrl *Controller) handleSOAPWriteTag(c *gin.Context, rawBody []byte) {
	req := alma.ParseWriteTagRequest(rawBody)
	channel := c.Param("channel")
	if channel == "" {
		channel = c.Query("channel")
	}

	itemIdentifier := strings.TrimSpace(req.ItemIdentifier)
	targetAFI := alma.SecurityStatusToAFI(req.IsSecured)

	if ctrl.logger != nil {
		ctrl.logger.Info().
			Str("tagUID", req.TagUID).
			Str("itemIdentifier", itemIdentifier).
			Str("isSecured", req.IsSecured).
			Str("targetAFI", targetAFI).
			Msg("Alma SOAP WriteTag empfangen")
	}

	activeTags := ctrl.hub.GetActiveTagsForChannel(channel, ctrl.cfg.DefaultReader)
	var targetTag *desk.TagItem
	if req.TagUID != "" {
		for _, t := range activeTags {
			if strings.EqualFold(t.UID, req.TagUID) {
				targetTag = t
				break
			}
		}
	}
	if targetTag == nil && itemIdentifier != "" {
		for _, t := range activeTags {
			if strings.EqualFold(t.ItemID, itemIdentifier) || strings.EqualFold(t.UID, itemIdentifier) {
				targetTag = t
				break
			}
		}
	}
	if targetTag == nil && len(activeTags) > 0 {
		targetTag = activeTags[0]
	}

	ctx := c.Request.Context()
	targetReader := ctrl.cfg.DefaultReader
	var targetSource string
	var targetReaderID string
	if targetTag != nil {
		targetSource = targetTag.Source
		targetReaderID = targetTag.ReaderID
	} else if len(activeTags) > 0 {
		targetSource = activeTags[0].Source
		targetReaderID = activeTags[0].ReaderID
	}
	if targetSource == "" {
		targetSource = targetReaderID
	}
	if targetSource == "" {
		targetSource = targetReader
	}
	success := false

	if targetTag != nil {
		targetUID := targetTag.UID
		if itemIdentifier != "" && !strings.EqualFold(targetTag.ItemID, itemIdentifier) {
			if ctrl.rfidClient != nil {
				writeReq := rfidclient.NewWriteTagRequest(itemIdentifier, targetUID)
				writeReq.ReaderID = targetReaderID
				if targetAFI != "" {
					writeReq.AFI = targetAFI
				}
				if targetSource != "" && targetSource != "all" {
					writeResp, err := ctrl.rfidClient.WriteTag(ctx, targetSource, writeReq)
					if err == nil && writeResp != nil && writeResp.Data.IsSuccess() {
						success = true
						targetTag.ItemID = itemIdentifier
						if targetAFI != "" {
							ctrl.hub.UpdateActiveAFI(targetAFI, targetUID)
						}
					} else if ctrl.logger != nil {
						ctrl.logger.Warn().Err(err).Interface("resp", writeResp).Str("target", targetSource).Str("readerId", targetReaderID).Msg("Alma SOAP WriteTag: Tag-Schreiben fehlgeschlagen")
					}
				} else {
					targetTag.ItemID = itemIdentifier
					if targetAFI != "" {
						ctrl.hub.UpdateActiveAFI(targetAFI, targetUID)
					}
					success = true
				}
			} else {
				targetTag.ItemID = itemIdentifier
				if targetAFI != "" {
					ctrl.hub.UpdateActiveAFI(targetAFI, targetUID)
				}
				success = true
			}
		} else if targetAFI != "" {
			if ctrl.rfidClient != nil {
				afiReq := rfidclient.NewWriteAFIRequest(targetAFI, targetUID)
				afiReq.ReaderID = targetReaderID
				if targetSource != "" && targetSource != "all" {
					afiResp, err := ctrl.rfidClient.WriteAFI(ctx, targetSource, afiReq)
					if err == nil && afiResp != nil && afiResp.Data.IsSuccess() {
						success = true
						ctrl.hub.UpdateActiveAFI(targetAFI, targetUID)
					} else if ctrl.logger != nil {
						ctrl.logger.Warn().Err(err).Interface("resp", afiResp).Str("target", targetSource).Str("readerId", targetReaderID).Msg("Alma SOAP WriteTag: AFI-Schreiben fehlgeschlagen")
					}
				} else if targetReader != "" {
					afiResp, err := ctrl.rfidClient.WriteAFI(ctx, targetReader, afiReq)
					if err == nil && afiResp != nil && afiResp.Data.IsSuccess() {
						success = true
						ctrl.hub.UpdateActiveAFI(targetAFI, targetUID)
					}
				} else {
					ctrl.hub.UpdateActiveAFI(targetAFI, targetUID)
					success = true
				}
			} else {
				ctrl.hub.UpdateActiveAFI(targetAFI, targetUID)
				success = true
			}
		} else {
			success = true
		}
	} else if ctrl.rfidClient != nil {
		if itemIdentifier != "" {
			writeReq := rfidclient.NewWriteTagRequest(itemIdentifier, req.TagUID)
			writeReq.ReaderID = targetReaderID
			if targetAFI != "" {
				writeReq.AFI = targetAFI
			}
			if targetSource != "" && targetSource != "all" {
				writeResp, err := ctrl.rfidClient.WriteTag(ctx, targetSource, writeReq)
				if err == nil && writeResp != nil && writeResp.Data.IsSuccess() {
					success = true
				}
			} else if targetReader != "" {
				writeResp, err := ctrl.rfidClient.WriteTag(ctx, targetReader, writeReq)
				if err == nil && writeResp != nil && writeResp.Data.IsSuccess() {
					success = true
				}
			} else {
				success = true
			}
		} else if targetAFI != "" {
			afiReq := rfidclient.NewWriteAFIRequest(targetAFI, req.TagUID)
			afiReq.ReaderID = targetReaderID
			if targetSource != "" && targetSource != "all" {
				afiResp, err := ctrl.rfidClient.WriteAFI(ctx, targetSource, afiReq)
				if err == nil && afiResp != nil && afiResp.Data.IsSuccess() {
					success = true
				}
			} else if targetReader != "" {
				afiResp, err := ctrl.rfidClient.WriteAFI(ctx, targetReader, afiReq)
				if err == nil && afiResp != nil && afiResp.Data.IsSuccess() {
					success = true
				}
			} else {
				success = true
			}
		}
	} else {
		success = true
	}

	ctrl.broadcastLoanTable(channel)

	resp := alma.WriteTagResponse{
		WriteTagResult: success,
	}
	sendSOAPResponse(c, resp)
}

func (ctrl *Controller) executeSOAPWrite(c *gin.Context, reqItemIdentifier, reqSecurityStatus, actionName string) bool {
	channel := c.Param("channel")
	if channel == "" {
		channel = c.Query("channel")
	}

	targetAFI := alma.SecurityStatusToAFI(reqSecurityStatus)
	itemIdentifier := strings.TrimSpace(reqItemIdentifier)

	if ctrl.logger != nil {
		ctrl.logger.Info().
			Str("action", actionName).
			Str("itemIdentifier", itemIdentifier).
			Str("securityStatus", reqSecurityStatus).
			Str("targetAFI", targetAFI).
			Msg("Alma SOAP Write Request empfangen")
	}

	activeTags := ctrl.hub.GetActiveTagsForChannel(channel, ctrl.cfg.DefaultReader)

	var targetTag *desk.TagItem
	if itemIdentifier != "" {
		for _, t := range activeTags {
			if strings.EqualFold(t.ItemID, itemIdentifier) || strings.EqualFold(t.UID, itemIdentifier) {
				targetTag = t
				break
			}
		}
	}
	if targetTag == nil && len(activeTags) > 0 {
		targetTag = activeTags[0]
	}

	ctx := c.Request.Context()
	targetReader := ctrl.cfg.DefaultReader
	var targetSource string
	var targetReaderID string
	if targetTag != nil {
		targetSource = targetTag.Source
		targetReaderID = targetTag.ReaderID
	} else if len(activeTags) > 0 {
		targetSource = activeTags[0].Source
		targetReaderID = activeTags[0].ReaderID
	}
	if targetSource == "" {
		targetSource = targetReaderID
	}
	if targetSource == "" {
		targetSource = targetReader
	}
	success := false

	if targetTag != nil {
		targetUID := targetTag.UID

		// Case 1: ItemIdentifier is provided and differs from current tag ItemID -> full write tag
		if itemIdentifier != "" && !strings.EqualFold(targetTag.ItemID, itemIdentifier) {
			if ctrl.rfidClient != nil {
				writeReq := rfidclient.NewWriteTagRequest(itemIdentifier, targetUID)
				writeReq.ReaderID = targetReaderID
				if targetAFI != "" {
					writeReq.AFI = targetAFI
				}
				if targetSource != "" && targetSource != "all" {
					writeResp, err := ctrl.rfidClient.WriteTag(ctx, targetSource, writeReq)
					if err == nil && writeResp != nil && writeResp.Data.IsSuccess() {
						success = true
						targetTag.ItemID = itemIdentifier
						if targetAFI != "" {
							ctrl.hub.UpdateActiveAFI(targetAFI, targetUID)
						}
					} else if ctrl.logger != nil {
						ctrl.logger.Warn().Err(err).Interface("resp", writeResp).Str("target", targetSource).Str("readerId", targetReaderID).Msgf("Alma SOAP %s: Tag-Schreiben fehlgeschlagen", actionName)
					}
				} else {
					targetTag.ItemID = itemIdentifier
					if targetAFI != "" {
						ctrl.hub.UpdateActiveAFI(targetAFI, targetUID)
					}
					success = true
				}
			} else {
				targetTag.ItemID = itemIdentifier
				if targetAFI != "" {
					ctrl.hub.UpdateActiveAFI(targetAFI, targetUID)
				}
				success = true
			}
		} else if targetAFI != "" {
			// Case 2: Only AFI / Security status toggle
			if ctrl.rfidClient != nil {
				afiReq := rfidclient.NewWriteAFIRequest(targetAFI, targetUID)
				afiReq.ReaderID = targetReaderID
				if targetSource != "" && targetSource != "all" {
					afiResp, err := ctrl.rfidClient.WriteAFI(ctx, targetSource, afiReq)
					if err == nil && afiResp != nil && afiResp.Data.IsSuccess() {
						success = true
						ctrl.hub.UpdateActiveAFI(targetAFI, targetUID)
					} else if ctrl.logger != nil {
						ctrl.logger.Warn().Err(err).Interface("resp", afiResp).Str("target", targetSource).Str("readerId", targetReaderID).Msgf("Alma SOAP %s: AFI-Schreiben fehlgeschlagen", actionName)
					}
				} else if targetReader != "" {
					afiResp, err := ctrl.rfidClient.WriteAFI(ctx, targetReader, afiReq)
					if err == nil && afiResp != nil && afiResp.Data.IsSuccess() {
						success = true
						ctrl.hub.UpdateActiveAFI(targetAFI, targetUID)
					}
				} else {
					ctrl.hub.UpdateActiveAFI(targetAFI, targetUID)
					success = true
				}
			} else {
				ctrl.hub.UpdateActiveAFI(targetAFI, targetUID)
				success = true
			}
		} else {
			success = true
		}
	} else if ctrl.rfidClient != nil && targetAFI != "" {
		// Blind write to reader field when tag is not currently in memory
		afiReq := rfidclient.NewWriteAFIRequest(targetAFI)
		afiReq.ReaderID = targetReaderID
		if targetSource != "" && targetSource != "all" {
			afiResp, err := ctrl.rfidClient.WriteAFI(ctx, targetSource, afiReq)
			if err == nil && afiResp != nil && afiResp.Data.IsSuccess() {
				success = true
				ctrl.hub.UpdateActiveAFI(targetAFI)
			}
		} else if targetReader != "" {
			afiResp, err := ctrl.rfidClient.WriteAFI(ctx, targetReader, afiReq)
			if err == nil && afiResp != nil && afiResp.Data.IsSuccess() {
				success = true
				ctrl.hub.UpdateActiveAFI(targetAFI)
			}
		} else {
			ctrl.hub.UpdateActiveAFI(targetAFI)
			success = true
		}
	}

	ctrl.broadcastLoanTable(channel)
	return success
}

func (ctrl *Controller) handleSOAPWSDL(c *gin.Context) {
	_, hasWSDL := c.GetQuery("wsdl")
	_, hasWSDLUcase := c.GetQuery("WSDL")
	if hasWSDL || hasWSDLUcase || c.Query("wsdl") != "" || c.Query("WSDL") != "" || strings.HasSuffix(c.Request.URL.Path, ".wsdl") {
		c.Header("Content-Type", "text/xml; charset=utf-8")
		c.String(http.StatusOK, alma.TagServiceWSDL)
		return
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, "Alma RFID TagService (SOAP) aktiv")
}
