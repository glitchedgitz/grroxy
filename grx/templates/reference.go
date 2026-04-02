package templates

// TemplateReference is the full syntax reference for creating templates.
// Used by /api/templates/info and MCP tools so AI agents can write templates.
const TemplateReference = `# Grroxy Template Reference

## Template Structure (YAML)

` + "```yaml" + `
id: unique-template-id
info:
  title: Human readable title
  description: What this template does
  author: Author name
config:
  type: actions
  mode: any          # "any" = stop after first match, "all" = run all matching tasks
  hooks:
    proxy:
      - request      # runs after request saved to DB (async)
      - response     # runs after response saved to DB (async)
      - before_request  # runs before sending upstream (sync, can modify request)
tasks:
  - id: default      # special: runs when no other task matches (fallback)
    todo:
      - create_label:
          name: "{{req.ext}}"
          color: blue
  - id: task-name
    disabled: false   # set true to skip this task
    condition: req.ext = '.js' OR req.ext = '.mjs'
    todo:
      - create_label:
          name: javascript
          color: yellow
          type: extension
` + "```" + `

## Condition Syntax (dadql)

Conditions filter which tasks run. Uses dadql query language on the request/response data.

### Operators

| Operator | Description          | Example                          |
|----------|----------------------|----------------------------------|
| =        | Exact match          | req.ext = '.js'                  |
| !=       | Not equal            | resp.status != 200               |
| >        | Greater than         | resp.status > 400                |
| >=       | Greater or equal     | resp.length >= 1000              |
| <        | Less than            | resp.status < 300                |
| <=       | Less or equal        | resp.length <= 100               |
| ~        | Contains / regex     | req.path ~ '/api'                |
| !~       | Not contains / regex | req.path !~ '/static'            |
| AND      | Logical AND          | req.ext = '.js' AND host ~ 'cdn' |
| OR       | Logical OR           | req.ext = '.js' OR req.ext = '.ts' |
| NOT      | Logical NOT          | NOT req.ext = '.css'             |
| ()       | Grouping             | (req.ext = '.js' OR req.ext = '.ts') AND host ~ 'api' |

### Regex

Use / to enclose regex patterns:
- req.path ~ /^\/api\/v[0-9]+/
- host ~ /.*\.example\.com$/

### Empty condition

An empty condition "" matches all requests/responses.

## Available Fields

### Request fields (req.*)

| Field              | Type   | Example                    |
|--------------------|--------|----------------------------|
| req.method         | string | GET, POST, PUT, DELETE     |
| req.url            | string | /api/users?page=1          |
| req.path           | string | /api/users                 |
| req.query          | string | page=1&sort=asc            |
| req.ext            | string | .js, .css, .html           |
| req.fragment       | string | section1                   |
| req.headers.<Name> | string | req.headers.User-Agent, req.headers.Content-Type, req.headers.Authorization |
| req.has_cookies    | bool   | true/false                 |
| req.has_params     | bool   | true/false                 |
| req.length         | number | body length in bytes       |

### Response fields (resp.*)

| Field              | Type   | Example                    |
|--------------------|--------|----------------------------|
| resp.status        | number | 200, 404, 500              |
| resp.mime          | string | application/json, text/html|
| resp.headers.<Name>| string | resp.headers.Set-Cookie, resp.headers.Content-Type, resp.headers.X-Powered-By |
| resp.has_cookies   | bool   | true/false                 |
| resp.length        | number | body length in bytes       |
| resp.title         | string | page title                 |
| resp.time          | string | response time              |

### Other fields

| Field    | Type   | Description              |
|----------|--------|--------------------------|
| host     | string | target hostname          |
| port     | string | target port              |
| index    | number | request index number     |
| is_https | bool   | true if HTTPS            |

## Variable Interpolation

Use {{field}} in action values to insert request/response data:
- {{req.ext}} → .js
- {{resp.mime}} → application/json
- {{req.headers.User-Agent}} → Mozilla/5.0 ...
- {{host}} → example.com

## Actions

### create_label
Create a label and attach it to the request row.
` + "```yaml" + `
- create_label:
    name: "{{req.ext}}"       # required, supports variables
    color: yellow              # optional (blue, red, green, yellow, orange, purple, pink, ignore)
    type: extension            # optional (extension, mime, endpoint, custom)
    icon: js                   # optional
` + "```" + `

### set
Modify request/response fields. Key is the field path, value is the new value.
` + "```yaml" + `
- set:
    req.method: POST
    req.headers.User-Agent: CustomBot/1.0
    req.headers.X-Custom: "{{host}}"
    req.body: '{"key":"value"}'
    req.path: /new/path
    req.query.page: "2"
` + "```" + `

### delete
Remove request/response fields. Supports wildcard with * suffix.
` + "```yaml" + `
- delete:
    req.headers.Sec-*: ""     # wildcard: removes all Sec-* headers
    req.headers.Cookie: ""
    req.body: ""
    req.query.debug: ""
` + "```" + `

### replace
Search and replace in the raw request. Supports string and regex.
` + "```yaml" + `
- replace:
    search: Mozilla/5.0       # required
    value: CustomBot/1.0      # required
    regex: false               # optional, default false
` + "```" + `

### send_request
Send a modified copy of the current request. Response is saved to DB.
` + "```yaml" + `
- send_request:
    req.method: PUT            # optional override
    req.headers:               # optional override (map)
      Content-Type: application/json
    req.body: '{"test":true}'  # optional override
` + "```" + `

## Hooks

| Hook            | When it runs                     | Sync/Async | Can modify request |
|-----------------|----------------------------------|------------|-------------------|
| before_request  | Before sending to target server  | Sync       | Yes (set/delete)  |
| request         | After request saved to DB        | Async      | No                |
| response        | After response saved to DB       | Async      | No                |

## Complete Examples

### Label JavaScript files
` + "```yaml" + `
id: label-js
info:
  title: Label JavaScript
  description: Labels all JavaScript requests
config:
  mode: any
  hooks:
    proxy: [request]
tasks:
  - id: default
    todo:
      - create_label:
          name: "{{req.ext}}"
          type: extension
          color: ignore
  - id: js
    condition: req.ext = '.js' OR req.ext = '.mjs'
    todo:
      - create_label:
          name: javascript
          icon: js
          type: extension
          color: yellow
` + "```" + `

### Strip tracking headers
` + "```yaml" + `
id: strip-tracking
info:
  title: Strip Tracking Headers
  description: Remove tracking and fingerprinting headers before sending
config:
  hooks:
    proxy: [before_request]
tasks:
  - id: remove-sec-headers
    condition: ""
    todo:
      - delete:
          req.headers.Sec-*: ""
` + "```" + `

### Label API responses by status
` + "```yaml" + `
id: status-labels
info:
  title: Status Labels
  description: Label responses by HTTP status code
config:
  mode: any
  hooks:
    proxy: [response]
tasks:
  - id: success
    condition: resp.status >= 200 AND resp.status < 300
    todo:
      - create_label:
          name: success
          color: green
  - id: redirect
    condition: resp.status >= 300 AND resp.status < 400
    todo:
      - create_label:
          name: redirect
          color: yellow
  - id: client-error
    condition: resp.status >= 400 AND resp.status < 500
    todo:
      - create_label:
          name: client-error
          color: orange
  - id: server-error
    condition: resp.status >= 500
    todo:
      - create_label:
          name: server-error
          color: red
` + "```" + `
`
