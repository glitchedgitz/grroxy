# grroxy

A cyber security toolkit blending manual testing with AI Agents. 
Focusing on product taste and user experience. Keeping it hacky and adaptable how things changes in this era, idea is to have a toolkit that prioritise how hackers work.

[![Website](https://img.shields.io/badge/Website-grroxy.com-blue)](https://grroxy.com) [![Discord](https://img.shields.io/badge/Discord-Join-5865F2?logo=discord&logoColor=white)](https://discord.gg/K8pGK6XatC)

<img src="https://github.com/user-attachments/assets/6f062f35-1236-4316-8918-d3be86184843" />
<img src="https://github.com/user-attachments/assets/6240afd6-68d6-4794-85ca-f68be81649a0">

# Why grrrr...?
Grroxy was started 4 years back, I had.. and have some pain points with the proxy tools, the initial [idea](https://x.com/glitchedgitz/status/1750176261475840215) was different and didn't workout very well, so here we are with the new [one](https://grroxy.com).

# Highlights
- Designed for productivity   
- AI Assistant Sidebar  
- Claude Code and MCP Support
- Launch Multiple Proxies with isolated profiles, Resume in a click!
- Encode/Decode on the go on the go
- CWD Preview
- And much more... 

<img src="https://github.com/user-attachments/assets/03fbdeec-6898-448f-b35b-51bfa32794ae">

<img src="https://github.com/user-attachments/assets/691c9121-f4f2-4451-8302-9979402b4512">

<img src="https://github.com/user-attachments/assets/cb052f69-f53c-4d20-ae4f-d938cd1c7d82">

<img src="https://github.com/user-attachments/assets/ba28a9d0-477e-4df5-9fcc-3649deebce87">

**Check more on website. [https://grroxy.com](https://grroxy.com)**


# Installation

<img width="64px" src="https://github.com/user-attachments/assets/311d61f6-5645-498e-9227-5c055807c52e">

### Desktop App
Download the latest release for your platform from [Releases](https://github.com/glitchedgitz/grroxy/releases):

```bash
# note: on mac, the app may show a prompt saying it's not signed. I've applied for an Apple Developer ID — it will take some time.
# current workaround: run the command and restart the app
xattr -cr /Applications/Grroxy.app
```

### Terminal

If you prefer using grroxy without the desktop app (but why?)

```bash
go install github.com/glitchedgitz/grroxy/cmd/grroxy@latest #grroxy
go install github.com/glitchedgitz/grroxy/cmd/grroxy-app@latest #grroxy-app
go install github.com/glitchedgitz/grroxy/cmd/grroxy-tool@latest #grroxy-tool
go install github.com/glitchedgitz/cook/v2/cmd/cook@latest #cook
```

```bash
grroxy start # http://127.0.0.1:8090
```

---

# Contributing

Top 3 ways to contribute

- `Security`: Find and report security bugs/issues in grroxy.
- `Ideation`: You can help by suggesting new features or joining active discussions on [discord](https://discord.gg/J4VPhZqnUu)
- `Contributing`: By testing feature or adding new feature to code.

Check more on [CONTRIBUTING.md](./CONTRIBUTING.md)

### Frontend directory is private ([#36](https://github.com/glitchedgitz/grroxy/issues/36))

I have kept the `frontend` directory private for now. Contributing directly to the frontend wouldn't be ideal, as AI lacks good design sense and we want to maintain product quality, experience, and the direction we're moving forward in — plus the frontend is messy.
