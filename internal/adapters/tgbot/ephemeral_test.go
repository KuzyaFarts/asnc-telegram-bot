package tgbot

import "testing"

func TestReactionEmoji(t *testing.T) {
	if got := reactionEmoji(true); got != "🥰" {
		t.Fatalf("reactionEmoji(true) = %q, want smiling face with hearts", got)
	}
	if got := reactionEmoji(false); got != "🤡" {
		t.Fatalf("reactionEmoji(false) = %q, want clown", got)
	}
}
