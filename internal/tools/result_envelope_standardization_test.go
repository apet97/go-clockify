package tools

import "testing"

func TestResultEnvelopeMetaStandardizationPreservesListMetadata(t *testing.T) {
	actions := []string{
		"clockify_invoices_list",
		"clockify_expenses_list",
		"clockify_webhooks_list",
		"clockify_invoices_payments_list",
	}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			env := ResultEnvelope{
				OK:     true,
				Action: action,
				Data: []map[string]any{
					{"id": "entity-1"},
				},
				Meta: map[string]any{
					"count":    1,
					"page":     2,
					"pageSize": 50,
					"pagination": map[string]any{
						"page":              2,
						"page_size":         50,
						"applied_page_size": 50,
					},
				},
			}

			got := standardizeDomainResult(action, "entity", "", env, nil)
			if got.Meta == nil {
				t.Fatalf("meta was dropped: %+v", got)
			}
			for _, key := range []string{"count", "page", "pageSize", "pagination"} {
				if _, ok := got.Meta[key]; !ok {
					t.Fatalf("meta[%s] missing after standardization: %+v", key, got.Meta)
				}
			}
			if got.Meta["count"] != 1 || got.Meta["page"] != 2 || got.Meta["pageSize"] != 50 {
				t.Fatalf("scalar list metadata changed: %+v", got.Meta)
			}
			if _, ok := got.Meta["pagination"].(map[string]any); !ok {
				t.Fatalf("pagination metadata was not preserved as an object: %+v", got.Meta["pagination"])
			}
		})
	}
}

func TestResultEnvelopeMetaStandardizationStillLiftsStringIDs(t *testing.T) {
	env := ResultEnvelope{
		OK:     true,
		Action: "clockify_invoices_get",
		Data:   map[string]any{"number": "INV-1"},
		Meta: map[string]any{
			"invoiceId": "invoice-1",
			"count":     1,
		},
	}

	got := standardizeDomainResult("clockify_invoices_get", "invoice", "", env, nil)
	if got.IDs["invoiceId"] != "invoice-1" {
		t.Fatalf("string meta ID was not lifted into ids: %+v", got.IDs)
	}
	if got.Meta["invoiceId"] != "invoice-1" || got.Meta["count"] != 1 {
		t.Fatalf("meta was not preserved alongside lifted IDs: %+v", got.Meta)
	}
}
