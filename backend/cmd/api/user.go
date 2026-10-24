package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/auth"
	"golang.org/x/term"
)

// manageUser is an operator command; passwords never appear in process arguments or logs.
func manageUser(ctx context.Context, users *auth.Postgres, args []string) error {
	if len(args) == 0 || args[0] != "user" {
		return errors.New("usage: /server user --username NAME [--password-stdin]")
	}
	flags := flag.NewFlagSet("user", flag.ContinueOnError)
	username := flags.String("username", "", "employee login")
	stdin := flags.Bool("password-stdin", false, "read one password line from stdin")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || *username == "" {
		return errors.New("usage: /server user --username NAME [--password-stdin]")
	}
	if err := auth.ValidateUsername(*username); err != nil {
		return err
	}
	var password string
	if *stdin {
		scanner := bufio.NewScanner(io.LimitReader(os.Stdin, 1024))
		if !scanner.Scan() {
			return errors.New("password is required on stdin")
		}
		password = scanner.Text()
	} else {
		fd := int(os.Stdin.Fd())
		if !term.IsTerminal(fd) {
			return errors.New("use an interactive terminal or --password-stdin")
		}
		for {
			fmt.Fprint(os.Stderr, "Пароль (для латиницы 12–72 символа, ввод скрыт): ")
			first, err := term.ReadPassword(fd)
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return errors.New("cannot read password")
			}
			if err := auth.ValidatePassword(string(first)); err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			fmt.Fprint(os.Stderr, "Повторите пароль: ")
			second, err := term.ReadPassword(fd)
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return errors.New("cannot read password confirmation")
			}
			if string(first) != string(second) {
				fmt.Fprintln(os.Stderr, "Пароли не совпадают. Введите пароль заново.")
				continue
			}
			password = string(first)
			break
		}
	}
	writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := users.SetPassword(writeCtx, *username, password); err != nil {
		return err
	}
	_, err := fmt.Fprintln(os.Stdout, "Пользователь сохранён. Его прежние сессии отозваны.")
	return err
}
