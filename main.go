package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	lang "github.com/cloudfoundry-attic/jibber_jabber"
	"github.com/emersion/go-autostart"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/robfig/cron"
)

var globalStore = GlobalStore{}
var locale string
var userForm *walk.Dialog

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

func createAllCompositesForSkins(skinLayouts *[]SkinLayout) []Widget {
	*skinLayouts = make([]SkinLayout, 4)
	var composites []Widget
	for _, skinLayout := range *skinLayouts {
		composites = append(composites, Composite{
			Layout: VBox{},
			Children: []Widget{
				Label{
					AssignTo: &skinLayout.Label,
					Text: "",
				}
			},
		})
	}
	return composites
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

func drawUserform(owner walk.Form) {
	var outLELogin *walk.LineEdit
	var outLEPassword *walk.LineEdit
	var outCBRegion *walk.ComboBox
	Dialog{
		AssignTo: &userForm,
		Title:    "Login",
		MinSize:  Size{Width: 200, Height: 250},
		Layout:   VBox{},
		Children: []Widget{
			Composite{
				Layout: HBox{},
				Children: []Widget{
					Label{
						Text: "Username:",
					},
					LineEdit{
						Name:     "Username",
						AssignTo: &outLELogin,
					},
				},
			},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					Label{
						Text: "Password:",
					},
					LineEdit{
						Name:         "Password",
						AssignTo:     &outLEPassword,
						PasswordMode: true,
					},
				},
			},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					Label{
						Text: "Region:",
					},
					ComboBox{
						Name:     "Region",
						AssignTo: &outCBRegion,
						Model:    []string{"AP", "BR", "ESPORTS", "EU", "KR", "LATAM", "NA"},
					},
				},
			},
			PushButton{
				Text: "Log in",
				OnClicked: func() {
					globalStore.User = User{Login: outLELogin.Text(), Password: outLEPassword.Text(), Region: outCBRegion.Text()}
					accessToken, err := getAccessToken()
					if err != nil {
						walk.MsgBox(nil, "Error", "Invalid credentials", walk.MsgBoxIconError)
						return
					}
					saveUserData(globalStore.User)
					globalStore.CurrentShop, _ = fetchSkinsWithToken(accessToken)
					userForm.Close(0)
				},
			},
		},
	}.Run(owner)
}

func drawMfaModal(owner walk.Form, codeLength int) string {
	var outLECode *walk.LineEdit
	var accessToken string
	Dialog{
		Title:   "Multi-factor authentication enabled",
		MinSize: Size{Width: 200, Height: 250},
		Layout:  VBox{},
		Children: []Widget{
			Label{
				Text: "Please enter MFA code given by Riot Games",
			},
			LineEdit{
				AssignTo:  &outLECode,
				Name:      "MFA code",
				MaxLength: codeLength,
			},
			PushButton{
				Text: "Submit code",
				OnClicked: func() {
					body, _ := json.Marshal(MFABody{Type: "multifactor", Code: outLECode.Text(), RememberDevice: false})
					req, _ := http.NewRequest("PUT", "https://auth.riotgames.com/api/v1/authorization", bytes.NewBuffer(body))
					setRequestHeaders(req)
					res, err := client.Do(req)
					if err != nil {
						walk.MsgBox(nil, "Error", "Couldn't connect with MFA", walk.MsgBoxIconError)
					}
					defer res.Body.Close()
					var accessTokenContainer AccessTokenContainer
					data, _ := io.ReadAll(res.Body)
					err = json.Unmarshal(data, &accessTokenContainer)
					if err != nil {
						walk.MsgBox(nil, "Error", "Couldn't connect with MFA", walk.MsgBoxIconError)
					}
					accessToken = accessTokenContainer.Response.Parameters.Uri.Query().Get("access_token")
				},
			},
		},
	}.Run(owner)
	return accessToken
}

//go:generate go-winres make --product-version=dev

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
		Name:        "Valorant Shopwatcher",
		DisplayName: "Valorant Shopwatcher",
		Exec:        []string{appExecResolved},
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
