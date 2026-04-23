package rooms

import "github.com/youruser/yourproject/internal/guards"

func init() {
	register(&RoomType{
		Name:   "admin",
		Guards: []GuardFunc{guards.RequireAdmin},
	})
}
