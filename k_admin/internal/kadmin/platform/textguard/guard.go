// Package textguard blocks user-supplied display text (file names, account
// names, titles and descriptions) that contains a word from a curated
// prohibited-word list.
//
// The list is a UTF-8 text file with one word per line; empty lines and lines
// starting with '#' are ignored. Both the dictionary and the incoming text are
// folded to a "skeleton" before comparison, so the usual evasions collapse onto
// the same key: full-width letters (NFKC), upper/lower case, inserted spaces,
// punctuation, stars, dots and zero-width characters, and simple leet digits
// (p0rn == porn, s3x == sex).
//
// Matching is substring-based over the skeleton: 色 情 / 色.情 / 色*情 all hit
// 色情. That is deliberate — it also means a dictionary word can match across
// a natural word boundary (方法轮换 contains 法轮), so the list must stay
// high-precision rather than broad.
package textguard

import (
	"log"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// minWordRunes drops one-rune entries: a single character (色, 赌) would match
// almost every title and is never the intent of a word list.
const minWordRunes = 2

// Field is one user-editable value to check, with the label used in the
// rejection message ("文件名包含违规内容").
type Field struct {
	Label string
	Value string
}

// Guard holds the loaded word list. It is safe for concurrent use and reloads
// its file when the on-disk version changes, so operators can edit the list
// without restarting the service.
type Guard struct {
	mu      sync.Mutex
	path    string
	pinned  bool
	words   []string
	loaded  bool
	modTime time.Time
	size    int64
	lastErr string
}

// New returns a guard with no word list; SetPath supplies one.
func New() *Guard { return &Guard{loaded: true} }

// FromWords returns a guard pinned to an in-memory list (tests, fixtures);
// SetPath is ignored so a config path can never overwrite the pinned words.
func FromWords(words []string) *Guard {
	guard := &Guard{loaded: true, pinned: true}
	guard.mu.Lock()
	guard.setWordsLocked(words)
	guard.mu.Unlock()
	return guard
}

// SetPath points the guard at a word list file. An empty path clears the list.
// The file is read lazily on the next check and re-read whenever its size or
// modification time changes.
func (g *Guard) SetPath(path string) {
	if g.pinned {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.path != path {
		g.path = path
		g.loaded = false
		g.modTime = time.Time{}
		g.size = 0
		if path == "" {
			g.words = nil
			g.loaded = true
			return
		}
	}
	g.refreshLocked()
}

// Words returns the normalized, deduplicated dictionary (for diagnostics).
func (g *Guard) Words() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.refreshLocked()
	return append([]string(nil), g.words...)
}

// Check reports the first dictionary word contained in text.
func (g *Guard) Check(text string) (string, bool) {
	g.mu.Lock()
	g.refreshLocked()
	words := g.words
	g.mu.Unlock()
	if len(words) == 0 {
		return "", false
	}
	skeleton := normalize(text)
	if skeleton == "" {
		return "", false
	}
	for _, word := range words {
		if strings.Contains(skeleton, word) {
			return word, true
		}
	}
	return "", false
}

// CheckFields reports the label and word of the first matching field.
func (g *Guard) CheckFields(fields []Field) (label, word string, hit bool) {
	for _, field := range fields {
		if matched, ok := g.Check(field.Value); ok {
			return field.Label, matched, true
		}
	}
	return "", "", false
}

// refreshLocked (re)loads the dictionary when the file appeared, disappeared or
// changed. A missing or unreadable file never blocks writes: the guard keeps an
// empty list and logs the reason once per distinct error text.
func (g *Guard) refreshLocked() {
	if g.pinned || g.path == "" {
		return
	}
	info, err := os.Stat(g.path)
	if err != nil {
		if g.lastErr != err.Error() {
			log.Printf("textguard: 敏感词词表不可用 %s: %v（本次放行，请修复配置）", g.path, err)
			g.lastErr = err.Error()
		}
		g.words = nil
		g.loaded = true
		g.modTime = time.Time{}
		g.size = 0
		return
	}
	if g.loaded && info.Size() == g.size && info.ModTime().Equal(g.modTime) {
		return
	}
	content, readErr := os.ReadFile(g.path)
	if readErr != nil {
		if g.lastErr != readErr.Error() {
			log.Printf("textguard: 敏感词词表读取失败 %s: %v（本次放行，请修复配置）", g.path, readErr)
			g.lastErr = readErr.Error()
		}
		return
	}
	g.loaded = true
	g.modTime = info.ModTime()
	g.size = info.Size()
	g.lastErr = ""
	g.setWordsLocked(strings.Split(string(content), "\n"))
}

func (g *Guard) setWordsLocked(lines []string) {
	words := make([]string, 0, len(lines))
	seen := make(map[string]bool, len(lines))
	for _, line := range lines {
		if index := strings.IndexByte(line, '#'); index >= 0 {
			line = line[:index]
		}
		word := normalize(line)
		if len([]rune(word)) < minWordRunes || seen[word] {
			continue
		}
		seen[word] = true
		words = append(words, word)
	}
	g.words = words
}

// normalize folds text to the comparison skeleton: NFKC (full-width → ASCII),
// lower case, leet-digit folding, then everything that is not a letter or digit
// is dropped (spaces, punctuation, symbols, zero-width joiners, marks).
func normalize(text string) string {
	if text == "" {
		return ""
	}
	text = norm.NFKC.String(text)
	var builder strings.Builder
	builder.Grow(len(text))
	for _, r := range text {
		switch r {
		case '0':
			r = 'o'
		case '1':
			r = 'l'
		case '3':
			r = 'e'
		case '4':
			r = 'a'
		case '5':
			r = 's'
		case '7':
			r = 't'
		case '8':
			r = 'b'
		case '9':
			r = 'g'
		case '@':
			r = 'a'
		case '$':
			r = 's'
		default:
			r = unicode.ToLower(r)
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
