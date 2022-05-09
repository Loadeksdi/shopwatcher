package main

import (
	beeep "github.com/gen2brain/beeep"
)

func notify(Skin Skin) {
	beeep.Notify("New skin available", Skin.LocalizedNames[locale]+"is in your shop!", Skin.AssetPath)
}
