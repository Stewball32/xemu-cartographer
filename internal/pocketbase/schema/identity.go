package schema

// identity.go is the single coordination point for schema registrations
// whose ordering matters — either because of cross-collection RelationFields
// (which must resolve their target CollectionId at registration time) or
// because of PB rule references through the M08 hasAdminRole subquery
// (which require user_roles to exist at rule-validation time).
//
// users.go updates the built-in users collection with a default_gamertag
// relation to gamertags, so gamertags must run before users. users.go keeps
// its own init() (registered later in alphabetical order by file), so we
// don't touch it here.
//
// containers.go and game_events.go don't reference any other M22/M23/M08
// collection in their rules or fields, so they keep their own init()s.
//
// Registration phases (in order they run from this init()):
//
//  1. roles + user_roles — the M08 join. user_roles relates to both users
//     and roles, so roles registers first. Both must come before anything
//     that references @collection.user_roles in a PB rule.
//
//  2. Rule-only consumers of user_roles — schemas whose rules embed the
//     hasAdminRole subquery but whose fields don't FK into user_roles.
//     Order among these doesn't matter; kept alphabetical for readability.
//     Pre-M08, these registered via their own init()s (sorting before
//     identity.go), but moving them in here is required for 8c since their
//     rules now reference user_roles.
//
//  3. Identity FK chain — gamertags → teams → rosters → the M23 add-ons.
//     rosters needs teams; team_log and team_membership_requests need
//     teams + users; team_log also relates to gamertags. The order here
//     was already correct pre-M08; the chain prefix changed for M08.
//
// The helper packages that consume these (internal/teamlog,
// internal/notifications, route handlers) look up collections at OnServe
// via FindCollectionByName, so all of this needs to be settled before
// route registration runs.
func init() {
	// 1. M08 roles join — load-bearing for every rule that follows.
	register(registerRolesCollection)
	register(registerUserRolesCollection)

	// 2. Rule-only consumers of user_roles.
	register(registerAuditLogCollection)
	register(registerCapturePoliciesCollection)
	register(registerNotificationsCollection)
	register(registerReservedNamesCollection)

	// 3. Identity FK chain.
	register(registerGamertagsCollection)
	register(registerTeamsCollection)
	register(registerRostersCollection)
	register(registerTeamLogCollection)
	register(registerTeamMembershipRequestsCollection)
}
