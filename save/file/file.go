package file

import (
	"log"
	"os"
	"path"
	"strings"

	"github.com/glitchedgitz/grroxy-db/types"
)

// Options required for file export
type Options struct {
	// OutputFolder is the folder where the logs will be stored
	OutputFolder string `yaml:"output-folder"`
}

// Client type for file based logging
type Client struct {
	options *Options
}

// New creates and returns a new client for file based logging
func New(option *Options) *Client {
	return &Client{
		options: &Options{
			OutputFolder: option.OutputFolder,
		},
	}
}

// Store writes the log to the file
func (c *Client) Save(data types.OutputData) error {
	// generate the file destination file name
	// destFile := path.Join(c.options.OutputFolder, fmt.Sprintf("%s.%s", data.Name, "txt"))

	// var hostFolder =
	// var port = ""

	// if strings.Contains(hostFolder, ":") {
	// 	t := strings.Split(data.Userdata.Host, ":")
	// 	hostFolder = t[0]
	// 	port = t[1]
	// }

	var destFolder = path.Join(c.options.OutputFolder, "targets", data.Userdata.Host, data.Userdata.Port, data.Folder)

	os.MkdirAll(destFolder, 0755)
	destFile := path.Join(destFolder, data.Userdata.ID)

	log.Println("saving to disk: ", destFile)

	f, err := os.OpenFile(destFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	// write to file

	// Need to uncomment later
	// fmt.Fprint(f, data.Userdata.Event.Data)
	return f.Close()
}

func NewHost(host string) {
	t := strings.Split(host, ":")
	hostFolder := t[0]
	port := t[1]

	os.MkdirAll(path.Join("targets", hostFolder, port), 0644)
	os.Mkdir(path.Join("targets", hostFolder, port, "req"), 0644)
	os.Mkdir(path.Join("targets", hostFolder, port, "resp"), 0644)
	os.Mkdir(path.Join("targets", hostFolder, port, "req_edited"), 0644)
	os.Mkdir(path.Join("targets", hostFolder, port, "resp_edited"), 0644)
}
