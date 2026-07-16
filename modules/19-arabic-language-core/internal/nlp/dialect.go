package nlp

import (
	"strings"
)

// msaKeywords returns Modern Standard Arabic formal keywords.
func msaKeywords() map[string]float64 {
	return map[string]float64{
		"إن": 3.0, "أن": 3.0, "الذي": 2.5, "التي": 2.5, "هناك": 2.0,
		"لا": 2.0, "هذا": 2.0, "هذه": 2.0, "على": 2.0, "إلى": 2.0,
		"عن": 2.0, "ما": 2.0, "من": 2.0, "هل": 2.0, "قد": 2.0,
		"فإن": 3.0, "وإن": 2.5, "أي": 2.0, "كل": 2.0, "بأن": 2.5,
		"ذلك": 2.0, "تلك": 2.0, "حيث": 2.0, "عند": 2.0, "بين": 2.0,
		"خلال": 2.0, "رغم": 2.0, "بالإضافة": 2.5, "مع": 2.0, "لكن": 2.0,
		"لذلك": 2.5, "نظراً": 2.5, "يُعتبر": 3.0, "تتمثل": 2.5,
		"يتضمن": 2.5, "يتطلب": 2.5, "يُمكن": 2.0, "يجب": 2.0,
	}
}

// saudiKeywords returns Gulf/Saudi dialect keywords.
func saudiKeywords() map[string]float64 {
	return map[string]float64{
		"شفت": 4.0, "وا": 3.0, "يلا": 3.0, "هبل": 2.5,
		"على": 2.0, "هالحياة": 3.5, "هاي": 3.0, "هذافي": 3.0,
		"شنو": 4.0, "شكن": 4.0, "وين": 3.5, "قولي": 3.0,
		"ليش": 3.5, "هون": 3.0, "هيك": 3.5, "هسه": 3.0,
		"كفاية": 3.0, "ياخذ": 2.5, "تقول": 2.0, "عيب": 2.5,
		"والله": 2.0, "بحاول": 2.5, "إن شاء الله": 2.0, "ماشي": 3.0,
		"تمام": 2.0, "ياغالي": 3.0, "هسا": 3.0, "هالمرة": 3.0,
		"هالساعة": 3.0, "هالبيت": 3.0, "هالناس": 3.0, "هالكليشيه": 3.5,
		"يهدلك": 2.5, "أبوي": 3.0, "بابا": 2.0,
		"جدا": 2.0, "يلا بينا": 3.0, "وش": 3.5, "هذي": 3.0,
	}
}

// emiratiKeywords returns Emirati dialect keywords.
func emiratiKeywords() map[string]float64 {
	return map[string]float64{
		"يوي": 4.0, "هلا": 3.0, "زبن": 3.5, "خلاص": 2.5,
		"وش": 3.0, "قرب": 3.0, "هون": 2.5, "شكراً": 2.0,
		"منور": 2.5, "تسلم": 2.0, "هالعيب": 3.0, "كثير": 2.0,
		"يلا": 2.5, "هاللي": 3.0, "هالمكان": 3.0, "هالسوالف": 3.0,
		"عيب": 2.5, "ههه": 2.0, "هذي": 2.5, "هالزبن": 3.5,
		"أهلاً": 2.0, "بسلامتك": 2.5, "الله": 2.0, "ماشي": 2.5,
		"تمام": 2.0, "حياك": 3.0, "هالك": 3.0, "ياخلو": 3.5,
		"هالدول": 3.0, "ياقلبي": 3.0, "هالأمري": 3.0,
	}
}

// kuwaitiKeywords returns Kuwaiti dialect keywords.
func kuwaitiKeywords() map[string]float64 {
	return map[string]float64{
		"وا": 3.5, "ههه": 2.5, "هسه": 3.5, "هيك": 3.5,
		"قولي": 3.0, "شنو": 3.5, "ليش": 3.0, "وين": 3.0,
		"هالحين": 3.5, "هالمرة": 3.0, "هالناس": 3.0, "هالأمر": 3.0,
		"ماشي": 3.0, "هاللي": 3.0, "يلا": 2.5,
		"عيب": 2.5, "هههه": 2.0, "هالك": 3.0, "أبوي": 3.0,
		"ياغالي": 3.0, "هالسوالف": 3.0, "هالزبون": 3.0, "هالفرحه": 3.0,
		"تمام": 2.0, "هالبيت": 3.0, "هالمشروع": 3.0, "هالوقت": 2.5,
		"حبيت": 2.5, "هالحياة": 3.0, "هالعيب": 3.0,
	}
}

// bahrainiKeywords returns Bahraini dialect keywords.
func bahrainiKeywords() map[string]float64 {
	return map[string]float64{
		"وش": 3.5, "شو": 3.0, "هسه": 3.0, "هيك": 3.5,
		"قولي": 2.5, "شنو": 3.5, "ليش": 2.5, "وين": 3.0,
		"هالحين": 3.0, "هالمرة": 3.0, "هالناس": 3.0, "هالأمر": 3.0,
		"ماشي": 2.5, "هاللي": 3.0, "يلا": 2.5,
		"عيب": 2.5, "ههه": 2.0, "هالك": 3.0, "أبوي": 2.5,
		"ياغالي": 3.0, "هالسوالف": 3.0, "هالزبون": 3.0, "هالفرحه": 3.0,
		"تمام": 2.0, "هالبيت": 3.0, "هالمشروع": 3.0, "هالوقت": 2.5,
		"حبيت": 2.5, "هالحياة": 3.0, "هالعيب": 3.0, "ياأخي": 2.5,
	}
}

// qatariKeywords returns Qatari dialect keywords.
func qatariKeywords() map[string]float64 {
	return map[string]float64{
		"وا": 3.5, "ههه": 2.5, "هسه": 3.5, "هيك": 3.5,
		"قولي": 3.0, "شنو": 3.5, "ليش": 3.0, "وين": 3.0,
		"هالحين": 3.5, "هالمرة": 3.0, "هالناس": 3.0, "هالأمر": 3.0,
		"ماشي": 3.0, "هاللي": 3.0, "يلا": 2.5,
		"عيب": 2.5, "هههه": 2.0, "هالك": 3.0, "أبوي": 3.0,
		"ياغالي": 3.0, "هالسوالف": 3.0, "هالزبون": 3.0, "هالفرحه": 3.0,
		"تمام": 2.0, "هالبيت": 3.0, "هالمشروع": 3.0, "هالوقت": 2.5,
		"حبيت": 2.5, "هالحياة": 3.0, "هالعيب": 3.0, "هالغالية": 3.0,
	}
}

// omaniKeywords returns Omani dialect keywords.
func omaniKeywords() map[string]float64 {
	return map[string]float64{
		"شفت": 3.5, "وش": 3.0, "شنو": 3.0, "شو": 2.5,
		"قولي": 2.5, "ليش": 2.5, "وين": 3.0, "هون": 2.5,
		"هيك": 3.0, "هسه": 3.0, "هالحين": 3.0, "هالمرة": 3.0,
		"ماشي": 2.5, "هاللي": 2.5, "يلا": 2.0,
		"عيب": 2.5, "هالك": 2.5, "أبوي": 2.5, "ياغالي": 2.5,
		"تمام": 2.0, "هالبيت": 3.0, "هالناس": 3.0, "هالوقت": 2.0,
		"حبيت": 2.0, "هالحياة": 3.0, "هالعيب": 3.0, "ياأخي": 2.0,
		"كثير": 2.0, "شكراً": 2.0, "تسلم": 2.0, "منور": 2.0,
		"هالمكان": 3.0, "هالسوالف": 3.0, "هالزبون": 3.0,
	}
}

// egyptianKeywords returns Egyptian dialect keywords.
func egyptianKeywords() map[string]float64 {
	return map[string]float64{
		"إزاي": 4.0, "ايه": 3.5, "عندك": 3.0, "يعني": 2.5,
		"مش": 3.5, "على": 2.0, "عشان": 3.5, "خلاص": 3.0,
		"ييجي": 3.0, "تروح": 2.5, "هقولك": 3.0, "تعال": 2.5,
		"ياخي": 2.5, "عيب": 2.0, "أنا": 1.5, "انت": 1.5,
		"هو": 2.0, "ده": 2.5, "دي": 2.5, "اللي": 2.0,
		"محتاج": 2.5, "مفيش": 3.0, "كده": 3.0,
		"طب": 2.5, "يلا": 2.5, "غداً": 2.0, "حاضر": 2.5,
	}
}

// levantKeywords returns Levantine dialect keywords.
func levantKeywords() map[string]float64 {
	return map[string]float64{
		"شلون": 4.0, "شلونك": 4.0, "وايد": 3.5, "هسه": 3.0,
		"شنو": 3.0, "وش": 3.0, "قوي": 3.0, "ياليت": 3.0,
		"تعبان": 3.0, "حلو": 2.5, "يعني": 2.0, "عندك": 2.5,
		"عشان": 3.0, "كاييـس": 3.5, "حبيبي": 2.5,
		"يازين": 3.5, "أهلاً": 2.0, "حاضر": 2.5, "إن شاء الله": 2.0,
		"ماشي": 2.5, "كثير": 2.0, "تمام": 2.0, "ياغالي": 2.5,
		"هالمرة": 2.5, "هالناس": 3.0, "هالأمر": 3.0,
	}
}

// moroccanKeywords returns Moroccan (Darija) dialect keywords.
func moroccanKeywords() map[string]float64 {
	return map[string]float64{
		"wach": 4.0, "كيفاش": 4.0, "بغيت": 3.5, "واش": 4.0,
		"كتاير": 3.0, "هاد": 3.5, "هادي": 3.5, "دير": 3.0,
		"غادي": 3.5, "جا": 2.5, "شنو": 2.5, "شكون": 3.5,
		"فيم": 3.0, "ديرها": 3.0, "حنا": 2.0, "بلاصة": 2.5,
		"خايب": 3.0, "صافي": 3.0, "أوف": 2.5, "حبيبي": 2.0,
		"ياخوي": 3.0, "عندك": 2.0, "حاضر": 2.0, "منور": 2.0,
		"هادشي": 3.5,
	}
}

// dialectKeywords maps dialect codes to their distinguishing keyword sets.
var dialectKeywords = map[string]map[string]float64{
	"msa":       msaKeywords(),
	"saudi":     saudiKeywords(),
	"emirati":   emiratiKeywords(),
	"kuwaiti":   kuwaitiKeywords(),
	"bahraini":  bahrainiKeywords(),
	"qatari":    qatariKeywords(),
	"omani":     omaniKeywords(),
	"egyptian":  egyptianKeywords(),
	"levantine": levantKeywords(),
	"moroccan":  moroccanKeywords(),
}

// KeywordResult holds the scoring output of dialect detection.
type KeywordResult struct {
	Dialect string  `json:"dialect"`
	Score   float64 `json:"score"`
}

// DialectDetectResult is the response for dialect detection.
type DialectDetectResult struct {
	Dialect    string         `json:"dialect"`
	Confidence float64        `json:"confidence"`
	AllScores  []KeywordResult `json:"all_scores"`
	IsMSA      bool           `json:"is_msa"`
}

// DetectDialect determines the dialect of the given Arabic text.
func DetectDialect(text string) DialectDetectResult {
	if text == "" {
		return DialectDetectResult{
			Dialect:    "unknown",
			Confidence: 0,
			AllScores:  nil,
			IsMSA:      true,
		}
	}

	// Normalize basic text first
	text = strings.ToLower(text)
	// Remove tashkeel for matching
	text = stripTashkeel(text)

	tokens := tokenizeArabic(text)
	if len(tokens) == 0 {
		return DialectDetectResult{
			Dialect:    "unknown",
			Confidence: 0,
			AllScores:  nil,
			IsMSA:      true,
		}
	}

	scores := make([]KeywordResult, 0)
	totalScore := 0.0
	bestScore := 0.0
	bestDialect := "msa"

	// Compute average keyword weight across all dialects for penalty normalization.
	totalKeywordCount := 0
	totalWeightSum := 0.0
	dialectCount := 0
	for _, keywords := range dialectKeywords {
		dialectCount++
		for _, w := range keywords {
			totalWeightSum += w
			totalKeywordCount++
		}
	}
	avgWeight := totalWeightSum / float64(totalKeywordCount)
	uniformPenalty := float64(dialectCount) * avgWeight

	for dialect, keywords := range dialectKeywords {
		s := scoreDialect(tokens, keywords)
		scores = append(scores, KeywordResult{Dialect: dialect, Score: s})
		totalScore += s
		if s > bestScore {
			bestScore = s
			bestDialect = dialect
		}
	}

	confidence := 0.0
	if totalScore > 0 {
		confidence = bestScore / (bestScore + uniformPenalty)
	}

	// MSA is the default / high-confidence baseline
	isMSA := bestDialect == "msa" || confidence < 0.1

	// Boost MSA if no strong dialect signal and text contains formal markers
	if !isMSA && confidence < 0.3 {
		isMSA = true
		bestDialect = "msa"
		// Recalculate MSA position in scores
		for i := range scores {
			if scores[i].Dialect == "msa" {
				scores[i].Score = scores[i].Score * 1.5
				break
			}
		}
	}

	// Clamp confidence
	if confidence > 1.0 {
		confidence = 1.0
	}

	return DialectDetectResult{
		Dialect:    bestDialect,
		Confidence: confidence,
		AllScores:  scores,
		IsMSA:      isMSA,
	}
}

// scoreDialect computes a weighted match score for a dialect's keyword dictionary.
func scoreDialect(tokens []string, keywords map[string]float64) float64 {
	score := 0.0
	tokenSet := make(map[string]bool, len(tokens))
	for _, t := range tokens {
		tokenSet[t] = true
	}
	for keyword, weight := range keywords {
		if tokenSet[keyword] {
			score += weight
		}
	}
	return score
}

// stripTashkeel removes diacritical marks from text.
func stripTashkeel(text string) string {
	var b strings.Builder
	for _, r := range text {
		if !isTashkeel(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// tokenizeArabic splits Arabic text into word tokens.
func tokenizeArabic(text string) []string {
	runes := []rune(text)
	var tokens []string
	var current strings.Builder

	for _, r := range runes {
		if isArabicWordChar(r) {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

// isArabicWordChar checks if a rune is an Arabic word character.
func isArabicWordChar(r rune) bool {
	return (r >= 0x0600 && r <= 0x06FF) ||
		(r >= 0x0750 && r <= 0x077F) ||
		(r >= 0x08A0 && r <= 0x08FF) ||
		(r >= 0xFB50 && r <= 0xFDFF) ||
		(r >= 0xFE70 && r <= 0xFEFF) ||
		(r >= 0x0030 && r <= 0x0039) // Arabic numerals 0-9
}