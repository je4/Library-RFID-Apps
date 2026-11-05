package sip2

import (
	"context"
	"fmt"
	"strings"
)

// Fields retains variable fields, including repeated screen messages and print lines.
type Fields map[string][]string

// MessageResponse contains the fixed and variable parts of a response. Dates are
// SIP2's 18-byte date/time representation, not necessarily UTC.
type MessageResponse struct {
	TransactionDate string
	Fields          Fields
}

type CheckoutRequest struct {
	RenewalPolicy, NoBlock                            bool
	TransactionDate, DueDate                          string
	InstitutionID, PatronID, ItemID, TerminalPassword string
	PatronPassword, ItemProperties                    string
	FeeAcknowledged, Cancel                           *bool
}

type CheckoutResponse struct {
	MessageResponse
	OK, RenewalOK              bool
	MagneticMedia, Desensitize byte
}

type CheckinRequest struct {
	NoBlock                                                  bool
	TransactionDate, ReturnDate                              string
	CurrentLocation, InstitutionID, ItemID, TerminalPassword string
	ItemProperties                                           string
	Cancel                                                   *bool
}

type CheckinResponse struct {
	MessageResponse
	OK, Resensitize, Alert bool
	MagneticMedia          byte
}

type EndSessionRequest struct {
	TransactionDate, InstitutionID, PatronID string
	TerminalPassword, PatronPassword         string
}

type EndSessionResponse struct {
	MessageResponse
	Ended bool
}

type ItemStatusUpdateRequest struct {
	TransactionDate, InstitutionID, ItemID, TerminalPassword, ItemProperties string
}

type ItemStatusUpdateResponse struct {
	MessageResponse
	PropertiesOK bool
}

type ItemInformationRequest struct {
	TransactionDate, InstitutionID, ItemID, TerminalPassword string
}

type ItemInformationResponse struct {
	MessageResponse
	CirculationStatus, SecurityMarker, FeeType string
}

func yn(b bool) string {
	if b {
		return "Y"
	}
	return "N"
}

func validDate(s string) bool {
	if len(s) != 18 {
		return false
	}
	for i := range s {
		if i >= 8 && i < 12 && (s[i] == ' ' || (i == 11 && s[i] >= 'A' && s[i] <= 'Z')) {
			continue
		}
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

type field struct {
	id, value string
	required  bool
}

func encodeFields(fields ...field) (string, error) {
	var b strings.Builder
	for _, f := range fields {
		if len(f.value) > 255 || strings.ContainsAny(f.value, "|\r\n\x00") {
			return "", fmt.Errorf("invalid SIP2 field %s", f.id)
		}
		if f.required || f.value != "" {
			b.WriteString(f.id)
			b.WriteString(f.value)
			b.WriteByte('|')
		}
	}
	return b.String(), nil
}

func optionalBool(id string, b *bool) field {
	if b == nil {
		return field{id: id}
	}
	return field{id: id, value: yn(*b)}
}

func parseResponse(raw, code, flags string, required ...string) (MessageResponse, error) {
	var result MessageResponse
	n := len(flags)
	if len(raw) < 2+n+18 || raw[:2] != code {
		return result, fmt.Errorf("invalid SIP2 %s response", code)
	}
	for i := range n {
		allowed := "YN"
		if flags[i] == '0' {
			allowed = "01"
		} else if flags[i] == 'U' {
			allowed = "YNU"
		} else if flags[i] == 'D' {
			allowed = "0123456789"
		} else if flags[i] == 'S' {
			allowed = " Y"
		}
		if !strings.ContainsRune(allowed, rune(raw[2+i])) {
			return result, fmt.Errorf("invalid SIP2 %s response flag", code)
		}
	}
	result.TransactionDate = raw[2+n : 2+n+18]
	if !validDate(result.TransactionDate) {
		return result, fmt.Errorf("invalid SIP2 response date")
	}
	result.Fields = make(Fields)
	rest := raw[2+n+18:]
	if rest != "" && !strings.HasSuffix(rest, "|") {
		return result, fmt.Errorf("unterminated SIP2 field")
	}
	if rest != "" {
		for _, part := range strings.Split(rest[:len(rest)-1], "|") {
			if len(part) < 2 || len(part)-2 > 255 || strings.ContainsAny(part, "\r\n\x00") {
				return result, fmt.Errorf("invalid SIP2 response field")
			}
			id, value := part[:2], part[2:]
			if (id == "BL" || id == "CQ" || id == "CI") && value != "Y" && value != "N" {
				return result, fmt.Errorf("invalid SIP2 field %s", id)
			}
			if id == "BW" && !validDate(value) {
				return result, fmt.Errorf("invalid SIP2 field %s", id)
			}
			result.Fields[part[:2]] = append(result.Fields[part[:2]], part[2:])
		}
	}
	for _, id := range required {
		if len(result.Fields[id]) == 0 {
			return result, fmt.Errorf("missing SIP2 field %s", id)
		}
	}
	return result, nil
}

func (c *Client) message(ctx context.Context, code, fixed, responseCode, flags string, fields []field, required ...string) (string, MessageResponse, error) {
	encoded, err := encodeFields(fields...)
	if err != nil {
		return "", MessageResponse{}, err
	}
	raw, err := c.Exchange(ctx, code+fixed+encoded, responseCode)
	if err != nil {
		return "", MessageResponse{}, err
	}
	response, err := parseResponse(raw, responseCode, flags, required...)
	if err != nil {
		c.Close()
		return "", MessageResponse{}, err
	}
	return raw, response, nil
}

func (c *Client) Checkout(ctx context.Context, r CheckoutRequest) (CheckoutResponse, error) {
	if !validDate(r.TransactionDate) || !validDate(r.DueDate) {
		return CheckoutResponse{}, fmt.Errorf("invalid SIP2 date")
	}
	raw, response, err := c.message(ctx, "11", yn(r.RenewalPolicy)+yn(r.NoBlock)+r.TransactionDate+r.DueDate, "12", "0YUU", []field{{"AO", r.InstitutionID, true}, {"AA", r.PatronID, true}, {"AB", r.ItemID, true}, {"AC", r.TerminalPassword, true}, {"AD", r.PatronPassword, false}, {"CH", r.ItemProperties, false}, optionalBool("BO", r.FeeAcknowledged), optionalBool("BI", r.Cancel)}, "AO", "AA", "AB", "AJ", "AH")
	if err != nil {
		return CheckoutResponse{}, err
	}
	return CheckoutResponse{response, raw[2] == '1', raw[3] == 'Y', raw[4], raw[5]}, nil
}

func (c *Client) Checkin(ctx context.Context, r CheckinRequest) (CheckinResponse, error) {
	if !validDate(r.TransactionDate) || !validDate(r.ReturnDate) {
		return CheckinResponse{}, fmt.Errorf("invalid SIP2 date")
	}
	raw, response, err := c.message(ctx, "09", yn(r.NoBlock)+r.TransactionDate+r.ReturnDate, "10", "0YUY", []field{{"AP", r.CurrentLocation, true}, {"AO", r.InstitutionID, true}, {"AB", r.ItemID, true}, {"AC", r.TerminalPassword, true}, {"CH", r.ItemProperties, false}, optionalBool("BI", r.Cancel)}, "AO", "AB", "AQ")
	if err != nil {
		return CheckinResponse{}, err
	}
	return CheckinResponse{response, raw[2] == '1', raw[3] == 'Y', raw[5] == 'Y', raw[4]}, nil
}

func (c *Client) EndSession(ctx context.Context, r EndSessionRequest) (EndSessionResponse, error) {
	if !validDate(r.TransactionDate) {
		return EndSessionResponse{}, fmt.Errorf("invalid SIP2 date")
	}
	raw, response, err := c.message(ctx, "35", r.TransactionDate, "36", "Y", []field{{"AO", r.InstitutionID, true}, {"AA", r.PatronID, true}, {"AC", r.TerminalPassword, false}, {"AD", r.PatronPassword, false}}, "AO", "AA")
	if err != nil {
		return EndSessionResponse{}, err
	}
	return EndSessionResponse{response, raw[2] == 'Y'}, nil
}

func (c *Client) ItemStatusUpdate(ctx context.Context, r ItemStatusUpdateRequest) (ItemStatusUpdateResponse, error) {
	if !validDate(r.TransactionDate) {
		return ItemStatusUpdateResponse{}, fmt.Errorf("invalid SIP2 date")
	}
	raw, response, err := c.message(ctx, "19", r.TransactionDate, "20", "0", []field{{"AO", r.InstitutionID, true}, {"AB", r.ItemID, true}, {"AC", r.TerminalPassword, false}, {"CH", r.ItemProperties, true}}, "AB")
	if err != nil {
		return ItemStatusUpdateResponse{}, err
	}
	return ItemStatusUpdateResponse{response, raw[2] == '1'}, nil
}

func (c *Client) ItemInformation(ctx context.Context, r ItemInformationRequest) (ItemInformationResponse, error) {
	if !validDate(r.TransactionDate) {
		return ItemInformationResponse{}, fmt.Errorf("invalid SIP2 date")
	}
	raw, response, err := c.message(ctx, "17", r.TransactionDate, "18", "DDDDDD", []field{{"AO", r.InstitutionID, true}, {"AB", r.ItemID, true}, {"AC", r.TerminalPassword, false}}, "AB", "AJ")
	if err != nil {
		return ItemInformationResponse{}, err
	}
	if raw[6:8] == "00" {
		c.Close()
		return ItemInformationResponse{}, fmt.Errorf("invalid SIP2 fee type")
	}
	return ItemInformationResponse{response, raw[2:4], raw[4:6], raw[6:8]}, nil
}

type PatronStatusRequest struct {
	Language, TransactionDate, InstitutionID, PatronID, TerminalPassword, PatronPassword string
}

type BlockPatronRequest struct {
	CardRetained                                                                   bool
	TransactionDate, InstitutionID, BlockedCardMessage, PatronID, TerminalPassword string
}

type PatronStatusResponse struct {
	MessageResponse
	PatronStatus, Language string
}

type PatronInformationRequest struct {
	Language, TransactionDate, Summary, InstitutionID, PatronID, TerminalPassword, PatronPassword, StartItem, EndItem string
}

type PatronInformationResponse struct {
	PatronStatusResponse
	HoldCount, OverdueCount, ChargedCount, FineCount, RecallCount, UnavailableHoldCount string
}

type PatronEnableRequest struct {
	TransactionDate, InstitutionID, PatronID, TerminalPassword, PatronPassword string
}

type SCStatusRequest struct {
	StatusCode                     byte
	MaxPrintWidth, ProtocolVersion string
}

type SCStatusResponse struct {
	MessageResponse
	Online, CheckinOK, CheckoutOK, RenewalPolicy, StatusUpdateOK, OfflineOK bool
	TimeoutPeriod, RetriesAllowed, ProtocolVersion                          string
}

type FeePaidRequest struct {
	TransactionDate, FeeType, PaymentType, CurrencyType, FeeAmount, InstitutionID, PatronID, TerminalPassword, PatronPassword, FeeIdentifier, TransactionID string
}

type FeePaidResponse struct {
	MessageResponse
	PaymentAccepted bool
}

type HoldRequest struct {
	HoldMode                                                                                                                              byte
	TransactionDate, ExpirationDate, PickupLocation, HoldType, InstitutionID, PatronID, PatronPassword, ItemID, TitleID, TerminalPassword string
	FeeAcknowledged                                                                                                                       *bool
}

type HoldResponse struct {
	MessageResponse
	OK, Available bool
}

type RenewRequest struct {
	ThirdPartyAllowed, NoBlock                                                                                           bool
	TransactionDate, DueDate, InstitutionID, PatronID, PatronPassword, ItemID, TitleID, TerminalPassword, ItemProperties string
	FeeAcknowledged                                                                                                      *bool
}

type RenewResponse = CheckoutResponse

type RenewAllRequest struct {
	TransactionDate, InstitutionID, PatronID, PatronPassword, TerminalPassword string
	FeeAcknowledged                                                            *bool
}

type RenewAllResponse struct {
	MessageResponse
	OK                           bool
	RenewedCount, UnrenewedCount string
}

func digits(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for i := range s {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func version(s string) bool {
	return len(s) == 4 && s[1] == '.' && digits(s[:1], 1) && digits(s[2:], 2)
}

// extendedResponse handles fixed fields following the transaction date.
func (c *Client) extendedResponse(ctx context.Context, code, fixed, responseCode, flags string, afterDate int, fields []field, required ...string) (string, MessageResponse, error) {
	encoded, err := encodeFields(fields...)
	if err != nil {
		return "", MessageResponse{}, err
	}
	raw, err := c.Exchange(ctx, code+fixed+encoded, responseCode)
	if err != nil {
		return "", MessageResponse{}, err
	}
	pos := 2 + len(flags) + 18
	if len(raw) < pos+afterDate {
		c.Close()
		return "", MessageResponse{}, fmt.Errorf("invalid SIP2 %s response length", responseCode)
	}
	response, err := parseResponse(raw[:pos]+raw[pos+afterDate:], responseCode, flags, required...)
	if err != nil {
		c.Close()
		return "", MessageResponse{}, err
	}
	return raw, response, nil
}

func (c *Client) patronResponse(ctx context.Context, code, fixed, responseCode string, fields []field) (PatronStatusResponse, error) {
	raw, response, err := c.message(ctx, code, fixed, responseCode, strings.Repeat("S", 14)+"DDD", fields, "AO", "AA", "AE")
	if err != nil {
		return PatronStatusResponse{}, err
	}
	return PatronStatusResponse{response, raw[2:16], raw[16:19]}, nil
}

func (c *Client) PatronStatus(ctx context.Context, r PatronStatusRequest) (PatronStatusResponse, error) {
	if !digits(r.Language, 3) || !validDate(r.TransactionDate) {
		return PatronStatusResponse{}, fmt.Errorf("invalid SIP2 language or date")
	}
	return c.patronResponse(ctx, "23", r.Language+r.TransactionDate, "24", []field{{"AO", r.InstitutionID, true}, {"AA", r.PatronID, true}, {"AC", r.TerminalPassword, true}, {"AD", r.PatronPassword, true}})
}

func (c *Client) BlockPatron(ctx context.Context, r BlockPatronRequest) (PatronStatusResponse, error) {
	if !validDate(r.TransactionDate) {
		return PatronStatusResponse{}, fmt.Errorf("invalid SIP2 date")
	}
	return c.patronResponse(ctx, "01", yn(r.CardRetained)+r.TransactionDate, "24", []field{{"AO", r.InstitutionID, true}, {"AL", r.BlockedCardMessage, true}, {"AA", r.PatronID, true}, {"AC", r.TerminalPassword, true}})
}

func (c *Client) PatronEnable(ctx context.Context, r PatronEnableRequest) (PatronStatusResponse, error) {
	if !validDate(r.TransactionDate) {
		return PatronStatusResponse{}, fmt.Errorf("invalid SIP2 date")
	}
	return c.patronResponse(ctx, "25", r.TransactionDate, "26", []field{{"AO", r.InstitutionID, true}, {"AA", r.PatronID, true}, {"AC", r.TerminalPassword, false}, {"AD", r.PatronPassword, false}})
}

func (c *Client) PatronInformation(ctx context.Context, r PatronInformationRequest) (PatronInformationResponse, error) {
	if !digits(r.Language, 3) || !validDate(r.TransactionDate) || len(r.Summary) != 10 {
		return PatronInformationResponse{}, fmt.Errorf("invalid SIP2 language, date or summary")
	}
	for i := range r.Summary {
		if r.Summary[i] != ' ' && r.Summary[i] != 'Y' {
			return PatronInformationResponse{}, fmt.Errorf("invalid SIP2 summary")
		}
	}
	if strings.Count(r.Summary, "Y") > 1 || strings.Contains(r.Summary[6:], "Y") {
		return PatronInformationResponse{}, fmt.Errorf("invalid SIP2 summary")
	}
	raw, response, err := c.extendedResponse(ctx, "63", r.Language+r.TransactionDate+r.Summary, "64", strings.Repeat("S", 14)+"DDD", 24, []field{{"AO", r.InstitutionID, true}, {"AA", r.PatronID, true}, {"AC", r.TerminalPassword, false}, {"AD", r.PatronPassword, false}, {"BP", r.StartItem, false}, {"BQ", r.EndItem, false}}, "AO", "AA", "AE")
	if err != nil {
		return PatronInformationResponse{}, err
	}
	counts := raw[37:61]
	for i := 0; i < 24; i += 4 {
		if !digits(counts[i:i+4], 4) && counts[i:i+4] != "    " {
			c.Close()
			return PatronInformationResponse{}, fmt.Errorf("invalid SIP2 patron count")
		}
	}
	return PatronInformationResponse{PatronStatusResponse{response, raw[2:16], raw[16:19]}, counts[0:4], counts[4:8], counts[8:12], counts[12:16], counts[16:20], counts[20:24]}, nil
}

func (c *Client) SCStatus(ctx context.Context, r SCStatusRequest) (SCStatusResponse, error) {
	if !strings.ContainsRune("012", rune(r.StatusCode)) || !digits(r.MaxPrintWidth, 3) || !version(r.ProtocolVersion) {
		return SCStatusResponse{}, fmt.Errorf("invalid SIP2 SC status")
	}
	raw, response, err := c.extendedResponse(ctx, "99", string(r.StatusCode)+r.MaxPrintWidth+r.ProtocolVersion, "98", "YYYYYYDDDDDD", 4, nil, "AO", "BX")
	if err != nil {
		return SCStatusResponse{}, err
	}
	if !version(raw[32:36]) {
		c.Close()
		return SCStatusResponse{}, fmt.Errorf("invalid SIP2 protocol version")
	}
	return SCStatusResponse{response, raw[2] == 'Y', raw[3] == 'Y', raw[4] == 'Y', raw[5] == 'Y', raw[6] == 'Y', raw[7] == 'Y', raw[8:11], raw[11:14], raw[32:36]}, nil
}

func (c *Client) FeePaid(ctx context.Context, r FeePaidRequest) (FeePaidResponse, error) {
	if !validDate(r.TransactionDate) || !digits(r.FeeType, 2) || r.FeeType == "00" || !digits(r.PaymentType, 2) || len(r.CurrencyType) != 3 || strings.ContainsAny(r.CurrencyType, "|\r\n\x00") {
		return FeePaidResponse{}, fmt.Errorf("invalid SIP2 fee payment fields")
	}
	raw, response, err := c.message(ctx, "37", r.TransactionDate+r.FeeType+r.PaymentType+r.CurrencyType, "38", "Y", []field{{"BV", r.FeeAmount, true}, {"AO", r.InstitutionID, true}, {"AA", r.PatronID, true}, {"AC", r.TerminalPassword, false}, {"AD", r.PatronPassword, false}, {"CG", r.FeeIdentifier, false}, {"BK", r.TransactionID, false}}, "AO", "AA")
	if err != nil {
		return FeePaidResponse{}, err
	}
	return FeePaidResponse{response, raw[2] == 'Y'}, nil
}

func (c *Client) Hold(ctx context.Context, r HoldRequest) (HoldResponse, error) {
	if !strings.ContainsRune("+-*", rune(r.HoldMode)) || !validDate(r.TransactionDate) || (r.ExpirationDate != "" && !validDate(r.ExpirationDate)) || (r.HoldType != "" && (len(r.HoldType) != 1 || r.HoldType[0] < '1' || r.HoldType[0] > '9')) || (r.ItemID == "" && r.TitleID == "") {
		return HoldResponse{}, fmt.Errorf("invalid SIP2 hold fields")
	}
	raw, response, err := c.message(ctx, "15", string(r.HoldMode)+r.TransactionDate, "16", "0Y", []field{{"BW", r.ExpirationDate, false}, {"BS", r.PickupLocation, false}, {"BY", r.HoldType, false}, {"AO", r.InstitutionID, true}, {"AA", r.PatronID, true}, {"AD", r.PatronPassword, false}, {"AB", r.ItemID, false}, {"AJ", r.TitleID, false}, {"AC", r.TerminalPassword, false}, optionalBool("BO", r.FeeAcknowledged)}, "AO", "AA")
	if err != nil {
		return HoldResponse{}, err
	}
	return HoldResponse{response, raw[2] == '1', raw[3] == 'Y'}, nil
}

func (c *Client) Renew(ctx context.Context, r RenewRequest) (RenewResponse, error) {
	if !validDate(r.TransactionDate) || !validDate(r.DueDate) || (r.ItemID == "" && r.TitleID == "") {
		return RenewResponse{}, fmt.Errorf("invalid SIP2 renew fields")
	}
	raw, response, err := c.message(ctx, "29", yn(r.ThirdPartyAllowed)+yn(r.NoBlock)+r.TransactionDate+r.DueDate, "30", "0YUU", []field{{"AO", r.InstitutionID, true}, {"AA", r.PatronID, true}, {"AD", r.PatronPassword, false}, {"AB", r.ItemID, false}, {"AJ", r.TitleID, false}, {"AC", r.TerminalPassword, false}, {"CH", r.ItemProperties, false}, optionalBool("BO", r.FeeAcknowledged)}, "AO", "AA", "AB", "AJ", "AH")
	if err != nil {
		return RenewResponse{}, err
	}
	return RenewResponse{response, raw[2] == '1', raw[3] == 'Y', raw[4], raw[5]}, nil
}

func (c *Client) RenewAll(ctx context.Context, r RenewAllRequest) (RenewAllResponse, error) {
	if !validDate(r.TransactionDate) {
		return RenewAllResponse{}, fmt.Errorf("invalid SIP2 date")
	}
	raw, response, err := c.message(ctx, "65", r.TransactionDate, "66", "0DDDDDDDD", []field{{"AO", r.InstitutionID, true}, {"AA", r.PatronID, true}, {"AD", r.PatronPassword, false}, {"AC", r.TerminalPassword, false}, optionalBool("BO", r.FeeAcknowledged)}, "AO")
	if err != nil {
		return RenewAllResponse{}, err
	}
	return RenewAllResponse{response, raw[2] == '1', raw[3:7], raw[7:11]}, nil
}
