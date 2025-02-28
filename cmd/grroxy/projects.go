package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/pocketbase/pocketbase/models"
	"github.com/rs/xid"
)

var (
	HomeDirectory   string
	CacheDirectory  string
	ConfigDirectory string
	ProjectFile     string
)

var ProjectState = struct {
	Active   string
	Unactive string
}{
	Active:   "active",
	Unactive: "unactive",
}

type ProjectData struct {
	IP    string `json:"ip" db:"ip"`
	state string `json:"state" db:"state"`
}

func listProjects() {
	collection, err := App.Dao().FindCollectionByNameOrId("_projects")

	if err != nil {
		fmt.Println("Error fetching projects:", err)
		return
	}

	if collection == nil {
		fmt.Println("Projects collection not found")
		return
	}

	records, err := App.Dao().FindRecordsByExpr(collection.Name)
	if err != nil {
		fmt.Println("Error fetching projects:", err)
		return
	}

	fmt.Println("\nProjects:")
	for i, record := range records {
		name := fmt.Sprintf("%v", record.Get("name"))
		path := fmt.Sprintf("%v", record.Get("path"))
		fmt.Printf("%d. %-15s (%s)\n", i+1, name, path)
	}
}

func createNewProject(projectName string) {
	collection, err := App.Dao().FindCollectionByNameOrId("_projects")
	if err != nil {
		fmt.Println("Error fetching projects:", err)
		return
	}

	projectId := xid.New().String()
	projectPath := path.Join(ConfigDirectory, projectId)
	os.MkdirAll(projectPath, 0755)

	record := models.NewRecord(collection)
	record.Set("name", projectName)
	record.Set("id", projectId)
	record.Set("path", projectPath)
	record.Set("data", ProjectData{
		IP:    "127.0.0.1",
		state: ProjectState.Active,
	})

	err = App.Dao().SaveRecord(record)
	if err != nil {
		fmt.Println("Error creating project:", err)
		return
	}

	startProject(projectPath)

	fmt.Println("Project created successfully")
}

func openProject(projectIndex int) {
	collection, err := App.Dao().FindCollectionByNameOrId("_projects")
	if err != nil {
		fmt.Println("Error fetching projects:", err)
		return
	}

	// get list of projects
	records, err := App.Dao().FindRecordsByExpr(collection.Name)
	if err != nil {
		fmt.Println("Error fetching projects:", err)
		return
	}

	_record_id := records[projectIndex].Get("id")

	record, err := App.Dao().FindRecordById(collection.Name, _record_id.(string))
	if err != nil {
		fmt.Println("Error fetching project:", err)
		return
	}

	record.Set("data", ProjectData{
		IP:    "127.0.0.1",
		state: ProjectState.Active,
	})

	err = App.Dao().SaveRecord(record)
	if err != nil {
		fmt.Printf("Error saving project state: %v\n", err)
		return
	}

	projectPath := fmt.Sprintf("%v", record.Get("path"))
	if projectPath == "" {
		fmt.Println("Error: Project path is empty")
		return
	}

	startProject(projectPath)

	record.Set("data", ProjectData{
		IP:    "127.0.0.1",
		state: ProjectState.Unactive,
	})

	err = App.Dao().SaveRecord(record)
	if err != nil {
		fmt.Println("Error saving project state:", err)
		return
	}

	fmt.Println("Project opened successfully")
}

func startProject(projectPath string) {
	cmd := exec.Command("grroxy-app", projectPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error executing grroxy command: %v\n", err)
		return
	}
}
