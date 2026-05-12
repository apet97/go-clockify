package clockify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

// Rate is Clockify's RateDtoV1 / HourlyRateDtoV1. The Amount is always
// in the minor unit of the currency (cents for EUR/USD/GBP, etc.), as
// an integer. e.g. {amount: 4321, currency: "EUR"} represents €43.21.
//
// Clockify can also serialise a rate as the literal string "##default",
// meaning "inherit from the parent scope (workspace/project/task)."
// We preserve that as Inherited=true with Amount=0 so callers can
// render it as "(inherited)" rather than dropping it.
type Rate struct {
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency,omitempty"`
	Inherited bool   `json:"inherited,omitempty"`
}

func (r *Rate) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		return nil
	}
	// String form: "##default" — sentinel for "inherits from parent."
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		if s == "##default" {
			r.Inherited = true
			return nil
		}
		return fmt.Errorf("clockify: unrecognised rate string %q", s)
	}
	// Object form: {amount, currency}.
	var raw struct {
		Amount   json.Number `json:"amount"`
		Currency string      `json:"currency"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	r.Currency = raw.Currency
	if raw.Amount == "" {
		return nil
	}
	if i, err := raw.Amount.Int64(); err == nil {
		r.Amount = i
		return nil
	}
	// Reports API occasionally returns float (e.g. 4321.0). Round to nearest cent.
	f, err := raw.Amount.Float64()
	if err != nil {
		return fmt.Errorf("clockify: rate amount %q is not numeric: %w", raw.Amount, err)
	}
	r.Amount = int64(math.Round(f))
	return nil
}

func (r *Rate) MarshalJSON() ([]byte, error) {
	if r == nil {
		return []byte("null"), nil
	}
	if r.Inherited {
		return []byte(`"##default"`), nil
	}
	return json.Marshal(struct {
		Amount   int64  `json:"amount"`
		Currency string `json:"currency,omitempty"`
	}{Amount: r.Amount, Currency: r.Currency})
}

// DecimalString renders Amount/100 with exactly two decimal places.
// Negative amounts are preserved verbatim. e.g. 4321 -> "43.21",
// 100 -> "1.00", -250 -> "-2.50".
func (r *Rate) DecimalString() string {
	if r == nil {
		return ""
	}
	return centsToDecimalString(r.Amount)
}

// Display returns the rate in "<symbol><decimal> <CCY>" form for the
// few currencies we know symbols for, falling back to "<decimal> <CCY>".
// Examples: 4321 EUR -> "€43.21 EUR"; 4321 USD -> "$43.21 USD";
// 4321 XYZ -> "43.21 XYZ".
func (r *Rate) Display() string {
	if r == nil {
		return ""
	}
	if r.Inherited {
		return "(inherited)"
	}
	dec := r.DecimalString()
	if dec == "" {
		return ""
	}
	if sym := currencySymbol(r.Currency); sym != "" && r.Currency != "" {
		return fmt.Sprintf("%s%s %s", sym, dec, r.Currency)
	}
	if r.Currency != "" {
		return fmt.Sprintf("%s %s", dec, r.Currency)
	}
	return dec
}

func centsToDecimalString(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	whole := cents / 100
	frac := cents % 100
	return fmt.Sprintf("%s%d.%02d", sign, whole, frac)
}

func currencySymbol(ccy string) string {
	switch ccy {
	case "USD", "AUD", "CAD", "NZD", "SGD", "HKD", "MXN":
		return "$"
	case "EUR":
		return "€"
	case "GBP":
		return "£"
	case "JPY", "CNY":
		return "¥"
	case "INR":
		return "₹"
	case "CHF":
		return "CHF "
	case "RSD":
		return "дин."
	default:
		return ""
	}
}

// CentsFromReportNumber converts a JSON number coming from the reports
// host (always integer-cents-as-float, per findings/rates-and-reports.md)
// to int64 cents using banker's rounding. The raw float is preserved by
// the caller for audit.
func CentsFromReportNumber(v float64) int64 {
	// math.Round is half-away-from-zero; that matches the standard
	// rounding Clockify itself uses for display, so we mirror it.
	return int64(math.Round(v))
}

// ParseRateAmount accepts the JSON number form that Clockify uses in
// rate-update request bodies ("amount":1234 or "amount":1234.0) and
// returns it as int64 cents. Used by tools that need to round-trip a
// caller's rate amount through the Clockify HTTP client.
func ParseRateAmount(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return int64(math.Round(f)), nil
}

// ProjectMembership models the `memberships[]` element returned on
// projects, tasks, workspaces, and users. Per-member rates here
// override their parent scope; nil means "fall through to parent."
type ProjectMembership struct {
	UserID           string `json:"userId"`
	TargetID         string `json:"targetId,omitempty"`
	MembershipType   string `json:"membershipType,omitempty"`
	MembershipStatus string `json:"membershipStatus,omitempty"`
	HourlyRate       *Rate  `json:"hourlyRate,omitempty"`
	CostRate         *Rate  `json:"costRate,omitempty"`
}
