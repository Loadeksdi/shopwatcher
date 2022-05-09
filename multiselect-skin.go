package main

import (
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
	var skinName string
	var ok bool
	if skinName, ok = m.AllSkins[index].LocalizedNames[locale]; !ok {
		skinName = m.AllSkins[index].LocalizedNames["en-US"]
	}
	return skinName
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

func remove(slice []Skin, s int) []Skin {
	return append(slice[:s], slice[s+1:]...)
}
