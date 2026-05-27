package schema

// identity.go controls the registration order of the three identity-domain
// collections (gamertags, teams, rosters) so cross-collection RelationFields
// can resolve their target CollectionId at registration time. Plain
// init()-per-file registration would sort alphabetically:
// gamertags → rosters → teams → users, which breaks because rosters'
// `team` relation needs teams to already exist.
//
// users.go updates the built-in users collection with a default_gamertag
// relation to gamertags, so gamertags must run before users. users.go keeps
// its own init() (registered later in alphabetical order by file), so we
// don't touch it here.
func init() {
	register(registerGamertagsCollection)
	register(registerTeamsCollection)
	register(registerRostersCollection)
}
