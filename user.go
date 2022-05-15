package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/text/encoding/unicode"

	"github.com/danieljoos/wincred"
	"github.com/lxn/walk"
)

func (skinLayout *SkinLayout) setData(skinName string, skinImage string) {
	skinLayout.Label.SetText(skinName)
	// skinLayout.Image.SetImage(walk.Image{walk.NewBitmap(skinImage)})
}

func loadSavedUser() (User, error) {
	var user User
	cred, err := wincred.GetGenericCredential("ValorantShopwatcher")
	if err == nil {
		decoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()
		blob, _ := decoder.Bytes(cred.CredentialBlob)
		if err != nil {
			fmt.Println(err)
		}
		s := strings.Split(string(blob), "\x00")
		user = User{cred.UserName, s[0], s[1]}
	}
	return user, err
}

func saveUserData(user User) {
	cred := wincred.NewGenericCredential("ValorantShopwatcher")
	cred.Persist = wincred.PersistEnterprise
	cred.TargetAlias = "ValorantShopwatcher"
	cred.TargetName = "ValorantShopwatcher"
	encoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder()
	blob, _ := encoder.Bytes([]byte(user.Password + "\x00" + user.Region))
	cred.CredentialBlob = blob
	cred.UserName = user.Login
	err := cred.Write()
	if err != nil {
		walk.MsgBox(nil, "Error", "The app could not save your credentials", walk.MsgBoxIconError)
	}
}

func getAccessToken() (string, error) {
	body, _ := json.Marshal(AuthBody{Client_id: "play-valorant-web-prod", Nonce: 1, Redirect_uri: "https://playvalorant.com/opt_in", Response_type: "token id_token", Scope: "account openid"})
	req, _ := http.NewRequest("POST", "https://auth.riotgames.com/api/v1/authorization", bytes.NewBuffer(body))
	setRequestHeaders(req)
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ = json.Marshal(UserBody{Type: "auth", Username: globalStore.User.Login, Password: globalStore.User.Password})
	req, _ = http.NewRequest("PUT", "https://auth.riotgames.com/api/v1/authorization", bytes.NewBuffer(body))
	setRequestHeaders(req)
	res, err = client.Do(req)
	if err != nil {
		return "", err
	}
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	var accessToken string
	var accessTokenContainer AccessTokenContainer
	json.Unmarshal(data, &accessTokenContainer)
	if accessTokenContainer.Type == "response" {
		accessToken = accessTokenContainer.Response.Parameters.Uri.Query().Get("access_token")
	} else if accessTokenContainer.Type == "multifactor" {
		var mfaResponse MFAResponse
		json.Unmarshal(data, &mfaResponse)
		accessToken = drawMfaModal(userForm, mfaResponse.Multifactor.MultiFactorCodeLength)
	} else {
		return "", errors.New(accessTokenContainer.Type)
	}
	return accessToken, nil
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
}
