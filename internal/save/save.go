package save

import (
	"github.com/projectdiscovery/gologger"

	"github.com/glitchedgitz/grroxy/internal/save/file"
	"github.com/glitchedgitz/grroxy/internal/types"
)

// const (
// 	dataWithNewLine    = "%s\n\n"
// 	dataWithoutNewLine = "%s"
// )

type OptionsLogger struct {
	OutputFolder string
	// Elastic *elastic.Options
	// Kafka   *kafka.Options
}

type Store interface {
	Save(data types.OutputData) error
}

type Logger struct {
	options    *OptionsLogger
	asyncqueue chan types.OutputData
	Store      []Store
}

// NewLogger instance
func NewLogger(options *OptionsLogger) *Logger {
	logger := &Logger{
		options:    options,
		asyncqueue: make(chan types.OutputData, 500),
	}
	// if options.Elastic.Addr != "" {
	// 	store, err := elastic.New(options.Elastic)
	// 	if err != nil {
	// 		gologger.Warning().Msgf("Error while creating elastic logger: %s", err)
	// 	} else {
	// 		logger.Store = append(logger.Store, store)
	// 	}
	// }
	// if options.Kafka.Addr != "" {
	// 	kfoptions := kafka.Options{
	// 		Addr:  options.Kafka.Addr,
	// 		Topic: options.Kafka.Topic,
	// 	}
	// 	store, err := kafka.New(&kfoptions)
	// 	if err != nil {
	// 		gologger.Warning().Msgf("Error while creating kafka logger: %s", err)
	// 	} else {
	// 		logger.Store = append(logger.Store, store)

	// 	}
	// }

	store := file.New(&file.Options{
		OutputFolder: options.OutputFolder,
	})
	// if err != nil {
	// 	gologger.Warning().Msgf("Error while creating file logger: %s", err)
	// }
	logger.Store = append(logger.Store, store)

	// if options.OutputFolder != "" {
	// 	store, err := file.New(&file.Options{
	// 		OutputFolder: options.OutputFolder,
	// 	})
	// 	if err != nil {
	// 		gologger.Warning().Msgf("Error while creating file logger: %s", err)
	// 	} else {
	// 		logger.Store = append(logger.Store, store)
	// 	}
	// }

	go logger.AsyncWrite()

	return logger
}

// AsyncWrite data
func (l *Logger) AsyncWrite() {
	for outputdata := range l.asyncqueue {
		// log.Println("Aysnc Write:")
		// if !outputdata.Userdata.OriginalRequest.HasResponse {
		// 	outputdata.PartSuffix = ".request"
		// } else if outputdata.Userdata.OriginalRequest.HasResponse {
		// 	outputdata.PartSuffix = ".response"
		// } else {
		// 	continue
		// }

		// outputdata.Name = fmt.Sprintf("%s%s-%s", outputdata.Userdata.OriginalRequest.Host, outputdata.PartSuffix, outputdata.Userdata.ID)

		// outputdata.Format = dataWithoutNewLine
		// if !strings.HasSuffix(string(outputdata.Data), "\n") {
		// 	outputdata.Format = dataWithNewLine
		// }

		// outputdata.DataString = fmt.Sprintf(outputdata.Format, outputdata.Data)

		for _, store := range l.Store {
			err := store.Save(outputdata)
			if err != nil {
				gologger.Warning().Msgf("Error while logging: %s", err)
			}
		}
	}
}

// LogRequest and user data
func (l *Logger) Save(folder string, userdata types.UserData) error {
	// log.Println("Save from printF")
	l.asyncqueue <- types.OutputData{Folder: folder, Userdata: userdata}
	return nil
}

// func isASCIICheckRequired(contentType string) bool {
// 	return stringsutils.ContainsAny(contentType, "application/octet-stream", "application/x-www-form-urlencoded")
// }

// Close logger instance
func (l *Logger) Close() {
	close(l.asyncqueue)
}
