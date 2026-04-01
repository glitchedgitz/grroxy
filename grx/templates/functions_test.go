package templates_test

import (
	"testing"

	"github.com/glitchedgitz/grroxy/grx/templates"
)

func TestParseTemplateActions_Disabled(t *testing.T) {
	tasks := []templates.Actions{
		{
			Id:        "enabled-task",
			Condition: "",
			Disabled:  false,
			Todo: []map[string]map[string]any{
				{"create_label": {"name": "test"}},
			},
		},
		{
			Id:        "disabled-task",
			Condition: "",
			Disabled:  true,
			Todo: []map[string]map[string]any{
				{"create_label": {"name": "should-not-appear"}},
			},
		},
	}

	results, err := templates.ParseTemplateActions(tasks, map[string]any{}, "all")
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 action, got %d", len(results))
	}

	if results[0].Data["name"] != "test" {
		t.Fatalf("expected label name 'test', got %v", results[0].Data["name"])
	}
}

func TestParseTemplateActions_DefaultFallback(t *testing.T) {
	tasks := []templates.Actions{
		{
			Id:        "default",
			Condition: "",
			Todo: []map[string]map[string]any{
				{"create_label": {"name": "fallback-label", "color": "blue"}},
			},
		},
		{
			Id:        "specific-task",
			Condition: "req.ext = '.nope'", // won't match
			Todo: []map[string]map[string]any{
				{"create_label": {"name": "specific"}},
			},
		},
	}

	data := map[string]any{
		"req": map[string]any{"ext": ".js"},
	}

	results, err := templates.ParseTemplateActions(tasks, data, "all")
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 default action, got %d", len(results))
	}

	if results[0].Data["name"] != "fallback-label" {
		t.Fatalf("expected 'fallback-label', got %v", results[0].Data["name"])
	}
}

func TestParseTemplateActions_ModeAny(t *testing.T) {
	tasks := []templates.Actions{
		{
			Id:        "task-1",
			Condition: "req.ext = '.js'",
			Todo: []map[string]map[string]any{
				{"create_label": {"name": "first"}},
			},
		},
		{
			Id:        "task-2",
			Condition: "req.ext = '.js'",
			Todo: []map[string]map[string]any{
				{"create_label": {"name": "second"}},
			},
		},
	}

	data := map[string]any{
		"req": map[string]any{"ext": ".js"},
	}

	results, err := templates.ParseTemplateActions(tasks, data, "any")
	if err != nil {
		t.Fatal(err)
	}

	// "any" mode should stop after first match
	if len(results) != 1 {
		t.Fatalf("expected 1 action in 'any' mode, got %d", len(results))
	}
}

func TestParseTemplateActions_ModeAll(t *testing.T) {
	tasks := []templates.Actions{
		{
			Id:        "task-1",
			Condition: "req.ext = '.js'",
			Todo: []map[string]map[string]any{
				{"create_label": {"name": "first"}},
			},
		},
		{
			Id:        "task-2",
			Condition: "req.ext = '.js'",
			Todo: []map[string]map[string]any{
				{"create_label": {"name": "second"}},
			},
		},
	}

	data := map[string]any{
		"req": map[string]any{"ext": ".js"},
	}

	results, err := templates.ParseTemplateActions(tasks, data, "all")
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 actions in 'all' mode, got %d", len(results))
	}
}

func TestParseTemplateActions_ConditionNoMatch(t *testing.T) {
	tasks := []templates.Actions{
		{
			Id:        "task-1",
			Condition: "req.ext = '.pdf'",
			Todo: []map[string]map[string]any{
				{"create_label": {"name": "pdf-label"}},
			},
		},
	}

	data := map[string]any{
		"req": map[string]any{"ext": ".js"},
	}

	results, err := templates.ParseTemplateActions(tasks, data, "all")
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 0 {
		t.Fatalf("expected 0 actions, got %d", len(results))
	}
}

func TestParseTemplateActions_VariableInterpolation(t *testing.T) {
	tasks := []templates.Actions{
		{
			Id:        "task-1",
			Condition: "",
			Todo: []map[string]map[string]any{
				{"create_label": {"name": "{{req.ext}}", "type": "extension"}},
			},
		},
	}

	data := map[string]any{
		"req": map[string]any{"ext": ".css"},
	}

	results, err := templates.ParseTemplateActions(tasks, data, "all")
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 action, got %d", len(results))
	}

	if results[0].Data["name"] != ".css" {
		t.Fatalf("expected interpolated name '.css', got %v", results[0].Data["name"])
	}
}

func TestParseTemplateActions_MultipleActionsPerTask(t *testing.T) {
	tasks := []templates.Actions{
		{
			Id:        "task-1",
			Condition: "",
			Todo: []map[string]map[string]any{
				{"set": {"req.headers.X-Custom": "value1"}},
				{"create_label": {"name": "my-label"}},
			},
		},
	}

	results, err := templates.ParseTemplateActions(tasks, map[string]any{}, "all")
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(results))
	}

	if results[0].ActionName != "set" {
		t.Fatalf("expected first action 'set', got %q", results[0].ActionName)
	}

	if results[1].ActionName != "create_label" {
		t.Fatalf("expected second action 'create_label', got %q", results[1].ActionName)
	}
}

func TestParseTemplateActions_EmptyTasks(t *testing.T) {
	results, err := templates.ParseTemplateActions([]templates.Actions{}, map[string]any{}, "all")
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 0 {
		t.Fatalf("expected 0 actions, got %d", len(results))
	}
}
