package tui

import "testing"

func TestVimStartsInInsertAndEscapeEntersNormal(t *testing.T) {
	editor := newVimEditor()
	editor.InsertText("你好 world")
	editor.Handle("esc")

	if editor.Mode() != vimNormal {
		t.Fatalf("mode = %s, want NORMAL", editor.Mode())
	}
	if editor.Cursor() != len([]rune("你好 world"))-1 {
		t.Fatalf("cursor = %d, want last character", editor.Cursor())
	}
}

func TestVimNormalHorizontalAndWordMotions(t *testing.T) {
	editor := vimEditorWithValue("alpha, beta 世界")

	editor.Handle("0")
	assertVimCursor(t, editor, 0)
	editor.Handle("w")
	assertVimCursor(t, editor, 5)
	editor.Handle("w")
	assertVimCursor(t, editor, 7)
	editor.Handle("e")
	assertVimCursor(t, editor, 10)
	editor.Handle("w")
	assertVimCursor(t, editor, 12)
	editor.Handle("b")
	assertVimCursor(t, editor, 7)
	editor.Handle("h")
	assertVimCursor(t, editor, 6)
	editor.Handle("l")
	assertVimCursor(t, editor, 7)
}

func TestVimWordEndAdvancesFromCurrentWordEnd(t *testing.T) {
	editor := vimEditorWithValue("one two")
	editor.Handle("0")
	editor.Handle("e")
	assertVimCursor(t, editor, 2)
	editor.Handle("e")
	assertVimCursor(t, editor, 6)
}

func TestVimNormalLineMotionsPreserveRuneColumn(t *testing.T) {
	editor := vimEditorWithValue("zero\n你好世界\nlast")

	editor.Handle("g")
	editor.Handle("g")
	editor.Handle("l")
	editor.Handle("l")
	editor.Handle("j")
	assertVimCursor(t, editor, len([]rune("zero\n你好")))
	editor.Handle("j")
	assertVimCursor(t, editor, len([]rune("zero\n你好世界\nla")))
	editor.Handle("k")
	assertVimCursor(t, editor, len([]rune("zero\n你好")))
	editor.Handle("$")
	assertVimCursor(t, editor, len([]rune("zero\n你好世界"))-1)
	editor.Handle("^")
	assertVimCursor(t, editor, len([]rune("zero\n")))
	editor.Handle("G")
	assertVimCursor(t, editor, len([]rune("zero\n你好世界\nlast"))-1)
}

func TestVimChangeWordAndEndEnterInsert(t *testing.T) {
	for _, motion := range []string{"w", "e"} {
		t.Run("c"+motion, func(t *testing.T) {
			editor := vimEditorWithValue("alpha beta")
			editor.Handle("0")
			editor.Handle("c")
			editor.Handle(motion)

			if editor.Mode() != vimInsert {
				t.Fatalf("mode = %s, want INSERT", editor.Mode())
			}
			if got := editor.Value(); got != " beta" {
				t.Fatalf("value after c%s = %q, want %q", motion, got, " beta")
			}
			editor.InsertText("gamma")
			if got := editor.Value(); got != "gamma beta" {
				t.Fatalf("replacement = %q, want %q", got, "gamma beta")
			}
		})
	}
}

func TestVimChangeWordKeepsInsertionPointAwayFromLineStart(t *testing.T) {
	editor := vimEditorWithValue("alpha beta")
	editor.Handle("b")
	editor.Handle("c")
	editor.Handle("w")
	editor.InsertText("gamma")

	if got := editor.Value(); got != "alpha gamma" {
		t.Fatalf("value after changing trailing word = %q, want %q", got, "alpha gamma")
	}
}

func TestVimDeleteWordUsesMotionRange(t *testing.T) {
	editor := vimEditorWithValue("alpha beta")
	editor.Handle("0")
	editor.Handle("d")
	editor.Handle("w")

	if got := editor.Value(); got != "beta" {
		t.Fatalf("value after dw = %q, want beta", got)
	}
}

func TestVimWordOperatorIncludesFinalRune(t *testing.T) {
	t.Run("dw", func(t *testing.T) {
		editor := vimEditorWithValue("alpha")
		editor.Handle("0")
		editor.Handle("d")
		editor.Handle("w")
		if got := editor.Value(); got != "" {
			t.Fatalf("value after final dw = %q, want empty", got)
		}
	})

	t.Run("yw", func(t *testing.T) {
		editor := vimEditorWithValue("alpha")
		editor.Handle("0")
		editor.Handle("y")
		editor.Handle("w")
		editor.Handle("$")
		editor.Handle("p")
		if got := editor.Value(); got != "alphaalpha" {
			t.Fatalf("value after final yw then p = %q", got)
		}
	})
}

func TestVimYankEndAndCharacterPaste(t *testing.T) {
	editor := vimEditorWithValue("alpha beta")
	editor.Handle("0")
	editor.Handle("y")
	editor.Handle("e")
	editor.Handle("$")
	editor.Handle("p")

	if got := editor.Value(); got != "alpha betaalpha" {
		t.Fatalf("value after ye then p = %q", got)
	}
}

func TestVimLinewiseYankPasteDeleteAndChange(t *testing.T) {
	t.Run("yy and p", func(t *testing.T) {
		editor := vimEditorWithValue("one\ntwo")
		editor.Handle("g")
		editor.Handle("g")
		editor.Handle("y")
		editor.Handle("y")
		editor.Handle("j")
		editor.Handle("p")

		if got := editor.Value(); got != "one\ntwo\none" {
			t.Fatalf("value after yy then p = %q", got)
		}
	})

	t.Run("dd", func(t *testing.T) {
		editor := vimEditorWithValue("one\ntwo")
		editor.Handle("g")
		editor.Handle("g")
		editor.Handle("d")
		editor.Handle("d")

		if got := editor.Value(); got != "two" {
			t.Fatalf("value after dd = %q, want two", got)
		}
	})

	t.Run("cc", func(t *testing.T) {
		editor := vimEditorWithValue("one\ntwo")
		editor.Handle("g")
		editor.Handle("g")
		editor.Handle("c")
		editor.Handle("c")

		if editor.Mode() != vimInsert || editor.Value() != "\ntwo" {
			t.Fatalf("after cc mode=%s value=%q", editor.Mode(), editor.Value())
		}
	})
}

func TestVimVisualYankDeleteAndChange(t *testing.T) {
	t.Run("yank and paste", func(t *testing.T) {
		editor := vimEditorWithValue("alpha beta")
		editor.Handle("0")
		editor.Handle("v")
		editor.Handle("e")
		start, end, ok := editor.SelectionRange()
		if !ok || start != 0 || end != 5 {
			t.Fatalf("selection = (%d, %d, %t), want (0, 5, true)", start, end, ok)
		}
		editor.Handle("y")
		editor.Handle("$")
		editor.Handle("p")
		if got := editor.Value(); got != "alpha betaalpha" {
			t.Fatalf("visual yank paste = %q", got)
		}
	})

	t.Run("change", func(t *testing.T) {
		editor := vimEditorWithValue("alpha beta")
		editor.Handle("0")
		editor.Handle("v")
		editor.Handle("e")
		editor.Handle("c")
		editor.InsertText("gamma")
		if editor.Mode() != vimInsert || editor.Value() != "gamma beta" {
			t.Fatalf("visual change mode=%s value=%q", editor.Mode(), editor.Value())
		}
	})
}

func TestVimDeleteAndUndo(t *testing.T) {
	editor := vimEditorWithValue("你好 world")
	editor.Handle("0")
	editor.Handle("x")
	if got := editor.Value(); got != "好 world" {
		t.Fatalf("value after x = %q", got)
	}
	editor.Handle("u")
	if got := editor.Value(); got != "你好 world" {
		t.Fatalf("value after u = %q", got)
	}
}

func TestVimInsertEntryAndReplaceKeys(t *testing.T) {
	editor := vimEditorWithValue("  one\ntwo")
	editor.Handle("g")
	editor.Handle("g")
	editor.Handle("I")
	editor.InsertText("first ")
	editor.Handle("esc")
	editor.Handle("A")
	editor.InsertText("!")
	editor.Handle("esc")
	editor.Handle("0")
	editor.Handle("r")
	editor.Handle("X")

	if got := editor.Value(); got != "X first one!\ntwo" {
		t.Fatalf("insert/replace value = %q", got)
	}
}

func TestVimInsertEntryKeysAtLastLine(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		text string
		want string
	}{
		{name: "append", keys: []string{"a"}, text: "!", want: "one!"},
		{name: "append line", keys: []string{"0", "A"}, text: "!", want: "one!"},
		{name: "open below", keys: []string{"o"}, text: "two", want: "one\ntwo"},
		{name: "open above", keys: []string{"O"}, text: "zero", want: "zero\none"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			editor := vimEditorWithValue("one")
			for _, key := range tt.keys {
				editor.Handle(key)
			}
			editor.InsertText(tt.text)
			if got := editor.Value(); got != tt.want {
				t.Fatalf("value = %q, want %q", got, tt.want)
			}
		})
	}
}

func vimEditorWithValue(value string) vimEditor {
	editor := newVimEditor()
	editor.InsertText(value)
	editor.Handle("esc")
	return editor
}

func assertVimCursor(t *testing.T, editor vimEditor, want int) {
	t.Helper()
	if got := editor.Cursor(); got != want {
		t.Fatalf("cursor = %d, want %d (value %q)", got, want, editor.Value())
	}
}
