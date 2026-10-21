package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// ErrUserCancelled is returned when the user cancels a prompt.
var ErrUserCancelled = errors.New("operation cancelled by user")

// SelectItem represents an option in a selection menu.
type SelectItem struct {
	Label string
	Value string
}

// Prompter defines the interface for interactive prompts.
// This interface enables testing with mock implementations.
type Prompter interface {
	SelectOne(title string, items []SelectItem) (string, error)
	Confirm(title string, defaultVal bool) (bool, error)
	WaitForUser(message string) error
}

// InteractivePrompter implements Prompter using stdin/stdout.
type InteractivePrompter struct {
	reader io.Reader
	writer io.Writer
	ui     *UI
}

// NewPrompter creates a new interactive prompter using stdin/stdout.
func NewPrompter() *InteractivePrompter {
	return &InteractivePrompter{
		reader: os.Stdin,
		writer: os.Stdout,
		ui:     NewUI(),
	}
}

// NewPrompterWithIO creates a prompter with custom I/O for testing.
func NewPrompterWithIO(reader io.Reader, writer io.Writer) *InteractivePrompter {
	return &InteractivePrompter{
		reader: reader,
		writer: writer,
		ui:     NewUIWithWriter(writer),
	}
}

// NewAccessiblePrompter creates a prompter that works without TTY support.
func NewAccessiblePrompter() *InteractivePrompter {
	return NewPrompter()
}

// SelectOne displays a numbered selection menu with search support.
func (p *InteractivePrompter) SelectOne(title string, items []SelectItem) (string, error) {
	if len(items) == 0 {
		return "", fmt.Errorf("no options available")
	}

	scanner := bufio.NewScanner(p.reader)

	for {
		fmt.Fprintf(p.writer, "\n%s\n", title)
		fmt.Fprintf(p.writer, "%s\n\n", strings.Repeat("-", len(title)))

		// Display items with numbers
		for i, item := range items {
			fmt.Fprintf(p.writer, "  %3d) %s\n", i+1, item.Label)
		}

		fmt.Fprintf(p.writer, "\nEnter number (1-%d), search term, or 'q' to cancel: ", len(items))

		if !scanner.Scan() {
			return "", ErrUserCancelled
		}
		input := strings.TrimSpace(scanner.Text())

		if input == "" {
			continue
		}

		// Cancel
		if input == "q" || input == "Q" || input == "quit" || input == "cancel" {
			return "", ErrUserCancelled
		}

		// Try as number
		if num, err := strconv.Atoi(input); err == nil {
			if num >= 1 && num <= len(items) {
				selected := items[num-1]
				fmt.Fprintf(p.writer, "  -> Selected: %s\n", selected.Label)
				return selected.Value, nil
			}
			fmt.Fprintf(p.writer, "  Invalid number. Please enter 1-%d.\n", len(items))
			continue
		}

		// Search by text
		matches := searchItems(items, input)
		if len(matches) == 0 {
			fmt.Fprintf(p.writer, "  No matches for '%s'. Try again.\n", input)
			continue
		}
		if len(matches) == 1 {
			fmt.Fprintf(p.writer, "  -> Selected: %s\n", matches[0].Label)
			return matches[0].Value, nil
		}

		// Show filtered results
		fmt.Fprintf(p.writer, "\n  Matches for '%s':\n", input)
		for i, m := range matches {
			fmt.Fprintf(p.writer, "  %3d) %s\n", i+1, m.Label)
		}
		fmt.Fprintf(p.writer, "\nEnter number (1-%d) or 'q' to cancel: ", len(matches))

		if !scanner.Scan() {
			return "", ErrUserCancelled
		}
		subInput := strings.TrimSpace(scanner.Text())

		if subInput == "q" || subInput == "Q" {
			return "", ErrUserCancelled
		}

		if num, err := strconv.Atoi(subInput); err == nil {
			if num >= 1 && num <= len(matches) {
				selected := matches[num-1]
				fmt.Fprintf(p.writer, "  -> Selected: %s\n", selected.Label)
				return selected.Value, nil
			}
		}
		fmt.Fprintf(p.writer, "  Invalid selection. Starting over.\n")
	}
}

// searchItems filters items by a search term (case-insensitive substring match).
func searchItems(items []SelectItem, term string) []SelectItem {
	lower := strings.ToLower(term)
	var matches []SelectItem
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Label), lower) ||
			strings.Contains(strings.ToLower(item.Value), lower) {
			matches = append(matches, item)
		}
	}
	return matches
}

// Confirm displays a yes/no confirmation prompt.
func (p *InteractivePrompter) Confirm(title string, defaultVal bool) (bool, error) {
	scanner := bufio.NewScanner(p.reader)

	hint := "[y/N]"
	if defaultVal {
		hint = "[Y/n]"
	}

	for {
		fmt.Fprintf(p.writer, "%s %s: ", title, hint)

		if !scanner.Scan() {
			return false, ErrUserCancelled
		}
		input := strings.TrimSpace(strings.ToLower(scanner.Text()))

		switch input {
		case "":
			return defaultVal, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		case "q", "quit", "cancel":
			return false, ErrUserCancelled
		default:
			fmt.Fprintf(p.writer, "  Please enter 'y' or 'n'.\n")
		}
	}
}

// WaitForUser displays a message and waits for the user to press Enter.
func (p *InteractivePrompter) WaitForUser(message string) error {
	confirmed, err := p.Confirm(message, true)
	if err != nil {
		return err
	}
	if !confirmed {
		return ErrUserCancelled
	}
	return nil
}

// MockPrompter is a test implementation of Prompter that returns predetermined values.
type MockPrompter struct {
	SelectResponses  []string
	ConfirmResponses []bool
	WaitResponses    []error
	selectIdx        int
	confirmIdx       int
	waitIdx          int
	SelectCalls      []string
	ConfirmCalls     []string
}

// SelectOne returns the next predetermined selection.
func (m *MockPrompter) SelectOne(title string, items []SelectItem) (string, error) {
	m.SelectCalls = append(m.SelectCalls, title)
	if m.selectIdx >= len(m.SelectResponses) {
		return "", ErrUserCancelled
	}
	val := m.SelectResponses[m.selectIdx]
	m.selectIdx++
	return val, nil
}

// Confirm returns the next predetermined confirmation.
func (m *MockPrompter) Confirm(title string, defaultVal bool) (bool, error) {
	m.ConfirmCalls = append(m.ConfirmCalls, title)
	if m.confirmIdx >= len(m.ConfirmResponses) {
		return false, ErrUserCancelled
	}
	val := m.ConfirmResponses[m.confirmIdx]
	m.confirmIdx++
	return val, nil
}

// WaitForUser returns the next predetermined response.
func (m *MockPrompter) WaitForUser(message string) error {
	if m.waitIdx >= len(m.WaitResponses) {
		return nil
	}
	err := m.WaitResponses[m.waitIdx]
	m.waitIdx++
	return err
}
