export const cert_api = [
	{
		id: 'cert',
		name: 'Download CA Cert',
		method: 'GET',
		path: '/cacert.crt',
		description: 'Download CA certificate for HTTPS interception',
		category: '/cacert',
		examples: [
			{
				name: 'Download certificate',
				description: 'Successfully downloads the CA certificate file used for HTTPS interception',
				request: {},
				response: {
					status: 200,
					body: '<binary file: grroxy-ca.crt>'
				}
			}
		]
	}
];

export const commands_api = [
	{
		id: 'commands',
		name: 'Run Command',
		method: 'POST',
		path: '/api/runcommand',
		description: 'Execute a shell command (requires auth)',
		category: '/api/commands',
		defaultBody: {
			command: 'echo hello',
			data: '',
			saveTo: 'collection',
			collection: 'cmd_output',
			filename: ''
		},
		examples: [
			{
				name: 'Save output to collection',
				description: 'Runs a command and saves each stdout line as a record in the specified collection',
				request: {
					command: 'echo hello',
					data: '',
					saveTo: 'collection',
					collection: 'cmd_output',
					filename: ''
				},
				response: {
					status: 200,
					body: { id: 'abc123' }
				}
			}
		]
	}
];

export const cook_api = [
	{
		id: 'cook',
		name: 'Search Patterns',
		method: 'POST',
		path: '/api/cook/search',
		description: 'Search available Cook patterns and methods (requires auth)',
		category: '/api/cook',
		defaultBody: {
			search: 'encode'
		},
		examples: [
			{
				name: 'Search found',
				description: 'Returns matching Cook patterns/methods for the search query',
				request: { search: 'encode' },
				response: {
					status: 200,
					body: { search: 'encode', results: ['url_encode', 'base64_encode', 'html_encode'] }
				}
			}
		]
	}
];

export const file_api = [
	{
		id: 'file',
		name: 'Read File',
		method: 'POST',
		path: '/api/readfile',
		description: 'Read file contents from cache, config, or cwd folder (requires auth)',
		category: '/api/files',
		defaultBody: {
			fileName: 'output.txt',
			folder: 'cache'
		},
		examples: [
			{
				name: 'Read a file',
				description: 'Reads file content from the specified folder (cache, config, or cwd)',
				request: { fileName: 'output.txt', folder: 'cache' },
				response: {
					status: 200,
					body: { filecontent: '<file contents>' }
				}
			}
		]
	},
	{
		id: 'file',
		name: 'Save File',
		method: 'POST',
		path: '/api/savefile',
		description: 'Save data to a file in cache, config, or cwd folder (requires auth)',
		category: '/api/files',
		defaultBody: {
			fileName: 'output.txt',
			fileData: 'hello world',
			folder: 'cache'
		},
		examples: [
			{
				name: 'Save a file',
				description: 'Writes fileData to the specified file in the given folder (cache, config, or cwd)',
				request: { fileName: 'output.txt', fileData: 'hello world', folder: 'cache' },
				response: {
					status: 200,
					body: { filepath: '/path/to/cache/output.txt' }
				}
			}
		]
	}
];

export const filewatcher_api = [
	{
		id: 'filewatcher',
		name: 'File Watcher',
		method: 'GET',
		path: '/api/filewatcher',
		description: 'Watch GRROXY_TEMPLATE_DIR for changes via SSE (Server-Sent Events)',
		category: '/api/filewatcher',
		examples: [
			{
				name: 'Watch for file changes',
				description: 'Opens an SSE stream that emits fsnotify events when the template directory changes',
				request: {},
				response: {
					status: 200,
					body: 'data: {"Name":"/path/to/file.txt","Op":"WRITE"}\n\n'
				}
			}
		]
	}
];

export const projects_api = [
	{
		id: 'projects',
		name: 'List Projects',
		method: 'GET',
		path: '/api/project/list',
		description: 'List all projects from the _projects collection',
		category: '/api/project',
		examples: [
			{
				name: 'List projects',
				description: 'Returns all project records with their name, path, and state data',
				request: {},
				response: {
					status: 200,
					body: [{ id: 'abc123', name: 'my-project', path: '/path/to/projects/abc123', data: { ip: '127.0.0.1:8091', state: 'active' } }]
				}
			}
		]
	},
	{
		id: 'projects',
		name: 'Create Project',
		method: 'POST',
		path: '/api/project/new',
		description: 'Create a new project and start its grroxy-app instance',
		category: '/api/project',
		defaultBody: {
			name: 'my-project'
		},
		examples: [
			{
				name: 'Create project',
				description: 'Creates project directory, saves to DB, and starts grroxy-app on an available port',
				request: { name: 'my-project' },
				response: {
					status: 200,
					body: { id: 'abc123', name: 'my-project', path: '/path/to/projects/abc123', data: { ip: '127.0.0.1:8091', state: 'active' } }
				}
			}
		]
	},
	{
		id: 'projects',
		name: 'Open Project',
		method: 'POST',
		path: '/api/project/open',
		description: 'Open an existing project by name or ID',
		category: '/api/project',
		defaultBody: {
			project: ''
		},
		examples: [
			{
				name: 'Open project',
				description: 'Opens the project (or returns existing data if already running)',
				request: { project: 'my-project' },
				response: {
					status: 200,
					body: { id: 'abc123', name: 'my-project', path: '/path/to/projects/abc123', data: { ip: '127.0.0.1:8091', state: 'active' } }
				}
			}
		]
	}
];

export const regex_api = [
	{
		id: 'regex',
		name: 'Regex Match',
		method: 'POST',
		path: '/api/regex',
		description: 'Test a regex pattern against a response body',
		category: '/api/regex',
		defaultBody: {
			regex: '<title>(.*?)</title>',
			responseBody: '<html><title>Test</title></html>'
		},
		examples: [
			{
				name: 'Test regex match',
				description: 'Tests whether the regex matches the provided response body string',
				request: { regex: '<title>(.*?)</title>', responseBody: '<html><title>Test</title></html>' },
				response: {
					status: 200,
					body: { matched: true }
				}
			}
		]
	}
];

export const sqltest_api = [
	{
		id: 'sqltest',
		name: 'SQL Query',
		method: 'POST',
		path: '/api/sqltest',
		description: 'Execute a raw SQL query against the database (requires auth)',
		category: '/api/sqltest',
		defaultBody: {
			sql: 'SELECT * FROM _projects LIMIT 10'
		},
		examples: [
			{
				name: 'Run SQL query',
				description: 'Executes the SQL query and returns rows as newline-delimited JSON',
				request: { sql: 'SELECT * FROM _projects LIMIT 10' },
				response: {
					status: 200,
					body: '{"id":"abc123","name":"my-project"}\n...'
				}
			}
		]
	}
];

export const tools_api = [
	{
		id: 'tools',
		name: 'Start Tool Server',
		method: 'GET',
		path: '/api/tool/server',
		description: 'Start or retrieve a grroxy-tool server instance by ID',
		category: '/api/tool',
		examples: [
			{
				name: 'Start tool server',
				description: 'Starts a new grroxy-tool server or returns existing one if already active',
				request: { id: 'abc123' },
				response: {
					status: 200,
					body: { path: '/path/to/projects', host: '127.0.0.1:9001', id: 'abc123', name: 'my-tool', username: 'new@example.com', password: '1234567890' }
				}
			}
		]
	},
	{
		id: 'tools',
		name: 'Start Tool Instance',
		method: 'GET',
		path: '/api/tool',
		description: 'Start a new PocketBase tool instance at the given path',
		category: '/api/tool',
		examples: [
			{
				name: 'Start tool instance',
				description: 'Bootstraps a new PocketBase instance and serves it on an available port',
				request: { path: '/path/to/project' },
				response: {
					status: 200,
					body: 'Path parameter: /path/to/project'
				}
			}
		]
	}
];

export const templates_api = [
	{
		id: 'templates',
		name: 'List Templates',
		method: 'GET',
		path: '/api/templates/list',
		description: 'List all YAML template files in the templates directory',
		category: '/api/templates',
		examples: [
			{
				name: 'List templates',
				description: 'Returns all .yaml/.yml files from the templates directory',
				request: {},
				response: {
					status: 200,
					body: { list: [{ name: 'template.yaml', path: '/path/to/templates/template.yaml', is_dir: false }] }
				}
			}
		]
	},
	{
		id: 'templates',
		name: 'Create Template',
		method: 'POST',
		path: '/api/templates/new',
		description: 'Create or overwrite a template file',
		category: '/api/templates',
		defaultBody: {
			name: 'my-template.yaml',
			content: 'id: my-template\ninfo:\n  name: My Template'
		},
		examples: [
			{
				name: 'Create template',
				description: 'Writes content to a file in the templates directory',
				request: { name: 'my-template.yaml', content: 'id: my-template' },
				response: {
					status: 200,
					body: { filepath: '/path/to/templates/my-template.yaml' }
				}
			}
		]
	},
	{
		id: 'templates',
		name: 'Delete Template',
		method: 'DELETE',
		path: '/api/templates/:template',
		description: 'Delete a template file by name',
		category: '/api/templates',
		examples: [
			{
				name: 'Delete template',
				description: 'Removes the specified template file from the templates directory',
				request: {},
				response: {
					status: 200,
					body: ''
				}
			}
		]
	}
];

export const update_api = [
	{
		id: 'update',
		name: 'Check for Updates',
		method: 'GET',
		path: '/api/update/check',
		description: 'Check if a newer version of grroxy is available',
		category: '/api/update',
		examples: [
			{
				name: 'Check update',
				description: 'Returns current version, latest version, and whether an update is available',
				request: {},
				response: {
					status: 200,
					body: { current_version: 'v2026.3.6', latest_version: 'v2026.3.7', update_available: true, platform: 'darwin/arm64' }
				}
			}
		]
	},
	{
		id: 'update',
		name: 'Perform Update',
		method: 'POST',
		path: '/api/update',
		description: 'Download and install the latest version of all grroxy binaries',
		category: '/api/update',
		examples: [
			{
				name: 'Update binaries',
				description: 'Downloads and replaces grroxy, grroxy-app, and grroxy-tool binaries',
				request: {},
				response: {
					status: 200,
					body: { previous_version: 'v2026.3.6', new_version: 'v2026.3.7', results: [{ binary: 'grroxy', status: 'updated', path: '/usr/local/bin/grroxy' }] }
				}
			}
		]
	}
];

export const launcherEndpoints = [
	...cert_api,
	...commands_api,
	...cook_api,
	...file_api,
	...filewatcher_api,
	...projects_api,
	...regex_api,
	...sqltest_api,
	...templates_api,
	...tools_api,
	...update_api
];
