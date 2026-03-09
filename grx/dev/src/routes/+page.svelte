<script lang="ts">
  import Logo from "$lib/Logo.svelte";
  import SidebarCategory from "$lib/SidebarCategory.svelte";
  import CodeEditor from "$lib/CodeEditor.svelte";
  import { Splitpanes, Pane } from "$lib/Splitpanes/splitpanes";
  import {
    APP_ENDPOINTS,
    getEndpoints,
    getCategories,
    sendRequest,
    login,
    fetchProjects,
    openProject,
    type AppType,
    type ApiEndpoint,
    type ApiResponse,
    type Project,
  } from "$lib/api";

  // Restore persisted session
  const saved = typeof localStorage !== "undefined"
    ? JSON.parse(localStorage.getItem("grroxy-session") || "null")
    : null;

  let appType = $state<AppType>(saved?.appType ?? "launcher");
  let ENDPOINTS = $derived(getEndpoints(appType));
  let API_CATEGORIES = $derived(getCategories(ENDPOINTS));
  let launcherUrl = $state(saved?.launcherUrl ?? "http://127.0.0.1:8090");
  let authToken = $state(saved?.authToken ?? "");
  let projectToken = $state(saved?.projectToken ?? "");
  let authEmail = $state(saved?.authEmail ?? "new@example.com");
  let authPassword = $state(saved?.authPassword ?? "1234567890");
  let authLoading = $state(false);
  let authError = $state("");
  let loggedIn = $state(!!saved?.authToken);
  let projects = $state<Project[]>(saved?.projects ?? []);
  let selectedProject = $state<Project | null>(saved?.selectedProject ?? null);
  let projectsLoading = $state(false);
  let projectDropdownOpen = $state(false);
  let showLoginPopup = $state(!saved?.authToken);
  let showEndpointsTable = $state(false);

  function saveSession() {
    localStorage.setItem("grroxy-session", JSON.stringify({
      launcherUrl,
      authToken,
      projectToken,
      authEmail,
      authPassword,
      appType,
      projects,
      selectedProject,
    }));
  }
  let baseUrl = $derived(
    appType === "launcher" ? launcherUrl
    : selectedProject ? `http://${selectedProject.data.ip}`
    : launcherUrl,
  );
  let selectedEndpoint = $state<ApiEndpoint | null>(null);
  let requestPath = $state("");
  let requestBody = $state("");
  let response = $state<ApiResponse | null>(null);
  let loading = $state(false);
  let error = $state("");
  let history = $state<
    Array<{
      endpoint: ApiEndpoint;
      path: string;
      response: ApiResponse;
      timestamp: Date;
    }>
  >([]);

  async function doLogin() {
    authLoading = true;
    authError = "";
    try {
      const res = await login(launcherUrl, authEmail, authPassword);
      authToken = res.token;
      loggedIn = true;
      saveSession();
      await loadProjects();
    } catch (err) {
      authError = err instanceof Error ? err.message : "Login failed";
    } finally {
      authLoading = false;
    }
  }

  function logout() {
    authToken = "";
    projectToken = "";
    loggedIn = false;
    projects = [];
    selectedProject = null;
    appType = "launcher";
    localStorage.removeItem("grroxy-session");
  }

  async function loadProjects() {
    projectsLoading = true;
    try {
      projects = await fetchProjects(launcherUrl, authToken);
      saveSession();
    } catch (err) {
      authError = err instanceof Error ? err.message : "Failed to load projects";
    } finally {
      projectsLoading = false;
    }
  }

  async function selectProject(project: Project) {
    projectsLoading = true;
    try {
      const opened = await openProject(launcherUrl, project.name, authToken);
      selectedProject = opened;
      // Login to the project's app server with the same credentials
      const projectUrl = `http://${opened.data.ip}`;
      const projAuth = await login(projectUrl, authEmail, authPassword);
      projectToken = projAuth.token;
      appType = "app";
      selectedEndpoint = null;
      projectDropdownOpen = false;
      saveSession();
    } catch (err) {
      error = err instanceof Error ? err.message : "Failed to open project";
    } finally {
      projectsLoading = false;
    }
  }

  function selectEndpoint(endpoint: ApiEndpoint) {
    selectedEndpoint = endpoint;
    requestPath = endpoint.path;
    requestBody = endpoint.defaultBody
      ? JSON.stringify(endpoint.defaultBody, null, 2)
      : "";
    response = null;
    error = "";
  }

  async function send() {
    if (!selectedEndpoint) return;
    loading = true;
    error = "";
    response = null;

    try {
      const activeToken = appType === "launcher" ? authToken : projectToken;
      const res = await sendRequest(
        baseUrl,
        selectedEndpoint,
        requestPath,
        requestBody,
        activeToken,
      );
      response = res;
      history = [
        {
          endpoint: selectedEndpoint,
          path: requestPath,
          response: res,
          timestamp: new Date(),
        },
        ...history.slice(0, 49),
      ];
    } catch (err) {
      error = err instanceof Error ? err.message : "Request failed";
    } finally {
      loading = false;
    }
  }

  function getMethodColor(method: string): string {
    switch (method) {
      case "GET":
        return "text-sage";
      case "POST":
        return "text-malibu";
      case "DELETE":
        return "text-coral";
      default:
        return "text-ivory";
    }
  }

  function getStatusColor(status: number): string {
    if (status < 300) return "text-green1";
    if (status < 400) return "text-yellow";
    if (status < 500) return "text-orange";
    return "text-coral";
  }

  const multiCategories = $derived(
    API_CATEGORIES.filter(
      (c) => ENDPOINTS.filter((e) => e.category === c).length > 1,
    ),
  );
  const singleEndpoints = $derived(
    ENDPOINTS.filter(
      (e) => ENDPOINTS.filter((x) => x.category === e.category).length === 1,
    ).sort((a, b) => {
      const aApi = a.path.startsWith("/api/") ? 1 : 0;
      const bApi = b.path.startsWith("/api/") ? 1 : 0;
      if (aApi !== bApi) return aApi - bApi;
      return a.path.localeCompare(b.path);
    }),
  );

  $effect(() => {
    if (!selectedEndpoint && ENDPOINTS.length > 0) {
      selectEndpoint(ENDPOINTS[0]);
    }
  });
</script>

<div class="h-screen overflow-hidden">
  <Splitpanes>
  <Pane size={20} minSize={15} maxSize={30}>
  <!-- Sidebar -->
  <nav
    class="flex h-full overflow-hidden flex-col bg-dark"
  >
    <!-- Logo + Config -->
    <div class="p-24 pb-8 relative">
      <Logo class="w-[80px]" />
      <div
        class="mt-4 text-[8px] font-OCR uppercase text-white/40 tracking-[2px]"
      >
        DEV MODE
      </div>
      <div
        class="h-24 bg-gradient-to-b from-dark via-50% w-full absolute bottom-0 -mb-24"
      ></div>
    </div>

    <!-- API Endpoints grouped by category -->
    <section class="flex flex-grow flex-col pt-16 overflow-auto">
      {#if singleEndpoints.length > 0}
        <SidebarCategory
          categoryName="/api"
          endpoints={singleEndpoints}
          {selectedEndpoint}
          onselect={selectEndpoint}
        />
      {/if}
      {#each multiCategories as category}
        <SidebarCategory
          categoryName={category}
          endpoints={ENDPOINTS.filter((e) => e.category === category)}
          {selectedEndpoint}
          onselect={selectEndpoint}
        />
      {/each}
    </section>

    <!-- History count -->
    {#if history.length > 0}
      <div
        class="p-12 border-t border-white/5 text-[10px] text-white/30 flex justify-between items-center"
      >
        <span>{history.length} requests</span>
        <button
          class="btn-red-ghost btn-sm"
          onclick={() => (history = [])}>clear</button
        >
      </div>
    {/if}
  </nav>
  </Pane>

  <Pane>
  <!-- Main Content -->
  <div class="h-full flex flex-col overflow-hidden bg-dark">
    <!-- Top Bar -->
    <div class="flex items-center gap-8 pl-0 p-16 border-b border-white/5">
      <!-- App Type Tabs -->
      <div class="flex gap-4">
        {#each ["launcher", "app", "tool"] as type}
          <button
            onclick={() => {
              appType = type as AppType;
              selectedEndpoint = null;
            }}
            class="btn-sm {appType === type ? 'btn-green-dim' : 'btn-white-ghost'}"
          >
            {type}
          </button>
        {/each}
      </div>

      <div class="flex-1"></div>

      <!-- Project Dropdown -->
      {#if selectedProject}
        <div class="relative">
          <button
            class="btn-sm btn-green-outline flex items-center gap-4"
            onclick={() => (projectDropdownOpen = !projectDropdownOpen)}
          >
            {selectedProject.name}
            <span class="text-[8px] text-green/60">{selectedProject.data.ip}</span>
            <span class="text-[10px]">{projectDropdownOpen ? "\u25B2" : "\u25BC"}</span>
          </button>
          {#if projectDropdownOpen}
            <div class="absolute right-0 top-full mt-4 z-50 bg-surface border border-white/10 rounded shadow-lg min-w-[200px] max-h-[300px] overflow-auto">
              {#each projects as project}
                <button
                  class="btn-sm btn-white-ghost w-full text-left {project.id === selectedProject?.id ? 'btn-green-dim' : ''}"
                  onclick={() => selectProject(project)}
                >
                  <span class="truncate flex-1">{project.name}</span>
                  <span class="text-[9px] text-white/30">{project.data?.state}</span>
                </button>
              {/each}
            </div>
          {/if}
        </div>
      {/if}

      <button onclick={() => (showEndpointsTable = true)} class="btn-white-ghost btn-sm">Endpoints</button>

      {#if loggedIn}
        <span class="text-[10px] text-green">{authEmail}</span>
        <button onclick={logout} class="btn-red-ghost btn-sm">logout</button>
      {:else}
        <button onclick={() => (showLoginPopup = true)} class="btn-green-dim btn-sm">Login</button>
      {/if}
    </div>

    <!-- Endpoints Table Popup -->
    {#if showEndpointsTable}
      <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
      <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onclick={() => (showEndpointsTable = false)}>
        <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
        <div class="bg-surface border border-white/10 rounded-lg p-16 w-[700px] max-h-[80vh] flex flex-col gap-12" onclick={(e) => e.stopPropagation()}>
          <div class="flex items-center justify-between">
            <div class="text-[11px] text-white/40 uppercase tracking-wider">{appType} endpoints</div>
            <button onclick={() => (showEndpointsTable = false)} class="btn-white-ghost btn-sm">Close</button>
          </div>
          <div class="overflow-auto">
            <table class="w-full text-[12px]">
              <thead>
                <tr class="border-b border-white/10 text-left text-[10px] text-white/40 uppercase tracking-wider">
                  <th class="py-6 pr-12">Method</th>
                  <th class="py-6 pr-12">Path</th>
                  <th class="py-6">Description</th>
                </tr>
              </thead>
              <tbody>
                {#each ENDPOINTS as endpoint}
                  <tr class="border-b border-white/[0.03] hover:bg-white/5 cursor-pointer" onclick={() => { selectEndpoint(endpoint); showEndpointsTable = false; }}>
                    <td class="py-4 pr-12 {getMethodColor(endpoint.method)} font-semibold">{endpoint.method}</td>
                    <td class="py-4 pr-12 text-white/70">{endpoint.path}</td>
                    <td class="py-4 text-white/30">{endpoint.description}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    {/if}

    <!-- Login / Project Popup -->
    {#if showLoginPopup}
      <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
        <div class="bg-surface border border-white/10 rounded-lg p-24 w-[400px] flex flex-col gap-12">
          {#if !loggedIn}
            <div class="text-[11px] text-white/40 uppercase tracking-wider">Connect to Launcher</div>
            <input
              type="text"
              bind:value={launcherUrl}
              placeholder="http://127.0.0.1:8090"
              class="input-variant-1 h-24"
            />
            <input
              type="text"
              bind:value={authEmail}
              placeholder="Email"
              class="input-variant-1 h-24"
            />
            <input
              type="password"
              bind:value={authPassword}
              placeholder="Password"
              class="input-variant-1 h-24"
            />
            {#if authError}
              <div class="text-[10px] text-coral">{authError}</div>
            {/if}
            <div class="flex gap-8 mt-4">
              <button
                onclick={doLogin}
                disabled={authLoading}
                class="btn-green flex-1 {authLoading ? 'cursor-not-allowed opacity-50' : ''}"
              >
                {authLoading ? "..." : "Login"}
              </button>
              <button onclick={() => (showLoginPopup = false)} class="btn-white-ghost">
                Cancel
              </button>
            </div>
          {:else}
            <div class="flex items-center justify-between">
              <div class="text-[11px] text-white/40 uppercase tracking-wider">Select a Project</div>
              <button class="btn-green-dim btn-sm" onclick={loadProjects} disabled={projectsLoading}>
                {projectsLoading ? "..." : "Refresh"}
              </button>
            </div>
            {#if projectsLoading}
              <div class="text-green text-[13px] animate-pulse py-12 text-center">Loading projects...</div>
            {:else if projects.length === 0}
              <div class="text-white/30 text-[12px] py-12 text-center">No projects found</div>
            {:else}
              <div class="flex flex-col gap-4 max-h-[300px] overflow-auto">
                {#each projects as project}
                  <button
                    class="btn-white-ghost w-full text-left p-12 rounded border border-white/5 hover:border-white/15"
                    onclick={() => {
                      selectProject(project);
                      showLoginPopup = false;
                    }}
                  >
                    <div class="text-[13px] text-white/80">{project.name}</div>
                    <div class="text-[10px] text-white/30 mt-2">{project.data?.ip} &middot; {project.data?.state}</div>
                  </button>
                {/each}
              </div>
            {/if}
            <button onclick={() => (showLoginPopup = false)} class="btn-white-ghost btn-sm self-end">
              Close
            </button>
          {/if}
        </div>
      </div>
    {/if}

    {#if selectedEndpoint}
      <div class="flex-1 overflow-hidden">
        <Splitpanes horizontal>
        <!-- Editors -->
        <Pane size={history.length > 0 ? 75 : 100} minSize={40}>
        <div class="h-full overflow-hidden">
          <Splitpanes>
          <Pane size={50}>
          <!-- Request Panel -->
          <div
            class="h-full bg-surface rounded flex flex-col overflow-hidden"
          >
            <div class="p-12 border-b border-white/5">
              <div class="flex relative items-center gap-8">
                <div
                  class="absolute px-12  {getMethodColor(selectedEndpoint.method)}  text-sm font-semi-bold tracking-widest"
                >
                  {selectedEndpoint.method}
                </div>
                <input
                  type="text"
                  bind:value={requestPath}
                  class="input-variant-1 h-24 pl-64 flex-1"
                />
                <button
                  onclick={send}
                  disabled={loading}
                  class="btn-green btn-sm {loading ? 'cursor-not-allowed opacity-50' : ''}"
                >
                  {loading ? "..." : "Send"}
                </button>
              </div>
              <p class="text-[11px] text-white/30 mt-6">
                {selectedEndpoint.description}
              </p>
            </div>

            {#if selectedEndpoint.method !== "GET" || selectedEndpoint.defaultBody}
              <div class="flex-1 flex flex-col overflow-hidden">
                <div
                  class="px-12 py-6 text-[10px] text-white/20 uppercase tracking-wider"
                >
                  Body
                </div>
                <div class="flex-1 overflow-hidden">
                  <CodeEditor bind:value={requestBody} placeholder={"{ }"} />
                </div>
              </div>
            {/if}

            {#if selectedEndpoint.examples && selectedEndpoint.examples.length > 0}
              <div class="px-12 py-8 border-t border-white/5 flex items-center gap-6 flex-wrap">
                <span class="text-[10px] text-white/20 uppercase tracking-wider">Examples</span>
                {#each selectedEndpoint.examples as example}
                  <button
                    class="btn-white-ghost btn-sm text-[10px]"
                    onclick={() => {
                      requestBody = JSON.stringify(example.request, null, 2);
                    }}
                  >
                    {example.name}
                  </button>
                {/each}
              </div>
            {/if}
          </div>

          </Pane>

          <Pane size={50}>
          <!-- Response Panel -->
          <div class="h-full flex flex-col bg-surface rounded overflow-hidden">
            <div
              class="px-12 py-8 border-b border-white/5 flex items-center justify-between"
            >
              <span class="text-[10px] text-white/20 uppercase tracking-wider"
                >Response</span
              >
              {#if response}
                <div class="flex items-center gap-12 text-[12px]">
                  <span class="{getStatusColor(response.status)} ">
                    {response.status}
                    {response.statusText}
                  </span>
                  <span class="text-white/30">{response.time}ms</span>
                </div>
              {/if}
            </div>

            <div class="flex-1 overflow-auto">
              {#if error}
                <div class="p-12">
                  <div
                    class="bg-red/10 border border-red/20 rounded p-12 text-coral text-[12px]"
                  >
                    {error}
                  </div>
                </div>
              {:else if response}
                <details class="border-b border-white/5">
                  <summary
                    class="px-12 py-6 text-[11px] text-white/30 cursor-pointer hover:text-white/60"
                  >
                    Headers ({Object.keys(response.headers).length})
                  </summary>
                  <div class="px-12 pb-8">
                    {#each Object.entries(response.headers) as [key, value]}
                      <div class="text-[11px] py-2">
                        <span class="text-whiskey">{key}:</span>
                        <span class="text-white/60 ml-4">{value}</span>
                      </div>
                    {/each}
                  </div>
                </details>

                <div class="flex-1 overflow-hidden">
                  <CodeEditor
                    value={typeof response.body === "string"
                      ? response.body
                      : JSON.stringify(response.body, null, 2)}
                    readonly
                  />
                </div>
              {:else if !loading}
                <div
                  class="flex items-center justify-center h-full text-white/20 text-[13px]"
                >
                  Click Send to make a request
                </div>
              {:else}
                <div class="flex items-center justify-center h-full">
                  <div class="text-green text-[13px] animate-pulse">
                    Sending...
                  </div>
                </div>
              {/if}
            </div>
          </div>
          </Pane>
          </Splitpanes>
        </div>
        </Pane>

        <!-- History -->
        {#if history.length > 0}
          <Pane size={25} minSize={10} maxSize={50}>
          <div class="h-full overflow-y-auto border-t border-white/5">
            <div class="px-12 py-6 flex items-center justify-between border-b border-white/5">
              <span class="text-[10px] text-white/20 uppercase tracking-wider">{history.length} requests</span>
              <button
                class="btn-red-ghost btn-sm"
                onclick={() => (history = [])}>clear</button
              >
            </div>
            {#each history as entry}
              <button
                class="btn-white-ghost btn-sm w-full text-left border-b border-white/[0.03]"
                onclick={() => {
                  selectEndpoint(entry.endpoint);
                  requestPath = entry.path;
                  response = entry.response;
                }}
              >
                <span class="{getMethodColor(entry.endpoint.method)}  w-36"
                  >{entry.endpoint.method}</span
                >
                <span class="text-white/60 truncate flex-1">{entry.path}</span>
                <span class={getStatusColor(entry.response.status)}
                  >{entry.response.status}</span
                >
                <span class="text-white/20">{entry.response.time}ms</span>
                <span class="text-white/[0.15]"
                  >{entry.timestamp.toLocaleTimeString()}</span
                >
              </button>
            {/each}
          </div>
          </Pane>
        {/if}
        </Splitpanes>
      </div>
    {/if}
  </div>
  </Pane>
  </Splitpanes>
</div>
