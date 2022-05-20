package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/lxn/walk"
)

type User struct {
	Login       string
	Password    string
	Region      string
	AccessToken string
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
		Uuid          string `json:"uuid"`
		DisplayName   string `json:"displayName"`
		StreamedVideo string `json:"streamedVideo"`
	} `json:"data"`
}

type SkinLayout struct {
	LinkLabel *walk.LinkLabel
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
	Channels    struct {
		NewToken    chan string
		LoginWindow chan bool
		MFAToken    chan bool
	}
}

type Response struct {
	Skins SortedSkins `json:"skins"`
}

type Skin struct {
	Name           string
	LocalizedNames SynchronizedMap
	Id             string
	AssetName      string
	AssetPath      string
	Video          string
}

type SortedSkins []Skin
type SynchronizedMap struct {
	*sync.Map
}

type SkinNameVideo struct {
	Name  string
	Video string
}

func (s SortedSkins) Len() int {
	return len(s)
}

func (s SortedSkins) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s SortedSkins) Less(i, j int) bool {
	if _, ok := s[i].LocalizedNames.Load(locale); !ok {
		locale = "en-US"
	}
	res1, _ := s[i].LocalizedNames.Load(locale)
	res2, _ := s[j].LocalizedNames.Load(locale)
	localizedSlice := []string{res1.(string), res2.(string)}
	sort.Strings(localizedSlice)
	return localizedSlice[0] == res1
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

func (syncMap *SynchronizedMap) UnmarshalJSON(data []byte) error {
	var dataMap map[string]string
	syncMap.Map = &sync.Map{}
	err := json.Unmarshal(data, &dataMap)
	if err != nil {
		return err
	}
	for key, value := range dataMap {
		syncMap.Store(key, value)
	}
	return nil
}

func (syncMap *SynchronizedMap) MarshalJSON() ([]byte, error) {
	dataMap := make(map[string]string)
	syncMap.Range(func(key any, value any) bool {
		dataMap[key.(string)] = value.(string)
		return true
	})
	return json.Marshal(dataMap)
}
