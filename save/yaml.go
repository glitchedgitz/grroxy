package save

import (
	"log"
	"os"
	"path"

	"gopkg.in/yaml.v2"
)

var ConfigFolder string
var IngredientsFolder string

func ReadFile(filepath string) []byte {
	log.Println("Opening Filepath: ", filepath)
	content, err := os.ReadFile(filepath)
	if err != nil {
		log.Println("Err: Reading File ", err)
		return []byte("File not saved yet")
	}
	log.Printf("Returned Filepath: %d %s", len(filepath), filepath)
	return content
}

func WriteFile(filepath string, data []byte) {
	err := os.WriteFile(filepath, data, 0644)
	if err != nil {
		log.Println("Err: Writing File ", filepath, err)
	}
}

func DeleteFile(filepath string) {
	err := os.Remove(filepath)
	if err != nil {
		log.Println("Err: Delete File ", filepath, err)
	}
}

func ReadYaml(filename string, m map[string]map[string][]string) {
	filepath := path.Join(ConfigFolder, IngredientsFolder, filename)

	content := ReadFile(filepath)

	err := yaml.Unmarshal([]byte(content), &m)
	if err != nil {
		log.Fatalf("Err: Parsing YAML %s %v", filepath, err)
	}
}

func WriteYaml(filepath string, m interface{}) {
	data, err := yaml.Marshal(&m)

	if err != nil {
		log.Fatal(err)
	}

	WriteFile(filepath, data)
}

func ReadInfoYaml(filepath string, m map[string][]string) {
	content := ReadFile(filepath)

	err := yaml.Unmarshal([]byte(content), &m)
	if err != nil {
		log.Fatalf("Err: Parsing YAML %s %v", filepath, err)
	}
}
