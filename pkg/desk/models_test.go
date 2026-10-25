package desk

import (
	"testing"
)

func TestViewModels(t *testing.T) {
	deskData := DeskData{}
	if deskData.IsLoan() {
		t.Errorf("DeskData.IsLoan() = true, want false")
	}
	if deskData.IsEncode() {
		t.Errorf("DeskData.IsEncode() = true, want false")
	}

	loanData := LoanData{}
	if !loanData.IsLoan() {
		t.Errorf("LoanData.IsLoan() = false, want true")
	}
	if loanData.IsEncode() {
		t.Errorf("LoanData.IsEncode() = true, want false")
	}

	encodeData := EncodeData{
		DefaultISIL:  "CH-000001",
		DefaultAFI:   "07",
		DefaultParts: 1,
	}
	if encodeData.IsLoan() {
		t.Errorf("EncodeData.IsLoan() = true, want false")
	}
	if !encodeData.IsEncode() {
		t.Errorf("EncodeData.IsEncode() = false, want true")
	}
	if encodeData.DefaultISIL != "CH-000001" {
		t.Errorf("EncodeData.DefaultISIL = %q, want %q", encodeData.DefaultISIL, "CH-000001")
	}
	if encodeData.DefaultAFI != "07" {
		t.Errorf("EncodeData.DefaultAFI = %q, want %q", encodeData.DefaultAFI, "07")
	}
}
