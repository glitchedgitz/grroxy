package config

import (
	"fmt"
	"os"
	"path"

	"github.com/glitchedgitz/grroxy-db/utils"
)

type Config struct {
	HostAddr  string
	ProxyAddr string // Deprecated: Use the API to start the proxy instead

	HomeDirectory     string
	CWDirectory       string
	ConfigDirectory   string
	ProjectsDirectory string
	CacheDirectory    string
	TemplateDirectory string

	ProjectFile string
	ProjectID   string // Project ID extracted from project path
	AppData     JSONData
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

	c.ProjectsDirectory, err = os.UserConfigDir()
	c.ProjectsDirectory = path.Join(c.ProjectsDirectory, "grroxy")
	os.MkdirAll(c.ProjectsDirectory, 0755)
	utils.CheckErr("", err)

	c.ProjectFile = path.Join(c.ProjectsDirectory, "projects.json")

	// c.LoadAppData()
}

func (c *Config) ShowConfig() {
	fmt.Println("Home:         ", c.HomeDirectory)
	fmt.Println("Cache:        ", c.CacheDirectory)
	fmt.Println("Config:       ", c.ProjectsDirectory)
	fmt.Println("Project File: ", c.ProjectFile)
}
