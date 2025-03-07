<script>
  import { App } from "cybernetic-ui";
  import { GetHost } from "../wailsjs/go/main/App";

  let host;

  GetHost().then((h) => {
    host = h;
  });
</script>

{#if host}
  <App {host} />
{/if}
