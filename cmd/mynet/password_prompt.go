package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// promptPassword reads a secret from the terminal without echoing input.
func promptPassword(label string) string {
	fmt.Fprint(os.Stderr, label)
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err == nil {
			return strings.TrimSpace(string(b))
		}
	}
	r := bufio.NewReader(os.Stdin)
	line, _ := r.ReadString('\n')
	fmt.Fprintln(os.Stderr)
	return strings.TrimSpace(line)
}
