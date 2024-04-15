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

var Collections = []DB{
	{
		"_raw",
		Store,
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
