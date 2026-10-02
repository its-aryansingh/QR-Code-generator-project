// Package invoicing issues GST-compliant invoices for enterprise contracts: tax mode by
// supplier and recipient location (CGST+SGST, IGST or export under LUT), consecutive
// numbering per financial year (next_invoice_number in the database), billing periods,
// seat true-up, dunning and amounts in words.
package invoicing

import (
	"fmt"
	"strings"
	"time"
)

// GST rate for SaaS (SAC 9983xx / 997331): 18%, split 9% + 9% within a state.
const (
	IGSTPercent = 18
	CGSTPercent = 9
	SGSTPercent = 9
)

// Tax modes (invoices.tax_mode).
const (
	ModeCGSTSGST  = "cgst_sgst"
	ModeIGST      = "igst"
	ModeExportLUT = "export_lut"
)

// ExportEndorsement is the declaration GST rules require on zero-rated export invoices.
const ExportEndorsement = "Supply meant for export under Bond or Letter of Undertaking without payment of Integrated Tax"

// States maps GST state codes to names (codes as printed on GSTINs).
var States = map[string]string{
	"01": "Jammu and Kashmir", "02": "Himachal Pradesh", "03": "Punjab", "04": "Chandigarh", "05": "Uttarakhand",
	"06": "Haryana", "07": "Delhi", "08": "Rajasthan", "09": "Uttar Pradesh", "10": "Bihar", "11": "Sikkim",
	"12": "Arunachal Pradesh", "13": "Nagaland", "14": "Manipur", "15": "Mizoram", "16": "Tripura", "17": "Meghalaya",
	"18": "Assam", "19": "West Bengal", "20": "Jharkhand", "21": "Odisha", "22": "Chhattisgarh", "23": "Madhya Pradesh",
	"24": "Gujarat", "26": "Dadra and Nagar Haveli and Daman and Diu", "27": "Maharashtra", "29": "Karnataka", "30": "Goa",
	"31": "Lakshadweep", "32": "Kerala", "33": "Tamil Nadu", "34": "Puducherry", "35": "Andaman and Nicobar Islands",
	"36": "Telangana", "37": "Andhra Pradesh", "38": "Ladakh", "97": "Other Territory",
}

// OutsideIndia is the place-of-supply code for recipients abroad.
const OutsideIndia = "96"

// Seller is the supplier block printed on every invoice.
type Seller struct {
	LegalName string `json:"legal_name"`
	GSTIN     string `json:"gstin"`
	Address   string `json:"address"`
	StateCode string `json:"state_code"`
	SAC       string `json:"sac"`
	LUTRef    string `json:"lut_ref,omitempty"`
	UPIVPA    string `json:"upi_vpa,omitempty"`
}

// Buyer is the recipient block.
type Buyer struct {
	LegalName string `json:"legal_name"`
	GSTIN     string `json:"gstin,omitempty"`
	Address   string `json:"address"`
	StateCode string `json:"state_code,omitempty"`
	Country   string `json:"country"`
	Email     string `json:"email,omitempty"`
	PONumber  string `json:"po_number,omitempty"`
}

// Line is one invoice line (amounts in minor units of the invoice currency).
type Line struct {
	Description string `json:"description"`
	SAC         string `json:"sac"`
	Qty         int64  `json:"qty"`
	UnitMinor   int64  `json:"unit_minor"`
	AmountMinor int64  `json:"amount_minor"`
}

// Totals is the computed tax breakup.
type Totals struct {
	TaxMode       string  `json:"tax_mode"`
	PlaceOfSupply string  `json:"place_of_supply"`
	SubtotalMinor int64   `json:"subtotal_minor"`
	CGSTMinor     int64   `json:"cgst_minor"`
	SGSTMinor     int64   `json:"sgst_minor"`
	IGSTMinor     int64   `json:"igst_minor"`
	TotalMinor    int64   `json:"total_minor"`
	Endorsement   *string `json:"endorsement,omitempty"`
}

// BuyerState returns the recipient's state code: explicit, else the GSTIN's first two digits.
func BuyerState(b Buyer) string {
	if b.StateCode != "" {
		return b.StateCode
	}
	if len(b.GSTIN) >= 2 {
		return b.GSTIN[:2]
	}
	return ""
}

func pct(amount int64, p int64) int64 { return (amount*p + 50) / 100 } // round half up

// Compute decides the tax mode and totals.
//   - Recipient in India, same state as the supplier: CGST 9% + SGST 9%.
//   - Recipient in India, another state (or unknown state): IGST 18%.
//   - Recipient outside India paid in foreign currency: export of services under LUT,
//     zero-rated, with the mandatory endorsement.
//   - Recipient outside India paid in INR: not an export of services; IGST 18%.
func Compute(s Seller, b Buyer, currency string, lines []Line) (Totals, error) {
	var t Totals
	for _, l := range lines {
		if l.AmountMinor < 0 {
			return t, fmt.Errorf("line %q has a negative amount", l.Description)
		}
		t.SubtotalMinor += l.AmountMinor
	}
	country := strings.ToUpper(strings.TrimSpace(b.Country))
	if country == "" {
		country = "IN"
	}
	switch {
	case country != "IN" && currency != "INR":
		t.TaxMode, t.PlaceOfSupply = ModeExportLUT, OutsideIndia+" - Other Countries"
		e := ExportEndorsement
		if s.LUTRef != "" {
			e += " (LUT " + s.LUTRef + ")"
		}
		t.Endorsement = &e
	case country != "IN":
		t.TaxMode, t.PlaceOfSupply = ModeIGST, OutsideIndia+" - Other Countries"
		t.IGSTMinor = pct(t.SubtotalMinor, IGSTPercent)
	default:
		st := BuyerState(b)
		if _, ok := States[st]; !ok {
			return t, fmt.Errorf("the buyer's GST state code is missing or invalid (%q); set billing_address.state_code or a GSTIN", st)
		}
		t.PlaceOfSupply = st + " - " + States[st]
		if st == s.StateCode {
			t.TaxMode = ModeCGSTSGST
			t.CGSTMinor, t.SGSTMinor = pct(t.SubtotalMinor, CGSTPercent), pct(t.SubtotalMinor, SGSTPercent)
		} else {
			t.TaxMode = ModeIGST
			t.IGSTMinor = pct(t.SubtotalMinor, IGSTPercent)
		}
	}
	t.TotalMinor = t.SubtotalMinor + t.CGSTMinor + t.SGSTMinor + t.IGSTMinor
	return t, nil
}

// Period is one billing period [Start, End] (inclusive end date).
type Period struct {
	Start, End time.Time
}

func step(interval string) (years, months int) {
	switch interval {
	case "month":
		return 0, 1
	case "quarter":
		return 0, 3
	default:
		return 1, 0
	}
}

// Periods lists the contract's billing periods that have started by today (billed in advance).
func Periods(startsOn, endsOn, today time.Time, interval string) []Period {
	y, m := step(interval)
	var out []Period
	for k := 0; k < 1200; k++ {
		ps := startsOn.AddDate(y*k, m*k, 0)
		if !ps.Before(endsOn) || ps.After(today) {
			break
		}
		pe := startsOn.AddDate(y*(k+1), m*(k+1), -1)
		if !pe.Before(endsOn) {
			pe = endsOn.AddDate(0, 0, -1)
		}
		out = append(out, Period{Start: ps, End: pe})
	}
	return out
}

// Prorate scales a full-period amount to a shorter final period (by days, half-up).
func Prorate(amount int64, p Period, interval string) int64 {
	y, m := step(interval)
	full := p.Start.AddDate(y, m, 0).Sub(p.Start).Hours() / 24
	days := p.End.Sub(p.Start).Hours()/24 + 1
	if days >= full {
		return amount
	}
	return int64(float64(amount)*days/full + 0.5)
}

// FormatMoney renders minor units, with Indian digit grouping for INR (₹6,00,000.00).
func FormatMoney(minor int64, currency string) string {
	neg := minor < 0
	if neg {
		minor = -minor
	}
	whole, frac := minor/100, minor%100
	digits := fmt.Sprintf("%d", whole)
	var grouped string
	if currency == "INR" && len(digits) > 3 {
		head, tail := digits[:len(digits)-3], digits[len(digits)-3:]
		var parts []string
		for len(head) > 2 {
			parts = append([]string{head[len(head)-2:]}, parts...)
			head = head[:len(head)-2]
		}
		if head != "" {
			parts = append([]string{head}, parts...)
		}
		grouped = strings.Join(parts, ",") + "," + tail
	} else {
		for i, c := range digits {
			if i > 0 && (len(digits)-i)%3 == 0 {
				grouped += ","
			}
			grouped += string(c)
		}
	}
	sym := map[string]string{"INR": "₹", "USD": "$", "EUR": "€", "GBP": "£"}[currency]
	if sym == "" {
		sym = currency + " "
	}
	s := fmt.Sprintf("%s%s.%02d", sym, grouped, frac)
	if neg {
		s = "-" + s
	}
	return s
}

var ones = []string{"", "One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine", "Ten", "Eleven", "Twelve",
	"Thirteen", "Fourteen", "Fifteen", "Sixteen", "Seventeen", "Eighteen", "Nineteen"}
var tens = []string{"", "", "Twenty", "Thirty", "Forty", "Fifty", "Sixty", "Seventy", "Eighty", "Ninety"}

func below100(n int64) string {
	if n < 20 {
		return ones[n]
	}
	if n%10 == 0 {
		return tens[n/10]
	}
	return tens[n/10] + " " + ones[n%10]
}

func below1000(n int64) string {
	switch {
	case n == 0:
		return ""
	case n < 100:
		return below100(n)
	case n%100 == 0:
		return ones[n/100] + " Hundred"
	default:
		return ones[n/100] + " Hundred " + below100(n%100)
	}
}

// inWords spells a whole number in the Indian (crore/lakh) or international system.
func inWords(n int64, indian bool) string {
	if n == 0 {
		return "Zero"
	}
	var parts []string
	if indian {
		for _, u := range []struct {
			div  int64
			name string
		}{{10000000, "Crore"}, {100000, "Lakh"}, {1000, "Thousand"}} {
			if n >= u.div {
				q := n / u.div
				if q >= 100 && u.name == "Crore" {
					parts = append(parts, inWords(q, true)+" Crore")
				} else {
					parts = append(parts, below100(q)+" "+u.name)
				}
				n %= u.div
			}
		}
	} else {
		for _, u := range []struct {
			div  int64
			name string
		}{{1000000000, "Billion"}, {1000000, "Million"}, {1000, "Thousand"}} {
			if n >= u.div {
				parts = append(parts, below1000(n/u.div)+" "+u.name)
				n %= u.div
			}
		}
	}
	if n > 0 {
		parts = append(parts, below1000(n))
	}
	return strings.Join(parts, " ")
}

// AmountInWords renders the total as GST invoices customarily print it.
func AmountInWords(minor int64, currency string) string {
	whole, frac := minor/100, minor%100
	switch currency {
	case "INR":
		s := "Rupees " + inWords(whole, true)
		if frac > 0 {
			s += " and " + below100(frac) + " Paise"
		}
		return s + " Only"
	default:
		name := map[string][2]string{"USD": {"US Dollars", "Cents"}, "EUR": {"Euros", "Cents"}, "GBP": {"Pounds Sterling", "Pence"}}[currency]
		if name[0] == "" {
			name = [2]string{currency, "Cents"}
		}
		s := name[0] + " " + inWords(whole, false)
		if frac > 0 {
			s += " and " + below100(frac) + " " + name[1]
		}
		return s + " Only"
	}
}
