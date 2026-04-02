package launcher

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/glitchedgitz/grroxy/internal/utils"
	"github.com/glitchedgitz/pocketbase/apis"
	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/models"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/rs/xid"
)

func (launcher *Launcher) API_ListProjects(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "GET",
		Path:   "/api/project/list",
		Handler: func(c echo.Context) error {

			collection, err := launcher.App.Dao().FindCollectionByNameOrId("_projects")

			if err != nil {
				fmt.Println("Error fetching projects:", err)
				return c.String(http.StatusInternalServerError, "Error fetching projects")
			}

			if collection == nil {
				fmt.Println("Projects collection not found")
				return c.String(http.StatusInternalServerError, "Error fetching projects")
			}

			records, err := launcher.App.Dao().FindRecordsByExpr(collection.Name)
			if err != nil {
				fmt.Println("Error fetching projects:", err)
				return c.String(http.StatusInternalServerError, "Error fetching projects")
			}

			return c.JSON(http.StatusOK, records)

		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(launcher.App),
		},
	})

	return nil
}

func (launcher *Launcher) API_CreateNewProject(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "POST",
		Path:   "/api/project/new",
		Handler: func(c echo.Context) error {

			var data struct {
				Name string `json:"name"`
			}

			if err := c.Bind(&data); err != nil {
				return c.String(http.StatusBadRequest, "Invalid request body")
			}

			if data.Name == "" || strings.TrimSpace(data.Name) == "" {
				return c.String(http.StatusBadRequest, "Project name cannot be empty or just whitespace")
			}

			projectData, err := launcher.CreateNewProject(data.Name)
			if err != nil {
				return c.String(http.StatusInternalServerError, "Error creating project")
			}

			return c.JSON(http.StatusOK, projectData)

		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(launcher.App),
		},
	})

	return nil
}

func (launcher *Launcher) API_OpenProject(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: "POST",
		Path:   "/api/project/open",
		Handler: func(c echo.Context) error {

			var data struct {
				Project string `json:"project"`
			}

			if err := c.Bind(&data); err != nil {
				return c.String(http.StatusBadRequest, "Invalid request body")
			}

			if data.Project == "" || strings.TrimSpace(data.Project) == "" {
				return c.String(http.StatusBadRequest, "Project can't be empty, send name or id")
			}

			projectData, err := launcher.OpenProjectFromNameOrId(data.Project)
			if err != nil {
				return c.String(http.StatusInternalServerError, err.Error())
			}

			return c.JSON(http.StatusOK, projectData)

		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(launcher.App),
		},
	})

	return nil
}

var ProjectState = struct {
	Active   string
	Unactive string
}{
	Active:   "active",
	Unactive: "unactive",
}

type ProjectStateData struct {
	Ip    string `json:"ip" db:"ip"`
	State string `json:"state" db:"state"`
}

// updateProjectData merges ip/state into the existing data field without overwriting other keys (e.g. templatesEnabled).
func updateProjectData(record *models.Record, ip string, state string) {
	existing := make(map[string]any)
	if raw, err := json.Marshal(record.Get("data")); err == nil {
		json.Unmarshal(raw, &existing)
	}
	existing["ip"] = ip
	existing["state"] = state
	record.Set("data", existing)
}

type ProjectData struct {
	Id      string           `json:"id" db:"id"`
	Name    string           `json:"name" db:"name"`
	Path    string           `json:"path" db:"path"`
	Data    ProjectStateData `json:"data" db:"data"`
	Version string           `json:"version" db:"version"`
}

func (launcher *Launcher) ListProjects() {
	fmt.Println("Listing projects")
	collection, err := launcher.App.Dao().FindCollectionByNameOrId("_projects")

	if err != nil {
		fmt.Println("Error fetching projects:", err)
		return
	}

	if collection == nil {
		fmt.Println("Projects collection not found")
		return
	}

	fmt.Println("Collection found")

	records, err := launcher.App.Dao().FindRecordsByExpr(collection.Name)
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

func (launcher *Launcher) CreateNewProject(projectName string) (ProjectData, error) {
	collection, err := launcher.App.Dao().FindCollectionByNameOrId("_projects")
	if err != nil {
		fmt.Println("Error fetching projects:", err)
		return ProjectData{}, err
	}

	ProjectIP, err := utils.CheckAndFindAvailablePort("127.0.0.1:8091")
	if err != nil {
		fmt.Println("Error fetching project IP:", err)
		return ProjectData{}, err
	}

	projectId := xid.New().String()
	projectPath := path.Join(launcher.Config.ProjectsDirectory, projectId)
	os.MkdirAll(projectPath, 0755)

	projectData := ProjectData{
		Id:   projectId,
		Name: projectName,
		Path: projectPath,
		Data: ProjectStateData{
			Ip:    ProjectIP,
			State: ProjectState.Active,
		},
	}

	record := models.NewRecord(collection)
	record.Set("name", projectName)
	record.Set("id", projectId)
	record.Set("path", projectPath)
	record.Set("data", projectData.Data)

	err = launcher.App.Dao().SaveRecord(record)
	if err != nil {
		fmt.Println("Error creating project:", err)
		return ProjectData{}, err
	}

	go StartProject(projectPath, ProjectIP, "127.0.0.1:8888", launcher.Config.HostAddr, func() {
		launcher.setProjectStateClose(projectId)
	})

	fmt.Println("Project created successfully")
	return projectData, nil
}

func (launcher *Launcher) setProjectStateClose(projectId string) {
	record, err := launcher.App.Dao().FindRecordById("_projects", projectId)
	if err != nil {
		fmt.Println("Error fetching project:", err)
		return
	}

	updateProjectData(record, "", ProjectState.Unactive)

	err = launcher.App.Dao().SaveRecord(record)
	if err != nil {
		fmt.Println("Error saving project state:", err)
		return
	}
}

func (launcher *Launcher) OpenProject(projectIndex int) (ProjectData, error) {
	// get list of projects
	records, err := launcher.App.Dao().FindRecordsByExpr("_projects")
	if err != nil {
		fmt.Println("Error fetching projects:", err)
		return ProjectData{}, err
	}

	_record_id := records[projectIndex].Get("id")

	record, err := launcher.App.Dao().FindRecordById("_projects", _record_id.(string))
	if err != nil {
		fmt.Println("Error fetching project:", err)
		return ProjectData{}, err
	}

	// Check if project is already running
	var existingStateData ProjectStateData
	dataInterface := record.Get("data")
	if dataInterface != nil {
		jsonData, err := json.Marshal(dataInterface)
		if err == nil {
			if err := json.Unmarshal(jsonData, &existingStateData); err == nil {
				if existingStateData.State == ProjectState.Active && existingStateData.Ip != "" {
					// Project is already running, return existing data
					return ProjectData{
						Id:   _record_id.(string),
						Name: record.Get("name").(string),
						Path: record.Get("path").(string),
						Data: existingStateData,
					}, nil
				}
			}
		}
	}

	ProjectIP, err := utils.CheckAndFindAvailablePort("127.0.0.1:8091")
	if err != nil {
		fmt.Println("Error fetching project IP:", err)
		return ProjectData{}, err
	}

	projectData := ProjectData{
		Id:   _record_id.(string),
		Name: record.Get("name").(string),
		Path: record.Get("path").(string),
		Data: ProjectStateData{
			Ip:    ProjectIP,
			State: ProjectState.Active,
		},
	}

	updateProjectData(record, ProjectIP, ProjectState.Active)

	err = launcher.App.Dao().SaveRecord(record)
	if err != nil {
		fmt.Printf("Error saving project state: %v\n", err)
		return ProjectData{}, err
	}

	projectPath := fmt.Sprintf("%v", record.Get("path"))
	if projectPath == "" {
		fmt.Println("Error: Project path is empty")
		return ProjectData{}, err
	}

	go StartProject(projectPath, ProjectIP, "127.0.0.1:8888", launcher.Config.HostAddr, func() {
		launcher.setProjectStateClose(record.Get("id").(string))
	})

	fmt.Println("Project opened successfully")
	return projectData, nil
}

func (launcher *Launcher) OpenProjectFromNameOrId(project string) (ProjectData, error) {
	record, err := launcher.App.Dao().FindFirstRecordByFilter("_projects", "name={:project} OR id={:project}", dbx.Params{
		"project": project,
	})

	if record == nil || err != nil {
		return ProjectData{}, err
	}

	// Check if project is already running
	var existingStateData ProjectStateData
	dataInterface := record.Get("data")
	if dataInterface != nil {
		jsonData, err := json.Marshal(dataInterface)
		if err == nil {
			if err := json.Unmarshal(jsonData, &existingStateData); err == nil {
				if existingStateData.State == ProjectState.Active && existingStateData.Ip != "" {
					// Project is already running, return existing data
					return ProjectData{
						Id:   record.Get("id").(string),
						Name: record.Get("name").(string),
						Path: record.Get("path").(string),
						Data: existingStateData,
					}, nil
				}
			}
		}
	}

	projectIp, err := utils.CheckAndFindAvailablePort("127.0.0.1:8091")
	if err != nil {
		fmt.Println("Error fetching project IP:", err)
		return ProjectData{}, err
	}

	projectStateData := ProjectStateData{
		Ip:    projectIp,
		State: ProjectState.Active,
	}

	projectData := ProjectData{
		Id:   record.Get("id").(string),
		Name: record.Get("name").(string),
		Path: record.Get("path").(string),
		Data: projectStateData,
	}

	updateProjectData(record, projectIp, ProjectState.Active)

	err = launcher.App.Dao().SaveRecord(record)
	if err != nil {
		fmt.Printf("Error saving project state: %v\n", err)
		return ProjectData{}, err
	}

	projectPath := fmt.Sprintf("%v", record.Get("path"))
	if projectPath == "" {
		fmt.Println("Error: Project path is empty")
		return ProjectData{}, err
	}

	go StartProject(projectPath, projectIp, "127.0.0.1:8888", launcher.Config.HostAddr, func() {
		launcher.setProjectStateClose(record.Get("id").(string))
	})

	fmt.Println("Project opened successfully")
	return projectData, nil
}

func StartProject(projectPath string, host string, proxy string, launcherAddr string, onClose func()) {
	cmd := exec.Command("grroxy-app", "-path", projectPath, "-host", host, "-proxy", proxy, "-launcher", launcherAddr, "-log")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error executing grroxy command: %v\n", err)
		return
	}

	onClose()
}

func (launcher *Launcher) ResetToolsStates(e *core.ServeEvent) error {

	records, err := launcher.App.Dao().FindRecordsByExpr("_tools")
	if err != nil {
		fmt.Println("Error fetching tools:", err)
		return err
	}

	for _, record := range records {

		record.Set("host", "")
		record.Set("state", "")

		err = launcher.App.Dao().SaveRecord(record)
		if err != nil {
			fmt.Println("Error saving tool state:", err)
			return err
		}
	}

	fmt.Println("Tools states reset successfully")
	return nil
}

func (launcher *Launcher) ResetProjectStates(e *core.ServeEvent) error {
	collection, err := launcher.App.Dao().FindCollectionByNameOrId("_projects")
	if err != nil {
		fmt.Println("Error fetching projects:", err)
		return err
	}

	records, err := launcher.App.Dao().FindRecordsByExpr(collection.Name)
	if err != nil {
		fmt.Println("Error fetching projects:", err)
		return err
	}

	for _, record := range records {
		var projectStateData ProjectStateData
		dataInterface := record.Get("data")
		if dataInterface == nil {
			continue
		}

		jsonData, err := json.Marshal(dataInterface)
		if err != nil {
			fmt.Printf("Error marshaling data for record %s: %v\n", record.Id, err)
			continue
		}

		if err := json.Unmarshal(jsonData, &projectStateData); err != nil {
			fmt.Printf("Error parsing project data for record %s: %v\n", record.Id, err)
			continue
		}

		if projectStateData.State != ProjectState.Active {
			continue
		}

		updateProjectData(record, "", ProjectState.Unactive)

		err = launcher.App.Dao().SaveRecord(record)
		if err != nil {
			fmt.Println("Error saving project state:", err)
			return err
		}
	}

	fmt.Println("Project states reset successfully")
	return nil
}
