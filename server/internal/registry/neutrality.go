package registry

import "strings"

// homoglyphs — латиница→кириллица (анти-обфускация taboo-лексикона: «нaрушение» с лат. 'a').
var homoglyphs = map[rune]rune{
	'a': 'а', 'b': 'в', 'c': 'с', 'e': 'е', 'h': 'н', 'k': 'к',
	'm': 'м', 'o': 'о', 'p': 'р', 't': 'т', 'x': 'х', 'y': 'у',
}

// normalize — нормализация для taboo-матчинга: casefold + фолд гомоглифов.
// NFKC намеренно АППРОКСИМИРОВАН (без golang.org/x/text — без новой зависимости): для гардрейла
// достаточно ToLower + гомоглифы + матч по корню. Полный Unicode-NFKC — задел, если понадобится.
func normalize(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if c, ok := homoglyphs[r]; ok {
			b.WriteRune(c)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// FindTaboo возвращает совпавшие корни taboo-лексикона локали в тексте (после нормализации).
// Матч по корню (подстрока) ловит склонения/множественное число без полного морфоанализатора.
func (r *Registry) FindTaboo(loc Locale, text string) []string {
	norm := normalize(text)
	var hits []string
	for _, root := range r.taboo[loc] {
		if root == "" {
			continue
		}
		if strings.Contains(norm, normalize(root)) {
			hits = append(hits, root)
		}
	}
	return hits
}
