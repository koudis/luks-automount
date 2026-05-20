package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"luks-automount/internal/config"
)

func exactArgs(count int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == count {
			return nil
		}

		usage := commandUsage(cmd)
		spec := argumentSpec(cmd)
		if len(args) < count {
			if count == 1 && spec != "" {
				return fmt.Errorf("missing required argument %s; usage: %s", spec, usage)
			}
			if spec != "" {
				return fmt.Errorf("missing required arguments %s; usage: %s", spec, usage)
			}
			return fmt.Errorf("missing required arguments; usage: %s", usage)
		}

		return fmt.Errorf("unexpected extra arguments for %s: %s; usage: %s", cmd.CommandPath(), strings.Join(args[count:], " "), usage)
	}
}

func noArgs() cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}

		usage := commandUsage(cmd)
		return fmt.Errorf("unexpected arguments for %s: %s; usage: %s", cmd.CommandPath(), strings.Join(args, " "), usage)
	}
}

func commandUsage(cmd *cobra.Command) string {
	usage := cmd.CommandPath()
	spec := argumentSpec(cmd)
	if spec == "" {
		return usage
	}
	return usage + " " + spec
}

func argumentSpec(cmd *cobra.Command) string {
	return strings.TrimSpace(strings.TrimPrefix(cmd.Use, cmd.Name()))
}

func loadConfig() (*config.Config, error) {
	path, err := config.DefaultPath()
	if err != nil {
		return nil, err
	}
	return config.Load(path)
}

func readPassphrase(prompt string) ([]byte, error) {
	fmt.Fprint(os.Stderr, prompt)
	if term.IsTerminal(int(syscall.Stdin)) {
		pass, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return nil, err
		}
		return pass, nil
	}
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	return []byte(strings.TrimRight(line, "\n")), nil
}

func readLine(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, os.ErrClosed) {
		return "", err
	}
	return strings.TrimRight(line, "\n"), nil
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
