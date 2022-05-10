package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

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
	FeaturedBundle struct {
		Bundle struct {
			ID          string `json:"ID"`
			DataAssetID string `json:"DataAssetID"`
			CurrencyID  string `json:"CurrencyID"`
			Items       []struct {
				Item struct {
					ItemTypeID string `json:"ItemTypeID"`
					ItemID     string `json:"ItemID"`
					Amount     int    `json:"Amount"`
				} `json:"Item"`
				BasePrice       int    `json:"BasePrice"`
				CurrencyID      string `json:"CurrencyID"`
				DiscountPercent int    `json:"DiscountPercent"`
				DiscountedPrice int    `json:"DiscountedPrice"`
				IsPromoItem     bool   `json:"IsPromoItem"`
			} `json:"Items"`
			DurationRemainingInSeconds int  `json:"DurationRemainingInSeconds"`
			WholesaleOnly              bool `json:"WholesaleOnly"`
		} `json:"Bundle"`
		Bundles []struct {
			ID          string `json:"ID"`
			DataAssetID string `json:"DataAssetID"`
			CurrencyID  string `json:"CurrencyID"`
			Items       []struct {
				Item struct {
					ItemTypeID string `json:"ItemTypeID"`
					ItemID     string `json:"ItemID"`
					Amount     int    `json:"Amount"`
				} `json:"Item"`
				BasePrice       int    `json:"BasePrice"`
				CurrencyID      string `json:"CurrencyID"`
				DiscountPercent int    `json:"DiscountPercent"`
				DiscountedPrice int    `json:"DiscountedPrice"`
				IsPromoItem     bool   `json:"IsPromoItem"`
			} `json:"Items"`
			DurationRemainingInSeconds int  `json:"DurationRemainingInSeconds"`
			WholesaleOnly              bool `json:"WholesaleOnly"`
		} `json:"Bundles"`
		BundleRemainingDurationInSeconds int `json:"BundleRemainingDurationInSeconds"`
	} `json:"FeaturedBundle"`
	SkinsPanelLayout struct {
		SingleItemOffers                           []string `json:"SingleItemOffers"`
		SingleItemOffersRemainingDurationInSeconds int      `json:"SingleItemOffersRemainingDurationInSeconds"`
	} `json:"SkinsPanelLayout"`
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
	cred, err := wincred.GetGenericCredential("ValorantShopwatcher")
    if err == nil {
		s := strings.Split(string(cred.CredentialBlob), ":")
		user = User{s[0], s[1], s[2]}
    } 
	return user, nil
}

func saveUserData(user User) {
	cred := wincred.NewGenericCredential("ValorantShopwatcher")
    cred.CredentialBlob = []byte(user.Login + ":" + user.Password + ":" + user.Region)
    err := cred.Write()
    if err != nil {
        walk.MsgBox(nil, "Error", "The app could not save your credentials", walk.MsgBoxIconError)
    }
}

var userForm *walk.Dialog

func drawUserform(owner walk.Form) (User, []Skin) {
	var outLELogin *walk.LineEdit
	var outLEPassword *walk.LineEdit
	var outCBRegion *walk.ComboBox
	var user User
	var skins []Skin
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
					accessToken, err := getAccessToken(user)
					if err != nil {
						walk.MsgBox(nil, "Error", "Invalid credentials", walk.MsgBoxIconError)
					}
					saveUserData(user)
					skins, _ = fetchSkinsWithToken(accessToken, user)
				},
			},
		},
	}
	dialog.Run(owner)
	return user, skins
}

func getAccessToken(user User) (string, error) {
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
	var mfaResponse MFAResponse
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	var accessToken string
	err = json.Unmarshal(data, &mfaResponse)
	if err == nil {
		accessToken = MFAModal(userForm, user, mfaResponse.Multifactor.MultiFactorCodeLength)
	} else {
		var accessTokenContainer AccessTokenContainer
		json.Unmarshal(data, &accessTokenContainer)
		accessToken = accessTokenContainer.Response.Parameters.Uri.Query().Get("access_token")
	}
	return accessToken, nil
}

func setRequestHeaders(req *http.Request) *http.Request {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "RiotClient/43.0.1.4195386.4190634 rso-auth (Windows; 10;;Professional, x64)")
	return req
}

func MFAModal(owner walk.Form, user User, codeLength int) string {
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
					re, _ := regexp.Compile("access_token=([^&]*)")
					accessTokenString := re.FindString(accessTokenContainer.Response.Parameters.Uri.Fragment)
					accessToken = accessTokenString[13:]
				},
			},
		},
	}.Run(owner)
	return accessToken
}

func fetchSkinsWithToken(accessToken string, user User) ([]Skin, error) {
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
	req, _ = http.NewRequest("GET", "https://pd." + user.Region + ".a.pvp.net/store/v2/storefront/" + userId.Sub, nil)
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
	var skinsInShop []Skin
	for _, tmpSkin := range shop.SkinsPanelLayout.SingleItemOffers {
		for _, skin := range globalStore.Ui.skinsListBox.AllSkins{
			if tmpSkin == skin.Id {
				skinsInShop = append(skinsInShop, skin)
			}
		}
	}
	return skinsInShop, nil
}
