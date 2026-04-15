package reputation

import (
	"regexp"

	"github.com/go-telegram/bot/models"
)

var trigger = regexp.MustCompile(`(?i)^([+]+|[-]+)(rep|реп)(?:$|\s|[[:punct:]])`)

type Trigger struct {
	Delta int
}

type stickerInfo struct {
	Name  string
	Value int
}

var (
	plusRepStickerIDs = map[string]stickerInfo{
		"AgADlYwAArYY6Uo": {
			Name:  "MR P.K.",
			Value: 1},
		"AgADUKMAAnEM8Uo": {
			Name:  "Kopatich",
			Value: 3},
	}
	minusRepStickerIDs = map[string]stickerInfo{

		"AgADTpAAAiB38Eo": {
			Name:  "MR P.K.",
			Value: -1},
		"AgADWJcAAvBI-Uo": {
			Name:  "Losyash",
			Value: -3},
	}
)

func Parse(msg *models.Message) *Trigger {
	if msg.Sticker != nil {
		return parseSticker(msg.Sticker)
	}
	return parseText(msg.Text)
}

func parseSticker(sticker *models.Sticker) *Trigger {
	if _, ok := plusRepStickerIDs[sticker.FileUniqueID]; ok {
		return &Trigger{Delta: plusRepStickerIDs[sticker.FileUniqueID].Value}
	}
	if _, ok := minusRepStickerIDs[sticker.FileUniqueID]; ok {
		return &Trigger{Delta: minusRepStickerIDs[sticker.FileUniqueID].Value}
	}
	return nil
}

func parseText(text string) *Trigger {
	m := trigger.FindStringSubmatch(text)
	if m == nil {
		return nil
	}
	signs := m[1]
	n := len(signs)
	if signs[0] == '-' {
		n = -n
	}
	return &Trigger{Delta: n}
}
