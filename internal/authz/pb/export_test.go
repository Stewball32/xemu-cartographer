package pb

// Test-only exports. The pb tests live in package pb_test because pbtest
// imports pb (an internal test package would cycle).
var (
	ClassifyToken = classifyToken
	UserUnusable  = userUnusable
	TouchLastUsed = (*PBDeps).touchLastUsed
	WithApp       = (*PBDeps).withApp
)

// PrincipalKey is the request-store key the middleware stores under.
const PrincipalKey = principalKey
