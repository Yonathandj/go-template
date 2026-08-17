package example

// visitsKey is the Redis key behind GET /example/visits.
const visitsKey = "example:visits"

// The generated server does not enforce the schema, so the service re-states the
// limits from api/server/specs/example.yaml. Keep the two in step when you edit the spec.
const (
	maxTitleLen = 200
	maxBodyLen  = 4000

	defaultLimit = 20
	maxLimit     = 100
)
