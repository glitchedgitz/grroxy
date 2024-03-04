package main

import "fmt"

var version = "v0.15.5 // EARLY ACCESS"

var bannerLogo = `
  ____________________________ ________  ____  ________.___.
 /  _____/\______   \______   \\_____  \ \   \/  /\__  |   |
/   \  ___ |       _/|       _/ /   |   \ \     /  /   |   |
\    \_\  \|    |   \|    |   \/    |    \/     \  \____   |
 \______  /|____|_  /|____|_  /\_______  /___/\  \ / ______|
        \/        \/        \/         \/      \_/ \/
`

func printCenter(text string, width int) string {
	padding := (width - len(text)) / 2
	return fmt.Sprintf("%*s%s\n", padding+len(text), "", text)
}

var description = printCenter(version, 16) + `
    A framework to make our favourite tools work together!
              Created by Gitz ¬ Gitesh Sharma
`

var banner = bannerLogo + description

var commandsUsage = banner + `
Usage: 
  list                       List projects
  create [ProjectName]       Create new project
  config                     Show config     

Flags:
  --no-proxy                 Disable proxy
  --no-banner                Don't print banner
`
