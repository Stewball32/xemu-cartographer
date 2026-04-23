package rooms

import "github.com/Stewball32/xemu-cartographer/internal/guards"

func init() {
	register(&RoomType{
		Name:   "admin",
		Guards: []GuardFunc{guards.RequireAdmin},
	})
}
