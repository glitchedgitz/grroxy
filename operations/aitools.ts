import { z } from "zod";

export const toolsData: Record<string,
    {
        title: string,
        icon?: string,
        description: string,
        parameters: z.ZodObject<any, any, any, any>,
        inputExample: any,
        outputExample?: any,
    }> = {

    'grroxyStatus': {
        title: 'Grroxy Status',
        icon: 'ri:settings-2-line',
        description: 'Check if the grroxy is active',
        parameters: z.object({
        }),
        inputExample: {},
        outputExample: {},
    },

    // Websearch
    'webSearch': {
        title: 'Web Search',
        icon: 'ri:search-line',
        description: 'Search the web for up-to-date information',
        parameters: z.object({
            query: z.string().min(1).max(100).describe('The search query'),
        }),
        inputExample: {
            "query": "What is the status of the grroxy?"
        },
        outputExample: {},
    },

    // File
    'readFile': {
        title: 'Read File',
        icon: 'ri:file-line',
        description: 'Read a file',
        parameters: z.object({
            path: z.string().describe('The path of the file'),
        }),
        inputExample: {
            "path": "/etc/passwd"
        },
        outputExample: {},
    },
    'writeFile': {
        title: 'Write File',
        icon: 'ri:file-line',
        description: 'Write a file',
        parameters: z.object({
            filename: z.string().describe('The name of the file'),
            content: z.string().describe('The content of the file'),
        }),
        inputExample: {
            "filename": "test.txt",
            "content": "Hello, world!"
        },
        outputExample: {},
    },

    // Play with filters
    'filterSaveSet': {
        title: 'Save Filters',
        description: 'Save a new filter set',
        icon: 'ri:filter-line',
        parameters: z.object({
            name: z.string().describe('the name of the filter set'),
            filter: z.string().describe('grroxy query filter'),
        }),
        inputExample: {
            "name": "test",
            "filter": "GET /test"
        },
        outputExample: {},
    },
    'filterGetSets': {
        title: 'Get Filters',
        description: 'Get all filter sets',
        icon: 'ri:filter-line',
        parameters: z.object({
        }),
        inputExample: {},
        outputExample: {},
    },

    'hudGetFilters': {
        title: 'Get HUD Filters',
        description: 'Get the current active filters applied on the connected HUD',
        icon: 'ri:filter-line',
        parameters: z.object({
        }),
        inputExample: {},
        outputExample: {
            filters: [
                { id: 'abc', checked: true, type: 'single', filter: 'resp.status = 200' },
                { id: 'def', checked: true, type: 'global', name: 'My Filter Set' },
            ],
            filterstring: 'resp.status = 200 AND (my filter template)',
        },
    },
    'hudSetFilters': {
        title: 'Set HUD Filters',
        description: `Set filters on the connected HUD. Always call hudGetFilters first to see current filters. Existing filters will be unchecked (not deleted) and new filters appended. Use filterGetSets to discover available saved filter sets. Each filter can be a single filter with a query string, or a global filter referencing a saved filter set by name.

Filter fields: id, index, host, port, has_resp, is_req_edited, is_resp_edited, is_https, has_params,
  Request: req.url, req.path, req.query, req.headers.HEADER_NAME, req.fragment, req.raw, req.ext, req.length, req.has_cookies,
  Response: resp.title, resp.mime, resp.status, resp.length, resp.raw, resp.has_cookies, resp.date, resp.time,
  Other: generated_by
Operators: AND, OR, NOT. Comparison: ~ (contains), !~ (not contains), =, !=, >, <, >=, <=
Example: "req.path ~ '/api/' AND resp.status = 200"`,
        icon: 'ri:filter-line',
        parameters: z.object({
            filters: z.array(z.object({
                type: z.enum(['single', 'global']).describe('single for inline filter query, global for referencing a saved filter set by name'),
                filter: z.string().optional().describe('the filter query string (for single type)'),
                name: z.string().optional().describe('the name of the saved filter set (for global type)'),
                checked: z.boolean().optional().describe('whether the filter is active, default true'),
            })).describe('the filters to apply on the HUD'),
        }),
        inputExample: {
            filters: [
                { type: 'single', filter: "req.path ~ '/api/' AND resp.mime ~ 'json'" },
                { type: 'global', name: 'My Saved Filter' },
            ],
        },
        outputExample: {},
    },

    // Data
    'getRequestResponseFromID': {
        title: 'Reading Request',
        description: 'Get the active request and response for active ID',
        parameters: z.object({
            activeID: z.string(),
        }),
        inputExample: {
            "activeID": "476"
        },
        outputExample: {},
    },

    'runResponseAnalyzer': {
        title: 'Run Response Analyzer',
        description: 'Run the response analyzer for the active ID',
        icon: 'ri:search-line',
        parameters: z.object({
            activeID: z.string(),
            objective: z.string().optional().describe('the objective to run the response analyzer for'),
        }),
        inputExample: {
            "activeID": "476",
            "todo": "Find secret in the response"
        },
        outputExample: {},
    },

    'askHumanToDoSomething': {
        title: 'AI is asking you ',
        description: 'Ask the human to do something',
        icon: 'ri:echo-line',
        parameters: z.object({
            message: z.string().describe('say what you want to ask the human to do'),
        }),
        inputExample: {
            "message": "Can you click signup for the request? I got stuck"
        },
        outputExample: {},
    },

    // === Target handling ===
    'hostPrintSitemap': {
        title: 'Reading Sitemap',
        icon: 'ri:list-check',
        description: 'Get the sitemap for a host',
        parameters: z.object({
            host: z.string().describe('the host to get the sitemap for'),
            path: z.string().describe('the path to get the sitemap for, use empty string "" to get the root sitemap'),
            depth: z.number().describe('the depth to get the sitemap for, default is -1, use -1 to get the full sitemap'),
        }),
        inputExample: {
            "host": "example.com",
            "path": "",
            "depth": -1
        },
        outputExample: {},
    },
    'hostPrintRowsInDetails': {
        title: 'Reading Table',
        icon: 'ri:book-open-line',
        description: 'Get the table for a host',
        parameters: z.object({
            host: z.string().describe('the host to get the table for'),
            page: z.number().describe('the page to get the data from, start from 1'),
            filter: z.string().describe('filter the results for faster search'),
        }),
        inputExample: {
            "host": "example.com",
            "page": 1,
            "filter": ""
        },
        outputExample: {},
    },
    'getNoteForHost': {
        title: 'Reading Note',
        icon: 'ri:mark-pen-line',
        description: 'Get the note for a host',
        parameters: z.object({
            host: z.string().describe('the host to get the note for'),
        }),
        inputExample: {
            "host": "example.com"
        },
        outputExample: {},
    },
    'setNoteForHost': {
        title: 'Writing Note',
        description: 'Set the note for a host',
        icon: 'ri:mark-pen-line',
        parameters: z.object({
            host: z.string().describe('the host to set the note for'),
            edit: z.array(z.object({
                index: z.number().describe('the index of the line to edit'),
                line: z.string().optional().describe('the content to edit the line with, to delete, write [delete]'),
            })).optional().describe('lines to be updated'),
        }),
        inputExample: {
            "host": "example.com",
            "edit": [
                { "index": 0, "line": "Hello, world!" }
            ]
        },
        outputExample: {},
    },
    'listHosts': {
        title: 'List Hosts',
        description: 'List all hosts with their technologies (as names) and labels (as names)',
        icon: 'ri:list-check',
        parameters: z.object({
            search: z.string().optional().describe('the search to get the table for, use empty string to get all results'),
            page: z.number().describe('the page to get the data from, start from 1'),
        }),
        inputExample: {
            "search": "example",
            "page": 1
        },
        outputExample: {
            "page": 1,
            "perPage": 5,
            "totalItems": 10,
            "totalPages": 2,
            "items": [
                {
                    "id": "abc123",
                    "host": "http://example.com",
                    "title": "Example Site",
                    "tech": ["Apache", "PHP"],
                    "labels": ["important"],
                    "notes": [
                        { "text": "This is a note", "author": "you" }
                    ]
                }
            ]
        },
    },

    'getHostInfo': {
        title: 'Get Host Info',
        description: 'Get detailed info for a specific host by ID, including technologies (as names), labels (as names), and notes',
        icon: 'ri:info-line',
        parameters: z.object({
            host: z.string().describe('the host ID to get the info for'),
        }),
        inputExample: {
            "host": "http://example.com"
        },
        outputExample: {
            "id": "abc123xyz",
            "host": "http://example.com",
            "title": "Example Site",
            "tech": ["Apache", "PHP", "MySQL"],
            "labels": ["important", "production"],
            "notes": [
                { "text": "This is a note", "author": "you" }
            ]
        },
    },

    'modifyHostLabels': {
        title: 'Update Host Label',
        description: 'Add or remove labels from a host',
        icon: 'ri:tag-line',
        parameters: z.object({
            host: z.string().describe('the host to update the label, include the protocol eg: http://example.com'),
            labels: z.array(z.object({
                action: z.enum(['add', 'remove', 'toggle']).describe('the action to perform on the label'),
                name: z.string().describe('the name of the label to update for the host'),
                color: z.string().optional().describe('the color of the label (only for add/toggle)'),
                type: z.string().optional().describe('the type of the label (only for add/toggle)'),
            })).describe('the labels to update for the host'),
        }),
        inputExample: {
            "host": "http://example.com",
            "labels": [
                { "action": "add", "name": "test", "color": "#ff0000" }
            ]
        },
        outputExample: {},
    },

    'modifyHostNotes': {
        title: 'Update Host Notes',
        description: 'Add, update, or remove notes for a host',
        icon: 'ri:mark-pen-line',
        parameters: z.object({
            host: z.string().describe('the host to update the note, include the protocol eg: http://example.com'),
            notes: z.array(z.object({
                action: z.enum(['add', 'update', 'remove']).describe('the action to perform on the note'),
                index: z.number().optional().describe('the index of the note to update/remove (not needed for add)'),
                text: z.string().optional().describe('the text of the note (for add/update actions)'),
                author: z.string().optional().describe('the author of the note (for add action, defaults to "you")'),
            })).describe('the notes to update for the host'),
        }),
        inputExample: {
            "host": "http://example.com",
            "notes": [
                { "action": "add", "text": "This is a new note", "author": "you" },
                { "action": "update", "index": 0, "text": "Updated note text" },
                { "action": "remove", "index": 2 }
            ]
        },
        outputExample: {},
    },

    // Quick Search
    'getQuickSearchSets': {
        title: 'Get Quick Searches',
        icon: 'ri:search-line',
        description: 'Get all saved quick search sets (regex patterns used for highlighting/searching in responses)',
        parameters: z.object({
        }),
        inputExample: {},
        outputExample: {
            "searches": [
                { "id": "abc123", "name": "Link Finder", "search": { "regexp": true, "search": "(https?://[^\"'\\s]+)" } },
                { "id": "def456", "name": "Juicy Words", "search": { "regexp": true, "search": "admin|secret|key" } }
            ]
        },
    },
    'addQuickSearchSet': {
        title: 'Add Quick Search',
        icon: 'ri:search-line',
        description: 'Save a new quick search set (regex pattern for highlighting/searching in responses)',
        parameters: z.object({
            name: z.string().describe('the name of the quick search set'),
            search: z.string().describe('the search pattern (regex or plain text)'),
            regexp: z.boolean().optional().describe('whether the search is a regex, default true'),
        }),
        inputExample: {
            "name": "API Keys",
            "search": "(api[_-]?key|api[_-]?secret)[\\s]*[=:][\\s]*[\"']?[A-Za-z0-9_\\-]+",
            "regexp": true
        },
        outputExample: {},
    },

    'deleteQuickSearchSet': {
        title: 'Delete Quick Search',
        icon: 'ri:delete-bin-line',
        description: 'Delete a saved quick search set by its ID',
        parameters: z.object({
            id: z.string().describe('the ID of the quick search set to delete'),
        }),
        inputExample: {
            "id": "abc123"
        },
        outputExample: {},
    },

    // wordlist
    'searchWordlist': {
        title: 'Search Wordlist',
        icon: 'ri:search-line',
        description: 'Search the wordlist for your pentest',
        parameters: z.object({
            search: z.string(),
        }),
        inputExample: {
            "search": "example"
        },
        outputExample: {},
    },
    // 'previewWordlist': {
    //     title: 'Preview Wordlist',
    //     description: 'Preview the wordlist for your pentest',
    //     parameters: z.object({
    //         wordlist: z.string(),
    //     }),
    // },

    // Play with requests
    'sendRequest': {
        title: 'Send Request',
        icon: 'ri:earth-line',
        description: 'Send a request via http. Mind the terminating the request with \r\n\r\n or \n\n if there are no body, mind the content length of the body, it should exactly match',
        parameters: z.object({
            tls: z.boolean().describe('use https or http'),
            host: z.string().describe('the host to send the request to'),
            port: z.number().describe('the port to send the request to'),
            httpVersion: z.number().describe('1 or 2'),
            attachToIndex: z.number().describe('origin index of request you are modifying'),
            request: z.string().describe('raw request'),
            note: z.string().describe('the note to attach to the request'),
            labels: z.array(z.string()).optional().describe('the labels to attach to the request'),
            autoUpdateContentLength: z.boolean().describe('auto update content length, default: true'),
        }),
        inputExample: {
            "tls": true,
            "host": "example.com",
            "port": 443,
            "httpVersion": 1,
            "index": 0,
            "request": "GET / HTTP/1.1\r\nHost: example.com\r\n\r\n Cookie: test=123 \r\n\r\n",
            "note": "test",
            "labels": ["test", "test2"],
            "autoUpdateContentLength": true
        },
        outputExample: {},
    },

    'getApiTestingTips': {
        title: 'API Testing Tips',
        icon: 'ri:book-open-line',
        description: 'Get API Testing Tips',
        parameters: z.object({
        }),
        inputExample: {
        },
        outputExample: {

        },
    },

    // === Report handling ===

    // // Create New Report
    // 'createPOCReport': {
    //     title: 'Create Report',
    //     description: 'Create a report',
    //     icon: 'ri:file-paper-2-line',
    //     parameters: z.object({
    //         title: z.string().describe('the title of the report'),
    //     }),
    //     inputExample: {
    //         "title": "test"
    //     },
    //     outputExample: {},
    // },
    // // Create New Report
    // 'readPOCReport': {
    //     title: 'Read Report',
    //     description: 'Create a report',
    //     icon: 'ri:file-paper-2-line',

    //     parameters: z.object({
    //         title: z.string().describe('the title of the report'),
    //     }),
    //     inputExample: {
    //         "title": "test"
    //     },
    //     outputExample: {},
    // },
    // // Create New Report
    // 'editPOCReport': {
    //     title: 'Edit Report',
    //     icon: 'ri:file-paper-2-line',
    //     description: 'Create a report',
    //     parameters: z.object({
    //         reportID: z.string().describe('the id of the report to edit'),
    //         title: z.string().optional().describe('update the title of the report'),
    //         edit: z.array(z.object({
    //             index: z.number().describe('the index of the line to edit'),
    //             line: z.string().optional().describe('the content to edit the line with, to delete, write [delete]'),
    //         })).optional().describe('lines to be updated'),
    //     }),
    //     inputExample: {
    //         "reportID": "123",
    //         "title": "test",
    //         "edit": [
    //             { "index": 0, "line": "Hello, world!" }
    //         ]
    //     },
    //     outputExample: {},
    // },

    'attachLabelToRequest': {
        title: 'Attach Label to Request',
        description: 'Attach a label to a request',
        icon: 'ri:mark-pen-line',
        parameters: z.object({
            index: z.number().describe('the index of the request (need to be the original index, don\'t auto generate it)'),
            label: z.string().describe('the label to attach to the request'),
        }),
        inputExample: {
            "index": 0,
            "label": "test"
        },
        outputExample: {},
    },

    'fuzzRequest': {
        title: 'Fuzz Request',
        icon: 'ph:lightning',
        description: 'Fuzz request with inline payloads. Create one or more markers with payload values.',
        parameters: z.object({
            attachToIndex: z.number().describe('origin index of request you are running fuzzer on'),
            request: z.string().describe('raw request with markers'),
            url: z.string().describe('host with schema and port eg: https://example.com:443'),
            note: z.string().describe('the note to attach to the fuzzer'),
            markers: z.array(z.object({
                marker: z.string().describe('the marker of the payload, §FUZZ§/§WUZZ§/...'),
                payloads: z.array(z.string()).describe('inline payload values'),
            })).describe('the markers with their payloads'),
        }),
        inputExample: {
            "index": 0,
            "request": "GET /?id=§FUZZ§ HTTP/1.1\r\nHost: example.com\r\n\r\n",
            "url": "https://example.com:443",
            "note": "testing param id with values",
            "markers": [
                { "marker": "§FUZZ§", "payloads": ["1", "2", "3"] }
            ]
        },
        outputExample: {},
    },
    'fuzzRequestWithWordlist': {
        title: 'Fuzz Request with Wordlist',
        icon: 'ph:lightning',
        description: 'Fuzz request using wordlist files. Create one or more markers mapped to wordlist file paths.',
        parameters: z.object({
            attachToIndex: z.number().describe('origin index of request you are running fuzzer on'),
            request: z.string().describe('raw request with markers'),
            url: z.string().describe('host with schema and port eg: https://example.com:443'),
            note: z.string().describe('the note to attach to the fuzzer'),
            markers: z.array(z.object({
                marker: z.string().describe('the marker of the payload, §FUZZ§/§WUZZ§/...'),
                filepath: z.string().describe('filepath of the wordlist'),
            })).describe('the markers with their wordlist file paths'),
        }),
        inputExample: {
            "index": 0,
            "request": "GET /§FUZZ§ HTTP/1.1\r\nHost: example.com\r\n\r\n",
            "url": "https://example.com:443",
            "note": "directory bruteforce",
            "markers": [
                { "marker": "§FUZZ§", "filepath": "/path/to/wordlist.txt" }
            ]
        },
        outputExample: {},
    },
    'fuzzReadTable': {
        title: 'Get Fuzzer Table',
        icon: 'ph:lightning',
        description: 'Read the fuzzer table',
        parameters: z.object({
            index: z.number().describe('the index of the request (need to be the original index, don\'t auto generate it)'),
            fuzzerID: z.string().describe('the fuzzer ID'),
            page: z.number().describe('the page to get the data from, start from 1'),
            filter: z.string().describe('filter the results for faster search, use empty string to get all results'),
        }),
        inputExample: {
            "index": 0,
            "fuzzerID": "123",
            "page": 1,
            "filter": ""
        },
        outputExample: {},
    },
    'fuzzReadRequestFromTable': {
        title: 'Get Fuzzer Request',
        icon: 'ph:lightning',
        description: 'Read a request from the fuzzer table',
        parameters: z.object({
            index: z.number().describe('the index of the request (need to be the original index, don\'t auto generate it)'),
            fuzzerID: z.string().describe('the fuzzer ID'),
            rowID: z.string().describe('the id of the row to get the data from'),
        }),
        inputExample: {
            "index": 0,
            "fuzzerID": "123",
            "rowID": "123"
        },
        outputExample: {},
    },

    // === Proxy Browser Automation ===
    'proxyList': {
        title: 'List Proxies',
        icon: 'ri:server-line',
        description: 'Get a list of all running proxy instances with their status, browser type, and configuration',
        parameters: z.object({
        }),
        inputExample: {},
        outputExample: {
            "proxies": [
                {
                    "id": "______________1",
                    "http": "127.0.0.1:8080",
                    "browser": "chrome",
                    "status": "running",
                    "pid": 12345,
                    "createdAt": "2026-01-21T10:00:00Z"
                }
            ]
        },
    },

    'proxyStart': {
        title: 'Start Proxy',
        icon: 'ri:play-circle-line',
        description: 'Start a new proxy instance with optional browser attachment (chrome, firefox, or none)',
        parameters: z.object({
            id: z.string().describe('The proxy ID to restart, leave empty to start new instance'),
        }),
        inputExample: {
            "id": "______________1"
        },
        outputExample: {
            "id": "______________1",
            "http": "127.0.0.1:8080",
            "browser": "chrome",
            "status": "running",
            "pid": 12345,
            "createdAt": "2026-01-21T10:00:00Z"
        },
    },

    'proxyStop': {
        title: 'Stop Proxy',
        icon: 'ri:stop-circle-line',
        description: 'Stop a running proxy instance by ID, or stop all proxies if no ID is provided',
        parameters: z.object({
            id: z.string().optional().describe('The proxy ID to stop. If not provided, stops all running proxies'),
        }),
        inputExample: {
            "id": "______________1"
        },
        outputExample: {
            "success": true,
            "message": "Proxy stopped successfully"
        },
    },

    // 'proxyRestart': {
    //     title: 'Restart Proxy',
    //     icon: 'ri:restart-line',
    //     description: 'Restart a running proxy instance by ID, useful for applying new configurations or recovering from errors',
    //     parameters: z.object({
    //         id: z.string().describe('The proxy ID to restart'),
    //     }),
    //     inputExample: {
    //         "id": "______________1"
    //     },
    //     outputExample: {
    //         "success": true,
    //         "message": "Proxy restarted successfully"
    //     },
    // },

    'proxyScreenshot': {
        title: 'Take Screenshot',
        icon: 'ri:screenshot-line',
        description: 'Capture a screenshot from Chrome browser attached to a proxy instance via Chrome DevTools Protocol, wait after calling the tool',
        parameters: z.object({
            id: z.string().describe('The proxy ID with Chrome browser attached'),
        }),
        inputExample: {
            "id": "______________1",
        },
        outputExample: {
            "message": "initiated taking screenshot, please wait",
            // "screenshot": "iVBORw0KGgoAAAANSUhEUgAAAAUA...",
            // "filePath": "/path/to/cache/screenshot-20260121-103045.png",
            // "size": 52480,
            // "timestamp": "2026-01-21T10:30:45Z"
        },
    },

    'proxyClick': {
        title: 'Click Element',
        icon: 'ri:cursor-line',
        description: 'Click an element on the page using Chrome browser attached to a proxy instance via Chrome DevTools Protocol',
        parameters: z.object({
            id: z.string().describe('The proxy ID with Chrome browser attached'),
            url: z.string().optional().describe('URL to navigate to before clicking. If empty, operates on the current active page'),
            selector: z.string().describe('CSS selector for the element to click (e.g., "#button-id", ".class-name", "button[type=\'submit\']")'),
            waitForNavigation: z.boolean().optional().describe('If true, waits for page navigation after click, useful for form submissions or links (default: false)'),
        }),
        inputExample: {
            "id": "______________1",
            "url": "https://example.com",
            "selector": "#submit-button",
            "waitForNavigation": false
        },
        outputExample: {
            "success": true,
            "message": "Element clicked successfully",
            "selector": "#submit-button",
            "timestamp": "2026-01-21T10:30:45Z"
        },
    },

    'proxyElements': {
        title: 'Get Clickable Elements',
        icon: 'ri:node-tree',
        description: 'Extract information about all clickable elements on the page (buttons, links, inputs) to help identify what can be clicked',
        parameters: z.object({
            id: z.string().describe('The proxy ID with Chrome browser attached'),
            url: z.string().optional().describe('URL to navigate to before extracting elements. If empty, analyzes the current active page'),
        }),
        inputExample: {
            "id": "______________1",
            "url": "https://example.com"
        },
        outputExample: {
            "elements": [
                {
                    "selector": "#login-button",
                    "tagName": "button",
                    "id": "login-button",
                    "class": "btn btn-primary",
                    "text": "Sign In",
                    "type": "submit",
                    "href": "",
                    "name": "",
                    "aria": "Login button",
                    "placeholder": ""
                }
            ],
            "count": 1,
            "timestamp": "2026-01-21T10:30:45Z"
        },
    },

    // === Chrome Tab Management ===
    'proxyListTabs': {
        title: 'List Chrome Tabs',
        icon: 'ri:window-line',
        description: 'Lists all open tabs in the Chrome browser attached to a proxy instance',
        parameters: z.object({
            proxyId: z.string().describe('The proxy ID with Chrome browser attached'),
        }),
        inputExample: {
            "proxyId": "______________1"
        },
        outputExample: {
            "tabs": [
                {
                    "id": "E4B3F8C9-1234-5678-90AB-CDEF12345678",
                    "title": "Example Domain",
                    "url": "https://example.com",
                    "type": "page",
                    "description": ""
                }
            ],
            "count": 2,
            "timestamp": "2026-02-15T17:30:00Z"
        },
    },

    'proxyOpenTab': {
        title: 'Open Chrome Tab',
        icon: 'ri:add-box-line',
        description: 'Opens a new tab in the Chrome browser attached to a proxy instance',
        parameters: z.object({
            proxyId: z.string().describe('The proxy ID with Chrome browser attached'),
            url: z.string().optional().describe('URL to open in the new tab. Defaults to "about:blank" if not provided'),
        }),
        inputExample: {
            "proxyId": "______________1",
            "url": "https://example.com"
        },
        outputExample: {
            "targetId": "E4B3F8C9-1234-5678-90AB-CDEF12345678",
            "url": "https://example.com",
            "timestamp": "2026-02-15T17:30:00Z"
        },
    },

    'proxyNavigateTab': {
        title: 'Navigate Chrome Tab',
        icon: 'ri:compass-line',
        description: 'Navigates a specific tab (or the active tab) to a URL with configurable wait conditions',
        parameters: z.object({
            proxyId: z.string().describe('The proxy ID with Chrome browser attached'),
            targetId: z.string().optional().describe('Chrome target ID of the tab to navigate. If empty, navigates the active tab'),
            url: z.string().describe('URL to navigate to'),
            waitUntil: z.enum(['domcontentloaded', 'load', 'networkidle']).optional().describe('Load state to wait for: "domcontentloaded" (faster), "load" (default), or "networkidle" (wait for network idle)'),
            timeoutMs: z.number().optional().describe('Timeout in milliseconds. Default: 30000'),
        }),
        inputExample: {
            "proxyId": "______________1",
            "targetId": "E4B3F8C9-1234-5678-90AB-CDEF12345678",
            "url": "https://example.com",
            "waitUntil": "load",
            "timeoutMs": 30000
        },
        outputExample: {
            "targetId": "E4B3F8C9-1234-5678-90AB-CDEF12345678",
            "url": "https://example.com",
            "status": "success",
            "navigationId": "nav_1739616000123456789",
            "timestamp": "2026-02-15T17:30:00Z"
        },
    },

    'proxyActivateTab': {
        title: 'Activate Chrome Tab',
        icon: 'ri:focus-line',
        description: 'Switches focus to a specific tab, making it the active tab in Chrome',
        parameters: z.object({
            proxyId: z.string().describe('The proxy ID with Chrome browser attached'),
            targetId: z.string().describe('Chrome target ID of the tab to activate'),
        }),
        inputExample: {
            "proxyId": "______________1",
            "targetId": "E4B3F8C9-1234-5678-90AB-CDEF12345678"
        },
        outputExample: {
            "ok": true,
            "targetId": "E4B3F8C9-1234-5678-90AB-CDEF12345678",
            "timestamp": "2026-02-15T17:30:00Z"
        },
    },

    'proxyCloseTab': {
        title: 'Close Chrome Tab',
        icon: 'ri:close-line',
        description: 'Closes a specific tab in Chrome',
        parameters: z.object({
            proxyId: z.string().describe('The proxy ID with Chrome browser attached'),
            targetId: z.string().describe('Chrome target ID of the tab to close'),
        }),
        inputExample: {
            "proxyId": "______________1",
            "targetId": "E4B3F8C9-1234-5678-90AB-CDEF12345678"
        },
        outputExample: {
            "ok": true,
            "targetId": "E4B3F8C9-1234-5678-90AB-CDEF12345678",
            "timestamp": "2026-02-15T17:30:00Z"
        },
    },

    'proxyReloadTab': {
        title: 'Reload Chrome Tab',
        icon: 'ri:refresh-line',
        description: 'Reloads a specific tab or the active tab, optionally bypassing cache',
        parameters: z.object({
            proxyId: z.string().describe('The proxy ID with Chrome browser attached'),
            targetId: z.string().optional().describe('Chrome target ID of the tab to reload. If empty, reloads the active tab'),
            bypassCache: z.boolean().optional().describe('If true, reloads ignoring cache (hard refresh). Default: false'),
        }),
        inputExample: {
            "proxyId": "______________1",
            "targetId": "E4B3F8C9-1234-5678-90AB-CDEF12345678",
            "bypassCache": false
        },
        outputExample: {
            "ok": true,
            "targetId": "E4B3F8C9-1234-5678-90AB-CDEF12345678",
            "timestamp": "2026-02-15T17:30:00Z"
        },
    },

    'proxyGoBack': {
        title: 'Go Back in Chrome',
        icon: 'ri:arrow-left-line',
        description: 'Navigates back in the browser history for a specific tab or the active tab',
        parameters: z.object({
            proxyId: z.string().describe('The proxy ID with Chrome browser attached'),
            targetId: z.string().optional().describe('Chrome target ID of the tab. If empty, operates on the active tab'),
        }),
        inputExample: {
            "proxyId": "______________1",
            "targetId": "E4B3F8C9-1234-5678-90AB-CDEF12345678"
        },
        outputExample: {
            "ok": true,
            "targetId": "E4B3F8C9-1234-5678-90AB-CDEF12345678",
            "timestamp": "2026-02-15T17:30:00Z"
        },
    },

    'proxyGoForward': {
        title: 'Go Forward in Chrome',
        icon: 'ri:arrow-right-line',
        description: 'Navigates forward in the browser history for a specific tab or the active tab',
        parameters: z.object({
            proxyId: z.string().describe('The proxy ID with Chrome browser attached'),
            targetId: z.string().optional().describe('Chrome target ID of the tab. If empty, operates on the active tab'),
        }),
        inputExample: {
            "proxyId": "______________1",
            "targetId": "E4B3F8C9-1234-5678-90AB-CDEF12345678"
        },
        outputExample: {
            "ok": true,
            "targetId": "E4B3F8C9-1234-5678-90AB-CDEF12345678",
            "timestamp": "2026-02-15T17:30:00Z"
        },
    },

    // TODO: Start/Stop Proxy  // Labelling the recorded
    // TODO: Modify the active request
    // TODO: Note taking for a target and request
    // TODO: Creating Report
    // TODO: Run Cmd
    // TODO: GET HOST INFO, including everything that we have saved
    // TODO: Set Auth profiling for multiple account?
    //       OR using seperate project for that? but then how to connect both with same AI?
    // TODO: Fuzz run status

    // WEBHOOK FOR BLIND SSRF/XXE Testing

    // HARD ONES: Record and Replay

}