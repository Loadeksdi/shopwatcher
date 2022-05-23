package main

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"os"

	"github.com/lxn/walk"
)

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

func fetchSkins() ([]Skin, error) {
	req, _ := http.NewRequest("GET", "https://eu.api.riotgames.com/val/content/v1/contents?locale="+locale, nil)
	apiKey, err := base64.StdEncoding.DecodeString("UkdBUEktMzc4MzdmYTItZmM1NS00MzYxLWE3NmUtNDdiNWE0YzlmNTE1")
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

func getSkinsInShop(shop Shop, accessToken string, entitlementsToken string) ([]Skin, error) {
	var skinsInShopNameVideos []SkinNameVideo
	var skinsInShop []Skin
	for _, tmpSkin := range shop.SkinsPanelLayout.SingleItemOffers {
		req, _ := http.NewRequest("GET", "https://valorant-api.com/v1/weapons/skinlevels/"+tmpSkin, nil)
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
		skinsInShopNameVideos = append(skinsInShopNameVideos, SkinNameVideo{Name: skinDataResponse.Data.DisplayName, Video: skinDataResponse.Data.StreamedVideo})
	}
	for _, skin := range globalStore.Ui.skinsListBox.AllSkins {
		for _, skinObject := range skinsInShopNameVideos {
			if skinObject.Name == skin.Name {
				skin.Video = skinObject.Video
				skinsInShop = append(skinsInShop, skin)
			}
		}
	}
	return skinsInShop, nil
}

func fetchSkinsWithToken(accessToken string) ([]Skin, error) {
	req, _ := http.NewRequest("POST", "https://entitlements.auth.riotgames.com/api/token/v1", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
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
