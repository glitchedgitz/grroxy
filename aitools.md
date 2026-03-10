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
| 9 | - | `getNoteForHost` | Reading Note | Target | Very Easy | Single host lookup |
| 10 | - | `setNoteForHost` | Writing Note | Target | Easy | Line-level edits on stored notes |
| 11 | - | `listHosts` | List Hosts | Target | Easy | Paginated list with search |
| 12 | - | `getHostInfo` | Get Host Info | Target | Easy | Single host lookup, aggregates stored data |
| 13 | - | `modifyHostLabels` | Update Host Label | Target | Medium | Add/remove/toggle actions with color/type |
| 14 | - | `modifyHostNotes` | Update Host Notes | Target | Medium | Add/update/remove with index management |
| 15 | - | `getQuickSearchSets` | Get Quick Searches | Search | Very Easy | No params, list stored data |
| 16 | - | `addQuickSearchSet` | Add Quick Search | Search | Easy | Name + regex string save |
| 17 | - | `deleteQuickSearchSet` | Delete Quick Search | Search | Very Easy | Single ID delete |
| 18 | ✅ | `sendRequest` | Send Raw Request | Requests | Hard | Takes host+port+tls+raw request+timeout+http2+index+url+note, calls sendRepeaterLogic, tagged as ai/mcp/claudecode |
| 19 | - | `attachLabelToRequest` | Attach Label to Request | Requests | Very Easy | Index + label string |
| 20 | - | `fuzzRequest` | Fuzz Request | Fuzzer | Hard | Marker parsing, payload injection, concurrent requests |
| 21 | - | `fuzzRequestWithWordlist` | Fuzz Request with Wordlist | Fuzzer | Hard | Same as fuzzRequest + file streaming |
| 22 | - | `fuzzReadTable` | Get Fuzzer Table | Fuzzer | Easy | Paginated read with filter |
| 23 | - | `fuzzReadRequestFromTable` | Get Fuzzer Request | Fuzzer | Easy | Single row lookup by IDs |
| 24 | - | `proxyList` | List Proxies | Proxy | Very Easy | No params, list running instances |
| 25 | - | `proxyStart` | Start Proxy | Proxy | Hard | Process spawning, port allocation, browser launch |
| 26 | - | `proxyStop` | Stop Proxy | Proxy | Medium | Process cleanup, graceful shutdown |
| 27 | - | `proxyScreenshot` | Take Screenshot | Proxy | Medium | CDP connection, image capture and encoding |
| 28 | - | `proxyClick` | Click Element | Proxy | Medium | CDP DOM interaction, selector resolution |
| 29 | - | `proxyElements` | Get Clickable Elements | Proxy | Medium | CDP DOM query, element extraction |
| 30 | - | `proxyListTabs` | List Chrome Tabs | Proxy Tabs | Easy | CDP target list query |
| 31 | - | `proxyOpenTab` | Open Chrome Tab | Proxy Tabs | Easy | CDP createTarget call |
| 32 | - | `proxyNavigateTab` | Navigate Chrome Tab | Proxy Tabs | Medium | CDP navigation with wait conditions |
| 33 | - | `proxyActivateTab` | Activate Chrome Tab | Proxy Tabs | Easy | CDP activateTarget call |
| 34 | - | `proxyCloseTab` | Close Chrome Tab | Proxy Tabs | Easy | CDP closeTarget call |
| 35 | - | `proxyReloadTab` | Reload Chrome Tab | Proxy Tabs | Easy | CDP Page.reload call |
| 36 | - | `proxyGoBack` | Go Back in Chrome | Proxy Tabs | Easy | CDP history navigation |
| 37 | - | `proxyGoForward` | Go Forward in Chrome | Proxy Tabs | Easy | CDP history navigation |
