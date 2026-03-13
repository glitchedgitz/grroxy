# AI Tools - Implementation Tracker

| # | State | Tool Key | Title | Category | Ease | Reason |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | ✅ | `grroxyStatus` | Grroxy Version | System | Very Easy | No params, returns release/backend/frontend versions |
| 2 | - | `filterSaveSet` | Save Filters | Filters | Easy | Simple key-value save |
| 3 | - | `filterGetSets` | Get Filters | Filters | Very Easy | No params, just list stored data |
| 4 | - | `hudGetFilters` | Get HUD Filters | Filters | Easy | No params, reads connected HUD state |
| 5 | - | `hudSetFilters` | Set HUD Filters | Filters | Medium | Array of filter objects, needs validation and HUD sync |
| 6 | ✅ | `getRequestResponseFromID` | Get Request/Response | Data | Easy | Takes record ID, pads to 15 chars, fetches raw req/resp from _req/_resp collections |
| 7 | ✅ | `hostPrintSitemap` | Print Sitemap Tree | Target | Medium | Takes host+path+depth, calls sitemapFetchLogic, returns tree of discovered paths |
| 8 | ✅ | `hostPrintRowsInDetails` | Get Host Rows | Target | Easy | Takes host+filter+limit+offset, expands _data relations, returns rows with req/resp JSON (headers stripped) |
| 9 | ✅ | `getNoteForHost` | Reading Note | Target | Very Easy | Fetches notes from _hosts by host filter |
| 10 | ✅ | `setNoteForHost` | Writing Note | Target | Easy | Line-level edits on stored notes array |
| 11 | ✅ | `listHosts` | List Hosts | Target | Easy | Paginated list with search, expands tech/labels to names |
| 12 | ✅ | `getHostInfo` | Get Host Info | Target | Easy | Expands tech/labels to names, returns full host details |
| 13 | ✅ | `modifyHostLabels` | Update Host Label | Target | Medium | Add/remove/toggle labels, auto-creates label if missing |
| 14 | ✅ | `modifyHostNotes` | Update Host Notes | Target | Medium | Add/update/remove notes array on _hosts record |
| 15 | - | `getQuickSearchSets` | Get Quick Searches | Search | Very Easy | No params, list stored data |
| 16 | - | `addQuickSearchSet` | Add Quick Search | Search | Easy | Name + regex string save |
| 17 | - | `deleteQuickSearchSet` | Delete Quick Search | Search | Very Easy | Single ID delete |
| 18 | ✅ | `sendRequest` | Send Raw Request | Requests | Hard | Takes host+port+tls+raw request+timeout+http2+index+url+note, calls sendRepeaterLogic, tagged as ai/mcp/claudecode |
| 19 | - | `attachLabelToRequest` | Attach Label to Request | Requests | Very Easy | Index + label string |
| 20 | - | `fuzzRequest` | Fuzz Request | Fuzzer | Hard | Marker parsing, payload injection, concurrent requests |
| 21 | - | `fuzzRequestWithWordlist` | Fuzz Request with Wordlist | Fuzzer | Hard | Same as fuzzRequest + file streaming |
| 22 | - | `fuzzReadTable` | Get Fuzzer Table | Fuzzer | Easy | Paginated read with filter |
| 23 | - | `fuzzReadRequestFromTable` | Get Fuzzer Request | Fuzzer | Easy | Single row lookup by IDs |
| 24 | ✅ | `proxyList` | List Proxies | Proxy | Very Easy | Wraps ProxyMgr, lists running instances |
| 25 | ✅ | `proxyStart` | Start Proxy | Proxy | Easy | Wraps ProxyMgr.GetInstance, returns proxy info |
| 26 | ✅ | `proxyStop` | Stop Proxy | Proxy | Easy | Wraps ProxyMgr.StopProxy / StopAllProxies |
| 27 | ✅ | `proxyScreenshot` | Take Screenshot | Proxy | Easy | Wraps ProxyMgr.TakeScreenshot, returns base64 |
| 28 | ✅ | `proxyClick` | Click Element | Proxy | Easy | Wraps ProxyMgr.ClickElement |
| 29 | ✅ | `proxyElements` | Get Clickable Elements | Proxy | Easy | Wraps ProxyMgr.GetElements |
| 30 | ✅ | `proxyListTabs` | List Chrome Tabs | Proxy Tabs | Easy | Wraps ChromeRemote.ListTabs |
| 31 | ✅ | `proxyOpenTab` | Open Chrome Tab | Proxy Tabs | Easy | Wraps ChromeRemote.OpenTab |
| 32 | ✅ | `proxyNavigateTab` | Navigate Chrome Tab | Proxy Tabs | Easy | Wraps ChromeRemote.Navigate |
| 33 | ✅ | `proxyActivateTab` | Activate Chrome Tab | Proxy Tabs | Easy | Wraps ChromeRemote.ActivateTab |
| 34 | ✅ | `proxyCloseTab` | Close Chrome Tab | Proxy Tabs | Easy | Wraps ChromeRemote.CloseTab |
| 35 | ✅ | `proxyReloadTab` | Reload Chrome Tab | Proxy Tabs | Easy | Wraps ChromeRemote.ReloadTab |
| 36 | ✅ | `proxyGoBack` | Go Back in Chrome | Proxy Tabs | Easy | Wraps ChromeRemote.GoBack |
| 37 | ✅ | `proxyGoForward` | Go Forward in Chrome | Proxy Tabs | Easy | Wraps ChromeRemote.GoForward |
| 38 | ✅ | `interceptToggle` | Toggle Intercept | Intercept | Easy | Enable/disable request/response interception on a proxy, auto-forwards pending when disabled |
| 39 | ✅ | `interceptPrintRowsInDetails` | List Intercepted Rows | Intercept | Easy | List intercepted requests/responses with full metadata (host, port, method, path, status, headers) |
| 40 | ✅ | `interceptGetRawRequestAndResponse` | Get Raw Intercept | Intercept | Easy | Get raw HTTP request/response strings for a specific intercepted record |
| 41 | ✅ | `interceptAction` | Intercept Action | Intercept | Medium | Forward (optionally with edits) or drop a pending intercept |
| 42 | ✅ | `proxyType` | Type in Browser | Proxy | Easy | Type text into form fields, clicks to focus, optionally clears, dispatches real key events |
| 43 | ✅ | `proxyEval` | Evaluate JS | Proxy | Medium | Execute arbitrary JavaScript in page context and return result |
| 44 | ✅ | `proxyWaitForSelector` | Wait for Selector | Proxy | Easy | Wait for a CSS selector to become visible, useful for SPA transitions |
