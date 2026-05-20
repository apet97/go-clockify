package tools

import (
	"context"
	"strings"

	"github.com/apet97/go-clockify/internal/clockify"
)

func (s *Service) ClockifyStatus(ctx context.Context, _ map[string]any) (any, error) {
	user, err := s.getCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	workspace, err := s.getWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	workspace.Memberships = nil
	warnings := []Warning{}
	currentTimer, err := s.currentTimer(ctx)
	if err != nil {
		warnings = append(warnings, Warning{Code: "timer_unavailable", Message: err.Error()})
	}
	featureStatus, featureWarnings := oneUserFeatureStatus(workspace.Features)
	warnings = append(warnings, featureWarnings...)
	return result("clockify_status", "workspace", map[string]string{
		"workspaceId": wsID,
		"userId":      user.ID,
	}, statusData{
		User:                  user,
		Workspace:             workspace,
		ActiveWorkspaceID:     user.ActiveWorkspace,
		DefaultWorkspaceID:    user.DefaultWorkspace,
		Timezone:              s.timezoneName(),
		WeekStart:             statusWeekStart(workspace, user),
		CurrentTimer:          currentTimer,
		FeatureSubscription:   workspace.FeatureSubscriptionType,
		FeatureStatus:         featureStatus,
		RecommendedFirstTools: []string{"clockify_tools_guide", "clockify_create_work_package", "clockify_log_work", "clockify_start_work", "clockify_review_day"},
	}, ChangeSet{}, warnings, []NextAction{
		{Tool: "clockify_tools_guide", Reason: "Pick the best workflow tool before falling back to domain tools."},
		{Tool: "clockify_create_work_package", Reason: "Create or reuse a client/project/task/tag package for work tracking."},
		{Tool: "clockify_log_work", Reason: "Log finished work with human-friendly project, task, and tag names."},
	}), nil
}

func statusWeekStart(workspace clockify.Workspace, user clockify.User) string {
	if weekStart, ok := workspaceSettingAny(workspace.WorkspaceSettings, "weekStart", "week_start").(string); ok && strings.TrimSpace(weekStart) != "" {
		return strings.ToUpper(strings.TrimSpace(weekStart))
	}
	if settings, ok := user.Settings.(map[string]any); ok {
		if weekStart := firstReportString(settings, "weekStart", "week_start"); weekStart != "" {
			return strings.ToUpper(weekStart)
		}
	}
	return ""
}

func oneUserFeatureStatus(features []string) (map[string]string, []Warning) {
	type signal struct {
		key     string
		needles []string
	}
	signals := []signal{
		{key: "invoices", needles: []string{"INVOICE", "INVOIC"}},
		{key: "expenses", needles: []string{"EXPENSE"}},
		{key: "customFields", needles: []string{"CUSTOM_FIELD", "CUSTOMFIELD"}},
		{key: "timeOff", needles: []string{"TIME_OFF", "TIMEOFF", "TIME OFF"}},
		{key: "scheduling", needles: []string{"SCHEDUL"}},
		{key: "approvals", needles: []string{"APPROVAL"}},
		{key: "webhooks", needles: []string{"WEBHOOK"}},
		{key: "reports", needles: []string{"REPORT"}},
		{key: "groups", needles: []string{"GROUP"}},
		{key: "holidays", needles: []string{"HOLIDAY"}},
		{key: "sharedReports", needles: []string{"SHARED_REPORT", "SHAREDREPORT"}},
	}
	normalized := make([]string, 0, len(features))
	for _, feature := range features {
		feature = strings.ToUpper(strings.TrimSpace(feature))
		if feature != "" {
			normalized = append(normalized, feature)
		}
	}
	out := make(map[string]string, len(signals))
	if len(normalized) == 0 {
		for _, sig := range signals {
			out[sig.key] = "unknown"
		}
		return out, []Warning{{Code: "feature_signals_unknown", Message: "Clockify did not return workspace feature signals; paid-feature tools may return recovery guidance when called."}}
	}
	notAdvertised := []string{}
	for _, sig := range signals {
		status := "not_advertised"
		if sig.key == "groups" || sig.key == "holidays" {
			status = "unknown"
		}
		for _, feature := range normalized {
			for _, needle := range sig.needles {
				if strings.Contains(feature, needle) {
					status = "available"
					break
				}
			}
			if status == "available" {
				break
			}
		}
		out[sig.key] = status
		if status == "not_advertised" {
			notAdvertised = append(notAdvertised, sig.key)
		}
	}
	if len(notAdvertised) == 0 {
		return out, nil
	}
	return out, []Warning{{Code: "feature_signals_not_advertised", Message: "Some optional Clockify feature signals were not advertised by the workspace: " + strings.Join(notAdvertised, ", ")}}
}
