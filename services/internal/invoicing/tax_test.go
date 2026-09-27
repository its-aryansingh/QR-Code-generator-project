package invoicing

import (
	"testing"
	"time"
)

var seller = Seller{LegalName: "QRit Technologies", GSTIN: "09AAACQ1234F1Z5", StateCode: "09", SAC: "998315", LUTRef: "AD090326000123X"}

func TestCompute(t *testing.T) {
	lines := []Line{{Description: "Enterprise plan", Qty: 1, UnitMinor: 60000000, AmountMinor: 60000000}} // ₹6,00,000
	igst, err := Compute(seller, Buyer{GSTIN: "27AAACM1234F1Z1", Country: "IN"}, "INR", lines)
	if err != nil || igst.TaxMode != ModeIGST || igst.IGSTMinor != 10800000 || igst.TotalMinor != 70800000 ||
		igst.PlaceOfSupply != "27 - Maharashtra" {
		t.Fatalf("IGST: %+v %v", igst, err)
	}
	intra, _ := Compute(seller, Buyer{StateCode: "09", Country: "IN"}, "INR", lines)
	if intra.TaxMode != ModeCGSTSGST || intra.CGSTMinor != 5400000 || intra.SGSTMinor != 5400000 || intra.IGSTMinor != 0 {
		t.Fatalf("CGST+SGST: %+v", intra)
	}
	exp, _ := Compute(seller, Buyer{Country: "US"}, "USD", []Line{{AmountMinor: 1200000}})
	if exp.TaxMode != ModeExportLUT || exp.TotalMinor != 1200000 || exp.Endorsement == nil ||
		*exp.Endorsement != ExportEndorsement+" (LUT AD090326000123X)" || exp.PlaceOfSupply != "96 - Other Countries" {
		t.Fatalf("export: %+v", exp)
	}
	inr, _ := Compute(seller, Buyer{Country: "AE"}, "INR", []Line{{AmountMinor: 100}})
	if inr.TaxMode != ModeIGST || inr.IGSTMinor != 18 {
		t.Fatalf("foreign buyer paying INR: %+v", inr)
	}
	if _, err := Compute(seller, Buyer{Country: "IN"}, "INR", lines); err == nil {
		t.Fatal("missing state accepted")
	}
	// Half-up rounding on paise.
	r, _ := Compute(seller, Buyer{StateCode: "29"}, "INR", []Line{{AmountMinor: 1003}})
	if r.IGSTMinor != 181 { // 180.54 → 181
		t.Fatalf("rounding: %+v", r)
	}
}

func d(s string) time.Time { t, _ := time.Parse("2006-01-02", s); return t }

func TestPeriods(t *testing.T) {
	ps := Periods(d("2026-04-15"), d("2027-04-15"), d("2026-10-20"), "quarter")
	if len(ps) != 3 || !ps[0].End.Equal(d("2026-07-14")) || !ps[2].Start.Equal(d("2026-10-15")) {
		t.Fatalf("quarterly: %+v", ps)
	}
	if ps := Periods(d("2026-04-01"), d("2027-04-01"), d("2026-03-31"), "year"); len(ps) != 0 {
		t.Fatal("not started")
	}
	ps = Periods(d("2026-01-31"), d("2026-04-15"), d("2026-12-01"), "month")
	last := ps[len(ps)-1]
	if len(ps) != 3 || !last.End.Equal(d("2026-04-14")) {
		t.Fatalf("monthly with short tail: %+v", ps)
	}
	if got := Prorate(3000, Period{Start: d("2026-04-01"), End: d("2026-04-15")}, "month"); got != 1500 {
		t.Fatalf("prorate: %d", got)
	}
	if got := Prorate(3000, Period{Start: d("2026-04-01"), End: d("2026-04-30")}, "month"); got != 3000 {
		t.Fatalf("full period: %d", got)
	}
}

func TestMoneyAndWords(t *testing.T) {
	cases := map[string]string{
		FormatMoney(70800000, "INR"):       "₹7,08,000.00",
		FormatMoney(123456789, "INR"):      "₹12,34,567.89",
		FormatMoney(99, "INR"):             "₹0.99",
		FormatMoney(123456789, "USD"):      "$1,234,567.89",
		AmountInWords(70800000, "INR"):     "Rupees Seven Lakh Eight Thousand Only",
		AmountInWords(1234567899, "INR"):   "Rupees One Crore Twenty Three Lakh Forty Five Thousand Six Hundred Seventy Eight and Ninety Nine Paise Only",
		AmountInWords(1200050, "USD"):      "US Dollars Twelve Thousand and Fifty Cents Only",
		AmountInWords(100000000000, "INR"): "Rupees One Hundred Crore Only",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("got %q want %q", got, want)
		}
	}
}
