package tools

import (
	"maps"
	"sort"
	"strings"

	"github.com/apet97/go-clockify/internal/clockify"
)

// UserView preserves the upstream user fields while adding compact settings
// blocks that agents can use for report defaults and feature-aware wording.
type UserView struct {
	ID                     string                 `json:"id"`
	Name                   string                 `json:"name"`
	Email                  string                 `json:"email"`
	ActiveWorkspace        string                 `json:"activeWorkspace,omitempty"`
	CustomFields           any                    `json:"customFields,omitempty"`
	CustomFieldsNormalized []CustomFieldValueView `json:"custom_fields_normalized,omitempty"`
	DefaultWorkspace       string                 `json:"defaultWorkspace,omitempty"`
	Memberships            any                    `json:"memberships,omitempty"`
	ProfilePicture         string                 `json:"profilePicture,omitempty"`
	Settings               any                    `json:"settings,omitempty"`
	Status                 string                 `json:"status,omitempty"`
	ReportDefaults         ReportDefaults         `json:"report_defaults"`
	FeatureFlags           UserFeatureFlags       `json:"feature_flags"`
}

type ReportDefaults struct {
	Timezone              string `json:"timezone,omitempty"`
	WeekStart             string `json:"week_start,omitempty"`
	DateFormat            string `json:"date_format,omitempty"`
	TimeFormat            string `json:"time_format,omitempty"`
	SummaryReportSettings any    `json:"summary_report_settings,omitempty"`
	Source                string `json:"source,omitempty"`
}

type UserFeatureFlags struct {
	Scheduling bool `json:"scheduling,omitempty"`
	Approval   bool `json:"approval,omitempty"`
	PTO        bool `json:"pto,omitempty"`
}

type WorkspaceView struct {
	ID                      string            `json:"id"`
	Name                    string            `json:"name"`
	CakeOrganizationID      string            `json:"cakeOrganizationId,omitempty"`
	CostRate                *clockify.Rate    `json:"costRate,omitempty"`
	Currencies              any               `json:"currencies,omitempty"`
	FeatureSubscriptionType string            `json:"featureSubscriptionType,omitempty"`
	Features                []string          `json:"features,omitempty"`
	FeaturesHuman           []FeatureLabel    `json:"features_human,omitempty"`
	SubscriptionLabel       string            `json:"subscription_label,omitempty"`
	PlanCohort              string            `json:"plan_cohort,omitempty"`
	HourlyRate              *clockify.Rate    `json:"hourlyRate,omitempty"`
	ImageURL                string            `json:"imageUrl,omitempty"`
	Memberships             any               `json:"memberships,omitempty"`
	Subdomain               any               `json:"subdomain,omitempty"`
	WorkspaceSettings       any               `json:"workspaceSettings,omitempty"`
	SettingsSummary         WorkspaceSettings `json:"settings_summary"`
	Raw                     map[string]any    `json:"raw,omitempty"`
}

type WorkspaceSettings struct {
	Currencies                any            `json:"currencies,omitempty"`
	Features                  []string       `json:"features,omitempty"`
	FeaturePlan               string         `json:"feature_plan,omitempty"`
	FeaturesHuman             []FeatureLabel `json:"features_human,omitempty"`
	SubscriptionLabel         string         `json:"subscription_label,omitempty"`
	PlanCohort                string         `json:"plan_cohort,omitempty"`
	DurationFormat            string         `json:"duration_format,omitempty"`
	CurrencyFormat            any            `json:"currency_format,omitempty"`
	NumberFormat              any            `json:"number_format,omitempty"`
	ProjectLabel              string         `json:"project_label,omitempty"`
	TaskLabel                 string         `json:"task_label,omitempty"`
	WorkingDays               []string       `json:"working_days,omitempty"`
	WorkingDayPattern         string         `json:"working_day_pattern,omitempty"`
	WeekendIncluded           bool           `json:"weekend_included,omitempty"`
	WorkingDayCount           int            `json:"working_day_count,omitempty"`
	LockPolicy                any            `json:"lock_policy,omitempty"`
	Rounding                  any            `json:"rounding,omitempty"`
	EntityCreationPermissions any            `json:"entity_creation_permissions,omitempty"`
	HasHourlyRate             bool           `json:"has_hourly_rate,omitempty"`
	HasCostRate               bool           `json:"has_cost_rate,omitempty"`
	MembershipsCount          int            `json:"memberships_count,omitempty"`
	Source                    string         `json:"source,omitempty"`
}

type FeatureLabel struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

func userViewFromUser(user clockify.User) UserView {
	settings, _ := user.Settings.(map[string]any)
	view := UserView{
		ID:                     user.ID,
		Name:                   user.Name,
		Email:                  user.Email,
		ActiveWorkspace:        user.ActiveWorkspace,
		CustomFields:           user.CustomFields,
		CustomFieldsNormalized: customFieldValuesFromRaw(user.CustomFields),
		DefaultWorkspace:       user.DefaultWorkspace,
		Memberships:            user.Memberships,
		ProfilePicture:         user.ProfilePicture,
		Settings:               user.Settings,
		Status:                 user.Status,
		ReportDefaults:         reportDefaultsFromSettings(settings),
		FeatureFlags:           userFeatureFlagsFromSettings(settings),
	}
	return view
}

func userViewsFromUsers(users []clockify.User) []UserView {
	out := make([]UserView, 0, len(users))
	for _, user := range users {
		out = append(out, userViewFromUser(user))
	}
	return out
}

func reportDefaultsFromSettings(settings map[string]any) ReportDefaults {
	out := ReportDefaults{Source: "user_settings"}
	if settings == nil {
		out.Source = entryFinancialSourceUnavailable
		return out
	}
	out.Timezone = firstReportString(settings, "timeZone", "timezone")
	out.WeekStart = strings.ToUpper(firstReportString(settings, "weekStart", "week_start"))
	out.DateFormat = firstReportString(settings, "dateFormat", "date_format")
	out.TimeFormat = firstReportString(settings, "timeFormat", "time_format")
	out.SummaryReportSettings = firstPresent(settings, "summaryReportSettings", "summary_report_settings")
	return out
}

func userFeatureFlagsFromSettings(settings map[string]any) UserFeatureFlags {
	if settings == nil {
		return UserFeatureFlags{}
	}
	return UserFeatureFlags{
		Scheduling: boolFromAny(settings["scheduling"]),
		Approval:   boolFromAny(settings["approval"]),
		PTO:        boolFromAny(firstPresent(settings, "pto", "timeOff", "time_off")),
	}
}

func workspaceViewFromRaw(raw map[string]any) WorkspaceView {
	view := WorkspaceView{
		ID:   firstReportString(raw, "id", "_id"),
		Name: firstReportString(raw, "name"),
		SettingsSummary: WorkspaceSettings{
			Currencies:  firstPresent(raw, "currencies"),
			FeaturePlan: firstReportString(raw, "featureSubscriptionType"),
			Source:      "workspace_api",
		},
		Raw: maps.Clone(raw),
	}
	if features, ok, _ := strictStringSliceArg(raw, "features"); ok {
		view.Features = features
		view.SettingsSummary.Features = features
	} else if values, ok := raw["features"].([]any); ok {
		for _, value := range values {
			if s := reportValueString(value); s != "" {
				view.Features = append(view.Features, s)
			}
		}
		view.SettingsSummary.Features = view.Features
	}
	view.FeaturesHuman = humanFeatureLabels(view.Features)
	view.SettingsSummary.FeaturesHuman = view.FeaturesHuman
	view.Currencies = firstPresent(raw, "currencies")
	view.FeatureSubscriptionType = firstReportString(raw, "featureSubscriptionType")
	view.SubscriptionLabel, view.PlanCohort = subscriptionLabels(view.FeatureSubscriptionType)
	view.SettingsSummary.SubscriptionLabel = view.SubscriptionLabel
	view.SettingsSummary.PlanCohort = view.PlanCohort
	view.ImageURL = firstReportString(raw, "imageUrl")
	view.Subdomain = firstPresent(raw, "subdomain")
	view.WorkspaceSettings = firstPresent(raw, "workspaceSettings")
	view.Memberships = firstPresent(raw, "memberships")
	view.SettingsSummary.MembershipsCount = len(mapSlice(raw["memberships"]))
	view.SettingsSummary.DurationFormat = workspaceSettingString(view.WorkspaceSettings, "durationFormat", "duration_format")
	view.SettingsSummary.CurrencyFormat = workspaceSettingAny(view.WorkspaceSettings, "currencyFormat", "currency_format")
	view.SettingsSummary.NumberFormat = workspaceSettingAny(view.WorkspaceSettings, "numberFormat", "number_format")
	view.SettingsSummary.ProjectLabel = workspaceSettingString(view.WorkspaceSettings, "projectLabel", "project_label")
	view.SettingsSummary.TaskLabel = workspaceSettingString(view.WorkspaceSettings, "taskLabel", "task_label")
	workingDays := sortedWorkingDays(workspaceSettingAny(view.WorkspaceSettings, "workingDays", "working_days"))
	view.SettingsSummary.WorkingDays = workingDays
	view.SettingsSummary.WorkingDayPattern = workingDayPattern(workingDays)
	view.SettingsSummary.WeekendIncluded = workingDaysIncludeWeekend(workingDays)
	view.SettingsSummary.WorkingDayCount = len(workingDays)
	view.SettingsSummary.LockPolicy = workspaceLockPolicy(view.WorkspaceSettings)
	view.SettingsSummary.Rounding = workspaceSettingAny(view.WorkspaceSettings, "round")
	view.SettingsSummary.EntityCreationPermissions = workspaceSettingAny(view.WorkspaceSettings, "entityCreationPermissions", "entity_creation_permissions")
	return view
}

func workspaceSettingAny(raw any, keys ...string) any {
	if m, ok := raw.(map[string]any); ok {
		return firstPresent(m, keys...)
	}
	return nil
}

func workspaceSettingString(raw any, keys ...string) string {
	if m, ok := raw.(map[string]any); ok {
		return firstReportString(m, keys...)
	}
	return ""
}

func workspaceLockPolicy(raw any) any {
	if m, ok := raw.(map[string]any); ok {
		return map[string]any{
			"lock_time_entries": firstPresent(m, "lockTimeEntries", "lock_time_entries"),
			"lock_time_zone":    firstReportString(m, "lockTimeZone", "lock_time_zone"),
			"automatic_lock":    firstPresent(m, "automaticLock", "automatic_lock"),
		}
	}
	return nil
}

func memberProfileViewFromRaw(raw map[string]any) map[string]any {
	view := maps.Clone(raw)
	workingDays := sortedWorkingDays(firstPresent(raw, "workingDays", "working_days"))
	view["profile"] = map[string]any{
		"id":                  firstReportString(raw, "id", "_id", "userId", "user_id"),
		"name":                firstReportString(raw, "name"),
		"email":               firstReportString(raw, "email"),
		"week_start":          strings.ToUpper(firstReportString(raw, "weekStart", "week_start")),
		"work_capacity":       firstPresent(raw, "workCapacity", "work_capacity"),
		"working_days":        workingDays,
		"working_day_pattern": workingDayPattern(workingDays),
		"weekend_included":    workingDaysIncludeWeekend(workingDays),
		"working_day_count":   len(workingDays),
		"status":              strings.ToUpper(firstReportString(raw, "status")),
		"source":              "member_profile_api",
	}
	if custom := customFieldsFromFirst(raw, "userCustomFields", "user_custom_fields", "customFields", "customFieldValues"); len(custom) > 0 {
		view["custom_fields_normalized"] = custom
	}
	view["raw"] = maps.Clone(raw)
	return view
}

func humanFeatureLabels(features []string) []FeatureLabel {
	if len(features) == 0 {
		return nil
	}
	out := make([]FeatureLabel, 0, len(features))
	for _, feature := range features {
		key := strings.ToUpper(strings.TrimSpace(feature))
		if key == "" {
			continue
		}
		out = append(out, FeatureLabel{Key: key, Label: humanizeFeatureKey(key)})
	}
	return out
}

func humanizeFeatureKey(key string) string {
	key = strings.ReplaceAll(strings.ToLower(key), "_", " ")
	parts := strings.Fields(key)
	for i := range parts {
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, " ")
}

func subscriptionLabels(raw string) (string, string) {
	value := strings.ToUpper(strings.TrimSpace(raw))
	if value == "" {
		return "", ""
	}
	label := humanizeFeatureKey(value)
	cohort := "paid"
	switch {
	case strings.Contains(value, "FREE"):
		cohort = "free"
	case strings.Contains(value, "TRIAL"):
		cohort = "trial"
	case strings.Contains(value, "ENTERPRISE"):
		cohort = "enterprise"
	}
	return label, cohort
}

func sortedWorkingDays(raw any) []string {
	order := map[string]int{"MONDAY": 1, "TUESDAY": 2, "WEDNESDAY": 3, "THURSDAY": 4, "FRIDAY": 5, "SATURDAY": 6, "SUNDAY": 7}
	seen := map[string]struct{}{}
	var days []string
	for _, value := range anyStringSlice(raw) {
		day := strings.ToUpper(strings.TrimSpace(value))
		if _, ok := order[day]; !ok {
			continue
		}
		if _, ok := seen[day]; ok {
			continue
		}
		seen[day] = struct{}{}
		days = append(days, day)
	}
	sort.Slice(days, func(i, j int) bool { return order[days[i]] < order[days[j]] })
	return days
}

func anyStringSlice(raw any) []string {
	switch v := raw.(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s := reportValueString(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		if s := reportValueString(raw); s != "" {
			return []string{s}
		}
		return nil
	}
}

func workingDayPattern(days []string) string {
	if len(days) == 0 {
		return ""
	}
	return strings.Join(days, ",")
}

func workingDaysIncludeWeekend(days []string) bool {
	for _, day := range days {
		if day == "SATURDAY" || day == "SUNDAY" {
			return true
		}
	}
	return false
}

func userManagerViewsFromRaw(items []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		view := maps.Clone(item)
		view["manager"] = map[string]any{
			"id":     firstReportString(item, "id", "_id", "userId", "user_id"),
			"name":   firstReportString(item, "name", "userName", "user_name"),
			"email":  firstReportString(item, "email", "userEmail", "user_email"),
			"access": firstReportString(item, "access", "role"),
			"source": "user_managers_api",
		}
		view["roles"] = firstPresent(item, "roles", "memberships", "managerRoles")
		view["raw"] = maps.Clone(item)
		out = append(out, view)
	}
	return out
}

func boolFromAny(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}
