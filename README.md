# grroxy

A cyber security toolkit blending manual testing with AI Agents. 
Focusing on product taste and user experience. Keeping it hacky and adaptable how things changes in this era, idea is to have a toolkit that prioritise how hackers work.

## Why grrrr...?
Grroxy was started 4 years back before AI was a thing, I had.. and have some pain points with the proxy tools, the initial [idea](https://x.com/glitchedgitz/status/1750176261475840215) was different and didn't workout very well, so here we are with the new [one](https://grroxy.com).

[![Website](https://img.shields.io/badge/Website-grroxy.com-blue)](https://grroxy.com) [![Discord](https://img.shields.io/badge/Discord-Join-5865F2?logo=discord&logoColor=white)](https://discord.gg/K8pGK6XatC)


<img width="1200" height="747" alt="image" src="https://github.com/user-attachments/assets/cf1d8388-f41e-47b1-bade-2206a1f561f8" />


## Installation

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
go install github.com/glitchedgitz/grroxy/cmd/grroxy@latest
go install github.com/glitchedgitz/grroxy/cmd/grroxy-app@latest
go install github.com/glitchedgitz/grroxy/cmd/grroxy-tool@latest
go install github.com/glitchedgitz/cook/v2/cmd/cook@latest
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

### Contributing to code
A separate developer interface is available at `grx/dev/` for contributors to test and build backend features ([#36](https://github.com/glitchedgitz/grroxy/issues/36)).

Use this interface to build and test backend features — the UI for accepted contributions will be added to the main frontend in subsequent releases.

```bash
cd grx/dev
npm install
npm run dev

# use sudo if you have too
```

But please refrain from creating PRs for new features without first discussing the implementation details.

### AI Contributions

Use of AI is recommended but make sure you know what code actually does.

### Frontend directory is private ([#36](https://github.com/glitchedgitz/grroxy/issues/36))

I have kept the `frontend` directory private for now. Contributing directly to the frontend wouldn't be ideal, as AI lacks good design sense and we want to maintain product quality, experience, and the direction we're moving forward in — plus the frontend is messy.
