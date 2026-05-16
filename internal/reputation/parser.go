package reputation

import (
	"regexp"
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
			Value: 1,
		},
		"AgADUKMAAnEM8Uo": {
			Name:  "Kopatich",
			Value: 3,
		},
	}
	minusRepStickerIDs = map[string]stickerInfo{

		"AgADTpAAAiB38Eo": {
			Name:  "MR P.K.",
			Value: -1,
		},
		"AgADWJcAAvBI-Uo": {
			Name:  "Losyash",
			Value: -3,
		},
	}
)

func ParseSticker(fileUniqueID string) *Trigger {
	if info, ok := plusRepStickerIDs[fileUniqueID]; ok {
		return &Trigger{Delta: info.Value}
	}
	if info, ok := minusRepStickerIDs[fileUniqueID]; ok {
		return &Trigger{Delta: info.Value}
	}
	return nil
}

func ParseText(text string) *Trigger {
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
