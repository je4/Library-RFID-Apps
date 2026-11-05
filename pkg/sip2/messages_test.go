package sip2

import (
	"bufio"
	"context"
	"net"
	"testing"
)

const testDate = "20261105    123456"

func TestMessagePairs(t *testing.T) {
	yes := true
	tests := []struct {
		name, request, response string
		call                    func(*Client) error
	}{
		{"checkout", "11YN" + testDate + testDate + "AOlib|AApat|ABbook|AC|BOY|", "121NYU" + testDate + "AOlib|AApat|ABbook|AJtitle|AHtomorrow|AFone|AFtwo|", func(c *Client) error {
			r, err := c.Checkout(context.Background(), CheckoutRequest{RenewalPolicy: true, TransactionDate: testDate, DueDate: testDate, InstitutionID: "lib", PatronID: "pat", ItemID: "book", FeeAcknowledged: &yes})
			if err == nil && (!r.OK || r.RenewalOK || r.Desensitize != 'U' || len(r.Fields["AF"]) != 2) {
				t.Error("incorrect checkout response", r)
			}
			return err
		}},
		{"checkin", "09N" + testDate + testDate + "APdesk|AOlib|ABbook|AC|", "101YUN" + testDate + "AOlib|ABbook|AQshelf|", func(c *Client) error {
			r, err := c.Checkin(context.Background(), CheckinRequest{TransactionDate: testDate, ReturnDate: testDate, CurrentLocation: "desk", InstitutionID: "lib", ItemID: "book"})
			if err == nil && (!r.OK || !r.Resensitize || r.Alert || r.MagneticMedia != 'U') {
				t.Error("incorrect checkin response", r)
			}
			return err
		}},
		{"end", "35" + testDate + "AOlib|AApat|", "36N" + testDate + "AOlib|AApat|", func(c *Client) error {
			r, err := c.EndSession(context.Background(), EndSessionRequest{TransactionDate: testDate, InstitutionID: "lib", PatronID: "pat"})
			if err == nil && r.Ended {
				t.Error("unexpected end session success")
			}
			return err
		}},
		{"update", "19" + testDate + "AOlib|ABbook|CHdata|", "201" + testDate + "ABbook|", func(c *Client) error {
			r, err := c.ItemStatusUpdate(context.Background(), ItemStatusUpdateRequest{TransactionDate: testDate, InstitutionID: "lib", ItemID: "book", ItemProperties: "data"})
			if err == nil && !r.PropertiesOK {
				t.Error("unexpected update failure")
			}
			return err
		}},
		{"item information", "17" + testDate + "AOlib|ABbook|", "18030101" + testDate + "ABbook|AJtitle|", func(c *Client) error {
			r, err := c.ItemInformation(context.Background(), ItemInformationRequest{TransactionDate: testDate, InstitutionID: "lib", ItemID: "book"})
			if err == nil && (r.CirculationStatus != "03" || r.SecurityMarker != "01" || r.FeeType != "01") {
				t.Error("incorrect item information", r)
			}
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := make(chan string, 1)
			c := testClient(t, func(conn net.Conn) {
				s, _ := bufio.NewReader(conn).ReadString('\r')
				requests <- s
				conn.Write([]byte(tt.response + "\r"))
			})
			if err := tt.call(c); err != nil {
				t.Fatal(err)
			}
			if got := <-requests; got != tt.request+"\r" {
				t.Fatalf("request %q, want %q", got, tt.request+"\r")
			}
		})
	}
}

func TestMessageValidation(t *testing.T) {
	for _, raw := range []string{"36X" + testDate + "AOlib|AApat|", "36Ybad", "36Y" + testDate + "AOlib|", "36Y" + testDate + "AOlib|AApat"} {
		if _, err := parseResponse(raw, "36", "Y", "AO", "AA"); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
	if _, err := encodeFields(field{"AA", "bad|value", true}); err == nil {
		t.Fatal("accepted delimiter")
	}
	if validDate("20261105    12345x") {
		t.Fatal("accepted invalid date")
	}
}

func TestAdditionalMessagePairs(t *testing.T) {
	status := "              001" + testDate + "AOlib|AApat|AEname|AFone|AFtwo|"
	tests := []struct {
		name, request, response string
		call                    func(*Client) error
	}{
		{"patron status", "23001" + testDate + "AOlib|AApat|AC|AD|", "24" + status, func(c *Client) error {
			r, err := c.PatronStatus(context.Background(), PatronStatusRequest{Language: "001", TransactionDate: testDate, InstitutionID: "lib", PatronID: "pat"})
			if err == nil && (r.Language != "001" || len(r.Fields["AF"]) != 2) {
				t.Error("incorrect patron status", r)
			}
			return err
		}},
		{"block", "01Y" + testDate + "AOlib|ALlost|AApat|AC|", "24" + status, func(c *Client) error {
			_, err := c.BlockPatron(context.Background(), BlockPatronRequest{CardRetained: true, TransactionDate: testDate, InstitutionID: "lib", BlockedCardMessage: "lost", PatronID: "pat"})
			return err
		}},
		{"enable", "25" + testDate + "AOlib|AApat|", "26" + status, func(c *Client) error {
			_, err := c.PatronEnable(context.Background(), PatronEnableRequest{TransactionDate: testDate, InstitutionID: "lib", PatronID: "pat"})
			return err
		}},
		{"information", "63001" + testDate + "Y         AOlib|AApat|", "64" + "              001" + testDate + "0002    0000000000000000AOlib|AApat|AEname|ASone|AStwo|", func(c *Client) error {
			r, err := c.PatronInformation(context.Background(), PatronInformationRequest{Language: "001", TransactionDate: testDate, Summary: "Y         ", InstitutionID: "lib", PatronID: "pat"})
			if err == nil && (r.HoldCount != "0002" || r.OverdueCount != "    " || len(r.Fields["AS"]) != 2) {
				t.Error("incorrect information", r)
			}
			return err
		}},
		{"status", "9900802.00", "98YYYYYN030003" + testDate + "2.00AOlib|BXYYYYYYYYYYYYYYYY|", func(c *Client) error {
			r, err := c.SCStatus(context.Background(), SCStatusRequest{StatusCode: '0', MaxPrintWidth: "080", ProtocolVersion: "2.00"})
			if err == nil && (!r.Online || r.OfflineOK || r.TimeoutPeriod != "030" || r.ProtocolVersion != "2.00") {
				t.Error("incorrect status", r)
			}
			return err
		}},
		{"fee", "37" + testDate + "0100USDBV1.25|AOlib|AApat|", "38N" + testDate + "AOlib|AApat|AFrefused|", func(c *Client) error {
			r, err := c.FeePaid(context.Background(), FeePaidRequest{TransactionDate: testDate, FeeType: "01", PaymentType: "00", CurrencyType: "USD", FeeAmount: "1.25", InstitutionID: "lib", PatronID: "pat"})
			if err == nil && (r.PaymentAccepted || r.Fields["AF"][0] != "refused") {
				t.Error("incorrect payment", r)
			}
			return err
		}},
		{"hold", "15+" + testDate + "AOlib|AApat|ABbook|", "161Y" + testDate + "AOlib|AApat|ABbook|", func(c *Client) error {
			r, err := c.Hold(context.Background(), HoldRequest{HoldMode: '+', TransactionDate: testDate, InstitutionID: "lib", PatronID: "pat", ItemID: "book"})
			if err == nil && (!r.OK || !r.Available) {
				t.Error("incorrect hold", r)
			}
			return err
		}},
		{"renew", "29NN" + testDate + testDate + "AOlib|AApat|ABbook|", "301NYU" + testDate + "AOlib|AApat|ABbook|AJtitle|AHtomorrow|", func(c *Client) error {
			r, err := c.Renew(context.Background(), RenewRequest{TransactionDate: testDate, DueDate: testDate, InstitutionID: "lib", PatronID: "pat", ItemID: "book"})
			if err == nil && !r.OK {
				t.Error("incorrect renew", r)
			}
			return err
		}},
		{"renew all", "65" + testDate + "AOlib|AApat|", "66100020001" + testDate + "AOlib|BMone|BMtwo|BNthree|", func(c *Client) error {
			r, err := c.RenewAll(context.Background(), RenewAllRequest{TransactionDate: testDate, InstitutionID: "lib", PatronID: "pat"})
			if err == nil && (r.RenewedCount != "0002" || len(r.Fields["BM"]) != 2) {
				t.Error("incorrect renew all", r)
			}
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := make(chan string, 1)
			c := testClient(t, func(conn net.Conn) {
				s, _ := bufio.NewReader(conn).ReadString('\r')
				requests <- s
				conn.Write([]byte(tt.response + "\r"))
			})
			if err := tt.call(c); err != nil {
				t.Fatal(err)
			}
			if got := <-requests; got != tt.request+"\r" {
				t.Fatalf("request %q, want %q", got, tt.request+"\r")
			}
		})
	}
}

func TestAdditionalMessageValidation(t *testing.T) {
	for _, raw := range []string{"24" + "N             001" + testDate + "AOlib|AApat|AEname|", "24" + "              001" + testDate + "AOlib|AApat|", "24" + "              001" + testDate + "AOlib|AApat|AEname|BLX|"} {
		if _, err := parseResponse(raw, "24", "SSSSSSSSSSSSSSDDD", "AO", "AA", "AE"); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
	if !validDate("20261105   Z123456") {
		t.Error("rejected UTC date")
	}
	c := &Client{}
	if _, err := c.Hold(context.Background(), HoldRequest{HoldMode: '+', TransactionDate: testDate}); err == nil {
		t.Error("accepted hold without item or title")
	}
	if _, err := c.Renew(context.Background(), RenewRequest{TransactionDate: testDate, DueDate: testDate}); err == nil {
		t.Error("accepted renewal without item or title")
	}
	if _, err := c.PatronInformation(context.Background(), PatronInformationRequest{Language: "001", TransactionDate: testDate, Summary: "YY        "}); err == nil {
		t.Error("accepted multiple summary categories")
	}
}
