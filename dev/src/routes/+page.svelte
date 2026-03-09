<script lang="ts">
  import Logo from "$lib/Logo.svelte";
  import SidebarCategory from "$lib/SidebarCategory.svelte";
  import {
    ENDPOINTS,
    API_CATEGORIES,
    sendRequest,
    login,
    type ApiEndpoint,
    type ApiResponse,
  } from "$lib/api";

  let baseUrl = $state("http://127.0.0.1:8090");
  let authToken = $state("");
  let authEmail = $state("new@example.com");
  let authPassword = $state("1234567890");
  let authLoading = $state(false);
  let authError = $state("");
  let loggedIn = $state(false);
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
      const res = await login(baseUrl, authEmail, authPassword);
      authToken = res.token;
      loggedIn = true;
    } catch (err) {
      authError = err instanceof Error ? err.message : "Login failed";
    } finally {
      authLoading = false;
    }
  }

  function logout() {
    authToken = "";
    loggedIn = false;
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
      const res = await sendRequest(
        baseUrl,
        selectedEndpoint,
        requestPath,
        requestBody,
        authToken,
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
    API_CATEGORIES.filter((c) => ENDPOINTS.filter((e) => e.category === c).length > 1)
  );
  const singleEndpoints = $derived(
    ENDPOINTS.filter((e) => ENDPOINTS.filter((x) => x.category === e.category).length === 1)
      .sort((a, b) => {
        const aApi = a.path.startsWith('/api/') ? 1 : 0;
        const bApi = b.path.startsWith('/api/') ? 1 : 0;
        if (aApi !== bApi) return aApi - bApi;
        return a.path.localeCompare(b.path);
      })
  );

  $effect(() => {
    if (!selectedEndpoint && ENDPOINTS.length > 0) {
      selectEndpoint(ENDPOINTS[0]);
    }
  });
</script>

<div class="flex h-screen overflow-hidden">
  <!-- Sidebar -->
  <nav
    class="flex h-full w-[280px] min-w-[280px] flex-col bg-dark border-r border-white/5"
  >
    <!-- Logo + Config -->
    <div class="p-24">
      <Logo class="w-[80px]" />
      <div class="mt-12 text-[10px] font-OCR uppercase text-white/40 tracking-[2px]">
        DEV MODE
      </div>

      <div class="mt-16 flex flex-col gap-6">
        <input
          type="text"
          bind:value={baseUrl}
          placeholder="http://127.0.0.1:8090"
          class="w-full bg-transparent border-b border-white/10 pb-4 text-[11px] text-white/80 focus:border-green focus:outline-none placeholder:text-white/20"
        />
        {#if !loggedIn}
          <input
            type="text"
            bind:value={authEmail}
            placeholder="Email"
            class="w-full bg-transparent border-b border-white/10 pb-4 text-[11px] text-white/80 focus:border-green focus:outline-none placeholder:text-white/20"
          />
          <input
            type="password"
            bind:value={authPassword}
            placeholder="Password"
            class="w-full bg-transparent border-b border-white/10 pb-4 text-[11px] text-white/80 focus:border-green focus:outline-none placeholder:text-white/20"
          />
          <button
            onclick={doLogin}
            disabled={authLoading}
            class="mt-4 w-full py-6 rounded text-[11px] font-OCR uppercase tracking-[2px] transition-colors
              {authLoading
                ? 'bg-white/5 text-white/30 cursor-not-allowed'
                : 'bg-green/20 text-green hover:bg-green/30'}"
          >
            {authLoading ? "..." : "Login"}
          </button>
          {#if authError}
            <div class="text-[10px] text-coral">{authError}</div>
          {/if}
        {:else}
          <div class="flex items-center justify-between">
            <span class="text-[10px] text-green">{authEmail}</span>
            <button
              onclick={logout}
              class="text-[10px] text-coral/60 hover:text-coral"
            >logout</button>
          </div>
        {/if}
      </div>
    </div>

    <!-- API Endpoints grouped by category -->
    <section class="flex flex-grow flex-col overflow-auto">
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
          class="text-coral/60 hover:text-coral"
          onclick={() => (history = [])}>clear</button
        >
      </div>
    {/if}
  </nav>

  <!-- Main Content -->
  <div class="flex-1 flex flex-col overflow-hidden bg-dark">
    {#if selectedEndpoint}
      <!-- Request / Response Split -->
      <div class="flex-1 flex overflow-hidden">
        <!-- Request Panel -->
        <div
          class="w-1/2 flex flex-col border-r border-white/5 overflow-hidden"
        >
          <div class="p-12 border-b border-white/5">
            <div class="flex items-center gap-8">
              <span
                class="{getMethodColor(selectedEndpoint.method)}  text-[13px]"
              >
                {selectedEndpoint.method}
              </span>
              <input
                type="text"
                bind:value={requestPath}
                class="flex-1 bg-white/5 border border-white/10 rounded px-8 py-6 text-[12px] text-white focus:border-green focus:outline-none"
              />
              <button
                onclick={send}
                disabled={loading}
                class="px-16 py-6 rounded text-[12px] transition-colors
									{loading
                  ? 'bg-white/5 text-white/30 cursor-not-allowed'
                  : 'bg-green text-dark hover:bg-green1'}"
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
              <textarea
                bind:value={requestBody}
                placeholder={"{ }"}
                spellcheck="false"
                class="flex-1 bg-dark text-[12px] text-white/80 p-12 resize-none focus:outline-none border-none leading-relaxed"
              ></textarea>
            </div>
          {/if}
        </div>

        <!-- Response Panel -->
        <div class="w-1/2 flex flex-col overflow-hidden">
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

              <pre
                class="p-12 text-[12px] text-white/80 leading-relaxed whitespace-pre-wrap break-words">{typeof response.body ===
                "string"
                  ? response.body
                  : JSON.stringify(response.body, null, 2)}</pre>
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
      </div>
    {/if}

    <!-- History -->
    {#if history.length > 0}
      <div class="border-t border-white/5 max-h-[140px] overflow-y-auto">
        {#each history as entry}
          <button
            class="w-full text-left px-12 py-4 text-[11px] hover:bg-white/5 flex items-center gap-8 border-b border-white/[0.03]"
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
    {/if}
  </div>
</div>
