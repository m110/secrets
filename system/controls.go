package system

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/math"
	"github.com/yohamta/donburi/features/transform"
	"github.com/yohamta/donburi/filter"

	"github.com/m110/secrets/archetype"
	"github.com/m110/secrets/component"
	"github.com/m110/secrets/domain"
	"github.com/m110/secrets/engine"
)

const (
	clickMoveThreshold = 10
)

type Controls struct {
	characterQuery *donburi.Query
	dialogQuery    *donburi.Query
	poiQuery       *donburi.Query
	activePOIQuery *donburi.Query
	passageQuery   *donburi.Query
	buttonsQuery   *donburi.Query
}

func NewControls() *Controls {
	return &Controls{
		characterQuery: donburi.NewQuery(
			filter.Contains(
				component.Sprite,
				component.Velocity,
				component.Animator,
			),
		),
		dialogQuery: donburi.NewQuery(
			filter.Contains(
				component.Dialog,
			),
		),
		poiQuery: donburi.NewQuery(
			filter.Contains(
				component.POI,
			),
		),
		activePOIQuery: donburi.NewQuery(
			filter.Contains(
				component.ActivePOI,
			),
		),
		passageQuery: donburi.NewQuery(filter.Contains(component.Passage)),
		buttonsQuery: donburi.NewQuery(
			filter.Contains(
				component.Collider,
				component.DialogOption,
			),
		),
	}
}

func (c *Controls) Update(w donburi.World) {
	lvl := engine.MustFindComponent[component.LevelData](w, component.Level)
	if lvl.Changing {
		return
	}

	in := engine.MustFindComponent[component.InputData](w, component.Input)

	character, characterFound := c.characterQuery.First(w)

	if in.Disabled {
		if characterFound {
			stopCharacter(character)
		}
		return
	}

	dialog, ok := c.dialogQuery.First(w)
	if ok && component.Active.Get(dialog).Active {
		if characterFound {
			stopCharacter(character)
		}
		c.UpdateDialog(w)
		return
	}

	var justClicked bool
	var x, y int

	touchIDs := inpututil.AppendJustPressedTouchIDs(nil)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		justClicked = true
		x, y = ebiten.CursorPosition()
	} else if len(touchIDs) > 0 {
		justClicked = true
		x, y = ebiten.TouchPosition(touchIDs[0])
	}

	if justClicked {
		levelCam := engine.MustFindWithComponent(w, component.LevelCamera)
		cam := component.Camera.Get(levelCam)

		clickPos := math.Vec2{
			X: float64(x),
			Y: float64(y),
		}
		worldClickPos := clickPos.DivScalar(cam.ViewportZoom).Add(cam.ViewportPosition)
		clickRect := engine.NewRect(worldClickPos.X, worldClickPos.Y, 1, 1)

		for entry := range c.poiQuery.Iter(w) {
			poi := component.POI.Get(entry)
			if poi.POI.EdgeTrigger != nil {
				continue
			}

			pos := transform.WorldPosition(entry)
			collider := component.Collider.Get(entry)
			colliderRect := engine.NewRect(pos.X, pos.Y, collider.Width, collider.Height)

			if colliderRect.Intersects(clickRect) {
				if characterFound {
					stopCharacter(character)
				}
				archetype.SelectPOI(entry)
				return
			}
		}
	}

	if !characterFound {
		return
	}

	var movingRight, movingLeft bool
	for _, key := range in.MoveRightKeys {
		if ebiten.IsKeyPressed(key) {
			movingRight = true
			break
		}
	}

	for _, key := range in.MoveLeftKeys {
		if ebiten.IsKeyPressed(key) {
			movingLeft = true
			break
		}
	}

	var clicked bool
	touchIDs = ebiten.AppendTouchIDs(nil)
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		clicked = true
		x, _ = ebiten.CursorPosition()
	} else if len(touchIDs) > 0 {
		clicked = true
		x, _ = ebiten.TouchPosition(touchIDs[0])
	}

	levelCamera := engine.MustFindWithComponent(w, component.LevelCamera)
	cam := component.Camera.Get(levelCamera)
	screenPos := cam.WorldPositionToViewportPosition(character)

	if clicked {
		diff := float64(x) - screenPos.X
		// TODO Pivot center instead of adding width
		if diff > 0 && diff > clickMoveThreshold+70 {
			movingRight = true
		} else if diff < -clickMoveThreshold {
			movingLeft = true
		}
	}

	pos := transform.WorldPosition(character)
	velocity := component.Velocity.Get(character)
	animator := component.Animator.Get(character)
	anim := animator.Animations["walk"]
	movementBounds := component.MovementBounds.Get(character)
	sprite := component.Sprite.Get(character)

	var moving bool
	if pos.X <= movementBounds.Range.Max && movingRight {
		velocity.Velocity.X = in.MoveSpeed
		sprite.FlipY = false
		anim.Start(character)
		moving = true
	}
	if pos.X >= movementBounds.Range.Min && movingLeft {
		velocity.Velocity.X = -in.MoveSpeed
		sprite.FlipY = true
		anim.Start(character)
		moving = true
	}

	if pos.X >= movementBounds.Range.Max && movingRight {
		for p := range c.poiQuery.Iter(w) {
			edge := component.POI.Get(p).POI.EdgeTrigger
			if edge != nil && *edge == domain.EdgeRight {
				archetype.SelectPOI(p)
				moving = true
				break
			}
		}
	} else if pos.X <= movementBounds.Range.Min && movingLeft {
		for p := range c.poiQuery.Iter(w) {
			edge := component.POI.Get(p).POI.EdgeTrigger
			if edge != nil && *edge == domain.EdgeLeft {
				archetype.SelectPOI(p)
				moving = true
				break
			}
		}
	}

	if !moving {
		velocity.Velocity.X = 0
		anim.Stop(character)
	}
}

func stopCharacter(entry *donburi.Entry) {
	velocity := component.Velocity.Get(entry)
	animator := component.Animator.Get(entry)
	anim := animator.Animations["walk"]
	velocity.Velocity.X = 0
	anim.Stop(entry)
}

func (c *Controls) UpdateDialog(w donburi.World) {
	entry, ok := c.passageQuery.First(w)
	if !ok {
		return
	}

	if !isActive(entry) {
		return
	}

	passage := component.Passage.Get(entry)
	stack := engine.MustGetParent(entry)
	stackedView := component.StackedView.Get(stack)

	// Game over?
	if len(passage.Passage.Links()) == 0 {
		return
	}

	var optionUpdated bool
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		passage.ActiveOption++
		optionUpdated = true
	} else if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		passage.ActiveOption--
		optionUpdated = true
	}

	_, wy := ebiten.Wheel()
	var scroll int
	if ebiten.IsKeyPressed(ebiten.KeyPageUp) {
		scroll = 10
	} else if ebiten.IsKeyPressed(ebiten.KeyPageDown) {
		scroll = -10
	} else if wy < 0 {
		scroll = -25
	} else if wy > 0 {
		scroll = 25
	}

	var touched bool
	if scroll == 0 {
		x, y := ebiten.CursorPosition()

		touchIDs := inpututil.AppendJustPressedTouchIDs(nil)
		touched = len(touchIDs) > 0
		if touched {
			x, y = ebiten.TouchPosition(touchIDs[0])
		}
		clickRect := engine.NewRect(float64(x), float64(y), 1, 1)

		c.buttonsQuery.Each(w, func(entry *donburi.Entry) {
			dialogCamera := engine.MustFindWithComponent(w, component.DialogCamera)
			pos := transform.WorldPosition(entry).Add(transform.WorldPosition(dialogCamera))
			collider := component.Collider.Get(entry)
			colliderRect := engine.NewRect(pos.X, pos.Y, collider.Width, collider.Height)
			if colliderRect.Intersects(clickRect) {
				passage.ActiveOption = component.DialogOption.Get(entry).Index
				optionUpdated = true
			}
		})
	}

	if optionUpdated {
		if passage.ActiveOption < 0 {
			passage.ActiveOption = len(passage.Passage.Links()) - 1
		}

		if passage.ActiveOption >= len(passage.Passage.Links()) {
			passage.ActiveOption = 0
		}

		indicator := engine.MustFindWithComponent(w, component.ActiveOptionIndicator)
		c.buttonsQuery.Each(w, func(entry *donburi.Entry) {
			if component.DialogOption.Get(entry).Index == passage.ActiveOption {
				transform.ChangeParent(indicator, entry, false)
			}
		})
	}

	var next bool
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) ||
		(inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && optionUpdated) ||
		(touched && optionUpdated) {
		next = true
	}

	if next && !stackedView.Scrolled {
		archetype.NextPassage(w)
	}

	camera := component.Camera.Get(engine.MustFindWithComponent(w, component.DialogLogCamera))

	if optionUpdated || next {
		// Scroll to the bottom, so the player sees the options
		// Otherwise, the player could select an option they don't see on the screen
		camera.ViewportPosition.Y = camera.ViewportBounds.Y.Max
		stackedView.Scrolled = false
	} else if scroll != 0 {
		stackedView.Scrolled = true

		viewportPos := camera.ViewportPosition
		viewportPos.Y -= float64(scroll)

		camera.SetViewportPosition(viewportPos)

		if camera.ViewportPosition.Y >= camera.ViewportBounds.Y.Max {
			// At the bottom
			stackedView.Scrolled = false
		}
	}
}
