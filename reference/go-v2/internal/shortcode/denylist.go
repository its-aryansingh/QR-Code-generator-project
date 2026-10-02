package shortcode

import "strings"

// Denylist contains offensive, profane, or reserved terms that must never appear in generated short codes.
var Denylist = map[string]struct{}{
	"ANUS": {}, "ARSE": {}, "ASSHOLE": {}, "BASTARD": {}, "BITCH": {},
	"BLOWJOB": {}, "BOOB": {}, "BULLSHIT": {}, "CLIT": {}, "COCK": {},
	"CRAP": {}, "CUM": {}, "CUNT": {}, "DAMN": {}, "DICK": {},
	"DILDO": {}, "DYKE": {}, "FAG": {}, "FAGGOT": {}, "FELLATIO": {},
	"FUCK": {}, "GODDAMN": {}, "HOMO": {}, "HORNY": {}, "JIZZ": {},
	"KAFIR": {}, "LABIA": {}, "MASTURBATE": {}, "NIGGER": {}, "NIGGA": {},
	"PENIS": {}, "PISS": {}, "POOP": {}, "PORN": {}, "PRICK": {},
	"PUSSY": {}, "QUEER": {}, "RAPE": {}, "RETARD": {}, "SCROTUM": {},
	"SEX": {}, "SHIT": {}, "SLUT": {}, "SMUT": {}, "SPIC": {},
	"TESTICLE": {}, "TIT": {}, "TWAT": {}, "VAGINA": {}, "VULVA": {},
	"WANKER": {}, "WHORE": {},
	// Reserved system codes
	"HEALTH": {}, "STATUS": {}, "METRIC": {}, "SYSTEM": {}, "ADMIN": {},
}

// normalisedDenylist contains Crockford Base32 normalised forms of the denylist terms.
var normalisedDenylist []string

func init() {
	normalisedDenylist = make([]string, 0, len(Denylist))
	for word := range Denylist {
		w := strings.ToUpper(word)
		w = strings.ReplaceAll(w, "O", "0")
		w = strings.ReplaceAll(w, "I", "1")
		w = strings.ReplaceAll(w, "L", "1")
		normalisedDenylist = append(normalisedDenylist, w)
	}
}

// ContainsDenylistedWord checks if the uppercase string s contains any forbidden words (raw or normalised).
func ContainsDenylistedWord(s string) bool {
	upper := strings.ToUpper(s)
	norm := upper
	norm = strings.ReplaceAll(norm, "O", "0")
	norm = strings.ReplaceAll(norm, "I", "1")
	norm = strings.ReplaceAll(norm, "L", "1")

	for _, word := range normalisedDenylist {
		if strings.Contains(norm, word) {
			return true
		}
	}
	for word := range Denylist {
		if strings.Contains(upper, word) {
			return true
		}
	}
	return false
}
