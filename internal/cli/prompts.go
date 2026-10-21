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

// SelectOne displays a modern searchable prompt with fuzzy matching.
func (p *InteractivePrompter) SelectOne(title string, items []SelectItem) (string, error) {
	if len(items) == 0 {
		return "", fmt.Errorf("no options available")
	}

	scanner := bufio.NewScanner(p.reader)

	for {
		fmt.Fprintf(p.writer, "\n%s\n", title)
		fmt.Fprintf(p.writer, "%s\n", strings.Repeat("-", len(title)))

		// If 5 or fewer items, show them as quick options
		if len(items) <= 5 {
			for i, item := range items {
				fmt.Fprintf(p.writer, "  %d) %s\n", i+1, item.Label)
			}
			fmt.Fprintf(p.writer, "\nType branch name or number (1-%d), or 'q' to cancel: ", len(items))
		} else {
			// For larger lists, do not dump all branches; prompt user to type to filter
			fmt.Fprintf(p.writer, "  (%d branches available - type any part of the name to search)\n\n", len(items))
			fmt.Fprintf(p.writer, "Search branch name (or 'q' to cancel): ")
		}

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

		// Check exact number if within range of all items
		if num, err := strconv.Atoi(input); err == nil {
			if num >= 1 && num <= len(items) {
				selected := items[num-1]
				fmt.Fprintf(p.writer, "  -> Selected: %s\n", selected.Label)
				return selected.Value, nil
			}
			fmt.Fprintf(p.writer, "  Invalid number. Please enter 1-%d.\n", len(items))
			continue
		}

		// Search items using fuzzy scoring
		matches := fuzzyFilter(items, input)
		if len(matches) == 0 {
			fmt.Fprintf(p.writer, "  No branches matching '%s'. Please try another term.\n", input)
			continue
		}

		// Exact match found or single candidate
		if len(matches) == 1 {
			fmt.Fprintf(p.writer, "  -> Selected: %s\n", matches[0].Label)
			return matches[0].Value, nil
		}

		// Check if first match is an exact string match for convenience
		if strings.EqualFold(matches[0].Value, input) || strings.EqualFold(matches[0].Label, input) {
			fmt.Fprintf(p.writer, "  -> Selected: %s\n", matches[0].Label)
			return matches[0].Value, nil
		}

		// Show top fuzzy matches
		maxShow := 8
		if len(matches) < maxShow {
			maxShow = len(matches)
		}

		fmt.Fprintf(p.writer, "\n  Top matches for '%s' (%d total found):\n", input, len(matches))
		for i := 0; i < maxShow; i++ {
			fmt.Fprintf(p.writer, "  %2d) %s\n", i+1, matches[i].Label)
		}
		if len(matches) > maxShow {
			fmt.Fprintf(p.writer, "      ... and %d more (refine your search to narrow down)\n", len(matches)-maxShow)
		}

		fmt.Fprintf(p.writer, "\nEnter number (1-%d), refine search, or 'q' to cancel: ", maxShow)
		if !scanner.Scan() {
			return "", ErrUserCancelled
		}
		subInput := strings.TrimSpace(scanner.Text())

		if subInput == "q" || subInput == "Q" {
			return "", ErrUserCancelled
		}

		if subInput == "" {
			continue
		}

		if num, err := strconv.Atoi(subInput); err == nil {
			if num >= 1 && num <= maxShow {
				selected := matches[num-1]
				fmt.Fprintf(p.writer, "  -> Selected: %s\n", selected.Label)
				return selected.Value, nil
			}
			fmt.Fprintf(p.writer, "  Invalid choice. Let's try searching again.\n")
			continue
		}

		// User entered another search query directly
		subMatches := fuzzyFilter(matches, subInput)
		if len(subMatches) == 1 {
			fmt.Fprintf(p.writer, "  -> Selected: %s\n", subMatches[0].Label)
			return subMatches[0].Value, nil
		} else if len(subMatches) > 1 {
			fmt.Fprintf(p.writer, "  -> Selected best match: %s\n", subMatches[0].Label)
			return subMatches[0].Value, nil
		}

		fmt.Fprintf(p.writer, "  No matches for '%s'. Restarting search.\n", subInput)
	}
}

// fuzzyMatch checks if pattern characters appear sequentially in target (case-insensitive)
// and returns a score (higher is better).
func fuzzyScore(target, pattern string) (bool, int) {
	tLower := strings.ToLower(target)
	pLower := strings.ToLower(pattern)

	// Direct substring match gets high priority
	if idx := strings.Index(tLower, pLower); idx != -1 {
		score := 1000 - idx*10
		if idx == 0 {
			score += 500 // Prefix match bonus
		}
		return true, score
	}

	// Sequential character fuzzy matching
	tIdx := 0
	score := 0
	prevMatch := -2

	for _, pRune := range pLower {
		found := false
		for tIdx < len(tLower) {
			if rune(tLower[tIdx]) == pRune {
				found = true
				if tIdx == prevMatch+1 {
					score += 20 // Consecutive characters bonus
				} else {
					score += 5
				}
				prevMatch = tIdx
				tIdx++
				break
			}
			tIdx++
		}
		if !found {
			return false, 0
		}
	}

	return true, score
}

// fuzzyFilter returns items matching pattern sorted by best match score.
func fuzzyFilter(items []SelectItem, term string) []SelectItem {
	type scoredItem struct {
		item  SelectItem
		score int
	}

	var matched []scoredItem
	for _, item := range items {
		okLabel, score1 := fuzzyScore(item.Label, term)
		okVal, score2 := fuzzyScore(item.Value, term)

		if okLabel || okVal {
			bestScore := score1
			if score2 > bestScore {
				bestScore = score2
			}
			matched = append(matched, scoredItem{item: item, score: bestScore})
		}
	}

	// Sort by score descending (simple stable sort)
	for i := 0; i < len(matched); i++ {
		for j := i + 1; j < len(matched); j++ {
			if matched[j].score > matched[i].score {
				matched[i], matched[j] = matched[j], matched[i]
			}
		}
	}

	result := make([]SelectItem, len(matched))
	for i, m := range matched {
		result[i] = m.item
	}
	return result
}

// searchItems filters items by a search term (case-insensitive substring match).
func searchItems(items []SelectItem, term string) []SelectItem {
	return fuzzyFilter(items, term)
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
