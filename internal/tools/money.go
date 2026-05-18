package tools

import (
	"math"

	"github.com/apet97/go-clockify/internal/clockify"
)

// RateScope identifies which layer of the Clockify rate hierarchy
// supplied the effective rate for a given time entry. Returned to
// the model so it can explain *why* a particular earning amount is
// what it is, rather than only the final number.
type RateScope string

const (
	RateScopeEntry         RateScope = "ENTRY"
	RateScopeTask          RateScope = "TASK"
	RateScopeProjectMember RateScope = "PROJECT_MEMBER"
	RateScopeProject       RateScope = "PROJECT"
	RateScopeWorkspace     RateScope = "WORKSPACE"
	RateScopeNone          RateScope = "NONE"
)

// MoneyView is the LLM-facing envelope for any monetary value the
// MCP surfaces. The model receives all four fields:
//
//   - amount_cents:   int64 minor units (authoritative).
//   - amount_decimal: string, two dp, signed (audit / direct render).
//   - currency:       ISO code from the source.
//   - display:        pre-rendered symbol form (e.g. "€43.21 EUR").
//
// Callers that have only a JSON-number from the reports host should
// use MoneyFromReportNumber; callers with an int-cents value (the
// primary API) should use MoneyFromCents.
type MoneyView struct {
	AmountCents   int64  `json:"amount_cents"`
	AmountDecimal string `json:"amount_decimal"`
	Currency      string `json:"currency,omitempty"`
	Display       string `json:"display,omitempty"`
}

// MoneyFromCents wraps an int64 cents value into a MoneyView.
func MoneyFromCents(cents int64, currency string) MoneyView {
	r := clockify.Rate{Amount: cents, Currency: currency}
	return MoneyView{
		AmountCents:   cents,
		AmountDecimal: r.DecimalString(),
		Currency:      currency,
		Display:       r.Display(),
	}
}

// MoneyFromRate is a nil-tolerant adapter for `*clockify.Rate`. A
// nil rate or an inherited sentinel returns the zero MoneyView so
// callers can decide whether to emit the field at all.
func MoneyFromRate(r *clockify.Rate) MoneyView {
	if r == nil || r.Inherited {
		return MoneyView{}
	}
	return MoneyFromCents(r.Amount, r.Currency)
}

// MoneyFromReportNumber converts a JSON-number cents value coming
// from the reports.api host (always integer-cents-as-float in
// practice) to MoneyView. Banker's rounding to int64 cents.
func MoneyFromReportNumber(v float64, currency string) MoneyView {
	return MoneyFromCents(clockify.CentsFromReportNumber(v), currency)
}

func moneyFromAny(v any, currency string) *MoneyView {
	if v == nil {
		return nil
	}
	switch typed := v.(type) {
	case *MoneyView:
		return typed
	case MoneyView:
		return &typed
	}
	if nested, ok := v.(map[string]any); ok {
		currency = firstNonEmptyString(reportCurrency(nested), currency)
		v = firstPresent(nested, "amount_cents", "amountCents", "value", "amount", "total")
	}
	if n, ok := reportNumber(v); ok {
		return moneyPtr(MoneyFromReportNumber(n, currency))
	}
	return nil
}

func moneySum(a, b *MoneyView) *MoneyView {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if a.Currency != "" && b.Currency != "" && a.Currency != b.Currency {
		return a
	}
	return moneyPtr(MoneyFromCents(a.AmountCents+b.AmountCents, firstNonEmptyString(a.Currency, b.Currency)))
}

func moneyDiff(a, b *MoneyView) *MoneyView {
	if a == nil && b == nil {
		return nil
	}
	if a == nil {
		zero := MoneyFromCents(0, b.Currency)
		a = &zero
	}
	if b == nil {
		zero := MoneyFromCents(0, a.Currency)
		b = &zero
	}
	if a.Currency != "" && b.Currency != "" && a.Currency != b.Currency {
		return nil
	}
	return moneyPtr(MoneyFromCents(a.AmountCents-b.AmountCents, firstNonEmptyString(a.Currency, b.Currency)))
}

func firstMoney(values ...*MoneyView) *MoneyView {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

// IsZero reports whether the MoneyView carries no information. Used
// by tool envelopes to decide whether to omit a money field rather
// than emit "$0.00" when no rate is actually known.
func (m MoneyView) IsZero() bool {
	return m.AmountCents == 0 && m.Currency == "" && m.Display == ""
}

// EffectiveRate is the resolved hourly + cost rate that should be
// applied to a given (workspace, project, task, entry, user)
// combination. The Scope fields tell the caller which layer of the
// hierarchy supplied the rate so the model can explain its answer.
type EffectiveRate struct {
	Hourly      *clockify.Rate `json:"hourly,omitempty"`
	Cost        *clockify.Rate `json:"cost,omitempty"`
	HourlyScope RateScope      `json:"hourly_scope"`
	CostScope   RateScope      `json:"cost_scope"`
}

// resolveLayer returns (rate, true) if `r` is a concrete rate
// (non-nil, not the "##default" sentinel). Inherited / nil rates
// fall through to the parent scope.
func resolveLayer(r *clockify.Rate) (*clockify.Rate, bool) {
	if r == nil || r.Inherited {
		return nil, false
	}
	return r, true
}

// membershipRate locates the per-user override on a memberships
// slice. Returns nil if the slice is nil, the user isn't present,
// or their rate is itself the sentinel.
func membershipRate(ms []clockify.ProjectMembership, userID string, cost bool) *clockify.Rate {
	if userID == "" {
		return nil
	}
	for i := range ms {
		if ms[i].UserID != userID {
			continue
		}
		if cost {
			if r, ok := resolveLayer(ms[i].CostRate); ok {
				return r
			}
			return nil
		}
		if r, ok := resolveLayer(ms[i].HourlyRate); ok {
			return r
		}
		return nil
	}
	return nil
}

// ResolveEffectiveRate walks the Clockify rate hierarchy:
//
//	entry > task > project-member (matched on userID) > project >
//	workspace-member (matched on userID) > workspace
//
// Any of the inputs may be nil. The returned EffectiveRate.Scope is
// RateScopeNone when no layer supplied a rate. Hourly and Cost are
// resolved independently because a workspace can legitimately set a
// cost rate without a billable rate (and vice versa).
func ResolveEffectiveRate(
	ws *clockify.Workspace,
	proj *clockify.Project,
	task *clockify.Task,
	entry *clockify.TimeEntry,
	userID string,
) EffectiveRate {
	var eff EffectiveRate
	eff.HourlyScope = RateScopeNone
	eff.CostScope = RateScopeNone

	// Hourly: entry -> task -> project member -> project -> ws member -> ws.
	if entry != nil {
		if r, ok := resolveLayer(entry.HourlyRate); ok {
			eff.Hourly, eff.HourlyScope = r, RateScopeEntry
		}
	}
	if eff.Hourly == nil && task != nil {
		if r, ok := resolveLayer(task.HourlyRate); ok {
			eff.Hourly, eff.HourlyScope = r, RateScopeTask
		}
	}
	if eff.Hourly == nil && proj != nil {
		if r := membershipRate(proj.Memberships, userID, false); r != nil {
			eff.Hourly, eff.HourlyScope = r, RateScopeProjectMember
		} else if r, ok := resolveLayer(proj.HourlyRate); ok {
			eff.Hourly, eff.HourlyScope = r, RateScopeProject
		}
	}
	if eff.Hourly == nil && ws != nil {
		if r := membershipRate(ws.Memberships, userID, false); r != nil {
			eff.Hourly, eff.HourlyScope = r, RateScopeWorkspace
		} else if r, ok := resolveLayer(ws.HourlyRate); ok {
			eff.Hourly, eff.HourlyScope = r, RateScopeWorkspace
		}
	}

	// Cost: identical precedence.
	if entry != nil {
		if r, ok := resolveLayer(entry.CostRate); ok {
			eff.Cost, eff.CostScope = r, RateScopeEntry
		}
	}
	if eff.Cost == nil && task != nil {
		if r, ok := resolveLayer(task.CostRate); ok {
			eff.Cost, eff.CostScope = r, RateScopeTask
		}
	}
	if eff.Cost == nil && proj != nil {
		if r := membershipRate(proj.Memberships, userID, true); r != nil {
			eff.Cost, eff.CostScope = r, RateScopeProjectMember
		} else if r, ok := resolveLayer(proj.CostRate); ok {
			eff.Cost, eff.CostScope = r, RateScopeProject
		}
	}
	if eff.Cost == nil && ws != nil {
		if r := membershipRate(ws.Memberships, userID, true); r != nil {
			eff.Cost, eff.CostScope = r, RateScopeWorkspace
		} else if r, ok := resolveLayer(ws.CostRate); ok {
			eff.Cost, eff.CostScope = r, RateScopeWorkspace
		}
	}

	return eff
}

// DerivedEarnings multiplies the effective hourly + cost rates by the
// given duration (in seconds) and returns the per-entry billable
// earnings and cost as MoneyViews. Both come back zero when no rate
// is set for that side. Rounding is to the nearest cent.
//
// Math: rate is cents-per-hour; duration is seconds; result is
//
//	cents = round(rateCents * durationSeconds / 3600)
//
// We use float intermediates and math.Round for the final cent —
// matching `CentsFromReportNumber`'s half-away-from-zero behaviour
// so the MCP-derived number agrees with Clockify's own.
func DerivedEarnings(eff EffectiveRate, durationSeconds int64) (billable, cost MoneyView) {
	if eff.Hourly != nil && durationSeconds > 0 {
		cents := int64(math.Round(float64(eff.Hourly.Amount) * float64(durationSeconds) / 3600.0))
		billable = MoneyFromCents(cents, eff.Hourly.Currency)
	}
	if eff.Cost != nil && durationSeconds > 0 {
		cents := int64(math.Round(float64(eff.Cost.Amount) * float64(durationSeconds) / 3600.0))
		cost = MoneyFromCents(cents, eff.Cost.Currency)
	}
	return billable, cost
}

// EntryRateView is the LLM-facing block we attach to every
// time-entry row across tool outputs. It bundles the raw entry-level
// rates (so the caller can see what Clockify itself stored) with the
// resolved effective rate and the derived per-entry money.
type EntryRateView struct {
	EntryHourly      *clockify.Rate `json:"entry_hourly,omitempty"`
	EntryCost        *clockify.Rate `json:"entry_cost,omitempty"`
	EffectiveHourly  *clockify.Rate `json:"effective_hourly,omitempty"`
	EffectiveCost    *clockify.Rate `json:"effective_cost,omitempty"`
	HourlyScope      RateScope      `json:"hourly_scope"`
	CostScope        RateScope      `json:"cost_scope"`
	DurationSeconds  int64          `json:"duration_seconds"`
	BillableEarnings MoneyView      `json:"billable_earnings,omitempty"`
	Cost             MoneyView      `json:"cost,omitempty"`
}

// BuildEntryRateView is the canonical adapter from a (workspace,
// project, task, entry, user) tuple to the EntryRateView envelope.
// Callers pass whatever context they have; nils are tolerated.
func BuildEntryRateView(
	ws *clockify.Workspace,
	proj *clockify.Project,
	task *clockify.Task,
	entry *clockify.TimeEntry,
) EntryRateView {
	view := EntryRateView{HourlyScope: RateScopeNone, CostScope: RateScopeNone}
	var userID string
	if entry != nil {
		view.EntryHourly = entry.HourlyRate
		view.EntryCost = entry.CostRate
		view.DurationSeconds = entry.DurationSeconds()
		userID = entry.UserID
	}
	eff := ResolveEffectiveRate(ws, proj, task, entry, userID)
	view.EffectiveHourly = eff.Hourly
	view.EffectiveCost = eff.Cost
	view.HourlyScope = eff.HourlyScope
	view.CostScope = eff.CostScope
	if entry != nil {
		view.BillableEarnings, view.Cost = DerivedEarnings(eff, view.DurationSeconds)
	}
	return view
}
