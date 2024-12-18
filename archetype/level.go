package archetype

import (
	math2 "math"
	"time"

	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/math"
	"github.com/yohamta/donburi/features/transform"

	"github.com/m110/secrets/assets"
	"github.com/m110/secrets/component"
	"github.com/m110/secrets/domain"
	"github.com/m110/secrets/engine"
)

const (
	levelMovementMargin               = 100
	levelCameraMarginPercent          = 0.2
	scrollingLevelCameraMarginPercent = 0.45

	levelTransitionDuration = 500 * time.Millisecond

	backgroundFadeDuration   = 1000 * time.Millisecond
	backgroundScrollSpeed    = 1
	backgroundScrollDistance = 100

	// TODO this is not generic
	nightFact = "night"
)

func NewLevel(w donburi.World, targetLevel domain.TargetLevel) {
	level, ok := assets.Assets.Levels[targetLevel.Name]
	if !ok {
		panic("Name not found: " + targetLevel.Name)
	}

	background := level.Background()

	entry := NewTagged(w, "Level").
		WithLayer(domain.SpriteLayerBackground).
		WithSprite(component.SpriteData{
			Image: background,
		}).
		With(component.Level).
		With(component.Animator).
		Entry()

	component.Level.SetValue(entry, component.LevelData{
		Name: targetLevel.Name,
	})

	game := component.MustFindGame(w)
	// TODO Move out
	if level.Outdoor && game.Story.Fact(nightFact) {
		NewTagged(w, "NightOverlay").
			WithParent(entry).
			WithLayerInherit().
			WithSprite(component.SpriteData{
				Image: assets.Assets.NightOverlay,
			}).
			Entry()
	}

	spawned := false

	levelCam := engine.MustFindWithComponent(w, component.LevelCamera)
	cam := component.Camera.Get(levelCam)
	input := engine.MustFindComponent[component.InputData](w, component.Input)

	anim := component.Animator.Get(entry)

	anim.SetAnimation("transition", &component.Animation{
		Active: true,
		Timer:  engine.NewTimer(levelTransitionDuration),
		OnStart: func(e *donburi.Entry) {
			input.Disabled = true
		},
		Update: func(e *donburi.Entry, a *component.Animation) {
			if spawned {
				cam.TransitionAlpha = a.Timer.PercentDone()
				if a.Timer.IsReady() {
					a.Stop(entry)
				}
			} else {
				cam.TransitionAlpha = 1 - a.Timer.PercentDone()
				if a.Timer.IsReady() {
					spawned = true
					input.Disabled = false
					a.Stop(entry)
				}
			}
		},
	})

	passage, ok := game.Story.PassageForLevel(targetLevel)
	if ok && passage.ConditionsMet() {
		passage.Visit()
		ShowPassage(w, passage, nil)
	}

	for _, o := range level.Objects {
		NewObject(entry, o)
	}

	for _, poi := range level.POIs {
		NewPOI(entry, poi)
	}

	var character *donburi.Entry

	var characterPos *domain.CharacterPosition
	if len(level.Entrypoints) > 0 && targetLevel.Entrypoint != nil {
		entrypoint := level.Entrypoints[*targetLevel.Entrypoint]
		characterPos = &entrypoint.CharacterPosition

		// For now, all levels have only one Y position for the character
		// For convenience, the first entrypoint's Y position is used
		characterPos.LocalPosition.Y = level.Character.PosY
	} else if targetLevel.CharacterPosition != nil {
		// If coming back to the previous level, use the previous character position
		characterPos = targetLevel.CharacterPosition
	}

	if characterPos != nil {
		// Default to the background boundaries
		boundsRange := engine.FloatRange{
			Min: levelMovementMargin,
			Max: float64(background.Bounds().Dx() - levelMovementMargin),
		}

		if level.Limits != nil {
			boundsRange = *level.Limits
		}

		bounds := component.MovementBoundsData{
			Range: boundsRange,
		}

		character = NewCharacter(entry, level.Character.Scale, bounds)

		transform.GetTransform(character).LocalPosition = characterPos.LocalPosition
		component.Sprite.Get(character).FlipY = characterPos.FlipY
	}

	cam.Root = entry

	if level.CameraZoom != 0 {
		cam.ViewportZoom = level.CameraZoom
	} else {
		// Calculate zoom to fit height with margins
		marginPercent := 0.01
		screenHeight := float64(game.Dimensions.ScreenHeight)
		bgHeight := float64(background.Bounds().Dy())

		totalMarginHeight := screenHeight * marginPercent * 2
		availableHeight := screenHeight - totalMarginHeight

		cam.ViewportZoom = availableHeight / bgHeight
	}

	// Multiply by zoom to go from world space to screen space
	// Divide by zoom to go from screen space to world space
	heightDiff := (float64(game.Dimensions.ScreenHeight) - float64(background.Bounds().Dy())*cam.ViewportZoom) / cam.ViewportZoom
	if heightDiff > 0 {
		cam.ViewportPosition.Y = -heightDiff / 2
	} else {
		// Should not happen?
		cam.ViewportPosition.Y = 0
	}

	bounds := component.Sprite.Get(entry).Image.Bounds()
	levelWidth := float64(bounds.Dx())

	screenWidth := float64(game.Dimensions.ScreenWidth)
	screenWorldWidth := screenWidth / cam.ViewportZoom
	viewportWorldWidth := float64(cam.Viewport.Bounds().Dx()) / cam.ViewportZoom

	if character == nil {
		// No character - make the background scroll horizontally
		targetPos := math.Vec2{
			X: levelWidth / 2.0,
			Y: cam.ViewportPosition.Y,
		}

		target := NewTagged(w, "ViewportTarget").
			WithParent(entry).
			WithPosition(targetPos).
			With(component.Velocity).
			WithBounds(engine.Size{
				Width:  50,
				Height: 50,
			}).
			Entry()

		component.Velocity.Get(target).Velocity = math.Vec2{
			X: backgroundScrollSpeed,
		}

		cam.ViewportTarget = target

		maxX := levelWidth
		if level.Fadepoint != nil {
			maxX = level.Fadepoint.X
		}

		// TODO Review these calculations, don't work well on smaller displays
		cam.ViewportBounds.X = &engine.FloatRange{
			Min: float64(-scrollingLevelCameraMargin(w)),
			Max: maxX + float64(scrollingLevelCameraMargin(w)) - viewportWorldWidth,
		}

		visible := true
		animating := false
		anim.SetAnimation("fade", &component.Animation{
			Active: true,
			Timer:  engine.NewTimer(backgroundFadeDuration),
			Update: func(e *donburi.Entry, a *component.Animation) {
				if animating {
					if visible {
						cam.TransitionAlpha = a.Timer.PercentDone()
						if a.Timer.IsReady() {
							a.Timer.Reset()
							visible = false
							transform.GetTransform(target).LocalPosition = targetPos
						}
					} else {
						cam.TransitionAlpha = 1 - a.Timer.PercentDone()
						if a.Timer.IsReady() {
							visible = true
							animating = false
						}
					}
				} else {
					if math2.Abs(cam.ViewportPosition.X-cam.ViewportBounds.X.Max) < backgroundScrollDistance {
						animating = true
						a.Timer.Reset()
					}
				}
			},
		})
	} else {
		cam.ViewportPosition.X = levelWidth/2.0 - screenWorldWidth/2.0
		cam.ViewportTarget = character

		cam.ViewportBounds.X = &engine.FloatRange{
			Min: float64(-levelCameraMargin(w)),
			Max: levelWidth + float64(levelCameraMargin(w)) - viewportWorldWidth,
		}
	}
}

func ChangeLevel(w donburi.World, level domain.TargetLevel) {
	currentLevel, ok := engine.FindWithComponent(w, component.Level)
	if ok {
		lvl := component.Level.Get(currentLevel)
		lvl.Changing = true
		game := component.MustFindGame(w)

		character, characterFound := engine.FindWithComponent(w, component.Character)
		if characterFound {
			var characterPos *domain.CharacterPosition
			pos := transform.GetTransform(character).LocalPosition
			flipY := component.Sprite.Get(character).FlipY
			characterPos = &domain.CharacterPosition{
				LocalPosition: pos,
				FlipY:         flipY,
			}

			game.PreviousLevel = &component.PreviousLevel{
				Name:              lvl.Name,
				CharacterPosition: characterPos,
			}
		}

		anim := component.Animator.Get(currentLevel)
		anim.Start("transition", currentLevel)
		transition := anim.Animations["transition"]
		transition.OnStop = func(e *donburi.Entry) {
			transform.RemoveRecursive(e)
			NewLevel(w, level)
		}
		anim.SetAnimation("transition", transition)
		return
	}

	NewLevel(w, level)
}

func levelCameraMargin(w donburi.World) int {
	game := component.MustFindGame(w)
	return int(float64(game.Dimensions.ScreenWidth) * levelCameraMarginPercent)
}

func scrollingLevelCameraMargin(w donburi.World) int {
	game := component.MustFindGame(w)
	return int(float64(game.Dimensions.ScreenWidth) * scrollingLevelCameraMarginPercent)
}
