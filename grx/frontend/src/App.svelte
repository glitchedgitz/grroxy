<script>
  import { App } from "cybernetic-ui";
  import "/node_modules/cybernetic-ui/dist/app.css";
  import "./assets/fonts/fonts.css";

  let host = null;

  if (window["electronAPI"]?.getHost) {
    window["electronAPI"].getHost().then((h) => {
      host = "http://" + h;
    });
  } else {
    host = "../";
  }
</script>

{#if host}
<div class="block fixed top-0 left-0 w-full h-full">
  <App {host} />
</div>
{/if}
