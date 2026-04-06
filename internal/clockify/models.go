package clockify

import "time"

type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Project struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ClientID string `json:"clientId"`
	Color    string `json:"color"`
	Archived bool   `json:"archived"`
}

type ClientEntity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Task struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ProjectID string `json:"projectId"`
}

type TimeInterval struct {
	Start    string `json:"start"`
	End      string `json:"end,omitempty"`
	Duration string `json:"duration,omitempty"`
}

type TimeEntry struct {
	ID           string       `json:"id"`
	Description  string       `json:"description"`
	ProjectID    string       `json:"projectId"`
	ProjectName  string       `json:"projectName,omitempty"`
	TaskID       string       `json:"taskId,omitempty"`
	TagIDs       []string     `json:"tagIds,omitempty"`
	Billable     bool         `json:"billable,omitempty"`
	UserID       string       `json:"userId,omitempty"`
	WorkspaceID  string       `json:"workspaceId,omitempty"`
	TimeInterval TimeInterval `json:"timeInterval"`
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
