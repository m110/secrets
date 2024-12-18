package system

import (
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/transform"
	"github.com/yohamta/donburi/filter"

	"github.com/m110/secrets/component"
)

type CameraFollow struct {
	query *donburi.Query
}

func NewCameraFollow() *CameraFollow {
	return &CameraFollow{
		query: donburi.NewQuery(
			filter.Contains(
				component.Camera,
			),
		),
	}
}

func (s *CameraFollow) Update(w donburi.World) {
	s.query.Each(w, func(entry *donburi.Entry) {
		cam := component.Camera.Get(entry)
		if cam.ViewportTarget == nil {
			return
		}

		pos := transform.WorldPosition(cam.ViewportTarget)

		viewportWorldWidth := float64(cam.Viewport.Bounds().Dx()) / cam.ViewportZoom

		targetCameraX := pos.X - viewportWorldWidth/2.0

		viewportPos := cam.ViewportPosition
		viewportPos.X = targetCameraX
		cam.SetViewportPosition(viewportPos)
	})
}
