package clockify

import (
	"encoding/json"
	"time"
)

// CurrencyView is a Clockify currency object: a Mongo-style id, an ISO code,
// and (within a workspace currency list) whether it is the workspace default.
// It is decode-only on Workspace.Currencies; no handler re-marshals it upstream.
type CurrencyView struct {
	ID        string `json:"id,omitempty"`
	Code      string `json:"code,omitempty"`
	IsDefault bool   `json:"isDefault,omitempty"`
}

// SubdomainView is a Clockify workspace subdomain configuration.
type SubdomainView struct {
	Name    string `json:"name,omitempty"`
	Enabled bool   `json:"enabled,omitempty"`
}

// Workspace is a Clockify workspace as returned by the API, including its
// rates, currencies, enabled features, and memberships.
type Workspace struct {
	ID                      string              `json:"id"`
	Name                    string              `json:"name"`
	CakeOrganizationID      string              `json:"cakeOrganizationId,omitempty"`
	CostRate                *Rate               `json:"costRate,omitempty"`
	Currencies              []CurrencyView      `json:"currencies,omitempty"`
	FeatureSubscriptionType string              `json:"featureSubscriptionType,omitempty"`
	Features                []string            `json:"features,omitempty"`
	HourlyRate              *Rate               `json:"hourlyRate,omitempty"`
	ImageURL                string              `json:"imageUrl,omitempty"`
	Memberships             []ProjectMembership `json:"memberships,omitempty"`
	Subdomain               *SubdomainView      `json:"subdomain,omitempty"`
	WorkspaceSettings       any                 `json:"workspaceSettings,omitempty"`
}

// User is a Clockify user, including the active/default workspace and profile
// settings returned by the users endpoints.
type User struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Email            string `json:"email"`
	ActiveWorkspace  string `json:"activeWorkspace,omitempty"`
	CustomFields     any    `json:"customFields,omitempty"`
	DefaultWorkspace string `json:"defaultWorkspace,omitempty"`
	Memberships      any    `json:"memberships,omitempty"`
	ProfilePicture   string `json:"profilePicture,omitempty"`
	Settings         any    `json:"settings,omitempty"`
	Status           string `json:"status,omitempty"`
}

// Project is a Clockify project, including its client linkage, rates, estimate
// and budget settings, memberships, and embedded tasks.
type Project struct {
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	ClientID       string              `json:"clientId,omitempty"`
	ClientName     string              `json:"clientName,omitempty"`
	Client         any                 `json:"client,omitempty"`
	Color          string              `json:"color,omitempty"`
	Archived       bool                `json:"archived"`
	Billable       bool                `json:"billable,omitempty"`
	BudgetEstimate any                 `json:"budgetEstimate,omitempty"`
	CostRate       *Rate               `json:"costRate,omitempty"`
	Currency       any                 `json:"currency,omitempty"`
	CustomFields   any                 `json:"customFields,omitempty"`
	Duration       string              `json:"duration,omitempty"`
	Estimate       any                 `json:"estimate,omitempty"`
	EstimateReset  any                 `json:"estimateReset,omitempty"`
	Expenses       any                 `json:"expenses,omitempty"`
	Favorite       bool                `json:"favorite,omitempty"`
	HourlyRate     *Rate               `json:"hourlyRate,omitempty"`
	Memberships    []ProjectMembership `json:"memberships,omitempty"`
	Note           string              `json:"note,omitempty"`
	Public         bool                `json:"public,omitempty"`
	Tasks          []Task              `json:"tasks,omitempty"`
	Template       bool                `json:"template,omitempty"`
	TimeEstimate   any                 `json:"timeEstimate,omitempty"`
	WorkspaceID    string              `json:"workspaceId,omitempty"`
}

// ClientEntity is a Clockify client (billing customer). It is named
// ClientEntity to avoid colliding with the HTTP Client type in this package.
type ClientEntity struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Address      string   `json:"address,omitempty"`
	Archived     bool     `json:"archived,omitempty"`
	CCEmails     []string `json:"ccEmails,omitempty"`
	CurrencyCode string   `json:"currencyCode,omitempty"`
	CurrencyID   string   `json:"currencyId,omitempty"`
	Email        string   `json:"email,omitempty"`
	Note         string   `json:"note,omitempty"`
	WorkspaceID  string   `json:"workspaceId,omitempty"`
}

// Tag is a Clockify tag used to label time entries.
type Tag struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Archived    bool   `json:"archived"`
	WorkspaceID string `json:"workspaceId,omitempty"`
}

// Task is a Clockify task within a project, including assignees, rates, and
// estimate/status fields.
type Task struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	ProjectID      string   `json:"projectId"`
	AssigneeID     string   `json:"assigneeId,omitempty"`
	AssigneeIDs    []string `json:"assigneeIds,omitempty"`
	Billable       bool     `json:"billable"`
	BudgetEstimate int64    `json:"budgetEstimate,omitempty"`
	CostRate       *Rate    `json:"costRate,omitempty"`
	Duration       string   `json:"duration,omitempty"`
	Estimate       string   `json:"estimate,omitempty"`
	HourlyRate     *Rate    `json:"hourlyRate,omitempty"`
	Status         string   `json:"status,omitempty"`
	UserGroupIDs   []string `json:"userGroupIds,omitempty"`
}

// TimeInterval is a Clockify time-entry interval: RFC3339 start, optional end
// (empty while running), and an ISO-8601 duration string.
type TimeInterval struct {
	Start    string `json:"start"`
	End      string `json:"end,omitempty"`
	Duration string `json:"duration,omitempty"`
}

// TimeEntry is a Clockify time entry. BillablePresent (excluded from JSON)
// records whether the upstream payload actually carried a billable field, so
// callers can distinguish an explicit false from an omitted value; see
// UnmarshalJSON.
type TimeEntry struct {
	ID                string       `json:"id"`
	Description       string       `json:"description"`
	ProjectID         string       `json:"projectId"`
	ProjectName       string       `json:"projectName,omitempty"`
	TaskID            string       `json:"taskId,omitempty"`
	TagIDs            []string     `json:"tagIds,omitempty"`
	Billable          bool         `json:"billable"`
	BillablePresent   bool         `json:"-"`
	CostRate          *Rate        `json:"costRate,omitempty"`
	CustomFieldValues any          `json:"customFieldValues,omitempty"`
	HourlyRate        *Rate        `json:"hourlyRate,omitempty"`
	IsLocked          bool         `json:"isLocked,omitempty"`
	KioskID           string       `json:"kioskId,omitempty"`
	Type              string       `json:"type,omitempty"`
	UserID            string       `json:"userId,omitempty"`
	WorkspaceID       string       `json:"workspaceId,omitempty"`
	TimeInterval      TimeInterval `json:"timeInterval"`
}

// UnmarshalJSON decodes a TimeEntry and additionally records whether the
// payload included a billable field, setting BillablePresent so an omitted
// billable is distinguishable from an explicit false.
func (e *TimeEntry) UnmarshalJSON(data []byte) error {
	type alias TimeEntry
	aux := struct {
		Billable *bool `json:"billable"`
		*alias
	}{
		alias: (*alias)(e),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	e.BillablePresent = aux.Billable != nil
	if aux.Billable != nil {
		e.Billable = *aux.Billable
	}
	return nil
}

// StartTime parses the entry's interval start as an RFC3339 time.
func (e TimeEntry) StartTime() (time.Time, error) {
	return time.Parse(time.RFC3339, e.TimeInterval.Start)
}

// EndTime parses the entry's interval end as an RFC3339 time, returning the
// zero time and a nil error when the entry is still running (no end set).
func (e TimeEntry) EndTime() (time.Time, error) {
	if e.TimeInterval.End == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, e.TimeInterval.End)
}

// IsRunning reports whether the entry has no end time and is therefore still
// running.
func (e TimeEntry) IsRunning() bool {
	return e.TimeInterval.End == ""
}

// DurationSeconds returns the entry's elapsed duration in whole seconds. For a
// running entry it measures from the start to now (UTC); it returns 0 when the
// start is unparseable or the computed end precedes the start.
func (e TimeEntry) DurationSeconds() int64 {
	start, err := e.StartTime()
	if err != nil {
		return 0
	}
	end, err := e.EndTime()
	if err != nil || end.IsZero() {
		end = time.Now().UTC()
	}
	if end.Before(start) {
		return 0
	}
	return int64(end.Sub(start).Seconds())
}
