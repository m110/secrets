package assets

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	_ "image/png"
	"io/fs"
	"path"
	"strings"

	"github.com/m110/secrets/engine"

	"github.com/lafriks/go-tiled"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/text/language"

	"github.com/m110/secrets/assets/twine"
	"github.com/m110/secrets/domain"
)

var (
	//go:embed fonts/UndeadPixelLight.ttf
	normalFontData []byte

	//go:embed *
	assetsFS embed.FS

	//go:embed story.twee
	story []byte

	Story domain.RawStory

	SmallFont  *text.GoTextFace
	NormalFont *text.GoTextFace

	Character []*ebiten.Image

	levelNames = map[string]struct{}{}
	Levels     = map[string]domain.Level{}
)

func MustLoadAssets() {
	SmallFont = mustLoadFont(normalFontData, 10)
	NormalFont = mustLoadFont(normalFontData, 24)

	s, err := twine.ParseStory(string(story))
	if err != nil {
		panic(err)
	}
	Story = s

	characterFrames := 4
	Character = make([]*ebiten.Image, 4)
	for i := range characterFrames {
		Character[i] = mustNewEbitenImage(mustReadFile(fmt.Sprintf("character/character-%v.png", i+1)))
	}

	levelPaths, err := fs.Glob(assetsFS, "levels/*.tmx")
	if err != nil {
		panic(err)
	}

	for _, p := range levelPaths {
		name := strings.TrimSuffix(path.Base(p), ".tmx")
		levelNames[name] = struct{}{}
	}

	for _, p := range levelPaths {
		name := strings.TrimSuffix(path.Base(p), ".tmx")
		Levels[name] = mustLoadLevel(p)
	}
}

func mustLoadLevel(path string) domain.Level {
	levelMap, err := tiled.LoadFile(path, tiled.WithFileSystem(assetsFS))
	if err != nil {
		panic(err)
	}

	var imageName string
	for _, t := range levelMap.ImageLayers {
		if t.Name == "Background" {
			imageName = t.Image.Source
		}
	}

	if imageName == "" {
		panic("background image not found")
	}

	var pois []domain.POI
	for _, o := range levelMap.ObjectGroups {
		for _, obj := range o.Objects {
			if obj.Class == "poi" {
				rect := engine.NewRect(obj.X, obj.Y, obj.Width, obj.Height)
				poi := domain.POI{
					ID:          fmt.Sprint(obj.ID),
					TriggerRect: rect,
					Rect:        rect,
				}

				passage := obj.Properties.GetString("passage")
				if passage != "" {
					assertPassageExists(passage)
					poi.Passage = passage
				}

				level := obj.Properties.GetString("level")
				if level != "" {
					assertLevelExists(level)
					poi.Level = level
				}

				pois = append(pois, poi)
			}
		}
	}

	for _, o := range levelMap.ObjectGroups {
		for _, obj := range o.Objects {
			if obj.Class == "trigger" {
				rect := engine.NewRect(obj.X, obj.Y, obj.Width, obj.Height)
				poiID := obj.Properties.GetString("poi")

				var found bool
				for i, p := range pois {
					if poiID == p.ID {
						p.TriggerRect = rect
						pois[i] = p
						found = true
						break
					}
				}

				if !found {
					panic(fmt.Sprintf("poi not found: %v", poiID))
				}
			}
		}
	}

	return domain.Level{
		Background: mustNewEbitenImage(mustReadFile(fmt.Sprintf("levels/%v", imageName))),
		POIs:       pois,
	}
}

func assertPassageExists(name string) {
	for _, p := range Story.Passages {
		if p.Title == name {
			return
		}
	}

	panic(fmt.Sprintf("passage not found: %v", name))
}

func assertLevelExists(name string) {
	if _, ok := levelNames[name]; !ok {
		panic(fmt.Sprintf("level not found: %v", name))
	}
}

func mustLoadFont(data []byte, size int) *text.GoTextFace {
	s, err := text.NewGoTextFaceSource(bytes.NewReader(data))
	if err != nil {
		panic(err)
	}

	return &text.GoTextFace{
		Source:    s,
		Direction: text.DirectionLeftToRight,
		Size:      float64(size),
		Language:  language.English,
	}
}

func mustReadFile(name string) []byte {
	data, err := assetsFS.ReadFile(name)
	if err != nil {
		panic(err)
	}

	return data
}

func mustNewEbitenImage(data []byte) *ebiten.Image {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		panic(err)
	}

	return ebiten.NewImageFromImage(img)
}
