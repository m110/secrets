package archetype

import (
	"github.com/yohamta/donburi"
	donburicomponent "github.com/yohamta/donburi/component"
	"github.com/yohamta/donburi/features/math"
	"github.com/yohamta/donburi/features/transform"

	"github.com/m110/secrets/domain"

	"github.com/m110/secrets/engine"

	"github.com/m110/secrets/component"
)

type EntryBuilder struct {
	entry *donburi.Entry
}

// New creates a new entry with Transform.
func New(w donburi.World) EntryBuilder {
	return EntryBuilder{
		entry: w.Entry(w.Create(transform.Transform)),
	}
}

func NewTagged(w donburi.World, tag string) EntryBuilder {
	e := New(w).WithTag(tag)
	return e
}

func (b EntryBuilder) With(c donburicomponent.IComponentType) EntryBuilder {
	if !b.entry.HasComponent(c) {
		b.entry.AddComponent(c)
	}

	return b
}

func (b EntryBuilder) WithPosition(pos math.Vec2) EntryBuilder {
	transform.Transform.Get(b.entry).LocalPosition = pos
	return b
}

func (b EntryBuilder) WithScale(scale math.Vec2) EntryBuilder {
	transform.Transform.Get(b.entry).LocalScale = scale
	return b
}

func (b EntryBuilder) WithParent(parent *donburi.Entry) EntryBuilder {
	transform.AppendChild(parent, b.entry, false)
	return b
}

func (b EntryBuilder) WithSprite(sprite component.SpriteData) EntryBuilder {
	if !b.entry.HasComponent(component.Layer) {
		b.With(component.Layer)
	}
	b.With(component.Sprite)
	component.Sprite.SetValue(b.entry, sprite)
	return b
}

func (b EntryBuilder) WithBounds(size engine.Size) EntryBuilder {
	b.With(component.Bounds)

	component.Bounds.SetValue(b.entry, component.BoundsData{
		Width:  float64(size.Width),
		Height: float64(size.Height),
	})

	return b
}

func (b EntryBuilder) WithSpriteBounds() EntryBuilder {
	b.With(component.Bounds)

	sprite := component.Sprite.Get(b.entry)
	imageBounds := sprite.Image.Bounds()
	scale := transform.WorldScale(b.entry)

	component.Bounds.SetValue(b.entry, component.BoundsData{
		Width:  float64(imageBounds.Dx()) * scale.X,
		Height: float64(imageBounds.Dy()) * scale.Y,
	})

	return b
}

func (b EntryBuilder) WithBoundsAsCollider(layer domain.ColliderLayer) EntryBuilder {
	b.With(component.Collider)

	bounds := component.Bounds.Get(b.entry)

	component.Collider.SetValue(b.entry, component.ColliderData{
		Rect:  engine.NewRect(0, 0, bounds.Width, bounds.Height),
		Layer: layer,
	})

	return b
}

func (b EntryBuilder) WithLayer(layer domain.LayerID) EntryBuilder {
	b.With(component.Layer)
	component.Layer.Get(b.entry).Layer = layer
	return b
}

func (b EntryBuilder) WithLayerInherit() EntryBuilder {
	b.With(component.Layer)

	parent, ok := transform.GetParent(b.entry)
	if !ok {
		panic("parent not found")
	}

	parentLayer := component.Layer.Get(parent).Layer
	component.Layer.Get(b.entry).Layer = parentLayer + 1
	return b
}

func (b EntryBuilder) WithText(text component.TextData) EntryBuilder {
	if !b.entry.HasComponent(component.Layer) {
		b.With(component.Layer)
	}
	b.With(component.Text)
	component.Text.SetValue(b.entry, text)
	return b
}

func (b EntryBuilder) WithTag(tag string) EntryBuilder {
	if !b.entry.HasComponent(component.Tag) {
		b.With(component.Tag)
	}
	component.Tag.SetValue(b.entry, component.TagData{
		Tag: tag,
	})
	return b
}

func (b EntryBuilder) Entry() *donburi.Entry {
	return b.entry
}
