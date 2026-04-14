package reputation

import (
	"regexp"

	"github.com/go-telegram/bot/models"
)

var trigger = regexp.MustCompile(`(?i)^([+]+|[-]+)(rep|реп)(?:$|\s|[[:punct:]])`)

type Trigger struct {
	Delta int
}

const (
	plusRepStickerID  = `AgADlYwAArYY6Uo`
	minusRepStickerID = `AgADTpAAAiB38Eo`
)

func Parse(msg *models.Message) *Trigger {
	if msg.Sticker != nil {
		return parseSticker(msg.Sticker)
	}
	return parseText(msg.Text)
}

func parseSticker(sticker *models.Sticker) *Trigger {
	if sticker.FileUniqueID == plusRepStickerID {
		return &Trigger{Delta: 1}
	} else if sticker.FileUniqueID == minusRepStickerID {
		return &Trigger{Delta: -1}
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
