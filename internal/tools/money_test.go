package tools

import (
	"testing"

	"github.com/apet97/go-clockify/internal/clockify"
)

func rate(amount int64, ccy string) *clockify.Rate {
	return &clockify.Rate{Amount: amount, Currency: ccy}
}

func TestMoneyFromCents(t *testing.T) {
	got := MoneyFromCents(4321, "EUR")
	if got.AmountCents != 4321 || got.AmountDecimal != "43.21" || got.Currency != "EUR" || got.Display != "€43.21 EUR" {
		t.Errorf("got %+v", got)
	}
	if !MoneyFromRate(nil).IsZero() {
		t.Error("nil rate should give zero MoneyView")
	}
	if !MoneyFromRate(&clockify.Rate{Inherited: true}).IsZero() {
		t.Error("inherited rate should give zero MoneyView")
	}
}

func TestResolveEffectiveRate_Precedence(t *testing.T) {
	ws := &clockify.Workspace{
		HourlyRate: rate(100, "EUR"),
		CostRate:   rate(50, "EUR"),
		Memberships: []clockify.ProjectMembership{
			{UserID: "u1", HourlyRate: rate(150, "EUR")},
		},
	}
	proj := &clockify.Project{
		HourlyRate: rate(200, "EUR"),
		Memberships: []clockify.ProjectMembership{
			{UserID: "u1", HourlyRate: rate(300, "EUR")},
		},
	}
	task := &clockify.Task{HourlyRate: rate(400, "EUR")}
	entry := &clockify.TimeEntry{HourlyRate: rate(500, "EUR"), UserID: "u1"}

	t.Run("entry_wins", func(t *testing.T) {
		eff := ResolveEffectiveRate(ws, proj, task, entry, "u1")
		if eff.Hourly == nil || eff.Hourly.Amount != 500 || eff.HourlyScope != RateScopeEntry {
			t.Fatalf("got %+v", eff)
		}
	})
	t.Run("task_wins_when_no_entry_rate", func(t *testing.T) {
		e2 := *entry
		e2.HourlyRate = nil
		eff := ResolveEffectiveRate(ws, proj, task, &e2, "u1")
		if eff.Hourly == nil || eff.Hourly.Amount != 400 || eff.HourlyScope != RateScopeTask {
			t.Fatalf("got %+v", eff)
		}
	})
	t.Run("project_member_wins_over_project", func(t *testing.T) {
		e2 := *entry
		e2.HourlyRate = nil
		t2 := *task
		t2.HourlyRate = nil
		eff := ResolveEffectiveRate(ws, proj, &t2, &e2, "u1")
		if eff.Hourly == nil || eff.Hourly.Amount != 300 || eff.HourlyScope != RateScopeProjectMember {
			t.Fatalf("got %+v", eff)
		}
	})
	t.Run("project_wins_when_no_member_match", func(t *testing.T) {
		e2 := *entry
		e2.HourlyRate = nil
		t2 := *task
		t2.HourlyRate = nil
		eff := ResolveEffectiveRate(ws, proj, &t2, &e2, "stranger")
		if eff.Hourly == nil || eff.Hourly.Amount != 200 || eff.HourlyScope != RateScopeProject {
			t.Fatalf("got %+v", eff)
		}
	})
	t.Run("workspace_member_falls_through_to_workspace", func(t *testing.T) {
		eff := ResolveEffectiveRate(ws, nil, nil, nil, "u1")
		if eff.Hourly == nil || eff.Hourly.Amount != 150 || eff.HourlyScope != RateScopeWorkspace {
			t.Fatalf("got %+v", eff)
		}
	})
	t.Run("workspace_default_when_user_not_in_membership", func(t *testing.T) {
		eff := ResolveEffectiveRate(ws, nil, nil, nil, "stranger")
		if eff.Hourly == nil || eff.Hourly.Amount != 100 || eff.HourlyScope != RateScopeWorkspace {
			t.Fatalf("got %+v", eff)
		}
	})
	t.Run("none_when_all_nil", func(t *testing.T) {
		eff := ResolveEffectiveRate(nil, nil, nil, nil, "")
		if eff.Hourly != nil || eff.HourlyScope != RateScopeNone {
			t.Fatalf("got %+v", eff)
		}
	})
	t.Run("inherited_sentinel_skips_layer", func(t *testing.T) {
		t2 := *task
		t2.HourlyRate = &clockify.Rate{Inherited: true}
		e2 := *entry
		e2.HourlyRate = nil
		eff := ResolveEffectiveRate(ws, proj, &t2, &e2, "u1")
		// Should fall through to project member -> 300.
		if eff.Hourly == nil || eff.Hourly.Amount != 300 || eff.HourlyScope != RateScopeProjectMember {
			t.Fatalf("got %+v", eff)
		}
	})
}

func TestResolveEffectiveRate_CostIndependent(t *testing.T) {
	// Workspace sets cost only; project sets hourly only.
	ws := &clockify.Workspace{CostRate: rate(50, "EUR")}
	proj := &clockify.Project{HourlyRate: rate(200, "EUR")}
	eff := ResolveEffectiveRate(ws, proj, nil, nil, "")
	if eff.Hourly == nil || eff.Hourly.Amount != 200 || eff.HourlyScope != RateScopeProject {
		t.Errorf("hourly: got %+v", eff)
	}
	if eff.Cost == nil || eff.Cost.Amount != 50 || eff.CostScope != RateScopeWorkspace {
		t.Errorf("cost: got %+v", eff)
	}
}

func TestDerivedEarnings(t *testing.T) {
	cases := []struct {
		name             string
		eff              EffectiveRate
		seconds          int64
		wantBillCents    int64
		wantCostCents    int64
	}{
		{"one_hour_at_4321", EffectiveRate{Hourly: rate(4321, "EUR"), Cost: rate(876, "EUR")}, 3600, 4321, 876},
		{"half_hour", EffectiveRate{Hourly: rate(4000, "USD")}, 1800, 2000, 0},
		{"quarter_hour_rounds", EffectiveRate{Hourly: rate(100, "USD")}, 900, 25, 0},
		{"no_rate_no_money", EffectiveRate{}, 3600, 0, 0},
		{"zero_duration", EffectiveRate{Hourly: rate(4321, "EUR")}, 0, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, c := DerivedEarnings(tc.eff, tc.seconds)
			if b.AmountCents != tc.wantBillCents {
				t.Errorf("billable=%d want %d", b.AmountCents, tc.wantBillCents)
			}
			if c.AmountCents != tc.wantCostCents {
				t.Errorf("cost=%d want %d", c.AmountCents, tc.wantCostCents)
			}
		})
	}
}

func TestBuildEntryRateView_TaskRateOverDuration(t *testing.T) {
	// Mirror the live probe shape: 1h entry, task hourly = 4321,
	// task cost = 876, project rate not persisted (0) on this plan.
	task := &clockify.Task{HourlyRate: rate(4321, "EUR"), CostRate: rate(876, "EUR")}
	proj := &clockify.Project{HourlyRate: rate(0, "EUR")} // 0 from the live probe
	entry := &clockify.TimeEntry{
		UserID: "u1",
		TimeInterval: clockify.TimeInterval{
			Start: "2026-05-12T12:12:00Z",
			End:   "2026-05-12T13:12:00Z",
		},
	}
	view := BuildEntryRateView(nil, proj, task, entry)
	if view.HourlyScope != RateScopeTask {
		t.Errorf("hourly_scope=%s want TASK", view.HourlyScope)
	}
	if view.BillableEarnings.AmountCents != 4321 {
		t.Errorf("earnings=%d want 4321 (€43.21 for 1h)", view.BillableEarnings.AmountCents)
	}
	if view.Cost.AmountCents != 876 {
		t.Errorf("cost=%d want 876", view.Cost.AmountCents)
	}
	if view.DurationSeconds != 3600 {
		t.Errorf("duration=%d want 3600", view.DurationSeconds)
	}
}
