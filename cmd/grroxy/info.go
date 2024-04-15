package main

import "fmt"

var version = "v0.16.0"

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

var description = `
                      Beta ` + version + `
                      
    A framework to make our favourite tools work together!
                  Created by Gitesh Sharma
`

var banner = bannerLogo + description

var commandsUsage = banner + `
Usage: 
  list                       List projects
  create [project name]      Create new project
  config                     Show config     
  resume                     Resume the project where you left

Flags:
  --host                     Host address for browser app (Default: '127.0.0.1:8090')
  --proxy                    Proxy Listening on (Default: '127.0.0.1:8888')
  --no-proxy                 Disable proxy
  --no-banner                Don't print banner
  --verbose                  Print verbose logs
`
