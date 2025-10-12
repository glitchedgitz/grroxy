<script>
  import { App } from "cybernetic-ui";
  import "./assets/fonts/fonts.css";
  // import { GetHost } from "../wailsjs/go/main/App";

  let host = "127.0.0.1:8090";

  // GetHost().then((h) => {
  //   host = h;
  // });
</script>

{#if host}
  <App {host} />
{/if}
