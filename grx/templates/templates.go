package templates

import (
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/glitchedgitz/dadql/dadql"
	"github.com/glitchedgitz/grroxy/internal/utils"
	"gopkg.in/fsnotify.v1"
	"gopkg.in/yaml.v2"
)

type Action struct {
	Data       map[string]any `yaml:"data" json:"data"`
	ActionName string         `yaml:"action_name" json:"action_name"`
}

type Actions struct {
	Id        string                      `yaml:"id" json:"id"`
	Condition string                      `yaml:"condition" json:"condition"`
	Todo      []map[string]map[string]any `yaml:"todo" json:"todo"`
	Disabled  bool                        `yaml:"disabled" json:"disabled"`
}

type Info struct {
	Title       string `yaml:"title,omitempty" json:"title,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Author      string `yaml:"author,omitempty" json:"author,omitempty"`
}

type Config struct {
	// Actions
	Type string `yaml:"type,omitempty" json:"type,omitempty"`

	// Mode?: By default it's 'all',
	//        Use 'any' to stop after one match
	Mode string `yaml:"mode,omitempty" json:"mode,omitempty"`

	// Hooks: Which templates one should run
	Hooks map[string][]string `yaml:"hooks,omitempty" json:"hooks,omitempty"`
}

type Template struct {
	Id     string `yaml:"id" json:"id"`
	Info   Info   `yaml:"info" json:"info"`
	Config Config `yaml:"config" json:"config"`

	// Tasks: List of actions to check
	Tasks []Actions `yaml:"tasks" json:"tasks"`
}

type Templates struct {
	TempalteDir string
	Templates   map[string]*Template
	watcher     *fsnotify.Watcher
}

func (t *Templates) Setup() {
	t.Templates = make(map[string]*Template)
	log.Println("[Template.Setup] Initialized (loading from database)")

	// // Directory-based loading (paused — loading from DB now)
	// // Initialize the watcher
	// var err error
	// t.watcher, err = fsnotify.NewWatcher()
	// if err != nil {
	// 	log.Fatalln("Error creating watcher:", err)
	// }
	//
	// // Start watching for file changes
	// go t.watchFiles()
	//
	// files, err := os.ReadDir(t.TempalteDir)
	// if err != nil {
	// 	log.Fatalln(err)
	// }
	//
	// log.Println("[Template.Setup]")
	//
	// for _, file := range files {
	// 	fileName := file.Name()
	// 	if strings.HasSuffix(fileName, ".yaml") || strings.HasSuffix(fileName, ".yml") {
	// 		l := Read(path.Join(t.TempalteDir, fileName))
	// 		log.Printf("Template:%v Scan:%v\n", fileName, len(l.Tasks))
	// 		log.Printf("Template:%v Mode:%v\n", fileName, l.Config.Mode)
	// 		t.Templates[l.Id] = l
	// 	}
	// }
	//
	// // Add the template directory to the watcher
	// err = t.watcher.Add(t.TempalteDir)
	// if err != nil {
	// 	log.Fatalln("Error adding directory to watcher:", err)
	// }
}

// LoadTemplate adds or updates a template in the map
func (t *Templates) LoadTemplate(tmpl *Template) {
	t.Templates[tmpl.Id] = tmpl
	log.Printf("[Template.LoadTemplate] Loaded: %s (%d tasks)", tmpl.Id, len(tmpl.Tasks))
}

// RemoveTemplate removes a template from the map
func (t *Templates) RemoveTemplate(id string) {
	delete(t.Templates, id)
	log.Printf("[Template.RemoveTemplate] Removed: %s", id)
}

func (t *Templates) watchFiles() {
	for {
		select {
		case event, ok := <-t.watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				fileName := path.Base(event.Name)
				if strings.HasSuffix(fileName, ".yaml") || strings.HasSuffix(fileName, ".yml") {
					log.Printf("Template file changed: %s\n", fileName)
					l := Read(event.Name)
					t.Templates[l.Id] = l
					log.Printf("Template:%v Scan:%v\n", fileName, len(l.Tasks))
					log.Printf("Template:%v Mode:%v\n", fileName, l.Config.Mode)
				}
			}
		case err, ok := <-t.watcher.Errors:
			if !ok {
				return
			}
			log.Println("Error watching files:", err)
		}
	}
}

func (t *Templates) Close() {
	if t.watcher != nil {
		t.watcher.Close()
	}
}

func Read(filePath string) *Template {

	yamlFile, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Error reading YAML file: %v", err)
	}

	// Unmarshal yaml
	var y Template

	err = yaml.Unmarshal(yamlFile, &y)
	if err != nil {
		fmt.Println("Error:", err)
	}

	return &y
}

func getParsedValue(data map[string]any, action Action) Action {
	a := Action{
		Data:       make(map[string]any),
		ActionName: action.ActionName,
	}
	for key, str := range action.Data {
		if strVal, ok := str.(string); ok {
			parsedValue := ParseVariable(&data, strVal)
			// if err != nil {
			// 	a.Data[key] = "error"
			// } else {
			a.Data[key] = parsedValue
			// }
		} else {
			a.Data[key] = str
		}
	}
	return a
}

func (t *Templates) Run(data map[string]any, hook string) ([]Action, error) {

	results := []Action{}

	log.Println("[Templates.Run] data", data)
	log.Println("[Templates.Run] hook", hook)

	hooks := strings.Split(hook, ":")

	for id, template := range t.Templates {

		log.Println("Template: ", template.Id)

		var actions []Action
		var defaultActions []Action

		if len(template.Tasks) == 0 {
			log.Println("[Templates.Run][", template.Id, "] No actions found in the template") // Fail if no actions are found
			continue
		}

		if values, found := template.Config.Hooks[hooks[0]]; !found {
			log.Println("[Templates.Run][", template.Id, "] Hook not found", template.Config.Hooks, hooks)
			continue
		} else {
			log.Println("[Templates.Run][", template.Id, "] Hook found", template.Config.Hooks, hooks)
			foundAny := false
			for _, hook := range hooks[1:] {
				if utils.ArrayContains(values, hook) {
					foundAny = true
				}
			}
			if !foundAny {
				continue
			}
		}

		log.Printf("[Templates.Run][%s] Running template: %s", id, template.Info.Title)

		for _, job := range template.Tasks {
			if job.Disabled {
				continue
			}
			// Collect default actions separately
			if job.Id == "default" {
				for _, action := range job.Todo {
					for function, d := range action {
						defaultActions = append(defaultActions, getParsedValue(data, Action{
							ActionName: function,
							Data:       d,
						}))
					}
				}
				continue
			}

			log.Println("[Templates.Run] Tasks: jobs: ", job)

			// check condition
			check, err := dadql.Filter(data, job.Condition)
			if err != nil {
				log.Printf("[Templates.Run] Filter parsing: %v", job.Condition)
				break
			}

			log.Println("[Templates.Run] Filter: ", check)

			if check {
				log.Println("[Templates.Run] Found with:", job.Condition)

				for _, action := range job.Todo {
					for function, d := range action {
						actions = append(actions, getParsedValue(data, Action{
							ActionName: function,
							Data:       d,
						}))
					}
				}

				if template.Config.Mode == "any" {
					if len(actions) > 0 {
						break
					}
				}
			}
		}

		// If no regular actions were found, use default actions
		if len(actions) == 0 {
			actions = defaultActions
		}

		results = append(results, actions...)
	}

	return results, nil
}

func ParseTemplateActions(tasks []Actions, data map[string]any, mode string) ([]Action, error) {

	var actions []Action
	var defaultActions []Action

	for _, job := range tasks {
		if job.Disabled {
			continue
		}
		// Collect default actions separately
		if job.Id == "default" {
			for _, action := range job.Todo {
				for function, d := range action {
					defaultActions = append(defaultActions, getParsedValue(data, Action{
						ActionName: function,
						Data:       d,
					}))
				}
			}
			continue
		}

		log.Println("[Templates.Run] Tasks: jobs: ", job)

		// check condition
		check, err := dadql.Filter(data, job.Condition)
		if err != nil {
			log.Printf("[Templates.Run] Filter parsing: %v", job.Condition)
			break
		}

		log.Println("[Templates.Run] Filter: ", check)

		if check {
			log.Println("[Templates.Run] Found with:", job.Condition)

			for _, action := range job.Todo {
				for function, d := range action {
					actions = append(actions, getParsedValue(data, Action{
						ActionName: function,
						Data:       d,
					}))
				}
			}

			if mode == "any" {
				if len(actions) > 0 {
					break
				}
			}
		}
	}

	// If no regular actions were found, use default actions
	if len(actions) == 0 {
		actions = defaultActions
	}

	return actions, nil
}

func ParseVariable(d *map[string]any, value string) string {

	log.Println("[ParseVariables] Using data ", value)

	re := regexp.MustCompile(`{{(.*?)}}`)

	// Find all matches
	matches := re.FindAllStringSubmatch(value, -1)

	// Extract the captured groups
	for _, match := range matches {
		if len(match) > 1 {
			field := match[1]
			fieldValue, _ := utils.ExtractValueFromMap(d, field)
			value = strings.ReplaceAll(value, match[0], fmt.Sprint(fieldValue))
		}
	}

	log.Println("[ParseVariables] Parsed value ", value)

	return value
}
