package gemini_responses

// -------------- //
// Message Roles //
// -------------//

type Role string

const (
	RoleUser   Role = "user"
	RoleModel  Role = "model"
	RoleSystem Role = "system"
)

// --------------------- //
// End Of Message Roles //
// ------------------- //
