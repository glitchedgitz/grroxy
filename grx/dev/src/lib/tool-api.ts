export const commands_api = [
	{
		id: 'commands',
		name: 'Run Command',
		method: 'POST',
		path: '/api/runcommand',
		description: 'Execute a shell command and save output to collection or file (requires auth)',
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
				name: 'Run command',
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

export const fuzzer_api = [
	{
		id: 'fuzzer',
		name: 'Start Fuzzer',
		method: 'POST',
		path: '/api/fuzzer/start',
		description: 'Start a fuzzing session with markers and payloads (requires auth)',
		category: '/api/fuzzer',
		defaultBody: {
			collection: 'fuzzer_results',
			request: 'GET /§path§ HTTP/1.1\r\nHost: example.com\r\n\r\n',
			host: 'example.com',
			port: '443',
			useTLS: true,
			http2: false,
			markers: { '§path§': ['admin', 'login', 'dashboard'] },
			mode: 'sniper',
			concurrency: 10,
			timeout: 10
		},
		examples: [
			{
				name: 'Start fuzzer',
				description: 'Starts fuzzing with the given request template, markers, and payloads',
				request: {
					collection: 'fuzzer_results',
					request: 'GET /§path§ HTTP/1.1\r\nHost: example.com\r\n\r\n',
					host: 'example.com',
					port: '443',
					useTLS: true,
					http2: false,
					markers: { '§path§': ['admin', 'login'] },
					mode: 'sniper',
					concurrency: 10,
					timeout: 10
				},
				response: {
					status: 200,
					body: { status: 'started', process_id: 'abc123', fuzzer_id: 'abc123' }
				}
			}
		]
	},
	{
		id: 'fuzzer',
		name: 'Stop Fuzzer',
		method: 'POST',
		path: '/api/fuzzer/stop',
		description: 'Stop a running fuzzer instance by ID (requires auth)',
		category: '/api/fuzzer',
		defaultBody: {
			id: ''
		},
		examples: [
			{
				name: 'Stop fuzzer',
				description: 'Stops a running fuzzer and updates its process state to Killed',
				request: { id: 'abc123' },
				response: {
					status: 200,
					body: { status: 'stopped', process_id: 'abc123', fuzzer_id: 'abc123' }
				}
			}
		]
	}
];

export const sdk_api = [
	{
		id: 'sdk',
		name: 'SDK Status',
		method: 'GET',
		path: '/api/sdk/status',
		description: 'Check if the tool is connected to the main app via SDK (requires auth)',
		category: '/api/sdk',
		examples: [
			{
				name: 'Check SDK status',
				description: 'Returns whether the SDK client is connected and the main app URL',
				request: {},
				response: {
					status: 200,
					body: { status: 'success', connected: true, url: 'http://localhost:8090' }
				}
			}
		]
	},
	{
		id: 'sdk',
		name: 'SDK Login',
		method: 'POST',
		path: '/api/sdk/login',
		description: 'Authenticate the SDK client with the main app (requires auth)',
		category: '/api/sdk',
		defaultBody: {
			url: 'http://localhost:8090',
			email: 'admin@example.com',
			password: ''
		},
		examples: [
			{
				name: 'Login to main app',
				description: 'Authenticates with the main app and establishes SDK connection',
				request: { url: 'http://localhost:8090', email: 'admin@example.com', password: 'password123' },
				response: {
					status: 200,
					body: { status: 'success', connected: true, url: 'http://localhost:8090' }
				}
			}
		]
	}
];

export const toolEndpoints = [
	...commands_api,
	...fuzzer_api,
	...sdk_api
];
