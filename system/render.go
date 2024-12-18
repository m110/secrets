package system

import (
	"fmt"
	"image/color"
	stdmath "math"
	"sort"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/transform"
	"github.com/yohamta/donburi/filter"
	"golang.org/x/image/colornames"

	"github.com/m110/secrets/archetype"
	"github.com/m110/secrets/assets"
	"github.com/m110/secrets/component"
	"github.com/m110/secrets/domain"
)

type Render struct {
	camerasQuery *donburi.OrderedQuery[component.CameraData]

	offscreen *ebiten.Image
}

func NewRender() *Render {
	return &Render{
		camerasQuery: donburi.NewOrderedQuery[component.CameraData](
			filter.Contains(
				component.Camera,
			),
		),
	}
}

func (r *Render) Init(w donburi.World) {
	game := component.MustFindGame(w)
	width := game.Dimensions.ScreenWidth
	height := game.Dimensions.ScreenHeight
	r.offscreen = ebiten.NewImage(width, height)
}

func (r *Render) Draw(w donburi.World, screen *ebiten.Image) {
	game := component.MustFindGame(w)
	width := game.Dimensions.ScreenWidth
	height := game.Dimensions.ScreenHeight
	bounds := r.offscreen.Bounds()
	if width != bounds.Dx() || height != bounds.Dy() {
		// Window size changed
		r.offscreen = ebiten.NewImage(width, height)
	} else {
		r.offscreen.Clear()
	}

	count := 0
	r.camerasQuery.EachOrdered(w, component.Camera, func(entry *donburi.Entry) {
		if !entry.HasComponent(component.InnerCamera) {
			count += r.renderCamera(entry, r.offscreen, game)
		}
	})

	screen.DrawImage(r.offscreen, nil)

	if game.Debug.Enabled {
		debugX := 280
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("FPS: %v", int(ebiten.ActualFPS())), debugX, 20)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("TPS: %v", int(ebiten.ActualTPS())), debugX, 40)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("World entities: %v", w.Len()), debugX, 80)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Rendered entities: %v", count), debugX, 100)
	}
}

func (r *Render) renderCamera(entry *donburi.Entry, offscreen *ebiten.Image, game *component.GameData) int {
	if entry.HasComponent(component.Active) {
		if !component.Active.Get(entry).Active {
			return 0
		}
	}

	camera := component.Camera.Get(entry)

	if !camera.Root.HasComponent(component.Layer) {
		panic("missing root layer")
	}

	rootLayer := component.Layer.Get(camera.Root).Layer
	children := r.getAllChildren(camera.Root, rootLayer, game)

	byLayer := map[int][]entryWithLayer{}
	for _, child := range children {
		byLayer[int(child.layer)] = append(byLayer[int(child.layer)], child)
	}

	var layers []int
	for l := range byLayer {
		layers = append(layers, l)
	}

	sort.Ints(layers)

	camera.Viewport.Clear()

	count := 0
	for _, layer := range layers {
		for _, child := range byLayer[layer] {
			count++

			if child.entry.HasComponent(component.Sprite) {
				renderSprite(child.entry, camera)
			}

			if child.entry.HasComponent(component.Text) {
				renderText(child.entry, camera)
			}

			if child.entry.HasComponent(component.Camera) {
				count += r.renderCamera(child.entry, camera.Viewport, game)
			}

			if game.Debug.Enabled {
				if child.entry.HasComponent(component.Bounds) {
					renderBoundsDebug(child.entry, camera)
				}

				if child.entry.HasComponent(component.Collider) {
					renderColliderDebug(child.entry, camera)
				}

				if child.entry.HasComponent(component.Tag) {
					renderTagDebug(child.entry, camera)
				}
			}
		}
	}

	if camera.Mask != nil {
		op := &ebiten.DrawImageOptions{}
		op.Blend = ebiten.BlendDestinationIn
		op.Filter = ebiten.FilterLinear
		camera.Viewport.DrawImage(camera.Mask, op)
	}

	if camera.TransitionOverlay != nil {
		op := &ebiten.DrawImageOptions{}
		op.ColorScale.Scale(1, 1, 1, float32(camera.TransitionAlpha))
		op.Filter = ebiten.FilterLinear
		camera.Viewport.DrawImage(camera.TransitionOverlay, op)
	}

	cameraPos := transform.WorldPosition(entry)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(cameraPos.X, cameraPos.Y)
	op.Filter = ebiten.FilterLinear

	if camera.AlphaOverride != nil {
		op.ColorM.Scale(1.0, 1.0, 1.0, camera.AlphaOverride.A)
	}
	if camera.ColorOverride != nil {
		op.ColorM.Translate(camera.ColorOverride.R, camera.ColorOverride.G, camera.ColorOverride.B, 0)
	}

	offscreen.DrawImage(camera.Viewport, op)

	if game.Debug.Enabled {
		renderCameraDebug(entry, offscreen)
	}

	return count
}

func renderCameraDebug(entry *donburi.Entry, offscreen *ebiten.Image) {
	pos := transform.WorldPosition(entry)
	camera := component.Camera.Get(entry)
	bounds := camera.Viewport.Bounds()
	vector.StrokeRect(offscreen, float32(pos.X), float32(pos.Y), float32(bounds.Dx()), float32(bounds.Dy()), 1, colornames.Red, false)
}

func renderBoundsDebug(entry *donburi.Entry, camera *component.CameraData) {
	bounds := component.Bounds.Get(entry)
	pos := camera.WorldPositionToViewportPosition(entry)
	zoom := camera.ViewportZoom
	w := bounds.Width * zoom
	h := bounds.Height * zoom
	vector.StrokeRect(camera.Viewport, float32(pos.X), float32(pos.Y), float32(w), float32(h), 1, colornames.Magenta, false)
}

func renderTagDebug(entry *donburi.Entry, camera *component.CameraData) {
	tag := component.Tag.Get(entry)
	pos := camera.WorldPositionToViewportPosition(entry)
	op := &text.DrawOptions{}
	op.ColorScale.ScaleWithColor(colornames.Magenta)
	op.GeoM.Translate(pos.X, pos.Y)
	text.Draw(camera.Viewport, tag.Tag, assets.SmallFont, op)
}

func renderColliderDebug(entry *donburi.Entry, camera *component.CameraData) {
	collider := component.Collider.Get(entry)
	pos := camera.WorldPositionToViewportPosition(entry)
	zoom := camera.ViewportZoom

	rect := collider.Rect.Move(pos)
	w := rect.Width * zoom
	h := rect.Height * zoom

	vector.StrokeRect(camera.Viewport, float32(rect.X), float32(rect.Y), float32(w), float32(h), 1, colornames.Lime, false)
}

func (r *Render) getAllChildren(entry *donburi.Entry, rootLayer domain.LayerID, game *component.GameData) []entryWithLayer {
	if !entry.Valid() || !isActive(entry) {
		return nil
	}

	result := make([]entryWithLayer, 0, 32)
	seen := make(map[*donburi.Entry]bool)

	stack := []*donburi.Entry{entry}

	for len(stack) > 0 {
		e := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if !e.Valid() || !isActive(e) || seen[e] {
			continue
		}

		seen[e] = true

		if e.HasComponent(component.Sprite) || e.HasComponent(component.Text) || e.HasComponent(component.Camera) {
			result = append(result, getEntryWithLayer(e, rootLayer))
		}

		if game.Debug.Enabled {
			if e.HasComponent(component.Collider) || e.HasComponent(component.Bounds) {
				result = append(result, getEntryWithLayer(e, rootLayer))
			}
		}

		children, ok := transform.GetChildren(e)
		if ok {
			stack = append(stack, children...)
		}
	}

	return result
}

func getEntryWithLayer(entry *donburi.Entry, rootLayer domain.LayerID) entryWithLayer {
	if !entry.HasComponent(component.Layer) {
		return entryWithLayer{
			entry: entry,
			layer: rootLayer,
		}
	}

	layer := component.Layer.Get(entry)
	return entryWithLayer{
		entry: entry,
		layer: layer.Layer,
	}
}

func renderSprite(entry *donburi.Entry, camera *component.CameraData) {
	sprite := component.Sprite.Get(entry)

	if sprite.Image == nil {
		panic(fmt.Sprintf("sprite image is nil: %s", entry))
	}

	if sprite.Hidden {
		return
	}

	bounds := sprite.Image.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	halfW, halfH := float64(width)/2, float64(height)/2

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-halfW, -halfH)
	op.GeoM.Rotate(float64(int(transform.WorldRotation(entry)-sprite.OriginalRotation)%360) * 2 * stdmath.Pi / 360)
	op.GeoM.Translate(halfW, halfH)

	if sprite.FlipY {
		op.GeoM.Translate(-halfW, 0)
		op.GeoM.Scale(-1, 1)
		op.GeoM.Translate(halfW, 0)
	}

	position := camera.WorldPositionToViewportPosition(entry)
	x := position.X
	y := position.Y

	scale := transform.WorldScale(entry)

	switch sprite.Pivot {
	case component.SpritePivotCenter:
		x -= halfW * scale.X
		y -= halfH * scale.Y
	}

	op.GeoM.Scale(scale.X, scale.Y)

	if sprite.ColorOverride != nil {
		op.ColorM.Translate(sprite.ColorOverride.R, sprite.ColorOverride.G, sprite.ColorOverride.B, 0)
	}
	if sprite.ColorBlendOverride != nil {
		blendMonochrome(op, sprite.ColorBlendOverride.Value)
	}
	if sprite.AlphaOverride != nil {
		op.ColorM.Scale(1.0, 1.0, 1.0, sprite.AlphaOverride.A)
	}

	op.GeoM.Scale(camera.ViewportZoom, camera.ViewportZoom)
	op.GeoM.Translate(x, y)
	op.Filter = ebiten.FilterLinear

	camera.Viewport.DrawImage(sprite.Image, op)
}

func renderText(entry *donburi.Entry, camera *component.CameraData) {
	t := component.Text.Get(entry)

	if t.Hidden {
		return
	}

	font := archetype.FontFromSize(t.Size)

	pos := camera.WorldPositionToViewportPosition(entry)

	var col color.Color = assets.TextColor
	if t.Color != nil {
		col = t.Color
	}

	length := utf8.RuneCountInString(t.Text)
	if t.Streaming {
		length = int(float64(length) * t.StreamingTimer.PercentDone())
	}

	textToDraw := t.Text[:length]

	op := &text.DrawOptions{}
	op.LineSpacing = font.Size
	op.PrimaryAlign = t.Align
	op.GeoM.Scale(camera.ViewportZoom, camera.ViewportZoom)
	op.GeoM.Translate(pos.X, pos.Y)
	op.ColorScale.ScaleWithColor(col)

	if t.AlphaOverride != nil {
		op.ColorScale.ScaleAlpha(float32(t.AlphaOverride.A))
	} else {

	}

	text.Draw(camera.Viewport, textToDraw, font, op)
}

func isActive(entry *donburi.Entry) bool {
	if entry.HasComponent(component.Active) && !component.Active.Get(entry).Active {
		return false
	}

	return true
}

type entryWithLayer struct {
	entry *donburi.Entry
	layer domain.LayerID
}

func blendMonochrome(op *ebiten.DrawImageOptions, blend float64) {
	if blend < 0 {
		blend = 0
	}
	if blend > 1 {
		blend = 1
	}

	// Standard luminance coefficients
	rCoeff := 0.299
	gCoeff := 0.587
	bCoeff := 0.114

	cm := ebiten.ColorM{}

	// Monochrome matrix components
	monoR := rCoeff * (1 - blend)
	monoG := gCoeff * (1 - blend)
	monoB := bCoeff * (1 - blend)

	// Color preservation component
	colorComponent := blend

	cm.Reset()
	// Red output
	cm.SetElement(0, 0, monoR+colorComponent)
	cm.SetElement(0, 1, monoG)
	cm.SetElement(0, 2, monoB)

	// Green output
	cm.SetElement(1, 0, monoR)
	cm.SetElement(1, 1, monoG+colorComponent)
	cm.SetElement(1, 2, monoB)

	// Blue output
	cm.SetElement(2, 0, monoR)
	cm.SetElement(2, 1, monoG)
	cm.SetElement(2, 2, monoB+colorComponent)

	op.ColorM = cm
}
