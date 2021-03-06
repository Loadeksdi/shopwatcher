package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"golang.org/x/text/encoding/unicode"

	"github.com/danieljoos/wincred"
	"github.com/lxn/walk"
)

func (skinLayout *SkinLayout) setData(skinName string, skinUrl string) {
	if skinUrl != "" {
		skinLayout.LinkLabel.SetText("<a href=\"" + skinUrl + "\">" + skinName + "</a>")
	} else {
		skinLayout.LinkLabel.SetText(skinName)
	}
}

func loadSavedUser() (User, error) {
	var user User
	cred, err := wincred.GetGenericCredential("ValorantShopwatcher")
	if err == nil {
		decoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()
		blob, _ := decoder.Bytes(cred.CredentialBlob)
		if err != nil {
			log.Fatal(err)
		}
		s := strings.Split(string(blob), "\x00")
		if !isAccessTokenValid(s[2]) {
			getAccessToken()
			user = User{cred.UserName, s[0], s[1], globalStore.User.AccessToken}
		} else {
			user = User{cred.UserName, s[0], s[1], s[2]}
		}
	}
	return user, err
}

func saveUserData(user User) {
	cred := wincred.NewGenericCredential("ValorantShopwatcher")
	cred.Persist = wincred.PersistEnterprise
	cred.TargetAlias = "ValorantShopwatcher"
	cred.TargetName = "ValorantShopwatcher"
	encoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder()
	blob, _ := encoder.Bytes([]byte(user.Password + "\x00" + user.Region + "\x00" + user.AccessToken))
	cred.CredentialBlob = blob
	cred.UserName = user.Login
	err := cred.Write()
	if err != nil {
		walk.MsgBox(nil, "Error", "The app could not save your credentials", walk.MsgBoxIconError)
		globalStore.Ui.mainWindow.WindowBase.Synchronize(func() {
			globalStore.Ui.mainWindow.Show()
		})
	}
}

func getAccessToken() {
	body, _ := json.Marshal(AuthBody{Client_id: "play-valorant-web-prod", Nonce: 1, Redirect_uri: "https://playvalorant.com/opt_in", Response_type: "token id_token", Scope: "account openid"})
	req, _ := http.NewRequest("POST", "https://auth.riotgames.com/api/v1/authorization", bytes.NewBuffer(body))
	setRequestHeaders(req)
	res, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	body, _ = json.Marshal(UserBody{Type: "auth", Username: globalStore.User.Login, Password: globalStore.User.Password})
	req, _ = http.NewRequest("PUT", "https://auth.riotgames.com/api/v1/authorization", bytes.NewBuffer(body))
	setRequestHeaders(req)
	res, err = client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	data, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	var accessToken string
	var accessTokenContainer AccessTokenContainer
	json.Unmarshal(data, &accessTokenContainer)
	if accessTokenContainer.Type == "response" {
		accessToken = accessTokenContainer.Response.Parameters.Uri.Query().Get("access_token")
		globalStore.User.AccessToken = accessToken
	} else if accessTokenContainer.Type == "multifactor" {
		var mfaResponse MFAResponse
		json.Unmarshal(data, &mfaResponse)
		drawMfaModal(globalStore.Ui.mainWindow, mfaResponse.Multifactor.MultiFactorCodeLength)
	}
	select {
	case globalStore.Channels.NewToken <- globalStore.User.AccessToken:
		return
	default:
		return
	}

}

func seedUser() {
	var err error
	if globalStore.User.Login == "" {
		go drawUserform(globalStore.Ui.mainWindow)
		<-globalStore.Channels.LoginWindow
	}
	if !isAccessTokenValid(globalStore.User.AccessToken) {
		go getAccessToken()
	}
	<-globalStore.Channels.NewToken
	if err != nil {
		walk.MsgBox(nil, "Error", "The app could not call Riot servers", walk.MsgBoxIconError)
		globalStore.Ui.mainWindow.WindowBase.Synchronize(func() {
			globalStore.Ui.mainWindow.Show()
		})
	}
	globalStore.CurrentShop, err = fetchSkinsWithToken(globalStore.User.AccessToken)
	if err != nil {
		walk.MsgBox(nil, "Error", "The app could not fetch skins", walk.MsgBoxIconError)
		globalStore.Ui.mainWindow.WindowBase.Synchronize(func() {
			globalStore.Ui.mainWindow.Show()
		})
	}
	drawShop()
}

func isAccessTokenValid(accessToken string) bool {
	req, _ := http.NewRequest("POST", "https://entitlements.auth.riotgames.com/api/token/v1", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil || res.Status != "200 OK" {
		return false
	}
	return true
}
