package web

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/Library-RFID-Apps/pkg/rfidclient"
)

// mockSOAPRfidClient implements rfidclient.Client for testing SOAP Write operations.
type mockSOAPRfidClient struct {
	rfidclient.Client
	lastWriteAFIReq *rfidclient.WriteAFIRequest
	lastWriteTagReq *rfidclient.WriteTagRequest
}

func (m *mockSOAPRfidClient) WriteAFI(ctx context.Context, target string, req rfidclient.WriteAFIRequest) (*rfidclient.WriteAFIResponse, error) {
	m.lastWriteAFIReq = &req
	return &rfidclient.WriteAFIResponse{
		Status:         "ok",
		UID:            req.UID,
		SuccessfulUIDs: req.UIDs,
		AFI:            req.AFI,
	}, nil
}

func (m *mockSOAPRfidClient) BroadcastAFI(ctx context.Context, channel string, req rfidclient.WriteAFIRequest) error {
	m.lastWriteAFIReq = &req
	return nil
}

func (m *mockSOAPRfidClient) SendAFI(ctx context.Context, target string, req rfidclient.WriteAFIRequest) error {
	m.lastWriteAFIReq = &req
	return nil
}

func (m *mockSOAPRfidClient) WriteTag(ctx context.Context, target string, req rfidclient.WriteTagRequest) (*rfidclient.WriteTagResponse, error) {
	m.lastWriteTagReq = &req
	return &rfidclient.WriteTagResponse{
		Status: "ok",
		UID:    req.UID,
		ItemID: req.ItemID,
		AFI:    req.AFI,
	}, nil
}

func setupTestSOAPController(t *testing.T) (*Controller, *desk.Hub, *mockSOAPRfidClient) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	hub := desk.NewHub(50)
	hub.Start(ctx)
	t.Cleanup(hub.Close)

	mockClient := &mockSOAPRfidClient{}

	ctrl, err := NewController(Config{
		Addr:          "localhost:8762",
		ExtAddr:       "http://localhost:8762",
		ClientName:    "test-rfid-desk",
		WSBroker:      "https://localhost:8765",
		Channels:      []string{"all", "intern", "rfid"},
		DefaultReader: "reader-01",
		Hub:           hub,
		RFIDClient:    mockClient,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	return ctrl, hub, mockClient
}

func TestSOAP_WSDLAndStatus(t *testing.T) {
	ctrl, _, _ := setupTestSOAPController(t)

	// 1. Test GET /TagService?wsdl
	reqWSDL := httptest.NewRequest(http.MethodGet, "/TagService?wsdl", nil)
	recWSDL := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recWSDL, reqWSDL)

	if recWSDL.Code != http.StatusOK {
		t.Errorf("expected 200 for WSDL, got %d", recWSDL.Code)
	}
	bodyWSDL := recWSDL.Body.String()
	if !strings.Contains(bodyWSDL, "wsdl:definitions") || !strings.Contains(bodyWSDL, "GetItems") || !strings.Contains(bodyWSDL, "SetTagSecurity") {
		t.Errorf("expected valid WSDL containing GetItems and SetTagSecurity, got: %s", bodyWSDL)
	}

	// 2. Test GET /TagService status string
	reqStatus := httptest.NewRequest(http.MethodGet, "/TagService", nil)
	recStatus := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recStatus, reqStatus)

	if recStatus.Code != http.StatusOK {
		t.Errorf("expected 200 for TagService status, got %d", recStatus.Code)
	}
	if !strings.Contains(recStatus.Body.String(), "Alma RFID TagService (SOAP) aktiv") {
		t.Errorf("expected status message, got: %s", recStatus.Body.String())
	}

	// 3. Test CORS OPTIONS
	reqOpt := httptest.NewRequest(http.MethodOptions, "/TagService", nil)
	reqOpt.Header.Set("Origin", "https://alma.exlibrisgroup.com")
	reqOpt.Header.Set("Access-Control-Request-Method", "POST")
	reqOpt.Header.Set("Access-Control-Request-Headers", "SOAPAction, Content-Type")
	recOpt := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recOpt, reqOpt)

	if recOpt.Code != http.StatusOK && recOpt.Code != http.StatusNoContent {
		t.Errorf("expected 200 or 204 for OPTIONS preflight, got %d", recOpt.Code)
	}
	if recOpt.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected Access-Control-Allow-Origin: *, got %s", recOpt.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestSOAP_ConnectReader_And_IsOnline_And_Ping(t *testing.T) {
	ctrl, _, _ := setupTestSOAPController(t)

	// 1. ConnectReader
	soapConnect := `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:tem="http://tempuri.org/"><soapenv:Header/><soapenv:Body><tem:ConnectReader/></soapenv:Body></soapenv:Envelope>`
	req1 := httptest.NewRequest(http.MethodPost, "/TagService", bytes.NewBufferString(soapConnect))
	req1.Header.Set("SOAPAction", "http://tempuri.org/ICoreTagService/ConnectReader")
	rec1 := httptest.NewRecorder()
	ctrl.router.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 for ConnectReader, got %d", rec1.Code)
	}
	if !strings.Contains(rec1.Body.String(), "<ConnectReaderResult>true</ConnectReaderResult>") {
		t.Errorf("expected ConnectReaderResult true, got: %s", rec1.Body.String())
	}

	// 2. IsOnline
	soapIsOnline := `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:tem="http://tempuri.org/"><soapenv:Header/><soapenv:Body><tem:IsOnline/></soapenv:Body></soapenv:Envelope>`
	req2 := httptest.NewRequest(http.MethodPost, "/TagService", bytes.NewBufferString(soapIsOnline))
	req2.Header.Set("SOAPAction", "http://tempuri.org/ICoreTagService/IsOnline")
	rec2 := httptest.NewRecorder()
	ctrl.router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 for IsOnline, got %d", rec2.Code)
	}
	if !strings.Contains(rec2.Body.String(), "<IsOnlineResult>Online</IsOnlineResult>") {
		t.Errorf("expected IsOnlineResult Online, got: %s", rec2.Body.String())
	}

	// 3. Ping
	soapPing := `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"><soapenv:Body><Ping/></soapenv:Body></soapenv:Envelope>`
	req3 := httptest.NewRequest(http.MethodPost, "/TagService", bytes.NewBufferString(soapPing))
	rec3 := httptest.NewRecorder()
	ctrl.router.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusOK {
		t.Fatalf("expected 200 for Ping, got %d", rec3.Code)
	}
	if !strings.Contains(rec3.Body.String(), "<PingResult>OK</PingResult>") {
		t.Errorf("expected PingResult OK, got: %s", rec3.Body.String())
	}
}

func TestSOAP_GetItems_Alma3M896(t *testing.T) {
	ctrl, hub, _ := setupTestSOAPController(t)

	// Add active secured and unsecured tags to Hub
	hub.AddTag(&rfidclient.TagReadEvent{
		UID:      "E004015099887766",
		ItemID:   "ALMA-3M-ITEM-01",
		AFI:      rfidclient.AFISecured,
		ClientID: "reader-01",
		Channel:  "rfid",
	})
	hub.AddTag(&rfidclient.TagReadEvent{
		UID:      "E004015099887767",
		ItemID:   "ALMA-3M-ITEM-02",
		AFI:      rfidclient.AFIUnsecured,
		ClientID: "reader-01",
		Channel:  "rfid",
	})

	// Exact SOAP request sent by Alma 3M896 popup
	soapAlmaGetItems := `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:tem="http://tempuri.org/">   <soapenv:Header/>   <soapenv:Body>      <tem:GetItems/>   </soapenv:Body></soapenv:Envelope>`

	req := httptest.NewRequest(http.MethodPost, "/TagService", bytes.NewBufferString(soapAlmaGetItems))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", "http://tempuri.org/ITagService/GetItems")
	rec := httptest.NewRecorder()
	ctrl.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for GetItems, got %d, body: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "<GetItemsResponse") {
		t.Errorf("expected GetItemsResponse in output, got: %s", body)
	}
	if !strings.Contains(body, "<a:Id>ALMA-3M-ITEM-01</a:Id>") {
		t.Errorf("expected item barcode ALMA-3M-ITEM-01, got: %s", body)
	}
	if !strings.Contains(body, "<a:IsSecured>Yes</a:IsSecured>") {
		t.Errorf("expected IsSecured Yes for protected item, got: %s", body)
	}
	if !strings.Contains(body, "<a:IsSecured>No</a:IsSecured>") {
		t.Errorf("expected IsSecured No for unprotected item, got: %s", body)
	}
	if !strings.Contains(body, "<Type>Item</Type>") {
		t.Errorf("expected <Type>Item</Type>, got: %s", body)
	}
	if !strings.Contains(body, "<a:Tag><a:Id>E004015099887766</a:Id>") {
		t.Errorf("expected <a:Tag><a:Id>E004015099887766</a:Id>, got: %s", body)
	}
	if !strings.Contains(body, "<a:TagField><a:Name>BranchID</a:Name>") {
		t.Errorf("expected <a:TagField> BranchID, got: %s", body)
	}
}

func TestSOAP_SetTagSecurity_Alma3M896(t *testing.T) {
	ctrl, hub, mockClient := setupTestSOAPController(t)

	hub.AddTag(&rfidclient.TagReadEvent{
		UID:      "E004015099887766",
		ItemID:   "ALMA-3M-ITEM-01",
		AFI:      rfidclient.AFISecured,
		ClientID: "reader-01",
		Channel:  "rfid",
	})

	// Alma SetTagSecurity to unlock (unsecure / on loan)
	soapSetSec := `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:tem="http://tempuri.org/">
		<soapenv:Header/>
		<soapenv:Body>
			<tem:SetTagSecurity>
				<tem:tagId>E004015099887766</tem:tagId>
				<tem:isSecured>false</tem:isSecured>
			</tem:SetTagSecurity>
		</soapenv:Body>
	</soapenv:Envelope>`

	req := httptest.NewRequest(http.MethodPost, "/TagService", bytes.NewBufferString(soapSetSec))
	req.Header.Set("SOAPAction", "http://tempuri.org/ICoreTagService/SetTagSecurity")
	rec := httptest.NewRecorder()
	ctrl.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for SetTagSecurity, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "<SetTagSecurityResult>true</SetTagSecurityResult>") {
		t.Errorf("expected SetTagSecurityResult true, got: %s", body)
	}

	if mockClient.lastWriteAFIReq == nil || mockClient.lastWriteAFIReq.AFI != rfidclient.AFIUnsecured || mockClient.lastWriteAFIReq.UID != "E004015099887766" {
		t.Errorf("expected WriteAFI on UID E004015099887766 with AFI %s, got: %+v", rfidclient.AFIUnsecured, mockClient.lastWriteAFIReq)
	}
}

func TestSOAP_WriteTag_Alma3M896(t *testing.T) {
	ctrl, hub, mockClient := setupTestSOAPController(t)

	hub.AddTag(&rfidclient.TagReadEvent{
		UID:      "E004015012345678",
		ItemID:   "OLD-BARCODE",
		AFI:      rfidclient.AFISecured,
		ClientID: "reader-01",
		Channel:  "rfid",
	})

	soapWriteTag3M := `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
		<s:Body>
			<WriteTag xmlns="http://tempuri.org/">
				<tag xmlns:a="http://schemas.datacontract.org/2004/07/MMM.Arbiter.ServiceModel" xmlns:i="http://www.w3.org/2001/XMLSchema-instance">
					<a:Fields xmlns:b="http://schemas.microsoft.com/2003/10/Serialization/Arrays">
						<a:TagField><a:Name>ItemID</a:Name><a:Value>NEW-BARCODE-99</a:Value></a:TagField>
						<a:TagField><a:Name>BranchID</a:Name><a:Value>1</a:Value></a:TagField>
					</a:Fields>
					<a:Id>E004015012345678</a:Id>
					<a:IsTagSecured>true</a:IsTagSecured>
					<a:TagFormat>ISO 28560-2</a:TagFormat>
					<a:Type>Item</a:Type>
				</tag>
			</WriteTag>
		</s:Body>
	</s:Envelope>`

	req := httptest.NewRequest(http.MethodPost, "/TagService", bytes.NewBufferString(soapWriteTag3M))
	req.Header.Set("SOAPAction", "http://tempuri.org/ITagService/WriteTag")
	rec := httptest.NewRecorder()
	ctrl.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for WriteTag, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "<WriteTagResult>true</WriteTagResult>") {
		t.Errorf("expected WriteTagResult true, got: %s", body)
	}

	if mockClient.lastWriteTagReq == nil || mockClient.lastWriteTagReq.ItemID != "NEW-BARCODE-99" || mockClient.lastWriteTagReq.UID != "E004015012345678" {
		t.Errorf("expected WriteTag on UID E004015012345678 with ItemID NEW-BARCODE-99, got: %+v", mockClient.lastWriteTagReq)
	}
}

func TestSOAP_GetTag_Alma(t *testing.T) {
	ctrl, hub, _ := setupTestSOAPController(t)

	hub.AddTag(&rfidclient.TagReadEvent{
		UID:      "E004015011223344",
		ItemID:   "ALMA-BOOK-12345",
		AFI:      rfidclient.AFISecured,
		ClientID: "reader-01",
		Channel:  "rfid",
	})

	soapReq := `<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/" xmlns:tem="http://tempuri.org/">
  <soap:Body>
    <tem:GetTag />
  </soap:Body>
</soap:Envelope>`

	req := httptest.NewRequest(http.MethodPost, "/TagService", bytes.NewBufferString(soapReq))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	rec := httptest.NewRecorder()
	ctrl.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for GetTag, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "<Success>true</Success>") || !strings.Contains(body, "<ItemIdentifier>ALMA-BOOK-12345</ItemIdentifier>") || !strings.Contains(body, "<SecurityStatus>Protected</SecurityStatus>") {
		t.Errorf("expected GetTagResponse with Success true and ItemIdentifier ALMA-BOOK-12345, got: %s", body)
	}
}

func TestSOAP_ChannelScopedEndpoint(t *testing.T) {
	ctrl, hub, _ := setupTestSOAPController(t)

	// Add tag to channel "rfid"
	hub.AddTag(&rfidclient.TagReadEvent{
		UID:      "E004015099887766",
		ItemID:   "ITEM-RFID-CHANNEL",
		AFI:      rfidclient.AFISecured,
		ClientID: "reader-01",
		Channel:  "rfid",
	})

	// Query /rfid/TagService
	soapReq := `<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/" xmlns:tem="http://tempuri.org/">
  <soap:Body>
    <tem:GetTag />
  </soap:Body>
</soap:Envelope>`

	reqRFID := httptest.NewRequest(http.MethodPost, "/rfid/TagService", bytes.NewBufferString(soapReq))
	reqRFID.Header.Set("Content-Type", "text/xml; charset=utf-8")
	recRFID := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recRFID, reqRFID)

	if recRFID.Code != http.StatusOK {
		t.Fatalf("expected 200 for /rfid/TagService, got %d", recRFID.Code)
	}
	if !strings.Contains(recRFID.Body.String(), "ITEM-RFID-CHANNEL") {
		t.Errorf("expected ITEM-RFID-CHANNEL in /rfid/TagService, got: %s", recRFID.Body.String())
	}

	// Query /intern/TagService (should find no active tags for channel intern)
	reqIntern := httptest.NewRequest(http.MethodPost, "/intern/TagService", bytes.NewBufferString(soapReq))
	reqIntern.Header.Set("Content-Type", "text/xml; charset=utf-8")
	recIntern := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recIntern, reqIntern)

	if recIntern.Code != http.StatusOK {
		t.Fatalf("expected 200 for /intern/TagService, got %d", recIntern.Code)
	}
	if !strings.Contains(recIntern.Body.String(), "<Success>false</Success>") {
		t.Errorf("expected Success false for channel with no tags, got: %s", recIntern.Body.String())
	}
}
