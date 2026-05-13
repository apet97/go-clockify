package tools

import (
	"context"
	"maps"

	"github.com/apet97/go-clockify/internal/paths"
)

type WorkspaceGovernanceView struct {
	WorkspaceID                string         `json:"workspaceId"`
	Workspace                  WorkspaceView  `json:"workspace"`
	LockPolicy                 any            `json:"lock_policy,omitempty"`
	Approval                   any            `json:"approval,omitempty"`
	Rounding                   any            `json:"rounding,omitempty"`
	ForceFields                any            `json:"force_fields,omitempty"`
	EntityCreationPermissions  any            `json:"entity_creation_permissions,omitempty"`
	Features                   []string       `json:"features,omitempty"`
	FeaturesHuman              []FeatureLabel `json:"features_human,omitempty"`
	SubscriptionLabel          string         `json:"subscription_label,omitempty"`
	PlanCohort                 string         `json:"plan_cohort,omitempty"`
	WorkingDays                []string       `json:"working_days,omitempty"`
	WorkingDayPattern          string         `json:"working_day_pattern,omitempty"`
	WeekendIncluded            bool           `json:"weekend_included,omitempty"`
	WorkingDayCount            int            `json:"working_day_count,omitempty"`
	ErrorTranslationReferences []string       `json:"error_translation_refs,omitempty"`
	RawSettings                map[string]any `json:"raw_settings,omitempty"`
}

func (s *Service) WorkspaceGovernance(ctx context.Context) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var raw map[string]any
	if err := s.Client.Get(ctx, path, nil, &raw); err != nil {
		return ResultEnvelope{}, err
	}
	workspace := workspaceViewFromRaw(raw)
	settings, _ := workspace.WorkspaceSettings.(map[string]any)
	view := WorkspaceGovernanceView{
		WorkspaceID:                wsID,
		Workspace:                  workspace,
		LockPolicy:                 workspace.SettingsSummary.LockPolicy,
		Approval:                   firstPresent(settings, "approval", "approvalSettings", "approval_settings", "timeTrackingApproval"),
		Rounding:                   workspace.SettingsSummary.Rounding,
		ForceFields:                firstPresent(settings, "forceFields", "force_fields", "timeEntryForceFields"),
		EntityCreationPermissions:  workspace.SettingsSummary.EntityCreationPermissions,
		Features:                   workspace.Features,
		FeaturesHuman:              workspace.FeaturesHuman,
		SubscriptionLabel:          workspace.SubscriptionLabel,
		PlanCohort:                 workspace.PlanCohort,
		WorkingDays:                workspace.SettingsSummary.WorkingDays,
		WorkingDayPattern:          workspace.SettingsSummary.WorkingDayPattern,
		WeekendIncluded:            workspace.SettingsSummary.WeekendIncluded,
		WorkingDayCount:            workspace.SettingsSummary.WorkingDayCount,
		ErrorTranslationReferences: []string{"clockify_error_translation"},
	}
	if settings != nil {
		view.RawSettings = maps.Clone(settings)
	}
	return ok("clockify_workspace_governance", view, map[string]any{"workspaceId": wsID, "source": "workspace_api"}), nil
}
