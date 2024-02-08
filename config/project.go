package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/glitchedgitz/grroxy-db/save"
	"github.com/olekukonko/tablewriter"
	"github.com/rs/xid"
)

type Update struct {
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Die         int    `json:"die"`
	Created     string `json:"created"`
	Version     string `json:"version"`
}

type Project struct {
	Name     string `json:"name"`
	Location string `json:"location"`
	Created  string `json:"created"`
	Updated  string `json:"updated"`
}

type JSONData struct {
	Version  string    `json:"Version"`
	Updates  []Update  `json:"Updates"`
	Projects []Project `json:"Projects"`
}

func newTable() *tablewriter.Table {
	t := tablewriter.NewWriter(os.Stdout)
	t.SetHeader([]string{"Index", "Name", "Location", "Created", "Updated"})
	t.SetAutoFormatHeaders(true)
	t.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	t.SetAlignment(tablewriter.ALIGN_LEFT)
	t.SetCenterSeparator("")
	t.SetColumnSeparator("")
	t.SetRowSeparator("")
	t.SetHeaderLine(false)
	t.SetTablePadding("\t") // pad with tabs
	t.SetNoWhiteSpace(true)
	return t
}

func (c *Config) ListProjects() {

	var n int
	var err error
	loop := 0
	total := len(c.AppData.Projects)

	for {

		start := loop * 10
		end := start + 10

		if total <= start {
			// Here we reach end
			os.Exit(0)
		} else if start < total && total < end {
			// Here we less than 10 to show
			end = total
		}

		// Show Table
		fmt.Printf("\nProjects [%d-%d]/%d", start, end, total)

		table := newTable()
		for i, project := range c.AppData.Projects[start:end] {
			table.Append([]string{fmt.Sprint(start + i), project.Name, project.Location, project.Created, project.Updated})
		}
		table.Render()
		// ==================

		loop += 1

		// Ask for input
		fmt.Print("\n(n) Enter Project Index / (enter) to Loadmore / (anything else) to close: ")

		reader := bufio.NewReader(os.Stdin)
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)
		if choice == "" {
			continue
		}
		n, err = strconv.Atoi(choice)
		if err == nil {
			break
		} else {
			os.Exit(0)
		}
	}
	c.OpenProject(n)
}

func (c *Config) NewProject(projectName string) {
	projectId := xid.New().String()
	projectPath := path.Join(c.ConfigDirectory, projectId)
	os.MkdirAll(projectPath, 0755)

	currenttime := time.Now().Format(time.DateTime)

	new := Project{
		Name:     projectName,
		Location: projectPath,
		Created:  currenttime,
		Updated:  currenttime,
	}

	c.AddProject(new)

	log.Println("Created New Project")
	log.Println("-------------------")
	log.Println("Name:      ", new.Name)
	log.Println("Location:  ", new.Location)
	log.Println("Created:   ", new.Created)
	log.Println("Updated:   ", new.Updated)

}

func (c *Config) AddProject(project Project) {
	c.AppData.Projects = append([]Project{project}, c.AppData.Projects...)
	c.SaveAppData()
	os.Chdir(project.Location)
}

func (c *Config) UpdateProject(project Project, index int) {
	c.AppData.Projects = append([]Project{project}, append(c.AppData.Projects[:index], c.AppData.Projects[index+1:]...)...)
	c.SaveAppData()
	os.Chdir(project.Location)
}

func (c *Config) OpenProject(index int) {
	currenttime := time.Now().Format(time.DateTime)
	project := c.AppData.Projects[index]
	project.Updated = currenttime
	c.UpdateProject(project, index)
}

func (c *Config) OpenCWD() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalln(err)
	}
	c.CWDirectory = cwd
	found := false
	currenttime := time.Now().Format(time.DateTime)
	for i, project := range c.AppData.Projects {
		if cwd == project.Location {
			found = true
			project.Updated = currenttime
			c.UpdateProject(project, i)
			break
		}
	}

	if !found {
		c.AddProject(Project{
			Name:     filepath.Base(cwd),
			Location: cwd,
			Created:  currenttime,
			Updated:  currenttime,
		})
	}
}

func (c *Config) SaveAppData() {
	jsonDataBytes, err := json.Marshal(c.AppData)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return
	}

	save.WriteFile(c.ProjectFile, jsonDataBytes)
}

func (c *Config) LoadAppData() {
	_, err := os.Stat(c.ProjectFile)

	if err != nil {
		if os.IsNotExist(err) {
			c.SaveAppData()
		} else {
			// An error occurred, but it's not due to the file not existing
			log.Fatalln("Error Reading projects.json", err)
		}
	} else {
		byteData := save.ReadFile(c.ProjectFile)
		if err := json.Unmarshal(byteData, &c.AppData); err != nil {
			log.Fatalln(err)
			return
		}
	}
}
