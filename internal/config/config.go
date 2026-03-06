package config

import (
	"fmt"
	"os"
	"path"

	"github.com/glitchedgitz/grroxy/internal/utils"
)

type Config struct {
	HostAddr string

	HomeDirectory     string // User's home directory
	ConfigDirectory   string // Config directory
	ProjectsDirectory string // Projects directory
	CacheDirectory    string // Cache directory
	TemplateDirectory string // Template directory

	ProjectID   string //  Active Project's ID
	CWDirectory string //  Projects Directory + ProjectID
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

	c.ConfigDirectory = path.Join(c.HomeDirectory, ".config", "grroxy")
	os.MkdirAll(c.ConfigDirectory, 0755)
}

func (c *Config) ShowConfig() {
	fmt.Println("Home:         ", c.HomeDirectory)
	fmt.Println("Cache:        ", c.CacheDirectory)
	fmt.Println("Config:       ", c.ProjectsDirectory)
}
