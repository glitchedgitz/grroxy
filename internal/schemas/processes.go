package schemas

import "github.com/glitchedgitz/pocketbase/models/schema"

type Process struct {
	ID          string         `db:"id" json:"id"`
	Name        string         `db:"name" json:"name"`
	Description string         `db:"description" json:"description"`
	Type        string         `db:"type" json:"type"`
	Input       map[string]any `db:"input" json:"input"`
	Output      map[string]any `db:"output" json:"output"`
	Data        map[string]any `db:"data" json:"data"`
	State       string         `db:"state" json:"state"`
	ParentID    string         `db:"parent_id" json:"parent_id"`
	GeneratedBy string         `db:"generated_by" json:"generated_by"`
	CreatedBy   string         `db:"created_by" json:"created_by"`
}

var ProcessState = struct {
	Inqueue   string
	Running   string
	Completed string
	Killed    string
	Failed    string
	Paused    string
}{
	Inqueue:   "In Queue",
	Running:   "Running",
	Completed: "Completed",
	Killed:    "Killed",
	Failed:    "Failed",
	Paused:    "Paused",
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
	&schema.SchemaField{
		Name: "parent_id",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "generated_by",
		Type: schema.FieldTypeText,
	},
	&schema.SchemaField{
		Name: "created_by",
		Type: schema.FieldTypeText,
	},
)
