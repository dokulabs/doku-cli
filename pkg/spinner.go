package pkg

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
)

// Spinner wraps briandowns/spinner with our custom methods
type Spinner struct {
	spinner      *spinner.Spinner
	mutex        sync.Mutex
	successColor *color.Color
	errorColor   *color.Color
	promptColor  *color.Color
	infoColor    *color.Color
	warningColor *color.Color
	noticeColor  *color.Color
}

// NewSpinner creates a new spinner with default configuration
func NewSpinner() *Spinner {
	s := spinner.New(
		spinner.CharSets[9],  // Same frames as your original
		100*time.Millisecond, // Frame rate
		spinner.WithHiddenCursor(false),
	)

	return &Spinner{
		spinner:      s,
		successColor: color.New(color.FgGreen, color.Bold),
		errorColor:   color.New(color.FgRed, color.Bold),
		promptColor:  color.New(color.FgCyan, color.Bold),
		infoColor:    color.New(color.FgBlue, color.Bold),
		warningColor: color.New(color.FgYellow, color.Bold),
		noticeColor:  color.New(color.FgMagenta, color.Bold),
	}
}

// Start begins the spinner animation
func (s *Spinner) Start(format string, a ...interface{}) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	message := fmt.Sprintf(format, a...)
	s.spinner.Suffix = " " + message

	if !s.spinner.Active() {
		s.spinner.Start()
	}
}

// Stop ends the spinner animation and displays a final message if provided
func (s *Spinner) Stop(format string, a ...interface{}) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.spinner.Active() {
		s.spinner.Stop()
	}

	if format != "" {
		message := fmt.Sprintf(format, a...)
		s.successColor.Printf("✓ %s\n", message)
	}
}

// UpdateMessage changes the spinner message
func (s *Spinner) UpdateMessage(format string, a ...interface{}) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	message := fmt.Sprintf(format, a...)
	s.spinner.Suffix = " " + message + "\n"
}

// Success prints a success message
func (s *Spinner) Success(format string, a ...interface{}) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	wasActive := s.spinner.Active()
	if wasActive {
		s.spinner.Stop()
	}

	message := fmt.Sprintf(format, a...)
	s.successColor.Printf("✓ %s\n", message)

	if wasActive {
		s.spinner.Start()
	}
}

// Info prints an informational message
func (s *Spinner) Info(format string, a ...interface{}) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	wasActive := s.spinner.Active()
	if wasActive {
		s.spinner.Stop()
	}

	message := fmt.Sprintf(format, a...)
	s.infoColor.Printf("ℹ %s\n", message)

	if wasActive {
		s.spinner.Start()
	}
}

// Warning prints a warning message
func (s *Spinner) Warning(format string, a ...interface{}) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	wasActive := s.spinner.Active()
	if wasActive {
		s.spinner.Stop()
	}

	message := fmt.Sprintf(format, a...)
	s.warningColor.Printf("⚠ %s\n", message)

	if wasActive {
		s.spinner.Start()
	}
}

// Notice prints a notice message
func (s *Spinner) Notice(format string, a ...interface{}) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	wasActive := s.spinner.Active()
	if wasActive {
		s.spinner.Stop()
	}

	message := fmt.Sprintf(format, a...)
	s.noticeColor.Printf("• %s\n", message)

	if wasActive {
		s.spinner.Start()
	}
}

// Error stops the spinner and exits with the error message
func (s *Spinner) Error(format string, a ...interface{}) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.spinner.Active() {
		s.spinner.Stop()
	}

	message := fmt.Sprintf(format, a...)
	s.errorColor.Printf("✗ %s\n", message)
	os.Exit(1)
}

// Prompt pauses the spinner, shows a prompt, and returns user input
func (s *Spinner) Prompt(format string, a ...interface{}) string {
	s.mutex.Lock()
	wasActive := s.spinner.Active()
	if wasActive {
		s.spinner.Stop()
	}
	s.mutex.Unlock()

	message := fmt.Sprintf(format, a...)
	s.promptColor.Printf("? %s: ", message)

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	s.mutex.Lock()
	if wasActive {
		s.spinner.Start()
	}
	s.mutex.Unlock()

	return input
}
