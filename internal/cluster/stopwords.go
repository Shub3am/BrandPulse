// Stopword list for the tokeniser: English plus the Hinglish function words
// that Indian D2C discourse actually runs on. Without the Hinglish half every
// cluster centres on "hai", "ka", "nahi" and "yaar" instead of on a topic.
//
// This file holds words only. It must not grow logic, and it must not import
// anything: the tokeniser owns the decision of what to do with a match.
package cluster

// stopwords is built once at init rather than per call. Tokenize runs over
// every mention in a window, so rebuilding a 200-entry map per document is a
// measurable waste on the pipeline's hot path.
var stopwords map[string]struct{}

// englishStopwords are the function words that carry no topic signal. One rune
// words are dropped by Tokenize regardless, so "a" and "i" are not listed.
var englishStopwords = []string{
	"about", "after", "again", "against", "all", "also", "am", "an", "and",
	"any", "are", "as", "at", "back", "be", "because", "been", "before",
	"being", "between", "both", "but", "by", "can", "could", "did", "do",
	"does", "doing", "done", "down", "during", "each", "even", "ever",
	"every", "few", "for", "from", "get", "got", "had", "has", "have",
	"having", "he", "her", "here", "him", "his", "how", "if", "in", "into",
	"is", "it", "its", "just", "me", "more", "most", "much", "my", "no",
	"nor", "not", "now", "of", "on", "one", "only", "or", "other", "our",
	"out", "over", "own", "really", "same", "she", "should", "so", "some",
	"still", "such", "than", "that", "the", "their", "them", "then", "there",
	"these", "they", "this", "those", "through", "to", "too", "under",
	"until", "up", "us", "very", "was", "we", "were", "what", "when",
	"where", "which", "while", "who", "why", "will", "with", "would", "you",
	"your",
}

// hinglishStopwords are Roman-script Hindi function words. The seed list is
// fixed by docs/tracks/B3-intel.md; anything added here changes clustering for
// every brand, so add a word only when it is a function word in every context
// it appears in. "accha" and "bekaar" are sentiment, not function, and are
// deliberately absent.
var hinglishStopwords = []string{
	"aur", "bahut", "bhai", "bhi", "bohot", "hai", "hain", "ka", "kaise",
	"kar", "karo", "ke", "ki", "ko", "kya", "kyun", "me", "mein", "nahi",
	"nahin", "par", "pe", "sab", "se", "tha", "thi", "toh", "vo", "wala",
	"wali", "woh", "yaar", "ye", "yeh",
}

func init() {
	stopwords = make(map[string]struct{}, len(englishStopwords)+len(hinglishStopwords))
	for _, w := range englishStopwords {
		stopwords[w] = struct{}{}
	}
	for _, w := range hinglishStopwords {
		stopwords[w] = struct{}{}
	}
}
