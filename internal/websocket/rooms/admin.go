package rooms

// The admin room is joinable by a principal holding the admin.admin scope
// (the seeded admin role) or a superuser — decided by authz.Can(room.join)
// in the join_room handler, so Guards stays nil.
func init() {
	register(&RoomType{
		Name:   "admin",
		Guards: nil,
	})
}
