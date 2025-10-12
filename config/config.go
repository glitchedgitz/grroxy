package config

import (
	"fmt"
	"os"
	"path"

	"github.com/glitchedgitz/grroxy-db/utils"
)

type Config struct {
	HostAddr          string
	ProxyAddr         string // Deprecated: Use the API to start the proxy instead
	HomeDirectory     string 
	CWDirectory       string
	ConfigDirectory   string
	CacheDirectory    string
	TemplateDirectory string
	ProjectFile       string
	AppData           JSONData
}

func (c *Config) Initiate() {
	var err error

	// Probably not used
	c.HomeDirectory, err = os.UserHomeDir()
	utils.CheckErr("", err)

	c.CacheDirectory, err = os.UserCacheDir()
	c.CacheDirectory = path.Join(c.CacheDirectory, "grroxy")
	os.MkdirAll(c.CacheDirectory, 0755)
	utils.CheckErr("", err)

	c.ConfigDirectory, err = os.UserConfigDir()
	c.ConfigDirectory = path.Join(c.ConfigDirectory, "grroxy")
	os.MkdirAll(c.ConfigDirectory, 0755)
	utils.CheckErr("", err)

	c.ProjectFile = path.Join(c.ConfigDirectory, "projects.json")

	// c.LoadAppData()
}

func (c *Config) ShowConfig() {
	fmt.Println("Home:         ", c.HomeDirectory)
	fmt.Println("Cache:        ", c.CacheDirectory)
	fmt.Println("Config:       ", c.ConfigDirectory)
	fmt.Println("Project File: ", c.ProjectFile)
}
