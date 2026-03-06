# Grroxy

**Center of your web hacking operations**

An intercepting proxy and web security toolkit with a browser-based UI.

<div >

[![Website](https://img.shields.io/badge/Website-grroxy.com-blue)](https://grroxy.com)

<video src="https://framerusercontent.com/assets/C7VTrJ7zEVWVftMFKQgf6mu0Wos.mp4" autoplay loop muted playsinline></video>

</div>

## Project Structure

```
/apps         main apps (launcher, app, tool runner)
/cmd          go binaries
/grx          grroxy engine (core packages: rawhttp, rawproxy, fuzzer, browser, frontend, templates)
/internal     database schemas, types, config, utilities
/docs         documentation
```

## Installation

```bash
go install github.com/glitchedgitz/grroxy/cmd/grroxy@latest
go install github.com/glitchedgitz/grroxy/cmd/grroxy-app@latest
go install github.com/glitchedgitz/grroxy/cmd/grroxy-tool@latest
go install -v github.com/glitchedgitz/cook/v2/cmd/cook@latest
```

## Usage

```bash
# Start grroxy
grroxy start
```

The web UI is available at `http://127.0.0.1:8090` by default.
