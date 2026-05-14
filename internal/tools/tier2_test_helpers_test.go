package tools

import (
	"github.com/apet97/go-clockify/internal/mcp"
)

// tier2GroupBuilders maps legacy family names onto the handler builders
// they grouped. The product binary now ships every tool in
// FullAccessRegistry at startup, but tests still find it convenient to
// fetch descriptors by family.
var tier2GroupBuilders = map[string]func(*Service) []mcp.ToolDescriptor{
	"invoices":        invoiceHandlers,
	"expenses":        expenseHandlers,
	"custom_fields":   customFieldHandlers,
	"time_off":        timeOffHandlers,
	"scheduling":      schedulingHandlers,
	"approvals":       approvalHandlers,
	"webhooks":        webhookHandlers,
	"groups_holidays": groupsHolidaysHandlers,
	"project_admin":   projectAdminHandlers,
	"user_admin":      userAdminHandlers,
	"shared_reports":  sharedReportHandlers,
	"change_tracking": changeTrackingHandlers,
	"probe_lab_api":   probeLabAPIHandlers,
}

func tier2Handlers(svc *Service, name string) ([]mcp.ToolDescriptor, bool) {
	build, ok := tier2GroupBuilders[name]
	if !ok {
		return nil, false
	}
	return applyOpaqueOutputSchemas(normalizeDescriptors(build(svc))), true
}
