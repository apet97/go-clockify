package tools

import (
	"maps"
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
	Raw                    map[string]any         `json:"raw,omitempty"`
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
	HourlyRate              *clockify.Rate    `json:"hourlyRate,omitempty"`
	ImageURL                string            `json:"imageUrl,omitempty"`
	Memberships             any               `json:"memberships,omitempty"`
	Subdomain               any               `json:"subdomain,omitempty"`
	WorkspaceSettings       any               `json:"workspaceSettings,omitempty"`
	SettingsSummary         WorkspaceSettings `json:"settings_summary"`
	Raw                     map[string]any    `json:"raw,omitempty"`
}

type WorkspaceSettings struct {
	Currencies                any      `json:"currencies,omitempty"`
	Features                  []string `json:"features,omitempty"`
	FeaturePlan               string   `json:"feature_plan,omitempty"`
	DurationFormat            string   `json:"duration_format,omitempty"`
	CurrencyFormat            any      `json:"currency_format,omitempty"`
	NumberFormat              any      `json:"number_format,omitempty"`
	ProjectLabel              string   `json:"project_label,omitempty"`
	TaskLabel                 string   `json:"task_label,omitempty"`
	WorkingDays               any      `json:"working_days,omitempty"`
	LockPolicy                any      `json:"lock_policy,omitempty"`
	Rounding                  any      `json:"rounding,omitempty"`
	EntityCreationPermissions any      `json:"entity_creation_permissions,omitempty"`
	HasHourlyRate             bool     `json:"has_hourly_rate,omitempty"`
	HasCostRate               bool     `json:"has_cost_rate,omitempty"`
	MembershipsCount          int      `json:"memberships_count,omitempty"`
	Source                    string   `json:"source,omitempty"`
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
		Raw: map[string]any{
			"id":               user.ID,
			"name":             user.Name,
			"email":            user.Email,
			"activeWorkspace":  user.ActiveWorkspace,
			"customFields":     user.CustomFields,
			"defaultWorkspace": user.DefaultWorkspace,
			"memberships":      user.Memberships,
			"profilePicture":   user.ProfilePicture,
			"settings":         user.Settings,
			"status":           user.Status,
		},
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

func workspaceViewFromWorkspace(ws clockify.Workspace) WorkspaceView {
	raw := map[string]any{
		"id":                      ws.ID,
		"name":                    ws.Name,
		"cakeOrganizationId":      ws.CakeOrganizationID,
		"costRate":                ws.CostRate,
		"currencies":              ws.Currencies,
		"featureSubscriptionType": ws.FeatureSubscriptionType,
		"features":                ws.Features,
		"hourlyRate":              ws.HourlyRate,
		"imageUrl":                ws.ImageURL,
		"memberships":             ws.Memberships,
		"subdomain":               ws.Subdomain,
		"workspaceSettings":       ws.WorkspaceSettings,
	}
	return WorkspaceView{
		ID:                      ws.ID,
		Name:                    ws.Name,
		CakeOrganizationID:      ws.CakeOrganizationID,
		CostRate:                ws.CostRate,
		Currencies:              ws.Currencies,
		FeatureSubscriptionType: ws.FeatureSubscriptionType,
		Features:                ws.Features,
		HourlyRate:              ws.HourlyRate,
		ImageURL:                ws.ImageURL,
		Memberships:             ws.Memberships,
		Subdomain:               ws.Subdomain,
		WorkspaceSettings:       ws.WorkspaceSettings,
		SettingsSummary: WorkspaceSettings{
			Currencies:                ws.Currencies,
			Features:                  ws.Features,
			FeaturePlan:               ws.FeatureSubscriptionType,
			DurationFormat:            workspaceSettingString(ws.WorkspaceSettings, "durationFormat", "duration_format"),
			CurrencyFormat:            workspaceSettingAny(ws.WorkspaceSettings, "currencyFormat", "currency_format"),
			NumberFormat:              workspaceSettingAny(ws.WorkspaceSettings, "numberFormat", "number_format"),
			ProjectLabel:              workspaceSettingString(ws.WorkspaceSettings, "projectLabel", "project_label"),
			TaskLabel:                 workspaceSettingString(ws.WorkspaceSettings, "taskLabel", "task_label"),
			WorkingDays:               workspaceSettingAny(ws.WorkspaceSettings, "workingDays", "working_days"),
			LockPolicy:                workspaceLockPolicy(ws.WorkspaceSettings),
			Rounding:                  workspaceSettingAny(ws.WorkspaceSettings, "round"),
			EntityCreationPermissions: workspaceSettingAny(ws.WorkspaceSettings, "entityCreationPermissions", "entity_creation_permissions"),
			HasHourlyRate:             ws.HourlyRate != nil,
			HasCostRate:               ws.CostRate != nil,
			MembershipsCount:          len(ws.Memberships),
			Source:                    "workspace_api",
		},
		Raw: raw,
	}
}

func workspaceViewsFromWorkspaces(workspaces []clockify.Workspace) []WorkspaceView {
	out := make([]WorkspaceView, 0, len(workspaces))
	for _, ws := range workspaces {
		out = append(out, workspaceViewFromWorkspace(ws))
	}
	return out
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
	view.Currencies = firstPresent(raw, "currencies")
	view.FeatureSubscriptionType = firstReportString(raw, "featureSubscriptionType")
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
	view.SettingsSummary.WorkingDays = workspaceSettingAny(view.WorkspaceSettings, "workingDays", "working_days")
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
	view["profile"] = map[string]any{
		"id":            firstReportString(raw, "id", "_id", "userId", "user_id"),
		"name":          firstReportString(raw, "name"),
		"email":         firstReportString(raw, "email"),
		"week_start":    strings.ToUpper(firstReportString(raw, "weekStart", "week_start")),
		"work_capacity": firstPresent(raw, "workCapacity", "work_capacity"),
		"working_days":  firstPresent(raw, "workingDays", "working_days"),
		"status":        strings.ToUpper(firstReportString(raw, "status")),
		"source":        "member_profile_api",
	}
	if custom := customFieldsFromFirst(raw, "userCustomFields", "user_custom_fields", "customFields", "customFieldValues"); len(custom) > 0 {
		view["custom_fields_normalized"] = custom
	}
	view["raw"] = maps.Clone(raw)
	return view
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
