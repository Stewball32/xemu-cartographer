package rooms

func init() {
	register(&RoomType{
		Name:   "public",
		Guards: nil,
	})
}
