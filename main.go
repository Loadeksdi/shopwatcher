package main

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"sort"

	lang "github.com/cloudfoundry-attic/jibber_jabber"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

type SortedSkins []Skin

func (s SortedSkins) Len() int {
	return len(s)
}

func (s SortedSkins) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s SortedSkins) Less(i, j int) bool {
	if _, ok := s[i].LocalizedNames[locale]; !ok {
		locale = "en-US"
	}
	localizedSlice := []string{s[i].LocalizedNames[locale], s[j].LocalizedNames[locale]}
	sort.Strings(localizedSlice)
	return localizedSlice[0] == s[i].LocalizedNames[locale]
}

type Response struct {
	Skins SortedSkins
}

type Skin struct {
	Name           string
	LocalizedNames map[string]string
	Id             string
	AssetName      string
	AssetPath      string
}

type UiElems struct {
	skinsListBox         MultiSelectList
	selectedSkinsListBox MultiSelectList
}

type GlobalStore struct {
	Ui UiElems
}

var globalStore = GlobalStore{}
var client = &http.Client{}
var locale string

func fetchSkins() ([]Skin, error) {
	req, _ := http.NewRequest("GET", "https://eu.api.riotgames.com/val/content/v1/contents?locale="+locale, nil)
	apiKey, err := base64.StdEncoding.DecodeString("UkdBUEktMTkxOGFmYWYtYjg5ZS00MzA3LTg2YzctYzNiYTQ5MmY0Njcz")
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Riot-Token", string(apiKey))
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var response Response
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return nil, err
	}
	return response.Skins, nil
}

func loadSavedSkins() {
	file, _ := os.ReadFile("saves/skins.json")
	var savedSkins SortedSkins
	err := json.Unmarshal(file, &savedSkins)
	if err != nil {
		walk.MsgBox(nil, "Error", "The app could not load saved skins", walk.MsgBoxIconError)
	}
	globalStore.Ui.selectedSkinsListBox.AllSkins = savedSkins
}

func feedData() {
	res, err := fetchSkins()
	if err != nil {
		walk.MsgBox(nil, "Error", "The app could not fetch skins", walk.MsgBoxIconError)
	}
	globalStore.Ui.skinsListBox.FeedList(res)
}

func saveData() {
	json, err := json.MarshalIndent(globalStore.Ui.selectedSkinsListBox.AllSkins, "", "  ")
	if err != nil {
		walk.MsgBox(nil, "Error", "The app could not save the data", walk.MsgBoxIconError)
	}
	if _, err := os.Stat("saves"); os.IsNotExist(err) {
		os.Mkdir("saves", 0777)
	}
	err = ioutil.WriteFile("saves/skins.json", json, 0644)
	if err != nil {
		walk.MsgBox(nil, "Error", "The app could not save the data", walk.MsgBoxIconError)
	}
}

//go:generate go-winres make --product-version=dev

func main() {
	var err error
	locale, err = lang.DetectIETF()
	if err != nil {
		locale = "en-US"
	}
	loadSavedSkins()
	window := MainWindow{
		Title:   "Valorant Shopwatcher",
		MinSize: Size{600, 400},
		Layout:  VBox{},
		Children: []Widget{
			Composite{
				Layout: HBox{},
				Children: []Widget{
					Composite{
						Layout: VBox{},
						Children: []Widget{
							Label{
								Text: "All skins",
							},
							ListBox{
								Name:                     "Skins",
								AssignTo:                 &globalStore.Ui.skinsListBox.ListBox,
								MultiSelection:           true,
								OnSelectedIndexesChanged: globalStore.Ui.skinsListBox.SelectedIndexesChanged,
							},
						},
					},
					Composite{
						Layout: VBox{},
						Children: []Widget{
							PushButton{
								Text: ">>",
								OnClicked: func() {
									globalStore.Ui.selectedSkinsListBox.FeedList(append(globalStore.Ui.selectedSkinsListBox.AllSkins, globalStore.Ui.skinsListBox.SelectedSkins...))
									globalStore.Ui.skinsListBox.SetSelectedIndexes([]int{})
									saveData()
								},
							},
							PushButton{
								Text: "<<",
								OnClicked: func() {
									for index, elmIndex := range globalStore.Ui.selectedSkinsListBox.SelectedIndexes() {
										globalStore.Ui.selectedSkinsListBox.AllSkins = remove(globalStore.Ui.selectedSkinsListBox.AllSkins, elmIndex-index)
									}
									globalStore.Ui.selectedSkinsListBox.FeedList(globalStore.Ui.selectedSkinsListBox.AllSkins)
									globalStore.Ui.selectedSkinsListBox.SetSelectedIndexes([]int{})
									saveData()
								},
							},
						},
					},
					Composite{
						Layout: VBox{},
						Children: []Widget{
							Label{
								Text: "Selected skins",
							},
							ListBox{
								Name:                     "Selected skins",
								MultiSelection:           true,
								Model:                    &globalStore.Ui.selectedSkinsListBox,
								AssignTo:                 &globalStore.Ui.selectedSkinsListBox.ListBox,
								OnSelectedIndexesChanged: globalStore.Ui.selectedSkinsListBox.SelectedIndexesChanged,
							},
						},
					},
				},
			},
		},
	}
	go feedData()
	window.Run()
}
