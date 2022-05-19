package main

import (
	"fmt"
	"sort"

	"github.com/lxn/walk"
)

type MultiSelectList struct {
	*walk.ListBox
	walk.ListModelBase
	SelectedSkins []Skin
	AllSkins      []Skin
}

func (m *MultiSelectList) ItemCount() int {
	return len(m.AllSkins)
}

func (m *MultiSelectList) Value(index int) interface{} {
	var skinName any
	var ok bool
	m.AllSkins[index].LocalizedNames.Range(func(key any, value any) bool{
		fmt.Println(key, value)
		return true
	})
	if skinName, ok = m.AllSkins[index].LocalizedNames.Load(locale); !ok {
		skinName, _ = m.AllSkins[index].LocalizedNames.Load("en-US")
	}
	if skinName == nil {
		return nil
	}
	return skinName.(string)
}

func (m *MultiSelectList) FeedList(skins SortedSkins) {
	sort.Sort(skins)
	m.AllSkins = skins
	m.SetModel(m)
}

func (m *MultiSelectList) SelectedIndexesChanged() {
	list := m.ListBox.SelectedIndexes()
	m.SelectedSkins = make([]Skin, len(list))
	for i, v := range list {
		m.SelectedSkins[i] = m.AllSkins[v]
	}
}

func (m *MultiSelectList) checkIfSkinIsAlreadySelected(skin Skin) bool {
	for _, alreadySelectedSkin := range m.AllSkins {
		if alreadySelectedSkin.Id == skin.Id {
			return true
		}
	}
	return false
}

func (m *MultiSelectList) InsertSelectedSkins(skins []Skin) {
	var sortedSkins SortedSkins = m.AllSkins 
	for _, skin := range skins {
		if !m.checkIfSkinIsAlreadySelected(skin) {
			sortedSkins = append(sortedSkins, skin)
		}
	}
	sort.Sort(sortedSkins)
	m.AllSkins = sortedSkins
	m.SetModel(m)
}

func remove(slice []Skin, s int) []Skin {
	return append(slice[:s], slice[s+1:]...)
}
