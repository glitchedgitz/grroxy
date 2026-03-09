export interface ApiEndpoint {
	id: string;
	name: string;
	method: 'GET' | 'POST' | 'DELETE';
	path: string;
	description: string;
	category: string;
	defaultBody?: Record<string, unknown>;
}

export interface ApiResponse {
	status: number;
	statusText: string;
	headers: Record<string, string>;
	body: unknown;
	time: number;
}

export const API_CATEGORIES = [
	'/api/info',
	'/api/proxy',
	'/api/proxy/chrome',
	'/api/playground',
	'/api/repeater',
	'/api/request',
	'/api/filter',
	'/api/templates',
	'/api/cook',
	'/api/label',
	'/api/regex',
	'/api/sitemap',
	'/api/commands',
	'/api/tools',
	'/api/raw',
	'/api/extract',
	'/cacert'
] as const;

export const ENDPOINTS: ApiEndpoint[] = [
	// Info
	{
		id: 'info',
		name: 'Get Info',
		method: 'GET',
		path: '/api/info',
		description: 'Returns version, paths, and instance info',
		category: '/api/info'
	},

	// Proxy Management
	{
		id: 'proxy-start',
		name: 'Start Proxy',
		method: 'POST',
		path: '/api/proxy/start',
		description: 'Start a new proxy instance',
		category: '/api/proxy',
		defaultBody: {
			http: '127.0.0.1:8080',
			browser: 'chrome',
			name: ''
		}
	},
	{
		id: 'proxy-stop',
		name: 'Stop Proxy',
		method: 'POST',
		path: '/api/proxy/stop',
		description: 'Stop a running proxy (empty id stops all)',
		category: '/api/proxy',
		defaultBody: { id: '' }
	},
	{
		id: 'proxy-restart',
		name: 'Restart Proxy',
		method: 'POST',
		path: '/api/proxy/restart',
		description: 'Restart a stopped proxy by ID',
		category: '/api/proxy',
		defaultBody: { id: '' }
	},
	{
		id: 'proxy-list',
		name: 'List Proxies',
		method: 'GET',
		path: '/api/proxy/list',
		description: 'List all running proxy instances',
		category: '/api/proxy'
	},
	{
		id: 'proxy-screenshot',
		name: 'Take Screenshot',
		method: 'POST',
		path: '/api/proxy/screenshot',
		description: 'Capture screenshot from Chrome proxy',
		category: '/api/proxy',
		defaultBody: { id: '', fullPage: false, saveFile: false }
	},
	{
		id: 'proxy-click',
		name: 'Click Element',
		method: 'POST',
		path: '/api/proxy/click',
		description: 'Click an element in Chrome via CSS selector',
		category: '/api/proxy',
		defaultBody: { id: '', url: '', selector: '', waitForNavigation: false }
	},
	{
		id: 'proxy-elements',
		name: 'Get Clickable Elements',
		method: 'POST',
		path: '/api/proxy/elements',
		description: 'Extract clickable elements from page',
		category: '/api/proxy',
		defaultBody: { id: '', url: '' }
	},

	// Chrome Tab Management
	{
		id: 'chrome-tabs',
		name: 'List Tabs',
		method: 'POST',
		path: '/api/proxy/chrome/tabs',
		description: 'List all open Chrome tabs',
		category: '/api/proxy/chrome',
		defaultBody: { proxyId: '' }
	},
	{
		id: 'chrome-tab-open',
		name: 'Open Tab',
		method: 'POST',
		path: '/api/proxy/chrome/tab/open',
		description: 'Open a new Chrome tab',
		category: '/api/proxy/chrome',
		defaultBody: { proxyId: '', url: 'about:blank' }
	},
	{
		id: 'chrome-tab-navigate',
		name: 'Navigate Tab',
		method: 'POST',
		path: '/api/proxy/chrome/tab/navigate',
		description: 'Navigate a tab to a URL',
		category: '/api/proxy/chrome',
		defaultBody: { proxyId: '', targetId: '', url: '', waitUntil: 'load', timeoutMs: 30000 }
	},
	{
		id: 'chrome-tab-activate',
		name: 'Activate Tab',
		method: 'POST',
		path: '/api/proxy/chrome/tab/activate',
		description: 'Switch focus to a specific tab',
		category: '/api/proxy/chrome',
		defaultBody: { proxyId: '', targetId: '' }
	},
	{
		id: 'chrome-tab-close',
		name: 'Close Tab',
		method: 'POST',
		path: '/api/proxy/chrome/tab/close',
		description: 'Close a specific tab',
		category: '/api/proxy/chrome',
		defaultBody: { proxyId: '', targetId: '' }
	},
	{
		id: 'chrome-tab-reload',
		name: 'Reload Tab',
		method: 'POST',
		path: '/api/proxy/chrome/tab/reload',
		description: 'Reload a tab, optionally bypassing cache',
		category: '/api/proxy/chrome',
		defaultBody: { proxyId: '', targetId: '', bypassCache: false }
	},
	{
		id: 'chrome-tab-back',
		name: 'Go Back',
		method: 'POST',
		path: '/api/proxy/chrome/tab/back',
		description: 'Navigate back in history',
		category: '/api/proxy/chrome',
		defaultBody: { proxyId: '', targetId: '' }
	},
	{
		id: 'chrome-tab-forward',
		name: 'Go Forward',
		method: 'POST',
		path: '/api/proxy/chrome/tab/forward',
		description: 'Navigate forward in history',
		category: '/api/proxy/chrome',
		defaultBody: { proxyId: '', targetId: '' }
	},

	// Intercept
	{
		id: 'intercept-action',
		name: 'Intercept Action',
		method: 'POST',
		path: '/api/intercept/action',
		description: 'Forward or drop an intercepted request',
		category: '/api/proxy',
		defaultBody: {
			id: '',
			action: 'forward',
			is_req_edited: false,
			is_resp_edited: false,
			req_edited: '',
			resp_edited: ''
		}
	},

	// Playground
	{
		id: 'playground-new',
		name: 'New Playground',
		method: 'POST',
		path: '/api/playground/new',
		description: 'Create a new playground item',
		category: '/api/playground',
		defaultBody: { name: 'New Playground', parent_id: '', type: 'playground', expanded: false }
	},
	{
		id: 'playground-add',
		name: 'Add Items',
		method: 'POST',
		path: '/api/playground/add',
		description: 'Add tool items to a playground folder',
		category: '/api/playground',
		defaultBody: {
			parent_id: '',
			items: [{ name: 'Test Request', type: 'repeater', tool_data: { url: '', req: '', resp: '' } }]
		}
	},
	{
		id: 'playground-delete',
		name: 'Delete Playground',
		method: 'POST',
		path: '/api/playground/delete',
		description: 'Delete playground item and children',
		category: '/api/playground',
		defaultBody: { id: '' }
	},

	// Repeater
	{
		id: 'repeater-send',
		name: 'Send Request',
		method: 'POST',
		path: '/api/repeater/send',
		description: 'Send a raw HTTP request via repeater',
		category: '/api/repeater',
		defaultBody: {
			host: 'example.com',
			port: '443',
			tls: true,
			request: 'GET / HTTP/1.1\r\nHost: example.com\r\n\r\n',
			timeout: 10,
			http2: false,
			index: 1.0,
			url: 'https://example.com'
		}
	},

	// Modify
	{
		id: 'request-modify',
		name: 'Modify Request',
		method: 'POST',
		path: '/api/request/modify',
		description: 'Apply transformations to an HTTP request',
		category: '/api/request',
		defaultBody: {
			request: 'GET /api/test HTTP/1.1\r\nHost: example.com\r\n\r\n',
			url: 'https://example.com/api/test',
			tasks: [{ action: 'set', key: 'req.method', value: 'POST' }]
		}
	},

	// Filters
	{
		id: 'filter-check',
		name: 'Check Filter',
		method: 'POST',
		path: '/api/filter/check',
		description: 'Evaluate a dadql filter expression',
		category: '/api/filter',
		defaultBody: {
			filter: "status == 200 && method == 'GET'",
			columns: { status: 200, method: 'GET', path: '/api/test' }
		}
	},

	// Templates
	{
		id: 'templates-list',
		name: 'List Templates',
		method: 'GET',
		path: '/api/templates/list',
		description: 'List all YAML template files',
		category: '/api/templates'
	},
	{
		id: 'templates-new',
		name: 'Create Template',
		method: 'POST',
		path: '/api/templates/new',
		description: 'Create a new template file',
		category: '/api/templates',
		defaultBody: { name: 'test-template.yaml', content: 'template: content\nhere: value' }
	},
	{
		id: 'templates-delete',
		name: 'Delete Template',
		method: 'DELETE',
		path: '/api/templates/:template',
		description: 'Delete a template file (replace :template in path)',
		category: '/api/templates'
	},

	// Files
	{
		id: 'readfile',
		name: 'Read File',
		method: 'POST',
		path: '/api/readfile',
		description: 'Read a file from cache/config/cwd',
		category: '/api/readfile',
		defaultBody: { fileName: '', folder: 'cache' }
	},
	{
		id: 'savefile',
		name: 'Save File',
		method: 'POST',
		path: '/api/savefile',
		description: 'Save content to a file',
		category: '/api/savefile',
		defaultBody: { fileName: 'test.txt', fileData: 'hello world', folder: 'cache' }
	},

	// Cook
	{
		id: 'cook-generate',
		name: 'Generate Patterns',
		method: 'POST',
		path: '/api/cook/generate',
		description: 'Generate strings from Cook pattern syntax',
		category: '/api/cook',
		defaultBody: { pattern: ['admin{1-3}', 'user@{example,test}.com'] }
	},
	{
		id: 'cook-apply',
		name: 'Apply Methods',
		method: 'POST',
		path: '/api/cook/apply',
		description: 'Apply transformation methods to strings',
		category: '/api/cook',
		defaultBody: { strings: ['example', 'TEST'], methods: ['upper', 'lower'] }
	},
	{
		id: 'cook-search',
		name: 'Search Patterns',
		method: 'POST',
		path: '/api/cook/search',
		description: 'Search available Cook patterns/methods',
		category: '/api/cook',
		defaultBody: { search: 'encode' }
	},

	// Labels
	{
		id: 'label-new',
		name: 'Create Label',
		method: 'POST',
		path: '/api/label/new',
		description: 'Create a new label',
		category: '/api/label',
		defaultBody: { name: 'Important', color: '#FF0000', type: 'request' }
	},
	{
		id: 'label-delete',
		name: 'Delete Label',
		method: 'POST',
		path: '/api/label/delete',
		description: 'Delete a label by ID or name',
		category: '/api/label',
		defaultBody: { id: '' }
	},
	{
		id: 'label-attach',
		name: 'Attach Label',
		method: 'POST',
		path: '/api/label/attach',
		description: 'Attach a label to a record',
		category: '/api/label',
		defaultBody: { id: '', name: 'Important', color: '#FF0000' }
	},

	// Regex
	{
		id: 'regex',
		name: 'Test Regex',
		method: 'POST',
		path: '/api/regex',
		description: 'Test regex pattern against text',
		category: '/api/regex',
		defaultBody: { regex: '\\bpassword\\b', responseBody: 'This contains password field' }
	},

	// Sitemap
	{
		id: 'sitemap-new',
		name: 'New Entry',
		method: 'POST',
		path: '/api/sitemap/new',
		description: 'Create a new sitemap entry',
		category: '/api/sitemap/new',
		defaultBody: {
			host: 'https://example.com',
			data: 'endpoint_id',
			path: '/api/users',
			query: 'page=1',
			fragment: '',
			type: 'endpoint',
			ext: 'json'
		}
	},
	{
		id: 'sitemap-fetch',
		name: 'Fetch Sitemap',
		method: 'POST',
		path: '/api/sitemap/fetch',
		description: 'Fetch sitemap data as tree',
		category: '/api/sitemap/fetch',
		defaultBody: { host: 'https://example.com', path: '', depth: 1 }
	},

	// Commands
	{
		id: 'runcommand',
		name: 'Run Command',
		method: 'POST',
		path: '/api/runcommand',
		description: 'Execute a shell command',
		category: '/api/commands',
		defaultBody: { command: 'echo hello', data: '', saveTo: 'collection', collection: 'cmd_output', filename: '' }
	},

	// Tools
	{
		id: 'tool-server',
		name: 'Tool Server',
		method: 'GET',
		path: '/api/tool/server',
		description: 'Start a tool server instance',
		category: '/api/tool/server'
	},
	{
		id: 'tool-instance',
		name: 'Tool Instance',
		method: 'GET',
		path: '/api/tool',
		description: 'Start PocketBase at path (add ?path=...)',
		category: '/api/tool'
	},

	// Raw HTTP
	{
		id: 'sendrawrequest',
		name: 'Send Raw Request',
		method: 'POST',
		path: '/api/sendrawrequest',
		description: 'Send raw HTTP/1.1 or HTTP/2 request',
		category: '/api/raw',
		defaultBody: {
			host: 'example.com',
			port: '443',
			req: 'GET / HTTP/1.1\r\nHost: example.com\r\n\r\n',
			tls: true,
			timeout: 10,
			httpversion: 1
		}
	},

	// Extractor
	{
		id: 'extract',
		name: 'Extract Data',
		method: 'POST',
		path: '/api/extract',
		description: 'Extract records for a host to JSONL file',
		category: '/api/extract',
		defaultBody: {
			host: 'http://example.com',
			fields: ['host', 'req.method', 'req.url', 'req.path', 'req.params'],
			outputFile: ''
		}
	},

	// Certificates
	{
		id: 'cacert',
		name: 'Download CA Cert',
		method: 'GET',
		path: '/cacert.crt',
		description: 'Download CA certificate for HTTPS interception',
		category: '/cacert'
	}
];

export async function login(
	baseUrl: string,
	identity: string,
	password: string,
): Promise<{ token: string }> {
	const res = await fetch(`${baseUrl}/api/admins/auth-with-password`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ identity, password }),
	});
	if (!res.ok) {
		const text = await res.text();
		throw new Error(`Login failed (${res.status}): ${text}`);
	}
	return res.json();
}

export async function sendRequest(
	baseUrl: string,
	endpoint: ApiEndpoint,
	path: string,
	body?: string,
	authToken?: string,
): Promise<ApiResponse> {
	const url = `${baseUrl}${path}`;
	const headers: Record<string, string> = {
		'Content-Type': 'application/json'
	};

	if (authToken) {
		headers['Authorization'] = authToken;
	}

	const options: RequestInit = {
		method: endpoint.method,
		headers
	};

	if (endpoint.method !== 'GET' && body) {
		options.body = body;
	}

	const start = performance.now();
	const response = await fetch(url, options);
	const time = Math.round(performance.now() - start);

	const responseHeaders: Record<string, string> = {};
	response.headers.forEach((value, key) => {
		responseHeaders[key] = value;
	});

	let responseBody: unknown;
	const contentType = response.headers.get('content-type') || '';
	if (contentType.includes('json')) {
		responseBody = await response.json();
	} else {
		responseBody = await response.text();
	}

	return {
		status: response.status,
		statusText: response.statusText,
		headers: responseHeaders,
		body: responseBody,
		time
	};
}
