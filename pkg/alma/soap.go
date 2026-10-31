package alma

import (
	"encoding/xml"
	"strings"

	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
)

// Standard namespaces for SOAP 1.1 and Alma TagService.
const (
	NamespaceSOAPEnvelope  = "http://schemas.xmlsoap.org/soap/envelope/"
	NamespaceTempURI       = "http://tempuri.org/"
	NamespaceArbiter       = "http://schemas.datacontract.org/2004/07/MMM.Arbiter.ServiceModel"
	NamespaceArrays        = "http://schemas.microsoft.com/2003/10/Serialization/Arrays"
	NamespaceXMLSchema     = "http://www.w3.org/2001/XMLSchema"
	NamespaceXMLSchemaInst = "http://www.w3.org/2001/XMLSchema-instance"
)

// Security status strings expected by Ex Libris Alma.
const (
	SecurityStatusProtected   = "Protected"
	SecurityStatusUnprotected = "Unprotected"
	SecurityStatusUndefined   = "Undefined"
	SecurityStatusUnknown     = "Unknown"
)

// Envelope represents a standard SOAP 1.1 envelope.
type Envelope struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    Body     `xml:"Body"`
}

// Body represents the body of a SOAP envelope containing arbitrary XML content.
type Body struct {
	XMLName xml.Name `xml:"Body"`
	Content []byte   `xml:",innerxml"`
}

// ArbiterTagField represents a key-value tag metadata field in 3M Arbiter DataContract.
type ArbiterTagField struct {
	XMLName xml.Name `xml:"a:TagField"`
	Name    string   `xml:"a:Name"`
	Type    string   `xml:"a:Type,omitempty"`
	Value   string   `xml:"a:Value,omitempty"`
}

// ArbiterTag represents a physical RFID chip / tag element in 3M Arbiter DataContract.
type ArbiterTag struct {
	XMLName      xml.Name `xml:"a:Tag"`
	ID           string   `xml:"a:Id"`
	IsTagSecured string   `xml:"a:IsTagSecured,omitempty"`
	TagFormat    string   `xml:"a:TagFormat,omitempty"`
	Type         string   `xml:"a:Type,omitempty"`
}

// ArbiterItem represents an RFID item in 3M Arbiter DataContract (3M896 protocol for Alma).
type ArbiterItem struct {
	XMLName       xml.Name          `xml:"a:Item"`
	Fields        []ArbiterTagField `xml:"a:Fields>a:TagField,omitempty"`
	FirstSeen     string            `xml:"a:FirstSeen,omitempty"`
	ID            string            `xml:"a:Id"`
	IsSecured     string            `xml:"a:IsSecured"`
	IsValid       string            `xml:"a:IsValid"`
	TagFormatName string            `xml:"a:TagFormatName"`
	Type          string            `xml:"Type"`
	Tag           ArbiterTag        `xml:"a:Tag"`
}

// GetItemsResult represents the result container for 3M Arbiter GetItemsResponse.
type GetItemsResult struct {
	XMLName xml.Name      `xml:"GetItemsResult"`
	XmlnsA  string        `xml:"xmlns:a,attr"`
	XmlnsI  string        `xml:"xmlns:i,attr"`
	Items   []ArbiterItem `xml:"a:Item,omitempty"`
}

// GetItemsResponse represents the SOAP response payload for querying RFID items (3M896 / Alma).
type GetItemsResponse struct {
	XMLName        xml.Name       `xml:"http://tempuri.org/ GetItemsResponse"`
	GetItemsResult GetItemsResult `xml:"GetItemsResult"`
}

// GetTagRequest represents the SOAP request payload for querying RFID tag data.
type GetTagRequest struct {
	XMLName xml.Name `xml:"GetTag"`
}

// GetTagResponse represents the SOAP response payload for querying RFID tag data.
type GetTagResponse struct {
	XMLName        xml.Name `xml:"http://tempuri.org/ GetTagResponse"`
	Success        bool     `xml:"GetTagResult>Success"`
	ItemIdentifier string   `xml:"GetTagResult>ItemIdentifier"`
	SecurityStatus string   `xml:"SecurityStatus"`
}

// ConnectReaderResponse represents the SOAP response for ConnectReader action.
type ConnectReaderResponse struct {
	XMLName             xml.Name `xml:"http://tempuri.org/ ConnectReaderResponse"`
	ConnectReaderResult bool     `xml:"ConnectReaderResult"`
}

// IsOnlineResponse represents the SOAP response for IsOnline action.
type IsOnlineResponse struct {
	XMLName        xml.Name `xml:"http://tempuri.org/ IsOnlineResponse"`
	IsOnlineResult string   `xml:"IsOnlineResult"`
}

// PingResponse represents the SOAP response for Ping action.
type PingResponse struct {
	XMLName    xml.Name `xml:"http://tempuri.org/ PingResponse"`
	PingResult string   `xml:"PingResult"`
}

// SetTagSecurityRequest represents parsed parameters for a security bit write operation.
type SetTagSecurityRequest struct {
	TagID     string
	IsSecured bool
	Barcode   string
}

// SetTagSecurityResponse represents the SOAP response payload for SetTagSecurity (3M896 / Alma).
type SetTagSecurityResponse struct {
	XMLName              xml.Name `xml:"http://tempuri.org/ SetTagSecurityResponse"`
	SetTagSecurityResult bool     `xml:"SetTagSecurityResult"`
}

// SetSecurityResponse represents the SOAP response payload for SetSecurity (Bibliotheca / Alma).
type SetSecurityResponse struct {
	XMLName           xml.Name `xml:"http://liber8Connect.com/WCF/Bibliotheca/RFIDPadService SetSecurityResponse"`
	SetSecurityResult bool     `xml:"SetSecurityResult"`
}

// WriteTagRequest represents generic write parameters.
type WriteTagRequest struct {
	XMLName        xml.Name `xml:"WriteTag"`
	ItemIdentifier string   `xml:"ItemIdentifier"`
	SecurityStatus string   `xml:"SecurityStatus"`
}

// ParsedWriteTag contains extracted parameters from either 3M Arbiter WriteTag or generic WriteTag.
type ParsedWriteTag struct {
	TagUID         string
	ItemIdentifier string
	IsSecured      string // "true" | "false" | "Yes" | "No" | "Protected" | "Unprotected"
	TagFormat      string
	MediaType      string
	BranchID       string
	LibraryID      string
	SetNumber      string
	SetSize        string
	CustomData     string
}

// WriteTagResponse represents the SOAP response payload for write operations.
type WriteTagResponse struct {
	XMLName        xml.Name `xml:"http://tempuri.org/ WriteTagResponse"`
	WriteTagResult bool     `xml:"WriteTagResult"`
}

// WriteItemsRequest represents the SOAP request payload for writing/updating RFID items.
type WriteItemsRequest struct {
	XMLName        xml.Name `xml:"WriteItems"`
	ItemIdentifier string   `xml:"ItemIdentifier"`
	SecurityStatus string   `xml:"SecurityStatus"`
}

// WriteItemsResponse represents the SOAP response payload for write operations (3M896 / Alma).
type WriteItemsResponse struct {
	XMLName          xml.Name `xml:"http://tempuri.org/ WriteItemsResponse"`
	WriteItemsResult bool     `xml:"WriteItemsResult"`
}

// SOAPEnvelopeResponse wraps payload inside a standard SOAP 1.1 envelope with Alma tempuri namespace.
type SOAPEnvelopeResponse struct {
	XMLName   xml.Name `xml:"soapenv:Envelope"`
	XmlnsSoap string   `xml:"xmlns:soapenv,attr"`
	Header    string   `xml:"soapenv:Header"`
	Body      struct {
		Data any
	} `xml:"soapenv:Body"`
}

// NewSOAPEnvelopeResponse creates a new SOAP 1.1 response envelope with standard namespace declarations.
func NewSOAPEnvelopeResponse(payload any) SOAPEnvelopeResponse {
	return SOAPEnvelopeResponse{
		XmlnsSoap: NamespaceSOAPEnvelope,
		Body: struct{ Data any }{
			Data: payload,
		},
	}
}

// SecurityStatusToAFI converts an Alma security status string into an ISO 28560 AFI hex string.
func SecurityStatusToAFI(status string) string {
	s := strings.TrimSpace(strings.ToLower(status))
	switch s {
	case "protected", "secured", "gesichert", "true", "yes", "1", "07", "7":
		return libraryclient.AFISecured
	case "unprotected", "unsecured", "entsichert", "false", "no", "0", "c2":
		return libraryclient.AFIUnsecured
	default:
		return ""
	}
}

// AFIToSecurityStatus converts an ISO 28560 AFI hex string into an Alma security status string.
func AFIToSecurityStatus(afi string) string {
	norm := libraryclient.NormalizeAFI(afi)
	switch norm {
	case libraryclient.AFISecured:
		return SecurityStatusProtected
	case libraryclient.AFIUnsecured:
		return SecurityStatusUnprotected
	default:
		return SecurityStatusUndefined
	}
}

// IsGetTag returns true if the inner XML represents a GetTag command.
func IsGetTag(content []byte) bool {
	return ContainsXMLTag(content, "GetTag")
}

// IsGetItems returns true if the inner XML represents a GetItems command (3M896 / Alma).
func IsGetItems(content []byte) bool {
	return ContainsXMLTag(content, "GetItems") || ContainsXMLTag(content, "GetDetectedItems")
}

// IsWriteTag returns true if the inner XML represents a WriteTag command.
func IsWriteTag(content []byte) bool {
	return ContainsXMLTag(content, "WriteTag") || ContainsXMLTag(content, "ItemUpdate")
}

// IsWriteItems returns true if the inner XML represents a WriteItems command.
func IsWriteItems(content []byte) bool {
	return ContainsXMLTag(content, "WriteItems") || ContainsXMLTag(content, "WriteDetectedItems")
}

// IsSetTagSecurity returns true if the inner XML represents a SetTagSecurity / SetSecurity command.
func IsSetTagSecurity(content []byte) bool {
	return ContainsXMLTag(content, "SetTagSecurity") || ContainsXMLTag(content, "SetSecurity")
}

// IsConnectReader returns true if the inner XML represents a ConnectReader / Connect command.
func IsConnectReader(content []byte) bool {
	return ContainsXMLTag(content, "ConnectReader") || ContainsXMLTag(content, "Connect")
}

// IsIsOnline returns true if the inner XML represents an IsOnline command.
func IsIsOnline(content []byte) bool {
	return ContainsXMLTag(content, "IsOnline")
}

// IsPing returns true if the inner XML represents a Ping command.
func IsPing(content []byte) bool {
	return ContainsXMLTag(content, "Ping")
}

// ContainsXMLTag checks if the given XML data starts with or contains the specified local tag name.
func ContainsXMLTag(content []byte, tagName string) bool {
	var f struct {
		XMLName xml.Name
	}
	err := xml.Unmarshal(content, &f)
	if err == nil && strings.EqualFold(f.XMLName.Local, tagName) {
		return true
	}
	// Fallback check in case inner XML has leading namespaces/whitespaces
	d := xml.NewDecoder(strings.NewReader(string(content)))
	for {
		tok, err := d.Token()
		if err != nil {
			break
		}
		if se, ok := tok.(xml.StartElement); ok {
			if strings.EqualFold(se.Name.Local, tagName) {
				return true
			}
		}
	}
	return false
}

// ParseSetTagSecurityRequest parses SetTagSecurity (3M) or SetSecurity (Bibliotheca) XML payload.
func ParseSetTagSecurityRequest(content []byte) SetTagSecurityRequest {
	req := SetTagSecurityRequest{}
	d := xml.NewDecoder(strings.NewReader(string(content)))

	var currentTag string
	for {
		tok, err := d.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			currentTag = strings.ToLower(t.Name.Local)
		case xml.EndElement:
			currentTag = ""
		case xml.CharData:
			val := strings.TrimSpace(string(t))
			if val == "" {
				continue
			}
			switch currentTag {
			case "tagid":
				req.TagID = val
			case "issecured", "issecure":
				req.IsSecured = strings.EqualFold(val, "true") || strings.EqualFold(val, "yes") || val == "1"
			case "itemid", "barcode":
				req.Barcode = val
			}
		}
	}
	return req
}

// ParseWriteTagRequest parses WriteTag XML payloads in both 3M Arbiter and generic formats.
func ParseWriteTagRequest(content []byte) ParsedWriteTag {
	res := ParsedWriteTag{}
	d := xml.NewDecoder(strings.NewReader(string(content)))

	var tagStack []string
	var lastFieldName string

	for {
		tok, err := d.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			local := strings.ToLower(t.Name.Local)
			tagStack = append(tagStack, local)
		case xml.EndElement:
			if len(tagStack) > 0 {
				tagStack = tagStack[:len(tagStack)-1]
			}
		case xml.CharData:
			val := strings.TrimSpace(string(t))
			if val == "" || len(tagStack) == 0 {
				continue
			}
			cur := tagStack[len(tagStack)-1]

			if cur == "name" {
				lastFieldName = strings.ToLower(val)
			} else if cur == "value" && lastFieldName != "" {
				switch lastFieldName {
				case "itemid":
					res.ItemIdentifier = val
				case "branchid":
					res.BranchID = val
				case "libraryid":
					res.LibraryID = val
				case "mediatype":
					res.MediaType = val
				case "setnumber":
					res.SetNumber = val
				case "setsize":
					res.SetSize = val
				case "customdata":
					res.CustomData = val
				}
			} else {
				switch cur {
				case "itemidentifier":
					res.ItemIdentifier = val
				case "securitystatus":
					res.IsSecured = val
				case "istagsecured", "issecured", "issecure":
					res.IsSecured = val
				case "tagformat":
					res.TagFormat = val
				case "id":
					// In 3M WriteTag, <a:Id> directly on tag is the chip UID
					if res.TagUID == "" {
						res.TagUID = val
					}
				}
			}
		}
	}
	return res
}

// TagServiceWSDL contains the WSDL definition for Alma TagService integration.
const TagServiceWSDL = `<?xml version="1.0" encoding="utf-8"?>
<wsdl:definitions xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/" xmlns:tm="http://microsoft.com/wsdl/mime/textMatching/" xmlns:soapenc="http://schemas.xmlsoap.org/soap/encoding/" xmlns:mime="http://schemas.xmlsoap.org/wsdl/mime/" xmlns:tns="http://tempuri.org/" xmlns:s="http://www.w3.org/2001/XMLSchema" xmlns:soap12="http://schemas.xmlsoap.org/wsdl/soap12/" xmlns:http="http://schemas.xmlsoap.org/wsdl/http/" targetNamespace="http://tempuri.org/" xmlns:wsdl="http://schemas.xmlsoap.org/wsdl/">
  <wsdl:types>
    <s:schema elementFormDefault="qualified" targetNamespace="http://tempuri.org/">
      <s:element name="GetTag">
        <s:complexType />
      </s:element>
      <s:element name="GetTagResponse">
        <s:complexType>
          <s:sequence>
            <s:element minOccurs="0" maxOccurs="1" name="GetTagResult" type="tns:TagResult" />
            <s:element minOccurs="0" maxOccurs="1" name="SecurityStatus" type="s:string" />
          </s:sequence>
        </s:complexType>
      </s:element>
      <s:element name="GetItems">
        <s:complexType />
      </s:element>
      <s:element name="GetItemsResponse">
        <s:complexType>
          <s:sequence>
            <s:element minOccurs="0" maxOccurs="1" name="GetItemsResult" type="tns:GetItemsResult" />
          </s:sequence>
        </s:complexType>
      </s:element>
      <s:complexType name="GetItemsResult">
        <s:sequence>
          <s:element minOccurs="1" maxOccurs="1" name="Success" type="s:boolean" />
          <s:element minOccurs="0" maxOccurs="1" name="Items" type="tns:ArrayOfItem" />
        </s:sequence>
      </s:complexType>
      <s:complexType name="ArrayOfItem">
        <s:sequence>
          <s:element minOccurs="0" maxOccurs="unbounded" name="Item" nillable="true" type="tns:Item" />
        </s:sequence>
      </s:complexType>
      <s:complexType name="Item">
        <s:sequence>
          <s:element minOccurs="0" maxOccurs="1" name="ItemIdentifier" type="s:string" />
          <s:element minOccurs="0" maxOccurs="1" name="SecurityStatus" type="s:string" />
        </s:sequence>
      </s:complexType>
      <s:complexType name="TagResult">
        <s:sequence>
          <s:element minOccurs="1" maxOccurs="1" name="Success" type="s:boolean" />
          <s:element minOccurs="0" maxOccurs="1" name="ItemIdentifier" type="s:string" />
        </s:sequence>
      </s:complexType>
      <s:element name="WriteTag">
        <s:complexType>
          <s:sequence>
            <s:element minOccurs="0" maxOccurs="1" name="ItemIdentifier" type="s:string" />
            <s:element minOccurs="0" maxOccurs="1" name="SecurityStatus" type="s:string" />
          </s:sequence>
        </s:complexType>
      </s:element>
      <s:element name="WriteTagResponse">
        <s:complexType>
          <s:sequence>
            <s:element minOccurs="1" maxOccurs="1" name="WriteTagResult" type="tns:WriteResult" />
          </s:sequence>
        </s:complexType>
      </s:element>
      <s:element name="WriteItems">
        <s:complexType>
          <s:sequence>
            <s:element minOccurs="0" maxOccurs="1" name="ItemIdentifier" type="s:string" />
            <s:element minOccurs="0" maxOccurs="1" name="SecurityStatus" type="s:string" />
          </s:sequence>
        </s:complexType>
      </s:element>
      <s:element name="WriteItemsResponse">
        <s:complexType>
          <s:sequence>
            <s:element minOccurs="1" maxOccurs="1" name="WriteItemsResult" type="tns:WriteResult" />
          </s:sequence>
        </s:complexType>
      </s:element>
      <s:element name="SetTagSecurity">
        <s:complexType>
          <s:sequence>
            <s:element minOccurs="0" maxOccurs="1" name="tagId" type="s:string" />
            <s:element minOccurs="1" maxOccurs="1" name="isSecured" type="s:boolean" />
          </s:sequence>
        </s:complexType>
      </s:element>
      <s:element name="SetTagSecurityResponse">
        <s:complexType>
          <s:sequence>
            <s:element minOccurs="1" maxOccurs="1" name="SetTagSecurityResult" type="s:boolean" />
          </s:sequence>
        </s:complexType>
      </s:element>
      <s:element name="ConnectReader">
        <s:complexType />
      </s:element>
      <s:element name="ConnectReaderResponse">
        <s:complexType>
          <s:sequence>
            <s:element minOccurs="1" maxOccurs="1" name="ConnectReaderResult" type="s:boolean" />
          </s:sequence>
        </s:complexType>
      </s:element>
      <s:element name="IsOnline">
        <s:complexType />
      </s:element>
      <s:element name="IsOnlineResponse">
        <s:complexType>
          <s:sequence>
            <s:element minOccurs="0" maxOccurs="1" name="IsOnlineResult" type="s:string" />
          </s:sequence>
        </s:complexType>
      </s:element>
      <s:complexType name="WriteResult">
        <s:sequence>
          <s:element minOccurs="1" maxOccurs="1" name="Success" type="s:boolean" />
        </s:sequence>
      </s:complexType>
    </s:schema>
  </wsdl:types>
  <wsdl:message name="GetTagSoapIn">
    <wsdl:part name="parameters" element="tns:GetTag" />
  </wsdl:message>
  <wsdl:message name="GetTagSoapOut">
    <wsdl:part name="parameters" element="tns:GetTagResponse" />
  </wsdl:message>
  <wsdl:message name="GetItemsSoapIn">
    <wsdl:part name="parameters" element="tns:GetItems" />
  </wsdl:message>
  <wsdl:message name="GetItemsSoapOut">
    <wsdl:part name="parameters" element="tns:GetItemsResponse" />
  </wsdl:message>
  <wsdl:message name="WriteTagSoapIn">
    <wsdl:part name="parameters" element="tns:WriteTag" />
  </wsdl:message>
  <wsdl:message name="WriteTagSoapOut">
    <wsdl:part name="parameters" element="tns:WriteTagResponse" />
  </wsdl:message>
  <wsdl:message name="WriteItemsSoapIn">
    <wsdl:part name="parameters" element="tns:WriteItems" />
  </wsdl:message>
  <wsdl:message name="WriteItemsSoapOut">
    <wsdl:part name="parameters" element="tns:WriteItemsResponse" />
  </wsdl:message>
  <wsdl:message name="SetTagSecuritySoapIn">
    <wsdl:part name="parameters" element="tns:SetTagSecurity" />
  </wsdl:message>
  <wsdl:message name="SetTagSecuritySoapOut">
    <wsdl:part name="parameters" element="tns:SetTagSecurityResponse" />
  </wsdl:message>
  <wsdl:portType name="TagServiceSoap">
    <wsdl:operation name="GetTag">
      <wsdl:input message="tns:GetTagSoapIn" />
      <wsdl:output message="tns:GetTagSoapOut" />
    </wsdl:operation>
    <wsdl:operation name="GetItems">
      <wsdl:input message="tns:GetItemsSoapIn" />
      <wsdl:output message="tns:GetItemsSoapOut" />
    </wsdl:operation>
    <wsdl:operation name="WriteTag">
      <wsdl:input message="tns:WriteTagSoapIn" />
      <wsdl:output message="tns:WriteTagSoapOut" />
    </wsdl:operation>
    <wsdl:operation name="WriteItems">
      <wsdl:input message="tns:WriteItemsSoapIn" />
      <wsdl:output message="tns:WriteItemsSoapOut" />
    </wsdl:operation>
    <wsdl:operation name="SetTagSecurity">
      <wsdl:input message="tns:SetTagSecuritySoapIn" />
      <wsdl:output message="tns:SetTagSecuritySoapOut" />
    </wsdl:operation>
  </wsdl:portType>
  <wsdl:binding name="TagServiceSoap" type="tns:TagServiceSoap">
    <soap:binding transport="http://schemas.xmlsoap.org/soap/http" />
    <wsdl:operation name="GetTag">
      <soap:operation soapAction="http://tempuri.org/GetTag" style="document" />
      <wsdl:input>
        <soap:body use="literal" />
      </wsdl:input>
      <wsdl:output>
        <soap:body use="literal" />
      </wsdl:output>
    </wsdl:operation>
    <wsdl:operation name="GetItems">
      <soap:operation soapAction="http://tempuri.org/ITagService/GetItems" style="document" />
      <wsdl:input>
        <soap:body use="literal" />
      </wsdl:input>
      <wsdl:output>
        <soap:body use="literal" />
      </wsdl:output>
    </wsdl:operation>
    <wsdl:operation name="WriteTag">
      <soap:operation soapAction="http://tempuri.org/ITagService/WriteTag" style="document" />
      <wsdl:input>
        <soap:body use="literal" />
      </wsdl:input>
      <wsdl:output>
        <soap:body use="literal" />
      </wsdl:output>
    </wsdl:operation>
    <wsdl:operation name="WriteItems">
      <soap:operation soapAction="http://tempuri.org/WriteItems" style="document" />
      <wsdl:input>
        <soap:body use="literal" />
      </wsdl:input>
      <wsdl:output>
        <soap:body use="literal" />
      </wsdl:output>
    </wsdl:operation>
    <wsdl:operation name="SetTagSecurity">
      <soap:operation soapAction="http://tempuri.org/ICoreTagService/SetTagSecurity" style="document" />
      <wsdl:input>
        <soap:body use="literal" />
      </wsdl:input>
      <wsdl:output>
        <soap:body use="literal" />
      </wsdl:output>
    </wsdl:operation>
  </wsdl:binding>
  <wsdl:service name="TagService">
    <wsdl:port name="TagServiceSoap" binding="tns:TagServiceSoap">
      <soap:address location="http://127.0.0.1:5000/TagService" />
    </wsdl:port>
  </wsdl:service>
</wsdl:definitions>`
