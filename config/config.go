package config

import (
	"fmt"
	"os"
	"path"

	"github.com/glitchedgitz/grroxy-db/base"
)

type Config struct {
	HostAddr        string
	ProxyAddr        string
	HomeDirectory   string
	CWDirectory     string
	ConfigDirectory string
	CacheDirectory  string
	ProjectFile     string
	AppData         JSONData
}

func (c *Config) Initiate() {
	var err error

	// Probably not used
	c.HomeDirectory, err = os.UserHomeDir()
	base.CheckErr("", err)

	c.CacheDirectory, err = os.UserCacheDir()
	c.CacheDirectory = path.Join(c.CacheDirectory, "grroxy")
	os.MkdirAll(c.CacheDirectory, 0755)
	base.CheckErr("", err)

	c.ConfigDirectory, err = os.UserConfigDir()
	c.ConfigDirectory = path.Join(c.ConfigDirectory, "grroxy")
	os.MkdirAll(c.ConfigDirectory, 0755)
	base.CheckErr("", err)

	c.ProjectFile = path.Join(c.ConfigDirectory, "projects.json")

	c.LoadAppData()
}
func (c *Config) ShowConfig() {
	fmt.Println("Home:         ", c.HomeDirectory)
	fmt.Println("Cache:        ", c.CacheDirectory)
	fmt.Println("Config:       ", c.ConfigDirectory)
	fmt.Println("Project File: ", c.ProjectFile)
}
