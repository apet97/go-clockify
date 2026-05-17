package testclockify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/apet97/go-clockify/internal/clockify"
)

type Server struct {
	WorkspaceID string
	UserID      string
	URL         string

	http *httptest.Server
	mu   sync.Mutex
	seq  int

	clients  map[string]clockify.ClientEntity
	projects map[string]clockify.Project
	tasks    map[string]map[string]clockify.Task
	tags     map[string]clockify.Tag
	entries  map[string]clockify.TimeEntry
	invoices map[string]map[string]any
	expenses map[string]map[string]any
	timeOff  map[string]map[string]any
	schedule map[string]map[string]any
	webhooks map[string]map[string]any
	errors   map[string]fakeError
}

type fakeError struct {
	Status  int
	Message string
}

func NewServer(workspaceID string) *Server {
	if workspaceID == "" {
		workspaceID = "65b382b606de527a7ee2b60e"
	}
	s := &Server{
		WorkspaceID: workspaceID,
		UserID:      "user-1",
		clients:     map[string]clockify.ClientEntity{},
		projects:    map[string]clockify.Project{},
		tasks:       map[string]map[string]clockify.Task{},
		tags:        map[string]clockify.Tag{},
		entries:     map[string]clockify.TimeEntry{},
		invoices:    map[string]map[string]any{},
		expenses:    map[string]map[string]any{},
		timeOff:     map[string]map[string]any{},
		schedule:    map[string]map[string]any{},
		webhooks:    map[string]map[string]any{},
		errors:      map[string]fakeError{},
	}
	s.http = httptest.NewServer(http.HandlerFunc(s.serveHTTP))
	s.URL = s.http.URL
	return s
}

func (s *Server) Close() {
	s.http.Close()
}

func (s *Server) SetError(method, path string, status int, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errors[method+" "+path] = fakeError{Status: status, Message: message}
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Header.Get("X-Api-Key") == "" {
		http.Error(w, `{"message":"missing api key"}`, http.StatusUnauthorized)
		return
	}
	s.mu.Lock()
	injected, hasInjected := s.errors[r.Method+" "+r.URL.Path]
	s.mu.Unlock()
	if hasInjected {
		http.Error(w, fmt.Sprintf(`{"message":%q}`, injected.Message), injected.Status)
		return
	}
	parts := splitPath(r.URL.Path)
	if r.Method == http.MethodGet && len(parts) == 1 && parts[0] == "user" {
		writeJSON(w, clockify.User{ID: s.UserID, Name: "Test User", Email: "test@example.com", ActiveWorkspace: s.WorkspaceID, DefaultWorkspace: s.WorkspaceID})
		return
	}
	if len(parts) < 2 || parts[0] != "workspaces" || parts[1] != s.WorkspaceID {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodGet && len(parts) == 2 {
		writeJSON(w, clockify.Workspace{
			ID:                      s.WorkspaceID,
			Name:                    "Test Workspace",
			FeatureSubscriptionType: "FREE",
			Features:                []string{"PROJECTS", "TAGS", "TASKS"},
			WorkspaceSettings:       map[string]any{"weekStart": "MONDAY"},
		})
		return
	}
	switch {
	case len(parts) >= 3 && parts[2] == "clients":
		s.handleClients(w, r, parts)
	case len(parts) >= 3 && parts[2] == "projects":
		s.handleProjects(w, r, parts)
	case len(parts) >= 3 && parts[2] == "tags":
		s.handleTags(w, r, parts)
	case len(parts) >= 3 && parts[2] == "time-entries":
		s.handleEntries(w, r, parts)
	case len(parts) >= 5 && parts[2] == "user" && parts[4] == "time-entries":
		s.handleUserEntries(w, r, parts)
	case len(parts) >= 3 && parts[2] == "users":
		s.handleUsers(w, r, parts)
	case len(parts) >= 3 && parts[2] == "invoices":
		s.handleInvoices(w, r, parts)
	case len(parts) >= 3 && parts[2] == "expenses":
		s.handleExpenses(w, r, parts)
	case len(parts) >= 3 && parts[2] == "time-off":
		s.handleTimeOff(w, r, parts)
	case len(parts) >= 3 && parts[2] == "scheduling":
		s.handleScheduling(w, r, parts)
	case len(parts) >= 3 && parts[2] == "webhooks":
		s.handleWebhooks(w, r, parts)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleClients(w http.ResponseWriter, r *http.Request, parts []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(parts) == 3 && r.Method == http.MethodGet {
		out := make([]clockify.ClientEntity, 0, len(s.clients))
		for _, c := range s.clients {
			out = append(out, c)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
		out = paginate(out, r)
		writeJSON(w, out)
		return
	}
	if len(parts) == 3 && r.Method == http.MethodPost {
		var body struct {
			Name string `json:"name"`
		}
		if err := decode(r, &body); err != nil || strings.TrimSpace(body.Name) == "" {
			http.Error(w, `{"message":"name is required"}`, http.StatusBadRequest)
			return
		}
		c := clockify.ClientEntity{ID: s.nextID("client"), Name: body.Name, WorkspaceID: s.WorkspaceID}
		s.clients[c.ID] = c
		writeJSON(w, c)
		return
	}
	if len(parts) == 4 && r.Method == http.MethodDelete {
		delete(s.clients, parts[3])
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if len(parts) == 4 && r.Method == http.MethodPut {
		client, ok := s.clients[parts[3]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		if err := decode(r, &body); err != nil {
			http.Error(w, `{"message":"invalid client body"}`, http.StatusBadRequest)
			return
		}
		if name, ok := body["name"].(string); ok && strings.TrimSpace(name) != "" {
			client.Name = name
		}
		if archived, ok := body["archived"].(bool); ok {
			client.Archived = archived
		}
		s.clients[client.ID] = client
		writeJSON(w, client)
		return
	}
	if len(parts) == 4 && r.Method == http.MethodGet {
		client, ok := s.clients[parts[3]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, client)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request, parts []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(parts) == 3 && r.Method == http.MethodGet {
		out := make([]clockify.Project, 0, len(s.projects))
		clientFilter := r.URL.Query()["clients"]
		archivedFilter := r.URL.Query().Get("archived")
		for _, p := range s.projects {
			if len(clientFilter) > 0 && !slices.Contains(clientFilter, p.ClientID) {
				continue
			}
			if archivedFilter != "" && fmt.Sprint(p.Archived) != archivedFilter {
				continue
			}
			out = append(out, p)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
		out = paginate(out, r)
		writeJSON(w, out)
		return
	}
	if len(parts) == 3 && r.Method == http.MethodPost {
		var body struct {
			Name     string `json:"name"`
			ClientID string `json:"clientId"`
			Billable bool   `json:"billable"`
			Public   bool   `json:"isPublic"`
		}
		if err := decode(r, &body); err != nil || strings.TrimSpace(body.Name) == "" {
			http.Error(w, `{"message":"name is required"}`, http.StatusBadRequest)
			return
		}
		p := clockify.Project{ID: s.nextID("project"), Name: body.Name, ClientID: body.ClientID, Billable: body.Billable, Public: body.Public, WorkspaceID: s.WorkspaceID}
		s.projects[p.ID] = p
		s.tasks[p.ID] = map[string]clockify.Task{}
		writeJSON(w, p)
		return
	}
	if len(parts) == 4 && r.Method == http.MethodDelete {
		delete(s.projects, parts[3])
		delete(s.tasks, parts[3])
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if len(parts) == 4 && r.Method == http.MethodPut {
		project, ok := s.projects[parts[3]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		if err := decode(r, &body); err != nil {
			http.Error(w, `{"message":"invalid project body"}`, http.StatusBadRequest)
			return
		}
		if name, ok := body["name"].(string); ok && strings.TrimSpace(name) != "" {
			project.Name = name
		}
		if archived, ok := body["archived"].(bool); ok {
			project.Archived = archived
		}
		s.projects[project.ID] = project
		writeJSON(w, project)
		return
	}
	if len(parts) == 4 && r.Method == http.MethodGet {
		project, ok := s.projects[parts[3]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, project)
		return
	}
	if len(parts) >= 5 && parts[4] == "tasks" {
		s.handleTasksLocked(w, r, parts)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleTasksLocked(w http.ResponseWriter, r *http.Request, parts []string) {
	projectID := parts[3]
	if _, ok := s.projects[projectID]; !ok {
		http.NotFound(w, r)
		return
	}
	if _, ok := s.tasks[projectID]; !ok {
		s.tasks[projectID] = map[string]clockify.Task{}
	}
	if len(parts) == 5 && r.Method == http.MethodGet {
		out := make([]clockify.Task, 0, len(s.tasks[projectID]))
		for _, t := range s.tasks[projectID] {
			out = append(out, t)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
		out = paginate(out, r)
		writeJSON(w, out)
		return
	}
	if len(parts) == 5 && r.Method == http.MethodPost {
		var body struct {
			Name     string `json:"name"`
			Billable bool   `json:"billable"`
		}
		if err := decode(r, &body); err != nil || strings.TrimSpace(body.Name) == "" {
			http.Error(w, `{"message":"name is required"}`, http.StatusBadRequest)
			return
		}
		t := clockify.Task{ID: s.nextID("task"), Name: body.Name, ProjectID: projectID, Billable: body.Billable}
		s.tasks[projectID][t.ID] = t
		writeJSON(w, t)
		return
	}
	if len(parts) == 6 && r.Method == http.MethodDelete {
		delete(s.tasks[projectID], parts[5])
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if len(parts) == 6 && r.Method == http.MethodPut {
		task, ok := s.tasks[projectID][parts[5]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		if err := decode(r, &body); err != nil {
			http.Error(w, `{"message":"invalid task body"}`, http.StatusBadRequest)
			return
		}
		if name, ok := body["name"].(string); ok && strings.TrimSpace(name) != "" {
			task.Name = name
		}
		if billable, ok := body["billable"].(bool); ok {
			task.Billable = billable
		}
		if status, ok := body["status"].(string); ok {
			task.Status = status
		}
		s.tasks[projectID][task.ID] = task
		writeJSON(w, task)
		return
	}
	if len(parts) == 6 && r.Method == http.MethodGet {
		task, ok := s.tasks[projectID][parts[5]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, task)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleTags(w http.ResponseWriter, r *http.Request, parts []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(parts) == 3 && r.Method == http.MethodGet {
		out := make([]clockify.Tag, 0, len(s.tags))
		for _, t := range s.tags {
			out = append(out, t)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
		out = paginate(out, r)
		writeJSON(w, out)
		return
	}
	if len(parts) == 3 && r.Method == http.MethodPost {
		var body struct {
			Name string `json:"name"`
		}
		if err := decode(r, &body); err != nil || strings.TrimSpace(body.Name) == "" {
			http.Error(w, `{"message":"name is required"}`, http.StatusBadRequest)
			return
		}
		t := clockify.Tag{ID: s.nextID("tag"), Name: body.Name, WorkspaceID: s.WorkspaceID}
		s.tags[t.ID] = t
		writeJSON(w, t)
		return
	}
	if len(parts) == 4 && r.Method == http.MethodDelete {
		delete(s.tags, parts[3])
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if len(parts) == 4 && r.Method == http.MethodGet {
		tag, ok := s.tags[parts[3]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, tag)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleEntries(w http.ResponseWriter, r *http.Request, parts []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(parts) == 3 && r.Method == http.MethodPost {
		var body struct {
			Start       string   `json:"start"`
			End         string   `json:"end"`
			Description string   `json:"description"`
			ProjectID   string   `json:"projectId"`
			TaskID      string   `json:"taskId"`
			TagIDs      []string `json:"tagIds"`
			Billable    bool     `json:"billable"`
		}
		if err := decode(r, &body); err != nil || strings.TrimSpace(body.Start) == "" {
			http.Error(w, `{"message":"start is required"}`, http.StatusBadRequest)
			return
		}
		e := clockify.TimeEntry{
			ID:          s.nextID("entry"),
			Description: body.Description,
			ProjectID:   body.ProjectID,
			TaskID:      body.TaskID,
			TagIDs:      body.TagIDs,
			Billable:    body.Billable,
			UserID:      s.UserID,
			WorkspaceID: s.WorkspaceID,
			TimeInterval: clockify.TimeInterval{
				Start: body.Start,
				End:   body.End,
			},
		}
		if project, ok := s.projects[body.ProjectID]; ok {
			e.ProjectName = project.Name
		}
		s.entries[e.ID] = e
		writeJSON(w, e)
		return
	}
	if len(parts) == 4 && r.Method == http.MethodDelete {
		delete(s.entries, parts[3])
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if len(parts) == 4 && r.Method == http.MethodGet {
		entry, ok := s.entries[parts[3]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, entry)
		return
	}
	if len(parts) == 4 && (r.Method == http.MethodPut || r.Method == http.MethodPatch) {
		entry, ok := s.entries[parts[3]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		if err := decode(r, &body); err != nil {
			http.Error(w, `{"message":"invalid entry body"}`, http.StatusBadRequest)
			return
		}
		if description, ok := body["description"].(string); ok {
			entry.Description = description
		}
		if projectID, ok := body["projectId"].(string); ok {
			entry.ProjectID = projectID
			if project, exists := s.projects[projectID]; exists {
				entry.ProjectName = project.Name
			}
		}
		if taskID, ok := body["taskId"].(string); ok {
			entry.TaskID = taskID
		}
		if tagIDs := stringListFromAny(body["tagIds"]); len(tagIDs) > 0 {
			entry.TagIDs = tagIDs
		}
		if billable, ok := body["billable"].(bool); ok {
			entry.Billable = billable
		}
		if start, ok := body["start"].(string); ok {
			entry.TimeInterval.Start = start
		}
		if end, ok := body["end"].(string); ok {
			entry.TimeInterval.End = end
		}
		s.entries[entry.ID] = entry
		writeJSON(w, entry)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleUserEntries(w http.ResponseWriter, r *http.Request, parts []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(parts) == 5 && parts[3] == s.UserID && r.Method == http.MethodGet {
		projectFilter := r.URL.Query().Get("project")
		inProgress := r.URL.Query().Get("in-progress")
		out := make([]clockify.TimeEntry, 0, len(s.entries))
		for _, e := range s.entries {
			if projectFilter != "" && e.ProjectID != projectFilter {
				continue
			}
			if inProgress == "true" && e.TimeInterval.End != "" {
				continue
			}
			out = append(out, e)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
		out = paginate(out, r)
		writeJSON(w, out)
		return
	}
	if len(parts) == 5 && parts[3] == s.UserID && r.Method == http.MethodPatch {
		var body struct {
			End string `json:"end"`
		}
		if err := decode(r, &body); err != nil || strings.TrimSpace(body.End) == "" {
			http.Error(w, `{"message":"end is required"}`, http.StatusBadRequest)
			return
		}
		for id, e := range s.entries {
			if e.UserID == s.UserID && e.TimeInterval.End == "" {
				e.TimeInterval.End = body.End
				s.entries[id] = e
				writeJSON(w, e)
				return
			}
		}
		http.Error(w, `{"message":"no running timer"}`, http.StatusNotFound)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) == 3 && r.Method == http.MethodGet {
		writeJSON(w, []map[string]any{{
			"id":               s.UserID,
			"name":             "Test User",
			"email":            "test@example.com",
			"activeWorkspace":  s.WorkspaceID,
			"defaultWorkspace": s.WorkspaceID,
		}})
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleInvoices(w http.ResponseWriter, r *http.Request, parts []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(parts) == 3 && r.Method == http.MethodPost {
		var body map[string]any
		if err := decode(r, &body); err != nil {
			http.Error(w, `{"message":"invalid invoice body"}`, http.StatusBadRequest)
			return
		}
		invoice := map[string]any{
			"id":       s.nextID("invoice"),
			"clientId": body["clientId"],
			"number":   firstString(body, "number", "INV-FAKE"),
			"status":   "DRAFT",
		}
		s.invoices[fmt.Sprint(invoice["id"])] = invoice
		writeJSON(w, invoice)
		return
	}
	if len(parts) == 3 && r.Method == http.MethodGet {
		out := make([]map[string]any, 0, len(s.invoices))
		for _, invoice := range s.invoices {
			out = append(out, invoice)
		}
		writeJSON(w, out)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleExpenses(w http.ResponseWriter, r *http.Request, parts []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(parts) == 3 && r.Method == http.MethodPost {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, `{"message":"invalid expense form"}`, http.StatusBadRequest)
			return
		}
		expense := map[string]any{
			"id":         s.nextID("expense"),
			"userId":     r.FormValue("userId"),
			"categoryId": r.FormValue("categoryId"),
			"projectId":  r.FormValue("projectId"),
			"taskId":     r.FormValue("taskId"),
			"amount":     r.FormValue("amount"),
			"date":       r.FormValue("date"),
			"notes":      r.FormValue("notes"),
			"status":     "PENDING",
		}
		s.expenses[fmt.Sprint(expense["id"])] = expense
		writeJSON(w, expense)
		return
	}
	if len(parts) == 3 && r.Method == http.MethodGet {
		out := make([]map[string]any, 0, len(s.expenses))
		for _, expense := range s.expenses {
			out = append(out, expense)
		}
		writeJSON(w, out)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleTimeOff(w http.ResponseWriter, r *http.Request, parts []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(parts) == 6 && parts[3] == "policies" && parts[5] == "requests" && r.Method == http.MethodPost {
		var body map[string]any
		if err := decode(r, &body); err != nil {
			http.Error(w, `{"message":"invalid time off body"}`, http.StatusBadRequest)
			return
		}
		request := map[string]any{
			"id":       s.nextID("timeoff"),
			"policyId": parts[4],
			"userId":   s.UserID,
			"note":     firstString(body, "note", ""),
			"status":   "PENDING",
		}
		s.timeOff[fmt.Sprint(request["id"])] = request
		writeJSON(w, request)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleScheduling(w http.ResponseWriter, r *http.Request, parts []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(parts) == 5 && parts[3] == "assignments" && parts[4] == "recurring" && r.Method == http.MethodPost {
		var body map[string]any
		if err := decode(r, &body); err != nil {
			http.Error(w, `{"message":"invalid scheduling body"}`, http.StatusBadRequest)
			return
		}
		assignment := map[string]any{
			"id":          s.nextID("assignment"),
			"userId":      body["userId"],
			"projectId":   body["projectId"],
			"start":       body["start"],
			"end":         body["end"],
			"hoursPerDay": body["hoursPerDay"],
		}
		s.schedule[fmt.Sprint(assignment["id"])] = assignment
		writeJSON(w, []map[string]any{assignment})
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleWebhooks(w http.ResponseWriter, r *http.Request, parts []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(parts) == 3 && r.Method == http.MethodPost {
		var body map[string]any
		if err := decode(r, &body); err != nil {
			http.Error(w, `{"message":"invalid webhook body"}`, http.StatusBadRequest)
			return
		}
		webhook := map[string]any{
			"id":                 s.nextID("webhook"),
			"name":               body["name"],
			"url":                body["url"],
			"webhookEvent":       body["webhookEvent"],
			"triggerSourceType":  body["triggerSourceType"],
			"triggerSource":      body["triggerSource"],
			"webhookEventStatus": "ACTIVE",
		}
		s.webhooks[fmt.Sprint(webhook["id"])] = webhook
		writeJSON(w, webhook)
		return
	}
	if len(parts) == 3 && r.Method == http.MethodGet {
		out := make([]map[string]any, 0, len(s.webhooks))
		for _, webhook := range s.webhooks {
			out = append(out, webhook)
		}
		writeJSON(w, out)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) nextID(prefix string) string {
	s.seq++
	return fmt.Sprintf("%s-%d", prefix, s.seq)
}

func firstString(m map[string]any, key, fallback string) string {
	if value, ok := m[key].(string); ok && value != "" {
		return value
	}
	return fallback
}

func stringListFromAny(value any) []string {
	switch v := value.(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok && str != "" {
				out = append(out, str)
			}
		}
		return out
	default:
		return nil
	}
}

func splitPath(path string) []string {
	raw := strings.Split(strings.Trim(path, "/"), "/")
	out := raw[:0]
	for _, p := range raw {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func decode(r *http.Request, out any) error {
	defer func() { _ = r.Body.Close() }()
	return json.NewDecoder(r.Body).Decode(out)
}

func writeJSON(w http.ResponseWriter, v any) {
	_ = json.NewEncoder(w).Encode(v)
}

func paginate[T any](items []T, r *http.Request) []T {
	page := queryInt(r, "page", 1)
	pageSize := queryInt(r, "page-size", len(items))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = len(items)
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []T{}
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

func queryInt(r *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}
