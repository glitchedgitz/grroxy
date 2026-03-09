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
		name: 'Generate Patterns',
		method: 'POST',
		path: '/api/cook/generate',
		description: 'Generate strings from Cook pattern syntax (requires auth)',
		category: '/api/cook',
		defaultBody: {
			pattern: ['intigriti,bugcrowd,hackerone _,- users.rar,secret.zip']
		},
		examples: [
			{
				name: 'Comma and space combined',
				description: 'Comma separates alternatives, space concatenates — generates all combinations',
				request: {
					pattern: ['intigriti,bugcrowd,hackerone _,- users.rar,secret.zip']
				},
				response: {
					status: 200,
					body: { results: [
						'intigriti_users.rar', 'intigriti-users.rar',
						'bugcrowd_users.rar', 'bugcrowd-users.rar',
						'hackerone_users.rar', 'hackerone-users.rar'
					]}
				}
			}
		]
	},
	{
		id: 'cook',
		name: 'Apply Methods',
		method: 'POST',
		path: '/api/cook/apply',
		description: 'Apply transformation methods to strings (requires auth)',
		category: '/api/cook',
		defaultBody: {
			strings: ['example', 'TEST'],
			methods: ['b64e', 'urle']
		},
		examples: [
			{
				name: 'Base64 encode',
				description: 'Applies b64e method to base64-encode each input string',
				request: {
					strings: ['hello world'],
					methods: ['b64e']
				},
				response: {
					status: 200,
					body: { results: ['aGVsbG8gd29ybGQ='] }
				}
			}
		]
	},
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
				request: {
					search: 'encode'
				},
				response: {
					status: 200,
					body: { search: 'encode', results: ['url_encode', 'base64_encode', 'html_encode'] }
				}
			}
		]
	}
];

export const extractor_api = [
	{
		id: 'extractor',
		name: 'Extract Data',
		method: 'POST',
		path: '/api/extract',
		description: 'Extract specified fields from intercepted request/response records for a host (requires auth)',
		category: '/api/extract',
		defaultBody: {
			host: 'example.com',
			fields: ['req.method', 'req.url', 'req.path', 'req.params'],
			outputFile: ''
		},
		examples: [
			{
				name: 'Extract fields for a host',
				description: 'Extracts specified fields (req.*, resp.*, req_edited.*, resp_edited.*) from intercepted records',
				request: {
					host: 'example.com',
					fields: ['req.method', 'req.url', 'resp.status'],
					outputFile: 'extract.jsonl'
				},
				response: {
					status: 200,
					body: {
						success: true,
						filePath: '/path/to/projects/project1/example_com/extract.jsonl',
						host: 'example.com',
						fields: ['req.method', 'req.url', 'resp.status'],
						extractedAt: '2026-03-09T12:00:00Z'
					}
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
				request: {
					fileName: 'output.txt',
					folder: 'cache'
				},
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
				request: {
					fileName: 'output.txt',
					fileData: 'hello world',
					folder: 'cache'
				},
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
		name: 'CWD Content',
		method: 'GET',
		path: '/api/cwd',
		description: 'List files and directories in the current project directory',
		category: '/api/filewatcher',
		examples: [
			{
				name: 'List project directory',
				description: 'Returns the current working directory path and its file/directory listing',
				request: {},
				response: {
					status: 200,
					body: {
						cwd: '/path/to/projects/project1',
						list: [
							{ name: 'example_com', path: '/path/to/projects/project1/example_com', isDir: true },
							{ name: 'output.txt', path: '/path/to/projects/project1/output.txt', isDir: false }
						]
					}
				}
			}
		]
	},
	{
		id: 'filewatcher',
		name: 'File Watcher',
		method: 'POST',
		path: '/api/filewatcher',
		description: 'Watch a file/directory for changes via SSE (Server-Sent Events)',
		category: '/api/filewatcher',
		defaultBody: {
			filePath: '/path/to/watch'
		},
		examples: [
			{
				name: 'Watch for file changes',
				description: 'Opens an SSE stream that emits fsnotify events when the watched path changes',
				request: {
					filePath: '/path/to/projects/project1'
				},
				response: {
					status: 200,
					body: 'data: {"Name":"/path/to/file.txt","Op":"WRITE"}\n\n'
				}
			}
		]
	}
];

export const filters_api = [
	{
		id: 'filters',
		name: 'Filter Check',
		method: 'POST',
		path: '/api/filter/check',
		description: 'Evaluate a dadql filter expression against a columns map (requires auth)',
		category: '/api/filter',
		defaultBody: {
			filter: 'status == 200',
			columns: { status: 200, method: 'GET', path: '/api/test' }
		},
		examples: [
			{
				name: 'Check filter match',
				description: 'Evaluates the dadql filter against the provided columns and returns whether it matches',
				request: {
					filter: 'status == 200',
					columns: { status: 200, method: 'GET' }
				},
				response: {
					status: 200,
					body: { ok: true, match: true }
				}
			}
		]
	}
];

export const http_api = [
	{
		id: 'http',
		name: 'Send Raw Request',
		method: 'POST',
		path: '/api/sendrawrequest',
		description: 'Send a raw HTTP/1 or HTTP/2 request to a host (requires auth)',
		category: '/api/http',
		defaultBody: {
			host: 'example.com',
			port: '443',
			tls: true,
			req: 'GET / HTTP/1.1\r\nHost: example.com\r\n\r\n',
			timeout: 10,
			httpversion: 1
		},
		examples: [
			{
				name: 'Send raw HTTP request',
				description: 'Sends a raw HTTP request and returns the response with timing',
				request: {
					host: 'example.com',
					port: '443',
					tls: true,
					req: 'GET / HTTP/1.1\r\nHost: example.com\r\n\r\n',
					timeout: 10,
					httpversion: 1
				},
				response: {
					status: 200,
					body: { resp: 'HTTP/1.1 200 OK\r\n...', time: '120ms' }
				}
			}
		]
	}
];

export const info_api = [
	{
		id: 'info',
		name: 'App Info',
		method: 'GET',
		path: '/api/info',
		description: 'Get application info including version, paths, and project details',
		category: '/api/info',
		examples: [
			{
				name: 'Get app info',
				description: 'Returns version, cwd, project_id, cache, config, and template paths',
				request: {},
				response: {
					status: 200,
					body: {
						version: '2026.3.6',
						cwd: '/path/to/projects/project1',
						project_id: 'project1',
						cache: '/path/to/cache',
						config: '/path/to/config',
						template: '/path/to/templates'
					}
				}
			}
		]
	}
];

export const intercept_api = [
	{
		id: 'intercept',
		name: 'Intercept Action',
		method: 'POST',
		path: '/api/intercept/action',
		description: 'Forward or drop an intercepted request (requires auth)',
		category: '/api/intercept',
		defaultBody: {
			id: '',
			action: 'forward',
			is_req_edited: false,
			is_resp_edited: false,
			req_edited: '',
			resp_edited: ''
		},
		examples: [
			{
				name: 'Forward intercepted request',
				description: 'Forwards or drops an intercepted request, optionally with edited req/resp',
				request: {
					id: 'abc123',
					action: 'forward',
					is_req_edited: false,
					is_resp_edited: false
				},
				response: {
					status: 200,
					body: { success: true, message: 'Intercept action processed successfully' }
				}
			}
		]
	}
];

export const labels_api = [
	{
		id: 'labels',
		name: 'Create Label',
		method: 'POST',
		path: '/api/label/new',
		description: 'Create a new label (requires auth)',
		category: '/api/label',
		defaultBody: {
			name: 'bug',
			color: '#ff0000',
			type: 'default'
		},
		examples: [
			{
				name: 'Create a label',
				description: 'Creates a new label or returns existing one if name already exists',
				request: { name: 'bug', color: '#ff0000', type: 'default' },
				response: {
					status: 200,
					body: { id: 'abc123', alreadyExists: false }
				}
			}
		]
	},
	{
		id: 'labels',
		name: 'Delete Label',
		method: 'POST',
		path: '/api/label/delete',
		description: 'Delete a label by ID or name (requires auth)',
		category: '/api/label',
		defaultBody: {
			id: '',
			name: 'bug'
		},
		examples: [
			{
				name: 'Delete a label',
				description: 'Deletes the label and its associated collection',
				request: { name: 'bug' },
				response: {
					status: 200,
					body: 'Deleted'
				}
			}
		]
	},
	{
		id: 'labels',
		name: 'Attach Label',
		method: 'POST',
		path: '/api/label/attach',
		description: 'Attach a label to a record (requires auth)',
		category: '/api/label',
		defaultBody: {
			id: '',
			name: 'bug',
			color: '#ff0000',
			type: 'default'
		},
		examples: [
			{
				name: 'Attach a label',
				description: 'Creates label if needed and attaches it to the specified record',
				request: { id: 'record123', name: 'bug', color: '#ff0000', type: 'default' },
				response: {
					status: 200,
					body: 'Created'
				}
			}
		]
	}
];

export const modify_api = [
	{
		id: 'modify',
		name: 'Modify Request',
		method: 'POST',
		path: '/api/request/modify',
		description: 'Apply transformation tasks (set, delete, replace) to a raw HTTP request (requires auth)',
		category: '/api/request',
		defaultBody: {
			request: 'GET / HTTP/1.1\r\nHost: example.com\r\n\r\n',
			url: 'https://example.com',
			tasks: [{ set: { 'req.headers.X-Custom': 'value' } }]
		},
		examples: [
			{
				name: 'Modify a request',
				description: 'Applies set/delete/replace actions to a raw HTTP request and returns the modified result',
				request: {
					request: 'GET / HTTP/1.1\r\nHost: example.com\r\n\r\n',
					url: 'https://example.com',
					tasks: [{ set: { 'req.headers.X-Custom': 'value' } }]
				},
				response: {
					status: 200,
					body: { success: 'true', request: 'GET / HTTP/1.1\r\nHost: example.com\r\nX-Custom: value\r\n\r\n' }
				}
			}
		]
	}
];

export const playground_api = [
	{
		id: 'playground',
		name: 'New Playground',
		method: 'POST',
		path: '/api/playground/new',
		description: 'Create a new playground item (requires auth)',
		category: '/api/playground',
		defaultBody: {
			name: 'New Playground',
			type: 'playground',
			parent_id: '',
			expanded: false
		},
		examples: [
			{
				name: 'Create playground',
				description: 'Creates a new playground record with auto-calculated sort order',
				request: { name: 'My Playground', type: 'playground', parent_id: '' },
				response: {
					status: 200,
					body: { id: 'abc123', name: 'My Playground', type: 'playground', sort_order: 1000 }
				}
			}
		]
	},
	{
		id: 'playground',
		name: 'Add Child',
		method: 'POST',
		path: '/api/playground/add',
		description: 'Add child items (repeater/fuzzer) to a playground (requires auth)',
		category: '/api/playground',
		defaultBody: {
			parent_id: '',
			items: [{ name: 'Repeater 1', type: 'repeater', tool_data: {} }]
		},
		examples: [
			{
				name: 'Add child items',
				description: 'Adds repeater/fuzzer items under a parent playground and creates associated collections',
				request: { parent_id: 'abc123', items: [{ name: 'Repeater 1', type: 'repeater', tool_data: { url: '', req: '', resp: '' } }] },
				response: {
					status: 200,
					body: { success: true, items: [] }
				}
			}
		]
	},
	{
		id: 'playground',
		name: 'Delete Playground',
		method: 'POST',
		path: '/api/playground/delete',
		description: 'Recursively delete a playground item and its children (requires auth)',
		category: '/api/playground',
		defaultBody: {
			id: ''
		},
		examples: [
			{
				name: 'Delete playground',
				description: 'Deletes the item, all children recursively, and associated repeater/intruder collections',
				request: { id: 'abc123' },
				response: {
					status: 200,
					body: { success: true, id: 'abc123' }
				}
			}
		]
	}
];

export const rawhttp_api = [
	{
		id: 'rawhttp',
		name: 'Send Raw HTTP',
		method: 'POST',
		path: '/api/http/raw',
		description: 'Send a raw HTTP request using the rawhttp client (requires auth)',
		category: '/api/http',
		defaultBody: {
			host: 'example.com',
			port: '443',
			tls: true,
			req: 'GET / HTTP/1.1\r\nHost: example.com\r\n\r\n',
			timeout: 10,
			http2: false
		},
		examples: [
			{
				name: 'Send raw request',
				description: 'Sends a raw HTTP request via the rawhttp client and returns response with timing',
				request: {
					host: 'example.com',
					port: '443',
					tls: true,
					req: 'GET / HTTP/1.1\r\nHost: example.com\r\n\r\n',
					timeout: 10,
					http2: false
				},
				response: {
					status: 200,
					body: { resp: 'HTTP/1.1 200 OK\r\n...', time: '120ms' }
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
				request: {
					regex: '<title>(.*?)</title>',
					responseBody: '<html><title>Test</title></html>'
				},
				response: {
					status: 200,
					body: { matched: true }
				}
			}
		]
	}
];

export const repeater_api = [
	{
		id: 'repeater',
		name: 'Send Repeater',
		method: 'POST',
		path: '/api/repeater/send',
		description: 'Send a raw HTTP request via repeater and save to backend (requires auth)',
		category: '/api/repeater',
		defaultBody: {
			host: 'example.com',
			port: '443',
			tls: true,
			request: 'GET / HTTP/1.1\r\nHost: example.com\r\n\r\n',
			timeout: 10,
			http2: false,
			index: 0,
			url: 'https://example.com',
			generated_by: '',
			note: ''
		},
		examples: [
			{
				name: 'Send repeater request',
				description: 'Sends a raw request, saves req/resp to backend, and returns response with timing and userdata',
				request: {
					host: 'example.com',
					port: '443',
					tls: true,
					request: 'GET / HTTP/1.1\r\nHost: example.com\r\n\r\n',
					timeout: 10,
					http2: false,
					url: 'https://example.com',
					generated_by: 'pg_abc123'
				},
				response: {
					status: 200,
					body: { response: 'HTTP/1.1 200 OK\r\n...', time: '120ms', userdata: {} }
				}
			}
		]
	}
];

export const request_api = [
	{
		id: 'request',
		name: 'Add Request',
		method: 'POST',
		path: '/api/request/add',
		description: 'Add a request/response pair to the backend database',
		category: '/api/request',
		defaultBody: {
			url: 'https://example.com',
			index: 0,
			request: 'GET / HTTP/1.1\r\nHost: example.com\r\n\r\n',
			response: '',
			generated_by: 'manual',
			note: ''
		},
		examples: [
			{
				name: 'Add a request',
				description: 'Saves a request (and optional response) to _data, _req, _resp, _attached collections',
				request: {
					url: 'https://example.com',
					request: 'GET / HTTP/1.1\r\nHost: example.com\r\n\r\n',
					generated_by: 'manual'
				},
				response: {
					status: 200,
					body: { id: '000000000000001', host: 'https://example.com', index: 1 }
				}
			}
		]
	}
];

export const sitemap_api = [
	{
		id: 'sitemap',
		name: 'Sitemap New',
		method: 'POST',
		path: '/api/sitemap/new',
		description: 'Add a new sitemap entry for a host (requires auth)',
		category: '/api/sitemap',
		defaultBody: {
			host: 'https://example.com',
			path: '/api/users',
			query: 'page=1',
			fragment: '',
			ext: '',
			type: 'folder',
			data: ''
		},
		examples: [
			{
				name: 'Add sitemap entry',
				description: 'Creates host collection if needed, adds endpoint entry, and runs wappalyzer fingerprinting',
				request: { host: 'https://example.com', path: '/api/users', type: 'folder', data: 'record123' },
				response: {
					status: 200,
					body: 'Created'
				}
			}
		]
	},
	{
		id: 'sitemap',
		name: 'Sitemap Fetch',
		method: 'POST',
		path: '/api/sitemap/fetch',
		description: 'Fetch sitemap tree for a host with depth control (requires auth)',
		category: '/api/sitemap',
		defaultBody: {
			host: 'https://example.com',
			path: '',
			depth: 1
		},
		examples: [
			{
				name: 'Fetch sitemap tree',
				description: 'Returns a hierarchical tree of paths for the given host, with configurable depth (-1 for unlimited)',
				request: { host: 'https://example.com', path: '', depth: 1 },
				response: {
					status: 200,
					body: [
						{ host: 'https://example.com', path: '/api', title: 'api', isFolder: true, childrenCount: 2, children: [] }
					]
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
			sql: 'SELECT * FROM _data LIMIT 10'
		},
		examples: [
			{
				name: 'Run SQL query',
				description: 'Executes the SQL query and returns rows as newline-delimited JSON',
				request: { sql: 'SELECT * FROM _data LIMIT 10' },
				response: {
					status: 200,
					body: '{"id":"abc123","host":"example.com"}\n...'
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

export const tools_api = [
	{
		id: 'tools',
		name: 'Tool Server',
		method: 'GET',
		path: '/api/tool/server',
		description: 'Start a new grroxy-tool server instance on an available port',
		category: '/api/tool',
		examples: [
			{
				name: 'Start tool server',
				description: 'Launches a grroxy-tool process and returns connection details',
				request: {},
				response: {
					status: 200,
					body: { path: '/path/to/projects', hostAddress: '127.0.0.1:8091', id: 'abc123', name: 'xyz', username: 'new@example.com', password: '1234567890' }
				}
			}
		]
	},
	{
		id: 'tools',
		name: 'Tool',
		method: 'GET',
		path: '/api/tool',
		description: 'Start a new PocketBase instance at the specified path',
		category: '/api/tool',
		examples: [
			{
				name: 'Start tool',
				description: 'Bootstraps and serves a new PocketBase instance on an available port',
				request: {},
				response: {
					status: 200,
					body: 'Path parameter: /path/to/data'
				}
			}
		]
	}
];

export const xterm_api = [
	{
		id: 'xterm',
		name: 'Start Terminal',
		method: 'POST',
		path: '/api/xterm/start',
		description: 'Start a new terminal session with optional shell and working directory',
		category: '/api/xterm',
		defaultBody: {
			shell: '',
			workdir: '',
			env: {}
		},
		examples: [
			{
				name: 'Start terminal session',
				description: 'Creates a new PTY session and returns session ID for WebSocket connection',
				request: { shell: 'bash', workdir: '/home/user' },
				response: {
					status: 200,
					body: { session_id: 'abc123', shell: 'bash', workdir: '/home/user' }
				}
			}
		]
	},
	{
		id: 'xterm',
		name: 'List Sessions',
		method: 'GET',
		path: '/api/xterm/sessions',
		description: 'List all active terminal sessions',
		category: '/api/xterm',
		examples: [
			{
				name: 'List terminal sessions',
				description: 'Returns all active terminal sessions with their details',
				request: {},
				response: {
					status: 200,
					body: { sessions: [{ id: 'abc123', shell: 'bash', workdir: '/home/user', running: true }] }
				}
			}
		]
	},
	{
		id: 'xterm',
		name: 'Close Session',
		method: 'DELETE',
		path: '/api/xterm/sessions/:id',
		description: 'Close a terminal session by ID',
		category: '/api/xterm',
		examples: [
			{
				name: 'Close terminal session',
				description: 'Closes the PTY and kills the process for the specified session',
				request: {},
				response: {
					status: 200,
					body: { message: 'Session closed successfully' }
				}
			}
		]
	},
	{
		id: 'xterm',
		name: 'Terminal WebSocket',
		method: 'GET',
		path: '/api/xterm/ws/:id',
		description: 'WebSocket endpoint for terminal I/O (input, resize, ping)',
		category: '/api/xterm',
		examples: [
			{
				name: 'Connect to terminal',
				description: 'Upgrades to WebSocket for bidirectional terminal communication',
				request: {},
				response: {
					status: 101,
					body: 'WebSocket upgrade'
				}
			}
		]
	}
];

export const proxy_api = [
	{
		id: 'proxy',
		name: 'Start Proxy',
		method: 'POST',
		path: '/api/proxy/start',
		description: 'Start a new proxy instance with optional browser (requires auth)',
		category: '/api/proxy',
		defaultBody: {
			http: '127.0.0.1:8080',
			browser: '',
			name: ''
		},
		examples: [
			{
				name: 'Start proxy',
				description: 'Starts a proxy on the given address, optionally launching a browser',
				request: { http: '127.0.0.1:8080' },
				response: {
					status: 200,
					body: { id: 'abc123', address: '127.0.0.1:8080', label: '127.0.0.1:8080' }
				}
			}
		]
	},
	{
		id: 'proxy',
		name: 'Stop Proxy',
		method: 'POST',
		path: '/api/proxy/stop',
		description: 'Stop a running proxy instance (requires auth)',
		category: '/api/proxy',
		defaultBody: { id: '' },
		examples: [
			{
				name: 'Stop proxy',
				description: 'Stops the proxy and kills associated browser if any',
				request: { id: 'abc123' },
				response: { status: 200, body: { success: true } }
			}
		]
	},
	{
		id: 'proxy',
		name: 'Restart Proxy',
		method: 'POST',
		path: '/api/proxy/restart',
		description: 'Restart a proxy instance (requires auth)',
		category: '/api/proxy',
		defaultBody: { id: '' },
		examples: [
			{
				name: 'Restart proxy',
				description: 'Stops and restarts the proxy with the same configuration',
				request: { id: 'abc123' },
				response: { status: 200, body: { success: true } }
			}
		]
	},
	{
		id: 'proxy',
		name: 'List Proxies',
		method: 'GET',
		path: '/api/proxy/list',
		description: 'List all running proxy instances (requires auth)',
		category: '/api/proxy',
		examples: [
			{
				name: 'List proxies',
				description: 'Returns all active proxy instances with their details',
				request: {},
				response: { status: 200, body: [] }
			}
		]
	},
	{
		id: 'proxy',
		name: 'Screenshot',
		method: 'POST',
		path: '/api/proxy/screenshot',
		description: 'Take a screenshot of the browser attached to a proxy (requires auth)',
		category: '/api/proxy',
		defaultBody: { id: '' },
		examples: [
			{
				name: 'Take screenshot',
				description: 'Captures a screenshot from the Chrome instance attached to the proxy',
				request: { id: 'abc123' },
				response: { status: 200, body: { screenshot: '<base64 data>' } }
			}
		]
	},
	{
		id: 'proxy',
		name: 'Click Element',
		method: 'POST',
		path: '/api/proxy/click',
		description: 'Click an element in the browser by selector (requires auth)',
		category: '/api/proxy',
		defaultBody: { id: '', selector: '' },
		examples: [
			{
				name: 'Click element',
				description: 'Clicks a DOM element in the Chrome instance by CSS selector',
				request: { id: 'abc123', selector: '#submit' },
				response: { status: 200, body: { success: true } }
			}
		]
	},
	{
		id: 'proxy',
		name: 'Get Elements',
		method: 'POST',
		path: '/api/proxy/elements',
		description: 'Get DOM elements from the browser by selector (requires auth)',
		category: '/api/proxy',
		defaultBody: { id: '', selector: '' },
		examples: [
			{
				name: 'Get elements',
				description: 'Returns DOM elements matching the CSS selector from the Chrome instance',
				request: { id: 'abc123', selector: 'a' },
				response: { status: 200, body: { elements: [] } }
			}
		]
	},
	{
		id: 'proxy',
		name: 'Chrome Tabs',
		method: 'POST',
		path: '/api/proxy/chrome/tabs',
		description: 'List Chrome tabs for a proxy (requires auth)',
		category: '/api/proxy',
		defaultBody: { id: '' },
		examples: [
			{
				name: 'List Chrome tabs',
				description: 'Returns all open tabs in the Chrome instance',
				request: { id: 'abc123' },
				response: { status: 200, body: { tabs: [] } }
			}
		]
	},
	{
		id: 'proxy',
		name: 'Open Tab',
		method: 'POST',
		path: '/api/proxy/chrome/tab/open',
		description: 'Open a new Chrome tab (requires auth)',
		category: '/api/proxy',
		defaultBody: { id: '', url: 'about:blank' },
		examples: [
			{
				name: 'Open new tab',
				description: 'Opens a new tab in the Chrome instance',
				request: { id: 'abc123', url: 'https://example.com' },
				response: { status: 200, body: { success: true } }
			}
		]
	},
	{
		id: 'proxy',
		name: 'Navigate Tab',
		method: 'POST',
		path: '/api/proxy/chrome/tab/navigate',
		description: 'Navigate a Chrome tab to a URL (requires auth)',
		category: '/api/proxy',
		defaultBody: { id: '', url: '' },
		examples: [
			{
				name: 'Navigate tab',
				description: 'Navigates the active tab to the specified URL',
				request: { id: 'abc123', url: 'https://example.com' },
				response: { status: 200, body: { success: true } }
			}
		]
	},
	{
		id: 'proxy',
		name: 'Activate Tab',
		method: 'POST',
		path: '/api/proxy/chrome/tab/activate',
		description: 'Activate a Chrome tab by ID (requires auth)',
		category: '/api/proxy',
		defaultBody: { id: '', tabId: '' },
		examples: [
			{
				name: 'Activate tab',
				description: 'Switches to the specified Chrome tab',
				request: { id: 'abc123', tabId: 'tab1' },
				response: { status: 200, body: { success: true } }
			}
		]
	},
	{
		id: 'proxy',
		name: 'Close Tab',
		method: 'POST',
		path: '/api/proxy/chrome/tab/close',
		description: 'Close a Chrome tab by ID (requires auth)',
		category: '/api/proxy',
		defaultBody: { id: '', tabId: '' },
		examples: [
			{
				name: 'Close tab',
				description: 'Closes the specified Chrome tab',
				request: { id: 'abc123', tabId: 'tab1' },
				response: { status: 200, body: { success: true } }
			}
		]
	},
	{
		id: 'proxy',
		name: 'Reload Tab',
		method: 'POST',
		path: '/api/proxy/chrome/tab/reload',
		description: 'Reload the active Chrome tab (requires auth)',
		category: '/api/proxy',
		defaultBody: { id: '' },
		examples: [
			{
				name: 'Reload tab',
				description: 'Reloads the active tab in the Chrome instance',
				request: { id: 'abc123' },
				response: { status: 200, body: { success: true } }
			}
		]
	},
	{
		id: 'proxy',
		name: 'Back',
		method: 'POST',
		path: '/api/proxy/chrome/tab/back',
		description: 'Navigate back in the active Chrome tab (requires auth)',
		category: '/api/proxy',
		defaultBody: { id: '' },
		examples: [
			{
				name: 'Navigate back',
				description: 'Goes back in the browser history of the active tab',
				request: { id: 'abc123' },
				response: { status: 200, body: { success: true } }
			}
		]
	},
	{
		id: 'proxy',
		name: 'Forward',
		method: 'POST',
		path: '/api/proxy/chrome/tab/forward',
		description: 'Navigate forward in the active Chrome tab (requires auth)',
		category: '/api/proxy',
		defaultBody: { id: '' },
		examples: [
			{
				name: 'Navigate forward',
				description: 'Goes forward in the browser history of the active tab',
				request: { id: 'abc123' },
				response: { status: 200, body: { success: true } }
			}
		]
	}
];

export const appEndpoints = [
	...cert_api,
	...commands_api,
	...cook_api,
	...extractor_api,
	...file_api,
	...filewatcher_api,
	...filters_api,
	...http_api,
	...info_api,
	...intercept_api,
	...labels_api,
	...modify_api,
	...playground_api,
	...proxy_api,
	...rawhttp_api,
	...regex_api,
	...repeater_api,
	...request_api,
	...sitemap_api,
	...sqltest_api,
	...templates_api,
	...tools_api,
	...xterm_api
];
