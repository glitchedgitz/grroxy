package templates

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/glitchedgitz/filters"
	"github.com/glitchedgitz/grroxy-db/utils"
	"gopkg.in/yaml.v2"
)

type Action struct {
	Data       map[string]any `yaml:"data"`
	ActionName string         `yaml:"action_name"`
}

type Actions struct {
	Condition string                      `yaml:"condition"`
	Todo      []map[string]map[string]any `yaml:"todo"`
}

type Info struct {
	Title       string `yaml:"title,omitempty"`
	Description string `yaml:"description,omitempty"`
	Author      string `yaml:"author,omitempty"`
}

type Config struct {
	// Actions
	Type string `yaml:"type,omitempty"`

	// Mode?: By default it's 'all',
	//        Use 'any' to stop after one match
	Mode string `yaml:"mode,omitempty"`

	// Hooks: Which templates one should run
	Hooks map[string][]string `yaml:"hooks,omitempty"`

	// Default?: actions to perform when nothing match
	Default []map[string]map[string]any `yaml:"default,omitempty"`
}

type Template struct {
	Id     string `yaml:"id"`
	Info   Info   `yaml:"info"`
	Config Config `yaml:"config"`

	// Tasks: List of actions to check
	Tasks []Actions `yaml:"tasks"`
}

type Templates struct {
	Templates map[string]*Template
}

func Setup() *Templates {
	var t Templates

	t.Templates = make(map[string]*Template)

	files, err := os.ReadDir(`D:\sdks\go\src\github.com\glitchedgitz\grroxy-db\grroxy-templates`)
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("[Template.Setup]")

	for _, file := range files {
		fileName := file.Name()
		if strings.HasSuffix(fileName, ".yaml") || strings.HasPrefix(fileName, ".yml") {
			l := Read(`D:\sdks\go\src\github.com\glitchedgitz\grroxy-db\grroxy-templates\` + fileName)
			log.Printf("Template:%v Scan:%v\n", fileName, len(l.Tasks))
			log.Printf("Template:%v Mode:%v\n", fileName, l.Config.Mode)
			t.Templates[l.Id] = l
		}
	}

	return &t
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

	hooks := strings.Split(hook, ":")

	for id, template := range t.Templates {

		var actions []Action

		if len(template.Tasks) == 0 {
			log.Println("No actions found") // Fail if no actions are found
			continue
		}

		if values, found := template.Config.Hooks[hooks[0]]; !found {
			continue
		} else {
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
			check, err := filters.Filter(data, job.Condition)
			if err != nil {
				log.Printf("[Templates.Run] Filter parsing: %v", job.Condition)
				break
			}

			if check {
				log.Println("[template.Run] Found with:", job.Condition)

				for _, action := range job.Todo {
					for function, data := range action {
						actions = append(actions, getParsedValue(data, Action{
							ActionName: function,
							Data:       data,
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

		if len(actions) == 0 {
			if len(template.Config.Default) > 0 {
				log.Println("[template.Run] Using default for", data)

				for _, _action := range template.Config.Default {
					for actionName, actionData := range _action {
						a := Action{
							ActionName: actionName,
							Data:       actionData,
						}

						log.Println("Before even sending: ", _action)
						results = append(results, getParsedValue(data, a))
					}
				}
			}
		}

		results = append(results, actions...)
	}

	return results, nil
}

func ParseVariable(d *map[string]any, value string) string {

	log.Println("[ParseVariables] Using data ", *d)

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

	return value
}
