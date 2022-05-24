package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	lang "github.com/cloudfoundry-attic/jibber_jabber"
	"github.com/emersion/go-autostart"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
	"github.com/robfig/cron"
)

var globalStore = GlobalStore{}
var locale string

func createNotifyIcon() {
	ni, err := walk.NewNotifyIcon(globalStore.Ui.mainWindow)
	if err != nil {
		log.Fatal(err)
	}
	ni.SetToolTip("Valorant Shopwatcher - Currently running")
	if err != nil {
		log.Fatal(err)
	}
	icon, err := walk.Resources.Icon("winres/icon.ico")
	if err != nil {
		log.Fatal(err)
	}
	err = ni.SetIcon(icon)
	if err != nil {
		log.Fatal(err)
	}
	ni.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button == walk.LeftButton {
			globalStore.Ui.mainWindow.Show()
			win.ShowWindow(globalStore.Ui.mainWindow.Handle(), win.SW_RESTORE)
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
	for i := 0; i < len(*skinLayouts); i++ {
		composites = append(composites, Composite{
			Layout: VBox{},
			Children: []Widget{
				LinkLabel{
					AssignTo: &(*skinLayouts)[i].LinkLabel,
					Text:     "",
					OnLinkActivated: func(link *walk.LinkLabelLink) {
						exec.Command("explorer", link.URL()).Start()
					},
				},
			},
		})
	}
	return composites
}

func notifyUserIfTheyHaveWantedSkins(notifyIcon *walk.NotifyIcon) {
	for _, skin := range globalStore.Ui.selectedSkinsListBox.AllSkins {
		for _, storeSkin := range globalStore.CurrentShop {
			if skin.Id == storeSkin.Id {
				skinLocalizedName, _ := skin.LocalizedNames.Load(locale)
				notifyIcon.ShowInfo("Valorant Shopwatcher", skinLocalizedName.(string)+" is available in your Valorant shop!")
			}
		}
	}
}

func drawUserform(owner walk.Form) {
	var userForm *walk.Dialog
	var outLELogin *walk.LineEdit
	var outLEPassword *walk.LineEdit
	var outCBRegion *walk.ComboBox
	globalStore.Ui.mainWindow.WindowBase.Synchronize(func() {
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
							Model:    []string{"AP", "EU", "KR", "NA"},
						},
					},
				},
				PushButton{
					Text: "Log in",
					OnClicked: func() {
						globalStore.User = User{Login: outLELogin.Text(), Password: outLEPassword.Text(), Region: outCBRegion.Text()}
						var err error
						if err != nil {
							walk.MsgBox(nil, "Error", "Invalid credentials", walk.MsgBoxIconError)
							globalStore.Ui.mainWindow.WindowBase.Synchronize(func() {
								globalStore.Ui.mainWindow.Show()
								userForm.Run()
							})
							return
						}
						userForm.Close(-1)
					},
				},
			},
		}.Create(owner)
		userForm.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
			if userForm.Result() != -1 {
				os.Exit(0)
			} else {
				globalStore.Channels.LoginWindow <- true
				saveUserData(globalStore.User)
			}
		})
		globalStore.Ui.mainWindow.Show()
		userForm.Run()
	})
}

func drawShop() {
	for index, skinLayout := range globalStore.Ui.skinLayouts {
		res, _ := globalStore.CurrentShop[index].LocalizedNames.Load(locale)
		skinLayout.setData(res.(string), globalStore.CurrentShop[index].Video)
	}
	notifyUserIfTheyHaveWantedSkins(globalStore.Ui.notifyIcon)
}

func drawMfaModal(owner walk.Form, codeLength int) {
	var outLECode *walk.LineEdit
	var mfa *walk.Dialog
	globalStore.Ui.mainWindow.WindowBase.Synchronize(func() {
		Dialog{
			AssignTo: &mfa,
			Title:    "Multi-factor authentication enabled",
			MinSize:  Size{Width: 200, Height: 250},
			Layout:   VBox{},
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
							globalStore.Ui.mainWindow.WindowBase.Synchronize(func() {
								globalStore.Ui.mainWindow.Show()
							})
						}
						defer res.Body.Close()
						var accessTokenContainer AccessTokenContainer
						data, _ := io.ReadAll(res.Body)
						err = json.Unmarshal(data, &accessTokenContainer)
						if err != nil {
							walk.MsgBox(nil, "Error", "Couldn't connect with MFA", walk.MsgBoxIconError)
							globalStore.Ui.mainWindow.WindowBase.Synchronize(func() {
								globalStore.Ui.mainWindow.Show()
							})
						}
						globalStore.User.AccessToken = accessTokenContainer.Response.Parameters.Uri.Query().Get("access_token")
						saveUserData(globalStore.User)
						globalStore.Channels.MFAToken <- true
						mfa.Close(-1)
					},
				},
			},
		}.Create(owner)
		mfa.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
			if mfa.Result() != -1 {
				os.Exit(0)
			}
		})
		globalStore.Ui.mainWindow.Show()
		mfa.Run()
	})
	<-globalStore.Channels.MFAToken
}

func startCron() {
	c := cron.New()
	c.AddFunc("0 0 2 ? * *", func() {
		seedUser()
		notifyUserIfTheyHaveWantedSkins(globalStore.Ui.notifyIcon)
	})
	c.Start()
}

func runAppOnStartup() {
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
}

func setupChannels() {
	globalStore.Channels.MFAToken = make(chan bool)
	globalStore.Channels.NewToken = make(chan string)
	globalStore.Channels.LoginWindow = make(chan bool)
}

//go:generate go-winres make --product-version=dev

func main() {
	runAppOnStartup()
	var err error
	if locale, err = lang.DetectIETF(); err != nil {
		locale = "en-US"
	}
	setupChannels()
	globalStore.User, _ = loadSavedUser()
	loadSavedSkins()
	rect := win.RECT{}
	win.GetWindowRect(win.GetDesktopWindow(), &rect)
	MainWindow{
		AssignTo: &globalStore.Ui.mainWindow,
		Title:    "Valorant Shopwatcher",
		Layout:   VBox{},
		Bounds:   Rectangle{X: int(rect.Right)/2 - 700, Y: int(rect.Bottom)/2 - 400, Width: 1400, Height: 800},
		OnSizeChanged: func() {
			if win.IsIconic(globalStore.Ui.mainWindow.Handle()) {
				globalStore.Ui.mainWindow.Hide()
			}
		},
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
								Text: "My wishlist",
							},
							ListBox{
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
	go startCron()
	globalStore.Ui.mainWindow.Hide()
	globalStore.Ui.mainWindow.Run()
}
