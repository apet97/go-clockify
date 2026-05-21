package tools

import (
	"context"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

func TestEveryResourceHasATitle(t *testing.T) {
	svc := New(clockify.NewClient("k", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	resources, err := svc.ListResources(context.Background())
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	if len(resources) == 0 {
		t.Fatal("ListResources returned nothing")
	}
	for _, r := range resources {
		if r.Title == "" {
			t.Errorf("resource %q has no Title", r.URI)
		}
	}
}

func TestEveryResourceTemplateHasATitle(t *testing.T) {
	svc := New(clockify.NewClient("k", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	templates, err := svc.ListResourceTemplates(context.Background())
	if err != nil {
		t.Fatalf("ListResourceTemplates: %v", err)
	}
	if len(templates) == 0 {
		t.Fatal("ListResourceTemplates returned nothing")
	}
	for _, tmpl := range templates {
		if tmpl.Title == "" {
			t.Errorf("resource template %q has no Title", tmpl.URITemplate)
		}
	}
}
