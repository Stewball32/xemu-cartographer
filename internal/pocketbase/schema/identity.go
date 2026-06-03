package schema

// identity.go controls the registration order of the identity-domain
// collections so cross-collection RelationFields can resolve their target
// CollectionId at registration time. Plain init()-per-file registration
// would sort alphabetically, which breaks because rosters' `team` relation
// needs teams to already exist (and the two M23 collections add more
// dependencies on top).
//
// users.go updates the built-in users collection with a default_gamertag
// relation to gamertags, so gamertags must run before users. users.go keeps
// its own init() (registered later in alphabetical order by file), so we
// don't touch it here.
//
// M23c adds team_log and team_membership_requests at the end of the chain.
// Both depend on teams + users; team_log additionally relates to gamertags.
// Order matters: the helper packages that consume them (internal/teamlog,
// route handlers) lookup the collections at OnServe via FindCollectionByName,
// so registration must finish before the routes register.
//
// M08 prepends roles + user_roles. user_roles relates to both users and
// roles, so roles must run first. Both run before gamertags so the M08
// migration in users.go (which backfills isAdmin → user_roles) can find a
// live `roles` collection at runtime.
func init() {
	register(registerRolesCollection)
	register(registerUserRolesCollection)
	register(registerGamertagsCollection)
	register(registerTeamsCollection)
	register(registerRostersCollection)
	register(registerTeamLogCollection)
	register(registerTeamMembershipRequestsCollection)
}
