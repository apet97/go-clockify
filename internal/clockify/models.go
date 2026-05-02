package clockify

import "time"

type Workspace struct {
	ID                      string   `json:"id"`
	Name                    string   `json:"name"`
	CakeOrganizationID      string   `json:"cakeOrganizationId,omitempty"`
	CostRate                any      `json:"costRate,omitempty"`
	Currencies              any      `json:"currencies,omitempty"`
	FeatureSubscriptionType string   `json:"featureSubscriptionType,omitempty"`
	Features                []string `json:"features,omitempty"`
	HourlyRate              any      `json:"hourlyRate,omitempty"`
	ImageURL                string   `json:"imageUrl,omitempty"`
	Memberships             any      `json:"memberships,omitempty"`
	Subdomain               any      `json:"subdomain,omitempty"`
	WorkspaceSettings       any      `json:"workspaceSettings,omitempty"`
}

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

type Project struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	ClientID       string `json:"clientId,omitempty"`
	ClientName     string `json:"clientName,omitempty"`
	Color          string `json:"color,omitempty"`
	Archived       bool   `json:"archived"`
	Billable       bool   `json:"billable,omitempty"`
	BudgetEstimate any    `json:"budgetEstimate,omitempty"`
	CostRate       any    `json:"costRate,omitempty"`
	Duration       string `json:"duration,omitempty"`
	Estimate       any    `json:"estimate,omitempty"`
	EstimateReset  any    `json:"estimateReset,omitempty"`
	HourlyRate     any    `json:"hourlyRate,omitempty"`
	Memberships    any    `json:"memberships,omitempty"`
	Note           string `json:"note,omitempty"`
	Public         bool   `json:"public,omitempty"`
	Template       bool   `json:"template,omitempty"`
	TimeEstimate   any    `json:"timeEstimate,omitempty"`
	WorkspaceID    string `json:"workspaceId,omitempty"`
}

type ClientEntity struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Address      string `json:"address,omitempty"`
	Archived     bool   `json:"archived,omitempty"`
	CCEmails     any    `json:"ccEmails,omitempty"`
	CurrencyCode string `json:"currencyCode,omitempty"`
	CurrencyID   string `json:"currencyId,omitempty"`
	Email        string `json:"email,omitempty"`
	Note         string `json:"note,omitempty"`
	WorkspaceID  string `json:"workspaceId,omitempty"`
}

type Tag struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Archived    bool   `json:"archived,omitempty"`
	WorkspaceID string `json:"workspaceId,omitempty"`
}

type Task struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	ProjectID      string   `json:"projectId"`
	AssigneeID     string   `json:"assigneeId,omitempty"`
	AssigneeIDs    []string `json:"assigneeIds,omitempty"`
	Billable       bool     `json:"billable,omitempty"`
	BudgetEstimate int64    `json:"budgetEstimate,omitempty"`
	CostRate       any      `json:"costRate,omitempty"`
	Duration       string   `json:"duration,omitempty"`
	Estimate       string   `json:"estimate,omitempty"`
	HourlyRate     any      `json:"hourlyRate,omitempty"`
	Status         string   `json:"status,omitempty"`
	UserGroupIDs   []string `json:"userGroupIds,omitempty"`
}

type TimeInterval struct {
	Start    string `json:"start"`
	End      string `json:"end,omitempty"`
	Duration string `json:"duration,omitempty"`
}

type TimeEntry struct {
	ID                string       `json:"id"`
	Description       string       `json:"description"`
	ProjectID         string       `json:"projectId"`
	ProjectName       string       `json:"projectName,omitempty"`
	TaskID            string       `json:"taskId,omitempty"`
	TagIDs            []string     `json:"tagIds,omitempty"`
	Billable          bool         `json:"billable,omitempty"`
	CostRate          any          `json:"costRate,omitempty"`
	CustomFieldValues any          `json:"customFieldValues,omitempty"`
	HourlyRate        any          `json:"hourlyRate,omitempty"`
	IsLocked          bool         `json:"isLocked,omitempty"`
	KioskID           string       `json:"kioskId,omitempty"`
	Type              string       `json:"type,omitempty"`
	UserID            string       `json:"userId,omitempty"`
	WorkspaceID       string       `json:"workspaceId,omitempty"`
	TimeInterval      TimeInterval `json:"timeInterval"`
}

func (e TimeEntry) StartTime() (time.Time, error) {
	return time.Parse(time.RFC3339, e.TimeInterval.Start)
}

func (e TimeEntry) EndTime() (time.Time, error) {
	if e.TimeInterval.End == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, e.TimeInterval.End)
}

func (e TimeEntry) IsRunning() bool {
	return e.TimeInterval.End == ""
}

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
