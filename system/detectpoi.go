package system

import (
	"math"

	"github.com/m110/secrets/engine"

	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/filter"

	"github.com/m110/secrets/archetype"
	"github.com/m110/secrets/component"
)

type DetectPOI struct {
	poiQuery       *donburi.Query
	characterQuery *donburi.Query
}

func NewDetectPOI() *DetectPOI {
	return &DetectPOI{
		poiQuery:       donburi.NewQuery(filter.Contains(component.POIImage)),
		characterQuery: donburi.NewQuery(filter.Contains(component.Character)),
	}
}

func (d *DetectPOI) Update(w donburi.World) {
	character, ok := d.characterQuery.First(w)
	if !ok {
		return
	}

	characterPos := archetype.HorizontalCenterPosition(character)

	d.poiQuery.Each(w, func(poi *donburi.Entry) {
		poiPos := archetype.HorizontalCenterPosition(poi)

		distance := math.Abs(characterPos - poiPos)
		if distance > archetype.PoiVisibleDistance.Max {
			return
		}

		value := 0.0
		// TODO Probably not efficient to do it each frame
		// Consider marking the POI as interactable asynchronously
		if archetype.CanInteractWithPOI(engine.MustGetParent(poi)) {
			value = distanceToBlendValue(distance)
		}

		component.Sprite.Get(poi).ColorBlendOverride.Value = value
	})
}

func distanceToBlendValue(currentDist float64) float64 {
	rng := archetype.PoiVisibleDistance
	if currentDist <= rng.Min {
		return 1
	}

	if currentDist >= rng.Max {
		return 0
	}

	return 1 - (currentDist-rng.Min)/(rng.Max-rng.Min)
}
