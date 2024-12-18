package domain

import (
	"fmt"
	"strconv"
	"time"

	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/math"
)

const (
	CreditsPassage = "credits"
)

type RawStory struct {
	Title      string
	Passages   []RawPassage
	StartLevel TargetLevel
}

type RawPassage struct {
	Title      string
	Tags       []string
	Paragraphs []Paragraph
	Conditions []Condition
	Macros     []Macro
	Links      []RawLink
}

type Paragraph struct {
	Text           string
	Align          ParagraphAlign
	Type           ParagraphType
	Conditions     []Condition
	Delay          time.Duration
	Effect         ParagraphEffect
	EffectDuration time.Duration
}

type ParagraphAlign int

const (
	ParagraphAlignLeft ParagraphAlign = iota
	ParagraphAlignCenter
)

type ParagraphType int

const (
	ParagraphTypeStandard ParagraphType = iota
	ParagraphTypeHeader
	ParagraphTypeHint
	ParagraphTypeFear
	ParagraphTypeReceived
	ParagraphTypeLost
	ParagraphTypeRead
)

type ParagraphEffect int

const (
	ParagraphEffectDefault ParagraphEffect = iota
	ParagraphEffectTyping
	ParagraphEffectFadeIn
	ParagraphEffectNone
)

type RawLink struct {
	Text       string
	Target     string
	Level      *TargetLevel
	Conditions []Condition
	Tags       []string
}

type TargetLevel struct {
	Name              string
	Entrypoint        *int
	CharacterPosition *CharacterPosition
}

type CharacterPosition struct {
	LocalPosition math.Vec2
	FlipY         bool
}

type Story struct {
	world donburi.World

	Title      string
	Passages   map[string]*Passage
	StartLevel TargetLevel

	Money int
	Items []Item
	Facts map[string]bool
}

type Item struct {
	Name  string
	Count int
}

func NewStory(w donburi.World, rawStory RawStory) *Story {
	story := &Story{
		world:      w,
		Title:      rawStory.Title,
		StartLevel: rawStory.StartLevel,
		Items:      []Item{},
		Facts:      map[string]bool{},
	}

	passagesMap := map[string]*Passage{}
	for _, p := range rawStory.Passages {
		var isOneTime bool
		for _, tag := range p.Tags {
			if tag == "once" {
				isOneTime = true
			}
		}

		// Set all facts to false initially - useful for debug
		for _, c := range p.Conditions {
			if c.Type == ConditionTypeFact {
				story.Facts[c.Value] = false
			}
		}
		for _, m := range p.Macros {
			if m.Type == MacroTypeSetFact {
				story.Facts[m.Value] = false
			}
		}

		passage := &Passage{
			story:      story,
			Title:      p.Title,
			Paragraphs: p.Paragraphs,
			Conditions: p.Conditions,
			Macros:     p.Macros,
			IsOneTime:  isOneTime,
		}

		var links []*Link
		for _, l := range p.Links {
			// Set all facts to false initially - useful for debug
			for _, c := range l.Conditions {
				if c.Type == ConditionTypeFact {
					story.Facts[c.Value] = false
				}
			}

			links = append(links, &Link{
				passage:    passage,
				Text:       l.Text,
				Level:      l.Level,
				Conditions: l.Conditions,
				Tags:       l.Tags,
			})
		}

		passage.AllLinks = links
		passagesMap[p.Title] = passage
	}

	for _, p := range rawStory.Passages {
		for i, l := range p.Links {
			passagesMap[p.Title].AllLinks[i].Target = passagesMap[l.Target]
		}
	}

	story.Passages = passagesMap

	return story
}

func (s *Story) Fact(fact string) bool {
	return s.Facts[fact]
}

func (s *Story) AddMoney(amount int) {
	s.Money += amount
	if s.Money < 0 {
		panic("Negative money")
	}
	if amount > 0 {
		MoneyReceivedEvent.Publish(s.world, MoneyReceived{
			Amount: amount,
		})
	} else if amount < 0 {
		MoneySpentEvent.Publish(s.world, MoneySpent{
			Amount: -amount,
		})
	}
	s.publishInventoryUpdated()
}

func (s *Story) PassageForLevel(level TargetLevel) (*Passage, bool) {
	passage, ok := s.Passages[level.Name]
	if ok {
		return passage, ok
	}

	if level.Entrypoint == nil {
		return nil, false
	}

	passage, ok = s.Passages[fmt.Sprintf("%s,%d", level.Name, *level.Entrypoint)]
	return passage, ok
}

func (s *Story) PassageByTitle(title string) *Passage {
	p, ok := s.Passages[title]
	if !ok {
		panic("Passage not found: " + title)
	}

	return p
}

func (s *Story) AddItem(item string) {
	for _, i := range s.Items {
		if i.Name == item {
			i.Count++
			return
		}
	}

	s.Items = append(s.Items, Item{
		Name:  item,
		Count: 1,
	})

	ItemReceivedEvent.Publish(s.world, ItemReceived{
		Item: InventoryItem{
			Name:  item,
			Count: 1,
		},
	})
	s.publishInventoryUpdated()
}

func (s *Story) TakeItem(item string) {
	for i, it := range s.Items {
		if it.Name == item {
			if it.Count == 1 {
				s.Items = append(s.Items[:i], s.Items[i+1:]...)
			} else {
				s.Items[i].Count--
			}

			ItemLostEvent.Publish(s.world, ItemLost{
				Item: InventoryItem{
					Name:  item,
					Count: 1,
				},
			})

			s.publishInventoryUpdated()
			return
		}
	}
}

func (s *Story) publishInventoryUpdated() {
	var eventItems []InventoryItem
	for _, i := range s.Items {
		eventItems = append(eventItems, InventoryItem{
			Name:  i.Name,
			Count: i.Count,
		})
	}
	InventoryUpdatedEvent.Publish(s.world, InventoryUpdated{
		Money: s.Money,
		Items: eventItems,
	})
}

func (s *Story) SetFact(fact string) {
	s.Facts[fact] = true

	StoryFactSetEvent.Publish(s.world, StoryFactSet{
		Fact: fact,
	})
}

// SetFactTo removes a fact from the story
// Use only for debugging purposes.
func (s *Story) SetFactTo(fact string, value bool) {
	if value {
		s.SetFact(fact)
		return
	}

	s.Facts[fact] = false
}

func (s *Story) TestCondition(c Condition) bool {
	switch c.Type {
	case ConditionTypeHasItem:
		found := false
		for _, i := range s.Items {
			if i.Name == c.Value {
				found = true
				break
			}
		}

		return found == c.Positive
	case ConditionTypeFact:
		return s.Facts[c.Value] == c.Positive
	case ConditionTypeHasMoney:
		money, err := strconv.Atoi(c.Value)
		if err != nil {
			panic(err)
		}

		return s.Money >= money == c.Positive
	default:
		panic("Unknown condition type: " + c.Type)
	}

	return false
}

type Passage struct {
	story *Story

	Title      string
	Paragraphs []Paragraph
	Conditions []Condition
	Macros     []Macro
	AllLinks   []*Link

	IsOneTime bool
	Visited   bool
}

func (p *Passage) AvailableParagraphs() []Paragraph {
	var paragraphs []Paragraph

	for _, s := range p.Paragraphs {
		if len(s.Conditions) > 0 {
			var skip bool
			for _, c := range s.Conditions {
				if !p.story.TestCondition(c) {
					skip = true
					break
				}
			}

			if skip {
				continue
			}
		}

		paragraphs = append(paragraphs, s)
	}

	return paragraphs
}

func (p *Passage) Content() string {
	var content string

	for _, s := range p.AvailableParagraphs() {
		content += s.Text
	}

	return content
}

func (p *Passage) ConditionsMet() bool {
	if p.IsOneTime && p.Visited {
		return false
	}

	for _, c := range p.Conditions {
		if !p.story.TestCondition(c) {
			return false
		}
	}

	return true
}

func (p *Passage) Visit() {
	p.Visited = true

	for _, m := range p.Macros {
		switch m.Type {
		case MacroTypeAddItem:
			p.story.AddItem(m.Value)
		case MacroTypeTakeItem:
			p.story.TakeItem(m.Value)
		case MacroTypeSetFact:
			p.story.SetFact(m.Value)
		case MacroTypeAddMoney:
			money, err := strconv.Atoi(m.Value)
			if err != nil {
				panic(err)
			}
			p.story.AddMoney(money)
		case MacroTypeTakeMoney:
			money, err := strconv.Atoi(m.Value)
			if err != nil {
				panic(err)
			}
			p.story.AddMoney(-money)
		case MacroTypePlayMusic:
			MusicChangedEvent.Publish(p.story.world, MusicChanged{
				Track: m.Value,
			})
		case MacroTypeChangeCharacterSpeed:
			speed, err := strconv.ParseInt(m.Value, 10, 64)
			if err != nil {
				// TODO This validation should be done at the parser level
				panic(err)
			}

			CharacterSpeedChangedEvent.Publish(p.story.world, CharacterSpeedChanged{
				SpeedChange: float64(speed),
			})
		default:
			panic("Unknown macro type: " + m.Type)
		}
	}
}

func (p *Passage) Links() []*Link {
	var links []*Link
	for _, l := range p.AllLinks {
		if l.Target.IsOneTime && l.Target.Visited {
			// Edge case scenario: allow the link if it points back to the same passage
			// This is useful for "exit" links in the Twine editor
			if !l.IsExit() || l.Target != p {
				continue
			}
		}

		var skip bool
		for _, c := range l.Conditions {
			if !p.story.TestCondition(c) {
				skip = true
				break
			}
		}

		if skip {
			continue
		}

		links = append(links, l)
	}

	return links
}

type Link struct {
	passage *Passage

	Text       string
	Target     *Passage
	Level      *TargetLevel
	Conditions []Condition
	Visited    bool
	Tags       []string
}

func (l *Link) Visit() {
	if l.IsExit() || l.Level != nil {
		return
	}
	l.Visited = true
	l.Target.Visit()
}

func (l *Link) IsExit() bool {
	for _, t := range l.Tags {
		if t == "exit" {
			return true
		}
	}

	return false
}

func (l *Link) AllVisited() bool {
	if !l.Visited {
		return false
	}

	if l.IsExit() || l.Level != nil {
		return false
	}

	for _, link := range deepChildLinks(l, l.passage) {
		if !link.Visited && !l.IsExit() {
			return false
		}
	}

	return true
}

func (l *Link) HasTag(tag string) bool {
	for _, t := range l.Tags {
		if t == tag {
			return true
		}
	}

	return false
}

func deepChildLinks(link *Link, source *Passage) []*Link {
	visited := make(map[*Link]bool)
	var links []*Link
	deepChildLinksRecursive(link, source, visited, &links)
	return links
}

func deepChildLinksRecursive(link *Link, source *Passage, visited map[*Link]bool, result *[]*Link) {
	// Skip if we've already visited this link
	if visited[link] {
		return
	}

	// Mark current link as visited
	visited[link] = true

	// Process all child links
	for _, l := range link.Target.Links() {
		if l.Target == source {
			continue
		}

		if l.HasTag("back") {
			continue
		}

		// Add the link if we haven't seen it
		if !visited[l] {
			*result = append(*result, l)
			deepChildLinksRecursive(l, source, visited, result)
		}
	}
}

type MacroType string

const (
	MacroTypeAddItem              MacroType = "addItem"
	MacroTypeTakeItem             MacroType = "takeItem"
	MacroTypeSetFact              MacroType = "setFact"
	MacroTypeAddMoney             MacroType = "addMoney"
	MacroTypeTakeMoney            MacroType = "takeMoney"
	MacroTypePlayMusic            MacroType = "playMusic"
	MacroTypeChangeCharacterSpeed MacroType = "changeCharacterSpeed"
)

type Macro struct {
	Type  MacroType
	Value string
}

type ConditionType string

const (
	ConditionTypeHasItem  ConditionType = "hasItem"
	ConditionTypeFact     ConditionType = "fact"
	ConditionTypeHasMoney ConditionType = "hasMoney"
)

type Condition struct {
	Positive bool
	Type     ConditionType
	Value    string
}
