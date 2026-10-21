package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestUI_Info(t *testing.T) {
	var buf bytes.Buffer
	ui := NewUIWithWriter(&buf)
	ui.Info("test message")
	if !strings.Contains(buf.String(), "test message") {
		t.Errorf("expected 'test message' in output, got: %s", buf.String())
	}
}

func TestUI_Success(t *testing.T) {
	var buf bytes.Buffer
	ui := NewUIWithWriter(&buf)
	ui.Success("operation complete")
	if !strings.Contains(buf.String(), "operation complete") {
		t.Errorf("expected message in output, got: %s", buf.String())
	}
}

func TestUI_Warning(t *testing.T) {
	var buf bytes.Buffer
	ui := NewUIWithWriter(&buf)
	ui.Warning("be careful")
	output := buf.String()
	if !strings.Contains(output, "be careful") {
		t.Errorf("expected warning text, got: %s", output)
	}
	if !strings.Contains(output, "WARNING") {
		t.Errorf("expected WARNING prefix, got: %s", output)
	}
}

func TestUI_Error(t *testing.T) {
	var buf bytes.Buffer
	ui := NewUIWithWriter(&buf)
	ui.Error("something failed")
	output := buf.String()
	if !strings.Contains(output, "something failed") {
		t.Errorf("expected error text, got: %s", output)
	}
	if !strings.Contains(output, "ERROR") {
		t.Errorf("expected ERROR prefix, got: %s", output)
	}
}

func TestUI_Step(t *testing.T) {
	var buf bytes.Buffer
	ui := NewUIWithWriter(&buf)
	ui.Step("A.1", "Fetching remote branches")
	output := buf.String()
	if !strings.Contains(output, "[A.1]") {
		t.Errorf("expected '[A.1]' in output, got: %s", output)
	}
	if !strings.Contains(output, "Fetching remote branches") {
		t.Errorf("expected step description, got: %s", output)
	}
}

func TestUI_FileList(t *testing.T) {
	var buf bytes.Buffer
	ui := NewUIWithWriter(&buf)
	ui.FileList("Conflicted files:", []string{"file1.go", "file2.go"})
	output := buf.String()
	if !strings.Contains(output, "file1.go") || !strings.Contains(output, "file2.go") {
		t.Errorf("expected file names in output, got: %s", output)
	}
}

func TestUI_SummaryLine(t *testing.T) {
	var buf bytes.Buffer
	ui := NewUIWithWriter(&buf)
	ui.SummaryLine("Repository", "my-repo")
	output := buf.String()
	if !strings.Contains(output, "Repository") || !strings.Contains(output, "my-repo") {
		t.Errorf("expected key-value pair, got: %s", output)
	}
}

func TestUI_BranchName_NoColor(t *testing.T) {
	var buf bytes.Buffer
	ui := NewUIWithWriter(&buf)
	name := ui.BranchName("test/branch")
	// Without color, should return the name unchanged
	if name != "test/branch" {
		t.Errorf("expected plain 'test/branch', got %q", name)
	}
}

func TestUI_Dim_NoColor(t *testing.T) {
	var buf bytes.Buffer
	ui := NewUIWithWriter(&buf)
	dimmed := ui.Dim("some text")
	if dimmed != "some text" {
		t.Errorf("expected plain text without color, got %q", dimmed)
	}
}

func TestUI_FormattedMethods(t *testing.T) {
	var buf bytes.Buffer
	ui := NewUIWithWriter(&buf)
	ui.Infof("count: %d", 42)
	if !strings.Contains(buf.String(), "count: 42") {
		t.Errorf("expected formatted output, got: %s", buf.String())
	}

	buf.Reset()
	ui.Successf("done in %ds", 3)
	if !strings.Contains(buf.String(), "done in 3s") {
		t.Errorf("expected formatted output, got: %s", buf.String())
	}

	buf.Reset()
	ui.Warningf("try %d", 2)
	if !strings.Contains(buf.String(), "try 2") {
		t.Errorf("expected formatted output, got: %s", buf.String())
	}

	buf.Reset()
	ui.Errorf("fail #%d", 1)
	if !strings.Contains(buf.String(), "fail #1") {
		t.Errorf("expected formatted output, got: %s", buf.String())
	}
}

func TestPrompter_SelectOne_ByNumber(t *testing.T) {
	input := strings.NewReader("1\n")
	var output bytes.Buffer
	p := NewPrompterWithIO(input, &output)

	items := []SelectItem{
		{Label: "Option A", Value: "a"},
		{Label: "Option B", Value: "b"},
	}

	val, err := p.SelectOne("Pick one:", items)
	if err != nil {
		t.Fatal(err)
	}
	if val != "a" {
		t.Errorf("expected 'a', got %q", val)
	}
}

func TestPrompter_SelectOne_BySearch(t *testing.T) {
	input := strings.NewReader("Option B\n")
	var output bytes.Buffer
	p := NewPrompterWithIO(input, &output)

	items := []SelectItem{
		{Label: "Option A", Value: "a"},
		{Label: "Option B", Value: "b"},
	}

	val, err := p.SelectOne("Pick one:", items)
	if err != nil {
		t.Fatal(err)
	}
	if val != "b" {
		t.Errorf("expected 'b', got %q", val)
	}
}

func TestPrompter_SelectOne_Cancel(t *testing.T) {
	input := strings.NewReader("q\n")
	var output bytes.Buffer
	p := NewPrompterWithIO(input, &output)

	items := []SelectItem{
		{Label: "Option A", Value: "a"},
	}

	_, err := p.SelectOne("Pick one:", items)
	if err != ErrUserCancelled {
		t.Errorf("expected ErrUserCancelled, got %v", err)
	}
}

func TestPrompter_SelectOne_Empty(t *testing.T) {
	input := strings.NewReader("")
	var output bytes.Buffer
	p := NewPrompterWithIO(input, &output)

	_, err := p.SelectOne("Pick one:", nil)
	if err == nil {
		t.Error("expected error for empty items")
	}
}

func TestPrompter_Confirm_Yes(t *testing.T) {
	input := strings.NewReader("y\n")
	var output bytes.Buffer
	p := NewPrompterWithIO(input, &output)

	val, err := p.Confirm("Continue?", false)
	if err != nil {
		t.Fatal(err)
	}
	if !val {
		t.Error("expected true for 'y' input")
	}
}

func TestPrompter_Confirm_No(t *testing.T) {
	input := strings.NewReader("n\n")
	var output bytes.Buffer
	p := NewPrompterWithIO(input, &output)

	val, err := p.Confirm("Continue?", true)
	if err != nil {
		t.Fatal(err)
	}
	if val {
		t.Error("expected false for 'n' input")
	}
}

func TestPrompter_Confirm_Default(t *testing.T) {
	input := strings.NewReader("\n")
	var output bytes.Buffer
	p := NewPrompterWithIO(input, &output)

	val, err := p.Confirm("Continue?", true)
	if err != nil {
		t.Fatal(err)
	}
	if !val {
		t.Error("expected true for default=true with empty input")
	}

	input2 := strings.NewReader("\n")
	p2 := NewPrompterWithIO(input2, &output)
	val2, err := p2.Confirm("Continue?", false)
	if err != nil {
		t.Fatal(err)
	}
	if val2 {
		t.Error("expected false for default=false with empty input")
	}
}

func TestPrompter_Confirm_Cancel(t *testing.T) {
	input := strings.NewReader("q\n")
	var output bytes.Buffer
	p := NewPrompterWithIO(input, &output)

	_, err := p.Confirm("Continue?", true)
	if err != ErrUserCancelled {
		t.Errorf("expected ErrUserCancelled, got %v", err)
	}
}

func TestSearchItems(t *testing.T) {
	items := []SelectItem{
		{Label: "origin/feature/payment", Value: "origin/feature/payment"},
		{Label: "origin/feature/auth", Value: "origin/feature/auth"},
		{Label: "origin/develop", Value: "origin/develop"},
		{Label: "origin/test/it-20261020", Value: "origin/test/it-20261020"},
	}

	tests := []struct {
		term          string
		expectedCount int
	}{
		{"payment", 1},
		{"feature", 2},
		{"origin", 4},
		{"test", 1},
		{"nonexistent", 0},
		{"PAYMENT", 1}, // case insensitive
	}

	for _, tt := range tests {
		t.Run(tt.term, func(t *testing.T) {
			matches := searchItems(items, tt.term)
			if len(matches) != tt.expectedCount {
				t.Errorf("search(%q): expected %d matches, got %d: %+v",
					tt.term, tt.expectedCount, len(matches), matches)
			}
		})
	}
}

func TestMockPrompter(t *testing.T) {
	mock := &MockPrompter{
		SelectResponses:  []string{"value1", "value2"},
		ConfirmResponses: []bool{true, false},
	}

	val, _ := mock.SelectOne("test", []SelectItem{{Label: "a", Value: "value1"}})
	if val != "value1" {
		t.Errorf("expected 'value1', got %q", val)
	}

	val, _ = mock.SelectOne("test2", []SelectItem{{Label: "b", Value: "value2"}})
	if val != "value2" {
		t.Errorf("expected 'value2', got %q", val)
	}

	// Third call should return cancel
	_, err := mock.SelectOne("test3", []SelectItem{{Label: "c", Value: "c"}})
	if err != ErrUserCancelled {
		t.Errorf("expected ErrUserCancelled, got %v", err)
	}

	// Check recorded calls
	if len(mock.SelectCalls) != 3 {
		t.Errorf("expected 3 select calls, got %d", len(mock.SelectCalls))
	}

	confirmed, _ := mock.Confirm("q1", false)
	if !confirmed {
		t.Error("expected true for first confirm")
	}

	confirmed, _ = mock.Confirm("q2", true)
	if confirmed {
		t.Error("expected false for second confirm")
	}
}
