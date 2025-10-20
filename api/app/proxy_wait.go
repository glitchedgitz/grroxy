package api

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/glitchedgitz/grroxy-db/types"
	"github.com/pocketbase/pocketbase/models"
)

// InterceptUpdate represents an update to an intercept request
type InterceptUpdate struct {
	Action       string
	IsReqEdited  bool
	IsRespEdited bool
}

func (rp *RawProxyWrapper) interceptWait(userdata *types.UserData, field string, contentLength int64) (string, bool) {
	id := userdata.ID

	originalData := field
	editedData := field + "_edited"

	dao := rp.backend.App.Dao()

	log.Printf("[InterceptWait][%s] Creating intercept record for field: %s\n", id, field)

	// Create intercept record in database
	interceptCollection, err := dao.FindCollectionByNameOrId("_intercept")
	if err != nil {
		log.Printf("[InterceptWait][%s][ERROR] Failed to find _intercept collection: %v", id, err)
		return "", false
	}

	interceptRecord := models.NewRecord(interceptCollection)
	interceptRecord.Set("id", userdata.ID)
	interceptRecord.Set("index", userdata.Index)
	interceptRecord.Set("host", userdata.Host)
	interceptRecord.Set("port", userdata.Port)
	interceptRecord.Set("req", userdata.Req)
	interceptRecord.Set("resp", userdata.Resp)
	interceptRecord.Set("has_resp", userdata.HasResp)
	interceptRecord.Set("is_req_edited", userdata.IsReqEdited)
	interceptRecord.Set("is_resp_edited", userdata.IsRespEdited)
	interceptRecord.Set("raw", userdata.ID)
	interceptRecord.Set("attached", userdata.ID)
	interceptRecord.Set("action", "")

	if err := dao.SaveRecord(interceptRecord); err != nil {
		log.Printf("[InterceptWait][%s][ERROR] Failed to save intercept record: %v", id, err)
		return "", false
	}

	log.Printf("[InterceptWait][%s] Intercept record created, waiting for action...\n", id)

	// Poll for updates (check every 100ms)
	// This replaces the realtime subscription approach
	action := ""
	isReqEdited := false
	isRespEdited := false
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(5 * time.Minute) // 5 minute timeout

	for {
		select {
		case <-timeout:
			log.Printf("[InterceptWait][%s] Timeout waiting for action, forwarding by default\n", id)
			action = "forward"
			goto processAction

		case <-ticker.C:
			// Poll the database for updates
			record, err := dao.FindRecordById("_intercept", id)
			if err != nil {
				log.Printf("[InterceptWait][%s][ERROR] Failed to find intercept record: %v", id, err)
				continue
			}

			recordAction := record.GetString("action")
			if recordAction == "forward" || recordAction == "drop" {
				action = recordAction
				isReqEdited = record.GetBool("is_req_edited")
				isRespEdited = record.GetBool("is_resp_edited")
				log.Printf("[InterceptWait][%s] Action received: %s (req_edited=%v, resp_edited=%v)\n",
					id, action, isReqEdited, isRespEdited)
				goto processAction
			}
		}
	}

processAction:
	log.Printf("[InterceptWait][%s] Processing action: %s\n", id, action)

	// Delete intercept record
	if err := dao.DeleteRecord(interceptRecord); err != nil {
		log.Printf("[InterceptWait][%s][ERROR] Failed to delete intercept record: %v", id, err)
	}

	if action == "drop" {
		userdata.Action = "drop"
		log.Printf("[InterceptWait][%s] Dropping request/response\n", id)
		return "", false
	}

	// Get updated data from _raw collection
	rawRecord, err := dao.FindRecordById("_raw", id)
	if err != nil {
		log.Printf("[InterceptWait][%s][ERROR] Failed to find _raw record: %v", id, err)
		return "", false
	}

	var updatedString string

	log.Printf("[InterceptWait][%s] Checking for edits...\n", id)

	edited := false
	if field == "req" && isReqEdited {
		edited = true
		log.Printf("[InterceptWait][%s] Request is edited\n", id)
	} else if field == "resp" && isRespEdited {
		edited = true
		log.Printf("[InterceptWait][%s] Response is edited\n", id)
	}

	if edited {
		updatedString = rawRecord.GetString(editedData)
		originalString := rawRecord.GetString(originalData)

		// Try different separators for updated string
		var updatedParts []string
		var separator string

		// Try \r\n\r\n first
		updatedParts = strings.SplitN(updatedString, "\r\n\r\n", 2)

		// If not found, try \n\n
		if len(updatedParts) != 2 {
			updatedParts = strings.SplitN(updatedString, "\n\n", 2)
			separator = "\n\n"
		} else {
			separator = "\r\n\r\n"
		}

		if len(updatedParts) == 2 {
			// Calculate body length
			updatedBodyLength := len(updatedParts[1])

			// Parse the actual Content-Length from the headers
			headers := updatedParts[0]
			actualContentLength := int64(-1)

			// Try to find Content-Length header in the updated headers
			headerLines := strings.Split(headers, "\n")
			for _, line := range headerLines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(strings.ToLower(line), "content-length:") {
					// Extract the value
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						value := strings.TrimSpace(parts[1])
						fmt.Sscanf(value, "%d", &actualContentLength)
						break
					}
				}
			}

			// If we found the Content-Length header, use it; otherwise use the passed parameter
			if actualContentLength == -1 {
				actualContentLength = contentLength
			}

			diffLength := updatedBodyLength - int(actualContentLength)

			if diffLength != 0 && actualContentLength >= 0 {
				// Update Content-Length header
				previousContentHeader := "Content-Length: " + fmt.Sprint(actualContentLength)
				newContentHeader := "Content-Length: " + fmt.Sprint(int64(updatedBodyLength))
				headers = strings.Replace(headers, previousContentHeader, newContentHeader, 1)

				previousContentHeader = "Content-Length:" + fmt.Sprint(actualContentLength)
				newContentHeader = "Content-Length:" + fmt.Sprint(int64(updatedBodyLength))
				headers = strings.Replace(headers, previousContentHeader, newContentHeader, 1)

				// Reconstruct the full request/response using the detected separator
				updatedString = headers + separator + updatedParts[1]
			}

			logstatement := ""
			logstatement += fmt.Sprintf("[passedContentLength] %d\n", contentLength)
			logstatement += fmt.Sprintf("[actualContentLength] %d\n", actualContentLength)
			logstatement += fmt.Sprintf("[newContentLength] %d\n", updatedBodyLength)
			logstatement += fmt.Sprintf("[updatedBodyLength] %d\n", updatedBodyLength)
			logstatement += fmt.Sprintf("[diffLength] %d\n", diffLength)
			logstatement += fmt.Sprintf("[separator] %q\n", separator)
			logstatement += "==============================================\n"
			logstatement += fmt.Sprintf("[originalData] %s\n", originalString)
			logstatement += "==============================================\n"
			logstatement += fmt.Sprintf("[editedData] %s\n", updatedString)
			logstatement += "==============================================\n"
			log.Println(logstatement)
		}
	} else {
		updatedString = rawRecord.GetString(originalData)
	}

	log.Printf("[InterceptWait][%s] Completed - edited=%v\n", id, edited)
	return updatedString, edited
}
