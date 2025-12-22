package templates_test

import (
	"testing"

	"github.com/glitchedgitz/grroxy-db/grx/templates"
)

func TestParseVariable(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]any
		input    string
		expected string
	}{
		{
			name: "simple variable",
			data: map[string]any{
				"name": "test",
			},
			input:    "Hello {{name}}",
			expected: "Hello test",
		},
		{
			name: "nested variable",
			data: map[string]any{
				"user": map[string]any{
					"name": "john",
				},
			},
			input:    "Hello {{user.name}}",
			expected: "Hello john",
		},
		{
			name: "multiple variables",
			data: map[string]any{
				"first": "john",
				"last":  "doe",
			},
			input:    "Name: {{first}} {{last}}",
			expected: "Name: john doe",
		},
		{
			name: "non-string values",
			data: map[string]any{
				"age":    25,
				"active": true,
			},
			input:    "Age: {{age}}, Active: {{active}}",
			expected: "Age: 25, Active: true",
		},
		{
			name: "missing variable",
			data: map[string]any{
				"name": "test",
			},
			input:    "Hello {{missing}}",
			expected: "Hello <nil>",
		},
		{
			name: "no variables",
			data: map[string]any{
				"name": "test",
			},
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name: "nested request extension",
			data: map[string]any{
				"req": map[string]any{
					"ext":  ".pdf",
					"path": "/some/path",
				},
			},
			input:    "File extension is {{req.ext}}",
			expected: "File extension is .pdf",
		},
		{
			name: "multiple nested variables",
			data: map[string]any{
				"req": map[string]any{
					"ext":  ".pdf",
					"path": "/documents/report",
				},
			},
			input:    "Path: {{req.path}}, Type: {{req.ext}}",
			expected: "Path: /documents/report, Type: .pdf",
		},
		{
			name: "deeply nested variable",
			data: map[string]any{
				"req": map[string]any{
					"headers": map[string]any{
						"content": map[string]any{
							"type": "application/json",
						},
					},
				},
			},
			input:    "Content-Type: {{req.headers.content.type}}",
			expected: "Content-Type: application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := templates.ParseVariable(&tt.data, tt.input)
			if result != tt.expected {
				t.Errorf("ParseVariable() = %v, want %v", result, tt.expected)
			}
		})
	}
}
