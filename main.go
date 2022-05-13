package main

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"sort"

	lang "github.com/cloudfoundry-attic/jibber_jabber"
	"github.com/emersion/go-autostart"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/robfig/cron"
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
	Skins SortedSkins `json:"skins"`
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
	shop                 *walk.Composite
	mainWindow           *walk.MainWindow
	skinLayouts          []SkinLayout
	notifyIcon           *walk.NotifyIcon
}

type GlobalStore struct {
	Ui          UiElems
	CurrentShop []Skin
	User        User
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
	apiKey, err := base64.StdEncoding.DecodeString("UkdBUEktMmNiYTdjOGQtZWFlMy00ZTk0LThlY2EtMGRkOWU0MzhiZmI0")
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

func seedUser() {
	if globalStore.User.Login == "" {
		drawUserform(globalStore.Ui.mainWindow)
	}
	accessToken, err := getAccessToken()
	if err != nil {
		walk.MsgBox(nil, "Error", "The app could not call Riot servers", walk.MsgBoxIconError)
	}
	globalStore.CurrentShop, err = fetchSkinsWithToken(accessToken)
	if err != nil {
		walk.MsgBox(nil, "Error", "The app could not fetch skins", walk.MsgBoxIconError)
	}
	/**
	for index, skinLayout := range globalStore.Ui.skinLayouts {
		skinInShop := globalStore.CurrentShop[index]
		skinLayout.setData(skinInShop.LocalizedNames[locale], skinInShop.AssetPath)
	}*/
}

func notifyUserIfTheyHaveWantedSkins(notifyIcon *walk.NotifyIcon) {
	for _, skin := range globalStore.Ui.selectedSkinsListBox.AllSkins {
		for _, storeSkin := range globalStore.CurrentShop {
			if skin.Id == storeSkin.Id {
				notifyIcon.ShowInfo("Valorant Shopwatcher", skin.LocalizedNames[locale]+" is available in your Valorant shop!")
			}
		}
	}
}

//go:generate go-winres make --product-version=dev

func createNotifyIcon() {
	ni, err := walk.NewNotifyIcon(globalStore.Ui.mainWindow)
	if err != nil {
		log.Fatal(err)
	}
	ni.SetToolTip("Valorant Shopwatcher - Currently running")
	ni.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button != walk.LeftButton {
			return
		}

	})
	exitAction := walk.NewAction()
	if err := exitAction.SetText("Exit"); err != nil {
		log.Fatal(err)
	}
	exitAction.Triggered().Attach(func() { walk.App().Exit(0) })
	if err := ni.ContextMenu().Actions().Add(exitAction); err != nil {
		log.Fatal(err)
	}
	if err := ni.SetVisible(true); err != nil {
		log.Fatal(err)
	}
	globalStore.Ui.notifyIcon = ni
}

func main() {
	var appExec string
	var err error
	if appExec, err = os.Executable(); err != nil {
		log.Fatal(err)
	}
	var appExecResolved string
	if appExecResolved, err = filepath.EvalSymlinks(appExec); err != nil {
		log.Fatal(err)
	}
	app := &autostart.App{
		Name: "Valorant Shopwatcher",
		DisplayName: "Valorant Shopwatcher",
		Exec: []string{appExecResolved},
	}
	if err := app.Enable(); err != nil {
		log.Fatal(err)
	}
	if locale, err = lang.DetectIETF(); err != nil {
		locale = "en-US"
	}
	globalStore.User, _ = loadSavedUser()
	loadSavedSkins()
	MainWindow{
		AssignTo: &globalStore.Ui.mainWindow,
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
									globalStore.Ui.selectedSkinsListBox.InsertSelectedSkins(globalStore.Ui.skinsListBox.SelectedSkins)
									globalStore.Ui.skinsListBox.SetSelectedIndexes([]int{})
									saveSkinsData()
									notifyUserIfTheyHaveWantedSkins(globalStore.Ui.notifyIcon)
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
			Composite{
				AssignTo: &globalStore.Ui.shop,
				Layout:   HBox{},
				Children: createAllCompositesForSkins(&globalStore.Ui.skinLayouts),
			},
		},
	}.Create()
	createNotifyIcon()
	go seedUser()
	go feedData()
	globalStore.Ui.mainWindow.Run()
	c := cron.New()
	c.AddFunc("0 0 2 ? * *", func() {
		seedUser()
		notifyUserIfTheyHaveWantedSkins(globalStore.Ui.notifyIcon)
	})
}
