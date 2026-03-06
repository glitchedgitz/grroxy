package schemas

import (
	"github.com/glitchedgitz/pocketbase/models/schema"
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
		"_req",
		RequestData,
		true,
		`
		CREATE INDEX idx_req_method ON _req (method);
		`,
	},
	{
		"_resp",
		ResponseData,
		true,
		`
		CREATE INDEX idx_resp_status ON _resp (status);
		`,
	},
	{
		"_req_edited",
		RequestData,
		true,
		`
		CREATE INDEX idx_req_edited_method ON _req_edited (method);
		`,
	},
	{
		"_resp_edited",
		ResponseData,
		true,
		`
		CREATE INDEX idx_resp_edited_status ON _resp_edited (status);
		`,
	},
	{
		"_data",
		Rows,
		true,
		`
		CREATE INDEX idx_data_generated_by ON _data (generated_by);
		`,
	},
	{
		"_proxies",
		Proxies,
		false,
		"",
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
		true,
		`
		CREATE UNIQUE INDEX idx_hosts_name ON _hosts (host);
		`,
	},
	{
		"_settings",
		Settings,
		false,
		"",
	},
	{
		"_configs",
		ConfigSchema,
		true,
		`
		CREATE UNIQUE INDEX idx_configs_key ON _configs (key);
		`,
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
	// {
	// 	"_websockets",
	// 	Websockets,
	// 	true,
	// 	`
	// 	CREATE INDEX idx_websockets_reqid ON _websockets (request_id);
	// 	`,
	// },
}
