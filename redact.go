package main

import "hash/fnv"

// Locked tweaks are sent to the window with their text scrambled, so people on the free version
// can see how much is waiting for them but can't read (or look up) what each one does.

// tweakUI is a tweak as the window sees it.
type tweakUI struct {
	Tweak
	Changes  []string `json:"changes"`
	Pro      bool     `json:"pro"`
	Redacted bool     `json:"redacted,omitempty"`
}

const fillerWords = "lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt ut labore et dolore magna aliqua enim ad minim veniam quis nostrud exercitation ullamco laboris nisi aliquip ex ea commodo consequat"

func scramble(seed, text string) string {
	if text == "" {
		return ""
	}
	h := fnv.New32a()
	h.Write([]byte(seed))
	pos := int(h.Sum32() % uint32(len(fillerWords)))
	out := make([]byte, 0, len(text))
	for i := 0; i < len(text); i++ {
		c := fillerWords[(pos+i)%len(fillerWords)]
		if text[i] == ' ' {
			c = ' '
		}
		out = append(out, c)
	}
	return string(out)
}

func redactTweak(u tweakUI) tweakUI {
	u.Title = scramble(u.ID+"t", u.Title)
	u.Desc = scramble(u.ID+"d", u.Desc)
	u.Benefit = scramble(u.ID+"b", u.Benefit)
	u.Note = ""
	u.Changes = nil
	u.Actions = nil
	u.Redacted = true
	return u
}
