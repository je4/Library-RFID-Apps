package alma

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/je4/Library-RFID-Apps/pkg/rfidclient"
)

func TestSecurityStatusConversion(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{"Protected", rfidclient.AFISecured},
		{"protected", rfidclient.AFISecured},
		{"Secured", rfidclient.AFISecured},
		{"Gesichert", rfidclient.AFISecured},
		{"07", rfidclient.AFISecured},
		{"true", rfidclient.AFISecured},
		{"yes", rfidclient.AFISecured},
		{"Unprotected", rfidclient.AFIUnsecured},
		{"unprotected", rfidclient.AFIUnsecured},
		{"Unsecured", rfidclient.AFIUnsecured},
		{"Entsichert", rfidclient.AFIUnsecured},
		{"C2", rfidclient.AFIUnsecured},
		{"false", rfidclient.AFIUnsecured},
		{"no", rfidclient.AFIUnsecured},
		{"unknown", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := SecurityStatusToAFI(tt.status)
		if got != tt.expected {
			t.Errorf("SecurityStatusToAFI(%q) = %q, expected %q", tt.status, got, tt.expected)
		}
	}

	afiTests := []struct {
		afi      string
		expected string
	}{
		{"07", SecurityStatusProtected},
		{"7", SecurityStatusProtected},
		{"C2", SecurityStatusUnprotected},
		{"c2", SecurityStatusUnprotected},
		{"00", SecurityStatusUndefined},
		{"", SecurityStatusUndefined},
	}

	for _, tt := range afiTests {
		got := AFIToSecurityStatus(tt.afi)
		if got != tt.expected {
			t.Errorf("AFIToSecurityStatus(%q) = %q, expected %q", tt.afi, got, tt.expected)
		}
	}
}

func TestContainsXMLTag(t *testing.T) {
	xmlGetTag1 := []byte(`<GetTag xmlns="http://tempuri.org/" />`)
	xmlGetTag2 := []byte(`<tem:GetTag xmlns:tem="http://tempuri.org/"></tem:GetTag>`)
	xmlGetItems := []byte(`<tem:GetItems xmlns:tem="http://tempuri.org/"/>`)
	xmlWriteTag := []byte(`<WriteTag xmlns="http://tempuri.org/"><ItemIdentifier>123</ItemIdentifier></WriteTag>`)
	xmlWriteItems := []byte(`<tem:WriteItems xmlns:tem="http://tempuri.org/"><tem:ItemIdentifier>123</tem:ItemIdentifier></tem:WriteItems>`)
	xmlConnect := []byte(`<tem:ConnectReader xmlns:tem="http://tempuri.org/"/>`)
	xmlIsOnline := []byte(`<tem:IsOnline xmlns:tem="http://tempuri.org/"/>`)
	xmlSetTagSec := []byte(`<tem:SetTagSecurity xmlns:tem="http://tempuri.org/"><tem:tagId>E004</tem:tagId></tem:SetTagSecurity>`)

	if !ContainsXMLTag(xmlGetTag1, "GetTag") || !IsGetTag(xmlGetTag1) {
		t.Errorf("expected ContainsXMLTag(xmlGetTag1, GetTag) == true")
	}
	if !ContainsXMLTag(xmlGetTag2, "GetTag") || !IsGetTag(xmlGetTag2) {
		t.Errorf("expected ContainsXMLTag(xmlGetTag2, GetTag) == true")
	}
	if !ContainsXMLTag(xmlGetItems, "GetItems") || !IsGetItems(xmlGetItems) {
		t.Errorf("expected ContainsXMLTag(xmlGetItems, GetItems) == true")
	}
	if !ContainsXMLTag(xmlWriteTag, "WriteTag") || !IsWriteTag(xmlWriteTag) {
		t.Errorf("expected ContainsXMLTag(xmlWriteTag, WriteTag) == true")
	}
	if !ContainsXMLTag(xmlWriteItems, "WriteItems") || !IsWriteItems(xmlWriteItems) {
		t.Errorf("expected ContainsXMLTag(xmlWriteItems, WriteItems) == true")
	}
	if !IsConnectReader(xmlConnect) {
		t.Errorf("expected IsConnectReader == true")
	}
	if !IsIsOnline(xmlIsOnline) {
		t.Errorf("expected IsIsOnline == true")
	}
	if !IsSetTagSecurity(xmlSetTagSec) {
		t.Errorf("expected IsSetTagSecurity == true")
	}
	if IsGetTag(xmlWriteTag) {
		t.Errorf("expected IsGetTag(xmlWriteTag) == false")
	}
}

func TestArbiterGetItemsMarshaling(t *testing.T) {
	fields := []ArbiterTagField{
		{Name: "ItemID", Type: "String", Value: "A1001141486"},
		{Name: "SetNumber", Type: "UnsignedInteger32", Value: "1"},
		{Name: "SetSize", Type: "UnsignedInteger32", Value: "1"},
		{Name: "MediaType", Type: "UnsignedInteger32", Value: "0"},
		{Name: "BranchID", Type: "UnsignedInteger32", Value: "1"},
		{Name: "LibraryID", Type: "UnsignedInteger32", Value: "1"},
	}

	arbiterTag := ArbiterTag{
		ID:           "E004015012345678",
		IsTagSecured: "true",
		TagFormat:    "ISO 28560-2",
		Type:         "Item",
	}

	item := ArbiterItem{
		Fields:        fields,
		ID:            "A1001141486",
		IsSecured:     "Yes",
		IsValid:       "True",
		TagFormatName: "ISO 28560-2",
		Type:          "Item",
		Tag:           arbiterTag,
	}

	resp := GetItemsResponse{
		GetItemsResult: GetItemsResult{
			XmlnsA: NamespaceArbiter,
			XmlnsI: NamespaceXMLSchemaInst,
			Items:  []ArbiterItem{item},
		},
	}

	envResp := NewSOAPEnvelopeResponse(resp)
	out, err := xml.Marshal(envResp)
	if err != nil {
		t.Fatalf("failed to marshal Arbiter GetItemsResponse: %v", err)
	}

	outStr := string(out)

	// Verify crucial XML structure elements parsed by rfid.html in Alma:
	if !strings.Contains(outStr, `<GetItemsResponse xmlns="http://tempuri.org/">`) {
		t.Errorf("missing GetItemsResponse with xmlns: %s", outStr)
	}
	if !strings.Contains(outStr, `<a:Id>A1001141486</a:Id>`) {
		t.Errorf("missing <a:Id> for item barcode: %s", outStr)
	}
	if !strings.Contains(outStr, `<a:IsSecured>Yes</a:IsSecured>`) {
		t.Errorf("missing <a:IsSecured>Yes: %s", outStr)
	}
	if !strings.Contains(outStr, `<a:IsValid>True</a:IsValid>`) {
		t.Errorf("missing <a:IsValid>True: %s", outStr)
	}
	if !strings.Contains(outStr, `<a:TagFormatName>ISO 28560-2</a:TagFormatName>`) {
		t.Errorf("missing <a:TagFormatName>: %s", outStr)
	}
	if !strings.Contains(outStr, `<Type>Item</Type>`) {
		t.Errorf("missing <Type>Item</Type>: %s", outStr)
	}
	if !strings.Contains(outStr, `<a:Tag><a:Id>E004015012345678</a:Id>`) {
		t.Errorf("missing <a:Tag><a:Id> UID: %s", outStr)
	}
	if !strings.Contains(outStr, `<a:TagField><a:Name>BranchID</a:Name>`) {
		t.Errorf("missing <a:TagField> BranchID: %s", outStr)
	}
}

func TestParseSetTagSecurityRequest(t *testing.T) {
	raw3M := []byte(`<tem:SetTagSecurity xmlns:tem="http://tempuri.org/">
		<tem:tagId>E004015099887766</tem:tagId>
		<tem:isSecured>true</tem:isSecured>
	</tem:SetTagSecurity>`)

	req3M := ParseSetTagSecurityRequest(raw3M)
	if req3M.TagID != "E004015099887766" {
		t.Errorf("expected TagID E004015099887766, got %s", req3M.TagID)
	}
	if !req3M.IsSecured {
		t.Errorf("expected IsSecured true")
	}

	rawB := []byte(`<rfid:SetSecurity xmlns:rfid="http://liber8Connect.com/WCF/Bibliotheca/RFIDPadService">
		<itemID>BARCODE-99</itemID>
		<isSecure>false</isSecure>
	</rfid:SetSecurity>`)

	reqB := ParseSetTagSecurityRequest(rawB)
	if reqB.Barcode != "BARCODE-99" {
		t.Errorf("expected Barcode BARCODE-99, got %s", reqB.Barcode)
	}
	if reqB.IsSecured {
		t.Errorf("expected IsSecured false")
	}
}

func TestParseWriteTagRequest(t *testing.T) {
	raw3M := []byte(`<WriteTag xmlns="http://tempuri.org/">
		<tag xmlns:a="http://schemas.datacontract.org/2004/07/MMM.Arbiter.ServiceModel" xmlns:i="http://www.w3.org/2001/XMLSchema-instance">
			<a:Fields xmlns:b="http://schemas.microsoft.com/2003/10/Serialization/Arrays">
				<a:TagField>
					<a:Name>ItemID</a:Name>
					<a:Type>String</a:Type>
					<a:Value>390000998877</a:Value>
				</a:TagField>
				<a:TagField>
					<a:Name>BranchID</a:Name>
					<a:Type>UnsignedInteger32</a:Type>
					<a:Value>2</a:Value>
				</a:TagField>
			</a:Fields>
			<a:Id>E004015012345678</a:Id>
			<a:IsTagSecured>true</a:IsTagSecured>
			<a:TagFormat>ISO 28560-2</a:TagFormat>
			<a:Type>Item</a:Type>
		</tag>
	</WriteTag>`)

	parsed3M := ParseWriteTagRequest(raw3M)
	if parsed3M.TagUID != "E004015012345678" {
		t.Errorf("expected TagUID E004015012345678, got %s", parsed3M.TagUID)
	}
	if parsed3M.ItemIdentifier != "390000998877" {
		t.Errorf("expected ItemIdentifier 390000998877, got %s", parsed3M.ItemIdentifier)
	}
	if parsed3M.BranchID != "2" {
		t.Errorf("expected BranchID 2, got %s", parsed3M.BranchID)
	}
	if parsed3M.IsSecured != "true" {
		t.Errorf("expected IsSecured true, got %s", parsed3M.IsSecured)
	}
}
