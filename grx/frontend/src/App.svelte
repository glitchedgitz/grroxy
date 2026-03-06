<script>
  import { App } from "cybernetic-ui";
  import "/node_modules/cybernetic-ui/dist/app.css";
  import "./assets/fonts/fonts.css";

  let host = "../";
  // let host = "127.0.0.1:8090";
  // import { GetHost } from "../wailsjs/go/main/App";
  // let host = GetHost();
</script>

<div class="block fixed top-0 left-0 w-full h-full">
  <App {host} />
</div>
