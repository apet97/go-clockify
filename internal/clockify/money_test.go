package clockify

import (
	"encoding/json"
	"testing"
)

func TestRateUnmarshal(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want Rate
		ptr  bool // true if we expect a non-nil pointer to a Rate
	}{
		{name: "object", in: `{"amount":4321,"currency":"EUR"}`, want: Rate{Amount: 4321, Currency: "EUR"}, ptr: true},
		{name: "object_float_amount", in: `{"amount":4321.0,"currency":"EUR"}`, want: Rate{Amount: 4321, Currency: "EUR"}, ptr: true},
		{name: "object_zero", in: `{"amount":0,"currency":"USD"}`, want: Rate{Amount: 0, Currency: "USD"}, ptr: true},
		{name: "object_no_currency", in: `{"amount":150}`, want: Rate{Amount: 150}, ptr: true},
		{name: "string_default_sentinel", in: `"##default"`, want: Rate{Inherited: true}, ptr: true},
		{name: "null", in: `null`, ptr: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var holder struct {
				Rate *Rate `json:"rate"`
			}
			body := []byte(`{"rate":` + tc.in + `}`)
			if err := json.Unmarshal(body, &holder); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if !tc.ptr {
				if holder.Rate != nil {
					t.Fatalf("expected nil rate for %q, got %+v", tc.in, holder.Rate)
				}
				return
			}
			if holder.Rate == nil {
				t.Fatalf("expected non-nil rate for %q", tc.in)
			}
			if *holder.Rate != tc.want {
				t.Fatalf("got %+v want %+v", *holder.Rate, tc.want)
			}
		})
	}
}

func TestRateUnmarshalRejectsUnknownString(t *testing.T) {
	var r Rate
	if err := json.Unmarshal([]byte(`"surprise"`), &r); err == nil {
		t.Fatal("expected error for unknown rate string")
	}
}

func TestRateMarshalRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		in   Rate
		want string
	}{
		{name: "amount_currency", in: Rate{Amount: 4321, Currency: "EUR"}, want: `{"amount":4321,"currency":"EUR"}`},
		{name: "inherited", in: Rate{Inherited: true}, want: `"##default"`},
		{name: "zero_no_currency", in: Rate{Amount: 0}, want: `{"amount":0}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(&tc.in)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
	t.Run("nil_marshals_null", func(t *testing.T) {
		var r *Rate
		got, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("marshal nil: %v", err)
		}
		if string(got) != "null" {
			t.Fatalf("got %s want null", got)
		}
	})
}

func TestRateDecimalString(t *testing.T) {
	cases := []struct {
		cents int64
		want  string
	}{
		{0, "0.00"},
		{1, "0.01"},
		{99, "0.99"},
		{100, "1.00"},
		{4321, "43.21"},
		{20000, "200.00"},
		{123400, "1234.00"},
		{-250, "-2.50"},
	}
	for _, tc := range cases {
		r := &Rate{Amount: tc.cents}
		if got := r.DecimalString(); got != tc.want {
			t.Errorf("DecimalString(%d)=%q want %q", tc.cents, got, tc.want)
		}
	}
	var nilRate *Rate
	if got := nilRate.DecimalString(); got != "" {
		t.Errorf("nil DecimalString=%q want empty", got)
	}
}

func TestRateDisplay(t *testing.T) {
	cases := []struct {
		name string
		r    *Rate
		want string
	}{
		{"eur", &Rate{Amount: 4321, Currency: "EUR"}, "€43.21 EUR"},
		{"usd", &Rate{Amount: 4321, Currency: "USD"}, "$43.21 USD"},
		{"gbp", &Rate{Amount: 100, Currency: "GBP"}, "£1.00 GBP"},
		{"unknown_ccy", &Rate{Amount: 250, Currency: "ZWL"}, "2.50 ZWL"},
		{"no_currency", &Rate{Amount: 250}, "2.50"},
		{"inherited", &Rate{Inherited: true}, "(inherited)"},
		{"nil", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.r.Display(); got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestCentsFromReportNumber(t *testing.T) {
	cases := []struct {
		in   float64
		want int64
	}{
		{0, 0},
		{4321, 4321},
		{4321.0, 4321},
		{4321.4, 4321},
		{4321.5, 4322},
		{4321.6, 4322},
		{-2.5, -3},
	}
	for _, tc := range cases {
		if got := CentsFromReportNumber(tc.in); got != tc.want {
			t.Errorf("CentsFromReportNumber(%v)=%d want %d", tc.in, got, tc.want)
		}
	}
}

func TestProjectDecodesMembershipsAndRate(t *testing.T) {
	// Shape captured live in fixtures/rates-and-reports/project-get.json
	// (probe-lab repo, 2026-05-12).
	body := []byte(`{
		"id": "p1",
		"name": "mcp-probe-proj",
		"hourlyRate": {"amount": 0, "currency": "EUR"},
		"costRate": null,
		"memberships": [
			{
				"userId": "u1",
				"hourlyRate": null,
				"costRate": null,
				"targetId": "p1",
				"membershipType": "PROJECT",
				"membershipStatus": "ACTIVE"
			}
		]
	}`)
	var p Project
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.HourlyRate == nil {
		t.Fatal("expected non-nil hourly rate")
	}
	if p.HourlyRate.Currency != "EUR" || p.HourlyRate.Amount != 0 {
		t.Errorf("hourly rate = %+v want {Amount:0 Currency:EUR}", *p.HourlyRate)
	}
	if p.CostRate != nil {
		t.Errorf("expected nil cost rate, got %+v", *p.CostRate)
	}
	if len(p.Memberships) != 1 {
		t.Fatalf("expected 1 membership, got %d", len(p.Memberships))
	}
	m := p.Memberships[0]
	if m.UserID != "u1" || m.MembershipType != "PROJECT" || m.MembershipStatus != "ACTIVE" {
		t.Errorf("membership shape wrong: %+v", m)
	}
	if m.HourlyRate != nil || m.CostRate != nil {
		t.Errorf("expected nil membership rates, got hourly=%+v cost=%+v", m.HourlyRate, m.CostRate)
	}
}

func TestWorkspaceDecodesMembershipRates(t *testing.T) {
	// Shape captured live in fixtures/rates-and-reports/workspaces-list.json.
	body := []byte(`[{
		"id": "w1",
		"name": "W",
		"hourlyRate": {"amount": 150, "currency": "EUR"},
		"costRate":   {"amount": 75,  "currency": "EUR"},
		"memberships": [
			{
				"userId": "u1",
				"hourlyRate": {"amount": 20000, "currency": "EUR"},
				"costRate":   {"amount": 999900, "currency": "EUR"},
				"membershipType": "WORKSPACE",
				"membershipStatus": "ACTIVE"
			},
			{
				"userId": "u2",
				"hourlyRate": null,
				"costRate":   null,
				"membershipType": "WORKSPACE",
				"membershipStatus": "INACTIVE"
			}
		]
	}]`)
	var ws []Workspace
	if err := json.Unmarshal(body, &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(ws) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(ws))
	}
	w := ws[0]
	if w.HourlyRate == nil || w.HourlyRate.Amount != 150 {
		t.Errorf("workspace hourly rate = %+v want amount=150", w.HourlyRate)
	}
	if w.CostRate == nil || w.CostRate.Amount != 75 {
		t.Errorf("workspace cost rate = %+v want amount=75", w.CostRate)
	}
	if len(w.Memberships) != 2 {
		t.Fatalf("expected 2 memberships, got %d", len(w.Memberships))
	}
	if w.Memberships[0].HourlyRate == nil || w.Memberships[0].HourlyRate.Amount != 20000 {
		t.Errorf("member 0 hourly = %+v", w.Memberships[0].HourlyRate)
	}
	if w.Memberships[1].HourlyRate != nil {
		t.Errorf("member 1 hourly should be nil, got %+v", w.Memberships[1].HourlyRate)
	}
}
