package system

import (
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/transform"
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

	characterPos := transform.WorldPosition(character)

	d.poiQuery.Each(w, func(poi *donburi.Entry) {
		poiPos := transform.WorldPosition(poi)
		// TODO Probably should consider width + height and calculate off center
		// TODO Could be based on just X pos
		distance := characterPos.Distance(poiPos)
		component.Sprite.Get(poi).ColorBlendOverride.Value = distanceToBlendValue(distance)
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
