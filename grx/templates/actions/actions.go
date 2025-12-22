package actions

// Actions:
// Perform some action after filter match, in templates

const (
	CreateLabel = "create_label" // Currently only supported one
	CreateTech  = "create_tech"

	Replace = "replace" // modify request/response
	Set     = "set"     // modify request/response
	Delete  = "delete"  // delete request/response

	SendNotification = "send_notification"

	CreatePlayground             = "create_playground"
	CreatePlaygroundWithIntruder = "create_playground_intruder"
	CreatePlaygroundWithRepeater = "create_playground_repeater"
	CreatePlaygroundWithCommand  = "create_playground_command"
)
