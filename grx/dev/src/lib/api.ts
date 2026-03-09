import { appEndpoints } from './app-api';
import { launcherEndpoints } from './launcher-api';
import { toolEndpoints } from './tool-api';

export interface ApiEndpoint {
	id: string;
	name: string;
	method: 'GET' | 'POST' | 'DELETE';
	path: string;
	description: string;
	category: string;
	defaultBody?: Record<string, unknown>;
	examples?: Array<{
		name: string;
		description: string;
		request: Record<string, unknown>;
		response: { status: number; body: unknown };
	}>;
}

export interface ApiResponse {
	status: number;
	statusText: string;
	headers: Record<string, string>;
	body: unknown;
	time: number;
}

export type AppType = 'app' | 'launcher' | 'tool';

export const APP_ENDPOINTS: Record<AppType, ApiEndpoint[]> = {
	app: appEndpoints as ApiEndpoint[],
	launcher: launcherEndpoints as ApiEndpoint[],
	tool: toolEndpoints as ApiEndpoint[],
};

export function getEndpoints(appType: AppType): ApiEndpoint[] {
	return APP_ENDPOINTS[appType];
}

export function getCategories(endpoints: ApiEndpoint[]): string[] {
	return [...new Set(endpoints.map((e) => e.category))];
}

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

export interface Project {
	id: string;
	name: string;
	path: string;
	data: { ip: string; state: string };
}

export async function fetchProjects(
	baseUrl: string,
	authToken: string,
): Promise<Project[]> {
	const res = await fetch(`${baseUrl}/api/project/list`, {
		headers: { Authorization: authToken },
	});
	if (!res.ok) {
		const text = await res.text();
		throw new Error(`Failed to list projects (${res.status}): ${text}`);
	}
	const json = await res.json();
	// Handle both array and wrapped responses
	const list = Array.isArray(json) ? json : (json.items ?? json.list ?? []);
	return list;
}

export async function openProject(
	baseUrl: string,
	project: string,
	authToken: string,
): Promise<Project> {
	const res = await fetch(`${baseUrl}/api/project/open`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', Authorization: authToken },
		body: JSON.stringify({ project }),
	});
	if (!res.ok) {
		const text = await res.text();
		throw new Error(`Failed to open project (${res.status}): ${text}`);
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
