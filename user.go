package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

type User struct {
	Login    string
	Password string
	Region   string
}

type AuthBody struct {
	Client_id     string `json:"client_id"`
	Nonce         int    `json:"nonce,string"`
	Redirect_uri  string `json:"redirect_uri"`
	Response_type string `json:"response_type"`
	Scope         string `json:"scope"`
}

type UserBody struct {
	Type     string `json:"type"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type MFABody struct {
	Type           string `json:"type"`
	Code           string `json:"code"`
	RememberDevice bool   `json:"rememberDevice"`
}

type Multifactor struct {
	Email                 string   `json:"email"`
	Method                string   `json:"method"`
	Methods               []string `json:"methods"`
	MultiFactorCodeLength int      `json:"multiFactorCodeLength"`
	MfaVersion            string   `json:"mfaVersion"`
}

type MFAResponse struct {
	Type            string      `json:"type"`
	Multifactor     Multifactor `json:"multifactor"`
	Country         string      `json:"country"`
	SecurityProfile string      `json:"securityProfile"`
}

type ParsedURL struct {
	*url.URL
}

type AccessTokenContainer struct {
	Type     string `json:"type"`
	Response struct {
		Mode       string `json:"mode"`
		Parameters struct {
			Uri ParsedURL `json:"uri"`
		} `json:"parameters"`
	} `json:"response"`
	Country string `json:"country"`
}

func (urlVar *ParsedURL) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}
	parsedUrl, err := url.Parse(s)
	if err != nil {
		return err
	}
	urlVar.URL = parsedUrl
	return nil
}

func loadSavedUser() (User, error) {
	var user User
	file, err := os.ReadFile("saves/user.json")
	if err != nil {
		return user, err
	}
	err = json.Unmarshal(file, &user)
	if err != nil {
		walk.MsgBox(nil, "Error", "The app could not load user information", walk.MsgBoxIconError)
	}
	return user, nil
}

func saveUserData(user User) {
	json, err := json.MarshalIndent(user, "", "  ")
	if err != nil {
		walk.MsgBox(nil, "Error", "The app could not save your login data", walk.MsgBoxIconError)
	}
	if _, err := os.Stat("saves"); os.IsNotExist(err) {
		os.Mkdir("saves", 0777)
	}
	err = ioutil.WriteFile("saves/user.json", json, 0644)
	if err != nil {
		walk.MsgBox(nil, "Error", "The app could not save your login data", walk.MsgBoxIconError)
	}
}

var userForm *walk.Dialog

func drawUserform(owner walk.Form) User {
	var outLELogin *walk.LineEdit
	var outLEPassword *walk.LineEdit
	var outCBRegion *walk.ComboBox
	var user User
	var dialog = Dialog{
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
					user = User{Login: outLELogin.Text(), Password: outLEPassword.Text(), Region: outCBRegion.Text()}
					_, err := checkUserCreds(user)
					if err != nil {
						walk.MsgBox(nil, "Error", "Invalid credentials", walk.MsgBoxIconError)
					}
					saveUserData(user)
				},
			},
		},
	}
	dialog.Run(owner)
	return user
}

func checkUserCreds(user User) (string, error) {
	body, _ := json.Marshal(AuthBody{Client_id: "play-valorant-web-prod", Nonce: 1, Redirect_uri: "https://playvalorant.com/opt_in", Response_type: "token id_token", Scope: "account openid"})
	req, _ := http.NewRequest("POST", "https://auth.riotgames.com/api/v1/authorization", bytes.NewBuffer(body))
	setRequestHeaders(req)
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ = json.Marshal(UserBody{Type: "auth", Username: user.Login, Password: user.Password})
	req, _ = http.NewRequest("PUT", "https://auth.riotgames.com/api/v1/authorization", bytes.NewBuffer(body))
	setRequestHeaders(req)
	res, err = client.Do(req)
	if err != nil {
		return "", err
	}
	var accessTokenContainer AccessTokenContainer
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	err = json.Unmarshal(data, &accessTokenContainer)
	if err == nil {
		accessToken := accessTokenContainer.Response.Parameters.Uri.Query().Get("access_token")
		fmt.Println(accessToken)
	} else {
		var mfaResponse MFAResponse
		err = json.Unmarshal(data, &mfaResponse)
		if err != nil {
			return "", err
		}
		MFAModal(userForm, mfaResponse.Multifactor.MultiFactorCodeLength)
	}
	return "ok", nil
}

func setRequestHeaders(req *http.Request) *http.Request {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "RiotClient/43.0.1.4195386.4190634 rso-auth (Windows; 10;;Professional, x64)")
	return req
}

func MFAModal(owner walk.Form, codeLength int) {
	var outLECode *walk.LineEdit
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
					err = json.NewDecoder(res.Body).Decode(&accessTokenContainer)
					if err != nil {
						walk.MsgBox(nil, "Error", "Couldn't connect with MFA", walk.MsgBoxIconError)
					}
					accessToken := accessTokenContainer.Response.Parameters.Uri.Query().Get("access_token")
					fmt.Println(accessToken)
				},
			},
		},
	}.Run(owner)
}
