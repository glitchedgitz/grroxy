package schemas

import "github.com/pocketbase/pocketbase/models/schema"

type Process struct {
	ID          string         `db:"id" json:"id"`
	Name        string         `db:"name" json:"name"`
	Description string         `db:"description" json:"description"`
	Type        string         `db:"type" json:"type"`
	Input       map[string]any `db:"input" json:"input"`
	Output      map[string]any `db:"output" json:"output"`
	Data        map[string]any `db:"data" json:"data"`
	State       string         `db:"state" json:"state"`
}

var ProcessState = struct {
	Inqueue   string
	Running   string
	Completed string
	Killed    string
}{
	Inqueue:   "In Queue",
	Running:   "Running",
	Completed: "Completed",
	Killed:    "Killed",
}

var PROCESSES = schema.NewSchema(
	&schema.SchemaField{
		Name:     "name",
		Type:     schema.FieldTypeText,
		Required: true,
	},
	&schema.SchemaField{
		Name: "description",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "type",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "data",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	&schema.SchemaField{
		Name: "input",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	&schema.SchemaField{
		Name: "output",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
	// running, queue, error, killed, completed
	&schema.SchemaField{
		Name: "state",
		Type: schema.FieldTypeText,
	},
)
