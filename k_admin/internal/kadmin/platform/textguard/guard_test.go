package textguard

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckCollapsesCommonEvasions(t *testing.T) {
	guard := FromWords([]string{"色情", "代考", "porn"})
	for _, text := range []string{
		"色情",
		"色 情",
		"色.情",
		"色*情",
		"色\u200b情",  // 零宽空格
		"色（情）",      // 全角标点
		"【色情】合集",    // 前后缀
		"代\t考",      // 制表符
		"ｐｏｒｎ",      // 全角拉丁
		"P O R N",   // 空格 + 大写
		"p0rn-star", // 数字替形
	} {
		if _, hit := guard.Check(text); !hit {
			t.Errorf("Check(%q) = no hit", text)
		}
	}
	for _, text := range []string{"高等数学", "Go 官网", "试卷A.pdf", "", "色", "代 课"} {
		if word, hit := guard.Check(text); hit {
			t.Errorf("Check(%q) hit %q, want none", text, word)
		}
	}
}

func TestSingleRuneWordsAreIgnored(t *testing.T) {
	guard := FromWords([]string{"色", "赌", "黄"})
	if len(guard.Words()) != 0 {
		t.Fatalf("words = %v, want none", guard.Words())
	}
	if _, hit := guard.Check("色情网站"); hit {
		t.Fatal("single-rune word must not block text")
	}
}

func TestCheckFieldsReportsFirstLabel(t *testing.T) {
	guard := FromWords([]string{"代考"})
	label, word, hit := guard.CheckFields([]Field{
		{Label: "标题", Value: "高等数学"},
		{Label: "简介", Value: "代 考 服务"},
		{Label: "备注", Value: "代考"},
	})
	if !hit || label != "简介" || word != normalizeWord("代考") {
		t.Fatalf("label=%q word=%q hit=%v", label, word, hit)
	}
}

// normalizeWord keeps the assertion readable: the dictionary stores the folded
// form, which for a plain CJK word is the word itself.
func normalizeWord(word string) string { return normalize(word) }

func TestWordFileLoadsAndHotReloads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "words.txt")
	writeWordsFile(t, path, "# 注释行\n色情\n\n代考\n", time.Unix(1700000000, 0))

	guard := New()
	guard.SetPath(path)
	if word, hit := guard.Check("代 考"); !hit || word != "代考" {
		t.Fatalf("initial load: word=%q hit=%v", word, hit)
	}

	// 改动文件（大小与 mtime 都变）后无需重启即可生效。
	writeWordsFile(t, path, "# 新词表\n赌博网站\n", time.Unix(1700003600, 0))
	if _, hit := guard.Check("代考"); hit {
		t.Fatal("removed word still matches after reload")
	}
	if _, hit := guard.Check("赌博 网站"); !hit {
		t.Fatal("added word did not match after reload")
	}
}

func TestMissingWordFileNeverBlocks(t *testing.T) {
	guard := New()
	guard.SetPath(filepath.Join(t.TempDir(), "absent.txt"))
	if _, hit := guard.Check("色情"); hit {
		t.Fatal("missing word file must not block writes")
	}
	// 清空路径同样等于没有词表。
	guard.SetPath("")
	if len(guard.Words()) != 0 {
		t.Fatalf("words = %v, want none", guard.Words())
	}
}

func TestPinnedGuardIgnoresSetPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "words.txt")
	writeWordsFile(t, path, "赌博\n", time.Unix(1700000000, 0))
	guard := FromWords([]string{"代考"})
	guard.SetPath(path)
	if _, hit := guard.Check("赌博"); hit {
		t.Fatal("pinned guard must not load the file")
	}
	if _, hit := guard.Check("代考"); !hit {
		t.Fatal("pinned words lost after SetPath")
	}
}

func writeWordsFile(t *testing.T, path, content string, modTime time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write words file: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
}
