package domain

import (
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/events"
)

type InventoryItem struct {
	Name  string
	Count int
}

type InventoryUpdated struct {
	Money int
	Items []InventoryItem
}

var InventoryUpdatedEvent = events.NewEventType[InventoryUpdated]()

type JustCollided struct {
	Entry      *donburi.Entry
	Layer      ColliderLayer
	Other      *donburi.Entry
	OtherLayer ColliderLayer
}

var JustCollidedEvent = events.NewEventType[JustCollided]()

type JustOutOfCollision struct {
	Entry      *donburi.Entry
	Layer      ColliderLayer
	Other      *donburi.Entry
	OtherLayer ColliderLayer
}

var JustOutOfCollisionEvent = events.NewEventType[JustOutOfCollision]()

type ButtonClicked struct{}

var ButtonClickedEvent = events.NewEventType[ButtonClicked]()

type MusicChanged struct {
	Track string
}

var MusicChangedEvent = events.NewEventType[MusicChanged]()
