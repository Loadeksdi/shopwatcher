package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/text/encoding/unicode"

	"github.com/danieljoos/wincred"
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

type EntitlementResponse struct {
	EntitlementsToken string `json:"entitlements_token"`
}

type UserId struct {
	Sub string `json:"sub"`
}

type Shop struct {
	SkinsPanelLayout struct {
		SingleItemOffers                           []string `json:"SingleItemOffers"`
		SingleItemOffersRemainingDurationInSeconds int      `json:"SingleItemOffersRemainingDurationInSeconds"`
	} `json:"SkinsPanelLayout"`
}

type SkinDataResponse struct {
	Data struct {
		Uuid        string `json:"uuid"`
		DisplayName string `json:"displayName"`
	} `json:"data"`
}

type SkinLayout struct {
	Label *walk.Label
	Image *walk.ImageView
}

func (skinLayout *SkinLayout) setData(skinName string, skinImage string) {
	skinLayout.Label.SetText(skinName)
	// skinLayout.Image.SetImage(walk.Image{walk.NewBitmap(skinImage)})
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
				},
				// ImageView{
				// 	AssignTo: &skinLayout.Image,
				// 	Image: "",
				// },
			},
		})
	}
	return composites
}

func (urlVar *ParsedURL) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}
	s = strings.Replace(s, "#", "?", -1)
	parsedUrl, err := url.Parse(s)
	if err != nil {
		return err
	}
	urlVar.URL = parsedUrl
	return nil
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

var userForm *walk.Dialog

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
		accessToken = mfaModal(userForm, mfaResponse.Multifactor.MultiFactorCodeLength)
	} else {
		return "", errors.New(accessTokenContainer.Type)
	}
	return accessToken, nil
}

func setRequestHeaders(req *http.Request) *http.Request {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "RiotClient/43.0.1.4195386.4190634 rso-auth (Windows; 10;;Professional, x64)")
	return req
}

func mfaModal(owner walk.Form, codeLength int) string {
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

func fetchSkinsWithToken(accessToken string) ([]Skin, error) {
	req, _ := http.NewRequest("POST", "https://entitlements.auth.riotgames.com/api/token/v1", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		walk.MsgBox(nil, "Error", "Couldn't fetch account information", walk.MsgBoxIconError)
	}
	defer res.Body.Close()
	var entitlementResponse EntitlementResponse
	json.NewDecoder(res.Body).Decode(&entitlementResponse)
	req, _ = http.NewRequest("POST", "https://auth.riotgames.com/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Riot-Entitlements-JWT", entitlementResponse.EntitlementsToken)
	res, err = client.Do(req)
	if err != nil {
		walk.MsgBox(nil, "Error", "Couldn't fetch account information", walk.MsgBoxIconError)
	}
	defer res.Body.Close()
	var userId UserId
	json.NewDecoder(res.Body).Decode(&userId)
	req, _ = http.NewRequest("GET", "https://pd."+globalStore.User.Region+".a.pvp.net/store/v2/storefront/"+userId.Sub, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Riot-Entitlements-JWT", entitlementResponse.EntitlementsToken)
	res, err = client.Do(req)
	if err != nil {
		walk.MsgBox(nil, "Error", "Couldn't fetch account information", walk.MsgBoxIconError)
	}
	defer res.Body.Close()
	var shop Shop
	err = json.NewDecoder(res.Body).Decode(&shop)
	if err != nil {
		walk.MsgBox(nil, "Error", "Couldn't fetch store information", walk.MsgBoxIconError)
	}
	skinsInShop, err := getSkinsInShop(shop, accessToken, entitlementResponse.EntitlementsToken)
	if err != nil {
		walk.MsgBox(nil, "Error", "Couldn't fetch skins", walk.MsgBoxIconError)
	}
	return skinsInShop, nil
}

func getSkinsInShop(shop Shop, accessToken string, entitlementsToken string) ([]Skin, error) {
	var skinsInShopIds []string
	var skinsInShop []Skin
	for _, tmpSkin := range shop.SkinsPanelLayout.SingleItemOffers {
		req, _ := http.NewRequest("GET", "https://valorant-api.com/v1/weapons/skins/"+tmpSkin, nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Riot-Entitlements-JWT", entitlementsToken)
		res, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		var skinDataResponse SkinDataResponse
		err = json.NewDecoder(res.Body).Decode(&skinDataResponse)
		if err != nil {
			return nil, err
		}
		skinsInShopIds = append(skinsInShopIds, strings.ToUpper(skinDataResponse.Data.Uuid))
	}
	for _, skin := range globalStore.Ui.skinsListBox.AllSkins {
		for _, skinId := range skinsInShopIds {
			if skinId == skin.Id {
				skinsInShop = append(skinsInShop, skin)
			}
		}
	}
	return skinsInShop, nil
}
