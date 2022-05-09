package main

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
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
var cj, _ = cookiejar.New(nil)
var defaultTransport = http.DefaultTransport.(*http.Transport)
var customTransport = &http.Transport{
	Proxy:                 defaultTransport.Proxy,
	DialContext:           defaultTransport.DialContext,
	MaxIdleConns:          defaultTransport.MaxIdleConns,
	IdleConnTimeout:       defaultTransport.IdleConnTimeout,
	ExpectContinueTimeout: defaultTransport.ExpectContinueTimeout,
	TLSHandshakeTimeout:   defaultTransport.TLSHandshakeTimeout,
	TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, PreferServerCipherSuites: false,
		CipherSuites: []uint16{
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		}},
}
var client = &http.Client{Jar: cj, Transport: customTransport}

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
	for _, skin := range response.Skins {
		skin.AssetPath = "https://github.com/InFinity54/Valorant_DDragon/blob/master/WeaponSkins/" + skin.Id + ".png"
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

func saveSkinsData() {
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
	var mw *walk.MainWindow
	var err error
	locale, err = lang.DetectIETF()
	if err != nil {
		locale = "en-US"
	}
	loadSavedSkins()
	MainWindow{
		AssignTo: &mw,
		Title:    "Valorant Shopwatcher",
		MinSize:  Size{Width: 600, Height: 400},
		Layout:   VBox{},
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
									saveSkinsData()
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
									saveSkinsData()
								},
							},
						},
					},
					Composite{
						Layout: VBox{},
						Children: []Widget{
							Label{
								Text: "My watchlist",
							},
							ListBox{
								Name:                     "My watchlist",
								MultiSelection:           true,
								Model:                    &globalStore.Ui.selectedSkinsListBox,
								AssignTo:                 &globalStore.Ui.selectedSkinsListBox.ListBox,
								OnSelectedIndexesChanged: globalStore.Ui.selectedSkinsListBox.SelectedIndexesChanged,
							},
						},
					},
				},
			},
			Label{
				Text: "My current shop",
			},
			ListBox{
				Name: "Shop",
			},
		},
	}.Create()
	user, err := loadSavedUser()
	if err != nil {
		user = drawUserform(mw)
	}
	if user.Login == "" {
	}
	go feedData()
	mw.Run()
}
