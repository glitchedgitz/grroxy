package app

import (
	"fmt"
	"log"
	"strings"

	"github.com/glitchedgitz/pocketbase/models"
)

// InterceptUpdate represents an update to an intercept request
type InterceptUpdate struct {
	Action        string
	IsReqEdited   bool
	IsRespEdited  bool
	ReqEditedRaw  string // Raw HTTP request string (if edited)
	RespEditedRaw string // Raw HTTP response string (if edited)
}

// interceptWait handles request/response interception by:
// 1. Creating an intercept record in the database for the frontend to display
// 2. Blocking until the user takes action (forward/drop/edit)
// 3. Returning the appropriate data (original or edited)
//
// Parameters:
//   - userdata: Request/response metadata
//   - field: "req" or "resp" to indicate which is being intercepted
//   - contentLength: Original Content-Length header value
//   - rawData: The raw HTTP request/response string (already in memory - no DB fetch needed!)
func (rp *RawProxyWrapper) interceptWait(userdata map[string]any, field string, contentLength int64, rawData string) (string, bool) {
	id := userdata["id"].(string)

	dao := rp.backend.App.Dao()

	log.Printf("[InterceptWait][%s] Creating intercept record for field: %s\n", id, field)

	// Create intercept record in database
	interceptRecord := models.NewRecord(rp.interceptCollection)
	interceptRecord.Load(userdata)

	if err := dao.SaveRecord(interceptRecord); err != nil {
		log.Printf("[InterceptWait][%s][ERROR] Failed to save intercept record: %v", id, err)
		return "", false
	}

	log.Printf("[InterceptWait][%s] Intercept record created, waiting for action...\n", id)
	log.Printf("[InterceptWait][%s] ========== INTERCEPT CREATED ==========", id)
	log.Printf("[InterceptWait][%s] Field: %s", id, field)
	log.Printf("[InterceptWait][%s] has_resp: %v", id, userdata["has_resp"].(bool))
	log.Printf("[InterceptWait][%s] req_json: %+v", id, userdata["req_json"])
	log.Printf("[InterceptWait][%s] resp_json: %+v", id, userdata["resp_json"])
	log.Printf("[InterceptWait][%s] =======================================", id)

	// Create a channel for this intercept and register it
	updateChan := make(chan InterceptUpdate, 1)
	RegisterInterceptChannel(id, updateChan)
	defer UnregisterInterceptChannel(id)

	// Wait for update notification from the API endpoint
	log.Printf("[InterceptWait][%s] Waiting for action from API endpoint...\n", id)
	update := <-updateChan

	action := update.Action
	isReqEdited := update.IsReqEdited
	isRespEdited := update.IsRespEdited
	reqEditedRaw := update.ReqEditedRaw
	respEditedRaw := update.RespEditedRaw

	log.Printf("[InterceptWait][%s] Action received: %s (req_edited=%v, resp_edited=%v)\n",
		id, action, isReqEdited, isRespEdited)

	log.Printf("[InterceptWait][%s] Processing action: %s\n", id, action)

	// Load the intercept record to delete it
	// interceptRecord, err := dao.FindRecordById("_intercept", id)
	// if err != nil {
	// 	log.Printf("[InterceptWait][%s][ERROR] Failed to find intercept record: %v", id, err)
	// 	return "", false
	// }

	// Delete intercept record
	defer func() {
		if err := dao.DeleteRecord(interceptRecord); err != nil {
			log.Printf("[InterceptWait][%s][ERROR] Failed to delete intercept record: %v", id, err)
		}
	}()

	if action == "drop" {
		userdata["action"] = "drop"
		log.Printf("[InterceptWait][%s] Dropping request/response\n", id)
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
		// Get the raw edited strings directly from the channel update
		if field == "req" && reqEditedRaw != "" {
			updatedString = reqEditedRaw
			userdata["is_req_edited"] = true
			log.Printf("[InterceptWait][%s] Using edited request data from channel", id)
		} else if field == "resp" && respEditedRaw != "" {
			updatedString = respEditedRaw
			userdata["is_resp_edited"] = true
			log.Printf("[InterceptWait][%s] Using edited response data from channel", id)
		} else {
			log.Printf("[InterceptWait][%s][WARN] Marked as edited but no raw string received", id)
			edited = false
		}

		if edited && updatedString != "" {
			log.Printf("[InterceptWait][%s] ========== EDITED %s DATA ==========", id, strings.ToUpper(field))
			log.Printf("[InterceptWait][%s] Raw length: %d bytes", id, len(updatedString))
			log.Printf("[InterceptWait][%s] Raw content:\n%s", id, updatedString)
			log.Printf("[InterceptWait][%s] =============================================", id)
		}
	}

	// If not edited or fallback, use the original raw data passed as parameter
	// No need to fetch from database - we already have it in memory!
	if !edited || updatedString == "" {
		updatedString = rawData
		log.Printf("[InterceptWait][%s] Using original %s data (from memory)\n", id, field)
		log.Printf("[InterceptWait][%s] ========== ORIGINAL %s DATA ==========", id, strings.ToUpper(field))
		log.Printf("[InterceptWait][%s] Raw length: %d bytes", id, len(updatedString))
		log.Printf("[InterceptWait][%s] Raw content:\n%s", id, updatedString)
		log.Printf("[InterceptWait][%s] =============================================", id)
	}

	// Process edited data: update Content-Length if needed
	if edited && updatedString != "" {
		// Use the original raw data passed as parameter for comparison
		originalString := rawData

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
	}

	log.Printf("[InterceptWait][%s] Completed - edited=%v\n", id, edited)
	log.Printf("[InterceptWait][%s] ========== FINAL OUTPUT ==========", id)
	log.Printf("[InterceptWait][%s] Field: %s, Edited: %v", id, field, edited)
	log.Printf("[InterceptWait][%s] Returning %d bytes", id, len(updatedString))
	log.Printf("[InterceptWait][%s] Final content:\n%s", id, updatedString)
	log.Printf("[InterceptWait][%s] ===================================", id)
	return updatedString, edited
}
