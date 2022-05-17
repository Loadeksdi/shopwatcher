package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/lxn/walk"
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

func setRequestHeaders(req *http.Request) *http.Request {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "RiotClient/43.0.1.4195386.4190634 rso-auth (Windows; 10;;Professional, x64)")
	return req
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
