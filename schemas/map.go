package schemas

import (
	"github.com/pocketbase/pocketbase/models/schema"
)

type DB struct {
	Name     string
	Schema   schema.Schema
	HasIndex bool
	Index    string
}

//	Indexes: types.JsonArray[string]{
//	       "CREATE UNIQUE INDEX idx_user ON example (user)",
//	   },

var Main = []DB{
	{
		"_projects",
		Projects,
		true,
		"CREATE UNIQUE INDEX idx_projects_name ON _projects (name);",
	},
	{
		"_processes",
		PROCESSES,
		false,
		"",
	},
	{
		"_tools",
		ToolsSchema,
		false,
		"",
	},
	{
		"_labels",
		Labels,
		false,
		"",
	},
	{
		"_searches",
		Searches,
		true,
		`
		CREATE UNIQUE INDEX idx_searches_name ON _searches (name);
		`,
	},
	{
		"_wordlists",
		Wordlists,
		true,
		`
		CREATE UNIQUE INDEX idx_wordlists_name ON _wordlists (name);
		`,
	},
	{
		"_filters",
		Filters,
		true,
		`
		CREATE UNIQUE INDEX idx_filters_name ON _filters (name);
		`,
	},
	{
		"_payloads",
		Payloads,
		false,
		"",
	},
	{
		"_settings",
		Settings,
		false,
		"",
	},
}

var Tools = []DB{
	{
		"_processes",
		PROCESSES,
		false,
		"",
	},
}

var Collections = []DB{
	{
		"_raw",
		Raw,
		true,
		`
		CREATE UNIQUE INDEX idx_hosts_host ON _hosts (host);
		`,
	},
	{
		"_data",
		Rows,
		false,
		`
		CREATE UNIQUE INDEX idx_data_index ON _data (index);
		`,
	},
	{
		"_labels",
		Labels,
		true, `
		CREATE UNIQUE INDEX idx_labelsname ON _labels (name);
		`,
	},
	{
		"_searches",
		Searches,
		true,
		`
		CREATE UNIQUE INDEX idx_searches_name ON _searches (name);
		`,
	},
	{
		"_filters",
		Filters,
		true,
		`
		CREATE UNIQUE INDEX idx_filters_name ON _filters (name);
		`,
	},
	{
		"_wordlists",
		Wordlists,
		true,
		`
		CREATE UNIQUE INDEX idx_wordlists_name ON _wordlists (name);
		`,
	},
	{
		"_playground",
		Playground,
		false,
		"",
	},
	{
		"_tech",
		Tech,
		true, `
		CREATE UNIQUE INDEX idx_techname ON _tech (name);
		`,
	},
	{
		"_intercept",
		Intercept,
		false,
		"",
	},
	{
		"_hosts",
		Sites,
		false,
		"",
	},
	{
		"_settings",
		Settings,
		false,
		"",
	},
	{
		"_processes",
		PROCESSES,
		false,
		"",
	},
	{
		"_ui",
		UI,
		true, `
		CREATE UNIQUE INDEX idx_ui_id ON _ui (unique_id);
		`,
	},
	{
		"_attached",
		Attached,
		false,
		"",
	},
}
